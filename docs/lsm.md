# LSM-Tree 存储引擎模块

## 1. 模块概述

LSM-Tree（Log-Structured Merge-Tree）是一种高性能的持久化键值存储引擎，专为写入密集型场景设计。模块通过将写入先缓冲在内存（MemTable）中，达到阈值后批量刷盘生成 SSTable（Sorted String Table）文件，并通过后台多层合并（Compaction）机制清理冗余数据，实现了高吞吐写入与可接受的读取性能之间的平衡。

**包路径**: `internal/lsm`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| MemTable 写入 | 使用跳表（SkipList）有序存储键值对，支持插入、更新、删除操作 |
| SSTable 刷盘 | MemTable 达到阈值后冻结并排序，异步写入磁盘生成带索引的 SSTable 文件 |
| 多层合并压缩 | SSTable 按层（Level 0 ~ Level N）组织，后台自动将上层文件与下层重叠文件合并，丢弃删除标记和旧版本数据 |
| 点查询（Get） | 先搜索内存结构（MemTable → Immutable → Flushing），再按层搜索 SSTable |
| 范围查询（Range） | 按起始键和结束键扫描，合并多层结果并按时间戳取最新版本 |
| Tombstone 删除标记 | 删除操作写入特殊墓碑标记，合并时真正物理删除 |
| 数据持久化与恢复 | 启动时自动从磁盘加载已有 SSTable 文件 |
| 并发安全 | 多 goroutine 安全写入、读取、范围扫描 |

## 3. 核心结构体与职责

### 3.1 DB

LSM-Tree 主结构体，对外提供所有存储操作接口。

```go
type DB struct {
    mu         sync.RWMutex
    config     Config
    memTable   *MemTable
    immutable  []*MemTable
    flushing   *MemTable
    levels     []*Level
    seqNum     int64
    closed     bool
    mergeMu    sync.Mutex
    merging    atomic.Bool
    flushCh    chan struct{}
    mergeCh    chan struct{}
    wg         sync.WaitGroup
    stopCh     chan struct{}
}
```

**职责**:
- 管理内存结构（活跃 MemTable、Immutable 队列、正在刷盘的 MemTable）
- 管理多层 SSTable 目录
- 协调异步刷盘（flushLoop）与合并压缩（mergeLoop）后台 goroutine
- 提供 Put / Get / Delete / Range 等对外 API
- 保证并发操作的线程安全

### 3.2 MemTable

内存表，使用跳表作为底层有序数据结构。

```go
type MemTable struct {
    mu        sync.RWMutex
    data      *SkipList
    size      int
    maxSize   int
    frozen    bool
}
```

**职责**:
- 缓冲实时写入的键值对，按 key 字典序有序存储
- 记录当前数据字节大小，达到 `maxSize` 时触发刷盘
- 支持冻结（Freeze），冻结后不可再写入，等待刷盘
- 提供 `Get`（过滤 tombstone）和 `GetWithTombstone`（包含 tombstone）两种查询接口
- 提供 `Range` 范围扫描接口

### 3.3 SkipList

跳表（Skip List），一种概率性有序数据结构，作为 MemTable 的底层存储。

```go
type SkipList struct {
    header *SkipListNode
    tail   *SkipListNode
    level  int
    length int
    size   int
    random *rand.Rand
}
```

**职责**:
- 提供 O(log N) 平均复杂度的插入、查询、删除操作
- 按 key 字典序维护有序链表，支持范围扫描
- 通过随机层级（randomLevel）实现概率平衡，无需复杂的旋转操作
- 提供双向迭代器（Iterator）支持顺序遍历与 Seek 定位

### 3.4 Entry

键值条目，包含 key、value、删除标记与时间戳。

```go
type Entry struct {
    Key       string
    Value     string
    Tombstone bool
    Timestamp int64
}
```

**职责**:
- 表示 LSM-Tree 中的一条数据记录
- `Tombstone=true` 表示该 key 已被删除（墓碑标记）
- `Timestamp` 用于并发写入冲突解决，时间戳较大的版本胜出
- 支持二进制编码/解码用于持久化

### 3.5 SSTable

Sorted String Table，磁盘上的有序不可变键值文件。

```go
type SSTable struct {
    mu          sync.RWMutex
    filename    string
    level       int
    index       map[string]*IndexEntry
    indexOffset int64
    indexLen    int32
    minKey      string
    maxKey      string
    entryCount  int
    fileSize    int64
}
```

**职责**:
- 持久化存储已排序的键值对
- 维护内存索引（key → offset）实现 O(1) 点查询
- 记录 key 范围（minKey ~ maxKey）用于快速判断是否可能包含目标 key
- 提供 Get / Range / AllEntries 查询接口
- 文件格式：`[数据块...] + [索引块...] + [indexOffset(int64)] + [indexLen(int32)]`

### 3.6 IndexEntry

SSTable 索引条目，记录 key 在文件中的偏移位置。

```go
type IndexEntry struct {
    Key      string
    Offset   int64
    EntryLen int32
}
```

### 3.7 Level

SSTable 层级，管理同一层的所有 SSTable 文件。

```go
type Level struct {
    mu      sync.RWMutex
    level   int
    tables  []*SSTable
    maxSize int
}
```

**职责**:
- 管理同层所有 SSTable 文件，按 minKey 排序
- L0 层：文件间 key 范围可重叠，查询需遍历所有文件（从新到旧）
- L1+ 层：文件间 key 范围互不重叠，查询可用二分查找定位文件
- 提供 `NeedsCompaction()` 判断是否超过文件数阈值需要合并
- 提供 `FindOverlappingTables(min, max)` 查找与指定 key 范围重叠的文件

### 3.8 Config

LSM-Tree 配置。

```go
type Config struct {
    MemTableSize    int      // MemTable 刷盘阈值（字节）
    MaxLevel        int      // 最大层级数
    LevelMaxFiles   []int    // 每层允许的最大文件数
    TargetFileSize  int      // 合并时目标单文件大小（字节）
    DataDir         string   // SSTable 文件存储目录
}
```

## 4. 数据从写入到持久化的完整路径

### 4.1 写入流程（Put / Delete）

```
用户调用 Put(key, value)
        │
        ▼
  校验参数（空键 / DB 已关闭）
        │
        ▼
  获取 db.mu 写锁
        │
        ▼
  写入活跃 MemTable
  ├─ Put: 写入 {key, value, Tombstone=false, Timestamp=now}
  └─ Delete: 写入 {key, "", Tombstone=true, Timestamp=now}
        │
        ▼
  MemTable.size >= MemTableSize ?
        ├─ 否: 释放锁，返回成功
        │
        └─ 是:
             ├─ Freeze() 当前 MemTable（标记 frozen=true）
             ├─ 追加到 immutable 队列尾部
             ├─ 创建新的活跃 MemTable
             ├─ 向 flushCh 发送信号（非阻塞）
             └─ 释放锁，返回成功
```

### 4.2 刷盘流程（flushLoop）

```
flushLoop 后台 goroutine 接收 flushCh 信号
        │
        ▼
  flushImmutable():
        │
        ▼
  获取 db.mu 写锁
        │
        ▼
  immutable 队列为空? ──是──► 返回
        │否
        ▼
  取出队首 MemTable
  设置 db.flushing = 该 MemTable  （关键：保证搜索不遗漏）
        │
        ▼
  释放 db.mu 写锁
        │
        ▼
  flushMemTable(mt, level=0):
        │
        ▼
  1. 获取 mt.AllEntries()（跳表有序遍历）
  2. 分配全局递增 seqNum
  3. 生成文件名: L{level}_{seqNum}.sst
  4. 调用 NewSSTable() 写入磁盘：
     ├─ 按 key 排序并去重（同 key 取时间戳最大版本）
     ├─ 写入数据块（每个 Entry 二进制编码）
     ├─ 写入索引块（每个 key → offset 映射）
     └─ 写入 footer: indexOffset(int64) + indexLen(int32)
  5. 将新 SSTable 添加到 db.levels[0]
  6. 向 mergeCh 发送信号触发合并检查
        │
        ▼
  获取 db.mu 写锁
  设置 db.flushing = nil
  释放锁
        │
        ▼
  循环处理下一个 immutable MemTable
```

### 4.3 读取流程（Get）

```
用户调用 Get(key)
        │
        ▼
  校验参数
        │
        ▼
  获取 db.mu 读锁
        │
        ▼
  按优先级依次搜索：
        │
        ├─ 1. 活跃 MemTable（db.memTable.GetWithTombstone）
        │     ├─ 找到且 Tombstone=true  → 返回 ErrKeyNotFound
        │     ├─ 找到且 Tombstone=false → 返回 value
        │     └─ 未找到 → 继续
        │
        ├─ 2. Immutable 队列（从新到旧）
        │     └─ 同上逻辑
        │
        ├─ 3. 正在刷盘的 MemTable（db.flushing）
        │     └─ 同上逻辑
        │
        └─ 4. 多层 SSTable（从 L0 到 Ln）
              │
              ├─ L0 层: 从新到旧遍历所有 SSTable
              │     └─ 找到 entry:
              │           ├─ Tombstone=true  → 返回 ErrKeyNotFound
              │           └─ Tombstone=false → 返回 value
              │
              └─ L1+ 层: 二分查找定位可能包含 key 的 SSTable
                    └─ 在该 SSTable 内用索引 O(1) 定位
                          └─ 找到后同上判断 tombstone
        │
        ▼
  所有层均未找到 → 返回 ErrKeyNotFound
```

### 4.4 合并压缩流程（Compaction）

```
mergeLoop 接收 mergeCh 信号 或 定时 ticker 触发检查
        │
        ▼
  runCompaction():
        │
        ▼
  CAS 设置 merging=true（防并发合并）
        │
        ▼
  从 L0 到 Ln-1 逐层检查 NeedsCompaction()
        │
        ▼
  compactLevel(level):
        │
        ▼
  1. 选取本层待合并文件：
     ├─ L0: 取一个文件 + 所有与其 key 范围重叠的 L0 文件（L0 范围可重叠）
     └─ L1+: 取一个文件
        │
        ▼
  2. 计算合并范围 [minKey, maxKey]
        │
        ▼
  3. 在下层（level+1）查找所有与该范围重叠的 SSTable
        │
        ▼
  4. 读取所有待合并文件的全部 Entry
     按 key 聚合，同 key 取时间戳最大版本
        │
        ▼
  5. 检查更深层（level+2 及以下）是否仍有重叠 key 范围：
     ├─ 有重叠 → 保留 tombstone（防止旧数据"复活"）
     └─ 无重叠 → 丢弃 tombstone（真正物理删除）
        │
        ▼
  6. 按 TargetFileSize 将合并结果切分为多个新 SSTable
     写入到 level+1 层
        │
        ▼
  7. 原子替换：
     ├─ 从本层移除被合并的旧文件
     ├─ 从下层移除被合并的旧文件
     ├─ 向下层添加新生成的 SSTable
     └─ 删除磁盘上的旧 SSTable 文件
        │
        ▼
  循环直到该层不再需要合并
```

### 4.5 范围查询流程（Range）

```
用户调用 Range(start, end)
        │
        ▼
  校验参数（start > end → ErrInvalidRange）
        │
        ▼
  创建 resultMap（map[key]*Entry，用于按时间戳取最新版本）
        │
        ▼
  依次收集各数据源的范围内条目（含 tombstone）：
        │
        ├─ 1. 活跃 MemTable.RangeWithTombstone(start, end)
        ├─ 2. Immutable 队列（从旧到新，后者覆盖前者）
        ├─ 3. Flushing MemTable
        └─ 4. 多层 SSTable（L0 → Ln，同 key 高时间戳胜出）
              ├─ L0: 遍历所有文件，RangeWithTombstone 含 tombstone
              └─ L1+: 二分查找过滤范围不重叠的文件
        │
        ▼
  合并去重：对同 key 保留 Timestamp 最大的版本
        │
        ▼
  过滤 Tombstone=true 的条目（最终返回给用户前才过滤）
        │
        ▼
  按 key 字典序排序后返回
```

**范围查询中的墓碑处理策略**：

范围查询的墓碑处理与点查询（Get）有本质区别。点查询找到 tombstone 即可立即返回不存在，但范围查询必须收集所有层的 tombstone 才能确保正确的结果。

| 阶段 | 处理策略 | 说明 |
|------|---------|------|
| 数据收集阶段 | **不过滤 tombstone** | 各层调用 `RangeWithTombstone()` 方法，保留所有 tombstone 条目进入 resultMap |
| 版本合并阶段 | **按 Timestamp 取最大** | 同 key 的多个版本中，Timestamp 最大的胜出。如果最新版本是 tombstone，会覆盖下层的存活数据 |
| 最终返回阶段 | **过滤 tombstone** | 所有层合并完成后，统一过滤掉 Tombstone=true 的条目，不返回给用户 |

**为什么不能在各层内部过滤 tombstone**：
- 如果在 MemTable 层过滤掉 tombstone，resultMap 中就不会有该 key 的记录
- 后续遍历 SSTable 层时，该 key 的旧版本存活数据会被加入 resultMap
- 最终返回给用户时，这条已经被删除的数据又"复活"了
- 正确做法是让 tombstone 进入 resultMap，靠其较大的 Timestamp 覆盖下层旧版本

## 5. 关键设计决策

### 5.1 三层内存结构（MemTable + Immutable + Flushing）

为解决刷盘过程中的数据可见性问题，采用三层内存结构：

| 结构 | 可写 | 搜索优先级 | 说明 |
|------|------|-----------|------|
| memTable | ✅ | 最高 | 活跃写入表 |
| immutable[] | ❌ | 次高 | 已冻结、等待刷盘的队列 |
| flushing | ❌ | 第三 | 正在被刷盘 goroutine 写入磁盘的表 |

**关键不变式**：任何已接受写入的数据，至少存在于以上三层之一或已持久化到 SSTable，搜索时不会遗漏。

### 5.2 Tombstone（墓碑）机制

删除不是立即物理删除，而是写入一条 `Tombstone=true` 的特殊条目：

1. **读取时**：遇到 tombstone 立即返回 ErrKeyNotFound，不再继续搜索下层
2. **Range 时**：过滤掉 tombstone 条目，不返回给用户
3. **合并时**：仅当更深层不存在重叠 key 范围时才丢弃 tombstone，防止旧版本数据"复活"

### 5.3 层级策略差异

- **L0 层**：由刷盘直接写入，文件间 key 范围可重叠。查询时必须从新到旧遍历所有文件，确保读取到最新版本。
- **L1+ 层**：文件间 key 范围严格不重叠。查询时可通过二分查找快速定位目标文件，读取性能接近有序数组。

### 5.4 Level.Range 的墓碑处理策略

`Level.Range`（对外范围查询）与 `Level.RangeWithTombstone`（内部使用）在墓碑处理上存在本质差异。为避免跨文件比较时墓碑被提前过滤导致已删除 key 错误复活，必须遵循"先合并去重、最后过滤"的原则。

#### 错误的处理方式（已废弃）

```go
// L0 层遍历每个 SSTable 时直接调用 sst.Range（已过滤 tombstone）
for _, sst := range l.tables {
    entries, _ := sst.Range(start, end)  // ❌ 墓碑在这里已经被过滤了
    for _, e := range entries {
        if !seen[e.Key] {
            seen[e.Key] = true
            result = append(result, e)
        }
    }
}
```

**问题**：
- SSTable A（较新）包含 key "foo" 的 tombstone
- SSTable B（较旧）包含 key "foo" 的存活数据
- 遍历 A 时 sst.Range 过滤掉了 tombstone，resultMap 中无 "foo" 记录
- 遍历 B 时 sst.Range 返回存活数据，"foo" 被错误地加入结果
- 已删除的 key 在范围查询中"复活"了

#### 正确的处理方式（当前实现）

```go
// 第一步：调用 rangeInternal 收集含 tombstone 的所有条目，按 Timestamp 去重
func (l *Level) Range(start, end string) ([]*Entry, error) {
    entries, err := l.rangeInternal(start, end)  // 含 tombstone
    if err != nil {
        return nil, err
    }
    // 第二步：合并去重完成后，最后才过滤 tombstone
    result := make([]*Entry, 0, len(entries))
    for _, e := range entries {
        if !e.Tombstone {  // ✅ 在跨文件版本竞争之后才过滤
            result = append(result, e)
        }
    }
    return result, nil
}

// rangeInternal 内部始终使用 sst.RangeWithTombstone
func (l *Level) rangeInternal(start, end string) ([]*Entry, error) {
    for i := len(l.tables) - 1; i >= 0; i-- {
        entries, _ := sst.RangeWithTombstone(start, end)  // ✅ 保留 tombstone
        for _, e := range entries {
            existing, ok := resultMap[e.Key]
            if !ok || e.Timestamp > existing.Timestamp {
                resultMap[e.Key] = e  // tombstone 靠大 Timestamp 覆盖旧版本
            }
        }
    }
    // ...
}
```

**正确性保证**：
- 所有层的所有条目（包括 tombstone）都先进入 `resultMap`
- 同 key 的多个版本按 `Timestamp` 比较，时间戳大的胜出
- 如果最新版本是 tombstone，它会覆盖下层的存活数据
- 所有层合并完成后，才统一过滤掉 `Tombstone=true` 的条目
- 已删除的 key 不会"复活"

### 5.5 Range/RangeWithTombstone 的代码复用设计

`Range`（过滤 tombstone）和 `RangeWithTombstone`（保留 tombstone）两个方法的逻辑几乎完全相同，仅差一行墓碑过滤。如果分别维护两套代码，未来修改时极易出现同步遗漏导致的 bug（例如只改了 Range 忘了改 RangeWithTombstone）。

#### SSTable 的参数化复用

通过私有方法 `rangeInternal` + `filterTombstone` 布尔参数实现统一实现：

```go
// 对外 API：过滤 tombstone
func (sst *SSTable) Range(start, end string) ([]*Entry, error) {
    return sst.rangeInternal(start, end, true)
}

// 对内 API：保留 tombstone
func (sst *SSTable) RangeWithTombstone(start, end string) ([]*Entry, error) {
    return sst.rangeInternal(start, end, false)
}

// 统一实现：仅在追加结果时根据参数决定是否过滤
func (sst *SSTable) rangeInternal(start, end string, filterTombstone bool) ([]*Entry, error) {
    // ... 打开文件、按范围收集 key、排序等公共逻辑 ...
    for _, key := range sortedKeys {
        // ... 读取并解码 entry ...
        if !filterTombstone || !entry.Tombstone {
            result = append(result, entry)  // 唯一的差异点
        }
    }
    return result, nil
}
```

**优点**：
- 公共逻辑（文件打开、范围过滤、key 排序、磁盘 I/O）只有一份
- 修改核心逻辑时只需改动一处
- 两个对外方法仅为简单的参数转发，不可能出现行为不一致

#### Level 的分层复用

Level 层的差异点更多（不仅是过滤 tombstone，还要跨文件版本合并），采用"先合并、后过滤"的分层策略：

```go
// 对外 API：先收集全部含 tombstone 条目去重，最后再过滤
func (l *Level) Range(start, end string) ([]*Entry, error) {
    entries, err := l.rangeInternal(start, end)  // 步骤 1：含 tombstone 的合并去重
    if err != nil {
        return nil, err
    }
    result := make([]*Entry, 0, len(entries))
    for _, e := range entries {
        if !e.Tombstone {  // 步骤 2：最后过滤
            result = append(result, e)
        }
    }
    return result, nil
}

// 对内 API：直接返回合并去重后的结果（含 tombstone）
func (l *Level) RangeWithTombstone(start, end string) ([]*Entry, error) {
    return l.rangeInternal(start, end)
}

// 统一核心实现：跨文件按 Timestamp 合并去重
func (l *Level) rangeInternal(start, end string) ([]*Entry, error) {
    // ... 遍历 SSTable、调用 sst.RangeWithTombstone、版本竞争去重 ...
}
```

**复用原则**：
- SSTable 层：使用布尔参数 `filterTombstone`，因为两个方法的结构完全一致
- Level 层：使用"内部方法 + 外层过滤"的分层方式，因为 `Range` 需要在去重后才能过滤 tombstone
- 无论采用哪种方式，核心原则都是"差异逻辑最小化，公共逻辑唯一化"

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
    "fmt"
    "log"
    "solocoder-go/internal/lsm"
)

func main() {
    // 使用默认配置
    config := lsm.DefaultConfig()
    config.DataDir = "./my_lsm_data"

    db, err := lsm.NewDB(config)
    if err != nil {
        log.Fatal("NewDB failed:", err)
    }
    defer db.Close()

    // 写入数据
    if err := db.Put("user:1:name", "Alice"); err != nil {
        log.Fatal(err)
    }
    if err := db.Put("user:1:email", "alice@example.com"); err != nil {
        log.Fatal(err)
    }

    // 读取数据
    name, err := db.Get("user:1:name")
    if err != nil {
        if err == lsm.ErrKeyNotFound {
            fmt.Println("Key not found")
        } else {
            log.Fatal(err)
        }
    } else {
        fmt.Println("Name:", name) // Alice
    }

    // 更新数据
    db.Put("user:1:name", "Alice Smith")
    name, _ = db.Get("user:1:name")
    fmt.Println("Updated:", name) // Alice Smith

    // 删除数据
    if err := db.Delete("user:1:email"); err != nil {
        log.Fatal(err)
    }
    _, err = db.Get("user:1:email")
    fmt.Println("After delete:", err) // key not found
}
```

### 6.2 范围查询

```go
// 写入一批有序数据
for i := 0; i < 100; i++ {
    key := fmt.Sprintf("user:%04d", i)
    db.Put(key, fmt.Sprintf("value_%d", i))
}

// 扫描 [user:0010, user:0019] 范围
entries, err := db.Range("user:0010", "user:0019")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Found %d entries:\n", len(entries)) // 10
for _, e := range entries {
    fmt.Printf("  %s = %s\n", e.Key, e.Value)
}
// 输出按 key 字典序排序
```

### 6.3 自定义配置与大数据量

```go
// 针对大规模数据优化的配置
config := lsm.Config{
    MemTableSize:   64 * 1024 * 1024,  // 64MB MemTable
    MaxLevel:       6,                  // 6 层
    LevelMaxFiles:  []int{8, 16, 32, 64, 128, 256},
    TargetFileSize: 8 * 1024 * 1024,   // 8MB 每 SSTable
    DataDir:        "./large_data",
}

db, err := lsm.NewDB(config)
if err != nil {
    log.Fatal(err)
}
defer db.Close()

// 写入 10 万条数据
for i := 0; i < 100000; i++ {
    key := fmt.Sprintf("key_%08d", i)
    val := fmt.Sprintf("value_%08d_somewhat_longer_payload", i)
    if err := db.Put(key, val); err != nil {
        log.Fatal(err)
    }
}

// 后台自动刷盘与合并
// 可以通过 DebugInfo 查看内部状态
fmt.Println(db.DebugInfo())
```

### 6.4 并发使用

```go
var wg sync.WaitGroup

// 10 个 goroutine 并发写入，每个 1000 条
for g := 0; g < 10; g++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for i := 0; i < 1000; i++ {
            key := fmt.Sprintf("worker_%d_key_%d", id, i)
            db.Put(key, fmt.Sprintf("val_%d", i))
        }
    }(g)
}

// 同时有 goroutine 进行读取
for g := 0; g < 5; g++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        for i := 0; i < 100; i++ {
            key := fmt.Sprintf("worker_0_key_%d", i)
            db.Get(key) // 忽略错误，只验证不 panic
        }
    }()
}

wg.Wait()
fmt.Println("All concurrent ops done")
```

### 6.5 重启恢复

```go
// 第一次运行：写入数据并关闭
config := lsm.Config{DataDir: "./persistent_data"}
db, _ := lsm.NewDB(config)
db.Put("important_key", "critical_value")
db.Close() // 刷盘所有剩余 MemTable

// 第二次运行：自动加载磁盘上的 SSTable
db2, _ := lsm.NewDB(config)
defer db2.Close()
val, _ := db2.Get("important_key")
fmt.Println("Recovered:", val) // critical_value
```

## 7. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | Get 未找到或已被 tombstone 标记删除 |
| `ErrInvalidRange` | 范围无效 | Range(start, end) 中 start > end |
| `ErrInvalidLimit` | 限制参数非法 | 预留错误 |
| `ErrDBClosed` | 数据库已关闭 | Close() 后再调用 Put / Get / Delete / Range |
| `ErrEmptyKey` | 键不能为空 | Put / Get / Delete 传入空字符串 key |
| `ErrMergeInProgress` | 合并进行中 | 预留错误，当前合并通过 CAS 原子控制 |

## 8. SSTable 文件格式

```
┌─────────────────────────────────────────────────────────────┐
│                        SSTable 文件                          │
├─────────────────────────────────────────────────────────────┤
│  数据块 (Data Block)                                         │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ Entry 1: [keyLen(4B)][key][valLen(4B)][val]           │  │
│  │          [tombstone(1B)][timestamp(8B)]               │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ Entry 2: ...                                           │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ ... (按 key 字典序排列)                                │  │
│  └───────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  索引块 (Index Block)                                        │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ IndexEntry 1: [keyLen(4B)][key]                       │  │
│  │               [offset(8B)][entryLen(4B)]              │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ IndexEntry 2: ...                                      │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ ... (按 key 字典序排列)                                │  │
│  └───────────────────────────────────────────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│  Footer (固定 12 字节)                                       │
│  ┌───────────────────────────────────────────────────────┐  │
│  │ indexOffset: int64 (8B)  索引块起始偏移               │  │
│  ├───────────────────────────────────────────────────────┤  │
│  │ indexLen:    int32 (4B)  索引块总字节数               │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

## 9. 性能特征与时间复杂度

| 操作 | 平均复杂度 | 最坏复杂度 | 说明 |
|------|-----------|-----------|------|
| Put | O(log M) | O(log M) | M 为 MemTable 大小，跳表插入 |
| Delete | O(log M) | O(log M) | 同 Put，写入 tombstone |
| Get（内存命中） | O(log M) | O(log M) | 跳表查询 |
| Get（磁盘命中） | O(log M + L + log K) | O(M + L * N) | L=层级数，K=单 SSTable key 数；L0 最坏遍历全部文件 |
| Range | O(R + S log S) | O(N log N) | R=范围内条目数，S=合并后条目数，需排序 |
| Flush | O(K) | O(K) | K=MemTable 条目数，顺序写磁盘 |
| Compaction | O(N) | O(N) | N=被合并文件总条目数，归并去重 |

### 写放大与空间放大

- **写放大（Write Amplification）**：同一条数据可能被多次写入磁盘（每次经过的层级合并各写一次）。层级越多，最坏写放大越大。典型配置下写放大约为 10~50 倍。
- **空间放大（Space Amplification）**：同一 key 的旧版本和 tombstone 会暂时占用额外空间，直到被合并清理。合并越及时，空间放大越低。

## 10. 并发安全保证

模块使用细粒度锁机制保证并发安全：

| 组件 | 锁机制 | 说明 |
|------|--------|------|
| DB 整体状态 | `sync.RWMutex` | 保护 memTable / immutable / flushing / levels / seqNum / closed |
| MemTable | `sync.RWMutex` | 保护跳表引用、大小统计、冻结状态 |
| SkipList | `sync.RWMutex` | 保护跳表内部节点结构。迭代器在整个迭代期间持有读锁，防止并发修改导致 panic |
| SSTable | `sync.RWMutex` | 保护索引与元数据读取（SSTable 本身不可变，锁主要用于加载） |
| Level | `sync.RWMutex` | 保护该层 SSTable 列表的增删与排序 |
| Compaction | `mergeMu` + `atomic.Bool` | `merging` 原子标记防止并发合并，`mergeMu` 串行化合并过程 |

### 10.1 SkipList 迭代器锁生命周期管理

SkipListIterator 在创建时获取 SkipList 的读锁，并通过三层保底机制确保锁最终一定会被释放，防止读锁泄漏导致写路径永久阻塞。

#### 锁生命周期的三层保底机制

```
创建迭代器（Iterator()）
        │
        ▼
  ① 构造时获取读锁 + 注册 runtime.SetFinalizer
        │
        ▼
  ② 迭代结束自动释放：Next() 返回 false 时自动调用 Close()
        │
        ▼
  ③ GC 兜底释放：迭代器对象被 GC 时 finalizer 调用 Close()
```

**代码实现**：
```go
// SkipList.Iterator(): 构造时加锁 + 注册 finalizer
func (sl *SkipList) Iterator() *SkipListIterator {
    sl.mu.RLock()
    it := &SkipListIterator{
        sl:      sl,
        current: sl.header.forward[0],
        locked:  true,
    }
    runtime.SetFinalizer(it, func(iter *SkipListIterator) {
        iter.Close()  // 第三层：GC 兜底
    })
    return it
}

// Next(): 迭代结束时自动释放锁
func (it *SkipListIterator) Next() bool {
    if !it.started {
        it.started = true
        if it.current == nil {
            it.Close()  // 第二层：空迭代器立即释放
            return false
        }
        return true
    }
    if it.current == nil {
        it.Close()
        return false
    }
    it.current = it.current.forward[0]
    if it.current == nil {
        it.Close()  // 第二层：遍历到末尾自动释放
        return false
    }
    return true
}

// Close(): 幂等操作，多次调用安全
func (it *SkipListIterator) Close() {
    if it.locked {
        it.sl.mu.RUnlock()
        it.locked = false
    }
}
```

#### 各层保底的触发场景与局限性

| 层级 | 机制 | 触发时机 | 优点 | 局限性 |
|------|------|---------|------|--------|
| ① 最佳实践 | 调用者 `defer iter.Close()` | 迭代器使用完毕立即释放 | 释放最及时，锁持有时间最短 | 依赖调用者自觉遵守，调用者可能忘记 |
| ② 自动释放 | `Next()` 返回 `false` | 迭代正常结束（遍历到末尾或空表） | 无需调用者额外操作，覆盖"完整遍历"场景 | **不覆盖**提前 break 的场景；Seek 后不调用 Next 不会触发 |
| ③ 兜底释放 | `runtime.SetFinalizer` | 迭代器对象被 GC 回收 | 最终一定会释放，防止永久泄漏 | **GC 触发时机不可控**，锁可能在 GC 前长时间持有；大量迭代器未显式 Close 会导致写路径长时间阻塞；仅作为最后防线，不应依赖 |

> **重要提醒**：第③层 finalizer 机制仅为防止资源永久泄漏的安全网，绝不能作为常规释放手段。在高并发写入场景下，如果调用方频繁创建迭代器却不显式 Close，读锁会大量累积，导致 SkipList 写操作长时间阻塞，整个 LSM-Tree 写入吞吐量严重下降。所有生产级代码都应使用第①层 `defer iter.Close()` 模式。

#### 为什么需要三层保底

最初版本仅依赖调用者显式调用 `Close()`，存在以下问题：
- 测试代码中创建迭代器后未调用 `Close()`
- 调用者遍历到一半 `break` 跳出循环时，`Next()` 还没返回 `false`
- 迭代器被提前丢弃，引用丢失

如果读锁永久不释放：
- SkipList 的所有写操作（`Insert`/`Delete`）会永久阻塞等待写锁
- 整个 LSM-Tree 写入路径卡死，数据库停止响应

三层保底确保即使上层调用方式不规范，锁最终仍会被释放。

#### 为什么迭代器需要在整个迭代期间持有锁

- 如果迭代器仅在构造时获取 `current` 指针后立即释放锁
- 其他 goroutine 在迭代过程中修改跳表节点（`Insert`/`Delete`）
- 节点的 `forward` 指针可能被修改或节点内存被回收
- 迭代器访问 `current.forward[0]` 时可能读到已修改的指针导致 panic

**已修复的问题**：原实现中 SkipList 没有任何锁保护，Iterator 在构造时获取 current 指针后不再持有锁，并发场景下存在严重的并发安全隐患，且锁泄漏会导致写路径永久阻塞。

### 10.2 关闭状态检查的原子性（TOCTOU 问题修复）

DB 的 `closed` 字段表示数据库是否已关闭。所有对外 API（Put/Delete/Get/Range）必须在操作前检查该字段，防止关闭后继续写入导致数据丢失。

#### 错误的检查方式（已废弃）
```go
db.mu.RLock()
if db.closed {
    db.mu.RUnlock()
    return ErrDBClosed
}
db.mu.RUnlock()  // 这里释放了锁

// ┌───────────────────────┐
// │  TOCTOU 窗口期        │
// │  此时其他 goroutine   │
// │  可能调用 db.Close()  │
// └───────────────────────┘

db.mu.Lock()  // 重新获取锁，但此时 db.closed 可能已经是 true
db.memTable.Put(key, value)  // 已关闭的 DB 仍可接受写入！
db.mu.Unlock()
```

这是典型的 **TOCTOU（Time Of Check, Time Of Use）** 竞态条件：检查和使用之间的时间窗口内，状态可能被改变。

#### 正确的原子性检查（当前实现）

**对于写入操作（Put/Delete）**：
```go
db.mu.Lock()
defer db.mu.Unlock()

if db.closed {  // 检查和操作在同一个写锁持有期间
    return ErrDBClosed
}
db.memTable.Put(key, value)  // 安全，锁保证了 closed 不会在操作中改变
```
检查和操作在同一个写锁持有期间完成，不存在窗口期。

**对于读取操作（Get/Range）**：
```go
db.mu.RLock()
if db.closed {
    db.mu.RUnlock()
    return ErrDBClosed
}
// 内存部分搜索在同一个读锁持有期间完成
entry, found := db.memTable.GetWithTombstone(key)
// ... immutable / flushing 也在同一个读锁下搜索
db.mu.RUnlock()

// SSTable 部分搜索不需要持有 DB 读锁（SSTable 有自己的锁）
// 但每一层搜索前重新检查 closed
for level := 0; level < maxLevel; level++ {
    db.mu.RLock()
    if db.closed {
        db.mu.RUnlock()
        return ErrDBClosed
    }
    db.mu.RUnlock()
    // ... 搜索该层 SSTable
}
```
- 内存部分的搜索与 `closed` 检查在同一个读锁持有期间，保证原子性
- SSTable 部分每次搜索前重新检查 `closed`，即使中途被关闭也能及时响应
- 不存在 TOCTOU 窗口期

### 10.3 已验证场景
- 多 goroutine 并发 Put（10 goroutine × 100 条）无数据丢失
- 并发 Put + Get 无幽灵丢失（数据已写入但返回不存在）
- 刷盘过程中读取不丢失不可变 MemTable 中的数据
- Close 后所有操作正确返回 ErrDBClosed
- 并发迭代跳表 + 并发修改跳表无 panic
- Close 与 Put 并发执行时不会出现已关闭 DB 仍写入的情况

## 11. 注意事项与限制

1. **WAL（预写日志）暂未实现**：当前版本写入仅存在于内存中，若进程在刷盘前崩溃，最近的写入会丢失。生产环境需配合 WAL 使用。
2. **无布隆过滤器**：当前 SSTable 查询直接使用内存哈希索引，内存占用较高。可后续添加布隆过滤器快速拒绝不存在的 key。
3. **合并是同步阻塞的**：合并过程中占用大量磁盘 I/O 与 CPU，高写入压力下可能产生写入延迟毛刺。
4. **L0 层无范围限制**：大量瞬时写入可能导致 L0 文件堆积，查询退化为线性扫描。建议合理设置 MemTableSize 与 LevelMaxFiles[0]。
5. **不支持事务**：每次 Put/Delete 是独立原子操作，但不支持跨多键的 ACID 事务。
6. **无快照读取**：Range 和 Get 不保证读取某一特定时间点的一致性视图，仅保证单键最终一致。
