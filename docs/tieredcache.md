# TieredCache 多级缓存架构模块

## 1. 模块概述

TieredCache 是一个高性能的多级缓存架构模块，实现了 L1 内存缓存 + L2 磁盘缓存的两级缓存架构。通过级联查询、智能写入策略和独立容量控制，在保证访问速度的同时提供了数据持久化能力，适用于需要权衡性能与存储容量的场景。

**包路径**: `internal/tieredcache`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| L1/L2 级联查询 | 查询时先检查 L1 内存缓存，未命中再查 L2 磁盘缓存，L2 命中后自动回填到 L1 |
| 双写模式（Write Through） | 写入时同时更新 L1 和 L2，保证数据一致性 |
| 回写模式（Write Back） | 写入时仅更新 L1 并标记为脏，异步刷入 L2，提升写入性能 |
| 独立容量控制 | L1 和 L2 各自维护独立的容量上限，支持条目数量或字节数两种计量方式 |
| LRU 淘汰策略 | 内置 LRU（最近最少使用）淘汰算法，缓存满时自动逐出最久未使用的条目 |
| 磁盘持久化 | L2 缓存数据持久化到磁盘，重启后可自动加载恢复 |
| 并发安全 | 所有操作均支持并发访问，通过读写锁保证线程安全 |

## 3. 核心结构体与职责

### 3.1 TieredCache

多级缓存主结构体，对外提供所有操作接口。

```go
type TieredCache struct {
    l1              *lruCache
    l2              *lruCache
    l2Dir           string
    writePolicy     WritePolicy
    mu              sync.RWMutex
    writeBackTicker *time.Ticker
    writeBackStop   chan struct{}
}
```

**职责**:
- 协调 L1 和 L2 两级缓存的交互
- 实现级联查询逻辑与数据回填
- 根据配置的写入策略执行写入操作
- 管理回写模式下的异步刷盘定时器
- 提供统一的并发访问控制

### 3.2 lruCache

LRU 缓存实现，作为 L1 和 L2 的底层存储引擎。

```go
type lruCache struct {
    items       map[string]*list.Element
    orderList   *list.List
    capacity    int64
    capacityMode CapacityMode
    count       int64
    totalBytes  int64
    mu          sync.RWMutex
    onEvict     func(*CacheEntry)
}
```

**职责**:
- 基于 `container/list` 实现 O(1) 复杂度的 LRU 算法
- 维护缓存条目，支持按容量或字节数限制
- 缓存满时自动执行淘汰策略
- 支持淘汰回调通知，用于清理磁盘文件或回写脏数据

### 3.3 CacheEntry

缓存条目结构体，存储单个键值对的元数据。

```go
type CacheEntry struct {
    Key       string
    Value     []byte
    Size      int
    Timestamp int64
    Dirty     bool
}
```

**职责**:
- 存储键值对数据及大小信息
- `Dirty` 标记用于回写模式，表示数据需要同步到磁盘
- `Timestamp` 记录创建/更新时间，用于调试和监控

### 3.4 Config

多级缓存配置结构体。

```go
type Config struct {
    L1Config          CacheLevelConfig
    L2Config          CacheLevelConfig
    WritePolicy       WritePolicy
    L2Dir             string
    WriteBackInterval time.Duration
}
```

### 3.5 CacheLevelConfig

单级缓存配置结构体。

```go
type CacheLevelConfig struct {
    Capacity       int64
    CapacityMode   CapacityMode
    EvictionPolicy EvictionPolicy
}
```

## 4. 核心常量与类型

### 4.1 写入策略（WritePolicy）

| 常量 | 描述 |
|------|------|
| `WritePolicyWriteThrough` | 双写模式：同时写入 L1 和 L2 |
| `WritePolicyWriteBack` | 回写模式：仅写入 L1，异步刷入 L2 |

### 4.2 容量模式（CapacityMode）

| 常量 | 描述 |
|------|------|
| `CapacityModeCount` | 按条目数量限制容量 |
| `CapacityModeBytes` | 按总字节数限制容量 |

### 4.3 淘汰策略（EvictionPolicy）

| 常量 | 描述 |
|------|------|
| `EvictionPolicyLRU` | LRU（最近最少使用）淘汰算法 |

### 4.4 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | L1 和 L2 中均未找到指定键 |
| `ErrInvalidCapacity` | 无效容量 | 配置容量小于等于 0 |
| `ErrInvalidPolicy` | 无效策略 | 配置了未支持的写入策略 |
| `ErrNilValue` | 值为空 | Put 操作传入 nil 值 |
| `ErrEmptyKey` | 键为空 | 操作传入空字符串键 |
| `ErrWriteBackFailed` | 回写失败 | 回写模式下超过最大重试次数后仍有永久失败条目 |

## 5. 数据流转路径

### 5.1 查询流程（Get）

```
Get(key) 流程:
  ┌──────────────────────────┐
  │ 1. 检查 L1 内存缓存      │
  │    l1.get(key)           │
  └──────────┬───────────────┘
             │
        ┌────┴────┐
        │  命中?  │
        └┬───────┬┘
         是       否
         ▼        ▼
      返回值  ┌─────────────────────┐
              │ 2. 检查 L2 缓存     │
              │    l2.get(key)      │
              └──────────┬──────────┘
                         │
                    ┌────┴────┐
                    │  命中?  │
                    └┬───────┬┘
                     是       否
                     ▼        ▼
              ┌────────────┐ 返回 ErrKeyNotFound
              │ 3. 回填 L1 │
              │  l1.put()  │
              └─────┬──────┘
                    ▼
                 返回值
```

**设计要点**:
- L2 命中后必须回填到 L1，加速后续访问
- 整个查询过程持有读锁，保证并发安全
- 回填操作在持有读锁的情况下进行（lruCache 内部有独立锁）

### 5.2 写入流程 - 双写模式（Write Through）

```
Put(key, value) [WriteThrough]:
  ┌──────────────────────────┐
  │ 1. 写入 L1 内存缓存      │
  │    l1.put(key, value)    │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 2. 写入 L2 缓存          │
  │    l2.put(key, value)    │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 3. 写入磁盘文件          │
  │    writeToL2()           │
  └──────────────────────────┘
```

**特点**:
- 数据同步写入内存和磁盘，一致性最高
- 写入性能受限于磁盘 I/O 速度
- 适用于数据一致性要求高的场景

### 5.3 写入流程 - 回写模式（Write Back）

```
Put(key, value) [WriteBack]:
  ┌──────────────────────────┐
  │ 1. 写入 L1 内存缓存      │
  │    l1.put(key, value)    │
  │    标记 Dirty = true     │
  └──────────────────────────┘

          异步定时刷盘:
  ┌──────────────────────────┐
  │ 遍历 L1 中所有 Dirty 条目│
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 写入 L2 缓存和磁盘文件   │
  │ 清除 Dirty 标记          │
  └──────────────────────────┘
```

**触发刷盘的时机**:
1. **定时刷盘**: 按 `WriteBackInterval` 配置的间隔自动刷盘（默认 5 秒）
2. **淘汰触发**: 当 L1 淘汰 Dirty 条目时，先刷盘再淘汰
3. **手动刷盘**: 调用 `Flush()` 方法立即刷盘
4. **关闭触发**: 调用 `Close()` 方法时自动刷盘

**特点**:
- 写入操作仅操作内存，性能极高
- 存在数据丢失风险（进程崩溃时未刷盘的 Dirty 数据会丢失）
- 适用于写入频繁、可容忍少量数据丢失的场景

### 5.4 淘汰流程

```
LRU 淘汰触发:
  ┌──────────────────────────┐
  │ 容量达到上限?            │
  └──────────┬───────────────┘
             │ 是
             ▼
  ┌──────────────────────────┐
  │ 选择最近最少使用的条目   │
  │ (orderList.Back())       │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 调用 onEvict 回调        │
  │  - L1: 若 Dirty 则刷盘   │
  │  - L2: 删除磁盘文件      │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 从缓存中移除条目         │
  └──────────────────────────┘
```

**淘汰策略特性**:
- L1 和 L2 独立配置容量和淘汰策略
- L1 淘汰 Dirty 条目时会自动写入 L2
- L2 淘汰时会删除对应的磁盘文件
- 淘汰回调通过异步队列执行，不阻塞缓存操作

### 5.5 删除流程（Delete）

```
Delete(key) 流程:
  ┌──────────────────────────┐
  │ 1. 清除 L1 Dirty 标记    │
  │    l1.clearDirty(key)    │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 2. 从 L1 缓存删除        │
  │    l1.delete(key)        │
  │    (条目入异步淘汰队列)   │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 3. 从 L2 缓存删除        │
  │    l2.delete(key)        │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 4. 删除磁盘文件          │
  │    deleteFromL2(key)     │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 异步: L1 淘汰回调执行    │
  │ handleL1Eviction(entry)  │
  │  → entry.Dirty == false  │
  │  → 跳过 writeToL2       │
  └──────────────────────────┘
```

**Delete 与异步淘汰回调的交互关系**:

关键设计：**先清除 Dirty 标记，再删除条目**。

| 步骤 | 操作 | 说明 |
|------|------|------|
| 1 | `l1.clearDirty(key)` | 在条目仍存在于 L1 时，将 Dirty 标记置为 false |
| 2 | `l1.delete(key)` | 从 L1 移除条目，放入异步淘汰队列 |
| 3 | `l2.delete(key)` | 从 L2 移除条目，放入异步淘汰队列 |
| 4 | `deleteFromL2(key)` | 立即删除磁盘文件 |
| 异步 | `handleL1Eviction(entry)` | 检查 Dirty==false，跳过 writeToL2 |

**为什么必须先清除 Dirty 标记**：

在回写模式下，L1 中的条目可能标记为 Dirty（尚未刷盘）。如果 Delete 直接删除 L1 条目而不先清除 Dirty：
1. `l1.delete(key)` 将 Dirty=true 的条目放入异步淘汰队列
2. `deleteFromL2(key)` 立即删除磁盘文件
3. 异步回调 `handleL1Eviction` 延迟执行，发现 Dirty=true，调用 `writeToL2` 将已删除的数据重新写回磁盘
4. 结果：磁盘文件永久残留，形成**数据泄漏**

通过先清除 Dirty 标记，异步回调执行时发现 `entry.Dirty == false`，直接跳过写入操作，避免数据泄漏。

**WriteBack 模式下 Delete 的数据安全保证**：
- 删除操作保证数据不会在磁盘上残留
- 异步回调与同步删除操作之间不存在竞争条件
- 无论异步回调何时执行，都不会重新写入已删除的数据

## 6. LRU 算法实现

### 6.1 数据结构

使用 `container/list` 维护访问顺序，`map` 提供 O(1) 查找：

```
items map[string]*list.Element
        │
        ▼
    key1 ────► elem ────► CacheEntry{key1, ...}
    key2 ────► elem ────► CacheEntry{key2, ...}
                           ▲
                           │
orderList list.List:  Head ──► elem(key3) ──► elem(key2) ──► elem(key1) ◄── Tail
                        最近使用                          最久未使用（淘汰候选）
```

### 6.2 核心操作

**Get 操作**:
1. 通过 map 查找元素（O(1)）
2. 将元素移到链表头部（标记为最近使用）
3. 返回值

**Put 操作**:
1. 若键已存在：更新值，移到链表头部
2. 若键不存在：
   - 创建新元素，插入链表头部
   - 检查容量，若已满则淘汰链表尾部元素

### 6.3 时间复杂度

| 操作 | 时间复杂度 |
|------|-----------|
| Get | O(1) |
| Put | O(1) |
| Delete | O(1) |
| Evict | O(1) |

## 7. 并发安全设计

### 7.1 锁层次结构

```
TieredCache.mu (RWMutex)
    ├─ l1.mu (RWMutex)
    └─ l2.mu (RWMutex)
```

### 7.2 锁策略

| 操作 | TieredCache 锁 | l1/l2 锁 | 说明 |
|------|---------------|----------|------|
| Get | RLock | **Lock** | **所有 lruCache.get() 使用写锁保护链表修改（MoveToFront） |
| Put | Lock | Lock | 写入需要加写锁 |
| Delete | Lock | Lock | 删除需要加写锁 |
| Clear | Lock | Lock | 清空需要加写锁 |
| Flush | Lock | Lock | 刷盘需要加写锁 |

### 7.3 LRU Get 操作的并发安全保证

**问题背景**：
`lruCache.get()` 需要调用 `container/list.MoveToFront()` 修改链表指针，而 `container/list` 本身不是并发安全的。即使外层使用读锁允许并发 Get，多个 `MoveToFront` 同时修改链表会形成数据竞争。

**修复方案**：
将 `lruCache.get()` 从 `RLock/RUnlock` 改为 `Lock/Unlock`，确保链表修改的独占访问。

```go
// 修复前：存在数据竞争
func (c *lruCache) get(key string) (*CacheEntry, bool) {
    c.mu.RLock()   // 读锁，多个 goroutine 可并行
    defer c.mu.RUnlock()
    if elem, ok := c.items[key]; ok {
        c.orderList.MoveToFront(elem)  // ❌ 并发修改链表，数据竞争
        ...
    }
}

// 修复后：无数据竞争
func (c *lruCache) get(key string) (*CacheEntry, bool) {
    c.mu.Lock()    // 写锁，独占访问
    defer c.mu.Unlock()
    if elem, ok := c.items[key]; ok {
        c.orderList.MoveToFront(elem)  // ✅ 安全的链表修改
        ...
    }
}
```

**性能权衡**：虽然 Get 操作使用写锁会降低读并发度，但这是保证数据正确性的必要代价。实际场景中，L1 缓存命中率通常较高，链表操作是纳秒级，性能影响可控。

### 7.4 死锁避免

- 严格按照 `TieredCache.mu` → `l1.mu` → `l2.mu` 的顺序加锁
- 永远不反向获取锁
- 回调函数（onEvict）通过异步队列执行，不在持锁时执行磁盘 I/O

### 7.5 淘汰回调的执行层级

#### 问题背景

原始实现中，淘汰回调（onEvict）在持有 lruCache 互斥锁时同步执行：
- 磁盘 I/O（os.Remove、os.WriteFile）在持锁状态下阻塞全部缓存读写操作
- 长时间持锁导致系统吞吐量急剧下降

#### 修复方案：异步回调队列

```
淘汰触发:
  ┌──────────────────────────┐
  │ 1. 持有锁，从缓存中│
  │    移除条目             │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ 2. 加入 evictQueue │
  │    (enqueueEvict())│
  └──────────┬───────────────┘
             │ 释放锁
             ▼
  ┌──────────────────────────┐
  │ 3. 独立后台协程     │
  │    processEvictQueue() │
  │    异步执行 onEvict │
  └──────────────────────────┘
```

**关键实现**：
- `evictQueue`: 淘汰条目的缓冲队列
- `evictQueueMu`: 队列的互斥保护
- `evictWg`: 待处理条目的 WaitGroup
- `processEvictQueue()`: 独立 goroutine 消费队列
- `waitEvictions()`: 等待所有排队的淘汰处理完成
- `Close()`: 关闭时标记并等待队列清空

**保证语义**：
- 缓存数据结构的修改与回调的磁盘 I/O 完全解耦
- 持锁时间从毫秒级降至纳秒级
- Close/Flush 时通过 `waitEvictions()` 确保回调完成

### 7.6 回写失败的处理策略

#### 问题背景

原始实现存在两个问题：
1. `flushWriteBack()` 对 `writeToL2` 错误用 `continue` 静默跳过，失败条目的 Dirty 标记未被清除，每次刷盘都重试注定失败的条目，形成死循环
2. `handleL1Eviction()` 对 `writeToL2` 错误直接 `return` 静默丢弃，调用方完全无法感知脏数据已丢失。两个触发持久化的路径对失败的处理方式不一致

#### 修复方案：统一失败追踪 + 有限重试

两条持久化路径现在使用统一的 `writeBackErrors` 计数器：

| 触发路径 | 失败处理 | 错误追踪 |
|----------|----------|----------|
| `flushWriteBack` | FailCount 递增，超过 `maxWriteBackRetries` 后清除 Dirty 并记录 | `writeBackErrors.Add(1)` |
| `handleL1Eviction` | 条目已从 L1 移除无法重试，直接记录 | `writeBackErrors.Add(1)` |

**flushWriteBack 失败处理流程**：

```
flushWriteBack 处理每个 Dirty 条目:
  ┌──────────────────────────┐
  │ 1. incrementFailCount()   │
  │    失败计数 +1        │
  └──────────┬───────────────┘
             │
        ┌────┴────────────┐
        │ FailCount > 3? │
        └┬───────────────┬┘
         是               否
         ▼                ▼
  ┌──────────────┐   ┌────────────────┐
  │ 清除 Dirty  │   │ 执行 writeToL2│
  │ 标记         │   └────────┬───────┘
  │ writeBackErrors │            │
  │ 计数器 +1   │       ┌────┴────┐
  └──────────────┘       │ 成功?    │
                           └┬────────┬┘
                            是        否
                            ▼         ▼
                     ┌───────────┐  保留 Dirty，
                     │ 写入 L2 │  下次重试
                     │ 清除 Dirty│
                     └───────────┘
```

**核心实现**：
```go
const maxWriteBackRetries = 3

type CacheEntry struct {
    Key       string
    Value     []byte
    Dirty     bool
    FailCount int  // 新增：失败计数
}

// 每次刷盘前先递增失败计数
failCount := tc.l1.incrementFailCount(entry.Key)
if failCount > maxWriteBackRetries {
    tc.l1.clearDirty(entry.Key)       // 清除 Dirty，停止重试
    tc.writeBackErrors.Add(1)            // 记录永久失败
    continue
}
```

**对外暴露**：
- `ErrWriteBackFailed`: 永久失败的错误类型
- `Flush()` 返回错误包含失败数量
- `WriteBackErrorCount()` 查询永久失败计数

### 7.7 磁盘恢复过程的数据安全

#### 问题背景

原始 `loadL2FromDisk()` 在 L2 容量不足时，逐条加载触发淘汰回调，回调中 `os.Remove` 将磁盘文件永久删除，导致重启恢复过程反而丢失了比重启前更多的持久化数据。

#### 修复方案：加载时禁用淘汰删除

使用 `putWithoutEvictCallback()` 方法：

```go
// 加载时使用的特殊 put 方法：
// - 容量不足时不触发淘汰
// - 不执行任何磁盘删除回调
// - 返回 (nil, false) 表示容量不足，跳过该条目
func (c *lruCache) putWithoutEvictCallback(key string, value []byte) (*CacheEntry, bool)
```

**加载流程保证**：

```
loadL2FromDisk:
  ┌──────────────────────────────┐
  │ 遍历磁盘文件              │
  └──────────┬───────────────┘
             ▼
  ┌──────────────────────────┐
  │ putWithoutEvictCallback│
  │  容量足够?            │
  └┬──────────────────┬┘
    是                  否
    ▼                   ▼
  加载到内存         跳过加载，
  （不删除磁盘文件   保留磁盘文件
```

**数据安全保证**：
- 磁盘文件在任何情况下都不会被加载过程删除
- L2 容量不足只影响内存中可加载的条目数量
- 未加载到内存的文件仍在磁盘上，后续 LRU 淘汰腾出空间后仍可访问

### 7.8 并发一致性保证

| 场景 | 一致性保证 |
|------|-----------|
| 并发 Get | 写锁保护链表，无数据竞争 |
| 并发 Get + Put | TieredCache 写锁阻塞所有读 |
| 淘汰回调 | 异步队列执行，不阻塞缓存操作 |
| 回写失败 | 统一 writeBackErrors 计数器，有限重试后永久失败标记 |
| 重启加载 | 不删除磁盘文件，保证数据安全 |
| Delete + 异步回调 | 先清除 Dirty 标记再删除，避免异步回调重新写入已删除数据 |

## 8. 使用示例

### 8.1 基本使用 - 双写模式

```go
package main

import (
    "fmt"
    "solocoder-go/internal/tieredcache"
)

func main() {
    // 使用默认配置创建（双写模式）
    tc, err := tieredcache.NewTieredCache()
    if err != nil {
        panic(err)
    }
    defer tc.Close()

    // 写入数据
    err = tc.Put("user:1:name", []byte("Alice"))
    if err != nil {
        panic(err)
    }

    // 读取数据
    val, err := tc.Get("user:1:name")
    if err != nil {
        panic(err)
    }
    fmt.Println("Name:", string(val))

    // 删除数据
    err = tc.Delete("user:1:name")
    if err != nil {
        panic(err)
    }

    // 获取统计信息
    fmt.Println("L1 count:", tc.L1Count())
    fmt.Println("L2 count:", tc.L2Count())
}
```

### 8.2 自定义配置 - 回写模式

```go
import "time"

cfg := tieredcache.Config{
    L1Config: tieredcache.CacheLevelConfig{
        Capacity:       1000,
        CapacityMode:   tieredcache.CapacityModeCount,
        EvictionPolicy: tieredcache.EvictionPolicyLRU,
    },
    L2Config: tieredcache.CacheLevelConfig{
        Capacity:       100 * 1024 * 1024, // 100MB
        CapacityMode:   tieredcache.CapacityModeBytes,
        EvictionPolicy: tieredcache.EvictionPolicyLRU,
    },
    WritePolicy:       tieredcache.WritePolicyWriteBack,
    L2Dir:             "/var/cache/tiered",
    WriteBackInterval: 10 * time.Second,
}

tc, err := tieredcache.NewTieredCacheWithConfig(cfg)
if err != nil {
    panic(err)
}
defer tc.Close()
```

### 8.3 容量模式 - 字节数限制

```go
cfg := tieredcache.Config{
    L1Config: tieredcache.CacheLevelConfig{
        Capacity:       64 * 1024 * 1024, // 64MB
        CapacityMode:   tieredcache.CapacityModeBytes,
        EvictionPolicy: tieredcache.EvictionPolicyLRU,
    },
    L2Config: tieredcache.CacheLevelConfig{
        Capacity:       1024 * 1024 * 1024, // 1GB
        CapacityMode:   tieredcache.CapacityModeBytes,
        EvictionPolicy: tieredcache.EvictionPolicyLRU,
    },
    // ...
}
```

### 8.4 手动刷盘（回写模式）

```go
// 写入大量数据
for i := 0; i < 10000; i++ {
    key := fmt.Sprintf("key:%d", i)
    val := []byte(fmt.Sprintf("value:%d", i))
    tc.Put(key, val)
}

// 强制刷盘，确保数据持久化
err := tc.Flush()
if err != nil {
    panic(err)
}
fmt.Println("All dirty data flushed to disk")
```

### 8.5 并发使用

```go
var wg sync.WaitGroup

// 10 个 goroutine 并发写入
for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for j := 0; j < 1000; j++ {
            key := fmt.Sprintf("worker:%d:%d", id, j)
            val := []byte(fmt.Sprintf("value:%d", j))
            tc.Put(key, val)
        }
    }(i)
}

// 20 个 goroutine 并发读取
for i := 0; i < 20; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for j := 0; j < 500; j++ {
            key := fmt.Sprintf("worker:%d:%d", id%10, j)
            val, err := tc.Get(key)
            if err == nil {
                // process val
            }
        }
    }(i)
}

wg.Wait()
```

### 8.6 清空缓存

```go
// 清空所有缓存数据（包括磁盘文件）
err := tc.Clear()
if err != nil {
    panic(err)
}

fmt.Println("L1 count after clear:", tc.L1Count()) // 0
fmt.Println("L2 count after clear:", tc.L2Count()) // 0
```

## 9. 性能特征

### 9.1 访问性能

| 场景 | 延迟 | 说明 |
|------|------|------|
| L1 命中 | 极快（纯内存） | O(1) 哈希查找 + 链表移动 |
| L2 命中 | 中等（内存 + 回填） | L1 miss + L2 hit + L1 put |
| L2 miss | 慢（磁盘 I/O） | L1 miss + L2 miss + 磁盘查找失败 |
| 双写模式写入 | 慢（磁盘 I/O） | L1 put + L2 put + 磁盘写入 |
| 回写模式写入 | 极快（纯内存） | 仅 L1 put + Dirty 标记 |

### 9.2 并发性能

- **读-读并发**: 完全并行，L1 和 L2 均支持并发读
- **读-写并发**: 写操作阻塞读操作（TieredCache 级别的写锁）
- **写-写并发**: 串行化执行

### 9.3 内存开销

- 每个条目额外开销约 100-150 字节（map 项 + list 元素 + CacheEntry 结构体）
- 100 万条目约消耗 100-150MB 额外内存
- 建议根据实际数据大小合理配置容量

## 10. 注意事项与限制

1. **数据序列化**: 值必须是 `[]byte` 类型，使用方需自行处理序列化/反序列化
2. **回写模式数据丢失风险**: 进程崩溃时，未刷盘的 Dirty 数据会丢失
3. **磁盘空间**: L2 缓存持久化到磁盘，需确保磁盘空间充足
4. **键名限制**: 键名会被转换为安全的文件名，特殊字符会被转义
5. **容量模式选择**: 
   - 小对象建议使用 `CapacityModeCount`
   - 大对象建议使用 `CapacityModeBytes`
6. **关闭缓存**: 必须调用 `Close()` 方法释放资源，尤其是回写模式下确保数据刷盘
7. **并发写入**: 虽然支持并发，但大量并发写入在双写模式下会受限于磁盘 I/O

## 11. 默认配置

```go
func DefaultConfig() Config {
    return Config{
        L1Config: CacheLevelConfig{
            Capacity:       1000,
            CapacityMode:   CapacityModeCount,
            EvictionPolicy: EvictionPolicyLRU,
        },
        L2Config: CacheLevelConfig{
            Capacity:       10000,
            CapacityMode:   CapacityModeCount,
            EvictionPolicy: EvictionPolicyLRU,
        },
        WritePolicy:       WritePolicyWriteThrough,
        L2Dir:             filepath.Join(os.TempDir(), "tieredcache"),
        WriteBackInterval: 5 * time.Second,
    }
}
```
