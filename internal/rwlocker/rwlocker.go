package rwlocker

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"
)

var (
	goroutineLockRegistryMu sync.Mutex
	goroutineLocks          = make(map[int64]*goroutineLockInfo)
)

func getGoroutineID() int64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseInt(string(b), 10, 64)
	return n
}

func getGoroutineLockInfo(gid int64) *goroutineLockInfo {
	goroutineLockRegistryMu.Lock()
	defer goroutineLockRegistryMu.Unlock()
	info, ok := goroutineLocks[gid]
	if !ok {
		info = &goroutineLockInfo{
			holders: make(map[*RWLocker]*lockHolder),
		}
		goroutineLocks[gid] = info
	}
	return info
}

func removeGoroutineLockInfo(gid int64, locker *RWLocker) {
	goroutineLockRegistryMu.Lock()
	defer goroutineLockRegistryMu.Unlock()
	info, ok := goroutineLocks[gid]
	if !ok {
		return
	}
	delete(info.holders, locker)
	if len(info.holders) == 0 {
		delete(goroutineLocks, gid)
	}
}

type RWLocker struct {
	name                 string
	mu                   sync.RWMutex
	readTimeout          time.Duration
	writeTimeout         time.Duration
	enableDeadlockDetect bool
	enableStats          bool
	holdDurationWarn     time.Duration
	onHoldDurationWarn   func(warning *HoldDurationWarning)

	statsMu sync.Mutex
	stats   Stats

	readerCount   int
	writerWaiting bool
	writerActive  bool
	upgradeMu     sync.Mutex
	upgradeCond   *sync.Cond
}

func New(cfg *Config) *RWLocker {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	rw := &RWLocker{
		name:                 cfg.Name,
		readTimeout:          cfg.ReadTimeout,
		writeTimeout:         cfg.WriteTimeout,
		enableDeadlockDetect: cfg.EnableDeadlockDetect,
		enableStats:          cfg.EnableStats,
		holdDurationWarn:     cfg.HoldDurationWarn,
		onHoldDurationWarn:   cfg.OnHoldDurationWarn,
	}
	rw.upgradeCond = sync.NewCond(&rw.upgradeMu)
	return rw
}

func (rw *RWLocker) Name() string {
	return rw.name
}

func (rw *RWLocker) GetStats() *Stats {
	if !rw.enableStats {
		return nil
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	return rw.stats.clone()
}

func (rw *RWLocker) ResetStats() {
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	rw.stats = Stats{}
}

func (rw *RWLocker) incRequest(lockType LockType) {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	switch lockType {
	case LockTypeRead:
		rw.stats.ReadRequests++
	case LockTypeWrite:
		rw.stats.WriteRequests++
	}
}

func (rw *RWLocker) incSuccess(lockType LockType, waitDuration time.Duration) {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	switch lockType {
	case LockTypeRead:
		rw.stats.ReadSuccess++
		rw.stats.ReadWaitTotal += waitDuration
		if waitDuration > rw.stats.ReadWaitMax {
			rw.stats.ReadWaitMax = waitDuration
		}
	case LockTypeWrite:
		rw.stats.WriteSuccess++
		rw.stats.WriteWaitTotal += waitDuration
		if waitDuration > rw.stats.WriteWaitMax {
			rw.stats.WriteWaitMax = waitDuration
		}
	}
}

func (rw *RWLocker) incUpgradeRequest() {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	rw.stats.UpgradeRequests++
}

func (rw *RWLocker) incUpgradeSuccess(waitDuration time.Duration) {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	rw.stats.UpgradeSuccess++
	rw.stats.UpgradeWaitTotal += waitDuration
	if waitDuration > rw.stats.UpgradeWaitMax {
		rw.stats.UpgradeWaitMax = waitDuration
	}
}

func (rw *RWLocker) incDeadlockDetected() {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	rw.stats.DeadlockDetected++
}

func (rw *RWLocker) incTimeout() {
	if !rw.enableStats {
		return
	}
	rw.statsMu.Lock()
	defer rw.statsMu.Unlock()
	rw.stats.TimeoutCount++
}

func (rw *RWLocker) checkDeadlock(lockType LockType) error {
	if !rw.enableDeadlockDetect {
		return nil
	}
	gid := getGoroutineID()
	info := getGoroutineLockInfo(gid)
	holder, exists := info.holders[rw]
	if !exists {
		return nil
	}
	if holder.lockType == LockTypeRead && lockType == LockTypeRead {
		return nil
	}
	rw.incDeadlockDetected()
	return &DeadlockError{
		LockType:    string(lockType),
		GoroutineID: gid,
		AlreadyHeld: string(holder.lockType),
	}
}

func (rw *RWLocker) registerHolder(lockType LockType) {
	if !rw.enableDeadlockDetect {
		return
	}
	gid := getGoroutineID()
	info := getGoroutineLockInfo(gid)
	holder, exists := info.holders[rw]
	if !exists {
		info.holders[rw] = &lockHolder{
			goroutineID: gid,
			lockType:    lockType,
			acquireTime: time.Now(),
			count:       1,
		}
	} else {
		holder.count++
	}
}

func (rw *RWLocker) unregisterHolder() {
	if !rw.enableDeadlockDetect {
		return
	}
	gid := getGoroutineID()
	info := getGoroutineLockInfo(gid)
	holder, exists := info.holders[rw]
	if !exists {
		return
	}
	if holder.count > 1 {
		holder.count--
		return
	}
	if rw.holdDurationWarn > 0 && rw.onHoldDurationWarn != nil {
		holdDuration := time.Since(holder.acquireTime)
		if holdDuration > rw.holdDurationWarn {
			warning := &HoldDurationWarning{
				LockType:     string(holder.lockType),
				HoldDuration: holdDuration,
				Threshold:    rw.holdDurationWarn,
				GoroutineID:  holder.goroutineID,
			}
			rw.onHoldDurationWarn(warning)
		}
	}
	removeGoroutineLockInfo(gid, rw)
}

func (rw *RWLocker) waitForWriterWaiting() {
	rw.upgradeMu.Lock()
	for rw.writerWaiting {
		rw.upgradeCond.Wait()
	}
	rw.readerCount++
	rw.upgradeMu.Unlock()
}

func (rw *RWLocker) RLock() error {
	if err := rw.checkDeadlock(LockTypeRead); err != nil {
		return err
	}
	rw.incRequest(LockTypeRead)
	start := time.Now()

	if rw.readTimeout <= 0 {
		rw.waitForWriterWaiting()
		rw.mu.RLock()
		rw.registerHolder(LockTypeRead)
		rw.incSuccess(LockTypeRead, time.Since(start))
		return nil
	}

	done := make(chan struct{})
	go func() {
		rw.upgradeMu.Lock()
		for rw.writerWaiting {
			rw.upgradeCond.Wait()
		}
		rw.readerCount++
		rw.upgradeMu.Unlock()
		rw.mu.RLock()
		close(done)
	}()

	timer := time.NewTimer(rw.readTimeout)
	defer timer.Stop()

	select {
	case <-done:
		rw.registerHolder(LockTypeRead)
		rw.incSuccess(LockTypeRead, time.Since(start))
		return nil
	case <-timer.C:
		rw.incTimeout()
		go func() {
			<-done
			rw.upgradeMu.Lock()
			rw.readerCount--
			if rw.writerWaiting && rw.readerCount == 0 {
				rw.upgradeCond.Broadcast()
			}
			rw.upgradeMu.Unlock()
			rw.mu.RUnlock()
		}()
		return &TimeoutError{
			LockType: string(LockTypeRead),
			Timeout:  rw.readTimeout,
		}
	}
}

func (rw *RWLocker) RUnlock() error {
	if rw.enableDeadlockDetect {
		gid := getGoroutineID()
		info := getGoroutineLockInfo(gid)
		if _, ok := info.holders[rw]; !ok {
			return ErrNotHeld
		}
	}

	rw.upgradeMu.Lock()
	if rw.readerCount > 0 {
		rw.readerCount--
		if rw.writerWaiting && rw.readerCount == 0 {
			rw.upgradeCond.Broadcast()
		}
	}
	rw.upgradeMu.Unlock()

	rw.unregisterHolder()
	rw.mu.RUnlock()
	return nil
}

func (rw *RWLocker) Lock() error {
	if err := rw.checkDeadlock(LockTypeWrite); err != nil {
		return err
	}
	rw.incRequest(LockTypeWrite)
	start := time.Now()

	if rw.writeTimeout <= 0 {
		rw.mu.Lock()
		rw.registerHolder(LockTypeWrite)
		rw.upgradeMu.Lock()
		rw.writerActive = true
		rw.upgradeMu.Unlock()
		rw.incSuccess(LockTypeWrite, time.Since(start))
		return nil
	}

	done := make(chan struct{})
	go func() {
		rw.mu.Lock()
		close(done)
	}()

	timer := time.NewTimer(rw.writeTimeout)
	defer timer.Stop()

	select {
	case <-done:
		rw.registerHolder(LockTypeWrite)
		rw.upgradeMu.Lock()
		rw.writerActive = true
		rw.upgradeMu.Unlock()
		rw.incSuccess(LockTypeWrite, time.Since(start))
		return nil
	case <-timer.C:
		rw.incTimeout()
		go func() {
			<-done
			rw.mu.Unlock()
		}()
		return &TimeoutError{
			LockType: string(LockTypeWrite),
			Timeout:  rw.writeTimeout,
		}
	}
}

func (rw *RWLocker) Unlock() error {
	if rw.enableDeadlockDetect {
		gid := getGoroutineID()
		info := getGoroutineLockInfo(gid)
		if _, ok := info.holders[rw]; !ok {
			return ErrNotHeld
		}
	}

	rw.upgradeMu.Lock()
	rw.writerActive = false
	rw.upgradeMu.Unlock()

	rw.unregisterHolder()
	rw.mu.Unlock()
	return nil
}

func (rw *RWLocker) TryUpgrade(mode UpgradeMode, timeout time.Duration) error {
	var myReadCount int
	if rw.enableDeadlockDetect {
		gid := getGoroutineID()
		info := getGoroutineLockInfo(gid)
		holder, exists := info.holders[rw]
		if !exists {
			return &UpgradeError{
				Reason:   "current goroutine does not hold read lock",
				Blocking: mode == UpgradeBlocking,
			}
		}
		if holder.lockType != LockTypeRead {
			return &UpgradeError{
				Reason:   fmt.Sprintf("current goroutine holds %s lock, not read lock", holder.lockType),
				Blocking: mode == UpgradeBlocking,
			}
		}
		myReadCount = holder.count
	} else {
		myReadCount = 1
	}

	rw.incUpgradeRequest()
	start := time.Now()

	rw.upgradeMu.Lock()
	currentReaders := rw.readerCount
	otherReaders := currentReaders - myReadCount

	if otherReaders <= 0 {
		rw.readerCount = 0
		rw.writerWaiting = true
		rw.upgradeMu.Unlock()

		if rw.enableDeadlockDetect {
			for i := 0; i < myReadCount; i++ {
				rw.unregisterHolder()
			}
		}
		for i := 0; i < myReadCount; i++ {
			rw.mu.RUnlock()
		}
		rw.mu.Lock()

		rw.upgradeMu.Lock()
		rw.writerWaiting = false
		rw.writerActive = true
		rw.upgradeCond.Broadcast()
		rw.upgradeMu.Unlock()

		rw.registerHolder(LockTypeWrite)
		rw.incUpgradeSuccess(time.Since(start))
		return nil
	}

	if mode == UpgradeNonBlocking {
		err := &UpgradeError{
			Reason:      "other readers hold the lock",
			ReaderCount: otherReaders,
			Blocking:    false,
		}
		rw.upgradeMu.Unlock()
		return err
	}

	rw.writerWaiting = true
	rw.readerCount -= myReadCount
	rw.upgradeMu.Unlock()

	if rw.enableDeadlockDetect {
		for i := 0; i < myReadCount; i++ {
			rw.unregisterHolder()
		}
	}
	for i := 0; i < myReadCount; i++ {
		rw.mu.RUnlock()
	}

	deadline := time.Time{}
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}

	var timer *time.Timer
	if timeout > 0 {
		timer = time.AfterFunc(timeout, func() {
			rw.upgradeMu.Lock()
			rw.upgradeCond.Broadcast()
			rw.upgradeMu.Unlock()
		})
		defer timer.Stop()
	}

	rw.upgradeMu.Lock()
	for rw.readerCount > 0 {
		if timeout > 0 && time.Now().After(deadline) {
			rw.writerWaiting = false
			remainingReaders := rw.readerCount
			rw.readerCount += myReadCount
			rw.upgradeCond.Broadcast()
			rw.upgradeMu.Unlock()
			rw.incTimeout()

			for i := 0; i < myReadCount; i++ {
				rw.mu.RLock()
				rw.registerHolder(LockTypeRead)
			}

			return &UpgradeError{
				Reason:      "timed out waiting for other readers",
				ReaderCount: remainingReaders,
				Blocking:    true,
			}
		}
		rw.upgradeCond.Wait()
	}
	rw.writerWaiting = false
	rw.upgradeCond.Broadcast()
	rw.upgradeMu.Unlock()

	rw.mu.Lock()

	rw.upgradeMu.Lock()
	rw.writerActive = true
	rw.upgradeMu.Unlock()

	rw.registerHolder(LockTypeWrite)
	rw.incUpgradeSuccess(time.Since(start))
	return nil
}

func (rw *RWLocker) TryRLock() (bool, error) {
	if err := rw.checkDeadlock(LockTypeRead); err != nil {
		return false, err
	}

	rw.upgradeMu.Lock()
	if rw.writerWaiting {
		rw.upgradeMu.Unlock()
		return false, nil
	}
	rw.readerCount++
	rw.upgradeMu.Unlock()

	rw.incRequest(LockTypeRead)
	start := time.Now()

	ok := rw.mu.TryRLock()
	if !ok {
		rw.upgradeMu.Lock()
		rw.readerCount--
		if rw.writerWaiting && rw.readerCount == 0 {
			rw.upgradeCond.Broadcast()
		}
		rw.upgradeMu.Unlock()
		return false, nil
	}

	rw.registerHolder(LockTypeRead)
	rw.incSuccess(LockTypeRead, time.Since(start))
	return true, nil
}

func (rw *RWLocker) TryLock() (bool, error) {
	if err := rw.checkDeadlock(LockTypeWrite); err != nil {
		return false, err
	}
	rw.incRequest(LockTypeWrite)
	start := time.Now()

	ok := rw.mu.TryLock()
	if !ok {
		return false, nil
	}

	rw.registerHolder(LockTypeWrite)
	rw.upgradeMu.Lock()
	rw.writerActive = true
	rw.upgradeMu.Unlock()
	rw.incSuccess(LockTypeWrite, time.Since(start))
	return true, nil
}

func (rw *RWLocker) ReaderCount() int {
	rw.upgradeMu.Lock()
	defer rw.upgradeMu.Unlock()
	return rw.readerCount
}

func (rw *RWLocker) IsWriterActive() bool {
	rw.upgradeMu.Lock()
	defer rw.upgradeMu.Unlock()
	return rw.writerActive
}
