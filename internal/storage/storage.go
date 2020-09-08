package storage

import (
	"github.com/coreos/etcd/raft/raftpb"
	"github.com/orishu/deeb/internal/backend"
)

type DBStorage struct {
	backend backend.DBBackend
}

// NewDBStorage creates a DBStorage object.
func NewDBStorage(backend backend.DBBackend) *DBStorage {
	return &DBStorage{
		backend: backend,
	}
}

// InitialState implements the Storage interface.
func (ds *DBStorage) InitialState() (raftpb.HardState, raftpb.ConfState, error) {
	return raftpb.HardState{}, raftpb.ConfState{}, nil
}

// SetHardState saves the current HardState.
func (ds *DBStorage) SetHardState(st raftpb.HardState) error {
	return nil
}

// Entries implements the Storage interface.
func (ds *DBStorage) Entries(lo, hi, maxSize uint64) ([]raftpb.Entry, error) {
	return []raftpb.Entry{}, nil
}

// Term implements the Storage interface.
func (ds *DBStorage) Term(i uint64) (uint64, error) {
	return 0, nil
}

// LastIndex implements the Storage interface.
func (ds *DBStorage) LastIndex() (uint64, error) {
	return 0, nil
}

// FirstIndex implements the Storage interface.
func (ds *DBStorage) FirstIndex() (uint64, error) {
	return 0, nil
}

// Snapshot implements the Storage interface.
func (ds *DBStorage) Snapshot() (raftpb.Snapshot, error) {
	return raftpb.Snapshot{}, nil
}

// ApplySnapshot overwrites the contents of this Storage object with
// those of the given snapshot.
func (ds *DBStorage) ApplySnapshot(snap raftpb.Snapshot) error {
	return nil
}

// CreateSnapshot makes a snapshot which can be retrieved with Snapshot() and
// can be used to reconstruct the state at that point.
// If any configuration changes have been made since the last compaction,
// the result of the last ApplyConfChange must be passed in.
func (ds *DBStorage) CreateSnapshot(i uint64, cs *raftpb.ConfState, data []byte) (raftpb.Snapshot, error) {
	return raftpb.Snapshot{}, nil
}

// Append the new entries to storage.
func (ds *DBStorage) Append(entries []raftpb.Entry) error {
	return nil
}
