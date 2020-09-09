package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/etcd/raft/raftpb"
)

// Client abstracts the RPCs to a remote node.
type Client interface {
	SendRaftMessage(ctx context.Context, msg *raftpb.Message) error
	GetRemoteID(ctx context.Context) (uint64, error)
	Close() error
}

// RaftCallback is the callback type for incoming Raft messages.
type RaftCallback func(ctx context.Context, m *raftpb.Message) error

// TransportManager is the interface abstracting client creation and delivering
// messages to registered nodes.
type TransportManager interface {
	CreateClient(ctx context.Context, addr string, port string) (Client, error)
	RegisterDestCallback(id uint64, callback RaftCallback)
	DeliverMessage(ctx context.Context, destID uint64, m *raftpb.Message) error
}

// GRPCTransportManager is the gRPC implementation of a transport manager
type GRPCTransportManager struct {
	destMap map[uint64]RaftCallback
	mutex   sync.RWMutex
}

// NewGRPCTransportManager creates a new gRPC TransportManager
func NewGRPCTransportManager() TransportManager {
	return &GRPCTransportManager{destMap: make(map[uint64]RaftCallback)}
}

// CreateClient creates a new client to a remote node.
func (tm *GRPCTransportManager) CreateClient(ctx context.Context, addr string, port string) (Client, error) {
	return NewGRPCClient(ctx, addr, port)
}

// RegisterDestCallback registers a callback with a node ID for delivering
// incoming messages.
func (tm *GRPCTransportManager) RegisterDestCallback(id uint64, callback RaftCallback) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.destMap[id] = callback
}

// DeliverMessage processes incoming message by calling the registered
// destination's callback.
func (tm *GRPCTransportManager) DeliverMessage(ctx context.Context, destID uint64, m *raftpb.Message) error {
	tm.mutex.RLock()
	cb, ok := tm.destMap[destID]
	tm.mutex.RUnlock()
	if ok {
		return cb(ctx, m)
	}
	return fmt.Errorf("destination not found %d", destID)
}
