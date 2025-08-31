package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"time"

	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3"
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

	// Currently, the Raft implementation panics if LastIndex fails, so it
	// need to try until it succeeds. If MySQL is down, it will fail, but
	// hopefully MySQL will come up at some point.
	for {
		idx, err := queryInteger(ctx, b.raftdb, query)
		if err == nil {
			return idx, nil
		}
		b.logger.Warnf("failed getting last index, error: %+v", err)
		time.Sleep(2 * time.Second)
	}
}

// FirstIndex returns the index of the first log entry that is
// possibly available via Entries (older entries have been incorporated
// into the latest Snapshot
func (b *Backend) FirstIndex() (uint64, error) {
	ctx := context.Background()
	query := `SELECT ifnull(min(idx),1) FROM entries`
	// Currently, the Raft implementation panics if FirstIndex fails, so it
	// need to try until it succeeds. If MySQL is down, it will fail, but
	// hopefully MySQL will come up at some point.
	for {
		idx, err := queryInteger(ctx, b.raftdb, query)
		if err == nil {
			return idx, nil
		}
		b.logger.Warnf("failed getting first index, error: %+v", err)
		time.Sleep(2 * time.Second)
	}
}

// Snapshot returns not the actual snapshot, but a reference to how to fetch a
// snapshot through an SSH port.
func (b *Backend) Snapshot() (raftpb.Snapshot, error) {
	ctx := context.Background()
	query := `SELECT term, idx FROM state WHERE id = 1`
	row := b.raftdb.QueryRowContext(ctx, query)

	snapMeta := raftpb.SnapshotMetadata{}
	err := row.Scan(&snapMeta.Term, &snapMeta.Index)
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrap(err, "retrieving raft state")
	}
	snapRef := snapshotReference{
		Addr:    b.addr,
		SSHPort: b.sshPort,
	}
	snapRefBytes, err := json.Marshal(snapRef)
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrap(err, "marshaling snapshot reference struct")
	}
	return raftpb.Snapshot{
		Data:     snapRefBytes,
		Metadata: snapMeta,
	}, nil
}
