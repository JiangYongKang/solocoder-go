# ColStore 列式数据存储模块

## 1. 模块概述

ColStore 是一个高性能的内存列式数据存储模块，专为分析型查询场景设计。模块采用列式存储布局，将数据按列而非行进行组织，配合可选的字典编码压缩、列裁剪读取和谓词下推过滤等优化技术，大幅提升大数据量下的查询性能。

**包路径**: `internal/colstore`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 按列批量写入 | 以列为单位批量追加数据，同一批次写入具备原子性保证 |
| 列裁剪读取 | 按需指定返回列集合，仅读取请求的列，跳过无关列 |
| 字典编码压缩 | 可配置开启/关闭，开启时自动对列中重复值构建字典映射 |
| 谓词下推过滤 | 在存储层执行过滤条件，扫描阶段即过滤不满足条件的行 |
| 严格类型比较 | 谓词过滤对类型不匹配的值严格判定为不相等，避免误匹配 |
| 并发安全 | 支持多 goroutine 并发读写，内置全局读写锁保护 |

## 3. 核心结构体与职责

### 3.1 ColumnStore

列式存储主引擎，对外提供所有操作接口。

```go
type ColumnStore struct {
    columns     map[string]*Column
    colOrder    []string
    rowCount    int
    mu          sync.RWMutex
    closed      bool
    dictEnabled bool
}
```

**职责**:
- 管理所有列的元数据和存储引用
- 维护列的创建顺序（`colOrder`）
- 追踪总行数
- 保存全局字典编码开关配置 `dictEnabled`
- 通过 `sync.RWMutex` 保护并发访问
- 协调批量写入、查询、过滤等跨列操作

### 3.2 Column

单列数据存储单元，根据配置选择编码模式或原始模式。

```go
type Column struct {
    name        string
    dictEnabled bool
    dict        map[Value]int
    reverseDict []Value
    encodedData []int
    rawData     []Value
}
```

**职责**:
- `dictEnabled`: 该列是否启用字典编码（继承自 ColumnStore 配置）
- **字典编码模式（dictEnabled=true）**:
  - `dict`: 值到字典下标的正向映射（编码用）
  - `reverseDict`: 字典下标到值的反向映射（解码用）
  - `encodedData`: 存储编码后的整数下标数组
- **原始存储模式（dictEnabled=false）**:
  - `rawData`: 直接存储原始值，不进行编解码
- `encode(v)` / `decode(idx)`: 字典编解码（仅编码模式下使用）
- `appendValues(values)`: 批量追加值，根据模式选择存储方式
- `getValueAt(rowIdx)`: 根据模式返回对应值

> **设计说明**: Column 内部不再维护独立的 `sync.RWMutex`。所有列级方法均在 ColumnStore 已持有全局锁的前提下调用，避免了不必要的双重加锁开销。

### 3.3 ColumnBatch

批量写入时的单列数据结构。

```go
type ColumnBatch struct {
    Name   string
    Values []Value
}
```

**职责**:
- 描述一次批量写入中某一列的数据
- `Name`: 列名
- `Values`: 该列本次写入的所有值

### 3.4 Predicate

谓词过滤条件，用于谓词下推。

```go
type Predicate struct {
    Column string
    Op     Operator
    Value  Value
    Values []Value
}
```

**职责**:
- `Column`: 要过滤的列名
- `Op`: 比较运算符（见 Operator 常量）
- `Value`: 单值比较的右操作数（=, !=, >, >=, <, <=）
- `Values`: 多值比较的操作数集合（IN, NOT IN）

### 3.5 Operator

支持的谓词运算符。

```go
const (
    OpEq    Operator = "="       // 等于
    OpNeq   Operator = "!="      // 不等于
    OpGt    Operator = ">"       // 大于
    OpGte   Operator = ">="      // 大于等于
    OpLt    Operator = "<"       // 小于
    OpLte   Operator = "<="      // 小于等于
    OpIn    Operator = "IN"      // 包含于集合
    OpNotIn Operator = "NOT IN"  // 不包含于集合
)
```

### 3.6 Row

查询结果中的一行数据。

```go
type Row struct {
    Values map[string]Value
}
```

**职责**:
- 以 `列名 -> 值` 的映射形式存储一行的投影列数据
- 仅包含查询时请求的列

### 3.7 QueryResult

查询结果封装。

```go
type QueryResult struct {
    Rows         []*Row
    Columns      []string
    TotalScanned int
    TotalMatched int
}
```

**职责**:
- `Rows`: 匹配的行数据列表
- `Columns`: 投影列名（按请求顺序）
- `TotalScanned`: 扫描过的总行数
- `TotalMatched`: 满足过滤条件的行数

### 3.8 Config

模块配置。

```go
type Config struct {
    DictionaryEnabled bool
}
```

**职责**:
- `DictionaryEnabled`: 是否启用字典编码压缩（默认 true）

**配置行为说明**:
- `true`: 每列维护字典映射，将值编码为整数下标存储。适合列值重复度高的场景，可显著节省内存。
- `false`: 直接存储原始值，不构建字典。适合唯一值多、重复度低的场景，避免字典带来的额外开销。
- 配置在创建 ColumnStore 时指定，创建后不可修改。所有列共享同一配置。
- `DictionarySize()` 在禁用时返回 0。

## 4. 数据流程：从写入到查询

### 4.1 列式写入流程

```
调用 Write(batch []ColumnBatch)
        │
        ▼
  ┌─────────────────┐
  │ 1. 参数校验     │
  │    - 批次非空    │
  │    - 列名不重复  │
  │    - 所有列行数一致│
  └────────┬────────┘
           │
           ▼
  ┌─────────────────┐
  │ 2. 列初始化     │
  │    对每个列名：   │
  │    不存在则创建 Column │
  │    (继承全局 dictEnabled)│
  └────────┬────────┘
           │
           ▼
  ┌─────────────────────────────┐
  │ 3. 按配置选择存储模式        │
  │                              │
  │  dictEnabled=true ?         │
  │    ┌────┴────┐              │
  │    是        否             │
  │    │         │              │
  │    ▼         ▼              │
  │  字典编码   原始存储        │
  │  encode(v)  rawData=append  │
  └──────────────┬──────────────┘
                 │
                 ▼
  ┌─────────────────┐
  │ 4. 更新 rowCount │
  │    原子性完成    │
  └─────────────────┘
```

**原子性保证**:
- 所有校验在任何修改前完成
- 任一列校验失败，整个批次回滚（无任何副作用）
- 全程持有写锁，其他 goroutine 不会看到中间状态

### 4.2 字典编码流程（dictEnabled=true）

```
写入值 "category_A"
        │
        ▼
 dict 中存在 "category_A" ?
    ┌───┴───┐
   是       否
   │        │
   ▼        ▼
返回下标  分配新下标 idx = len(reverseDict)
          dict["category_A"] = idx
          reverseDict = append(reverseDict, "category_A")
          返回 idx
        │
        ▼
 encodedData = append(encodedData, idx)  // 仅存整数！
```

**压缩效果示例**:
- 原始：10000 个字符串 "category_A"，每个约 12 字节 → 约 120KB
- 编码后：10000 个 int（4 字节）+ 字典 1 条 → 约 40KB + 少量开销
- 压缩率：约 67%，重复度越高效果越好

### 4.3 原始存储流程（dictEnabled=false）

```
写入值 any_value
        │
        ▼
 rawData = append(rawData, any_value)  // 直接存储，无编解码
```

**适用场景**:
- 列值几乎没有重复（如主键列、UUID 列）
- 字典编码的 map 和 reverseDict 开销超过压缩收益
- 需要避免任何编解码 CPU 开销

### 4.4 列裁剪读取流程

```
Read(columns=["id", "name"])
        │
        ▼
  ┌─────────────────────┐
  │ 1. 校验列名有效性    │
  └──────────┬──────────┘
             │
             ▼
  ┌─────────────────────┐
  │ 2. 谓词下推过滤      │
  │    对每一行：         │
  │    evaluatePredicates│
  │    收集匹配行索引     │
  └──────────┬──────────┘
             │
             ▼
  ┌─────────────────────────────────┐
  │ 3. 仅读取请求列                  │
  │    对每个匹配行 i：               │
  │    dictEnabled ?                 │
  │      是: decode(encodedData[i])  │
  │      否: rawData[i]              │
  │    **跳过 age, score 等其他列**  │
  └──────────────┬──────────────────┘
                 │
                 ▼
  组装 QueryResult 返回
```

**列裁剪优势**:
- 减少内存访问：仅遍历请求列的存储数组
- 减少解码开销（编码模式下）：仅解码请求列的字典值
- 减少返回数据量：结果仅含需要的列

### 4.5 谓词下推过滤流程

```
ReadWithFilter(columns, predicates)
        │
        ▼
  ┌──────────────────────────┐
  │ 存储层内部扫描每一行 i：   │
  │                          │
  │  for i in [0, rowCount): │
  │    if 所有谓词匹配:       │
  │      将 i 加入匹配集      │
  │                          │
  │  ** 过滤发生在存储层 **   │
  │  ** 仅返回匹配行给上层 ** │
  └──────────────┬───────────┘
                 │
                 ▼
  上层调用者仅收到匹配行
  无需再做过滤
```

**谓词下推优势**:
- 减少层间数据传输：仅传递匹配行
- 减少上层 CPU 开销：上层无需再遍历过滤
- 利用存储层局部性：列数据连续存储，缓存友好

### 4.6 完整数据生命周期

```
           写入阶段 (dictEnabled=true)         查询阶段
┌───────────────────────────────┐    ┌──────────────────────────────┐
│                               │    │                              │
│  ColumnBatch{                 │    │  ReadWithFilter(             │
│    Name: "city",              │    │    columns=["name","city"],  │
│    Values: ["NYC",            │───▶│    predicates=[              │
│             "LA",             │    │      {city, =, "NYC"}        │
│             "NYC"]            │    │    ])                        │
│  }                            │    │                              │
│           │                   │    │             │                │
│           ▼                   │    │             ▼                │
│  dict: NYC→0, LA→1            │    │  存储层扫描：                 │
│  reverseDict: [NYC, LA]       │    │    row0: decode(0)=NYC ✓匹配 │
│  encodedData: [0, 1, 0]       │    │    row1: decode(1)=LA  ✗过滤 │
│                               │    │    row2: decode(0)=NYC ✓匹配 │
└───────────────────────────────┘    │                              │
                                     │  列裁剪读取：                 │
                                     │    row0: name=..., city=NYC  │
                                     │    row2: name=..., city=NYC  │
                                     │                              │
                                     │  返回 2 行（非 3 行）         │
                                     └──────────────────────────────┘
```

## 5. 并发安全设计

### 5.1 锁层级

```
ColumnStore.mu (sync.RWMutex)  ← 全局唯一锁
    │
    ├─ Write(): 持有写锁
    │   └─ Column.appendValues(): 直接操作，无内部锁
    │
    ├─ Read() / ReadWithFilter(): 持有读锁
    │   └─ Column.getValueAt(): 直接操作，无内部锁
    │
    └─ Close(): 持有写锁
```

> **优化说明**: Column 结构体不再维护独立的 `sync.RWMutex`。由于所有列级方法均在 ColumnStore 全局锁的保护下执行，列级锁构成了不必要的双重加锁，既增加内存占用又带来额外的锁获取/释放开销。移除后实现更简洁、性能更优。

### 5.2 并发语义

| 操作组合 | 并发安全性 | 说明 |
|----------|-----------|------|
| 读 + 读 | ✅ 完全并行 | 多个读操作共享读锁 |
| 读 + 写 | ❌ 串行 | 写操作需获取写锁，阻塞所有读 |
| 写 + 写 | ❌ 串行 | 写操作互斥执行 |

## 6. 谓词过滤的类型兼容性约定

### 6.1 比较规则

`compareValues(a, b)` 函数遵循以下严格类型比较规则：

| 比较场景 | 行为 | 示例 |
|----------|------|------|
| 同类型比较 | 按类型语义比较 | `int(1) < int(2)` → true |
| `int` ↔ `float64` | 自动类型提升后比较 | `int(1) == float64(1.0)` → true |
| 其他跨类型比较 | **严格判定为不相等**，并按类型名字典序确定大小关系 | `int(1) == string("1")` → **false** |
| `nil` 比较 | `nil == nil`，`nil < 任何非nil` | `nil == nil` → true |

### 6.2 类型不匹配的行为示例

| 值 A | 值 B | `OpEq` 结果 | 说明 |
|------|------|------------|------|
| `int(1)` | `string("1")` | ❌ 不匹配 | 类型不同，即使字符串表示相同也不等 |
| `bool(true)` | `int(1)` | ❌ 不匹配 | bool 与 int 不兼容 |
| `string("true")` | `bool(true)` | ❌ 不匹配 | 字符串与 bool 不兼容 |
| `float64(1.0)` | `string("1.0")` | ❌ 不匹配 | 浮点数与字符串不兼容 |
| `int(2)` | `float64(2.0)` | ✅ 匹配 | int 与 float64 兼容，自动提升 |

> **设计动机**: 避免因隐式类型转换导致的谓词误匹配。例如 `int(1)` 与 `string("1")` 在语义上是完全不同的值，不应被误判为相等。仅 `int` 和 `float64` 之间允许自动类型提升，这是数据库系统中的常见约定。

## 7. 使用示例

### 7.1 基本使用（默认启用字典编码）

```go
package main

import (
    "fmt"
    "solocoder-go/internal/colstore"
)

func main() {
    cs := colstore.NewColumnStore() // DictionaryEnabled=true (默认)

    batch := []colstore.ColumnBatch{
        {Name: "id", Values: []colstore.Value{1, 2, 3}},
        {Name: "name", Values: []colstore.Value{"Alice", "Bob", "Charlie"}},
        {Name: "age", Values: []colstore.Value{30, 25, 35}},
        {Name: "city", Values: []colstore.Value{"NYC", "LA", "NYC"}},
    }

    if err := cs.Write(batch); err != nil {
        panic(err)
    }

    fmt.Println("Dictionary enabled:", cs.IsDictionaryEnabled()) // true
    fmt.Println("City column unique values:", func() int {
        s, _ := cs.DictionarySize("city")
        return s
    }()) // 2 (NYC, LA)
    fmt.Println("Total rows:", cs.RowCount())
}
```

### 7.2 禁用字典编码

```go
// 主键列、UUID 列等重复度极低的场景，建议禁用字典编码
cfg := colstore.Config{DictionaryEnabled: false}
cs := colstore.NewColumnStoreWithConfig(cfg)

fmt.Println("Dictionary enabled:", cs.IsDictionaryEnabled()) // false

uuids := make([]colstore.Value, 1000)
for i := range uuids {
    uuids[i] = fmt.Sprintf("uuid-%d", i) // 每个值唯一
}
cs.Write([]colstore.ColumnBatch{
    {Name: "uuid", Values: uuids},
})

// 禁用时 DictionarySize 始终返回 0
size, _ := cs.DictionarySize("uuid")
fmt.Println("Dictionary size (disabled):", size) // 0

// 数据读写行为与启用时完全一致
result, _ := cs.Read([]string{"uuid"})
fmt.Println("Rows read:", len(result.Rows)) // 1000
```

### 7.3 列裁剪读取

```go
result, err := cs.Read([]string{"id", "name"})
if err != nil {
    panic(err)
}

fmt.Printf("Scanned: %d, Matched: %d\n", result.TotalScanned, result.TotalMatched)
for _, row := range result.Rows {
    fmt.Printf("id=%v, name=%v\n", row.Values["id"], row.Values["name"])
    // row.Values["age"] 不存在，已被裁剪
}
```

### 7.4 谓词下推过滤（注意类型匹配）

```go
predicates := []*colstore.Predicate{
    {Column: "city", Op: colstore.OpEq, Value: "NYC"},     // ✅ string vs string
    {Column: "age", Op: colstore.OpGte, Value: 30},        // ✅ int vs int
}

result, err := cs.ReadWithFilter([]string{"id", "name"}, predicates)
if err != nil {
    panic(err)
}
fmt.Printf("Matched %d out of %d rows\n", result.TotalMatched, result.TotalScanned)

// ⚠️ 类型不匹配不会匹配
badPredicates := []*colstore.Predicate{
    {Column: "age", Op: colstore.OpEq, Value: "30"},      // ❌ int vs string，永不匹配
}
badResult, _ := cs.ReadWithFilter([]string{"id"}, badPredicates)
fmt.Println("Bad predicate matches:", badResult.TotalMatched) // 0
```

### 7.5 IN / NOT IN 操作

```go
predicates := []*colstore.Predicate{
    {Column: "city", Op: colstore.OpIn, Values: []colstore.Value{"NYC", "SF"}},
}

result, _ := cs.ReadWithFilter([]string{"name"}, predicates)
for _, row := range result.Rows {
    fmt.Println(row.Values["name"])
}
```

### 7.6 分批追加数据

```go
for page := 0; page < 10; page++ {
    ids := make([]colstore.Value, 100)
    names := make([]colstore.Value, 100)
    for i := 0; i < 100; i++ {
        ids[i] = page*100 + i
        names[i] = fmt.Sprintf("user_%d", page*100+i)
    }

    batch := []colstore.ColumnBatch{
        {Name: "id", Values: ids},
        {Name: "name", Values: names},
    }
    cs.Write(batch)
}

fmt.Println("Total:", cs.RowCount()) // 1000
```

## 8. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyBatch` | 空批次 | `Write(nil)` 或空切片 |
| `ErrColumnMismatch` | 列行数不一致 | 批次中各列 Values 长度不同 |
| `ErrDuplicateColumnName` | 列名重复 | 同一批次中列名重复 |
| `ErrColumnNotFound` | 列不存在 | 读取或过滤时引用了不存在的列 |
| `ErrEmptyColumnSet` | 空投影列 | `Read([]string{})` |
| `ErrInvalidPredicate` | 无效谓词 | 单值运算符 Value 为 nil，或 IN/NOT IN 的 Values 为空 |
| `ErrInvalidOp` | 无效运算符 | 使用了未定义的 Operator |
| `ErrStoreClosed` | 存储已关闭 | Close() 之后继续读写 |

## 9. 数据类型支持

`compareValues` 函数支持以下类型的比较：

| 类型 | 支持的运算符 | 说明 |
|------|-------------|------|
| `int` | 全部 | 整数比较 |
| `float64` | 全部 | 浮点数比较 |
| `int` ↔ `float64` | 全部 | **自动类型提升**后比较（唯一允许的跨类型比较） |
| `string` | 全部 | 字典序比较 |
| `bool` | =, !=, >, < | false < true |
| `nil` | 全部 | nil 小于任何非 nil 值，仅 nil 等于 nil |
| **其他跨类型组合** | =, != 永不相等 | 如 int vs string、bool vs int 等，严格判定为不相等 |

## 10. 性能特征

### 10.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Write（编码模式单值） | 平均 O(1) | map 查找字典 |
| Write（原始模式单值） | O(1) | 直接 append |
| Write（N 行 M 列） | O(N × M) | 每行每列一次存储 |
| Read（无过滤，P 列） | O(N × P) | N 行 × P 列读取 |
| ReadWithFilter（K 个谓词，P 列） | O(N × (K + P)) | 每行先 K 次过滤，再 P 列读取 |
| DictionarySize | O(1) | 禁用时返回 0 |

### 10.2 空间复杂度

| 模式 | 每列空间 | 适用场景 |
|------|---------|---------|
| 字典编码 (dictEnabled=true) | `O(D + N)`，D=字典大小(唯一值数)，N=行数 | 高重复度列：D << N，空间节省显著 |
| 原始存储 (dictEnabled=false) | `O(N)` | 低重复度列：避免字典的 map/reverseDict 额外开销 |

## 11. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存，进程退出即丢失
2. **追加写模型**: 仅支持在末尾追加数据，不支持修改已有行
3. **列稀疏性**: 后续批次新增列时，历史行在该列的值为 nil
4. **字典配置不可变**: DictionaryEnabled 在创建 ColumnStore 时指定，运行时不可修改，所有列共享
5. **字典无回收**: 编码模式下字典一旦添加不会删除，适合值集合稳定的列
6. **谓词全量扫描**: 谓词下推通过逐行扫描实现，未建索引
7. **写入阻塞读取**: 写操作持有全局写锁，大数据量写入可能阻塞读取
8. **严格类型比较**: 谓词过滤中，除 int↔float64 外的跨类型值永不相等，不会因字符串表示相同而误匹配
9. **列类型一致性**: 同一列建议存放相同类型值。虽然技术上支持混合类型存储，但可能导致谓词过滤结果不符合直觉
