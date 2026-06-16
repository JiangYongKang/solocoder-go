# 降级策略链模块 (Fallback Chain)

## 1. 模块概述

降级策略链模块提供了一个灵活、可配置的多级降级方案管理系统。当主业务逻辑出现故障时，系统能够自动切换到预先配置的降级方案，确保服务的可用性和稳定性。

该模块支持：
- 多级降级方案按优先级排列
- 主策略失败后自动切换到下一级方案
- 可配置的降级触发条件（超时、特定错误类型、错误率阈值等）
- 系统恢复后自动切回主策略（支持主动探测和被动检测两种模式）
- 平滑的预热期机制，确保主策略稳定后再完全切换

## 2. 核心结构体职责

### 2.1 Chain

`Chain` 是降级策略链的核心管理器，负责：
- 管理所有注册的降级策略
- 维护策略的优先级顺序
- 协调降级切换和恢复流程
- 收集执行指标和统计数据
- 管理恢复检测循环

```go
type Chain struct {
    mu               sync.RWMutex
    strategies       []*Strategy
    strategyMap      map[string]*Strategy
    currentIndex     int
    state            ChainState
    metrics          ChainMetrics
    recoveryCfg      RecoveryConfig
    stopCh           chan struct{}
    wg               sync.WaitGroup
    running          bool
    activeStrategyID string
}
```

### 2.2 Strategy

`Strategy` 代表一个具体的降级策略，包含：
- 策略的基本信息（ID、名称、优先级）
- 处理函数
- 触发条件配置
- 执行状态和统计数据
- 错误时间窗口记录

```go
type Strategy struct {
    ID              string
    Name            string
    Priority        int
    Handler         HandlerFunc
    TriggerCond     *TriggerCondition
    State           StrategyState
    LastUsedAt      time.Time
    SuccessCount    uint64
    FailureCount    uint64
    ConsecutiveFail uint64
    ErrorWindow     []errorEntry
    mu              sync.RWMutex
}
```

### 2.3 TriggerCondition

`TriggerCondition` 定义了降级触发的条件，支持多种类型：

| 类型 | 说明 | 配置项 |
|------|------|--------|
| `TriggerConditionTimeout` | 执行超时触发 | `Timeout` - 超时时间 |
| `TriggerConditionErrorType` | 特定错误类型触发 | `ErrorTypes` - 目标错误列表 |
| `TriggerConditionErrorRate` | 错误率超过阈值触发 | `ErrorRate` - 错误率阈值, `ErrorWindow` - 统计窗口 |
| `TriggerConditionCustom` | 自定义判断逻辑 | `CustomCheck` - 自定义判断函数 |

### 2.4 RecoveryConfig

`RecoveryConfig` 配置恢复检测行为：

| 字段 | 说明 |
|------|------|
| `Mode` | 恢复模式：主动探测或被动检测 |
| `CheckInterval` | 主动探测的检查间隔 |
| `ProbeSuccessThreshold` | 主动探测成功次数阈值 |
| `ProbeFailureThreshold` | 主动探测失败次数阈值 |
| `WarmUpDuration` | 预热期持续时间 |
| `PassiveSuccessWindow` | 被动检测的成功统计窗口 |
| `PassiveSuccessCount` | 被动检测的成功次数阈值 |

### 2.5 ChainMetrics

`ChainMetrics` 记录执行统计指标：

```go
type ChainMetrics struct {
    TotalExecutions    uint64
    TotalSuccesses     uint64
    TotalFailures      uint64
    FallbackCount      uint64
    RecoveryCount      uint64
    AvgResponseTime    time.Duration
}
```

## 3. 降级策略链状态流转

### 3.1 ChainState 状态

| 状态 | 说明 |
|------|------|
| `ChainStateHealthy` | 健康状态，使用主策略 |
| `ChainStateDegraded` | 降级状态，使用降级策略 |
| `ChainStateRecovering` | 恢复中，正在验证主策略可用性 |

### 3.2 StrategyState 状态

| 状态 | 说明 |
|------|------|
| `StrategyStateActive` | 活跃状态，可被调用 |
| `StrategyStateDegraded` | 降级状态，出现过故障 |
| `StrategyStateRecovering` | 恢复中，正在进行探测 |
| `StrategyStateWarmingUp` | 预热中，验证稳定性 |

### 3.3 状态流转图

```
                        所有策略失败
┌─────────────────┐  ┌─────────────────┐
│ ChainState      │  │ StrategyState   │
└─────────────────┘  └─────────────────┘
        │                    │
        ▼                    ▼
┌─────────────────┐  ┌─────────────────┐
│    HEALTHY      │  │     ACTIVE      │◄─┐
└─────────┬───────┘  └────────┬────────┘  │
          │                   │           │
          │ 主策略失败        │ 失败      │
          ▼                   ▼           │
┌─────────────────┐  ┌─────────────────┐  │
│   DEGRADED      │  │   DEGRADED      │  │
└─────────┬───────┘  └────────┬────────┘  │
          │                   │           │
          │ 开始恢复          │ 开始探测  │
          ▼                   ▼           │
┌─────────────────┐  ┌─────────────────┐  │
│  RECOVERING     │──│  RECOVERING     │  │
└─────────────────┘  └────────┬────────┘  │
                              │           │
                              │ 探测成功  │
                              ▼           │
                     ┌─────────────────┐  │
                     │   WARMING_UP    │  │
                     └────────┬────────┘  │
                              │           │
                              │ 预热完成  │
                              ▼           │
                     ┌─────────────────┐  │
                     │     ACTIVE      │──┘
                     └─────────────────┘
```

### 3.4 执行流程

```
调用 Execute()
    │
    ▼
获取当前策略列表和起始索引
    │
    ▼
┌─────────────────────────────────────────┐
│  循环执行策略（从当前索引开始）         │
│  ┌───────────────────────────────────┐  │
│  │  1. 检查触发条件，支持快速跳转     │  │
│  ├───────────────────────────────────┤  │
│  │  2. 执行策略（带超时保护）        │  │
│  ├───────────────────────────────────┤  │
│  │  3. 更新策略统计数据              │  │
│  ├───────────────────────────────────┤  │
│  │  4. 成功：更新活动策略，检查恢复  │──┼──┐
│  ├───────────────────────────────────┤  │  │
│  │  5. 失败：标记降级，记录错误      │  │  │
│  └───────────────────────────────────┘  │  │
└───────────────────┬─────────────────────┘  │
                    │                        │
                    │ 所有策略失败           │ 成功
                    ▼                        ▼
          返回 AggregateError          返回执行结果
                    │
                    ▼
          更新 Chain 状态为 DEGRADED
```

### 3.5 恢复流程

#### 主动探测模式 (RecoveryModeActive)

```
启动后台探测 goroutine
    │
    ▼
定期检查：是否使用降级策略？
    │
    ├─ 否：跳过本次检查
    │
    └─ 是：启动探测流程
          │
          ▼
    将主策略标记为 RECOVERING
          │
          ▼
    循环执行探测请求
          │
          ├─ 连续成功达到阈值 → 进入预热期
          │
          └─ 失败达到阈值 → 标记为 DEGRADED，退出
                │
                ▼
          预热期验证
                │
                ├─ 预热成功 → 切换回主策略
                │
                └─ 预热失败 → 保持降级状态
```

#### 被动检测模式 (RecoveryModePassive)

```
每次成功执行降级策略后
    │
    ▼
检查主策略的连续成功次数
    │
    ├─ 未达到阈值 → 继续使用降级策略
    │
    └─ 达到阈值 → 启动恢复流程
          │
          ▼
    进入预热期验证
          │
          ├─ 预热成功 → 切换回主策略
          │
          └─ 预热失败 → 保持降级状态
```

## 4. 使用示例

### 4.1 基础使用

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "solocoder-go/internal/fallback"
)

func main() {
    // 创建降级链
    chain := fallback.NewChain(nil)

    // 注册主策略
    chain.RegisterStrategy("main", "主数据库", 0,
        func(ctx context.Context) (interface{}, error) {
            return queryFromPrimaryDB(ctx)
        },
        nil, // 无特殊触发条件
    )

    // 注册降级策略1：只读副本
    chain.RegisterStrategy("replica", "只读副本", 1,
        func(ctx context.Context) (interface{}, error) {
            return queryFromReplica(ctx)
        },
        nil,
    )

    // 注册降级策略2：缓存
    cacheCond := &fallback.TriggerCondition{
        Type:       fallback.TriggerConditionErrorType,
        ErrorTypes: []error{errors.New("db connection failed")},
    }
    chain.RegisterStrategy("cache", "缓存查询", 2,
        func(ctx context.Context) (interface{}, error) {
            return queryFromCache(ctx)
        },
        cacheCond,
    )

    // 启动降级链
    ctx := context.Background()
    if err := chain.Start(ctx); err != nil {
        panic(err)
    }
    defer chain.Stop()

    // 执行业务逻辑
    result, err := chain.Execute(ctx)
    if err != nil {
        fmt.Printf("执行失败: %v\n", err)
        return
    }
    fmt.Printf("执行成功: %v\n", result)
}
```

### 4.2 配置超时触发

```go
timeoutCond := &fallback.TriggerCondition{
    Type:    fallback.TriggerConditionTimeout,
    Timeout: 500 * time.Millisecond,
}

chain.RegisterStrategy("slow_api", "慢接口", 0,
    func(ctx context.Context) (interface{}, error) {
        return callSlowAPI(ctx)
    },
    timeoutCond,
)
```

### 4.3 配置自定义触发条件

```go
customCond := &fallback.TriggerCondition{
    Type: fallback.TriggerConditionCustom,
    CustomCheck: func(err error) bool {
        if err == nil {
            return false
        }
        // 限流错误直接降级
        return strings.Contains(err.Error(), "rate limit")
    },
}

chain.RegisterStrategy("api", "外部API", 0, handler, customCond)
```

### 4.4 主动探测恢复配置

```go
cfg := &fallback.ChainConfig{
    Recovery: fallback.RecoveryConfig{
        Mode:                  fallback.RecoveryModeActive,
        CheckInterval:         5 * time.Second,
        ProbeSuccessThreshold: 3,
        ProbeFailureThreshold: 1,
        WarmUpDuration:        10 * time.Second,
    },
}

chain := fallback.NewChain(cfg)
```

### 4.5 被动检测恢复配置

```go
cfg := &fallback.ChainConfig{
    Recovery: fallback.RecoveryConfig{
        Mode:                  fallback.RecoveryModePassive,
        PassiveSuccessWindow:  30 * time.Second,
        PassiveSuccessCount:   5,
        WarmUpDuration:        5 * time.Second,
    },
}

chain := fallback.NewChain(cfg)
```

### 4.6 获取统计信息

```go
// 获取链状态
state := chain.State()
fmt.Printf("当前状态: %s\n", state)

// 获取当前活动策略
activeID := chain.CurrentStrategyID()
fmt.Printf("当前活动策略: %s\n", activeID)

// 获取指标
metrics := chain.Metrics()
fmt.Printf("总执行次数: %d\n", metrics.TotalExecutions)
fmt.Printf("降级次数: %d\n", metrics.FallbackCount)
fmt.Printf("恢复次数: %d\n", metrics.RecoveryCount)

// 获取策略统计
if strategy, ok := chain.GetStrategy("main"); ok {
    success, failures, consecFail := strategy.Stats()
    fmt.Printf("主策略: 成功=%d, 失败=%d, 连续