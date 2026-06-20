# Benchfrm 基准测试框架模块

## 模块概述

`internal/benchfrm` 包提供了一个功能完整、生产级的 Go 基准测试框架。框架支持对任意函数进行性能基准测试，具备重复运行统计、预热阶段、内存分配统计、超时控制、多组对比报告、性能回归自动检测等核心能力。模块通过接口抽象，支持扩展自定义的基线存储后端和报告输出格式。

---

## 核心功能详解

### 1. 重复运行与统计指标

框架支持对被测函数执行指定次数的重复运行，通过多次采样消除偶然波动，得到可靠的性能数据。

**统计指标定义：**

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `Name` | `string` | 基准测试组的名称，用于标识和区分不同测试项 |
| `Iterations` | `int` | 实际执行的正式迭代次数（不含预热） |
| `MeanDuration` | `time.Duration` | 所有正式运行的**算术平均耗时**，反映整体性能水平 |
| `MinDuration` | `time.Duration` | 所有正式运行中的**最短耗时**，通常代表代码的最佳性能 |
| `MaxDuration` | `time.Duration` | 所有正式运行中的**最长耗时**，反映最坏情况和性能波动幅度 |
| `StdDevDuration` | `time.Duration` | 耗时的**标准差**，衡量数据的离散程度；值越小说明性能越稳定 |
| `MeanAllocBytes` | `uint64` | 每次运行平均分配的**字节数**，反映内存使用效率 |
| `MeanAllocCount` | `uint64` | 每次运行平均分配的**次数**，反映对象创建频率 |

**统计计算公式：**
- 平均耗时：`MeanDuration = TotalDuration / N`
- 标准差：`StdDevDuration = sqrt(Σ(xi - Mean)² / N)`

### 2. 预热阶段（Warmup）

Go 运行时具有 JIT 编译（在部分实现中）、CPU 分支预测、缓存预热、内存分配器初始化等特性。为了获得稳定的基准测试结果，框架支持在正式计时前执行预热迭代。

**预热阶段特性：**
- 预热迭代**完全不计入**统计结果
- 预热阶段**不计时、不统计内存**
- 预热次数和正式运行次数**可独立配置**
- 预热阶段的函数错误**会被静默忽略**，不影响后续执行
- 默认预热次数：10 次

### 3. 内存分配统计

通过 Go 标准库 `runtime.MemStats` 在每次运行前后采样内存状态，精确统计被测函数的内存分配行为。

**实现原理：**
1. 运行前执行 `runtime.GC()` 触发垃圾回收以获得干净的起始状态
2. 通过 `runtime.ReadMemStats(&startMem)` 读取起始内存统计
3. 执行被测函数并计时
4. 通过 `runtime.ReadMemStats(&endMem)` 读取结束内存统计
5. 计算差值：`AllocBytes = endMem.TotalAlloc - startMem.TotalAlloc`，`AllocCount = endMem.Mallocs - startMem.Mallocs`

**注意事项：**
- 内存统计仅在**正式运行**期间累计，预热阶段不影响统计
- 可通过配置 `WithMemoryCollection(false)` 禁用内存统计以加速测试
- GC 的存在可能导致内存统计存在微小误差，建议通过多次运行取平均

### 4. 超时控制

支持为每次运行设置超时时间，防止被测函数意外卡死影响整体测试流程。

**超时机制：**
- 使用 `context.WithTimeout` 实现超时控制
- 超时时间通过 `WithTimeout(d time.Duration)` 配置，单位纳秒到小时均可
- 默认超时值为 `0`，表示**不启用超时**
- 超时触发时，被测函数所在的 goroutine **无法被强制终止**，但框架会立即返回 `context.DeadlineExceeded` 错误
- **重要提示：** 超时后的 goroutine 会继续在后台执行，务必确保被测函数最终能正常结束

### 5. 多组对比报告

支持注册多个基准测试组（可理解为不同的实现方案或不同的参数配置），运行完成后生成相对基线的对比报告。

**对比维度：**
- 平均耗时变化百分比（Duration Δ%）
- 内存分配字节数变化百分比（Bytes Δ%）
- 内存分配次数变化百分比（Allocs Δ%）

**百分比计算：**
```
变化率 = (当前值 - 基线值) / 基线值 × 100%
```
- 正值（带 ↑ 箭头）：当前性能**劣于**基线
- 负值（带 ↓ 箭头）：当前性能**优于**基线
- 零值：与基线完全一致

### 6. 性能回归自动检测

将当前基准测试结果与已保存的历史基线进行比较，当任意关键指标的劣化超过配置的阈值时，自动标记为性能回归并输出详细的告警信息。

**检测的关键指标：**
| 指标名称 | 含义 | 阈值判断条件 |
|---------|------|-------------|
| `MeanDuration` | 平均耗时 | `(Current - Baseline) / Baseline × 100% > Threshold` |
| `MeanAllocBytes` | 平均分配字节数 | `(Current - Baseline) / Baseline × 100% > Threshold` |
| `MeanAllocCount` | 平均分配次数 | `(Current - Baseline) / Baseline × 100% > Threshold` |

**告警信息包含：**
- 劣化的指标名称
- 当前值与基线值
- 劣化百分比与配置阈值
- 明确的 REGRESSED 标记

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

**实现类：** `benchmarker`（私有结构体，使用 `sync.RWMutex` 保证并发安全）

#### BaselineStore（基线存储接口）

抽象基线数据的持久化层，支持多种存储后端。

```go
type BaselineStore interface {
    Save(groupName string, stats GroupStatistics) error
    Load(groupName string) (GroupStatistics, bool, error)
}
```

**内置实现：**

| 实现类 | 存储方式 | 适用场景 |
|-------|---------|---------|
| `MemoryStore` | 进程内内存 map | 单元测试、临时比较 |
| `FileStore` | 磁盘 JSON 文件 | CI/CD 流水线、跨运行对比 |

#### Reporter（报告生成器接口）

抽象报告的输出格式，支持扩展自定义格式（如 JSON、HTML、Markdown）。

```go
type Reporter interface {
    Report(stats []GroupStatistics) string
    ReportComparison(report ComparisonReport) string
    ReportRegression(report RegressionReport) string
}
```

**内置实现：** `TextReporter` —— 人可读的纯文本格式，适合控制台输出。

### 数据结构

#### RunConfig（运行配置）

单个基准测试组的运行参数配置。

| 字段 | 类型 | 默认值 | 说明 |
|-----|------|-------|------|
| `Iterations` | `int` | 100 | 正式运行迭代次数，必须 > 0 |
| `WarmupIterations` | `int` | 10 | 预热迭代次数，必须 >= 0 |
| `CollectMemory` | `bool` | `true` | 是否收集内存分配统计 |
| `Timeout` | `time.Duration` | 0 | 单次运行超时时间，0 表示无超时 |

**选项函数（Functional Options）：**
- `WithIterations(n int)` —— 设置正式运行次数
- `WithWarmupIterations(n int)` —— 设置预热次数
- `WithMemoryCollection(enabled bool)` —— 开关内存统计
- `WithTimeout(d time.Duration)` —— 设置单次运行超时

#### BenchmarkFunc（被测函数类型）

```go
type BenchmarkFunc func() error
```

基准测试框架执行的目标函数签名。函数返回 `error` 时，框架会立即中止当前基准测试组并向上传递错误。

#### RunResult（单次运行原始结果）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Duration` | `time.Duration` | 本次运行的实际耗时 |
| `AllocBytes` | `uint64` | 本次运行分配的字节数 |
| `AllocCount` | `uint64` | 本次运行的分配次数 |
| `Error` | `error` | 本次运行返回的错误 |

#### GroupStatistics（组统计结果）

完整的单组基准测试统计结果，结构定义见「核心功能详解 → 重复运行与统计指标」。

#### ComparisonReport（对比报告）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Baseline` | `string` | 基线组的名称 |
| `Items` | `[]ComparisonItem` | 所有组的对比项列表 |
| `GeneratedAt` | `time.Time` | 报告生成的时间戳 |

#### ComparisonItem（对比项）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `Group` | `string` | 测试组名称 |
| `MeanDuration` | `time.Duration` | 该组的平均耗时 |
| `MeanAllocBytes` | `uint64` | 该组的平均分配字节数 |
| `MeanAllocCount` | `uint64` | 该组的平均分配次数 |
| `VsBaselinePct` | `float64` | 耗时相对于基线的变化百分比 |
| `AllocBytesPct` | `float64` | 分配字节数相对于基线的变化百分比 |
| `AllocCountPct` | `float64` | 分配次数相对于基线的变化百分比 |

#### RegressionReport（回归检测报告）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `IsRegression` | `bool` | 是否存在性能回归（任意指标劣化超过阈值即为 true） |
| `Checks` | `[]RegressionCheck` | 各指标的详细检测结果 |
| `GeneratedAt` | `time.Time` | 报告生成的时间戳 |

#### RegressionCheck（单指标检测项）

| 字段 | 类型 | 说明 |
|-----|------|------|
| `MetricName` | `string` | 指标名称（`MeanDuration` / `MeanAllocBytes` / `MeanAllocCount`） |
| `CurrentValue` | `float64` | 当前运行的指标值 |
| `BaselineValue` | `float64` | 已保存基线的指标值 |
| `DegradationPct` | `float64