# HTTP 内容协商模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [Accept 头解析流程](#4-accept-头解析流程)
5. [格式选择决策流程](#5-格式选择决策流程)
6. [406 Not Acceptable 响应处理](#6-406-not-acceptable-响应处理)
7. [序列化机制](#7-序列化机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)

---

## 1. 模块概述

HTTP 内容协商（Content Negotiation）模块实现了 RFC 7231 定义的服务端驱动内容协商机制。模块通过解析 HTTP 请求的 `Accept` 请求头，结合服务器支持的响应格式列表，自动选择客户端偏好度最高的响应格式，并将响应对象序列化为对应的字节流。

**包路径**: `internal/contentneg`

**设计目标**:
- 标准兼容：严格遵循 HTTP/1.1 内容协商规范（RFC 7231 Section 5.3.2）
- 精确解析：完整解析 Accept 头的媒体类型、质量因子 q 值和扩展参数
- 智能排序：按 q 值降序 → 匹配精度降序 → 出现顺序升序的多级优先级算法选择最优格式
- 多格式支持：内置 JSON、XML、Protobuf 三种主流序列化格式
- 可扩展性：通过注册机制支持自定义响应格式
- 优雅降级：无匹配格式时返回标准 406 Not Acceptable 响应并列出支持格式
- 完整错误处理：覆盖所有边界条件和异常分支
- HTTP 友好：提供 `net/http` 标准库兼容的响应写入接口

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Accept 头解析 | 解析 Accept 头中的媒体类型列表、质量因子 q 值、扩展参数 |
| 通配符匹配 | 支持 `*/*`（所有类型）和 `application/*`（子类型通配符）匹配 |
| 质量因子计算 | 按 q 值从高到低排序，范围 [0.0, 1.0]，默认 q=1.0 |
| 精度优先 | 相同 q 值时精确匹配（`application/json`）> 子类型通配（`application/*`）> 全局通配（`*/*`） |
| 顺序优先 | q 值和精度都相同时，按 Accept 头中出现的顺序靠前优先 |
| 格式注册 | 支持运行时动态注册自定义序列化格式 |
| 格式序列化 | 内置 JSON、XML、Protobuf 三种格式的序列化函数 |
| 406 响应 | 无可接受格式时返回 406 状态码并列出服务器支持的格式 |
| HTTP 集成 | 提供与 `net/http` 标准库兼容的响应写入方法 |
| 默认回退 | 支持无可匹配格式时回退到指定默认格式 |

---

## 3. 核心结构体与职责

### 3.1 MediaType

**职责**: 表示 Accept 头中解析出的单个媒体类型条目

```go
type MediaType struct {
    Type       string            // 主类型（如 application、text）
    Subtype    string            // 子类型（如 json、xml）
    Quality    float64           // 质量因子 q 值，范围 [0.0, 1.0]
    Params     map[string]string // 扩展参数（如 charset=utf-8）
    Raw        string            // 原始条目字符串
    OrderIndex int               // 在 Accept 头中的出现顺序（0-based）
}
```

| 方法 | 说明 |
|------|------|
| `FullType() string` | 返回完整媒体类型字符串，如 `"application/json"` |
| `IsWildcardAll() bool` | 是否为全局通配符 `*/*` |
| `IsWildcardSubtype() bool` | 是否为子类型通配符（如 `application/*`） |
| `Matches(ct string) bool` | 判断是否匹配给定的内容类型，支持大小写不敏感比较 |

### 3.2 Format

**职责**: 定义服务器支持的单个响应格式

```go
type Format struct {
    ContentType string                             // MIME 内容类型
    Marshal     func(v interface{}) ([]byte, error) // 序列化函数
}
```

**约束**:
- `ContentType` 不能为空，应使用标准 MIME 类型
- `Marshal` 不能为空，将任意 Go 值序列化为对应格式的字节流

### 3.3 Negotiator

**职责**: 内容协商器核心，管理服务器支持的格式列表并执行协商决策

```go
type Negotiator struct {
    formats map[string]*Format // 按 ContentType 索引的格式映射
}
```

| 方法 | 说明 |
|------|------|
| `NewNegotiator() *Negotiator` | 创建协商器，预注册 JSON、XML、Protobuf 三种格式 |
| `RegisterFormat(f *Format) error` | 注册自定义响应格式 |
| `SupportedFormats() []string` | 返回所有支持格式的 ContentType 列表（已排序） |
| `GetFormat(ct string) (*Format, bool)` | 按 ContentType 查找格式（大小写不敏感） |
| `Negotiate(acceptHeader string) (*Format, error)` | 根据 Accept 头选择最优格式 |
| `NegotiateRequest(r *http.Request) (*Format, error)` | 从 HTTP 请求中提取 Accept 头并协商 |
| `NegotiateWithDefault(accept, defaultCT string) (*NegotiateResult, error)` | 协商失败时回退到指定默认格式 |
| `WriteResponse(w, r, status, data) error` | 自动协商格式并写入 HTTP 响应 |
| `WriteResponseWithFormat(w, format, status, data) error` | 使用指定格式写入 HTTP 响应 |
| `WriteNotAcceptable(w) error` | 写入标准 406 Not Acceptable 响应 |

### 3.4 NegotiateResult

**职责**: 封装带默认回退的协商结果

```go
type NegotiateResult struct {
    Format      *Format // 选中的格式对象
    ContentType string  // 选中格式的 ContentType
}
```

### 3.5 NotAcceptableResponse

**职责**: 406 响应体结构，包含错误信息和支持格式列表

```go
type NotAcceptableResponse struct {
    Status  string   `json:"status" xml:"status"`
    Code    int      `json:"code" xml:"code"`
    Message string   `json:"message" xml:"message"`
    Formats []string `json:"supported_formats" xml:"supported_formats>format"`
}
```

---

## 4. Accept 头解析流程

### 4.1 解析步骤

```
Accept 头原始字符串
        ↓
按逗号分割为多个条目（entries）
        ↓
对每个条目：
  ├─ 去除首尾空白
  ├─ 按分号分割为 [媒体范围, 参数1, 参数2, ...]
  ├─ 第一部分为 mediaRange，如 "application/json"
  ├─ 处理后续参数：
  │    ├─ 参数名 = "q" → 解析为 Quality 浮点值
  │    │    ├─ 解析失败 → 默认 q=1.0
  │    │    └─ 值范围裁剪到 [0.0, 1.0]
  │    └─ 其他参数 → 存入 Params map
  ├─ 将 mediaRange 按 "/" 分割为 Type 和 Subtype
  │    ├─ 缺失 Subtype → Subtype="*"
  │    ├─ Type 为空 → Type="*"
  │    └─ Subtype 为空 → Subtype="*"
  └─ 记录 OrderIndex（条目在原始列表中的索引）
        ↓
过滤空条目
        ↓
结果为空时，添加默认值 */*;q=1.0
        ↓
返回 []*MediaType
```

### 4.2 解析规则详解

**媒体类型标准化**:
- 所有 Type 和 Subtype 转换为小写
- 去除媒体类型各部分的首尾空白

**质量因子 q**:
- 范围约束：q < 0 → 0.0；q > 1 → 1.0
- 解析失败（非浮点数）→ 默认 q=1.0
- q=0 表示明确拒绝该媒体类型，协商时自动跳过

**顺序索引**:
- `OrderIndex` 是原始条目的索引，不随后续排序改变
- 用于 q 值和匹配精度相同时的「出现顺序优先」规则

**空值兜底**:
- Accept 头为空字符串 → 视为 `*/*`（接受所有格式）
- Accept 头解析后无有效条目 → 视为 `*/*`

### 4.3 解析示例

**输入**:
```
Accept: text/html, application/json;q=0.9;charset=utf-8, application/xml;q=0.7, */*;q=0.5
```

**输出**（[]*MediaType）:
| 索引 | Type | Subtype | Quality | Params | OrderIndex |
|------|------|---------|---------|--------|------------|
| 0 | text | html | 1.0 | {} | 0 |
| 1 | application | json | 0.9 | {"charset":"utf-8"} | 1 |
| 2 | application | xml | 0.7 | {} | 2 |
| 3 | * | * | 0.5 | {} | 3 |

---

## 5. 格式选择决策流程

### 5.1 候选收集阶段

```
遍历每个解析出的 MediaType（跳过 q ≤ 0 的条目）
        ↓
    遍历服务器支持的每个 Format
        ↓
    MediaType.Matches(Format.ContentType)?
        ├─ 否 → 跳过
        └─ 是 → 生成候选 rankedFormat
                ├─ contentType: Format.ContentType
                ├─ format: Format 引用
                ├─ quality: MediaType.Quality
                ├─ orderIndex: MediaType.OrderIndex
                └─ matchLevel: 匹配精度等级
                       ├─ 精确匹配（如 application/json）→ 2
                       ├─ 子类型通配（如 application/*）→ 1
                       └─ 全局通配（*/*）→ 0
```

### 5.2 候选排序阶段

**排序键优先级从高到低**:

| 优先级 | 排序键 | 排序方向 | 说明 |
|--------|--------|---------|------|
| 1 | quality | 降序 | q 值越高越优先 |
| 2 | matchLevel | 降序 | 匹配越精确越优先 |
| 3 | orderIndex | 升序 | 在 Accept 头中出现越早越优先 |
| 4 | contentType | 字典序升序 | 以上全相同时按字母排序，保证确定性 |

### 5.3 选择阶段

```
候选列表为空？
    ├─ 是 → 返回 ErrNoAcceptableFormat
    └─ 否 → 返回排序后第一个候选的 Format
```

### 5.4 决策示例

**场景 1: q 值主导**
```
Accept: application/json;q=0.5, application/xml;q=0.9
排序: xml(q=0.9) → json(q=0.5)
结果: application/xml
```

**场景 2: 相同 q，精度优先**
```
Accept: application/xml;q=0.8, */*;q=0.8
候选: xml(level=2,q=0.8), json(level=0,q=0.8), protobuf(level=0,q=0.8), xml(level=0,q=0.8)
排序: xml(精确) → json(通配) → protobuf(通配) → xml(通配)
结果: application/xml
```

**场景 3: 相同 q 和精度，顺序优先**
```
Accept: application/json;q=0.8, application/xml;q=0.8
候选: json(q=0.8, order=0), xml(q=0.8, order=1)
排序: json → xml
结果: application/json
```

**场景 4: 高 q 通配 > 低 q 精确**
```
Accept: application/xml;q=0.5, */*;q=0.8
候选: xml(q=0.5, level=2), json(q=0.8, level=0), protobuf(q=0.8, level=0), xml(q=0.8, level=0)
排序: json(q=0.8) → protobuf(q=0.8) → xml(q=0.8) → xml(q=0.5)
结果: application/json（字母序优先）
```

---

## 6. 406 Not Acceptable 响应处理

### 6.1 触发条件

当 `Negotiate` 方法返回 `ErrNoAcceptableFormat` 时，即 Accept 头中没有任何媒体类型（排除 q=0 后）与服务器支持的格式匹配。

### 6.2 响应规范

**HTTP 状态码**: `406 Not Acceptable`

**响应头**:
```
Content-Type: application/json
```

**响应体结构**（JSON 格式）:
```json
{
  "status": "Not Acceptable",
  "code": 406,
  "message": "No acceptable representation found for the requested resource.",
  "supported_formats": [
    "application/json",
    "application/protobuf",
    "application/xml"
  ]
}
```

### 6.3 处理流程

```
WriteResponse 调用
        ↓
调用 NegotiateRequest
        ↓
返回 ErrNoAcceptableFormat？
        ├─ 否 → 使用选中的格式正常序列化响应
        └─ 是 → 调用 WriteNotAcceptable
                 ├─ 调用 SupportedFormats() 获取支持格式列表
                 ├─ 构造 NotAcceptableResponse 对象
                 ├─ json.MarshalIndent 序列化为 JSON
                 │    └─ 序列化失败时使用预置的极简 JSON 字符串兜底
                 ├─ 设置 Content-Type: application/json
                 ├─ 写入 406 状态码
                 └─ 写入响应体
```

### 6.4 406 响应的特殊性质

- **不计入正常序列化流程**: 406 响应总是使用 JSON 格式，不受客户端 Accept 头影响
- **自包含**: 响应体中列出所有支持的格式，客户端可据此重试请求
- **兜底保护**: JSON 序列化失败时仍能返回有效的极简 JSON 字符串

---

## 7. 序列化机制

### 7.1 内置格式与内容类型

| 格式 | ContentType | 序列化函数 | 依赖 |
|------|-------------|-----------|------|
| JSON | `application/json` | `marshalJSON` | 标准库 `encoding/json` |
| XML | `application/xml` | `marshalXML` | 标准库 `encoding/xml` |
| Protobuf | `application/protobuf` | `marshalProtobuf` | 项目内 `internal/serialize` 模块 |

### 7.2 各格式序列化实现

**JSON**:
```go
func marshalJSON(v interface{}) ([]byte, error) {
    return json.Marshal(v)
}
```
- 使用标准库 `encoding/json`
- 支持所有可 JSON 序列化的 Go 类型
- 结构体字段支持 `json` 标签

**XML**:
```go
func marshalXML(v interface{}) ([]byte, error) {
    return xml.Marshal(v)
}
```
- 使用标准库 `encoding/xml`
- 支持所有可 XML 序列化的 Go 类型
- 结构体字段支持 `xml` 标签

**Protobuf**:
```go
func marshalProtobuf(v interface{}) ([]byte, error) {
    opts := serialize.DefaultOptions()
    opts.ZeroCopy = false
    ser := serialize.NewProtoBufSerializer()
    return ser.Marshal(v, opts)
}
```
- 使用项目内置的 `ProtoBufSerializer`
- 仅支持 struct 类型（符合 Protobuf 约定）
- 结构体字段需通过 `serialize` 标签指定 protobuf 字段号
- 禁用 ZeroCopy 选项以确保返回字节流的独立所有权

### 7.3 自定义格式注册

通过 `RegisterFormat` 方法可在运行时添加自定义格式：

```go
n := contentneg.NewNegotiator()
n.RegisterFormat(&contentneg.Format{
    ContentType: "application/yaml",
    Marshal: func(v interface{}) ([]byte, error) {
        return yaml.Marshal(v)
    },
})
```

---

## 8. 使用示例

### 8.1 基本用法：HTTP Handler 中自动协商

```go
package main

import (
    "net/http"
    "solocoder-go/internal/contentneg"
)

type User struct {
    ID   int    `json:"id" xml:"id"`
    Name string `json:"name" xml:"name"`
}

var negotiator = contentneg.NewNegotiator()

func getUserHandler(w http.ResponseWriter, r *http.Request) {
    user := &User{ID: 1, Name: "Alice"}

    // 自动根据 Accept 头选择格式，不匹配时返回 406
    err := negotiator.WriteResponse(w, r, http.StatusOK, user)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}
```

**请求示例**:
```
GET /user
Accept: application/xml
```
**响应**: 200 OK，`Content-Type: application/xml`，XML 格式的 User 数据

**请求示例**:
```
GET /user
Accept: text/html
```
**响应**: 406 Not Acceptable，`Content-Type: application/json`，响应体列出支持格式

### 8.2 手动协商与默认回退

```go
func handler(w http.ResponseWriter, r *http.Request) {
    acceptHeader := r.Header.Get("Accept")

    // 协商失败时回退到 JSON
    result, _ := negotiator.NegotiateWithDefault(acceptHeader, contentneg.ContentTypeJSON)

    // 使用协商结果（或回退后的 JSON）写入响应
    negotiator.WriteResponseWithFormat(w, result.Format, http.StatusOK, data)
}
```

### 8.3 仅使用协商逻辑（非 HTTP 场景）

```go
func serializeForClient(acceptHeader string, data interface{}) (ct string, body []byte, err error) {
    format, err := negotiator.Negotiate(acceptHeader)
    if err != nil {
        return "", nil, err
    }
    body, err = format.Marshal(data)
    if err != nil {
        return "", nil, err
    }
    return format.ContentType, body, nil
}
```

### 8.4 注册自定义格式

```go
func init() {
    negotiator := contentneg.NewNegotiator()

    // 注册 YAML 格式
    negotiator.RegisterFormat(&contentneg.Format{
        ContentType: "application/yaml",
        Marshal: func(v interface{}) ([]byte, error) {
            return yaml.Marshal(v)
        },
    })

    // 注册 MessagePack 格式
    negotiator.RegisterFormat(&contentneg.Format{
        ContentType: "application/msgpack",
        Marshal: func(v interface{}) ([]byte, error) {
            return msgpack.Marshal(v)
        },
    })
}
```

### 8.5 获取和检查支持格式

```go
formats := negotiator.SupportedFormats()
// 返回: ["application/json", "application/protobuf", "application/xml"]

format, ok := negotiator.GetFormat("application/json")
if ok {
    // format 可用于序列化
}
```

### 8.6 直接写入 406 响应

```go
func strictHandler(w http.ResponseWriter, r *http.Request) {
    acceptHeader := r.Header.Get("Accept")
    format, err := negotiator.Negotiate(acceptHeader)
    if err != nil {
        // 业务逻辑决定在特定场景下直接返回 406
        negotiator.WriteNotAcceptable(w)
        return
    }
    negotiator.WriteResponseWithFormat(w, format, http.StatusOK, data)
}
```

---

## 9. 错误定义

| 错误 | 值 | 说明 | 触发场景 |
|------|----|------|---------|
| `ErrNoAcceptableFormat` | `contentneg: no acceptable format found` | 无可接受的响应格式 | Accept 头中无匹配类型，或匹配类型 q 值均为 0 |
| `ErrNilResponseWriter` | `contentneg: nil response writer` | 响应写入器为 nil | 调用 WriteResponse 等方法时传入 nil |
| `ErrSerialization` | `contentneg: serialization failed` | 序列化失败 | 格式的 Marshal 函数返回错误时包装此错误 |

**RegisterFormat 额外错误**（未导出为哨兵错误，直接返回 fmt.Errorf）:
- `contentneg: nil format` - 注册的 Format 为 nil
- `contentneg: empty content type` - Format.ContentType 为空字符串
- `contentneg: nil marshal function` - Format.Marshal 为 nil

---

## 10. 完整决策树

```
接收到 HTTP 请求
        ↓
提取 Accept 请求头
        ↓
ParseAccept(acceptHeader)
        ├─ 空值 → [MediaType{*/*, q=1.0}]
        └─ 非空 → 解析为 []*MediaType
        ↓
收集候选 rankedFormat：
  对每个 MediaType（q > 0）:
    对每个支持的 Format:
      MediaType.Matches(Format)？
        └─ 是 → 计算 matchLevel，加入候选
        ↓
候选为空？
  ├─ 是 → 返回 406 Not Acceptable 响应
  │       ├─ 状态码: 406
  │       ├─ Content-Type: application/json
  │       └─ 响应体: 状态、错误码、消息、支持格式列表
  └─ 否 → 排序候选
            ├─ 第 1 级: quality 降序
            ├─ 第 2 级: matchLevel 降序
            ├─ 第 3 级: orderIndex 升序
            └─ 第 4 级: contentType 升序（确定性兜底）
        ↓
取排序后的第一个 Format
        ↓
调用 Format.Marshal(data)
        ├─ 失败 → 返回 ErrSerialization 包装的错误
        └─ 成功 → 设置响应头 Content-Type
                  设置 HTTP 状态码
                  写入序列化后的字节流
```
