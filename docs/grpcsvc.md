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
    Send() (interface{}, error)
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
- `SendMsg(msg)` - 向客户端发送消息（写入 sendCh）
- `RecvMsg(msg)` - 从客户端接收消息（从 recvCh 读取，仅返回错误）
- `Recv()` - 从客户端接收消息并返回（从 recvCh 读取，返回消息内容）
- `Send()` - 读取服务端发送的消息（从 sendCh 读取，供测试和框架内部使用）
- `PutRecv(msg)` - 向 recvCh 写入消息（模拟客户端发送，供测试和框架内部使用）
- `SetHeader(md)` - 设置响应 header
- `SetTrailer(md)` - 设置响应 trailer
- `Header()` - 获取已设置的 header
- `Close()` - 关闭流
- `Closed()` - 查询流是否已关闭

实现细节见 [Stream](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L151-L163)

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
   ├─ 查找方法 → 不存在