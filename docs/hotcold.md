# HotCold 冷热数据分离检测器模块

## 1. 模块概述

HotCold 是一个冷热数据分离检测与管理模块，专为需要优化存储成本和访问性能的系统设计。模块通过热度评分算法自动识别热数据（频繁访问）和冷数据（极少访问），并在快速存储和低成本存储之间自动迁移数据，实现性能与成本的动态平衡。

**包路径**: `internal/hotcold`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 热度评分 | 基于访问频率和新近度计算综合热度评分，反映数据冷热程度，评分查询保证实时时效性 |
| 热数据提升 | 热度评分超过热阈值时，自动迁移到快速存储（热层） |
| 冷数据降级 | 热度评分连续低于冷阈值达到指定周期后，迁移到低成本存储（冷层） |
| 自适应阈值 | 根据热数据容量占比目标 + 系统整体访问负载双重维度动态调整阈值，避免频繁迁移 |
| 精确错误分类 | 所有错误变量（ErrKeyNotFound、ErrInvalidConfig 等）在对应场景下被正确返回，支持 errors.Is 精确分类 |
| 并发安全 | 内置读写锁，支持高并发场景下的数据访问和迁移 |

## 3. 核心结构体与职责

### 3.1 HotColdManager

冷热数据管理器，是模块的核心入口，对外提供所有操作接口。

```go
type HotColdManager struct {
    mu                sync.RWMutex
    hotStore          map[string]*DataEntry
    coldStore         map[string]*DataEntry
    cfg               Config
    lastAdjustTime    time.Time
    totalAccesses     int64
    accessesLastEpoch int64
}
```

**字段说明**:
- `mu`: 读写互斥锁，保护并发安全
- `hotStore`: 热数据存储（快速存储区）
- `coldStore`: 冷数据存储（低成本存储区）
- `cfg`: 模块配置
- `lastAdjustTime`: 上次自适应阈值调整时间
- `totalAccesses`: 累计总访问次数（Put + Get），用于负载统计
- `accessesLastEpoch`: 上次阈值调整时的访问次数基准，用于计算周期内访问速率

**职责**:
- 管理热存储（hotStore）和冷存储（coldStore）两层数据
- 协调数据在冷热层之间的迁移
- 维护配置参数和自适应阈值调整状态
- 统计系统访问负载，参与自适应阈值决策
- 提供线程安全的数据访问接口

### 3.2 DataEntry

数据条目，记录单个数据项的完整元信息。

```go
type DataEntry struct {
    Key                   string
    Value                 interface{}
    AccessCount           int64
    LastAccessTime        time.Time
    CreatedAt             time.Time
    Tier                  DataTier
    ConsecutiveColdCycles int
    Score                 float64
}
```

**字段说明**:
- `Key`: 数据键，唯一标识
- `Value`: 数据值，支持任意类型（interface{}）
- `AccessCount`: 累计访问次数
- `LastAccessTime`: 最近一次访问时间
- `CreatedAt`: 数据创建时间
- `Tier`: 当前所属层级（TierHot / TierCold）
- `ConsecutiveColdCycles`: 连续低于冷阈值的周期数（用于冷数据降级判定）
- `Score`: 最近一次更新时缓存的热度评分（注意：通过 GetScore/GetEntry 查询时会返回实时计算值，而非此缓存值）

### 3.3 Config

模块配置结构体，用于自定义冷热分离策略。

```go
type Config struct {
    HotThreshold         float64
    ColdThreshold        float64
    DecayHalfLife        time.Duration
    ColdCheckCycles      int
    HotCapacityRatio     float64
    AutoAdjustThresholds bool
    AdjustInterval       time.Duration
    MinHotThreshold      float64
    MaxHotThreshold      float64
    MinColdThreshold     float64
    MaxColdThreshold     float64
}
```

**配置项说明**:

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| `HotThreshold` | 10.0 | 热数据阈值，评分 ≥ 此值则提升为热数据 |
| `ColdThreshold` | 2.0 | 冷数据阈值，评分 < 此值则可能降级为冷数据 |
| `DecayHalfLife` | 1小时 | 热度衰减半衰期，时间越久热度越低 |
| `ColdCheckCycles` | 3 | 冷数据降级所需的连续低评分周期数 |
| `HotCapacityRatio` | 0.3 | 热数据容量占比目标（0 < ratio < 1，用于自适应阈值） |
| `AutoAdjustThresholds` | true | 是否启用自适应阈值调整 |
| `AdjustInterval` | 5分钟 | 自适应阈值调整的最小时间间隔 |
| `MinHotThreshold` | 5.0 | 热阈值下限（自适应时不会低于此值） |
| `MaxHotThreshold` | 50.0 | 热阈值上限（自适应时不会高于此值） |
| `MinColdThreshold` | 0.5 | 冷阈值下限 |
| `MaxColdThreshold` | 10.0 | 冷阈值上限 |

### 3.4 DataTier

数据层级枚举类型。

```go
type DataTier int

const (
    TierCold DataTier = iota  // 冷数据层（低成本存储）
    TierHot                   // 热数据层（快速存储）
)
```

## 4. 构造函数与配置验证

### 4.1 NewHotColdManager

使用默认配置创建管理器。

```go
func NewHotColdManager() *HotColdManager
```

该函数内部调用 `NewHotColdManagerWithConfig(DefaultConfig())`，默认配置始终合法，因此不返回错误。

### 4.2 NewHotColdManagerWithConfig

使用自定义配置创建管理器，返回管理器实例和可能的配置错误。

```go
func NewHotColdManagerWithConfig(cfg Config) (*HotColdManager, error)
```

**错误返回规则**:
- **返回 `ErrInvalidConfig`**（配置明显错误，无法自动修复）:
  - `HotCapacityRatio < 0` 或 `HotCapacityRatio >= 1`（超出合理范围）
  - `MinHotThreshold < 0`，或同时设置了 `MinHotThreshold > 0` 且 `MaxHotThreshold > 0` 但 `MaxHotThreshold <= MinHotThreshold`
  - `MinColdThreshold < 0`，或同时设置了 `MinColdThreshold > 0` 且 `MaxColdThreshold > 0` 但 `MaxColdThreshold <= MinColdThreshold`
  - `HotThreshold < 0` 或 `ColdThreshold < 0`
  - 同时设置了 `HotThreshold > 0` 且 `ColdThreshold > 0` 但 `HotThreshold <= ColdThreshold`
  - `DecayHalfLife < 0`、`ColdCheckCycles < 0`、`AdjustInterval < 0`（负数无意义）
- **自动填充默认值**（零值或未设置）:
  - `HotThreshold == 0` → 默认 10.0
  - `ColdThreshold == 0` → 默认 2.0
  - `DecayHalfLife == 0` → 默认 1 小时
  - 其他零值配置项也会自动填充合理默认

### 4.3 ValidateConfig

独立的配置验证函数，严格校验所有配置项的合法性。

```go
func ValidateConfig(cfg Config) error
```

该函数不执行任何默认值填充，要求传入的 cfg 所有字段都已设置为合法值。任何字段不合法都会返回 `ErrInvalidConfig`。

典型使用场景：在持久化配置前进行预校验。

## 5. 热度评分算法

### 5.1 评分公式

热度评分综合考虑**访问频率**和**访问新近度**两个维度，采用指数衰减模型：

```
score = accessCount * decayFactor * newEntryBoost
```

其中：

- **accessCount**: 累计访问次数
- **decayFactor**: 时间衰减因子，随距最近访问的时间增长而衰减
- **newEntryBoost**: 新数据热度加成（创建时间在一个半衰期内有效）

### 5.2 指数衰减机制

采用指数衰减（Exponential Decay）模型，热度随时间自然衰减：

```
decayFactor = e^(-t / halfLife * ln2)
```

或等价表示为：

```
decayFactor = (1/2)^(t / halfLife)
```

**参数说明**:
- `t`: 距离最近一次访问的时间
- `halfLife`: 衰减半衰期（DecayHalfLife）

**特性**:
- 每经过一个半衰期，热度衰减为原来的 50%
- 刚访问过时，decayFactor = 1.0（无衰减）
- 连续访问可累积热度，时间越近权重越高

### 5.3 新数据热度加成

为避免新创建的数据因访问次数少而被立即判定为冷数据，模块提供新数据热度加成：

```
if timeSinceCreation < DecayHalfLife:
    boost = 1.0 + (1.0 - timeSinceCreation/DecayHalfLife) * 0.5
    score *= boost
```

**特性**:
- 新创建的数据最高可获得 1.5 倍热度加成
- 加成随时间线性衰减，经过一个半衰期后降为 0
- 确保新数据有足够时间被"观察"，避免误判

### 5.4 评分时效性保证

模块通过以下机制保证评分查询的实时性：

| 方法 | 评分行为 |
|------|---------|
| `Put` / `Get` | 操作成功时实时重算评分并更新条目缓存 |
| `GetScore(key)` | **每次调用都基于当前时间实时计算**，返回当前时刻的精确热度评分 |
| `GetEntry(key)` | 返回的 DataEntry 副本中 Score 字段为**当前时刻实时计算值** |
| `CheckAndMigrate` | 遍历所有条目时实时重算评分，并更新缓存 |

> **重要语义**: `GetScore` 和 `GetEntry` 方法名暗示的"当前评分"语义与实际行为完全一致——不会返回过时刻度值。即使内部 `DataEntry.Score` 字段可能是缓存值，对外暴露的接口始终保证时效性。

这意味着：
- 连续两次调用 `GetScore` 且间隔时间足够长时，会观察到评分自然衰减（无需任何写操作触发）
- `GetScore` 使用读锁（RLock），实时计算不修改内部状态，可安全并发调用

## 6. 数据迁移判定逻辑

### 6.1 热数据提升（Promotion）

**触发条件**: 冷数据的热度评分 ≥ HotThreshold

**迁移流程**:
```
冷数据访问 → 更新访问计数和时间 → 重新计算评分
         ↓
    评分 ≥ 热阈值？
         ↓ 是
    从冷存储移除
         ↓
    添加到热存储
         ↓
    标记 Tier = TierHot
         ↓
    重置连续冷周期计数
```

**触发时机**:
- 每次 Get / Put 操作时自动检查
- CheckAndMigrate() 批量检查时

### 6.2 冷数据降级（Demotion）

**触发条件**: 热数据连续 `ColdCheckCycles` 个检查周期的评分都 < ColdThreshold

**迁移流程**:
```
CheckAndMigrate 被调用
         ↓
    遍历所有热数据
         ↓
    重新计算评分
         ↓
    评分 < 冷阈值？
      ↓ 是        ↓ 否
  冷周期计数+1   冷周期计数清零
      ↓
  冷周期 ≥ ColdCheckCycles？
      ↓ 是
  从热存储移除
      ↓
  添加到冷存储
      ↓
  标记 Tier = TierCold
```

**设计意图**:
- 引入"连续周期"机制，避免热度短暂波动导致的频繁迁移
- 只有持续变冷的数据才会被降级，减少"乒乓效应"

### 6.3 迁移时机汇总

| 操作 | 是否检查提升 | 是否检查降级 | 说明 |
|------|-------------|-------------|------|
| Get | ✅（如果在冷层） | ❌ | 访问冷数据时可能触发提升 |
| Put | ✅（如果在冷层） | ❌ | 写入冷数据时可能触发提升 |
| CheckAndMigrate | ✅ | ✅ | 全量检查，双向迁移 |

## 7. 自适应阈值调整

### 7.1 设计背景

固定阈值在流量波动时可能导致问题：
- **流量高峰**: 大量数据被提升为热数据，快速存储容量不足
- **流量低谷**: 热数据过少，快速存储利用率低
- **频繁迁移**: 阈值附近的数据反复升降，产生"乒乓效应"

### 7.2 双维度调整策略

自适应阈值综合考虑**容量维度**和**负载维度**两个因素进行动态调整。

#### 维度一：热数据容量占比（Capacity Ratio）

```
目标: 热数据量 / 总数据量 ≈ HotCapacityRatio (默认 30%)
```

**调整规则**:
- 若热数据占比 > 目标的 110% → 阈值上调（让更少数据成为热数据）
- 若热数据占比 < 目标的 90% → 阈值下调（让更多数据成为热数据）
- 在目标范围内 → 容量维度不贡献调整

**容量因子幅度**: ±10%（乘以 1.1 或 0.9）

#### 维度二：系统访问负载（Access Load）

通过统计最近一个调整周期内的访问速率，感知系统整体负载：

```
accessRate = accessesInEpoch / AdjustInterval(seconds)
expectedBaseRate = totalDataCount * 0.5  // 基准访问速率：每条数据每秒 0.5 次
loadFactor = accessRate / expectedBaseRate
```

**负载调整规则**:
- 高负载（`loadFactor > 2.0`）：访问非常密集 → 阈值额外 ×0.9（降低阈值，让更多数据进入热层以分散压力）
- 低负载（`loadFactor < 0.5`）：访问非常稀疏 → 阈值额外 ×1.1（提高阈值，只有真正热点才保留在热层）
- 正常负载（0.5 ≤ loadFactor ≤ 2.0）：负载维度不贡献调整

#### 综合调整

两个维度的调整因子相乘得到最终调整系数：

```
finalAdjustFactor = capacityFactor * loadFactor
```

只有当 finalAdjustFactor ≠ 1.0 时才执行实际阈值调整。

### 7.3 约束条件

为防止阈值漂移过大，设置上下限约束：

- 热阈值范围: [MinHotThreshold, MaxHotThreshold]
- 冷阈值范围: [MinColdThreshold, MaxColdThreshold]
- 热阈值始终 > 冷阈值（确保存在缓冲区间隔）

### 7.4 调整频率

- 仅在 CheckAndMigrate() 时检查是否需要调整
- 两次调整之间至少间隔 AdjustInterval（默认 5 分钟）
- 每个调整周期结束时，会重置 `accessesLastEpoch` 基准值，以便下一个周期计算负载增量
- 避免频繁调整导致系统不稳定

## 8. 错误处理约定

### 8.1 错误变量一览

模块定义了四个哨兵错误（Sentinel Errors），所有公开方法在对应场景下会精确返回这些错误，调用者可通过 `errors.Is()` 进行精确分类。

| 错误变量 | 含义 | 触发场景与返回方法 |
|----------|------|-------------------|
| `ErrKeyNotFound` | 指定的键不存在 | `Get(key)`、`GetScore(key)`、`GetEntry(key)` 查询不存在的键时 |
| `ErrInvalidConfig` | 配置参数不合法 | `NewHotColdManagerWithConfig(cfg)` 传入明显错误的配置；`ValidateConfig(cfg)` 校验失败 |
| `ErrNilManager` | 对 nil 管理器调用方法 | 所有方法在接收者为 nil 时返回 |
| `ErrEmptyKey` | 传入了空字符串键 | `Put("", v)`、`Get("")`、`GetScore("")`、`GetEntry("")` 等传入空键时 |

### 8.2 错误返回模式

模块方法遵循以下返回模式：

**返回 (value, bool, error) 的方法**（Get、GetScore、GetEntry）:
- 成功 → `(value, true, nil)`
- 键不存在 → `(零值, false, ErrKeyNotFound)`
- 其他错误（空键、nil 管理器） → `(零值, false, ErrXxx)`

**返回 error 的方法**（Put）:
- 成功 → `nil`
- 失败 → 对应 `ErrXxx`

**返回 bool 的方法**（Delete）:
- 为保持 API 简洁，Delete 不返回 error；nil 管理器或空键直接返回 `false`

### 8.3 errors.Is 使用示例

```go
val, ok, err := manager.Get("some_key")
if err != nil {
    switch {
    case errors.Is(err, hotcold.ErrKeyNotFound):
        // 键不存在，执行特定逻辑
    case errors.Is(err, hotcold.ErrEmptyKey):
        // 空键参数错误
    case errors.Is(err, hotcold.ErrNilManager):
        // 管理器未初始化
    default:
        // 其他未预期错误
    }
}
```

## 9. 使用示例

### 9.1 基本使用

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "solocoder-go/internal/hotcold"
)

func main() {
    manager := hotcold.NewHotColdManager()

    if err := manager.Put("user:1001", map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
    }); err != nil {
        log.Fatalf("Put failed: %v", err)
    }

    value, ok, err := manager.Get("user:1001")
    if err != nil {
        if errors.Is(err, hotcold.ErrKeyNotFound) {
            fmt.Println("User not found")
        } else {
            log.Fatalf("Get failed: %v", err)
        }
    } else if ok {
        fmt.Println("User:", value)
    }

    score, ok, err := manager.GetScore("user:1001")
    if err == nil && ok {
        fmt.Printf("Hot score: %.2f\n", score)
    }

    fmt.Printf("Hot data: %d, Cold data: %d, Total: %d\n",
        manager.HotCount(), manager.ColdCount(), manager.TotalCount())
}
```

### 9.2 自定义配置与错误处理

```go
cfg := hotcold.Config{
    HotThreshold:     15.0,
    ColdThreshold:    3.0,
    DecayHalfLife:    2 * time.Hour,
    ColdCheckCycles:  5,
    HotCapacityRatio: 0.2,
}

manager, err := hotcold.NewHotColdManagerWithConfig(cfg)
if err != nil {
    if errors.Is(err, hotcold.ErrInvalidConfig) {
        log.Fatalf("Invalid configuration: %v", err)
    }
    log.Fatalf("Failed to create manager: %v", err)
}
```

### 9.3 使用 ValidateConfig 预校验配置

```go
cfg := hotcold.Config{
    HotCapacityRatio: 1.5, // 错误：必须 < 1
}

if err := hotcold.ValidateConfig(cfg); err != nil {
    if errors.Is(err, hotcold.ErrInvalidConfig) {
        log.Printf("Config validation failed, will use defaults")
        cfg = hotcold.DefaultConfig()
    }
}
```

### 9.4 验证评分实时衰减

```go
cfg := hotcold.DefaultConfig()
cfg.DecayHalfLife = 10 * time.Millisecond
manager, _ := hotcold.NewHotColdManagerWithConfig(cfg)

manager.Put("k", "v")
for i := 0; i < 10; i++ {
    manager.Get("k")
}

s1, _, _ := manager.GetScore("k")
time.Sleep(20 * time.Millisecond)
s2, _, _ := manager.GetScore("k")

// s2 会小于 s1，评分随时间实时衰减，无需写操作触发
fmt.Printf("Score decayed: %.2f -> %.2f\n", s1, s2)
```

### 9.5 定期数据迁移检查

```go
go func() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        migrated := manager.CheckAndMigrate()
        if migrated > 0 {
            log.Printf("Migrated %d entries between hot/cold tiers", migrated)
        }
    }
}()
```

### 9.6 获取数据条目详情

```go
entry, exists, err := manager.GetEntry("user:1001")
if err != nil {
    if errors.Is(err, hotcold.ErrKeyNotFound) {
        fmt.Println("Key does not exist")
    }
    return
}
if exists {
    fmt.Printf("Key: %s\n", entry.Key)
    fmt.Printf("Access count: %d\n", entry.AccessCount)
    fmt.Printf("Last access: %v\n", entry.LastAccessTime)
    fmt.Printf("Tier: %s\n", map[hotcold.DataTier]string{
        hotcold.TierHot:  "HOT",
        hotcold.TierCold: "COLD",
    }[entry.Tier])
    // entry.Score 是实时计算的当前评分
    fmt.Printf("Live score: %.2f\n", entry.Score)
}
```

### 9.7 批量操作

```go
hotKeys := manager.GetAllHotKeys()
fmt.Printf("Hot keys: %v\n", hotKeys)

coldKeys := manager.GetAllColdKeys()
fmt.Printf("Cold keys: %v\n", coldKeys)

deleted := manager.Delete("old_key")
fmt.Println("Deleted:", deleted)
```

### 9.8 禁用自适应阈值

```go
cfg := hotcold.DefaultConfig()
cfg.AutoAdjustThresholds = false
cfg.HotThreshold = 20.0
cfg.ColdThreshold = 5.0

manager, _ := hotcold.NewHotColdManagerWithConfig(cfg)
// 此时阈值固定为 20.0 和 5.0，不会随负载或容量变化
```

## 10. 并发安全

模块使用 `sync.RWMutex` 保证并发安全：

| 操作 | 锁类型 | 说明 |
|------|--------|------|
| Put | 写锁 | 修改数据需要独占访问 |
| Get | 写锁 | Get 会更新访问计数和评分缓存，需要写锁 |
| Delete | 写锁 | 删除数据需要独占访问 |
| GetScore | 读锁 | **实时计算**当前评分，不修改内部状态，可高并发 |
| GetEntry | 读锁 | 返回副本，其中 Score 为**实时计算**值，可高并发 |
| HotCount / ColdCount | 读锁 | 只读计数 |
| GetAllHotKeys / GetAllColdKeys | 读锁 | 返回键列表副本 |
| CheckAndMigrate | 写锁 | 迁移操作 + 可能的阈值调整需要独占访问 |

**注意**: Get 操作使用写锁而非读锁，因为它会更新访问元数据。这是设计权衡：
- 优点：热度统计精确，迁移决策及时
- 缺点：读操作也会竞争写锁，高并发读场景下有一定性能影响

**GetScore / GetEntry 的读锁优势**：这两个查询接口使用读锁，即使实时计算评分也不修改内部状态。在"读多写少"的监控场景（如定期展示评分面板）下，可安全地大量并发调用而不阻塞写操作。

## 11. 性能特征

### 11.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Put | O(1) | 哈希表查找 + 可能的迁移 |
| Get | O(1) | 哈希表查找 + 评分重算 + 可能的迁移 |
| Delete | O(1) | 哈希表删除 |
| GetScore | O(1) | 哈希表查找 + 实时评分计算（指数运算，常数级） |
| GetEntry | O(1) | 哈希表查找 + 结构体拷贝 + 实时评分计算 |
| CheckAndMigrate | O(N) | N 为总数据量，需遍历所有数据并重算评分 |

### 11.2 空间复杂度

- O(N)：存储 N 个数据条目及其元数据
- 每个条目额外开销：约 100 字节（时间戳、计数、评分等元数据）

## 12. 注意事项与限制

1. **纯内存实现**: 当前模块仅管理内存中的数据分层，实际的存储介质切换需由上层系统实现
2. **Get 操作加写锁**: 因为要更新访问统计，高并发读场景下可能成为瓶颈；可考虑用 GetScore 替代仅查询评分的场景
3. **CheckAndMigrate 全量扫描**: 数据量很大时（百万级以上），建议降低检查频率或分批处理
4. **评分实时计算成本**: GetScore / GetEntry 每次调用都执行实时评分计算（含指数函数），极端高频调用下有一定 CPU 开销
5. **新数据加成效应**: 新创建的数据有热度加成，可能暂时提升到热层，属正常现象
6. **自适应阈值调整间隔**: 调整不会立即生效，需等待 AdjustInterval 间隔
7. **冷降级延迟**: 数据变冷后需要经过 ColdCheckCycles 个检查周期才会真正降级，避免抖动
8. **阈值缓冲区间**: 热阈值始终大于冷阈值，两者之间的缓冲区可减少频繁迁移
9. **构造函数错误处理**: `NewHotColdManagerWithConfig` 现在返回 `(*HotColdManager, error)`，调用者必须检查错误；零值配置会自动填充默认值，但明显错误的配置会返回 `ErrInvalidConfig`
