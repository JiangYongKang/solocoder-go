# API 版本化路由模块

## 1. 模块功能概述

`apiver` 模块提供了一套完整的 API 版本化路由解决方案，支持多种版本策略和自动版本兼容转换。该模块位于 `internal/apiver/` 包下，主要功能包括：

- **多策略版本提取**：支持 URL 路径、HTTP 请求头、查询参数三种版本指定方式
- **统一版本格式校验**：三种策略共用同一套版本格式校验规则，确保一致性
- **版本优先级管理**：支持多版本处理器注册与管理，可自定义提取策略优先级
- **自动版本转换**：当客户端请求旧版本时，自动进行请求和响应的格式转换
- **优雅降级**：当请求转换器缺失时，自动降级直接调用对应版本的处理器
- **线程安全**：所有操作都经过并发安全设计

## 2. 核心结构体与职责

### 2.1 Version 类型

```go
type Version string
```

- **职责**：表示 API 版本号，通常格式为 `v1`, `v2`, `v10` 等

**主要方法**：
- `String()`: 返回版本字符串
- `Compare(other Version) int`: 版本比较，返回 -1, 0, 1 分别表示小于、等于、大于

### 2.2 VersionExtractor 接口

```go
type VersionExtractor interface {
    ExtractVersion(r *http.Request) (Version, bool)
    Strategy() VersionStrategy
}
```

- **职责**：定义版本提取器的通用接口

### 2.3 PathVersionExtractor

```go
type PathVersionExtractor struct {
    pattern *regexp.Regexp
}
```

- **职责**：从 URL 路径中提取版本号
- **默认匹配模式**：`^/(v\d+)(/.*)?$`
- **示例**：`/v1/users` 提取版本 `v1`

### 2.4 HeaderVersionExtractor

```go
type HeaderVersionExtractor struct {
    HeaderName string
}
```

- **职责**：从 HTTP 请求头中提取版本号
- **默认请求头**：`API-Version`
- **可定制**：通过 `NewHeaderVersionExtractorWithName()` 自定义请求头名称

### 2.5 QueryVersionExtractor

```go
type QueryVersionExtractor struct {
    ParamName string
}
```

- **职责**：从 URL 查询参数中提取版本号
- **默认参数名**：`version`
- **可定制**：通过 `NewQueryVersionExtractorWithName()` 自定义参数名称

### 2.6 VersionRouter

```go
type VersionRouter struct {
    mu              sync.RWMutex
    handlers        map[Version]http.HandlerFunc
    requestConvs    map[converterPair]RequestConverter
    responseConvs   map[converterPair]ResponseConverter
    extractors      []VersionExtractor
    defaultVersion  Version
}
```

- **职责**：核心路由器，负责版本提取、路由分发和转换
- **核心功能**：
  - 注册不同版本的处理器
  - 注册版本间的请求/响应转换器
  - 根据配置版本提取策略和优先级
  - 自动处理版本转换逻辑

**主要方法**：
- `NewVersionRouter() *VersionRouter`: 创建新的版本路由器
- `RegisterHandler(v Version, h http.HandlerFunc)`: 注册版本处理器
- `RegisterRequestConverter(from, to Version, conv RequestConverter)`: 注册请求转换器
- `RegisterResponseConverter(from, to Version, conv ResponseConverter)`: 注册响应转换器
- `SetExtractors(extractors ...VersionExtractor)`: 设置版本提取器（决定优先级）
- `SetDefaultVersion(v Version)`: 设置默认版本
- `ExtractVersion(r *http.Request) (Version, *http.Request, error)`: 提取版本
- `ServeHTTP(w http.ResponseWriter, r *http.Request)`: 处理 HTTP 请求

### 2.7 转换器类型

```go
type RequestConverter func(r *http.Request) (*http.Request, error)
type ResponseConverter func(statusCode int, header http.Header, body []byte) (int, http.Header, []byte, error)
```

- **RequestConverter**: 将旧版本请求转换为新版本请求格式
- **ResponseConverter**: 将新版本响应转换为旧版本响应格式

### 2.8 版本格式校验

```go
func IsValidVersion(v Version) bool
```

- **职责**：校验版本号格式是否合法
- **合法格式**：`v` 前缀后跟纯数字，例如 `v1`, `v2`, `v10`, `v123`
- **不合法格式**：`v1.0`, `v1-beta`, `V1`, `1`, `va` 等

所有版本提取策略在提取到版本号后，都会通过 `IsValidVersion()` 进行统一校验，确保三种策略对合法版本的定义一致。

## 3. 版本策略优先级与组合规则

### 3.1 默认优先级

默认情况下，版本提取器按以下优先级顺序尝试：

1. **URL 路径策略 (PathStrategy) - 最高优先级
2. **HTTP 请求头策略 (HeaderStrategy) - 中等优先级
3. **查询参数策略 (QueryStrategy) - 最低优先级

### 3.2 优先级规则

- 提取器按顺序决定优先级，排在前面的提取器优先匹配
- 一旦某个提取器成功提取到版本号，后续提取器不再尝试
- 如果所有提取器都未能提取到版本号：
  - 如果设置了默认版本 (`defaultVersion`)，则使用默认版本
  - 否则返回 `ErrVersionNotFound` 错误

### 3.3 自定义优先级示例

```go
// 只使用请求头策略
vr.SetExtractors(NewHeaderVersionExtractor())

// 自定义优先级：查询参数 > 请求头 > 路径
vr.SetExtractors(
    NewQueryVersionExtractor(),
    NewHeaderVersionExtractor(),
    NewPathVersionExtractor(),
)
```

### 3.4 版本匹配规则

- 当请求的版本与最新版本相同时，直接调用对应版本的处理器
- 当请求的版本早于最新版本时：
  1. 检查是否存在从请求版本到最新版本的请求转换器
  2. **如果请求转换器存在**：
     - 使用 `RequestConverter` 将请求从旧版本转换为最新版本格式
     - 调用最新版本的处理器处理请求
     - 使用 `ResponseConverter` 将响应从最新版本转换回旧版本格式
     - 返回转换后的响应给客户端
  3. **如果请求转换器不存在（优雅降级）**：
     - 直接调用请求版本对应的处理器
     - 不进行版本转换，响应直接返回
- 响应转换器缺失处理：
  - 如果请求转换器存在但响应转换器不存在，返回 `ErrConverterNotFound` 错误 (HTTP 500)
  - 避免新版本格式的响应直接透传给旧版本客户端，防止客户端解析失败且错误难以定位

### 3.5 版本格式校验规则

所有三种版本提取策略共享同一套版本格式校验规则：
- 合法格式：`v` 前缀 + 纯数字（如 `v1`, `v2`, `v10`, `v123`）
- 格式不合法时返回 `ErrInvalidVersionFormat` 错误 (HTTP 400)
- 默认版本格式不合法时同样返回 `ErrInvalidVersionFormat` 错误

**注意**：对于路径策略，只有符合 `v\d+` 格式的路径前缀才会被识别为版本号；不符合格式的前缀（如 `/va/users`）会被视为普通路径，继续尝试下一个提取器。

## 4. 使用示例

### 4.1 基本使用 - URL 路径版本化

```go
package main

import (
    "net/http"
    "solocoder-go/internal/apiver"
)

func main() {
    vr := apiver.NewVersionRouter()

    // 注册 v1 处理器
    vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"user":"v1"}`))
    })

    // 注册 v2 处理器
    vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"username":"v2","meta":{}}`))
    })

    // 请求 /v1/users -> v1 处理器
    // 请求 /v2/users -> v2 处理器
    http.ListenAndServe(":8080", vr)
}
```

### 4.2 请求头版本化

```go
vr := apiver.NewVersionRouter()
vr.SetExtractors(apiver.NewHeaderVersionExtractor())

vr.RegisterHandler("v1", handlerV1)
vr.RegisterHandler("v2", handlerV2)

// 客户端请求：
// GET /users
// API-Version: v1
```

### 4.3 查询参数版本化

```go
vr := apiver.NewVersionRouter()
vr.SetExtractors(apiver.NewQueryVersionExtractorWithName("api_version"))

// 客户端请求：
// GET /users?api_version=v1
```

### 4.4 版本自动转换

```go
vr := apiver.NewVersionRouter()

// v1 处理器（旧版本）
vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
    // 实际不会被调用，因为 v2 是最新版本
})

// v2 处理器（最新版本）
vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
    body, _ := io.ReadAll(r.Body)
    // 处理 v2 格式的请求
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    w.Write([]byte(`{"new_field":"value"}`))
})

// 注册 v1 -> v2 请求转换器
vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
    body, _ := io.ReadAll(r.Body)
    r.Body.Close()
    // 将 v1 格式 {"old_field":"value"} -> v2 格式 {"new_field":"value"}
    converted := bytes.Replace(body, []byte("old_field"), []byte("new_field"), 1)
    newReq := r.Clone(r.Context())
    newReq.Body = io.NopCloser(bytes.NewReader(converted))
    newReq.ContentLength = int64(len(converted))
    return newReq, nil
})

// 注册 v2 -> v1 响应转换器
vr.RegisterResponseConverter("v2", "v1", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
    // 将 v2 响应转换为 v1 格式
    converted := bytes.Replace(body, []byte("new_field"), []byte("old_field"), 1)
    return http.StatusOK, header, converted, nil
})

// 客户端请求 v1 时，自动转换
// 请求: {"old_field":"test"}
// 转换为 v2 格式: {"new_field":"test"}
// 处理后响应: {"new_field":"value"}
// 转换回 v1 格式: {"old_field":"value"}
```

### 4.5 组合多种策略

```go
vr := apiver.NewVersionRouter()

// 使用所有策略，优先级：路径 > 请求头 > 查询参数
vr.SetExtractors(
    apiver.NewPathVersionExtractor(),
    apiver.NewHeaderVersionExtractorWithName("X-API-Version"),
    apiver.NewQueryVersionExtractor(),
)

// 设置默认版本为 v1
vr.SetDefaultVersion("v1")

vr.RegisterHandler("v1", handlerV1)
vr.RegisterHandler("v2", handlerV2)
```

### 4.6 获取剥离后的路径

当使用路径版本策略时，版本前缀会从路径中剥离，可以通过上下文获取剥离后的路径：

```go
vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
    // 请求路径为 /v1/users
    // r.URL.Path 自动变为 /users
    // 也可以通过上下文获取
    if path, ok := apiver.StrippedPathFromContext(r.Context()); ok {
        fmt.Println("Stripped path:", path) // 输出: /users
    }
})
```

## 5. 错误处理

模块定义了以下错误变量：

| 错误 | 说明 | HTTP 状态码
|------|------|-------------
| `ErrVersionNotFound` | 无法从请求中提取版本号，且未设置默认版本 | 400
| `ErrHandlerNotFound` | 请求的版本没有注册处理器 | 404
| `ErrNoVersionExtractor` | 没有配置任何版本提取器 | 400
| `ErrInvalidVersionFormat` | 版本格式无效（必须为 `v` + 纯数字格式） | 400
| `ErrConverterNotFound` | 转换器未找到 | 请求转换器缺失时降级，响应转换器缺失时返回 500

### 5.1 转换器缺失处理逻辑

- **请求转换器缺失**：当请求版本对应的处理器已注册，但缺少请求版本到最新版本的请求转换器时，**自动优雅降级**，直接调用请求版本的处理器处理请求。此场景下响应转换器是否存在不影响行为，因为不会进入版本转换流程。
- **响应转换器缺失**：当请求转换器存在但响应转换器缺失时，返回 `ErrConverterNotFound` 错误 (HTTP 500)。这是为了避免新版本格式的响应直接透传给旧版本客户端，导致客户端解析失败且错误难以定位。

两种场景的处理结果截然不同：
- 请求转换器缺失 → HTTP 200 + 请求版本处理器的原始响应（优雅降级）
- 响应转换器缺失 → HTTP 500 + 错误信息（严格报错）

## 6. 并发安全

`VersionRouter` 的所有公共方法都是并发安全的，内部使用 `sync.RWMutex` 保证多 goroutine 安全访问。

### 6.1 转换器查找竞态防护

`ServeHTTP` 方法中对转换器的查找采用"查询即使用"模式，避免重复查找带来的竞态窗口：

- **修复前**：先调用 `GetRequestConverter` 检查转换器是否存在，再通过 `convertRequest` 间接调用 `GetRequestConverter` 获取转换器执行转换。两次调用之间存在时间窗口，期间转换器可能被并发删除，导致本应降级处理的请求返回 500 错误。
- **修复后**：`GetRequestConverter` 只调用一次，保存转换器函数引用后直接使用。无论后续是否有并发的转换器删除操作，已获取的函数引用始终有效。`GetResponseConverter` 同理。

### 6.2 responseCapture.WriteHeader 幂等性

`responseCapture` 的 `WriteHeader` 方法实现了幂等保护，符合标准 `http.ResponseWriter` 只允许调用一次 `WriteHeader` 的规范：

- 内部维护 `wroteHeader` 布尔标志
- 首次调用时记录状态码并设置标志
- 后续调用直接忽略，防止状态码被错误覆盖

## 7. 测试覆盖

单元测试覆盖了以下场景：

- 版本比较和解析
- 三种版本提取策略的正常和边界情况
- 版本格式一致性校验
- 版本提取优先级
- 处理器和转换器的注册与获取
- 版本排序和最新版本获取
- 完整的请求处理流程（含转换）
- 转换器缺失时的优雅降级
- 响应转换器缺失时的错误处理
- 错误处理分支
- 并发访问安全性
- 响应捕获和转换
- 上下文路径传递
- WriteHeader 幂等性保护
- 转换器删除后 ServeHTTP 竞态安全性
- 并发转换器删除场景下的降级行为

运行测试：
```bash
go test ./internal/apiver/ -v
```
