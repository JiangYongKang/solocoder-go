package fallback

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

func NewChain(cfg *ChainConfig) *Chain {
	if cfg == nil {
		cfg = DefaultChainConfig()
	}
	return &Chain{
		strategies:   make([]*Strategy, 0),
		strategyMap:  make(map[string]*Strategy),
		currentIndex: 0,
		state:        ChainStateHealthy,
		recoveryCfg:  cfg.Recovery,
		metrics:      ChainMetrics{},
	}
}

func (c *Chain) RegisterStrategy(id, name string, priority int, handler HandlerFunc, triggerCond *TriggerCondition) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if handler == nil {
		return ErrNilHandler
	}
	if id == "" {
		return fmt.Errorf("%w: strategy id cannot be empty", ErrInvalidConfig)
	}
	if priority < 0 {
		return fmt.Errorf("%w: priority must be >= 0", ErrInvalidPriority)
	}
	if _, exists := c.strategyMap[id]; exists {
		return fmt.Errorf("%w: %s", ErrStrategyAlreadyExists, id)
	}

	strategy := &Strategy{
		ID:          id,
		Name:        name,
		Priority:    priority,
		Handler:     handler,
		TriggerCond: triggerCond,
		State:       StrategyStateActive,
		ErrorWindow: make([]errorEntry, 0),
	}

	c.strategies = append(c.strategies, strategy)
	c.strategyMap[id] = strategy
	c.sortStrategies()

	if len(c.strategies) == 1 {
		c.activeStrategyID = id
	}

	return nil
}

func (c *Chain) sortStrategies() {
	sort.Slice(c.strategies, func(i, j int) bool {
		return c.strategies[i].Priority < c.strategies[j].Priority
	})
}

func (c *Chain) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return ErrChainAlreadyRunning
	}
	if len(c.strategies) == 0 {
		c.mu.Unlock()
		return ErrNoStrategies
	}

	c.running = true
	c.stopCh = make(chan struct{})
	c.state = ChainStateHealthy
	c.currentIndex = 0
	c.activeStrategyID = c.strategies[0].ID
	c.mu.Unlock()

	if c.recoveryCfg.Mode == RecoveryModeActive {
		c.wg.Add(1)
		go c.activeProbeLoop(ctx)
	}

	return nil
}

func (c *Chain) Stop() {
	c.mu.Lock()
	if !c.running {
		c.mu.Unlock()
		return
	}
	c.running = false
	close(c.stopCh)
	c.mu.Unlock()

	c.wg.Wait()
}

func (c *Chain) Execute(ctx context.Context) (interface{}, error) {
	c.mu.RLock()
	if !c.running {
		c.mu.RUnlock()
		return nil, ErrChainNotRunning
	}
	if len(c.strategies) == 0 {
		c.mu.RUnlock()
		return nil, ErrNoStrategies
	}
	strategies := make([]*Strategy, len(c.strategies))
	copy(strategies, c.strategies)
	startIndex := c.currentIndex
	c.mu.RUnlock()

	var result interface{}
	var errs []error
	totalStart := time.Now()

	for i := startIndex; i < len(strategies); i++ {
		strategy := strategies[i]

		if i > startIndex {
			c.updateFallbackCount()
		}

		if shouldSkip, skipIndex := c.checkTriggerConditions(ctx, strategy, errs, i, strategies); shouldSkip {
			i = skipIndex - 1
			continue
		}

		execResult := c.executeStrategy(ctx, strategy)
		c.updateStrategyStats(strategy, execResult)

		if execResult.Err == nil {
			result = execResult.Result
			c.updateActiveStrategy(i, strategy.ID)
			c.updateMetrics(true, time.Since(totalStart))
			c.checkPassiveRecovery(strategy)
			return result, nil
		}

		errs = append(errs, execResult.Err)
		c.markStrategyDegraded(strategy)

		if shouldSkip, skipIndex := c.checkTriggerConditions(ctx, strategy, errs, i, strategies); shouldSkip {
			i = skipIndex - 1
		}
	}

	c.updateMetrics(false, time.Since(totalStart))
	c.updateChainState(ChainStateDegraded)

	if len(errs) == 0 {
		return nil, ErrAllStrategiesFailed
	}

	return nil, &AggregateError{Errors: errs}
}

func (c *Chain) executeStrategy(ctx context.Context, strategy *Strategy) executionResult {
	start := time.Now()

	var result interface{}
	var err error

	if strategy.TriggerCond != nil && strategy.TriggerCond.Type == TriggerConditionTimeout && strategy.TriggerCond.Timeout > 0 {
		done := make(chan struct{})
		var panicVal interface{}

		go func() {
			defer func() {
				if r := recover(); r != nil {
					panicVal = r
				}
				close(done)
			}()
			result, err = strategy.Handler(ctx)
		}()

		select {
		case <-done:
			if panicVal != nil {
				err = fmt.Errorf("handler panic: %v", panicVal)
			}
		case <-time.After(strategy.TriggerCond.Timeout):
			err = fmt.Errorf("%w: strategy %s timed out after %v", ErrExecutionTimeout, strategy.Name, strategy.TriggerCond.Timeout)
		case <-ctx.Done():
			err = ctx.Err()
		}
	} else {
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("handler panic: %v", r)
				}
			}()
			result, err = strategy.Handler(ctx)
		}()
	}

	return executionResult{
		Result:       result,
		Err:          err,
		StrategyID:   strategy.ID,
		StrategyName: strategy.Name,
		Duration:     time.Since(start),
	}
}

func (c *Chain) checkTriggerConditions(ctx context.Context, strategy *Strategy, errs []error, currentIdx int, strategies []*Strategy) (bool, int) {
	if strategy.TriggerCond == nil || len(errs) == 0 {
		return false, 0
	}

	lastErr := errs[len(errs)-1]

	for i := currentIdx + 1; i < len(strategies); i++ {
		s := strategies[i]
		if s.TriggerCond == nil {
			continue
		}

		if c.matchTriggerCondition(lastErr, s.TriggerCond) {
			return true, i
		}
	}

	if c.matchTriggerCondition(lastErr, strategy.TriggerCond) {
		if currentIdx+1 < len(strategies) {
			return true, currentIdx + 1
		}
	}

	return false, 0
}

func (c *Chain) matchTriggerCondition(err error, cond *TriggerCondition) bool {
	if cond == nil {
		return false
	}

	switch cond.Type {
	case TriggerConditionErrorType:
		for _, targetErr := range cond.ErrorTypes {
			if errIsType(err, targetErr) {
				return true
			}
		}
	case TriggerConditionErrorRate:
		return false
	case TriggerConditionTimeout:
		if errIsType(err, ErrExecutionTimeout) {
			return true
		}
	case TriggerConditionCustom:
		if cond.CustomCheck != nil {
			return cond.CustomCheck(err)
		}
	}

	return false
}

func errIsType(err, target error) bool {
	if err == nil || target == nil {
		return false
	}
	if err == target {
		return true
	}

	type unwrapper interface {
		Unwrap() error
	}

	type multiUnwrapper interface {
		Unwrap() []error
	}

	if u, ok := err.(multiUnwrapper); ok {
		for _, e := range u.Unwrap() {
			if errIsType(e, target) {
				return true
			}
		}
	} else if u, ok := err.(unwrapper); ok {
		return errIsType(u.Unwrap(), target)
	}

	return false
}

func (c *Chain) updateStrategyStats(strategy *Strategy, result executionResult) {
	strategy.mu.Lock()
	defer strategy.mu.Unlock()

	strategy.LastUsedAt = time.Now()

	if result.Err == nil {
		strategy.SuccessCount++
		strategy.ConsecutiveFail = 0
	} else {
		strategy.FailureCount++
		strategy.ConsecutiveFail++
		strategy.ErrorWindow = append(strategy.ErrorWindow, errorEntry{
			Time: time.Now(),
			Err:  result.Err,
		})
		c.cleanupErrorWindow(strategy)
	}
}

func (c *Chain) cleanupErrorWindow(strategy *Strategy) {
	if strategy.TriggerCond == nil || strategy.TriggerCond.ErrorWindow <= 0 {
		if len(strategy.ErrorWindow) > 100 {
			strategy.ErrorWindow = strategy.ErrorWindow[len(strategy.ErrorWindow)-100:]
		}
		return
	}

	cutoff := time.Now().Add(-strategy.TriggerCond.ErrorWindow)
	i := 0
	for ; i < len(strategy.ErrorWindow); i++ {
		if strategy.ErrorWindow[i].Time.After(cutoff) {
			break
		}
	}
	strategy.ErrorWindow = strategy.ErrorWindow[i:]
}

func (c *Chain) markStrategyDegraded(strategy *Strategy) {
	strategy.mu.Lock()
	defer strategy.mu.Unlock()
	strategy.State = StrategyStateDegraded
}

func (c *Chain) updateActiveStrategy(index int, strategyID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.currentIndex = index
	c.activeStrategyID = strategyID
}

func (c *Chain) updateFallbackCount() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics.FallbackCount++
}

func (c *Chain) updateMetrics(success bool, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.metrics.TotalExecutions++
	if success {
		c.metrics.TotalSuccesses++
	} else {
		c.metrics.TotalFailures++
	}

	if c.metrics.TotalExecutions == 1 {
		c.metrics.AvgResponseTime = duration
	} else {
		total := c.metrics.AvgResponseTime*time.Duration(c.metrics.TotalExecutions-1) + duration
		c.metrics.AvgResponseTime = total / time.Duration(c.metrics.TotalExecutions)
	}
}

func (c *Chain) updateChainState(state ChainState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

func (c *Chain) checkPassiveRecovery(currentStrategy *Strategy) {
	if c.recoveryCfg.Mode != RecoveryModePassive {
		return
	}

	c.mu.RLock()
	if c.currentIndex == 0 {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()

	mainStrategy := c.getMainStrategy()
	if mainStrategy == nil {
		return
	}

	mainStrategy.mu.RLock()
	consecutiveFail := mainStrategy.ConsecutiveFail
	mainStrategy.mu.RUnlock()

	if consecutiveFail > 0 {
		return
	}

	successCount := c.countRecentSuccesses(mainStrategy)
	if successCount >= c.recoveryCfg.PassiveSuccessCount {
		go c.initiateRecovery(mainStrategy)
	}
}

func (c *Chain) countRecentSuccesses(strategy *Strategy) int {
	strategy.mu.RLock()
	defer strategy.mu.RUnlock()

	window := c.recoveryCfg.PassiveSuccessWindow
	if window <= 0 {
		return int(strategy.SuccessCount)
	}

	cutoff := time.Now().Add(-window)
	count := 0
	for _, entry := range strategy.ErrorWindow {
		if entry.Time.After(cutoff) {
			count++
		}
	}

	return int(strategy.SuccessCount) - count
}

func (c *Chain) getMainStrategy() *Strategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.strategies) == 0 {
		return nil
	}
	return c.strategies[0]
}

func (c *Chain) activeProbeLoop(ctx context.Context) {
	defer c.wg.Done()

	interval := c.recoveryCfg.CheckInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.runProbeCycle(ctx)
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

func (c *Chain) runProbeCycle(ctx context.Context) {
	c.mu.RLock()
	if c.currentIndex == 0 {
		c.mu.RUnlock()
		return
	}
	mainStrategy := c.strategies[0]
	c.mu.RUnlock()

	if mainStrategy == nil {
		return
	}

	mainStrategy.mu.Lock()
	if mainStrategy.State != StrategyStateDegraded {
		mainStrategy.mu.Unlock()
		return
	}
	mainStrategy.State = StrategyStateRecovering
	mainStrategy.mu.Unlock()

	successCount := 0
	failureCount := 0
	threshold := c.recoveryCfg.ProbeSuccessThreshold
	if threshold <= 0 {
		threshold = 3
	}
	failThreshold := c.recoveryCfg.ProbeFailureThreshold
	if failThreshold <= 0 {
		failThreshold = 1
	}

	for i := 0; i < threshold*2; i++ {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		result := c.executeStrategy(ctx, mainStrategy)
		if result.Err == nil {
			successCount++
			failureCount = 0
			if successCount >= threshold {
				break
			}
		} else {
			failureCount++
			successCount = 0
			if failureCount >= failThreshold {
				mainStrategy.mu.Lock()
				mainStrategy.State = StrategyStateDegraded
				mainStrategy.mu.Unlock()
				return
			}
		}

		time.Sleep(100 * time.Millisecond)
	}

	if successCount >= threshold {
		c.initiateRecovery(mainStrategy)
	} else {
		mainStrategy.mu.Lock()
		mainStrategy.State = StrategyStateDegraded
		mainStrategy.mu.Unlock()
	}
}

func (c *Chain) initiateRecovery(mainStrategy *Strategy) {
	mainStrategy.mu.Lock()
	mainStrategy.State = StrategyStateWarmingUp
	mainStrategy.mu.Unlock()

	c.updateChainState(ChainStateRecovering)

	warmUpDuration := c.recoveryCfg.WarmUpDuration
	if warmUpDuration > 0 {
		if !c.runWarmUpPeriod(mainStrategy, warmUpDuration) {
			mainStrategy.mu.Lock()
			mainStrategy.State = StrategyStateDegraded
			mainStrategy.mu.Unlock()
			c.updateChainState(ChainStateDegraded)
			return
		}
	}

	c.switchToMainStrategy(mainStrategy)
}

func (c *Chain) runWarmUpPeriod(mainStrategy *Strategy, duration time.Duration) bool {
	ctx := context.Background()
	deadline := time.Now().Add(duration)
	successCount := 0
	failureCount := 0

	for time.Now().Before(deadline) {
		select {
		case <-c.stopCh:
			return false
		default:
		}

		result := c.executeStrategy(ctx, mainStrategy)
		if result.Err == nil {
			successCount++
			failureCount = 0
		} else {
			failureCount++
			if failureCount >= c.recoveryCfg.ProbeFailureThreshold {
				return false
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	return successCount > 0
}

func (c *Chain) switchToMainStrategy(mainStrategy *Strategy) {
	mainStrategy.mu.Lock()
	mainStrategy.State = StrategyStateActive
	mainStrategy.ConsecutiveFail = 0
	mainStrategy.mu.Unlock()

	c.mu.Lock()
	c.currentIndex = 0
	c.activeStrategyID = mainStrategy.ID
	c.state = ChainStateHealthy
	c.metrics.RecoveryCount++
	c.mu.Unlock()

	for _, s := range c.strategies {
		if s.ID != mainStrategy.ID {
			s.mu.Lock()
			s.State = StrategyStateActive
			s.ConsecutiveFail = 0
			s.mu.Unlock()
		}
	}
}

func (c *Chain) State() ChainState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Chain) CurrentStrategyID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.activeStrategyID
}

func (c *Chain) CurrentIndex() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.currentIndex
}

func (c *Chain) Metrics() ChainMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.metrics
}

func (c *Chain) StrategyCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.strategies)
}

func (c *Chain) GetStrategy(id string) (*Strategy, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s, exists := c.strategyMap[id]
	return s, exists
}

func (c *Chain) GetAllStrategies() []*Strategy {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*Strategy, len(c.strategies))
	copy(result, c.strategies)
	return result
}

func (c *Chain) ResetMetrics() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = ChainMetrics{}
}

func (s *Strategy) Stats() (successCount, failureCount, consecutiveFail uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SuccessCount, s.FailureCount, s.ConsecutiveFail
}

func (s *Strategy) GetState() StrategyState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

var ErrInvalidConfig = fmt.Errorf("invalid configuration")

func (c *Chain) ForceSwitchToStrategy(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	strategy, exists := c.strategyMap[id]
	if !exists {
		return fmt.Errorf("%w: %s", ErrStrategyNotFound, id)
	}

	for i, s := range c.strategies {
		if s.ID == id {
			c.currentIndex = i
			c.activeStrategyID = id
			break
		}
	}

	strategy.mu.Lock()
	strategy.State = StrategyStateActive
	strategy.mu.Unlock()

	return nil
}

func (c *Chain) ForceSwitchToMain() error {
	c.mu.Lock()
	if len(c.strategies) == 0 {
		c.mu.Unlock()
		return ErrNoStrategies
	}
	mainID := c.strategies[0].ID
	c.mu.Unlock()
	return c.ForceSwitchToStrategy(mainID)
}

func (c *Chain) IsRunning() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.running
}

func (c *Chain) CalculateErrorRate(strategyID string, window time.Duration) (float64, error) {
	strategy, exists := c.GetStrategy(strategyID)
	if !exists {
		return 0, fmt.Errorf("%w: %s", ErrStrategyNotFound, strategyID)
	}

	strategy.mu.RLock()
	defer strategy.mu.RUnlock()

	if window <= 0 {
		total := strategy.SuccessCount + strategy.FailureCount
		if total == 0 {
			return 0, nil
		}
		return float64(strategy.FailureCount) / float64(total), nil
	}

	cutoff := time.Now().Add(-window)
	successes := 0
	failures := 0

	for _, entry := range strategy.ErrorWindow {
		if entry.Time.After(cutoff) {
			failures++
		}
	}

	total := successes + failures
	if total == 0 {
		return 0, nil
	}

	return float64(failures) / float64(total), nil
}

var _ sync.Locker = (*sync.Mutex)(nil)
