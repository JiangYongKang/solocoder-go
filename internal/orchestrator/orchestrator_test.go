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

func TestUpdateTaskFunc_Basic(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return "old", nil
	}, 0)

	err := o.UpdateTaskFunc("a", func(ctx context.Context) (interface{}, error) {
		return "new", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected Run error: %v", err)
	}

	result := report.TaskResults["a"]
	if result.Output != "new" {
		t.Errorf("expected updated output 'new', got %v", result.Output)
	}
}

func TestUpdateTaskFunc_NotFound(t *testing.T) {
	o := NewOrchestrator()

	err := o.UpdateTaskFunc("nope", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	})
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestUpdateTaskFunc_WhileRunning(t *testing.T) {
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

	err := o.UpdateTaskFunc("slow", func(ctx context.Context) (interface{}, error) {
		return nil, nil
	})
	if err != ErrOrchestratorRunning {
		t.Errorf("expected ErrOrchestratorRunning, got %v", err)
	}

	wg.Wait()
}

func TestUpdateTaskTimeout_Basic(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "ok", nil
	}, 1*time.Second)

	o.UpdateTaskTimeout("a", 20*time.Millisecond)

	report, _ := o.Run(context.Background())

	result := report.TaskResults["a"]
	if result.Status != StatusTimeout {
		t.Errorf("expected StatusTimeout after timeout update, got %s", result.Status)
	}
}

func TestUpdateTaskTimeout_NotFound(t *testing.T) {
	o := NewOrchestrator()

	err := o.UpdateTaskTimeout("nope", 5*time.Second)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestUpdateTaskFunc_ThenRetry(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return "a-ok", nil
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return nil, errors.New("b has a bug")
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c-ok", nil
	}, 0, "b")

	report1, _ := o.Run(context.Background())

	if report1.TaskResults["a"].Status != StatusSuccess {
		t.Fatalf("a should be Success, got %s", report1.TaskResults["a"].Status)
	}
	if report1.TaskResults["b"].Status != StatusFailed {
		t.Fatalf("b should be Failed, got %s", report1.TaskResults["b"].Status)
	}
	if report1.TaskResults["c"].Status != StatusSkipped {
		t.Fatalf("c should be Skipped, got %s", report1.TaskResults["c"].Status)
	}

	var aExecCount int32
	originalAFunc := o.tasks["a"].Func
	o.tasks["a"].Func = func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&aExecCount, 1)
		return originalAFunc(ctx)
	}

	err := o.UpdateTaskFunc("b", func(ctx context.Context) (interface{}, error) {
		return "b-fixed", nil
	})
	if err != nil {
		t.Fatalf("unexpected UpdateTaskFunc error: %v", err)
	}

	retryReport, err := o.RetryTask(context.Background(), "b")
	if err != nil {
		t.Fatalf("unexpected RetryTask error: %v", err)
	}

	if atomic.LoadInt32(&aExecCount) != 0 {
		t.Errorf("task a should not be re-executed during retry, got %d executions", aExecCount)
	}

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("a should remain Success after retry, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("b should be Success after fix, got %s", retryReport.TaskResults["b"].Status)
	}
	if retryReport.TaskResults["b"].Output != "b-fixed" {
		t.Errorf("b output should be 'b-fixed', got %v", retryReport.TaskResults["b"].Output)
	}
	if retryReport.TaskResults["c"].Status != StatusSuccess {
		t.Errorf("c should be Success after fix, got %s", retryReport.TaskResults["c"].Status)
	}
}

func TestUpdateTaskFunc_RetryRootCause(t *testing.T) {
	o := NewOrchestrator()

	rootCause := errors.New("database connection refused")

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return nil, rootCause
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c-ok", nil
	}, 0, "b")

	o.Run(context.Background())

	o.UpdateTaskFunc("a", func(ctx context.Context) (interface{}, error) {
		return "a-recovered", nil
	})

	retryReport, _ := o.RetryTask(context.Background(), "a")

	if !retryReport.Success {
		t.Error("expected retry to succeed")
	}
	for _, id := range []string{"a", "b", "c"} {
		if retryReport.TaskResults[id].Status != StatusSuccess {
			t.Errorf("task %s should be Success, got %s", id, retryReport.TaskResults[id].Status)
		}
	}
}

func TestErrorPropagation_DeepChain(t *testing.T) {
	o := NewOrchestrator()

	rootErr := errors.New("root cause: invalid API key")

	depth := 5
	fn := func(id string, errToReturn error) TaskFunc {
		return func(ctx context.Context) (interface{}, error) {
			if errToReturn != nil {
				return nil, errToReturn
			}
			return id + "-ok", nil
		}
	}

	o.AddTask("t0", "T0", fn("t0", rootErr), 0)
	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		dep := fmt.Sprintf("t%d", i-1)
		o.AddTask(id, id, fn(id, nil), 0, dep)
	}

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t0Result := report.TaskResults["t0"]
	if t0Result.Status != StatusFailed {
		t.Fatalf("t0 should be Failed, got %s", t0Result.Status)
	}
	if t0Result.Error != rootErr {
		t.Errorf("t0 Error should be root cause, got %v", t0Result.Error)
	}

	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		prevID := fmt.Sprintf("t%d", i-1)
		result := report.TaskResults[id]
		if result.Status != StatusSkipped {
			t.Errorf("task %s should be Skipped, got %s", id, result.Status)
			continue
		}
		if result.Error == nil {
			t.Errorf("task %s: expected non-nil error", id)
			continue
		}
		errMsg := result.Error.Error()
		if errMsg == "" {
			t.Errorf("task %s: expected non-empty error message", id)
		}
		if !containsString(errMsg, prevID) {
			t.Errorf("task %s: error should mention direct dependency '%s', got '%s'", id, prevID, errMsg)
		}
		if i == 1 {
			if !errors.Is(result.Error, rootErr) {
				t.Errorf("task %s: expected error chain to wrap root cause via errors.Is, got %v", id, result.Error)
			}
		}
	}

	if depth >= 2 {
		lastID := fmt.Sprintf("t%d", depth-1)
		lastErr := report.TaskResults[lastID].Error
		if !errors.Is(lastErr, rootErr) {
			t.Errorf("deepest task %s: error chain should ultimately contain root cause via errors.Is, got %v", lastID, lastErr)
		}
	}
}

func TestErrorPropagation_TimeoutCause(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("slow", "Slow", func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return nil, nil
	}, 20*time.Millisecond)
	o.AddTask("mid", "Mid", func(ctx context.Context) (interface{}, error) {
		return "mid", nil
	}, 0, "slow")
	o.AddTask("end", "End", func(ctx context.Context) (interface{}, error) {
		return "end", nil
	}, 0, "mid")

	report, _ := o.Run(context.Background())

	slowResult := report.TaskResults["slow"]
	if slowResult.Status != StatusTimeout {
		t.Fatalf("slow should be Timeout, got %s", slowResult.Status)
	}

	for _, id := range []string{"mid", "end"} {
		result := report.TaskResults[id]
		if result.Status != StatusSkipped {
			t.Errorf("task %s should be Skipped, got %s", id, result.Status)
			continue
		}
		if result.Error == nil {
			t.Errorf("task %s should have error", id)
			continue
		}
		if !errors.Is(result.Error, ErrTimeout) {
			t.Errorf("task %s: expected error chain to contain ErrTimeout, got %v", id, result.Error)
		}
	}
}

func TestErrorPropagation_MultiDep(t *testing.T) {
	o := NewOrchestrator()

	errA := errors.New("a failed badly")

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) { return nil, errA }, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) { return "b-ok", nil }, 0)
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) { return "c-ok", nil }, 0, "a", "b")

	report, _ := o.Run(context.Background())

	cResult := report.TaskResults["c"]
	if cResult.Status != StatusSkipped {
		t.Fatalf("c should be Skipped, got %s", cResult.Status)
	}
	if !errors.Is(cResult.Error, errA) {
		t.Errorf("c's error should wrap errA via errors.Is, got %v", cResult.Error)
	}
}

func TestUpdateTaskFunc_UpdateTimeoutThenRetry(t *testing.T) {
	o := NewOrchestrator()

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "a-ok", nil
	}, 20*time.Millisecond)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")

	report1, _ := o.Run(context.Background())

	if report1.TaskResults["a"].Status != StatusTimeout {
		t.Fatalf("a should be Timeout, got %s", report1.TaskResults["a"].Status)
	}

	o.UpdateTaskTimeout("a", 200*time.Millisecond)

	retryReport, _ := o.RetryTask(context.Background(), "a")

	if retryReport.TaskResults["a"].Status != StatusSuccess {
		t.Errorf("a should be Success after timeout increase, got %s", retryReport.TaskResults["a"].Status)
	}
	if retryReport.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("b should be Success after timeout increase, got %s", retryReport.TaskResults["b"].Status)
	}
}

type customDBError struct {
	Code    int
	Message string
}

func (e *customDBError) Error() string {
	return fmt.Sprintf("database error code=%d: %s", e.Code, e.Message)
}

func TestErrorPropagation_CustomErrorsWithErrorsAs(t *testing.T) {
	o := NewOrchestrator()

	rootErr := &customDBError{Code: 1045, Message: "Access denied for user"}

	depth := 4
	o.AddTask("t0", "T0", func(ctx context.Context) (interface{}, error) {
		return nil, rootErr
	}, 0)
	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		dep := fmt.Sprintf("t%d", i-1)
		o.AddTask(id, id, func(ctx context.Context) (interface{}, error) {
			return "ok", nil
		}, 0, dep)
	}

	report, _ := o.Run(context.Background())

	lastID := fmt.Sprintf("t%d", depth-1)
	lastErr := report.TaskResults[lastID].Error

	var dbErr *customDBError
	if !errors.As(lastErr, &dbErr) {
		t.Errorf("expected errors.As to find customDBError in chain, got %v", lastErr)
	}
	if dbErr != nil {
		if dbErr.Code != 1045 {
			t.Errorf("expected code 1045, got %d", dbErr.Code)
		}
		if dbErr.Message != "Access denied for user" {
			t.Errorf("unexpected message: %s", dbErr.Message)
		}
	}
}

func TestErrorPropagation_VeryDeepChain(t *testing.T) {
	o := NewOrchestrator()

	rootErr := errors.New("very deep root cause")
	depth := 20

	o.AddTask("t0", "T0", func(ctx context.Context) (interface{}, error) {
		return nil, rootErr
	}, 0)
	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		dep := fmt.Sprintf("t%d", i-1)
		o.AddTask(id, id, func(ctx context.Context) (interface{}, error) {
			return nil, nil
		}, 0, dep)
	}

	report, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	firstResult := report.TaskResults["t0"]
	if firstResult.Status != StatusFailed {
		t.Fatalf("t0 should be Failed, got %s", firstResult.Status)
	}

	for i := 1; i < depth; i++ {
		id := fmt.Sprintf("t%d", i)
		result := report.TaskResults[id]
		if result.Status != StatusSkipped {
			t.Errorf("task %s should be Skipped, got %s", id, result.Status)
			continue
		}
		if result.Error == nil {
			t.Errorf("task %s should have non-nil error", id)
			continue
		}
		if !errors.Is(result.Error, rootErr) {
			t.Errorf("task %s: errors.Is should find root cause in chain, got %v", id, result.Error)
		}
	}

	lastID := fmt.Sprintf("t%d", depth-1)
	lastResult := report.TaskResults[lastID]
	errStr := lastResult.Error.Error()
	if len(errStr) < len("very deep root cause") {
		t.Errorf("expected error string to be substantial, got length %d: %s", len(errStr), errStr)
	}
}

func TestUpdateTaskFunc_CombinedRetryScenario(t *testing.T) {
	o := NewOrchestrator()

	dbErr := &customDBError{Code: 2003, Message: "Can't connect to MySQL server"}

	o.AddTask("validate", "Validate Input", func(ctx context.Context) (interface{}, error) {
		return "validated", nil
	}, 0)

	o.AddTask("fetch_db", "Fetch from DB", func(ctx context.Context) (interface{}, error) {
		return nil, dbErr
	}, 100*time.Millisecond, "validate")

	o.AddTask("process", "Process Data", func(ctx context.Context) (interface{}, error) {
		return "processed", nil
	}, 0, "fetch_db")

	o.AddTask("notify", "Notify Users", func(ctx context.Context) (interface{}, error) {
		return "notified", nil
	}, 0, "process")

	report1, _ := o.Run(context.Background())

	if report1.TaskResults["validate"].Status != StatusSuccess {
		t.Fatalf("validate should be Success, got %s", report1.TaskResults["validate"].Status)
	}
	if report1.TaskResults["fetch_db"].Status != StatusFailed {
		t.Fatalf("fetch_db should be Failed, got %s", report1.TaskResults["fetch_db"].Status)
	}

	var foundDBErr *customDBError
	if !errors.As(report1.TaskResults["notify"].Error, &foundDBErr) {
		t.Errorf("notify's error should trace back to customDBError via errors.As, got %v", report1.TaskResults["notify"].Error)
	}

	var validateExecCount int32
	origValidate := o.tasks["validate"].Func
	o.tasks["validate"].Func = func(ctx context.Context) (interface{}, error) {
		atomic.AddInt32(&validateExecCount, 1)
		return origValidate(ctx)
	}

	errFunc := o.UpdateTaskFunc("fetch_db", func(ctx context.Context) (interface{}, error) {
		return []string{"row1", "row2", "row3"}, nil
	})
	if errFunc != nil {
		t.Fatalf("unexpected UpdateTaskFunc error: %v", errFunc)
	}

	errTimeout := o.UpdateTaskTimeout("fetch_db", 5*time.Second)
	if errTimeout != nil {
		t.Fatalf("unexpected UpdateTaskTimeout error: %v", errTimeout)
	}

	retryReport, retryErr := o.RetryTask(context.Background(), "fetch_db")
	if retryErr != nil {
		t.Fatalf("unexpected RetryTask error: %v", retryErr)
	}

	if !retryReport.Success {
		t.Error("expected overall success after retry")
	}

	if atomic.LoadInt32(&validateExecCount) != 0 {
		t.Errorf("validate should not be re-executed, got %d runs", validateExecCount)
	}

	if retryReport.TaskResults["validate"].Status != StatusSuccess {
		t.Errorf("validate should still be Success, got %s", retryReport.TaskResults["validate"].Status)
	}

	fetchResult := retryReport.TaskResults["fetch_db"]
	if fetchResult.Status != StatusSuccess {
		t.Errorf("fetch_db should be Success, got %s", fetchResult.Status)
	}
	rows, ok := fetchResult.Output.([]string)
	if !ok || len(rows) != 3 {
		t.Errorf("expected 3 rows output, got %v", fetchResult.Output)
	}

	if retryReport.TaskResults["process"].Status != StatusSuccess {
		t.Errorf("process should be Success, got %s", retryReport.TaskResults["process"].Status)
	}
	if retryReport.TaskResults["notify"].Status != StatusSuccess {
		t.Errorf("notify should be Success, got %s", retryReport.TaskResults["notify"].Status)
	}
}

func TestErrorPropagation_SkippedCausePropagates(t *testing.T) {
	o := NewOrchestrator()

	rootErr := errors.New("root boom")

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		return nil, rootErr
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c", nil
	}, 0, "b")

	report, _ := o.Run(context.Background())

	bErr := report.TaskResults["b"].Error
	if bErr == nil {
		t.Fatal("b should have error")
	}
	if !errors.Is(bErr, rootErr) {
		t.Errorf("b's error should contain root via errors.Is, got %v", bErr)
	}

	cErr := report.TaskResults["c"].Error
	if cErr == nil {
		t.Fatal("c should have error")
	}
	if !errors.Is(cErr, bErr) {
		t.Errorf("c's error should wrap b's error via errors.Is, got %v", cErr)
	}
	if !errors.Is(cErr, rootErr) {
		t.Errorf("c's error should still contain original root via errors.Is, got %v", cErr)
	}
}

func TestUpdateTaskTimeout_WhileRunning(t *testing.T) {
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

	err := o.UpdateTaskTimeout("slow", 5*time.Second)
	if err != ErrOrchestratorRunning {
		t.Errorf("expected ErrOrchestratorRunning, got %v", err)
	}

	wg.Wait()
}

func TestRetryTask_MultipleRetriesWithFix(t *testing.T) {
	o := NewOrchestrator()

	var aAttempts int32

	o.AddTask("a", "A", func(ctx context.Context) (interface{}, error) {
		a := atomic.AddInt32(&aAttempts, 1)
		return nil, fmt.Errorf("attempt %d fails", a)
	}, 0)
	o.AddTask("b", "B", func(ctx context.Context) (interface{}, error) {
		return "b-ok", nil
	}, 0, "a")
	o.AddTask("c", "C", func(ctx context.Context) (interface{}, error) {
		return "c-ok", nil
	}, 0, "b")

	r1, _ := o.Run(context.Background())
	if r1.TaskResults["a"].Status != StatusFailed {
		t.Fatalf("run 1: a should be Failed")
	}
	if atomic.LoadInt32(&aAttempts) != 1 {
		t.Errorf("run 1: expected 1 attempt for a, got %d", atomic.LoadInt32(&aAttempts))
	}

	r2, err2 := o.RetryTask(context.Background(), "a")
	if err2 != nil {
		t.Fatalf("retry 1 error: %v", err2)
	}
	if r2.TaskResults["a"].Status != StatusFailed {
		t.Errorf("retry 1: a should still be Failed")
	}
	if atomic.LoadInt32(&aAttempts) != 2 {
		t.Errorf("retry 1: expected 2 attempts for a, got %d", atomic.LoadInt32(&aAttempts))
	}

	o.UpdateTaskFunc("a", func(ctx context.Context) (interface{}, error) {
		return "a-fixed", nil
	})

	r3, err3 := o.RetryTask(context.Background(), "a")
	if err3 != nil {
		t.Fatalf("retry 2 error: %v", err3)
	}
	if !r3.Success {
		t.Error("retry 2: expected overall success after fix")
	}
	if r3.TaskResults["a"].Output != "a-fixed" {
		t.Errorf("retry 2: expected 'a-fixed', got %v", r3.TaskResults["a"].Output)
	}
	if r3.TaskResults["b"].Status != StatusSuccess {
		t.Errorf("retry 2: b should be Success, got %s", r3.TaskResults["b"].Status)
	}
	if r3.TaskResults["c"].Status != StatusSuccess {
		t.Errorf("retry 2: c should be Success, got %s", r3.TaskResults["c"].Status)
	}
	if atomic.LoadInt32(&aAttempts) != 2 {
		t.Errorf("after fix, aAttempts should still be 2 (fixed func replaced), got %d", atomic.LoadInt32(&aAttempts))
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
