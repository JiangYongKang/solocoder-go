package leaderelect

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"solocoder-go/internal/distlock"
)

type mockBackend struct {
	mu              sync.Mutex
	tryLockFn       func(key, token string, ttl time.Duration) (bool, error)
	heartbeatFn     func(key, token string, ttl time.Duration) error
	unlockFn        func(key, token string) error
	getHolderFn     func(key string) (string, int, time.Duration, error)
	isLockedFn      func(key string) (bool, error)
}

func newMockBackend() *mockBackend {
	lm := distlock.NewLockManager()
	return &mockBackend{
		tryLockFn: func(key, token string, ttl time.Duration) (bool, error) {
			return lm.TryLock(key, token, ttl)
		},
		heartbeatFn: func(key, token string, ttl time.Duration) error {
			return lm.Heartbeat(key, token, ttl)
		},
		unlockFn: func(key, token string) error {
			return lm.Unlock(key, token)
		},
		getHolderFn: func(key string) (string, int, time.Duration, error) {
			return lm.GetHolder(key)
		},
		isLockedFn: func(key string) (bool, error) {
			return lm.IsLocked(key)
		},
	}
}

func (m *mockBackend) TryLock(key, token string, ttl time.Duration) (bool, error) {
	m.mu.Lock()
	fn := m.tryLockFn
	m.mu.Unlock()
	return fn(key, token, ttl)
}

func (m *mockBackend) Heartbeat(key, token string, ttl time.Duration) error {
	m.mu.Lock()
	fn := m.heartbeatFn
	m.mu.Unlock()
	return fn(key, token, ttl)
}

func (m *mockBackend) Unlock(key, token string) error {
	m.mu.Lock()
	fn := m.unlockFn
	m.mu.Unlock()
	return fn(key, token)
}

func (m *mockBackend) GetHolder(key string) (string, int, time.Duration, error) {
	m.mu.Lock()
	fn := m.getHolderFn
	m.mu.Unlock()
	return fn(key)
}

func (m *mockBackend) IsLocked(key string) (bool, error) {
	m.mu.Lock()
	fn := m.isLockedFn
	m.mu.Unlock()
	return fn(key)
}

func (m *mockBackend) setHeartbeatFn(fn func(key, token string, ttl time.Duration) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.heartbeatFn = fn
}

func (m *mockBackend) setGetHolderFn(fn func(key string) (string, int, time.Duration, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getHolderFn = fn
}

func (m *mockBackend) setTryLockFn(fn func(key, token string, ttl time.Duration) (bool, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tryLockFn = fn
}

func waitForEvent(ch chan ElectionEvent, timeout time.Duration) (ElectionEvent, bool) {
	select {
	case evt := <-ch:
		return evt, true
	case <-time.After(timeout):
		return ElectionEvent{}, false
	}
}

func TestNodeRole_String(t *testing.T) {
	tests := []struct {
		role     NodeRole
		expected string
	}{
		{RoleFollower, "follower"},
		{RoleCandidate, "candidate"},
		{RoleLeader, "leader"},
		{NodeRole(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.role.String(); got != tt.expected {
			t.Errorf("NodeRole(%d).String() = %q, want %q", tt.role, got, tt.expected)
		}
	}
}

func TestElectionEventType_String(t *testing.T) {
	tests := []struct {
		eventType ElectionEventType
		expected  string
	}{
		{EventBecomeLeader, "become_leader"},
		{EventBecomeFollower, "become_follower"},
		{EventElectionStart, "election_start"},
		{EventElectionEnd, "election_end"},
		{EventHeartbeat, "heartbeat"},
		{EventLeaderLost, "leader_lost"},
		{ElectionEventType(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.eventType.String(); got != tt.expected {
			t.Errorf("ElectionEventType(%d).String() = %q, want %q", tt.eventType, got, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LeaseDuration != DefaultLeaseDuration {
		t.Errorf("DefaultConfig().LeaseDuration = %v, want %v", cfg.LeaseDuration, DefaultLeaseDuration)
	}
	if cfg.HeartbeatFactor != DefaultHeartbeatFactor {
		t.Errorf("DefaultConfig().HeartbeatFactor = %v, want %v", cfg.HeartbeatFactor, DefaultHeartbeatFactor)
	}
	if cfg.CheckFactor != DefaultCheckFactor {
		t.Errorf("DefaultConfig().CheckFactor = %v, want %v", cfg.CheckFactor, DefaultCheckFactor)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0.3,
				CheckFactor:     0.5,
			},
			wantErr: false,
		},
		{
			name: "invalid lease duration zero",
			cfg: Config{
				LeaseDuration:   0,
				HeartbeatFactor: 0.3,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid lease duration negative",
			cfg: Config{
				LeaseDuration:   -1 * time.Second,
				HeartbeatFactor: 0.3,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat factor zero",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat factor negative",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: -0.1,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat factor one",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 1,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
		{
			name: "invalid check factor zero",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0.3,
				CheckFactor:     0,
			},
			wantErr: true,
		},
		{
			name: "invalid check factor negative",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0.3,
				CheckFactor:     -0.1,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat >= check factor",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0.5,
				CheckFactor:     0.3,
			},
			wantErr: true,
		},
		{
			name: "invalid heartbeat equals check factor",
			cfg: Config{
				LeaseDuration:   5 * time.Second,
				HeartbeatFactor: 0.5,
				CheckFactor:     0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("Config.Validate() error should wrap ErrInvalidConfig, got %v", err)
			}
		})
	}
}

func TestConfig_Intervals(t *testing.T) {
	cfg := Config{
		LeaseDuration:   10 * time.Second,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	expectedHeartbeat := 3 * time.Second
	if got := cfg.HeartbeatInterval(); got != expectedHeartbeat {
		t.Errorf("HeartbeatInterval() = %v, want %v", got, expectedHeartbeat)
	}

	expectedCheck := 5 * time.Second
	if got := cfg.CheckInterval(); got != expectedCheck {
		t.Errorf("CheckInterval() = %v, want %v", got, expectedCheck)
	}
}

func TestNewLeaderElector_Validation(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := DefaultConfig()

	tests := []struct {
		name        string
		nodeID      string
		electionKey string
		cfg         Config
		backend     LockBackend
		wantErr     error
	}{
		{
			name:        "empty node id",
			nodeID:      "",
			electionKey: "test-election",
			cfg:         cfg,
			backend:     backend,
			wantErr:     ErrEmptyNodeID,
		},
		{
			name:        "empty election key",
			nodeID:      "node-1",
			electionKey: "",
			cfg:         cfg,
			backend:     backend,
			wantErr:     ErrEmptyElectionKey,
		},
		{
			name:        "invalid config",
			nodeID:      "node-1",
			electionKey: "test-election",
			cfg:         Config{LeaseDuration: -1},
			backend:     backend,
			wantErr:     ErrInvalidConfig,
		},
		{
			name:        "nil backend",
			nodeID:      "node-1",
			electionKey: "test-election",
			cfg:         cfg,
			backend:     nil,
			wantErr:     ErrInvalidConfig,
		},
		{
			name:        "valid",
			nodeID:      "node-1",
			electionKey: "test-election",
			cfg:         cfg,
			backend:     backend,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elector, err := NewLeaderElector(tt.nodeID, tt.electionKey, tt.cfg, tt.backend)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("NewLeaderElector() expected error %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NewLeaderElector() error = %v, want %v", err, tt.wantErr)
				}
				if elector != nil {
					t.Error("NewLeaderElector() should return nil on error")
				}
			} else {
				if err != nil {
					t.Fatalf("NewLeaderElector() unexpected error: %v", err)
				}
				if elector == nil {
					t.Fatal("NewLeaderElector() returned nil")
				}
				if elector.NodeID() != tt.nodeID {
					t.Errorf("NodeID() = %q, want %q", elector.NodeID(), tt.nodeID)
				}
				if elector.Role() != RoleFollower {
					t.Errorf("Role() = %v, want %v", elector.Role(), RoleFollower)
				}
				if elector.IsLeader() {
					t.Error("IsLeader() should be false initially")
				}
				if elector.Running() {
					t.Error("Running() should be false initially")
				}
			}
		})
	}
}

func TestLeaderElector_SingleNodeBecomesLeader(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 10)
	elector.RegisterCallback(func(event ElectionEvent) {
		eventCh <- event
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	foundBecomeLeader := false
	timeout := time.After(300 * time.Millisecond)
loop:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventBecomeLeader {
				foundBecomeLeader = true
			}
		case <-timeout:
			break loop
		}
	}

	if !elector.IsLeader() {
		t.Error("Expected node to become leader")
	}
	if elector.Role() != RoleLeader {
		t.Errorf("Role() = %v, want %v", elector.Role(), RoleLeader)
	}
	if elector.LeaderID() != "node-1" {
		t.Errorf("LeaderID() = %q, want %q", elector.LeaderID(), "node-1")
	}
	if elector.Term() < 1 {
		t.Errorf("Term() = %d, want >= 1", elector.Term())
	}
	if !foundBecomeLeader {
		t.Error("Expected EventBecomeLeader callback")
	}
}

func TestLeaderElector_MultipleNodesOnlyOneLeader(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   200 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	numNodes := 5
	electors := make([]*LeaderElector, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeID := "node-" + string(rune('0'+i))
		e, err := NewLeaderElector(nodeID, "test-election", cfg, backend)
		if err != nil {
			t.Fatalf("NewLeaderElector(%s) error: %v", nodeID, err)
		}
		electors[i] = e
	}

	var wg sync.WaitGroup
	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(e *LeaderElector) {
			defer wg.Done()
			_ = e.Start()
		}(electors[i])
	}
	wg.Wait()

	defer func() {
		for _, e := range electors {
			e.Stop()
		}
	}()

	time.Sleep(300 * time.Millisecond)

	leaderCount := 0
	var leaderID string
	for _, e := range electors {
		if e.IsLeader() {
			leaderCount++
			leaderID = e.NodeID()
		}
	}

	if leaderCount != 1 {
		t.Errorf("Expected exactly 1 leader, got %d", leaderCount)
	}

	for _, e := range electors {
		if e.LeaderID() != leaderID {
			t.Errorf("Node %s has LeaderID %q, but expected %q", e.NodeID(), e.LeaderID(), leaderID)
		}
	}
}

func TestLeaderElector_HeartbeatRenewsLease(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.2,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 100)
	elector.RegisterCallback(func(event ElectionEvent) {
		select {
		case eventCh <- event:
		default:
		}
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	time.Sleep(150 * time.Millisecond)

	if !elector.IsLeader() {
		t.Fatal("Expected node to be leader")
	}

	heartbeatCount := 0
	timeout := time.After(50 * time.Millisecond)
loop:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventHeartbeat {
				heartbeatCount++
			}
		case <-timeout:
			break loop
		}
	}

	if heartbeatCount < 1 {
		t.Errorf("Expected at least 1 heartbeat, got %d", heartbeatCount)
	}

	time.Sleep(100 * time.Millisecond)

	if !elector.IsLeader() {
		t.Error("Leader should still be leader after heartbeat renewals")
	}
}

func TestLeaderElector_LeaderFailureReElection(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	leader, err := NewLeaderElector("leader-node", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(leader) error: %v", err)
	}

	follower, err := NewLeaderElector("follower-node", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(follower) error: %v", err)
	}

	if err := leader.Start(); err != nil {
		t.Fatalf("leader.Start() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := follower.Start(); err != nil {
		t.Fatalf("follower.Start() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !leader.IsLeader() {
		t.Fatal("Expected leader-node to be leader")
	}
	if follower.IsLeader() {
		t.Error("Expected follower-node to be follower")
	}

	leader.Stop()

	time.Sleep(200 * time.Millisecond)

	if !follower.IsLeader() {
		t.Error("Expected follower-node to become leader after leader failure")
	}
	if follower.LeaderID() != "follower-node" {
		t.Errorf("LeaderID() = %q, want %q", follower.LeaderID(), "follower-node")
	}
}

func TestLeaderElector_Resign(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 100)
	elector.RegisterCallback(func(event ElectionEvent) {
		eventCh <- event
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	time.Sleep(80 * time.Millisecond)

	if !elector.IsLeader() {
		t.Fatal("Expected node to be leader")
	}

	if err := elector.Resign(); err != nil {
		t.Fatalf("Resign() error: %v", err)
	}

	if elector.IsLeader() {
		t.Error("Expected node to be follower after resign")
	}
	if elector.Role() != RoleFollower {
		t.Errorf("Role() = %v, want %v", elector.Role(), RoleFollower)
	}

	foundLeaderLost := false
	foundBecomeFollower := false
	timeout := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventLeaderLost {
				foundLeaderLost = true
			}
			if evt.Type == EventBecomeFollower && evt.Role == RoleFollower {
				foundBecomeFollower = true
			}
		case <-timeout:
			break loop
		}
	}

	if !foundLeaderLost {
		t.Error("Expected EventLeaderLost callback")
	}
	if !foundBecomeFollower {
		t.Error("Expected EventBecomeFollower callback after resign")
	}
}

func TestLeaderElector_ResignWhenNotLeader(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	leader, err := NewLeaderElector("leader", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(leader) error: %v", err)
	}

	follower, err := NewLeaderElector("follower", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(follower) error: %v", err)
	}

	if err := leader.Start(); err != nil {
		t.Fatalf("leader.Start() error: %v", err)
	}
	defer leader.Stop()

	time.Sleep(50 * time.Millisecond)

	if err := follower.Start(); err != nil {
		t.Fatalf("follower.Start() error: %v", err)
	}
	defer follower.Stop()

	time.Sleep(50 * time.Millisecond)

	if follower.IsLeader() {
		t.Fatal("Expected follower to not be leader")
	}

	err = follower.Resign()
	if err != nil {
		t.Errorf("Resign() when not leader should return nil, got %v", err)
	}
}

func TestLeaderElector_StartWhenStopped(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := DefaultConfig()

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	elector.Stop()

	err = elector.Start()
	if !errors.Is(err, ErrElectorStopped) {
		t.Errorf("Start() after Stop() error = %v, want %v", err, ErrElectorStopped)
	}
}

func TestLeaderElector_StartWhenRunning(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}
	defer elector.Stop()

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	err = elector.Start()
	if !errors.Is(err, ErrElectorRunning) {
		t.Errorf("Start() when running error = %v, want %v", err, ErrElectorRunning)
	}
}

func TestLeaderElector_StopMultipleTimes(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	elector.Stop()
	elector.Stop()

	if elector.Running() {
		t.Error("Running() should be false after Stop()")
	}
}

func TestLeaderElector_ElectionCallbacks(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 100)
	elector.RegisterCallback(func(event ElectionEvent) {
		eventCh <- event
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	foundElectionStart := false
	foundElectionEnd := false
	foundBecomeLeader := false
	foundHeartbeat := false
	timeout := time.After(300 * time.Millisecond)
loop:
	for {
		select {
		case evt := <-eventCh:
			switch evt.Type {
			case EventElectionStart:
				foundElectionStart = true
			case EventElectionEnd:
				foundElectionEnd = true
			case EventBecomeLeader:
				foundBecomeLeader = true
			case EventHeartbeat:
				foundHeartbeat = true
			}
		case <-timeout:
			break loop
		}
	}

	if !foundElectionStart {
		t.Error("Expected EventElectionStart event")
	}
	if !foundElectionEnd {
		t.Error("Expected EventElectionEnd event")
	}
	if !foundBecomeLeader {
		t.Error("Expected EventBecomeLeader event")
	}
	if !foundHeartbeat {
		t.Error("Expected EventHeartbeat event")
	}
}

func TestLeaderElector_MultipleCallbacks(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	ch1 := make(chan struct{}, 1)
	ch2 := make(chan struct{}, 1)

	elector.RegisterCallback(func(event ElectionEvent) {
		if event.Type == EventBecomeLeader {
			select {
			case ch1 <- struct{}{}:
			default:
			}
		}
	})
	elector.RegisterCallback(func(event ElectionEvent) {
		if event.Type == EventBecomeLeader {
			select {
			case ch2 <- struct{}{}:
			default:
			}
		}
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	select {
	case <-ch1:
	case <-time.After(500 * time.Millisecond):
		t.Error("Callback 1 not called")
	}

	select {
	case <-ch2:
	case <-time.After(500 * time.Millisecond):
		t.Error("Callback 2 not called")
	}
}

func TestLeaderElector_LeaderRecoversAsFollower(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	node1, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(node-1) error: %v", err)
	}

	node2, err := NewLeaderElector("node-2", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(node-2) error: %v", err)
	}

	if err := node1.Start(); err != nil {
		t.Fatalf("node1.Start() error: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if !node1.IsLeader() {
		t.Fatal("Expected node-1 to be leader")
	}

	node1.Stop()

	time.Sleep(150 * time.Millisecond)

	if err := node2.Start(); err != nil {
		t.Fatalf("node2.Start() error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !node2.IsLeader() {
		t.Fatal("Expected node-2 to become leader after node-1 failure")
	}

	node1Restarted, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector(node-1 restarted) error: %v", err)
	}

	if err := node1Restarted.Start(); err != nil {
		t.Fatalf("node1 restarted Start() error: %v", err)
	}
	defer node1Restarted.Stop()
	defer node2.Stop()

	time.Sleep(100 * time.Millisecond)

	if node1Restarted.IsLeader() {
		t.Error("node-1 should not be leader after rejoining, node-2 should remain leader")
	}
	if !node2.IsLeader() {
		t.Error("node-2 should remain leader")
	}
}

func TestLockManagerBackend(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)

	success, err := backend.TryLock("key", "token", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock() error: %v", err)
	}
	if !success {
		t.Error("TryLock() should succeed")
	}

	locked, err := backend.IsLocked("key")
	if err != nil {
		t.Fatalf("IsLocked() error: %v", err)
	}
	if !locked {
		t.Error("IsLocked() should return true")
	}

	holder, _, _, err := backend.GetHolder("key")
	if err != nil {
		t.Fatalf("GetHolder() error: %v", err)
	}
	if holder != "token" {
		t.Errorf("GetHolder() = %q, want %q", holder, "token")
	}

	err = backend.Heartbeat("key", "token", 10*time.Second)
	if err != nil {
		t.Fatalf("Heartbeat() error: %v", err)
	}

	err = backend.Unlock("key", "token")
	if err != nil {
		t.Fatalf("Unlock() error: %v", err)
	}

	locked, err = backend.IsLocked("key")
	if err != nil {
		t.Fatalf("IsLocked() error: %v", err)
	}
	if locked {
		t.Error("IsLocked() should return false after unlock")
	}
}

func TestLeaderElector_NoBrainSplit(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   150 * time.Millisecond,
		HeartbeatFactor: 0.2,
		CheckFactor:     0.4,
	}

	numNodes := 10
	electors := make([]*LeaderElector, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeID := "node-" + string(rune('0'+i))
		e, err := NewLeaderElector(nodeID, "test-election", cfg, backend)
		if err != nil {
			t.Fatalf("NewLeaderElector(%s) error: %v", nodeID, err)
		}
		electors[i] = e
	}

	var wg sync.WaitGroup
	for i := 0; i < numNodes; i++ {
		wg.Add(1)
		go func(e *LeaderElector) {
			defer wg.Done()
			_ = e.Start()
		}(electors[i])
	}
	wg.Wait()

	defer func() {
		for _, e := range electors {
			e.Stop()
		}
	}()

	for i := 0; i < 5; i++ {
		time.Sleep(100 * time.Millisecond)

		leaderCount := 0
		for _, e := range electors {
			if e.IsLeader() {
				leaderCount++
			}
		}

		if leaderCount > 1 {
			t.Fatalf("Brain split detected: %d leaders at iteration %d", leaderCount, i)
		}
	}
}

func TestLeaderElector_ResignWhenStopped(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := DefaultConfig()

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	elector.Stop()

	err = elector.Resign()
	if !errors.Is(err, ErrElectorStopped) {
		t.Errorf("Resign() when stopped error = %v, want %v", err, ErrElectorStopped)
	}
}

func TestLeaderElector_HeartbeatFailure(t *testing.T) {
	mock := newMockBackend()
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.2,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, mock)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 100)
	elector.RegisterCallback(func(event ElectionEvent) {
		eventCh <- event
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	timeout := time.After(300 * time.Millisecond)
	becameLeader := false
loop1:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventBecomeLeader {
				becameLeader = true
			}
		case <-timeout:
			break loop1
		}
	}
	if !becameLeader {
		t.Fatal("Expected node to become leader before heartbeat failure test")
	}
	if !elector.IsLeader() {
		t.Fatal("Expected node to be leader")
	}

	heartbeatErr := errors.New("mock heartbeat failure")
	mock.setHeartbeatFn(func(key, token string, ttl time.Duration) error {
		return heartbeatErr
	})
	mock.setGetHolderFn(func(key string) (string, int, time.Duration, error) {
		return "", 0, 0, distlock.ErrLockNotHeld
	})
	mock.setTryLockFn(func(key, token string, ttl time.Duration) (bool, error) {
		return false, nil
	})

	foundLeaderLost := false
	foundBecomeFollower := false
	timeout2 := time.After(500 * time.Millisecond)
loop2:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventLeaderLost {
				foundLeaderLost = true
				if evt.Role != RoleFollower {
					t.Errorf("EventLeaderLost event Role = %v, want %v", evt.Role, RoleFollower)
				}
			}
			if evt.Type == EventBecomeFollower {
				foundBecomeFollower = true
				if evt.Role != RoleFollower {
					t.Errorf("EventBecomeFollower event Role = %v, want %v", evt.Role, RoleFollower)
				}
			}
			if foundLeaderLost && foundBecomeFollower {
				break loop2
			}
		case <-timeout2:
			break loop2
		}
	}

	if elector.IsLeader() {
		t.Error("Expected node to lose leadership after heartbeat failure")
	}
	if elector.Role() != RoleFollower {
		t.Errorf("Role() = %v, want %v", elector.Role(), RoleFollower)
	}
	if !foundLeaderLost {
		t.Error("Expected EventLeaderLost callback on heartbeat failure")
	}
	if !foundBecomeFollower {
		t.Error("Expected EventBecomeFollower callback on heartbeat failure")
	}
}

func TestLeaderElector_CheckLeaderBackendErrorTriggersElection(t *testing.T) {
	mock := newMockBackend()
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, mock)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	eventCh := make(chan ElectionEvent, 100)
	elector.RegisterCallback(func(event ElectionEvent) {
		select {
		case eventCh <- event:
		default:
		}
	})

	backendErr := errors.New("mock backend error")
	mock.setGetHolderFn(func(key string) (string, int, time.Duration, error) {
		return "", 0, 0, backendErr
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	foundElectionStart := false
	timeout := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == EventElectionStart {
				foundElectionStart = true
			}
		case <-timeout:
			break loop
		}
	}

	if !foundElectionStart {
		t.Error("Expected election to be triggered when GetHolder returns unexpected error")
	}
}

func TestLeaderElector_CallbacksAsync(t *testing.T) {
	lm := distlock.NewLockManager()
	backend := NewLockManagerBackend(lm)
	cfg := Config{
		LeaseDuration:   100 * time.Millisecond,
		HeartbeatFactor: 0.3,
		CheckFactor:     0.5,
	}

	elector, err := NewLeaderElector("node-1", "test-election", cfg, backend)
	if err != nil {
		t.Fatalf("NewLeaderElector() error: %v", err)
	}

	var callbackStarted int32
	var callbackDone int32
	doneCh := make(chan struct{})
	var once sync.Once

	elector.RegisterCallback(func(event ElectionEvent) {
		if !atomic.CompareAndSwapInt32(&callbackStarted, 0, 1) {
			return
		}
		time.Sleep(200 * time.Millisecond)
		atomic.StoreInt32(&callbackDone, 1)
		once.Do(func() {
			close(doneCh)
		})
	})

	if err := elector.Start(); err != nil {
		t.Fatalf("Start() error: %v", err)
	}
	defer elector.Stop()

	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&callbackStarted) == 0 {
		t.Fatal("Expected callback to start executing")
	}
	if atomic.LoadInt32(&callbackDone) == 1 {
		t.Error("Expected callback to not complete synchronously (should be async)")
	}

	select {
	case <-doneCh:
	case <-time.After(500 * time.Millisecond):
		t.Error("Callback did not complete within timeout")
	}

	if atomic.LoadInt32(&callbackDone) == 0 {
		t.Error("Expected callback to complete eventually")
	}
}
