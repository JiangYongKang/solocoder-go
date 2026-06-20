# 模糊测试驱动器 (Fuzz Driver) 模块

## 1. 模块概述

模糊测试驱动器是一个基于覆盖率引导的模糊测试框架，用于自动发现软件中的漏洞、崩溃和内存安全问题。该模块通过对被测函数执行字节级变异操作，生成大量测试输入，并监控代码覆盖率和内存使用情况，以发现潜在的安全漏洞和程序错误。

### 1.1 核心功能

- **覆盖率引导的输入变异**：通过位翻转、字节插入、字节删除和字节替换等操作生成变异输入，基于覆盖率反馈优化变异策略
- **语料库管理**：自动维护和扩展测试语料库，持久化存储到磁盘，支持从外部加载初始种子
- **Crash 输入保存与复现**：自动捕获并保存导致崩溃或错误的输入，支持后续复现和验证
- **内存安全检测**：基于基线校准的内存监控，识别异常内存使用模式
- **可扩展的覆盖率收集**：支持自定义覆盖率 Hook，默认基于 PC (程序计数器) 的真实执行路径追踪

---

## 2. 核心结构体与职责

### 2.1 `Coverage` - 覆盖率统计器

**职责**：跟踪和管理代码覆盖路径，支持并发安全的操作。

```go
type Coverage struct {
    mu      sync.RWMutex
    covered map[uint64]bool
}
```

**主要方法**：
- `Add(addr uint64)`：添加一个覆盖地址
- `Has(addr uint64) bool`：检查地址是否已被覆盖
- `Count() int`：返回当前覆盖路径数量
- `Merge(other *Coverage) int`：合并另一个覆盖率数据，返回新增路径数
- `Clear()`：清空所有覆盖率数据
- `Snapshot() []uint64`：返回排序后的覆盖地址快照

### 2.2 `Mutator` - 输入变异器

**职责**：对输入字节流执行各种变异操作，生成新的测试用例。

```go
type Mutator struct {
    mu sync.Mutex
}
```

**主要变异方法**：
- `FlipBit(input []byte) []byte`：随机翻转输入中的某一位
- `InsertByte(input []byte, maxSize int) []byte`：在随机位置插入一个随机字节
- `DeleteByte(input []byte) []byte`：随机删除一个字节
- `ReplaceByte(input []byte) []byte`：随机替换一个字节为新值
- `Mutate(input []byte, maxSize int) []byte`：随机选择一种变异方式
- `MutateN(input []byte, n, maxSize int) []byte`：执行 n 次连续变异

### 2.3 `Corpus` - 语料库管理器

**职责**：管理测试语料库，包括种子输入的加载、存储、轮询选择和持久化。

```go
type Corpus struct {
    mu         sync.RWMutex
    inputs     [][]byte
    currentIdx int
    dir        string
}
```

**主要方法**：
- `Add(input []byte)`：向语料库添加新输入
- `Count() int`：返回语料库大小
- `Next() []byte`：按轮询策略获取下一个种子输入
- `GetAll() [][]byte`：获取所有输入的副本
- `Load() error`：从磁盘目录加载语料库
- `Save(input []byte) error`：将输入持久化到磁盘

### 2.4 `Fuzzer` - 模糊测试主驱动器

**职责**：协调整个模糊测试流程，包括输入变异、执行、覆盖率分析、内存监控和结果收集。

```go
type Fuzzer struct {
    config              FuzzerConfig
    target              TargetFunc
    corpus              *Corpus
    mutator             *Mutator
    globalCoverage      *Coverage
    memoryBaseline      MemoryBaseline
    baselineSamples     []BaselineSample
    coverageHook        CoverageHook
    suspiciousRecords   []SuspiciousMemoryRecord
    crashRecords        []CrashRecord
    stats               FuzzerStats
    statsMu             sync.Mutex
    stopChan            chan struct{}
    stopped             bool
    mu                  sync.Mutex
}
```

**主要方法**：
- `NewFuzzer(target TargetFunc, config FuzzerConfig) (*Fuzzer, error)`：创建新的模糊测试器
- `AddSeed(input []byte) error`：添加初始种子输入
- `Run() error`：启动模糊测试主循环
- `Stop()`：停止模糊测试
- `Stats() FuzzerStats`：获取当前统计信息
- `CrashRecords() []CrashRecord`：获取所有崩溃记录
- `SuspiciousRecords() []SuspiciousMemoryRecord`：获取所有可疑内存记录
- `LoadCrashInput(path string) ([]byte, error)`：从磁盘加载崩溃输入
- `Reproduce(input []byte) error`：使用保存的输入复现问题（会直接抛出原始 panic）
- `CalibrateMemoryBaseline() error`：执行内存基线校准
- `GetMemoryBaseline() MemoryBaseline`：获取当前内存基线数据

### 2.5 `CoverageHook` - 覆盖率收集钩子

**职责**：定义覆盖率收集的扩展点，允许用户注入自定义的覆盖率收集逻辑。

```go
type CoverageHook func(input []byte) []uint64
```

**内置实现**：
- `DefaultCoverageHook(depth int) CoverageHook`：基于 `runtime.Callers` 获取真实程序计数器 (PC) 的执行路径
- `InputBasedCoverageHook(input []byte) []uint64`：基于输入内容生成确定性覆盖标记（用于测试和调试）

### 2.6 `MemoryBaseline` - 内存基线数据

**职责**：存储被测函数正常内存使用的统计数据，用于异常检测的基准。

```go
type MemoryBaseline struct {
    AvgAllocatedBytes   float64 // 平均分配字节数
    AvgNumAllocations   float64 // 平均分配次数
    MaxAllocatedBytes   uint64  // 最大分配字节数
    MaxNumAllocations   uint64  // 最大分配次数
    MinAllocatedBytes   uint64  // 最小分配字节数
    MinNumAllocations   uint64  // 最小分配次数
    StdDevAllocated     float64 // 分配字节数标准差
    StdDevAllocations   float64 // 分配次数标准差
    Calibrated          bool    // 是否已完成校准
}
```

### 2.7 `BaselineSample` - 基线样本

**职责**：存储单次基线采样的内存数据。

```go
type BaselineSample struct {
    AllocatedBytes uint64 // 分配字节数增量
    NumAllocations uint64 // 分配次数增量
}

### 2.8 配置与数据结构

#### `FuzzerConfig` - 模糊测试配置

```go
type FuzzerConfig struct {
    FunctionName              string        // 被测函数名称
    CorpusDir                 string        // 语料库目录
    CrashDir                  string        // 崩溃输入保存目录
    MaxInputSize              int           // 最大输入大小 (默认 64KB)
    MemoryThreshold           uint64        // 内存分配字节阈值 (默认 10MB)
    MemoryAllocThreshold      uint64        // 内存分配次数阈值 (默认 1000)
    MemoryMultiplier          float64       // 基线相对阈值倍数 (默认 5x)
    MutationsPerInput         int           // 每个种子的变异次数 (默认 100)
    MaxIterations             int           // 最大迭代次数 (0 表示无限制)
    MaxDuration               time.Duration // 最大运行时长 (0 表示无限制)
    CoverageHook              CoverageHook  // 自定义覆盖率收集 Hook
    CoverageTraceDepth        int           // 覆盖率追踪栈深度 (默认 10)
    BaselineRuns              int           // 每个种子的基线运行次数 (默认 10)
    EnableBaselineCalibration bool          // 是否启用基线校准 (默认 true)
}
```

#### `FuzzerStats` - 运行统计信息

```go
type FuzzerStats struct {
    TotalIterations  int64         // 总执行次数
    NewPathsFound    int64         // 新发现路径数
    CrashesFound     int64         // 发现崩溃数
    SuspiciousMemory int64         // 可疑内存行为数
    CorpusSize       int           // 当前语料库大小
    StartTime        time.Time     // 开始时间
    CurrentDuration  time.Duration // 已运行时长
}
```

#### `CrashRecord` - 崩溃记录

```go
type CrashRecord struct {
    Input        []byte    // 导致崩溃的输入
    Timestamp    time.Time // 发生时间
    FunctionName string    // 被测函数名
    Error        string    // 错误信息
}
```

#### `SuspiciousMemoryRecord` - 可疑内存记录

```go
type SuspiciousMemoryRecord struct {
    Input          []byte    // 触发可疑行为的输入
    Timestamp      time.Time // 发生时间
    AllocatedDiff  uint64    // 内存分配增量
    AllocationDiff uint64    // 分配次数增量
    Threshold      uint64    // 检测阈值
}
```

---

## 3. 模糊测试完整执行流程

### 3.1 初始化阶段

```
用户调用 NewFuzzer()
    ↓
验证目标函数和配置参数
    ↓
初始化覆盖率 Hook (默认使用 PC 追踪)
    ↓
创建语料库管理器并从磁盘加载现有语料
    ↓
创建变异器、覆盖率统计器
    ↓
返回 Fuzzer 实例
```

### 3.2 测试准备阶段

```
用户调用 AddSeed() 添加初始种子 (可选)
    ↓
种子输入被添加到语料库
```

### 3.3 基线校准阶段 (可选)

```
调用 Run() 启动测试
    ↓
如果语料库为空，自动添加默认种子
    ↓
如果启用基线校准:
    ├─ 对每个种子运行 N 次 (BaselineRuns)
    ├─ 记录每次运行的内存分配增量
    ├─ 计算平均值、最大值、最小值、标准差
    └─ 标记基线校准完成
```

### 3.4 主测试循环

```
┌─────────────────────────────────────────────────┐
│  循环直到满足停止条件 (迭代数/时长/手动停止)   │
│    ↓                                            │
│  从语料库轮询选择一个种子输入                    │
│    ↓                                            │
│  对种子执行 N 次变异操作                         │
│    ↓                                            │
│  对每个变异输入执行 processInput():              │
│    ├─ 执行前记录内存统计                        │
│    ├─ 执行前调用 CoverageHook 收集 PC           │
│    ├─ 安全执行被测函数 (捕获 panic)             │
│    ├─ 执行后调用 CoverageHook 收集 PC           │
│    ├─ 执行后记录内存统计                        │
│    ├─ 检查是否崩溃或返回错误 → 保存 Crash       │
│    ├─ 检查内存使用是否异常 (双维度检测)         │
│    │   ├─ 分配字节量 vs 阈值                    │
│    │   └─ 分配次数 vs 阈值                      │
│    ├─ 检查覆盖率是否有新增 → 加入语料库        │
│    └─ 更新统计信息                              │
└─────────────────────────────────────────────────┘
    ↓
测试结束，返回 nil
```

### 3.5 Crash 复现行为约定

`Reproduce()` 方法的行为约定：

1. **不做任何包装**：直接调用目标函数，不添加任何 panic 捕获或错误包装
2. **保留原始类型**：如果目标函数发生 panic，原始 panic 值会被直接抛出，类型信息完整保留
3. **错误原样返回**：如果目标函数返回错误，错误会被原样返回
4. **调用方负责处理**：使用 `Reproduce()` 时，调用方必须自行使用 `recover()` 来捕获可能的 panic

```go
// 正确的复现方式
defer func() {
    if r := recover(); r != nil {
        // r 是原始 panic 值，保持完整类型信息
        switch v := r.(type) {
        case MyCustomError:
            // 处理自定义错误类型
        default:
            // 其他类型
        }
    }
}()
err := fuzzer.Reproduce(crashInput)
```

### 3.4 结果处理阶段

```
测试结束后
    ↓
调用 Stats() 获取运行统计
    ↓
调用 CrashRecords() 获取所有崩溃
    ↓
调用 SuspiciousRecords() 获取可疑内存记录
    ↓
使用 LoadCrashInput() 和 Reproduce() 复现问题
```

---

## 4. 使用示例

### 4.1 基本使用

```go
package main

import (
    "errors"
    "fmt"
    "time"

    "solocoder-go/internal/fuzzdriver"
)

// 被测函数示例
func myFunction(input []byte) error {
    if len(input) >= 3 && input[0] == 'F' && input[1] == 'U' && input[2] == 'Z' && input[3] == 'Z' {
        panic("crash triggered!")
    }
    if len(input) >= 2 && input[0] == 'E' && input[1] == 'R' {
        return errors.New("error triggered")
    }
    return nil
}

func main() {
    // 创建配置
    config := fuzzdriver.DefaultConfig("myFunction")
    config.MaxIterations = 1000
    config.MaxDuration = 30 * time.Second
    config.MutationsPerInput = 50

    // 创建模糊测试器
    fuzzer, err := fuzzdriver.NewFuzzer(myFunction, config)
    if err != nil {
        panic(err)
    }

    // 添加初始种子
    fuzzer.AddSeed([]byte("FUZZ"))
    fuzzer.AddSeed([]byte("ERROR"))
    fuzzer.AddSeed([]byte("test123"))

    // 运行模糊测试
    fmt.Println("Starting fuzz test...")
    err = fuzzer.Run()
    if err != nil {
        fmt.Printf("Fuzz test error: %v\n", err)
    }

    // 获取统计信息
    stats := fuzzer.Stats()
    fmt.Printf("\n=== Fuzz Test Results ===\n")
    fmt.Printf("Total iterations: %d\n", stats.TotalIterations)
    fmt.Printf("New paths found: %d\n", stats.NewPathsFound)
    fmt.Printf("Crashes found: %d\n", stats.CrashesFound)
    fmt.Printf("Suspicious memory: %d\n", stats.SuspiciousMemory)
    fmt.Printf("Corpus size: %d\n", stats.CorpusSize)
    fmt.Printf("Duration: %v\n", stats.CurrentDuration)

    // 检查崩溃记录
    crashes := fuzzer.CrashRecords()
    for i, crash := range crashes {
        fmt.Printf("\nCrash %d:\n", i+1)
        fmt.Printf("  Input: %q\n", crash.Input)
        fmt.Printf("  Error: %s\n", crash.Error)
        fmt.Printf("  Time: %v\n", crash.Timestamp)
    }
}
```

### 4.2 Crash 复现

```go
// 加载并复现之前保存的 Crash 输入
crashPath := "crashes/myFunction/crash_myFunction_20260620T153000+0800_abcdef12"
crashInput, err := fuzzer.LoadCrashInput(crashPath)
if err != nil {
    log.Fatal(err)
}

// 复现问题 (注意：这会重新触发 panic)
defer func() {
    if r := recover(); r != nil {
        fmt.Printf("Successfully reproduced crash: %v\n", r)
    }
}()
err = fuzzer.Reproduce(crashInput)
```

### 4.3 从配置字符串创建

```go
opts := map[string]string{
    "functionname":      "myParser",
    "corpusdir":         "./myparser_corpus",
    "crashdir":          "./myparser_crashes",
    "maxinputsize":      "65536",
    "memorythreshold":   "10485760",
    "mutationsperinput": "100",
    "maxiterations":     "5000",
    "maxduration":       "1m",
}

config, err := fuzzdriver.ParseConfig(opts)
if err != nil {
    log.Fatal(err)
}

fuzzer, err := fuzzdriver.NewFuzzer(myParser, config)
```

### 4.4 生成随机种子

```go
// 生成 128 字节的随机种子
seed, err := fuzzdriver.GenerateRandomSeed(128)
if err != nil {
    log.Fatal(err)
}
fuzzer.AddSeed(seed)
```

---

## 5. 覆盖率统计实现方式

### 5.1 基于 PC (程序计数器) 的真实执行路径追踪

默认的覆盖率收集机制使用 `runtime.Callers` 获取调用栈的程序计数器 (PC) 值，真实反映代码执行路径。

**实现原理**：

```go
func DefaultCoverageHook(depth int) CoverageHook {
    return func(input []byte) []uint64 {
        pcs := make([]uintptr, depth)
        n := runtime.Callers(2, pcs)  // 跳过 runtime.Callers 和 Hook 本身
        result := make([]uint64, n)
        for i := 0; i < n; i++ {
            result[i] = uint64(pcs[i])
        }
        return result
    }
}
```

**执行流程**：

1. **执行前追踪**：调用目标函数前调用 `CoverageHook`，记录当前执行路径
2. **执行后追踪**：调用目标函数后再次调用 `CoverageHook`，记录执行后的路径（地址最高位设为 1 以区分）
3. **覆盖率标记**：
   - 错误返回时添加 `0xDEADBEEF` 标记
   - panic 时在 `recover` 中补充执行后的覆盖率数据，确保不丢失

### 5.2 自定义覆盖率 Hook

用户可以注入自定义的覆盖率收集逻辑，例如集成 Go 原生的覆盖测试工具或其他插桩框架。

**示例**：

```go
// 使用基于输入的确定性覆盖率 (适用于测试)
config.CoverageHook = fuzzdriver.InputBasedCoverageHook

// 自定义覆盖率收集
config.CoverageHook = func(input []byte) []uint64 {
    // 集成自定义的覆盖率工具
    return myCoverageCollector.GetCoveragePoints()
}
```

### 5.3 Panic 场景的覆盖率数据传递

为确保 panic 场景下覆盖率数据不丢失，采用两层 defer/recover 机制：

```
executeWithCoverage()
    ↓
    defer 函数 (第一层)
        ↓ 目标函数发生 panic
        执行目标函数
        ↓ panic 向上传播
    executeSafe()
        ↓
        defer 函数 (第二层，捕获 panic)
            ↓
            保存已收集的覆盖率数据
            补充执行后的覆盖率标记
            添加 panic 标记 0xDEADBEEF
            返回覆盖率、错误信息和 crashed=true
```

---

## 6. 内存检测的基线校准机制

### 6.1 基线校准流程

内存基线校准通过采集正常输入下的内存使用模式，建立检测基准，减少误报。

**校准步骤**：

1. **采样阶段**：对每个种子输入运行 N 次（默认 10 次）
2. **数据采集**：记录每次运行的内存分配增量（字节数和次数）
3. **统计计算**：计算平均值、最大值、最小值、标准差
4. **阈值计算**：使用 `平均值 × 倍率` 作为异常检测阈值

### 6.2 双维度异常检测

同时检测两个维度的内存使用异常：

| 维度 | 说明 | 阈值来源 |
|------|------|---------|
| **分配字节量** | 单次执行分配的内存字节数增量 | 已校准：`AvgAllocatedBytes × MemoryMultiplier` <br> 未校准：`MemoryThreshold` |
| **分配次数** | 单次执行的内存分配次数增量 | 已校准：`AvgNumAllocations × MemoryMultiplier` <br> 未校准：`MemoryAllocThreshold` |

**最小阈值保护**：即使已校准，阈值也不会低于配置的绝对阈值，避免基线异常导致漏检。

### 6.3 阈值计算示例

假设基线校准结果：
- 平均分配字节：1000 bytes
- 平均分配次数：50 次
- MemoryMultiplier：3.0
- MemoryThreshold：500 bytes
- MemoryAllocThreshold：20 次

计算阈值：
- 字节阈值 = max(1000 × 3.0, 500) = 3000 bytes
- 次数阈值 = max(50 × 3.0, 20) = 150 次

任何一次执行满足以下任一条件即被标记为可疑：
- 分配字节增量 > 3000 bytes
- 分配次数增量 > 150 次

---

## 7. 变异操作详解

### 7.1 位翻转 (Flip Bit)

随机选择输入中的某一位，将其取反（0 变 1，1 变 0）。

**示例**：
```
输入:  11111111 00000000 (0xFF, 0x00)
翻转第 3 位 (bit 3 of byte 0):
输出:  11110111 00000000 (0xF7, 0x00)
```

### 7.2 字节插入 (Insert Byte)

在随机位置插入一个随机字节值。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
在位置 1 插入 0xAB:
输出:  [0x01, 0xAB, 0x02, 0x03]
```

### 7.3 字节删除 (Delete Byte)

随机删除一个字节。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
删除位置 1:
输出:  [0x01, 0x03]
```

### 7.4 字节替换 (Replace Byte)

将随机位置的字节替换为新的随机值。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
替换位置 1 为 0xCD:
输出:  [0x01, 0xCD, 0x03]
```

---

## 8. 错误码列表

| 错误变量 | 说明 |
|---------|------|
| `ErrNilTargetFunction` | 目标函数为 nil |
| `ErrEmptyCorpus` | 语料库为空 |
| `ErrInvalidInput` | 无效输入 |
| `ErrCorpusDirNotFound` | 语料库目录未找到 |
| `ErrCrashDirNotFound` | Crash 目录未找到 |
| `ErrInvalidMaxInputSize` | 最大输入大小无效 (必须为正) |
| `ErrInvalidThreshold` | 内存阈值无效 (必须为正) |
| `ErrNilInput` | 输入为 nil 或空 |
| `ErrInputTooLarge` | 输入超过最大大小限制 |
| `ErrCorpusLoadFailed` | 加载语料库失败 |
| `ErrCrashSaveFailed` | 保存 Crash 输入失败 |
| `ErrCorpusSaveFailed` | 保存语料库输入失败 |
| `ErrBaselineCalibrationFailed` | 内存基线校准失败 |
| `ErrInvalidMultiplier` | 内存倍率无效 (必须大于 1) |

---

## 9. 常量配置

| 常量 | 默认值 | 说明 |
|------|--------|------|
| `DefaultCorpusDir` | `"corpus"` | 默认语料库根目录 |
| `DefaultCrashDir` | `"crashes"` | 默认 Crash 根目录 |
| `DefaultMaxInputSize` | `1 << 16` (64KB) | 默认最大输入大小 |
| `DefaultMemoryThreshold` | `10 * 1024 * 1024` (10MB) | 默认内存分配字节阈值 |
| `DefaultMemoryAllocThreshold` | `1000` | 默认内存分配次数阈值 |
| `DefaultMemoryMultiplier` | `5` | 默认基线相对阈值倍数 |
| `DefaultMutationsPerInput` | `100` | 每个种子的默认变异次数 |
| `DefaultCoverageTraceDepth` | `10` | 默认覆盖率追踪栈深度 |
| `DefaultBaselineRuns` | `10` | 每个种子的默认基线运行次数 |

---

## 10. 并发安全

本模块的所有公共方法都是并发安全的：
- `Coverage` 使用 `sync.RWMutex` 保护内部 map
- `Corpus` 使用 `sync.RWMutex` 保护输入列表和索引
- `Mutator` 使用 `sync.Mutex` 保护随机数生成
- `Fuzzer` 的统计信息使用 `sync.Mutex` 保护
- `Fuzzer` 的停止标志使用 `sync.Mutex` 保护

可以安全地在多个 goroutine 中调用 `Stats()`、`CrashRecords()`、`SuspiciousRecords()` 等方法。

---

## 11. 最佳实践

1. **提供高质量种子**：好的初始种子可以显著提高模糊测试效率
2. **启用基线校准**：使用 `EnableBaselineCalibration=true` 减少内存检测误报
3. **合理设置内存倍率**：`MemoryMultiplier` 建议设置在 2-5 倍之间
4. **定期持久化语料库**：框架会自动保存新增语料，无需手动干预
5. **监控测试进度**：定期调用 `Stats()` 了解测试进展
6. **及时复现和修复 Crash**：发现 Crash 后尽快使用 `Reproduce()` 复现问题
7. **根据需要选择覆盖率 Hook**：
   - 默认 PC 追踪适合大多数场景
   - `InputBasedCoverageHook` 适合需要确定性的测试场景
   - 自定义 Hook 适合集成专业覆盖率工具

---

## 12. 使用示例补充

### 12.1 使用自定义覆盖率 Hook

```go
// 使用基于输入的确定性覆盖率（适用于自动化测试）
config := fuzzdriver.DefaultConfig("myFunction")
config.CoverageHook = fuzzdriver.InputBasedCoverageHook
```

### 12.2 显式执行基线校准

```go
fuzzer, _ := fuzzdriver.NewFuzzer(myFunction, config)
fuzzer.AddSeed([]byte("seed1"))
fuzzer.AddSeed([]byte("seed2"))

// 手动执行基线校准
if err := fuzzer.CalibrateMemoryBaseline(); err != nil {
    log.Printf("Warning: baseline calibration failed: %v", err)
}

// 获取基线数据
baseline := fuzzer.GetMemoryBaseline()
log.Printf("Baseline: avg=%f bytes, avg=%f allocs", 
    baseline.AvgAllocatedBytes, baseline.AvgNumAllocations)
```

### 12.3 调整内存检测参数

```go
config := fuzzdriver.DefaultConfig("memoryIntensiveFunc")
config.EnableBaselineCalibration = true
config.MemoryMultiplier = 3.0      // 3 倍于平均水平
config.MemoryThreshold = 10 * 1024 * 1024  // 至少 10MB
config.MemoryAllocThreshold = 5000  // 至少 5000 次分配
```
