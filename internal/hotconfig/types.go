package hotconfig

import (
	"time"
)

type ConfigFormat string

const (
	FormatJSON ConfigFormat = "json"
	FormatYAML ConfigFormat = "yaml"
	FormatYML  ConfigFormat = "yml"
	FormatTOML ConfigFormat = "toml"
)

type ValidationRuleType int

const (
	RuleRequired ValidationRuleType = iota
	RuleMinValue
	RuleMaxValue
	RuleMinLength
	RuleMaxLength
	RulePattern
	RuleEnum
	RuleCustom
)

type ValidationRule struct {
	Type     ValidationRuleType
	Field    string
	MinValue interface{}
	MaxValue interface{}
	MinLen   int
	MaxLen   int
	Pattern  string
	Enum     []interface{}
	Custom   func(value interface{}) error
}

type FieldSchema struct {
	Path         string
	Type         string
	Required     bool
	DefaultValue interface{}
	Rules        []*ValidationRule
}

type Schema struct {
	Fields []*FieldSchema
}

type ConfigSnapshot struct {
	Data      map[string]interface{}
	Timestamp time.Time
	Source    string
	Format    ConfigFormat
	Version   uint64
}

type ChangeCallback func(oldSnapshot, newSnapshot *ConfigSnapshot)

type HotConfigOptions struct {
	AutoReload     bool
	DebounceTime   time.Duration
	FailOnError    bool
	UseDefaultOnError bool
}

func DefaultHotConfigOptions() *HotConfigOptions {
	return &HotConfigOptions{
		AutoReload:      true,
		DebounceTime:    100 * time.Millisecond,
		FailOnError:     false,
		UseDefaultOnError: true,
	}
}

type Parser interface {
	Format() ConfigFormat
	Parse(data []byte) (map[string]interface{}, error)
}

type fileEvent struct {
	path string
	time time.Time
}
