# OAuth2 授权服务模块 (oauth2svc)

## 1. 模块功能概述

本模块实现了一个完整的 OAuth2.0 授权服务器，支持以下核心功能：

### 1.1 授权码流程 (Authorization Code Flow)
- 客户端引导用户跳转到授权页面
- 用户同意授权后生成一次性授权码
- 客户端使用授权码向令牌端点换取访问令牌和刷新令牌
- 适用于有用户参与的第三方应用授权场景

### 1.2 客户端凭证流程 (Client Credentials Flow)
- 客户端使用自身的 client_id 和 client_secret 直接请求访问令牌
- 无需用户参与，适用于服务间调用场景
- 仅返回访问令牌，不返回刷新令牌

### 1.3 Token 签发与刷新
- 访问令牌采用 JWT 格式，携带过期时间
- 支持使用刷新令牌换取新的访问令牌
- 刷新令牌可配置过期时间
- **刷新令牌滚动刷新（可配置）**：
  - 当 `Config.RefreshTokenRotation = true`（默认）时：刷新令牌使用后旧令牌立即失效，同时签发新的刷新令牌
  - 当 `Config.RefreshTokenRotation = false` 时：刷新令牌使用后旧令牌仍然有效，不会签发新的刷新令牌（非滚动模式）

### 1.4 Scope 校验
- 客户端请求时声明所需的权限范围
- 授权服务校验客户端是否有权访问请求的 Scope
- **授权码流程 Scope 子集校验**：令牌端点校验请求 Scope 必须是授权码原始 Scope 的子集，防止权限升级
- **刷新令牌 Scope 子集校验**：刷新时请求的 Scope 必须是原始 Scope 的子集
- 令牌签发后将 Scope 信息编码到 JWT 中
- 资源服务可根据令牌中的 Scope 做细粒度权限控制

### 1.5 授权码一次性消费
- 授权码一旦被成功用于换取令牌后立即标记为已使用
- 重复使用已消费的授权码返回 `invalid_grant` 错误
- 防止授权码被截获后重复利用

## 2. 核心结构体职责

### 2.1 AuthorizationServer
**文件**: [oauth2svc.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc.go)

授权服务器核心结构体，协调各个组件完成 OAuth2 流程：
- `Authorize()`: 处理授权请求，生成授权码
- `Token()`: 处理令牌请求，根据 grant_type 分发到不同处理器
- `ValidateToken()`: 验证访问令牌的有效性
- 内部方法处理三种授权模式的具体逻辑

### 2.2 Config
**文件**: [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go#L83-L101)

授权服务器配置：
- `Issuer`: 令牌签发者标识
- `AccessTokenTTL`: 访问令牌过期时间（默认 1 小时）
- `RefreshTokenTTL`: 刷新令牌过期时间（默认 7 天）
- `AuthorizationCodeTTL`: 授权码过期时间（默认 10 分钟）
- `RefreshTokenRotation`: 是否启用刷新令牌滚动刷新
- `SigningKey`: JWT 签名密钥

### 2.3 Client
**文件**: [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go#L36-L41)

OAuth2 客户端信息：
- `ID`: 客户端唯一标识
- `Secret`: 客户端密钥
- `RedirectURIs`: 允许的重定向 URI 列表
- `Scopes`: 客户端被允许的权限范围

### 2.4 AuthorizationCode
**文件**: [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go#L43-L52)

授权码信息：
- `Code`: 授权码字符串
- `ClientID`: 关联的客户端 ID
- `UserID`: 授权用户 ID
- `Scope`: 授权的权限范围
- `RedirectURI`: 重定向 URI
- `ExpiresAt`: 过期时间
- `Used`: 是否已使用标记
- `CreatedAt`: 创建时间

### 2.5 RefreshToken
**文件**: [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go#L62-L70)

刷新令牌信息：
- `Token`: 刷新令牌字符串
- `ClientID`: 关联的客户端 ID
- `UserID`: 关联的用户 ID
- `Scope`: 权限范围
- `ExpiresAt`: 过期时间
- `Revoked`: 是否已吊销
- `CreatedAt`: 创建时间

### 2.6 AccessTokenClaims
**文件**: [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go#L72-L81)

JWT 访问令牌声明：
- `Issuer` (iss): 签发者
- `Subject` (sub): 主体（用户 ID）
- `Audience` (aud): 受众（客户端 ID）
- `ExpiresAt` (exp): 过期时间
- `IssuedAt` (iat): 签发时间
- `ClientID` (cid): 客户端 ID
- `Scope` (scope): 权限范围
- `TokenID` (jti): 令牌唯一标识

### 2.7 存储接口
**文件**: [store.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/store.go)

- `ClientStore`: 客户端存储接口
- `AuthorizationCodeStore`: 授权码存储接口
- `RefreshTokenStore`: 刷新令牌存储接口

各接口均提供内存实现 `Memory*Store`，使用 `sync.RWMutex` 保证并发安全。

### 2.8 JWT 工具
**文件**: [jwt.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/jwt.go)

- `GenerateJWT()`: 生成 HS256 签名的 JWT 令牌
- `ParseJWT()` / `ValidateJWT()`: 解析和验证 JWT 令牌
- 内置 Base64URL 编码解码和 HMAC-SHA256 签名验证

## 3. 授权码流程完整时序

```
客户端                          授权服务器                          用户
  |                                 |                               |
  | 1. 引导用户到授权端点            |                               |
  | GET /authorize?                 |                               |
  |   response_type=code            |                               |
  |   &client_id=xxx                |                               |
  |   &redirect_uri=xxx             |                               |
  |   &scope=read+write             |------------------------------>|
  |                                 |  2. 显示授权页面              |
  |                                 |<------------------------------|
  |                                 |  3. 用户同意授权              |
  |<--------------------------------|                               |
  | 4. 重定向到 redirect_uri        |                               |
  |    ?code=AUTH_CODE&state=xxx    |                               |
  |                                 |                               |
  | 5. 使用授权码换取令牌           |                               |
  | POST /token                     |                               |
  |   grant_type=authorization_code |                               |
  |   &code=AUTH_CODE               |                               |
  |   &redirect_uri=xxx             |                               |
  |   &client_id=xxx                |                               |
  |   &client_secret=xxx            |------------------------------>|
  |                                 | 6. 验证授权码                 |
  |                                 |    - 校验 client_id/secret    |
  |                                 |    - 校验 code 是否有效       |
  |                                 |    - 校验 redirect_uri 匹配   |
  |                                 |    - 标记 code 为已使用       |
  |                                 |    - 生成 access_token        |
  |                                 |    - 生成 refresh_token       |
  |<--------------------------------|                               |
  | 7. 返回令牌响应                 |                               |
  | {                               |                               |
  |   "access_token": "JWT_TOKEN",  |                               |
  |   "token_type": "Bearer",       |                               |
  |   "expires_in": 3600,            |                               |
  |   "refresh_token": "REFRESH",   |                               |
  |   "scope": "read write"         |                               |
  | }                               |                               |
  |                                 |                               |
  | 8. 使用访问令牌访问资源         |                               |
  | GET /api/resource               |                               |
  | Authorization: Bearer JWT_TOKEN |------------------------------>|
  |                                 | 9. 验证 JWT 令牌              |
  |                                 |    - 验证签名                 |
  |                                 |    - 检查过期时间             |
  |                                 |    - 检查 Scope 权限          |
  |<--------------------------------|                               |
  | 10. 返回资源                    |                               |
```

### 刷新令牌时序

#### 滚动刷新模式（RefreshTokenRotation = true，默认）

```
客户端                          授权服务器
  |                                 |
  | 1. 使用刷新令牌                 |
  | POST /token                     |
  |   grant_type=refresh_token      |
  |   &refresh_token=OLD_REFRESH    |
  |   &client_id=xxx                |
  |   &client_secret=xxx            |------------------------------>|
  |                                 | 2. 验证刷新令牌               |
  |                                 |    - 校验 client_id/secret    |
  |                                 |    - 校验 refresh_token 有效  |
  |                                 |    - 吊销旧 refresh_token     |
  |                                 |    - 生成新 access_token      |
  |                                 |    - 生成新 refresh_token     |
  |<--------------------------------|
  | 3. 返回新令牌                   |
  | {                               |
  |   "access_token": "NEW_JWT",    |
  |   "token_type": "Bearer",       |
  |   "expires_in": 3600,            |
  |   "refresh_token": "NEW_REFRESH"|
  | }                               |
```

#### 非滚动模式（RefreshTokenRotation = false）

```
客户端                          授权服务器
  |                                 |
  | 1. 使用刷新令牌                 |
  | POST /token                     |
  |   grant_type=refresh_token      |
  |   &refresh_token=OLD_REFRESH    |
  |   &client_id=xxx                |
  |   &client_secret=xxx            |------------------------------>|
  |                                 | 2. 验证刷新令牌               |
  |                                 |    - 校验 client_id/secret    |
  |                                 |    - 校验 refresh_token 有效  |
  |                                 |    - **不吊销旧 refresh_token**|
  |                                 |    - 生成新 access_token      |
  |                                 |    - **不生成新 refresh_token**|
  |<--------------------------------|
  | 3. 返回新访问令牌               |
  | {                               |
  |   "access_token": "NEW_JWT",    |
  |   "token_type": "Bearer",       |
  |   "expires_in": 3600,            |
  |   "refresh_token": "OLD_REFRESH"|
  | }                               |
```

## 4. 使用示例

### 4.1 初始化授权服务器

```go
package main

import (
    "solocoder-go/internal/oauth2svc"
    "time"
)

func main() {
    config := oauth2svc.DefaultConfig()
    config.SigningKey = []byte("your-secret-key")
    config.AccessTokenTTL = 2 * time.Hour
    config.RefreshTokenTTL = 14 * 24 * time.Hour

    clientStore := oauth2svc.NewMemoryClientStore()
    codeStore := oauth2svc.NewMemoryAuthorizationCodeStore()
    refreshTokenStore := oauth2svc.NewMemoryRefreshTokenStore()

    clientStore.SaveClient(&oauth2svc.Client{
        ID:           "web-app",
        Secret:       "web-app-secret",
        RedirectURIs: []string{"http://localhost:8080/callback"},
        Scopes:       []string{"read", "write", "profile"},
    })

    server := oauth2svc.NewAuthorizationServer(
        config,
        clientStore,
        codeStore,
        refreshTokenStore,
    )

    // 使用 server 处理授权请求...
}
```

### 4.2 授权码流程示例

```go
// 1. 用户授权请求 - 生成授权码
authReq := &oauth2svc.AuthorizeRequest{
    ResponseType: oauth2svc.ResponseTypeCode,
    ClientID:     "web-app",
    RedirectURI:  "http://localhost:8080/callback",
    Scope:        "read write",
    State:        "random-state-123",
    UserID:       "user-456",
}

code, err := server.Authorize(authReq)
if err != nil {
    // 处理错误
}

// 2. 使用授权码换取令牌
tokenReq := &oauth2svc.TokenRequest{
    GrantType:    oauth2svc.GrantTypeAuthorizationCode,
    ClientID:     "web-app",
    ClientSecret: "web-app-secret",
    Code:         code,
    RedirectURI:  "http://localhost:8080/callback",
}

tokenResp, err := server.Token(tokenReq)
if err != nil {
    // 处理错误
}

fmt.Printf("Access Token: %s\n", tokenResp.AccessToken)
fmt.Printf("Refresh Token: %s\n", tokenResp.RefreshToken)
fmt.Printf("Expires In: %d seconds\n", tokenResp.ExpiresIn)
```

### 4.3 客户端凭证流程示例

```go
tokenReq := &oauth2svc.TokenRequest{
    GrantType:    oauth2svc.GrantTypeClientCredentials,
    ClientID:     "backend-service",
    ClientSecret: "backend-secret",
    Scope:        "read",
}

tokenResp, err := server.Token(tokenReq)
if err != nil {
    // 处理错误
}

// 客户端凭证流程不返回 refresh_token
fmt.Printf("Access Token: %s\n", tokenResp.AccessToken)
fmt.Printf("Refresh Token (should be empty): %s\n", tokenResp.RefreshToken)
```

### 4.4 刷新令牌示例

```go
refreshReq := &oauth2svc.TokenRequest{
    GrantType:    oauth2svc.GrantTypeRefreshToken,
    ClientID:     "web-app",
    ClientSecret: "web-app-secret",
    RefreshToken: "old-refresh-token",
    // 可以选择缩小 scope，但不能扩大
    Scope:        "read",
}

newTokenResp, err := server.Token(refreshReq)
if err != nil {
    // 处理错误
}

// 滚动刷新模式（RefreshTokenRotation = true）时，旧的刷新令牌已被吊销，必须使用新的
// 非滚动模式（RefreshTokenRotation = false）时，旧的刷新令牌仍然有效
fmt.Printf("New Access Token: %s\n", newTokenResp.AccessToken)
fmt.Printf("New Refresh Token: %s\n", newTokenResp.RefreshToken)
```

### 4.5 令牌验证示例

```go
// 资源服务器验证令牌
claims, err := server.ValidateToken(accessToken)
if err != nil {
    if err == oauth2svc.ErrExpiredToken {
        // 令牌已过期
    } else if err == oauth2svc.ErrInvalidToken {
        // 令牌无效（签名错误、格式错误等）
    }
    return
}

// 从令牌中获取信息进行权限控制
fmt.Printf("User ID: %s\n", claims.Subject)
fmt.Printf("Client ID: %s\n", claims.ClientID)
fmt.Printf("Scopes: %s\n", claims.Scope)

// 检查具体的 Scope 权限
hasScope := func(scope string) bool {
    scopes := strings.Fields(claims.Scope)
    for _, s := range scopes {
        if s == scope {
            return true
        }
    }
    return false
}

if hasScope("read") {
    // 允许读取操作
}
```

## 5. 错误码说明

| 错误 | 说明 |
|------|------|
| `ErrInvalidClient` | 客户端 ID 或密钥无效 |
| `ErrInvalidGrant` | 授权码或刷新令牌无效、已过期、已使用 |
| `ErrInvalidScope` | 请求的 Scope 超出客户端权限范围 |
| `ErrInvalidRequest` | 请求参数缺失或无效 |
| `ErrUnauthorizedClient` | 客户端未被授权使用该 grant_type |
| `ErrUnsupportedGrantType` | 不支持的授权类型 |
| `ErrInvalidToken` | 令牌格式错误或签名无效 |
| `ErrExpiredToken` | 令牌已过期 |
| `ErrCodeUsed` | 授权码已被使用 |
| `ErrCodeExpired` | 授权码已过期 |
| `ErrRefreshTokenRevoked` | 刷新令牌已被吊销 |

## 6. 安全注意事项

1. **签名密钥**: 生产环境必须使用足够强度的随机密钥，避免硬编码
2. **HTTPS**: 所有 OAuth2 端点必须通过 HTTPS 传输
3. **State 参数**: 授权请求中使用 state 参数防止 CSRF 攻击
4. **PKCE**: 对于公共客户端（如移动端、单页应用）应使用 PKCE 扩展
5. **授权码过期**: 授权码应设置较短的过期时间（默认 10 分钟）
6. **刷新令牌安全**: 刷新令牌应安全存储，避免泄露
7. **Scope 最小化**: 只请求必要的 Scope，遵循最小权限原则
8. **JWT 算法白名单校验**: 实现了严格的算法白名单机制，仅允许 `HS256` 算法签名，防止算法混淆攻击（如 `alg=none` 攻击）。在解析 JWT 时会先校验 `alg` 头字段，非白名单算法的令牌将被直接拒绝
