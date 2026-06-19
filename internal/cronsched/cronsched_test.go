package cronsched

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ------------------------------ Parser Tests ------------------------------

func TestParse_Basic(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{"six fields wildcard", "* * * * * *", false},
		{"seven fields wildcard", "* * * * * * *", false},
		{"single values", "0 30 14 1 6 *", false},
		{"with year", "0 30 14 1 6 * 2025", false},
		{"step", "*/15 * * * * *", false},
		{"range", "0 9-17 * * * *", false},
		{"list", "0 0,30 * * * *", false},
		{"range with step", "0 10-20/2 * * * *", false},
		{"complex", "0,30 9-17/2 * * 1-5 *", false},
		{"month names", "0 0 1 * JAN *", false},
		{"weekday names", "0 0 * * * MON-FRI", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := Parse(tt.expr)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.expr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.expr, err)
			}
			if !tt.wantErr && expr == nil {
				t.Errorf("expected non-nil expression for %q", tt.expr)
			}
		})
	}
}

func TestParse_InvalidFieldCount(t *testing.T) {
	tests := []string{
		"",
		"*",
		"* *",
		"* * *",
		"* * * *",
		"* * * * *",
		"* * * * * * * *",
	}

	for _, expr := range tests {
		t.Run(fmt.Sprintf("fields-%d", len(expr)), func(t *testing.T) {
			_, err := Parse(expr)
			if err == nil {
				t.Errorf("expected error for %q", expr)
			}
			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Errorf("expected ParseError, got %T", err)
			}
		})
	}
}

func TestParse_InvalidValues(t *testing.T) {
	tests := []struct {
		expr string
		ft   FieldType
	}{
		{"60 * * * * *", FieldSecond},
		{"* 60 * * * *", FieldMinute},
		{"* * 24 * * *", FieldHour},
		{"* * * 32 * *", FieldDay},
		{"* * * * 13 *", FieldMonth},
		{"* * * * * 7", FieldWeekday},
		{"* * * * * * 1969", FieldYear},
		{"a * * * * *", FieldSecond},
		{"* */0 * * * *", FieldMinute},
		{"* 5-3 * * * *", FieldMinute},
		{"* 10-60 * * * *", FieldMinute},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			_, err := Parse(tt.expr)
			if err == nil {
				t.Errorf("expected error for %q", tt.expr)
			}
			var perr *ParseError
			if !errors.As(err, &perr) {
				t.Errorf("expected ParseError, got %T: %v", err, err)
			} else if perr.Field != tt.ft {
				t.Errorf("expected field %v, got %v", tt.ft, perr.Field)
			}
		})
	}
}

func TestParse_DayWeekdayMutex(t *testing.T) {
	_, err := Parse("0 0 * 1 * 1")
	if !errors.Is(err, ErrDayWeekdayMutex) {
		t.Errorf("expected ErrDayWeekdayMutex, got %v", err)
	}

	_, err = Parse("0 0 * * * 1")
	if err != nil {
		t.Errorf("expected no error when only weekday set, got %v", err)
	}

	_, err = Parse("0 0 * 1 * *")
	if err != nil {
		t.Errorf("expected no error when only day set, got %v", err)
	}
}

func TestParse_InvalidTimezone(t *testing.T) {
	_, err := ParseWithLocation("* * * * * *", nil)
	if !errors.Is(err, ErrInvalidTimezone) {
		t.Errorf("expected ErrInvalidTimezone, got %v", err)
	}
}

// ------------------------------ Field Matching Tests ------------------------------

func TestField_Matches(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		ft     FieldType
		value  int
		want   bool
	}{
		{"wildcard", "*", FieldSecond, 30, true},
		{"single match", "30", FieldSecond, 30, true},
		{"single no match", "30", FieldSecond, 31, false},
		{"range match low", "10-20", FieldSecond, 10, true},
		{"range match high", "10-20", FieldSecond, 20, true},
		{"range match mid", "10-20", FieldSecond, 15, true},
		{"range no match below", "10-20", FieldSecond, 9, false},
		{"range no match above", "10-20", FieldSecond, 21, false},
		{"step wildcard match", "*/10", FieldSecond, 0, true},
		{"step wildcard match 10", "*/10", FieldSecond, 10, true},
		{"step wildcard match 50", "*/10", FieldSecond, 50, true},
		{"step wildcard no match", "*/10", FieldSecond, 5, false},
		{"step range match", "10-30/5", FieldSecond, 10, true},
		{"step range match 15", "10-30/5", FieldSecond, 15, true},
		{"step range match 30", "10-30/5", FieldSecond, 30, true},
		{"step range no match below", "10-30/5", FieldSecond, 5, false},
		{"step range no match above", "10-30/5", FieldSecond, 35, false},
		{"step range no match remainder", "10-30/5", FieldSecond, 12, false},
		{"list match first", "10,20,30", FieldSecond, 10, true},
		{"list match mid", "10,20,30", FieldSecond, 20, true},
		{"list match last", "10,20,30", FieldSecond, 30, true},
		{"list no match", "10,20,30", FieldSecond, 15, false},
		{"mixed list range", "10,20-25,30", FieldSecond, 22, true},
		{"mixed list step", "10,*/15,30", FieldSecond, 15, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cf, err := parseField(tt.raw, tt.ft)
			if err != nil {
				t.Fatalf("parseField failed: %v", err)
			}
			if got := cf.Matches(tt.value); got != tt.want {
				t.Errorf("cf.Matches(%d) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

// ------------------------------ NextTime Tests ------------------------------

func TestNextTime_Basic(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		expr string
		want time.Time
	}{
		{"0 35 10 * * *", time.Date(2025, 6, 15, 10, 35, 0, 0, time.UTC)},
		{"0 0 11 * * *", time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
		{"0 0 0 * * *", time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)},
		{"0 0 10 * * *", time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)},
		{"30 0 10 * * *", time.Date(2025, 6, 16, 10, 0, 30, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			got, err := NextTime(expr, from)
			if err != nil {
				t.Fatalf("NextTime failed: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("NextTime(%q, %v) = %v, want %v", tt.expr, from, got, tt.want)
			}
		})
	}
}

func TestNextTime_StepAndRange(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 7, 0, 0, time.UTC)

	expr, err := Parse("*/15 * * * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 15, 10, 7, 15, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("step cron: got %v, want %v", got, want)
	}

	from2 := time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC)
	expr2, err := Parse("0 0 9-17 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got2, err := NextTime(expr2, from2)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want2 := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	if !got2.Equal(want2) {
		t.Errorf("range cron: got %v, want %v", got2, want2)
	}
}

func TestNextTime_LeapYear(t *testing.T) {
	from := time.Date(2024, 2, 28, 10, 0, 0, 0, time.UTC)

	expr, err := Parse("0 0 10 29 2 *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("leap year feb 29: got %v, want %v", got, want)
	}

	from2 := time.Date(2025, 2, 28, 10, 0, 0, 0, time.UTC)
	got2, err := NextTime(expr, from2)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want2 := time.Date(2028, 2, 29, 10, 0, 0, 0, time.UTC)
	if !got2.Equal(want2) {
		t.Errorf("non-leap year feb 29 should jump to next leap year: got %v, want %v", got2, want2)
	}
}

func TestNextTime_MonthDays(t *testing.T) {
	from := time.Date(2025, 1, 31, 10, 0, 0, 0, time.UTC)

	expr, err := Parse("0 0 10 31 * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 3, 31, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("month with 31 days: got %v, want %v", got, want)
	}
}

func TestNextTime_Weekday(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	expr, err := Parse("0 0 10 * * 1")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("weekday 1 (Monday): got %v, want %v", got, want)
	}
}

func TestNextTime_YearField(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	expr, err := Parse("0 0 10 1 1 * 2026")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("year field: got %v, want %v", got, want)
	}
}

func TestNextTime_NoNextTime(t *testing.T) {
	expr, err := Parse("0 0 10 1 1 * 2020")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err = NextTime(expr, from)
	if !errors.Is(err, ErrNoNextTime) {
		t.Errorf("expected ErrNoNextTime, got %v", err)
	}
}

func TestNextTimes_Multiple(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	expr, err := Parse("0 0 * * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	times, err := NextTimes(expr, from, 5)
	if err != nil {
		t.Fatalf("NextTimes failed: %v", err)
	}
	if len(times) != 5 {
		t.Fatalf("expected 5 times, got %d", len(times))
	}

	for i, tt := range times {
		expected := time.Date(2025, 6, 15, 11+i, 0, 0, 0, time.UTC)
		if !tt.Equal(expected) {
			t.Errorf("time[%d] = %v, want %v", i, tt, expected)
		}
	}
}

func TestNextTime_SecondPrecision(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 30, 45, 0, time.UTC)

	expr, err := Parse("*/10 * * * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 15, 10, 30, 50, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("second precision: got %v, want %v", got, want)
	}
}

// ------------------------------ Timezone Tests ------------------------------

func TestParseWithLocation(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	expr, err := ParseWithLocation("0 0 9 * * *", loc)
	if err != nil {
		t.Fatalf("ParseWithLocation failed: %v", err)
	}
	if expr.Location != loc {
		t.Errorf("expected location %v, got %v", loc, expr.Location)
	}
}

func TestNextTime_Timezone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	expr, err := ParseWithLocation("0 0 9 * * *", loc)
	if err != nil {
		t.Fatalf("ParseWithLocation failed: %v", err)
	}

	from := time.Date(2025, 6, 15, 8, 0, 0, 0, loc)
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 15, 9, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("timezone: got %v, want %v", got, want)
	}

	utcGot := got.UTC()
	utcWant := time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC)
	if !utcGot.Equal(utcWant) {
		t.Errorf("UTC conversion: got %v, want %v", utcGot, utcWant)
	}
}

func TestNextTime_DST_SpringForward(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	expr, err := ParseWithLocation("0 30 2 * * *", loc)
	if err != nil {
		t.Fatalf("ParseWithLocation failed: %v", err)
	}

	from := time.Date(2025, 3, 8, 0, 0, 0, 0, loc)
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}

	if got.Before(from) {
		t.Errorf("DST spring forward: got %v before %v", got, from)
	}

	utc := got.UTC()
	if utc.IsZero() {
		t.Errorf("UTC conversion failed")
	}
}

func TestNextTime_DST_FallBack(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone not available: %v", err)
	}

	expr, err := ParseWithLocation("0 30 1 * * *", loc)
	if err != nil {
		t.Fatalf("ParseWithLocation failed: %v", err)
	}

	from := time.Date(2025, 11, 2, 0, 0, 0, 0, loc)
	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}

	expected := time.Date(2025, 11, 2, 1, 30, 0, 0, loc)
	if !got.Equal(expected) {
		t.Errorf("DST fall back: got %v, want %v", got, expected)
	}
}

// ------------------------------ Validate Tests ------------------------------

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		want    bool
		wantDesc bool
	}{
		{"valid all wildcard", "* * * * * *", true, true},
		{"valid specific time", "0 30 14 * * *", true, true},
		{"valid with year", "0 0 1 1 1 * 2030", true, true},
		{"invalid field count", "* * * * *", false, false},
		{"invalid value", "60 * * * * *", false, false},
		{"invalid range", "* 5-3 * * * *", false, false},
		{"day weekday mutex", "0 0 * 1 * 1", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Validate(tt.expr)
			if result.Valid != tt.want {
				t.Errorf("Validate(%q).Valid = %v, want %v", tt.expr, result.Valid, tt.want)
			}
			if tt.wantDesc && result.Description == "" {
				t.Errorf("Validate(%q).Description is empty", tt.expr)
			}
			if result.Valid && len(result.Errors) > 0 {
				t.Errorf("Validate(%q) has errors despite being valid: %v", tt.expr, result.Errors)
			}
			if !result.Valid && len(result.Errors) == 0 {
				t.Errorf("Validate(%q) has no errors despite being invalid", tt.expr)
			}
		})
	}
}

// ------------------------------ Description Tests ------------------------------

func TestGenerateDescription(t *testing.T) {
	tests := []struct {
		expr     string
		contains []string
	}{
		{"* * * * * *", []string{"每秒执行"}},
		{"0 0 2 * * *", []string{"2时", "0分", "0秒"}},
		{"0 0 2 * * 1-5", []string{"2时", "周一", "周五"}},
		{"0 0 2 1 * *", []string{"2时", "每月的", "1日"}},
		{"0 0 2 1 1 *", []string{"2时", "1日", "一月"}},
		{"*/10 * * * * *", []string{"每10秒"}},
		{"0 */5 * * * *", []string{"每5分"}},
		{"0 0 9-17 * * *", []string{"9-17时"}},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			expr, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			desc := GenerateDescription(expr)
			if desc == "" {
				t.Errorf("GenerateDescription returned empty string")
			}
			for _, contain := range tt.contains {
				if !strings.Contains(desc, contain) {
					t.Errorf("description %q should contain %q", desc, contain)
				}
			}
		})
	}
}

// ------------------------------ Matches Tests ------------------------------

func TestExpression_Matches(t *testing.T) {
	expr, err := Parse("0 30 14 15 6 *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	tests := []struct {
		t    time.Time
		want bool
	}{
		{time.Date(2025, 6, 15, 14, 30, 0, 0, time.UTC), true},
		{time.Date(2025, 6, 15, 14, 30, 1, 0, time.UTC), false},
		{time.Date(2025, 6, 15, 14, 31, 0, 0, time.UTC), false},
		{time.Date(2025, 6, 15, 15, 30, 0, 0, time.UTC), false},
		{time.Date(2025, 6, 16, 14, 30, 0, 0, time.UTC), false},
		{time.Date(2025, 7, 15, 14, 30, 0, 0, time.UTC), false},
	}

	for _, tt := range tests {
		t.Run(tt.t.String(), func(t *testing.T) {
			if got := expr.Matches(tt.t); got != tt.want {
				t.Errorf("expr.Matches(%v) = %v, want %v", tt.t, got, tt.want)
			}
		})
	}
}

// ------------------------------ Scheduler Tests ------------------------------

func TestNewScheduler(t *testing.T) {
	s := NewScheduler()
	if s == nil {
		t.Fatal("NewScheduler returned nil")
	}
	if s.TaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", s.TaskCount())
	}
}

func TestScheduler_AddAndExecute(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var mu sync.Mutex
	executed := make(map[string]int)

	var wg sync.WaitGroup
	wg.Add(3)

	fn := func(id string) TaskFunc {
		return func(_ context.Context) {
			mu.Lock()
			executed[id]++
			mu.Unlock()
			wg.Done()
		}
	}

	startTime := time.Now().UTC().Add(500 * time.Millisecond)
	t1 := startTime
	t2 := startTime.Add(1 * time.Second)
	t3 := startTime.Add(2 * time.Second)

	expr1 := fmt.Sprintf("%d %d %d %d %d *", t1.Second(), t1.Minute(), t1.Hour(), t1.Day(), int(t1.Month()))
	expr2 := fmt.Sprintf("%d %d %d %d %d *", t2.Second(), t2.Minute(), t2.Hour(), t2.Day(), int(t2.Month()))
	expr3 := fmt.Sprintf("%d %d %d %d %d *", t3.Second(), t3.Minute(), t3.Hour(), t3.Day(), int(t3.Month()))

	err := s.AddWithID("t1", expr1, fn("t1"))
	if err != nil {
		t.Fatalf("failed to add t1: %v", err)
	}

	err = s.AddWithID("t2", expr2, fn("t2"))
	if err != nil {
		t.Fatalf("failed to add t2: %v", err)
	}

	err = s.AddWithID("t3", expr3, fn("t3"))
	if err != nil {
		t.Fatalf("failed to add t3: %v", err)
	}

	if s.TaskCount() != 3 {
		t.Errorf("expected 3 tasks, got %d", s.TaskCount())
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for tasks to execute")
	}

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"t1", "t2", "t3"} {
		if executed[id] != 1 {
			t.Errorf("task %s executed %d times, expected 1", id, executed[id])
		}
	}
}

func TestScheduler_AddWithLocation(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)

	loc := time.UTC
	id, err := s.AddWithLocation("* * * * * *", loc, func(_ context.Context) {
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("AddWithLocation failed: %v", err)
	}
	if id == "" {
		t.Error("AddWithLocation returned empty ID")
	}

	task, err := s.GetTask(id)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.Location != loc {
		t.Errorf("expected location %v, got %v", loc, task.Location)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("task did not execute in time")
	}
}

func TestScheduler_Add_AutoID(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	id1, err := s.Add("* * * * * *", func(_ context.Context) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id1 == "" {
		t.Error("Add returned empty ID")
	}

	id2, err := s.Add("* * * * * *", func(_ context.Context) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id1 == id2 {
		t.Error("expected different IDs")
	}

	wg.Wait()
}

func TestScheduler_Add_DuplicateID(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.AddWithID("dup", "* * * * * *", func(_ context.Context) {})
	if err != nil {
		t.Fatalf("first AddWithID failed: %v", err)
	}

	err = s.AddWithID("dup", "* * * * * *", func(_ context.Context) {})
	if !errors.Is(err, ErrTaskAlreadyExists) {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}
}

func TestScheduler_Cancel(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var executed int32

	err := s.AddWithID("cancel-me", "* * * * * *", func(_ context.Context) {
		atomic.AddInt32(&executed, 1)
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	task, _ := s.GetTask("cancel-me")
	nextRun := task.NextRun
	time.Sleep(nextRun.Sub(time.Now()) / 2)

	err = s.Cancel("cancel-me")
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}

	if s.TaskCount() != 0 {
		t.Errorf("expected 0 tasks after cancel, got %d", s.TaskCount())
	}

	time.Sleep(2 * time.Second)

	if atomic.LoadInt32(&executed) != 0 {
		t.Error("cancelled task was executed")
	}
}

func TestScheduler_Cancel_NotFound(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.Cancel("nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestScheduler_GetTask(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_ = s.AddWithID("get-test", "* * * * * *", func(_ context.Context) {})

	task, err := s.GetTask("get-test")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.ID != "get-test" {
		t.Errorf("expected ID get-test, got %s", task.ID)
	}
	if task.Status != StatusPending {
		t.Errorf("expected StatusPending, got %v", task.Status)
	}

	_, err = s.GetTask("nonexistent")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestScheduler_StartStop(t *testing.T) {
	s := NewScheduler()
	s.Start()
	s.Start()

	s.Stop()
	s.Stop()

	done := make(chan struct{})
	go func() {
		s.Start()
		s.Stop()
		done <- struct{}{}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Start/Stop deadlocked")
	}
}

func TestScheduler_Add_Stopped(t *testing.T) {
	s := NewScheduler()

	_, err := s.Add("* * * * * *", func(_ context.Context) {})
	if !errors.Is(err, ErrSchedulerStopped) {
		t.Errorf("expected ErrSchedulerStopped, got %v", err)
	}
}

func TestScheduler_StopBeforeExecute(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var executed int32
	_ = s.AddWithID("stop-early", "* * * * * *", func(_ context.Context) {
		atomic.AddInt32(&executed, 1)
	})

	s.Stop()
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 0 {
		t.Error("task executed after scheduler stopped")
	}
}

func TestScheduler_TaskPanic(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)
	var count int32

	_ = s.AddWithID("panic-task", "* * * * * *", func(_ context.Context) {
		n := atomic.AddInt32(&count, 1)
		if n == 1 {
			panic("intentional panic")
		}
		if n >= 2 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Errorf("task did not recover from panic. count=%d", atomic.LoadInt32(&count))
	}
}

func TestScheduler_MultipleTimezones(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	now := time.Now()
	nextSecond := now.Second() + 1
	if nextSecond >= 60 {
		nextSecond = 0
	}

	expr := fmt.Sprintf("%d * * * * *", nextSecond)

	loc1 := time.UTC
	loc2 := time.FixedZone("UTC+1", 3600)

	err := s.AddWithIDAndLocation("tz1", expr, loc1, func(_ context.Context) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Add tz1 failed: %v", err)
	}

	err = s.AddWithIDAndLocation("tz2", expr, loc2, func(_ context.Context) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Add tz2 failed: %v", err)
	}

	select {
	case <-waitTimeout(&wg, 2*time.Second):
	case <-time.After(3 * time.Second):
		t.Error("tasks did not execute in time")
	}
}

func waitTimeout(wg *sync.WaitGroup, timeout time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return done
}

// ------------------------------ Expand Tests ------------------------------

func TestFieldValue_Expand(t *testing.T) {
	tests := []struct {
		name string
		fv   FieldValue
		min  int
		max  int
		want []int
	}{
		{"wildcard", FieldValue{Type: ValueWildcard}, 0, 5, []int{0, 1, 2, 3, 4, 5}},
		{"single", FieldValue{Type: ValueSingle, Value: 5}, 0, 10, []int{5}},
		{"range", FieldValue{Type: ValueRange, RangeLow: 2, RangeHigh: 5}, 0, 10, []int{2, 3, 4, 5}},
		{"step wildcard", FieldValue{Type: ValueStep, RangeLow: 0, RangeHigh: 10, Step: 2}, 0, 10, []int{0, 2, 4, 6, 8, 10}},
		{"step range", FieldValue{Type: ValueStep, RangeLow: 1, RangeHigh: 9, Step: 3}, 0, 10, []int{1, 4, 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fv.Expand(tt.min, tt.max)
			if !slicesEqual(got, tt.want) {
				t.Errorf("Expand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCronField_Expand(t *testing.T) {
	cf, err := parseField("1,3-5,*/10", FieldSecond)
	if err != nil {
		t.Fatalf("parseField failed: %v", err)
	}
	got := cf.Expand()
	want := []int{0, 1, 3, 4, 5, 10, 20, 30, 40, 50}
	if !slicesEqual(got, want) {
		t.Errorf("Expand() = %v, want %v", got, want)
	}
}

func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ------------------------------ Error Tests ------------------------------

func TestParseError_Error(t *testing.T) {
	err := NewParseError(FieldSecond, 3, "61", "value out of range")
	errStr := err.Error()
	if !strings.Contains(errStr, "second") {
		t.Errorf("error should contain field name: %s", errStr)
	}
	if !strings.Contains(errStr, "61") {
		t.Errorf("error should contain raw value: %s", errStr)
	}
	if !strings.Contains(errStr, "position 3") {
		t.Errorf("error should contain position: %s", errStr)
	}
}

func TestIsValidDay(t *testing.T) {
	tests := []struct {
		year  int
		month int
		day   int
		want  bool
	}{
		{2025, 1, 31, true},
		{2025, 2, 28, true},
		{2025, 2, 29, false},
		{2024, 2, 29, true},
		{2024, 2, 30, false},
		{2025, 4, 30, true},
		{2025, 4, 31, false},
		{2025, 6, 31, false},
		{2025, 9, 31, false},
		{2025, 11, 31, false},
		{2025, 12, 31, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d-%02d-%02d", tt.year, tt.month, tt.day), func(t *testing.T) {
			if got := isValidDay(tt.year, tt.month, tt.day); got != tt.want {
				t.Errorf("isValidDay(%d, %d, %d) = %v, want %v", tt.year, tt.month, tt.day, got, tt.want)
			}
		})
	}
}

// ------------------------------ Concurrent Tests ------------------------------

func TestScheduler_ConcurrentAdd(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-add-%d", i)
			_ = s.AddWithID(id, "* * * * * *", func(_ context.Context) {})
		}(i)
	}
	wg.Wait()

	if s.TaskCount() != n {
		t.Errorf("expected %d tasks, got %d", n, s.TaskCount())
	}
}

func TestScheduler_ConcurrentCancel(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	n := 50
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("concurrent-cancel-%d", i)
		_ = s.AddWithID(id, "* * * * * *", func(_ context.Context) {})
	}

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-cancel-%d", i)
			_ = s.Cancel(id)
		}(i)
	}
	wg.Wait()

	if s.TaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", s.TaskCount())
	}
}

// ------------------------------ Boundary Tests ------------------------------

func TestNextTime_YearBoundary(t *testing.T) {
	from := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC)

	expr, err := Parse("0 0 0 1 1 *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("year boundary: got %v, want %v", got, want)
	}
}

func TestNextTime_DayBoundary(t *testing.T) {
	from := time.Date(2025, 6, 15, 23, 59, 0, 0, time.UTC)

	expr, err := Parse("0 0 0 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("day boundary: got %v, want %v", got, want)
	}
}

func TestNextTime_HourBoundary(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 59, 30, 0, time.UTC)

	expr, err := Parse("0 0 11 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("hour boundary: got %v, want %v", got, want)
	}
}

func TestNextTime_FromExactMatch(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	expr, err := Parse("0 30 10 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	got, err := NextTime(expr, from)
	if err != nil {
		t.Fatalf("NextTime failed: %v", err)
	}
	want := time.Date(2025, 6, 16, 10, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("exact match should return next occurrence, got %v, want %v", got, want)
	}
}

// ------------------------------ String Methods Tests ------------------------------

func TestStringMethods(t *testing.T) {
	expr, err := Parse("0 30 14 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if expr.String() != "0 30 14 * * *" {
		t.Errorf("expr.String() = %q, want %q", expr.String(), "0 30 14 * * *")
	}

	if expr.Second.String() != "0" {
		t.Errorf("second.String() = %q, want %q", expr.Second.String(), "0")
	}

	if expr.Minute.String() != "30" {
		t.Errorf("minute.String() = %q, want %q", expr.Minute.String(), "30")
	}
}

func TestTaskStatus_String(t *testing.T) {
	tests := []struct {
		status TaskStatus
		want   string
	}{
		{StatusPending, "PENDING"},
		{StatusRunning, "RUNNING"},
		{StatusCancelled, "CANCELLED"},
		{StatusDone, "DONE"},
		{TaskStatus(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.status.String(); got != tt.want {
				t.Errorf("TaskStatus(%d).String() = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestFieldType_String(t *testing.T) {
	tests := []struct {
		ft   FieldType
		want string
	}{
		{FieldSecond, "second"},
		{FieldMinute, "minute"},
		{FieldHour, "hour"},
		{FieldDay, "day"},
		{FieldMonth, "month"},
		{FieldWeekday, "weekday"},
		{FieldYear, "year"},
		{FieldType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ft.String(); got != tt.want {
				t.Errorf("FieldType(%d).String() = %q, want %q", tt.ft, got, tt.want)
			}
		})
	}
}

// ------------------------------ Expression Methods Tests ------------------------------

func TestExpression_Next(t *testing.T) {
	expr, err := Parse("0 0 10 * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	from := time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC)
	got, err := expr.Next(from)
	if err != nil {
		t.Fatalf("expr.Next failed: %v", err)
	}
	want := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("expr.Next() = %v, want %v", got, want)
	}
}

func TestExpression_NextN(t *testing.T) {
	expr, err := Parse("0 0 * * * *")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	from := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)
	times, err := expr.NextN(from, 3)
	if err != nil {
		t.Fatalf("expr.NextN failed: %v", err)
	}
	if len(times) != 3 {
		t.Fatalf("expected 3 times, got %d", len(times))
	}

	expected := []time.Time{
		time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC),
		time.Date(2025, 6, 15, 13, 0, 0, 0, time.UTC),
	}

	for i, got := range times {
		if !got.Equal(expected[i]) {
			t.Errorf("times[%d] = %v, want %v", i, got, expected[i])
		}
	}
}

// ------------------------------ Scheduler Cancel While Running Tests ------------------------------

func TestScheduler_Cancel_WhileRunning(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	started := make(chan struct{}, 1)
	release := make(chan struct{}, 1)

	_ = s.AddWithID("running-task", "* * * * * *", func(_ context.Context) {
		started <- struct{}{}
		<-release
	})

	<-started

	err := s.Cancel("running-task")
	if !errors.Is(err, ErrTaskRunning) {
		t.Errorf("expected ErrTaskRunning, got %v", err)
	}

	release <- struct{}{}
	time.Sleep(50 * time.Millisecond)

	err = s.Cancel("running-task")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound after completion, got %v", err)
	}
}

// ------------------------------ Memory Leak Tests ------------------------------

func TestScheduler_CancelPending_NoMemoryLeak(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	const iterations = 100
	const batchSize = 10

	for i := 0; i < iterations; i++ {
		for j := 0; j < batchSize; j++ {
			id := fmt.Sprintf("leak-pending-%d-%d", i, j)
			err := s.AddWithID(id, "* * * * * *", func(_ context.Context) {})
			if err != nil {
				t.Fatalf("iteration %d AddWithID failed: %v", i, err)
			}
		}
		for j := 0; j < batchSize; j++ {
			id := fmt.Sprintf("leak-pending-%d-%d", i, j)
			err := s.Cancel(id)
			if err != nil {
				t.Fatalf("iteration %d Cancel failed: %v", i, err)
			}
		}
	}

	if count := s.TaskCount(); count != 0 {
		t.Errorf("after %d iterations of add+cancel, TaskCount = %d (expected 0) — memory leak?",
			iterations, count)
	}
}
