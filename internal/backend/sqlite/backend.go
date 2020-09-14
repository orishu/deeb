package sqlite

import (
	"archive/tar"
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/golang/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/orishu/deeb/internal/backend"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func New(dbDir string, logger *zap.SugaredLogger) backend.DBBackend {
	return &Backend{
		dbDir:      dbDir,
		dbPath:     fmt.Sprintf("%s/db.sqlite", dbDir),
		mgmtDBPath: fmt.Sprintf("%s/mgmt.sqlite", dbDir),
		raftDBPath: fmt.Sprintf("%s/raft.sqlite", dbDir),
		logger:     logger,
	}
}

// Backend is the sqlite backend
type Backend struct {
	dbDir      string
	dbPath     string
	mgmtDBPath string
	raftDBPath string
	db         *sql.DB
	mgmtdb     *sql.DB
	raftdb     *sql.DB
	logger     *zap.SugaredLogger
}

// Implementation of the backend.DBBackend interace

func (b *Backend) Start(ctx context.Context) error {
	db, err := sql.Open("sqlite3", b.dbPath)
	if err != nil {
		return errors.Wrapf(err, "opening db at %s", b.dbPath)
	}
	b.db = db
	mgmtdb, err := sql.Open("sqlite3", b.mgmtDBPath)
	if err != nil {
		return errors.Wrapf(err, "opening mgmt db at %s", b.mgmtDBPath)
	}
	b.mgmtdb = mgmtdb
	raftdb, err := sql.Open("sqlite3", b.raftDBPath)
	if err != nil {
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
	_, err = b.mgmtdb.ExecContext(ctx, query)
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
	if b.mgmtdb != nil {
		b.mgmtdb.Close()
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

func (b *Backend) SaveApplied(ctx context.Context, Term uint64, Index uint64) error {
	query := `UPDATE state SET term = ?, idx = ? WHERE rowid = 1`
	_, err := b.raftdb.ExecContext(ctx, query, Term, Index)
	if err != nil {
		return errors.Wrap(err, "saving applied state")
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
	_, err := b.mgmtdb.ExecContext(ctx, query, nodeID, addr, port)
	if err != nil {
		return errors.Wrap(err, "upserting peer")
	}
	return nil
}

func (b *Backend) RemovePeer(ctx context.Context, nodeID uint64) error {
	query := `DELETE FROM peers WHERE nodeid = ?`
	_, err := b.mgmtdb.ExecContext(ctx, query, nodeID)
	if err != nil {
		return errors.Wrap(err, "deleting peer")
	}
	return nil
}

func (b *Backend) ApplySnapshot(ctx context.Context, snap raftpb.Snapshot) error {
	b.Stop(ctx)
	restarted := false
	defer func() {
		if !restarted {
			b.Start(ctx)
		}
	}()
	tr := tar.NewReader(bytes.NewReader(snap.Data))
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "reading tar")
		}
		fname := b.dbDir + "/" + hdr.Name
		f, err := os.OpenFile(fname, os.O_WRONLY|os.O_CREATE, os.FileMode(hdr.Mode))
		if err != nil {
			return errors.Wrapf(err, "opening file %s for writing", hdr.Name)
		}
		_, err = io.Copy(f, tr)
		if err != nil {
			f.Close()
			return errors.Wrapf(err, "extracting %s", hdr.Name)
		}
		f.Close()
	}

	if err := b.Start(ctx); err != nil {
		return errors.Wrap(err, "restarting databases after applying snapshot")
	}
	restarted = true

	err := b.SaveConfState(ctx, &snap.Metadata.ConfState)
	if err != nil {
		return errors.Wrap(err, "saving conf state from snapshot")
	}
	err = b.SaveApplied(ctx, snap.Metadata.Term, snap.Metadata.Index)
	if err != nil {
		return errors.Wrap(err, "saving term and index from snapshot")
	}

	return nil
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
