# 同步屏障模块 (Barrier)

## 1. 模块概述

同步屏障（Barrier）是一种并发同步原语，用于协调多个 goroutine 在某个预定的屏障点（barrier point）等待，直到所有参与方都到达该点后，所有等待的 goroutine 才会被同时释放继续执行。

本模块提供了功能完善的同步屏障实现，支持超时放弃机制、可选回调触发、可重复使用重置等高级特性，并提供了 `CyclicBarrier` 用于需要多轮循环同步的场景。

## 2. 功能特性

### 2.1 多 goroutine 同步等待
- 支持创建指定参与方数量的屏障
- 多个 goroutine 调用等待接口后阻塞在屏障点
- 当到达屏障的 goroutine 数量达到预设的参与方数量时，所有等待的 goroutine 同时被释放

### 2.2 超时放弃机制
- 每个等待的 goroutine 可设置独立的等待超时时间
- 超时后该 goroutine 放弃等待并返回 `ErrTimeout` 错误
- 超时 goroutine 的名额会从屏障计数器中自动移除（`effectiveNeeded` 递减）
- 剩余等待者继续等待，直到剩余参与方全部到齐或各自超时

### 2.3 可选回调触发
- 支持在创建屏障时指定一个可选的屏障回调函数 (`CallbackFunc`)
- 回调在所有等待 goroutine 被释放前**同步执行**
- 回调执行完毕后，所有等待 goroutine 才被告知可以继续
- 回调执行失败（返回 error）不影响屏障释放，但失败信息会返回给各等待 goroutine
- **并发安全**：回调函数在锁外执行，可以安全地调用 Barrier 的查询方法和其他非阻塞方法，不会导致死锁

### 2.4 可重复使用的屏障重置
- 屏障在一次释放后支持重置回到初始状态以便下一轮使用
- 重置操作将计数器清零，并可选择是否修改参与方数量
- 如果有 goroutine 仍在等待时，`Reset()` 不允许重置并返回 `ErrResetWhileWaiting` 错误
- 支持 `ForceReset()` 强制重置，会向所有等待者发送 `ErrBarrierReset` 错误
- 重置后新的 goroutine 可重新聚集到屏障点

### 2.5 辅助功能
- `Break()`: 手动破坏屏障，所有当前和后续等待者返回 `ErrBroken`
- `SetCallback()`: 动态设置或替换回调函数
- 状态查询方法：`Participants()`, `Arrived()`, `Waiting()`, `EffectiveNeeded()`, `IsBroken()`, `IsReleased()`

### 2.6 CyclicBarrier 循环屏障
- 自动循环语义：每轮释放后自动进入下一轮等待状态
- 轮次感知：支持查询当前轮次号（从 0 开始计数）
- 轮次回调：回调函数可接收当前轮次号，方便区分不同轮次的处理逻辑
- 独立锁保护：使用独立的互斥锁管理循环状态，与基础屏障的锁互不干扰

## 3. 核心结构体

### 3.1 Barrier

主屏障结构体，提供基础的屏障同步功能。

| 字段 | 类型 | 职责 |
|------|------|------|
| `mu` | `sync.Mutex` | 保护所有共享状态的互斥锁 |
| `participants` | `int` | 初始设置的参与方总数 |
| `effectiveNeeded` | `int` | 当前实际需要的参与方数（超时后会减少） |
| `arrived` | `int` | 当前已到达屏障点的 goroutine 数量 |
| `waiting` | `int` | 当前正在等待的 goroutine 数量 |
| `callback` | `CallbackFunc` | 屏障释放时执行的可选回调函数 |
| `generation` | `uint64` | 屏障代数，用于区分不同轮次的等待 |
| `broken` | `bool` | 标记屏障是否被破坏 |
| `released` | `bool` | 标记当前轮是否已释放 |
| `waiters` | `map[uint64]*waiter` | 当前所有等待者的映射表 |
| `nextWaiterID` | `uint64` | 下一个等待者的唯一 ID |

### 3.2 waiter

内部结构体，表示一个等待中的 goroutine。

| 字段 | 类型 | 职责 |
|------|------|------|
| `id` | `uint64` | 等待者唯一标识 |
| `done` | `chan error` | 用于通知等待者释放的通道，同时传递错误信息 |

### 3.3 CyclicBarrier

循环屏障，通过组合 `Barrier` 实现具有自动循环语义的屏障。

| 字段 | 类型 | 职责 |
|------|------|------|
| `mu` | `sync.Mutex` | 保护循环状态的互斥锁 |
| `barrier` | `*Barrier` | 底层基础屏障实例 |
| `round` | `uint64` | 当前轮次号（从 0 开始） |
| `callback` | `CyclicCallbackFunc` | 带轮次信息的回调函数 |

### 3.4 CallbackFunc

```go
type CallbackFunc func() error
```

屏障回调函数类型，在屏障释放前同步执行。返回的 error 会传递给所有等待的 goroutine。

### 3.5 CyclicCallbackFunc

```go
type CyclicCallbackFunc func(round uint64) error
```

循环屏障的回调函数类型，回调参数为当前轮次号（从 0 开始）。

## 4. 错误定义

| 错误常量 | 说明 |
|----------|------|
| `ErrInvalidParticipants` | 参与方数量必须大于 0 |
| `ErrTimeout` | goroutine 等待超时 |
| `ErrBarrierReset` | 等待过程中屏障被强制重置 |
| `ErrResetWhileWaiting` | 有 goroutine 等待时不允许普通重置 |
| `ErrBroken` | 屏障被破坏 |

## 5. 屏障等待与释放的生命周期

### 5.1 正常生命周期

```
创建屏障 (New)
    ↓
[ 第 N 轮 ]
    ↓
goroutine1 调用 Wait() ──→ arrived=1, waiting=1, 阻塞等待
goroutine2 调用 Wait() ──→ arrived=2, waiting=2, 阻塞等待
    ...
goroutineN 调用 Wait() ──→ arrived=N, 触发释放
    ↓
    ├─ 快照所有 waiter 的 channel
    ├─ 更新状态（arrived=0, generation++, released=true）
    ├─ 释放互斥锁
    ├─ 执行 callback() (如果设置了，在锁外执行)
    └─ 通过 done channel 通知所有等待者（附带 callback error）
    ↓
所有 goroutine 同时被释放
    ↓
[ 可选择 Reset() 或 ForceReset() 后开始下一轮 ]
```

### 5.2 超时生命周期

```
创建屏障 (participants=5)
    ↓
goroutine1 调用 Wait(50ms)  ──→ 阻塞，设置定时器
goroutine2 调用 Wait(0)      ──→ 阻塞，无超时
goroutine3 调用 Wait(0)      ──→ 阻塞，无超时
    ↓
50ms 后 goroutine1 超时
    ↓
    ├─ effectiveNeeded = 5 - 1 = 4
    ├─ arrived = 3 - 1 = 2
    ├─ waiting = 3 - 1 = 2
    ├─ goroutine1 返回 ErrTimeout
    └─ 检查：arrived(2) < effectiveNeeded(4)，不释放
    ↓
goroutine4 调用 Wait(0) ──→ arrived=3, effectiveNeeded=4
goroutine5 调用 Wait(0) ──→ arrived=4, effectiveNeeded=4，触发释放
    ↓
    ├─ 快照 waiter channels
    ├─ 更新状态，释放锁
    ├─ 执行 callback() (如果设置了)
    ├─ goroutine2/3/4/5 同时被释放
    └─ 状态清理，generation++
```

### 5.3 强制重置生命周期

```
goroutines 正在屏障点等待
    ↓
调用 ForceReset()
    ↓
    ├─ 向所有等待者发送 ErrBarrierReset
    ├─ 清理 waiters 映射表
    ├─ arrived = 0, waiting = 0
    ├─ generation++
    └─ released = false, broken = false
    ↓
所有等待者收到 ErrBarrierReset 后返回
    ↓
屏障可重新使用
```

### 5.4 CyclicBarrier 循环生命周期

```
创建循环屏障 (NewCyclic)
    ↓
[ 第 0 轮 ]
    ├─ 第一个 goroutine 调用 Await()
    ├─ 检查屏障已释放 → round++ → 自动 Reset()
    └─ 所有 goroutine 到达后释放，回调收到 round=0
    ↓
[ 第 1 轮 ]
    ├─ 第一个 goroutine 调用 Await()
    ├─ 检查屏障已释放 → round++ → 自动 Reset()
    └─ 所有 goroutine 到达后释放，回调收到 round=1
    ↓
    ... (持续循环)
    ↓
调用 ResetBarrier() → round 归零，重新从第 0 轮开始
```

## 6. 主要 API

### 6.1 构造函数

#### `New(participants int, callback ...CallbackFunc) (*Barrier, error)`

创建一个新的屏障。

- **参数**:
  - `participants`: 需要等待的参与方总数，必须 > 0
  - `callback`: 可选，屏障释放时执行的回调函数
- **返回**: 屏障实例或错误

#### `NewCyclic(parties int, callback ...CyclicCallbackFunc) (*CyclicBarrier, error)`

创建一个新的循环屏障。

- **参数**:
  - `parties`: 每轮需要等待的参与方总数，必须 > 0
  - `callback`: 可选，带轮次信息的回调函数
- **返回**: 循环屏障实例或错误

### 6.2 Barrier 核心方法

#### `(b *Barrier) Wait(timeout time.Duration) error`

等待屏障点。

- **参数**:
  - `timeout`: 等待超时时间，0 表示无限等待
- **返回**:
  - `nil`: 正常释放
  - `ErrTimeout`: 等待超时
  - `ErrBroken`: 屏障被破坏
  - `ErrBarrierReset`: 等待中被强制重置
  - `callback error`: 回调返回的错误

#### `(b *Barrier) Reset(newParticipants ...int) error`

重置屏障（无等待者时）。

- **参数**: `newParticipants` 可选，重置时修改参与方数量
- **返回**:
  - `nil`: 重置成功
  - `ErrResetWhileWaiting`: 有 goroutine 等待
  - `ErrInvalidParticipants`: 参与方数量非法

#### `(b *Barrier) ForceReset(newParticipants ...int) error`

强制重置屏障（即使有等待者）。所有等待者收到 `ErrBarrierReset`。

#### `(b *Barrier) Break()`

破坏屏障，所有当前和后续等待者返回 `ErrBroken`。

#### `(b *Barrier) SetCallback(cb CallbackFunc)`

动态设置或替换回调函数。

### 6.3 Barrier 查询方法

| 方法 | 返回类型 | 说明 |
|------|----------|------|
| `Participants()` | `int` | 获取初始参与方数量 |
| `Arrived()` | `int` | 获取当前已到达数量 |
| `Waiting()` | `int` | 获取当前正在等待的数量 |
| `EffectiveNeeded()` | `int` | 获取当前实际需要的参与方数 |
| `IsBroken()` | `bool` | 屏障是否被破坏 |
| `IsReleased()` | `bool` | 当前轮是否已释放 |

### 6.4 CyclicBarrier 方法

| 方法 | 返回类型 | 说明 |
|------|----------|------|
| `Await(timeout time.Duration) error` | `error` | 等待屏障（自动循环，每轮释放后进入下一轮） |
| `GetParties()` | `int` | 获取参与方数量 |
| `GetNumberWaiting()` | `int` | 获取当前等待者数量 |
| `GetRound()` | `uint64` | 获取当前轮次号（从 0 开始） |
| `ResetBarrier(newParties ...int) error` | `error` | 重置屏障，轮次归零 |
| `ForceReset(newParties ...int) error` | `error` | 强制重置屏障，轮次归零 |
| `IsBroken()` | `bool` | 屏障是否被破坏 |
| `SetCallback(c CyclicCallbackFunc)` | 无 | 设置带轮次信息的回调函数 |

## 7. 回调执行与并发安全

### 7.1 回调执行时机

回调函数在屏障满足释放条件时执行，执行时机遵循以下规则：

1. **状态先行更新**：在执行回调之前，屏障的内部状态（`arrived`, `waiters`, `generation`, `released` 等）已全部更新完成
2. **锁外执行**：回调函数在不持有屏障互斥锁的状态下执行
3. **同步执行**：回调函数同步执行，执行完毕后才会通知所有等待的 goroutine
4. **单例执行**：每轮屏障释放时，回调函数有且仅有一次调用

### 7.2 并发安全说明

> **重要警告**：回调函数在屏障锁外执行，这意味着：
>
> - ✅ **可以安全调用**：屏障的所有查询方法（`Participants()`, `Arrived()`, `Waiting()`, `EffectiveNeeded()`, `IsBroken()`, `IsReleased()`）
> - ✅ **可以安全调用**：`SetCallback()` 方法
> - ✅ **可以安全调用**：`Reset()`、`ForceReset()`、`Break()` 等修改方法（但需注意语义影响）
> - ⚠️ **注意**：由于回调执行时屏障状态已更新，查询方法返回的是**下一轮**的初始状态
> - ❌ **不推荐**：在回调中启动新的 goroutine 并调用 `Wait()`，这可能导致意外的并发行为

### 7.3 回调中的状态查询

当回调函数执行时，屏障已完成本轮的状态切换：

- `arrived` 已清零（0）
- `generation` 已递增（指向下一轮）
- `released` 为 `true`
- `waiting` 为 0

因此，在回调中调用查询方法得到的是**下一轮**的初始状态，而不是刚结束那一轮的状态。如果需要在回调中处理本轮的业务数据，应在 goroutine 到达屏障点之前将数据准备好。

## 8. 使用示例

### 8.1 基础同步屏障

```go
package main

import (
	"fmt"
	"sync"
	"time"

	"solocoder-go/internal/barrier"
)

func main() {
	const numWorkers = 3

	b, _ := barrier.New(numWorkers)

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			<-start

			fmt.Printf("Worker %d: 执行第一阶段任务\n", workerID)
			time.Sleep(time.Duration(workerID) * 100 * time.Millisecond)

			fmt.Printf("Worker %d: 到达屏障点\n", workerID)
			err := b.Wait(0)
			if err != nil {
				fmt.Printf("Worker %d: 等待错误: %v\n", workerID, err)
				return
			}

			fmt.Printf("Worker %d: 屏障释放，继续执行第二阶段\n", workerID)
		}()
	}

	close(start)
	wg.Wait()
	fmt.Println("所有任务完成")
}
```

### 8.2 带超时的屏障

```go
b, _ := barrier.New(5)

var wg sync.WaitGroup

for i := 0; i < 5; i++ {
	wg.Add(1)
	go func(idx int) {
		defer wg.Done()
		timeout := time.Duration((idx+1)*100) * time.Millisecond
		err := b.Wait(timeout)
		if errors.Is(err, barrier.ErrTimeout) {
			fmt.Printf("Goroutine %d 超时放弃\n", idx)
		} else if err == nil {
			fmt.Printf("Goroutine %d 正常通过\n", idx)
		}
	}(i)
}

wg.Wait()
```

### 8.3 带回调的屏障

```go
callback := func() error {
	fmt.Println("所有 goroutine 已到齐，执行屏障回调...")
	if someCondition {
		return errors.New("回调执行失败")
	}
	return nil
}

b, _ := barrier.New(3, callback)

var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Wait(0)
		if err != nil {
			fmt.Println("回调失败:", err)
		} else {
			fmt.Println("屏障正常释放")
		}
	}()
}
wg.Wait()
```

### 8.4 回调中安全调用屏障方法

```go
b, _ := barrier.New(3)

callback := func() error {
	// 回调在锁外执行，可以安全调用查询方法
	// 注意：此时屏障已切换到下一轮状态
	parties := b.Participants()
	isReleased := b.IsReleased()
	fmt.Printf("回调中查询: parties=%d, released=%v\n", parties, isReleased)
	return nil
}
b.SetCallback(callback)
```

### 8.5 可重复使用的屏障

```go
b, _ := barrier.New(2)

for round := 0; round < 3; round++ {
	fmt.Printf("=== 第 %d 轮 ===\n", round+1)

	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("Goroutine %d 开始执行\n", id)
			b.Wait(0)
			fmt.Printf("Goroutine %d 通过屏障\n", id)
		}(i)
	}
	wg.Wait()

	err := b.Reset()
	if err != nil {
		panic(err)
	}
}
```

### 8.6 CyclicBarrier 循环屏障

```go
cb, _ := barrier.NewCyclic(2, func(round uint64) error {
	fmt.Printf("--- 第 %d 轮屏障点通过 ---\n", round)
	return nil
})

for round := 0; round < 3; round++ {
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("[Round %d] Goroutine %d 到达\n", round, id)
			cb.Await(0)
			fmt.Printf("[Round %d] Goroutine %d 继续\n", round, id)
		}(i)
	}
	wg.Wait()
}

// 查询当前轮次
fmt.Printf("当前轮次: %d\n", cb.GetRound())
```

## 9. 设计考量

### 9.1 并发安全
- 所有共享状态通过 `sync.Mutex` 保护
- 等待通知通过带缓冲的 channel (`chan error, 1`) 实现，避免死锁
- 使用 `generation` 代数区分不同轮次，防止跨轮次干扰
- 回调在锁外执行，避免用户回调引发死锁

### 9.2 内存管理
- 每轮释放后立即清理 `waiters` 映射表，避免内存泄漏
- 使用定时器 (`time.AfterFunc`) 并在正常释放时停止，确保资源回收
- waiter channel 使用带缓冲模式，避免发送方阻塞

### 9.3 回调执行顺序
- 回调在状态更新后、goroutine 通知前执行
- 回调在锁外执行，保证并发安全
- 回调错误通过 channel 传递给每个等待者，保证每个等待者都能收到相同的错误信息

### 9.4 超时语义
- 超时 goroutine 立即返回，不参与后续的屏障同步
- `effectiveNeeded` 随超时递减，避免死等永远不会到达的 goroutine
- 超时过程是原子的，使用锁保护确保不会与正常释放路径冲突

### 9.5 CyclicBarrier 设计
- 采用组合而非继承的方式，职责更清晰
- 使用独立的互斥锁管理循环状态，避免与基础屏障锁的嵌套
- 自动循环语义：每轮第一个到达的 goroutine 触发轮次切换和重置
- 轮次回调支持按轮次处理不同的业务逻辑
