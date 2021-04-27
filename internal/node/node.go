package node

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Node is the object encapsulating the Raft node.
type Node struct {
	config         raft.Config
	raftNode       raft.Node
	backend        backend.DBBackend
	done           chan bool
	peerManager    *transport.PeerManager
	transportMgr   *transport.TransportManager
	nodeInfo       NodeInfo
	potentialPeers []NodeInfo
	isNewCluster   bool
	logger         *zap.SugaredLogger
	pendingWrites  *lib.ChannelPool
}

// NodeInfo groups the node's metadata outside of its Raft configuration
type NodeInfo struct {
	Addr string `json:"addr"`
	Port string `json:"port"`
}

// NodeParams is the group of params required for create a node object
type NodeParams struct {
	NodeID         uint64
	AddrPort       NodeInfo
	IsNewCluster   bool
	PotentialPeers []NodeInfo
}

// New creates new single-node RaftCluster
func New(
	params NodeParams,
	peerManager *transport.PeerManager,
	transportMgr *transport.TransportManager,
	storage raft.Storage,
	backend backend.DBBackend,
	logger *zap.SugaredLogger,
) *Node {
	c := raft.Config{
		ID:              params.NodeID,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	return &Node{
		config:         c,
		backend:        backend,
		done:           make(chan bool, 1),
		peerManager:    peerManager,
		transportMgr:   transportMgr,
		nodeInfo:       params.AddrPort,
		potentialPeers: params.PotentialPeers,
		isNewCluster:   params.IsNewCluster,
		logger:         logger,
		pendingWrites:  lib.NewChannelPool(),
	}
}

// Start runs the main Raft loop
func (n *Node) Start(ctx context.Context) error {
	n.logger.Info("starting node")

	if err := n.backend.Start(ctx); err != nil {
		return errors.Wrap(err, "starting backend")
	}

	var peers []raft.Peer
	if n.isNewCluster {
		b, err := json.Marshal(n.nodeInfo)
		if err != nil {
			return errors.Wrap(err, "marshalling nodeInfo")
		}
		peers = []raft.Peer{{ID: n.config.ID, Context: b}}
	}
	if err := n.loadStoredPeers(ctx); err != nil {
		n.logger.Errorf("failed loading stored peers: %+v", err)
	}
	n.discoverPotentialPeers(ctx, n.potentialPeers)
	n.raftNode = raft.StartNode(&n.config, peers)
	n.transportMgr.RegisterDestCallback(n.config.ID, n.handleRaftRPC)

	go func() { n.runMainLoop(context.Background()) }()
	return nil
}

// Restart runs the main Raft loop for a existing but stopped node.
func (n *Node) Restart(ctx context.Context) error {
	n.logger.Info("restarting node")

	if err := n.backend.Start(ctx); err != nil {
		return errors.Wrap(err, "starting backend")
	}
	if n.isNewCluster {
		return errors.New("cannot restart a new cluster")
	}
	if err := n.loadStoredPeers(ctx); err != nil {
		n.logger.Errorf("failed loading stored peers: %+v", err)
	}
	n.discoverPotentialPeers(ctx, n.potentialPeers)
	applied, err := n.backend.GetAppliedIndex(ctx)
	if err != nil {
		return errors.Wrap(err, "getting applied index for node restart")
	}
	n.config.Applied = applied
	n.raftNode = raft.RestartNode(&n.config)
	n.transportMgr.RegisterDestCallback(n.config.ID, n.handleRaftRPC)

	go func() { n.runMainLoop(ctx) }()
	return nil
}

func (n *Node) runMainLoop(ctx context.Context) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	isDone := false
	for !isDone {
		select {
		case <-ticker.C:
			n.raftNode.Tick()
		case rd := <-n.raftNode.Ready():
			n.saveToStorage(ctx, rd.HardState, rd.Entries)
			snapHandle := n.saveSnapshotIfNotEmpty(ctx, rd.Snapshot)
			n.sendMessages(ctx, rd.Messages)
			n.applySnapshotIfNotEmpty(ctx, rd.Snapshot, snapHandle)
			for _, entry := range rd.CommittedEntries {
				if entry.Type == raftpb.EntryNormal && len(entry.Data) > 0 {
					n.processCommittedData(ctx, entry)
				}
				if entry.Type == raftpb.EntryConfChange {
					var cc raftpb.ConfChange
					cc.Unmarshal(entry.Data)
					err := n.processConfChange(ctx, cc)
					if err == nil {
						state := n.raftNode.ApplyConfChange(cc)
						if err := n.backend.SaveConfState(ctx, state); err != nil {
							n.logger.Errorf("failed saving conf state: %+v", err)
						}
						n.logger.Infof("new Raft state: %#v", state)
					} else {
						n.logger.Errorf("failed processing conf change: %+v", err)
					}
				}
			}
			n.raftNode.Advance()
		case <-n.done:
			isDone = true
		}
	}
	_ = n.peerManager.Close()
	n.transportMgr.UnregisterDestCallback(n.config.ID)
	n.raftNode.Stop()
	n.backend.Stop(ctx)
}

// Stop stops the main Raft loop
func (n *Node) Stop(ctx context.Context) {
	n.done <- true
}

// GetID returns the node ID
func (n *Node) GetID() uint64 {
	return n.config.ID
}

// RaftStatus returns the Raft status
func (n *Node) RaftStatus() raft.Status {
	return n.raftNode.Status()
}

// WriteQuery proposes a write query to the Raft data and synchronuously
// waits for the data to be committed.
func (n *Node) WriteQuery(ctx context.Context, sql string) error {
	chid, ch := n.pendingWrites.GetNewChannel()
	q := pb.WriteQuery{NodeID: n.config.ID, QueryID: uint64(chid), Sql: sql}
	data, err := q.Marshal()
	if err != nil {
		n.pendingWrites.Remove(chid)
		return errors.Wrapf(err, "marshaling sql %s", sql)
	}
	err = n.propose(ctx, data)
	if err != nil {
		n.pendingWrites.Remove(chid)
		return errors.Wrapf(err, "proposing sql %s", sql)
	}

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		n.pendingWrites.Remove(chid)
		return ctx.Err()
	}
}

// ReadQuery runs a read query against the backend. If run on a follower node,
// it may not return the latest data.
func (n *Node) ReadQuery(ctx context.Context, sql string) (*sql.Rows, error) {
	return n.backend.QuerySQL(ctx, sql)
}

// propose proposes Raft data to the cluster.
func (n *Node) propose(ctx context.Context, data []byte) error {
	if n.raftNode == nil {
		return errors.New("node not started yet")
	}
	return n.raftNode.Propose(ctx, data)
}

func (n *Node) sendMessages(ctx context.Context, messages []raftpb.Message) {
	for _, m := range messages {
		c := n.peerManager.ClientForPeer(m.To)
		if c == nil {
			n.logger.Errorf("no peer information for node ID %d", m.To)
			n.raftNode.ReportUnreachable(m.To)
			continue
		}
		m := m
		childCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		err := c.SendRaftMessage(childCtx, &m)
		if err != nil {
			n.logger.Errorf("error sending message: %+v", err)
			n.raftNode.ReportUnreachable(m.To)
		}
		if m.Type == raftpb.MsgSnap {
			status := raft.SnapshotFinish
			if err != nil {
				status = raft.SnapshotFailure
			}
			n.raftNode.ReportSnapshot(m.To, status)
		}
	}
}

func (n *Node) handleRaftRPC(ctx context.Context, m *raftpb.Message) error {
	return n.raftNode.Step(ctx, *m)
}

func (n *Node) saveToStorage(
	ctx context.Context,
	hardState raftpb.HardState,
	entries []raftpb.Entry,
) {
	_ = n.backend.AppendEntries(ctx, entries)
	if !raft.IsEmptyHardState(hardState) {
		_ = n.backend.SaveHardState(ctx, &hardState)
	}
}

func (n *Node) saveSnapshotIfNotEmpty(ctx context.Context, snap raftpb.Snapshot) uint64 {
	if raft.IsEmptySnap(snap) {
		return 0
	}
	snapHandle, err := n.backend.SaveSnapshot(ctx, snap)
	if err != nil {
		n.logger.Errorf("failed saving snapshot: %+v", err)
		panic(err)
	}
	return snapHandle
}

func (n *Node) applySnapshotIfNotEmpty(ctx context.Context, snap raftpb.Snapshot, snapHandle uint64) {
	if raft.IsEmptySnap(snap) {
		return
	}
	n.logger.Info("got snapshot")
	err := n.backend.ApplySnapshot(ctx, snap)
	if err != nil {
		n.logger.Errorf("failed applying snapshot: %+v", err)
		panic(err)
	}
	if snapHandle != 0 {
		err = n.backend.RemoveSavedSnapshot(ctx, snapHandle)
		if err != nil {
			n.logger.Errorf("failed removing saved snapshot: %+v", err)
		}
	}
}

func (n *Node) processConfChange(ctx context.Context, cc raftpb.ConfChange) error {
	if cc.ID == n.config.ID {
		return nil
	}
	if cc.Type == raftpb.ConfChangeRemoveNode {
		n.peerManager.RemovePeer(ctx, cc.ID)
		n.backend.RemovePeer(ctx, cc.ID)
		return nil
	}

	var nodeInfo NodeInfo
	err := json.Unmarshal(cc.Context, &nodeInfo)
	if err != nil {
		return err
	}
	err = n.peerManager.UpsertPeer(ctx, transport.PeerParams{
		NodeID: cc.ID,
		Addr:   nodeInfo.Addr,
		Port:   nodeInfo.Port,
	})
	if err != nil {
		return err
	}
	err = n.backend.UpsertPeer(ctx, backend.PeerInfo{
		NodeID: cc.ID,
		Addr:   nodeInfo.Addr,
		Port:   nodeInfo.Port,
	})
	return err
}

func (n *Node) processCommittedData(ctx context.Context, entry raftpb.Entry) error {
	var query pb.WriteQuery
	if err := query.Unmarshal(entry.Data); err != nil {
		return errors.Wrap(err, "unmarshaling committed data")
	}
	err := n.backend.ExecSQL(ctx, entry.Term, entry.Index, query.Sql)
	if err != nil {
		// An inherent DB error should be treated as executed. Other
		// errors, such as connection errors, mean failure to process.
		if e, ok := err.(*backend.DBError); ok {
			err = e.Cause
			n.logger.Errorf("Actual database error %+v executing committed command: %s", err, query.Sql)
		} else {
			n.logger.Errorf("Execution error %+v executing committed command: %s", err, query.Sql)
			return err
		}
	}
	n.logger.Infof("Incoming data seen by node %d: %s", n.config.ID, query.Sql)
	if query.NodeID == n.config.ID {
		if ch := n.pendingWrites.Remove(query.QueryID); ch != nil {
			n.logger.Infof("Releasing pending write; node %d: %s", n.config.ID, query.Sql)
			ch <- err
		}
	}
	return nil
}

func (n *Node) loadStoredPeers(ctx context.Context) error {
	peers, err := n.backend.LoadPeers(ctx)
	if err != nil {
		return errors.Wrap(err, "loading peers from backend")
	}
	for _, p := range peers {
		err := n.peerManager.UpsertPeer(ctx, transport.PeerParams{
			NodeID: p.NodeID,
			Addr:   p.Addr,
			Port:   p.Port,
		})
		if err != nil {
			return errors.Wrap(err, "upserting peers")
		}
	}
	return nil
}

func (n *Node) discoverPotentialPeers(ctx context.Context, peers []NodeInfo) {
	for _, p := range peers {
		c, err := n.transportMgr.CreateClient(ctx, p.Addr, p.Port)
		if err != nil {
			n.logger.Errorf("failed connecting to potential peer %+v, %+v", p, err)
			continue
		}
		defer c.Close()
		id, err := c.GetRemoteID(ctx)
		if err != nil {
			n.logger.Errorf("failed getting ID from potential peer %+v, %+v", p, err)
			continue
		}
		n.peerManager.UpsertPeer(ctx, transport.PeerParams{
			NodeID: id,
			Addr:   p.Addr,
			Port:   p.Port,
		})
	}
}
