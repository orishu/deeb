package server

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/client"
)

type raftService struct {
	nodeID       uint64
	transportMgr client.TransportManager
}

func (r raftService) Message(ctx context.Context, msg *raftpb.Message) (*types.Empty, error) {
	err := r.transportMgr.DeliverMessage(ctx, r.nodeID, msg)
	return &types.Empty{}, err
}

func (r raftService) GetID(ctx context.Context, unused *types.Empty) (*pb.GetIDResponse, error) {
	res := pb.GetIDResponse{Id: r.nodeID}
	return &res, nil
}
