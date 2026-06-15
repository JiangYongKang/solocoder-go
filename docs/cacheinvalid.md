# CacheInvalid 缓存失效管理器模块

## 1. 模块概述

CacheInvalid 是一个功能完善的缓存失效管理模块，提供了多种缓存失效策略和数据一致性保证机制。模块支持基于 TTL 的惰性过期、基于事件的通知失效、缓存预加载以及热点数据永不过期标记等核心功能，适用于需要高性能缓存且对数据一致性有要求的场景。

**包路径**: `internal/cacheinvalid`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| TTL 惰性过期 | 为每个缓存条目设置生存时间，访问时检查并惰性删除过期条目，无需后台线程 |
| 事件通知失效 | 支持注册失效事件监听器，数据变更时发送失效事件主动删除对应缓存 |
| 缓存预加载 | 支持系统启动或指定时机预加载热点数据，跳过 TTL 检查减少冷启动穿透 |
| 热点数据标记 | 支持将频繁访问的数据标记为永不过期，仅在显式失效或手动清除时移除 |
| 自动热点识别 | 访问次数超过阈值的条目自动标记为热点数据 |
| 容量限制 | 支持最大缓存条目数配置，超出时自动淘汰非热点条目 |
| 并发安全 | 基于读写锁的并发安全设计，支持高并发访问 |

## 3. 核心结构体与职责

### 3.1 CacheInvalidManager

缓存失效管理器主结构体，对外提供所有操作接口。

```go
type CacheInvalidManager struct {
    mu           sync.RWMutex
    entries      map[string]*CacheEntry
    config       Config
    listeners    map[string][]*listenerEntry
    listenerMap  map[string]*listenerEntry
    listenerEventType map[string]string
    nextListenerID uint64
    idMu         sync.Mutex
    preloadLoader PreloadLoader
}
```

**职责**:
- 管理所有缓存条目，提供 CRUD 操作
- 维护 TTL 过期检查与惰性删除逻辑
- 管理事件监听器的注册、移除与事件发布
- 处理缓存预加载逻辑
- 维护热点数据标记与自动识别
- 协调容量限制与条目淘汰

### 3.2 CacheEntry

缓存条目结构体，存储单个缓存项的完整信息。

```go
type CacheEntry struct {
    Key         string
    Value       interface{}
    ExpiresAt   time.Time
    TTL         time.Duration
    IsHot       atomic.Bool
    IsPreloaded bool
    CreateTime  time.Time
    AccessCount atomic.Int64
}
```

**职责**:
- 存储缓存键值对数据
- 记录过期时间与 TTL 配置
- 标记是否为热点数据（原子类型，支持读锁下并发更新）
- 标记是否为预加载数据
- 统计访问次数用于自动热点识别（原子类型，支持读锁下并发更新）
- 记录创建时间用于 FIFO 淘汰

> **注意**: `IsHot` 和 `AccessCount` 使用 `sync/atomic` 原子类型，目的是支持在读锁保护下进行并发更新，充分发挥 `sync.RWMutex` 的并发读优势。

### 3.3 InvalidationEvent

缓存失效事件结构体，封装失效通知的完整信息。

```go
type InvalidationEvent struct {
    Key       string
    EventType string
    Payload   interface{}
    Timestamp time.Time
}
```

**职责**:
- 携带失效缓存的键信息
- 标识事件类型（如 "invalidate"、"data.updated" 等）
- 可选携带事件负载数据
- 记录事件发生时间戳

### 3.4 Config

缓存失效管理器配置结构体。

```go
type Config struct {
    DefaultTTL         time.Duration
    MaxEntries         int
    HotAccessThreshold int64
    PreloadSize        int
    PreloadOnStart     bool
}
```

**配置项说明**:
- `DefaultTTL`: 默认缓存生存时间，默认 5 分钟
- `MaxEntries`: 最大缓存条目数，默认 10000
- `HotAccessThreshold`: 热点访问阈值，超过此访问次数自动标记为热点，默认 100
- `PreloadSize`: 预加载数据量上限，默认 100
- `PreloadOnStart`: 是否在启动时自动预加载，默认 false

### 3.5 CacheItem

预加载缓存项结构体，用于预加载数据源返回。

```go
type CacheItem struct {
    Key   string
    Value interface{}
    IsHot bool
}
```

**职责**:
- 定义预加载数据的标准格式
- 支持指定预加载数据是否为热点数据

### 3.6 listenerEntry

内部监听器条目结构体。

```go
type listenerEntry struct {
    id       string
    listener InvalidationListener
}
```

**职责**:
- 内部维护监听器 ID 与函数的对应关系
- 支持通过 ID 精确移除监听器

## 4. 缓存条目生命周期

### 4.1 完整生命周期流程

```
              创建
               │
               ▼
          ┌─────────┐
          │  新建    │ Put/PutWithTTL
          │  条目    │
          └────┬────┘
               │
               ▼
          ┌─────────┐
          │  正常    │ Get 时检查 TTL
          │  缓存中  │ 未过期则返回
          └────┬────┘
               │
       ┌───────┴───────┐
       ▼               ▼
  ┌─────────┐     ┌─────────┐
  │ TTL 过期│     │  访问超  │
  │ 惰性删除│     │  过阈值  │ 自动标记
  └────┬────┘     └────┬────┘
       │               │
       │               ▼
       │          ┌─────────┐
       │          │  热点    │ 不受 TTL 控制
       │          │  数据    │
       │          └────┬────┘
       │               │
       └───────┬───────┘
               ▼
          ┌─────────┐
          │  失效/   │ 事件触发、
          │  删除    │ 手动删除
          └─────────┘
               │
               ▼
              销毁
```

### 4.2 生命周期阶段说明

**1. 创建阶段**
- 通过 `Put` 或 `PutWithTTL` 方法创建缓存条目
- 设置初始 TTL、过期时间、创建时间等属性
- 新条目默认为非热点、非预加载状态
- 如达到容量上限，先淘汰一条非热点条目

**2. 正常缓存阶段**
- 条目处于正常缓存状态，受 TTL 控制
- 每次 `Get` 访问时：
  - 检查是否过期，过期则惰性删除
  - 访问计数加 1
  - 如达到热点阈值，自动标记为热点数据

**3. 热点数据阶段**
- 可通过 `MarkHot` 手动标记或访问自动升级
- 不受 TTL 过期控制，永不过期
- 容量淘汰时受到保护，不会被自动淘汰
- 可通过 `UnmarkHot` 取消热点标记，恢复正常 TTL 控制

**4. 预加载数据阶段**
- 通过 `Preload` 方法从预加载加载器加载数据
- 预加载数据不受 TTL 控制，永不过期
- 可设置为热点或非热点状态
- 可通过 `UnmarkPreloaded` 取消预加载标记，恢复正常 TTL 控制

**5. 失效阶段**
- **TTL 惰性失效**: `Get` 时检测到过期自动删除
- **事件触发失效**: 收到失效事件后主动删除
- **手动失效**: 调用 `Delete`、`Invalidate` 等方法
- **容量淘汰**: 达到最大条目数时自动淘汰最旧的非热点条目

## 5. TTL 惰性过期机制

### 5.1 设计原理

惰性过期（Lazy Expiration）是一种权衡性能与内存的过期策略：

- **不启动后台扫描线程**: 避免额外的 CPU 和线程开销
- **访问时检查**: 仅在 `Get` 操作时检查条目是否过期
- **过期即删除**: 发现过期立即从缓存中移除

### 5.2 优缺点

**优点**:
- 无后台线程，资源消耗低
- 实现简单，易于维护
- 对未访问的过期数据不产生处理开销

**缺点**:
- 过期但未访问的数据仍占用内存
- 第一次访问过期数据时会有微小延迟（删除操作）

### 5.3 Get 操作流程

```
Get(key)
  │
  ├─► 查找条目
  │     │
  │     ├─ 不存在 ──► 返回 (nil, false)
  │     │
  │     └─ 存在 ────► 检查状态
  │                       │
  │                       ├─ 热点/预加载 ──► 访问计数++，返回值
  │                       │
  │                       └─ 普通条目 ────► 检查过期
  │                                         │
  │                                         ├─ 已过期 ──► 删除条目，返回 (nil, false)
  │                                         │
  │                                         └─ 未过期 ──► 访问计数++，返回值
  │
  └─► 返回结果
```

### 5.4 主动清理

除了惰性过期外，模块还提供 `CleanupExpired()` 方法支持主动清理所有过期条目：
- 遍历所有条目，删除已过期的非热点、非预加载条目
- 返回清理的条目数量
- 可在系统空闲时定期调用以释放内存

## 6. 事件通知失效机制

### 6.1 设计原理

基于事件的通知失效是一种主动缓存一致性策略：

- 底层数据变更时发送失效事件
- 缓存管理器收到事件后主动删除对应缓存
- 确保缓存与数据源的最终一致性

### 6.2 核心组件

**失效监听器 (InvalidationListener)**:
- 类型为 `func(event InvalidationEvent)`
- 接收失效事件并执行相应处理

**事件类型 (EventType)**:
- 字符串类型，支持自定义事件类型
- 如 "invalidate"、"data.updated"、"data.deleted" 等
- 监听器按事件类型订阅

### 6.3 事件发布流程

```
数据变更
    │
    ▼
发布失效事件 ──► 遍历该类型所有监听器
                     │
                     ├─ 监听器1 ──► 执行处理（含 panic 恢复）
                     ├─ 监听器2 ──► 执行处理（含 panic 恢复）
                     └─ ...
```

### 6.4 主动失效方法

- `Invalidate(key)`: 失效指定键并发布 "invalidate" 事件
- `InvalidateWithEvent(key, eventType, payload)`: 失效指定键并发布自定义事件
- `PublishEvent(event)`: 仅发布事件，不删除缓存

### 6.5 Panic 恢复

事件发布器内置 panic 恢复机制：
- 单个监听器 panic 不会影响其他监听器执行
- 使用 defer + recover 保护每个监听器调用
- 确保事件发布的健壮性

## 7. 缓存预加载机制

### 7.1 设计原理

缓存预加载是一种冷启动优化策略：

- 系统启动或指定时机预先加载热点数据
- 预加载数据跳过 TTL 检查，永不过期
- 减少冷启动时的缓存穿透问题

### 7.2 预加载加载器 (PreloadLoader)

```go
type PreloadLoader func() ([]CacheItem, error)
```

- 由调用方实现具体的数据加载逻辑
- 返回 `[]CacheItem` 数组，每项包含键、值和是否热点标记
- 可从数据库、配置文件、远程服务等来源加载数据

### 7.3 预加载流程

```
Preload()
    │
    ▼
检查 loader 是否设置
    │
    ├─ 未设置 ──► 返回 ErrNilLoader
    │
    └─ 已设置 ──► 调用 loader 加载数据
                      │
                      ├─ 加载失败 ──► 返回错误
                      │
                      └─ 加载成功 ──► 遍历数据项
                                          │
                                          ├─ 达到 PreloadSize 限制 ──► 停止
                                          │
                                          ├─ 达到 MaxEntries 限制 ───► 停止
                                          │
                                          └─ 正常 ──► 创建预加载条目
                                                       IsPreloaded = true
                                                       不受 TTL 限制
```

### 7.4 预加载数据特性

- **跳过 TTL 检查**: 预加载数据永不过期
- **可配置数量**: 通过 `PreloadSize` 限制预加载数量
- **支持热点标记**: 预加载时可指定是否为热点数据
- **可取消预加载标记**: 调用 `UnmarkPreloaded` 恢复正常 TTL 控制

## 8. 热点数据标记机制

### 8.1 设计原理

热点数据永不过期机制是一种性能优化策略：

- 频繁访问的热点数据不受 TTL 控制
- 减少缓存失效导致的回源压力
- 保证高频访问数据的响应速度

### 8.2 热点标记方式

**1. 手动标记**
- `MarkHot(key)`: 手动将指定键标记为热点
- `UnmarkHot(key)`: 取消热点标记

**2. 自动识别**
- 每次 `Get` 访问时增加访问计数
- 访问计数达到 `HotAccessThreshold` 阈值时自动标记为热点
- 阈值可通过配置调整，默认 100 次

### 8.3 热点数据特性

- **永不过期**: 不受 TTL 控制，不会因过期被删除
- **淘汰保护**: 容量满时不会被自动淘汰
- **可手动失效**: 仍可通过 `Delete`、`Invalidate` 等方法手动删除
- **可取消标记**: 取消后恢复正常 TTL 控制

### 8.4 与预加载的关系

热点标记和预加载标记是两个独立的属性：
- 预加载数据可以是热点或非热点
- 热点数据可以是预加载或正常添加的
- 两者都能让数据跳过 TTL 检查
- 取消预加载标记后，如同时是热点数据，仍不会过期

## 9. 容量管理与淘汰策略

### 9.1 容量限制

通过 `MaxEntries` 配置最大缓存条目数：
- 新增条目时如已达上限，先淘汰一条
- 保护热点数据和预加载数据不被淘汰
- 采用 FIFO 策略淘汰最旧的非保护条目

### 9.2 淘汰算法

```
evictOne()
    │
    ▼
遍历所有条目，寻找可淘汰目标
    │
    ├─ 跳过热点数据
    ├─ 跳过预加载数据
    └─ 选择创建时间最早的条目
         │
         ├─ 找到 ──► 删除该条目
         │
         └─ 未找到（全是热点/预加载）
                  └─► 删除任意一条（极端情况）
```

### 9.3 淘汰保护优先级

1. **最高优先级**: 既是热点又是预加载的数据
2. **高优先级**: 热点数据
3. **高优先级**: 预加载数据
4. **普通优先级**: 正常缓存条目

## 10. 使用示例

### 10.1 基本使用

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/cacheinvalid"
)

func main() {
    // 创建缓存管理器
    cache := cacheinvalid.NewCacheInvalidManager()

    // 写入数据（使用默认 TTL）
    cache.Put("user:1", "Alice")

    // 写入数据（指定 TTL）
    cache.PutWithTTL("user:2", "Bob", 10*time.Minute)

    // 读取数据
    if value, ok := cache.Get("user:1"); ok {
        fmt.Println("User 1:", value)
    }

    // 删除数据
    cache.Delete("user:2")

    // 获取缓存数量
    fmt.Println("Cache count:", cache.Count())
}
```

### 10.2 自定义配置

```go
cfg := cacheinvalid.Config{
    DefaultTTL:         10 * time.Minute,
    MaxEntries:         5000,
    HotAccessThreshold: 50,
    PreloadSize:        200,
}
cache := cacheinvalid.NewCacheInvalidManagerWithConfig(cfg)
```

### 10.3 TTL 惰性过期

```go
cache := cacheinvalid.NewCacheInvalidManager()

// 设置短 TTL 的数据
cache.PutWithTTL("temp:1", "temporary data", 5*time.Second)

// 5 秒内访问正常
value, ok := cache.Get("temp:1")
fmt.Println("Found:", ok, "Value:", value)

// 等待过期
time.Sleep(6 * time.Second)

// 过期后访问返回不存在（惰性删除）
value, ok = cache.Get("temp:1")
fmt.Println("Found after expire:", ok) // false

// 检查是否过期
expired, err := cache.IsExpired("temp:1")
if err != nil {
    fmt.Println("Key not found")
}
```

### 10.4 事件通知失效

```go
cache := cacheinvalid.NewCacheInvalidManager()

// 注册失效事件监听器
listenerID, err := cache.AddListener("data.updated", func(event cacheinvalid.InvalidationEvent) {
    fmt.Printf("Cache invalidated: key=%s, type=%s\n", event.Key, event.EventType)
})
if err != nil {
    // 处理错误
}

// 写入缓存
cache.Put("product:123", "product data")

// 数据变更时主动失效并发布事件
cache.InvalidateWithEvent("product:123", "data.updated", "new product data")

// 移除监听器
cache.RemoveListener(listenerID)
```

### 10.5 缓存预加载

```go
cache := cacheinvalid.NewCacheInvalidManager()

// 设置预加载加载器
loader := func() ([]cacheinvalid.CacheItem, error) {
    // 从数据库或其他来源加载热点数据
    items := []cacheinvalid.CacheItem{
        {Key: "config:theme", Value: "dark", IsHot: true},
        {Key: "config:language", Value: "zh-CN", IsHot: true},
        {Key: "stats:total_users", Value: 10000, IsHot: false},
    }
    return items, nil
}

cache.SetPreloadLoader(loader)

// 执行预加载
if err := cache.Preload(); err != nil {
    fmt.Println("Preload failed:", err)
}

// 预加载数据永不过期
fmt.Println("Preloaded count:", cache.PreloadedCount())

// 取消预加载标记（恢复正常 TTL）
cache.UnmarkPreloaded("stats:total_users")
```

### 10.6 热点数据标记

```go
cache := cacheinvalid.NewCacheInvalidManager()

// 写入数据
cache.PutWithTTL("hot:page", "page content", 1*time.Minute)

// 手动标记为热点（永不过期）
cache.MarkHot("hot:page")

// 检查是否为热点
isHot, _ := cache.IsHot("hot:page")
fmt.Println("Is hot:", isHot) // true

// 等待超过 TTL
time.Sleep(2 * time.Minute)

// 热点数据仍然存在
value, ok := cache.Get("hot:page")
fmt.Println("Found after TTL:", ok) // true

// 取消热点标记
cache.UnmarkHot("hot:page")

// 获取热点数量
fmt.Println("Hot count:", cache.HotCount())
```

### 10.7 自动热点识别

```go
cfg := cacheinvalid.Config{
    DefaultTTL:         5 * time.Minute,
    HotAccessThreshold: 10, // 访问 10 次自动标记为热点
}
cache := cacheinvalid.NewCacheInvalidManagerWithConfig(cfg)

cache.Put("popular:item", "popular data")

// 多次访问，触发自动热点标记
for i := 0; i < 10; i++ {
    cache.Get("popular:item")
}

// 已自动标记为热点
isHot, _ := cache.IsHot("popular:item")
fmt.Println("Auto hot:", isHot) // true
```

### 10.8 主动清理过期数据

```go
cache := cacheinvalid.NewCacheInvalidManager()

// 添加一些带 TTL 的数据
cache.PutWithTTL("key1", "value1", 1*time.Second)
cache.PutWithTTL("key2", "value2", 1*time.Second)
cache.Put("key3", "value3") // 使用默认 TTL

time.Sleep(2 * time.Second)

// 主动清理所有过期数据
cleanedCount := cache.CleanupExpired()
fmt.Printf("Cleaned %d expired entries\n", cleanedCount)
```

### 10.9 并发使用

```go
var wg sync.WaitGroup
cache := cacheinvalid.NewCacheInvalidManager()

// 10 个 goroutine 并发写入和读取
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for j := 0; j < 100; j++ {
            key := fmt.Sprintf("key-%d-%d", id, j)
            cache.Put(key, "value")
            cache.Get(key)
        }
    }(i)
}

wg.Wait()
fmt.Println("Total entries:", cache.Count())
```

## 11. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | 对不存在的键执行操作时 |
| `ErrNilLoader` | 预加载加载器为空 | 未设置 loader 就调用 Preload |
| `ErrInvalidTTL` | TTL 无效 | PutWithTTL 传入负的 TTL |
| `ErrListenerNotFound` | 监听器不存在 | 移除不存在的监听器 ID |
| `ErrInvalidPreloadSize` | 预加载大小无效 | 预加载大小配置为负 |

## 12. 性能与并发

### 12.1 并发安全

- 使用 `sync.RWMutex` 保护共享数据
- 读操作（Get、Count、IsExpired 等）使用读锁
- 写操作（Put、Delete、MarkHot 等）使用写锁
- 事件发布在锁外执行，避免长时间持有锁

### 12.2 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Put | O(1) | 哈希表写入 + 可能的淘汰遍历 |
| Get | O(1) | 哈希表查找 + TTL 检查 |
| Delete | O(1) | 哈希表删除 |
| MarkHot | O(1) | 哈希表查找 + 标记 |
| Preload | O(n) | n 为预加载条目数 |
| CleanupExpired | O(n) | n 为总条目数 |
| PublishEvent | O(k) | k 为该类型监听器数量 |

### 12.3 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **惰性过期内存占用**: 长期不访问的过期数据会占用内存，建议定期调用 CleanupExpired
3. **热点数据膨胀**: 如大量数据被标记为热点，可能导致内存持续增长，需合理设置热点阈值
4. **预加载数据管理**: 预加载数据不会自动过期，需根据业务需要手动取消预加载标记
5. **单线程事件发布**: 事件发布为同步调用，监听器应避免耗时操作
6. **容量淘汰策略简单**: FIFO 淘汰策略相对简单，复杂场景可考虑扩展为 LRU
7. **受保护条目驱逐边界**: 当缓存中所有条目都是热点或预加载数据时，新的写入会返回 `ErrCapacityExhausted` 错误，不会强制驱逐受保护条目。需要合理控制热点数据比例，避免容量全部被受保护数据占满。
8. **高并发读优化**: Get 方法采用读锁 + 原子操作设计，高并发读场景下性能优异。但写入操作仍需写锁，写密集场景需评估对读性能的影响。

## 13. 架构设计考虑

### 13.1 设计权衡

**TTL 惰性过期 vs 定时扫描**:
- 选择惰性过期：实现简单、无后台开销
- 代价：过期数据可能暂用内存，首次访问有删除开销
- 补充：提供 CleanupExpired 主动清理方法

**事件同步 vs 异步发布**:
- 选择同步发布：实现简单、顺序可预测
- 代价：监听器耗时会影响发布性能
- 保护：内置 panic 恢复，单个监听器不影响整体

**热点自动识别阈值**:
- 选择访问计数方式：实现简单、开销低
- 代价：不能精确反映时间局部性（可能很久以前的热点）
- 可扩展：未来可增加滑动窗口等更复杂的热点识别算法

### 13.2 可扩展点

1. **淘汰策略扩展**: 可替换为 LRU、LFU 等更复杂的淘汰算法
2. **持久化支持**: 可增加持久化层，支持缓存数据落盘
3. **统计监控**: 可增加命中率、过期率等统计指标
4. **多级缓存**: 可与本地缓存、分布式缓存结合形成多级缓存
5. **批量操作**: 可增加批量失效、批量预加载等操作
6. **事件过滤**: 可增加事件过滤机制，支持更细粒度的订阅

## 14. 典型应用场景

### 14.1 配置缓存

使用预加载 + 热点标记：
- 系统启动时预加载所有配置
- 常用配置标记为热点，永不过期
- 配置变更时通过事件通知失效

### 14.2 用户会话缓存

使用 TTL 过期 + 自动热点：
- 用户登录后写入会话缓存，设置 TTL
- 活跃用户自动升级为热点，延长会话有效期
- 用户登出或会话过期自动清理

### 14.3 商品详情缓存

使用事件驱动失效：
- 商品详情缓存，设置适中 TTL
- 商品信息更新时发布更新事件
- 缓存收到事件后主动失效对应缓存
- 保证用户看到的商品信息及时更新

### 14.4 热点数据防护

使用热点标记 + 容量限制：
- 识别高频访问的热点数据
- 标记为永不过期，避免缓存击穿
- 容量满时保护热点数据不被淘汰
- 保障核心接口的性能稳定性
