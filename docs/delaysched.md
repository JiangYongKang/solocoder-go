# 延迟任务调度器 (delaysched) 模块需求文档

## 1. 模块概述

延迟任务调度器（Delay Scheduler）是一个基于最小堆实现的任务调度引擎，用于管理和执行延迟任务。支持一次性延迟任务、周期性重复任务（固定间隔或 Cron 表达式）、任务取消以及任务执行时间的动态调整。

该模块位于 `internal/delaysched/` 包下，提供线程安全的 API，可在多协程并发环境中使用。任务执行采用异步模型，调度循环不会被单个任务的执行时长所阻塞。

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
- 已开始执行的任务（状态为 `StatusRunning`）不可强制中止，返回 `ErrTaskRunning` 错误；若为周期性任务，会被标记为已取消，执行完毕后不再重新入堆
- 取消 Pending 状态的任务时，立即从最小堆和 tasks map 中双重移除，避免内存泄漏
- 不存在的任务返回 `ErrTaskNotFound` 错误
- 任务取消后不可恢复，如需重新执行需重新注册

### 2.4 动态重排
- 支持修改已注册但未执行任务的执行时间
- 修改后任务在堆中的位置自动调整（通过 `heap.Fix` 实现）
- 已开始执行的任务不可重排，返回 `ErrTaskRunning` 错误
- 已取消或已完成的任务返回 `ErrTaskNotFound` 错误

### 2.5 重复执行
- 支持注册周期性任务
- **固定间隔模式**：任务每次执行完毕后按固定时间间隔重新注册下一次执行
- **Cron 表达式模式**：首次和后续执行时间均严格按 Cron 语义计算（从 `now+delay` 起找下一个匹配时间点）
- 周期性任务可通过 `Cancel` 停止后续执行，Pending 时取消立即从 map 清理，Running 时取消在执行完毕后清理

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
    taskWg   sync.WaitGroup
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
| tasks | map[string]*Task | 任务 ID 到任务指针的映射，用于快速查找和内存泄漏检测 |
| running | bool | 调度器运行状态标志 |
| stopCh | chan struct{} | 停止信号通道 |
| wakeCh | chan struct{} | 唤醒信号通道，用于通知调度循环重新检查堆 |
| wg | sync.WaitGroup | 等待调度循环（runLoop）协程退出 |
| taskWg | sync.WaitGroup | 等待所有执行中（Running）任务协程完成，Stop 时使用 |
| ctx | context.Context | 根上下文，传递给每个任务执行函数 |
| cancel | context.CancelFunc | 取消函数，Stop 时调用，通知所有任务的子 ctx |
| nextID | uint64 | 自动生成 ID 的单调递增计数器 |
| idMu | sync.Mutex | 保护 ID 生成的互斥锁 |

### 3.3 taskHeap（最小堆）

实现了 Go 标准库 `container/heap` 接口的最小堆，元素为 `*Task`，按 `ExecuteAt` 升序排列。堆中可能残留状态已变更为 Cancelled/Done/Running 的元素，在调度循环的每次迭代开始时会从堆顶批量清理。

### 3.4 枚举类型

**RepeatType（重复类型）**：
- `RepeatNone`：不重复，一次性任务
- `RepeatInterval`：固定间隔重复
- `RepeatCron`：Cron 表达式重复

**TaskStatus（任务状态）**：
- `StatusPending`：待执行，位于最小堆中等待调度
- `StatusRunning`：执行中，任务函数正在 goroutine 内运行
- `StatusCancelled`：已取消，执行完毕后会立即从 tasks map 清理
- `StatusDone`：已完成，已从 tasks map 清理（理论上不会在 map 中看到）

### 3.5 错误变量

| 错误变量 | 说明 |
|----------|------|
| ErrTaskNotFound | 任务不存在（从未注册、已完成、或已被取消后清理） |
| ErrTaskAlreadyExists | 任务 ID 已存在，无法重复注册 |
| ErrTaskRunning | 任务正在执行中，无法立即取消或重排 |
| ErrInvalidCronExpr | Cron 表达式格式不合法或字段越界 |
| ErrSchedulerStopped | 调度器未启动或已停止，不接受任务注册 |

## 4. 最小堆调度机制

### 4.1 数据结构

调度器内部维护两个数据结构：
1. **最小堆（taskHeap）**：按执行时间排序，保证 O(1) 获取最近到期任务，O(log n) 插入和删除
2. **哈希表（tasks map）**：通过任务 ID 快速查找任务指针，O(1) 查找；同时用于检测和验证内存泄漏清理

### 4.2 调度循环 (runLoop)

调度器启动后在独立协程中运行 `runLoop`，其核心逻辑如下：

```
循环:
    加锁
    检查调度器是否停止 -> 退出
    
    // 惰性清理堆顶无效任务（已 Cancelled/Done/Running 但仍在堆顶的）
    while 堆非空 且 堆顶任务状态为 Cancelled/Done/Running:
        Pop 堆顶
        continue
    
    如果堆为空:
        wakeCh = 当前 s.wakeCh
        解锁
        select:
            stopCh -> 退出
            wakeCh -> 继续下一轮
        跳到循环开始
    
    取堆顶任务（不出堆），计算 waitTime = task.ExecuteAt - now
    
    如果 waitTime <= 0（任务到期）:
        Pop 堆顶
        task.Status = Running
        taskWg.Add(1)
        解锁
        启动独立 goroutine: executeTask(task)
        继续下一轮循环（不阻塞等待任务完成）
    
    否则（任务未到期）:
        wakeCh = 当前 s.wakeCh
        创建 Timer(waitTime)
        解锁
        select:
            Timer.C -> 下一轮检查到期
            wakeCh -> 停止定时器，下一轮重新计算堆顶
            stopCh -> 停止定时器，退出
```

### 4.3 唤醒机制 (wakeCh)

由于调度循环在等待定时器时可能持有旧的等待时间，当有以下事件发生时需要立即唤醒调度循环重新检查堆顶：

- 新任务加入（`Add*` 系列方法）
- 任务被取消（`Cancel`）
- 任务执行时间被调整（`Reschedule*`）
- 周期性任务执行完毕后重新入堆
- 调度器 Stop 时

通过 `wake()` 方法关闭当前 `wakeCh` 并创建新 channel，所有等待该 channel 的 select 分支都会被立即触发。

### 4.4 任务执行模型 (executeTask)

任务执行采用**异步并发模型**，调度循环不会被任何任务的执行时长阻塞：

1. 调度循环检测到任务到期后，通过 `go s.executeTask(task)` 启动独立 goroutine 执行任务
2. 使用 `taskWg.Add(1)` / `taskWg.Done()` 追踪运行中任务数量，供 Stop 等待
3. 任务函数外层包裹匿名函数 + `defer recover()`，捕获 panic 保证调度器整体不崩溃
4. 每个任务获得独立的 `context.WithCancel(s.ctx)`，Stop 时会取消所有运行中任务的 ctx
5. 任务执行完毕后，根据不同情况做清理和重入堆：

```
executeTask(task):
    defer taskWg.Done()
    
    // 执行任务函数，捕获 panic
    func():
        defer recover()
        ctx, cancel = context.WithCancel(s.ctx)
        defer cancel()
        task.Func(ctx)
    ()
    
    加锁
    if task.Status == StatusCancelled:
        delete(tasks, task.ID)      // 周期任务 Running 期间被 Cancel，执行完清理
        解锁
        return
    
    if task.RepeatType == RepeatNone:
        delete(tasks, task.ID)      // 一次性任务执行完毕，立即清理
        解锁
        return
    
    // 周期性任务：计算下一次执行时间并重新入堆
    switch RepeatType:
        RepeatInterval -> nextAt = time.Now() + Interval
        RepeatCron   -> nextAt = nextCronTime(CronExpr, time.Now())
    
    task.ExecuteAt = nextAt
    task.Status = Pending
    heap.Push(task)
    wake()
    解锁
```

### 4.5 任务取消清理机制（内存泄漏防护）

为防止 tasks map 因长期运行而无限增长，取消操作针对不同状态采取分级清理策略：

#### 4.5.1 Pending 状态取消

```
加锁
tasks 中不存在 -> 返回 ErrTaskNotFound
task.Status = StatusRunning -> 返回 ErrTaskRunning
task.Status = Pending:
    heap.Remove(s.heap, task.index)   // O(log n) 从最小堆移除
    delete(tasks, id)                 // ★ 关键：立即从 map 删除，防止泄漏
    wake()                            // 唤醒调度循环
    返回 nil
```

**效果**：Pending 取消是"完全删除"，堆和 map 都不再保留该任务的任何引用，GC 可立即回收。

#### 4.5.2 Running 状态取消

- **一次性任务（RepeatNone）**：直接返回 `ErrTaskRunning`，不做任何状态修改；任务执行完毕后 executeTask 会自行从 map 清理
- **周期性任务（RepeatInterval / RepeatCron）**：返回 `ErrTaskRunning`，但将 `task.Status = StatusCancelled`；executeTask 在任务结束时检测到该状态会立即执行 `delete(tasks, task.ID)`

**效果**：Running 取消不强行中止正在运行的函数，但保证后续不再重复执行，且执行完毕后 map 条目被清理。

#### 4.5.3 执行完毕后的清理

无论是一次性任务还是周期性任务，executeTask 末尾都会对 `tasks` map 做对应清理，确保所有任务的生命周期在 tasks map 中有明确的终点。

### 4.6 Stop 的等待语义

`Stop()` 会依次等待两个 `WaitGroup`：
1. `s.wg.Wait()`：等待调度循环（runLoop）协程退出
2. `s.taskWg.Wait()`：等待所有已启动但未完成的任务 goroutine 执行完毕

保证 Stop 返回时没有任何残余的调度或任务执行协程。

## 5. API 接口

### 5.1 创建与生命周期

| 方法 | 签名 | 说明 |
|------|------|------|
| NewScheduler | `func NewScheduler() *Scheduler` | 创建新的调度器实例 |
| Start | `func (s *Scheduler) Start()` | 启动调度器，开始调度循环（可多次安全调用） |
| Stop | `func (s *Scheduler) Stop()` | 停止调度器，等待调度循环和所有运行中任务全部完成后返回 |

### 5.2 任务注册（一次性任务）

| 方法 | 签名 | 说明 |
|------|------|------|
| Add | `func (s *Scheduler) Add(delay time.Duration, fn TaskFunc) (string, error)` | 添加延迟任务，自动生成 ID |
| AddAt | `func (s *Scheduler) AddAt(executeAt time.Time, fn TaskFunc) (string, error)` | 添加指定执行时间点的任务，自动生成 ID |
| AddWithID | `func (s *Scheduler) AddWithID(id string, delay time.Duration, fn TaskFunc) error` | 添加延迟任务，指定 ID |
| AddAtWithID | `func (s *Scheduler) AddAtWithID(id string, executeAt time.Time, fn TaskFunc) error` | 添加指定执行时间点的任务，指定 ID |

### 5.3 任务注册（周期性任务）

| 方法 | 签名 | 说明 |
|------|------|------|
| AddInterval | `func (s *Scheduler) AddInterval(delay, interval time.Duration, fn TaskFunc) (string, error)` | 固定间隔重复任务，自动生成 ID；首次 `now+delay`，后续每次 `prevFinish + interval` |
| AddIntervalWithID | `func (s *Scheduler) AddIntervalWithID(id string, delay, interval time.Duration, fn TaskFunc) error` | 固定间隔重复任务，指定 ID |
| AddCron | `func (s *Scheduler) AddCron(delay time.Duration, cronExpr string, fn TaskFunc) (string, error)` | Cron 表达式重复任务，自动生成 ID；首次与后续均按 Cron 语义计算 |
| AddCronWithID | `func (s *Scheduler) AddCronWithID(id string, delay time.Duration, cronExpr string, fn TaskFunc) error` | Cron 表达式重复任务，指定 ID |

> **Cron 关键说明**：`AddCron` 与 `AddCronWithID` 的首次执行时间 **不是** `now+delay`，而是 `nextCronTime(cronExpr, now.Add(delay))` —— 即从 `now+delay` 这一时刻开始，寻找满足 Cron 表达式的下一个时间点。之后每次执行完毕都以 `time.Now()` 为基准计算下一个 Cron 时间点。

### 5.4 任务管理

| 方法 | 签名 | 说明 |
|------|------|------|
| Cancel | `func (s *Scheduler) Cancel(id string) error` | 取消任务；Pending 时立即从 heap 和 tasks map 清理 |
| Reschedule | `func (s *Scheduler) Reschedule(id string, newExecuteAt time.Time) error` | 重排任务到新的绝对执行时间，堆位置通过 Fix 自动调整 |
| RescheduleDelay | `func (s *Scheduler) RescheduleDelay(id string, newDelay time.Duration) error` | 重排任务到 `now+newDelay`，相当于 `Reschedule(id, time.Now().Add(newDelay))` |
| GetTask | `func (s *Scheduler) GetTask(id string) (*Task, error)` | 获取任务当前的信息拷贝 |
| TaskCount | `func (s *Scheduler) TaskCount() int` | 返回当前处于 Pending 或 Running 状态的任务数量（不含已取消/已完成） |

### 5.5 Cron 表达式格式与支持范围

支持标准 **5 段式 Unix Cron 表达式**（不支持第 6 段秒级扩展）：

```
┌───────────── 字段 1：分钟 (0 - 59)
│ ┌───────────── 字段 2：小时 (0 - 23)
│ │ ┌───────────── 字段 3：日 (1 - 31)
│ │ │ ┌───────────── 字段 4：月 (1 - 12)
│ │ │ │ ┌───────────── 字段 5：星期 (0 - 6；0=周日，1=周一，…，6=周六)
│ │ │ │ │
│ │ │ │ │
M H D m W
```

#### 支持的语法

| 语法形式 | 作用域 | 说明 | 示例 |
|----------|--------|------|------|
| `*` | 全部字段 | 匹配该字段的所有合法值 | `* * * * *`（每分钟） |
| 数字精确值 | 全部字段 | 匹配单一值 | `30 9 * * *`（每天 9:30） |
| `,` 逗号列表 | 全部字段 | 匹配多个离散值 | `0,30 * * * *`（每小时第 0、30 分钟） |
| `-` 范围 | 全部字段 | 匹配闭区间内的所有值 | `0 9-17 * * 1-5`（工作日 9-17 点整点） |
| `*/n` 步长 | 全部字段 | 从最小值开始，每隔 n 匹配一次 | `*/15 * * * *`（每 15 分钟） |
| `start-end/step` | 全部字段 | 指定范围内按步长匹配 | `10-40/10 * * * *`（10-40 分钟之间每 10 分钟） |
| 列表与范围混合 | 全部字段 | 可在逗号列表中混用精确值和范围 | `0,15,30-45/5 * * * *` |

#### 各字段取值范围

| 字段 | 位置 | 最小值 | 最大值 | 备注 |
|------|------|--------|--------|------|
| 分钟 (Minute) | 第 1 段 | 0 | 59 | 越界（含 60）直接判无效 |
| 小时 (Hour) | 第 2 段 | 0 | 23 | 0 表示午夜 12 点；越界（≥24）无效 |
| 日 (Day of Month) | 第 3 段 | 1 | 31 | 不校验月份实际天数（如 2 月 31 日仍视为合法表达式，匹配时永远失败） |
| 月 (Month) | 第 4 段 | 1 | 12 | 1 = 一月，12 = 十二月；≥13 无效 |
| 星期 (Day of Week) | 第 5 段 | 0 | 6 | 0 = 周日，1 = 周一 … 6 = 周六；≥7 无效 |

#### 不支持的 Cron 特性

以下特性当前**未实现**，使用时会报 `ErrInvalidCronExpr` 或不生效：

- ❌ 秒级字段（第 6 段）
- ❌ 年字段（第 7 段）
- ❌ 英文缩写：`JAN`/`FEB`/`MON`/`TUE` 等，必须用数字
- ❌ `@yearly`/`@monthly`/`@weekly`/`@daily`/`@hourly` 等宏定义
- ❌ 日和星期字段的"或"语义（Linux Vixie Cron 的特殊约定），当前为与语义
- ❌ `L`、`W`、`#` 等特殊字符（日/星期字段的高级语法）
- ❌ 时区感知：所有 Cron 计算均基于 Go 运行时的本地时区（`time.Now()` 的 Location）

#### 关键说明

- **最小粒度为分钟**：所有匹配时间的秒和纳秒均被截断为 0（对齐到整分钟边界）
- **首次执行时间**：`AddCron(delay, cronExpr, fn)` 的首次 ExecuteAt 由 `nextCronTime(cronExpr, time.Now().Add(delay))` 计算得出，**不是** `time.Now().Add(delay)`。若 `delay=0`，则从当前时刻起寻找下一个满足 Cron 表达式的分钟整点
- **后续执行时间**：每次执行完毕，以 `time.Now()` 为基准计算 `nextCronTime`，保证周期与真实时钟对齐，不受任务执行时长抖动影响
- **长时间未匹配**：`nextCronTime` 最多向前扫描 366 天，若仍无匹配视为表达式无效（理论上仅当日期/月份组合极端矛盾时触发）

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
// Pending 状态取消，立即从 map 清理，防止内存泄漏
s.Cancel("heartbeat")
```

### 6.3 Cron 表达式任务

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

// 每天 9:00 和 18:00 执行；delay=0，首次即今天 18:00（若已过则明天 9:00）
cronExpr := "0 9,18 * * *"
_, err := s.AddCron(0, cronExpr, func(ctx context.Context) {
    fmt.Println("打卡时间到！")
})
if err != nil {
    panic(err) // Cron 表达式无效
}

// 工作日（周一至周五）9:30 执行
s.AddCronWithID("workday-checkin", 0, "30 9 * * 1-5", func(ctx context.Context) {
    fmt.Println("工作日早会")
})

// 每 15 分钟执行一次（delay=0，首次下一个 15 分整点）
s.AddCronWithID("frequent", 0, "*/15 * * * *", func(ctx context.Context) {
    fmt.Println("每15分钟检查")
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

// 提前到 2 秒后执行（RescheduleDelay 内部调用 heap.Fix，O(log n)）
s.RescheduleDelay("important", 2*time.Second)

// 或者取消任务：Pending 状态立即从 heap 和 map 双重移除，无内存泄漏
// s.Cancel("important")

time.Sleep(3 * time.Second)
```

### 6.5 利用异步执行模型处理长耗时任务

调度循环不会被任务阻塞，可以放心执行长耗时任务：

```go
s := delaysched.NewScheduler()
s.Start()
defer s.Stop()

// 这个任务需要 5 秒，但不会阻塞其他任务的调度
s.AddWithID("long-job", 1*time.Second, func(ctx context.Context) {
    time.Sleep(5 * time.Second)
    fmt.Println("长任务完成")
})

// 这个任务在 2 秒后执行，不会被上面的长任务阻塞
s.AddWithID("short-job", 2*time.Second, func(ctx context.Context) {
    fmt.Println("短任务完成，时间:", time.Now().Format("15:04:05"))
})

time.Sleep(8 * time.Second)
```

## 7. 测试覆盖

单元测试位于 `internal/delaysched/scheduler_test.go`，当前共 **40** 个测试用例，覆盖核心功能、Cron 语义、内存泄漏等维度。

### 7.1 最小堆单元测试

- `TestMinHeap_PushPop`：Push 乱序任务、验证 Pop 严格按 ExecuteAt 升序
- `TestMinHeap_SameExecuteTime`：相同执行时间的两个任务均可被 Pop（验证不丢任务）
- `TestMinHeap_Remove`：中间位置 Remove，验证剩余顺序正确
- `TestMinHeap_Fix`：修改元素的 ExecuteAt 后 heap.Fix，验证堆性质保持

### 7.2 调度器基础功能测试

- `TestNewScheduler`：创建实例，TaskCount 为 0
- `TestScheduler_AddAndExecute`：3 个不同延迟的任务，验证全部执行
- `TestScheduler_Add_AutoID`：自动生成 ID，非空且互不重复
- `TestScheduler_AddAt`：指定精确执行时间，验证在该时刻附近触发
- `TestScheduler_Add_DuplicateID`：ID 重复时返回 ErrTaskAlreadyExists
- `TestScheduler_StartStop`：Start/Stop 幂等性，无死锁
- `TestScheduler_Add_Stopped`：未启动调度器添加任务返回 ErrSchedulerStopped
- `TestScheduler_StopBeforeExecute`：Stop 后未到期任务不被执行
- `TestScheduler_MultipleOrder`：4 个乱序任务，验证执行顺序严格按延迟升序
- `TestScheduler_TaskPanic`：任务 panic 不影响后续执行，重复任务自动恢复
- `TestScheduler_EmptyHeap_Stop`：堆空情况下 Stop 无死锁
- `TestScheduler_Add_Concurrent`：50 组并发 Add+AddWithID，全部执行
- `TestScheduler_Reschedule_Concurrent`：20 协程并发 Reschedule，无竞态
- `TestScheduler_NegativeDelay`：负延迟任务立即执行

### 7.3 Cancel 测试（含清理语义）

- `TestScheduler_Cancel`：Pending 取消，任务实际未执行，TaskCount 归零
- `TestScheduler_Cancel_NotFound`：对不存在 ID 返回 ErrTaskNotFound
- `TestScheduler_Cancel_AlreadyCancelled_ReturnsNotFound`：Pending 取消后再次 Cancel，验证 map 已被清理（返回 ErrTaskNotFound 而非 nil，证明无残留）
- `TestScheduler_Cancel_WhileRunning`：执行中一次性任务 Cancel 返回 ErrTaskRunning；执行完毕后再次 Cancel 返回 ErrTaskNotFound，验证 map 已清理
- `TestScheduler_AddInterval_CancelStopsRepeat`：取消间隔任务后执行次数不再增加

### 7.4 Reschedule 测试

- `TestScheduler_Reschedule`：重排到更早时间，提前执行
- `TestScheduler_RescheduleDelay`：延迟形式重排
- `TestScheduler_Reschedule_NotFound`：不存在的 ID 返回 ErrTaskNotFound
- `TestScheduler_Reschedule_LaterTime`：重排到更晚时间，验证在中间时间点未提前执行
- `TestScheduler_Reschedule_WhileRunning`：执行中重排返回 ErrTaskRunning

### 7.5 Interval 周期性任务测试

- `TestScheduler_AddInterval`：首次延迟 + 固定间隔，至少执行 3 次
- `TestScheduler_AddIntervalWithID_Duplicate`：指定 ID 重复返回错误

### 7.6 Cron 语义验证测试

- `TestCron_Validate`（子测试 18 个）：覆盖全部字段的合法/非法范围、`*`、`,`、`-`、`/`、`-/-`、非法字符、步数为 0、逆序范围
- `TestCron_NextTime_Basic`（子测试 4 个）：精确时间、整点、跨天、24 小时后同一时刻
- `TestCron_NextTime_StepAndRange`：`*/15` 步长、`9-17` 小时范围
- `TestScheduler_Cron_FirstExecuteByCronSemantics`：**核心 Cron 语义测试**，使用具体分钟（now+5min）构造 Cron，验证：
  - 首次 ExecuteAt 的 Minute 精确等于指定分钟（而非 delay 后的立即时间）
  - 首次 ExecuteAt 在 (now+4min50s, now+65min) 合理区间
  - RepeatType == RepeatCron、CronExpr 正确存储
- `TestScheduler_Cron_NextExecuteSemantics`：**核心调度一致性测试**
  - 验证首次 ExecuteAt 对齐到分钟 0 秒
  - RescheduleDelay 到近未来触发第一次执行
  - 验证执行完毕后第二次 ExecuteAt 为 `(firstExecuteAt + 1min).Truncate(minute)`（Cron 对齐）
  - 验证第二次 ExecuteAt 同样秒数为 0
- `TestScheduler_AddCron_InvalidExpr`：无效表达式（语法错误、小时=25）被拒绝
- `TestScheduler_AddCronWithID_Duplicate`：重复 ID 返回错误

### 7.7 内存泄漏专项测试

- `TestScheduler_CancelPending_NoMemoryLeak`：1000 轮，每轮 AddWithID 100 个任务 + Cancel 100 个任务；验证：
  - 全部迭代后 TaskCount = 0
  - 重新添加 1 个任务后 TaskCount = 1（侧面验证 tasks map 无陈旧条目干扰）
  - 取消该任务后 TaskCount 再次回到 0
- `TestScheduler_CompletedOneTime_NoMemoryLeak`：100 个一次性短延迟任务，执行完毕后 TaskCount = 0
- `TestScheduler_RepeatCancel_NoMemoryLeak`：50 个间隔任务至少执行 1 次后 Cancel（允许部分 Running），等待 300ms 后验证 TaskCount = 0（同时验证 Running+Cancelled 的延迟清理路径）

## 8. 设计权衡与注意事项

1. **异步执行模型**：任务在独立 goroutine 中运行，调度循环永不阻塞，代价是每个运行中任务占用一个 goroutine。对于高吞吐短任务场景，可考虑 worker pool 模式的未来扩展。

2. **Cron 精度对齐到分钟**：所有 Cron 计算均使用 `.Truncate(time.Minute)`，避免 `time.Now()` 的亚毫秒抖动导致匹配失败。首次执行和后续执行的 ExecuteAt 始终是"分钟整点"。

3. **Cancel 语义的非幂等性**：Pending 任务被 Cancel 后从 tasks map 删除，因此第二次 Cancel 返回 ErrTaskNotFound 而非 nil。这是内存安全设计的副作用，使用方应将 `Cancel` 视为"尝试取消"，而不是严格幂等操作。

4. **heap 惰性清理**：Cancel Pending 任务时主动从堆中 Remove，但如果任务已被 Pop 出堆（极端竞态），堆的惰性清理（runLoop 每次迭代扫描堆顶）会兜底移除无效条目，确保正确性。

5. **taskWg 的作用**：Stop 时必须等待所有运行中任务结束，否则可能出现调度器已销毁但任务仍引用其 ctx 的问题。对 Stop 的延迟敏感的场景，可以在 Stop 前先 Cancel 所有周期性任务。
