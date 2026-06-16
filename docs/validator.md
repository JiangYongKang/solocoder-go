# 数据校验引擎模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [内置校验器列表](#4-内置校验器列表)
5. [标签语法说明](#5-标签语法说明)
6. [校验器注册与执行工作流程](#6-校验器注册与执行工作流程)
7. [嵌套结构校验机制](#7-嵌套结构校验机制)
8. [条件校验机制](#8-条件校验机制)
9. [错误聚合机制](#9-错误聚合机制)
10. [使用示例](#10-使用示例)
11. [错误定义](#11-错误定义)
12. [并发安全](#12-并发安全)

---

## 1. 模块概述

数据校验引擎模块是一个功能完备的声明式数据校验框架，提供基于结构体标签和编程接口的双重校验规则定义方式，支持嵌套结构递归校验、自定义校验器扩展、条件校验以及多字段错误聚合等高级特性。

**包路径**: `internal/validator`

**设计目标**:
- 通过结构体标签实现声明式校验，无需逐字段编写校验逻辑
- 支持深度嵌套结构（结构体、指针、切片、Map）的自动递归校验
- 提供丰富的内置校验器，覆盖常见校验场景
- 支持自定义校验器注册，与内置规则无缝组合使用
- 单次校验返回全部错误信息，便于一次性展示给用户
- 支持条件校验，仅在满足特定条件时执行校验规则

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 声明式规则定义 | 通过 `validate` 结构体标签或 `StructRules` 编程接口声明校验规则 |
| 内置校验器 | 提供必填、字符串长度、数值范围、正则、枚举、邮箱、URL、IP 等 15+ 内置校验器 |
| 嵌套结构校验 | 自动递归遍历嵌套结构体、指针字段、切片/数组元素和 Map 值 |
| 自定义校验器 | 通过 `RegisterValidator` 注册自定义校验函数，通过名称在规则中引用 |
| 错误聚合 | 单次校验收集所有字段的全部错误，按字段路径组织返回 |
| 条件校验 | 支持 `|when=` 语法，仅在条件满足时执行对应校验规则 |
| 自定义错误信息 | 支持 `|msg=` 语法为规则指定自定义错误描述 |
| 字段路径追踪 | 错误信息中包含完整字段路径（如 `Address.ZipCode`、`Friends[0].Name`） |

---

## 3. 核心结构体与职责

### 3.1 Validator

校验引擎主结构体，管理校验器注册、条件注册和校验执行流程。

```go
type Validator struct {
    mu          sync.RWMutex
    validators  map[string]ValidatorFunc
    conditions  map[string]ConditionFunc
}
```

**职责**:
- 维护已注册的校验器和条件函数映射表
- 提供线程安全的注册和查询接口
- 协调结构体遍历、规则解析和校验执行
- 通过 `Default()` 提供全局单例，通过 `New()` 创建独立实例

### 3.2 Rule

单条校验规则的描述结构体。

```go
type Rule struct {
    Validator     string        // 校验器名称
    Params        string        // 校验器参数
    Message       string        // 自定义错误信息（可选）
    Condition     ConditionFunc // 执行条件函数（可选，优先级最高）
    ConditionName string        // 已注册的条件名称（可选，标签解析时设置）
}
```

**职责**:
- 封装单条校验规则的完整信息
- 支持自定义错误消息覆盖默认提示
- 支持关联条件函数实现条件校验
- 支持通过条件名称引用已注册的条件函数

**条件解析优先级**:
1. 优先使用 `Condition` 字段（直接设置的函数）
2. 其次尝试用 `ConditionName` 从已注册条件中查找
3. 最后将 `ConditionName` 作为表达式解析（字段引用）

### 3.3 StructRules

编程接口方式定义的结构体校验规则集合。

```go
type StructRules struct {
    Fields map[string][]Rule
}
```

**职责**:
- 通过字段名映射到该字段的校验规则列表
- 适用于无法修改结构体标签的场景（如第三方结构体）
- 与标签方式可同时使用，规则合并执行

### 3.4 ValidationError

单个字段的校验错误。

```go
type ValidationError struct {
    Field   string // 字段路径
    Message string // 错误描述
}
```

**职责**:
- 记录具体哪个字段发生了什么错误
- 字段路径支持嵌套表示（点号分隔层级，方括号表示索引）

### 3.5 ValidationErrors

校验错误集合，实现 `error` 接口。

```go
type ValidationErrors []*ValidationError
```

**职责**:
- 聚合所有校验错误
- 提供 `HasErrors()` 快速判断是否存在错误
- 提供 `FieldErrors(field)` 按字段名筛选错误
- 实现 `Error()` 方法将所有错误格式化为可读字符串

### 3.6 ValidatorFunc

校验器函数类型定义。

```go
type ValidatorFunc func(value interface{}, params string) (bool, string)
```

**职责**:
- 定义所有校验器的统一函数签名
- 参数 `value`: 待校验的字段值
- 参数 `params`: 规则中指定的参数字符串
- 返回 `bool`: 校验是否通过
- 返回 `string`: 校验失败时的错误描述

### 3.7 ConditionFunc

条件函数类型定义。

```go
type ConditionFunc func(structValue interface{}) bool
```

**职责**:
- 定义条件判断的统一函数签名
- 参数 `structValue`: 指向被校验结构体的指针
- 返回 `bool`: 条件是否满足（true 表示执行校验）

---

## 4. 内置校验器列表

| 校验器名称 | 参数格式 | 说明 | 适用类型 |
|-----------|---------|------|---------|
| `required` | 无 | 字段值不能为空（非零值、非 nil） | 所有类型 |
| `min` | 数值（如 `18`、`1.5`） | 数值最小值 | int/uint/float 系列 |
| `max` | 数值（如 `120`、`10.5`） | 数值最大值 | int/uint/float 系列 |
| `minLen` | 整数（如 `3`） | 字符串/切片/Map 最小长度 | string, slice, array, map |
| `maxLen` | 整数（如 `50`） | 字符串/切片/Map 最大长度 | string, slice, array, map |
| `len` | 整数（如 `6`） | 字符串/切片/Map 精确长度 | string, slice, array, map |
| `email` | 无 | 邮箱格式校验 | string |
| `regexp` | 正则表达式（如 `^[A-Z]+$`） | 正则匹配 | string |
| `enum` | 枚举值用 `\|` 分隔（如 `a\|b\|c`） | 枚举值校验 | 所有可转为字符串的类型 |
| `numeric` | 无 | 数值字符串校验 | string（或任意数值类型） |
| `positive` | 无 | 数值必须为正数（> 0） | int/uint/float 系列 |
| `negative` | 无 | 数值必须为负数（< 0） | int/float 系列（有符号） |
| `url` | 无 | URL 格式校验（http/https） | string |
| `ip` | 无 | IPv4 地址格式校验 | string |

---

## 5. 标签语法说明

### 5.1 基本语法

通过在结构体字段上添加 `validate` 标签声明校验规则：

```go
type User struct {
    Name  string `validate:"required,minLen=2,maxLen=50"`
    Email string `validate:"required,email"`
}
```

多个规则之间用逗号 `,` 分隔。

### 5.2 参数传递

校验器名后接 `=` 号和参数值：

```go
Age   int    `validate:"min=18,max=120"`
Role  string `validate:"enum=admin|editor|viewer"`
Code  string `validate:"regexp=^[A-Z]{3}[0-9]{3}$"`
```

### 5.3 自定义错误信息

使用 `|msg=` 后缀为规则指定自定义错误描述：

```go
Name string `validate:"required|msg=用户名是必填的"`
```

### 5.4 条件校验

使用 `|when=` 后缀指定触发条件：

```go
CompanyName  string `validate:"required|when=IsCompany"`     // IsCompany 为 true 时校验
PersonalName string `validate:"required|when=!IsCompany"`    // IsCompany 为 false 时校验
FieldA       string `validate:"required|when=Type=a"`        // Type 等于 "a" 时校验
```

条件表达式语法：
- `FieldName`: 当该字段为非零/非空值时条件成立
- `!FieldName`: 当该字段为零值/空值时条件成立
- `FieldName=value`: 当该字段的字符串表示等于指定值时条件成立

### 5.5 跳过字段

使用 `validate:"-"` 标签表示该字段不解析标签规则（但嵌套结构仍会递归遍历）：

```go
InternalData SomeStruct `validate:"-"`
```

---

## 6. 校验器注册与执行工作流程

### 6.1 校验器注册流程

```
RegisterValidator(name, fn)
    │
    ├─ 参数校验（name 非空、fn 非 nil）
    │
    ├─ 获取写锁
    │
    ├─ 存储到 validators map
    │       key:   校验器名称
    │       value: ValidatorFunc
    │
    └─ 返回 nil（成功）或 ErrInvalidRule（失败）
```

### 6.2 校验执行主流程

```
Validate(structPtr)
    │
    ├─ 步骤 1：输入检查
    │       ├─ 检查是否为 nil
    │       ├─ 解引用指针
    │       └─ 检查是否为结构体类型
    │
    ├─ 步骤 2：初始化错误收集器
    │       └─ 创建空的 ValidationErrors 切片
    │
    ├─ 步骤 3：遍历结构体字段（validateStruct）
    │       │
    │       ├─ 对每个导出字段：
    │       │     │
    │       │     ├─ 构建字段路径（如 Address.Street）
    │       │     │
    │       │     ├─ 解析 validate 标签
    │       │     │     └─ parseTag() → []Rule
    │       │     │
    │       │     ├─ 对当前字段应用规则（applyRules）
    │       │     │
    │       │     └─ 递归校验字段值（validateFieldValue）
    │       │           ├─ 指针 → 解引用后递归
    │       │           ├─ 结构体 → 递归 validateStruct
    │       │           ├─ 切片/数组 → 遍历每个元素
    │       │           └─ Map → 遍历每个值
    │       │
    │       └─ 非导出字段直接跳过
    │
    └─ 步骤 4：返回聚合的 ValidationErrors
```

### 6.3 单条规则应用流程

```
applyRules(fieldVal, fieldPath, rules, structPtr, errs)
    │
    └─ 对每条 Rule：
          │
          ├─ 检查条件（Rule.Condition != nil）
          │     ├─ true → 继续执行校验
          │     └─ false → 跳过该规则
          │
          ├─ 查找校验器（getValidator）
          │     ├─ 找到 → 执行
          │     └─ 未找到 → 记录 "validator not found" 错误
          │
          ├─ 执行校验函数 ValidatorFunc(value, params)
          │     ├─ 返回 (true, "") → 校验通过
          │     └─ 返回 (false, msg) → 校验失败
          │
          └─ 校验失败处理：
                ├─ 优先使用 Rule.Message（自定义消息）
                ├─ 否则使用校验器返回的 msg
                └─ 追加到 ValidationErrors 切片
```

---

## 7. 嵌套结构校验机制

### 7.1 支持的嵌套类型

| 类型 | 处理方式 | 错误路径示例 |
|------|---------|-------------|
| 结构体字段 | 递归遍历其所有字段 | `Address.City` |
| 指针字段 | 非 nil 时解引用后递归 | `Profile.Age`（指针指向结构体） |
| 切片/数组 | 遍历每个元素，索引用方括号 | `Tags[0]`、`Friends[1].Name` |
| Map | 遍历每个值，键用方括号 | `Data["key1"].Value` |

### 7.2 字段路径构造规则

- 结构体嵌套用点号 `.` 连接：`Outer.Middle.Inner`
- 切片索引用方括号 `[n]`：`Items[2]`
- 混合嵌套：`Users[0].Address.ZipCode`

### 7.3 递归终止条件

- 非导出字段：直接跳过
- nil 指针：不递归进入
- 基本类型（string, int, bool 等）：到达叶子节点
- 接口类型：不递归遍历（仅对具体类型生效）

---

## 8. 条件校验机制

### 8.1 条件表达式语法

条件表达式在标签中通过 `|when=` 后缀指定，支持三种形式：

| 表达式形式 | 含义 | 示例 |
|-----------|------|------|
| `FieldName` | 字段非空/非零值时成立 | `when=HasPhone` |
| `!FieldName` | 字段为空/零值时成立 | `when=!IsCompany` |
| `FieldName=value` | 字段字符串值等于指定值时成立 | `when=Type=admin` |

### 8.2 条件函数

通过编程接口可传入任意 `ConditionFunc`，实现更复杂的条件逻辑：

```go
rules := StructRules{
    Fields: map[string][]Rule{
        "Phone": {
            {
                Validator: "required",
                Condition: func(s interface{}) bool {
                    u := s.(*User)
                    return u.UsePhone && u.Country == "CN"
                },
            },
        },
    },
}
```

### 8.3 执行时机

条件判断在校验器执行之前进行：
- 条件成立 → 执行该条规则的校验
- 条件不成立 → 跳过该条规则，不产生错误

---

## 9. 错误聚合机制

### 9.1 错误收集策略

- **不提前终止**：校验过程中遇到错误不会立即返回，而是继续校验剩余字段
- **全部收集**：所有字段的所有规则产生的错误都会被收集
- **顺序保留**：错误顺序与字段遍历顺序、规则顺序一致

### 9.2 ValidationErrors 接口

```go
type ValidationErrors []*ValidationError

// 判断是否存在错误
func (ve ValidationErrors) HasErrors() bool

// 按字段名筛选错误（支持前缀匹配）
func (ve ValidationErrors) FieldErrors(field string) []*ValidationError

// 格式化为可读字符串
func (ve ValidationErrors) Error() string
```

### 9.3 FieldErrors 前缀匹配

`FieldErrors("Address")` 会返回：
- `Address`（精确匹配）
- `Address.Street`（点号前缀）
- `Address[0]`（方括号前缀）

---

## 10. 使用示例

### 10.1 基本使用（标签方式）

```go
package main

import (
    "fmt"
    "solocoder-go/internal/validator"
)

type User struct {
    Name     string `validate:"required,minLen=2,maxLen=50"`
    Email    string `validate:"required,email"`
    Age      int    `validate:"min=18,max=120"`
    Password string `validate:"required,minLen=8,regexp=^[a-zA-Z0-9]+$"`
}

func main() {
    user := User{
        Name:     "Alice",
        Email:    "alice@example.com",
        Age:      25,
        Password: "Password123",
    }

    errs := validator.Validate(&user)
    if errs.HasErrors() {
        fmt.Println("校验失败:", errs)
        for _, e := range errs {
            fmt.Printf("  字段 %s: %s\n", e.Field, e.Message)
        }
    } else {
        fmt.Println("校验通过")
    }
}
```

### 10.2 嵌套结构校验

```go
type Address struct {
    Street  string `validate:"required,minLen=3"`
    City    string `validate:"required"`
    ZipCode string `validate:"required,len=6"`
}

type Customer struct {
    Name    string   `validate:"required"`
    Address Address  `validate:"-"`
}

func main() {
    customer := Customer{
        Name: "Bob",
        Address: Address{
            Street:  "123 Main Street",
            City:    "Beijing",
            ZipCode: "100001",
        },
    }

    errs := validator.Validate(&customer)
    // 嵌套字段错误路径示例：Address.Street, Address.City, Address.ZipCode
}
```

### 10.3 切片元素校验

```go
type Order struct {
    ID      string   `validate:"required"`
    Items   []Item   `validate:"-"`
}

type Item struct {
    Product string `validate:"required"`
    Qty     int    `validate:"min=1,max=99"`
}

func main() {
    order := Order{
        ID: "ORD-001",
        Items: []Item{
            {Product: "Book", Qty: 2},
            {Product: "", Qty: 0}, // 会产生 Items[1].Product 和 Items[1].Qty 错误
        },
    }

    errs := validator.Validate(&order)
}
```

### 10.4 自定义校验器

```go
func main() {
    err := validator.RegisterValidator("is_even", func(value interface{}, params string) (bool, string) {
        if n, ok := value.(int); ok {
            if n%2 == 0 {
                return true, ""
            }
            return false, "数值必须为偶数"
        }
        return false, "值必须为整数"
    })
    if err != nil {
        panic(err)
    }

    type TestStruct struct {
        Number int `validate:"is_even"`
    }

    s := TestStruct{Number: 3}
    errs := validator.Validate(&s)
    // Number: 数值必须为偶数
}
```

### 10.5 编程接口方式（StructRules）

```go
func main() {
    type User struct {
        Name  string
        Age   int
        Email string
    }

    rules := validator.StructRules{
        Fields: map[string][]validator.Rule{
            "Name": {
                {Validator: "required"},
                {Validator: "minLen", Params: "2"},
            },
            "Age": {
                {Validator: "min", Params: "18"},
                {Validator: "max", Params: "120"},
            },
            "Email": {
                {Validator: "email"},
            },
        },
    }

    user := User{Name: "A", Age: 150, Email: "invalid"}
    errs := validator.ValidateWithRules(&user, rules)
}
```

### 10.6 条件校验

```go
type Contact struct {
    IsCompany    bool   `validate:"-"`
    CompanyName  string `validate:"required|when=IsCompany"`
    PersonalName string `validate:"required|when=!IsCompany"`
    TaxID        string `validate:"required|when=IsCompany=true"`
}

func main() {
    // 公司场景：CompanyName 必填，PersonalName 可选
    company := Contact{
        IsCompany:   true,
        CompanyName: "Acme Corp",
    }
    errs := validator.Validate(&company) // 通过

    // 个人场景：PersonalName 必填，CompanyName 可选
    person := Contact{
        IsCompany:    false,
        PersonalName: "Alice",
    }
    errs = validator.Validate(&person) // 通过
}
```

### 10.7 自定义错误消息

```go
type Form struct {
    Username string `validate:"required|msg=用户名不能为空"`
    Password string `validate:"required,minLen=8|msg=密码至少8位"`
}
```

---

## 11. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidValue` | 无效值 | 通用校验失败 |
| `ErrValidatorNotFound` | 校验器未找到 | 规则引用了未注册的校验器名称 |
| `ErrInvalidRule` | 无效规则 | 注册校验器时空名称或 nil 函数 |
| `ErrConditionNotMet` | 条件不满足 | 内部条件判断（不对外暴露为错误） |
| `ErrUnsupportedType` | 不支持的类型 | 校验器应用于不兼容的类型 |
| `ErrNonStructValidation` | 非结构体值 | 向 Validate 传入非结构体类型 |

---

## 12. 并发安全

模块完全支持并发访问，通过以下机制保证线程安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| 校验器映射表 | `sync.RWMutex` | 注册使用写锁，查询使用读锁 |
| 条件映射表 | `sync.RWMutex` | 同上 |
| 校验执行过程 | 无锁 | 只读操作，不修改共享状态 |
| 全局单例 | `sync.Once` | 保证 Default() 只初始化一次 |

**最佳实践**:
- 在校验开始前完成所有自定义校验器的注册
- 运行时频繁注册/注销校验器会影响性能（持有写锁）
- 推荐使用全局 `Default()` 单例，避免重复创建 Validator 实例
