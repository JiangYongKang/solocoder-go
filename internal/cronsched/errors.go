package cronsched

import "fmt"

var (
	ErrInvalidExpression     = fmt.Errorf("invalid cron expression")
	ErrInvalidFieldCount     = fmt.Errorf("invalid number of fields")
	ErrInvalidFieldValue     = fmt.Errorf("invalid field value")
	ErrInvalidRange          = fmt.Errorf("invalid range")
	ErrInvalidStep           = fmt.Errorf("invalid step")
	ErrValueOutOfRange       = fmt.Errorf("value out of range")
	ErrDayWeekdayMutex       = fmt.Errorf("day and weekday fields are mutually exclusive")
	ErrNoNextTime            = fmt.Errorf("cannot find next execution time within limit")
	ErrSchedulerStopped      = fmt.Errorf("scheduler is stopped")
	ErrTaskNotFound          = fmt.Errorf("task not found")
	ErrTaskAlreadyExists     = fmt.Errorf("task already exists")
	ErrTaskRunning           = fmt.Errorf("task is running and cannot be cancelled immediately")
	ErrInvalidTimezone       = fmt.Errorf("invalid timezone")
)

type ParseError struct {
	Field     FieldType
	Position  int
	RawValue  string
	Message   string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("parse error in %s field at position %d (value: %q): %s",
		e.Field, e.Position, e.RawValue, e.Message)
}

func NewParseError(field FieldType, pos int, raw, format string, args ...interface{}) error {
	return &ParseError{
		Field:    field,
		Position: pos,
		RawValue: raw,
		Message:  fmt.Sprintf(format, args...),
	}
}
