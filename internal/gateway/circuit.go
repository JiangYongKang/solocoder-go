package gateway

import (
	"time"
)

func NewCircuitBreaker(name string, windowSize time.Duration, failureThreshold int, openDuration time.Duration, halfOpenMaxRequests int) *CircuitBreaker {
	return &CircuitBreaker{
		name:                name,
		state:               StateClosed,
		failures:            make([]FailureEntry, 0),
		windowSize:          windowSize,
		failureThreshold:    failureThreshold,
		openDuration:        openDuration,
		halfOpenMaxRequests: halfOpenMaxRequests,
		halfOpenRequests:    0,
		halfOpenSuccesses:   0,
		halfOpenFailures:    0,
	}
}

func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.checkStateTransition()
	return cb.state
}

func (cb *CircuitBreaker) Name() string {
	return cb.name
}

func (cb *CircuitBreaker) Allow() (bool, bool) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.checkStateTransition()

	switch cb.state {
	case StateClosed:
		return true, true
	case StateOpen:
		return false, false
	case StateHalfOpen:
		if cb.halfOpenRequests >= cb.halfOpenMaxRequests {
			return false, false
		}
		cb.halfOpenRequests++
		return true, true
	}
	return false, false
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		cb.failures = cb.failures[:0]
	case StateHalfOpen:
		cb.halfOpenSuccesses++
		if cb.halfOpenSuccesses >= cb.halfOpenMaxRequests {
			cb.transitionToClosed()
		}
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cb.failures = append(cb.failures, FailureEntry{
		Time:    now,
		Success: false,
	})

	switch cb.state {
	case StateClosed:
		if cb.countRecentFailures() >= cb.failureThreshold {
			cb.transitionToOpen()
		}
	case StateHalfOpen:
		cb.halfOpenFailures++
		cb.transitionToOpen()
	}
}

func (cb *CircuitBreaker) checkStateTransition() {
	if cb.state == StateOpen {
		elapsed := time.Since(cb.lastOpenTime)
		if elapsed >= cb.openDuration {
			cb.transitionToHalfOpen()
		}
	}
}

func (cb *CircuitBreaker) transitionToOpen() {
	cb.state = StateOpen
	cb.lastOpenTime = time.Now()
}

func (cb *CircuitBreaker) transitionToHalfOpen() {
	cb.state = StateHalfOpen
	cb.halfOpenRequests = 0
	cb.halfOpenSuccesses = 0
	cb.halfOpenFailures = 0
}

func (cb *CircuitBreaker) transitionToClosed() {
	cb.state = StateClosed
	cb.failures = make([]FailureEntry, 0)
}

func (cb *CircuitBreaker) countRecentFailures() int {
	cutoff := time.Now().Add(-cb.windowSize)
	count := 0
	for i := len(cb.failures) - 1; i >= 0; i-- {
		entry := cb.failures[i]
		if entry.Time.Before(cutoff) {
			break
		}
		if !entry.Success {
			count++
		}
	}
	return count
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToClosed()
}

func (cb *CircuitBreaker) ForceOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToOpen()
}

func (cb *CircuitBreaker) ForceClosed() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.transitionToClosed()
}

func (cb *CircuitBreaker) FailureCount() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.countRecentFailures()
}

type CircuitBreakerConfig struct {
	Name                string
	WindowSize          time.Duration
	FailureThreshold    int
	OpenDuration        time.Duration
	HalfOpenMaxRequests int
	Fallback            HandlerFunc
}
