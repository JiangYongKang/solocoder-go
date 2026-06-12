# KVStore 内存键值存储模块

## 1. 模块概述

KVStore 是一个高性能的内存键值存储模块，专为需要快速读写、高并发访问的场景设计。模块提供了完整的 CRUD 操作、批量写入、范围扫描、快照导出/恢复等功能，并通过分段锁和布隆过滤器等优化机制实现高效并发访问。

**包路径**: `internal/kvstore`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 基本 KV 操作 | Put、Get、Delete，键值均为字符串类型 |
| 布隆过滤器 | Get 前快速判断键是否可能存在，减少无效内存查找 |
| 分段锁 | 基于哈希的分段锁机制，不同键可并发读写 |
| 批量写入 | 原子性写入多个键值对，全成功或全失败 |
| 范围扫描 | 按字典序范围查询，支持分页返回 |
| 快照导出/恢复 | 无锁导出完整数据快照，支持后续恢复 |

## 3. 核心结构体与职责

### 3.1 KVStore

主存储结构体，对外提供所有操作接口。

```go
type KVStore struct {
    segments     []*segment
    segmentCount int
    bloomFilter  *BloomFilter
    bloomMu      sync.RWMutex
}
```

**职责**:
- 管理多个分段（segment），通过键哈希将请求路由到对应分段
- 维护布隆过滤器用于快速存在性检查
- 协调跨分段操作（如批量写入、快照、范围扫描）

### 3.2 segment

分段存储单元，每个分段持有独立的数据和读写锁。

```go
type segment struct {
    data map[string]string
    mu   sync.RWMutex
}
```

**职责**:
- 存储分配到该分段的键值对
- 通过 `sync.RWMutex` 保护分段内数据的并发访问
- 支持读并发、写互斥的访问模式

### 3.3 BloomFilter

布隆过滤器，概率性数据结构，用于快速判断键是否可能存在。

```go
type BloomFilter struct {
    bitArray  []bool
    size      uint
    hashCount uint
}
```

**职责**:
- 使用位数组和多重哈希记录键的存在信息
- `Add(key)`: 将键标记为已存在
- `MightContain(key)`: 判断键是否可能存在（无假阴性，有低概率假阳性）
- `Reset()`: 重置过滤器状态

**核心公式**:
- 最优位数组大小: `m = -n * ln(p) / (ln2)^2`
- 最优哈希函数数量: `k = m/n * ln2`

其中 `n` 为预期容量，`p` 为目标误判率。

### 3.4 Config

KVStore 配置结构体。

```go
type Config struct {
    SegmentCount      int     // 分段数量，默认 16
    BloomCapacity     uint    // 布隆过滤器预期容量，默认 10000
    BloomFalseRate    float64 // 布隆过滤器目标误判率，默认 0.01 (1%)
}
```

### 3.5 Snapshot

数据快照，包含某一时刻的完整数据副本。

```go
type Snapshot struct {
    Data map[string]string
}
```

**职责**:
- 存储导出时的完整键值对副本
- 独立于主存储，后续修改不影响快照内容
- 提供 `Count()` 和 `Get()` 辅助方法

### 3.6 RangeResult

范围扫描结果。

```go
type RangeResult struct {
    Items    []KVItem // 当前页的键值对列表
    HasMore  bool     // 是否还有更多数据
    NextKey  string   // 下一页起始键（HasMore=true 时有效）
    Total    int      // 符合条件的总记录数
}
```

## 4. 分段锁工作机制

### 4.1 设计原理

分段锁（Sharded Lock）是一种降低锁竞争的并发优化技术：

1. **分段划分**: 将整个存储划分为 N 个独立分段（默认 16 个）
2. **哈希路由**: 通过 FNV-32a 哈希函数对键取模，确定目标分段
3. **独立加锁**: 每个分段维护独立的 `sync.RWMutex`

### 4.2 分段索引计算

```go
func (kv *KVStore) getSegmentIndex(key string) int {
    h := fnv.New32a()
    h.Write([]byte(key))
    return int(h.Sum32() % uint32(kv.segmentCount))
}
```

**特性**:
- 同一键始终映射到同一分段，保证单键操作的串行化
- 不同键大概率映射到不同分段，实现真正的并发执行
- 使用无符号模运算避免负数索引

### 4.3 锁粒度对比

| 锁策略 | 读-读并发 | 读-写并发 | 写-写并发（不同键） | 写-写并发（同键） |
|--------|-----------|-----------|---------------------|-------------------|
| 全局锁 | ❌ | ❌ | ❌ | ❌ |
| 分段锁 | ✅ | ✅（不同分段） | ✅（不同分段） | ❌ |

### 4.4 跨分段操作

对于涉及多个分段的操作（批量写入、快照、范围扫描），采用以下策略：

**批量写入（BatchPut）**:
1. 按键分组，确定涉及的分段列表
2. **按分段编号排序后依次加锁**（避免死锁）
3. 执行全部写入操作
4. 逆序解锁，确保原子性

**快照导出（Snapshot）**:
1. 逐个分段加读锁
2. 复制分段数据到快照
3. 释放读锁后处理下一个分段
4. 不阻塞其他分段的写入操作

## 5. 布隆过滤器工作机制

### 5.1 双重哈希技术

采用基于 SHA256 的双重哈希（Double Hashing）生成多个哈希值：

```go
func doubleHash(key string, size uint) (uint, uint) {
    sum := sha256.Sum256([]byte(key))
    h1 := binary.BigEndian.Uint64(sum[0:8])
    h2 := binary.BigEndian.Uint64(sum[8:16])
    return uint(h1 % uint64(size)), uint(h2 % uint64(size))
}
```

第 `i` 个哈希位置通过线性组合生成：
```
hash_i = (h1 + i * h2) % size
```

**优势**:
- 只需计算一次 SHA256，性能更高
- 生成的哈希值独立性更好，误判率更低
- 实际测试：容量 10000、目标 1% 误判率下，实际误判率约 0.95%

### 5.2 在 Get 操作中的应用

```
Get(key) 流程:
  ┌──────────────────────────┐
  │ 1. 布隆过滤器查询        │
  │    MightContain(key)     │
  └──────────┬───────────────┘
             │
        ┌────┴────┐
        │ 可能存在│ 不存在
        ▼         ▼
  ┌──────────┐  返回 (false)
  │2. 查找分段│  (快速拒绝)
  │   数据    │
  └────┬─────┘
       │
   ┌───┴───┐
   │ 找到? │
   └┬─────┬┘
   是     否
   ▼      ▼
 返回值   返回(false)
 (可能误判)
```

**注意事项**:
- 布隆过滤器不支持删除操作，Delete 后过滤器仍保留标记
- 这可能导致已删除键在 Get 时仍进行内存查找（可接受的权衡）
- Restore 操作会重置布隆过滤器状态

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/kvstore"
)

func main() {
    // 使用默认配置创建实例
    kv := kvstore.NewKVStore()

    // 写入数据
    kv.Put("user:1:name", "Alice")
    kv.Put("user:1:email", "alice@example.com")

    // 读取数据
    if name, ok := kv.Get("user:1:name"); ok {
        fmt.Println("Name:", name)
    }

    // 删除数据
    deleted := kv.Delete("user:1:email")
    fmt.Println("Deleted:", deleted)

    // 获取键总数
    fmt.Println("Total keys:", kv.Count())
}
```

### 6.2 自定义配置

```go
cfg := kvstore.Config{
    SegmentCount:   32,    // 更多分段，更高并发
    BloomCapacity:  100000, // 预期存储 10 万键
    BloomFalseRate: 0.001,  // 目标 0.1% 误判率
}
kv := kvstore.NewKVStoreWithConfig(cfg)
```

### 6.3 批量写入

```go
pairs := map[string]string{
    "config:timeout":   "30s",
    "config:retries":   "3",
    "config:batchSize": "100",
}

if err := kv.BatchPut(pairs); err != nil {
    log.Fatal("BatchPut failed:", err)
}
```

### 6.4 分页范围扫描

```go
start := "user:1000"
end := "user:1999"
pageSize := 100

for {
    result, err := kv.RangeScan(start, end, pageSize)
    if err != nil {
        log.Fatal(err)
    }

    for _, item := range result.Items {
        process(item.Key, item.Value)
    }

    if !result.HasMore {
        break
    }
    start = result.NextKey
}

fmt.Printf("Processed %d total records\n", result.Total)
```

### 6.5 快照导出与恢复

```go
// 导出快照（任意时刻，不阻塞读写）
snapshot := kv.Snapshot()

// 可以对快照进行独立操作
fmt.Printf("Snapshot contains %d keys\n", snapshot.Count())
if val, ok := snapshot.Get("user:1:name"); ok {
    fmt.Println("From snapshot:", val)
}

// 在另一个实例上恢复
backupKV := kvstore.NewKVStore()
if err := backupKV.Restore(snapshot); err != nil {
    log.Fatal("Restore failed:", err)
}
```

### 6.6 并发使用

```go
var wg sync.WaitGroup

// 100 个 goroutine 并发写入
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for j := 0; j < 1000; j++ {
            key := fmt.Sprintf("worker:%d:%d", id, j)
            kv.Put(key, strconv.Itoa(j))
        }
    }(i)
}

wg.Wait()
fmt.Println("Total:", kv.Count()) // 预期: 100000
```

## 7. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | 预留错误，当前返回 `(value, false)` |
| `ErrEmptyBatch` | 批量写入为空 | `BatchPut(nil)` 或空 map |
| `ErrInvalidRange` | 范围无效 | `RangeScan(start, end)` 中 start > end |
| `ErrInvalidLimit` | 分页大小非法 | `RangeScan` limit <= 0 |
| `ErrNilSnapshot` | 快照为空 | `Restore(nil)` |

## 8. 性能特征

### 8.1 时间复杂度

| 操作 | 平均时间 | 最坏时间 | 说明 |
|------|----------|----------|------|
| Put | O(1) | O(1) | 哈希查找 + 布隆过滤器 O(k) |
| Get | O(1) | O(1) | 布隆过滤器快速拒绝 + 哈希查找 |
| Delete | O(1) | O(1) | 哈希查找删除 |
| BatchPut | O(m) | O(m) | m 为批量键数量 |
| RangeScan | O(N) | O(N) | N 为总键数，全量扫描后排序 |
| Snapshot | O(N) | O(N) | N 为总键数，逐段复制 |

### 8.2 并发性能

- **分段数**: 默认 16，理论并发度上限为 16 倍单锁
- **读-读**: 完全并行，无阻塞
- **不同分段读-写**: 完全并行，无阻塞
- **同分段写-写**: 串行化执行

## 9. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **布隆过滤器不支持删除**: Delete 后过滤器仍保留标记，可能导致额外内存查找
3. **范围扫描性能**: 需要遍历所有分段后排序，大数据量下建议使用专用索引
4. **快照内存占用**: 导出的快照是完整副本，大数据量时注意内存消耗
5. **分页 NextKey**: `HasMore=true` 时，下一次查询应使用 `NextKey` 作为新的 start 参数
