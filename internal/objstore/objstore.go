package objstore

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrKeyNotFound       = errors.New("key not found")
	ErrVersionNotFound   = errors.New("version not found")
	ErrInvalidMaxVersion = errors.New("invalid max versions: must be positive")
	ErrNilData           = errors.New("nil data")
	ErrEmptyKey          = errors.New("empty key")
	ErrInvalidBatchSize  = errors.New("invalid cleanup batch size: must be positive")
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
	mu             sync.RWMutex
	objects        map[string][]*ObjectVersion
	config         Config
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
	return NewObjectStoreWithConfig(DefaultConfig())
}

func NewObjectStoreWithConfig(cfg Config) *ObjectStore {
	if cfg.MaxVersions <= 0 {
		cfg.MaxVersions = 10
	}
	if cfg.CleanupBatchSize <= 0 {
		cfg.CleanupBatchSize = 1
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 1
	}

	return &ObjectStore{
		objects: make(map[string][]*ObjectVersion),
		config:  cfg,
	}
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

	for _, v := range versions {
		if v.Version == version {
			data := make([]byte, len(v.Data))
			copy(data, v.Data)
			return data, nil
		}
	}

	return nil, ErrVersionNotFound
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

	var targetData []byte
	for _, v := range versions {
		if v.Version == version {
			targetData = v.Data
			break
		}
	}

	if targetData == nil {
		return 0, ErrVersionNotFound
	}

	latestVersion := versions[len(versions)-1].Version
	newVersion := latestVersion + 1

	ov := &ObjectVersion{
		Version:   newVersion,
		Data:      make([]byte, len(targetData)),
		CreatedAt: time.Now(),
	}
	copy(ov.Data, targetData)

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
		totalCleaned += s.cleanupKeyLocked(key, s.config.CleanupBatchSize)
	}
	s.opsSinceCleanup = 0

	return totalCleaned
}

func (s *ObjectStore) maybeCleanupLocked() {
	if s.opsSinceCleanup < s.config.CleanupInterval {
		return
	}

	totalCleaned := 0
	for key := range s.objects {
		totalCleaned += s.cleanupKeyLocked(key, s.config.CleanupBatchSize)
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
