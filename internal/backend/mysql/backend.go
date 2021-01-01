package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	_ "github.com/go-sql-driver/mysql"
	"github.com/golang/protobuf/proto"
	"github.com/orishu/deeb/internal/backend"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Params struct {
	MysqlPort       int
	EntriesToRetain uint64
}

func New(params Params, logger *zap.SugaredLogger) (backend.DBBackend, raft.Storage) {
	b := &Backend{
		mgmtDBName:      "mgmt",
		raftDBName:      "raft",
		entriesToRetain: params.EntriesToRetain,
		mysqlPort:       params.MysqlPort,
		logger:          logger,
	}
	return b, b
}

// Backend is the mysql backend
type Backend struct {
	mgmtDBName      string
	raftDBName      string
	entriesToRetain uint64
	mysqlPort       int
	maindb          *sql.DB
	mgmtdb          *sql.DB
	raftdb          *sql.DB
	logger          *zap.SugaredLogger
}

// Implementation of the backend.DBBackend interace

func (b *Backend) Start(ctx context.Context) error {
	return b.innerStart(ctx, true)
}

func (b *Backend) innerStart(ctx context.Context, create bool) error {
	connStr := fmt.Sprintf("root@tcp(127.0.0.1:%d)/", b.mysqlPort)
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return errors.Wrapf(err, "connecting to db, string: %s", connStr)
	}
	b.maindb = db
	if create {
		return b.createTables(ctx)
	}
	return nil
}

func (b *Backend) createTables(ctx context.Context) error {
	query := `CREATE DATABASE IF NOT EXISTS raft`
	_, err := b.maindb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating raft database")
	}
	query = `CREATE DATABASE IF NOT EXISTS mgmt`
	_, err = b.maindb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating mgmt database")
	}
	connStrPrefix := fmt.Sprintf("root@tcp(127.0.0.1:%d)/", b.mysqlPort)
	connStr := connStrPrefix + "raft"
	db, err := sql.Open("mysql", connStr)
	if err != nil {
		return errors.Wrapf(err, "connecting to db, string: %s", connStr)
	}
	b.raftdb = db
	connStr = connStrPrefix + "mgmt"
	db, err = sql.Open("mysql", connStr)
	if err != nil {
		return errors.Wrapf(err, "connecting to db, string: %s", connStr)
	}
	b.mgmtdb = db

	query = `CREATE TABLE IF NOT EXISTS state (
			id INT NOT NULL AUTO_INCREMENT PRIMARY KEY,
			idx INT NOT NULL,
			term INT NOT NULL,
			hardstate_term INT NOT NULL,
			hardstate_vote INT NOT NULL,
			hardstate_commit INT NOT NULL,
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
			(idx, term, hardstate_term, hardstate_vote, hardstate_commit)
			VALUES (0, 0, 0, 0, 0)`
		_, err = b.raftdb.ExecContext(ctx, query)
		if err != nil {
			return errors.Wrap(err, "creating first row of state table")
		}
	}

	query = `CREATE TABLE IF NOT EXISTS peers (
			nodeid INT PRIMARY KEY,
			address TEXT NOT NULL,
			port TEXT NOT NULL)`
	_, err = b.mgmtdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating peers table")
	}

	query = `CREATE TABLE IF NOT EXISTS entries (
			idx INT PRIMARY KEY,
			term INT NOT NULL,
			type INT NOT NULL,
			data BLOB)`
	_, err = b.raftdb.ExecContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "creating entries table")
	}

	return nil
}

func (b *Backend) Stop(ctx context.Context) {
	if b.maindb != nil {
		b.maindb.Close()
	}
	if b.raftdb != nil {
		b.raftdb.Close()
	}
	if b.mgmtdb != nil {
		b.mgmtdb.Close()
	}
}

func (b *Backend) AppendEntries(ctx context.Context, entries []raftpb.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	tryInsert := func(entry raftpb.Entry) (sql.Result, error) {
		query := `INSERT INTO entries (idx, term, type, data) VALUES (?,?,?,?)`
		return b.raftdb.ExecContext(ctx, query, entry.Index, entry.Term, entry.Type, entry.Data)
	}
	for _, entry := range entries {
		var res sql.Result
		var err error
		res, err = tryInsert(entry)
		if err != nil {
			/*
				TODO: handle conflicting raft entries

				if e, ok := err.(sqlite.Error); ok {
					if e.Code == sqlite.ErrConstraint {
						query := `DELETE FROM entries WHERE idx >= ?`
						_, err2 := b.raftdb.ExecContext(ctx, query, entry.Index)
						if err2 != nil {
							return errors.Wrapf(err, "deleting conflicting entries as of %d", entry.Index)
						}
						res, err = tryInsert(entry)
					}
				}
			*/
			if err != nil {
				return errors.Wrap(err, "appending raft entries")
			}
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return errors.Wrap(err, "querying rows affected")
		}
		if rows != 1 {
			return fmt.Errorf("unexpected number of rows affected: %d", rows)
		}
	}
	if b.entriesToRetain != 0 {
		query := `DELETE FROM entries WHERE idx + ? <= ?`
		lastIdx := entries[len(entries)-1].Index
		_, err := b.raftdb.ExecContext(ctx, query, b.entriesToRetain, lastIdx)
		if err != nil {
			return errors.Wrap(err, "deleting old raft entries")
		}
	}
	return nil
}

func (b *Backend) SaveHardState(ctx context.Context, hardState *raftpb.HardState) error {
	query := `UPDATE state SET
			hardstate_term = ?,
			hardstate_vote = ?,
			hardstate_commit = ?
			WHERE id = 1`
	_, err := b.raftdb.ExecContext(
		ctx, query, hardState.Term, hardState.Vote, hardState.Commit)
	if err != nil {
		return errors.Wrap(err, "saving hard state")
	}
	return nil
}

func (b *Backend) SaveApplied(ctx context.Context, Term uint64, Index uint64) error {
	query := `UPDATE state SET term = ?, idx = ? WHERE id = 1`
	_, err := b.raftdb.ExecContext(ctx, query, Term, Index)
	if err != nil {
		return errors.Wrap(err, "saving applied state")
	}
	return nil
}

func (b *Backend) GetAppliedIndex(ctx context.Context) (uint64, error) {
	query := `SELECT idx FROM state WHERE id = 1`
	idx, err := queryInteger(ctx, b.raftdb, query)
	if err != nil {
		return 0, errors.Wrap(err, "getting applied index")
	}
	return idx, nil
}

func (b *Backend) SaveConfState(ctx context.Context, confState *raftpb.ConfState) error {
	query := `UPDATE state SET confstate = ? WHERE id = 1`
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

func (b *Backend) LoadPeers(ctx context.Context) ([]backend.PeerInfo, error) {
	query := `SELECT nodeid, address, port FROM peers`
	rows, err := b.mgmtdb.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrapf(err, "query: %s", query)
	}
	res := make([]backend.PeerInfo, 0)
	for rows.Next() {
		var pi backend.PeerInfo
		err := rows.Scan(&pi.NodeID, &pi.Addr, &pi.Port)
		if err != nil {
			return nil, errors.Wrapf(err, "scanning row of query: %s", query)
		}
		res = append(res, pi)
	}
	return res, nil
}

func (b *Backend) UpsertPeer(ctx context.Context, pi backend.PeerInfo) error {
	query := `REPLACE INTO peers (nodeid, address, port) VALUES (?,?,?)`
	_, err := b.mgmtdb.ExecContext(ctx, query, pi.NodeID, pi.Addr, pi.Port)
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
	return nil
}

// Returns a snapshot "handle" that can later be released using RemoveSavedSnapshot
func (b *Backend) SaveSnapshot(ctx context.Context, snap raftpb.Snapshot) (uint64, error) {
	return 0, nil
}

func (b *Backend) RemoveSavedSnapshot(ctx context.Context, snapHandle uint64) error {
	return nil
}

func (b *Backend) ExecSQL(ctx context.Context, term uint64, idx uint64, sql string) error {
	// TODO: run the query

	return b.SaveApplied(ctx, term, idx)
}

func (b *Backend) QuerySQL(ctx context.Context, sql string) (*sql.Rows, error) {
	return nil, nil
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
