package backend

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
)

type DBBackend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context)
	AppendEntries(ctx context.Context, entries []raftpb.Entry) error
	SaveHardState(ctx context.Context, hardState *raftpb.HardState) error
	SaveConfState(ctx context.Context, confState *raftpb.ConfState) error
	ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error
	UpsertPeer(ctx context.Context, nodeID uint64, addr string, port string) error
	RemovePeer(ctx context.Context, nodeID uint64) error
}
