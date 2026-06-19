# 通用分页助手模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [游标分页机制](#4-游标分页机制)
5. [偏移量分页机制](#5-偏移量分页机制)
6. [总条数统计与注入](#6-总条数统计与注入)
7. [标准化响应封装](#7-标准化响应封装)
8. [适用场景对比](#8-适用场景对比)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [边界条件处理](#11-边界条件处理)

---

## 1. 模块概述

通用分页助手模块（Pagination Helper）是一个提供两种分页模式的通用工具模块：**基于游标的前向/后向分页**和**基于偏移量的传统分页**。模块采用 Go 泛型设计，可适配任意数据类型，提供统一的响应封装结构，内置总条数统计机制，支持数据层在查询完成后注入总记录数。

**包路径**: `internal/pagination`

**设计目标**:
- 支持游标分页（前向 + 后向）与偏移量分页两种主流模式
- 使用 Go 泛型适配任意数据类型，无需为每种类型重复实现
- 统一标准化的响应结构，包含数据列表、分页元信息和导航字段
- 独立的总条数设置接口，支持数据层先分页查询再注入总数
- 完善的参数校验与边界处理，页码超出范围返回空列表而非报错
- 内置页面大小上限保护，防止过大的分页请求影响系统性能

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 游标前向分页 | 从指定游标值之后获取数据，返回下一页游标和是否有更多页 |
| 游标后向分页 | 从指定游标值之前获取数据，返回上一页游标和是否有更多页 |
| 偏移量分页 | 通过页码（从 1 开始）和每页大小计算偏移量，返回对应范围数据 |
| 总条数注入 | 提供 `SetTotal()` 接口，支持查询完成后动态注入总记录数 |
| 构建响应时统计 | 在构建响应时直接传入总条数，一次性计算分页信息 |
| 空响应构建 | 快速构建零记录的空分页响应 |
| 参数校验 | 独立的参数校验函数，支持提前验证分页参数合法性 |
| 偏移量计算 | 提供 `Offset()`/`Limit()` 辅助方法，直接用于数据库查询 |
| 导航信息 | 自动计算前后页/上下游标等导航字段，方便前端翻页 |
| 边界保护 | 页码超出范围自动返回空列表，不抛异常 |

---

## 3. 核心结构体与职责

### 3.1 PageResponse[T]

通用分页响应包装器，使用 Go 泛型适配任意数据类型。

```go
type PageResponse[T any] struct {
    Data    []T
    Meta    any
    Nav     any
    Success bool
}
```

**职责**:
- `Data`: 当前页的实际数据列表，类型为泛型参数 `T` 的切片
- `Meta`: 分页元信息，游标分页时为 `*CursorPageMeta`，偏移量分页时为 `*OffsetPageMeta`
- `Nav`: 导航信息，游标分页时为 `*CursorNav`，偏移量分页时为 `*OffsetNav`
- `Success`: 请求成功标志，模块内部构建时恒为 `true`

**主要方法**:
- `SetTotal(total int64) error`: 向分页响应中注入总记录数，根据 Meta 类型自动更新相关字段

### 3.2 CursorPageRequest

游标分页的请求参数结构体。

```go
type CursorPageRequest struct {
    Cursor    string
    Direction CursorDirection
    Size      int
}
```

**字段说明**:
- `Cursor`: 当前游标值，第一页请求时为空字符串
- `Direction`: 分页方向，`CursorForward`（前向）或 `CursorBackward`（后向）
- `Size`: 每页记录数，必须为正整数且不超过 `MaxPageSize`

**创建函数**: `NewCursorPageRequest(cursor, direction, size)`，包含完整参数校验

### 3.3 OffsetPageRequest

偏移量分页的请求参数结构体。

```go
type OffsetPageRequest struct {
    Page int
    Size int
}
```

**字段说明**:
- `Page`: 当前页码，从 1 开始计数
- `Size`: 每页记录数，必须为正整数且不超过 `MaxPageSize`

**主要方法**:
- `Offset() int`: 计算偏移量 = `(Page - 1) * Size`
- `Limit() int`: 返回 `Size`，即查询的记录条数限制

**创建函数**: `NewOffsetPageRequest(page, size)`，包含完整参数校验

### 3.4 CursorPageMeta

游标分页的元信息结构体。

```go
type CursorPageMeta struct {
    StartCursor   string
    EndCursor     string
    HasNextPage   bool
    HasPrevPage   bool
    NextCursor    string
    PrevCursor    string
    TotalCount    *int64
    TotalPages    *int
    CurrentCursor string
    PageSize      int
}
```

**职责**:
- `StartCursor` / `EndCursor`: 当前页第一条和最后一条记录的游标值
- `HasNextPage` / `HasPrevPage`: 是否存在下一页/上一页
- `NextCursor` / `PrevCursor`: 下一页/上一页的游标值（仅当对应页存在时设置）
- `TotalCount`: 满足条件的总记录数，指针类型（`nil` 表示未提供，游标分页不强制）
- `TotalPages`: 总页数，指针类型（`nil` 表示未计算）
- `CurrentCursor`: 请求传入的当前游标值
- `PageSize`: 每页大小，用于 `SetTotal` 时计算总页数

### 3.5 OffsetPageMeta

偏移量分页的元信息结构体。

```go
type OffsetPageMeta struct {
    CurrentPage int
    PageSize    int
    TotalPages  int
    TotalCount  int64
    HasNextPage bool
    HasPrevPage bool
}
```

**职责**:
- `CurrentPage`: 当前页码（从 1 开始）
- `PageSize`: 每页记录数
- `TotalPages`: 总页数 = `ceil(TotalCount / PageSize)`
- `TotalCount`: 满足条件的总记录数
- `HasNextPage` / `HasPrevPage`: 是否存在下一页/上一页

### 3.6 CursorNav

游标分页的导航信息结构体。

```go
type CursorNav struct {
    NextCursor string
    PrevCursor string
}
```

**职责**: 提供纯粹的导航游标字段，方便前端直接取用上一页/下一页的游标值。

### 3.7 OffsetNav

偏移量分页的导航信息结构体。

```go
type OffsetNav struct {
    FirstPage int
    LastPage  int
    PrevPage  *int
    NextPage  *int
}
```

**职责**:
- `FirstPage`: 第一页页码，恒为 1
- `LastPage`: 最后一页页码（等于总页数）
- `PrevPage`: 上一页页码（指针，`nil` 表示无上一页）
- `NextPage`: 下一页页码（指针，`nil` 表示无下一页）

---

## 4. 游标分页机制

### 4.1 前向分页 (Forward)

前向分页从当前游标值**之后**获取记录，适用于常规的"加载更多"场景：

```
请求: cursor=item-00010, direction=forward, size=10

数据库查询: WHERE id > 'item-00010' ORDER BY id ASC LIMIT 11
                                                        ↑
                                                 多查 1 条判断是否有下一页

响应:
  Data:        [item-00011, ..., item-00020]  (共 10 条)
  StartCursor: "item-00011"
  EndCursor:   "item-00020"
  HasNextPage: true  (因为查到了第 11 条)
  HasPrevPage: true  (cursor 非空表示有前面的记录)
  NextCursor:  "item-00020"  (用于下次 forward 请求)
  PrevCursor:  "item-00011"  (用于下次 backward 请求)
```

### 4.2 后向分页 (Backward)

后向分页从当前游标值**之前**获取记录，适用于"上一页"或"向前翻页"场景：

```
请求: cursor=item-00021, direction=backward, size=10

数据库查询: WHERE id < 'item-00021' ORDER BY id DESC LIMIT 11
                                                         ↑
                                                  多查 1 条判断是否有上一页
返回后再反转顺序，保证正向排列

响应:
  Data:        [item-00011, ..., item-00020]  (共 10 条，已反转)
  StartCursor: "item-00011"
  EndCursor:   "item-00020"
  HasNextPage: true  (cursor 非空表示有后面的记录)
  HasPrevPage: true  (因为查到了第 11 条)
  NextCursor:  "item-00020"
  PrevCursor:  "item-00011"
```

### 4.3 hasMore 判断约定

调用方在执行数据查询时，需要：
1. **多查 1 条**记录（即 `LIMIT size + 1`）
2. 根据实际返回条数判断是否有更多页：
   - 前向分页时，若返回 `size + 1` 条，则 `hasMoreAfter = true`，截断最后 1 条
   - 后向分页时，若返回 `size + 1` 条，则 `hasMoreBefore = true`，截断最后 1 条
3. 将截断后的 `size` 条记录和布尔标志传入构建函数

### 4.4 游标值提取

模块通过 `cursorFn func(T) string` 回调函数从每条记录中提取游标值，支持任意字段作为游标（ID、创建时间、排序字段组合等）。若不提供 `cursorFn`，则所有游标相关字段为空字符串，但 `HasNextPage`/`HasPrevPage` 标志仍然有效。

---

## 5. 偏移量分页机制

### 5.1 基本原理

偏移量分页通过页码和每页大小计算偏移量：

```
Offset = (Page - 1) * Size

请求: page=3, size=10
  → Offset = (3-1) * 10 = 20
  → 数据库查询: LIMIT 10 OFFSET 20
  → 返回第 21-30 条记录
```

### 5.2 总页数计算

```go
TotalPages = int(math.Ceil(float64(TotalCount) / float64(PageSize)))
```

| TotalCount | PageSize | TotalPages | 说明 |
|------------|----------|------------|------|
| 0 | 10 | 0 | 无记录 |
| 1 | 10 | 1 | 不满 1 页按 1 页算 |
| 10 | 10 | 1 | 恰好 1 页 |
| 11 | 10 | 2 | 超出部分需单独 1 页 |
| 100 | 10 | 10 | 恰好 10 页 |

### 5.3 导航标志

| 条件 | HasPrevPage | HasNextPage |
|------|-------------|-------------|
| Page == 1 | false | Page < TotalPages |
| 1 < Page < TotalPages | true | true |
| Page == TotalPages (且 TotalPages > 0) | true | false |
| Page > TotalPages (且 TotalPages > 0) | true (Page > 1) | false |
| TotalPages == 0 | false | false |

---

## 6. 总条数统计与注入

### 6.1 两种提供方式

**方式一：构建响应时直接传入（推荐）**

```go
// 偏移量分页：一次性传入
resp := pagination.BuildOffsetResponse(items, req, totalCount)

// 游标分页：一次性传入
resp := pagination.BuildCursorResponseWithTotal(items, req, cursorFn, hasMoreAfter, hasMoreBefore, totalCount)
```

**方式二：先构建再注入（适用于分步查询场景）**

```go
// 1. 先执行分页查询，构建响应
resp := pagination.BuildOffsetResponse(pageItems, req, 0)

// 2. 再执行 COUNT 查询，注入总条数
totalCount := db.Count("SELECT COUNT(*) FROM ...")
resp.SetTotal(totalCount)
```

### 6.2 SetTotal 的自动更新

调用 `SetTotal(total int64)` 时，根据响应类型自动更新：

**偏移量分页**:
1. 设置 `TotalCount = total`
2. 重新计算 `TotalPages = ceil(total / PageSize)`
3. 重新评估 `HasNextPage` 和 `HasPrevPage`
4. 同步更新 `OffsetNav` 中的 `LastPage`、`PrevPage`、`NextPage`

**游标分页**:
1. 设置 `TotalCount = &total`
2. 若 `PageSize > 0`，计算并设置 `TotalPages = &ceil(total / PageSize)`
3. 游标分页不强制要求总条数，故 `TotalCount` 和 `TotalPages` 使用指针类型（可为 `nil`）

---

## 7. 标准化响应封装

### 7.1 游标分页响应结构示例

```json
{
  "Data": [
    { "ID": "item-00011", "Name": "Item 11" },
    { "ID": "item-00012", "Name": "Item 12" }
  ],
  "Meta": {
    "StartCursor": "item-00011",
    "EndCursor": "item-00020",
    "HasNextPage": true,
    "HasPrevPage": true,
    "NextCursor": "item-00020",
    "PrevCursor": "item-00011",
    "TotalCount": 250,
    "TotalPages": 25,
    "CurrentCursor": "item-00010",
    "PageSize": 10
  },
  "Nav": {
    "NextCursor": "item-00020",
    "PrevCursor": "item-00011"
  },
  "Success": true
}
```

### 7.2 偏移量分页响应结构示例

```json
{
  "Data": [
    { "ID": "item-00021", "Name": "Item 21" },
    { "ID": "item-00022", "Name": "Item 22" }
  ],
  "Meta": {
    "CurrentPage": 3,
    "PageSize": 10,
    "TotalPages": 25,
    "TotalCount": 250,
    "HasNextPage": true,
    "HasPrevPage": true
  },
  "Nav": {
    "FirstPage": 1,
    "LastPage": 25,
    "PrevPage": 2,
    "NextPage": 4
  },
  "Success": true
}
```

---

## 8. 适用场景对比

| 维度 | 游标分页 (Cursor) | 偏移量分页 (Offset) |
|------|-------------------|---------------------|
| **性能** | 优。基于索引字段定位，O(log n) 查询，数据量大时稳定 | 差。大偏移量时 `OFFSET` 跳过大量记录，O(n) 性能退化 |
| **数据一致性** | 优。新增/删除记录不会导致漏读或重复 | 差。数据变动时翻页可能漏读或重复读到记录 |
| **随机跳转** | 不支持。只能顺序前翻/后翻 | 支持。可直接跳转到任意页码 |
| **适用场景** | 社交信息流、评论列表、时间线、"加载更多"按钮 | 后台管理列表、搜索结果、需要页码导航的场景 |
| **总条数要求** | 不强制。可只提供 `hasMore` 标志，不查 COUNT | 强制需要。计算总页数和导航必须知道总条数 |
| **实现复杂度** | 较高。需要选择合适的游标字段，处理双向查询 | 较低。直接使用 `LIMIT/OFFSET` 即可 |
| **深分页表现** | 稳定。无论翻到第几页性能都一致 | 极差。第 10000 页可能需要几秒甚至几十秒 |
| **排序要求** | 游标字段必须参与排序且唯一有序 | 灵活。任意排序方式均可 |

### 场景选择建议

**优先使用游标分页**:
- 面向用户的前端列表（App、移动端 H5）
- 数据量巨大且持续增长的时序数据
- 使用"加载更多"而非"页码导航"的交互
- 对性能和数据一致性有高要求

**优先使用偏移量分页**:
- 后台管理系统的表格列表
- 需要显示总页数、跳转到指定页码的场景
- 需要精确知道某条记录在第几页
- 数据量较小（< 10 万条），性能差异不明显

---

## 9. 使用示例

### 9.1 基础：偏移量分页查询用户列表

```go
package main

import (
    "fmt"
    "solocoder-go/internal/pagination"
)

type User struct {
    ID   int64
    Name string
}

func ListUsers(page, size int) (*pagination.PageResponse[User], error) {
    // 1. 构建并校验分页请求
    req, err := pagination.NewOffsetPageRequest(page, size)
    if err != nil {
        return nil, fmt.Errorf("invalid pagination params: %w", err)
    }

    // 2. 执行 COUNT 查询
    totalCount := int64(156) // 实际从 DB 查询: SELECT COUNT(*) FROM users

    // 3. 执行分页查询
    offset := req.Offset()
    limit := req.Limit()
    users := queryUsers(offset, limit) // 实际: SELECT * FROM users LIMIT ? OFFSET ?

    // 4. 构建标准化响应
    resp := pagination.BuildOffsetResponse(users, req, totalCount)

    return resp, nil
}

func main() {
    resp, _ := ListUsers(3, 10)

    meta := resp.Meta.(*pagination.OffsetPageMeta)
    nav := resp.Nav.(*pagination.OffsetNav)

    fmt.Printf("当前第 %d 页，共 %d 条记录\n", meta.CurrentPage, meta.TotalCount)
    fmt.Printf("每页 %d 条，共 %d 页\n", meta.PageSize, meta.TotalPages)
    if nav.PrevPage != nil {
        fmt.Printf("上一页: %d\n", *nav.PrevPage)
    }
    if nav.NextPage != nil {
        fmt.Printf("下一页: %d\n", *nav.NextPage)
    }

    for _, user := range resp.Data {
        fmt.Printf("  - User %d: %s\n", user.ID, user.Name)
    }
}
```

### 9.2 进阶：游标分页加载动态流

```go
package main

import (
    "fmt"
    "solocoder-go/internal/pagination"
)

type Post struct {
    ID        string
    Title     string
    CreatedAt int64
}

// 游标使用 ID（假设 ID 为有序 ULID/UUID v7）
func postCursor(p Post) string {
    return p.ID
}

// 前向分页：获取指定游标之后的文章
func FetchPostsForward(cursor string, size int) (*pagination.PageResponse[Post], error) {
    req, err := pagination.NewCursorPageRequest(cursor, pagination.CursorForward, size)
    if err != nil {
        return nil, err
    }

    // 多查 1 条判断是否有更多
    queryLimit := size + 1

    var posts []Post
    if cursor == "" {
        // 第一页：无游标条件
        posts = queryPosts("", queryLimit)
    } else {
        // 后续页：ID > cursor
        posts = queryPostsAfter(cursor, queryLimit)
    }

    // 判断是否有下一页，并截断多余的 1 条
    hasMoreAfter := len(posts) > size
    if hasMoreAfter {
        posts = posts[:size]
    }

    // 第一页无上一页；非第一页有上一页（因为 cursor 非空）
    hasMoreBefore := cursor != ""

    // 构建响应（不强制传入总条数）
    resp := pagination.BuildCursorResponse(posts, req, postCursor, hasMoreAfter, hasMoreBefore)

    // 可选：异步查询总条数后注入
    go func() {
        total := int64(countAllPosts())
        resp.SetTotal(total)
    }()

    return resp, nil
}

// 完整翻页流程
func main() {
    var allPosts []Post
    cursor := ""

    for {
        resp, _ := FetchPostsForward(cursor, 20)
        allPosts = append(allPosts, resp.Data...)

        meta := resp.Meta.(*pagination.CursorPageMeta)
        if !meta.HasNextPage {
            break // 已到最后一页
        }

        // 使用 EndCursor 作为下一页的起点
        cursor = meta.EndCursor
    }

    fmt.Printf("共加载 %d 篇文章\n", len(allPosts))
}
```

### 9.3 SetTotal 分步注入示例

```go
// 数据访问层：先查分页数据，再查总数（或并行查询）
func SearchArticles(keyword string, page, size int) *pagination.PageResponse[Article] {
    req, _ := pagination.NewOffsetPageRequest(page, size)

    // 先构建响应（总条数先填 0）
    items := searchWithPagination(keyword, req.Offset(), req.Limit())
    resp := pagination.BuildOffsetResponse(items, req, 0)

    // 再执行 COUNT 查询，注入总条数
    total := countSearchResults(keyword)
    resp.SetTotal(total) // 自动更新 TotalPages、HasNextPage、Nav 等

    return resp
}
```

### 9.4 边界场景：页码超出范围

```go
resp, _ := ListUsers(999, 10) // 总共只有 156 条，最多 16 页

meta := resp.Meta.(*pagination.OffsetPageMeta)
fmt.Printf("请求页: %d\n", meta.CurrentPage)    // 999
fmt.Printf("总页数: %d\n", meta.TotalPages)      // 16
fmt.Printf("数据条数: %d\n", len(resp.Data))    // 0（超出范围返回空列表，不报错）
fmt.Printf("有上一页: %v\n", meta.HasPrevPage)  // true（999 > 1）
fmt.Printf("有下一页: %v\n", meta.HasNextPage)  // false
```

### 9.5 参数预校验

```go
func handler(page, size int) error {
    // 提前校验参数，不通过直接返回 400
    if err := pagination.ValidateOffsetRequest(page, size); err != nil {
        return fmt.Errorf("bad request: %w", err)
    }
    // 继续处理...
    return nil
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidPageSize` | 每页条数非法 | `size <= 0` |
| `ErrInvalidPageNumber` | 页码非法 | `page <= 0`（偏移量分页） |
| `ErrPageSizeExceedsMax` | 每页条数超过上限 | `size > MaxPageSize`（默认 1000） |
| `ErrNilData` | 数据切片为 nil | 保留错误，当前构建函数自动处理 nil |

**参数校验规则**:

| 参数 | 有效值范围 | 默认值 |
|------|-----------|--------|
| `Page` (偏移量) | `>= 1` | 无，必须显式提供 |
| `Cursor` | 任意字符串（含空串） | `""`（空串表示第一页） |
| `Direction` | `"forward"` / `"backward"` / `""` | `""` → `"forward"` |
| `Size` | `1 <= size <= 1000` | 无，必须显式提供 |

---

## 11. 边界条件处理

### 11.1 偏移量分页边界

| 场景 | 处理方式 |
|------|----------|
| 数据为空 (`total = 0`) | `Data = []`, `TotalPages = 0`, 所有导航标志为 false |
| 请求第 1 页 | `HasPrevPage = false`, `PrevPage = nil` |
| 请求最后一页 (`Page = TotalPages`) | `HasNextPage = false`, `NextPage = nil` |
| `Page > TotalPages > 0` | 返回空 `Data`，保留 `CurrentPage` 值，不报错 |
| `Size > TotalCount > 0` | `TotalPages = 1`，所有数据在第一页 |
| 整除情况 (`TotalCount = n * Size`) | `TotalPages = n`，最后一页 `HasNextPage = false` |

### 11.2 游标分页边界

| 场景 | 处理方式 |
|------|----------|
| 第一页请求 (`cursor = ""`) | `HasPrevPage = false`, `PrevCursor = ""` |
| 最后一页 (`hasMoreAfter = false`) | `HasNextPage = false`, `NextCursor = ""` |
| `nil cursorFn` | 所有游标字段为空串，但 `Has*Page` 标志正常 |
| 空数据列表 | `StartCursor = EndCursor = ""`，导航游标均为空 |
| `SetTotal` 未调用 | `TotalCount = nil`, `TotalPages = nil`（合法状态） |
| 不提供总条数 | 完全合法，`hasMore` 标志足以支持翻页 |

### 11.3 构造函数返回约定

- 所有 `New*Request` 构造函数：参数校验失败返回 `(nil, error)`
- 所有 `Build*Response` 构造函数：永不返回 `nil`，`Data` 恒为非 nil 切片（输入 nil 时转为空切片）
- `SetTotal()`：成功返回 `nil`，不支持的 Meta 类型返回错误（正常使用不会触发）

---
