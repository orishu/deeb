package server

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	nd "github.com/orishu/deeb/internal/node"
)

type raftService struct {
	node *nd.Node
}

func (r raftService) Message(ctx context.Context, msg *raftpb.Message) (*types.Empty, error) {
	r.node.HandleRaftRPC(ctx, *msg)
	return nil, nil
}
