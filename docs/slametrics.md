# SLA 指标计算器模块

## 模块概述

`internal/slametrics` 包提供了一个完整的 SLA（服务等级协议）指标计算和监控功能模块，支持可用性百分比计算、延迟百分位统计（P50/P90/P99）、错误率统计、SLA 达标判定以及违约事件的记录与查询。

## 核心结构体职责

### SLAMetrics（SLA 指标计算器）

SLAMetrics 是模块的核心结构体，负责存储请求记录、计算各类 SLA 指标、执行 SLA 达标判定以及管理违约事件。

**主要职责：**
- 存储请求记录（时间戳、成功状态、延迟、错误码）
- 按时间窗口过滤数据
- 计算可用性百分比
- 计算延迟百分位值（P50/P90/P99）
- 统计错误率（按错误类型分组）
- 执行 SLA 达标判定
- 记录和去重存储违约事件
- 提供查询接口

**并发安全：** 内部使用 `sync.RWMutex` 读写锁保护数据，支持并发读写。

### RequestRecord（请求记录）

表示一次请求的元数据记录。

| 字段 | 类型 | 说明 |
|------|------|------|
| Timestamp | time.Time | 请求发生时间 |
| Success | bool | 请求是否成功 |
| Latency | float64 | 请求延迟（单位由调用方约定，如毫秒） |
| ErrorKey | string | 错误码或错误类型标识，成功请求可留空 |

### AvailabilityResult（可用性计算结果）

可用性百分比计算的返回结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| TotalRequests | int | 时间窗口内总请求数 |
| SuccessRequests | int | 成功请求数 |
| FailedRequests | int | 失败请求数 |
| Availability | float64 | 可用性百分比（0-100） |

### LatencyPercentiles（延迟百分位统计结果）

延迟百分位计算的返回结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| P50 | float64 | 第 50 百分位延迟（中位数） |
| P90 | float64 | 第 90 百分位延迟 |
| P99 | float64 | 第 99 百分位延迟 |
| Count | int | 样本总数 |
| Min | float64 | 最小延迟 |
| Max | float64 | 最大延迟 |

### ErrorStat（单类错误统计）

按错误类型分组的统计数据。

| 字段 | 类型 | 说明 |
|------|------|------|
| Count | int | 该类型错误的数量 |
| ErrorRate | float64 | 该类型错误占总请求数的百分比 |

### ErrorRateResult（错误率计算结果）

错误率统计的完整返回结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| TotalRequests | int | 总请求数 |
| TotalErrors | int | 总错误数 |
| TotalErrorRate | float64 | 总错误率百分比 |
| ByErrorKey | map[string]ErrorStat | 按错误类型分组的统计 |

### SLAConfig（SLA 目标配置）

SLA 达标判定的目标阈值配置。

| 字段 | 类型 | 说明 |
|------|------|------|
| MinAvailability | float64 | 最低可用性要求（百分比），设为 0 表示不检查 |
| MaxP50Latency | float64 | P50 延迟上限，设为 0 表示不检查 |
| MaxP90Latency | float64 | P90 延迟上限，设为 0 表示不检查 |
| MaxP99Latency | float64 | P99 延迟上限，设为 0 表示不检查 |
| MaxTotalErrorRate | float64 | 总错误率上限（百分比），设为负值表示不检查 |

### ViolationDetail（违约详情）

单次 SLA 判定中某一项指标未达标的详情。

| 字段 | 类型 | 说明 |
|------|------|------|
| MetricName | string | 指标名称（如 availability、p99_latency、total_error_rate） |
| Actual | float64 | 实际值 |
| Target | float64 | 目标值 |

### SLAEvaluation（SLA 判定结果）

SLA 达标判定的完整返回结果。

| 字段 | 类型 | 说明 |
|------|------|------|
| WindowStart | time.Time | 时间窗口起始时间 |
| WindowEnd | time.Time | 时间窗口结束时间 |
| Compliant | bool | 是否全部达标 |
| Violations | []ViolationDetail | 未达标指标列表 |
| Availability | float64 | 实际可用性百分比 |
| LatencyStats | LatencyPercentiles | 实际延迟百分位数据 |
| ErrorStats | ErrorRateResult | 实际错误率数据 |

### ViolationEvent（违约事件）

持久化存储的违约事件记录。

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 事件唯一标识（由窗口时间+指标名生成，用于去重） |
| WindowStart | time.Time | 违约窗口起始时间 |
| WindowEnd | time.Time | 违约窗口结束时间 |
| MetricName | string | 未达标指标名称 |
| Actual | float64 | 实际值 |
| Target | float64 | 目标值 |
| RecordedAt | time.Time | 事件记录时间 |

### TimeWindow（时间窗口）

表示一个时间区间，包含起止时间（两端均包含）。

| 字段 | 类型 | 说明 |
|------|------|------|
| Start | time.Time | 窗口起始时间（包含） |
| End | time.Time | 窗口结束时间（包含） |

## 百分位计算方法

模块采用 **最近秩百分位法（Nearest Rank Method）** 计算百分位值，确保返回值始终是原始数据集中实际存在的元素之一。

### 算法原理

对于已排序的数据集（升序），计算第 p 百分位值的步骤：

1. 计算秩：`rank = ceil(p / 100 * n)`，其中 n 是样本总数
2. 返回排序后数组中第 rank 个元素（索引为 rank-1）

### 边界处理

- 当 p ≤ 0 时，返回数据集最小值
- 当 p ≥ 100 时，返回数据集最大值
- 当 rank < 1 时，取 rank = 1
- 当 rank > n 时，取 rank = n

### 算法示例

数据集：`[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]`（n=10）

- P50：rank = ceil(50/100 * 10) = ceil(5) = 5 → 返回第 5 个元素 = **5**
- P90：rank = ceil(90/100 * 10) = ceil(9) = 9 → 返回第 9 个元素 = **9**
- P99：rank = ceil(99/100 * 10) = ceil(9.9) = 10 → 返回第 10 个元素 = **10**

### 与插值法的区别

与线性插值百分位法不同，最近秩法不进行数值插值，始终返回数据集中的实际值。这在 SLA 场景中有以下优势：
- 结果真实可信，不产生数据集中不存在的"虚拟值"
- 便于与原始请求日志进行交叉验证
- 计算简单高效，无浮点精度损失

## SLA 达标判定流程

### 整体流程

```
1. 输入：时间窗口 [Start, End]、SLA 目标配置
2. 分别计算三项核心指标：
   a. 可用性百分比
   b. 延迟百分位（P50/P90/P99）
   c. 错误率统计
3. 将每项实际值与配置的目标阈值逐一比较：
   - 可用性：实际值 < 目标值 → 违约
   - 延迟分位：实际值 > 目标值（且目标值 > 0）→ 违约
   - 错误率：实际值 > 目标值（且目标值 ≥ 0）→ 违约
4. 任一指标不达标 → 整个窗口标记为 SLA 违约（Compliant = false）
5. 所有未达标指标生成违约详情，记录为违约事件（去重）
```

### 违约事件去重机制

违约事件使用复合键去重，键格式为：`{windowStartUnixNano}_{windowEndUnixNano}_{metricName}`

- 同一时间窗口的同一指标违约只会被记录一次
- 重复对同一窗口执行 SLA 判定不会产生重复的违约事件
- 不同指标的违约分别独立记录

### 违约事件排序

违约事件按 `RecordedAt`（记录时间）升序存储和返回，即先发生的违约排在前面。

## 使用示例

### 基本使用

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/slametrics"
)

func main() {
    // 创建 SLA 指标计算器
    sla := slametrics.NewSLAMetrics()

    // 记录请求数据
    base := time.Now()
    sla.RecordRequest(slametrics.RequestRecord{
        Timestamp: base,
        Success:   true,
        Latency:   23.5,
    })
    sla.RecordRequest(slametrics.RequestRecord{
        Timestamp: base.Add(10 * time.Millisecond),
        Success:   false,
        Latency:   150.0,
        ErrorKey:  "timeout",
    })
}
```

### 可用性计算

```go
window := slametrics.TimeWindow{
    Start: base,
    End:   base.Add(1 * time.Hour),
}

result, err := sla.CalculateAvailability(window, 2)
if err != nil {
    panic(err)
}
fmt.Printf("可用性: %.2f%%\n", result.Availability)
fmt.Printf("总请求: %d, 成功: %d, 失败: %d\n",
    result.TotalRequests, result.SuccessRequests, result.FailedRequests)
```

### 延迟百分位计算

```go
percentiles, err := sla.CalculateLatencyPercentiles(window)
if err != nil {
    panic(err)
}
fmt.Printf("P50: %.2fms, P90: %.2fms, P99: %.2fms\n",
    percentiles.P50, percentiles.P90, percentiles.P99)
fmt.Printf("样本数: %d, 最小: %.2f, 最大: %.2f\n",
    percentiles.Count, percentiles.Min, percentiles.Max)
```

### 错误率统计

```go
errors, err := sla.CalculateErrorRate(window, 2)
if err != nil {
    panic(err)
}
fmt.Printf("总错误率: %.2f%%\n", errors.TotalErrorRate)
for key, stat := range errors.ByErrorKey {
    fmt.Printf("  [%s] %d 次, 占比 %.2f%%\n", key, stat.Count, stat.ErrorRate)
}
```

### SLA 达标判定

```go
cfg := &slametrics.SLAConfig{
    MinAvailability:   99.9,
    MaxP50Latency:     50,
    MaxP90Latency:     100,
    MaxP99Latency:     300,
    MaxTotalErrorRate: 0.1,
}

eval, err := sla.EvaluateSLA(window, cfg, 2)
if err != nil {
    panic(err)
}

if eval.Compliant {
    fmt.Println("✅ SLA 全部达标")
} else {
    fmt.Println("❌ SLA 违约：")
    for _, v := range eval.Violations {
        fmt.Printf("  - %s: 实际 %.2f, 目标 %.2f\n",
            v.MetricName, v.Actual, v.Target)
    }
}
```

### 查询违约事件

```go
// 获取所有违约事件（按时间排序）
allEvents := sla.GetViolationEvents()
for _, e := range allEvents {
    fmt.Printf("[%s] %s: %.2f vs 目标 %.2f\n",
        e.RecordedAt.Format("15:04:05"),
        e.MetricName, e.Actual, e.Target)
}

// 获取指定时间范围内的违约事件
recent := sla.GetViolationEventsInRange(
    time.Now().Add(-24*time.Hour),
    time.Now(),
)
```

### 重置数据

```go
sla.Reset()  // 清空所有请求记录和违约事件
```

## 错误处理

模块使用显式 error 返回值处理所有异常情况：

| 错误变量 | 触发条件 |
|----------|----------|
| ErrNoRequests | 时间窗口内没有任何请求记录 |
| ErrNoLatencyData | 时间窗口内没有延迟数据 |
| ErrInvalidTimeRange | 时间窗口起始时间不早于结束时间 |
| ErrInvalidDecimalPlaces | 指定的小数位数为负数 |
| ErrInvalidPercentile | 百分位值超出有效范围（内部使用） |
| ErrEmptyErrorKey | 错误码为空（内部使用） |
| ErrNilSLAConfig | SLA 配置为 nil |
| ErrWindowNotFound | 窗口未找到（预留） |

## 并发安全

`SLAMetrics` 的所有公共方法均支持并发安全调用：

- 写入操作（RecordRequest、RecordRequests、Reset、EvaluateSLA）使用写锁
- 读取操作（CalculateAvailability、CalculateLatencyPercentiles、CalculateErrorRate、GetViolationEvents 等）使用读锁
- 读写锁允许多个读操作并发执行，写操作与所有其他操作互斥

## 测试

运行单元测试：

```bash
go test ./internal/slametrics/ -v
```

测试覆盖范围：
- 可用性计算：正常、100%、0%、无请求、小数位精度、时间过滤、边界校验
- 百分位计算：最近秩算法验证、所有结果均来自数据集、空数据、单元素、边界值
- 错误率统计：多类型错误、无错误、空错误键处理、无请求、边界校验
- SLA 判定：全达标、可用性违约、延迟违约、错误率违约、多指标同时违约、阈值边界（等于目标值不违约）
- 违约事件：去重验证、排序验证、范围查询、字段完整性
- 并发安全：多 goroutine 同时读写
- 数据重置：清空请求记录和违约事件
