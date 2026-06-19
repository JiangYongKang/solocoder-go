package chaosfault

import (
	"math/rand"
	"time"
)

func NewFaultInjector(opts ...FaultInjectorOption) *FaultInjector {
	fi := &FaultInjector{
		randSrc:     rand.New(rand.NewSource(time.Now().UnixNano())),
		sleepFunc:   func(d time.Duration) { time.Sleep(d) },
		timeNowFunc: func() time.Time { return time.Now() },
	}
	for _, opt := range opts {
		opt(fi)
	}
	return fi
}

func WithRandSource(r *rand.Rand) FaultInjectorOption {
	return func(fi *FaultInjector) {
		if r != nil {
			fi.randSrc = r
		}
	}
}

func WithSleepFunc(fn func(time.Duration)) FaultInjectorOption {
	return func(fi *FaultInjector) {
		if fn != nil {
			fi.sleepFunc = fn
		}
	}
}

func WithTimeNowFunc(fn func() time.Time) FaultInjectorOption {
	return func(fi *FaultInjector) {
		if fn != nil {
			fi.timeNowFunc = fn
		}
	}
}

func (fi *FaultInjector) SetDelayConfig(cfg DelayConfig) error {
	if err := validateTargetRatio(cfg.TargetRatio); err != nil {
		return err
	}
	if cfg.TimeWindow != nil {
		if err := validateTimeWindow(cfg.TimeWindow); err != nil {
			return err
		}
	}
	if cfg.Enabled {
		if cfg.Mode == DelayModeFixed && cfg.Fixed <= 0 {
			return wrapError(ErrInvalidConfig, "fixed delay must be positive")
		}
		if cfg.Mode == DelayModeRandom {
			if cfg.Min <= 0 || cfg.Max <= 0 {
				return wrapError(ErrInvalidConfig, "random delay min and max must be positive")
			}
			if cfg.Min >= cfg.Max {
				return wrapError(ErrInvalidConfig, "random delay min must be less than max")
			}
		}
	}

	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.delayCfg = cfg
	return nil
}

func (fi *FaultInjector) SetErrorConfig(cfg ErrorConfig) error {
	if err := validateTargetRatio(cfg.TargetRatio); err != nil {
		return err
	}
	if cfg.TimeWindow != nil {
		if err := validateTimeWindow(cfg.TimeWindow); err != nil {
			return err
		}
	}
	if cfg.Enabled && cfg.Err == nil && cfg.Message == "" {
		return wrapError(ErrInvalidConfig, "error fault requires Err or Message")
	}

	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.errorCfg = cfg
	return nil
}

func (fi *FaultInjector) SetDisconnectConfig(cfg DisconnectConfig) error {
	if cfg.TimeWindow != nil {
		if err := validateTimeWindow(cfg.TimeWindow); err != nil {
			return err
		}
	}

	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.disconnectCfg = cfg
	return nil
}

func (fi *FaultInjector) Disconnect() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.disconnected = true
}

func (fi *FaultInjector) Reconnect() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.disconnected = false
}

func (fi *FaultInjector) IsDisconnected() bool {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.isDisconnectedLocked()
}

func (fi *FaultInjector) isDisconnectedLocked() bool {
	if fi.disconnected {
		return true
	}
	if !fi.disconnectCfg.Enabled {
		return false
	}
	if fi.disconnectCfg.TimeWindow != nil && !fi.isTimeWindowActive(fi.disconnectCfg.TimeWindow) {
		return false
	}
	return true
}

func (fi *FaultInjector) ApplyDelay() {
	var delay time.Duration
	fi.mu.RLock()
	cfg := fi.delayCfg
	if cfg.Enabled {
		if cfg.TimeWindow == nil || fi.isTimeWindowActive(cfg.TimeWindow) {
			if fi.hitTargetRatio(cfg.TargetRatio) {
				delay = fi.calculateDelay(cfg)
			}
		}
	}
	fi.mu.RUnlock()

	if delay > 0 {
		fi.sleepFunc(delay)
	}
}

func (fi *FaultInjector) calculateDelay(cfg DelayConfig) time.Duration {
	switch cfg.Mode {
	case DelayModeFixed:
		return cfg.Fixed
	case DelayModeRandom:
		if cfg.Min >= cfg.Max {
			return cfg.Min
		}
		delta := cfg.Max - cfg.Min
		return cfg.Min + time.Duration(fi.randSrc.Int63n(int64(delta)))
	default:
		return 0
	}
}

func (fi *FaultInjector) CheckError() error {
	var hit bool
	var cfg ErrorConfig
	fi.mu.RLock()
	cfg = fi.errorCfg
	if cfg.Enabled {
		if cfg.TimeWindow == nil || fi.isTimeWindowActive(cfg.TimeWindow) {
			hit = fi.hitTargetRatio(cfg.TargetRatio)
		}
	}
	fi.mu.RUnlock()

	if !hit {
		return nil
	}

	if cfg.Err != nil {
		return &InjectedError{
			Message: cfg.Message,
			Cause:   cfg.Err,
		}
	}
	return &InjectedError{
		Message: cfg.Message,
	}
}

func (fi *FaultInjector) CheckDisconnect() error {
	fi.mu.RLock()
	disconnected := fi.isDisconnectedLocked()
	fi.mu.RUnlock()

	if !disconnected {
		return nil
	}
	return &ConnectionBrokenError{
		Message: "connection is broken due to chaos fault injection",
	}
}

func (fi *FaultInjector) Inject(fn func() error) error {
	if err := fi.CheckDisconnect(); err != nil {
		return err
	}

	if err := fi.CheckError(); err != nil {
		return err
	}

	fi.ApplyDelay()

	return fn()
}

func (fi *FaultInjector) isTimeWindowActive(tw *TimeWindow) bool {
	if tw == nil {
		return true
	}
	now := fi.timeNowFunc()
	if !tw.StartTime.IsZero() && now.Before(tw.StartTime) {
		return false
	}
	if !tw.EndTime.IsZero() && now.After(tw.EndTime) {
		return false
	}
	return true
}

func (fi *FaultInjector) hitTargetRatio(ratio float64) bool {
	if ratio <= 0 {
		return false
	}
	if ratio >= 1.0 {
		return true
	}
	return fi.randSrc.Float64() < ratio
}

func validateTargetRatio(ratio float64) error {
	if ratio < 0 || ratio > 1.0 {
		return wrapError(ErrInvalidTargetRatio, "ratio must be between 0 and 1.0")
	}
	return nil
}

func validateTimeWindow(tw *TimeWindow) error {
	if tw == nil {
		return nil
	}
	if !tw.StartTime.IsZero() && !tw.EndTime.IsZero() && tw.StartTime.After(tw.EndTime) {
		return wrapError(ErrInvalidTimeWindow, "start time must be before end time")
	}
	return nil
}

func (fi *FaultInjector) EnableDelay(mode DelayMode, value ...time.Duration) error {
	cfg := DelayConfig{
		Enabled:     true,
		Mode:        mode,
		TargetRatio: 1.0,
	}
	switch mode {
	case DelayModeFixed:
		if len(value) > 0 {
			cfg.Fixed = value[0]
		}
	case DelayModeRandom:
		if len(value) >= 2 {
			cfg.Min = value[0]
			cfg.Max = value[1]
		}
	}
	return fi.SetDelayConfig(cfg)
}

func (fi *FaultInjector) DisableDelay() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.delayCfg.Enabled = false
}

func (fi *FaultInjector) EnableError(err error, message string) error {
	cfg := ErrorConfig{
		Enabled:     true,
		Err:         err,
		Message:     message,
		TargetRatio: 1.0,
	}
	return fi.SetErrorConfig(cfg)
}

func (fi *FaultInjector) DisableError() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.errorCfg.Enabled = false
}

func (fi *FaultInjector) EnableDisconnect() error {
	cfg := DisconnectConfig{
		Enabled: true,
	}
	return fi.SetDisconnectConfig(cfg)
}

func (fi *FaultInjector) DisableDisconnect() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.disconnectCfg.Enabled = false
}

func (fi *FaultInjector) GetDelayConfig() DelayConfig {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.delayCfg
}

func (fi *FaultInjector) GetErrorConfig() ErrorConfig {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.errorCfg
}

func (fi *FaultInjector) GetDisconnectConfig() DisconnectConfig {
	fi.mu.RLock()
	defer fi.mu.RUnlock()
	return fi.disconnectCfg
}

func (fi *FaultInjector) Reset() {
	fi.mu.Lock()
	defer fi.mu.Unlock()
	fi.delayCfg = DelayConfig{}
	fi.errorCfg = ErrorConfig{}
	fi.disconnectCfg = DisconnectConfig{}
	fi.disconnected = false
}
