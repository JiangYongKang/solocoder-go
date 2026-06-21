# Benchfrm 基准测试框架模块

## 模块概述

`internal/benchfrm` 包提供了一个功能完整、生产级的 Go 基准测试框架。框架支持对任意函数进行性能基准测试，具备**重复运行统计、预热阶段、内存分配统计、超时控制、多组对比报告、性能回归自动检测**六大核心能力。模块通过接口抽象，支持扩展自定义的基线存储后端和报告输出格式。

---

## 核心功能详解

### 1. 重复运行与统计指标

框架支持对被测函数执行指定次数的重复运行，通过多次采样消除偶然波动，得到可靠的性能数据。

**全部统计指标定义：**

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `Name` | `string` | 基准测试组的名称，用于标识和区分不同测试项 |
| `Iterations` | `int` | 实际执行的正式迭代次数（不含预热，排除失败运行） |
| `MeanDuration` | `time.Duration` | 所有成功运行的**算术平均耗时**，反映整体性能水平 |
| `MinDuration` | `time.Duration` | 所有成功运行中的**最短耗时**，通常代表代码的最佳性能 |
| `MaxDuration` | `time.Duration` | 所有成功运行中的**最长耗时**，反映最坏情况和性能波动幅度 |
| `StdDevDuration` | `time.Duration` | 耗时的**标准差**，衡量数据的离散程度；值越小说明性能越稳定 |
| `MeanAllocBytes` | `uint64` | 每次运行平均分配的**字节数**，反映内存使用效率 |
| `MeanAllocCount` | `uint64` | 每次运行平均分配的**次数**，反映对象创建频率 |

**统计计算公式：**
- 平均耗时：`MeanDuration = Σ(xi) / N`，其中 N 为成功运行次数
- 标准差：`StdDevDuration = sqrt(Σ(xi - Mean)² / N)`
- 平均分配字节数：`MeanAllocBytes = Σ(AllocBytesi) / N`
- 平均分配次数：`MeanAllocCount = Σ(AllocCounti) / N`

### 2. 预热阶段（Warmup）

Go 运行时具有内联优化、逃逸分析、CPU 分支预测、缓存预热、内存分配器初始化等特性。为了获得稳定的基准测试结果，框架支持在正式计时前执行预热迭代。

**预热阶段特性：**
- 预热迭代**完全不计入**统计结果
- 预热阶段**不计时、不统计内存**
- 预热次数和正式运行次数**可独立配置**
- 预热阶段的函数错误**会被静默忽略**，不影响后续执行
- 默认预热次数：10 次，可通过 `WithWarmupIterations(n)` 设置

### 3. 内存分配统计

通过 Go 标准库 `runtime.MemStats` 在每次运行前后采样内存状态，精确统计被测函数的内存分配行为。

**实现原理：**
1. 运行前执行 `runtime.GC()` 触发垃圾回收以获得干净的起始状态
2. 通过 `runtime.ReadMemStats(&startMem)` 读取起始内存统计
3. 执行被测函数并计时
4. 通过 `runtime.ReadMemStats(&endMem)` 读取结束内存统计
5. 计算差值：
   - `AllocBytes = endMem.TotalAlloc - startMem.TotalAlloc`
   - `AllocCount = endMem.Mallocs - startMem.Mallocs`

**注意事项：**
- 内存统计仅在**正式运行**期间累计，预热阶段不影响统计
- 可通过配置 `WithMemoryCollection(false)` 禁用内存统计以加速测试
- GC 的存在可能导致内存统计存在微小误差，建议通过多次运行取平均

### 4. 超时控制

支持为每次运行设置超时时间，防止被测函数意外卡死影响整体测试流程。

**超时机制详解：**
- 使用 `context.WithTimeout` 实现超时控制
- 超时时间通过 `WithTimeout(d time.Duration)` 配置，纳秒到小时均可
- 默认超时值为 `0`，表示**不启用超时**
- 超时触发时，被测函数所在的 goroutine **无法被强制终止**，但框架会立即返回 `context.DeadlineExceeded` 错误并跳过该次运行
- **重要提示：** 超时后的 goroutine 会继续在后台执行，务必确保被测函数最终能正常结束

**失败处理策略：**
- 单次运行超时或返回错误不会导致整个基准测试终止
- 框架会跳过失败的运行，继续执行后续迭代
- 只有当**所有迭代全部失败**（零次成功）时，才返回 `ErrGroupEmptyResult` 错误
- 返回的 `ErrGroupEmptyResult` 会包装首个失败原因，支持 `errors.Is` 同时识别两个错误

### 5. 多组对比报告

支持注册多个基准测试组（可理解为不同的实现方案或不同的参数配置），运行完成后生成相对基线的对比报告。

**对比维度（三项）：**
| 维度 | 字段名 | 含义 |
|-----|-------|------|
| 耗时变化率 | `VsBaselinePct` | 平均耗时相对基线的变化百分比 |
| 内存字节变化率 | `AllocBytesPct` | 平均分配字节数相对基线的变化百分比 |
| 分配次数变化率 | `AllocCountPct` | 平均分配次数相对基线的变化百分比 |

**百分比计算公式：**
```
变化率 = (当前值 - 基线值) / 基线值 × 100%
```
- 正值（报告中带 ↑ 箭头）：当前性能**劣于**基线
- 负值（报告中带 ↓ 箭头）：当前性能**优于**基线
- 零值：与基线完全一致

### 6. 性能回归自动检测

将当前基准测试结果与已保存的历史基线进行比较，当任意关键指标的劣化超过配置的阈值时，自动标记为性能回归并输出详细的告警信息。

**检测的三项关键指标：**

| 指标名称 | 含义 | 阈值判断条件 |
|---------|------|-------------|
| `MeanDuration` | 平均耗时 | `(Current - Baseline) / Baseline × 100% > Threshold` |
| `MeanAllocBytes` | 平均分配字节数 | `(Current - Baseline) / Baseline × 100% > Threshold` |
| `MeanAllocCount` | 平均分配次数 | `(Current - Baseline) / Baseline × 100% > Threshold` |

**告警信息包含：**
- 劣化的指标名称
- 当前值与基线值
- 劣化百分比与配置阈值
- 明确的 REGRESSED 标记（`⚠️  REGRESSED`）

---

## 核心结构体职责

### 接口类型

#### Benchmarker（基准测试器接口）

框架的对外核心接口，定义了基准测试的完整生命周期操作。

```go
type Benchmarker interface {
    AddGroup(name string, fn BenchmarkFunc, opts ...RunOption)
    RunAll() ([]GroupStatistics, error)
    Compare(baseline string) (ComparisonReport, error)
    CheckRegression(thresholdPct float64) (RegressionReport, error)
    SaveBaseline() error
    LoadBaseline() (map[string]GroupStatistics, error)
    SetBaselineStore(store BaselineStore)
    SetReporter(reporter Reporter)
}
```

**职责：**
- 管理基准测试组的注册
- 协调预热与正式执行流程
- 调度各测试组的独立运行
- 生成多维度对比报告
- 管理基线数据的持久化

**实现类：** `benchmarker`（私有结构体，内部使用 `sync.RWMutex` 保证并发安全）

#### BaselineStore（基线存储接口）

抽象基线数据的持久化层，支持多种存储后端。

```go
type BaselineStore interface {
    Save(groupName string, stats GroupStatistics) error
    Load(groupName string) (GroupStatistics, bool, error)
}
```

**内置实现对比：**

| 实现类 | 存储方式 | 适用场景 | 线程安全 |
|-------|---------|---------|---------|
| `MemoryStore` | 进程内内存 map | 单元测试、临时比较 | ✅ RWMutex |
| `FileStore` | 磁盘 JSON 文件 | CI/CD 流水线、跨运行对比 | ✅ RWMutex |

#### Reporter（报告生成器接口）

抽象报告的输出格式，支持扩展自定义格式（如 JSON、HTML、Markdown、JUnit XML）。

```go
type Reporter interface {
    Report(stats []GroupStatistics) string
    ReportComparison(report ComparisonReport) string
    ReportRegression(report RegressionReport) string
}
```

**内置实现：** `TextReporter` —— 人可读的纯文本格式，适合控制台输出和日志记录。

---

### 数据结构定义

#### RunConfig（运行配置结构体）

单个基准测试组的运行参数配置。

| 字段 | 类型 | 默认值 | 约束 | 说明 |
|-----|------|-------|------|------|
| `Iterations` | `int` | 100 | 必须 > 0 | 正式运行迭代次数 |
| `WarmupIterations` | `int` | 10 | 必须 >= 0 | 预热迭代次数 |
| `CollectMemory` | `bool` | `true` | — | 是否收集内存分配统计 |
| `Timeout` | `time.Duration` | 0 | >= 0 | 单次运行超时时间，0 表示无超时 |

**选项函数（Functional Options Pattern）：**
- `WithIterations(n int)` —— 设置正式运行次数
- `WithWarmupIterations(n int)` —— 设置预热次数
- `WithMemoryCollection(enabled bool)` —— 开关内存统计
- `WithTimeout(d time.Duration)` —— 设置单次运行超时

#### BenchmarkFunc（被测函数类型）

```go
type BenchmarkFunc func() error
```

基准测试框架执行的目标函数签名。函数返回 `error` 时，该次运行被标记为失败并跳过，不会计入统计。只有当全部迭代都失败时，框架才向上返回错误。

#### RunResult（单次运行原始结果）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Duration` | `time.Duration` | 本次运行的实际耗时 |
| `AllocBytes` | `uint64` | 本次运行分配的字节数 |
| `AllocCount` | `uint64` | 本次运行的分配次数 |
| `Error` | `error` | 本次运行返回的错误，nil 表示成功 |

#### GroupStatistics（组统计结果结构体）

完整的单组基准测试统计结果，字段定义见「核心功能详解 → 重复运行与统计指标」中的 8 项指标表。

#### ComparisonReport（对比报告结构体）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Baseline` | `string` | 作为基线的测试组名称 |
| `Items` | `[]ComparisonItem` | 所有测试组的对比项列表 |
| `GeneratedAt` | `time.Time` | 报告生成的时间戳 |

#### ComparisonItem（单组对比项结构体）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Group` | `string` | 测试组名称 |
| `MeanDuration` | `time.Duration` | 该组的平均耗时 |
| `MeanAllocBytes` | `uint64` | 该组的平均分配字节数 |
| `MeanAllocCount` | `uint64` | 该组的平均分配次数 |
| `VsBaselinePct` | `float64` | 耗时相对于基线的变化百分比 |
| `AllocBytesPct` | `float64` | 分配字节数相对于基线的变化百分比 |
| `AllocCountPct` | `float64` | 分配次数相对于基线的变化百分比 |

#### RegressionReport（回归检测报告结构体）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `IsRegression` | `bool` | 是否存在性能回归（任意指标劣化超过阈值即为 true） |
| `Checks` | `[]RegressionCheck` | 各指标的详细检测结果列表 |
| `GeneratedAt` | `time.Time` | 报告生成的时间戳 |

#### RegressionCheck（单指标检测项结构体）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `MetricName` | `string` | 指标名称（`MeanDuration` / `MeanAllocBytes` / `MeanAllocCount`） |
| `CurrentValue` | `float64` | 当前运行的指标值 |
| `BaselineValue` | `float64` | 已保存基线的指标值 |
| `DegradationPct` | `float64` | 劣化百分比（正值为劣化，负值为优化） |
| `ThresholdPct` | `float64` | 配置的回归检测阈值 |
| `IsDegraded` | `bool` | 该指标是否发生性能回归 |

---

## 基准测试完整执行流程

基准测试的执行分为 **7 个有序阶段**，每个阶段的职责明确，便于调试和扩展。

### 阶段 1：初始化与配置

```
用户代码
   ↓
NewBenchmarker() → 创建 benchmarker 实例（含 RWMutex、空 groups 切片）
   ↓
SetBaselineStore() → 可选：配置基线持久化存储（Memory / File / 自定义）
SetReporter()      → 可选：配置自定义报告生成器
   ↓
AddGroup(name, fn, opts...) × N → 注册 1 到 N 个基准测试组
```

**关键逻辑：**
- 每个组独立配置（迭代次数、预热次数、超时、内存收集等）
- 重复组名触发 panic（`ErrDuplicateGroupName`）
- nil 函数触发 panic（`ErrNilFunction`）
- 非法配置（Iterations<=0 或 Warmup<0）触发 panic

### 阶段 2：遍历测试组

调用 `RunAll()` 后，框架**按注册顺序串行**处理每个已注册的测试组。各组之间相互独立，一组失败不会污染其他组已完成的统计结果。

### 阶段 3：预热执行

对当前测试组执行 `WarmupIterations` 次预热运行：

```
for i in [0, WarmupIterations):
    直接调用 fn()
    不计时、不统计内存
    忽略返回的 error（静默失败）
```

**设计意图：** 让 Go 运行时的内联优化、逃逸分析结果、CPU 分支预测缓存、内存分配器的 arena 等在预热期间达到稳定状态。

### 阶段 4：正式运行（含超时控制与容错）

对当前测试组执行 `Iterations` 次正式运行，每次运行流程：

```
1. 检查 Timeout 配置
   ├── Timeout > 0 → 走超时控制路径（runSingleWithTimeout）
   └── Timeout = 0 → 走直接执行路径（runSingle）

2. 如果启用内存统计：
   runtime.GC()                        → 触发 GC 获得干净起点
   runtime.ReadMemStats(&startMem)     → 记录起始内存快照

3. 记录开始时间戳 → 执行 fn() → 计算 Duration = Now - Start

4. 如果启用内存统计：
   runtime.ReadMemStats(&endMem)       → 记录结束内存快照
   AllocBytes = endMem.TotalAlloc - startMem.TotalAlloc
   AllocCount = endMem.Mallocs - startMem.Mallocs

5. 结果判定：
   ├── fn() 返回 error 或触发超时 → 记录 firstErr，continue 跳过本次
   └── 成功 → 将 RunResult 追加到 runResults 切片
```

**超时控制实现细节：**
```go
ctx, cancel := context.WithTimeout(context.Background(), Timeout)
defer cancel()

done := make(chan RunResult, 1)
go func() { done <- runSingle(...) }()

select {
case result := <-done:
    return result              // 正常完成
case <-ctx.Done():
    return RunResult{Error: ctx.Err()}  // 超时，返回 DeadlineExceeded
}
```

### 阶段 5：统计计算与空结果判定

将当前组成功的 `RunResult` 汇总计算，生成 `GroupStatistics`：

```
┌───────────────────────────────────────────────────────────┐
│ 输入：runResults（仅包含成功运行的结果）                     │
├───────────────────────────────────────────────────────────┤
│ 1. if len(runResults) == 0:                                │
│      if firstErr != nil → 返回 ErrGroupEmptyResult 包装 firstErr │
│      else               → 返回 ErrGroupEmptyResult         │
│                                                             │
│ 2. 累加计算 TotalDuration / TotalAllocBytes / TotalAllocCount  │
│ 3. 遍历记录 MinDuration / MaxDuration                       │
│ 4. 计算 Mean = Total / N（N 为成功次数）                     │
│ 5. 计算 Variance = Σ(xi - Mean)² / N                        │
│ 6. 计算 StdDev = sqrt(Variance)                             │
│ 7. 构造并返回 GroupStatistics                                │
└───────────────────────────────────────────────────────────┘
```

**容错语义：** 允许部分运行失败，只要至少有一次成功即可产生有效统计。全部失败时通过 `ErrGroupEmptyResult` 明确告知调用方。

### 阶段 6：可选的对比报告生成

运行完成后，可选择执行多组对比分析：

**Compare(baseline) 流程：**
```
1. 检查 lastResults 非空 → 否则 ErrNoGroupsRegistered
2. 在 lastResults 中查找基线组 → 找不到返回 ErrGroupNotFound
3. 遍历每个组，计算 Duration Δ%、AllocBytes Δ%、AllocCount Δ% 三个维度
4. 返回 ComparisonReport（含基线名 + 所有对比项）
```

### 阶段 7：可选的回归检测与基线保存

```
SaveBaseline() 流程：
  1. 检查已配置 BaselineStore → 否则 ErrNoBaselineStore
  2. 检查 lastResults 非空 → 否则 ErrNoGroupsRegistered
  3. 遍历每组，调用 store.Save(name, stats) 持久化

CheckRegression(threshold) 流程：
  1. 检查阈值 > 0 → 否则 ErrInvalidThreshold
  2. 检查已配置 BaselineStore → 否则 ErrNoBaselineStore
  3. 检查 lastResults 非空 → 否则 ErrNoGroupsRegistered
  4. 遍历每组，从 store.Load 加载基线
     ├── 加载失败 → 返回错误
     └── 找不到基线 → 返回 ErrBaselineNotFound（含组名）
  5. 对每组的 3 个指标执行 checkMetric() 判定
  6. 任一指标 IsDegraded=true → 设置 IsRegression=true
  7. 返回 RegressionReport
```

---

## 报告格式说明

### 基准测试结果报告格式

```
=== Benchmark Results ===
Generated at: 2024-01-15T10:30:00+08:00

Group: fast_sort
  Iterations:     100
  Mean Duration:  1.234ms
  Min Duration:   0.987ms
  Max Duration:   2.156ms
  Std Dev:        0.245ms
  Mean Alloc Bytes: 4096 bytes/op
  Mean Alloc Count: 3 allocs/op

Group: slow_sort
  Iterations:     100
  Mean Duration:  15.678ms
  Min Duration:   12.345ms
  Max Duration:   20.123ms
  Std Dev:        1.567ms
  Mean Alloc Bytes: 16384 bytes/op
  Mean Alloc Count: 12 allocs/op
```

**字段说明（按顺序）：**
1. 标题行标识报告类型
2. `Generated at` 为 ISO 8601 格式的时间戳
3. 每个 `Group:` 块对应一个基准测试组
4. 8 行统计指标按固定顺序排列，便于脚本 grep/awk 解析

### 对比报告格式

```
=== Comparison Report ===
Baseline: fast_sort
Generated at: 2024-01-15T10:30:00+08:00

Group                Mean Duration      Duration Δ%     Alloc Bytes     Bytes Δ%      Allocs Δ%
-----------------------------------------------------------------------------------------------
fast_sort            1.234ms            +0.00%          4096            +0.00%        +0.00%
slow_sort            15.678ms           +1170.12% ↑     16384           +300.00% ↑    +300.00% ↑
optimized_sort       0.876ms            -29.01% ↓       2048            -50.00% ↓     -66.67% ↓
```

**列定义：**

| 列名 | 含义 | 标记约定 |
|-----|------|---------|
| `Group` | 测试组名称 | — |
| `Mean Duration` | 该组的平均耗时 | — |
| `Duration Δ%` | 耗时相对基线的变化率 | `↑` 更慢，`↓` 更快 |
| `Alloc Bytes` | 该组的平均分配字节数 | — |
| `Bytes Δ%` | 字节数相对基线的变化率 | `↑` 更多，`↓` 更少 |
| `Allocs Δ%` | 分配次数相对基线的变化率 | `↑` 更多，`↓` 更少 |

### 回归检测报告格式

**无性能回归时：**
```
=== Regression Check Report ===
Generated at: 2024-01-15T10:30:00+08:00
✅ No performance regression detected.

Metric               Current         Baseline        Degradation %   Status
--------------------------------------------------------------------------------
MeanDuration         1234567.00      1200000.00      2.88%           OK
MeanAllocBytes       4096.00         4096.00         0.00%           OK
MeanAllocCount       3.00            3.00            0.00%           OK
```

**检测到性能回归时：**
```
=== Regression Check Report ===
Generated at: 2024-01-15T10:30:00+08:00
⚠️  PERFORMANCE REGRESSION DETECTED!

Metric               Current         Baseline        Degradation %   Status
--------------------------------------------------------------------------------
MeanDuration         2500000.00      1200000.00      108.33%         ⚠️  REGRESSED
MeanAllocBytes       8192.00         4096.00         100.00%         ⚠️  REGRESSED
MeanAllocCount       5.00            3.00            66.67%          ⚠️  REGRESSED
```

**列定义：**
- `Metric` — 指标名称
- `Current` — 当前值（耗时单位为纳秒，便于数值精确比较）
- `Baseline` — 基线值
- `Degradation %` — 劣化百分比（负值表示性能优化）
- `Status` — `OK` 或 `⚠️  REGRESSED`

---

## 错误处理

### 完整错误定义表

| 错误变量 | 触发场景 | 错误消息 |
|---------|---------|---------|
| `ErrNoGroupsRegistered` | 调用 RunAll/Compare/SaveBaseline 时未注册任何组 | `benchfrm: no benchmark groups registered` |
| `ErrGroupNotFound` | Compare 中指定的基线组不存在于当前结果 | `benchfrm: benchmark group not found` |
| `ErrInvalidIterations` | Iterations 配置 <= 0（AddGroup 时 panic） | `benchfrm: invalid number of iterations` |
| `ErrInvalidWarmup` | WarmupIterations 配置 < 0（AddGroup 时 panic） | `benchfrm: invalid number of warmup iterations` |
| `ErrInvalidThreshold` | CheckRegression 阈值 <= 0 | `benchfrm: invalid regression threshold` |
| `ErrNilFunction` | AddGroup 传入 nil 函数（panic） | `benchfrm: benchmark function cannot be nil` |
| `ErrDuplicateGroupName` | AddGroup 组名重复（panic） | `benchfrm: duplicate group name` |
| `ErrNoBaselineStore` | 调用 SaveBaseline/LoadBaseline/CheckRegression 前未配置存储 | `benchfrm: no baseline store configured` |
| `ErrBaselineNotFound` | CheckRegression 时某组缺少已保存的基线 | `benchfrm: baseline not found for group: <name>` |
| `ErrGroupEmptyResult` | 组内全部迭代均失败，无有效运行结果（可包装首个失败原因） | `benchfrm: group has no valid results` |

### 错误返回策略

| 操作类型 | 错误返回方式 | 说明 |
|---------|-------------|------|
| 配置验证（AddGroup） | **panic** | 属于编程错误，应在开发阶段修复 |
| 运行时部分失败 | 静默跳过 | 单次运行失败不终止整体，继续后续迭代 |
| 运行时全部失败 | `return ErrGroupEmptyResult` | 包装首个失败原因，支持 errors.Is 双向识别 |
| 存储 I/O 错误 | `return error` | 向上传递原始错误 |
| 被测函数自身错误 | 被包装 | 单次失败被跳过，全部失败时作为 ErrGroupEmptyResult 的 cause |
| context 超时错误 | 被包装 | 同被测函数错误处理 |

**错误链示例：**
```go
_, err := b.RunAll()
errors.Is(err, ErrGroupEmptyResult)        // true — 全部失败
errors.Is(err, context.DeadlineExceeded)   // true — 首个失败原因是超时
```

---

## 使用示例

### 示例 1：基本基准测试

注册单个函数，运行 100 次正式迭代和 20 次预热，输出统计结果。

```go
package main

import (
    "fmt"
    "sort"
    "solocoder-go/internal/benchfrm"
)

func main() {
    b := benchfrm.NewBenchmarker()

    b.AddGroup("sort_1000_ints", func() error {
        data := make([]int, 1000)
        for i := range data {
            data[i] = 1000 - i
        }
        sort.Ints(data)
        return nil
    }, benchfrm.WithIterations(100), benchfrm.WithWarmupIterations(20))

    results, err := b.RunAll()
    if err != nil {
        panic(fmt.Sprintf("benchmark failed: %v", err))
    }

    reporter := benchfrm.NewTextReporter()
    fmt.Println(reporter.Report(results))
}
```

### 示例 2：多算法对比与报告

对三种排序算法进行基准测试，以冒泡排序为基线生成对比报告。

```go
package main

import (
    "fmt"
    "sort"
    "solocoder-go/internal/benchfrm"
)

func main() {
    testData := make([]int, 5000)
    for i := range testData {
        testData[i] = 5000 - i
    }

    b := benchfrm.NewBenchmarker()

    b.AddGroup("bubble_sort", func() error {
        data := make([]int, len(testData))
        copy(data, testData)
        bubbleSort(data)
        return nil
    }, benchfrm.WithIterations(20))

    b.AddGroup("quick_sort", func() error {
        data := make([]int, len(testData))
        copy(data, testData)
        quickSort(data)
        return nil
    }, benchfrm.WithIterations(50))

    b.AddGroup("std_sort", func() error {
        data := make([]int, len(testData))
        copy(data, testData)
        sort.Ints(data)
        return nil
    }, benchfrm.WithIterations(50))

    _, err := b.RunAll()
    if err != nil {
        panic(fmt.Sprintf("benchmark failed: %v", err))
    }

    report, err := b.Compare("bubble_sort")
    if err != nil {
        panic(fmt.Sprintf("compare failed: %v", err))
    }

    reporter := benchfrm.NewTextReporter()
    fmt.Println(reporter.ReportComparison(report))
}

func bubbleSort(arr []int) { /* ... */ }
func quickSort(arr []int)  { /* ... */ }
```

### 示例 3：CI 流水线中的性能回归检测

在每次提交时运行基准测试，与主分支保存的基线比较，劣化超过 15% 时报警并阻止合入。

```go
package main

import (
    "errors"
    "fmt"
    "os"
    "solocoder-go/internal/benchfrm"
)

func main() {
    store, err := benchfrm.NewFileStore("./.benchmarks/baselines")
    if err != nil {
        panic(fmt.Sprintf("failed to create baseline store: %v", err))
    }

    b := benchfrm.NewBenchmarker()
    b.SetBaselineStore(store)

    b.AddGroup("critical_path_func", func() error {
        return criticalPathFunc()
    }, benchfrm.WithIterations(200), benchfrm.WithWarmupIterations(30))

    _, err = b.RunAll()
    if err != nil {
        if errors.Is(err, benchfrm.ErrGroupEmptyResult) {
            panic(fmt.Sprintf("all benchmark runs failed: %v", err))
        }
        panic(fmt.Sprintf("benchmark execution failed: %v", err))
    }

    regressionReport, err := b.CheckRegression(15.0) // 15% 阈值
    if err != nil {
        panic(fmt.Sprintf("regression check failed: %v", err))
    }

    reporter := benchfrm.NewTextReporter()
    fmt.Println(reporter.ReportRegression(regressionReport))

    if regressionReport.IsRegression {
        os.Exit(1) // CI 失败，阻止合入
    }

    // 仅在 main 分支上更新基线
    if os.Getenv("CI_COMMIT_BRANCH") == "main" {
        if err := b.SaveBaseline(); err != nil {
            panic(fmt.Sprintf("failed to save baseline: %v", err))
        }
        fmt.Println("Baseline updated for main branch.")
    }
}

func criticalPathFunc() error { return nil }
```

### 示例 4：带超时的基准测试

防止被测函数（如 RPC 调用）意外卡住导致 CI 整体超时。

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"
    "solocoder-go/internal/benchfrm"
    "time"
)

func main() {
    b := benchfrm.NewBenchmarker()

    b.AddGroup("rpc_call", func() error {
        resp, err := remoteService.Call()
        if err != nil {
            return err
        }
        _ = resp
        return nil
    },
        benchfrm.WithIterations(30),
        benchfrm.WithWarmupIterations(5),
        benchfrm.WithTimeout(2*time.Second),   // 单次 RPC 超过 2s 判定失败
        benchfrm.WithMemoryCollection(false), // RPC 场景不关注内存，关闭以加速
    )

    results, err := b.RunAll()
    if err != nil {
        if errors.Is(err, benchfrm.ErrGroupEmptyResult) {
            if errors.Is(err, context.DeadlineExceeded) {
                log.Fatal("All RPC calls timed out. Check network connectivity.")
            }
            log.Fatalf("All RPC calls failed: %v", err)
        }
        log.Fatalf("Unexpected error: %v", err)
    }

    reporter := benchfrm.NewTextReporter()
    fmt.Println(reporter.Report(results))
}
```

### 示例 5：使用内存存储进行 A/B 临时比较

不持久化到磁盘，仅在程序内对新旧两个实现进行对比。

```go
package main

import (
    "fmt"
    "solocoder-go/internal/benchfrm"
)

func main() {
    store := benchfrm.NewMemoryStore()
    b := benchfrm.NewBenchmarker()
    b.SetBaselineStore(store)

    // 第一次运行：旧实现作为基线
    b.AddGroup("process_data", func() error {
        result := oldProcess(largeDataset)
        _ = result
        return nil
    }, benchfrm.WithIterations(100), benchfrm.WithWarmupIterations(20))

    _, err := b.RunAll()
    if err != nil {
        panic(err)
    }
    if err := b.SaveBaseline(); err != nil {
        panic(err)
    }

    // 第二轮：运行新实现并与基线对比
    b2 := benchfrm.NewBenchmarker()
    b2.SetBaselineStore(store)

    b2.AddGroup("process_data", func() error {
        result := newProcess(largeDataset)
        _ = result
        return nil
    }, benchfrm.WithIterations(100), benchfrm.WithWarmupIterations(20))

    _, err = b2.RunAll()
    if err != nil {
        panic(err)
    }

    // 检查是否有回归
    report, err := b2.CheckRegression(5.0) // 5% 阈值
    if err != nil {
        panic(err)
    }

    reporter := benchfrm.NewTextReporter()
    fmt.Println(reporter.ReportRegression(report))

    if report.IsRegression {
        fmt.Println("❌ 新实现存在性能回归，建议优化后再合入。")
    } else {
        fmt.Println("✅ 新实现性能达标，可以合入。")
    }
}
```

---

## 并发安全说明

`Benchmarker` 及其内部的所有组件均设计为并发安全：

| 组件 | 同步机制 | 锁粒度说明 |
|-----|---------|-----------|
| `benchmarker` | `sync.RWMutex` | 读操作（Compare、CheckRegression、LoadBaseline）用读锁；写操作（AddGroup、RunAll、SaveBaseline、Set*）用写锁 |
| `MemoryStore` | `sync.RWMutex` | Save 用写锁，Load 用读锁 |
| `FileStore` | `sync.RWMutex` | Save 用写锁，Load 用读锁 |
| `TextReporter` | 无状态 | 无需锁，纯函数式输出 |

**使用建议：** 虽然框架本身是并发安全的，但通常基准测试应**串行执行**以避免 CPU 缓存争用、调度器抖动等导致数据失真。多个 `Benchmarker` 实例之间完全独立。

---

## 性能与开销

框架自身的运行开销非常小，对被测函数的性能影响可忽略：

| 开销项 | 典型值 | 说明 |
|-------|-------|------|
| 单次调用开销 | 50-200ns | 不含被测函数和内存统计 |
| 内存统计开销 | 1-2µs / 次 | 主要消耗在 `runtime.GC()` 和 `ReadMemStats` 系统调用 |
| goroutine 开销 | 约 2KB 栈空间 | 仅在启用超时（Timeout>0）时为每次运行额外创建 |
| 内存占用 | ~40B / 次迭代 | 存储 `RunResult`，100 万次迭代约 40MB |

**优化建议：**
- 不需要内存统计时，务必设置 `WithMemoryCollection(false)`，可减少 90% 以上的框架自身开销
- 预热次数无需超过 50 次，通常 10-30 次已足够让运行时稳定
- 迭代次数越多结果越稳定，但超过 10000 次后的边际收益很低（统计置信度提升不明显）
- 设置合理的超时值可以防止卡死，但过小的超时可能导致所有运行失败

---

## 测试覆盖范围

模块内置的单元测试覆盖以下维度（**总计 52 个测试用例**）：

| 类别 | 覆盖内容 | 测试数量 |
|-----|---------|---------|
| 配置验证 | 默认值检查、选项函数、Validate 边界（0 / 负数） | 5 |
| 组注册 | 正常注册、nil 函数 panic、重复名称 panic、非法配置 panic | 5 |
| 运行流程 | 无组错误、单组成功、多组对比、函数错误、预热隔离 | 7 |
| 超时控制 | 触发超时、无超时正常运行、大超时无影响 | 3 |
| 容错机制 | 部分失败跳过、全部失败返回 ErrGroupEmptyResult、错误链 unwrap | 3 |
| 内存统计 | 启用统计、禁用统计 | 2 |
| 统计计算 | 多值计算正确性、空切片处理 | 2 |
| 对比报告 | 正常对比、无结果、基线不存在、百分比正确性验证 | 4 |
| 回归检测 | 阈值验证、无存储、无结果、无基线、无回归、有回归 | 6 |
| 存储后端 | MemoryStore 读写、FileStore 读写与自动建目录 | 3 |
| 报告生成 | 基准报告、对比报告（含 Δ% 列）、回归报告（正常/告警） | 3 |
| 边界条件 | 零预热、单迭代（Min=Max=Mean、StdDev=0）、结构体字段验证 | 4 |
| 基线管理 | 保存、无存储保存、无结果保存、加载、无存储加载 | 5 |
| 错误变量 | 全部 10 个错误变量的定义与消息验证 | 1 |
| 并发安全 | 多 goroutine 并发访问存储与运行 | 1 |
| 选项集成 | 所有配置选项正确存储与传递验证 | 1 |
| **合计** | | **52** |

运行测试命令：
```bash
go test ./internal/benchfrm/ -v
```

---

## 扩展开发指南

### 自定义基线存储后端

实现 `BaselineStore` 接口，可接入数据库、Redis、对象存储、远程 HTTP 服务等任意后端。

**Redis 存储实现示例：**
```go
package benchfrm_ext

import (
    "context"
    "encoding/json"
    "errors"
    "github.com/redis/go-redis/v9"
    "solocoder-go/internal/benchfrm"
)

type RedisStore struct {
    client *redis.Client
    prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
    return &RedisStore{client: client, prefix: prefix}
}

func (s *RedisStore) Save(groupName string, stats benchfrm.GroupStatistics) error {
    data, err := json.Marshal(stats)
    if err != nil {
        return err
    }
    return s.client.Set(context.Background(), s.prefix+groupName, data, 0).Err()
}

func (s *RedisStore) Load(groupName string) (benchfrm.GroupStatistics, bool, error) {
    data, err := s.client.Get(context.Background(), s.prefix+groupName).Result()
    if errors.Is(err, redis.Nil) {
        return benchfrm.GroupStatistics{}, false, nil
    }
    if err != nil {
        return benchfrm.GroupStatistics{}, false, err
    }
    var stats benchfrm.GroupStatistics
    if err := json.Unmarshal([]byte(data), &stats); err != nil {
        return benchfrm.GroupStatistics{}, false, err
    }
    return stats, true, nil
}
```

### 自定义报告生成器

实现 `Reporter` 接口，可输出 JSON、HTML、Markdown、JUnit XML、Prometheus 指标等任意格式。

**JSON 报告实现示例：**
```go
package benchfrm_ext

import (
    "encoding/json"
    "solocoder-go/internal/benchfrm"
)

type JSONReporter struct {
    Indent bool
}

func NewJSONReporter(indent bool) *JSONReporter {
    return &JSONReporter{Indent: indent}
}

func (r *JSONReporter) Report(stats []benchfrm.GroupStatistics) string {
    return r.marshal(struct {
        Type    string                         `json:"type"`
        Results []benchfrm.GroupStatistics     `json:"results"`
    }{Type: "benchmark_results", Results: stats})
}

func (r *JSONReporter) ReportComparison(report benchfrm.ComparisonReport) string {
    return r.marshal(struct {
        Type   string                    `json:"type"`
        Report benchfrm.ComparisonReport `json:"report"`
    }{Type: "comparison_report", Report: report})
}

func (r *JSONReporter) ReportRegression(report benchfrm.RegressionReport) string {
    return r.marshal(struct {
        Type   string                    `json:"