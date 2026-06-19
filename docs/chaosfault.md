# 混沌工程故障注入器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [延迟注入原理](#4-延迟注入原理)
5. [错误注入原理](#5-错误注入原理)
6. [连接断开模拟原理](#6-连接断开模拟原理)
7. [时间窗口与目标比例控制](#7-时间窗口与目标比例控制)
8. [并发安全策略](#8-并发安全策略)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [配置说明](#11-配置说明)
12. [最佳实践](#12-最佳实践)

---

## 1. 模块概述

混沌工程故障注入器（Chaos Fault Injector）是一个用于在系统中主动注入故障以验证系统容错能力的通用模块。通过模拟延迟、错误和连接断开等常见故障场景，可以帮助开发团队提前发现系统中的脆弱点，提高系统的可靠性和弹性。

**包路径**: `internal/chaosfault`

**设计目标**:
- 提供标准化的故障注入能力，支持延迟、错误、连接断开三种故障模式
- 支持时间窗口控制，故障仅在指定时间段内生效
- 延迟和错误注入支持目标比例控制，可精确控制受故障影响的请求百分比
- 连接断开采用持久断开语义，一旦断开所有后续请求持续失败直到恢复
- 提供便捷的启用/禁用接口，支持动态调整故障注入策略
- 完全并发安全，所有共享状态访问统一在 `sync.RWMutex` 保护下进行
- 支持自定义随机源、睡眠函数和时间函数，便于单元测试

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 延迟注入 | 向目标函数调用注入人工延迟，支持固定时长和随机范围两种模式，按比例触发 |
| 错误注入 | 向目标函数调用注入预定义错误，按概率触发，命中时直接返回错误不执行原逻辑 |
| 连接断开模拟 | 模拟目标服务连接断开，采用持久断开语义，断开后所有请求持续失败直到恢复 |
| 时间窗口控制 | 所有故障注入支持设置生效的时间窗口，仅在窗口内故障生效 |
| 目标比例控制 | 延迟和错误注入支持配置目标比例，控制受影响请求的百分比 |
| 综合注入入口 | 提供 `Inject()` 方法，按优先级依次应用断开、错误、延迟故障 |
| 配置化管理 | 支持完整的配置结构和便捷的启用/禁用辅助方法 |

---

## 3. 核心结构体与职责

### 3.1 FaultInjector

故障注入器主结构体，封装所有故障注入逻辑与状态。

```go
type FaultInjector struct {
    mu            sync.RWMutex
    delayCfg      DelayConfig
    errorCfg      ErrorConfig
    disconnectCfg DisconnectConfig
    disconnected  bool
    randSrc       *rand.Rand
    sleepFunc     func(time.Duration)
    timeNowFunc   func() time.Time
}
```

**职责**:
- 管理延迟、错误、连接断开三种故障的配置
- 提供 `ApplyDelay()`、`CheckError()`、`CheckDisconnect()` 等单一故障检查方法
- 提供 `Inject()` 综合注入入口，按优先级依次应用故障
- 提供 `Disconnect()` / `Reconnect()` 手动控制连接状态
- 维护内部随机源、睡眠函数和时间函数，支持测试注入
- 提供配置的获取、设置、重置等管理接口

### 3.2 DelayConfig

延迟注入配置结构体。

```go
type DelayConfig struct {
    Enabled     bool
    Mode        DelayMode
    Fixed       time.Duration
    Min         time.Duration
    Max         time.Duration
    TimeWindow  *TimeWindow
    TargetRatio float64
}
```

**字段说明**:
- `Enabled`: 是否启用延迟注入
- `Mode`: 延迟模式，可选 `DelayModeFixed`（固定延迟）或 `DelayModeRandom`（随机延迟）
- `Fixed`: 固定延迟时长（固定模式下使用）
- `Min`: 随机延迟最小值（随机模式下使用）
- `Max`: 随机延迟最大值（随机模式下使用）
- `TimeWindow`: 生效时间窗口，nil 表示始终生效
- `TargetRatio`: 目标比例，范围 [0, 1.0]，表示受影响请求的百分比

### 3.3 ErrorConfig

错误注入配置结构体。

```go
type ErrorConfig struct {
    Enabled     bool
    Err         error
    Message     string
    TimeWindow  *TimeWindow
    TargetRatio float64
}
```

**字段说明**:
- `Enabled`: 是否启用错误注入
- `Err`: 要注入的原始错误，可通过 `errors.Is` 检查
- `Message`: 错误消息文本
- `TimeWindow`: 生效时间窗口
- `TargetRatio`: 目标比例

### 3.4 DisconnectConfig

连接断开配置结构体。

```go
type DisconnectConfig struct {
    Enabled    bool
    TimeWindow *TimeWindow
}
```

**字段说明**:
- `Enabled`: 是否启用连接断开模拟
- `TimeWindow`: 生效时间窗口

**注意**: `DisconnectConfig` 不包含 `TargetRatio` 字段。连接断开采用持久断开语义，与延迟/错误注入的逐请求概率模型有本质区别，详见[第6节](#6-连接断开模拟原理)。

### 3.5 TimeWindow

时间窗口结构体，用于控制故障的生效时间范围。

```go
type TimeWindow struct {
    StartTime time.Time
    EndTime   time.Time
}
```

**字段说明**:
- `StartTime`: 窗口开始时间，零值表示无开始时间限制
- `EndTime`: 窗口结束时间，零值表示无结束时间限制

### 3.6 InjectedError

注入错误类型，封装注入的错误信息。

```go
type InjectedError struct {
    Message string
    Cause   error
}
```

**职责**:
- 标识这是一个注入的错误，而非真实业务错误
- 支持 `errors.Is` 和 `errors.As` 链式检查
- 保留原始错误原因（Cause）

### 3.7 ConnectionBrokenError

连接断开错误类型。

```go
type ConnectionBrokenError struct {
    Message string
}
```

**职责**:
- 标识连接断开故障
- 支持 `errors.Is(err, ErrConnectionBroken)` 判断
- 可自定义错误消息

---

## 4. 延迟注入原理

### 4.1 工作机制

延迟注入在目标函数调用执行前生效，通过阻塞当前 goroutine 一段时间来模拟网络延迟或处理延迟。延迟注入不会影响其他不相关的调用，每个调用独立判断是否需要注入延迟。

**执行流程**:
```
调用 ApplyDelay()
    │
    ├─ 获取读锁，读取配置
    ├─ 检查是否启用 → 未启用则释放锁并返回
    ├─ 检查时间窗口 → 不在窗口内则释放锁并返回
    ├─ 检查目标比例 → 未命中则释放锁并返回
    ├─ 计算延迟时长（在锁内完成随机数生成）
    ├─ 释放读锁
    └─ 调用 sleepFunc 阻塞等待（不持锁）
```

### 4.2 固定延迟模式

固定延迟模式（`DelayModeFixed`）下，每次命中延迟注入都会等待完全相同的时长。适用于模拟稳定的网络延迟场景。

```go
cfg := DelayConfig{
    Enabled:     true,
    Mode:        DelayModeFixed,
    Fixed:       100 * time.Millisecond,
    TargetRatio: 1.0,
}
```

### 4.3 随机延迟模式

随机延迟模式（`DelayModeRandom`）下，每次命中延迟注入会在 [Min, Max) 区间内随机选择一个延迟时长。适用于模拟真实网络中延迟波动的场景。

```go
cfg := DelayConfig{
    Enabled:     true,
    Mode:        DelayModeRandom,
    Min:         50 * time.Millisecond,
    Max:         200 * time.Millisecond,
    TargetRatio: 1.0,
}
```

随机延迟计算公式:
```
delay = Min + rand(0, Max - Min)
```

---

## 5. 错误注入原理

### 5.1 工作机制

错误注入用于模拟业务处理失败的场景。当错误注入命中时，被注入的函数直接返回指定错误，而不执行原有业务逻辑。错误注入支持按概率触发，只有部分请求会受到影响。

**执行流程**:
```
调用 CheckError()
    │
    ├─ 获取读锁，读取配置
    ├─ 检查是否启用 → 未启用则释放锁并返回 nil
    ├─ 检查时间窗口 → 不在窗口内则释放锁并返回 nil
    ├─ 检查目标比例（在锁内完成随机数生成）
    ├─ 释放读锁
    └─ 命中则返回 InjectedError（在锁外构造错误对象）
```

### 5.2 错误类型

注入的错误类型为 `*InjectedError`，它包含:
- `Message`: 自定义错误消息文本
- `Cause`: 原始错误原因（可选）

通过 `errors.Is(err, originalErr)` 可以判断注入的错误是否包装了某个特定错误，便于调用方根据错误类型进行处理。

### 5.3 错误注入优先级

在综合注入入口 `Inject()` 中，错误注入的优先级低于连接断开，但高于延迟注入。也就是说，如果同时启用了连接断开和错误注入，连接断开会先生效；如果只有错误注入命中，则函数不会执行，直接返回注入的错误。

---

## 6. 连接断开模拟原理

### 6.1 持久断开语义

连接断开模拟采用**持久断开**语义：一旦连接进入断开状态，所有后续请求都会立即返回连接中断错误，直到连接被显式恢复。这与延迟/错误注入的逐请求概率模型有本质区别：

| 维度 | 延迟/错误注入 | 连接断开模拟 |
|------|-------------|------------|
| 触发方式 | 每次请求独立概率判定 | 状态切换（连通 ↔ 断开） |
| 影响范围 | 仅命中比例的请求受影响 | 断开后所有请求持续受影响 |
| 恢复方式 | 无需恢复，每次请求重新判定 | 需要显式恢复操作 |
| TargetRatio | 适用 | 不适用 |

### 6.2 两种控制方式

#### 方式一：手动控制

通过 `Disconnect()` 和 `Reconnect()` 方法手动控制连接状态。适用于测试场景下精确控制故障的开始和结束时间。

```go
fi.Disconnect()   // 手动断开连接
// ... 所有请求都会失败 ...
fi.Reconnect()    // 恢复连接（仅清除手动断开标志）
```

**重要**: `Reconnect()` 仅清除手动断开标志（`disconnected = false`）。如果 `DisconnectConfig.Enabled` 仍为 true 且在时间窗口内，连接仍会处于断开状态。要完全恢复连接，需要同时调用 `DisableDisconnect()` 或等待时间窗口结束。

#### 方式二：配置化控制

通过 `DisconnectConfig` 配置连接断开，结合时间窗口使用。当配置启用且处于时间窗口内时，连接持续处于断开状态。

```go
cfg := DisconnectConfig{
    Enabled: true,
    TimeWindow: &TimeWindow{
        StartTime: start,
        EndTime:   end,
    },
}
fi.SetDisconnectConfig(cfg)
```

配置化断开的特点:
- **无 TargetRatio**: 断开是持久状态，不适用逐请求概率判定
- **时间窗口内持续断开**: 只要配置启用且在窗口内，所有请求都会失败
- **恢复方式**: 调用 `DisableDisconnect()` 或等待时间窗口结束

### 6.3 断开状态判定逻辑

```
isDisconnectedLocked()
    │
    ├─ 检查手动断开标志 → 已断开则返回 true
    ├─ 检查配置是否启用 → 未启用则返回 false
    ├─ 检查时间窗口 → 不在窗口内则返回 false
    └─ 在窗口内 → 返回 true（持久断开）
```

### 6.4 优先级

连接断开放在综合注入的最前面检查，具有最高优先级。只要连接处于断开状态，不管其他故障是否启用，都会立即返回连接断开错误。

---

## 7. 时间窗口与目标比例控制

### 7.1 时间窗口机制

时间窗口用于控制故障的生效时间段。只有当前时间落在窗口内时，故障才可能生效。

**时间窗口判断逻辑**:
```
当前时间 >= StartTime  AND  当前时间 <= EndTime
```

- 如果 `StartTime` 为零值，表示无开始时间限制（只要没到结束时间都有效）
- 如果 `EndTime` 为零值，表示无结束时间限制（只要过了开始时间都有效）
- 如果 `TimeWindow` 为 nil，表示始终生效

### 7.2 目标比例机制（仅限延迟和错误注入）

目标比例（Target Ratio）用于控制受故障影响的请求占总请求的百分比。取值范围为 [0, 1.0]，其中:
- `0.0`: 所有请求都不受影响
- `0.5`: 约 50% 的请求会受影响
- `1.0`: 所有请求都会受影响

目标比例使用随机数判断，每次请求独立计算是否命中。对于大量请求，实际受影响的比例会趋近于配置值。

**注意**: `DisconnectConfig` 不支持 `TargetRatio`。连接断开是持久状态，不适用逐请求概率判定。如需模拟"部分请求遇到连接错误"的效果，应使用 `ErrorConfig` 配合连接相关的错误类型。

### 7.3 组合使用

时间窗口和目标比例可以组合使用，形成"在指定时间段内，对 N% 的请求注入故障"的效果。

判断顺序:
1. 首先检查故障是否启用
2. 然后检查是否在时间窗口内
3. 最后检查是否命中目标比例（仅延迟和错误注入）

只有同时满足所有条件，故障才会真正生效。

---

## 8. 并发安全策略

### 8.1 统一锁保护原则

`FaultInjector` 内部使用单个 `sync.RWMutex` 保护所有共享可变状态的访问，确保并发安全策略的一致性。核心共享状态包括：

- `delayCfg`、`errorCfg`、`disconnectCfg`：故障配置
- `disconnected`：手动断开标志
- `randSrc`：随机数生成器（`math/rand.Rand` 非并发安全）

### 8.2 各方法的锁使用策略

| 方法 | 锁类型 | randSrc 访问 | 说明 |
|------|--------|-------------|------|
| `ApplyDelay()` | RLock | ✓ 锁内 | 读取配置、判断比例、计算延迟均在锁内完成；sleepFunc 在锁外调用 |
| `CheckError()` | RLock | ✓ 锁内 | 读取配置、判断比例均在锁内完成；构造 InjectedError 在锁外完成 |
| `CheckDisconnect()` | RLock | 不涉及 | 读取断开状态在锁内完成；构造 ConnectionBrokenError 在锁外完成 |
| `IsDisconnected()` | RLock | 不涉及 | 持久断开语义无需随机数 |
| `SetDelayConfig()` 等 | Lock | 不涉及 | 写锁保护配置更新 |
| `Disconnect()`/`Reconnect()` | Lock | 不涉及 | 写锁保护状态变更 |

### 8.3 关键设计决策

**randSrc 必须在锁内访问**: `math/rand.Rand` 不是并发安全的，其内部状态在每次调用 `Float64()`/`Int63n()` 时都会修改。所有对 `randSrc` 的访问（包括 `hitTargetRatio()` 和 `calculateDelay()` 中的随机数生成）均在持有 `mu` 读锁的状态下完成，避免数据竞争。

**sleepFunc 在锁外调用**: `ApplyDelay()` 在锁内完成所有判断和计算后释放锁，然后在锁外调用 `sleepFunc`。这确保延迟等待期间不会阻塞其他 goroutine 的正常访问。

**错误对象在锁外构造**: `CheckError()` 和 `CheckDisconnect()` 在锁内完成状态判定后释放锁，然后在锁外构造并返回错误对象。错误对象构造不涉及共享状态访问，无需持锁。

---

## 9. 使用示例

### 9.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/chaosfault"
    "time"
)

func main() {
    fi := chaosfault.NewFaultInjector()

    fi.EnableDelay(chaosfault.DelayModeFixed, 100*time.Millisecond)

    err := fi.Inject(func() error {
        fmt.Println("执行业务逻辑")
        return nil
    })

    if err != nil {
        fmt.Printf("调用失败: %v\n", err)
    }
}
```

### 9.2 随机延迟注入

```go
fi := chaosfault.NewFaultInjector()

cfg := chaosfault.DelayConfig{
    Enabled:     true,
    Mode:        chaosfault.DelayModeRandom,
    Min:         50 * time.Millisecond,
    Max:         200 * time.Millisecond,
    TargetRatio: 0.3,
}
fi.SetDelayConfig(cfg)
```

### 9.3 错误注入（按概率触发）

```go
fi := chaosfault.NewFaultInjector()

customErr := errors.New("database connection refused")
cfg := chaosfault.ErrorConfig{
    Enabled:     true,
    Err:         customErr,
    Message:     "模拟数据库连接失败",
    TargetRatio: 0.1, // 10% 的请求会被注入此错误
}
fi.SetErrorConfig(cfg)
```

### 9.4 连接断开模拟（手动控制）

```go
fi := chaosfault.NewFaultInjector()

fi.Disconnect()

err := fi.Inject(func() error {
    return callRemoteService()
})

if errors.Is(err, chaosfault.ErrConnectionBroken) {
    fmt.Println("连接已断开，请求失败")
}

fi.Reconnect()
```

### 9.5 连接断开模拟（配置化持久断开）

```go
fi := chaosfault.NewFaultInjector()

now := time.Now()
cfg := chaosfault.DisconnectConfig{
    Enabled: true,
    TimeWindow: &chaosfault.TimeWindow{
        StartTime: now.Add(1 * time.Hour),
        EndTime:   now.Add(2 * time.Hour),
    },
}
fi.SetDisconnectConfig(cfg)

// 在时间窗口内，所有请求都会返回连接断开错误
// 恢复方式：等待窗口结束，或调用 fi.DisableDisconnect()
```

### 9.6 时间窗口控制

```go
fi := chaosfault.NewFaultInjector()

now := time.Now()
cfg := chaosfault.ErrorConfig{
    Enabled: true,
    Message: "计划内故障注入",
    TimeWindow: &chaosfault.TimeWindow{
        StartTime: now.Add(1 * time.Hour),
        EndTime:   now.Add(2 * time.Hour),
    },
    TargetRatio: 0.5,
}
fi.SetErrorConfig(cfg)
```

### 9.7 综合使用多种故障

```go
fi := chaosfault.NewFaultInjector()

fi.SetDelayConfig(chaosfault.DelayConfig{
    Enabled:     true,
    Mode:        chaosfault.DelayModeFixed,
    Fixed:       100 * time.Millisecond,
    TargetRatio: 0.5,
})

testErr := errors.New("injected timeout")
fi.SetErrorConfig(chaosfault.ErrorConfig{
    Enabled:     true,
    Err:         testErr,
    Message:     "请求超时",
    TargetRatio: 0.1,
})

for i := 0; i < 100; i++ {
    err := fi.Inject(func() error {
        return actualBusinessLogic()
    })
    if err != nil {
        // 处理错误
    }
}
```

### 9.8 用于单元测试

```go
func TestMyFunction_WithDelay(t *testing.T) {
    var slept time.Duration
    fi := chaosfault.NewFaultInjector(
        chaosfault.WithSleepFunc(func(d time.Duration) {
            slept = d
        }),
        chaosfault.WithRandSource(rand.New(rand.NewSource(42))),
    )

    fi.EnableDelay(chaosfault.DelayModeFixed, 200*time.Millisecond)

    err := fi.Inject(func() error {
        return myFunction()
    })

    if slept != 200*time.Millisecond {
        t.Errorf("expected 200ms delay, got %v", slept)
    }
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidConfig` | 配置无效 | 配置参数不合法时返回 |
| `ErrConnectionBroken` | 连接已断开 | 连接断开状态下发起请求 |
| `ErrInvalidTargetRatio` | 目标比例无效 | 比例超出 [0, 1.0] 范围 |
| `ErrInvalidTimeWindow` | 时间窗口无效 | 开始时间晚于结束时间 |

### 10.1 错误类型

| 错误类型 | 说明 |
|---------|------|
| `*InjectedError` | 注入的错误，包含 Message 和 Cause |
| `*ConnectionBrokenError` | 连接断开错误，可自定义 Message |

**错误检查示例**:
```go
// 检查是否为注入的错误
var injErr *chaosfault.InjectedError
if errors.As(err, &injErr) {
    fmt.Println("这是注入的错误:", injErr.Message)
}

// 检查是否为连接断开
if errors.Is(err, chaosfault.ErrConnectionBroken) {
    fmt.Println("连接已断开")
}

// 检查注入错误是否包含特定原始错误
var dbErr = errors.New("db error")
if errors.Is(err, dbErr) {
    fmt.Println("检测到数据库错误")
}
```

---

## 11. 配置说明

### 11.1 DelayConfig 完整参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Enabled` | `bool` | `false` | 是否启用延迟注入 |
| `Mode` | `DelayMode` | `DelayModeFixed` | 延迟模式：固定或随机 |
| `Fixed` | `time.Duration` | `0` | 固定延迟时长（固定模式） |
| `Min` | `time.Duration` | `0` | 随机延迟最小值（随机模式） |
| `Max` | `time.Duration` | `0` | 随机延迟最大值（随机模式） |
| `TimeWindow` | `*TimeWindow` | `nil` | 生效时间窗口，nil 表示始终生效 |
| `TargetRatio` | `float64` | `0` | 目标比例 [0, 1.0] |

### 11.2 ErrorConfig 完整参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Enabled` | `bool` | `false` | 是否启用错误注入 |
| `Err` | `error` | `nil` | 注入的原始错误 |
| `Message` | `string` | `""` | 错误消息文本 |
| `TimeWindow` | `*TimeWindow` | `nil` | 生效时间窗口 |
| `TargetRatio` | `float64` | `0` | 目标比例 |

### 11.3 DisconnectConfig 完整参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Enabled` | `bool` | `false` | 是否启用连接断开模拟 |
| `TimeWindow` | `*TimeWindow` | `nil` | 生效时间窗口 |

**注意**: `DisconnectConfig` 不包含 `TargetRatio`。连接断开是持久状态，启用后在时间窗口内所有请求持续失败。

### 11.4 配置验证规则

创建或更新配置时，以下非法值会导致错误:

| 非法情况 | 适用配置 | 错误类型 |
|---------|---------|---------|
| `TargetRatio < 0` 或 `> 1.0` | DelayConfig、ErrorConfig | `ErrInvalidTargetRatio` |
| 时间窗口 StartTime > EndTime | 所有配置 | `ErrInvalidTimeWindow` |
| 固定延迟模式下 Fixed <= 0 | DelayConfig | `ErrInvalidConfig` |
| 随机延迟模式下 Min 或 Max <= 0 | DelayConfig | `ErrInvalidConfig` |
| 随机延迟模式下 Min >= Max | DelayConfig | `ErrInvalidConfig` |
| 错误注入启用但 Err 和 Message 都为空 | ErrorConfig | `ErrInvalidConfig` |

---

## 12. 最佳实践

### 12.1 生产环境使用建议

1. **默认关闭**: 生产环境默认关闭所有故障注入，需要时通过配置或管理接口动态启用
2. **小比例验证**: 初次使用时，设置较小的目标比例（如 1%），观察系统反应
3. **时间窗口保护**: 使用时间窗口限制故障注入的持续时间，避免长时间影响线上服务
4. **监控告警**: 配合监控系统，在故障注入期间密切关注关键指标
5. **一键止损**: 提供快速禁用所有故障注入的应急手段（调用 `Reset()`）

### 12.2 测试场景建议

1. **单元测试**: 使用自定义 `sleepFunc` 和 `randSrc`，避免测试等待真实时间
2. **集成测试**: 结合时间窗口，在测试的特定阶段启用故障注入
3. **故障演练**: 手动控制断开/恢复，验证系统的故障转移和恢复能力
4. **边界测试**: 分别测试 0%、50%、100% 三种比例下的系统行为

### 12.3 典型反模式

❌ **反模式 1: 在生产环境长期启用高比例故障注入**
```go
// 危险：生产环境 100% 注入错误
fi.EnableError(err, "error")
```

✅ **修正**: 使用小比例 + 时间窗口，或仅在测试/演练环境使用。

---

❌ **反模式 2: 注入错误不标识，导致业务逻辑混淆**
```go
// 危险：注入的错误和真实错误无法区分
fi.EnableError(dbErr, "")
```

✅ **修正**: 通过 `errors.As(err, &injErr)` 判断是否为注入错误，或使用特定的 Message 标识。

---

❌ **反模式 3: 期望 Reconnect() 恢复配置化断开**
```go
// 危险：Reconnect() 不会覆盖配置化的持久断开
fi.EnableDisconnect()
fi.Reconnect() // 连接仍然是断开的！
```

✅ **修正**: 使用 `DisableDisconnect()` 禁用配置，或等待时间窗口结束。

---

❌ **反模式 4: 延迟注入阻塞关键路径**
```go
// 危险：在主循环中使用长延迟
for {
    fi.ApplyDelay() // 10秒延迟
    process()
}
```

✅ **修正**: 仅在模拟外部依赖调用时使用延迟注入，避免影响核心处理逻辑。
