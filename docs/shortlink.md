# 短链接生成器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [三种短码生成策略](#4-三种短码生成策略)
5. [自定义短码机制](#5-自定义短码机制)
6. [访问统计计数](#6-访问统计计数)
7. [冲突重试机制](#7-冲突重试机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全设计](#10-并发安全设计)

---

## 1. 模块概述

短链接生成器（ShortLink Generator）是一个用于将长 URL 转换为短 URL 的核心模块，提供四种短码生成策略、自定义短码支持、访问统计计数、冲突自动重试等完整功能。模块采用内存存储，支持自增 ID Base62 编码、URL 哈希摘要、随机字符串三种自动生成策略，以及自定义短码功能。

**包路径**: `internal/shortlink`

**设计目标**:
- 多种短码生成策略，适应不同业务场景
- 可配置的哈希算法和字符集
- 精确的访问次数统计（并发安全）
- 自定义短码的格式校验和唯一性检查
- 自动冲突检测与重试机制
- 完整的生命周期管理

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 自增 ID 策略 | 基于自增整数通过 Base62 编码生成短码，保证唯一且长度随 ID 增长 |
| 哈希策略 | 基于原始 URL 计算哈希摘要并截取指定长度作为短码，支持 MD5/SHA1/SHA256 |
| 随机字符串策略 | 基于可配置字符集和长度生成随机短码，自动检查唯一性 |
| 自定义短码 | 允许调用方指定自定义短码，校验格式合法性和唯一性 |
| 访问统计 | 每次访问自动递增计数，支持单个查询和汇总统计 |
| 冲突重试 | 哈希和随机策略在冲突时自动重试，超过最大次数返回错误 |
| 短码解析 | 通过短码获取原始 URL，同时自动累加访问计数 |
| 元信息查询 | 查询短链接的完整元信息（短码、原 URL、访问次数、创建时间） |
| 列表查询 | 列出所有短链接，按创建时间倒序排列 |
| 删除短链接 | 删除指定短码及其关联数据 |

---

## 3. 核心结构体与职责

### 3.1 Manager

短链接管理器主结构体，对外提供所有操作接口。

```go
type Manager struct {
    mu            sync.RWMutex
    links         map[string]*ShortLink
    autoIncrement atomic.Int64
    cfg           Config
}
```

**职责**:
- 管理所有短链接的存储和索引
- 维护自增 ID 计数器
- 通过 RWMutex 保证并发访问安全
- 持有配置信息，提供策略相关参数

**主要方法**:
- `NewManager()` - 创建使用默认配置的管理器
- `NewManagerWithConfig(cfg)` - 使用自定义配置创建管理器
- `Create(opts)` - 创建短链接
- `GetOriginalURL(shortCode)` - 获取原始 URL（自动计数+1）
- `GetMeta(shortCode)` - 查询短链接元信息
- `GetVisitCount(shortCode)` - 查询单个短码访问次数
- `GetTotalVisitCount()` - 查询全部短码总访问次数
- `ListAll()` - 列出所有短链接
- `Delete(shortCode)` - 删除短链接
- `Count()` - 获取短链接总数

### 3.2 ShortLink

单个短链接的内部表示。

```go
type ShortLink struct {
    ShortCode   string
    OriginalURL string
    VisitCount  atomic.Int64
    CreatedAt   time.Time
}
```

**职责**:
- 存储短码和原始 URL 的映射关系
- 通过 `atomic.Int64` 保证访问计数并发安全
- 记录创建时间

### 3.3 ShortLinkMeta

短链接元信息的只读视图。

```go
type ShortLinkMeta struct {
    ShortCode   string
    OriginalURL string
    VisitCount  int64
    CreatedAt   time.Time
}
```

**职责**:
- 提供短链接状态的只读快照
- 不包含任何可写引用，防止外部修改内部状态

### 3.4 CreateOptions

创建短链接时的配置选项。

```go
type CreateOptions struct {
    OriginalURL string
    CustomCode  string
    Strategy    ShortCodeStrategy
}
```

**字段说明**:
- `OriginalURL`: 需要缩短的原始 URL（必填）
- `CustomCode`: 自定义短码，设置后自动使用自定义策略
- `Strategy`: 短码生成策略，默认为自增 ID 策略

### 3.5 Config

管理器全局配置。

```go
type Config struct {
    HashConfig      HashStrategyConfig
    RandomConfig    RandomStrategyConfig
    AutoIncrement   AutoIncrementConfig
    DefaultStrategy ShortCodeStrategy
}
```

### 3.6 HashStrategyConfig

哈希策略配置。

```go
type HashStrategyConfig struct {
    Algorithm  HashAlgorithm
    Length     int
    MaxRetries int
}
```

**字段说明**:
- `Algorithm`: 哈希算法，支持 MD5/SHA1/SHA256
- `Length`: 截取的哈希长度（1-64）
- `MaxRetries`: 冲突时最大重试次数

### 3.7 RandomStrategyConfig

随机策略配置。

```go
type RandomStrategyConfig struct {
    Length     int
    Charset    string
    MaxRetries int
}
```

**字段说明**:
- `Length`: 随机字符串长度
- `Charset`: 字符集（如仅数字、仅字母、Base62 等）
- `MaxRetries`: 冲突时最大重试次数

### 3.8 AutoIncrementConfig

自增 ID 策略配置。

```go
type AutoIncrementConfig struct {
    StartID int64
}
```

**字段说明**:
- `StartID`: 自增 ID 起始值，默认为 1

### 3.9 ShortCodeStrategy

短码生成策略枚举。

```go
type ShortCodeStrategy string

const (
    StrategyAutoIncrement ShortCodeStrategy = "auto_increment"
    StrategyHash          ShortCodeStrategy = "hash"
    StrategyRandom        ShortCodeStrategy = "random"
    StrategyCustom        ShortCodeStrategy = "custom"
)
```

### 3.10 HashAlgorithm

哈希算法枚举。

```go
type HashAlgorithm string

const (
    HashMD5    HashAlgorithm = "md5"
    HashSHA1   HashAlgorithm = "sha1"
    HashSHA256 HashAlgorithm = "sha256"
)
```

---

## 4. 三种短码生成策略

### 4.1 策略选型对比

| 维度 | 自增 ID 策略 | 哈希策略 | 随机字符串策略 |
|------|-------------|---------|--------------|
| **唯一性** | 100% 保证 | 冲突概率极低 | 冲突概率极低 |
| **可预测性** | 可预测（递增） | 不可预测 | 不可预测 |
| **短码长度** | 随 ID 增长逐步增加 | 固定长度 | 固定长度 |
| **相同 URL** | 生成不同短码 | 生成相同短码 | 生成不同短码 |
| **冲突处理** | 无需处理 | 自动重试 | 自动重试 |
| **适用场景** | 内部系统、有序场景 | URL 去重、固定长度场景 | 公开分享、防预测场景 |

### 4.2 自增 ID 策略（StrategyAutoIncrement）

**工作原理**:
1. 使用 `atomic.Int64` 维护一个原子自增计数器
2. 每次创建时原子 +1 获取唯一整数 ID
3. 通过 Base62 编码将整数转换为紧凑字符串

**Base62 字符集**: `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`

**编码示例**:
```
ID=1       → "1"
ID=10      → "A"
ID=62      → "10"
ID=999     → "G7"
ID=1000000 → "4c92"
```

**优点**:
- 100% 保证唯一性，不会冲突
- 短码紧凑，空间利用率最高
- 生成速度极快

**缺点**:
- 短码可预测，不适合防爬取场景

### 4.3 哈希策略（StrategyHash）

**工作原理**:
1. 对原始 URL 计算指定哈希算法（MD5/SHA1/SHA256）
2. 计算哈希摘要并转为十六进制字符串
3. 从哈希值中截取指定长度的子串作为短码
4. 检查是否存在，存在则自动在原始 URL 附加后缀重新计算
5. 循环重试，最多 MaxRetries 次

**冲突处理流程**:
```
GenerateHash(originalURL)
    │
    ├─ 1. 计算哈希(URL)
    │   └─ 短码唯一 → 返回短码
    │
    ├─ 冲突 → 重试: URL + 重试次数 + 时间戳
    │
    └─ 超过 MaxRetries → 返回 ErrGenerateFailed
```

**优点**:
- 固定长度短码
- 相同 URL 生成相同短码（可用于去重）
- 不可预测

**缺点**:
- 存在理论冲突概率

### 4.4 随机字符串策略（StrategyRandom）

**工作原理**:
1. 使用 `crypto/rand` 生成安全随机字节
2. 按字符集映射为指定长度的随机字符串
3. 检查唯一性，冲突则重新生成
4. 循环重试，最多 MaxRetries 次

**可配置字符集示例**:
- Base62: `0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz`
- 仅数字: `0123456789`
- 仅小写字母: `abcdefghijklmnopqrstuvwxyz`

**优点**:
- 高度不可预测
- 固定长度
- 灵活配置字符集和长度

**缺点**:
- 存在理论冲突概率

---

## 5. 自定义短码机制

### 5.1 格式校验

自定义短码必须符合以下规则（正则: `^[A-Za-z0-9_-]{1,32}$`）：
- 允许字符: 大小写字母、数字、下划线 `_`、连字符 `-`
- 长度: 1-32 个字符
- 不允许: 空格、斜杠、问号、点号等特殊字符

### 5.2 唯一性检查

创建时:
1. 先进行格式校验，格式非法返回 `ErrInvalidCustomShortCode`
2. 检查唯一性，已占用返回 `ErrShortCodeExists`
3. 通过后创建短链接

---

## 6. 访问统计计数

### 6.1 计数原理

- `VisitCount` 使用 `atomic.Int64` 存储，保证原子读取
- `GetOriginalURL()` 每次调用时原子 +1
- 高并发场景下计数精确

### 6.2 统计接口

```go
// 查询单个短码访问次数
count, _ := manager.GetVisitCount(shortCode)

// 查询全部短码总访问次数
total := manager.GetTotalVisitCount()
```

---

## 7. 冲突重试机制

### 7.1 适用策略

哈希策略和随机字符串策略在生成短码时，会自动进行冲突检测和重试。

### 7.2 重试流程

```
生成短码
    │
    ├─ 生成候选短码
    │   └─ 检查唯一性
    │       ├─ 唯一 → 返回成功
    │       └─ 冲突 → 重试
    │
    └─ 重试次数 > MaxRetries
        └─ 返回 ErrGenerateFailed
```

### 7.3 默认配置

- 重试次数: 默认 10 次

---

## 8. 使用示例

### 8.1 基本使用：创建和访问短链接

```go
package main

import (
    "fmt"
    "solocoder-go/internal/shortlink"
)

func main() {
    manager := shortlink.NewManager()

    // 使用默认策略（自增 ID）
    meta1, err := manager.Create(shortlink.CreateOptions{
        OriginalURL: "https://example.com/article/123",
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("短码: %s\n", meta1.ShortCode)

    // 使用哈希策略
    meta2, err := manager.Create(shortlink.CreateOptions{
        OriginalURL: "https://blog.example.com/posts/hello",
        Strategy:    shortlink.StrategyHash,
    })

    // 使用随机策略
    meta3, err := manager.Create(shortlink.CreateOptions{
        OriginalURL: "https://shop.example.com/products/42",
        Strategy:    shortlink.StrategyRandom,
    })

    // 使用自定义短码
    meta4, err := manager.Create(shortlink.CreateOptions{
        OriginalURL: "https://special.example.com",
        CustomCode:  "my-special-link",
    })

    // 访问短链接（自动计数）
    originalURL, err := manager.GetOriginalURL(meta1.ShortCode)
    if err != nil {
        panic(err)
    }
    fmt.Printf("原始 URL: %s\n", originalURL)
}
```

### 8.2 自定义配置

```go
cfg := shortlink.Config{
    HashConfig: shortlink.HashStrategyConfig{
        Algorithm:  shortlink.HashSHA256,
        Length:     10,
        MaxRetries: 20,
    },
    RandomConfig: shortlink.RandomStrategyConfig{
        Length:     6,
        Charset:    "0123456789abcdef",
        MaxRetries: 15,
    },
    AutoIncrement: shortlink.AutoIncrementConfig{
        StartID: 1000,
    },
    DefaultStrategy: shortlink.StrategyAutoIncrement,
}

manager := shortlink.NewManagerWithConfig(cfg)
```

### 8.3 访问统计

```go
func printStats(manager *shortlink.Manager) {
    // 查询单个短码访问次数
    count, _ := manager.GetVisitCount("abc123")
    fmt.Printf("访问次数: %d\n", count)

    // 查询总访问次数
    total := manager.GetTotalVisitCount()
    fmt.Printf("总访问次数: %d\n", total)

    // 列出所有短链接
    all := manager.ListAll()
    for _, link := range all {
        fmt.Printf("%s -> %s (%d 次访问)\n",
            link.ShortCode,
            link.OriginalURL,
            link.VisitCount,
        )
    }
}
```

### 8.4 自定义短码

```go
meta, err := manager.Create(shortlink.CreateOptions{
    OriginalURL: "https://example.com/promo",
    CustomCode:  "summer-sale-2024",
})

if errors.Is(err, shortlink.ErrShortCodeExists) {
    fmt.Println("短码已被占用，请换一个")
} else if errors.Is(err, shortlink.ErrInvalidCustomShortCode) {
    fmt.Println("短码格式不合法，只能包含字母、数字、下划线和连字符")
}
```

### 8.5 删除短链接

```go
err := manager.Delete("abc123")
if errors.Is(err, shortlink.ErrShortCodeNotFound) {
    fmt.Println("短码不存在")
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrShortCodeNotFound` | 短码不存在 | 查询/删除/访问时找不到对应短码 |
| `ErrShortCodeExists` | 短码已存在 | 自定义短码冲突 |
| `ErrEmptyOriginalURL` | 原始 URL 为空 | 创建时未提供原始 URL |
| `ErrEmptyShortCode` | 短码为空 | 查询/删除时短码为空 |
| `ErrInvalidCustomShortCode` | 自定义短码格式非法 | 自定义短码包含非法字符 |
| `ErrGenerateFailed` | 生成唯一短码失败 | 超过最大重试次数仍冲突 |
| `ErrMaxRetriesZeroOrNegative` | 重试次数配置非法 | MaxRetries <= 0 |
| `ErrHashLengthInvalid` | 哈希长度配置非法 | Length 不在 1-64 范围 |
| `ErrRandomLengthInvalid` | 随机长度配置非法 | Length <= 0 |
| `ErrInvalidCharset` | 字符集为空 | 随机策略字符集为空 |
| `ErrUnsupportedHashAlgo` | 不支持的哈希算法 | Algorithm 不是 MD5/SHA1/SHA256 |

---

## 10. 并发安全设计

### 10.1 分层锁设计

| 层次 | 保护对象 | 同步机制 | 说明 |
|------|---------|---------|------|
| Manager 层 | `links` map | `sync.RWMutex` | 多读单写，创建/查询分离 |
| 访问计数 | `VisitCount` | `atomic.Int64` | 无锁原子操作 |
| 自增 ID | `autoIncrement` | `atomic.Int64` | 无锁原子操作 |

### 10.2 无锁热点路径

最频繁的访问操作 `GetOriginalURL` 中：
- **计数递增**: `VisitCount.Add(1)` - 纯原子操作，无锁
- 只有在读取 links map 查找时短暂持有读锁

### 10.3 并发测试覆盖

测试套件包含以下并发场景：
- 100 并发 `Create`：所有创建成功
- 500 并发 `GetOriginalURL`：计数精确到 500
- 混合读/写并发压力测试：无数据竞争、无死锁
