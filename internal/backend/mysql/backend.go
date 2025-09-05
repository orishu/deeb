package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysqldrv "github.com/go-sql-driver/mysql"
	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	"github.com/pkg/errors"
	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.uber.org/zap"
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
	if b.maindb != nil {
		b.maindb.Close()
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
	d, err := confState.Marshal()
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

func (b *Backend) UpsertPeer(ctx context.Context, pi backend.PeerInfo) (uint64, error) {
	query := `SELECT nodeid FROM peers WHERE address = ? AND port = ?`
	rows, err := b.mgmtdb.QueryContext(ctx, query, pi.Addr, pi.Port)
	if err != nil {
		return 0, errors.Wrapf(err, "query when upserting address %s port %s", pi.Addr, pi.Port)
	}
	var existingNodeID uint64
	if rows.Next() {
		if err := rows.Scan(&existingNodeID); err != nil {
			rows.Close()
			return 0, errors.Wrap(err, "scanning row for existing peer node ID")
		}
	}
	rows.Close()

	query = `REPLACE INTO peers (nodeid, address, port) VALUES (?,?,?)`
	_, err = b.mgmtdb.ExecContext(ctx, query, pi.NodeID, pi.Addr, pi.Port)
	if err != nil {
		return 0, errors.Wrap(err, "upserting peer")
	}
	return existingNodeID, err
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
	return b.ApplySnapshotWithPortRecovery(ctx, snap, func() error { return nil })
}

func (b *Backend) ApplySnapshotWithPortRecovery(ctx context.Context, snap raftpb.Snapshot, recoverPortForwarding func() error) error {
	// Ensure BLOCKED file exists to prevent MySQL from starting until restore is complete
	b.logger.Info("Ensuring BLOCKED file exists to prevent MySQL startup...")
	err := b.runSSHCommand("touch /var/lib/mysql/BLOCKED && echo 'BLOCKED file ready'")
	if err != nil {
		return errors.Wrap(err, "ensuring BLOCKED file exists")
	}

	// Before applying the snapshot, set the database to read-only.
	_, err = b.maindb.ExecContext(ctx, "SHUTDOWN")
	if err != nil {
		return errors.Wrap(err, "shutting down")
	}

	// Wait for MySQL container to signal shutdown completion
	b.logger.Info("Waiting for MySQL shutdown to complete...")
	err = b.waitForShutdownCompletion(ctx, 30)
	if err != nil {
		return errors.Wrap(err, "MySQL shutdown did not complete properly")
	}

	// Remove data dir files - MySQL container will be blocked from starting
	err = b.runSSHCommand("rm -rf /var/lib/mysql/active/*")
	if err != nil {
		return errors.Wrap(err, "removing old data dir")
	}
	success := false
	defer func() {
		if !success {
			// Clean up on failure - remove BLOCKED file and cleanup temp directories
			b.runSSHCommand("rm -f /var/lib/mysql/BLOCKED")
			b.runSSHCommand("rm -rf /var/lib/mysql/restore")
		}
	}()

	var snapRef snapshotReference
	err = json.Unmarshal(snap.Data, &snapRef)
	if err != nil {
		return errors.Wrap(err, "unmarshaling snapshot reference")
	}

	backupCmd := "xtrabackup --backup --stream=xbstream -u root -S /var/lib/mysql/mysql.sock"
	remoteSSH, remoteStderr, err := lib.MakeSSHSession(snapRef.Addr, snapRef.SSHPort, "mysql", b.privateKey)
	remoteStdout, err := remoteSSH.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "creating remote stdout")
	}
	err = remoteSSH.Start(backupCmd)
	if err != nil {
		return errors.Wrapf(err, "running ssh backup command: %s", backupCmd)
	}
	defer remoteSSH.Close()
	restoreCmd := "bash -c 'cd /var/lib/mysql && rm -rf restore new && mkdir restore new && cd restore && xbstream -x'"
	localSSH, localStderr, err := lib.MakeSSHSession("localhost", b.sshPort, "mysql", b.privateKey)
	if err != nil {
		return errors.Wrapf(err, "creating local ssh")
	}
	defer localSSH.Close()
	localStdin, err := localSSH.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating local stdin")
	}
	err = localSSH.Start(restoreCmd)
	if err != nil {
		return errors.Wrapf(err, "running ssh restore command: %s", restoreCmd)
	}

	pipedBytes, err := io.Copy(localStdin, remoteStdout)
	b.logger.Infof("Piped xtrabackup bytes: %d", pipedBytes)
	if err != nil {
		remoteErrBytes, _ := io.ReadAll(remoteStderr)
		localErrBytes, _ := io.ReadAll(localStderr)
		msg := fmt.Sprintf("%s\n===== remote stderr =====\n%s\n===== local stderr =====\n%s", "piping backup", string(remoteErrBytes), string(localErrBytes))
		return errors.Wrap(err, msg)
	}
	remoteSSH.Close()
	localSSH.Close()

	// Prepare the backup and move it directly to active directory
	err = b.runSSHCommand("cd /var/lib/mysql && xtrabackup --prepare --target-dir=/var/lib/mysql/restore")
	if err != nil {
		return errors.Wrap(err, "preparing xtrabackup")
	}

	// Use xtrabackup --move-back to efficiently move (not copy) the restored data
	err = b.runSSHCommand("cd /var/lib/mysql && xtrabackup --move-back --datadir=/var/lib/mysql/active --target-dir=/var/lib/mysql/restore")
	if err != nil {
		return errors.Wrap(err, "moving restored data to active directory")
	}

	// Set proper permissions for MySQL access
	err = b.runSSHCommand("chown -R 1001:1001 /var/lib/mysql/active && chmod -R 755 /var/lib/mysql/active && find /var/lib/mysql/active -type f -exec chmod 644 {} \\;")
	if err != nil {
		return errors.Wrap(err, "setting permissions on restored data")
	}

	// Remove BLOCKED file to allow MySQL to start with restored data
	b.logger.Info("Removing BLOCKED file to allow MySQL to start...")
	err = b.runSSHCommand("rm -f /var/lib/mysql/BLOCKED && echo 'BLOCKED file removed'")
	if err != nil {
		b.logger.Warn("Failed to remove BLOCKED file: " + err.Error())
	}

	// Give MySQL container time to detect that BLOCKED file is gone and start
	b.logger.Info("Waiting for MySQL container to detect restored data...")
	time.Sleep(5 * time.Second)

	// Wait for MySQL process to start inside the pod before recovering port forwarding
	b.logger.Info("Waiting for MySQL to start listening on port 3306 inside pod...")

	err = b.waitForMySQLToStart(ctx, 60)
	if err != nil {
		// If MySQL fails to start, let's check what's in the MySQL container logs
		b.logger.Error("MySQL failed to start. This requires checking the MySQL container logs with: kubectl logs <pod-name> -c <mysql-container-name>")
		return errors.Wrap(err, "MySQL did not start listening inside pod")
	}

	// Recover port forwarding after MySQL shutdown/restart
	err = recoverPortForwarding()
	if err != nil {
		return errors.Wrap(err, "recovering port forwarding")
	}

	err = b.waitForDatabaseToComeUpWithRecovery(ctx, 360, recoverPortForwarding)
	if err != nil {
		return errors.Wrap(err, "database did not come up")
	}

	// Clean up temporary restore directory
	b.runSSHCommand("rm -rf /var/lib/mysql/restore")

	success = true
	return nil
}

func (b *Backend) runSSHCommand(command string) error {
	localSSH, localStderr, err := lib.MakeSSHSession("localhost", b.sshPort, "mysql", b.privateKey)
	if err != nil {
		return errors.Wrapf(err, "starting SSH session for command [%s]", command)
	}
	defer localSSH.Close()

	localStdout, err := localSSH.StdoutPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdout pipe")
	}

	err = localSSH.Start(command)
	if err != nil {
		return errors.Wrapf(err, "starting command [%s]", command)
	}

	output, err := io.ReadAll(localStdout)
	if err != nil {
		return errors.Wrap(err, "reading stdout")
	}

	err = localSSH.Wait()
	if err != nil {
		stderrBytes, _ := io.ReadAll(localStderr)
		b.logger.Errorf("SSH command failed: %s\nSTDOUT:\n%s\nSTDERR:\n%s", command, string(output), string(stderrBytes))
		return wrapErrorAndAddStderr(err, fmt.Sprintf("running command [%s]", command), localStderr)
	}

	b.logger.Infof("SSH command output for [%s]:\n%s", command, string(output))
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

func (b *Backend) SSHPort() int {
	return b.sshPort
}

func (b *Backend) waitForDatabaseToComeUp(ctx context.Context, attempts int) error {
	return b.waitForDatabaseToComeUpWithRecovery(ctx, attempts, func() error { return nil })
}

func (b *Backend) waitForDatabaseToComeUpWithRecovery(ctx context.Context, attempts int, recoverPortForwarding func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = b.innerStart(ctx, false)
		if err == nil {
			_, err2 := b.maindb.ExecContext(ctx, "SELECT 1")
			if err2 == nil {
				break
			}
		}
		// Try to recover port forwarding on connection failures
		if i > 0 && (i%5 == 0) { // Try recovery every 5 attempts to avoid spam
			b.logger.Infof("Attempting port forwarding recovery after %d failed attempts", i+1)
			recoveryErr := recoverPortForwarding()
			if recoveryErr != nil {
				b.logger.Warnf("Port forwarding recovery failed: %v", recoveryErr)
			}
		}
		time.Sleep(time.Second)
	}
	return err
}

func (b *Backend) waitForShutdownCompletion(ctx context.Context, timeoutSeconds int) error {
	for i := 0; i < timeoutSeconds; i++ {
		// Try to ping MySQL - if it fails, MySQL has shut down
		if b.maindb != nil {
			err := b.maindb.PingContext(ctx)
			if err != nil {
				b.logger.Info("MySQL shutdown almost complete - database connection no longer responds")
				// Extra few seconds to let mysql shut down even more gracefully.
				time.Sleep(3 * time.Second)
				b.logger.Info("MySQL shutdown probably completed")
				return nil
			}
		}

		b.logger.Infof("Waiting for MySQL shutdown completion, attempt %d/%d", i+1, timeoutSeconds)
		time.Sleep(time.Second)
	}
	return errors.New("MySQL shutdown did not complete within timeout")
}

func (b *Backend) waitForMySQLToStart(ctx context.Context, attempts int) error {
	for i := 0; i < attempts; i++ {
		// Check if MySQL is listening on port 3306 inside the pod via SSH
		// Use ss without -p flag to avoid permission issues, or check for mysqld process
		checkCmd := "ss -tln | grep ':3306 ' || netstat -tln | grep ':3306 ' || pgrep -f mysqld"
		err := b.runSSHCommand(checkCmd)
		if err == nil {
			b.logger.Info("MySQL is now listening on port 3306 inside pod")
			return nil
		}

		// Every 10 attempts, check MySQL error logs to understand why it's not starting
		if i > 0 && (i%10 == 0) {
			b.logger.Infof("MySQL still not started after %d attempts, checking error logs...", i+1)
			errorLogCmd := "tail -20 /var/log/mysql/error.log 2>/dev/null || journalctl -u mysql -n 20 2>/dev/null || echo 'No error logs found'"
			b.runSSHCommand(errorLogCmd) // Ignore error, just for debugging
		}

		b.logger.Infof("MySQL not yet listening inside pod, attempt %d/%d", i+1, attempts)
		time.Sleep(time.Second)
	}
	return errors.New("MySQL did not start listening on port 3306 within timeout")
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

func wrapErrorAndAddStderr(err error, msg string, stderr io.Reader) error {
	errBytes := []byte{}
	errBytes, _ = io.ReadAll(stderr)
	msg = fmt.Sprintf("%s\n===== stderr =====\n%s", msg, string(errBytes))
	return errors.Wrap(err, msg)
}
