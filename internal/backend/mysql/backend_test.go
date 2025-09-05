package mysql

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	internaltesting "github.com/orishu/deeb/internal/lib/testing"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

func Test_basic_mysql_access(t *testing.T) {
	ctx := context.Background()

	logger := lib.NewDevelopmentLogger()
	kubeHelper, err := lib.NewKubeHelper("test", logger)
	require.NoError(t, err)

	err = kubeHelper.EnsureSecret(ctx, "my-ssh-key", internaltesting.SSHSecretSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureConfigMap(ctx, "t1-deeb-configuration", internaltesting.ConfigMapSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureStatefulSet(ctx, "t1-deeb", statefulSetSpec)
	require.NoError(t, err)

	podName := "t1-deeb-0"
	err = kubeHelper.WaitForPodToBeReady(ctx, podName, 30)
	require.NoError(t, err)
	time.Sleep(time.Second)

	ports, err := freeport.GetFreePorts(3)
	require.NoError(t, err)
	mysqlPort := ports[0]
	sshPort := ports[1]
	mockSourceSSHPort := ports[2]

	portForwardCloser1, err := kubeHelper.PortForward(podName, mysqlPort, 3306)
	require.NoError(t, err)
	defer portForwardCloser1()
	portForwardCloser2, err := kubeHelper.PortForward(podName, sshPort, 22)
	require.NoError(t, err)
	defer portForwardCloser2()

	privKey, err := lib.ExtractBytesFromSecretYAML(internaltesting.SSHSecretSpec, "id_rsa")
	require.NoError(t, err)

	b, st := New(Params{
		EntriesToRetain: 5,
		Addr:            "localhost",
		MysqlPort:       mysqlPort,
		SSHPort:         sshPort,
		PrivateKey:      privKey,
	}, logger)
	err = b.Start(ctx)
	defer b.Stop(ctx)
	require.NoError(t, err)

	err = b.AppendEntries(ctx, []raftpb.Entry{
		{Index: 1, Term: 1, Type: raftpb.EntryNormal, Data: []byte("hello")},
		{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
		{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
		{Index: 4, Term: 2, Type: raftpb.EntryNormal, Data: []byte("there")},
		{Index: 5, Term: 2, Type: raftpb.EntryNormal, Data: []byte("fifth")},
	})
	require.NoError(t, err)

	err = b.AppendEntries(ctx, []raftpb.Entry{
		{Index: 4, Term: 2, Type: raftpb.EntryNormal, Data: []byte("there2")},
	})
	_, err = st.Entries(5, 5, 1)
	require.Error(t, err)

	err = b.SaveHardState(ctx, &raftpb.HardState{Term: 1, Vote: 12, Commit: 100})
	require.NoError(t, err)
	err = b.SaveHardState(ctx, &raftpb.HardState{Term: 2, Vote: 12, Commit: 101})
	require.NoError(t, err)

	term, err := st.Term(2)
	require.NoError(t, err)
	require.Equal(t, uint64(1), term)

	entries, err := st.Entries(2, 4, 10)
	require.NoError(t, err)
	require.Equal(t,
		[]raftpb.Entry{
			{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
			{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
		},
		entries,
	)

	minIdx, err := st.FirstIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(1), minIdx)
	maxIdx, err := st.LastIndex()
	require.NoError(t, err)
	require.Equal(t, uint64(4), maxIdx)

	err = b.SaveConfState(ctx, &raftpb.ConfState{Voters: []uint64{3, 4}, Learners: []uint64{5}})
	require.NoError(t, err)

	prevID, err := b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 2, Addr: "localhost", Port: "10000"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), prevID)

	// Upserting the same addr/port should return the previous node ID
	// associated with them.
	prevID, err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"})
	require.NoError(t, err)
	require.Equal(t, uint64(2), prevID)

	prevID, err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"})
	require.NoError(t, err)
	require.Equal(t, uint64(0), prevID)

	peerInfos, err := b.LoadPeers(ctx)
	require.NoError(t, err)
	require.Equal(t, 3, len(peerInfos))
	require.Equal(t, backend.PeerInfo{NodeID: 2, Addr: "localhost", Port: "10000"}, peerInfos[0])
	require.Equal(t, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"}, peerInfos[1])
	require.Equal(t, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"}, peerInfos[2])

	err = b.RemovePeer(ctx, 3)
	require.NoError(t, err)

	hs, cs, err := st.InitialState()
	require.NoError(t, err)
	require.Equal(t, raftpb.ConfState{Voters: []uint64{3, 4}, Learners: []uint64{5}}, cs)
	require.Equal(t, raftpb.HardState{Term: 2, Vote: 12, Commit: 101}, hs)

	err = b.SaveApplied(ctx, 10, 123)
	require.NoError(t, err)

	err = b.ExecSQL(ctx, 10, 1, "CREATE DATABASE IF NOT EXISTS unittest")
	require.NoError(t, err)
	err = b.ExecSQL(ctx, 10, 2, "DROP TABLE IF EXISTS unittest.table1")
	require.NoError(t, err)
	err = b.ExecSQL(ctx, 10, 3, "CREATE TABLE unittest.table1 (col1 INT, col2 INT)")
	require.NoError(t, err)
	err = b.ExecSQL(ctx, 10, 4, "INSERT INTO unittest.table1 (col1, col2) VALUES (100, 200), (101, 201)")
	require.NoError(t, err)

	checkTable1 := func() {
		rows, err := b.QuerySQL(ctx, "SELECT col1, col2 FROM unittest.table1")
		require.NoError(t, err)
		require.True(t, rows.Next())
		var v1, v2 int
		err = rows.Scan(&v1, &v2)
		require.NoError(t, err)
		require.Equal(t, 100, v1)
		require.Equal(t, 200, v2)
		require.True(t, rows.Next())
		err = rows.Scan(&v1, &v2)
		require.NoError(t, err)
		require.Equal(t, 101, v1)
		require.Equal(t, 201, v2)
		require.False(t, rows.Next())
	}
	checkTable1()

	snap, err := st.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(10), snap.Metadata.Term)
	require.Equal(t, uint64(4), snap.Metadata.Index)

	var snapRef snapshotReference
	err = json.Unmarshal(snap.Data, &snapRef)
	require.NoError(t, err)
	require.Equal(t, "localhost", snapRef.Addr)
	require.Equal(t, sshPort, snapRef.SSHPort)

	backupCommand := "xtrabackup --backup --stream=xbstream -u root -S /var/lib/mysql/mysql.sock"
	session, _, err := lib.MakeSSHSession("localhost", sshPort, "mysql", privKey)
	require.NoError(t, err)

	snapOutFile, err := os.OpenFile("snap.bin", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	defer os.Remove("snap.bin")

	outPipe, err := session.StdoutPipe()
	require.NoError(t, err)
	err = session.Start(backupCommand)
	require.NoError(t, err)
	_, err = io.Copy(snapOutFile, outPipe)
	_ = snapOutFile.Close()
	session.Close()
	require.NoError(t, err)

	// Override some data so we verify that it's back to the original
	// values after applying the snapshot.
	err = b.ExecSQL(ctx, 10, 5, "UPDATE unittest.table1 set col1 = 999")
	require.NoError(t, err)

	expectedCommand := "xtrabackup --backup --stream=xbstream -u root -S /var/lib/mysql/mysql.sock"
	snapFile, err := os.Open("snap.bin")
	defer snapFile.Close()

	mockSSHServer := lib.NewSSHRespondingServer(
		mockSourceSSHPort,
		privKey, // use something as host key
		expectedCommand,
		snapFile,
		logger,
	)
	mockSSHServer.StartAsync()
	defer mockSSHServer.Stop()
	time.Sleep(time.Second)

	// Restore from snapshot
	snapMeta := raftpb.SnapshotMetadata{Term: 10, Index: 4, ConfState: cs}
	snapRefData, err := json.Marshal(snapshotReference{Addr: "localhost", SSHPort: mockSourceSSHPort})
	require.NoError(t, err)
	snap2 := raftpb.Snapshot{Data: snapRefData, Metadata: snapMeta}
	err = b.ApplySnapshot(ctx, snap2)
	require.NoError(t, err)

	checkTable1()
}
