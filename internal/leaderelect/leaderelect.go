package leaderelect

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"solocoder-go/internal/distlock"
)

var (
	ErrElectorStopped   = errors.New("leaderelect: elector is stopped")
	ErrElectorRunning   = errors.New("leaderelect: elector is already running")
	ErrInvalidConfig    = errors.New("leaderelect: invalid configuration")
	ErrEmptyNodeID      = errors.New("leaderelect: node id is empty")
	ErrEmptyElectionKey = errors.New("leaderelect: election key is empty")
)

type NodeRole int

const (
	RoleFollower NodeRole = iota
	RoleCandidate
	RoleLeader
)

func (r NodeRole) String() string {
	switch r {
	case RoleFollower:
		return "follower"
	case RoleCandidate:
		return "candidate"
	case RoleLeader:
		return "leader"
	default:
		return "unknown"
	}
}

type ElectionEventType int

const (
	EventBecomeLeader ElectionEventType = iota
	EventBecomeFollower
	EventElectionStart
	EventElectionEnd
	EventHeartbeat
	EventLeaderLost
)

func (e ElectionEventType) String() string {
	switch e {
	case EventBecomeLeader:
		return "become_leader"
	case EventBecomeFollower:
		return "become_follower"
	case EventElectionStart:
		return "election_start"
	case EventElectionEnd:
		return "election_end"
	case EventHeartbeat:
		return "heartbeat"
	case EventLeaderLost:
		return "leader_lost"
	default:
		return "unknown"
	}
}

type ElectionEvent struct {
	Type      ElectionEventType
	NodeID    string
	Role      NodeRole
	LeaderID  string
	Term      int64
	Timestamp time.Time
}

type ElectionCallback func(event ElectionEvent)

type Config struct {
	LeaseDuration   time.Duration
	HeartbeatFactor float64
	CheckFactor     float64
}

const (
	DefaultLeaseDuration   = 5 * time.Second
	DefaultHeartbeatFactor = 0.3
	DefaultCheckFactor     = 0.5
)

func DefaultConfig() Config {
	return Config{
		LeaseDuration:   DefaultLeaseDuration,
		HeartbeatFactor: DefaultHeartbeatFactor,
		CheckFactor:     DefaultCheckFactor,
	}
}

func (c Config) Validate() error {
	if c.LeaseDuration <= 0 {
		return fmt.Errorf("%w: lease duration must be positive", ErrInvalidConfig)
	}
	if c.HeartbeatFactor <= 0 || c.HeartbeatFactor >= 1 {
		return fmt.Errorf("%w: heartbeat factor must be between 0 and 1", ErrInvalidConfig)
	}
	if c.CheckFactor <= 0 || c.CheckFactor >= 1 {
		return fmt.Errorf("%w: check factor must be between 0 and 1", ErrInvalidConfig)
	}
	if c.HeartbeatFactor >= c.CheckFactor {
		return fmt.Errorf("%w: heartbeat factor must be less than check factor", ErrInvalidConfig)
	}
	return nil
}

func (c Config) HeartbeatInterval() time.Duration {
	return time.Duration(float64(c.LeaseDuration) * c.HeartbeatFactor)
}

func (c Config) CheckInterval() time.Duration {
	return time.Duration(float64(c.LeaseDuration) * c.CheckFactor)
}

type LockBackend interface {
	TryLock(key, token string, ttl time.Duration) (bool, error)
	Heartbeat(key, token string, ttl time.Duration) error
	Unlock(key, token string) error
	GetHolder(key string) (string, int, time.Duration, error)
	IsLocked(key string) (bool, error)
}

type lockManagerBackend struct {
	lm *distlock.LockManager
}

func NewLockManagerBackend(lm *distlock.LockManager) LockBackend {
	return &lockManagerBackend{lm: lm}
}

func (b *lockManagerBackend) TryLock(key, token string, ttl time.Duration) (bool, error) {
	return b.lm.TryLock(key, token, ttl)
}

func (b *lockManagerBackend) Heartbeat(key, token string, ttl time.Duration) error {
	return b.lm.Heartbeat(key, token, ttl)
}

func (b *lockManagerBackend) Unlock(key, token string) error {
	return b.lm.Unlock(key, token)
}

func (b *lockManagerBackend) GetHolder(key string) (string, int, time.Duration, error) {
	return b.lm.GetHolder(key)
}

func (b *lockManagerBackend) IsLocked(key string) (bool, error) {
	return b.lm.IsLocked(key)
}

type LeaderElector struct {
	nodeID       string
	electionKey  string
	cfg          Config
	backend      LockBackend
	callbacks    []ElectionCallback
	callbacksMu  sync.RWMutex

	mu           sync.Mutex
	role         NodeRole
	term         int64
	leaderID     string
	running      bool
	stopped      bool
	stopCh       chan struct{}
	heartbeatCh  chan struct{}
	wg           sync.WaitGroup

	nowFunc      func() time.Time
}

func NewLeaderElector(nodeID string, electionKey string, cfg Config, backend LockBackend) (*LeaderElector, error) {
	if nodeID == "" {
		return nil, ErrEmptyNodeID
	}
	if electionKey == "" {
		return nil, ErrEmptyElectionKey
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, fmt.Errorf("%w: backend is nil", ErrInvalidConfig)
	}

	return &LeaderElector{
		nodeID:      nodeID,
		electionKey: electionKey,
		cfg:         cfg,
		backend:     backend,
		role:        RoleFollower,
		stopCh:      make(chan struct{}),
		heartbeatCh: make(chan struct{}, 1),
		nowFunc:     time.Now,
	}, nil
}

func (e *LeaderElector) RegisterCallback(cb ElectionCallback) {
	e.callbacksMu.Lock()
	defer e.callbacksMu.Unlock()
	e.callbacks = append(e.callbacks, cb)
}

func (e *LeaderElector) notify(eventType ElectionEventType) {
	e.callbacksMu.RLock()
	defer e.callbacksMu.RUnlock()

	if len(e.callbacks) == 0 {
		return
	}

	event := ElectionEvent{
		Type:      eventType,
		NodeID:    e.nodeID,
		Role:      e.role,
		LeaderID:  e.leaderID,
		Term:      e.term,
		Timestamp: e.nowFunc(),
	}

	for _, cb := range e.callbacks {
		cb(event)
	}
}

func (e *LeaderElector) Start() error {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return ErrElectorStopped
	}
	if e.running {
		e.mu.Unlock()
		return ErrElectorRunning
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	e.wg.Add(1)
	go e.runLoop()

	return nil
}

func (e *LeaderElector) Stop() {
	e.mu.Lock()
	if e.stopped {
		e.mu.Unlock()
		return
	}
	e.stopped = true
	if e.running {
		e.running = false
		close(e.stopCh)
	}
	e.mu.Unlock()

	e.wg.Wait()

	e.mu.Lock()
	wasLeader := e.role == RoleLeader
	e.role = RoleFollower
	e.mu.Unlock()

	if wasLeader {
		_ = e.backend.Unlock(e.electionKey, e.nodeID)
		e.notify(EventBecomeFollower)
	}
}

func (e *LeaderElector) Role() NodeRole {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.role
}

func (e *LeaderElector) IsLeader() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.role == RoleLeader
}

func (e *LeaderElector) LeaderID() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.leaderID
}

func (e *LeaderElector) Term() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.term
}

func (e *LeaderElector) NodeID() string {
	return e.nodeID
}

func (e *LeaderElector) runLoop() {
	defer e.wg.Done()

	checkTicker := time.NewTicker(e.cfg.CheckInterval())
	defer checkTicker.Stop()

	heartbeatTicker := time.NewTicker(e.cfg.HeartbeatInterval())
	defer heartbeatTicker.Stop()

	e.checkLeader()

	for {
		select {
		case <-e.stopCh:
			return
		case <-heartbeatTicker.C:
			if e.IsLeader() {
				e.doHeartbeat()
			}
		case <-checkTicker.C:
			e.checkLeader()
		}
	}
}

func (e *LeaderElector) doHeartbeat() {
	err := e.backend.Heartbeat(e.electionKey, e.nodeID, e.cfg.LeaseDuration)
	if err != nil {
		e.mu.Lock()
		wasLeader := e.role == RoleLeader
		e.role = RoleFollower
		e.leaderID = ""
		e.mu.Unlock()

		if wasLeader {
			e.notify(EventLeaderLost)
			e.notify(EventBecomeFollower)
		}
		return
	}

	e.notify(EventHeartbeat)
}

func (e *LeaderElector) checkLeader() {
	holder, _, _, err := e.backend.GetHolder(e.electionKey)
	if err != nil {
		if errors.Is(err, distlock.ErrLockNotHeld) || errors.Is(err, distlock.ErrLockExpired) {
			e.startElection()
			return
		}
		return
	}

	e.mu.Lock()
	prevLeader := e.leaderID
	e.leaderID = holder
	e.mu.Unlock()

	if prevLeader != holder && holder != "" {
		e.notify(EventElectionEnd)
	}

	if e.IsLeader() {
		return
	}

	if holder == e.nodeID {
		e.becomeLeader()
	}
}

func (e *LeaderElector) startElection() {
	e.mu.Lock()
	if e.role == RoleLeader {
		e.mu.Unlock()
		return
	}
	e.role = RoleCandidate
	e.term++
	e.mu.Unlock()

	e.notify(EventElectionStart)

	success, err := e.backend.TryLock(e.electionKey, e.nodeID, e.cfg.LeaseDuration)
	if err != nil {
		e.mu.Lock()
		e.role = RoleFollower
		e.mu.Unlock()
		e.notify(EventBecomeFollower)
		return
	}

	if success {
		e.becomeLeader()
	} else {
		e.mu.Lock()
		e.role = RoleFollower
		e.leaderID = ""
		e.mu.Unlock()

		holder, _, _, err := e.backend.GetHolder(e.electionKey)
		if err == nil && holder != "" {
			e.mu.Lock()
			e.leaderID = holder
			e.mu.Unlock()
		}
		e.notify(EventElectionEnd)
		e.notify(EventBecomeFollower)
	}
}

func (e *LeaderElector) becomeLeader() {
	e.mu.Lock()
	wasLeader := e.role == RoleLeader
	e.role = RoleLeader
	e.leaderID = e.nodeID
	e.mu.Unlock()

	if !wasLeader {
		e.notify(EventElectionEnd)
		e.notify(EventBecomeLeader)
	}
}

func (e *LeaderElector) Resign() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return ErrElectorStopped
	}
	isLeader := e.role == RoleLeader
	e.mu.Unlock()

	if !isLeader {
		return nil
	}

	err := e.backend.Unlock(e.electionKey, e.nodeID)
	if err != nil && !errors.Is(err, distlock.ErrLockNotHeld) {
		return err
	}

	e.mu.Lock()
	e.role = RoleFollower
	e.leaderID = ""
	e.mu.Unlock()

	e.notify(EventLeaderLost)
	e.notify(EventBecomeFollower)

	return nil
}

func (e *LeaderElector) Running() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}
