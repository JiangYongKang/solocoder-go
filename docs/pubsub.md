# Pub/Sub 消息代理模块需求文档

## 1. 模块概述

Pub/Sub 是一个基于内存的发布-订阅消息代理模块，提供完整的消息路由、持久化订阅、消息确认与重试、背压控制等企业级消息队列特性。它支持按主题发布订阅、通配符模式匹配、消费者离线消息缓存、消息超时重推、死信队列等功能，适用于系统内部组件解耦、异步消息处理、事件驱动架构等场景。

### 主要特性

- **主题订阅**：支持消费者按主题订阅消息，生产者向指定主题发布消息，所有订阅该主题的消费者都能收到消息
- **通配符匹配**：主题名支持通配符订阅，`*` 匹配单层主题段，`#` 匹配多层主题段
- **持久化订阅**：支持创建持久化订阅，消费者离线时消息缓存在代理端，重新上线后继续投递
- **消息确认机制**：消费者收到消息后需显式确认（Ack），未确认消息在超时后自动重推
- **死信队列**：重推次数达到上限的消息自动移入死信队列，避免无限重试
- **背压控制**：每个消费者有最大未确认消息数限制，达到上限时暂停投递，确认后恢复
- **并发安全**：所有公共方法均为并发安全，可在多个协程中同时调用

## 2. 核心结构体

### 2.1 Config

```go
type Config struct {
    AckTimeout     time.Duration
    MaxRetry       int
    MaxUnacked     int
    ConsumerBuffer int
}
```

**职责**：Broker 配置结构体，定义消息代理的运行参数。

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `AckTimeout` | 消息确认超时时间，超过该时间未确认的消息将被重推 | 30秒 |
| `MaxRetry` | 最大重试次数，超过该次数的消息移入死信队列 | 3次 |
| `MaxUnacked` | 每个消费者最大未确认消息数（背压阈值） | 100 |
| `ConsumerBuffer` | 消费者通道缓冲区大小 | 1024 |

### 2.2 Message

```go
type Message struct {
    ID         string
    Topic      string
    Payload    interface{}
    Timestamp  time.Time
    RetryCount int
}
```

**职责**：表示一条消息单元，封装消息的唯一标识、主题、负载数据、时间戳和重试次数。

| 字段 | 说明 |
|------|------|
| `ID` | 消息唯一标识，格式为 `msg-{timestamp}-{seq}`，由 Broker 自动生成 |
| `Topic` | 消息主题，使用 `.` 分隔层级，如 `sports.football.worldcup` |
| `Payload` | 消息负载数据，可存储任意类型的业务数据 |
| `Timestamp` | 消息发布时间戳 |
| `RetryCount` | 消息重试次数，首次投递为 0，每次重推递增 |

### 2.3 MessageStatus

```go
type MessageStatus int

const (
    MessageStatusPending MessageStatus = iota
    MessageStatusDelivered
    MessageStatusAcked
    MessageStatusDead
)
```

**职责**：消息状态枚举，表示消息在处理流程中的不同阶段。

| 状态 | 说明 |
|------|------|
| `MessageStatusPending` | 待投递状态（持久化缓存中） |
| `MessageStatusDelivered` | 已投递，等待消费者确认 |
| `MessageStatusAcked` | 已确认，处理完成 |
| `MessageStatusDead` | 已死亡，移入死信队列 |

### 2.4 Consumer

```go
type Consumer struct {
    unackedCount  int64
    ID            string
    ch            chan *Message
    connected     bool
    disconnected  bool
    closeOnce     sync.Once
    maxUnacked    int
    pending       map[string]*pendingMessage
    pendingList   *list.List
    durableBuffer []*Message
}
```

**职责**：消费者结构体，管理单个消费者的连接状态、消息通道、未确认消息和持久化缓存。

| 字段 | 说明 |
|------|------|
| `ID` | 消费者唯一标识，由调用方指定 |
| `ch` | 消费者消息通道，Broker 通过此通道向消费者投递消息 |
| `connected` | 消费者是否在线 |
| `disconnected` | 消费者是否已被移除（不可重连） |
| `maxUnacked` | 该消费者的最大未确认消息数（背压阈值） |
| `pending` | 未确认消息映射，key 为消息 ID |
| `pendingList` | 未确认消息链表，按投递时间排序，用于超时检测 |
| `durableBuffer` | 持久化订阅的消息缓存，消费者离线或背压时存储消息 |

### 2.5 Subscription

```go
type Subscription struct {
    ConsumerID  string
    TopicFilter string
    Durable     bool
}
```

**职责**：订阅关系结构体，表示某个消费者对某个主题模式的订阅。

| 字段 | 说明 |
|------|------|
| `ConsumerID` | 订阅的消费者 ID |
| `TopicFilter` | 主题过滤模式，支持通配符 `*` 和 `#` |
| `Durable` | 是否为持久化订阅，离线时是否缓存消息 |

### 2.6 topicNode

```go
type topicNode struct {
    children    map[string]*topicNode
    subscribers map[string]*Subscription
    wildcardOne map[string]*Subscription
    wildcardAny map[string]*Subscription
}
```

**职责**：主题树节点，用于实现高效的主题模式匹配。采用字典树（Trie）结构存储订阅关系。

| 字段 | 说明 |
|------|------|
| `children` | 子节点映射，key 为主题段名称 |
| `subscribers` | 精确匹配该节点的订阅者 |
| `wildcardOne` | 单层通配符 `*` 订阅者 |
| `wildcardAny` | 多层通配符 `#` 订阅者 |

### 2.7 Broker

```go
type Broker struct {
    nextMsgID     uint64
    cfg           Config
    mu            sync.RWMutex
    consumers     map[string]*Consumer
    subscriptions map[string][]*Subscription
    topicTree     *topicNode
    deadLetters   []*Message
    running       bool
    stopCh        chan struct{}
    wg            sync.WaitGroup
    ackTimerCh    chan string
    retryTimerCh  chan string
}
```

**职责**：消息代理核心结构体，负责管理所有消费者、订阅关系、主题路由、消息发布与投递、超时检测和死信队列。

核心职责包括：
- 管理消费者的创建、连接、断开和移除
- 管理订阅关系的添加和删除
- 维护主题树实现高效的通配符匹配
- 接收生产者发布的消息并路由到匹配的消费者
- 检测消息确认超时并触发重推
- 管理死信队列
- 实现背压控制

## 3. 消息完整处理链路

消息从发布到确认的完整流程：

```
                              ┌──────────────┐
                              │  生产者发布   │
                              │   Publish    │
                              └──────┬───────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  验证主题格式      │
                          │  validateTopic()   │
                          └──────────┬─────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  生成消息 ID       │
                          │  generateMsgID()   │
                          └──────────┬─────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  查找匹配的订阅    │
                          │  findMatchingSubs()│
                          └──────────┬─────────┘
                                     │
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                无匹配订阅者，直接返回      有匹配订阅者，进入投递阶段
                         │                       │
                         │                       ▼
                         │            ┌────────────────────┐
                         │            │  遍历每个订阅者    │
                         │            │  deliverToConsumer │
                         │            └──────────┬─────────┘
                         │                       │
                         │          ┌────────────┴────────────┐
                         │          │                         │
                         │          ▼                         ▼
                         │  消费者不在线？              消费者在线？
                         │          │                         │
                         │          ▼                         ▼
                         │  是否持久订阅？              背压是否已满？
                         │          │                         │
                         │          ├─ 是 → 加入 durableBuffer
                         │          │                         │
                         │          └─ 否 → 丢弃消息          ▼
                         │                      背压已满？ ── 是 ── 持久订阅？ ── 是 → 加入 durableBuffer
                         │                                  │                         │
                         │                                  ▼                         └─ 否 → 丢弃
                         │                              背压未满
                         │                                  │
                         │                                  ▼
                         │                      ┌────────────────────────┐
                         │                      │  发送到消费者 channel  │
                         │                      │  创建 pendingMessage   │
                         │                      │  加入 pending 列表     │
                         │                      │  unackedCount++        │
                         │                      └──────────┬─────────────┘
                         │                                 │
                         └─────────────────────────────────┘
                                     │
                                     ▼
                          ┌────────────────────┐
                          │  消费者接收消息    │
                          │  <-consumer.ch     │
                          └──────────┬─────────┘
                                     │
                         ┌───────────┴───────────┐
                         │                       │
                         ▼                       ▼
                处理成功，调用 Ack       处理失败，调用 Nack
                         │                       │
                         ▼                       ▼
              ┌────────────────────┐  ┌────────────────────┐
              │  从 pending 移除    │  │  RetryCount++      │
              │  unackedCount--     │  │  检查是否超过      │
              │  flushDurableBuffer │  │  MaxRetry          │
              └──────────┬─────────┘  └──────────┬─────────┘
                         │                       │
                         │                       ├─ 是 → 移入死信队列
                         │                       │
                         │                       └─ 否 → 检查背压
                         │                                  │
                         │                                  ▼
                         │                              背压未满？ ── 是 → 重新投递
                         │                                  │
                         │                                  └─ 否 → 加入 durableBuffer
                         │
                         ▼
              ┌────────────────────────┐
              │  检查 durableBuffer     │
              │  有可投递消息则继续投递  │
              └────────────────────────┘
```

### 3.1 链路阶段说明

#### 1. 发布阶段（Publish）

- 验证主题格式：主题不能为空，不能包含连续的 `.`
- 生成唯一消息 ID：格式为 `msg-{timestamp}-{sequence}`
- 查找匹配的订阅者：通过主题树 `findMatchingSubs()` 查找所有匹配的订阅
- 遍历每个匹配的订阅者，调用 `deliverToConsumer()` 进行投递

#### 2. 投递阶段（deliverToConsumer）

- 检查消费者是否在线：
  - 不在线且是持久化订阅：加入 `durableBuffer` 缓存
  - 不在线且非持久化：丢弃消息
- 检查背压（未确认消息数是否达到上限）：
  - 背压已满且是持久化订阅：加入 `durableBuffer` 缓存
  - 背压已满且非持久化：丢弃消息
- 尝试发送到消费者 channel：
  - 发送成功：创建 `pendingMessage`，加入 `pending` 映射和 `pendingList` 链表，`unackedCount++`
  - 发送失败（channel 满）且是持久化订阅：加入 `durableBuffer` 缓存

#### 3. 确认阶段（Ack）

- 从 `pending` 映射和 `pendingList` 中移除消息
- `unackedCount--`
- 调用 `flushDurableBuffer()` 尝试投递缓存中的消息

#### 4. 否定确认阶段（Nack）

- `RetryCount++`
- 检查是否超过 `MaxRetry`：
  - 超过：移入死信队列，从 `pending` 移除，`unackedCount--`
  - 未超过：检查背压，未满则重新投递，已满则加入 `durableBuffer`

#### 5. 超时检测阶段（ProcessTimeouts）

- 后台定时任务（默认 100ms 检查一次）
- 遍历每个消费者的 `pendingList`
- 检查消息的 `deliverAt` 是否已过期
- 过期消息调用 `redeliverOrDeadLetter()` 进行重推或移入死信队列

#### 6. 缓存刷新阶段（flushDurableBuffer）

- 当消费者确认消息或重连时调用
- 遍历 `durableBuffer` 中的消息
- 背压未满时尝试重新投递
- 背压已满或投递失败的消息保留在缓存中

### 3.2 持久化订阅流程

```
消费者在线 → 订阅（Durable=true） → 正常接收消息
    │
    ▼
消费者离线（DisconnectConsumer）
    │
    ├─ 未确认消息移至 durableBuffer
    │
    ▼
生产者发布消息 → 检查消费者在线状态
    │
    └─ 不在线但有持久订阅 → 加入 durableBuffer
    │
    ▼
消费者重连（ReconnectConsumer）
    │
    ├─ 标记为在线
    └─ 调用 flushDurableBuffer() 投递所有缓存消息
```

### 3.3 背压控制流程

```
消费者 maxUnacked = N
    │
    ▼
投递消息 → unackedCount++
    │
    ├─ unackedCount < N → 继续投递
    │
    └─ unackedCount >= N → 暂停投递，新消息进入 durableBuffer
    │
    ▼
消费者 Ack 消息 → unackedCount--
    │
    └─ unackedCount < N → 调用 flushDurableBuffer() 恢复投递
```

## 4. 核心算法与策略

### 4.1 通配符匹配算法

采用递归回溯算法实现主题模式匹配：

```go
func matchParts(filterParts, topicParts []string, fi, ti int) bool {
    if fi == len(filterParts) {
        return ti == len(topicParts)
    }
    fp := filterParts[fi]
    
    if fp == "#" {
        // # 匹配 0 或多层，尝试所有可能
        for i := ti; i <= len(topicParts); i++ {
            if matchParts(filterParts, topicParts, fi+1, i) {
                return true
            }
        }
        return false
    }
    if ti >= len(topicParts) {
        return false
    }
    if fp == "*" {
        // * 匹配恰好一层
        return matchParts(filterParts, topicParts, fi+1, ti+1)
    }
    // 精确匹配
    return fp == topicParts[ti] && matchParts(filterParts, topicParts, fi+1, ti+1)
}
```

**匹配规则**：
- `*`：匹配恰好一个主题段
- `#`：匹配零个或多个主题段，只能出现在模式末尾
- 其他字符串：精确匹配主题段

**示例**：
| 模式 | 匹配主题 | 不匹配主题 |
|------|----------|------------|
| `sports.*` | `sports.football`, `sports.basketball` | `sports`, `sports.football.worldcup` |
| `sports.#` | `sports`, `sports.football`, `sports.football.worldcup` | `news.politics` |
| `a.*.c` | `a.b.c`, `a.x.c` | `a.b`, `a.b.c.d` |
| `#` | 所有主题 | 无 |

### 4.2 主题树路由算法

采用字典树（Trie）结构存储订阅关系，实现高效的多模式匹配：

```
主题树结构（以订阅 a.b.c, a.*.c, a.# 为例）：

          root
           │
           a
         / │ \
        /  │  \
       b   *   # (wildcardAny)
      /    │
     /     │
    c      c
    │      │
subs1    subs2
```

**查找流程**：
1. 从根节点开始，按主题段逐层遍历
2. 在每个节点收集：
   - `wildcardAny` 订阅者（`#` 匹配所有后续段）
   - `wildcardOne` 订阅者（`*` 匹配当前段）
   - 精确匹配子节点的订阅者
3. 递归遍历子节点，直到主题段遍历完成

### 4.3 消息 ID 生成算法

```go
func (b *Broker) generateMsgID() string {
    id := atomic.AddUint64(&b.nextMsgID, 1)
    return fmt.Sprintf("msg-%d-%d", time.Now().UnixNano(), id)
}
```

采用时间戳 + 原子递增序列的组合，保证全局唯一性和有序性。

### 4.4 超时检测策略

- 每条消息投递时设置 `deliverAt = now + AckTimeout`
- 后台定时任务每 100ms 遍历一次所有消费者的 `pendingList`
- `pendingList` 按投递时间排序，便于快速找到超时消息
- 超时消息触发重推或移入死信队列

## 5. API 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/pubsub"
)

func main() {
    // 1. 创建 Broker，使用默认配置
    cfg := pubsub.DefaultConfig()
    b := pubsub.NewBroker(cfg)
    defer b.Stop()

    // 2. 启动超时检测循环
    b.Start()

    // 3. 添加消费者
    ch, err := b.AddConsumer("consumer-1")
    if err != nil {
        panic(err)
    }

    // 4. 订阅主题（非持久化）
    err = b.Subscribe("consumer-1", "order.created")
    if err != nil {
        panic(err)
    }

    // 5. 启动消费者协程
    go func() {
        for msg := range ch {
            fmt.Printf("收到消息: ID=%s, Topic=%s, Payload=%v\n",
                msg.ID, msg.Topic, msg.Payload)
            
            // 处理成功，确认消息
            err := b.Ack("consumer-1", msg.ID)
            if err != nil {
                fmt.Printf("确认失败: %v\n", err)
            }
        }
    }()

    // 6. 发布消息
    msgID, err := b.Publish("order.created", map[string]interface{}{
        "order_id": 12345,
        "amount":   99.99,
    })
    if err != nil {
        panic(err)
    }
    fmt.Printf("消息已发布，ID=%s\n", msgID)

    // 等待消息处理
    time.Sleep(100 * time.Millisecond)
}
```

### 5.2 通配符订阅

```go
b := pubsub.NewBroker(pubsub.DefaultConfig())
defer b.Stop()

ch1, _ := b.AddConsumer("sports-fan")
ch2, _ := b.AddConsumer("all-events")

// 订阅所有 sports 下的单层主题
b.Subscribe("sports-fan", "sports.*")

// 订阅所有主题
b.Subscribe("all-events", "#")

// 发布不同主题的消息
b.Publish("sports.football", "足球比赛结果")
b.Publish("sports.basketball", "篮球比赛结果")
b.Publish("news.politics", "政治新闻")

// sports-fan 会收到 sports.football 和 sports.basketball
// all-events 会收到所有三条消息
```

### 5.3 持久化订阅

```go
b := pubsub.NewBroker(pubsub.DefaultConfig())
defer b.Stop()
b.Start()

ch, _ := b.AddConsumer("durable-consumer")

// 创建持久化订阅
b.SubscribeDurable("durable-consumer", "important.events", true)

// 消费者离线
b.DisconnectConsumer("durable-consumer")

// 消费者离线期间发布的消息会被缓存
b.Publish("important.events", "重要通知1")
b.Publish("important.events", "重要通知2")

// 检查待处理消息数
pending, _ := b.PendingCount("durable-consumer")
fmt.Printf("待处理消息: %d\n", pending) // 输出: 2

// 消费者重新上线，会收到缓存的消息
b.ReconnectConsumer("durable-consumer")

// 接收并确认消息
for i := 0; i < 2; i++ {
    msg := <-ch
    fmt.Printf("收到: %v\n", msg.Payload)
    b.Ack("durable-consumer", msg.ID)
}
```

### 5.4 消息确认与重试

```go
cfg := pubsub.DefaultConfig()
cfg.AckTimeout = 5 * time.Second  // 5秒未确认则重推
cfg.MaxRetry = 3                   // 最多重试3次
b := pubsub.NewBroker(cfg)
defer b.Stop()
b.Start()

ch, _ := b.AddConsumer("consumer-1")
b.Subscribe("consumer-1", "test.topic")

b.Publish("test.topic", "需要处理的数据")

msg := <-ch
fmt.Printf("首次收到，重试次数: %d\n", msg.RetryCount) // 0

// 故意不确认，等待超时重推
time.Sleep(6 * time.Second)
b.ProcessTimeouts() // 手动触发超时检测（或由 Start() 的后台循环自动处理）

msg2 := <-ch
fmt.Printf("重推收到，重试次数: %d\n", msg2.RetryCount) // 1

// 处理失败，主动 Nack
b.Nack("consumer-1", msg2.ID)

msg3 := <-ch
fmt.Printf("再次重推，重试次数: %d\n", msg3.RetryCount) // 2

// 处理成功，确认
b.Ack("consumer-1", msg3.ID)

// 检查死信队列
fmt.Printf("死信队列长度: %d\n", b.DeadLetterCount()) // 0
```

### 5.5 背压控制

```go
cfg := pubsub.DefaultConfig()
cfg.MaxUnacked = 2 // 每个消费者最多2条未确认消息
b := pubsub.NewBroker(cfg)
defer b.Stop()

ch, _ := b.AddConsumerWithOptions("slow-consumer", 2)
b.SubscribeDurable("slow-consumer", "data.stream", true)

// 发布3条消息
b.Publish("data.stream", "msg1")
b.Publish("data.stream", "msg2")
b.Publish("data.stream", "msg3")

// 消费者只能收到前2条，第3条进入缓存
msg1 := <-ch
msg2 := <-ch

unacked, _ := b.UnackedCount("slow-consumer")
fmt.Printf("未确认消息数: %d\n", unacked) // 2

pending, _ := b.PendingCount("slow-consumer")
fmt.Printf("缓存消息数: %d\n", pending) // 1

// 确认一条消息，触发第3条的投递
b.Ack("slow-consumer", msg1.ID)

msg3 := <-ch
fmt.Printf("收到第3条: %v\n", msg3.Payload) // "msg3"

// 确认剩余消息
b.Ack("slow-consumer", msg2.ID)
b.Ack("slow-consumer", msg3.ID)
```

### 5.6 死信队列

```go
cfg := pubsub.DefaultConfig()
cfg.MaxRetry = 0 // 不重试，直接入死信
b := pubsub.NewBroker(cfg)
defer b.Stop()

ch, _ := b.AddConsumer("consumer-1")
b.Subscribe("consumer-1", "test.topic")

b.Publish("test.topic", "无法处理的消息")

msg := <-ch
b.Nack("consumer-1", msg.ID)

// 检查死信队列
fmt.Printf("死信队列长度: %d\n", b.DeadLetterCount()) // 1

// 获取死信消息
deadLetters := b.GetDeadLetters()
for _, dl := range deadLetters {
    fmt.Printf("死信: ID=%s, Topic=%s, Payload=%v, RetryCount=%d\n",
        dl.ID, dl.Topic, dl.Payload, dl.RetryCount)
}

// 清空死信队列
b.ClearDeadLetters()
fmt.Printf("清空后死信队列长度: %d\n", b.DeadLetterCount()) // 0
```

### 5.7 多个消费者与订阅

```go
b := pubsub.NewBroker(pubsub.DefaultConfig())
defer b.Stop()

ch1, _ := b.AddConsumer("consumer-1")
ch2, _ := b.AddConsumer("consumer-2")

// consumer-1 订阅精确主题
b.Subscribe("consumer-1", "notification.email")

// consumer-2 订阅所有 notification
b.Subscribe("consumer-2", "notification.#")

// 发布消息
b.Publish("notification.email", "邮件通知")
b.Publish("notification.sms", "短信通知")
b.Publish("notification.push", "推送通知")

// consumer-1 只收到 "notification.email"
// consumer-2 收到所有三条 notification 消息

// 取消订阅
b.Unsubscribe("consumer-2", "notification.#")

// consumer-2 不再收到 notification 消息
b.Publish("notification.email", "新邮件通知")
```

## 6. 错误处理

| 错误 | 场景 |
|------|------|
| `ErrBrokerStopped` | Broker 已停止后调用操作方法 |
| `ErrConsumerNotFound` | 操作不存在的消费者 |
| `ErrConsumerExists` | 添加已存在的消费者 ID |
| `ErrSubscriptionNotFound` | 取消不存在的订阅 |
| `ErrTopicInvalid` | 主题或过滤模式格式错误 |
| `ErrMessageNotFound` | 确认或否认不存在的消息 ID |
| `ErrBackpressureFull` | 消费者背压缓冲区已满（预留错误） |
| `ErrConsumerDisconnected` | 尝试重连已被移除的消费者 |

## 7. 线程安全说明

Broker 所有公共方法均为**并发安全**，可在多个 goroutine 中同时调用：

- 核心状态通过 `sync.RWMutex` 保护
- 读操作（查询计数、获取死信等）使用读锁
- 写操作（发布、订阅、确认等）使用写锁
- 未确认消息计数 `unackedCount` 使用 `atomic` 原子操作
- 消息 ID 生成使用 `atomic.AddUint64` 保证原子性
- 消费者 channel 的关闭使用 `sync.Once` 保证幂等性
- 后台协程通过 `sync.WaitGroup` 管理生命周期

## 8. 模块对比：PubSub vs EventBus

| 特性 | PubSub | EventBus |
|------|--------|----------|
| 分发模式 | 异步（channel 推送） | 同步/异步 |
| 过滤机制 | 基于 Topic 模式匹配 | 基于事件属性的灵活过滤 |
| 优先级 | 不支持 | 支持 |
| 中断机制 | 不支持 | 支持（同步模式） |
| 消息持久化 | 支持（死信队列、持久订阅） | 不支持 |
| ACK 机制 | 支持 | 不支持 |
| 重试机制 | 支持（超时重推） | 不支持 |
| 背压控制 | 支持 | 不支持 |
| 适用场景 | 消息队列、跨组件通信、可靠投递 | 进程内事件驱动、领域事件 |

**使用建议**：
- 需要消息可靠投递、消费解耦、离线缓存 → 使用 PubSub
- 进程内模块解耦、需要同步处理、优先级控制 → 使用 EventBus

## 9. 配置调优建议

### 9.1 AckTimeout 设置

- 太短：消息处理时间较长时容易误判为超时，导致不必要的重推
- 太长：真正失败的消息不能及时重推，影响系统响应性
- 建议：设置为平均处理时间的 2-3 倍

### 9.2 MaxRetry 设置

- 太小：暂时性故障（如网络抖动）导致消息过早进入死信
- 太大：坏消息长时间占用系统资源
- 建议：根据业务场景设置 3-5 次

### 9.3 MaxUnacked 设置

- 太小：消费者吞吐量上不去，频繁触发背压
- 太大：消费者内存压力大，失败时丢失的消息多
- 建议：根据消费者处理能力和内存容量设置，通常 100-1000

### 9.4 ConsumerBuffer 设置

- 太小：发布消息时容易阻塞，影响生产者性能
- 太大：内存占用高，GC 压力大
- 建议：设置为 MaxUnacked 的 2-5 倍
