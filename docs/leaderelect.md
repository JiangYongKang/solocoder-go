# Leader 选举模块需求文档

## 1. 模块概述

Leader 选举模块是一个基于租约（Lease）的分布式 Leader 选举实现，提供候选者竞选、心跳续约、故障自动重选举和事件回调通知等核心功能。模块通过分布式锁机制确保同一时刻只有一个 Leader，有效避免脑裂问题，适用于需要主从架构、任务调度协调、配置中心等需要单一协调者的分布式场景。

### 主要特性

- **候选者竞选**：多个候选者实例同时争抢成为 Leader，最先成功获取租约的候选者成为当前 Leader
- **防脑裂保护**：基于分布式锁的原子性确保同一时刻只有一个 Leader，避免多个实例同时声称自己是 Leader
- **心跳续约**：当前 Leader 在租约到期前通过定时心跳续约，每次续约成功将租约过期时间向后顺延
- **自动重选举**：Followers 定期检查 Leader 心跳状态，若在指定周期内未检测到心跳则判定 Leader 宕机并触发新一轮选举
- **事件回调**：提供角色变更（Leader/Follower）、选举开始/结束等事件的回调通知机制
- **优雅退出**：Leader 可主动辞职（Resign），释放租约并转为 Follower，触发新一轮选举
- **原 Leader 恢复**：原 Leader 恢复后以 Follower 身份重新加入集群，等待下一次竞选机会

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    LeaseDuration   time.Duration
    HeartbeatFactor float64
    CheckFactor     float64
}
```

**职责**：Leader 选举配置结构体，定义选举的时间参数和运行策略。

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `LeaseDuration` | 租约有效期，Leader 获取锁后的持有时间 | 5秒 |
| `HeartbeatFactor` | 心跳间隔系数，心跳间隔 = LeaseDuration * HeartbeatFactor | 0.3 |
| `CheckFactor` | 检查间隔系数，Follower 检查 Leader 状态的间隔 = LeaseDuration * CheckFactor | 0.5 |

**设计说明：
- HeartbeatFactor 必须小于 CheckFactor，确保 Leader 在 Follower 检测到租约过期前完成续约
- 两个系数都必须在 (0, 1) 区间内
- LeaseDuration 越长，选举灵敏度越低但网络开销越小

### 2.2 NodeRole

```go
type NodeRole int

const (
    RoleFollower NodeRole = iota
    RoleCandidate
    RoleLeader
)
```

**职责**：节点角色枚举，表示节点在选举过程中的身份状态。

| 角色 | 说明 |
|------|------|
| `RoleFollower` | 追随者角色，被动等待选举结果，监控 Leader 状态 |
| `RoleCandidate` | 候选者角色，正在参与选举竞争 |
| `RoleLeader` | 领导者角色，持有租约并定期续约 |

### 2.3 ElectionEventType

```go
type ElectionEventType int

const (
    EventBecomeLeader ElectionEventType = iota
    EventBecomeFollower
    EventElectionStart
    EventElectionEnd
    EventHeartbeat
    EventLeaderLost
)
```

**职责**：选举事件类型枚举，定义所有可触发回调的事件类型。

| 事件类型 | 说明 |
|--------|------|
| `EventBecomeLeader` | 节点角色变为 Leader |
| `EventBecomeFollower` | 节点角色变为 Follower |
| `EventElectionStart` | 开始新一轮选举 |
| `EventElectionEnd` | 选举结束（产生新 Leader 或确认现有 Leader 有效 |
| `EventHeartbeat` | Leader 成功完成一次心跳续约 |
| `EventLeaderLost` | Leader 失去领导地位（如心跳失败或主动辞职） |

### 2.4 ElectionEvent

```go
type ElectionEvent struct {
    Type      ElectionEventType
    NodeID    string
    Role      NodeRole
    LeaderID  string
    Term      int64
    Timestamp time.Time
}
```

**职责**：选举事件结构体，封装事件发生时的上下文信息，通过回调函数传递给调用方。

| 字段 | 说明 |
|------|------|
| `Type` | 事件类型 |
| `NodeID` | 当前节点 ID |
| `Role` | 当前节点角色 |
| `LeaderID` | 当前已知的 Leader ID |
| `Term` | 当前选举任期号，每次新选举递增 |
| `Timestamp` | 事件发生的时间戳 |

### 2.5 LockBackend

```go
type LockBackend interface {
    TryLock(key, token string, ttl time.Duration) (bool, error)
    Heartbeat(key, token string, ttl time.Duration) error
    Unlock(key, token string) error
    GetHolder(key string) (string, int, time.Duration, error)
    ısLocked(key string) (bool, error)
}
```

**职责**：锁后端接口，抽象底层分布式锁的操作，使选举模块与具体锁实现解耦。

| 方法 | 说明 |
|------|------|
| `TryLock` | 尝试获取锁，非阻塞，成功返回 true |
| `Heartbeat` | 心跳续约，延长锁的持有时间 |
| `Unlock` | 释放锁 |
| `GetHolder` | 获取当前锁的持有者 token |
| `IsLocked` | 检查锁是否被持有 |

模块提供基于内存锁后端实现：`lockManagerBackend`，基于 `distlock.LockManager`，通过 `NewLockManagerBackend()` 工厂函数创建。

### 2.6 LeaderElector

```go
type LeaderElector struct {
    nodeID       string
    electionKey  string
    cfg          Config
    backend      LockBackend
    // ... 内部字段
}
```

**职责**：Leader 选举器，是模块的核心结构体，管理节点的选举生命周期。

#### 核心方法

| 方法 | 说明 |
|------|------|
| `NewLeaderElector` | 创建新的选举器实例 |
| `Start` | 启动选举器，开始参与选举 |
| `Stop` | 停止选举器，释放资源 |
| `RegisterCallback` | 注册选举事件回调函数 |
| `Role` | 获取当前节点角色 |
| `IsLeader` | 判断当前节点是否为 Leader |
| `LeaderID` | 获取当前 Leader 的节点 ID |
| `Term` | 获取当前选举任期号 |
| `NodeID` | 获取当前节点 ID |
| `Resign` | 主动辞职，释放 Leader 身份 |
| `Running` | 判断选举器是否正在运行 |

## 3. 选举状态流转

### 3.1 状态流转图

```
                    ┌─────────────────┐
                    │   Follower   │
                    └────────┬────┘
                             │
                             │ 检测到 Leader 不存在/租约过期
                             ▼
                    ┌─────────────────┐
                    │  ElectionStart  │
                    │ (成为 Candidate) │
                    └────────┬────┘
                             │
              ┌──────────────┴───────────────┐
              │                              │
              ▼                              ▼
     ┌──────────────┐            ┌──────────────┐
     │  获取锁成功  │            │  获取锁失败  │
     │ 成为 Leader │            │  转为 Follower │
     └──────┬─────┘            └──────┬─────┘
            │                               │
            ▼                               ▼
     ┌──────────────┐            ┌──────────────┐
     │    Leader     │◄──────────┐  Follower  │
     └──────┬──────┘            └──────────────┘
            │                                    │
            │ 心跳续约失败/主动辞职               │
            ▼                                    │
     ┌──────────────┐                           │
     │ LeaderLost   │                           │
     │ 转为 Follower │───────────────────────────┘
     └──────────────┘
```

### 3.2 状态流转说明

#### Follower → Candidate

- **触发条件**：Follower 定期检查 Leader 状态，发现 Leader 不存在、租约已过期或后端返回任何错误（保守策略，避免漏检 Leader 宕机）
- **行为**：节点角色变为 Candidate，任期号递增，发送 EventElectionStart 事件

#### Candidate → Leader

- **触发条件**：候选者成功获取到分布式锁（租约）
- **行为**：节点角色变为 Leader，发送 EventElectionEnd 和 EventBecomeLeader 事件

#### Candidate → Follower

- **触发条件**：候选者未获取到锁（其他候选者已成为 Leader
- **行为**：节点角色变为 Follower，记录当前 Leader ID，发送 EventElectionEnd 和 EventBecomeFollower 事件

#### Leader → Follower

- **触发条件 1**：心跳续约失败（租约可能已过期）
- **触发条件 2**：主动调用 Resign() 辞职
- **行为**：节点角色变为 Follower，发送 EventLeaderLost 和 EventBecomeFollower 事件

#### Follower → Follower（检测到新 Leader）

- **触发条件**：Follower 检测到新的 Leader（与之前记录的不同
- **行为**：更新 Leader ID，发送 EventElectionEnd 事件

## 4. 核心流程

### 4.1 选举流程

1. **启动检查**：选举器启动后立即进行一次 Leader 状态检查
2. **Leader 不存在**：如果没有 Leader 或租约已过期，触发选举
3. **成为候选者**：节点角色变为 Candidate，任期号递增
4. **尝试获取锁**：调用 TryLock 尝试获取分布式锁
5. **获取成功**：成为 Leader，启动心跳续约
6. **获取失败**：转为 Follower，记录当前 Leader

### 4.2 心跳续约流程

1. **定时触发**：Leader 按 HeartbeatInterval 定时触发心跳
2. **续约操作**：调用 Heartbeat 方法延长租约时间
3. **续约成功**：发送 EventHeartbeat 事件
4. **续约失败**：转为 Follower，发送 EventLeaderLost 和 EventBecomeFollower 事件

### 4.3 Follower 检查流程

1. **定时检查**：Follower 按 CheckInterval 定时检查 Leader 状态
2. **查询持有者**：调用 GetHolder 查询当前锁的持有者
3. **Leader 存在**：更新 Leader ID，若检测到新 Leader 则发送 EventElectionEnd 事件
4. **Leader 不存在或查询失败**：如果后端返回任何错误（包括 Leader 不存在、租约过期或其他异常），采用保守策略触发新一轮选举，避免因后端异常导致选举延迟

### 4.4 故障恢复流程

1. **Leader 宕机**：Leader 节点宕机后，租约到期自动过期
2. **Follower 检测**：Follower 在检查时发现 Leader 不存在
3. **触发重选**：所有 Follower 转为 Candidate 参与新一轮选举
4. **新 Leader 产生**：最先获取锁的候选者成为新 Leader
5. **原 Leader 恢复**：原 Leader 恢复后以 Follower 身份加入，不影响当前 Leader

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "time"

    "solocoder-go/internal/distlock"
    "solocoder-go/internal/leaderelect"
)

func main() {
    lm := distlock.NewLockManager()
    backend := leaderelect.NewLockManagerBackend(lm)

    cfg := leaderelect.DefaultConfig()

    elector, err := leaderelect.NewLeaderElector(
        "node-1",
        "my-service-election",
        cfg,
        backend,
    )
    if err != nil {
        panic(err)
    }

    elector.RegisterCallback(func(event leaderelect.ElectionEvent) {
        switch event.Type {
        case leaderelect.EventBecomeLeader:
            fmt.Println("I am the leader now!")
        case leaderelect.EventBecomeFollower:
            fmt.Println("I am a follower now.")
        }
    })

    if err := elector.Start(); err != nil {
        panic(err)
    }
    defer elector.Stop()

    select {}
}
```

### 5.2 多节点选举

```go
func main() {
    lm := distlock.NewLockManager()

    nodes := []string{"node-1", "node-2", "node-3"}
    electors := make([]*leaderelect.LeaderElector, 0, len(nodes))

    for _, nodeID := range nodes {
        backend := leaderelect.NewLockManagerBackend(lm)
        cfg := leaderelect.DefaultConfig()

        elector, err := leaderelect.NewLeaderElector(
            nodeID,
            "service-election",
            cfg,
            backend,
        )
        if err != nil {
            panic(err)
        }

        elector.RegisterCallback(func(event leaderelect.ElectionEvent) {
            fmt.Printf("[%s] %s: role=%s, leader=%s\n",
                event.Timestamp.Format("15:04:05"),
                event.Type,
                event.Role,
                event.LeaderID,
            )
        })

        electors = append(electors, elector)
    }

    for _, e := range electors {
        _ = e.Start()
        defer e.Stop()
    }

    time.Sleep(10 * time.Second)
}
```

### 5.3 主动辞职

```go
if elector.IsLeader() {
    fmt.Println("Resigning as leader...")
    if err := elector.Resign(); err != nil {
        log.Printf("Resign error: %v", err)
    }
}
```

## 6. 错误定义

| 错误 | 说明 |
|------|------|
| `ErrElectorStopped` | 选举器已停止 |
| `ErrElectorRunning` | 选举器已在运行 |
| `ErrInvalidConfig` | 配置无效 |
| `ErrEmptyNodeID` | 节点 ID 为空 |
| `ErrEmptyElectionKey` | 选举 Key 为空 |

## 7. 并发安全

LeaderElector 的所有公共方法均为并发安全，可在多个 goroutine 中同时调用。内部使用互斥锁（`mu`）保护共享状态（`role`、`term`、`leaderID`、`running`、`stopped` 等字段），使用读写锁（`callbacksMu`）保护回调函数列表。

事件通知机制的并发设计：
- `notify` 方法在构造 `ElectionEvent` 时先持有 `mu` 互斥锁读取共享字段（`role`、`leaderID`、`term`）的快照，确保不会出现并发读写的数据竞争
- 回调函数在单独的 goroutine 中异步执行，不会阻塞选举主循环（包括心跳发送和 Leader 检查）
- 回调函数列表在调用前会复制一份快照，避免后续注册/注销操作影响当前通知

回调函数注意事项：
- 回调函数应尽量简短，虽然不会阻塞选举主循环，但会占用独立的 goroutine 资源
- 回调函数中不应调用选举器的阻塞方法（如 Stop）
- 多个回调函数按注册顺序依次在同一个异步 goroutine 中执行
- 由于回调是异步执行的，事件到达顺序可能与实际状态变化存在微小时间差
