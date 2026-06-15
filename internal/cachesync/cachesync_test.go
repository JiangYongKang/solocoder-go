package cachesync

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for: %s", msg)
}

func TestNewCluster(t *testing.T) {
	c := NewCluster(DefaultConfig())
	if c == nil {
		t.Fatal("NewCluster returned nil")
	}
	if c.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", c.NodeCount())
	}
	c.Stop()
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LockTimeout != defaultLockTimeout {
		t.Errorf("expected default LockTimeout %v, got %v", defaultLockTimeout, cfg.LockTimeout)
	}
	if cfg.ReconcileInterval != defaultReconcileInterval {
		t.Errorf("expected default ReconcileInterval %v, got %v", defaultReconcileInterval, cfg.ReconcileInterval)
	}
	if cfg.MessageBuffer != defaultMessageBuffer {
		t.Errorf("expected default MessageBuffer %d, got %d", defaultMessageBuffer, cfg.MessageBuffer)
	}

	cfg2 := Config{}
	c := NewCluster(cfg2)
	if c.cfg.LockTimeout != defaultLockTimeout {
		t.Errorf("expected zero LockTimeout to default to %v, got %v", defaultLockTimeout, c.cfg.LockTimeout)
	}
	if c.cfg.ReconcileInterval != defaultReconcileInterval {
		t.Errorf("expected zero ReconcileInterval to default to %v, got %v", defaultReconcileInterval, c.cfg.ReconcileInterval)
	}
	if c.cfg.MessageBuffer != defaultMessageBuffer {
		t.Errorf("expected zero MessageBuffer to default to %d, got %d", defaultMessageBuffer, c.cfg.MessageBuffer)
	}
	c.Stop()
}

func TestAddNode(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, err := c.AddNode("node1")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	if node1 == nil {
		t.Fatal("node1 is nil")
	}
	if node1.ID != "node1" {
		t.Errorf("expected node ID 'node1', got '%s'", node1.ID)
	}
	if c.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", c.NodeCount())
	}

	_, err = c.AddNode("node1")
	if !errors.Is(err, ErrNodeExists) {
		t.Errorf("expected ErrNodeExists, got %v", err)
	}

	_, err = c.AddNode("")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestRemoveNode(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	_, err := c.AddNode("node1")
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}

	err = c.RemoveNode("node1")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}
	if c.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", c.NodeCount())
	}

	err = c.RemoveNode("node1")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestGetNode(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	c.AddNode("node1")

	node, err := c.GetNode("node1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if node == nil {
		t.Fatal("node is nil")
	}
	if node.ID != "node1" {
		t.Errorf("expected node ID 'node1', got '%s'", node.ID)
	}

	_, err = c.GetNode("nonexistent")
	if !errors.Is(err, ErrNodeNotFound) {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}
}

func TestCacheSetAndGet(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	entry := node1.Set("key1", "value1")
	if entry == nil {
		t.Fatal("Set returned nil")
	}
	if entry.Key != "key1" {
		t.Errorf("expected key 'key1', got '%s'", entry.Key)
	}
	if entry.Value != "value1" {
		t.Errorf("expected value 'value1', got %v", entry.Value)
	}
	if entry.Version != 1 {
		t.Errorf("expected version 1, got %d", entry.Version)
	}
	if entry.NodeID != "node1" {
		t.Errorf("expected nodeID 'node1', got '%s'", entry.NodeID)
	}

	got := node1.Get("key1")
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Value != "value1" {
		t.Errorf("expected value 'value1', got %v", got.Value)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}

	gotNil := node1.Get("nonexistent")
	if gotNil != nil {
		t.Error("expected nil for nonexistent key")
	}
}

func TestCacheVersionIncrement(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "v1")
	entry := node1.Set("key1", "v2")
	if entry.Version != 2 {
		t.Errorf("expected version 2, got %d", entry.Version)
	}

	entry = node1.Set("key1", "v3")
	if entry.Version != 3 {
		t.Errorf("expected version 3, got %d", entry.Version)
	}

	got := node1.Get("key1")
	if got.Version != 3 {
		t.Errorf("expected version 3, got %d", got.Version)
	}
	if got.Value != "v3" {
		t.Errorf("expected value 'v3', got %v", got.Value)
	}
}

func TestVersionedUpdateNotification(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")
	node3, _ := c.AddNode("node3")

	node1.Set("key1", "value1")

	waitForCondition(t, func() bool {
		return node2.Get("key1") != nil && node3.Get("key1") != nil
	}, 2*time.Second, "nodes to receive update notification")

	got2 := node2.Get("key1")
	if got2 == nil {
		t.Fatal("node2 did not receive key1")
	}
	if got2.Value != "value1" {
		t.Errorf("node2 expected value 'value1', got %v", got2.Value)
	}
	if got2.Version != 1 {
		t.Errorf("node2 expected version 1, got %d", got2.Version)
	}

	got3 := node3.Get("key1")
	if got3 == nil {
		t.Fatal("node3 did not receive key1")
	}
	if got3.Value != "value1" {
		t.Errorf("node3 expected value 'value1', got %v", got3.Value)
	}

	node1.Set("key1", "value2")

	waitForCondition(t, func() bool {
		g2 := node2.Get("key1")
		g3 := node3.Get("key1")
		return g2 != nil && g2.Version == 2 && g3 != nil && g3.Version == 2
	}, 2*time.Second, "nodes to receive updated version")

	got2 = node2.Get("key1")
	if got2.Version != 2 {
		t.Errorf("node2 expected version 2, got %d", got2.Version)
	}
	if got2.Value != "value2" {
		t.Errorf("node2 expected value 'value2', got %v", got2.Value)
	}
}

func TestOlderVersionRejected(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "new-value")

	oldMsg := &Message{
		Type:      MsgUpdateNotify,
		Key:       "key1",
		Value:     "old-value",
		Version:   0,
		Timestamp: time.Now(),
	}
	node1.handleUpdateNotify(oldMsg)

	got := node1.Get("key1")
	if got.Value != "new-value" {
		t.Errorf("older version should not overwrite, expected 'new-value', got %v", got.Value)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
}

func TestCacheDelete(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "value1")
	if node1.Get("key1") == nil {
		t.Fatal("expected key1 to exist")
	}

	deleted := node1.Delete("key1")
	if !deleted {
		t.Error("Delete should return true for existing key")
	}
	if node1.Get("key1") != nil {
		t.Error("key1 should be deleted")
	}

	deleted = node1.Delete("nonexistent")
	if deleted {
		t.Error("Delete should return false for nonexistent key")
	}
}

func TestInvalidateBroadcast(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")
	node3, _ := c.AddNode("node3")

	node1.SetWithInvalidate("key1", "value1")

	time.Sleep(100 * time.Millisecond)

	if node1.Get("key1") == nil {
		t.Error("node1 should still have key1 after SetWithInvalidate")
	}
	if node2.Get("key1") != nil {
		t.Error("node2 should have key1 invalidated (deleted)")
	}
	if node3.Get("key1") != nil {
		t.Error("node3 should have key1 invalidated (deleted)")
	}
}

func TestDeleteInvalidatesOtherNodes(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")
	node3, _ := c.AddNode("node3")

	node1.Set("key1", "value1")
	waitForCondition(t, func() bool {
		return node2.Get("key1") != nil && node3.Get("key1") != nil
	}, 2*time.Second, "nodes to receive initial value")

	node1.Delete("key1")

	waitForCondition(t, func() bool {
		return node2.Get("key1") == nil && node3.Get("key1") == nil
	}, 2*time.Second, "nodes to receive delete invalidation")

	if node2.Get("key1") != nil {
		t.Error("node2 should have key1 deleted via invalidate broadcast")
	}
	if node3.Get("key1") != nil {
		t.Error("node3 should have key1 deleted via invalidate broadcast")
	}
}

func TestLockAcquireAndRelease(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	holder, err := node1.Lock("key1", 5*time.Second)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if holder != "node1" {
		t.Errorf("expected holder 'node1', got '%s'", holder)
	}

	if !node1.IsLocked("key1") {
		t.Error("key1 should be locked")
	}
	if node1.GetLockHolder("key1") != "node1" {
		t.Error("lock holder should be node1")
	}

	err = node1.Unlock("key1")
	if err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	if node1.IsLocked("key1") {
		t.Error("key1 should not be locked after unlock")
	}
	if node1.GetLockHolder("key1") != "" {
		t.Error("lock holder should be empty after unlock")
	}
}

func TestUnlockNotHeld(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	err := node1.Unlock("key1")
	if !errors.Is(err, ErrLockNotHeld) {
		t.Errorf("expected ErrLockNotHeld, got %v", err)
	}
}

func TestLockTimeout(t *testing.T) {
	cfg := Config{
		LockTimeout:       100 * time.Millisecond,
		ReconcileInterval: 1 * time.Second,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	_, err := node1.Lock("key1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	if !node1.IsLocked("key1") {
		t.Error("key1 should be locked")
	}

	time.Sleep(100 * time.Millisecond)

	if node1.IsLocked("key1") {
		t.Error("key1 should be unlocked after timeout")
	}
}

func TestLockContention(t *testing.T) {
	cfg := Config{
		LockTimeout:       2 * time.Second,
		ReconcileInterval: 1 * time.Second,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	_, err := node1.Lock("key1", 5*time.Second)
	if err != nil {
		t.Fatalf("node1 Lock failed: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := node2.Lock("key1", 100*time.Millisecond)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrLockTimeout) {
			t.Errorf("expected ErrLockTimeout, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("node2 Lock should have timed out")
	}
}

func TestLockAfterRelease(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	_, err := node1.Lock("key1", 5*time.Second)
	if err != nil {
		t.Fatalf("node1 Lock failed: %v", err)
	}

	err = node1.Unlock("key1")
	if err != nil {
		t.Fatalf("node1 Unlock failed: %v", err)
	}

	waitForCondition(t, func() bool {
		return !node2.IsLocked("key1")
	}, 2*time.Second, "node2 to process lock release message")

	holder, err := node2.Lock("key1", 2*time.Second)
	if err != nil {
		t.Fatalf("node2 Lock failed after release: %v", err)
	}
	if holder != "node2" {
		t.Errorf("expected holder 'node2', got '%s'", holder)
	}
}

func TestLockHolderDiagnostic(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	_, err := node1.Lock("key1", 5*time.Second)
	if err != nil {
		t.Fatalf("node1 Lock failed: %v", err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := node2.Lock("key1", 50*time.Millisecond)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from node2 Lock")
		}
		errStr := err.Error()
		if !contains(errStr, "node1") {
			t.Errorf("error message should contain holder 'node1', got: %s", errStr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("node2 Lock should have timed out")
	}
}

func TestReconciliation(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")
	node3, _ := c.AddNode("node3")

	c.SetMessageDropRate(1.0)

	node1.Set("key1", "value-from-node1")
	node2.Set("key2", "value-from-node2")
	node3.Set("key1", "value-from-node3-v2")

	time.Sleep(100 * time.Millisecond)

	c.SetMessageDropRate(0.0)

	c.runReconciliation()

	waitForCondition(t, func() bool {
		e1 := node1.Get("key1")
		e2 := node2.Get("key2")
		e3 := node3.Get("key1")
		return e1 != nil && e1.Version >= 1 && e2 != nil && e3 != nil
	}, 3*time.Second, "reconciliation to complete")

	got1 := node1.Get("key1")
	if got1 == nil {
		t.Fatal("node1 should have key1 after reconciliation")
	}
	got3 := node3.Get("key1")
	if got3 == nil {
		t.Fatal("node3 should have key1 after reconciliation")
	}

	highestVersion := got1.Version
	if got3.Version > highestVersion {
		highestVersion = got3.Version
	}

	allEntries := [][]interface{}{
		{node1.Get("key1"), "node1 key1"},
		{node2.Get("key1"), "node2 key1"},
		{node3.Get("key1"), "node3 key1"},
	}

	for _, item := range allEntries {
		entry := item[0].(*CacheEntry)
		name := item[1].(string)
		if entry == nil {
			t.Errorf("%s should not be nil after reconciliation", name)
			continue
		}
		if entry.Version != highestVersion {
			t.Errorf("%s expected version %d, got %d", name, highestVersion, entry.Version)
		}
	}

	got2_k2 := node2.Get("key2")
	if got2_k2 == nil {
		t.Error("node2 should have key2")
	}
}

func TestAutomaticReconciler(t *testing.T) {
	cfg := Config{
		LockTimeout:       5 * time.Second,
		ReconcileInterval: 50 * time.Millisecond,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	c.SetMessageDropRate(1.0)

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	node1.Set("key1", "value1")
	node2.Set("key2", "value2")

	time.Sleep(100 * time.Millisecond)

	c.SetMessageDropRate(0.0)

	c.StartReconciler()

	waitForCondition(t, func() bool {
		return node1.Get("key2") != nil && node2.Get("key1") != nil
	}, 3*time.Second, "automatic reconciliation to sync")

	if node1.Get("key2") == nil {
		t.Error("node1 should have key2 after automatic reconciliation")
	}
	if node2.Get("key1") == nil {
		t.Error("node2 should have key1 after automatic reconciliation")
	}
}

func TestGetAll(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "v1")
	node1.Set("key2", "v2")
	node1.Set("key3", "v3")

	entries := node1.GetAll()
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	keys := make(map[string]bool)
	for _, e := range entries {
		keys[e.Key] = true
	}
	for _, k := range []string{"key1", "key2", "key3"} {
		if !keys[k] {
			t.Errorf("missing key %s in GetAll result", k)
		}
	}
}

func TestStats(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	sent, recv, cacheSize, lockCount := node1.Stats()
	if sent != 0 || recv != 0 || cacheSize != 0 || lockCount != 0 {
		t.Errorf("expected all zeros for fresh node, got sent=%d recv=%d cacheSize=%d lockCount=%d",
			sent, recv, cacheSize, lockCount)
	}

	node1.Set("key1", "v1")
	node1.Set("key2", "v2")

	_, _, cacheSize, _ = node1.Stats()
	if cacheSize != 2 {
		t.Errorf("expected cacheSize 2, got %d", cacheSize)
	}

	node1.Lock("key1", 5*time.Second)
	_, _, _, lockCount = node1.Stats()
	if lockCount != 1 {
		t.Errorf("expected lockCount 1, got %d", lockCount)
	}
}

func TestConcurrentSets(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				node1.Set("key", id*1000+j)
			}
		}(i)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				node2.Set("key", id*10000+j)
			}
		}(i)
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	entry1 := node1.Get("key")
	entry2 := node2.Get("key")
	if entry1 == nil || entry2 == nil {
		t.Fatal("both nodes should have 'key'")
	}
	if entry1.Version != entry2.Version {
		t.Errorf("versions should converge: node1=%d node2=%d", entry1.Version, entry2.Version)
	}
}

func TestMultipleKeysBroadcast(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	numKeys := 50
	for i := 0; i < numKeys; i++ {
		key := "key" + string(rune('0'+i%10)) + "-" + string(rune('0'+i/10))
		node1.Set(key, i)
	}

	waitForCondition(t, func() bool {
		return len(node2.GetAll()) == numKeys
	}, 3*time.Second, "node2 to receive all keys")

	entries := node2.GetAll()
	if len(entries) != numKeys {
		t.Errorf("expected %d entries on node2, got %d", numKeys, len(entries))
	}
}

func TestClusterStopTwice(t *testing.T) {
	c := NewCluster(DefaultConfig())
	c.Stop()
	c.Stop()
}

func TestAddNodeAfterStop(t *testing.T) {
	c := NewCluster(DefaultConfig())
	c.Stop()

	_, err := c.AddNode("node1")
	if !errors.Is(err, ErrClusterStopped) {
		t.Errorf("expected ErrClusterStopped, got %v", err)
	}
}

func TestHandleInvalidateMessage(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "value1")
	if node1.Get("key1") == nil {
		t.Fatal("key1 should exist")
	}

	msg := &Message{
		Type:      MsgInvalidate,
		Key:       "key1",
		Timestamp: time.Now(),
	}
	node1.handleInvalidate(msg)

	if node1.Get("key1") != nil {
		t.Error("key1 should be deleted after invalidate message")
	}
}

func TestHandleLockReleaseFromOtherNode(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.lockMu.Lock()
	node1.locks["key1"] = &lockInfo{
		holder:     "node2",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	node1.lockMu.Unlock()

	if !node1.IsLocked("key1") {
		t.Error("key1 should be locked")
	}

	releaseMsg := &Message{
		Type:       MsgLockRelease,
		FromNodeID: "node2",
		Key:        "key1",
		Timestamp:  time.Now(),
	}
	node1.handleLockRelease(releaseMsg)

	if node1.IsLocked("key1") {
		t.Error("key1 should be unlocked after release message from holder")
	}
}

func TestHandleLockReleaseFromNonHolder(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.lockMu.Lock()
	node1.locks["key1"] = &lockInfo{
		holder:     "node2",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	node1.lockMu.Unlock()

	releaseMsg := &Message{
		Type:       MsgLockRelease,
		FromNodeID: "node3",
		Key:        "key1",
		Timestamp:  time.Now(),
	}
	node1.handleLockRelease(releaseMsg)

	if !node1.IsLocked("key1") {
		t.Error("lock should still be held by node2, non-holder cannot release")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestHandleLockAcquireDenied(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.lockMu.Lock()
	node1.locks["key1"] = &lockInfo{
		holder:     "node1",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	node1.lockMu.Unlock()

	node2, _ := c.AddNode("node2")

	go func() {
		select {
		case <-node2.inbox:
		case <-time.After(500 * time.Millisecond):
		}
	}()

	acquireMsg := &Message{
		Type:       MsgLockAcquire,
		FromNodeID: "node2",
		Key:        "key1",
		LockTTL:    5 * time.Second,
		Timestamp:  time.Now(),
	}
	node1.handleLockAcquire(acquireMsg)

	time.Sleep(100 * time.Millisecond)
}

func TestHandleLockAcquireGranted(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	acquireMsg := &Message{
		Type:       MsgLockAcquire,
		FromNodeID: "node2",
		Key:        "key1",
		LockTTL:    5 * time.Second,
		Timestamp:  time.Now(),
	}
	node1.handleLockAcquire(acquireMsg)

	if node1.GetLockHolder("key1") != "node2" {
		t.Errorf("expected holder node2, got '%s'", node1.GetLockHolder("key1"))
	}
}

func TestMessageTypes(t *testing.T) {
	if MsgUpdateNotify == MsgInvalidate {
		t.Error("message types should be distinct")
	}
	if MsgLockAcquire == MsgLockRelease {
		t.Error("lock message types should be distinct")
	}
	types := []MessageType{
		MsgUpdateNotify,
		MsgInvalidate,
		MsgReconcileRequest,
		MsgReconcileResponse,
		MsgLockAcquire,
		MsgLockRelease,
		MsgLockGranted,
		MsgLockDenied,
	}
	seen := make(map[MessageType]bool)
	for _, mt := range types {
		if seen[mt] {
			t.Errorf("duplicate message type value: %d", mt)
		}
		seen[mt] = true
	}
}

func TestEntryIsolation(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	node1.Set("key1", "original")
	entry := node1.Get("key1")

	entry.Value = "modified"
	entry.Version = 999

	stillOriginal := node1.Get("key1")
	if stillOriginal.Value != "original" {
		t.Error("Get should return a copy, modification should not affect cache")
	}
	if stillOriginal.Version != 1 {
		t.Error("Get should return a copy, version modification should not affect cache")
	}
}
