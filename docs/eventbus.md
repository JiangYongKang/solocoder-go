# EventBus 事件总线模块需求文档

## 1. 模块概述

EventBus 是一个基于内存的事件发布-订阅路由模块，提供灵活的事件分发与处理能力。它支持按事件类型注册订阅者、同步/异步两种分发模式、基于事件属性的过滤机制、订阅者优先级排序以及中断分发等高级特性，适用于系统内部模块解耦、事件驱动架构、领域事件处理等场景。

### 主要特性

- **事件类型注册**：支持按事件类型注册多个订阅者处理函数，支持动态添加和移除订阅者
- **同步与异步分发**：同步模式下发布者阻塞等待所有订阅者处理完成；异步模式下发布者立即返回，订阅者在后台协程中并发处理
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
│  逐个调用 Handler  │                    │  为每个匹配订阅者   │
│  （按优先级顺序）  │                    │  启动 goroutine     │
└──────────┬─────────┘                    └──────────┬─────────┘
           │                                          │
           ▼                                          ▼
┌────────────────────┐                    ┌────────────────────┐
│  Panic 恢复包装    │                    │  Panic 恢复包装    │
│  callHandler()     │                    │  后台异步执行      │
└──────────┬─────────┘                    └──────────┬─────────┘
           │                                          │
           ▼                                          ▼
┌────────────────────┐                    ┌────────────────────┐
│  检查返回 error    │                    │  发布者立即返回     │
└──────────┬─────────┘                    │  订阅者后台执行     │
           │                              └────────────────────┘
           ├───────────┐
           │           │
           ▼           ▼
      ErrInterrupt   其他 error
           │           │
           ▼           ▼
┌────────────────────┐│
│  中断后续调用      ││
│  返回 ErrInterrupt ││
└────────────────────┘│
           │           ▼
           │  继续调用后续订阅者
           │  收集第一个非 nil error
           ▼
┌────────────────────┐
│  返回第一个 error  │
│  （或 nil）        │
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
     - 为每个匹配的订阅者启动一个独立的 goroutine
     - 每个 goroutine 中通过 `callHandler()` 包装，自动捕获 panic
     - 发布者立即返回，不等待订阅者处理完成
     - 可通过 `Wait()` 等待所有异步分发的订阅者处理完成
     - 异步模式下忽略订阅者返回的 error（包括 `ErrInterrupt`）

### 3.2 中断机制说明

中断机制仅在**同步分发**模式下生效：

- 订阅者返回 `ErrInterrupt` 特殊错误值时，事件总线立即停止调用后续低优先级订阅者
- 已调用的高优先级订阅者不受影响
- `PublishSync()` 返回 `ErrInterrupt`
- 异步分发模式下，`ErrInterrupt` 不触发中断，所有匹配的订阅者都会被并发调用

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

### 4.5 ID 生成算法

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
| `ErrInterrupt` | 订阅者返回此错误以中断后续低优先级订阅者的调用（仅同步模式） |
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
| 中断机制 | 支持（同步模式） | 不支持 |
| 消息持久化 | 不支持 | 支持（死信队列、持久订阅） |
| ACK 机制 | 不支持 | 支持 |
| 重试机制 | 不支持 | 支持（指数退避） |
| 适用场景 | 进程内事件驱动、领域事件 | 消息队列、跨组件通信 |

**使用建议**：
- 进程内模块解耦、需要同步处理、优先级控制 → 使用 EventBus
- 跨组件/跨服务通信、需要消息可靠投递、消费解耦 → 使用 PubSub
