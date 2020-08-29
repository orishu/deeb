package main

import (
	"context"
	"fmt"
	"log"
	"net"

	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/node"
	"github.com/orishu/deeb/internal/storage"
	"google.golang.org/grpc"
)

func main() {
	fmt.Println("Controller")
	storage.Foo()
	n := node.New()
	go func() {
		//		n.Start()
	}()
	startGRPC()
	//	n.Stop()
	_ = n

}

type controlService struct {
}

func (c controlService) Status(context.Context, *pb.StatusRequest) (*pb.StatusResponse, error) {
	resp := pb.StatusResponse{Code: pb.StatusCode_OK}
	return &resp, nil
}

func startGRPC() {
	cservice := controlService{}
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterControlServiceServer(s, &cservice)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
