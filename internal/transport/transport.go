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

// ClientFactory is the type of a function that creates an RPC client
type ClientFactory func(ctx context.Context, addr string, port string) (Client, error)

// TransportManager is the gRPC implementation of a transport manager
type TransportManager struct {
	clientFactory ClientFactory
	destMap       map[uint64]RaftCallback
	mutex         sync.RWMutex
}

// NewTransportManager creates a new gRPC TransportManager
func NewTransportManager(clientFactory ClientFactory) *TransportManager {
	return &TransportManager{clientFactory: clientFactory, destMap: make(map[uint64]RaftCallback)}
}

// CreateClient creates a new client to a remote node.
func (tm *TransportManager) CreateClient(ctx context.Context, addr string, port string) (Client, error) {
	return tm.clientFactory(ctx, addr, port)
}

// RegisterDestCallback registers a callback with a node ID for delivering
// incoming messages.
func (tm *TransportManager) RegisterDestCallback(id uint64, callback RaftCallback) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.destMap[id] = callback
}

// DeliverMessage processes incoming message by calling the registered
// destination's callback.
func (tm *TransportManager) DeliverMessage(ctx context.Context, destID uint64, m *raftpb.Message) error {
	tm.mutex.RLock()
	cb, ok := tm.destMap[destID]
	tm.mutex.RUnlock()
	if ok {
		return cb(ctx, m)
	}
	return fmt.Errorf("destination not found %d", destID)
}
