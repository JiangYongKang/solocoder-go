package alertengine

import "time"

type AlertLevel string

const (
	LevelInfo     AlertLevel = "info"
	LevelWarning  AlertLevel = "warning"
	LevelAlert    AlertLevel = "alert"
	LevelCritical AlertLevel = "critical"
)

type AlertStatus string

const (
	StatusPending   AlertStatus = "pending"
	StatusFiring    AlertStatus = "firing"
	StatusSuppressed AlertStatus = "suppressed"
	StatusResolved  AlertStatus = "resolved"
)

type ComparisonOperator string

const (
	OpGreaterThan        ComparisonOperator = ">"
	OpLessThan           ComparisonOperator = "<"
	OpGreaterThanOrEqual ComparisonOperator = ">="
	OpLessThanOrEqual    ComparisonOperator = "<="
)

type CompareType string

const (
	CompareRingbi CompareType = "ringbi"
	CompareTongbi CompareType = "tongbi"
)

type DurationType string

const (
	DurationByCount DurationType = "count"
	DurationByTime  DurationType = "time"
)

type SilentType string

const (
	SilentDaily  SilentType = "daily"
	SilentRange  SilentType = "range"
)

type MetricDataPoint struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

type ThresholdCondition struct {
	Operator ComparisonOperator
	Threshold float64
}

type RingbiTongbiCondition struct {
	CompareType    CompareType
	PercentThreshold float64
	Period         time.Duration
}

type DurationCondition struct {
	Type          DurationType
	CheckCount    int
	TimeWindow    time.Duration
}

type SilentWindow struct {
	Type       SilentType
	StartTime  string
	EndTime    string
	StartDate  time.Time
	EndDate    time.Time
	Tags       []string
}

type EscalationRule struct {
	AfterDuration time.Duration
	FromLevel     AlertLevel
	ToLevel       AlertLevel
}

type Notification struct {
	RuleID       string
	AlertName    string
	TriggerValue float64
	TriggerTime  time.Time
	CurrentLevel AlertLevel
	Labels       map[string]string
	Message      string
}

type Notifier interface {
	Send(notification Notification) error
	Name() string
}

type AlertState struct {
	RuleID        string
	Status        AlertStatus
	CurrentLevel  AlertLevel
	TriggerValue  float64
	TriggerTime   time.Time
	LastFiredTime time.Time
	FirstFiredTime time.Time
	ConsecutiveHits int
	HistoryValues   []MetricDataPoint
	LastEvaluatedTime time.Time
	LastNotifiedTime  time.Time
	ResolvedTime      time.Time
}

type RuleState struct {
	Alert *AlertState
}

type AlertRule struct {
	ID                string
	Name              string
	MetricName        string
	Labels            map[string]string
	Tags              []string
	InitialLevel      AlertLevel
	Threshold         *ThresholdCondition
	RingbiTongbi      *RingbiTongbiCondition
	Duration          *DurationCondition
	InhibitDuration   time.Duration
	SilentWindows     []SilentWindow
	Escalations       []EscalationRule
	Notifiers         []string
}

type EngineConfig struct {
	DefaultInhibitDuration time.Duration
	Notifiers              map[string]Notifier
}
