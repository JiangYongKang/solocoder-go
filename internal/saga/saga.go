package saga

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSagaNotFound         = errors.New("saga: saga not found")
	ErrSagaAlreadyExists    = errors.New("saga: saga already exists")
	ErrStepNotFound         = errors.New("saga: step not found")
	ErrStepAlreadyExists    = errors.New("saga: step already exists")
	ErrSagaRunning          = errors.New("saga: saga is currently running")
	ErrNoStepsRegistered    = errors.New("saga: no steps registered")
	ErrInvalidStepID        = errors.New("saga: invalid step id")
	ErrCompensationFailed   = errors.New("saga: compensation failed")
	ErrInterventionNotFound = errors.New("saga: intervention not found")
	ErrExecutionNotFound    = errors.New("saga: execution not found")
)

type OperationStatus int

const (
	StatusPending OperationStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
)

func (s OperationStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusRunning:
		return "Running"
	case StatusSuccess:
		return "Success"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

type OperationType int

const (
	OpTypeForward OperationType = iota
	OpTypeCompensation
)

func (t OperationType) String() string {
	switch t {
	case OpTypeForward:
		return "Forward"
	case OpTypeCompensation:
		return "Compensation"
	default:
		return "Unknown"
	}
}

type StepFunc func(ctx context.Context, data map[string]interface{}) (interface{}, error)

type Step struct {
	ID           string
	Name         string
	ForwardFunc  StepFunc
	CompensateFunc StepFunc
}

type StepResult struct {
	StepID      string
	Status      OperationStatus
	Output      interface{}
	Error       error
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
}

type LogEntry struct {
	ID            string
	TransactionID string
	StepID        string
	OperationType OperationType
	Status        OperationStatus
	Timestamp     time.Time
	Error         error
	Details       string
}

type CompensationFailure struct {
	TransactionID    string
	StepID           string
	ForwardStepID    string
	FailureReason    string
	Error            error
	FailureTime      time.Time
	Resolved         bool
	ResolutionNotes  string
}

type Saga struct {
	ID       string
	Name     string
	Steps    []*Step
	stepMap  map[string]*Step
}

type SagaExecution struct {
	ID                string
	SagaID            string
	Name              string
	Status            OperationStatus
	Steps             []*Step
	StepResults       map[string]*StepResult
	Compensations     map[string]*StepResult
	Data              map[string]interface{}
	Error             error
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	NeedsIntervention bool
	InterventionNotes []*CompensationFailure
}

type Coordinator struct {
	nextLogID            int64
	nextExecID           int64
	mu                   sync.RWMutex
	sagas                map[string]*Saga
	executions           map[string]*SagaExecution
	logs                 []*LogEntry
	logsByTransaction    map[string][]*LogEntry
	pendingInterventions []*CompensationFailure
	runningSagas         map[string]bool
}

func NewCoordinator() *Coordinator {
	return &Coordinator{
		sagas:                make(map[string]*Saga),
		executions:           make(map[string]*SagaExecution),
		logs:                 make([]*LogEntry, 0),
		logsByTransaction:    make(map[string][]*LogEntry),
		pendingInterventions: make([]*CompensationFailure, 0),
		runningSagas:         make(map[string]bool),
	}
}

func (c *Coordinator) NewSaga(id, name string) (*Saga, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.sagas[id]; exists {
		return nil, ErrSagaAlreadyExists
	}

	saga := &Saga{
		ID:      id,
		Name:    name,
		Steps:   make([]*Step, 0),
		stepMap: make(map[string]*Step),
	}

	c.sagas[id] = saga
	return saga, nil
}

func (s *Saga) AddStep(id, name string, forwardFunc StepFunc, compensateFunc StepFunc) error {
	if id == "" {
		return ErrInvalidStepID
	}

	if _, exists := s.stepMap[id]; exists {
		return ErrStepAlreadyExists
	}

	if forwardFunc == nil {
		return errors.New("saga: forward function cannot be nil")
	}

	step := &Step{
		ID:               id,
		Name:             name,
		ForwardFunc:      forwardFunc,
		CompensateFunc:   compensateFunc,
	}

	s.Steps = append(s.Steps, step)
	s.stepMap[id] = step
	return nil
}

func (s *Saga) GetStep(id string) (*Step, error) {
	step, exists := s.stepMap[id]
	if !exists {
		return nil, ErrStepNotFound
	}
	return step, nil
}

func (c *Coordinator) GetSaga(id string) (*Saga, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	saga, exists := c.sagas[id]
	if !exists {
		return nil, ErrSagaNotFound
	}
	return saga, nil
}

func (c *Coordinator) Execute(ctx context.Context, sagaID string, initialData map[string]interface{}) (*SagaExecution, error) {
	c.mu.Lock()
	saga, exists := c.sagas[sagaID]
	if !exists {
		c.mu.Unlock()
		return nil, ErrSagaNotFound
	}

	if len(saga.Steps) == 0 {
		c.mu.Unlock()
		return nil, ErrNoStepsRegistered
	}

	if c.runningSagas[sagaID] {
		c.mu.Unlock()
		return nil, ErrSagaRunning
	}

	c.runningSagas[sagaID] = true

	execID := atomic.AddInt64(&c.nextExecID, 1)
	executionID := fmt.Sprintf("%s-exec-%d", sagaID, execID)
	data := make(map[string]interface{})
	if initialData != nil {
		for k, v := range initialData {
			data[k] = v
		}
	}

	execution := &SagaExecution{
		ID:                executionID,
		SagaID:            sagaID,
		Name:              saga.Name,
		Status:            StatusRunning,
		Steps:             make([]*Step, len(saga.Steps)),
		StepResults:       make(map[string]*StepResult),
		Compensations:     make(map[string]*StepResult),
		Data:              data,
		StartTime:         time.Now(),
		NeedsIntervention: false,
		InterventionNotes: make([]*CompensationFailure, 0),
	}
	copy(execution.Steps, saga.Steps)

	c.executions[executionID] = execution
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.runningSagas, sagaID)
		c.mu.Unlock()
	}()

	c.logOperation(executionID, "", OpTypeForward, StatusRunning, nil, "Saga execution started")

	successfulSteps := make([]*Step, 0)
	var executionErr error

	for i, step := range saga.Steps {
		select {
		case <-ctx.Done():
			executionErr = ctx.Err()
			c.logOperation(executionID, step.ID, OpTypeForward, StatusFailed, ctx.Err(), "Forward operation cancelled")
			break
		default:
		}

		if executionErr != nil {
			break
		}

		result := c.executeForward(ctx, execution, step, i)
		execution.StepResults[step.ID] = result

		if result.Status == StatusSuccess {
			successfulSteps = append(successfulSteps, step)
			if result.Output != nil {
				data[step.ID] = result.Output
			}
		} else {
			executionErr = result.Error
			break
		}
	}

	if executionErr != nil {
		execution.Status = StatusFailed
		execution.Error = executionErr

		c.logOperation(executionID, "", OpTypeForward, StatusFailed, executionErr, "Saga execution failed, starting compensation")

		c.executeCompensations(ctx, execution, successfulSteps)

		if execution.NeedsIntervention {
			execution.Error = fmt.Errorf("%w: forward failed: %v", ErrCompensationFailed, executionErr)
		}
	} else {
		execution.Status = StatusSuccess
		c.logOperation(executionID, "", OpTypeForward, StatusSuccess, nil, "Saga execution completed successfully")
	}

	execution.EndTime = time.Now()
	execution.Duration = execution.EndTime.Sub(execution.StartTime)

	c.mu.Lock()
	c.executions[executionID] = execution
	c.mu.Unlock()

	return execution, nil
}

func (c *Coordinator) executeForward(ctx context.Context, execution *SagaExecution, step *Step, index int) *StepResult {
	c.logOperation(execution.ID, step.ID, OpTypeForward, StatusRunning, nil, fmt.Sprintf("Step %d: %s starting", index, step.Name))

	result := &StepResult{
		StepID:    step.ID,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}

	output, err := safeExecute(ctx, step.ForwardFunc, execution.Data)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		c.logOperation(execution.ID, step.ID, OpTypeForward, StatusFailed, err, fmt.Sprintf("Step %d: %s failed", index, step.Name))
	} else {
		result.Status = StatusSuccess
		result.Output = output
		c.logOperation(execution.ID, step.ID, OpTypeForward, StatusSuccess, nil, fmt.Sprintf("Step %d: %s completed successfully", index, step.Name))
	}

	return result
}

func (c *Coordinator) executeCompensations(ctx context.Context, execution *SagaExecution, successfulSteps []*Step) {
	for i := len(successfulSteps) - 1; i >= 0; i-- {
		step := successfulSteps[i]

		if step.CompensateFunc == nil {
			continue
		}

		compStepID := step.ID + "-compensate"
		compResult := c.executeCompensation(ctx, execution, step, compStepID)
		execution.Compensations[compStepID] = compResult

		if compResult.Status == StatusFailed {
			failure := &CompensationFailure{
				TransactionID: execution.ID,
				StepID:        compStepID,
				ForwardStepID: step.ID,
				FailureReason: compResult.Error.Error(),
				Error:         compResult.Error,
				FailureTime:   time.Now(),
				Resolved:      false,
			}

			execution.NeedsIntervention = true
			execution.InterventionNotes = append(execution.InterventionNotes, failure)

			c.mu.Lock()
			c.pendingInterventions = append(c.pendingInterventions, failure)
			c.mu.Unlock()

			c.logOperation(execution.ID, compStepID, OpTypeCompensation, StatusFailed, compResult.Error,
				fmt.Sprintf("Compensation for step %s failed, marked for manual intervention", step.Name))
		}
	}
}

func (c *Coordinator) executeCompensation(ctx context.Context, execution *SagaExecution, step *Step, compStepID string) *StepResult {
	c.logOperation(execution.ID, compStepID, OpTypeCompensation, StatusRunning, nil, fmt.Sprintf("Compensation for step %s starting", step.Name))

	result := &StepResult{
		StepID:    compStepID,
		Status:    StatusRunning,
		StartTime: time.Now(),
	}

	_, err := safeExecute(ctx, step.CompensateFunc, execution.Data)

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	if err != nil {
		result.Status = StatusFailed
		result.Error = err
		c.logOperation(execution.ID, compStepID, OpTypeCompensation, StatusFailed, err, fmt.Sprintf("Compensation for step %s failed", step.Name))
	} else {
		result.Status = StatusSuccess
		c.logOperation(execution.ID, compStepID, OpTypeCompensation, StatusSuccess, nil, fmt.Sprintf("Compensation for step %s completed successfully", step.Name))
	}

	return result
}

func safeExecute(ctx context.Context, fn StepFunc, data map[string]interface{}) (output interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("saga: operation panicked: %v", r)
		}
	}()

	return fn(ctx, data)
}

func (c *Coordinator) logOperation(transactionID, stepID string, opType OperationType, status OperationStatus, err error, details string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.nextLogID++
	entry := &LogEntry{
		ID:            fmt.Sprintf("log-%d", c.nextLogID),
		TransactionID: transactionID,
		StepID:        stepID,
		OperationType: opType,
		Status:        status,
		Timestamp:     time.Now(),
		Error:         err,
		Details:       details,
	}

	c.logs = append(c.logs, entry)
	c.logsByTransaction[transactionID] = append(c.logsByTransaction[transactionID], entry)
}

func (c *Coordinator) GetLogs(transactionID string) ([]*LogEntry, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	logs, exists := c.logsByTransaction[transactionID]
	if !exists {
		return nil, ErrExecutionNotFound
	}

	result := make([]*LogEntry, len(logs))
	copy(result, logs)
	return result, nil
}

func (c *Coordinator) GetAllLogs() []*LogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*LogEntry, len(c.logs))
	copy(result, c.logs)
	return result
}

func (c *Coordinator) GetPendingInterventions() []*CompensationFailure {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CompensationFailure, 0)
	for _, failure := range c.pendingInterventions {
		if !failure.Resolved {
			result = append(result, failure)
		}
	}
	return result
}

func (c *Coordinator) ResolveIntervention(transactionID, stepID, notes string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, failure := range c.pendingInterventions {
		if failure.TransactionID == transactionID && failure.StepID == stepID && !failure.Resolved {
			failure.Resolved = true
			failure.ResolutionNotes = notes

			execution, exists := c.executions[transactionID]
			if exists {
				allResolved := true
				for _, note := range execution.InterventionNotes {
					if !note.Resolved {
						allResolved = false
						break
					}
				}
				if allResolved {
					execution.NeedsIntervention = false
				}
			}

			return nil
		}
	}

	return ErrInterventionNotFound
}

func (c *Coordinator) GetExecution(executionID string) (*SagaExecution, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	execution, exists := c.executions[executionID]
	if !exists {
		return nil, ErrExecutionNotFound
	}
	return execution, nil
}

func (c *Coordinator) GetExecutionsBySaga(sagaID string) []*SagaExecution {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*SagaExecution, 0)
	for _, exec := range c.executions {
		if exec.SagaID == sagaID {
			result = append(result, exec)
		}
	}
	return result
}

func (c *Coordinator) RemoveSaga(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.sagas[id]; !exists {
		return ErrSagaNotFound
	}

	delete(c.sagas, id)
	return nil
}
