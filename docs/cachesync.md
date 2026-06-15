# 分布式缓存一致性协议模块需求文档

## 1. 模块概述

`cachesync` 是一个基于内存的分布式缓存一致性协议模块，提供完整的多节点缓存同步解决方案。它实现了基于版本号的缓存更新通知、缓存行排他锁定、写入无效化广播以及定期对账的最终一致性保障，适用于需要多节点缓存数据保持一致的分布式系统场景。

### 主要特性

- **基于版本号的更新通知**：每个缓存条目携带单调递增的版本号，节点更新时广播版本化通知，其他节点对比版本号后选择性更新本地缓存
- **版本拒绝可观测性**：当节点接收到版本号过低的更新消息时，通过回调事件和计数器暴露拒绝统计，便于监控和诊断
- **缓存行锁定**：支持对单个缓存条目加排他锁，防止并发修改冲突，锁支持超时自动释放并返回持有者标识用于死锁诊断
- **锁状态自动回滚**：当锁获取被拒绝或超时时，自动向已授予临时锁的节点发送释放消息，避免残留锁状态导致后续请求被错误拒绝
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
- 版本号小于等于本地版本号的消息会触发版本拒绝事件，可通过回调或计数器观测
- `Get()` 方法返回条目的副本，防止外部修改污染内部缓存状态

### 2.3 VersionRejectEvent

```go
type VersionRejectEvent struct {
    Key          string
    LocalVersion uint64
    MsgVersion   uint64
    FromNodeID   string
    RejectedAt   time.Time
}
```

**职责**：版本拒绝事件，封装一次更新消息被拒绝的详细信息，用于可观测性和诊断。

| 字段 | 说明 |
|------|------|
| `Key` | 被拒绝更新的缓存键 |
| `LocalVersion` | 本地缓存的当前版本号 |
| `MsgVersion` | 收到消息中携带的版本号（<= LocalVersion） |
| `FromNodeID` | 发送该更新消息的节点 ID |
| `RejectedAt` | 拒绝发生的时间戳 |

### 2.4 VersionRejectHandler

```go
type VersionRejectHandler func(event VersionRejectEvent)
```

**职责**：版本拒绝事件处理器的函数签名，用户可通过 `AddVersionRejectHandler()` 注册回调，在每次版本拒绝发生时获得通知。

**注意事项**：
- 回调函数在消息处理 goroutine 中同步执行，应避免阻塞操作
- 回调函数内部 panic 会被自动 recover，不会影响消息处理主流程
- 可注册多个处理器，按注册顺序依次调用

### 2.5 MessageType

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
| `MsgLockRelease` | 锁释放通知，广播告知其他节点某缓存条目的锁已被释放，或用于回滚临时授予的锁 |
| `MsgLockGranted` | 锁授予响应，同意请求节点获取锁 |
| `MsgLockDenied` | 锁拒绝响应，拒绝请求节点获取锁，并返回当前持有者 |

### 2.6 Message

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

### 2.7 lockInfo

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

### 2.8 pendingLock

```go
type pendingLockStatus int

const (
    pendingLockStatusPending   pendingLockStatus = iota
    pendingLockStatusSucceeded
    pendingLockStatusFailed
)

type pendingLock struct {
    grantCh    chan bool
    denyCh     chan string
    grantedBy  map[string]struct{}
    grantedMu  sync.Mutex
    status     pendingLockStatus
    statusMu   sync.RWMutex
}
```

**职责**：表示一个正在等待锁请求响应的等待者，用于异步接收其他节点的锁授予或拒绝消息，并记录已授予锁的节点列表供失败回滚使用。包含状态机用于事件驱动的自动回滚。

| 字段 | 说明 |
|------|------|
| `grantCh` | 授予通道，收到其他节点的 `MsgLockGranted` 时向此通道写入信号 |
| `denyCh` | 拒绝通道，收到其他节点的 `MsgLockDenied` 时向此通道写入持有者 ID |
| `grantedBy` | 已返回 `MsgLockGranted` 的节点 ID 集合，用于锁失败时的状态回滚 |
| `grantedMu` | 保护 `grantedBy` 并发访问的互斥锁 |
| `status` | 待处理锁的状态（Pending/Succeeded/Failed），用于事件驱动回滚决策 |
| `statusMu` | 保护 `status` 并发访问的读写锁 |

**状态机说明**：
- `Pending`：初始状态，正在收集授权响应
- `Succeeded`：成功状态，已收集足够授权
- `Failed`：失败状态，已被拒绝或超时，需要回滚

状态转换是单向的：Pending → Succeeded 或 Pending → Failed，状态一旦变更不可回退。

### 2.9 Node

```go
type Node struct {
    ID                  string
    cache               map[string]*CacheEntry
    locks               map[string]*lockInfo
    pendingLocks        map[string]*pendingLock
    rejectHandlers      []VersionRejectHandler
    rejectCount         uint64
    cacheMu             sync.RWMutex
    lockMu              sync.Mutex
    pendingLockMu       sync.Mutex
    rejectHandlerMu     sync.RWMutex
    inbox               chan *Message
    cluster             *Cluster
    running             bool
    stopCh              chan struct{}
    wg                  sync.WaitGroup
    msgSent             uint64
    msgRecv             uint64
}
```

**职责**：表示集群中的一个缓存节点，负责管理本地缓存、处理协议消息、参与锁协商和对账同步。

核心职责包括：
- 管理本地缓存条目（增删改查、版本号维护）
- 处理收到的协议消息（更新通知、无效化、锁消息等）
- 向其他节点广播更新和无效化消息
- 参与分布式锁的获取、释放、协商和状态回滚
- 维护版本拒绝事件处理器和拒绝统计
- 提供缓存统计信息和运行状态
- 通过 `messageLoop` 后台协程异步处理消息队列

### 2.10 Cluster

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
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                   节点B 接收消息         节点C 接收消息
                         │                       │
                         ▼                       ▼
              ┌────────────────────┐  ┌────────────────────┐
              │ 比较本地版本号      │  │ 比较本地版本号      │
              │ 收到的版本 > 本地？ │  │ 收到的版本 > 本地？ │
              └─────────┬──────────┘  └─────────┬──────────┘
                   ┌────┴────┐              ┌────┴────┐
                   │         │              │         │
                   ▼         ▼              ▼         ▼
                  是        否             是         否
                   │         │              │         │
                   ▼         ▼              ▼         ▼
            更新本地     触发拒绝事件    更新本地     触发拒绝事件
            缓存条目     递增rejectCount  缓存条目    递增rejectCount
                         调用handlers
```

**关键规则**：
- 写入节点先更新本地缓存并递增版本号，再广播通知
- 接收节点仅在消息版本号严格大于本地版本号时才更新
- 版本号相等或更小的消息触发拒绝事件（不静默丢弃），可通过 `AddVersionRejectHandler()` 或 `VersionRejectCount()` 观测
- 拒绝事件的触发不影响数据一致性，仅用于监控和诊断

### 3.2 写入无效化广播流程

当使用 `SetWithInvalidate()` 或 `Delete()` 时，采用无效化策略：

```
                          ┌───────────────────────┐
                          │  节点A SetWithInvalidate() │
                          │  或 Delete()            │
                          └──────────┬────────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  更新/删除本地缓存   │
                          └──────────┬─────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  广播 MsgInvalidate  │
                          │  (仅携带 Key)        │
                          └──────────┬─────────┘
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                   节点B 接收消息         节点C 接收消息
                         │                       │
                         ▼                       ▼
                   从本地缓存中             从本地缓存中
                   删除 Key 对应的          删除 Key 对应的
                   条目（如有）             条目（如有）
```

**适用场景**：
- `SetWithInvalidate()`：写入后要求其他节点删除本地副本，下次读取时从数据源或最新节点重新获取
- `Delete()`：删除操作，所有节点都应删除该条目
- 与 `MsgUpdateNotify` 的区别：无效化直接删除，不携带新值，适合大值对象或明确不需要其他节点立即持有新值的场景

### 3.3 分布式锁获取与回滚流程

```
                              ┌──────────────┐
                              │  节点B Lock() │
                              └──────┬───────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │ 检查本地是否已有锁  │
                          └──────────┬─────────┘
                               ┌─────┴─────┐
                               │           │
                               ▼           ▼
                         已被本地持有   无锁或已过期
                         返回成功        │
                                        ▼
                          ┌────────────────────┐
                          │ 注册 pendingLock    │
                          │ 初始化 grantedBy={} │
                          │ 广播 MsgLockAcquire │
                          └──────────┬─────────┘
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                   节点A 接收消息         节点C 接收消息
                         │                       │
                         ▼                       ▼
              ┌────────────────────┐  ┌────────────────────┐
              │ 本地该 Key 有锁？   │  │ 本地该 Key 有锁？   │
              └───────┬────────────┘  └───────┬────────────┘
                  ┌───┴───┐                ┌───┴───┐
                  │       │                │       │
                  ▼       ▼                ▼       ▼
                 有锁    无锁              有锁    无锁
                  │       │                │       │
                  ▼       ▼                ▼       ▼
            MsgLock  记录holder    MsgLock  记录holder
            Denied   返回Granted   Denied   返回Granted
                  │       │                │       │
                  └───┬───┘                └───┬───┘
                      ▼                          ▼
              单播 MsgLockGranted/Denied 至节点B
                                     │
                                     ▼
                          ┌────────────────────┐
                          │ 节点B 收集响应       │
                          │ (Granted→写入grantedBy)│
                          └──────────┬─────────┘
                               ┌─────┴─────┐
                               │           │
                               ▼           ▼
                    全部节点Granted    任意节点Denied/超时
                               │           │
                               ▼           ▼
                         获取锁成功    ┌───────────────┐
                         记录本地状态  │ rollbackLock() │
                                     └───────┬───────┘
                                             │
                                             ▼
                                   遍历 grantedBy 节点列表
                                   向每个节点单播 MsgLockRelease
                                   清理所有临时授予的锁记录
                                             │
                                             ▼
                                   返回错误(含持有者ID)
```

**关键机制**：
- 需要获得**所有其他节点**的同意才能获取锁
- 只要有一个节点拒绝（`MsgLockDenied`），获取立即失败并触发回滚
- 回滚时向所有已授予临时锁的节点单播 `MsgLockRelease`，确保不会有残留的锁状态
- 超时同样触发完整的回滚流程
- 失败时错误信息包含当前锁持有者 ID，用于死锁诊断
- 锁带有 TTL，超时自动释放，防止死锁
- 单节点模式（peerCount=0）直接获取成功

### 3.4 锁释放流程

```
                              ┌──────────────┐
                              │ 节点A Unlock()│
                              └──────┬───────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │ 验证本地锁状态:     │
                          │ - 锁是否存在        │
                          │ - holder 是否为自己 │
                          └──────────┬─────────┘
                               ┌─────┴─────┐
                               │           │
                               ▼           ▼
                         验证失败      验证通过
                      返回错误         本地删除锁记录
                                         │
                                         ▼
                              ┌────────────────────┐
                              │ 广播 MsgLockRelease │
                              └──────────┬─────────┘
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                   节点B 接收消息         节点C 接收消息
                         │                       │
                         ▼                       ▼
              ┌────────────────────┐  ┌────────────────────┐
              │ 本地有该 Key 的锁？  │  │ 本地有该 Key 的锁？  │
              │ holder==FromNodeID?│  │ holder==FromNodeID?│
              └───────┬────────────┘  └───────┬────────────┘
                      │                        │
                      ▼                        ▼
                    是→删除                  是→删除
                    否→忽略                  否→忽略
```

### 3.5 定期对账修复流程

在网络分区或消息丢失导致节点数据不一致时，定期对账机制确保最终一致：

```
                    ┌─────────────────────────────┐
                    │  定时触发 (ReconcileInterval) │
                    └──────────────┬──────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────┐
                    │  收集所有节点的版本号清单     │
                    │  {nodeID: {key: version}}    │
                    └──────────────┬──────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────┐
                    │  合并计算每个 Key 的最高版本  │
                    │  记录最高版本对应的来源节点   │
                    └──────────────┬──────────────┘
                                   │
                                   ▼
                    ┌─────────────────────────────┐
                    │  遍历每个节点的本地版本清单   │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │                             │
                    ▼                             ▼
         本地版本 >= 最高版本              本地缺失或版本较低
                    │                             │
                    ▼                             ▼
              无需处理                    从来源节点获取最新值
                                          调用 handleUpdateNotify()
                                          按正常版本号逻辑更新
                                          如版本仍低于本地则触发拒绝
```

**对账算法要点**：
- 以最高版本号为准，不做冲突合并（Last-Write-Wins 策略）
- 对账过程中直接调用 `handleUpdateNotify`，走正常的版本号判断逻辑和拒绝事件触发
- 对账周期由 `ReconcileInterval` 配置，默认 2 秒
- 可通过 `Cluster.StartReconciler()` 启动自动对账，或调用 `Node.Reconcile()` 手动触发

## 4. 核心算法与策略

### 4.1 版本号管理策略

采用**单调递增版本号**机制，保证可比较性：

```go
func (n *Node) Set(key string, value interface{}) *CacheEntry {
    n.cacheMu.Lock()
    newVersion := uint64(1)
    if existing, ok := n.cache[key]; ok {
        newVersion = existing.Version + 1
    }
    // ... 创建新条目并存储
}
```

**版本号比较规则**：
```
if 收到消息.Version > 本地.Version:
    执行更新
else if 收到消息.Version <= 本地.Version:
    触发 VersionRejectEvent
    递增 rejectCount
    返回包装了 ErrVersionTooOld 的错误
    (数据层面不做修改)
```

仅当消息版本**严格大于**本地版本才会更新，确保：
- 乱序到达的旧消息不会覆盖新数据
- 对账修复时使用相同逻辑，保证一致性
- 并发写入时版本号更高的胜出（Last-Write-Wins）
- 所有拒绝事件都可通过回调或计数器被观测到

### 4.2 分布式锁共识算法与回滚机制

采用**全节点共识**策略（类似于 Two-Phase Locking 的简化版），并包含完善的失败自动回滚机制和多层容错保障。

#### 4.2.1 基本流程

```
获取锁条件：
  - 本地无有效锁
  - 所有其他节点都返回 MsgLockGranted
  - 在超时时间内收集到全部同意

失败条件：
  - 任意节点返回 MsgLockDenied → 立即触发回滚并失败
  - 超时时间内未收集到全部同意 → 触发回滚并超时失败

回滚流程 (rollbackLock):
  1. 加锁读取 pendingLock.grantedBy 中的节点列表
  2. 构造 MsgLockRelease 消息 (FromNodeID=请求者)
  3. 向 grantedBy 中的每个节点可靠单播释放消息
  4. 向所有节点可靠广播释放消息作为兜底
  5. 各节点收到后按正常逻辑验证 holder 并删除临时锁
```

#### 4.2.2 事件驱动自动回滚

通过 pendingLock 状态机实现事件驱动的即时回滚，消除异步监视器与删除操作之间的竞态窗口：

```
状态转换：
  Pending → Succeeded (收集到足够授权)
  Pending → Failed      (被拒绝或超时)

handleLockGranted 三层防御：
  第一层：找不到 pendingLock 时 → 防御性回滚（检查本地是否为持锁者，非持锁者主动回复 Release）
  第二层：加入 grantedBy 前检查状态 → 已 Failed 则立即回滚
  第三层：加入 grantedBy 后二次检查 → 已 Failed 则立即回滚（双重检查锁定模式）
```

#### 4.2.3 释放标记（Tombstone）机制

用于处理消息乱序问题，防止迟到的 Acquire 消息造成残留锁：

```
handleLockRelease:
  - 释放锁后设置释放标记（tombstone）
  - 标记包含时间戳和过期时间（默认 5 秒）
  - 标记有效期内的 Acquire 消息将被忽略

handleLockAcquire:
  - 接收 Acquire 前先检查释放标记
  - 若 Acquire 时间戳 ≤ 释放标记时间戳 → 忽略该 Acquire
  - 定期清理过期的释放标记
```

#### 4.2.4 Lock 自动重试

Lock 方法内置一次快速重试，应对消息延迟等边缘情况：

```
Lock(key, timeout):
  1. 第一次尝试 tryLock()
  2. 若成功则直接返回
  3. 若因锁被持有而失败 → 等待 50ms
  4. 第二次尝试 tryLock()
  5. 返回第二次结果
```

#### 4.2.5 死锁诊断与可靠性

- 锁被持有时，`GetLockHolder(key)` 返回持有者节点 ID
- 锁获取失败时，错误信息包含 `held by {nodeID}`
- 支持锁自动超时释放，防止节点崩溃导致的永久死锁
- 回滚机制避免了"半锁定"状态导致的后续请求被错误拒绝
- 可靠消息传递确保回滚释放消息到达目标节点
- 释放标记机制应对消息乱序导致的残留锁
- 自动重试机制应对低概率的时序竞态

### 4.3 消息传递机制与容错策略

集群内消息传递分为普通模式和可靠模式，针对不同场景选择不同可靠性级别的消息传递方式。

#### 4.3.1 普通消息传递

**广播（Broadcast）**：
```go
func (c *Cluster) broadcast(fromNodeID string, msg *Message)
```
- 向除发送节点外的所有其他节点发送消息
- 非阻塞发送，channel 满则静默丢弃
- 用于：更新通知、无效化消息、锁获取请求

**单播（Unicast）**：
```go
func (c *Cluster) unicast(fromNodeID, toNodeID string, msg *Message) error
```
- 向单个指定节点发送消息
- 非阻塞发送，channel 满则返回 `ErrMessageDropped` 错误
- 用于：锁授予响应、锁拒绝响应、对账请求/响应

#### 4.3.2 可靠消息传递

针对关键消息（如锁释放）提供可靠传递保障：

**可靠单播（Reliable Unicast）**：
```go
func (c *Cluster) reliableUnicast(fromNodeID, toNodeID string, msg *Message) error
```
- 带超时重试的阻塞发送，保障关键消息到达
- 重试 5 次，单次超时 50ms，重试间隔 20ms
- 用于：回滚时向已知授权节点发送释放消息

**可靠广播（Reliable Broadcast）**：
```go
func (c *Cluster) reliableBroadcast(fromNodeID string, msg *Message)
```
- 每个目标节点独立重试的广播机制
- 并发发送，使用 WaitGroup 等待所有节点完成
- 每个节点重试 5 次，单次超时 100ms
- 用于：回滚时的兜底释放消息，覆盖 Grant 消息丢失的节点

#### 4.3.3 消息传递容错策略

不同类型消息采用不同可靠性级别：

| 消息类型 | 传递方式 | 可靠性级别 | 原因 |
|---------|---------|-----------|------|
| 更新通知 | 普通广播 | 低 | 定期对账可兜底，丢消息影响小 |
| 无效化消息 | 普通广播 | 低 | 对账可补齐，最终一致 |
| 锁获取请求 | 普通广播 | 中 | 请求者超时可重试 |
| 锁授予/拒绝 | 普通单播 | 中 | 请求者超时可重试 |
| 锁释放（正常） | 普通广播 | 中 | TTL 自然过期兜底 |
| 锁释放（回滚） | 可靠单播+可靠广播 | 高 | 防止残留锁导致后续请求失败 |

#### 4.3.4 消息丢弃模拟

```go
func (c *Cluster) SetMessageDropRate(rate float64)
```
- `rate` 范围 [0.0, 1.0]，0.0 表示不丢弃，1.0 表示全部丢弃
- 用于测试网络分区或消息丢失场景下的最终一致性
- 基于时间戳取模实现伪随机丢弃，满足测试需求
- 仅影响普通消息传递，可靠消息传递不受影响

### 4.4 对账修复算法

对账是最终一致性的核心保障：

```
步骤：
1. 收集所有节点的 {key → version} 映射
2. 对每个 key，找出所有节点中的最高版本号及对应来源节点
3. 遍历每个节点，对比本地版本号与最高版本号
4. 若本地缺失或版本较低，从来源节点获取最新值并更新
   (调用 handleUpdateNotify，复用版本判断逻辑与拒绝机制)
```

**设计选择**：
- 由 Cluster 集中执行对账，而非节点间 P2P 对账，简化实现
- 对账时直接调用 `handleUpdateNotify`，复用版本号判断逻辑
- 不做删除同步：只补齐缺失或更新过时条目，不删除多余条目（删除由 Delete() 的无效化消息保证）

## 5. API 使用示例

### 5.1 基本使用：两节点缓存同步

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/cachesync"
)

func main() {
    // 1. 创建集群，使用默认配置
    cfg := cachesync.DefaultConfig()
    cluster := cachesync.NewCluster(cfg)
    defer cluster.Stop()

    // 2. 添加节点
    node1, _ := cluster.AddNode("node-1")
    node2, _ := cluster.AddNode("node-2")

    // 3. 节点1写入缓存，会自动广播给其他节点
    entry := node1.Set("user:1001", map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
    })
    fmt.Printf("节点1写入: version=%d\n", entry.Version)

    // 4. 等待消息传播（实际应用中由业务逻辑或对账保证）
    time.Sleep(50 * time.Millisecond)

    // 5. 节点2可以读取到同步过来的数据
    got := node2.Get("user:1001")
    if got != nil {
        fmt.Printf("节点2读取: value=%v, version=%d\n", got.Value, got.Version)
    }

    // 6. 节点1更新，版本号递增
    entry2 := node1.Set("user:1001", map[string]interface{}{
        "name":  "Alice Smith",
        "email": "alice.smith@example.com",
    })
    fmt.Printf("节点1更新: version=%d\n", entry2.Version)

    time.Sleep(50 * time.Millisecond)

    got2 := node2.Get("user:1001")
    if got2 != nil {
        fmt.Printf("节点2读取更新后: version=%d\n", got2.Version)
    }
}
```

### 5.2 版本拒绝可观测性

```go
cfg := cachesync.DefaultConfig()
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")

// 在节点2上注册版本拒绝处理器
var rejectEvent cachesync.VersionRejectEvent
eventReceived := make(chan struct{}, 1)

node2.AddVersionRejectHandler(func(event cachesync.VersionRejectEvent) {
    rejectEvent = event
    select {
    case eventReceived <- struct{}{}:
    default:
    }
})

// 先让节点2有一个较高版本的条目
node2.Set("key:1", "value-v2") // version=1
node2.Set("key:1", "value-v2") // version=2

// 此时 node2 的 version=2

// 节点1写入 version=1 的旧版本并广播
// 注意：直接构造低版本消息需要通过内部机制，正常 Set() 会递增
// 这里为演示效果，先让节点1产生一个本地 version=1，然后通过
// 手动广播或对账来触发拒绝

// 查看拒绝计数器
fmt.Printf("拒绝次数: %d\n", node2.VersionRejectCount())

// 如果发生了拒绝，可以获取事件详情
select {
case <-eventReceived:
    fmt.Printf("被拒绝的 key=%s, 本地版本=%d, 消息版本=%d, 来自节点=%s\n",
        rejectEvent.Key,
        rejectEvent.LocalVersion,
        rejectEvent.MsgVersion,
        rejectEvent.FromNodeID,
    )
case <-time.After(100 * time.Millisecond):
    fmt.Println("未触发拒绝事件")
}

// Stats() 也返回 rejectCount 统计
_, _, rejectCount, _, _ := node2.Stats()
fmt.Printf("Stats 中的拒绝次数: %d\n", rejectCount)
```

### 5.3 缓存行锁定与回滚

```go
cfg := cachesync.DefaultConfig()
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")
node3, _ := cluster.AddNode("node-3") // 3节点集群演示回滚效果

// 节点1先获取锁
holder, err := node1.Lock("resource:42", 10*time.Second)
if err != nil {
    fmt.Printf("节点1获取锁失败: %v\n", err)
    return
}
fmt.Printf("节点1获得锁: holder=%s\n", holder)

// 节点2尝试获取同一把锁：
// 节点3会先授予临时锁，然后收到节点1的拒绝，此时回滚机制
// 会向节点3发送释放消息，清理节点3上的临时锁记录
_, err = node2.Lock("resource:42", 100*time.Millisecond)
if err != nil {
    fmt.Printf("节点2获取锁失败: %v\n", err)
    // 输出包含 "held by node-1"，可用于诊断
}

// 关键：回滚后，节点3上不会残留认为"node2持有锁"的记录
// 验证：释放节点1的锁后，节点2可以正常获取
time.Sleep(50 * time.Millisecond) // 等待回滚消息处理完成

node1.Unlock("resource:42")
time.Sleep(50 * time.Millisecond) // 等待释放广播完成

holder2, err := node2.Lock("resource:42", 10*time.Second)
if err == nil {
    fmt.Printf("节点2现在成功获得锁: holder=%s\n", holder2)
    // 这里不会因为节点3的残留锁状态而失败，证明回滚有效
    node2.Unlock("resource:42")
} else {
    fmt.Printf("节点2仍然失败(不应该发生): %v\n", err)
}
```

### 5.4 写入无效化模式

```go
cfg := cachesync.DefaultConfig()
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")
node3, _ := cluster.AddNode("node-3")

// 使用 SetWithInvalidate：本地保留，其他节点删除
node1.SetWithInvalidate("session:abc", map[string]interface{}{
    "user_id": 1001,
    "expires": time.Now().Add(30 * time.Minute),
})

time.Sleep(50 * time.Millisecond)

// 节点1有值
fmt.Printf("节点1有值: %v\n", node1.Get("session:abc") != nil) // true

// 节点2和3被无效化（删除）
fmt.Printf("节点2有值: %v\n", node2.Get("session:abc") != nil) // false
fmt.Printf("节点3有值: %v\n", node3.Get("session:abc") != nil) // false

// 删除操作同样会触发无效化广播
node1.Delete("session:abc")
time.Sleep(50 * time.Millisecond)
fmt.Printf("节点1删除后有值: %v\n", node1.Get("session:abc") != nil) // false
```

### 5.5 最终一致性：消息丢失后的对账修复

```go
cfg := cachesync.DefaultConfig()
cfg.ReconcileInterval = 50 * time.Millisecond // 缩短对账周期便于演示
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

// 模拟网络分区：100% 丢弃消息
cluster.SetMessageDropRate(1.0)

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")
node3, _ := cluster.AddNode("node-3")

// 三个节点各自写入（消息被丢弃，无法同步）
node1.Set("key-A", "value-from-node1-v1")
node2.Set("key-B", "value-from-node2-v1")
node3.Set("key-A", "value-from-node3-v1") // 同 key，版本号独立

// 此时三个节点数据不一致
time.Sleep(50 * time.Millisecond)
fmt.Printf("node1 有 key-A: %v, key-B: %v\n",
    node1.Get("key-A") != nil, node1.Get("key-B") != nil) // true, false
fmt.Printf("node2 有 key-A: %v, key-B: %v\n",
    node2.Get("key-A") != nil, node2.Get("key-B") != nil) // false, true
fmt.Printf("node3 有 key-A: %v, key-B: %v\n",
    node3.Get("key-A") != nil, node3.Get("key-B") != nil) // true, false

// 恢复网络：停止丢弃消息
cluster.SetMessageDropRate(0.0)

// 启动自动对账
cluster.StartReconciler()

// 等待对账完成（至少1个对账周期）
time.Sleep(200 * time.Millisecond)

// 对账后三个节点数据一致
fmt.Println("=== 对账后 ===")
fmt.Printf("node1: key-A存在=%v, key-B存在=%v\n",
    node1.Get("key-A") != nil, node1.Get("key-B") != nil) // true, true
fmt.Printf("node2: key-A存在=%v, key-B存在=%v\n",
    node2.Get("key-A") != nil, node2.Get("key-B") != nil) // true, true
fmt.Printf("node3: key-A存在=%v, key-B存在=%v\n",
    node3.Get("key-A") != nil, node3.Get("key-B") != nil) // true, true

// key-A 在各节点上的版本号都收敛到最高值
fmt.Printf("node1 key-A version: %d\n", node1.Get("key-A").Version)
fmt.Printf("node2 key-A version: %d\n", node2.Get("key-A").Version)
fmt.Printf("node3 key-A version: %d\n", node3.Get("key-A").Version)
// 以上三者相同，且等于 max(node1.v1, node3.v1) = 1 或更高
```

### 5.6 并发写入场景

```go
cfg := cachesync.DefaultConfig()
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")
node3, _ := cluster.AddNode("node-3")

// 注册拒绝处理器监控拒绝事件
totalRejects := uint64(0)
for _, n := range []*cachesync.Node{node1, node2, node3} {
    n.AddVersionRejectHandler(func(_ cachesync.VersionRejectEvent) {
        // 在生产环境中可以上报监控系统
        atomic.AddUint64(&totalRejects, 1)
    })
}

// 启动自动对账保证最终一致
cluster.StartReconciler()

var wg sync.WaitGroup

// 三个节点并发写入同一个 key
for i := 0; i < 50; i++ {
    wg.Add(3)
    go func(val int) {
        defer wg.Done()
        node1.Set("counter", val)
    }(i)
    go func(val int) {
        defer wg.Done()
        node2.Set("counter", val+1000)
    }(i)
    go func(val int) {
        defer wg.Done()
        node3.Set("counter", val+2000)
    }(i)
}

wg.Wait()
time.Sleep(200 * time.Millisecond) // 等待对账收敛

// 三个节点的版本号最终收敛
v1 := node1.Get("counter")
v2 := node2.Get("counter")
v3 := node3.Get("counter")
fmt.Printf("版本收敛: node1=%d, node2=%d, node3=%d\n",
    v1.Version, v2.Version, v3.Version)
// 输出三者版本号相同（最终一致）

fmt.Printf("期间发生版本拒绝次数: %d\n", atomic.LoadUint64(&totalRejects))
```

### 5.7 锁诊断与监控

```go
cfg := cachesync.DefaultConfig()
cluster := cachesync.NewCluster(cfg)
defer cluster.Stop()

node1, _ := cluster.AddNode("node-1")
node2, _ := cluster.AddNode("node-2")

// 节点1获取锁
node1.Lock("config:global", 5*time.Second)

// 检查锁状态
fmt.Printf("key 是否被锁: %v\n", node1.IsLocked("config:global"))   // true
fmt.Printf("锁持有者是谁: %s\n", node1.GetLockHolder("config:global")) // "node-1"

// 节点2尝试获取并获取诊断信息
_, err := node2.Lock("config:global", 50*time.Millisecond)
if err != nil {
    // 错误信息中包含持有者 ID，可用于死锁检测
    fmt.Printf("获取锁失败: %v\n", err)
    // 输出类似: "cachesync: lock acquisition timed out: held by node-1"
}

// 查看节点统计信息（新增 rejectCount）
sent, recv, rejectCount, cacheSize, lockCount := node1.Stats()
fmt.Printf("节点1统计: sent=%d, recv=%d, rejects=%d, cache=%d, locks=%d\n",
    sent, recv, rejectCount, cacheSize, lockCount)
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrNodeNotFound` | 操作不存在的节点 ID，或传入空字符串作为节点 ID |
| `ErrNodeExists` | 添加已存在的节点 ID |
| `ErrKeyNotFound` | 查询不存在的缓存键（预留） |
| `ErrLockTimeout` | 获取锁超时或被其他节点持有，错误信息包含持有者 ID |
| `ErrLockNotHeld` | 尝试释放未被当前节点持有的锁（包括锁不存在或被其他节点持有） |
| `ErrClusterStopped` | 集群已停止后调用 AddNode 等操作 |
| `ErrInvalidMessageType` | 收到未知类型的协议消息（预留） |
| `ErrVersionTooOld` | `handleUpdateNotify()` 返回的底层错误，当消息版本号 <= 本地版本号时触发，外部可通过 `errors.Is(err, ErrVersionTooOld)` 判断。拒绝事件可通过 `AddVersionRejectHandler()` / `VersionRejectCount()` 观测 |

## 7. 线程安全说明

Cluster 和 Node 的所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：

- 节点缓存使用 `sync.RWMutex` 保护：读操作用读锁，写操作用写锁
- 锁状态使用独立的 `sync.Mutex` 保护，避免与缓存操作相互阻塞
- 等待中的锁请求使用独立的 `sync.Mutex` 保护
- 锁回滚时的 grantedBy 列表使用独立的 `sync.Mutex` 保护
- 版本拒绝处理器列表使用 `sync.RWMutex` 保护，支持多读少写
- 节点列表使用 `sync.RWMutex` 保护
- 消息发送/接收计数、拒绝计数使用 `sync/atomic` 原子操作
- 消息丢弃率使用 `sync.Mutex` 保护
- 后台协程通过 `sync.WaitGroup` 管理生命周期
- 每个节点通过独立的 goroutine (`messageLoop`) 串行处理消息队列，避免竞态
- `VersionRejectHandler` 回调内部 panic 会被 recover，不会影响消息主循环

## 8. 一致性模型说明

### 8.1 一致性级别

本模块实现的是**最终一致性（Eventual Consistency）**，具体特性：

| 特性 | 说明 |
|------|------|
| **顺序一致性** | 不保证。不同节点可能以不同顺序看到更新 |
| **写后读一致性** | 同节点保证；跨节点不保证（需对账或等待消息传播） |
| **因果一致性** | 不保证。仅依赖版本号大小做 LWW 判定 |
| **最终一致性** | 保证。停止写入并经过对账周期后，所有节点数据趋于一致 |

### 8.2 适用场景

**适合**：
- 对一致性要求不严格的分布式缓存
- 读多写少、可以容忍短暂不一致的场景
- 需要分布式锁避免并发修改冲突的场景
- 希望通过版本号避免旧数据覆盖新数据的场景
- 需要对版本拒绝事件进行监控和诊断的生产环境

**不适合**：
- 要求强一致性的场景（如分布式事务、数据库事务）
- 对写入顺序有严格要求的场景
- 节点数非常多（本实现为全节点广播 + 全节点共识锁，节点越多开销越大）

### 8.3 与其他方案对比

| 方案 | 一致性 | 可用性 | 性能 | 复杂度 |
|------|--------|--------|------|--------|
| 本模块（版本号+对账+回滚） | 最终一致 | 高 | 高（广播异步） | 低 |
| 两阶段提交（2PC） | 强一致 | 低 | 低 | 高 |
| Paxos/Raft | 强一致 | 高 | 中 | 很高 |
| 纯广播无对账 | 弱一致 | 高 | 很高 | 很低 |

## 9. 配置调优建议

### 9.1 LockTimeout

- **太短**：正常业务操作可能还没完成锁就过期了，导致并发冲突
- **太长**：节点崩溃或忘记释放锁时，其他节点等待时间过长
- **建议**：设置为业务操作平均耗时的 3-5 倍

### 9.2 ReconcileInterval

- **太短**：对账过于频繁，占用大量 CPU 和网络资源
- **太长**：数据不一致的时间窗口变大，最终一致的收敛时间变长
- **建议**：
  - 对一致性要求较高：500ms - 1s
  - 常规场景：2s - 5s
  - 对一致性要求低：10s 以上

### 9.3 MessageBuffer

- **太小**：消息堆积时容易丢失（当前实现为非阻塞发送，channel 满则丢弃）
- **太大**：内存占用高，节点处理不及时可能导致消息延迟过大
- **建议**：根据预期并发写入量设置，通常 1024-4096 足够

### 9.4 版本拒绝监控建议

建议在生产环境中：
- 通过 `AddVersionRejectHandler()` 注册回调，将拒绝事件上报到监控系统（Prometheus、Grafana 等）
- 设置 `VersionRejectCount()` 的告警阈值，拒绝率异常升高通常意味着网络问题或节点时钟偏移
- 关注 `Stats()` 中的 rejectCount 与 msgRecv 的比值，正常应远小于 1
