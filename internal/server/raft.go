package server

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
)

type raftService struct {
	node         *nd.Node
	transportMgr *transport.TransportManager
}

func (r raftService) Message(ctx context.Context, msg *raftpb.Message) (*types.Empty, error) {
	err := r.transportMgr.DeliverMessage(ctx, msg)
	return &types.Empty{}, err
}

func (r raftService) GetID(ctx context.Context, unused *types.Empty) (*pb.GetIDResponse, error) {
	res := pb.GetIDResponse{Id: r.node.GetID()}
	return &res, nil
}

func (r raftService) HighestID(context.Context, *types.Empty) (*pb.HighestIDResponse, error) {
	status := r.node.RaftStatus()
	var maxID uint64
	for id, _ := range status.Progress {
		if id > maxID {
			maxID = id
		}
	}
	return &pb.HighestIDResponse{Id: maxID}, nil
}

func (r raftService) Progress(context.Context, *types.Empty) (*pb.ProgressResponse, error) {
	status := r.node.RaftStatus()
	resp := pb.ProgressResponse{
		Id:          status.ID,
		Applied:     status.Applied,
		State:       status.SoftState.RaftState.String(),
		ProgressMap: make(map[uint64]*pb.NodeProgress),
	}
	for id, progress := range status.Progress {
		p := pb.NodeProgress{Match: progress.Match}
		resp.ProgressMap[id] = &p
	}
	return &resp, nil
}

func (r raftService) CheckHealth(context.Context, *types.Empty) (*pb.CheckHealthResponse, error) {
	return nil, nil
}
