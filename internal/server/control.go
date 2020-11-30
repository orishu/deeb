package server

import (
	"context"

	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
)

type controlService struct {
	node *nd.Node
}

func (c controlService) Status(context.Context, *types.Empty) (*pb.StatusResponse, error) {
	resp := pb.StatusResponse{Code: pb.StatusCode_OK}
	return &resp, nil
}

func (c controlService) AddPeer(ctx context.Context, req *pb.AddPeerRequest) (*types.Empty, error) {
	err := c.node.AddNode(ctx, req.Id, nd.NodeInfo{Addr: req.Addr, Port: req.Port})
	return &types.Empty{}, err
}

func (c controlService) QuerySQL(*pb.QuerySQLRequest, pb.ControlService_QuerySQLServer) error {
	return nil
}
