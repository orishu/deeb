package transport

import (
	"context"
	"net"

	"go.etcd.io/etcd/raft/v3/raftpb"
	"google.golang.org/protobuf/types/known/emptypb"
	pb "github.com/orishu/deeb/api"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
)

// Client abstracts the RPCs to a remote node.
type Client interface {
	SendRaftMessage(ctx context.Context, msg *raftpb.Message) error
	GetRemoteID(ctx context.Context) (uint64, error)
	GetHighestID(ctx context.Context) (uint64, error)
	Progress(ctx context.Context) (*pb.ProgressResponse, error)
	AddNewPeer(ctx context.Context, nodeID uint64, addr string, port string) error
	CheckHealth(ctx context.Context, lagThreshold uint64) error
	Close() error
}

// GRPCClient is a gRPC client for communicating with other nodes
type GRPCClient struct {
	conn       *grpc.ClientConn
	raftClient pb.RaftClient
	ctrlClient pb.ControlServiceClient
}

// NewGRPCClient creates a gRPC client
func NewGRPCClient(ctx context.Context, addr string, port string) (Client, error) {
	/*
		cred := credentials.NewTLS(&tls.Config{
			ServerName: addr,
			//		RootCAs:            insecure.CertPool,
			InsecureSkipVerify: true,
		})
	*/
	conn, err := grpc.DialContext(
		ctx,
		net.JoinHostPort(addr, port),
		// grpc.WithTransportCredentials(cred),
		grpc.WithInsecure(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "grpc dial error %s:%s", addr, port)
	}
	raftClient := pb.NewRaftClient(conn)
	ctrlClient := pb.NewControlServiceClient(conn)
	return &GRPCClient{
		conn:       conn,
		raftClient: raftClient,
		ctrlClient: ctrlClient,
	}, nil
}

// SendRaftMessage sends a Raft message through the Raft client.
func (c *GRPCClient) SendRaftMessage(ctx context.Context, msg *raftpb.Message) error {
	// Serialize the raft message using etcd's marshaling
	data, err := msg.Marshal()
	if err != nil {
		return errors.Wrap(err, "failed to marshal raft message")
	}
	
	// Create the gRPC request
	req := &pb.RaftMessage{Data: data}
	
	// Send the message via gRPC
	_, err = c.raftClient.Message(ctx, req)
	if err != nil {
		return errors.Wrap(err, "failed to send raft message via gRPC")
	}
	
	return nil
}

// GetRemoteID fetches the node ID of the remote node.
func (c *GRPCClient) GetRemoteID(ctx context.Context) (uint64, error) {
	resp, err := c.raftClient.GetID(ctx, &emptypb.Empty{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

// GetHighestID fetches the highest ID recorded by the remote node if the node
// is the leader.
func (c *GRPCClient) GetHighestID(ctx context.Context) (uint64, error) {
	resp, err := c.raftClient.HighestID(ctx, &emptypb.Empty{})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

// Progress fetches the Raft progress state from the remote node.
func (c *GRPCClient) Progress(ctx context.Context) (*pb.ProgressResponse, error) {
	return c.raftClient.Progress(ctx, &emptypb.Empty{})
}

// AddNewPeer tells the remote node about a new joining node
func (c *GRPCClient) AddNewPeer(ctx context.Context, nodeID uint64, addr string, port string) error {
	_, err := c.ctrlClient.AddPeer(ctx, &pb.AddPeerRequest{
		Id:   nodeID,
		Addr: addr,
		Port: port,
	})
	return err
}

// CheckHealth returns error if the node is not keeping up with the leader.
// It's used for the readiness probe.
func (c *GRPCClient) CheckHealth(ctx context.Context, lagThreshold uint64) error {
	req := pb.CheckHealthRequest{LagThreshold: lagThreshold}
	_, err := c.raftClient.CheckHealth(ctx, &req)
	return err
}

// Close closes the gRPC connection
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
