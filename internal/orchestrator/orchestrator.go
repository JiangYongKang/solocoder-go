package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrTaskNotFound        = errors.New("orchestrator: task not found")
	ErrTaskAlreadyExists   = errors.New("orchestrator: task already exists")
	ErrCycleDetected       = errors.New("orchestrator: cycle detected in DAG")
	ErrInvalidDependency   = errors.New("orchestrator: invalid dependency")
	ErrOrchestratorRunning = errors.New("orchestrator: orchestrator is currently running")
	ErrTimeout             = errors.New("orchestrator: task execution timed out")
	ErrCannotRetry         = errors.New("orchestrator: task is not in a retryable state")
)

type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
	StatusSkipped
	StatusTimeout
)

func (s TaskStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusRunning:
		return "Running"
	case StatusSuccess:
		return "Success"
	case StatusFailed:
		return "Failed"
	case StatusSkipped:
		return "Skipped"
	case StatusTimeout:
		return "Timeout"
	default:
		return "Unknown"
	}
}

func (s TaskStatus) IsTerminal() bool {
	return s == StatusSuccess || s == StatusFailed || s == StatusSkipped || s == StatusTimeout
}

type TaskFunc func(ctx context.Context) (interface{}, error)

type Task struct {
	ID           string
	Name         string
	Func         TaskFunc
	Timeout      time.Duration
	Dependencies []string
	Successors   []string
	MaxRetry     int
}

type TaskResult struct {
	TaskID    string
	Status    TaskStatus
	Output    interface{}
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Attempts  int
}

type ExecutionReport struct {
	Success     bool
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	TaskResults map[string]*TaskResult
}

type Orchestrator struct {
	mu      sync.Mutex
	tasks   map[string]*Task
	results map[string]*TaskResult
	running bool
	cancel  context.CancelFunc
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		tasks:   make(map[string]*Task),
		results: make(map[string]*TaskResult),
	}
}

func (o *Orchestrator) AddTask(id string, name string, fn TaskFunc, timeout time.Duration, dependencies ...string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.running {
		return ErrOrchestratorRunning
	}

	if _, exists := o.tasks[id]; exists {
		return ErrTaskAlreadyExists
	}

	for _, dep := range dependencies {
		if dep == id {
			return ErrInvalidDependency
		}
	}

	task := &Task{
		ID:           id,
		Name:         name,
		Func:         fn,
		Timeout:      timeout,
		Dependencies: dependencies,
		MaxRetry:     0,
	}

	for _, dep := range dependencies {
		if depTask, ok := o.tasks[dep]; ok {
			depTask.Successors = append(depTask.Successors, id)
		}
	}

	o.tasks[id] = task
	o.results[id] = &TaskResult{
		TaskID: id,
		Status: StatusPending,
	}

	return nil
}

func (o *Orchestrator) SetTaskRetry(id string, maxRetry int) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	task, exists := o.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	task.MaxRetry = maxRetry
	return nil
}

func (o *Orchestrator) ValidateDAG() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		task := o.tasks[node]
		for _, successor := range task.Successors {
			if !visited[successor] {
				if hasCycle(successor) {
					return true
				}
			} else if recStack[successor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for id := range o.tasks {
		if !visited[id] {
			if hasCycle(id) {
				return ErrCycleDetected
			}
		}
	}

	for id, task := range o.tasks {
		for _, dep := range task.Dependencies {
			if _, exists := o.tasks[dep]; !exists {
				return fmt.Errorf("%w: task '%s' depends on non-existent task '%s'", ErrInvalidDependency, id, dep)
			}
		}
	}

	return nil
}

func (o *Orchestrator) topologicalSort() ([]string, error) {
	inDegree := make(map[string]int)
	for id := range o.tasks {
		inDegree[id] = 0
	}

	for _, task := range o.tasks {
		for _, successor := range task.Successors {
			inDegree[successor]++
		}
	}

	var queue []string
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	sort.Strings(queue)

	var result []string
	for len(queue) > 0 {
		sort.Strings(queue)
		node := queue[0]
		queue = queue[1:]
		result = append(result, node)

		task := o.tasks[node]
		for _, successor := range task.Successors {
			inDegree[successor]--
			if inDegree[successor] == 0 {
				queue = append(queue, successor)
			}
		}
	}

	if len(result) != len(o.tasks) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

func (o *Orchestrator) shouldSkip(taskID string) (bool, string) {
	task := o.tasks[taskID]
	for _, dep := range task.Dependencies {
		depResult := o.results[dep]
		if depResult.Status == StatusFailed || depResult.Status == StatusTimeout || depResult.Status == StatusSkipped {
			return true, dep
		}
	}
	return false, ""
}

func (o *Orchestrator) Run(ctx context.Context) (*ExecutionReport, error) {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return nil, ErrOrchestratorRunning
	}

	if err := o.validateDAGLocked(); err != nil {
		o.mu.Unlock()
		return nil, err
	}

	for id := range o.results {
		o.results[id] = &TaskResult{
			TaskID: id,
			Status: StatusPending,
		}
	}

	o.running = true
	actx, cancel := context.WithCancel(ctx)
	o.cancel = cancel
	startTime := time.Now()
	o.mu.Unlock()

	report := o.runScheduler(actx, o.tasks, startTime)
	return report, nil
}

func (o *Orchestrator) validateDAGLocked() error {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		task := o.tasks[node]
		for _, successor := range task.Successors {
			if !visited[successor] {
				if hasCycle(successor) {
					return true
				}
			} else if recStack[successor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for id := range o.tasks {
		if !visited[id] {
			if hasCycle(id) {
				return ErrCycleDetected
			}
		}
	}

	for id, task := range o.tasks {
		for _, dep := range task.Dependencies {
			if _, exists := o.tasks[dep]; !exists {
				return fmt.Errorf("%w: task '%s' depends on non-existent task '%s'", ErrInvalidDependency, id, dep)
			}
		}
	}

	return nil
}

func (o *Orchestrator) runScheduler(actx context.Context, taskSet map[string]*Task, startTime time.Time) *ExecutionReport {
	readyCount := make(map[string]int)
	for id, task := range taskSet {
		depCount := 0
		for _, dep := range task.Dependencies {
			if _, ok := taskSet[dep]; ok {
				depCount++
			}
		}
		readyCount[id] = depCount
	}

	taskCh := make(chan string, len(taskSet))
	completed := make(chan string, len(taskSet))
	pendingCount := len(taskSet)
	completedCount := 0
	var wg sync.WaitGroup

	for id := range taskSet {
		if readyCount[id] == 0 {
			o.mu.Lock()
			skip, depID := o.shouldSkip(id)
			if skip {
				o.results[id].Status = StatusSkipped
				o.results[id].Error = fmt.Errorf("skipped due to failure in dependency '%s'", depID)
			}
			o.mu.Unlock()

			if skip {
				select {
				case completed <- id:
				case <-actx.Done():
				}
			} else {
				select {
				case taskCh <- id:
				case <-actx.Done():
				}
			}
		}
	}

loop:
	for completedCount < pendingCount {
		select {
		case <-actx.Done():
			o.mu.Lock()
			for id := range taskSet {
				if o.results[id].Status == StatusPending || o.results[id].Status == StatusRunning {
					o.results[id].Status = StatusSkipped
					o.results[id].Error = actx.Err()
				}
			}
			o.mu.Unlock()
			break loop

		case taskID := <-taskCh:
			wg.Add(1)
			go func(tid string) {
				defer wg.Done()
				o.executeTask(actx, tid, completed)
			}(taskID)

		case tid := <-completed:
			completedCount++

			o.mu.Lock()
			task := o.tasks[tid]
			o.mu.Unlock()

			for _, successor := range task.Successors {
				if _, ok := taskSet[successor]; !ok {
					continue
				}
				readyCount[successor]--
				if readyCount[successor] == 0 {
					o.mu.Lock()
					skip, depID := o.shouldSkip(successor)
					if skip {
						o.results[successor].Status = StatusSkipped
						o.results[successor].Error = fmt.Errorf("skipped due to failure in dependency '%s'", depID)
					}
					o.mu.Unlock()

					if skip {
						select {
						case completed <- successor:
						case <-actx.Done():
						}
					} else {
						select {
						case taskCh <- successor:
						case <-actx.Done():
						}
					}
				}
			}
		}
	}

	wg.Wait()

	o.mu.Lock()
	o.running = false
	endTime := time.Now()
	o.mu.Unlock()

	return o.buildReport(startTime, endTime)
}

func (o *Orchestrator) executeTask(ctx context.Context, taskID string, completed chan<- string) {
	o.mu.Lock()
	task := o.tasks[taskID]
	result := o.results[taskID]
	result.Status = StatusRunning
	result.StartTime = time.Now()
	o.mu.Unlock()

	var output interface{}
	var err error
	attempts := 0
	maxAttempts := 1 + task.MaxRetry

	for attempts < maxAttempts {
		attempts++

		var cancel context.CancelFunc
		runCtx := ctx
		if task.Timeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, task.Timeout)
		}

		done := make(chan struct{})
		var runOutput interface{}
		var runErr error

		go func() {
			defer close(done)
			defer func() {
				if r := recover(); r != nil {
					runErr = fmt.Errorf("task panicked: %v", r)
				}
			}()
			runOutput, runErr = task.Func(runCtx)
		}()

		select {
		case <-done:
			output = runOutput
			err = runErr
		case <-runCtx.Done():
			if ctx.Err() != nil {
				err = ctx.Err()
			} else {
				err = ErrTimeout
			}
		}

		if cancel != nil {
			cancel()
		}

		if err == nil {
			break
		}

		if ctx.Err() != nil {
			break
		}
	}

	o.mu.Lock()
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Attempts = attempts

	if err == nil {
		result.Status = StatusSuccess
		result.Output = output
	} else if errors.Is(err, ErrTimeout) {
		result.Status = StatusTimeout
		result.Error = err
	} else if ctx.Err() != nil {
		result.Status = StatusFailed
		result.Error = err
	} else {
		result.Status = StatusFailed
		result.Error = err
	}

	o.results[taskID] = result
	o.mu.Unlock()

	select {
	case completed <- taskID:
	case <-ctx.Done():
	}
}

func (o *Orchestrator) buildReport(startTime, endTime time.Time) *ExecutionReport {
	o.mu.Lock()
	defer o.mu.Unlock()

	success := true
	for _, result := range o.results {
		if result.Status == StatusFailed || result.Status == StatusTimeout {
			success = false
			break
		}
	}

	report := &ExecutionReport{
		Success:     success,
		StartTime:   startTime,
		EndTime:     endTime,
		Duration:    endTime.Sub(startTime),
		TaskResults: make(map[string]*TaskResult),
	}

	for id, result := range o.results {
		report.TaskResults[id] = &TaskResult{
			TaskID:    result.TaskID,
			Status:    result.Status,
			Output:    result.Output,
			Error:     result.Error,
			StartTime: result.StartTime,
			EndTime:   result.EndTime,
			Duration:  result.Duration,
			Attempts:  result.Attempts,
		}
	}

	return report
}

func (o *Orchestrator) collectDownstreamLocked(taskID string) map[string]bool {
	targets := make(map[string]bool)
	visited := make(map[string]bool)
	var queue []string

	queue = append(queue, taskID)
	visited[taskID] = true
	targets[taskID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		task := o.tasks[current]
		for _, successor := range task.Successors {
			if !visited[successor] {
				visited[successor] = true
				targets[successor] = true
				queue = append(queue, successor)
			}
		}
	}

	return targets
}

func (o *Orchestrator) RetryTask(ctx context.Context, taskID string) (*ExecutionReport, error) {
	o.mu.Lock()
	if o.running {
		o.mu.Unlock()
		return nil, ErrOrchestratorRunning
	}

	if _, exists := o.tasks[taskID]; !exists {
		o.mu.Unlock()
		return nil, ErrTaskNotFound
	}

	result, ok := o.results[taskID]
	if !ok || (result.Status != StatusFailed && result.Status != StatusTimeout && result.Status != StatusSkipped) {
		o.mu.Unlock()
		return nil, ErrCannotRetry
	}

	retryTargets := o.collectDownstreamLocked(taskID)

	for id := range retryTargets {
		o.results[id] = &TaskResult{
			TaskID: id,
			Status: StatusPending,
		}
	}

	o.running = true
	actx, cancel := context.WithCancel(ctx)
	o.cancel = cancel
	startTime := time.Now()
	o.mu.Unlock()

	report := o.runScheduler(actx, func() map[string]*Task {
		o.mu.Lock()
		defer o.mu.Unlock()
		subset := make(map[string]*Task)
		for id := range retryTargets {
			subset[id] = o.tasks[id]
		}
		return subset
	}(), startTime)

	return report, nil
}

func (o *Orchestrator) GetTask(id string) (*Task, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	task, exists := o.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	taskCopy := &Task{
		ID:           task.ID,
		Name:         task.Name,
		Timeout:      task.Timeout,
		MaxRetry:     task.MaxRetry,
		Dependencies: make([]string, len(task.Dependencies)),
		Successors:   make([]string, len(task.Successors)),
	}
	copy(taskCopy.Dependencies, task.Dependencies)
	copy(taskCopy.Successors, task.Successors)

	return taskCopy, nil
}

func (o *Orchestrator) GetTaskResult(id string) (*TaskResult, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	result, exists := o.results[id]
	if !exists {
		return nil, ErrTaskNotFound
	}

	resultCopy := &TaskResult{
		TaskID:    result.TaskID,
		Status:    result.Status,
		Output:    result.Output,
		Error:     result.Error,
		StartTime: result.StartTime,
		EndTime:   result.EndTime,
		Duration:  result.Duration,
		Attempts:  result.Attempts,
	}

	return resultCopy, nil
}

func (o *Orchestrator) TaskCount() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.tasks)
}

func (o *Orchestrator) Stop() {
	o.mu.Lock()
	if !o.running {
		o.mu.Unlock()
		return
	}
	cancel := o.cancel
	o.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}
