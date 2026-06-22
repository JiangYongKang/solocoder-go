# 对象池 (ObjectPool) 模块需求文档

## 1. 模块概述

对象池是一个基于 Go 泛型的通用对象复用组件，用于管理和复用任意类型的对象。通过在内存中维护一组已创建的对象，避免了频繁创建和销毁对象带来的性能开销，同时提供了空闲对象自动回收和最大容量限制机制。

本模块使用 Go 1.18+ 泛型支持任意对象类型，通过可配置的工厂函数（Factory）和销毁函数（Destroy）与具体对象类型解耦，归还时对象状态由调用方负责重置。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 对象借出 (Acquire) | 从池中获取一个可用对象，优先复用空闲对象，无空闲且未达上限时自动创建；池满时支持阻塞等待或立即返回错误两种策略 |
| F2 | 对象归还 (Release) | 将使用完毕的对象归还到池中以供后续复用，归还时对象状态由调用方负责重置 |
| F3 | 空闲对象自动回收 | 后台定时检查池中空闲对象，将超过最大空闲时间的对象从池中移除并调用销毁回调释放资源 |
| F4 | 最大池容量限制 | 池中总对象数（已借出 + 空闲）不得超过配置的最大容量，池满时借出请求可阻塞等待或直接返回错误 |
| F5 | 对象创建工厂函数注册 | 创建池时注册工厂函数，当池中无空闲对象且总对象数未达上限时自动调用工厂函数创建新对象 |
| F6 | 池关闭 (Close) | 关闭对象池，释放所有对象资源，停止后台回收协程 |
| F7 | 状态查询 | 查询对象池的总对象数、空闲对象数、活跃对象数 |

## 3. 核心结构体与职责

### 3.1 Config[T] - 对象池配置

```go
type Config[T any] struct {
    MaxCap          int           // 最大对象数，池中允许存在的对象上限（必填，必须 > 0）
    MaxIdleTime     time.Duration // 最大空闲时间，超过此时间的空闲对象被回收（0 表示不回收）
    WaitTimeout     time.Duration // 借出等待超时，0 表示不等待直接返回错误
    CleanupInterval time.Duration // 回收检查间隔，默认为 MaxIdleTime / 2
    Factory         Factory[T]    // 对象工厂函数（必填）
    Destroy         DestroyFunc[T] // 对象销毁函数（默认空实现）
}
```

**配置约束与默认值：**
- `Factory` 必须提供，否则 `NewPool` 返回错误
- `MaxCap` 必须大于 0，否则 `NewPool` 返回错误
- `MaxIdleTime` 默认为 0（不启用自动回收）
- `CleanupInterval` 默认为 `MaxIdleTime / 2`；若计算结果 ≤ 0 则取 `MaxIdleTime`
- `WaitTimeout` 默认为 0（池满时立即返回 `ErrPoolExhausted`）
- `Destroy` 默认空实现（`func(T) {}`），不做任何资源释放

### 3.2 Pool[T] - 对象池主体

```go
type Pool[T any] struct {
    cfg      Config[T]              // 配置快照
    mu       sync.Mutex             // 保护内部状态的互斥锁
    cond     *sync.Cond             // 条件变量，用于借出等待唤醒
    idleList *list.List             // 空闲对象双向链表（LRU 队列）
    active   map[any]*idleEntry[T]  // 活跃对象集合（已借出未归还）
    count    int32                  // 当前总对象数（原子操作）
    closed   bool                   // 池是否已关闭
    stopCh   chan struct{}          // 后台协程停止信号
    wg       sync.WaitGroup         // 后台协程同步等待组
}
```

**主要职责：**
- 维护对象的生命周期状态（空闲/活跃）
- 协调并发的对象借出与归还操作
- 驱动空闲对象回收等后台协程
- 保证线程安全，通过互斥锁和条件变量实现同步

### 3.3 idleEntry[T] - 对象元数据包装

```go
type idleEntry[T any] struct {
    obj      T         // 实际对象
    lastUsed time.Time // 最后一次使用时间（用于空闲超时判断）
}
```

**主要职责：**
- 包装实际对象，附加生命周期管理所需的时间戳
- 在空闲链表与活跃集合之间传递时保留元数据

### 3.4 类型定义

```go
type Factory[T any] func() (T, error)     // 对象工厂函数签名
type DestroyFunc[T any] func(T)           // 对象销毁函数签名
```

### 3.5 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrPoolClosed` | 对象池已关闭 | 已关闭的池上调用 Acquire/Release |
| `ErrPoolExhausted` | 对象池耗尽 | 无空闲对象且达到 MaxCap，WaitTimeout=0 或等待超时 |
| `ErrNotBorrowed` | 对象未借出 | 归还的对象不属于此池（外部对象、重复归还等） |

## 4. 对象生命周期管理流程

### 4.1 对象池创建

```
NewPool[T](config)
   │
   ├─ 参数校验 → 失败则返回 error
   │     ├─ Factory == nil → error
   │     └─ MaxCap <= 0 → error
   │
   ├─ 初始化默认值
   │     ├─ CleanupInterval = MaxIdleTime / 2（若未指定）
   │     └─ Destroy = func(T) {}（若未指定）
   │
   ├─ 初始化内部结构（idleList、active、stopCh 等）
   │
   ├─ MaxIdleTime > 0 → 启动 cleanupLoop 协程
   │
   └─ 返回 *Pool[T]
```

### 4.2 对象借出流程 (Acquire)

```
Acquire()
   │
   ├─ [外层循环：直到获取成功/错误]
   │
   ├─ mu.Lock() → 检查 closed → 返回 ErrPoolClosed
   │
   ├─ 尝试从 idleList 获取空闲对象
   │     └─ 取出 idleEntry
   │     └─ 更新 lastUsed = now
   │     └─ 加入 active，返回 obj
   │
   ├─ 无空闲对象，尝试创建新对象
   │     └─ count < MaxCap
   │     └─ CAS count++（防止并发超建）
   │     └─ mu.Unlock(), 调用 Factory()
   │     ├─ 失败：count--, 返回 error
   │     └─ 成功：封装 idleEntry，加入 active，返回 obj
   │
   ├─ 无空闲且达到 MaxCap
   │     ├─ WaitTimeout <= 0 → 返回 ErrPoolExhausted
   │     │
   │     └─ WaitTimeout > 0 → 等待模式
   │           ├─ deadline = now + WaitTimeout（仅设置一次）
   │           ├─ 启动超时协程：到时获取锁 → Broadcast → 释放锁
   │           ├─ [内层循环：所有检查在持有锁状态下执行]
   │           │     ├─ 检查 closed → 返回 ErrPoolClosed
   │           │     ├─ 检查 time.Now().After(deadline) → 返回 ErrPoolExhausted
   │           │     ├─ 检查 idleList>0 或 count<MaxCap → 跳出等待
   │           │     └─ cond.Wait() → 挂起等待 Signal/Broadcast
   │           └─ 条件满足 → mu.Unlock()，回到外层循环重试
   │
   └─ 返回 (obj, error)
```

**等待唤醒机制说明：**
- `deadline` 在 `Acquire()` 函数最外层声明，整个调用生命周期内只设置一次，确保超时时间不被重置
- 超时检查 `time.Now().After(deadline)` 在持有锁的状态下执行，保证原子性
- 超时协程触发时会先获取锁再 Broadcast，确保等待者被唤醒后能立即检测到超时

### 4.3 对象归还流程 (Release)

```
Release(obj)
   │
   ├─ mu.Lock() → 检查 closed → 返回 ErrPoolClosed
   │
   ├─ 从 active 查找 idleEntry 元数据
   │     └─ 不存在 → 返回 ErrNotBorrowed（外部对象、重复归还等）
   │
   ├─ 从 active 中删除
   ├─ 更新 lastUsed = now
   ├─ idleEntry 加入 idleList 头部（标记为最近使用）
   │
   ├─ cond.Signal()（唤醒一个等待者）
   ├─ mu.Unlock()
   │
   └─ 返回 nil
```

**归还时对象状态重置说明：**
- 对象归还时，池不负责重置对象状态
- 调用方在调用 `Release` 前应自行将对象状态重置为可复用状态
- 这种设计将状态管理职责交给更了解对象内部结构的调用方，避免池对对象内部实现产生依赖

### 4.4 空闲对象自动回收流程

```
cleanupLoop（后台协程，CleanupInterval 驱动）
   │
   └─ [ticker.C]
      │
      ├─ mu.Lock() → closed → 直接返回
      │
      ├─ 遍历 idleList
      │     └─ now.Sub(lastUsed) > MaxIdleTime
      │           └─ 从 idleList 移除
      │           └─ 加入 expired 列表
      │           └─ count--
      │
      ├─ len(expired) > 0 → cond.Broadcast()（唤醒阻塞的 Acquire 调用者）
      ├─ mu.Unlock()
      │
      └─ 遍历 expired，逐个调用 Destroy(obj)
```

**回收唤醒机制说明：**
- 当有空闲对象因超时而被回收时，总对象数 `count` 减少，释放了创建新对象的容量
- 此时调用 `cond.Broadcast()` 立即唤醒所有阻塞中的 `Acquire` 调用者，使它们有机会创建新对象
- 避免了等待者只能等到自身超时后才能重试的不必要延迟

### 4.5 对象池关闭流程

```
Close()
   │
   ├─ mu.Lock() → 已关闭 → 直接返回
   │
   ├─ closed = true
   ├─ close(stopCh)（通知后台协程退出）
   ├─ cond.Broadcast()（唤醒所有等待中的 Acquire 调用）
   │
   ├─ 收集 idleList 中所有 obj 到 toDestroy
   ├─ 收集 active 中所有 obj 到 toDestroy
   ├─ 清空 idleList、active，count=0
   │
   ├─ mu.Unlock()
   │
   ├─ 遍历 toDestroy，逐个调用 Destroy(obj)
   │
   └─ wg.Wait()（等待所有后台协程退出）
```

## 5. 泛型设计说明

本模块使用 Go 1.18+ 泛型实现，类型参数 `T` 支持任意对象类型：

- **指针类型**：`Pool[*MyStruct]` - 适用于需要引用语义的对象
- **值类型**：`Pool[int]`、`Pool[string]` - 适用于简单值对象
- **接口类型**：`Pool[io.Closer]` - 适用于多态对象

工厂函数 `Factory[T]` 返回 `(T, error)`，销毁函数 `DestroyFunc[T]` 接收 `T` 参数，与池的泛型参数一致。

**关于 nil 对象的处理：**
由于 Go 泛型中 `any(nil_pointer)` 不等于 `nil`（接口持有类型信息），本模块不在 `Release` 中做特殊的 nil 检查。所有未通过 `Acquire` 获取的对象（包括 nil 对象）在 `Release` 时都会返回 `ErrNotBorrowed`。

## 6. LRU 策略说明

空闲对象采用双向链表 (`container/list`) 维护 LRU 顺序：

- **归还 (Release)**：将对象插入链表头部（最近使用）
- **借出 (Acquire)**：从链表头部开始取（优先取最近使用的）
- **超量回收**：从链表尾部开始移除（最久未使用的优先被回收）

这种策略保证了热点对象被持续复用，而冷对象在资源紧张时优先被释放。

## 7. 线程安全与性能优化说明

对象池是完全并发安全的：
- 所有内部状态访问受 `mu` 互斥锁保护
- 总对象数 `count` 使用 `atomic` 原子操作，在创建新对象的 CAS 阶段减少锁持有时间
- 等待唤醒使用 `sync.Cond`，避免忙等待导致的 CPU 空转
- 后台协程通过 `stopCh` + `WaitGroup` 实现优雅退出

**关键性能优化：**
- **原子超时检查**：等待模式下的超时检查在持有锁的状态下执行，确保与 `cond.Wait()` 的原子性，避免"假等待"问题
- **回收即时唤醒**：空闲超时回收后立即调用 `cond.Broadcast()` 唤醒阻塞的 Acquire 调用者，减少请求延迟
- **CAS 创建控制**：使用 `atomic.CompareAndSwapInt32` 控制并发创建，避免超出 MaxCap

## 8. 使用示例

### 8.1 基础使用：缓冲区对象池

```go
package main

import (
    "fmt"
    "sync/atomic"
    "time"
    "solocoder-go/internal/objectpool"
)

type Buffer struct {
    ID   int
    Data []byte
}

func main() {
    var idCounter int64

    pool, err := objectpool.NewPool[*Buffer](objectpool.Config[*Buffer]{
        MaxCap:      10,
        MaxIdleTime: 5 * time.Minute,
        WaitTimeout: 3 * time.Second,
        Factory: func() (*Buffer, error) {
            id := atomic.AddInt64(&idCounter, 1)
            return &Buffer{ID: int(id), Data: make([]byte, 1024)}, nil
        },
        Destroy: func(b *Buffer) {
            b.Data = nil
        },
    })
    if err != nil {
        panic(err)
    }
    defer pool.Close()

    // 借出对象
    buf, err := pool.Acquire()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Using buffer #%d\n", buf.ID)

    // 使用对象...
    buf.Data[0] = 'H'

    // 使用完毕，重置状态后归还
    for i := range buf.Data {
        buf.Data[i] = 0
    }
    if err := pool.Release(buf); err != nil {
        panic(err)
    }
}
```

### 8.2 非阻塞模式（WaitTimeout=0）

```go
pool, _ := objectpool.NewPool[*Buffer](objectpool.Config[*Buffer]{
    MaxCap:      5,
    WaitTimeout: 0, // 无可用对象时立即返回 ErrPoolExhausted
    Factory:     factory,
})
defer pool.Close()

obj, err := pool.Acquire()
if err == objectpool.ErrPoolExhausted {
    // 处理对象池耗尽：降级、排队、或返回业务错误
    return
}
```

### 8.3 值类型对象池

```go
var counter int64
pool, _ := objectpool.NewPool[int](objectpool.Config[int]{
    MaxCap: 100,
    Factory: func() (int, error) {
        return int(atomic.AddInt64(&counter, 1)), nil
    },
})
defer pool.Close()

val, _ := pool.Acquire()
// 使用 val...
pool.Release(val)
```

### 8.4 监控对象池状态

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        total  := pool.Len()
        idle   := pool.IdleCount()
        active := pool.ActiveCount()
        log.Printf("ObjectPool: total=%d idle=%d active=%d", total, idle, active)
    }
}()
```

## 9. 文件结构

```
internal/objectpool/
├── pool.go       # 对象池核心实现
└── pool_test.go  # 单元测试（覆盖正常流程、边界条件、异常分支）

docs/
└── objectpool.md # 本文档
```
