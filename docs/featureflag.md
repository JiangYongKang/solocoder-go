# 功能开关评估器模块 (Feature Flag Evaluator)

## 1. 模块概述

功能开关评估器（Feature Flag Evaluator）是一个用于在运行时控制功能发布和灰度的轻量级模块。它支持三种类型的功能开关，提供热更新能力、稳定的用户分桶算法以及完整的变更审计日志。

### 主要功能

- **布尔型开关**：简单的开/关控制
- **百分比灰度开关**：按用户比例进行灰度发布
- **白名单开关**：基于用户ID的精确控制
- **热更新**：运行时动态修改开关配置，立即生效
- **稳定分桶**：同一用户始终落在同一分桶
- **审计日志**：完整记录所有配置变更

## 2. 核心结构体

### 2.1 Evaluator

功能开关评估器主结构体，负责管理所有开关、评估请求和审计日志。

```go
type Evaluator struct {
    mu      sync.RWMutex
    flags   map[string]*FlagConfig
    audit   []*AuditLogEntry
    seed    hashSeed
}
```

**职责**：
- 管理开关的增删改查
- 执行开关评估逻辑
- 维护审计日志
- 提供线程安全的并发访问控制

**构造函数**：
- `NewEvaluator()` - 使用默认种子创建评估器
- `NewEvaluatorWithSeed(seed uint64)` - 使用自定义种子创建评估器（用于多环境一致性）

### 2.2 FlagConfig

功能开关配置结构体，定义单个开关的所有属性。

```go
type FlagConfig struct {
    Key         string      // 唯一标识
    Type        FlagType    // 开关类型
    Enabled     bool        // 布尔型开关值
    Percentage  int         // 百分比灰度值 (0-100)
    Whitelist   []string    // 白名单用户列表
    Description string      // 描述信息
}
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| Key | string | 开关唯一标识，不能为空 |
| Type | FlagType | 开关类型枚举：Boolean/Percentage/Whitelist |
| Enabled | bool | 布尔型开关的状态值 |
| Percentage | int | 百分比开关的灰度比例，范围 [0, 100] |
| Whitelist | []string | 白名单开关的用户ID列表 |
| Description | string | 可选的描述信息 |

### 2.3 FlagType

开关类型枚举。

```go
type FlagType int

const (
    FlagTypeBoolean    FlagType = iota  // 布尔型
    FlagTypePercentage                   // 百分比灰度
    FlagTypeWhitelist                    // 白名单
)
```

### 2.4 AuditLogEntry

审计日志条目，记录每次配置变更。

```go
type AuditLogEntry struct {
    Timestamp   time.Time     // 变更时间戳
    FlagKey     string        // 开关标识
    Before      *FlagConfig   // 变更前配置快照
    After       *FlagConfig   // 变更后配置快照
    Operation   string        // 操作类型描述
}
```

### 2.5 AuditLogQuery

审计日志查询条件。

```go
type AuditLogQuery struct {
    FlagKey   string        // 按开关标识过滤
    StartTime *time.Time    // 起始时间（含）
    EndTime   *time.Time    // 终止时间（含）
}
```

## 3. 百分比灰度分桶算法

### 3.1 算法原理

百分比灰度采用 **SHA-256 哈希 + 固定种子** 的确定性分桶算法，确保同一用户在相同配置下始终获得一致的评估结果。

```
分桶值 = SHA256(固定种子 + 用户ID) → 取前8字节 → 对100取模
```

### 3.2 算法步骤

1. **拼接输入**：将 8 字节的固定种子与用户ID字符串拼接
2. **哈希计算**：使用 SHA-256 算法对拼接后的数据进行哈希
3. **提取数值**：取哈希结果的前 8 字节，转换为 uint64 大端整数
4. **取模分桶**：将该整数对 100 取模，得到 0-99 的分桶值
5. **范围判断**：若分桶值 < 灰度比例百分比，则返回 true（启用），否则返回 false

### 3.3 算法特点

- **确定性**：相同的 (种子, 用户ID) 输入始终产生相同的分桶值
- **均匀分布**：SHA-256 的输出均匀随机，用户分桶覆盖 0-99 的所有值
- **不可预测**：无法从用户ID直接推断分桶归属
- **种子可配置**：不同环境可使用不同种子，避免分桶结果一致
- **无状态**：无需存储用户分桶信息，纯计算即可

### 3.4 代码实现（伪代码）

```go
func computeUserBucket(userID string, seed uint64) int {
    h := sha256.New()
    seedBytes := make([]byte, 8)
    binary.BigEndian.PutUint64(seedBytes, seed)
    h.Write(seedBytes)
    h.Write([]byte(userID))
    hash := h.Sum(nil)

    value := binary.BigEndian.Uint64(hash[:8])
    return int(value % 100)
}
```

### 3.5 边界情况

- **百分比 = 0**：所有用户返回 false（不启用）
- **百分比 = 100**：所有用户返回 true（全量启用）
- **用户ID为空**：返回 ErrNilUserID 错误

## 4. API 接口

### 4.1 开关管理

| 方法 | 说明 |
|------|------|
| `CreateFlag(cfg *FlagConfig) error` | 创建新开关 |
| `UpdateFlag(cfg *FlagConfig) error` | 更新已有开关配置 |
| `DeleteFlag(key string) error` | 删除开关 |
| `GetFlag(key string) (*FlagConfig, error)` | 获取单个开关配置 |
| `ListFlags() []*FlagConfig` | 列出所有开关（按Key排序） |

### 4.2 开关评估

| 方法 | 说明 |
|------|------|
| `Evaluate(key string, userID string) (bool, error)` | 评估指定开关的当前状态 |

### 4.3 热更新（便捷方法）

| 方法 | 说明 | 类型校验 |
|------|------|----------|
| `SetBooleanValue(key string, enabled bool) error` | 设置布尔开关值 | 仅允许 Boolean 类型，否则返回 `ErrInvalidFlagType` |
| `SetPercentage(key string, percentage int) error` | 设置百分比灰度值 | 仅允许 Percentage 类型，否则返回 `ErrInvalidFlagType` |
| `AddToWhitelist(key string, userID string) error` | 向白名单添加用户 | 仅允许 Whitelist 类型，否则返回 `ErrInvalidFlagType` |
| `RemoveFromWhitelist(key string, userID string) error` | 从白名单移除用户 | 仅允许 Whitelist 类型，否则返回 `ErrInvalidFlagType` |
| `ChangeFlagType(key, newType, enabled, percentage, whitelist) error` | 切换开关类型 | 无类型限制，允许从任意类型切换到任意类型 |

### 4.4 审计日志

| 方法 | 说明 |
|------|------|
| `AuditLogCount() int` | 获取审计日志总数 |
| `QueryAuditLogs(query AuditLogQuery) []*AuditLogEntry` | 按条件查询审计日志 |

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/featureflag"
)

func main() {
    // 创建评估器
    eval := featureflag.NewEvaluator()

    // 创建布尔型开关
    _ = eval.CreateFlag(&featureflag.FlagConfig{
        Key:         "new-ui",
        Type:        featureflag.FlagTypeBoolean,
        Enabled:     false,
        Description: "新UI开关",
    })

    // 评估布尔开关
    enabled, _ := eval.Evaluate("new-ui", "")
    fmt.Printf("新UI开关: %v\n", enabled) // 输出: false

    // 热更新：打开开关
    _ = eval.SetBooleanValue("new-ui", true)
    enabled, _ = eval.Evaluate("new-ui", "")
    fmt.Printf("新UI开关: %v\n", enabled) // 输出: true
}
```

### 5.2 百分比灰度发布

```go
// 创建百分比灰度开关（10% 用户进入）
_ = eval.CreateFlag(&featureflag.FlagConfig{
    Key:        "checkout-v2",
    Type:       featureflag.FlagTypePercentage,
    Percentage: 10,
})

// 评估（用户ID不能为空）
userID := "user-12345"
result, _ := eval.Evaluate("checkout-v2", userID)
if result {
    fmt.Println("进入新结账流程")
} else {
    fmt.Println("使用旧结账流程")
}

// 扩大灰度到 50%
_ = eval.SetPercentage("checkout-v2", 50)

// 全量发布
_ = eval.SetPercentage("checkout-v2", 100)
```

### 5.3 白名单开关

```go
// 创建白名单开关
_ = eval.CreateFlag(&featureflag.FlagConfig{
    Key:       "beta-feature",
    Type:      featureflag.FlagTypeWhitelist,
    Whitelist: []string{"qa-user-1", "qa-user-2"},
})

// 评估
r1, _ := eval.Evaluate("beta-feature", "qa-user-1")
fmt.Println(r1) // true

r2, _ := eval.Evaluate("beta-feature", "normal-user")
fmt.Println(r2) // false

// 动态添加用户
_ = eval.AddToWhitelist("beta-feature", "pm-user-1")

// 动态移除用户
_ = eval.RemoveFromWhitelist("beta-feature", "qa-user-2")
```

### 5.4 开关类型切换

```go
// 创建百分比灰度开关
_ = eval.CreateFlag(&featureflag.FlagConfig{
    Key:        "smart-sort",
    Type:       featureflag.FlagTypePercentage,
    Percentage: 20,
})

// 灰度完成，切换为布尔型全量开启
_ = eval.ChangeFlagType(
    "smart-sort",
    featureflag.FlagTypeBoolean,
    true,   // enabled
    0,      // percentage (布尔型不使用)
    nil,    // whitelist (布尔型不使用)
)
```

### 5.5 查询审计日志

```go
// 查询特定开关的所有变更
logs := eval.QueryAuditLogs(featureflag.AuditLogQuery{
    FlagKey: "checkout-v2",
})

for _, log := range logs {
    fmt.Printf("[%s] %s - %s\n",
        log.Timestamp.Format("2006-01-02 15:04:05"),
        log.Operation,
        log.FlagKey,
    )
}

// 按时间范围查询
start := time.Now().Add(-24 * time.Hour)
end := time.Now()
recentLogs := eval.QueryAuditLogs(featureflag.AuditLogQuery{
    StartTime: &start,
    EndTime:   &end,
})
```

### 5.6 多环境一致性

```go
// 生产环境使用种子1
prodEval := featureflag.NewEvaluatorWithSeed(0xDEADBEEF)

// 测试环境使用种子2
testEval := featureflag.NewEvaluatorWithSeed(0xC0FFEE)

// 不同种子下同一用户可能分到不同桶
// 同一种子下同一用户始终分到相同桶（跨进程/跨机器一致）
```

## 6. 错误处理

| 错误变量 | 说明 |
|----------|------|
| `ErrFlagNotFound` | 指定的开关不存在 |
| `ErrFlagAlreadyExists` | 创建时开关已存在 |
| `ErrInvalidFlagType` | 无效的开关类型 |
| `ErrInvalidPercentage` | 百分比超出 [0,100] 范围 |
| `ErrNilUserID` | 百分比/白名单评估时用户ID为空 |
| `ErrInvalidConfig` | 配置验证失败 |
| `ErrNilConfig` | 传入了空配置 |
| `ErrNilFlagKey` | 开关标识为空 |

## 7. 线程安全

Evaluator 使用 `sync.RWMutex` 实现线程安全：
- **读操作**（Evaluate、GetFlag、ListFlags、QueryAuditLogs）：使用读锁，支持并发
- **写操作**（CreateFlag、UpdateFlag、DeleteFlag、Set*、Add/Remove*）：使用写锁，串行执行

所有返回给调用方的配置对象均为深拷贝，避免外部修改影响内部状态。

## 8. 类型校验策略

### 8.1 设计原则

所有便捷更新方法（`SetBooleanValue`、`SetPercentage`、`AddToWhitelist`、`RemoveFromWhitelist`）均执行严格的类型校验，确保操作仅对匹配类型的开关生效。`ChangeFlagType` 和 `UpdateFlag` 不受类型校验限制，因为它们的设计目的就是允许跨类型变更。

### 8.2 校验规则

| 方法 | 允许的开关类型 | 拒绝时的错误 |
|------|---------------|-------------|
| `SetBooleanValue` | `FlagTypeBoolean` | `ErrInvalidFlagType: flag type is X, not Boolean` |
| `SetPercentage` | `FlagTypePercentage` | `ErrInvalidFlagType: flag type is X, not Percentage` |
| `AddToWhitelist` | `FlagTypeWhitelist` | `ErrInvalidFlagType: flag type is X, not Whitelist` |
| `RemoveFromWhitelist` | `FlagTypeWhitelist` | `ErrInvalidFlagType: flag type is X, not Whitelist` |

### 8.3 类型校验失败的副作用

当类型校验失败时：
- **不会修改开关配置**：开关的内部状态保持不变
- **不会记录审计日志**：只有成功执行的变更才会生成审计日志条目
- **立即返回错误**：调用方可根据返回的 `ErrInvalidFlagType` 识别类型不匹配

### 8.4 示例

```go
// 创建百分比灰度开关
_ = eval.CreateFlag(&featureflag.FlagConfig{
    Key:        "gradual-rollout",
    Type:       featureflag.FlagTypePercentage,
    Percentage: 10,
})

// 错误：对百分比开关调用 SetBooleanValue
err := eval.SetBooleanValue("gradual-rollout", true)
// err = ErrInvalidFlagType: flag type is Percentage, not Boolean

// 正确：使用 SetPercentage
err = eval.SetPercentage("gradual-rollout", 50)
// err = nil

// 正确：使用 ChangeFlagType 切换类型后再设置布尔值
_ = eval.ChangeFlagType("gradual-rollout", featureflag.FlagTypeBoolean, true, 0, nil)
_ = eval.SetBooleanValue("gradual-rollout", false) // 此时类型已是 Boolean，允许操作
```

## 9. 审计日志时间范围查询

### 9.1 查询接口

通过 `QueryAuditLogs(query AuditLogQuery)` 方法按时间范围过滤审计日志。`AuditLogQuery` 支持三个可选维度：

- **FlagKey**：按开关标识过滤（精确匹配）
- **StartTime**：只返回时间戳 >= StartTime 的日志（含边界）
- **EndTime**：只返回时间戳 <= EndTime 的日志（含边界）

三个维度可以单独使用或任意组合，均为可选参数（nil 表示不过滤该维度）。

### 9.2 时间边界行为

时间范围的过滤是**闭区间**（inclusive）：

- `StartTime` 和 `EndTime` 均为包含边界
- 若某条日志的 `Timestamp` 恰好等于 `StartTime` 或 `EndTime`，该日志会被包含在结果中
- 使用 `time.Before()` 判断是否早于 `StartTime`，使用 `time.After()` 判断是否晚于 `EndTime`

### 9.3 查询组合场景

| 场景 | StartTime | EndTime | FlagKey | 行为 |
|------|-----------|---------|---------|------|
| 全量查询 | nil | nil | "" | 返回所有日志 |
| 按开关过滤 | nil | nil | "flag-a" | 返回 flag-a 的所有日志 |
| 仅起始时间 | &start | nil | "" | 返回 start 之后的日志 |
| 仅终止时间 | nil | &end | "" | 返回 end 之前的日志 |
| 时间范围 | &start | &end | "" | 返回 [start, end] 内的日志 |
| 开关 + 时间范围 | &start | &end | "flag-a" | 返回 flag-a 在 [start, end] 内的日志 |

### 9.4 示例

```go
// 查询最近 1 小时内 flag-a 的变更
start := time.Now().Add(-1 * time.Hour)
end := time.Now()
logs := eval.QueryAuditLogs(featureflag.AuditLogQuery{
    FlagKey:   "flag-a",
    StartTime: &start,
    EndTime:   &end,
})

// 查询某个时间点之后的所有变更
afterDeploy := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)
recentLogs := eval.QueryAuditLogs(featureflag.AuditLogQuery{
    StartTime: &afterDeploy,
})

// 查询某个时间点之前的所有变更（用于回溯问题）
incidentTime := time.Date(2026, 6, 20, 14, 30, 0, 0, time.UTC)
beforeIncident := eval.QueryAuditLogs(featureflag.AuditLogQuery{
    EndTime: &incidentTime,
})
```
