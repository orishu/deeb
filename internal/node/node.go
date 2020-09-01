package node

import (
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
)

// Node is the object encapsulating the Raft node.
type Node struct {
	config   raft.Config
	raftNode raft.Node
	storage  *raft.MemoryStorage
	done     chan bool
}

// New creates new single-node RaftCluster
func New() *Node {
	storage := raft.NewMemoryStorage()

	c := raft.Config{
		ID:              0x01,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         storage,
		MaxSizePerMsg:   4096,
		MaxInflightMsgs: 256,
	}

	return &Node{
		config:  c,
		storage: storage,
		done:    make(chan bool),
	}
}

// Start runs the main Raft loop
func (n *Node) Start() {
	peers := []raft.Peer{{ID: n.config.ID}}
	n.raftNode = raft.StartNode(&n.config, peers)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			n.raftNode.Tick()
		case rd := <-n.raftNode.Ready():
			n.saveToStorage(rd.HardState, rd.Entries, rd.Snapshot)
			//      send(rd.Messages)
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
			return
		}
	}
}

// Stop stops the main Raft loop
func (n *Node) Stop() {
	n.done <- true
}

func (n *Node) saveToStorage(hardState raftpb.HardState, entries []raftpb.Entry, snap raftpb.Snapshot) {
	_ = n.storage.Append(entries)
	_ = n.storage.SetHardState(hardState)
	_ = n.storage.ApplySnapshot(snap)
}
