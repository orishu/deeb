package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/etcd/raft/raftpb"
)

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

// UnregisterDestCallback unregisters a node ID from delivering incoming messages.
func (tm *TransportManager) UnregisterDestCallback(id uint64) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	delete(tm.destMap, id)
}

// DeliverMessage processes incoming message by calling the registered
// destination's callback.
func (tm *TransportManager) DeliverMessage(ctx context.Context, m *raftpb.Message) error {
	tm.mutex.RLock()
	cb, ok := tm.destMap[m.To]
	tm.mutex.RUnlock()
	if ok {
		return cb(ctx, m)
	}
	return fmt.Errorf("destination not found %d", m.To)
}
