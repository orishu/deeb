package backend

import (
	"context"
	"database/sql"

	"github.com/coreos/etcd/raft/raftpb"
)

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
	ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error
	LoadPeers(ctx context.Context) ([]PeerInfo, error)
	UpsertPeer(ctx context.Context, peerInfo PeerInfo) error
	RemovePeer(ctx context.Context, nodeID uint64) error
	ExecSQL(ctx context.Context, term uint64, idx uint64, sql string) error
	QuerySQL(ctx context.Context, sql string) (*sql.Rows, error)
}
