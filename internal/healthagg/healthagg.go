package healthagg

import (
	"errors"
	"sync"
)

var (
	ErrProbeNotFound      = errors.New("healthagg: probe not found")
	ErrProbeExists        = errors.New("healthagg: probe already exists")
	ErrInvalidProbe       = errors.New("healthagg: invalid probe")
	ErrInvalidConfig      = errors.New("healthagg: invalid configuration")
	ErrAggregatorStopped  = errors.New("healthagg: aggregator is stopped")
)

type HealthStatus int

const (
	StatusHealthy   HealthStatus = iota
	StatusDegraded
	StatusUnhealthy
)

func (s HealthStatus) String() string {
	switch s {
	case StatusHealthy:
		return "healthy"
	case StatusDegraded:
		return "degraded"
	case StatusUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

type AggregationStrategy int

const (
	StrategyAllHealthy AggregationStrategy = iota
	StrategyWeightedMajority
)

type ProbeResult struct {
	Healthy bool
	Details string
}

type ProbeCheckResult struct {
	Name    string
	Healthy bool
	Details string
}

type ProbeFunc func() ProbeResult

type ProbeConfig struct {
	Name     string
	Probe    ProbeFunc
	Critical bool
	Weight   int
}

type StatusChangeEvent struct {
	PreviousStatus HealthStatus
	CurrentStatus  HealthStatus
	FailedProbes   []string
	Timestamp      int64
}

type AlertCallback func(event StatusChangeEvent)

type AggregatedHealth struct {
	Status       HealthStatus
	ProbeResults []ProbeCheckResult
	FailedProbes []string
	HealthyCount int
	TotalCount   int
}

type HealthAggregator struct {
	mu             sync.RWMutex
	probes         map[string]*ProbeConfig
	strategy       AggregationStrategy
	majorityRatio  float64
	lastStatus     HealthStatus
	alertCallbacks map[string]AlertCallback
	nextCallbackID uint64
	running        bool
}

type probeOutcome struct {
	name   string
	result ProbeResult
	config *ProbeConfig
}

type AggregatorConfig struct {
	Strategy      AggregationStrategy
	MajorityRatio float64
}

func DefaultAggregatorConfig() AggregatorConfig {
	return AggregatorConfig{
		Strategy:      StrategyAllHealthy,
		MajorityRatio: 0.5,
	}
}

func NewHealthAggregator(cfg AggregatorConfig) (*HealthAggregator, error) {
	if cfg.MajorityRatio < 0 || cfg.MajorityRatio > 1 {
		return nil, ErrInvalidConfig
	}
	if cfg.MajorityRatio == 0 {
		cfg.MajorityRatio = 0.5
	}

	return &HealthAggregator{
		probes:         make(map[string]*ProbeConfig),
		strategy:       cfg.Strategy,
		majorityRatio:  cfg.MajorityRatio,
		lastStatus:     StatusHealthy,
		alertCallbacks: make(map[string]AlertCallback),
		running:        true,
	}, nil
}

func (ha *HealthAggregator) RegisterProbe(cfg ProbeConfig) error {
	if cfg.Name == "" || cfg.Probe == nil {
		return ErrInvalidProbe
	}
	if cfg.Weight < 0 {
		cfg.Weight = 1
	}
	if cfg.Weight == 0 {
		cfg.Weight = 1
	}

	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !ha.running {
		return ErrAggregatorStopped
	}

	if _, exists := ha.probes[cfg.Name]; exists {
		return ErrProbeExists
	}

	ha.probes[cfg.Name] = &ProbeConfig{
		Name:     cfg.Name,
		Probe:    cfg.Probe,
		Critical: cfg.Critical,
		Weight:   cfg.Weight,
	}

	return nil
}

func (ha *HealthAggregator) UnregisterProbe(name string) error {
	if name == "" {
		return ErrInvalidProbe
	}

	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !ha.running {
		return ErrAggregatorStopped
	}

	if _, exists := ha.probes[name]; !exists {
		return ErrProbeNotFound
	}

	delete(ha.probes, name)
	return nil
}

func (ha *HealthAggregator) GetProbe(name string) (*ProbeConfig, error) {
	if name == "" {
		return nil, ErrInvalidProbe
	}

	ha.mu.RLock()
	defer ha.mu.RUnlock()

	if !ha.running {
		return nil, ErrAggregatorStopped
	}

	probe, exists := ha.probes[name]
	if !exists {
		return nil, ErrProbeNotFound
	}

	return &ProbeConfig{
		Name:     probe.Name,
		Critical: probe.Critical,
		Weight:   probe.Weight,
	}, nil
}

func (ha *HealthAggregator) ProbeCount() int {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	return len(ha.probes)
}

func (ha *HealthAggregator) Check() AggregatedHealth {
	ha.mu.RLock()
	if !ha.running {
		ha.mu.RUnlock()
		return AggregatedHealth{
			Status:     StatusUnhealthy,
			TotalCount: 0,
		}
	}

	probeConfigs := make([]*ProbeConfig, 0, len(ha.probes))
	for _, p := range ha.probes {
		probeConfigs = append(probeConfigs, p)
	}
	strategy := ha.strategy
	majorityRatio := ha.majorityRatio
	ha.mu.RUnlock()

	outcomes := make([]probeOutcome, 0, len(probeConfigs))
	for _, pc := range probeConfigs {
		r := pc.Probe()
		outcomes = append(outcomes, probeOutcome{name: pc.Name, result: r, config: pc})
	}

	aggregated := ha.aggregateResults(outcomes, strategy, majorityRatio)

	ha.maybeTriggerAlert(aggregated)

	return aggregated
}

func (ha *HealthAggregator) aggregateResults(
	outcomes []probeOutcome,
	strategy AggregationStrategy,
	majorityRatio float64,
) AggregatedHealth {
	healthyCount := 0
	criticalFailed := false
	var failedProbes []string

	totalWeight := 0
	healthyWeight := 0
	criticalTotalWeight := 0
	criticalHealthyWeight := 0

	var checkResults []ProbeCheckResult

	for _, o := range outcomes {
		weight := o.config.Weight
		totalWeight += weight

		checkResults = append(checkResults, ProbeCheckResult{
			Name:    o.name,
			Healthy: o.result.Healthy,
			Details: o.result.Details,
		})

		if o.result.Healthy {
			healthyCount++
			healthyWeight += weight
			if o.config.Critical {
				criticalTotalWeight += weight
				criticalHealthyWeight += weight
			}
		} else {
			failedProbes = append(failedProbes, o.name)
			if o.config.Critical {
				criticalFailed = true
				criticalTotalWeight += weight
			}
		}
	}

	var status HealthStatus

	switch strategy {
	case StrategyAllHealthy:
		if criticalFailed {
			status = StatusUnhealthy
		} else if len(failedProbes) > 0 {
			status = StatusDegraded
		} else {
			status = StatusHealthy
		}

	case StrategyWeightedMajority:
		if totalWeight == 0 {
			status = StatusHealthy
		} else {
			healthyRatio := float64(healthyWeight) / float64(totalWeight)

			if criticalTotalWeight > 0 {
				criticalHealthyRatio := float64(criticalHealthyWeight) / float64(criticalTotalWeight)
				if criticalHealthyRatio <= majorityRatio {
					status = StatusUnhealthy
				} else if healthyRatio <= majorityRatio {
					status = StatusDegraded
				} else {
					status = StatusHealthy
				}
			} else {
				if healthyRatio <= majorityRatio {
					status = StatusDegraded
				} else {
					status = StatusHealthy
				}
			}
		}

	default:
		status = StatusHealthy
	}

	return AggregatedHealth{
		Status:       status,
		ProbeResults: checkResults,
		FailedProbes: failedProbes,
		HealthyCount: healthyCount,
		TotalCount:   len(outcomes),
	}
}

func (ha *HealthAggregator) maybeTriggerAlert(aggregated AggregatedHealth) {
	ha.mu.Lock()

	prevStatus := ha.lastStatus
	if prevStatus == aggregated.Status {
		ha.mu.Unlock()
		return
	}

	ha.lastStatus = aggregated.Status

	if prevStatus != StatusUnhealthy && aggregated.Status != StatusUnhealthy {
		ha.mu.Unlock()
		return
	}

	callbacks := make([]AlertCallback, 0, len(ha.alertCallbacks))
	for _, cb := range ha.alertCallbacks {
		callbacks = append(callbacks, cb)
	}
	ha.mu.Unlock()

	event := StatusChangeEvent{
		PreviousStatus: prevStatus,
		CurrentStatus:  aggregated.Status,
		FailedProbes:   aggregated.FailedProbes,
		Timestamp:      0,
	}

	for _, cb := range callbacks {
		cb(event)
	}
}

func (ha *HealthAggregator) SubscribeAlert(callback AlertCallback) (string, error) {
	if callback == nil {
		return "", errors.New("healthagg: callback cannot be nil")
	}

	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !ha.running {
		return "", ErrAggregatorStopped
	}

	ha.nextCallbackID++
	id := "alert-" + uint64ToStr(ha.nextCallbackID)
	ha.alertCallbacks[id] = callback

	return id, nil
}

func (ha *HealthAggregator) UnsubscribeAlert(id string) error {
	if id == "" {
		return errors.New("healthagg: invalid callback id")
	}

	ha.mu.Lock()
	defer ha.mu.Unlock()

	if !ha.running {
		return ErrAggregatorStopped
	}

	if _, exists := ha.alertCallbacks[id]; !exists {
		return ErrProbeNotFound
	}

	delete(ha.alertCallbacks, id)
	return nil
}

func (ha *HealthAggregator) AlertCallbackCount() int {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	return len(ha.alertCallbacks)
}

func (ha *HealthAggregator) LastStatus() HealthStatus {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	return ha.lastStatus
}

func (ha *HealthAggregator) Stop() {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.running = false
}

func (ha *HealthAggregator) Start() {
	ha.mu.Lock()
	defer ha.mu.Unlock()
	ha.running = true
}

func (ha *HealthAggregator) IsRunning() bool {
	ha.mu.RLock()
	defer ha.mu.RUnlock()
	return ha.running
}

func uint64ToStr(n uint64) string {
	if n == 0 {
		return "0"
	}
	digits := make([]byte, 0, 20)
	for n > 0 {
		digits = append(digits, byte('0'+n%10))
		n /= 10
	}
	for i, j := 0, len(digits)-1; i < j; i, j = i+1, j-1 {
		digits[i], digits[j] = digits[j], digits[i]
	}
	return string(digits)
}
