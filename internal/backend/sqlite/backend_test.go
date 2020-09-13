package sqlite

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/orishu/deeb/internal/lib"
	"github.com/stretchr/testify/require"
)

func Test_basic_sqlite_access(t *testing.T) {
	dir, err := ioutil.TempDir("./testdb", fmt.Sprintf("%s-*", t.Name()))
	defer os.RemoveAll(dir)

	require.NoError(t, err)
	b := New(dir, lib.NewDevelopmentLogger())
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

	err = b.SaveHardState(ctx, raftpb.HardState{Term: 1, Vote: 12, Commit: 100})
	require.NoError(t, err)
	err = b.SaveHardState(ctx, raftpb.HardState{Term: 2, Vote: 12, Commit: 101})
	require.NoError(t, err)

	term, err := b.QueryEntryTerm(ctx, 2)
	require.NoError(t, err)
	require.Equal(t, uint64(1), term)

	entries, err := b.QueryEntries(ctx, 2, 4, 10)
	require.NoError(t, err)
	require.Equal(t,
		[]raftpb.Entry{
			{Index: 2, Term: 1, Type: raftpb.EntryNormal, Data: []byte("world")},
			{Index: 3, Term: 2, Type: raftpb.EntryNormal, Data: []byte("hi")},
		},
		entries,
	)

	minIdx, err := b.QueryFirstIndex(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), minIdx)
	maxIdx, err := b.QueryLastIndex(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(4), maxIdx)
}
