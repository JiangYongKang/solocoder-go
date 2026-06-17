# 优雅关闭管理器模块 (Graceful Shutdown Manager)

## 1. 模块概述

优雅关闭管理器是一个用于协调 Go 应用程序安全退出的核心模块。它负责监听操作系统信号（如 SIGINT、SIGTERM）和内部关闭指令，按照预定义的阶段顺序执行关闭流程，确保正在处理的请求能够自然完成、已注册的资源清理回调能够按序执行，同时提供多层超时控制机制，在必要时强制终止并记录详细的状态快照。

### 主要功能

- **关闭信号监听**：监听操作系统终止信号和内部关闭指令，支持手动编程触发
- **请求等待机制**：关闭启动后拒绝新请求，等待进行中请求自然完成，设置最大等待时长
- **资源清理回调**：提供回调注册接口，各模块可注册资源释放函数，支持优先级和独立超时
- **全局超时保护**：设置整个关闭流程的全局超时，超时后强制终止并记录状态快照
- **多阶段顺序编排**：将关闭过程划分为明确阶段，各阶段按顺序严格执行

## 2. 核心结构体

### 2.1 Manager（关闭管理器）

包路径：`internal/graceful`

**职责**：
- 管理整个优雅关闭生命周期
- 协调信号监听、状态转换、阶段执行
- 维护活动请求计数器
- 管理清理回调的注册与执行
- 生成最终关闭报告

**主要字段（内部）**：
- `activeRequests int64`：活动请求计数器（原子操作，64位对齐）
- `cfg Config`：关闭配置参数
- `mu sync.RWMutex`：状态保护锁
- `state ShutdownState`：当前运行状态
- `phase ShutdownPhase`：当前关闭阶段
- `accepting bool`：是否仍接受新请求
- `callbacks map[string]*CleanupCallback`：已注册回调表

### 2.2 Config（配置结构体）

```go
type Config struct {
    RequestWaitTimeout      time.Duration   // 等待活动请求完成的最大时长
    GlobalTimeout           time.Duration   // 整个关闭流程的全局超时
    DefaultCallbackTimeout  time.Duration   // 回调默认超时时间
    StopAcceptingTimeout    time.Duration   // 停止接受阶段的缓冲时长
    Signals                 []os.Signal     // 监听的操作系统信号列表
}
```

**默认值（DefaultConfig()）**：
- `RequestWaitTimeout`: 30 秒
- `GlobalTimeout`: 60 秒
- `DefaultCallbackTimeout`: 10 秒
- `StopAcceptingTimeout`: 5 秒
- `Signals`: [SIGINT, SIGTERM]

### 2.3 CleanupCallback（清理回调）

```go
type CleanupCallback struct {
    Name     string        // 回调唯一标识名称
    Func     CleanupFunc   // 清理函数
    Timeout  time.Duration // 独立超时时间（0 则使用默认值）
    Priority int           // 优先级（越大越先执行，实际按反向遍历执行）
}
```

**说明**：
- 回调按 `Priority` 降序排序后，**从最后一个元素倒序执行**（即低优先级先执行，高优先级后执行）
- 相同优先级按名称字典序排序
- 每个回调有独立的超时保护，防止单个回调阻塞整个关闭流程

### 2.4 ShutdownReport（关闭报告）

```go
type ShutdownReport struct {
    Success             bool               // 关闭是否完全成功
    Forced              bool               // 是否因全局超时被强制终止
    Phase               ShutdownPhase      // 最终停留的阶段
    TotalDuration       time.Duration      // 关闭总耗时
    ActiveRequests      int                // 剩余活跃请求数快照
    GoroutineCount      int                // 协程总数快照
    CallbackResults     []*CallbackResult  // 各回调的执行结果
    IncompleteCallbacks []string           // 未完成回调的名称列表
    Errors              []error            // 所有阶段错误汇总
}
```

### 2.5 CallbackResult（回调执行结果）

```go
type CallbackResult struct {
    Name     string        // 回调名称
    Success  bool          // 是否执行成功
    TimedOut bool          // 是否因超时被中断
    Error    error         // 错误信息（如有）
    Duration time.Duration // 实际执行耗时
}
```

## 3. 状态与阶段定义

### 3.1 ShutdownState（运行状态）

| 状态 | 值 | 描述 |
|------|----|------|
| `StateRunning` | 0 | 正常运行，接受新请求 |
| `StateShuttingDown` | 1 | 关闭流程进行中 |
| `StateCompleted` | 2 | 关闭流程已结束 |

### 3.2 ShutdownPhase（关闭阶段）

| 阶段 | 值 | 描述 |
|------|----|------|
| `PhaseInit` | 0 | 初始化状态，尚未启动关闭 |
| `PhaseStopAccepting` | 1 | 停止接收新请求阶段 |
| `PhaseWaitRequests` | 2 | 等待进行中请求完成阶段 |
| `PhaseExecuteCallbacks` | 3 | 执行资源清理回调阶段 |
| `PhaseComplete` | 4 | 所有阶段成功完成 |
| `PhaseForced` | 5 | 因全局超时被强制终止 |

## 4. 关闭阶段执行顺序

关闭流程严格按以下顺序执行，每一阶段完成后才进入下一阶段。每一阶段执行前后都检查全局超时。

### 阶段 1: PhaseStopAccepting（停止接收新请求）

**触发动作**：
- 设置 `accepting = false`
- 所有后续 `BeginRequest()` 调用返回 `ErrManagerAlreadyShuttingDown`

**超时控制**：`StopAcceptingTimeout`（缓冲期，用于等待并发的 BeginRequest 完成原子增减）

**成功条件**：请求计数器归零 或 达到缓冲时间上限

### 阶段 2: PhaseWaitRequests（等待进行中请求）

**触发动作**：
- 以 50ms 的轮询间隔检查活动请求数
- 持续统计剩余未完成请求数

**超时控制**：`RequestWaitTimeout`

**成功条件**：活动请求数归零

**超时后**：记录超时错误（包含当前剩余请求数），继续进入下一阶段

### 阶段 3: PhaseExecuteCallbacks（执行清理回调）

**执行策略**：
1. 将所有回调按 Priority 降序排序（同优先级按名称排序）
2. **按排序结果反序遍历**（低优先级 → 高优先级执行）
3. 每个回调在独立 goroutine 中执行
4. 每个回调受自身独立超时保护
5. 捕获回调 panic，包装为错误
6. 任一阶段检查全局超时时，将剩余回调全部标记为超时未完成

**超时控制**：每个回调独立的 `Timeout` + 全局 `GlobalTimeout`

### 阶段 4: PhaseComplete / PhaseForced

**正常路径**：所有阶段在全局超时内完成 → `PhaseComplete`，`Success = true`

**强制终止**：任一阶段执行时触发全局超时 → `PhaseForced`，`Forced = true`

**最终状态**：所有情况下最终 `state` 都设置为 `StateCompleted`

## 5. 超时控制层级

```
GlobalTimeout (最外层)
    ├── PhaseStopAccepting (StopAcceptingTimeout)
    ├── PhaseWaitRequests (RequestWaitTimeout)
    └── PhaseExecuteCallbacks
            ├── Callback_1.Timeout
            ├── Callback_2.Timeout
            └── ...
```

- **全局超时**：最先被检查，超过后所有剩余操作被强制终止
- **阶段超时**：各自独立控制，超时后记录错误并进入下一阶段
- **回调超时**：单个回调的执行时限，超时后标记 TimedOut=true，继续执行下一个回调

## 6. 错误定义

| 错误变量 | 描述 |
|----------|------|
| `ErrManagerAlreadyShuttingDown` | 关闭已启动或完成，重复触发/注册/请求 |
| `ErrManagerNotRunning` | Manager 已经启动过，重复 Start 调用 |
| `ErrCallbackAlreadyRegistered` | 回调名称已被注册 |
| `ErrCallbackNotFound` | 尝试注销不存在的回调 |
| `ErrNilCallback` | 注册的清理函数为 nil |
| `ErrManagerNotStarted` | （预留）Manager 未启动 |

## 7. 使用示例

### 7.1 基础使用

```go
package main

import (
    "context"
    "log"
    "solocoder-go/internal/graceful"
    "time"
)

func main() {
    cfg := graceful.DefaultConfig()
    cfg.RequestWaitTimeout = 10 * time.Second
    cfg.GlobalTimeout = 30 * time.Second

    mgr := graceful.NewManager(cfg)

    // 注册资源清理回调
    mgr.RegisterCallback("close-db", func(ctx context.Context) error {
        // 关闭数据库连接
        return db.Close()
    }, 5*time.Second, 100) // 高优先级最后执行

    mgr.RegisterCallback("close-cache", func(ctx context.Context) error {
        // 关闭缓存客户端
        return cache.Close()
    }, 3*time.Second, 50)

    // 启动信号监听
    if err := mgr.Start(); err != nil {
        log.Fatal(err)
    }

    // 模拟处理请求
    go handleRequests(mgr)

    // 等待关闭完成并获取报告
    report := mgr.WaitForShutdown()

    if report.Success {
        log.Println("优雅关闭成功")
    } else {
        log.Printf("关闭不完全: 阶段=%v, 错误数=%d", report.Phase, len(report.Errors))
        for _, name := range report.IncompleteCallbacks {
            log.Printf("  未完成回调: %s", name)
        }
    }
}

func handleRequests(mgr *graceful.Manager) {
    for {
        if err := mgr.BeginRequest(); err != nil {
            return // 关闭已启动，退出
        }
        // ... 处理请求 ...
        mgr.EndRequest()
    }
}
```

### 7.2 手动触发关闭

```go
// 除了信号触发，也可以在代码中主动关闭
if err := mgr.TriggerShutdown(); err != nil {
    log.Printf("触发关闭失败: %v", err)
}

// 或者直接阻塞调用
mgr.Shutdown()

// 获取报告
report := mgr.GetReport()
```

### 7.3 结合 HTTP 服务器

```go
func main() {
    mgr := graceful.NewManager(graceful.DefaultConfig())
    mgr.Start()

    srv := &http.Server{Addr: ":8080"}

    // 注册 HTTP 服务关闭回调
    mgr.RegisterCallback("http-server", func(ctx context.Context) error {
        return srv.Shutdown(ctx)
    }, 15*time.Second, 100)

    // HTTP Handler 中使用
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        if err := mgr.BeginRequest(); err != nil {
            http.Error(w, "服务关闭中", http.StatusServiceUnavailable)
            return
        }
        defer mgr.EndRequest()
        // ... 处理业务逻辑 ...
    })

    go srv.ListenAndServe()

    // 阻塞直到关闭完成
    mgr.WaitForShutdown()
}
```

### 7.4 回调优先级顺序示例

注册三个回调：
```go
mgr.RegisterCallback("A", cleanupA, 0, 100) // 最高优先级
mgr.RegisterCallback("B", cleanupB, 0, 50)  // 中等优先级
mgr.RegisterCallback("C", cleanupC, 0, 10)  // 最低优先级
```

**实际执行顺序**：C → B → A

原因：按 Priority 降序排序后得到 [A, B, C]，然后从最后一个元素倒序遍历。这样设计使得高优先级的资源（如数据库连接）在最后释放，确保低优先级组件在清理时高优先级资源仍可用。

## 8. 并发安全

Manager 的所有公共方法都是并发安全的：
- `BeginRequest/EndRequest` 使用原子操作维护计数
- 状态/回调表等共享结构使用 `sync.RWMutex` 保护
- `Shutdown()` 使用 `sync.Once` 确保只执行一次
- 重复调用 `Shutdown()` 返回 `ErrManagerAlreadyShuttingDown`

## 9. 编程接口速览

| 方法 | 签名 | 说明 |
|------|------|------|
| `NewManager` | `func(cfg Config) *Manager` | 创建新管理器实例 |
| `Start` | `func() error` | 启动信号监听协程 |
| `Shutdown` | `func() error` | 同步触发关闭流程（阻塞） |
| `TriggerShutdown` | `func() error` | 异步触发关闭流程（不阻塞） |
| `WaitForShutdown` | `func() *ShutdownReport` | 阻塞等待关闭完成，返回报告 |
| `ShutdownDone` | `func() <-chan struct{}` | 获取关闭完成通知通道 |
| `GetReport` | `func() *ShutdownReport` | 获取关闭报告（无需等待） |
| `BeginRequest` | `func() error` | 标记请求开始，关闭中返回错误 |
| `EndRequest` | `func()` | 标记请求结束 |
| `ActiveRequests` | `func() int` | 查询当前活动请求数 |
| `IsAccepting` | `func() bool` | 查询是否仍接受新请求 |
| `State` | `func() ShutdownState` | 查询当前运行状态 |
| `Phase` | `func() ShutdownPhase` | 查询当前关闭阶段 |
| `RegisterCallback` | `func(name string, fn CleanupFunc, timeout time.Duration, priority int) error` | 注册清理回调 |
| `UnregisterCallback` | `func(name string) error` | 注销已注册回调 |

## 10. 模块文件结构

```
internal/graceful/
├── graceful.go        # 核心实现代码 (~600 行)
└── graceful_test.go   # 单元测试代码 (~1000 行)

docs/
└── graceful.md        # 本文档
```

### 测试覆盖范围

单元测试共 **35 个测试用例**，覆盖：

**正常流程**：
- 创建/启动/关闭基本流程
- 请求计数器的正确性
- 回调注册与执行
- 回调按优先级反序执行

**边界条件**：
- 空配置/零值配置回退到默认值
- 重复 Start / Shutdown / TriggerShutdown
- 回调名称重复/空名/nil 函数
- 0 活动请求快速关闭
- 同时有多个 WaitForShutdown 等待者

**异常分支**：
- 回调返回错误
- 回调 panic
- 单个回调超时
- 请求等待阶段超时
- 全局超时在各阶段触发
- 关闭启动后拒绝新请求
- 关闭启动后禁止注册回调
- 并发 BeginRequest 与关闭的竞态
- 并发注册回调与关闭的竞态
