# 超时传播器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [超时传播与预算分配流程](#4-超时传播与预算分配流程)
5. [超时类型与阶段状态](#5-超时类型与阶段状态)
6. [链路追踪报告](#6-链路追踪报告)
7. [使用示例](#7-使用示例)
8. [错误定义](#8-错误定义)
9. [配置说明](#9-配置说明)
10. [并发安全](#10-并发安全)
11. [最佳实践](#11-最佳实践)

---

## 1. 模块概述

超时传播器（Timeout Propagator）是一个用于在分布式调用链中管理和传播超时的通用模块。在复杂的微服务架构中，一个请求往往需要经过多个处理环节，每个环节都有自己的超时要求。超时传播器通过统一管理调用链的超时预算，确保整体超时得到严格控制，同时为每个环节动态分配剩余时间，并提供完整的超时链路追踪能力。

**包路径**: `internal/timeoutprop`

**设计目标**:
- 提供标准化的超时传播机制，确保调用链整体超时可控
- 支持为每个子环节预分配时间预算，预算可结转复用
- 动态计算各环节剩余可用时间，不足最小阈值时自动跳过
- 记录完整的超时链路追踪信息，便于问题定位与分析
- 完全兼容 `context.Context`，支持父 context 的超时传递
- 生成可读的调用链超时报告，供调用方查询分析

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Context 超时自动传递 | 根 context 设置总超时，各子环节自动派生子 context，父超时则所有子同步取消 |
| 剩余时间动态计算 | 每个环节开始时自动查询剩余可用时间，等于截止时间减去当前系统时间 |
| 最小阈值跳过 | 剩余时间不足可配置的最小阈值时，直接跳过该环节并记录超时日志 |
| 超时预算分配 | 为各子环节预先分配时间预算，总预算不超过根 context 总超时 |
| 预算结转复用 | 环节提前完成时，未消费的剩余预算传递给后续环节使用 |
| 超时链路追踪 | 记录每个环节的超时类型、已消耗时间与分配预算对比 |
| 调用链报告 | 汇总整条调用链的超时追踪信息，生成可读报告 |
| 阶段状态管理 | 跟踪每个阶段的状态：待执行、运行中、已完成、已跳过、已超时、失败 |
| 重置功能 | 支持重置传播器状态，便于重复使用 |
| 便捷函数调用 | 提供 `Execute()` 函数式调用方式，支持 Option 模式配置 |

---

## 3. 核心结构体与职责

### 3.1 Propagator

超时传播器主结构体，管理整个调用链的超时传播。

```go
type Propagator struct {
    mu           sync.Mutex
    stages       []*Stage
    stageMap     map[string]*Stage
    totalTimeout time.Duration
    minThreshold time.Duration
    rootCancel   context.CancelFunc
    report       *ChainReport
    executed     bool
}
```

**职责**:
- 管理调用链中的所有阶段（Stage）
- 执行 `Execute()` 主方法，驱动整条调用链
- 创建根 context 并设置总超时
- 动态计算每个阶段的可用预算（含结转预算）
- 检测剩余时间是否低于最小阈值
- 收集各阶段执行信息，生成链路追踪报告
- 提供状态查询接口：`Report()`、`GetStageInfo()`、`RemainingTime()`
- 支持 `Reset()` 重置状态，便于重复使用

### 3.2 Stage

调用链中的一个执行阶段。

```go
type Stage struct {
    name            string
    fn              StageFunc
    allocatedBudget time.Duration
    mu              sync.Mutex
    info            *StageInfo
}
```

**职责**:
- 封装阶段名称、执行函数和分配的预算
- 维护阶段执行信息（状态、耗时、错误等）
- 提供线程安全的信息更新和读取

### 3.3 StageInfo

阶段执行信息结构体，记录单个阶段的详细执行数据。

```go
type StageInfo struct {
    Name            string
    AllocatedBudget time.Duration
    UsedBudget      time.Duration
    RemainingBudget time.Duration
    Status          StageStatus
    TimeoutType     TimeoutType
    StartTime       time.Time
    EndTime         time.Time
    Error           error
}
```

**职责**:
- 记录阶段的预算分配和使用情况
- 跟踪阶段的执行状态和超时类型
- 记录阶段的开始和结束时间
- 保存阶段执行过程中发生的错误

### 3.4 ChainReport

调用链超时报告，汇总整条链的执行信息。

```go
type ChainReport struct {
    TotalTimeout  time.Duration
    TotalUsed     time.Duration
    RemainingTime time.Duration
    Stages        []*StageInfo
    Success       bool
    FailedStage   string
    TimeoutReason string
}
```

**职责**:
- 汇总调用链的总超时、总耗时和剩余时间
- 按顺序包含所有阶段的详细信息
- 标识调用链是否成功执行
- 记录失败阶段和超时原因
- 提供 `String()` 方法生成可读的文本报告

### 3.5 Config

配置结构体，用于定制超时传播器行为。

```go
type Config struct {
    TotalTimeout time.Duration
    MinThreshold time.Duration
}
```

**默认配置**（`DefaultConfig()`）:
- `TotalTimeout`: 10s
- `MinThreshold`: 10ms

### 3.6 函数类型

```go
type StageFunc func(ctx context.Context) error
type Option    func(*Config)
```

| 类型 | 说明 |
|------|------|
| `StageFunc` | 阶段执行函数，接收 Context 便于传递取消信号 |
| `Option` | 函数式选项，用于便捷函数调用 |

---

## 4. 超时传播与预算分配流程

### 4.1 整体执行流程

```
调用 Execute(parentCtx)
    │
    ▼
  ┌─────────────────────────────────────────────┐
  │  1. 前置检查                                │
  │     - 是否已执行过                          │
  │     - 是否有注册阶段                        │
  │     - 总预算是否超过总超时                  │
  └─────────────────────────────────────────────┘
    │
    ▼
  ┌─────────────────────────────────────────────┐
  │  2. 创建根 context                          │
  │     rootCtx = WithTimeout(parentCtx, total) │
  └─────────────────────────────────────────────┘
    │
    ▼
  ┌─────────────────────────────────────────────┐
  │  3. 遍历所有阶段                            │
  │     ┌──────────────────────────────────┐   │
  │     │  3.1 检查根 context 是否已取消   │   │
  │     │     - 已取消 → 标记跳过 → continue│   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.2 计算阶段可用预算            │   │
  │     │     可用预算 = 分配预算 + 结转预算│   │
  │     │     可用预算 = min(可用预算, 剩余)│   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.3 最小阈值检查                │   │
  │     │     预算 < 最小阈值 → 跳过 → next │   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.4 创建阶段 context            │   │
  │     │     - 有预算 → WithTimeout       │   │
  │     │     - 无预算 → WithCancel        │   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.5 执行阶段函数                │   │
  │     │     在 goroutine 中运行，带 panic  │   │
  │     │     保护                          │   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.6 等待完成或超时              │   │
  │     │     select 监听完成/阶段超时/总超时│   │
  │     └──────────────────────────────────┘   │
  │     ┌──────────────────────────────────┐   │
  │     │  3.7 处理结果                    │   │
  │     │     - 成功 → 计算结转预算 → next  │   │
  │     │     - 失败 → 标记后续跳过 → break │   │
  │     └──────────────────────────────────┘   │
  └─────────────────────────────────────────────┘
    │
    ▼
  ┌─────────────────────────────────────────────┐
  │  4. 生成报告并返回                          │
  └─────────────────────────────────────────────┘
```

### 4.2 预算分配与结转规则

**预算计算公式**:
```
阶段可用预算 = 该阶段分配预算 + 从前序阶段结转的剩余预算
阶段实际预算 = min(阶段可用预算, 根 context 剩余时间)
```

**预算结转规则**:
- 如果阶段在分配的预算内提前完成，剩余预算结转到下一阶段
- 结转预算 = 阶段预算 - 实际耗时
- 如果阶段耗时超过预算（超时），则没有预算结转
- 如果阶段没有预算限制（分配预算为 0），则不产生结转预算

**预算约束**:
- 所有阶段的分配预算之和不得超过总超时时间
- 阶段实际预算不得超过根 context 的剩余时间

### 4.3 最小阈值跳过机制

在每个阶段开始执行前，检查**根 context 的剩余时间**是否小于配置的最小阈值（`MinThreshold`）。如果剩余时间不足，该阶段直接被跳过，不执行实际业务逻辑。

**判定逻辑**:
```
剩余时间 = 根 context 截止时间 - 当前系统时间
如果 剩余时间 < MinThreshold:
    标记阶段为 SKIPPED，超时类型为 MIN_THRESHOLD_SKIP
    跳过当前阶段，继续下一阶段
```

**应用场景**:
- 避免为了极短的剩余时间而启动一个可能来不及完成的操作
- 减少不必要的资源消耗
- 快速失败，将剩余时间留给更重要的环节

**注意**:
- 使用根 context 的**剩余时间**进行判断，而非阶段自身的预算
- **零预算阶段也会检查最小阈值**，不设例外
- 被跳过的阶段状态标记为 `SKIPPED`，超时类型为 `MIN_THRESHOLD_SKIP`
- 当 `MinThreshold = 0` 时，不进行最小阈值检查

---

## 5. 超时类型与阶段状态

### 5.1 TimeoutType（超时类型）

| 常量 | 值 | 说明 |
|------|----|------|
| `TimeoutTypeNone` | 0 | 未发生超时 |
| `TimeoutTypeTotal` | 1 | 总超时（根 context 超时） |
| `TimeoutTypeBudget` | 2 | 预算超时（阶段自身预算耗尽） |
| `TimeoutTypeMinThreshold` | 3 | 因低于最小阈值而被跳过 |

### 5.2 StageStatus（阶段状态）

| 常量 | 值 | 说明 |
|------|----|------|
| `StageStatusPending` | 0 | 待执行 |
| `StageStatusRunning` | 1 | 正在执行 |
| `StageStatusCompleted` | 2 | 成功完成 |
| `StageStatusSkipped` | 3 | 被跳过（因最小阈值或前置阶段失败） |
| `StageStatusTimedOut` | 4 | **超时**（因总超时或预算超时，错误为 `context.DeadlineExceeded`） |
| `StageStatusFailed` | 5 | **业务失败**（阶段返回非超时类的业务错误或 panic） |

**错误类型区分策略**:

| 错误场景 | 阶段状态 | 超时类型 | 返回错误 | `errors.Is(err, context.DeadlineExceeded)` |
|----------|----------|----------|----------|-------------------------------------------|
| 根 context 超时 | `TimedOut` | `Total` | `*StageTimeoutError` | ✅ `true` |
| 阶段预算超时 | `TimedOut` | `Budget` | `*StageTimeoutError` | ✅ `true` |
| 阶段返回业务错误 | `Failed` | `None` | 原始业务错误 | ❌ `false` |
| 阶段发生 panic | `Failed` | `None` | 包装后的 panic 错误 | ❌ `false` |
| 低于最小阈值被跳过 | `Skipped` | `MinThreshold` | 无（不返回错误） | - |

---

## 6. 链路追踪报告

### 6.1 报告内容

`ChainReport` 提供以下信息：

- **整体信息**:
  - 总超时时间
  - 已使用时间
  - 剩余时间
  - 是否成功
  - 失败阶段名称
  - 超时原因

- **各阶段信息**（按执行顺序）:
  - 阶段名称
  - 分配预算 vs 已用预算 vs 剩余预算
  - 阶段状态
  - 超时类型（如果发生超时）
  - 开始时间和结束时间
  - 错误信息（如果发生错误）

### 6.2 报告示例

```
=== Timeout Propagation Chain Report ===
Total Timeout: 1s
Total Used: 150ms
Remaining: 850ms
Success: true

Stages:
  [0] validate
      Status: COMPLETED
      Budget: 100ms / Used: 50ms / Remaining: 50ms
  [1] process
      Status: COMPLETED
      Budget: 150ms / Used: 100ms / Remaining: 50ms
  [2] respond
      Status: COMPLETED
      Budget: 200ms / Used: 0s / Remaining: 200ms
```

---

## 7. 使用示例

### 7.1 基本使用（使用 Propagator 实例）

```go
package main

import (
    "context"
    "fmt"
    "solocoder-go/internal/timeoutprop"
    "time"
)

func main() {
    p := timeoutprop.NewPropagatorWithConfig(timeoutprop.Config{
        TotalTimeout: 1 * time.Second,
        MinThreshold: 10 * time.Millisecond,
    })

    p.AddStage("validate", 100*time.Millisecond, func(ctx context.Context) error {
        return validateRequest(ctx)
    })

    p.AddStage("process", 300*time.Millisecond, func(ctx context.Context) error {
        return processData(ctx)
    })

    p.AddStage("respond", 200*time.Millisecond, func(ctx context.Context) error {
        return sendResponse(ctx)
    })

    report, err := p.Execute(context.Background())
    if err != nil {
        fmt.Printf("调用链执行失败: %v\n", err)
        fmt.Printf("失败阶段: %s\n", report.FailedStage)
        fmt.Printf("超时原因: %s\n", report.TimeoutReason)
    }

    fmt.Println(report.String())
}
```

### 7.2 便捷函数式调用（Option 模式）

```go
import "solocoder-go/internal/timeoutprop"

report, err := timeoutprop.Execute(ctx,
    func(p *timeoutprop.Propagator) error {
        p.AddStage("stage1", 100*time.Millisecond, handler1)
        p.AddStage("stage2", 200*time.Millisecond, handler2)
        return nil
    },
    timeoutprop.WithTotalTimeout(1*time.Second),
    timeoutprop.WithMinThreshold(5*time.Millisecond),
)
```

### 7.3 查询剩余时间

```go
func myHandler(ctx context.Context) error {
    p := timeoutprop.NewPropagator()
    remaining := p.RemainingTime(ctx)
    
    if remaining < 50*time.Millisecond {
        return fmt.Errorf("时间不足，剩余 %v", remaining)
    }
    
    return doWork(ctx)
}
```

### 7.4 检查超时类型

```go
import "errors"
import "context"

report, err := p.Execute(ctx)
if err != nil {
    // 使用标准的 errors.Is 判断是否为超时错误（推荐）
    if errors.Is(err, context.DeadlineExceeded) {
        log.Println("发生超时错误")
    }

    // 使用 errors.As 提取详细的超时信息
    var stageErr *timeoutprop.StageTimeoutError
    if errors.As(err, &stageErr) {
        switch stageErr.TimeoutType {
        case timeoutprop.TimeoutTypeTotal:
            log.Println("总超时：根 context 超时")
        case timeoutprop.TimeoutTypeBudget:
            log.Printf("阶段 %s 预算超时：分配 %v，使用 %v",
                stageErr.StageName, stageErr.Allocated, stageErr.Used)
        }
    }

    // 通过报告判断具体的失败类型
    if report.FailedStage != "" {
        stageInfo := report.Stages[0] // 获取第一个失败的阶段
        switch stageInfo.Status {
        case timeoutprop.StageStatusTimedOut:
            log.Printf("阶段 %s 超时失败", stageInfo.Name)
        case timeoutprop.StageStatusFailed:
            log.Printf("阶段 %s 业务错误失败: %v", stageInfo.Name, stageInfo.Error)
        }
    }
}
```

### 7.5 重置并重用 Propagator

```go
p := timeoutprop.NewPropagator()
p.AddStage("stage1", 100*time.Millisecond, handler)

// 第一次执行
report1, _ := p.Execute(ctx1)

// 重置状态
p.Reset()

// 第二次执行
report2, _ := p.Execute(ctx2)
```

### 7.6 无预算限制的阶段

```go
p.AddStage("unlimited", 0, func(ctx context.Context) error {
    // 该阶段没有独立的预算限制
    // 使用根 context 的所有剩余时间
    return longRunningTask(ctx)
})
```

---

## 8. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidConfig` | 配置无效 | 预留错误常量 |
| `ErrStageNotFound` | 阶段不存在 | 查询未注册的阶段时 |
| `ErrStageAlreadyExists` | 阶段已存在 | 添加重名阶段时 |
| `ErrEmptyName` | 阶段名称为空 | 添加阶段时名称为空 |
| `ErrNilHandler` | 处理函数为空 | 添加阶段时函数为 nil |
| `ErrBudgetExceedsTotal` | 总预算超过总超时 | 所有阶段预算之和 > 总超时时 |
| `ErrNegativeBudget` | 预算为负 | 添加阶段时预算为负数 |
| `ErrChainAlreadyExecuted` | 调用链已执行 | 已执行后再添加阶段或重复执行 |
| `ErrNoStages` | 没有注册阶段 | 执行时没有任何阶段 |
| `*StageTimeoutError` | 阶段超时错误 | 阶段因总超时或预算超时而失败时 |

**`StageTimeoutError` 结构**:
```go
type StageTimeoutError struct {
    StageName   string        // 超时阶段名称
    TimeoutType TimeoutType   // 超时类型（Total/Budget）
    Allocated   time.Duration // 分配的预算
    Used        time.Duration // 实际消耗的时间
}
```

**错误处理约定**:
- `StageTimeoutError` 实现了 `Unwrap() error` 方法，对于超时类型为 `Total` 或 `Budget` 的错误，返回标准的 `context.DeadlineExceeded`
- 调用方可以使用 `errors.Is(err, context.DeadlineExceeded)` 来判断是否为超时错误
- 调用方可以使用 `errors.As(err, &stageErr)` 来提取 `StageTimeoutError` 的详细信息
- 业务错误（非超时）不会被包装为 `StageTimeoutError`，而是直接返回原始错误，此时阶段状态为 `Failed`，超时类型为 `None`

---

## 9. 配置说明

### 9.1 完整配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `TotalTimeout` | `time.Duration` | 10s | 调用链总超时时间（正数） |
| `MinThreshold` | `time.Duration` | 10ms | 最小执行阈值，**根 context 剩余时间**低于此值则跳过阶段 |

### 9.2 配置归一化规则

创建 `Propagator` 时，以下非法值会被自动修正：

| 非法值 | 修正为 |
|--------|--------|
| `TotalTimeout <= 0` | 默认值 10s |
| `MinThreshold < 0` | 0（不设置最小阈值） |

### 9.3 Option 函数

| 函数 | 说明 |
|------|------|
| `WithTotalTimeout(d)` | 设置总超时时间 |
| `WithMinThreshold(d)` | 设置最小阈值 |

---

## 10. 并发安全

| 维度 | 安全状态 | 说明 |
|------|---------|------|
| 单个 `Propagator` 实例并发调用 `Execute()` | ⚠️ 不安全 | `Propagator` 内部维护 `executed` 和 `report` 状态，单个实例不应被多个 goroutine 共享执行 |
| 多个 goroutine 各自创建 `Propagator` | ✓ 安全 | 推荐每个 goroutine 使用独立的 `Propagator` 实例 |
| 便捷函数 `Execute()` 并发调用 | ✓ 安全 | 每次调用内部创建独立 `Propagator`，完全并发安全 |
| 读取已执行完成的 `Report()` | ✓ 安全 | 执行完成后读取报告是安全的 |

**推荐模式**:

```go
// 好：每个 goroutine 创建独立 Propagator
for i := 0; i < N; i++ {
    go func() {
        p := timeoutprop.NewPropagator()
        p.Execute(ctx, stages)
    }()
}

// 好：直接使用并发安全的便捷函数
for i := 0; i < N; i++ {
    go func() {
        timeoutprop.Execute(ctx, setupFunc,
            timeoutprop.WithTotalTimeout(1*time.Second),
        )
    }()
}

// 不好：共享同一个 Propagator
p := timeoutprop.NewPropagator()
for i := 0; i < N; i++ {
    go func() {
        p.Execute(ctx)  // 并发读写内部状态，有竞态
    }()
}
```

---

## 11. 最佳实践

### 11.1 预算分配建议

| 场景 | 总超时 | 预算分配策略 |
|------|--------|-------------|
| 请求处理流水线 | 1s | 按环节重要性分配，关键环节预留更多预算 |
| 数据库操作链 | 500ms | 给查询操作更多预算，更新操作少一些 |
| 外部服务调用 | 2s | 考虑网络波动，适当放宽预算 |
| 本地计算任务 | 100ms | 精确分配，减少浪费 |

### 11.2 最小阈值设置建议

- **高吞吐低延迟场景**: 设置较小的阈值（如 1-5ms），避免浪费时间
- **高开销启动场景**: 设置较大的阈值（如 50-100ms），避免启动了却来不及完成
- **不确定场景**: 从 10ms 开始，根据实际情况调整

### 11.3 防御性编程

1. **Context 必传**: 始终传递带有超时的 `context.Context`
2. **检查返回错误**: 始终检查 `Execute()` 的返回错误
3. **利用报告诊断**: 超时时通过 `ChainReport` 定位瓶颈阶段
4. **合理设置预算**: 根据历史数据合理分配各阶段预算
5. **预算结转利用**: 通过预算结转提高整体时间利用率

### 11.4 典型反模式

❌ **反模式 1: 预算总和远小于总超时**
```go
// 不好：预算过于保守，总超时用不完
p := timeoutprop.NewPropagatorWithConfig(timeoutprop.Config{
    TotalTimeout: 5 * time.Second,
})
p.AddStage("a", 100*time.Millisecond, handlerA)
p.AddStage("b", 200*time.Millisecond, handlerB)
// 总预算才 300ms，总超时 5s，大部分时间被浪费
```

✅ **修正**: 根据实际需要合理分配预算，或使用 0 预算让阶段使用全部剩余时间。

---

❌ **反模式 2: 所有阶段都不设预算**
```go
// 不好：无法控制各阶段时间占比
p.AddStage("a", 0, handlerA)
p.AddStage("b", 0, handlerB)
p.AddStage("c", 0, handlerC)
// 第一个阶段可能把时间都用光了
```

✅ **修正**: 给每个阶段分配合适的预算，确保关键阶段有时间执行。

---

❌ **反模式 3: 忽略最小阈值**
```go
// 不好：只剩 1ms 还启动一个数据库查询
p := timeoutprop.NewPropagatorWithConfig(timeoutprop.Config{
    MinThreshold: 0,  // 不设最小阈值
})
```

✅ **修正**: 设置合理的最小阈值，避免为来不及完成的操作浪费资源。
