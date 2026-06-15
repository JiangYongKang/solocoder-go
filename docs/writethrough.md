# Write-Through 缓存策略模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [写入策略详解](#4-写入策略详解)
5. [Delete 操作一致性策略](#5-delete-操作一致性策略)
6. [状态流转机制](#6-状态流转机制)
7. [读操作与缓存回填](#7-读操作与缓存回填)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全](#10-并发安全)
11. [修复说明与最佳实践](#11-修复说明与最佳实践)

---

## 1. 模块概述

Write-Through（写穿透）缓存策略模块是一个高性能、高可用的缓存抽象层，提供缓存与底层存储的同步写入、失败重试、自动降级恢复以及读穿透缓存回填等功能。模块设计用于需要数据强一致性和高可用性的缓存场景。

**包路径**: `internal/writethrough`

**设计目标**:
- 保证缓存与底层存储的数据一致性
- 支持存储故障时的自动降级与恢复
- 提供后台重试机制提高写入成功率
- 通过读穿透和缓存回填提升读性能

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Write-Through 写入 | 写操作先写入缓存，再同步写入底层存储，两者都成功才返回成功 |
| 写入失败重试 | 底层存储写入失败时自动重试，重试次数和间隔可配置 |
| 待处理队列 | 写入失败的数据暂存于缓存，后台异步重试 |
| Write-Around 自动降级 | 连续写入失败超过阈值时自动降级为 Write-Around 模式 |
| 自动恢复机制 | 时间窗口内连续成功达到阈值时自动恢复为 Write-Through 模式 |
| 读穿透 + 缓存回填 | 缓存未命中时从底层存储读取，数据自动回填到缓存 |
| Delete 操作一致性 | 删除操作先删除存储，成功后再删除缓存，确保数据一致性 |

---

## 3. 核心结构体与职责

### 3.1 WriteThroughCache

主缓存结构体，对外提供所有操作接口，协调缓存与底层存储的交互。

```go
type WriteThroughCache struct {
    cache     Cache
    storage   Storage
    config    Config
    strategy  atomic.Int32
    // ... 内部状态字段
}
```

**职责**:
- 管理缓存与底层存储的交互策略
- 维护当前写入策略状态（WriteThrough / WriteAround）
- 处理写入失败的重试逻辑
- 监控失败/成功计数，触发降级与恢复
- 管理后台重试协程的生命周期
- 区分前台操作与后台操作对状态的影响

### 3.2 Storage 接口

底层存储接口，定义了持久化存储的操作契约。

```go
type Storage interface {
    Get(key string) (string, error)
    Put(key string, value string) error
    Delete(key string) error
}
```

**职责**:
- 定义底层存储的抽象接口
- 支持任意实现（数据库、文件系统、远程存储等）
- 返回错误供上层进行重试和降级决策

### 3.3 Cache 接口

缓存层接口，定义内存缓存的操作契约。

```go
type Cache interface {
    Get(key string) (string, bool)
    Put(key string, value string)
    Delete(key string) bool
}
```

**职责**:
- 定义缓存层的抽象接口
- 默认提供基于内存 map 的实现（memoryCache）
- 支持替换为其他缓存实现（如 LRU、TTL 缓存等）

### 3.4 Config

配置结构体，用于定制模块行为。

```go
type Config struct {
    MaxRetries        int           // 最大重试次数
    RetryInterval     time.Duration // 重试间隔
    DegradeThreshold  int           // 降级阈值（连续写入失败次数）
    RecoverThreshold  int           // 恢复阈值（时间窗口内成功次数）
    RecoverWindow     time.Duration // 恢复统计时间窗口
    EnableReadThrough bool          // 是否启用读穿透
}
```

**默认配置**:
- `MaxRetries`: 3 次
- `RetryInterval`: 100ms
- `DegradeThreshold`: 5 次
- `RecoverThreshold`: 3 次
- `RecoverWindow`: 5 秒
- `EnableReadThrough`: true

### 3.5 WriteStrategy

写入策略枚举类型。

```go
type WriteStrategy int

const (
    WriteThroughStrategy WriteStrategy = iota
    WriteAroundStrategy
)
```

**策略说明**:
- `WriteThroughStrategy`: 写穿透模式，写入缓存同时写入存储
- `WriteAroundStrategy`: 绕写模式，只写入存储，不更新缓存

### 3.6 pendingItem

待重试数据项，记录需要后台重试的键值对。

```go
type pendingItem struct {
    key      string
    value    string
    retryCnt int
    nextTry  time.Time
}
```

**职责**:
- 保存写入失败的键值数据
- 记录已重试次数
- 记录下一次重试时间

---

## 4. 写入策略详解

### 4.1 Write-Through（写穿透）

Write-Through 策略确保缓存与底层存储的数据一致性。写入时先写缓存，再写存储，两者都成功才返回成功。

**写入流程**:

```
Put(key, value)
    │
    ├─→ 步骤 1：写入缓存
    │       └─ cache.Put(key, value)  // 立即成功，保证读可获取最新值
    │
    ├─→ 步骤 2：写入底层存储（同步进行）
    │       │
    │       ├─ 成功分支
    │       │    ├─ 调用 recordSuccess(true)
    │       │    ├─ 重置前台失败计数为 0
    │       │    ├─ 记录恢复成功时间
    │       │    └─ 返回 nil（写入成功）
    │       │
    │       └─ 失败分支
    │            ├─ 重试机制（最多 MaxRetries 次）
    │            │    ├─ 每次间隔 RetryInterval
    │            │    ├─ 重试成功 → 进入成功分支
    │            │    └─ 重试失败 → 继续重试
    │            │
    │            └─ 所有重试都失败
    │                 ├─ 调用 recordFailure(true)
    │                 ├─ 前台失败计数 +1
    │                 ├─ 加入待处理队列（由后台异步重试）
    │                 ├─ 检查是否触发降级
    │                 │    ├─ 失败次数 >= DegradeThreshold → 降级为 WriteAround
    │                 │    └─ 否则 → 保持 WriteThrough
    │                 └─ 返回错误
    │
    └─ 后台重试循环（异步）
          │
          ├─ 定期检查待处理队列
          ├─ 对每个到期的待处理项调用 storage.Put
          │    ├─ 成功 → recordSuccess(false)，不重置前台失败计数
          │    └─ 失败 → recordFailure(false)，不增加前台失败计数
          │
          └─ 超过最大重试次数 → 从队列移除（放弃）
```

**关键特性**:
- 缓存总是先写入，保证读操作能拿到最新数据
- 存储写入失败时，数据仍在缓存中（高可用性）
- 后台异步重试，不阻塞后续操作
- 前台操作影响降级/恢复决策，后台操作不影响前台失败计数

### 4.2 Write-Around（绕写）

Write-Around 策略在底层存储不稳定时自动启用，避免缓存与存储的数据不一致。此时只写存储，不更新缓存。

**写入流程**:

```
Put(key, value)
    │
    └─→ 仅写入底层存储（不更新缓存）
            │
            ├─ 成功分支
            │    ├─ 调用 recordSuccess(true)
            │    ├─ 重置前台失败计数为 0
            │    ├─ 添加到恢复成功记录
            │    ├─ 检查是否满足恢复条件
            │    │    ├─ 时间窗口内成功次数 >= RecoverThreshold
            │    │    ├─ 是 → 恢复为 WriteThrough
            │    │    └─ 否 → 保持 WriteAround
            │    └─ 返回 nil（写入成功）
            │
            └─ 失败分支
                 ├─ 调用 recordFailure(true)
                 ├─ 前台失败计数 +1
                 ├─ 清空恢复成功记录
                 └─ 返回错误
```

**关键特性**:
- 只写存储，不更新缓存
- 避免存储失败时缓存与存储数据不一致
- 读操作仍可通过读穿透从存储获取数据
- 成功写入累积到一定数量可自动恢复为 Write-Through

**与 Write-Through 的对比**:

| 特性 | Write-Through | Write-Around |
|------|--------------|--------------|
| 写入缓存 | 是 | 否 |
| 写入存储 | 是 | 是 |
| 数据一致性 | 高（缓存始终有最新值） | 中（缓存可能不是最新） |
| 读性能 | 高（缓存命中率高） | 低（需靠读穿透回填） |
| 适用场景 | 存储稳定时 | 存储不稳定时 |
| 触发降级 | - | 连续写入失败达到阈值 |
| 触发恢复 | 连续成功达到阈值 | - |

---

## 5. Delete 操作一致性策略

### 5.1 一致性问题分析

**原始问题**：
如果先删除缓存再删除存储，当存储删除失败时会出现：
- 缓存已清空
- 存储中数据仍存在
- 后续 Get 通过读穿透回填数据，掩盖了删除失败的真相
- 造成缓存与存储数据不一致

**修复方案**：
Delete 操作采用 **先删存储，成功后再删缓存** 的顺序，保证：
- 存储删除失败时，缓存保持不变 → 读操作仍能获取数据
- 存储删除成功后，才删除缓存 → 数据最终一致
- 要么都删除成功，要么缓存保留数据

### 5.2 Delete 操作流程

```
Delete(key)
    │
    ├─ 判断当前策略
    │
    ├─ WriteThrough 模式
    │    │
    │    ├─ 步骤 1：调用 storage.Delete(key)
    │    │    │
    │    │    ├─ 失败
    │    │    │    ├─ 调用 recordFailure(false)
    │    │    │    ├─ 不增加前台失败计数
    │    │    │    ├─ 缓存保持不变
    │    │    │    └─ 返回错误
    │    │    │
    │    │    └─ 成功
    │    │         ├─ 调用 recordSuccess(false)
    │    │         ├─ 不清零前台失败计数
    │    │         ├─ 调用 cache.Delete(key)
    │    │         └─ 返回 nil
    │    │
    │    └─ 一致性保证：要么都删除，要么都保留
    │
    └─ WriteAround 模式
         │
         ├─ 步骤 1：调用 storage.Delete(key)
         │    │
         │    ├─ 失败
         │    │    ├─ 调用 recordFailure(false)
         │    │    ├─ 不增加前台失败计数
         │    │    └─ 返回错误
         │    │
         │    └─ 成功
         │         ├─ 调用 recordSuccess(false)
         │         ├─ 不清零前台失败计数
         │         ├─ 调用 cache.Delete(key)
         │         └─ 返回 nil
         │
         └─ 保持与 WriteThrough 一致的行为
```

### 5.3 Delete 操作与降级决策

根据需求，降级仅由"底层存储连续写入失败"触发，因此 Delete 操作不参与降级决策。

**各操作对降级计数的影响**:

| 操作类型 | 操作结果 | 对 failureCnt 的影响 | 是否影响降级 |
|---------|---------|---------------------|-------------|
| Put（前台） | 成功 | `failureCnt = 0` | 是（重置计数） |
| Put（前台） | 失败 | `failureCnt += 1` | 是（增加计数） |
| Delete（前台） | 成功 | `failureCnt 不变` | 否 |
| Delete（前台） | 失败 | `failureCnt 不变` | 否 |
| 后台重试 | 成功 | `failureCnt 不变` | 否 |
| 后台重试 | 失败 | `failureCnt 不变` | 否 |

**场景示例**:
- DegradeThreshold = 3
- 第 1 次 Put 失败 → failureCnt = 1 → 不降级
- 第 2 次 Put 失败 → failureCnt = 2 → 不降级
- Delete 成功 → failureCnt 仍为 2 → 不重置
- Delete 失败 → failureCnt 仍为 2 → 不增加
- 后台重试成功 → failureCnt 仍为 2 → 不重置
- 第 3 次 Put 失败 → failureCnt = 3 → 触发降级

---

## 6. 状态流转机制

### 6.1 完整状态流转图

```
                    ┌─────────────────────────────────┐
                    │                                 │
                    │        Write-Through            │
                    │           (正常模式)             │
                    │                                 │
                    │  • 同时写缓存和存储              │
                    │  • Put 失败计入降级计数          │
                    │  • Delete 不影响降级计数         │
                    │  • 后台重试不影响前台计数        │
                    │                                 │
                    └───────────────┬─────────────────┘
                                    │
                                    │
                                    │ 【降级条件】
                                    │  1. 当前处于 Write-Through 模式
                                    │  2. 前台 Put 操作存储写入失败
                                    │  3. 连续失败次数 >= DegradeThreshold
                                    │
                                    ▼
                    ┌─────────────────────────────────┐
                    │                                 │
                    │        Write-Around             │
                    │           (降级模式)             │
                    │                                 │
                    │  • 只写存储，不写缓存            │
                    │  • 成功写入计入恢复计数          │
                    │  • 使用滑动窗口统计成功次数      │
                    │                                 │
                    └───────────────┬─────────────────┘
                                    │
                                    │
                                    │ 【恢复条件】
                                    │  1. 当前处于 Write-Around 模式
                                    │  2. 前台 Put 操作存储写入成功
                                    │  3. RecoverWindow 时间窗口内
                                    │  4. 成功次数 >= RecoverThreshold
                                    │
                                    ▼
                    ┌─────────────────────────────────┐
                    │                                 │
                    │        Write-Through            │
                    │         (已恢复正常)             │
                    │                                 │
                    │  • 重置所有失败和成功计数        │
                    │  • 恢复正常写入流程              │
                    │                                 │
                    └─────────────────────────────────┘
```

### 6.2 降级触发条件（Write-Through → Write-Around）

**精确触发条件**:

```
IF 当前策略 == WriteThrough
   AND 操作类型 == Put（前台操作）
   AND 存储写入失败
   AND failureCnt + 1 >= DegradeThreshold
THEN 降级为