package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	_ "github.com/go-sql-driver/mysql"
	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/golang/protobuf/proto"
	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

type Params struct {
	Addr            string
	MysqlPort       int
	SSHPort         int
	PrivateKey      []byte
	EntriesToRetain uint64
}

type snapshotReference struct {
	Addr    string `json:"addr"`
	SSHPort int    `json:"ssh_port"`
}

func New(params Params, logger *zap.SugaredLogger) (backend.DBBackend, raft.Storage) {
	b := &Backend{
		mgmtDBName:      "mgmt",
		raftDBName:      "raft",
		entriesToRetain: params.EntriesToRetain,
		addr:            params.Addr,
		mysqlPort:       params.MysqlPort,
		sshPort:         params.SSHPort,
		privateKey:      params.PrivateKey,
		logger:          logger,
	}
	return b, b
}

// Backend is the mysql backend
type Backend struct {
	mgmtDBName      string
	raftDBName      string
	entriesToRetain uint64
	addr            string
	mysqlPort       int
	sshPort         int
	privateKey      []byte
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
			if e, ok := err.(*mysqldrv.MySQLError); ok {
				if e.Number == 1062 { // Duplicate index
					query := `DELETE FROM entries WHERE idx >= ?`
					_, err2 := b.raftdb.ExecContext(ctx, query, entry.Index)
					if err2 != nil {
						return errors.Wrapf(err, "deleting conflicting entries as of %d", entry.Index)
					}
					res, err = tryInsert(entry)
				}
			}
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
	/*
		_, err := b.maindb.ExecContext(ctx, "FLUSH TABLES WITH READ LOCK")
		if err != nil {
			return errors.Wrap(err, "flushing tables")
		}
		toRevert := true
		defer func() {
			if toRevert {
				_, _ = b.maindb.ExecContext(ctx, "SET GLOBAL read_only = 0")
				_, _ = b.maindb.ExecContext(ctx, "UNLOCK TABLES")
			}
		}()
		_, err = b.maindb.ExecContext(ctx, "SET GLOBAL read_only = 1")
		if err != nil {
			return errors.Wrap(err, "setting read-only")
		}
	*/
	var snapRef snapshotReference
	err := json.Unmarshal(snap.Data, &snapRef)
	if err != nil {
		return errors.Wrap(err, "unmarshaling snapshot reference")
	}

	backupCmd := "xtrabackup --backup --databases-exclude=raft --stream=xbstream --compress -u root"
	remoteSSH, err := lib.RunSSHCommand(snapRef.Addr, snapRef.SSHPort, "mysql", b.privateKey, backupCmd)
	if err != nil {
		return errors.Wrapf(err, "running ssh backup command: %s", backupCmd)
	}
	defer remoteSSH.Close()
	restoreCmd := "bash -c 'cd /var/lib/mysql && rm -rf restore && mkdir restore && cd restore && xbstream -x'"
	localSSH, err := lib.RunSSHCommand("localhost", b.sshPort, "mysql", b.privateKey, restoreCmd)
	if err != nil {
		return errors.Wrapf(err, "running ssh restore command: %s", restoreCmd)
	}
	defer localSSH.Close()

	remoteStdout, err := remoteSSH.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "creating remote stdout")
	}
	localStdin, err := localSSH.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating local stdin")
	}
	_, err = io.Copy(localStdin, remoteStdout)
	if err != nil {
		return wrapErrorAndAddStdoutStderr(err, "piping snapshot reader->writer", localSSH)
	}
	remoteSSH.Close()
	localSSH.Close()

	_, err = b.maindb.ExecContext(ctx, "SHUTDOWN")
	if err != nil {
		return errors.Wrap(err, "shutting down")
	}
	time.Sleep(3 * time.Second) // TODO: make it more scientific

	prepareCmd := "xtrabackup --prepare -u root --target-dir=/var/lib/mysql"
	localSSH2, err := lib.RunSSHCommand("localhost", b.sshPort, "mysql", b.privateKey, prepareCmd)
	if err != nil {
		return errors.Wrapf(err, "running ssh prepare command: %s", prepareCmd)
	}
	defer localSSH2.Close()

	// TODO HERE: wait and check exit code

	if err != nil {
		return wrapErrorAndAddStdoutStderr(err, "running ssh prepare", localSSH2)
	}

	// Wait for mysql container to restart
	for i := 0; i < 60; i++ {
		err = b.innerStart(ctx, false)
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}

	err = b.SaveConfState(ctx, &snap.Metadata.ConfState)
	if err != nil {
		return errors.Wrap(err, "saving conf state from snapshot")
	}
	err = b.SaveApplied(ctx, snap.Metadata.Term, snap.Metadata.Index)
	if err != nil {
		return errors.Wrap(err, "saving term and index from snapshot")
	}

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
	_, err := b.maindb.ExecContext(ctx, sql)
	if err != nil {
		if e, ok := err.(*mysqldrv.MySQLError); ok {
			err = &backend.DBError{Cause: e}
		}
		return err
	}
	return b.SaveApplied(ctx, term, idx)
}

func (b *Backend) QuerySQL(ctx context.Context, sql string) (*sql.Rows, error) {
	return b.maindb.QueryContext(ctx, sql)
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

func wrapErrorAndAddStdoutStderr(err error, msg string, session *ssh.Session) error {
	outBytes := []byte{}
	if outPipe, err := session.StdoutPipe(); err != nil {
		outBytes, _ = ioutil.ReadAll(outPipe)
	}
	errBytes := []byte{}
	if errPipe, err := session.StderrPipe(); err != nil {
		errBytes, _ = ioutil.ReadAll(errPipe)
	}
	msg = fmt.Sprintf("%s\n===== stdout =====%s\n===== stderr =====\n%s", msg, string(outBytes), string(errBytes))
	return errors.Wrap(err, msg)
}
