# gRPC 服务框架 (grpcsvc) 模块需求文档

## 1. 模块概述

gRPC 服务框架是一个通用的 gRPC 服务端开发框架，提供服务注册、请求处理、拦截器链、超时传播和元数据透传等核心功能。通过抽象的 `Stream` 接口与具体的 gRPC 实现解耦，可以方便地集成不同的 gRPC 库。

本模块使用内存数据结构管理服务注册信息和拦截器链，通过互斥锁保证并发安全，支持高并发场景下的稳定运行。框架支持一元 RPC 调用以及服务端流式、客户端流式和双向流式三种流式 RPC 调用方式。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 服务注册 | 支持将服务实现注册到框架中，注册时指定服务名称和服务描述符 |
| F2 | 一元 RPC 调用 | 支持处理一元 RPC 请求（单次请求-单次响应） |
| F3 | 服务端流式 RPC | 支持服务端流式 RPC 调用，服务端可向客户端发送多条消息 |
| F4 | 客户端流式 RPC | 支持客户端流式 RPC 调用，客户端可向服务端发送多条消息 |
| F5 | 双向流式 RPC | 支持双向流式 RPC 调用，客户端和服务端可双向流式通信 |
| F6 | 一元拦截器链 | 支持注册多个一元拦截器并按注册顺序构建处理链 |
| F7 | 流式拦截器链 | 支持注册多个流式拦截器并按注册顺序构建处理链 |
| F8 | 超时传播 | 从请求上下文中读取客户端设置的超时时间，在处理过程中持续校验剩余超时时间 |
| F9 | 请求元数据透传 | 支持在请求头中携带自定义的键值对元数据，注入请求上下文 |
| F10 | 响应元数据回传 | 支持通过 header 和 trailer 元数据向客户端回传额外信息 |
| F11 | 服务查询 | 支持查询已注册的服务信息、服务数量和方法数量 |
| F12 | 服务启停 | 支持服务器的启动和停止，停止后拒绝新的请求 |
| F13 | 并发流控制 | 支持配置最大并发流数量，超出时拒绝新的流创建请求 |
| F14 | 连接超时 | 支持配置连接超时，自动为所有请求添加上下文超时 |
| F15 | 活跃流统计 | 支持查询当前活跃的流数量 |
| F16 | Header 元数据 | 支持在请求处理过程中设置 header 元数据，通过上下文回传给客户端 |
| F17 | Stream 接口完整性 | Stream 接口提供完整的双向通信能力，包括 Send、Recv、PutRecv 等方法 |
| F18 | Panic 恢复 | 在 handler 执行过程中捕获 panic，避免服务器崩溃 |

## 3. 核心结构体与职责

### 3.1 Server - gRPC 服务器

```go
type Server struct {
    mu                 sync.RWMutex
    services           map[string]*service
    unaryInterceptors  []UnaryInterceptor
    streamInterceptors []StreamInterceptor
    running            bool
    options            ServerOptions
    activeStreams      int32
}
```

**主要职责：**
- 管理服务注册信息，维护服务名到服务实现的映射
- 管理拦截器链，支持一元和流式两种拦截器类型
- 提供 RPC 调用入口，分发请求到对应服务的处理方法
- 维护服务器运行状态，支持优雅停止
- 管理并发流数量，使用原子操作保证计数安全
- 应用连接超时配置，自动为请求添加上下文超时
- 保证所有操作的并发安全

**核心方法：**
- `RegisterService(sd *ServiceDesc, srv interface{}) error` - 注册服务
- `AddUnaryInterceptor(interceptor UnaryInterceptor) error` - 添加一元拦截器
- `AddStreamInterceptor(interceptor StreamInterceptor) error` - 添加流式拦截器
- `Invoke(ctx context.Context, serviceName, methodName string, req interface{}) (interface{}, error)` - 调用一元方法
- `NewStream(ctx context.Context, serviceName, streamName string) (Stream, error)` - 创建流
- `HandleStream(ctx context.Context, serviceName, streamName string, stream Stream) error` - 处理流式调用
- `ActiveStreams() int` - 获取当前活跃流数量
- `Options() ServerOptions` - 获取服务器配置
- `Stop()` - 停止服务器

实现细节见 [Server](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L213-L739)

### 3.2 ServerOptions - 服务器配置

```go
type ServerOptions struct {
    MaxConcurrentStreams uint32
    ConnectionTimeout    time.Duration
}
```

**主要职责：**
- `MaxConcurrentStreams` - 最大并发流数量，默认为 100，超出时返回 ErrTooManyStreams
- `ConnectionTimeout` - 连接超时时间，默认为 30 秒，自动为所有请求添加上下文超时

### 3.3 ServiceDesc - 服务描述符

```go
type ServiceDesc struct {
    ServiceName string
    Methods     []MethodDesc
    Streams     []StreamDesc
}
```

**主要职责：**
- 描述一个 gRPC 服务的完整定义
- 包含服务名称、一元方法列表、流式方法列表

### 3.4 MethodDesc - 一元方法描述符

```go
type MethodDesc struct {
    MethodName string
    Handler    UnaryHandler
}
```

**主要职责：**
- 描述一个一元 RPC 方法
- 包含方法名和处理函数

### 3.5 StreamDesc - 流式方法描述符

```go
type StreamDesc struct {
    StreamName    string
    Handler       StreamHandler
    ServerStreams bool
    ClientStreams bool
}
```

**主要职责：**
- 描述一个流式 RPC 方法
- 包含流名称、处理函数和流类型标识
- `ServerStreams` 标识服务端是否流式发送
- `ClientStreams` 标识客户端是否流式发送

### 3.6 Stream - 流接口

```go
type Stream interface {
    Context() context.Context
    SendMsg(msg interface{}) error
    RecvMsg(msg interface{}) error
    Recv() (interface{}, error)
    RecvFromServer() (interface{}, error)
    PutRecv(msg interface{}) error
    SetHeader(md MD)
    SetTrailer(md MD)
    Header() (MD, bool)
    Close() error
    Closed() bool
}
```

**主要职责：**
- 抽象底层 gRPC 流，与具体实现解耦
- `Context()` - 获取流上下文
- `SendMsg(msg)` - 服务端向客户端发送消息（写入 sendCh），供服务端 handler 调用
- `RecvMsg(msg)` - 服务端从客户端接收消息（从 recvCh 读取），通过 reflect 将数据写入 msg 指针，msg 必须是非 nil 指针
- `Recv()` - 服务端从客户端接收消息并返回（从 recvCh 读取，返回消息内容）
- `RecvFromServer()` - 从 sendCh 读取服务端发送的消息（供客户端/测试方消费服务端响应），语义为"接收来自服务端的消息"
- `PutRecv(msg)` - 向 recvCh 写入消息（模拟客户端发送，供客户端/测试方注入客户端消息）
- `SetHeader(md)` - 设置响应 header
- `SetTrailer(md)` - 设置响应 trailer
- `Header()` - 获取已设置的 header
- `Close()` - 关闭流，同时调用 cancelFn 释放 context 资源
- `Closed()` - 查询流是否已关闭

**消息流向说明：**

| 方法 | 数据方向 | 使用方 |
|------|----------|--------|
| `SendMsg(msg)` | 服务端 → sendCh → 客户端 | 服务端 handler |
| `RecvFromServer()` | sendCh → 客户端/测试方 | 客户端/测试方 |
| `PutRecv(msg)` | 客户端/测试方 → recvCh | 客户端/测试方 |
| `Recv()` / `RecvMsg(msg)` | recvCh → 服务端 handler | 服务端 handler |

实现细节见 [Stream](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L152-L164)

### 3.7 MD - 元数据类型

```go
type MD map[string][]string
```

**主要职责：**
- 表示 gRPC 元数据（HTTP/2 header 的抽象）
- 支持多值键
- 提供 Get、Set、Add、Delete、Copy 等操作方法

实现细节见 [MD](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L50-L86)

### 3.8 UnaryInterceptor - 一元拦截器

```go
type UnaryInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error)
```

**主要职责：**
- 一元 RPC 拦截器函数类型
- 可在请求处理前后执行自定义逻辑（认证、日志、限流等）
- 通过调用 `handler` 继续执行后续处理链

### 3.9 StreamInterceptor - 流式拦截器

```go
type StreamInterceptor func(srv interface{}, ss Stream, info *StreamServerInfo, handler StreamHandler) error
```

**主要职责：**
- 流式 RPC 拦截器函数类型
- 可在流式调用处理前后执行自定义逻辑
- 通过调用 `handler` 继续执行后续处理链

### 3.10 UnaryServerInfo - 一元调用信息

```go
type UnaryServerInfo struct {
    Server      interface{}
    FullMethod  string
    ServiceName string
    MethodName  string
}
```

**主要职责：**
- 携带一元 RPC 调用的元信息
- 供拦截器获取调用上下文信息

### 3.11 StreamServerInfo - 流式调用信息

```go
type StreamServerInfo struct {
    Server         interface{}
    FullMethod     string
    ServiceName    string
    MethodName     string
    IsClientStream bool
    IsServerStream bool
}
```

**主要职责：**
- 携带流式 RPC 调用的元信息
- 标识流的类型（服务端流、客户端流、双向流）

### 3.12 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrServiceNotFound` | 服务不存在 | 调用未注册的服务 |
| `ErrServiceExists` | 服务已存在 | 重复注册同名服务 |
| `ErrMethodNotFound` | 方法不存在 | 调用不存在的方法 |
| `ErrInvalidServiceDesc` | 服务描述符无效 | 注册时传入无效的服务描述符 |
| `ErrInvalidMethodDesc` | 方法描述符无效 | 注册时传入无效的方法描述符 |
| `ErrServerStopped` | 服务器已停止 | 已停止的服务器上调用方法 |
| `ErrDeadlineExceeded` | 超时 | 请求处理超过了设置的截止时间 |
| `ErrStreamClosed` | 流已关闭 | 在已关闭的流上执行操作 |
| `ErrNilHandler` | 处理器为空 | 注册方法时 handler 为 nil |
| `ErrTooManyStreams` | 并发流过多 | 活跃流数量达到 MaxConcurrentStreams 限制 |
| `ErrConnectionTimeout` | 连接超时 | 连接超时（保留错误，实际通过 DeadlineExceeded 体现） |

## 4. 核心流程说明

### 4.1 服务注册流程

```
RegisterService(sd, srv)
   │
   ├─ 参数校验
   │     ├─ sd 为 nil → 返回 ErrInvalidServiceDesc
   │     ├─ ServiceName 为空 → 返回 ErrInvalidServiceDesc
   │     └─ srv 为 nil → 返回 ErrInvalidServiceDesc
   │
   ├─ mu.Lock() → 检查 running → 返回 ErrServerStopped
   │
   ├─ 检查服务是否已存在 → 存在 → 返回 ErrServiceExists
   │
   ├─ 创建 service 对象
   │     ├─ 遍历 Methods，校验每个方法
   │     │     ├─ MethodName 为空 → 返回 ErrInvalidMethodDesc
   │     │     └─ Handler 为 nil → 返回 ErrNilHandler
   │     └─ 遍历 Streams，校验每个流
   │           ├─ StreamName 为空 → 返回 ErrInvalidMethodDesc
   │           └─ Handler 为 nil → 返回 ErrNilHandler
   │
   ├─ 加入 services 映射
   │
   └─ 返回 nil
```

### 4.2 一元调用流程

```
Invoke(ctx, serviceName, methodName, req)
   │
   ├─ mu.RLock() → 检查 running → 返回 ErrServerStopped
   │
   ├─ 查找服务 → 不存在 → 返回 ErrServiceNotFound
   │
   ├─ 查找方法 → 不存在 → 返回 ErrMethodNotFound
   │
   ├─ 获取拦截器链和服务实现
   │
   ├─ mu.RUnlock()
   │
   ├─ 检查超时 → 已超时 → 返回 ErrDeadlineExceeded
   │
   ├─ 确保上下文有 header 和 trailer 存储空间
   │
   ├─ 应用 ConnectionTimeout → 创建带超时的上下文
   │
   ├─ 构建 UnaryServerInfo
   │
   └─ 执行拦截器链 + 持续超时检查
        │
        ├─ 启动单 goroutine 执行实际 handler（含 panic 恢复）
        │
        └─ 调用方直接 select 多路复用：
              ├─ ticker 触发 → 检查超时 → 超时则返回 ErrDeadlineExceeded
              ├─ handler 完成 → 返回结果
              └─ ctx 取消 → 返回对应错误
```

实现细节见 [Invoke()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L403-L513)

### 4.3 流式调用流程

```
HandleStream(ctx, serviceName, streamName, stream)
   │
   ├─ mu.RLock() → 检查 running → 返回 ErrServerStopped
   │
   ├─ 查找服务 → 不存在 → 返回 ErrServiceNotFound
   │
   ├─ 查找流方法 → 不存在 → 返回 ErrMethodNotFound
   │
   ├─ 获取拦截器链和服务实现
   │
   ├─ mu.RUnlock()
   │
   ├─ 检查超时 → 已超时 → 返回 ErrDeadlineExceeded
   │
   ├─ 确保上下文有 header 和 trailer 存储空间
   │
   ├─ 构建 StreamServerInfo
   │
   └─ 执行拦截器链 + 持续超时检查
        │
        ├─ 启动单 goroutine 执行实际 stream handler（含 panic 恢复）
        │
        └─ 调用方直接 select 多路复用：
              ├─ ticker 触发 → 检查超时 → 超时则返回 ErrDeadlineExceeded
              ├─ handler 完成 → 返回结果
              └─ ctx 取消 → 返回对应错误
```

实现细节见 [HandleStream()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L569-L676)

### 4.4 流创建与并发控制流程

```
NewStream(ctx, serviceName, streamName)
   │
   ├─ mu.RLock() → 检查 running → 返回 ErrServerStopped
   │
   ├─ 查找服务 → 不存在 → 返回 ErrServiceNotFound
   │
   ├─ 查找流方法 → 不存在 → 返回 ErrMethodNotFound
   │
   ├─ mu.RUnlock()
   │
   ├─ 检查超时 → 已超时 → 返回 ErrDeadlineExceeded
   │
   ├─ acquireStream() → 原子增加活跃流计数
   │     └─ 超过 MaxConcurrentStreams → 返回 ErrTooManyStreams
   │
   ├─ 确保上下文有 header 和 trailer 存储空间
   │
   ├─ 应用 ConnectionTimeout → 创建带超时的上下文，保存 cancelFn
   │
   ├─ 创建 serverStream，注册 cancelFn 和 releaseFn
   │
   └─ 返回 Stream 接口
```

实现细节见 [NewStream()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L515-L567)

### 4.5 拦截器链执行顺序

拦截器按照注册顺序执行，形成洋葱模型：

```
请求 → 拦截器1 before → 拦截器2 before → ... → 实际处理器
                ↑                                    ↓
                └── 拦截器1 after ← 拦截器2 after ← ┘
```

**执行顺序说明：**
1. 先注册的拦截器先执行 before 逻辑
2. 后注册的拦截器更靠近实际处理器
3. after 逻辑的执行顺序与 before 相反
4. 拦截器可以通过不调用 `handler` 来中断请求处理

实现细节见 [UnaryInterceptorChain()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L343-L361)

### 4.6 超时持续校验流程

```
Handler 包装器（一元和流式相同）
   │
   ├─ 启动单 goroutine 执行实际 handler
   │     ├─ defer 捕获 panic → 返回 panic 错误
   │     └─ 将结果写入 handlerDone channel
   │
   ├─ 创建 ticker = time.NewTicker(10ms)
   ├─ defer ticker.Stop()
   │
   └─ for 循环 select 多路复用：
         ├─ case <-ticker.C:
         │     └─ checkDeadline(ctx) → 超时 → 返回 ErrDeadlineExceeded
         ├─ case r := <-handlerDone:
         │     └─ 返回 handler 结果
         └─ case <-ctx.Done():
               ├─ DeadlineExceeded → 返回 ErrDeadlineExceeded
               └─ 其他 → 返回 ctx.Err()
```

**关键特性：**
- 每 10ms 检查一次超时
- handler 在独立 goroutine 中执行，即使阻塞也不影响超时检查
- 超时后立即返回错误，即使 handler 仍在后台运行
- 支持 panic 恢复，避免服务器崩溃
- 同时监听 ctx.Done() 信号
- 单 goroutine + select 多路复用，避免双层 goroutine 嵌套的调度开销

实现细节见 [Invoke() handler wrapper](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L450-L510)

### 4.7 元数据透传流程

**请求元数据（入站）：**
```
客户端 → HTTP/2 Headers → 框架解析为 MD → 注入 context → 拦截器/业务逻辑读取
```

**响应 Header 元数据（出站）：**
```
业务逻辑 → SetHeader(ctx, md) → 写入 context → 框架读取 → HTTP/2 Headers → 客户端
```

**响应 Trailer 元数据（出站）：**
```
业务逻辑 → SetTrailer(ctx, md) → 写入 context → 框架读取 → HTTP/2 Trailers → 客户端
```

**使用方式：**
- 服务端：通过 `FromContext(ctx)` 读取请求元数据，通过 `SetHeader(ctx, md)` 和 `SetTrailer(ctx, md)` 设置响应元数据
- 客户端：通过 `NewContextWithMD(ctx, md)` 附加请求元数据，通过 `HeaderFromContext(ctx)` 和 `TrailerFromContext(ctx)` 读取响应元数据

## 5. 核心机制说明

### 5.1 服务注册机制

服务采用"描述符 + 实现"分离的注册模式：
- `ServiceDesc` 描述服务的接口定义（方法名、处理函数等）
- `srv` 是服务的具体实现对象
- 框架通过闭包的方式将调用分发到具体实现

### 5.2 拦截器链构建机制

拦截器链采用"洋葱模型"设计：
- 通过闭包嵌套的方式构建处理链
- 每个拦截器接收 `handler` 参数，即链中的下一个处理器
- 从最后一个拦截器开始向前构建，最外层是第一个注册的拦截器
- 支持零个或多个拦截器

实现细节见 [ChainUnaryInterceptors()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L914-L930)

### 5.3 超时持续校验机制

超时采用"单 goroutine handler + select 多路复用"模式：
- 实际 handler 在独立 goroutine 中执行，不受阻塞影响
- 调用方 goroutine 通过 select 同时监听 ticker、handlerDone 和 ctx.Done()
- 每 10ms 检查一次 deadline
- 超时后立即返回错误，不等待 handler 完成
- 支持 panic 恢复，保证服务器稳定性
- 相比双层 goroutine 嵌套，减少 50% 的 goroutine 创建开销

### 5.4 元数据透传机制

元数据采用"上下文携带"模式：
- 请求元数据通过 `mdKey` 存储在 context 中
- 响应 header 通过 `headerKey` 存储在 context 中
- 响应 trailer 通过 `trailerKey` 存储在 context 中
- header 和 trailer 使用指针存储，支持在处理过程中动态设置
- 提供 `NewContextWithHeader` 和 `NewContextWithTrailer` 函数创建存储空间

### 5.5 并发流控制机制

并发流控制采用"原子计数 + CAS 操作"模式：
- `activeStreams` 使用 int32 原子变量存储活跃流数量
- `acquireStream()` 使用 CompareAndSwap 原子操作增加计数
- `releaseFn` 在流关闭时自动减少计数
- 达到 `MaxConcurrentStreams` 限制时返回 `ErrTooManyStreams`
- 使用 `sync/atomic` 包保证并发安全，性能优于互斥锁

实现细节见 [acquireStream()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L383-L393)

### 5.6 连接超时机制

连接超时采用"自动包装上下文"模式：
- 如果配置了 `ConnectionTimeout`（> 0），自动为请求创建带超时的上下文
- 使用 `context.WithTimeout` 包装原始上下文
- 超时时间与客户端设置的 deadline 取更严格的一个
- 一元调用和流式调用都应用此机制

### 5.7 并发安全设计

服务器完全并发安全：
- 所有共享状态（services、interceptors、running）受 `mu` 互斥锁保护
- 活跃流计数使用 `sync/atomic` 原子操作
- 服务内部状态通过各自的机制保证安全
- 后台 goroutine 通过 `running` 标志控制
- 使用 `sync.RWMutex` 提高读多写少场景的性能

### 5.8 Panic 恢复机制

所有 handler 执行都包含 panic 恢复：
- 使用 defer + recover 捕获 handler 中的 panic
- 将 panic 转换为错误返回
- 避免单个请求的 panic 导致整个服务器崩溃
- 一元和流式调用都应用此机制

## 6. 使用示例

### 6.1 基础使用：简单 Echo 服务

```go
package main

import (
    "context"
    "fmt"
    "log"

    "solocoder-go/internal/grpcsvc"
)

type echoServiceImpl struct{}

func main() {
    // 1. 创建服务器（带自定义配置）
    opts := grpcsvc.ServerOptions{
        MaxConcurrentStreams: 200,
        ConnectionTimeout:    60 * time.Second,
    }
    server := grpcsvc.NewServerWithOptions(opts)

    // 2. 定义服务描述符
    sd := &grpcsvc.ServiceDesc{
        ServiceName: "EchoService",
        Methods: []grpcsvc.MethodDesc{
            {
                MethodName: "Echo",
                Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
                    return req, nil
                },
            },
            {
                MethodName: "Hello",
                Handler: func(ctx context.Context, req interface{}) (interface{}, error) {
                    name := req.(string)
                    return fmt.Sprintf("Hello, %s!", name), nil
                },
            },
        },
    }

    // 3. 注册服务
    err := server.RegisterService(sd, &echoServiceImpl{})
    if err != nil {
        log.Fatalf("Failed to register service: %v", err)
    }

    // 4. 调用一元方法
    ctx := context.Background()
    resp, err := server.Invoke(ctx, "EchoService", "Hello", "World")
    if err != nil {
        log.Printf("Invoke failed: %v", err)
        return
    }
    fmt.Printf("Response: %v\n", resp) // Output: Response: Hello, World!

    // 5. 查询服务器状态
    fmt.Printf("Active streams: %d\n", server.ActiveStreams())
    fmt.Printf("Max concurrent streams: %d\n", server.Options().MaxConcurrentStreams)
}
```

### 6.2 使用拦截器

```go
package main

import (
    "context"
    "log"
    "time"

    "solocoder-go/internal/grpcsvc"
)

func loggingInterceptor(ctx context.Context, req interface{}, info *grpcsvc.UnaryServerInfo, handler grpcsvc.UnaryHandler) (interface{}, error) {
    start := time.Now()
    log.Printf("Request started: %s", info.FullMethod)

    resp, err := handler(ctx, req)

    log.Printf("Request completed: %s, duration: %v, error: %v",
        info.FullMethod, time.Since(start), err)
    return resp, err
}

func authInterceptor(ctx context.Context, req interface{}, info *grpcsvc.UnaryServerInfo, handler grpcsvc.UnaryHandler) (interface{}, error) {
    md, ok := grpcsvc.FromContext(ctx)
    if !ok {
        return nil, errors.New("missing metadata")
    }

    token := md.Get("authorization")
    if len(token) == 0 || token[0] != "Bearer valid-token" {
        return nil, errors.New("unauthorized")
    }

    return handler(ctx, req)
}

func main() {
    server := grpcsvc.NewServer()

    // 添加拦截器（按注册顺序执行）
    server.AddUnaryInterceptor(loggingInterceptor)
    server.AddUnaryInterceptor(authInterceptor)

    // 注册服务...
}
```

### 6.3 使用元数据

```go
func handler(ctx context.Context, req interface{}) (interface{}, error) {
    // 读取请求元数据
    md, ok := grpcsvc.FromContext(ctx)
    if ok {
        requestID := md.Get("x-request-id")
        log.Printf("Request ID: %v", requestID)
    }

    // 设置响应 header
    header := grpcsvc.NewMD()
    header.Set("x-server-version", "1.0.0")
    grpcsvc.SetHeader(ctx, header)

    // 设置响应 trailer
    trailer := grpcsvc.NewMD()
    trailer.Set("x-trace-id", "trace-123")
    grpcsvc.SetTrailer(ctx, trailer)

    return "response", nil
}

func clientCall() {
    // 客户端设置请求元数据
    md := grpcsvc.NewMD()
    md.Set("authorization", "Bearer token")
    ctx := grpcsvc.NewContextWithMD(context.Background(), md)

    // 准备接收 header 和 trailer
    ctx = grpcsvc.NewContextWithHeader(ctx)
    ctx = grpcsvc.NewContextWithTrailer(ctx)

    resp, err := server.Invoke(ctx, "Service", "Method", req)

    // 读取响应 header
    header, ok := grpcsvc.HeaderFromContext(ctx)
    if ok {
        version := header.Get("x-server-version")
        log.Printf("Server version: %v", version)
    }

    // 读取响应 trailer
    trailer, ok := grpcsvc.TrailerFromContext(ctx)
    if ok {
        traceID := trailer.Get("x-trace-id")
        log.Printf("Trace ID: %v", traceID)
    }
}
```

### 6.4 使用超时

```go
func main() {
    // 方式 1：客户端设置超时
    server := grpcsvc.NewServer()
    // 注册服务...

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := server.Invoke(ctx, "SlowService", "SlowMethod", req)
    if err != nil {
        if errors.Is(err, grpcsvc.ErrDeadlineExceeded) {
            log.Println("Request timed out")
        }
        return
    }

    // 方式 2：服务器配置连接超时
    opts := grpcsvc.ServerOptions{
        ConnectionTimeout: 10 * time.Second,
    }
    server2 := grpcsvc.NewServerWithOptions(opts)

    // 所有请求自动应用 10 秒超时
    resp, err = server2.Invoke(context.Background(), "Service", "Method", req)
}
```

### 6.5 并发流控制

```go
func main() {
    opts := grpcsvc.ServerOptions{
        MaxConcurrentStreams: 10,
    }
    server := grpcsvc.NewServerWithOptions(opts)
    // 注册服务...

    // 创建流
    streams := make([]grpcsvc.Stream, 0, 10)
    for i := 0; i < 10; i++ {
        stream, err := server.NewStream(context.Background(), "Service", "Stream")
        if err != nil {
            log.Fatalf("Failed to create stream: %v", err)
        }
        streams = append(streams, stream)
    }

    // 第 11 个流会失败
    _, err := server.NewStream(context.Background(), "Service", "Stream")
    if errors.Is(err, grpcsvc.ErrTooManyStreams) {
        log.Println("Too many concurrent streams")
    }

    // 关闭一个流后可以创建新的
    streams[0].Close()
    time.Sleep(10 * time.Millisecond)

    stream, err := server.NewStream(context.Background(), "Service", "Stream")
    if err != nil {
        log.Fatalf("Failed to create stream: %v", err)
    }
    defer stream.Close()

    log.Printf("Active streams: %d", server.ActiveStreams())
}
```

### 6.6 客户端流式 RPC

```go
// 服务端实现
func clientStreamHandler(srv interface{}, stream grpcsvc.Stream) error {
    count := 0
    for {
        msg, err := stream.Recv()
        if err != nil {
            if errors.Is(err, grpcsvc.ErrStreamClosed) {
                break
            }
            return err
        }
        log.Printf("Received: %v", msg)
        count++
        if count == 5 {
            break
        }
    }
    return stream.SendMsg(fmt.Sprintf("processed %d messages", count))
}

// 客户端使用（通过 Stream 接口）
func clientSend(stream grpcsvc.Stream) error {
    for i := 0; i < 5; i++ {
        // 通过 Stream 接口的 PutRecv 发送消息
        err := stream.PutRecv(fmt.Sprintf("msg-%d", i))
        if err != nil {
            return err
        }
    }

    // 读取服务端响应（通过 Stream 接口的 RecvFromServer）
    resp, err := stream.RecvFromServer()
    if err != nil {
        return err
    }
    log.Printf("Response: %v", resp)

    stream.Close()
    return nil
}
```

### 6.7 双向流式 RPC

```go
// 服务端实现
func bidiStreamHandler(srv interface{}, stream grpcsvc.Stream) error {
    for {
        msg, err := stream.Recv()
        if err != nil {
            if errors.Is(err, grpcsvc.ErrStreamClosed) {
                return nil
            }
            return err
        }
        log.Printf("Received: %v", msg)

        response := fmt.Sprintf("echo: %v", msg)
        if err := stream.SendMsg(response); err != nil {
            return err
        }
    }
}

// 客户端使用
func clientBidi(stream grpcsvc.Stream) error {
    for i := 0; i < 3; i++ {
        // 发送请求
        err := stream.PutRecv(fmt.Sprintf("ping-%d", i))
        if err != nil {
            return err
        }

        // 读取服务端响应（RecvFromServer 语义为"接收来自服务端的消息"）
        resp, err := stream.RecvFromServer()
        if err != nil {
            return err
        }
        log.Printf("Response: %v", resp)
    }

    stream.Close()
    return nil
}
```

### 6.8 流式拦截器

```go
func streamLoggingInterceptor(srv interface{}, ss grpcsvc.Stream, info *grpcsvc.StreamServerInfo, handler grpcsvc.StreamHandler) error {
    start := time.Now()
    log.Printf("Stream started: %s, type: server=%v client=%v",
        info.FullMethod, info.IsServerStream, info.IsClientStream)

    err := handler(srv, ss)

    log.Printf("Stream completed: %s, duration: %v, error: %v",
        info.FullMethod, time.Since(start), err)
    return err
}

func main() {
    server := grpcsvc.NewServer()
    server.AddStreamInterceptor(streamLoggingInterceptor)
    // ...
}
```

## 7. 文件结构

```
internal/grpcsvc/
├── grpcsvc.go       # gRPC 服务框架核心实现
└── grpcsvc_test.go  # 单元测试（覆盖正常流程、边界条件、异常分支）

docs/
└── grpcsvc.md       # 本文档
```

## 8. 测试覆盖范围

单元测试覆盖以下场景：

### 正常流程
- ✅ 服务器创建与配置
- ✅ 服务注册与查询
- ✅ 一元 RPC 调用
- ✅ 流式 RPC 调用（服务端流、客户端流、双向流）
- ✅ 一元拦截器链执行
- ✅ 流式拦截器链执行
- ✅ 元数据读取与设置（header 和 trailer）
- ✅ 超时正常请求
- ✅ Header 和 Trailer 设置与读取
- ✅ 并发调用
- ✅ 并发注册
- ✅ 并发流创建与释放
- ✅ Stream 接口完整性（PutRecv、Recv、Send 通过接口调用）
- ✅ 服务器配置选项读取
- ✅ 活跃流统计

### 边界条件
- ✅ 空服务名注册
- ✅ nil 服务描述符
- ✅ nil 服务实现
- ✅ 重复注册服务
- ✅ nil 拦截器
- ✅ 空拦截器链
- ✅ nil 元数据操作
- ✅ 服务器停止后操作
- ✅ 流关闭后操作
- ✅ 双重关闭流
- ✅ RPC 类型字符串转换
- ✅ 零值配置自动应用默认值
- ✅ 并发流达到上限
- ✅ 并发流释放后可重新创建

### 异常分支
- ✅ 调用不存在的服务
- ✅ 调用不存在的方法
- ✅ 请求超时
- ✅ 立即超时（deadline 已过）
- ✅ **长时间阻塞 handler 的持续超时检查**（一元调用）
- ✅ **长时间阻塞 handler 的持续超时检查**（流式调用）
- ✅ 连接超时配置生效（一元调用）
- ✅ 连接超时配置生效（流式调用）
- ✅ 流式调用超时
- ✅ 拦截器中断请求
- ✅ 元数据认证失败
- ✅ 已关闭流的发送/接收
- ✅ 服务器停止后的注册
- ✅ 服务器停止后的调用
- ✅ 无效方法描述符
- ✅ nil 处理器注册
- ✅ **并发流超过上限**
- ✅ **Stream 接口完整性验证（客户端流）**
- ✅ **Stream 接口完整性验证（双向流）**
- ✅ **RecvMsg 数据传递验证（字符串、整数、多条消息）**
- ✅ **RecvMsg 无效参数（nil、非指针）**
- ✅ **cancel 资源释放验证（流关闭后 context 立即取消）**
- ✅ **cancelFn 存在性验证（ConnectionTimeout > 0 时设置）**
- ✅ **RecvFromServer 命名正确性（服务端流、客户端流、双向流）**

## 9. 修复记录

### v2.0 修复内容（针对设计缺陷）

1. **修复持续超时校验**
   - 问题：checkDeadline 只在调用前检查一次，handler 阻塞时无法中断
   - 修复：在 Invoke 和 HandleStream 中添加独立的监控 goroutine，每 10ms 检查一次超时
   - 新增：panic 恢复机制，避免 handler panic 导致服务器崩溃
   - 相关测试：`TestDeadline_ContinuousCheck`, `TestDeadline_StreamContinuousCheck`

2. **修复 Stream 接口完整性**
   - 问题：PutRecv 方法仅在 *serverStream 上定义，未暴露在 Stream 接口中
   - 修复：在 Stream 接口中添加 PutRecv、Send、Recv、Header 方法
   - 新增 Send() 方法用于读取服务端发送的消息
   - 相关测试：`TestClientStream_InterfaceIntegrity`, `TestBidiStream_InterfaceIntegrity`, `TestStream_RecvInterface`

3. **移除虚假 API 表面积**
   - 问题：ServiceDesc 的 HandlerType 和 Metadata 字段定义后从未使用
   - 修复：从 ServiceDesc 中移除这两个字段
   - 影响：简化 API，减少混淆

4. **简化接口设计**
   - 问题：ServerStream 和 ClientStream 接口仅内嵌 Stream，无额外方法
   - 修复：移除这两个接口，统一使用 Stream 接口
   - 影响：简化 API，避免不必要的类型转换

5. **实现 ServerOptions 配置**
   - 问题：MaxConcurrentStreams 和 ConnectionTimeout 定义后从未使用
   - 修复：
     - MaxConcurrentStreams：新增 activeStreams 原子计数，acquireStream/releaseStream 机制
     - ConnectionTimeout：自动为请求创建带超时的上下文
   - 新增错误：ErrTooManyStreams
   - 新增方法：ActiveStreams(), Options()
   - 相关测试：`TestMaxConcurrentStreams`, `TestNewStream_TooManyConcurrent`, `TestConnectionTimeout`, `TestConnectionTimeout_Stream`

6. **修复 SetHeader 机制**
   - 问题：newContextWithStream 和 streamKey 定义后从未使用，SetHeader 为空操作
   - 修复：移除 streamKey 和 newContextWithStream，重新设计 header 机制
   - 新增：headerKey, NewContextWithHeader(), SetHeader(), HeaderFromContext()
   - 相关测试：`TestHeader`, `TestStreamHeader`

7. **新增测试用例**
   - 超时持续校验：一元调用、流式调用
   - ServerOptions 验证：默认值、自定义值、零值处理
   - 并发流控制：达到上限、释放后恢复
   - 连接超时：一元调用、流式调用
   - Stream 接口完整性：客户端流、双向流
   - Header 元数据：一元调用、流式调用

### v3.0 修复内容（针对代码缺陷）

1. **修复 RecvMsg 数据传递**
   - 问题：RecvMsg 从 recvCh 读取消息后用空白标识符丢弃，msg 参数从未填充
   - 修复：使用 reflect.ValueOf 将 recvCh 数据写入 msg 指针，msg 必须为非 nil 指针
   - 校验顺序：先检查流是否关闭，再校验 msg 参数（避免无效参数导致数据丢失）
   - 相关测试：`TestRecvMsg_DataPopulation`, `TestRecvMsg_MultipleDataPopulation`, `TestRecvMsg_NilPointer`, `TestRecvMsg_IntegerData`

2. **修复 cancel 资源泄漏**
   - 问题：NewStream 中 context.WithTimeout 的 cancel 函数被 `_ = cancel` 丢弃，流关闭后 context 内部 goroutine 泄漏
   - 修复：在 serverStream 中新增 cancelFn 字段，Close() 时调用 cancelFn 释放资源
   - 相关测试：`TestCancel_ReleaseOnStreamClose`, `TestCancel_NoLeakOnStreamClose`, `TestCancel_CancelFnSetWithConnectionTimeout`

3. **修复 Send/Recv 命名冲突**
   - 问题：Send() 方法实际从 sendCh 读取（接收服务端消息），与 SendMsg()（写入 sendCh）方向相反，命名极易混淆
   - 修复：将 Send() 重命名为 RecvFromServer()，语义为"接收来自服务端的消息"
   - 更新：Stream 接口、serverStream 实现、mockStream、所有测试用例和文档示例
   - 相关测试：`TestRecvFromServer_NamingCorrectness`, `TestRecvFromServer_BidiNaming`, `TestRecvFromServer_ClientStream`

4. **简化双层 goroutine 为单层+select**
   - 问题：Invoke 和 HandleStream 中每调用一次 RPC 启动两个 goroutine（外层监控+内层 handler），高并发下调度开销成倍
   - 修复：改为单 goroutine 执行 handler，调用方 goroutine 直接 select 监听 handlerDone/ticker.C/ctx.Done()
   - 消除：resultCh 中转 channel 和多余的 done channel
   - 影响：每 RPC 调用减少 1 个 goroutine，降低调度开销
