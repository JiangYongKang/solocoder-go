# CSRF 防护中间件模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [同步令牌模式（Synchronizer Token Pattern）](#4-同步令牌模式synchronizer-token-pattern)
5. [双重提交 Cookie 模式（Double Submit Cookie Pattern）](#5-双重提交-cookie-模式double-submit-cookie-pattern)
6. [Token 与会话绑定机制](#6-token-与会话绑定机制)
7. [请求来源校验机制](#7-请求来源校验机制)
8. [Token 轮换机制](#8-token-轮换机制)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [配置说明](#11-配置说明)
12. [并发安全](#12-并发安全)
13. [最佳实践](#13-最佳实践)

---

## 1. 模块概述

CSRF（Cross-Site Request Forgery，跨站请求伪造）防护中间件模块提供了完整的 Web 应用 CSRF 攻击防护解决方案。模块支持两种业界标准的防护模式：同步令牌模式和双重提交 Cookie 模式，同时提供请求来源校验、Token 与会话绑定、Token 自动轮换等高级安全特性，可有效抵御各类 CSRF 攻击。

**包路径**: `internal/csrf`

**设计目标**:
- 提供标准化的 CSRF 防护模式实现
- 确保 Token 与用户会话强绑定，防止跨会话复用
- 通过 Origin/Referer 校验增加防御深度
- 支持 Token 轮换机制，防止重放攻击
- 提供灵活的白名单机制应对合法跨域场景
- 完全兼容标准 `net/http` 接口

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 同步令牌模式 | Token 存储在服务端，客户端通过请求头/表单/Cookie 提交 |
| 双重提交 Cookie 模式 | Token 同时存在 Cookie 和请求头中，服务端比对一致性 |
| 会话绑定 | 每个 Token 唯一绑定用户会话，跨会话使用直接拒绝 |
| Origin 校验 | 校验 `Origin` 请求头是否为同源或在白名单中 |
| Referer 校验 | 校验 `Referer` 请求头来源是否合法 |
| 跨域白名单 | 支持配置受信任的跨域来源列表 |
| Token 轮换 | 每次成功请求后自动生成新 Token，旧 Token 立即失效 |
| Token 过期 | 支持 Token TTL 配置，自动清理过期 Token |
| 会话失效 | 用户登出时自动清理对应 Token |
| 自定义错误处理 | 支持自定义 403 拒绝响应 |

---

## 3. 核心结构体与职责

### 3.1 CSRF

主结构体，对外提供所有操作接口和 HTTP 中间件。

```go
type CSRF struct {
    cfg      Config
    mu       sync.RWMutex
    tokens   map[string]*sessionToken
    sessions map[string]string
}
```

**职责**:
- 管理 Token 的生成、验证、轮换、失效生命周期
- 维护 Token 到会话的双向映射关系
- 执行 HTTP 请求的完整 CSRF 校验流程
- 处理来源校验（Origin/Referer）
- 提供标准 `http.Handler` 中间件接口
- 维护过期 Token 的清理逻辑

### 3.2 Config

配置结构体，用于定制模块行为。

```go
type Config struct {
    Mode                ProtectionMode
    TokenLength         int
    TokenTTL            time.Duration
    CookieName          string
    CookieDomain        string
    CookiePath          string
    CookieSecure        bool
    CookieHTTPOnly      bool
    CookieSameSite      http.SameSite
    HeaderName          string
    FormFieldName       string
    SessionIDHeader     string
    SessionIDCookie     string
    TrustedOrigins      []string
    ProtectedMethods    []string
    EnableOriginCheck   bool
    EnableRefererCheck  bool
    EnableTokenRotation bool
    ErrorHandler        func(w http.ResponseWriter, r *http.Request, err error)
}
```

**默认配置**（`DefaultConfig()`）:
- `Mode`: `SynchronizerTokenMode`
- `TokenLength`: 32 字节
- `TokenTTL`: 24 小时
- `CookieName`: `XSRF-TOKEN`
- `HeaderName`: `X-CSRF-Token`
- `FormFieldName`: `csrf_token`
- `ProtectedMethods`: POST, PUT, DELETE, PATCH
- `EnableOriginCheck`: true
- `EnableRefererCheck`: true
- `EnableTokenRotation`: true
- `CookieSameSite`: `SameSiteStrictMode`

### 3.3 ProtectionMode

防护模式枚举类型。

```go
type ProtectionMode int

const (
    SynchronizerTokenMode   ProtectionMode = iota
    DoubleSubmitCookieMode
)
```

**模式说明**:
- `SynchronizerTokenMode`: 同步令牌模式，服务端存储 Token
- `DoubleSubmitCookieMode`: 双重提交 Cookie 模式，Token 同时在 Cookie 和请求头中

### 3.4 sessionToken

内部 Token 存储结构，记录 Token 元数据。

```go
type sessionToken struct {
    Token     string
    SessionID string
    ExpiresAt time.Time
}
```

**职责**:
- 存储 Token 字符串本身
- 记录绑定的会话 ID
- 记录过期时间戳

---

## 4. 同步令牌模式（Synchronizer Token Pattern）

同步令牌模式是最经典、最安全的 CSRF 防护方式，核心思想是"服务端持有真相"。

### 4.1 工作原理

```
用户会话建立
    │
    ▼
服务端生成随机 Token ──► 存储在服务端（绑定会话ID）
    │
    ▼
响应中携带 Token（响应头 / 表单隐藏字段）
    │
    ▼
客户端发起状态变更请求（POST/PUT/DELETE）
    │
    ├─► 从请求头 X-CSRF-Token 提取
    ├─► 或从表单字段 csrf_token 提取
    └─► 或从 Cookie XSRF-TOKEN 提取
    │
    ▼
服务端校验流程
    │
    ├─ 校验 1：Origin/Referer 来源是否合法
    ├─ 校验 2：Token 是否存在且未过期
    ├─ 校验 3：Token 绑定的会话ID是否与当前会话匹配
    │
    ├─ 全部通过 ──► 执行业务逻辑
    │                  │
    │                  └─► [可选] 轮换 Token，返回新 Token
    │
    └─ 任一失败 ──► 返回 403 Forbidden
```

### 4.2 Token 提取优先级

同步令牌模式下，Token 按以下顺序从请求中提取（先匹配先使用）：

1. **HTTP 请求头**：`X-CSRF-Token`（可配置）
2. **Cookie**：`XSRF-TOKEN`（可配置）
3. **表单字段**：`csrf_token`（可配置，仅 POST/PUT/PATCH）

### 4.3 优缺点

| 维度 | 说明 |
|------|------|
| **安全性** | 最高，Token 完全由服务端控制 |
| **存储开销** | 需要服务端存储 Token 映射 |
| **分布式支持** | 需要共享存储（Redis 等）或粘性会话 |
| **适用场景** | 有服务端会话存储的传统 Web 应用 |

---

## 5. 双重提交 Cookie 模式（Double Submit Cookie Pattern）

双重提交 Cookie 模式是一种无状态的 CSRF 防护方式，利用同源策略限制 Cookie 读取。

### 5.1 工作原理

```
用户首次访问（GET 请求）
    │
    ▼
服务端生成随机 Token
    │
    ├─► 设置 Cookie：XSRF-TOKEN=<token>（HttpOnly=false，允许 JS 读取）
    └─► 响应头：X-CSRF-Token: <token>
    │
    ▼
客户端发起状态变更请求
    │
    ├─► 浏览器自动携带 Cookie：XSRF-TOKEN=<token>
    └─► JS 从 Cookie 读取 Token，放入请求头：X-CSRF-Token: <token>
    │
    ▼
服务端校验流程
    │
    ├─ 校验 1：Origin/Referer 来源是否合法
    ├─ 校验 2：Cookie 中的 Token 和请求头中的 Token 是否都存在
    ├─ 校验 3：两个 Token 值是否完全一致
    ├─ 校验 4：Token 是否绑定到当前会话且未过期
    │
    ├─ 全部通过 ──► 执行业务逻辑
    │                  │
    │                  └─► [可选] 轮换 Token，更新 Cookie 和响应头
    │
    └─ 任一失败 ──► 返回 403 Forbidden，清除 Cookie
```

### 5.2 安全前提

双重提交 Cookie 模式的安全性建立在以下基础之上：

1. **同源策略**：攻击者网站的 JS 无法读取目标网站的 Cookie
2. **Cookie 设置**：Token Cookie 必须设置 `SameSite=Strict/Lax`
3. **HTTPS**：生产环境必须启用 `CookieSecure=true` 防止中间人攻击

### 5.3 优缺点

| 维度 | 说明 |
|------|------|
| **安全性** | 高，依赖同源策略 |
| **存储开销** | 无状态，无需服务端存储（可选绑定会话） |
| **分布式支持** | 天然支持，无需共享存储 |
| **适用场景** | SPA 单页应用、前后端分离架构、微服务 |

---

## 6. Token 与会话绑定机制

### 6.1 设计目标

防止攻击者将一个合法用户的 Token 用于另一个用户的会话（跨会话 Token 盗用）。

### 6.2 绑定关系

```
双向映射保证一致性：

sessions map:  SessionID ─────────────► Token
                    │                    │
                    │ 1:1 强绑定         │
                    ▼                    ▼
tokens map:    Token ───────► sessionToken{SessionID, ExpiresAt}
```

### 6.3 校验逻辑

```
ValidateToken(token, sessionID):
    │
    ├─ 步骤 1：在 tokens map 中查找 token
    │      └─ 不存在 → ErrTokenInvalid
    │
    ├─ 步骤 2：检查 token 是否已过期
    │      └─ 过期 → 清理后返回 ErrTokenInvalid
    │
    ├─ 步骤 3：比对 token 绑定的 SessionID
    │      ├─ token.SessionID != sessionID → ErrSessionMismatch
    │      └─ 匹配 → 返回 nil（校验通过）
    │
    └─ 关键：即使 token 本身有效，只要不属于当前会话就拒绝
```

### 6.4 会话销毁时的清理

当用户登出或会话过期时，通过 `InvalidateSession(sessionID)` 触发：

1. 根据 `sessionID` 从 `sessions` map 找到对应 Token
2. 从 `tokens` map 删除 Token 记录
3. 从 `sessions` map 删除会话映射

---

## 7. 请求来源校验机制

作为 CSRF 防护的**深度防御**层，在 Token 校验前先验证请求来源。

### 7.1 Origin 校验

**校验流程**:

```
checkOrigin(r):
    │
    ├─ EnableOriginCheck = false → 跳过
    │
    ├─ 读取 Origin 请求头
    │      └─ 空值 → 跳过（兼容旧浏览器）
    │
    ├─ 校验 1：是否同源（Origin.Host == r.Host）
    │      └─ 是 → 通过
    │
    └─ 校验 2：是否在 TrustedOrigins 白名单中
           ├─ 是 → 通过
           └─ 否 → ErrOriginNotAllowed
```

**白名单匹配规则**:

| 白名单配置 | 匹配 Origin | 说明 |
|-----------|------------|------|
| `https://trusted.com` | `https://trusted.com/path` | 协议+主机完全匹配 |
| `trusted.com` | `https://trusted.com` | 主机名匹配 |
| `trusted.com` | `https://app.trusted.com` | 子域名匹配（`.trusted.com` 后缀） |
| `http://192.168.1.1:8080` | `http://192.168.1.1:8080/api` | IP+端口匹配 |

### 7.2 Referer 校验

**校验流程**:

```
checkReferer(r):
    │
    ├─ EnableRefererCheck = false → 跳过
    │
    ├─ 读取 Referer 请求头
    │      └─ 空值 → 跳过（兼容隐私设置）
    │
    ├─ 解析 Referer URL，提取 Host
    │
    ├─ 校验 1：Referer.Host == r.Host（同源）
    │      └─ 是 → 通过
    │
    └─ 校验 2：Referer 的 Origin 在白名单中
           ├─ 是 → 通过
           └─ 否 → ErrRefererNotAllowed
```

### 7.3 安全建议

- 生产环境**必须启用** Origin 校验
- Referer 校验作为补充，某些场景下用户可能禁用
- 白名单仅配置**确实需要跨域**的来源
- 配合 `SameSite` Cookie 属性使用效果更佳

---

## 8. Token 轮换机制

### 8.1 设计目标

防止 Token 泄露后的**重放攻击**。即使 Token 被窃取，攻击者也只能使用一次。

### 8.2 轮换流程

```
POST 请求（Token 校验通过）
    │
    ▼
EnableTokenRotation = true ?
    │
    ├─ 否 → 跳过，使用原 Token
    │
    └─ 是
         │
         ├─ 步骤 1：调用 RotateToken(oldToken, sessionID)
         │        │
         │        ├─ 验证 oldToken 有效性
         │        ├─ 从 tokens/sessions map 删除 oldToken
         │        ├─ 生成 cryptographically secure 新 Token
         │        ├─ 建立新 Token 与会话的映射
         │        └─ 返回 newToken
         │
         ├─ 步骤 2：根据模式更新响应
         │        │
         │        ├─ 同步令牌模式：设置响应头 X-CSRF-Token
         │        └─ 双重提交模式：设置 Cookie + 响应头
         │
         └─ 步骤 3：客户端下次请求使用新 Token
```

### 8.3 轮换安全性

| 特性 | 说明 |
|------|------|
| **原子性** | 旧 Token 删除和新 Token 创建在同一临界区完成 |
| **一次性** | 旧 Token 立即失效，不可二次使用 |
| **密码学安全** | 使用 `crypto/rand` 生成，不可预测 |
| **会话绑定** | 新 Token 继续绑定同一会话 |

---

## 9. 使用示例

### 9.1 基本使用（同步令牌模式）

```go
package main

import (
    "fmt"
    "net/http"
    "solocoder-go/internal/csrf"
)

func main() {
    // 使用默认配置（同步令牌模式）
    csrfProtector := csrf.NewCSRF()

    mux := http.NewServeMux()

    // Token 发放端点
    mux.HandleFunc("/csrf/token", csrfProtector.GenerateHandler)

    // 受保护的表单页面
    mux.HandleFunc("/form", func(w http.ResponseWriter, r *http.Request) {
        sessionID := r.Header.Get("X-Session-ID")
        token, err := csrfProtector.GetToken(sessionID)
        if err != nil {
            token, _ = csrfProtector.GenerateToken(sessionID)
        }
        fmt.Fprintf(w, `
            <form method="POST" action="/submit">
                <input type="hidden" name="csrf_token" value="%s">
                <input type="text" name="data">
                <button type="submit">提交</button>
            </form>
        `, token)
    })

    // 受保护的提交端点
    submitHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("提交成功！"))
    })
    mux.Handle("/submit", csrfProtector.Middleware(submitHandler))

    http.ListenAndServe(":8080", mux)
}
```

### 9.2 双重提交 Cookie 模式（推荐用于 SPA）

```go
package main

import (
    "net/http"
    "solocoder-go/internal/csrf"
    "time"
)

func main() {
    cfg := csrf.Config{
        Mode:                csrf.DoubleSubmitCookieMode,
        TokenLength:         32,
        TokenTTL:            12 * time.Hour,
        CookieName:          "XSRF-TOKEN",
        CookiePath:          "/",
        CookieSecure:        true,  // 生产环境必须 true
        CookieHTTPOnly:      false, // 必须为 false，JS 需要读取
        CookieSameSite:      http.SameSiteStrictMode,
        HeaderName:          "X-CSRF-Token",
        SessionIDCookie:     "SESSIONID",
        TrustedOrigins:      []string{"https://app.example.com"},
        ProtectedMethods:    []string{"POST", "PUT", "DELETE", "PATCH"},
        EnableOriginCheck:   true,
        EnableRefererCheck:  true,
        EnableTokenRotation: true,
    }

    csrfProtector, err := csrf.NewCSRFWithConfig(cfg)
    if err != nil {
        panic(err)
    }

    mux := http.NewServeMux()

    // API 端点
    apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.Write([]byte(`{"status":"ok"}`))
    })
    mux.Handle("/api/data", csrfProtector.Middleware(apiHandler))

    http.ListenAndServe(":8080", mux)
}
```

### 9.3 自定义错误处理

```go
cfg := csrf.DefaultConfig()
cfg.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusForbidden)
    w.Write([]byte(`{"error":"csrf_protection_failed","detail":"` + err.Error() + `"}`))
}

csrfProtector, _ := csrf.NewCSRFWithConfig(cfg)
```

### 9.4 手动管理 Token 生命周期

```go
csrfProtector := csrf.NewCSRF()
sessionID := "user-session-123"

// 生成 Token
token, err := csrfProtector.GenerateToken(sessionID)

// 查询当前 Token
currentToken, err := csrfProtector.GetToken(sessionID)

// 验证 Token
err = csrfProtector.ValidateToken(token, sessionID)

// 手动轮换
newToken, err := csrfProtector.RotateToken(token, sessionID)

// 用户登出时失效会话
csrfProtector.InvalidateSession(sessionID)

// 手动失效单个 Token
csrfProtector.InvalidateToken(token)

// 清理所有过期 Token
cleanedCount := csrfProtector.CleanExpired()
```

### 9.5 跨域白名单配置

```go
cfg := csrf.DefaultConfig()
cfg.TrustedOrigins = []string{
    // 完整 URL 匹配
    "https://app.example.com",
    "https://admin.example.com:8443",
    // 主机名匹配（支持子域名）
    "internal.corp",
    // IP + 端口
    "http://10.0.0.1:3000",
}
```

---

## 10. 错误定义

| 错误变量 | HTTP 状态 | 含义 | 触发场景 |
|----------|----------|------|----------|
| `ErrTokenNotFound` | 403 | Token 未找到 | 请求中未提取到 Token |
| `ErrTokenMismatch` | 403 | Token 不匹配 | 双重提交模式下 Cookie 与请求头 Token 不一致 |
| `ErrTokenInvalid` | 403 | Token 无效 | Token 不存在、已过期或已被轮换失效 |
| `ErrOriginNotAllowed` | 403 | 来源不允许 | Origin 请求头不在同源或白名单中 |
| `ErrRefererNotAllowed` | 403 | 引用不允许 | Referer 请求头来源不合法 |
| `ErrSessionNotFound` | 403/401 | 会话未找到 | 会话 ID 为空或 GenerateToken 时无会话 |
| `ErrSessionMismatch` | 403 | 会话不匹配 | Token 绑定的会话与当前会话不一致 |
| `ErrInvalidConfig` | - | 配置无效 | 创建 CSRF 实例时配置参数非法 |

---

## 11. 配置说明

### 11.1 完整配置参数

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `Mode` | `ProtectionMode` | `SynchronizerTokenMode` | 防护模式选择 |
| `TokenLength` | `int` | 32 | Token 字节长度（最小 16） |
| `TokenTTL` | `time.Duration` | 24h | Token 有效期 |
| `CookieName` | `string` | `XSRF-TOKEN` | Token Cookie 名称 |
| `CookieDomain` | `string` | `""` | Cookie 作用域名 |
| `CookiePath` | `string` | `/` | Cookie 路径 |
| `CookieSecure` | `bool` | false | HTTPS-only Cookie |
| `CookieHTTPOnly` | `bool` | false | JS 不可读 Cookie（双重提交模式必须 false） |
| `CookieSameSite` | `http.SameSite` | `SameSiteStrict` | SameSite 属性 |
| `HeaderName` | `string` | `X-CSRF-Token` | Token 请求头/响应头名称 |
| `FormFieldName` | `string` | `csrf_token` | 表单隐藏字段名称 |
| `SessionIDHeader` | `string` | `X-Session-ID` | 会话 ID 请求头名称 |
| `SessionIDCookie` | `string` | `SESSIONID` | 会话 ID Cookie 名称 |
| `TrustedOrigins` | `[]string` | `[]` | 跨域白名单 |
| `ProtectedMethods` | `[]string` | POST/PUT/DELETE/PATCH | 需要校验的 HTTP 方法 |
| `EnableOriginCheck` | `bool` | true | 启用 Origin 校验 |
| `EnableRefererCheck` | `bool` | true | 启用 Referer 校验 |
| `EnableTokenRotation` | `bool` | true | 启用 Token 轮换 |
| `ErrorHandler` | `func` | 默认 403 响应 | 自定义错误处理 |

### 11.2 配置校验规则

创建实例时会自动校验以下规则，不满足则返回 `ErrInvalidConfig`：

- `TokenLength` >= 16 字节
- `TokenTTL` >= 0
- `CookieName` 非空
- `HeaderName` 非空
- `FormFieldName` 非空

---

## 12. 并发安全

模块完全支持并发访问，通过以下机制保证线程安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| Token map | `sync.RWMutex` | 读多写少场景，读锁共享 |
| Session map | `sync.RWMutex` | 与 Token map 共用同一把锁 |
| 生成 Token | 互斥锁临界区 | 确保替换旧 Token 和新 Token 创建的原子性 |
| Token 轮换 | 互斥锁临界区 | 旧删除 + 新创建在同一锁内完成 |
| HTTP 中间件 | 天然并发安全 | 每个请求独立处理，无共享可变状态 |

---

## 13. 最佳实践

### 13.1 生产环境必选配置

```go
cfg := csrf.Config{
    CookieSecure:   true,          // 仅 HTTPS 传输 Cookie
    CookieSameSite: http.SameSiteStrictMode, // 最强 SameSite 保护
    EnableOriginCheck:   true,     // 启用来源校验
    EnableRefererCheck:  true,     // 启用引用校验
    EnableTokenRotation: true,     // 启用自动轮换
    TokenLength: 32,               // 足够的熵
    TokenTTL:    1 * time.Hour,    // 较短的有效期
}
```

### 13.2 防护模式选择

| 应用架构 | 推荐模式 | 理由 |
|---------|---------|------|
| 传统服务端渲染（SSR） | 同步令牌模式 | 有服务端会话，安全性最高 |
| SPA + REST API | 双重提交 Cookie 模式 | 无状态，JS 易读取 Cookie |
| 微服务网关层 | 双重提交 Cookie 模式 | 分布式友好，无需共享状态 |
| 敏感操作（支付、管理） | 同步令牌模式 + 严格模式 | 安全性优先 |

### 13.3 防御深度建议

1. **多层防护**：不要仅依赖 CSRF 中间件，同时配置：
   - Cookie `SameSite=Strict`
   - 合理的 CORS 策略
   - 敏感操作二次验证（密码、短信验证码）

2. **Token 泄露应对**：
   - 启用 `EnableTokenRotation` 减少泄露影响窗口
   - 设置较短的 `TokenTTL`
   - 用户修改密码/登出时调用 `InvalidateSession`

3. **日志监控**：
   - 自定义 `ErrorHandler` 记录 403 拒绝日志
   - 监控短时间内大量 CSRF 失败（可能表示攻击）

4. **测试验证**：
   - 验证跨域请求被正确拒绝
   - 验证 Token 轮换后旧 Token 失效
   - 验证会话切换后原会话 Token 不可用
