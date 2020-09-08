package backend

import (
	"context"

	"github.com/coreos/etcd/raft/raftpb"
)

type DBBackend interface {
	AppendEntries(ctx context.Context, entries []raftpb.Entry) error
	SaveHardState(ctx context.Context, hardState raftpb.HardState) error
	ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error
	QueryEntries(ctx context.Context, lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error)
	QueryEntryTerm(ctx context.Context, i uint64) (uint64, error)
	QueryLastIndex(ctx context.Context) (uint64, error)
	QueryFirstIndex(ctx context.Context) (uint64, error)
}
