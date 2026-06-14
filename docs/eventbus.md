# EventBus 事件总线模块需求文档

## 1. 模块概述

EventBus 是一个基于内存的事件发布-订阅路由模块，提供灵活的事件分发与处理能力。它支持按事件类型注册订阅者、同步/异步两种分发模式、基于事件属性的过滤机制、订阅者优先级排序以及中断分发等高级特性，适用于系统内部模块解耦、事件驱动架构、领域事件处理等场景。

### 主要特性

- **事件类型注册**：支持按事件类型注册多个订阅者处理函数，支持动态添加和移除订阅者
- **同步与异步分发**：同步模式下发布者阻塞等待所有订阅者处理完成；异步模式下发布者立即返回，订阅者在后台协程中按优先级顺序处理，支持中断机制
- **事件过滤**：订阅者注册时可指定过滤条件，只有满足条件的事件才会被投递，支持等值比较、范围比较及逻辑组合过滤
- **优先级排序**：每个订阅者可设置优先级权重，同一事件按优先级从高到低依次调用，高优先级订阅者可中断后续低优先级订阅者的执行
- **Panic 恢复**：自动捕获订阅者处理函数中的 panic 并转化为错误，避免单个订阅者崩溃影响整个事件分发流程
- **并发安全**：所有公共方法均为并发安全，可在多个协程中同时调用

## 2. 核心结构体

### 2.1 Event

```go
type Event struct {
    Type       string
    Payload    interface{}
    Attributes map[string]interface{}
}
```

**职责**：表示一个事件单元，封装事件的类型、负载数据和属性集合。

| 字段 | 说明 |
|------|------|
| `Type` | 事件类型标识，用于路由匹配，如 "user.created"、"order.paid" |
| `Payload` | 事件负载数据，可存储任意类型的业务数据，由订阅者处理函数解析使用 |
| `Attributes` | 事件属性集合，用于事件过滤，键为属性名，值为任意类型 |

### 2.2 HandlerFunc

```go
type HandlerFunc func(event Event) error
```

**职责**：订阅者处理函数类型定义，接收一个 `Event` 参数并返回 `error`。

- 返回 `nil` 表示处理成功
- 返回非 `nil error 表示处理失败，同步分发时会收集第一个非 `nil error
- 返回 `ErrInterrupt` 可中断后续低优先级订阅者的调用
- 返回其他 `error 表示处理出错

### 2.3 Filter 接口

```go
type Filter interface {
    Match(event Event) bool
}
```

**职责**：事件过滤器接口，定义事件匹配规则。只有 `Match()` 返回 `true` 的事件才会被投递给对应的订阅者。

### 2.4 EqualsFilter

```go
type EqualsFilter struct {
    Key   string
    Value interface{}
}
```

**职责**：等值比较过滤器，检查事件 `Attributes` 中指定键的值是否与预期值相等（使用 `reflect.DeepEqual` 比较）。

### 2.5 RangeFilter

```go
type RangeFilter struct {
    Key    string
    Min    float64
    Max    float64
    HasMin bool
    HasMax bool
}
```

**职责**：范围比较过滤器，检查事件 `Attributes` 中指定键的数值是否在指定范围内。支持以下模式：
- `HasMin=false, HasMax=false`：无范围限制（等同于无过滤）
- `HasMin=true, HasMax=false`：仅检查下限 `>= Min`
- `HasMin=false, HasMax=true`：仅检查上限 `<= Max`
- `HasMin=true, HasMax=true`：检查闭区间 `[Min, Max]`

支持的数值类型：`int`, `int8`, `int16`, `int32`, `int64`, `uint`, `uint8`, `uint16`, `uint32`, `uint64`, `float32`, `float64`。

### 2.6 AndFilter / OrFilter / NotFilter

```go
type AndFilter struct {
    Filters []Filter
}

type OrFilter struct {
    Filters []Filter
}

type NotFilter struct {
    Inner Filter
}
```

**职责**：逻辑组合过滤器，支持与、或、非三种逻辑运算，可组合构建复杂的过滤条件。

- `AndFilter`：所有子过滤器都匹配时才匹配
- `OrFilter`：任意子过滤器匹配时即匹配
- `NotFilter`：子过滤器不匹配时才匹配

### 2.7 subscriber（内部结构体）

```go
type subscriber struct {
    ID       string
    Handler  HandlerFunc
    Priority int
    Filter   Filter
}
```

**职责**：内部订阅者结构体，封装订阅者的完整信息。

| 字段 | 说明 |
|------|------|
| `ID` | 订阅者唯一标识，可由调用方指定或自动生成 |
| `Handler` | 订阅者处理函数 |
| `Priority` | 优先级权重，数值越大优先级越高 |
| `Filter` | 事件过滤器，为 `nil` 时匹配所有事件 |

### 2.8 SubscribeConfig

```go
type SubscribeConfig struct {
    ID       string
    Priority int
    Filter   Filter
}
```

**职责**：订阅配置结构体，在调用 `Subscribe()` 时传入，用于配置订阅者的属性。

| 字段 | 说明 |
|------|------|
| `ID` | 订阅者 ID，为空时自动生成格式为 "sub-{n}" 的 ID |
| `Priority` | 优先级权重，默认为 0 |
| `Filter` | 事件过滤器，为 `nil` 时匹配所有事件 |

### 2.9 EventBus

```go
type EventBus struct {
    // ... 内部字段省略
}
```

**职责**：事件总线核心管理器，负责：
- 维护事件类型到订阅者列表的映射
- 管理订阅者的注册与注销
- 按优先级排序匹配的订阅者
- 执行同步或异步事件分发
- 处理订阅者 panic 恢复
- 管理异步分发的协程等待组

## 3. 事件完整处理链路

事件从发布到订阅者处理完成的完整流程：

```
                              ┌──────────────┐
                              │  发布事件    │
                              │ PublishSync/ │
                              │ PublishAsync │
                              └──────┬───────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  按事件类型查找    │
                          │  订阅者列表        │
                          └──────────┬─────────┘
                                     │
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                无订阅者，直接返回      有订阅者，进入过滤阶段
                         │                       │
                         │                       ▼
                         │            ┌────────────────────┐
                         │            │  逐个调用 Filter   │
                         │            │  Match(event)      │
                         │            └──────────┬─────────┘
                         │                       │
                         │          ┌────────────┴────────────┐
                         │          │                         │
                         │          ▼                         ▼
                         │  Filter 返回 false          Filter 返回 true
                         │  跳过该订阅者              加入匹配列表
                         │          │                         │
                         │          └────────────┬────────────┘
                         │                       │
                         │                       ▼
                         │            ┌────────────────────┐
                         │            │  按 Priority 降序  │
                         │            │  排序匹配的订阅者  │
                         │            └──────────┬─────────┘
                         │                       │
          ┌──────────────┴───────────────────────┘
          │
          ▼
┌─────────────────────────────────────────────────────┐
│              分发模式选择                           │
└─────────┬───────────────────────────────────────────┘
          │
          ├─────────────── 同步分发 ────────────────┐
          │                                          │
          ▼                                          ▼
┌────────────────────┐                    ┌────────────────────┐
│  逐个调用 Handler  │                    │  启动单个后台       │
│  （按优先级顺序）  │                    │  goroutine          │
└──────────┬─────────┘                    └──────────┬─────────┘
           │                                          │
           ▼                                          ▼
┌────────────────────┐                    ┌────────────────────┐
│  Panic 恢复包装    │                    │  按优先级顺序       │
│  callHandler()     │                    │  逐个调用 Handler   │
└──────────┬─────────┘                    └──────────┬─────────┘
           │                                          │
           ▼                                          ▼
┌────────────────────┐                    ┌────────────────────┐
│  检查返回 error    │                    │  Panic 恢复包装    │
└──────────┬─────────┘                    │  callHandler()     │
           │                              └──────────┬─────────┘
           ├───────────┐                             │
           │           │                             ▼
           ▼           ▼                   ┌────────────────────┐
      ErrInterrupt   其他 error            │  检查返回 error    │
           │           │                   └──────────┬─────────┘
           ▼           ▼                              │
┌────────────────────┐│                 ┌─────────────┴─────────────┐
│  中断后续调用      ││                 │                           │
│  返回 ErrInterrupt ││                 ▼                           ▼
└────────────────────┘│           ErrInterrupt          其他 error / nil
           │           │                 │                           │
           │  继续调用后续订阅者         ▼                           │
           │  收集第一个非 nil error ┌────────────────────┐         │
           ▼                       │  中断后续调用      │         │
┌────────────────────┐             │  后台 goroutine    │         │
│  返回第一个 error  │             │  直接返回          │         │
│  （或 nil）        │             └────────────────────┘         │
└────────────────────┘                                           │
                                                   继续调用后续订阅者
                                                                 │
                                                                 ▼
                                                   ┌────────────────────┐
                                                   │  后台 goroutine    │
                                                   │  处理完毕          │
                                                   └────────────────────┘
```

### 3.1 链路阶段说明

1. **发布阶段**：调用方通过 `PublishSync()` 或 `PublishAsync()` 发布事件
   - 必须指定 `Event.Type` 字段用于路由
   - `Payload` 和 `Attributes` 为可选字段

2. **订阅者查找阶段**：
   - 按 `Event.Type` 从 `subscribers` map 中查找对应的订阅者列表
   - 无订阅者时直接返回，不产生任何错误

3. **过滤阶段**：
   - 遍历该事件类型的所有订阅者
   - 对每个订阅者，若 `Filter` 不为 `nil`，调用 `Filter.Match(event)`
   - 匹配成功的订阅者加入匹配列表
   - 支持 `AndFilter`、`OrFilter`、`NotFilter` 进行复杂逻辑组合

4. **优先级排序阶段**：
   - 使用 `sort.SliceStable` 对匹配列表按 `Priority` 降序排序
   - 数值越大优先级越高，越先被调用
   - 相同优先级保持注册顺序（稳定排序）

5. **分发阶段**：
   - **同步分发（PublishSync）**：
     - 在当前协程中按优先级顺序逐个调用订阅者处理函数
     - 每个调用通过 `callHandler()` 包装，自动捕获 panic
     - 若订阅者返回 `ErrInterrupt`，立即中断后续调用并返回 `ErrInterrupt`
     - 若订阅者返回非 `nil error`，继续调用后续订阅者，但记录第一个 error
     - 所有订阅者处理完成后返回第一个 error（或 nil）
   - **异步分发（PublishAsync）**：
     - 启动单个后台 goroutine，发布者立即返回
     - 在后台 goroutine 中按优先级顺序逐个调用订阅者处理函数（与同步模式相同）
     - 每个调用通过 `callHandler()` 包装，自动捕获 panic
     - 若订阅者返回 `ErrInterrupt`，后台 goroutine 立即停止调用后续低优先级订阅者并退出
     - 若订阅者返回非 `nil error`（非 `ErrInterrupt`），继续调用后续订阅者
     - 可通过 `Wait()` 等待所有异步分发完成
     - 异步模式下订阅者返回的 error 不会传递给发布者（因为发布者已提前返回）

### 3.2 中断机制说明

中断机制在**同步分发和异步分发**模式下均生效：

- 订阅者返回 `ErrInterrupt` 特殊错误值时，事件总线立即停止调用后续低优先级订阅者
- 已调用的高优先级订阅者不受影响
- **同步模式**：`PublishSync()` 返回 `ErrInterrupt`
- **异步模式**：后台 goroutine 检测到 `ErrInterrupt` 后停止调用剩余订阅者，发布者不会收到此错误（已提前返回）

#### 3.2.1 异步分发优先级与中断的设计策略

异步分发的核心设计原则是：**在保证发布者非阻塞的前提下，维持与同步模式完全一致的优先级顺序和中断语义**。

具体实现策略：

1. **单 goroutine 顺序执行**：`PublishAsync` 启动一个后台 goroutine，在该 goroutine 内部按优先级从高到低逐个调用订阅者。不为每个订阅者单独启动 goroutine，因为并发执行会导致优先级顺序丢失和中断机制失效。

2. **中断传播**：每个订阅者执行完毕后，后台 goroutine 检查返回值。若返回 `ErrInterrupt`，立即退出循环，跳过所有剩余低优先级订阅者。

3. **Panic 隔离**：每个订阅者通过 `callHandler()` 包装执行。即使某个订阅者 panic，也不会影响后续订阅者的调用（panic 被恢复为 error，且该 error 不是 `ErrInterrupt`）。

4. **等待机制**：每次 `PublishAsync` 调用对应一个 `sync.WaitGroup` 计数器。调用 `Wait()` 可阻塞等待所有异步分发（包括其中所有订阅者）处理完毕。

**为什么不为每个订阅者启动独立 goroutine？**

若为每个匹配的订阅者并发启动独立 goroutine，会导致以下问题：
- **优先级丢失**：所有 goroutine 并发执行，操作系统调度不可控，高优先级订阅者可能晚于低优先级完成
- **中断失效**：并发执行时无法在某个订阅者返回 `ErrInterrupt` 后阻止低优先级订阅者的执行，因为低优先级 goroutine 可能已经开始运行
- **语义不一致**：同步和异步模式的优先级与中断行为会产生差异，增加使用者的认知负担

## 4. 核心算法与策略

### 4.1 等值比较算法

`EqualsFilter.Match()` 使用 `reflect.DeepEqual` 进行深度比较：

```go
func (f *EqualsFilter) Match(event Event) bool {
    if event.Attributes == nil {
        return false
    }
    val, ok := event.Attributes[f.Key]
    if !ok {
        return false
    }
    return reflect.DeepEqual(val, f.Value)
}
```

支持比较任意类型的值，包括基本类型、结构体、切片、map 等。

### 4.2 范围比较算法

`RangeFilter.Match()` 先将属性值转换为 `float64`，再进行范围比较：

```go
func (f *RangeFilter) Match(event Event) bool {
    // ... 获取属性值并转换为 float64
    if f.HasMin && floatVal < f.Min {
        return false
    }
    if f.HasMax && floatVal > f.Max {
        return false
    }
    return true
}
```

### 4.3 优先级排序算法

使用 `sort.SliceStable` 进行稳定排序，确保相同优先级的订阅者保持注册顺序：

```go
sort.SliceStable(matched, func(i, j int) bool {
    return matched[i].Priority > matched[j].Priority
})
```

### 4.4 Panic 恢复机制

`callHandler()` 函数通过 defer+recover 捕获处理函数中的 panic：

```go
func callHandler(handler HandlerFunc, event Event) (retErr error) {
    defer func() {
        if r := recover(); r != nil {
            retErr = fmt.Errorf("handler panic: %v", r)
        }
    }()
    return handler(event)
}
```

### 4.5 异步分发执行与中断策略

`PublishAsync` 的核心目标是在保证发布者非阻塞的同时，维持与同步模式一致的优先级顺序和中断语义。为此采用**单 goroutine 顺序执行**策略，而非为每个订阅者独立启动 goroutine。

核心实现：

```go
func (bus *EventBus) PublishAsync(event Event) {
    matched := bus.getMatchedSubscribers(event.Type, event)
    if len(matched) == 0 {
        return
    }

    bus.asyncWg.Add(1)
    go func(subs []*subscriber, e Event) {
        defer bus.asyncWg.Done()
        for _, sub := range subs {
            err := callHandler(sub.Handler, e)
            if err != nil && errors.Is(err, ErrInterrupt) {
                return
            }
        }
    }(matched, event)
}
```

#### 4.5.1 优先级保证机制

`getMatchedSubscribers()` 在分发前已完成按 `Priority` 降序排序（见 4.3 节）。后台 goroutine 使用 `for range` 遍历该已排序切片，由于遍历在单个 goroutine 内顺序执行，天然保证了高优先级订阅者的 Handler 先于低优先级开始执行和返回。即使在多核 CPU 上，也不会出现低优先级订阅者先执行的竞态。

与"每订阅者独立 goroutine"方案的对比：
- **独立 goroutine 方案（错误）**：调度顺序由 Go runtime 决定，`Priority=100` 的 Handler 可能晚于 `Priority=10` 的 Handler 获得执行时间片，优先级仅为注释，无实际语义。
- **单 goroutine 方案（实际实现）**：调度顺序由代码的 `for` 循环决定，`Priority=100` 必然在 `Priority=10` 之前开始执行，优先级为强语义保证。

#### 4.5.2 中断保证机制

中断通过 `ErrInterrupt` + 每次循环后的 error 检查实现：

1. 每个订阅者执行完毕后，判断 `errors.Is(err, ErrInterrupt)`：
   - 若为 `true`：立即 `return` 跳出循环，剩余未遍历的低优先级订阅者均不会被调用
   - 若为 `false`（`nil` 或其他 error）：继续下一次循环

2. 由于在单个 goroutine 中顺序执行，中断判断是一个完全同步的决策点，不存在以下竞态：
   - 中断触发时低优先级订阅者已在另一个 goroutine 中开始运行
   - 中断信号传递存在延迟导致部分低优先级执行

#### 4.5.3 错误语义差异

同步与异步模式在错误传递上存在差异：

| 模式 | `ErrInterrupt` 中断行为 | 一般错误（非中断） | 返回值 |
|------|------------------------|-------------------|--------|
| PublishSync | 中断后续调用 | 继续调用，记录第一个 | 返回第一个 error 或 `ErrInterrupt` |
| PublishAsync | 中断后续调用 | 继续调用，不记录 | 无返回值，所有 error 不向外传递 |

原因：异步模式下发布者已提前返回，无渠道接收错误。若业务需要感知异步分发的错误或中断结果，应在订阅者 Handler 内部将错误写入独立的错误收集 channel、日志或指标系统。

### 4.6 ID 生成算法

订阅者 ID 采用递增序列生成，格式为 "sub-{n}"：

```go
func (bus *EventBus) generateID() string {
    bus.idMu.Lock()
    defer bus.idMu.Unlock()
    bus.nextID++
    return fmt.Sprintf("sub-%d", bus.nextID)
}
```

## 5. API 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/eventbus"
)

func main() {
    // 1. 创建事件总线
    bus := eventbus.NewEventBus()

    // 2. 注册订阅者
    bus.Subscribe("user.created", func(event eventbus.Event) error {
        userID := event.Attributes["user_id"].(int)
        fmt.Printf("欢迎新用户: %d\n", userID)
        return nil
    }, eventbus.SubscribeConfig{})

    // 3. 同步发布事件
    event := eventbus.Event{
        Type:       "user.created",
        Attributes: map[string]interface{}{"user_id": 123},
    }
    err := bus.PublishSync(event)
    if err != nil {
        fmt.Printf("发布失败: %v\n", err)
    }
}
```

### 5.2 异步分发

异步分发模式下，发布者立即返回，订阅者在后台 goroutine 中按优先级顺序执行，中断机制同样生效：

```go
bus := eventbus.NewEventBus()

bus.Subscribe("email.send", func(event eventbus.Event) error {
    // 模拟发送邮件的耗时操作
    time.Sleep(100 * time.Millisecond)
    fmt.Println("邮件已发送")
    return nil
}, eventbus.SubscribeConfig{})

// 异步发布，立即返回
bus.PublishAsync(eventbus.Event{Type: "email.send"})
fmt.Println("发布完成，无需等待邮件发送")

// 可选：等待所有异步处理完成
bus.Wait()
fmt.Println("所有异步处理已完成")
```

异步模式下的优先级与中断示例：

```go
bus := eventbus.NewEventBus()

var order []int
var mu sync.Mutex

// 高优先级：风控检查
bus.Subscribe("order.create", func(event eventbus.Event) error {
    mu.Lock()
    order = append(order, 100)
    mu.Unlock()
    amount := event.Attributes["amount"].(float64)
    if amount > 100000 {
        fmt.Println("大额订单风控拦截")
        return eventbus.ErrInterrupt // 异步模式下同样触发中断
    }
    return nil
}, eventbus.SubscribeConfig{Priority: 100})

// 中优先级：扣减库存
bus.Subscribe("order.create", func(event eventbus.Event) error {
    mu.Lock()
    order = append(order, 50)
    mu.Unlock()
    return nil
}, eventbus.SubscribeConfig{Priority: 50})

// 低优先级：创建订单记录
bus.Subscribe("order.create", func(event eventbus.Event) error {
    mu.Lock()
    order = append(order, 10)
    mu.Unlock()
    return nil
}, eventbus.SubscribeConfig{Priority: 10})

// 触发风控拦截，order 结果为 [100]，50 和 10 不会执行
bus.PublishAsync(eventbus.Event{
    Type:       "order.create",
    Attributes: map[string]interface{}{"amount": 200000.0},
})
bus.Wait()
```

### 5.3 事件过滤

```go
bus := eventbus.NewEventBus()

// 只处理管理员登录事件
bus.Subscribe("user.login", func(event eventbus.Event) error {
    fmt.Println("管理员登录，记录审计日志")
    return nil
}, eventbus.SubscribeConfig{
    Filter: &eventbus.EqualsFilter{
        Key:   "role",
        Value: "admin",
    },
})

// 只处理金额大于 1000 的订单
bus.Subscribe("order.paid", func(event eventbus.Event) error {
    fmt.Println("大额订单，需要审批")
    return nil
}, eventbus.SubscribeConfig{
    Filter: &eventbus.RangeFilter{
        Key:    "amount",
        Min:    1000,
        HasMin: true,
    },
})

// 组合过滤：美国或加拿大的 18-35 岁非封禁用户
bus.Subscribe("promotion.send", func(event eventbus.Event) error {
    fmt.Println("发送定向推广")
    return nil
}, eventbus.SubscribeConfig{
    Filter: &eventbus.AndFilter{
        Filters: []eventbus.Filter{
            &eventbus.OrFilter{
                Filters: []eventbus.Filter{
                    &eventbus.EqualsFilter{Key: "country", Value: "US"},
                    &eventbus.EqualsFilter{Key: "country", Value: "CA"},
                },
            },
            &eventbus.RangeFilter{Key: "age", Min: 18, Max: 35, HasMin: true, HasMax: true},
            &eventbus.NotFilter{
                Inner: &eventbus.EqualsFilter{Key: "status", Value: "banned"},
            },
        },
    },
})
```

### 5.4 优先级与中断

```go
bus := eventbus.NewEventBus()

// 高优先级：黑名单检查
bus.Subscribe("comment.create", func(event eventbus.Event) error {
    userID := event.Attributes["user_id"].(int)
    if isInBlacklist(userID) {
        fmt.Println("用户在黑名单中，禁止评论")
        return eventbus.ErrInterrupt // 中断后续处理
    }
    return nil
}, eventbus.SubscribeConfig{Priority: 100})

// 中优先级：敏感词过滤
bus.Subscribe("comment.create", func(event eventbus.Event) error {
    content := event.Payload.(string)
    if containsSensitiveWords(content) {
        fmt.Println("评论包含敏感词，拒绝发布")
        return eventbus.ErrInterrupt
    }
    return nil
}, eventbus.SubscribeConfig{Priority: 50})

// 低优先级：保存评论
bus.Subscribe("comment.create", func(event eventbus.Event) error {
    content := event.Payload.(string)
    saveComment(content)
    fmt.Println("评论已发布")
    return nil
}, eventbus.SubscribeConfig{Priority: 10})

// 发布评论事件
// 如果用户在黑名单中，只会调用高优先级订阅者，中、低优先级不会被调用
// 如果评论包含敏感词，高、中优先级会被调用，低优先级不会被调用
// 都通过时，三个订阅者按优先级顺序全部被调用
err := bus.PublishSync(eventbus.Event{
    Type:       "comment.create",
    Payload:    "这是一条评论",
    Attributes: map[string]interface{}{"user_id": 123},
})

if errors.Is(err, eventbus.ErrInterrupt) {
    fmt.Println("评论发布被中断")
}
```

### 5.5 动态订阅与取消

```go
bus := eventbus.NewEventBus()

// 注册订阅者，获取订阅 ID
id, err := bus.Subscribe("event.type", handler, eventbus.SubscribeConfig{
    ID: "my-subscriber", // 可自定义 ID
})
if err != nil {
    if errors.Is(err, eventbus.ErrSubscriberExists) {
        fmt.Println("订阅者 ID 已存在")
    }
}

// 检查订阅者是否存在
if bus.HasSubscriber("event.type", id) {
    fmt.Println("订阅者存在")
}

// 取消订阅
err = bus.Unsubscribe("event.type", id)
if err != nil {
    if errors.Is(err, eventbus.ErrSubscriberNotFound) {
        fmt.Println("订阅者不存在")
    }
}

// 获取订阅者数量
count := bus.SubscriberCount("event.type")
total := bus.TotalSubscriberCount()

// 获取所有事件类型
types := bus.EventTypes()
```

### 5.6 并发场景

```go
bus := eventbus.NewEventBus()

// 多个 goroutine 同时订阅
for i := 0; i < 100; i++ {
    go func(i int) {
        bus.Subscribe(fmt.Sprintf("event.%d", i%10), handler, eventbus.SubscribeConfig{})
    }(i)
}

// 多个 goroutine 同时发布
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        bus.PublishSync(eventbus.Event{Type: "event.0"})
    }()
}
wg.Wait()
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrSubscriberNotFound` | 取消订阅时指定的事件类型或订阅者 ID 不存在 |
| `ErrSubscriberExists` | 注册订阅者时指定的 ID 已存在于该事件类型 |
| `ErrInterrupt` | 订阅者返回此错误以中断后续低优先级订阅者的调用（同步模式和异步模式均生效） |
| `errors.New("handler panic: ...")` | 订阅者处理函数发生 panic 时自动包装的错误 |
| `errors.New("handler cannot be nil")` | 调用 Subscribe() 时传入 nil handler |

## 7. 线程安全说明

EventBus 所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：

- 订阅者映射 `subscribers` 通过 `sync.RWMutex` 保护
- 读操作（查询订阅者数量、检查订阅者存在等）使用读锁
- 写操作（订阅、取消订阅）使用写锁
- ID 生成通过独立的 `sync.Mutex` 保护
- 异步分发的协程等待通过 `sync.WaitGroup` 管理

## 8. 模块对比：EventBus vs PubSub

| 特性 | EventBus | PubSub |
|------|----------|--------|
| 分发模式 | 同步/异步 | 异步（channel 推送） |
| 过滤机制 | 基于事件属性的灵活过滤 | 基于 Topic 模式匹配 |
| 优先级 | 支持 | 不支持 |
| 中断机制 | 支持（同步/异步模式） | 不支持 |
| 消息持久化 | 不支持 | 支持（死信队列、持久订阅） |
| ACK 机制 | 不支持 | 支持 |
| 重试机制 | 不支持 | 支持（指数退避） |
| 适用场景 | 进程内事件驱动、领域事件 | 消息队列、跨组件通信 |

**使用建议**：
- 进程内模块解耦、需要同步处理、优先级控制 → 使用 EventBus
- 跨组件/跨服务通信、需要消息可靠投递、消费解耦 → 使用 PubSub
