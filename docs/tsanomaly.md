# tsanomaly - 时序异常检测器模块

## 模块功能

`tsanomaly` 提供了一套针对时间序列数据的异常检测机制，支持以下核心功能：

1. **基于移动平均的基线计算**：通过配置窗口大小计算时间序列数据的移动平均值作为正常行为的基线，新增数据点时增量更新基线，无需每次全量重算整个窗口。窗口超出大小时自动淘汰最旧数据点，保证内存占用恒定。

2. **标准差偏离检测**：在移动平均基线的基础上计算数据的标准差（样本无偏估计），当新数据点偏离基线超过配置倍数的标准差时标记为异常。偏离方向支持上偏检测、下偏检测、双向检测三种模式，倍数阈值可灵活配置。

3. **季节性周期建模**：支持按指定周期长度学习数据的季节性模式（如每天24小时、每周7天的周期规律）。每个周期位置维护独立的基线均值和标准差，检测时使用对应周期位置的基线进行判断，而非全局基线，有效提升具有周期性规律数据的检测准确率。

4. **异常事件标记**：每个被检测为异常的数据点记录完整的诊断信息，包括发生时间、实际值、基线值、标准差、偏离度、偏离倍数、判断阈值、偏离方向、严重程度和季节性索引。支持丰富的历史异常事件查询，可按时间范围、偏离方向、严重程度筛选，异常事件自动按时间升序排序。

5. **生命周期管理**：支持动态更新配置、重置检测器状态、关闭检测器等操作。内置并发安全保护，可在多 Goroutine 环境下安全使用。

## 核心结构体与职责

### Config

检测器配置结构体，包含所有可调参数。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `WindowSize` | int | 100 | 移动平均窗口大小，必须为正整数 |
| `StdDevFactor` | float64 | 3.0 | 标准差偏离倍数阈值，非负，0表示任何偏离都算异常 |
| `MinSamples` | int | 10 | 启动异常检测的最小样本数，必须 > 0 且 <= WindowSize |
| `EnableSeasonal` | bool | false | 是否启用季节性模式 |
| `PeriodLength` | int | 0 | 季节性周期长度（槽位数量），启用季节性时必须为正 |
| `PeriodSlot` | time.Duration | 0 | 每个周期槽位的时间跨度，启用季节性时必须为正。例如：1小时表示每小时一个槽位 |
| `SeasonalEpoch` | time.Time | 零值 | 周期基准时间，用于计算时间戳对应的周期槽位索引。建议使用业务周期的起始对齐点 |
| `Direction` | DeviationDirection | DirectionBoth | 异常检测方向 |
| `MaxAnomalyHistory` | int | 1000 | 最大异常事件历史记录数，超限后自动淘汰最旧记录，0表示不限 |

方法：

| 方法 | 说明 |
|------|------|
| `DefaultConfig() Config` | 返回默认配置 |
| `ValidateConfig(cfg Config) error` | 校验配置合法性 |

### DeviationDirection

偏离方向枚举类型。

| 常量 | 值 | 说明 |
|------|----|------|
| `DirectionBoth` | 0 | 双向检测：上偏或下偏均判为异常 |
| `DirectionUp` | 1 | 仅上偏检测：仅当值高于基线超过阈值时判为异常 |
| `DirectionDown` | 2 | 仅下偏检测：仅当值低于基线超过阈值时判为异常 |

方法：

| 方法 | 说明 |
|------|------|
| `String() string` | 返回方向的字符串表示 |

### AnomalySeverity

异常严重程度级别。

| 常量 | 值 | 说明 |
|------|----|------|
| `SeverityWarning` | "warning" | 警告级：偏离倍数 >= StdDevFactor 且 < 2*StdDevFactor |
| `SeverityCritical` | "critical" | 严重级：偏离倍数 >= 2*StdDevFactor |

### DataPoint

时间序列数据点。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Timestamp` | time.Time | 数据点时间戳 |
| `Value` | float64 | 数据点数值 |

### AnomalyEvent

异常事件记录结构体，包含完整的诊断信息。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Timestamp` | time.Time | 异常发生时间戳 |
| `ActualValue` | float64 | 实际观测值 |
| `BaselineValue` | float64 | 检测时使用的基线值（均值） |
| `StdDev` | float64 | 检测时使用的标准差 |
| `Deviation` | float64 | 绝对偏离量 = ActualValue - BaselineValue |
| `DeviationRatio` | float64 | 相对偏离倍数 = \|Deviation\| / StdDev（StdDev为0时回退为相对于Baseline的比值） |
| `Threshold` | float64 | 判断阈值 = StdDevFactor * StdDev |
| `Direction` | DeviationDirection | 偏离方向 |
| `Severity` | AnomalySeverity | 严重程度 |
| `SeasonalIndex` | int | 季节性索引（季节性模式下的周期位置，非季节性时为0） |

### AnomalyQuery

异常事件查询条件结构体。

| 字段 | 类型 | 说明 |
|------|------|------|
| `StartTime` | *time.Time | 起始时间过滤（可选，nil表示不限） |
| `EndTime` | *time.Time | 结束时间过滤（可选，nil表示不限） |
| `Direction` | *DeviationDirection | 偏离方向过滤（可选，nil表示不限） |
| `Severity` | *AnomalySeverity | 严重程度过滤（可选，nil表示不限） |
| `Limit` | int | 返回结果数量限制（取最新N条，<=0表示不限） |

### windowStats（内部）

滑动窗口统计量内部结构体，负责增量维护窗口内的统计信息。

内部字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `values` | *list.List | 窗口内的数据值（链表，便于从头部淘汰） |
| `sum` | float64 | 窗口内值的累加和（增量维护） |
| `sumSq` | float64 | 窗口内值的平方和（增量维护，用于计算方差） |

方法：

| 方法 | 说明 |
|------|------|
| `count() int` | 当前窗口内数据点数量 |
| `mean() float64` | 窗口均值，空窗口返回0 |
| `variance() float64` | 样本无偏方差，样本数 < 2 时返回0 |
| `stdDev() float64` | 样本标准差 = sqrt(variance) |
| `add(value float64, maxSize int)` | 添加新值并自动淘汰超出窗口大小的最旧值 |
| `reset()` | 重置到初始空状态 |

### Detector

时序异常检测器主结构体，对外提供完整的异常检测功能。

主要方法：

| 方法 | 说明 |
|------|------|
| `NewDetector(cfg Config) (*Detector, error)` | 创建检测器，配置不合法时返回错误 |
| `NewDetectorWithDefault() *Detector` | 使用默认配置创建检测器 |
| `Config() Config` | 获取当前配置 |
| `UpdateConfig(cfg Config) error` | 动态更新配置 |
| `Add(point *DataPoint) (*AnomalyEvent, error)` | 添加单个数据点，返回异常事件（若触发） |
| `BatchAdd(points []*DataPoint) ([]*AnomalyEvent, error)` | 批量添加数据点，返回所有触发的异常事件 |
| `GetBaseline() (mean, stdDev float64, count int)` | 获取全局基线统计量 |
| `GetSeasonalBaseline(index int) (mean, stdDev float64, count int, err error)` | 获取指定季节性索引的基线统计量 |
| `GetAnomalies(query *AnomalyQuery) []*AnomalyEvent` | 查询历史异常事件 |
| `AnomalyCount() int` | 获取异常事件总数 |
| `PointCount() int64` | 获取已添加的数据点总数 |
| `Reset()` | 重置所有状态（基线、历史、计数） |
| `Close()` | 关闭检测器，之后Add会返回错误 |
| `IsClosed() bool` | 判断检测器是否已关闭 |

## 异常检测数学模型

### 1. 移动平均基线（增量计算）

设窗口大小为 N，当前窗口内数据为 x₁, x₂, ..., xₙ (n ≤ N)

**均值（算术平均）：**

```
μ = Σ(xᵢ) / n
```

增量更新：新值 x_new 加入，若窗口已满则移除最旧值 x_old：

```
sum ← sum - x_old + x_new
μ ← sum / n
```

### 2. 样本标准差（无偏估计）

使用样本无偏方差公式（Bessel校正）：

```
σ² = Σ(xᵢ - μ)² / (n - 1)
```

利用平方和加速计算：

```
Σ(xᵢ - μ)² = Σxᵢ² - (Σxᵢ)² / n

σ² = (Σxᵢ²/n - μ²) * n/(n-1)

σ = √σ²
```

增量更新时同步维护 `sum` 和 `sumSq` 两个累加变量，避免每次重算整个窗口。

### 3. 异常判断规则

设阈值倍数为 k（StdDevFactor），当前值为 x：

**双向检测（DirectionBoth）：**

```
若 |x - μ| > k * σ  →  异常
  x - μ > 0  →  上偏（DirectionUp）
  否则        →  下偏（DirectionDown）
```

**仅上偏检测（DirectionUp）：**

```
若 x - μ > k * σ  →  上偏异常
```

**仅下偏检测（DirectionDown）：**

```
若 x - μ < -k * σ  →  下偏异常
```

### 4. 严重程度分级

计算偏离倍数 r = |x - μ| / σ（σ = 0 时回退到 |x - μ| / |μ|，μ = 0 时记为 0）

```
若 r >= 2k  →  严重（SeverityCritical）
若 k ≤ r < 2k →  警告（SeverityWarning）
```

### 5. 季节性周期模型

季节性周期索引基于**数据点时间戳**计算，而非数据点的序列序号，确保时间间隔不均匀时季节性基线学习依然有效。

设周期长度（槽位数）为 P，每个槽位的时间跨度为 S（PeriodSlot），周期基准时间为 E（SeasonalEpoch）。对于任意时间戳 T，其对应的周期槽位索引计算方式为：

```
totalSlots = floor( (T - E) / S )   // 计算从基准时间起经过的完整槽位数
idx = totalSlots mod P              // 对周期长度取模得到周期位置
if idx < 0: idx += P                // 处理时间戳早于基准时间的情况
```

每个周期索引 idx 维护独立的 μ_idx 和 σ_idx，检测时间戳 T 对应的数据点时使用 μ_idx 和 σ_idx 而非全局统计量，从而消除周期性波动对检测结果的干扰。

典型应用场景：
- 日周期（小时粒度）：P=24, S=1小时, E=某日 00:00 → 每个小时对应一个槽位（0-23）
- 周周期（日粒度）：P=7, S=1天, E=某周一 → 周一=0, 周二=1, ..., 周日=6
- 业务周期：P=自定义长度，S=自定义粒度

注意事项：
- PeriodSlot 的选择应与数据采集频率匹配（如每分钟采集数据建议用 1分钟作为 PeriodSlot）
- SeasonalEpoch 建议选择周期的自然对齐点（如一天的 00:00、一周的周一）
- 同一检测器的所有数据点必须使用一致的 PeriodSlot 和 SeasonalEpoch，配置变更时会自动执行基线迁移

## 错误定义

| 错误 | 说明 |
|------|------|
| `ErrInvalidWindowSize` | 窗口大小无效（<= 0） |
| `ErrInvalidStdDevFactor` | 标准差倍数阈值无效（< 0） |
| `ErrInvalidPeriodLength` | 季节周期长度无效（启用季节模式时 <= 0） |
| `ErrInvalidPeriodSlot` | 周期槽位时间跨度无效（启用季节模式时 <= 0） |
| `ErrInvalidMinSamples` | 最小样本数无效（<= 0 或 > WindowSize） |
| `ErrNilDataPoint` | 数据点为 nil |
| `ErrDetectorClosed` | 检测器已关闭 |
| `ErrInvalidDirection` | 偏离方向枚举值无效 |

## 配置变更基线迁移策略

当调用 `UpdateConfig` 调整季节性相关配置时，检测器会根据以下策略智能迁移已积累的基线数据，尽可能避免回到冷启动状态：

### 场景 1：非季节性 → 非季节性（EnableSeasonal 均为 false）
- 行为：仅更新配置字段，全局基线数据（窗口、样本数、均值、标准差）完全保留。
- 影响：无数据丢失，检测连续。

### 场景 2：季节性 → 非季节性（EnableSeasonal 从 true 变为 false）
- 行为：将所有周期槽位的窗口统计量（sum、sumSq、values）按顺序合并到全局基线窗口中，合并后窗口大小仍受 WindowSize 限制（超出则淘汰最旧值）。
- 影响：全局基线获得更丰富的历史数据，季节性基线被清除。

### 场景 3：非季节性 → 季节性（EnableSeasonal 从 false 变为 true）
- 行为：将全局窗口中的历史值按索引轮询方式分配到各个新周期槽位中（第 1 个值 → 槽位 0，第 2 个 → 槽位 1，...，第 P 个 → 槽位 P-1，循环往复）。
- 影响：每个周期槽位获得部分预热数据，加速冷启动过程；但由于分配是按顺序而非真实周期位置，初期检测准确率略低于完整学习。

### 场景 4：季节性 → 季节性（PeriodLength、PeriodSlot、SeasonalEpoch 均未变化）
- 行为：逐槽位深拷贝旧的窗口统计量（sum、sumSq、values 链表）到新结构中。
- 影响：所有基线数据 100% 保留，检测完全连续，仅 WindowSize 等其他配置生效。

### 场景 5：季节性 → 季节性（周期相关参数发生变化）
- 行为：采用**一一索引映射策略**保证不同周期相位的数据不被错误共享。具体规则：
  1. 对每个旧槽位 oldIdx，计算其在新周期中的映射位置 newIdx = oldIdx % newPeriodLength。
  2. 将旧槽位的统计量（sum、sumSq、values）合并到 newSeasonal[newIdx] 中。
  3. 每个旧槽位只映射到**唯一**一个新槽位，不会复制到多个槽位。
  4. 合并时仍遵守 WindowSize 限制，超出窗口大小的值会被淘汰。
  5. 如果 PeriodSlot 或 SeasonalEpoch 发生变化，所有槽位数据仍按索引映射（注意：此时时间戳到索引的计算方式已改变，旧数据的周期位置可能与新配置不完全对齐）。

**示例**（PeriodLength 从 4 变为 6）：
- 旧槽位 0 → 新槽位 0（0 % 6 = 0）
- 旧槽位 1 → 新槽位 1（1 % 6 = 1）
- 旧槽位 2 → 新槽位 2（2 % 6 = 2）
- 旧槽位 3 → 新槽位 3（3 % 6 = 3）
- 新槽位 4、5 为空（无旧数据映射）
- 总样本数保持不变，无数据复制或丢失

迁移策略的设计原则：**数据完整性优先于覆盖率，不同相位的数据绝不共享**。任何场景下都不会简单丢弃已积累的全部基线数据，也不会将一个相位的数据扩散到多个相位。

## 并发安全设计

`Detector` 使用 `sync.RWMutex` 保证并发安全：

- **读操作**（GetBaseline、GetSeasonalBaseline、GetAnomalies、AnomalyCount、PointCount、Config、IsClosed）使用读锁 `RLock/RUnlock`，支持多读并发
- **写操作**（Add、BatchAdd、Reset、Close、UpdateConfig）使用写锁 `Lock/Unlock`，保证互斥访问
- Add 方法中先检测再更新基线的逻辑在同一写锁保护下完成，保证检测的原子性

## 使用示例

### 基本使用：全局基线异常检测

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/tsanomaly"
)

func main() {
    cfg := tsanomaly.Config{
        WindowSize:        100,
        StdDevFactor:      3.0,
        MinSamples:        20,
        Direction:         tsanomaly.DirectionBoth,
        MaxAnomalyHistory: 1000,
    }

    detector, err := tsanomaly.NewDetector(cfg)
    if err != nil {
        panic(err)
    }

    baseTime := time.Now()

    // 添加20个正常值（预热期，不检测）
    for i := 0; i < 20; i++ {
        point := &tsanomaly.DataPoint{
            Timestamp: baseTime.Add(time.Duration(i) * time.Second),
            Value:     100.0 + 5.0*float64(i%10-5),
        }
        event, _ := detector.Add(point)
        if event != nil {
            fmt.Printf("Warmup phase should not detect anomaly\n")
        }
    }

    // 继续添加正常值
    for i := 20; i < 40; i++ {
        point := &tsanomaly.DataPoint{
            Timestamp: baseTime.Add(time.Duration(i) * time.Second),
            Value:     100.0 + 5.0*float64(i%10-5),
        }
        event, _ := detector.Add(point)
        if event != nil {
            fmt.Printf("Normal value detected as anomaly? value=%.2f\n", point.Value)
        }
    }

    // 添加上偏异常值
    anomalyPoint := &tsanomaly.DataPoint{
        Timestamp: baseTime.Add(40 * time.Second),
        Value:     200.0,
    }
    event, _ := detector.Add(anomalyPoint)
    if event != nil {
        fmt.Printf("✅ 检测到异常:\n")
        fmt.Printf("  时间: %v\n", event.Timestamp.Format("15:04:05"))
        fmt.Printf("  实际值: %.2f, 基线: %.2f\n", event.ActualValue, event.BaselineValue)
        fmt.Printf("  偏离: %.2f (%.1fσ), 阈值: %.2f\n",
            event.Deviation, event.DeviationRatio, event.Threshold)
        fmt.Printf("  方向: %v, 严重程度: %v\n", event.Direction, event.Severity)
    }

    // 查询基线统计
    mean, stdDev, count := detector.GetBaseline()
    fmt.Printf("\n📊 当前基线: μ=%.2f, σ=%.2f, 样本数=%d\n", mean, stdDev, count)
}
```

### 季节性模式：按小时检测日周期数据

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/tsanomaly"
)

func main() {
    epoch := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    cfg := tsanomaly.Config{
        WindowSize:        30,               // 每个时段保留30天数据
        StdDevFactor:      2.5,
        MinSamples:        5,                // 至少5个同周期点后才检测
        EnableSeasonal:    true,
        PeriodLength:      24,               // 24小时日周期（24个槽位）
        PeriodSlot:        time.Hour,        // 每个槽位代表1小时
        SeasonalEpoch:     epoch,            // 周期基准时间（对齐到某日零点）
        Direction:         tsanomaly.DirectionBoth,
        MaxAnomalyHistory: 500,
    }

    detector, _ := tsanomaly.NewDetector(cfg)

    // 模拟10天的每小时流量数据（早高峰和晚高峰模式）
    baseTime := epoch
    for day := 0; day < 10; day++ {
        for hour := 0; hour < 24; hour++ {
            var baseFlow float64
            switch {
            case hour >= 8 && hour <= 10:
                baseFlow = 5000 // 早高峰
            case hour >= 18 && hour <= 20:
                baseFlow = 6000 // 晚高峰
            case hour >= 0 && hour <= 5:
                baseFlow = 500  // 凌晨低谷
            default:
                baseFlow = 2000 // 平时
            }
            // 添加一些随机波动
            flow := baseFlow + 100.0*float64(day%5-2)

            point := &tsanomaly.DataPoint{
                Timestamp: baseTime.Add(time.Duration(day*24+hour) * time.Hour),
                Value:     flow,
            }
            _, _ = detector.Add(point)
        }
    }

    // 第11天早上9点发生异常：流量暴跌
    crashPoint := &tsanomaly.DataPoint{
        Timestamp: baseTime.Add(240 * time.Hour).Add(9 * time.Hour),
        Value:     100.0, // 正常应≈5000
    }
    event, _ := detector.Add(crashPoint)

    if event != nil {
        fmt.Printf("✅ 季节性异常检测成功:\n")
        fmt.Printf("  周期索引（小时）: %d\n", event.SeasonalIndex)
        fmt.Printf("  该时段基线: %.0f, 实际值: %.0f\n", event.BaselineValue, event.ActualValue)
        fmt.Printf("  使用周期 %d 号位置独立基线而非全局基线\n", event.SeasonalIndex)
    }

    // 查看各时段基线
    for i := 0; i < 24; i += 6 {
        mean, stdDev, count, _ := detector.GetSeasonalBaseline(i)
        fmt.Printf("  时段%02d:00 - 基线=%.0f, σ=%.0f, 样本=%d\n", i, mean, stdDev, count)
    }
}
```

### 偏离方向过滤：仅检测超卖（上偏）

```go
// 监控库存，仅检测库存异常增加（可能是误操作导致的超卖）
cfg := tsanomaly.Config{
    WindowSize:        50,
    StdDevFactor:      2.0,
    MinSamples:        10,
    Direction:         tsanomaly.DirectionUp, // 仅关注上偏
    MaxAnomalyHistory: 100,
}
detector, _ := tsanomaly.NewDetector(cfg)

// 历史正常出库数据
baseTime := time.Now()
for i := 0; i < 30; i++ {
    point := &tsanomaly.DataPoint{
        Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
        Value:     float64(50 + i%20),
    }
    _, _ = detector.Add(point)
}

// 一次超卖：出库量骤增
oversell := &tsanomaly.DataPoint{
    Timestamp: baseTime.Add(30 * time.Minute),
    Value:     500.0,
}
event, _ := detector.Add(oversell)
if event != nil && event.Direction == tsanomaly.DirectionUp {
    fmt.Println("⚠️  检测到超卖事件！出库量:", event.ActualValue)
}

// 一次数据缺失：出库量骤降（不会被DirectionUp模式检测）
missing := &tsanomaly.DataPoint{
    Timestamp: baseTime.Add(31 * time.Minute),
    Value:     0.0,
}
event, _ = detector.Add(missing)
if event == nil {
    fmt.Println("说明：DirectionUp模式不会触发下偏异常")
}
```

### 批量添加与异常查询

```go
detector, _ := tsanomaly.NewDetector(tsanomaly.DefaultConfig())

baseTime := time.Now()
batch := make([]*tsanomaly.DataPoint, 100)
for i := 0; i < 100; i++ {
    v := 100.0
    if i == 77 {
        v = 999.0 // 注入异常
    }
    if i == 88 {
        v = 888.0 // 注入异常
    }
    batch[i] = &tsanomaly.DataPoint{
        Timestamp: baseTime.Add(time.Duration(i) * time.Second),
        Value:     v,
    }
}

// 批量添加
events, err := detector.BatchAdd(batch)
if err != nil {
    panic(err)
}
fmt.Printf("批量添加100条数据，检测到 %d 个异常\n", len(events))

// 查询最近的严重异常
critical := tsanomaly.SeverityCritical
query := &tsanomaly.AnomalyQuery{
    Severity: &critical,
    Limit:    10,
}
result := detector.GetAnomalies(query)
fmt.Printf("严重级别异常共 %d 条\n", len(result))

// 按时间范围查询
start := baseTime.Add(70 * time.Second)
end := baseTime.Add(90 * time.Second)
rangeQuery := &tsanomaly.AnomalyQuery{
    StartTime: &start,
    EndTime:   &end,
}
rangeResult := detector.GetAnomalies(rangeQuery)
fmt.Printf("时间范围内异常共 %d 条\n", len(rangeResult))

// 查看统计
fmt.Printf("总数据点: %d, 总异常: %d\n", detector.PointCount(), detector.AnomalyCount())
```

### 完整示例：实时服务指标监控

```go
package main

import (
    "fmt"
    "sync"
    "time"
    "solocoder-go/internal/tsanomaly"
)

type ServiceMonitor struct {
    latencyDetector *tsanomaly.Detector
    qpsDetector     *tsanomaly.Detector
    mu              sync.Mutex
}

func NewServiceMonitor() *ServiceMonitor {
    latencyCfg := tsanomaly.Config{
        WindowSize:        200,
        StdDevFactor:      2.5,
        MinSamples:        30,
        Direction:         tsanomaly.DirectionUp, // 延迟只关注升高
        MaxAnomalyHistory: 500,
    }
    qpsCfg := tsanomaly.Config{
        WindowSize:        100,
        StdDevFactor:      2.0,
        MinSamples:        20,
        Direction:         tsanomaly.DirectionBoth, // QPS关注双向
        EnableSeasonal:    true,
        PeriodLength:      24,                      // 小时粒度日周期（24个槽位）
        PeriodSlot:        time.Hour,               // 每个槽位代表1小时
        SeasonalEpoch:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
        MaxAnomalyHistory: 500,
    }

    latencyDet, _ := tsanomaly.NewDetector(latencyCfg)
    qpsDet, _ := tsanomaly.NewDetector(qpsCfg)

    return &ServiceMonitor{
        latencyDetector: latencyDet,
        qpsDetector:     qpsDet,
    }
}

func (m *ServiceMonitor) ReportMetrics(ts time.Time, latencyMs float64, qps float64) {
    m.mu.Lock()
    defer m.mu.Unlock()

    latEvent, _ := m.latencyDetector.Add(&tsanomaly.DataPoint{
        Timestamp: ts, Value: latencyMs,
    })
    qpsEvent, _ := m.qpsDetector.Add(&tsanomaly.DataPoint{
        Timestamp: ts, Value: qps,
    })

    if latEvent != nil {
        fmt.Printf("[ALERT] 延迟异常: %.0fms (基线%.0fms), 程度=%v\n",
            latEvent.ActualValue, latEvent.BaselineValue, latEvent.Severity)
    }
    if qpsEvent != nil {
        dir := "升高"
        if qpsEvent.Direction == tsanomaly.DirectionDown {
            dir = "下降"
        }
        fmt.Printf("[ALERT] QPS%s: %.0f (基线%.0f), 时段%02d点\n",
            dir, qpsEvent.ActualValue, qpsEvent.BaselineValue, qpsEvent.SeasonalIndex)
    }
}

func (m *ServiceMonitor) Status() {
    lMean, lStd, _ := m.latencyDetector.GetBaseline()
    fmt.Printf("延迟基线: %.1fms ± %.1fms\n", lMean, lStd)

    down := tsanomaly.DirectionDown
    recentDrops := m.qpsDetector.GetAnomalies(&tsanomaly.AnomalyQuery{
        Direction: &down,
        Limit:     5,
    })
    fmt.Printf("最近5次QPS下跌事件: %d 条\n", len(recentDrops))
}

func main() {
    monitor := NewServiceMonitor()
    now := time.Now()

    // 模拟100个时间片的正常数据
    for i := 0; i < 100; i++ {
        ts := now.Add(time.Duration(i) * time.Minute)
        baseLatency := 50.0 + 10.0*float64(i%20-10)
        baseQPS := 1000.0 + 200.0*float64(i%24-12)
        monitor.ReportMetrics(ts, baseLatency, baseQPS)
    }

    // 模拟延迟激增
    monitor.ReportMetrics(now.Add(100*time.Minute), 300.0, 980.0)
    // 模拟QPS暴跌
    monitor.ReportMetrics(now.Add(101*time.Minute), 55.0, 100.0)

    monitor.Status()
}
```
