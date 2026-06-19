# RaftLog 共识日志模块

## 1. 模块概述

RaftLog 是一个基于 Raft 共识算法的分布式日志模块，专为需要强一致性、高可用的分布式系统设计。模块实现了 Raft 算法的三大核心子问题：Leader 选举、日志复制、成员变更，并提供了快照安装机制用于日志压缩。模块使用内存传输层模拟节点间网络通信，便于测试和集成。

**包路径**: `internal/raftlog`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Leader 选举 | 节点启动后自动选举 Leader，超时后重新选举，同一任期每节点仅投一票 |
| 日志复制 | Leader 接收客户端请求并复制到所有 Followers，多数确认后提交 |
| 日志提交 | 已提交日志应用到状态机，Leader 推进提交索引并通知 Followers |
| 快照安装 | Follower 日志落后过多时，Leader 发送快照加速追赶 |
| 日志压缩 | 定期压缩已提交日志，减少内存占用 |
| 成员变更 | 支持动态添加/移除节点，采用两阶段联合共识保证平滑过渡 |
| 内存传输 | 基于内存的网络传输模拟，支持延迟注入，便于测试 |
| 内存状态机 | 内置简易键值状态机实现，支持快照导出/恢复 |

## 3. 核心结构体与职责

### 3.1 RaftNode

Raft 节点主结构体，实现了完整的 Raft 协议状态机。

```go
type RaftNode struct {
    id          string
    cfg         RaftConfig
    state       NodeState
    sm          StateMachine
    transport   Transport
    // ... 内部状态字段
}
```

**职责**:
- 维护节点状态（Follower/Candidate/Leader）及转换逻辑
- 处理 RequestVote、AppendEntries、InstallSnapshot 三类 RPC
- 管理本地日志、提交索引、已应用索引
- 作为 Leader 时负责日志复制和心跳发送
- 作为 Follower 时负责响应 Leader 请求和投票
- 管理选举超时和心跳计时器
- 处理成员变更的两阶段联合共识

### 3.2 Cluster

集群管理器，用于创建和管理多个 Raft 节点，方便测试和使用。

```go
type Cluster struct {
    mu        sync.RWMutex
    nodes     map[string]*RaftNode
    transport *MemoryTransport
}
```

**职责**:
- 创建多节点 Raft 集群
- 管理节点的启动和停止
- 提供 Leader 查询、等待 Leader 选举等辅助方法
- 支持动态添加/移除节点

### 3.3 LogEntry

日志条目结构体，表示 Raft 日志中的一条记录。

```go
type LogEntry struct {
    Term    int
    Index   int
    Type    LogEntryType
    Command []byte
}
```

**字段说明**:
- `Term`: 日志条目所属的任期号
- `Index`: 日志条目的索引（全局单调递增）
- `Type`: 日志类型（普通日志、联合配置、新配置）
- `Command`: 状态机命令数据

### 3.4 Configuration

集群配置结构体，表示当前集群中的节点集合。

```go
type Configuration struct {
    Nodes map[string]bool
}
```

**职责**:
- 存储集群中的节点 ID 集合
- 提供多数派计算（Quorum）
- 提供节点存在性检查、克隆等辅助方法

### 3.5 JointConfiguration

联合配置结构体，用于成员变更的两阶段提交。

```go
type JointConfiguration struct {
    Old *Configuration
    New *Configuration
}
```

**职责**:
- 同时保存旧配置和新配置
- 联合共识期间，决策需要同时获得旧配置和新配置的多数派同意

### 3.6 Snapshot

快照结构体，包含某一时刻状态机的完整状态。

```go
type Snapshot struct {
    LastIncludedIndex int
    LastIncludedTerm  int
    Config            *Configuration
    Data              []byte
}
```

**职责**:
- 保存快照对应的最后日志索引和任期
- 保存快照时刻的集群配置
- 保存状态机的序列化数据

### 3.7 MemoryTransport

内存传输层，模拟节点间的网络通信。

```go
type MemoryTransport struct {
    mu    sync.RWMutex
    nodes map[string]*RaftNode
    delay time.Duration
}
```

**职责**:
- 注册和注销节点
- 实现三类 RPC 的同步调用
- 支持注入网络延迟，模拟真实网络环境
- 提供节点间的直接消息传递

### 3.8 MemoryStateMachine

内存状态机实现，基于键值对的简易状态机。

```go
type MemoryStateMachine struct {
    mu   sync.RWMutex
    data map[string]string
}
```

**职责**:
- 应用日志条目到状态机
- 生成状态机快照
- 从快照恢复状态机状态
- 提供键值查询接口

### 3.9 RaftConfig

Raft 节点配置结构体。

```go
type RaftConfig struct {
    ElectionTimeoutMin time.Duration
    ElectionTimeoutMax time.Duration
    HeartbeatInterval  time.Duration
    SnapshotThreshold  int
    SnapshotChunkSize  int
}
```

**字段说明**:
- `ElectionTimeoutMin`: 选举超时最小时间
- `ElectionTimeoutMax`: 选举超时最大时间（随机化避免活锁）
- `HeartbeatInterval`: Leader 心跳间隔
- `SnapshotThreshold`: 触发快照的日志条数阈值
- `SnapshotChunkSize`: 快照分块大小

## 4. Raft 共识三大子问题

### 4.1 Leader 选举

#### 4.1.1 基本流程

```
节点启动
  │
  ▼
Follower 状态
  │
  ├─ 收到 Leader 心跳 → 重置选举计时器，保持 Follower
  │
  └─ 选举超时 → 转为 Candidate
                  │
                  ▼
            增加任期号
            给自己投票
            向其他节点请求投票
                  │
         ┌────────┴────────┐
         ▼                 ▼
    获得多数票         未获得多数票
         │                 │
         ▼                 ▼
    成为 Leader      等待下一次超时
    开始发心跳        重新发起选举
```

#### 4.1.2 关键机制

**随机化选举超时**:
- 每个节点的选举超时时间在 [ElectionTimeoutMin, ElectionTimeoutMax) 范围内随机选择
- 避免多个节点同时超时导致的选票分散
- 通常建议超时时长远大于网络往返时间

**投票规则**:
- 每个任期内，每个节点最多投一票
- 先到先得（先来的请求先获得投票）
- 候选人的日志必须至少和自己的日志一样新才投票

**日志新旧判断**:
- 首先比较最后一条日志的任期号，任期号大的更新
- 任期号相同则比较日志长度，长的更新

**选举获胜条件**:
- 获得当前配置中多数节点的投票
- 联合共识期间需要同时获得旧配置和新配置的多数票

#### 4.1.3 RequestVote RPC

请求投票 RPC，由 Candidate 在选举时发起。

**请求参数**:
- `Term`: 候选人的任期号
- `CandidateID`: 候选人 ID
- `LastLogIndex`: 候选人最后一条日志的索引
- `LastLogTerm`: 候选人最后一条日志的任期

**响应结果**:
- `Term`: 接收节点的当前任期（用于候选人更新自己）
- `VoteGranted`: 是否同意投票

**处理逻辑**:
1. 如果请求任期 > 当前任期，转为 Follower 并更新任期
2. 如果请求任期 < 当前任期，拒绝投票
3. 如果还没投票（或已投给该候选人），且候选人日志至少一样新，则同意投票
4. 同意投票后重置选举计时器

### 4.2 日志复制

#### 4.2.1 基本流程

```
客户端提交命令
      │
      ▼
   Leader 接收
      │
      ├─ 追加到本地日志
      │
      └─ 并发向所有 Followers 发送 AppendEntries
                │
       ┌────────┴────────┐
       ▼                 ▼
  多数节点确认      未达多数节点
       │                 │
       ▼                 ▼
  标记为已提交      重试（指数退避）
  应用到状态机
  更新 commitIndex
  后续心跳通知 Followers
```

#### 4.2.2 关键机制

**日志匹配属性**:
- 不同日志中的相同索引和任期的条目，存储相同的命令
- 不同日志中的相同索引和任期的条目，之前的所有条目也都一致

**一致性检查**:
- AppendEntries 请求包含 `PrevLogIndex` 和 `PrevLogTerm`
- Follower 检查自己在该索引处的日志任期是否匹配
- 不匹配则返回失败，并告知冲突位置，Leader 回退重试

**提交规则**:
- Leader 只能提交当前任期的日志
- 当前任期的日志被复制到多数节点后即标记为已提交
- 之前任期的日志通过"附带提交"方式提交（随当前任期日志一起提交）

**状态机应用**:
- 已提交的日志按顺序应用到状态机
- 每个节点独立应用，但最终状态一致
- `lastApplied` 记录已应用到状态机的最高日志索引

#### 4.2.3 AppendEntries RPC

追加日志 RPC，由 Leader 发起，用于日志复制和心跳。

**请求参数**:
- `Term`: Leader 的任期号
- `LeaderID`: Leader ID（用于 Followers 重定向客户端）
- `PrevLogIndex`: 前一条日志的索引
- `PrevLogTerm`: 前一条日志的任期
- `Entries`: 要追加的日志条目（心跳时为空）
- `LeaderCommit`: Leader 的已提交索引

**响应结果**:
- `Term`: 接收节点的当前任期（用于 Leader 更新自己）
- `Success`: 是否成功追加
- `MatchIndex`: 匹配的最高日志索引（成功时）
- `ConflictTerm`: 冲突条目的任期（失败时）
- `ConflictIndex`: 冲突条目的起始索引（失败时）

**处理逻辑**:
1. 如果请求任期 > 当前任期，转为 Follower 并更新任期
2. 如果请求任期 < 当前任期，返回失败
3. 检查 PrevLogIndex/PrevLogTerm 是否匹配
4. 不匹配则返回冲突信息，Leader 据此回退 nextIndex
5. 匹配则追加新日志（冲突时截断并覆盖）
6. 根据 LeaderCommit 更新本地 commitIndex
7. 通知应用协程应用已提交日志

### 4.3 成员变更

#### 4.3.1 联合共识两阶段

成员变更采用两阶段联合共识（Joint Consensus）确保安全过渡：

```
阶段一：联合配置 (C_old + C_new)
  │
  ├─ Leader 追加 LogEntryConfigJoint 日志
  ├─ 复制到所有节点
  ├─ 需要 C_old 和 C_new 两边都达到多数派才提交
  └─ 提交后，所有决策都使用联合配置（两边都要多数）

阶段二：新配置 (C_new)
  │
  ├─ Leader 追加 LogEntryConfigNew 日志
  ├─ 复制到所有节点
  ├─ 需要 C_old 和 C_new 两边都达到多数派才提交
  └─ 提交后，切换到新配置，旧配置失效
```

#### 4.3.2 关键机制

**联合配置特性**:
- 日志同时复制到旧配置和新配置中的所有节点
- 选举和提交需要同时获得旧配置多数派和新配置多数派
- 保证变更期间集群仍能正常处理请求

**安全性保证**:
- 不会出现两个独立的多数派
- 任何时刻最多只有一个 Leader
- 配置变更过程中不会中断服务

**新节点追赶**:
- 新节点加入前，Leader 先确保其日志追赶上
- 日志落后过多时通过快照安装快速追赶
- 追赶完成后才正式进入联合共识阶段

#### 4.3.3 配置变更日志条目

成员变更通过特殊类型的日志条目记录：

- `LogEntryConfigJoint`: 联合配置条目，标志第一阶段开始
- `LogEntryConfigNew`: 新配置条目，标志第二阶段开始

配置变更日志条目通过普通的日志复制机制达成共识，遵循相同的提交规则。

## 5. 快照安装

### 5.1 触发条件

当 Leader 检测到某个 Follower 的 `nextIndex` 已经落后于 `logOffset`（即需要的日志条目已被压缩删除）时，触发快照安装。

### 5.2 基本流程

```
Leader 检测到 Follower 日志过于落后
          │
          ▼
    生成状态机快照
          │
          ▼
    发送 InstallSnapshot RPC
          │
    ┌─────┴─────┐
    ▼           ▼
  成功        失败
    │           │
    ▼           ▼
更新 nextIndex  重试
更新 matchIndex
Follower 替换状态机
Follower 截断日志到快照索引
```

### 5.3 InstallSnapshot RPC

安装快照 RPC，由 Leader 向落后的 Follower 发送。

**请求参数**:
- `Term`: Leader 的任期号
- `LeaderID`: Leader ID
- `LastIncludedIndex`: 快照包含的最后日志索引
- `LastIncludedTerm`: 快照包含的最后日志任期
- `Config`: 快照时刻的集群配置
- `Data`: 快照数据
- `Done`: 是否是最后一个块（当前实现一次性发送）

**响应结果**:
- `Term`: 接收节点的当前任期
- `Success`: 是否安装成功

**处理逻辑**:
1. 如果请求任期 > 当前任期，转为 Follower
2. 如果请求任期 < 当前任期，返回失败
3. 如果快照索引小于等于本地已应用快照索引，直接返回成功
4. 应用快照到状态机
5. 更新 lastSnapshotIndex/lastSnapshotTerm
6. 截断本地日志到快照索引
7. 更新 commitIndex 和 lastApplied

## 6. 使用示例

### 6.1 创建单节点

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/raftlog"
)

func main() {
    transport := raftlog.NewMemoryTransport()
    sm := raftlog.NewMemoryStateMachine()
    cfg := raftlog.DefaultRaftConfig()

    node := raftlog.NewRaftNode("node1", cfg, sm, transport, []string{"node1"})
    node.Start()
    defer node.Stop()

    time.Sleep(100 * time.Millisecond)

    fmt.Println("Node state:", node.State())
    fmt.Println("Current term:", node.CurrentTerm())
}
```

### 6.2 创建三节点集群

```go
nodeIDs := []string{"n1", "n2", "n3"}
cfg := raftlog.DefaultRaftConfig()

cluster, err := raftlog.NewCluster(nodeIDs, cfg, nil)
if err != nil {
    log.Fatal(err)
}
defer cluster.Stop()

cluster.Start()

leader, err := cluster.WaitForLeader(2 * time.Second)
if err != nil {
    log.Fatal("No leader elected:", err)
}

fmt.Println("Leader:", leader.ID())
```

### 6.3 提交日志条目

```go
// 向 Leader 提交命令
idx, term, err := leader.Propose([]byte("set:key1=value1"))
if err != nil {
    log.Fatal("Propose failed:", err)
}

fmt.Printf("Proposed entry: index=%d, term=%d\n", idx, term)

// 等待提交
deadline := time.Now().Add(1 * time.Second)
for time.Now().Before(deadline) {
    if leader.CommitIndex() >= idx {
        fmt.Println("Entry committed!")
        break
    }
    time.Sleep(10 * time.Millisecond)
}
```

### 6.4 使用状态机

```go
sm := raftlog.NewMemoryStateMachine()

// 应用日志
entry := &raftlog.LogEntry{
    Term:    1,
    Index:   1,
    Type:    raftlog.LogEntryNormal,
    Command: []byte("mykey"),
}
sm.Apply(entry)

// 查询状态
val, ok := sm.Get("mykey")
if ok {
    fmt.Println("Value:", val)
}

// 生成快照
snapData, err := sm.Snapshot()
if err != nil {
    log.Fatal(err)
}

// 从快照恢复
snap := &raftlog.Snapshot{
    LastIncludedIndex: 1,
    LastIncludedTerm:  1,
    Data:              snapData,
}
sm.ApplySnapshot(snap)
```

### 6.5 动态添加节点

```go
// 先创建并启动新节点
newSM := raftlog.NewMemoryStateMachine()
err := cluster.AddNode("n4", newSM)
if err != nil {
    log.Fatal("AddNode failed:", err)
}

// 等待配置变更完成
time.Sleep(200 * time.Millisecond)

fmt.Println("Cluster size:", cluster.NodeCount())
```

### 6.6 动态移除节点

```go
err := cluster.RemoveNode("n3")
if err != nil {
    log.Fatal("RemoveNode failed:", err)
}

time.Sleep(200 * time.Millisecond)
fmt.Println("Cluster size:", cluster.NodeCount())
```

### 6.7 日志压缩

```go
// 提交若干日志后进行压缩
for i := 0; i < 100; i++ {
    leader.Propose([]byte(fmt.Sprintf("cmd-%d", i)))
}

// 等待提交
time.Sleep(500 * time.Millisecond)

// 压缩到指定索引
err := leader.CompactLog(50)
if err != nil {
    log.Fatal("CompactLog failed:", err)
}

fmt.Println("Log offset:", leader.LastLogIndex())
```

### 6.8 带延迟的内存传输

```go
// 创建带 10ms 网络延迟的传输层
transport := raftlog.NewMemoryTransportWithDelay(10 * time.Millisecond)

sm := raftlog.NewMemoryStateMachine()
cfg := raftlog.DefaultRaftConfig()

node := raftlog.NewRaftNode("node1", cfg, sm, transport, []string{"node1"})
node.Start()
```

## 7. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrNodeStopped` | 节点已停止 | 对已停止的节点调用操作 |
| `ErrNotLeader` | 节点不是 Leader | 向 Follower 提交日志或变更配置 |
| `ErrInvalidIndex` | 索引无效 | 压缩索引大于提交索引 |
| `ErrLogCompacted` | 日志已被压缩 | 访问已压缩的日志条目 |
| `ErrSnapshotInstalling` | 快照安装中 | 预留错误，当前未使用 |
| `ErrConfigChangeInFlight` | 配置变更进行中 | 已有变更时再次发起变更 |
| `ErrEmptyConfig` | 配置为空 | 移除最后一个节点 |
| `ErrNodeNotFound` | 节点不存在 | 访问不存在的节点 |
| `ErrNodeExists` | 节点已存在 | 添加已存在的节点 |
| `ErrTransportClosed` | 传输层已关闭 | 预留错误，当前未使用 |
| `ErrApplyFailed` | 应用失败 | 状态机应用失败 |

## 8. 性能与并发特征

### 8.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| 日志追加 | O(1) | 本地追加，分摊常数时间 |
| 日志提交 | O(N) | N 为节点数，需要多数派确认 |
| 日志应用 | O(1) | 状态机应用单条日志 |
| 快照生成 | O(M) | M 为状态机数据量 |
| 快照安装 | O(M) | M 为状态机数据量 |
| 成员变更 | O(N) | N 为节点数，两阶段提交 |

### 8.2 并发安全

- 所有共享状态通过 `sync.Mutex` 保护
- RPC 处理是线程安全的
- 状态机操作由单协程顺序应用，保证一致性
- 心跳和日志复制通过 goroutine 并发发送

### 8.3 一致性保证

模块提供以下一致性保证：

| 特性 | 保证 |
|------|------|
| 选举安全性 | 每个任期最多一个 Leader |
| Leader 只追加 | Leader 只追加日志，不删除或修改 |
| 日志匹配 | 相同索引和任期的日志内容相同，前缀也相同 |
| Leader 完整性 | 已提交的日志必然存在于新 Leader 中 |
| 状态机安全性 | 每个状态机按相同顺序应用相同的日志 |

## 9. 注意事项与限制

1. **内存传输**: 当前实现使用内存传输，仅适用于测试和单进程场景，生产环境需实现真实网络传输
2. **无持久化**: 日志和状态均在内存中，进程重启后丢失，生产环境需增加持久化
3. **单线程状态机应用**: 状态机应用在独立协程中顺序执行，保证一致性
4. **无分块快照**: 当前快照一次性发送，大数据量下可能占用较多内存
5. **配置变更串行化**: 同一时间只能有一个配置变更在进行中
6. **选举超时设置**: 选举超时时长应远大于网络往返时间和心跳间隔，避免不必要的选举
7. **日志压缩手动触发**: 当前需手动调用 CompactLog，生产环境建议增加自动压缩策略
8. **联合共识期间的提交**: 联合共识期间，普通日志的提交也需要两边多数派，可能影响性能
