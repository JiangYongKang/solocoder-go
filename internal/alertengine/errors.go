package alertengine

import "errors"

var (
	ErrRuleNotFound         = errors.New("alertengine: rule not found")
	ErrRuleAlreadyExists    = errors.New("alertengine: rule already exists")
	ErrInvalidRule          = errors.New("alertengine: invalid rule")
	ErrInvalidCondition     = errors.New("alertengine: invalid condition")
	ErrInvalidOperator      = errors.New("alertengine: invalid comparison operator")
	ErrInvalidThreshold     = errors.New("alertengine: invalid threshold")
	ErrInvalidDuration      = errors.New("alertengine: invalid duration configuration")
	ErrInvalidSilentWindow  = errors.New("alertengine: invalid silent window")
	ErrInvalidLevel         = errors.New("alertengine: invalid alert level")
	ErrNotifierNotFound     = errors.New("alertengine: notifier not found")
	ErrInvalidMetricData    = errors.New("alertengine: invalid metric data")
	ErrNoConditionDefined   = errors.New("alertengine: no condition defined in rule")
)
