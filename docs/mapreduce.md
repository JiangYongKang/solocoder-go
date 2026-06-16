# MapReduce 模块需求文档

## 1. 模块概述

MapReduce 是一个分布式计算框架模块，提供 Map 和 Reduce 两阶段并行计算能力。它支持将输入数据按可配置的分区策略切分为多个分片并行处理，中间结果按 Key 哈希分区后路由到对应的 Reduce 任务，最终将各 Reduce 任务的输出合并为最终结果。

### 主要特性

- **Map/Reduce 任务分发**：输入数据按分片分配给 Map 任务并行处理，Map 输出的中间键值对按 Key 分组后分发给对应的 Reduce 任务
- **中间数据 Shuffle**：支持同步（ShuffleSync）和异步（ShuffleAsync）两种 Shuffle 模式，Reduce 任务在收到对应分区的全部中间数据后开始执行
- **任务失败重试**：Map 或 Reduce 任务执行失败时自动重试，重试次数可配置，重试时重新执行完整计算逻辑而非从失败点续算
- **结果合并**：所有 Reduce 任务完成后合并各分区输出，支持简单拼接或自定义合并函数

## 2. 核心结构体

### 2.1 KeyValue

```go
type KeyValue struct {
    Key   string
    Value interface{}
}
```

**职责**：表示一个键值对，是 Map 阶段输出和 Reduce 阶段输入/输出的基本数据单元。

| 字段 | 说明 |
|------|------|
| Key | 键，用于分组和分区路由 |
| Value | 值，Map 阶段输出的中间值或 Reduce 阶段的计算结果 |

### 2.2 Config

```go
type Config struct {
    MapFunc       MapFunc        // Map 函数，必填
    ReduceFunc    ReduceFunc     // Reduce 函数，必填
    NumReduce     int            // Reduce 任务数量，必须 > 0
    MaxRetries    int            // 最大重试次数，必须 >= 0
    ShuffleMode   ShuffleMode    // Shuffle 模式：ShuffleSync 或 ShuffleAsync
    PartitionFunc PartitionFunc  // 分区函数，为空时默认使用 HashPartition
    MergeFunc     MergeFunc      // 自定义合并函数，为空时简单拼接各分区结果
}
```

**职责**：定义 MapReduce 作业的完整配置参数。

| 字段 | 说明 |
|------|------|
| MapFunc | 用户定义的 Map 函数，处理输入的每个键值对，输出一组中间键值对 |
| ReduceFunc | 用户定义的 Reduce 函数，对同一 Key 的所有 Values 进行聚合 |
| NumReduce | Reduce 任务数量，决定中间数据被分为多少个分区 |
| MaxRetries | 任务失败后的最大重试次数（0 表示不重试） |
| ShuffleMode | Shuffle 模式，同步或异步 |
| PartitionFunc | 将 Key 映射到 Reduce 分区编号的函数 |
| MergeFunc | 将所有 Reduce 分区结果合并为最终结果的函数 |

### 2.3 MapFunc

```go
type MapFunc func(ctx context.Context, key string, value interface{}) ([]KeyValue, error)
```

**职责**：用户定义的 Map 函数，接收一个输入键值对，输出零到多个中间键值对。

### 2.4 ReduceFunc

```go
type ReduceFunc func(ctx context.Context, key string, values []interface{}) (interface{}, error)
```

**职责**：用户定义的 Reduce 函数，接收一个 Key 及其对应的所有 Value，输出一个聚合结果。

### 2.5 PartitionFunc

```go
type PartitionFunc func(key string, numPartitions int) int
```

**职责**：将中间键值对的 Key 映射到 [0, numPartitions) 范围内的分区编号。模块提供默认实现 `HashPartition`，使用 FNV-1a 哈希算法。

### 2.6 MergeFunc

```go
type MergeFunc func(results []interface{}) (interface{}, error)
```

**职责**：将所有 Reduce 分区的输出合并为最终结果。每个分区结果为 `[]KeyValue` 类型（空分区为 `nil`）。未设置时，默认将所有分区结果拼接为 `[]interface{}`。

### 2.7 ShuffleMode

```go
type ShuffleMode int

const (
    ShuffleSync  ShuffleMode = iota  // 同步 Shuffle
    ShuffleAsync                       // 异步 Shuffle
)
```

| 模式 | 说明 |
|------|------|
| ShuffleSync | 等待所有 Map 任务完成后，统一将中间数据按 Key 分区路由到对应 Reduce 任务，再开始执行 Reduce |
| ShuffleAsync | Map 任务完成一个就立即将其中间数据分区路由，Reduce 任务在有数据到达时即可开始处理，全部 Map 完成后所有 Reduce 开始最终处理 |

### 2.8 TaskStatus

```go
type TaskStatus int

const (
    TaskStatusPending   TaskStatus = iota  // 等待执行
    TaskStatusRunning                       // 正在执行
    TaskStatusCompleted                     // 执行完成
    TaskStatusFailed                        // 执行失败
)
```

**职责**：表示 MapReduce 作业的当前状态。

### 2.9 TaskError

```go
type TaskError struct {
    TaskType string  // "map" 或 "reduce"
    TaskID   int     // 任务编号
    Attempt  int     // 尝试次数
    Err      error   // 原始错误
}
```

**职责**：封装任务失败的详细信息，支持 `Unwrap()` 以便使用 `errors.Is()` 检查内部错误。

### 2.10 MapReduce

```go
type MapReduce struct {
    // ... 内部字段省略
}
```

**职责**：MapReduce 作业核心管理器，负责：
- 管理 Map 任务的并行执行与重试
- 执行 Shuffle 过程（同步或异步模式）
- 管理 Reduce 任务的并行执行与重试
- 合并 Reduce 结果并提供最终输出
- 维护作业状态与错误记录
- 支持上下文取消与作业取消

## 3. 数据流转路径

数据从 Map 到 Shuffle 再到 Reduce 的完整流转路径：

```
                    ┌──────────────────┐
                    │   输入数据        │
                    │ []KeyValue       │
                    └────────┬─────────┘
                             │
                             │ 每个 KeyValue 对应一个 Map 任务
                             │ Map 任务并行执行
                             ▼
              ┌──────────────────────────────┐
              │       Map 阶段               │
              │  MapFunc(key, value)         │
              │       → []KeyValue           │
              └──────────────┬───────────────┘
                             │
                             │ Map 输出的中间键值对
                             ▼
              ┌──────────────────────────────┐
              │     Shuffle 阶段             │
              │                              │
              │  对每个中间 KeyValue:          │
              │  partition = PartitionFunc(  │
              │      kv.Key, NumReduce)      │
              │                              │
              │  ┌─────────────────────┐     │
              │  │ ShuffleSync 模式:    │     │
              │  │ 等待所有 Map 完成后   │     │
              │  │ 统一分区路由         │     │
              │  └─────────────────────┘     │
              │  ┌─────────────────────┐     │
              │  │ ShuffleAsync 模式:   │     │
              │  │ Map 完成一个就路由   │     │
              │  │ Reduce 可提前开始    │     │
              │  └─────────────────────┘     │
              └──────────────┬───────────────┘
                             │
                             │ 中间数据按 Key 分组
                             │ 路由到对应的 Reduce 分区
                             ▼
              ┌──────────────────────────────┐
              │     Reduce 阶段              │
              │                              │
              │  对每个分区（共 NumReduce 个）│
              │  按 Key 分组:                 │
              │    key → []interface{}        │
              │                              │
              │  对每个 Key 调用:              │
              │  ReduceFunc(key, values)      │
              │       → result               │
              │                              │
              │  分区结果: []KeyValue          │
              │  (空分区为 nil)               │
              └──────────────┬───────────────┘
                             │
                             │ 各分区 Reduce 结果
                             ▼
              ┌──────────────────────────────┐
              │     合并阶段                 │
              │                              │
              │  ┌─────────────────────┐     │
              │  │ 有 MergeFunc:        │     │
              │  │ 调用自定义合并函数   │     │
              │  │ 合并各分区结果       │     │
              │  └─────────────────────┘     │
              │  ┌─────────────────────┐     │
              │  │ 无 MergeFunc:        │     │
              │  │ 简单拼接为           │     │
              │  │ []interface{}        │     │
              │  └─────────────────────┘     │
              └──────────────┬───────────────┘
                             │
                             ▼
                    ┌──────────────────┐
                    │   最终输出        │
                    └──────────────────┘
```

### 数据流转详细说明

1. **输入阶段**：调用 `SetInput()` 设置输入数据 `[]KeyValue`，每个元素作为一个 Map 任务的输入
2. **Map 阶段**：每个 Map 任务并行调用 `MapFunc(key, value)`，输出 `[]KeyValue` 中间键值对
3. **Shuffle 阶段**：
   - 对每个中间 KeyValue，调用 `PartitionFunc(key, NumReduce)` 计算分区编号
   - 同一 Key 的所有 Value 被路由到同一个 Reduce 分区
   - 按 Key 分组后，每个分区维护一个 `map[string][]interface{}`
4. **Reduce 阶段**：每个 Reduce 分区并行执行，对分区内的每个 Key 调用 `ReduceFunc(key, values)`，输出结果以 `[]KeyValue` 形式保存
5. **合并阶段**：收集所有分区的 Reduce 结果，通过 `MergeFunc`（自定义）或简单拼接合并为最终输出

## 4. 核心算法与策略

### 4.1 哈希分区策略

默认使用 FNV-1a 哈希算法将 Key 映射到分区编号：

```go
func HashPartition(key string, numPartitions int) int {
    h := fnv.New32a()
    h.Write([]byte(key))
    return int(h.Sum32()) % numPartitions
}
```

用户可通过 `Config.PartitionFunc` 提供自定义分区函数。

### 4.2 任务重试策略

Map 和 Reduce 任务失败时自动重试：

- 初始执行 + 最多 `MaxRetries` 次重试
- 每次重试重新执行该任务的**完整计算逻辑**，不从失败点续算
- 超过最大重试次数后，任务标记为最终失败并上报 `TaskError`
- Map 任务失败：整个作业失败，返回错误
- Reduce 任务失败：整个作业失败，返回错误

```
attempt 0 (初始执行)
  ├── 成功 → 返回结果
  └── 失败 → attempt 1 (重试)
                ├── 成功 → 返回结果
                └── 失败 → attempt 2 (重试)
                              ├── 成功 → 返回结果
                              └── 失败 → ... → attempt == MaxRetries → 最终失败
```

### 4.3 Panic 恢复

Map 和 Reduce 函数执行通过 `safeExecuteMap` / `safeExecuteReduce` 包裹，自动捕获 panic 并转化为错误：

```go
func (mr *MapReduce) safeExecuteMap(input KeyValue) (kvs []KeyValue, err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("map task panicked: %v", r)
            kvs = nil
        }
    }()
    return mr.cfg.MapFunc(mr.ctx, input.Key, input.Value)
}
```

Panic 被捕获后视为普通错误，遵循重试策略处理。

### 4.4 上下文取消

- `Run(ctx)` 接受 `context.Context`，支持超时和取消
- `Cancel()` 方法主动取消正在执行的作业
- 任务在执行前检查 `ctx.Done()`，如果已取消则立即返回

## 5. API 使用示例

### 5.1 基本 WordCount

```go
package main

import (
    "context"
    "fmt"
    "strings"
    "solocoder-go/internal/mapreduce"
)

func main() {
    mr, err := mapreduce.NewMapReduce(mapreduce.Config{
        MapFunc: func(_ context.Context, key string, value interface{}) ([]mapreduce.KeyValue, error) {
            words := strings.Fields(value.(string))
            var kvs []mapreduce.KeyValue
            for _, w := range words {
                kvs = append(kvs, mapreduce.KeyValue{Key: w, Value: 1})
            }
            return kvs, nil
        },
        ReduceFunc: func(_ context.Context, key string, values []interface{}) (interface{}, error) {
            sum := 0
            for _, v := range values {
                sum += v.(int)
            }
            return sum, nil
        },
        NumReduce:   2,
        MaxRetries:  2,
        ShuffleMode: mapreduce.ShuffleSync,
    })
    if err != nil {
        panic(err)
    }

    mr.SetInput([]mapreduce.KeyValue{
        {Key: "doc1", Value: "hello world hello"},
        {Key: "doc2", Value: "world go world"},
    })

    result, err := mr.Run(context.Background())
    if err != nil {
        panic(err)
    }

    partitions := result.([]interface{})
    wordCounts := make(map[string]int)
    for _, partition := range partitions {
        if partition == nil {
            continue
        }
        for _, kv := range partition.([]mapreduce.KeyValue) {
            wordCounts[kv.Key] += kv.Value.(int)
        }
    }

    fmt.Printf("hello: %d\n", wordCounts["hello"])
    fmt.Printf("world: %d\n", wordCounts["world"])
    fmt.Printf("go: %d\n", wordCounts["go"])
}
```

### 5.2 自定义合并函数

```go
mr, _ := mapreduce.NewMapReduce(mapreduce.Config{
    MapFunc:    wordCountMap,
    ReduceFunc: wordCountReduce,
    NumReduce:  4,
    MergeFunc: func(results []interface{}) (interface{}, error) {
        merged := make(map[string]int)
        for _, r := range results {
            if r == nil {
                continue
            }
            for _, kv := range r.([]mapreduce.KeyValue) {
                merged[kv.Key] += kv.Value.(int)
            }
        }
        return merged, nil
    },
})

result, _ := mr.Run(context.Background())
wordCounts := result.(map[string]int)
fmt.Printf("word counts: %v\n", wordCounts)
```

### 5.3 自定义分区函数

```go
mr, _ := mapreduce.NewReduce(mapreduce.Config{
    MapFunc:    myMapFunc,
    ReduceFunc: myReduceFunc,
    NumReduce:  2,
    PartitionFunc: func(key string, numPartitions int) int {
        if key < "m" {
            return 0
        }
        return 1
    },
})
```

### 5.4 异步 Shuffle 模式

```go
mr, _ := mapreduce.NewMapReduce(mapreduce.Config{
    MapFunc:     myMapFunc,
    ReduceFunc:  myReduceFunc,
    NumReduce:   4,
    ShuffleMode: mapreduce.ShuffleAsync,
})
```

### 5.5 查询作业状态与错误

```go
result, err := mr.Run(context.Background())
if err != nil {
    fmt.Printf("作业失败: %v\n", err)

    for _, te := range mr.Errors() {
        fmt.Printf("任务错误: type=%s id=%d attempt=%d err=%v\n",
            te.TaskType, te.TaskID, te.Attempt, te.Err)
    }
    return
}

fmt.Printf("完成的 Map 任务数: %d\n", mr.CompletedMapCount())
fmt.Printf("完成的 Reduce 任务数: %d\n", mr.CompletedReduceCount())
fmt.Printf("作业状态: %d\n", mr.Status())
```

### 5.6 上下文超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := mr.Run(ctx)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("作业超时")
    }
}
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrNoInputData` | 未设置输入数据就调用 `Run()` |
| `ErrNoMapFunc` | 创建 MapReduce 时未提供 MapFunc |
| `ErrNoReduceFunc` | 创建 MapReduce 时未提供 ReduceFunc |
| `ErrInvalidReduceNum` | NumReduce <= 0 |
| `ErrInvalidRetry` | MaxRetries < 0 |
| `ErrAlreadyRunning` | 作业正在运行时再次调用 `Run()` |

任务级别的错误通过 `TaskError` 结构体记录，包含任务类型（map/reduce）、任务编号、尝试次数和原始错误。可通过 `Errors()` 方法获取所有任务错误。

## 7. 线程安全说明

MapReduce 所有公共方法均为**并发安全**：
- 内部使用 `sync.Mutex` 保护中间数据分区
- 使用 `sync.RWMutex` 保护作业状态
- 使用 `sync/atomic` 进行计数器操作
- Panic 恢复确保单个任务崩溃不影响其他任务

## 8. 生命周期

- **创建**：`NewMapReduce(cfg)` 创建实例，验证配置参数
- **设置输入**：`SetInput(input)` 设置输入数据
- **执行**：`Run(ctx)` 启动作业，阻塞直到完成或失败
- **查询**：`Status()`、`Result()`、`Errors()`、`CompletedMapCount()`、`CompletedReduceCount()` 查询作业状态
- **取消**：`Cancel()` 主动取消正在执行的作业
