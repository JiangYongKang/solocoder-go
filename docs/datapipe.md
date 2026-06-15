# DataPipe 数据迁移管道模块需求文档

## 1. 模块概述

DataPipe 是一个通用的数据迁移管道模块，提供从源端数据存储到目标端数据存储的批量同步能力。它支持按批次原子处理、增量迁移、断点续传以及实时进度上报，适用于数据库迁移、数据仓库 ETL、跨存储数据同步等场景。

### 主要特性

- **批次同步**：以批次为单位从源端读取并写入目标端，每个批次作为独立的原子执行单元
- **增量迁移**：支持基于时间戳（Timestamp）或递增标识（ID）两种增量模式，仅迁移变更数据
- **断点续传**：迁移过程中自动记录已完成的批次位置，任务中断后可从最近断点恢复
- **进度上报**：定期通过回调接口上报迁移进度，包括已处理条数、总数、速率、百分比等
- **失败重试**：写入批次失败时按指数退避策略自动重试，超过最大次数则终止
- **优雅停止**：支持调用 Stop() 安全停止迁移，已处理批次不会重复
- **上下文取消**：支持通过 context.Context 取消迁移任务

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    BatchSize            int           // 每批次记录数，必须 > 0
    IncrementalMode      IncrementalMode // 增量迁移模式
    IncrementalField     string        // 增量字段名（非全量模式时必填）
    EnableCheckpoint     bool          // 是否启用断点续传
    ProgressInterval     time.Duration // 进度上报间隔，0 表示仅在结束时上报
    TimeoutPerBatch      time.Duration // 单批次读取/写入超时
    MaxRetryPerBatch     int           // 单批次写入最大重试次数
    RetryBackoff         time.Duration // 重试基础退避间隔
}
```

**职责**：定义迁移管道的配置参数，在创建 Pipeline 实例时传入。

### 2.2 IncrementalMode

```go
type IncrementalMode int

const (
    IncrementalModeFull      IncrementalMode = iota // 全量迁移
    IncrementalModeTimestamp                         // 基于时间戳增量
    IncrementalModeID                                // 基于递增 ID 增量
)
```

**职责**：枚举增量迁移的三种模式。

### 2.3 Cursor

```go
type Cursor struct {
    Mode       IncrementalMode // 当前增量模式
    LastValue  interface{}     // 最后处理的增量值（时间戳或 ID）
    LastOffset int64           // 已处理的总记录数
    UpdateTime time.Time       // 最后更新时间
}
```

**职责**：记录迁移进度的断点游标，用于断点续传。

| 字段 | 说明 |
|------|------|
| Mode | 迁移模式，与 Config.IncrementalMode 一致 |
| LastValue | 全量模式下忽略；时间戳模式为最后一条记录的 Timestamp；ID 模式为最后一条记录的 SeqID |
| LastOffset | 已处理的累计记录条数，用于全量模式定位起点 |
| UpdateTime | 断点最后一次保存的时间戳 |

### 2.4 Record

```go
type Record struct {
    ID        string
    Data      map[string]interface{}
    Timestamp time.Time
    SeqID     int64
}
```

**职责**：表示一条数据记录。

| 字段 | 说明 |
|------|------|
| ID | 记录唯一标识（字符串形式） |
| Data | 记录的业务字段键值对 |
| Timestamp | 记录的时间戳（用于时间戳增量模式） |
| SeqID | 记录的递增序列号（用于 ID 增量模式） |

提供辅助方法 `GetField(name string) (interface{}, bool)` 从 Data 中按字段名取值。

### 2.5 Batch

```go
type Batch struct {
    ID       int64
    Records  []*Record
    FirstSeq int64
    LastSeq  int64
    StartTs  time.Time
    EndTs    time.Time
}
```

**职责**：表示一个批次的数据，是迁移的最小原子单元。

| 字段 | 说明 |
|------|------|
| ID | 批次序号（从 1 开始） |
| Records | 本批次包含的记录列表 |
| FirstSeq / LastSeq | 本批次第一条和最后一条记录的 SeqID |
| StartTs / EndTs | 本批次第一条和最后一条记录的 Timestamp |

提供辅助方法 `Size() int` 返回批次记录数。

### 2.6 Source

```go
type Source interface {
    Fetch(ctx context.Context, cursor *Cursor, batchSize int) (*Batch, error)
    Count(ctx context.Context, cursor *Cursor) (int64, error)
    Close(ctx context.Context) error
}
```

**职责**：源端数据存储的抽象接口，使用方需自行实现。

| 方法 | 说明 |
|------|------|
| Fetch | 按 cursor 指定的位置读取 batchSize 条记录，返回空 Batch 表示数据已读完 |
| Count | 统计 cursor 之后的剩余记录数（用于进度显示，失败不影响迁移） |
| Close | 释放源端资源（迁移结束时自动调用） |

### 2.7 Target

```go
type Target interface {
    Write(ctx context.Context, batch *Batch) error
    Close(ctx context.Context) error
}
```

**职责**：目标端数据存储的抽象接口，使用方需自行实现。

| 方法 | 说明 |
|------|------|
| Write | 将一个批次的记录写入目标存储，应保证原子性 |
| Close | 释放目标端资源（迁移结束时自动调用） |

### 2.8 CheckpointStore

```go
type CheckpointStore interface {
    Save(ctx context.Context, cursor *Cursor) error
    Load(ctx context.Context) (*Cursor, error)
    Clear(ctx context.Context) error
}
```

**职责**：断点存储的抽象接口。

模块内置 `NewMemoryCheckpointStore()` 返回基于内存的实现（进程重启后丢失）。生产环境使用方需自行实现持久化存储（如数据库、文件等）。

### 2.9 ProgressInfo

```go
type ProgressInfo struct {
    Processed    int64
    Total        int64
    Batches      int64
    RatePerSec   float64
    Percent      float64
    Elapsed      time.Duration
    Remaining    time.Duration
    CurrentBatch int64
}
```

**职责**：封装迁移进度信息，通过 ProgressCallback 回调对外通知。

| 字段 | 说明 |
|------|------|
| Processed | 已处理记录数 |
| Total | 总记录数（Count 成功时才有值） |
| Batches | 已完成批次数 |
| RatePerSec | 每秒处理速率（条/秒） |
| Percent | 完成百分比（0-100） |
| Elapsed | 已耗时时长 |
| Remaining | 预计剩余时长（基于速率估算） |
| CurrentBatch | 当前正在处理的批次号 |

### 2.10 ProgressCallback

```go
type ProgressCallback func(info ProgressInfo)
```

**职责**：进度回调函数类型，使用方通过 `SetProgressCallback()` 注册。

### 2.11 PipelineStatus

```go
type PipelineStatus int

const (
    PipelineStatusIdle      PipelineStatus = iota // 空闲，未启动
    PipelineStatusRunning                          // 运行中
    PipelineStatusPaused                           // 暂停（预留）
    PipelineStatusCompleted                        // 正常完成
    PipelineStatusFailed                           // 异常失败
    PipelineStatusStopped                          // 被 Stop() 停止
)
```

**职责**：枚举迁移管道的生命周期状态，提供 `String()` 方法获取可读描述。

### 2.12 Pipeline

```go
type Pipeline struct {
    // 内部字段省略
}
```

**职责**：数据迁移管道的核心管理器，负责：
- 协调 Source 读取与 Target 写入
- 管理批次循环处理
- 维护断点 Cursor 并持久化到 CheckpointStore
- 调度进度上报
- 执行写入失败重试
- 管理上下文取消与用户停止

## 3. 完整迁移流程

### 3.1 流程总览

```
┌───────────────────────────────────────────────────────────────────┐
│                        NewPipeline()                              │
│  校验参数 → 创建实例 → 状态 = Idle                                  │
└─────────────────────────────┬─────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                           Run(ctx)                                │
├───────────────────────────────────────────────────────────────────┤
│ 1. 状态 = Running                                                  │
│ 2. initCursor()：从 CheckpointStore 加载断点 或 创建新 Cursor       │
│ 3. Source.Count()：获取总数（失败则忽略，Total=0）                  │
│ 4. startTime = now                                                 │
│ 5. 启动进度上报 goroutine（若设置了 ProgressInterval）              │
│ 6. 进入 runLoop 批次循环 ←───┐                                     │
│ 7. 停止进度上报 goroutine    │                                     │
│ 8. 最后一次进度上报          │                                     │
│ 9. 状态 = Completed / Failed / Stopped                            │
│ 10. 调用 Source.Close() + Target.Close()                          │
└─────────────────────────────┬─────────────────────────────────────┘
                              │
                              ▼
┌───────────────────────────────────────────────────────────────────┐
│                       runLoop（批次循环）                           │
├───────────────────────────────────────────────────────────────────┤
│  for {                                                             │
│    ┌─ 检查 ctx.Done() / stopCh → 退出循环                          │
│    │                                                               │
│    ├─ fetchNextBatch() ────────────────────────────────┐          │
│    │   • 带超时从 Source.Fetch() 读取下一批             │          │
│    │   • 空批次 → break 循环                            │          │
│    │   • 出错 → 返回错误                                │          │
│    └────────────────────────────────────────────────────┘          │
│                              │                                    │
│                              ▼                                    │
│    ┌─ writeBatchWithRetry(batch) ──────────────────────┐          │
│    │   • 循环 MaxRetryPerBatch+1 次                    │          │
│    │   • 每次失败按指数退避等待后重试                   │          │
│    │   • 中途收到 Stop → 返回 ErrPipelineStopped       │          │
│    │   • 全部重试耗尽 → 返回最后错误                    │          │
│    └────────────────────────────────────────────────────┘          │
│                              │                                    │
│                              ▼                                    │
│    ┌─ updateCursor(batch) ─────────────────────────────┐          │
│    │   • 更新 Cursor.LastValue（按模式）                │          │
│    │   • Cursor.LastOffset += batch.Size()             │          │
│    │   • 若启用 Checkpoint → 持久化到存储              │          │
│    └────────────────────────────────────────────────────┘          │
│                              │                                    │
│                              ▼                                    │
│    ┌─ 更新原子计数器：Processed += batch.Size(), Batches++ ──┐    │
│    └────────────────────────────────────────────────────────┘    │
│  } ←───────────────────────────────────────────────────────┘      │
└───────────────────────────────────────────────────────────────────┘
```

### 3.2 阶段详细说明

#### 阶段 1：初始化与断点加载（initCursor）

1. 若 `EnableCheckpoint = false`，创建全新 Cursor（LastOffset=0）
2. 若启用断点：
   - 调用 `CheckpointStore.Load()` 加载已保存的断点
   - 加载成功且非 nil → 使用该断点作为起点
   - 加载失败 → 返回错误，终止迁移
   - 无断点（返回 nil）→ 创建全新 Cursor

#### 阶段 2：总数统计

- 调用 `Source.Count(ctx, cursor)` 获取剩余待迁移总数
- **成功**：将值存入 `total` 字段，用于百分比计算
- **失败**：静默忽略，`total = 0`，迁移正常继续（进度仅显示 Processed，无 Percent）

#### 阶段 3：批次循环（核心）

每个批次按顺序执行以下四步：

**Step 3.1：读取批次（fetchNextBatch）**
- 创建带 `TimeoutPerBatch` 超时的子 context
- 异步调用 `Source.Fetch()`
- 同时监听 `stopCh`：用户 Stop 则取消读取并返回空批次
- 返回空批次（Size=0）→ 表示源端已读完，正常退出循环

**Step 3.2：写入批次（writeBatchWithRetry）**
- 首次尝试直接调用 `Target.Write()`
- 若失败且 `MaxRetryPerBatch > 0`：
  - 按指数退避：`RetryBackoff * 2^(attempt-1)` 等待
  - 等待期间监听 ctx 取消和 stopCh
  - 重试成功 → 继续下一步
- 所有重试都失败 → 返回错误，迁移终止（状态=Failed）
- 收到 stopCh → 返回 `ErrPipelineStopped`（上层特殊处理）

**Step 3.3：更新断点（updateCursor）**
根据增量模式更新 Cursor.LastValue：
- **全量模式**：LastValue 不变
- **时间戳模式**：`LastValue = batch.EndTs`（最后一条记录的时间戳）
- **ID 模式**：`LastValue = batch.LastSeq`（最后一条记录的 SeqID）

LastOffset 累加本批次记录数。
若启用 Checkpoint → 调用 `CheckpointStore.Save()` 持久化。

**Step 3.4：更新统计计数**
- 原子递增 `processed`（已处理条数）
- 原子递增 `batches`（已完成批次数）

#### 阶段 4：结束处理

1. 退出循环原因判定：
   - 空批次（源端读完）→ 正常完成
   - ctx 取消 / Stop → 返回对应状态
   - 读取或写入错误 → 失败
2. 停止进度上报 goroutine
3. 触发最后一次进度上报（确保调用方收到 100% 的最终状态）
4. 设置最终状态：Completed / Failed / Stopped
5. 调用 Source.Close() 和 Target.Close() 释放资源

## 4. 断点续传机制

断点续传的核心思想：**每个批次写入成功并持久化断点后，该批次不再重复处理**。

### 4.1 一致性保证

```
Source.Fetch(批次 N)
        ↓ 成功读取
Target.Write(批次 N)  ←── 原子写入目标端
        ↓ 成功写入
CheckpointStore.Save(Cursor)  ←── 持久化断点
        ↓ 成功保存
processed += batch.Size()  ←── 更新计数
```

写入 + 断点保存两步全部成功后，批次才算"已完成"。若任一步失败：
- 写入失败 → 重试，重试耗尽则终止（下次重跑会从同一起点重新读取该批次）
- 断点保存失败 → 终止迁移，计数不更新（下次重跑可能重复写入该批次，使用方需保证幂等）

### 4.2 恢复流程

假设迁移在第 N 批次断点保存成功后崩溃：

1. 重新创建 Pipeline，传入同一 CheckpointStore
2. Run() → initCursor() 从存储加载 Cursor，LastOffset = 已处理记录数
3. Source.Fetch() 接收 cursor 参数，使用方实现应跳过前 LastOffset 条记录
4. 从第 N+1 批次开始继续迁移

> **重要**：使用方实现 Source.Fetch 时必须根据 cursor.LastOffset 或 cursor.LastValue 正确定位读取起点，否则断点续传无效。

## 5. 增量迁移机制

### 5.1 全量模式（IncrementalModeFull）

每次迁移都从源端读取全部数据。断点续传仍基于 LastOffset 定位。适用于：
- 源端无合适的时间戳或递增字段
- 数据量小，全量扫描成本可接受

### 5.2 时间戳增量（IncrementalModeTimestamp）

每次迁移仅读取时间戳大于断点值的记录。适用于：
- 记录包含创建时间或最后更新时间字段
- 新记录的时间戳严格递增

断点保存内容：`LastValue = batch.EndTs`（本批次最后一条记录的 Timestamp）

### 5.3 ID 增量（IncrementalModeID）

每次迁移仅读取递增 ID 大于断点值的记录。适用于：
- 记录拥有自增主键或单调递增的序列号
- ID 永不回退

断点保存内容：`LastValue = batch.LastSeq`（本批次最后一条记录的 SeqID）

## 6. 进度上报机制

### 6.1 触发时机

两种触发模式：
1. **定时触发**：若 Config.ProgressInterval > 0，启动后台 goroutine 按固定间隔调用回调
2. **结束触发**：Run() 返回前无论成功失败都调用一次回调，确保最终状态送达

### 6.2 统计量计算

```
RatePerSec   = Processed / Elapsed.Seconds()   (Elapsed > 0 时)
Percent      = Processed / Total * 100         (Total > 0 时)
Remaining    = (Total - Processed) / RatePerSec (RatePerSec > 0 时)
```

所有统计基于原子计数器读取，并发安全。

## 7. API 使用示例

### 7.1 基本使用：全量迁移

```go
package main

import (
    "context"
    "fmt"
    "log"
    "solocoder-go/internal/datapipe"
)

// 实现 Source 接口
type MySource struct {
    data []*datapipe.Record
}

func (s *MySource) Fetch(_ context.Context, cursor *datapipe.Cursor, batchSize int) (*datapipe.Batch, error) {
    start := int(cursor.LastOffset)
    if start >= len(s.data) {
        return &datapipe.Batch{Records: nil}, nil
    }
    end := start + batchSize
    if end > len(s.data) {
        end = len(s.data)
    }
    recs := make([]*datapipe.Record, end-start)
    copy(recs, s.data[start:end])
    b := &datapipe.Batch{Records: recs}
    if len(recs) > 0 {
        b.FirstSeq = recs[0].SeqID
        b.LastSeq = recs[len(recs)-1].SeqID
        b.StartTs = recs[0].Timestamp
        b.EndTs = recs[len(recs)-1].Timestamp
    }
    return b, nil
}

func (s *MySource) Count(_ context.Context, cursor *datapipe.Cursor) (int64, error) {
    return int64(len(s.data) - int(cursor.LastOffset)), nil
}

func (s *MySource) Close(context.Context) error { return nil }

// 实现 Target 接口
type MyTarget struct {
    mu     sync.Mutex
    stored []*datapipe.Record
}

func (t *MyTarget) Write(_ context.Context, batch *datapipe.Batch) error {
    t.mu.Lock()
    defer t.mu.Unlock()
    for _, r := range batch.Records {
        t.stored = append(t.stored, r)
    }
    return nil
}

func (t *MyTarget) Close(context.Context) error { return nil }

func main() {
    // 准备源数据
    srcData := make([]*datapipe.Record, 1000)
    for i := 0; i < 1000; i++ {
        srcData[i] = &datapipe.Record{
            ID:        fmt.Sprintf("rec-%d", i),
            SeqID:     int64(i + 1),
            Timestamp: time.Now().Add(time.Duration(i) * time.Second),
            Data:      map[string]interface{}{"idx": i},
        }
    }
    src := &MySource{data: srcData}
    tgt := &MyTarget{}

    // 创建迁移管道
    cfg := datapipe.Config{
        BatchSize:            100,
        IncrementalMode:      datapipe.IncrementalModeFull,
        EnableCheckpoint:     true,
        ProgressInterval:     500 * time.Millisecond,
        TimeoutPerBatch:      10 * time.Second,
        MaxRetryPerBatch:     3,
        RetryBackoff:         100 * time.Millisecond,
    }
    pipe, err := datapipe.NewPipeline(cfg, src, tgt, datapipe.NewMemoryCheckpointStore())
    if err != nil {
        log.Fatal(err)
    }

    // 注册进度回调
    pipe.SetProgressCallback(func(info datapipe.ProgressInfo) {
        log.Printf("进度: %d/%d (%.1f%%), 速率: %.0f 条/秒, 已用: %v, 剩余: %v",
            info.Processed, info.Total, info.Percent,
            info.RatePerSec, info.Elapsed.Round(time.Millisecond),
            info.Remaining.Round(time.Second))
    })

    // 启动迁移
    err = pipe.Run(context.Background())
    if err != nil {
        log.Fatalf("迁移失败: %v (状态=%v)", err, pipe.Status())
    }

    log.Printf("迁移完成！共 %d 条记录，%d 个批次，状态=%v",
        pipe.GetProcessed(), pipe.GetBatches(), pipe.Status())
}
```

### 7.2 基于时间戳的增量迁移

```go
cfg := datapipe.Config{
    BatchSize:            50,
    IncrementalMode:      datapipe.IncrementalModeTimestamp,
    IncrementalField:     "updated_at",
    EnableCheckpoint:     true,
    ProgressInterval:     1 * time.Second,
    MaxRetryPerBatch:     5,
    RetryBackoff:         200 * time.Millisecond,
}
pipe, _ := datapipe.NewPipeline(cfg, src, tgt, persistentCheckpointStore)

// 首次运行：全量同步
_ = pipe.Run(ctx)

// 几天后再次运行：仅同步上次运行后新增/变更的数据
pipe2, _ := datapipe.NewPipeline(cfg, src2, tgt2, persistentCheckpointStore)
_ = pipe2.Run(ctx)
```

### 7.3 支持断点续传的持久化存储

```go
type DBCheckpointStore struct {
    db    *sql.DB
    jobID string
}

func (s *DBCheckpointStore) Save(ctx context.Context, cursor *datapipe.Cursor) error {
    valueBytes, _ := json.Marshal(cursor.LastValue)
    _, err := s.db.ExecContext(ctx, `
        INSERT INTO migration_checkpoints (job_id, mode, last_value, last_offset, update_time)
        VALUES (?, ?, ?, ?, ?)
        ON DUPLICATE KEY UPDATE
            mode = VALUES(mode),
            last_value = VALUES(last_value),
            last_offset = VALUES(last_offset),
            update_time = VALUES(update_time)
    `, s.jobID, cursor.Mode, valueBytes, cursor.LastOffset, cursor.UpdateTime)
    return err
}

// Load / Clear 类似实现
```

### 7.4 可取消迁移 + 进度监控

```go
ctx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
defer cancel()

var lastPercent float64
pipe.SetProgressCallback(func(info datapipe.ProgressInfo) {
    if info.Percent-lastPercent >= 10 || info.Processed == info.Total {
        lastPercent = info.Percent
        fmt.Printf("已完成 %.0f%% (%d/%d)\n", info.Percent, info.Processed, info.Total)
    }
})

// 外部条件触发停止
go func() {
    <-shutdownSignal
    fmt.Println("\n收到停止信号，等待当前批次完成...")
    pipe.Stop()
}()

err := pipe.Run(ctx)
switch pipe.Status() {
case datapipe.PipelineStatusCompleted:
    fmt.Println("迁移全部完成")
case datapipe.PipelineStatusStopped:
    fmt.Printf("迁移已暂停，已处理 %d 条，下次继续\n", pipe.GetProcessed())
case datapipe.PipelineStatusFailed:
    fmt.Printf("迁移失败: %v\n", err)
}
```

## 8. 错误处理

### 8.1 配置校验错误（NewPipeline 返回）

| 错误 | 场景 |
|------|------|
| `ErrSourceNil` | source 参数为 nil |
| `ErrTargetNil` | target 参数为 nil |
| `ErrBatchSizeInvalid` | BatchSize <= 0 |
| `ErrIncrementalNoField` | 非全量模式且未设置 IncrementalField |
| `ErrProgressInterval` | ProgressInterval < 0 |

### 8.2 运行时错误（Run 返回）

- `context.Canceled` / `context.DeadlineExceeded`：ctx 被取消或超时
- `fmt.Errorf("datapipe: fetch batch failed: %w", err)`：源端读取失败（含重试耗尽）
- `fmt.Errorf("datapipe: write batch failed: %w", err)`：写入目标失败（含重试耗尽）
- `fmt.Errorf("datapipe: update checkpoint failed: %w", err)`：断点持久化失败
- `fmt.Errorf("datapipe: failed to load checkpoint: %w", err)`：加载断点失败

### 8.3 其他公共错误

| 错误 | 场景 |
|------|------|
| `ErrPipelineStopped` | 写入/读取过程中用户调用 Stop()（内部处理，通常不对外暴露） |
| `ErrInvalidCursor` | CheckpointStore.Save(nil) |
| `ErrPipelineRunning` | Run() 重复调用（并发安全保护） |

## 9. 线程安全说明

Pipeline 所有公共方法均为并发安全：
- 内部通过 `sync.Mutex` 保护状态与 Cursor
- 计数器（processed/total/batches）使用 `sync/atomic` 原子操作
- Stop() 由 `sync.Once` 保证关闭 stopCh 的幂等性
- 并发 `Status() / GetProcessed() / GetTotal() / GetBatches() / GetCursor()` 可与 Run() 同时调用

## 10. 资源与生命周期

- **创建**：`NewPipeline()` 校验配置并创建实例（不启动任何 goroutine）
- **运行**：`Run(ctx)` 启动迁移，阻塞直到完成/失败/取消
  - 内部启动可选的进度上报 goroutine
  - Run() 返回前保证所有内部 goroutine 已退出
- **停止**：
  - `ctx` 取消 → 迁移终止，状态通常为 Failed（ctx.Err 不为 nil）
  - `Stop()` → 等待当前批次完成后安全退出，状态为 Stopped
  - 二者均保证 Source.Close() 与 Target.Close() 被调用
- **多次使用**：Run() 返回后可再次调用（如周期性增量同步），每次 Run 前内部重置 stopCh
