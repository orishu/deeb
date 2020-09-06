package node

import (
	"context"
	"encoding/json"

	"github.com/coreos/etcd/raft/raftpb"
)

// AddPeerNode adds a new member to the Raft node set.
func (n *Node) AddPeerNode(ctx context.Context, nodeID uint64, addr string, port string) error {
	metadata, err := json.Marshal(nodeMetadata{
		Addr: addr,
		Port: port,
	})
	if err != nil {
		return err
	}
	cc := raftpb.ConfChange{
		ID:      nodeID,
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  n.config.ID,
		Context: metadata,
	}
	return n.raftNode.ProposeConfChange(ctx, cc)
}
