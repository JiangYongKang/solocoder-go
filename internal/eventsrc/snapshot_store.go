package eventsrc

import "sync"

type SnapshotStore interface {
	SaveSnapshot(snapshot *Snapshot) error
	LoadSnapshot(aggregateID string) (*Snapshot, error)
}

type InMemorySnapshotStore struct {
	mu        sync.RWMutex
	snapshots map[string]*Snapshot
}

func NewInMemorySnapshotStore() *InMemorySnapshotStore {
	return &InMemorySnapshotStore{
		snapshots: make(map[string]*Snapshot),
	}
}

func (s *InMemorySnapshotStore) SaveSnapshot(snapshot *Snapshot) error {
	if snapshot == nil {
		return ErrSnapshotNil
	}
	if snapshot.AggregateID == "" {
		return ErrInvalidAggregateID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.snapshots[snapshot.AggregateID] = snapshot
	return nil
}

func (s *InMemorySnapshotStore) LoadSnapshot(aggregateID string) (*Snapshot, error) {
	if aggregateID == "" {
		return nil, ErrInvalidAggregateID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot, ok := s.snapshots[aggregateID]
	if !ok {
		return nil, ErrSnapshotNotFound
	}

	return snapshot, nil
}
