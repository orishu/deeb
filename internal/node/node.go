package node

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	"github.com/orishu/deeb/internal/backend"
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
		done:           make(chan bool),
		peerManager:    peerManager,
		transportMgr:   transportMgr,
		nodeInfo:       params.AddrPort,
		potentialPeers: params.PotentialPeers,
		isNewCluster:   params.IsNewCluster,
		logger:         logger,
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
	// TODO: load peers from the database
	n.discoverPotentialPeers(ctx, n.potentialPeers)
	n.raftNode = raft.StartNode(&n.config, peers)
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
			n.saveToStorage(ctx, rd.HardState, rd.Entries, rd.Snapshot)
			n.sendMessages(ctx, rd.Messages)
			if !raft.IsEmptySnap(rd.Snapshot) {
				////        processSnapshot(rd.Snapshot)
				n.logger.Info("got snapshot")
			}
			for _, entry := range rd.CommittedEntries {
				if entry.Type == raftpb.EntryNormal && len(entry.Data) > 0 {
					n.processCommittedData(ctx, entry.Data)
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

// Propose proposes Raft data to the cluster.
func (n *Node) Propose(ctx context.Context, data []byte) error {
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
	}
}

func (n *Node) handleRaftRPC(ctx context.Context, m *raftpb.Message) error {
	return n.raftNode.Step(ctx, *m)
}

func (n *Node) saveToStorage(
	ctx context.Context,
	hardState raftpb.HardState,
	entries []raftpb.Entry,
	snap raftpb.Snapshot,
) {
	_ = n.backend.AppendEntries(ctx, entries)
	_ = n.backend.SaveHardState(ctx, &hardState)
	if len(snap.Data) > 0 {
		_ = n.backend.ApplySnapshot(ctx, snap)
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
	err = n.backend.UpsertPeer(ctx, cc.ID, nodeInfo.Addr, nodeInfo.Port)
	return err
}

func (n *Node) processCommittedData(ctx context.Context, data []byte) error {
	n.logger.Infof("Incoming data: %s", string(data))
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
