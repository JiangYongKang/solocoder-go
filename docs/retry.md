# 指数退避重试器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [指数退避曲线原理](#4-指数退避曲线原理)
5. [随机抖动机制](#5-随机抖动机制)
6. [可重试错误判定机制](#6-可重试错误判定机制)
7. [重试回调钩子](#7-重试回调钩子)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [配置说明](#10-配置说明)
11. [并发安全](#11-并发安全)
12. [最佳实践](#12-最佳实践)

---

## 1. 模块概述

指数退避重试器（Exponential Backoff Retrier）是一个用于处理临时性故障自动重试的通用模块。在分布式系统中，网络超时、服务暂时不可用、资源竞争等临时性错误是常态，通过指数退避策略可以在不加重系统负载的前提下，显著提高操作的成功率。

**包路径**: `internal/retry`

**设计目标**:
- 提供标准化的指数退避重试实现，避免各业务模块重复造轮子
- 通过随机抖动（Jitter）避免羊群效应（Thundering Herd）
- 支持可配置的可重试错误判定，对不可恢复错误快速失败
- 提供重试前后的回调钩子，便于日志记录与指标收集
- 完全兼容 `context.Context`，支持超时与取消
- 返回聚合错误，便于调用方诊断所有失败原因

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 指数退避间隔 | 等待间隔从初始值开始每次乘以 2，直到达到上限后保持恒定 |
| 最大间隔封顶 | 退避间隔增长到配置的最大间隔后不再增加 |
| 随机抖动 | 在计算间隔基础上叠加 ±N% 的随机偏移，避免并发重试碰撞 |
| 可重试判定 | 通过 `IsRetryableFunc` 自定义哪些错误需要重试，哪些立即中止 |
| 最大重试次数 | 设置重试上限，达到后返回聚合错误 |
| 重试前回调 | `OnRetryBefore` 在每次等待之前执行，用于日志/指标 |
| 重试后回调 | `OnRetryAfter` 在每次等待之后执行，用于清理或状态同步 |
| Context 支持 | 完全兼容 `context.Context`，支持取消与超时 |
| 聚合错误 | 返回所有尝试的错误集合，调用方可通过 `errors.As` 提取 |
| 便捷函数 | 提供 `Do()` 函数式调用方式，支持 Option 模式配置 |

---

## 3. 核心结构体与职责

### 3.1 Retryer

重试器主结构体，封装重试逻辑与状态。

```go
type Retryer struct {
    cfg       Config
    attempts  int
    errs      []error
    randSrc   *rand.Rand
    sleepFunc func(time.Duration)
}
```

**职责**:
- 执行 `Do()` 主循环，驱动重试流程
- 维护重试计数与错误历史
- 计算每次重试的指数退避间隔（含抖动）
- 在重试前后触发回调钩子
- 提供状态查询接口 `Attempts()`、`Errors()`、`Config()`

### 3.2 Config

配置结构体，用于定制重试行为。

```go
type Config struct {
    InitialInterval time.Duration
    MaxInterval     time.Duration
    MaxRetries      int
    JitterFactor    float64
    IsRetryable     IsRetryableFunc
    OnRetryBefore   OnRetryFunc
    OnRetryAfter    OnRetryFunc
}
```

**默认配置**（`DefaultConfig()`）:
- `InitialInterval`: 100ms
- `MaxInterval`: 10s
- `MaxRetries`: 3
- `JitterFactor`: 0.1（±10%）
- `IsRetryable`: `DefaultIsRetryable`

### 3.3 AggregateError

聚合错误类型，封装所有尝试的错误。

```go
type AggregateError struct {
    Errors []error
}
```

**职责**:
- 收集所有失败尝试的错误
- 实现 `error` 接口，提供格式化的错误信息
- 通过 `Unwrap() []error` 支持 `errors.Is`/`errors.As` 链式检查
- 调用方可通过 `errors.As(err, &aggErr)` 提取所有错误

### 3.4 函数类型

```go
type RetryableFunc func(ctx context.Context) error
type IsRetryableFunc func(err error) bool
type OnRetryFunc     func(attempt int, err error)
type Option          func(*Config)
```

| 类型 | 说明 |
|------|------|
| `RetryableFunc` | 需要重试的业务操作，接收 Context 便于传递取消信号 |
| `IsRetryableFunc` | 错误判定函数，返回 true 表示该错误需要重试 |
| `OnRetryFunc` | 重试回调函数，参数为当前重试次数（从 1 开始）和刚发生的错误 |
| `Option` | 函数式选项，用于 `Do()` 便捷调用 |

---

## 4. 指数退避曲线原理

### 4.1 算法描述

指数退避的核心思想是：随着失败次数增加，重试间隔呈指数级增长，给系统足够的恢复时间。

**计算公式**:

```
interval(attempt) = min(InitialInterval * 2^attempt, MaxInterval)
```

其中 `attempt` 从 0 开始计数（第 1 次重试前的等待对应 attempt=0）。

### 4.2 退避曲线示例

假设配置:
- `InitialInterval = 100ms`
- `MaxInterval = 10s`

| 重试次数 | attempt 参数 | 计算间隔 | 实际等待 |
|---------|-------------|---------|---------|
| 第 1 次重试前 | 0 | 100 × 2⁰ = 100ms | 100ms |
| 第 2 次重试前 | 1 | 100 × 2¹ = 200ms | 200ms |
| 第 3 次重试前 | 2 | 100 × 2² = 400ms | 400ms |
| 第 4 次重试前 | 3 | 100 × 2³ = 800ms | 800ms |
| 第 5 次重试前 | 4 | 100 × 2⁴ = 1600ms | 1.6s |
| 第 6 次重试前 | 5 | 100 × 2⁵ = 3200ms | 3.2s |
| 第 7 次重试前 | 6 | 100 × 2⁶ = 6400ms | 6.4s |
| 第 8 次重试前 | 7 | 100 × 2⁷ = 12800ms | **10s（封顶）** |
| 第 9 次重试前 | 8 | 100 × 2⁸ = 25600ms | **10s（封顶）** |
| ... | ... | ... | **10s（封顶）** |

### 4.3 为什么需要最大间隔封顶

如果没有最大间隔限制，指数增长会导致间隔变得过长（例如第 15 次重试间隔达到 3276.8 秒 ≈ 54 分钟），这在实际系统中是不可接受的。封顶后，重试频率保持在一个合理的上限。

---

## 5. 随机抖动机制

### 5.1 羊群效应问题

在没有抖动的情况下，如果多个客户端在同一时刻遇到故障（例如服务端瞬时重启），它们会按照完全相同的退避间隔进行重试：

```
时间轴:  0ms     100ms     200ms     400ms     800ms
客户端A:  × ──────┴─────────┴─────────┴─────────┴──►
客户端B:  × ──────┴─────────┴─────────┴─────────┴──►
客户端C:  × ──────┴─────────┴─────────┴─────────┴──►
                              ↑ 同时重试，形成流量尖峰
```

这种同步重试会对刚刚恢复的服务造成突发流量冲击，可能导致服务再次崩溃。

### 5.2 抖动算法

抖动在计算出的基础间隔上叠加一个随机偏移量：

```
jitter_range   = base_interval × JitterFactor
random_offset  = random(-jitter_range, +jitter_range)
final_interval = base_interval + random_offset
```

以 `base_interval = 1000ms`、`JitterFactor = 0.1` 为例：
- `jitter_range = 1000ms × 0.1 = 100ms`
- 最终间隔在 `[900ms, 1100ms]` 区间内均匀分布

**添加抖动后的效果**:

```
时间轴:  0ms     900ms  1000ms  1100ms  1800ms  2000ms  2200ms
客户端A:  × ────────┴──────────────────────┴──────────────────►
客户端B:  × ────────────────┴───────────────────────┴─────────►
客户端C:  × ─────────────┴───────────────┴────────────────────►
                              ↑ 重试分散开，避免流量尖峰
```

### 5.3 JitterFactor 取值建议

| 场景 | 推荐值 | 说明 |
|------|--------|------|
| 低并发 | 0.05 ~ 0.1 | 小幅抖动即可 |
| 高并发 | 0.2 ~ 0.3 | 需要更大的分散度 |
| 极高并发（>1000客户端） | 0.5 | 强烈建议使用 Full Jitter |

---

## 6. 可重试错误判定机制

### 6.1 设计目标

并非所有错误都应该重试。对于不可恢复的错误（如权限不足、参数非法），重试只会浪费资源并延迟错误暴露。

### 6.2 默认判定逻辑

`DefaultIsRetryable(err)` 的行为:

| 错误类型 | 是否重试 | 说明 |
|---------|---------|------|
| `nil` | ✗ | 无错误无需重试 |
| `context.DeadlineExceeded` | ✓ | 超时通常是临时性的 |
| `context.Canceled` | ✗ | 主动取消不应重试 |
| 其他错误 | ✓ | 默认视为可重试 |

### 6.3 自定义判定示例

```go
var ErrPermissionDenied = errors.New("permission denied")
var ErrInvalidArgument  = errors.New("invalid argument")

func myIsRetryable(err error) bool {
    if errors.Is(err, ErrPermissionDenied) {
        return false
    }
    if errors.Is(err, ErrInvalidArgument) {
        return false
    }
    if errors.Is(err, context.Canceled) {
        return false
    }
    return true
}
```

### 6.4 常见可重试 vs 不可重试错误

| 可重试（临时性） | 不可重试（永久性） |
|----------------|------------------|
| 网络超时 | 权限不足 (401/403) |
| 连接被拒绝 | 参数非法 (400) |
| 服务暂时不可用 (503) | 资源不存在 (404) |
| 限流 (429 + Retry-After) | 数据格式错误 |
| 数据库死锁 | 业务规则冲突 |
| DNS 解析超时 | 证书验证失败 |

---

## 7. 重试回调钩子

### 7.1 执行时序

```
调用 Do(fn)
    │
    ▼
  ┌────────────────────────────────────────────┐
  │  for {                                     │
  │    1. 检查 Context 是否已取消              │
  │    2. 执行 fn(ctx)                         │
  │    3. 成功 → 返回 nil                      │
  │    4. 失败 → attempts++，记录错误          │
  │    5. 检查是否可重试 → 不可重试则返回      │
  │    6. 检查是否超过最大次数 → 超过则返回    │
  │    7. 执行 OnRetryBefore(attempt, err)  ◄──┼── 重试前回调
  │    8. 计算间隔，等待（可被 Context 中断）   │
  │    9. 执行 OnRetryAfter(attempt, err)   ◄──┼── 重试后回调
  │  }                                          │
  └────────────────────────────────────────────┘
```

### 7.2 回调异常安全

回调函数的 panic 会被 `recover()` 捕获，不会中断重试流程。这确保了即使日志/指标系统出现问题，核心重试逻辑仍然可靠。

### 7.3 典型用途

| 回调 | 典型用途 |
|------|---------|
| `OnRetryBefore` | 记录警告日志、上报重试指标、发送告警 |
| `OnRetryAfter` | 清理临时资源、刷新配置、同步状态 |

---

## 8. 使用示例

### 8.1 基本使用（使用 Retryer 实例）

```go
package main

import (
    "context"
    "fmt"
    "solocoder-go/internal/retry"
    "time"
)

func main() {
    r := retry.NewRetryerWithConfig(retry.Config{
        InitialInterval: 100 * time.Millisecond,
        MaxInterval:     5 * time.Second,
        MaxRetries:      5,
        JitterFactor:    0.2,
    })

    err := r.Do(context.Background(), func(ctx context.Context) error {
        return callRemoteAPI(ctx)
    })

    if err != nil {
        fmt.Printf("操作失败，共重试 %d 次\n", r.Attempts())
        var agg *retry.AggregateError
        if errors.As(err, &agg) {
            for i, e := range agg.Errors {
                fmt.Printf("  第 %d 次错误: %v\n", i+1, e)
            }
        }
    }
}
```

### 8.2 便捷函数式调用（Option 模式）

```go
import "solocoder-go/internal/retry"

err := retry.Do(context.Background(),
    func(ctx context.Context) error {
        return fetchData(ctx)
    },
    retry.WithInitialInterval(50*time.Millisecond),
    retry.WithMaxInterval(2*time.Second),
    retry.WithMaxRetries(3),
    retry.WithJitterFactor(0.15),
)
```

### 8.3 自定义可重试错误判定

```go
err := retry.Do(ctx,
    func(ctx context.Context) error {
        return db.Query(ctx)
    },
    retry.WithMaxRetries(5),
    retry.WithIsRetryable(func(err error) bool {
        if errors.Is(err, context.Canceled) {
            return false
        }
        if strings.Contains(err.Error(), "permission denied") {
            return false
        }
        return true
    }),
)
```

### 8.4 注册回调钩子

```go
var retryCount int32

r := retry.NewRetryerWithConfig(retry.Config{
    MaxRetries:      5,
    InitialInterval: 100 * time.Millisecond,
    JitterFactor:    0.1,
    OnRetryBefore: func(attempt int, err error) {
        atomic.AddInt32(&retryCount, 1)
        log.Printf("第 %d 次重试前，错误: %v", attempt, err)
    },
    OnRetryAfter: func(attempt int, err error) {
        log.Printf("第 %d 次重试等待结束，即将执行", attempt)
    },
})
```

### 8.5 结合 Context 超时

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := retry.Do(ctx,
    func(ctx context.Context) error {
        return longRunningOperation(ctx)
    },
    retry.WithMaxRetries(10),
    retry.WithMaxInterval(2*time.Second),
)
if errors.Is(err, context.DeadlineExceeded) {
    log.Println("整体操作超时")
}
```

### 8.6 检查聚合错误

```go
err := r.Do(ctx, operation)
if err != nil {
    var aggErr *retry.AggregateError
    if errors.As(err, &aggErr) {
        fmt.Printf("共 %d 次尝试失败\n", len(aggErr.Errors))
        
        allTimeout := true
        for _, e := range aggErr.Errors {
            if !errors.Is(e, context.DeadlineExceeded) {
                allTimeout = false
                break
            }
        }
        if allTimeout {
            log.Println("所有失败均由超时导致，可能需要增加超时时间")
        }
    }
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrMaxRetriesExceeded` | 超过最大重试次数 | 预留常量，实际通过 `AggregateError` 返回 |
| `ErrInvalidConfig` | 配置无效 | 预留常量，实际配置会自动归一化 |
| `ErrNonRetryable` | 不可重试错误 | 预留常量，实际通过 `AggregateError` 返回 |
| `*AggregateError` | 聚合错误 | 任意失败场景下返回，包含所有错误历史 |

**注意**: 调用方应通过 `errors.As(err, &aggErr)` 来判断是否为聚合错误，而非比较错误变量。

---

## 10. 配置说明

### 10.1 完整配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `InitialInterval` | `time.Duration` | 100ms | 首次重试前的等待间隔（正数） |
| `MaxInterval` | `time.Duration` | 10s | 重试间隔上限，不得小于 InitialInterval |
| `MaxRetries` | `int` | 3 | 最大重试次数（0 表示不重试，仅执行 1 次） |
| `JitterFactor` | `float64` | 0.1 | 抖动因子，范围 [0, 1]，表示 ±百分比 |
| `IsRetryable` | `IsRetryableFunc` | `DefaultIsRetryable` | 可重试错误判定函数 |
| `OnRetryBefore` | `OnRetryFunc` | `nil` | 重试等待前回调 |
| `OnRetryAfter` | `OnRetryFunc` | `nil` | 重试等待后回调 |

### 10.2 配置归一化规则

创建 `Retryer` 时，以下非法值会被自动修正：

| 非法值 | 修正为 |
|--------|--------|
| `InitialInterval <= 0` | 默认值 100ms |
| `MaxInterval <= 0` | 默认值 10s |
| `MaxInterval < InitialInterval` | `InitialInterval` |
| `MaxRetries < 0` | 0（不重试） |
| `JitterFactor < 0` | 0（无抖动） |
| `JitterFactor > 1` | 1.0（±100% 抖动） |
| `IsRetryable == nil` | `DefaultIsRetryable` |

---

## 11. 并发安全

| 维度 | 安全状态 | 说明 |
|------|---------|------|
| 单个 `Retryer` 实例并发调用 `Do()` | ⚠️ 不安全 | `Retryer` 内部维护 `attempts` 和 `errs` 状态，单个实例不应被多个 goroutine 共享 |
| 多个 goroutine 各自创建 `Retryer` | ✓ 安全 | 推荐每个 goroutine 使用独立的 `Retryer` 实例 |
| 便捷函数 `Do()` 并发调用 | ✓ 安全 | 每次调用内部创建独立 `Retryer`，完全并发安全 |

**推荐模式**:

```go
// 好：每个 goroutine 创建独立 Retryer
for i := 0; i < N; i++ {
    go func() {
        r := retry.NewRetryer()
        r.Do(ctx, operation)
    }()
}

// 好：直接使用并发安全的便捷函数
for i := 0; i < N; i++ {
    go func() {
        retry.Do(ctx, operation, retry.WithMaxRetries(3))
    }()
}

// 不好：共享同一个 Retryer
r := retry.NewRetryer()
for i := 0; i < N; i++ {
    go func() {
        r.Do(ctx, operation)  // 并发读写内部状态，有竞态
    }()
}
```

---

## 12. 最佳实践

### 12.1 参数调优建议

| 场景 | InitialInterval | MaxInterval | MaxRetries | JitterFactor |
|------|----------------|-------------|------------|-------------|
| 内存/本地操作 | 10ms | 200ms | 3 | 0.1 |
| 数据库查询 | 50ms | 2s | 5 | 0.2 |
| HTTP API 调用 | 100ms | 10s | 5 | 0.2 |
| 外部服务（第三方） | 500ms | 30s | 8 | 0.3 |
| 消息队列消费 | 1s | 60s | 10 | 0.3 |

### 12.2 防御性编程

1. **Context 必传**: 始终传递带有超时的 `context.Context`，防止无限等待
2. **幂等性保证**: 被重试的操作必须具备幂等性（重复执行不会产生副作用）
3. **快速失败**: 通过 `IsRetryable` 对永久性错误立即中止，不浪费重试次数
4. **监控告警**: 在 `OnRetryBefore` 中上报指标，监控重试率异常升高

### 12.3 典型反模式

❌ **反模式 1: 重试非幂等操作**
```go
// 危险：转账操作重试可能导致多次扣款
retry.Do(ctx, func(ctx context.Context) error {
    return transferMoney(from, to, amount)  // 需要服务端支持幂等键
})
```

✅ **修正**: 使用幂等键（Idempotency Key）让服务端去重。

---

❌ **反模式 2: 无限重试**
```go
// 危险：可能永久卡住
retry.Do(ctx, func(ctx context.Context) error {
    return brokenOperation(ctx)  // 如果永远失败...
}, retry.WithMaxRetries(math.MaxInt32))
```

✅ **修正**: 设置合理的最大重试次数，并配合 Context 超时。

---

❌ **反模式 3: 重试吞没所有错误**
```go
// 危险：永久性错误被掩盖，问题迟迟不暴露
retry.Do(ctx, func(ctx context.Context) error {
    return parseBrokenData()  // 参数错误，重试也没用
}, retry.WithIsRetryable(func(err error) bool { return true }))
```

✅ **修正**: 实现精准的 `IsRetryable` 判定函数。
