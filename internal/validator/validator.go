package validator

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	ErrInvalidValue        = errors.New("invalid value")
	ErrValidatorNotFound   = errors.New("validator not found")
	ErrInvalidRule         = errors.New("invalid validation rule")
	ErrConditionNotMet     = errors.New("condition not met")
	ErrUnsupportedType     = errors.New("unsupported type")
	ErrNonStructValidation = errors.New("non-struct value passed for struct validation")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type ValidationErrors []*ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	msgs := make([]string, len(ve))
	for i, e := range ve {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "; ")
}

func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

func (ve ValidationErrors) FieldErrors(field string) []*ValidationError {
	var result []*ValidationError
	for _, e := range ve {
		if e.Field == field || strings.HasPrefix(e.Field, field+".") || strings.HasPrefix(e.Field, field+"[") {
			result = append(result, e)
		}
	}
	return result
}

type ValidatorFunc func(value interface{}, params string) (bool, string)

type ConditionFunc func(structValue interface{}) bool

type Rule struct {
	Validator     string
	Params        string
	Message       string
	Condition     ConditionFunc
	ConditionName string
}

type FieldRules struct {
	Field string
	Rules []Rule
}

type StructRules struct {
	Fields      map[string][]Rule
	IncludeTags bool
}

type Validator struct {
	mu          sync.RWMutex
	validators  map[string]ValidatorFunc
	conditions  map[string]ConditionFunc
}

var defaultValidator *Validator
var defaultOnce sync.Once

func Default() *Validator {
	defaultOnce.Do(func() {
		defaultValidator = New()
		registerBuiltinValidators(defaultValidator)
	})
	return defaultValidator
}

func New() *Validator {
	return &Validator{
		validators: make(map[string]ValidatorFunc),
		conditions: make(map[string]ConditionFunc),
	}
}

func (v *Validator) RegisterValidator(name string, fn ValidatorFunc) error {
	if name == "" {
		return ErrInvalidRule
	}
	if fn == nil {
		return ErrInvalidRule
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.validators[name] = fn
	return nil
}

func (v *Validator) RegisterCondition(name string, fn ConditionFunc) error {
	if name == "" {
		return ErrInvalidRule
	}
	if fn == nil {
		return ErrInvalidRule
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	v.conditions[name] = fn
	return nil
}

func (v *Validator) getValidator(name string) (ValidatorFunc, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	fn, ok := v.validators[name]
	return fn, ok
}

func (v *Validator) getCondition(name string) (ConditionFunc, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	fn, ok := v.conditions[name]
	return fn, ok
}

func (v *Validator) Validate(s interface{}) ValidationErrors {
	if s == nil {
		return ValidationErrors{{Field: "", Message: "value is nil"}}
	}
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ValidationErrors{{Field: "", Message: "value is nil"}}
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ValidationErrors{{Field: "", Message: ErrNonStructValidation.Error()}}
	}

	errs := make(ValidationErrors, 0)
	v.validateStruct(val, "", s, &errs)
	return errs
}

func (v *Validator) ValidateWithRules(s interface{}, rules StructRules) ValidationErrors {
	if s == nil {
		return ValidationErrors{{Field: "", Message: "value is nil"}}
	}
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ValidationErrors{{Field: "", Message: "value is nil"}}
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return ValidationErrors{{Field: "", Message: ErrNonStructValidation.Error()}}
	}

	errs := make(ValidationErrors, 0)

	for fieldName, fieldRules := range rules.Fields {
		fieldVal, ok := findFieldByName(val, fieldName)
		if !ok {
			continue
		}
		fieldPath := fieldName
		if len(fieldRules) > 0 {
			v.applyRules(fieldVal, fieldPath, fieldRules, s, &errs)
		}
	}

	if rules.IncludeTags {
		v.validateStruct(val, "", s, &errs)
	}

	return errs
}

func (v *Validator) validateStruct(val reflect.Value, path string, structPtr interface{}, errs *ValidationErrors) {
	t := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		fieldVal := val.Field(i)

		fieldPath := field.Name
		if path != "" {
			fieldPath = path + "." + field.Name
		}

		tag := field.Tag.Get("validate")
		if tag != "" && tag != "-" {
			rules := parseTag(tag)
			v.applyRules(fieldVal, fieldPath, rules, structPtr, errs)
		}

		v.validateFieldValue(fieldVal, fieldPath, structPtr, errs)
	}
}

func (v *Validator) validateFieldValue(fieldVal reflect.Value, fieldPath string, structPtr interface{}, errs *ValidationErrors) {
	kind := fieldVal.Kind()

	switch kind {
	case reflect.Ptr:
		if !fieldVal.IsNil() {
			elemVal := fieldVal.Elem()
			if elemVal.Kind() == reflect.Struct {
				v.validateStruct(elemVal, fieldPath, structPtr, errs)
			} else {
				v.validateFieldValue(elemVal, fieldPath, structPtr, errs)
			}
		}
	case reflect.Struct:
		v.validateStruct(fieldVal, fieldPath, structPtr, errs)
	case reflect.Slice, reflect.Array:
		for i := 0; i < fieldVal.Len(); i++ {
			elemPath := fmt.Sprintf("%s[%d]", fieldPath, i)
			elemVal := fieldVal.Index(i)
			v.validateFieldValue(elemVal, elemPath, structPtr, errs)
		}
	case reflect.Map:
		for _, key := range fieldVal.MapKeys() {
			elemPath := fmt.Sprintf("%s[%v]", fieldPath, key.Interface())
			elemVal := fieldVal.MapIndex(key)
			v.validateFieldValue(elemVal, elemPath, structPtr, errs)
		}
	}
}

func (v *Validator) applyRules(fieldVal reflect.Value, fieldPath string, rules []Rule, structPtr interface{}, errs *ValidationErrors) {
	for _, rule := range rules {
		if rule.ConditionName != "" {
			if !v.isRegisteredCondition(rule.ConditionName) {
				fieldName := extractFieldNameFromCondition(rule.ConditionName)
				if !v.structHasField(structPtr, fieldName) {
					*errs = append(*errs, &ValidationError{
						Field:   fieldPath,
						Message: fmt.Sprintf("condition references unknown field '%s'", fieldName),
					})
					continue
				}
			}
		}

		cond := v.resolveCondition(&rule)
		if cond != nil {
			if !cond(structPtr) {
				continue
			}
		}

		fn, ok := v.getValidator(rule.Validator)
		if !ok {
			*errs = append(*errs, &ValidationError{
				Field:   fieldPath,
				Message: fmt.Sprintf("validator '%s' not found", rule.Validator),
			})
			continue
		}

		var value interface{}
		if fieldVal.IsValid() && fieldVal.CanInterface() {
			actualVal := dereferenceValue(fieldVal)
			if actualVal.IsValid() && actualVal.CanInterface() {
				value = actualVal.Interface()
			} else if fieldVal.Kind() == reflect.Ptr && fieldVal.IsNil() {
				value = nil
			} else {
				value = fieldVal.Interface()
			}
		}

		valid, msg := fn(value, rule.Params)
		if !valid {
			if rule.Message != "" {
				msg = rule.Message
			}
			*errs = append(*errs, &ValidationError{
				Field:   fieldPath,
				Message: msg,
			})
		}
	}
}

func (v *Validator) isRegisteredCondition(name string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	_, ok := v.conditions[name]
	return ok
}

type conditionMode int

const (
	conditionSimple conditionMode = iota
	conditionNegate
	conditionEquals
)

type parsedCondition struct {
	mode      conditionMode
	fieldName string
	expected  string
}

func parseCondition(expr string) parsedCondition {
	if strings.HasPrefix(expr, "!") {
		return parsedCondition{
			mode:      conditionNegate,
			fieldName: strings.TrimSpace(strings.TrimPrefix(expr, "!")),
		}
	}
	eqIdx := strings.Index(expr, "=")
	if eqIdx != -1 {
		return parsedCondition{
			mode:      conditionEquals,
			fieldName: strings.TrimSpace(expr[:eqIdx]),
			expected:  strings.TrimSpace(expr[eqIdx+1:]),
		}
	}
	return parsedCondition{
		mode:      conditionSimple,
		fieldName: strings.TrimSpace(expr),
	}
}

func extractFieldNameFromCondition(expr string) string {
	return parseCondition(expr).fieldName
}

func (v *Validator) structHasField(s interface{}, fieldName string) bool {
	if s == nil {
		return false
	}
	val := reflect.ValueOf(s)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return false
		}
		val = val.Elem()
	}
	if val.Kind() != reflect.Struct {
		return false
	}
	_, ok := findFieldByName(val, fieldName)
	return ok
}

func dereferenceValue(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func (v *Validator) resolveCondition(rule *Rule) ConditionFunc {
	if rule.Condition != nil {
		return rule.Condition
	}
	if rule.ConditionName == "" {
		return nil
	}
	if fn, ok := v.getCondition(rule.ConditionName); ok {
		return fn
	}
	return buildCondition(rule.ConditionName)
}

func findFieldByName(val reflect.Value, name string) (reflect.Value, bool) {
	parts := strings.Split(name, ".")
	current := val

	for _, part := range parts {
		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				return reflect.Value{}, false
			}
			current = current.Elem()
		}
		if current.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		field := current.FieldByName(part)
		if !field.IsValid() {
			return reflect.Value{}, false
		}
		current = field
	}
	return current, true
}

func parseTag(tag string) []Rule {
	rules := make([]Rule, 0)
	parts := splitTag(tag)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		rule := Rule{}
		msgIdx := strings.Index(part, "|msg=")
		if msgIdx != -1 {
			rule.Message = part[msgIdx+5:]
			part = part[:msgIdx]
		}

		condIdx := strings.Index(part, "|when=")
		if condIdx != -1 {
			condStr := part[condIdx+6:]
			part = part[:condIdx]
			rule.ConditionName = condStr
		}

		eqIdx := strings.Index(part, "=")
		if eqIdx != -1 {
			rule.Validator = strings.TrimSpace(part[:eqIdx])
			rule.Params = strings.TrimSpace(part[eqIdx+1:])
		} else {
			rule.Validator = strings.TrimSpace(part)
			rule.Params = ""
		}

		if rule.Validator != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

func splitTag(tag string) []string {
	var result []string
	var current strings.Builder
	depth := 0

	for i := 0; i < len(tag); i++ {
		c := tag[i]
		switch c {
		case '(':
			depth++
			current.WriteByte(c)
		case ')':
			depth--
			current.WriteByte(c)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteByte(c)
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func buildCondition(expr string) ConditionFunc {
	parsed := parseCondition(expr)
	return func(s interface{}) bool {
		if s == nil {
			return false
		}
		val := reflect.ValueOf(s)
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return false
			}
			val = val.Elem()
		}
		if val.Kind() != reflect.Struct {
			return false
		}

		switch parsed.mode {
		case conditionNegate:
			fv, ok := findFieldByName(val, parsed.fieldName)
			if !ok {
				return false
			}
			return isEmptyValue(fv)
		case conditionEquals:
			fv, ok := findFieldByName(val, parsed.fieldName)
			if !ok {
				return false
			}
			return fmt.Sprintf("%v", fv.Interface()) == parsed.expected
		default:
			fv, ok := findFieldByName(val, parsed.fieldName)
			if !ok {
				return false
			}
			return !isEmptyValue(fv)
		}
	}
}

func isEmptyValue(v reflect.Value) bool {
	v = dereferenceValue(v)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	}
	return false
}

func registerBuiltinValidators(v *Validator) {
	v.RegisterValidator("required", validateRequired)
	v.RegisterValidator("min", validateMin)
	v.RegisterValidator("max", validateMax)
	v.RegisterValidator("minLen", validateMinLen)
	v.RegisterValidator("maxLen", validateMaxLen)
	v.RegisterValidator("len", validateLen)
	v.RegisterValidator("email", validateEmail)
	v.RegisterValidator("regexp", validateRegexp)
	v.RegisterValidator("enum", validateEnum)
	v.RegisterValidator("numeric", validateNumeric)
	v.RegisterValidator("positive", validatePositive)
	v.RegisterValidator("negative", validateNegative)
	v.RegisterValidator("url", validateURL)
	v.RegisterValidator("ip", validateIP)
}

func validateRequired(value interface{}, params string) (bool, string) {
	if value == nil {
		return false, "field is required"
	}
	v := reflect.ValueOf(value)
	if isEmptyValue(v) {
		return false, "field is required"
	}
	return true, ""
}

func validateMin(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	minVal, err := strconv.ParseFloat(params, 64)
	if err != nil {
		return false, fmt.Sprintf("invalid min value: %s", params)
	}

	switch v := value.(type) {
	case int:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case int8:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case int16:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case int32:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case int64:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case uint:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case uint8:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case uint16:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case uint32:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case uint64:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case float32:
		if float64(v) < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	case float64:
		if v < minVal {
			return false, fmt.Sprintf("value must be at least %v", params)
		}
	default:
		return false, "value must be numeric"
	}
	return true, ""
}

func validateMax(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	maxVal, err := strconv.ParseFloat(params, 64)
	if err != nil {
		return false, fmt.Sprintf("invalid max value: %s", params)
	}

	switch v := value.(type) {
	case int:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case int8:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case int16:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case int32:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case int64:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case uint:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case uint8:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case uint16:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case uint32:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case uint64:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case float32:
		if float64(v) > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	case float64:
		if v > maxVal {
			return false, fmt.Sprintf("value must be at most %v", params)
		}
	default:
		return false, "value must be numeric"
	}
	return true, ""
}

func validateMinLen(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	minLen, err := strconv.Atoi(params)
	if err != nil {
		return false, fmt.Sprintf("invalid min length: %s", params)
	}

	switch v := value.(type) {
	case string:
		if len(v) < minLen {
			return false, fmt.Sprintf("length must be at least %d", minLen)
		}
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			if rv.Len() < minLen {
				return false, fmt.Sprintf("length must be at least %d", minLen)
			}
		default:
			return false, "value must be string, slice, array or map"
		}
	}
	return true, ""
}

func validateMaxLen(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	maxLen, err := strconv.Atoi(params)
	if err != nil {
		return false, fmt.Sprintf("invalid max length: %s", params)
	}

	switch v := value.(type) {
	case string:
		if len(v) > maxLen {
			return false, fmt.Sprintf("length must be at most %d", maxLen)
		}
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			if rv.Len() > maxLen {
				return false, fmt.Sprintf("length must be at most %d", maxLen)
			}
		default:
			return false, "value must be string, slice, array or map"
		}
	}
	return true, ""
}

func validateLen(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	exactLen, err := strconv.Atoi(params)
	if err != nil {
		return false, fmt.Sprintf("invalid length: %s", params)
	}

	switch v := value.(type) {
	case string:
		if len(v) != exactLen {
			return false, fmt.Sprintf("length must be exactly %d", exactLen)
		}
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			if rv.Len() != exactLen {
				return false, fmt.Sprintf("length must be exactly %d", exactLen)
			}
		default:
			return false, "value must be string, slice, array or map"
		}
	}
	return true, ""
}

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

func validateEmail(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	s, ok := value.(string)
	if !ok {
		return false, "value must be a string"
	}
	if s == "" {
		return true, ""
	}
	if !emailRegexp.MatchString(s) {
		return false, "invalid email format"
	}
	return true, ""
}

func validateRegexp(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	s, ok := value.(string)
	if !ok {
		return false, "value must be a string"
	}
	if s == "" {
		return true, ""
	}
	re, err := regexp.Compile(params)
	if err != nil {
		return false, fmt.Sprintf("invalid regexp: %s", params)
	}
	if !re.MatchString(s) {
		return false, fmt.Sprintf("value does not match pattern %s", params)
	}
	return true, ""
}

func validateEnum(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	v := reflect.ValueOf(value)
	if isEmptyValue(v) {
		return true, ""
	}
	values := strings.Split(params, "|")
	valStr := fmt.Sprintf("%v", value)
	for _, ev := range values {
		if strings.TrimSpace(ev) == valStr {
			return true, ""
		}
	}
	return false, fmt.Sprintf("value must be one of [%s]", params)
}

func validateNumeric(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	s, ok := value.(string)
	if !ok {
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true, ""
		}
		return false, "value must be numeric"
	}
	if s == "" {
		return true, ""
	}
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return false, "value must be numeric"
	}
	return true, ""
}

func validatePositive(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	switch v := value.(type) {
	case int:
		if v <= 0 {
			return false, "value must be positive"
		}
	case int8:
		if v <= 0 {
			return false, "value must be positive"
		}
	case int16:
		if v <= 0 {
			return false, "value must be positive"
		}
	case int32:
		if v <= 0 {
			return false, "value must be positive"
		}
	case int64:
		if v <= 0 {
			return false, "value must be positive"
		}
	case uint:
		if v == 0 {
			return false, "value must not be zero"
		}
	case uint8:
		if v == 0 {
			return false, "value must not be zero"
		}
	case uint16:
		if v == 0 {
			return false, "value must not be zero"
		}
	case uint32:
		if v == 0 {
			return false, "value must not be zero"
		}
	case uint64:
		if v == 0 {
			return false, "value must not be zero"
		}
	case uintptr:
		if v == 0 {
			return false, "value must not be zero"
		}
	case float32:
		if v <= 0 {
			return false, "value must be positive"
		}
	case float64:
		if v <= 0 {
			return false, "value must be positive"
		}
	default:
		return false, "value must be numeric"
	}
	return true, ""
}

func validateNegative(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	switch v := value.(type) {
	case int:
		if v >= 0 {
			return false, "value must be negative"
		}
	case int8:
		if v >= 0 {
			return false, "value must be negative"
		}
	case int16:
		if v >= 0 {
			return false, "value must be negative"
		}
	case int32:
		if v >= 0 {
			return false, "value must be negative"
		}
	case int64:
		if v >= 0 {
			return false, "value must be negative"
		}
	case float32:
		if v >= 0 {
			return false, "value must be negative"
		}
	case float64:
		if v >= 0 {
			return false, "value must be negative"
		}
	default:
		return false, "value must be numeric and signed"
	}
	return true, ""
}

var urlRegexp = regexp.MustCompile(`^https?://[^\s/$.?#].[^\s]*$`)

func validateURL(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	s, ok := value.(string)
	if !ok {
		return false, "value must be a string"
	}
	if s == "" {
		return true, ""
	}
	if !urlRegexp.MatchString(s) {
		return false, "invalid URL format"
	}
	return true, ""
}

var ipRegexp = regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

func validateIP(value interface{}, params string) (bool, string) {
	if value == nil {
		return true, ""
	}
	s, ok := value.(string)
	if !ok {
		return false, "value must be a string"
	}
	if s == "" {
		return true, ""
	}
	if !ipRegexp.MatchString(s) {
		return false, "invalid IP address format"
	}
	return true, ""
}

func Validate(s interface{}) ValidationErrors {
	return Default().Validate(s)
}

func ValidateWithRules(s interface{}, rules StructRules) ValidationErrors {
	return Default().ValidateWithRules(s, rules)
}

func RegisterValidator(name string, fn ValidatorFunc) error {
	return Default().RegisterValidator(name, fn)
}

func RegisterCondition(name string, fn ConditionFunc) error {
	return Default().RegisterCondition(name, fn)
}
