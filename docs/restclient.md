# 声明式 REST HTTP 客户端模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [请求模板生命周期](#4-请求模板生命周期)
5. [参数绑定流程](#5-参数绑定流程)
6. [认证注入机制](#6-认证注入机制)
7. [重试与超时机制](#7-重试与超时机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全](#10-并发安全)
11. [最佳实践](#11-最佳实践)

---

## 1. 模块概述

声明式 REST HTTP 客户端（Declarative REST HTTP Client）是一个通过模板化方式管理 HTTP 请求的通用模块。它允许开发者预定义请求模板（包含 HTTP 方法、URL 路径、默认头、超时、重试策略等），然后通过模板名称快速发起请求，避免重复编写样板代码。

**包路径**: `internal/restclient`

**设计目标**:
- 提供声明式的请求模板注册机制，减少重复代码
- 支持路径参数与查询参数的声明式绑定
- 支持默认请求头与认证信息的自动注入
- 内置请求超时与重试机制，提高请求可靠性
- 使用内存 HTTP 传输层（`net/http/httptest`）支持单元测试
- 完全兼容 `context.Context`，支持取消与超时传递

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 请求模板注册 | 预定义 HTTP 方法、基础 URL、路径、默认请求头等，按名称注册与获取 |
| 模板覆盖 | 同一名称重复注册时，后者覆盖前者 |
| 路径参数绑定 | URL 路径中使用 `{param}` 占位符，请求时传入键值对自动替换 |
| 查询参数绑定 | 通过键值对传入查询参数，自动按 URL 编码规则拼接 |
| 默认请求头 | 为每个模板配置默认请求头，发起请求时自动注入 |
| 请求头合并 | 模板默认头与本次请求额外头合并，额外头优先级更高 |
| 认证提供器 | 注册全局认证提供器，请求前自动注入令牌或签名 |
| 请求超时 | 为每个模板配置独立的请求超时时间 |
| 自动重试 | 请求失败后按配置的重试次数自动重新发起 |
| 固定重试间隔 | 两次重试之间使用固定间隔等待 |
| 错误链式包装 | 支持 `errors.Is` / `errors.As` 穿透错误链检查 |
| 非可重试错误识别 | 请求构建类错误不进入重试流程，直接返回 |

---

## 3. 核心结构体与职责

### 3.1 Client

HTTP 客户端主结构体，管理模板注册、认证提供器和 HTTP 传输。

```go
type Client struct {
    mu            sync.RWMutex
    templates     map[string]*RequestTemplate
    authProviders map[string]AuthProvider
    httpClient    *http.Client
    baseURL       string
}
```

**职责**:
- 管理请求模板的注册、获取与注销
- 管理认证提供器的注册与获取
- 执行 HTTP 请求（通过 `Do` 方法）
- 协调重试与超时逻辑
- 处理请求头合并与认证注入

### 3.2 RequestTemplate

请求模板结构体，定义一个请求的静态配置。

```go
type RequestTemplate struct {
    Name           string
    Method         string
    BaseURL        string
    Path           string
    DefaultHeaders http.Header
    Timeout        time.Duration
    MaxRetries     int
    RetryInterval  time.Duration
    AuthProvider   string
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `Name` | `string` | 模板名称，唯一标识 |
| `Method` | `string` | HTTP 方法，默认为 GET |
| `BaseURL` | `string` | 模板级基础 URL，优先级高于 Client 级 |
| `Path` | `string` | URL 路径，可包含 `{param}` 占位符 |
| `DefaultHeaders` | `http.Header` | 默认请求头 |
| `Timeout` | `time.Duration` | 单次请求超时时间 |
| `MaxRetries` | `int` | 最大重试次数（不含首次请求） |
| `RetryInterval` | `time.Duration` | 重试间隔 |
| `AuthProvider` | `string` | 使用的认证提供器名称 |

### 3.3 RequestOptions

单次请求的动态参数。

```go
type RequestOptions struct {
    PathParams  map[string]string
    QueryParams map[string]string
    Headers     http.Header
    Body        []byte
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `PathParams` | `map[string]string` | 路径参数键值对 |
| `QueryParams` | `map[string]string` | 查询参数键值对 |
| `Headers` | `http.Header` | 本次请求额外的请求头 |
| `Body` | `[]byte` | 请求体内容 |

### 3.4 AuthProvider

认证提供器接口。

```go
type AuthProvider interface {
    Name() string
    Inject(req *http.Request) error
}
```

**职责**:
- `Name()` 返回提供器名称，用于注册和查找
- `Inject(req)` 将认证信息注入到 HTTP 请求头中

### 3.5 requestBuildError

请求构建错误类型（内部类型）。

```go
type requestBuildError struct {
    err error
}
```

**职责**:
- 包装请求构建过程中的错误
- 实现 `Is(target error) bool` 方法以支持 `errors.Is` 匹配 `ErrRequestBuildFailed`
- 实现 `Unwrap() error` 方法以支持错误链穿透

---

## 4. 请求模板生命周期

### 4.1 注册阶段

```
调用 RegisterTemplate(tmpl)
        │
        ▼
  校验模板名称非空
        │
        ▼
  归一化默认值：
  - Method 为空 → GET
  - DefaultHeaders 为 nil → 空 Header
  - Timeout < 0 → 0
  - MaxRetries < 0 → 0
  - RetryInterval < 0 → 0
        │
        ▼
  存入 templates map（同名覆盖）
```

### 4.2 使用阶段

```
调用 Do(ctx, templateName, opts)
        │
        ▼
  查找模板 → 不存在返回 ErrTemplateNotFound
        │
        ▼
  for 循环（最多 1 + MaxRetries 次）：
    ├── 检查 Context 是否已取消
    ├── 构建请求 URL（路径参数 + 查询参数）
    ├── 创建 http.Request
    ├── 合并请求头（模板默认头 + 请求头）
    ├── 注入认证（如果配置了 AuthProvider）
    ├── 执行 HTTP 请求
    ├── 成功 → 返回 Response
    └── 失败：
        ├── 非可重试错误 → 直接返回错误
        ├── 达到最大重试次数 → 返回 ErrMaxRetriesExceeded
        └── 否则 → 等待 RetryInterval 后继续
```

### 4.3 注销阶段

```
调用 UnregisterTemplate(name)
        │
        ▼
  从 templates map 中删除
```

---

## 5. 参数绑定流程

### 5.1 路径参数绑定

**占位符格式**: `{参数名}`

**示例**:
- 模板路径: `/users/{id}/posts/{postId}`
- 路径参数: `{"id": "123", "postId": "456"}`
- 结果路径: `/users/123/posts/456`

**绑定规则**:
1. 参数值使用 `url.PathEscape` 进行 URL 路径编码
2. 同一占位符可出现多次，全部替换
3. 若路径中存在未被替换的占位符，返回 `ErrPathParamMissing`
4. 若路径中包含占位符但未提供路径参数，返回 `ErrPathParamMissing`

### 5.2 查询参数绑定

**绑定规则**:
1. 通过 `url.Values` 进行 URL 查询编码
2. 自动处理特殊字符的转义（如空格转 `+` 或 `%20`）
3. 如果 URL 中已存在 `?`，则使用 `&` 拼接
4. 如果 URL 中不存在 `?`，则使用 `?` 拼接

### 5.3 基础 URL 拼接

**优先级**: 模板级 `BaseURL` > Client 级 `baseURL`

**斜杠处理**:
- `base` 结尾有 `/` + `path` 开头有 `/` → 去掉 path 的开头 `/`
- `base` 结尾无 `/` + `path` 开头无 `/` → 中间添加 `/`
- 其他情况 → 直接拼接

---

## 6. 认证注入机制

### 6.1 工作原理

1. 实现 `AuthProvider` 接口，定义认证注入逻辑
2. 通过 `RegisterAuthProvider` 注册到 Client
3. 在请求模板中通过 `AuthProvider` 字段指定使用的认证提供器名称
4. 每次请求前自动调用 `Inject` 方法注入认证信息

### 6.2 执行时机

认证注入发生在：
- 请求头合并之后
- HTTP 请求发送之前

### 6.3 自定义认证提供器示例

```go
type BearerAuthProvider struct {
    token string
}

func (p *BearerAuthProvider) Name() string {
    return "bearer"
}

func (p *BearerAuthProvider) Inject(req *http.Request) error {
    req.Header.Set("Authorization", "Bearer "+p.token)
    return nil
}
```

---

## 7. 重试与超时机制

### 7.1 请求超时

- 每个模板可配置独立的 `Timeout`
- 使用 `context.WithTimeout` 为单次请求创建超时上下文
- 超时到达后请求被取消，返回超时错误
- `Timeout <= 0` 表示不设置单次请求超时（使用 http.Client 的默认超时）

### 7.2 自动重试

**重试触发条件**:
- HTTP 请求执行失败（网络错误等）
- 错误为可重试错误（非请求构建类错误）

**不可重试错误**:
- `ErrRequestBuildFailed` — 请求构建失败
- `ErrPathParamMissing` — 路径参数缺失
- `ErrAuthProviderNotFound` — 认证提供器不存在
- `context.Canceled` — 上下文被主动取消

**重试次数**:
- `MaxRetries` 表示最大重试次数，不包含首次请求
- 总请求次数 = 1（首次） + `MaxRetries`（重试）
- `MaxRetries = 0` 表示不重试（仅执行 1 次）

### 7.3 重试间隔

- 使用固定间隔策略（`RetryInterval`）
- 两次重试之间等待固定时间
- 等待可被 Context 取消中断
- `RetryInterval <= 0` 表示不等待立即重试

### 7.4 错误返回

- 首次成功: 返回 `(response, nil)`
- 非可重试错误: 返回 `(nil, 原始错误)`
- 达到最大重试次数: 返回 `(nil, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr))`

---

## 8. 使用示例

### 8.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "io"
    "solocoder-go/internal/restclient"
)

func main() {
    client := restclient.NewClient(
        restclient.WithBaseURL("https://api.example.com"),
    )

    defaultHeaders := make(http.Header)
    defaultHeaders.Set("Content-Type", "application/json")
    defaultHeaders.Set("Accept", "application/json")

    client.RegisterTemplate(restclient.RequestTemplate{
        Name:           "get_user",
        Method:         http.MethodGet,
        Path:           "/users/{id}",
        DefaultHeaders: defaultHeaders,
        Timeout:        5 * time.Second,
        MaxRetries:     3,
        RetryInterval:  500 * time.Millisecond,
    })

    opts := &restclient.RequestOptions{
        PathParams: map[string]string{
            "id": "123",
        },
        QueryParams: map[string]string{
            "verbose": "true",
        },
    }

    resp, err := client.Do(context.Background(), "get_user", opts)
    if err != nil {
        fmt.Printf("请求失败: %v\n", err)
        return
    }
    defer resp.Body.Close()

    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("响应: %s\n", string(body))
}
```

### 8.2 使用认证提供器

```go
client := restclient.NewClient(
    restclient.WithBaseURL("https://api.example.com"),
)

auth := &BearerAuthProvider{token: "my-secret-token"}
client.RegisterAuthProvider(auth)

client.RegisterTemplate(restclient.RequestTemplate{
    Name:         "create_user",
    Method:       http.MethodPost,
    Path:         "/users",
    AuthProvider: "bearer",
    Timeout:      10 * time.Second,
})

opts := &restclient.RequestOptions{
    Body: []byte(`{"name":"Alice","email":"alice@example.com"}`),
}

resp, err := client.Do(ctx, "create_user", opts)
```

### 8.3 模板覆盖

```go
client.RegisterTemplate(restclient.RequestTemplate{
    Name:   "list_items",
    Method: http.MethodGet,
    Path:   "/v1/items",
})

// 覆盖为 v2 版本
client.RegisterTemplate(restclient.RequestTemplate{
    Name:   "list_items",
    Method: http.MethodGet,
    Path:   "/v2/items",
})

// 后续请求使用 v2 路径
```

### 8.4 请求头合并

```go
defaultHeaders := make(http.Header)
defaultHeaders.Set("Accept", "application/json")
defaultHeaders.Set("X-API-Version", "1.0")

client.RegisterTemplate(restclient.RequestTemplate{
    Name:           "test",
    Method:         http.MethodGet,
    Path:           "/test",
    DefaultHeaders: defaultHeaders,
})

requestHeaders := make(http.Header)
requestHeaders.Set("Authorization", "Bearer token")
requestHeaders.Add("Accept", "text/plain")  // 追加，不是覆盖

opts := &restclient.RequestOptions{
    Headers: requestHeaders,
}

// 最终请求头：
//   Accept: application/json, text/plain
//   X-API-Version: 1.0
//   Authorization: Bearer token
```

### 8.5 错误处理

```go
resp, err := client.Do(ctx, "get_user", opts)
if err != nil {
    switch {
    case errors.Is(err, restclient.ErrTemplateNotFound):
        fmt.Println("模板不存在")
    case errors.Is(err, restclient.ErrPathParamMissing):
        fmt.Println("路径参数缺失")
    case errors.Is(err, restclient.ErrMaxRetriesExceeded):
        fmt.Println("重试次数耗尽")
    case errors.Is(err, restclient.ErrAuthProviderNotFound):
        fmt.Println("认证提供器不存在")
    case errors.Is(err, context.DeadlineExceeded):
        fmt.Println("请求超时")
    case errors.Is(err, context.Canceled):
        fmt.Println("请求被取消")
    default:
        fmt.Printf("未知错误: %v\n", err)
    }
    return
}
```

### 8.6 使用自定义 HTTP Client

```go
customTransport := &myCustomTransport{}
customClient := &http.Client{
    Transport: customTransport,
    Timeout:   30 * time.Second,
}

client := restclient.NewClient(
    restclient.WithHTTPClient(customClient),
)
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrTemplateNotFound` | 模板不存在 | 使用未注册的模板名称发起请求 |
| `ErrTemplateNameEmpty` | 模板名称为空 | 注册空名称的模板 |
| `ErrPathParamMissing` | 路径参数缺失 | URL 路径包含占位符但缺少对应参数值 |
| `ErrRequestBuildFailed` | 请求构建失败 | URL 构建或请求创建失败（包装内部错误） |
| `ErrMaxRetriesExceeded` | 超过最大重试次数 | 所有重试均失败 |
| `ErrAuthProviderNotFound` | 认证提供器不存在 | 模板指定的认证提供器未注册 |

**错误链穿透**:
- `ErrRequestBuildFailed` 包装的内部错误可通过 `errors.Is` 和 `errors.As` 穿透访问
- 例如: `errors.Is(err, ErrPathParamMissing)` 在错误被 `ErrRequestBuildFailed` 包装时仍然返回 `true`

---

## 10. 并发安全

| 操作 | 安全状态 | 说明 |
|------|---------|------|
| 注册模板 | ✓ 安全 | 使用 `sync.RWMutex` 写锁保护 |
| 获取模板 | ✓ 安全 | 使用 `sync.RWMutex` 读锁保护 |
| 注册认证提供器 | ✓ 安全 | 使用 `sync.RWMutex` 写锁保护 |
| 获取认证提供器 | ✓ 安全 | 使用 `sync.RWMutex` 读锁保护 |
| 并发调用 Do | ✓ 安全 | 每次请求读取模板快照，无共享状态 |
| GetTemplate 返回值修改 | - 安全 | 返回模板副本，修改不影响内部状态 |

**注意**: `RequestTemplate` 的 `DefaultHeaders` 字段在注册时直接存储引用。建议注册后不要修改外部传入的 `http.Header`，以避免潜在的并发问题。

---

## 11. 最佳实践

### 11.1 模板设计建议

- **模板粒度**: 按业务接口设计模板，一个接口对应一个模板
- **命名规范**: 使用 `动词_资源` 或 `资源_操作` 命名，如 `get_user`、`create_order`
- **默认头配置**: 将通用头（如 Content-Type、Accept）放入模板默认头
- **超时设置**: 根据接口特性设置合理的超时时间，避免过长或过短

### 11.2 重试策略建议

| 场景 | MaxRetries | RetryInterval | Timeout |
|------|-----------|---------------|---------|
| 内部服务调用 | 2 ~ 3 | 100 ~ 500ms | 1 ~ 5s |
| 外部 API 调用 | 3 ~ 5 | 500ms ~ 2s | 5 ~ 30s |
| 关键写操作 | 0 ~ 1 | 1s | 10s |
| 幂等读操作 | 3 ~ 5 | 200ms ~ 1s | 5s |

### 11.3 安全注意事项

1. **认证令牌安全**: 认证提供器中的令牌不应通过日志输出
2. **敏感参数**: 路径参数和查询参数中的敏感信息应注意日志脱敏
3. **HTTPS**: 生产环境必须使用 HTTPS 传输

### 11.4 测试建议

- 使用 `httptest.NewServer` 创建内存 HTTP 服务器进行测试
- 通过自定义 `http.RoundTripper` 模拟网络错误和重试场景
- 覆盖正常流程、边界条件和异常分支
