package benchfrm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type MemoryStore struct {
	mu     sync.RWMutex
	values map[string]GroupStatistics
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		values: make(map[string]GroupStatistics),
	}
}

func (s *MemoryStore) Save(groupName string, stats GroupStatistics) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[groupName] = stats
	return nil
}

func (s *MemoryStore) Load(groupName string) (GroupStatistics, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats, ok := s.values[groupName]
	return stats, ok, nil
}

type FileStore struct {
	baseDir string
	mu      sync.RWMutex
}

func NewFileStore(baseDir string) (*FileStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("benchfrm: failed to create base dir: %w", err)
	}
	return &FileStore{baseDir: baseDir}, nil
}

func (s *FileStore) Save(groupName string, stats GroupStatistics) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("benchfrm: failed to marshal stats: %w", err)
	}

	filename := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", groupName))
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("benchfrm: failed to write file: %w", err)
	}

	return nil
}

func (s *FileStore) Load(groupName string) (GroupStatistics, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filename := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", groupName))
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return GroupStatistics{}, false, nil
		}
		return GroupStatistics{}, false, fmt.Errorf("benchfrm: failed to read file: %w", err)
	}

	var stats GroupStatistics
	if err := json.Unmarshal(data, &stats); err != nil {
		return GroupStatistics{}, false, fmt.Errorf("benchfrm: failed to unmarshal stats: %w", err)
	}

	return stats, true, nil
}
