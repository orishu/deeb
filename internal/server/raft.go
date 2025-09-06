package server

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/transport"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.uber.org/zap"
)

type raftService struct {
	pb.UnimplementedRaftServer
	node         *nd.Node
	transportMgr *transport.TransportManager
	logger       *zap.SugaredLogger
}

func (r raftService) Message(ctx context.Context, req *pb.RaftMessage) (*emptypb.Empty, error) {
	// Deserialize the raft message from the protobuf bytes using etcd's unmarshaling
	var msg raftpb.Message
	if err := msg.Unmarshal(req.Data); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal raft message")
	}
	
	// Send the message to the node
	if err := r.node.HandleRaftMessage(ctx, &msg); err != nil {
		return nil, errors.Wrap(err, "failed to handle raft message")
	}
	
	return &emptypb.Empty{}, nil
}

func (r raftService) GetID(ctx context.Context, unused *emptypb.Empty) (*pb.GetIDResponse, error) {
	res := pb.GetIDResponse{Id: r.node.GetID()}
	return &res, nil
}

func (r raftService) HighestID(context.Context, *emptypb.Empty) (*pb.HighestIDResponse, error) {
	status := r.node.RaftStatus()
	var maxID uint64
	
	// Always include this node's ID
	maxID = status.ID
	
	// Include IDs from Progress map (only populated on leader)
	for id, _ := range status.Progress {
		if id > maxID {
			maxID = id
		}
	}
	
	return &pb.HighestIDResponse{Id: maxID}, nil
}

func (r raftService) Progress(context.Context, *emptypb.Empty) (*pb.ProgressResponse, error) {
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
