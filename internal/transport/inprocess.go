package transport

import (
	"context"
	"errors"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
	pb "github.com/orishu/deeb/api"
)

// InProcessRegistry is used for testing, to emulate inter-node transport
// running in the same process.
type InProcessRegistry struct {
	transportMgrs    map[uint64]*TransportManager
	addrportToNodeID map[string]uint64
}

// Register registers a node in the fake in-process transport layer so it will be
// routable.
func (ipr *InProcessRegistry) Register(tm *TransportManager, addr string, port string, nodeID uint64) {
	ipr.addrportToNodeID[fmt.Sprintf("%s:%s", addr, port)] = nodeID
	ipr.transportMgrs[nodeID] = tm
}

// NewInProcessRegistry creates a registry for emulated in-process transport.
func NewInProcessRegistry() *InProcessRegistry {
	return &InProcessRegistry{
		transportMgrs:    make(map[uint64]*TransportManager),
		addrportToNodeID: make(map[string]uint64),
	}
}

// NewInProcessClientFactory returns a function that creates an
// InProcessClient.
func NewInProcessClientFactory(registry *InProcessRegistry) ClientFactory {
	return func(ctx context.Context, addr string, port string) (Client, error) {
		return &InProcessClient{
			destAddrport: fmt.Sprintf("%s:%s", addr, port),
			registry:     registry,
		}, nil
	}
}

// InProcessClient implements the Client interface for in-process transport.
type InProcessClient struct {
	destAddrport string
	registry     *InProcessRegistry
}

// SendRaftMessage is part of the Client implementation.
func (ic *InProcessClient) SendRaftMessage(ctx context.Context, msg *raftpb.Message) error {
	return ic.registry.transportMgrs[msg.To].DeliverMessage(ctx, msg)
}

// GetRemoteID is part of the Client implementation.
func (ic *InProcessClient) GetRemoteID(ctx context.Context) (uint64, error) {
	nodeID, ok := ic.registry.addrportToNodeID[ic.destAddrport]
	if !ok {
		return 0, fmt.Errorf("no registered node ID for destination %s", ic.destAddrport)
	}
	return nodeID, nil
}

// GetHighestID is part of the Client implementation.
func (ic *InProcessClient) GetHighestID(ctx context.Context) (uint64, error) {
	var maxID uint64
	for _, nodeID := range ic.registry.addrportToNodeID {
		if nodeID > maxID {
			maxID = nodeID
		}
	}
	return maxID, nil
}

// Progress fetches the Raft progress state from the remote node.
func (ic *InProcessClient) Progress(ctx context.Context) (*pb.ProgressResponse, error) {
	return nil, errors.New("not implemented")
}

// AddNewPeer tells the remote node about a new joining node
func (ic *InProcessClient) AddNewPeer(ctx context.Context, nodeID uint64, addr string, port string) error {
	ic.registry.addrportToNodeID[fmt.Sprintf("%s:%s", addr, port)] = nodeID
	return nil
}

// CheckHealth returns error if the node is not keeping up with the leader.
// It's used for the readiness probe.
func (ic *InProcessClient) CheckHealth(context.Context, uint64) error {
	return errors.New("not implemented")
}

// Close is part of the Client implementation.
func (ic *InProcessClient) Close() error {
	return nil
}
