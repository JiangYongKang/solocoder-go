# 连接池管理器 (ConnPool) 模块需求文档

## 1. 模块概述

连接池管理器是一个通用的连接复用组件，用于管理和复用昂贵的网络连接（如数据库连接、RPC 连接等）。通过在内存中维护一组已建立的连接，避免了频繁创建和销毁连接带来的性能开销，同时提供了完善的连接生命周期管理机制。

本模块使用内存数据结构模拟网络连接对象，通过可配置的工厂函数（Factory）、心跳函数（Ping）和关闭函数（Close）与具体的连接类型解耦。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 连接借用 (Get) | 从连接池中获取一个可用连接，支持阻塞等待或立即返回错误两种策略 |
| F2 | 连接归还 (Put) | 将使用完毕的连接归还到连接池中，供后续复用 |
| F3 | 心跳检测 | 对池中空闲连接定时发送心跳探测，检测到不可用连接自动关闭并移除 |
| F4 | 空闲超时回收 | 连接超过配置的空闲时间未被借用，自动关闭回收 |
| F5 | 最大生命周期 | 每个连接有最大存活时间，超过后不可再借用，归还时自动关闭 |
| F6 | 空闲数超量回收 (LRU) | 池中空闲连接数超过最大空闲数时，按 LRU 顺序关闭多余连接 |
| F7 | 池关闭 (Close) | 关闭连接池，释放所有连接资源，停止后台协程 |
| F8 | 状态查询 | 查询连接池的总连接数、空闲连接数、活跃连接数 |

## 3. 核心结构体与职责

### 3.1 Config - 连接池配置

```go
type Config struct {
    InitialCap        int           // 初始连接数，创建池时预建的连接数量
    MaxCap            int           // 最大连接数，池中允许存在的连接上限
    MaxIdle           int           // 最大空闲数，空闲连接超过此数时触发 LRU 回收
    WaitTimeout       time.Duration // 借用等待超时，0 表示不等待直接返回错误
    IdleTimeout       time.Duration // 空闲超时，超过此时间未使用的连接被回收
    MaxLifetime       time.Duration // 最大生命周期，连接从创建起的最大存活时间
    HeartbeatInterval time.Duration // 心跳检测间隔，0 表示不启用心跳
    Factory           Factory       // 连接工厂函数，用于创建新连接
    Ping              PingFunc      // 心跳检测函数，检测连接是否可用
    Close             CloseFunc     // 连接关闭函数，释放连接资源
}
```

**配置约束与默认值：**
- `Factory` 必须提供，否则 `NewPool` 返回错误
- `MaxCap` 必须大于 0，否则 `NewPool` 返回错误
- `InitialCap` 默认为 0，超过 `MaxCap` 时自动截断为 `MaxCap`
- `MaxIdle` 默认为 `MaxCap`，超过 `MaxCap` 时自动截断为 `MaxCap`
- `Ping` 默认空实现（返回 nil），认为所有连接均可用
- `Close` 默认空实现（返回 nil），不做任何资源释放

### 3.2 Pool - 连接池主体

```go
type Pool struct {
    cfg      Config           // 配置快照
    mu       sync.Mutex       // 保护内部状态的互斥锁
    cond     *sync.Cond       // 条件变量，用于借用等待唤醒
    idleList *list.List       // 空闲连接双向链表（LRU 队列）
    active   map[Conn]*idleConn // 活跃连接集合（已借出未归还）
    count    int32            // 当前总连接数（原子操作）
    closed   bool             // 池是否已关闭
    stopCh   chan struct{}    // 后台协程停止信号
    wg       sync.WaitGroup   // 后台协程同步等待组
}
```

**主要职责：**
- 维护连接的生命周期状态（空闲/活跃）
- 协调并发的连接借用与归还操作
- 驱动心跳检测、空闲回收等后台协程
- 保证线程安全，通过互斥锁和条件变量实现同步

### 3.3 idleConn - 连接元数据包装

```go
type idleConn struct {
    conn       Conn      // 实际连接对象
    createTime time.Time // 连接创建时间（用于最大生命周期判断）
    lastUsed   time.Time // 最后一次使用时间（用于空闲超时判断）
}
```

**主要职责：**
- 包装实际连接对象，附加生命周期管理所需的时间戳
- 在空闲链表与活跃集合之间传递时保留元数据

### 3.4 类型别名

```go
type Conn interface{}            // 连接类型，使用空接口与具体实现解耦
type Factory func() (Conn, error)       // 连接工厂函数签名
type PingFunc func(Conn) error          // 心跳检测函数签名
type CloseFunc func(Conn) error         // 连接关闭函数签名
```

### 3.5 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrPoolClosed` | 连接池已关闭 | 已关闭的池上调用 Get/Put |
| `ErrPoolExhausted` | 连接池耗尽 | 无空闲连接且达到 MaxCap，WaitTimeout=0 或等待超时 |
| `ErrConnExpired` | 连接已过期 | 预留错误，当前在内部处理 |
| `ErrConnBad` | 连接不可用 | 预留错误，当前在内部处理 |

## 4. 连接生命周期管理流程

### 4.1 连接创建

```
NewPool(config)
   │
   ├─ 参数校验 → 失败则返回 error
   │
   ├─ 初始化内部结构（idleList、active、stopCh 等）
   │
   ├─ 循环创建 InitialCap 个初始连接
   │     └─ Factory() → 失败则清理已创建连接并返回 error
   │     └─ 封装为 idleConn（createTime=now, lastUsed=now）
   │     └─ 加入 idleList 尾部
   │     └─ count++
   │
   ├─ HeartbeatInterval > 0 → 启动 heartbeatLoop 协程
   └─ IdleTimeout > 0 → 启动 idleTimeoutLoop 协程
   │
   └─ 返回 *Pool
```

### 4.2 连接借用流程 (Get)

```
Get()
   │
   ├─ [外层循环：直到获取成功/错误]
   │
   ├─ mu.Lock() → 检查 closed → 返回 ErrPoolClosed
   │
   ├─ 尝试从 idleList 获取空闲连接
   │     └─ 遍历 idleList 头部开始
   │     └─ 取出 idleConn
   │     ├─ MaxLifetime 检查：过期 → count--, Close(conn), 继续
   │     ├─ Ping 检查：失败 → count--, Close(conn), 继续
   │     └─ 通过所有检查 → 加入 active, 返回 conn
   │
   ├─ 无空闲连接，尝试创建新连接
   │     └─ count < MaxCap
   │     └─ CAS count++（防止并发超建）
   │     └─ mu.Unlock(), 调用 Factory()
   │     ├─ 失败：count--, 返回 error
   │     └─ 成功：封装 idleConn，加入 active，返回 conn
   │
   ├─ 无空闲且达到 MaxCap
   │     ├─ WaitTimeout == 0 → 返回 ErrPoolExhausted
   │     │
   │     └─ WaitTimeout > 0 → 等待模式
   │           ├─ 设置 deadline = now + WaitTimeout
   │           ├─ 启动超时协程：到时 Broadcast 并关闭 timedOut
   │           ├─ [内层循环：等待条件]
   │           │     ├─ closed → 返回 ErrPoolClosed
   │           │     ├─ timedOut → 返回 ErrPoolExhausted
   │           │     ├─ idleList>0 或 count<MaxCap → 跳出等待
   │           │     └─ cond.Wait() → 挂起等待 Signal/Broadcast
   │           └─ 条件满足 → mu.Unlock()，回到外层循环重试
   │
   └─ 返回 (conn, error)
```

### 4.3 连接归还流程 (Put)

```
Put(conn)
   │
   ├─ conn == nil → 返回错误
   │
   ├─ mu.Lock() → 检查 closed → 返回 ErrPoolClosed
   │
   ├─ 从 active 查找 idleConn 元数据
   │     └─ 不存在 → 返回 "connection not borrowed" 错误
   │
   ├─ 从 active 中删除
   │
   ├─ MaxLifetime 检查
   │     └─ time.Since(createTime) > MaxLifetime
   │     └─ count--, cond.Signal()（唤醒一个等待者）
   │     └─ 调用 Close(conn)
   │     └─ 返回 Close 的结果
   │
   ├─ 更新 lastUsed = now
   ├─ idleConn 加入 idleList 头部（标记为最近使用）
   │
   ├─ 超量 LRU 回收
   │     └─ [while idleList.Len() > MaxIdle]
   │           └─ 取 idleList 尾部（最久未使用）
   │           └─ 移除并 count--
   │           └─ mu.Unlock(), Close(conn), mu.Lock()
   │
   ├─ cond.Signal()（唤醒一个等待者）
   ├─ mu.Unlock()
   │
   └─ 返回 nil
```

### 4.4 心跳检测流程

```
heartbeatLoop（后台协程，HeartbeatInterval 驱动）
   │
   └─ [ticker.C]
      │
      ├─ mu.Lock() → closed → 直接返回
      │
      ├─ 取出所有 idleList 元素到 toCheck，清空 idleList
      ├─ mu.Unlock()
      │
      ├─ 遍历 toCheck，逐个调用 Ping(conn)
      │     ├─ 成功 → 加入 good 列表
      │     └─ 失败 → 加入 bad 列表，count--
      │
      ├─ mu.Lock()
      │     ├─ !closed：遍历 good，未被借用的重新加入 idleList 尾部
      │     └─ closed：good 追加到 bad（全部关闭）
      │     └─ cond.Broadcast()（唤醒所有等待者）
      ├─ mu.Unlock()
      │
      └─ 遍历 bad，逐个调用 Close(conn)
```

### 4.5 空闲超时回收流程

```
idleTimeoutLoop（后台协程，IdleTimeout/2 驱动）
   │
   └─ [ticker.C]
      │
      ├─ mu.Lock() → closed → 直接返回
      │
      ├─ 遍历 idleList
      │     └─ now.Sub(lastUsed) > IdleTimeout
      │           └─ 从 idleList 移除
      │           └─ 加入 expired 列表
      │           └─ count--
      │
      ├─ mu.Unlock()
      │
      └─ 遍历 expired，逐个调用 Close(conn)
```

### 4.6 连接池关闭流程

```
Close()
   │
   ├─ mu.Lock() → 已关闭 → 直接返回
   │
   ├─ closed = true
   ├─ close(stopCh)（通知后台协程退出）
   ├─ cond.Broadcast()（唤醒所有等待中的 Get 调用）
   │
   ├─ 收集 idleList 中所有 conn 到 toClose
   ├─ 收集 active 中所有 conn 到 toClose
   ├─ 清空 idleList、active，count=0
   │
   ├─ mu.Unlock()
   │
   ├─ 遍历 toClose，逐个调用 Close(conn)
   │
   └─ wg.Wait()（等待所有后台协程退出）
```

## 5. LRU 策略说明

空闲连接采用双向链表 (`container/list`) 维护 LRU 顺序：

- **归还 (Put)**：将连接插入链表头部（最近使用）
- **借用 (Get)**：从链表头部开始遍历（优先取最近使用的）
- **超量回收**：从链表尾部开始移除（最久未使用的优先被回收）

这种策略保证了热点连接被持续复用，而冷连接在资源紧张时优先被释放。

## 6. 线程安全说明

连接池是完全并发安全的：
- 所有内部状态访问受 `mu` 互斥锁保护
- 总连接数 `count` 使用 `atomic` 原子操作，在创建新连接的 CAS 阶段减少锁持有时间
- 等待唤醒使用 `sync.Cond`，避免忙等待导致的 CPU 空转
- 后台协程通过 `stopCh` + `WaitGroup` 实现优雅退出

## 7. 使用示例

### 7.1 基础使用：数据库连接池

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/connpool"
)

type DBConnection struct {
    ID   int
    Host string
}

func main() {
    var idCounter int64

    cfg := connpool.Config{
        InitialCap:        2,
        MaxCap:            10,
        MaxIdle:           5,
        WaitTimeout:       3 * time.Second,
        IdleTimeout:       5 * time.Minute,
        MaxLifetime:       30 * time.Minute,
        HeartbeatInterval: 30 * time.Second,

        Factory: func() (connpool.Conn, error) {
            id := atomic.AddInt64(&idCounter, 1)
            return &DBConnection{ID: int(id), Host: "db.local"}, nil
        },

        Ping: func(c connpool.Conn) error {
            db := c.(*DBConnection)
            return pingDatabase(db.Host)
        },

        Close: func(c connpool.Conn) error {
            db := c.(*DBConnection)
            return closeDatabase(db.Host, db.ID)
        },
    }

    pool, err := connpool.NewPool(cfg)
    if err != nil {
        panic(err)
    }
    defer pool.Close()

    // 借用连接
    conn, err := pool.Get()
    if err != nil {
        panic(err)
    }
    db := conn.(*DBConnection)
    fmt.Printf("Using DB connection #%d\n", db.ID)

    // 使用完毕归还
    if err := pool.Put(conn); err != nil {
        panic(err)
    }
}
```

### 7.2 非阻塞模式（WaitTimeout=0）

```go
cfg := connpool.Config{
    MaxCap:      5,
    WaitTimeout: 0, // 无可用连接时立即返回 ErrPoolExhausted
    Factory:     factory,
}

pool, _ := connpool.NewPool(cfg)
defer pool.Close()

conn, err := pool.Get()
if err == connpool.ErrPoolExhausted {
    // 处理连接池耗尽：降级、排队、或返回业务错误
    log.Warn("connection pool exhausted, request rejected")
    return
}
```

### 7.3 监控连接池状态

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        total := pool.Len()
        idle := pool.IdleCount()
        active := pool.ActiveCount()
        log.Printf("ConnPool: total=%d idle=%d active=%d", total, idle, active)
    }
}()
```

### 7.4 模拟连接测试

```go
// 单元测试风格的模拟连接
type mockConn struct {
    id    int
    alive bool
}

func TestBasic(t *testing.T) {
    var counter int64
    pool, _ := connpool.NewPool(connpool.Config{
        InitialCap: 3,
        MaxCap:     5,
        Factory: func() (connpool.Conn, error) {
            return &mockConn{id: int(atomic.AddInt64(&counter, 1)), alive: true}, nil
        },
        Ping: func(c connpool.Conn) error {
            if !c.(*mockConn).alive {
                return errors.New("dead")
            }
            return nil
        },
    })
    defer pool.Close()

    c, _ := pool.Get()
    // 使用连接...
    pool.Put(c)
}
```

## 8. 文件结构

```
internal/connpool/
├── pool.go      # 连接池核心实现
└── pool_test.go # 单元测试（覆盖正常流程、边界条件、异常分支）

docs/
└── connpool.md  # 本文档
```
