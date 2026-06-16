# API 密钥管理器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [密钥生命周期](#4-密钥生命周期)
5. [权限范围模型](#5-权限范围模型)
6. [使用次数限制机制](#6-使用次数限制机制)
7. [过期时间管理](#7-过期时间管理)
8. [紧急吊销机制](#8-紧急吊销机制)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [并发安全设计](#11-并发安全设计)

---

## 1. 模块概述

API 密钥管理器（APIKey Manager）是一个用于管理 API 访问密钥的安全模块，提供密钥的安全生成、权限绑定、使用计数、过期控制和紧急吊销等完整生命周期管理功能。模块采用安全随机数生成密钥，密钥明文仅在创建时返回一次，后续仅存储 SHA-256 哈希值用于校验，确保密钥安全性。

**包路径**: `internal/apikey`

**设计目标**:
- 安全的密钥生成与存储（仅存哈希值，不明文存储）
- 细粒度的权限范围控制（资源+操作级）
- 精确的使用次数计数（并发安全）
- 灵活的过期时间设置（绝对时间 + 相对时长）
- 不可逆的紧急吊销机制
- 完整的生命周期状态追踪

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 密钥生成 | 使用 `crypto/rand` 安全随机生成 32 字节密钥，格式为 `sk_<hex>`，生成唯一 Key ID，返回明文密钥一次 |
| 权限绑定 | 创建密钥时可绑定多个 `资源:操作` 形式的权限，支持细粒度访问控制 |
| 使用次数限制 | 设置最大使用次数上限，每次认证成功计数器原子递增，达到上限自动失效 |
| 过期时间设置 | 支持绝对时间戳（ExpiresAt）和相对时长（TTL）两种方式设置有效期 |
| 密钥校验 | 通过密钥哈希值验证密钥有效性，同时检查吊销、过期、使用次数等状态 |
| 访问控制 | 校验密钥是否拥有执行特定操作的权限 |
| 前缀查询 | 通过密钥前缀匹配查询密钥元信息（无法获取明文） |
| 紧急吊销 | 提供不可逆的吊销接口，标记密钥为已吊销并记录时间与原因 |
| 元信息查询 | 查询密钥的完整元信息（状态、权限、剩余次数、剩余时间等） |
| 到期更新 | 支持重新设置过期时间或 TTL，延长密钥有效期 |

---

## 3. 核心结构体与职责

### 3.1 Manager

密钥管理器主结构体，对外提供所有操作接口。

```go
type Manager struct {
    mu       sync.RWMutex
    keys     map[string]*APIKey
    byPrefix map[string][]string
}
```

**职责**:
- 管理所有密钥的存储与索引
- 提供密钥的创建、查询、吊销入口
- 通过 `byPrefix` 前缀索引加速前缀匹配查询
- 通过 RWMutex 保证并发访问安全

**主要方法**:
- `NewManager()` - 创建新的密钥管理器
- `CreateKey(opts)` - 创建新密钥
- `GetKeyMeta(id)` - 通过 Key ID 查询元信息
- `ListKeysByPrefix(prefix)` - 通过前缀批量查询
- `ListAllKeys()` - 列出所有密钥
- `RevokeKey(id, reason)` - 紧急吊销
- `VerifyKey(secret)` - 验证密钥有效性
- `CheckAccess(id, permission)` - 检查访问权限
- `VerifyAndCheckAccess(secret, permission)` - 组合验证和权限检查
- `IncrementUsage(id)` - 手动增加使用计数
- `GetRemainingUses(id)` - 查询剩余使用次数
- `GetRemainingTime(id)` - 查询剩余有效时间
- `SetExpiresAt(id, time)` - 设置绝对过期时间
- `SetTTL(id, duration)` - 设置相对过期时长
- `KeyCount()` - 获取密钥总数

### 3.2 APIKey

单个密钥的内部表示，包含密钥状态和所有元数据。

```go
type APIKey struct {
    ID           string
    Prefix       string
    SecretHash   string
    Name         string
    Description  string
    MaxUses      int64
    UsedCount    atomic.Int64
    CreatedAt    time.Time
    revoked      atomic.Bool
    hasExp       atomic.Bool
    stateMu      sync.Mutex
    state        keyState
}
```

**职责**:
- 存储密钥的唯一标识和前缀
- 存储密钥的 SHA-256 哈希值（不存明文）
- 通过 `atomic.Bool` 无锁读取吊销和过期标志
- 通过 `atomic.Int64` 保证使用计数并发安全
- 通过 `keyState` 封装可变状态字段，由 Mutex 保护

### 3.3 keyState

密钥的可变内部状态，由 Mutex 保护以避免竞态。

```go
type keyState struct {
    expiresAt     time.Time
    hasExpiration bool
    revokedAt     time.Time
    revokeReason  string
    permissions   map[Permission]bool
    lastUsedAt    time.Time
}
```

**职责**:
- 封装所有可能被并发修改的可变字段
- 存储过期时间配置
- 存储吊销信息（时间、原因）
- 存储权限集合（使用 map 实现 O(1) 查询）
- 记录最后使用时间

### 3.4 Permission

权限定义，采用资源+操作的二元组模型。

```go
type Permission struct {
    Resource string
    Action   string
}
```

**职责**:
- 定义细粒度的访问控制单元
- `Resource`: 被访问的资源类型（如 `users`, `articles`, `orders`）
- `Action`: 对资源执行的操作（如 `read`, `write`, `delete`）
- 字符串表示为 `resource:action`

### 3.5 APIKeyMeta

密钥元信息的只读视图，用于外部查询。

```go
type APIKeyMeta struct {
    ID            string
    Prefix        string
    Name          string
    Description   string
    Status        KeyStatus
    Permissions   []Permission
    MaxUses       int64
    UsedCount     int64
    RemainingUses int64
    CreatedAt     time.Time
    ExpiresAt     time.Time
    HasExpiration bool
    Revoked       bool
    RevokedAt     time.Time
    RevokeReason  string
    LastUsedAt    time.Time
}
```

**职责**:
- 提供密钥状态的只读快照
- 包含计算字段（Status、RemainingUses）
- 不包含任何可写引用，防止外部修改内部状态
- 权限列表按字典序排序，输出稳定

### 3.6 CreatedKey

密钥创建时的返回值，唯一一次返回密钥明文。

```go
type CreatedKey struct {
    ID     string
    Prefix string
    Secret string
}
```

**职责**:
- 作为 `CreateKey` 的返回值
- 包含密钥唯一 ID、前缀和完整明文密钥
- **重要**: 此结构体中的 `Secret` 仅在此刻可用，后续无法通过任何接口获取

### 3.7 CreateKeyOptions

创建密钥时的配置选项。

```go
type CreateKeyOptions struct {
    Name          string
    Description   string
    Permissions   []Permission
    MaxUses       int64
    ExpiresAt     time.Time
    TTL           time.Duration
    HasExpiration bool
}
```

**字段说明**:
- `Name`: 密钥名称，用于人类识别
- `Description`: 密钥描述信息
- `Permissions`: 权限列表，默认为空（无任何权限）
- `MaxUses`: 最大使用次数，`0` 表示无限次
- `ExpiresAt`: 绝对过期时间，与 `TTL` 互斥，优先级低于 TTL
- `TTL`: 相对过期时长（从创建时计算），优先级高于 ExpiresAt
- `HasExpiration`: 是否启用过期，当 TTL 或 ExpiresAt 被设置时自动为 true

### 3.8 KeyStatus

密钥状态枚举。

```go
type KeyStatus string

const (
    StatusActive   KeyStatus = "active"
    StatusExpired  KeyStatus = "expired"
    StatusRevoked  KeyStatus = "revoked"
    StatusDepleted KeyStatus = "depleted"
)
```

**状态优先级（从高到低）**:
1. `StatusRevoked` - 已吊销（优先级最高，一旦吊销即为最终状态）
2. `StatusExpired` - 已过期
3. `StatusDepleted` - 使用次数耗尽
4. `StatusActive` - 正常可用

### 3.9 VerifyResult

密钥验证结果。

```go
type VerifyResult struct {
    Valid   bool
    KeyMeta *APIKeyMeta
    Reason  error
}
```

**字段说明**:
- `Valid`: 是否验证通过
- `KeyMeta`: 验证通过时返回密钥元信息；验证失败时，若密钥存在也会返回（如已吊销的密钥）
- `Reason`: 验证失败的具体原因

### 3.10 CheckAccessResult

权限检查结果。

```go
type CheckAccessResult struct {
    Allowed bool
    Reason  error
}
```

**字段说明**:
- `Allowed`: 是否允许访问
- `Reason`: 拒绝访问的具体原因

---

## 4. 密钥生命周期

### 4.1 完整生命周期图

```
                    ┌─────────────────────────────────────┐
                    │                                     │
                    │         创建密钥 (CreateKey)        │
                    │                                     │
                    │  • 生成唯一 Key ID (32 字符 hex)    │
                    │  • 生成 32 字节安全随机密钥         │
                    │  • 计算 SHA-256 哈希并存储          │
                    │  • 返回明文密钥一次（仅此一次）      │
                    │  • 绑定权限、次数、过期配置         │
                    │                                     │
                    └───────────────────┬─────────────────┘
                                        │
                                        ▼
                    ┌─────────────────────────────────────┐
                    │                                     │
                    │         状态: Active (活跃)         │
                    │                                     │
                    │  可用操作:                         │
                    │  • VerifyKey() - 验证并计数+1       │
                    │  • CheckAccess() - 权限检查         │
                    │  • IncrementUsage() - 手动计数+1    │
                    │  • GetRemainingUses() - 查询剩余    │
                    │  • GetRemainingTime() - 查询时间    │
                    │  • SetExpiresAt()/SetTTL() - 更新  │
                    │  • RevokeKey() - 紧急吊销           │
                    │                                     │
                    └───────┬──────────┬──────────┬──────┘
                            │          │          │
              使用次数耗尽   │          │          │   到达过期时间
              StatusDepleted│          │          │   StatusExpired
                            │          │          │
                            ▼          ▼          ▼
                    ┌─────────────────────────────────────┐
                    │                                     │
                    │       终止状态 (不可逆)             │
                    │                                     │
                    │  • 吊销 (Revoked)                   │
                    │    - 主动操作，立即可逆              │
                    │    - 记录时间和原因                  │
                    │    - 所有校验均视为无效              │
                    │                                     │
                    │  • 过期 (Expired)                   │
                    │    - 到达 ExpiresAt 时间点           │
                    │    - 可通过 SetTTL/SetExpiresAt 恢复 │
                    │                                     │
                    │  • 耗尽 (Depleted)                  │
                    │    - 达到 MaxUses 上限              │
                    │    - 计数器到达阈值                  │
                    │                                     │
                    └─────────────────────────────────────┘
```

### 4.2 各阶段详细说明

#### 阶段 1: 创建密钥

**触发方式**: 调用 `Manager.CreateKey(opts)`

**执行步骤**:
1. 生成唯一 Key ID
   - 使用 `crypto/rand` 生成 16 字节随机数
   - 编码为 32 字符十六进制字符串
   - 检查唯一性，最多重试 100 次

2. 生成密钥明文
   - 使用 `crypto/rand` 生成 32 字节随机数
   - 加上 `sk_` 前缀形成完整密钥（共 68 字符）
   - 取前 11 字符（`sk_` + 8 字符）作为密钥前缀

3. 计算密钥哈希
   - 对完整密钥计算 SHA-256 哈希
   - 编码为 64 字符十六进制字符串存储
   - **明文密钥仅在返回值中出现一次**

4. 应用配置选项
   - 绑定权限集合（存入 map 便于 O(1) 查询）
   - 设置使用次数上限（0 表示无限）
   - 计算过期时间（TTL 优先于 ExpiresAt）
   - 保存名称和描述

5. 建立索引
   - 按 Key ID 存入 `keys` map
   - 按前缀存入 `byPrefix` 倒排索引

**返回值**: `CreatedKey{ID, Prefix, Secret}`，其中 Secret 是唯一一次获取明文的机会

#### 阶段 2: 活跃使用

密钥处于 `StatusActive` 状态时，可进行以下操作：

**密钥验证流程 (VerifyKey)**:
```
VerifyKey(secret)
    │
    ├─ 1. 计算 secret 的 SHA-256 哈希
    │
    ├─ 2. 通过前缀索引快速定位候选密钥
    │   （若前缀匹配失败则遍历全部密钥）
    │
    ├─ 3. 匹配哈希值，找到对应密钥
    │   ├─ 未找到 → 返回 ErrInvalidSecret
    │   └─ 找到 → 继续校验
    │
    ├─ 4. 检查状态标志
    │   ├─ revoked=true → 返回 ErrKeyRevoked
    │   ├─ hasExp=true + Now()>ExpiresAt → 返回 ErrKeyExpired
    │   └─ MaxUses>0 + UsedCount>=MaxUses → 返回 ErrUsageLimitExceeded
    │
    ├─ 5. 原子增加使用计数（CAS 循环）
    │   在 MaxUses 限制下，使用 CompareAndSwap 保证精确计数
    │
    ├─ 6. 更新最后使用时间
    │
    └─ 7. 返回 Valid=true + 元信息
```

**权限检查流程 (CheckAccess)**:
```
CheckAccess(id, permission)
    │
    ├─ 1. 通过 Key ID 查找密钥
    │   └─ 未找到 → ErrKeyNotFound
    │
    ├─ 2. 检查密钥状态（同 VerifyKey 步骤 4）
    │
    └─ 3. 检查权限集合
        ├─ 密钥权限 map 中存在该权限 → Allowed=true
        └─ 不存在 → ErrPermissionDenied + permission 字符串
```

**计数管理**:
- `IncrementUsage(id)` - 手动增加使用次数，带状态校验和 CAS 精确计数
- `GetRemainingUses(id)` - 查询剩余次数（`MaxUses - UsedCount`，无限次返回 -1）

**有效期管理**:
- `SetExpiresAt(id, time)` - 设置为某个绝对时间点过期
- `SetTTL(id, duration)` - 设置从现在起的相对时长（duration=0 取消过期）
- `GetRemainingTime(id)` - 查询剩余时间（无过期返回 has=false）

#### 阶段 3: 终止状态

**紧急吊销 (RevokeKey)**:
- 操作不可逆（与过期和耗尽不同）
- 使用 `CompareAndSwap` 保证只有第一个吊销请求成功
- 记录吊销时间和原因（必须提供原因）
- 吊销后所有校验均立即失败，返回 `ErrKeyRevoked`
- 即使延长过期时间也无法恢复

**过期 (Expired)**:
- 到达 `ExpiresAt` 时间后，验证时返回 `ErrKeyExpired`
- 可通过 `SetExpiresAt` 或 `SetTTL` 重新设置有效期，恢复 Active 状态
- 但如果密钥已被吊销，延长有效期无效

**耗尽 (Depleted)**:
- `UsedCount` 达到 `MaxUses` 后，验证时返回 `ErrUsageLimitExceeded`
- 耗尽状态目前不可恢复（MaxUses 创建后不可修改）

---

## 5. 权限范围模型

### 5.1 权限定义格式

权限采用二元组 `(Resource, Action)` 模型：

```
Permission{Resource: "users", Action: "read"}
// 字符串表示: "users:read"
```

### 5.2 解析函数

```go
func ParsePermission(s string) (Permission, error)
```

- 输入格式: `"resource:action"`
- 使用 `SplitN(s, ":", 2)` 分割，只分割第一个冒号
- 因此 action 可以包含冒号（如 `"gateway:route:add"` 解析为 resource=`gateway`, action=`route:add`）
- resource 或 action 为空均返回 `ErrInvalidPermission`

### 5.3 权限校验逻辑

权限检查遵循"显式授权"原则：
- 密钥必须在创建时显式绑定所有需要的权限
- 未绑定的权限一律拒绝访问
- 空权限集合 = 无任何权限（所有访问被拒绝）
- 权限匹配为精确匹配，不支持通配符（如需通配符可在上层实现）

### 5.4 常见权限设计模式

| 场景 | 权限示例 |
|------|----------|
| 只读用户 | `users:read` |
| 用户管理（读写） | `users:read`, `users:write`, `users:delete` |
| 文章只读 | `articles:read` |
| 文章发布 | `articles:read`, `articles:write` |
| 管理员（全部） | `<resource>:<action>` 所有组合 |

---

## 6. 使用次数限制机制

### 6.1 计数原理

- `UsedCount` 使用 `atomic.Int64` 存储，保证原子读取
- 有限次使用时（`MaxUses > 0`），采用 **CompareAndSwap (CAS)** 循环递增：
  ```go
  for {
      used := key.UsedCount.Load()
      if used >= key.MaxUses { return ErrUsageLimitExceeded }
      if key.UsedCount.CompareAndSwap(used, used+1) { break }
  }
  ```
- 无限次使用时（`MaxUses == 0`），直接 `Add(1)` 递增，无需检查

### 6.2 并发精确性保证

CAS 循环确保即使在高并发下：
- 实际使用次数不会超过 `MaxUses`
- 每次成功验证对应精确的 +1 计数
- 在 150 并发请求、`MaxUses=100` 场景下：
  - 恰好 100 次成功，50 次返回 `ErrUsageLimitExceeded`
  - `UsedCount` 精确等于 100，不多不少

### 6.3 剩余次数计算

```go
func (k *APIKey) RemainingUses() int64 {
    if k.MaxUses <= 0 { return -1 }  // -1 表示无限
    used := k.UsedCount.Load()
    remaining := k.MaxUses - used
    if remaining < 0 { return 0 }    // 防止负数
    return remaining
}
```

---

## 7. 过期时间管理

### 7.1 两种设置方式

#### 方式一: 相对时长 TTL（推荐）

```go
opts := CreateKeyOptions{
    TTL: 30 * 24 * time.Hour,  // 30 天有效期
}
```

- 从密钥创建时刻起计算：`ExpiresAt = Now() + TTL`
- 适用于大多数场景，语义清晰

#### 方式二: 绝对时间戳 ExpiresAt

```go
opts := CreateKeyOptions{
    ExpiresAt:     time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC),
    HasExpiration: true,
}
```

- 指定具体的过期时间点
- 适用于有明确截止日期的场景（如年度授权）

### 7.2 优先级规则

当 TTL 和 ExpiresAt 同时设置时：
1. `TTL > 0` → 使用 TTL 计算（优先级高）
2. `ExpiresAt` 非零 + `HasExpiration=true` → 使用 ExpiresAt
3. 都不设置 → 永不过期

### 7.3 有效期更新

已创建的密钥可延长有效期：

```go
// 延长到某个时间点
manager.SetExpiresAt(keyID, time.Now().Add(90*24*time.Hour))

// 延长相对时长
manager.SetTTL(keyID, 90*24*time.Hour)

// 取消过期（永久有效）
manager.SetTTL(keyID, 0)
```

**注意**: 已吊销的密钥无法通过更新有效期恢复。

---

## 8. 紧急吊销机制

### 8.1 吊销触发场景

- 密钥明文意外泄露（日志泄露、代码提交到公开仓库等）
- 关联的客户端不再可信
- 安全事件响应（入侵检测后批量吊销）
- 员工离职时撤销其个人密钥

### 8.2 吊销特性

- **不可逆**: 一旦吊销，永久失效，不能恢复为 Active
- **原子性**: 使用 `CompareAndSwap(false, true)`，并发下仅第一个请求成功
- **强制生效**: 吊销后所有验证立即返回 `ErrKeyRevoked`，包括后续的权限检查
- **可审计**: 必须提供吊销原因，记录吊销时间戳

### 8.3 双吊销保护

```go
if !key.revoked.CompareAndSwap(false, true) {
    return ErrAlreadyRevoked
}
```

- 第一次吊销: CAS 成功，执行吊销
- 重复吊销: CAS 失败，返回 `ErrAlreadyRevoked`
- 吊销原因不会被覆盖（始终保留第一次的原因）

---

## 9. 使用示例

### 9.1 基本使用：创建和验证密钥

```go
package main

import (
    "fmt"
    "solocoder-go/internal/apikey"
)

func main() {
    manager := apikey.NewManager()

    // 创建密钥
    created, err := manager.CreateKey(apikey.CreateKeyOptions{
        Name:        "My Application Key",
        Description: "用于主应用的 API 访问",
        Permissions: []apikey.Permission{
            {Resource: "users", Action: "read"},
            {Resource: "users", Action: "write"},
            {Resource: "articles", Action: "read"},
        },
        MaxUses: 10000,                     // 最多使用 10000 次
        TTL:     30 * 24 * time.Hour,       // 30 天有效期
    })
    if err != nil {
        panic(err)
    }

    // 保存密钥明文（仅此一次机会）
    fmt.Printf("密钥创建成功！\n")
    fmt.Printf("  Key ID: %s\n", created.ID)
    fmt.Printf("  Prefix: %s\n", created.Prefix)
    fmt.Printf("  Secret: %s\n", created.Secret)  // 请妥善保存！
    fmt.Printf("  ⚠️  此密钥明文仅显示一次，请安全存储\n")

    // ... 之后在请求中验证密钥

    secretFromRequest := created.Secret  // 从请求头中获取

    // 验证密钥有效性
    result := manager.VerifyKey(secretFromRequest)
    if !result.Valid {
        fmt.Printf("密钥无效: %v\n", result.Reason)
        return
    }

    fmt.Printf("密钥有效！已使用 %d/%d 次\n",
        result.KeyMeta.UsedCount,
        result.KeyMeta.MaxUses)
}
```

### 9.2 权限校验

```go
// 在处理具体请求时
func handleReadUserRequest(manager *apikey.Manager, keyID string) error {
    // 检查是否有读取用户的权限
    access := manager.CheckAccess(keyID, apikey.Permission{
        Resource: "users",
        Action:   "read",
    })

    if !access.Allowed {
        return fmt.Errorf("访问被拒绝: %w", access.Reason)
    }

    // 继续处理请求...
    return nil
}

// 一步完成验证 + 权限检查
func handleRequest(manager *apikey.Manager, secret string) error {
    meta, access := manager.VerifyAndCheckAccess(secret, apikey.Permission{
        Resource: "articles",
        Action:   "publish",
    })

    if access.Allowed {
        fmt.Printf("用户 %s 有权限发布文章\n", meta.Name)
        // 处理业务逻辑...
    }

    return nil
}
```

### 9.3 查询密钥状态

```go
func printKeyStatus(manager *apikey.Manager, keyID string) {
    meta, err := manager.GetKeyMeta(keyID)
    if err != nil {
        fmt.Printf("查询失败: %v\n", err)
        return
    }

    fmt.Printf("密钥: %s\n", meta.Name)
    fmt.Printf("状态: %s\n", meta.Status)

    // 剩余使用次数
    remaining, _ := manager.GetRemainingUses(keyID)
    if remaining == -1 {
        fmt.Println("使用次数: 无限")
    } else {
        fmt.Printf("剩余次数: %d / %d\n", remaining, meta.MaxUses)
    }

    // 剩余时间
    timeLeft, hasExp, _ := manager.GetRemainingTime(keyID)
    if hasExp {
        fmt.Printf("剩余时间: %v (到期: %s)\n",
            timeLeft.Round(time.Minute),
            meta.ExpiresAt.Format("2006-01-02 15:04:05"))
    } else {
        fmt.Println("有效期: 永久")
    }

    // 权限列表
    fmt.Println("权限列表:")
    for _, p := range meta.Permissions {
        fmt.Printf("  - %s\n", p.String())
    }

    if meta.Revoked {
        fmt.Printf("已吊销: %s (原因: %s)\n",
            meta.RevokedAt.Format("2006-01-02 15:04:05"),
            meta.RevokeReason)
    }
}
```

### 9.4 通过前缀查询

```go
func findKeyByPrefix(manager *apikey.Manager, secret string) {
    // 用户只记得密钥前缀时，可以查询元信息
    prefix := secret[:11]  // "sk_" + 8 字符

    keys, err := manager.ListKeysByPrefix(prefix)
    if err != nil {
        panic(err)
    }

    fmt.Printf("找到 %d 个匹配前缀的密钥:\n", len(keys))
    for _, k := range keys {
        fmt.Printf("  - ID: %s, Name: %s, Status: %s\n",
            k.ID[:8]+"...", k.Name, k.Status)
    }
}
```

### 9.5 紧急吊销

```go
func emergencyRevoke(manager *apikey.Manager, keyID string) error {
    reason := "密钥泄露 - 发现于公开 GitHub 仓库 commit abc123"

    err := manager.RevokeKey(keyID, reason)
    if err != nil {
        if errors.Is(err, apikey.ErrAlreadyRevoked) {
            fmt.Println("密钥已处于吊销状态")
            return nil
        }
        return fmt.Errorf("吊销失败: %w", err)
    }

    fmt.Println("密钥已成功吊销！")

    // 验证吊销后确实不可用
    meta, _ := manager.GetKeyMeta(keyID)
    fmt.Printf("吊销时间: %s\n", meta.RevokedAt.Format("2006-01-02 15:04:05"))
    fmt.Printf("吊销原因: %s\n", meta.RevokeReason)
    fmt.Printf("当前状态: %s\n", meta.Status)  // "revoked"

    return nil
}
```

### 9.6 完整场景：限时一次性密钥

```go
func createOneTimeLink(manager *apikey.Manager) (string, error) {
    // 创建仅可用 1 次、有效期 10 分钟的密钥
    created, err := manager.CreateKey(apikey.CreateKeyOptions{
        Name:        "一次性下载链接",
        Description: "用于临时文件下载，使用一次后失效",
        Permissions: []apikey.Permission{
            {Resource: "files", Action: "download"},
        },
        MaxUses: 1,                      // 仅用 1 次
        TTL:     10 * time.Minute,       // 10 分钟内有效
    })
    if err != nil {
        return "", err
    }

    downloadURL := fmt.Sprintf("https://api.example.com/download?key=%s", created.Secret)
    return downloadURL, nil
}

func useOneTimeKey(manager *apikey.Manager, secret string) ([]byte, error) {
    // 第一次使用：成功
    result := manager.VerifyKey(secret)
    if !result.Valid {
        return nil, fmt.Errorf("密钥无效: %w", result.Reason)
    }

    // 检查权限
    access := manager.CheckAccess(result.KeyMeta.ID, apikey.Permission{
        Resource: "files", Action: "download",
    })
    if !access.Allowed {
        return nil, fmt.Errorf("无下载权限: %w", access.Reason)
    }

    // 执行下载...
    data := []byte("file contents")

    // 第二次使用：密钥已耗尽，失败
    result2 := manager.VerifyKey(secret)
    if result2.Valid {
        panic("一次性密钥不应可第二次使用")
    }
    fmt.Printf("第二次使用被正确拒绝: %v\n", result2.Reason)
    // Output: "apikey: usage limit exceeded"

    return data, nil
}
```

### 9.7 延长密钥有效期

```go
func extendKeyValidity(manager *apikey.Manager, keyID string) error {
    // 原密钥还有 3 天过期，延长到 90 天
    _, hasExp, _ := manager.GetRemainingTime(keyID)
    if hasExp {
        // 方式一：设置新 TTL
        err := manager.SetTTL(keyID, 90*24*time.Hour)
        if err != nil {
            if errors.Is(err, apikey.ErrKeyRevoked) {
                return fmt.Errorf("密钥已吊销，无法延长")
            }
            return err
        }
    }

    // 方式二：设置具体截止日期
    newExpiry := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
    err := manager.SetExpiresAt(keyID, newExpiry)
    if err != nil {
        return err
    }

    newLeft, _, _ := manager.GetRemainingTime(keyID)
    fmt.Printf("密钥已延期！新的剩余时间: %v\n", newLeft.Round(24*time.Hour))
    return nil
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrKeyNotFound` | 密钥不存在 | 通过 ID 查询时找不到对应密钥 |
| `ErrKeyExists` | 密钥已存在 | 创建时 ID 冲突（内部使用） |
| `ErrEmptyKeyID` | Key ID 为空 | 传入空字符串作为 Key ID |
| `ErrEmptyPrefix` | 前缀为空 | 前缀查询时前缀参数为空 |
| `ErrEmptySecret` | 密钥为空 | VerifyKey 传入空字符串 |
| `ErrEmptyResource` | 权限资源为空 | 创建/检查权限时 Resource 为空 |
| `ErrEmptyAction` | 权限操作为空 | 创建/检查权限时 Action 为空 |
| `ErrEmptyRevokeReason` | 吊销原因为空 | 吊销密钥时未提供原因 |
| `ErrKeyRevoked` | 密钥已吊销 | 对已吊销密钥执行验证/权限检查 |
| `ErrKeyExpired` | 密钥已过期 | 密钥到达过期时间 |
| `ErrUsageLimitExceeded` | 使用次数超限 | 达到 MaxUses 上限后继续使用 |
| `ErrInvalidPermission` | 权限格式无效 | ParsePermission 解析失败 |
| `ErrPermissionDenied` | 无操作权限 | 密钥缺少所需权限 |
| `ErrInvalidSecret` | 密钥无效 | 哈希不匹配，密钥不存在或篡改 |
| `ErrMaxUsesZeroOrNegative` | MaxUses 非法 | 创建时 MaxUses < 0 |
| `ErrExpiresAtInThePast` | 过期时间在过去 | 设置的 ExpiresAt 早于当前时间 |
| `ErrNegativeTTL` | TTL 为负 | TTL < 0 |
| `ErrAlreadyRevoked` | 密钥已被吊销 | 重复吊销同一密钥 |

---

## 11. 并发安全设计

### 11.1 分层锁设计

模块采用**分层混合锁**设计，在保证安全的前提下最大化并发性能：

| 层次 | 保护对象 | 同步机制 | 说明 |
|------|---------|---------|------|
| Manager 层 | `keys`, `byPrefix` | `sync.RWMutex` | 多读单写，创建/查询分离 |
| 密钥标志 | `revoked`, `hasExp` | `atomic.Bool` | 无锁原子读取，CAS 写入 |
| 密钥计数 | `UsedCount` | `atomic.Int64` | 无锁原子操作，CAS 精确计数 |
| 密钥状态 | `keyState` (权限、过期、吊销详情) | `sync.Mutex` | 简单互斥，复制读取 |

### 11.2 无锁热点路径

最频繁的验证操作 `VerifyKey` 中：
- **吊销检查**: `revoked.Load()` - 纯原子操作，无锁
- **过期检查**: `hasExp.Load()` - 纯原子操作，无锁
- **计数校验+递增**: `CompareAndSwap` 循环 - 原子操作，无锁
- 只有在更新 `lastUsedAt` 和读取权限时才短暂加锁

### 11.3 避免死锁

- 严格的锁获取顺序：Manager 锁 → 密钥锁（嵌套时外层先获取）
- **禁止**在持有密钥锁时请求 Manager 锁
- 所有锁持有时间尽可能短（只做拷贝/赋值，不做耗时操作）
- 使用简单 Mutex 替代 RWMutex 避免写优先导致的读饥饿

### 11.4 并发测试覆盖

测试套件包含以下并发场景：
- 1000 并发 `VerifyKey`（1000 次限制）：计数精确到 1000，0 错误
- 150 并发 `IncrementUsage`（100 次限制）：恰好 100 成功 + 50 超限
- 50 并发 `CreateKey`：所有创建成功，数量精确
- 混合读/写并发压力测试（200ms 持续运行）：无数据竞争、无死锁

---
