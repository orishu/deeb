package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
	_ "github.com/mattn/go-sqlite3"
	"github.com/orishu/deeb/internal/backend"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func New(dbPath string, logger *zap.SugaredLogger) backend.DBBackend {
	return &Backend{
		dbPath:     fmt.Sprintf("%s/db.sqlite", dbPath),
		raftDBPath: fmt.Sprintf("%s/raft.sqlite", dbPath),
		logger:     logger,
	}
}

// Backend is the sqlite backend
type Backend struct {
	dbPath     string
	raftDBPath string
	db         *sql.DB
	raftdb     *sql.DB
	logger     *zap.SugaredLogger
}

func (b *Backend) Start(ctx context.Context) error {
	db, err := sql.Open("sqlite3", b.dbPath)
	if err != nil {
		return errors.Wrapf(err, "opening db at %s", b.dbPath)
	}
	b.db = db
	raftdb, err := sql.Open("sqlite3", b.raftDBPath)
	if err != nil {
		return err
		return errors.Wrapf(err, "opening raft db at %s", b.raftDBPath)
	}
	b.raftdb = raftdb

	query := `CREATE TABLE IF NOT EXISTS hard_state (
			term INTEGER NOT NULL,
			vote INTEGER NOT NULL,
			commitidx INTEGER NOT NULL)`
	_, err = b.raftdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating hard_state table")
	}

	query = `CREATE TABLE IF NOT EXISTS entries (
			idx INTEGER PRIMARY KEY,
			term INTEGER NOT NULL,
			type INTEGER NOT NULL,
			data BLOB NOT NULL) WITHOUT ROWID`
	_, err = b.raftdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating entries table")
	}
	return nil
}

func (b *Backend) Stop(ctx context.Context) {
	if b.raftdb != nil {
		b.raftdb.Close()
	}
	if b.db != nil {
		b.db.Close()
	}
}

func (b *Backend) AppendEntries(ctx context.Context, entries []raftpb.Entry) error {
	for _, entry := range entries {
		query := `INSERT INTO entries
				(idx, term, type, data)
				VALUES (?,?,?,?)`
		res, err := b.raftdb.ExecContext(
			ctx, query, entry.Index, entry.Term, entry.Type, entry.Data)
		if err != nil {
			return errors.Wrap(err, "appending raft entries")
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "querying rows affected")
		}
		if rows != 1 {
			return fmt.Errorf("unexpected number of rows affected: %d", rows)
		}
	}
	return nil
}

func (b *Backend) SaveHardState(ctx context.Context, hardState raftpb.HardState) error {
	query := `REPLACE INTO hard_state
			(rowid, term, vote, commitidx)
			VALUES (?,?,?,?)`
	_, err := b.raftdb.ExecContext(
		ctx, query, 1, hardState.Term, hardState.Vote, hardState.Commit)
	if err != nil {
		return errors.Wrap(err, "saving hard state")
	}
	return nil
}

func (b *Backend) ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error {
	return nil
}

func (b *Backend) QueryEntries(ctx context.Context, lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error) {
	query := `SELECT idx, term, type, data FROM entries
			WHERE idx >= ? AND idx < ? LIMIT ?`
	result := make([]raftpb.Entry, 0, maxSize)
	rows, err := b.raftdb.QueryContext(ctx, query, lo, hi, maxSize)
	if err != nil {
		return nil, errors.Wrapf(err, "querying entries [%d,%d) limit %d", lo, hi, maxSize)
	}
	defer rows.Close()
	for rows.Next() {
		entry := raftpb.Entry{}
		err = rows.Scan(&entry.Index, &entry.Term, &entry.Type, &entry.Data)
		if err != nil {
			return nil, errors.Wrap(err, "reading rows")
		}
		result = append(result, entry)
	}
	return result, nil
}

func (b *Backend) QueryEntryTerm(ctx context.Context, i uint64) (uint64, error) {
	query := `SELECT term FROM entries WHERE idx = ?`
	return queryInteger(ctx, b.raftdb, query, i)
}

func (b *Backend) QueryLastIndex(ctx context.Context) (uint64, error) {
	query := `SELECT max(idx) FROM entries`
	return queryInteger(ctx, b.raftdb, query)
}

func (b *Backend) QueryFirstIndex(ctx context.Context) (uint64, error) {
	query := `SELECT min(idx) FROM entries`
	return queryInteger(ctx, b.raftdb, query)
}

func queryInteger(ctx context.Context, db *sql.DB, query string, args ...interface{}) (uint64, error) {
	row := db.QueryRowContext(ctx, query, args...)
	var result uint64
	err := row.Scan(&result)
	if err != nil {
		return 0, errors.Wrapf(err, "query: %s %+v", query, args)
	}
	return result, nil
}
