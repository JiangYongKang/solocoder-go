package timeoutprop

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Config struct {
	TotalTimeout time.Duration
	MinThreshold time.Duration
}

func DefaultConfig() Config {
	return Config{
		TotalTimeout: 10 * time.Second,
		MinThreshold: 10 * time.Millisecond,
	}
}

func NewPropagator() *Propagator {
	return NewPropagatorWithConfig(DefaultConfig())
}

func NewPropagatorWithConfig(cfg Config) *Propagator {
	if cfg.TotalTimeout <= 0 {
		cfg.TotalTimeout = DefaultConfig().TotalTimeout
	}
	if cfg.MinThreshold < 0 {
		cfg.MinThreshold = 0
	}
	return &Propagator{
		stages:       make([]*Stage, 0),
		stageMap:     make(map[string]*Stage),
		totalTimeout: cfg.TotalTimeout,
		minThreshold: cfg.MinThreshold,
	}
}

func (p *Propagator) AddStage(name string, budget time.Duration, fn StageFunc) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.executed {
		return ErrChainAlreadyExecuted
	}
	if name == "" {
		return ErrEmptyName
	}
	if fn == nil {
		return ErrNilHandler
	}
	if budget < 0 {
		return ErrNegativeBudget
	}
	if _, exists := p.stageMap[name]; exists {
		return fmt.Errorf("%w: %s", ErrStageAlreadyExists, name)
	}

	stage := newStage(name, budget, fn)
	p.stages = append(p.stages, stage)
	p.stageMap[name] = stage
	return nil
}

func (p *Propagator) TotalBudget() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	var total time.Duration
	for _, s := range p.stages {
		total += s.allocatedBudget
	}
	return total
}

func (p *Propagator) StageCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.stages)
}

func (p *Propagator) Execute(parentCtx context.Context) (*ChainReport, error) {
	p.mu.Lock()
	if p.executed {
		p.mu.Unlock()
		return p.buildInitialReport(false, "", ""), ErrChainAlreadyExecuted
	}
	if len(p.stages) == 0 {
		p.mu.Unlock()
		return p.buildInitialReport(false, "", "no stages registered"), ErrNoStages
	}
	p.executed = true

	var totalBudget time.Duration
	for _, s := range p.stages {
		totalBudget += s.allocatedBudget
	}
	if totalBudget > p.totalTimeout {
		report := p.buildInitialReport(false, "",
			fmt.Sprintf("total budget %v exceeds total timeout %v", totalBudget, p.totalTimeout))
		p.mu.Unlock()
		return report, fmt.Errorf("%w: total budget %v exceeds total timeout %v",
			ErrBudgetExceedsTotal, totalBudget, p.totalTimeout)
	}
	p.mu.Unlock()

	rootCtx, rootCancel := context.WithTimeout(parentCtx, p.totalTimeout)
	defer rootCancel()

	p.rootCancel = rootCancel

	chainStart := time.Now()
	var lastError error
	var failedStage string
	var timeoutReason string
	success := true

	var carryOverBudget time.Duration

	for i, stage := range p.stages {
		if rootCtx.Err() != nil {
			markStageSkipped(stage, TimeoutTypeTotal)
			success = false
			if failedStage == "" {
				failedStage = stage.name
				timeoutReason = "total timeout reached before stage start"
				lastError = rootCtx.Err()
			}
			continue
		}

		remainingTime := getRemainingTime(rootCtx)

		if remainingTime < p.minThreshold {
			markStageSkipped(stage, TimeoutTypeMinThreshold)
			continue
		}

		hasBudget := stage.allocatedBudget > 0 || carryOverBudget > 0
		stageBudget := stage.allocatedBudget + carryOverBudget
		if stageBudget > remainingTime {
			stageBudget = remainingTime
		}

		var stageCtx context.Context
		var stageCancel context.CancelFunc
		if hasBudget {
			stageCtx, stageCancel = context.WithTimeout(rootCtx, stageBudget)
		} else {
			stageCtx, stageCancel = context.WithCancel(rootCtx)
			stageBudget = remainingTime
		}

		startTime := time.Now()
		updateStageStart(stage, startTime, stageBudget)

		done := make(chan error, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("stage panic: %v", r)
				}
			}()
			done <- stage.fn(stageCtx)
		}()

		var err error
		select {
		case err = <-done:
		case <-stageCtx.Done():
			err = stageCtx.Err()
		case <-rootCtx.Done():
			err = rootCtx.Err()
		}

		stageCancel()

		endTime := time.Now()
		usedTime := endTime.Sub(startTime)

		if err != nil {
			success = false
			if failedStage == "" {
				failedStage = stage.name
			}

			timeoutType := TimeoutTypeNone
			stageStatus := StageStatusFailed
			if errors.Is(err, context.DeadlineExceeded) {
				stageStatus = StageStatusTimedOut
				if !hasBudget || errors.Is(rootCtx.Err(), context.DeadlineExceeded) {
					timeoutType = TimeoutTypeTotal
					timeoutReason = "total timeout exceeded"
				} else {
					timeoutType = TimeoutTypeBudget
					timeoutReason = fmt.Sprintf("stage budget exceeded: %v", stageBudget)
				}
				lastError = &StageTimeoutError{
					StageName:   stage.name,
					TimeoutType: timeoutType,
					Allocated:   stageBudget,
					Used:        usedTime,
				}
			} else {
				timeoutReason = fmt.Sprintf("stage error: %v", err)
				lastError = err
			}

			updateStageEnd(stage, endTime, usedTime, stageStatus, timeoutType, err)

			for j := i + 1; j < len(p.stages); j++ {
				markStageSkipped(p.stages[j], timeoutType)
			}
			break
		} else {
			updateStageEnd(stage, endTime, usedTime, StageStatusCompleted, TimeoutTypeNone, nil)

			if hasBudget && usedTime < stageBudget {
				carryOverBudget = stageBudget - usedTime
			} else {
				carryOverBudget = 0
			}
		}
	}

	totalUsed := time.Since(chainStart)
	remainingTime := p.totalTimeout - totalUsed
	if remainingTime < 0 {
		remainingTime = 0
	}

	stageInfos := make([]*StageInfo, len(p.stages))
	for i, s := range p.stages {
		stageInfos[i] = getStageInfoCopy(s)
	}

	report := &ChainReport{
		TotalTimeout:  p.totalTimeout,
		TotalUsed:     totalUsed,
		RemainingTime: remainingTime,
		Stages:        stageInfos,
		Success:       success,
		FailedStage:   failedStage,
		TimeoutReason: timeoutReason,
	}

	p.mu.Lock()
	p.report = report
	p.mu.Unlock()

	if !success {
		return report, lastError
	}
	return report, nil
}

func (p *Propagator) buildInitialReport(success bool, failedStage string, reason string) *ChainReport {
	stageInfos := make([]*StageInfo, len(p.stages))
	for i, s := range p.stages {
		stageInfos[i] = getStageInfoCopy(s)
	}
	return &ChainReport{
		TotalTimeout:  p.totalTimeout,
		TotalUsed:     0,
		RemainingTime: p.totalTimeout,
		Stages:        stageInfos,
		Success:       success,
		FailedStage:   failedStage,
		TimeoutReason: reason,
	}
}

func (p *Propagator) Report() *ChainReport {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.report == nil {
		return nil
	}
	return copyReport(p.report)
}

func (p *Propagator) GetStageInfo(name string) (*StageInfo, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stage, exists := p.stageMap[name]
	if !exists {
		return nil, false
	}
	return getStageInfoCopy(stage), true
}

func (p *Propagator) RemainingTime(ctx context.Context) time.Duration {
	return getRemainingTime(ctx)
}

func (p *Propagator) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.executed = false
	p.report = nil
	p.rootCancel = nil

	for _, s := range p.stages {
		s.mu.Lock()
		s.info = &StageInfo{
			Name:            s.name,
			AllocatedBudget: s.allocatedBudget,
			Status:          StageStatusPending,
			TimeoutType:     TimeoutTypeNone,
		}
		s.mu.Unlock()
	}
}

func getRemainingTime(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 1<<63 - 1
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}

func markStageSkipped(stage *Stage, timeoutType TimeoutType) {
	stage.mu.Lock()
	defer stage.mu.Unlock()
	now := time.Now()
	stage.info.Status = StageStatusSkipped
	stage.info.TimeoutType = timeoutType
	stage.info.StartTime = now
	stage.info.EndTime = now
	stage.info.UsedBudget = 0
	stage.info.RemainingBudget = stage.allocatedBudget
}

func updateStageStart(stage *Stage, startTime time.Time, budget time.Duration) {
	stage.mu.Lock()
	defer stage.mu.Unlock()
	stage.info.Status = StageStatusRunning
	stage.info.StartTime = startTime
	stage.info.AllocatedBudget = budget
	stage.info.RemainingBudget = budget
}

func updateStageEnd(stage *Stage, endTime time.Time, used time.Duration, status StageStatus, timeoutType TimeoutType, err error) {
	stage.mu.Lock()
	defer stage.mu.Unlock()
	stage.info.Status = status
	stage.info.EndTime = endTime
	stage.info.UsedBudget = used
	stage.info.RemainingBudget = stage.info.AllocatedBudget - used
	if stage.info.RemainingBudget < 0 {
		stage.info.RemainingBudget = 0
	}
	stage.info.TimeoutType = timeoutType
	stage.info.Error = err
}

func getStageInfoCopy(stage *Stage) *StageInfo {
	stage.mu.Lock()
	defer stage.mu.Unlock()
	info := *stage.info
	return &info
}

func copyReport(r *ChainReport) *ChainReport {
	cp := *r
	cp.Stages = make([]*StageInfo, len(r.Stages))
	for i, s := range r.Stages {
		info := *s
		cp.Stages[i] = &info
	}
	return &cp
}

type Option func(*Config)

func WithTotalTimeout(d time.Duration) Option {
	return func(c *Config) {
		c.TotalTimeout = d
	}
}

func WithMinThreshold(d time.Duration) Option {
	return func(c *Config) {
		c.MinThreshold = d
	}
}

func Execute(parentCtx context.Context, fn func(p *Propagator) error, opts ...Option) (*ChainReport, error) {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	p := NewPropagatorWithConfig(cfg)
	if err := fn(p); err != nil {
		return nil, err
	}
	return p.Execute(parentCtx)
}

var _ sync.Locker = (*sync.Mutex)(nil)
