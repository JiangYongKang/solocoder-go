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

### 6.3 JavaScript 字符串编码（ContextJavaScript）

**编码规则表**：

| 字符/类别 | 编码方式 | 示例 |
|----------|---------|-----|
| 反斜杠 `\` | `\\` | `\` → `\\` |
| 双引号 `"` | `\"` | `"` → `\"` |
| 单引号 `'` | `\'` | `'` → `\'` |
| 换行符 `\n` | `\n` | - |
| 回车符 `\r` | `\r` | - |
| 制表符 `\t` | `\t` | - |
| 退格符 `\b` | `\b` | - |
| 换页符 `\f` | `\f` | - |
| 小于号 `<` | `\u003c` | Unicode 转义 |
| 大于号 `>` | `\u003e` | Unicode 转义 |
| 和号 `&` | `\u0026` | Unicode 转义 |
| ASCII 控制字符（< 32） | `\uXXXX` | `\x00` → `\u0000` |
| 行分隔符 U+2028 | `\u2028` | 防止 JSON 注入 |
| 段分隔符 U+2029 | `\u2029` | 防止 JSON 注入 |

> **注意**：正斜杠 `/` 在 JavaScript 字符串上下文中不需要转义，以避免破坏 URL 路径和正则表达式字面量。URL `https://example.com/path` 和正则 `/\d+/g` 中的斜杠会被原样保留。

**典型应用场景**：
- 将用户数据注入 JSON 响应
- 在 `<script>` 标签内输出动态数据
- 生成动态 JavaScript 代码片段

### 6.4 URL 参数编码（ContextURL）

**编码规则**：使用 Go 标准库 `url.QueryEscape`

**处理示例**：
```
输入:  hello world&test=value
输出:  hello+world%26test%3Dvalue
```

**编码覆盖的字符**：
- 所有非 ASCII 字符（按 UTF-8 编码后百分号编码）
- 空格编码为 `+`
- 特殊字符：`&`, `=`, `?`, `#`, `/`, `:` 等

**典型应用场景**：
- 构造 URL 查询字符串参数
- 生成重定向 URL 中的动态参数
- 表单提交数据的编码

### 6.5 CSS 编码（ContextCSS）

**编码规则表**：

| 字符/类别 | 编码方式 | 示例 |
|----------|---------|-----|
| 反斜杠 `\` | `\\` | - |
| 双引号 `"` | `\"` | - |
| 单引号 `'` | `\'` | - |
| 换行符 `\n` | `\A ` | 带尾随空格 |
| 回车符 `\r` | `\D ` | 带尾随空格 |
| 制表符 `\t` | `\9 ` | 带尾随空格 |
| 小于号 `<` | `\3C ` | 十六进制 + 空格 |
| 大于号 `>` | `\3E ` | 十六进制 + 空格 |
| 和号 `&` | `\26 ` | 十六进制 + 空格 |
| 控制字符 + DEL | `\XX ` | 两位十六进制 + 空格 |

**尾随空格说明**：CSS 转义序列后的尾随空格用于分隔转义序列和后续的十六进制字符，避免歧义。

**典型应用场景**：
- 动态生成 CSS 属性值（如自定义背景色）
- 用户自定义主题样式
- 内联 `style` 属性中的动态内容

---

## 7. 敏感数据脱敏机制

### 7.1 内置脱敏模式

#### 身份证号（id_card）
- **匹配规则**: 18 位中国居民身份证号码
- **正则模式**: `\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`
- **脱敏方式**: 前 6 位 + `********` + 后 4 位
- **示例**: `110101199003076578` → `110101********6578`

#### 手机号（phone）
- **匹配规则**: 中国 11 位手机号码（号段 130-199）
- **正则模式**: `\b1[3-9]\d{9}\b`
- **脱敏方式**: 前 3 位 + `****` + 后 4 位
- **示例**: `13812345678` → `138****5678`

#### 银行卡号（bank_card）
- **匹配规则**: 16-19 位数字的银行卡号
- **正则模式**: `\b\d{16,19}\b`
- **脱敏方式**: 前 4 位 + ` **** **** ` + 后 4 位
- **示例**: `6222021234567890123` → `6222 **** **** 0123`

#### 邮箱地址（email）
- **匹配规则**: 标准邮箱格式
- **正则模式**: `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`
- **脱敏方式**: 用户名首 2 位可见，其余用 `*` 替代；域名完整保留
- **示例**:
  - `testuser@example.com` → `te******@example.com`
  - `ab@example.com` → `a*@example.com`

### 7.2 自定义脱敏模式注册

```go
masker.RegisterPattern(
    "ssn",                                           // 模式名称
    `\b\d{3}-\d{2}-\d{4}\b`,                         // 正则表达式
    func(s string) string {                          // 脱敏函数
        return "***-**-" + s[len(s)-4:]
    },
    "US Social Security Number",                     // 描述
)
```

### 7.3 脱敏执行流程

```
Mask(input)
    │
    ├─→ 空输入检查：为空直接返回空字符串
    │
    ├─→ 读取锁保护下的所有脱敏模式
    │
    ├─→ 按注册顺序逐个应用脱敏模式
    │       └─→ Pattern.ReplaceAllStringFunc(result, MaskFunc)
    │             ├─→ 正则匹配所有敏感内容
    │             └─→ 对每个匹配项调用自定义脱敏函数
    │
    └─→ 返回经过所有模式脱敏后的结果
```

---

## 8. 使用示例

### 8.1 基本使用 - XSS 检测

```go
package main

import (
    "fmt"
    "solocoder-go/internal/contentsec"
)

func main() {
    detector := contentsec.NewXSSDetector()

    userInput := `<p onclick="alert(1)">Hello<script>xss</script></p>`
    violations := detector.Detect(userInput)

    if len(violations) > 0 {
        fmt.Printf("检测到 %d 处 XSS 风险:\n", len(violations))
        for _, v := range violations {
            fmt.Printf("  位置 %d, 类型 %s: %s\n",
                v.Position, v.Type, v.Content)
        }
    } else {
        fmt.Println("输入内容安全")
    }
}
```

### 8.2 输入净化处理

```go
package main

import (
    "fmt"
    "solocoder-go/internal/contentsec"
)

func main() {
    sanitizer := contentsec.NewHTMLSanitizer()

    // 基础净化
    userInput := `<p onclick="alert(1)">Hello <script>xss</script>World</p>`
    clean := sanitizer.Sanitize(userInput)
    fmt.Println("净化后:", clean)
    // 输出: <p>Hello World</p>

    // 自定义净化配置
    customCfg := &contentsec.SanitizerConfig{
        AllowedTags:       map[string]bool{"b": true, "i": true},
        AllowedAttributes: map[string]bool{},
        EscapeCharacters:  contentsec.DefaultSanitizerConfig().EscapeCharacters,
        StripComments:     true,
    }
    customSanitizer, _ := contentsec.NewHTMLSanitizerWithConfig(customCfg)

    // 动态添加允许的标签
    customSanitizer.AddAllowedTag("span")
    customSanitizer.AddAllowedAttribute("class")
}
```

### 8.3 输出编码策略应用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/contentsec"
)

func main() {
    encoder := contentsec.NewOutputEncoder()
    userInput := `<script>alert("xss")</script>`

    // HTML 上下文
    htmlSafe := encoder.Encode(userInput, contentsec.ContextHTML)
    fmt.Printf("HTML 安全: %s\n", htmlSafe)

    // JavaScript 上下文
    jsSafe := encoder.Encode(userInput, contentsec.ContextJavaScript)
    fmt.Printf("JS 安全: %s\n", jsSafe)

    // URL 参数上下文
    urlSafe := encoder.Encode(userInput, contentsec.ContextURL)
    fmt.Printf("URL 安全: %s\n", urlSafe)

    // CSS 上下文
    cssSafe := encoder.Encode(userInput, contentsec.ContextCSS)
    fmt.Printf("CSS 安全: %s\n", cssSafe)
}
```

### 8.4 敏感数据脱敏

```go
package main

import (
    "fmt"
    "solocoder-go/internal/contentsec"
)

func main() {
    masker := contentsec.NewDataMasker()

    // 自动检测并脱敏所有敏感数据
    text := `
        姓名: 张三
        身份证: 110101199003076578
        手机: 13812345678
        银行卡: 6222021234567890123
        邮箱: zhangsan@example.com
    `
    masked := masker.Mask(text)
    fmt.Println("脱敏后:", masked)

    // 仅使用指定的脱敏模式
    phoneOnly, _ := masker.MaskWithPatterns(text, []string{"phone"})
    fmt.Println("仅脱敏手机号:", phoneOnly)

    // 注册自定义模式
    masker.RegisterPattern(
        "passport",
        `\b[A-Z]\d{8}\b`,
        func(s string) string {
            return s[:2] + "******" + s[len(s)-2:]
        },
        "Chinese passport number",
    )
}
```

### 8.5 完整安全流水线

```go
package main

import (
    "fmt"
    "solocoder-go/internal/contentsec"
)

func main() {
    engine := contentsec.NewContentSecurityEngine()

    // 用户提交的原始内容
    userInput := `<p onclick="stealCookie()">
        联系我: 手机 13812345678，邮箱 test@example.com
        <script>document.cookie</script>
    </p>`

    // 步骤 1: 检测 XSS 风险 + 净化 HTML
    sanitized, violations := engine.CheckAndSanitize(userInput)
    if len(violations) > 0 {
        fmt.Printf("⚠️  检测到 %d 处 XSS 风险，已自动净化\n", len(violations))
    }

    // 步骤 2: 脱敏敏感信息
    masked := engine.MaskSensitiveData(sanitized)

    // 步骤 3: 根据输出上下文编码
    htmlOutput := engine.SecureOutput(masked, contentsec.ContextHTML)

    fmt.Println("最终安全输出:", htmlOutput)
}
```

### 8.6 自定义 XSS 过滤规则

```go
package main

import (
    "fmt"
    "strings"
    "solocoder-go/internal/contentsec"
)

func main() {
    detector := contentsec.NewXSSDetector()

    // 方式一: 使用自定义函数
    detector.RegisterFilter("bad_word_filter", func(input string) []contentsec.XSSViolation {
        var violations []contentsec.XSSViolation
        badWords := []string{"fuck", "shit", "bitch"}
        lower := strings.ToLower(input)
        for _, word := range badWords {
            idx := strings.Index(lower, word)
            if idx != -1 {
                violations = append(violations, contentsec.XSSViolation{
                    Position: idx,
                    Type:     "bad_word",
                    Content:  input[idx : idx+len(word)],
                })
            }
        }
        return violations
    })

    // 方式二: 使用正则模式快速注册
    detector.RegisterPatternFilter(
        "template_injection",
        `\{\{.*?\}\}|\{%.*?%\}`,
        "template_expression",
    )

    // 测试检测
    testInput := "Hello {{user.name}} fuck you!"
    violations := detector.Detect(testInput)
    for _, v := range violations {
        fmt.Printf("[%d] %s: %s\n", v.Position, v.Type, v.Content)
    }
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrNilFilter` | 过滤器函数为 nil | 注册 XSS 过滤器时传入 nil 函数 |
| `ErrFilterExists` | 同名过滤器已存在 | 注册的过滤器名称与现有过滤器冲突 |
| `ErrFilterNotFound` | 过滤器不存在 | 注销不存在的过滤器 |
| `ErrInvalidPattern` | 正则表达式无效 | 传入的正则模式无法编译 |
| `ErrNilSanitizerRule` | 净化配置为 nil | 创建净化器或设置配置时传入 nil |
| `ErrNilMaskPattern` | 脱敏函数为 nil | 注册脱敏模式时传入 nil 函数 |
| `ErrMaskPatternExists` | 同名脱敏模式已存在 | 注册的脱敏模式名称与现有冲突 |
| `ErrMaskPatternNotFound` | 脱敏模式不存在 | 使用或注销不存在的脱敏模式 |

---

## 10. 并发安全

模块所有核心组件都完全支持并发访问，通过以下同步机制保证线程安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| `XSSDetector` | `sync.RWMutex` | 过滤器注册表读写锁保护 |
| `HTMLSanitizer` | `sync.RWMutex` | 净化配置读写锁保护 |
| `DataMasker` | `sync.RWMutex` | 脱敏模式注册表读写锁保护 |
| `OutputEncoder` | 无状态 | 纯函数实现，无需同步 |
| `ContentSecurityEngine` | 各子组件独立保证 | 组合模式，各子组件自行负责并发安全 |

**并发测试验证**：
- 所有测试包含 50 并发协程的压力测试
- 无数据竞争、无死锁、结果一致性保证

---

## 11. 最佳实践

### 11.1 纵深防御策略

不要依赖单一安全措施，应组合使用多种防护：

```
用户输入
    │
    ▼
XSS 检测（记录日志 + 告警）
    │
    ▼
HTML 净化（白名单过滤）
    │
    ▼
敏感数据脱敏（保护隐私）
    │
    ▼
输出编码（根据上下文选择策略）
    │
    ▼
安全输出
```

### 11.2 编码策略选择原则

| 输出位置 | 应使用的编码策略 | 错误示例 |
|---------|----------------|---------|
| HTML 标签内文本 | `ContextHTML` | 不编码 |
| HTML 属性值 | `ContextHTML` | 仅用引号包裹 |
| `<script>` 内字符串 | `ContextJavaScript` | 使用 HTML 编码 |
| URL 查询参数值 | `ContextURL` | 手动拼接 |
| `style` 属性值 | `ContextCSS` | 不编码或使用 HTML 编码 |
| JSON 响应字段 | `ContextJavaScript` | 直接拼接字符串 |

### 11.3 净化配置建议

1. **最小权限原则**：只允许确实需要的标签和属性，宁可少不可多
2. **事件处理器一律禁止**：`on*` 开头的属性绝对不应出现在白名单中
3. **URL 属性额外校验**：`href`、`src` 等属性应额外校验协议（仅允许 `http:`、`https:`）
4. **定期审计白名单**：随着业务演进，白名单可能需要调整

### 11.4 脱敏策略建议

1. **保留可识别性**：脱敏后应保留足够的字符供人工识别，但不足以还原原始数据
2. **不可逆性**：脱敏操作应是单向的，无法从脱敏结果推导出原始值
3. **一致性**：同一原始值每次脱敏结果应保持一致（如需关联分析场景）
4. **长度保持**：尽量保持脱敏前后字符串长度一致，避免泄露长度信息

### 11.5 监控与审计

1. 对每次 XSS 检测命中记录详细日志，包括：
   - 违规类型和位置
   - 原始请求上下文
   - 客户端 IP 和 User-Agent
2. 定期统计违规类型分布，识别新的攻击趋势
3. 对脱敏操作记录审计日志，确保敏感数据被正确处理
