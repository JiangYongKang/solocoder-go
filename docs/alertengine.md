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

#### 按等级配置通知渠道
每个告警规则支持为不同告警等级配置独立的通知渠道列表。当告警升级或降级时，通知渠道会自动切换到对应等级的配置。

**优先级规则：**
1. 如果当前等级在 `LevelNotifiers` 中有配置且非空，则使用该等级对应的通知渠道
2. 否则降级使用通用的 `Notifiers` 列表
3. 如果两者都未配置，则使用引擎中注册的所有通知渠道

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
- `Tags`: 规则标签列表，用于静默窗口匹配
- `InitialLevel`: 初始告警等级
- `Threshold`: 阈值告警条件（与 RingbiTongbi 二选一）
- `RingbiTongbi`: 同环比告警条件（与 Threshold 二选一）
- `Duration`: 持续时长要求
- `InhibitDuration`: 告警抑制窗口，0 表示不抑制，负值使用默认值
- `SilentWindows`: 静默时段配置列表
- `Escalations`: 告警升级规则列表
- `Notifiers`: 通用通知渠道名称列表
- `LevelNotifiers`: 按告警等级划分的通知渠道，优先级高于 Notifiers

### AlertState (告警状态)
维护告警规则的运行时状态，记录告警的生命周期信息。

**关键字段：**
- `RuleID`: 关联的规则 ID
- `Status`: 当前告警状态 (pending/firing/suppressed/resolved)
- `CurrentLevel`: 当前告警等级
- `TriggerValue`: 最近一次触发值
- `TriggerTime`: 最近一次触发时间
- `LastFiredTime`: 条件最近一次满足的时间
- `FirstFiredTime`: 条件首次满足的时间（用于持续时长判断）
- `FirstTriggeredTime`: 告警实际触发时间（告警升级的计时起点）
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
- `Tolerance`: 历史数据匹配容差，未配置时使用默认值

#### 历史数据匹配容差 (Tolerance)

在进行同环比比较时，需要从历史数据中找到与目标时间点匹配的数据点。由于数据上报可能存在时间偏差，需要设置匹配容差。

**默认容差规则：**
- **同比 (Tongbi)**: 默认容差为 24 小时。因为同比数据跨越一整年，数据采集时间点可能有较大偏移，较大的容差能确保找到匹配的历史数据。
- **环比 (Ringbi)**: 默认容差为周期的一半。例如周期为 1 小时，则容差为 30 分钟。

**配置建议：**
- 对于数据上报规律性强、时间精度高的场景，可以适当减小容差
- 对于数据上报不规律或需要宽松匹配的场景，可以增大容差
- 同比场景建议至少保留数小时的容差，避免因节假日、周末等因素导致数据无法匹配

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
- `Tags`: 标签列表，用于按标签维度匹配静默规则

#### 按标签维度静默

静默窗口支持通过 `Tags` 字段实现按标签维度的静默匹配。当静默窗口配置了标签时，只有规则标签与静默窗口标签存在交集时，该静默窗口才会生效。

**匹配规则：**
- 如果静默窗口未配置 Tags（空或 nil），则该静默窗口对所有规则生效
- 如果静默窗口配置了 Tags，则只有当规则的 Tags 中至少包含一个静默窗口的标签时，静默才生效
- 标签匹配是"或"逻辑，只要有一个标签匹配即可

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
   - 设置 `FirstTriggeredTime`（告警实际触发时间），作为告警升级的计时起点
   - 检查是否在抑制窗口内（自上次通知起的抑制时间）
   - 检查是否在静默时段内
   - 如果不在抑制期且不在静默时段，则发送通知
   - 每次评估时检查是否需要告警升级（自首次触发起持续未恢复超过指定时间）

#### 告警升级时钟规则

告警升级的计时起点是 **告警实际触发时间** (`FirstTriggeredTime`)，而非基础条件首次命中时间 (`FirstFiredTime`)。

**设计原因：**
- 在持续时长告警场景中，基础条件可能很早就命中了，但需要满足持续时长要求后告警才真正触发
- 如果从基础条件首次命中就开始计时升级，可能导致告警在刚触发时就被直接升级，跳过初始等级
- 从告警实际触发时间开始计时，确保每个等级都有完整的升级等待周期

**时间线示例（持续 3 次检查后触发，持续 5 分钟后升级）：**
```
时间点:    T0      T1      T2      T3      T4      T5      T6
条件:      ✓       ✓       ✓       ✓       ✓       ✓       ✓
状态:   Pending Pending Pending  Firing  Firing  Firing  升级
升级计时:  0       0       0       0s     1min    4min    5min→升级
```
- T0-T2：条件满足但未达到持续要求，不触发告警，升级计时不启动
- T3：满足持续时长要求，告警触发，`FirstTriggeredTime` 设置，升级计时开始
- T3 后 5 分钟：告警升级到下一等级

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

### 按等级配置通知渠道 + 告警升级

```go
rule := &alertengine.AlertRule{
    ID:           "level-notify",
    Name:         "数据库连接池告警",
    InitialLevel: alertengine.LevelWarning,
    Threshold: &alertengine.ThresholdCondition{
        Operator:  alertengine.OpGreaterThan,
        Threshold: 80,
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
    // 按等级配置不同的通知渠道
    LevelNotifiers: map[alertengine.AlertLevel][]string{
        alertengine.LevelWarning:  {"console"},           // 警告只发控制台
        alertengine.LevelAlert:    {"console", "email"},  // 告警发控制台+邮件
        alertengine.LevelCritical: {"console", "email", "sms"}, // 严重告警发所有渠道
    },
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

### 并发安全策略

引擎采用 **全局互斥锁** (`sync.RWMutex`) 保护所有共享状态，确保并发场景下的数据一致性。

#### 读写锁使用策略

| 操作 | 锁类型 | 说明 |
|------|--------|------|
| 规则查询 (`GetRule`) | 读锁 (`RLock`) | 只读操作，支持并发读 |
| 告警状态查询 (`GetAlertState`) | 读锁 (`RLock`) | 只读操作，支持并发读 |
| 添加规则 (`AddRule`) | 写锁 (`Lock`) | 修改规则映射，需独占访问 |
| 移除规则 (`RemoveRule`) | 写锁 (`Lock`) | 修改规则映射，需独占访问 |
| 注册通知器 (`RegisterNotifier`) | 写锁 (`Lock`) | 修改通知器映射，需独占访问 |
| 规则评估 (`Evaluate`) | 写锁 (`Lock`) | 修改告警状态，全程锁保护 |

#### Evaluate 方法的并发安全

`Evaluate` 方法是最核心的写操作，**整个评估过程全程在写锁保护下执行**，包括：
- 读取规则配置
- 读取和修改告警状态
- 更新历史数据
- 检查持续时长、抑制期、静默期
- 触发告警升级
- 发送通知

**设计考量：**
- 虽然全程加锁会降低并发度，但保证了状态修改的原子性和一致性
- 避免了"先检查后执行"的竞态条件
- 避免了状态在评估过程中被其他 goroutine 中途修改
- 通知发送在锁内执行（通知器内部有自己的锁），确保状态与通知的时序一致性

### 通知器的并发安全

- `ConsoleNotifier`: 内部使用互斥锁保护通知记录列表
- `CallbackNotifier`: 内部使用互斥锁保护通知记录列表和回调函数
- 所有通知器方法都是并发安全的

### 并发场景最佳实践

1. **避免长时间持有锁**: 自定义 `Notifier` 的 `Send` 方法应尽快返回，避免阻塞
2. **单一规则评估**: 同一规则的多次评估是串行执行的，不同规则的评估也串行（全局锁）
3. **读多写少场景**: 对于高频查询场景，读操作使用 RLock 可提高并发度
4. **数据一致性**: 由于使用全局锁，状态查询总是能看到一致的快照

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
- 同比告警历史数据匹配容差（默认容差和自定义容差）
- 告警抑制窗口的生效和失效
- 每日静默时段（含跨天时段）的判断
- 日期范围静默时段的判断
- 静默窗口按标签维度匹配
- 告警升级的正确触发
- 告警升级从告警实际触发时间开始计时
- 按告警等级切换通知渠道
- 等级通知渠道的 fallback 机制
- 控制台通知器和回调通知器的功能
- 多渠道和默认渠道通知分发
- 通知内容的完整性校验
- 无效规则、无效操作符、无效静默窗口等错误处理
- AddRule 配置校验门禁（等级、操作符、比较类型、阈值、时长、静默窗口、升级规则）
- RegisterNotifier 配置校验
- 无效指标数据校验
- 默认等级和默认抑制时长等默认值设置
- 并发场景下的评估安全性
- 并发场景下的状态一致性
- 历史数据点的自动裁剪
