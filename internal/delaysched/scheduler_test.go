package delaysched

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

func TestMinHeap_PushPop(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	t1 := &Task{ID: "1", ExecuteAt: now.Add(3 * time.Second)}
	t2 := &Task{ID: "2", ExecuteAt: now.Add(1 * time.Second)}
	t3 := &Task{ID: "3", ExecuteAt: now.Add(2 * time.Second)}

	heap.Push(h, t1)
	heap.Push(h, t2)
	heap.Push(h, t3)

	if h.Len() != 3 {
		t.Fatalf("expected heap length 3, got %d", h.Len())
	}

	popped := heap.Pop(h).(*Task)
	if popped.ID != "2" {
		t.Errorf("expected first pop to be task 2, got %s", popped.ID)
	}

	popped = heap.Pop(h).(*Task)
	if popped.ID != "3" {
		t.Errorf("expected second pop to be task 3, got %s", popped.ID)
	}

	popped = heap.Pop(h).(*Task)
	if popped.ID != "1" {
		t.Errorf("expected third pop to be task 1, got %s", popped.ID)
	}

	if h.Len() != 0 {
		t.Errorf("expected empty heap, got length %d", h.Len())
	}
}

func TestMinHeap_SameExecuteTime(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	sameTime := time.Now()
	t1 := &Task{ID: "a", ExecuteAt: sameTime}
	t2 := &Task{ID: "b", ExecuteAt: sameTime}

	heap.Push(h, t1)
	heap.Push(h, t2)

	if h.Len() != 2 {
		t.Fatalf("expected length 2, got %d", h.Len())
	}

	ids := make(map[string]bool)
	ids[heap.Pop(h).(*Task).ID] = true
	ids[heap.Pop(h).(*Task).ID] = true

	if !ids["a"] || !ids["b"] {
		t.Errorf("expected both tasks to be popped, got %v", ids)
	}
}

func TestMinHeap_Remove(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	t1 := &Task{ID: "1", ExecuteAt: now.Add(1 * time.Second)}
	t2 := &Task{ID: "2", ExecuteAt: now.Add(2 * time.Second)}
	t3 := &Task{ID: "3", ExecuteAt: now.Add(3 * time.Second)}

	heap.Push(h, t1)
	heap.Push(h, t2)
	heap.Push(h, t3)

	heap.Remove(h, t2.index)

	if h.Len() != 2 {
		t.Errorf("expected length 2, got %d", h.Len())
	}

	popped := heap.Pop(h).(*Task)
	if popped.ID != "1" {
		t.Errorf("expected task 1, got %s", popped.ID)
	}
	popped = heap.Pop(h).(*Task)
	if popped.ID != "3" {
		t.Errorf("expected task 3, got %s", popped.ID)
	}
}

func TestMinHeap_Fix(t *testing.T) {
	h := &taskHeap{}
	heap.Init(h)

	now := time.Now()
	t1 := &Task{ID: "1", ExecuteAt: now.Add(3 * time.Second)}
	t2 := &Task{ID: "2", ExecuteAt: now.Add(2 * time.Second)}
	t3 := &Task{ID: "3", ExecuteAt: now.Add(1 * time.Second)}

	heap.Push(h, t1)
	heap.Push(h, t2)
	heap.Push(h, t3)

	t1.ExecuteAt = now.Add(500 * time.Millisecond)
	heap.Fix(h, t1.index)

	popped := heap.Pop(h).(*Task)
	if popped.ID != "1" {
		t.Errorf("expected task 1 after fix, got %s", popped.ID)
	}
}

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

	err := s.AddWithID("t1", 100*time.Millisecond, fn("t1"))
	if err != nil {
		t.Fatalf("failed to add t1: %v", err)
	}

	err = s.AddWithID("t2", 50*time.Millisecond, fn("t2"))
	if err != nil {
		t.Fatalf("failed to add t2: %v", err)
	}

	err = s.AddWithID("t3", 150*time.Millisecond, fn("t3"))
	if err != nil {
		t.Fatalf("failed to add t3: %v", err)
	}

	if s.TaskCount() != 3 {
		t.Errorf("expected 3 tasks, got %d", s.TaskCount())
	}

	wg.Wait()
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"t1", "t2", "t3"} {
		if executed[id] != 1 {
			t.Errorf("task %s executed %d times, expected 1", id, executed[id])
		}
	}
}

func TestScheduler_Add_AutoID(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	wg.Add(2)

	id1, err := s.Add(50*time.Millisecond, func(_ context.Context) {
		wg.Done()
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}
	if id1 == "" {
		t.Error("Add returned empty ID")
	}

	id2, err := s.Add(60*time.Millisecond, func(_ context.Context) {
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

func TestScheduler_AddAt(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)
	executeAt := time.Now().Add(80 * time.Millisecond)

	id, err := s.AddAt(executeAt, func(_ context.Context) {
		done <- struct{}{}
	})
	if err != nil {
		t.Fatalf("AddAt failed: %v", err)
	}
	if id == "" {
		t.Error("empty id")
	}

	select {
	case <-done:
		elapsed := time.Since(executeAt)
		if elapsed < -50*time.Millisecond {
			t.Errorf("task executed too early, elapsed since executeAt: %v", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("task did not execute in time")
	}
}

func TestScheduler_Add_DuplicateID(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.AddWithID("dup", 100*time.Millisecond, func(_ context.Context) {})
	if err != nil {
		t.Fatalf("first AddWithID failed: %v", err)
	}

	err = s.AddWithID("dup", 200*time.Millisecond, func(_ context.Context) {})
	if !errors.Is(err, ErrTaskAlreadyExists) {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}
}

func TestScheduler_Cancel(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var executed int32

	err := s.AddWithID("cancel-me", 100*time.Millisecond, func(_ context.Context) {
		atomic.AddInt32(&executed, 1)
	})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	err = s.Cancel("cancel-me")
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}

	if s.TaskCount() != 0 {
		t.Errorf("expected 0 tasks after cancel, got %d", s.TaskCount())
	}

	time.Sleep(200 * time.Millisecond)

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

func TestScheduler_Cancel_AlreadyCancelled(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_ = s.AddWithID("twice", 500*time.Millisecond, func(_ context.Context) {})
	_ = s.Cancel("twice")

	err := s.Cancel("twice")
	if err != nil {
		t.Errorf("cancelling already cancelled task should return nil, got %v", err)
	}
}

func TestScheduler_Reschedule(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	start := time.Now()
	done := make(chan struct{}, 1)

	_ = s.AddWithID("resched", 300*time.Millisecond, func(_ context.Context) {
		done <- struct{}{}
	})

	newTime := time.Now().Add(80 * time.Millisecond)
	err := s.Reschedule("resched", newTime)
	if err != nil {
		t.Fatalf("Reschedule failed: %v", err)
	}

	select {
	case <-done:
		elapsed := time.Since(start)
		if elapsed > 200*time.Millisecond {
			t.Errorf("task executed at %v, expected around 80ms", elapsed)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("task did not execute in time")
	}
}

func TestScheduler_RescheduleDelay(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)

	_ = s.AddWithID("rdelay", 500*time.Millisecond, func(_ context.Context) {
		done <- struct{}{}
	})

	err := s.RescheduleDelay("rdelay", 80*time.Millisecond)
	if err != nil {
		t.Fatalf("RescheduleDelay failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Error("task did not execute after reschedule delay")
	}
}

func TestScheduler_Reschedule_NotFound(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.Reschedule("ghost", time.Now().Add(time.Second))
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestScheduler_Reschedule_LaterTime(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var executed int32

	_ = s.AddWithID("later", 50*time.Millisecond, func(_ context.Context) {
		atomic.AddInt32(&executed, 1)
	})

	err := s.RescheduleDelay("later", 300*time.Millisecond)
	if err != nil {
		t.Fatalf("Reschedule failed: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if atomic.LoadInt32(&executed) != 0 {
		t.Error("task executed too early after reschedule to later time")
	}
}

func TestScheduler_AddInterval(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var count int32
	done := make(chan struct{}, 1)

	fn := func(_ context.Context) {
		n := atomic.AddInt32(&count, 1)
		if n >= 3 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}

	id, err := s.AddInterval(20*time.Millisecond, 50*time.Millisecond, fn)
	if err != nil {
		t.Fatalf("AddInterval failed: %v", err)
	}
	if id == "" {
		t.Error("empty id")
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Errorf("interval task did not execute 3 times, count=%d", atomic.LoadInt32(&count))
	}

	finalCount := atomic.LoadInt32(&count)
	if finalCount < 3 {
		t.Errorf("expected at least 3 executions, got %d", finalCount)
	}
}

func TestScheduler_AddInterval_CancelStopsRepeat(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var count int32
	fn := func(_ context.Context) {
		atomic.AddInt32(&count, 1)
	}

	_ = s.AddIntervalWithID("repeat-cancel", 10*time.Millisecond, 30*time.Millisecond, fn)

	time.Sleep(60 * time.Millisecond)

	err := s.Cancel("repeat-cancel")
	if err != nil {
		t.Errorf("Cancel failed: %v", err)
	}

	countAfterCancel := atomic.LoadInt32(&count)
	time.Sleep(100 * time.Millisecond)
	countLater := atomic.LoadInt32(&count)

	if countLater != countAfterCancel {
		t.Errorf("task continued executing after cancel: before=%d after=%d", countAfterCancel, countLater)
	}
}

func TestScheduler_AddIntervalWithID_Duplicate(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.AddIntervalWithID("dup", 10*time.Millisecond, 100*time.Millisecond, func(_ context.Context) {})
	if err != nil {
		t.Fatalf("first AddIntervalWithID failed: %v", err)
	}

	err = s.AddIntervalWithID("dup", 10*time.Millisecond, 100*time.Millisecond, func(_ context.Context) {})
	if !errors.Is(err, ErrTaskAlreadyExists) {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}
}

func TestCron_Validate(t *testing.T) {
	tests := []struct {
		expr    string
		wantErr bool
	}{
		{"* * * * *", false},
		{"0 0 * * *", false},
		{"30 14 * * 1-5", false},
		{"*/15 * * * *", false},
		{"0 9-17 * * *", false},
		{"0,30 * * * *", false},
		{"5 4 * * 0", false},
		{"60 * * * *", true},
		{"* 24 * * *", true},
		{"* * 32 * *", true},
		{"* * * 13 *", true},
		{"* * * * 7", true},
		{"invalid", true},
		{"* * *", true},
		{"* * * * * *", true},
		{"a * * * *", true},
		{"* */0 * * *", true},
		{"* 5-3 * * *", true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			err := validateCron(tt.expr)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q, got nil", tt.expr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.expr, err)
			}
		})
	}
}

func TestCron_NextTime_Basic(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		expr string
		want time.Time
	}{
		{"35 10 * * *", time.Date(2025, 6, 15, 10, 35, 0, 0, time.UTC)},
		{"0 11 * * *", time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC)},
		{"0 0 * * *", time.Date(2025, 6, 16, 0, 0, 0, 0, time.UTC)},
		{"0 10 * * *", time.Date(2025, 6, 16, 10, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			got, err := nextCronTime(tt.expr, from)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("nextCronTime(%q, %v) = %v, want %v", tt.expr, from, got, tt.want)
			}
		})
	}
}

func TestCron_NextTime_StepAndRange(t *testing.T) {
	from := time.Date(2025, 6, 15, 10, 7, 0, 0, time.UTC)

	got, err := nextCronTime("*/15 * * * *", from)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2025, 6, 15, 10, 15, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("step cron: got %v, want %v", got, want)
	}

	from2 := time.Date(2025, 6, 15, 8, 0, 0, 0, time.UTC)
	got2, err := nextCronTime("0 9-17 * * *", from2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want2 := time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC)
	if !got2.Equal(want2) {
		t.Errorf("range cron: got %v, want %v", got2, want2)
	}
}

func TestScheduler_AddCron(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)
	start := time.Now()

	nextMinute := start.Add(2 * time.Minute).Minute()
	cronExpr := "* * * * *"

	_, err := s.AddCron(10*time.Millisecond, cronExpr, func(_ context.Context) {
		select {
		case done <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("AddCron failed: %v", err)
	}
	_ = nextMinute

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Error("cron task did not execute in time")
	}
}

func TestScheduler_AddCron_InvalidExpr(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_, err := s.AddCron(10*time.Millisecond, "invalid cron", func(_ context.Context) {})
	if !errors.Is(err, ErrInvalidCronExpr) {
		t.Errorf("expected ErrInvalidCronExpr, got %v", err)
	}

	err = s.AddCronWithID("badcron", 10*time.Millisecond, "* 25 * * *", func(_ context.Context) {})
	if !errors.Is(err, ErrInvalidCronExpr) {
		t.Errorf("expected ErrInvalidCronExpr for hour 25, got %v", err)
	}
}

func TestScheduler_AddCronWithID_Duplicate(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	err := s.AddCronWithID("crondup", 10*time.Millisecond, "* * * * *", func(_ context.Context) {})
	if err != nil {
		t.Fatalf("first AddCronWithID failed: %v", err)
	}

	err = s.AddCronWithID("crondup", 10*time.Millisecond, "* * * * *", func(_ context.Context) {})
	if !errors.Is(err, ErrTaskAlreadyExists) {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}
}

func TestScheduler_GetTask(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_ = s.AddWithID("get-test", 1*time.Hour, func(_ context.Context) {})

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

	_, err := s.Add(10*time.Millisecond, func(_ context.Context) {})
	if !errors.Is(err, ErrSchedulerStopped) {
		t.Errorf("expected ErrSchedulerStopped, got %v", err)
	}
}

func TestScheduler_StopBeforeExecute(t *testing.T) {
	s := NewScheduler()
	s.Start()

	var executed int32
	_ = s.AddWithID("stop-early", 500*time.Millisecond, func(_ context.Context) {
		atomic.AddInt32(&executed, 1)
	})

	s.Stop()
	time.Sleep(100 * time.Millisecond)

	if atomic.LoadInt32(&executed) != 0 {
		t.Error("task executed after scheduler stopped")
	}
}

func TestScheduler_MultipleOrder(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var orderMu sync.Mutex
	var order []string
	var wg sync.WaitGroup
	wg.Add(4)

	makeFn := func(id string) TaskFunc {
		return func(_ context.Context) {
			orderMu.Lock()
			order = append(order, id)
			orderMu.Unlock()
			wg.Done()
		}
	}

	_ = s.AddWithID("a", 200*time.Millisecond, makeFn("a"))
	_ = s.AddWithID("b", 50*time.Millisecond, makeFn("b"))
	_ = s.AddWithID("c", 150*time.Millisecond, makeFn("c"))
	_ = s.AddWithID("d", 100*time.Millisecond, makeFn("d"))

	wg.Wait()

	expected := []string{"b", "d", "c", "a"}
	orderMu.Lock()
	defer orderMu.Unlock()
	if len(order) != len(expected) {
		t.Fatalf("expected %d tasks, got %d: %v", len(expected), len(order), order)
	}
	for i, id := range expected {
		if order[i] != id {
			t.Errorf("order[%d] = %s, want %s. full order: %v", i, order[i], id, order)
		}
	}
}

func TestScheduler_TaskPanic(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)
	var count int32

	_ = s.AddIntervalWithID("panic-task", 10*time.Millisecond, 40*time.Millisecond, func(_ context.Context) {
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
	case <-time.After(500 * time.Millisecond):
		t.Errorf("task did not recover from panic. count=%d", atomic.LoadInt32(&count))
	}
}

func TestScheduler_EmptyHeap_Stop(t *testing.T) {
	s := NewScheduler()
	s.Start()

	done := make(chan struct{})
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Stop()
		done <- struct{}{}
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("Stop on empty heap deadlocked")
	}
}

func TestScheduler_Reschedule_Concurrent(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	n := 20

	_ = s.AddWithID("concurrent-resched", 5*time.Second, func(_ context.Context) {})

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.RescheduleDelay("concurrent-resched", time.Duration(1000+i)*time.Millisecond)
		}(i)
	}
	wg.Wait()

	task, err := s.GetTask("concurrent-resched")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if task.ExecuteAt.Before(time.Now()) {
		t.Errorf("task scheduled in past: %v", task.ExecuteAt)
	}
}

func TestScheduler_Add_Concurrent(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	var wg sync.WaitGroup
	var counter int32
	n := 50

	for i := 0; i < n; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("concurrent-add-%d", i)
			_ = s.AddWithID(id, 30*time.Millisecond, func(_ context.Context) {
				atomic.AddInt32(&counter, 1)
			})
		}(i)
		go func(i int) {
			defer wg.Done()
			s.Add(30*time.Millisecond, func(_ context.Context) {
				atomic.AddInt32(&counter, 1)
			})
		}(i)
	}
	wg.Wait()

	time.Sleep(300 * time.Millisecond)
	got := atomic.LoadInt32(&counter)
	if got < int32(n) {
		t.Errorf("expected at least %d executions, got %d", n, got)
	}
}

func TestScheduler_Cancel_WhileRunning(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	started := make(chan struct{}, 1)
	release := make(chan struct{}, 1)

	_ = s.AddWithID("running-task", 10*time.Millisecond, func(_ context.Context) {
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
	if err != nil && !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected nil or ErrTaskNotFound after completion, got %v", err)
	}
}

func TestScheduler_Reschedule_WhileRunning(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	started := make(chan struct{}, 1)
	release := make(chan struct{}, 1)

	_ = s.AddWithID("running-resched", 10*time.Millisecond, func(_ context.Context) {
		started <- struct{}{}
		<-release
	})

	<-started

	err := s.RescheduleDelay("running-resched", time.Hour)
	if !errors.Is(err, ErrTaskRunning) {
		t.Errorf("expected ErrTaskRunning, got %v", err)
	}

	release <- struct{}{}
}

func TestScheduler_NegativeDelay(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	done := make(chan struct{}, 1)

	_, err := s.Add(-1*time.Millisecond, func(_ context.Context) {
		done <- struct{}{}
	})
	if err != nil {
		t.Fatalf("Add with negative delay failed: %v", err)
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Error("negative delay task did not execute immediately")
	}
}
