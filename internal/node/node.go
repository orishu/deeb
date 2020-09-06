package node

import (
	"context"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	"github.com/orishu/deeb/internal/client"
)

// Node is the object encapsulating the Raft node.
type Node struct {
	config      raft.Config
	raftNode    raft.Node
	storage     *raft.MemoryStorage
	done        chan bool
	peerManager *client.PeerManager
}

type nodeMetadata struct {
	Addr string `json:"addr"`
	Port string `json:"port"`
}

// New creates new single-node RaftCluster
func New(nodeID uint64) *Node {
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
	}
}

// Start runs the main Raft loop
func (n *Node) Start(ctx context.Context, newCluster bool) {
	peers := []raft.Peer{}
	if newCluster {
		peers = append(peers, raft.Peer{ID: n.config.ID})
	}
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
			}
			for _, entry := range rd.CommittedEntries {
				//        process(entry)
				if entry.Type == raftpb.EntryConfChange {
					var cc raftpb.ConfChange
					cc.Unmarshal(entry.Data)
					n.raftNode.ApplyConfChange(cc)
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

func (n *Node) sendMessages(ctx context.Context, messages []raftpb.Message) {
	for _, m := range messages {
		c := n.peerManager.ClientForPeer(m.To)
		if c == nil {
			continue
		}
		m := m
		_, err := c.RaftClient.Message(ctx, &m)
		if err != nil {
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
