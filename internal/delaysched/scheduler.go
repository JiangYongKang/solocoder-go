package delaysched

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrTaskNotFound      = errors.New("task not found")
	ErrTaskAlreadyExists = errors.New("task already exists")
	ErrTaskRunning       = errors.New("task is running and cannot be cancelled immediately")
	ErrInvalidCronExpr   = errors.New("invalid cron expression")
	ErrSchedulerStopped  = errors.New("scheduler is stopped")
)

type TaskFunc func(ctx context.Context)

type RepeatType int

const (
	RepeatNone     RepeatType = iota
	RepeatInterval RepeatType = iota
	RepeatCron     RepeatType = iota
)

type TaskStatus int

const (
	StatusPending   TaskStatus = iota
	StatusRunning   TaskStatus = iota
	StatusCancelled TaskStatus = iota
	StatusDone      TaskStatus = iota
)

type Task struct {
	ID         string
	ExecuteAt  time.Time
	Func       TaskFunc
	RepeatType RepeatType
	Interval   time.Duration
	CronExpr   string
	Status     TaskStatus
	index      int
}

type taskHeap []*Task

func (h taskHeap) Len() int { return len(h) }

func (h taskHeap) Less(i, j int) bool {
	return h[i].ExecuteAt.Before(h[j].ExecuteAt)
}

func (h taskHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *taskHeap) Push(x interface{}) {
	n := len(*h)
	task := x.(*Task)
	task.index = n
	*h = append(*h, task)
}

func (h *taskHeap) Pop() interface{} {
	old := *h
	n := len(old)
	task := old[n-1]
	old[n-1] = nil
	task.index = -1
	*h = old[0 : n-1]
	return task
}

type Scheduler struct {
	mu       sync.Mutex
	heap     *taskHeap
	tasks    map[string]*Task
	running  bool
	stopCh   chan struct{}
	wakeCh   chan struct{}
	wg       sync.WaitGroup
	taskWg   sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	nextID   uint64
	idMu     sync.Mutex
}

func NewScheduler() *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	h := make(taskHeap, 0)
	heap.Init(&h)
	return &Scheduler{
		heap:   &h,
		tasks:  make(map[string]*Task),
		stopCh: make(chan struct{}),
		wakeCh: make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *Scheduler) generateID() string {
	s.idMu.Lock()
	defer s.idMu.Unlock()
	s.nextID++
	return fmt.Sprintf("task-%d-%d", time.Now().UnixNano(), s.nextID)
}

func (s *Scheduler) wake() {
	close(s.wakeCh)
	s.wakeCh = make(chan struct{})
}

func (s *Scheduler) Add(delay time.Duration, fn TaskFunc) (string, error) {
	return s.AddAt(time.Now().Add(delay), fn)
}

func (s *Scheduler) AddAt(executeAt time.Time, fn TaskFunc) (string, error) {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return "", ErrSchedulerStopped
	}
	s.mu.Unlock()

	id := s.generateID()
	task := &Task{
		ID:         id,
		ExecuteAt:  executeAt,
		Func:       fn,
		RepeatType: RepeatNone,
		Status:     StatusPending,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return "", ErrSchedulerStopped
	}
	if _, exists := s.tasks[id]; exists {
		return "", ErrTaskAlreadyExists
	}
	s.tasks[id] = task
	heap.Push(s.heap, task)
	s.wake()
	return id, nil
}

func (s *Scheduler) AddWithID(id string, delay time.Duration, fn TaskFunc) error {
	return s.AddAtWithID(id, time.Now().Add(delay), fn)
}

func (s *Scheduler) AddAtWithID(id string, executeAt time.Time, fn TaskFunc) error {
	task := &Task{
		ID:         id,
		ExecuteAt:  executeAt,
		Func:       fn,
		RepeatType: RepeatNone,
		Status:     StatusPending,
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

func (s *Scheduler) AddInterval(delay time.Duration, interval time.Duration, fn TaskFunc) (string, error) {
	id := s.generateID()
	return s.addRepeat(id, time.Now().Add(delay), fn, RepeatInterval, interval, "")
}

func (s *Scheduler) AddIntervalWithID(id string, delay time.Duration, interval time.Duration, fn TaskFunc) error {
	_, err := s.addRepeat(id, time.Now().Add(delay), fn, RepeatInterval, interval, "")
	return err
}

func (s *Scheduler) AddCron(delay time.Duration, cronExpr string, fn TaskFunc) (string, error) {
	if err := validateCron(cronExpr); err != nil {
		return "", err
	}
	id := s.generateID()
	firstAt, err := nextCronTime(cronExpr, time.Now().Add(delay))
	if err != nil {
		return "", err
	}
	return s.addRepeat(id, firstAt, fn, RepeatCron, 0, cronExpr)
}

func (s *Scheduler) AddCronWithID(id string, delay time.Duration, cronExpr string, fn TaskFunc) error {
	if err := validateCron(cronExpr); err != nil {
		return err
	}
	firstAt, err := nextCronTime(cronExpr, time.Now().Add(delay))
	if err != nil {
		return err
	}
	_, err = s.addRepeat(id, firstAt, fn, RepeatCron, 0, cronExpr)
	return err
}

func (s *Scheduler) addRepeat(id string, executeAt time.Time, fn TaskFunc, rt RepeatType, interval time.Duration, cronExpr string) (string, error) {
	task := &Task{
		ID:         id,
		ExecuteAt:  executeAt,
		Func:       fn,
		RepeatType: rt,
		Interval:   interval,
		CronExpr:   cronExpr,
		Status:     StatusPending,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return "", ErrSchedulerStopped
	}
	if _, exists := s.tasks[id]; exists {
		return "", ErrTaskAlreadyExists
	}
	s.tasks[id] = task
	heap.Push(s.heap, task)
	s.wake()
	return id, nil
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
		if task.index >= 0 && task.index < s.heap.Len() && (*s.heap)[task.index] == task {
			heap.Remove(s.heap, task.index)
		}
		delete(s.tasks, id)
		s.wake()
		return nil
	case StatusRunning:
		if task.RepeatType == RepeatNone {
			return ErrTaskRunning
		}
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

func (s *Scheduler) Reschedule(id string, newExecuteAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	if task.Status == StatusRunning {
		return ErrTaskRunning
	}

	if task.Status == StatusCancelled || task.Status == StatusDone {
		return ErrTaskNotFound
	}

	task.ExecuteAt = newExecuteAt
	if task.index >= 0 && task.index < s.heap.Len() && (*s.heap)[task.index] == task {
		heap.Fix(s.heap, task.index)
	}
	s.wake()
	return nil
}

func (s *Scheduler) RescheduleDelay(id string, newDelay time.Duration) error {
	return s.Reschedule(id, time.Now().Add(newDelay))
}

func (s *Scheduler) GetTask(id string) (*Task, error) {
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
			if task.Status == StatusCancelled || task.Status == StatusDone || task.Status == StatusRunning {
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
		now := time.Now()
		waitTime := task.ExecuteAt.Sub(now)

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

func (s *Scheduler) executeTask(task *Task) {
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

	if task.Status == StatusCancelled {
		delete(s.tasks, task.ID)
		return
	}

	if task.RepeatType == RepeatNone {
		task.Status = StatusDone
		delete(s.tasks, task.ID)
		return
	}

	if _, exists := s.tasks[task.ID]; !exists {
		return
	}

	var nextExecuteAt time.Time
	switch task.RepeatType {
	case RepeatInterval:
		nextExecuteAt = time.Now().Add(task.Interval)
	case RepeatCron:
		next, err := nextCronTime(task.CronExpr, time.Now())
		if err != nil {
			delete(s.tasks, task.ID)
			task.Status = StatusDone
			return
		}
		nextExecuteAt = next
	}

	task.ExecuteAt = nextExecuteAt
	task.Status = StatusPending
	heap.Push(s.heap, task)
	s.wake()
}

func validateCron(expr string) error {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return ErrInvalidCronExpr
	}

	ranges := [][]int{
		{0, 59},
		{0, 23},
		{1, 31},
		{1, 12},
		{0, 6},
	}

	for i, field := range fields {
		if err := validateCronField(field, ranges[i][0], ranges[i][1]); err != nil {
			return err
		}
	}
	return nil
}

func validateCronField(field string, min, max int) error {
	if field == "*" {
		return nil
	}

	parts := strings.Split(field, ",")
	for _, part := range parts {
		if strings.Contains(part, "/") {
			stepParts := strings.Split(part, "/")
			if len(stepParts) != 2 {
				return ErrInvalidCronExpr
			}
			rangePart := stepParts[0]
			stepStr := stepParts[1]
			step, err := strconv.Atoi(stepStr)
			if err != nil || step <= 0 {
				return ErrInvalidCronExpr
			}
			if rangePart == "*" {
				continue
			}
			if strings.Contains(rangePart, "-") {
				rangeVals := strings.Split(rangePart, "-")
				if len(rangeVals) != 2 {
					return ErrInvalidCronExpr
				}
				low, err1 := strconv.Atoi(rangeVals[0])
				high, err2 := strconv.Atoi(rangeVals[1])
				if err1 != nil || err2 != nil || low < min || high > max || low > high {
					return ErrInvalidCronExpr
				}
				continue
			}
			return ErrInvalidCronExpr
		}

		if strings.Contains(part, "-") {
			rangeVals := strings.Split(part, "-")
			if len(rangeVals) != 2 {
				return ErrInvalidCronExpr
			}
			low, err1 := strconv.Atoi(rangeVals[0])
			high, err2 := strconv.Atoi(rangeVals[1])
			if err1 != nil || err2 != nil || low < min || high > max || low > high {
				return ErrInvalidCronExpr
			}
			continue
		}

		val, err := strconv.Atoi(part)
		if err != nil || val < min || val > max {
			return ErrInvalidCronExpr
		}
	}
	return nil
}

func nextCronTime(expr string, from time.Time) (time.Time, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return time.Time{}, ErrInvalidCronExpr
	}

	minuteField := fields[0]
	hourField := fields[1]
	dayField := fields[2]
	monthField := fields[3]
	weekdayField := fields[4]

	candidate := from.Add(time.Minute).Truncate(time.Minute)

	for i := 0; i < 366*24*60; i++ {
		month := int(candidate.Month())
		day := candidate.Day()
		hour := candidate.Hour()
		minute := candidate.Minute()
		weekday := int(candidate.Weekday())

		if matchesCronField(monthField, month, 1, 12) &&
			matchesCronField(dayField, day, 1, 31) &&
			matchesCronField(hourField, hour, 0, 23) &&
			matchesCronField(minuteField, minute, 0, 59) &&
			matchesCronField(weekdayField, weekday, 0, 6) {
			return candidate, nil
		}
		candidate = candidate.Add(time.Minute)
	}

	return time.Time{}, ErrInvalidCronExpr
}

func matchesCronField(field string, value int, min, max int) bool {
	if field == "*" {
		return true
	}

	parts := strings.Split(field, ",")
	for _, part := range parts {
		if strings.Contains(part, "/") {
			stepParts := strings.Split(part, "/")
			rangePart := stepParts[0]
			stepStr := stepParts[1]
			step, _ := strconv.Atoi(stepStr)

			var rangeStart, rangeEnd int
			if rangePart == "*" {
				rangeStart = min
				rangeEnd = max
			} else if strings.Contains(rangePart, "-") {
				rangeVals := strings.Split(rangePart, "-")
				rangeStart, _ = strconv.Atoi(rangeVals[0])
				rangeEnd, _ = strconv.Atoi(rangeVals[1])
			} else {
				rangeStart, _ = strconv.Atoi(rangePart)
				rangeEnd = max
			}

			if value >= rangeStart && value <= rangeEnd {
				offset := value - rangeStart
				if offset%step == 0 {
					return true
				}
			}
			continue
		}

		if strings.Contains(part, "-") {
			rangeVals := strings.Split(part, "-")
			low, _ := strconv.Atoi(rangeVals[0])
			high, _ := strconv.Atoi(rangeVals[1])
			if value >= low && value <= high {
				return true
			}
			continue
		}

		val, _ := strconv.Atoi(part)
		if value == val {
			return true
		}
	}
	return false
}
