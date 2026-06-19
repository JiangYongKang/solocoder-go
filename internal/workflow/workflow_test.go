package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestExecutionContext_BasicOperations(t *testing.T) {
	ctx := NewExecutionContext()

	ctx.Set("key1", "value1")
	ctx.Set("key2", 42)
	ctx.Set("key3", 3.14)

	if val, ok := ctx.GetString("key1"); !ok || val != "value1" {
		t.Errorf("expected key1=value1, got %v", val)
	}

	if val, ok := ctx.GetInt("key2"); !ok || val != 42 {
		t.Errorf("expected key2=42, got %v", val)
	}

	if val, ok := ctx.Get("key3"); !ok || val.(float64) != 3.14 {
		t.Errorf("expected key3=3.14, got %v", val)
	}

	if _, ok := ctx.Get("nonexistent"); ok {
		t.Error("expected nonexistent key to return false")
	}
}

func TestExecutionContext_Clone(t *testing.T) {
	ctx := NewExecutionContext()
	ctx.Set("key1", "value1")
	ctx.Set("key2", 100)

	cloned := ctx.Clone()
	cloned.Set("key1", "modified")
	cloned.Set("key3", "new")

	if val, _ := ctx.GetString("key1"); val != "value1" {
		t.Error("original context should not be modified")
	}
	if val, _ := cloned.GetString("key1"); val != "modified" {
		t.Error("cloned context should have modified value")
	}
	if _, ok := ctx.Get("key3"); ok {
		t.Error("original should not have key3")
	}
}

func TestCondition_Evaluate(t *testing.T) {
	ctx := NewExecutionContext()
	ctx.Set("name", "test")
	ctx.Set("age", 25)
	ctx.Set("active", true)
	ctx.Set("score", 85.5)

	tests := []struct {
		name     string
		cond     Condition
		expected bool
	}{
		{"eq string", Condition{Field: "name", Operator: "eq", Value: "test"}, true},
		{"eq int", Condition{Field: "age", Operator: "==", Value: 25}, true},
		{"ne string", Condition{Field: "name", Operator: "ne", Value: "other"}, true},
		{"ne int", Condition{Field: "age", Operator: "!=", Value: 30}, true},
		{"contains", Condition{Field: "name", Operator: "contains", Value: "es"}, true},
		{"notcontains", Condition{Field: "name", Operator: "notcontains", Value: "xyz"}, true},
		{"gt", Condition{Field: "age", Operator: "gt", Value: 20}, true},
		{"gte", Condition{Field: "age", Operator: ">=", Value: 25}, true},
		{"lt", Condition{Field: "age", Operator: "lt", Value: 30}, true},
		{"lte", Condition{Field: "age", Operator: "<=", Value: 25}, true},
		{"eq fail", Condition{Field: "name", Operator: "eq", Value: "other"}, false},
		{"gt fail", Condition{Field: "age", Operator: "gt", Value: 30}, false},
		{"field not found", Condition{Field: "nonexistent", Operator: "eq", Value: "x"}, false},
		{"unknown operator", Condition{Field: "age", Operator: "invalid", Value: 25}, false},
		{"nil condition", Condition{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cond.Evaluate(ctx)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTaskNode_Execute(t *testing.T) {
	task := NewTaskNode("test", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "success", nil
	})

	result, err := task.Execute(context.Background(), NewExecutionContext())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if result.Output != "success" {
		t.Errorf("expected output 'success', got %v", result.Output)
	}
}

func TestTaskNode_ExecuteError(t *testing.T) {
	expectedErr := errors.New("task failed")
	task := NewTaskNode("test", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return nil, expectedErr
	})

	result, err := task.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error, got nil")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if result.Error != expectedErr.Error() {
		t.Errorf("expected error message %v, got %v", expectedErr.Error(), result.Error)
	}
}

func TestTaskNode_WithContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := NewTaskNode("test", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "success", nil
	})

	result, err := task.Execute(ctx, NewExecutionContext())
	if err == nil {
		t.Error("expected error for canceled context")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestSequentialNode_Execute(t *testing.T) {
	var execOrder []string

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execOrder = append(execOrder, "task1")
		execCtx.Set("task1_result", 1)
		return 1, nil
	})

	task2 := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execOrder = append(execOrder, "task2")
		val, _ := execCtx.GetInt("task1_result")
		execCtx.Set("task2_result", val+1)
		return 2, nil
	})

	task3 := NewCallbackTaskNode("task3", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execOrder = append(execOrder, "task3")
		val, _ := execCtx.GetInt("task2_result")
		return val + 1, nil
	})

	seq := NewSequentialNode("seq", task1, task2, task3)
	execCtx := NewExecutionContext()
	result, err := seq.Execute(context.Background(), execCtx)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if len(execOrder) != 3 || execOrder[0] != "task1" || execOrder[1] != "task2" || execOrder[2] != "task3" {
		t.Errorf("expected execution order [task1 task2 task3], got %v", execOrder)
	}

	finalVal, _ := execCtx.GetInt("task2_result")
	if finalVal != 2 {
		t.Errorf("expected context value 2, got %v", finalVal)
	}
}

func TestSequentialNode_StopOnFailure(t *testing.T) {
	var execOrder []string

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execOrder = append(execOrder, "task1")
		return 1, nil
	})

	task2 := NewFailTask("task2", errors.New("task2 failed"))

	task3 := NewCallbackTaskNode("task3", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execOrder = append(execOrder, "task3")
		return 3, nil
	})

	seq := NewSequentialNode("seq", task1, task2, task3)
	result, err := seq.Execute(context.Background(), NewExecutionContext())

	if err == nil {
		t.Error("expected error, got nil")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if len(execOrder) != 1 || execOrder[0] != "task1" {
		t.Errorf("expected only task1 executed, got %v", execOrder)
	}
}

func TestSequentialNode_EmptyNodes(t *testing.T) {
	seq := NewSequentialNode("seq")
	result, err := seq.Execute(context.Background(), NewExecutionContext())

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
}

func TestParallelNode_Execute(t *testing.T) {
	var mu sync.Mutex
	var execOrder []string

	createTask := func(name string, delay time.Duration) *CallbackTaskNode {
		return NewCallbackTaskNode(name, func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			time.Sleep(delay)
			mu.Lock()
			execOrder = append(execOrder, name)
			mu.Unlock()
			return name, nil
		})
	}

	task1 := createTask("task1", 50*time.Millisecond)
	task2 := createTask("task2", 10*time.Millisecond)
	task3 := createTask("task3", 30*time.Millisecond)

	parallel := NewParallelNode("parallel", task1, task2, task3)
	parallel.SetMaxFailures(-1)

	start := time.Now()
	result, err := parallel.Execute(context.Background(), NewExecutionContext())
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected parallel execution within 200ms, took %v", elapsed)
	}

	mu.Lock()
	if len(execOrder) != 3 {
		t.Errorf("expected 3 tasks executed, got %v", execOrder)
	}
	mu.Unlock()
}

func TestParallelNode_MaxFailures(t *testing.T) {
	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "success1", nil
	})
	task2 := NewFailTask("task2", errors.New("failed2"))
	task3 := NewFailTask("task3", errors.New("failed3"))
	task4 := NewDelayTask("task4", 100*time.Millisecond, "success4")

	parallel := NewParallelNode("parallel", task1, task2, task3, task4)
	parallel.SetMaxFailures(1)

	start := time.Now()
	result, err := parallel.Execute(context.Background(), NewExecutionContext())
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected error due to max failures exceeded")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("expected early termination, but took %v", elapsed)
	}
}

func TestParallelNode_Timeout(t *testing.T) {
	task1 := NewDelayTask("task1", 200*time.Millisecond, "result1")
	task2 := NewDelayTask("task2", 10*time.Millisecond, "result2")

	parallel := NewParallelNode("parallel", task1, task2)
	parallel.SetTimeout(50 * time.Millisecond)
	parallel.SetMaxFailures(-1)

	start := time.Now()
	result, err := parallel.Execute(context.Background(), NewExecutionContext())
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > 150*time.Millisecond {
		t.Errorf("expected timeout around 50ms, took %v", elapsed)
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestConditionalNode_WithDefaultBranch(t *testing.T) {
	taskA := NewCallbackTaskNode("branchA", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "branchA executed", nil
	})
	taskB := NewCallbackTaskNode("branchB", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "branchB executed", nil
	})
	defaultTask := NewCallbackTaskNode("default", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "default executed", nil
	})

	cond := NewConditionalNode("conditional")
	cond.AddBranch(&Condition{Field: "type", Operator: "eq", Value: "A"}, taskA)
	cond.AddBranch(&Condition{Field: "type", Operator: "eq", Value: "B"}, taskB)
	cond.SetDefaultBranch(defaultTask)

	execCtx := NewExecutionContext()
	execCtx.Set("type", "A")
	result, err := cond.Execute(context.Background(), execCtx)
	if err != nil {
		t.Errorf("expected no error for type=A, got %v", err)
	}
	output := result.Output.(map[string]interface{})
	if output["selected_branch"] != "branchA" {
		t.Errorf("expected branchA selected, got %v", output["selected_branch"])
	}

	execCtx2 := NewExecutionContext()
	execCtx2.Set("type", "C")
	result2, err := cond.Execute(context.Background(), execCtx2)
	if err != nil {
		t.Errorf("expected no error for type=C, got %v", err)
	}
	output2 := result2.Output.(map[string]interface{})
	if output2["default_branch"] != true {
		t.Errorf("expected default branch to be taken, got %v", output2)
	}
}

func TestConditionalNode_NoDefaultNoMatch(t *testing.T) {
	taskA := NewCallbackTaskNode("branchA", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "branchA executed", nil
	})

	cond := NewConditionalNode("conditional")
	cond.AddBranch(&Condition{Field: "type", Operator: "eq", Value: "A"}, taskA)

	execCtx := NewExecutionContext()
	execCtx.Set("type", "B")
	result, err := cond.Execute(context.Background(), execCtx)

	if err == nil {
		t.Error("expected error when no branch matches and no default")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestConditionalNode_BranchFails(t *testing.T) {
	failTask := NewFailTask("failBranch", errors.New("branch failed"))

	cond := NewConditionalNode("conditional")
	cond.AddBranch(nil, failTask)

	result, err := cond.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error from failing branch")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
}

func TestLoopNode_FixedIterations(t *testing.T) {
	var counter int32

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		count, _ := execCtx.GetInt("iteration_count")
		atomic.AddInt32(&counter, 1)
		return map[string]int{"idx": idx, "count": count}, nil
	})

	loop := NewLoopNode("loop", task)
	loop.SetFixedIterations(5)

	result, err := loop.Execute(context.Background(), NewExecutionContext())
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if atomic.LoadInt32(&counter) != 5 {
		t.Errorf("expected 5 iterations, got %d", counter)
	}

	output := result.Output.(map[string]interface{})
	if output["total_iterations"] != 5 {
		t.Errorf("expected total_iterations=5, got %v", output["total_iterations"])
	}
}

func TestLoopNode_DynamicIterations(t *testing.T) {
	var iterations []int

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		iterations = append(iterations, idx)
		return idx, nil
	})

	loop := NewLoopNode("loop", task)
	loop.SetDynamicIterations("repeat_count")

	execCtx := NewExecutionContext()
	execCtx.Set("repeat_count", 3)

	_, err := loop.Execute(context.Background(), execCtx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if len(iterations) != 3 {
		t.Errorf("expected 3 iterations, got %v", iterations)
	}
	if iterations[0] != 0 || iterations[1] != 1 || iterations[2] != 2 {
		t.Errorf("expected iteration indices [0,1,2], got %v", iterations)
	}
}

func TestLoopNode_ContinueOnError(t *testing.T) {
	var executed []int

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		executed = append(executed, idx)
		if idx == 1 {
			return nil, fmt.Errorf("error at iteration %d", idx)
		}
		return idx, nil
	})

	loop := NewLoopNode("loop", task)
	loop.SetFixedIterations(3)
	loop.SetContinueOnError(true)

	result, err := loop.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error due to iteration failure")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if len(executed) != 3 {
		t.Errorf("expected all 3 iterations to execute, got %v", executed)
	}
}

func TestLoopNode_StopOnError(t *testing.T) {
	var executed []int

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		executed = append(executed, idx)
		if idx == 1 {
			return nil, fmt.Errorf("error at iteration %d", idx)
		}
		return idx, nil
	})

	loop := NewLoopNode("loop", task)
	loop.SetFixedIterations(5)
	loop.SetContinueOnError(false)

	result, err := loop.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error due to iteration failure")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if len(executed) != 2 {
		t.Errorf("expected only 2 iterations executed, got %v", executed)
	}
}

func TestLoopNode_ContextPropagation(t *testing.T) {
	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		existing, _ := execCtx.GetInt("sum")
		execCtx.Set("sum", existing+idx)
		return nil, nil
	})

	loop := NewLoopNode("loop", task)
	loop.SetFixedIterations(5)

	execCtx := NewExecutionContext()
	execCtx.Set("sum", 0)

	_, err := loop.Execute(context.Background(), execCtx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	sum, _ := execCtx.GetInt("sum")
	if sum != 10 {
		t.Errorf("expected sum=10 (0+1+2+3+4), got %d", sum)
	}
}

func TestNodeRetry_SuccessAfterRetries(t *testing.T) {
	flaky := NewFlakyTask("flaky", 2, "success")
	flaky.SetRetryConfig(RetryConfig{
		MaxRetries: 3,
		Interval:   1 * time.Millisecond,
		Strategy:   RetryFixed,
	})

	result, err := flaky.Execute(context.Background(), NewExecutionContext())
	if err != nil {
		t.Errorf("expected no error after retries, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if result.RetryCount != 2 {
		t.Errorf("expected 2 retries, got %d", result.RetryCount)
	}
	if result.Output != "success" {
		t.Errorf("expected output 'success', got %v", result.Output)
	}
}

func TestNodeRetry_ExhaustRetries(t *testing.T) {
	flaky := NewFlakyTask("flaky", 5, "success")
	flaky.SetRetryConfig(RetryConfig{
		MaxRetries: 2,
		Interval:   1 * time.Millisecond,
		Strategy:   RetryFixed,
	})

	result, err := flaky.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if result.RetryCount != 2 {
		t.Errorf("expected 2 retries, got %d", result.RetryCount)
	}
}

func TestNodeRetry_NoRetryConfig(t *testing.T) {
	flaky := NewFlakyTask("flaky", 2, "success")

	result, err := flaky.Execute(context.Background(), NewExecutionContext())
	if err == nil {
		t.Error("expected error without retries")
	}
	if result.RetryCount != 0 {
		t.Errorf("expected 0 retries, got %d", result.RetryCount)
	}
}

func TestNodeRetry_ExponentialBackoff(t *testing.T) {
	var attempts []time.Time
	mu := sync.Mutex{}

	task := NewCallbackTaskNode("test", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		attempts = append(attempts, time.Now())
		mu.Unlock()
		return nil, errors.New("fail")
	})

	task.SetRetryConfig(RetryConfig{
		MaxRetries:      2,
		Interval:        10 * time.Millisecond,
		Strategy:        RetryExponential,
		BackoffFactor:   2.0,
	})

	start := time.Now()
	task.Execute(context.Background(), NewExecutionContext())
	total := time.Since(start)

	mu.Lock()
	defer mu.Unlock()

	if len(attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", len(attempts))
	}

	if total < 30*time.Millisecond {
		t.Errorf("expected total time >= 30ms, got %v", total)
	}
}

func TestRetryInterval_Calculation(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RetryConfig
		attempt  int
		expected time.Duration
	}{
		{
			name:    "fixed",
			cfg:     RetryConfig{Interval: 100 * time.Millisecond, Strategy: RetryFixed},
			attempt: 3,
			expected: 100 * time.Millisecond,
		},
		{
			name:    "linear factor 1",
			cfg:     RetryConfig{Interval: 100 * time.Millisecond, Strategy: RetryLinear, BackoffFactor: 1},
			attempt: 3,
			expected: 300 * time.Millisecond,
		},
		{
			name:    "linear factor 2",
			cfg:     RetryConfig{Interval: 100 * time.Millisecond, Strategy: RetryLinear, BackoffFactor: 2},
			attempt: 3,
			expected: 600 * time.Millisecond,
		},
		{
			name:    "exponential factor 2",
			cfg:     RetryConfig{Interval: 100 * time.Millisecond, Strategy: RetryExponential, BackoffFactor: 2},
			attempt: 3,
			expected: 400 * time.Millisecond,
		},
		{
			name:    "zero interval immediate retry",
			cfg:     RetryConfig{Strategy: RetryFixed, Interval: 0},
			attempt: 1,
			expected: 0,
		},
		{
			name:    "negative interval default",
			cfg:     RetryConfig{Strategy: RetryFixed, Interval: -1},
			attempt: 1,
			expected: 100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateRetryInterval(tt.cfg, tt.attempt)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestWorkflowEngine_RegisterAndExecute(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewCallbackTaskNode("testTask", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		return "done", nil
	})

	wf := &WorkflowDefinition{
		ID:       "wf1",
		Name:     "Test Workflow",
		RootNode: task,
	}

	err := engine.RegisterWorkflow(wf)
	if err != nil {
		t.Fatalf("failed to register workflow: %v", err)
	}

	result, err := engine.ExecuteWorkflow(context.Background(), "wf1", nil)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v", result.Status)
	}
	if result.WorkflowID != "wf1" {
		t.Errorf("expected workflow ID wf1, got %v", result.WorkflowID)
	}
}

func TestWorkflowEngine_RegisterValidation(t *testing.T) {
	engine := NewWorkflowEngine()

	err := engine.RegisterWorkflow(nil)
	if err == nil {
		t.Error("expected error for nil workflow")
	}

	err = engine.RegisterWorkflow(&WorkflowDefinition{ID: "", RootNode: NewTaskNode("t", nil)})
	if err == nil {
		t.Error("expected error for empty ID")
	}

	err = engine.RegisterWorkflow(&WorkflowDefinition{ID: "wf1", RootNode: nil})
	if err == nil {
		t.Error("expected error for nil root node")
	}
}

func TestWorkflowEngine_ExecuteUnknownWorkflow(t *testing.T) {
	engine := NewWorkflowEngine()
	_, err := engine.ExecuteWorkflow(context.Background(), "unknown", nil)
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestWorkflowEngine_InitialContext(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		val, _ := execCtx.GetString("user")
		return val, nil
	})

	wf := &WorkflowDefinition{
		ID:       "wf_ctx",
		RootNode: task,
	}
	engine.RegisterWorkflow(wf)

	result, err := engine.ExecuteWorkflow(context.Background(), "wf_ctx", map[string]interface{}{
		"user": "testuser",
		"env":  "prod",
	})
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	if result.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed, got %v", result.Status)
	}
}

func TestWorkflowEngine_StatePersistence(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		val, _ := execCtx.GetInt("count")
		execCtx.Set("count", val+1)
		return val + 1, nil
	})

	wf := &WorkflowDefinition{
		ID:       "wf_persist",
		RootNode: task,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "wf_persist", map[string]interface{}{
		"count": 41,
	})
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	if state.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed, got %v", state.Status)
	}

	data, err := engine.SaveState(state)
	if err != nil {
		t.Fatalf("save state failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("saved state data is empty")
	}

	loadedState, err := engine.LoadState(data)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}
	if loadedState.WorkflowID != "wf_persist" {
		t.Errorf("expected workflow ID wf_persist, got %v", loadedState.WorkflowID)
	}
	if loadedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v", loadedState.Status)
	}
	countVal := loadedState.Context["count"]
	var countInt int
	switch v := countVal.(type) {
	case int:
		countInt = v
	case float64:
		countInt = int(v)
	default:
		t.Errorf("unexpected type for count: %T", countVal)
	}
	if countInt != 42 {
		t.Errorf("expected context count=42, got %v", countVal)
	}
}

func TestWorkflowEngine_ComplexWorkflow(t *testing.T) {
	engine := NewWorkflowEngine()

	setup := NewSetContextTask("setup", "value", 10)

	checkValue := NewCallbackTaskNode("check", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		val, _ := execCtx.GetInt("value")
		execCtx.Set("check_result", val > 5)
		return val > 5, nil
	})

	branchTrue := NewCallbackTaskNode("true_branch", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		val, _ := execCtx.GetInt("value")
		return val * 2, nil
	})

	branchFalse := NewCallbackTaskNode("false_branch", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		val, _ := execCtx.GetInt("value")
		return val + 10, nil
	})

	conditional := NewConditionalNode("conditional")
	conditional.AddBranch(&Condition{Field: "check_result", Operator: "eq", Value: true}, branchTrue)
	conditional.SetDefaultBranch(branchFalse)

	parallelTasks := NewParallelNode("parallel",
		NewCallbackTaskNode("p1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			return "p1_done", nil
		}),
		NewCallbackTaskNode("p2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			return "p2_done", nil
		}),
	)
	parallelTasks.SetMaxFailures(-1)

	loopTask := NewCallbackTaskNode("loop_task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		idx, _ := execCtx.GetInt("iteration_index")
		execCtx.Set(fmt.Sprintf("loop_%d", idx), idx)
		return idx, nil
	})
	loop := NewLoopNode("loop", loopTask)
	loop.SetFixedIterations(3)

	root := NewSequentialNode("root", setup, checkValue, conditional, parallelTasks, loop)

	wf := &WorkflowDefinition{
		ID:       "complex_wf",
		Name:     "Complex Workflow",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	result, err := engine.ExecuteWorkflow(context.Background(), "complex_wf", nil)
	if err != nil {
		t.Fatalf("workflow execution failed: %v", err)
	}
	if result.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v (error: %v)", result.Status, result.Error)
	}
	if len(result.Results) < 5 {
		t.Errorf("expected at least 5 results, got %d", len(result.Results))
	}
}

func TestWorkflowEngine_FailingWorkflow(t *testing.T) {
	engine := NewWorkflowEngine()

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execCtx.Set("step", 1)
		return 1, nil
	})

	task2 := NewFailTask("task2", errors.New("critical failure"))

	task3 := NewCallbackTaskNode("task3", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execCtx.Set("step", 3)
		return 3, nil
	})

	root := NewSequentialNode("root", task1, task2, task3)

	wf := &WorkflowDefinition{
		ID:       "failing_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	result, err := engine.ExecuteWorkflow(context.Background(), "failing_wf", nil)
	if err == nil {
		t.Error("expected error from failing workflow")
	}
	if result.Status != WorkflowStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestWorkflowEngine_ConcurrentExecution(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewCallbackTaskNode("task", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "done", nil
	})

	wf := &WorkflowDefinition{
		ID:       "concurrent_wf",
		RootNode: task,
	}
	engine.RegisterWorkflow(wf)

	var wg sync.WaitGroup
	var successCount int32
	const goroutines = 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := engine.ExecuteWorkflow(context.Background(), "concurrent_wf", nil)
			if err == nil && result.Status == WorkflowStatusCompleted {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()
	if successCount != goroutines {
		t.Errorf("expected %d successful executions, got %d", goroutines, successCount)
	}
}

func TestWorkflowEngine_WorkflowManagement(t *testing.T) {
	engine := NewWorkflowEngine()

	wf1 := &WorkflowDefinition{ID: "wf1", RootNode: NewTaskNode("t", nil)}
	wf2 := &WorkflowDefinition{ID: "wf2", RootNode: NewTaskNode("t", nil)}

	engine.RegisterWorkflow(wf1)
	engine.RegisterWorkflow(wf2)

	list := engine.ListWorkflows()
	if len(list) != 2 {
		t.Errorf("expected 2 workflows, got %d", len(list))
	}

	_, ok := engine.GetWorkflow("wf1")
	if !ok {
		t.Error("expected to find wf1")
	}

	err := engine.RemoveWorkflow("wf1")
	if err != nil {
		t.Errorf("remove failed: %v", err)
	}

	list = engine.ListWorkflows()
	if len(list) != 1 {
		t.Errorf("expected 1 workflow after removal, got %d", len(list))
	}

	err = engine.RemoveWorkflow("nonexistent")
	if err != ErrWorkflowNotFound {
		t.Errorf("expected ErrWorkflowNotFound, got %v", err)
	}
}

func TestWorkflowEngine_ContextCancelDuringExecution(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewDelayTask("long_task", 500*time.Millisecond, "done")
	wf := &WorkflowDefinition{
		ID:       "cancel_wf",
		RootNode: task,
	}
	engine.RegisterWorkflow(wf)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := engine.ExecuteWorkflow(ctx, "cancel_wf", nil)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected error due to cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("workflow did not cancel")
	}
}

func TestBaseNode_Properties(t *testing.T) {
	task := NewTaskNode("test", nil)

	if task.GetID() == "" {
		t.Error("expected non-empty ID")
	}
	if task.GetType() != NodeTypeTask {
		t.Errorf("expected task type, got %v", task.GetType())
	}
	if task.GetName() != "test" {
		t.Errorf("expected name 'test', got %v", task.GetName())
	}

	task.SetName("newname")
	if task.GetName() != "newname" {
		t.Errorf("expected name 'newname', got %v", task.GetName())
	}

	cfg := RetryConfig{MaxRetries: 5}
	task.SetRetryConfig(cfg)
	if task.GetRetryConfig().MaxRetries != 5 {
		t.Errorf("expected MaxRetries 5, got %v", task.GetRetryConfig().MaxRetries)
	}
}

func TestNodeIDGeneration(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateNodeID()
		if ids[id] {
			t.Errorf("duplicate node ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestSetContextTask(t *testing.T) {
	task := NewSetContextTask("set", "key1", "value1")
	execCtx := NewExecutionContext()

	result, err := task.Execute(context.Background(), execCtx)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Status != NodeStatusCompleted {
		t.Errorf("expected completed, got %v", result.Status)
	}

	val, ok := execCtx.GetString("key1")
	if !ok || val != "value1" {
		t.Errorf("expected key1=value1, got %v", val)
	}
}

func TestDelayTask(t *testing.T) {
	task := NewDelayTask("delay", 50*time.Millisecond, "result")

	start := time.Now()
	result, err := task.Execute(context.Background(), NewExecutionContext())
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result.Output != "result" {
		t.Errorf("expected output 'result', got %v", result.Output)
	}
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected delay around 50ms, got %v", elapsed)
	}
}

func TestDelayTask_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	task := NewDelayTask("delay", 500*time.Millisecond, "result")

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := task.Execute(ctx, NewExecutionContext())
	if err == nil {
		t.Error("expected error due to canceled context")
	}
}

func TestWorkflowEngine_InstanceManagement(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewTaskNode("t", nil)
	wf := &WorkflowDefinition{ID: "wf1", RootNode: task}
	engine.RegisterWorkflow(wf)

	_, _ = engine.ExecuteWorkflowWithState(context.Background(), "wf1", nil)

	instances := engine.ListInstances()
	if len(instances) == 0 {
		t.Error("expected at least one instance")
	}

	for _, id := range instances {
		_, ok := engine.GetInstanceState(id)
		if !ok {
			t.Errorf("expected to find instance %s", id)
		}

		err := engine.RemoveInstance(id)
		if err != nil {
			t.Errorf("failed to remove instance: %v", err)
		}
	}

	instances = engine.ListInstances()
	if len(instances) != 0 {
		t.Error("expected no instances after removal")
	}
}

func TestWorkflowEngine_SaveStateValidation(t *testing.T) {
	engine := NewWorkflowEngine()

	_, err := engine.SaveState(nil)
	if err == nil {
		t.Error("expected error for nil state")
	}

	_, err = engine.LoadState([]byte{})
	if err == nil {
		t.Error("expected error for empty data")
	}

	_, err = engine.LoadState([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid json")
	}
}

func TestCompareNumbers(t *testing.T) {
	tests := []struct {
		a        interface{}
		b        interface{}
		expected int
	}{
		{10, 5, 1},
		{5, 10, -1},
		{5, 5, 0},
		{10.5, 5.5, 1},
		{"10", "5", 1},
		{"abc", 5, -2},
		{true, 5, -2},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v vs %v", tt.a, tt.b), func(t *testing.T) {
			result := compareNumbers(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestResumeWorkflow_Sequential_ResumeFromBreakpoint(t *testing.T) {
	engine := NewWorkflowEngine()

	var executionCount map[string]int
	var mu sync.Mutex
	executionCount = make(map[string]int)

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		executionCount["task1"]++
		mu.Unlock()
		execCtx.Set("task1_done", true)
		return "task1_result", nil
	})

	task2 := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		executionCount["task2"]++
		mu.Unlock()
		execCtx.Set("task2_done", true)
		return nil, fmt.Errorf("task2 failed intentionally")
	})

	task3 := NewCallbackTaskNode("task3", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		executionCount["task3"]++
		mu.Unlock()
		execCtx.Set("task3_done", true)
		return "task3_result", nil
	})

	root := NewSequentialNode("root", task1, task2, task3)

	wf := &WorkflowDefinition{
		ID:       "resume_seq_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "resume_seq_wf", nil)
	if err == nil {
		t.Fatal("expected error from failing workflow")
	}
	if state.Status != WorkflowStatusFailed {
		t.Errorf("expected failed status, got %v", state.Status)
	}

	mu.Lock()
	if executionCount["task1"] != 1 {
		t.Errorf("expected task1 to execute once, got %d", executionCount["task1"])
	}
	if executionCount["task2"] != 1 {
		t.Errorf("expected task2 to execute once, got %d", executionCount["task2"])
	}
	if executionCount["task3"] != 0 {
		t.Errorf("expected task3 to not execute, got %d", executionCount["task3"])
	}
	mu.Unlock()

	task1Done, _ := state.Context["task1_done"].(bool)
	if !task1Done {
		t.Error("expected task1_done in context")
	}

	task2Fixed := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		executionCount["task2"]++
		mu.Unlock()
		execCtx.Set("task2_done", true)
		return "task2_fixed_result", nil
	})

	task2.ID = root.Nodes[1].GetID()
	root.Nodes[1] = task2Fixed

	state.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), state)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v (error: %v)", resumedState.Status, resumedState.Error)
	}

	mu.Lock()
	if executionCount["task1"] != 1 {
		t.Errorf("task1 should not be re-executed, expected 1 got %d", executionCount["task1"])
	}
	if executionCount["task2"] != 2 {
		t.Errorf("task2 should be executed again, expected 2 got %d", executionCount["task2"])
	}
	if executionCount["task3"] != 1 {
		t.Errorf("task3 should be executed once, expected 1 got %d", executionCount["task3"])
	}
	mu.Unlock()

	task1Done, _ = resumedState.Context["task1_done"].(bool)
	if !task1Done {
		t.Error("task1_done should persist after resume")
	}
	task2Done, _ := resumedState.Context["task2_done"].(bool)
	if !task2Done {
		t.Error("task2_done should be set after resume")
	}
	task3Done, _ := resumedState.Context["task3_done"].(bool)
	if !task3Done {
		t.Error("task3_done should be set after resume")
	}

	if len(resumedState.CompletedNodes) < 3 {
		t.Errorf("expected at least 3 completed nodes, got %d", len(resumedState.CompletedNodes))
	}
}

func TestResumeWorkflow_ContextVariablesPreserved(t *testing.T) {
	engine := NewWorkflowEngine()

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execCtx.Set("user_id", 123)
		execCtx.Set("username", "testuser")
		execCtx.Set("settings", map[string]string{"theme": "dark"})
		return nil, fmt.Errorf("fail after setting context")
	})

	task2 := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		userId, _ := execCtx.GetInt("user_id")
		username, _ := execCtx.GetString("username")
		execCtx.Set("verified", true)
		return map[string]interface{}{"user_id": userId, "username": username}, nil
	})

	root := NewSequentialNode("root", task1, task2)

	wf := &WorkflowDefinition{
		ID:       "resume_ctx_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "resume_ctx_wf", nil)
	if err == nil {
		t.Fatal("expected error from failing workflow")
	}

	userId, _ := state.Context["user_id"]
	if userId == nil {
		t.Error("user_id should be in failed state context")
	}
	username, _ := state.Context["username"]
	if username == nil {
		t.Error("username should be in failed state context")
	}

	task1Fixed := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execCtx.Set("user_id", 123)
		execCtx.Set("username", "testuser")
		execCtx.Set("settings", map[string]string{"theme": "dark"})
		return "task1 done", nil
	})
	task1Fixed.ID = root.Nodes[0].GetID()
	root.Nodes[0] = task1Fixed
	state.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), state)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v (error: %v)", resumedState.Status, resumedState.Error)
	}

	resumedUserId, _ := resumedState.Context["user_id"]
	var userIdInt int
	switch v := resumedUserId.(type) {
	case int:
		userIdInt = v
	case float64:
		userIdInt = int(v)
	}
	if userIdInt != 123 {
		t.Errorf("user_id should be preserved, expected 123 got %v", resumedUserId)
	}

	resumedUsername, _ := resumedState.Context["username"].(string)
	if resumedUsername != "testuser" {
		t.Errorf("username should be preserved, expected 'testuser' got '%v'", resumedUsername)
	}

	verified, _ := resumedState.Context["verified"].(bool)
	if !verified {
		t.Error("verified should be set after resume")
	}
}

func TestResumeWorkflow_CompletedNodesNotReexecuted(t *testing.T) {
	engine := NewWorkflowEngine()

	var executedNodes []string
	var mu sync.Mutex

	createTrackedTask := func(name string) *CallbackTaskNode {
		return NewCallbackTaskNode(name, func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			mu.Lock()
			executedNodes = append(executedNodes, name)
			mu.Unlock()
			execCtx.Set(fmt.Sprintf("%s_done", name), true)
			return name, nil
		})
	}

	task1 := createTrackedTask("task1")
	task2 := createTrackedTask("task2")
	failTask := NewFailTask("failTask", errors.New("intentional failure"))
	task3 := createTrackedTask("task3")
	task4 := createTrackedTask("task4")

	root := NewSequentialNode("root", task1, task2, failTask, task3, task4)

	wf := &WorkflowDefinition{
		ID:       "no_reexec_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "no_reexec_wf", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	mu.Lock()
	initialExecution := make([]string, len(executedNodes))
	copy(initialExecution, executedNodes)
	mu.Unlock()

	if len(initialExecution) != 2 || initialExecution[0] != "task1" || initialExecution[1] != "task2" {
		t.Errorf("expected [task1 task2] executed initially, got %v", initialExecution)
	}

	fixedTask := createTrackedTask("failTask")
	fixedTask.ID = root.Nodes[2].GetID()
	root.Nodes[2] = fixedTask

	state.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), state)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}

	mu.Lock()
	finalExecution := make([]string, len(executedNodes))
	copy(finalExecution, executedNodes)
	mu.Unlock()

	task1Count := 0
	task2Count := 0
	for _, name := range finalExecution {
		if name == "task1" {
			task1Count++
		}
		if name == "task2" {
			task2Count++
		}
	}

	if task1Count != 1 {
		t.Errorf("task1 should be executed exactly once, got %d times. Full execution: %v", task1Count, finalExecution)
	}
	if task2Count != 1 {
		t.Errorf("task2 should be executed exactly once, got %d times. Full execution: %v", task2Count, finalExecution)
	}

	if len(resumedState.CompletedNodes) != 6 {
		t.Errorf("expected 6 completed nodes, got %d: %v", len(resumedState.CompletedNodes), resumedState.CompletedNodes)
	}

	task1Done, _ := resumedState.Context["task1_done"].(bool)
	if !task1Done {
		t.Error("task1_done should be true")
	}
	task2Done, _ := resumedState.Context["task2_done"].(bool)
	if !task2Done {
		t.Error("task2_done should be true")
	}
	task3Done, _ := resumedState.Context["task3_done"].(bool)
	if !task3Done {
		t.Error("task3_done should be true")
	}
	task4Done, _ := resumedState.Context["task4_done"].(bool)
	if !task4Done {
		t.Error("task4_done should be true")
	}
}

func TestResumeWorkflow_ParallelResume(t *testing.T) {
	engine := NewWorkflowEngine()

	var mu sync.Mutex
	executionCount := make(map[string]int)

	createTask := func(name string, shouldFail bool) *CallbackTaskNode {
		return NewCallbackTaskNode(name, func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
			mu.Lock()
			executionCount[name]++
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			if shouldFail {
				return nil, fmt.Errorf("%s failed", name)
			}
			return name, nil
		})
	}

	task1 := createTask("p_task1", false)
	task2 := createTask("p_task2", true)
	task3 := createTask("p_task3", false)

	parallel := NewParallelNode("parallel", task1, task2, task3)
	parallel.SetMaxFailures(0)

	wf := &WorkflowDefinition{
		ID:       "resume_parallel_wf",
		RootNode: parallel,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "resume_parallel_wf", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	mu.Lock()
	if executionCount["p_task1"] != 1 {
		t.Errorf("expected p_task1=1, got %d", executionCount["p_task1"])
	}
	if executionCount["p_task2"] != 1 {
		t.Errorf("expected p_task2=1, got %d", executionCount["p_task2"])
	}
	if executionCount["p_task3"] != 1 {
		t.Errorf("expected p_task3=1, got %d", executionCount["p_task3"])
	}
	mu.Unlock()

	task2Fixed := createTask("p_task2", false)
	task2Fixed.ID = parallel.Nodes[1].GetID()
	parallel.Nodes[1] = task2Fixed

	state.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), state)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed, got %v: %v", resumedState.Status, resumedState.Error)
	}

	mu.Lock()
	if executionCount["p_task1"] != 1 {
		t.Errorf("p_task1 should not re-execute, expected 1 got %d", executionCount["p_task1"])
	}
	if executionCount["p_task2"] != 2 {
		t.Errorf("p_task2 should execute again, expected 2 got %d", executionCount["p_task2"])
	}
	if executionCount["p_task3"] != 1 {
		t.Errorf("p_task3 should not re-execute, expected 1 got %d", executionCount["p_task3"])
	}
	mu.Unlock()
}

func TestResumeWorkflow_SaveLoadResume(t *testing.T) {
	engine := NewWorkflowEngine()

	var execOrder []string
	var mu sync.Mutex

	task1 := NewCallbackTaskNode("task1", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		execOrder = append(execOrder, "task1")
		mu.Unlock()
		execCtx.Set("step", 1)
		return 1, nil
	})

	task2 := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		execOrder = append(execOrder, "task2")
		mu.Unlock()
		return nil, fmt.Errorf("task2 failed")
	})

	task3 := NewCallbackTaskNode("task3", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		execOrder = append(execOrder, "task3")
		mu.Unlock()
		execCtx.Set("step", 3)
		return 3, nil
	})

	root := NewSequentialNode("root", task1, task2, task3)

	wf := &WorkflowDefinition{
		ID:       "save_load_resume_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "save_load_resume_wf", nil)
	if err == nil {
		t.Fatal("expected error")
	}

	data, err := engine.SaveState(state)
	if err != nil {
		t.Fatalf("save state failed: %v", err)
	}

	loadedState, err := engine.LoadState(data)
	if err != nil {
		t.Fatalf("load state failed: %v", err)
	}

	stepVal := loadedState.Context["step"]
	var stepInt int
	switch v := stepVal.(type) {
	case int:
		stepInt = v
	case float64:
		stepInt = int(v)
	}
	if stepInt != 1 {
		t.Errorf("expected step=1 in loaded state, got %v", stepVal)
	}

	task2Fixed := NewCallbackTaskNode("task2", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		execOrder = append(execOrder, "task2")
		mu.Unlock()
		execCtx.Set("step", 2)
		return 2, nil
	})
	task2Fixed.ID = root.Nodes[1].GetID()
	root.Nodes[1] = task2Fixed

	loadedState.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), loadedState)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed, got %v", resumedState.Status)
	}

	mu.Lock()
	task1Count := 0
	for _, name := range execOrder {
		if name == "task1" {
			task1Count++
		}
	}
	mu.Unlock()

	if task1Count != 1 {
		t.Errorf("task1 should execute once, got %d times: %v", task1Count, execOrder)
	}

	resumedStepVal := resumedState.Context["step"]
	switch v := resumedStepVal.(type) {
	case int:
		stepInt = v
	case float64:
		stepInt = int(v)
	}
	if stepInt != 3 {
		t.Errorf("expected step=3 after resume, got %v", resumedStepVal)
	}
}

func TestWorkflowResult_ContextNotMixedWithResults(t *testing.T) {
	engine := NewWorkflowEngine()

	task := NewCallbackTaskNode("testTask", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		execCtx.Set("user_id", 123)
		execCtx.Set("role", "admin")
		return "task_result", nil
	})

	wf := &WorkflowDefinition{
		ID:       "ctx_separate_wf",
		RootNode: task,
	}
	engine.RegisterWorkflow(wf)

	result, err := engine.ExecuteWorkflow(context.Background(), "ctx_separate_wf", nil)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	if result.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed, got %v", result.Status)
	}

	for key := range result.Results {
		if key == "user_id" || key == "role" {
			t.Errorf("context key '%s' should not be in Results, context should be separate", key)
		}
	}

	if result.Context == nil {
		t.Fatal("result.Context should not be nil")
	}

	userId, ok := result.Context["user_id"]
	if !ok {
		t.Error("user_id should be in result.Context")
	}
	if userId != 123 {
		t.Errorf("expected user_id=123, got %v", userId)
	}

	role, ok := result.Context["role"]
	if !ok {
		t.Error("role should be in result.Context")
	}
	if role != "admin" {
		t.Errorf("expected role=admin, got %v", role)
	}

	taskResult, ok := result.Results[task.GetID()]
	if !ok {
		t.Fatalf("task result should be in Results with key %s", task.GetID())
	}
	if taskResult.Output != "task_result" {
		t.Errorf("expected task output 'task_result', got %v", taskResult.Output)
	}
}

func TestNodeRetry_ZeroIntervalImmediateRetry(t *testing.T) {
	var attempts []time.Time
	mu := sync.Mutex{}

	task := NewCallbackTaskNode("test", func(ctx context.Context, execCtx *ExecutionContext) (interface{}, error) {
		mu.Lock()
		attempts = append(attempts, time.Now())
		mu.Unlock()
		return nil, errors.New("fail")
	})

	task.SetRetryConfig(RetryConfig{
		MaxRetries: 2,
		Interval:   0,
		Strategy:   RetryFixed,
	})

	start := time.Now()
	result, err := task.Execute(context.Background(), NewExecutionContext())
	total := time.Since(start)

	if err == nil {
		t.Error("expected error")
	}
	if result.Status != NodeStatusFailed {
		t.Errorf("expected failed status, got %v", result.Status)
	}

	mu.Lock()
	if len(attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", len(attempts))
	}
	mu.Unlock()

	if total > 50*time.Millisecond {
		t.Errorf("expected immediate retry (total time < 50ms), took %v", total)
	}
}

func TestAsInt_TypeCoverage(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
		ok       bool
	}{
		{"int", int(42), 42, true},
		{"int8", int8(42), 42, true},
		{"int16", int16(42), 42, true},
		{"int32", int32(42), 42, true},
		{"int64", int64(42), 42, true},
		{"uint", uint(42), 42, true},
		{"uint8", uint8(42), 42, true},
		{"uint16", uint16(42), 42, true},
		{"uint32", uint32(42), 42, true},
		{"uint64", uint64(42), 42, true},
		{"uint64_large", uint64(1000000000), 1000000000, true},
		{"uint64_overflow", uint64(1) << 63, 0, false},
		{"float32", float32(42.7), 42, true},
		{"float64", float64(42.9), 42, true},
		{"string_valid", "42", 42, true},
		{"string_negative", "-10", -10, true},
		{"string_invalid", "not_a_number", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
		{"struct", struct{}{}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := asInt(tt.input)
			if ok != tt.ok {
				t.Errorf("expected ok=%v, got ok=%v", tt.ok, ok)
			}
			if ok && result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFlakyTask_RestoreStateResetsFailures(t *testing.T) {
	task := NewFlakyTask("flaky", 2, "success")

	task.mu.Lock()
	task.failures = 5
	task.mu.Unlock()

	task.RestoreState(nil)

	task.mu.Lock()
	if task.failures != 0 {
		t.Errorf("expected failures=0 after RestoreState, got %d", task.failures)
	}
	task.mu.Unlock()
}

func TestFlakyTask_ResumeFromBreakpoint_RetryFromScratch(t *testing.T) {
	engine := NewWorkflowEngine()

	flaky := NewFlakyTask("flaky_task", 10, "success")
	flaky.SetRetryConfig(RetryConfig{
		MaxRetries: 2,
		Interval:   0,
		Strategy:   RetryFixed,
	})
	flakyID := flaky.GetID()

	root := NewSequentialNode("root", flaky)
	wf := &WorkflowDefinition{
		ID:       "flaky_resume_wf",
		RootNode: root,
	}
	engine.RegisterWorkflow(wf)

	state, err := engine.ExecuteWorkflowWithState(context.Background(), "flaky_resume_wf", nil)
	if err == nil {
		t.Fatal("expected error from failing flaky task")
	}

	flaky.mu.Lock()
	prevFailures := flaky.failures
	flaky.mu.Unlock()
	if prevFailures <= 0 {
		t.Fatalf("expected failures > 0 after failed execution, got %d", prevFailures)
	}

	savedData, err := engine.SaveState(state)
	if err != nil {
		t.Fatalf("SaveState failed: %v", err)
	}

	loadedState, err := engine.LoadState(savedData)
	if err != nil {
		t.Fatalf("LoadState failed: %v", err)
	}

	if loadedState.NodeStates == nil || len(loadedState.NodeStates) == 0 {
		t.Fatal("expected NodeStates to be non-empty after LoadState")
	}

	var flakyState *NodeExecutionState
	for k, v := range loadedState.NodeStates {
		if k == flakyID {
			flakyState = v
		}
	}

	if flakyState == nil {
		t.Fatalf("expected NodeStates for flaky_task (id=%s) after LoadState", flakyID)
	}
	if flakyState.InternalState == nil {
		t.Fatal("expected InternalState for flaky_task to be non-nil (GetState should return non-nil marker)")
	}

	flaky.FailCount = 1

	loadedState.Status = WorkflowStatusPaused

	resumedState, err := engine.ResumeWorkflow(context.Background(), loadedState)
	if err != nil {
		t.Fatalf("resume failed: %v", err)
	}
	if resumedState.Status != WorkflowStatusCompleted {
		t.Errorf("expected completed status, got %v (error: %v)", resumedState.Status, resumedState.Error)
	}

	flaky.mu.Lock()
	resumedFailures := flaky.failures
	flaky.mu.Unlock()

	if resumedFailures >= prevFailures {
		t.Errorf("expected failures to restart from 0 on resume, got %d (was %d before resume)", resumedFailures, prevFailures)
	}
	if resumedFailures > 2 {
		t.Errorf("expected failures <= 2 (FailCount=1 + MaxRetries=2), got %d", resumedFailures)
	}
}
