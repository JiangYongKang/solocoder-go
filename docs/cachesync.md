# 分布式缓存一致性协议模块需求文档

## 1. 模块概述

`cachesync` 是一个基于内存的分布式缓存一致性协议模块，提供完整的多节点缓存同步解决方案。它实现了基于版本号的缓存更新通知、缓存行排他锁定、写入无效化广播以及定期对账的最终一致性保障，适用于需要多节点缓存数据保持一致的分布式系统场景。

### 主要特性

- **基于版本号的更新通知**：每个缓存条目携带单调递增的版本号，节点更新时广播版本化通知，其他节点对比版本号后选择性更新本地缓存
- **缓存行锁定**：支持对单个缓存条目加排他锁，防止并发修改冲突，锁支持超时自动释放并返回持有者标识用于死锁诊断
- **写入无效化广播**：节点写入或删除缓存条目后，向集群所有其他节点广播无效化消息，收到消息的节点删除本地对应条目
- **最终一致性保障**：通过定期对账机制比对各节点的缓存版本号，发现不一致时以最高版本号为准进行修复，保证数据最终一致
- **模拟消息丢失**：支持配置消息丢弃率，用于模拟网络分区或消息丢失场景下的最终一致性验证
- **并发安全**：所有公共方法均为并发安全，可在多个协程中同时调用

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    LockTimeout       time.Duration
    ReconcileInterval time.Duration
    MessageBuffer     int
}
```

**职责**：Cluster 配置结构体，定义分布式缓存集群的运行参数。

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `LockTimeout` | 缓存锁超时时间，超过该时间未释放的锁将自动失效 | 5秒 |
| `ReconcileInterval` | 对账周期，定期执行一致性对账的时间间隔 | 2秒 |
| `MessageBuffer` | 每个节点消息收件箱的通道缓冲区大小 | 1024 |

### 2.2 CacheEntry

```go
type CacheEntry struct {
    Key       string
    Value     interface{}
    Version   uint64
    UpdatedAt time.Time
    NodeID    string
}
```

**职责**：表示一个缓存条目，封装缓存的键、值、版本号、更新时间和来源节点标识。

| 字段 | 说明 |
|------|------|
| `Key` | 缓存键，唯一标识一个缓存条目 |
| `Value` | 缓存值，可存储任意类型的业务数据 |
| `Version` | 版本号，单调递增，每次写入自动 +1，用于判断新旧 |
| `UpdatedAt` | 最后更新时间戳 |
| `NodeID` | 最后更新该条目的节点 ID |

**重要设计**：
- 版本号采用单调递增策略，每次写入操作都会使版本号 +1
- 接收到版本号更新通知时，只有当消息中的版本号大于本地版本号时才执行更新
- `Get()` 方法返回条目的副本，防止外部修改污染内部缓存状态

### 2.3 MessageType

```go
type MessageType int

const (
    MsgUpdateNotify MessageType = iota
    MsgInvalidate
    MsgReconcileRequest
    MsgReconcileResponse
    MsgLockAcquire
    MsgLockRelease
    MsgLockGranted
    MsgLockDenied
)
```

**职责**：消息类型枚举，表示集群节点间传递的不同协议消息。

| 消息类型 | 说明 |
|----------|------|
| `MsgUpdateNotify` | 版本化更新通知，携带新值和版本号，接收方按版本号决定是否更新 |
| `MsgInvalidate` | 无效化消息，接收方收到后删除本地对应缓存条目 |
| `MsgReconcileRequest` | 对账请求消息，用于主动请求其他节点的版本号清单 |
| `MsgReconcileResponse` | 对账响应消息，携带本地所有缓存条目的版本号 |
| `MsgLockAcquire` | 锁获取请求，向其他节点请求获取某缓存条目的排他锁 |
| `MsgLockRelease` | 锁释放通知，广播告知其他节点某缓存条目的锁已被释放 |
| `MsgLockGranted` | 锁授予响应，同意请求节点获取锁 |
| `MsgLockDenied` | 锁拒绝响应，拒绝请求节点获取锁，并返回当前持有者 |

### 2.4 Message

```go
type Message struct {
    Type       MessageType
    FromNodeID string
    ToNodeID   string
    Key        string
    Value      interface{}
    Version    uint64
    Timestamp  time.Time
    LockHolder string
    LockTTL    time.Duration
    Entries    map[string]uint64
}
```

**职责**：集群节点间传递的协议消息单元，根据消息类型不同，使用不同的字段组合。

| 字段 | 说明 | 适用消息类型 |
|------|------|-------------|
| `Type` | 消息类型，所有消息必选 | 全部 |
| `FromNodeID` | 发送节点 ID | 全部 |
| `ToNodeID` | 目标节点 ID（单播时设置） | 全部 |
| `Key` | 缓存键 | 更新、无效化、锁相关消息 |
| `Value` | 缓存值 | `MsgUpdateNotify` |
| `Version` | 版本号 | `MsgUpdateNotify` |
| `Timestamp` | 消息发送时间戳 | 全部 |
| `LockHolder` | 当前锁持有者 ID | `MsgLockDenied` |
| `LockTTL` | 锁存活时间 | `MsgLockAcquire` |
| `Entries` | 版本号清单（key→version） | `MsgReconcileResponse` |

### 2.5 lockInfo

```go
type lockInfo struct {
    holder     string
    expiresAt  time.Time
    acquiredAt time.Time
}
```

**职责**：表示一个缓存锁的内部状态信息，用于追踪锁的持有者和有效期。

| 字段 | 说明 |
|------|------|
| `holder` | 锁的持有者节点 ID |
| `expiresAt` | 锁过期时间，超过此时间锁自动失效 |
| `acquiredAt` | 锁获取时间，用于诊断和监控 |

**锁超时机制**：
- 每次获取锁时设置 `expiresAt = now + LockTTL`
- 检查锁是否有效时判断 `time.Now().Before(expiresAt)`
- 过期的锁视为自动释放，新的请求可以重新获取

### 2.6 pendingLock

```go
type pendingLock struct {
    grantCh chan bool
    denyCh  chan string
}
```

**职责**：表示一个正在等待锁请求响应的等待者，用于异步接收其他节点的锁授予或拒绝消息。

| 字段 | 说明 |
|------|------|
| `grantCh` | 授予通道，收到其他节点的 `MsgLockGranted` 时向此通道写入信号 |
| `denyCh` | 拒绝通道，收到其他节点的 `MsgLockDenied` 时向此通道写入持有者 ID |

### 2.7 Node

```go
type Node struct {
    ID              string
    cache           map[string]*CacheEntry
    locks           map[string]*lockInfo
    pendingLocks    map[string]*pendingLock
    cacheMu         sync.RWMutex
    lockMu          sync.Mutex
    pendingLockMu   sync.Mutex
    inbox           chan *Message
    cluster         *Cluster
    running         bool
    stopCh          chan struct{}
    wg              sync.WaitGroup
    msgSent         uint64
    msgRecv         uint64
}
```

**职责**：表示集群中的一个缓存节点，负责管理本地缓存、处理协议消息、参与锁协商和对账同步。

核心职责包括：
- 管理本地缓存条目（增删改查、版本号维护）
- 处理收到的协议消息（更新通知、无效化、锁消息等）
- 向其他节点广播更新和无效化消息
- 参与分布式锁的获取、释放和协商
- 提供缓存统计信息和运行状态
- 通过 `messageLoop` 后台协程异步处理消息队列

### 2.8 Cluster

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
```

**职责**：分布式缓存集群管理器，负责创建和管理节点、提供消息广播/单播能力、调度定期对账任务、控制消息丢弃率用于测试。

核心职责包括：
- 管理节点的生命周期（添加、移除、查询）
- 实现节点间的消息广播和单播
- 调度定期对账任务（`StartReconciler`）
- 执行对账算法修复数据不一致
- 支持配置消息丢弃率，模拟网络分区/消息丢失场景

## 3. 缓存一致性协议消息流转路径

### 3.1 版本化更新通知流程

当某个节点需要更新一个缓存条目时，执行以下流程：

```
                              ┌──────────────┐
                              │  节点A Set()  │
                              └──────┬───────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  本地版本号递增+1    │
                          │  更新本地缓存       │
                          └──────────┬─────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  广播 MsgUpdateNotify│
                          │  (携带 Key/Value/    │
                          │   Version/Timestamp) │
                          └──────────┬─────────┘
                         ┌───────────┴