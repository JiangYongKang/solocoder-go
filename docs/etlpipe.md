# ETL 数据管道模块

## 一、模块概述

`etlpipe` 是一个灵活、可扩展的 ETL（Extract-Transform-Load）数据管道模块，提供从多数据源提取、多阶段转换到目标端批量写入的完整数据处理能力。模块具备以下核心特性：

- **多数据源支持**：可注册多种数据源类型，统一数据提取接口
- **全量/增量提取**：支持全量提取和增量提取，增量模式下通过时间戳或自增 ID 追踪进度
- **多阶段转换链**：支持字段映射、类型转换、值替换、字段过滤、字段计算等转换规则，执行顺序可灵活编排
- **批量写入容错**：支持配置批量写入大小和超时时间，单条记录写入失败不中断整批
- **错误隔离机制**：转换错误和写入错误均被隔离记录，不影响整体处理流程

## 二、核心结构体职责

### 2.1 数据模型

#### `Record`
数据记录的基础单元，包含：
- `ID`：记录唯一标识
- `Data`：键值对形式的字段数据
- `Timestamp`：记录时间戳
- `SeqID`：自增序列 ID

提供 `GetField`、`SetField`、`DeleteField`、`Clone` 等字段操作方法。

#### `Batch`
批量数据集合，包含：
- `Records`：记录数组
- `FirstSeq` / `LastSeq`：批次首尾序列 ID
- `StartTs` / `EndTs`：批次首尾时间戳
- `Size()`：返回批次记录数

#### `Cursor`
提取进度游标，用于增量提取：
- `Mode`：提取模式（全量/时间戳/ID）
- `LastValue`：最后提取的增量值
- `LastOffset`：已处理的记录偏移量
- `UpdateTime`：游标更新时间

### 2.2 数据源层

#### `Source` 接口
统一数据源提取接口，任何数据源只需实现：
```go
type Source interface {
    Fetch(ctx context.Context, cursor *Cursor, batchSize int) (*Batch, error)
    Count(ctx context.Context, cursor *Cursor) (int64, error)
    Close(ctx context.Context) error
}
```

#### `SourceRegistry`
数据源类型注册中心，支持：
- `Register(sourceType, factory)`：注册数据源工厂
- `Create(sourceType, config)`：根据类型创建数据源实例
- `Has` / `List` / `Unregister`：查询和管理已注册类型

#### `MemorySource`
内置内存数据源实现，用于测试和演示，支持：
- 自定义数据记录
- 配置提取延迟模拟 IO 等待
- 配置提取错误模拟异常场景
- 游标进度追踪

### 2.3 转换层

#### `Transformer` 接口
转换单元接口：
```go
type Transformer interface {
    Name() string
    Transform(record *Record) (*Record, error)
}
```

#### `BaseTransformer`
内置基础转换器，支持 5 种转换规则类型：

| 类型 | 说明 | 结构体字段 |
|------|------|-----------|
| `TransformTypeFieldMap` | 字段重命名/映射 | `FieldMappings` |
| `TransformTypeTypeConvert` | 字段类型转换 | `TypeConversions` |
| `TransformTypeValueReplace` | 字段值替换 | `Replacements` |
| `TransformTypeFieldFilter` | 字段保留/移除 | `Filter` |
| `TransformTypeFieldCalculate` | 自定义字段计算 | `Calculation` |

支持的类型转换目标类型：`string`、`int`、`int64`、`float64`、`bool`、`time`。

#### `TransformChain`
转换链，按顺序编排多个转换器：
- `Add`：追加转换器
- `Insert(index, t)`：在指定位置插入
- `Remove(index)`：移除指定位置转换器
- `List` / `Count`：查询转换器列表
- `Process(record)`：执行完整转换链

### 2.4 错误隔离层

#### `TransformError`
转换错误记录，包含：
- `StageName` / `StageIndex`：发生错误的转换阶段
- `Record`：原始记录快照
- `Err`：具体错误原因
- `Timestamp`：错误发生时间

#### `WriteError`
写入错误记录，包含：
- `Record`：写入失败的记录
- `Err`：错误原因
- `Timestamp`：错误时间

#### `ErrorQueue`
错误队列，隔离存储转换和写入错误：
- 支持最大容量限制（超出时丢弃最早的错误）
- `AddTransformError` / `AddWriteError`：添加错误
- `GetTransformErrors` / `GetWriteErrors`：获取错误列表
- `TransformErrorCount` / `WriteErrorCount` / `TotalErrorCount`：统计
- `Clear()`：清空错误队列

### 2.5 写入层

#### `Target` 接口
目标端写入接口：
```go
type Target interface {
    WriteBatch(ctx context.Context, records []*Record) ([]int, error)
    Close(ctx context.Context) error
}
```
`WriteBatch` 返回失败记录的索引数组，失败的单条记录不影响其他记录的写入。

#### `MemoryTarget`
内置内存目标端实现，支持：
- 配置指定 ID 的记录模拟写入失败
- `Count` / `GetAll` / `Clear`：数据查询和清理

### 2.6 管道核心

#### `Pipeline`
ETL 管道主结构体，协调整个数据流程：

**配置项 `Config`**：
- `BatchSize`：每批提取/写入的记录条数上限
- `WriteTimeout`：写入操作超时时间
- `ExtractMode`：提取模式（全量/时间戳/ID）
- `IncrementalField`：增量提取字段名
- `MaxErrorQueueSize`：错误队列最大容量

**运行状态 `PipelineStatus`**：
- `Idle`：待运行
- `Running`：运行中
- `Completed`：正常完成
- `Failed`：异常失败
- `Stopped`：主动停止

**统计信息 `PipelineStats`**：
- `ExtractedCount`：已提取记录数
- `TransformedCount`：成功转换记录数
- `WrittenCount`：成功写入记录数
- `TransformErrorCount`：转换错误数
- `WriteErrorCount`：写入错误数
- `BatchCount`：已处理批次数
- `ElapsedTime` / `StartTime`：耗时信息

## 三、完整 ETL 流程

```
┌─────────────────┐     ┌──────────────────────────┐     ┌─────────────────┐
│   数据源 Source  │────▶│      Pipeline 主循环      │────▶│   目标端 Target  │
│                 │     │                          │     │                 │
│  · Fetch(cursor)│     │  1. 提取下一批数据        │     │  · WriteBatch() │
│  · Count()      │     │  2. 更新 ExtractedCount  │     │                 │
└─────────────────┘     │  3. 执行转换链 Process   │     └─────────────────┘
                        │     │                      │
                        │     ▼                      │
                        │  ┌───┴──────────────────┐  │
                        │  │  每条记录转换成功？   │  │
                        │  └───┬──────────────────┘  │
                        │    是│         │否          │
                        │      ▼         ▼           │
                        │  Transformed  AddTransformError│
                        │  Records++   TransformError++ │
                        │      │                       │
                        │      ▼                       │
                        │  4. 批量写入 WriteBatch      │
                        │     │                        │
                        │     ▼                        │
                        │  ┌──┴───────────────────┐   │
                        │  │  写入结果处理         │   │
                        │  └──┬───────────────────┘   │
                        │   成功│        │失败索引     │
                        │     ▼        ▼              │
                        │ Written++  AddWriteError    │
                        │            WriteError++     │
                        │      │                       │
                        │      ▼                       │
                        │  5. 更新游标 Cursor          │
                        │  6. BatchCount++             │
                        │  7. 循环直到无数据           │
                        └──────────────────────────────┘
```

### 详细执行步骤：

1. **初始化**：创建 Pipeline，校验配置有效性，初始化游标、错误队列
2. **提取循环**：
   - 调用 `Source.Fetch(cursor, batchSize)` 提取下一批数据
   - 如果提取返回空批次，正常结束
   - 如果提取报错，管道标记为 Failed 并返回错误
3. **逐记录转换**：
   - 对批次中每条记录执行转换链
   - 转换成功：加入成功列表，TransformedCount++
   - 转换失败：封装 TransformError 入错误队列，TransformErrorCount++，**继续处理下一条**
4. **批量写入**：
   - 对成功转换的记录调用 `Target.WriteBatch()`，带超时控制
   - 如果整体写入超时/报错：所有记录标记为 WriteError 入队列
   - 如果部分记录失败（返回失败索引）：失败记录入 WriteError 队列，其余成功 WrittenCount++
5. **进度更新**：根据提取模式更新游标（LastOffset、LastValue）
6. **状态流转**：所有数据处理完成后，状态转为 Completed；主动 Stop 转为 Stopped；异常报错转为 Failed

## 四、使用示例

### 4.1 基本使用：完整 ETL 流程

```go
package main

import (
    "context"
    "fmt"
    "solocoder-go/internal/etlpipe"
)

func main() {
    // 1. 准备测试数据
    records := make([]*etlpipe.Record, 100)
    for i := 0; i < 100; i++ {
        records[i] = &etlpipe.Record{
            ID:    fmt.Sprintf("user-%d", i),
            SeqID: int64(i + 1),
            Data: map[string]interface{}{
                "username":    fmt.Sprintf("user%d", i),
                "age_str":     fmt.Sprintf("%d", 20+i%30),
                "raw_status":  "A",
                "score":       float64(60 + i),
                "unused":      "remove_me",
            },
        }
    }

    // 2. 创建数据源和目标端
    source := etlpipe.NewMemorySource(records)
    target := etlpipe.NewMemoryTarget()

    // 3. 配置管道（增量模式，按自增 ID 追踪）
    cfg := etlpipe.Config{
        BatchSize:        20,
        WriteTimeout:     30 * time.Second,
        ExtractMode:      etlpipe.ExtractModeID,
        IncrementalField: "id",
    }

    // 4. 创建管道
    pipeline, err := etlpipe.NewPipeline(cfg, source, target)
    if err != nil {
        panic(err)
    }

    // 5. 添加多阶段转换器
    // 阶段1：类型转换
    _ = pipeline.AddTransformer(etlpipe.NewBaseTransformer("type_convert", []etlpipe.TransformRule{
        {
            Name: "age_to_int",
            Type: etlpipe.TransformTypeTypeConvert,
            TypeConversions: []etlpipe.TypeConversion{
                {Field: "age_str", TargetType: "int"},
            },
        },
    }))

    // 阶段2：值替换
    _ = pipeline.AddTransformer(etlpipe.NewBaseTransformer("status_map", []etlpipe.TransformRule{
        {
            Name: "status_replace",
            Type: etlpipe.TransformTypeValueReplace,
            Replacements: []etlpipe.ValueReplacement{
                {Field: "raw_status", Old: "A", New: "ACTIVE"},
                {Field: "raw_status", Old: "I", New: "INACTIVE"},
            },
        },
    }))

    // 阶段3：字段过滤
    _ = pipeline.AddTransformer(etlpipe.NewBaseTransformer("cleanup", []etlpipe.TransformRule{
        {
            Name: "remove_unused",
            Type: etlpipe.TransformTypeFieldFilter,
            Filter: etlpipe.FieldFilter{
                RemoveFields: []string{"unused"},
            },
        },
    }))

    // 阶段4：字段计算
    _ = pipeline.AddTransformer(etlpipe.NewBaseTransformer("add_grade", []etlpipe.TransformRule{
        {
            Name: "score_grade",
            Type: etlpipe.TransformTypeFieldCalculate,
            Calculation: &etlpipe.FieldCalculation{
                TargetField: "grade",
                Calculator: func(data map[string]interface{}) (interface{}, error) {
                    score, _ := data["score"].(float64)
                    switch {
                    case score >= 90:
                        return "A", nil
                    case score >= 80:
                        return "B", nil
                    default:
                        return "C", nil
                    }
                },
            },
        },
    }))

    // 6. 运行管道
    ctx := context.Background()
    if err := pipeline.Run(ctx); err != nil {
        fmt.Printf("Pipeline failed: %v\n", err)
        return
    }

    // 7. 查看结果
    stats := pipeline.Stats()
    fmt.Printf("Status: %s\n", pipeline.Status())
    fmt.Printf("Extracted: %d, Transformed: %d, Written: %d\n",
        stats.ExtractedCount, stats.TransformedCount, stats.WrittenCount)
    fmt.Printf("Transform errors: %d, Write errors: %d\n",
        stats.TransformErrorCount, stats.WriteErrorCount)
    fmt.Printf("Batches: %d, Elapsed: %v\n", stats.BatchCount, stats.ElapsedTime)
    fmt.Printf("Target records: %d\n", target.Count())

    // 8. 查看错误队列
    eq := pipeline.GetErrorQueue()
    fmt.Printf("Total errors in queue: %d\n", eq.TotalErrorCount())
}
```

### 4.2 使用数据源注册中心

```go
// 注册自定义数据源类型
registry := etlpipe.NewSourceRegistry()
err := registry.Register("memory", etlpipe.NewMemorySourceFactory())
if err != nil {
    panic(err)
}

// 通过注册中心创建数据源
source, err := registry.Create("memory", map[string]interface{}{
    "records": records,
})
```

### 4.3 错误处理场景

```go
// 模拟写入失败
target.SetFailRecord("user-15")
target.SetFailRecord("user-