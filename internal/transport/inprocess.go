package transport

import (
	"context"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
)

type InProcessRegistry struct {
	transportMgr     *TransportManager
	addrportToNodeID map[string]uint64
}

func (ipr *InProcessRegistry) RegisterAddrPortNodeID(addr string, port string, nodeID uint64) {
	ipr.addrportToNodeID[fmt.Sprintf("%s:%s", addr, port)] = nodeID
}

type InProcessClient struct {
	destAddrport string
	registry     *InProcessRegistry
}

func NewInProcessRegistry(transportMgr *TransportManager) *InProcessRegistry {
	return &InProcessRegistry{transportMgr: transportMgr}
}

func NewInProcessClientFactory(registry *InProcessRegistry) ClientFactory {
	return func(ctx context.Context, addr string, port string) (Client, error) {
		return &InProcessClient{
			destAddrport: fmt.Sprintf("%s:%s", addr, port),
			registry:     registry,
		}, nil
	}
}

func (ic *InProcessClient) SendRaftMessage(ctx context.Context, msg *raftpb.Message) error {
	return ic.registry.transportMgr.DeliverMessage(ctx, msg)
}

func (ic *InProcessClient) GetRemoteID(ctx context.Context) (uint64, error) {
	nodeID, ok := ic.registry.addrportToNodeID[ic.destAddrport]
	if !ok {
		return 0, fmt.Errorf("no registered node ID for destination %s", ic.destAddrport)
	}
	return nodeID, nil
}

func (ic *InProcessClient) Close() error {
	return nil
}
