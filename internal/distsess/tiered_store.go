package distsess

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type TieredStore struct {
	hitCount       uint64
	missCount      uint64
	expiredCount   uint64
	memoryStore    map[string]*Session
	persistence    PersistenceStore
	mu             sync.RWMutex
	autoRenew      bool
	defaultTTL     time.Duration
	nodeID         string
}

func NewTieredStore(cfg Config, persistence PersistenceStore) (*TieredStore, error) {
	if persistence == nil {
		return nil, fmt.Errorf("%w: persistence store cannot be nil", ErrInvalidConfig)
	}

	ts := &TieredStore{
		memoryStore: make(map[string]*Session),
		persistence: persistence,
		autoRenew:   cfg.AutoRenew,
		defaultTTL:  cfg.DefaultTTL,
		nodeID:      cfg.NodeID,
	}

	sessions, err := persistence.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("%w: failed to load sessions from persistence: %v", ErrPersistenceFailed, err)
	}

	now := time.Now()
	for _, s := range sessions {
		if s.TTL > 0 && now.After(s.ExpiresAt) {
			_ = persistence.Delete(s.ID)
			atomic.AddUint64(&ts.expiredCount, 1)
			continue
		}
		ts.memoryStore[s.ID] = s
	}

	return ts, nil
}

func (ts *TieredStore) Get(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	ts.mu.RLock()
	session, ok := ts.memoryStore[sessionID]
	ts.mu.RUnlock()

	if ok {
		atomic.AddUint64(&ts.hitCount, 1)
		if session.IsExpired() {
			ts.mu.Lock()
			delete(ts.memoryStore, sessionID)
			ts.mu.Unlock()
			_ = ts.persistence.Delete(sessionID)
			atomic.AddUint64(&ts.expiredCount, 1)
			return nil, ErrSessionExpired
		}
		if ts.autoRenew {
			sessionCopy := session.DeepCopy()
			sessionCopy.Renew()
			ts.mu.Lock()
			ts.memoryStore[sessionID] = sessionCopy
			ts.mu.Unlock()
			_ = ts.persistence.Save(sessionCopy)
			return sessionCopy, nil
		}
		return session.DeepCopy(), nil
	}

	atomic.AddUint64(&ts.missCount, 1)

	session, err := ts.persistence.Load(sessionID)
	if err != nil {
		return nil, err
	}

	if session.IsExpired() {
		_ = ts.persistence.Delete(sessionID)
		atomic.AddUint64(&ts.expiredCount, 1)
		return nil, ErrSessionExpired
	}

	if ts.autoRenew {
		session.Renew()
		_ = ts.persistence.Save(session)
	}

	ts.mu.Lock()
	ts.memoryStore[sessionID] = session
	ts.mu.Unlock()

	return session.DeepCopy(), nil
}

func (ts *TieredStore) GetWithoutRenew(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	ts.mu.RLock()
	session, ok := ts.memoryStore[sessionID]
	ts.mu.RUnlock()

	if ok {
		if session.IsExpired() {
			ts.mu.Lock()
			delete(ts.memoryStore, sessionID)
			ts.mu.Unlock()
			_ = ts.persistence.Delete(sessionID)
			atomic.AddUint64(&ts.expiredCount, 1)
			return nil, ErrSessionExpired
		}
		return session.DeepCopy(), nil
	}

	session, err := ts.persistence.Load(sessionID)
	if err != nil {
		return nil, err
	}

	if session.IsExpired() {
		_ = ts.persistence.Delete(sessionID)
		atomic.AddUint64(&ts.expiredCount, 1)
		return nil, ErrSessionExpired
	}

	ts.mu.Lock()
	ts.memoryStore[sessionID] = session
	ts.mu.Unlock()

	return session.DeepCopy(), nil
}

func (ts *TieredStore) Set(session *Session) error {
	if session == nil {
		return ErrNilSessionData
	}
	if session.ID == "" {
		return ErrEmptySessionID
	}

	session.NodeID = ts.nodeID
	session.UpdatedAt = time.Now()

	ts.mu.Lock()
	existing, ok := ts.memoryStore[session.ID]
	if ok {
		session.Version = existing.Version + 1
	} else {
		session.Version = 1
	}
	ts.memoryStore[session.ID] = session
	ts.mu.Unlock()

	if err := ts.persistence.Save(session); err != nil {
		ts.mu.Lock()
		if ok {
			ts.memoryStore[session.ID] = existing
		} else {
			delete(ts.memoryStore, session.ID)
		}
		ts.mu.Unlock()
		return err
	}

	return nil
}

func (ts *TieredStore) SetWithTTL(sessionID string, data SessionData, ttl time.Duration) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}
	if data == nil {
		return nil, ErrNilSessionData
	}

	if ttl <= 0 {
		ttl = ts.defaultTTL
	}

	now := time.Now()
	session := &Session{
		ID:        sessionID,
		Data:      data,
		TTL:       ttl,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
		UpdatedAt: now,
		NodeID:    ts.nodeID,
	}

	ts.mu.Lock()
	existing, ok := ts.memoryStore[sessionID]
	if ok {
		session.CreatedAt = existing.CreatedAt
		session.Version = existing.Version + 1
	} else {
		session.Version = 1
	}
	ts.memoryStore[sessionID] = session
	ts.mu.Unlock()

	if err := ts.persistence.Save(session); err != nil {
		ts.mu.Lock()
		if ok {
			ts.memoryStore[sessionID] = existing
		} else {
			delete(ts.memoryStore, sessionID)
		}
		ts.mu.Unlock()
		return nil, err
	}

	return session.DeepCopy(), nil
}

func (ts *TieredStore) Delete(sessionID string) (bool, error) {
	if sessionID == "" {
		return false, ErrEmptySessionID
	}

	ts.mu.Lock()
	_, existed := ts.memoryStore[sessionID]
	delete(ts.memoryStore, sessionID)
	ts.mu.Unlock()

	if err := ts.persistence.Delete(sessionID); err != nil {
		return false, err
	}

	return existed, nil
}

func (ts *TieredStore) Renew(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	ts.mu.RLock()
	session, ok := ts.memoryStore[sessionID]
	ts.mu.RUnlock()

	if !ok {
		var err error
		session, err = ts.persistence.Load(sessionID)
		if err != nil {
			return nil, err
		}
	}

	if session.IsExpired() {
		ts.mu.Lock()
		delete(ts.memoryStore, sessionID)
		ts.mu.Unlock()
		_ = ts.persistence.Delete(sessionID)
		atomic.AddUint64(&ts.expiredCount, 1)
		return nil, ErrSessionExpired
	}

	sessionCopy := session.DeepCopy()
	sessionCopy.Renew()

	ts.mu.Lock()
	ts.memoryStore[sessionID] = sessionCopy
	ts.mu.Unlock()

	if err := ts.persistence.Save(sessionCopy); err != nil {
		return nil, err
	}

	return sessionCopy, nil
}

func (ts *TieredStore) CleanupExpired() int {
	ts.mu.Lock()
	now := time.Now()
	expired := make([]string, 0)
	for id, session := range ts.memoryStore {
		if session.TTL > 0 && now.After(session.ExpiresAt) {
			expired = append(expired, id)
		}
	}
	ts.mu.Unlock()

	for _, id := range expired {
		ts.mu.Lock()
		delete(ts.memoryStore, id)
		ts.mu.Unlock()
		_ = ts.persistence.Delete(id)
		atomic.AddUint64(&ts.expiredCount, 1)
	}

	return len(expired)
}

func (ts *TieredStore) Exists(sessionID string) bool {
	if sessionID == "" {
		return false
	}

	ts.mu.RLock()
	_, ok := ts.memoryStore[sessionID]
	ts.mu.RUnlock()

	if ok {
		return true
	}

	_, err := ts.persistence.Load(sessionID)
	return err == nil
}

func (ts *TieredStore) GetAll() ([]*Session, error) {
	ts.mu.RLock()
	sessions := make([]*Session, 0, len(ts.memoryStore))
	for _, s := range ts.memoryStore {
		sessions = append(sessions, s.DeepCopy())
	}
	ts.mu.RUnlock()

	return sessions, nil
}

func (ts *TieredStore) GetMemoryCount() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.memoryStore)
}

func (ts *TieredStore) GetPersistedCount() (int, error) {
	return ts.persistence.Count()
}

func (ts *TieredStore) Clear() error {
	ts.mu.Lock()
	ts.memoryStore = make(map[string]*Session)
	ts.mu.Unlock()

	return ts.persistence.Clear()
}

func (ts *TieredStore) Close() error {
	return ts.persistence.Close()
}

func (ts *TieredStore) mergeRemoteSession(remote *Session) bool {
	if remote == nil {
		return false
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	local, ok := ts.memoryStore[remote.ID]
	if !ok {
		ts.memoryStore[remote.ID] = remote.DeepCopy()
		_ = ts.persistence.Save(remote)
		return true
	}

	if remote.Version > local.Version {
		ts.memoryStore[remote.ID] = remote.DeepCopy()
		_ = ts.persistence.Save(remote)
		return true
	}

	return false
}

func (ts *TieredStore) applyRemoteDelete(sessionID string, version uint64) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	local, ok := ts.memoryStore[sessionID]
	if !ok {
		return false
	}

	if version >= local.Version {
		delete(ts.memoryStore, sessionID)
		_ = ts.persistence.Delete(sessionID)
		return true
	}

	return false
}

func (ts *TieredStore) getStats() Stats {
	persistedCount, _ := ts.persistence.Count()
	return Stats{
		MemoryCount:    ts.GetMemoryCount(),
		PersistedCount: persistedCount,
		ExpiredCount:   atomic.LoadUint64(&ts.expiredCount),
		HitCount:       atomic.LoadUint64(&ts.hitCount),
		MissCount:      atomic.LoadUint64(&ts.missCount),
	}
}
