# GraphQL 查询解析器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [类型系统](#4-类型系统)
5. [Schema Definition Language (SDL) 解析](#5-schema-definition-language-sdl-解析)
6. [字段解析器注册机制](#6-字段解析器注册机制)
7. [查询验证流程](#7-查询验证流程)
8. [DataLoader 批量数据加载](#8-dataloader-批量数据加载)
9. [查询执行引擎](#9-查询执行引擎)
10. [使用示例](#10-使用示例)
11. [错误定义](#11-错误定义)
12. [并发安全](#12-并发安全)

---

## 1. 模块概述

GraphQL 查询解析器模块是一个功能完整的 GraphQL 服务端实现，提供 Schema 定义、查询解析、验证、执行以及批量数据加载等核心能力。模块使用内存数据结构模拟数据源，无需外部依赖即可运行。

**包路径**: `internal/gqlparser`

**设计目标**:
- 支持标准 Schema Definition Language (SDL) 定义类型系统
- 提供灵活的字段解析器注册机制，支持覆盖
- 实现完整的查询验证，提供明确的错误路径提示
- 通过 DataLoader 模式解决 N+1 查询问题
- 支持嵌套查询、别名、变量、片段等 GraphQL 核心特性

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| Schema 定义 | 通过 SDL 定义对象类型、标量类型、列表类型、非空标记、Query 和 Mutation 操作 |
| 类型查询 | 支持查询已注册的类型及其字段信息 |
| 解析器注册 | 为每个类型的每个字段注册解析函数，后者覆盖前者 |
| 查询解析 | 将 GraphQL 查询字符串解析为 AST（抽象语法树） |
| 查询验证 | 检查字段存在性、参数类型、嵌套深度、必选参数等 |
| 嵌套执行 | 支持任意深度的嵌套查询，正确处理列表和对象类型 |
| DataLoader | 批量收集加载请求，统一执行后分发结果，避免 N+1 问题 |
| 别名支持 | 查询字段可指定别名，结果按别名返回 |
| 变量支持 | 支持查询变量定义、类型注解和默认值 |
| 片段支持 | 支持内联片段 `... on Type { ... }` |

---

## 3. 核心结构体与职责

### 3.1 Schema

GraphQL Schema 核心结构体，管理类型系统和解析器注册。

```go
type Schema struct {
    mu           sync.RWMutex
    types        map[string]*Type
    queryType    *Type
    mutationType *Type
    resolvers    map[string]map[string]ResolverFunc
}
```

**职责**:
- 维护类型注册表，支持类型的注册和查询
- 管理 Query 和 Mutation 根操作类型
- 维护字段解析器注册表，按 `类型名.字段名` 两级索引
- 提供 SDL 解析入口 `ParseSDL()`

### 3.2 Type

GraphQL 类型定义结构体，表示标量、对象、列表、非空等各种类型。

```go
type Type struct {
    Kind      TypeKind
    Name      string
    OfType    *Type
    Fields    map[string]*Field
    IsBuiltin bool
}
```

**职责**:
- 描述类型的种类（标量/对象/列表/非空/Query/Mutation）
- 通过 `OfType` 构成包装类型链（如 `[User!]!`）
- 对象类型维护字段映射表

### 3.3 Field

对象类型中的字段定义。

```go
type Field struct {
    Name      string
    Type      *Type
    Arguments map[string]*Argument
}
```

### 3.4 Validator

查询验证器，在执行前对查询进行合法性检查。

```go
type Validator struct {
    MaxDepth int
}
```

**职责**:
- 验证请求字段是否在 Schema 中定义
- 验证参数字段是否存在、类型是否匹配
- 验证必选参数是否提供
- 验证嵌套深度是否超过限制

### 3.5 DataLoader

批量数据加载器，解决 N+1 查询问题。

```go
type DataLoader struct {
    mu      sync.Mutex
    fn      DataLoaderFunc
    pending []*loaderRequest
}
```

**职责**:
- 收集待加载的键
- 通过批量加载函数统一获取数据
- 将结果分发到各个等待请求
- 支持清除单个或全部待加载请求

### 3.6 Executor

查询执行引擎，负责解析、验证并执行 GraphQL 查询。

```go
type Executor struct {
    Schema    *Schema
    Validator *Validator
}
```

**职责**:
- 解析查询字符串为 AST
- 调用 Validator 进行验证
- 遍历 AST 调用解析器获取数据
- 组装最终结果

---

## 4. 类型系统

### 4.1 类型种类 (TypeKind)

| 常量 | 值 | 描述 |
|------|-----|------|
| `TypeKindScalar` | 0 | 标量类型（Int, Float, String, Boolean, ID） |
| `TypeKindObject` | 1 | 对象类型 |
| `TypeKindList` | 2 | 列表类型 |
| `TypeKindNonNull` | 3 | 非空包装类型 |
| `TypeKindQuery` | 4 | Query 根操作类型 |
| `TypeKindMutation` | 5 | Mutation 根操作类型 |

### 4.2 内置标量类型

Schema 创建时自动注册 5 个内置标量：
- `Int`: 32 位整数
- `Float`: 双精度浮点数
- `String`: UTF-8 字符串
- `Boolean`: 布尔值
- `ID`: 唯一标识符（字符串形式）

### 4.3 类型辅助方法

- `Unwrap() *Type`: 递归去除所有 NonNull 和 List 包装，返回最内层类型
- `IsList() bool`: 判断类型是否为列表（含被 NonNull 包装的列表）
- `IsNonNull() bool`: 判断类型是否为 NonNull
- `InnerType() *Type`: 返回直接的内层类型

---

## 5. Schema Definition Language (SDL) 解析

### 5.1 支持的语法

**对象类型定义**:
```graphql
type User {
  id: ID!
  name: String!
  email: String
  age: Int
  posts: [Post]
}
```

**字段参数**:
```graphql
type Query {
  user(id: ID!): User
  search(query: String!, limit: Int = 10): [User]
}
```

**非空标记**: `Type!` 表示不可为空
**列表类型**: `[Type]` 表示元素为 Type 的列表
**组合使用**: `[User!]!` 表示非空的非空元素列表

### 5.2 SDL 解析流程

1. 词法扫描：逐字符扫描输入，识别关键字、名称、符号
2. 类型定义识别：识别 `type` / `scalar` 关键字
3. 字段解析：解析每个字段的名称、参数、类型引用
4. 类型注册：将解析出的类型注册到 Schema

---

## 6. 字段解析器注册机制

### 6.1 ResolverFunc 签名

```go
type ResolverFunc func(parent interface{}, args map[string]interface{}) (interface{}, error)
```

**参数**:
- `parent`: 父类型的解析结果（顶层字段为 nil）
- `args`: 字段参数映射，已解析变量和默认值

**返回值**:
- 标量值（string, int, float, bool）
- 对象值（map[string]interface{} 或 struct）
- 列表值（slice）
- error（解析失败时）

### 6.2 注册规则

```go
func (s *Schema) RegisterResolver(typeName, fieldName string, fn ResolverFunc) error
```

- 类型必须已在 Schema 中注册
- 字段必须在该类型中已定义
- 同一字段重复注册时，后者覆盖前者
- 返回 error 表示类型或字段不存在

---

## 7. 查询验证流程

查询执行前，`Validator` 执行以下检查：

### 7.1 操作类型验证
- Query 操作：Schema 必须已设置 Query 类型
- Mutation 操作：Schema 必须已设置 Mutation 类型

### 7.2 字段存在性验证
- 遍历选择集，检查每个字段是否存在于父类型的 Fields 中
- 内联片段检查目标类型是否存在
- 错误信息包含完整路径，如 `user.posts.title`

### 7.3 参数验证
- 检查字段参数名是否已定义
- 检查 NonNull 参数是否提供
- 检查参数值类型是否匹配（Int/Float/String/Boolean/ID）

### 7.4 深度限制验证
- 递归计算嵌套选择集的深度
- 超过 `MaxDepth`（默认 10）返回错误

### 7.5 子选择验证
- 标量类型不能有子选择集
- 对象类型的子选择集不能为空

---

## 8. DataLoader 批量数据加载

### 8.1 N+1 问题

当查询返回 N 个父对象，每个父对象需要加载子字段时，朴素实现会产生 1 + N 次数据库查询。DataLoader 通过批量收集 + 单次查询将其优化为 2 次。

### 8.2 工作流程

```
┌──────────────┐     Load(key1)     ┌──────────────────┐
│  Goroutine 1 │ ──────────────────> │                  │
└──────────────┘                     │                  │
┌──────────────┐     Load(key2)     │   DataLoader     │
│  Goroutine 2 │ ──────────────────> │   pending 队列   │
└──────────────┘                     │   [key1, key2]   │
┌──────────────┐     Load(key3)     │                  │
│  Goroutine 3 │ ──────────────────> │                  │
└──────────────┘                     └────────┬─────────┘
                                              │
                                              │ Flush()
                                              ▼
                                     ┌──────────────────┐
                                     │  Batch Load Func │
                                     │  fn([k1,k2,k3])  │
                                     └────────┬─────────┘
                                              │
                                              │ 分发结果
                                              ▼
┌──────────────┐      val1         ┌──────────────────┐
│  Goroutine 1 │ <──────────────── │                  │
└──────────────┘                   │                  │
┌──────────────┐      val2         │   DataLoader     │
│  Goroutine 2 │ <──────────────── │  结果分发通道    │
└──────────────┘                   │                  │
┌──────────────┐      val3         │                  │
│  Goroutine 3 │ <──────────────── │                  │
└──────────────┘                   └──────────────────┘
```

**详细步骤**:

1. **收集阶段**: 多个 goroutine 调用 `Load(key)`，每个调用将 key 加入 `pending` 队列，并在 `result` channel 上阻塞等待
2. **触发阶段**: 外部调用 `Flush()`，将 `pending` 队列取出，调用批量加载函数 `fn(keys)`
3. **执行阶段**: 批量加载函数一次性获取所有 key 对应的数据，返回 `[]interface{}`
4. **分发阶段**: 遍历结果，按顺序将每个结果发送到对应请求的 `result` channel
5. **完成阶段**: 各 goroutine 从 channel 接收结果，继续执行

### 8.3 DataLoaderFunc 签名

```go
type DataLoaderFunc func(keys []interface{}) ([]interface{}, error)
```

- 输入: 待加载的键列表，顺序与调用顺序一致
- 输出: 对应的值列表，长度必须等于 keys 长度
- 若返回 error，所有请求均收到该错误

### 8.4 核心 API

| 方法 | 描述 |
|------|------|
| `NewDataLoader(fn)` | 创建新的 DataLoader 实例 |
| `Load(key)` | 加载单个键，阻塞等待结果 |
| `LoadMany(keys)` | 批量加载多个键，一次性收集所有键后等待 |
| `Flush()` | 触发批量加载，分发结果 |
| `Clear(key)` | 从待加载队列中移除指定键 |
| `ClearAll()` | 清空所有待加载请求 |

---

## 9. 查询执行引擎

### 9.1 执行流程

```
查询字符串
    │
    ▼
ParseQuery() → AST (Document)
    │
    ▼
Validator.Validate() → 错误列表
    │
    ▼
Executor.executeOperation()
    │
    ├── 解析变量参数（处理 VariableRef 和默认值）
    │
    └── 遍历选择集
            │
            ├── executeField()
            │       │
            │       ├── 查找并调用 Resolver
            │       └── 确定字段类型
            │
            ├── executeSelectionSetOnValue()
            │       │
            │       ├── 列表类型 → executeList()
            │       └── 对象类型 → executeObject()
            │
            └── 递归处理嵌套选择
```

### 9.2 值解析

解析器返回值支持以下类型：
- **标量值**: `string`, `int`, `int64`, `float64`, `bool` → 直接返回
- **Map**: `map[string]interface{}` → 按键名查找子字段
- **Struct**: 任意结构体 → 通过反射按字段名或 json tag 查找
- **Slice/Array**: 任意切片或数组 → 遍历每个元素递归执行选择集

### 9.3 别名处理

字段可指定别名：
```graphql
{
  user(id: "1") {
    userName: name
    userAge: age
  }
}
```
结果按别名返回：
```json
{"user": {"userName": "Alice", "userAge": 30}}
```

---

## 10. 使用示例

### 10.1 基础查询

```go
package main

import (
    "fmt"
    "solocoder-go/internal/gqlparser"
)

func main() {
    s := gqlparser.NewSchema()

    // 1. 解析 SDL 定义类型
    sdl := `
        type User {
            id: ID!
            name: String!
            age: Int
        }
        type Query {
            user(id: ID!): User
        }
    `
    if err := s.ParseSDL(sdl); err != nil {
        panic(err)
    }

    // 2. 注册字段解析器
    users := map[string]map[string]interface{}{
        "1": {"id": "1", "name": "Alice", "age": 30},
    }
    s.RegisterResolver("Query", "user", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
        id := fmt.Sprintf("%v", args["id"])
        return users[id], nil
    })

    // 3. 执行查询
    e := &gqlparser.Executor{Schema: s, Validator: gqlparser.NewValidator()}
    result, errs := e.Execute(`{ user(id: "1") { name age } }`, nil, nil)

    fmt.Println(result)  // map[user:map[age:30 name:Alice]]
}
```

### 10.2 嵌套查询 + DataLoader

```go
// 模拟用户和帖子数据
userPosts := map[string][]string{
    "1": {"101", "103"},
    "2": {"102"},
}
posts := map[string]map[string]interface{}{
    "101": {"id": "101", "title": "First Post"},
    "102": {"id": "102", "title": "Second Post"},
    "103": {"id": "103", "title": "Third Post"},
}

// 创建 DataLoader，批量加载帖子
postLoader := gqlparser.NewDataLoader(func(keys []interface{}) ([]interface{}, error) {
    results := make([]interface{}, len(keys))
    for i, k := range keys {
        id := fmt.Sprintf("%v", k)
        results[i] = posts[id]
    }
    return results, nil
})

// User.posts 解析器：返回帖子 ID 列表
s.RegisterResolver("User", "posts", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
    userMap := parent.(map[string]interface{})
    userId := fmt.Sprintf("%v", userMap["id"])
    postIds := userPosts[userId]
    result := make([]interface{}, len(postIds))
    for i, id := range postIds {
        val, _ := postLoader.Load(id)
        result[i] = val
    }
    return result, nil
})

// 执行嵌套查询
query := `{
    users {
        name
        posts { title }
    }
}`
dataLoaders := map[string]*gqlparser.DataLoader{"posts": postLoader}
result, errs := e.Execute(query, nil, dataLoaders)

// 在 goroutine 中触发 Flush，或使用单独的调度机制
go func() {
    postLoader.Flush()
}()
```

### 10.3 Mutation 操作

```go
sdl := `
    type Mutation {
        createUser(name: String!, age: Int): User
    }
`
s.ParseSDL(sdl)

s.RegisterResolver("Mutation", "createUser", func(parent interface{}, args map[string]interface{}) (interface{}, error) {
    name := fmt.Sprintf("%v", args["name"])
    age, _ := args["age"].(int)
    newUser := map[string]interface{}{
        "id":   "new-id",
        "name": name,
        "age":  age,
    }
    return newUser, nil
})

mutation := `mutation {
    createUser(name: "Charlie", age: 28) {
        id
        name
    }
}`
result, errs := e.Execute(mutation, nil, nil)
```

---

## 11. 错误定义

| 错误变量 | 描述 |
|----------|------|
| `ErrTypeNotFound` | 请求的类型未在 Schema 中注册 |
| `ErrFieldNotFound` | 请求的字段在类型中不存在 |
| `ErrTypeAlreadyExists` | 注册类型时名称冲突 |
| `ErrInvalidSDL` | SDL 语法错误 |
| `ErrInvalidQuery` | 查询语法错误 |
| `ErrNestedTooDeep` | 嵌套深度超过限制 |
| `ErrDataLoaderNotReady` | DataLoader 返回值数量不足 |
| `ValidationError` | 结构化验证错误，包含 `Path` 和 `Message` |

**ValidationError 示例**:
```
Path: "user.posts.invalidField"
Message: "field \"invalidField\" not found in type Post"
```

---

## 12. 并发安全

- `Schema` 使用 `sync.RWMutex` 保护类型和解析器注册表
  - 读操作（GetType, GetResolver, GetAllTypes）使用读锁
  - 写操作（ParseSDL, RegisterType, RegisterResolver）使用写锁
- `DataLoader` 使用 `sync.Mutex` 保护 pending 队列
- `Validator` 无状态，可并发调用
- `Executor` 执行查询为纯只读操作，同一 Schema 可被多个 Executor 并发使用
