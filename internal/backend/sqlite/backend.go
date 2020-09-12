package sqlite

import (
	"context"
	"database/sql"

	"github.com/coreos/etcd/raft/raftpb"
	_ "github.com/mattn/go-sqlite3"
	"github.com/orishu/deeb/internal/backend"
	"go.uber.org/zap"
)

func New(dbFilename string, logger *zap.SugaredLogger) (backend.DBBackend, error) {
	db, err := sql.Open("sqlite3", dbFilename)
	if err != nil {
		return nil, err
	}

	return &Backend{
		db:     db,
		logger: logger,
	}, nil
}

// Backend is the sqlite backend
type Backend struct {
	db     *sql.DB
	logger *zap.SugaredLogger
}

func (b *Backend) Start(ctx context.Context) error {
	return nil
}

func (b *Backend) Stop(ctx context.Context) {
	if b.db != nil {
		b.db.Close()
	}
}

func (b *Backend) AppendEntries(ctx context.Context, entries []raftpb.Entry) error {
	return nil
}

func (b *Backend) SaveHardState(ctx context.Context, hardState raftpb.HardState) error {
	return nil
}

func (b *Backend) ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error {
	return nil
}

func (b *Backend) QueryEntries(ctx context.Context, lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error) {
	return []raftpb.Entry{}, nil
}

func (b *Backend) QueryEntryTerm(ctx context.Context, i uint64) (uint64, error) {
	return 0, nil
}

func (b *Backend) QueryLastIndex(ctx context.Context) (uint64, error) {
	return 0, nil
}

func (b *Backend) QueryFirstIndex(ctx context.Context) (uint64, error) {
	return 0, nil
}
