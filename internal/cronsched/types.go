package cronsched

import (
	"context"
	"time"
)

type FieldType int

const (
	FieldSecond FieldType = iota
	FieldMinute
	FieldHour
	FieldDay
	FieldMonth
	FieldWeekday
	FieldYear
)

func (f FieldType) String() string {
	switch f {
	case FieldSecond:
		return "second"
	case FieldMinute:
		return "minute"
	case FieldHour:
		return "hour"
	case FieldDay:
		return "day"
	case FieldMonth:
		return "month"
	case FieldWeekday:
		return "weekday"
	case FieldYear:
		return "year"
	default:
		return "unknown"
	}
}

type ValueType int

const (
	ValueWildcard ValueType = iota
	ValueSingle
	ValueList
	ValueRange
	ValueStep
)

type FieldValue struct {
	Type      ValueType
	Value     int
	Values    []int
	RangeLow  int
	RangeHigh int
	Step      int
}

type CronField struct {
	FieldType FieldType
	Raw       string
	Values    []FieldValue
	Min       int
	Max       int
}

type CronExpression struct {
	Raw       string
	Fields    []*CronField
	Second    *CronField
	Minute    *CronField
	Hour      *CronField
	Day       *CronField
	Month     *CronField
	Weekday   *CronField
	Year      *CronField
	Location  *time.Location
	HasYear   bool
}

type ValidationResult struct {
	Valid       bool
	Description string
	Errors      []error
}

type TaskFunc func(ctx context.Context)

type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusCancelled
	StatusDone
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "PENDING"
	case StatusRunning:
		return "RUNNING"
	case StatusCancelled:
		return "CANCELLED"
	case StatusDone:
		return "DONE"
	default:
		return "UNKNOWN"
	}
}

type CronTask struct {
	ID         string
	CronExpr   *CronExpression
	Func       TaskFunc
	Status     TaskStatus
	NextRun    time.Time
	Location   *time.Location
	CreatedAt  time.Time
	LastRun    time.Time
	RunCount   uint64
}

type SchedulerConfig struct {
	MaxIterations int
}

func DefaultConfig() *SchedulerConfig {
	return &SchedulerConfig{
		MaxIterations: 366 * 24 * 60 * 60,
	}
}
