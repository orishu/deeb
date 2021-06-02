package server

import (
	"context"
	"fmt"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type raftService struct {
	node         *nd.Node
	transportMgr *transport.TransportManager
	logger       *zap.SugaredLogger
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

func (r raftService) CheckHealth(ctx context.Context, req *pb.CheckHealthRequest) (*pb.CheckHealthResponse, error) {
	status := r.node.RaftStatus()
	resp := pb.CheckHealthResponse{}
	if status.SoftState.RaftState.String() == "StateLeader" {
		resp.IsLeader = true
		return &resp, nil
	}
	peerClients := r.node.GetPeerClients()
	limitedCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	responseChan := make(chan *pb.ProgressResponse)
	for _, peerClient := range peerClients {
		client := peerClient
		go func() {
			progress, err := client.Progress(limitedCtx)
			if err != nil || progress == nil {
				r.logger.Warn("failed checking remote node progress, err: %+v", err)
				responseChan <- nil
				return
			}
			responseChan <- progress
		}()
	}
	var leaderResponse *pb.ProgressResponse
	for i := 0; i < len(peerClients); i++ {
		if progress := <-responseChan; progress != nil && progress.State == "StateLeader" {
			leaderResponse = progress
			break
		}
	}
	if leaderResponse == nil {
		return nil, errors.New("did not received a Progress() response from a leader node")
	}
	status = r.node.RaftStatus()
	if leaderResponse.Applied > status.Applied+req.LagThreshold {
		return nil, fmt.Errorf("node is behind leader by %d", leaderResponse.Applied-status.Applied)
	}
	return &resp, nil
}
