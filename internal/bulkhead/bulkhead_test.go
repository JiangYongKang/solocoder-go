package bulkhead

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewBulkhead_InvalidConfig(t *testing.T) {
	_, err := NewBulkhead("", Config{MaxConcurrency: 1, MaxQueueSize: 1})
	if err != ErrInvalidName {
		t.Errorf("expected ErrInvalidName, got %v", err)
	}

	_, err = NewBulkhead("test", Config{MaxConcurrency: 0, MaxQueueSize: 1})
	if err != ErrInvalidConcurrency {
		t.Errorf("expected ErrInvalidConcurrency, got %v", err)
	}

	_, err = NewBulkhead("test", Config{MaxConcurrency: -1, MaxQueueSize: 1})
	if err != ErrInvalidConcurrency {
		t.Errorf("expected ErrInvalidConcurrency, got %v", err)
	}

	_, err = NewBulkhead("test", Config{MaxConcurrency: 1, MaxQueueSize: -1})
	if err != ErrInvalidQueueSize {
		t.Errorf("expected ErrInvalidQueueSize, got %v", err)
	}
}

func TestNewBulkhead_Success(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 3,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	if b.Name() != "test" {
		t.Errorf("expected name 'test', got '%s'", b.Name())
	}
	if b.MaxConcurrency() != 3 {
		t.Errorf("expected MaxConcurrency=3, got %d", b.MaxConcurrency())
	}
	if b.MaxQueueSize() != 5 {
		t.Errorf("expected MaxQueueSize=5, got %d", b.MaxQueueSize())
	}
	if b.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0, got %d", b.ActiveCount())
	}
	if b.QueueLength() != 0 {
		t.Errorf("expected QueueLength=0, got %d", b.QueueLength())
	}
	if b.WorkerCount() != 3 {
		t.Errorf("expected WorkerCount=3, got %d", b.WorkerCount())
	}
}

func TestSubmit_SingleTask(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	done := make(chan struct{})
	task := func() {
		close(done)
	}

	err = b.Submit(task)
	if err != nil {
		t.Fatalf("unexpected error on Submit: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task did not execute within timeout")
	}
}

func TestSubmit_NilTask(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	err = b.Submit(nil)
	if err == nil {
		t.Error("expected error for nil task")
	}

	_, err = b.TrySubmit(nil)
	if err == nil {
		t.Error("expected error for nil task in TrySubmit")
	}
}

func TestSubmit_ClosedBulkhead(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b.Close()
	b.Close()

	err = b.Submit(func() {})
	if err != ErrBulkheadClosed {
		t.Errorf("expected ErrBulkheadClosed, got %v", err)
	}

	_, err = b.TrySubmit(func() {})
	if err != ErrBulkheadClosed {
		t.Errorf("expected ErrBulkheadClosed from TrySubmit, got %v", err)
	}
}

func TestSubmit_QueueFull_NoWait(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
		WaitTimeout:    0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	task := func() {
		close(started)
		<-block
	}

	err = b.Submit(task)
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	err = b.Submit(func() {})
	if err != nil {
		t.Fatalf("second submit (queued) failed: %v", err)
	}

	if b.QueueLength() != 1 {
		t.Errorf("expected QueueLength=1, got %d", b.QueueLength())
	}

	err = b.Submit(func() {})
	if err == nil {
		t.Fatal("expected error for full bulkhead")
	}

	var fullErr *FullError
	if !errors.As(err, &fullErr) {
		t.Fatalf("expected FullError, got %T", err)
	}

	if fullErr.Name != "test" {
		t.Errorf("expected Name='test', got '%s'", fullErr.Name)
	}
	if fullErr.MaxConcurrency != 1 {
		t.Errorf("expected MaxConcurrency=1, got %d", fullErr.MaxConcurrency)
	}
	if fullErr.MaxQueueSize != 1 {
		t.Errorf("expected MaxQueueSize=1, got %d", fullErr.MaxQueueSize)
	}
	if fullErr.ActiveCount != 1 {
		t.Errorf("expected ActiveCount=1, got %d", fullErr.ActiveCount)
	}
	if fullErr.QueueLength != 1 {
		t.Errorf("expected QueueLength=1, got %d", fullErr.QueueLength)
	}

	close(block)
}

func TestSubmit_WaitTimeout(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   0,
		WaitTimeout:    100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	task := func() {
		close(started)
		<-block
	}

	err = b.Submit(task)
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	start := time.Now()
	err = b.Submit(func() {})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	var fullErr *FullError
	if !errors.As(err, &fullErr) {
		t.Fatalf("expected FullError, got %T", err)
	}

	if elapsed < 80*time.Millisecond {
		t.Errorf("expected to wait ~100ms, waited %v", elapsed)
	}

	close(block)
}

func TestSubmit_WaitSuccess(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   0,
		WaitTimeout:    500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	task := func() {
		close(started)
		<-block
	}

	err = b.Submit(task)
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	var submitErr error
	done := make(chan struct{})
	go func() {
		submitErr = b.Submit(func() {})
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	close(block)

	select {
	case <-done:
		if submitErr != nil {
			t.Fatalf("expected successful submit after wait, got: %v", submitErr)
		}
	case <-time.After(time.Second):
		t.Fatal("submit did not complete")
	}
}

func TestTrySubmit(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	task := func() {
		close(started)
		<-block
	}

	ok, err := b.TrySubmit(task)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected TrySubmit to succeed")
	}

	<-started

	ok, err = b.TrySubmit(func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected second TrySubmit to succeed (queued)")
	}

	ok, err = b.TrySubmit(func() {})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected third TrySubmit to fail (queue full)")
	}

	close(block)
}

func TestActiveCount_And_QueueLength(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	for i := 0; i < 2; i++ {
		err = b.Submit(func() {
			started <- struct{}{}
			<-block
		})
		if err != nil {
			t.Fatalf("submit %d failed: %v", i, err)
		}
	}

	for i := 0; i < 2; i++ {
		<-started
	}

	if b.ActiveCount() != 2 {
		t.Errorf("expected ActiveCount=2, got %d", b.ActiveCount())
	}

	for i := 0; i < 3; i++ {
		err = b.Submit(func() {})
		if err != nil {
			t.Fatalf("queued submit %d failed: %v", i, err)
		}
	}

	if b.QueueLength() != 3 {
		t.Errorf("expected QueueLength=3, got %d", b.QueueLength())
	}

	close(block)

	time.Sleep(100 * time.Millisecond)

	if b.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0 after all tasks done, got %d", b.ActiveCount())
	}
	if b.QueueLength() != 0 {
		t.Errorf("expected QueueLength=0 after all tasks done, got %d", b.QueueLength())
	}
}

func TestResize_Concurrency_Expand(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	if b.WorkerCount() != 2 {
		t.Fatalf("expected WorkerCount=2, got %d", b.WorkerCount())
	}

	err = b.Resize(5, 5)
	if err != nil {
		t.Fatalf("unexpected error on Resize: %v", err)
	}

	if b.MaxConcurrency() != 5 {
		t.Errorf("expected MaxConcurrency=5, got %d", b.MaxConcurrency())
	}
	if b.WorkerCount() != 5 {
		t.Errorf("expected WorkerCount=5, got %d", b.WorkerCount())
	}
}

func TestResize_Concurrency_Shrink(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 5,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	if b.WorkerCount() != 5 {
		t.Fatalf("expected WorkerCount=5, got %d", b.WorkerCount())
	}

	err = b.Resize(2, 5)
	if err != nil {
		t.Fatalf("unexpected error on Resize: %v", err)
	}

	if b.MaxConcurrency() != 2 {
		t.Errorf("expected MaxConcurrency=2, got %d", b.MaxConcurrency())
	}

	time.Sleep(100 * time.Millisecond)

	if b.WorkerCount() != 2 {
		t.Errorf("expected WorkerCount=2 after shrink, got %d", b.WorkerCount())
	}
}

func TestResize_Concurrency_ShrinkWithActiveTasks(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 3,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{}, 3)
	block := make(chan struct{})

	for i := 0; i < 3; i++ {
		err = b.Submit(func() {
			started <- struct{}{}
			<-block
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}

	for i := 0; i < 3; i++ {
		<-started
	}

	if b.ActiveCount() != 3 {
		t.Fatalf("expected ActiveCount=3, got %d", b.ActiveCount())
	}

	err = b.Resize(1, 5)
	if err != nil {
		t.Fatalf("unexpected error on Resize: %v", err)
	}

	if b.MaxConcurrency() != 1 {
		t.Errorf("expected MaxConcurrency=1, got %d", b.MaxConcurrency())
	}

	if b.ActiveCount() != 3 {
		t.Errorf("expected ActiveCount still 3 (tasks still running), got %d", b.ActiveCount())
	}

	if b.WorkerCount() != 3 {
		t.Errorf("expected WorkerCount still 3 (tasks still running), got %d", b.WorkerCount())
	}

	close(block)

	time.Sleep(100 * time.Millisecond)

	if b.ActiveCount() != 0 {
		t.Errorf("expected ActiveCount=0 after tasks complete, got %d", b.ActiveCount())
	}
	if b.WorkerCount() != 1 {
		t.Errorf("expected WorkerCount=1 after tasks complete and shrink, got %d", b.WorkerCount())
	}
}

func TestResize_Queue_Expand(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	err = b.Submit(func() {
		close(started)
		<-block
	})
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	for i := 0; i < 2; i++ {
		err = b.Submit(func() {})
		if err != nil {
			t.Fatalf("queued submit %d failed: %v", i, err)
		}
	}

	if b.QueueLength() != 2 {
		t.Fatalf("expected QueueLength=2, got %d", b.QueueLength())
	}

	err = b.Resize(1, 5)
	if err != nil {
		t.Fatalf("unexpected error on Resize: %v", err)
	}

	if b.MaxQueueSize() != 5 {
		t.Errorf("expected MaxQueueSize=5, got %d", b.MaxQueueSize())
	}
	if b.QueueLength() != 2 {
		t.Errorf("expected QueueLength=2 (tasks preserved), got %d", b.QueueLength())
	}

	for i := 0; i < 3; i++ {
		ok, _ := b.TrySubmit(func() {})
		if !ok {
			t.Errorf("expected TrySubmit %d to succeed after expansion", i)
		}
	}

	if b.QueueLength() != 5 {
		t.Errorf("expected QueueLength=5, got %d", b.QueueLength())
	}

	close(block)
}

func TestResize_Queue_Shrink(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	err = b.Submit(func() {
		close(started)
		<-block
	})
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	for i := 0; i < 5; i++ {
		err = b.Submit(func() {})
		if err != nil {
			t.Fatalf("queued submit %d failed: %v", i, err)
		}
	}

	if b.QueueLength() != 5 {
		t.Fatalf("expected QueueLength=5, got %d", b.QueueLength())
	}

	err = b.Resize(1, 2)
	if err != nil {
		t.Fatalf("unexpected error on Resize: %v", err)
	}

	if b.MaxQueueSize() != 2 {
		t.Errorf("expected MaxQueueSize=2, got %d", b.MaxQueueSize())
	}
	if b.QueueLength() != 5 {
		t.Errorf("expected QueueLength=5 (submitted tasks preserved), got %d", b.QueueLength())
	}

	ok, err := b.TrySubmit(func() {})
	if err != nil {
		t.Fatalf("unexpected error on TrySubmit: %v", err)
	}
	if ok {
		t.Error("expected TrySubmit to fail after queue shrink, but it succeeded")
	}

	close(block)
}

func TestResize_InvalidParams(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	err = b.Resize(0, 2)
	if err != ErrInvalidConcurrency {
		t.Errorf("expected ErrInvalidConcurrency, got %v", err)
	}

	err = b.Resize(2, -1)
	if err != ErrInvalidQueueSize {
		t.Errorf("expected ErrInvalidQueueSize, got %v", err)
	}

	b.Close()
	err = b.Resize(3, 3)
	if err != ErrBulkheadClosed {
		t.Errorf("expected ErrBulkheadClosed, got %v", err)
	}
}

func TestResize_SameSize(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	err = b.Resize(2, 3)
	if err != nil {
		t.Fatalf("unexpected error on Resize with same size: %v", err)
	}

	if b.MaxConcurrency() != 2 {
		t.Errorf("expected MaxConcurrency=2, got %d", b.MaxConcurrency())
	}
	if b.MaxQueueSize() != 3 {
		t.Errorf("expected MaxQueueSize=3, got %d", b.MaxQueueSize())
	}
	if b.WorkerCount() != 2 {
		t.Errorf("expected WorkerCount=2, got %d", b.WorkerCount())
	}
}

func TestRegistry_Basic(t *testing.T) {
	r := NewRegistry()
	defer r.CloseAll()

	b, err := r.NewBulkhead("service-a", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b == nil {
		t.Fatal("expected non-nil bulkhead")
	}

	names := r.Names()
	if len(names) != 1 {
		t.Errorf("expected 1 bulkhead, got %d", len(names))
	}
	if names[0] != "service-a" {
		t.Errorf("expected name 'service-a', got '%s'", names[0])
	}

	b2, ok := r.Get("service-a")
	if !ok {
		t.Fatal("expected to find bulkhead 'service-a'")
	}
	if b2 != b {
		t.Error("expected same bulkhead instance")
	}

	_, ok = r.Get("service-b")
	if ok {
		t.Error("expected not to find bulkhead 'service-b'")
	}
}

func TestRegistry_DuplicateName(t *testing.T) {
	r := NewRegistry()
	defer r.CloseAll()

	_, err := r.NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = r.NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestRegistry_Remove(t *testing.T) {
	r := NewRegistry()
	defer r.CloseAll()

	_, err := r.NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r.Remove("test")

	_, ok := r.Get("test")
	if ok {
		t.Error("expected bulkhead to be removed")
	}

	r.Remove("nonexistent")
}

func TestBulkhead_Isolation(t *testing.T) {
	r := NewRegistry()
	defer r.CloseAll()

	b1, _ := r.NewBulkhead("service-a", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   1,
	})
	b2, _ := r.NewBulkhead("service-b", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   3,
	})

	started1 := make(chan struct{})
	block1 := make(chan struct{})
	err := b1.Submit(func() {
		close(started1)
		<-block1
	})
	if err != nil {
		t.Fatalf("b1 submit failed: %v", err)
	}
	<-started1

	err = b1.Submit(func() {})
	if err != nil {
		t.Fatalf("b1 second submit (queued) failed: %v", err)
	}

	err = b1.Submit(func() {})
	if err == nil {
		t.Fatal("expected b1 to be full")
	}

	var fullErr *FullError
	errors.As(err, &fullErr)
	if fullErr.Name != "service-a" {
		t.Errorf("expected error for service-a, got '%s'", fullErr.Name)
	}

	err = b2.Submit(func() {})
	if err != nil {
		t.Errorf("b2 should still accept tasks, got error: %v", err)
	}

	close(block1)
}

func TestConcurrent_Submit(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 5,
		MaxQueueSize:   50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	var wg sync.WaitGroup
	var counter int64
	numTasks := 100

	for i := 0; i < numTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := b.Submit(func() {
				atomic.AddInt64(&counter, 1)
			})
			if err != nil {
				t.Errorf("submit failed: %v", err)
			}
		}()
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	finalCount := atomic.LoadInt64(&counter)
	if finalCount != int64(numTasks) {
		t.Errorf("expected %d tasks executed, got %d", numTasks, finalCount)
	}
}

func TestFullError_ErrorString(t *testing.T) {
	err := &FullError{
		Name:           "test-svc",
		ActiveCount:    5,
		MaxConcurrency: 5,
		QueueLength:    10,
		MaxQueueSize:   10,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
	if !contains(msg, "test-svc") {
		t.Errorf("error message should contain name, got: %s", msg)
	}
}

func TestZeroQueueSize(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer b.Close()

	started := make(chan struct{})
	block := make(chan struct{})

	err = b.Submit(func() {
		close(started)
		<-block
	})
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	err = b.Submit(func() {})
	if err == nil {
		t.Fatal("expected error when queue size is 0 and worker is busy")
	}

	close(block)
}

func TestClose_WithPendingTasks(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 2,
		MaxQueueSize:   10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var counter int64
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		err = b.Submit(func() {
			defer wg.Done()
			atomic.AddInt64(&counter, 1)
			time.Sleep(10 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("submit failed: %v", err)
		}
	}

	b.Close()

	wg.Wait()

	if atomic.LoadInt64(&counter) != 5 {
		t.Errorf("expected 5 tasks to complete, got %d", atomic.LoadInt64(&counter))
	}
}

func TestSubmit_WaitClosedDuringWait(t *testing.T) {
	b, err := NewBulkhead("test", Config{
		MaxConcurrency: 1,
		MaxQueueSize:   0,
		WaitTimeout:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	started := make(chan struct{})
	block := make(chan struct{})

	err = b.Submit(func() {
		close(started)
		<-block
	})
	if err != nil {
		t.Fatalf("first submit failed: %v", err)
	}

	<-started

	var submitErr error
	submitDone := make(chan struct{})
	go func() {
		submitErr = b.Submit(func() {})
		close(submitDone)
	}()

	time.Sleep(50 * time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		b.Close()
		close(closeDone)
	}()

	select {
	case <-submitDone:
		if submitErr != ErrBulkheadClosed {
			t.Errorf("expected ErrBulkheadClosed, got %v", submitErr)
		}
	case <-time.After(time.Second):
		t.Fatal("submit should have been unblocked by close")
	}

	close(block)

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("close should have completed after task finishes")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
