# HTTP 网关 (Gateway) 模块需求文档

## 1. 模块概述

HTTP 网关是一个位于客户端与上游服务之间的中间层组件，负责统一处理请求的路由转发、身份认证、流量控制、日志审计、熔断降级和健康检查等横切关注点。通过将这些通用能力下沉到网关层，上游服务可以专注于业务逻辑的实现。

本模块使用内存数据结构模拟上游服务（`UpstreamHandler` 接口），所有核心功能（路由、鉴权、限流、熔断、健康检查）均为纯内存实现，不依赖外部中间件，便于单元测试和本地开发调试。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 路由转发 - 精确匹配 | 路径与路由表完全一致时转发到指定上游 |
| F2 | 路由转发 - 通配符匹配 | 支持前缀通配符（`/*`），最长前缀优先 |
| F3 | 路由转发 - 404 返回 | 未匹配任何路由规则时返回 HTTP 404 |
| F4 | 鉴权中间件 - Bearer Token | 校验 `Authorization: Bearer <token>` 头，无效返回 401 |
| F5 | 鉴权中间件 - 上下文注入 | 鉴权通过后将用户信息注入 request context |
| F6 | 鉴权中间件 - 路径豁免 | 支持配置豁免路径（如 `/health`）不做鉴权 |
| F7 | 限流中间件 - 令牌桶 | 按请求来源 IP 做令牌桶限流，超阈值返回 429 |
| F8 | 限流中间件 - IP 提取 | 优先级：X-Forwarded-For > X-Real-IP > RemoteAddr |
| F9 | 请求日志中间件 | 记录 method、path、status、duration，输出到标准输出 |
| F10 | 熔断器 - 滑动窗口 | 维护上游失败计数，窗口内连续失败超阈值跳闸 |
| F11 | 熔断器 - 状态流转 | 关闭 → 打开 → 半开 → 关闭，支持降级响应 |
| F12 | 熔断器 - 半开探测 | 打开一段时间后允许少量探测请求验证恢复 |
| F13 | 健康检查 - 定时探测 | 后台协程周期调用上游 `HealthCheck()` |
| F14 | 健康检查 - 自动摘除 | 连续失败超阈值后从路由表中摘除 |
| F15 | 健康检查 - 自动恢复 | 连续成功超阈值后重新加入路由表 |
| F16 | 并发安全 | 所有共享状态使用互斥锁保护，支持高并发访问 |

## 3. 核心结构体与职责

### 3.1 Gateway - 网关主体

```go
type Gateway struct {
    router      *Router                  // 路由匹配器
    upstreams   map[string]UpstreamHandler // 已注册的上游服务
    middlewares []Middleware             // 中间件链（按执行顺序）
    auth        *AuthMiddleware          // 鉴权中间件（可空）
    limiter     *RateLimiter             // 限流中间件（可空）
    circuits    map[string]*CircuitBreaker // 熔断器集合
    health      *HealthChecker           // 健康检查器
    fallback    map[string]HandlerFunc   // 熔断降级响应函数
    logger      *LoggerMiddleware        // 日志中间件（可空）
    server      *http.Server             // 启动的 HTTP 服务器
}
```

**主要职责：**
- 持有并协调各子模块（路由、鉴权、限流、熔断、健康检查）
- 构建中间件执行链（日志 → 限流 → 鉴权 → 路由转发）
- 提供上游注册、路由配置、熔断器注册、降级函数注册等 API
- 实现 `http.Handler` 接口，可直接挂到标准库 `http.Server`

### 3.2 Router - 路由匹配器

```go
type Router struct {
    routes    map[string]string   // 精确匹配路径 → 上游名
    wildcards []wildcardRoute     // 通配符路由列表（按前缀长度匹配）
    mu        sync.RWMutex
    health    *HealthChecker      // 关联健康检查，跳过不健康上游
}

type wildcardRoute struct {
    Prefix   string // 去掉末尾 * 的前缀
    Upstream string
}
```

**主要职责：**
- 管理精确匹配路由与通配符路由
- 通配符匹配采用"最长前缀优先"策略
- 匹配时查询健康检查状态，自动跳过不健康上游
- 生成最终的请求转发 Handler（含熔断器逻辑）

### 3.3 AuthMiddleware - 鉴权中间件

```go
type AuthMiddleware struct {
    store       TokenStore        // Token 校验器
    exemptPaths map[string]bool   // 豁免鉴权的路径集合
}
```

**主要职责：**
- 解析 `Authorization` 请求头，校验 Bearer Token 格式
- 委托 `TokenStore` 完成 Token → UserInfo 的映射
- 鉴权失败直接返回 401，成功则将用户信息注入 Context
- 支持路径级豁免，豁免路径直接放行

### 3.4 RateLimiter - 限流中间件

```go
type RateLimiter struct {
    tokens     map[string]*tokenBucket // IP → 令牌桶
    rate       int                     // 每次补充的令牌数
    capacity   int                     // 桶容量（突发上限）
    mu         sync.Mutex
    refillRate time.Duration           // 补充间隔
}
```

**主要职责：**
- 按来源 IP 维护独立的令牌桶
- 令牌桶采用"懒惰补充"策略，取用时计算流逝时间并补充
- 超限请求返回 429 并附带 `Retry-After: 1` 头
- 支持手动 `Reset(ip)` 清空桶，便于测试

### 3.5 CircuitBreaker - 熔断器

```go
type CircuitBreaker struct {
    name                string
    state               CircuitBreakerState // Closed/Open/HalfOpen
    failures            []FailureEntry       // 滑动窗口的请求记录
    windowSize          time.Duration        // 滑动窗口大小
    failureThreshold    int                  // 失败阈值（达到则跳闸）
    openDuration        time.Duration        // 打开状态持续时间
    halfOpenMaxRequests int                  // 半开状态允许的探测请求数
    halfOpenRequests    int
    halfOpenSuccesses   int
    halfOpenFailures    int
    lastOpenTime        time.Time
    mu                  sync.Mutex
}
```

**主要职责：**
- 维护三态状态机，线程安全地管理状态流转
- Closed 状态：记录失败，达到阈值 → 跳闸到 Open
- Open 状态：直接拒绝所有请求，触发降级响应
- HalfOpen 状态：有限放行探测请求，全成功 → 关闭，任一失败 → 重新打开
- Closed 状态下成功请求清空失败窗口（服务正常就重置计数）

**熔断器记录机制说明：**

`FailureEntry` 只记录失败请求的时间戳，不记录成功请求。这样设计的原因是：

1. **简化计数逻辑**：滑动窗口只需统计失败数量，无需过滤成功记录
2. **内存效率高**：仅保留失败条目，成功时直接清空切片（O(1)）
3. **语义清晰**：`failures` 切片名符其实，只存放失败事件

```go
type FailureEntry struct {
    Time time.Time // 失败请求发生的时间
}
```

状态流转中的计数行为：
- **Closed → RecordFailure**：追加到 `failures` 切片，计算窗口内失败数 ≥ 阈值则跳闸
- **Closed → RecordSuccess**：直接清空 `failures` 切片（一次成功即重置失败计数）
- **HalfOpen → RecordFailure**：立即跳闸回 Open，无需看窗口
- **HalfOpen → RecordSuccess**：半开成功计数 +1，达到 `halfOpenMaxRequests` 则转 Closed

### 3.6 HealthChecker - 健康检查器

```go
type HealthChecker struct {
    upstreams     map[string]*upstreamHealth
    checkInterval time.Duration  // 探测间隔
    failThreshold int            // 连续失败次数阈值（摘除）
    passThreshold int            // 连续成功次数阈值（恢复）
    stopCh        chan struct{}  // 停止信号
    running       bool
    mu            sync.RWMutex
}

type upstreamHealth struct {
    handler   UpstreamHandler
    healthy   bool
    lastCheck time.Time
    failCount int  // 连续失败计数
    passCount int  // 连续成功计数
}
```

**主要职责：**
- 后台协程按 `checkInterval` 轮询所有已注册上游
- 调用每个上游的 `HealthCheck()` 方法判断健康状态
- 连续失败达到 `failThreshold` 标记为不健康（路由匹配自动跳过）
- 连续成功达到 `passThreshold` 恢复为健康（重新加入路由）
- 提供手动 `SetHealthy(name, bool)` API 便于测试和人工干预

### 3.7 UpstreamHandler 接口

```go
type UpstreamHandler interface {
    ServeHTTP(http.ResponseWriter, *http.Request) // 处理请求
    Name() string                                 // 返回上游名称
    HealthCheck() bool                            // 健康探测
}
```

上游服务必须实现此接口。模块内提供了 `MockUpstreamHandler` 用于测试，支持：
- 自定义响应状态码和响应体（**原子更新**，保证状态码与 body 一致）
- 切换健康/不健康状态
- 注入人为延迟（模拟慢请求）
- 设置自定义 Handler
- 统计请求调用次数
- 所有可变字段均由统一的 `sync.RWMutex` 保护，并发安全

### 3.8 类型别名与辅助类型

```go
type HandlerFunc func(http.ResponseWriter, *http.Request)
type Middleware func(HandlerFunc) HandlerFunc

type RouteType int   // ExactMatch / WildcardMatch
type Route struct { Path string; Type RouteType; Upstream string }

type UserInfo struct { UserID string; Roles []string }
type TokenStore interface { Validate(token string) (*UserInfo, bool) }

type CircuitBreakerState int // StateClosed / StateOpen / StateHalfOpen
```

## 4. 熔断器状态流转图

熔断器采用经典的三态模型：

```
                        ┌───────────────────────────────────────┐
                        │                                       │
                        │         ┌─────────────────────┐       │
                        ▼         │   成功 >= 探测数    │       │
                   ┌──────────┐   │                     │       │
          ┌───────▶│  Closed  │───┘                     │       │
          │        └────┬─────┘                         │       │
          │             │                               │       │
          │             │ 窗口内失败数 >= 阈值          │       │
          │             ▼                               │       │
          │        ┌──────────┐                         │       │
          │        │   Open   │                         │       │
          │        └────┬─────┘                         │       │
          │             │                               │       │
          │             │ 经过 openDuration 时间        │       │
          │             ▼                               │       │
          │        ┌──────────────┐                     │       │
          │        │  Half Open   │◀────────────────────┘       │
          │        └──────┬───────┘                             │
          │               │                                     │
          │               │ 任一探测请求失败                    │
          └───────────────┘                                     │
                        │                                       │
                        │ 所有探测请求都失败则回到 Open         │
                        └───────────────────────────────────────┘
```

**各状态行为说明：**

| 状态 | Allow() 结果 | 记录请求 | 触发条件 |
|------|-------------|---------|---------|
| **Closed（关闭）** | `(true, true)`：全部放行，全部记录 | 成功 → 清空失败窗口；失败 → 追加到窗口并计数 | 初始状态；半开状态探测全部成功 |
| **Open（打开）** | `(false, false)`：全部拒绝，不记录 | 无记录 | 关闭状态连续失败达阈值；半开状态任一探测失败 |
| **HalfOpen（半开）** | 最多放行 `halfOpenMaxRequests` 个，超出则拒绝 | 全成功 → 转关闭；任一失败 → 转打开 | 打开状态持续 `openDuration` 后自动进入 |

**路由转发中的熔断器逻辑：**
1. 请求匹配到上游后，优先查询熔断器 `Allow()`
2. 若被拒绝：优先使用自定义降级函数（`Fallback`），否则返回默认 `503 Circuit Breaker Open`
3. 若放行：包装 ResponseWriter 捕获状态码，5xx 记为失败，其余记为成功

## 5. 请求处理流程（全链路）

```
客户端请求
    │
    ▼
[1] 日志中间件 LoggerMiddleware
    │   记录开始时间
    │   包装 ResponseWriter 捕获状态码
    │   （在下游返回后输出日志）
    │
    ▼
[2] 限流中间件 RateLimiter
    │   提取请求来源 IP
    │   查询对应令牌桶是否有令牌
    │   无令牌 → 429 Too Many Requests
    │
    ▼
[3] 鉴权中间件 AuthMiddleware
    │   检查路径是否豁免
    │   解析 Authorization: Bearer <token>
    │   校验 Token 有效性
    │   无效/缺失 → 401 Unauthorized
    │   有效 → UserInfo 注入 request Context
    │
    ▼
[4] 路由转发 Router.Handler
    │   精确匹配 → 通配符匹配（最长前缀）
    │   未匹配 → 404 Not Found
    │   匹配时跳过不健康上游
    │
    ▼
[5] 熔断器 CircuitBreaker
    │   Allow() = false → 执行降级函数 / 默认 503
    │   Allow() = true  → 调用上游
    │   捕获响应状态：
    │       5xx → RecordFailure()
    │       其他 → RecordSuccess()
    │
    ▼
[6] 上游服务 UpstreamHandler.ServeHTTP
    │   实际业务处理（模拟）
    │
    ▼
响应返回（逐层回溯，日志层最后输出）
```

**中间件执行顺序说明：**
- 日志最外层：保证能记录到所有响应（包括 429、401、404、503）
- 限流失效鉴权之前：避免无效鉴权计算
- 鉴权在路由之前：避免未授权请求做路由匹配
- 熔断器在路由层内部实现：与具体上游绑定

## 6. 配置参数说明

### 6.1 GatewayConfig - 网关配置

```go
type GatewayConfig struct {
    TokenStore      TokenStore                    // Token 存储（鉴权用）
    Rate            int                           // 令牌桶每次补充数量
    RateCapacity    int                           // 令牌桶容量（突发上限）
    RefillRate      time.Duration                 // 令牌桶补充间隔
    CheckInterval   time.Duration                 // 健康检查探测间隔
    FailThreshold   int                           // 健康检查连续失败阈值
    PassThreshold   int                           // 健康检查连续成功阈值
    CircuitConfigs  map[string]CircuitBreakerConfig // 各上游熔断器配置
    AuthExemptPaths []string                      // 鉴权豁免路径列表
    EnableAuth      bool                          // 是否启用鉴权
    EnableRateLimit bool                          // 是否启用限流
    EnableLogger    bool                          // 是否启用日志
}
```

### 6.2 CircuitBreakerConfig - 熔断器配置

```go
type CircuitBreakerConfig struct {
    Name                string        // 上游名称（必须与注册名一致）
    WindowSize          time.Duration // 滑动窗口大小
    FailureThreshold    int           // 窗口内失败数阈值（跳闸）
    OpenDuration        time.Duration // 打开状态持续时间
    HalfOpenMaxRequests int           // 半开状态探测请求数
    Fallback            HandlerFunc   // 自定义降级响应（可选）
}
```

## 7. 使用示例

### 7.1 基础示例：搭建最小网关

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "solocoder-go/internal/gateway"
)

func main() {
    // 1. 准备 Token 存储
    tokenStore := gateway.NewMemoryTokenStore()
    tokenStore.Add("user-token-123", &gateway.UserInfo{
        UserID: "u_001",
        Roles:  []string{"user", "editor"},
    })

    // 2. 创建网关配置
    config := gateway.GatewayConfig{
        TokenStore:      tokenStore,
        Rate:            2,
        RateCapacity:    10,
        RefillRate:      100 * time.Millisecond,
        CheckInterval:   5 * time.Second,
        FailThreshold:   3,
        PassThreshold:   2,
        EnableAuth:      true,
        EnableRateLimit: true,
        EnableLogger:    true,
        AuthExemptPaths: []string{"/health", "/metrics"},
        CircuitConfigs: map[string]gateway.CircuitBreakerConfig{
            "order-service": {
                Name:                "order-service",
                WindowSize:          1 * time.Minute,
                FailureThreshold:    5,
                OpenDuration:        30 * time.Second,
                HalfOpenMaxRequests: 2,
                Fallback: func(w http.ResponseWriter, r *http.Request) {
                    w.WriteHeader(http.StatusServiceUnavailable)
                    w.Write([]byte(`{"error":"订单服务暂不可用，请稍后重试"}`))
                },
            },
        },
    }

    // 3. 构造网关
    gw := gateway.NewGateway(config)
    defer gw.StopHealthCheck()

    // 4. 注册上游服务
    userSvc := gateway.NewMockUpstreamHandler("user-service")
    userSvc.SetResponse(http.StatusOK, `{"id":1,"name":"Alice"}`)

    orderSvc := gateway.NewMockUpstreamHandler("order-service")
    orderSvc.SetResponse(http.StatusOK, `[{"order_id":"A001"}]`)

    gw.RegisterUpstream("user-service", userSvc)
    gw.RegisterUpstream("order-service", orderSvc)

    // 5. 添加路由规则
    gw.AddRoute("/health", gateway.ExactMatch, "user-service")
    gw.AddRoute("/api/users/me", gateway.ExactMatch, "user-service")
    gw.AddRoute("/api/orders/*", gateway.WildcardMatch, "order-service")

    // 6. 启动服务
    log.Println("Gateway starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", gw.Handler()))
}
```

### 7.2 测试风格示例：模拟请求

```go
func TestExample(t *testing.T) {
    tokenStore := gateway.NewMemoryTokenStore()
    tokenStore.Add("abc", &gateway.UserInfo{UserID: "test"})

    gw := gateway.NewGateway(gateway.GatewayConfig{
        TokenStore:      tokenStore,
        EnableAuth:      true,
        EnableRateLimit: false,
        EnableLogger:    false,
    })

    svc := gateway.NewMockUpstreamHandler("svc")
    gw.RegisterUpstream("svc", svc)
    gw.AddRoute("/api/data", gateway.ExactMatch, "svc")

    req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
    req.Header.Set("Authorization", "Bearer abc")
    w := httptest.NewRecorder()
    gw.ServeHTTP(w, req)

    fmt.Println("Status:", w.Code)
    fmt.Println("Body:", w.Body.String())
}
```

### 7.3 自定义上游服务（实现 UpstreamHandler）

```go
type RealUserService struct {
    db *sql.DB
}

func (s *RealUserService) Name() string   { return "user-service" }

func (s *RealUserService) HealthCheck() bool {
    return s.db.Ping() == nil
}

func (s *RealUserService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 实际业务逻辑
    rows, err := s.db.Query("SELECT id, name FROM users")
    // ...
}
```

### 7.4 自定义 TokenStore（集成 Redis/JWT）

```go
type JWTTokenStore struct {
    verifier *jwt.Verifier
}

func (j *JWTTokenStore) Validate(token string) (*gateway.UserInfo, bool) {
    claims, err := j.verifier.Verify(token)
    if err != nil {
        return nil, false
    }
    return &gateway.UserInfo{
        UserID: claims.Subject,
        Roles:  claims.GetStringSlice("roles"),
    }, true
}
```

## 8. 线程安全说明

模块内所有共享可变状态均受并发保护：

| 结构体 | 保护机制 | 说明 |
|--------|---------|------|
| `Router.routes / wildcards` | `sync.RWMutex` | 读多写少，允许多 goroutine 并发匹配 |
| `RateLimiter.tokens` + 每个 `tokenBucket` | 两层 `sync.Mutex` | 外层按 IP 查找桶，内层操作令牌计数 |
| `CircuitBreaker.*` | `sync.Mutex` | 状态机变更需原子性，所有操作加锁 |
| `HealthChecker.upstreams` | `sync.RWMutex` | 探测轮询与路由查询并发进行 |
| `MemoryTokenStore.tokens` | `sync.RWMutex` | 读多写少场景 |
| `MockUpstreamHandler.*` | 单一 `sync.RWMutex` | 所有可变字段（healthy/statusCode/body/count/latency/customHandler）由同一把锁保护，保证响应状态码与 body 读取的一致性，避免并发下出现"旧状态码+新响应体"的撕裂 |

中间件本身是无状态的（只读配置），可以被任意并发调用。

**MockUpstreamHandler 并发保证策略：**

修复前：`statusCode` 与 `responseBody` 为裸字段，`SetResponse` 写入时不加锁，`ServeHTTP` 读取时也不加锁。并发场景下可能出现"调用方刚改完 statusCode 还没改 body，请求就读到了新状态码 + 旧响应体"的数据撕裂问题。

修复后：所有可变字段由**单一 `sync.RWMutex`** 统一保护：
- **写操作**（`SetResponse` / `SetHealthy` / `SetLatency` / `SetCustomHandler` / `ResetCount`）：`Lock()` 独占访问
- **读操作**（`HealthCheck` / `RequestCount`）：`RLock()` 共享访问
- **读-拷贝-使用**（`ServeHTTP`）：`Lock()` 一次性将所有需要的字段（statusCode/body/handler/latency）读到局部变量，再释放锁，然后在锁外执行实际的响应写入和 sleep。这样既保证了读取到的状态码和 body 是同一时刻的一致快照，又不会在 sleep 期间长期持锁阻塞其他写操作

## 9. 文件结构

```
internal/gateway/
├── types.go         # 核心类型定义（HandlerFunc、Middleware、各结构体字段）
├── gateway.go       # Gateway 主体 + HealthChecker 方法 + MockUpstreamHandler
├── router.go        # 路由匹配（精确/通配符）+ 熔断器转发逻辑
├── auth.go          # 鉴权中间件 + TokenStore 接口 + 内存 TokenStore
├── ratelimit.go     # 限流中间件 + 令牌桶实现 + IP 提取工具
├── circuit.go       # 熔断器状态机 + 配置结构体
├── logger.go        # 日志中间件 + 日志收集器（测试用）
└── gateway_test.go  # 完整单元测试（40+ 测试用例，含并发测试）

docs/
└── gateway.md       # 本文档
```
