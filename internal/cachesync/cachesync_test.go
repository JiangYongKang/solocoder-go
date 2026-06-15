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
		Type:       MsgUpdateNotify,
		FromNodeID: "other-node",
		Key:        "key1",
		Value:      "old-value",
		Version:    0,
		Timestamp:  time.Now(),
	}
	err := node1.handleUpdateNotify(oldMsg)
	if err == nil {
		t.Fatal("expected error for older version, got nil")
	}
	if !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got %v", err)
	}

	got := node1.Get("key1")
	if got.Value != "new-value" {
		t.Errorf("older version should not overwrite, expected 'new-value', got %v", got.Value)
	}
	if got.Version != 1 {
		t.Errorf("expected version 1, got %d", got.Version)
	}
	if node1.VersionRejectCount() != 1 {
		t.Errorf("expected reject count 1, got %d", node1.VersionRejectCount())
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

	sent, recv, rejectCount, cacheSize, lockCount := node1.Stats()
	if sent != 0 || recv != 0 || rejectCount != 0 || cacheSize != 0 || lockCount != 0 {
		t.Errorf("expected all zeros for fresh node, got sent=%d recv=%d rejects=%d cacheSize=%d lockCount=%d",
			sent, recv, rejectCount, cacheSize, lockCount)
	}

	node1.Set("key1", "v1")
	node1.Set("key2", "v2")

	_, _, _, cacheSize, _ = node1.Stats()
	if cacheSize != 2 {
		t.Errorf("expected cacheSize 2, got %d", cacheSize)
	}

	node1.Lock("key1", 5*time.Second)
	_, _, _, _, lockCount = node1.Stats()
	if lockCount != 1 {
		t.Errorf("expected lockCount 1, got %d", lockCount)
	}

	oldMsg := &Message{
		Type:       MsgUpdateNotify,
		FromNodeID: "x",
		Key:        "key1",
		Value:      "old",
		Version:    0,
		Timestamp:  time.Now(),
	}
	_ = node1.handleUpdateNotify(oldMsg)
	_, _, rejectCount, _, _ = node1.Stats()
	if rejectCount != 1 {
		t.Errorf("expected rejectCount 1 after old message, got %d", rejectCount)
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

func TestVersionRejectObservability(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")

	var receivedEvent VersionRejectEvent
	eventCh := make(chan struct{}, 1)
	node2.AddVersionRejectHandler(func(event VersionRejectEvent) {
		receivedEvent = event
		select {
		case eventCh <- struct{}{}:
		default:
		}
	})

	node2.AddVersionRejectHandler(nil)

	node2.Set("key-obs", "v2-latest")
	node2.Set("key-obs", "v2-latest")
	if node2.Get("key-obs").Version != 2 {
		t.Fatalf("expected version 2, got %d", node2.Get("key-obs").Version)
	}

	oldMsg := &Message{
		Type:       MsgUpdateNotify,
		FromNodeID: "node1",
		Key:        "key-obs",
		Value:      "old-from-node1",
		Version:    1,
		Timestamp:  time.Now(),
	}
	err := node2.handleUpdateNotify(oldMsg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld in chain, got %v", err)
	}

	select {
	case <-eventCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for reject handler to be called")
	}

	if receivedEvent.Key != "key-obs" {
		t.Errorf("expected key 'key-obs', got '%s'", receivedEvent.Key)
	}
	if receivedEvent.LocalVersion != 2 {
		t.Errorf("expected LocalVersion 2, got %d", receivedEvent.LocalVersion)
	}
	if receivedEvent.MsgVersion != 1 {
		t.Errorf("expected MsgVersion 1, got %d", receivedEvent.MsgVersion)
	}
	if receivedEvent.FromNodeID != "node1" {
		t.Errorf("expected FromNodeID 'node1', got '%s'", receivedEvent.FromNodeID)
	}
	if receivedEvent.RejectedAt.IsZero() {
		t.Error("RejectedAt should not be zero")
	}

	if node2.VersionRejectCount() != 1 {
		t.Errorf("expected VersionRejectCount=1, got %d", node2.VersionRejectCount())
	}

	got := node2.Get("key-obs")
	if got.Value != "v2-latest" {
		t.Errorf("old value should not overwrite, got %v", got.Value)
	}
	if got.Version != 2 {
		t.Errorf("version should remain 2, got %d", got.Version)
	}

	_ = node1
}

func setupCleanLocks(t *testing.T, nodes []*Node, key string) {
	t.Helper()
	for _, n := range nodes {
		n.lockMu.Lock()
		delete(n.locks, key)
		n.lockMu.Unlock()
	}
}

func TestLockRollbackAfterDenial(t *testing.T) {
	cfg := Config{
		LockTimeout:       100 * time.Millisecond,
		ReconcileInterval: 10 * time.Second,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	nodeA, _ := c.AddNode("A")
	nodeB, _ := c.AddNode("B")
	nodeC, _ := c.AddNode("C")

	_, err := nodeA.Lock("k", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("A Lock failed: %v", err)
	}

	waitForCondition(t, func() bool {
		return !nodeA.IsLocked("k") &&
			!nodeB.IsLocked("k") &&
			!nodeC.IsLocked("k")
	}, 3*time.Second, "all initial locks to expire (short TTL)")

	setupCleanLocks(t, []*Node{nodeA, nodeB, nodeC}, "k")

	nodeA.lockMu.Lock()
	nodeA.locks["k"] = &lockInfo{
		holder:     "A",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	nodeA.lockMu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		_, err := nodeB.Lock("k", 150*time.Millisecond)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("B Lock should fail: A manually holds lock")
		}
		if !errors.Is(err, ErrLockTimeout) {
			t.Errorf("expected ErrLockTimeout, got %v", err)
		}
		errStr := err.Error()
		if !contains(errStr, "A") {
			t.Errorf("error should mention holder A, got: %s", errStr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B Lock should have completed quickly (denied, not timeout)")
	}

	waitForCondition(t, func() bool {
		h := nodeC.GetLockHolder("k")
		return h == "" || h == "A"
	}, 5*time.Second, "C has either no temp lock or only A's lock")

	holderC := nodeC.GetLockHolder("k")
	if holderC != "" && holderC != "A" {
		t.Errorf("C holder should be '' or 'A' after B's failed attempt rollback, got '%s'. "+
			"Stale temp lock for B was not rolled back!", holderC)
	}

	if !nodeA.IsLocked("k") {
		t.Error("A should still hold the lock")
	}

	nodeA.lockMu.Lock()
	delete(nodeA.locks, "k")
	nodeA.lockMu.Unlock()

	waitForCondition(t, func() bool {
		return !nodeB.IsLocked("k") && !nodeC.IsLocked("k")
	}, 3*time.Second, "B and C clean after manual A unlock")

	holder, err := nodeC.Lock("k", 5*time.Second)
	if err != nil {
		t.Fatalf("C should acquire lock now (no stale B temp lock). Error: %v", err)
	}
	if holder != "C" {
		t.Errorf("expected holder=C, got '%s'", holder)
	}
	if err := nodeC.Unlock("k"); err != nil {
		t.Fatalf("C Unlock failed: %v", err)
	}

	waitForCondition(t, func() bool {
		return !nodeA.IsLocked("k") &&
			!nodeB.IsLocked("k") &&
			!nodeC.IsLocked("k")
	}, 3*time.Second, "all nodes see C's unlock broadcast")

	holder2, err := nodeB.Lock("k", 5*time.Second)
	if err != nil {
		t.Fatalf("B should acquire lock now afterwards. Error: %v", err)
	}
	if holder2 != "B" {
		t.Errorf("expected holder=B, got '%s'", holder2)
	}
	_ = nodeB.Unlock("k")
}

func TestLockTimeoutRollback(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node2, _ := c.AddNode("node2")
	node3, _ := c.AddNode("node3")

	_, err := node1.Lock("tk", 40*time.Millisecond)
	if err != nil {
		t.Fatalf("node1 Lock failed: %v", err)
	}

	waitForCondition(t, func() bool {
		return !node1.IsLocked("tk") &&
			!node2.IsLocked("tk") &&
			!node3.IsLocked("tk")
	}, 3*time.Second, "all locks expire")

	setupCleanLocks(t, []*Node{node1, node2, node3}, "tk")

	node1.lockMu.Lock()
	node1.locks["tk"] = &lockInfo{
		holder:     "node1",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	node1.lockMu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		_, err := node3.Lock("tk", 60*time.Millisecond)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("node3 Lock should fail: node1 manually holds lock")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("node3 Lock should complete quickly via denial")
	}

	waitForCondition(t, func() bool {
		h := node2.GetLockHolder("tk")
		return h == "" || h == "node1"
	}, 3*time.Second, "node2 clears stale temp lock for node3")

	holder2 := node2.GetLockHolder("tk")
	if holder2 != "" && holder2 != "node1" {
		t.Errorf("node2 has unexpected holder '%s' (expected '' or 'node1')", holder2)
	}

	node1.lockMu.Lock()
	delete(node1.locks, "tk")
	node1.lockMu.Unlock()

	waitForCondition(t, func() bool {
		return !node2.IsLocked("tk") && !node3.IsLocked("tk")
	}, 2*time.Second, "locks fully cleared")

	holder, err := node2.Lock("tk", 5*time.Second)
	if err != nil {
		t.Fatalf("node2 should acquire after rollback. Error: %v", err)
	}
	if holder != "node2" {
		t.Errorf("expected holder node2, got '%s'", holder)
	}
	_ = node2.Unlock("tk")
}

func TestVersionRejectHandlerPanicSafety(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")

	panicked := false
	node1.AddVersionRejectHandler(func(_ VersionRejectEvent) {
		panicked = true
		panic("handler panic test")
	})

	secondCalled := false
	node1.AddVersionRejectHandler(func(_ VersionRejectEvent) {
		secondCalled = true
	})

	node1.Set("panic-key", "latest")

	oldMsg := &Message{
		Type:       MsgUpdateNotify,
		FromNodeID: "x",
		Key:        "panic-key",
		Value:      "old",
		Version:    0,
		Timestamp:  time.Now(),
	}
	err := node1.handleUpdateNotify(oldMsg)
	if err == nil || !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld, got %v", err)
	}

	if !panicked {
		t.Error("first handler should have been called (and panicked)")
	}
	if !secondCalled {
		t.Error("second handler should still be called after first handler panic")
	}
}

func TestMultiNodeReconciliationWithConflicts(t *testing.T) {
	cfg := Config{
		LockTimeout:       5 * time.Second,
		ReconcileInterval: 10 * time.Second,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	c.SetMessageDropRate(1.0)

	node1, _ := c.AddNode("n1")
	node2, _ := c.AddNode("n2")
	node3, _ := c.AddNode("n3")
	node4, _ := c.AddNode("n4")

	node1.Set("alpha", "from-n1-v1")
	node2.Set("alpha", "from-n2-v1")
	node2.Set("alpha", "from-n2-v2")
	node3.Set("beta", "from-n3")
	node4.Set("gamma", "from-n4-v1")
	node4.Set("gamma", "from-n4-v2")
	node4.Set("gamma", "from-n4-v3")

	expectedAlphaVersions := map[string]uint64{
		"n1": node1.Get("alpha").Version,
		"n2": node2.Get("alpha").Version,
		"n3": 0,
		"n4": 0,
	}
	highestAlpha := uint64(0)
	for _, v := range expectedAlphaVersions {
		if v > highestAlpha {
			highestAlpha = v
		}
	}
	if highestAlpha == 0 {
		t.Fatal("highest alpha version should not be 0")
	}

	expectedGammaVersion := node4.Get("gamma").Version
	if expectedGammaVersion != 3 {
		t.Errorf("node4 gamma version expected 3, got %d", expectedGammaVersion)
	}

	c.SetMessageDropRate(0.0)
	c.runReconciliation()
	c.runReconciliation()

	waitForCondition(t, func() bool {
		return node1.Get("alpha") != nil &&
			node2.Get("alpha") != nil &&
			node3.Get("alpha") != nil &&
			node4.Get("alpha") != nil &&
			node1.Get("beta") != nil &&
			node2.Get("beta") != nil &&
			node3.Get("beta") != nil &&
			node4.Get("beta") != nil &&
			node1.Get("gamma") != nil &&
			node2.Get("gamma") != nil &&
			node3.Get("gamma") != nil &&
			node4.Get("gamma") != nil
	}, 5*time.Second, "all 4 nodes to converge on all 3 keys")

	type nodeEntry struct {
		n    *Node
		name string
	}
	nodes := []nodeEntry{
		{node1, "n1"}, {node2, "n2"}, {node3, "n3"}, {node4, "n4"},
	}

	for _, ne := range nodes {
		a := ne.n.Get("alpha")
		if a == nil {
			t.Errorf("%s missing alpha", ne.name)
			continue
		}
		if a.Version != highestAlpha {
			t.Errorf("%s alpha version=%d, expected %d",
				ne.name, a.Version, highestAlpha)
		}

		b := ne.n.Get("beta")
		if b == nil {
			t.Errorf("%s missing beta", ne.name)
			continue
		}
		if b.Version != 1 {
			t.Errorf("%s beta version=%d, expected 1", ne.name, b.Version)
		}
		if b.Value != "from-n3" {
			t.Errorf("%s beta value=%v, expected 'from-n3'", ne.name, b.Value)
		}

		g := ne.n.Get("gamma")
		if g == nil {
			t.Errorf("%s missing gamma", ne.name)
			continue
		}
		if g.Version != expectedGammaVersion {
			t.Errorf("%s gamma version=%d, expected %d",
				ne.name, g.Version, expectedGammaVersion)
		}
		if g.Value != "from-n4-v3" {
			t.Errorf("%s gamma value=%v, expected 'from-n4-v3'", ne.name, g.Value)
		}
	}
}

func TestAddNilVersionRejectHandler(t *testing.T) {
	c := NewCluster(DefaultConfig())
	defer c.Stop()

	node1, _ := c.AddNode("node1")
	node1.AddVersionRejectHandler(nil)

	node1.Set("k", "v2")
	node1.Set("k", "v2")

	oldMsg := &Message{
		Type:      MsgUpdateNotify,
		Key:       "k",
		Value:     "v0",
		Version:   0,
		Timestamp: time.Now(),
	}
	err := node1.handleUpdateNotify(oldMsg)
	if !errors.Is(err, ErrVersionTooOld) {
		t.Errorf("expected ErrVersionTooOld even with nil handler registered, got %v", err)
	}
}

func TestLockRollbackWithFourNodes(t *testing.T) {
	cfg := Config{
		LockTimeout:       50 * time.Millisecond,
		ReconcileInterval: 10 * time.Second,
		MessageBuffer:     1024,
	}
	c := NewCluster(cfg)
	defer c.Stop()

	nodeA, _ := c.AddNode("A")
	nodeB, _ := c.AddNode("B")
	nodeC, _ := c.AddNode("C")
	nodeD, _ := c.AddNode("D")

	_, err := nodeA.Lock("rx", 30*time.Millisecond)
	if err != nil {
		t.Fatalf("A Lock failed: %v", err)
	}

	waitForCondition(t, func() bool {
		return !nodeA.IsLocked("rx") &&
			!nodeB.IsLocked("rx") &&
			!nodeC.IsLocked("rx") &&
			!nodeD.IsLocked("rx")
	}, 5*time.Second, "all initial short-TTL locks expire")

	setupCleanLocks(t, []*Node{nodeA, nodeB, nodeC, nodeD}, "rx")

	nodeA.lockMu.Lock()
	nodeA.locks["rx"] = &lockInfo{
		holder:     "A",
		expiresAt:  time.Now().Add(10 * time.Second),
		acquiredAt: time.Now(),
	}
	nodeA.lockMu.Unlock()

	errCh := make(chan error, 1)
	go func() {
		_, err := nodeB.Lock("rx", 50*time.Millisecond)
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("B Lock should fail, A manually holds lock")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("B Lock should complete quickly (denied)")
	}

	waitForCondition(t, func() bool {
		hc := nodeC.GetLockHolder("rx")
		hd := nodeD.GetLockHolder("rx")
		return (hc == "" || hc == "A") && (hd == "" || hd == "A")
	}, 5*time.Second, "C and D to have no stale temp locks for B")

	hc := nodeC.GetLockHolder("rx")
	hd := nodeD.GetLockHolder("rx")
	if hc != "" && hc != "A" {
		t.Errorf("C has stale temp lock holder=%s (expected '' or 'A')", hc)
	}
	if hd != "" && hd != "A" {
		t.Errorf("D has stale temp lock holder=%s (expected '' or 'A')", hd)
	}

	nodeA.lockMu.Lock()
	delete(nodeA.locks, "rx")
	nodeA.lockMu.Unlock()

	waitForCondition(t, func() bool {
		return !nodeB.IsLocked("rx") &&
			!nodeC.IsLocked("rx") &&
			!nodeD.IsLocked("rx")
	}, 3*time.Second, "B/C/D to see A's manual unlock")

	holder, err := nodeC.Lock("rx", 5*time.Second)
	if err != nil {
		t.Fatalf("C should acquire after rollback. Error: %v", err)
	}
	if holder != "C" {
		t.Errorf("expected holder=C, got %s", holder)
	}
	if err := nodeC.Unlock("rx"); err != nil {
		t.Fatalf("C Unlock failed: %v", err)
	}
	waitForCondition(t, func() bool {
		return !nodeA.IsLocked("rx") &&
			!nodeB.IsLocked("rx") &&
			!nodeC.IsLocked("rx") &&
			!nodeD.IsLocked("rx")
	}, 5*time.Second, "all four nodes see C's unlock")
}
