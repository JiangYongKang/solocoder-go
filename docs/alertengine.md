# AlertEngine 告警规则引擎模块

## 模块概述

`internal/alertengine` 包提供了一个功能完整的告警规则引擎，支持多种告警条件配置、持续时长检测、告警抑制、静默时段、告警升级以及多渠道通知分发等核心能力。适用于监控系统、业务指标告警等场景。

## 核心功能

### 1. 阈值告警
支持对指标值设置上限或下限阈值，当指标值越过阈值时触发告警。

**支持的比较操作符：**
- `>` (大于)
- `<` (小于)
- `>=` (大于等于)
- `<=` (小于等于)

### 2. 同环比告警
支持将当前周期的指标值与历史周期数据进行比较，当变化幅度超过指定百分比阈值时触发告警。

- **环比 (Ringbi)**: 与上一周期比较，周期时长可配置
- **同比 (Tongbi)**: 与去年同期比较

### 3. 持续时长告警
当某个告警条件在连续多个检查周期内持续成立时才触发告警，避免短暂波动导致的误报。

**两种配置方式：**
- 按检查次数 (`DurationByCount`): 连续 N 次检查都满足条件才触发
- 按时间长度 (`DurationByTime`): 在指定时间窗口内持续满足条件才触发

### 4. 告警抑制
同一条告警规则在触发后的一段时间内，即使条件再次满足也不再重复发送通知。抑制时间窗口可配置，抑制期结束后如果条件仍然成立则重新发送通知。

### 5. 静默时段
支持为告警规则配置静默时间窗口，在静默时段内规则照常评估但抑制所有通知。

**两种静默方式：**
- **按天静默 (`SilentDaily`)**: 按每天的起止时间设置，如夜间 22:00 - 06:00，支持跨天时段
- **按日期范围静默 (`SilentRange`)**: 指定具体的开始和结束日期时间

### 6. 告警升级
告警持续未恢复的时间超过指定阈值后，自动将告警等级升级。升级后可使用不同的通知渠道和通知接收人。

**告警等级从低到高：**
- `info` (信息)
- `warning` (警告)
- `alert` (告警)
- `critical` (严重)

### 7. 多渠道通知分发
支持为不同告警等级配置不同的通知渠道：
- **控制台输出 (`ConsoleNotifier`)**: 将告警信息输出到标准输出
- **回调函数 (`CallbackNotifier`)**: 允许外部注入自定义通知逻辑，如发送邮件、推送消息、调用 Webhook 等

通知内容包含：告警名称、触发值、触发时间、当前等级、标签等信息。

## 核心结构体职责

### Engine (告警引擎)
告警引擎的核心入口，负责管理告警规则、维护告警状态、执行规则评估和分发通知。

**主要职责：**
- 规则的注册、查询和移除
- 告警状态的管理和持久化
- 接收指标数据并执行规则评估
- 管理通知渠道并分发告警通知
- 维护历史指标数据用于同环比计算

**关键方法：**
- `NewEngine(cfg EngineConfig) *Engine`: 创建告警引擎实例
- `AddRule(rule *AlertRule) error`: 添加告警规则
- `RemoveRule(ruleID string) error`: 移除告警规则
- `GetRule(ruleID string) (*AlertRule, error)`: 查询告警规则
- `GetAlertState(ruleID string) (*AlertState, error)`: 获取告警当前状态
- `Evaluate(ruleID string, dataPoint MetricDataPoint) error`: 执行一次规则评估
- `RegisterNotifier(notifier Notifier)`: 注册通知渠道

### AlertRule (告警规则)
定义一条完整的告警规则，包含规则标识、触发条件、持续时长要求、抑制配置、静默时段、升级策略和通知渠道等。

**关键字段：**
- `ID`: 规则唯一标识
- `Name`: 告警名称
- `MetricName`: 关联的指标名称
- `Labels`: 规则标签，用于维度过滤
- `Tags`: 规则标签列表
- `InitialLevel`: 初始告警等级
- `Threshold`: 阈值告警条件（与 RingbiTongbi 二选一）
- `RingbiTongbi`: 同环比告警条件（与 Threshold 二选一）
- `Duration`: 持续时长要求
- `InhibitDuration`: 告警抑制窗口
- `SilentWindows`: 静默时段配置列表
- `Escalations`: 告警升级规则列表
- `Notifiers`: 通知渠道名称列表

### AlertState (告警状态)
维护告警规则的运行时状态，记录告警的生命周期信息。

**关键字段：**
- `RuleID`: 关联的规则 ID
- `Status`: 当前告警状态 (pending/firing/suppressed/resolved)
- `CurrentLevel`: 当前告警等级
- `TriggerValue`: 最近一次触发值
- `TriggerTime`: 最近一次触发时间
- `LastFiredTime`: 条件最近一次满足的时间
- `FirstFiredTime`: 条件首次满足的时间（用于持续时长和升级判断）
- `ConsecutiveHits`: 连续满足条件的次数
- `HistoryValues`: 历史指标数据点（用于同环比计算）
- `LastEvaluatedTime`: 最近一次评估时间
- `LastNotifiedTime`: 最近一次发送通知的时间（用于抑制判断）
- `ResolvedTime`: 告警恢复时间

### ThresholdCondition (阈值条件)
定义阈值告警的触发条件。

**关键字段：**
- `Operator`: 比较操作符 (>, <, >=, <=)
- `Threshold`: 阈值数值

### RingbiTongbiCondition (同环比条件)
定义同环比告警的触发条件。

**关键字段：**
- `CompareType`: 比较类型 (ringbi/tongbi)
- `PercentThreshold`: 变化百分比阈值（绝对值）
- `Period`: 环比周期时长（同比固定为一年）

### DurationCondition (持续时长条件)
定义告警持续时长要求。

**关键字段：**
- `Type`: 持续方式 (count/time)
- `CheckCount`: 需要连续满足的检查次数
- `TimeWindow`: 需要持续满足的时间长度

### SilentWindow (静默时段)
定义静默时间窗口配置。

**关键字段：**
- `Type`: 静默类型 (daily/range)
- `StartTime/EndTime`: 按天静默的起止时间，格式 "HH:MM"
- `StartDate/EndDate`: 按日期范围静默的起止时间
- `Tags`: 关联的标签

### EscalationRule (升级规则)
定义告警升级策略。

**关键字段：**
- `AfterDuration`: 持续多久后升级
- `FromLevel`: 从哪个等级开始升级
- `ToLevel`: 升级到哪个等级

### Notification (通知内容)
通知的结构化数据。

**关键字段：**
- `RuleID`: 规则 ID
- `AlertName`: 告警名称
- `TriggerValue`: 触发值
- `TriggerTime`: 触发时间
- `CurrentLevel`: 当前告警等级
- `Labels`: 规则标签
- `Message`: 格式化的告警消息

### Notifier 接口
通知渠道抽象接口，所有通知渠道必须实现此接口。

```go
type Notifier interface {
    Send(notification Notification) error
    Name() string
}
```

**内置实现：**
- `ConsoleNotifier`: 控制台输出通知，同时会记录所有已发送的通知供测试或调试使用
- `CallbackNotifier`: 回调函数通知，允许通过注入自定义函数实现任意通知逻辑

## 告警生命周期

告警从创建到恢复经历完整的状态流转：

```
          ┌──────────────────────────────────────────────┐
          │                                              │
          ▼                                              │
   ┌───────────┐    条件满足+持续要求    ┌───────────┐    │
   │  Pending  │ ──────────────────────► │  Firing   │ ───┘
   └───────────┘                         └─────┬─────┘
          ▲                                    │
          │                                    │ 条件不满足
          │                                    ▼
          │                              ┌───────────┐
          └──────────────────────────────│ Resolved  │
                   条件不满足             └───────────┘
```

**完整流程说明：**

1. **规则创建 (Pending)**: 告警规则被添加到引擎后，初始状态为 `Pending`，等待指标数据输入。

2. **条件评估**:
   - 每次调用 `Evaluate()` 传入一个新的指标数据点
   - 引擎先判断基础条件（阈值或同环比）是否满足
   - 如果配置了持续时长要求，则需要连续满足一定次数或时间后才算真正触发
   - 历史数据点会被存储用于同环比比较（最多保留最近 1000 个数据点）

3. **告警触发 (Firing)**:
   - 条件满足且达到持续时长要求后，告警状态变为 `Firing`
   - 检查是否在抑制窗口内（自上次通知起的抑制时间）
   - 检查是否在静默时段内
   - 如果不在抑制期且不在静默时段，则发送通知
   - 发送通知时检查是否需要告警升级（持续未恢复超过指定时间）

4. **通知发送**:
   - 根据规则配置的通知渠道列表选择渠道
   - 如果未配置具体渠道，则使用引擎注册的所有渠道
   - 通知内容包含告警名称、触发值、触发时间、当前等级和标签
   - 各通知渠道独立执行，单个渠道失败不影响其他渠道

5. **告警恢复 (Resolved)**:
   - 当条件不再满足时，告警状态变为 `Resolved`
   - 重置连续命中计数和首次触发时间
   - 记录恢复时间

## 错误处理

模块定义了以下错误类型：

| 错误 | 说明 |
|------|------|
| `ErrRuleNotFound` | 规则不存在 |
| `ErrRuleAlreadyExists` | 规则已存在 |
| `ErrInvalidRule` | 无效的规则配置 |
| `ErrInvalidCondition` | 无效的条件配置 |
| `ErrInvalidOperator` | 无效的比较操作符 |
| `ErrInvalidThreshold` | 无效的阈值配置 |
| `ErrInvalidDuration` | 无效的持续时长配置 |
| `ErrInvalidSilentWindow` | 无效的静默时段配置 |
| `ErrInvalidLevel` | 无效的告警等级 |
| `ErrNotifierNotFound` | 通知渠道不存在 |
| `ErrInvalidMetricData` | 无效的指标数据 |
| `ErrNoConditionDefined` | 规则未定义任何触发条件 |

## 使用示例

### 基本使用：阈值告警

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/alertengine"
)

func main() {
    // 创建控制台通知器
    console := alertengine.NewConsoleNotifier()

    // 创建告警引擎
    engine := alertengine.NewEngine(alertengine.EngineConfig{
        Notifiers: map[string]alertengine.Notifier{
            "console": console,
        },
    })

    // 创建 CPU 使用率过高告警规则
    cpuRule := &alertengine.AlertRule{
        ID:           "cpu-high",
        Name:         "CPU使用率过高",
        MetricName:   "cpu_usage_percent",
        InitialLevel: alertengine.LevelWarning,
        Threshold: &alertengine.ThresholdCondition{
            Operator:  alertengine.OpGreaterThan,
            Threshold: 80,
        },
        Notifiers: []string{"console"},
    }

    if err := engine.AddRule(cpuRule); err != nil {
        panic(err)
    }

    // 模拟指标数据上报
    engine.Evaluate("cpu-high", alertengine.MetricDataPoint{
        Timestamp: time.Now(),
        Value:     85.5,
        Labels:    map[string]string{"host": "server-01"},
    })

    // 查看告警状态
    state, _ := engine.GetAlertState("cpu-high")
    fmt.Printf("告警状态: %s, 等级: %s\n", state.Status, state.CurrentLevel)
}
```

### 持续时长告警

```go
// 连续 3 次检查都超过阈值才触发告警
rule := &alertengine.AlertRule{
    ID:           "latency-high",
    Name:         "接口延迟过高",
    MetricName:   "request_latency",
    InitialLevel: alertengine.LevelAlert,
    Threshold: &alertengine.ThresholdCondition{
        Operator:  alertengine.OpGreaterThan,
        Threshold: 500,
    },
    Duration: &alertengine.DurationCondition{
        Type:       alertengine.DurationByCount,
        CheckCount: 3,
    },
    Notifiers: []string{"console"},
}
```

### 同环比告警

```go
// 当前值较上一小时变化超过 30% 触发告警
rule := &alertengine.AlertRule{
    ID:           "qps-change",
    Name:         "QPS突增/突降",
    InitialLevel: alertengine.LevelWarning,
    RingbiTongbi: &alertengine.RingbiTongbiCondition{
        CompareType:      alertengine.CompareRingbi,
        PercentThreshold: 30,
        Period:           time.Hour,
    },
    Notifiers: []string{"console"},
}
```

### 告警抑制 + 静默时段

```go
rule := &alertengine.AlertRule{
    ID:           "disk-warning",
    Name:         "磁盘空间告警",
    InitialLevel: alertengine.LevelAlert,
    Threshold: &alertengine.ThresholdCondition{
        Operator:  alertengine.OpGreaterThan,
        Threshold: 90,
    },
    // 触发后 10 分钟内不再重复通知
    InhibitDuration: 10 * time.Minute,
    // 每天夜间 22:00 - 次日 06:00 静默
    SilentWindows: []alertengine.SilentWindow{
        {
            Type:      alertengine.SilentDaily,
            StartTime: "22:00",
            EndTime:   "06:00",
        },
    },
    Notifiers: []string{"console"},
}
```

### 告警升级

```go
rule := &alertengine.AlertRule{
    ID:           "db-connection",
    Name:         "数据库连接数告警",
    InitialLevel: alertengine.LevelWarning,
    Threshold: &alertengine.ThresholdCondition{
        Operator:  alertengine.OpGreaterThan,
        Threshold: 500,
    },
    Escalations: []alertengine.EscalationRule{
        // 持续 5 分钟未恢复，升级为告警
        {
            AfterDuration: 5 * time.Minute,
            FromLevel:     alertengine.LevelWarning,
            ToLevel:       alertengine.LevelAlert,
        },
        // 再持续 10 分钟，升级为严重告警
        {
            AfterDuration: 15 * time.Minute,
            FromLevel:     alertengine.LevelAlert,
            ToLevel:       alertengine.LevelCritical,
        },
    },
    Notifiers: []string{"console"},
}
```

### 自定义回调通知

```go
// 创建 Webhook 回调通知器
webhook := alertengine.NewCallbackNotifier("webhook", func(n alertengine.Notification) error {
    // 这里可以实现发送 HTTP 请求、邮件、短信等逻辑
    fmt.Printf("[Webhook] 告警: %s, 值: %.2f, 等级: %s\n",
        n.AlertName, n.TriggerValue, n.CurrentLevel)
    return nil
})

engine := alertengine.NewEngine(alertengine.EngineConfig{
    Notifiers: map[string]alertengine.Notifier{
        "console": alertengine.NewConsoleNotifier(),
        "webhook": webhook,
    },
})

rule := &alertengine.AlertRule{
    ID:         "multi-channel",
    Name:       "多渠道通知示例",
    Threshold:  &alertengine.ThresholdCondition{Operator: alertengine.OpGreaterThan, Threshold: 100},
    Notifiers:  []string{"console", "webhook"},
}
engine.AddRule(rule)
```

## 并发安全

引擎内部使用互斥锁（`sync.RWMutex`）保护规则和状态数据，支持并发安全地进行：
- 规则的添加、删除和查询
- 告警状态的查询
- 多协程并发调用 `Evaluate()` 进行规则评估

`ConsoleNotifier` 和 `CallbackNotifier` 内部也使用互斥锁保护通知记录，支持并发安全访问。

## 测试

运行单元测试：

```bash
go test ./internal/alertengine/ -v
```

测试覆盖以下方面：
- 阈值告警四种操作符的正确行为
- 阈值告警未触发和恢复场景
- 持续时长按计数和按时间两种方式
- 持续时长中断后的重置逻辑
- 环比和同比告警的正确触发
- 同环比缺少历史数据或比较值为 0 的边界情况
- 告警抑制窗口的生效和失效
- 每日静默时段（含跨天时段）的判断
- 日期范围静默时段的判断
- 告警升级的正确触发
- 控制台通知器和回调通知器的功能
- 多渠道和默认渠道通知分发
- 通知内容的完整性校验
- 无效规则、无效操作符、无效静默窗口等错误处理
- 默认等级和默认抑制时长等默认值设置
- 并发场景下的评估安全性
- 历史数据点的自动裁剪
