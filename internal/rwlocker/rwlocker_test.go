package rwlocker

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	rw := New(nil)
	if rw == nil {
		t.Fatal("New returned nil")
	}
	if rw.Name() != "" {
		t.Errorf("expected empty name, got %q", rw.Name())
	}

	cfg := &Config{
		Name:                 "test-lock",
		ReadTimeout:          100 * time.Millisecond,
		WriteTimeout:         200 * time.Millisecond,
		EnableDeadlockDetect: true,
		EnableStats:          true,
	}
	rw2 := New(cfg)
	if rw2.Name() != "test-lock" {
		t.Errorf("expected name 'test-lock', got %q", rw2.Name())
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if !cfg.EnableDeadlockDetect {
		t.Error("EnableDeadlockDetect should be true by default")
	}
	if !cfg.EnableStats {
		t.Error("EnableStats should be true by default")
	}
	if cfg.ReadTimeout != 0 {
		t.Error("ReadTimeout should be 0 by default")
	}
	if cfg.WriteTimeout != 0 {
		t.Error("WriteTimeout should be 0 by default")
	}
}

func TestRLockAndRUnlock(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}
	if rw.ReaderCount() != 1 {
		t.Errorf("expected 1 reader, got %d", rw.ReaderCount())
	}

	if err := rw.RUnlock(); err != nil {
		t.Fatalf("RUnlock failed: %v", err)
	}
	if rw.ReaderCount() != 0 {
		t.Errorf("expected 0 readers, got %d", rw.ReaderCount())
	}
}

func TestLockAndUnlock(t *testing.T) {
	rw := New(nil)

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}
	if !rw.IsWriterActive() {
		t.Error("expected writer to be active")
	}

	if err := rw.Unlock(); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if rw.IsWriterActive() {
		t.Error("expected writer to be inactive")
	}
}

func TestMultipleReaders(t *testing.T) {
	rw := New(nil)

	for i := 0; i < 5; i++ {
		if err := rw.RLock(); err != nil {
			t.Fatalf("RLock %d failed: %v", i, err)
		}
	}
	if rw.ReaderCount() != 5 {
		t.Errorf("expected 5 readers, got %d", rw.ReaderCount())
	}

	for i := 0; i < 5; i++ {
		if err := rw.RUnlock(); err != nil {
			t.Fatalf("RUnlock %d failed: %v", i, err)
		}
	}
	if rw.ReaderCount() != 0 {
		t.Errorf("expected 0 readers, got %d", rw.ReaderCount())
	}
}

func TestConcurrentReaders(t *testing.T) {
	rw := New(nil)
	var wg sync.WaitGroup
	readerCount := 10
	var activeReaders int32

	for i := 0; i < readerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rw.RLock(); err != nil {
				t.Errorf("RLock failed: %v", err)
				return
			}
			atomic.AddInt32(&activeReaders, 1)
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&activeReaders, -1)
			if err := rw.RUnlock(); err != nil {
				t.Errorf("RUnlock failed: %v", err)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt32(&activeReaders) != 0 {
		t.Errorf("expected 0 active readers, got %d", activeReaders)
	}
	if rw.ReaderCount() != 0 {
		t.Errorf("expected 0 readers in lock, got %d", rw.ReaderCount())
	}
}

func TestWriterBlocksReaders(t *testing.T) {
	rw := New(nil)

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	readerDone := make(chan bool, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock failed: %v", err)
			return
		}
		readerDone <- true
		rw.RUnlock()
	}()

	select {
	case <-readerDone:
		t.Error("reader should have been blocked by writer")
	case <-time.After(100 * time.Millisecond):
	}

	rw.Unlock()

	select {
	case <-readerDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("reader should have acquired lock after writer released")
	}
}

func TestReadersBlockWriter(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	writerDone := make(chan bool, 1)
	go func() {
		if err := rw.Lock(); err != nil {
			t.Errorf("Lock failed: %v", err)
			return
		}
		writerDone <- true
		rw.Unlock()
	}()

	select {
	case <-writerDone:
		t.Error("writer should have been blocked by readers")
	case <-time.After(100 * time.Millisecond):
	}

	rw.RUnlock()

	select {
	case <-writerDone:
	case <-time.After(500 * time.Millisecond):
		t.Error("writer should have acquired lock after readers released")
	}
}

func TestTryRLock(t *testing.T) {
	rw := New(nil)

	ok, err := rw.TryRLock()
	if err != nil {
		t.Fatalf("TryRLock failed: %v", err)
	}
	if !ok {
		t.Error("TryRLock should have succeeded")
	}
	rw.RUnlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.Lock()
		time.Sleep(100 * time.Millisecond)
		rw.Unlock()
	}()
	time.Sleep(20 * time.Millisecond)

	ok, err = rw.TryRLock()
	if err != nil {
		t.Fatalf("TryRLock with writer active failed: %v", err)
	}
	if ok {
		t.Error("TryRLock should have failed when writer holds lock")
	}
	wg.Wait()
}

func TestTryLock(t *testing.T) {
	rw := New(nil)

	ok, err := rw.TryLock()
	if err != nil {
		t.Fatalf("TryLock failed: %v", err)
	}
	if !ok {
		t.Error("TryLock should have succeeded")
	}
	rw.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.RLock()
		time.Sleep(100 * time.Millisecond)
		rw.RUnlock()
	}()
	time.Sleep(20 * time.Millisecond)

	ok, err = rw.TryLock()
	if err != nil {
		t.Fatalf("TryLock with reader active failed: %v", err)
	}
	if ok {
		t.Error("TryLock should have failed when reader holds lock")
	}
	wg.Wait()
}

func TestReadLockTimeout(t *testing.T) {
	rw := New(&Config{
		ReadTimeout: 50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.Lock()
		time.Sleep(200 * time.Millisecond)
		rw.Unlock()
	}()
	time.Sleep(10 * time.Millisecond)

	err := rw.RLock()
	if err == nil {
		t.Error("RLock should have timed out")
		rw.RUnlock()
	}
	if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Error("expected TimeoutError type")
	} else if timeoutErr.LockType != "read" {
		t.Errorf("expected lock type 'read', got %q", timeoutErr.LockType)
	}

	wg.Wait()
}

func TestWriteLockTimeout(t *testing.T) {
	rw := New(&Config{
		WriteTimeout: 50 * time.Millisecond,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.RLock()
		time.Sleep(200 * time.Millisecond)
		rw.RUnlock()
	}()
	time.Sleep(10 * time.Millisecond)

	err := rw.Lock()
	if err == nil {
		t.Error("Lock should have timed out")
		rw.Unlock()
	}
	if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	var timeoutErr *TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Error("expected TimeoutError type")
	} else if timeoutErr.LockType != "write" {
		t.Errorf("expected lock type 'write', got %q", timeoutErr.LockType)
	}

	wg.Wait()
}

func TestReadLockReentrant(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("first RLock failed: %v", err)
	}

	err := rw.RLock()
	if err != nil {
		t.Errorf("reentrant RLock should be allowed, got: %v", err)
	} else {
		rw.RUnlock()
	}

	rw.RUnlock()
}

func TestDeadlockDetectionWriteAfterWrite(t *testing.T) {
	rw := New(nil)

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err := rw.Lock()
	if err == nil {
		t.Error("should have detected deadlock")
		rw.Unlock()
	}
	if !errors.Is(err, ErrDeadlockDetected) {
		t.Errorf("expected ErrDeadlockDetected, got %v", err)
	}

	rw.Unlock()
}

func TestDeadlockDetectionWriteAfterRead(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	err := rw.Lock()
	if err == nil {
		t.Error("should have detected deadlock")
		rw.Unlock()
	}
	if !errors.Is(err, ErrDeadlockDetected) {
		t.Errorf("expected ErrDeadlockDetected, got %v", err)
	}

	rw.RUnlock()
}

func TestDeadlockDetectionReadAfterWrite(t *testing.T) {
	rw := New(nil)

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock failed: %v", err)
	}

	err := rw.RLock()
	if err == nil {
		t.Error("should have detected deadlock")
		rw.RUnlock()
	}
	if !errors.Is(err, ErrDeadlockDetected) {
		t.Errorf("expected ErrDeadlockDetected, got %v", err)
	}

	rw.Unlock()
}

func TestDeadlockDetectionDisabled(t *testing.T) {
	rw := New(&Config{
		EnableDeadlockDetect: false,
		EnableStats:          true,
	})

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	err := rw.RLock()
	if err != nil {
		t.Errorf("reentrant RLock should be allowed when deadlock detection disabled, got %v", err)
	} else {
		rw.RUnlock()
	}

	rw.RUnlock()

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.DeadlockDetected != 0 {
		t.Errorf("expected 0 deadlocks detected when disabled, got %d", stats.DeadlockDetected)
	}
}

func TestUnlockWithoutLock(t *testing.T) {
	rw := New(nil)

	err := rw.Unlock()
	if err == nil {
		t.Error("Unlock without lock should fail")
	}
	if !errors.Is(err, ErrNotHeld) {
		t.Errorf("expected ErrNotHeld, got %v", err)
	}
}

func TestRUnlockWithoutRLock(t *testing.T) {
	rw := New(nil)

	err := rw.RUnlock()
	if err == nil {
		t.Error("RUnlock without RLock should fail")
	}
	if !errors.Is(err, ErrNotHeld) {
		t.Errorf("expected ErrNotHeld, got %v", err)
	}
}

func TestUpgradeNonBlockingSuccess(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err != nil {
		t.Fatalf("TryUpgrade should have succeeded: %v", err)
	}

	if !rw.IsWriterActive() {
		t.Error("expected writer to be active after upgrade")
	}
	if rw.ReaderCount() != 0 {
		t.Errorf("expected 0 readers after upgrade, got %d", rw.ReaderCount())
	}

	rw.Unlock()
}

func TestUpgradeNonBlockingSuccessWithMultipleReadLocks(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("first RLock failed: %v", err)
	}
	if err := rw.RLock(); err != nil {
		t.Fatalf("second RLock failed: %v", err)
	}

	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err != nil {
		t.Fatalf("TryUpgrade with multiple reentrant read locks should have succeeded: %v", err)
	}

	if !rw.IsWriterActive() {
		t.Error("expected writer to be active after upgrade")
	}

	rw.Unlock()
}

func TestUpgradeNonBlockingFailure(t *testing.T) {
	rw := New(nil)

	otherReaderReady := make(chan bool, 1)
	otherReaderDone := make(chan bool, 1)
	go func() {
		rw.RLock()
		otherReaderReady <- true
		<-otherReaderDone
		rw.RUnlock()
	}()

	<-otherReaderReady

	if err := rw.RLock(); err != nil {
		t.Fatalf("my RLock failed: %v", err)
	}

	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err == nil {
		t.Error("TryUpgrade should have failed with other readers")
		rw.Unlock()
		rw.RUnlock()
		otherReaderDone <- true
		return
	}
	if !errors.Is(err, ErrUpgradeFailed) {
		t.Errorf("expected ErrUpgradeFailed, got %v", err)
	}

	var upgradeErr *UpgradeError
	if !errors.As(err, &upgradeErr) {
		t.Error("expected UpgradeError type")
	} else if upgradeErr.ReaderCount < 1 {
		t.Errorf("expected at least 1 other reader, got %d", upgradeErr.ReaderCount)
	}

	rw.RUnlock()
	otherReaderDone <- true
}

func TestUpgradeBlockingSuccess(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	otherReaderReady := make(chan bool, 1)
	go func() {
		rw.RLock()
		otherReaderReady <- true
		time.Sleep(100 * time.Millisecond)
		rw.RUnlock()
	}()

	<-otherReaderReady

	err := rw.TryUpgrade(UpgradeBlocking, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("TryUpgrade should have succeeded: %v", err)
	}

	if !rw.IsWriterActive() {
		t.Error("expected writer to be active after upgrade")
	}

	rw.Unlock()
}

func TestUpgradeBlockingTimeout(t *testing.T) {
	rw := New(nil)

	otherReaderReady := make(chan bool, 1)
	otherReaderDone := make(chan bool, 1)
	go func() {
		rw.RLock()
		otherReaderReady <- true
		<-otherReaderDone
		rw.RUnlock()
	}()

	<-otherReaderReady

	if err := rw.RLock(); err != nil {
		t.Fatalf("my RLock failed: %v", err)
	}

	err := rw.TryUpgrade(UpgradeBlocking, 50*time.Millisecond)
	if err == nil {
		t.Error("TryUpgrade should have timed out")
		rw.Unlock()
		otherReaderDone <- true
		return
	}

	rw.RUnlock()
	otherReaderDone <- true
}

func TestUpgradeWithoutReadLock(t *testing.T) {
	rw := New(nil)

	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err == nil {
		t.Error("TryUpgrade should have failed without read lock")
	}
}

func TestUpgradeWithWriteLock(t *testing.T) {
	rw := New(nil)

	rw.Lock()
	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err == nil {
		t.Error("TryUpgrade should have failed when holding write lock")
		rw.Unlock()
		return
	}
	rw.Unlock()
}

func TestStatsBasic(t *testing.T) {
	rw := New(nil)

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}

	rw.RLock()
	rw.RUnlock()

	stats = rw.GetStats()
	if stats.ReadRequests != 1 {
		t.Errorf("expected 1 read request, got %d", stats.ReadRequests)
	}
	if stats.ReadSuccess != 1 {
		t.Errorf("expected 1 read success, got %d", stats.ReadSuccess)
	}

	rw.Lock()
	rw.Unlock()

	stats = rw.GetStats()
	if stats.WriteRequests != 1 {
		t.Errorf("expected 1 write request, got %d", stats.WriteRequests)
	}
	if stats.WriteSuccess != 1 {
		t.Errorf("expected 1 write success, got %d", stats.WriteSuccess)
	}
}

func TestStatsTimeout(t *testing.T) {
	rw := New(&Config{
		WriteTimeout: 30 * time.Millisecond,
		EnableStats:  true,
	})

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rw.RLock()
		time.Sleep(200 * time.Millisecond)
		rw.RUnlock()
	}()
	time.Sleep(10 * time.Millisecond)

	err := rw.Lock()
	if err == nil {
		t.Fatal("Lock should have timed out")
	}
	wg.Wait()

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.WriteRequests != 1 {
		t.Errorf("expected 1 write request, got %d", stats.WriteRequests)
	}
	if stats.TimeoutCount != 1 {
		t.Errorf("expected 1 timeout, got %d", stats.TimeoutCount)
	}
}

func TestStatsDeadlockDetection(t *testing.T) {
	rw := New(nil)

	rw.Lock()
	err := rw.Lock()
	if err == nil {
		t.Fatal("Lock should have detected deadlock")
		rw.Unlock()
	}
	rw.Unlock()

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.DeadlockDetected != 1 {
		t.Errorf("expected 1 deadlock detected, got %d", stats.DeadlockDetected)
	}
}

func TestStatsUpgrade(t *testing.T) {
	rw := New(nil)

	rw.RLock()
	if err := rw.TryUpgrade(UpgradeNonBlocking, 0); err != nil {
		t.Fatalf("TryUpgrade failed: %v", err)
	}
	rw.Unlock()

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.UpgradeRequests != 1 {
		t.Errorf("expected 1 upgrade request, got %d", stats.UpgradeRequests)
	}
	if stats.UpgradeSuccess != 1 {
		t.Errorf("expected 1 upgrade success, got %d", stats.UpgradeSuccess)
	}
}

func TestResetStats(t *testing.T) {
	rw := New(nil)

	rw.RLock()
	rw.RUnlock()
	rw.Lock()
	rw.Unlock()

	stats := rw.GetStats()
	if stats == nil || stats.ReadRequests == 0 || stats.WriteRequests == 0 {
		t.Fatal("stats should not be zero before reset")
	}

	rw.ResetStats()

	stats = rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil after reset")
	}
	if stats.ReadRequests != 0 {
		t.Errorf("expected 0 read requests after reset, got %d", stats.ReadRequests)
	}
	if stats.WriteRequests != 0 {
		t.Errorf("expected 0 write requests after reset, got %d", stats.WriteRequests)
	}
}

func TestStatsDisabled(t *testing.T) {
	rw := New(&Config{
		EnableStats: false,
	})

	rw.RLock()
	rw.RUnlock()

	stats := rw.GetStats()
	if stats != nil {
		t.Error("GetStats should return nil when stats disabled")
	}
}

func TestHoldDurationWarning(t *testing.T) {
	var warned atomic.Bool
	var receivedWarning *HoldDurationWarning

	rw := New(&Config{
		HoldDurationWarn: 30 * time.Millisecond,
		OnHoldDurationWarn: func(w *HoldDurationWarning) {
			warned.Store(true)
			receivedWarning = w
		},
		EnableDeadlockDetect: true,
	})

	rw.Lock()
	time.Sleep(80 * time.Millisecond)
	rw.Unlock()

	if !warned.Load() {
		t.Error("expected hold duration warning")
	}
	if receivedWarning == nil {
		t.Fatal("received warning is nil")
	}
	if receivedWarning.LockType != "write" {
		t.Errorf("expected lock type 'write', got %q", receivedWarning.LockType)
	}
	if receivedWarning.HoldDuration <= receivedWarning.Threshold {
		t.Error("hold duration should exceed threshold")
	}
}

func TestErrorsUnwrap(t *testing.T) {
	timeoutErr := &TimeoutError{LockType: "read", Timeout: time.Second}
	if !errors.Is(timeoutErr, ErrLockTimeout) {
		t.Error("TimeoutError should unwrap to ErrLockTimeout")
	}

	deadlockErr := &DeadlockError{LockType: "write", GoroutineID: 1, AlreadyHeld: "read"}
	if !errors.Is(deadlockErr, ErrDeadlockDetected) {
		t.Error("DeadlockError should unwrap to ErrDeadlockDetected")
	}

	upgradeErr := &UpgradeError{Reason: "test", ReaderCount: 2, Blocking: false}
	if !errors.Is(upgradeErr, ErrUpgradeFailed) {
		t.Error("UpgradeError should unwrap to ErrUpgradeFailed")
	}

	holdWarn := &HoldDurationWarning{
		LockType:     "write",
		HoldDuration: 2 * time.Second,
		Threshold:    time.Second,
		GoroutineID:  1,
	}
	if !errors.Is(holdWarn, ErrHoldDurationExceeded) {
		t.Error("HoldDurationWarning should unwrap to ErrHoldDurationExceeded")
	}
}

func TestErrorMessages(t *testing.T) {
	timeoutErr := &TimeoutError{LockType: "write", Timeout: 100 * time.Millisecond}
	if timeoutErr.Error() == "" {
		t.Error("TimeoutError.Error() should not be empty")
	}

	deadlockErr := &DeadlockError{LockType: "write", GoroutineID: 123, AlreadyHeld: "read"}
	if deadlockErr.Error() == "" {
		t.Error("DeadlockError.Error() should not be empty")
	}

	upgradeErr := &UpgradeError{Reason: "test", ReaderCount: 5, Blocking: true}
	if upgradeErr.Error() == "" {
		t.Error("UpgradeError.Error() should not be empty")
	}

	upgradeErrNonBlock := &UpgradeError{Reason: "test", ReaderCount: 3, Blocking: false}
	if upgradeErrNonBlock.Error() == "" {
		t.Error("UpgradeError(non-blocking).Error() should not be empty")
	}

	holdWarn := &HoldDurationWarning{
		LockType:     "read",
		HoldDuration: 5 * time.Second,
		Threshold:    time.Second,
		GoroutineID:  42,
	}
	if holdWarn.Error() == "" {
		t.Error("HoldDurationWarning.Error() should not be empty")
	}
}

func TestStatsClone(t *testing.T) {
	original := &Stats{
		ReadRequests:     10,
		ReadSuccess:      8,
		ReadWaitTotal:    100 * time.Millisecond,
		ReadWaitMax:      50 * time.Millisecond,
		WriteRequests:    5,
		WriteSuccess:     4,
		WriteWaitTotal:   200 * time.Millisecond,
		WriteWaitMax:     100 * time.Millisecond,
		UpgradeRequests:  2,
		UpgradeSuccess:   1,
		UpgradeWaitTotal: 50 * time.Millisecond,
		UpgradeWaitMax:   30 * time.Millisecond,
		DeadlockDetected: 1,
		TimeoutCount:     2,
	}

	cloned := original.clone()
	if cloned == nil {
		t.Fatal("clone returned nil")
	}

	if cloned.ReadRequests != original.ReadRequests {
		t.Errorf("ReadRequests mismatch")
	}
	if cloned.WriteSuccess != original.WriteSuccess {
		t.Errorf("WriteSuccess mismatch")
	}
	if cloned.UpgradeWaitMax != original.UpgradeWaitMax {
		t.Errorf("UpgradeWaitMax mismatch")
	}

	cloned.ReadRequests = 999
	if original.ReadRequests == 999 {
		t.Error("clone should be independent of original")
	}

	var nilStats *Stats
	if nilStats.clone() != nil {
		t.Error("nil stats clone should return nil")
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	rw := New(nil)
	var wg sync.WaitGroup
	iterations := 100
	var counter int64

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if err := rw.RLock(); err != nil {
					t.Errorf("RLock failed: %v", err)
					return
				}
				_ = atomic.LoadInt64(&counter)
				rw.RUnlock()
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if err := rw.Lock(); err != nil {
					t.Errorf("Lock failed: %v", err)
					return
				}
				atomic.AddInt64(&counter, 1)
				rw.Unlock()
			}
		}()
	}

	wg.Wait()

	if counter != int64(5*iterations) {
		t.Errorf("expected counter to be %d, got %d", 5*iterations, counter)
	}
}

func TestRLockBlockedDuringUpgrade(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	upgradeStarted := make(chan bool, 1)
	upgradeDone := make(chan error, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock in upgrade goroutine failed: %v", err)
			return
		}
		upgradeStarted <- true
		err := rw.TryUpgrade(UpgradeBlocking, 3*time.Second)
		upgradeDone <- err
		if err == nil {
			rw.Unlock()
		}
	}()

	<-upgradeStarted
	time.Sleep(50 * time.Millisecond)

	readerAcquired := make(chan bool, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock after upgrade: %v", err)
			return
		}
		readerAcquired <- true
		rw.RUnlock()
	}()

	select {
	case <-readerAcquired:
		t.Error("RLock should be blocked while writerWaiting is true during upgrade")
	case <-time.After(100 * time.Millisecond):
	}

	rw.RUnlock()

	select {
	case err := <-upgradeDone:
		if err != nil {
			t.Fatalf("TryUpgrade should have succeeded: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryUpgrade should have completed after other readers released")
	}

	select {
	case <-readerAcquired:
	case <-time.After(500 * time.Millisecond):
		t.Error("RLock should have acquired lock after upgrade completed")
	}
}

func TestTryRLockRejectedDuringUpgrade(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	upgradeStarted := make(chan bool, 1)
	upgradeDone := make(chan error, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock in upgrade goroutine failed: %v", err)
			return
		}
		upgradeStarted <- true
		err := rw.TryUpgrade(UpgradeBlocking, 3*time.Second)
		upgradeDone <- err
		if err == nil {
			rw.Unlock()
		}
	}()

	<-upgradeStarted
	time.Sleep(50 * time.Millisecond)

	ok, err := rw.TryRLock()
	if err != nil {
		t.Errorf("TryRLock should not return error during upgrade, got: %v", err)
	}
	if ok {
		t.Error("TryRLock should return false when writerWaiting is true")
		rw.RUnlock()
	}

	rw.RUnlock()

	select {
	case err := <-upgradeDone:
		if err != nil {
			t.Fatalf("TryUpgrade should have succeeded: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryUpgrade should have completed")
	}
}

func TestUpgradeNoStarvationUnderConcurrentReads(t *testing.T) {
	rw := New(nil)

	if err := rw.RLock(); err != nil {
		t.Fatalf("initial RLock failed: %v", err)
	}

	upgradeStarted := make(chan bool, 1)
	upgradeDone := make(chan error, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock in upgrade goroutine failed: %v", err)
			return
		}
		upgradeStarted <- true
		err := rw.TryUpgrade(UpgradeBlocking, 3*time.Second)
		upgradeDone <- err
		if err == nil {
			rw.Unlock()
		}
	}()

	<-upgradeStarted
	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 10; i++ {
		ok, _ := rw.TryRLock()
		if ok {
			t.Errorf("TryRLock attempt %d should be rejected during upgrade", i)
			rw.RUnlock()
		}
	}

	rw.RUnlock()

	select {
	case err := <-upgradeDone:
		if err != nil {
			t.Fatalf("TryUpgrade should have succeeded: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("TryUpgrade timed out - writer starvation detected")
	}
}

func TestUpgradeRequestNotCountedForInvalidAttempt(t *testing.T) {
	rw := New(nil)

	err := rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err == nil {
		t.Error("TryUpgrade without read lock should fail")
	}

	stats := rw.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.UpgradeRequests != 0 {
		t.Errorf("invalid upgrade request should not be counted, got UpgradeRequests=%d", stats.UpgradeRequests)
	}

	rw.Lock()
	err = rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err == nil {
		t.Error("TryUpgrade with write lock should fail")
		rw.Unlock()
		return
	}
	rw.Unlock()

	stats = rw.GetStats()
	if stats.UpgradeRequests != 0 {
		t.Errorf("invalid upgrade request with write lock should not be counted, got UpgradeRequests=%d", stats.UpgradeRequests)
	}

	rw.RLock()
	err = rw.TryUpgrade(UpgradeNonBlocking, 0)
	if err != nil {
		t.Fatalf("valid upgrade should succeed: %v", err)
	}
	rw.Unlock()

	stats = rw.GetStats()
	if stats.UpgradeRequests != 1 {
		t.Errorf("valid upgrade request should be counted, got UpgradeRequests=%d", stats.UpgradeRequests)
	}
	if stats.UpgradeSuccess != 1 {
		t.Errorf("upgrade success should be counted, got UpgradeSuccess=%d", stats.UpgradeSuccess)
	}
}

func TestRLockTimeoutDuringWriterWaiting(t *testing.T) {
	rw := New(&Config{
		ReadTimeout: 50 * time.Millisecond,
	})

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock failed: %v", err)
	}

	upgradeStarted := make(chan bool, 1)
	upgradeDone := make(chan error, 1)
	go func() {
		if err := rw.RLock(); err != nil {
			t.Errorf("RLock in upgrade goroutine failed: %v", err)
			return
		}
		upgradeStarted <- true
		err := rw.TryUpgrade(UpgradeBlocking, 500*time.Millisecond)
		upgradeDone <- err
		if err == nil {
			rw.Unlock()
		}
	}()

	<-upgradeStarted
	time.Sleep(20 * time.Millisecond)

	err := rw.RLock()
	if err == nil {
		t.Error("RLock should have timed out during writerWaiting")
		rw.RUnlock()
	} else if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	rw.RUnlock()

	select {
	case err := <-upgradeDone:
		if err != nil {
			t.Errorf("TryUpgrade should have succeeded: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("TryUpgrade should have completed")
	}
}

func TestRLockTimeoutCleanupGoroutineNoLeak(t *testing.T) {
	rw := New(&Config{
		ReadTimeout: 50 * time.Millisecond,
	})

	rw.Lock()

	err := rw.RLock()
	if err == nil {
		t.Error("RLock should have timed out")
		rw.RUnlock()
	}
	if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	rw.Unlock()

	time.Sleep(80 * time.Millisecond)

	if rw.ReaderCount() != 0 {
		t.Errorf("readerCount should be 0 after RLock timeout cleanup, got %d", rw.ReaderCount())
	}

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock should succeed after previous timeout: %v", err)
	}
	if rw.ReaderCount() != 1 {
		t.Errorf("expected 1 reader, got %d", rw.ReaderCount())
	}
	rw.RUnlock()

	if rw.ReaderCount() != 0 {
		t.Errorf("readerCount should be 0 after RUnlock, got %d", rw.ReaderCount())
	}
}

func TestLockTimeoutCleanupGoroutineNoLeak(t *testing.T) {
	rw := New(&Config{
		WriteTimeout: 50 * time.Millisecond,
	})

	rw.RLock()

	err := rw.Lock()
	if err == nil {
		t.Error("Lock should have timed out")
		rw.Unlock()
	}
	if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	rw.RUnlock()

	time.Sleep(80 * time.Millisecond)

	if rw.IsWriterActive() {
		t.Error("writerActive should be false after Lock timeout cleanup and reader released")
	}

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock should succeed after previous timeout: %v", err)
	}
	if !rw.IsWriterActive() {
		t.Error("writerActive should be true after Lock succeeds")
	}
	rw.Unlock()

	if rw.IsWriterActive() {
		t.Error("writerActive should be false after Unlock")
	}
}

func TestRLockSuccessPathNoGoroutineLeak(t *testing.T) {
	rw := New(&Config{
		ReadTimeout: 500 * time.Millisecond,
	})

	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		if err := rw.RLock(); err != nil {
			t.Fatalf("RLock %d failed: %v", i, err)
		}
		rw.RUnlock()
	}

	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Errorf("potential goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestLockSuccessPathNoGoroutineLeak(t *testing.T) {
	rw := New(&Config{
		WriteTimeout: 500 * time.Millisecond,
	})

	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ {
		if err := rw.Lock(); err != nil {
			t.Fatalf("Lock %d failed: %v", i, err)
		}
		rw.Unlock()
	}

	time.Sleep(50 * time.Millisecond)

	after := runtime.NumGoroutine()
	if after > before+5 {
		t.Errorf("potential goroutine leak: before=%d, after=%d", before, after)
	}
}

func TestRLockTimeoutReaderCountCleanup(t *testing.T) {
	rw := New(&Config{
		ReadTimeout: 30 * time.Millisecond,
	})

	writerReleased := make(chan struct{})
	go func() {
		rw.Lock()
		time.Sleep(200 * time.Millisecond)
		rw.Unlock()
		close(writerReleased)
	}()
	time.Sleep(10 * time.Millisecond)

	err := rw.RLock()
	if err == nil {
		t.Error("RLock should have timed out while writer holds lock")
		rw.RUnlock()
	} else if !errors.Is(err, ErrLockTimeout) {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}

	<-writerReleased

	time.Sleep(100 * time.Millisecond)

	count := rw.ReaderCount()
	if count != 0 {
		t.Errorf("readerCount should be 0 after RLock timeout cleanup, got %d", count)
	}

	if err := rw.RLock(); err != nil {
		t.Fatalf("RLock should succeed after cleanup: %v", err)
	}
	if rw.ReaderCount() != 1 {
		t.Errorf("expected 1 reader after new RLock, got %d", rw.ReaderCount())
	}
	rw.RUnlock()
}

func TestLockTimeoutSubsequentAcquire(t *testing.T) {
	rw := New(&Config{
		WriteTimeout: 30 * time.Millisecond,
	})

	holderDone := make(chan struct{})
	go func() {
		rw.Lock()
		time.Sleep(200 * time.Millisecond)
		rw.Unlock()
		close(holderDone)
	}()
	time.Sleep(10 * time.Millisecond)

	err := rw.Lock()
	if err == nil {
		t.Error("Lock should have timed out")
		rw.Unlock()
	}

	<-holderDone

	if err := rw.Lock(); err != nil {
		t.Fatalf("Lock should succeed after holder released: %v", err)
	}
	rw.Unlock()
}

func TestNoTimeoutConfiguration(t *testing.T) {
	rw := New(&Config{
		ReadTimeout:  0,
		WriteTimeout: 0,
	})

	done := make(chan bool, 1)
	go func() {
		rw.Lock()
		time.Sleep(50 * time.Millisecond)
		rw.Unlock()
		done <- true
	}()

	time.Sleep(10 * time.Millisecond)

	if err := rw.RLock(); err != nil {
		t.Errorf("RLock without timeout should wait and succeed: %v", err)
		return
	}
	rw.RUnlock()

	<-done
}
