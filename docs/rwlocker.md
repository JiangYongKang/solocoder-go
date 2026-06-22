# 增强型读写锁模块 (RWLocker)

## 1. 模块概述

读写锁（Read-Write Lock）是一种常用的并发同步原语，它允许多个读操作并发执行，但写操作需要独占访问。标准库 `sync.RWMutex` 提供了基础的读写锁功能，但在实际生产环境中，我们通常需要更多的高级特性来帮助排查性能问题和避免死锁。

本模块对标准库 `sync.RWMutex` 进行了增强封装，提供了以下核心功能：
- 读锁升级为写锁的能力
- 加锁超时检测
- 锁竞争统计
- 死锁自动检测与告警

## 2. 功能特性

### 2.1 读锁升级为写锁

在持有读锁的场景下，有时需要将读锁升级为写锁以进行数据修改。本模块提供了两种升级模式：

- **非阻塞模式（UpgradeNonBlocking）**：尝试立即升级，如果当前有其他读锁持有者，则立即返回失败错误，不会阻塞等待
- **阻塞模式（UpgradeBlocking）**：释放自身持有的读锁，然后等待其他所有读锁持有者释放锁，最后获取写锁。支持设置超时时间

升级操作保证了原子性：在非阻塞模式下，只有当当前 goroutine 是唯一的读锁持有者时才能升级成功，避免了经典的"写者饥饿"问题。

### 2.2 锁持有超时检测

支持为读锁和写锁分别配置超时时间：
- `ReadTimeout`：获取读锁的最大等待时间
- `WriteTimeout`：获取写锁的最大等待时间
- 超时时间为 0 表示无限等待（与标准库行为一致）
- 超时后返回 `TimeoutError`，调用方可通过 `errors.Is(err, ErrLockTimeout)` 判断

**超时实现机制**：

两种锁均使用 `done` channel + `time.NewTimer` + `select` 模式实现超时控制：
- 锁获取在后台 goroutine 中执行，通过 `close(done)` 通知成功
- 超时分支启动清理 goroutine 等待后台锁获取完成后立即释放，避免 goroutine 泄漏

**写锁（Lock）超时清理**：

写锁超时后的清理逻辑较为简单：
1. 后台 goroutine 通过 `mu.Lock()` 获取写锁后 `close(done)`
2. 若超时分支选中 `timer.C`，启动清理 goroutine：等待 `<-done` 后调用 `mu.Unlock()` 释放写锁
3. 写锁不涉及 `readerCount` 和 `upgradeCond`，清理只需释放底层互斥锁即可

**读锁（RLock）超时清理**：

读锁超时后的清理逻辑更复杂，需要回退 `readerCount` 并处理升级等待通知：
1. 后台 goroutine 先通过 `waitForWriterWaiting()` 检查 `writerWaiting` 标记，确认无写者等待升级后递增 `readerCount`，再通过 `mu.RLock()` 获取读锁后 `close(done)`
2. 若超时分支选中 `timer.C`，启动清理 goroutine：等待 `<-done` 后执行以下步骤：
   - 获取 `upgradeMu`，递减 `readerCount`
   - 若 `writerWaiting == true` 且 `readerCount == 0`，调用 `upgradeCond.Broadcast()` 通知等待升级的写者
   - 释放 `upgradeMu`
   - 调用 `mu.RUnlock()` 释放底层读锁
3. 之所以需要回退 `readerCount` 并通知 `upgradeCond`，是因为后台 goroutine 在获取 `mu.RLock` 前已递增了 `readerCount`，而调用方并未实际持有读锁，若不回退将导致等待升级的写者永远看不到 `readerCount` 归零

### 2.3 锁竞争统计

自动收集锁的使用统计数据，帮助排查性能瓶颈：
- 读锁：请求次数、成功次数、等待总时长、最长等待时间
- 写锁：请求次数、成功次数、等待总时长、最长等待时间
- 锁升级：请求次数、成功次数、等待总时长、最长等待时间
- 异常统计：死锁检测次数、超时次数

提供 `GetStats()` 方法获取统计快照，`ResetStats()` 方法重置统计数据。统计功能可通过配置启用或禁用。

### 2.4 死锁自动检测

提供两种层级的死锁防护：

**单 goroutine 死锁检测**：
- 在加锁操作时检测当前 goroutine 是否已持有该锁
- 读锁支持重入（同一 goroutine 可多次获取读锁）
- 写锁不可重入，读锁持有状态下不可获取写锁，写锁持有状态下不可获取读锁
- 检测到重复加锁时立即返回 `DeadlockError`，不会阻塞

**锁持有时间阈值告警**：
- 配置 `HoldDurationWarn` 设置锁持有时间阈值
- 释放锁时如果持有时间超过阈值，触发 `OnHoldDurationWarn` 回调
- 辅助发现锁持有时间过长的情况，间接排查多 goroutine 间的死锁

## 3. 核心结构体

### 3.1 RWLocker

增强型读写锁的主结构体，封装了 `sync.RWMutex` 并提供所有扩展功能。

| 字段 | 类型 | 职责 |
|------|------|------|
| `name` | `string` | 锁的名称，用于标识和日志输出 |
| `mu` | `sync.RWMutex` | 底层标准读写锁 |
| `readTimeout` | `time.Duration` | 读锁获取超时时间 |
| `writeTimeout` | `time.Duration` | 写锁获取超时时间 |
| `enableDeadlockDetect` | `bool` | 是否启用死锁检测 |
| `enableStats` | `bool` | 是否启用竞争统计 |
| `holdDurationWarn` | `time.Duration` | 锁持有时间告警阈值 |
| `onHoldDurationWarn` | `func(*HoldDurationWarning)` | 持有时间超阈值回调函数 |
| `statsMu` | `sync.Mutex` | 保护统计数据的互斥锁 |
| `stats` | `Stats` | 锁竞争统计数据 |
| `readerCount` | `int` | 当前读锁持有者数量 |
| `writerWaiting` | `bool` | 是否有写者在等待升级 |
| `writerActive` | `bool` | 是否有写者持有锁 |
| `upgradeMu` | `sync.Mutex` | 保护升级相关状态的互斥锁 |
| `upgradeCond` | `*sync.Cond` | 锁升级等待的条件变量 |

主要方法：

| 方法 | 返回类型 | 说明 |
|------|----------|------|
| `New(cfg *Config)` | `*RWLocker` | 创建新的增强型读写锁 |
| `RLock()` | `error` | 获取读锁，支持超时 |
| `RUnlock()` | `error` | 释放读锁 |
| `Lock()` | `error` | 获取写锁，支持超时 |
| `Unlock()` | `error` | 释放写锁 |
| `TryRLock()` | `(bool, error)` | 非阻塞尝试获取读锁 |
| `TryLock()` | `(bool, error)` | 非阻塞尝试获取写锁 |
| `TryUpgrade(mode, timeout)` | `error` | 尝试将读锁升级为写锁 |
| `GetStats()` | `*Stats` | 获取锁竞争统计快照 |
| `ResetStats()` | 无 | 重置统计数据 |
| `ReaderCount()` | `int` | 获取当前读锁持有者数量 |
| `IsWriterActive()` | `bool` | 是否有写者持有锁 |
| `Name()` | `string` | 获取锁名称 |

### 3.2 Config

RWLocker 的配置结构体。

| 字段 | 类型 | 说明 | 默认值 |
|------|------|------|--------|
| `Name` | `string` | 锁名称 | `""` |
| `ReadTimeout` | `time.Duration` | 读锁超时时间（0 表示无限等待） | `0` |
| `WriteTimeout` | `time.Duration` | 写锁超时时间（0 表示无限等待） | `0` |
| `EnableDeadlockDetect` | `bool` | 是否启用死锁检测 | `true` |
| `EnableStats` | `bool` | 是否启用竞争统计 | `true` |
| `HoldDurationWarn` | `time.Duration` | 锁持有时间告警阈值（0 表示禁用） | `0` |
| `OnHoldDurationWarn` | `func(*HoldDurationWarning)` | 持有时间超阈值回调 | `nil` |

### 3.3 Stats

锁竞争统计数据结构体，所有字段都是累计值。

| 字段 | 类型 | 说明 |
|------|------|------|
| `ReadRequests` | `uint64` | 读锁请求总次数 |
| `ReadSuccess` | `uint64` | 读锁获取成功次数 |
| `ReadWaitTotal` | `time.Duration` | 读锁等待总时长 |
| `ReadWaitMax` | `time.Duration` | 读锁最长单次等待时间 |
| `WriteRequests` | `uint64` | 写锁请求总次数 |
| `WriteSuccess` | `uint64` | 写锁获取成功次数 |
| `WriteWaitTotal` | `time.Duration` | 写锁等待总时长 |
| `WriteWaitMax` | `time.Duration` | 写锁最长单次等待时间 |
| `UpgradeRequests` | `uint64` | 锁升级请求总次数 |
| `UpgradeSuccess` | `uint64` | 锁升级成功次数 |
| `UpgradeWaitTotal` | `time.Duration` | 锁升级等待总时长 |
| `UpgradeWaitMax` | `time.Duration` | 锁升级最长单次等待时间 |
| `DeadlockDetected` | `uint64` | 死锁检测触发次数 |
| `TimeoutCount` | `uint64` | 锁获取超时总次数 |

### 3.4 错误类型

#### TimeoutError

锁获取超时错误。

| 字段 | 类型 | 说明 |
|------|------|------|
| `LockType` | `string` | 锁类型："read" 或 "write" |
| `Timeout` | `time.Duration` | 配置的超时时间 |

#### DeadlockError

死锁检测错误。

| 字段 | 类型 | 说明 |
|------|------|------|
| `LockType` | `string` | 正在尝试获取的锁类型 |
| `GoroutineID` | `int64` | 发生死锁的 goroutine ID |
| `AlreadyHeld` | `string` | 已持有的锁类型 |

#### UpgradeError

锁升级失败错误。

| 字段 | 类型 | 说明 |
|------|------|------|
| `Reason` | `string` | 升级失败原因 |
| `ReaderCount` | `int` | 当前其他读锁持有者数量 |
| `Blocking` | `bool` | 是否为阻塞模式 |

#### HoldDurationWarning

锁持有时间超阈值告警。

| 字段 | 类型 | 说明 |
|------|------|------|
| `LockType` | `string` | 锁类型 |
| `HoldDuration` | `time.Duration` | 实际持有时间 |
| `Threshold` | `time.Duration` | 配置的告警阈值 |
| `GoroutineID` | `int64` | 持有锁的 goroutine ID |

## 4. 锁升级工作机制

### 4.1 非阻塞升级流程

```
调用 TryUpgrade(UpgradeNonBlocking, 0)
    ↓
检查当前 goroutine 是否持有读锁
    ↓ 否
返回 UpgradeError("current goroutine does not hold read lock")
    ↓ 是
获取 upgradeMu，计算其他读锁数量 = 总读锁数 - 当前 goroutine 持有的读锁数
    ↓
其他读锁数量 > 0？
    ├─ 是 → 释放 upgradeMu，返回 UpgradeError("other readers hold the lock")
    └─ 否
        ↓
        设置 readerCount = 0，标记 writerWaiting = true
        ↓
        释放 upgradeMu
        ↓
        释放所有自身持有的读锁（mu.RUnlock N 次）
        ↓
        获取写锁（mu.Lock）
        ↓
        设置 writerActive = true，清除 writerWaiting
        ↓
        注册写锁持有者，更新统计
        ↓
        返回成功（nil）
```

### 4.2 阻塞升级流程

```
调用 TryUpgrade(UpgradeBlocking, timeout)
    ↓
检查当前 goroutine 是否持有读锁
    ↓ 否
返回 UpgradeError
    ↓ 是
记录 incUpgradeRequest（仅有效请求计入统计）
    ↓
获取 upgradeMu，计算其他读锁数量
    ↓
其他读锁数量 > 0？
    ├─ 否 → 走非阻塞升级的成功路径
    └─ 是
        ↓
        标记 writerWaiting = true，readerCount -= 自身读锁数
        ↓
        释放 upgradeMu
        ↓
        释放所有自身持有的读锁（mu.RUnlock N 次）
        ↓
        此时 RLock/TryRLock 入口检查 writerWaiting，阻止新读者进入
        ↓
        启动超时定时器（如配置了 timeout）
        ↓
        重新获取 upgradeMu
        ↓
        循环等待 readerCount == 0：
            ├─ 超时 → 清除 writerWaiting，恢复 readerCount，Broadcast 唤醒阻塞的 RLock，
            │         重新获取读锁，返回超时错误
            └─ 被唤醒且 readerCount == 0 → 退出循环
        ↓
        清除 writerWaiting，Broadcast 唤醒阻塞的 RLock
        ↓
        获取写锁（mu.Lock）
        ↓
        标记 writerActive = true
        ↓
        注册写锁持有者，更新统计
        ↓
        返回成功（nil）
```

**关键设计要点**：
- 升级前先释放自身读锁，避免与其他等待写锁的 goroutine 产生死锁
- 使用 `upgradeCond` 条件变量在读者释放锁时通知等待升级的写者
- 阻塞模式下如果超时，会重新获取读锁保证状态一致性
- **写者饥饿防护**：阻塞升级设置 `writerWaiting` 标记后，`RLock` 和 `TryRLock` 入口会检查该标记，阻止新读者进入，确保 `readerCount` 能归零从而升级成功
- 升级完成或超时退出时，均会调用 `upgradeCond.Broadcast()` 唤醒所有因 `writerWaiting` 而阻塞的读锁请求
- **统计准确性**：`incUpgradeRequest()` 仅在读锁持有校验通过后才调用，无效升级请求不计入统计

## 5. 写者饥饿防护机制

当 goroutine 调用 `TryUpgrade(UpgradeBlocking)` 等待其他读者释放锁时，如果新读者不断进入，`readerCount` 可能永远无法归零，导致升级 goroutine 无限等待，即"写者饥饿"问题。

### 5.1 防护策略

本模块通过 `writerWaiting` 标记实现写者饥饿防护：

**RLock 入口检查**：
- 无超时路径：`waitForWriterWaiting()` 在 `upgradeMu` 保护下循环检查 `writerWaiting`，若为 true 则在 `upgradeCond` 上等待，直到升级完成或超时后被唤醒
- 有超时路径：后台 goroutine 同样先通过 `waitForWriterWaiting` 逻辑，超时后启动清理 goroutine 回退 `readerCount` 并释放底层锁

**TryRLock 入口检查**：
- 在 `upgradeMu` 保护下检查 `writerWaiting`，若为 true 则直接返回 `(false, nil)`，不增加 `readerCount`
- 若底层 `mu.TryRLock()` 失败，回退已增加的 `readerCount` 并在必要时通知 `upgradeCond`

**升级完成/退出时广播**：
- 升级成功：设置 `writerWaiting = false` 后调用 `upgradeCond.Broadcast()`，唤醒所有等待的 RLock
- 升级超时退出：同样清除 `writerWaiting` 并 `Broadcast`，确保被阻塞的 RLock 能继续执行

### 5.2 流程示意

```
TryUpgrade 设置 writerWaiting = true
    ↓
RLock 调用 → waitForWriterWaiting()
    → writerWaiting == true → upgradeCond.Wait() 阻塞
    ↓
TryRLock 调用 → 检查 writerWaiting == true → 返回 false
    ↓
最后一个 RUnlock → readerCount == 0 → upgradeCond.Broadcast()
    ↓
TryUpgrade 醒来 → 获取写锁
    ↓
TryUpgrade 完成 → writerWaiting = false → upgradeCond.Broadcast()
    ↓
被阻塞的 RLock 醒来 → writerWaiting == false → 继续获取读锁
```

## 6. 死锁检测工作机制

### 6.1 单 goroutine 死锁检测

通过全局的 goroutine → 锁持有者注册表实现：

```
goroutineLocks (map[int64]*goroutineLockInfo)
    ↓
每个 goroutine 对应一个 goroutineLockInfo
    ↓
包含 holders: map[*RWLocker]*lockHolder
    ↓
记录该 goroutine 对每个 RWLocker 的持有状态
    ↓
lockHolder 包含：goroutineID、lockType、acquireTime、count
```

**检测规则**（调用加锁方法时执行）：

| 已持有 \ 请求 | 读锁 | 写锁 |
|---------------|------|------|
| 无 | 允许 | 允许 |
| 读锁 | 允许（重入，count++） | 禁止（返回死锁错误） |
| 写锁 | 禁止（返回死锁错误） | 禁止（返回死锁错误） |

### 6.2 锁持有时间告警

在释放锁时检查持有时长：

```
调用 Unlock() / RUnlock()
    ↓
从注册表中取出该 goroutine 对此锁的 holder 记录
    ↓
holder.count > 1？
    ├─ 是 → count--，直接返回（重入场景）
    └─ 否
        ↓
        计算 holdDuration = time.Now() - holder.acquireTime
        ↓
        holdDuration > HoldDurationWarn 且配置了回调？
            ├─ 是 → 调用 OnHoldDurationWarn(&HoldDurationWarning{...})
            └─ 否 → 跳过
        ↓
        从注册表中移除该 holder 记录
```

## 7. 使用示例

### 7.1 基础使用

```go
package main

import (
	"fmt"
	"solocoder-go/internal/rwlocker"
	"time"
)

func main() {
	// 使用默认配置创建
	lock := rwlocker.New(nil)

	// 获取读锁
	if err := lock.RLock(); err != nil {
		fmt.Println("获取读锁失败:", err)
		return
	}
	fmt.Println("当前读者数量:", lock.ReaderCount())
	lock.RUnlock()

	// 获取写锁
	if err := lock.Lock(); err != nil {
		fmt.Println("获取写锁失败:", err)
		return
	}
	fmt.Println("写者是否活跃:", lock.IsWriterActive())
	lock.Unlock()
}
```

### 7.2 带超时配置

```go
lock := rwlocker.New(&rwlocker.Config{
	Name:         "user-cache",
	ReadTimeout:  100 * time.Millisecond,
	WriteTimeout: 500 * time.Millisecond,
})

if err := lock.Lock(); err != nil {
	if errors.Is(err, rwlocker.ErrLockTimeout) {
		var timeoutErr *rwlocker.TimeoutError
		if errors.As(err, &timeoutErr) {
			log.Printf("获取写锁超时，超时时间: %v", timeoutErr.Timeout)
		}
		// 执行降级逻辑
		return
	}
}
defer lock.Unlock()
```

### 7.3 读锁升级

```go
lock := rwlocker.New(nil)

// 先获取读锁读取数据
if err := lock.RLock(); err != nil {
	return err
}

// 检查数据是否需要修改
if !dataNeedsUpdate() {
	lock.RUnlock()
	return nil
}

// 尝试非阻塞升级
err := lock.TryUpgrade(rwlocker.UpgradeNonBlocking, 0)
if err != nil {
	// 升级失败，先释放读锁再获取写锁
	lock.RUnlock()
	if err := lock.Lock(); err != nil {
		return err
	}
}
defer lock.Unlock()

// 执行写操作
updateData()
```

### 7.4 竞争统计与监控

```go
lock := rwlocker.New(&rwlocker.Config{
	Name:        "order-db",
	EnableStats: true,
})

// 定期输出统计信息
go func() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		stats := lock.GetStats()
		if stats == nil {
			continue
		}
		log.Printf("[%s] 读锁: %d/%d 请求/成功, 最长等待: %v",
			lock.Name(),
			stats.ReadRequests,
			stats.ReadSuccess,
			stats.ReadWaitMax,
		)
		log.Printf("[%s] 写锁: %d/%d 请求/成功, 最长等待: %v",
			lock.Name(),
			stats.WriteRequests,
			stats.WriteSuccess,
			stats.WriteWaitMax,
		)
		log.Printf("[%s] 死锁检测: %d, 超时: %d",
			lock.Name(),
			stats.DeadlockDetected,
			stats.TimeoutCount,
		)
		lock.ResetStats()
	}
}()
```

### 7.5 锁持有时间告警

```go
lock := rwlocker.New(&rwlocker.Config{
	Name:             "critical-section",
	HoldDurationWarn: 2 * time.Second,
	OnHoldDurationWarn: func(w *rwlocker.HoldDurationWarning) {
		log.Printf("警告: goroutine %d 持有 %s 锁 %v，超过阈值 %v",
			w.GoroutineID,
			w.LockType,
			w.HoldDuration,
			w.Threshold,
		)
		// 可以在此处输出堆栈信息辅助排查
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		log.Printf("当前堆栈:\n%s", buf[:n])
	},
})
```

## 8. 错误常量速查

| 常量 | 说明 |
|------|------|
| `ErrLockTimeout` | 锁获取超时，可配合 `TimeoutError` 使用 |
| `ErrDeadlockDetected` | 检测到死锁，可配合 `DeadlockError` 使用 |
| `ErrUpgradeFailed` | 锁升级失败，可配合 `UpgradeError` 使用 |
| `ErrNotHeld` | 当前 goroutine 未持有该锁却尝试释放 |
| `ErrInvalidTimeout` | 配置的超时时间无效（负数） |
| `ErrHoldDurationExceeded` | 锁持有时间超过阈值（告警用） |

## 9. 性能说明

- 启用死锁检测会在每次加锁/解锁时增加 goroutine ID 获取和注册表查询的开销，对性能敏感的场景可通过 `EnableDeadlockDetect: false` 关闭
- 启用统计功能会在每次加锁/解锁时增加原子计数操作，开销较小
- goroutine ID 通过解析 `runtime.Stack()` 输出获取，该操作有一定开销，高并发场景下如需极致性能可考虑关闭死锁检测
