# DistSess 分布式会话存储模块

## 1. 模块概述

DistSess 是一个高性能的分布式会话存储模块，专为需要高可用、可扩展、跨节点同步的会话管理场景设计。模块采用内存+持久化双层存储架构，支持会话自动过期与续期、跨节点数据同步、序列化迁移等核心功能。

**包路径**: `internal/distsess`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 双层存储架构 | 会话数据同时维护在内存和持久化存储，读取优先走内存，未命中回退到持久化并回填 |
| 写透模式 | 写入同时更新内存和持久化两层，保证数据一致性和容灾恢复能力 |
| 会话过期机制 | 每个会话支持独立 TTL 配置，支持全局默认 TTL |
| 自动续期 | 访问未过期会话时自动延长过期时间，可配置开关 |
| 后台清理 | 定时扫描并移除过期会话，从两层存储中同时删除 |
| 跨节点同步 | 任一节点的变更通过消息广播同步到其他节点 |
| 版本冲突检测 | 使用单调递增版本号解决并发更新冲突 |
| 序列化迁移 | 支持单个/全部会话导出为 JSON 格式，支持校验和验证的数据完整性保证 |
| 变更通知 | 支持注册变更处理器，监听会话的创建/更新/删除/续期事件 |

## 3. 核心结构体与职责

### 3.1 Session

会话数据结构体，存储单个会话的完整信息。

```go
type Session struct {
    ID        string
    Data      SessionData
    TTL       time.Duration
    ExpiresAt time.Time
    CreatedAt time.Time
    UpdatedAt time.Time
    Version   uint64
    NodeID    string
}
```

**职责**:
- `ID`: 会话唯一标识符
- `Data`: 会话业务数据，键值对形式
- `TTL`: 会话存活时间，0 表示永不过期
- `ExpiresAt`: 会话过期时间点
- `CreatedAt` / `UpdatedAt`: 创建和更新时间戳
- `Version`: 单调递增版本号，用于并发冲突检测
- `NodeID`: 最后修改该会话的节点 ID

**核心方法**:
- `IsExpired() bool`: 判断会话是否已过期
- `Renew()`: 续期会话，延长过期时间并递增版本号
- `DeepCopy() *Session`: 创建会话深拷贝，避免外部修改影响内部状态

### 3.2 Config

模块配置结构体。

```go
type Config struct {
    NodeID          string
    DefaultTTL      time.Duration
    CleanupInterval time.Duration
    AutoRenew       bool
    PersistenceDir  string
    SyncBuffer      int
    EnableSync      bool
}
```

**配置项说明**:
- `NodeID`: 节点唯一标识，自动生成或手动指定
- `DefaultTTL`: 全局默认会话过期时间，默认 30 分钟
- `CleanupInterval`: 后台过期清理扫描间隔，默认 1 分钟
- `AutoRenew`: 是否启用自动续期，默认开启
- `PersistenceDir`: 文件持久化存储目录
- `SyncBuffer`: 节点间同步消息通道缓冲区大小，默认 1024
- `EnableSync`: 是否启用跨节点同步，默认开启

### 3.3 TieredStore

双层存储管理器，实现内存+持久化的读写逻辑。

```go
type TieredStore struct {
    hitCount       uint64
    missCount      uint64
    expiredCount   uint64
    memoryStore    map[string]*Session
    persistence    PersistenceStore
    mu             sync.RWMutex
    autoRenew      bool
    defaultTTL     time.Duration
    nodeID         string
}
```

**职责**:
- 管理内存层的会话缓存（`memoryStore`）
- 协调与持久化层的交互（`persistence`）
- 实现读写路径的核心逻辑（内存优先、写透模式）
- 维护命中率统计（`hitCount`/`missCount`）
- 处理会话自动续期逻辑
- 实现远程会话合并和删除应用

> **原子字段对齐注意**: `uint64` 类型字段位于结构体开头，保证 32 位系统上原子操作的正确对齐。

### 3.4 PersistenceStore

持久化存储接口，抽象不同的持久化实现。

```go
type PersistenceStore interface {
    Save(session *Session) error
    Load(sessionID string) (*Session, error)
    Delete(sessionID string) error
    LoadAll() ([]*Session, error)
    Count() (int, error)
    Clear() error
    Close() error
}
```

**内置实现**:
- `FilePersistenceStore`: 基于文件系统的 JSON 持久化，使用 SHA-256 哈希作为文件名
- `MemoryPersistenceStore`: 内存持久化实现，主要用于测试

### 3.5 Store

对外 API 接口，单节点模式的会话管理器。

```go
type Store struct {
    cfg             Config
    store           *TieredStore
    persistence     PersistenceStore
    changeHandlers  []ChangeHandler
    handlerMu       sync.RWMutex
    cleanupTicker   *time.Ticker
    cleanupStopCh   chan struct{}
    running         bool
    mu              sync.Mutex
}
```

**职责**:
- 提供用户友好的对外 API（`Get`/`Set`/`Delete`/`Renew` 等）
- 管理后台过期清理循环
- 维护变更处理器列表并分发变更通知
- 提供导出/导入迁移功能

### 3.6 Cluster / Node

集群模式管理器，支持多节点部署。

```go
type Cluster struct {
    cfg          Config
    nodes        map[string]*Node
    nodesMu      sync.RWMutex
    running      bool
    stopCh       chan struct{}
    wg           sync.WaitGroup
    msgDropRate  float64
    msgDropMu    sync.Mutex
}

type Node struct {
    msgSent             uint64
    msgRecv             uint64
    syncedCount         uint64
    rejectCount         uint64
    ID                  string
    store               *TieredStore
    inbox               chan *Message
    cluster             *Cluster
    // ... 其他字段
}
```

**职责**:
- `Cluster`: 管理集群节点生命周期，实现消息广播/单播
- `Node`: 集群中的单个节点，处理同步消息，维护本地存储

### 3.7 ChangeNotification

变更通知结构体，用于跨节点同步和本地变更回调。

```go
type ChangeNotification struct {
    Type       ChangeType
    SessionID  string
    Version    uint64
    NodeID     string
    Timestamp  time.Time
    DataDigest string
    Data       *Session
}
```

**变更类型**:
- `ChangeTypeCreate`: 会话创建
- `ChangeTypeUpdate`: 会话更新
- `ChangeTypeDelete`: 会话删除
- `ChangeTypeRenew`: 会话续期

### 3.8 MigrationData / MigrationResult

序列化迁移相关结构体。

```go
type MigrationData struct {
    Header   MigrationHeader `json:"header"`
    Sessions []*Session      `json:"sessions"`
}

type MigrationHeader struct {
    FormatVersion  int             `json:"format_version"`
    ExportedAt     time.Time       `json:"exported_at"`
    SourceNodeID   string          `json:"source_node_id,omitempty"`
    SessionCount   int             `json:"session_count"`
    Checksum       string          `json:"checksum"`
    Format         MigrationFormat `json:"format"`
}

type MigrationResult struct {
    ImportedCount int
    SkippedCount  int
    FailedCount   int
    Errors        []error
}
```

**职责**:
- `MigrationData`: 迁移数据容器，包含头部元数据和会话列表
- `MigrationHeader`: 迁移元数据，包含版本、校验和、导出时间等
- `MigrationResult`: 导入结果统计，记录成功/跳过/失败数量及错误详情

## 4. 双层存储读写路径

### 4.1 设计原理

双层存储架构结合了内存存储的高性能和持久化存储的可靠性：

1. **内存层**: 提供高速读写访问，使用 `map[string]*Session` 存储
2. **持久化层**: 保证数据容灾恢复，支持文件或自定义实现

### 4.2 读取路径（Get）

```
Get(sessionID) 流程:
  ┌──────────────────────────┐
  │ 1. 内存层查找            │
  │    (加读锁)               │
  └──────────┬───────────────┘
             │
        ┌────┴────┐
        │  命中?  │
        └┬───────┬┘
        是       否
        ▼        ▼
  ┌──────────┐ ┌────────────────────┐
  │ 命中计数 │ │ 未命中计数         │
  │ +1       │ │ +1                 │
  └────┬─────┘ └──────────┬─────────┘
       │                  │
       │          ┌───────▼────────┐
       │          │ 2. 持久化层查找 │
       │          └───────┬────────┘
       │                  │
       │          ┌───────▼────────┐
       │          │   找到?         │
       │          └──┬──────────┬───┘
       │             是          否
       │             ▼           ▼
       │     ┌────────────┐  返回错误
       │     │ 3. 回填内存 │  (ErrSessionNotFound)
       │     └─────┬──────┘
       │           │
       └───────────┘
                  │
          ┌───────▼────────┐
          │ 4. 过期检查     │
          └───┬────────────┘
              │
         ┌────┴─────┐
         │  过期?   │
         └┬────────┬┘
          是        否
          ▼         ▼
  ┌─────────────┐ ┌────────────────┐
  │ 删除两层数据 │ │ 5. 自动续期?   │
  │ 返回错误     │ └───┬────────────┘
  │ (ErrSession│     │ 是          │
  │  Expired)  │     ▼             │
  └─────────────┘ ┌──────────────┐ │
                  │ 续期会话     │ │
                  │ 更新两层存储 │ │
                  └──────┬───────┘ │
                         │         │
                  ┌──────▼─────────▼─┐
                  │ 6. 返回会话拷贝  │
                  └──────────────────┘
```

**关键特性**:
- 内存命中时直接返回，性能最优
- 持久化命中后回填内存，后续读取走内存
- 过期会话被懒删除（访问时发现过期立即删除）
- 自动续期在读取时透明执行，版本号递增

### 4.3 写入路径（SetWithTTL）

```
SetWithTTL(sessionID, data, ttl) 流程:
  ┌──────────────────────────┐
  │ 1. 参数校验              │
  │    (空 ID/空数据返回错误) │
  └──────────┬───────────────┘
             │
          ┌──▼─────────────┐
          │ 2. 创建会话对象 │
          │    设置 TTL     │
          │    版本号初始为 1│
          └───┬─────────────┘
              │
         ┌────▼──────────────┐
         │ 3. 检查是否已存在 │
         │    (加写锁)        │
         └────┬──────────────┘
              │
         ┌────▼──────────────┐
         │ 4. 已存在?        │
         └──┬────────────┬───┘
            是           否
            ▼            ▼
  ┌──────────────────┐ ┌──────────────┐
  │ 版本号 = 现有+1   │ │ 版本号 = 1    │
  │ 保留 CreatedAt    │ └──────┬───────┘
  └───────┬──────────┘        │
          │                   │
          └──────────┬────────┘
                     │
              ┌──────▼────────┐
              │ 5. 更新内存层 │
              └──────┬────────┘
                     │
              ┌──────▼────────┐
              │ 6. 写入持久化 │
              └───┬───────────┘
                  │
             ┌────▼────────────┐
             │ 持久化失败?      │
             └──┬──────────┬───┘
                是          否
                ▼           ▼
       ┌────────────────┐ ┌──────────────┐
       │ 回滚内存层修改  │ │ 返回会话拷贝 │
       │ 返回错误        │ └──────────────┘
       └────────────────┘
```

**关键特性**:
- 写透模式：先写内存，再写持久化
- 原子保证：持久化失败时回滚内存修改
- 版本管理：新建会话版本为 1，更新时递增
- 幂等性：同一 ID 多次写入会递增版本号

### 4.4 删除路径（Delete）

```
Delete(sessionID) 流程:
  ┌──────────────────────────┐
  │ 1. 内存层删除            │
  │    (加写锁)               │
  └──────────┬───────────────┘
             │
          ┌──▼──────────────┐
          │ 2. 记录是否存在 │
          └───┬─────────────┘
              │
          ┌───▼─────────────┐
          │ 3. 持久化层删除 │
          └───┬─────────────┘
              │
          ┌───▼─────────────┐
          │ 4. 返回是否存在 │
          └─────────────────┘
```

**关键特性**:
- 两层存储同时删除，保证一致性
- 返回值表示删除前是否存在
- 空 ID 返回 `ErrEmptySessionID`

## 5. 会话同步一致性模型

### 5.1 最终一致性模型

DistSess 采用**基于版本向量的最终一致性**模型：

1. **版本号机制**: 每个会话维护单调递增的 `Version` 字段
2. **变更广播**: 节点上的变更通过消息广播到其他节点
3. **版本比较**: 接收节点比较消息版本与本地版本，只接受更新版本
4. **冲突解决**: 高版本覆盖低版本，旧版本消息被拒绝

### 5.2 变更通知流程

```
节点 A 发生变更:
  ┌─────────────────────────┐
  │ 1. 本地更新会话数据     │
  │    (版本号 +1)           │
  └──────────┬──────────────┘
             │
  ┌──────────▼──────────────┐
  │ 2. 构建变更通知消息     │
  │    (包含 ID、版本、数据) │
  └──────────┬──────────────┘
             │
  ┌──────────▼──────────────┐
  │ 3. 广播到集群其他节点    │
  └─────────────────────────┘

节点 B 接收消息:
  ┌─────────────────────────┐
  │ 1. 读取本地会话版本     │
  └──────────┬──────────────┘
             │
  ┌──────────▼──────────────┐
  │ 2. 版本比较             │
  │    msg.Version > local? │
  └──────────┬──────────────┘
             │
      ┌──────┴──────┐
      是             否
      ▼              ▼
┌────────────┐ ┌───────────────┐
│ 合并更新   │ │ 拒绝并计数     │
│ 内存+持久化│ │ rejectCount++ │
└─────┬──────┘ └───────────────┘
      │
┌─────▼──────────────┐
│ 触发本地变更回调   │
└────────────────────┘
```

### 5.3 同步消息类型

| 消息类型 | 触发场景 | 处理逻辑 |
|---------|---------|---------|
| `MsgChangeNotify` | 会话创建/更新/续期 | 比较版本，高版本则合并 |
| `MsgInvalidate` | 会话删除 | 比较版本，高版本则删除本地 |
| `MsgSyncRequest` | 节点主动同步请求 | 返回本地全量会话 |
| `MsgSyncResponse` | 同步请求响应 | 逐个合并接收到的会话 |

### 5.4 一致性保证

**不变式保证**:
- 对于任意会话 S 和节点 N：如果 N 收到版本为 V 的变更消息，且 N 本地版本 < V，则 N 必然应用该变更
- 版本号单调递增，不会出现 ABA 问题
- 删除操作使用 `版本号 + 1` 作为无效化版本，确保删除不被旧更新覆盖

**边界情况处理**:
- 消息丢失：节点可通过 `SyncWith(targetNodeID)` 主动发起同步
- 消息乱序：版本号机制保证最终状态正确
- 节点宕机：重启后从持久化层恢复数据，可通过同步追上集群状态

## 6. 数据摘要与完整性校验

### 6.1 Data Digest（数据摘要）

用于变更通知中的数据完整性标识，采用 SHA-256 哈希算法：

```go
func computeDataDigest(data SessionData) string {
    // 按 key 排序确保确定性
    // 对每个 key:value 对计算哈希
    // 返回十六进制编码的哈希值
}
```

**特性**:
- 确定性：相同数据始终生成相同摘要
- 快速比较：通过摘要比较可快速判断数据是否变更
- 防篡改：难以构造不同数据生成相同摘要

### 6.2 Migration Checksum（迁移校验和）

用于序列化迁移的数据完整性保证：

```go
func computeChecksum(sessions []*Session) string {
    // 将会话按 ID 排序
    // 对每个会话计算: ID + 排序后的 key:value + Version + TTL
    // 整体 SHA-256 哈希
}
```

**校验流程**:
1. 导出时计算所有会话的校验和，写入迁移头部
2. 导入时重新计算校验和，与头部比较
3. 不匹配则拒绝导入，返回 `ErrChecksumMismatch`

## 7. 使用示例

### 7.1 单节点基本使用

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/distsess"
)

func main() {
    cfg := distsess.DefaultConfig()
    cfg.PersistenceDir = "./sessions"
    
    store, err := distsess.NewStore(cfg)
    if err != nil {
        panic(err)
    }
    defer store.Close()

    // 创建会话（使用全局默认 TTL）
    session, err := store.Set("user:1", distsess.SessionData{
        "username": "alice",
        "role":     "admin",
    })
    if err != nil {
        panic(err)
    }
    fmt.Println("Created session:", session.ID, "Version:", session.Version)

    // 创建会话（指定独立 TTL）
    session, err = store.Set("user:2", distsess.SessionData{
        "username": "bob",
    }, 1*time.Hour)

    // 读取会话（自动续期如果配置开启）
    session, err = store.Get("user:1")
    if err == distsess.ErrSessionNotFound {
        fmt.Println("Session not found")
    } else if err == distsess.ErrSessionExpired {
        fmt.Println("Session expired")
    } else if err != nil {
        panic(err)
    } else {
        fmt.Println("User:", session.Data["username"])
    }

    // 手动续期
    session, err = store.Renew("user:1")

    // 删除会话
    existed, err := store.Delete("user:2")
    fmt.Println("Deleted existed:", existed)

    // 获取统计信息
    stats := store.Stats()
    fmt.Printf("Memory: %d, Persisted: %d, Hits: %d, Misses: %d\n",
        stats.MemoryCount, stats.PersistedCount, stats.HitCount, stats.MissCount)
}
```

### 7.2 集群模式使用

```go
cfg := distsess.DefaultConfig()
cfg.EnableSync = true

cluster := distsess.NewCluster(cfg)
defer cluster.Stop()

// 添加节点
mps1 := distsess.NewMemoryPersistenceStore()
node1, _ := cluster.AddNode("node-1", mps1)

mps2 := distsess.NewMemoryPersistenceStore()
node2, _ := cluster.AddNode("node-2", mps2)

// 节点 1 创建会话
_, _ = node1.Set("sess:1", distsess.SessionData{"value": "test"}, 1*time.Hour)

// 等待同步
time.Sleep(100 * time.Millisecond)

// 节点 2 可读取到同步的会话
session, err := node2.Get("sess:1")
if err == nil {
    fmt.Println("Synchronized session:", session.Data["value"])
}

// 主动同步
_ = node2.SyncWith("node-1")

// 节点统计
sent, recv, reject := node2.MessageStats()
fmt.Printf("Sent: %d, Received: %d, Rejected: %d\n", sent, recv, reject)
```

### 7.3 变更通知监听

```go
store, _ := distsess.NewStoreWithMemoryPersistence(cfg)

store.AddChangeHandler(func(notification distsess.ChangeNotification) {
    switch notification.Type {
    case distsess.ChangeTypeCreate:
        fmt.Printf("Session created: %s (version %d)\n",
            notification.SessionID, notification.Version)
    case distsess.ChangeTypeUpdate:
        fmt.Printf("Session updated: %s (version %d)\n",
            notification.SessionID, notification.Version)
    case distsess.ChangeTypeDelete:
        fmt.Printf("Session deleted: %s\n", notification.SessionID)
    case distsess.ChangeTypeRenew:
        fmt.Printf("Session renewed: %s (version %d)\n",
            notification.SessionID, notification.Version)
    }
})
```

### 7.4 数据迁移

```go
// 导出所有会话
exported, err := store.ExportAll()
if err != nil {
    panic(err)
}

// 验证迁移数据完整性
if err := distsess.ValidateMigrationData(exported); err != nil {
    panic("Migration data corrupted: " + err.Error())
}

// 保存到文件
_ = os.WriteFile("sessions_backup.json", exported, 0644)

// 在新实例中导入
newStore, _ := distsess.NewStoreWithMemoryPersistence(cfg)
result, err := newStore.ImportAll(exported, true) // overwrite=true
if err != nil {
    panic(err)
}

fmt.Printf("Imported: %d, Skipped: %d, Failed: %d\n",
    result.ImportedCount, result.SkippedCount, result.FailedCount)

if len(result.Errors) > 0 {
    for _, e := range result.Errors {
        fmt.Println("Import error:", e)
    }
}
```

### 7.5 并发安全使用

```go
store, _ := distsess.NewStoreWithMemoryPersistence(cfg)
defer store.Close()

var wg sync.WaitGroup

// 并发写入
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        sessID := fmt.Sprintf("sess:%d", id)
        _, _ = store.Set(sessID, distsess.SessionData{
            "worker": id,
            "time":   time.Now().Unix(),
        }, 30*time.Minute)
    }(i)
}

// 并发读取
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        sessID := fmt.Sprintf("sess:%d", id)
        _, _ = store.Get(sessID)
    }(i)
}

wg.Wait()

stats := store.Stats()
fmt.Printf("Total sessions: %d, Hit rate: %.2f%%\n",
    stats.MemoryCount,
    float64(stats.HitCount)/float64(stats.HitCount+stats.MissCount)*100)
```

## 8. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrSessionNotFound` | 会话不存在 | `Get`/`Load` 未找到会话 |
| `ErrSessionExpired` | 会话已过期 | 访问已过期的会话 |
| `ErrEmptySessionID` | 会话 ID 为空 | 传入空字符串作为会话 ID |
| `ErrNilSessionData` | 会话数据为空 | `Set` 时传入 `nil` 数据 |
| `ErrInvalidConfig` | 配置无效 | 配置参数不合法 |
| `ErrPersistenceFailed` | 持久化操作失败 | 文件读写错误等 |
| `ErrNodeNotFound` | 节点不存在 | 集群操作指定不存在的节点 |
| `ErrClusterStopped` | 集群已停止 | 集群停止后继续操作 |
| `ErrVersionTooOld` | 版本过旧 | 同步消息版本低于本地版本 |
| `ErrMigrationFailed` | 迁移操作失败 | 导入导出过程出错 |
| `ErrChecksumMismatch` | 校验和不匹配 | 迁移数据被篡改或损坏 |
| `ErrSessionExists` | 会话已存在 | 预留错误 |
| `ErrInvalidMessageType` | 无效消息类型 | 收到未知类型的同步消息 |

## 9. 性能特征

### 9.1 时间复杂度

| 操作 | 平均时间 | 最坏时间 | 说明 |
|------|----------|----------|------|
| Get（内存命中） | O(1) | O(1) | 内存 map 查找 |
| Get（内存未命中） | O(1) + 持久化耗时 | O(1) + 持久化耗时 | 持久化 + 内存回填 |
| Set | O(1) + 持久化耗时 | O(1) + 持久化耗时 | 内存写入 + 持久化写入 |
| Delete | O(1) + 持久化耗时 | O(1) + 持久化耗时 | 内存删除 + 持久化删除 |
| Renew | O(1) + 持久化耗时 | O(1) + 持久化耗时 | 同 Set |
| CleanupExpired | O(N) | O(N) | N 为内存中会话总数 |
| ExportAll | O(N) | O(N) | N 为总会话数，含哈希计算 |
| ImportAll | O(N) | O(N) | N 为导入会话数，含校验和验证 |

### 9.2 并发性能

- **内存层**: 使用 `sync.RWMutex`，读-读并发，读-写和写-写互斥
- **持久化层**: 依赖具体实现，文件存储使用独立的 `sync.RWMutex`
- **命中性能**: 内存命中时无持久化开销，性能接近纯内存 map
- **自动续期开销**: 续期会触发持久化写入，高读场景可考虑关闭自动续期

### 9.3 内存占用

每个会话的内存开销：
- 结构体字段：约 100 字节（不含实际数据）
- `SessionData`：取决于存储的键值对数量
- 持久化副本：深拷贝存储，内存占用翻倍

## 10. 注意事项与限制

1. **持久化实现**: 内置文件持久化适用于中小规模场景，大规模部署建议实现数据库版 `PersistenceStore`
2. **最终一致性**: 跨节点同步是异步的，存在短暂的不一致窗口，强一致性场景需额外处理
3. **自动续期开销**: 高频读场景下自动续期会产生大量持久化写入，可根据业务场景权衡
4. **内存容量**: 内存层存储全量会话，会话数量过多时需考虑内存限制或实现淘汰策略
5. **序列化格式**: 迁移数据采用 JSON 格式，二进制敏感数据应额外加密
6. **节点 ID 唯一性**: 集群模式下需确保节点 ID 唯一，否则会导致同步异常
7. **32 位系统对齐**: 结构体中 `uint64` 原子字段已放在开头以保证正确对齐，新增字段时注意保持
8. **消息丢失**: 节点间消息可能丢失，重要数据应通过 `SyncWith` 主动同步
9. **过期清理**: 后台清理周期不宜过短（默认 1 分钟），避免频繁扫描影响性能
10. **版本溢出**: `uint64` 版本号理论上限约 1.8e19，正常使用不会溢出
