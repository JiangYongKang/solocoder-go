# 属性测试框架 (proptest) 模块需求文档

## 1. 模块概述

本模块提供了一个完整的属性测试（Property-Based Testing）框架，类似于 QuickCheck/Hypothesis 的思想。开发者可以通过声明输入的生成规则和需要验证的不变量属性，让框架自动生成大量随机测试用例，从而发现传统单元测试难以覆盖的边界情况。

### 核心功能

1. **基于生成器的随机输入生成**：内置多种输入生成器，支持自定义范围和复合类型组合
2. **不变量断言验证**：自动执行大量随机输入验证不变量条件
3. **失败用例最小化收缩**：自动将失败用例简化为最小可复现形式
4. **可复现的随机种子**：记录种子值以精确复现测试结果
5. **Verbose 详细输出**：开启后输出测试执行过程和收缩步骤的详细信息
6. **结构化错误返回**：通过 `Result.Err` 字段返回可判别的错误类型，便于程序化处理

---

## 2. 核心结构体职责

### 2.1 接口定义

#### `Generator[T any]`
生成器的通用接口，所有生成器必须实现：

```go
type Generator[T any] interface {
    Generate(r *rand.Rand) T    // 根据随机源生成一个值
    Shrink(value T) []T         // 对给定值产生更小/更简单的候选值列表
}
```

### 2.2 配置与结果

#### `Config`
测试运行配置：

| 字段          | 类型        | 默认值       | 说明                                               |
|--------------|-------------|-------------|----------------------------------------------------|
| `Iterations` | `int`       | 100          | 随机输入迭代验证次数                               |
| `MaxShrinks` | `int`       | 1000         | 收缩阶段最大尝试次数                               |
| `Seed`       | `int64`     | 0            | 随机种子（`UseRandomSeed=false` 时生效）           |
| `UseRandomSeed` | `bool`   | `true`       | 是否使用基于时间戳的随机种子                       |
| `Verbose`    | `bool`      | `false`      | 是否输出详细信息                                   |
| `Writer`     | `io.Writer` | `nil`        | Verbose 输出目标；`nil` 时 Verbose=true 写入 os.Stdout |

#### `Result[T any]`
测试执行结果：

| 字段          | 类型              | 说明                                         |
|--------------|-------------------|----------------------------------------------|
| `Passed`     | `bool`            | 属性是否通过所有验证                         |
| `Seed`       | `int64`           | 本次测试使用的种子（用于复现）               |
| `Iterations` | `int`             | 实际执行的迭代次数                           |
| `FailCase`   | `*FailCase[T]`    | 失败用例详情（通过时为 `nil`）               |
| `ShrinkSteps`| `int`             | 收缩阶段实际执行的步数                       |
| `Err`        | `error`           | 失败时的结构化错误（通过时为 `nil`）          |

#### `FailCase[T any]`
失败用例详情：

| 字段        | 类型      | 说明                                  |
|------------|-----------|---------------------------------------|
| `Input`    | `T`       | 触发失败的（收缩后）输入值            |
| `Seed`     | `int64`   | 测试种子（与 Result.Seed 一致）       |
| `Iteration`| `int`     | 在第几次迭代中发现失败                |
| `Message`  | `string`  | 属性函数返回的失败描述                |

---

## 3. 内置生成器

### 3.1 整数生成器 `IntGenerator`

| 构造函数                     | 说明                                             |
|-----------------------------|--------------------------------------------------|
| `Int()`                     | 完整整数范围 `[math.MinInt, math.MaxInt]`        |
| `IntRange(min, max int)`    | 自定义范围 `[min, max]`（自动处理 min>max）      |
| `IntNonNegative()`          | 非负整数 `[0, math.MaxInt]`                      |
| `IntPositive()`             | 正整数 `[1, math.MaxInt]`                        |

整数生成器使用 `uint64` 算术计算跨度，避免大范围场景下的整数溢出。对于 `[MinInt, MaxInt]` 的全范围使用 `r.Uint64()` 生成；其他大跨度范围使用拒绝采样法确保均匀性。

**示例**：
```go
gen := IntRange(-10, 10)        // 生成 -10 到 10 之间的整数
gen := IntNonNegative()         // 生成 >= 0 的整数
```

### 3.2 浮点数生成器 `Float64Generator`

| 构造函数                            | 说明                                          |
|------------------------------------|-----------------------------------------------|
| `Float64()`                        | 完整范围 `[-MaxFloat64, MaxFloat64]`          |
| `Float64Range(min, max float64)`   | 自定义浮点数范围                              |

### 3.3 布尔生成器 `BoolGenerator`

| 构造函数      | 说明                      |
|--------------|---------------------------|
| `Bool()`     | 50% 概率 true / false    |

### 3.4 字符串生成器 `StringGenerator`

| 构造函数                                                        | 说明                                                      |
|----------------------------------------------------------------|-----------------------------------------------------------|
| `String()`                                                     | 默认长度 `[0, 50]`，字母数字字符集                        |
| `StringLen(minLen, maxLen int)`                                | 指定长度范围，字母数字字符集                              |
| `StringWithCharset(minLen, maxLen int, ranges []RuneRange)`    | 指定长度范围和自定义字符集                                |

**预设字符集**：

| 变量名                | 说明                              |
|----------------------|-----------------------------------|
| `ASCIILetters`       | `[a-z, A-Z]`                      |
| `ASCIIDigits`        | `[0-9]`                           |
| `ASCIIAlphanumeric`  | 字母 + 数字（默认）               |
| `ASCIIPrintable`     | 可打印 ASCII `[空格, ~]`          |
| `UnicodeAll`         | 基本多文种平面 + 扩展             |

**示例**：
```go
// 生成 5 到 10 个小写字母
gen := StringWithCharset(5, 10, []RuneRange{{'a', 'z'}})
```

### 3.5 切片生成器 `SliceGenerator[T]`

| 构造函数                                                  | 说明                                       |
|----------------------------------------------------------|--------------------------------------------|
| `Slice[T](elemGen Generator[T])`                         | 长度 `[0, 30]`，元素由 `elemGen` 生成      |
| `SliceLen[T](elemGen Generator[T], minLen, maxLen int)`  | 指定切片长度范围                           |

**示例**：
```go
// 生成 3 到 8 个 0-99 之间整数组成的切片
gen := SliceLen(IntRange(0, 99), 3, 8)
```

---

## 4. 生成器组合方式

### 4.1 Pair 组合 `PairGenerator[A, B]`

将两个生成器组合成二元组：

```go
type Pair[A, B any] struct {
    A A
    B B
}

gen := PairOf(IntRange(0, 10), StringLen(1, 5))
// 生成 Pair[int, string]{A: 3, B: "abc"} 形式的值
```

### 4.2 Tuple3 组合 `Tuple3Generator[A, B, C]`

将三个生成器组合成三元组：

```go
type Tuple3[A, B, C any] struct {
    A A; B B; C C
}

gen := Tuple3Of(Int(), Bool(), String())
```

### 4.3 Map 转换 `MapGenerator[T, U]`

将一个生成器的输出通过函数转换为另一种类型：

```go
// 将整数转为对应长度的字符串
gen := Map(IntRange(0, 10), func(i int) string {
    return strings.Repeat("x", i)
})

// 带自定义收缩函数
gen := MapWithShrink(
    IntRange(0, 100),
    func(i int) string { return strings.Repeat("a", i) },
    func(s string) []string { /* 自定义收缩 */ return nil },
)
```

### 4.4 常量生成器 `ConstGenerator[T]`

总是生成固定值（用于组合中某一分支需要固定值的场景）：

```go
gen := Const(42)    // 永远生成 42
```

---

## 5. 收缩机制说明

### 5.1 总体策略

当某个输入导致属性验证失败时，框架会尝试找到"最小的"仍能触发失败的输入。这个过程称为收缩（Shrinking）。

收缩算法流程：
1. 从初始失败输入开始
2. 调用生成器的 `Shrink()` 方法获取候选简化值列表
3. 逐个验证候选值，找到第一个仍触发失败的候选
4. 将该候选设为当前值，重复步骤 2-3
5. 当没有候选能触发失败、或达到 `MaxShrinks` 限制时停止
6. 返回当前最小失败输入

### 5.2 各类型收缩策略

#### 整数 `IntGenerator.Shrink`
1. 若 0 在范围内 → 首先尝试 0
2. 若为负数 → 尝试对应的正数（取反）
3. **二分法向 0 逼近**：不断将值减半，产生更小的候选
4. **邻居检查**：最后尝试 `±1` 的邻居值

#### 浮点数 `Float64Generator.Shrink`
1. 若 0.0 在范围内 → 首先尝试 0.0
2. **二分法逼近**：不断按 1/2 步长向 0.0 逼近，直到精度阈值 `1e-9`

#### 布尔 `BoolGenerator.Shrink`
- `true` → 尝试 `false`（单一候选）
- `false` → 无候选（已是最小值）

#### 字符串 `StringGenerator.Shrink`
1. 若允许空串 → 尝试 `""`
2. **长度缩减**：生成 n-1、n/2、n/4... 以及 MinLen 长度的前缀/后缀
3. **逐个删除**：生成删除每个单字符的候选

#### 切片 `SliceGenerator[T].Shrink`
1. 若允许空切片 → 尝试 `[]`
2. **长度缩减**：同字符串的长度缩减策略
3. **逐个删除元素**：生成删除每个位置元素的候选
4. **元素级收缩**：对每个元素调用其生成器的 `Shrink()`，替换后产生候选

#### Pair / Tuple3
- 分别对每个分量独立收缩，其他分量保持不变
- 优先收缩 A，再收缩 B，再收缩 C

#### Map / Const
- `Map`：如果提供了 `ShrinkFn`，则使用自定义收缩；否则不收缩
- `Const`：永远不收缩（无候选）

---

## 6. Verbose 模式

### 6.1 启用方式

通过 `Config.Verbose = true` 或 `WithVerbose(true)` 选项启用：

```go
// 方式一：通过 Config
cfg := proptest.Config{
    Iterations: 200,
    Seed:       42,
    Verbose:    true,
}
runner := proptest.NewRunner[int](cfg)

// 方式二：通过 Check 便捷函数
result := proptest.Check(gen, prop, proptest.WithVerbose(true))

// 方式三：指定自定义 Writer（用于测试捕获输出）
var buf bytes.Buffer
cfg := proptest.Config{
    Verbose: true,
    Writer:  &buf,    // Verbose 输出写入 buf 而非 os.Stdout
}
```

### 6.2 输出内容

当 Verbose 启用时，框架会在以下关键节点输出信息：

| 阶段             | 输出内容                                              |
|-----------------|------------------------------------------------------|
| 测试开始         | `proptest: starting check (seed=..., iterations=..., maxShrinks=...)` |
| 每 100 次迭代    | `proptest: N iterations passed`                       |
| 发现失败         | `proptest: failure at iteration N, input=...: message` |
| 收缩每步进展     | `proptest: shrink step N: found simpler failing input ...` |
| 收缩完成         | `proptest: shrunk to ... in N steps`                  |
| 全部通过         | `proptest: all N iterations passed`                   |

### 6.3 Writer 配置

- `Config.Writer` 为 `nil` 且 `Verbose=true` → 默认写入 `os.Stdout`
- `Config.Writer` 为 `nil` 且 `Verbose=false` → 使用 `io.Discard`，不产生任何输出
- `Config.Writer` 非 `nil` → Verbose 输出写入指定 Writer（常用于测试中捕获输出）

---

## 7. 错误变量与使用场景

### 7.1 错误变量定义

| 错误变量              | 触发场景                                                    |
|----------------------|-------------------------------------------------------------|
| `ErrPropertyFailed`  | 属性验证失败（不变量被违反）                                 |
| `ErrInvalidConfig`   | 配置非法（如传入 nil 属性函数）                               |
| `ErrShrinkLimit`     | 收缩达到最大迭代限制（`MaxShrinks` 用尽）                    |
| `ErrGeneratorNil`    | 传入 nil 生成器（Runner.Check 返回时设置，生成器构造时 panic）|

### 7.2 错误在 Result.Err 中的设置规则

| 场景                     | `Result.Err` 值                                           |
|-------------------------|-----------------------------------------------------------|
| 属性验证通过             | `nil`                                                     |
| 传入 nil 生成器          | `ErrGeneratorNil`                                         |
| 传入 nil 属性函数        | `ErrInvalidConfig`                                        |
| 属性验证失败（收缩未受限）| `ErrPropertyFailed`                                       |
| 属性验证失败（收缩达上限）| `fmt.Errorf("%w: reached max shrinks (%d)", ErrShrinkLimit, maxShrinks)` |

### 7.3 使用 errors.Is 判断错误类型

```go
result := proptest.Check(gen, prop, proptest.WithSeed(42))

if !result.Passed {
    switch {
    case errors.Is(result.Err, proptest.ErrGeneratorNil):
        // 生成器为空，需检查生成器构造
    case errors.Is(result.Err, proptest.ErrInvalidConfig):
        // 配置非法，需检查属性函数
    case errors.Is(result.Err, proptest.ErrShrinkLimit):
        // 收缩受限，可考虑增大 MaxShrinks
    case errors.Is(result.Err, proptest.ErrPropertyFailed):
        // 正常的属性失败，检查 FailCase 获取详情
    }
}
```

`ErrShrinkLimit` 使用 `%w` 动词包装，因此 `errors.Is` 可以正确解包。当收缩达到上限时，`Result.Err` 同时匹配 `ErrShrinkLimit` 和 `ErrPropertyFailed`（因为 `ErrShrinkLimit` 场景是 `ErrPropertyFailed` 的子集），建议先检查 `ErrShrinkLimit`。

---

## 8. 使用方式

### 8.1 方式一：便捷函数 `Check`

```go
result := proptest.Check(
    IntNonNegative(),                                     // 生成器
    func(x int) (bool, string) {                          // 属性函数
        return x + 0 == x, "x + 0 should equal x"
    },
    proptest.WithIterations(1000),
    proptest.WithSeed(12345),
)

if result.Passed {
    fmt.Println("Passed!")
} else {
    fmt.Println(result.String())
    if errors.Is(result.Err, proptest.ErrShrinkLimit) {
        fmt.Println("Hint: consider increasing MaxShrinks")
    }
}
```

### 8.2 方式二：显式 Runner

```go
cfg := proptest.Config{
    Iterations:    500,
    MaxShrinks:    2000,
    Seed:          42,
    UseRandomSeed: false,
}
runner := proptest.NewRunner[int](cfg)
result := runner.Check(IntRange(0, 100), func(x int) (bool, string) {
    if x < 50 {
        return true, ""
    }
    return false, fmt.Sprintf("%d should be < 50", x)
})
```

### 8.3 使用种子复现失败

当测试失败时，`Result.Seed` 会记录导致失败的种子。下次运行时使用相同种子即可精确复现：

```go
seed := result.Seed

result2 := proptest.Check(gen, prop, proptest.WithSeed(seed))
// result2 会生成完全相同的输入序列
```

### 8.4 启用 Verbose 调试

```go
var buf bytes.Buffer
result := proptest.Check(
    IntRange(0, 100),
    func(x int) (bool, string) {
        if x > 50 { return false, "too large" }
        return true, ""
    },
    proptest.WithIterations(200),
    proptest.WithSeed(42),
    proptest.WithVerbose(true),
    // proptest.WithWriter(&buf),  // 可选：捕获到 buffer
)
// Verbose 输出已写入 os.Stdout（或指定 Writer）
```

### 8.5 复合示例：验证排序算法

```go
gen := SliceLen(IntRange(-100, 100), 0, 50)
prop := func(nums []int) (bool, string) {
    sorted := make([]int, len(nums))
    copy(sorted, nums)
    sort.Ints(sorted)

    for i := 1; i < len(sorted); i++ {
        if sorted[i] < sorted[i-1] {
            return false, fmt.Sprintf("not sorted at index %d: %v", i, sorted)
        }
    }
    if len(sorted) != len(nums) {
        return false, "length changed"
    }
    return true, ""
}

result := proptest.Check(gen, prop, proptest.WithIterations(1000))
```

---

## 9. 配置选项（Option 模式）

| 选项函数                      | 说明                                      |
|------------------------------|-------------------------------------------|
| `WithIterations(n int)`      | 设置迭代次数（仅 n>0 时生效）             |
| `WithMaxShrinks(n int)`      | 设置最大收缩尝试次数                      |
| `WithSeed(seed int64)`       | 指定固定随机种子（同时禁用 UseRandomSeed）|
| `WithVerbose(v bool)`        | 启用/禁用详细输出                         |

---

## 10. 包路径

- 实现：`internal/proptest/proptest.go`
- 测试：`internal/proptest/proptest_test.go`
- 文档：`docs/proptest.md`
