# healthagg - 健康检查聚合器模块

## 模块功能

`healthagg` 提供了一套多维度健康检查聚合机制，支持以下核心功能：

1. **多维度健康探针注册**：提供探针注册接口，支持注册多个不同维度的健康探针，例如数据库连接状态、缓存可用性、第三方服务连通性等。每个探针是一个可执行健康检查的函数，返回健康状态和可选的状态详情。

2. **探针结果聚合**：对所有已注册探针的检查结果进行聚合，根据各探针的状态综合得出系统的整体健康状态。聚合策略支持两种模式：
   - **全部健康才算健康**：所有探针都健康时系统才健康
   - **基于权重的多数健康**：权重健康占比超过阈值时系统健康

3. **健康状态分级**：将健康状态分为健康、降级和不健康三个级别：
   - **健康**：所有探针正常运行
   - **降级**：非关键探针失败但核心功能正常
   - **不健康**：关键探针失败，系统不可用
   - 提供各状态的详细探针结果列表

4. **状态变更告警回调**：当整体健康状态发生变化时触发告警回调。调用方可注册告警处理函数接收状态变更事件，事件包含变更前后的状态和具体失败的探针列表。

## 核心结构体

### HealthStatus

健康状态枚举，定义了系统的三级健康状态。

| 常量 | 值 | 说明 |
|------|----|------|
| `StatusHealthy` | 0 | 健康状态，所有探针正常 |
| `StatusDegraded` | 1 | 降级状态，非关键探针失败 |
| `StatusUnhealthy` | 2 | 不健康状态，关键探针失败 |

方法：

| 方法 | 说明 |
|------|------|
| `String() string` | 返回状态的字符串表示 |

### AggregationStrategy

聚合策略枚举。

| 常量 | 值 | 说明 |
|------|----|------|
| `StrategyAllHealthy` | 0 | 全部健康策略，所有探针健康才算健康 |
| `StrategyWeightedMajority` | 1 | 权重多数策略，健康权重占比超过阈值才算健康 |

### ProbeResult

单个探针的检查结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | string | 探针名称 |
| `Healthy` | bool | 是否健康 |
| `Details` | string | 状态详情（可选） |

### ProbeFunc

探针函数类型，执行健康检查并返回结果。

```go
type ProbeFunc func() ProbeResult
```

### ProbeConfig

探针配置，用于注册探针。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Name` | string | - | 探针唯一名称 |
| `Probe` | ProbeFunc | - | 探针检查函数 |
| `Critical` | bool | false | 是否为关键探针，关键探针失败会导致系统不健康 |
| `Weight` | int | 1 | 探针权重，用于加权多数策略，必须大于 0 |

### AggregatedHealth

聚合后的健康检查结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Status` | HealthStatus | 整体健康状态 |
| `ProbeResults` | []ProbeResult | 所有探针的详细结果列表 |
| `FailedProbes` | []string | 失败探针的名称列表 |
| `HealthyCount` | int | 健康探针数量 |
| `TotalCount` | int | 探针总数 |

### StatusChangeEvent

状态变更事件，在健康状态发生变化时传递给告警回调。

| 字段 | 类型 | 说明 |
|------|------|------|
| `PreviousStatus` | HealthStatus | 变更前的状态 |
| `CurrentStatus` | HealthStatus | 变更后的状态 |
| `FailedProbes` | []string | 当前失败的探针列表 |
| `Timestamp` | int64 | 时间戳（预留字段） |

### AlertCallback

告警回调函数类型。

```go
type AlertCallback func(event StatusChangeEvent)
```

### AggregatorConfig

健康检查聚合器配置。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Strategy` | AggregationStrategy | `StrategyAllHealthy` | 聚合策略 |
| `MajorityRatio` | float64 | 0.5 | 权重多数策略的阈值（0 < ratio <= 1） |

### HealthAggregator

健康检查聚合器，管理所有探针的注册、执行、聚合和告警。

主要方法：

| 方法 | 说明 |
|------|------|
| `NewHealthAggregator(cfg AggregatorConfig) (*HealthAggregator, error)` | 创建新的健康检查聚合器 |
| `DefaultAggregatorConfig() AggregatorConfig` | 返回默认配置 |
| `RegisterProbe(cfg ProbeConfig) error` | 注册一个健康探针 |
| `UnregisterProbe(name string) error` | 注销一个健康探针 |
| `GetProbe(name string) (*ProbeConfig, error)` | 获取探针配置（返回副本） |
| `ProbeCount() int` | 获取已注册探针数量 |
| `Check() AggregatedHealth` | 执行所有探针检查并返回聚合结果 |
| `SubscribeAlert(callback AlertCallback) (string, error)` | 订阅状态变更告警 |
| `UnsubscribeAlert(id string) error` | 取消订阅告警 |
| `AlertCallbackCount() int` | 获取告警回调数量 |
| `LastStatus() HealthStatus` | 获取上次检查的健康状态 |
| `Start()` | 启动聚合器（幂等） |
| `Stop()` | 停止聚合器（幂等） |
| `IsRunning() bool` | 检查聚合器是否在运行 |

## 三级健康状态判定逻辑

### StrategyAllHealthy（全部健康策略）

该策略下，只要有探针失败就会影响系统状态，但区分关键和非关键探针：

1. **健康 (StatusHealthy)**：所有探针都健康
2. **降级 (StatusDegraded)**：存在失败探针，但所有失败的探针都是非关键探针
3. **不健康 (StatusUnhealthy)**：至少有一个关键探针失败

### StrategyWeightedMajority（权重多数策略）

该策略基于权重计算健康占比，超过阈值才算健康：

1. 计算总权重 = 所有探针权重之和
2. 计算健康权重 = 所有健康探针权重之和
3. 计算健康占比 = 健康权重 / 总权重
4. 计算关键探针的健康占比

判定规则：

- **不健康 (StatusUnhealthy)**：关键探针的健康占比 <= majorityRatio
- **降级 (StatusDegraded)**：关键探针健康占比 > majorityRatio，但整体健康占比 <= majorityRatio
- **健康 (StatusHealthy)**：整体健康占比 > majorityRatio（且关键探针健康占比 > majorityRatio）

如果没有探针，默认状态为健康。

## 告警回调机制

### 触发条件

只有当整体健康状态发生变化时才会触发告警回调。相同状态的连续检查不会重复触发告警。

### 状态变更场景

支持所有状态间的转换：
- 健康 → 降级
- 健康 → 不健康
- 降级 → 健康
- 降级 → 不健康
- 不健康 → 健康
- 不健康 → 降级

### 事件内容

状态变更事件包含：
- `PreviousStatus`：变更前的状态
- `CurrentStatus`：变更后的状态
- `FailedProbes`：当前失败的探针列表

### 回调执行

所有已注册的告警回调都会被调用，回调在锁外执行，避免阻塞聚合器的正常操作。

## 并发安全设计

`HealthAggregator` 使用 `sync.RWMutex` 保证并发安全：

- 读操作（GetProbe、ProbeCount、LastStatus、IsRunning 等）使用读锁
- 写操作（RegisterProbe、UnregisterProbe、SubscribeAlert 等）使用写锁
- Check 方法先在持锁状态下获取探针列表快照，然后在锁外执行探针检查，避免长时间持锁
- 告警回调在锁外执行，防止回调函数阻塞聚合器

## 使用示例

### 创建健康检查聚合器

```go
// 使用默认配置（全部健康策略）
cfg := healthagg.DefaultAggregatorConfig()
ha, err := healthagg.NewHealthAggregator(cfg)
if err != nil {
    log.Fatal(err)
}
defer ha.Stop()
```

### 使用权重多数策略

```go
cfg := healthagg.AggregatorConfig{
    Strategy:      healthagg.StrategyWeightedMajority,
    MajorityRatio: 0.6, // 60% 以上权重健康才算健康
}
ha, err := healthagg.NewHealthAggregator(cfg)
```

### 注册健康探针

```go
// 注册关键探针：数据库连接
ha.RegisterProbe(healthagg.ProbeConfig{
    Name:     "database",
    Critical: true,
    Weight:   5,
    Probe: func() healthagg.ProbeResult {
        err := db.Ping()
        if err != nil {
            return healthagg.ProbeResult{
                Healthy: false,
                Details: err.Error(),
            }
        }
        return healthagg.ProbeResult{
            Healthy: true,
            Details: "connected",
        }
    },
})

// 注册非关键探针：监控服务
ha.RegisterProbe(healthagg.ProbeConfig{
    Name:     "monitoring",
    Critical: false,
    Weight:   1,
    Probe: func() healthagg.ProbeResult {
        resp, err := http.Get("http://monitoring/health")
        if err != nil || resp.StatusCode != 200 {
            return healthagg.ProbeResult{Healthy: false}
        }
        return healthagg.ProbeResult{Healthy: true}
    },
})
```

### 执行健康检查

```go
result := ha.Check()

fmt.Printf("系统状态: %s\n", result.Status)
fmt.Printf("健康探针: %d/%d\n", result.HealthyCount, result.TotalCount)

if len(result.FailedProbes) > 0 {
    fmt.Printf("失败探针: %v\n", result.FailedProbes)
}

// 查看每个探针的详细结果
for _, pr := range result.ProbeResults {
    fmt.Printf("  - %s: %s (%s)\n", pr.Name,
        map[bool]string{true: "健康", false: "失败"}[pr.Healthy],
        pr.Details)
}
```

### 订阅状态变更告警

```go
alertID, err := ha.SubscribeAlert(func(event healthagg.StatusChangeEvent) {
    log.Printf("健康状态变更: %s → %s",
        event.PreviousStatus, event.CurrentStatus)
    
    if len(event.FailedProbes) > 0 {
        log.Printf("失败探针: %v", event.FailedProbes)
    }
    
    // 根据状态级别执行不同的告警动作
    switch event.CurrentStatus {
    case healthagg.StatusUnhealthy:
        // 发送严重告警（短信、电话等）
        sendCriticalAlert(event)
    case healthagg.StatusDegraded:
        // 发送一般告警（邮件、钉钉等）
        sendWarningAlert(event)
    case healthagg.StatusHealthy:
        // 发送恢复通知
        sendRecoveryNotice(event)
    }
})
defer ha.UnsubscribeAlert(alertID)
```

### 管理探针

```go
// 获取探针数量
count := ha.ProbeCount()

// 获取探针配置
probe, err := ha.GetProbe("database")
if err == nil {
    fmt.Printf("探针 %s: 关键=%v, 权重=%d\n",
        probe.Name, probe.Critical, probe.Weight)
}

// 注销探针
err := ha.UnregisterProbe("monitoring")
```

### 完整示例

```go
package main

import (
    "fmt"
    "log"
    "time"
    "solocoder-go/internal/healthagg"
)

func main() {
    cfg := healthagg.AggregatorConfig{
        Strategy:      healthagg.StrategyAllHealthy,
        MajorityRatio: 0.5,
    }
    ha, err := healthagg.NewHealthAggregator(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer ha.Stop()

    // 注册告警回调
    ha.SubscribeAlert(func(event healthagg.StatusChangeEvent) {
        log.Printf("[ALERT] 状态变更: %s -> %s, 失败探针: %v",
            event.PreviousStatus, event.CurrentStatus, event.FailedProbes)
    })

    // 注册探针
    ha.RegisterProbe(healthagg.ProbeConfig{
        Name:     "db",
        Critical: true,
        Weight:   3,
        Probe: func() healthagg.ProbeResult {
            return healthagg.ProbeResult{Healthy: true, Details: "ok"}
        },
    })

    ha.RegisterProbe(healthagg.ProbeConfig{
        Name:     "cache",
        Critical: false,
        Weight:   1,
        Probe: func() healthagg.ProbeResult {
            return healthagg.ProbeResult{Healthy: true, Details: "ok"}
        },
    })

    // 第一次检查：健康
    result := ha.Check()
    fmt.Printf("状态: %s\n", result.Status) // healthy

    // 模拟缓存失败
    ha.UnregisterProbe("cache")
    ha.RegisterProbe(healthagg.ProbeConfig{
        Name:     "cache",
        Critical: false,
        Probe: func() healthagg.ProbeResult {
            return healthagg.ProbeResult{Healthy: false, Details: "timeout"}
        },
    })

    // 第二次检查：降级（非关键探针失败）
    result = ha.Check()
    fmt.Printf("状态: %s\n", result.Status) // degraded
    fmt.Printf("失败探针: %v\n", result.FailedProbes) // [cache]

    // 模拟数据库失败
    ha.UnregisterProbe("db")
    ha.RegisterProbe(healthagg.ProbeConfig{
        Name:     "db",
        Critical: true,
        Probe: func() healthagg.ProbeResult {
            return healthagg.ProbeResult{Healthy: false, Details: "connection lost"}
        },
    })

    // 第三次检查：不健康（关键探针失败）
    result = ha.Check()
    fmt.Printf("状态: %s\n", result.Status) // unhealthy
}
```
