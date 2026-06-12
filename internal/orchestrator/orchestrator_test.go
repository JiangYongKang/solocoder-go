package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewOrchestrator(t *testing.T) {
	o := NewOrchestrator()
	if o == nil {
		t.Fatal("expected non-nil orchestrator")
	}
	if o.TaskCount() != 0 {
		t.Errorf("expected 0 tasks, got %d", o.TaskCount())
	}
}

func TestAddTask(t *testing.T) {
	o := NewOrchestrator()

	fn := func(ctx context.Context) (interface{}, error) {
		return "result", nil
	}

	err := o.AddTask("task1", "Task One", fn, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if o.TaskCount() != 1 {
		t.Errorf("expected 1 task, got %d", o.TaskCount())
	}

	task, err := o.GetTask("task1")
	if err != nil {
		t.Fatalf("unexpected error getting task: %v", err)
	}
	if task.ID != "task1" {
		t.Errorf("expected ID 'task1', got '%s'", task.ID)
	}
	if task.Name != "Task One" {
		t.Errorf("expected Name 'Task One', got '%s'", task.Name)
	}

	err = o.AddTask("task1", "Duplicate", fn, 0)
	if err != ErrTaskAlreadyExists {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}

	err = o.AddTask("task2", "Self Dep", fn, 0, "task2")
	if err != ErrInvalidDependency {
		t.Errorf("expected ErrInvalidDependency for self-dependency, got %v", err)
	}
}

func TestAddTaskWithDependencies(t *testing.T) {
	o := NewOrchestrator()

	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	err := o.AddTask("task1", "Task 1", fn, 0)
	if err != nil {
		t.Fatal(err)
	}

	err = o.AddTask("task2", "Task 2", fn, 0, "task1")
	if err != nil {
		t.Fatal(err)
	}

	task1, _ := o.GetTask("task1")
	if len(task1.Successors) != 1 || task1.Successors[0] != "task2" {
		t.Errorf("expected task1 to have successor 'task2', got %v", task1.Successors)
	}

	task2, _ := o.GetTask("task2")
	if len(task2.Dependencies) != 1 || task2.Dependencies[0] != "task1" {
		t.Errorf("expected task2 to have dependency 'task1', got %v", task2.Dependencies)
	}
}

func TestAddTaskWhileRunning(t *testing.T) {
	o := NewOrchestrator()
	fn := func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}
	o.AddTask("t1", "T1", fn, 0)

	var runErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, runErr = o.Run(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	err := o.AddTask("t2", "T2", fn, 0)
	if err != ErrOrchestratorRunning {
		t.Errorf("expected ErrOrchestratorRunning, got %v", err)
	}

	wg.Wait()
	if runErr != nil {
		t.Fatalf("unexpected Run error: %v", runErr)
	}
}

func TestValidateDAG_Success(t *testing.T) {
	o := NewOrchestrator()
	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	o.AddTask("a", "A", fn, 0)
	o.AddTask("b", "B", fn, 0, "a")
	o.AddTask("c", "C", fn, 0, "a")
	o.AddTask("d", "D", fn, 0, "b", "c")

	err := o.ValidateDAG()
	if err != nil {
		t.Errorf("expected valid DAG, got error: %v", err)
	}
}

func TestValidateDAG_Cycle(t *testing.T) {
	o := NewOrchestrator()
	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	o.AddTask("a", "A", fn, 0)
	o.AddTask("b", "B", fn, 0, "a")

	o.mu.Lock()
	o.tasks["a"].Dependencies = []string{"b"}
	o.tasks["b"].Successors = append(o.tasks["b"].Successors, "a")
	o.mu.Unlock()

	err := o.ValidateDAG()
	if err != ErrCycleDetected {
		t.Errorf("expected ErrCycleDetected, got %v", err)
	}
}

func TestValidateDAG_InvalidDependency(t *testing.T) {
	o := NewOrchestrator()
	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	o.AddTask("a", "A", fn, 0)
	o.AddTask("b", "B", fn, 0, "nonexistent")

	err := o.ValidateDAG()
	if err == nil {
		t.Error("expected error for invalid dependency")
	}
}

func TestRun_EmptyOrchestrator(t *testing.T) {
	o := NewOrchestrator()

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Success {
		t.Error("expected success for empty orchestrator")
	}
	if len(report.TaskResults) != 0 {
		t.Errorf("expected 0 task results, got %d", len(report.TaskResults))
	}
}

func TestRun_SingleTask(t *testing.T) {
	o := NewOrchestrator()

	executed := false
	fn := func(ctx context.Context) (interface{}, error) {
		executed = true
		return "hello", nil
	}

	o.AddTask("task1", "Task 1", fn, 0)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected report.Success to be true")
	}

	if !executed {
		t.Error("expected task to be executed")
	}

	result := report.TaskResults["task1"]
	if result == nil {
		t.Fatal("expected task1 result")
	}
	if result.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %s", result.Status)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got %v", result.Output)
	}
	if result.Duration < 0 {
		t.Error("expected non-negative duration")
	}
	if result.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", result.Attempts)
	}
}

func TestRun_LinearDAG(t *testing.T) {
	o := NewOrchestrator()

	var order []string
	var mu sync.Mutex

	makeTask := func(id string, delay time.Duration) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			time.Sleep(delay)
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			return id, nil
		}
	}

	o.AddTask("a", "A", makeTask("a", 10*time.Millisecond), 0)
	o.AddTask("b", "B", makeTask("b", 10*time.Millisecond), 0, "a")
	o.AddTask("c", "C", makeTask("c", 10*time.Millisecond), 0, "b")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	expectedOrder := []string{"a", "b", "c"}
	for i, id := range expectedOrder {
		if order[i] != id {
			t.Errorf("expected order %v, got %v", expectedOrder, order)
			break
		}
	}

	for _, id := range expectedOrder {
		result := report.TaskResults[id]
		if result.Status != StatusSuccess {
			t.Errorf("task %s: expected Success, got %s", id, result.Status)
		}
		if result.Output != id {
			t.Errorf("task %s: expected output %s, got %v", id, id, result.Output)
		}
	}
}

func TestRun_DiamondDAG(t *testing.T) {
	o := NewOrchestrator()

	var order []string
	var mu sync.Mutex

	makeTask := func(id string) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			return id, nil
		}
	}

	o.AddTask("a", "A", makeTask("a"), 0)
	o.AddTask("b", "B", makeTask("b"), 0, "a")
	o.AddTask("c", "C", makeTask("c"), 0, "a")
	o.AddTask("d", "D", makeTask("d"), 0, "b", "c")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	mu.Lock()
	if order[0] != "a" {
		t.Errorf("expected 'a' first, got %v", order)
	}
	lastIdx := len(order) - 1
	if order[lastIdx] != "d" {
		t.Errorf("expected 'd' last, got %v", order)
	}
	mu.Unlock()

	for _, id := range []string{"a", "b", "c", "d"} {
		result := report.TaskResults[id]
		if result.Status != StatusSuccess {
			t.Errorf("task %s: expected Success, got %s", id, result.Status)
		}
	}
}

func TestRun_ParallelDAG(t *testing.T) {
	o := NewOrchestrator()

	var started int32
	var completed int32

	makeTask := func(id string) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			atomic.AddInt32(&started, 1)
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&completed, 1)
			return id, nil
		}
	}

	o.AddTask("root", "Root", makeTask("root"), 0)
	o.AddTask("a", "A", makeTask("a"), 0, "root")
	o.AddTask("b", "B", makeTask("b"), 0, "root")
	o.AddTask("c", "C", makeTask("c"), 0, "root")

	start := time.Now()
	report, err := o.Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("expected parallel execution to take ~100ms, took %v", elapsed)
	}

	for _, id := range []string{"root", "a", "b", "c"} {
		result := report.TaskResults[id]
		if result.Status != StatusSuccess {
			t.Errorf("task %s: expected Success, got %s", id, result.Status)
		}
	}
}

func TestRun_TaskTimeout(t *testing.T) {
	o := NewOrchestrator()

	fn := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		return "done", nil
	}

	o.AddTask("timeout", "Timeout Task", fn, 20*time.Millisecond)
	o.AddTask("dependent", "Dependent", func(ctx context.Context) (interface{}, error) {
		return "should not run", nil
	}, 0, "timeout")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure due to timeout")
	}

	timeoutResult := report.TaskResults["timeout"]
	if timeoutResult.Status != StatusTimeout {
		t.Errorf("expected StatusTimeout, got %s", timeoutResult.Status)
	}
	if timeoutResult.Error != ErrTimeout {
		t.Errorf("expected ErrTimeout, got %v", timeoutResult.Error)
	}

	depResult := report.TaskResults["dependent"]
	if depResult.Status != StatusSkipped {
		t.Errorf("expected dependent to be Skipped, got %s", depResult.Status)
	}
}

func TestRun_ErrorPropagation(t *testing.T) {
	o := NewOrchestrator()

	fnSuccess := func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}

	fnFail := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("task failed")
	}

	o.AddTask("a", "A", fnSuccess, 0)
	o.AddTask("b", "B", fnFail, 0, "a")
	o.AddTask("c", "C", fnSuccess, 0, "a")
	o.AddTask("d", "D", fnSuccess, 0, "b", "c")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure")
	}

	resultA := report.TaskResults["a"]
	if resultA.Status != StatusSuccess {
		t.Errorf("task a: expected Success, got %s", resultA.Status)
	}

	resultB := report.TaskResults["b"]
	if resultB.Status != StatusFailed {
		t.Errorf("task b: expected Failed, got %s", resultB.Status)
	}

	resultC := report.TaskResults["c"]
	if resultC.Status != StatusSuccess {
		t.Errorf("task c: expected Success, got %s", resultC.Status)
	}

	resultD := report.TaskResults["d"]
	if resultD.Status != StatusSkipped {
		t.Errorf("task d: expected Skipped (depends on failed b), got %s", resultD.Status)
	}
}

func TestRun_IndependentBranches(t *testing.T) {
	o := NewOrchestrator()

	fnFail := func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("failed")
	}

	fnSuccess := func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}

	o.AddTask("a1", "A1", fnFail, 0)
	o.AddTask("a2", "A2", fnSuccess, 0, "a1")

	o.AddTask("b1", "B1", fnSuccess, 0)
	o.AddTask("b2", "B2", fnSuccess, 0, "b1")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected overall failure due to branch A")
	}

	if report.TaskResults["a1"].Status != StatusFailed {
		t.Errorf("a1 should be Failed, got %s", report.TaskResults["a1"].Status)
	}
	if report.TaskResults["a2"].Status != StatusSkipped {
		t.Errorf("a2 should be Skipped, got %s", report.TaskResults["a2"].Status)
	}

	if report.TaskResults["b1"].Status != StatusSuccess {
		t.Errorf("b1 should be Success, got %s", report.TaskResults["b1"].Status)
	}
	if report.TaskResults["b2"].Status != StatusSuccess {
		t.Errorf("b2 should be Success, got %s", report.TaskResults["b2"].Status)
	}
}

func TestRun_CascadingFailure(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("root failure")
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c", nil
	}, 0, "b")
	o.AddTask("d", "D", func(ctx context.Context) (interface{}, error) {
		return "d", nil
	}, 0, "c")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TaskResults["a"].Status != StatusFailed {
		t.Errorf("a should be Failed, got %s", report.TaskResults["a"].Status)
	}
	for _, id := range []string{"b", "c", "d"} {
		if report.TaskResults[id].Status != StatusSkipped {
			t.Errorf("%s should be Skipped, got %s", id, report.TaskResults[id].Status)
		}
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("slow", "Slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(5 * time.Second)
		return "done", nil
	}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	report, err := o.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure due to context cancellation")
	}
}

func TestRun_TaskPanic(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("panic", "Panic", func(ctx context.Context) (interface{}, error) {
		panic("something went wrong")
	}, 0)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure due to panic")
	}

	result := report.TaskResults["panic"]
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", result.Status)
	}
	if result.Error == nil {
		t.Error("expected error message for panic")
	}
}

func TestRun_TaskWithRetry(t *testing.T) {
	o := NewOrchestrator()

	var attempts int32

	o.AddTask("retry", "Retry", func(ctx context.Context) (interface{}, error) {
		a := atomic.AddInt32(&attempts, 1)
		if a < 3 {
			return nil, errors.New("not yet")
		}
		return "finally", nil
	}, 0)
	o.SetTaskRetry("retry", 3)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success after retries")
	}

	result := report.TaskResults["retry"]
	if result.Status != StatusSuccess {
		t.Errorf("expected StatusSuccess, got %s", result.Status)
	}
	if result.Output != "finally" {
		t.Errorf("expected output 'finally', got %v", result.Output)
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", result.Attempts)
	}
}

func TestRun_TaskRetryExhausted(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("fail", "Fail", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("always fails")
	}, 0)
	o.SetTaskRetry("fail", 2)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure")
	}

	result := report.TaskResults["fail"]
	if result.Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", result.Status)
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts (1 + 2 retries), got %d", result.Attempts)
	}
}

func TestRun_TimeoutWithRetry(t *testing.T) {
	o := NewOrchestrator()

	var attempts int32

	o.AddTask("timeout_retry", "Timeout Retry", func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&attempts, 1)
		time.Sleep(200 * time.Millisecond)
		return nil, nil
	}, 30*time.Millisecond)
	o.SetTaskRetry("timeout_retry", 1)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := report.TaskResults["timeout_retry"]
	if result.Status != StatusTimeout {
		t.Errorf("expected StatusTimeout, got %s", result.Status)
	}
}

func TestRun_ExecutionReportTiming(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("t1", "T1", func(ctx context.Context) (interface{}, error) {
		time.Sleep(20 * time.Millisecond)
		return "ok", nil
	}, 0)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Duration <= 0 {
		t.Error("expected positive total duration")
	}
	if report.StartTime.IsZero() {
		t.Error("expected non-zero start time")
	}
	if report.EndTime.IsZero() {
		t.Error("expected non-zero end time")
	}
	if report.EndTime.Before(report.StartTime) {
		t.Error("expected end time after start time")
	}
}

func TestRun_AllTasksHaveResults(t *testing.T) {
	o := NewOrchestrator()

	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	o.AddTask("a", "A", fn, 0)
	o.AddTask("b", "B", fn, 0, "a")
	o.AddTask("c", "C", fn, 0, "a")
	o.AddTask("d", "D", fn, 0, "b", "c")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.TaskResults) != 4 {
		t.Errorf("expected 4 task results, got %d", len(report.TaskResults))
	}

	for _, id := range []string{"a", "b", "c", "d"} {
		result, ok := report.TaskResults[id]
		if !ok {
			t.Errorf("missing result for task %s", id)
			continue
		}
		if result.TaskID != id {
			t.Errorf("expected task ID %s, got %s", id, result.TaskID)
		}
	}
}

func TestRetryTask_Success(t *testing.T) {
	o := NewOrchestrator()

	var aAttempts int32

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		if atomic.AddInt32(&aAttempts, 1) == 1 {
			return nil, errors.New("first attempt fails")
		}
		return "a-ok", nil
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c-ok", nil
	}, 0, "b")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TaskResults["a"].Status != StatusFailed {
		t.Fatalf("expected a to be Failed after initial run, got %s", report.TaskResults["a"].Status)
	}
	if report.TaskResults["b"].Status != StatusSkipped {
		t.Errorf("expected b to be Skipped, got %s", report.TaskResults["b"].Status)
	}
	if report.TaskResults["c"].Status != StatusSkipped {
		t.Errorf("expected c to be Skipped, got %s", report.TaskResults["c"].Status)
	}

	retryReport, err := o.RetryTask(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}

	if !retryReport.Success {
		t.Error("expected retry to succeed")
	}

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("expected a to be Success after retry, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["a"].Output != "a-ok" {
		t.Errorf("expected a output 'a-ok', got %v", retryReport.TaskResults["a"].Output)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("expected b to be Success after retry, got %s", retryReport.TaskResults["b"].Status)
	}
	if retryReport.TaskResults["c"].Status != StatusSuccess {
		t.Errorf("expected c to be Success after retry, got %s", retryReport.TaskResults["c"].Status)
	}
}

func TestRetryTask_PreservesSuccessfulUpstream(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("root", "Root", func(ctx context.Context) (interface{}, error) {
		return "root-ok", nil
	}, 0)
	o.AddTask("fail", "Fail", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fail")
	}, 0, "root")
	o.AddTask("downstream", "Downstream", func(ctx context.Context) (interface{}, error) {
		return "ds-ok", nil
	}, 0, "fail")

	o.Run(context.Background())

	var rootExecCount int32
	originalRoot := o.tasks["root"].Func
	o.tasks["root"].Func = func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&rootExecCount, 1)
		return originalRoot(ctx)
	}

	retryReport, err := o.RetryTask(context.Background(), "fail")
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}

	if retryReport.TaskResults["root"].Status != StatusSuccess {
		t.Errorf("root should still be Success, got %s", retryReport.TaskResults["root"].Status)
	}

	if atomic.LoadInt32(&rootExecCount) != 0 {
		t.Error("root task should not be re-executed during retry")
	}

	if retryReport.TaskResults["fail"].Status != StatusFailed {
		t.Errorf("fail should be Failed on retry (same func), got %s", retryReport.TaskResults["fail"].Status)
	}
}

func TestRetryTask_MidDAGRetry(t *testing.T) {
	o := NewOrchestrator()

	var bAttempts int32

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return "a-ok", nil
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		if atomic.AddInt32(&bAttempts, 1) == 1 {
			return nil, errors.New("b fails first time")
		}
		return "b-ok", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c-ok", nil
	}, 0, "b")

	report, _ := o.Run(context.Background())

	if report.TaskResults["a"].Status != StatusSuccess {
		t.Fatalf("a should be Success, got %s", report.TaskResults["a"].Status)
	}
	if report.TaskResults["b"].Status != StatusFailed {
		t.Fatalf("b should be Failed, got %s", report.TaskResults["b"].Status)
	}
	if report.TaskResults["c"].Status != StatusSkipped {
		t.Errorf("c should be Skipped, got %s", report.TaskResults["c"].Status)
	}

	retryReport, err := o.RetryTask(context.Background(), "b")
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("a should still be Success after retry, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("b should be Success after retry, got %s", retryReport.TaskResults["b"].Status)
	}
	if retryReport.TaskResults["c"].Status != StatusSuccess {
		t.Errorf("c should be Success after retry, got %s", retryReport.TaskResults["c"].Status)
	}
}

func TestRetryTask_NotFound(t *testing.T) {
	o := NewOrchestrator()

	_, err := o.RetryTask(context.Background(), "nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestRetryTask_NotRetryable(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, 0)

	o.Run(context.Background())

	_, err := o.RetryTask(context.Background(), "a")
	if err != ErrCannotRetry {
		t.Errorf("expected ErrCannotRetry for successful task, got %v", err)
	}
}

func TestRetryTask_WhileRunning(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("slow", "Slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return nil, nil
	}, 0)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.Run(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	_, err := o.RetryTask(context.Background(), "slow")
	if err != ErrOrchestratorRunning {
		t.Errorf("expected ErrOrchestratorRunning, got %v", err)
	}

	wg.Wait()
}

func TestGetTask_NotFound(t *testing.T) {
	o := NewOrchestrator()

	_, err := o.GetTask("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestGetTaskResult_NotFound(t *testing.T) {
	o := NewOrchestrator()

	_, err := o.GetTaskResult("nonexistent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestSetTaskRetry_NotFound(t *testing.T) {
	o := NewOrchestrator()

	err := o.SetTaskRetry("nonexistent", 3)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestStop(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("slow", "Slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(5 * time.Second)
		return "done", nil
	}, 0)

	go func() {
		time.Sleep(50 * time.Millisecond)
		o.Stop()
	}()

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure after Stop")
	}
}

func TestStop_WhenNotRunning(t *testing.T) {
	o := NewOrchestrator()
	o.Stop()
}

func TestRun_DoubleRun(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("t1", "T1", func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return nil, nil
	}, 0)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		o.Run(context.Background())
	}()

	time.Sleep(20 * time.Millisecond)
	_, err := o.Run(context.Background())
	if err != ErrOrchestratorRunning {
		t.Errorf("expected ErrOrchestratorRunning, got %v", err)
	}

	wg.Wait()
}

func TestRun_MultipleRoots(t *testing.T) {
	o := NewOrchestrator()

	var order []string
	var mu sync.Mutex

	makeTask := func(id string) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			mu.Lock()
			order = append(order, id)
			mu.Unlock()
			return id, nil
		}
	}

	o.AddTask("r1", "R1", makeTask("r1"), 0)
	o.AddTask("r2", "R2", makeTask("r2"), 0)
	o.AddTask("r3", "R3", makeTask("r3"), 0)
	o.AddTask("final", "Final", makeTask("final"), 0, "r1", "r2", "r3")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	mu.Lock()
	if len(order) != 4 {
		t.Errorf("expected 4 tasks executed, got %d", len(order))
	}
	if order[len(order)-1] != "final" {
		t.Errorf("expected 'final' to be last, got order %v", order)
	}
	mu.Unlock()
}

func TestRun_MultipleFailures(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("fail1", "Fail1", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fail1")
	}, 0)
	o.AddTask("fail2", "Fail2", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("fail2")
	}, 0)
	o.AddTask("down", "Down", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, 0, "fail1", "fail2")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Success {
		t.Error("expected failure")
	}
	if report.TaskResults["fail1"].Status != StatusFailed {
		t.Errorf("fail1 should be Failed, got %s", report.TaskResults["fail1"].Status)
	}
	if report.TaskResults["fail2"].Status != StatusFailed {
		t.Errorf("fail2 should be Failed, got %s", report.TaskResults["fail2"].Status)
	}
	if report.TaskResults["down"].Status != StatusSkipped {
		t.Errorf("down should be Skipped, got %s", report.TaskResults["down"].Status)
	}
}

func TestRun_TimeoutBranchAndSuccessBranch(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("root", "Root", func(ctx context.Context) (interface{}, error) {
		return "root-ok", nil
	}, 0)

	o.AddTask("slow", "Slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(5 * time.Second)
		return "slow", nil
	}, 30*time.Millisecond, "root")

	o.AddTask("fast", "Fast", func(ctx context.Context) (interface{}, error) {
		return "fast-ok", nil
	}, 0, "root")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TaskResults["root"].Status != StatusSuccess {
		t.Errorf("root should be Success, got %s", report.TaskResults["root"].Status)
	}
	if report.TaskResults["slow"].Status != StatusTimeout {
		t.Errorf("slow should be Timeout, got %s", report.TaskResults["slow"].Status)
	}
	if report.TaskResults["fast"].Status != StatusSuccess {
		t.Errorf("fast should be Success, got %s", report.TaskResults["fast"].Status)
	}
}

func TestRun_LargeParallelGraph(t *testing.T) {
	o := NewOrchestrator()

	var executed int32
	fn := func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&executed, 1)
		return nil, nil
	}

	o.AddTask("root", "Root", fn, 0)

	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("child_%d", i)
		o.AddTask(id, id, fn, 0, "root")
	}

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	if atomic.LoadInt32(&executed) != 21 {
		t.Errorf("expected 21 executions, got %d", atomic.LoadInt32(&executed))
	}
}

func TestRun_ConcurrentTaskExecution(t *testing.T) {
	o := NewOrchestrator()

	var concurrentCount int32
	var maxConcurrent int32

	makeTask := func(id string) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			c := atomic.AddInt32(&concurrentCount, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if c <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, c) {
					break
				}
			}
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&concurrentCount, -1)
			return id, nil
		}
	}

	o.AddTask("a", "A", makeTask("a"), 0)
	o.AddTask("b", "B", makeTask("b"), 0)
	o.AddTask("c", "C", makeTask("c"), 0)
	o.AddTask("d", "D", makeTask("d"), 0)
	o.AddTask("e", "E", makeTask("e"), 0, "a", "b", "c", "d")

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	mc := atomic.LoadInt32(&maxConcurrent)
	if mc < 2 {
		t.Errorf("expected at least 2 concurrent tasks, got max %d", mc)
	}
}

func TestRetryTask_WithTimeout(t *testing.T) {
	o := NewOrchestrator()

	var attempts int32

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		a := atomic.AddInt32(&attempts, 1)
		if a == 1 {
			time.Sleep(200 * time.Millisecond)
			return nil, nil
		}
		return "a-ok", nil
	}, 30*time.Millisecond)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")

	report, _ := o.Run(context.Background())

	if report.TaskResults["a"].Status != StatusTimeout {
		t.Fatalf("expected a to be Timeout, got %s", report.TaskResults["a"].Status)
	}

	retryReport, err := o.RetryTask(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("expected a to be Success after retry, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("expected b to be Success after retry, got %s", retryReport.TaskResults["b"].Status)
	}
}

func TestTaskStatus_String(t *testing.T) {
	statuses := map[TaskStatus]string{
		StatusPending: "Pending",
		StatusRunning: "Running",
		StatusSuccess: "Success",
		StatusFailed:  "Failed",
		StatusSkipped: "Skipped",
		StatusTimeout: "Timeout",
	}

	for status, expected := range statuses {
		if status.String() != expected {
			t.Errorf("expected %s, got %s", expected, status.String())
		}
	}

	unknown := TaskStatus(99)
	if unknown.String() != "Unknown" {
		t.Errorf("expected Unknown, got %s", unknown.String())
	}
}

func TestTaskStatus_IsTerminal(t *testing.T) {
	terminals := []TaskStatus{StatusSuccess, StatusFailed, StatusSkipped, StatusTimeout}
	nonTerminals := []TaskStatus{StatusPending, StatusRunning}

	for _, s := range terminals {
		if !s.IsTerminal() {
			t.Errorf("expected %s to be terminal", s)
		}
	}

	for _, s := range nonTerminals {
		if s.IsTerminal() {
			t.Errorf("expected %s to be non-terminal", s)
		}
	}
}

func TestGetTask_ReturnsCopy(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) { return nil, nil }, 0, "b")
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) { return nil, nil }, 0)

	task, _ := o.GetTask("a")
	task.Name = "Modified"

	original, _ := o.GetTask("a")
	if original.Name != "A" {
		t.Error("GetTask should return a copy, not a reference")
	}
}

func TestGetTaskResult_ReturnsCopy(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) { return "result", nil }, 0)
	o.Run(context.Background())

	result, _ := o.GetTaskResult("a")
	result.Output = "modified"

	original, _ := o.GetTaskResult("a")
	if original.Output != "result" {
		t.Error("GetTaskResult should return a copy, not a reference")
	}
}

func TestRun_ZeroTimeout(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	}, 0)

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success with zero timeout")
	}
}

func TestRetryTask_SkippedTask(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("a fails")
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")

	o.Run(context.Background())

	if report := o.results["b"]; report.Status != StatusSkipped {
		t.Fatalf("expected b to be Skipped, got %s", report.Status)
	}

	o.tasks["a"].Func = func(ctx context.Context) (interface{}, error) {
		return "a-ok", nil
	}

	retryReport, err := o.RetryTask(context.Background(), "a")
	if err != nil {
		t.Fatalf("unexpected retry error: %v", err)
	}

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("expected a to be Success after retry, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("expected b to be Success after retry, got %s", retryReport.TaskResults["b"].Status)
	}
}

func TestRun_DeepDAG(t *testing.T) {
	o := NewOrchestrator()

	depth := 50
	fn := func(ctx context.Context) (interface{}, error) { return nil, nil }

	o.AddTask("t0", "T0", fn, 0)
	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		dep := fmt.Sprintf("t%d", i-1)
		o.AddTask(id, id, fn, 0, dep)
	}

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.Success {
		t.Error("expected success")
	}

	for i := 0; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		if report.TaskResults[id].Status != StatusSuccess {
			t.Errorf("task %s: expected Success, got %s", id, report.TaskResults[id].Status)
		}
	}
}
