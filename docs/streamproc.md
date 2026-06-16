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
