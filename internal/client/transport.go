package client

import (
	"context"
	"fmt"
	"sync"

	"github.com/coreos/etcd/raft/raftpb"
)

type Client interface {
	SendRaftMessage(ctx context.Context, msg *raftpb.Message) error
	GetRemoteID(ctx context.Context) (uint64, error)
	Close() error
}

type RaftCallback func(ctx context.Context, m *raftpb.Message) error

type TransportManager interface {
	CreateClient(ctx context.Context, addr string, port string) (Client, error)
	RegisterDestCallback(id uint64, callback RaftCallback)
	DeliverMessage(ctx context.Context, destID uint64, m *raftpb.Message) error
}

type GRPCTransportManager struct {
	destMap map[uint64]RaftCallback
	mutex   sync.RWMutex
}

func NewGRPCTransportManager() TransportManager {
	return &GRPCTransportManager{destMap: make(map[uint64]RaftCallback)}
}

func (tm *GRPCTransportManager) CreateClient(ctx context.Context, addr string, port string) (Client, error) {
	return NewGRPCClient(ctx, addr, port)
}

func (tm *GRPCTransportManager) RegisterDestCallback(id uint64, callback RaftCallback) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.destMap[id] = callback
}

func (tm *GRPCTransportManager) DeliverMessage(ctx context.Context, destID uint64, m *raftpb.Message) error {
	tm.mutex.RLock()
	cb, ok := tm.destMap[destID]
	tm.mutex.RUnlock()
	if ok {
		return cb(ctx, m)
	}
	return fmt.Errorf("destination not found %d", destID)
}
