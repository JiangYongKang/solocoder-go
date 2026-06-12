# JobQueue 模块需求文档

## 1. 模块概述

JobQueue 是一个基于内存的任务队列模块，提供任务的异步调度与执行能力。它支持优先级排序、延迟执行、协程池并发控制、失败自动重试以及死信队列等高级特性，适用于需要异步解耦、流量削峰、后台任务处理等场景。

### 主要特性

- **任务入队与出队**：支持提交任务到队列，消费者自动取出执行并返回结果
- **优先级队列**：高优先级任务优先被消费，同优先级按入队时间先入先出
- **延迟执行**：支持提交延迟任务，等待指定时间后才可被执行
- **协程池限制**：通过可配置的协程池限制并发执行数量，池满时出队请求阻塞等待
- **失败重试**：任务执行失败时按指数退避策略自动重试，超过最大重试次数进入死信队列
- **结果通知**：支持同步等待任务执行结果，支持超时控制

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    PoolSize        int           // 协程池大小，必须 > 0
    DefaultMaxRetry int           // 默认最大重试次数，< 0 时设为 0
    ShutdownTimeout time.Duration // 优雅关闭超时时间，<= 0 时默认 30s
}
```

**职责**：定义 JobQueue 的配置参数，在创建队列实例时传入。

### 2.2 Job

```go
type Job struct {
    ID          string
    Priority    int
    Payload     interface{}
    Delay       time.Duration
    EnqueueTime time.Time
    ReadyTime   time.Time
    RetryCount  int
    MaxRetries  int
    Status      JobStatus
    Result      interface{}
    Error       error
}
```

**职责**：表示一个任务单元，包含任务的全部元数据与执行状态。

| 字段 | 说明 |
|------|------|
| ID | 任务唯一标识，为空时自动生成 |
| Priority | 任务优先级，数值越大优先级越高 |
| Payload | 任务负载数据，由 Handler 处理 |
| Delay | 首次延迟执行时间 |
| EnqueueTime | 最近一次入队时间 |
| ReadyTime | 任务就绪时间（延迟到期或重试到期） |
| RetryCount | 已重试次数 |
| MaxRetries | 最大重试次数 |
| Status | 当前任务状态 |
| Result | 执行结果（成功时） |
| Error | 执行错误（失败时） |

### 2.3 JobStatus

任务状态枚举，各状态语义严格区分：

| 状态 | 说明 |
|------|------|
| `JobStatusPending` | **首次等待执行**：新任务入队后尚未被执行过，在队列中等待调度 |
| `JobStatusRunning` | 正在执行中：已被 worker 协程取出，正在调用 Handler |
| `JobStatusCompleted` | **执行成功**：Handler 返回 nil error，任务正常结束 |
| `JobStatusFailed` | **执行失败，等待重试**：Handler 返回 error，但未超过最大重试次数，在延迟队列中等待重试 |
| `JobStatusDeadLetter` | **执行失败，已放弃**：超过最大重试次数，移入死信队列，不再重试 |

> **关键区分**：`Pending` 表示从未执行过的新任务在等待，`Failed` 表示执行失败后在等待重试。通过 `GetJobStatus()` 可明确区分这两种不同语义的等待状态。

### 2.4 JobResult

```go
type JobResult struct {
    JobID  string
    Result interface{}
    Error  error
}
```

**职责**：封装任务执行结果，用于对外返回。

### 2.5 JobQueue

```go
type JobQueue struct {
    // ... 内部字段省略
}
```

**职责**：任务队列核心管理器，负责：
- 维护优先级队列与延迟队列
- 调度任务到协程池执行
- 处理任务重试与死信转移
- 管理任务状态与结果存储
- 提供结果查询与通知机制

## 3. 任务生命周期

任务从创建到结束的完整状态流转（修复后的状态机）：

```
                         ┌──────────────┐
                         │  Enqueue()   │
                         └──────┬───────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │   JobStatusPending   │
                    │  (首次等待执行)      │
                    └──────────┬───────────┘
                               │
           ┌───────────────────┼────────────────────┐
           │                   │                    │
     delay > 0           delay = 0                  │
           │                   │                    │
           ▼                   ▼                    │
    ┌─────────────┐    ┌──────────────┐             │
    │  DelayQueue │    │ PriorityQueue│             │
    └──────┬──────┘    └──────┬───────┘             │
           │                  │                     │
           │  time passes     │  dispatchLoop       │
           └─────────────────>│  picks job          │
                              ▼                     │
                    ┌──────────────────┐            │
                    │ acquire sem slot │            │
                    └────────┬─────────┘            │
                             │                      │
                             ▼                      │
                    ┌──────────────────┐            │
                    │ JobStatusRunning │            │
                    └────────┬─────────┘            │
                             │                      │
                 ┌───────────┴───────────┐          │
                 │                       │          │
                 ▼                       ▼          │
       ┌──────────────────┐   ┌──────────────────┐  │
       │  Handler 成功    │   │   Handler 失败    │  │
       └────────┬─────────┘   └────────┬─────────┘  │
                │                      │            │
                │                      ▼            │
                │             RetryCount++          │
                │                      │            │
                │          ┌───────────┴───────┐    │
                │          │                   │    │
                │          ▼                   ▼    │
                │  RetryCount > MaxRetries   否则   │
                │          │                   │    │
                ▼          ▼                   │    │
    ┌────────────────────┐  ┌──────────────────────┐ │
    │ JobStatusCompleted │  │  JobStatusFailed     │─┘
    │(存入 successResults)│ │(等待重试，存入延迟队列)│
    └────────────────────┘  └──────────────────────┘
                                          │
                                          ▼
                    （重试到期后重新进入 Running）
                                          │
                                          ▼
                    （重试成功 → Completed，重试耗尽 → DeadLetter）
                                          │
                                          ▼
                          ┌──────────────────────┐
                          │ JobStatusDeadLetter  │
                          │(存入 deadLetterResults)│
                          └──────────────────────┘
```

### 生命周期阶段说明

1. **入队阶段**：通过 `Enqueue()` 或 `EnqueueWithRetry()` 提交任务
   - 新任务初始状态为 `JobStatusPending`
   - 指定 `Delay > 0` → 进入延迟队列（DelayQueue）
   - 指定 `Delay = 0` → 直接进入优先级队列（PriorityQueue）

2. **等待阶段**：
   - 延迟队列中的任务等待 `ReadyTime` 到达后转入优先级队列
   - 优先级队列中的任务按优先级+入队时间排序，等待调度
   - **两类等待状态的语义区分（通过 `GetJobStatus()` 查询）**：
     - `JobStatusPending`：**首次等待执行**。状态为 Pending 的任务从未被 Handler 执行过，是刚入队的全新任务，`RetryCount == 0`，`Error == nil`。
     - `JobStatusFailed`：**执行失败后等待重试**。状态为 Failed 的任务已经被 Handler 执行过至少一次且返回了 error，`RetryCount >= 1`，`Error` 字段保存上一次失败的原因，正处于指数退避的延迟等待期，等待下一次重试调度。
   - 两类状态的任务均可能位于优先级队列（ReadyTime 已到）或延迟队列（等待 ReadyTime）中，不依赖所在堆类型区分，仅通过 `Status` 字段判断。
   - **使用示例**：
     ```go
     status, _ := jq.GetJobStatus(jobID)
     if status == jobqueue.JobStatusPending {
         fmt.Println("任务尚未开始执行，首次等待中")
     } else if status == jobqueue.JobStatusFailed {
         fmt.Println("任务执行过但失败了，正在等待重试")
     }
     ```

3. **调度阶段**：
   - `dispatchLoop` 协程循环检查队列
   - 从优先级队列取出队首任务（状态可能为 Pending 或 Failed）
   - 尝试获取协程池信号量（sem），获取失败则阻塞等待

4. **执行阶段**：
   - 获取信号量后启动 worker 协程执行任务
   - 任务状态变更为 `JobStatusRunning`
   - 调用用户注册的 `JobHandler` 执行任务逻辑

5. **结束阶段**：
   - **成功**：状态 → `JobStatusCompleted`，结果存入 `successResults`
   - **失败**：`RetryCount++`
     - 仍可重试（`RetryCount <= MaxRetries`）：状态 → `JobStatusFailed`，计算指数退避延迟 → 重新进入延迟队列等待重试
     - 超过最大重试（`RetryCount > MaxRetries`）：状态 → `JobStatusDeadLetter`，移入死信队列，结果存入 `deadLetterResults`

### 3.1 状态机流转规则总结

| 当前状态 | 触发事件 | 下一状态 | RetryCount 变化 | Error 字段 |
|----------|----------|----------|-----------------|------------|
| （新建任务） | `Enqueue()` / `EnqueueWithRetry()` | `Pending` | 赋值为 0 | 清空为 nil |
| `Pending` | 被调度取出进入协程池 | `Running` | 不变 | 不变（保持 nil） |
| `Failed` | 重试延迟到期，被调度取出 | `Running` | 不变 | 保留上一次错误值 |
| `Running` | Handler 返回 `nil error` | `Completed` | 不变 | 清空为 nil |
| `Running` | Handler 返回 error，且 `RetryCount+1 <= MaxRetries` | `Failed` | 加 1 | 设置为本次错误 |
| `Running` | Handler 返回 error，且 `RetryCount+1 > MaxRetries` | `DeadLetter` | 加 1 | 设置为本次错误 |

**状态迁移关键约束**：
- `Pending` 只能来自新建任务入队，只能转为 `Running`
- `Failed` 只能来自 `Running` 执行失败（未超过重试上限），不能从 Pending 直接跳转
- 等待阶段是 Pending 还是 Failed，与任务在「优先级队列」还是「延迟队列」中无关，仅由任务的执行历史决定

## 4. 核心算法与策略

### 4.1 优先级排序策略

使用 `container/heap` 实现最小堆（逻辑上最大堆）：

```go
func (pq priorityQueue) Less(i, j int) bool {
    // 1. 优先按 Priority 降序（数值越大越先出队）
    if pq[i].Priority != pq[j].Priority {
        return pq[i].Priority > pq[j].Priority
    }
    // 2. 同优先级按 EnqueueTime 升序（先入先出 FIFO）
    return pq[i].EnqueueTime.Before(pq[j].EnqueueTime)
}
```

### 4.2 延迟队列策略

同样使用堆结构，按 `ReadyTime` 升序排列：

```go
func (dq delayQueue) Less(i, j int) bool {
    return dq[i].ReadyTime.Before(dq[j].ReadyTime)
}
```

`dispatchLoop` 在每次迭代时将已到期（`IsReady() == true`）的延迟任务转移到优先级队列。

### 4.3 指数退避重试策略

失败重试间隔采用指数递增方式：

```go
func (j *Job) BackoffDelay() time.Duration {
    base := 100 * time.Millisecond
    return base * time.Duration(1<<uint(j.RetryCount))
}
```

| 重试次数 (RetryCount) | 延迟时间 |
|------------------------|----------|
| 0 (首次失败)           | 100ms    |
| 1                      | 200ms    |
| 2                      | 400ms    |
| 3                      | 800ms    |
| 4                      | 1.6s     |
| ...                    | ...      |

### 4.4 协程池并发控制

使用带缓冲 channel 作为信号量（Semaphore）：

```go
sem := make(chan struct{}, PoolSize)

// 获取槽位（阻塞等待）
sem <- struct{}{}

// 释放槽位
<-sem
```

- 每个 worker 协程启动前必须获取信号量槽位
- 槽位用尽时新任务阻塞等待，直到有 worker 完成并释放槽位
- 优雅关闭期间（running=false）忽略信号量限制，确保剩余任务被处理

### 4.5 Panic 恢复

任务执行通过 `safeExecute()` 包裹，自动捕获 panic 并转化为错误返回：

```go
func (jq *JobQueue) safeExecute(job *Job) (result interface{}, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("job panicked: %v", r)
            result = nil
        }
    }()
    // ... 执行 handler
}
```

## 5. API 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"
    "solocoder-go/internal/jobqueue"
)

func main() {
    // 1. 创建队列：协程池大小为 5，默认重试 3 次
    jq, err := jobqueue.NewJobQueue(jobqueue.Config{
        PoolSize:        5,
        DefaultMaxRetry: 3,
        ShutdownTimeout: 10 * time.Second,
    })
    if err != nil {
        panic(err)
    }
    defer jq.Stop()

    // 2. 设置任务处理函数
    jq.SetHandler(func(ctx context.Context, job *jobqueue.Job) (interface{}, error) {
        payload := job.Payload.(string)
        return fmt.Sprintf("processed: %s", payload), nil
    })

    // 3. 启动队列
    if err := jq.Start(); err != nil {
        panic(err)
    }

    // 4. 提交任务
    jobID, _ := jq.Enqueue("task-001", 5, "hello world", 0)

    // 5. 等待结果（带超时）
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    result, err := jq.WaitForResult(ctx, jobID)
    if err != nil {
        fmt.Printf("等待失败: %v\n", err)
        return
    }
    if result.Error != nil {
        fmt.Printf("任务执行失败: %v\n", result.Error)
    } else {
        fmt.Printf("任务结果: %v\n", result.Result)
    }
}
```

### 5.2 优先级与延迟任务

```go
// 高优先级任务（立即执行）
jq.Enqueue("urgent-task", 100, urgentData, 0)

// 普通优先级任务
jq.Enqueue("normal-task", 5, normalData, 0)

// 低优先级延迟任务（5 分钟后执行）
jq.Enqueue("batch-report", 1, reportConfig, 5*time.Minute)
```

### 5.3 自定义重试次数

```go
// 最多重试 5 次（默认 3 次）
jq.EnqueueWithRetry("unreliable-api", 10, reqData, 0, 5)

// 不重试（失败直接入死信）
jq.EnqueueWithRetry("no-retry-job", 1, data, 0, 0)
```

### 5.4 监控与统计

#### 5.4.1 结果存储机制

队列内部将结果分开存储为两个独立的 map，确保成功与失败（死信）语义互斥且可独立统计：
- `successResults`：仅存储**成功完成**的任务结果（状态为 `JobStatusCompleted`）
  - 进入条件：Handler 返回 `nil error`
  - `JobResult.Error == nil`，`JobResult.Result` 保存 Handler 返回值
- `deadLetterResults`：仅存储**失败且已放弃**的任务结果（状态为 `JobStatusDeadLetter`）
  - 进入条件：Handler 返回 error，且 `RetryCount > MaxRetries`
  - `JobResult.Error` 保存最后一次失败原因，`JobResult.Result == nil`

#### 5.4.2 统计方法语义定义

| 方法 | 返回值类型 | 精确语义 | 计数来源 |
|------|-----------|----------|----------|
| `PendingCount()` | int | 所有**尚未开始执行**的任务总数，包括首次等待和等待重试的 | 优先级队列长度 + 延迟队列长度 + 等待调度槽位的任务数 |
| `ActiveCount()` | int | **正在执行中**的任务数（状态为 Running 的 worker 协程数） | 原子计数器 `activeCount` |
| `CompletedCount()` | int | **成功完成**的任务总数（仅包含 Handler 返回 nil error 的） | `len(successResults)` |
| `FailedCount()` | int | **失败且已放弃**的任务总数（仅包含已进入死信队列的） | `len(deadLetterResults)` |
| `DeadLetterCount()` | int | 死信队列中保存的任务结构体数（与 FailedCount 同步增长） | `len(deadLetters)` |
| `TaskCount()` (Total) | `Pending + Active + Completed + Failed` | 系统中所有生命周期阶段的任务总数 | 各项求和 |

**关键恒等式**：
```
CompletedCount() + FailedCount() = 总已结束任务数
```
即：所有通过 `Enqueue()` / `EnqueueWithRetry()` 入队且不再处于等待或执行中状态的任务，要么计入 Completed（成功），要么计入 Failed（死信），两者之和等于已结束任务总数，不会重复计数也不会遗漏。

#### 5.4.3 使用示例

```go
// 获取各阶段任务数量
pending := jq.PendingCount()      // 等待执行的任务数（Pending + Failed 状态，在队列中）
active := jq.ActiveCount()        // 正在执行的任务数（Running 状态）
completed := jq.CompletedCount()  // 仅成功完成的任务数（存入 successResults）
failed := jq.FailedCount()        // 仅进入死信队列的任务数（存入 deadLetterResults）
deadLetters := jq.DeadLetterCount() // 死信队列中的任务数

fmt.Printf("统计: pending=%d, active=%d, completed=%d, failed=%d, dead=%d\n",
    pending, active, completed, failed, deadLetters)

// 验证总已结束任务数
totalEnqueued := 1000  // 假设总共入队了 1000 个任务
totalFinished := jq.CompletedCount() + jq.FailedCount()
totalInFlight := pending + active
if totalFinished + totalInFlight != totalEnqueued {
    fmt.Println("警告：任务数不匹配，可能有任务状态异常")
}

// 死信队列告警示例
if jq.FailedCount() > 10 {
    fmt.Printf("告警：已有 %d 个任务进入死信队列，请检查处理\n", jq.FailedCount())

    // 获取死信任务详情
    dls := jq.GetDeadLetters()
    for _, job := range dls {
        fmt.Printf("死信任务: %s, 重试次数: %d, 错误: %v\n",
            job.ID, job.RetryCount, job.Error)
    }
    jq.ClearDeadLetters()  // 处理完毕后清空死信队列
}

// 查询任务结果（同时查找 successResults 和 deadLetterResults）
result, err := jq.GetResult("job-123")
if err == nil {
    if result.Error != nil {
        fmt.Printf("任务最终失败（已入死信）: %v\n", result.Error)
    } else {
        fmt.Printf("任务成功: %v\n", result.Result)
    }
}

// 通过 GetJobStatus 区分两类等待（监控示例）
status, err := jq.GetJobStatus("job-456")
if err == nil {
    switch status {
    case jobqueue.JobStatusPending:
        fmt.Println("任务首次等待中，尚未执行")
    case jobqueue.JobStatusFailed:
        fmt.Println("任务曾失败，正在等待重试")
    case jobqueue.JobStatusRunning:
        fmt.Println("任务正在执行")
    case jobqueue.JobStatusCompleted:
        fmt.Println("任务已成功完成")
    case jobqueue.JobStatusDeadLetter:
        fmt.Println("任务已进入死信，重试耗尽")
    }
}

// 清理历史结果（同时清空 successResults 和 deadLetterResults）
jq.ClearResults()
```

### 5.5 并发池限制场景

```go
// 创建仅有 2 个并发槽位的队列
jq, _ := jobqueue.NewJobQueue(jobqueue.Config{PoolSize: 2})
jq.SetHandler(func(ctx context.Context, job *jobqueue.Job) (interface{}, error) {
    time.Sleep(100 * time.Millisecond)  // 模拟耗时操作
    return nil, nil
})
jq.Start()

// 提交 10 个任务：前 2 个立即执行，其余 8 个排队等待
for i := 0; i < 10; i++ {
    jq.Enqueue(fmt.Sprintf("job-%d", i), 1, i, 0)
}

// 任意时刻最多只有 2 个任务处于 Running 状态
fmt.Printf("最大并发被限制在 %d\n", jq.ActiveCount())  // <= 2
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrPoolSizeInvalid` | 创建队列时 PoolSize <= 0 |
| `ErrHandlerNotSet` | 调用 Start() 前未通过 SetHandler 设置处理函数 |
| `ErrQueueStopped` | 队列未启动或已停止时尝试入队 |
| `ErrJobNotFound` | 查询不存在的 JobID 的状态或结果 |

上下文取消/超时：
- `WaitForResult(ctx, jobID)` 在 ctx 被取消或超时时返回 `ctx.Err()`

## 7. 线程安全说明

JobQueue 所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：
- 队列内部通过 `sync.Mutex`、`sync.RWMutex` 与原子操作保护共享数据
- `dispatchLoop` 单协程调度，避免竞争条件
- 结果存储使用读写锁分离，多读场景高效

## 8. 资源与生命周期

- **创建**：`NewJobQueue()` 创建实例
- **启动**：`Start()` 启动调度循环，必须先设置 Handler
- **运行**：入队任务、等待结果、查询状态
- **停止**：`Stop()` 触发优雅关闭
  - 停止接收新任务
  - 等待已出队任务完成（不受 PoolSize 限制）
  - 等待所有正在执行的任务完成
  - 超过 `ShutdownTimeout` 时强制返回
- **多次调用**：`Start()`、`Stop()` 均为幂等操作
