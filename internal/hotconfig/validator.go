package hotconfig

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

func ValidateConfig(data map[string]interface{}, schema *Schema) error {
	if schema == nil || len(schema.Fields) == 0 {
		return nil
	}

	var errors []*ValidationError

	for _, field := range schema.Fields {
		value, exists := getNestedValue(data, field.Path)

		if !exists {
			if field.Required {
				errors = append(errors, &ValidationError{
					Field:   field.Path,
					Message: "field is required but missing",
					Err:     ErrFieldRequired,
				})
			}
			continue
		}

		for _, rule := range field.Rules {
			if err := validateRule(field.Path, value, rule); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return &AggregateValidationError{Errors: errors}
	}

	return nil
}

func validateRule(fieldPath string, value interface{}, rule *ValidationRule) *ValidationError {
	switch rule.Type {
	case RuleRequired:
		if isEmpty(value) {
			return &ValidationError{
				Field:   fieldPath,
				Message: "field is required and cannot be empty",
				Err:     ErrFieldRequired,
			}
		}

	case RuleMinValue:
		if cmp, ok := compareNumeric(value, rule.MinValue); ok && cmp < 0 {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("value %v is less than minimum %v", value, rule.MinValue),
				Err:     ErrFieldOutOfRange,
			}
		}

	case RuleMaxValue:
		if cmp, ok := compareNumeric(value, rule.MaxValue); ok && cmp > 0 {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("value %v is greater than maximum %v", value, rule.MaxValue),
				Err:     ErrFieldOutOfRange,
			}
		}

	case RuleMinLength:
		length, ok := getLength(value)
		if !ok {
			return &ValidationError{
				Field:   fieldPath,
				Message: "cannot get length of value for min length validation",
				Err:     ErrFieldTypeMismatch,
			}
		}
		if length < rule.MinLen {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("length %d is less than minimum %d", length, rule.MinLen),
				Err:     ErrFieldOutOfRange,
			}
		}

	case RuleMaxLength:
		length, ok := getLength(value)
		if !ok {
			return &ValidationError{
				Field:   fieldPath,
				Message: "cannot get length of value for max length validation",
				Err:     ErrFieldTypeMismatch,
			}
		}
		if length > rule.MaxLen {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("length %d is greater than maximum %d", length, rule.MaxLen),
				Err:     ErrFieldOutOfRange,
			}
		}

	case RulePattern:
		str, ok := value.(string)
		if !ok {
			return &ValidationError{
				Field:   fieldPath,
				Message: "pattern validation requires string value",
				Err:     ErrFieldTypeMismatch,
			}
		}
		matched, err := regexp.MatchString(rule.Pattern, str)
		if err != nil {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("invalid regex pattern: %v", err),
				Err:     err,
			}
		}
		if !matched {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("value %q does not match pattern %q", str, rule.Pattern),
				Err:     ErrFieldInvalidFormat,
			}
		}

	case RuleEnum:
		if !isEnumValue(value, rule.Enum) {
			return &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("value %v is not in allowed enum %v", value, rule.Enum),
				Err:     ErrFieldOutOfRange,
			}
		}

	case RuleCustom:
		if rule.Custom != nil {
			if err := rule.Custom(value); err != nil {
				return &ValidationError{
					Field:   fieldPath,
					Message: err.Error(),
					Err:     err,
				}
			}
		}
	}

	return nil
}

func getNestedValue(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			val, exists := current[part]
			return val, exists
		}

		next, exists := current[part]
		if !exists {
			return nil, false
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current = nextMap
	}

	return nil, false
}

func setNestedValue(data map[string]interface{}, path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		next, exists := current[part]
		if !exists {
			next = make(map[string]interface{})
			current[part] = next
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			nextMap = make(map[string]interface{})
			current[part] = nextMap
		}
		current = nextMap
	}
}

func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func compareNumeric(a, b interface{}) (int, bool) {
	fa, oka := toFloat64(a)
	fb, okb := toFloat64(b)
	if !oka || !okb {
		return 0, false
	}

	switch {
	case fa < fb:
		return -1, true
	case fa > fb:
		return 1, true
	default:
		return 0, true
	}
}

func getLength(value interface{}) (int, bool) {
	switch v := value.(type) {
	case string:
		return len(v), true
	case []interface{}:
		return len(v), true
	case map[string]interface{}:
		return len(v), true
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map, reflect.String:
			return rv.Len(), true
		default:
			return 0, false
		}
	}
}

func isEnumValue(value interface{}, enum []interface{}) bool {
	for _, e := range enum {
		if reflect.DeepEqual(value, e) {
			return true
		}
	}
	return false
}

func ApplyDefaults(data map[string]interface{}, schema *Schema) map[string]interface{} {
	if schema == nil {
		return data
	}

	result := deepCopyMap(data)

	for _, field := range schema.Fields {
		_, exists := getNestedValue(result, field.Path)
		if !exists && field.DefaultValue != nil {
			setNestedValue(result, field.Path, field.DefaultValue)
		}
	}

	return result
}

func ApplyDefaultsOnValidationFailure(data map[string]interface{}, schema *Schema, validationErr error) map[string]interface{} {
	if schema == nil || validationErr == nil {
		return data
	}

	result := deepCopyMap(data)

	aggErr, ok := validationErr.(*AggregateValidationError)
	if !ok {
		return result
	}

	failedFields := make(map[string]bool)
	for _, ve := range aggErr.Errors {
		failedFields[ve.Field] = true
	}

	for _, field := range schema.Fields {
		if failedFields[field.Path] && field.DefaultValue != nil {
			setNestedValue(result, field.Path, field.DefaultValue)
		}
	}

	return result
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		result[k] = deepCopyValue(v)
	}
	return result
}

func deepCopyValue(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(val)
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, item := range val {
			result[i] = deepCopyValue(item)
		}
		return result
	default:
		return v
	}
}
