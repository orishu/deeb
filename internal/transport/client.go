package transport

import (
	"context"
	"net"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/gogo/protobuf/types"
	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/insecure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// GRPCClient is a gRPC client for communicating with other nodes
type GRPCClient struct {
	conn       *grpc.ClientConn
	raftClient pb.RaftClient
}

// NewGRPCClient creates a gRPC client
func NewGRPCClient(ctx context.Context, addr string, port string) (*GRPCClient, error) {
	conn, err := grpc.DialContext(ctx, net.JoinHostPort(addr, port),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "grpc dial error %s:%s", addr, port)
	}
	raftClient := pb.NewRaftClient(conn)
	return &GRPCClient{conn: conn, raftClient: raftClient}, nil
}

func (c *GRPCClient) SendRaftMessage(ctx context.Context, msg *raftpb.Message) error {
	_, err := c.raftClient.Message(ctx, msg)
	return err
}

func (c *GRPCClient) GetRemoteID(ctx context.Context) (uint64, error) {
	resp, err := c.raftClient.GetID(ctx, &types.Empty{})
	return resp.Id, err
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
