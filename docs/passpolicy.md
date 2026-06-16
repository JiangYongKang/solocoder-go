# 密码策略引擎模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [密码复杂度校验详解](#4-密码复杂度校验详解)
5. [历史密码检查机制](#5-历史密码检查机制)
6. [密码过期与生命周期管理](#6-密码过期与生命周期管理)
7. [bcrypt 自适应哈希机制](#7-bcrypt-自适应哈希机制)
8. [完整密码生命周期](#8-完整密码生命周期)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [并发安全](#11-并发安全)
12. [配置参数与默认值](#12-配置参数与默认值)

---

## 1. 模块概述

密码策略引擎模块（Password Policy Engine）是一个功能完整的密码安全管理组件，提供密码复杂度校验、历史密码复用检测、密码过期提醒、bcrypt 自适应哈希存储等核心功能。模块设计用于需要高安全等级密码管理的系统，确保密码从创建、使用到过期回收的全生命周期安全。

**包路径**: `internal/passpolicy`

**设计目标**:
- 提供可配置的密码复杂度规则，强制用户使用安全密码
- 防止密码重复使用，降低历史密码泄露带来的风险
- 通过密码过期机制强制用户定期更换密码
- 使用 bcrypt 自适应哈希算法，支持动态提升安全强度
- 线程安全设计，支持高并发场景下的密码操作

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 密码复杂度校验 | 支持最小长度、大写字母、小写字母、数字、特殊字符等可配置规则，返回详细违规原因 |
| 历史密码检查 | 维护每个用户的密码哈希历史，深度可配置，禁止重复使用近期密码 |
| 密码过期管理 | 记录密码创建时间，自动计算剩余有效天数，支持强制更换密码标记 |
| bcrypt 自适应哈希 | 使用 bcrypt 存储密码，cost 因子可配置并支持动态升级 |
| 自动重新哈希 | 验证密码时，若存储的 cost 低于当前配置，自动升级为更高强度的哈希 |
| 密码状态查询 | 返回密码是否过期、剩余有效天数、是否处于警告期等状态信息 |
| 管理员强制重置 | 支持管理员标记用户必须在下次登录时修改密码 |
| 用户数据管理 | 支持查询密码哈希、历史记录、删除用户等管理操作 |

---

## 3. 核心结构体与职责

### 3.1 Engine

主引擎结构体，对外提供所有密码管理操作接口，内部维护用户密码状态和配置信息。

```go
type Engine struct {
    mu     sync.RWMutex
    config Config
    users  map[string]*UserState
    now    func() time.Time
}
```

**职责**:
- 管理全局配置参数（复杂度规则、哈希强度、历史深度等）
- 维护所有用户的密码状态数据
- 通过 `sync.RWMutex` 保证并发安全
- 提供时间注入接口 `now`，便于测试和时间模拟
- 协调密码校验、哈希、历史记录等各子功能

### 3.2 Config

配置结构体，用于定制密码策略引擎的行为参数。

```go
type Config struct {
    MinLength    int
    BcryptCost   int
    HistoryDepth int
    ExpiryDays   int
    WarningDays  int
    Complexity   ComplexityConfig
}
```

### 3.3 ComplexityConfig

密码复杂度规则配置，每项规则可独立开关。

```go
type ComplexityConfig struct {
    RequireUppercase bool
    RequireLowercase bool
    RequireDigit     bool
    RequireSpecial   bool
}
```

### 3.4 ValidationResult

密码校验结果结构体，返回校验是否通过及具体违规项列表。

```go
type ValidationResult struct {
    Valid      bool
    Violations []PolicyViolation
}
```

**职责**:
- `Valid`: 整体校验是否通过
- `Violations`: 所有违规项的详细列表
- 提供 `ErrorMessages()` 获取人类可读的错误消息列表
- 提供 `CombinedError()` 合并为单一 error 返回

### 3.5 PolicyViolation

单条规则违规详情。

```go
type PolicyViolation struct {
    Err     error
    Message string
}
```

### 3.6 PasswordRecord

当前密码记录，存储哈希值、哈希参数和创建时间。

```go
type PasswordRecord struct {
    Hash      []byte
    Cost      int
    CreatedAt time.Time
}
```

**职责**:
- `Hash`: bcrypt 哈希后的密码字节
- `Cost`: 哈希时使用的 bcrypt cost 因子（用于升级判断）
- `CreatedAt`: 密码设置时间（用于过期计算）

### 3.7 HistoryEntry

历史密码条目，结构与 PasswordRecord 一致，用于历史复用检测。

```go
type HistoryEntry struct {
    Hash      []byte
    Cost      int
    CreatedAt time.Time
}
```

### 3.8 UserState

用户完整的密码状态，包括当前密码、历史记录和管理标记。

```go
type UserState struct {
    UserID       string
    Current      *PasswordRecord
    History      []HistoryEntry
    LastChanged  time.Time
    MustChange   bool
}
```

**职责**:
- `UserID`: 用户唯一标识
- `Current`: 当前有效密码记录（nil 表示无密码）
- `History`: 历史密码列表，按设置时间升序排列
- `LastChanged`: 最近一次密码修改时间
- `MustChange`: 管理员强制下次登录修改密码标记

### 3.9 PasswordStatus

密码状态查询结果，供业务系统展示和判断使用。

```go
type PasswordStatus struct {
    UserID            string
    IsExpired         bool
    DaysRemaining     int
    DaysSinceChanged  int
    IsWarningPeriod   bool
    MustChange        bool
    CreatedAt         time.Time
}
```

**职责**:
- `IsExpired`: 密码是否已过期（过期后禁止登录）
- `DaysRemaining`: 剩余有效天数（负数表示已过期）
- `DaysSinceChanged`: 距上次修改已过天数
- `IsWarningPeriod`: 是否处于临近过期警告期
- `MustChange`: 是否被强制要求修改密码

### 3.10 VerifyResult

密码验证返回结果，包含验证状态、升级信息和密码状态。

```go
type VerifyResult struct {
    Valid         bool
    Rehashed      bool
    NewHash       []byte
    NewCost       int
    PasswordState *PasswordStatus
}
```

**职责**:
- `Valid`: 密码验证是否通过
- `Rehashed`: 是否触发了自动重新哈希（cost 升级）
- `NewHash`/`NewCost`: 重新哈希后的结果（便于持久化同步）
- `PasswordState`: 验证时的密码状态（用于业务逻辑判断）

---

## 4. 密码复杂度校验详解

### 4.1 校验流程

```
ValidatePassword(password)
    │
    ├─→ 步骤 1：检查最小长度
    │       若 len(password) < MinLength
    │       → 添加 ErrPasswordTooShort 违规
    │
    ├─→ 步骤 2：检查大写字母（若开启）
    │       若 !containsUpper(password)
    │       → 添加 ErrPasswordMissingUppercase 违规
    │
    ├─→ 步骤 3：检查小写字母（若开启）
    │       若 !containsLower(password)
    │       → 添加 ErrPasswordMissingLowercase 违规
    │
    ├─→ 步骤 4：检查数字（若开启）
    │       若 !containsDigit(password)
    │       → 添加 ErrPasswordMissingDigit 违规
    │
    └─→ 步骤 5：检查特殊字符（若开启）
            若 !containsSpecial(password)
            → 添加 ErrPasswordMissingSpecial 违规
```

### 4.2 特殊字符判定规则

特殊字符包含三类，满足任一即算通过：
1. **Unicode 标点类** (`unicode.IsPunct`): 如 `.,;:!?'"-()[]{}` 等
2. **Unicode 符号类** (`unicode.IsSymbol`): 如 `©®™¥€$` 等
3. **显式符号集**: `!@#$%^&*()-_=+[]{};:,.<>?/~`|\`
   覆盖常见 ASCII 特殊符号

### 4.3 违规结果处理

- 所有规则**并行检查**，返回完整违规列表，而非仅返回首个违规
- `ValidationResult.ErrorMessages()` 返回人类可读的消息切片
- `ValidationResult.CombinedError()`:
  - 无违规 → `nil`
  - 单条违规 → 返回对应 `error`
  - 多条违规 → 合并为带分隔符的单一错误

---

## 5. 历史密码检查机制

### 5.1 历史记录维护时机

历史密码在**密码变更时**自动维护：

1. **SetPassword（管理员重置）**: 若用户已有密码，将当前密码移入历史
2. **ChangePassword（用户自助修改）**: 修改成功后，将旧密码移入历史

### 5.2 历史深度裁剪

每次添加历史记录后，若总条数超过 `HistoryDepth`：

```go
if len(state.History) > e.config.HistoryDepth {
    state.History = state.History[len(state.History)-e.config.HistoryDepth:]
}
```

- 采用**先进先出（FIFO）**策略，保留最近 N 条
- 裁剪在写入时同步完成，查询时无需额外计算

### 5.3 历史复用检测

在 `ChangePassword` 流程中，对新密码进行历史匹配：

```
ChangePassword(userID, oldPassword, newPassword)
    │
    ├─→ 验证 oldPassword 正确（否则返回 ErrPasswordMismatch）
    │
    ├─→ 若 HistoryDepth > 0，进行历史检查
    │       │
    │       ├─→ 1. 与当前密码比对（不能与当前相同）
    │       │     bcrypt.Compare(current.Hash, newPassword) == nil
    │       │     → 返回 ErrPasswordHistoryReused
    │       │
    │       └─→ 2. 与所有历史密码比对
    │             倒序遍历 History（优先检查更近的密码）
    │             任一匹配 → 返回 ErrPasswordHistoryReused
    │
    └─→ 通过历史检查后，执行复杂度校验和更新
```

**注意**: bcrypt 历史比对是逐个哈希进行，属于计算密集操作。`HistoryDepth` 过大（如 > 20）可能影响修改密码性能。

### 5.4 深度为 0 的特殊行为

当 `HistoryDepth = 0` 时：
- 不维护任何历史记录（SetPassword 时不产生历史条目）
- ChangePassword 时跳过历史复用检查（允许使用任何历史密码）
- 适用于内部系统、测试环境等低安全场景

---

## 6. 密码过期与生命周期管理

### 6.1 过期计算模型

过期判断基于**密码创建时间**与当前时间的差值：

```go
daysSinceChanged := int(now.Sub(password.CreatedAt).Hours() / 24)
daysRemaining := ExpiryDays - daysSinceChanged

isExpired       = ExpiryDays > 0 && daysRemaining <= 0
isWarningPeriod = ExpiryDays > 0 && !isExpired && daysRemaining <= WarningDays
```

- `ExpiryDays = 0` 表示**永不过期**（适用于服务账号、系统账号）
- `WarningDays` 用于业务系统展示"即将过期"提醒，不影响登录逻辑

### 6.2 过期对登录的影响

在 `VerifyPassword` 流程中，密码验证**通过后**检查过期状态：

```
VerifyPassword(userID, password)
    │
    ├─→ bcrypt 哈希比对（失败 → ErrPasswordMismatch）
    │
    ├─→ 计算过期状态
    │
    ├─→ 若 IsExpired == true
    │       result.Valid = false
    │       返回 ErrPasswordExpired
    │
    ├─→ 若 MustChange == true
    │       result.Valid = false
    │       返回 ErrPasswordExpired
    │
    └─→ 验证通过，执行 cost 升级检查
```

**设计要点**: 即使密码正确，过期或被强制修改时，`VerifyPassword` 仍返回错误。业务系统需捕获该错误并跳转至修改密码页面。

### 6.3 强制修改密码（MustChange）

管理员可通过 `ForcePasswordChange(userID)` 设置强制修改标记：

- 仅修改 `UserState.MustChange = true`，不影响密码哈希本身
- 行为与过期一致：VerifyPassword 返回 ErrPasswordExpired
- 用户成功调用 ChangePassword 或 SetPassword 后，MustChange 自动重置为 false

### 6.4 状态查询接口

`GetPasswordStatus(userID)` 用于：
- 用户登录前的状态预览（如在登录页展示"密码将在 X 天后过期"）
- 管理后台的密码安全审计
- 批量任务扫描过期用户并发送邮件提醒

---

## 7. bcrypt 自适应哈希机制

### 7.1 bcrypt 基础

bcrypt 是基于 Blowfish 的自适应密码哈希算法，核心特性：
- **慢哈希**: 故意设计为计算缓慢，抵御暴力破解
- **可调成本**: `cost` 因子（4-31）控制迭代次数，`cost=N` 表示迭代 `2^N` 次
- **内置盐值**: 每次哈希自动生成随机盐，相同密码产生不同哈希

### 7.2 哈希存储格式

```go
PasswordRecord {
    Hash: []byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy")
    Cost: 10
    CreatedAt: ...
}
```

- `Hash` 字段已包含 bcrypt 版本、cost 和盐值，`bcrypt.CompareHashAndPassword` 可直接解析
- `Cost` 字段**冗余存储**，目的是在不解析 Hash 字符串的前提下快速判断是否需要升级

### 7.3 动态 Cost 升级流程

管理员可通过 `UpdateBcryptCost(newCost)` 提升全局哈希强度，升级在用户登录时透明完成：

```
VerifyPassword(userID, password)
    │
    ├─→ 校验通过后
    │
    ├─→ 比较 storedCost vs currentCost
    │       若 storedCost >= currentCost → 无需升级，正常返回
    │       若 storedCost <  currentCost → 触发升级
    │
    ├─→ 升级流程
    │       1. 使用当前明文 password 和 newCost 重新哈希
    │          newHash = bcrypt.GenerateFromPassword(password, currentCost)
    │
    │       2. 更新用户状态
    │          user.Current.Hash = newHash
    │          user.Current.Cost = currentCost
    │
    │       3. 设置结果标记
    │          result.Rehashed = true
    │          result.NewHash   = newHash
    │          result.NewCost   = currentCost
    │
    └─→ 返回验证结果（Valid=true）
```

### 7.4 升级失败的降级处理

若重新哈希过程发生错误（理论上极少，除非内存耗尽）：
- **不影响本次验证**，仍返回 Valid=true
- `Rehashed = false`，下次登录时重试升级
- 保证用户体验优先，安全升级是尽力而为（best-effort）

---

## 8. 完整密码生命周期

### 8.1 状态流转全图

```
                       ┌───────────────────────────────────┐
                       │                                   │
                       │         阶段 1: 创建              │
                       │   管理员 SetPassword / 新用户注册   │
                       │                                   │
                       │   • 复杂度校验 ValidatePassword   │
                       │   • bcrypt 哈希生成               │
                       │   • CreatedAt = 现在              │
                       │   • MustChange = false            │
                       │   • History = []                  │
                       │                                   │
                       └──────────────┬────────────────────┘
                                      │
                                      ▼
                       ┌───────────────────────────────────┐
                       │                                   │
                       │       阶段 2: 正常使用期           │
                       │     DaysRemaining > WarningDays   │
                       │                                   │
                       │   • VerifyPassword → Valid=true   │
                       │   • 登录成功，正常使用             │
                       │   • 可能触发 bcrypt cost 升级      │
                       │                                   │
                       └──────────────┬────────────────────┘
                                      │
                                      ▼
                       ┌───────────────────────────────────┐
                       │                                   │
                       │       阶段 3: 警告期              │
                       │   0 < DaysRemaining ≤ WarningDays │
                       │                                   │
                       │   • VerifyPassword → Valid=true   │
                       │   • PasswordState.IsWarningPeriod  │
                       │   • 业务层提示"X 天后过期"         │
                       │                                   │
                       └──────────────┬────────────────────┘
                                      │
          ┌───────────────────────────┼───────────────────────────┐
          │                           │                           │
          ▼                           ▼                           ▼
┌────────────────────┐   ┌────────────────────┐   ┌────────────────────┐
│  路径 A: 用户主动   │   │  路径 B: 管理员     │   │  路径 C: 未修改    │
│  修改密码           │   │  强制重置           │   │  等待过期          │
│                    │   │                    │   │                    │
│ ChangePassword     │   │ SetPassword        │   │ DaysRemaining <= 0 │
│ → 复杂度校验       │   │ → 跳过旧密码验证   │   │                    │
│ → 历史复用检测     │   │ → 复杂度校验       │   │ VerifyPassword     │
│ → 旧密码移至历史   │   │ → 当前移至历史     │   │ → ErrPasswordExpired│
│ → MustChange=false │   │ → MustChange=false │   │   (禁止登录)       │
└────────┬───────────┘   └─────────┬──────────┘   └────────┬───────────┘
         │                        │                        │
         └────────────────────┐   │   ┌────────────────────┘
                              ▼   ▼   ▼
                       ┌───────────────────────────────────┐
                       │                                   │
                       │       回到阶段 2: 正常使用期       │
                       │   (路径 C 需先完成修改才能恢复)    │
                       │                                   │
                       └───────────────────────────────────┘
```

### 8.2 关键事件节点

| 事件 | 触发方式 | 对状态的影响 |
|------|---------|-------------|
| 密码创建 | SetPassword | 初始化 Current，清空 MustChange，History 追加旧值（如果有） |
| 用户修改密码 | ChangePassword | 验证旧密码，检查历史复用，更新 Current，追加 History |
| 密码过期 | 时间流逝自动触发 | VerifyPassword 返回 ErrPasswordExpired |
| 强制重置 | ForcePasswordChange | MustChange=true，下次 VerifyPassword 返回错误 |
| 管理员重置 | SetPassword | 无需旧密码，直接覆盖，History 追加，MustChange=false |
| Cost 升级 | UpdateBcryptCost 配置变更后首次 VerifyPassword | 透明更新 Hash 和 Cost，用户无感知 |
| 用户删除 | DeleteUser | 清除该用户所有密码状态和历史记录 |

---

## 9. 使用示例

### 9.1 基本使用：初始化引擎 + 用户登录流程

```go
package main

import (
    "errors"
    "fmt"
    "solocoder-go/internal/passpolicy"
)

func main() {
    // 使用默认配置初始化
    engine, err := passpolicy.NewEngine()
    if err != nil {
        panic(err)
    }

    // 1. 管理员为新用户设置密码
    err = engine.SetPassword("alice", "SecurePass123!")
    if err != nil {
        fmt.Printf("设置密码失败: %v\n", err)
        return
    }

    // 2. 用户登录验证
    result, err := engine.VerifyPassword("alice", "SecurePass123!")
    if err != nil {
        if errors.Is(err, passpolicy.ErrPasswordExpired) {
            fmt.Println("密码已过期，请修改后重新登录")
        } else {
            fmt.Printf("登录失败: %v\n", err)
        }
        return
    }
    fmt.Println("登录成功！")

    // 3. 查看密码状态
    status, _ := engine.GetPasswordStatus("alice")
    fmt.Printf("密码剩余有效天数: %d\n", status.DaysRemaining)
    if status.IsWarningPeriod {
        fmt.Printf("警告: 密码将在 %d 天后过期，请尽快修改\n", status.DaysRemaining)
    }

    // 4. 检查是否触发了自动哈希升级
    if result.Rehashed {
        fmt.Printf("密码已自动升级到 cost=%d，建议同步更新持久化存储\n", result.NewCost)
    }
}
```

### 9.2 自定义配置：企业级严格策略

```go
package main

import (
    "solocoder-go/internal/passpolicy"
)

func main() {
    cfg := passpolicy.Config{
        MinLength:    14,                   // 至少 14 位
        BcryptCost:   14,                   // 高强度哈希（约 1 秒/次）
        HistoryDepth: 10,                   // 禁止重复使用最近 10 个密码
        ExpiryDays:   60,                   // 每 60 天更换
        WarningDays:  14,                   // 提前 14 天提醒
        Complexity: passpolicy.ComplexityConfig{
            RequireUppercase: true,
            RequireLowercase: true,
            RequireDigit:     true,
            RequireSpecial:   true,
        },
    }

    engine, err := passpolicy.NewEngineWithConfig(cfg)
    if err != nil {
        panic(err)
    }

    // 使用 engine...
    _ = engine
}
```

### 9.3 用户修改密码（含历史复用检查）

```go
package main

import (
    "errors"
    "fmt"
    "solocoder-go/internal/passpolicy"
)

func handleChangePassword(engine *passpolicy.Engine, userID, oldPw, newPw string) {
    // 先对新密码进行预览校验，给出友好提示
    vr := engine.ValidatePassword(newPw)
    if !vr.Valid {
        fmt.Println("密码不符合以下要求:")
        for _, msg := range vr.ErrorMessages() {
            fmt.Printf("  - %s\n", msg)
        }
        return
    }

    // 执行修改
    err := engine.ChangePassword(userID, oldPw, newPw)
    switch {
    case err == nil:
        fmt.Println("密码修改成功！")

    case errors.Is(err, passpolicy.ErrPasswordMismatch):
        fmt.Println("原密码错误，请重新输入")

    case errors.Is(err, passpolicy.ErrPasswordHistoryReused):
        fmt.Println("该密码在最近使用过，请选择一个全新的密码")

    default:
        fmt.Printf("修改失败: %v\n", err)
    }
}
```

### 9.4 动态升级 bcrypt 强度

```go
package main

import (
    "fmt"
    "solocoder-go/internal/passpolicy"
)

func main() {
    // 初始使用较低 cost（便于快速开发/测试）
    engine, _ := passpolicy.NewEngine()
    engine.SetPassword("user1", "TestPass1!") // 使用 DefaultCost=10

    // 上线后，安全团队要求提升强度
    err := engine.UpdateBcryptCost(12) // 成本增加 4 倍
    if err != nil {
        panic(err)
    }
    fmt.Println("全局 bcrypt cost 已提升至 12")

    // 用户首次登录时自动升级
    result, _ := engine.VerifyPassword("user1", "TestPass1!")
    if result.Rehashed {
        fmt.Printf("用户 user1 的密码已自动升级: cost %d → %d\n",
            10, result.NewCost)
    }

    // 后续登录不再重复升级
    result2, _ := engine.VerifyPassword("user1", "TestPass1!")
    fmt.Printf("再次登录是否触发重哈希: %v (应为 false)\n", result2.Rehashed)
}
```

### 9.5 密码过期批量扫描与提醒

```go
package main

import (
    "fmt"
    "solocoder-go/internal/passpolicy"
)

func scanExpiringPasswords(engine *passpolicy.Engine, userIDs []string) {
    fmt.Println("=== 密码过期状态报告 ===")
    for _, uid := range userIDs {
        status, err := engine.GetPasswordStatus(uid)
        if err != nil {
            continue
        }

        switch {
        case status.IsExpired:
            fmt.Printf("[已过期] 用户 %s: 已过期 %d 天，立即锁定账号\n",
                uid, -status.DaysRemaining)

        case status.IsWarningPeriod:
            fmt.Printf("[临近过期] 用户 %s: 还剩 %d 天，发送提醒邮件\n",
                uid, status.DaysRemaining)

        case status.MustChange:
            fmt.Printf("[强制修改] 用户 %s: 管理员要求重置密码\n", uid)
        }
    }
}
```

### 9.6 测试中模拟时间

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/passpolicy"
)

func main() {
    engine, _ := passpolicy.NewEngine()

    // 使用固定时间，确保测试可重复
    baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
    engine.SetNowFunc(func() time.Time { return baseTime })

    engine.SetPassword("user1", "TestPass1!")

    // 模拟 91 天后（超过默认 90 天有效期）
    futureTime := baseTime.Add(91 * 24 * time.Hour)
    engine.SetNowFunc(func() time.Time { return futureTime })

    _, err := engine.VerifyPassword("user1", "TestPass1!")
    if err == passpolicy.ErrPasswordExpired {
        fmt.Println("正确：密码已过期")
    }
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyUserID` | 用户 ID 为空 | 所有接收 userID 参数的方法 |
| `ErrEmptyPassword` | 密码为空 | SetPassword/ChangePassword/VerifyPassword |
| `ErrUserNotFound` | 用户不存在 | 查询或修改不存在的用户 |
| `ErrPasswordExpired` | 密码已过期或需强制修改 | VerifyPassword 时密码过期或 MustChange=true |
| `ErrPasswordHistoryReused` | 密码与近期重复 | ChangePassword 时命中当前或历史密码 |
| `ErrInvalidBcryptCost` | bcrypt cost 超出有效范围 | 创建引擎或 UpdateBcryptCost 时 cost 不在 [4,31] |
| `ErrInvalidMinLength` | 最小长度非法 | MinLength < 1 |
| `ErrInvalidHistoryDepth` | 历史深度非法 | HistoryDepth < 0 |
| `ErrInvalidExpiryDays` | 过期天数非法 | ExpiryDays < 0 |
| `ErrInvalidWarningDays` | 警告天数非法 | WarningDays < 0 |
| `ErrPasswordMismatch` | 密码不匹配 | ChangePassword 旧密码错、VerifyPassword 密码错 |
| `ErrPasswordTooShort` | 密码长度不足 | 复杂度校验时 len(password) < MinLength |
| `ErrPasswordMissingUppercase` | 缺少大写字母 | 开启规则但密码无大写 |
| `ErrPasswordMissingLowercase` | 缺少小写字母 | 开启规则但密码无小写 |
| `ErrPasswordMissingDigit` | 缺少数字 | 开启规则但密码无数字 |
| `ErrPasswordMissingSpecial` | 缺少特殊字符 | 开启规则但密码无特殊字符 |

---

## 11. 并发安全

模块完全支持高并发访问，所有共享状态访问均受 `sync.RWMutex` 保护：

| 操作类型 | 锁类型 | 说明 |
|---------|-------|------|
| SetPassword | 写锁 | 修改用户状态（新增/更新） |
| ChangePassword | 先读后写 | 读取历史用读锁，更新时升级为写锁 |
| VerifyPassword | 先读后写 | 校验用读锁，触发 rehash 时用写锁 |
| GetPasswordStatus | 读锁 | 只读查询 |
| ValidatePassword | 无锁 | 纯函数，不访问共享状态 |
| ForcePasswordChange | 写锁 | 修改 MustChange 标记 |
| DeleteUser | 写锁 | 删除用户数据 |
| UpdateBcryptCost | 写锁 | 修改全局配置 |
| GetConfig/GetPasswordHash/GetHistoryEntries/UserCount | 读锁 | 只读查询 |
| SetNowFunc | 写锁 | 修改时间函数指针 |

**性能建议**:
- ValidatePassword 是无状态纯函数，可无限并发
- bcrypt 哈希计算本身是 CPU 密集型，大并发修改密码场景需注意 CPU 上限
- HistoryDepth 过大时，ChangePassword 的历史遍历可能延长锁持有时间

---

## 12. 配置参数与默认值

| 参数 | 默认值 | 最小值 | 最大值 | 说明 |
|------|-------|-------|-------|------|
| `MinLength` | 8 | 1 | 无 | 密码允许的最短长度 |
| `BcryptCost` | 10 (bcrypt.DefaultCost) | 4 | 31 | bcrypt 迭代成本，cost=N 表示 2^N 次 |
| `HistoryDepth` | 5 | 0 | 无 | 禁止复用的历史密码数量，0 表示不限制 |
| `ExpiryDays` | 90 | 0 | 无 | 密码有效期（天），0 表示永不过期 |
| `WarningDays` | 7 | 0 | 无 | 提前进入警告期的天数阈值 |
| `Complexity.RequireUppercase` | true | - | - | 是否必须包含大写字母 |
| `Complexity.RequireLowercase` | true | - | - | 是否必须包含小写字母 |
| `Complexity.RequireDigit` | true | - | - | 是否必须包含数字 |
| `Complexity.RequireSpecial` | true | - | - | 是否必须包含特殊字符 |

### bcrypt Cost 参考值

| Cost | 近似耗时（现代 CPU） | 适用场景 |
|------|-------------------|---------|
| 4 | ~1 ms | 仅测试使用，绝对不要在生产使用 |
| 10 | ~70 ms | 默认值，平衡性能与安全 |
| 12 | ~250 ms | 高安全系统，可接受稍长登录时间 |
| 14 | ~1000 ms | 极高安全要求，用户量不大的场景 |
| ≥ 15 | ≥ 4 s | 不推荐，严重影响用户体验 |

> 建议：目标是让每次密码验证耗时在 100ms - 500ms 之间，既能抵御暴力破解，又不影响正常登录体验。
