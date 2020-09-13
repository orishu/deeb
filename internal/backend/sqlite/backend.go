package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/golang/protobuf/proto"
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

	query := `CREATE TABLE IF NOT EXISTS state (
			idx INTEGER NOT NULL,
			term INTEGER NOT NULL,
			hardstate_term INTEGER NOT NULL,
			hardstate_vote INTEGER NOT NULL,
			hardstate_commit INTEGER NOT NULL,
			confstate BLOB)`
	_, err = b.raftdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating state table")
	}
	query = "SELECT count(*) FROM state"
	count, err := queryInteger(ctx, b.raftdb, query)
	if err != nil {
		return errors.Wrap(err, "querying state table")
	}
	if count == 0 {
		query = `INSERT INTO state
			(rowid, idx, term, hardstate_term, hardstate_vote, hardstate_commit)
			VALUES (1, 0, 0, 0, 0, 0)`
		_, err = b.raftdb.ExecContext(ctx, query)
		if err != nil {
			return errors.Wrap(err, "creating first row of state table")
		}
	}

	query = `CREATE TABLE IF NOT EXISTS peers (
			nodeid INTEGER PRIMARY KEY,
			address TEXT NOT NULL,
			port TEXT NOT NULL)`
	_, err = b.raftdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating peers table")
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

func (b *Backend) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	ctx := context.Background()
	query := `SELECT hardstate_term, hardstate_vote, hardstate_commit, confstate
			FROM state WHERE rowid = 1`
	row := b.raftdb.QueryRowContext(ctx, query)
	hard := raftpb.HardState{}
	var confBytes []byte
	err := row.Scan(&hard.Term, &hard.Vote, &hard.Commit, &confBytes)
	if err != nil {
		return raftpb.HardState{}, raftpb.ConfState{}, errors.Wrapf(err, "query: %s", query)
	}
	conf := raftpb.ConfState{}
	if len(confBytes) > 0 {
		err = proto.Unmarshal(confBytes, &conf)
		if err != nil {
			return hard, conf, errors.New("unmarshalling conf state")
		}
	}
	return hard, conf, nil
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

func (b *Backend) SaveHardState(ctx context.Context, hardState *raftpb.HardState) error {
	query := `UPDATE state SET
			hardstate_term = ?,
			hardstate_vote = ?,
			hardstate_commit = ?
			WHERE rowid = 1`
	_, err := b.raftdb.ExecContext(
		ctx, query, hardState.Term, hardState.Vote, hardState.Commit)
	if err != nil {
		return errors.Wrap(err, "saving hard state")
	}
	return nil
}

func (b *Backend) SaveConfState(ctx context.Context, confState *raftpb.ConfState) error {
	query := `UPDATE state SET confstate = ? WHERE rowid = 1`
	d, err := proto.Marshal(confState)
	if err != nil {
		return errors.Wrap(err, "marshalling conf state")
	}
	_, err = b.raftdb.ExecContext(ctx, query, d)
	if err != nil {
		return errors.Wrap(err, "saving conf state")
	}
	return nil
}

func (b *Backend) UpsertPeer(ctx context.Context, nodeID uint64, addr string, port string) error {
	query := `REPLACE INTO peers (nodeid, address, port) VALUES (?,?,?)`
	_, err := b.raftdb.ExecContext(ctx, query, nodeID, addr, port)
	if err != nil {
		return errors.Wrap(err, "upserting peer")
	}
	return nil
}

func (b *Backend) RemovePeer(ctx context.Context, nodeID uint64) error {
	query := `DELETE FROM peers WHERE nodeid = ?`
	_, err := b.raftdb.ExecContext(ctx, query, nodeID)
	if err != nil {
		return errors.Wrap(err, "deleting peer")
	}
	return nil
}

func (b *Backend) Snapshot() (raftpb.Snapshot, error) {
	return raftpb.Snapshot{}, nil
}

func (b *Backend) ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error {
	return nil
}

func (b *Backend) Entries(lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error) {
	ctx := context.Background()
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

func (b *Backend) Term(i uint64) (uint64, error) {
	ctx := context.Background()
	query := `SELECT term FROM entries WHERE idx = ?`
	return queryInteger(ctx, b.raftdb, query, i)
}

func (b *Backend) LastIndex() (uint64, error) {
	ctx := context.Background()
	query := `SELECT max(idx) FROM entries`
	return queryInteger(ctx, b.raftdb, query)
}

func (b *Backend) FirstIndex() (uint64, error) {
	ctx := context.Background()
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
