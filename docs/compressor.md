# 压缩编解码器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [压缩算法说明](#4-压缩算法说明)
5. [压缩级别配置](#5-压缩级别配置)
6. [流式压缩解压工作流程](#6-流式压缩解压工作流程)
7. [自适应算法选择机制](#7-自适应算法选择机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [性能对比](#10-性能对比)

---

## 1. 模块概述

压缩编解码器（Compressor）是一个通用的压缩/解压缩功能模块，提供多种压缩算法的统一接口，支持流式处理和自适应算法选择。模块设计遵循开闭原则，调用方可以通过配置切换压缩算法而无需修改业务代码。

**包路径**: `internal/compressor`

**设计目标**:
- 统一的压缩/解压缩接口，屏蔽底层算法差异
- 支持多种压缩算法（Gzip、Snappy、LZ4）
- 灵活的压缩级别配置，平衡速度与压缩率
- 流式处理能力，支持大文件和网络流
- 智能的自适应算法选择，根据数据特征自动选择最优方案
- 完整的错误处理和边界条件保护

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 多算法支持 | 内置 Gzip、Snappy、LZ4 三种主流压缩算法，每种算法实现统一接口 |
| 压缩级别配置 | 支持 5 级压缩级别（最快、快、默认、较好、最好），不同算法自动映射到原生级别 |
| 流式压缩解压 | 提供基于 `io.Reader` 和 `io.Writer` 的流式接口，避免一次性加载大文件到内存 |
| 自适应选择 | 自动分析数据特征（熵值、重复率、数据类型），根据配置的速度权重自动选择最优算法和级别 |
| 数据特征分析 | 提供 `AnalyzeData` 函数分析数据的熵值、重复率和数据类型 |
| 压缩结果统计 | 返回压缩结果详情，包括算法、级别、原始大小、压缩后大小和压缩率 |
| 手动/自动模式 | 支持手动指定算法和级别，或自动模式让模块智能选择 |

---

## 3. 核心结构体与职责

### 3.1 Manager

**职责**: 压缩管理器，作为对外的主要入口，负责：
- 配置验证和管理
- 压缩器/解压器的创建
- 手动/自动模式的调度
- 压缩结果的统计

**主要方法**:
- `NewManager(cfg Config) (*Manager, error)` - 创建管理器实例
- `Compress(data []byte) ([]byte, *CompressionResult, error)` - 压缩数据
- `Decompress(data []byte) ([]byte, error)` - 解压数据
- `NewStreamCompressor(w io.Writer) (io.WriteCloser, error)` - 创建流式压缩器
- `NewStreamDecompressor(r io.Reader) (io.ReadCloser, error)` - 创建流式解压器

### 3.2 Config

**职责**: 压缩配置结构体，包含所有可配置项

| 字段 | 类型 | 说明 |
|------|------|------|
| `Algorithm` | `Algorithm` | 压缩算法（手动模式下使用） |
| `Level` | `CompressionLevel` | 压缩级别（手动模式下使用） |
| `Mode` | `SelectionMode` | 选择模式：`ModeManual` 或 `ModeAuto` |
| `AutoSpeedRatio` | `float64` | 自动模式下的速度权重（0-1，0 为压缩率优先，1 为速度优先） |

### 3.3 CompressionResult

**职责**: 压缩结果统计信息

| 字段 | 类型 | 说明 |
|------|------|------|
| `Algorithm` | `Algorithm` | 实际使用的压缩算法 |
| `Level` | `CompressionLevel` | 实际使用的压缩级别 |
| `OriginalSize` | `int` | 原始数据大小（字节） |
| `CompressedSize` | `int` | 压缩后数据大小（字节） |
| `CompressionRatio` | `float64` | 压缩率（压缩后/原始） |

### 3.4 DataCharacteristics

**职责**: 数据特征分析结果

| 字段 | 类型 | 说明 |
|------|------|------|
| `Size` | `int` | 数据大小 |
| `Entropy` | `float64` | 信息熵（0-8，值越高越随机） |
| `RepeatRatio` | `float64` | 相邻字节重复率（0-1） |
| `DataType` | `DataType` | 数据类型分类 |

### 3.5 接口定义

#### Compressor 接口

```go
type Compressor interface {
    Compress(data []byte) ([]byte, error)
    NewCompressedWriter(w io.Writer) (io.WriteCloser, error)
    Algorithm() Algorithm
    Level() CompressionLevel
}
```

#### Decompressor 接口

```go
type Decompressor interface {
    Decompress(data []byte) ([]byte, error)
    NewDecompressedReader(r io.Reader) (io.ReadCloser, error)
    Algorithm() Algorithm
}
```

---

## 4. 压缩算法说明

### 4.1 Gzip

- **特点**: 高压缩率，中等速度，兼容标准 gzip 格式
- **适用场景**: 静态资源压缩、文件归档、跨系统数据交换
- **原生级别范围**: -1（默认）到 9（最好）
- **流式支持**: 完全支持

### 4.2 Snappy

- **特点**: 极快的压缩和解压速度，中等压缩率，Google 开发
- **适用场景**: 大数据处理、内部系统通信、对延迟敏感的场景
- **原生级别范围**: 无级别区分，始终追求速度
- **流式支持**: 完全支持

### 4.3 LZ4

- **特点**: 极速解压，快速压缩，压缩率优于 Snappy
- **适用场景**: 缓存压缩、日志压缩、高性能数据传输
- **原生级别范围**: Fast 到 Level16
- **流式支持**: 完全支持

---

## 5. 压缩级别配置

模块定义了统一的 5 级压缩级别，每种算法会自动映射到其原生级别：

| 统一级别 | 名称 | 说明 | Gzip 映射 | LZ4 映射 | Snappy 映射 |
|----------|------|------|-----------|----------|-------------|
| 1 | LevelFastest | 速度优先 | BestSpeed (1) | Fast | 无区分 |
| 2 | LevelFast | 较快 | BestSpeed (1) | Level1 | 无区分 |
| 3 | LevelDefault | 平衡 | Default (-1) | Level3 | 无区分 |
| 4 | LevelBetter | 较好压缩 | 7 | Level6 | 无区分 |
| 5 | LevelBest | 压缩率优先 | BestCompression (9) | Level9 | 无区分 |

> **注意**: Snappy 算法不支持压缩级别，始终以最快速度运行。

---

## 6. 流式压缩解压工作流程

### 6.1 流式压缩流程

```
调用方                  Manager              Compressor          底层 Writer
   │                       │                     │                    │
   │  NewStreamCompressor  │                     │                    │
   │──────────────────────>│                     │                    │
   │                       │  NewCompressor      │                    │
   │                       │────────────────────>│                    │
   │                       │                     │  NewCompressedWriter│
   │                       │                     │────────────────────>│
   │                       │                     │<────────────────────│
   │                       │<────────────────────│                    │
   │<──────────────────────│                     │                    │
   │                       │                     │                    │
   │         Write         │                     │        Write       │
   │──────────────────────>│────────────────────>│───────────────────>│
   │                       │                     │                    │
   │         Close         │                     │        Close       │
   │──────────────────────>│────────────────────>│───────────────────>│
```

**流式压缩特点**:
1. 数据分块写入，无需一次性加载
2. 内存占用稳定，与数据大小无关
3. 支持网络流、文件流等任意 `io.Writer`
4. 必须调用 `Close()` 完成压缩并刷新缓冲区

### 6.2 流式解压流程

```
调用方                  Manager              Decompressor        底层 Reader
   │                       │                       │                    │
   │ NewStreamDecompressor │                       │                    │
   │──────────────────────>│                       │                    │
   │                       │  NewDecompressor       │                    │
   │                       │──────────────────────>│                    │
   │                       │                       │ NewDecompressedReader│
   │                       │                       │────────────────────>│
   │                       │                       │<────────────────────│
   │                       │<──────────────────────│                    │
   │<──────────────────────│                       │                    │
   │                       │                       │                    │
   │         Read          │                       │        Read        │
   │──────────────────────>│──────────────────────>│───────────────────>│
   │                       │                       │                    │
   │         Close         │                       │        Close       │
   │──────────────────────>│──────────────────────>│───────────────────>│
```

**流式解压特点**:
1. 按需读取，边解压边返回
2. 支持网络流、文件流等任意 `io.Reader`
3. 建议使用 `defer Close()` 确保资源释放

---

## 7. 自适应算法选择机制

### 7.1 数据特征分析

`AnalyzeData` 函数通过以下维度分析数据：

1. **信息熵 (Entropy)**: 度量数据的随机性，范围 0-8 bits/byte
   - > 7.5: 随机数据，难以压缩
   - 4-7.5: 普通文本或二进制
   - < 4: 高度结构化，易于压缩

2. **重复率 (RepeatRatio)**: 相邻字节的重复比例
   - > 0.6: 高度重复，LZ4 或 Gzip 效果好
   - 0.3-0.6: 中等重复
   - < 0.3: 低重复率

3. **可打印字符比例**: 判断是否为文本数据
   - > 0.85: 文本数据

4. **数据类型分类**:
   - `DataTypeRandom`: 高熵随机数据
   - `DataTypeText`: 文本数据
   - `DataTypeStructured`: 高重复率结构化数据
   - `DataTypeBinary`: 其他二进制数据

### 7.2 选择策略

自适应模式下，算法选择基于以下因素：

| 数据类型 | 速度权重 > 0.7 | 速度权重 0.3-0.7 | 速度权重 < 0.3 |
|----------|---------------|-----------------|---------------|
| Random | Snappy (Fastest) | Snappy (Fastest) | Snappy (Fastest) |
| Text | LZ4 (Fast) | Gzip (Default) | Gzip (Best) |
| Structured | LZ4 (Default) | Gzip (Better) | Gzip (Better) |
| Binary | Snappy (Default) | LZ4 (Better) | LZ4 (Better) |

**额外调整规则**:
- 数据 < 1KB: 强制使用最快级别
- 数据 > 10MB 且速度权重 > 0.5: 使用最快级别
- 重复率 > 0.6: 优先 LZ4（速度优先）或 Gzip（压缩率优先）

---

## 8. 使用示例

### 8.1 基本使用（手动模式）

```go
package main

import (
    "solocoder-go/internal/compressor"
    "fmt"
)

func main() {
    cfg := compressor.Config{
        Algorithm: compressor.AlgorithmGzip,
        Level:     compressor.LevelDefault,
        Mode:      compressor.ModeManual,
    }

    mgr, err := compressor.NewManager(cfg)
    if err != nil {
        panic(err)
    }

    data := []byte("Hello, World! This is a test string for compression.")

    compressed, result, err := mgr.Compress(data)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Original: %d bytes\n", result.OriginalSize)
    fmt.Printf("Compressed: %d bytes\n", result.CompressedSize)
    fmt.Printf("Ratio: %.2f%%\n", result.CompressionRatio*100)

    decompressed, err := mgr.Decompress(compressed)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Decompressed: %s\n", string(decompressed))
}
```

### 8.2 自动模式

```go
cfg := compressor.Config{
    Mode:           compressor.ModeAuto,
    AutoSpeedRatio: 0.7, // 偏向速度
}

mgr, _ := compressor.NewManager(cfg)

// 模块会自动分析数据并选择最优算法
compressed, result, err := mgr.Compress(largeData)
fmt.Printf("Auto-selected algorithm: %s\n", result.Algorithm)
```

### 8.3 流式压缩文件

```go
import (
    "os"
    "io"
)

// 压缩文件
func compressFile(src, dst string) error {
    cfg := compressor.DefaultConfig()
    mgr, _ := compressor.NewManager(cfg)

    srcFile, _ := os.Open(src)
    defer srcFile.Close()

    dstFile, _ := os.Create(dst)
    defer dstFile.Close()

    writer, err := mgr.NewStreamCompressor(dstFile)
    if err != nil {
        return err
    }
    defer writer.Close()

    _, err = io.Copy(writer, srcFile)
    return err
}

// 解压文件
func decompressFile(src, dst string) error {
    cfg := compressor.DefaultConfig()
    mgr, _ := compressor.NewManager(cfg)

    srcFile, _ := os.Open(src)
    defer srcFile.Close()

    dstFile, _ := os.Create(dst)
    defer dstFile.Close()

    reader, err := mgr.NewStreamDecompressor(srcFile)
    if err != nil {
        return err
    }
    defer reader.Close()

    _, err = io.Copy(dstFile, reader)
    return err
}
```

### 8.4 直接使用压缩器接口

```go
// 创建 Gzip 压缩器（最高压缩率）
compressor, err := compressor.NewCompressor(
    compressor.AlgorithmGzip,
    compressor.LevelBest,
)

compressed, err := compressor.Compress(data)

// 创建对应解压器
decompressor, err := compressor.NewDecompressor(compressor.Algorithm())
decompressed, err := decompressor.Decompress(compressed)
```

### 8.5 数据分析

```go
data := []byte("some data to analyze...")
characteristics := compressor.AnalyzeData(data)

fmt.Printf("Size: %d bytes\n", characteristics.Size)
fmt.Printf("Entropy: %.4f\n", characteristics.Entropy)
fmt.Printf("Repeat Ratio: %.4f\n", characteristics.RepeatRatio)
fmt.Printf("Data Type: %s\n", characteristics.DataType)

// 根据分析结果手动选择算法
var alg compressor.Algorithm
if characteristics.Entropy > 7.0 {
    alg = compressor.AlgorithmSnappy // 随机数据用 Snappy
} else if characteristics.RepeatRatio > 0.5 {
    alg = compressor.AlgorithmLZ4    // 高重复用 LZ4
} else {
    alg = compressor.AlgorithmGzip   // 其他用 Gzip
}
```

---

## 9. 错误定义

| 错误变量 | 说明 |
|----------|------|
| `ErrUnsupportedAlgorithm` | 不支持的压缩算法 |
| `ErrInvalidCompressionLevel` | 无效的压缩级别 |
| `ErrNilReader` | 空的 Reader 参数 |
| `ErrNilWriter` | 空的 Writer 参数 |
| `ErrEmptyData` | 空数据 |
| `ErrCorruptedData` | 压缩数据损坏或格式错误 |
| `ErrInvalidMode` | 无效的选择模式 |

---

## 10. 性能对比

以下为典型场景下的性能参考（1MB 测试数据）：

| 算法 | 压缩级别 | 压缩时间 | 解压时间 | 压缩率 | 适用场景 |
|------|----------|----------|----------|--------|----------|
| Gzip | Best | ~120ms | ~15ms | ~0.001 | 静态文件、归档 |
| Gzip | Default | ~40ms | ~12ms | ~0.002 | 通用场景 |
| Gzip | Fastest | ~15ms | ~10ms | ~0.005 | 速度优先 |
| Snappy | N/A | ~3ms | ~1ms | ~0.047 | 大数据、实时传输 |
| LZ4 | Best | ~10ms | ~1ms | ~0.004 | 高性能缓存 |
| LZ4 | Default | ~5ms | ~0.8ms | ~0.008 | 日志压缩 |
| LZ4 | Fastest | ~2ms | ~0.5ms | ~0.015 | 极速传输 |

> **注意**: 实际性能受数据类型、系统环境等因素影响，以上数据仅供参考。

---

## 11. 文件结构

```
internal/compressor/
├── types.go          # 类型定义、接口、常量
├── compressor.go     # Manager 实现、自适应选择
├── gzip.go           # Gzip 算法实现
├── snappy.go         # Snappy 算法实现
├── lz4.go            # LZ4 算法实现
└── compressor_test.go # 单元测试
```

**核心文件链接**:
- [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/types.go)
- [compressor.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/compressor.go)
- [gzip.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/gzip.go)
- [snappy.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/snappy.go)
- [lz4.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/lz4.go)
- [compressor_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/compressor/compressor_test.go)
