# 模糊测试驱动器 (Fuzz Driver) 模块

## 1. 模块概述

模糊测试驱动器是一个基于覆盖率引导的模糊测试框架，用于自动发现软件中的漏洞、崩溃和内存安全问题。该模块通过对被测函数执行字节级变异操作，生成大量测试输入，并监控代码覆盖率和内存使用情况，以发现潜在的安全漏洞和程序错误。

### 1.1 核心功能

- **覆盖率引导的输入变异**：通过位翻转、字节插入、字节删除和字节替换等操作生成变异输入，基于覆盖率反馈优化变异策略
- **语料库管理**：自动维护和扩展测试语料库，持久化存储到磁盘，支持从外部加载初始种子
- **Crash 输入保存与复现**：自动捕获并保存导致崩溃或错误的输入，支持后续复现和验证
- **内存安全检测**：监控内存分配行为，识别异常内存使用模式

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
    config            FuzzerConfig
    target            TargetFunc
    corpus            *Corpus
    mutator           *Mutator
    globalCoverage    *Coverage
    suspiciousRecords []SuspiciousMemoryRecord
    crashRecords      []CrashRecord
    stats             FuzzerStats
    statsMu           sync.Mutex
    stopChan          chan struct{}
    stopped           bool
    mu                sync.Mutex
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
- `Reproduce(input []byte) error`：使用保存的输入复现问题

### 2.5 配置与数据结构

#### `FuzzerConfig` - 模糊测试配置

```go
type FuzzerConfig struct {
    FunctionName      string        // 被测函数名称
    CorpusDir         string        // 语料库目录
    CrashDir          string        // 崩溃输入保存目录
    MaxInputSize      int           // 最大输入大小 (默认 64KB)
    MemoryThreshold   uint64        // 内存异常阈值 (默认 10MB)
    MutationsPerInput int           // 每个种子的变异次数 (默认 100)
    MaxIterations     int           // 最大迭代次数 (0 表示无限制)
    MaxDuration       time.Duration // 最大运行时长 (0 表示无限制)
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

### 3.3 主测试循环

```
调用 Run() 启动测试
    ↓
如果语料库为空，自动添加默认种子
    ↓
┌─────────────────────────────────────────────────┐
│  循环直到满足停止条件 (迭代数/时长/手动停止)   │
│    ↓                                            │
│  从语料库轮询选择一个种子输入                    │
│    ↓                                            │
│  对种子执行 N 次变异操作                         │
│    ↓                                            │
│  对每个变异输入执行 processInput():              │
│    ├─ 执行前记录内存统计                        │
│    ├─ 安全执行被测函数 (捕获 panic)             │
│    ├─ 执行后记录内存统计                        │
│    ├─ 检查是否崩溃或返回错误 → 保存 Crash       │
│    ├─ 检查内存使用是否异常 → 记录可疑行为       │
│    ├─ 检查覆盖率是否有新增 → 加入语料库        │
│    └─ 更新统计信息                              │
└─────────────────────────────────────────────────┘
    ↓
测试结束，返回 nil
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

## 5. 变异操作详解

### 5.1 位翻转 (Flip Bit)

随机选择输入中的某一位，将其取反（0 变 1，1 变 0）。

**示例**：
```
输入:  11111111 00000000 (0xFF, 0x00)
翻转第 3 位 (bit 3 of byte 0):
输出:  11110111 00000000 (0xF7, 0x00)
```

### 5.2 字节插入 (Insert Byte)

在随机位置插入一个随机字节值。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
在位置 1 插入 0xAB:
输出:  [0x01, 0xAB, 0x02, 0x03]
```

### 5.3 字节删除 (Delete Byte)

随机删除一个字节。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
删除位置 1:
输出:  [0x01, 0x03]
```

### 5.4 字节替换 (Replace Byte)

将随机位置的字节替换为新的随机值。

**示例**：
```
输入:  [0x01, 0x02, 0x03]
替换位置 1 为 0xCD:
输出:  [0x01, 0xCD, 0x03]
```

---

## 6. 错误码列表

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

---

## 7. 常量配置

| 常量 | 默认值 | 说明 |
|------|--------|------|
| `DefaultCorpusDir` | `"corpus"` | 默认语料库根目录 |
| `DefaultCrashDir` | `"crashes"` | 默认 Crash 根目录 |
| `DefaultMaxInputSize` | `1 << 16` (64KB) | 默认最大输入大小 |
| `DefaultMemoryThreshold` | `10 * 1024 * 1024` (10MB) | 默认内存分配阈值 |
| `DefaultMutationsPerInput` | `100` | 每个种子的默认变异次数 |

---

## 8. 并发安全

本模块的所有公共方法都是并发安全的：
- `Coverage` 使用 `sync.RWMutex` 保护内部 map
- `Corpus` 使用 `sync.RWMutex` 保护输入列表和索引
- `Mutator` 使用 `sync.Mutex` 保护随机数生成
- `Fuzzer` 的统计信息使用 `sync.Mutex` 保护
- `Fuzzer` 的停止标志使用 `sync.Mutex` 保护

可以安全地在多个 goroutine 中调用 `Stats()`、`CrashRecords()`、`SuspiciousRecords()` 等方法。

---

## 9. 最佳实践

1. **提供高质量种子**：好的初始种子可以显著提高模糊测试效率
2. **合理设置内存阈值**：根据被测函数的正常内存使用情况调整阈值
3. **定期持久化语料库**：框架会自动保存新增语料，无需手动干预
4. **监控测试进度**：定期调用 `Stats()` 了解测试进展
5. **及时复现和修复 Crash**：发现 Crash 后尽快使用 `Reproduce()` 复现问题
