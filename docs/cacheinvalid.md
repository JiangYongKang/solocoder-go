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
    IsHot       bool
    IsPreloaded bool
    CreateTime  time.Time
    AccessCount int64
}
```

**职责**:
- 存储缓存键值对数据
- 记录过期时间与 TTL 配置
- 标记是否为热点数据
- 标记是否为预加载数据
- 统计访问次数用于自动热点识别
- 记录创建时间用于 FIFO 淘汰

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
