package distsess

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Store struct {
	cfg             Config
	store           *TieredStore
	persistence     PersistenceStore
	changeHandlers  []ChangeHandler
	handlerMu       sync.RWMutex
	cleanupTicker   *time.Ticker
	cleanupStopCh   chan struct{}
	running         bool
	mu              sync.RWMutex
}

func NewStore(cfg Config) (*Store, error) {
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = DefaultTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCleanupInterval
	}
	if cfg.NodeID == "" {
		cfg.NodeID = generateNodeID()
	}

	persistence, err := NewFilePersistenceStore(cfg.PersistenceDir)
	if err != nil {
		return nil, err
	}

	return NewStoreWithPersistence(cfg, persistence)
}

func NewStoreWithMemoryPersistence(cfg Config) (*Store, error) {
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = DefaultTTL
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = DefaultCleanupInterval
	}
	if cfg.NodeID == "" {
		cfg.NodeID = generateNodeID()
	}

	return NewStoreWithPersistence(cfg, NewMemoryPersistenceStore())
}

func NewStoreWithPersistence(cfg Config, persistence PersistenceStore) (*Store, error) {
	if persistence == nil {
		return nil, fmt.Errorf("%w: persistence store cannot be nil", ErrInvalidConfig)
	}

	store, err := NewTieredStore(cfg, persistence)
	if err != nil {
		return nil, err
	}

	s := &Store{
		cfg:           cfg,
		store:         store,
		persistence:   persistence,
		running:       true,
		cleanupStopCh: make(chan struct{}),
	}

	if cfg.CleanupInterval > 0 {
		s.cleanupTicker = time.NewTicker(cfg.CleanupInterval)
		go s.cleanupLoop()
	}

	return s, nil
}

func (s *Store) cleanupLoop() {
	for {
		select {
		case <-s.cleanupStopCh:
			return
		case <-s.cleanupTicker.C:
			s.store.CleanupExpired()
		}
	}
}

func (s *Store) Get(sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	return s.store.Get(sessionID)
}

func (s *Store) Set(sessionID string, data SessionData, ttl ...time.Duration) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	sessionTTL := s.cfg.DefaultTTL
	if len(ttl) > 0 && ttl[0] > 0 {
		sessionTTL = ttl[0]
	}

	session, err := s.store.SetWithTTL(sessionID, data, sessionTTL)
	if err != nil {
		return nil, err
	}

	changeType := ChangeTypeCreate
	if session.Version > 1 {
		changeType = ChangeTypeUpdate
	}

	s.notifyChangeHandlers(ChangeNotification{
		Type:       changeType,
		SessionID:  sessionID,
		Version:    session.Version,
		NodeID:     s.cfg.NodeID,
		Timestamp:  time.Now(),
		DataDigest: computeDataDigest(session.Data),
		Data:       session,
	})

	return session, nil
}

func (s *Store) Delete(sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return false, ErrClusterStopped
	}

	existed, err := s.store.Delete(sessionID)
	if err != nil {
		return false, err
	}

	if existed {
		s.notifyChangeHandlers(ChangeNotification{
			Type:      ChangeTypeDelete,
			SessionID: sessionID,
			NodeID:    s.cfg.NodeID,
			Timestamp: time.Now(),
		})
	}

	return existed, nil
}

func (s *Store) Renew(sessionID string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	session, err := s.store.Renew(sessionID)
	if err != nil {
		return nil, err
	}

	s.notifyChangeHandlers(ChangeNotification{
		Type:       ChangeTypeRenew,
		SessionID:  sessionID,
		Version:    session.Version,
		NodeID:     s.cfg.NodeID,
		Timestamp:  time.Now(),
		DataDigest: computeDataDigest(session.Data),
		Data:       session,
	})

	return session, nil
}

func (s *Store) Exists(sessionID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return false
	}

	return s.store.Exists(sessionID)
}

func (s *Store) GetAll() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	return s.store.GetAll()
}

func (s *Store) AddChangeHandler(handler ChangeHandler) {
	if handler == nil {
		return
	}
	s.handlerMu.Lock()
	s.changeHandlers = append(s.changeHandlers, handler)
	s.handlerMu.Unlock()
}

func (s *Store) notifyChangeHandlers(notification ChangeNotification) {
	s.handlerMu.RLock()
	handlers := make([]ChangeHandler, len(s.changeHandlers))
	copy(handlers, s.changeHandlers)
	s.handlerMu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() { recover() }()
			h(notification)
		}()
	}
}

func (s *Store) ExportSession(sessionID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	session, err := s.store.GetWithoutRenew(sessionID)
	if err != nil {
		return nil, err
	}

	return ExportSession(session)
}

func (s *Store) ExportAll() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	sessions, err := s.store.GetAll()
	if err != nil {
		return nil, err
	}

	return exportSessions(sessions, s.cfg.NodeID)
}

func (s *Store) ImportSession(data []byte) (*MigrationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	var migrationData MigrationData
	if err := validateAndParseMigrationData(data, &migrationData); err != nil {
		return nil, err
	}

	result := &MigrationResult{
		Errors: make([]error, 0),
	}

	now := time.Now()
	for _, session := range migrationData.Sessions {
		if session == nil || session.ID == "" {
			result.SkippedCount++
			continue
		}

		if session.TTL > 0 && now.After(session.ExpiresAt) {
			result.SkippedCount++
			continue
		}

		sessionCopy := session.DeepCopy()
		_, err := s.store.SetWithTTL(sessionCopy.ID, sessionCopy.Data, sessionCopy.TTL)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, err)
			continue
		}

		result.ImportedCount++
	}

	return result, nil
}

func (s *Store) ImportAll(data []byte, overwrite bool) (*MigrationResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil, ErrClusterStopped
	}

	var migrationData MigrationData
	if err := validateAndParseMigrationData(data, &migrationData); err != nil {
		return nil, err
	}

	result := &MigrationResult{
		Errors: make([]error, 0),
	}

	now := time.Now()
	for _, session := range migrationData.Sessions {
		if session == nil || session.ID == "" {
			result.SkippedCount++
			result.Errors = append(result.Errors, fmt.Errorf("invalid session"))
			continue
		}

		if session.TTL > 0 && now.After(session.ExpiresAt) {
			result.SkippedCount++
			continue
		}

		if !overwrite && s.store.Exists(session.ID) {
			result.SkippedCount++
			continue
		}

		sessionCopy := session.DeepCopy()
		_, err := s.store.SetWithTTL(sessionCopy.ID, sessionCopy.Data, sessionCopy.TTL)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Errorf("session %s: %w", sessionCopy.ID, err))
			continue
		}

		result.ImportedCount++
	}

	return result, nil
}

func validateAndParseMigrationData(data []byte, out *MigrationData) error {
	if len(data) == 0 {
		return fmt.Errorf("%w: empty data", ErrMigrationFailed)
	}

	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("%w: failed to unmarshal: %v", ErrMigrationFailed, err)
	}

	if err := ValidateMigrationData(data); err != nil {
		return err
	}

	return nil
}

func (s *Store) CleanupExpired() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return 0
	}

	return s.store.CleanupExpired()
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.store.getStats()
}

func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return ErrClusterStopped
	}

	return s.store.Clear()
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
		close(s.cleanupStopCh)
	}

	return s.store.Close()
}

func (s *Store) GetNode() *Node {
	return nil
}

func (s *Store) GetPersistence() PersistenceStore {
	return s.persistence
}

func (s *Store) GetConfig() Config {
	return s.cfg
}

type StandaloneStore struct {
	hitCount  uint64
	missCount uint64
	*Store
}

func NewStandaloneStore(cfg Config) (*StandaloneStore, error) {
	cfg.EnableSync = false
	store, err := NewStore(cfg)
	if err != nil {
		return nil, err
	}
	return &StandaloneStore{Store: store}, nil
}

func (s *StandaloneStore) Get(sessionID string) (*Session, error) {
	session, err := s.Store.Get(sessionID)
	if err == nil {
		atomic.AddUint64(&s.hitCount, 1)
	} else if err == ErrSessionNotFound {
		atomic.AddUint64(&s.missCount, 1)
	}
	return session, err
}

func (s *StandaloneStore) HitCount() uint64 {
	return atomic.LoadUint64(&s.hitCount)
}

func (s *StandaloneStore) MissCount() uint64 {
	return atomic.LoadUint64(&s.missCount)
}
