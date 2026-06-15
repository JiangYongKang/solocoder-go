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
| F8 | 超时传播 | 从请求上下文中读取客户端设置的超时时间，持续校验剩余超时时间 |
| F9 | 请求元数据透传 | 支持在请求头中携带自定义的键值对元数据，注入请求上下文 |
| F10 | 响应元数据回传 | 支持通过 trailer 元数据向客户端回传额外信息 |
| F11 | 服务查询 | 支持查询已注册的服务信息、服务数量和方法数量 |
| F12 | 服务启停 | 支持服务器的启动和停止，停止后拒绝新的请求 |

## 3. 核心结构体与职责

### 3.1 Server - gRPC 服务器

```go
type Server struct {
    mu              sync.RWMutex
    services        map[string]*service
    unaryInterceptors  []UnaryInterceptor
    streamInterceptors []StreamInterceptor
    running         bool
    options         ServerOptions
}
```

**主要职责：**
- 管理服务注册信息，维护服务名到服务实现的映射
- 管理拦截器链，支持一元和流式两种拦截器类型
- 提供 RPC 调用入口，分发请求到对应服务的处理方法
- 维护服务器运行状态，支持优雅停止
- 保证所有操作的并发安全

**核心方法：**
- `RegisterService(sd *ServiceDesc, srv interface{}) error` - 注册服务
- `AddUnaryInterceptor(interceptor UnaryInterceptor) error` - 添加一元拦截器
- `AddStreamInterceptor(interceptor StreamInterceptor) error` - 添加流式拦截器
- `Invoke(ctx context.Context, serviceName, methodName string, req interface{}) (interface{}, error)` - 调用一元方法
- `NewStream(ctx context.Context, serviceName, streamName string) (Stream, error)` - 创建流
- `HandleStream(ctx context.Context, serviceName, streamName string, stream Stream) error` - 处理流式调用
- `Stop()` - 停止服务器

实现细节见 [Server](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L186-L534)

### 3.2 ServiceDesc - 服务描述符

```go
type ServiceDesc struct {
    ServiceName string
    HandlerType interface{}
    Methods     []MethodDesc
    Streams     []StreamDesc
    Metadata    interface{}
}
```

**主要职责：**
- 描述一个 gRPC 服务的完整定义
- 包含服务名称、一元方法列表、流式方法列表
- 关联服务实现的类型信息

### 3.3 MethodDesc - 一元方法描述符

```go
type MethodDesc struct {
    MethodName  string
    Handler     UnaryHandler
}
```

**主要职责：**
- 描述一个一元 RPC 方法
- 包含方法名和处理函数

### 3.4 StreamDesc - 流式方法描述符

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

### 3.5 Stream - 流接口

```go
type Stream interface {
    Context() context.Context
    SendMsg(msg interface{}) error
    RecvMsg(msg interface{}) error
    SetHeader(md MD)
    SetTrailer(md MD)
    Close() error
    Closed() bool
}
```

**主要职责：**
- 抽象底层 gRPC 流，与具体实现解耦
- 提供消息发送、接收的统一接口
- 提供 header 和 trailer 设置接口
- 提供流关闭和状态查询接口

### 3.6 MD - 元数据类型

```go
type MD map[string][]string
```

**主要职责：**
- 表示 gRPC 元数据（HTTP/2 header 的抽象）
- 支持多值键
- 提供 Get、Set、Add、Delete、Copy 等操作方法

实现细节见 [MD](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L47-L83)

### 3.7 UnaryInterceptor - 一元拦截器

```go
type UnaryInterceptor func(ctx context.Context, req interface{}, info *UnaryServerInfo, handler UnaryHandler) (interface{}, error)
```

**主要职责：**
- 一元 RPC 拦截器函数类型
- 可在请求处理前后执行自定义逻辑（认证、日志、限流等）
- 通过调用 `handler` 继续执行后续处理链

### 3.8 StreamInterceptor - 流式拦截器

```go
type StreamInterceptor func(srv interface{}, ss ServerStream, info *StreamServerInfo, handler StreamHandler) error
```

**主要职责：**
- 流式 RPC 拦截器函数类型
- 可在流式调用处理前后执行自定义逻辑
- 通过调用 `handler` 继续执行后续处理链

### 3.9 UnaryServerInfo - 一元调用信息

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

### 3.10 StreamServerInfo - 流式调用信息

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

### 3.11 预定义错误

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
   ├─ 确保上下文有 trailer 存储空间
   │
   ├─ 构建 UnaryServerInfo
   │
   └─ 执行拦截器链
        │
        ├─ 拦截器 1 before 逻辑
        │     ├─ 拦截器 2 before 逻辑
        │     │     └─ ... → 实际 handler
        │     └─ 拦截器 2 after 逻辑
        └─ 拦截器 1 after 逻辑
```

实现细节见 [Invoke()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L349-L392)

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
   ├─ 构建 StreamServerInfo
   │
   └─ 执行拦截器链
        │
        ├─ 拦截器 1 before 逻辑
        │     ├─ 拦截器 2 before 逻辑
        │     │     └─ ... → 实际 stream handler
        │     └─ 拦截器 2 after 逻辑
        └─ 拦截器 1 after 逻辑
```

实现细节见 [HandleStream()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L432-L479)

### 4.4 拦截器链执行顺序

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

实现细节见 [UnaryInterceptorChain()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L308-L326)

### 4.5 超时传播流程

```
checkDeadline(ctx)
   │
   ├─ 从 ctx 获取 deadline
   │
   ├─ 没有 deadline → 返回 nil（不超时）
   │
   ├─ 比较当前时间与 deadline
   │     ├─ 已超过 → 返回 ErrDeadlineExceeded
   │     └─ 未超过 → 返回 nil
   │
   └─ 在以下节点检查超时
        ├─ 请求进入时（Invoke/HandleStream 入口）
        ├─ 实际处理器执行前
        ├─ 流发送/接收消息时
        └─ 流上下文取消时
```

实现细节见 [checkDeadline()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L690-L697)

### 4.6 元数据透传流程

**请求元数据（入站）：**
```
客户端 → HTTP/2 Headers → 框架解析为 MD → 注入 context → 拦截器/业务逻辑读取
```

**响应元数据（出站）：**
```
业务逻辑 → SetTrailer() → 写入 context → 框架读取 → HTTP/2 Trailers → 客户端
```

**使用方式：**
- 服务端：通过 `FromContext(ctx)` 读取请求元数据，通过 `SetTrailer(ctx, md)` 设置响应元数据
- 客户端：通过 `NewContextWithMD(ctx, md)` 附加请求元数据，通过 `TrailerFromContext(ctx)` 读取响应元数据

## 5. 核心机制说明

### 5.1 服务注册机制

服务采用"描述符 + 实现"分离的注册模式：
- `ServiceDesc` 描述服务的接口定义（方法名、处理函数等）
- `srv` 是服务的具体实现对象
- 框架通过反射/闭包的方式将调用分发到具体实现

### 5.2 拦截器链构建机制

拦截器链采用"洋葱模型"设计：
- 通过闭包嵌套的方式构建处理链
- 每个拦截器接收 `handler` 参数，即链中的下一个处理器
- 从最后一个拦截器开始向前构建，最外层是第一个注册的拦截器
- 支持零个或多个拦截器

实现细节见 [ChainUnaryInterceptors()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L699-L715)

### 5.3 超时传播机制

超时采用"上下文传递 + 多点校验"模式：
- 使用标准 `context.Context` 的 Deadline 机制
- 在请求处理的多个关键节点检查超时
- 超时后立即返回 `ErrDeadlineExceeded` 错误
- 流操作也会响应 context 的取消信号

### 5.4 元数据透传机制

元数据采用"上下文携带"模式：
- 请求元数据通过 `mdKey` 存储在 context 中
- 响应元数据（trailer）通过 `trailerKey` 存储在 context 中
- trailer 使用指针存储，支持在处理过程中动态设置
- 提供 `NewContextWithTrailer` 函数创建带 trailer 存储的上下文

### 5.5 并发安全设计

服务器完全并发安全：
- 所有共享状态（services、interceptors、running）受 `mu` 互斥锁保护
- 服务内部状态通过各自的机制保证安全
- 后台协程通过 `running` 标志控制
- 使用 `sync.RWMutex` 提高读多写少场景的性能

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
    // 1. 创建服务器
    server := grpcsvc.NewServer()

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

    // 设置响应元数据（trailer）
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

    // 准备接收 trailer
    ctx = grpcsvc.NewContextWithTrailer(ctx)

    resp, err := server.Invoke(ctx, "Service", "Method", req)

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
    server := grpcsvc.NewServer()
    // 注册服务...

    // 设置 5 秒超时
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    resp, err := server.Invoke(ctx, "SlowService", "SlowMethod", req)
    if err != nil {
        if errors.Is(err, grpcsvc.ErrDeadlineExceeded) {
            log.Println("Request timed out")
        }
        return
    }
    // 处理响应...
}
```

### 6.5 流式调用

```go
// 服务端流式 RPC
func serverStreamHandler(srv interface{}, stream grpcsvc.Stream) error {
    for i := 0; i < 10; i++ {
        msg := fmt.Sprintf("message-%d", i)
        if err := stream.SendMsg(msg); err != nil {
            return err
        }
    }
    return nil
}

// 客户端流式 RPC
func clientStreamHandler(srv interface{}, stream grpcsvc.Stream) error {
    count := 0
    for {
        err := stream.RecvMsg(nil)
        if err != nil {
            if errors.Is(err, grpcsvc.ErrStreamClosed) {
                break
            }
            return err
        }
        count++
    }
    return stream.SendMsg(fmt.Sprintf("received %d messages", count))
}

// 双向流式 RPC
func bidiStreamHandler(srv interface{}, stream grpcsvc.Stream) error {
    for {
        err := stream.RecvMsg(nil)
        if err != nil {
            if errors.Is(err, grpcsvc.ErrStreamClosed) {
                return nil
            }
            return err
        }
        if err := stream.SendMsg("echo"); err != nil {
            return err
        }
    }
}
```

### 6.6 流式拦截器

```go
func streamLoggingInterceptor(srv interface{}, ss grpcsvc.ServerStream, info *grpcsvc.StreamServerInfo, handler grpcsvc.StreamHandler) error {
    start := time.Now()
    log.Printf("Stream started: %s", info.FullMethod)

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
- ✅ 流式 RPC 调用
- ✅ 一元拦截器链执行
- ✅ 流式拦截器链执行
- ✅ 元数据读取与设置
- ✅ 超时正常请求
- ✅ Trailer 设置与读取
- ✅ 并发调用
- ✅ 并发注册

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

### 异常分支
- ✅ 调用不存在的服务
- ✅ 调用不存在的方法
- ✅ 请求超时
- ✅ 立即超时（deadline 已过）
- ✅ 流式调用超时
- ✅ 拦截器中断请求
- ✅ 元数据认证失败
- ✅ 已关闭流的发送/接收
- ✅ 服务器停止后的注册
- ✅ 服务器停止后的调用
- ✅ 无效方法描述符
- ✅ nil 处理器注册
