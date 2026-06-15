# API 版本化路由模块

## 1. 模块功能概述

`apiver` 模块提供了一套完整的 API 版本化路由解决方案，支持多种版本策略和自动版本兼容转换。该模块位于 `internal/apiver/` 包下，主要功能包括：

- **多策略版本提取：支持 URL 路径、HTTP 请求头、查询参数三种版本指定方式
- **版本优先级管理：支持多版本处理器注册与管理
- **自动版本转换：当客户端请求旧版本时，自动进行请求和响应的格式转换
- **线程安全：所有操作都经过并发安全设计

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
  1. 使用 `RequestConverter` 将请求从旧版本转换为最新版本格式
  2. 调用最新版本的处理器处理请求
  3. 使用 `ResponseConverter` 将响应从最新版本转换回旧版本格式
  4. 返回转换后的响应给客户端
- 如果缺少必要的转换器不存在：
  - 请求转换器不存在：返回 `ErrConverterNotFound` 错误 (HTTP 400)
  - 响应转换器不存在：响应原样返回（透传）

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
| `ErrConverterNotFound` | 请求转换找不到对应的转换器 | 400
| `ErrInvalidVersionFormat` | 版本格式无效 | 400

## 6. 并发安全

`VersionRouter` 的所有公共方法都是并发安全的，内部使用 `sync.RWMutex` 保证多 goroutine 安全访问。

## 7. 测试覆盖

单元测试覆盖了以下场景：

- 版本比较和解析
- 三种版本提取策略的正常和边界情况
- 版本提取优先级
- 处理器和转换器的注册与获取
- 版本排序和最新版本获取
- 完整的请求处理流程（含转换）
- 错误处理分支
- 并发访问安全性
- 响应捕获和转换
- 上下文路径传递
```

运行测试：
```bash
go test ./internal/apiver/ -v
```
