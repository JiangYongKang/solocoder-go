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
1. 获取互斥锁
2. 检查缓冲区是否已满 (count == capacity)
3. 如果已满，释放锁，返回 false
4. 否则，将值写入 writePos 位置
5. writePos 向后移动一位 (模 capacity)
6. count 加 1
7. 检查高水位状态，记录需触发的回调（若有）
8. 释放互斥锁
9. 若有待执行的回调，在锁外执行回调，返回 true

**Overwrite 模式：
1. 获取互斥锁
2. 检查缓冲区是否已满 (count == capacity)
3. 如果已满：
   - 将 readPos 位置的槽位置零（防止引用类型内存泄漏）
   - readPos 向后移动一位（丢弃最旧数据），count 减 1
4. 将值写入 writePos 位置
5. writePos 向后移动一位 (模 capacity)
6. count 加 1
7. 检查高水位状态，记录需触发的回调（若有）
8. 释放互斥锁
9. 若有待执行的回调，在锁外执行回调，返回 true

### 4.3 读取操作 (Read)

```go
func (rb *RingBuffer[T]) Read() (T, bool)
```

1. 获取互斥锁
2. 检查缓冲区是否为空 (count == 0)
3. 如果为空，释放锁，返回零值和 false
4. 否则，读取 readPos 位置的值
5. **将 readPos 位置的槽位置零**（防止引用类型内存泄漏，确保 GC 可以回收已读取对象）
6. readPos 向后移动一位 (模 capacity)
7. count 减 1
8. 检查高水位状态，记录需触发的回调（若有）
9. 释放互斥锁
10. 若有待执行的回调，在锁外执行回调，返回读取的值和 true

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

## 11. 死锁预防与回调调度设计

### 11.1 问题背景

高水位告警回调 (`onHighWater` / `onLowWater`) 如果在持有互斥锁的状态下直接调用，而回调函数内部又尝试调用任何需要获取同一把锁的 RingBuffer 方法（如 `Read`、`Write`、`Len` 等），就会导致经典的"自死锁"（同一个 goroutine 试图重复获取已持有的不可重入锁）。

此外，如果使用显式 `Unlock` 而不是 `defer`，当锁内逻辑发生 panic 时会导致锁泄漏。

### 11.2 设计方案：IIFE + defer + 三阶段回调调度

采用三层防御机制：

**第一层：IIFE 包裹锁内逻辑 + defer Unlock**

使用立即执行函数（IIFE）包裹所有锁内操作，内部通过 `defer rb.mu.Unlock()` 保证即使发生 panic 锁也会被释放，同时锁内逻辑的结果通过闭包外层变量传递。

```go
func (rb *RingBuffer[T]) Write(value T) bool {
    var (
        needHigh bool
        needLow  bool
        result   bool
    )

    func() {
        rb.mu.Lock()
        defer rb.mu.Unlock()  // 防御性解锁：panic 也不会泄漏锁

        if rb.strategy == NoOverwrite && rb.count == rb.capacity {
            result = false
            return  // defer 保证锁被释放
        }

        // ... 状态修改 ...
        needHigh, needLow = rb.checkWaterMarkLocked(overwrote)
        result = true
    }()  // IIFE 结束时 defer 自动释放锁

    // 锁已释放，安全执行回调
    rb.dispatchCallbacks(needHigh, needLow)
    return result
}
```

**第二层：checkWaterMarkLocked 只记录状态标志，不捕获回调**

`checkWaterMarkLocked` 不再返回函数引用，只返回两个布尔标志 `needHigh` 和 `needLow`，表示是否需要触发高/低水位回调。这样做避免了在状态修改阶段就绑定具体的回调函数。

```go
func (rb *RingBuffer[T]) checkWaterMarkLocked(overwrote bool) (needHigh bool, needLow bool) {
    if rb.count >= rb.highWater && !rb.highWaterAlarm {
        rb.highWaterAlarm = true
        return true, false  // 只返回标志，不捕获回调
    } else if rb.count < rb.highWater && rb.highWaterAlarm && !overwrote {
        rb.highWaterAlarm = false
        return false, true
    }
    return false, false
}
```

**第三层：dispatchCallbacks 在执行前重新读取最新回调**

独立的 `dispatchCallbacks` 方法在锁完全释放后被调用。它会**再次短暂获取锁**，读取当前最新注册的回调函数引用，然后释放锁后在无锁状态下执行。

这样确保即使在"状态修改"和"回调执行"之间有另一 goroutine 通过 `OnHighWater` / `OnLowWater` 替换了回调函数，执行的也始终是最新注册的版本。

```go
func (rb *RingBuffer[T]) dispatchCallbacks(needHigh bool, needLow bool) {
    var (
        highCb func()
        lowCb  func()
    )

    if needHigh || needLow {
        rb.mu.Lock()
        if needHigh {
            highCb = rb.onHighWater  // 读取最新注册的回调
        }
        if needLow {
            lowCb = rb.onLowWater
        }
        rb.mu.Unlock()
    }

    // 无锁状态下执行回调
    if highCb != nil {
        highCb()
    }
    if lowCb != nil {
        lowCb()
    }
}
```

### 11.3 回调时序与语义

完整的执行时序如下：

```
T1: 获取锁
T2: 修改缓冲区状态（指针移动、count 更新）
T3: checkWaterMarkLocked 检测水位变化，设置 highWaterAlarm 标志，返回 needHigh/needLow 标志
T4: defer 释放锁
T5: （时间窗口：其他 goroutine 可调用 OnHighWater/OnLowWater 替换回调）
T6: dispatchCallbacks 再次获取锁
T7: 读取最新的 onHighWater/onLowWater 函数引用
T8: 释放锁
T9: 无锁状态下执行回调
```

**语义保证**：回调总是使用执行时刻（T9）之前最新注册的版本，而非状态变化时刻（T3）的版本。

### 11.4 安全性保证

| 场景 | 旧实现（锁内调回调 + 显式 Unlock） | 新实现（IIFE + defer + dispatchCallbacks） |
|------|----------------------------------|--------------------------------------------|
| 回调中调用 `rb.Len()` | **死锁** | 正常执行 |
| 回调中调用 `rb.Read()` | **死锁** | 正常执行 |
| 回调中调用 `rb.Write()` | **死锁** | 正常执行 |
| 锁内 panic | **锁泄漏** | 正常（defer 自动解锁） |
| 回调在 T5 窗口被替换 | 执行旧版（过时）回调 | 执行最新版回调 |
| 状态一致性 | 有保证 | 有保证（状态已在锁内完成修改） |

## 12. 内存安全设计

### 12.1 问题背景

当泛型参数 `T` 为引用类型（指针、slice、map、chan、接口等）时，`Read` 操作只复制指针值然后推进读指针，但底层数组对应槽位仍保留着原对象的引用。这会导致：
- 已被"读取消费"的对象仍然被缓冲区数组引用
- Go 垃圾回收器 (GC) 认为这些对象仍然可达，无法回收
- 长时间运行会造成内存泄漏，尤其当对象较大或持有更多引用时

### 12.2 清零策略

在以下操作中，将不再使用的数组槽位显式设置为泛型类型零值 `var zero T`：

**1. Read 操作：读取后清零读指针槽位**
```go
value := rb.buf[rb.readPos]
rb.buf[rb.readPos] = zero   // 关键：断开引用链，使 GC 可回收
rb.readPos = (rb.readPos + 1) % rb.capacity
```

**2. Overwrite 模式 Write 操作：覆盖前先清零被丢弃槽位**
```go
if rb.strategy == Overwrite && rb.count == rb.capacity {
    rb.buf[rb.readPos] = zero   // 清零最旧数据的槽位
    rb.readPos = (rb.readPos + 1) % rb.capacity
    rb.count--
}
```

**3. Clear 操作：遍历整个缓冲区清零所有槽位**
```go
var zero T
for i := range rb.buf {
    rb.buf[i] = zero
}
```

### 12.3 零值语义说明

Go 泛型的零值 (`var zero T`) 对于不同类型的表现：
- **值类型** (int, struct, float64 等)：零值为该类型的默认值，清零不影响 GC（本身不涉及引用）
- **指针类型** (*T)：零值为 `nil`，清零后对象不再被引用，GC 可正常回收
- **Slice / Map / Chan**：零值为 `nil`，底层数组/哈希表/队列失去引用后可被回收
- **接口类型** (interface{})：零值为 `nil`，若接口持有动态值引用，清零后动态值可被回收
- **字符串** (string)：虽然 string 是不可变值类型，但清零可使其指向的底层字节数组更早被回收

### 12.4 性能影响

清零操作是 O(1) 的单元素赋值（Clear 除外为 O(n)），开销极小：
- 对于值类型：仅一次内存写，开销可忽略
- 对于引用类型：一次指针写 (`nil`)，相比 GC 回收大量内存的收益，代价可忽略
- 不引入额外的内存分配
