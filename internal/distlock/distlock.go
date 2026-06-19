package distlock

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrLockNotHeld        = errors.New("distlock: lock is not held")
	ErrLockAlreadyHeld    = errors.New("distlock: lock is already held by another token")
	ErrInvalidToken       = errors.New("distlock: token mismatch, cannot release lock")
	ErrLockExpired        = errors.New("distlock: lock has expired")
	ErrEmptyKey           = errors.New("distlock: lock key is empty")
	ErrEmptyToken         = errors.New("distlock: token is empty")
	ErrInvalidTTL         = errors.New("distlock: ttl must be positive")
	ErrMaxReentrancy      = errors.New("distlock: max reentrancy count exceeded")
	ErrNodeFailure        = errors.New("distlock: node operation failed")
	ErrQuorumNotReached   = errors.New("distlock: could not acquire lock on majority of nodes")
	ErrInvalidNodeCount   = errors.New("distlock: node count must be odd and positive")
	ErrLockManagerStopped = errors.New("distlock: lock manager is stopped")
)

const (
	DefaultMaxReentrancy = 32
	DefaultCleanInterval = 100 * time.Millisecond
)

type lockEntry struct {
	key        string
	token      string
	expiresAt  time.Time
	reentrancy int
}

func (e *lockEntry) isExpired(now time.Time) bool {
	return now.After(e.expiresAt)
}

func (e *lockEntry) remainingTTL(now time.Time) time.Duration {
	d := e.expiresAt.Sub(now)
	if d < 0 {
		return 0
	}
	return d
}

type LockManagerConfig struct {
	MaxReentrancy int
	CleanInterval time.Duration
}

func DefaultLockManagerConfig() LockManagerConfig {
	return LockManagerConfig{
		MaxReentrancy: DefaultMaxReentrancy,
		CleanInterval: DefaultCleanInterval,
	}
}

type LockManager struct {
	cfg     LockManagerConfig
	mu      sync.Mutex
	locks   map[string]*lockEntry
	running bool
	stopped bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	nowFunc func() time.Time
}

func NewLockManager() *LockManager {
	lm, _ := NewLockManagerWithConfig(DefaultLockManagerConfig())
	return lm
}

func NewLockManagerWithConfig(cfg LockManagerConfig) (*LockManager, error) {
	if cfg.MaxReentrancy < 1 {
		cfg.MaxReentrancy = DefaultMaxReentrancy
	}
	if cfg.CleanInterval < 0 {
		return nil, ErrInvalidTTL
	}
	if cfg.CleanInterval == 0 {
		cfg.CleanInterval = DefaultCleanInterval
	}

	lm := &LockManager{
		cfg:     cfg,
		locks:   make(map[string]*lockEntry),
		stopCh:  make(chan struct{}),
		nowFunc: time.Now,
	}
	return lm, nil
}

func (lm *LockManager) Start() {
	lm.mu.Lock()
	if lm.stopped {
		lm.mu.Unlock()
		return
	}
	if lm.running {
		lm.mu.Unlock()
		return
	}
	lm.running = true
	lm.stopCh = make(chan struct{})
	lm.mu.Unlock()

	lm.wg.Add(1)
	go lm.cleanLoop()
}

func (lm *LockManager) Stop() {
	lm.mu.Lock()
	if lm.stopped {
		lm.mu.Unlock()
		return
	}
	lm.stopped = true
	if lm.running {
		lm.running = false
		close(lm.stopCh)
	}
	lm.mu.Unlock()

	lm.wg.Wait()
}

func (lm *LockManager) Lock(key, token string, ttl time.Duration) error {
	return lm.lockInternal(key, token, ttl, false)
}

func (lm *LockManager) TryLock(key, token string, ttl time.Duration) (bool, error) {
	err := lm.lockInternal(key, token, ttl, true)
	if err != nil {
		if errors.Is(err, ErrLockAlreadyHeld) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (lm *LockManager) lockInternal(key, token string, ttl time.Duration, tryOnly bool) error {
	if key == "" {
		return ErrEmptyKey
	}
	if token == "" {
		return ErrEmptyToken
	}
	if ttl <= 0 {
		return ErrInvalidTTL
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return ErrLockManagerStopped
	}

	now := lm.nowFunc()
	entry, exists := lm.locks[key]

	if !exists || entry.isExpired(now) {
		lm.locks[key] = &lockEntry{
			key:        key,
			token:      token,
			expiresAt:  now.Add(ttl),
			reentrancy: 1,
		}
		return nil
	}

	if entry.token == token {
		if entry.reentrancy >= lm.cfg.MaxReentrancy {
			return ErrMaxReentrancy
		}
		entry.reentrancy++
		entry.expiresAt = now.Add(ttl)
		return nil
	}

	if tryOnly {
		return ErrLockAlreadyHeld
	}
	return ErrLockAlreadyHeld
}

func (lm *LockManager) Unlock(key, token string) error {
	if key == "" {
		return ErrEmptyKey
	}
	if token == "" {
		return ErrEmptyToken
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return ErrLockManagerStopped
	}

	entry, exists := lm.locks[key]
	if !exists {
		return ErrLockNotHeld
	}

	now := lm.nowFunc()
	if entry.isExpired(now) {
		delete(lm.locks, key)
		return ErrLockExpired
	}

	if entry.token != token {
		return ErrInvalidToken
	}

	entry.reentrancy--
	if entry.reentrancy <= 0 {
		delete(lm.locks, key)
	}

	return nil
}

func (lm *LockManager) Heartbeat(key, token string, ttl time.Duration) error {
	if key == "" {
		return ErrEmptyKey
	}
	if token == "" {
		return ErrEmptyToken
	}
	if ttl <= 0 {
		return ErrInvalidTTL
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return ErrLockManagerStopped
	}

	entry, exists := lm.locks[key]
	if !exists {
		return ErrLockNotHeld
	}

	now := lm.nowFunc()
	if entry.isExpired(now) {
		delete(lm.locks, key)
		return ErrLockExpired
	}

	if entry.token != token {
		return ErrInvalidToken
	}

	entry.expiresAt = now.Add(ttl)
	return nil
}

func (lm *LockManager) IsLocked(key string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return false, ErrLockManagerStopped
	}

	entry, exists := lm.locks[key]
	if !exists {
		return false, nil
	}

	now := lm.nowFunc()
	if entry.isExpired(now) {
		delete(lm.locks, key)
		return false, nil
	}

	return true, nil
}

func (lm *LockManager) GetHolder(key string) (string, int, time.Duration, error) {
	if key == "" {
		return "", 0, 0, ErrEmptyKey
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return "", 0, 0, ErrLockManagerStopped
	}

	entry, exists := lm.locks[key]
	if !exists {
		return "", 0, 0, ErrLockNotHeld
	}

	now := lm.nowFunc()
	if entry.isExpired(now) {
		delete(lm.locks, key)
		return "", 0, 0, ErrLockExpired
	}

	return entry.token, entry.reentrancy, entry.remainingTTL(now), nil
}

func (lm *LockManager) ForceUnlock(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return ErrLockManagerStopped
	}

	delete(lm.locks, key)
	return nil
}

func (lm *LockManager) Count() (int, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return 0, ErrLockManagerStopped
	}

	now := lm.nowFunc()
	count := 0
	for _, entry := range lm.locks {
		if !entry.isExpired(now) {
			count++
		}
	}
	return count, nil
}

func (lm *LockManager) Clear() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return ErrLockManagerStopped
	}

	lm.locks = make(map[string]*lockEntry)
	return nil
}

func (lm *LockManager) CleanExpired() (int, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	if lm.stopped {
		return 0, ErrLockManagerStopped
	}

	return lm.cleanExpiredLocked(), nil
}

func (lm *LockManager) cleanExpiredLocked() int {
	now := lm.nowFunc()
	cleaned := 0
	for key, entry := range lm.locks {
		if entry.isExpired(now) {
			delete(lm.locks, key)
			cleaned++
		}
	}
	return cleaned
}

func (lm *LockManager) cleanLoop() {
	defer lm.wg.Done()

	ticker := time.NewTicker(lm.cfg.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lm.stopCh:
			return
		case <-ticker.C:
			_, _ = lm.CleanExpired()
		}
	}
}

type LockNode interface {
	Lock(key, token string, ttl time.Duration) error
	TryLock(key, token string, ttl time.Duration) (bool, error)
	Unlock(key, token string) error
	Heartbeat(key, token string, ttl time.Duration) error
	IsLocked(key string) (bool, error)
	GetRemainingTTL(key string) (time.Duration, error)
	ID() string
}

type MemoryLockNode struct {
	id      string
	manager *LockManager
}

func NewMemoryLockNode(id string) *MemoryLockNode {
	return &MemoryLockNode{
		id:      id,
		manager: NewLockManager(),
	}
}

func NewMemoryLockNodeWithConfig(id string, cfg LockManagerConfig) (*MemoryLockNode, error) {
	mgr, err := NewLockManagerWithConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &MemoryLockNode{
		id:      id,
		manager: mgr,
	}, nil
}

func (n *MemoryLockNode) ID() string {
	return n.id
}

func (n *MemoryLockNode) Lock(key, token string, ttl time.Duration) error {
	return n.manager.Lock(key, token, ttl)
}

func (n *MemoryLockNode) TryLock(key, token string, ttl time.Duration) (bool, error) {
	return n.manager.TryLock(key, token, ttl)
}

func (n *MemoryLockNode) Unlock(key, token string) error {
	return n.manager.Unlock(key, token)
}

func (n *MemoryLockNode) Heartbeat(key, token string, ttl time.Duration) error {
	return n.manager.Heartbeat(key, token, ttl)
}

func (n *MemoryLockNode) IsLocked(key string) (bool, error) {
	return n.manager.IsLocked(key)
}

func (n *MemoryLockNode) GetRemainingTTL(key string) (time.Duration, error) {
	_, _, ttl, err := n.manager.GetHolder(key)
	return ttl, err
}

type RedlockConfig struct {
	AcquireTimeout time.Duration
	RetryDelay     time.Duration
	ClockDrift     time.Duration
}

func DefaultRedlockConfig() RedlockConfig {
	return RedlockConfig{
		AcquireTimeout: 5 * time.Second,
		RetryDelay:     100 * time.Millisecond,
		ClockDrift:     50 * time.Millisecond,
	}
}

type LockAcquisition struct {
	Key           string
	Token         string
	Expiry        time.Time
	NodeExpiries  map[string]time.Time
	NodeCount     int
	SuccessCount  int
}

func (la *LockAcquisition) IsValid() bool {
	return la != nil && la.Expiry.After(time.Now())
}

func (la *LockAcquisition) RemainingTTL() time.Duration {
	if la == nil {
		return 0
	}
	d := la.Expiry.Sub(time.Now())
	if d < 0 {
		return 0
	}
	return d
}

type Redlock struct {
	cfg   RedlockConfig
	nodes []LockNode
}

func NewRedlock(nodes []LockNode) (*Redlock, error) {
	return NewRedlockWithConfig(nodes, DefaultRedlockConfig())
}

func NewRedlockWithConfig(nodes []LockNode, cfg RedlockConfig) (*Redlock, error) {
	if len(nodes) == 0 {
		return nil, ErrInvalidNodeCount
	}
	if len(nodes)%2 == 0 {
		return nil, ErrInvalidNodeCount
	}
	if cfg.AcquireTimeout < 0 {
		return nil, ErrInvalidTTL
	}
	if cfg.RetryDelay < 0 {
		return nil, ErrInvalidTTL
	}
	if cfg.ClockDrift < 0 {
		return nil, ErrInvalidTTL
	}
	if cfg.AcquireTimeout == 0 {
		cfg.AcquireTimeout = DefaultRedlockConfig().AcquireTimeout
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = DefaultRedlockConfig().RetryDelay
	}
	return &Redlock{
		cfg:   cfg,
		nodes: append([]LockNode(nil), nodes...),
	}, nil
}

func (r *Redlock) majority() int {
	return len(r.nodes)/2 + 1
}

func (r *Redlock) NodeCount() int {
	return len(r.nodes)
}

func (r *Redlock) Lock(key, token string, ttl time.Duration) (*LockAcquisition, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	if token == "" {
		return nil, ErrEmptyToken
	}
	if ttl <= 0 {
		return nil, ErrInvalidTTL
	}

	deadline := time.Now().Add(r.cfg.AcquireTimeout)

	for {
		if time.Now().After(deadline) {
			return nil, ErrQuorumNotReached
		}

		acq, err := r.attemptLock(key, token, ttl)
		if err == nil {
			return acq, nil
		}

		r.rollbackPartialLock(key, token, acq)

		if time.Now().After(deadline) {
			return nil, ErrQuorumNotReached
		}

		time.Sleep(r.cfg.RetryDelay)
	}
}

func (r *Redlock) TryLock(key, token string, ttl time.Duration) (*LockAcquisition, error) {
	if key == "" {
		return nil, ErrEmptyKey
	}
	if token == "" {
		return nil, ErrEmptyToken
	}
	if ttl <= 0 {
		return nil, ErrInvalidTTL
	}

	acq, err := r.attemptLock(key, token, ttl)
	if err != nil {
		r.rollbackPartialLock(key, token, acq)
		return nil, err
	}
	return acq, nil
}

func (r *Redlock) attemptLock(key, token string, ttl time.Duration) (*LockAcquisition, error) {
	nodeExpiries := make(map[string]time.Time)
	successCount := 0

	for _, node := range r.nodes {
		err := node.Lock(key, token, ttl)
		if err == nil {
			successCount++
			nodeExpiries[node.ID()] = time.Now().Add(ttl)
		}
	}

	if successCount < r.majority() {
		acq := &LockAcquisition{
			Key:          key,
			Token:        token,
			NodeExpiries: nodeExpiries,
			NodeCount:    len(r.nodes),
			SuccessCount: successCount,
		}
		return acq, ErrQuorumNotReached
	}

	var minExpiry time.Time
	first := true
	for _, exp := range nodeExpiries {
		if first || exp.Before(minExpiry) {
			minExpiry = exp
			first = false
		}
	}

	minExpiry = minExpiry.Add(-r.cfg.ClockDrift)

	if !minExpiry.After(time.Now()) {
		acq := &LockAcquisition{
			Key:          key,
			Token:        token,
			NodeExpiries: nodeExpiries,
			NodeCount:    len(r.nodes),
			SuccessCount: successCount,
		}
		return acq, ErrQuorumNotReached
	}

	return &LockAcquisition{
		Key:           key,
		Token:         token,
		Expiry:        minExpiry,
		NodeExpiries:  nodeExpiries,
		NodeCount:     len(r.nodes),
		SuccessCount:  successCount,
	}, nil
}

func (r *Redlock) rollbackPartialLock(key, token string, acq *LockAcquisition) {
	if acq == nil {
		for _, node := range r.nodes {
			_ = node.Unlock(key, token)
		}
		return
	}
	for nodeID := range acq.NodeExpiries {
		for _, node := range r.nodes {
			if node.ID() == nodeID {
				_ = node.Unlock(key, token)
				break
			}
		}
	}
}

func (r *Redlock) Unlock(acq *LockAcquisition) error {
	if acq == nil {
		return ErrLockNotHeld
	}
	if acq.Key == "" {
		return ErrEmptyKey
	}
	if acq.Token == "" {
		return ErrEmptyToken
	}

	var lastErr error
	successCount := 0
	for _, node := range r.nodes {
		err := node.Unlock(acq.Key, acq.Token)
		if err == nil {
			successCount++
		} else {
			lastErr = err
		}
	}

	if successCount == 0 && lastErr != nil {
		return fmt.Errorf("%w: %v", ErrNodeFailure, lastErr)
	}
	return nil
}

func (r *Redlock) Heartbeat(acq *LockAcquisition, ttl time.Duration) (*LockAcquisition, error) {
	if acq == nil {
		return nil, ErrLockNotHeld
	}
	if acq.Key == "" {
		return nil, ErrEmptyKey
	}
	if acq.Token == "" {
		return nil, ErrEmptyToken
	}
	if ttl <= 0 {
		return nil, ErrInvalidTTL
	}

	nodeExpiries := make(map[string]time.Time)
	successCount := 0

	for nodeID := range acq.NodeExpiries {
		for _, node := range r.nodes {
			if node.ID() == nodeID {
				err := node.Heartbeat(acq.Key, acq.Token, ttl)
				if err == nil {
					successCount++
					nodeExpiries[node.ID()] = time.Now().Add(ttl)
				}
				break
			}
		}
	}

	if successCount < r.majority() {
		return nil, ErrQuorumNotReached
	}

	var minExpiry time.Time
	first := true
	for _, exp := range nodeExpiries {
		if first || exp.Before(minExpiry) {
			minExpiry = exp
			first = false
		}
	}

	minExpiry = minExpiry.Add(-r.cfg.ClockDrift)

	if !minExpiry.After(time.Now()) {
		return nil, ErrQuorumNotReached
	}

	return &LockAcquisition{
		Key:           acq.Key,
		Token:         acq.Token,
		Expiry:        minExpiry,
		NodeExpiries:  nodeExpiries,
		NodeCount:     len(r.nodes),
		SuccessCount:  successCount,
	}, nil
}

func (r *Redlock) IsLocked(key string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	count := 0
	for _, node := range r.nodes {
		locked, err := node.IsLocked(key)
		if err == nil && locked {
			count++
		}
	}
	return count >= r.majority(), nil
}
