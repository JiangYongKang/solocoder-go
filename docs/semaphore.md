# 信号量模块 (Semaphore)

## 1. 模块概述

信号量（Semaphore）是一种经典的并发同步原语，用于控制对共享资源的并发访问数量。信号量维护一组许可，调用方通过获取许可来访问资源，使用完毕后释放许可。当许可耗尽时，新的获取请求会阻塞等待，直到有许可被释放。

本模块提供了功能完善的信号量实现，支持超时等待、许可数量动态调整、公平/非公平模式等高级特性。

## 2. 功能特性

### 2.1 许可获取与释放

- 信号量在创建时设定总许可数量
- 调用方通过 `Acquire` 获取一个许可后，已持有许可数加一，可用许可数减一
- 通过 `Release` 释放许可后，已持有许可数减一，可用许可数相应增加
- 当可用许可数为零时，新的 `Acquire` 调用阻塞等待

### 2.2 带超时的等待

- `Acquire` 操作支持传入超时时间
- 在指定时间内如果获取到许可则返回 `true`
- 超时仍未获取则返回 `false` 表示获取失败
- 超时为 0 时表示无限等待，直到获取到许可为止
- 调用方根据返回值决定后续行为

### 2.3 许可数量的动态调整

- 支持在运行时动态增加或减少信号量的总许可数
- 增加许可时唤醒等待中的 `Acquire` 调用
- 减少许可时不强制收回已持有的许可，而是减少后续可获取的许可数
- 如果当前已持有许可数超过新的总许可数，超出部分的许可在释放时不会增加可用许可数，直到已持有数量降回到总许可数以下

### 2.4 公平排队模式

- 支持启用公平模式，在该模式下等待许可的 goroutine 按先到先得（FIFO）的顺序排队获取
- 禁用时使用非公平模式，新到达的 goroutine 如果恰好在许可释放时到达，可以直接获取（插队），性能更高
- 两种模式可在创建时选择
- 公平模式保证了不会出现饥饿现象，但吞吐量相对较低

### 2.5 辅助功能

- `TryAcquire()`: 非阻塞尝试获取许可，立即返回成功或失败
- `AvailablePermits()`: 查询当前可用许可数
- `TotalPermits()`: 查询总许可数
- `QueueLength()`: 查询当前等待队列长度
- `IsFair()`: 查询是否为公平模式

## 3. 核心结构体

### 3.1 Semaphore

主信号量结构体，提供信号量的核心功能。

| 字段 | 类型 | 职责 |
|------|------|------|
| `mu` | `sync.Mutex` | 保护所有共享状态的互斥锁 |
| `held` | `int` | 当前已持有的许可数量 |
| `totalPermits` | `int` | 总许可数量上限 |
| `fair` | `bool` | 是否启用公平模式 |
| `waiters` | `[]*waiter` | 等待队列，存储所有等待中的 goroutine |

主要方法：

| 方法 | 返回类型 | 说明 |
|------|----------|------|
| `Acquire(timeout)` | `bool` | 获取一个许可，支持超时等待 |
| `Release()` | 无 | 释放一个许可 |
| `TryAcquire()` | `bool` | 非阻塞尝试获取许可 |
| `IncreasePermits(delta)` | `error` | 增加总许可数 |
| `DecreasePermits(delta)` | `error` | 减少总许可数 |
| `AvailablePermits()` | `int` | 获取当前可用许可数 |
| `TotalPermits()` | `int` | 获取总许可数 |
| `QueueLength()` | `int` | 获取等待队列长度 |
| `IsFair()` | `bool` | 是否为公平模式 |

### 3.2 waiter

内部结构体，表示一个等待中的 goroutine。

| 字段 | 类型 | 职责 |
|------|------|------|
| `ch` | `chan bool` | 用于通知等待者的通道，`true` 表示获取成功，`false` 表示超时 |

## 4. 错误定义

| 错误常量 | 说明 |
|----------|------|
| `ErrInvalidPermits` | 创建信号量时许可数不能为负数 |
| `ErrNegativePermits` | 减少许可后总许可数不能为负数 |
| `ErrInvalidDelta` | 调整许可数量时 delta 必须大于 0 |

## 5. 信号量的获取与释放机制

### 5.1 许可获取（Acquire）

```
调用 Acquire(timeout)
    ↓
获取互斥锁
    ↓
有可用许可 且 (非公平 或 队列为空)？
    ├─ 是 → held++, 释放锁, 返回 true
    └─ 否 → 创建 waiter, 加入等待队列
                ↓
            设置超时定时器（如果 timeout > 0）
                ↓
            释放互斥锁
                ↓
            阻塞等待 waiter.ch
                ↓
            收到结果 → 停止定时器, 返回结果
```

### 5.2 许可释放（Release）

```
调用 Release()
    ↓
获取互斥锁
    ↓
held > 0 ?
    ├─ 是 → held--
    └─ 否 → 不做处理（防止过度释放）
    ↓
调用 dispatchWaiters() 尝试唤醒等待者
    ↓
释放互斥锁
```

### 5.3 等待者调度（dispatchWaiters）

```
循环：
    有可用许可 且 队列非空 ?
        ├─ 否 → 结束循环
        └─ 是 → 取出队首 waiter
                    ↓
                held++
                    ↓
                向 waiter.ch 发送 true
                    ↓
                继续循环
```

## 6. 许可数量动态调整机制

### 6.1 增加许可（IncreasePermits）

```
调用 IncreasePermits(delta)
    ↓
delta <= 0 ? → 返回 ErrInvalidDelta
    ↓
获取互斥锁
    ↓
totalPermits += delta
    ↓
调用 dispatchWaiters() 唤醒等待者
    ↓
释放互斥锁, 返回 nil
```

### 6.2 减少许可（DecreasePermits）

```
调用 DecreasePermits(delta)
    ↓
delta <= 0 ? → 返回 ErrInvalidDelta
    ↓
获取互斥锁
    ↓
totalPermits - delta < 0 ? → 返回 ErrNegativePermits
    ↓
totalPermits -= delta
    ↓
释放互斥锁, 返回 nil
```

**注意**：减少许可时不强制收回已持有的许可。如果当前已持有许可数超过新的总许可数，超出部分的许可在释放时不会增加可用许可数，直到已持有数量降回到总许可数以下。

### 6.3 超额持有状态

当总许可数减少到低于当前已持有数量时，信号量进入"超额持有"状态：

```
初始状态：total=5, held=2, available=3
    ↓
DecreasePermits(4): total=1
    ↓
此时 held(2) > total(1)，进入超额持有状态
    ↓
available = max(0, total - held) = 0
    ↓
第一次 Release: held=1, held == total, available=0
    ↓
第二次 Release: held=0, held < total, available=1
    ↓
恢复正常状态
```

## 7. 公平模式与非公平模式

### 7.1 非公平模式（默认）

- **插队（Barging）**：新到达的 goroutine 如果发现有可用许可，可以直接获取，无需排队
- **优点**：吞吐量高，减少了上下文切换的开销
- **缺点**：可能出现饥饿现象（某些 goroutine 长时间得不到调度）
- **适用场景**：对公平性要求不高，追求性能的场景

### 7.2 公平模式

- **FIFO 排队**：所有等待的 goroutine 按到达顺序排队，先到先得
- **无插队**：即使有可用许可，如果队列中有等待者，新到达的 goroutine 也必须排队
- **优点**：保证公平性，不会出现饥饿
- **缺点**：吞吐量相对较低
- **适用场景**：对公平性要求高的场景

### 7.3 TryAcquire 的公平语义

- **非公平模式**：`TryAcquire` 总是尝试立即获取（如果有可用许可），不管队列是否有等待者
- **公平模式**：`TryAcquire` 也遵循公平原则，如果队列中有等待者，即使有可用许可也会失败

## 8. 超时机制

### 8.1 超时等待流程

```
goroutine 调用 Acquire(timeout)
    ↓
无可用许可 → 创建 waiter，加入队列，启动定时器
    ↓
定时器触发：
    - 获取锁
    - 从等待队列中移除该 waiter
    - 向 waiter.ch 发送 false
    - 释放锁
    ↓
goroutine 收到 false，返回获取失败
```

### 8.2 超时清理

- 超时的 goroutine 会被从等待队列中移除
- 定时器在获取成功后会被正确停止，避免资源泄漏
- 超时回调在持有互斥锁的状态下执行，保证操作原子性

## 9. 使用示例

### 9.1 基本用法

```go
package main

import (
	"fmt"
	"sync"
	"time"

	"solocoder-go/internal/semaphore"
)

func main() {
	s, err := semaphore.New(3)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			s.Acquire(0)
			defer s.Release()

			fmt.Printf("Goroutine %d: 开始执行\n", id)
			time.Sleep(100 * time.Millisecond)
			fmt.Printf("Goroutine %d: 执行完毕\n", id)
		}(i)
	}

	wg.Wait()
	fmt.Println("所有任务完成")
}
```

### 9.2 带超时的获取

```go
s, _ := semaphore.New(1)

s.Acquire(0)

go func() {
	time.Sleep(500 * time.Millisecond)
	s.Release()
}()

start := time.Now()
success := s.Acquire(1 * time.Second)
elapsed := time.Since(start)

if success {
	fmt.Println("获取成功，等待了", elapsed)
	s.Release()
} else {
	fmt.Println("超时失败，等待了", elapsed)
}
```

### 9.3 非阻塞尝试获取

```go
s, _ := semaphore.New(1)

if s.TryAcquire() {
	fmt.Println("获取成功")
	s.Release()
} else {
	fmt.Println("获取失败，没有可用许可")
}
```

### 9.4 动态调整许可数

```go
s, _ := semaphore.New(2)

// 占满所有许可
s.Acquire(0)
s.Acquire(0)

// 有新的 goroutine 等待
var wg sync.WaitGroup
wg.Add(1)
go func() {
	defer wg.Done()
	s.Acquire(0)
	fmt.Println("第三个 goroutine 获取到许可")
}()

time.Sleep(100 * time.Millisecond)
fmt.Println("等待队列长度:", s.QueueLength()) // 1

// 增加许可，唤醒等待者
s.IncreasePermits(2)
fmt.Println("总许可数:", s.TotalPermits()) // 4

wg.Wait()

// 减少许可
s.DecreasePermits(1)
fmt.Println("总许可数:", s.TotalPermits()) // 3
fmt.Println("可用许可数:", s.AvailablePermits()) // 1
```

### 9.5 公平模式

```go
s, _ := semaphore.New(0, true) // 启用公平模式

const n = 5
results := make([]int, n)
ready := make([]chan struct{}, n)

for i := 0; i < n; i++ {
	ready[i] = make(chan struct{})
}

var wg sync.WaitGroup
for i := 0; i < n; i++ {
	wg.Add(1)
	go func(id int) {
		defer wg.Done()
		close(ready[id])
		s.Acquire(0)
		results[id] = id
		s.Release()
	}(i)
	<-ready[i]
	time.Sleep(10 * time.Millisecond)
}

s.IncreasePermits(1)
wg.Wait()

// 公平模式下，按到达顺序获取，results[0] 应该是第一个获取的
fmt.Println("公平模式下，先到先得")
```

### 9.6 限流场景

```go
// 使用信号量实现并发限流
type RateLimiter struct {
	sem *semaphore.Semaphore
}

func NewRateLimiter(maxConcurrency int) *RateLimiter {
	sem, _ := semaphore.New(maxConcurrency)
	return &RateLimiter{sem: sem}
}

func (r *RateLimiter) Do(task func(), timeout time.Duration) bool {
	if !r.sem.Acquire(timeout) {
		return false
	}
	defer r.sem.Release()
	task()
	return true
}
```

## 10. 设计考量

### 10.1 并发安全

- 所有共享状态通过 `sync.Mutex` 保护
- 等待通知通过带缓冲的 channel（`chan bool, 1`）实现，避免发送方阻塞
- 定时器回调在持有锁的状态下执行，保证状态一致性
- 超时时从等待队列中原子地移除等待者

### 10.2 内存管理

- 每个等待者使用独立的 channel，获取完成后自动回收
- 定时器在正常获取后立即停止，确保资源回收
- 等待队列使用切片实现，FIFO 顺序，公平模式下按顺序唤醒

### 10.3 公平性设计

- 公平模式严格遵循 FIFO 原则
- 非公平模式允许插队，提高吞吐量
- TryAcquire 同样遵循公平/非公平语义
- 调度时总是从队首取出等待者

### 10.4 动态调整语义

- 增加许可立即生效，并唤醒等待者
- 减少许可不强制收回，超额部分由持有者自行释放后"消解"
- 总许可数保证非负
- 已持有许可数可以超过总许可数（超额持有状态）

### 10.5 超时语义

- timeout = 0 表示无限等待
- timeout > 0 表示最多等待指定时间
- 超时后立即从等待队列移除，不影响其他等待者
- 超时回调与正常释放路径通过互斥锁保证互斥
