package mysql

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func Test_basic_mysql_access(t *testing.T) {
	ctx := context.Background()

	logger := lib.NewDevelopmentLogger()
	kubeHelper, err := lib.NewKubeHelper("test", logger)
	require.NoError(t, err)

	err = kubeHelper.EnsureSecret(ctx, "my-ssh-key", sshSecretSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureConfigMap(ctx, "t1-deeb-configuration", configMapSpec)
	require.NoError(t, err)
	err = kubeHelper.EnsureStatefulSet(ctx, "t1-deeb", statefulSetSpec)
	require.NoError(t, err)

	podName := "t1-deeb-0"
	err = kubeHelper.WaitForPodToBeReady(ctx, podName, 30)
	require.NoError(t, err)

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

	privKey, err := lib.ExtractBytesFromSecretYAML(sshSecretSpec, "id_rsa")
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

	err = b.SaveConfState(ctx, &raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}})
	require.NoError(t, err)

	err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"})
	require.NoError(t, err)
	err = b.UpsertPeer(ctx, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"})
	require.NoError(t, err)
	peerInfos, err := b.LoadPeers(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, len(peerInfos))
	require.Equal(t, backend.PeerInfo{NodeID: 3, Addr: "localhost", Port: "10000"}, peerInfos[0])
	require.Equal(t, backend.PeerInfo{NodeID: 4, Addr: "localhost", Port: "10001"}, peerInfos[1])

	err = b.RemovePeer(ctx, 3)
	require.NoError(t, err)

	hs, cs, err := st.InitialState()
	require.NoError(t, err)
	require.Equal(t, raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}}, cs)
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

	snap, err := st.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(10), snap.Metadata.Term)
	require.Equal(t, uint64(4), snap.Metadata.Index)

	var snapRef snapshotReference
	err = json.Unmarshal(snap.Data, &snapRef)
	require.NoError(t, err)
	require.Equal(t, "localhost", snapRef.Addr)
	require.Equal(t, sshPort, snapRef.SSHPort)

	backupCommand := "xtrabackup --backup --databases-exclude=raft --stream=xbstream -u root"
	session, _, err := lib.MakeSSHSession("localhost", sshPort, "mysql", privKey)
	require.NoError(t, err)

	snapOutFile, err := os.OpenFile("snap.bin", os.O_RDWR|os.O_CREATE, 0644)
	require.NoError(t, err)
	// TODO: defer os.Remove("snap.bin")

	outPipe, err := session.StdoutPipe()
	require.NoError(t, err)
	err = session.Start(backupCommand)
	require.NoError(t, err)
	_, err = io.Copy(snapOutFile, outPipe)
	_ = snapOutFile.Close()
	session.Close()
	require.NoError(t, err)

	expectedCommand := "xtrabackup --backup --databases-exclude=raft --stream=xbstream -u root"
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
	snapRefData, err := json.Marshal(snapRef)
	require.NoError(t, err)
	snap2 := raftpb.Snapshot{Data: snapRefData, Metadata: snapMeta}
	err = b.ApplySnapshot(ctx, snap2)
	require.NoError(t, err)

	mockSSHServer.Stop()
	/*
		tarFile, err := os.OpenFile(dir+"/testtar.tar", os.O_WRONLY|os.O_CREATE, 0644)
		_, err = io.Copy(tarFile, bytes.NewReader(snap.Data))
		require.NoError(t, err)
		err = tarFile.Close()
		require.NoError(t, err)

		err = b.RemovePeer(ctx, 4)
		require.NoError(t, err)

		tarFile, err = os.Open(dir + "/testtar.tar")
		require.NoError(t, err)
		var buf bytes.Buffer
		_, err = io.Copy(&buf, tarFile)
		require.NoError(t, err)
		tarFile.Close()

		// Remember the current conf state
		_, cs, err = st.InitialState()
		require.NoError(t, err)

		// Override conf state before restoring from snapshot
		err = b.SaveConfState(ctx, &raftpb.ConfState{Nodes: []uint64{13, 14}, Learners: []uint64{15}})
		require.NoError(t, err)

		// Restore from snapshot, use the remembered conf state as metadata
		snapMeta := raftpb.SnapshotMetadata{Term: 30, Index: 300, ConfState: cs}
		snap2 := raftpb.Snapshot{Data: buf.Bytes(), Metadata: snapMeta}
		err = b.ApplySnapshot(ctx, snap2)
		require.NoError(t, err)

		// Check that the overriden conf state is back
		_, cs, err = st.InitialState()
		require.NoError(t, err)
		require.Equal(t, raftpb.ConfState{Nodes: []uint64{3, 4}, Learners: []uint64{5}}, cs)

		// Append some more entries to see older ones deleted
		err = b.AppendEntries(ctx, []raftpb.Entry{
			{Index: 5, Term: 2, Type: raftpb.EntryNormal, Data: []byte("one")},
			{Index: 6, Term: 2, Type: raftpb.EntryNormal, Data: []byte("two")},
			{Index: 7, Term: 2, Type: raftpb.EntryNormal, Data: []byte("three")},
		})
		require.NoError(t, err)

		minIdx, err = st.FirstIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(3), minIdx)
		maxIdx, err = st.LastIndex()
		require.NoError(t, err)
		require.Equal(t, uint64(7), maxIdx)
	*/
}
