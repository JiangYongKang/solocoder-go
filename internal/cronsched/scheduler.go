package cronsched

import (
	"container/heap"
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Scheduler struct {
	mu       sync.Mutex
	heap     *taskHeap
	tasks    map[string]*CronTask
	running  bool
	stopCh   chan struct{}
	wakeCh   chan struct{}
	wg       sync.WaitGroup
	taskWg   sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	nextID   uint64
	idMu     sync.Mutex
	config   *SchedulerConfig
}

type taskHeap []*CronTask

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	return h[i].NextRun.Before(h[j].NextRun)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *taskHeap) Push(x interface{}) {
	task := x.(*CronTask)
	*h = append(*h, task)
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	*h = old[0 : n-1]
	return task
}

func NewScheduler() *Scheduler {
	return NewSchedulerWithConfig(nil)
}

func NewSchedulerWithConfig(config *SchedulerConfig) *Scheduler {
	if config == nil {
		config = DefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	h := make(taskHeap, 0)
	heap.Init(&h)
	return &Scheduler{
		heap:   &h,
		tasks:  make(map[string]*CronTask),
		stopCh: make(chan struct{}),
		wakeCh: make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
		config: config,
	}
}

func (s *Scheduler) generateID() string {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.nextID++
	return fmt.Sprintf("cron-%d-%d", time.Now().UnixNano(), s.nextID)
}

func (s *Scheduler) wake() {
	close(s.wakeCh)
	s.wakeCh = make(chan struct{})
}

func (s *Scheduler) Add(expr string, fn TaskFunc) (string, error) {
	return s.AddWithLocation(expr, time.UTC, fn)
}

func (s *Scheduler) AddWithLocation(expr string, loc *time.Location, fn TaskFunc) (string, error) {
	id := s.generateID()
	err := s.AddWithIDAndLocation(id, expr, loc, fn)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Scheduler) AddWithID(id string, expr string, fn TaskFunc) error {
	return s.AddWithIDAndLocation(id, expr, time.UTC, fn)
}

func (s *Scheduler) AddWithIDAndLocation(id string, expr string, loc *time.Location, fn TaskFunc) error {
	if loc == nil {
		return ErrInvalidTimezone
	}

	cronExpr, err := ParseWithLocation(expr, loc)
	if err != nil {
		return err
	}

	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return ErrSchedulerStopped
	}
	s.mu.Unlock()

	now := time.Now().In(loc)
	nextRun, err := NextTime(cronExpr, now)
	if err != nil {
		return err
	}

	task := &CronTask{
		ID:        id,
		CronExpr:  cronExpr,
		Func:      fn,
		Status:    StatusPending,
		NextRun:   nextRun,
		Location:  loc,
		CreatedAt: time.Now(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return ErrSchedulerStopped
	}
	if _, exists := s.tasks[id]; exists {
		return ErrTaskAlreadyExists
	}
	s.tasks[id] = task
	heap.Push(s.heap, task)
	s.wake()
	return nil
}

func (s *Scheduler) Cancel(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	switch task.Status {
	case StatusPending:
		idx := -1
		for i, t := range *s.heap {
			if t == task {
				idx = i
				break
			}
		}
		if idx >= 0 {
			heap.Remove(s.heap, idx)
		}
		delete(s.tasks, id)
		s.wake()
		return nil
	case StatusRunning:
		task.Status = StatusCancelled
		return ErrTaskRunning
	case StatusCancelled:
		return nil
	case StatusDone:
		return nil
	default:
		return nil
	}
}

func (s *Scheduler) GetTask(id string) (*CronTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

func (s *Scheduler) TaskCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, t := range s.tasks {
		if t.Status == StatusPending || t.Status == StatusRunning {
			count++
		}
	}
	return count
}

func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.stopCh = make(chan struct{})
	s.wakeCh = make(chan struct{})
	s.mu.Unlock()

	s.wg.Add(1)
	go s.runLoop()
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.cancel()
	close(s.stopCh)
	s.wake()
	s.mu.Unlock()

	s.wg.Wait()
	s.taskWg.Wait()
}

func (s *Scheduler) runLoop() {
	defer s.wg.Done()

	for {
		s.mu.Lock()
		if !s.running {
			s.mu.Unlock()
			return
		}

		for s.heap.Len() > 0 {
			task := (*s.heap)[0]
			if task.Status == StatusCancelled || task.Status == StatusDone {
				heap.Pop(s.heap)
				continue
			}
			break
		}

		if s.heap.Len() == 0 {
			wakeCh := s.wakeCh
			s.mu.Unlock()
			select {
			case <-s.stopCh:
				return
			case <-wakeCh:
				continue
			}
		}

		task := (*s.heap)[0]
		now := time.Now().In(task.Location)
		waitTime := task.NextRun.Sub(now)

		if waitTime <= 0 {
			heap.Pop(s.heap)
			task.Status = StatusRunning
			s.taskWg.Add(1)
			s.mu.Unlock()
			go s.executeTask(task)
			continue
		}

		wakeCh := s.wakeCh
		timer := time.NewTimer(waitTime)
		s.mu.Unlock()

		select {
		case <-timer.C:
		case <-wakeCh:
			timer.Stop()
		case <-s.stopCh:
			timer.Stop()
			return
		}
	}
}

func (s *Scheduler) executeTask(task *CronTask) {
	defer s.taskWg.Done()

	func() {
		defer func() {
			recover()
		}()
		ctx, cancel := context.WithCancel(s.ctx)
		defer cancel()
		task.Func(ctx)
	}()

	s.mu.Lock()
	defer s.mu.Unlock()

	task.LastRun = time.Now()
	task.RunCount++

	if task.Status == StatusCancelled {
		delete(s.tasks, task.ID)
		return
	}

	if _, exists := s.tasks[task.ID]; !exists {
		return
	}

	nextRun, err := NextTime(task.CronExpr, time.Now().In(task.Location))
	if err != nil {
		delete(s.tasks, task.ID)
		task.Status = StatusDone
		return
	}

	task.NextRun = nextRun
	task.Status = StatusPending
	heap.Push(s.heap, task)
	s.wake()
}

func NextTime(expr *CronExpression, from time.Time) (time.Time, error) {
	return NextTimeWithConfig(expr, from, DefaultConfig())
}

func NextTimeWithConfig(expr *CronExpression, from time.Time, config *SchedulerConfig) (time.Time, error) {
	if expr == nil {
		return time.Time{}, ErrInvalidExpression
	}
	if config == nil {
		config = DefaultConfig()
	}

	loc := expr.Location
	if loc == nil {
		loc = time.UTC
	}

	fromLocal := from.In(loc)
	candidate := fromLocal.Add(time.Second).Truncate(time.Second)

	daySet := !isWildcard(expr.Day)
	weekdaySet := !isWildcard(expr.Weekday)

	for i := 0; i < config.MaxIterations; i++ {
		candidate = adjustForDST(candidate, loc)

		year := candidate.Year()
		month := int(candidate.Month())
		day := candidate.Day()
		hour := candidate.Hour()
		minute := candidate.Minute()
		second := candidate.Second()
		weekday := int(candidate.Weekday())

		if expr.HasYear && !expr.Year.Matches(year) {
			candidate = time.Date(year+1, 1, 1, 0, 0, 0, 0, loc)
			continue
		}

		if !expr.Month.Matches(month) {
			if month == 12 {
				candidate = time.Date(year+1, time.Month(1), 1, 0, 0, 0, 0, loc)
			} else {
				candidate = time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, loc)
			}
			continue
		}

		dayMatched := true
		if daySet || weekdaySet {
			if daySet && !expr.Day.Matches(day) {
				dayMatched = false
			}
			if weekdaySet && !expr.Weekday.Matches(weekday) {
				dayMatched = false
			}
		} else {
			dayMatched = expr.Day.Matches(day) && expr.Weekday.Matches(weekday)
		}

		if !dayMatched {
			candidate = time.Date(year, time.Month(month), day+1, 0, 0, 0, 0, loc)
			continue
		}

		if !expr.Hour.Matches(hour) {
			candidate = time.Date(year, time.Month(month), day, hour+1, 0, 0, 0, loc)
			continue
		}

		if !expr.Minute.Matches(minute) {
			candidate = time.Date(year, time.Month(month), day, hour, minute+1, 0, 0, loc)
			continue
		}

		if !expr.Second.Matches(second) {
			candidate = time.Date(year, time.Month(month), day, hour, minute, second+1, 0, loc)
			continue
		}

		if !isValidDay(year, month, day) {
			candidate = time.Date(year, time.Month(month), day+1, 0, 0, 0, 0, loc)
			continue
		}

		return candidate, nil
	}

	return time.Time{}, ErrNoNextTime
}

func NextTimes(expr *CronExpression, from time.Time, count int) ([]time.Time, error) {
	times := make([]time.Time, 0, count)
	current := from

	for i := 0; i < count; i++ {
		next, err := NextTime(expr, current)
		if err != nil {
			return nil, err
		}
		times = append(times, next)
		current = next
	}

	return times, nil
}

func adjustForDST(t time.Time, loc *time.Location) time.Time {
	tInLoc := t.In(loc)
	utc := tInLoc.UTC()
	convertedBack := utc.In(loc)
	if !tInLoc.Equal(convertedBack) {
		return convertedBack
	}
	return tInLoc
}

func isValidDay(year, month, day int) bool {
	if day < 1 || day > 31 {
		return false
	}
	daysInMonth := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	return day <= daysInMonth
}

func Validate(expr string) *ValidationResult {
	return ValidateWithLocation(expr, time.UTC)
}

func ValidateWithLocation(expr string, loc *time.Location) *ValidationResult {
	result := &ValidationResult{
		Valid:  false,
		Errors: make([]error, 0),
	}

	cronExpr, err := ParseWithLocation(expr, loc)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	result.Valid = true
	result.Description = GenerateDescription(cronExpr)

	_, err = NextTime(cronExpr, time.Now().In(loc))
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err)
	}

	return result
}

func GenerateDescription(expr *CronExpression) string {
	if expr == nil {
		return ""
	}

	parts := make([]string, 0, 7)

	if !isWildcard(expr.Second) {
		parts = append(parts, describeField(expr.Second, "秒"))
	}

	if !isWildcard(expr.Minute) {
		parts = append(parts, describeField(expr.Minute, "分"))
	}

	if !isWildcard(expr.Hour) {
		parts = append(parts, describeField(expr.Hour, "时"))
	}

	if !isWildcard(expr.Day) {
		parts = append(parts, "在每月的 "+describeField(expr.Day, "日"))
	}

	if !isWildcard(expr.Month) {
		parts = append(parts, describeMonthField(expr.Month))
	}

	if !isWildcard(expr.Weekday) {
		parts = append(parts, "在每"+describeWeekdayField(expr.Weekday))
	}

	if expr.HasYear && !isWildcard(expr.Year) {
		parts = append(parts, describeField(expr.Year, "年"))
	}

	if len(parts) == 0 {
		return "每秒执行"
	}

	return strings.Join(parts, "，") + " 执行"
}

func describeField(cf *CronField, unit string) string {
	if len(cf.Values) == 1 {
		fv := cf.Values[0]
		switch fv.Type {
		case ValueWildcard:
			return "每" + unit
		case ValueSingle:
			return fmt.Sprintf("%d%s", fv.Value, unit)
		case ValueRange:
			return fmt.Sprintf("%d-%d%s", fv.RangeLow, fv.RangeHigh, unit)
		case ValueStep:
			if fv.RangeLow == cf.Min && fv.RangeHigh == cf.Max {
				return fmt.Sprintf("每%d%s", fv.Step, unit)
			}
			return fmt.Sprintf("%d-%d%s 每%d%s", fv.RangeLow, fv.RangeHigh, unit, fv.Step, unit)
		}
	}

	values := make([]string, 0, len(cf.Values))
	for _, fv := range cf.Values {
		switch fv.Type {
		case ValueSingle:
			values = append(values, fmt.Sprintf("%d", fv.Value))
		case ValueRange:
			values = append(values, fmt.Sprintf("%d-%d", fv.RangeLow, fv.RangeHigh))
		case ValueStep:
			values = append(values, fmt.Sprintf("%d-%d/%d", fv.RangeLow, fv.RangeHigh, fv.Step))
		}
	}
	return strings.Join(values, ",") + unit
}

func describeMonthField(cf *CronField) string {
	monthNames := []string{"一月", "二月", "三月", "四月", "五月", "六月",
		"七月", "八月", "九月", "十月", "十一月", "十二月"}

	if len(cf.Values) == 1 {
		fv := cf.Values[0]
		switch fv.Type {
		case ValueSingle:
			return "在 " + monthNames[fv.Value-1]
		case ValueRange:
			return "从 " + monthNames[fv.RangeLow-1] + " 到 " + monthNames[fv.RangeHigh-1]
		case ValueStep:
			return fmt.Sprintf("从 %s 到 %s 每%d个月",
				monthNames[fv.RangeLow-1], monthNames[fv.RangeHigh-1], fv.Step)
		}
	}
	return "在 " + describeField(cf, "月")
}

func describeWeekdayField(cf *CronField) string {
	weekdayNames := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

	if len(cf.Values) == 1 {
		fv := cf.Values[0]
		switch fv.Type {
		case ValueSingle:
			return weekdayNames[fv.Value]
		case ValueRange:
			return weekdayNames[fv.RangeLow] + " 到 " + weekdayNames[fv.RangeHigh]
		case ValueStep:
			return fmt.Sprintf("%s 到 %s 每%d天",
				weekdayNames[fv.RangeLow], weekdayNames[fv.RangeHigh], fv.Step)
		}
	}

	values := make([]string, 0, len(cf.Values))
	for _, fv := range cf.Values {
		switch fv.Type {
		case ValueSingle:
			values = append(values, weekdayNames[fv.Value])
		case ValueRange:
			values = append(values, weekdayNames[fv.RangeLow]+"-"+weekdayNames[fv.RangeHigh])
		case ValueStep:
			values = append(values, fmt.Sprintf("%s-%s/%d",
				weekdayNames[fv.RangeLow], weekdayNames[fv.RangeHigh], fv.Step))
		}
	}
	return strings.Join(values, ",")
}

func (ce *CronExpression) Next(from time.Time) (time.Time, error) {
	return NextTime(ce, from)
}

func (ce *CronExpression) NextN(from time.Time, count int) ([]time.Time, error) {
	return NextTimes(ce, from, count)
}

func (ce *CronExpression) Matches(t time.Time) bool {
	loc := ce.Location
	if loc == nil {
		loc = time.UTC
	}
	tLocal := t.In(loc)

	year := tLocal.Year()
	month := int(tLocal.Month())
	day := tLocal.Day()
	hour := tLocal.Hour()
	minute := tLocal.Minute()
	second := tLocal.Second()
	weekday := int(tLocal.Weekday())

	if ce.HasYear && !ce.Year.Matches(year) {
		return false
	}
	if !ce.Month.Matches(month) {
		return false
	}

	daySet := !isWildcard(ce.Day)
	weekdaySet := !isWildcard(ce.Weekday)

	if daySet || weekdaySet {
		if daySet && !ce.Day.Matches(day) {
			return false
		}
		if weekdaySet && !ce.Weekday.Matches(weekday) {
			return false
		}
	} else {
		if !ce.Day.Matches(day) || !ce.Weekday.Matches(weekday) {
			return false
		}
	}

	if !ce.Hour.Matches(hour) {
		return false
	}
	if !ce.Minute.Matches(minute) {
		return false
	}
	if !ce.Second.Matches(second) {
		return false
	}

	return isValidDay(year, month, day)
}

func (ce *CronExpression) String() string {
	return ce.Raw
}

func (cf *CronField) String() string {
	return cf.Raw
}

func (fv *FieldValue) Expand(min, max int) []int {
	result := make([]int, 0)

	switch fv.Type {
	case ValueWildcard:
		for i := min; i <= max; i++ {
			result = append(result, i)
		}
	case ValueSingle:
		result = append(result, fv.Value)
	case ValueRange:
		for i := fv.RangeLow; i <= fv.RangeHigh; i++ {
			result = append(result, i)
		}
	case ValueStep:
		for i := fv.RangeLow; i <= fv.RangeHigh; i += fv.Step {
			result = append(result, i)
		}
	}

	return result
}

func (cf *CronField) Expand() []int {
	result := make([]int, 0)
	seen := make(map[int]bool)

	for _, fv := range cf.Values {
		for _, v := range fv.Expand(cf.Min, cf.Max) {
			if !seen[v] {
				seen[v] = true
				result = append(result, v)
			}
		}
	}

	sort.Ints(result)
	return result
}
