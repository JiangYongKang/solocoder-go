package shardkv

import (
	"errors"
	"sync"
)

var (
	ErrKeyNotFound  = errors.New("key not found")
	ErrShardDown    = errors.New("shard is down")
	ErrNoAvailable  = errors.New("no available shard")
	ErrQuorumFailed = errors.New("quorum write failed")
)

type ShardStatus int

const (
	ShardStatusUp   ShardStatus = iota
	ShardStatusDown
	ShardStatusMigrating
)

type Shard struct {
	mu     sync.RWMutex
	id     string
	status ShardStatus
	data   map[string][]byte
}

func NewShard(id string) *Shard {
	return &Shard{
		id:     id,
		status: ShardStatusUp,
		data:   make(map[string][]byte),
	}
}

func (s *Shard) ID() string {
	return s.id
}

func (s *Shard) Status() ShardStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

func (s *Shard) SetStatus(status ShardStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = status
}

func (s *Shard) Get(key string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.status == ShardStatusDown {
		return nil, ErrShardDown
	}

	val, exists := s.data[key]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return val, nil
}

func (s *Shard) Put(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == ShardStatusDown {
		return ErrShardDown
	}

	s.data[key] = make([]byte, len(value))
	copy(s.data[key], value)
	return nil
}

func (s *Shard) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == ShardStatusDown {
		return ErrShardDown
	}

	if _, exists := s.data[key]; !exists {
		return ErrKeyNotFound
	}
	delete(s.data, key)
	return nil
}

func (s *Shard) HasKey(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.data[key]
	return exists
}

func (s *Shard) GetAllKeys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func (s *Shard) GetAllData() map[string][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := make(map[string][]byte, len(s.data))
	for k, v := range s.data {
		data[k] = make([]byte, len(v))
		copy(data[k], v)
	}
	return data
}

func (s *Shard) DataCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.data)
}

func (s *Shard) ForcePut(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = make([]byte, len(value))
	copy(s.data[key], value)
}

func (s *Shard) ForceDelete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}
