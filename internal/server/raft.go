package server

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
)

type raftService struct {
	node *nd.Node
}

func (r raftService) Message(ctx context.Context, msg *raftpb.Message) (*types.Empty, error) {
	r.node.HandleRaftRPC(ctx, *msg)
	return &types.Empty{}, nil
}

func (r raftService) GetID(ctx context.Context, unused *types.Empty) (*pb.GetIDResponse, error) {
	res := pb.GetIDResponse{Id: r.node.GetID()}
	return &res, nil
}
