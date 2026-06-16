# JWT 令牌管理器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [令牌签发流程](#4-令牌签发流程)
5. [令牌校验流程](#5-令牌校验流程)
6. [黑名单吊销机制](#6-黑名单吊销机制)
7. [令牌续期机制](#7-令牌续期机制)
8. [刷新令牌轮换机制](#8-刷新令牌轮换机制)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [并发安全](#11-并发安全)

---

## 1. 模块概述

JWT 令牌管理器模块是一个功能完整的 JSON Web Token 管理系统，支持双算法签发、声明校验、黑名单吊销、令牌续期和刷新令牌轮换等核心功能。模块设计遵循 RFC 7519 规范，提供安全、可靠、高性能的 JWT 生命周期管理能力。

**包路径**: `internal/jwtmgr`

**设计目标**:
- 支持 HS256（对称密钥）和 RS256（非对称密钥）双算法签发
- 提供完整的令牌校验能力，包括签名、过期时间、签发者、受众等
- 支持基于内存的黑名单机制，可配置 TTL 自动清理
- 提供令牌续期能力，支持在可续期窗口内换取新令牌
- 实现刷新令牌轮换机制，防止刷新令牌被截获后长期滥用
- 完全符合 RFC 7519 规范

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 双算法签发 | 支持 HS256 对称密钥和 RS256 非对称密钥两种签名算法 |
| 标准声明支持 | 支持 iss（签发者）、sub（主题）、aud（受众）、exp（过期时间）、nbf（生效时间）、iat（签发时间）、jti（令牌ID）等标准声明 |
| 自定义声明 | 支持添加任意自定义声明，扩展令牌承载的信息 |
| 声明校验 | 校验令牌签名有效性、是否过期、签发者是否匹配、受众是否包含当前服务等 |
| 黑名单吊销 | 支持将令牌 jti 加入黑名单，已吊销令牌即使未过期也视为无效 |
| TTL 自动清理 | 黑名单支持按 TTL 自动清理过期记录，防止内存泄漏 |
| 令牌续期 | 令牌即将过期或刚过期但在可续期窗口内时，可换取新令牌 |
| 刷新令牌 | 签发访问令牌时可同时返回刷新令牌，用于获取新的访问令牌 |
| 刷新令牌轮换 | 使用刷新令牌时签发全新的刷新令牌并使旧令牌失效 |
| 可配置策略 | 支持配置是否自动将续期后的旧令牌加入黑名单 |

---

## 3. 核心结构体与职责

### 3.1 Manager

JWT 管理器主结构体，对外提供所有令牌管理操作接口。

```go
type Manager struct {
    config       Config
    signingKey   SigningKey
    blacklist    Blacklist
    refreshStore RefreshTokenStore
}
```

**职责**:
- 令牌的签发、验证、续期和吊销
- 协调签名密钥、黑名单和刷新令牌存储的交互
- 维护配置信息和默认值
- 管理资源的生命周期（Close 方法）

**核心方法**:

| 方法 | 说明 |
|------|------|
| `IssueToken(claims)` | 签发单个访问令牌 |
| `IssueTokenPair(claims)` | 同时签发访问令牌和刷新令牌 |
| `ValidateToken(token, opts)` | 校验令牌有效性和声明 |
| `RevokeToken(tokenID)` | 将令牌加入黑名单 |
| `RenewToken(token, opts)` | 在续期窗口内换取新令牌 |
| `RefreshAccessToken(refreshToken)` | 使用刷新令牌获取新令牌对 |
| `Close()` | 释放资源，停止清理协程 |
| `GetConfig()` | 获取当前配置 |

### 3.2 Config

配置结构体，用于定制模块行为。

```go
type Config struct {
    Issuer               string
    Audience             []string
    AccessTokenTTL       time.Duration
    RefreshTokenTTL      time.Duration
    RenewalWindow        time.Duration
    AutoBlacklistOld     bool
    BlacklistTTL         time.Duration
    BlacklistCleanupInt  time.Duration
    RefreshTokenRotation bool
}
```

**字段说明**:

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Issuer` | string | "jwtmgr" | 令牌签发者标识 |
| `Audience` | []string | ["api"] | 默认受众列表 |
| `AccessTokenTTL` | Duration | 1h | 访问令牌有效期 |
| `RefreshTokenTTL` | Duration | 168h (7d) | 刷新令牌有效期 |
| `RenewalWindow` | Duration | 5m | 续期窗口时长 |
| `AutoBlacklistOld` | bool | true | 续期后是否自动将旧令牌加入黑名单 |
| `BlacklistTTL` | Duration | 24h | 黑名单记录的默认 TTL |
| `BlacklistCleanupInt` | Duration | 1h | 黑名单清理协程的执行间隔 |
| `RefreshTokenRotation` | bool | true | 是否启用刷新令牌轮换 |

**默认配置** 由 [DefaultConfig](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/config.go#L20-L32) 函数返回。

### 3.3 SigningKey

签名密钥结构体，支持 HMAC 和 RSA 两种密钥类型。

```go
type SigningKey struct {
    Algorithm  Algorithm
    HMACKey    []byte
    PublicKey  *rsa.PublicKey
    PrivateKey *rsa.PrivateKey
}
```

**职责**:
- 封装签名算法和对应的密钥材料
- 支持 HS256 和 RS256 两种算法
- 提供工厂方法简化密钥创建

**工厂方法**:
- [NewHS256Config](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/config.go#L34-L38): 创建 HS256 签名密钥
- [NewRS256Config](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/config.go#L40-L46): 创建 RS256 签名密钥

### 3.4 Claims

JWT 声明结构体，包含标准声明和自定义声明。

```go
type Claims struct {
    Issuer    string                 `json:"iss"`
    Subject   string                 `json:"sub"`
    Audience  []string               `json:"aud"`
    ExpiresAt time.Time              `json:"exp"`
    NotBefore time.Time              `json:"nbf,omitempty"`
    IssuedAt  time.Time              `json:"iat"`
    ID        string                 `json:"jti"`
    Custom    map[string]interface{} `json:"-"`
}
```

**职责**:
- 存储 JWT 的所有声明信息
- 支持标准声明（iss, sub, aud, exp, nbf, iat, jti）
- 通过 Custom 字段支持任意自定义声明
- 实现自定义 JSON 序列化/反序列化，处理单值 audience 的兼容问题

**序列化约定**:
- `aud` 字段：单值时序列化为字符串，多值时序列化为数组，符合 RFC 7519 允许的两种格式
- `Custom` 字段中的键值对会合并到 JSON 顶层，与标准声明平级
- 零值字段（如未设置的 `nbf`）在序列化时省略

### 3.5 TokenPair

令牌对结构体，包含访问令牌和刷新令牌。

```go
type TokenPair struct {
    AccessToken  string
    RefreshToken string
    TokenID      string
    ExpiresAt    time.Time
}
```

**职责**:
- 封装签发的访问令牌和刷新令牌对
- 提供令牌 ID 和过期时间等元数据

### 3.6 ValidationOptions

校验选项结构体，用于灵活配置令牌校验行为。

```go
type ValidationOptions struct {
    ExpectedIssuer    string
    ExpectedAudience  []string
    ValidateExpiry    bool
    ValidateIssuer    bool
    ValidateAudience  bool
    ValidateNotBefore bool
}
```

**职责**:
- 配置校验时期望的签发者和受众
- 控制各项校验的开关状态
- 支持灵活的校验策略组合

**默认选项** 由 [DefaultValidationOptions](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/types.go#L148-L154) 函数返回，所有校验项默认启用。

### 3.7 Blacklist 接口

黑名单接口，定义令牌吊销存储的操作契约。

```go
type Blacklist interface {
    Add(tokenID string, ttl time.Duration) error
    Contains(tokenID string) (bool, error)
    Remove(tokenID string) error
    Close() error
}
```

**方法约定**:

| 方法 | 行为 |
|------|------|
| `Add` | 将 tokenID 添加到黑名单，设置 TTL；空 tokenID 返回 `ErrInvalidToken`；已关闭时返回 `ErrBlacklistClosed` |
| `Contains` | 检查 tokenID 是否在黑名单中且未过期；空 tokenID 返回 `(false, ErrInvalidToken)`，与 `Add` 行为一致 |
| `Remove` | 从黑名单中移除 tokenID |
| `Close` | 关闭黑名单，停止清理协程；支持多次调用不报错 |

**职责**:
- 定义黑名单存储的抽象接口
- 默认提供基于内存的实现 ([MemoryBlacklist](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/blacklist.go#L15-L110))
- 支持替换为其他存储实现（如 Redis、数据库等）

### 3.8 RefreshTokenStore 接口

刷新令牌存储接口，定义刷新令牌的管理契约。

```go
type RefreshTokenStore interface {
    Save(token *RefreshTokenInfo) error
    Get(token string) (*RefreshTokenInfo, error)
    Revoke(token string) error
    Close() error
}
```

**方法约定**:

| 方法 | 行为 |
|------|------|
| `Save` | 保存刷新令牌信息；nil 或空 token 返回 `ErrInvalidRefreshToken` |
| `Get` | 查询刷新令牌；空 token 返回 `ErrInvalidRefreshToken`；已吊销返回 `ErrRefreshTokenRevoked`；已过期返回 `ErrRefreshTokenExpired` |
| `Revoke` | 吊销刷新令牌；空或不存在的 token 返回 `ErrInvalidRefreshToken` |
| `Close` | 关闭存储 |

**职责**:
- 定义刷新令牌存储的抽象接口
- 默认提供基于内存的实现 ([MemoryRefreshStore](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/refresh_store.go#L15-L77))
- 支持替换为其他持久化存储实现

### 3.9 Algorithm 枚举

签名算法枚举类型。

```go
type Algorithm string

const (
    HS256 Algorithm = "HS256"
    RS256 Algorithm = "RS256"
)
```

**算法说明**:

| 算法 | 签名方式 | 密钥要求 | 适用场景 |
|------|---------|---------|---------|
| HS256 | HMAC-SHA256 对称签名 | 共享密钥（建议 ≥ 32 字节） | 单服务内部使用，密钥分发简单 |
| RS256 | RSA-SHA256 非对称签名 | RSA 私钥（≥ 2048 位）+ 公钥 | 跨服务场景，公钥可公开分发 |

---

## 4. 令牌签发流程

### 4.1 单访问令牌签发 ([IssueToken](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L97-L118))

签发单个访问令牌，适用于不需要刷新令牌的场景。

```
IssueToken(claims)
    │
    ├─→ 步骤 1：参数校验
    │       └─ claims 为 nil → 返回 ErrInvalidToken
    │
    ├─→ 步骤 2：设置默认值
    │       ├─ IssuedAt: 未设置则使用当前时间
    │       ├─ ExpiresAt: 未设置则使用当前时间 + AccessTokenTTL
    │       ├─ Issuer: 未设置则使用配置中的 Issuer
    │       ├─ Audience: 未设置则使用配置中的 Audience
    │       └─ ID (jti): 未设置则生成随机字符串
    │
    ├─→ 步骤 3：构建 JWT 头部
    │       └─ Header{Alg: <signing algorithm>, Typ: "JWT"}
    │
    ├─→ 步骤 4：序列化
    │       ├─ JSON 序列化头部
    │       ├─ JSON 序列化声明（包含自定义声明）
    │       └─ Base64URL 编码头部和声明
    │
    ├─→ 步骤 5：签名
    │       ├─ 拼接编码后的头部和声明作为签名输入
    │       ├─ 根据算法选择签名方式：
    │       │   ├─ HS256: HMAC-SHA256 对称签名
    │       │   └─ RS256: RSA-SHA256 非对称签名
    │       └─ Base64URL 编码签名
    │
    └─→ 步骤 6：组装 Token
            └─ 返回格式: <header>.<claims>.<signature>
```

**签发后的 Token 格式符合 RFC 7519 规范**:
- Header: `{"alg":"HS256","typ":"JWT"}` 或 `{"alg":"RS256","typ":"JWT"}`
- Claims: 包含 iss、sub、aud、exp、iat、jti 等标准声明以及自定义声明
- Signature: 对 `header.claims` 的签名

### 4.2 令牌对签发 ([IssueTokenPair](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L120-L162))

同时签发访问令牌和刷新令牌，适用于需要长期访问的场景。

```
IssueTokenPair(claims)
    │
    ├─→ 步骤 1：参数校验
    │       └─ claims 为 nil → 返回 ErrInvalidToken
    │
    ├─→ 步骤 2：签发访问令牌（同 IssueToken 流程）
    │
    ├─→ 步骤 3：生成刷新令牌
    │       └─ 生成 64 字节随机字符串（128 位十六进制）作为刷新令牌
    │
    ├─→ 步骤 4：存储刷新令牌信息
    │       ├─ Token: 刷新令牌字符串
    │       ├─ TokenID: 访问令牌的 jti
    │       ├─ Subject: 用户标识
    │       ├─ ExpiresAt: 当前时间 + RefreshTokenTTL
    │       ├─ CreatedAt: 当前时间
    │       └─ Claims: 原始声明（用于签发新令牌时恢复声明信息）
    │
    └─→ 步骤 5：返回 TokenPair
            ├─ AccessToken: 访问令牌
            ├─ RefreshToken: 刷新令牌
            ├─ TokenID: 令牌 ID
            └─ ExpiresAt: 访问令牌过期时间
```

---

## 5. 令牌校验流程 ([ValidateToken](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L196-L243))

完整的令牌校验流程，确保令牌的合法性和有效性。校验按顺序执行，任何一步失败即返回对应错误。

```
ValidateToken(token, options)
    │
    ├─→ 步骤 1：格式校验
    │       ├─ 检查 token 是否为空 → ErrEmptyToken
    │       └─ 按 '.' 分割，必须为 3 部分 → ErrInvalidToken
    │
    ├─→ 步骤 2：解码头部
    │       ├─ Base64URL 解码头部 → 失败返回 ErrInvalidToken
    │       ├─ JSON 反序列化为 Header → 失败返回 ErrInvalidToken
    │       └─ 校验算法是否与 Manager 配置匹配 → 不匹配返回 ErrInvalidAlgorithm
    │
    ├─→ 步骤 3：签名校验
    │       ├─ Base64URL 解码签名 → 失败返回 ErrInvalidToken
    │       ├─ 重新计算签名并比较：
    │       │   ├─ HS256: 使用 HMACKey 重新计算 HMAC-SHA256
    │       │   └─ RS256: 使用 PublicKey 验证（若为 nil 则从 PrivateKey 推导）
    │       └─ 签名不匹配 → ErrInvalidSignature
    │
    ├─→ 步骤 4：解码声明
    │       ├─ Base64URL 解码声明部分 → 失败返回 ErrInvalidToken
    │       └─ JSON 反序列化为 Claims → 失败返回 ErrInvalidToken
    │
    ├─→ 步骤 5：声明校验
    │       ├─ 过期时间校验 (exp):
    │       │   └─ ValidateExpiry=true 且已过期 → ErrExpiredToken
    │       ├─ 生效时间校验 (nbf):
    │       │   └─ ValidateNotBefore=true 且未生效 → ErrNotYetValid
    │       ├─ 签发者校验 (iss):
    │       │   └─ ValidateIssuer=true 且不匹配 ExpectedIssuer → ErrInvalidIssuer
    │       └─ 受众校验 (aud):
    │           └─ ValidateAudience=true 且不包含所有 ExpectedAudience → ErrInvalidAudience
    │
    └─→ 步骤 6：黑名单校验
            ├─ 调用 blacklist.Contains(claims.ID)
            │   ├─ tokenID 为空时 Contains 返回 (false, ErrInvalidToken)
            │   └─ 正常情况下返回是否存在及错误
            └─ 存在于黑名单 → ErrTokenBlacklisted
```

**校验选项说明**:

| 选项 | 默认值 | 说明 |
|------|--------|------|
| `ValidateExpiry` | true | 是否校验 exp 过期时间 |
| `ValidateNotBefore` | true | 是否校验 nbf 生效时间 |
| `ValidateIssuer` | true | 是否校验 iss 签发者 |
| `ValidateAudience` | true | 是否校验 aud 受众 |
| `ExpectedIssuer` | "" | 期望的签发者，仅 ValidateIssuer=true 时生效 |
| `ExpectedAudience` | nil | 期望的受众列表，仅 ValidateAudience=true 时生效 |

**受众校验规则**: 令牌中的 aud 声明必须包含 `ExpectedAudience` 中**所有**值，即 `ExpectedAudience` 是令牌受众的子集。

---

## 6. 黑名单吊销机制

### 6.1 内存黑名单实现 ([MemoryBlacklist](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/blacklist.go#L15-L110))

基于内存 map 的黑名单实现，支持 TTL 和自动清理。

```
内存黑名单结构:
{
    items: map[string]time.Time  // tokenID → 过期时间
    mu:    sync.RWMutex          // 读写锁保护并发访问
    stopCh:  chan struct{}       // 停止清理协程的信号通道
    closed:  bool                // 是否已关闭
    cleanupInt: time.Duration    // 清理间隔，0 表示不启动清理协程
}
```

### 6.2 吊销流程 ([RevokeToken](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L304-L306))

```
RevokeToken(tokenID)
    │
    └─→ 调用 blacklist.Add(tokenID, BlacklistTTL)
            │
            ├─ 检查 tokenID 是否为空 → ErrInvalidToken
            ├─ 检查黑名单是否已关闭 → ErrBlacklistClosed
            ├─ 计算过期时间: 当前时间 + BlacklistTTL
            └─ 存入 items map
```

### 6.3 Add 方法行为约定

| 条件 | 返回值 | 说明 |
|------|--------|------|
| tokenID 为空 | `ErrInvalidToken` | 拒绝空标识，避免无意义存储 |
| 黑名单已关闭 | `ErrBlacklistClosed` | 拒绝写入，防止数据静默丢失 |
| 正常情况 | `nil` | 成功存入黑名单 |

### 6.4 Contains 方法行为约定

| 条件 | 返回值 | 说明 |
|------|--------|------|
| tokenID 为空 | `(false, ErrInvalidToken)` | 与 Add 行为一致，返回错误便于调用方区分语义 |
| tokenID 不存在 | `(false, nil)` | 正常不存在，非错误 |
| tokenID 已过期 | `(false, nil)` | TTL 已过，视为不存在 |
| tokenID 存在且未过期 | `(true, nil)` | 在黑名单中 |

**设计要点**: `Add` 和 `Contains` 对空 tokenID 均返回 `ErrInvalidToken`，保证调用方可以通过错误类型区分"非法输入"和"正常不存在"两种语义。

### 6.5 Close 方法行为约定

| 条件 | 返回值 | 说明 |
|------|--------|------|
| 首次调用 | `nil` | 关闭 stopCh，停止清理协程，设置 closed=true |
| 重复调用 | `nil` | 检测到已 closed，不再重复 close channel，直接返回 |
| Close 后调用 Add | `ErrBlacklistClosed` | 拒绝写入，避免数据静默丢失 |

### 6.6 TTL 自动清理

```
cleanupLoop()
    │
    └─→ 定期触发（每 BlacklistCleanupInt）
            │
            └─→ cleanup()
                    │
                    ├─ 加写锁
                    ├─ 遍历所有 items
                    ├─ 过期时间已过则删除
                    └─ 释放锁
```

**设计要点**:
- 黑名单记录的 TTL 应至少等于访问令牌的最大有效期
- 自动清理协程在 Close 时会被正确停止
- 读取时也会检查 TTL，即使未到清理时间也不会返回过期记录
- `BlacklistCleanupInt` 为 0 时不启动清理协程，仅靠读取时惰性检查

---

## 7. 令牌续期机制 ([RenewToken](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L308-L330))

允许使用即将过期或刚过期的令牌换取新令牌，避免用户频繁重新登录。

```
RenewToken(token, options)
    │
    ├─→ 步骤 1：验证原令牌（跳过过期校验）
    │       └─ 使用 ValidateToken，但强制设置 ValidateExpiry=false
    │           这样即使令牌已过期，只要签名和格式合法仍可通过
    │
    ├─→ 步骤 2：检查续期窗口
    │       ├─ 计算可续期截止时间: ExpiresAt + RenewalWindow
    │       └─ 当前时间已超过截止时间 → ErrRenewalWindowExpired
    │
    ├─→ 步骤 3：可选吊销旧令牌
    │       └─ AutoBlacklistOld=true 时，将原 jti 加入黑名单
    │           防止旧令牌被继续使用
    │
    ├─→ 步骤 4：签发新令牌
    │       ├─ 复制原声明信息（包括自定义声明）
    │       ├─ 生成新的 jti（随机字符串）
    │       ├─ 更新 IssuedAt 为当前时间
    │       ├─ 更新 ExpiresAt 为当前时间 + AccessTokenTTL
    │       └─ 使用相同算法签名新令牌
    │
    └─→ 返回新的访问令牌
```

**续期窗口设计**:
- 目的：允许令牌过期后短时间内仍可续期，提升用户体验
- 典型场景：用户操作过程中令牌过期，无需重新登录
- 安全考量：续期窗口不宜过长（默认 5 分钟），平衡体验和安全
- 续期后的新令牌保留原令牌的所有声明信息（Subject、Custom 等）

---

## 8. 刷新令牌轮换机制 ([RefreshAccessToken](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jwtmgr/jwtmgr.go#L332-L384))

使用刷新令牌获取新的访问令牌，支持刷新令牌轮换增强安全性。

```
RefreshAccessToken(refreshToken)
    │
    ├─→ 步骤 1：验证刷新令牌
    │       ├─ 检查是否为空 → ErrInvalidRefreshToken
    │       ├─ 从存储中查询 → 未找到返回 ErrInvalidRefreshToken
    │       ├─ 检查是否已吊销 → ErrRefreshTokenRevoked
    │       └─ 检查是否过期 → ErrRefreshTokenExpired
    │
    ├─→ 步骤 2：签发新的访问令牌
    │       ├─ 使用原声明或创建新声明
    │       ├─ 生成新的 jti
    │       ├─ 设置新的 Issuer 和 Audience（从配置获取）
    │       ├─ 设置新的 IssuedAt 和 ExpiresAt
    │       └─ 签名生成新访问令牌
    │
    ├─→ 步骤 3：刷新令牌轮换（可选）
    │       │
    │       ├─ RefreshTokenRotation=true 时：
    │       │   ├─ 吊销旧刷新令牌（标记 Revoked=true）
    │       │   ├─ 生成新的刷新令牌（64 字节随机字符串）
    │       │   ├─ 存储新刷新令牌信息
    │       │   └─ 返回新的刷新令牌
    │       │
    │       └─ RefreshTokenRotation=false 时：
    │           └─ 返回原刷新令牌（不吊销、不更换）
    │
    └─→ 步骤 4：返回 TokenPair
            ├─ AccessToken: 新的访问令牌
            ├─ RefreshToken: 刷新令牌（新或旧）
            ├─ TokenID: 新令牌 ID
            └─ ExpiresAt: 新令牌过期时间
```

**轮换机制的安全价值**:
- 防止刷新令牌被截获后长期滥用
- 即使刷新令牌泄漏，攻击者最多只能使用一次
- 正常用户使用后，泄漏的令牌立即失效
- 可以检测到令牌泄漏（同一刷新令牌被多次使用时第二次会返回 `ErrRefreshTokenRevoked`）

---

## 9. 使用示例

### 9.1 基本使用（HS256）

```go
package main

import (
    "fmt"
    "solocoder-go/internal/jwtmgr"
)

func main() {
    config := jwtmgr.DefaultConfig()
    config.Issuer = "my-app"
    config.Audience = []string{"api", "web"}

    signingKey := jwtmgr.NewHS256Config([]byte("your-secret-key-at-least-32-bytes"))

    mgr, err := jwtmgr.NewManager(config, signingKey, nil, nil)
    if err != nil {
        panic(err)
    }
    defer mgr.Close()

    claims := &jwtmgr.Claims{
        Subject: "user123",
        Custom: map[string]interface{}{
            "role":  "admin",
            "email": "user@example.com",
        },
    }

    token, err := mgr.IssueToken(claims)
    if err != nil {
        panic(err)
    }
    fmt.Println("访问令牌:", token)

    opts := jwtmgr.DefaultValidationOptions()
    opts.ExpectedIssuer = "my-app"
    opts.ExpectedAudience = []string{"api"}

    parsedClaims, err := mgr.ValidateToken(token, opts)
    if err != nil {
        fmt.Println("令牌无效:", err)
        return
    }
    fmt.Println("用户:", parsedClaims.Subject)
    fmt.Println("角色:", parsedClaims.Custom["role"])
}
```

### 9.2 使用 RS256 非对称算法

```go
package main

import (
    "crypto/rand"
    "crypto/rsa"
    "solocoder-go/internal/jwtmgr"
)

func main() {
    privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        panic(err)
    }
    publicKey := &privateKey.PublicKey

    config := jwtmgr.DefaultConfig()
    signingKey := jwtmgr.NewRS256Config(privateKey, publicKey)

    mgr, err := jwtmgr.NewManager(config, signingKey, nil, nil)
    if err != nil {
        panic(err)
    }
    defer mgr.Close()

    claims := &jwtmgr.Claims{
        Subject: "user456",
    }

    token, err := mgr.IssueToken(claims)
    if err != nil {
        panic(err)
    }
}
```

### 9.3 使用刷新令牌

```go
package main

import (
    "fmt"
    "solocoder-go/internal/jwtmgr"
)

func main() {
    config := jwtmgr.DefaultConfig()
    config.RefreshTokenRotation = true
    signingKey := jwtmgr.NewHS256Config([]byte("your-secret-key"))

    mgr, _ := jwtmgr.NewManager(config, signingKey, nil, nil)
    defer mgr.Close()

    claims := &jwtmgr.Claims{Subject: "user123"}
    pair, err := mgr.IssueTokenPair(claims)
    if err != nil {
        panic(err)
    }

    fmt.Println("访问令牌:", pair.AccessToken)
    fmt.Println("刷新令牌:", pair.RefreshToken)

    newPair, err := mgr.RefreshAccessToken(pair.RefreshToken)
    if err != nil {
        panic(err)
    }

    fmt.Println("新访问令牌:", newPair.AccessToken)
    fmt.Println("新刷新令牌:", newPair.RefreshToken)

    // 旧刷新令牌已失效，再次使用会返回 ErrRefreshTokenRevoked
    _, err = mgr.RefreshAccessToken(pair.RefreshToken)
    if err == jwtmgr.ErrRefreshTokenRevoked {
        fmt.Println("旧刷新令牌已被吊销")
    }
}
```

### 9.4 令牌续期

```go
package main

import (
    "fmt"
    "solocoder-go/internal/jwtmgr"
)

func main() {
    config := jwtmgr.DefaultConfig()
    config.AutoBlacklistOld = true
    signingKey := jwtmgr.NewHS256Config([]byte("your-secret-key"))

    mgr, _ := jwtmgr.NewManager(config, signingKey, nil, nil)
    defer mgr.Close()

    claims := &jwtmgr.Claims{Subject: "user123"}
    token, _ := mgr.IssueToken(claims)

    opts := jwtmgr.DefaultValidationOptions()
    opts.ExpectedIssuer = "jwtmgr"
    opts.ExpectedAudience = []string{"api"}

    newToken, err := mgr.RenewToken(token, opts)
    if err != nil {
        fmt.Println("续期失败:", err)
        return
    }

    fmt.Println("新令牌:", newToken)

    // 旧令牌已自动加入黑名单
    _, err = mgr.ValidateToken(token, opts)
    if err == jwtmgr.ErrTokenBlacklisted {
        fmt.Println("旧令牌已自动吊销")
    }
}
```

### 9.5 吊销令牌

```go
package main

import (
    "fmt"
    "solocoder-go/internal/jwtmgr"
)

func main() {
    signingKey := jwtmgr.NewHS256Config([]byte("your-secret-key"))
    mgr, _ := jwtmgr.NewManager(jwtmgr.DefaultConfig(), signingKey, nil, nil)
    defer mgr.Close()

    claims := &jwtmgr.Claims{Subject: "user123"}
    token, _ := mgr.IssueToken(claims)

    opts := jwtmgr.DefaultValidationOptions()
    opts.ExpectedIssuer = "jwtmgr"
    opts.ExpectedAudience = []string{"api"}

    parsed, _ := mgr.ValidateToken(token, opts)
    fmt.Println("令牌 ID:", parsed.ID)

    err := mgr.RevokeToken(parsed.ID)
    if err != nil {
        panic(err)
    }

    _, err = mgr.ValidateToken(token, opts)
    if err == jwtmgr.ErrTokenBlacklisted {
        fmt.Println("令牌已成功吊销")
    }
}
```

### 9.6 完整工作流程

```go
package main

import (
    "fmt"
    "solocoder-go/internal/jwtmgr"
    "time"
)

func main() {
    config := jwtmgr.Config{
        Issuer:               "auth-service",
        Audience:             []string{"api", "web", "mobile"},
        AccessTokenTTL:       15 * time.Minute,
        RefreshTokenTTL:      7 * 24 * time.Hour,
        RenewalWindow:        5 * time.Minute,
        AutoBlacklistOld:     true,
        BlacklistTTL:         24 * time.Hour,
        BlacklistCleanupInt:  time.Hour,
        RefreshTokenRotation: true,
    }

    signingKey := jwtmgr.NewHS256Config([]byte("strong-secret-key-32-bytes-minimum"))
    mgr, _ := jwtmgr.NewManager(config, signingKey, nil, nil)
    defer mgr.Close()

    fmt.Println("=== 1. 用户登录，获取令牌对 ===")
    loginClaims := &jwtmgr.Claims{
        Subject: "john.doe@example.com",
        Custom: map[string]interface{}{
            "user_id": 1001,
            "role":    "admin",
            "name":    "John Doe",
        },
    }
    tokenPair, _ := mgr.IssueTokenPair(loginClaims)
    fmt.Println("访问令牌已签发")
    fmt.Println("刷新令牌已签发")

    fmt.Println("\n=== 2. 验证访问令牌 ===")
    validateOpts := jwtmgr.DefaultValidationOptions()
    validateOpts.ExpectedIssuer = "auth-service"
    validateOpts.ExpectedAudience = []string{"api"}

    claims, err := mgr.ValidateToken(tokenPair.AccessToken, validateOpts)
    if err != nil {
        fmt.Println("令牌无效:", err)
        return
    }
    fmt.Printf("用户: %s, 角色: %s\n", claims.Subject, claims.Custom["role"])

    fmt.Println("\n=== 3. 令牌续期 ===")
    renewedToken, _ := mgr.RenewToken(tokenPair.AccessToken, validateOpts)
    fmt.Println("令牌已续期，旧令牌已自动吊销")

    fmt.Println("\n=== 4. 使用刷新令牌获取新令牌对 ===")
    newPair, _ := mgr.RefreshAccessToken(tokenPair.RefreshToken)
    fmt.Println("新访问令牌已签发")
    fmt.Println("新刷新令牌已签发（旧刷新令牌已失效）")

    fmt.Println("\n=== 5. 主动吊销令牌 ===")
    mgr.RevokeToken(newPair.TokenID)
    fmt.Println("令牌已吊销，即使未过期也无法使用")
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidToken` | 令牌格式无效 | Token 格式不正确、解码失败、JSON 解析失败等；也用于 Blacklist 空 tokenID |
| `ErrEmptyToken` | 令牌为空 | 传入空字符串进行验证 |
| `ErrExpiredToken` | 令牌已过期 | 当前时间超过 exp 声明时间 |
| `ErrNotYetValid` | 令牌尚未生效 | 当前时间早于 nbf 声明时间 |
| `ErrInvalidSignature` | 签名无效 | 签名校验不通过，可能被篡改 |
| `ErrInvalidAlgorithm` | 算法不匹配 | Token 头部的 alg 与配置的算法不一致 |
| `ErrInvalidIssuer` | 签发者无效 | iss 声明与预期签发者不匹配 |
| `ErrInvalidAudience` | 受众无效 | aud 声明不包含所有预期受众 |
| `ErrTokenBlacklisted` | 令牌已吊销 | Token 的 jti 在黑名单中 |
| `ErrRenewalWindowExpired` | 续期窗口已过 | Token 过期时间 + RenewalWindow < 当前时间 |
| `ErrInvalidKey` | 密钥无效 | 签名密钥配置错误 |
| `ErrMissingKey` | 缺少密钥 | 创建 Manager 时未提供必要的密钥材料 |
| `ErrInvalidRefreshToken` | 刷新令牌无效 | 刷新令牌不存在或格式错误 |
| `ErrRefreshTokenRevoked` | 刷新令牌已吊销 | 刷新令牌已被标记为吊销 |
| `ErrRefreshTokenExpired` | 刷新令牌已过期 | 刷新令牌超过有效期 |
| `ErrBlacklistClosed` | 黑名单已关闭 | Close 后再调用 Add 尝试写入 |

---

## 11. 并发安全

模块完全支持并发访问，通过以下机制保证线程安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| MemoryBlacklist | `sync.RWMutex` | 读写锁保护 items map，多读单写 |
| MemoryRefreshStore | `sync.RWMutex` | 读写锁保护 tokens map，多读单写 |
| Manager 配置字段 | 只读 | 配置在创建时确定，运行期间不修改 |
| SigningKey | 只读 | 密钥材料在创建时确定，运行期间不修改 |

**最佳实践**:
1. **密钥管理**: HS256 密钥长度建议至少 32 字节，RS256 建议使用 2048 位以上的 RSA 密钥
2. **过期时间**: 访问令牌 TTL 不宜过长（建议 15-60 分钟），刷新令牌可设置较长时间
3. **续期窗口**: 续期窗口应远小于访问令牌 TTL，平衡体验和安全
4. **资源释放**: 使用完毕必须调用 `Close()` 停止黑名单清理协程
5. **算法选择**: 跨服务场景推荐使用 RS256，便于密钥分发和轮换
6. **敏感信息**: 不要在 JWT 中存放敏感信息，Payload 仅 Base64 编码并未加密
7. **Close 后操作**: Close 后调用 Add 会返回 `ErrBlacklistClosed`，调用方应检查此错误避免数据丢失
