# 数据去重引擎模块

## 1. 模块概述

数据去重引擎模块（datadedup）是一个高性能、可扩展的数据去重解决方案，支持多种去重策略，包括精确去重、模糊去重和分块去重。该模块提供了统一的 API 接口，支持索引持久化，适用于需要检测和消除重复数据的各种应用场景。

### 1.1 核心功能

- **精确去重**：基于哈希指纹的精确内容比对
- **模糊去重**：基于局部敏感哈希（SimHash）的相似度阈值判定
- **分块去重**：支持固定大小和内容边界的分块策略，适用于大块数据
- **持久化存储**：支持索引的磁盘持久化和恢复，带完整性校验
- **可插拔架构**：相似度计算、分块策略、持久化方式均可自定义替换

## 2. 核心结构体职责

### 2.1 核心接口

#### `DedupEngine` 接口
统一的去重引擎接口，定义了所有去重模式的公共操作。

```go
type DedupEngine interface {
    Check(data []byte) (*DedupResult, error)    // 检查数据是否重复
    Add(data []byte) error                      // 添加数据到索引
    CheckAndAdd(data []byte) (*DedupResult, error)  // 检查并添加（原子操作）
    Contains(data []byte) (bool, error)        // 判断数据是否存在
    Delete(data []byte) error                  // 删除数据索引
    Clear() error                              // 清空所有索引
    Count() int                                // 获取索引数量
    Save(path string) error                    // 保存索引到磁盘
    Load(path string) error                    // 从磁盘加载索引
    Close() error                              // 关闭引擎
}
```

#### `SimilarityCalculator` 接口
相似度计算器接口，用于模糊去重模式。

```go
type SimilarityCalculator interface {
    Calculate(data []byte) (Fingerprint, error)  // 计算数据的相似度指纹
    Similarity(fp1, fp2 Fingerprint) (float64, error)  // 计算两个指纹的相似度
    Algorithm() SimilarityAlgorithm              // 获取算法名称
}
```

#### `Chunker` 接口
数据分块器接口，用于分块去重模式。

```go
type Chunker interface {
    Chunk(data []byte) ([]Chunk, error)  // 将数据切分为多个块
    Strategy() ChunkStrategy             // 获取分块策略名称
}
```

#### `PersistIndex` 接口
索引持久化接口，支持索引的保存和加载。

```go
type PersistIndex interface {
    Save(index FingerprintIndex, path string) error   // 保存完整索引
    Load(path string) (FingerprintIndex, error)       // 加载索引
    Append(fp Fingerprint, path string) error         // 增量追加指纹
    Verify(path string) (bool, error)                 // 验证文件完整性
}
```

#### `HashProvider` 接口
哈希提供者接口，用于计算数据的哈希指纹。

```go
type HashProvider interface {
    Hash(data []byte) (Fingerprint, error)  // 计算数据的哈希指纹
    Algorithm() HashAlgorithm               // 获取哈希算法名称
}
```

### 2.2 核心数据结构

#### `Config`
引擎配置结构体，用于配置去重引擎的各项参数。

```go
type Config struct {
    Mode                DedupMode              // 去重模式
    HashAlgorithm       HashAlgorithm          // 哈希算法
    SimilarityAlgo      SimilarityAlgorithm    // 相似度算法
    SimilarityThreshold float64                // 相似度阈值 (0-1)
    ChunkStrategy       ChunkStrategy          // 分块策略
    ChunkSize           int                    // 分块大小（固定大小模式）
    ContentBoundary     byte                   // 内容边界字节（内容模式）
    MinChunkSize        int                    // 最小分块大小
    MaxChunkSize        int                    // 最大分块大小
    PersistPath         string                 // 持久化文件路径
    AutoPersist         bool                   // 是否自动持久化
    AutoPersistCount    int                    // 自动持久化触发阈值
    SimilarityCalc      SimilarityCalculator   // 自定义相似度计算器
    Chunker             Chunker                // 自定义分块器
    Persister           PersistIndex           // 自定义持久化器
    HashProvider        HashProvider           // 自定义哈希提供者
}
```

#### `DedupResult`
去重结果结构体，包含去重检查的详细信息。

```go
type DedupResult struct {
    IsDuplicate   bool           // 是否为重复数据
    MatchedFPs    []Fingerprint  // 匹配的指纹列表
    Similarity    float64        // 最高相似度（模糊模式）
    MatchedChunks []Chunk        // 匹配的块列表（分块模式）
}
```

#### `Chunk`
数据块结构体，用于分块去重模式。

```go
type Chunk struct {
    Data        []byte       // 块数据
    Offset      int64        // 在原始数据中的偏移量
    Fingerprint Fingerprint  // 块的哈希指纹
}
```

## 3. 去重模式详解

### 3.1 精确去重模式（Exact Mode）

**适用场景**：
- 需要完全精确匹配的场景
- 文档、代码、二进制文件等需要严格去重的数据
- 对误判零容忍的系统

**工作原理**：
1. 对输入数据的完整内容计算加密哈希指纹（SHA256/SHA1/MD5）
2. 将指纹与内存索引中的已有指纹进行比对
3. 指纹相同即判定为重复数据

**特点**：
- 准确率 100%，哈希碰撞可忽略不计
- 性能高，时间复杂度 O(1)
- 只能检测完全相同的数据

### 3.2 模糊去重模式（Fuzzy Mode）

**适用场景**：
- 文本内容去重（如文章、新闻、评论）
- 近似重复数据检测
- 需要容忍微小差异的场景

**工作原理**：
1. 使用 SimHash 算法计算数据的局部敏感哈希指纹
2. 计算新数据指纹与已有指纹的汉明距离
3. 将汉明距离转换为相似度（0-1）
4. 相似度超过配置阈值即判定为重复

**特点**：
- 支持相似度阈值配置（0-1）
- 可检测近似重复数据
- 相似度计算方式可插拔替换
- 时间复杂度 O(n)，n 为索引中指纹数量

### 3.3 分块去重模式（Chunked Mode）

**适用场景**：
- 大文件去重（如日志文件、备份数据）
- 部分内容重复的大块数据
- 需要细粒度去重的场景

**工作原理**：
1. 将大块数据按照配置的分块策略切分为多个小块
2. 对每个块独立计算哈希指纹
3. 任一数据块的指纹命中即认为数据包含重复内容

**分块策略**：
- **固定大小分块**：按照固定字节大小切分，实现简单但对数据偏移敏感
- **内容边界分块**：基于指定的内容边界字节（如换行符）切分，更智能

**特点**：
- 支持检测部分重复的数据
- 分块策略可配置
- 适用于大块数据处理
- 任一数据块重复即判定整体重复

## 4. 持久化机制

### 4.1 持久化格式

持久化文件采用二进制格式，包含以下部分：

```
+-----------------+
|   Header (16B)  |  Magic(4B) + Version(2B) + Reserved(2B) + Count(8B)
+-----------------+
|   Entry 1       |  Type(1B) + Len(4B) + Fingerprint(N) + Timestamp(8B)
+-----------------+
|   Entry 2       |  ...
+-----------------+
|   ...           |  ...
+-----------------+
|   Checksum      |  Type(1B) + Len(4B) + SHA256(64) + Timestamp(8B)
+-----------------+
```

### 4.2 完整性校验

- 持久化文件包含 SHA256 校验和
- 加载时自动校验文件完整性
- 校验失败返回明确的错误信息
- 支持手动调用 `Verify` 方法验证文件

### 4.3 增量写入

- `Append` 方法支持增量添加新指纹
- 内部实现为读取-修改-重写的原子操作
- 每次写入都会更新校验和
- 支持自动持久化（配置 `AutoPersist` 和 `AutoPersistCount`）

## 5. 使用示例

### 5.1 精确去重示例

```go
package main

import (
    "fmt"
    "solocoder-go/internal/datadedup"
)

func main() {
    // 使用默认配置创建精确去重引擎
    cfg := datadedup.DefaultConfig().WithMode(datadedup.DedupModeExact)
    engine, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    // 添加数据
    data1 := []byte("hello world")
    result, err := engine.CheckAndAdd(data1)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Is duplicate: %v\n", result.IsDuplicate) // false

    // 再次添加相同数据
    result, err = engine.CheckAndAdd(data1)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Is duplicate: %v\n", result.IsDuplicate) // true

    fmt.Printf("Total unique items: %d\n", engine.Count()) // 1
}
```

### 5.2 模糊去重示例

```go
package main

import (
    "fmt"
    "solocoder-go/internal/datadedup"
)

func main() {
    // 创建模糊去重引擎，相似度阈值 0.8
    cfg := datadedup.DefaultConfig().
        WithMode(datadedup.DedupModeFuzzy).
        WithSimilarityThreshold(0.8)
    
    engine, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    // 添加原始文本
    text1 := []byte("The quick brown fox jumps over the lazy dog")
    engine.Add(text1)

    // 检测相似文本
    text2 := []byte("The quick brown fox jumps over the lazy dogs")
    result, err := engine.Check(text2)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Is duplicate: %v\n", result.IsDuplicate)   // true
    fmt.Printf("Similarity: %.2f\n", result.Similarity)  // ~0.95
}
```

### 5.3 分块去重示例

```go
package main

import (
    "fmt"
    "solocoder-go/internal/datadedup"
)

func main() {
    // 创建分块去重引擎，固定大小 100 字节
    cfg := datadedup.DefaultConfig().
        WithMode(datadedup.DedupModeChunked).
        WithChunkSize(100)
    
    engine, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    // 添加包含重复块的大数据
    data1 := make([]byte, 250)
    for i := range data1 {
        data1[i] = byte(i % 256)
    }
    engine.Add(data1)

    // 第二个数据包含相同的前 100 字节
    data2 := make([]byte, 200)
    copy(data2[:100], data1[:100])
    for i := 100; i < 200; i++ {
        data2[i] = byte(255 - i%256)
    }

    result, err := engine.Check(data2)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Is duplicate: %v\n", result.IsDuplicate)        // true
    fmt.Printf("Matched chunks: %d\n", len(result.MatchedChunks)) // 1
}
```

### 5.4 持久化示例

```go
package main

import (
    "fmt"
    "solocoder-go/internal/datadedup"
)

func main() {
    // 配置自动持久化
    cfg := datadedup.DefaultConfig().
        WithMode(datadedup.DedupModeExact).
        WithPersistPath("/data/dedup_index.idx")
    
    // 创建引擎时自动加载已有索引
    engine, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }

    // 添加数据，达到阈值时自动持久化
    for i := 0; i < 1500; i++ {
        data := []byte(fmt.Sprintf("data_%d", i))
        engine.Add(data) // 每 1000 条自动持久化
    }

    // 手动保存
    err = engine.Save("/data/dedup_index.idx")
    if err != nil {
        panic(err)
    }

    engine.Close() // 关闭时也会自动保存

    // 重新创建引擎，数据会自动恢复
    engine2, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }
    defer engine2.Close()

    fmt.Printf("Loaded items: %d\n", engine2.Count()) // 1500
}
```

### 5.5 使用自定义组件

```go
package main

import (
    "solocoder-go/internal/datadedup"
)

// 自定义相似度计算器
type CustomSimilarityCalc struct{}

func (c *CustomSimilarityCalc) Calculate(data []byte) (datadedup.Fingerprint, error) {
    // 自定义实现...
    return datadedup.Fingerprint(string(data)), nil
}

func (c *CustomSimilarityCalc) Similarity(fp1, fp2 datadedup.Fingerprint) (float64, error) {
    // 自定义实现...
    return 0.5, nil
}

func (c *CustomSimilarityCalc) Algorithm() datadedup.SimilarityAlgorithm {
    return "custom"
}

func main() {
    cfg := datadedup.DefaultConfig().
        WithMode(datadedup.DedupModeFuzzy)
    cfg.SimilarityCalc = &CustomSimilarityCalc{}

    engine, err := datadedup.NewDedupEngine(cfg)
    if err != nil {
        panic(err)
    }
    defer engine.Close()

    // 使用自定义相似度计算...
}
```

## 6. 配置说明

### 6.1 默认配置

```go
func DefaultConfig() Config {
    return Config{
        Mode:                DedupModeExact,
        HashAlgorithm:       HashAlgorithmSHA256,
        SimilarityAlgo:      SimilarityAlgorithmSimHash,
        SimilarityThreshold: 0.85,
        ChunkStrategy:       ChunkStrategyFixedSize,
        ChunkSize:           4096,
        ContentBoundary:     '\n',
        MinChunkSize:        1024,
        MaxChunkSize:        16384,
        AutoPersist:         false,
        AutoPersistCount:    1000,
    }
}
```

### 6.2 配置验证

配置在创建引擎时会自动验证，以下情况会返回错误：
- 无效的去重模式
- 相似度阈值超出 [0, 1] 范围
- 分块大小为零或负数
- 内容分块模式下最小分块大于最大分块

## 7. 性能特性

- **线程安全**：所有公共方法均支持并发调用
- **内存索引**：使用 map 存储，查找性能 O(1)
- **读写锁**：使用 RWMutex 优化读多写少场景
- **零拷贝**：尽量避免不必要的数据拷贝

## 8. 错误处理

模块定义了丰富的错误类型，便于上层应用进行错误处理：

| 错误类型 | 说明 |
|---------|------|
| `ErrEmptyData` | 输入数据为空 |
| `ErrInvalidConfig` | 配置无效 |
| `ErrEngineClosed` | 引擎已关闭 |
| `ErrInvalidThreshold` | 相似度阈值无效 |
| `ErrInvalidChunkSize` | 分块大小无效 |
| `ErrPersistFileNotExist` | 持久化文件不存在 |
| `ErrPersistCorrupted` | 持久化文件损坏 |
| `ErrChecksumMismatch` | 校验和不匹配 |

## 9. 注意事项

1. **内存消耗**：索引存储在内存中，大数据量时需要考虑内存占用
2. **持久化性能**：自动持久化会触发磁盘写入，高并发场景需要合理配置阈值
3. **模糊去重性能**：相似度比对是 O(n) 复杂度，索引过大时性能会下降
4. **哈希算法选择**：SHA256 安全性最高，MD5 性能最快，根据场景选择
5. **分块策略选择**：固定大小性能好，内容边界更智能，根据数据特性选择

## 10. 代码位置

- 核心代码：`internal/datadedup/`
  - `types.go` - 类型定义和接口
  - `errors.go` - 错误定义
  - `config.go` - 配置管理
  - `hash_provider.go` - 哈希提供者实现
  - `exact_dedup.go` - 精确去重引擎
  - `simhash.go` - SimHash 算法和模糊去重引擎
  - `chunker.go` - 分块策略和分块去重引擎
  - `persistence.go` - 持久化实现
  - `dedup_engine.go` - 统一引擎入口
  - `datadedup_test.go` - 单元测试
- 文档：`docs/datadedup.md`
