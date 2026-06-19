package distlock

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewLockManager(t *testing.T) {
	lm := NewLockManager()
	if lm == nil {
		t.Fatal("NewLockManager returned nil")
	}

	cfg := DefaultLockManagerConfig()
	cfg.MaxReentrancy = 10
	cfg.CleanInterval = 50 * time.Millisecond
	lm2, err := NewLockManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewLockManagerWithConfig error: %v", err)
	}
	if lm2 == nil {
		t.Fatal("NewLockManagerWithConfig returned nil")
	}

	cfgInvalid := LockManagerConfig{CleanInterval: -1}
	_, err = NewLockManagerWithConfig(cfgInvalid)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL, got %v", err)
	}

	cfgZero := LockManagerConfig{MaxReentrancy: 0, CleanInterval: 0}
	lm3, err := NewLockManagerWithConfig(cfgZero)
	if err != nil {
		t.Fatalf("Expected no error for zero config, got %v", err)
	}
	if lm3.cfg.MaxReentrancy != DefaultMaxReentrancy {
		t.Errorf("Expected default MaxReentrancy %d, got %d", DefaultMaxReentrancy, lm3.cfg.MaxReentrancy)
	}
}

func TestLockManager_LockAndUnlock(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err = lm.Unlock("test-key", "token-1")
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestLockManager_TokenMismatch(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err = lm.Unlock("test-key", "token-2")
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}

	err = lm.Unlock("test-key", "token-1")
	if err != nil {
		t.Fatalf("Unlock with correct token failed: %v", err)
	}
}

func TestLockManager_LockAlreadyHeld(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err = lm.Lock("test-key", "token-2", 5*time.Second)
	if !errors.Is(err, ErrLockAlreadyHeld) {
		t.Errorf("Expected ErrLockAlreadyHeld, got %v", err)
	}

	ok, err := lm.TryLock("test-key", "token-2", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if ok {
		t.Error("TryLock should return false when lock is held")
	}
}

func TestLockManager_TryLock(t *testing.T) {
	lm := NewLockManager()

	ok, err := lm.TryLock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if !ok {
		t.Error("TryLock should succeed for free lock")
	}

	ok, err = lm.TryLock("test-key", "token-2", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if ok {
		t.Error("TryLock should fail for held lock")
	}
}

func TestLockManager_Reentrancy(t *testing.T) {
	lm := NewLockManager()

	for i := 0; i < 5; i++ {
		err := lm.Lock("test-key", "token-1", 5*time.Second)
		if err != nil {
			t.Fatalf("Reentrant lock %d failed: %v", i+1, err)
		}
	}

	token, reentrancy, _, err := lm.GetHolder("test-key")
	if err != nil {
		t.Fatalf("GetHolder failed: %v", err)
	}
	if token != "token-1" {
		t.Errorf("Expected token token-1, got %s", token)
	}
	if reentrancy != 5 {
		t.Errorf("Expected reentrancy 5, got %d", reentrancy)
	}

	for i := 0; i < 5; i++ {
		err = lm.Unlock("test-key", "token-1")
		if err != nil {
			t.Fatalf("Unlock %d failed: %v", i+1, err)
		}
	}

	locked, err := lm.IsLocked("test-key")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if locked {
		t.Error("Lock should be released after all unlocks")
	}
}

func TestLockManager_ReentrancyAfterRelease(t *testing.T) {
	cfg := LockManagerConfig{MaxReentrancy: 3}
	lm, _ := NewLockManagerWithConfig(cfg)

	err := lm.Lock("k", "t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Lock("k", "t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Unlock("k", "t")
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Lock("k", "t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _, _ := lm.GetHolder("k")
	if r != 2 {
		t.Errorf("Expected reentrancy 2, got %d", r)
	}
	err = lm.Unlock("k", "t")
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Unlock("k", "t")
	if err != nil {
		t.Fatal(err)
	}
}

func TestLockManager_MaxReentrancy(t *testing.T) {
	cfg := LockManagerConfig{MaxReentrancy: 3}
	lm, err := NewLockManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewLockManagerWithConfig error: %v", err)
	}

	for i := 0; i < 3; i++ {
		err := lm.Lock("test-key", "token-1", 5*time.Second)
		if err != nil {
			t.Fatalf("Lock %d failed: %v", i+1, err)
		}
	}

	err = lm.Lock("test-key", "token-1", 5*time.Second)
	if !errors.Is(err, ErrMaxReentrancy) {
		t.Errorf("Expected ErrMaxReentrancy, got %v", err)
	}
}

func TestLockManager_Expiration(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	_, _, _, err = lm.GetHolder("test-key")
	if !errors.Is(err, ErrLockExpired) {
		t.Errorf("Expected ErrLockExpired from GetHolder, got %v", err)
	}

	err = lm.Unlock("test-key", "token-1")
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("Expected ErrLockNotHeld after expired lock was cleaned, got %v", err)
	}
}

func TestLockManager_Heartbeat(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	err = lm.Heartbeat("test-key", "token-1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	time.Sleep(30 * time.Millisecond)

	locked, err := lm.IsLocked("test-key")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if !locked {
		t.Error("Lock should still be held after heartbeat")
	}

	_, _, ttl, err := lm.GetHolder("test-key")
	if err != nil {
		t.Fatalf("GetHolder error: %v", err)
	}
	if ttl <= 0 {
		t.Error("TTL should be positive after heartbeat")
	}
}

func TestLockManager_HeartbeatExpired(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	time.Sleep(60 * time.Millisecond)

	err = lm.Heartbeat("test-key", "token-1", 100*time.Millisecond)
	if !errors.Is(err, ErrLockExpired) {
		t.Errorf("Expected ErrLockExpired, got %v", err)
	}
}

func TestLockManager_HeartbeatTokenMismatch(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err = lm.Heartbeat("test-key", "token-2", 100*time.Millisecond)
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestLockManager_ValidationErrors(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("", "token-1", 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey, got %v", err)
	}

	err = lm.Lock("key", "", 5*time.Second)
	if !errors.Is(err, ErrEmptyToken) {
		t.Errorf("Expected ErrEmptyToken, got %v", err)
	}

	err = lm.Lock("key", "token", 0)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for zero ttl, got %v", err)
	}

	err = lm.Lock("key", "token", -1*time.Second)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for negative ttl, got %v", err)
	}

	err = lm.Unlock("", "token")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for unlock, got %v", err)
	}

	err = lm.Unlock("key", "")
	if !errors.Is(err, ErrEmptyToken) {
		t.Errorf("Expected ErrEmptyToken for unlock, got %v", err)
	}

	_, err = lm.TryLock("", "token", 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for trylock, got %v", err)
	}

	err = lm.Heartbeat("", "token", 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for heartbeat, got %v", err)
	}

	err = lm.Heartbeat("key", "token", 0)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for heartbeat with zero ttl, got %v", err)
	}
}

func TestLockManager_UnlockNotHeld(t *testing.T) {
	lm := NewLockManager()

	err := lm.Unlock("nonexistent", "token")
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("Expected ErrLockNotHeld, got %v", err)
	}
}

func TestLockManager_IsLocked(t *testing.T) {
	lm := NewLockManager()

	locked, err := lm.IsLocked("nonexistent")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if locked {
		t.Error("IsLocked should return false for nonexistent key")
	}

	err = lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	locked, err = lm.IsLocked("test-key")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if !locked {
		t.Error("IsLocked should return true for held lock")
	}

	_, err = lm.IsLocked("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey, got %v", err)
	}
}

func TestLockManager_GetHolder(t *testing.T) {
	lm := NewLockManager()

	_, _, _, err := lm.GetHolder("nonexistent")
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("Expected ErrLockNotHeld, got %v", err)
	}

	err = lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	token, reentrancy, ttl, err := lm.GetHolder("test-key")
	if err != nil {
		t.Fatalf("GetHolder error: %v", err)
	}
	if token != "token-1" {
		t.Errorf("Expected token token-1, got %s", token)
	}
	if reentrancy != 1 {
		t.Errorf("Expected reentrancy 1, got %d", reentrancy)
	}
	if ttl <= 0 {
		t.Error("TTL should be positive")
	}

	_, _, _, err = lm.GetHolder("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey, got %v", err)
	}
}

func TestLockManager_ForceUnlock(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("test-key", "token-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err = lm.ForceUnlock("test-key")
	if err != nil {
		t.Fatalf("ForceUnlock failed: %v", err)
	}

	locked, err := lm.IsLocked("test-key")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if locked {
		t.Error("Lock should be force unlocked")
	}

	err = lm.ForceUnlock("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey, got %v", err)
	}
}

func TestLockManager_CountAndClear(t *testing.T) {
	lm := NewLockManager()

	count, err := lm.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	for i := 0; i < 5; i++ {
		key := "key-" + string(rune('a'+i))
		err := lm.Lock(key, "token", 5*time.Second)
		if err != nil {
			t.Fatalf("Lock %s failed: %v", key, err)
		}
	}

	count, err = lm.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected count 5, got %d", count)
	}

	err = lm.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	count, err = lm.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0 after clear, got %d", count)
	}
}

func TestLockManager_CleanExpired(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("expired-1", "t1", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Lock("expired-2", "t2", 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	err = lm.Lock("active", "t3", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	cleaned, err := lm.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 2 {
		t.Errorf("Expected cleaned 2, got %d", cleaned)
	}

	count, err := lm.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestLockManager_StartStop(t *testing.T) {
	lm := NewLockManager()

	lm.Start()
	lm.Start()

	err := lm.Lock("test", "token", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	lm.Stop()
	lm.Stop()

	err = lm.Lock("test2", "token", 5*time.Second)
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped after stop, got %v", err)
	}
}

func TestLockManager_StoppedOperations(t *testing.T) {
	lm := NewLockManager()
	lm.Stop()

	_, err := lm.Count()
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for Count, got %v", err)
	}

	err = lm.Clear()
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for Clear, got %v", err)
	}

	_, err = lm.CleanExpired()
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for CleanExpired, got %v", err)
	}

	err = lm.ForceUnlock("k")
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for ForceUnlock, got %v", err)
	}

	_, err = lm.IsLocked("k")
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for IsLocked, got %v", err)
	}

	_, _, _, err = lm.GetHolder("k")
	if !errors.Is(err, ErrLockManagerStopped) {
		t.Errorf("Expected ErrLockManagerStopped for GetHolder, got %v", err)
	}
}

func TestLockManager_ConcurrentAccess(t *testing.T) {
	lm := NewLockManager()
	const goroutines = 20
	var counter int32
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			token := "token-" + string(rune('A'+id%10))
			for j := 0; j < 100; j++ {
				key := "key-" + string(rune('a'+j%5))
				err := lm.Lock(key, token, 5*time.Second)
				if err != nil {
					if !errors.Is(err, ErrLockAlreadyHeld) {
						errs <- err
						return
					}
					continue
				}
				atomic.AddInt32(&counter, 1)
				err = lm.Unlock(key, token)
				if err != nil && !errors.Is(err, ErrLockExpired) {
					errs <- err
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("Concurrent error: %v", err)
	}
}

func TestMemoryLockNode(t *testing.T) {
	node := NewMemoryLockNode("node-1")
	if node.ID() != "node-1" {
		t.Errorf("Expected ID node-1, got %s", node.ID())
	}

	err := node.Lock("key", "token", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	ok, err := node.TryLock("key", "token2", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock error: %v", err)
	}
	if ok {
		t.Error("TryLock should fail for held lock")
	}

	locked, err := node.IsLocked("key")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if !locked {
		t.Error("IsLocked should be true")
	}

	ttl, err := node.GetRemainingTTL("key")
	if err != nil {
		t.Fatalf("GetRemainingTTL error: %v", err)
	}
	if ttl <= 0 {
		t.Error("TTL should be positive")
	}

	err = node.Heartbeat("key", "token", 10*time.Second)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	err = node.Unlock("key", "token")
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestMemoryLockNode_WithConfig(t *testing.T) {
	cfg := LockManagerConfig{MaxReentrancy: 2}
	node, err := NewMemoryLockNodeWithConfig("node-test", cfg)
	if err != nil {
		t.Fatalf("NewMemoryLockNodeWithConfig error: %v", err)
	}

	err = node.Lock("k", "t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = node.Lock("k", "t", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = node.Lock("k", "t", time.Second)
	if !errors.Is(err, ErrMaxReentrancy) {
		t.Errorf("Expected ErrMaxReentrancy, got %v", err)
	}
}

func TestNewRedlock(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}

	rl, err := NewRedlock(nodes)
	if err != nil {
		t.Fatalf("NewRedlock error: %v", err)
	}
	if rl.NodeCount() != 3 {
		t.Errorf("Expected 3 nodes, got %d", rl.NodeCount())
	}

	evenNodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
	}
	_, err = NewRedlock(evenNodes)
	if !errors.Is(err, ErrInvalidNodeCount) {
		t.Errorf("Expected ErrInvalidNodeCount for even nodes, got %v", err)
	}

	_, err = NewRedlock(nil)
	if !errors.Is(err, ErrInvalidNodeCount) {
		t.Errorf("Expected ErrInvalidNodeCount for nil nodes, got %v", err)
	}
}

func TestNewRedlockWithConfig(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}

	cfg := RedlockConfig{
		AcquireTimeout: 2 * time.Second,
		RetryDelay:     50 * time.Millisecond,
		ClockDrift:     10 * time.Millisecond,
	}
	rl, err := NewRedlockWithConfig(nodes, cfg)
	if err != nil {
		t.Fatalf("NewRedlockWithConfig error: %v", err)
	}
	if rl == nil {
		t.Fatal("NewRedlockWithConfig returned nil")
	}

	invalidCfg := RedlockConfig{AcquireTimeout: -1}
	_, err = NewRedlockWithConfig(nodes, invalidCfg)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL, got %v", err)
	}

	invalidCfg2 := RedlockConfig{RetryDelay: -1}
	_, err = NewRedlockWithConfig(nodes, invalidCfg2)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for RetryDelay, got %v", err)
	}

	invalidCfg3 := RedlockConfig{ClockDrift: -1}
	_, err = NewRedlockWithConfig(nodes, invalidCfg3)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for ClockDrift, got %v", err)
	}
}

func TestRedlock_LockAndUnlock(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, err := NewRedlock(nodes)
	if err != nil {
		t.Fatalf("NewRedlock error: %v", err)
	}

	acq, err := rl.Lock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if acq == nil {
		t.Fatal("LockAcquisition is nil")
	}
	if acq.Key != "resource" {
		t.Errorf("Expected key resource, got %s", acq.Key)
	}
	if acq.Token != "client-1" {
		t.Errorf("Expected token client-1, got %s", acq.Token)
	}
	if acq.SuccessCount < 2 {
		t.Errorf("Expected at least 2 success, got %d", acq.SuccessCount)
	}
	if len(acq.NodeExpiries) < 2 {
		t.Errorf("Expected at least 2 node expiries, got %d", len(acq.NodeExpiries))
	}
	if acq.Expiry.IsZero() {
		t.Error("Expiry should not be zero")
	}

	if !acq.IsValid() {
		t.Error("Acquisition should be valid")
	}

	err = rl.Unlock(acq)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	locked, err := rl.IsLocked("resource")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if locked {
		t.Error("Lock should be unlocked")
	}
}

func TestRedlock_TryLock(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, _ := NewRedlock(nodes)

	acq1, err := rl.TryLock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if acq1 == nil {
		t.Fatal("First TryLock returned nil")
	}

	acq2, err := rl.TryLock("resource", "client-2", 5*time.Second)
	if !errors.Is(err, ErrQuorumNotReached) {
		t.Errorf("Expected ErrQuorumNotReached, got %v", err)
	}
	if acq2 != nil {
		t.Error("Second TryLock should return nil acquisition")
	}

	err = rl.Unlock(acq1)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestRedlock_Contention(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, _ := NewRedlock(nodes)

	acq1, err := rl.Lock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	cfg := RedlockConfig{
		AcquireTimeout: 100 * time.Millisecond,
		RetryDelay:     10 * time.Millisecond,
		ClockDrift:     1 * time.Millisecond,
	}
	rl2, _ := NewRedlockWithConfig(nodes, cfg)

	acq2, err := rl2.Lock("resource", "client-2", 5*time.Second)
	if !errors.Is(err, ErrQuorumNotReached) {
		t.Errorf("Expected ErrQuorumNotReached, got %v", err)
	}
	if acq2 != nil {
		t.Error("Acquisition should be nil on failure")
	}

	err = rl.Unlock(acq1)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestRedlock_Heartbeat(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, _ := NewRedlock(nodes)

	acq, err := rl.Lock("resource", "client-1", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	acq2, err := rl.Heartbeat(acq, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}
	if acq2 == nil {
		t.Fatal("Heartbeat returned nil acquisition")
	}

	time.Sleep(100 * time.Millisecond)

	locked, err := rl.IsLocked("resource")
	if err != nil {
		t.Fatalf("IsLocked error: %v", err)
	}
	if !locked {
		t.Error("Lock should still be held after heartbeat")
	}

	err = rl.Unlock(acq2)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestRedlock_ValidationErrors(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, _ := NewRedlock(nodes)

	_, err := rl.Lock("", "token", 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey, got %v", err)
	}

	_, err = rl.Lock("key", "", 5*time.Second)
	if !errors.Is(err, ErrEmptyToken) {
		t.Errorf("Expected ErrEmptyToken, got %v", err)
	}

	_, err = rl.Lock("key", "token", 0)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL, got %v", err)
	}

	_, err = rl.TryLock("", "token", 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for TryLock, got %v", err)
	}

	err = rl.Unlock(nil)
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("Expected ErrLockNotHeld for nil unlock, got %v", err)
	}

	emptyAcq := &LockAcquisition{}
	err = rl.Unlock(emptyAcq)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for empty acq unlock, got %v", err)
	}

	acq := &LockAcquisition{Key: "k", Token: "t", NodeExpiries: map[string]time.Time{}}
	_, err = rl.Heartbeat(acq, 5*time.Second)
	if !errors.Is(err, ErrQuorumNotReached) {
		t.Errorf("Expected ErrQuorumNotReached for empty heartbeat, got %v", err)
	}

	_, err = rl.Heartbeat(nil, 5*time.Second)
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("Expected ErrLockNotHeld for nil heartbeat, got %v", err)
	}

	badAcq := &LockAcquisition{Key: "", Token: "t"}
	_, err = rl.Heartbeat(badAcq, 5*time.Second)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for heartbeat, got %v", err)
	}

	badAcq2 := &LockAcquisition{Key: "k", Token: ""}
	_, err = rl.Heartbeat(badAcq2, 5*time.Second)
	if !errors.Is(err, ErrEmptyToken) {
		t.Errorf("Expected ErrEmptyToken for heartbeat, got %v", err)
	}

	acqNoExp := &LockAcquisition{Key: "k", Token: "t", NodeExpiries: map[string]time.Time{"n1": time.Now()}}
	_, err = rl.Heartbeat(acqNoExp, 0)
	if !errors.Is(err, ErrInvalidTTL) {
		t.Errorf("Expected ErrInvalidTTL for heartbeat zero ttl, got %v", err)
	}

	_, err = rl.IsLocked("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("Expected ErrEmptyKey for IsLocked, got %v", err)
	}
}

func TestRedlock_MajorityWith5Nodes(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
		NewMemoryLockNode("n4"),
		NewMemoryLockNode("n5"),
	}
	rl, _ := NewRedlock(nodes)

	acq, err := rl.Lock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if acq.SuccessCount < 3 {
		t.Errorf("Expected at least 3 success on 5 nodes, got %d", acq.SuccessCount)
	}

	err = rl.Unlock(acq)
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
}

func TestLockAcquisition_RemainingTTL(t *testing.T) {
	var acq *LockAcquisition
	if acq.RemainingTTL() != 0 {
		t.Error("Nil acquisition should have 0 TTL")
	}

	acq = &LockAcquisition{
		Expiry: time.Now().Add(5 * time.Second),
	}
	ttl := acq.RemainingTTL()
	if ttl <= 0 {
		t.Error("RemainingTTL should be positive")
	}
	if ttl > 5*time.Second {
		t.Errorf("RemainingTTL should be <= 5s, got %v", ttl)
	}

	acqExpired := &LockAcquisition{
		Expiry: time.Now().Add(-1 * time.Second),
	}
	if acqExpired.RemainingTTL() != 0 {
		t.Error("Expired acquisition should have 0 TTL")
	}
	if acqExpired.IsValid() {
		t.Error("Expired acquisition should not be valid")
	}
}

func TestLockEntry_ExpirationMethods(t *testing.T) {
	entry := &lockEntry{
		key:       "test",
		token:     "t",
		expiresAt: time.Now().Add(5 * time.Second),
		reentrancy: 1,
	}

	now := time.Now()
	if entry.isExpired(now) {
		t.Error("Entry should not be expired")
	}

	ttl := entry.remainingTTL(now)
	if ttl <= 0 || ttl > 5*time.Second {
		t.Errorf("Unexpected TTL: %v", ttl)
	}

	if !entry.isExpired(entry.expiresAt.Add(time.Second)) {
		t.Error("Entry should be expired after expiry time")
	}

	ttl = entry.remainingTTL(entry.expiresAt.Add(time.Second))
	if ttl != 0 {
		t.Errorf("Expired entry should have 0 TTL, got %v", ttl)
	}
}

func TestRedlock_UnlockPartialFailure(t *testing.T) {
	n1 := NewMemoryLockNode("n1")
	n2 := NewMemoryLockNode("n2")
	n3 := NewMemoryLockNode("n3")
	nodes := []LockNode{n1, n2, n3}
	rl, _ := NewRedlock(nodes)

	acq, err := rl.Lock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	_ = n2.Unlock("resource", "client-1")
	_ = n3.Unlock("resource", "client-1")

	err = rl.Unlock(acq)
	if err != nil {
		t.Fatalf("Unlock with partial nodes already unlocked should not fail, got: %v", err)
	}
}

func TestLockManager_ExpiredLockCanBeReacquired(t *testing.T) {
	lm := NewLockManager()

	err := lm.Lock("k", "t1", 30*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(60 * time.Millisecond)

	err = lm.Lock("k", "t2", 5*time.Second)
	if err != nil {
		t.Fatalf("Should be able to reacquire expired lock: %v", err)
	}

	token, _, _, err := lm.GetHolder("k")
	if err != nil {
		t.Fatal(err)
	}
	if token != "t2" {
		t.Errorf("Expected t2, got %s", token)
	}
}

func TestRedlock_HeartbeatNodeExpiry(t *testing.T) {
	nodes := []LockNode{
		NewMemoryLockNode("n1"),
		NewMemoryLockNode("n2"),
		NewMemoryLockNode("n3"),
	}
	rl, _ := NewRedlock(nodes)

	acq, err := rl.Lock("resource", "client-1", 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}

	acq2, err := rl.Heartbeat(acq, 10*time.Second)
	if err != nil {
		t.Fatalf("Heartbeat failed: %v", err)
	}

	if acq2.SuccessCount < 2 {
		t.Errorf("Expected >=2 success, got %d", acq2.SuccessCount)
	}

	for nodeID, exp := range acq2.NodeExpiries {
		_ = nodeID
		if exp.Before(time.Now()) {
			t.Errorf("Node %s expiry should be in future: %v", nodeID, exp)
		}
	}

	rl.Unlock(acq2)
}

func TestDefaultConfigs(t *testing.T) {
	lmCfg := DefaultLockManagerConfig()
	if lmCfg.MaxReentrancy != DefaultMaxReentrancy {
		t.Errorf("Default MaxReentrancy mismatch")
	}
	if lmCfg.CleanInterval != DefaultCleanInterval {
		t.Errorf("Default CleanInterval mismatch")
	}

	rlCfg := DefaultRedlockConfig()
	if rlCfg.AcquireTimeout <= 0 {
		t.Error("Default AcquireTimeout should be positive")
	}
	if rlCfg.RetryDelay <= 0 {
		t.Error("Default RetryDelay should be positive")
	}
	if rlCfg.ClockDrift < 0 {
		t.Error("Default ClockDrift should not be negative")
	}
}
