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

// Client abstracts the RPCs to a remote node.
type Client interface {
	SendRaftMessage(ctx context.Context, msg *raftpb.Message) error
	GetRemoteID(ctx context.Context) (uint64, error)
	GetHighestID(ctx context.Context) (uint64, error)
	Close() error
}

// GRPCClient is a gRPC client for communicating with other nodes
type GRPCClient struct {
	conn       *grpc.ClientConn
	raftClient pb.RaftClient
}

// NewGRPCClient creates a gRPC client
func NewGRPCClient(ctx context.Context, addr string, port string) (Client, error) {
	conn, err := grpc.DialContext(ctx, net.JoinHostPort(addr, port),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "grpc dial error %s:%s", addr, port)
	}
	raftClient := pb.NewRaftClient(conn)
	return &GRPCClient{conn: conn, raftClient: raftClient}, nil
}

// SendRaftMessage sends a Raft message through the Raft client.
func (c *GRPCClient) SendRaftMessage(ctx context.Context, msg *raftpb.Message) error {
	_, err := c.raftClient.Message(ctx, msg)
	return err
}

// GetRemoteID fetches the node ID of the remote node.
func (c *GRPCClient) GetRemoteID(ctx context.Context) (uint64, error) {
	resp, err := c.raftClient.GetID(ctx, &types.Empty{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

// GetHighestID fetches the highest ID recorded by the remote node if the node
// is the leader.
func (c *GRPCClient) GetHighestID(ctx context.Context) (uint64, error) {
	resp, err := c.raftClient.HighestID(ctx, &types.Empty{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
