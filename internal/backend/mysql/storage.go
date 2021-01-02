package mysql

import (
	"context"
	"database/sql"
	"math"

	"github.com/coreos/etcd/raft/raftpb"
	"github.com/etcd-io/etcd/raft"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// This file is the implementation of the raft.Storage interface

// InitialState returns the saved HardState and ConfState information.
func (b *Backend) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	ctx := context.Background()
	query := `SELECT hardstate_term, hardstate_vote, hardstate_commit, confstate
			FROM state WHERE id = 1`
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

// Entries returns a slice of log entries in the range [lo,hi).
func (b *Backend) Entries(lo uint64, hi uint64, maxSize uint64) ([]raftpb.Entry, error) {
	ctx := context.Background()
	result := []raftpb.Entry{}
	query := `SELECT idx, term, type, data FROM entries
			WHERE idx >= ? AND idx < ? ORDER BY idx ASC`
	var rows *sql.Rows
	var err error
	if maxSize != math.MaxUint64 {
		rows, err = b.raftdb.QueryContext(ctx, query+" LIMIT ?", lo, hi, maxSize)
	} else {
		rows, err = b.raftdb.QueryContext(ctx, query, lo, hi)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "querying entries [%d,%d) limit %d", lo, hi, maxSize)
	}
	defer rows.Close()
	var loIdx, hiIdx uint64
	for rows.Next() {
		entry := raftpb.Entry{}
		err = rows.Scan(&entry.Index, &entry.Term, &entry.Type, &entry.Data)
		if err != nil {
			return nil, errors.Wrap(err, "reading rows")
		}
		result = append(result, entry)
		if loIdx == 0 {
			loIdx = entry.Index
		}
		hiIdx = entry.Index
	}
	if lo < loIdx {
		return nil, raft.ErrCompacted
	}
	if hi > hiIdx+1 {
		return nil, raft.ErrUnavailable
	}
	return result, nil
}

// Term returns the term of entry i, which must be in the range
// [FirstIndex()-1, LastIndex()]. The term of the entry before
// FirstIndex is retained for matching purposes even though the
// rest of that entry may not be available.
func (b *Backend) Term(i uint64) (uint64, error) {
	if i == 0 {
		return 0, nil
	}
	ctx := context.Background()
	query := `SELECT term FROM entries WHERE idx = ?`
	term, err := queryInteger(ctx, b.raftdb, query, i)
	if err == sql.ErrNoRows {
		firstIdx, _ := b.FirstIndex()
		if i < firstIdx {
			return 0, raft.ErrCompacted
		}
		return 0, raft.ErrUnavailable
	}
	if err != nil {
		return 0, err
	}
	return term, nil
}

// LastIndex returns the index of the last entry in the log.
func (b *Backend) LastIndex() (uint64, error) {
	ctx := context.Background()
	query := `SELECT ifnull(max(idx),0) FROM entries`
	return queryInteger(ctx, b.raftdb, query)
}

// FirstIndex returns the index of the first log entry that is
// possibly available via Entries (older entries have been incorporated
// into the latest Snapshot
func (b *Backend) FirstIndex() (uint64, error) {
	ctx := context.Background()
	query := `SELECT ifnull(min(idx),1) FROM entries`
	return queryInteger(ctx, b.raftdb, query)
}

// Snapshot returns the most recent snapshot.
func (b *Backend) Snapshot() (raftpb.Snapshot, error) {
	return raftpb.Snapshot{}, nil
}
