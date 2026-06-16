package streamproc

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var checkpointCounter int64

type MemoryCheckpointStore struct {
	mu       sync.RWMutex
	checkpoints map[string]*Checkpoint
	order    []string
}

func NewMemoryCheckpointStore() *MemoryCheckpointStore {
	return &MemoryCheckpointStore{
		checkpoints: make(map[string]*Checkpoint),
		order:       make([]string, 0),
	}
}

func (m *MemoryCheckpointStore) Save(_ context.Context, cp *Checkpoint) error {
	if cp == nil {
		return ErrInvalidCheckpoint
	}
	if cp.ID == "" {
		return fmt.Errorf("streamproc: checkpoint id cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	cpCopy := &Checkpoint{
		ID:             cp.ID,
		Timestamp:      cp.Timestamp,
		SourceOffset:   cp.SourceOffset,
		OperatorStates: make(map[string][]byte, len(cp.OperatorStates)),
		WindowStates:   make(map[string][]byte, len(cp.WindowStates)),
		Metadata:       make(map[string]interface{}, len(cp.Metadata)),
	}

	for k, v := range cp.OperatorStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.OperatorStates[k] = data
	}

	for k, v := range cp.WindowStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.WindowStates[k] = data
	}

	for k, v := range cp.Metadata {
		cpCopy.Metadata[k] = v
	}

	if _, exists := m.checkpoints[cp.ID]; !exists {
		m.order = append(m.order, cp.ID)
	}
	m.checkpoints[cp.ID] = cpCopy

	return nil
}

func (m *MemoryCheckpointStore) Load(_ context.Context, id string) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cp, ok := m.checkpoints[id]
	if !ok {
		return nil, ErrCheckpointNotFound
	}

	cpCopy := &Checkpoint{
		ID:             cp.ID,
		Timestamp:      cp.Timestamp,
		SourceOffset:   cp.SourceOffset,
		OperatorStates: make(map[string][]byte, len(cp.OperatorStates)),
		WindowStates:   make(map[string][]byte, len(cp.WindowStates)),
		Metadata:       make(map[string]interface{}, len(cp.Metadata)),
	}

	for k, v := range cp.OperatorStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.OperatorStates[k] = data
	}

	for k, v := range cp.WindowStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.WindowStates[k] = data
	}

	for k, v := range cp.Metadata {
		cpCopy.Metadata[k] = v
	}

	return cpCopy, nil
}

func (m *MemoryCheckpointStore) Latest(_ context.Context) (*Checkpoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.order) == 0 {
		return nil, ErrCheckpointNotFound
	}

	latestID := m.order[len(m.order)-1]
	cp := m.checkpoints[latestID]

	cpCopy := &Checkpoint{
		ID:             cp.ID,
		Timestamp:      cp.Timestamp,
		SourceOffset:   cp.SourceOffset,
		OperatorStates: make(map[string][]byte, len(cp.OperatorStates)),
		WindowStates:   make(map[string][]byte, len(cp.WindowStates)),
		Metadata:       make(map[string]interface{}, len(cp.Metadata)),
	}

	for k, v := range cp.OperatorStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.OperatorStates[k] = data
	}

	for k, v := range cp.WindowStates {
		data := make([]byte, len(v))
		copy(data, v)
		cpCopy.WindowStates[k] = data
	}

	for k, v := range cp.Metadata {
		cpCopy.Metadata[k] = v
	}

	return cpCopy, nil
}

func (m *MemoryCheckpointStore) List(_ context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]string, len(m.order))
	copy(result, m.order)
	sort.Strings(result)
	return result, nil
}

func (m *MemoryCheckpointStore) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.checkpoints[id]; !ok {
		return ErrCheckpointNotFound
	}

	delete(m.checkpoints, id)
	for i, oid := range m.order {
		if oid == id {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}

	return nil
}

func (m *MemoryCheckpointStore) Clear(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checkpoints = make(map[string]*Checkpoint)
	m.order = make([]string, 0)
	return nil
}

func (m *MemoryCheckpointStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.checkpoints)
}

func GenerateCheckpointID() string {
	counter := atomic.AddInt64(&checkpointCounter, 1)
	return fmt.Sprintf("cp-%d-%d", time.Now().UnixNano(), counter)
}
