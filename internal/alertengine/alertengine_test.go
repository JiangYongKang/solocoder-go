package alertengine

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	e := NewEngine(EngineConfig{})
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.cfg.DefaultInhibitDuration != 5*time.Minute {
		t.Errorf("Expected default inhibit duration 5m, got %v", e.cfg.DefaultInhibitDuration)
	}
}

func TestAddRule(t *testing.T) {
	e := NewEngine(EngineConfig{})

	rule := &AlertRule{
		ID:           "rule-1",
		Name:         "Test Alert",
		MetricName:   "cpu_usage",
		InitialLevel: LevelWarning,
		Threshold: &ThresholdCondition{
			Operator:  OpGreaterThan,
			Threshold: 80,
		},
	}

	err := e.AddRule(rule)
	if err != nil {
		t.Fatalf("AddRule failed: %v", err)
	}

	err = e.AddRule(rule)
	if err != ErrRuleAlreadyExists {
		t.Errorf("Expected ErrRuleAlreadyExists, got %v", err)
	}

	err = e.AddRule(nil)
	if err != ErrInvalidRule {
		t.Errorf("Expected ErrInvalidRule, got %v", err)
	}

	emptyRule := &AlertRule{ID: "empty", Name: "empty"}
	err = e.AddRule(emptyRule)
	if err != ErrNoConditionDefined {
		t.Errorf("Expected ErrNoConditionDefined, got %v", err)
	}
}

func TestAddRuleValidation(t *testing.T) {
	e := NewEngine(EngineConfig{})

	t.Run("invalid initial level", func(t *testing.T) {
		rule := &AlertRule{
			ID:           "inv-level",
			Name:         "Invalid Level",
			InitialLevel: AlertLevel("unknown"),
			Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidLevel {
			t.Errorf("Expected ErrInvalidLevel, got %v", err)
		}
	})

	t.Run("invalid threshold operator", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-op",
			Name: "Invalid Operator",
			Threshold: &ThresholdCondition{
				Operator:  ComparisonOperator("!="),
				Threshold: 80,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidOperator {
			t.Errorf("Expected ErrInvalidOperator, got %v", err)
		}
	})

	t.Run("invalid ringbi compare type", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-compare",
			Name: "Invalid Compare Type",
			RingbiTongbi: &RingbiTongbiCondition{
				CompareType:      CompareType("unknown"),
				PercentThreshold: 10,
				Period:           time.Minute,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidCondition {
			t.Errorf("Expected ErrInvalidCondition, got %v", err)
		}
	})

	t.Run("negative percent threshold", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "neg-pct",
			Name: "Negative Percent",
			RingbiTongbi: &RingbiTongbiCondition{
				CompareType:      CompareRingbi,
				PercentThreshold: -5,
				Period:           time.Minute,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidThreshold {
			t.Errorf("Expected ErrInvalidThreshold, got %v", err)
		}
	})

	t.Run("zero period", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "zero-period",
			Name: "Zero Period",
			RingbiTongbi: &RingbiTongbiCondition{
				CompareType:      CompareRingbi,
				PercentThreshold: 10,
				Period:           0,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidDuration {
			t.Errorf("Expected ErrInvalidDuration, got %v", err)
		}
	})

	t.Run("invalid duration type", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-dur-type",
			Name: "Invalid Duration Type",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			Duration: &DurationCondition{
				Type:       DurationType("unknown"),
				CheckCount: 3,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidDuration {
			t.Errorf("Expected ErrInvalidDuration, got %v", err)
		}
	})

	t.Run("zero check count", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "zero-count",
			Name: "Zero Check Count",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			Duration: &DurationCondition{
				Type:       DurationByCount,
				CheckCount: 0,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidDuration {
			t.Errorf("Expected ErrInvalidDuration, got %v", err)
		}
	})

	t.Run("zero time window", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "zero-window",
			Name: "Zero Time Window",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			Duration: &DurationCondition{
				Type:       DurationByTime,
				TimeWindow: 0,
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidDuration {
			t.Errorf("Expected ErrInvalidDuration, got %v", err)
		}
	})

	t.Run("invalid silent window type", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-silent-type",
			Name: "Invalid Silent Type",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{Type: SilentType("unknown")},
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidSilentWindow {
			t.Errorf("Expected ErrInvalidSilentWindow, got %v", err)
		}
	})

	t.Run("invalid silent daily time format", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-silent-time",
			Name: "Invalid Silent Time",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{Type: SilentDaily, StartTime: "abc", EndTime: "25:00"},
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidSilentWindow {
			t.Errorf("Expected ErrInvalidSilentWindow, got %v", err)
		}
	})

	t.Run("invalid silent range dates", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-silent-range",
			Name: "Invalid Silent Range",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{
					Type:      SilentRange,
					StartDate: time.Now().Add(24 * time.Hour),
					EndDate:   time.Now(),
				},
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidSilentWindow {
			t.Errorf("Expected ErrInvalidSilentWindow, got %v", err)
		}
	})

	t.Run("invalid escalation level", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "inv-esc-level",
			Name: "Invalid Escalation Level",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			Escalations: []EscalationRule{
				{
					AfterDuration: time.Minute,
					FromLevel:     AlertLevel("unknown"),
					ToLevel:       LevelCritical,
				},
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidLevel {
			t.Errorf("Expected ErrInvalidLevel, got %v", err)
		}
	})

	t.Run("zero escalation duration", func(t *testing.T) {
		rule := &AlertRule{
			ID:   "zero-esc-dur",
			Name: "Zero Escalation Duration",
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			Escalations: []EscalationRule{
				{
					AfterDuration: 0,
					FromLevel:     LevelAlert,
					ToLevel:       LevelCritical,
				},
			},
		}
		err := e.AddRule(rule)
		if err != ErrInvalidDuration {
			t.Errorf("Expected ErrInvalidDuration, got %v", err)
		}
	})
}

func TestRegisterNotifierValidation(t *testing.T) {
	e := NewEngine(EngineConfig{})

	err := e.RegisterNotifier(nil)
	if err == nil {
		t.Error("Expected error for nil notifier")
	}

	emptyNameCb := NewCallbackNotifier("", nil)
	err = e.RegisterNotifier(emptyNameCb)
	if err == nil {
		t.Error("Expected error for empty name notifier")
	}

	validCb := NewCallbackNotifier("valid", nil)
	err = e.RegisterNotifier(validCb)
	if err != nil {
		t.Errorf("Expected no error for valid notifier, got %v", err)
	}
}

func TestRemoveRule(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:         "rule-1",
		Name:       "Test",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 10},
	}
	e.AddRule(rule)

	err := e.RemoveRule("rule-1")
	if err != nil {
		t.Fatalf("RemoveRule failed: %v", err)
	}

	err = e.RemoveRule("rule-1")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestGetRule(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:         "rule-1",
		Name:       "Test",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 10},
	}
	e.AddRule(rule)

	r, err := e.GetRule("rule-1")
	if err != nil {
		t.Fatalf("GetRule failed: %v", err)
	}
	if r.ID != "rule-1" {
		t.Errorf("Expected rule ID rule-1, got %s", r.ID)
	}

	_, err = e.GetRule("nonexistent")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestThresholdGreaterThan(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "thresh-gt",
		Name:         "CPU High",
		MetricName:   "cpu",
		InitialLevel: LevelAlert,
		Threshold: &ThresholdCondition{
			Operator:  OpGreaterThan,
			Threshold: 80,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	dp := MetricDataPoint{Timestamp: time.Now(), Value: 85}
	err := e.Evaluate("thresh-gt", dp)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	state, _ := e.GetAlertState("thresh-gt")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring, got %s", state.Status)
	}
	if state.TriggerValue != 85 {
		t.Errorf("Expected trigger value 85, got %.2f", state.TriggerValue)
	}
	if len(console.Notifications()) != 1 {
		t.Errorf("Expected 1 notification, got %d", len(console.Notifications()))
	}
}

func TestThresholdLessThan(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "thresh-lt",
		Name:         "Memory Low",
		MetricName:   "memory",
		InitialLevel: LevelWarning,
		Threshold: &ThresholdCondition{
			Operator:  OpLessThan,
			Threshold: 20,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	dp := MetricDataPoint{Timestamp: time.Now(), Value: 15}
	e.Evaluate("thresh-lt", dp)

	state, _ := e.GetAlertState("thresh-lt")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring, got %s", state.Status)
	}
}

func TestThresholdGreaterThanOrEqual(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "thresh-gte",
		Name:       "Test GTE",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThanOrEqual, Threshold: 100},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	dp := MetricDataPoint{Timestamp: time.Now(), Value: 100}
	e.Evaluate("thresh-gte", dp)

	state, _ := e.GetAlertState("thresh-gte")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring for equal value with GTE operator, got %s", state.Status)
	}
}

func TestThresholdLessThanOrEqual(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "thresh-lte",
		Name:       "Test LTE",
		Threshold:  &ThresholdCondition{Operator: OpLessThanOrEqual, Threshold: 50},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	dp := MetricDataPoint{Timestamp: time.Now(), Value: 50}
	e.Evaluate("thresh-lte", dp)

	state, _ := e.GetAlertState("thresh-lte")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring for equal value with LTE operator, got %s", state.Status)
	}
}

func TestThresholdNotMet(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "thresh-nomet",
		Name:       "Test No Met",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 100},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	dp := MetricDataPoint{Timestamp: time.Now(), Value: 50}
	e.Evaluate("thresh-nomet", dp)

	state, _ := e.GetAlertState("thresh-nomet")
	if state.Status == StatusFiring {
		t.Error("Expected alert not to fire when threshold not met")
	}
	if len(console.Notifications()) != 0 {
		t.Errorf("Expected 0 notifications, got %d", len(console.Notifications()))
	}
}

func TestInvalidOperator(t *testing.T) {
	cond := &ThresholdCondition{Operator: ComparisonOperator("invalid"), Threshold: 10}
	dp := MetricDataPoint{Value: 20, Timestamp: time.Now()}
	_, _, err := evaluateThreshold(cond, dp)
	if err != ErrInvalidOperator {
		t.Errorf("Expected ErrInvalidOperator, got %v", err)
	}
}

func TestAlertResolve(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "resolve-test",
		Name:       "Resolve Test",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("resolve-test", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	state, _ := e.GetAlertState("resolve-test")
	if state.Status != StatusFiring {
		t.Fatalf("Expected StatusFiring, got %s", state.Status)
	}

	e.Evaluate("resolve-test", MetricDataPoint{Timestamp: time.Now(), Value: 70})
	state, _ = e.GetAlertState("resolve-test")
	if state.Status != StatusResolved {
		t.Errorf("Expected StatusResolved, got %s", state.Status)
	}
}

func TestDurationByCount(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "dur-count",
		Name:       "Duration Count Test",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Duration:   &DurationCondition{Type: DurationByCount, CheckCount: 3},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	for i := 0; i < 2; i++ {
		e.Evaluate("dur-count", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	}
	state, _ := e.GetAlertState("dur-count")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire before meeting check count")
	}

	e.Evaluate("dur-count", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	state, _ = e.GetAlertState("dur-count")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring after 3 checks, got %s", state.Status)
	}
}

func TestDurationByCountWithReset(t *testing.T) {
	e := NewEngine(EngineConfig{})

	rule := &AlertRule{
		ID:         "dur-count-reset",
		Name:       "Duration Count Reset",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Duration:   &DurationCondition{Type: DurationByCount, CheckCount: 3},
	}
	e.AddRule(rule)

	e.Evaluate("dur-count-reset", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	e.Evaluate("dur-count-reset", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	e.Evaluate("dur-count-reset", MetricDataPoint{Timestamp: time.Now(), Value: 70})
	e.Evaluate("dur-count-reset", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	e.Evaluate("dur-count-reset", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	state, _ := e.GetAlertState("dur-count-reset")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire - consecutive count was reset by non-matching value")
	}
}

func TestDurationByTime(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:         "dur-time",
		Name:       "Duration Time Test",
		Threshold:  &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Duration:   &DurationCondition{Type: DurationByTime, TimeWindow: 100 * time.Millisecond},
		Notifiers:  []string{"console"},
	}
	e.AddRule(rule)

	start := time.Now()
	e.Evaluate("dur-time", MetricDataPoint{Timestamp: start, Value: 90})

	state, _ := e.GetAlertState("dur-time")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire immediately with time-based duration")
	}

	time.Sleep(150 * time.Millisecond)
	e.Evaluate("dur-time", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	state, _ = e.GetAlertState("dur-time")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring after time window, got %s", state.Status)
	}
}

func TestRingbiAlert(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "ringbi-test",
		Name:         "Ringbi Test",
		InitialLevel: LevelWarning,
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareRingbi,
			PercentThreshold: 20,
			Period:           10 * time.Minute,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	now := time.Now()
	tenMinutesAgo := now.Add(-10 * time.Minute)

	state, _ := e.GetAlertState("ringbi-test")
	state.HistoryValues = append(state.HistoryValues, MetricDataPoint{
		Timestamp: tenMinutesAgo,
		Value:     100,
	})

	e.Evaluate("ringbi-test", MetricDataPoint{Timestamp: now, Value: 130})
	state, _ = e.GetAlertState("ringbi-test")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring for 30%% increase, got %s", state.Status)
	}
}

func TestRingbiAlertBelowThreshold(t *testing.T) {
	e := NewEngine(EngineConfig{})

	rule := &AlertRule{
		ID:           "ringbi-below",
		Name:         "Ringbi Below Threshold",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareRingbi,
			PercentThreshold: 50,
			Period:           10 * time.Minute,
		},
	}
	e.AddRule(rule)

	now := time.Now()
	tenMinutesAgo := now.Add(-10 * time.Minute)

	state, _ := e.GetAlertState("ringbi-below")
	state.HistoryValues = append(state.HistoryValues, MetricDataPoint{
		Timestamp: tenMinutesAgo,
		Value:     100,
	})

	e.Evaluate("ringbi-below", MetricDataPoint{Timestamp: now, Value: 120})
	state, _ = e.GetAlertState("ringbi-below")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire for 20% increase with 50% threshold")
	}
}

func TestTongbiAlert(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "tongbi-test",
		Name:         "Tongbi Test",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareTongbi,
			PercentThreshold: 30,
			Period:           time.Hour,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	now := time.Now()
	lastYear := now.AddDate(-1, 0, 0).Add(6 * time.Hour)

	state, _ := e.GetAlertState("tongbi-test")
	state.HistoryValues = append(state.HistoryValues, MetricDataPoint{
		Timestamp: lastYear,
		Value:     100,
	})

	e.Evaluate("tongbi-test", MetricDataPoint{Timestamp: now, Value: 150})
	state, _ = e.GetAlertState("tongbi-test")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring for tongbi comparison, got %s", state.Status)
	}
}

func TestTongbiDefaultTolerance(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:   "tongbi-tol",
		Name: "Tongbi Tolerance Test",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareTongbi,
			PercentThreshold: 30,
			Period:           time.Hour,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	now := time.Now()
	lastYearPlus12Hours := now.AddDate(-1, 0, 0).Add(12 * time.Hour)

	state, _ := e.GetAlertState("tongbi-tol")
	state.HistoryValues = append(state.HistoryValues, MetricDataPoint{
		Timestamp: lastYearPlus12Hours,
		Value:     100,
	})

	e.Evaluate("tongbi-tol", MetricDataPoint{Timestamp: now, Value: 150})
	state, _ = e.GetAlertState("tongbi-tol")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring with 12h offset within 24h tolerance, got %s", state.Status)
	}
}

func TestTongbiCustomTolerance(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:   "tongbi-custom-tol",
		Name: "Tongbi Custom Tolerance Test",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareTongbi,
			PercentThreshold: 30,
			Period:           time.Hour,
			Tolerance:        1 * time.Hour,
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	now := time.Now()
	lastYearPlus30Min := now.AddDate(-1, 0, 0).Add(30 * time.Minute)
	lastYearPlus2Hours := now.AddDate(-1, 0, 0).Add(2 * time.Hour)

	state, _ := e.GetAlertState("tongbi-custom-tol")
	state.HistoryValues = append(state.HistoryValues,
		MetricDataPoint{Timestamp: lastYearPlus2Hours, Value: 100},
	)

	e.Evaluate("tongbi-custom-tol", MetricDataPoint{Timestamp: now, Value: 150})
	state, _ = e.GetAlertState("tongbi-custom-tol")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire when data point outside custom tolerance")
	}

	state.HistoryValues = append(state.HistoryValues,
		MetricDataPoint{Timestamp: lastYearPlus30Min, Value: 100},
	)
	e.Evaluate("tongbi-custom-tol", MetricDataPoint{Timestamp: now, Value: 150})
	state, _ = e.GetAlertState("tongbi-custom-tol")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring with data point within custom tolerance, got %s", state.Status)
	}
}

func TestInhibitDuration(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:              "inhibit-test",
		Name:            "Inhibit Test",
		Threshold:       &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		InhibitDuration: 100 * time.Millisecond,
		Notifiers:       []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("inhibit-test", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	if len(console.Notifications()) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(console.Notifications()))
	}

	e.Evaluate("inhibit-test", MetricDataPoint{Timestamp: time.Now(), Value: 95})
	if len(console.Notifications()) != 1 {
		t.Errorf("Expected still 1 notification during inhibit period, got %d", len(console.Notifications()))
	}

	time.Sleep(150 * time.Millisecond)
	e.Evaluate("inhibit-test", MetricDataPoint{Timestamp: time.Now(), Value: 92})
	if len(console.Notifications()) != 2 {
		t.Errorf("Expected 2 notifications after inhibit period, got %d", len(console.Notifications()))
	}
}

func TestSilentDailyWindow(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	now := time.Now()
	startTime := now.Add(-1 * time.Hour).Format("15:04")
	endTime := now.Add(1 * time.Hour).Format("15:04")

	rule := &AlertRule{
		ID:        "silent-daily",
		Name:      "Silent Daily Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		SilentWindows: []SilentWindow{
			{
				Type:      SilentDaily,
				StartTime: startTime,
				EndTime:   endTime,
			},
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("silent-daily", MetricDataPoint{Timestamp: now, Value: 90})
	state, _ := e.GetAlertState("silent-daily")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring (rule still evaluates), got %s", state.Status)
	}
	if len(console.Notifications()) != 0 {
		t.Errorf("Expected 0 notifications during silent period, got %d", len(console.Notifications()))
	}
}

func TestSilentWindowWithTags(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	now := time.Now()
	startTime := now.Add(-1 * time.Hour).Format("15:04")
	endTime := now.Add(1 * time.Hour).Format("15:04")

	t.Run("tag matched - silent", func(t *testing.T) {
		console.Clear()
		rule := &AlertRule{
			ID:    "silent-tag-match",
			Name:  "Silent Tag Match",
			Tags:  []string{"production", "critical"},
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{
					Type:      SilentDaily,
					StartTime: startTime,
					EndTime:   endTime,
					Tags:      []string{"production"},
				},
			},
			Notifiers: []string{"console"},
		}
		e.AddRule(rule)

		e.Evaluate("silent-tag-match", MetricDataPoint{Timestamp: now, Value: 90})
		if len(console.Notifications()) != 0 {
			t.Errorf("Expected 0 notifications when tag matches silent window, got %d", len(console.Notifications()))
		}
	})

	t.Run("tag not matched - not silent", func(t *testing.T) {
		console.Clear()
		rule := &AlertRule{
			ID:   "silent-tag-nomatch",
			Name: "Silent Tag No Match",
			Tags: []string{"staging"},
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{
					Type:      SilentDaily,
					StartTime: startTime,
					EndTime:   endTime,
					Tags:      []string{"production"},
				},
			},
			Notifiers: []string{"console"},
		}
		e.AddRule(rule)

		e.Evaluate("silent-tag-nomatch", MetricDataPoint{Timestamp: now, Value: 90})
		if len(console.Notifications()) != 1 {
			t.Errorf("Expected 1 notification when tag does not match silent window, got %d", len(console.Notifications()))
		}
	})

	t.Run("no tags on silent window - always apply", func(t *testing.T) {
		console.Clear()
		rule := &AlertRule{
			ID:   "silent-notags",
			Name: "Silent No Tags",
			Tags: []string{"any-tag"},
			Threshold: &ThresholdCondition{
				Operator:  OpGreaterThan,
				Threshold: 80,
			},
			SilentWindows: []SilentWindow{
				{
					Type:      SilentDaily,
					StartTime: startTime,
					EndTime:   endTime,
					Tags:      nil,
				},
			},
			Notifiers: []string{"console"},
		}
		e.AddRule(rule)

		e.Evaluate("silent-notags", MetricDataPoint{Timestamp: now, Value: 90})
		if len(console.Notifications()) != 0 {
			t.Errorf("Expected 0 notifications when silent window has no tags, got %d", len(console.Notifications()))
		}
	})
}

func TestSilentDailyWindowOvernight(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:        "silent-overnight",
		Name:      "Silent Overnight Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		SilentWindows: []SilentWindow{
			{
				Type:      SilentDaily,
				StartTime: "22:00",
				EndTime:   "06:00",
			},
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	nightTime := time.Date(2025, 1, 15, 23, 30, 0, 0, time.Local)
	morningTime := time.Date(2025, 1, 15, 3, 0, 0, 0, time.Local)

	tests := []struct {
		name string
		t    time.Time
	}{
		{"late night", nightTime},
		{"early morning", morningTime},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			console.Clear()
			state, _ := e.GetAlertState("silent-overnight")
			state.Status = StatusPending
			state.FirstFiredTime = time.Time{}
			state.LastNotifiedTime = time.Time{}
			state.FirstTriggeredTime = time.Time{}

			e.Evaluate("silent-overnight", MetricDataPoint{Timestamp: tt.t, Value: 90})
			if len(console.Notifications()) != 0 {
				t.Errorf("Expected 0 notifications at %v, got %d", tt.t, len(console.Notifications()))
			}
		})
	}
}

func TestSilentRangeWindow(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	now := time.Now()

	rule := &AlertRule{
		ID:        "silent-range",
		Name:      "Silent Range Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		SilentWindows: []SilentWindow{
			{
				Type:      SilentRange,
				StartDate: now.Add(-1 * time.Hour),
				EndDate:   now.Add(1 * time.Hour),
			},
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("silent-range", MetricDataPoint{Timestamp: now, Value: 90})
	if len(console.Notifications()) != 0 {
		t.Errorf("Expected 0 notifications during silent range, got %d", len(console.Notifications()))
	}
}

func TestSilentRangeOutsideWindow(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	now := time.Now()

	rule := &AlertRule{
		ID:        "silent-range-out",
		Name:      "Silent Range Outside Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		SilentWindows: []SilentWindow{
			{
				Type:      SilentRange,
				StartDate: now.Add(-24 * time.Hour),
				EndDate:   now.Add(-23 * time.Hour),
			},
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("silent-range-out", MetricDataPoint{Timestamp: now, Value: 90})
	if len(console.Notifications()) != 1 {
		t.Errorf("Expected 1 notification outside silent range, got %d", len(console.Notifications()))
	}
}

func TestEscalation(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "escalation-test",
		Name:         "Escalation Test",
		InitialLevel: LevelAlert,
		Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Escalations: []EscalationRule{
			{
				AfterDuration: 50 * time.Millisecond,
				FromLevel:     LevelAlert,
				ToLevel:       LevelCritical,
			},
		},
		InhibitDuration: 0,
		Notifiers:       []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("escalation-test", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	state, _ := e.GetAlertState("escalation-test")
	if state.CurrentLevel != LevelAlert {
		t.Errorf("Expected initial level %s, got %s", LevelAlert, state.CurrentLevel)
	}
	if state.FirstTriggeredTime.IsZero() {
		t.Error("Expected FirstTriggeredTime to be set")
	}

	time.Sleep(100 * time.Millisecond)
	e.Evaluate("escalation-test", MetricDataPoint{Timestamp: time.Now(), Value: 95})
	state, _ = e.GetAlertState("escalation-test")
	if state.CurrentLevel != LevelCritical {
		t.Errorf("Expected escalated level %s, got %s", LevelCritical, state.CurrentLevel)
	}
}

func TestEscalationStartFromTriggerTime(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:           "escalation-trigger-time",
		Name:         "Escalation Trigger Time Test",
		InitialLevel: LevelWarning,
		Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Duration:     &DurationCondition{Type: DurationByCount, CheckCount: 3},
		Escalations: []EscalationRule{
			{
				AfterDuration: 100 * time.Millisecond,
				FromLevel:     LevelWarning,
				ToLevel:       LevelAlert,
			},
		},
		InhibitDuration: 0,
		Notifiers:       []string{"console"},
	}
	e.AddRule(rule)

	for i := 0; i < 2; i++ {
		e.Evaluate("escalation-trigger-time", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	}

	state, _ := e.GetAlertState("escalation-trigger-time")
	if !state.FirstFiredTime.IsZero() && state.FirstTriggeredTime.IsZero() {
		t.Log("FirstFiredTime is set before trigger, FirstTriggeredTime is zero - correct behavior")
	}

	time.Sleep(150 * time.Millisecond)

	e.Evaluate("escalation-trigger-time", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	state, _ = e.GetAlertState("escalation-trigger-time")

	if state.Status != StatusFiring {
		t.Fatalf("Expected StatusFiring, got %s", state.Status)
	}

	if state.CurrentLevel != LevelWarning {
		t.Errorf("Expected level %s right after trigger, got %s (escalation should start from trigger time, not first condition hit)", LevelWarning, state.CurrentLevel)
	}
}

func TestLevelNotifiers(t *testing.T) {
	console := NewConsoleNotifier()
	emailCb := NewCallbackNotifier("email", func(n Notification) error { return nil })
	smsCb := NewCallbackNotifier("sms", func(n Notification) error { return nil })

	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{
			"console": console,
			"email":   emailCb,
			"sms":     smsCb,
		},
	})

	rule := &AlertRule{
		ID:           "level-notifiers",
		Name:         "Level Notifiers Test",
		InitialLevel: LevelWarning,
		Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Escalations: []EscalationRule{
			{
				AfterDuration: 50 * time.Millisecond,
				FromLevel:     LevelWarning,
				ToLevel:       LevelCritical,
			},
		},
		InhibitDuration: 0,
		LevelNotifiers: map[AlertLevel][]string{
			LevelWarning:  {"console"},
			LevelCritical: {"console", "email", "sms"},
		},
	}
	e.AddRule(rule)

	t.Run("warning level uses console only", func(t *testing.T) {
		console.Clear()
		emailCb.Clear()
		smsCb.Clear()

		e.Evaluate("level-notifiers", MetricDataPoint{Timestamp: time.Now(), Value: 90})

		if len(console.Notifications()) != 1 {
			t.Errorf("Expected 1 console notification at warning level, got %d", len(console.Notifications()))
		}
		if len(emailCb.Notifications()) != 0 {
			t.Errorf("Expected 0 email notifications at warning level, got %d", len(emailCb.Notifications()))
		}
		if len(smsCb.Notifications()) != 0 {
			t.Errorf("Expected 0 sms notifications at warning level, got %d", len(smsCb.Notifications()))
		}
	})

	t.Run("critical level uses all channels", func(t *testing.T) {
		console.Clear()
		emailCb.Clear()
		smsCb.Clear()

		time.Sleep(100 * time.Millisecond)
		e.Evaluate("level-notifiers", MetricDataPoint{Timestamp: time.Now(), Value: 95})

		state, _ := e.GetAlertState("level-notifiers")
		if state.CurrentLevel != LevelCritical {
			t.Fatalf("Expected critical level, got %s", state.CurrentLevel)
		}

		if len(console.Notifications()) != 1 {
			t.Errorf("Expected 1 console notification at critical level, got %d", len(console.Notifications()))
		}
		if len(emailCb.Notifications()) != 1 {
			t.Errorf("Expected 1 email notification at critical level, got %d", len(emailCb.Notifications()))
		}
		if len(smsCb.Notifications()) != 1 {
			t.Errorf("Expected 1 sms notification at critical level, got %d", len(smsCb.Notifications()))
		}
	})
}

func TestLevelNotifiersFallback(t *testing.T) {
	console := NewConsoleNotifier()
	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{"console": console},
	})

	rule := &AlertRule{
		ID:           "level-notifiers-fallback",
		Name:         "Level Notifiers Fallback Test",
		InitialLevel: LevelWarning,
		Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		LevelNotifiers: map[AlertLevel][]string{
			LevelCritical: {"console"},
		},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	e.Evaluate("level-notifiers-fallback", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	if len(console.Notifications()) != 1 {
		t.Errorf("Expected 1 notification using fallback Notifiers, got %d", len(console.Notifications()))
	}
}

func TestCallbackNotifier(t *testing.T) {
	var mu sync.Mutex
	var received []Notification
	callback := func(n Notification) error {
		mu.Lock()
		defer mu.Unlock()
		received = append(received, n)
		return nil
	}

	cb := NewCallbackNotifier("custom", callback)
	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{
			"custom": cb,
		},
	})

	rule := &AlertRule{
		ID:        "callback-test",
		Name:      "Callback Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Notifiers: []string{"custom"},
	}
	e.AddRule(rule)

	e.Evaluate("callback-test", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	mu.Lock()
	if len(received) != 1 {
		t.Errorf("Expected 1 callback notification, got %d", len(received))
	}
	if received[0].AlertName != "Callback Test" {
		t.Errorf("Expected alert name 'Callback Test', got '%s'", received[0].AlertName)
	}
	mu.Unlock()
}

func TestCallbackNotifierError(t *testing.T) {
	cb := NewCallbackNotifier("error-cb", func(n Notification) error {
		return errors.New("send failed")
	})
	e := NewEngine(EngineConfig{})
	e.RegisterNotifier(cb)

	rule := &AlertRule{
		ID:        "callback-error",
		Name:      "Callback Error Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Notifiers: []string{"error-cb"},
	}
	e.AddRule(rule)

	err := e.Evaluate("callback-error", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	if err != nil {
		t.Errorf("Evaluate should not return error even if callback fails: %v", err)
	}
}

func TestMultipleNotifiers(t *testing.T) {
	console := NewConsoleNotifier()
	cb := NewCallbackNotifier("cb1", func(n Notification) error { return nil })

	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{
			"console": console,
			"cb1":     cb,
		},
	})

	rule := &AlertRule{
		ID:        "multi-notifier",
		Name:      "Multi Notifier Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Notifiers: []string{"console", "cb1"},
	}
	e.AddRule(rule)

	e.Evaluate("multi-notifier", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	if len(console.Notifications()) != 1 {
		t.Errorf("Expected 1 console notification, got %d", len(console.Notifications()))
	}
	if len(cb.Notifications()) != 1 {
		t.Errorf("Expected 1 callback notification, got %d", len(cb.Notifications()))
	}
}

func TestDefaultNotifiersWhenNoneSpecified(t *testing.T) {
	console := NewConsoleNotifier()
	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{
			"console": console,
		},
	})

	rule := &AlertRule{
		ID:        "default-notifier",
		Name:      "Default Notifier Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
	}
	e.AddRule(rule)

	e.Evaluate("default-notifier", MetricDataPoint{Timestamp: time.Now(), Value: 90})

	if len(console.Notifications()) != 1 {
		t.Errorf("Expected 1 notification using default notifier, got %d", len(console.Notifications()))
	}
}

func TestNotificationContent(t *testing.T) {
	console := NewConsoleNotifier()
	e := NewEngine(EngineConfig{
		Notifiers: map[string]Notifier{"console": console},
	})

	rule := &AlertRule{
		ID:           "content-test",
		Name:         "Content Test Alert",
		MetricName:   "test_metric",
		InitialLevel: LevelWarning,
		Labels:       map[string]string{"env": "prod", "service": "api"},
		Threshold:    &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
		Notifiers:    []string{"console"},
	}
	e.AddRule(rule)

	testTime := time.Now()
	e.Evaluate("content-test", MetricDataPoint{Timestamp: testTime, Value: 95})

	notifs := console.Notifications()
	if len(notifs) != 1 {
		t.Fatalf("Expected 1 notification, got %d", len(notifs))
	}

	n := notifs[0]
	if n.RuleID != "content-test" {
		t.Errorf("Expected RuleID 'content-test', got '%s'", n.RuleID)
	}
	if n.AlertName != "Content Test Alert" {
		t.Errorf("Expected AlertName 'Content Test Alert', got '%s'", n.AlertName)
	}
	if n.TriggerValue != 95 {
		t.Errorf("Expected TriggerValue 95, got %.2f", n.TriggerValue)
	}
	if n.CurrentLevel != LevelWarning {
		t.Errorf("Expected CurrentLevel %s, got %s", LevelWarning, n.CurrentLevel)
	}
	if n.Labels["env"] != "prod" || n.Labels["service"] != "api" {
		t.Errorf("Expected labels env=prod and service=api, got %v", n.Labels)
	}
	if n.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestEvaluateNonexistentRule(t *testing.T) {
	e := NewEngine(EngineConfig{})
	err := e.Evaluate("nonexistent", MetricDataPoint{Value: 100, Timestamp: time.Now()})
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestEvaluateInvalidMetricData(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:        "invalid-data",
		Name:      "Invalid Data Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
	}
	e.AddRule(rule)

	err := e.Evaluate("invalid-data", MetricDataPoint{Value: 100})
	if err != ErrInvalidMetricData {
		t.Errorf("Expected ErrInvalidMetricData, got %v", err)
	}
}

func TestDefaultInitialLevel(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:        "default-level",
		Name:      "Default Level Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 80},
	}
	e.AddRule(rule)

	e.Evaluate("default-level", MetricDataPoint{Timestamp: time.Now(), Value: 90})
	state, _ := e.GetAlertState("default-level")
	if state.CurrentLevel != LevelAlert {
		t.Errorf("Expected default level %s, got %s", LevelAlert, state.CurrentLevel)
	}
}

func TestConsoleNotifierClear(t *testing.T) {
	c := NewConsoleNotifier()
	c.notifications = append(c.notifications, Notification{AlertName: "test"})
	if len(c.Notifications()) != 1 {
		t.Fatal("Expected 1 notification before clear")
	}
	c.Clear()
	if len(c.Notifications()) != 0 {
		t.Errorf("Expected 0 notifications after clear, got %d", len(c.Notifications()))
	}
}

func TestCallbackNotifierName(t *testing.T) {
	cb := NewCallbackNotifier("my-callback", nil)
	if cb.Name() != "my-callback" {
		t.Errorf("Expected name 'my-callback', got '%s'", cb.Name())
	}
}

func TestConsoleNotifierName(t *testing.T) {
	c := NewConsoleNotifier()
	if c.Name() != "console" {
		t.Errorf("Expected name 'console', got '%s'", c.Name())
	}
}

func TestInvalidSilentWindowParse(t *testing.T) {
	_, _, err := parseTimeStr("invalid")
	if err != ErrInvalidSilentWindow {
		t.Errorf("Expected ErrInvalidSilentWindow, got %v", err)
	}

	_, _, err = parseTimeStr("25:00")
	if err != ErrInvalidSilentWindow {
		t.Errorf("Expected ErrInvalidSilentWindow for hour 25, got %v", err)
	}

	_, _, err = parseTimeStr("12:60")
	if err != ErrInvalidSilentWindow {
		t.Errorf("Expected ErrInvalidSilentWindow for minute 60, got %v", err)
	}
}

func TestParseTimeStrSuccess(t *testing.T) {
	h, m, err := parseTimeStr("14:30")
	if err != nil {
		t.Fatalf("parseTimeStr failed: %v", err)
	}
	if h != 14 || m != 30 {
		t.Errorf("Expected 14:30, got %d:%d", h, m)
	}
}

func TestEvaluateNilThreshold(t *testing.T) {
	_, _, err := evaluateThreshold(nil, MetricDataPoint{})
	if err != ErrInvalidCondition {
		t.Errorf("Expected ErrInvalidCondition, got %v", err)
	}
}

func TestEvaluateNilRingbiTongbi(t *testing.T) {
	_, _, err := evaluateRingbiTongbi(nil, MetricDataPoint{}, nil)
	if err != ErrInvalidCondition {
		t.Errorf("Expected ErrInvalidCondition, got %v", err)
	}
}

func TestRingbiNoHistoryData(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:   "ringbi-nohistory",
		Name: "Ringbi No History",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareRingbi,
			PercentThreshold: 10,
			Period:           10 * time.Minute,
		},
	}
	e.AddRule(rule)

	e.Evaluate("ringbi-nohistory", MetricDataPoint{Timestamp: time.Now(), Value: 100})
	state, _ := e.GetAlertState("ringbi-nohistory")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire when no history data available")
	}
}

func TestRingbiZeroCompareValue(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:   "ringbi-zerovalue",
		Name: "Ringbi Zero Value",
		RingbiTongbi: &RingbiTongbiCondition{
			CompareType:      CompareRingbi,
			PercentThreshold: 10,
			Period:           10 * time.Minute,
		},
	}
	e.AddRule(rule)

	now := time.Now()
	tenMinutesAgo := now.Add(-10 * time.Minute)
	state, _ := e.GetAlertState("ringbi-zerovalue")
	state.HistoryValues = append(state.HistoryValues, MetricDataPoint{
		Timestamp: tenMinutesAgo,
		Value:     0,
	})

	e.Evaluate("ringbi-zerovalue", MetricDataPoint{Timestamp: now, Value: 100})
	state, _ = e.GetAlertState("ringbi-zerovalue")
	if state.Status == StatusFiring {
		t.Error("Alert should not fire when compare value is zero")
	}
}

func TestInvalidRingbiThreshold(t *testing.T) {
	cond := &RingbiTongbiCondition{
		CompareType:      CompareRingbi,
		PercentThreshold: -1,
		Period:           time.Minute,
	}
	_, _, err := evaluateRingbiTongbi(cond, MetricDataPoint{}, nil)
	if err != ErrInvalidThreshold {
		t.Errorf("Expected ErrInvalidThreshold, got %v", err)
	}
}

func TestDurationNilCondition(t *testing.T) {
	result := evaluateDuration(nil, true, &AlertState{}, MetricDataPoint{}, time.Now())
	if !result {
		t.Error("Expected true for nil duration condition")
	}
}

func TestGetAlertStateNotFound(t *testing.T) {
	e := NewEngine(EngineConfig{})
	_, err := e.GetAlertState("nonexistent")
	if err != ErrRuleNotFound {
		t.Errorf("Expected ErrRuleNotFound, got %v", err)
	}
}

func TestConcurrentEvaluation(t *testing.T) {
	e := NewEngine(EngineConfig{})
	console := NewConsoleNotifier()
	e.RegisterNotifier(console)

	rule := &AlertRule{
		ID:        "concurrent-test",
		Name:      "Concurrent Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 50},
		Notifiers: []string{"console"},
	}
	e.AddRule(rule)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(val float64) {
			defer wg.Done()
			e.Evaluate("concurrent-test", MetricDataPoint{
				Timestamp: time.Now(),
				Value:     val,
			})
		}(float64(60 + i))
	}
	wg.Wait()

	state, _ := e.GetAlertState("concurrent-test")
	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring after concurrent evaluations, got %s", state.Status)
	}

	if state.ConsecutiveHits <= 0 {
		t.Errorf("Expected ConsecutiveHits > 0, got %d", state.ConsecutiveHits)
	}
}

func TestConcurrentStateConsistency(t *testing.T) {
	e := NewEngine(EngineConfig{})

	rule := &AlertRule{
		ID:        "concurrent-consistency",
		Name:      "Concurrent Consistency Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 50},
	}
	e.AddRule(rule)

	var wg sync.WaitGroup
	iterations := 100

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			val := float64(60 + i%10)
			e.Evaluate("concurrent-consistency", MetricDataPoint{
				Timestamp: time.Now(),
				Value:     val,
			})
		}(i)
	}
	wg.Wait()

	state, err := e.GetAlertState("concurrent-consistency")
	if err != nil {
		t.Fatalf("GetAlertState failed: %v", err)
	}

	if state.Status != StatusFiring {
		t.Errorf("Expected StatusFiring, got %s", state.Status)
	}

	if len(state.HistoryValues) != iterations {
		t.Errorf("Expected %d history values, got %d", iterations, len(state.HistoryValues))
	}
}

func TestHistoryValuesTrim(t *testing.T) {
	e := NewEngine(EngineConfig{})
	rule := &AlertRule{
		ID:        "history-trim",
		Name:      "History Trim Test",
		Threshold: &ThresholdCondition{Operator: OpGreaterThan, Threshold: 1000},
	}
	e.AddRule(rule)

	for i := 0; i < maxHistorySize+100; i++ {
		e.Evaluate("history-trim", MetricDataPoint{
			Timestamp: time.Now(),
			Value:     float64(i),
		})
	}

	state, _ := e.GetAlertState("history-trim")
	if len(state.HistoryValues) != maxHistorySize {
		t.Errorf("Expected history size capped at %d, got %d", maxHistorySize, len(state.HistoryValues))
	}
}
