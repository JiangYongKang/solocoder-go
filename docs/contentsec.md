# 内容安全策略引擎模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [XSS 过滤器详解](#4-xss-过滤器详解)
5. [输入净化规则详解](#5-输入净化规则详解)
6. [输出编码策略详解](#6-输出编码策略详解)
7. [敏感数据脱敏机制](#7-敏感数据脱敏机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全](#10-并发安全)
11. [最佳实践](#11-最佳实践)

---

## 1. 模块概述

内容安全策略引擎（Content Security Engine）是一个综合性的 Web 应用安全防护模块，提供从输入检测、内容净化到输出编码、敏感数据脱敏的全链路安全保障。模块设计遵循纵深防御原则，在数据生命周期的不同阶段应用相应的安全策略，有效防御 XSS 攻击、SQL 注入、敏感信息泄露等常见安全威胁。

**包路径**: `internal/contentsec`

**设计目标**:
- 提供可扩展的 XSS 攻击检测机制，支持自定义规则注册
- 实现灵活的 HTML 输入净化，支持标签和属性的白名单配置
- 支持多种输出上下文的编码策略，防止注入攻击
- 提供敏感数据自动脱敏能力，保护个人隐私信息
- 保证并发安全，支持高并发场景下的稳定运行

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| XSS 攻击检测 | 内置 9 种常见 XSS 攻击模式检测，支持自定义过滤器注册，返回危险内容位置和类型 |
| HTML 输入净化 | 基于白名单的标签和属性过滤，支持特殊字符 HTML 实体编码，HTML 注释剥离 |
| 输出上下文编码 | 支持 HTML、JavaScript、URL、CSS 四种上下文的专用编码策略 |
| 敏感数据脱敏 | 内置身份证号、手机号、银行卡号、邮箱四种模式，支持自定义模式注册 |
| 统一安全引擎 | 提供 ContentSecurityEngine 统一入口，整合所有安全功能 |

---

## 3. 核心结构体与职责

### 3.1 XSSViolation

XSS 违规记录结构体，表示检测到的一次 XSS 攻击尝试。

```go
type XSSViolation struct {
    Position int    // 违规内容在输入字符串中的起始位置
    Type     string // 违规类型（如 script_tag、event_handler 等）
    Content  string // 匹配到的违规内容
}
```

**职责**:
- 记录违规内容的精确位置，便于日志审计和问题定位
- 标识违规类型，帮助安全人员分析攻击向量
- 保存原始违规内容，支持事后取证分析

### 3.2 XSSDetector

XSS 检测器，负责管理过滤器注册并执行检测。

```go
type XSSDetector struct {
    mu      sync.RWMutex
    filters map[string]XSSFilterFunc
}
```

**职责**:
- 维护过滤器注册表，支持过滤器的动态注册和注销
- 内置 9 种常见 XSS 攻击模式的检测规则
- 协调所有过滤器执行检测，聚合并排序检测结果
- 保证并发安全，支持多协程同时检测

### 3.3 SanitizerConfig

HTML 净化配置结构体，定义净化规则。

```go
type SanitizerConfig struct {
    AllowedTags       map[string]bool   // 允许保留的 HTML 标签白名单
    AllowedAttributes map[string]bool   // 允许保留的属性白名单
    EscapeCharacters  map[rune]string   // 需要转义的特殊字符及对应实体
    StripComments     bool              // 是否剥离 HTML 注释
}
```

**职责**:
- 集中管理所有净化规则，支持灵活配置
- 提供标签级和属性级的细粒度控制
- 支持自定义特殊字符转义映射
- 默认配置覆盖大部分安全场景

### 3.4 HTMLSanitizer

HTML 内容净化器，根据配置规则净化输入内容。

```go
type HTMLSanitizer struct {
    mu     sync.RWMutex
    config *SanitizerConfig
}
```

**职责**:
- 解析 HTML 标签结构，区分标签内容和文本内容
- 基于白名单过滤危险标签和属性
- 对属性值进行 HTML 实体编码
- 对文本内容中的特殊字符进行转义
- 支持配置的热更新

### 3.5 OutputContext

输出上下文枚举，标识内容将被嵌入的环境。

```go
type OutputContext int

const (
    ContextHTML       OutputContext = iota  // HTML 文档内容上下文
    ContextJavaScript                        // JavaScript 字符串上下文
    ContextURL                               // URL 查询参数上下文
    ContextCSS                               // CSS 样式上下文
)
```

**职责**:
- 抽象四种常见输出场景，指导编码策略选择
- 确保内容在不同上下文环境中的安全性
- 避免上下文混淆导致的注入漏洞

### 3.6 OutputEncoder

输出编码器，根据上下文选择正确的编码策略。

```go
type OutputEncoder struct{}
```

**职责**:
- 实现 HTML 实体编码，转义 HTML 特殊字符
- 实现 JavaScript 字符串编码，防止脚本注入
- 实现 URL 参数编码，符合 RFC 3986 规范
- 实现 CSS 编码，防止样式注入

### 3.7 MaskPattern

脱敏模式结构体，定义一种敏感数据的识别和脱敏规则。

```go
type MaskPattern struct {
    Name        string              // 模式名称
    Pattern     *regexp.Regexp      // 匹配正则表达式
    MaskFunc    func(string) string // 脱敏处理函数
    Description string              // 模式描述
}
```

**职责**:
- 定义敏感数据的识别模式（正则表达式）
- 提供脱敏处理逻辑，支持自定义脱敏格式
- 保存模式元信息，便于管理和调试

### 3.8 DataMasker

数据脱敏器，负责管理脱敏模式并执行脱敏处理。

```go
type DataMasker struct {
    mu       sync.RWMutex
    patterns map[string]*MaskPattern
}
```

**职责**:
- 维护脱敏模式注册表，支持模式的动态注册和注销
- 内置 4 种常见敏感数据的识别和脱敏模式
- 对文本内容进行扫描，自动检测并脱敏敏感信息
- 支持按指定模式集合进行脱敏，而非全部模式

### 3.9 ContentSecurityEngine

内容安全引擎，整合所有安全功能的统一入口。

```go
type ContentSecurityEngine struct {
    XSSDetector *XSSDetector    // XSS 检测器
    Sanitizer   *HTMLSanitizer  // HTML 净化器
    Encoder     *OutputEncoder  // 输出编码器
    Masker      *DataMasker     // 数据脱敏器
}
```

**职责**:
- 提供一站式安全处理接口，简化使用方式
- 协调 XSS 检测和内容净化的联合执行
- 支持完整的安全处理流水线：检测 → 净化 → 脱敏 → 编码
- 各子模块保持独立，可单独使用

---

## 4. XSS 过滤器详解

### 4.1 内置过滤器列表

模块内置了 9 种常见 XSS 攻击模式的检测规则：

| 过滤器名称 | 违规类型 | 检测内容 |
|-----------|---------|---------|
| `script_tag` | script_tag | `<script>` 标签及其内容 |
| `event_handler` | event_handler | 内联事件处理器（如 `onclick=`、`onerror=` 等） |
| `javascript_protocol` | javascript_protocol | `javascript:` 伪协议（在 href/src/action/data 属性中） |
| `iframe_tag` | iframe_tag | `<iframe>` 标签 |
| `object_embed_tag` | object_embed_tag | `<object>`、`<embed>`、`<applet>` 标签 |
| `form_tag` | form_tag | `<form>` 标签（防止表单钓鱼） |
| `eval_expression` | eval_expression | `eval()` 函数调用 |
| `vbscript_protocol` | vbscript_protocol | `vbscript:` 伪协议 |
| `data_uri_html` | data_uri_html | `data:text/html` URI（可用于 XSS） |

### 4.2 过滤器注册机制

**注册方式 1 - 自定义函数过滤器**:
```go
detector.RegisterFilter("custom_filter", func(input string) []XSSViolation {
    // 自定义检测逻辑
    return violations
})
```

**注册方式 2 - 正则表达式模式过滤器**:
```go
detector.RegisterPatternFilter(
    "sql_injection",           // 过滤器名称
    `(?i)\b(union|select|drop|insert)\b`,  // 正则模式
    "sql_keyword"              // 违规类型
)
```

### 4.3 检测执行流程

```
Detect(input)
    │
    ├─→ 空输入检查：为空直接返回 nil
    │
    ├─→ 读取锁保护下的过滤器列表
    │
    ├─→ 并发安全地逐个执行过滤器
    │       ├─→ 每个过滤器独立扫描输入
    │       └─→ 收集各自的违规记录
    │
    ├─→ 聚合所有违规记录
    │
    └─→ 按 Position 字段升序排序后返回
```

---

## 5. 输入净化规则详解

### 5.1 默认白名单配置

**默认允许的 HTML 标签**（共 28 种）：
`a, b, br, code, div, em, h1-h6, hr, i, img, li, ol, p, pre, span, strong, table, tbody, td, th, thead, tr, u, ul`

**默认允许的属性**（共 12 种）：
`alt, class, colspan, href, id, rel, rowspan, src, target, title, width, height`

**默认转义的特殊字符**：

| 字符 | 转义为 | 说明 |
|-----|-------|-----|
| `<` | `&lt;` | 小于号 |
| `>` | `&gt;` | 大于号 |
| `&` | `&amp;` | 和号 |
| `"` | `&quot;` | 双引号 |
| `'` | `&#39;` | 单引号 |

### 5.2 净化处理流程

```
Sanitize(input)
    │
    ├─→ 空输入检查：为空直接返回空字符串
    │
    ├─→ 步骤 1：HTML 注释剥离（StripComments=true 时）
    │       └─→ 正则移除 <!-- ... --> 注释块
    │
    ├─→ 步骤 2：逐字符解析 HTML 结构
    │       │
    │       ├─→ 遇到 '<' → 进入标签处理
    │       │       ├─→ 提取完整标签字符串
    │       │       ├─→ 解析标签名并检查白名单
    │       │       │   ├─→ 不在白名单 → 丢弃整个标签
    │       │       │   └─→ 在白名单 → 继续处理属性
    │       │       ├─→ 解析属性列表并检查属性白名单
    │       │       │   ├─→ 不在白名单 → 丢弃该属性
    │       │       │   └─→ 在白名单 → 对属性值进行 HTML 编码后保留
    │       │       └─→ 重建安全的标签
    │       │
    │       └─→ 遇到普通文本 → 进行特殊字符转义
    │
    └─→ 返回净化后的完整内容
```

### 5.3 自闭合标签识别

以下标签会被自动识别为自闭合标签，无需闭合标签：
`br, hr, img, input, meta, link`

---

## 6. 输出编码策略详解

### 6.1 四种编码策略适用场景

| 策略名称 | 枚举值 | 适用场景 | 处理目标 |
|---------|-------|---------|---------|
| **HTML 实体编码** | `ContextHTML` | 内容将被嵌入 HTML 文档的文本节点或属性值中 | 转义所有 HTML 特殊字符，防止标签注入 |
| **JavaScript 编码** | `ContextJavaScript` | 内容将被嵌入 JavaScript 代码的字符串字面量中 | 转义引号、反斜杠、控制字符及 HTML 边界字符，防止脚本注入 |
| **URL 参数编码** | `ContextURL` | 内容将被用作 URL 的查询参数值 | 按 RFC 3986 编码非安全字符，防止参数注入 |
| **CSS 编码** | `ContextCSS` | 内容将被嵌入 CSS 样式表或内联样式中 | 转义特殊字符为 CSS 转义序列，防止样式注入 |

### 6.2 HTML 实体编码（ContextHTML）

**编码规则**：使用 Go 标准库 `html.EscapeString`

**处理示例**：
```
输入:  <script>alert("xss") & 'test'</script>
输出:  &lt;script&gt;alert(&#34;xss&#34;) &amp; &#39;test&#39;&lt;/script&gt;
```

**典型应用场景**：
- 用户提交的评论内容展示
- 富文本编辑器内容的安全渲染
- 搜索关键字的结果高亮展示

###