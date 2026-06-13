# 消息去重中间件 (Dedup) 模块需求文档

## 1. 模块概述

消息去重中间件是一个基于内存的消息幂等性保障组件，用于在消息投递链路中识别并丢弃重复消息。通过维护一个滑动时间窗口内的消息 ID 集合，确保同一条消息在窗口时间内不会被重复消费，同时自动清理过期记录以避免内存无限增长。

本模块适用于消息队列、事件总线、任务调度等需要保证"恰好一次"语义的场景，通过可配置的窗口大小和清理间隔在去重精度与内存占用之间取得平衡。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 幂等检查与标记 (CheckAndMark) | 检查消息 ID 是否已处理，未处理则标记为已处理并通过，已处理则直接拒绝 |
| F2 | 去重窗口滑动 | 只维护最近一段时间窗口内的消息 ID，超出窗口的记录自动失效 |
| F3 | 访问时间刷新 (Touch on Access) | 重复消息在窗口内被再次访问时，刷新其时间戳并移动到窗口末尾，延长有效期 |
| F4 | 手动过期清理 (CleanExpired) | 主动扫描并清理超过窗口时间的过期记录，返回清理数量 |
| F5 | 后台定时清理 | 启动后台协程按配置间隔自动执行过期清理，无需调用方手动触发 |
| F6 | 存在性查询 (Contains) | 查询消息 ID 是否在有效窗口内，不改变其状态 |
| F7 | 全量清空 (Clear) | 清空所有去重记录，重置去重器状态 |
| F8 | 数量统计 (Count) | 查询当前有效窗口内的记录总数 |
| F9 | 生命周期管理 (Start/Stop) | 启动和停止后台清理协程，支持幂等调用 |

## 3. 核心结构体与职责

### 3.1 Config - 去重器配置

```go
type Config struct {
    WindowSize    time.Duration // 去重窗口大小，记录在此时间后视为过期
    CleanInterval time.Duration // 后台清理协程的执行间隔
}
```

**配置约束与默认值：**
- `WindowSize`：必须大于 0，默认为 5 分钟。设置为 0 或负数时自动使用默认值
- `CleanInterval`：必须大于 0，默认为 `WindowSize / 5`（最少 1 秒）。设置为 0 或负数时自动根据窗口大小推导
- `CleanInterval` 不宜大于 `WindowSize`，否则过期记录可能长时间驻留内存
- 推荐配置：`CleanInterval` 为 `WindowSize` 的 1/5 ~ 1/2，在清理频率和 CPU 开销间取得平衡

### 3.2 Deduplicator - 去重器主体

```go
type Deduplicator struct {
    cfg      Config              // 配置快照
    mu       sync.Mutex          // 保护内部状态的互斥锁
    idMap    map[string]*list.Element // 消息ID → 链表节点的快速查找索引
    idList   *list.List          // 按插入/访问顺序排列的双向链表（FIFO 队列）
    running  bool                // 后台清理协程是否运行中
    stopCh   chan struct{}       // 后台协程停止信号通道
    wg       sync.WaitGroup      // 后台协程同步等待组
}
```

**主要职责：**
- 维护消息 ID 的去重状态，通过 `idMap` 提供 O(1) 的存在性查询
- 通过 `idList` 双向链表维护 FIFO 顺序，支持高效的批量过期清理（只需从链表头部扫描）
- 驱动后台定时清理协程，自动回收过期记录
- 保证线程安全，通过互斥锁保护所有内部状态访问

### 3.3 idEntry - 消息记录节点

```go
type idEntry struct {
    id        string    // 消息唯一标识符
    createdAt time.Time // 记录创建（或最后访问刷新）的时间戳
}
```

**主要职责：**
- 存储消息 ID 和对应的时间戳，作为链表节点的载荷
- `createdAt` 字段随每次访问被刷新，实现"访问即续期"的 LRU 风格行为

### 3.4 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyMessageID` | 消息 ID 为空 | 调用 `CheckAndMark("")` 传入空字符串 |
| `ErrDeduplicatorStop` | 去重器已停止 | 保留错误，用于未来扩展 |
| `ErrInvalidConfig` | 配置无效 | 保留错误，用于未来扩展 |

## 4. 核心机制详解

### 4.1 幂等检查流程 (CheckAndMark)

```
CheckAndMark(msgID)
   │
   ├─ msgID == "" → 返回 (false, ErrEmptyMessageID)
   │
   ├─ mu.Lock()
   │
   ├─ 计算 cutoff = now - WindowSize
   │
   ├─ [idMap 中存在 msgID]
   │     │
   │     ├─ createdAt > cutoff（在窗口内）
   │     │     ├─ 移动节点到 idList 尾部（Touch）
   │     │     ├─ 刷新 createdAt = now
   │     │     └─ 返回 (false, nil)  → 重复消息，拒绝
   │     │
   │     └─ createdAt <= cutoff（已过期）
   │           ├─ 移动节点到 idList 尾部
   │           ├─ 刷新 createdAt = now
   │           └─ 返回 (true, nil)   → 过期后重新接受
   │
   └─ [idMap 中不存在 msgID]
         ├─ 创建 idEntry{msgID, now}
         ├─ 追加到 idList 尾部
         ├─ 建立 idMap[msgID] → 节点映射
         └─ 返回 (true, nil)      → 新消息，通过
```

**Touch on Access 机制说明：**
- 当重复消息在窗口内被再次访问时，虽然被拒绝消费（返回 `false`），但其 `createdAt` 时间戳会被刷新为当前时间，并在链表中移动到尾部
- 这种机制确保"热点"重复消息（持续被重发的消息）只要在窗口间隔内不断被访问，就不会被清理，避免了在消息重试场景下因窗口过期导致的误接受
- 同时该行为也保证了链表节点顺序与"最后访问时间"一致，使 FIFO 清理策略天然正确

### 4.2 滑动窗口机制

滑动窗口通过 **双向链表 + 时间阈值** 组合实现，核心思想是"时间顺序即链表顺序"：

```
  链表头部（最早）                          链表尾部（最新）
  ┌──────┐   ┌──────┐         ┌──────┐
  │  id1 │──▶│  id2 │──▶ ... ──▶│ idN │
  │ t=1  │   │ t=5  │         │ t=99 │
  └──────┘   └──────┘         └──────┘
     │
     ▼
 cutoff = now - WindowSize
  若 id1.createdAt <= cutoff → 移除，继续检查下一个
  若 id1.createdAt > cutoff  → 停止，后续节点均在窗口内
```

**滑动原理：**
1. 新记录总是追加到链表尾部，链表自然按"创建/访问时间"升序排列
2. 每次访问（包括重复消息访问）都会将对应节点移到尾部并刷新时间戳
3. 清理时只需从链表头部开始顺序扫描，遇到第一个未过期节点即可停止——其后的所有节点必然也在窗口内
4. 窗口随时间自然"滑动"：每次清理时根据当前时间重新计算 cutoff，过期的头部节点被丢弃

**时间复杂度：**
- 插入/查询/标记：O(1)（借助 `idMap` 哈希查找 + 链表节点移动）
- 过期清理：O(k)，k 为本次清理的过期节点数（非总节点数）

### 4.3 手动过期清理流程 (CleanExpired)

```
CleanExpired()
   │
   ├─ mu.Lock()
   │
   ├─ 计算 cutoff = now - WindowSize
   ├─ cleaned = 0
   │
   └─ [循环：从 idList 头部开始]
         │
         ├─ 头部为空 → 跳出
         │
         ├─ 取头部节点的 idEntry
         │
         ├─ createdAt > cutoff → 跳出（后续都未过期）
         │
         └─ createdAt <= cutoff（过期）
               ├─ 从 idList 移除头部节点
               ├─ 从 idMap 删除对应条目
               ├─ cleaned++
               └─ 继续循环
   │
   └─ mu.Unlock()，返回 cleaned
```

**优化点：链表有序性保证了提前终止**
由于链表按时间顺序排列，一旦遇到第一个未过期节点，其后的所有节点的 `createdAt` 都更大（更新），因此无需继续扫描。这意味着每次清理的工作量仅与过期节点数成正比，而非与总节点数成正比。

### 4.4 后台定时清理流程

```
Start()
   │
   ├─ mu.Lock()
   ├─ 已运行 → Unlock 直接返回
   ├─ running = true
   ├─ stopCh = make(chan struct{})
   ├─ mu.Unlock()
   │
   ├─ wg.Add(1)
   └─ 启动 cleanLoop 协程

cleanLoop（后台协程）
   │
   ├─ 创建 ticker = time.NewTicker(CleanInterval)
   │
   └─ [循环]
         │
         ├─ select
         │     ├─ stopCh 关闭 → ticker.Stop()，wg.Done()，退出
         │     └─ ticker.C 触发 → 调用 CleanExpired()
         │
         └─ 继续循环

Stop()
   │
   ├─ mu.Lock()
   ├─ 未运行 → Unlock 直接返回
   ├─ running = false
   ├─ close(stopCh)
   ├─ mu.Unlock()
   │
   └─ wg.Wait()（等待清理协程退出）
```

### 4.5 生命周期与资源回收

**典型生命周期：**
```
NewDeduplicator() → Start() → [CheckAndMark() 反复调用] → Stop()
                    │
                    └─ 可选：不调用 Start()，仅手动调用 CleanExpired()
```

**资源安全：**
- `Start()` 和 `Stop()` 均支持幂等调用，重复调用不会产生副作用
- `Stop()` 会阻塞直到后台清理协程完全退出，确保协程泄漏防护
- 不调用 `Start()` 也可正常使用去重功能（仅缺失自动清理，需手动调用 `CleanExpired()`）

## 5. 线程安全设计

所有公共方法均通过互斥锁 `mu` 保护内部状态：
- **写操作**（`CheckAndMark`、`CleanExpired`、`Clear`、`Start`、`Stop`）：获取排他锁
- **读操作**（`Contains`、`Count`）：同样获取排他锁（当前使用 `sync.Mutex`，如读多写少场景可升级为 `RWMutex`）
- **并发安全验证**：单元测试中的 `TestConcurrent_*` 系列测试通过多协程并发调用验证无竞态条件

## 6. 使用示例

### 6.1 基础使用：消息队列消费者去重

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/dedup"
)

type Message struct {
    ID      string
    Payload string
}

func main() {
    cfg := dedup.Config{
        WindowSize:    10 * time.Minute,  // 10 分钟窗口
        CleanInterval: 2 * time.Minute,   // 每 2 分钟清理一次
    }
    d := dedup.NewDeduplicatorWithConfig(cfg)
    d.Start()
    defer d.Stop()

    processMessage := func(msg Message) error {
        ok, err := d.CheckAndMark(msg.ID)
        if err != nil {
            return fmt.Errorf("dedup check failed: %w", err)
        }
        if !ok {
            fmt.Printf("丢弃重复消息: %s\n", msg.ID)
            return nil
        }
        fmt.Printf("处理消息: %s -> %s\n", msg.ID, msg.Payload)
        return nil
    }

    msgs := []Message{
        {ID: "m1", Payload: "order-1"},
        {ID: "m2", Payload: "order-2"},
        {ID: "m1", Payload: "order-1"}, // 重复，将被丢弃
        {ID: "m3", Payload: "order-3"},
    }

    for _, m := range msgs {
        _ = processMessage(m)
    }
    // 输出:
    // 处理消息: m1 -> order-1
    // 处理消息: m2 -> order-2
    // 丢弃重复消息: m1
    // 处理消息: m3 -> order-3
}
```

### 6.2 手动清理模式（不启动后台协程）

```go
d := dedup.NewDeduplicatorWithConfig(dedup.Config{
    WindowSize: 5 * time.Minute,
})
// 注意：不调用 d.Start()，无后台协程

for {
    batch := receiveBatch()
    for _, msg := range batch {
        ok, _ := d.CheckAndMark(msg.ID)
        if ok {
            handle(msg)
        }
    }
    // 每批处理完后手动清理过期记录
    cleaned := d.CleanExpired()
    if cleaned > 0 {
        log.Printf("清理了 %d 条过期去重记录", cleaned)
    }
}
```

### 6.3 监控去重器状态

```go
go func() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        count := d.Count()
        log.Printf("Dedup: 窗口内记录数 = %d", count)
    }
}()
```

### 6.4 单元测试风格的模拟场景

```go
func TestMessageDedupScenario(t *testing.T) {
    d := dedup.NewDeduplicatorWithConfig(dedup.Config{
        WindowSize: 100 * time.Millisecond,
    })

    // 第一轮：全部通过
    for i := 0; i < 10; i++ {
        id := fmt.Sprintf("msg-%d", i)
        ok, err := d.CheckAndMark(id)
        assert.True(t, ok)
        assert.NoError(t, err)
    }
    assert.Equal(t, 10, d.Count())

    // 第二轮：重复消息全部被拒
    for i := 0; i < 10; i++ {
        id := fmt.Sprintf("msg-%d", i)
        ok, _ := d.CheckAndMark(id)
        assert.False(t, ok)
    }

    // 等待窗口过期
    time.Sleep(150 * time.Millisecond)

    // 第三轮：过期后重新接受
    ok, _ := d.CheckAndMark("msg-0")
    assert.True(t, ok)
}
```

## 7. 文件结构

```
internal/dedup/
├── dedup.go      # 去重器核心实现
└── dedup_test.go # 单元测试（覆盖正常流程、边界条件、异常分支、并发场景）

docs/
└── dedup.md      # 本文档
```

## 8. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **基础创建** | `TestNewDeduplicator`、`TestDefaultConfig`、`TestNewDeduplicatorWithConfig_*` | 构造函数、默认值、配置推导 |
| **去重判断** | `TestCheckAndMark_NewMessage`、`TestCheckAndMark_DuplicateMessage`、`TestCheckAndMark_MultipleMessages` | 新消息通过、重复消息拒绝、批量验证 |
| **边界条件** | `TestCheckAndMark_EmptyMessageID`、`TestContains` | 空ID处理、存在性查询 |
| **窗口滑动** | `TestCheckAndMark_ExpiredThenReaccept`、`TestCheckAndMark_TouchOnAccess` | 过期后重新接受、访问续期 |
| **过期清理** | `TestCleanExpired_NoExpired`、`TestCleanExpired_AllExpired`、`TestCleanExpired_PartialExpired`、`TestCleanExpired_FIFOOrder` | 无过期/全过期/部分过期、FIFO顺序 |
| **生命周期** | `TestStartStop_Idempotent`、`TestStartStop_BackgroundCleanup` | 幂等启停、后台自动清理 |
| **并发安全** | `TestConcurrent_CheckAndMark`、`TestConcurrent_Duplicates`、`TestConcurrent_CleanAndCheck` | 并发写入、并发去重、清理与读写竞态 |
| **内存泄漏** | `TestMemoryLeak_AfterCleanup` | 长期运行后内存占用受控 |
| **内部机制** | `TestCheckAndMark_OrderPreservedInList`、`TestCheckAndMark_TouchMovesToBack` | 链表顺序正确性、Touch 行为 |
