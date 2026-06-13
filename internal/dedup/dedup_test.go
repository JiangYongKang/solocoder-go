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
	if d.Count() != 0 {
		t.Errorf("expected Count=0, got %d", d.Count())
	}
}

func TestNewDeduplicatorWithConfig_Defaults(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{})
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

func TestNewDeduplicatorWithConfig_CleanIntervalFromWindow(t *testing.T) {
	window := 10 * time.Second
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: window,
	})
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
	if d.Count() != 1 {
		t.Errorf("expected Count=1, got %d", d.Count())
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
	if d.Count() != 1 {
		t.Errorf("expected Count=1 (no duplicate added), got %d", d.Count())
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

	if d.Count() != n {
		t.Errorf("expected Count=%d, got %d", n, d.Count())
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

	if d.Count() != n {
		t.Errorf("expected Count unchanged=%d, got %d", n, d.Count())
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
	if d.Count() != 0 {
		t.Errorf("expected Count=0, got %d", d.Count())
	}
}

func TestContains(t *testing.T) {
	d := NewDeduplicator()

	if d.Contains("not-exist") {
		t.Error("Contains should return false for non-existent id")
	}

	_, _ = d.CheckAndMark("exist-1")
	if !d.Contains("exist-1") {
		t.Error("Contains should return true for existing id")
	}

	if d.Contains("") {
		t.Error("Contains should return false for empty id")
	}
}

func TestContains_Expired(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: 50 * time.Millisecond,
	})

	_, _ = d.CheckAndMark("expired-msg")
	if !d.Contains("expired-msg") {
		t.Fatal("message should be contained initially")
	}

	time.Sleep(100 * time.Millisecond)

	if d.Contains("expired-msg") {
		t.Error("expired message should not be contained")
	}
}

func TestClear(t *testing.T) {
	d := NewDeduplicator()

	for i := 0; i < 50; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("clear-%d", i))
	}
	if d.Count() != 50 {
		t.Fatalf("expected 50 entries, got %d", d.Count())
	}

	d.Clear()
	if d.Count() != 0 {
		t.Errorf("expected Count=0 after Clear, got %d", d.Count())
	}

	for i := 0; i < 50; i++ {
		ok, _ := d.CheckAndMark(fmt.Sprintf("clear-%d", i))
		if !ok {
			t.Errorf("cleared message %d should pass again", i)
		}
	}
}

func TestCleanExpired_NoExpired(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: 1 * time.Hour,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("fresh-%d", i))
	}

	cleaned := d.CleanExpired()
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned, got %d", cleaned)
	}
	if d.Count() != 10 {
		t.Errorf("expected Count=10, got %d", d.Count())
	}
}

func TestCleanExpired_AllExpired(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: 30 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("exp-all-%d", i))
	}

	time.Sleep(80 * time.Millisecond)

	cleaned := d.CleanExpired()
	if cleaned != 10 {
		t.Errorf("expected 10 cleaned, got %d", cleaned)
	}
	if d.Count() != 0 {
		t.Errorf("expected Count=0 after cleaning all, got %d", d.Count())
	}
}

func TestCleanExpired_PartialExpired(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
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

	cleaned := d.CleanExpired()
	if cleaned != 5 {
		t.Errorf("expected 5 cleaned (early ones), got %d", cleaned)
	}
	if d.Count() != 5 {
		t.Errorf("expected Count=5 (late ones remain), got %d", d.Count())
	}

	for i := 0; i < 5; i++ {
		if !d.Contains(fmt.Sprintf("late-%d", i)) {
			t.Errorf("late-%d should still be contained", i)
		}
		if d.Contains(fmt.Sprintf("early-%d", i)) {
			t.Errorf("early-%d should be cleaned", i)
		}
	}
}

func TestCheckAndMark_ExpiredThenReaccept(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
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
	if d.Count() != 1 {
		t.Errorf("expected Count=1 (fresh entry), got %d", d.Count())
	}
}

func TestCheckAndMark_TouchOnAccess(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: 80 * time.Millisecond,
	})

	_, _ = d.CheckAndMark("touch-msg")

	time.Sleep(50 * time.Millisecond)

	_, _ = d.CheckAndMark("touch-msg")

	time.Sleep(50 * time.Millisecond)

	if !d.Contains("touch-msg") {
		t.Error("touched message should still be within window after second access refreshed it")
	}

	time.Sleep(60 * time.Millisecond)

	if d.Contains("touch-msg") {
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

func TestStartStop_BackgroundCleanup(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
		WindowSize:    30 * time.Millisecond,
		CleanInterval: 20 * time.Millisecond,
	})

	d.Start()
	defer d.Stop()

	for i := 0; i < 20; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("bg-%d", i))
	}
	if d.Count() != 20 {
		t.Fatalf("expected 20 entries, got %d", d.Count())
	}

	time.Sleep(150 * time.Millisecond)

	count := d.Count()
	if count != 0 {
		t.Errorf("expected Count=0 after background cleanup, got %d", count)
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
					t.Errorf("goroutine %d iteration %d error: %v", gid, i, err)
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
	if d.Count() != int(expected) {
		t.Errorf("expected Count=%d, got %d", expected, d.Count())
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
	d := NewDeduplicatorWithConfig(Config{
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
			d.CleanExpired()
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
			_ = d.Count()
			_ = d.Contains(fmt.Sprintf("race-msg-%d", i%100))
			time.Sleep(300 * time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestMemoryLeak_AfterCleanup(t *testing.T) {
	d := NewDeduplicatorWithConfig(Config{
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

	count := d.Count()
	if count > batchSize {
		t.Errorf("after %d iterations, Count=%d exceeds batch size %d — memory leak?",
			iterations, count, batchSize)
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
	d := NewDeduplicatorWithConfig(Config{
		WindowSize: 100 * time.Millisecond,
	})

	for i := 0; i < 10; i++ {
		_, _ = d.CheckAndMark(fmt.Sprintf("fifo-%d", i))
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(60 * time.Millisecond)

	cleaned := d.CleanExpired()
	if cleaned < 5 || cleaned > 7 {
		t.Errorf("expected ~6 cleaned (first 6 entries past window), got %d", cleaned)
	}

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("fifo-%d", i)
		if d.Contains(id) {
			t.Errorf("early entry %s should be cleaned", id)
		}
	}
	for i := 8; i < 10; i++ {
		id := fmt.Sprintf("fifo-%d", i)
		if !d.Contains(id) {
			t.Errorf("late entry %s should remain", id)
		}
	}
}
