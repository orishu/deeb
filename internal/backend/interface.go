package backend

import (
	"context"

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
	SaveHardState(ctx context.Context, hardState *raftpb.HardState) error
	SaveConfState(ctx context.Context, confState *raftpb.ConfState) error
	ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error
	LoadPeers(ctx context.Context) ([]PeerInfo, error)
	UpsertPeer(ctx context.Context, peerInfo PeerInfo) error
	RemovePeer(ctx context.Context, nodeID uint64) error
}
