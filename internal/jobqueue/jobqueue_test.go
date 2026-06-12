package jobqueue

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func makeSuccessHandler() JobHandler {
	return func(ctx context.Context, job *Job) (interface{}, error) {
		return fmt.Sprintf("result:%s", job.ID), nil
	}
}

func makeDelayHandler(delay time.Duration) JobHandler {
	return func(ctx context.Context, job *Job) (interface{}, error) {
		time.Sleep(delay)
		return fmt.Sprintf("result:%s", job.ID), nil
	}
}

func makeFailHandler(maxFails int) JobHandler {
	var mu sync.Mutex
	failCount := make(map[string]int)
	return func(ctx context.Context, job *Job) (interface{}, error) {
		mu.Lock()
		defer mu.Unlock()
		failCount[job.ID]++
		if failCount[job.ID] <= maxFails {
			return nil, fmt.Errorf("fail #%d for %s", failCount[job.ID], job.ID)
		}
		return fmt.Sprintf("success:%s", job.ID), nil
	}
}

func makeAlwaysFailHandler() JobHandler {
	return func(ctx context.Context, job *Job) (interface{}, error) {
		return nil, errors.New("always fail")
	}
}

func TestNewJobQueue_InvalidConfig(t *testing.T) {
	_, err := NewJobQueue(Config{PoolSize: 0})
	if err != ErrPoolSizeInvalid {
		t.Errorf("expected ErrPoolSizeInvalid, got %v", err)
	}

	_, err = NewJobQueue(Config{PoolSize: -1})
	if err != ErrPoolSizeInvalid {
		t.Errorf("expected ErrPoolSizeInvalid, got %v", err)
	}
}

func TestNewJobQueue_DefaultValues(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: -1,
		ShutdownTimeout: -1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()

	if jq.cfg.DefaultMaxRetry != 0 {
		t.Errorf("expected DefaultMaxRetry=0, got %d", jq.cfg.DefaultMaxRetry)
	}
	if jq.cfg.ShutdownTimeout <= 0 {
		t.Errorf("expected positive ShutdownTimeout, got %v", jq.cfg.ShutdownTimeout)
	}
}

func TestStart_HandlerNotSet(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()

	err = jq.Start()
	if err != ErrHandlerNotSet {
		t.Errorf("expected ErrHandlerNotSet, got %v", err)
	}
}

func TestStart_Success(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()

	jq.SetHandler(makeSuccessHandler())
	err = jq.Start()
	if err != nil {
		t.Errorf("unexpected error on Start: %v", err)
	}

	err = jq.Start()
	if err != nil {
		t.Errorf("expected no error on second Start, got %v", err)
	}
}

func TestEnqueue_QueueStopped(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	jq.SetHandler(makeSuccessHandler())

	_, err = jq.Enqueue("job1", 1, "payload", 0)
	if err != ErrQueueStopped {
		t.Errorf("expected ErrQueueStopped, got %v", err)
	}
}

func TestEnqueue_AutoGenerateID(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id1, err := jq.Enqueue("", 1, "payload1", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	id2, err := jq.Enqueue("", 1, "payload2", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	if id1 == "" || id2 == "" {
		t.Error("expected non-empty auto-generated IDs")
	}
	if id1 == id2 {
		t.Error("expected different auto-generated IDs")
	}
}

func TestEnqueue_SameID_NoDuplicate(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	id, err := jq.Enqueue("same-id", 1, "payload1", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}
	if id != "same-id" {
		t.Errorf("expected id=same-id, got %s", id)
	}

	id2, err := jq.Enqueue("same-id", 1, "payload2", 0)
	if err != nil {
		t.Fatalf("second Enqueue failed: %v", err)
	}
	if id2 != "same-id" {
		t.Errorf("expected id=same-id, got %s", id2)
	}
}

func TestEnqueueAndExecute_Success(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.Enqueue("job-success", 1, "test-payload", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
	expectedResult := fmt.Sprintf("result:%s", jobID)
	if result.Result != expectedResult {
		t.Errorf("expected result=%s, got %v", expectedResult, result.Result)
	}

	status, err := jq.GetJobStatus(jobID)
	if err != nil {
		t.Errorf("GetJobStatus failed: %v", err)
	}
	if status != JobStatusCompleted {
		t.Errorf("expected status=completed, got %s", status)
	}
}

func TestGetResult_NotFound(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = jq.GetResult("non-existent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestGetJobStatus_NotFound(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, err = jq.GetJobStatus("non-existent")
	if err != ErrJobNotFound {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestPriorityQueue_HighPriorityFirst(t *testing.T) {
	var mu sync.Mutex
	var executionOrder []string
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		mu.Lock()
		executionOrder = append(executionOrder, job.ID)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	ids := []string{"job-low1", "job-high1", "job-low2", "job-high2", "job-mid"}
	priorities := []int{1, 10, 2, 9, 5}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		for i, id := range ids {
			_, _ = jq.Enqueue(id, priorities[i], "payload", 0)
		}
	}()
	wg.Wait()

	time.Sleep(500 * time.Millisecond)
	jq.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != 5 {
		t.Fatalf("expected 5 executions, got %d: %v", len(executionOrder), executionOrder)
	}

	highPriorityIdx := -1
	midPriorityIdx := -1
	lowPriorityIdx := -1
	for i, id := range executionOrder {
		switch id {
		case "job-high1", "job-high2":
			if highPriorityIdx == -1 {
				highPriorityIdx = i
			}
		case "job-mid":
			midPriorityIdx = i
		case "job-low1", "job-low2":
			if lowPriorityIdx == -1 {
				lowPriorityIdx = i
			}
		}
	}

	if highPriorityIdx >= midPriorityIdx && midPriorityIdx != -1 {
		t.Errorf("high priority should execute before mid. order: %v", executionOrder)
	}
	if midPriorityIdx >= lowPriorityIdx && lowPriorityIdx != -1 && midPriorityIdx != -1 {
		t.Errorf("mid priority should execute before low. order: %v", executionOrder)
	}
}

func TestPriorityQueue_SamePriority_FIFO(t *testing.T) {
	var mu sync.Mutex
	var executionOrder []string
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		mu.Lock()
		executionOrder = append(executionOrder, job.ID)
		mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	expected := []string{"job-a", "job-b", "job-c", "job-d", "job-e"}
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		for _, id := range expected {
			_, _ = jq.Enqueue(id, 5, "payload", 0)
			time.Sleep(1 * time.Millisecond)
		}
	}()
	wg.Wait()

	time.Sleep(500 * time.Millisecond)
	jq.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d executions, got %d", len(expected), len(executionOrder))
	}

	firstIdx := make(map[string]int)
	for i, id := range executionOrder {
		if _, ok := firstIdx[id]; !ok {
			firstIdx[id] = i
		}
	}

	for i := 0; i < len(expected)-1; i++ {
		a, b := expected[i], expected[i+1]
		if firstIdx[a] > firstIdx[b] {
			t.Errorf("FIFO order violated: %s (idx=%d) should be before %s (idx=%d). actual order: %v",
				a, firstIdx[a], b, firstIdx[b], executionOrder)
		}
	}
}

func TestDelayTask_Execution(t *testing.T) {
	var mu sync.Mutex
	executed := make(map[string]time.Time)
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		mu.Lock()
		executed[job.ID] = time.Now()
		mu.Unlock()
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	enqueueTime := time.Now()
	delay := 200 * time.Millisecond
	jobID, err := jq.Enqueue("delayed-job", 1, "payload", delay)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	if _, ok := executed[jobID]; ok {
		mu.Unlock()
		t.Fatal("delayed job executed too early")
	}
	mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}

	mu.Lock()
	execTime, ok := executed[jobID]
	mu.Unlock()
	if !ok {
		t.Fatal("job not recorded as executed")
	}
	actualDelay := execTime.Sub(enqueueTime)
	if actualDelay < delay-10*time.Millisecond {
		t.Errorf("expected delay >= %v, actual delay was %v", delay, actualDelay)
	}
}

func TestDelayTask_ZeroDelay(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.Enqueue("zero-delay", 1, "payload", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error != nil {
		t.Errorf("expected no error, got %v", result.Error)
	}
}

func TestPoolSize_LimitConcurrency(t *testing.T) {
	poolSize := 3
	var concurrentCount int64
	var maxConcurrent int64

	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		current := atomic.AddInt64(&concurrentCount, 1)
		defer atomic.AddInt64(&concurrentCount, -1)

		for {
			max := atomic.LoadInt64(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: poolSize})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobCount := 10
	var wg sync.WaitGroup
	wg.Add(jobCount)
	for i := 0; i < jobCount; i++ {
		go func(idx int) {
			defer wg.Done()
			jobID := fmt.Sprintf("conc-job-%d", idx)
			_, _ = jq.Enqueue(jobID, 1, "payload", 0)
		}(i)
	}
	wg.Wait()

	time.Sleep(1 * time.Second)
	jq.Stop()

	if max := atomic.LoadInt64(&maxConcurrent); max > int64(poolSize) {
		t.Errorf("max concurrent %d exceeded pool size %d", max, poolSize)
	}
	if jq.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0 after stop, got %d", jq.ActiveCount())
	}
}

func TestPoolSize_BlockingWhenFull(t *testing.T) {
	poolSize := 2
	var mu sync.Mutex
	var started int
	blocked := make(chan struct{})
	continueCh := make(chan struct{})

	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		mu.Lock()
		started++
		if started == poolSize {
			close(blocked)
		}
		mu.Unlock()
		<-continueCh
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: poolSize})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() {
		close(continueCh)
		jq.Stop()
	}()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < poolSize+1; i++ {
		jobID := fmt.Sprintf("block-job-%d", i)
		_, _ = jq.Enqueue(jobID, 1, "payload", 0)
	}

	select {
	case <-blocked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for pool to fill")
	}

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	cnt := started
	mu.Unlock()
	if cnt != poolSize {
		t.Errorf("expected %d started jobs, got %d", poolSize, cnt)
	}
	if jq.ActiveCount() != poolSize {
		t.Errorf("expected ActiveCount=%d, got %d", poolSize, jq.ActiveCount())
	}
	if jq.PendingCount() != 1 {
		t.Errorf("expected PendingCount=1, got %d", jq.PendingCount())
	}
}

func TestRetry_SuccessAfterFailures(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: 3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeFailHandler(2))
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.EnqueueWithRetry("retry-succeed", 1, "payload", 0, 2)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error != nil {
		t.Errorf("expected success after retries, got error: %v", result.Error)
	}

	status, err := jq.GetJobStatus(jobID)
	if err != nil {
		t.Errorf("GetJobStatus failed: %v", err)
	}
	if status != JobStatusCompleted {
		t.Errorf("expected status=completed, got %s", status)
	}
}

func TestRetry_ExhaustedToDeadLetter(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeAlwaysFailHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.EnqueueWithRetry("dead-letter-job", 1, "payload", 0, 2)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for job exhausted retries")
	}

	time.Sleep(100 * time.Millisecond)
	if jq.DeadLetterCount() != 1 {
		t.Errorf("expected DeadLetterCount=1, got %d", jq.DeadLetterCount())
	}

	deadLetters := jq.GetDeadLetters()
	if len(deadLetters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(deadLetters))
	}
	if deadLetters[0].ID != jobID {
		t.Errorf("expected dead letter id=%s, got %s", jobID, deadLetters[0].ID)
	}
	if deadLetters[0].Status != JobStatusDeadLetter {
		t.Errorf("expected dead letter status, got %s", deadLetters[0].Status)
	}
	if deadLetters[0].RetryCount <= 2 {
		t.Errorf("expected RetryCount > 2, got %d", deadLetters[0].RetryCount)
	}
}

func TestRetry_DefaultMaxRetry(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeAlwaysFailHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.Enqueue("default-retry", 1, "payload", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for job exhausted retries")
	}

	time.Sleep(100 * time.Millisecond)
	deadLetters := jq.GetDeadLetters()
	found := false
	for _, dl := range deadLetters {
		if dl.ID == jobID {
			found = true
			if dl.RetryCount != 2 {
				t.Errorf("expected RetryCount=2 (max=1), got %d", dl.RetryCount)
			}
			break
		}
	}
	if !found {
		t.Error("job not found in dead letters")
	}
}

func TestBackoffDelay_Increasing(t *testing.T) {
	job := &Job{RetryCount: 0}
	d1 := job.BackoffDelay()
	job.RetryCount = 1
	d2 := job.BackoffDelay()
	job.RetryCount = 2
	d3 := job.BackoffDelay()
	job.RetryCount = 3
	d4 := job.BackoffDelay()

	if d1 >= d2 {
		t.Errorf("backoff not increasing: d1=%v >= d2=%v", d1, d2)
	}
	if d2 >= d3 {
		t.Errorf("backoff not increasing: d2=%v >= d3=%v", d2, d3)
	}
	if d3 >= d4 {
		t.Errorf("backoff not increasing: d3=%v >= d4=%v", d3, d4)
	}

	expected1 := 100 * time.Millisecond
	expected2 := 200 * time.Millisecond
	expected3 := 400 * time.Millisecond
	expected4 := 800 * time.Millisecond

	job.RetryCount = 0
	if job.BackoffDelay() != expected1 {
		t.Errorf("RetryCount=0: expected %v, got %v", expected1, job.BackoffDelay())
	}
	job.RetryCount = 1
	if job.BackoffDelay() != expected2 {
		t.Errorf("RetryCount=1: expected %v, got %v", expected2, job.BackoffDelay())
	}
	job.RetryCount = 2
	if job.BackoffDelay() != expected3 {
		t.Errorf("RetryCount=2: expected %v, got %v", expected3, job.BackoffDelay())
	}
	job.RetryCount = 3
	if job.BackoffDelay() != expected4 {
		t.Errorf("RetryCount=3: expected %v, got %v", expected4, job.BackoffDelay())
	}
}

func TestClearDeadLetters(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeAlwaysFailHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		jobIDs[i], _ = jq.Enqueue(fmt.Sprintf("dl-clear-%d", i), 1, "payload", 0)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, id := range jobIDs {
		_, _ = jq.WaitForResult(ctx, id)
	}

	time.Sleep(100 * time.Millisecond)
	if jq.DeadLetterCount() != 3 {
		t.Errorf("expected 3 dead letters, got %d", jq.DeadLetterCount())
	}

	jq.ClearDeadLetters()
	if jq.DeadLetterCount() != 0 {
		t.Errorf("expected 0 dead letters after clear, got %d", jq.DeadLetterCount())
	}
	if len(jq.GetDeadLetters()) != 0 {
		t.Error("GetDeadLetters should return empty after clear")
	}
}

func TestHandlerPanic_Recovered(t *testing.T) {
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		if job.ID == "panic-job" {
			panic("something went wrong")
		}
		return "ok", nil
	}

	jq, err := NewJobQueue(Config{
		PoolSize:        2,
		DefaultMaxRetry: 0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, err := jq.Enqueue("panic-job", 1, "payload", 0)
	if err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := jq.WaitForResult(ctx, jobID)
	if err != nil {
		t.Fatalf("WaitForResult failed: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error from panic recovery")
	}

	normalID, _ := jq.Enqueue("normal-job", 1, "payload", 0)
	result2, err := jq.WaitForResult(ctx, normalID)
	if err != nil {
		t.Fatalf("WaitForResult for normal job failed: %v", err)
	}
	if result2.Error != nil {
		t.Errorf("normal job should succeed, got error: %v", result2.Error)
	}
}

func TestWaitForResult_Timeout(t *testing.T) {
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		time.Sleep(2 * time.Second)
		return "done", nil
	}

	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	jobID, _ := jq.Enqueue("timeout-job", 1, "payload", 0)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err = jq.WaitForResult(ctx, jobID)
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestStop_Idempotent(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	jq.SetHandler(makeSuccessHandler())

	jq.Stop()
	jq.Stop()
}

func TestStop_WaitsForRunningJobs(t *testing.T) {
	var mu sync.Mutex
	completed := make(map[string]bool)
	handler := func(ctx context.Context, job *Job) (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		completed[job.ID] = true
		mu.Unlock()
		return job.ID, nil
	}

	jq, err := NewJobQueue(Config{PoolSize: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	jq.SetHandler(handler)
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, _ = jq.Enqueue(fmt.Sprintf("stop-wait-%d", i), 1, "payload", 0)
	}

	time.Sleep(20 * time.Millisecond)
	jq.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(completed) != 5 {
		t.Errorf("expected all 5 jobs completed after stop, got %d", len(completed))
	}
}

func TestPendingCount_CountsDelayedAndReady(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeDelayHandler(100 * time.Millisecond))
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_, _ = jq.Enqueue("pending-a", 1, "payload", 0)
	_, _ = jq.Enqueue("pending-b", 1, "payload", 500*time.Millisecond)
	_, _ = jq.Enqueue("pending-c", 1, "payload", 1*time.Second)

	time.Sleep(10 * time.Millisecond)
	pending := jq.PendingCount()
	if pending < 2 {
		t.Errorf("expected at least 2 pending (1 delayed + possibly 1 waiting), got %d", pending)
	}
}

func TestCompletedCount(t *testing.T) {
	jq, err := NewJobQueue(Config{PoolSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	for i := 0; i < 5; i++ {
		_, _ = jq.Enqueue(fmt.Sprintf("complete-%d", i), 1, "payload", 0)
	}

	time.Sleep(500 * time.Millisecond)
	if jq.CompletedCount() != 5 {
		t.Errorf("expected CompletedCount=5, got %d", jq.CompletedCount())
	}

	jq.ClearResults()
	if jq.CompletedCount() != 0 {
		t.Errorf("expected CompletedCount=0 after clear, got %d", jq.CompletedCount())
	}
}

func TestConcurrentEnqueueAndExecute(t *testing.T) {
	jq, err := NewJobQueue(Config{
		PoolSize:        5,
		DefaultMaxRetry: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer jq.Stop()
	jq.SetHandler(makeSuccessHandler())
	if err := jq.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	numJobs := 100
	var success int64
	var enqueueErr int64
	var wg sync.WaitGroup

	wg.Add(numJobs)
	for i := 0; i < numJobs; i++ {
		go func(idx int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-%d", idx)
			jobID, err := jq.Enqueue(id, idx%10, fmt.Sprintf("payload-%d", idx), 0)
			if err != nil {
				atomic.AddInt64(&enqueueErr, 1)
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			result, err := jq.WaitForResult(ctx, jobID)
			if err == nil && result.Error == nil {
				atomic.AddInt64(&success, 1)
			}
		}(i)
	}
	wg.Wait()

	if enqueueErr != 0 {
		t.Errorf("got %d enqueue errors", enqueueErr)
	}
	if success != int64(numJobs) {
		t.Errorf("expected %d successful jobs, got %d", numJobs, success)
	}
}

func TestJob_IsReady(t *testing.T) {
	now := time.Now()

	job := &Job{ReadyTime: now.Add(1 * time.Hour)}
	if job.IsReady() {
		t.Error("job with future ReadyTime should not be ready")
	}

	job2 := &Job{ReadyTime: now.Add(-1 * time.Hour)}
	if !job2.IsReady() {
		t.Error("job with past ReadyTime should be ready")
	}

	job3 := &Job{ReadyTime: now}
	time.Sleep(1 * time.Millisecond)
	if !job3.IsReady() {
		t.Error("job with current ReadyTime should be ready")
	}
}

func TestNewJob_InitialState(t *testing.T) {
	now := time.Now()
	job := NewJob("test-id", 5, "my-payload", 3, 2*time.Second)

	if job.ID != "test-id" {
		t.Errorf("expected ID=test-id, got %s", job.ID)
	}
	if job.Priority != 5 {
		t.Errorf("expected Priority=5, got %d", job.Priority)
	}
	if job.Payload != "my-payload" {
		t.Errorf("expected Payload=my-payload, got %v", job.Payload)
	}
	if job.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", job.MaxRetries)
	}
	if job.Delay != 2*time.Second {
		t.Errorf("expected Delay=2s, got %v", job.Delay)
	}
	if job.Status != JobStatusPending {
		t.Errorf("expected Status=pending, got %s", job.Status)
	}
	if job.RetryCount != 0 {
		t.Errorf("expected RetryCount=0, got %d", job.RetryCount)
	}
	if job.EnqueueTime.Before(now) || job.EnqueueTime.After(now.Add(10*time.Millisecond)) {
		t.Errorf("EnqueueTime not set correctly: %v vs now %v", job.EnqueueTime, now)
	}
	expectedReady := now.Add(2 * time.Second)
	diff := job.ReadyTime.Sub(expectedReady)
	if diff < -10*time.Millisecond || diff > 10*time.Millisecond {
		t.Errorf("ReadyTime not set correctly: %v vs expected %v", job.ReadyTime, expectedReady)
	}
}

func TestPriorityQueue_Sorting(t *testing.T) {
	pq := NewPriorityQueue()

	j1 := &Job{ID: "j1", Priority: 1, EnqueueTime: time.Now().Add(-3 * time.Hour)}
	j2 := &Job{ID: "j2", Priority: 5, EnqueueTime: time.Now().Add(-2 * time.Hour)}
	j3 := &Job{ID: "j3", Priority: 10, EnqueueTime: time.Now().Add(-1 * time.Hour)}
	j4 := &Job{ID: "j4", Priority: 5, EnqueueTime: time.Now()}

	heap.Push(pq, j1)
	heap.Push(pq, j2)
	heap.Push(pq, j3)
	heap.Push(pq, j4)

	out1 := heap.Pop(pq).(*Job)
	if out1.ID != "j3" {
		t.Errorf("first pop should be highest priority j3, got %s", out1.ID)
	}

	out2 := heap.Pop(pq).(*Job)
	if out2.ID != "j2" {
		t.Errorf("second pop should be j2 (same priority as j4 but earlier), got %s", out2.ID)
	}

	out3 := heap.Pop(pq).(*Job)
	if out3.ID != "j4" {
		t.Errorf("third pop should be j4 (same priority as j2 but later), got %s", out3.ID)
	}

	out4 := heap.Pop(pq).(*Job)
	if out4.ID != "j1" {
		t.Errorf("fourth pop should be lowest priority j1, got %s", out4.ID)
	}
}

func TestDelayQueue_Sorting(t *testing.T) {
	dq := NewDelayQueue()

	now := time.Now()
	j1 := &Job{ID: "d1", ReadyTime: now.Add(3 * time.Hour)}
	j2 := &Job{ID: "d2", ReadyTime: now.Add(1 * time.Hour)}
	j3 := &Job{ID: "d3", ReadyTime: now.Add(2 * time.Hour)}

	heap.Push(dq, j1)
	heap.Push(dq, j2)
	heap.Push(dq, j3)

	if dq.Peek().ID != "d2" {
		t.Errorf("Peek should return earliest ready time d2, got %s", dq.Peek().ID)
	}

	out1 := heap.Pop(dq).(*Job)
	if out1.ID != "d2" {
		t.Errorf("first pop should be d2, got %s", out1.ID)
	}

	if dq.Peek().ID != "d3" {
		t.Errorf("Peek should now return d3, got %s", dq.Peek().ID)
	}

	out2 := heap.Pop(dq).(*Job)
	if out2.ID != "d3" {
		t.Errorf("second pop should be d3, got %s", out2.ID)
	}

	out3 := heap.Pop(dq).(*Job)
	if out3.ID != "d1" {
		t.Errorf("third pop should be d1, got %s", out3.ID)
	}

	if dq.Peek() != nil {
		t.Error("Peek should return nil for empty queue")
	}
}
