package graceful

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var (
	ErrManagerAlreadyShuttingDown = errors.New("graceful: manager is already shutting down")
	ErrManagerNotRunning          = errors.New("graceful: manager is not running")
	ErrCallbackAlreadyRegistered  = errors.New("graceful: callback already registered with this name")
	ErrCallbackNotFound           = errors.New("graceful: callback not found")
	ErrNilCallback                = errors.New("graceful: callback function cannot be nil")
	ErrManagerNotStarted          = errors.New("graceful: manager has not been started")
)

type ShutdownPhase int

const (
	PhaseInit ShutdownPhase = iota
	PhaseStopAccepting
	PhaseWaitRequests
	PhaseExecuteCallbacks
	PhaseComplete
	PhaseForced
)

func (p ShutdownPhase) String() string {
	switch p {
	case PhaseInit:
		return "Init"
	case PhaseStopAccepting:
		return "StopAccepting"
	case PhaseWaitRequests:
		return "WaitRequests"
	case PhaseExecuteCallbacks:
		return "ExecuteCallbacks"
	case PhaseComplete:
		return "Complete"
	case PhaseForced:
		return "Forced"
	default:
		return "Unknown"
	}
}

type ShutdownState int

const (
	StateRunning ShutdownState = iota
	StateShuttingDown
	StateCompleted
)

func (s ShutdownState) String() string {
	switch s {
	case StateRunning:
		return "Running"
	case StateShuttingDown:
		return "ShuttingDown"
	case StateCompleted:
		return "Completed"
	default:
		return "Unknown"
	}
}

type CleanupFunc func(ctx context.Context) error

type CleanupCallback struct {
	Name     string
	Func     CleanupFunc
	Timeout  time.Duration
	Priority int
}

type CallbackResult struct {
	Name     string
	Success  bool
	TimedOut bool
	Error    error
	Duration time.Duration
}

type ShutdownReport struct {
	Success             bool
	Forced              bool
	Phase               ShutdownPhase
	TotalDuration       time.Duration
	ActiveRequests      int
	GoroutineCount      int
	CallbackResults     []*CallbackResult
	IncompleteCallbacks []string
	Errors              []error
}

type Config struct {
	RequestWaitTimeout  time.Duration
	GlobalTimeout       time.Duration
	DefaultCallbackTimeout time.Duration
	StopAcceptingTimeout time.Duration
	Signals             []os.Signal
}

func DefaultConfig() Config {
	return Config{
		RequestWaitTimeout:     30 * time.Second,
		GlobalTimeout:          60 * time.Second,
		DefaultCallbackTimeout: 10 * time.Second,
		StopAcceptingTimeout:   5 * time.Second,
		Signals:                []os.Signal{syscall.SIGINT, syscall.SIGTERM},
	}
}

type Manager struct {
	activeRequests int64

	cfg Config

	mu             sync.RWMutex
	state          ShutdownState
	phase          ShutdownPhase
	accepting      bool

	callbacks map[string]*CleanupCallback

	signalCh    chan os.Signal
	stopSignalCh chan struct{}

	shutdownCh    chan struct{}
	shutdownOnce  sync.Once
	completedCh   chan struct{}

	report   *ShutdownReport
	reportMu sync.Mutex

	manualTriggerCh chan struct{}
}

func NewManager(cfg Config) *Manager {
	if cfg.RequestWaitTimeout <= 0 {
		cfg.RequestWaitTimeout = DefaultConfig().RequestWaitTimeout
	}
	if cfg.GlobalTimeout <= 0 {
		cfg.GlobalTimeout = DefaultConfig().GlobalTimeout
	}
	if cfg.DefaultCallbackTimeout <= 0 {
		cfg.DefaultCallbackTimeout = DefaultConfig().DefaultCallbackTimeout
	}
	if cfg.StopAcceptingTimeout <= 0 {
		cfg.StopAcceptingTimeout = DefaultConfig().StopAcceptingTimeout
	}
	if len(cfg.Signals) == 0 {
		cfg.Signals = DefaultConfig().Signals
	}

	return &Manager{
		cfg:             cfg,
		state:           StateRunning,
		phase:           PhaseInit,
		accepting:       true,
		callbacks:       make(map[string]*CleanupCallback),
		shutdownCh:      make(chan struct{}),
		completedCh:     make(chan struct{}),
		manualTriggerCh: make(chan struct{}, 1),
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.state != StateRunning || m.signalCh != nil {
		m.mu.Unlock()
		return ErrManagerNotRunning
	}

	m.signalCh = make(chan os.Signal, 1)
	m.stopSignalCh = make(chan struct{})
	signal.Notify(m.signalCh, m.cfg.Signals...)

	go m.signalListener()

	m.mu.Unlock()
	return nil
}

func (m *Manager) signalListener() {
	for {
		select {
		case <-m.signalCh:
			_ = m.Shutdown()
			return
		case <-m.manualTriggerCh:
			_ = m.Shutdown()
			return
		case <-m.stopSignalCh:
			return
		}
	}
}

func (m *Manager) RegisterCallback(name string, fn CleanupFunc, timeout time.Duration, priority int) error {
	if fn == nil {
		return ErrNilCallback
	}
	if name == "" {
		return errors.New("graceful: callback name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state != StateRunning {
		return ErrManagerAlreadyShuttingDown
	}

	if _, exists := m.callbacks[name]; exists {
		return ErrCallbackAlreadyRegistered
	}

	if timeout <= 0 {
		timeout = m.cfg.DefaultCallbackTimeout
	}

	m.callbacks[name] = &CleanupCallback{
		Name:     name,
		Func:     fn,
		Timeout:  timeout,
		Priority: priority,
	}

	return nil
}

func (m *Manager) UnregisterCallback(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.callbacks[name]; !exists {
		return ErrCallbackNotFound
	}

	delete(m.callbacks, name)
	return nil
}

func (m *Manager) BeginRequest() error {
	for {
		m.mu.RLock()
		accepting := m.accepting
		state := m.state
		m.mu.RUnlock()

		if !accepting || state != StateRunning {
			return ErrManagerAlreadyShuttingDown
		}

		newCount := atomic.AddInt64(&m.activeRequests, 1)
		m.mu.RLock()
		stillAccepting := m.accepting
		stillRunning := m.state == StateRunning
		m.mu.RUnlock()

		if stillAccepting && stillRunning {
			return nil
		}

		if newCount == atomic.LoadInt64(&m.activeRequests) {
			atomic.AddInt64(&m.activeRequests, -1)
		} else {
			atomic.AddInt64(&m.activeRequests, -1)
		}
		return ErrManagerAlreadyShuttingDown
	}
}

func (m *Manager) EndRequest() {
	atomic.AddInt64(&m.activeRequests, -1)
}

func (m *Manager) ActiveRequests() int {
	return int(atomic.LoadInt64(&m.activeRequests))
}

func (m *Manager) IsAccepting() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accepting
}

func (m *Manager) State() ShutdownState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) Phase() ShutdownPhase {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.phase
}

func (m *Manager) Shutdown() error {
	m.mu.RLock()
	currentState := m.state
	m.mu.RUnlock()

	if currentState != StateRunning {
		return ErrManagerAlreadyShuttingDown
	}

	var firstErr error

	m.shutdownOnce.Do(func() {
		m.mu.Lock()
		if m.state != StateRunning {
			m.mu.Unlock()
			firstErr = ErrManagerAlreadyShuttingDown
			return
		}
		m.state = StateShuttingDown
		m.mu.Unlock()

		close(m.shutdownCh)

		if m.stopSignalCh != nil {
			close(m.stopSignalCh)
			signal.Stop(m.signalCh)
		}

		report := m.runShutdownPhases()

		m.mu.Lock()
		m.state = StateCompleted
		m.report = report
		m.mu.Unlock()

		close(m.completedCh)
	})

	return firstErr
}

func (m *Manager) TriggerShutdown() error {
	m.mu.RLock()
	state := m.state
	signalChExists := m.signalCh != nil
	m.mu.RUnlock()

	if state != StateRunning {
		return ErrManagerAlreadyShuttingDown
	}

	if !signalChExists {
		return m.Shutdown()
	}

	select {
	case m.manualTriggerCh <- struct{}{}:
	case <-m.shutdownCh:
		return ErrManagerAlreadyShuttingDown
	}

	return nil
}

func (m *Manager) WaitForShutdown() *ShutdownReport {
	select {
	case <-m.completedCh:
	case <-m.shutdownCh:
		<-m.completedCh
	}

	m.mu.RLock()
	rep := m.report
	m.mu.RUnlock()

	if rep == nil {
		return &ShutdownReport{Success: false}
	}
	return rep
}

func (m *Manager) ShutdownDone() <-chan struct{} {
	return m.completedCh
}

func (m *Manager) GetReport() *ShutdownReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.report
}

func (m *Manager) runShutdownPhases() *ShutdownReport {
	startTime := time.Now()
	report := &ShutdownReport{
		Success: false,
	}

	globalCtx, globalCancel := context.WithTimeout(context.Background(), m.cfg.GlobalTimeout)
	defer globalCancel()

	phaseErrors := make([]error, 0)

	m.setPhase(PhaseStopAccepting)
	if err := m.phaseStopAccepting(globalCtx); err != nil {
		phaseErrors = append(phaseErrors, fmt.Errorf("phase StopAccepting: %w", err))
	}

	if globalCtx.Err() != nil {
		m.setPhase(PhaseForced)
		report.Forced = true
		report.Errors = append(report.Errors, fmt.Errorf("global timeout during StopAccepting phase: %w", globalCtx.Err()))
		return m.finalizeReport(report, startTime, phaseErrors)
	}

	m.setPhase(PhaseWaitRequests)
	waitErr := m.phaseWaitRequests(globalCtx)
	if waitErr != nil {
		phaseErrors = append(phaseErrors, fmt.Errorf("phase WaitRequests: %w", waitErr))
	}

	if globalCtx.Err() != nil {
		m.setPhase(PhaseForced)
		report.Forced = true
		report.Errors = append(report.Errors, fmt.Errorf("global timeout during WaitRequests phase: %w", globalCtx.Err()))
		return m.finalizeReport(report, startTime, phaseErrors)
	}

	m.setPhase(PhaseExecuteCallbacks)
	cbResults, incomplete, cbErr := m.phaseExecuteCallbacks(globalCtx)
	report.CallbackResults = cbResults
	report.IncompleteCallbacks = incomplete
	if cbErr != nil {
		phaseErrors = append(phaseErrors, fmt.Errorf("phase ExecuteCallbacks: %w", cbErr))
	}

	if globalCtx.Err() != nil {
		m.setPhase(PhaseForced)
		report.Forced = true
		report.Errors = append(report.Errors, fmt.Errorf("global timeout during ExecuteCallbacks phase: %w", globalCtx.Err()))
	} else {
		m.setPhase(PhaseComplete)
		allCallbacksSucceeded := true
		for _, r := range report.CallbackResults {
			if !r.Success {
				allCallbacksSucceeded = false
				break
			}
		}
		if len(phaseErrors) == 0 && allCallbacksSucceeded {
			report.Success = true
		}
	}

	return m.finalizeReport(report, startTime, phaseErrors)
}

func (m *Manager) finalizeReport(report *ShutdownReport, startTime time.Time, errs []error) *ShutdownReport {
	report.TotalDuration = time.Since(startTime)
	report.Phase = m.Phase()
	report.ActiveRequests = m.ActiveRequests()
	report.GoroutineCount = runtime.NumGoroutine()
	report.Errors = append(report.Errors, errs...)
	return report
}

func (m *Manager) setPhase(phase ShutdownPhase) {
	m.mu.Lock()
	m.phase = phase
	m.mu.Unlock()
}

func (m *Manager) phaseStopAccepting(ctx context.Context) error {
	m.mu.Lock()
	m.accepting = false
	m.mu.Unlock()

	deadline := time.Now().Add(m.cfg.StopAcceptingTimeout)
	for time.Now().Before(deadline) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if m.ActiveRequests() == 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

func (m *Manager) phaseWaitRequests(ctx context.Context) error {
	waitCtx, cancel := context.WithTimeout(ctx, m.cfg.RequestWaitTimeout)
	defer cancel()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			if waitCtx.Err() == context.DeadlineExceeded {
				return fmt.Errorf("wait requests timed out after %v: %d active requests remaining",
					m.cfg.RequestWaitTimeout, m.ActiveRequests())
			}
			return waitCtx.Err()

		case <-ticker.C:
			if m.ActiveRequests() == 0 {
				return nil
			}
		}
	}
}

func (m *Manager) phaseExecuteCallbacks(ctx context.Context) ([]*CallbackResult, []string, error) {
	m.mu.RLock()
	cbs := make([]*CleanupCallback, 0, len(m.callbacks))
	for _, cb := range m.callbacks {
		cbs = append(cbs, cb)
	}
	m.mu.RUnlock()

	sort.SliceStable(cbs, func(i, j int) bool {
		if cbs[i].Priority != cbs[j].Priority {
			return cbs[i].Priority > cbs[j].Priority
		}
		return cbs[i].Name < cbs[j].Name
	})

	results := make([]*CallbackResult, 0, len(cbs))
	incomplete := make([]string, 0)
	var firstErr error

	for i := len(cbs) - 1; i >= 0; i-- {
		if ctx.Err() != nil {
			for j := i; j >= 0; j-- {
				incomplete = append(incomplete, cbs[j].Name)
				results = append(results, &CallbackResult{
					Name:     cbs[j].Name,
					Success:  false,
					TimedOut: true,
					Error:    ctx.Err(),
					Duration: 0,
				})
			}
			break
		}

		cb := cbs[i]
		result := m.executeSingleCallback(ctx, cb)
		results = append(results, result)

		if !result.Success {
			incomplete = append(incomplete, result.Name)
			if firstErr == nil {
				firstErr = fmt.Errorf("callback '%s' failed: %w", result.Name, result.Error)
			}
		}
	}

	return results, incomplete, firstErr
}

func (m *Manager) executeSingleCallback(parentCtx context.Context, cb *CleanupCallback) *CallbackResult {
	result := &CallbackResult{
		Name: cb.Name,
	}

	startTime := time.Now()

	cbCtx, cancel := context.WithTimeout(parentCtx, cb.Timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("callback panicked: %v", r)
			}
			done <- err
		}()
		err = cb.Func(cbCtx)
	}()

	select {
	case err := <-done:
		result.Duration = time.Since(startTime)
		if err != nil {
			result.Success = false
			result.Error = err
		} else {
			result.Success = true
		}
	case <-cbCtx.Done():
		result.Duration = time.Since(startTime)
		result.Success = false
		result.TimedOut = true
		if parentCtx.Err() != nil {
			result.Error = fmt.Errorf("global timeout interrupted callback")
		} else {
			result.Error = fmt.Errorf("callback timed out after %v", cb.Timeout)
		}
	}

	return result
}
