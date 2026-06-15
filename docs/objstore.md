# ObjStore 版本化对象存储模块

## 1. 模块概述

ObjStore 是一个支持版本控制的内存对象存储模块，专为需要保留历史版本、支持数据回滚的场景设计。模块提供完整的对象读写、版本管理、回滚和过期清理功能，通过读写锁保证并发安全。

**包路径**: `internal/objstore`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 对象写入 | 向存储中写入对象数据，每次写入自动生成递增版本号 |
| 对象读取 | 支持按最新版本读取和按指定历史版本读取 |
| 版本列表 | 查询某个键的所有版本信息，包含版本号和创建时间 |
| 版本回滚 | 将指定键的数据回滚到任意历史版本（创建新版本） |
| 过期清理 | 配置最大保留版本数，超过阈值时自动清理最旧版本 |
| 清理策略 | 可配置清理触发间隔和每次清理的批次大小 |
| 对象删除 | 删除整个键及其所有历史版本 |

## 3. 核心结构体与职责

### 3.1 ObjectStore

主存储结构体，对外提供所有操作接口。

```go
type ObjectStore struct {
    mu             sync.RWMutex
    objects        map[string][]*ObjectVersion
    config         Config
    opsSinceCleanup int
}
```

**职责**:
- 管理所有键的版本化对象数据
- 通过 `sync.RWMutex` 保护并发访问
- 维护版本计数器和清理触发计数
- 协调整个存储的版本管理和清理策略

### 3.2 ObjectVersion

单个对象版本的完整数据。

```go
type ObjectVersion struct {
    Version   uint64
    Data      []byte
    CreatedAt time.Time
}
```

**职责**:
- 存储一个特定版本的对象数据
- `Version`: 递增的版本号，从 1 开始
- `Data`: 对象数据的深拷贝
- `CreatedAt`: 版本创建时间戳

### 3.3 VersionInfo

版本元信息，用于列表查询。

```go
type VersionInfo struct {
    Version   uint64
    CreatedAt time.Time
}
```

**职责**:
- 提供版本的元数据信息，不含实际数据
- 用于 `ListVersions` 方法，轻量级查询版本列表
- 按版本号升序排列

### 3.4 Config

存储配置结构体。

```go
type Config struct {
    MaxVersions      int
    CleanupBatchSize int
    CleanupInterval  int
}
```

**配置项说明**:

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| MaxVersions | 10 | 每个键最大保留的版本数 |
| CleanupBatchSize | 1 | 每次清理操作移除的旧版本数量 |
| CleanupInterval | 1 | 每隔多少次写入操作触发一次清理 |

## 4. 对象生命周期

### 4.1 完整生命周期流程

```
对象写入 (Put / Rollback)
        │
        ▼
版本号递增 + 创建新版本对象
        │
        ▼
   添加到版本列表尾部
        │
        ▼
 opsSinceCleanup++
        │
        ▼
 达到清理间隔? ──否──► 结束
        │
        是
        ▼
   触发清理检查
        │
        ▼
 超过 MaxVersions? ──否──► 重置计数器，结束
        │
        是
        ▼
 按 CleanupBatchSize 移除最旧版本
        │
        ▼
 重置 opsSinceCleanup = 0
        │
        ▼
       结束
```

### 4.2 版本号生成规则

- 每个键独立维护版本号序列
- 首次写入生成版本号 1
- 后续每次写入（Put 或 Rollback）版本号递增 1
- 版本号一旦分配永不复用，即使旧版本被清理
- 版本号单调递增，保证历史可追溯

### 4.3 版本清理策略

清理机制支持灵活配置，平衡内存占用和清理开销：

**触发时机** (`CleanupInterval`):
- 值为 1：每次写入后立即检查清理（最及时）
- 值为 N：每 N 次写入后检查一次（减少清理开销）
- 适用于写入频繁、对清理实时性要求不高的场景

**清理批次** (`CleanupBatchSize`):
- 每次清理最多移除的旧版本数量
- 防止单次清理操作耗时过长
- 当超出版本数少于批次大小时，仅清理超出部分

### 4.4 回滚机制

回滚操作**不修改历史版本**，而是创建一个新版本：

1. 查找目标历史版本的数据
2. 创建一个新版本号，复制目标版本的数据
3. 将新版本追加到版本列表末尾
4. 触发清理检查（与普通写入相同）

**设计优势**:
- 历史版本保持完整，可随时查阅
- 回滚操作本身也有版本记录，可追踪
- 与清理策略无缝集成

## 5. 并发安全设计

### 5.1 锁策略

模块使用单个全局 `sync.RWMutex` 保证并发安全：

| 操作 | 锁类型 | 说明 |
|------|--------|------|
| Put | 写锁 | 修改版本列表 |
| Get | 读锁 | 读取最新版本 |
| GetVersion | 读锁 | 读取指定版本 |
| ListVersions | 读锁 | 读取版本列表 |
| Rollback | 写锁 | 添加新版本 |
| Delete | 写锁 | 删除键及其版本 |
| CleanupAll | 写锁 | 执行清理操作 |
| Count | 读锁 | 统计键数量 |
| VersionCount | 读锁 | 统计版本数量 |

### 5.2 数据安全性

所有返回给调用者的数据都是**深拷贝**：
- Put 时复制输入数据，防止外部修改影响存储
- Get / GetVersion 时返回复制数据，防止外部修改存储内容
- ListVersions 返回 VersionInfo 结构体副本，不含实际数据引用

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/objstore"
)

func main() {
    store := objstore.NewObjectStore()

    ver, err := store.Put("user:1:profile", []byte(`{"name":"Alice","age":30}`))
    if err != nil {
        panic(err)
    }
    fmt.Println("First version:", ver) // 1

    ver, err = store.Put("user:1:profile", []byte(`{"name":"Alice","age":31}`))
    fmt.Println("Second version:", ver) // 2

    data, version, err := store.Get("user:1:profile")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Latest version %d: %s\n", version, data)
}
```

### 6.2 自定义配置

```go
cfg := objstore.Config{
    MaxVersions:      50,    // 每个键保留 50 个版本
    CleanupBatchSize: 5,     // 每次清理 5 个旧版本
    CleanupInterval:  10,    // 每 10 次写入触发一次清理
}
store := objstore.NewObjectStoreWithConfig(cfg)
```

### 6.3 版本管理

```go
store.Put("doc:readme", []byte("version 1"))
store.Put("doc:readme", []byte("version 2"))
store.Put("doc:readme", []byte("version 3"))

versions, err := store.ListVersions("doc:readme")
if err != nil {
    panic(err)
}
for _, v := range versions {
    fmt.Printf("Version %d, created at %v\n", v.Version, v.CreatedAt)
}

oldData, err := store.GetVersion("doc:readme", 1)
if err != nil {
    panic(err)
}
fmt.Println("Version 1 content:", string(oldData))
```

### 6.4 版本回滚

```go
store.Put("config:api", []byte("timeout=30s"))
store.Put("config:api", []byte("timeout=60s"))

newVer, err := store.Rollback("config:api", 1)
if err != nil {
    panic(err)
}
fmt.Printf("Rolled back to version 1, created new version %d\n", newVer)

data, _, _ := store.Get("config:api")
fmt.Println("Current data:", string(data)) // timeout=30s

v2Data, _ := store.GetVersion("config:api", 2)
fmt.Println("Version 2 still exists:", string(v2Data)) // timeout=60s
```

### 6.5 过期版本清理

```go
cfg := objstore.Config{
    MaxVersions:      3,
    CleanupBatchSize: 1,
    CleanupInterval:  1,
}
store := objstore.NewObjectStoreWithConfig(cfg)

for i := 1; i <= 10; i++ {
    store.Put("metric:cpu", []byte(fmt.Sprintf("value_%d", i)))
}

count, _ := store.VersionCount("metric:cpu")
fmt.Printf("Kept %d versions (max %d)\n", count, cfg.MaxVersions)

versions, _ := store.ListVersions("metric:cpu")
fmt.Printf("Oldest kept version: %d\n", versions[0].Version) // 8
fmt.Printf("Newest version: %d\n", versions[len(versions)-1].Version) // 10
```

### 6.6 手动清理

```go
cfg := objstore.Config{
    MaxVersions:      5,
    CleanupBatchSize: 2,
    CleanupInterval:  100, // 不自动触发清理
}
store := objstore.NewObjectStoreWithConfig(cfg)

for i := 0; i < 20; i++ {
    store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
}

count, _ := store.VersionCount("key1")
fmt.Println("Before manual cleanup:", count) // 20

cleaned := store.CleanupAll()
fmt.Println("Cleaned versions:", cleaned) // 2 (按批次清理)

count, _ = store.VersionCount("key1")
fmt.Println("After cleanup:", count) // 18
```

### 6.7 并发使用

```go
var wg sync.WaitGroup
store := objstore.NewObjectStore()

for g := 0; g < 10; g++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        for i := 0; i < 100; i++ {
            key := fmt.Sprintf("worker:%d", id%5)
            data := []byte(fmt.Sprintf("data_%d", i))
            store.Put(key, data)
        }
    }(g)
}

wg.Wait()
fmt.Println("Total keys:", store.Count())
```

## 7. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | Get、GetVersion、ListVersions、Rollback、VersionCount 时键不存在 |
| `ErrVersionNotFound` | 版本不存在 | GetVersion、Rollback 时指定版本号不存在 |
| `ErrEmptyKey` | 键为空 | 所有接受 key 参数的方法传入空字符串 |
| `ErrNilData` | 数据为 nil | Put 时传入 nil 数据 |
| `ErrInvalidMaxVersion` | 最大版本数无效 | 预留错误，当前使用默认值替代 |
| `ErrInvalidBatchSize` | 清理批次大小无效 | 预留错误，当前使用默认值替代 |

## 8. 性能特征

### 8.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Put | O(n) | 版本号计算 O(1)，清理操作涉及切片截断 |
| Get | O(1) | 直接访问最新版本 |
| GetVersion | O(n) | 线性查找指定版本 |
| ListVersions | O(n) | 复制版本元信息列表 |
| Rollback | O(n) | 查找目标版本 + 创建新版本 |
| Delete | O(1) | Map 删除操作 |
| CleanupAll | O(k*b) | k 为键数，b 为批次大小 |

### 8.2 并发性能

- **读-读并发**: 完全并行，无阻塞
- **读-写并发**: 互斥，写操作阻塞所有读操作
- **写-写并发**: 串行化执行
- **适用场景**: 读多写少、版本数量可控的场景

### 8.3 内存占用

- 每个版本包含数据拷贝，版本数越多内存占用越大
- 可通过 `MaxVersions` 限制内存使用
- 大数据量对象建议结合外部存储使用

## 9. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **单全局锁**: 大并发写入场景下可能成为瓶颈，可考虑分段锁优化
3. **版本查找**: GetVersion 为线性查找，版本数量大时可能影响性能
4. **深拷贝开销**: 所有数据进出都进行深拷贝，大对象有额外开销
5. **清理粒度**: 清理按全局操作计数触发，不是按每个键独立计数
6. **回滚不可逆**: 回滚操作创建新版本，旧版本仍保留（除非被清理）
7. **空数据支持**: 支持写入空字节切片 (`[]byte{}`)，但不支持 nil
