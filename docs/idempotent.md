# 请求幂等性中间件 (Idempotent) 模块需求文档

## 1. 模块概述

请求幂等性中间件是一个基于内存的 HTTP 接口幂等性保障组件，用于在分布式系统中确保相同请求的重复提交不会产生副作用。通过维护"幂等键 → 响应结果"的映射关系，客户端在每个需要幂等性保证的请求中携带唯一的幂等键，中间件在处理请求前检查该键是否已存在，已存在的请求直接返回首次处理的结果而不重复执行业务逻辑。

本模块适用于支付下单、订单创建、资源提交等需要保证"恰好一次"语义的 HTTP 接口场景，通过可配置的过期时间和清理间隔在幂等性保证与内存占用之间取得平衡。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 幂等键请求去重 (Execute) | 检查幂等键是否已处理，未处理则执行业务逻辑并缓存结果，已处理则直接返回缓存结果 |
| F2 | 幂等结果缓存 | 将首次请求的处理结果（响应状态码 + 响应体）与幂等键关联缓存 |
| F3 | 并发请求锁等待 | 相同幂等键的并发请求中，第一个请求获取锁并执行业务逻辑，其余请求等待第一个完成后共享结果 |
| F4 | 过期键自动回收 | 每个幂等键记录携带过期时间，后台定时扫描并清理已过期的记录 |
| F5 | HTTP 中间件 (Middleware) | 提供标准 `http.Handler` 中间件，可直接集成到任何 net/http 兼容的 Web 框架中 |
| F6 | 手动过期清理 (CleanExpired) | 主动扫描并清理过期的幂等记录，返回清理数量 |
| F7 | 存在性查询 (Contains) | 查询幂等键是否在有效期内，不改变其状态 |
| F8 | 获取缓存结果 (Get) | 获取指定幂等键的缓存结果（状态码和响应体） |
| F9 | 设置缓存结果 (Set) | 手动设置幂等键的缓存结果 |
| F10 | 删除缓存 (Delete) | 手动删除指定幂等键的缓存记录 |
| F11 | 全量清空 (Clear) | 清空所有幂等缓存记录，重置中间件状态 |
| F12 | 数量统计 (Count) | 查询当前有效的幂等记录总数（排除已过期但尚未清理的记录） |
| F13 | 生命周期管理 (Start/Stop) | 启动和停止后台清理协程，支持幂等调用；Stop 后所有操作永久拒绝 |
| F14 | 可配置幂等键头 | 支持自定义 HTTP 请求头名称作为幂等键的来源 |

## 3. 核心结构体与职责

### 3.1 Config - 幂等中间件配置

```go
type Config struct {
    TTL           time.Duration // 幂等键的过期时间，记录在此时间后视为过期
    CleanInterval time.Duration // 后台清理协程的执行间隔
    KeyHeader     string        // HTTP 请求中幂等键的请求头名称
}
```

**配置约束与默认值：**
- `TTL`：必须 >= 0。设置为 **负数** 时返回 `ErrInvalidConfig`；设置为 **0** 时自动使用默认值 5 分钟
- `CleanInterval`：必须 >= 0。设置为 **负数** 时返回 `ErrInvalidConfig`；设置为 **0** 时自动根据 `TTL / 5` 推导（最少 1 秒）
- `KeyHeader`：设置为空字符串时自动使用默认值 `"X-Idempotency-Key"`
- `CleanInterval` 不宜大于 `TTL`，否则过期记录可能长时间驻留内存
- 推荐配置：`CleanInterval` 为 `TTL` 的 1/5 ~ 1/2，在清理频率和 CPU 开销间取得平衡

### 3.2 Idempotent - 幂等中间件主体

```go
type Idempotent struct {
    cfg       Config              // 配置快照
    mu        sync.Mutex          // 保护内部状态的互斥锁
    cache     map[string]*cacheEntry  // 幂等键 → 缓存结果的映射
    pending   map[string]*pendingEntry // 正在处理中的幂等键 → 等待条目
    running   bool                // 后台清理协程是否运行中
    stopped   bool                // 是否已永久停止（Stop 调用后置为 true，不可逆）
    stopCh    chan struct{}       // 后台协程停止信号通道
    wg        sync.WaitGroup      // 后台协程同步等待组
}
```

**主要职责：**
- 维护幂等键与响应结果的缓存映射，通过 `cache` 提供 O(1) 的结果查询
- 管理并发请求的等待队列，通过 `pending` 追踪正在处理中的请求
- 驱动后台定时清理协程，自动回收过期记录
- 维护 `stopped` 生命周期标记，`Stop()` 后永久拒绝所有操作
- 提供 HTTP 中间件方法，无缝集成到 HTTP 处理链路
- 保证线程安全，通过互斥锁保护所有内部状态访问

### 3.3 cacheEntry - 缓存记录条目

```go
type cacheEntry struct {
    key        string    // 幂等键
    statusCode int       // 缓存的 HTTP 响应状态码
    body       []byte    // 缓存的 HTTP 响应体
    expiresAt  time.Time // 记录过期时间
}
```

**主要职责：**
- 存储幂等键对应的完整响应结果，包括状态码和响应体
- `expiresAt` 字段记录过期时间戳，用于判断记录是否仍然有效

### 3.4 pendingEntry - 待处理请求条目

```go
type pendingEntry struct {
    key        string        // 幂等键
    done       chan struct{} // 处理完成信号通道
    statusCode int           // 处理结果状态码（处理完成后填充）
    body       []byte        // 处理结果响应体（处理完成后填充）
}
```

**主要职责：**
- 表示一个正在被处理的幂等请求
- 通过 `done` channel 实现并发等待：其他相同幂等键的请求阻塞在该 channel 上
- 处理完成后填充 `statusCode` 和 `body`，供等待的请求读取结果

### 3.5 responseRecorder - HTTP 响应记录器

```go
type responseRecorder struct {
    header     http.Header
    statusCode int
    body       []byte
}
```

**主要职责：**
- 实现 `http.ResponseWriter` 接口，用于捕获下游处理器的响应
- 在中间件执行时，将真实的 `ResponseWriter` 替换为记录器，以便缓存响应结果
- 缓存命中时，将记录的响应头、状态码和响应体写回真实的 `ResponseWriter`

### 3.6 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyKey` | 幂等键为空 | 调用 `Execute("")`、`Get("")`、`Set("", ...)`、`Delete("")`、`Contains("")` 传入空字符串 |
| `ErrIdempotentStopped` | 幂等中间件已永久停止 | 调用 `Stop()` 后，任何公共方法都会返回此错误。`Stop()` 一旦调用不可逆。 |
| `ErrInvalidConfig` | 配置无效 | `NewIdempotentWithConfig()` 传入的 `TTL` 或 `CleanInterval` 为负数时返回此错误。 |
| `ErrHandlerNil` | 处理函数为空 | 调用 `Execute(key, nil)` 传入 nil 处理器函数 |

## 4. 核心机制详解

### 4.1 幂等执行流程 (Execute)

```
Execute(key, handler)
   │
   ├─ key == "" → 返回 (0, nil, false, ErrEmptyKey)
   ├─ handler == nil → 返回 (0, nil, false, ErrHandlerNil)
   │
   ├─ mu.Lock()
   │
   ├─ stopped == true → mu.Unlock()，返回 (0, nil, false, ErrIdempotentStopped)
   │
   ├─ [cache 中存在 key]
   │     │
   │     ├─ expiresAt > now（未过期）
   │     │     └─ mu.Unlock()，返回 (statusCode, body, true, nil) → 缓存命中
   │     │
   │     └─ expiresAt <= now（已过期）
   │           └─ delete(cache, key)，继续往下执行
   │
   ├─ [pending 中存在 key] —— 有相同键的请求正在处理中
   │     ├─ 取出 pendingEntry
   │     ├─ mu.Unlock()
   │     ├─ <-pending.done —— 阻塞等待第一个请求完成
   │     └─ 返回 (pending.statusCode, pending.body, true, nil)
   │
   └─ [既无缓存也无待处理] —— 当前请求是第一个
         ├─ 创建 pendingEntry{key, done: make(chan struct{})}
         ├─ 加入 pending[key] = pendingEntry
         ├─ mu.Unlock()
         │
         ├─ 调用 handler()，执行业务逻辑，得到 (statusCode, body)
         │
         ├─ mu.Lock()
         ├─ [stopped == true]
         │     ├─ delete(pending, key)
         │     ├─ close(pending.done)
         │     ├─ mu.Unlock()
         │     └─ 返回 (0, nil, false, ErrIdempotentStopped)
         │
         ├─ 填充 pendingEntry.statusCode 和 .body
         ├─ 创建 cacheEntry 并加入 cache[key]
         ├─ 从 pending 中删除该条目
         ├─ close(pending.done) —— 唤醒所有等待的请求
         ├─ mu.Unlock()
         │
         └─ 返回 (statusCode, body, false, nil) → 首次执行
```

**缓存命中判断逻辑：**
- 优先检查 `cache` 中是否有已完成的缓存记录
- 如果缓存存在但已过期，则删除过期记录，继续处理（视为新请求）
- 如果有相同键的请求正在 `pending` 队列中，则加入等待

### 4.2 并发请求锁等待机制

当多个相同幂等键的请求同时到达时，模块采用"单飞（single-flight）"模式确保只有一个请求执行业务逻辑，其余请求共享结果：

```
  请求1 ──┐
  请求2 ──┤
  请求3 ──┼──▶ [检查 pending 队列]
  请求4 ──┤     │
  请求5 ──┘     ├─ 请求1：第一个到达，创建 pendingEntry，执行业务逻辑
                │
                ├─ 请求2-5：发现 pendingEntry 已存在，阻塞在 done channel 上
                │
                ▼
           请求1处理完成
                │
                ├─ 填充结果到 cache 和 pendingEntry
                ├─ close(done) —— 同时唤醒所有等待的请求
                │
                ▼
           请求2-5被唤醒，直接读取 pendingEntry 中的结果返回
```

**机制特点：**
- **避免重复执行**：相同幂等键的并发请求中，业务逻辑仅执行一次
- **结果一致性**：所有等待的请求获得完全相同的响应结果
- **无忙等**：通过 channel 阻塞等待，不消耗 CPU
- **自动清理**：处理完成后自动从 pending 队列中移除条目

### 4.3 HTTP 中间件工作流程

```
Middleware(next http.Handler) http.Handler
   │
   └─ 返回 http.HandlerFunc(func(w, r))
         │
         ├─ 从请求头中读取幂等键（KeyHeader 配置项）
         ├─ [幂等键为空] → 直接调用 next.ServeHTTP(w, r)，跳过幂等逻辑
         │
         ├─ 创建 responseRecorder（替代真实 ResponseWriter）
         │
         ├─ 构造 handler 函数：内部调用 next.ServeHTTP(rr, r)
         │
         ├─ 调用 Execute(key, handler)，获得 (statusCode, body, fromCache, err)
         │
         ├─ [err != nil]
         │     ├─ ErrIdempotentStopped → 返回 503 Service Unavailable
         │     └─ 其他错误 → 降级为直接调用 next.ServeHTTP(w, r)
         │
         ├─ 将 rr.header 中的响应头复制到真实 ResponseWriter
         │
         ├─ 设置 X-Idempotent-Cache 响应头：
         │     ├─ fromCache=true  → "HIT"
         │     └─ fromCache=false → "MISS"
         │
         ├─ w.WriteHeader(statusCode)
         └─ w.Write(body)
```

**响应头说明：**
- `X-Idempotent-Cache: HIT`：该请求命中了幂等缓存，未执行业务逻辑
- `X-Idempotent-Cache: MISS`：该请求未命中缓存，执行业务逻辑并缓存了结果

### 4.4 过期与清理机制

每个幂等键记录都携带 `expiresAt` 过期时间戳，采用"惰性删除 + 定期清理"双策略：

**惰性删除（访问时检查）：**
- 每次 `Get`、`Contains`、`Execute` 访问时，先检查记录是否过期
- 如已过期则立即删除该记录，返回"不存在"结果
- 优点：过期记录一旦被访问就会被清理，不占用有效查询结果
- 缺点：永不访问的过期记录会一直驻留内存

**定期清理（后台协程）：**
- 后台协程按 `CleanInterval` 间隔定时执行全量扫描
- 遍历所有缓存记录，删除 `expiresAt <= now` 的过期条目
- 优点：确保所有过期记录最终都会被清理，防止内存无限增长
- 缺点：扫描有一定的 CPU 开销

### 4.5 后台定时清理流程

```
Start()
   │
   ├─ mu.Lock()
   ├─ stopped == true → Unlock 直接返回（已停止，无法重启）
   ├─ 已运行 → Unlock 直接返回
   ├─ running = true
   ├─ stopCh = make(chan struct{})
   ├─ mu.Unlock()
   │
   ├─ wg.Add(1)
   └─ 启动 cleanLoop 协程

cleanLoop（后台协程）
   │
   ├─ 创建 ticker = time.NewTicker(CleanInterval)
   │
   └─ [循环]
         │
         ├─ select
         │     ├─ stopCh 关闭 → ticker.Stop()，wg.Done()，退出
         │     └─ ticker.C 触发 → 调用 CleanExpired()（忽略返回值）
         │
         └─ 继续循环

Stop()
   │
   ├─ mu.Lock()
   ├─ stopped == true → Unlock 直接返回
   ├─ stopped = true  ← 永久标记，不可逆
   ├─ 若 running：
   │     ├─ running = false
   │     ├─ close(stopCh)
   ├─ mu.Unlock()
   │
   └─ wg.Wait()（等待清理协程退出）
```

### 4.6 生命周期与资源回收

**典型生命周期：**
```
NewIdempotent() → Start() → [Execute()/Middleware 反复调用] → Stop() → 所有后续操作返回 ErrIdempotentStopped
                    │
                    └─ 可选：不调用 Start()，仅手动调用 CleanExpired()
```

**生命周期状态说明：**

| 状态 | `stopped` | `running` | 行为 |
|------|-----------|-----------|------|
| **初始态** | `false` | `false` | 所有公共方法正常工作，无后台清理。需手动调用 `CleanExpired()` |
| **运行态** | `false` | `true` | 所有公共方法正常工作，后台协程按 `CleanInterval` 自动清理 |
| **已停止** | `true` | `false` | 所有公共方法均返回 `ErrIdempotentStopped`。`Start()` 无效。`Stop()` 幂等。 |

**不可逆停止约定：**
- `Stop()` 一旦调用，`stopped` 标记永久设置为 `true`，中间件进入"已停止"状态，**不可逆转**
- 调用方可以通过检查 `errors.Is(err, ErrIdempotentStopped)` 来判断中间件是否已停止
- 如需重新使用，必须创建新的 `Idempotent` 实例
- `Start()` 在已停止状态下调用会被静默忽略，不会重启

**资源安全：**
- `Start()` 和 `Stop()` 均支持幂等调用，重复调用不会产生副作用
- `Stop()` 会阻塞直到后台清理协程完全退出，确保协程泄漏防护
- 不调用 `Start()` 也可正常使用幂等功能（仅缺失自动清理，需手动调用 `CleanExpired()`）
- `Stop()` 即使在未调用 `Start()` 的情况下也可安全调用，调用后中间件同样进入永久停止状态
- `Stop()` 时如果有待处理（pending）的请求，会唤醒它们并返回 `ErrIdempotentStopped`

### 4.7 公共方法签名与错误返回约定

所有可能失败的公共方法均返回 `error`，统一错误处理模式：

| 方法 | 签名 | 错误返回说明 |
|------|------|-------------|
| `NewIdempotentWithConfig` | `(*Idempotent, error)` | 配置负数值时返回 `ErrInvalidConfig` |
| `Execute` | `(int, []byte, bool, error)` | 空键返回 `ErrEmptyKey`；nil 处理器返回 `ErrHandlerNil`；已停止返回 `ErrIdempotentStopped`。第三个返回值 `bool` 表示是否命中缓存 |
| `Middleware` | `func(http.Handler) http.Handler` | 无错误返回，内部错误自动降级处理 |
| `Get` | `(int, []byte, bool, error)` | 空键返回 `ErrEmptyKey`；已停止返回 `ErrIdempotentStopped` |
| `Set` | `error` | 空键返回 `ErrEmptyKey`；已停止返回 `ErrIdempotentStopped` |
| `Delete` | `error` | 空键返回 `ErrEmptyKey`；已停止返回 `ErrIdempotentStopped` |
| `Contains` | `(bool, error)` | 空键返回 `ErrEmptyKey`；已停止返回 `ErrIdempotentStopped` |
| `Count` | `(int, error)` | 已停止返回 `ErrIdempotentStopped` |
| `CleanExpired` | `(int, error)` | 已停止返回 `ErrIdempotentStopped` |
| `Clear` | `error` | 已停止返回 `ErrIdempotentStopped` |

## 5. 线程安全设计

所有公共方法均通过互斥锁 `mu` 保护内部状态：
- **写操作**（`Execute`、`Set`、`Delete`、`Clear`、`CleanExpired`、`Start`、`Stop`）：获取排他锁
- **读操作**（`Get`、`Contains`、`Count`）：同样获取排他锁（当前使用 `sync.Mutex`，如读多写少场景可升级为 `RWMutex`）
- **并发安全验证**：单元测试中的 `TestConcurrent_*` 系列测试通过多协程并发调用验证无竞态条件
- **Pending 队列**：处理中的请求通过 channel 实现等待，不占用锁资源

## 6. 使用示例

### 6.1 基础使用：HTTP 接口幂等性

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "net/http"
    "time"
    "solocoder-go/internal/idempotent"
)

func main() {
    cfg := idempotent.Config{
        TTL:           10 * time.Minute,  // 幂等键 10 分钟过期
        CleanInterval: 2 * time.Minute,   // 每 2 分钟清理一次
        KeyHeader:     "X-Idempotency-Key",
    }
    i, err := idempotent.NewIdempotentWithConfig(cfg)
    if err != nil {
        log.Fatalf("创建幂等中间件失败: %v", err)
    }
    i.Start()
    defer i.Stop()

    createOrderHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 模拟创建订单的业务逻辑
        orderID := "ORD-" + time.Now().Format("20060102150405")
        w.Header().Set("X-Order-ID", orderID)
        w.WriteHeader(http.StatusCreated)
        fmt.Fprintf(w, `{"order_id":"%s","status":"created"}`, orderID)
    })

    mux := http.NewServeMux()
    mux.Handle("/orders", i.Middleware(createOrderHandler))

    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", mux)
}
```

客户端使用方式：
```bash
# 第一次请求，执行业务逻辑，响应头 X-Idempotent-Cache: MISS
curl -X POST http://localhost:8080/orders \
  -H "X-Idempotency-Key: my-unique-key-001"

# 第二次请求（相同幂等键），直接返回缓存结果，X-Idempotent-Cache: HIT
curl -X POST http://localhost:8080/orders \
  -H "X-Idempotency-Key: my-unique-key-001"
```

### 6.2 编程式使用：直接调用 Execute

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "net/http"
    "time"
    "solocoder-go/internal/idempotent"
)

func main() {
    i := idempotent.NewIdempotent()
    i.Start()
    defer i.Stop()

    processPayment := func(orderID string) (int, []byte, error) {
        handler := func() (int, []byte) {
            // 实际的支付处理逻辑
            time.Sleep(100 * time.Millisecond)
            result := fmt.Sprintf(`{"order_id":"%s","status":"paid"}`, orderID)
            return http.StatusOK, []byte(result)
        }

        statusCode, body, fromCache, err := i.Execute(orderID, handler)
        if errors.Is(err, idempotent.ErrIdempotentStopped) {
            return 0, nil, fmt.Errorf("幂等服务已停止")
        }
        if err != nil {
            return 0, nil, err
        }
        if fromCache {
            log.Printf("命中幂等缓存: %s", orderID)
        } else {
            log.Printf("首次处理请求: %s", orderID)
        }
        return statusCode, body, nil
    }

    // 第一次调用：执行业务逻辑
    status, body, _ := processPayment("order-123")
    fmt.Printf("Status: %d, Body: %s\n", status, string(body))

    // 第二次调用：命中缓存
    status, body, _ = processPayment("order-123")
    fmt.Printf("Status: %d, Body: %s\n", status, string(body))
}
```

### 6.3 手动清理模式（不启动后台协程）

```go
i, err := idempotent.NewIdempotentWithConfig(idempotent.Config{
    TTL: 5 * time.Minute,
})
if err != nil {
    log.Fatal(err)
}
// 注意：不调用 i.Start()，无后台协程

for {
    batch := processBatch()
    for _, req := range batch {
        handler := func() (int, []byte) {
            return handleRequest(req)
        }
        statusCode, body, fromCache, err := i.Execute(req.ID, handler)
        if errors.Is(err, idempotent.ErrIdempotentStopped) {
            log.Fatal("幂等中间件已停止")
        }
        if err != nil {
            log.Printf("幂等处理失败: %v", err)
            continue
        }
        // 使用结果...
    }
    // 每批处理完后手动清理过期记录
    cleaned, err := i.CleanExpired()
    if err != nil {
        log.Printf("清理失败: %v", err)
    }
    if cleaned > 0 {
        log.Printf("清理了 %d 条过期幂等记录", cleaned)
    }
}
```

### 6.4 监控幂等中间件状态

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        count, err := i.Count()
        if errors.Is(err, idempotent.ErrIdempotentStopped) {
            log.Printf("Idempotent: 已停止")
            return
        }
        if err != nil {
            log.Printf("Idempotent: 获取数量失败: %v", err)
            continue
        }
        log.Printf("Idempotent: 有效记录数 = %d", count)
    }
}()
```

### 6.5 并发请求场景演示

```go
func TestConcurrentIdempotent(t *testing.T) {
    i := idempotent.NewIdempotent()
    i.Start()
    defer i.Stop()

    var callCount int64
    numGoroutines := 20

    handler := func() (int, []byte) {
        atomic.AddInt64(&callCount, 1)
        time.Sleep(50 * time.Millisecond) // 模拟业务处理耗时
        return http.StatusOK, []byte("result")
    }

    var wg sync.WaitGroup
    for g := 0; g < numGoroutines; g++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            statusCode, body, fromCache, err := i.Execute("same-key", handler)
            assert.NoError(t, err)
            assert.Equal(t, http.StatusOK, statusCode)
            assert.Equal(t, "result", string(body))
        }()
    }
    wg.Wait()

    // 20 个并发请求，但业务逻辑只执行了一次
    assert.Equal(t, int64(1), callCount)
}
```

### 6.6 优雅关闭示例

```go
stopCh := make(chan os.Signal, 1)
signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)

go func() {
    <-stopCh
    log.Println("收到关闭信号，停止幂等中间件...")
    i.Stop()
    log.Println("幂等中间件已停止")
}()

// 启动 HTTP 服务器...
```

## 7. 文件结构

```
internal/idempotent/
├── idempotent.go      # 幂等中间件核心实现
└── idempotent_test.go # 单元测试（覆盖正常流程、边界条件、异常分支、并发场景）

docs/
└── idempotent.md      # 本文档
```

## 8. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **基础创建** | `TestNewIdempotent`、`TestDefaultConfig`、`TestNewIdempotentWithConfig_*` | 构造函数、默认值、配置推导、无效配置校验 |
| **幂等执行** | `TestExecute_FirstRequest`、`TestExecute_CacheHit`、`TestExecute_MultipleKeys` | 首次执行、缓存命中、多键验证 |
| **响应缓存** | `TestExecute_DifferentStatusCodes`、`TestGet`、`TestSet`、`TestSet_Overwrite` | 状态码/响应体缓存、Get/Set 操作、覆盖写入 |
| **边界条件** | `TestExecute_EmptyKey`、`TestExecute_NilHandler`、`TestDelete` | 空键处理、nil 处理器、删除操作 |
| **并发锁等待** | `TestExecute_ConcurrentSameKey`、`TestExecute_ConcurrentDifferentKeys` | 同键并发只执行一次、不同键并发独立执行 |
| **HTTP 中间件** | `TestMiddleware_NoKey`、`TestMiddleware_WithKeyFirstRequest`、`TestMiddleware_WithKeyCacheHit`、`TestMiddleware_CustomHeader` | 无键跳过、首次请求 MISS、缓存命中 HIT、自定义请求头 |
| **过期清理** | `TestCleanExpired_NoExpired`、`TestCleanExpired_AllExpired`、`TestCleanExpired_PartialExpired` | 无过期/全过期/部分过期清理 |
| **过期行为** | `TestExecute_ExpiredThenReexecute`、`TestGet_Expired`、`TestContains_Expired` | 过期后重新执行、Get/Contains 过期判断 |
| **Count 语义** | `TestCount`、`TestCount_ExcludesExpired` | Count 只统计有效记录、排除已过期但未清理的记录 |
| **生命周期** | `TestStartStop_Idempotent`、`TestStartStop_BackgroundCleanup` | 幂等启停、后台自动清理 |
| **停止状态** | `TestStop_RejectsAllOperations`、`TestStop_WithoutStart`、`TestStart_AfterStop` | Stop 后所有操作返回错误、不可逆 |
| **并发安全** | `TestConcurrent_ExecuteAndClean` | 执行与清理并发无竞态 |
| **内存泄漏** | `TestMemoryLeak_AfterCleanup` | 长期运行后内存占用受控 |
| **待处理清理** | `TestExecute_PendingEntryCleanupOnStop` | Stop 时正确处理待处理请求 |
| **响应记录器** | `TestResponseRecorder`、`TestResponseRecorder_DefaultStatus` | responseRecorder 实现正确性 |
