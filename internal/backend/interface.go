package backend

import (
	"context"
	"database/sql"
	"fmt"

	"go.etcd.io/etcd/raft/v3/raftpb"
)

// DBError is an error type that encapsulates an actual database error, in
// contrast to a connection error. We need to distinguish those two types as we
// process DB writes that are inherently incorrect and would always fail.
type DBError struct {
	Cause error
}

func (e *DBError) Error() string {
	return fmt.Sprintf("DB error: %+v", e.Cause)
}

type PeerInfo struct {
	NodeID uint64
	Addr   string
	Port   string
}

type DBBackend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context)
	AppendEntries(ctx context.Context, entries []raftpb.Entry) error
	SaveApplied(ctx context.Context, Term uint64, Index uint64) error
	GetAppliedIndex(ctx context.Context) (uint64, error)
	SaveHardState(ctx context.Context, hardState *raftpb.HardState) error
	SaveConfState(ctx context.Context, confState *raftpb.ConfState) error
	SaveSnapshot(ctx context.Context, snap raftpb.Snapshot) (uint64, error)
	RemoveSavedSnapshot(ctx context.Context, snapHandle uint64) error
	ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error
	LoadPeers(ctx context.Context) ([]PeerInfo, error)
	UpsertPeer(ctx context.Context, peerInfo PeerInfo) (uint64, error)
	RemovePeer(ctx context.Context, nodeID uint64) error
	ExecSQL(ctx context.Context, term uint64, idx uint64, sql string) error
	QuerySQL(ctx context.Context, sql string) (*sql.Rows, error)
}
