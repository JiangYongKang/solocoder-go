# 流式数据处理器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [数据流转路径](#4-数据流转路径)
5. [数据源订阅](#5-数据源订阅)
6. [算子链式组装](#6-算子链式组装)
7. [窗口聚合计算](#7-窗口聚合计算)
8. [背压信号传递](#8-背压信号传递)
9. [检查点状态持久化](#9-检查点状态持久化)
10. [使用示例](#10-使用示例)
11. [错误定义](#11-错误定义)
12. [并发安全](#12-并发安全)

---

## 1. 模块概述

流式数据处理器（Stream Processor）是一个高性能、高可用的实时数据处理引擎，提供数据源订阅、算子链式处理、窗口聚合计算、背压控制和检查点持久化等完整的流处理能力。模块设计用于需要实时数据处理、状态管理和故障恢复的场景。

**包路径**: `internal/streamproc`

**设计目标**:
- 支持多种数据源抽象（Channel、Slice、Generator）
- 提供灵活的算子链式组装能力
- 支持时间窗口和计数窗口的聚合计算
- 实现自适应背压控制机制
- 提供可靠的检查点和故障恢复能力
- 保证线程安全，支持高并发场景

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 数据源订阅 | 支持注册和启动多种数据源，提供 Start/Pause/Resume/Stop 生命周期管理 |
| 算子链式组装 | 提供 Filter/Map/FlatMap 等基本算子，支持链式组装成处理管道 |
| 窗口聚合计算 | 支持滚动/滑动时间窗口和计数窗口，提供 Sum/Count/Avg/Min/Max 聚合 |
| 背压信号传递 | 基于待处理队列长度的三级背压机制（Normal/Warning/Critical），自动控制数据源流速 |
| 检查点持久化 | 支持手动和定时自动触发检查点，保存算子状态和处理进度，支持故障恢复 |
| 状态管理 | 所有算子和窗口支持状态保存和恢复，保证 Exactly-Once 语义 |
| 监控统计 | 提供详细的处理统计信息，包括输入/输出/丢弃记录数、窗口关闭数、检查点数等 |

---

## 3. 核心结构体与职责

### 3.1 Pipeline

主处理管道结构体，整合所有组件，对外提供完整的流处理接口。

```go
type Pipeline struct {
    pendingCount int64         // 待处理记录数（原子操作）
    sourceOffset int64         // 数据源处理偏移量（原子操作）
    stats        PipelineStats // 处理统计信息（原子操作字段）

    cfg        PipelineConfig  // 管道配置
    source     Source          // 数据源
    operators  *OperatorChain  // 算子链
    window     *WindowAggregator // 窗口聚合器（可选）
    sink       Sink            // 输出接收器（可选）
    checkpoint CheckpointStore // 检查点存储

    // ... 内部状态和同步字段
}
```

**职责**:
- 协调整个数据处理流程的生命周期
- 管理数据源、算子链、窗口聚合器的启动和停止
- 实现背压监控和控制逻辑
- 调度检查点的保存和恢复
- 收集和统计处理指标
- 处理运行时错误和异常情况

### 3.2 Source 接口

数据源抽象接口，定义数据生产者的契约。

```go
type Source interface {
    Name() string
    Start(ctx context.Context) error
    Pause() error
    Resume() error
    Stop() error
    State() SourceState
    Output() <-chan *Record
}
```

**职责**:
- 定义数据源的统一接口
- 支持 Start/Pause/Resume/Stop 生命周期管理
- 提供状态查询能力
- 通过 channel 向下游推送数据记录

**内置实现**:
- `ChannelSource`: 从 Go channel 读取数据
- `SliceSource`: 从内存切片产生数据，支持发送间隔配置
- `GeneratorSource`: 通过用户提供的生成函数产生数据

### 3.3 Operator 接口

数据处理算子抽象接口，定义单个处理单元的契约。

```go
type Operator interface {
    Name() string
    Process(ctx context.Context, input *Record) ([]*Record, error)
    SaveState() ([]byte, error)
    RestoreState(data []byte) error
}
```

**职责**:
- 定义算子的统一处理接口
- 支持状态持久化和恢复
- 接收单条记录，输出零条或多条记录

**内置实现**:
- `FilterOperator`: 过滤算子，根据条件筛选记录
- `MapOperator`: 映射算子，对每条记录进行转换
- `FlatMapOperator`: 扁平化映射算子，每条记录可产生多条输出

### 3.4 OperatorChain

算子链结构体，管理多个算子的顺序执行。

```go
type OperatorChain struct {
    operators []Operator
    mu        sync.RWMutex
}
```

**职责**:
- 管理多个算子的有序执行
- 支持动态添加、插入、删除算子
- 提供批量状态保存和恢复能力
- 处理记录在算子间的流转

### 3.5 WindowAggregator

窗口聚合器结构体，实现基于窗口的聚合计算。

```go
type WindowAggregator struct {
    seqCounter    int64  // 序列号计数器（原子操作）
    closedWindows int64  // 已关闭窗口数（原子操作）
    countSize     int64  // 计数窗口大小（原子操作）
    countSlide    int64  // 计数窗口滑动步长（原子操作）

    name          string
    windowType    WindowType
    aggregation   AggregationType
    size          time.Duration // 时间窗口大小
    slide         time.Duration // 时间窗口滑动步长
    extractor     ValueExtractor
    // ... 内部状态和同步字段
}
```

**职责**:
- 管理窗口的创建、数据分配和关闭
- 支持时间窗口和计数窗口两种类型
- 支持滚动窗口和滑动窗口两种模式
- 实现多种聚合函数（Sum/Count/Avg/Min/Max）
- 支持窗口状态的持久化和恢复

### 3.6 CheckpointStore 接口

检查点存储抽象接口，定义状态持久化的契约。

```go
type CheckpointStore interface {
    Save(ctx context.Context, checkpoint *Checkpoint) error
    Load(ctx context.Context, id string) (*Checkpoint, error)
    LoadLatest(ctx context.Context) (*Checkpoint, error)
    List(ctx context.Context) ([]*Checkpoint, error)
    Delete(ctx context.Context, id string) error
    Clear(ctx context.Context) error
}
```

**职责**:
- 定义检查点存储的统一接口
- 支持检查点的保存、加载、列出、删除操作
- 提供最新检查点的获取能力
- 保证状态数据的隔离性（深拷贝）

**内置实现**:
- `MemoryCheckpointStore`: 内存存储实现，用于测试和简单场景

### 3.7 Record

数据记录结构体，流处理中的基本数据单元。

```go
type Record struct {
    ID        string
    SeqID     int64
    Timestamp time.Time
    Data      interface{}
    Metadata  map[string]interface{}
}
```

**职责**:
- 封装流处理中的数据记录
- 提供唯一标识和序列号
- 支持时间戳和元数据
- 提供克隆方法保证数据安全

### 3.8 Sink 接口

数据接收器抽象接口，定义处理结果输出的契约。

```go
type Sink interface {
    Consume(ctx context.Context, record *Record) error
    ConsumeWindow(ctx context.Context, result *WindowResult) error
    Close(ctx context.Context) error
}
```

**职责**:
- 定义处理结果的输出接口
- 支持单条记录和窗口结果的消费
- 提供资源清理能力

**内置实现**:
- `CollectSink`: 内存收集器，用于测试和调试

---

## 4. 数据流转路径

### 4.1 完整流转图

```
┌─────────────────┐
│   Data Source   │  产生数据记录
│ (Channel/Slice/ │
│   Generator)    │
└────────┬────────┘
         │
         ▼  Record
┌─────────────────┐
│  Pipeline       │
│  sourceReader   │  读取数据源输出，
│                 │  应用背压控制，
│                 │  发送到处理通道
└────────┬────────┘
         │
         ▼  Record
┌─────────────────┐
│  Pipeline       │
│ recordProcessor │  处理单条记录：
│                 │  1. 算子链处理
│                 │  2. 窗口聚合（可选）
│                 │  3. 发送到 Sink（可选）
└────────┬────────┘
         │
         ├───────────────────────────┐
         │                           │
         ▼  []*Record                ▼  WindowResult
┌─────────────────┐        ┌─────────────────┐
│  OperatorChain  │        │ WindowAggregator│
│  Filter/Map/    │        │  窗口分配、     │
│  FlatMap        │        │  聚合计算、     │
│                 │        │  窗口关闭       │
└────────┬────────┘        └────────┬────────┘
         │                           │
         ▼  []*Record                ▼  WindowResult
┌─────────────────┐        ┌─────────────────┐
│      Sink       │        │      Sink       │
│  (可选)         │        │  (可选)         │
└─────────────────┘        └─────────────────┘
```

### 4.2 详细流转步骤

1. **数据源启动**
   - 调用 `Pipeline.Start()` 启动整个管道
   - 数据源被启动，开始产生 `Record` 并发送到输出 channel
   - 每个 `Record` 包含唯一 `SeqID` 和时间戳

2. **数据读取与背压控制**
   - `sourceReader` goroutine 从数据源的输出 channel 读取记录
   - 每读取一条记录，增加 `RecordsIn` 统计
   - 将记录发送到内部 `recordCh`，同时增加 `pendingCount`
   - 检查背压状态，如果达到 Critical 级别则暂停数据源

3. **算子链处理**
   - `recordProcessor` goroutine 从 `recordCh` 读取记录
   - 减少 `pendingCount`，记录已处理的 `SeqID`
   - 更新 `sourceOffset` 为当前处理的序列号
   - 将记录传入 `OperatorChain.Process()` 进行链式处理
   - 如果算子返回 nil 或空切片，记录被丢弃，增加 `RecordsDropped`

4. **窗口聚合（可选）**
   - 如果配置了 `WindowAggregator`，处理后的记录被送入窗口聚合器
   - 根据窗口类型（时间/计数）和模式（滚动/滑动）分配到相应窗口
   - 窗口内的数据进行聚合计算（Sum/Count/Avg/Min/Max）
   - 当窗口满足关闭条件时，产生 `WindowResult` 并发送到结果通道

5. **结果输出（可选）**
   - 如果配置了 `Sink`，处理后的记录和窗口结果被发送到 Sink
   - 调用 `Sink.Consume()` 处理单条记录
   - 调用 `Sink.ConsumeWindow()` 处理窗口结果
   - 增加 `RecordsOut` 统计

6. **检查点持久化（可选）**
   - 如果启用了检查点，`checkpointLoop` goroutine 定期触发检查点
   - 收集所有算子的状态、窗口状态、当前 `sourceOffset`
   - 创建 `Checkpoint` 对象并保存到 `CheckpointStore`
   - 增加 `CheckpointsMade` 统计

7. **背压信号传递**
   - `pendingCount` 超过 `BackpressureThreshold * WarningRatio` 时进入 Warning 状态
   - `pendingCount` 超过 `BackpressureThreshold * CriticalRatio` 时进入 Critical 状态
   - Critical 状态下自动调用 `source.Pause()` 暂停数据源
   - 当 `pendingCount` 降低到 Normal 范围时自动调用 `source.Resume()` 恢复

---

## 5. 数据源订阅

### 5.1 数据源类型

#### ChannelSource

从现有 Go channel 读取数据的数据源。

```go
// 创建 ChannelSource
input := make(chan *Record, 100)
source := NewChannelSource("my-source", input, 100)

// 启动数据源
ctx := context.Background()
_ = source.Start(ctx)

// 发送数据到 channel
input <- NewRecord("data")

// 停止数据源
_ = source.Stop()
```

**特点**:
- 适用于接入外部数据流
- 支持缓冲区大小配置
- 完全由外部控制数据产生速率

#### SliceSource

从内存切片产生数据的数据源，支持发送间隔配置。

```go
// 创建测试数据
records := []*Record{
    NewRecord(1),
    NewRecord(2),
    NewRecord(3),
}

// 创建 SliceSource，每秒发送一条记录
source := NewSliceSource("slice-source", records, 100, time.Second)

// 启动后会按顺序发送 records 中的数据
_ = source.Start(ctx)
```

**特点**:
- 适用于测试和批处理场景
- 支持发送间隔配置（0 表示尽可能快）
- 数据自动克隆，保证隔离性
- 支持当前处理索引查询

#### GeneratorSource

通过用户提供的生成函数动态产生数据的数据源。

```go
// 创建 GeneratorSource，产生 100 条递增序列
generator := func(seq int64) *Record {
    return NewRecord(int(seq))
}
source := NewGeneratorSource("gen-source", generator, 100, 100, 10*time.Millisecond)

_ = source.Start(ctx)
```

**特点**:
- 适用于需要动态生成数据的场景
- 支持最大记录数配置（0 表示无限）
- 支持生成间隔配置
- 生成函数返回 nil 表示跳过该序列号

### 5.2 生命周期管理

所有数据源都支持完整的生命周期管理：

```
      ┌─────────┐
      │  Idle   │  初始状态
      └────┬────┘
           │ Start()
           ▼
      ┌─────────┐
      │ Running │  正常产生数据
      └────┬────┘
           │ Pause()
           ▼
      ┌─────────┐
      │ Paused  │  暂停数据产生
      └────┬────┘
           │ Resume()
           ▼
      ┌─────────┐
      │ Running │
      └────┬────┘
           │ Stop()
           ▼
      ┌─────────┐
      │ Stopped │  终止，不可重启
      └─────────┘
```

**状态转换规则**:
- `Idle` → `Running`: 调用 `Start()`
- `Running` → `Paused`: 调用 `Pause()`
- `Paused` → `Running`: 调用 `Resume()`
- `Running`/`Paused` → `Stopped`: 调用 `Stop()`
- `Stopped` 是终态，不可转换到其他状态

---

## 6. 算子链式组装

### 6.1 算子类型

#### FilterOperator

过滤算子，根据用户提供的过滤函数决定记录是否通过。

```go
// 创建过滤器：只保留偶数
filter := NewFilterOperator("even-filter", func(ctx context.Context, r *Record) (bool, error) {
    val := r.Data.(int)
    return val%2 == 0, nil
})

// 处理记录
result, err := filter.Process(ctx, NewRecord(4))
// result = [Record(4)]

result, err = filter.Process(ctx, NewRecord(3))
// result = nil（被过滤）
```

**统计信息**:
- `Processed`: 处理的记录总数
- `Passed`: 通过过滤的记录数
- `Dropped`: 被过滤的记录数

#### MapOperator

映射算子，对每条记录进行转换。

```go
// 创建映射器：将数值乘以 2
mapper := NewMapOperator("double", func(ctx context.Context, r *Record) (*Record, error) {
    val := r.Data.(int)
    return NewRecord(val * 2), nil
})

// 处理记录
result, err := mapper.Process(ctx, NewRecord(5))
// result = [Record(10)]
```

**注意**:
- 映射函数返回 nil 表示该记录被丢弃
- 返回错误会终止处理并向上传播

#### FlatMapOperator

扁平化映射算子，每条输入记录可产生多条输出记录。

```go
// 创建扁平映射器：将字符串按字符拆分
flatMapper := NewFlatMapOperator("split", func(ctx context.Context, r *Record) ([]*Record, error) {
    str := r.Data.(string)
    results := make([]*Record, 0, len(str))
    for _, c := range str {
        results = append(results, NewRecord(string(c)))
    }
    return results, nil
})

// 处理记录
result, err := flatMapper.Process(ctx, NewRecord("abc"))
// result = [Record("a"), Record("b"), Record("c")]
```

**统计信息**:
- `Processed`: 处理的输入记录数
- `OutputCnt`: 产生的输出记录数

### 6.2 算子链操作

```go
// 创建算子链
chain := NewOperatorChain()

// 添加算子（追加到末尾）
_ = chain.Add(filter)
_ = chain.Add(mapper)
_ = chain.Add(flatMapper)

// 插入算子到指定位置
_ = chain.Insert(1, anotherFilter)

// 移除指定位置的算子
_ = chain.Remove(0)

// 获取算子列表
names := chain.List()  // ["even-filter", "double", ...]

// 获取算子数量
count := chain.Count()
```

### 6.3 链式处理流程

```
Input Record
     │
     ▼
┌────────────┐
│ Operator 1 │ → 结果为 nil 则终止，丢弃记录
└─────┬──────┘
      │
      ▼
┌────────────┐
│ Operator 2 │ → 每条输出记录继续传递
└─────┬──────┘
      │
      ▼
┌────────────┐
│ Operator 3 │
└─────┬──────┘
      │
      ▼
Output Records
```

**处理规则**:
- 算子按顺序执行，前一个算子的输出是后一个算子的输入
- 如果某个算子返回 nil 或空切片，该记录被丢弃，后续算子不再处理
- 如果某个算子返回错误，整个处理链终止，错误向上传播
- FlatMapOperator 的多条输出会分别传递给后续算子

---

## 7. 窗口聚合计算

### 7.1 窗口类型

#### 滚动计数窗口（TumblingCountWindow）

按固定数量的记录划分窗口，窗口之间不重叠。

```go
// 创建滚动计数窗口：每 5 条记录求和
window, _ := NewWindowAggregator("sum-window", WindowConfig{
    WindowType:  WindowTypeTumblingCount,
    Aggregation: AggregationSum,
    CountSize:   5,
    Extractor: func(r *Record) (float64, error) {
        return float64(r.Data.(int)), nil
    },
})
```

**窗口划分示例**（Size=5）:
```
Records:  1  2  3  4  5 | 6  7  8  9  10 | 11 12 ...
Windows: [  Window 1  ] | [  Window 2   ] | [ Window 3 ...
```

#### 滑动计数窗口（SlidingCountWindow）

按固定步长滑动的窗口，窗口之间可以重叠。

```go
// 创建滑动计数窗口：窗口大小 3，滑动步长 1，求平均值
window, _ := NewWindowAggregator("avg-window", WindowConfig{
    WindowType:  WindowTypeSlidingCount,
    Aggregation: AggregationAvg,
    CountSize:   3,
    CountSlide:  1,
    Extractor: func(r *Record) (float64, error) {
        return float64(r.Data.(int)), nil
    },
})
```

**窗口划分示例**（Size=3, Slide=1）:
```
Records:  1   2   3   4   5   6 ...
Window 1: [1, 2, 3]
Window 2:    [2, 3, 4]
Window 3:       [3, 4, 5]
Window 4:          [4, 5, 6]
```

#### 滚动时间窗口（TumblingTimeWindow）

按固定时间间隔划分窗口，窗口之间不重叠。

```go
// 创建滚动时间窗口：每 10 秒计数
window, _ := NewWindowAggregator("count-window", WindowConfig{
    WindowType:  WindowTypeTumblingTime,
    Aggregation: AggregationCount,
    Size:        10 * time.Second,
    Extractor: func(r *Record) (float64, error) {
        return 1.0, nil // 每条记录计数 1
    },
})
```

#### 滑动时间窗口（SlidingTimeWindow）

按固定时间步长滑动的窗口，窗口之间可以重叠。

```go
// 创建滑动时间窗口：窗口 1 分钟，滑动 30 秒，求最大值
window, _ := NewWindowAggregator("max-window", WindowConfig{
    WindowType:  WindowTypeSlidingTime,
    Aggregation: AggregationMax,
    Size:        1 * time.Minute,
    Slide:       30 * time.Second,
    Watermark:   5 * time.Second, // 水印延迟
    Extractor: func(r *Record) (float64, error) {
        return float64(r.Data.(int)), nil
    },
})
```

### 7.2 聚合类型

| 聚合类型 | 说明 |
|----------|------|
| `AggregationSum` | 求和 |
| `AggregationCount` | 计数 |
| `AggregationAvg` | 平均值 |
| `AggregationMin` | 最小值 |
| `AggregationMax` | 最大值 |

### 7.3 窗口关闭条件

| 窗口类型 | 关闭条件 |
|----------|----------|
| 滚动计数窗口 | 窗口内记录数 >= CountSize |
| 滑动计数窗口 | 当前处理 SeqID >= 窗口 EndSeq |
| 滚动时间窗口 | 当前时间 >= 窗口 EndTime + Watermark |
| 滑动时间窗口 | 当前时间 >= 窗口 EndTime + Watermark |

**Watermark（水印）**:
- 仅适用于时间窗口
- 用于处理乱序数据，允许延迟一定时间再关闭窗口
- 默认值为 0，表示立即关闭

### 7.4 窗口结果

当窗口关闭时，会产生 `WindowResult` 对象：

```go
type WindowResult struct {
    WindowID    string
    WindowType  WindowType
    Start       time.Time     // 时间窗口的开始时间
    End         time.Time     // 时间窗口的结束时间
    StartSeq    int64         // 计数窗口的开始 SeqID
    EndSeq      int64         // 计数窗口的结束 SeqID
    Aggregation AggregationType
    Value       float64       // 聚合结果值
    Count       int64         // 窗口内记录数
    RecordIDs   []string      // 窗口内记录 ID 列表
}
```

---

## 8. 背压信号传递

### 8.1 背压状态

```go
type BackpressureState int

const (
    BackpressureNormal   BackpressureState = iota // 正常
    BackpressureWarning                            // 警告
    BackpressureCritical                           // 严重
)
```

### 8.2 背压机制

背压基于待处理记录数 `pendingCount` 和配置的阈值进行判断：

```go
type PipelineConfig struct {
    BackpressureThreshold     int     // 背压阈值
    BackpressureWarningRatio  float64 // 警告阈值比例（默认 0.7）
    BackpressureCriticalRatio float64 // 严重阈值比例（默认 0.9）
    // ...
}
```

**状态判断逻辑**:
- `pendingCount < BackpressureThreshold * WarningRatio` → **Normal**
- `BackpressureThreshold * WarningRatio <= pendingCount < BackpressureThreshold * CriticalRatio` → **Warning**
- `pendingCount >= BackpressureThreshold * CriticalRatio` → **Critical**

### 8.3 背压控制流程

```
pendingCount 增长
     │
     ├─ < WarningThreshold → Normal → 数据源正常运行
     │
     ├─ >= WarningThreshold → Warning → 仅记录状态，不控制
     │
     └─ >= CriticalThreshold → Critical → 调用 source.Pause() 暂停数据源
                                                     │
                                                     ▼
                                             等待 pendingCount 下降
                                                     │
                                                     ▼
                                  pendingCount < WarningThreshold → Normal
                                                     │
                                                     ▼
                                             调用 source.Resume() 恢复
```

### 8.4 背压状态查询

```go
// 获取当前背压信息
info := pipeline.GetBackpressureInfo()

fmt.Printf("状态: %s\n", info.State)
fmt.Printf("待处理: %d\n", info.PendingCount)
fmt.Printf("阈值: %d\n", info.Threshold)
fmt.Printf("警告比例: %.2f\n", info.WarningRatio)
fmt.Printf("严重比例: %.2f\n", info.CriticalRatio)
```

**BackpressureInfo 结构体**:
```go
type BackpressureInfo struct {
    State         BackpressureState
    PendingCount  int
    Threshold     int
    WarningRatio  float64
    CriticalRatio float64
}
```

---

## 9. 检查点状态持久化

### 9.1 检查点内容

```go
type Checkpoint struct {
    ID             string
    Timestamp      time.Time
    SourceOffset   int64
    OperatorStates map[string][]byte
    WindowStates   map[string][]byte
    Metadata       map[string]interface{}
}
```

**检查点包含**:
- `ID`: 唯一标识，由 `GenerateCheckpointID()` 生成
- `Timestamp`: 检查点创建时间
- `SourceOffset`: 已处理的最大记录 SeqID
- `OperatorStates`: 各算子的状态数据（算子名称 → 状态字节）
- `WindowStates`: 各窗口的状态数据（窗口 ID → 状态字节）
- `Metadata`: 附加元数据

### 9.2 检查点触发方式

#### 手动触发

```go
// 立即保存检查点
err := pipeline.SaveCheckpoint()
if err != nil {
    log.Printf("保存检查点失败: %v", err)
}
```

#### 定时自动触发

```go
cfg := DefaultPipelineConfig()
cfg.EnableCheckpoint = true
cfg.CheckpointInterval = 30 * time.Second // 每 30 秒自动保存

pipeline, _ := NewPipeline(cfg, source)
```

**停止时自动保存**:
- 如果启用了检查点但尚未保存过任何检查点，`Stop()` 时会自动保存一次

### 9.3 故障恢复

```go
// 从最新检查点恢复
err := pipeline.RestoreFromLatestCheckpoint()
if err != nil {
    log.Printf("恢复检查点失败: %v", err)
}

// 或从指定检查点恢复
err = pipeline.RestoreFromCheckpoint("checkpoint-id-123")
```

**恢复内容**:
1. 恢复所有算子的内部状态（处理计数等）
2. 恢复窗口聚合器的所有活动窗口状态
3. 恢复 `SourceOffset`，跳过已处理的记录

**注意**:
- 恢复操作必须在管道启动前执行
- 恢复操作会跳过 `SourceOffset` 之前的记录，保证不重复处理

### 9.4 检查点管理

```go
// 列出所有检查点
store := NewMemoryCheckpointStore()
checkpoints, _ := store.List(ctx)

// 删除指定检查点
_ = store.Delete(ctx, "checkpoint-id")

// 清空所有检查点
_ = store.Clear(ctx)
```

### 9.5 状态隔离

`MemoryCheckpointStore` 在保存和加载时会进行深拷贝，保证：
- 保存后修改原始状态不影响已保存的检查点
- 加载后修改检查点数据不影响后续恢复
- 多协程并发访问安全

---

## 10. 使用示例

### 10.1 简单流处理

```go
package main

import (
    "context"
    "fmt"
    "time"

    "solocoder-go/internal/streamproc"
)

func main() {
    // 1. 创建数据源：生成 10 条递增数字
    generator := func(seq int64) *streamproc.Record {
        return streamproc.NewRecord(int(seq))
    }
    source := streamproc.NewGeneratorSource("numbers", generator, 10, 100, 10*time.Millisecond)

    // 2. 创建管道配置
    cfg := streamproc.DefaultPipelineConfig()
    cfg.BackpressureThreshold = 50

    // 3. 创建管道
    pipeline, err := streamproc.NewPipeline(cfg, source)
    if err != nil {
        panic(err)
    }

    // 4. 添加算子：过滤偶数，然后乘以 2
    filter := streamproc.NewFilterOperator("even", func(ctx context.Context, r *streamproc.Record) (bool, error) {
        return r.Data.(int)%2 == 0, nil
    })
    _ = pipeline.AddOperator(filter)

    mapper := streamproc.NewMapOperator("double", func(ctx context.Context, r *streamproc.Record) (*streamproc.Record, error) {
        return streamproc.NewRecord(r.Data.(int) * 2), nil
    })
    _ = pipeline.AddOperator(mapper)

    // 5. 设置 Sink 收集结果
    sink := streamproc.NewCollectSink()
    _ = pipeline.SetSink(sink)

    // 6. 启动管道
    ctx := context.Background()
    _ = pipeline.Start(ctx)

    // 7. 等待处理完成
    time.Sleep(500 * time.Millisecond)
    _ = pipeline.Stop()

    // 8. 查看结果
    records, _ := sink.Count()
    fmt.Printf("处理完成，共 %d 条记录\n", records)

    // 9. 查看统计
    stats := pipeline.Stats()
    fmt.Printf("输入: %d, 输出: %d, 丢弃: %d\n",
        stats.RecordsIn, stats.RecordsOut, stats.RecordsDropped)
}
```

### 10.2 带窗口聚合的流处理

```go
package main

import (
    "context"
    "fmt"
    "time"

    "solocoder-go/internal/streamproc"
)

func main() {
    // 1. 创建数据源：每秒产生一条温度数据
    temperatures := []*streamproc.Record{
        streamproc.NewRecord(25),
        streamproc.NewRecord(27),
        streamproc.NewRecord(30),
        streamproc.NewRecord(28),
        streamproc.NewRecord(32),
        streamproc.NewRecord(35),
    }
    source := streamproc.NewSliceSource("temp", temperatures, 100, 100*time.Millisecond)

    // 2. 创建窗口聚合器：每 3 条记录求平均值
    window, _ := streamproc.NewWindowAggregator("avg-temp", streamproc.WindowConfig{
        WindowType:  streamproc.WindowTypeTumblingCount,
        Aggregation: streamproc.AggregationAvg,
        CountSize:   3,
        Extractor: func(r *streamproc.Record) (float64, error) {
            return float64(r.Data.(int)), nil
        },
    })

    // 3. 创建管道
    cfg := streamproc.DefaultPipelineConfig()
    cfg.WindowAggregator = window

    sink := streamproc.NewCollectSink()
    cfg.Sink = sink

    pipeline, _ := streamproc.NewPipeline(cfg, source)

    // 4. 启动并等待
    ctx := context.Background()
    _ = pipeline.Start(ctx)
    time.Sleep(1 * time.Second)
    _ = pipeline.Stop()

    // 5. 查看窗口结果
    _, windowResults := sink.Count()
    fmt.Printf("产生 %d 个窗口结果\n", windowResults)

    for _, wr := range sink.WindowResults() {
        fmt.Printf("窗口 %s: 平均值 = %.1f, 记录数 = %d\n",
            wr.WindowID, wr.Value, wr.Count)
    }
}
```

### 10.3 带检查点和故障恢复的流处理

```go
package main

import (
    "context"
    "fmt"
    "time"

    "solocoder-go/internal/streamproc"
)

func main() {
    // 1. 创建检查点存储
    checkpointStore := streamproc.NewMemoryCheckpointStore()

    // 2. 创建数据源：产生 100 条记录
    generator := func(seq int64) *streamproc.Record {
        return streamproc.NewRecord(int(seq))
    }
    source := streamproc.NewGeneratorSource("data", generator, 100, 100, 5*time.Millisecond)

    // 3. 创建管道，启用检查点
    cfg := streamproc.DefaultPipelineConfig()
    cfg.EnableCheckpoint = true
    cfg.CheckpointInterval = 50 * time.Millisecond
    cfg.CheckpointStore = checkpointStore

    sink := streamproc.NewCollectSink()
    cfg.Sink = sink

    pipeline, _ := streamproc.NewPipeline(cfg, source)

    // 4. 添加有状态的算子
    filter := streamproc.NewFilterOperator("filter", func(ctx context.Context, r *streamproc.Record) (bool, error) {
        return r.Data.(int)%3 != 0, nil
    })
    _ = pipeline.AddOperator(filter)

    // 5. 启动管道，运行一段时间后停止
    ctx := context.Background()
    _ = pipeline.Start(ctx)
    time.Sleep(100 * time.Millisecond)
    _ = pipeline.Stop()

    // 6. 查看已处理进度
    stats := pipeline.Stats()
    fmt.Printf("第一次运行：处理到偏移量 %d\n", stats.SourceOffset)
    fmt.Printf("检查点数量：%d\n", stats.CheckpointsMade)

    // 7. 创建新管道，从检查点恢复
    source2 := streamproc.NewGeneratorSource("data", generator, 100, 100, 5*time.Millisecond)
    pipeline2, _ := streamproc.NewPipeline(cfg, source2)
    _ = pipeline2.AddOperator(filter)

    // 从最新检查点恢复
    err := pipeline2.RestoreFromLatestCheckpoint()
    if err != nil {
        fmt.Printf("恢复失败: %v\n", err)
    }

    sink2 := streamproc.NewCollectSink()
    _ = pipeline2.SetSink(sink2)

    // 8. 继续处理剩余数据
    _ = pipeline2.Start(ctx)
    time.Sleep(500 * time.Millisecond)
    _ = pipeline2.Stop()

    // 9. 验证结果
    stats2 := pipeline2.Stats()
    fmt.Printf("第二次运行：处理到偏移量 %d\n", stats2.SourceOffset)

    // 两次处理的记录总数应该等于 100（无重复，无遗漏）
    total := sink.RecordCount() + sink2.RecordCount()
    fmt.Printf("总处理记录数: %d（期望约 67，因为过滤掉了 1/3）\n", total)
}
```

---

## 11. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrSourceNil` | 数据源为空 | 创建 Pipeline 时 source 参数为 nil |
| `ErrSourceNotStarted` | 数据源未启动 | 对未启动的数据源调用 Pause/Resume |
| `ErrSourceAlreadyStarted` | 数据源已启动 | 重复调用 Start |
| `ErrOperatorNil` | 算子为空 | 添加 nil 算子到算子链 |
| `ErrInvalidBackpressureThreshold` | 背压阈值无效 | BackpressureThreshold <= 0 |
| `ErrPipelineRunning` | 管道运行中 | 在运行时修改管道配置 |
| `ErrPipelineStopped` | 管道已停止 | 对已停止的管道调用 Start |
| `ErrInvalidCheckpoint` | 检查点无效 | 保存 nil 检查点或检查点 ID 为空 |
| `ErrCheckpointNotFound` | 检查点不存在 | 加载不存在的检查点 |
| `ErrInvalidWindowConfig` | 窗口配置无效 | 窗口配置参数错误 |
| `ErrNegativeInterval` | 间隔为负 | 创建 Source 时 interval < 0 |

---

## 12. 并发安全

模块完全支持并发访问，通过以下机制保证线程安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| 原子计数字段 | `sync/atomic` | 所有 int64 统计字段使用原子操作，确保 32 位系统兼容性 |
| 数据源状态 | `sync.RWMutex` | 读写锁保护状态访问 |
| 算子链 | `sync.RWMutex` | 读写锁保护算子列表修改 |
| 窗口聚合器 | `sync.RWMutex` | 互斥锁保护窗口状态 |
| 检查点存储 | `sync.RWMutex` | 读写锁保护检查点数据 |
| Pipeline 状态 | `sync.RWMutex` | 读写锁保护管道状态 |
| Pipeline 统计 | `sync.RWMutex` | 读写锁保护统计信息 |
| 已处理序列号 | `sync.Mutex` | 互斥锁保护已处理记录集合 |

**32 位系统兼容性**:
- 所有进行原子操作的 int64 字段都放置在结构体的最前面，保证 8 字节对齐
- 避免 `unaligned 64-bit atomic operation` panic

**最佳实践**:
1. 不要在多个 goroutine 中同时调用生命周期方法（Start/Stop/Pause/Resume）
2. 检查点恢复操作必须在 Start 之前执行
3. 使用完毕后务必调用 Stop() 释放资源
4. 定期检查 `Stats()` 和 `GetBackpressureInfo()` 监控系统状态
5. 背压阈值应根据实际处理能力合理设置，避免频繁暂停/恢复
