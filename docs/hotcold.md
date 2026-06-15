# HotCold 冷热数据分离检测器模块

## 1. 模块概述

HotCold 是一个冷热数据分离检测与管理模块，专为需要优化存储成本和访问性能的系统设计。模块通过热度评分算法自动识别热数据（频繁访问）和冷数据（极少访问），并在快速存储和低成本存储之间自动迁移数据，实现性能与成本的动态平衡。

**包路径**: `internal/hotcold`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 热度评分 | 基于访问频率和新近度计算综合热度评分，反映数据冷热程度 |
| 热数据提升 | 热度评分超过热阈值时，自动迁移到快速存储（热层） |
| 冷数据降级 | 热度评分连续低于冷阈值达到指定周期后，迁移到低成本存储（冷层） |
| 自适应阈值 | 根据系统整体负载和数据量动态调整冷热阈值，避免频繁迁移 |
| 并发安全 | 内置读写锁，支持高并发场景下的数据访问和迁移 |

## 3. 核心结构体与职责

### 3.1 HotColdManager

冷热数据管理器，是模块的核心入口，对外提供所有操作接口。

```go
type HotColdManager struct {
    mu             sync.RWMutex
    hotStore       map[string]*DataEntry
    coldStore      map[string]*DataEntry
    cfg            Config
    lastAdjustTime time.Time
    totalAccesses  int64
}
```

**职责**:
- 管理热存储（hotStore）和冷存储（coldStore）两层数据
- 协调数据在冷热层之间的迁移
- 维护配置参数和自适应阈值调整状态
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
- `Score`: 当前热度评分

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
| `HotCapacityRatio` | 0.3 | 热数据容量占比目标（用于自适应阈值） |
| `AutoAdjustThresholds` | true | 是否启用自适应阈值调整 |
| `AdjustInterval` | 5分钟 | 自适应阈值调整的时间间隔 |
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

## 4. 热度评分算法

### 4.1 评分公式

热度评分综合考虑**访问频率**和**访问新近度**两个维度，采用指数衰减模型：

```
score = accessCount * decayFactor * newEntryBoost
```

其中：

- **accessCount**: 累计访问次数
- **decayFactor**: 时间衰减因子，随距最近访问的时间增长而衰减
- **newEntryBoost**: 新数据热度加成（创建时间在一个半衰期内有效）

### 4.2 指数衰减机制

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

### 4.3 新数据热度加成

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

## 5. 数据迁移判定逻辑

### 5.1 热数据提升（Promotion）

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

### 5.2 冷数据降级（Demotion）

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

### 5.3 迁移时机汇总

| 操作 | 是否检查提升 | 是否检查降级 | 说明 |
|------|-------------|-------------|------|
| Get | ✅（如果在冷层） | ❌ | 访问冷数据时可能触发提升 |
| Put | ✅（如果在冷层） | ❌ | 写入冷数据时可能触发提升 |
| CheckAndMigrate | ✅ | ✅ | 全量检查，双向迁移 |

## 6. 自适应阈值调整

### 6.1 设计背景

固定阈值在流量波动时可能导致问题：
- **流量高峰**: 大量数据被提升为热数据，快速存储容量不足
- **流量低谷**: 热数据过少，快速存储利用率低
- **频繁迁移**: 阈值附近的数据反复升降，产生"乒乓效应"

### 6.2 调整策略

自适应阈值基于**热数据容量占比**目标进行动态调整：

```
目标: 热数据量 / 总数据量 ≈ HotCapacityRatio (默认 30%)
```

**调整规则**:
- 若热数据占比 > 目标的 110% → 提高阈值（让更少数据成为热数据）
- 若热数据占比 < 目标的 90% → 降低阈值（让更多数据成为热数据）
- 在目标范围内 → 不调整

**调整幅度**: 每次调整 ±10%（乘以 1.1 或 0.9）

### 6.3 约束条件

为防止阈值漂移过大，设置上下限约束：

- 热阈值范围: [MinHotThreshold, MaxHotThreshold]
- 冷阈值范围: [MinColdThreshold, MaxColdThreshold]
- 热阈值始终 > 冷阈值（确保存在缓冲区间隔）

### 6.4 调整频率

- 仅在 CheckAndMigrate() 时检查是否需要调整
- 两次调整之间至少间隔 AdjustInterval（默认 5 分钟）
- 避免频繁调整导致系统不稳定

## 7. 使用示例

### 7.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/hotcold"
)

func main() {
    // 使用默认配置创建管理器
    manager := hotcold.NewHotColdManager()

    // 写入数据（默认进入冷层）
    manager.Put("user:1001", map[string]interface{}{
        "name":  "Alice",
        "email": "alice@example.com",
    })

    // 读取数据（更新热度）
    if value, ok, err := manager.Get("user:1001"); ok && err == nil {
        fmt.Println("User:", value)
    }

    // 查看热度评分
    if score, ok, _ := manager.GetScore("user:1001"); ok {
        fmt.Printf("Hot score: %.2f\n", score)
    }

    // 查看各层数据量
    fmt.Printf("Hot data: %d, Cold data: %d, Total: %d\n",
        manager.HotCount(), manager.ColdCount(), manager.TotalCount())
}
```

### 7.2 自定义配置

```go
cfg := hotcold.Config{
    HotThreshold:     15.0,     // 热阈值设为 15
    ColdThreshold:    3.0,      // 冷阈值设为 3
    DecayHalfLife:    2 * time.Hour, // 半衰期设为 2 小时
    ColdCheckCycles:  5,        // 连续 5 个周期才降级
    HotCapacityRatio: 0.2,      // 目标热数据占比 20%
}
manager := hotcold.NewHotColdManagerWithConfig(cfg)
```

### 7.3 定期数据迁移检查

```go
// 启动后台 goroutine 定期检查
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

### 7.4 获取数据条目详情

```go
entry, exists, err := manager.GetEntry("user:1001")
if exists && err == nil {
    fmt.Printf("Key: %s\n", entry.Key)
    fmt.Printf("Access count: %d\n", entry.AccessCount)
    fmt.Printf("Last access: %v\n", entry.LastAccessTime)
    fmt.Printf("Tier: %s\n", map[hotcold.DataTier]string{
        hotcold.TierHot:  "HOT",
        hotcold.TierCold: "COLD",
    }[entry.Tier])
    fmt.Printf("Score: %.2f\n", entry.Score)
}
```

### 7.5 批量操作

```go
// 获取所有热数据键
hotKeys := manager.GetAllHotKeys()
fmt.Printf("Hot keys: %v\n", hotKeys)

// 获取所有冷数据键
coldKeys := manager.GetAllColdKeys()
fmt.Printf("Cold keys: %v\n", coldKeys)

// 删除数据
deleted := manager.Delete("old_key")
fmt.Println("Deleted:", deleted)
```

### 7.6 禁用自适应阈值

```go
cfg := hotcold.DefaultConfig()
cfg.AutoAdjustThresholds = false
cfg.HotThreshold = 20.0
cfg.ColdThreshold = 5.0

manager := hotcold.NewHotColdManagerWithConfig(cfg)
// 此时阈值固定为 20.0 和 5.0，不会随负载变化
```

## 8. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 键不存在 | 预留错误，当前返回 `(value, false, nil)` |
| `ErrInvalidConfig` | 配置无效 | 预留错误 |
| `ErrNilManager` | 管理器为空 | 对 nil 指针调用方法时 |
| `ErrEmptyKey` | 键为空字符串 | Put / Get / Delete 传入空键时 |

## 9. 并发安全

模块使用 `sync.RWMutex` 保证并发安全：

| 操作 | 锁类型 | 说明 |
|------|--------|------|
| Put | 写锁 | 修改数据需要独占访问 |
| Get | 写锁 | Get 会更新访问计数和评分，需要写锁 |
| Delete | 写锁 | 删除数据需要独占访问 |
| GetScore | 读锁 | 只读操作，可并发 |
| GetEntry | 读锁 | 返回副本，可并发 |
| HotCount / ColdCount | 读锁 | 只读计数 |
| GetAllHotKeys / GetAllColdKeys | 读锁 | 返回键列表副本 |
| CheckAndMigrate | 写锁 | 迁移操作需要独占访问 |

**注意**: Get 操作使用写锁而非读锁，因为它会更新访问元数据。这是设计权衡：
- 优点：热度统计精确，迁移决策及时
- 缺点：读操作也会竞争写锁，高并发读场景下有一定性能影响

## 10. 性能特征

### 10.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Put | O(1) | 哈希表查找 + 可能的迁移 |
| Get | O(1) | 哈希表查找 + 评分重算 + 可能的迁移 |
| Delete | O(1) | 哈希表删除 |
| GetScore | O(1) | 哈希表查找 |
| GetEntry | O(1) | 哈希表查找 + 结构体拷贝 |
| CheckAndMigrate | O(N) | N 为总数据量，需遍历所有数据 |

### 10.2 空间复杂度

- O(N)：存储 N 个数据条目及其元数据
- 每个条目额外开销：约 100 字节（时间戳、计数、评分等元数据）

## 11. 注意事项与限制

1. **纯内存实现**: 当前模块仅管理内存中的数据分层，实际的存储介质切换需由上层系统实现
2. **Get 操作加写锁**: 因为要更新访问统计，高并发读场景下可能成为瓶颈
3. **CheckAndMigrate 全量扫描**: 数据量很大时（百万级以上），建议降低检查频率或分批处理
4. **新数据加成效应**: 新创建的数据有热度加成，可能暂时提升到热层，属正常现象
5. **自适应阈值调整间隔**: 调整不会立即生效，需等待 AdjustInterval 间隔
6. **冷降级延迟**: 数据变冷后需要经过 ColdCheckCycles 个检查周期才会真正降级，避免抖动
7. **阈值缓冲区间**: 热阈值始终大于冷阈值，两者之间的缓冲区可减少频繁迁移
