# 可扩展布隆过滤器 (ScalableBloom) 模块需求文档

## 1. 模块概述

可扩展布隆过滤器是一个支持动态容量扩展的概率型数据结构，用于高效判断元素是否"可能存在"于集合中。当内层过滤器容量耗尽时自动创建新的过滤器层以扩展总体容量，查询时依次检查所有内层过滤器，只要任意一层返回可能存在即判定为可能存在。同时支持哈希函数数量的自适应计算、过滤器状态的序列化持久化，以及多过滤器联合查询。

本模块适用于大规模去重、URL 爬虫去重、缓存穿透防护等元素数量事先不确定或持续增长的场景，通过分层扩展机制在内存使用和误判率之间取得平衡。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 元素插入 (Add) | 向布隆过滤器插入元素，当前内层过滤器容量耗尽时自动扩展新层 |
| F2 | 存在性查询 (MightContain) | 查询元素是否可能存在，依次检查所有内层过滤器，任一层命中即返回 true |
| F3 | 动态容量扩展 | 当当前内层过滤器的已插入数量达到其设计容量时，自动创建新的内层过滤器 |
| F4 | 哈希参数自适应计算 | 根据目标误判率和预期元素数量自动计算最优哈希函数个数和位数组大小，扩展时新层使用更严格的参数 |
| F5 | 序列化持久化 (Serialize) | 将布隆过滤器的全部状态序列化为二进制格式写入文件 |
| F6 | 反序列化恢复 (Deserialize) | 从序列化文件反序列化恢复布隆过滤器到内存继续使用 |
| F7 | 多过滤器联合查询 (UnionQuery) | 判断元素是否存在于多个 ScalableBloom 实例中的任意一个 |
| F8 | 元素计数 (Count) | 返回已插入的元素总数 |
| F9 | 过滤器层数 (FilterCount) | 返回当前内层过滤器的数量 |
| F10 | 总容量 (Capacity) | 返回所有内层过滤器的设计容量之和 |
| F11 | 线程安全 | 所有公共方法通过互斥锁保护，支持并发访问 |

## 3. 核心结构体与职责

### 3.1 Config - 过滤器配置

```go
type Config struct {
    InitialCapacity uint    // 初始内层过滤器的预期元素数量
    FPRate          float64 // 目标误判率（必须为 (0, 1) 之间的值）
    Ratio           float64 // 误判率收紧比例（必须为 (0, 1) 之间的值）
}
```

**配置约束与默认值：**
- `InitialCapacity`：必须 > 0。设置为 0 时返回 `ErrInvalidCapacity`
- `FPRate`：必须在 (0, 1) 范围内。设置为 ≤0 或 ≥1 时返回 `ErrInvalidFPRate`
- `Ratio`：必须在 (0, 1) 范围内。设置为 ≤0 或 ≥1 时返回 `ErrInvalidRatio`。该值控制每次扩展时新层误判率相对于初始误判率的收紧程度
- 默认配置：`InitialCapacity=1000, FPRate=0.01, Ratio=0.85`

### 3.2 bloomFilter - 内层布隆过滤器

```go
type bloomFilter struct {
    bits      []uint64 // 位数组，使用 uint64 切片存储以节省内存
    numBits   uint     // 位数组的总位数
    hashCount uint     // 哈希函数数量
    capacity  uint     // 设计容量（预期元素数量）
    count     uint     // 已插入元素数量
}
```

**主要职责：**
- 维护一个固定大小的位数组和哈希函数配置
- 提供元素插入 (`add`) 和存在性查询 (`mightContain`) 操作
- 使用双哈希技术 (double hashing) 从 SHA-256 派生多个哈希值
- 通过 `isFull()` 判断是否已达到设计容量

### 3.3 ScalableBloom - 可扩展布隆过滤器主体

```go
type ScalableBloom struct {
    mu      sync.Mutex       // 保护内部状态的互斥锁
    filters []*bloomFilter   // 内层过滤器列表（按创建顺序排列）
    cfg     Config           // 配置快照
    count   uint             // 已插入元素总数
}
```

**主要职责：**
- 管理多个内层布隆过滤器，在当前层满时自动创建新层
- 查询时依次遍历所有内层过滤器，实现"并集"查询语义
- 维护全局元素计数
- 保证线程安全，通过互斥锁保护所有内部状态访问
- 支持将完整状态序列化到文件和从文件恢复

### 3.4 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidFPRate` | 误判率无效 | `FPRate` ≤ 0 或 ≥ 1 |
| `ErrInvalidCapacity` | 容量无效 | `InitialCapacity` = 0 |
| `ErrInvalidRatio` | 比例无效 | `Ratio` ≤ 0 或 ≥ 1 |
| `ErrEmptyKey` | 键为空 | 调用 `Add("")` 或 `MightContain("")` |
| `ErrNoFilters` | 无过滤器 | 调用 `UnionQuery` 传入空切片 |
| `ErrFileOpen` | 文件打开失败 | 序列化时无法创建文件 |
| `ErrFileWrite` | 文件写入失败 | 序列化时写入操作出错 |
| `ErrFileRead` | 文件读取失败 | 反序列化时无法打开文件 |
| `ErrInvalidData` | 数据无效 | 反序列化时数据格式损坏或截断 |
| `ErrVersionMismatch` | 版本不匹配 | 反序列化时文件版本与当前版本不一致 |

## 4. 核心机制详解

### 4.1 动态容量扩展

```
Add(key)
   │
   ├─ key == "" → 返回 ErrEmptyKey
   │
   ├─ mu.Lock()
   │
   ├─ 取当前活跃过滤器 active = filters 的最后一个
   │
   ├─ active.isFull()？
   │     ├─ 否 → 直接向 active 插入
   │     └─ 是 → 创建新的内层过滤器
   │           ├─ newCapacity = active.capacity × 2
   │           ├─ newFPRate = FPRate × Ratio^(len(filters))
   │           ├─ newFilter = newBloomFilter(newCapacity, newFPRate)
   │           ├─ append filters
   │           └─ active = newFilter
   │
   ├─ active.add(key)
   ├─ count++
   └─ mu.Unlock()
```

**扩展策略说明：**
- 容量翻倍：每层新过滤器的容量是前一层的 2 倍，适应指数级增长
- 误判率收紧：第 i 层过滤器的目标误判率为 `FPRate × Ratio^i`，随着层数增加误判率逐层收紧
- 整体误判率：`P_total = 1 - ∏(1 - P_i)`，由于各层 P_i 递减，总体误判率可控

### 4.2 哈希参数自适应计算

**最优位数组大小：**

```
m = -n × ln(p) / (ln2)²
```

其中 n 为预期元素数量，p 为目标误判率。

**最优哈希函数数量：**

```
k = (m / n) × ln2
```

其中 m 为位数组大小，n 为预期元素数量。k 最小值为 1。

**扩展时参数更新：**

| 层级 | 容量 | 目标误判率 | 位数组大小 | 哈希函数数 |
|------|------|-----------|-----------|-----------|
| 0 | InitialCapacity | FPRate | m₀ = -n₀ × ln(p₀) / (ln2)² | k₀ = (m₀/n₀) × ln2 |
| 1 | 2 × InitialCapacity | FPRate × Ratio | m₁ = -n₁ × ln(p₁) / (ln2)² | k₁ = (m₁/n₁) × ln2 |
| i | 2^i × InitialCapacity | FPRate × Ratio^i | mᵢ = -nᵢ × ln(pᵢ) / (ln2)² | kᵢ = (mᵢ/nᵢ) × ln2 |

### 4.3 存在性查询

```
MightContain(key)
   │
   ├─ key == "" → 返回 (false, ErrEmptyKey)
   │
   ├─ mu.Lock()
   │
   └─ 遍历所有内层过滤器：
         ├─ 任一层 mightContain(key) == true → 返回 (true, nil)
         │
         └─ 全部返回 false → 返回 (false, nil)
```

**关键特性：**
- 不存在假阴性：已插入的元素一定会被查询到（跨所有层）
- 可能存在假阳性：未插入的元素有小概率被误判为存在
- 布隆过滤器适合"允许少量误判但不允许遗漏"的场景

### 4.4 序列化格式

二进制序列化格式如下（Big Endian 字节序）：

| 字段 | 类型 | 大小 | 说明 |
|------|------|------|------|
| version | uint32 | 4 | 格式版本号，当前为 1 |
| initialCapacity | uint32 | 4 | 初始容量 |
| fpRate | float64 | 8 | 目标误判率 |
| ratio | float64 | 8 | 误判率收紧比例 |
| count | uint32 | 4 | 已插入元素总数 |
| numFilters | uint32 | 4 | 内层过滤器数量 |
| **过滤器数据** | | | 每个过滤器重复以下结构 |
| numBits | uint32 | 4 | 位数组总位数 |
| hashCount | uint32 | 4 | 哈希函数数量 |
| capacity | uint32 | 4 | 设计容量 |
| filterCount | uint32 | 4 | 已插入元素数 |
| numWords | uint32 | 4 | uint64 切片长度 |
| bits | []uint64 | numWords × 8 | 位数组数据 |

### 4.5 多过滤器联合查询

```
UnionQuery(filters, key)
   │
   ├─ len(filters) == 0 → 返回 (false, ErrNoFilters)
   ├─ key == "" → 返回 (false, ErrEmptyKey)
   │
   └─ 遍历所有 ScalableBloom 实例：
         ├─ 任一实例 MightContain(key) == true → 返回 (true, nil)
         │
         └─ 全部返回 false → 返回 (false, nil)
```

**联合查询语义：**
- 逻辑 OR：任一过滤器返回可能存在 → 整体返回可能存在
- 所有过滤器均返回肯定不存在 → 整体才返回肯定不存在
- 短路优化：遇到第一个"可能存在"即返回，不继续检查后续过滤器

## 5. 线程安全设计

所有公共方法均通过互斥锁 `mu` 保护内部状态：
- **写操作**（`Add`、`Serialize`）：获取排他锁
- **读操作**（`MightContain`、`Count`、`FilterCount`、`Capacity`）：同样获取排他锁
- `Deserialize` 是包级函数，创建新实例时无需加锁
- `UnionQuery` 是包级函数，内部调用各实例的 `MightContain` 方法（各实例自行加锁）

## 6. 使用示例

### 6.1 基础使用

```go
package main

import (
    "errors"
    "fmt"
    "solocoder-go/internal/scalablebloom"
)

func main() {
    cfg := scalablebloom.Config{
        InitialCapacity: 1000,
        FPRate:          0.01,
        Ratio:           0.85,
    }
    sb, err := scalablebloom.New(cfg)
    if err != nil {
        panic(err)
    }

    sb.Add("user-123")
    sb.Add("user-456")

    found, err := sb.MightContain("user-123")
    if err != nil {
        panic(err)
    }
    fmt.Printf("user-123 exists: %v\n", found) // true

    found, _ = sb.MightContain("user-999")
    fmt.Printf("user-999 exists: %v\n", found) // false (probably)
}
```

### 6.2 自动扩展

```go
cfg := scalablebloom.Config{
    InitialCapacity: 100,
    FPRate:          0.01,
    Ratio:           0.85,
}
sb, _ := scalablebloom.New(cfg)

fmt.Printf("Filters: %d, Capacity: %d\n", sb.FilterCount(), sb.Capacity())
// Filters: 1, Capacity: 100

for i := 0; i < 150; i++ {
    sb.Add(fmt.Sprintf("item-%d", i))
}

fmt.Printf("Filters: %d, Capacity: %d\n", sb.FilterCount(), sb.Capacity())
// Filters: 2, Capacity: 300 (100 + 200)

// 所有已插入元素仍然可查询
for i := 0; i < 150; i++ {
    found, _ := sb.MightContain(fmt.Sprintf("item-%d", i))
    if !found {
        panic("false negative!")
    }
}
```

### 6.3 序列化与反序列化

```go
sb, _ := scalablebloom.New(scalablebloom.DefaultConfig())
for i := 0; i < 500; i++ {
    sb.Add(fmt.Sprintf("key-%d", i))
}

// 持久化到文件
err := sb.Serialize("bloom.dat")
if err != nil {
    panic(err)
}

// 从文件恢复
loaded, err := scalablebloom.Deserialize("bloom.dat")
if err != nil {
    panic(err)
}

// 继续使用
loaded.Add("new-key")
found, _ := loaded.MightContain("new-key")
fmt.Printf("new-key exists: %v\n", found) // true
```

### 6.4 多过滤器联合查询

```go
// 不同业务线的布隆过滤器
userFilter, _ := scalablebloom.New(scalablebloom.DefaultConfig())
orderFilter, _ := scalablebloom.New(scalablebloom.DefaultConfig())

userFilter.Add("id-abc")
orderFilter.Add("id-xyz")

// 联合查询：在任意过滤器中存在即可
found, err := scalablebloom.UnionQuery(
    []*scalablebloom.ScalableBloom{userFilter, orderFilter},
    "id-abc",
)
if err != nil {
    panic(err)
}
fmt.Printf("id-abc in any filter: %v\n", found) // true

found, _ = scalablebloom.UnionQuery(
    []*scalablebloom.ScalableBloom{userFilter, orderFilter},
    "id-xyz",
)
fmt.Printf("id-xyz in any filter: %v\n", found) // true

found, _ = scalablebloom.UnionQuery(
    []*scalablebloom.ScalableBloom{userFilter, orderFilter},
    "id-not-exist",
)
fmt.Printf("id-not-exist in any filter: %v\n", found) // false
```

## 7. 文件结构

```
internal/scalablebloom/
├── scalablebloom.go      # 可扩展布隆过滤器核心实现
└── scalablebloom_test.go # 单元测试（覆盖正常流程、边界条件、异常分支、并发场景）

docs/
└── scalablebloom.md      # 本文档
```

## 8. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **基础创建** | `TestNew_DefaultConfig`、`TestDefaultConfig_Values` | 构造函数、默认值验证 |
| **参数校验** | `TestNew_InvalidCapacity`、`TestNew_InvalidFPRate_*`、`TestNew_InvalidRatio_*` | 无效配置返回对应错误 |
| **元素插入** | `TestAdd_SingleElement`、`TestAdd_MultipleElements`、`TestAdd_EmptyKey` | 正常插入、空键拒绝 |
| **存在性查询** | `TestMightContain_ExistingElement`、`TestMightContain_NonExistingElement` | 查询正确性、空键拒绝 |
| **无假阴性** | `TestMightContain_NoFalseNegatives` | 已插入元素必须可查询 |
| **动态扩展** | `TestDynamicExpansion_Triggered`、`TestDynamicExpansion_MultipleExpansions`、`TestDynamicExpansion_QueryAfterExpansion` | 扩展触发、多次扩展、扩展后查询 |
| **参数自适应** | `TestDynamicExpansion_NewFilterTighterFPRate`、`TestExpansion_NewFilterParams` | 新层容量翻倍、误判率收紧 |
| **误判率验证** | `TestFalsePositiveRate_WithinBounds` | 实际误判率在合理范围内 |
| **序列化** | `TestSerializeDeserialize_BasicRoundTrip`、`TestSerializeDeserialize_AfterExpansion` | 基本往返、扩展后序列化 |
| **反序列化** | `TestDeserialize_ConfigPreserved`、`TestDeserialize_ContinueUsingAfterLoad` | 配置保留、恢复后继续使用 |
| **序列化异常** | `TestSerialize_InvalidPath`、`TestDeserialize_NonexistentFile`、`TestDeserialize_InvalidData`、`TestDeserialize_VersionMismatch`、`TestDeserialize_CorruptedData` | 路径无效、文件不存在、数据损坏、版本不匹配 |
| **联合查询** | `TestUnionQuery_BasicFound`、`TestUnionQuery_NotFound`、`TestUnionQuery_NoFilters`、`TestUnionQuery_ShortCircuitOnFound` | 正常查找、未找到、空列表、短路优化 |
| **并发安全** | `TestConcurrent_AddAndQuery`、`TestConcurrent_SerializeAndAdd` | 并发读写、并发序列化与插入 |
| **边界条件** | `TestNew_SmallCapacity`、`TestNew_VeryLowFPRate`、`TestOptimalHashCount_MinimumOne` | 极小容量、极低误判率、哈希数下限 |
