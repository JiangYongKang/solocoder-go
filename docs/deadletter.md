# DeadLetter 死信队列处理器模块需求文档

## 1. 模块概述

DeadLetter 是一个基于内存的死信队列（Dead Letter Queue, DLQ）处理器模块，用于管理消息处理失败后的重试、永久失败和告警。当业务系统中的消息经过多次重试仍无法成功处理时，该模块提供统一的失败消息托管、延迟重试策略、永久失败标记和阈值告警等能力，防止消息处理故障扩散，保障系统稳定性。

### 主要特性

- **失败消息自动转移**：当消息处理失败次数达到上限后，自动转移到死信队列，完整记录原始主题、失败原因和转移时间戳
- **延时重试策略**：支持固定延迟和指数递增延迟两种策略，消息进入死信队列后等待指定时间再重新投递给消费者处理
- **最大重试次数限制**：死信消息的重新投递也有独立的重试次数上限，超过上限后标记为永久失败
- **死信告警机制**：当死信队列中待处理消息数量超过配置阈值时触发告警回调，支持按失败原因分类统计
- **手动干预能力**：支持手动触发重试、查询消息状态、移除消息、清理永久失败消息
- **优雅关闭**：支持优雅关闭，等待正在执行中的处理任务完成

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    MaxRetries     int
    DelayStrategy    DelayStrategy
    AlertThreshold   int
    AlertCallback    AlertCallback
}
```

**职责**：定义死信处理器的全局配置参数，在创建处理器实例时传入。

| 字段 | 说明 |
|------|------|
| MaxRetries | 默认最大重试次数，每条死信消息的重新投递上限 |
| DelayStrategy | 延迟重试策略配置 |
| AlertThreshold | 告警阈值，死信队列中待处理消息超过此数量时触发告警 |
| AlertCallback | 告警回调函数，触发告警时被调用 |

### 2.2 DelayStrategy

```go
type DelayStrategy struct {
    Type     DelayStrategyType
    Base     time.Duration
    Max      time.Duration
}
```

**职责**：定义延迟重试的策略。

| 字段 | 说明 |
|------|------|
| Type | 延迟策略类型：`DelayStrategyFixed（固定延迟）、`DelayStrategyExponential`（指数递增延迟 |
| Base | 基础延迟时间 |
| Max | 最大延迟时间（指数策略的上限，防止延迟无限增大） |

### 2.3 DeadLetterMessage

```go
type DeadLetterMessage struct {
    ID             string
    OriginalTopic  string
    Payload        interface{}
    FailureReason  string
    TransferTime   time.Time
    RetryCount     int
    MaxRetries     int
    NextRetryAt    time.Time
    Status         MessageStatus
    LastError      string
}
```

**职责**：表示一条死信消息，完整记录失败消息的全部元数据与处理状态。

| 字段 | 说明 |
|------|------|
| ID | 死信消息唯一标识，转移到死信队列时自动生成 |
| OriginalTopic | 原始消息所属的原始主题/队列名 |
| Payload | 消息负载数据，供消费者重试时使用 |
| FailureReason | 首次进入死信队列的失败原因 |
| TransferTime | 消息转移到死信队列的时间戳 |
| RetryCount | 已重试次数 |
| MaxRetries | 当前消息允许的最大重试次数 |
| NextRetryAt | 下一次重试的时间点 |
| Status | 当前消息状态 |
| LastError | 最近一次重试失败的错误信息 |

### 2.4 MessageStatus

消息状态枚举：

| 状态 | 说明 |
|------|------|
| `StatusPending` | 等待重试：消息在死信队列中等待下一次重试时间点到达 |
| `StatusRetrying` | 正在重试：消息已被取出，正在调用 Handler 处理中 |
| `StatusPermanentlyFailed` | 永久失败：重试次数已达上限，不再自动投递 |
| `StatusProcessed` | 处理成功：重试成功，消息已从死信队列移除 |

### 2.5 AlertInfo

```go
type AlertInfo struct {
    TotalCount    int
    ReasonStats   map[string]int
    Threshold     int
    Timestamp     time.Time
}
```

**职责**：封装告警触发时传递给回调函数的告警信息。

| 字段 | 说明 |
|------|------|
| TotalCount | 当前待处理死信消息总数 |
| ReasonStats | 按失败原因分类的统计，`map[失败原因]消息数 |
| Threshold | 触发告警的阈值 |
| Timestamp | 告警触发时间戳 |

### 2.6 Processor

```go
type Processor struct {
    // ... 内部字段省略
}
```

**职责**：死信队列处理器核心管理器，负责：

- 接收并管理死信消息的存储与状态管理
- 调度延迟重试到期的消息自动投递给消费者
- 按策略计算下一次重试时间
- 检测告警阈值并触发回调
- 提供消息查询、手动重试、消息移除等管理接口
- 生命周期管理（启动/停止

## 3. 消息完整流转过程

### 3.1 状态机流转

```
                    ┌──────────────────────┐
                    │  MoveToDeadLetter() │
                    │  (业务系统调用)  │
                    └─────────┬────────┘
                              │
                              ▼
                  ┌──────────────────────┐
                  │    StatusPending     │
                  │ (等待重试时间到达)   │
                  └──────────┬───────────┘
                             │
              ┌──────────────┼───────────────┐
              │              │               │
     NextRetryAt 未到   NextRetryAt 已到   手动 RetryMessage()
              │              │               │
              │              ▼               │
              │   ┌──────────────────────┐   │
              │   │    StatusRetrying    │   │
              │   │  (调用 Handler 处理) │◄──┘
              │   └──────────┬───────────┘
              │              │
              │    ┌─────────┴─────────┐
              │    │                   │
              │    ▼                   ▼
              │  Handler 成功       Handler 失败
              │    │                   │
              │    │               RetryCount++
              │    │                   │
              │    │         ┌─────────┴──────────┐
              │    │         │                       │
              │    │         ▼                       ▼
              │    │  RetryCount > MaxRetries      否则
              │    │         │                       │
              ▼    ▼         ▼                       │
     ┌─────────────────┐  ┌──────────────────────┐    │
     │StatusProcessed│  │    StatusPending     │◄───┘
     │ (从队列移除) │  │ (计算 NextRetryAt)  │
     └─────────────────┘  └──────────────────────┘
                                  │
                                  ▼
                        ┌──────────────────────┐
                        │StatusPermanentlyFailed│
                        │ (标记永久失败，保留) │
                        └──────────────────────┘
```

### 3.2 流转阶段说明

#### 阶段一：失败消息转移

1. 业务系统在消息处理失败且达到自身重试耗尽后，调用 `MoveToDeadLetter()`
2. 处理器为消息生成唯一 ID，记录：
   - 原始主题 `OriginalTopic`
   - 原始负载 `Payload`
   - 首次失败原因 `FailureReason`
   - 转移时间 `TransferTime`
3. 根据延迟策略计算首次重试时间 `NextRetryAt`
4. 消息状态设为 `StatusPending`
5. 检测死信队列中的消息数是否超过告警阈值，若超过且未触发过告警，则触发告警回调

#### 阶段二：等待重试

1. 处理器后台 `runLoop` 循环检测所有 `StatusPending` 状态的消息
2. 若消息的 `NextRetryAt` 到达当前时间：
   - 将消息状态变更为 `StatusRetrying`
   - 调用用户注册的 `MessageHandler` 重新处理消息

#### 阶段三：重试结果处理

**处理成功**：
- 消息状态设为 `StatusProcessed`
- 消息从死信队列中移除
- 重新检测告警阈值（队列消息数下降到阈值以下时，重置告警标记，允许下一次超过阈值时可再次触发告警

**处理失败且未超过重试上限**：
- `RetryCount` 加 1
- 记录最近一次错误信息 `LastError`
- 根据延迟策略计算下一次重试时间 `NextRetryAt`
- 消息状态回退到 `StatusPending`
- 等待下一次重试调度

**处理失败且超过重试上限**：
- `RetryCount` 加 1
- 记录最近一次错误信息 `LastError`
- 消息状态设为 `StatusPermanentlyFailed`
- 消息保留在队列中，不再自动重试

### 3.3 延迟重试策略计算

#### 固定延迟策略（`DelayStrategyFixed`）：

每次重试的延迟时间固定为 `DelayStrategy.Base`，不受重试次数影响。

#### 指数递增延迟策略（`DelayStrategyExponential`）：

第 n 次重试的延迟时间 = `Base * 2^n`，但不超过 `Max`：

```
delay = Base * (1 << retryCount)
if delay > Max {
    delay = Max
}
```

| 重试次数 (retryCount) | 延迟计算 (Base=10ms) |
|----------------------|---------------------|
| 0 | 10ms |
| 1 | 20ms |
| 2 | 40ms |
| 3 | 80ms |
| 4 | 160ms |
| 10 (Max=100ms) | 100ms (被 Max 值限制) |

### 3.4 告警触发机制

- 告警触发条件：死信队列中 `StatusPending` + `StatusRetrying` 状态的消息数 `>= AlertThreshold`
- 告警去重：阈值以上时只触发一次告警，直到消息数下降到阈值以下后，告警状态重置
- 告警信息包含：
  - 当前待处理消息总数
  - 各失败原因分类计数
  - 配置的告警阈值
  - 告警触发时间戳

## 4. API 使用示例

### 4.1 基本使用

```go
package main

import (
    "context"
    "fmt"
    "time"

    "solocoder-go/internal/deadletter"
)

func main() {
    // 1. 创建死信处理器
    dlq, err := deadletter.NewProcessor(deadletter.Config{
        MaxRetries: 3,
        AlertThreshold: 10,
        AlertCallback: func(info deadletter.AlertInfo) {
            fmt.Printf("死信队列告警示例: 总数=%d, 阈值=%d, 时间=%v\n",
                info.TotalCount, info.Threshold, info.Timestamp)
            for reason, cnt := range info.ReasonStats {
                fmt.Printf("  原因[%s]: %d 条\n", reason, cnt)
            }
        },
        DelayStrategy: deadletter.DelayStrategy{
            Type: deadletter.DelayStrategyExponential,
            Base: 100 * time.Millisecond,
            Max:  5 * time.Second,
        },
    })
    if err != nil {
        panic(err)
    }
    defer dlq.Stop()

    // 2. 设置消息处理函数（注意：必须监听 ctx.Done() 以响应优雅关闭信号
    dlq.SetHandler(func(ctx context.Context, msg *deadletter.DeadLetterMessage) error {
        payload := msg.Payload.(string)
        fmt.Printf("重试处理消息: topic=%s, payload=%s\n",
            msg.OriginalTopic, payload)

        // 最佳实践：在所有可能阻塞的操作中监听 ctx.Done()
        // 确保处理器优雅关闭时 Handler 能够及时退出
        select {
        case <-ctx.Done():
            return fmt.Errorf("handler cancelled: %w", ctx.Err())
        case result := <-processOrderAsync(payload):
            if result.err != nil {
                return result.err
            }
            return nil
        }
    })

    // 3. 启动处理器
    if err := dlq.Start(); err != nil {
        panic(err)
    }

    // 4. 业务系统处理失败，转移到死信队列
    id, err := dlq.MoveToDeadLetter(
        "order-events",
        `{"order_id":123}`,
        "database connection timeout",
        3,
    )
    if err != nil {
        fmt.Printf("转移失败: %v\n", err)
    }
    fmt.Printf("消息已转移到死信队列, ID=%s\n", id)
}
```

### 4.2 固定延迟策略

```go
dlq, _ := deadletter.NewProcessor(deadletter.Config{
    MaxRetries: 5,
    DelayStrategy: deadletter.DelayStrategy{
        Type: deadletter.DelayStrategyFixed,
        Base: 500 * time.Millisecond,
        Max:  500 * time.Millisecond,
    },
})
```

### 4.3 手动管理

```go
// 查询单条消息
msg, err := dlq.GetMessage("dl-xxx")
if err == nil {
    fmt.Printf("消息状态: %v, 已重试: %d 次\n", msg.Status, msg.RetryCount)
}

// 手动触发重试（忽略延迟，立即重试）
err = dlq.RetryMessage("dl-xxx")
if errors.Is(err, deadletter.ErrMaxRetriesExceeded) {
    fmt.Println("该消息已永久失败，无法重试")
}

// 移除单条消息
dlq.RemoveMessage("dl-xxx")

// 获取所有永久失败消息并清理
failed := dlq.GetMessagesByStatus(deadletter.StatusPermanentlyFailed)
fmt.Printf("永久失败消息数: %d\n", len(failed))
cleared := dlq.ClearPermanentlyFailed()
fmt.Printf("已清理 %d 条永久失败消息\n", cleared)
```

### 4.4 监控统计

```go
pending := dlq.PendingCount()          // 待处理消息数（等待重试 + 重试中
failed := dlq.PermanentlyFailedCount()  // 永久失败消息数
all := dlq.GetAllMessages()            // 所有消息

fmt.Printf("待处理: %d, 永久失败: %d, 总数: %d\n", pending, failed, len(all))
```

### 4.5 每条消息独立配置重试次数

```go
// 使用默认配置的 MaxRetries（传 -1）
dlq.MoveToDeadLetter("topic", payload, "reason", -1)

// 该消息最多重试 0 次（首次重试失败即永久失败）
dlq.MoveToDeadLetter("critical-topic", payload, "fatal error", 0)

// 该消息最多重试 5 次
dlq.MoveToDeadLetter("retryable-topic", payload, "temporary error", 5)
```

## 5. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrProcessorStopped | 处理器未启动或已停止时调用 `MoveToDeadLetter()` |
| `ErrMessageNotFound | 查询或操作不存在的消息 ID |
| `ErrInvalidConfig | 创建处理器时配置参数非法 |
| `ErrMaxRetriesExceeded | 对永久失败的消息调用 `RetryMessage()` |
| `ErrAlreadyStarted | 重复调用 `Start()` |
| `ErrHandlerNotSet | 调用 `Start()` 前未通过 `SetHandler()` |

## 6. Handler 上下文传递机制

### 6.1 上下文生命周期

处理器在内部维护一个可取消的 `context.Context`，其生命周期与处理器的运行状态严格绑定：

- **创建时**：`NewProcessor()` 初始化一个根 context
- **启动时**：`Start()` 重新创建一个全新的可取消 context，确保支持多次启停
- **停止时**：`Stop()` 首先调用 `cancel()` 取消 context，然后才等待任务完成

### 6.2 上下文传递路径

```
Stop() 被调用
    │
    ▼
p.cancel()  ──►  ctx.Done() 被触发
    │                     │
    │                     ▼
    │         所有正在执行的 Handler 收到取消信号
    │                     │
    │                     ▼
    │         Handler 检查 ctx.Done() 并及时返回
    │                     │
    ▼                     ▼
p.taskWg.Wait() ◄───  processMessage 返回
    │
    ▼
Stop() 正常返回
```

### 6.3 Handler 编写最佳实践

**MessageHandler** 的签名为 `func(ctx context.Context, msg *DeadLetterMessage) error`，其中 `ctx` 参数由处理器在调用时自动注入，开发者必须遵循以下最佳实践：

1. **必须监听 ctx.Done()**：在所有可能长时间阻塞的操作（如网络调用、数据库查询、循环处理）中，必须通过 `select` 监听 `ctx.Done()` 通道，确保优雅关闭时能够及时退出。

   **正确示例**：
   ```go
   dlq.SetHandler(func(ctx context.Context, msg *deadletter.DeadLetterMessage) error {
       // 错误示例：死循环不检查 ctx，会导致 Stop() 永久挂起
       // for {
       //     time.Sleep(time.Second)
       // }

       // 正确示例：循环中检查 ctx.Done()
       for {
           select {
           case <-ctx.Done():
               return ctx.Err()
           case <-time.After(time.Second):
               // 执行业务逻辑
           }
       }
   })
   ```

2. **传递 ctx 给下游操作**：将 ctx 传递给所有支持 context 的下游函数（如 HTTP 请求、数据库查询、Redis 操作等），确保取消信号能够完整传递。

   ```go
   dlq.SetHandler(func(ctx context.Context, msg *deadletter.DeadLetterMessage) error {
       // 将 ctx 传递给 HTTP Client
       req, _ := http.NewRequestWithContext(ctx, "POST", url, body)
       resp, err := http.DefaultClient.Do(req)
       // ...
   })
   ```

3. **不要忽略 ctx**：即使业务逻辑非常简单，也应该在合适的位置检查 ctx 是否已取消，避免小概率的长时间阻塞场景。

### 6.4 取消后的消息处理

当 Handler 因 ctx 被取消而返回错误时：
- 消息不会被标记为 `StatusProcessed`
- 消息会按照正常重试流程，根据 `RetryCount` 判断是否继续重试或标记为永久失败
- 这确保了在关闭过程中未完成处理的消息不会丢失，下次启动时可以继续处理

## 7. 优雅关闭保障策略

### 7.1 关闭流程详解

`Stop()` 方法采用三级保障机制，确保在各种情况下都能尽可能优雅地关闭：

```
调用 Stop()
    │
    ├─►  第一级：状态标记
    │       p.running = false
    │       禁止新的 MoveToDeadLetter() 调用
    │
    ├─►  第二级：取消信号广播
    │       p.cancel()  ──►  所有运行中 Handler 的 ctx.Done() 触发
    │       close(p.stopCh) ──► runLoop 退出调度循环
    │       p.wake() ──► 唤醒 runLoop，立即响应停止
    │
    └─►  第三级：等待任务完成
            p.wg.Wait() ──► 等待 runLoop 协程退出
            p.taskWg.Wait() ──► 等待所有 Handler 协程返回
                               （依赖 Handler 正确响应 ctx 取消）
            │
            ▼
          Stop() 返回
```

### 7.2 各级保障的作用

| 保障级别 | 机制 | 作用 | 失败后果 |
|----------|------|------|----------|
| 第一级 | `p.running = false` | 阻止新消息进入，防止关闭过程中任务堆积 | 新消息仍可入队，关闭时间延长 |
| 第二级 | `p.cancel()` | 向所有运行中 Handler 广播取消信号 | 核心保障，缺失会导致 Stop() 永久阻塞 |
| 第三级 | `taskWg.Wait()` | 确保所有 Handler 真正完成后才返回 | 提前返回可能导致资源泄漏 |

### 7.3 防止死循环阻塞的保障

为了防止 Handler 内部死循环或忽略 ctx 导致 `Stop()` 永久挂起，系统提供以下机制：

1. **Context 取消是强制性信号**：`Stop()` 调用时必然会调用 `p.cancel()`，这是不可绕过的机制
2. **开发者责任**：Handler 必须遵循最佳实践监听 `ctx.Done()`，这是保障优雅关闭的前提
3. **设计原则**：处理器不强制 kill Handler goroutine（Go 不支持强制停止 goroutine），而是通过 context 取消信号进行协作式取消

### 7.4 优雅关闭的最佳实践

1. **设置合理的超时**：如果担心某些 Handler 可能忽略 ctx，可以在 `Stop()` 外层设置超时保护：

   ```go
   stopped := make(chan struct{})
   go func() {
       dlq.Stop()
       close(stopped)
   }()

   select {
   case <-stopped:
       fmt.Println("优雅关闭完成")
   case <-time.After(30 * time.Second):
       fmt.Println("警告：部分 Handler 未及时响应关闭，强制继续")
   }
   ```

2. **分阶段关闭**：在关闭前先通过 `PendingCount()` 观察队列状态，必要时先手动处理关键消息。

3. **监控关闭耗时**：正常情况下 `Stop()` 应该在毫秒级返回，如果耗时超过预期，说明存在 Handler 未正确响应 ctx 取消，需要检查 Handler 实现。

## 8. 线程安全说明

DeadLetter Processor 所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：

- 处理器内部通过 `sync.Mutex` 保护共享数据结构
- 后台调度循环 (`runLoop`) 单协程运行，避免竞争条件
- Handler panic 自动捕获并转化为错误，不会导致整个处理器崩溃
- `Stop()` 方法会等待所有正在执行的处理任务完成后再返回

## 9. 资源与生命周期

- **创建**：`NewProcessor(cfg)` 创建实例，验证配置参数，初始化根 context
- **设置 Handler**：`SetHandler(handler)` 设置消息处理函数
- **启动**：`Start()` 启动后台调度循环，重新创建可取消 context
- **运行**：转移消息、查询状态、手动重试、移除消息等
- **停止**：`Stop()` 优雅关闭：
  - 标记 `running = false`，停止接收新消息
  - 调用 `cancel()` 广播取消信号给所有运行中的 Handler
  - 等待 `runLoop` 协程退出
  - 等待所有正在执行的 Handler 协程返回（依赖 Handler 正确响应 ctx）
  - 返回后所有 goroutine 全部退出
- **幂等性**：`Start()`、`Stop()` 均为幂等操作，支持多次启停
