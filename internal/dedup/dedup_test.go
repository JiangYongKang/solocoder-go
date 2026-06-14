package dedup

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewDeduplicator(t *testing.T) {
	d := NewDeduplicator()
	if d == nil {
		t.Fatal("NewDeduplicator returned nil")
	}
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected Count=0, got %d", count)
	}
}

func TestNewDeduplicatorWithConfig_Defaults(t *testing.T) {
	d, err := NewDeduplicatorWithConfig(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil {
		t.Fatal("NewDeduplicatorWithConfig returned nil")
	}
	if d.cfg.WindowSize <= 0 {
		t.Error("WindowSize should have default value")
	}
	if d.cfg.CleanInterval <= 0 {
		t.Error("CleanInterval should have default value")
	}
}

func TestNewDeduplicatorWithConfig_InvalidConfig(t *testing.T) {
	_, err := NewDeduplicatorWithConfig(Config{
		WindowSize: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative WindowSize, got %v", err)
	}

	_, err = NewDeduplicatorWithConfig(Config{
		CleanInterval: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative CleanInterval, got %v", err)
	}
}

func TestNewDeduplicatorWithConfig_CleanIntervalFromWindow(t *testing.T) {
	window := 10 * time.Second
	d, err := NewDeduplicatorWithConfig(Config{
		WindowSize: window,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := window / 5
	if d.cfg.CleanInterval != expected {
		t.Errorf("expected CleanInterval=%v, got %v", expected, d.cfg.CleanInterval)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.WindowSize != 5*time.Minute {
		t.Errorf("expected WindowSize=5m, got %v", cfg.WindowSize)
	}
	if cfg.CleanInterval != 1*time.Minute {
		t.Errorf("expected CleanInterval=1m, got %v", cfg.CleanInterval)
	}
}

func TestCheckAndMark_NewMessage(t *testing.T) {
	d := NewDeduplicator()

	ok, err := d.CheckAndMark("msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected new message to pass")
	}
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected Count=1, got %d", count)
	}
}

func TestCheckAndMark_DuplicateMessage(t *testing.T) {
	d := NewDeduplicator()

	ok, err := d.CheckAndMark("msg-dup")
	if err != nil {
		t.Fatalf("first CheckAndMark error: %v", err)
	}
	if !ok {
		t.Fatal("first CheckAndMark should pass")
	}

	ok, err = d.CheckAndMark("msg-dup")
	if err != nil {
		t.Fatalf("second CheckAndMark error: %v", err)
	}
	if ok {
		t.Error("duplicate message should be rejected")
	}
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected Count=1 (no duplicate added), got %d", count)
	}
}

func TestCheckAndMark_MultipleMessages(t *testing.T) {
	d := NewDeduplicator()
	n := 100

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("msg-%d", i)
		ok, err := d.CheckAndMark(id)
		if err != nil {
			t.Fatalf("CheckAndMark %s error: %v", id, err)
		}
		if !ok {
			t.Errorf("message %s should pass", id)
		}
	}

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != n {
		t.Errorf("expected Count=%d, got %d", n, count)
	}

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("msg-%d", i)
		ok, err := d.CheckAndMark(id)
		if err != nil {
			t.Fatalf("dup CheckAndMark %s error: %v", id, err)
		}
		if ok {
			t.Errorf("duplicate %s should be rejected", id)
		}
	}

	count, err = d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != n {
		t.Errorf("expected Count unchanged=%d, got %d", n, count)
	}
}

func TestCheckAndMark_EmptyMessageID(t *testing.T) {
	d := NewDeduplicator()

	ok, err := d.CheckAndMark("")
	if !errors.Is(err, ErrEmptyMessageID) {
		t.Errorf("expected ErrEmptyMessageID, got %v", err)
	}
	if ok {
		t.Error("empty message id should not pass")
	}
	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected Count=0, got %d", count)
	}
}

func TestContains(t *testing.T) {
	d := NewDeduplicator()

	exists, err := d.Contains("not-exist")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if exists {
		t.Error("Contains should return false for non-existent id")
	}

	_, _ = d.CheckAndMark("exist-1")
	exists, err = d.Contains("exist-1")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if !exists {
		t.Error("Contains should return true for existing id")
	}

	exists, err = d.Contains("")
	if !errors.Is(err, ErrEmptyMessageID) {
		t.Errorf("expected ErrEmptyMessageID, got %v", err)
	}
	if exists {
		t.Error("Contains should return false for empty id")
	}
}

func TestContains_Expired(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 50 * time.Millisecond,
	})

	_, _ = d.CheckAndMark("expired-msg")
	exists, _ := d.Contains("expired-msg")
	if !exists {
		t.Fatal("message should be contained initially")
	}

	time.Sleep(100 * time.Millisecond)

	exists, err := d.Contains("expired-msg")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if exists {
		t.Error("expired message should not be contained")
	}
}

func TestCount_ExcludesExpired(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 100 * time.Millisecond,
	})

	for i := 0; i < 5; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("count-early-%d", i))
	}

	time.Sleep(70 * time.Millisecond)

	for i := 0; i < 5; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("count-late-%d", i))
	}

	time.Sleep(50 * time.Millisecond)

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected Count=5 (only late entries within window), got %d", count)
	}

	if len(d.idMap) != 10 {
		t.Errorf("idMap should still have 10 entries (not yet cleaned), got %d", len(d.idMap))
	}
}

func TestClear(t *testing.T) {
	d := NewDeduplicator()

	for i := 0; i < 50; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("clear-%d", i))
	}
	count, _ := d.Count()
	if count != 50 {
		t.Fatalf("expected 50 entries, got %d", count)
	}

	err := d.Clear()
	if err != nil {
		t.Fatalf("Clear error: %v", err)
	}
	count, _ = d.Count()
	if count != 0 {
		t.Errorf("expected Count=0 after Clear, got %d", count)
	}

	for i := 0; i < 50; i++ {
		ok, _ := d.CheckAndMark(fmt.Sprintf("clear-%d", i))
		if !ok {
			t.Errorf("cleared message %d should pass again", i)
		}
	}
}

func TestCleanExpired_NoExpired(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 1 * time.Hour,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("fresh-%d", i))
	}

	cleaned, err := d.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned, got %d", cleaned)
	}
	count, _ := d.Count()
	if count != 10 {
		t.Errorf("expected Count=10, got %d", count)
	}
}

func TestCleanExpired_AllExpired(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 30 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("exp-all-%d", i))
	}

	time.Sleep(80 * time.Millisecond)

	cleaned, err := d.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 10 {
		t.Errorf("expected 10 cleaned, got %d", cleaned)
	}
	count, _ := d.Count()
	if count != 0 {
		t.Errorf("expected Count=0 after cleaning all, got %d", count)
	}
	if len(d.idMap) != 0 {
		t.Errorf("idMap should be empty after cleaning all, got %d", len(d.idMap))
	}
}

func TestCleanExpired_PartialExpired(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 100 * time.Millisecond,
	})

	for i := 0; i < 5; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("early-%d", i))
	}

	time.Sleep(70 * time.Millisecond)

	for i := 0; i < 5; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("late-%d", i))
	}

	time.Sleep(50 * time.Millisecond)

	cleaned, err := d.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 5 {
		t.Errorf("expected 5 cleaned (early ones), got %d", cleaned)
	}
	count, _ := d.Count()
	if count != 5 {
		t.Errorf("expected Count=5 (late ones remain), got %d", count)
	}

	for i := 0; i < 5; i++ {
		exists, _ := d.Contains(fmt.Sprintf("late-%d", i))
		if !exists {
			t.Errorf("late-%d should still be contained", i)
		}
		exists, _ = d.Contains(fmt.Sprintf("early-%d", i))
		if exists {
			t.Errorf("early-%d should be cleaned", i)
		}
	}
}

func TestCheckAndMark_ExpiredThenReaccept(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 40 * time.Millisecond,
	})

	ok1, err := d.CheckAndMark("reaccept-msg")
	if err != nil {
		t.Fatalf("first check error: %v", err)
	}
	if !ok1 {
		t.Fatal("first check should pass")
	}

	time.Sleep(100 * time.Millisecond)

	ok2, err := d.CheckAndMark("reaccept-msg")
	if err != nil {
		t.Fatalf("second check (after expiry) error: %v", err)
	}
	if !ok2 {
		t.Error("expired message should be re-accepted")
	}
	count, _ := d.Count()
	if count != 1 {
		t.Errorf("expected Count=1 (fresh entry), got %d", count)
	}
}

func TestCheckAndMark_TouchOnAccess(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 80 * time.Millisecond,
	})

	_, _ = d.CheckAndMark("touch-msg")

	time.Sleep(50 * time.Millisecond)

	_, _ = d.CheckAndMark("touch-msg")

	time.Sleep(50 * time.Millisecond)

	exists, _ := d.Contains("touch-msg")
	if !exists {
		t.Error("touched message should still be within window after second access refreshed it")
	}

	time.Sleep(60 * time.Millisecond)

	exists, _ = d.Contains("touch-msg")
	if exists {
		t.Error("message should expire after window passes without access")
	}
}

func TestStartStop_Idempotent(t *testing.T) {
	d := NewDeduplicator()

	d.Start()
	d.Start()

	d.Stop()
	d.Stop()

	done := make(chan struct{})
	go func() {
		d.Start()
		d.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start/Stop deadlocked")
	}
}

func TestStop_RejectsAllOperations(t *testing.T) {
	d := NewDeduplicator()
	d.Start()

	_, _ = d.CheckAndMark("before-stop")
	d.Stop()

	_, err := d.CheckAndMark("after-stop")
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop from CheckAndMark after Stop, got %v", err)
	}

	_, err = d.Contains("before-stop")
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop from Contains after Stop, got %v", err)
	}

	_, err = d.Count()
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop from Count after Stop, got %v", err)
	}

	_, err = d.CleanExpired()
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop from CleanExpired after Stop, got %v", err)
	}

	err = d.Clear()
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop from Clear after Stop, got %v", err)
	}
}

func TestStop_WithoutStart(t *testing.T) {
	d := NewDeduplicator()

	d.Stop()

	_, err := d.CheckAndMark("msg")
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("expected ErrDeduplicatorStop after Stop (without prior Start), got %v", err)
	}
}

func TestStart_AfterStop(t *testing.T) {
	d := NewDeduplicator()
	d.Start()
	d.Stop()

	d.Start()

	_, err := d.CheckAndMark("after-restart")
	if !errors.Is(err, ErrDeduplicatorStop) {
		t.Errorf("Start after Stop should not revive the deduplicator, expected ErrDeduplicatorStop, got %v", err)
	}
}

func TestStartStop_BackgroundCleanup(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize:    30 * time.Millisecond,
		CleanInterval: 20 * time.Millisecond,
	})

	d.Start()
	defer d.Stop()

	for i := 0; i < 20; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("bg-%d", i))
	}
	count, _ := d.Count()
	if count != 20 {
		t.Fatalf("expected 20 entries, got %d", count)
	}

	time.Sleep(150 * time.Millisecond)

	finalCount, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if finalCount != 0 {
		t.Errorf("expected Count=0 after background cleanup, got %d", finalCount)
	}
	if len(d.idMap) != 0 {
		t.Errorf("idMap should be empty after background cleanup, got %d", len(d.idMap))
	}
}

func TestConcurrent_CheckAndMark(t *testing.T) {
	d := NewDeduplicator()
	d.Start()
	defer d.Stop()

	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 100

	var passed int64
	var rejected int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := fmt.Sprintf("concurrent-%d-%d", gid, i)
				ok, err := d.CheckAndMark(id)
				if err != nil {
					if !errors.Is(err, ErrDeduplicatorStop) {
						t.Errorf("goroutine %d iteration %d error: %v", gid, i, err)
					}
					return
				}
				if ok {
					atomic.AddInt64(&passed, 1)
				} else {
					atomic.AddInt64(&rejected, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	if passed != expected {
		t.Errorf("expected %d passed, got %d passed, %d rejected", expected, passed, rejected)
	}
	count, _ := d.Count()
	if count != int(expected) {
		t.Errorf("expected Count=%d, got %d", expected, count)
	}
}

func TestConcurrent_Duplicates(t *testing.T) {
	d := NewDeduplicator()
	d.Start()
	defer d.Stop()

	var wg sync.WaitGroup
	numGoroutines := 30
	messageCount := 50

	var passCount int64
	var rejectCount int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < messageCount; i++ {
				id := fmt.Sprintf("dup-msg-%d", i)
				ok, err := d.CheckAndMark(id)
				if err != nil {
					return
				}
				if ok {
					atomic.AddInt64(&passCount, 1)
				} else {
					atomic.AddInt64(&rejectCount, 1)
				}
			}
		}()
	}

	wg.Wait()

	if passCount != int64(messageCount) {
		t.Errorf("exactly %d should pass (first registrants), got %d passed, %d rejected",
			messageCount, passCount, rejectCount)
	}
	expectedTotal := int64(numGoroutines * messageCount)
	if passCount+rejectCount != expectedTotal {
		t.Errorf("expected total operations=%d, got %d", expectedTotal, passCount+rejectCount)
	}
}

func TestConcurrent_CleanAndCheck(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize:    50 * time.Millisecond,
		CleanInterval: 25 * time.Millisecond,
	})
	d.Start()
	defer d.Stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, _ = d.CleanExpired()
			time.Sleep(500 * time.Microsecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 500; i++ {
			id := fmt.Sprintf("race-msg-%d", i)
			_, _ = d.CheckAndMark(id)
			time.Sleep(200 * time.Microsecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			_, _ = d.Count()
			_, _ = d.Contains(fmt.Sprintf("race-msg-%d", i%100))
			time.Sleep(300 * time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestMemoryLeak_AfterCleanup(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize:    20 * time.Millisecond,
		CleanInterval: 10 * time.Millisecond,
	})
	d.Start()
	defer d.Stop()

	const iterations = 100
	const batchSize = 50

	for i := 0; i < iterations; i++ {
		for j := 0; j < batchSize; j++ {
			id := fmt.Sprintf("leak-%d-%d", i, j)
			_, _ = d.CheckAndMark(id)
		}
		time.Sleep(40 * time.Millisecond)
	}

	count, _ := d.Count()
	if count > batchSize {
		t.Errorf("after %d iterations, Count=%d exceeds batch size %d — memory leak?",
			iterations, count, batchSize)
	}
	if len(d.idMap) > batchSize*2 {
		t.Errorf("after %d iterations, idMap size=%d exceeds 2x batch size %d — memory leak?",
			iterations, len(d.idMap), batchSize*2)
	}
}

func TestCheckAndMark_OrderPreservedInList(t *testing.T) {
	d := NewDeduplicator()

	for i := 0; i < 5; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("ordered-%d", i))
	}

	d.mu.Lock()
	if d.idList.Len() != 5 {
		d.mu.Unlock()
		t.Fatalf("expected list len 5, got %d", d.idList.Len())
	}

	i := 0
	for e := d.idList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*idEntry)
		expected := fmt.Sprintf("ordered-%d", i)
		if entry.id != expected {
			d.mu.Unlock()
			t.Errorf("position %d: expected id=%s, got %s", i, expected, entry.id)
			return
		}
		i++
	}
	d.mu.Unlock()
}

func TestCheckAndMark_TouchMovesToBack(t *testing.T) {
	d := NewDeduplicator()

	_, _ = d.CheckAndMark("a")
	_, _ = d.CheckAndMark("b")
	_, _ = d.CheckAndMark("c")

	_, _ = d.CheckAndMark("a")

	d.mu.Lock()
	defer d.mu.Unlock()

	if d.idList.Len() != 3 {
		t.Fatalf("expected list len 3, got %d", d.idList.Len())
	}

	front := d.idList.Front().Value.(*idEntry)
	if front.id != "b" {
		t.Errorf("after touching 'a', front should be 'b', got %s", front.id)
	}

	back := d.idList.Back().Value.(*idEntry)
	if back.id != "a" {
		t.Errorf("after touching 'a', back should be 'a', got %s", back.id)
	}
}

func TestCleanExpired_FIFOOrder(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 100 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("fifo-%d", i))
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(60 * time.Millisecond)

	cleaned, err := d.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned < 5 || cleaned > 7 {
		t.Errorf("expected ~6 cleaned (first 6 entries past window), got %d", cleaned)
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("fifo-%d", i)
		exists, _ := d.Contains(id)
		if exists {
			t.Errorf("early entry %s should be cleaned", id)
		}
	}
	for i := 8; i < 10; i++ {
		id := fmt.Sprintf("fifo-%d", i)
		exists, _ := d.Contains(id)
		if !exists {
			t.Errorf("late entry %s should remain", id)
		}
	}
}

func TestCount_AccuracyWithMixedExpiry(t *testing.T) {
	d, _ := NewDeduplicatorWithConfig(Config{
		WindowSize: 100 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("mix-%d", i))
	}

	time.Sleep(120 * time.Millisecond)

	for i := 10; i < 20; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("mix-%d", i))
	}

	count, err := d.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 10 {
		t.Errorf("expected Count=10 (only the second batch within window), got %d", count)
	}

	if len(d.idMap) != 20 {
		t.Errorf("idMap should have 20 entries (expired ones not yet cleaned), got %d", len(d.idMap))
	}

	cleaned, _ := d.CleanExpired()
	if cleaned != 10 {
		t.Errorf("CleanExpired should remove 10 expired entries, got %d", cleaned)
	}

	count, _ = d.Count()
	if count != 10 {
		t.Errorf("Count should still be 10 after cleanup, got %d", count)
	}
	if len(d.idMap) != 10 {
		t.Errorf("idMap should have 10 entries after cleanup, got %d", len(d.idMap))
	}
}
