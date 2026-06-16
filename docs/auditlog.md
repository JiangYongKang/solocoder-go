# 审计日志系统模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [审计字段标准化](#4-审计字段标准化)
5. [异步写入与重试降级机制](#5-异步写入与重试降级机制)
6. [哈希链防篡改原理](#6-哈希链防篡改原理)
7. [三维查询机制](#7-三维查询机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全](#10-并发安全)
11. [最佳实践](#11-最佳实践)

---

## 1. 模块概述

审计日志系统模块是一个提供操作事件异步记录、标准字段、多维查询和防篡改校验能力的通用审计基础设施。模块设计用于高并发、高可用的业务系统中，确保关键操作可追溯、可验证、不可篡改。

**包路径**: `internal/auditlog`

**设计目标**:
- 提供统一的审计日志写入接口，异步写入不阻塞业务流程
- 审计字段命名与格式规范化，便于检索与分析
- 支持按主体、资源、时间三维度组合查询，结果按时间倒序
- 通过哈希链实现日志完整性校验，检测篡改并定位位置
- 写入失败时自动重试，重试耗尽时触发降级处理
- 完全并发安全，支持高吞吐量场景

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 异步写入 | 日志写入采用 channel + worker 协程模式，不阻塞调用方业务流程 |
| 同步写入 | 提供 `LogSync` 接口，适用于需要确认写入成功的关键场景 |
| 自动重试 | 写入失败后按配置自动重试，重试次数与间隔可参数化 |
| 降级处理 | 重试耗尽后调用降级回调，支持告警、落盘等自定义兜底策略 |
| Panic 恢复 | Writer 或降级处理器 panic 时自动 recover，避免整体崩溃 |
| 标准字段 | 每条日志包含 13+ 标准化字段，格式与命名统一规范 |
| 哈希链防篡改 | 使用 SHA-256 构建哈希链，校验可定位第一条被篡改日志 |
| 索引查询 | 内置主体/资源倒排索引，查询性能 O(1) 定位 + 时间过滤 |
| 时间范围过滤 | 支持按开始时间、结束时间或两者组合筛选 |
| 维度组合 | 主体、资源、时间三种维度可自由组合使用 |
| 倒序排列 | 查询结果默认按时间戳倒序（最新在前） |
| 多 Worker | 支持配置多个写入 Worker，提升高并发吞吐量 |
| 缓冲溢出降级 | channel 满时自动启动新协程写入，不丢日志 |
| 优雅关闭 | `Stop()` 方法确保缓冲中所有日志均被处理后再退出 |

---

## 3. 核心结构体与职责

### 3.1 Logger

审计日志器主结构体，对外提供写入、查询、校验等全部接口。

```go
type Logger struct {
    mu             sync.Mutex
    cfg            Config
    writer         Writer
    degradeHandler DegradeHandler
    logs           []*AuditLog
    lastHash       string
    idCounter      uint64
    buffer         chan *AuditLog
    started        bool
    stopped        bool
    stopCh         chan struct{}
    wg             sync.WaitGroup
    indexBySubject map[string][]*AuditLog
    indexByResource map[string][]*AuditLog
}
```

**职责**:
- 管理生命周期（Start / Stop），协调异步写入协程
- 构造标准化 AuditLog 记录，计算哈希链
- 维护按主体、按资源的倒排索引
- 调度缓冲队列，执行写入重试与降级流程
- 提供按维度查询和完整性校验接口
- 通过 `sync.Mutex` 保护内部状态的并发安全

### 3.2 AuditLog

审计日志标准化记录结构体，定义了全部规范字段。

```go
type AuditLog struct {
    EventID       string
    Timestamp     time.Time
    SubjectID     string
    Operation     OperationType
    OperationDesc string
    ResourceID    string
    ResourceType  string
    Result        OperationResult
    SourceIP      string
    UserAgent     string
    Detail        string
    PreviousHash  string
    CurrentHash   string
}
```

**职责**:
- 承载单条审计事件的全部语义信息
- 作为哈希链的节点，包含前向与当前哈希
- 通过 Writer 接口持久化到目标存储

### 3.3 Entry

调用方传入的审计日志条目结构体，包含业务语义字段。

```go
type Entry struct {
    SubjectID    string
    Operation    OperationType
    ResourceID   string
    ResourceType string
    Result       OperationResult
    SourceIP     string
    UserAgent    string
    Detail       string
}
```

**职责**:
- 定义业务方调用写入接口时的入参规范
- 屏蔽系统字段（EventID、Timestamp、哈希等），降低调用方复杂度

### 3.4 Config

审计日志器配置结构体，用于定制行为参数。

```go
type Config struct {
    MaxRetries      int
    RetryInterval   time.Duration
    BufferSize      int
    WorkerCount     int
    EnableHashChain bool
}
```

**默认配置** (`DefaultConfig()`):
- `MaxRetries`: 3 次
- `RetryInterval`: 50 毫秒
- `BufferSize`: 1024 条
- `WorkerCount`: 1 个
- `EnableHashChain`: true

**职责**:
- 参数化写入重试策略、缓冲大小、并发度、哈希链开关
- 在 `NewLoggerWithConfig` 中进行合法性校验

### 3.5 Query

查询条件结构体，支持主体、资源、时间三维度。

```go
type Query struct {
    SubjectID  string
    ResourceID string
    StartTime  *time.Time
    EndTime    *time.Time
}
```

**职责**:
- 统一查询参数结构，三种维度可任意组合
- `nil` 时间字段表示不设边界，空字符串表示不筛选该维度

### 3.6 VerificationResult

完整性校验结果结构体，返回校验状态与篡改位置。

```go
type VerificationResult struct {
    Valid         bool
    TamperedIndex int
    Message       string
}
```

**职责**:
- `Valid`: 全部日志完整无篡改时为 `true`
- `TamperedIndex`: 发现问题的首条日志索引（-1 表示无问题）
- `Message`: 人类可读的校验详情

### 3.7 Writer 接口

日志持久化写入器抽象接口，支持自定义实现。

```go
type Writer interface {
    Write(log *AuditLog) error
}
```

**职责**:
- 定义审计日志持久化层契约
- 默认提供基于内存的 `MemoryWriter` 实现（测试与演示用）
- 业务方可自行实现数据库、文件、消息队列等 Writer

---

## 4. 审计字段标准化

### 4.1 字段一览表

| 字段名 | 类型 | 必填 | 说明 | 示例值 |
|--------|------|------|------|--------|
| EventID | string | 是 | 事件唯一标识，格式 `audit-{纳秒时间戳}-{自增序号}` | `audit-1718000000000-1` |
| Timestamp | time.Time | 是 | 事件发生时间（UTC+本地时区） | 系统自动生成 |
| SubjectID | string | 推荐 | 操作主体标识（用户ID、服务名等） | `"user:10086"` |
| Operation | OperationType | 是 | 操作类型枚举 | `OpCreate` |
| OperationDesc | string | 是 | 操作类型文本描述（自动生成） | `"CREATE"` |
| ResourceID | string | 推荐 | 目标资源标识 | `"order:20240101"` |
| ResourceType | string | 推荐 | 目标资源类型 | `"order"` |
| Result | OperationResult | 是 | 操作结果枚举 | `ResultSuccess` |
| SourceIP | string | 推荐 | 请求来源 IP 地址 | `"192.168.1.100"` |
| UserAgent | string | 可选 | 请求 User-Agent | `"Mozilla/5.0 ..."` |
| Detail | string | 可选 | 操作详情 JSON 或描述文本 | `"{\"qty\":2}"` |
| PreviousHash | string | 是 | 前一条日志哈希值（首条为空串） | SHA-256 hex 或 `""` |
| CurrentHash | string | 是 | 当前日志内容哈希值 | SHA-256 hex |

### 4.2 OperationType 枚举

| 常量 | 值 | 字符串 | 说明 |
|------|----|--------|------|
| OpCreate | 0 | CREATE | 新增操作 |
| OpRead | 1 | READ | 查询操作 |
| OpUpdate | 2 | UPDATE | 修改操作 |
| OpDelete | 3 | DELETE | 删除操作 |
| OpLogin | 4 | LOGIN | 登录认证 |
| OpLogout | 5 | LOGOUT | 登出操作 |
| OpCustom | 6 | CUSTOM | 自定义操作 |

### 4.3 OperationResult 枚举

| 常量 | 值 | 字符串 | 说明 |
|------|----|--------|------|
| ResultSuccess | 0 | SUCCESS | 操作成功 |
| ResultFailure | 1 | FAILURE | 操作失败 |

---

## 5. 异步写入与重试降级机制

### 5.1 整体写入流程

```
调用方 Log(entry)
    │
    ├─→ 步骤 1：参数检查
    │       ├─ entry 为 nil → 返回错误
    │       └─ Logger 未启动/已停止 → 返回 ErrLoggerStopped
    │
    ├─→ 步骤 2：构造 AuditLog（createLog，互斥锁保护）
    │       ├─ 生成 EventID（纳秒时间戳 + 原子自增）
    │       ├─ 设置 Timestamp = time.Now()
    │       ├─ 自动补 OperationDesc = Operation.String()
    │       ├─ 设置 PreviousHash = 上一条 CurrentHash
    │       ├─ 启用哈希链时计算 CurrentHash = SHA-256(全部字段)
    │       ├─ 更新 lastHash = CurrentHash
    │       ├─ 追加到 logs 切片
    │       └─ 更新 subject/resource 倒排索引
    │
    └─→ 步骤 3：异步投递（非阻塞）
            │
            ├─ 主路径：select 写入 buffer channel
            │       └─ 成功 → 返回 nil，由 Worker 消费
            │
            └─ 缓冲满降级：启动独立 goroutine
                    └─ 立即调用 persistWithRetry，不阻塞调用方
```

### 5.2 Worker 消费循环

```
workerLoop()
    │
    ├─ select {
    │   case <-stopCh: → 退出
    │   case log <-buffer: → 调用 persistWithRetry(log)
    │ }
    └─ 循环
```

`Stop()` 执行流程：
1. 设置 `stopped = true`，关闭 `stopCh` 通知 Worker 退出
2. `wg.Wait()` 等待所有 Worker 正常退出
3. 关闭 `buffer` channel
4. 遍历 channel 中剩余日志，逐条 `persistWithRetry`（不丢失任何一条）

### 5.3 重试机制 (persistWithRetry)

```
persistWithRetry(log)
    │
    ├─ 循环 attempt = 0 .. MaxRetries
    │       ├─ attempt > 0 → sleep RetryInterval
    │       ├─ 调用 writer.Write (defer recover 捕获 panic)
    │       ├─ 成功 → return
    │       └─ 失败 → 记录 lastErr，继续下一轮
    │
    └─ 全部重试失败
            ├─ 获取 degradeHandler
            └─ 非 nil → recover 保护下调用 handler(entry, ErrWriteFailed)
```

**关键点**:
- `writer.Write` 使用闭包 + `defer recover` 确保 Writer panic 不崩溃
- 降级回调本身也使用 `defer recover`，双重保护
- 降级回调收到的 `Entry` 是从 `AuditLog` 重建的业务语义副本

### 5.4 缓冲满降级策略

当 `buffer` channel 已满（高并发瞬时峰值）时，`Log()` 不阻塞等待，而是启动独立 goroutine 执行 `persistWithRetry(log)`。特点：
- 调用方始终非阻塞，业务延迟不受影响
- 极端情况下会有额外 goroutine 开销，但不会丢日志
- 独立 goroutine 与 Worker 走同样的重试+降级流程

---

## 6. 哈希链防篡改原理

### 6.1 哈希链结构

审计日志采用类似区块链的哈希链机制，每一条日志都包含前一条的哈希，形成不可分割的链条：

```
Log 0 (Genesis)                    Log 1                         Log 2
┌──────────────────────┐       ┌──────────────────────┐     ┌──────────────────────┐
│ EventID: audit-...-1 │       │ EventID: audit-...-2 │     │ EventID: audit-...-3 │
│ PreviousHash: ""     │◄──────│ PreviousHash: H0     │◄────│ PreviousHash: H1     │
│                      │       │                      │     │                      │
│ CurrentHash: H0 =    │       │ CurrentHash: H1 =    │     │ CurrentHash: H2 =    │
│ SHA256(字段+""  )    │       │ SHA256(字段+H0  )    │     │ SHA256(字段+H1  )    │
└──────────────────────┘       └──────────────────────┘     └──────────────────────┘
```

### 6.2 哈希计算方法 (computeHash)

```go
h := sha256.New()
h.Write([]byte(fmt.Sprintf("%s|%d|%s|%s|%s|%s|%s|%s|%s|%s|%s|%s",
    EventID,
    Timestamp.UnixNano(),
    SubjectID,
    Operation.String(),
    OperationDesc,
    ResourceID,
    ResourceType,
    Result.String(),
    SourceIP,
    UserAgent,
    Detail,
    PreviousHash,
)))
CurrentHash = hex.EncodeToString(h.Sum(nil))
```

**算法**: SHA-256
**输入**: 12 个字段以 `|` 分隔拼接（顺序严格固定）
**输出**: 64 字符小写十六进制字符串

### 6.3 完整性校验流程 (VerifyIntegrity)

```
VerifyIntegrity()
    │
    ├─ 日志为空 → Valid=true, Message="no logs"
    │
    ├─ 第 0 条 PreviousHash != "" → 失败 index=0
    │
    └─ 遍历 i = 0 .. N-1
            │
            ├─ 检查 HASH 自洽性
            │   expected = computeHash(logs[i])
            │   IF logs[i].CurrentHash != expected
            │       → Valid=false, TamperedIndex=i
            │       → Message="log at index i has been tampered: hash mismatch"
            │
            └─ 检查链连续性（i>0）
                IF logs[i].PreviousHash != logs[i-1].CurrentHash
                    → Valid=false, TamperedIndex=i
                    → Message="hash chain broken at index i"
```

**篡改检测能力**:

| 篡改类型 | 能否检测 | 定位方式 |
|---------|---------|---------|
| 修改某条日志的 Detail/Subject/Result 等字段 | ✅ | 该条日志 CurrentHash 与重计算值不匹配 |
| 修改某条日志的 Timestamp | ✅ | 同上（Timestamp 参与哈希） |
| 删除中间某条日志 | ✅ | 下一条的 PreviousHash 与新前一条不匹配 |
| 插入一条日志到中间 | ✅ | 链连续性被破坏 |
| 修改多条日志 | ✅ | 返回**第一条**被篡改的索引位置 |
| 修改第一条日志 PreviousHash | ✅ | 第 0 条 PreviousHash 应为空 |
| 修改某条后重新计算该条哈希 | ✅ | 下一条 PreviousHash 对不上 |
| 修改某条后重新计算该条及后续所有哈希 | ❌* | 需配合离线签名/公证等额外机制 |

> *注：纯粹的哈希链无法抵御"篡改+重算全部后续哈希"的攻击。对安全等级高的场景，应定期将最后一条哈希锚定到区块链或第三方存证服务。

---

## 7. 三维查询机制

### 7.1 查询维度矩阵

| SubjectID | ResourceID | 时间范围 | 效果 |
|-----------|-----------|---------|------|
| 空 | 空 | 空 | 返回全部日志 |
| 有值 | 空 | 空 | 该主体的所有操作（按时间倒序） |
| 空 | 有值 | 空 | 对该资源的所有操作（谁动过这个资源） |
| 有值 | 有值 | 空 | 该主体对该资源的所有操作（交集） |
| 任意 | 任意 | StartTime 非空 | 过滤 Timestamp >= StartTime |
| 任意 | 任意 | EndTime 非空 | 过滤 Timestamp <= EndTime |
| 任意 | 任意 | 两者非空 | 过滤 StartTime <= Timestamp <= EndTime |

### 7.2 索引结构与查询算法

```
内部存储：
  logs:               []*AuditLog          // 写入顺序切片
  indexBySubject:     map[SubjectID][]*AuditLog  // 主体倒排索引
  indexByResource:    map[ResourceID][]*AuditLog // 资源倒排索引

查询算法（Query函数）：
  1. 根据 SubjectID/ResourceID 选择候选集：
     - 双维度：先取主体候选 → 用资源 EventID Set 过滤交集
     - 单维度：直接取索引切片
     - 无维度：遍历全部 logs
  2. 时间范围过滤（StartTime/EndTime）
  3. 返回结果排序：sort.Slice 按 Timestamp 降序
```

### 7.3 查询结果排序

所有查询结果均按 `Timestamp` 从新到旧（**降序**）排列：
- `results[0]`: 最新的一条日志
- `results[n-1]`: 最早的一条日志

---

## 8. 使用示例

### 8.1 基本使用（异步写入）

```go
package main

import (
    "fmt"
    "solocoder-go/internal/auditlog"
)

func main() {
    writer := auditlog.NewMemoryWriter()

    logger, err := auditlog.NewLogger(writer)
    if err != nil {
        panic(err)
    }
    defer logger.Stop()

    if err := logger.Start(); err != nil {
        panic(err)
    }

    err = logger.Log(&auditlog.Entry{
        SubjectID:    "user:10086",
        Operation:    auditlog.OpCreate,
        ResourceID:   "order:20240101",
        ResourceType: "order",
        Result:       auditlog.ResultSuccess,
        SourceIP:     "192.168.1.100",
        UserAgent:    "Mozilla/5.0",
        Detail:       `{"amount": 99.9}`,
    })
    if err != nil {
        fmt.Println("写入失败:", err)
    }

    fmt.Println("日志总数:", logger.Count())
}
```

### 8.2 自定义配置 + 降级处理

```go
package main

import (
    "fmt"
    "log"
    "time"
    "solocoder-go/internal/auditlog"
)

type DummyWriter struct{}

func (d *DummyWriter) Write(l *auditlog.AuditLog) error {
    // 模拟有时会失败的数据库写入
    if time.Now().Unix()%2 == 0 {
        return fmt.Errorf("db down")
    }
    return nil
}

func main() {
    cfg := auditlog.Config{
        MaxRetries:      5,
        RetryInterval:   200 * time.Millisecond,
        BufferSize:      2048,
        WorkerCount:     4,
        EnableHashChain: true,
    }

    logger, err := auditlog.NewLoggerWithConfig(&DummyWriter{}, cfg)
    if err != nil {
        panic(err)
    }

    logger.SetDegradeHandler(func(entry *auditlog.Entry, err error) {
        // 降级策略：打印告警 + 写入本地文件
        log.Printf("[ALERT] 审计日志写入失败: subject=%s, res=%s, err=%v",
            entry.SubjectID, entry.ResourceID, err)
        // appendToFallbackFile(entry) ...
    })

    if err := logger.Start(); err != nil {
        panic(err)
    }
    defer logger.Stop()

    for i := 0; i < 1000; i++ {
        logger.Log(&auditlog.Entry{
            SubjectID: fmt.Sprintf("user:%d", i),
            Operation: auditlog.OpUpdate,
            ResourceID: fmt.Sprintf("item:%d", i),
            Result: auditlog.ResultSuccess,
            SourceIP: "10.0.0.1",
        })
    }
}
```

### 8.3 按主体查询 + 时间过滤

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/auditlog"
)

func main() {
    writer := auditlog.NewMemoryWriter()
    logger, _ := auditlog.NewLogger(writer)
    logger.Start()
    defer logger.Stop()

    // 写入一批日志...

    // 查询用户 user:alice 最近 24 小时的所有操作
    yesterday := time.Now().Add(-24 * time.Hour)
    now := time.Now()

    results := logger.Query(auditlog.Query{
        SubjectID: "user:alice",
        StartTime: &yesterday,
        EndTime:   &now,
    })

    fmt.Printf("alice 最近 24 小时操作 %d 次:\n", len(results))
    for i, log := range results {
        fmt.Printf("  [%d] %s | %s | %s | %s | %s\n",
            i,
            log.Timestamp.Format("2006-01-02 15:04:05"),
            log.OperationDesc,
            log.ResourceID,
            log.Result.String(),
            log.SourceIP,
        )
    }
}
```

### 8.4 查询谁操作过某个资源

```go
// 查询所有对文档 doc:secret-001 的操作
results := logger.Query(auditlog.Query{ResourceID: "doc:secret-001"})

for _, log := range results {
    fmt.Printf("%s 由 %s 执行 %s，结果 %s (IP: %s)\n",
        log.Timestamp.Format(time.RFC3339),
        log.SubjectID,
        log.OperationDesc,
        log.Result.String(),
        log.SourceIP,
    )
}
```

### 8.5 完整性校验

```go
package main

import (
    "fmt"
    "solocoder-go/internal/auditlog"
)

func main() {
    writer := auditlog.NewMemoryWriter()
    logger, _ := auditlog.NewLogger(writer)
    logger.Start()

    for i := 0; i < 100; i++ {
        logger.LogSync(&auditlog.Entry{
            SubjectID: fmt.Sprintf("u%d", i),
            Operation: auditlog.OpCreate,
            ResourceID: fmt.Sprintf("r%d", i),
            Result: auditlog.ResultSuccess,
        })
    }
    logger.Stop()

    result := logger.VerifyIntegrity()
    if result.Valid {
        fmt.Println("✅ 审计日志完整:", result.Message)
    } else {
        fmt.Printf("❌ 审计日志被篡改！第 %d 条出问题\n", result.TamperedIndex)
        fmt.Println("详情:", result.Message)
    }
}
```

### 8.6 登录登出审计

```go
func auditLogin(userID, ip, ua string, success bool) {
    result := auditlog.ResultSuccess
    if !success {
        result = auditlog.ResultFailure
    }
    logger.Log(&auditlog.Entry{
        SubjectID:    userID,
        Operation:    auditlog.OpLogin,
        ResourceType: "session",
        Result:       result,
        SourceIP:     ip,
        UserAgent:    ua,
    })
}

func auditLogout(userID, ip string) {
    logger.Log(&auditlog.Entry{
        SubjectID:    userID,
        Operation:    auditlog.OpLogout,
        ResourceType: "session",
        Result:       auditlog.ResultSuccess,
        SourceIP:     ip,
    })
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrLoggerStopped` | 日志器已停止 | 调用 Log/LogSync 时 Logger 未 Start 或已 Stop |
| `ErrLoggerAlreadyStarted` | 日志器已启动 | 重复调用 Start() |
| `ErrWriteFailed` | 写入重试后仍失败 | 重试耗尽后通过降级回调返回 |
| `ErrInvalidConfig` | 配置参数非法 | Config 中字段值不符合约束 |
| `ErrLogNotFound` | 日志不存在 | GetByEventID 找不到指定事件ID |
| `ErrHashChainBroken` | 哈希链断裂 | 保留错误变量，供外部引用 |
| `ErrNilWriter` | Writer 为空 | NewLogger/NewLoggerWithConfig 传入 nil Writer |

---

## 10. 并发安全

模块通过分层同步策略实现完全并发安全：

| 数据结构 | 同步机制 | 说明 |
|---------|---------|------|
| Logger 内部状态 (logs/索引/hash/started) | `sync.Mutex` | 互斥锁保护 createLog/Stop/查询/校验 |
| Writer 调用 | 闭包 defer recover | 防止底层 panic 扩散 |
| buffer channel | Go channel 内建同步 | 多生产者多消费者安全 |
| stopCh / wg | `sync.WaitGroup` + `chan struct{}` | 优雅关闭生命周期 |
| MemoryWriter 内部 | `sync.RWMutex` | 读写锁保护内存存储 |
| ID 计数器 | 互斥锁内自增 | 配合时间戳保证全局唯一 |

**并发能力验证**（单元测试覆盖）：
- 10 协程 × 100 条异步写入 = 1000 条，哈希链校验通过
- 10 协程 × 50 条同步写入 = 500 条，全部返回成功
- 4 Worker 并发消费 100 条写入，哈希链完整

---

## 11. 最佳实践

### 11.1 使用建议

1. **优先使用异步写入**：绝大多数场景用 `Log()`，只有当必须确认写入成功时才用 `LogSync()`
2. **合理设置缓冲**：`BufferSize` 建议为峰值 QPS 的 2~5 倍，避免频繁触发降级 goroutine
3. **多 Worker 配置**：高并发场景可提高 `WorkerCount`（如 2~8 个），线性提升吞吐量
4. **务必设置降级回调**：通过 `SetDegradeHandler` 注册告警/落盘逻辑，极端情况下不丢失审计信息
5. **优雅关闭**：进程退出前调用 `logger.Stop()`，确保缓冲中所有日志被处理
6. **定期完整性校验**：每日/每小时定时调用 `VerifyIntegrity()`，发现异常及时告警
7. **主体与资源 ID 规范化**：建议使用 `{type}:{id}` 格式，如 `user:10086`，`order:20240101`

### 11.2 生产环境部署建议

```
┌────────────┐   async   ┌──────────────┐   batch   ┌──────────────┐
│ 业务服务   │ ────────► │ auditlog     │ ────────► │ 数据库/ES    │
│ (调用Log)  │           │ Logger       │           │ (Writer实现) │
└────────────┘           └──────┬───────┘           └──────────────┘
                                │
                                ▼
                     ┌──────────────────────┐
                     │ Degrade Handler      │
                     │  → 告警 (AlertManager)│
                     │  → 本地文件落盘      │
                     │  → 死信队列          │
                     └──────────────────────┘
```

### 11.3 安全加固

为抵御"重算后续哈希"攻击，建议叠加以下措施：

1. **定期存证**：每小时取最后一条日志的 `CurrentHash`，写入区块链或第三方时间戳服务
2. **只读副本**：审计日志写入后同步到只读从库/对象存储，校验时多副本比对
3. **权限隔离**：审计日志写入账号与查询账号分离，禁止 UPDATE/DELETE 权限
4. **加密传输**：Writer 实现中对敏感字段（IP、Detail）加密后存储
