# A/B 测试流量分割器 (ABTest) 模块需求文档

## 1. 模块概述

A/B 测试流量分割器是一个基于哈希的多实验流量分配组件，用于在应用中运行多个独立的 A/B 测试实验。通过确定性哈希函数将用户稳定分配到不同的实验分组，支持多实验正交流量分配、可配置的流量比例以及实验效果指标采集。

本模块适用于产品功能灰度发布、UI 方案对比、算法效果验证等场景，确保同一用户在同一实验中始终被分配到相同分组，且多个实验之间的流量分配互不干扰。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 用户哈希分桶 (HashBucket) | 根据用户 ID 通过哈希函数将用户稳定分配到 0-99 的 100 个桶中，同一用户多次调用结果一致 |
| F2 | 实验感知哈希分桶 (HashBucketWithExperiment) | 在用户哈希基础上叠加实验 ID 哈希，确保同一用户在不同实验中获得独立的桶号 |
| F3 | 实验管理 (Add/Remove/Get/List) | 支持动态添加、删除、查询和列举实验，每个实验拥有独立的流量配置 |
| F4 | 正交流量分配 (AssignGroup) | 根据用户 ID 和实验 ID 返回该用户在指定实验中的分组（实验组/对照组/不参与） |
| F5 | 批量实验分配 (AssignAllExperiments) | 一次性返回用户在所有已注册实验中的分组结果 |
| F6 | 实验组与对照组比例配置 | 每个实验支持独立配置实验组和对照组的流量百分比，两者之和不超过 100 |
| F7 | 自定义分组名称 | 支持为实验组和对照组指定自定义名称（如 "new_ui"、"old_ui"），默认使用 "experiment" 和 "control" |
| F8 | 指标上报 (RecordMetric) | 为指定实验的指定分组记录指标事件和对应的数值，支持累计统计 |
| F9 | 指标查询 (GetExperimentMetrics/GetGroupMetric) | 查询实验的整体指标数据或特定分组的特定指标统计值 |
| F10 | 指标重置 (ResetExperimentMetrics) | 清空指定实验的所有指标数据，用于重新开始统计 |

## 3. 核心结构体与职责

### 3.1 Experiment - 实验配置

```go
type Experiment struct {
    ID                  string // 实验唯一标识符
    ExperimentGroupPct  int    // 实验组流量百分比 (0-100)
    ControlGroupPct     int    // 对照组流量百分比 (0-100)
    ExperimentGroupName string // 实验组名称（可选，默认 "experiment"）
    ControlGroupName    string // 对照组名称（可选，默认 "control"）
}
```

**配置约束：**
- `ExperimentGroupPct + ControlGroupPct <= 100`，超出则返回 `ErrTrafficExceedsLimit`
- 两个百分比均不能为负数，否则返回 `ErrInvalidTrafficPercent`
- 剩余 `100 - (ExperimentGroupPct + ControlGroupPct)` 的流量标记为不参与实验（`no_assign`）
- 若未指定分组名称，默认使用 `GroupExperiment` ("experiment") 和 `GroupControl` ("control")

### 3.2 ABTest - A/B 测试管理器

```go
type ABTest struct {
    mu          sync.RWMutex                   // 读写锁，保护并发安全
    experiments map[string]*Experiment         // 实验ID → 实验配置映射
    metrics     map[string]*ExperimentMetrics  // 实验ID → 指标数据映射
}
```

**主要职责：**
- 管理所有实验的生命周期（添加、删除、查询）
- 维护每个实验的指标统计数据
- 提供线程安全的分组分配和指标上报接口
- 通过读写锁优化读多写少场景的并发性能

### 3.3 GroupMetrics - 分组指标统计

```go
type GroupMetrics struct {
    EventCount map[string]int64   // 指标名称 → 事件发生次数
    MetricSum  map[string]float64 // 指标名称 → 指标值累计总和
}
```

**主要职责：**
- 存储单个实验分组的所有指标统计数据
- `EventCount` 记录指标被上报的次数
- `MetricSum` 记录指标值的累计总和，用于计算平均值等统计量

### 3.4 ExperimentMetrics - 实验指标统计

```go
type ExperimentMetrics struct {
    GroupMetrics map[string]*GroupMetrics // 分组名称 → 该分组的指标统计
}
```

**主要职责：**
- 存储单个实验所有分组的指标数据
- 包含实验组、对照组和不参与组（`no_assign`）三个分组的统计

### 3.5 预定义常量与错误

**常量：**

| 常量名 | 值 | 含义 |
|--------|----|------|
| `BucketCount` | 100 | 分桶总数，0-99 共 100 个桶 |
| `GroupControl` | "control" | 默认对照组名称 |
| `GroupExperiment` | "experiment" | 默认实验组名称 |
| `GroupNoAssign` | "no_assign" | 不参与实验的分组名称 |

**错误变量：**

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyUserID` | 用户 ID 为空 | 调用哈希或分配方法时传入空用户 ID |
| `ErrEmptyExperimentID` | 实验 ID 为空 | 操作实验时传入空实验 ID |
| `ErrExperimentNotFound` | 实验不存在 | 操作未注册的实验 ID |
| `ErrExperimentExists` | 实验已存在 | 添加重复 ID 的实验 |
| `ErrInvalidTrafficPercent` | 流量百分比无效 | 流量百分比为负数 |
| `ErrTrafficExceedsLimit` | 流量超出限制 | 实验组+对照组百分比 > 100 |
| `ErrEmptyGroupName` | 分组名称为空 | 指标上报时分组名称为空 |
| `ErrEmptyMetricName` | 指标名称为空 | 指标操作时指标名称为空 |

## 4. 核心机制详解

### 4.1 哈希分桶算法 (HashBucket)

**算法原理：**

```go
func HashBucket(userID string) (int, error) {
    h := fnv.New32a()
    h.Write([]byte(userID))
    hash := h.Sum32()
    return int(hash % uint32(BucketCount)), nil
}
```

**关键特性：**
1. **确定性**：同一用户 ID 始终返回相同的桶号，不随时间、系统重启等因素变化
2. **均匀分布**：使用 FNV-1a 32 位哈希算法，确保用户在 100 个桶中均匀分布
3. **范围约束**：结果始终在 [0, 99] 范围内，共 100 个桶
4. **高性能**：纯内存计算，无 I/O 操作，时间复杂度 O(1)

**分桶结果用途：**
- 作为所有实验流量分配的基础机制
- 不直接用于实验分组，需结合实验 ID 进行二次哈希

### 4.2 正交流量分配算法 (HashBucketWithExperiment)

**算法原理：**

```go
func HashBucketWithExperiment(userID, experimentID string) (int, error) {
    h1 := fnv.New32a()
    h1.Write([]byte(userID))
    userHash := h1.Sum32()

    h2 := fnv.New32a()
    h2.Write([]byte(experimentID))
    expHash := h2.Sum32()

    // 使用 MurmurHash 风格的混合函数确保正交性
    mixed := userHash ^ (expHash * 2654435761)
    mixed = (mixed ^ (mixed >> 16)) * 2246822507
    mixed = (mixed ^ (mixed >> 13)) * 3266489909
    mixed = mixed ^ (mixed >> 16)

    return int(mixed % uint32(BucketCount)), nil
}
```

**正交性原理：**

正交分配的核心目标是：**同一用户在不同实验中的分组结果相互独立、互不影响**。

```
用户 U 在实验 A 中的分组 = f(Hash(U, A))
用户 U 在实验 B 中的分组 = f(Hash(U, B))

其中 Hash(U, A) ≠ Hash(U, B) 且两者统计独立
```

**实现要点：**
1. 分别对用户 ID 和实验 ID 进行独立哈希
2. 使用黄金分割乘数（2654435761 是接近 2^32/φ 的质数）扩散实验哈希的影响
3. 应用多轮 XOR-shift 混合操作，消除用户哈希和实验哈希之间的相关性
4. 最终结果模 100 得到桶号

**正交性验证：**
- 对于两个各占 50% 流量的实验，用户在两个实验中被分配到相同分组的概率应接近 50%
- 四个组合（实/实、实/对、对/实、对/对）的用户数应各占约 25%
- 单元测试 `TestAssignAllExperiments_Orthogonal` 验证此特性

### 4.3 分组分配逻辑 (AssignGroup)

**流量区间划分：**

```
桶号范围: 0 ──── 实验组 ──── 实验组+对照组 ──── 100
          │         │                 │
          ▼         ▼                 ▼
      [实验组]   [对照组]         [不参与]
```

**分配流程：**

```
AssignGroup(userID, experimentID)
   │
   ├─ 参数校验 → 空值返回对应错误
   │
   ├─ 计算 bucket = HashBucketWithExperiment(userID, experimentID)
   │
   ├─ 获取实验配置 exp
   │
   ├─ bucket < exp.ExperimentGroupPct
   │     └─ 返回 exp.ExperimentGroupName
   │
   ├─ bucket < exp.ExperimentGroupPct + exp.ControlGroupPct
   │     └─ 返回 exp.ControlGroupName
   │
   └─ 其他情况
         └─ 返回 GroupNoAssign
```

**示例：**
- 实验配置：实验组 30%，对照组 30%
- 桶号 0-29 → 实验组（30 个桶，30%）
- 桶号 30-59 → 对照组（30 个桶，30%）
- 桶号 60-99 → 不参与（40 个桶，40%）

### 4.4 指标采集