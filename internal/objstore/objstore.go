package objstore

import (
	"errors"
	"sort"
	"sync"
	"time"
)

var (
	ErrKeyNotFound            = errors.New("key not found")
	ErrVersionNotFound        = errors.New("version not found")
	ErrInvalidMaxVersion      = errors.New("invalid max versions: must be positive")
	ErrNilData                = errors.New("nil data")
	ErrEmptyKey               = errors.New("empty key")
	ErrInvalidBatchSize       = errors.New("invalid cleanup batch size: must be positive")
	ErrInvalidCleanupInterval = errors.New("invalid cleanup interval: must be positive")
)

type ObjectVersion struct {
	Version   uint64
	Data      []byte
	CreatedAt time.Time
}

type VersionInfo struct {
	Version   uint64
	CreatedAt time.Time
}

type Config struct {
	MaxVersions      int
	CleanupBatchSize int
	CleanupInterval  int
}

type ObjectStore struct {
	mu              sync.RWMutex
	objects         map[string][]*ObjectVersion
	config          Config
	opsSinceCleanup int
}

func DefaultConfig() Config {
	return Config{
		MaxVersions:      10,
		CleanupBatchSize: 1,
		CleanupInterval:  1,
	}
}

func NewObjectStore() *ObjectStore {
	store, _ := NewObjectStoreWithConfig(DefaultConfig())
	return store
}

func NewObjectStoreWithConfig(cfg Config) (*ObjectStore, error) {
	if cfg.MaxVersions <= 0 {
		return nil, ErrInvalidMaxVersion
	}
	if cfg.CleanupBatchSize <= 0 {
		return nil, ErrInvalidBatchSize
	}
	if cfg.CleanupInterval <= 0 {
		return nil, ErrInvalidCleanupInterval
	}

	return &ObjectStore{
		objects: make(map[string][]*ObjectVersion),
		config:  cfg,
	}, nil
}

func (s *ObjectStore) Put(key string, data []byte) (uint64, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}
	if data == nil {
		return 0, ErrNilData
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	versions := s.objects[key]
	var nextVersion uint64
	if len(versions) > 0 {
		nextVersion = versions[len(versions)-1].Version + 1
	} else {
		nextVersion = 1
	}

	ov := &ObjectVersion{
		Version:   nextVersion,
		Data:      make([]byte, len(data)),
		CreatedAt: time.Now(),
	}
	copy(ov.Data, data)

	versions = append(versions, ov)
	s.objects[key] = versions
	s.opsSinceCleanup++

	s.maybeCleanupLocked()

	return nextVersion, nil
}

func (s *ObjectStore) Get(key string) ([]byte, uint64, error) {
	if key == "" {
		return nil, 0, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.objects[key]
	if !ok || len(versions) == 0 {
		return nil, 0, ErrKeyNotFound
	}

	latest := versions[len(versions)-1]
	data := make([]byte, len(latest.Data))
	copy(data, latest.Data)
	return data, latest.Version, nil
}

func (s *ObjectStore) GetVersion(key string, version uint64) ([]byte, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.objects[key]
	if !ok || len(versions) == 0 {
		return nil, ErrKeyNotFound
	}

	v := s.findVersionLocked(versions, version)
	if v == nil {
		return nil, ErrVersionNotFound
	}

	data := make([]byte, len(v.Data))
	copy(data, v.Data)
	return data, nil
}

func (s *ObjectStore) ListVersions(key string) ([]VersionInfo, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.objects[key]
	if !ok || len(versions) == 0 {
		return nil, ErrKeyNotFound
	}

	result := make([]VersionInfo, len(versions))
	for i, v := range versions {
		result[i] = VersionInfo{
			Version:   v.Version,
			CreatedAt: v.CreatedAt,
		}
	}

	return result, nil
}

func (s *ObjectStore) Rollback(key string, version uint64) (uint64, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	versions, ok := s.objects[key]
	if !ok || len(versions) == 0 {
		return 0, ErrKeyNotFound
	}

	target := s.findVersionLocked(versions, version)
	if target == nil {
		return 0, ErrVersionNotFound
	}

	latestVersion := versions[len(versions)-1].Version
	newVersion := latestVersion + 1

	ov := &ObjectVersion{
		Version:   newVersion,
		Data:      make([]byte, len(target.Data)),
		CreatedAt: time.Now(),
	}
	copy(ov.Data, target.Data)

	versions = append(versions, ov)
	s.objects[key] = versions
	s.opsSinceCleanup++

	s.maybeCleanupLocked()

	return newVersion, nil
}

func (s *ObjectStore) Delete(key string) bool {
	if key == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.objects[key]
	if ok {
		delete(s.objects, key)
	}

	return ok
}

func (s *ObjectStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.objects)
}

func (s *ObjectStore) VersionCount(key string) (int, error) {
	if key == "" {
		return 0, ErrEmptyKey
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, ok := s.objects[key]
	if !ok {
		return 0, ErrKeyNotFound
	}

	return len(versions), nil
}

func (s *ObjectStore) CleanupAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	totalCleaned := 0
	for key := range s.objects {
		totalCleaned += s.cleanupKeyAllLocked(key)
	}
	s.opsSinceCleanup = 0

	return totalCleaned
}

func (s *ObjectStore) findVersionLocked(versions []*ObjectVersion, target uint64) *ObjectVersion {
	idx := sort.Search(len(versions), func(i int) bool {
		return versions[i].Version >= target
	})
	if idx < len(versions) && versions[idx].Version == target {
		return versions[idx]
	}
	return nil
}

func (s *ObjectStore) maybeCleanupLocked() {
	if s.opsSinceCleanup < s.config.CleanupInterval {
		return
	}

	for key := range s.objects {
		s.cleanupKeyLocked(key, s.config.CleanupBatchSize)
	}

	s.opsSinceCleanup = 0
}

func (s *ObjectStore) cleanupKeyLocked(key string, batchSize int) int {
	versions := s.objects[key]
	if len(versions) <= s.config.MaxVersions {
		return 0
	}

	excess := len(versions) - s.config.MaxVersions
	cleanCount := batchSize
	if excess < cleanCount {
		cleanCount = excess
	}

	s.objects[key] = versions[cleanCount:]
	return cleanCount
}

func (s *ObjectStore) cleanupKeyAllLocked(key string) int {
	versions := s.objects[key]
	if len(versions) <= s.config.MaxVersions {
		return 0
	}

	excess := len(versions) - s.config.MaxVersions
	s.objects[key] = versions[excess:]
	return excess
}
