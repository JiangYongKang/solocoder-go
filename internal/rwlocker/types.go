package rwlocker

import (
	"time"
)

type LockType string

const (
	LockTypeRead  LockType = "read"
	LockTypeWrite LockType = "write"
)

type UpgradeMode int

const (
	UpgradeNonBlocking UpgradeMode = iota
	UpgradeBlocking
)

type Config struct {
	Name                 string
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	EnableDeadlockDetect bool
	EnableStats          bool
	HoldDurationWarn     time.Duration
	OnHoldDurationWarn   func(warning *HoldDurationWarning)
}

func DefaultConfig() *Config {
	return &Config{
		Name:                 "",
		ReadTimeout:          0,
		WriteTimeout:         0,
		EnableDeadlockDetect: true,
		EnableStats:          true,
		HoldDurationWarn:     0,
		OnHoldDurationWarn:   nil,
	}
}

type Stats struct {
	ReadRequests     uint64
	ReadSuccess      uint64
	ReadWaitTotal    time.Duration
	ReadWaitMax      time.Duration
	WriteRequests    uint64
	WriteSuccess     uint64
	WriteWaitTotal   time.Duration
	WriteWaitMax     time.Duration
	UpgradeRequests  uint64
	UpgradeSuccess   uint64
	UpgradeWaitTotal time.Duration
	UpgradeWaitMax   time.Duration
	DeadlockDetected uint64
	TimeoutCount     uint64
}

func (s *Stats) clone() *Stats {
	if s == nil {
		return nil
	}
	return &Stats{
		ReadRequests:     s.ReadRequests,
		ReadSuccess:      s.ReadSuccess,
		ReadWaitTotal:    s.ReadWaitTotal,
		ReadWaitMax:      s.ReadWaitMax,
		WriteRequests:    s.WriteRequests,
		WriteSuccess:     s.WriteSuccess,
		WriteWaitTotal:   s.WriteWaitTotal,
		WriteWaitMax:     s.WriteWaitMax,
		UpgradeRequests:  s.UpgradeRequests,
		UpgradeSuccess:   s.UpgradeSuccess,
		UpgradeWaitTotal: s.UpgradeWaitTotal,
		UpgradeWaitMax:   s.UpgradeWaitMax,
		DeadlockDetected: s.DeadlockDetected,
		TimeoutCount:     s.TimeoutCount,
	}
}

type lockHolder struct {
	goroutineID int64
	lockType    LockType
	acquireTime time.Time
	count       int
}

type goroutineLockInfo struct {
	holders map[*RWLocker]*lockHolder
}
