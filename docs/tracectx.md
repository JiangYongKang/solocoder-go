# 分布式追踪上下文传播器 (tracectx)

## 模块功能

`tracectx` 包提供了一个轻量级的分布式追踪上下文传播器，用于在分布式系统中传递和管理追踪信息。该模块实现了以下核心功能：

1. **TraceID/SpanID/ParentSpanID 生成与传播**：全局唯一标识分布式请求链路，支持进程间传递
2. **Span 树构建**：根据 ParentSpanID 关系构建完整的调用树，支持节点添加和子树查询
3. **采样策略**：支持全量采样和概率采样，采样决策在根 Span 创建时确定并沿调用链传播
4. **跨进程上下文注入/提取**：兼容 W3C Trace Context 标准，支持 HTTP 请求头的注入和提取

## 核心结构体

### Sampler 接口

采样策略接口，定义了采样决策的标准方法。

```go
type Sampler interface {
    ShouldSample(traceID string) bool
}
```

### AlwaysSample

全量采样器，所有请求都被采样。

### NeverSample

从不采样器，所有请求都不被采样。

### ProbabilitySampler

概率采样器，根据配置的概率对请求进行采样。采样决策基于 TraceID 的哈希值，确保同一个 Trace 始终被采样或不被采样。

### Span

表示追踪链路中的一个操作单元。

**字段说明**：
- `TraceID`：全局唯一的追踪标识，16 字节（32 个十六进制字符）
- `SpanID`：当前操作的唯一标识，8 字节（16 个十六进制字符）
- `ParentSpanID`：父操作的 SpanID，根 Span 为空字符串
- `Name`：操作名称，用于标识该 Span 的业务含义
- `Sampled`：采样标志，表示该 Span 是否被采样
- `StartTime`：开始时间戳
- `EndTime`：结束时间戳
- `Attributes`：自定义属性键值对

**方法**：
- `SetAttribute(key, value string)`：设置 Span 属性
- `GetAttribute(key string) (string, bool)`：获取 Span 属性
- `IsRoot() bool`：判断是否为根 Span

### SpanTree

Span 调用树，用于管理和遍历 Span 之间的父子关系。同一棵 SpanTree 中只允许包含相同 TraceID 的 Span，确保追踪链路的数据一致性。

**方法**：
- `AddSpan(span *Span) error`：向树中添加 Span 节点（含 TraceID 一致性校验）
- `GetSpan(spanID string) (*Span, error)`：根据 SpanID 获取 Span
- `GetChildren(parentSpanID string) ([]*Span, error)`：获取指定 Span 的直接子节点
- `GetRoots() []*Span`：获取所有根节点
- `GetSubtree(rootSpanID string) ([]*Span, error)`：获取以指定 Span 为根的子树（包含根节点）
- `AllSpans() []*Span`：获取树中所有 Span
- `SpanCount() int`：获取树中 Span 总数
- `TraceID() string`：获取该树绑定的 TraceID

### TraceContext

追踪上下文，包含在进程间传递的核心追踪信息。

**字段说明**：
- `TraceID`：全局唯一的追踪标识
- `SpanID`：当前 Span 标识
- `ParentSpanID`：父 Span 标识。从 W3C Trace Context 提取时，该字段被设为请求头中 `parent-id` 的值，表示上游调用方的 SpanID
- `Sampled`：采样标志

**方法**：
- `String() string`：格式化为 W3C Trace Parent 字符串

## SpanTree 的数据一致性约束

### TraceID 一致性约束

SpanTree 在添加 Span 时会进行 TraceID 一致性校验，确保同一棵树中的所有 Span 属于同一条追踪链路：

1. **首个 Span 设置 TraceID**：当向空树添加第一个 Span 时，树的 `traceID` 字段被设置为该 Span 的 TraceID
2. **后续 Span 必须匹配**：后续添加的 Span 的 TraceID 必须与树已有的 TraceID 一致，否则返回 `ErrTraceIDMismatch` 错误
3. **防止链路混入**：该约束防止不同追踪链路的 Span 被错误地混入同一棵树，保证数据完整性和查询正确性

```go
tree := tracectx.NewSpanTree()
span1 := tracectx.NewSpan("trace-aaa", "span1", "", "root", true)
tree.AddSpan(span1)  // OK, tree.traceID = "trace-aaa"

span2 := tracectx.NewSpan("trace-bbb", "span2", "", "root", true)
err := tree.AddSpan(span2)  // 返回 ErrTraceIDMismatch
```

### SpanID 唯一性约束

同一棵树中不允许存在相同 SpanID 的 Span，添加重复 SpanID 会返回 `ErrDuplicateSpanID` 错误。

## SpanTree 的查询性能特性

### 内部数据结构

SpanTree 内部维护了三个索引结构：

1. **`spans map[string]*Span`**：SpanID → Span 的哈希表，支持 O(1) 的按 SpanID 查找
2. **`children map[string][]*Span`**：ParentSpanID → 子 Span 列表的哈希表，支持 O(1) 的直接子节点查找
3. **`roots []*Span`**：根节点列表，存储所有 ParentSpanID 为空的 Span

### 查询复杂度

| 操作 | 复杂度 | 说明 |
|------|--------|------|
| `AddSpan` | O(1) | 哈希表插入 + children 索引更新 |
| `GetSpan` | O(1) | 哈希表查找 |
| `GetChildren` | O(k) | k 为该节点的子节点数量，哈希表直接定位 |
| `GetRoots` | O(r) | r 为根节点数量，直接复制切片 |
| `GetSubtree` | O(n) | n 为子树节点总数，递归遍历 children 索引 |
| `SpanCount` | O(1) | 直接返回 map 长度 |
| `TraceID` | O(1) | 直接返回字段值 |

### 性能优化说明

`GetChildren` 和 `GetSubtree` 通过 `children` 哈希索引实现了 O(1) 的子节点定位，避免了旧版全量遍历所有 Span 的 O(N) 扫描。在节点数量较多的场景下（数千个 Span），性能提升显著。`GetSubtree` 的递归遍历仅访问子树内的节点，不遍历树的其他部分。

### 并发安全

SpanTree 使用 `sync.RWMutex` 读写锁保证并发安全，支持多 goroutine 同时读取和互斥写入。`GetChildren` 返回的是子节点切片的副本，修改返回值不会影响内部数据结构。

## Span 树的结构与遍历

### 树结构

每个 Span 通过 `ParentSpanID` 字段建立父子关系。

```
root (ParentSpanID = "")
├── child1 (ParentSpanID = root.SpanID)
│   ├── grandchild1 (ParentSpanID = child1.SpanID)
│   └── grandchild2 (ParentSpanID = child1.SpanID)
└── child2 (ParentSpanID = root.SpanID)
```

### 遍历方式

1. **获取直接子节点**：使用 `GetChildren(parentSpanID)` 获取指定节点的直接子节点列表
2. **获取子树**：使用 `GetSubtree(rootSpanID)` 递归获取以指定节点为根的所有后代节点（包含根节点）
3. **获取所有根节点**：使用 `GetRoots()` 获取所有没有父节点的 Span
4. **获取所有节点**：使用 `AllSpans()` 获取树中的所有 Span

## 使用示例

### 1. 创建根上下文和 Span

```go
import "solocoder-go/internal/tracectx"

sampler := tracectx.NewAlwaysSample()
ctx, rootSpan, err := tracectx.NewRootContext("service-a", sampler)
if err != nil {
    // 处理错误
}

// 使用 ctx 和 rootSpan
```

### 2. 创建子上下文

```go
childCtx, childSpan, err := tracectx.NewChildContext(parentCtx, "service-b")
if err != nil {
    // 处理错误
}
```

### 3. 构建 Span 树

```go
tree := tracectx.NewSpanTree()

// 添加 Span（自动校验 TraceID 一致性）
tree.AddSpan(rootSpan)
tree.AddSpan(childSpan)

// 查询树绑定的 TraceID
fmt.Println(tree.TraceID())

// 获取子节点
children, err := tree.GetChildren(rootSpan.SpanID)

// 获取子树
subtree, err := tree.GetSubtree(rootSpan.SpanID)
```

### 4. 上下文注入（HTTP 请求头）

```go
headers := tracectx.InjectTraceContext(ctx)
// headers["traceparent"] = "00-<traceid>-<spanid>-01"

// 将 headers 添加到 HTTP 请求头
for k, v := range headers {
    req.Header.Set(k, v)
}
```

### 5. 上下文提取（从 HTTP 请求头）

```go
// 从 HTTP 请求头提取
headers := make(map[string]string)
for k, v := range req.Header {
    headers[k] = v[0]
}

ctx, err := tracectx.ExtractTraceContext(headers)
if err != nil {
    // 处理错误，可能需要创建新的根上下文
}
// ctx.ParentSpanID 被设为上游调用方的 SpanID
// 后续通过 NewChildContext(ctx, name) 创建子上下文时，
// 子 Span 的 ParentSpanID 将自动指向 ctx.SpanID（即上游 Span）
```

### 6. 使用概率采样

```go
// 50% 采样率
sampler, err := tracectx.NewProbabilitySampler(0.5)
if err != nil {
    // 处理无效采样率
}

ctx, span, err := tracectx.NewRootContext("service-a", sampler)
if !span.Sampled {
    // 未被采样，可以跳过记录但仍需传播上下文
}
```

### 7. 完整工作流示例

```go
// 服务 A：创建根上下文
sampler := tracectx.NewAlwaysSample()
rootCtx, rootSpan, _ := tracectx.NewRootContext("gateway", sampler)

tree := tracectx.NewSpanTree()
tree.AddSpan(rootSpan)

// 调用服务 B：注入上下文
childCtx, childSpan, _ := tracectx.NewChildContext(rootCtx, "service-b")
tree.AddSpan(childSpan)

headers := tracectx.InjectTraceContext(childCtx)
// ... 发送 HTTP 请求，headers 放入请求头

// 服务 B：提取上下文
extractedCtx, _ := tracectx.ExtractTraceContext(headers)
// extractedCtx.ParentSpanID == childCtx.SpanID (上游调用方的 SpanID)

// 服务 B 调用服务 C
grandchildCtx, grandchildSpan, _ := tracectx.NewChildContext(extractedCtx, "service-c")
// grandchildSpan.ParentSpanID == extractedCtx.SpanID == childCtx.SpanID
tree.AddSpan(grandchildSpan)
```

## W3C Trace Context 格式

本模块兼容 W3C Trace Context 标准，使用 `traceparent` 请求头传递追踪信息。

**格式**：
```
traceparent: <version>-<trace-id>-<parent-id>-<trace-flags>
```

**字段说明**：
- `version`：版本号，固定为 `00`（2 个十六进制字符）
- `trace-id`：TraceID，16 字节（32 个十六进制字符）
- `parent-id`：调用方的 SpanID，8 字节（16 个十六进制字符）。提取时该值同时设为 `TraceContext.SpanID` 和 `TraceContext.ParentSpanID`
- `trace-flags`：标志位，1 字节（2 个十六进制字符），最低位为采样标志

**ParentSpanID 传播语义**：由于 W3C `traceparent` 仅包含一个 `parent-id` 字段，提取上下文时 `ParentSpanID` 被设为 `parent-id` 的值，表示该 Span 是所有下游子 Span 的父节点。这确保了 `NewChildContext(extractedCtx, name)` 创建的子 Span 能够正确建立与上游 Span 的父子关系。

**示例**：
```
traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
```

## 错误类型

| 错误 | 触发场景 |
|------|----------|
| `ErrInvalidTraceID` | 提取时 TraceID 格式无效 |
| `ErrInvalidSpanID` | 提取时 SpanID 格式无效 |
| `ErrInvalidTraceParent` | traceparent 头格式错误或缺失 |
| `ErrSpanNotFound` | 查询不存在的 Span |
| `ErrNilSpan` | 添加 nil Span |
| `ErrDuplicateSpanID` | 添加重复 SpanID |
| `ErrInvalidSamplingRate` | 采样率不在 [0, 1] 范围内 |
| `ErrEmptySpanID` | SpanID 为空 |
| `ErrEmptyTraceID` | TraceID 为空 |
| `ErrTraceIDMismatch` | Span 的 TraceID 与树已有的 TraceID 不一致 |
