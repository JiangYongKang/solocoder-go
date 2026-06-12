# 延迟任务调度器 (delaysched) 模块需求文档

## 1. 模块概述

延迟任务调度器（Delay Scheduler）是一个基于最小堆实现的任务调度引擎，用于管理和执行延迟任务。支持一次性延迟任务、周期性重复任务（固定间隔或 Cron 表达式）、任务取消以及任务执行时间的动态调整。

该模块位于 `internal/delaysched/` 包下，提供线程安全的 API，可在多协程并发环境中使用。

## 2. 功能需求

### 2.1 定时任务注册
- 支持注册一个在指定延迟后执行的任务
- 每个任务具有唯一标识（ID）、执行函数和执行时间
- 支持自动生成任务 ID 或用户指定自定义 ID
- 任务 ID 重复时返回 `ErrTaskAlreadyExists` 错误

### 2.2 最小堆排序
- 使用最小堆（Min-Heap）数据结构管理待执行任务
- 任务按执行时间（`ExecuteAt`）升序排列，堆顶始终为最近到期的任务
- 调度器循环从堆顶取出最近到期的任务执行
- 支持通过 `heap.Fix` 动态调整堆中任务的位置

### 2.3 任务取消
- 支持在任务到期执行前通过任务标识取消该任务
- 已开始执行的任务（状态为 `StatusRunning`）不可取消，返回 `ErrTaskRunning` 错误
- 已取消或已完成的任务再次取消时返回 nil（幂等操作）
- 不存在的任务返回 `ErrTaskNotFound` 错误

### 2.4 动态重排
- 支持修改已注册但未执行任务的执行时间
- 修改后任务在堆中的位置自动调整（通过 `heap.Fix` 实现）
- 已开始执行的任务不可重排，返回 `ErrTaskRunning` 错误
- 已取消或已完成的任务返回 `ErrTaskNotFound` 错误

### 2.5 重复执行
- 支持注册周期性任务
- **固定间隔模式**：任务每次执行完毕后按固定时间间隔重新注册下一次执行
- **Cron 表达式模式**：根据标准 5 段式 Cron 表达式计算下一次执行时间
- 周期性任务可通过 `Cancel` 停止后续执行

## 3. 核心结构体与职责

### 3.1 Task（任务）

```go
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
```

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 任务唯一标识 |
| ExecuteAt | time.Time | 任务执行时间 |
| Func | TaskFunc | 任务执行函数，签名为 `func(ctx context.Context)` |
| RepeatType | RepeatType | 重复类型：不重复/固定间隔/Cron |
| Interval | time.Duration | 固定间隔模式的执行间隔 |
| CronExpr | string | Cron 表达式（5 段式） |
| Status | TaskStatus | 任务状态：待执行/执行中/已取消/已完成 |
| index | int | 任务在堆中的索引位置（内部使用） |

### 3.2 Scheduler（调度器）

```go
type Scheduler struct {
    mu       sync.Mutex
    heap     *taskHeap
    tasks    map[string]*Task
    running  bool
    stopCh   chan struct{}
    wakeCh   chan struct{}
    wg       sync.WaitGroup
    ctx      context.Context
    cancel   context.CancelFunc
    nextID   uint64
    idMu     sync.Mutex
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| mu | sync.Mutex | 保护堆和任务映射的互斥锁 |
| heap | *taskHeap | 最小堆，存储待执行任务 |
| tasks | map[string]*Task | 任务 ID 到任务指针的映射，用于快速查找 |
| running | bool | 调度器运行状态标志 |
| stopCh | chan struct{} | 停止信号通道 |
| wakeCh | chan struct{} | 唤醒信号通道，用于通知调度循环重新检查堆 |
| wg | sync.WaitGroup | 等待调度协程退出 |
| ctx | context.Context | 根上下文，传递给任务执行函数 |
| cancel | context.CancelFunc | 取消函数，Stop 时调用 |
| nextID | uint64 | 自动生成 ID 的计数器 |
| idMu | sync.Mutex | 保护 ID 生成的互斥锁 |

### 3.3 taskHeap（最小堆）

实现了 Go 标准库 `container/heap` 接口的最小堆，元素为 `*Task`，按 `ExecuteAt` 升序排列。

### 3.4 枚举类型

**RepeatType（重复类型）**：
- `RepeatNone`：不重复，一次性任务
- `RepeatInterval`：固定间隔重复
- `RepeatCron`：Cron 表达式重复

**TaskStatus（任务状态）**：
- `StatusPending`：待执行
- `StatusRunning`：执行中
- `StatusCancelled`：已取消
- `StatusDone`：已完成

### 3.5 错误变量

| 错误变量 | 说明 |
|----------|------|
| ErrTaskNotFound | 任务不存在 |
| ErrTaskAlreadyExists | 任务 ID 已存在 |
| ErrTaskRunning | 任务正在执行，无法取消或重排 |
| ErrInvalidCronExpr | Cron 表达式无效 |
| ErrSchedulerStopped | 调度器未启动或已停止 |

## 4. 最小堆调度机制

### 4.1 数据结构

调度器内部维护两个数据结构：
1. **最小堆（taskHeap）**：按执行时间排序，保证 O(1) 获取最近到期任务，O(log n) 插入和删除
2. **哈希表（tasks map）**：通过任务 ID 快速查找任务指针，O(1) 查找

### 4.2 调度循环 (runLoop)

调度器启动后在独立协程中运行 `runLoop`，其核心逻辑如下：

```
循环:
    加锁
    检查调度器是否停止 -> 退出
    
    清理堆顶已取消/已完成的任务
    
    如果堆为空:
        解锁
        等待 stopCh（停止）或 wakeCh（新任务加入）
    
    获取堆顶任务，计算等待时间
    如果任务已到期:
        弹出堆顶，标记为 Running
        解锁
        执行任务（executeTask）
        继续下一轮循环
    
    否则:
        创建定时器（Timer）
        解锁
        等待定时器到期 / wakeCh（堆变化） / stopCh（停止）
```

### 4.3 唤醒机制 (wakeCh)

由于调度循环在等待定时器时可能持有旧的等待时间，当有新任务加入、任务被取消或重排时，需要立即唤醒调度循环重新检查堆顶。

通过 `wake()` 方法关闭当前 `wakeCh` 并创建新 channel，所有等待该 channel 的协程都会被唤醒。

### 4.4 任务执行 (executeTask)

1. 在独立协程中执行任务函数，使用 `recover()` 捕获 panic，保证调度器不崩溃
2. 任务执行完成后：
   - 一次性任务：从 tasks 映射中删除，状态设为 Done
   - 周期性任务：根据 RepeatType 计算下次执行时间，重新压入堆中
   - 已被取消的任务：清理并退出

## 5. API 接口

### 5.1 创建与生命周期

| 方法 | 签名 | 说明 |
|------|------|------|
| NewScheduler | `func NewScheduler() *Scheduler` | 创建新的调度器实例 |
| Start | `func (s *Scheduler) Start()` | 启动调度器，开始调度循环 |
| Stop | `func (s *Scheduler) Stop()` | 停止调度器，等待所有执行中任务完成 |

### 5.2 任务注册（一次性任务）

| 方法 | 签名 | 说明 |
|------|------|------|
| Add | `func (s *Scheduler) Add(delay time.Duration, fn TaskFunc) (string, error)` | 添加延迟任务，自动生成 ID |
| AddAt | `func (s *Scheduler) AddAt(executeAt time.Time, fn TaskFunc) (string, error)` | 添加指定执行时间的任务，自动生成 ID |
| AddWithID | `func (s *Scheduler) AddWithID(id string, delay time.Duration, fn TaskFunc) error` | 添加延迟任务，指定 ID |
| AddAtWithID | `func (s *Scheduler) AddAtWithID(id string, executeAt time.Time, fn TaskFunc) error` | 添加指定执行时间的任务，指定 ID |

### 5.3 任务注册（周期性任务）

| 方法 | 签名 | 说明 |
|------|------|------|
| AddInterval | `func (s *Scheduler) AddInterval(delay, interval time.Duration, fn TaskFunc) (string, error)` | 添加固定间隔重复任务，自动生成 ID |
| AddIntervalWithID | `func (s *Scheduler) AddIntervalWithID(id string, delay, interval time.Duration, fn TaskFunc) error` | 添加固定间隔重复任务，指定 ID |
| AddCron | `func (s *Scheduler) AddCron(delay time.Duration, cronExpr string, fn TaskFunc) (string, error)` | 添加 Cron 表达式重复任务，自动生成 ID |
| AddCronWithID | `func (s *Scheduler) AddCronWithID(id string, delay time.Duration, cronExpr string, fn TaskFunc) error` | 添加 Cron 表达式重复任务，指定 ID |

### 5.4 任务管理

| 方法 | 签名 | 说明 |
|------|------|------|
| Cancel | `func (s *Scheduler) Cancel(id string) error` | 取消任务 |
| Reschedule | `func (s *Scheduler) Reschedule(id string, newExecuteAt time.Time) error` | 重排任务到新的执行时间 |
| RescheduleDelay | `func (s *Scheduler) RescheduleDelay(id string, newDelay time.Duration) error` | 重排任务到新的延迟时间 |
| GetTask | `func (s *Scheduler) GetTask(id string) (*Task, error)` | 获取任务信息 |
| TaskCount | `func (s *Scheduler) TaskCount() int` | 获取当前待执行和执行中的任务数量 |

### 5.5 Cron 表达式格式

支持标准 5 段式 Cron 表达式：

```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日 (1 - 31)
│ │ │ ┌───────────── 月 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6, 0=周日)
│ │ │ │ │
* * * * *
```

支持的语法：
- `*`：匹配任意值
- `,`：列举多个值，如 `0,30`
- `-`：范围，如 `9-17`
- `/`：步长，如 `*/15`（每 15 单位），`10-20/5`（10 到 20 之间每 5 单位）

## 6. 使用示例

### 6.1 基础使用：一次性延迟任务

```go
package main

import (
    "context"
    "fmt"
    "time"
    "solocoder-go/internal/delaysched"
)

func main() {
    s := delaysched.NewScheduler()
    s.Start()
    defer s.Stop()

    // 3 秒后执行
    s.Add(3*time.Second, func(ctx context.Context) {
        fmt.Println("任务执行了！")
    })

    // 指定 ID 和精确执行时间
    s.AddAtWithID("task-1", time.Now().Add(5*time.Second), func(ctx context.Context) {
        fmt.Println("task-1 执行了！")
    })

    time.Sleep(6 * time.Second)
}
```

### 6.2 固定间隔重复任务

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

// 延迟 1 秒后开始，每 2 秒执行一次
s.AddIntervalWithID("heartbeat", 1*time.Second, 2*time.Second, func(ctx context.Context) {
    fmt.Println("心跳:", time.Now().Format("15:04:05"))
})

time.Sleep(10 * time.Second)
// 取消重复任务
s.Cancel("heartbeat")
```

### 6.3 Cron 表达式任务

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

// 每天 9:00 和 18:00 执行
cronExpr := "0 9,18 * * *"
_, err := s.AddCron(0, cronExpr, func(ctx context.Context) {
    fmt.Println("打卡时间到！")
})
if err != nil {
    // Cron 表达式无效
    panic(err)
}

// 工作日（周一至周五）9:30 执行
s.AddCronWithID("workday-checkin", 0, "30 9 * * 1-5", func(ctx context.Context) {
    fmt.Println("工作日早会")
})
```

### 6.4 任务取消与动态重排

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

// 添加一个 10 秒后执行的任务
s.AddWithID("important", 10*time.Second, func(ctx context.Context) {
    fmt.Println("重要任务执行")
})

// 决定提前到 2 秒后执行
s.RescheduleDelay("important", 2*time.Second)

// 或者取消任务
// s.Cancel("important")

time.Sleep(3 * time.Second)
```

### 6.5 并发安全使用

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(i int) {
        defer wg.Done()
        id := fmt.Sprintf("task-%d", i)
        s.AddWithID(id, time.Duration(i)*10*time.Millisecond, func(ctx context.Context) {
            fmt.Printf("Task %d executed\n", i)
        })
    }(i)
}
wg.Wait()
```

## 7. 测试覆盖

单元测试位于 `internal/delaysched/scheduler_test.go`，覆盖以下场景：

- **最小堆操作**：Push/Pop、相同执行时间、Remove、Fix 调整
- **任务注册与执行**：自动 ID、指定 ID、指定执行时间、重复 ID、执行顺序
- **任务取消**：正常取消、任务不存在、重复取消、执行中取消
- **动态重排**：提前执行、延后执行、任务不存在、执行中重排、并发重排
- **周期性任务**：固定间隔重复、取消停止重复、重复 ID、任务 panic 恢复
- **Cron 支持**：表达式校验、下一次执行时间计算、无效表达式
- **调度器生命周期**：Start/Stop 幂等性、空堆 Stop、停止后添加任务
- **边界条件**：负延迟（立即执行）、并发添加任务
