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

Span 调用树，用于管理和遍历 Span 之间的父子关系。

**方法**：
- `AddSpan(span *Span) error`：向树中添加 Span 节点
- `GetSpan(spanID string) (*Span, error)`：根据 SpanID 获取 Span
- `GetChildren(parentSpanID string) ([]*Span, error)`：获取指定 Span 的直接子节点
- `GetRoots() []*Span`：获取所有根节点
- `GetSubtree(rootSpanID string) ([]*Span, error)`：获取以指定 Span 为根的子树（包含根节点）
- `AllSpans() []*Span`：获取树中所有 Span
- `SpanCount() int`：获取树中 Span 总数

### TraceContext

追踪上下文，包含在进程间传递的核心追踪信息。

**字段说明**：
- `TraceID`：全局唯一的追踪标识
- `SpanID`：当前 Span 标识
- `ParentSpanID`：父 Span 标识
- `Sampled`：采样标志

**方法**：
- `String() string`：格式化为 W3C Trace Parent 字符串

## Span 树的结构与遍历

### 树结构

SpanTree 内部使用 `map[string]*Span` 存储所有 Span，并维护一个根节点列表 `roots`。每个 Span 通过 `ParentSpanID` 字段建立父子关系。

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

### 并发安全

SpanTree 使用 `sync.RWMutex` 读写锁保证并发安全，支持多 goroutine 同时读取和互斥写入。

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

// 添加 Span
tree.AddSpan(rootSpan)
tree.AddSpan(childSpan)

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

// 服务 B 调用服务 C
grandchildCtx, grandchildSpan, _ := tracectx.NewChildContext(extractedCtx, "service-c")
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
- `parent-id`：SpanID，8 字节（16 个十六进制字符）
- `trace-flags`：标志位，1 字节（2 个十六进制字符），最低位为采样标志

**示例**：
```
traceparent: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
```
