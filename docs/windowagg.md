# windowagg - 滑动窗口聚合器模块

## 模块功能

`windowagg` 提供了一套灵活的滑动窗口数据聚合机制，支持以下核心功能：

1. **五种聚合算子**：提供计数、求和、平均值、最大值、最小值五种聚合算子，每个算子实现统一的聚合接口，支持对窗口内的数据项执行对应的聚合计算并返回结果。

2. **灵活的窗口配置**：支持基于时间或基于计数的滑动窗口，窗口大小和滑动步长均可独立配置：
   - 当步长小于窗口大小时，相邻窗口存在重叠数据
   - 当步长等于窗口大小时，窗口不重叠（滚动窗口）
   - 步长不能大于窗口大小

3. **增量式窗口数据管理**：窗口内的数据随着窗口滑动而增删，新数据到达时加入当前窗口，数据滑出窗口时从聚合状态中移除，聚合结果随窗口滑动持续更新而非每次都重新计算整个窗口。

4. **多窗口并行聚合**：支持同时维护多个不同配置的滑动窗口对同一数据流进行聚合，每个窗口独立维护自己的数据视图和聚合状态，每个窗口的配置包括窗口大小、步长和聚合算子类型三者均可独立设置。

## 核心结构体与接口

### Aggregator 接口

聚合算子统一接口，定义了所有聚合算子必须实现的方法。

```go
type Aggregator interface {
    Add(value float64)
    Remove(value float64)
    Result() (float64, error)
    Reset()
    Name() string
}
```

| 方法 | 说明 |
|------|------|
| `Add(value float64)` | 向聚合器添加一个数据点 |
| `Remove(value float64)` | 从聚合器移除一个数据点（用于窗口滑动时的增量更新） |
| `Result() (float64, error)` | 返回当前聚合结果，空窗口时部分算子返回错误 |
| `Reset()` | 重置聚合器到初始状态 |
| `Name() string` | 返回聚合算子的名称 |

### AggregatorType

聚合算子类型枚举。

| 常量 | 值 | 说明 |
|------|----|------|
| `AggregatorCount` | 0 | 计数算子，统计窗口内数据点数量 |
| `AggregatorSum` | 1 | 求和算子，计算窗口内数据点之和 |
| `AggregatorAvg` | 2 | 平均值算子，计算窗口内数据点的算术平均值 |
| `AggregatorMax` | 3 | 最大值算子，返回窗口内数据点的最大值 |
| `AggregatorMin` | 4 | 最小值算子，返回窗口内数据点的最小值 |

方法：

| 方法 | 说明 |
|------|------|
| `String() string` | 返回算子类型的字符串表示 |

### 五种聚合算子实现

#### CountAggregator

计数聚合算子，统计窗口内数据点的数量。

- 空窗口结果：返回 `0`，无错误
- 增量特性：添加时计数 +1，移除时计数 -1（不会下溢到负数）

#### SumAggregator

求和聚合算子，计算窗口内所有数据点的累加和。

- 空窗口结果：返回 `0`，无错误
- 增量特性：添加时累加值，移除时减去值

#### AvgAggregator

平均值聚合算子，计算窗口内数据点的算术平均值。

- 空窗口结果：返回 `0` 和 `ErrEmptyWindow` 错误
- 增量特性：同时维护累加和与计数，结果为 `sum / count`

#### MaxAggregator

最大值聚合算子，返回窗口内数据点的最大值。

- 空窗口结果：返回 `0` 和 `ErrEmptyWindow` 错误
- 增量特性：添加时若新值大于当前最大值则更新；移除时若移走的是当前最大值则重新扫描

#### MinAggregator

最小值聚合算子，返回窗口内数据点的最小值。

- 空窗口结果：返回 `0` 和 `ErrEmptyWindow` 错误
- 增量特性：添加时若新值小于当前最小值则更新；移除时若移走的是当前最小值则重新扫描

### WindowType

窗口类型枚举。

| 常量 | 值 | 说明 |
|------|----|------|
| `WindowTypeCount` | 0 | 基于计数的滑动窗口，按数据点数量划分窗口 |
| `WindowTypeTime` | 1 | 基于时间的滑动窗口，按时间范围划分窗口 |

### WindowConfig

滑动窗口配置结构体。

| 字段 | 类型 | 说明 |
|------|------|------|
| `WindowType` | WindowType | 窗口类型（基于计数或基于时间） |
| `AggregatorType` | AggregatorType | 聚合算子类型 |
| `Size` | int64 | 窗口大小：计数窗口为数据点数量，时间窗口为毫秒数 |
| `Slide` | int64 | 滑动步长：计数窗口为数据点数量，时间窗口为毫秒数 |

配置约束：
- `Size` 必须大于 0
- `Slide` 必须大于 0
- `Slide` 不能大于 `Size`

### SlidingWindow

滑动窗口实例，管理单个窗口的数据和聚合状态。

主要方法：

| 方法 | 说明 |
|------|------|
| `NewSlidingWindow(name string, cfg WindowConfig) (*SlidingWindow, error)` | 创建新的滑动窗口 |
| `Name() string` | 返回窗口名称 |
| `Config() WindowConfig` | 返回窗口配置 |
| `AddValue(value float64, timestamp time.Time)` | 向窗口添加一个数据点（自动分配序号） |
| `AddValueWithSeq(value float64, timestamp time.Time, seq int64)` | 向窗口添加一个指定序号的数据点 |
| `Result() (float64, error)` | 获取当前窗口的聚合结果 |
| `Count() int` | 获取当前窗口内的数据点数量 |
| `Reset()` | 重置窗口到初始空状态 |

### WindowManager

多窗口管理器，支持同时维护多个不同配置的滑动窗口对同一数据流进行聚合。

主要方法：

| 方法 | 说明 |
|------|------|
| `NewWindowManager() *WindowManager` | 创建新的窗口管理器 |
| `AddWindow(name string, cfg WindowConfig) error` | 添加一个滑动窗口 |
| `RemoveWindow(name string) error` | 移除一个滑动窗口 |
| `GetWindow(name string) (*SlidingWindow, error)` | 获取指定名称的窗口 |
| `WindowCount() int` | 获取已注册窗口数量 |
| `Push(value float64, timestamp time.Time)` | 向所有窗口推送一个数据点 |
| `PushWithSeq(value float64, timestamp time.Time, seq int64)` | 向所有窗口推送一个指定序号的数据点 |
| `GetResult(name string) (float64, error)` | 获取指定窗口的聚合结果 |
| `GetAllResults() map[string]float64` | 获取所有窗口的聚合结果（跳过空窗口） |
| `Reset()` | 重置所有窗口 |
| `ResetWindow(name string) error` | 重置指定窗口 |

## 五种聚合算子语义说明

### 1. 计数 (Count)

统计窗口内包含的数据点数量。

- **语义**：`count = 窗口内数据点总数`
- **空窗口**：返回 0
- **适用场景**：统计事件频率、QPS、请求数等

### 2. 求和 (Sum)

计算窗口内所有数据点数值的累加和。

- **语义**：`sum = Σ(数据点数值)`
- **空窗口**：返回 0
- **适用场景**：统计总销售额、总流量、总收入等

### 3. 平均值 (Avg)

计算窗口内所有数据点数值的算术平均值。

- **语义**：`avg = Σ(数据点数值) / 数据点数量`
- **空窗口**：返回 ErrEmptyWindow 错误
- **适用场景**：平均响应时间、平均温度、平均价格等

### 4. 最大值 (Max)

返回窗口内所有数据点数值中的最大值。

- **语义**：`max = max(数据点数值集合)`
- **空窗口**：返回 ErrEmptyWindow 错误
- **适用场景**：峰值流量、最高温度、最大延迟等

### 5. 最小值 (Min)

返回窗口内所有数据点数值中的最小值。

- **语义**：`min = min(数据点数值集合)`
- **空窗口**：返回 ErrEmptyWindow 错误
- **适用场景**：最低温度、最小延迟、谷值流量等

## 滑动窗口语义说明

### 基于计数的滑动窗口

窗口大小和步长以数据点数量为单位。

#### 不重叠窗口（步长 = 窗口大小）

当 `Slide == Size` 时，窗口不重叠，形成滚动窗口：
- 数据点 1, 2, 3 → 窗口 1（包含 1, 2, 3）
- 数据点 4 → 窗口 2（仅包含 4，窗口 1 的数据全部滑出）

#### 重叠窗口（步长 < 窗口大小）

当 `Slide < Size` 时，相邻窗口存在重叠数据：
- 假设 Size=4, Slide=2
- 数据点 1, 2, 3, 4 → 窗口包含 [1, 2, 3, 4]
- 数据点 5 → 滑出 [1, 2]，窗口包含 [3, 4, 5]
- 数据点 6 → 窗口包含 [3, 4, 5, 6]

### 基于时间的滑动窗口

窗口大小和步长以毫秒为单位，窗口保留最近 `Size` 毫秒内的数据点。每当新数据到达时，自动移除时间戳早于 `当前时间 - Size` 的数据点。

## 错误定义

| 错误 | 说明 |
|------|------|
| `ErrInvalidWindowSize` | 窗口大小无效（<= 0） |
| `ErrInvalidSlideSize` | 滑动步长无效（<= 0） |
| `ErrSlideGreaterThanWindow` | 滑动步长大于窗口大小 |
| `ErrWindowNotFound` | 指定名称的窗口不存在 |
| `ErrWindowExists` | 指定名称的窗口已存在 |
| `ErrInvalidWindowType` | 无效的窗口类型 |
| `ErrInvalidAggregator` | 无效的聚合算子类型 |
| `ErrEmptyWindow` | 窗口为空（Avg/Max/Min 算子） |

## 并发安全设计

`SlidingWindow` 和 `WindowManager` 均使用 `sync.RWMutex` 保证并发安全：

- 读操作（Result、Count、GetWindow、GetResult 等）使用读锁
- 写操作（AddValue、Push、Reset 等）使用写锁
- WindowManager 在向所有窗口推送数据时，先获取窗口列表快照，然后在锁外执行推送，减少锁竞争

## 使用示例

### 基本使用：单个滑动窗口

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/windowagg"
)

func main() {
    // 创建一个基于计数的求和滑动窗口
    // 窗口大小：5 个数据点，滑动步长：5 个数据点（不重叠）
    w, err := windowagg.NewSlidingWindow("sales-sum", windowagg.WindowConfig{
        WindowType:     windowagg.WindowTypeCount,
        AggregatorType: windowagg.AggregatorSum,
        Size:           5,
        Slide:          5,
    })
    if err != nil {
        panic(err)
    }

    now := time.Now()

    // 添加数据
    for i := 1; i <= 5; i++ {
        w.AddValue(float64(i), now)
    }

    // 获取聚合结果
    result, _ := w.Result()
    fmt.Printf("Sum of first 5 numbers: %.0f\n", result) // 15 (1+2+3+4+5)

    // 添加新数据，旧数据滑出
    w.AddValue(10.0, now)

    result, _ = w.Result()
    fmt.Printf("Sum after slide: %.0f\n", result) // 10 (仅包含最新数据点)
}
```

### 重叠窗口：滑动平均值

```go
// 创建一个重叠的平均值窗口
// 窗口大小：4 个数据点，滑动步长：2 个数据点（50% 重叠）
w, _ := windowagg.NewSlidingWindow("moving-avg", windowagg.WindowConfig{
    WindowType:     windowagg.WindowTypeCount,
    AggregatorType: windowagg.AggregatorAvg,
    Size:           4,
    Slide:          2,
})

now := time.Now()
w.AddValue(1.0, now)
w.AddValue(2.0, now)
w.AddValue(3.0, now)
w.AddValue(4.0, now)

fmt.Printf("Count: %d\n", w.Count()) // 4
result, _ := w.Result()
fmt.Printf("Avg: %.2f\n", result) // 2.50

w.AddValue(5.0, now)
fmt.Printf("Count: %d\n", w.Count()) // 3 (滑出了 1,2)
result, _ = w.Result()
fmt.Printf("Avg: %.2f\n", result) // 4.00 (3+4+5)/3
```

### 基于时间的窗口

```go
// 创建基于时间的最大值窗口，保留最近 100ms 内的数据
w, _ := windowagg.NewSlidingWindow("max-100ms", windowagg.WindowConfig{
    WindowType:     windowagg.WindowTypeTime,
    AggregatorType: windowagg.AggregatorMax,
    Size:           100,
    Slide:          50,
})

baseTime := time.Now()
w.AddValue(10.0, baseTime)
w.AddValue(30.0, baseTime.Add(20*time.Millisecond))
w.AddValue(20.0, baseTime.Add(50*time.Millisecond))

result, _ := w.Result()
fmt.Printf("Max: %.0f\n", result) // 30

// 200ms 后，旧数据过期
w.AddValue(15.0, baseTime.Add(200*time.Millisecond))
result, _ = w.Result()
fmt.Printf("Max: %.0f\n", result) // 15
```

### 多窗口并行聚合

```go
// 创建窗口管理器
mgr := windowagg.NewWindowManager()

// 添加多个不同配置的窗口
mgr.AddWindow("sum-5", windowagg.WindowConfig{
    WindowType:     windowagg.WindowTypeCount,
    AggregatorType: windowagg.AggregatorSum,
    Size:           5,
    Slide:          5,
})

mgr.AddWindow("avg-3", windowagg.WindowConfig{
    WindowType:     windowagg.WindowTypeCount,
    AggregatorType: windowagg.AggregatorAvg,
    Size:           3,
    Slide:          1,
})

mgr.AddWindow("count-10", windowagg.WindowConfig{
    WindowType:     windowagg.WindowTypeCount,
    AggregatorType: windowagg.AggregatorCount,
    Size:           10,
    Slide:          10,
})

// 向所有窗口同时推送数据
now := time.Now()
for i := 1; i <= 5; i++ {
    mgr.Push(float64(i), now)
}

// 获取各个窗口的聚合结果
sumResult, _ := mgr.GetResult("sum-5")
fmt.Printf("Sum (5): %.0f\n", sumResult) // 15

avgResult, _ := mgr.GetResult("avg-3")
fmt.Printf("Avg (3): %.2f\n", avgResult) // 4.00 (3+4+5)/3

countResult, _ := mgr.GetResult("count-10")
fmt.Printf("Count (10): %.0f\n", countResult) // 5

// 一次性获取所有窗口结果
allResults := mgr.GetAllResults()
for name, val := range allResults {
    fmt.Printf("  %s: %.2f\n", name, val)
}
```

### 使用聚合算子工厂

```go
// 通过工厂函数创建聚合算子
agg, err := windowagg.NewAggregator(windowagg.AggregatorMin)
if err != nil {
    panic(err)
}

agg.Add(5.0)
agg.Add(2.0)
agg.Add(8.0)

result, _ := agg.Result()
fmt.Printf("Min: %.0f\n", result) // 2

agg.Remove(2.0)
result, _ = agg.Result()
fmt.Printf("Min after remove: %.0f\n", result) // 5

fmt.Printf("Aggregator name: %s\n", agg.Name()) // min
```

### 完整示例：实时指标监控

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/windowagg"
)

func main() {
    mgr := windowagg.NewWindowManager()

    // 1 分钟请求计数窗口
    mgr.AddWindow("qpm", windowagg.WindowConfig{
        WindowType:     windowagg.WindowTypeTime,
        AggregatorType: windowagg.AggregatorCount,
        Size:           60000, // 60s
        Slide:          1000,  // 1s
    })

    // 5 秒平均延迟窗口
    mgr.AddWindow("avg-latency", windowagg.WindowConfig{
        WindowType:     windowagg.WindowTypeTime,
        AggregatorType: windowagg.AggregatorAvg,
        Size:           5000,  // 5s
        Slide:          1000,  // 1s
    })

    // 10 秒最大延迟窗口
    mgr.AddWindow("max-latency", windowagg.WindowConfig{
        WindowType:     windowagg.WindowTypeTime,
        AggregatorType: windowagg.AggregatorMax,
        Size:           10000, // 10s
        Slide:          1000,  // 1s
    })

    // 模拟数据流
    baseTime := time.Now()
    latencies := []float64{15.0, 22.0, 18.0, 35.0, 28.0, 42.0, 19.0, 25.0}

    for i, lat := range latencies {
        ts := baseTime.Add(time.Duration(i*500) * time.Millisecond)
        mgr.Push(lat, ts)

        qpm, _ := mgr.GetResult("qpm")
        avgLat, _ := mgr.GetResult("avg-latency")
        maxLat, _ := mgr.GetResult("max-latency")

        fmt.Printf("[t=%dms] QPM=%.0f, AvgLat=%.1fms, MaxLat=%.1fms\n",
            i*500, qpm, avgLat, maxLat)
    }
}
```
