package sqlite

import (
	"archive/tar"
	"bytes"
	"context"
	"database/sql"
	"io"
	"math"
	"os"
	"path/filepath"

	"go.etcd.io/etcd/raft/v3/raftpb"
	"go.etcd.io/etcd/raft/v3"
	"github.com/pkg/errors"
)

// This file is the implementation of the raft.Storage interface

// InitialState returns the saved HardState and ConfState information.
func (b *Backend) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	ctx := context.Background()
	query := `SELECT hardstate_term, hardstate_vote, hardstate_commit, confstate
			FROM state WHERE rowid = 1`
	row := b.raftdb.QueryRowContext(ctx, query)
	hard := raftpb.HardState{}
	var confBytes []byte
	err := row.Scan(&hard.Term, &hard.Vote, &hard.Commit, &confBytes)
	if err != nil {
		return raftpb.HardState{}, raftpb.ConfState{}, errors.Wrapf(err, "query: %s", query)
	}
	conf := raftpb.ConfState{}
	if len(confBytes) > 0 {
		err = conf.Unmarshal(confBytes)
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
	dir, err := os.Open(b.dbDir)
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrapf(err, "opening dir %s", b.dbDir)
	}
	fileinfos, err := dir.Readdir(0)
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrapf(err, "reading dir %s", b.dbDir)
	}
	// First, open all files so they are read-locked.
	openFiles := make([]*os.File, 0, len(fileinfos))
	for _, fi := range fileinfos {
		f, err := os.Open(dir.Name() + "/" + fi.Name())
		if err != nil {
			return raftpb.Snapshot{}, errors.Wrapf(err, "opening file %s", fi.Name())
		}
		openFiles = append(openFiles, f)
		defer f.Close()
	}

	ctx := context.Background()
	query := `SELECT term, idx FROM state WHERE rowid = 1`
	row := b.raftdb.QueryRowContext(ctx, query)

	snapMeta := raftpb.SnapshotMetadata{}
	err = row.Scan(&snapMeta.Term, &snapMeta.Index)
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrap(err, "retrieving raft state")
	}

	var buf bytes.Buffer
	tarWriter := tar.NewWriter(&buf)
	for i, fi := range fileinfos {
		if filepath.Base(fi.Name()) == "raft.sqlite" {
			// Do not archive the raft DB
			continue
		}
		hdr := &tar.Header{
			Name: filepath.Base(fi.Name()),
			Mode: int64(fi.Mode()),
			Size: fi.Size(),
		}
		if err := tarWriter.WriteHeader(hdr); err != nil {
			return raftpb.Snapshot{}, errors.Wrapf(err, "archiving file header %s", fi.Name())
		}
		_, err = io.Copy(tarWriter, openFiles[i])
		if err != nil {
			return raftpb.Snapshot{}, errors.Wrapf(err, "archiving file %s", fi.Name())
		}
	}
	err = tarWriter.Close()
	if err != nil {
		return raftpb.Snapshot{}, errors.Wrap(err, "closing tar")
	}
	return raftpb.Snapshot{Data: buf.Bytes(), Metadata: snapMeta}, nil
}
