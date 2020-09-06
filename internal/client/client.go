package client

import (
	"context"
	"io/ioutil"
	"net"
	"os"

	pb "github.com/orishu/deeb/api"
	"github.com/orishu/deeb/internal/insecure"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/grpclog"
)

var log grpclog.LoggerV2

func init() {
	log = grpclog.NewLoggerV2(os.Stdout, ioutil.Discard, ioutil.Discard)
	grpclog.SetLoggerV2(log)
}

// Client is a gRPC client for communicating with other nodes
type Client struct {
	conn       *grpc.ClientConn
	RaftClient pb.RaftClient
}

// newClient creates a gRPC client
func newClient(ctx context.Context, addr, port string) (*Client, error) {
	conn, err := grpc.DialContext(ctx, net.JoinHostPort(addr, port),
		grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(insecure.CertPool, "")),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "grpc dial error %s:%s", addr, port)
	}
	raftClient := pb.NewRaftClient(conn)
	return &Client{conn: conn, RaftClient: raftClient}, nil
}

// Close closes the gRPC connection
func (c *Client) Close() error {
	return c.conn.Close()
}
