package sqlite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/orishu/deeb/internal/backend"
	"github.com/orishu/deeb/internal/lib"
	"github.com/stretchr/testify/require"
)

func Test_basic_sqlite_access(t *testing.T) {
	dir, err := ioutil.TempDir("./testdb", fmt.Sprintf("%s-*", t.Name()))
	defer os.RemoveAll(dir)

	require.NoError(t, err)
	b, st := New(dir, lib.NewDevelopmentLogger())
	ctx := context.Background()
	err = b.Start(ctx)
	defer b.Stop(ctx)
	require.NoError(t, err)

	err = b.AppendEntries(ctx, []raftpb.Entry{
		{Index: 1, Term: 1, Type: raftpb.EntryNormal, Data: []byte("hello")},
		{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
		{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
		{Index: 4, Term: 2, Type: raftpb.EntryNormal, Data: []byte("there")},
	})
	require.NoError(t, err)

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

	snap, err := st.Snapshot()
	require.NoError(t, err)
	require.Equal(t, uint64(10), snap.Metadata.Term)
	require.Equal(t, uint64(123), snap.Metadata.Index)
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
}
