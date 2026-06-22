# 跳表（SkipList）数据结构模块

## 1. 模块概述

跳表（Skip List）是一种基于概率的有序数据结构，通过在有序链表的基础上构建多层索引，实现接近平衡树的查询效率。本模块提供了一个基于 Go 泛型的并发安全跳表实现，支持任意可比较（`cmp.Ordered`）的键类型和任意值类型。

**核心特性：**
- 使用 Go 泛型，支持 `K cmp.Ordered` 和任意 `V` 类型
- 按序插入与删除，支持键值对的更新
- 精确查找，利用多层索引实现 O(log n) 平均查询复杂度
- 功能丰富的范围查询，支持开/闭区间、分页（Limit + Offset）
- 可配置的概率层高因子，平衡内存占用与查询效率
- 内置读写锁，支持并发安全操作

---

## 2. 核心结构体与职责

### 2.1 Config

跳表创建时的配置结构体。

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `MaxLevel` | `int` | 跳表允许的最大层数 | `32` |
| `P` | `float64` | 每层提升概率，范围 `(0, 1)` | `0.25` |
| `RandomSource` | `*rand.Rand` | 自定义随机数源；为 `nil` 时使用基于系统时间的随机源 | `nil` |

**说明：**
- **MaxLevel**：限制节点可以达到的最高层，避免极端情况下层高失控。理论上，当 `P=0.5` 时，32 层可支持约 40 亿个节点。
- **P（概率因子）**：决定节点从第 i 层提升到第 i+1 层的概率。值越大，层数越多，索引越密集，查询越快，但内存占用越大。
  - `P=0.5`（Redis 使用值）：约 1/p^k 个节点达到第 k 层
  - `P=0.25`（默认值）：内存更省，适合写多读少的场景
- **RandomSource**：注入可控随机源，便于测试或确定性复现。设置为固定种子的 `*rand.Rand` 可使跳表的层级结构完全可预测（不影响正确性，仅影响性能相关的层数分布）。

### 2.2 node[K, V]

跳表内部节点结构体（私有）。

| 字段 | 类型 | 说明 |
|------|------|------|
| `key` | `K` | 节点的键，用于排序和查找，必须实现 `cmp.Ordered` |
| `value` | `V` | 节点存储的值，任意类型 |
| `forward` | `[]*node[K, V]` | 向前指针数组，`forward[i]` 表示该节点在第 i 层的下一个节点 |

### 2.3 SkipList[K, V]

跳表主结构体，所有操作的入口。

| 字段 | 类型 | 说明 |
|------|------|------|
| `mu` | `sync.RWMutex` | 读写锁，保证并发安全。读操作（Search、Range、All 等）共享读锁，写操作（Insert、Delete、Clear）持有写锁 |
| `header` | `*node[K, V]` | 哨兵头节点，不存储有效数据，`forward` 数组长度等于 `maxLevel` |
| `tail` | `*node[K, V]` | 尾节点指针（最底层最后一个节点），用于 O(1) 获取最大值 |
| `level` | `int` | 当前跳表实际使用的最大层数（从 1 开始） |
| `length` | `int` | 当前跳表中的节点数量（有效键值对个数） |
| `maxLevel` | `int` | 配置的最大允许层数 |
| `p` | `float64` | 配置的概率因子 |
| `random` | `*rand.Rand` | 随机数生成器，用于 `randomLevel()` 的概率计算 |

### 2.4 Pair[K, V]

范围查询等操作返回的键值对结构。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Key` | `K` | 键 |
| `Value` | `V` | 值 |

### 2.5 RangeOptions

范围查询的选项配置。

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `StartInclusive` | `bool` | 起始 Key 是否包含（闭区间起点） | `true` |
| `EndInclusive` | `bool` | 结束 Key 是否包含（闭区间终点） | `true` |
| `Limit` | `int` | 返回结果的最大数量，`0` 表示不限制 | `0` |
| `Offset` | `int` | 跳过前 N 条符合条件的结果后再返回 | `0` |

**链式构造方法：**

- `DefaultRangeOptions() *RangeOptions`：返回默认选项
- `WithStartInclusive(v bool)`：设置起始区间开闭
- `WithEndInclusive(v bool)`：设置结束区间开闭
- `WithLimit(v int)`：设置返回数量上限
- `WithOffset(v int)`：设置偏移量

---

## 3. 跳表层级结构与概率机制

### 3.1 多层索引结构

跳表由多层有序链表组成，底层（第 0 层）是包含所有节点的完整有序链表，每一层都是下一层的"快速通道"（索引）：

```
Level 3:  header ──────────────────────────────────> [40] ──> nil
Level 2:  header ──────────────> [15] ─────────────> [40] ──> nil
Level 1:  header ──> [5] ───────> [15] ──> [25] ───> [40] ──> nil
Level 0:  header ──> [5] -> [10]-> [15] -> [20]-> [25] -> [30]-> [40] -> nil
```

- **第 0 层（底层）**：包含所有节点，按 Key 严格升序排列
- **第 i 层（i > 0）**：从第 i-1 层的节点中按概率 P 选出一部分节点作为索引
- **查询路径**：从顶层（当前最高层）开始，向右找到不超过目标的最大节点，然后下降到下一层继续，直到底层找到目标或确认不存在

### 3.2 概率层高算法（randomLevel）

每次插入新节点时，通过以下算法决定该节点的层高：

```
level := 1
for level < maxLevel && random.Float64() < P:
    level++
return level
```

**概率分布（P=0.25）：**
- 1 层：100%  - 所有节点都在底层
- 2 层：25%   - 约 1/4 的节点提升到第 2 层
- 3 层：6.25% - 约 1/16 的节点提升到第 3 层
- 4 层：1.56% - 约 1/64 的节点提升到第 4 层
- ...

### 3.3 时间复杂度分析

| 操作 | 平均复杂度 | 最坏复杂度 |
|------|-----------|-----------|
| Insert | O(log n) | O(n) |
| Delete | O(log n) | O(n) |
| Search | O(log n) | O(n) |
| Range  | O(k + log n)，k 为返回数量 | O(n) |

空间复杂度：O(n)，平均额外索引空间约为 n/(1-P)。

---

## 4. 范围查询机制

范围查询（`Range` 方法）是跳表的核心高级功能，执行流程如下：

### 4.1 定位起点

利用跳表的多层索引，从顶层向下快速找到**第一个 >= start 的节点**，这一步的时间复杂度为 O(log n)。

### 4.2 遍历与筛选

从定位到的节点开始，沿第 0 层（底层链表）向右遍历，对每个节点执行：

1. **终止检查**：如果节点 Key > end，立即终止遍历
2. **开闭区间判断**：
   - 当 Key == start 时，根据 `StartInclusive` 决定是否包含
   - 当 Key == end 时，根据 `EndInclusive` 决定是否包含
   - 当 start < Key < end 时，始终包含
3. **偏移与分页**：
   - 先跳过前 `Offset` 条符合条件的记录
   - 然后收集记录，直到达到 `Limit`（如果 Limit > 0）或遍历结束

### 4.3 区间组合支持

| StartInclusive | EndInclusive | 数学表示 | 说明 |
|---------------|-------------|---------|------|
| `true` | `true` | `[start, end]` | 闭区间（默认） |
| `false` | `true` | `(start, end]` | 左开右闭 |
| `true` | `false` | `[start, end)` | 左闭右开 |
| `false` | `false` | `(start, end)` | 开区间 |

---

## 5. 公开 API 说明

### 5.1 构造函数

#### `New[K cmp.Ordered, V any](configs ...*Config) (*SkipList[K, V], error)`

创建一个新的跳表实例。

- **参数**：可选的 `Config` 配置，不传则使用默认配置
- **返回**：跳表指针和错误（配置非法时返回错误）

**示例：**
```go
// 使用默认配置
sl, err := skiplist.New[int, string]()

// 自定义配置（高概率因子，层数更多）
cfg := &skiplist.Config{MaxLevel: 16, P: 0.5}
sl, err := skiplist.New[string, int](cfg)
```

### 5.2 基础操作

#### `Insert(key K, value V)`

插入或更新键值对。如果 Key 已存在，则更新对应的 Value。

#### `Delete(key K) (V, bool)`

删除指定 Key 的节点。返回被删除的值和是否成功删除。

#### `Search(key K) (V, bool)`

精确查找指定 Key。返回对应的值和是否存在。

#### `Contains(key K) bool`

判断 Key 是否存在。

### 5.3 范围查询

#### `Range(start, end K, opts ...*RangeOptions) []Pair[K, V]`

查询 `[start, end]`（可配置开闭）范围内的所有键值对，按键升序返回。

- 支持 `start > end`（返回空）
- 支持完全在范围外的查询（返回空）
- 支持分页（Offset + Limit）

#### `All() []Pair[K, V]`

返回跳表中的所有键值对，等价于 `Range(minKey, maxKey)`。

### 5.4 辅助方法

| 方法 | 说明 |
|------|------|
| `Len() int` | 返回节点数量 |
| `Level() int` | 返回当前实际使用的最大层数 |
| `First() (Pair[K, V], bool)` | 返回最小 Key 的键值对（O(1)） |
| `Last() (Pair[K, V], bool)` | 返回最大 Key 的键值对（O(1)） |
| `Clear()` | 清空跳表，重置为初始状态 |
| `SetRandomSeed(seed int64)` | 重置内部随机数源为使用指定种子的新生成器（内部加写锁，线程安全） |

---

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
	"fmt"
	"solocoder-go/internal/skiplist"
)

func main() {
	// 创建跳表：int 键，string 值
	sl, err := skiplist.New[int, string]()
	if err != nil {
		panic(err)
	}

	// 插入（乱序插入也会自动排序）
	sl.Insert(3, "three")
	sl.Insert(1, "one")
	sl.Insert(2, "two")
	sl.Insert(5, "five")
	sl.Insert(4, "four")

	// 精确查找
	val, ok := sl.Search(3)
	if ok {
		fmt.Println("Found:", val) // "three"
	}

	// 更新已存在的键
	sl.Insert(3, "THREE")
	val, _ = sl.Search(3)
	fmt.Println("Updated:", val) // "THREE"

	// 删除
	oldVal, ok := sl.Delete(5)
	if ok {
		fmt.Println("Deleted:", oldVal) // "five"
	}

	// 元素数量
	fmt.Println("Length:", sl.Len()) // 4
}
```

### 6.2 范围查询示例

```go
func demoRange() {
	sl, _ := skiplist.New[int, string]()
	for i := 1; i <= 100; i++ {
		sl.Insert(i, fmt.Sprintf("val-%d", i))
	}

	// 1. 闭区间 [10, 20]
	r1 := sl.Range(10, 20)
	fmt.Println(len(r1)) // 11 条 (10..20)

	// 2. 开区间 (10, 20)
	opts := skiplist.DefaultRangeOptions().
		WithStartInclusive(false).
		WithEndInclusive(false)
	r2 := sl.Range(10, 20, opts)
	fmt.Println(len(r2)) // 9 条 (11..19)

	// 3. 分页：第 3 页，每页 10 条（跳过 20，取 10）
	pageOpts := skiplist.DefaultRangeOptions().
		WithOffset(20).
		WithLimit(10)
	r3 := sl.Range(1, 100, pageOpts)
	fmt.Println(len(r3))           // 10 条
	fmt.Println(r3[0].Key)         // 21
	fmt.Println(r3[len(r3)-1].Key) // 30

	// 4. 左闭右开 [50, 60)，常用于时间区间
	timeOpts := skiplist.DefaultRangeOptions().WithEndInclusive(false)
	r4 := sl.Range(50, 60, timeOpts)
	fmt.Println(len(r4)) // 10 条 (50..59)
}
```

### 6.3 字符串键 + 自定义概率

```go
func demoStringAndProb() {
	// P=0.5，更多层，适合读密集场景
	cfg := &skiplist.Config{MaxLevel: 32, P: 0.5}
	sl, _ := skiplist.New[string, int](cfg)

	words := []string{"banana", "apple", "cherry", "date"}
	for i, w := range words {
		sl.Insert(w, i+1)
	}

	// 按字典序遍历
	for _, p := range sl.All() {
		fmt.Printf("%s: %d\n", p.Key, p.Value)
	}
	// 输出（字典序）：
	// apple: 2
	// banana: 1
	// cherry: 3
	// date: 4

	// 范围查询字典序在 [a, c) 的单词
	opts := skiplist.DefaultRangeOptions().WithEndInclusive(false)
	result := sl.Range("a", "c", opts)
	for _, p := range result {
		fmt.Println(p.Key) // "apple", "banana"
	}
}
```

---

## 7. 并发安全说明

本跳表实现通过 `sync.RWMutex` 保证并发安全：

- **读操作（共享锁）**：`Search`、`Range`、`All`、`Contains`、`First`、`Last`、`Len`、`Level`
  - 多个 goroutine 可同时执行读操作
- **写操作（排他锁）**：`Insert`、`Delete`、`Clear`
  - 写操作与任何其他操作互斥

**建议：** 对于超高并发的极端场景，可考虑分桶分片（分片跳表）进一步提升吞吐。

---

## 8. 测试覆盖

单元测试文件位于 `internal/skiplist/skiplist_test.go`，覆盖以下场景：

| 测试分类 | 覆盖内容 |
|---------|---------|
| **构造** | 默认配置、自定义配置、非法配置（MaxLevel/P 越界） |
| **插入与查找** | 乱序插入、重复插入（更新）、存在/不存在的查找 |
| **删除** | 存在/不存在的删除、空表删除、删除全部元素、删除后层级收敛 |
| **范围查询** | 全区间、部分区间、空区间（start>end）、越界区间、单元素区间 |
| **开闭区间** | 4 种区间组合的正确性验证 |
| **分页** | Limit 限制、Offset 偏移、Limit+Offset 组合、Offset>=总数、Limit=0（无限制） |
| **边界** | 单元素操作、负整数范围、边界附近的查询、First/Last/Clear |
| **类型支持** | int 键、string 键、float64 键 |
| **概率配置** | 使用 `Config.RandomSource` 或 `SetRandomSeed` 公开 API 注入固定种子 RNG 使测试完全确定性（不再直接访问未导出字段）；严格多层级分布验证：对 P=0.5 和 P=0.25 各取 50000 次采样，校验 level≥2、≥3、≥4 的比例与理论期望值 P、P²、P³ 的偏差在 ±0.015 容差以内；验证 P=0.9 层数 > P=0.01 层数；遍历 P∈{0.01,0.25,0.5,0.9} 下 All() 的有序性；校验 SetRandomSeed 与 RandomSource 的行为等价性 |
| **压力** | 10000 条大数据量插入/查找/范围 |
| **并发** | 多 goroutine 同时 Insert/Search，验证并发完成后元素总数、逐键查找正确性、All() 遍历有序性与值一致性 |
| **基准测试** | Insert/Search/Delete 的 Benchmark |

运行测试：
```bash
go test ./internal/skiplist/ -v
go test ./internal/skiplist/ -bench=. -benchmem
```
