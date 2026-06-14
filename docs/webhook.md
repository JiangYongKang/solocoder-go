# Webhook 调度器模块需求文档

## 1. 模块概述

Webhook 调度器是一个基于内存的 HTTP 回调通知模块，提供可靠的 Webhook 事件推送能力。它支持回调注册、灵活的重试策略、HMAC-SHA256 签名校验、请求超时控制等特性，适用于需要将业务事件可靠地推送到第三方系统的场景。

### 主要特性

- **回调注册**：支持注册 Webhook 回调，配置目标 URL、HTTP 方法、自定义请求头和请求体模板，注册成功后返回唯一回调标识
- **重试策略**：支持为每个回调配置失败后的重试策略，包括最大重试次数、重试间隔和退避方式（固定间隔或指数退避）
- **签名校验**：支持为回调请求生成 HMAC-SHA256 签名并附加到请求头中，接收端可通过共享密钥校验请求的完整性和来源真实性
- **超时控制**：为每个回调请求设置超时时间，超时未收到响应自动取消该次请求并记录超时失败
- **并发调度**：通过可配置的 worker 协程池控制并发请求数量
- **结果通知**：支持同步等待回调执行结果，支持超时控制和异步查询
- **生命周期管理**：支持取消已注册的回调，优雅关闭等待正在执行的请求完成

## 2. 核心结构体

### 2.1 BackoffType

退避策略类型枚举：

| 类型 | 说明 |
|------|------|
| `BackoffFixed` | 固定间隔退避，每次重试间隔相同 |
| `BackoffExponential` | 指数退避，重试间隔按 2 的幂次递增 |

### 2.2 CallbackStatus

回调状态枚举：

| 状态 | 说明 |
|------|------|
| `CallbackStatusPending` | 待执行：回调已注册，尚未触发或正在重试等待中 |
| `CallbackStatusSucceeded` | 执行成功：回调请求已成功送达并收到 2xx 响应 |
| `CallbackStatusFailed` | 最终失败：重试次数耗尽，所有尝试均失败 |
| `CallbackStatusCancelled` | 已取消：回调被主动取消，不再执行 |

### 2.3 DeliveryStatus

单次投递状态枚举：

| 状态 | 说明 |
|------|------|
| `DeliveryStatusPending` | 待投递：请求尚未发送 |
| `DeliveryStatusSucceeded` | 投递成功：收到 2xx 响应 |
| `DeliveryStatusFailed` | 投递失败：收到非 2xx 响应或请求发送错误 |
| `DeliveryStatusTimeout` | 投递超时：请求在超时时间内未收到响应 |

### 2.4 RetryPolicy

```go
type RetryPolicy struct {
    MaxRetries  int
    Interval    time.Duration
    BackoffType BackoffType
}
```

**职责**：定义回调失败后的重试策略。

| 字段 | 说明 |
|------|------|
| MaxRetries | 最大重试次数，0 表示不重试，只发送一次 |
| Interval | 重试基础间隔时间，必须 > 0 |
| BackoffType | 退避类型：固定间隔或指数退避 |

**核心方法**：

- `Validate()`：验证重试策略参数合法性
- `BackoffDelay(attempt int)`：根据重试次数计算实际延迟时间

### 2.5 Callback

```go
type Callback struct {
    ID           string
    URL          string
    Method       string
    Headers      map[string]string
    BodyTemplate string
    RetryPolicy  RetryPolicy
    Timeout      time.Duration
    Secret       string
    Status       CallbackStatus
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

**职责**：表示一个 Webhook 回调配置，包含目标地址、HTTP 方法、重试策略等完整元数据。

| 字段 | 说明 |
|------|------|
| ID | 回调唯一标识，注册时自动生成 |
| URL | 回调目标 URL，必须是 http 或 https 协议 |
| Method | HTTP 请求方法，支持 GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS |
| Headers | 自定义请求头 |
| BodyTemplate | 请求体模板字符串 |
| RetryPolicy | 重试策略配置 |
| Timeout | 单次请求超时时间 |
| Secret | HMAC 签名共享密钥，为空则不签名 |
| Status | 当前回调状态 |
| CreatedAt | 创建时间 |
| UpdatedAt | 最后更新时间 |

### 2.6 Delivery

```go
type Delivery struct {
    ID           string
    CallbackID   string
    Attempt      int
    Status       DeliveryStatus
    StatusCode   int
    ResponseBody string
    Error        string
    StartedAt    time.Time
    FinishedAt   time.Time
    Duration     time.Duration
}
```

**职责**：表示一次具体的 HTTP 请求投递记录，完整记录每次尝试的详细信息。

| 字段 | 说明 |
|------|------|
| ID | 投递记录唯一标识 |
| CallbackID | 关联的回调 ID |
| Attempt | 尝试次数，从 1 开始 |
| Status | 本次投递状态 |
| StatusCode | HTTP 响应状态码（成功时） |
| ResponseBody | HTTP 响应体内容 |
| Error | 错误信息（失败时） |
| StartedAt | 请求开始时间 |
| FinishedAt | 请求结束时间 |
| Duration | 请求耗时 |

### 2.7 DeliveryResult

```go
type DeliveryResult struct {
    Delivery *Delivery
    Final    bool
}
```

**职责**：封装回调最终执行结果，用于对外返回。

| 字段 | 说明 |
|------|------|
| Delivery | 最后一次投递记录，成功或最终失败的那次 |
| Final | 是否为最终结果，true 表示不再重试 |

### 2.8 SchedulerConfig

```go
type SchedulerConfig struct {
    WorkerCount int
    HTTPClient  HTTPClient
}
```

**职责**：调度器配置参数。

| 字段 | 说明 |
|------|------|
| WorkerCount | worker 协程池大小，控制并发请求数，<= 0 时使用默认值 4 |
| HTTPClient | 自定义 HTTP 客户端，用于发送请求，nil 时使用默认 http.Client |

### 2.9 Scheduler

```go
type Scheduler struct {
    // ... 内部字段省略
}
```

**职责**：Webhook 调度器核心管理器，负责：

- 维护回调注册信息与投递记录
- 管理待投递队列，按计划时间排序
- 调度 worker 协程发送 HTTP 请求
- 处理失败重试与退避延迟计算
- 生成 HMAC 签名与超时控制
- 管理回调状态与结果存储
- 提供结果查询与通知机制

## 3. 回调完整生命周期

### 3.1 状态机流转

```
                         ┌──────────────┐
                         │  Register()  │
                         └──────┬───────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │ CallbackStatusPending│
                    │  (已注册，等待触发)   │
                    └──────────┬───────────┘
                               │
                               ▼
                         ┌──────────┐
                         │ Trigger()│
                         └─────┬────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  加入待投递队列       │
                    │  (scheduledAt=now)   │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  dispatchLoop 调度    │
                    │  获取 worker 槽位     │
                    └──────────┬───────────┘
                               │
                               ▼
                    ┌──────────────────────┐
                    │  发送 HTTP 请求       │
                    │  (带超时与签名)       │
                    └──────────┬───────────┘
                               │
                 ┌─────────────┴─────────────┐
                 │                           │
                 ▼                           ▼
       ┌──────────────────┐       ┌──────────────────┐
       │  收到 2xx 响应   │       │  请求失败/超时    │
       │  Status=200~299  │       │  Status!=2xx/err │
       └────────┬─────────┘       └────────┬─────────┘
                │                           │
                ▼                           ▼
    ┌──────────────────────┐       ┌──────────────────────┐
    │ DeliveryStatusSuccess│       │ attempt++            │
    │ CallbackStatusSuccess│       │                      │
    │ 标记 Final=true      │       │ attempt <= MaxRetries│─────┐
    └──────────────────────┘       └──────────┬───────────┘     │
                                               │                 │
                                               ▼                 │
                                    ┌──────────────────────┐     │
                                    │  计算退避延迟         │     │
                                    │  scheduledAt=now+delay│     │
                                    └──────────┬───────────┘     │
                                               │                 │
                                               ▼                 │
                                    ┌──────────────────────┐     │
                                    │  重新加入待投递队列    │◄────┘
                                    │  等待下一次调度       │
                                    └──────────┬───────────┘
                                               │
                                               ▼
                                    attempt > MaxRetries
                                               │
                                               ▼
                                    ┌──────────────────────┐
                                    │ CallbackStatusFailed │
                                    │ 标记 Final=true      │
                                    └──────────────────────┘

                         ┌──────────────┐
                         │  Cancel()    │  可在任意时刻调用
                         └──────┬───────┘
                                │
                                ▼
                    ┌──────────────────────┐
                    │ CallbackStatusCancelled│
                    │ 标记 Final=true      │
                    └──────────────────────┘
```

### 3.2 生命周期阶段说明

#### 阶段一：回调注册

1. 调用 `Register(url, method, opts...)` 注册回调
2. 验证 URL 合法性（必须为 http/https 协议）
3. 验证 HTTP 方法合法性（GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS）
4. 应用可选配置（Headers、BodyTemplate、RetryPolicy、Timeout、Secret）
5. 验证重试策略和超时参数合法性
6. 生成唯一回调 ID，状态设为 `CallbackStatusPending`
7. 返回回调 ID 用于后续管理

#### 阶段二：触发回调

1. 调用 `Trigger(callbackID)` 触发回调执行
2. 检查回调是否存在且未被取消
3. 创建待投递任务，`scheduledAt` 设为当前时间
4. 加入优先级队列（按 scheduledAt 升序排列）
5. 唤醒调度循环

#### 阶段三：调度执行

1. `dispatchLoop` 协程循环检查待投递队列
2. 取出队首任务，若 scheduledAt 未到则等待
3. 尝试获取 worker 协程池信号量
4. 获取成功后启动 worker 协程执行投递

#### 阶段四：HTTP 请求发送

1. 构建 HTTP 请求：方法、URL、Headers、Body
2. 若配置了 Secret，生成 HMAC-SHA256 签名：
   - 签名格式：`sha256=hex(hmac_sha256(secret, timestamp + "." + body))`
   - 签名头：`X-Webhook-Signature`
   - 时间戳头：`X-Webhook-Timestamp`（Unix 时间戳秒数）
3. 设置请求超时（context.WithTimeout）
4. 发送请求并等待响应

#### 阶段五：结果处理

**成功（2xx 响应）**：
- 投递状态设为 `DeliveryStatusSucceeded`
- 回调状态设为 `CallbackStatusSucceeded`
- 标记 `Final=true`，保存结果
- 通知等待者

**失败（非 2xx 响应或网络错误）**：
- 投递状态设为 `DeliveryStatusFailed`
- 超时请求状态设为 `DeliveryStatusTimeout`
- 检查重试次数：
  - 仍可重试：计算退避延迟，重新加入待投递队列
  - 重试耗尽：回调状态设为 `CallbackStatusFailed`，标记 `Final=true`

**被取消**：
- 回调状态设为 `CallbackStatusCancelled`
- 标记 `Final=true`，不再执行后续重试

### 3.3 重试策略计算

#### 固定间隔退避（`BackoffFixed`）

每次重试的延迟时间固定为 `Interval`，不受重试次数影响。

| 尝试次数 (attempt) | 延迟时间 |
|-------------------|----------|
| 1（首次失败后）   | Interval |
| 2                 | Interval |
| 3                 | Interval |
| ...               | ...      |

#### 指数退避（`BackoffExponential`）

第 n 次重试的延迟时间 = `Interval * 2^(n-1)`：

```
delay = Interval * (1 << (attempt - 1))
```

| 尝试次数 (attempt) | 延迟计算 (Interval=1s) |
|-------------------|-----------------------|
| 1（首次失败后）   | 1s * 2^0 = 1s         |
| 2                 | 1s * 2^1 = 2s         |
| 3                 | 1s * 2^2 = 4s         |
| 4                 | 1s * 2^3 = 8s         |
| 5                 | 1s * 2^4 = 16s        |

### 3.4 签名校验机制

签名生成算法：

```
payload = request_body
timestamp = unix_timestamp_seconds
message = timestamp + "." + payload
signature = "sha256=" + hex(hmac_sha256(secret, message))
```

请求头：
- `X-Webhook-Signature`: 上述签名
- `X-Webhook-Timestamp`: 时间戳

接收端校验步骤：
1. 从请求头获取签名和时间戳
2. 使用相同的密钥、时间戳和请求体重构签名
3. 使用 `hmac.Equal` 进行常量时间比较，防止时序攻击

## 4. 核心算法与策略

### 4.1 待投递队列排序

使用 `container/heap` 实现最小堆，按 `scheduledAt` 升序排列：

```go
func (h deliveryHeap) Less(i, j int) bool {
    return h[i].scheduledAt.Before(h[j].scheduledAt)
}
```

保证最早计划执行的任务优先被调度。

### 4.2 并发控制

使用带缓冲 channel 作为信号量控制并发请求数：

```go
sem := make(chan struct{}, WorkerCount)

// 获取槽位（支持阻塞等待与取消）
select {
case sem <- struct{}{}:
    // 获取成功
case <-stopCh:
    // 停止信号
}

// 释放槽位
<-sem
```

### 4.3 调度循环唤醒机制

调度循环在队列为空或等待下一次调度时间时阻塞，通过 `wakeCh` 唤醒：

- 新任务入队时唤醒
- 调度器停止时唤醒
- 使用非阻塞 close 模式，避免并发 close panic

### 4.4 超时检测机制

使用 `context.WithTimeout` 为每个请求设置独立超时：

```go
ctx, cancel := context.WithTimeout(context.Background(), cb.Timeout)
defer cancel()
req = req.WithContext(ctx)
```

超时错误检测：
- 检查 `ctx.Err() == context.DeadlineExceeded`
- 检查错误信息是否包含 "context deadline exceeded"

## 5. API 使用示例

### 5.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"

    "solocoder-go/internal/webhook"
)

func main() {
    // 1. 创建调度器
    s := webhook.NewScheduler(webhook.SchedulerConfig{
        WorkerCount: 4,
    })
    s.Start()
    defer s.Stop()

    // 2. 注册回调
    callbackID, err := s.Register(
        "https://api.example.com/webhook",
        "POST",
        webhook.WithHeaders(map[string]string{
            "X-Custom-Header": "value",
        }),
        webhook.WithBodyTemplate(`{"event":"order_created","data":{}}`),
        webhook.WithRetryPolicy(webhook.RetryPolicy{
            MaxRetries:  3,
            Interval:    1 * time.Second,
            BackoffType: webhook.BackoffExponential,
        }),
        webhook.WithTimeout(10 * time.Second),
        webhook.WithSecret("my-shared-secret-key"),
    )
    if err != nil {
        panic(err)
    }
    fmt.Printf("回调注册成功，ID: %s\n", callbackID)

    // 3. 触发回调
    if err := s.Trigger(callbackID); err != nil {
        panic(err)
    }

    // 4. 等待结果（带超时）
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    result, err := s.WaitForResult(ctx, callbackID)
    if err != nil {
        fmt.Printf("等待结果失败: %v\n", err)
        return
    }

    if result.Delivery.Status == webhook.DeliveryStatusSucceeded {
        fmt.Printf("回调成功! 状态码: %d\n", result.Delivery.StatusCode)
    } else {
        fmt.Printf("回调失败! 错误: %s\n", result.Delivery.Error)
        // 查看所有投递记录
        deliveries, _ := s.GetDeliveries(callbackID)
        fmt.Printf("共尝试 %d 次\n", len(deliveries))
    }
}
```

### 5.2 固定间隔重试策略

```go
policy := webhook.RetryPolicy{
    MaxRetries:  5,
    Interval:    500 * time.Millisecond,
    BackoffType: webhook.BackoffFixed,
}

id, _ := s.Register(
    "https://api.example.com/webhook",
    "POST",
    webhook.WithRetryPolicy(policy),
)
```

### 5.3 不重试策略

```go
// 只发送一次，失败即标记为最终失败
policy := webhook.RetryPolicy{
    MaxRetries:  0,
    Interval:    1 * time.Second, // 仍需合法值
    BackoffType: webhook.BackoffFixed,
}
```

### 5.4 取消回调

```go
// 触发后立即取消
s.Trigger(callbackID)
time.Sleep(10 * time.Millisecond)
s.Cancel(callbackID)

// 等待结果
result, _ := s.WaitForResult(ctx, callbackID)
if result.Final {
    fmt.Println("回调已取消")
}
```

### 5.5 接收端签名校验示例

```go
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    secret := "my-shared-secret-key"

    // 读取请求体
    body, _ := io.ReadAll(r.Body)

    // 获取签名和时间戳
    signature := r.Header.Get("X-Webhook-Signature")
    timestamp := r.Header.Get("X-Webhook-Timestamp")

    // 校验签名
    if !webhook.VerifyHMACSHA256(secret, body, timestamp, signature) {
        http.Error(w, "invalid signature", http.StatusUnauthorized)
        return
    }

    // 处理业务逻辑
    w.WriteHeader(http.StatusOK)
}
```

### 5.6 监控统计

```go
// 获取统计信息
callbackCount := s.CallbackCount()     // 已注册回调数
pendingCount := s.PendingCount()       // 待执行任务数
activeCount := s.ActiveCount()         // 正在执行的请求数

fmt.Printf("回调数: %d, 待执行: %d, 执行中: %d\n",
    callbackCount, pendingCount, activeCount)

// 查询回调信息
cb, _ := s.GetCallback(callbackID)
fmt.Printf("回调状态: %v, 创建时间: %v\n", cb.Status, cb.CreatedAt)

// 查询投递历史
deliveries, _ := s.GetDeliveries(callbackID)
for _, d := range deliveries {
    fmt.Printf("尝试 %d: 状态=%v, 耗时=%v, 错误=%s\n",
        d.Attempt, d.Status, d.Duration, d.Error)
}
```

### 5.7 并发触发多个回调

```go
const n = 100
ids := make([]string, n)

// 注册 100 个回调
for i := 0; i < n; i++ {
    id, _ := s.Register(
        fmt.Sprintf("https://api.example.com/webhook/%d", i),
        "POST",
    )
    ids[i] = id
}

// 并发触发
var wg sync.WaitGroup
wg.Add(n)
for _, id := range ids {
    go func(cbID string) {
        defer wg.Done()
        s.Trigger(cbID)
    }(id)
}
wg.Wait()
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrCallbackNotFound` | 操作不存在的回调 ID |
| `ErrCallbackCancelled` | 触发已取消的回调（区分于不存在的回调） |
| `ErrCallbackAlreadyExists` | 使用指定 ID 注册时 ID 已存在 |
| `ErrSchedulerStopped` | 调度器未启动或已停止时调用注册/触发 |
| `ErrInvalidURL` | 回调 URL 不合法（非 http/https 或缺少 host） |
| `ErrInvalidMethod` | HTTP 方法不支持 |
| `ErrInvalidBackoffType` | 退避类型不合法 |
| `ErrInvalidMaxRetries` | 最大重试次数为负数 |
| `ErrInvalidInterval` | 重试间隔 <= 0 |
| `ErrInvalidTimeout` | 请求超时 <= 0 |
| `ErrDeliveryNotFound` | 查询结果时结果尚未产生 |

上下文取消/超时：
- `WaitForResult(ctx, callbackID)` 在 ctx 被取消或超时时返回 `ctx.Err()`

### 6.1 错误语义区分

`ErrCallbackNotFound` 与 `ErrCallbackCancelled` 是两个不同的错误，调用方应正确区分：

```go
err := s.Trigger(callbackID)
if err == ErrCallbackNotFound {
    // 回调 ID 不存在，可能是输入错误或已被清理
    log.Printf("回调 %s 不存在", callbackID)
} else if err == ErrCallbackCancelled {
    // 回调存在但已被取消，不应再触发
    log.Printf("回调 %s 已被取消，跳过触发", callbackID)
} else if err != nil {
    // 其他错误
    log.Printf("触发回调失败: %v", err)
}
```

### 6.2 失败结果的错误汇总

当重试耗尽最终失败时，`DeliveryResult.Error` 字段会包含汇总的错误信息，包含重试耗尽的上下文：

```go
result, _ := s.WaitForResult(ctx, callbackID)
if result.Final && result.Error != nil {
    // result.Error 包含汇总信息，如: "max retries exhausted: HTTP 500"
    // result.Delivery.Error 包含最后一次具体错误，如: "HTTP 500"
    log.Printf("最终失败: %v, 最后一次错误: %s",
        result.Error, result.Delivery.Error)
}
```

| 场景 | `result.Error` | `result.Delivery.Error` |
|------|---------------|------------------------|
| 重试耗尽最终失败 | `"max retries exhausted: HTTP 500"` | `"HTTP 500"` |
| 单次请求成功 | `nil` | `""` |
| 单次请求失败（MaxRetries=0） | `"max retries exhausted: send request: dial tcp"` | `"send request: dial tcp"` |
| 回调被取消 | `nil` | `""` |

## 7. 线程安全说明

Webhook Scheduler 所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：

- 调度器内部通过 `sync.Mutex` 保护共享数据结构（回调 map、投递记录 map、待投递堆）
- `dispatchLoop` 单协程调度，避免堆操作竞争条件
- 活跃计数使用 `sync/atomic` 原子操作
- 通知通道使用独立的 `sync.Mutex` 保护
- `Start()`、`Stop()` 均为幂等操作，可安全多次调用

## 8. 资源与生命周期

- **创建**：`NewScheduler(cfg)` 创建实例，应用默认配置
- **启动**：`Start()` 启动调度循环，必须在注册/触发前调用
- **运行**：注册回调、触发执行、查询状态、等待结果
- **停止**：`Stop()` 优雅关闭：
  - 停止接收新的注册和触发请求
  - 等待正在执行的请求完成
  - 待投递队列中的任务不再执行
  - 超过 `shutdownTimeout`（默认 30s）时强制返回
- **幂等性**：`Start()`、`Stop()` 均为幂等操作

## 9. 可观测性

调度器提供以下方法用于监控：

| 方法 | 说明 |
|------|------|
| `CallbackCount()` | 返回已注册的回调总数 |
| `PendingCount()` | 返回待执行任务数（队列中 + 执行中） |
| `ActiveCount()` | 返回正在执行的请求数 |
| `GetCallback(id)` | 查询单个回调的详细信息 |
| `GetDeliveries(id)` | 查询单个回调的所有投递记录 |
| `GetResult(id)` | 查询回调的最终结果（如果已完成） |

通过这些方法可以构建监控面板，追踪回调执行成功率、平均延迟、重试次数等关键指标。
