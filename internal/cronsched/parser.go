package cronsched

import (
	"strconv"
	"strings"
	"time"
)

type fieldPos struct {
	value string
	pos   int
}

func fieldsWithPositions(expr string) []fieldPos {
	var result []fieldPos
	i := 0
	for i < len(expr) {
		for i < len(expr) && (expr[i] == ' ' || expr[i] == '\t') {
			i++
		}
		if i >= len(expr) {
			break
		}
		start := i
		for i < len(expr) && expr[i] != ' ' && expr[i] != '\t' {
			i++
		}
		result = append(result, fieldPos{value: expr[start:i], pos: start})
	}
	return result
}

var fieldRanges = map[FieldType][2]int{
	FieldSecond:  {0, 59},
	FieldMinute:  {0, 59},
	FieldHour:    {0, 23},
	FieldDay:     {1, 31},
	FieldMonth:   {1, 12},
	FieldWeekday: {0, 6},
	FieldYear:    {1970, 2100},
}

var weekdayNames = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
	"sun": 0, "mon": 1, "tue": 2, "wed": 3, "thu": 4, "fri": 5, "sat": 6,
}

var monthNames = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	"jan": 1, "feb": 2, "mar": 3, "apr": 4, "may": 5, "jun": 6,
	"jul": 7, "aug": 8, "sep": 9, "oct": 10, "nov": 11, "dec": 12,
}

func Parse(expr string) (*CronExpression, error) {
	return ParseWithLocation(expr, time.UTC)
}

func ParseWithLocation(expr string, loc *time.Location) (*CronExpression, error) {
	if loc == nil {
		return nil, ErrInvalidTimezone
	}

	fp := fieldsWithPositions(expr)
	if len(fp) != 6 && len(fp) != 7 {
		return nil, NewParseError(FieldSecond, 0, expr,
			"expected 6 or 7 fields (second, minute, hour, day, month, weekday[, year])")
	}

	hasYear := len(fp) == 7
	if !hasYear {
		lastEnd := 0
		if len(fp) > 0 {
			last := fp[len(fp)-1]
			lastEnd = last.pos + len(last.value)
		}
		fp = append(fp, fieldPos{value: "*", pos: lastEnd})
	}

	fieldTypes := []FieldType{FieldSecond, FieldMinute, FieldHour, FieldDay, FieldMonth, FieldWeekday, FieldYear}
	cronFields := make([]*CronField, 7)

	for i, f := range fp {
		ft := fieldTypes[i]
		cf, err := parseField(f.value, ft, f.pos)
		if err != nil {
			return nil, err
		}
		cronFields[i] = cf
	}

	daySet := !isWildcard(cronFields[3])
	weekdaySet := !isWildcard(cronFields[5])
	if daySet && weekdaySet {
		return nil, ErrDayWeekdayMutex
	}

	return &CronExpression{
		Raw:      expr,
		Fields:   cronFields,
		Second:   cronFields[0],
		Minute:   cronFields[1],
		Hour:     cronFields[2],
		Day:      cronFields[3],
		Month:    cronFields[4],
		Weekday:  cronFields[5],
		Year:     cronFields[6],
		Location: loc,
		HasYear:  hasYear,
	}, nil
}

func isWildcard(cf *CronField) bool {
	if len(cf.Values) != 1 {
		return false
	}
	return cf.Values[0].Type == ValueWildcard
}

func parseField(raw string, ft FieldType, fieldOffset int) (*CronField, error) {
	minMax, ok := fieldRanges[ft]
	if !ok {
		return nil, NewParseError(ft, fieldOffset, raw, "unknown field type")
	}
	minVal, maxVal := minMax[0], minMax[1]

	if raw == "" {
		return nil, NewParseError(ft, fieldOffset, raw, "empty value")
	}

	cf := &CronField{
		FieldType: ft,
		Raw:       raw,
		Min:       minVal,
		Max:       maxVal,
	}

	parts := strings.Split(raw, ",")
	partOffset := 0
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, NewParseError(ft, fieldOffset+partOffset, raw, "empty part in list")
		}

		idx := strings.Index(raw[partOffset:], part)
		partStart := partOffset
		if idx >= 0 {
			partStart = fieldOffset + partOffset + idx
		} else {
			partStart = fieldOffset + partOffset
		}

		fv, err := parseValuePart(part, ft, minVal, maxVal, partStart)
		if err != nil {
			return nil, err
		}
		cf.Values = append(cf.Values, fv)
		partOffset += len(part) + 1
	}

	return cf, nil
}

func parseValuePart(part string, ft FieldType, minVal, maxVal int, offset int) (FieldValue, error) {
	if part == "*" {
		return FieldValue{Type: ValueWildcard}, nil
	}

	if strings.Contains(part, "/") {
		return parseStep(part, ft, minVal, maxVal, offset)
	}

	if strings.Contains(part, "-") {
		return parseRange(part, ft, minVal, maxVal, offset)
	}

	return parseSingle(part, ft, minVal, maxVal, offset)
}

func parseSingle(part string, ft FieldType, minVal, maxVal int, offset int) (FieldValue, error) {
	val, err := parseNumericValue(part, ft, offset)
	if err != nil {
		return FieldValue{}, err
	}
	if val < minVal || val > maxVal {
		return FieldValue{}, NewParseError(ft, offset, part,
			"value %d out of range [%d, %d]", val, minVal, maxVal)
	}
	return FieldValue{Type: ValueSingle, Value: val}, nil
}

func parseRange(part string, ft FieldType, minVal, maxVal int, offset int) (FieldValue, error) {
	rangeParts := strings.Split(part, "-")
	if len(rangeParts) != 2 {
		return FieldValue{}, NewParseError(ft, offset, part, "invalid range format")
	}

	lowStr := strings.TrimSpace(rangeParts[0])
	highStr := strings.TrimSpace(rangeParts[1])

	if lowStr == "" || highStr == "" {
		return FieldValue{}, NewParseError(ft, offset, part, "empty bound in range")
	}

	low, err := parseNumericValue(lowStr, ft, offset)
	if err != nil {
		return FieldValue{}, err
	}

	highOffset := offset + strings.Index(part, "-") + 1
	high, err := parseNumericValue(highStr, ft, highOffset)
	if err != nil {
		return FieldValue{}, err
	}

	if low < minVal || low > maxVal {
		return FieldValue{}, NewParseError(ft, offset, part,
			"range lower bound %d out of range [%d, %d]", low, minVal, maxVal)
	}

	if high < minVal || high > maxVal {
		return FieldValue{}, NewParseError(ft, offset, part,
			"range upper bound %d out of range [%d, %d]", high, minVal, maxVal)
	}

	if low > high {
		return FieldValue{}, NewParseError(ft, offset, part,
			"range lower bound %d greater than upper bound %d", low, high)
	}

	return FieldValue{
		Type:      ValueRange,
		RangeLow:  low,
		RangeHigh: high,
	}, nil
}

func parseStep(part string, ft FieldType, minVal, maxVal int, offset int) (FieldValue, error) {
	stepParts := strings.Split(part, "/")
	if len(stepParts) > 2 {
		slashCount := strings.Count(part, "/")
		return FieldValue{}, NewParseError(ft, offset, part,
			"invalid step format: found %d '/' separators, expected at most 1", slashCount)
	}
	if len(stepParts) != 2 {
		return FieldValue{}, NewParseError(ft, offset, part, "invalid step format")
	}

	rangePart := strings.TrimSpace(stepParts[0])
	stepStr := strings.TrimSpace(stepParts[1])

	if stepStr == "" {
		return FieldValue{}, NewParseError(ft, offset, part, "empty step value")
	}

	stepOffset := offset + strings.Index(part, "/") + 1
	step, err := strconv.Atoi(stepStr)
	if err != nil {
		return FieldValue{}, NewParseError(ft, stepOffset, part, "invalid step value: %s", stepStr)
	}

	if step <= 0 {
		return FieldValue{}, NewParseError(ft, stepOffset, part, "step value must be positive, got %d", step)
	}

	if rangePart == "*" {
		return FieldValue{
			Type:      ValueStep,
			RangeLow:  minVal,
			RangeHigh: maxVal,
			Step:      step,
		}, nil
	}

	if strings.Contains(rangePart, "-") {
		rangeParts := strings.Split(rangePart, "-")
		if len(rangeParts) != 2 {
			return FieldValue{}, NewParseError(ft, offset, part, "invalid range in step")
		}

		lowStr := strings.TrimSpace(rangeParts[0])
		highStr := strings.TrimSpace(rangeParts[1])

		low, err := parseNumericValue(lowStr, ft, offset)
		if err != nil {
			return FieldValue{}, err
		}

		dashIdx := strings.Index(rangePart, "-")
		highOffset := offset + dashIdx + 1
		high, err := parseNumericValue(highStr, ft, highOffset)
		if err != nil {
			return FieldValue{}, err
		}

		if low < minVal || low > maxVal {
			return FieldValue{}, NewParseError(ft, offset, part,
				"range lower bound %d out of range [%d, %d]", low, minVal, maxVal)
		}

		if high < minVal || high > maxVal {
			return FieldValue{}, NewParseError(ft, offset, part,
				"range upper bound %d out of range [%d, %d]", high, minVal, maxVal)
		}

		if low > high {
			return FieldValue{}, NewParseError(ft, offset, part,
				"range lower bound %d greater than upper bound %d", low, high)
		}

		return FieldValue{
			Type:      ValueStep,
			RangeLow:  low,
			RangeHigh: high,
			Step:      step,
		}, nil
	}

	low, err := parseNumericValue(rangePart, ft, offset)
	if err != nil {
		return FieldValue{}, err
	}

	if low < minVal || low > maxVal {
		return FieldValue{}, NewParseError(ft, offset, part,
			"value %d out of range [%d, %d]", low, minVal, maxVal)
	}

	return FieldValue{
		Type:      ValueStep,
		RangeLow:  low,
		RangeHigh: maxVal,
		Step:      step,
	}, nil
}

func parseNumericValue(s string, ft FieldType, offset int) (int, error) {
	if ft == FieldWeekday {
		if v, ok := weekdayNames[s]; ok {
			return v, nil
		}
	}

	if ft == FieldMonth {
		if v, ok := monthNames[s]; ok {
			return v, nil
		}
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, NewParseError(ft, offset, s, "invalid numeric value: %s", s)
	}
	return v, nil
}

func (cf *CronField) Matches(value int) bool {
	for _, fv := range cf.Values {
		if fv.matches(value) {
			return true
		}
	}
	return false
}

func (fv *FieldValue) matches(value int) bool {
	switch fv.Type {
	case ValueWildcard:
		return true
	case ValueSingle:
		return value == fv.Value
	case ValueRange:
		return value >= fv.RangeLow && value <= fv.RangeHigh
	case ValueStep:
		if value < fv.RangeLow || value > fv.RangeHigh {
			return false
		}
		return (value-fv.RangeLow)%fv.Step == 0
	}
	return false
}

func (cf *CronField) Next(value int) (int, bool) {
	for _, fv := range cf.Values {
		next, ok := fv.next(value)
		if ok {
			return next, true
		}
	}
	return 0, false
}

func (fv *FieldValue) next(value int) (int, bool) {
	switch fv.Type {
	case ValueWildcard:
		return value, true
	case ValueSingle:
		if fv.Value >= value {
			return fv.Value, true
		}
		return 0, false
	case ValueRange:
		if value <= fv.RangeHigh {
			next := value
			if next < fv.RangeLow {
				next = fv.RangeLow
			}
			return next, true
		}
		return 0, false
	case ValueStep:
		if value > fv.RangeHigh {
			return 0, false
		}
		start := fv.RangeLow
		if value > start {
			offset := value - start
			remainder := offset % fv.Step
			if remainder == 0 {
				return value, true
			}
			next := value + (fv.Step - remainder)
			if next <= fv.RangeHigh {
				return next, true
			}
			return 0, false
		}
		return start, true
	}
	return 0, false
}

func (cf *CronField) Has(value int) bool {
	return cf.Matches(value)
}
