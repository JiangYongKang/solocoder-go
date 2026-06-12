# 事件溯源引擎模块 (eventsrc)

## 一、模块功能

事件溯源（Event Sourcing）引擎模块提供了基于内存的事件溯源核心功能，通过将状态变更以事件的形式持久化存储，支持状态重建、乐观锁并发控制和快照优化等特性。

### 核心功能

1. **事件追加**：支持向指定聚合实例追加新事件，每个事件包含事件类型、事件数据和序列号，事件追加后该聚合实例的版本号递增。
2. **乐观锁冲突重试**：事件追加时校验聚合实例的当前版本号与期望版本号是否一致，版本冲突时返回冲突错误并由调用方决定重试，避免并发写入覆盖彼此的数据。
3. **事件回放**：支持加载指定聚合实例的全部历史事件并按顺序回放，通过回放事件序列重建该聚合实例的最新状态。
4. **快照生成**：支持对指定聚合实例的当前状态生成快照，快照记录当前版本号，后续回放时从快照版本之后的事件开始回放，减少回放事件数量。
5. **状态重建**：支持从快照加后续事件的组合方式重建聚合实例状态，如果不存在快照则从第一个事件开始全量回放。

## 二、核心结构体的职责

### 1. Event（事件）

位置：`internal/eventsrc/event.go`

事件是状态变更的不可变记录，每个事件代表聚合实例的一次状态变更。

**字段说明：**
- `AggregateID`：聚合实例唯一标识
- `EventType`：事件类型，用于区分不同的状态变更操作
- `Data`：事件数据，以字节数组形式存储具体的变更内容
- `Version`：事件序列号（版本号），在聚合实例内单调递增
- `Timestamp`：事件发生时间

### 2. Snapshot（快照）

位置：`internal/eventsrc/snapshot.go`

快照是聚合实例在某个版本的状态副本，用于加速状态重建过程。

**字段说明：**
- `AggregateID`：聚合实例唯一标识
- `Version`：快照对应的版本号
- `State`：聚合实例状态数据，以字节数组形式序列化存储
- `Timestamp`：快照创建时间

### 3. Aggregate 接口

位置：`internal/eventsrc/aggregate.go`

聚合是领域驱动设计中的核心概念，代表一组相关业务对象的集合，通过事件溯源维护其状态。

**方法说明：**
- `AggregateID()`：返回聚合实例的唯一标识
- `Version()`：返回聚合实例的当前版本号
- `Apply(event *Event) error`：应用事件到聚合实例，更新内部状态
- `MarshalState() ([]byte, error)`：将聚合状态序列化为字节数组
- `UnmarshalState(data []byte) error`：从字节数组反序列化恢复聚合状态

### 4. BaseAggregate（基础聚合）

位置：`internal/eventsrc/aggregate.go`

提供聚合的基础实现，包含 ID 和版本号管理，可被具体领域聚合嵌入使用。

**主要方法：**
- `NewBaseAggregate(id string)`：创建基础聚合实例
- `IncrementVersion()`：版本号递增
- `SetVersion(version int64)`：设置版本号

### 5. EventStore 接口与 InMemoryEventStore

位置：`internal/eventsrc/event_store.go`

事件存储负责事件的持久化存储和读取。

**方法说明：**
- `AppendEvents(aggregateID string, expectedVersion int64, events []*Event) error`：追加事件到指定聚合实例，使用乐观锁校验版本
- `LoadEvents(aggregateID string, fromVersion int64) ([]*Event, error)`：加载指定聚合实例从指定版本之后的所有事件
- `GetVersion(aggregateID string) (int64, error)`：获取指定聚合实例的当前版本号

**InMemoryEventStore 特点：**
- 使用内存 map 存储事件数据
- 支持并发安全的读写操作（sync.RWMutex）
- 每个聚合实例维护独立的事件列表和版本号

### 6. SnapshotStore 接口与 InMemorySnapshotStore

位置：`internal/eventsrc/snapshot_store.go`

快照存储负责快照的保存和加载。

**方法说明：**
- `SaveSnapshot(snapshot *Snapshot) error`：保存快照
- `LoadSnapshot(aggregateID string) (*Snapshot, error)`：加载指定聚合实例的最新快照

**InMemorySnapshotStore 特点：**
- 使用内存 map 存储快照数据
- 支持并发安全的读写操作
- 每个聚合实例只保留最新快照

### 7. EventSourcingEngine（事件溯源引擎）

位置：`internal/eventsrc/engine.go`

事件溯源引擎是模块的核心入口，整合事件存储和快照存储，提供完整的事件溯源功能。

**方法说明：**
- `AppendEvents(aggregateID string, expectedVersion int64, events []*Event) error`：追加事件（委托给 EventStore）
- `LoadEvents(aggregateID string, fromVersion int64) ([]*Event, error)`：加载事件（委托给 EventStore）
- `GetVersion(aggregateID string) (int64, error)`：获取版本号（委托给 EventStore）
- `ReplayEvents(aggregate Aggregate, events []*Event) error`：回放事件序列到聚合实例
- `RebuildState(aggregate Aggregate) error`：重建聚合实例状态（快照 + 后续事件）
- `CreateSnapshot(aggregate Aggregate) error`：为聚合实例创建快照
- `SaveSnapshot(snapshot *Snapshot) error`：保存快照（委托给 SnapshotStore）
- `LoadSnapshot(aggregateID string) (*Snapshot, error)`：加载快照（委托给 SnapshotStore）

## 三、事件回放与快照生成流程

### 1. 事件追加流程

```
调用方 -> EventSourcingEngine.AppendEvents()
              |
              v
         EventStore.AppendEvents()
              |
              +-- 校验聚合ID有效性
              +-- 校验事件列表非空
              +-- 校验当前版本与期望版本是否一致
              |     |
              |     +-- 不一致 -> 返回 ErrVersionConflict
              |
              +-- 为每个事件分配递增的版本号
              +-- 将事件追加到事件列表
              +-- 更新聚合实例版本号
              |
              v
         返回成功
```

**乐观锁机制说明：**
- 每次追加事件时需要传入期望版本号
- 如果实际版本号与期望版本号不一致，说明期间有其他写入操作，返回 `ErrVersionConflict`
- 调用方收到冲突错误后，可以重新加载最新状态并重试操作

### 2. 状态重建流程

```
调用方 -> EventSourcingEngine.RebuildState(aggregate)
              |
              v
         1. 尝试加载快照
              |
              +-- 快照存在
              |     |
              |     +-- 从快照反序列化状态到聚合
              |     +-- 设置聚合版本为快照版本
              |     +-- fromVersion = 快照版本
              |
              +-- 快照不存在
                    |
                    +-- fromVersion = 0
              |
              v
         2. 加载 fromVersion 之后的所有事件
              |
              v
         3. 按顺序回放事件
              |
              +-- 遍历事件列表
              +-- 逐个调用 aggregate.Apply(event)
              +-- 每个事件应用后版本号递增
              |
              v
         返回成功，聚合状态为最新
```

### 3. 快照生成流程

```
调用方 -> EventSourcingEngine.CreateSnapshot(aggregate)
              |
              v
         1. 校验聚合实例有效性
              |
              v
         2. 调用 aggregate.MarshalState() 序列化状态
              |
              v
         3. 创建 Snapshot 对象
              |
              +-- 设置 AggregateID
              +-- 设置 Version = aggregate.Version()
              +-- 设置 State = 序列化后的状态数据
              +-- 设置 Timestamp = 当前时间
              |
              v
         4. SnapshotStore.SaveSnapshot(snapshot)
              |
              v
         返回成功
```

## 四、使用示例

### 1. 定义领域聚合

```go
package main

import (
    "encoding/json"
    "solocoder-go/internal/eventsrc"
)

type UserAccount struct {
    eventsrc.BaseAggregate
    Username string
    Email    string
    Balance  float64
    Active   bool
}

func NewUserAccount(id string) *UserAccount {
    return &UserAccount{
        BaseAggregate: *eventsrc.NewBaseAggregate(id),
    }
}

func (a *UserAccount) Apply(event *eventsrc.Event) error {
    switch event.EventType {
    case "UserCreated":
        var data struct {
            Username string `json:"username"`
            Email    string `json:"email"`
        }
        if err := json.Unmarshal(event.Data, &data); err != nil {
            return err
        }
        a.Username = data.Username
        a.Email = data.Email
        a.Active = true
    case "Deposit":
        var data struct {
            Amount float64 `json:"amount"`
        }
        if err := json.Unmarshal(event.Data, &data); err != nil {
            return err
        }
        a.Balance += data.Amount
    case "UserDeactivated":
        a.Active = false
    }
    a.IncrementVersion()
    return nil
}

func (a *UserAccount) MarshalState() ([]byte, error) {
    state := struct {
        Username string  `json:"username"`
        Email    string  `json:"email"`
        Balance  float64 `json:"balance"`
        Active   bool    `json:"active"`
    }{
        Username: a.Username,
        Email:    a.Email,
        Balance:  a.Balance,
        Active:   a.Active,
    }
    return json.Marshal(state)
}

func (a *UserAccount) UnmarshalState(data []byte) error {
    var state struct {
        Username string  `json:"username"`
        Email    string  `json:"email"`
        Balance  float64 `json:"balance"`
        Active   bool    `json:"active"`
    }
    if err := json.Unmarshal(data, &state); err != nil {
        return err
    }
    a.Username = state.Username
    a.Email = state.Email
    a.Balance = state.Balance
    a.Active = state.Active
    return nil
}
```

### 2. 初始化事件溯源引擎

```go
eventStore := eventsrc.NewInMemoryEventStore()
snapshotStore := eventsrc.NewInMemorySnapshotStore()
engine := eventsrc.NewEventSourcingEngine(eventStore, snapshotStore)
```

### 3. 创建用户并追加事件

```go
aggregateID := "user-001"

// 创建用户
createdData, _ := json.Marshal(map[string]interface{}{
    "username": "alice",
    "email":    "alice@example.com",
})
createEvent := eventsrc.NewEvent(aggregateID, "UserCreated", createdData, 0)

err := engine.AppendEvents(aggregateID, 0, []*eventsrc.Event{createEvent})
if err != nil {
    // 处理错误
}

// 充值
depositData, _ := json.Marshal(map[string]interface{}{
    "amount": 100.0,
})
depositEvent := eventsrc.NewEvent(aggregateID, "Deposit", depositData, 0)

err = engine.AppendEvents(aggregateID, 1, []*eventsrc.Event{depositEvent})
if err != nil {
    // 处理错误
}
```

### 4. 使用乐观锁重试机制

```go
func appendWithRetry(engine *eventsrc.EventSourcingEngine, aggregateID string, event *eventsrc.Event, maxRetries int) error {
    for i := 0; i < maxRetries; i++ {
        version, err := engine.GetVersion(aggregateID)
        if err != nil {
            return err
        }
        
        err = engine.AppendEvents(aggregateID, version, []*eventsrc.Event{event})
        if err == nil {
            return nil
        }
        
        if errors.Is(err, eventsrc.ErrVersionConflict) {
            // 版本冲突，重试
            continue
        }
        
        return err
    }
    return eventsrc.ErrVersionConflict
}
```

### 5. 重建聚合状态

```go
account := NewUserAccount(aggregateID)
err := engine.RebuildState(account)
if err != nil {
    // 处理错误
}

fmt.Printf("用户: %s, 余额: %.2f, 版本: %d\n", 
    account.Username, account.Balance, account.Version())
```

### 6. 创建快照

```go
account := NewUserAccount(aggregateID)
engine.RebuildState(account)

// 创建快照
err := engine.CreateSnapshot(account)
if err != nil {
    // 处理错误
}
```

### 7. 完整工作流程示例

```go
func main() {
    // 1. 初始化引擎
    eventStore := eventsrc.NewInMemoryEventStore()
    snapshotStore := eventsrc.NewInMemorySnapshotStore()
    engine := eventsrc.NewEventSourcingEngine(eventStore, snapshotStore)
    
    aggregateID := "account-001"
    
    // 2. 创建账户
    createdData, _ := json.Marshal(map[string]interface{}{
        "owner": "Bob",
    })
    engine.AppendEvents(aggregateID, 0, []*eventsrc.Event{
        eventsrc.NewEvent(aggregateID, "AccountCreated", createdData, 0),
    })
    
    // 3. 多次操作
    for i := 0; i < 10; i++ {
        depositData, _ := json.Marshal(map[string]interface{}{
            "amount": 50.0,
        })
        version, _ := engine.GetVersion(aggregateID)
        engine.AppendEvents(aggregateID, version, []*eventsrc.Event{
            eventsrc.NewEvent(aggregateID, "Deposit", depositData, 0),
        })
    }
    
    // 4. 创建快照
    account := NewTestAccount(aggregateID)
    engine.RebuildState(account)
    engine.CreateSnapshot(account)
    
    // 5. 继续追加事件
    withdrawData, _ := json.Marshal(map[string]interface{}{
        "amount": 100.0,
    })
    version, _ := engine.GetVersion(aggregateID)
    engine.AppendEvents(aggregateID, version, []*eventsrc.Event{
        eventsrc.NewEvent(aggregateID, "Withdraw", withdrawData, 0),
    })
    
    // 6. 从快照+后续事件重建（更快）
    rebuiltAccount := NewTestAccount(aggregateID)
    engine.RebuildState(rebuiltAccount)
    
    fmt.Printf("最终余额: %.2f, 版本: %d\n", 
        rebuiltAccount.Balance, rebuiltAccount.Version())
}
```

## 五、错误类型

| 错误变量 | 说明 |
|---------|------|
| `ErrAggregateNotFound` | 聚合实例不存在 |
| `ErrVersionConflict` | 版本冲突（乐观锁校验失败） |
| `ErrInvalidEvent` | 无效事件 |
| `ErrSnapshotNotFound` | 快照不存在 |
| `ErrAggregateNil` | 聚合实例为空 |
| `ErrEventNil` | 事件为空 |
| `ErrSnapshotNil` | 快照为空 |
| `ErrInvalidAggregateID` | 无效的聚合ID |
