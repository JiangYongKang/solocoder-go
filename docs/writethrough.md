# Write-Through 缓存策略模块

## 1. 模块概述

Write-Through（写穿透）缓存策略模块是一个高性能、高可用的缓存抽象层，提供缓存与底层存储的同步写入、失败重试、自动降级恢复以及读穿透缓存回填等功能。模块设计用于需要数据强一致性和高可用性的缓存场景。

**包路径**: `internal/writethrough`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Write-Through 写入 | 写操作同时写入缓存和底层存储，两者都成功才返回成功 |
| 写入失败重试 | 底层存储写入失败时自动重试，重试次数和间隔可配置 |
| 待处理队列 | 写入失败的数据暂存于缓存，后台异步重试 |
| Write-Around 降级 | 连续失败超过阈值时自动降级为 Write-Around 模式 |
| 自动恢复 | 连续成功达到阈值时自动恢复为 Write-Through 模式 |
| 读穿透 + 缓存回填 | 缓存未命中时从底层存储读取，数据自动回填到缓存 |
| Delete 操作 | 支持删除操作，遵循当前写入策略 |

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
    DegradeThreshold  int           // 降级阈值（连续失败次数）
    RecoverThreshold  int           // 恢复阈值（连续成功次数）
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
    ├─→ 1. 写入缓存（立即成功）
    │
    ├─→ 2. 写入底层存储
    │       │
    │       ├─ 成功 → 记录成功，返回 nil
    │       │
    │       └─ 失败 → 重试（最多 MaxRetries 次）
    │                │
    │                ├─ 重试成功 → 记录成功，返回 nil
    │                │
    │                └─ 重试失败 → 加入待处理队列
    │                                 ├─ 记录失败
    │                                 ├─ 检查是否需要降级
