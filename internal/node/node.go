package node

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	"github.com/gogo/protobuf/types"
	"github.com/orishu/deeb/internal/client"
)

var logger *log.Logger

func init() {
	logger = log.New(os.Stdout, "[node] ", 0)
}

// Node is the object encapsulating the Raft node.
type Node struct {
	config      raft.Config
	raftNode    raft.Node
	storage     *raft.MemoryStorage
	done        chan bool
	peerManager *client.PeerManager
	nodeInfo    NodeInfo
}

// NodeInfo groups the node's metadata outside of its Raft configuration
type NodeInfo struct {
	Addr string `json:"addr"`
	Port string `json:"port"`
}

// New creates new single-node RaftCluster
func New(nodeID uint64, nodeInfo NodeInfo) *Node {
	storage := raft.NewMemoryStorage()

	c := raft.Config{
		ID:              nodeID,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	return &Node{
		config:      c,
		storage:     storage,
		done:        make(chan bool),
		peerManager: client.NewPeerManager(),
		nodeInfo:    nodeInfo,
	}
}

// Start runs the main Raft loop
func (n *Node) Start(ctx context.Context, newCluster bool, potentialPeers []NodeInfo) {
	logger.Printf("starting node")
	var peers []raft.Peer
	if newCluster {
		b, err := json.Marshal(n.nodeInfo)
		if err != nil {
			panic(err)
		}
		peers = []raft.Peer{{ID: n.config.ID, Context: b}}
	}
	n.discoverPotentialPeers(ctx, potentialPeers)
	n.raftNode = raft.StartNode(&n.config, peers)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	isDone := false
	for !isDone {
		select {
		case <-ticker.C:
			n.raftNode.Tick()
		case rd := <-n.raftNode.Ready():
			n.saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)
			n.sendMessages(ctx, rd.Messages)
			if !raft.IsEmptySnap(rd.Snapshot) {
				////        processSnapshot(rd.Snapshot)
				logger.Printf("got snapshot")
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
						logger.Printf("new Raft state: %#v", state)
					} else {
						logger.Printf("failed processing conf change: %+v", err)
					}
				}
			}
			n.raftNode.Advance()
		case <-n.done:
			isDone = true
		}
	}
	_ = n.peerManager.Close()
}

// Stop stops the main Raft loop
func (n *Node) Stop() {
	n.done <- true
}

func (n *Node) GetID() uint64 {
	return n.config.ID
}

func (n *Node) sendMessages(ctx context.Context, messages []raftpb.Message) {
	for _, m := range messages {
		c := n.peerManager.ClientForPeer(m.To)
		if c == nil {
			logger.Printf("no peer information for node ID %d", m.To)
			n.raftNode.ReportUnreachable(m.To)
			continue
		}
		m := m
		childCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, err := c.RaftClient.Message(childCtx, &m)
		if err != nil {
			logger.Printf("error sending message: %+v", err)
			n.raftNode.ReportUnreachable(m.To)
		}
	}
}

func (n *Node) HandleRaftRPC(ctx context.Context, m raftpb.Message) {
	_ = n.raftNode.Step(ctx, m)
}

func (n *Node) saveToStorage(hardState raftpb.HardState, entries []raftpb.Entry, snap raftpb.Snapshot) {
	_ = n.storage.Append(entries)
	_ = n.storage.SetHardState(hardState)
	_ = n.storage.ApplySnapshot(snap)
}

func (n *Node) processConfChange(ctx context.Context, cc raftpb.ConfChange) error {
	if cc.ID == n.config.ID {
		return nil
	}
	if cc.Type == raftpb.ConfChangeRemoveNode {
		n.peerManager.RemovePeer(ctx, cc.ID)
		return nil
	}

	var nodeInfo NodeInfo
	err := json.Unmarshal(cc.Context, &nodeInfo)
	if err != nil {
		return err
	}
	err = n.peerManager.UpsertPeer(ctx, client.PeerParams{
		NodeID: cc.ID,
		Addr:   nodeInfo.Addr,
		Port:   nodeInfo.Port,
	})
	return err
}

func (n *Node) processCommittedData(ctx context.Context, data []byte) error {
	logger.Printf("Incoming data: %s", string(data))
	return nil
}

func (n *Node) discoverPotentialPeers(ctx context.Context, peers []NodeInfo) {
	for _, p := range peers {
		c, err := client.NewClient(ctx, p.Addr, p.Port)
		if err != nil {
			logger.Printf("failed connecting to potential peer %+v, %+v", p, err)
			continue
		}
		defer c.Close()
		id, err := c.RaftClient.GetID(ctx, &types.Empty{})
		if err != nil {
			logger.Printf("failed getting ID from potential peer %+v, %+v", p, err)
			continue
		}
		n.peerManager.UpsertPeer(ctx, client.PeerParams{
			NodeID: id.Id,
			Addr:   p.Addr,
			Port:   p.Port,
		})
	}
}
