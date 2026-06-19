# 分布式锁模块 (distlock)

## 模块概述

distlock 是一个基于内存数据结构模拟分布式环境下锁管理的功能模块。该模块提供了完整的分布式锁能力，包括基于令牌的排他锁、锁超时自动过期、重入计数、心跳续期以及 Redlock 多节点冗余锁算法。所有锁节点使用内存数据结构实现，可用于测试和模拟分布式锁场景。

## 核心功能

### 1. 基于唯一令牌的锁获取与释放

每个锁由唯一的 `key` 标识，调用方在获取锁时需提供一个唯一的 `token` 作为锁的所有者标识：

- **加锁 (Lock/TryLock)**：调用方指定 key、token 和过期时间 TTL。若锁空闲或已过期，则创建锁记录并绑定 token；若锁已被其他 token 持有，则返回失败。
- **释放锁 (Unlock)**：仅持有对应 token 的调用方可以释放锁。token 不匹配的释放请求将被拒绝并返回 `ErrInvalidToken` 错误。
- **TryLock**：非阻塞尝试加锁，立即返回加锁是否成功，不会返回 `ErrLockAlreadyHeld` 错误。

### 2. 锁超时自动过期与心跳续期

每个锁在获取时必须设置过期时间 (TTL, Time To Live)：

- **自动过期**：锁持有者在过期时间内未主动释放锁，锁将自动过期并变为可用状态，防止调用方崩溃导致的死锁。
- **心跳机制 (Heartbeat)**：锁持有者可通过定期调用 `Heartbeat` 方法续期锁的 TTL，防止长时间任务执行期间锁被误释放。
- **自动清理**：`LockManager` 支持启动后台清理协程，按配置的 `CleanInterval` 周期自动清理过期的锁记录。

### 3. 重入计数

支持同一调用方（同一 token）对同一把锁的重入获取：

- **重入计数器**：每次成功获取锁时计数加一，每次释放锁时计数减一。
- **真正释放**：仅当重入计数归零时，锁才真正被释放并变为可用状态。
- **重入上限**：可通过 `LockManagerConfig.MaxReentrancy` 配置重入次数的上限，防止无限重入。默认上限为 32。当重入计数已达上限时，继续加锁将返回 `ErrMaxReentrancy` 错误。

### 4. Redlock 多节点冗余锁

实现了 Redis 官方提出的 Redlock 分布式锁算法，提供更高的容错能力：

- **多节点架构**：使用 N 个独立的锁节点（N 通常为奇数，如 3、5、7）。
- **多数派原则**：仅在超过半数节点（N/2 + 1）加锁成功时，才认为整体加锁成功。
- **最小过期时间**：每个节点加锁成功后立即记录该节点的实际过期时间（当前时刻 + TTL），总持有时间取各节点中的最小过期时间并扣除时钟漂移补偿，避免 TTL 被双倍缩短。
- **解锁广播**：解锁时向所有节点发送释放请求，确保锁在所有节点上被释放。
- **部分成功回滚**：若加锁未达到多数派，自动释放在已成功节点上获取的锁。
- **重试机制**：`Lock` 方法支持自动重试，在配置的 `AcquireTimeout` 超时前持续尝试加锁。

## 核心结构体与职责

### LockManager

单节点内存锁管理器，是整个模块的基础组件。

**职责**：
- 管理锁的生命周期（创建、获取、释放、过期）
- 验证锁持有者的 token 身份
- 维护锁的重入计数器，超过 `MaxReentrancy` 时拒绝加锁并返回 `ErrMaxReentrancy`
- 支持锁的心跳续期
- 提供后台过期锁清理能力
- 提供锁状态查询、强制释放、统计等辅助方法

**主要方法**：

| 方法 | 说明 |
|------|------|
| `NewLockManager()` | 创建默认配置的锁管理器 |
| `NewLockManagerWithConfig(cfg)` | 使用自定义配置创建锁管理器 |
| `Start()` | 启动后台过期锁清理协程 |
| `Stop()` | 停止后台清理协程，拒绝新的操作请求 |
| `Lock(key, token, ttl)` | 加锁，失败返回错误；重入达上限返回 `ErrMaxReentrancy` |
| `TryLock(key, token, ttl)` | 非阻塞尝试加锁 |
| `Unlock(key, token)` | 释放锁，验证 token，token 不匹配返回 `ErrInvalidToken` |
| `Heartbeat(key, token, ttl)` | 续期锁的 TTL |
| `IsLocked(key)` | 查询锁是否被持有 |
| `GetHolder(key)` | 获取锁持有者信息（token、重入计数、剩余 TTL） |
| `ForceUnlock(key)` | 强制释放锁（无需 token） |
| `Count()` | 统计当前有效锁数量 |
| `Clear()` | 清空所有锁 |
| `CleanExpired()` | 主动清理过期锁 |

### LockManagerConfig

`LockManager` 的配置结构体。

**字段**：

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `MaxReentrancy` | `int` | 最大重入次数，必须 >= 1，达到上限时 Lock 返回 `ErrMaxReentrancy` | 32 |
| `CleanInterval` | `time.Duration` | 后台清理周期，必须 >= 0 | 100ms |

### MemoryLockNode

`LockNode` 接口的内存实现，封装了 `LockManager` 并赋予节点 ID，用于构建 Redlock 的多节点集群。

**职责**：
- 作为 Redlock 中的一个独立锁节点
- 实现 `LockNode` 接口，提供统一的锁操作方法
- 通过唯一 ID 标识节点身份

**构造方法**：
- `NewMemoryLockNode(id)`：使用默认配置创建节点
- `NewMemoryLockNodeWithConfig(id, cfg)`：使用自定义 LockManagerConfig 创建节点

### LockNode 接口

定义了锁节点的统一操作契约，便于替换不同的锁实现（如未来可实现 RedisLockNode 等）。

```go
type LockNode interface {
    Lock(key, token string, ttl time.Duration) error
    TryLock(key, token string, ttl time.Duration) (bool, error)
    Unlock(key, token string) error
    Heartbeat(key, token string, ttl time.Duration) error
    IsLocked(key string) (bool, error)
    GetRemainingTTL(key string) (time.Duration, error)
    ID() string
}
```

### Redlock

Redlock 算法实现，通过协调多个独立的 `LockNode` 实现分布式冗余锁。

**职责**：
- 协调多节点的加锁操作，执行多数派判定
- 精确计算每个节点的实际过期时间（节点加锁成功时刻 + TTL），避免 TTL 双倍缩短
- 处理加锁失败时的部分节点回滚
- 支持加锁重试和超时控制
- 执行多节点的解锁广播和心跳续期

**主要方法**：

| 方法 | 说明 |
|------|------|
| `NewRedlock(nodes)` | 使用默认配置创建 Redlock 实例 |
| `NewRedlockWithConfig(nodes, cfg)` | 使用自定义配置创建 Redlock 实例 |
| `Lock(key, token, ttl)` | 多节点加锁，支持自动重试 |
| `TryLock(key, token, ttl)` | 单次尝试多节点加锁，不重试 |
| `Unlock(acq)` | 向所有节点广播释放锁 |
| `Heartbeat(acq, ttl)` | 在已成功的节点上批量续期 |
| `IsLocked(key)` | 检查多数节点上是否持有锁 |
| `NodeCount()` | 返回节点总数 |

### RedlockConfig

`Redlock` 的配置结构体。

**字段**：

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `AcquireTimeout` | `time.Duration` | 加锁总超时时间 | 5s |
| `RetryDelay` | `time.Duration` | 重试间隔时间 | 100ms |
| `ClockDrift` | `time.Duration` | 时钟漂移补偿量（仅在计算最终有效时间时扣除一次） | 50ms |

### LockAcquisition

表示一次成功的 Redlock 加锁结果，包含加锁的完整元信息。

**字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `Key` | `string` | 锁的唯一标识 |
| `Token` | `string` | 锁持有者令牌 |
| `Expiry` | `time.Time` | 总过期时间（取最小节点过期时间并扣除 ClockDrift） |
| `NodeExpiries` | `map[string]time.Time` | 各成功节点的实际过期时间（节点加锁成功时刻 + TTL） |
| `NodeCount` | `int` | 节点总数 |
| `SuccessCount` | `int` | 加锁成功的节点数 |

**方法**：
- `IsValid()`：判断锁是否仍然有效（未过期）
- `RemainingTTL()`：获取锁的剩余有效时间

## Redlock 多节点加锁流程

### 加锁流程（步骤级详解）

假设使用 3 个节点（N=3），多数派阈值 = N/2 + 1 = 2，TTL = 10 秒，ClockDrift = 50ms。

```
时间轴 ─────────────────────────────────────────────────────►

调用方 (Client)
  │
  │  T=0ms: 开始执行 attemptLock()
  │
  ├─ 步骤 1: 遍历所有 LockNode，逐节点加锁
  │   │
  │   ├─ T=0ms: 向 Node-1 发送 Lock(key, token, 10s)
  │   │     Node-1 加锁成功
  │   │     记录 NodeExpiries["Node-1"] = T+10s = 10000ms
  │   │     successCount = 1
  │   │
  │   ├─ T=10ms: 向 Node-2 发送 Lock(key, token, 10s)
  │   │     Node-2 加锁成功
  │   │     记录 NodeExpiries["Node-2"] = (T+10ms)+10s = 10010ms
  │   │     successCount = 2  ← 已达多数派
  │   │
  │   └─ T=20ms: 向 Node-3 发送 Lock(key, token, 10s)
  │         Node-3 加锁成功（可选，不影响结果）
  │         记录 NodeExpiries["Node-3"] = (T+20ms)+10s = 10020ms
  │         successCount = 3
  │
  ├─ 步骤 2: 多数派判定 (T=20ms)
  │   │  successCount(3) >= majority(2) → 满足多数派
  │   │  如果 successCount < majority → 跳转步骤 5（回滚）
  │   │
  │
  ├─ 步骤 3: 计算最小过期时间和最终有效时间 (T=20ms)
  │   │
  │   │  NodeExpiries:
  │   │    Node-1: 10000ms  ← 最小值 (minExpiry)
  │   │    Node-2: 10010ms
  │   │    Node-3: 10020ms
  │   │
  │   │  最终有效时间 Expiry = minExpiry - ClockDrift
  │   │                       = 10000ms - 50ms
  │   │                       = 9950ms
  │   │
  │   │  注: 每个节点的过期时间是其实际加锁时刻 + TTL，
  │   │      因此加锁耗时已经自然反映在各节点过期时间中，
  │   │      仅需扣除一次 ClockDrift，不会出现 TTL 双倍