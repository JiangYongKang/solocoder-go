# TSDB 时序数据库引擎模块

## 1. 模块概述

TSDB 是一个高性能的内存时序数据库引擎模块，专为时间序列数据的存储、查询、聚合和自动过期清理设计。模块提供了完整的时序数据写入、按时间范围查询、多维度标签索引、降采样聚合和 TTL 自动过期清理等功能，并通过读写锁机制实现安全的并发访问。

**包路径**: `internal/tsdb`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 时序数据写入 | 支持批量写入带标签的时序数据点，自动按时间戳排序存储 |
| 降采样聚合 | 支持按时间窗口对数据进行聚合，内置平均值、最大值、最小值、求和、计数等算子 |
| TTL 自动过期 | 支持配置数据生存时间，后台 goroutine 定期清理过期数据，支持配置清理间隔和批次大小 |
| 多维度标签索引 | 按标签键值建立倒排索引，支持单标签和多标签组合查询过滤 |
| 时间范围查询 | 支持按指定时间范围查询数据点，可结合标签过滤 |
| 并发安全 | 内置多把读写锁保护，支持多 goroutine 并发读写 |
| 数据完整性 | 写入时自动复制标签数据，查询时返回数据副本，避免外部修改影响内部状态 |

## 3. 核心结构体与职责

### 3.1 TSEngine

时序数据库引擎主结构体，对外提供所有操作接口。

```go
type TSEngine struct {
    data       []*DataPoint
    dataMu     sync.RWMutex
    tagIndex   map[string]map[string][]int
    tagIndexMu sync.RWMutex
    ttl        time.Duration
    cleanupInt time.Duration
    batchSize  int
    stopCh     chan struct{}
    closed     bool
    closedMu   sync.RWMutex
    wg         sync.WaitGroup
}
```

**职责**:
- 管理时序数据点切片（`data`），按时间戳升序存储
- 维护标签倒排索引（`tagIndex`），结构为 `tagKey -> tagValue -> []dataIndex`
- 管理 TTL 配置和后台清理 goroutine 生命周期
- 协调并发访问，通过多把读写锁保证线程安全
- 提供写入、查询、降采样、关闭等对外 API
- 后台 goroutine 通过 `stopCh` 接收停止信号，`wg` 用于等待清理协程退出

### 3.2 DataPoint

时序数据点结构体，存储单个时间点的指标数据。

```go
type DataPoint struct {
    Timestamp int64
    Value     float64
    Tags      map[string]string
}
```

**职责**:
- `Timestamp`: 毫秒级 Unix 时间戳，数据排序和时间范围查询的依据
- `Value`: 该时间点的指标数值，支持浮点数
- `Tags`: 多维度标签键值对，用于数据分类和索引查询

### 3.3 AggregatedPoint

降采样聚合结果点结构体。

```go
type AggregatedPoint struct {
    Timestamp int64
    Value     float64
    Count     int
}
```

**职责**:
- `Timestamp`: 聚合窗口的起始时间戳（与窗口大小对齐）
- `Value`: 聚合计算结果值，具体含义由聚合算子决定
- `Count`: 该窗口内包含的原始数据点数量

### 3.4 Config

引擎配置结构体。

```go
type Config struct {
    TTL              time.Duration
    CleanupInterval  time.Duration
    CleanupBatchSize int
}
```

**职责**:
- `TTL`: 数据生存时间，从数据点时间戳开始计算，超过此时间的数据将被清理。设置为负数表示禁用 TTL，0 表示使用默认值（24小时）
- `CleanupInterval`: 后台清理 goroutine 的运行间隔
- `CleanupBatchSize`: 每次清理操作处理的最大数据点数量，避免单次清理耗时过长

### 3.5 AggregatorType

聚合算子类型。

```go
type AggregatorType string

const (
    AggAvg   AggregatorType = "avg"
    AggMax   AggregatorType = "max"
    AggMin   AggregatorType = "min"
    AggSum   AggregatorType = "sum"
    AggCount AggregatorType = "count"
)
```

**职责**:
- 定义支持的聚合算子类型
- `AggAvg`: 计算窗口内数据的平均值
- `AggMax`: 计算窗口内数据的最大值
- `AggMin`: 计算窗口内数据的最小值
- `AggSum`: 计算窗口内数据的求和
- `AggCount`: 统计窗口内的数据点数量

## 4. 核心数据结构设计

### 4.1 数据存储结构

数据点按时间戳升序存储在切片中，保证查询效率：

```
data slice (按 Timestamp 升序):
┌─────┬─────┬─────┬─────┬─────┐
│ DP0 │ DP1 │ DP2 │ DP3 │ DP4 │
└─────┴─────┴─────┴─────┴─────┘
   0     1     2     3     4    <- 索引位置
```

### 4.2 标签倒排索引结构

标签索引采用二级 map 结构，实现快速的标签过滤：

```
tagIndex:
┌─────────────────────────────────────────────────┐
│ "host"                                          │
│   ┌─────────┬─────────┬─────────┐              │
│   │ "s1"    │ "s2"    │ "s3"    │              │
│   │ [0,2,4] │ [1,3]   │ [5]     │              │
│   └─────────┴─────────┴─────────┘              │
├─────────────────────────────────────────────────┤
│ "dc"                                            │
│   ┌─────────┬─────────┐                         │
│   │ "us"    │ "eu"    │                         │
│   │ [0,1,5] │ [2,3,4] │                         │
│   └─────────┴─────────┘                         │
└─────────────────────────────────────────────────┘
```

**查询逻辑**:
- 单标签查询：直接通过 `tagIndex[tagKey][tagValue]` 获取匹配的索引列表
- 多标签查询：对每个标签的结果集求交集，得到同时满足所有标签条件的索引

## 5. 数据点从写入到过期清理的完整生命周期

### 5.1 数据写入流程

```
Write(points)
       │
       ▼
  引擎状态检查
  └─ closed == true? ──是──► ErrEngineClosed
       │
       ▼
  空输入检查: len(points) == 0 → 直接返回 nil
       │
       ▼
  参数校验
  ├─ 任一 point == nil? ──是──► ErrNilDataPoint
  └─ 任一 point.Tags 为空? ──是──► ErrEmptyTags
       │
       ▼
  数据预处理
  └─ 复制所有 Tags map，避免外部修改影响内部数据
       │
       ▼
  获取写锁 (dataMu.Lock + tagIndexMu.Lock)
       │
       ▼
  追加数据: data = append(data, validPoints...)
       │
       ▼
  按时间戳排序: sort.Slice(data, by Timestamp)
       │
       ▼
  重建标签索引: rebuildTagIndex()
  └─ 遍历所有 data，为每个标签键值建立索引映射
       │
       ▼
  释放写锁
       │
       ▼
  返回 nil (成功)
```

### 5.2 数据查询流程

```
Query(start, end, tags)
       │
       ▼
  引擎状态检查 → closed? → ErrEngineClosed
       │
       ▼
  参数校验: start > end? → ErrInvalidTimeRange
       │
       ▼
  获取读锁 (dataMu.RLock)
       │
       ▼
  空引擎检查: len(data) == 0 → 返回空切片
       │
       ▼
  标签过滤: filterByTags(tags)
  ├─ tags 为空或 nil → 返回所有索引 [0, 1, 2, ..., n-1]
  ├─ 遍历每个标签条件:
  │  ├─ tagKey 不存在 → 返回空 []
  │  ├─ tagValue 不存在 → 返回空 []
  │  └─ 多标签时，对结果集逐步求交集
  └─ 返回匹配的索引列表
       │
       ▼
  时间范围过滤
  └─ 遍历匹配的索引，筛选 Timestamp 在 [start, end] 范围内的数据点
       │
       ▼
  结果封装
  ├─ 创建 DataPoint 副本（复制 Value 和 Tags）
  └─ 按时间戳升序排序
       │
       ▼
  释放读锁
       │
       ▼
  返回结果
```

### 5.3 降采样聚合流程

```
Downsample(start, end, windowSize, agg, tags)
       │
       ▼
  引擎状态检查 → closed? → ErrEngineClosed
       │
       ▼
  参数校验
  ├─ start > end? → ErrInvalidTimeRange
  ├─ windowSize <= 0? → ErrInvalidWindowSize
  └─ agg 不在支持列表? → ErrInvalidAggregator
       │
       ▼
  调用 Query(start, end, tags) 获取过滤后的数据点
       │
       ▼
  空结果检查: len(points) == 0 → 返回空 []
       │
       ▼
  窗口分桶
  ├─ windowMs = windowSize.Milliseconds()
  └─ 遍历每个数据点:
     ├─ bucketStart = (Timestamp / windowMs) * windowMs
     └─ 按 bucketStart 分组，维护每个桶的统计信息:
        - sum: 数值累加
        - count: 点数统计
        - min: 最小值（初始为第一个点的值）
        - max: 最大值（初始为第一个点的值）
       │
       ▼
  聚合计算
  └─ 遍历每个桶，根据聚合算子计算结果值:
     ├─ AggAvg: sum / count
     ├─ AggSum: sum
     ├─ AggMin: min
     ├─ AggMax: max
     └─ AggCount: float64(count)
       │
       ▼
  按窗口起始时间戳升序排序
       │
       ▼
  返回聚合结果
```

### 5.4 TTL 自动过期清理流程

#### 5.4.1 后台清理循环

```
cleanupLoop() (后台 goroutine)
       │
       ▼
  创建 ticker: time.NewTicker(cleanupInt)
       │
       ▼
  循环:
     ├─ select {
     │   ├─ <-ticker.C → 调用 cleanupExpired()
     │   └─ <-stopCh → 退出循环，return
     └─ }
```

#### 5.4.2 清理执行逻辑

```
cleanupExpired()
       │
       ▼
  引擎状态检查 → closed? → 直接返回
       │
       ▼
  TTL 检查: ttl < 0 → 直接返回（TTL 禁用）
       │
       ▼
  计算过期截止时间:
  cutoff = time.Now().Add(-ttl).UnixMilli()
       │
       ▼
  循环处理批次:
     ├─ 获取写锁 (dataMu.Lock + tagIndexMu.Lock)
     │
     ├─ 检查是否需要清理:
     │  ├─ len(data) == 0 → 释放锁，break
     │  └─ data[0].Timestamp >= cutoff → 释放锁，break
     │
     ├─ 统计可清理数量:
     │  removeCount = min(batchSize, len(data))
     │  从前往后遍历，遇到 Timestamp >= cutoff 则停止
     │
     ├─ removeCount == 0 → 释放锁，break
     │
     ├─ 执行清理:
     │  data = data[removeCount:]
     │
     ├─ 重建标签索引:
     │  rebuildTagIndex()
     │
     ├─ 释放写锁
     │
     └─ removeCount < batchSize → break（清理完成）
```

### 5.5 引擎关闭流程

```
Close()
       │
       ▼
  获取 closedMu 写锁
       │
       ▼
  检查 closed 状态，已关闭则解锁并返回
       │
       ▼
  设置 closed = true
       │
       ▼
  关闭 stopCh，通知清理 goroutine 退出
       │
       ▼
  释放 closedMu 写锁
       │
       ▼
  等待清理 goroutine 退出: wg.Wait()
       │
       ▼
  返回
```

## 6. 降采样窗口对齐算法

### 6.1 窗口起始时间计算

对于任意时间戳 `ts` 和窗口大小 `windowMs`，窗口起始时间计算为：

```
bucketStart = (ts / windowMs) * windowMs
```

**示例**：窗口大小 10s (10000ms)

| 时间戳 | 计算 | 窗口起始 |
|--------|------|----------|
| 1234567890123 | (1234567890123 / 10000) * 10000 | 1234567890000 |
| 1234567895000 | (1234567895000 / 10000) * 10000 | 1234567890000 |
| 1234567900000 | (1234567900000 / 10000) * 10000 | 1234567900000 |

### 6.2 分桶示例

窗口大小: 2s (2000ms)，数据点时间戳: 100, 1500, 2100, 4500

```
时间轴:
0 ──── 1000 ──── 2000 ──── 3000 ──── 4000 ──── 5000 ────>
       ▲       ▲         ▲                 ▲
     100      1500      2100              4500

分桶结果:
Bucket 0 (0-2000):    [100, 1500]
Bucket 2000 (2000-4000): [2100]
Bucket 4000 (4000-6000): [4500]
```

## 7. 并发安全设计

### 7.1 锁分层策略

模块采用多把读写锁设计，减少锁竞争：

| 锁 | 保护对象 | 锁类型 |
|----|---------|--------|
| `closedMu` | `closed` 状态标志 | `sync.RWMutex` |
| `dataMu` | `data` 数据切片 | `sync.RWMutex` |
| `tagIndexMu` | `tagIndex` 标签索引 | `sync.RWMutex` |

### 7.2 读写锁使用规则

- **状态检查** (所有操作入口):
  - 获取 `closedMu.RLock()` 检查引擎是否关闭
  - 检查完成立即释放

- **读操作** (Query, Downsample, Count):
  - 获取 `dataMu.RLock()` 读取数据
  - `tagIndexMu.RLock()` 由 `filterByTags` 内部获取
  - 允许多个读操作并发执行

- **写操作** (Write, cleanupExpired):
  - 按顺序获取 `dataMu.Lock()` → `tagIndexMu.Lock()`
  - 写操作完成后逆序释放
  - 写操作与其他读/写操作互斥

### 7.3 死锁避免

- 固定锁获取顺序：`closedMu` → `dataMu` → `tagIndexMu`
- 避免嵌套获取同一把锁
- 写操作完成后立即释放锁，减少持有时间
- 重建索引时在同一锁保护内完成，避免中间状态可见

## 8. 使用示例

### 8.1 基本使用

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/tsdb"
)

func main() {
    // 创建时序引擎实例（使用默认配置）
    engine := tsdb.NewTSEngine()
    defer engine.Close()

    now := time.Now().UnixMilli()

    // 写入时序数据
    points := []*tsdb.DataPoint{
        {Timestamp: now, Value: 23.5, Tags: map[string]string{"host": "server1", "metric": "cpu"}},
        {Timestamp: now + 1000, Value: 45.2, Tags: map[string]string{"host": "server1", "metric": "cpu"}},
        {Timestamp: now + 2000, Value: 67.8, Tags: map[string]string{"host": "server2", "metric": "cpu"}},
        {Timestamp: now + 3000, Value: 12.1, Tags: map[string]string{"host": "server1", "metric": "memory"}},
    }

    if err := engine.Write(points); err != nil {
        fmt.Printf("Write failed: %v\n", err)
        return
    }

    fmt.Printf("Total points: %d\n", engine.Count())

    // 按时间范围和标签查询
    result, err := engine.Query(now, now+5000, map[string]string{"host": "server1"})
    if err != nil {
        fmt.Printf("Query failed: %v\n", err)
        return
    }

    for _, p := range result {
        fmt.Printf("Time: %d, Value: %.2f, Tags: %v\n", p.Timestamp, p.Value, p.Tags)
    }
}
```

### 8.2 降采样聚合

```go
engine := tsdb.NewTSEngine()
defer engine.Close()

// 写入10秒内每秒一个数据点
now := time.Now().UnixMilli()
points := make([]*tsdb.DataPoint, 10)
for i := 0; i < 10; i++ {
    points[i] = &tsdb.DataPoint{
        Timestamp: now + int64(i)*1000,
        Value:     float64(i + 1),
        Tags:      map[string]string{"metric": "requests"},
    }
}
engine.Write(points)

// 按5秒窗口降采样，计算平均值
window := 5 * time.Second
tags := map[string]string{"metric": "requests"}

avgResult, _ := engine.Downsample(now, now+10000, window, tsdb.AggAvg, tags)
for _, ap := range avgResult {
    fmt.Printf("Window start: %d, Avg: %.2f, Count: %d\n",
        ap.Timestamp, ap.Value, ap.Count)
}

// 计算最大值
maxResult, _ := engine.Downsample(now, now+10000, window, tsdb.AggMax, tags)
for _, ap := range maxResult {
    fmt.Printf("Window start: %d, Max: %.2f\n", ap.Timestamp, ap.Value)
}

// 计算求和
sumResult, _ := engine.Downsample(now, now+10000, window, tsdb.AggSum, tags)
for _, ap := range sumResult {
    fmt.Printf("Window start: %d, Sum: %.2f\n", ap.Timestamp, ap.Value)
}
```

### 8.3 自定义 TTL 配置

```go
// 配置 TTL 为 1 小时，每 5 分钟清理一次，每次清理 500 条
cfg := tsdb.Config{
    TTL:              time.Hour,
    CleanupInterval:  5 * time.Minute,
    CleanupBatchSize: 500,
}

engine := tsdb.NewTSEngineWithConfig(cfg)
defer engine.Close()

// 写入数据
now := time.Now().UnixMilli()
engine.Write([]*tsdb.DataPoint{
    {Timestamp: now, Value: 1.0, Tags: map[string]string{"id": "1"}},
})

// 可以手动触发清理（主要用于测试）
engine.ForceCleanup()
```

### 8.4 禁用 TTL

```go
// TTL 设置为负数禁用自动过期清理
cfg := tsdb.Config{
    TTL:              -1,
    CleanupInterval:  time.Minute,
    CleanupBatchSize: 100,
}

engine := tsdb.NewTSEngineWithConfig(cfg)
defer engine.Close()

fmt.Printf("TTL disabled: %v\n", engine.GetTTL() < 0) // true
```

### 8.5 多标签组合查询

```go
engine := tsdb.NewTSEngine()
defer engine.Close()

now := time.Now().UnixMilli()
points := []*tsdb.DataPoint{
    {Timestamp: now, Value: 10.0, Tags: map[string]string{"host": "s1", "dc": "us", "env": "prod"}},
    {Timestamp: now + 1000, Value: 20.0, Tags: map[string]string{"host": "s1", "dc": "eu", "env": "prod"}},
    {Timestamp: now + 2000, Value: 30.0, Tags: map[string]string{"host": "s2", "dc": "us", "env": "staging"}},
    {Timestamp: now + 3000, Value: 40.0, Tags: map[string]string{"host": "s1", "dc": "us", "env": "prod"}},
}
engine.Write(points)

// 查询 host=s1 且 dc=us 且 env=prod 的数据
tags := map[string]string{
    "host": "s1",
    "dc":   "us",
    "env":  "prod",
}

result, _ := engine.Query(0, now+10000, tags)
fmt.Printf("Found %d points matching all tags\n", len(result)) // 2
```

### 8.6 并发使用

```go
var wg sync.WaitGroup
engine := tsdb.NewTSEngine()
defer engine.Close()

now := time.Now().UnixMilli()

// 并发写入
for g := 0; g < 10; g++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        points := make([]*tsdb.DataPoint, 100)
        for i := 0; i < 100; i++ {
            points[i] = &tsdb.DataPoint{
                Timestamp: now + int64(id*100+i),
                Value:     float64(i),
                Tags:      map[string]string{"goroutine": fmt.Sprintf("g%d", id)},
            }
        }
        if err := engine.Write(points); err != nil {
            log.Printf("Write failed in goroutine %d: %v", id, err)
        }
    }(g)
}

// 并发查询
for r := 0; r < 20; r++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 10; i++ {
            result, err := engine.Query(
                now, now+100000,
                map[string]string{"goroutine": "g1"},
            )
            if err != nil {
                log.Printf("Query failed: %v", err)
                return
            }
            _ = result
        }
    }()
}

wg.Wait()
fmt.Printf("Total points: %d\n", engine.Count()) // 1000
```

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidTimeRange` | 无效的时间范围 | `Query` 或 `Downsample` 时 start > end |
| `ErrInvalidWindowSize` | 无效的窗口大小 | `Downsample` 时 windowSize <= 0 |
| `ErrInvalidAggregator` | 无效的聚合算子 | `Downsample` 时使用未定义的 AggregatorType |
| `ErrInvalidTTL` | 无效的 TTL | 配置 TTL 时校验失败（当前实现中 0 表示使用默认值，负数表示禁用） |
| `ErrInvalidBatchSize` | 无效的批次大小 | 配置 CleanupBatchSize <= 0（当前实现中使用默认值） |
| `ErrInvalidInterval` | 无效的清理间隔 | 配置 CleanupInterval <= 0（当前实现中使用默认值） |
| `ErrEmptyTags` | 标签为空 | 写入数据点时 Tags map 为空 |
| `ErrNilDataPoint` | 数据点为 nil | 写入的 points 切片中包含 nil 元素 |
| `ErrEngineClosed` | 引擎已关闭 | 在 Close() 之后调用 Write/Query/Downsample |

## 10. 性能特征

### 10.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Write | O(N log N) | N 为总数据点数，主要耗时在排序上；索引重建 O(N × K)，K 为平均标签数 |
| Query | O(M + F log F) | M 为标签匹配的索引数，F 为过滤后的结果数 |
| Downsample | O(M + F + B log B) | M 同 Query，F 为过滤后的点数，B 为分桶数量 |
| filterByTags | O(K × M) | K 为标签条件数，M 为每个标签的平均匹配数 |
| cleanupExpired | O(R + (N-R) × K) | R 为清理的点数，N 为剩余点数，K 为平均标签数 |
| Count | O(1) | 直接返回切片长度 |

### 10.2 空间复杂度

- 数据存储: O(N × (1 + K))，N 为数据点数，K 为平均标签数
- 标签索引: O(K × V × M)，K 为标签键数，V 为每个键的平均标签值数，M 为每个键值对的平均匹配数
- 降采样聚合: O(B)，B 为分桶数量

### 10.3 性能优化点

1. **数据预排序**: 写入时一次性排序，查询时无需再次排序（除非标签过滤后乱序）
2. **标签索引交集**: 多标签查询时从最小集合开始求交集，减少比较次数
3. **批次清理**: 每次清理限制数量，避免长时间持有锁影响写入和查询
4. **读写锁分离**: 多把读写锁减少锁粒度，提高并发度

## 11. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失，适用于缓存和实时监控场景
2. **时间戳排序**: 数据按时间戳升序存储，写入乱序数据会触发全量排序，批量写入时建议预先排序
3. **索引重建开销**: 每次写入和清理都会重建整个标签索引，数据量大时写入开销较高
4. **TTL 精度**: 过期清理依赖后台定时任务，实际过期时间可能存在最多 `CleanupInterval` 的延迟
5. **查询结果副本**: Query 返回数据点副本，修改返回值不会影响内部存储，但会增加内存开销
6. **并发度**: 读写锁设计允许多读者并发，写操作串行，适合读多写少场景
7. **降采样浮点数精度**: 平均值计算可能存在浮点数精度问题，高精度场景请自行处理
8. **负数时间戳**: 支持负数时间戳（1970年之前的时间），适用于历史数据处理
9. **相同时间戳**: 支持同一时间戳写入多个数据点，排序稳定性不做保证
10. **标签键值设计**: 标签基数过高（如每个数据点唯一的标签值）会导致索引占用大量内存

## 12. 设计权衡

| 决策 | 优点 | 缺点 |
|------|------|------|
| 全量重建标签索引 | 实现简单，不易出错 | 写入和清理时性能开销较大 |
| 数据追加后全排序 | 查询效率高，保证时间有序 | 写入 O(N log N) 复杂度 |
| 多把读写锁 | 减少锁竞争，提高并发度 | 增加代码复杂度，需注意锁顺序 |
| 返回数据副本 | 安全，避免外部修改内部状态 | 增加内存分配和拷贝开销 |
| 批次清理 | 避免单次清理耗时过长 | 过期数据可能存在多批次才能清理完 |
