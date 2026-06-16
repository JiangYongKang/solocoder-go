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
|------