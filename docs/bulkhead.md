# 舱壁隔离模块 (Bulkhead) 需求文档

## 1. 模块概述

舱壁隔离模块是微服务架构中的容错组件，借鉴了造船业中"水密舱壁"的设计思想——将系统资源划分为多个独立的隔离舱，每个舱室拥有独立的协程池和任务队列。当某个服务或资源出现故障导致请求积压时，故障被限制在对应舱室内，不会蔓延至整个系统，从而保障了整体系统的稳定性。

本模块提供了独立协程池分配、信号量限流、资源耗尽快速失败、动态扩缩容等核心能力，支持运行时调整资源配额，适用于多租户隔离、服务分级保障、关键资源保护等场景。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 独立协程池分配 | 为不同服务或资源组分配独立的协程池，每个池有独立的并发上限和队列容量，舱室之间互不影响 |
| F2 | 信号量限流 | 通过信号量机制控制每个隔离舱的并发执行数量，达到上限后新任务可排队等待或被拒绝 |
| F3 | 信号量接口 | 提供独立的 `Acquire(timeout)` 和 `Release()` 方法，调用方可在自己的 goroutine 中获取/释放并发槽位 |
| F4 | 共享并发配额 | worker 池模式和信号量模式共享同一个并发上限，总并发数不超过 `maxConcurrency` |
| F5 | 等待超时机制 | 支持配置任务等待超时时间，超时后返回资源耗尽错误，避免无限阻塞 |
| F6 | 快速失败 | 当协程池和等待队列均满且不等待时，新任务立即返回携带诊断信息的 FullError |
| F7 | 动态扩缩容 | 运行时动态调整并发数和队列容量，扩容立即可用，缩容不影响已提交任务执行 |
| F8 | 渐进式缩容 | 缩容时空闲协程逐步回收，不强制杀死正在执行的协程，保证任务完整性 |
| F9 | 多隔离舱管理 | 通过 Registry 统一管理多个命名隔离舱，支持创建、查询、删除操作 |
| F10 | 状态查询 | 提供当前并发数、等待队列长度、信号量持有数、最大并发数、最大队列容量等查询接口 |
| F11 | 优雅关闭 | 关闭隔离舱时等待所有已提交任务执行完毕后再退出，确保任务不丢失 |

## 3. 核心结构体与职责

### 3.1 Config - 隔离舱配置

```go
type Config struct {
    MaxConcurrency int           // 最大并发数（协程池大小）
    MaxQueueSize   int           // 最大等待队列容量
    WaitTimeout    time.Duration // 任务等待超时时间，0 表示不等待立即失败
}
```

**配置约束：**
- `MaxConcurrency` 必须大于 0，否则返回 `ErrInvalidConcurrency`
- `MaxQueueSize` 必须大于等于 0，否则返回 `ErrInvalidQueueSize`
- `WaitTimeout` 为 0 时表示非阻塞模式，队列满时立即返回错误

### 3.2 Bulkhead - 隔离舱主体

```go
type Bulkhead struct {
    name           string        // 隔离舱名称
    maxConcurrency int           // 最大并发数
    maxQueueSize   int           // 最大队列容量
    waitTimeout    time.Duration // 等待超时时间

    mu             sync.Mutex    // 互斥锁，保护内部状态
    cond           *sync.Cond    // 条件变量，用于任务提交与 worker 的同步
    taskQueue      []Task        // 任务队列（切片实现，支持动态调整容量）
    active         int           // 当前正在执行的任务数
    idleWorkers    int           // 当前空闲的 worker 数
    closed         bool          // 是否已关闭
    workerCnt      int           // 当前 worker 总数
    shrinkCnt      int           // 待缩容的 worker 数
    wg             sync.WaitGroup // worker 同步等待组
}
```

**主要职责：**
- 管理 worker 协程池的生命周期
- 维护任务队列，协调任务提交与执行
- 实现并发控制与限流逻辑
- 支持动态调整资源配额
- 提供状态查询与优雅关闭能力

### 3.3 FullError - 资源耗尽错误

```go
type FullError struct {
    Name           string // 隔离舱名称
    ActiveCount    int    // 当前并发数
    MaxConcurrency int    // 最大并发数
    QueueLength    int    // 当前队列长度
    MaxQueueSize   int    // 最大队列容量
}
```

**主要职责：**
- 携带丰富的诊断信息，便于问题排查
- 实现 `error` 接口，格式化输出当前资源使用状态

### 3.4 Task - 任务类型

```go
type Task func()
```

任务函数类型，无参数无返回值。调用方通过闭包捕获所需上下文。

### 3.5 Registry - 隔离舱注册中心

```go
type Registry struct {
    mu        sync.RWMutex       // 读写锁
    bulkheads map[string]*Bulkhead // 命名隔离舱映射
}
```

**主要职责：**
- 统一管理多个命名隔离舱实例
- 提供创建、查询、删除、批量关闭等操作
- 防止同名隔离舱重复创建

### 3.6 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrBulkheadClosed` | 隔离舱已关闭 | 已关闭的隔离舱上调用 Submit/TrySubmit/Resize |
| `ErrBulkheadFull` | 隔离舱已满 | 资源耗尽且不等待时返回（实际返回 `*FullError`） |
| `ErrBulkheadTimeout` | 等待超时 | 任务等待超时时返回（实际返回 `*FullError`） |
| `ErrInvalidConcurrency` | 并发数无效 | MaxConcurrency <= 0 |
| `ErrInvalidQueueSize` | 队列大小无效 | MaxQueueSize < 0 |
| `ErrInvalidName` | 名称无效 | 名称为空字符串 |

## 4. 舱壁隔离与信号量限流的协作方式

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                      Registry                           │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐ │
│  │  Bulkhead A  │  │  Bulkhead B  │  │  Bulkhead C  │ │
│  │  (用户服务)   │  │  (订单服务)   │  │  (支付服务)   │ │
│  └──────────────┘  └──────────────┘  └──────────────┘ │
└─────────────────────────────────────────────────────────┘

单个 Bulkhead 内部结构：
┌───────────────────────────────────────────┐
│              Bulkhead                      │
│  ┌─────────────────────────────────────┐  │
│  │  Task Queue (MaxQueueSize)          │  │
│  │  [任务1] [任务2] [任务3] ...       │  │
│  └─────────────────────────────────────┘  │
│                     ↓                      │
│  ┌─────┐ ┌─────┐       ┌─────┐          │
│  │Worker│ │Worker│ ... │Worker│          │
│  │  #1  │ │  #2  │       │  #N │          │
│  └─────┘ └─────┘       └─────┘          │
│    MaxConcurrency 个 worker 协程          │
└───────────────────────────────────────────┘
```

### 4.2 协作机制说明

**舱壁隔离**与**信号量限流**是两个互补的机制，共同保障系统稳定性：

1. **舱壁隔离（空间维度）**：通过 Registry 将系统划分为多个独立的 Bulkhead，每个 Bulkhead 拥有独立的资源配额（协程池 + 队列）。一个 Bulkhead 的任务积压或故障不会影响其他 Bulkhead 的正常运行。

2. **信号量限流（时间维度）**：在单个 Bulkhead 内部，通过固定数量的 worker 协程实现并发控制（信号量模式）。同时执行的任务数不超过 `MaxConcurrency`，超出的任务进入队列等待。

两者的协作关系：
- **舱壁隔离**是"粗粒度"的资源划分，解决"故障蔓延"问题
- **信号量限流**是"细粒度"的并发控制，解决"单个舱室内过载"问题
- 两者结合形成纵深防御体系，既防止故障跨舱扩散，又防止单个舱室自身过载

### 4.3 任务提交流程

```
Submit(task)
    │
    ├─ 参数校验（task == nil → 返回错误）
    │
    ├─ mu.Lock()
    │
    ├─ 检查 closed → 返回 ErrBulkheadClosed
    │
    ├─ 检查是否可提交（canSubmit）
    │     ├─ 队列未满（len(queue) < maxQueueSize）→ 可以
    │     └─ 有空闲 worker（idleWorkers > 0）→ 可以
    │
    ├─ 可提交：
    │     ├─ task 追加到队列尾部
    │     ├─ cond.Signal() 唤醒一个 worker
    │     ├─ mu.Unlock()
    │     └─ 返回 nil
    │
    └─ 不可提交：
          ├─ WaitTimeout <= 0 → 快速失败
          │     ├─ 构造 FullError
          │     ├─ mu.Unlock()
          │     └─ 返回 FullError
          │
          └─ WaitTimeout > 0 → 等待模式
                ├─ deadline = now + WaitTimeout
                ├─ 启动超时定时器：到时 Broadcast
                ├─ [循环：直到可提交/超时/关闭]
                │     ├─ 检查 closed → 返回 ErrBulkheadClosed
                │     ├─ 检查超时 → 返回 FullError
                │     └─ cond.Wait() 挂起等待
                ├─ 可提交后：
                │     ├─ task 入队，Signal 唤醒 worker
                │     └─ 返回 nil
                └─ mu.Unlock()
```

### 4.4 Worker 执行流程

```
worker()
    │
    ├─ mu.Lock()
    │
    ├─ [外层循环]
    │     │
    │     ├─ [内层循环：队列为空 && 未关闭 && 不需要缩容]
    │     │     ├─ idleWorkers++
    │     │     ├─ cond.Broadcast() （通知等待中的 Submit）
    │     │     └─ cond.Wait() 等待任务
    │     │     └─ idleWorkers--
    │     │
    │     ├─ 队列为空？
    │     │     ├─ closed == true → workerCnt--，return
    │     │     └─ shrinkCnt > 0 → shrinkCnt--，workerCnt--，return
    │     │
    │     ├─ 从队列头部取出任务
    │     ├─ active++
    │     ├─ cond.Broadcast() （通知等待队列空间的 Submit）
    │     ├─ mu.Unlock()
    │     │
    │     ├─ 执行任务函数
    │     │
    │     └─ mu.Lock()
    │           ├─ active--
    │           └─ cond.Broadcast() （通知等待空闲 worker 的 Submit）
    │
    └─ 回到外层循环
```

## 5. 动态扩缩容机制

### 5.1 并发数扩缩容

**扩容（增加并发数）：**
- 计算需要新增的 worker 数量
- 逐个启动新的 worker goroutine
- 新 worker 启动后立即可用，参与任务处理
- 扩容操作不影响正在执行的任务

**缩容（减少并发数）：**
- 计算需要减少的 worker 数量，累加到 `shrinkCnt`
- 调用 `cond.Broadcast()` 唤醒所有空闲 worker
- worker 在空闲时检查 `shrinkCnt`，若大于 0 则递减并退出
- 正在执行任务的 worker 不受影响，任务完成后才可能被回收
- 保证缩容过程平滑，不中断任务执行

### 5.2 队列容量调整

**扩容（增大队列）：**
- 直接更新 `maxQueueSize`
- 调用 `cond.Broadcast()` 唤醒等待中的 Submit
- 新容量立即可用，可接受更多排队任务

**缩容（减小队列）：**
- 直接更新 `maxQueueSize`
- **已在队列中的任务全部保留**，不丢弃已提交的任务
- 新任务提交时按新容量判断是否队列已满
- 保证已提交任务的完整性，遵循"缩容不影响已提交任务"原则

## 6. 资源耗尽快速失败机制

当隔离舱的协程池和等待队列均满时，新任务请求根据 `WaitTimeout` 配置决定行为：

### 6.1 快速失败模式（WaitTimeout = 0）

- 立即返回 `*FullError` 错误
- 错误中携带：隔离舱名称、当前并发数、最大并发数、队列长度、队列容量
- 调用方可根据诊断信息快速定位问题
- 避免线程/协程阻塞堆积，防止故障扩散

### 6.2 等待超时模式（WaitTimeout > 0）

- 任务进入等待状态，直到有资源可用或超时
- 超时后返回 `*FullError` 错误，携带诊断信息
- 保证调用方不会无限阻塞
- 适用于可接受短暂等待的场景

## 7. 使用示例

### 7.1 基础使用：单隔离舱

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/bulkhead"
)

func main() {
    cfg := bulkhead.Config{
        MaxConcurrency: 10,
        MaxQueueSize:   100,
        WaitTimeout:    5 * time.Second,
    }

    b, err := bulkhead.NewBulkhead("user-service", cfg)
    if err != nil {
        panic(err)
    }
    defer b.Close()

    err = b.Submit(func() {
        fmt.Println("Task executed")
    })
    if err != nil {
        if fe, ok := err.(*bulkhead.FullError); ok {
            fmt.Printf("Bulkhead full: active=%d/%d queue=%d/%d\n",
                fe.ActiveCount, fe.MaxConcurrency,
                fe.QueueLength, fe.MaxQueueSize)
        }
        return
    }

    fmt.Printf("Active: %d, Queue: %d\n", b.ActiveCount(), b.QueueLength())
}
```

### 7.2 非阻塞模式（快速失败）

```go
cfg := bulkhead.Config{
    MaxConcurrency: 5,
    MaxQueueSize:   10,
    WaitTimeout:    0, // 队列满时立即失败
}

b, _ := bulkhead.NewBulkhead("payment-service", cfg)
defer b.Close()

ok, err := b.TrySubmit(func() {
    // 处理支付请求
})
if err != nil {
    // 处理错误（如已关闭）
    return
}
if !ok {
    // 资源不足，执行降级逻辑
    fmt.Println("Payment service is busy, please retry later")
    return
}
```

### 7.3 多隔离舱管理（Registry）

```go
registry := bulkhead.NewRegistry()
defer registry.CloseAll()

userCfg := bulkhead.Config{MaxConcurrency: 20, MaxQueueSize: 200}
orderCfg := bulkhead.Config{MaxConcurrency: 10, MaxQueueSize: 50}

userBulkhead, _ := registry.NewBulkhead("user-service", userCfg)
orderBulkhead, _ := registry.NewBulkhead("order-service", orderCfg)

// 提交用户服务任务
userBulkhead.Submit(func() { /* 处理用户请求 */ })

// 提交订单服务任务
orderBulkhead.Submit(func() { /* 处理订单请求 */ })

// 查询所有隔离舱名称
names := registry.Names()
fmt.Printf("Registered bulkheads: %v\n", names)

// 移除某个隔离舱
registry.Remove("order-service")
```

### 7.4 动态扩缩容

```go
b, _ := bulkhead.NewBulkhead("api-gateway", bulkhead.Config{
    MaxConcurrency: 10,
    MaxQueueSize:   50,
})

// 高峰期扩容
err := b.Resize(50, 200)
if err != nil {
    fmt.Printf("Resize failed: %v\n", err)
}
fmt.Printf("After scale up: concurrency=%d, queue=%d\n",
    b.MaxConcurrency(), b.MaxQueueSize())

// 低峰期缩容
err = b.Resize(5, 20)
if err != nil {
    fmt.Printf("Resize failed: %v\n", err)
}
fmt.Printf("After scale down: concurrency=%d, queue=%d\n",
    b.MaxConcurrency(), b.MaxQueueSize())
```

### 7.5 监控与指标采集

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        active := b.ActiveCount()
        queue := b.QueueLength()
        maxConc := b.MaxConcurrency()
        maxQueue := b.MaxQueueSize()

        fmt.Printf("[%s] active=%d/%d queue=%d/%d utilization=%.1f%%\n",
            b.Name(),
            active, maxConc,
            queue, maxQueue,
            float64(active)/float64(maxConc)*100)
    }
}()
```

### 7.6 信号量使用（Acquire/Release）

#### 7.6.1 基础使用：在调用方 goroutine 中执行受保护代码

```go
cfg := bulkhead.Config{
    MaxConcurrency: 5,
    MaxQueueSize:   10,
}

b, err := bulkhead.NewBulkhead("db-operations", cfg)
if err != nil {
    panic(err)
}
defer b.Close()

// 获取信号量槽位，最多等待 3 秒
err = b.Acquire(3 * time.Second)
if err != nil {
    if fe, ok := err.(*bulkhead.FullError); ok {
        fmt.Printf("Database bulkhead full: active=%d/%d\n",
            fe.ActiveCount, fe.MaxConcurrency)
    }
    return
}
defer func() {
    if err := b.Release(); err != nil {
        fmt.Printf("Release failed: %v\n", err)
    }
}()

// 执行受保护的数据库操作（在当前 goroutine 中）
rows, err := db.Query("SELECT * FROM users WHERE id = ?", userID)
if err != nil {
    return
}
defer rows.Close()

// 处理查询结果...
```

#### 7.6.2 非阻塞模式获取信号量

```go
// timeout=0 表示不等待，立即返回
err = b.Acquire(0)
if err != nil {
    if fe, ok := err.(*bulkhead.FullError); ok {
        // 资源不足，执行降级逻辑
        fmt.Println("Database is busy, using cache instead")
        return cachedResult
    }
    return err
}
defer b.Release()

// 执行数据库操作...
```

#### 7.6.3 信号量与 Worker 池混合使用

```go
cfg := bulkhead.Config{
    MaxConcurrency: 10,
    MaxQueueSize:   20,
}

b, _ := bulkhead.NewBulkhead("mixed-usage", cfg)
defer b.Close()

// Worker 池模式：提交异步任务
for i := 0; i < 5; i++ {
    go func(id int) {
        err := b.Submit(func() {
            fmt.Printf("Async task %d executed by worker\n", id)
        })
        if err != nil {
            log.Printf("Submit task %d failed: %v", id, err)
        }
    }(i)
}

// 直接调用模式：在当前 goroutine 中执行
err := b.Acquire(1 * time.Second)
if err == nil {
    fmt.Println("Acquired semaphore, executing protected code")
    // 执行需要并发控制的代码...
    b.Release()
}

// 查询状态
fmt.Printf("Active: %d (workers + semaphore), Semaphore holders: %d, Queue: %d\n",
    b.ActiveCount(), b.SemaphoreCount(), b.QueueLength())
```

#### 7.6.4 并发 HTTP 请求限流

```go
http.HandleFunc("/api/orders", func(w http.ResponseWriter, r *http.Request) {
    // 获取信号量，最多等待 500ms
    err := b.Acquire(500 * time.Millisecond)
    if err != nil {
        w.Header().Set("Retry-After", "10")
        http.Error(w, "Service busy", http.StatusServiceUnavailable)
        return
    }
    defer b.Release()

    // 处理订单创建请求（在当前 HTTP handler goroutine 中）
    orderID, err := createOrder(r.Body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusCreated)
    fmt.Fprintf(w, `{"order_id": "%s"}`, orderID)
})
```

## 8. 核心 API 说明

### 8.1 Acquire(timeout time.Duration) error

获取一个并发槽位（信号量）。

**参数：**
- `timeout`：等待超时时间。0 表示不等待，立即返回结果。

**返回值：**
- `nil`：成功获取槽位
- `*FullError`：资源耗尽或等待超时，携带诊断信息
- `ErrBulkheadClosed`：隔离舱已关闭

### 8.2 Release() error

释放通过 `Acquire()` 获取的并发槽位。

**返回值：**
- `nil`：成功释放
- `ErrNotAcquired`：没有通过 `Acquire()` 持有槽位时调用

### 8.3 SemaphoreCount() int

查询当前通过 `Acquire()` 持有的信号量槽位数。

**返回值：** 当前持有的信号量槽位数

## 9. 线程安全与设计要点

### 9.1 同步原语选择

- **互斥锁 (sync.Mutex)**：保护所有内部状态的并发访问
- **条件变量 (sync.Cond)**：实现 Submit 与 worker 之间的等待/唤醒机制，避免忙等待
- **等待组 (sync.WaitGroup)**：跟踪 worker 协程生命周期，实现优雅关闭

### 9.2 关键设计决策

1. **切片队列替代 Channel**：使用 `[]Task` + `sync.Cond` 替代 channel 实现任务队列，支持动态调整队列容量，同时保证并发安全。

2. **空闲 worker 计数**：维护 `idleWorkers` 计数器，支持 `MaxQueueSize=0` 的无缓冲场景，确保有空闲 worker 时任务可以直接被处理。

3. **渐进式缩容**：通过 `shrinkCnt` 计数器标记待回收的 worker 数量，worker 在空闲时主动检测并退出，不强制中断正在执行的任务。

4. **单一截止时间**：等待模式下，`deadline` 只在首次进入等待时设置一次，无论被唤醒多少次都复用同一截止时间，确保实际等待时间不超过配置的 `WaitTimeout`。

5. **优雅关闭**：Close() 设置 `closed=true` 并唤醒所有等待者，worker 处理完队列中所有任务后才退出，保证已提交任务不丢失。

6. **信号量与 Worker 池共享配额**：`active` 计数器统一管理 worker 执行的任务和外部调用方持有的信号量，两种模式共享同一个并发上限。

## 10. 文件结构

```
internal/bulkhead/
├── bulkhead.go      # 舱壁隔离核心实现（Bulkhead、Registry、Config 等）
└── bulkhead_test.go # 单元测试（覆盖正常流程、边界条件、异常分支）

docs/
└── bulkhead.md      # 本文档
```
