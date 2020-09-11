package server

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/transport"
)

type raftService struct {
	nodeID       uint64
	transportMgr *transport.TransportManager
}

func (r raftService) Message(ctx context.Context, msg *raftpb.Message) (*types.Empty, error) {
	err := r.transportMgr.DeliverMessage(ctx, msg)
	return &types.Empty{}, err
}

func (r raftService) GetID(ctx context.Context, unused *types.Empty) (*pb.GetIDResponse, error) {
	res := pb.GetIDResponse{Id: r.nodeID}
	return &res, nil
}
