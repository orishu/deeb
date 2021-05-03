package node

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
)

// AddNode adds a new member to the Raft node set.
func (n *Node) AddNode(ctx context.Context, nodeID uint64, nodeInfo NodeInfo) error {
	if n.raftNode == nil {
		return fmt.Errorf("node %d not started yet", nodeID)
	}
	nodeInfoBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return err
	}
	cc := raftpb.ConfChange{
		ID:      nodeID,
		Type:    raftpb.ConfChangeAddNode,
		NodeID:  nodeID,
		Context: nodeInfoBytes,
	}
	return n.raftNode.ProposeConfChange(ctx, cc)
}

// UpdateNode updates an existing member.
func (n *Node) UpdateNode(ctx context.Context, nodeID uint64, nodeInfo NodeInfo) error {
	nodeInfoBytes, err := json.Marshal(nodeInfo)
	if err != nil {
		return err
	}
	cc := raftpb.ConfChange{
		ID:      nodeID,
		Type:    raftpb.ConfChangeUpdateNode,
		NodeID:  nodeID,
		Context: nodeInfoBytes,
	}
	return n.raftNode.ProposeConfChange(ctx, cc)
}

// RemoveNode removes an existing member.
func (n *Node) RemoveNode(ctx context.Context, nodeID uint64) error {
	cc := raftpb.ConfChange{
		ID:     nodeID,
		Type:   raftpb.ConfChangeRemoveNode,
		NodeID: nodeID,
	}
	return n.raftNode.ProposeConfChange(ctx, cc)
}
