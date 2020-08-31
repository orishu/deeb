package server

import (
	"context"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
)

type controlService struct {
}

func (c controlService) Status(context.Context, *types.Empty) (*pb.StatusResponse, error) {
	resp := pb.StatusResponse{Code: pb.StatusCode_OK}
	return &resp, nil
}
