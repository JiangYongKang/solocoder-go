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
