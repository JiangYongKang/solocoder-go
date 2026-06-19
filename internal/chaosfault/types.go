package chaosfault

import (
	"math/rand"
	"sync"
	"time"
)

type FaultType int

const (
	FaultTypeDelay FaultType = iota
	FaultTypeError
	FaultTypeDisconnect
)

func (f FaultType) String() string {
	switch f {
	case FaultTypeDelay:
		return "delay"
	case FaultTypeError:
		return "error"
	case FaultTypeDisconnect:
		return "disconnect"
	default:
		return "unknown"
	}
}

type TimeWindow struct {
	StartTime time.Time
	EndTime   time.Time
}

func (tw *TimeWindow) IsActive() bool {
	if tw == nil {
		return true
	}
	now := time.Now()
	if !tw.StartTime.IsZero() && now.Before(tw.StartTime) {
		return false
	}
	if !tw.EndTime.IsZero() && now.After(tw.EndTime) {
		return false
	}
	return true
}

type DelayMode int

const (
	DelayModeFixed DelayMode = iota
	DelayModeRandom
)

type DelayConfig struct {
	Enabled     bool
	Mode        DelayMode
	Fixed       time.Duration
	Min         time.Duration
	Max         time.Duration
	TimeWindow  *TimeWindow
	TargetRatio float64
}

type ErrorConfig struct {
	Enabled     bool
	Err         error
	Message     string
	TimeWindow  *TimeWindow
	TargetRatio float64
}

type DisconnectConfig struct {
	Enabled    bool
	TimeWindow *TimeWindow
}

type FaultInjector struct {
	mu            sync.RWMutex
	delayCfg      DelayConfig
	errorCfg      ErrorConfig
	disconnectCfg DisconnectConfig
	disconnected  bool
	randSrc       *rand.Rand
	sleepFunc     func(time.Duration)
	timeNowFunc   func() time.Time
}

type FaultInjectorOption func(*FaultInjector)
