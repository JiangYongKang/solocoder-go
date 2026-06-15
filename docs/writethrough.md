# Write-Through 缓存策略模块

## 1. 模块概述

Write-Through（写穿透）缓存策略模块是一个高性能、高可用的缓存抽象层，提供缓存与底层存储的同步写入、失败重试、自动降级恢复以及读穿透缓存回填等功能。模块设计用于需要数据强一致性和高可用性的缓存场景。

**包路径**: `internal/writethrough`

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

## 4. 写入策略详解

### 4.1 Write-Through（写穿透）

Write-Through 策略确保缓存与底层存储的数据一致性。

**写入流程**:
```
Put(key, value)
    │
    ├─→ 1. 写入缓存（立即成功，保证读可获取最新值）
    │
    ├─→ 2. 写入底层存储（同步进行）
    │       │
    │       ├─ 成功
    │       │    ├─ 调用 recordSuccess(true)
    │       │    ├─ 重置前台失败计数为 0
    │       │    ├─ 记录恢复成功时间
    │       │    └─ 返回 nil
    │       │
    │       └─ 失败
    │            ├─ 重试（最多 MaxRetries 次，每次间隔 RetryInterval）
    │            │    ├─ 重试成功 → 同上成功分支
    │            │    └─ 重试失败 → 继续重试
    │            │
    │            └─ 所有重试失败
    │                 ├─ 调用 recordFailure(true)
    │                 ├─ 前台失败计数 +1
    │                 ├─ 加入待处理队列（由后台异步重试）
    │                 ├─ 检查失败计数 >= DegradeThreshold？
    │                 │    ├─ 是 → 降级为 WriteAround
    │                 │    └─ 否 → 保持 WriteThrough
    │                 └─ 返回错误
    │
    └─ 后台重试循环定期处理待处理队列
          │
          ├─ 对每个到期的待处理项调用 storage.Put
          │    ├─ 成功 → recordSuccess(false)，不重置前台失败计数
          │    └─ 失败 → recordFailure(false)，不增加前台失败计数
          │
          └─ 超过最大重试次数 → 从队列移除
```

**关键特性**:
- 缓存总是先写入，保证读操作能拿到最新数据
- 存储写入失败时，数据仍在缓存中（高可用性）
- 后台异步重试，不阻塞后续操作
- 前台操作影响降级/恢复决策，后台操作不影响前台失败计数

### 4.2 Write-Around（绕写）

Write-Around 策略在底层存储不稳定时启用，避免缓存与存储的数据不一致。

**写入流程**:
```
Put(key, value)
    │
    └─→ 1. 直接写入底层存储（不更新缓存）
            │
            ├─ 成功
            │    ├─ 调用 recordSuccess(true)
            │    ├─ 重置前台失败计数为 0
            │    ├─ 添加到恢复成功记录
            │    ├─ 检查时间窗口内成功次数 >= RecoverThreshold？
            │    │    ├─ 是 → 恢复为 WriteThrough
            │    │    └─ 否 → 保持 WriteAround
            │    └─ 返回 nil
            │
            └─ 失败
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

## 5. Delete 操作一致性策略

### 5.1 一致性问题与解决方案

**原始问题**：
如果先删除缓存再删除存储，当存储删除失败时会出现：
- 缓存已清空
- 存储中数据仍存在
- 后续 Get 通过读穿透回填数据，掩盖了删除失败的真相
- 造成缓存与存储数据不一致

**修复后的策略**：
Delete 操作采用 **先删存储，成功后再删缓存** 的顺序：

```
Delete(key)
    │
    ├─ 判断当前策略
    │
    ├─ WriteThrough 模式
    │    │
    │    ├─ 1. 调用 storage.Delete(key)
    │    │    │
    │    │    ├─ 失败
    │    │    │    ├─ recordFailure(false)  // 不影响降级计数
    │    │    │    ├─ 返回错误
    │    │    │    └─ 缓存保持不变（保证读一致性）
    │    │    │
    │    │    └─ 成功
    │    │         ├─ recordSuccess(false)  // 不影响降级计数
    │    │         ├─ 调用 cache.Delete(key)
    │    │         └─ 返回 nil
    │    │
    │    └─ 保证：要么都删除，要么都保留
    │
    └─ WriteAround 模式
         │
         ├─ 1. 调用 storage.Delete(key)
         │    │
         │    ├─ 失败
         │    │    ├─ recordFailure(false)  // 不影响降级计数
         │    │    └─ 返回错误
         │    │
         │    └─ 成功
         │         ├─ recordSuccess(false)  // 不影响降级计数
         │         ├─ 调用 cache.Delete(key)
         │         └─ 返回 nil
         │
         └─ 保持与 WriteThrough 一致的行为
```

### 5.2 Delete 操作与降级决策

根据需求，降级仅由"底层存储连续写入失败"触发，因此：
- **Put 失败** → `recordFailure(true)` → 前台失败计数 +1 → 可能触发降级
- **Delete 失败** → `recordFailure(false)` → 不增加前台失败计数 → 不影响降级
- **Put 成功** → `recordSuccess(true)` → 前台失败计数清零 → 可能触发恢复
- **Delete 成功** → `recordSuccess(false)` → 不清零前台失败计数 → 不打断失败累积
- **后台重试成功** → `recordSuccess(false)` → 不清零前台失败计数
- **后台重试失败** → `recordFailure(false)` → 不增加前台失败计数

## 6. 状态流转机制

### 6.1 完整状态流转图

```
                        ┌─────────────────────────────┐
                        │      Write-Through          │
                        │        (正常模式)            │
                        │  - 同时写缓存和存储          │
                        │  - 失败计入降级计数          │
                        └───────────────┬─────────────┘
                                        │
                                        │ 条件：
                                        │   1. 前台 Put 连续失败
                                        │   2. 失败次数 >= DegradeThreshold
                                        │
                                        ▼
                        ┌─────────────────────────────┐
                        │      Write-Around           │
                        │        (降级模式)            │
                        │  - 只写存储，不写缓存        │
                        │  - 成功写入计入恢复计数      │
                        └───────────────┬─────────────┘
                                        │
                                        │ 条件：
                                        │   1. 时间窗口 RecoverWindow 内
                                        │   2. 成功写入次数 >= RecoverThreshold
                                        │
                                        ▼
                        ┌─────────────────────────────┐
                        │      Write-Through          │
                        │       (已恢复正常)           │
                        │  - 重置所有计数             │
                        └─────────────────────────────┘
```

### 6.2