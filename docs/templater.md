# Templater 模板渲染引擎模块

## 1. 模块概述

Templater 是一个轻量级的 Go 模板渲染引擎模块，提供了变量替换、条件渲染、循环遍历、模板继承、自定义函数注册以及模板缓存与热加载等功能。该模块位于 `internal/templater/` 包下，采用自定义的模板语法，适用于需要动态内容生成的场景。

## 2. 模块功能

### 2.1 变量替换

支持在模板中使用 `{{ .VariableName }}` 语法标记变量占位符，渲染时传入键值对数据对象将占位符替换为对应的变量值。

**特性：**
- 支持点号路径访问嵌套结构体的字段，如 `{{ .User.Profile.Name }}`
- 支持 `map[string]interface{}`、`map[string]string` 以及 Go struct（包括指针类型）
- 变量不存在时根据配置决定行为：
  - `StrictVariables: true` 时返回 `ErrVariableNotFound` 错误
  - `StrictVariables: false` 时输出空字符串

**示例：**
```go
data := map[string]interface{}{
    "Name": "Alice",
    "User": map[string]interface{}{
        "Age": 25,
    },
}
// 模板: "Hello {{ .Name }}, Age: {{ .User.Age }}"
// 结果: "Hello Alice, Age: 25"
```

### 2.2 条件渲染

支持在模板中使用条件块控制内容的显示与隐藏。

**支持的条件操作：**
- 等于判断：`{{ if .Role == "admin" }}...{{ endif }}`
- 不等于判断：`{{ if .Status != "active" }}...{{ endif }}`
- 空值判断：`{{ if empty .Items }}...{{ endif }}`
- 布尔值判断：`{{ if .Enabled }}...{{ endif }}`
- 支持 if-else 二分支结构

**示例：**
```
{{ if .Role == "admin" }}
    <p>Welcome, Administrator!</p>
{{ else }}
    <p>Welcome, User!</p>
{{ endif }}
```

### 2.3 循环遍历

支持在模板中使用循环块遍历数组或切片类型的数据。

**语法：**
- 带索引和值：`{{ range $i, $item := range .Items }}...{{ endrange }}`
- 仅值：`{{ range $item := range .Items }}...{{ endrange }}`

**特性：**
- 循环块内可以访问当前迭代元素的值（$item）和索引（$i）
- 在循环块内部可以使用变量替换和条件渲染形成嵌套结构
- 使用 `$` 前缀的变量为循环作用域变量

**示例：**
```
<ul>
{{ range $i, $item := range .Items }}
    <li>{{ $i }}: {{ $item.Name }}</li>
{{ endrange }}
</ul>
```

### 2.4 模板继承

支持定义父模板作为基础布局，包含多个可被子模板填充的块占位符。

**语法：**
- 父模板定义块：`{{ block header }}Default Header{{ endblock }}`
- 子模板声明继承：`{{ extends "parent_template_name" }}`
- 子模板重写块：`{{ block header }}Custom Header{{ endblock }}`

**特性：**
- 子模板通过声明继承某个父模板并重写其中特定块的内容实现页面布局的复用
- 子模板中未重写的块保留父模板的默认内容
- 检测循环继承并返回 `ErrTemplateInheritanceLoop` 错误

**示例：**

父模板 (layout):
```
<!DOCTYPE html>
<html>
<head>{{ block head }}<title>{{ .Title }}</title>{{ endblock }}</head>
<body>{{ block content }}Default Content{{ endblock }}</body>
</html>
```

子模板 (page):
```
{{ extends "layout" }}
{{ block head }}<title>{{ .Title }} - MySite</title>{{ endblock }}
{{ block content }}<h1>Hello World</h1>{{ endblock }}
```

### 2.5 自定义函数注册

支持注册全局自定义函数供模板中调用。

**特性：**
- 自定义函数通过函数名映射到指定的 Go 函数
- 模板渲染时遇到函数调用表达式如 `{{ formatTime .CreatedAt }}` 时调用注册的函数处理参数并返回结果
- 函数参数支持变量（`.Variable`、`$Variable`）和字面量
- 支持函数返回 `(result, error)` 类型，error 非 nil 时渲染终止并返回错误

**示例：**
```go
e := templater.NewEngine(templater.Config{})
e.RegisterFunction("upper", func(s string) string {
    return strings.ToUpper(s)
})
e.RegisterFunction("formatTime", func(t time.Time, layout string) string {
    return t.Format(layout)
})
// 模板: "{{ upper .Name }}"
// 数据: map[string]interface{}{"Name": "hello"}
// 结果: "HELLO"
```

### 2.6 模板缓存与热加载

已解析的模板对象缓存到内存中避免每次渲染重复解析。

**特性：**
- 提供 `InvalidateCache(name string)` 接口删除指定模板的缓存项
- 重新注册同名模板时自动清除旧缓存
- 提供 `ClearCache()` 接口清除所有缓存
- 使用读写锁保证并发安全

## 3. 核心结构体的职责

### 3.1 Engine

**职责：**
- 模板引擎的核心入口，管理模板注册、函数注册、缓存管理和渲染调度

**主要方法：**
- `NewEngine(config Config)`: 创建新的模板引擎实例
- `RegisterTemplate(name, source string)`: 注册模板
- `RegisterFunction(name string, fn interface{})`: 注册自定义函数
- `Render(name string, data interface{})`: 渲染指定模板
- `GetTemplate(name string)`: 获取已解析的模板（含缓存）
- `InvalidateCache(name string)`: 使指定模板缓存失效
- `ClearCache()`: 清除所有缓存

### 3.2 Config

**职责：**
- 配置模板引擎的行为配置

**字段：**
- `StrictVariables bool`: 变量不存在时是否返回错误

### 3.3 Template

**职责：**
- 表示已解析的模板对象，包含模板的抽象语法树

**字段：**
- `Name string`: 模板名称
- `Source string`: 原始模板源字符串
- `Nodes []Node`: 解析后的节点树
- `Blocks map[string]*BlockNode`: 模板块映射
- `Extends *ExtendsNode`: 继承信息

### 3.4 Node 接口及实现

| 节点类型 | 职责 |
|---------|------|
| TextNode | 纯文本内容节点 |
| VariableNode | 变量引用节点（.Variable 或 $Variable） |
| FunctionNode | 函数调用节点 |
| IfNode | 条件判断节点（含 true/false 分支） |
| RangeNode | 循环遍历节点 |
| BlockNode | 模板块节点 |
| ExtendsNode | 模板继承节点 |

## 4. 模板继承的渲染流程

1. **解析阶段：**
   - 调用 `Render(name, data)` 开始渲染
   - 通过 `GetTemplate` 获取模板（优先从缓存读取，不存在则解析并缓存）

2. **继承检测：**
   - 检查模板是否包含 `Extends` 节点
   - 如果有继承，递归获取父模板
   - 检测循环继承（使用 visited 集合）

3. **块合并：**
   - 复制父模板的所有块定义
   - 用子模板重写的块覆盖父模板对应块

4. **渲染阶段：**
   - 遍历合并后的节点树
   - 遇到 BlockNode 时使用重写后的块内容

5. **完成渲染：**
   - 将渲染结果拼接为字符串返回

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/templater"
)

func main() {
    // 创建引擎
    e := templater.NewEngine(templater.Config{
        StrictVariables: true,
    })

    // 注册模板
    e.RegisterTemplate("hello", "Hello, {{ .Name }}!")

    // 准备数据
    data := map[string]interface{}{
        "Name": "World",
    }

    // 渲染
    result, err := e.Render("hello", data)
    if err != nil {
        panic(err)
    }
    fmt.Println(result) // Output: Hello, World!
}
```

### 5.2 完整示例

```go
package main

import (
    "fmt"
    "strings"
    "time"
    "solocoder-go/internal/templater"
)

func main() {
    e := templater.NewEngine(templater.Config{StrictVariables: true})

    // 注册自定义函数
    e.RegisterFunction("upper", func(s string) string {
        return strings.ToUpper(s)
    })
    e.RegisterFunction("formatDate", func(t time.Time) string {
        return t.Format("2006-01-02")
    })

    // 注册布局模板
    layout := `
<!DOCTYPE html>
<html>
<head>{{ block title }}Default Title{{ endblock }}</head>
<body>
<header>{{ block header }}Default Header{{ endblock }}</header>
<main>{{ block content }}{{ endblock }}</main>
<footer>© 2024 MyApp</footer>
</body>
</html>`
    e.RegisterTemplate("layout", layout)

    // 注册页面模板
    page := `
{{ extends "layout" }}
{{ block title }}{{ .PageTitle }}{{ endblock }}
{{ block header }}Welcome, {{ upper .User.Name }}{{ endblock }}
{{ block content }}
<h1>{{ .Heading }}</h1>
<p>Joined: {{ formatDate .User.JoinedAt }}</p>
{{ if .User.IsAdmin }}
<p class="admin-badge">ADMIN</p>
{{ endif }}
<h3>Items:</h3>
<ul>
{{ range $i, $item := range .Items }}
<li>{{ $i + 1 }}. {{ $item }}</li>
{{ endrange }}
</ul>
{{ if empty .Items }}
<p>No items yet.</p>
{{ endif }}
{{ endblock }}`
    e.RegisterTemplate("home", page)

    // 准备数据
    data := map[string]interface{}{
        "PageTitle": "Home Page",
        "Heading":   "Welcome to MyApp",
        "User": map[string]interface{}{
            "Name":     "Alice",
            "IsAdmin":  true,
            "JoinedAt": time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC),
        },
        "Items": []string{"Dashboard", "Reports", "Settings"},
    }

    // 渲染
    result, err := e.Render("home", data)
    if err != nil {
        panic(err)
    }
    fmt.Println(result)
}
```

## 6. 错误处理

模块定义了以下错误类型：

| 错误类型 | 说明 |
|---------|------|
| ErrTemplateNotFound | 模板未找到 |
| ErrVariableNotFound | 变量未找到（严格模式） |
| ErrInvalidVariablePath | 无效的变量路径 |
| ErrInvalidCondition | 无效的条件表达式 |
| ErrInvalidRange | 无效的 range 表达式 |
| ErrRangeNotIterable | range 值不可迭代 |
| ErrUnclosedBlock | 未闭合的块 |
| ErrInvalidBlockSyntax | 无效的块语法 |
| ErrTemplateInheritanceLoop | 模板继承循环 |
| ErrParentTemplateNotFound | 父模板未找到 |
| ErrFunctionNotFound | 函数未找到 |
| ErrInvalidFunctionCall | 无效的函数调用 |
| ErrInvalidArgumentCount | 函数参数数量不匹配 |
| ErrEmptyTemplateName | 空模板名 |
| ErrBlockNotFound | 块未找到 |
