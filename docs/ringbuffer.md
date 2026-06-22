# 环形缓冲区 (Ring Buffer) 模块

## 1. 模块概述

环形缓冲区（Circular Buffer / Ring Buffer) 是一种固定大小的先进先出 (FIFO) 数据结构，使用一段连续的内存空间模拟环形存储数据，当写指针到达缓冲区末尾时，会自动回到头部继续写入。本模块基于 Go 泛型实现，支持任意数据类型，提供非阻塞读写、覆盖策略开关和高水位告警机制。

**包路径**: `internal/ringbuffer`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 泛型支持 | 使用 Go 泛型实现，支持任意数据类型 T |
| 固定容量 | 缓冲区创建时设定固定容量，内存预先分配 |
| 循环写入 | 写指针到达末尾自动回到头部，实现循环利用 |
| 非阻塞读写 | 读操作在空缓冲区直接返回零值和 false；写操作在满缓冲区（不覆盖模式下返回 false |
| 覆盖策略 | 支持"覆盖"模式下缓冲区满时自动覆盖最旧数据并推进读指针；"不覆盖"模式下写入直接返回 false |
| 高水位告警 | 当缓冲区内有效数据量超过预设阈值时触发告警回调，水位回落到阈值以下时解除告警 |
| 线程安全 | 内部使用互斥锁保护，支持并发读写 |

## 3. 核心结构体与职责

### 3.1 RingBuffer[T any]

环形缓冲区的主结构体，对外提供所有操作接口。

```go
type RingBuffer[T any] struct {
    mu             sync.Mutex
    buf            []T
    capacity       int
    readPos        int
    writePos       int
    count          int
    strategy       OverwriteStrategy
    highWater      int
    highWaterAlarm bool
    onHighWater    func()
    onLowWater     func()
}
```

**职责**:
- 维护底层存储数组和读写指针位置
- 管理有效数据元素数量 (count)
- 控制写入覆盖策略 (Overwrite / NoOverwrite)
- 维护高水位告警状态和回调函数
- 提供线程安全的读写操作

### 3.2 Config

环形缓冲区的配置结构体。

```go
type Config struct {
    Capacity      int
    Strategy    OverwriteStrategy
    HighWaterMark int
}
```

**字段说明**:
- `Capacity`: 缓冲区容量（元素个数）
- `Strategy`: 写入覆盖策略，默认为 NoOverwrite
- `HighWaterMark`: 高水位告警阈值，0 表示不启用告警

### 3.3 OverwriteStrategy

写入覆盖策略枚举类型。

```go
type OverwriteStrategy int

const (
    NoOverwrite OverwriteStrategy = iota
    Overwrite
)
```

- `NoOverwrite`: 不覆盖模式，缓冲区满时写入返回 false
- `Overwrite`: 覆盖模式，缓冲区满时新数据覆盖最旧的未读数据

## 4. 环形缓冲区读写机制

### 4.1 基本概念

环形缓冲区使用两个指针维护数据的有效范围：
- **读指针 (readPos)**: 指向第一个有效数据的位置
- **写指针 (writePos)**: 指向下一个可写入的位置
- **元素计数 (count)**: 当前缓冲区中的有效元素数量

通过 `count` 字段直接记录有效元素数量，避免读写指针重合时无法区分"空"和"满"的歧义。

### 4.2 写入操作 (Write)

```go
func (rb *RingBuffer[T]) Write(value T) bool
```

**NoOverwrite 模式：
1. 检查缓冲区是否已满 (count == capacity)
2. 如果已满，直接返回 false
3. 否则，将值写入 writePos 位置
4. writePos 向后移动一位 (模 capacity)
5. count 加 1
6. 检查高水位告警，返回 true

**Overwrite 模式：
1. 检查缓冲区是否已满 (count == capacity)
2. 如果已满，readPos 向后移动一位（丢弃最旧数据），count 减 1
3. 将值写入 writePos 位置
4. writePos 向后移动一位 (模 capacity)
5. count 加 1
6. 检查高水位告警，返回 true

### 4.3 读取操作 (Read)

```go
func (rb *RingBuffer[T]) Read() (T, bool)
```

1. 检查缓冲区是否为空 (count == 0)
2. 如果为空，返回零值和 false
3. 否则，读取 readPos 位置的值
4. readPos 向后移动一位 (模 capacity)
5. count 减 1
6. 检查高水位告警，返回读取的值和 true

### 4.4 环绕示例

以容量为 4 的缓冲区为例：

```
初始状态: [_, _, _, _]
          readPos=0, writePos=0, count=0

写入 A: [A, _, _, _]
         readPos=0, writePos=1, count=1

写入 B: [A, B, _, _]
         readPos=0, writePos=2, count=2

读取: 返回 A
      [_, B, _, _]
      readPos=1, writePos=2, count=1

写入 C, D, E (NoOverwrite):
      写入 C: [_, B, C, _]  count=2
      写入 D: [_, B, C, D]  count=3
      写入 E: 返回 false (缓冲区满)

写入 C, D, E (Overwrite):
      写入 C: [_, B, C, _]  count=2
      写入 D: [_, B, C, D]  count=3
      写入 E: [E, _, C, D]  count=3 (B 被覆盖，readPos 移动到 2)
                         readPos=2, writePos=1
```

## 5. 覆盖机制详解

### 5.1 NoOverwrite (不覆盖模式)

- 缓冲区满时，新的写入操作直接返回 false
- 已有的数据保持不变，读指针不移动
- 适用于不能丢失数据的场景，需要调用方自行处理满缓冲区的情况

### 5.2 Overwrite (覆盖模式)

- 缓冲区满时，新数据自动覆盖最旧的未读数据
- 读指针随之前进一位，丢弃最旧数据
- count 保持不变（等于缓冲区容量）
- 适用于可以丢弃旧数据、始终保留最新数据的场景

### 5.3 策略切换

运行时可通过 `SetStrategy` 方法动态切换写入策略：

```go
rb.SetStrategy(ringbuffer.Overwrite)
```

## 6. 高水位告警机制

### 6.1 基本原理

高水位告警用于监控缓冲区的使用情况，当数据量超过预设阈值时触发告警回调，当数据量回落到阈值以下时解除告警。

### 6.2 告警触发条件

- **高水位触发：`count >= highWater` 且当前未处于告警状态
- **低水位解除：`count < highWater` 且当前处于告警状态
- 告警状态具有"滞回"特性：只有状态变化时才触发回调，避免频繁抖动

### 6.3 覆盖模式下的告警

在 Overwrite 模式下，由于 count 始终不会超过 capacity，当 highWater <= capacity：
- 写入操作不会触发高水位告警后，覆盖写入不会改变 count，不会重复触发
- 只有读取操作使 count 降到阈值以下时，才会触发低水位解除

### 6.4 告警回调注册

```go
rb.OnHighWater(func() {
    // 高水位告警处理逻辑
    log.Println("ring buffer: high water mark reached")
})

rb.OnLowWater(func() {
    // 低水位解除处理逻辑
    log.Println("ring buffer: back to normal")
})
```

## 7. API 参考

### 7.1 构造函数

```go
func NewRingBuffer[T any](capacity int) (*RingBuffer[T], error)
func NewRingBufferWithConfig[T any](cfg Config) (*RingBuffer[T], error)
func DefaultConfig() Config
```

### 7.2 基本操作

```go
func (rb *RingBuffer[T]) Write(value T) bool
func (rb *RingBuffer[T]) Read() (T, bool)
func (rb *RingBuffer[T]) Peek() (T, bool)
func (rb *RingBuffer[T]) Len() int
func (rb *RingBuffer[T]) Cap() int
func (rb *RingBuffer[T]) IsFull() bool
func (rb *RingBuffer[T]) IsEmpty() bool
func (rb *RingBuffer[T]) Clear()
```

### 7.3 策略管理

```go
func (rb *RingBuffer[T]) SetStrategy(strategy OverwriteStrategy)
func (rb *RingBuffer[T]) GetStrategy() OverwriteStrategy
```

### 7.4 高水位管理

```go
func (rb *RingBuffer[T]) SetHighWaterMark(mark int) error
func (rb *RingBuffer[T]) OnHighWater(fn func())
func (rb *RingBuffer[T]) OnLowWater(fn func())
```

## 8. 使用示例

### 8.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/ringbuffer"
)

func main() {
    rb, err := ringbuffer.NewRingBuffer[int](10)
    if err != nil {
        panic(err)
    }

    // 写入数据
    rb.Write(1)
    rb.Write(2)
    rb.Write(3)

    fmt.Printf("缓冲区大小: %d\n", rb.Len())

    // 读取数据
    val, ok := rb.Read()
    if ok {
        fmt.Printf("读取: %d\n", val)
    }

    // 查看但不读取
    val, ok = rb.Peek()
    if ok {
        fmt.Printf("查看: %d\n", val)
    }
}
```

### 8.2 覆盖模式

```go
cfg := ringbuffer.Config{
    Capacity: 5,
    Strategy: ringbuffer.Overwrite,
}
rb, _ := ringbuffer.NewRingBufferWithConfig[int](cfg)

for i := 0; i < 10; i++ {
    rb.Write(i)
}

// 缓冲区中保留的是最新的 5 个元素: 5, 6, 7, 8, 9
for !rb.IsEmpty() {
    val, _ := rb.Read()
    fmt.Println(val)
}
```

### 8.3 高水位告警

```go
cfg := ringbuffer.Config{
    Capacity:     100,
    HighWaterMark: 80,
}
rb, _ := ringbuffer.NewRingBufferWithConfig[int](cfg)

rb.OnHighWater(func() {
    fmt.Println("警告：缓冲区使用率超过 80%")
})

rb.OnLowWater(func() {
    fmt.Println("缓冲区已恢复正常水位")
})

// 写入大量数据
for i := 0; i < 90; i++ {
    rb.Write(i)
}

// 读取数据使水位下降
for i := 0; i < 20; i++ {
    rb.Read()
}
```

### 8.4 使用自定义类型

```go
type Message struct {
    ID      int
    Content string
}

rb, _ := ringbuffer.NewRingBuffer[Message](10)

rb.Write(Message{ID: 1, Content: "hello"})
rb.Write(Message{ID: 2, Content: "world"})

msg, ok := rb.Read()
if ok {
    fmt.Printf("消息 %d: %s\n", msg.ID, msg.Content)
}
```

### 8.5 清空缓冲区

```go
rb, _ := ringbuffer.NewRingBuffer[string](5)

rb.Write("a")
rb.Write("b")
rb.Write("c")

fmt.Println(rb.Len()) // 3

rb.Clear()

fmt.Println(rb.Len()) // 0
fmt.Println(rb.IsEmpty()) // true
```

## 9. 错误定义

| 错误 | 触发场景 |
|------|----------|
| `ErrInvalidCapacity` | 创建缓冲区时容量小于等于 0 |
| `ErrInvalidHighWater` | 高水位阈值小于 0 或大于容量 |

## 10. 并发安全

环形缓冲区内部使用 `sync.Mutex` 进行并发保护：
- 所有公共方法都加锁保护，支持多 goroutine 安全访问
- 写入和读取操作都是非阻塞的，不会等待条件满足
- 适用于生产者-消费者模式等并发场景
