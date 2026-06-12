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

// ------------------------------ Min-Heap Unit Tests ------------------------------

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

// ------------------------------ Scheduler Lifecycle & Basic Tests ------------------------------

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

// ------------------------------ Cancel Tests ------------------------------

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

// Cancel Pending 任务后 tasks map 被清理，再次 Cancel 应返回 ErrTaskNotFound
func TestScheduler_Cancel_AlreadyCancelled_ReturnsNotFound(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	_ = s.AddWithID("twice", 500*time.Millisecond, func(_ context.Context) {})
	err := s.Cancel("twice")
	if err != nil {
		t.Fatalf("first Cancel failed: %v", err)
	}

	// 由于 Pending 任务被取消时从 tasks map 中移除了，第二次取消找不到任务
	err = s.Cancel("twice")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound for second Cancel (task removed from map), got %v", err)
	}
}

// ------------------------------ Reschedule Tests ------------------------------

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

// ------------------------------ Interval Repeat Tests ------------------------------

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

// ------------------------------ Cron Expression Parsing ------------------------------

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

// ------------------------------ Cron Scheduler Semantic Tests ------------------------------

// 验证首次执行时间根据 Cron 表达式计算，而非立即执行
func TestScheduler_Cron_FirstExecuteByCronSemantics(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	now := time.Now()
	// 选择 5 分钟后的精确分钟，这样 Cron "M * * * *" 的首次执行就是 now+5min
	targetMinute := (now.Minute() + 5) % 60
	cronExpr := fmt.Sprintf("%d * * * *", targetMinute)

	taskID := "cron-first-semantic"
	err := s.AddCronWithID(taskID, 0, cronExpr, func(_ context.Context) {})
	if err != nil {
		t.Fatalf("AddCronWithID failed: %v", err)
	}

	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	// 首次执行时间的 Minute 必须等于 targetMinute（验证 Cron 语义，而非立即执行）
	if task.ExecuteAt.Minute() != targetMinute {
		t.Errorf("expected first ExecuteAt minute = %d (Cron semantics), got %d (ExecuteAt=%v)",
			targetMinute, task.ExecuteAt.Minute(), task.ExecuteAt)
	}

	// 验证首次执行时间在合理范围内（不超过 1 小时）
	upperBound := now.Add(65 * time.Minute)
	if task.ExecuteAt.After(upperBound) {
		t.Errorf("first ExecuteAt %v is too far in future (upper bound %v)", task.ExecuteAt, upperBound)
	}
	lowerBound := now.Add(4*time.Minute - 10*time.Second)
	if task.ExecuteAt.Before(lowerBound) {
		t.Errorf("first ExecuteAt %v is too early (lower bound %v)", task.ExecuteAt, lowerBound)
	}

	// 验证 RepeatType = RepeatCron 且 CronExpr 正确存储
	if task.RepeatType != RepeatCron {
		t.Errorf("expected RepeatType = RepeatCron, got %v", task.RepeatType)
	}
	if task.CronExpr != cronExpr {
		t.Errorf("expected CronExpr = %q, got %q", cronExpr, task.CronExpr)
	}
}

// 验证 Cron 任务第一次执行完毕后，第二次执行时间符合 Cron 语义
func TestScheduler_Cron_NextExecuteSemantics(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	firstExecuted := make(chan struct{}, 1)
	var firstExecuteAt time.Time

	// 先用通配 Cron 让首次 ExecuteAt = 下一分钟 00 秒
	cronExpr := "* * * * *"

	// 添加 Cron 任务，delay=0，首次执行时间由 Cron 决定（下一分钟）
	err := s.AddCronWithID("cron-next-semantic", 0, cronExpr, func(_ context.Context) {
		select {
		case firstExecuted <- struct{}{}:
			firstExecuteAt = time.Now()
		default:
		}
	})
	if err != nil {
		t.Fatalf("AddCronWithID failed: %v", err)
	}

	// 先确认首次 ExecuteAt 由 Cron 计算（而非立即执行）
	task, err := s.GetTask("cron-next-semantic")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	// 秒必须为 0（Cron 对齐到分钟边界）
	if task.ExecuteAt.Second() != 0 {
		t.Errorf("expected first ExecuteAt to be minute-aligned (second=0), got %v (second=%d)",
			task.ExecuteAt, task.ExecuteAt.Second())
	}

	// 将任务的执行时间手动改到很近的未来，以快速触发第一次实际执行
	err = s.RescheduleDelay("cron-next-semantic", 20*time.Millisecond)
	if err != nil {
		t.Fatalf("RescheduleDelay to near future failed: %v", err)
	}

	// 等待第一次执行
	select {
	case <-firstExecuted:
	case <-time.After(2 * time.Second):
		t.Fatal("first cron execution did not happen after reschedule")
	}

	// 给调度器一点时间把任务重新入堆
	time.Sleep(80 * time.Millisecond)

	task, err = s.GetTask("cron-next-semantic")
	if err != nil {
		t.Fatalf("GetTask after first exec failed: %v", err)
	}

	// 第二次执行时间应该是 firstExecuteAt 之后的下一分钟 00 秒
	expectedNext := firstExecuteAt.Add(time.Minute).Truncate(time.Minute)
	// 允许 ±5 秒误差
	diff := task.ExecuteAt.Sub(expectedNext)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("expected next ExecuteAt ≈ %v (minute-aligned after first exec), got %v (diff=%v)",
			expectedNext, task.ExecuteAt, diff)
	}

	// 验证第二次也是 minute-aligned
	if task.ExecuteAt.Second() != 0 {
		t.Errorf("expected next ExecuteAt to be minute-aligned (second=0), got %v (second=%d)",
			task.ExecuteAt, task.ExecuteAt.Second())
	}
}

// 验证无效 Cron 表达式在注册时被拒绝
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

// ------------------------------ Task Query & Other Tests ------------------------------

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
	// 对于执行中的一次性任务，应返回 ErrTaskRunning
	if !errors.Is(err, ErrTaskRunning) {
		t.Errorf("expected ErrTaskRunning, got %v", err)
	}

	release <- struct{}{}
	time.Sleep(50 * time.Millisecond)

	// 一次性任务执行完毕后被从 map 清理，再次 Cancel 返回 ErrTaskNotFound
	err = s.Cancel("running-task")
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound after completion (map cleaned), got %v", err)
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

// ------------------------------ Memory Leak Tests ------------------------------

// 验证：批量注册并取消 Pending 任务后，tasks map 不会无限增长
func TestScheduler_CancelPending_NoMemoryLeak(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	const iterations = 1000
	const batchSize = 100

	for i := 0; i < iterations; i++ {
		// 每轮添加一批任务
		for j := 0; j < batchSize; j++ {
			id := fmt.Sprintf("leak-pending-%d-%d", i, j)
			err := s.AddWithID(id, 10*time.Minute, func(_ context.Context) {})
			if err != nil {
				t.Fatalf("iteration %d AddWithID failed: %v", i, err)
			}
		}
		// 每轮再逐一取消
		for j := 0; j < batchSize; j++ {
			id := fmt.Sprintf("leak-pending-%d-%d", i, j)
			err := s.Cancel(id)
			if err != nil {
				t.Fatalf("iteration %d Cancel failed: %v", i, err)
			}
		}
	}

	// 所有任务都被取消，TaskCount 应为 0
	if count := s.TaskCount(); count != 0 {
		t.Errorf("after %d iterations of add+cancel, TaskCount = %d (expected 0) — memory leak?",
			iterations, count)
	}

	// 直接检查 tasks map 大小（通过取消一个不存在的 id 不能检查，但我们可以通过 TaskCount 间接验证）
	// 额外验证：再次添加 1 个任务后 TaskCount = 1
	singleID := "leak-verification-single"
	_ = s.AddWithID(singleID, 10*time.Minute, func(_ context.Context) {})
	if count := s.TaskCount(); count != 1 {
		t.Errorf("after adding single task, TaskCount = %d (expected 1) — stale entries remain?", count)
	}
	_ = s.Cancel(singleID)
	if count := s.TaskCount(); count != 0 {
		t.Errorf("after canceling single task, TaskCount = %d (expected 0)", count)
	}
}

// 验证：一次性任务执行完毕后，tasks map 中的条目被清理
func TestScheduler_CompletedOneTime_NoMemoryLeak(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		id := fmt.Sprintf("complete-once-%d", i)
		err := s.AddWithID(id, 10*time.Millisecond, func(_ context.Context) {
			wg.Done()
		})
		if err != nil {
			t.Fatalf("AddWithID %s failed: %v", id, err)
		}
	}

	wg.Wait()
	// 给调度器一些时间执行清理
	time.Sleep(100 * time.Millisecond)

	if count := s.TaskCount(); count != 0 {
		t.Errorf("after %d one-time tasks completed, TaskCount = %d (expected 0) — memory leak?", n, count)
	}
}

// 验证：周期性任务被取消后（即使是在 Running 状态下取消），tasks map 最终被清理
func TestScheduler_RepeatCancel_NoMemoryLeak(t *testing.T) {
	s := NewScheduler()
	s.Start()
	defer s.Stop()

	const n = 50
	var counts [50]int32
	var wg sync.WaitGroup
	wg.Add(n)

	for i := 0; i < n; i++ {
		i := i
		id := fmt.Sprintf("repeat-cancel-leak-%d", i)
		err := s.AddIntervalWithID(id, 10*time.Millisecond, 100*time.Millisecond, func(_ context.Context) {
			atomic.AddInt32(&counts[i], 1)
			// 第一次执行完成后释放 wg
			if atomic.LoadInt32(&counts[i]) == 1 {
				wg.Done()
			}
		})
		if err != nil {
			t.Fatalf("AddIntervalWithID %s failed: %v", id, err)
		}
	}

	// 等待所有任务至少执行一次（此时它们处于已执行或 Pending 下一轮的状态）
	wg.Wait()

	// 取消所有周期性任务
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("repeat-cancel-leak-%d", i)
		err := s.Cancel(id)
		// 某些任务可能正处于 Running，此时返回 ErrTaskRunning；但 Cancel 会将其标记为 Cancelled，
		// executeTask 完成后会清理
		if err != nil && !errors.Is(err, ErrTaskRunning) {
			t.Fatalf("Cancel %s unexpected error: %v", id, err)
		}
	}

	// 等待所有 Running 中的任务执行完毕并清理
	time.Sleep(300 * time.Millisecond)

	if count := s.TaskCount(); count != 0 {
		t.Errorf("after canceling all %d repeat tasks, TaskCount = %d (expected 0) — memory leak?", n, count)
	}
}

// ------------------------------ Cron End-to-End Tests ------------------------------

// 验证 Cron 任务完整端到端路径：入队 → Timer 自然等待 → 执行 → 重新入堆
// 本测试不使用 Reschedule 干预，完全依靠调度器的自然 Timer 唤醒机制
func TestScheduler_Cron_NaturalTimerEndToEnd(t *testing.T) {
	// 如果当前秒数很小，先等待到接近分钟切换（秒 >= 55），这样测试能在几秒内完成
	waitForNextMinuteBoundary(t)

	s := NewScheduler()
	s.Start()
	defer s.Stop()

	// 记录调度器注册任务时的时间
	registerTime := time.Now()

	// 计算预期的首次执行时间：下一分钟 00 秒
	expectedFirst := registerTime.Add(time.Minute).Truncate(time.Minute)
	waitDuration := expectedFirst.Sub(registerTime)

	// 如果等待时间太短（< 2 秒），可能存在竞态，跳过
	if waitDuration < 2*time.Second {
		t.Skipf("skip: wait duration %v too short, may have race condition", waitDuration)
	}

	t.Logf("Cron end-to-end: registerTime=%v, expectedFirst=%v, waitDuration=%v",
		registerTime, expectedFirst, waitDuration)

	firstExecuted := make(chan time.Time, 1)
	cronExpr := "* * * * *"

	taskID := "cron-end-to-end"
	err := s.AddCronWithID(taskID, 0, cronExpr, func(_ context.Context) {
		execTime := time.Now()
		t.Logf("Cron task executed at %v (expected first=%v)", execTime, expectedFirst)
		select {
		case firstExecuted <- execTime:
		default:
		}
	})
	if err != nil {
		t.Fatalf("AddCronWithID failed: %v", err)
	}

	// 1. 验证首次 ExecuteAt 由 Cron 计算正确
	task, err := s.GetTask(taskID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	// 首次执行时间必须是 minute-aligned（秒=0）
	if task.ExecuteAt.Second() != 0 {
		t.Fatalf("initial ExecuteAt should be minute-aligned (second=0), got %v (second=%d)",
			task.ExecuteAt, task.ExecuteAt.Second())
	}
	// 误差在 2 秒内
	diff := task.ExecuteAt.Sub(expectedFirst)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Fatalf("initial ExecuteAt = %v, expected ≈ %v (diff=%v)",
			task.ExecuteAt, expectedFirst, diff)
	}

	// 2. 等待 Timer 自然唤醒并执行（不走 Reschedule 捷径）
	select {
	case actualExecTime := <-firstExecuted:
		// 验证实际执行时间与预期的误差（允许 ±3 秒）
		execDiff := actualExecTime.Sub(expectedFirst)
		if execDiff < -3*time.Second || execDiff > 3*time.Second {
			t.Errorf("task executed at %v, expected ≈ %v (diff=%v, outside ±3s tolerance)",
				actualExecTime, expectedFirst, execDiff)
		}
	case <-time.After(waitDuration + 10*time.Second):
		t.Fatalf("Cron task did not execute within expected time window (expected within %v)",
			waitDuration+10*time.Second)
	}

	// 3. 给调度器一点时间完成后处理（重新入堆）
	time.Sleep(100 * time.Millisecond)

	// 4. 验证后处理逻辑：任务已被重新入堆，第二次 ExecuteAt 也是 minute-aligned
	task, err = s.GetTask(taskID)
	if err != nil {
		t.Fatalf("GetTask after first execution failed: %v — task was not rescheduled correctly", err)
	}
	// 状态必须回到 Pending
	if task.Status != StatusPending {
		t.Errorf("expected StatusPending after reschedule, got %v", task.Status)
	}
	// 第二次执行时间也必须是 minute-aligned（秒=0）
	if task.ExecuteAt.Second() != 0 {
		t.Errorf("second ExecuteAt should be minute-aligned (second=0), got %v (second=%d)",
			task.ExecuteAt, task.ExecuteAt.Second())
	}
	// 第二次执行时间应晚于第一次预期执行时间
	if !task.ExecuteAt.After(expectedFirst) {
		t.Errorf("second ExecuteAt %v should be after first execution time %v",
			task.ExecuteAt, expectedFirst)
	}
	// RepeatType 和 CronExpr 必须保持正确
	if task.RepeatType != RepeatCron {
		t.Errorf("expected RepeatType = RepeatCron, got %v", task.RepeatType)
	}
	if task.CronExpr != cronExpr {
		t.Errorf("expected CronExpr = %q, got %q", cronExpr, task.CronExpr)
	}
}

// 等待当前时间接近分钟切换（秒 >= 55），这样测试能快速完成
func waitForNextMinuteBoundary(t *testing.T) {
	for {
		now := time.Now()
		if now.Second() >= 55 {
			t.Logf("waitForNextMinuteBoundary: reached second=%d at %v", now.Second(), now)
			return
		}
		// 否则继续等待，每 1 秒检查一次
		sleepDur := time.Duration(55-now.Second()) * time.Second
		if sleepDur < 0 {
			sleepDur = 500 * time.Millisecond
		}
		t.Logf("waitForNextMinuteBoundary: current second=%d, waiting %v", now.Second(), sleepDur)
		time.Sleep(sleepDur)
	}
}
