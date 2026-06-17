package healthagg

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewHealthAggregator(t *testing.T) {
	ha, err := NewHealthAggregator(DefaultAggregatorConfig())
	if err != nil {
		t.Fatalf("NewHealthAggregator failed: %v", err)
	}
	if ha == nil {
		t.Fatal("NewHealthAggregator returned nil")
	}
	if !ha.IsRunning() {
		t.Error("expected aggregator to be running after creation")
	}
	if ha.ProbeCount() != 0 {
		t.Errorf("expected 0 probes, got %d", ha.ProbeCount())
	}
	if ha.LastStatus() != StatusHealthy {
		t.Errorf("expected initial status to be healthy, got %v", ha.LastStatus())
	}
}

func TestDefaultAggregatorConfig(t *testing.T) {
	cfg := DefaultAggregatorConfig()
	if cfg.Strategy != StrategyAllHealthy {
		t.Errorf("expected default strategy to be StrategyAllHealthy, got %v", cfg.Strategy)
	}
	if cfg.MajorityRatio != 0.5 {
		t.Errorf("expected default majority ratio to be 0.5, got %f", cfg.MajorityRatio)
	}
}

func TestNewHealthAggregatorWithInvalidRatio(t *testing.T) {
	tests := []struct {
		name        string
		ratio       float64
		expectError bool
	}{
		{"negative", -1.0, true},
		{"zero_not_set", 0.0, false},
		{"greater than one", 1.5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := AggregatorConfig{Strategy: StrategyWeightedMajority, MajorityRatio: tt.ratio}
			ha, err := NewHealthAggregator(cfg)
			if tt.expectError {
				if err != ErrInvalidConfig {
					t.Errorf("expected ErrInvalidConfig, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ha.majorityRatio != 0.5 {
				t.Errorf("expected majority ratio to default to 0.5, got %f", ha.majorityRatio)
			}
			ha.Stop()
		})
	}
}

func TestRegisterProbe(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.RegisterProbe(ProbeConfig{
		Name: "db",
		Probe: func() ProbeResult {
			return ProbeResult{Healthy: true, Details: "connected"}
		},
		Critical: true,
		Weight:   1,
	})
	if err != nil {
		t.Fatalf("RegisterProbe failed: %v", err)
	}
	if ha.ProbeCount() != 1 {
		t.Errorf("expected 1 probe, got %d", ha.ProbeCount())
	}
}

func TestRegisterProbeInvalid(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	tests := []struct {
		name string
		cfg  ProbeConfig
	}{
		{"empty name", ProbeConfig{Name: "", Probe: func() ProbeResult { return ProbeResult{} }}},
		{"nil probe", ProbeConfig{Name: "test", Probe: nil}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ha.RegisterProbe(tt.cfg)
			if err != ErrInvalidProbe {
				t.Errorf("expected ErrInvalidProbe for %s, got %v", tt.name, err)
			}
		})
	}
}

func TestRegisterProbeDuplicate(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	cfg := ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	}
	ha.RegisterProbe(cfg)

	err := ha.RegisterProbe(cfg)
	if err != ErrProbeExists {
		t.Errorf("expected ErrProbeExists, got %v", err)
	}
}

func TestRegisterProbeStopped(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	ha.Stop()

	err := ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	if err != ErrAggregatorStopped {
		t.Errorf("expected ErrAggregatorStopped, got %v", err)
	}
}

func TestRegisterProbeWithZeroWeight(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.RegisterProbe(ProbeConfig{
		Name:   "test",
		Probe:  func() ProbeResult { return ProbeResult{Healthy: true} },
		Weight: 0,
	})
	if err != nil {
		t.Fatalf("RegisterProbe failed: %v", err)
	}

	p, _ := ha.GetProbe("test")
	if p.Weight != 1 {
		t.Errorf("expected zero weight to default to 1, got %d", p.Weight)
	}
}

func TestRegisterProbeWithNegativeWeight(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.RegisterProbe(ProbeConfig{
		Name:   "test",
		Probe:  func() ProbeResult { return ProbeResult{Healthy: true} },
		Weight: -5,
	})
	if err != nil {
		t.Fatalf("RegisterProbe failed: %v", err)
	}

	p, _ := ha.GetProbe("test")
	if p.Weight != 1 {
		t.Errorf("expected negative weight to default to 1, got %d", p.Weight)
	}
}

func TestUnregisterProbe(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})

	err := ha.UnregisterProbe("db")
	if err != nil {
		t.Fatalf("UnregisterProbe failed: %v", err)
	}
	if ha.ProbeCount() != 0 {
		t.Errorf("expected 0 probes after unregister, got %d", ha.ProbeCount())
	}
}

func TestUnregisterProbeNotFound(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.UnregisterProbe("nonexistent")
	if err != ErrProbeNotFound {
		t.Errorf("expected ErrProbeNotFound, got %v", err)
	}
}

func TestUnregisterProbeInvalidName(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.UnregisterProbe("")
	if err != ErrInvalidProbe {
		t.Errorf("expected ErrInvalidProbe, got %v", err)
	}
}

func TestUnregisterProbeStopped(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.Stop()

	err := ha.UnregisterProbe("db")
	if err != ErrAggregatorStopped {
		t.Errorf("expected ErrAggregatorStopped, got %v", err)
	}
}

func TestGetProbe(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: true,
		Weight:   3,
	})

	p, err := ha.GetProbe("db")
	if err != nil {
		t.Fatalf("GetProbe failed: %v", err)
	}
	if p.Name != "db" {
		t.Errorf("expected name 'db', got %s", p.Name)
	}
	if !p.Critical {
		t.Error("expected probe to be critical")
	}
	if p.Weight != 3 {
		t.Errorf("expected weight 3, got %d", p.Weight)
	}
}

func TestGetProbeNotFound(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	_, err := ha.GetProbe("nonexistent")
	if err != ErrProbeNotFound {
		t.Errorf("expected ErrProbeNotFound, got %v", err)
	}
}

func TestGetProbeInvalidName(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	_, err := ha.GetProbe("")
	if err != ErrInvalidProbe {
		t.Errorf("expected ErrInvalidProbe, got %v", err)
	}
}

func TestGetProbeStopped(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.Stop()

	_, err := ha.GetProbe("db")
	if err != ErrAggregatorStopped {
		t.Errorf("expected ErrAggregatorStopped, got %v", err)
	}
}

func TestCheckAllHealthyStrategyAllPass(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true, Details: "ok"} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:  "cache",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true, Details: "ok"} },
	})

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy, got %v", result.Status)
	}
	if result.HealthyCount != 2 {
		t.Errorf("expected 2 healthy probes, got %d", result.HealthyCount)
	}
	if result.TotalCount != 2 {
		t.Errorf("expected 2 total probes, got %d", result.TotalCount)
	}
	if len(result.FailedProbes) != 0 {
		t.Errorf("expected 0 failed probes, got %d", len(result.FailedProbes))
	}
}

func TestCheckAllHealthyStrategyNonCriticalFails(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: true,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false, Details: "timeout"} },
		Critical: false,
	})

	result := ha.Check()
	if result.Status != StatusDegraded {
		t.Errorf("expected StatusDegraded, got %v", result.Status)
	}
	if len(result.FailedProbes) != 1 {
		t.Errorf("expected 1 failed probe, got %d", len(result.FailedProbes))
	}
	if result.FailedProbes[0] != "cache" {
		t.Errorf("expected failed probe 'cache', got %s", result.FailedProbes[0])
	}
}

func TestCheckAllHealthyStrategyCriticalFails(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false, Details: "connection lost"} },
		Critical: true,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
	})

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy, got %v", result.Status)
	}
	if len(result.FailedProbes) != 1 {
		t.Errorf("expected 1 failed probe, got %d", len(result.FailedProbes))
	}
}

func TestCheckAllHealthyStrategyMultipleFailures(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: true,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "api",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
	})

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy, got %v", result.Status)
	}
	if result.HealthyCount != 1 {
		t.Errorf("expected 1 healthy probe, got %d", result.HealthyCount)
	}
}

func TestCheckWeightedMajorityAllHealthy(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: true,
		Weight:   3,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
		Weight:   1,
	})

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy, got %v", result.Status)
	}
}

func TestCheckWeightedMajorityCriticalFails(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: true,
		Weight:   3,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
		Weight:   10,
	})

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy when critical probe below majority, got %v", result.Status)
	}
}

func TestCheckWeightedMajorityCriticalPassesNonCriticalFails(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: true,
		Weight:   5,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "monitoring",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
		Weight:   1,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "logging",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
		Weight:   1,
	})

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy when majority healthy, got %v", result.Status)
	}
}

func TestCheckWeightedMajorityMostFail(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.6,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
		Weight:   2,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "cache",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
		Weight:   3,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "api",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
		Weight:   3,
	})

	result := ha.Check()
	if result.Status != StatusDegraded {
		t.Errorf("expected StatusDegraded when below majority but no critical failures, got %v", result.Status)
	}
}

func TestCheckWeightedMajorityNoProbes(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy with no probes, got %v", result.Status)
	}
	if result.TotalCount != 0 {
		t.Errorf("expected 0 total probes, got %d", result.TotalCount)
	}
}

func TestCheckWeightedMajorityNoCriticalProbes(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.7,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "a",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
		Weight:   1,
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "b",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: false,
		Weight:   1,
	})

	result := ha.Check()
	if result.Status != StatusDegraded {
		t.Errorf("expected StatusDegraded when below majority, got %v", result.Status)
	}
}

func TestCheckStoppedAggregator(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	ha.RegisterProbe(ProbeConfig{
		Name:  "db",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.Stop()

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy for stopped aggregator, got %v", result.Status)
	}
}

func TestSubscribeAlert(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var receivedEvent StatusChangeEvent
	var mu sync.Mutex

	id, err := ha.SubscribeAlert(func(event StatusChangeEvent) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("SubscribeAlert failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty callback ID")
	}
	if ha.AlertCallbackCount() != 1 {
		t.Errorf("expected 1 callback, got %d", ha.AlertCallbackCount())
	}

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: true,
	})
	ha.Check()

	mu.Lock()
	if receivedEvent.CurrentStatus != StatusUnhealthy {
		t.Errorf("expected received event status unhealthy, got %v", receivedEvent.CurrentStatus)
	}
	mu.Unlock()
}

func TestSubscribeAlertNilCallback(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	_, err := ha.SubscribeAlert(nil)
	if err == nil {
		t.Error("expected error for nil callback")
	}
}

func TestSubscribeAlertStopped(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	ha.Stop()

	_, err := ha.SubscribeAlert(func(event StatusChangeEvent) {})
	if err != ErrAggregatorStopped {
		t.Errorf("expected ErrAggregatorStopped, got %v", err)
	}
}

func TestUnsubscribeAlert(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	id, _ := ha.SubscribeAlert(func(event StatusChangeEvent) {})

	err := ha.UnsubscribeAlert(id)
	if err != nil {
		t.Fatalf("UnsubscribeAlert failed: %v", err)
	}
	if ha.AlertCallbackCount() != 0 {
		t.Errorf("expected 0 callbacks after unsubscribe, got %d", ha.AlertCallbackCount())
	}
}

func TestUnsubscribeAlertNotFound(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.UnsubscribeAlert("nonexistent")
	if err != ErrProbeNotFound {
		t.Errorf("expected ErrProbeNotFound, got %v", err)
	}
}

func TestUnsubscribeAlertInvalidID(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	err := ha.UnsubscribeAlert("")
	if err == nil {
		t.Error("expected error for empty id")
	}
}

func TestUnsubscribeAlertStopped(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	id, _ := ha.SubscribeAlert(func(event StatusChangeEvent) {})
	ha.Stop()

	err := ha.UnsubscribeAlert(id)
	if err != ErrAggregatorStopped {
		t.Errorf("expected ErrAggregatorStopped, got %v", err)
	}
}

func TestAlertTriggeredOnStatusChange(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var eventCount int32
	var lastEvent StatusChangeEvent
	var mu sync.Mutex

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&eventCount, 1)
		mu.Lock()
		lastEvent = event
		mu.Unlock()
	})

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: true,
	})

	ha.Check()

	if atomic.LoadInt32(&eventCount) != 1 {
		t.Errorf("expected 1 alert event, got %d", atomic.LoadInt32(&eventCount))
	}

	mu.Lock()
	if lastEvent.PreviousStatus != StatusHealthy {
		t.Errorf("expected previous status healthy, got %v", lastEvent.PreviousStatus)
	}
	if lastEvent.CurrentStatus != StatusUnhealthy {
		t.Errorf("expected current status unhealthy, got %v", lastEvent.CurrentStatus)
	}
	if len(lastEvent.FailedProbes) != 1 {
		t.Errorf("expected 1 failed probe in event, got %d", len(lastEvent.FailedProbes))
	}
	mu.Unlock()
}

func TestAlertNotTriggeredOnSameStatus(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var eventCount int32

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&eventCount, 1)
	})

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: true,
	})

	ha.Check()
	ha.Check()
	ha.Check()

	if atomic.LoadInt32(&eventCount) != 0 {
		t.Errorf("expected 0 alert events when status doesn't change, got %d", atomic.LoadInt32(&eventCount))
	}
}

func TestAlertMultipleCallbacks(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var count1 int32
	var count2 int32

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&count1, 1)
	})
	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&count2, 1)
	})

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
		Critical: true,
	})

	ha.Check()

	if atomic.LoadInt32(&count1) != 1 {
		t.Errorf("expected callback 1 to be called once, got %d", atomic.LoadInt32(&count1))
	}
	if atomic.LoadInt32(&count2) != 1 {
		t.Errorf("expected callback 2 to be called once, got %d", atomic.LoadInt32(&count2))
	}
}

func TestAlertStatusRecovery(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var events []StatusChangeEvent
	var mu sync.Mutex

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	failing := true
	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe: func() ProbeResult {
			if failing {
				return ProbeResult{Healthy: false, Details: "down"}
			}
			return ProbeResult{Healthy: true, Details: "ok"}
		},
	})

	ha.Check()

	failing = false
	ha.Check()

	mu.Lock()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].PreviousStatus != StatusHealthy || events[0].CurrentStatus != StatusUnhealthy {
		t.Errorf("first event: expected healthy->unhealthy, got %v->%v", events[0].PreviousStatus, events[0].CurrentStatus)
	}
	if events[1].PreviousStatus != StatusUnhealthy || events[1].CurrentStatus != StatusHealthy {
		t.Errorf("second event: expected unhealthy->healthy, got %v->%v", events[1].PreviousStatus, events[1].CurrentStatus)
	}
	mu.Unlock()
}

func TestAlertDegradedToUnhealthy(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var events []StatusChangeEvent
	var mu sync.Mutex

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	criticalFailing := false
	nonCriticalFailing := true

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe: func() ProbeResult {
			if criticalFailing {
				return ProbeResult{Healthy: false}
			}
			return ProbeResult{Healthy: true}
		},
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "monitoring",
		Critical: false,
		Probe: func() ProbeResult {
			if nonCriticalFailing {
				return ProbeResult{Healthy: false}
			}
			return ProbeResult{Healthy: true}
		},
	})

	ha.Check()

	mu.Lock()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for healthy->degraded (no unhealthy involved), got %d", len(events))
	}
	mu.Unlock()

	criticalFailing = true
	ha.Check()

	mu.Lock()
	if len(events) != 1 {
		t.Fatalf("expected 1 event for degraded->unhealthy, got %d", len(events))
	}
	if events[0].PreviousStatus != StatusDegraded {
		t.Errorf("expected previous status degraded, got %v", events[0].PreviousStatus)
	}
	if events[0].CurrentStatus != StatusUnhealthy {
		t.Errorf("expected current status unhealthy, got %v", events[0].CurrentStatus)
	}
	mu.Unlock()
}

func TestHealthStatusString(t *testing.T) {
	tests := []struct {
		status   HealthStatus
		expected string
	}{
		{StatusHealthy, "healthy"},
		{StatusDegraded, "degraded"},
		{StatusUnhealthy, "unhealthy"},
		{HealthStatus(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.status.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.status.String())
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())

	if !ha.IsRunning() {
		t.Error("expected running after creation")
	}

	ha.Stop()
	if ha.IsRunning() {
		t.Error("expected stopped after Stop()")
	}

	ha.Start()
	if !ha.IsRunning() {
		t.Error("expected running after Start()")
	}
}

func TestStopIdempotent(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())

	ha.Stop()
	ha.Stop()
}

func TestStartIdempotent(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())

	ha.Start()
	ha.Start()
}

func TestConcurrentRegisterCheck(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n * 2)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			name := "probe-" + uint64ToStr(uint64(i))
			ha.RegisterProbe(ProbeConfig{
				Name:  name,
				Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
			})
		}(i)
	}

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ha.Check()
		}()
	}

	wg.Wait()
}

func TestConcurrentSubscribeAlert(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ha.SubscribeAlert(func(event StatusChangeEvent) {})
		}()
	}

	wg.Wait()

	if ha.AlertCallbackCount() != n {
		t.Errorf("expected %d callbacks, got %d", n, ha.AlertCallbackCount())
	}
}

func TestCheckProbeResults(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name: "db",
		Probe: func() ProbeResult {
			return ProbeResult{Healthy: true, Details: "connected"}
		},
	})
	ha.RegisterProbe(ProbeConfig{
		Name: "cache",
		Probe: func() ProbeResult {
			return ProbeResult{Healthy: false, Details: "timeout"}
		},
	})

	result := ha.Check()

	if len(result.ProbeResults) != 2 {
		t.Fatalf("expected 2 probe results, got %d", len(result.ProbeResults))
	}

	resultMap := make(map[string]ProbeCheckResult)
	for _, r := range result.ProbeResults {
		resultMap[r.Name] = r
	}

	if dbResult, ok := resultMap["db"]; !ok || !dbResult.Healthy || dbResult.Details != "connected" {
		t.Error("unexpected db probe result")
	}
	if cacheResult, ok := resultMap["cache"]; !ok || cacheResult.Healthy || cacheResult.Details != "timeout" {
		t.Error("unexpected cache probe result")
	}
}

func TestLastStatusUpdates(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})

	ha.Check()
	if ha.LastStatus() != StatusHealthy {
		t.Errorf("expected StatusHealthy, got %v", ha.LastStatus())
	}
}

func TestMultipleCriticalProbesAllHealthy(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "db1",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "db2",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "db3",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy with all critical probes healthy, got %v", result.Status)
	}
}

func TestWeightedMajorityCriticalPassesByWeight(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "primary-db",
		Critical: true,
		Weight:   10,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "replica-db",
		Critical: true,
		Weight:   1,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy when critical majority by weight passes, got %v", result.Status)
	}
}

func TestWeightedMajorityCriticalFailsByWeight(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.7,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "small-db",
		Critical: true,
		Weight:   3,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "big-db",
		Critical: true,
		Weight:   7,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy when critical majority by weight fails, got %v", result.Status)
	}
}

func TestProbeCount(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	if ha.ProbeCount() != 0 {
		t.Errorf("expected 0 probes, got %d", ha.ProbeCount())
	}

	ha.RegisterProbe(ProbeConfig{
		Name:  "a",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:  "b",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})

	if ha.ProbeCount() != 2 {
		t.Errorf("expected 2 probes, got %d", ha.ProbeCount())
	}

	ha.UnregisterProbe("a")

	if ha.ProbeCount() != 1 {
		t.Errorf("expected 1 probe after unregister, got %d", ha.ProbeCount())
	}
}

func TestAlertCallbackCount(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	if ha.AlertCallbackCount() != 0 {
		t.Errorf("expected 0 callbacks, got %d", ha.AlertCallbackCount())
	}

	ha.SubscribeAlert(func(event StatusChangeEvent) {})
	ha.SubscribeAlert(func(event StatusChangeEvent) {})

	if ha.AlertCallbackCount() != 2 {
		t.Errorf("expected 2 callbacks, got %d", ha.AlertCallbackCount())
	}
}

func TestGetProbeReturnsCopy(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "test",
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
		Critical: false,
		Weight:   1,
	})

	p, _ := ha.GetProbe("test")
	p.Critical = true
	p.Weight = 999

	p2, _ := ha.GetProbe("test")
	if p2.Critical {
		t.Error("GetProbe should return a copy, but modifications affected internal state (Critical)")
	}
	if p2.Weight != 1 {
		t.Errorf("GetProbe should return a copy, but modifications affected internal state (Weight: expected 1, got %d)", p2.Weight)
	}
}

func TestCheckNoProbes(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	result := ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy with no probes, got %v", result.Status)
	}
	if result.TotalCount != 0 {
		t.Errorf("expected 0 total probes, got %d", result.TotalCount)
	}
	if result.HealthyCount != 0 {
		t.Errorf("expected 0 healthy probes, got %d", result.HealthyCount)
	}
}

func TestWeightedMajorityAtThreshold(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{
		Strategy:      StrategyWeightedMajority,
		MajorityRatio: 0.5,
	})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "pass1",
		Critical: false,
		Weight:   5,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "fail1",
		Critical: false,
		Weight:   5,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	result := ha.Check()
	if result.Status != StatusDegraded {
		t.Errorf("expected StatusDegraded at exactly majority threshold, got %v", result.Status)
	}
}

func TestConcurrentAlerts(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	failing := int32(0)
	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe: func() ProbeResult {
			if atomic.LoadInt32(&failing) == 1 {
				return ProbeResult{Healthy: false}
			}
			return ProbeResult{Healthy: true}
		},
	})

	var alertCount int32
	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&alertCount, 1)
	})

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				atomic.StoreInt32(&failing, 1)
			} else {
				atomic.StoreInt32(&failing, 0)
			}
			ha.Check()
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt32(&alertCount) < 1 {
		t.Logf("alert count: %d", atomic.LoadInt32(&alertCount))
	}
}

func TestAllHealthyOnlyNonCriticalFail(t *testing.T) {
	ha, _ := NewHealthAggregator(AggregatorConfig{Strategy: StrategyAllHealthy})
	defer ha.Stop()

	ha.RegisterProbe(ProbeConfig{
		Name:     "c1",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "nc1",
		Critical: false,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "nc2",
		Critical: false,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	result := ha.Check()
	if result.Status != StatusDegraded {
		t.Errorf("expected StatusDegraded when only non-critical probes fail, got %v", result.Status)
	}
}

func TestAggregatorRestart(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())

	ha.Stop()
	if ha.IsRunning() {
		t.Error("should be stopped")
	}

	result := ha.Check()
	if result.Status != StatusUnhealthy {
		t.Errorf("expected StatusUnhealthy when stopped, got %v", result.Status)
	}

	ha.Start()
	if !ha.IsRunning() {
		t.Error("should be running after restart")
	}

	ha.RegisterProbe(ProbeConfig{
		Name:  "test",
		Probe: func() ProbeResult { return ProbeResult{Healthy: true} },
	})

	result = ha.Check()
	if result.Status != StatusHealthy {
		t.Errorf("expected StatusHealthy after restart with healthy probe, got %v", result.Status)
	}
}

func TestAlertNotTriggeredForHealthyToDegraded(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var eventCount int32

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		atomic.AddInt32(&eventCount, 1)
	})

	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "monitoring",
		Critical: false,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	ha.Check()

	if atomic.LoadInt32(&eventCount) != 0 {
		t.Errorf("expected 0 alerts for healthy->degraded (no unhealthy involved), got %d", atomic.LoadInt32(&eventCount))
	}
}

func TestAlertNotTriggeredForDegradedToHealthy(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var events []StatusChangeEvent
	var mu sync.Mutex

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	ncFailing := true
	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: true} },
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "monitoring",
		Critical: false,
		Probe: func() ProbeResult {
			if ncFailing {
				return ProbeResult{Healthy: false}
			}
			return ProbeResult{Healthy: true}
		},
	})

	ha.Check()

	mu.Lock()
	if len(events) != 0 {
		t.Fatalf("expected 0 events for healthy->degraded, got %d", len(events))
	}
	mu.Unlock()

	ncFailing = false
	ha.Check()

	mu.Lock()
	if len(events) != 0 {
		t.Errorf("expected 0 alerts for degraded->healthy (no unhealthy involved), got %d", len(events))
	}
	mu.Unlock()
}

func TestAlertTriggeredForUnhealthyToDegraded(t *testing.T) {
	ha, _ := NewHealthAggregator(DefaultAggregatorConfig())
	defer ha.Stop()

	var events []StatusChangeEvent
	var mu sync.Mutex

	ha.SubscribeAlert(func(event StatusChangeEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
	})

	criticalFailing := true
	ha.RegisterProbe(ProbeConfig{
		Name:     "db",
		Critical: true,
		Probe: func() ProbeResult {
			if criticalFailing {
				return ProbeResult{Healthy: false}
			}
			return ProbeResult{Healthy: true}
		},
	})
	ha.RegisterProbe(ProbeConfig{
		Name:     "monitoring",
		Critical: false,
		Probe:    func() ProbeResult { return ProbeResult{Healthy: false} },
	})

	ha.Check()

	criticalFailing = false
	ha.Check()

	mu.Lock()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].CurrentStatus != StatusUnhealthy {
		t.Errorf("first event: expected unhealthy, got %v", events[0].CurrentStatus)
	}
	if events[1].PreviousStatus != StatusUnhealthy || events[1].CurrentStatus != StatusDegraded {
		t.Errorf("second event: expected unhealthy->degraded, got %v->%v", events[1].PreviousStatus, events[1].CurrentStatus)
	}
	mu.Unlock()
}
