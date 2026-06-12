package eventsrc

import (
	"sort"
	"sync"
)

type EventStore interface {
	AppendEvents(aggregateID string, expectedVersion int64, events []*Event) error
	LoadEvents(aggregateID string, fromVersion int64) ([]*Event, error)
	GetVersion(aggregateID string) (int64, error)
}

type InMemoryEventStore struct {
	mu        sync.RWMutex
	events    map[string][]*Event
	versions  map[string]int64
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events:   make(map[string][]*Event),
		versions: make(map[string]int64),
	}
}

func (s *InMemoryEventStore) AppendEvents(aggregateID string, expectedVersion int64, events []*Event) error {
	if aggregateID == "" {
		return ErrInvalidAggregateID
	}
	if len(events) == 0 {
		return ErrInvalidEvent
	}
	for _, e := range events {
		if e == nil {
			return ErrEventNil
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentVersion := s.versions[aggregateID]
	if currentVersion != expectedVersion {
		return ErrVersionConflict
	}

	for _, event := range events {
		if event.AggregateID == "" {
			event.AggregateID = aggregateID
		}
		currentVersion++
		event.Version = currentVersion
		s.events[aggregateID] = append(s.events[aggregateID], event)
	}

	s.versions[aggregateID] = currentVersion
	return nil
}

func (s *InMemoryEventStore) LoadEvents(aggregateID string, fromVersion int64) ([]*Event, error) {
	if aggregateID == "" {
		return nil, ErrInvalidAggregateID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	allEvents, ok := s.events[aggregateID]
	if !ok {
		return nil, ErrAggregateNotFound
	}

	var result []*Event
	for _, e := range allEvents {
		if e.Version > fromVersion {
			result = append(result, e)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}

func (s *InMemoryEventStore) GetVersion(aggregateID string) (int64, error) {
	if aggregateID == "" {
		return 0, ErrInvalidAggregateID
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	version, ok := s.versions[aggregateID]
	if !ok {
		return 0, ErrAggregateNotFound
	}

	return version, nil
}
