# WebSocket 连接中心 (WSCenter) 模块需求文档

## 1. 模块概述

WebSocket 连接中心是一个通用的 WebSocket 连接管理组件，用于管理多个客户端的 WebSocket 连接，提供房间管理、消息广播、点对点消息、心跳保持和断线通知等功能。通过抽象的 `Conn` 接口与具体的 WebSocket 实现解耦，可以方便地集成不同的 WebSocket 库（如 gorilla/websocket）。

本模块使用内存数据结构管理客户端连接和房间信息，通过互斥锁保证并发安全，支持高并发场景下的稳定运行。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 客户端连接管理 | 支持客户端接入、断开，维护客户端连接生命周期 |
| F2 | 房间创建 | 支持创建指定 ID 的房间，房间不存在时自动创建 |
| F3 | 加入房间 | 客户端可以加入指定房间，加入后可接收房间内的广播消息 |
| F4 | 离开房间 | 客户端可以主动离开房间，离开后不再接收房间消息 |
| F5 | 房间自动销毁 | 当房间内没有客户端时，自动销毁房间以释放资源 |
| F6 | 查询房间在线列表 | 支持查询指定房间内的所有在线客户端 ID 列表 |
| F7 | 房间广播消息 | 向指定房间内的所有客户端发送广播消息，发送失败不影响其他客户端 |
| F8 | 点对点消息 | 向指定客户端发送一对一消息，目标不在线时返回失败 |
| F9 | 心跳保持 | 服务端定期向客户端发送 Ping 消息，检测连接活性 |
| F10 | 超时断开 | 客户端在超时时间内未回复 Pong 消息，视为断线并主动关闭连接 |
| F11 | 断线通知 | 客户端断线后，向其所在的所有房间广播离开通知 |
| F12 | 加入通知 | 客户端加入房间时，向房间内其他成员广播加入通知 |
| F13 | 离开通知 | 客户端离开房间时，向房间内其他成员广播离开通知 |
| F14 | 状态查询 | 查询当前连接的客户端数量、房间数量 |

## 3. 核心结构体与职责

### 3.1 Config - 连接中心配置

```go
type Config struct {
    PingInterval     time.Duration  // 心跳发送间隔
    PongTimeout      time.Duration  // Pong 回复超时时间
    SendTimeout      time.Duration  // 消息发送超时时间
    ClientBufferSize int            // 客户端发送缓冲区大小
    Logger           *log.Logger    // 日志记录器
}
```

**配置约束与默认值：**
- `PingInterval` 默认为 30 秒，<= 0 时使用默认值
- `PongTimeout` 默认为 10 秒，<= 0 时使用默认值
- `SendTimeout` 默认为 5 秒，<= 0 时使用默认值
- `ClientBufferSize` 默认为 256，<= 0 时使用默认值
- `Logger` 默认为 `log.Default()`

### 3.2 Message - 消息结构

```go
type Message struct {
    Type      MessageType  // 消息类型
    RoomID    string       // 房间 ID（广播消息时使用）
    From      string       // 发送方客户端 ID
    To        string       // 接收方客户端 ID
    Payload   []byte       // 消息内容
    Timestamp time.Time    // 消息时间戳
}
```

**消息类型：**
- `MessageTypeText` - 文本消息
- `MessageTypeBinary` - 二进制消息
- `MessageTypePing` - 心跳 Ping 消息
- `MessageTypePong` - 心跳 Pong 回复
- `MessageTypeJoin` - 加入房间通知
- `MessageTypeLeave` - 离开房间通知
- `MessageTypeBroadcast` - 房间广播消息
- `MessageTypeDirect` - 点对点消息

### 3.3 Conn - 连接抽象接口

```go
type Conn interface {
    SendMessage(msg *Message) error  // 发送消息
    Close() error                    // 关闭连接
    ID() string                      // 获取连接 ID
}
```

**主要职责：**
- 抽象底层 WebSocket 连接，与具体实现解耦
- 提供消息发送、连接关闭、ID 获取的统一接口

### 3.4 Client - 客户端连接包装

```go
type Client struct {
    id         string           // 客户端 ID
    conn       Conn             // 底层连接
    rooms      map[string]*Room // 加入的房间集合
    sendCh     chan *Message    // 发送消息通道
    lastPong   time.Time        // 最后一次收到 Pong 的时间
    mu         sync.RWMutex     // 保护内部状态
    closeOnce  sync.Once        // 保证 Close 只执行一次
    disconnect bool             // 是否已断开连接
}
```

**主要职责：**
- 包装底层 `Conn` 连接，附加房间信息和心跳状态
- 管理发送通道，实现异步消息发送
- 维护心跳状态，记录最后 Pong 时间

### 3.5 Room - 房间管理

```go
type Room struct {
    id       string             // 房间 ID
    clients  map[string]*Client // 房间内的客户端集合
    mu       sync.RWMutex       // 保护内部状态
    createAt time.Time          // 房间创建时间
}
```

**主要职责：**
- 管理房间内的客户端集合
- 提供客户端加入、离开、查询功能
- 维护房间创建时间等元数据

### 3.6 WSCenter - 连接中心主体

```go
type WSCenter struct {
    cfg       Config             // 配置快照
    mu        sync.RWMutex       // 保护内部状态
    clients   map[string]*Client // 所有连接的客户端
    rooms     map[string]*Room   // 所有存在的房间
    running   bool               // 是否运行中
    stopCh    chan struct{}      // 后台协程停止信号
    wg        sync.WaitGroup     // 后台协程同步等待组
    logger    *log.Logger        // 日志记录器
    nextMsgID uint64             // 消息 ID 生成器
}
```

**主要职责：**
- 维护全局客户端和房间映射
- 协调并发的客户端连接、房间操作
- 驱动心跳检测、超时检查等后台协程
- 提供对外 API 接口，保证线程安全

### 3.7 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrCenterStopped` | 连接中心已停止 | 已停止的中心上调用 Connect/CreateRoom 等 |
| `ErrClientNotFound` | 客户端不存在 | 操作不存在的客户端 ID |
| `ErrClientExists` | 客户端已存在 | 重复连接相同 ID 的客户端 |
| `ErrRoomNotFound` | 房间不存在 | 操作不存在的房间 ID |
| `ErrRoomExists` | 房间已存在 | 重复创建相同 ID 的房间 |
| `ErrClientNotInRoom` | 客户端不在房间内 | 离开未加入的房间 |
| `ErrClientAlreadyInRoom` | 客户端已在房间内 | 重复加入同一个房间 |
| `ErrInvalidID` | ID 无效 | 传入空字符串的 ID |
| `ErrSendTimeout` | 发送超时 | 消息发送缓冲区满且超时 |
| `ErrClientOffline` | 客户端已离线 | 向已断开的客户端发送消息 |

## 4. 消息流转路径

### 4.1 客户端连接流程

```
Connect(conn)
   │
   ├─ 参数校验 → conn 为 nil 或 ID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.Lock() → 检查 running → 返回 ErrCenterStopped
   │
   ├─ 检查客户端是否已存在 → 存在 → 返回 ErrClientExists
   │
   ├─ 创建 Client 对象
   │     └─ sendCh 缓冲通道初始化
   │     └─ lastPong 设置为当前时间
   │
   ├─ 加入 clients 映射
   │
   ├─ 启动 clientWriteLoop 后台协程（负责从 sendCh 读取并发送消息）
   │
   ├─ mu.Unlock()
   │
   └─ 返回 *Client
```

### 4.2 加入房间流程

```
JoinRoom(clientID, roomID)
   │
   ├─ 参数校验 → ID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.RLock() → 查找客户端和房间
   │     ├─ 客户端不存在 → 返回 ErrClientNotFound
   │     └─ 房间不存在 → 返回 ErrRoomNotFound
   │
   ├─ 检查客户端是否已断开 → 已断开 → 返回 ErrClientOffline
   │
   ├─ 房间添加客户端
   │     └─ room.mu.Lock() → 检查是否已在房间内
   │     └─ 已在房间内 → 返回 ErrClientAlreadyInRoom
   │     └─ 加入 clients 映射
   │     └─ room.mu.Unlock()
   │
   ├─ 客户端记录房间
   │     └─ client.mu.Lock()
   │     └─ 加入 rooms 映射
   │     └─ client.mu.Unlock()
   │
   └─ 广播加入通知 → 向房间内其他成员发送 MessageTypeJoin 消息
```

### 4.3 离开房间流程

```
LeaveRoom(clientID, roomID)
   │
   ├─ 参数校验 → ID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.RLock() → 查找客户端和房间
   │     ├─ 客户端不存在 → 返回 ErrClientNotFound
   │     └─ 房间不存在 → 返回 ErrRoomNotFound
   │
   ├─ 房间移除客户端
   │     └─ room.mu.Lock() → 检查是否在房间内
   │     └─ 不在房间内 → 返回 ErrClientNotInRoom
   │     └─ 从 clients 映射移除
   │     └─ room.mu.Unlock()
   │
   ├─ 客户端移除房间
   │     └─ client.mu.Lock()
   │     └─ 从 rooms 映射移除
   │     └─ client.mu.Unlock()
   │
   ├─ 广播离开通知 → 向房间内其他成员发送 MessageTypeLeave 消息
   │
   └─ 检查房间是否为空
         └─ 为空 → mu.Lock() → 再次检查 → 从 rooms 映射删除 → mu.Unlock()
```

### 4.4 房间广播流程

```
BroadcastToRoom(roomID, payload, fromClientID)
   │
   ├─ 参数校验 → roomID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.RLock() → 查找房间
   │     └─ 房间不存在 → 返回 ErrRoomNotFound
   │
   ├─ 获取房间内所有客户端列表
   │
   ├─ 遍历客户端列表
   │     ├─ 跳过发送者（client.id == fromClientID）
   │     ├─ 构造 MessageTypeBroadcast 消息
   │     ├─ 调用 client.send(msg, timeout)
   │     │     ├─ 检查客户端是否已断开 → 已断开 → 记录日志
   │     │     ├─ 尝试写入 sendCh → 成功 → successCount++
   │     │     └─ 超时 → 记录日志，继续下一个
   │     └─ 发送失败只记录日志，不中断循环
   │
   └─ 返回 (successCount, nil)
```

**关键特性：**
- 单个客户端发送失败不会影响其他客户端
- 发送失败只记录日志，不返回错误给调用者
- 返回成功发送的客户端数量

### 4.5 点对点消息流程

```
SendToClient(fromClientID, toClientID, payload)
   │
   ├─ 参数校验 → ID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.RLock() → 查找发送方和接收方
   │     ├─ 发送方不存在 → 返回 ErrClientNotFound
   │     └─ 接收方不存在 → 返回 ErrClientOffline
   │
   ├─ 检查发送方是否已断开 → 已断开 → 返回 ErrClientOffline
   │
   ├─ 检查接收方是否已断开 → 已断开 → 返回 ErrClientOffline
   │
   ├─ 构造 MessageTypeDirect 消息
   │
   ├─ 调用 client.send(msg, timeout)
   │     ├─ 成功 → 返回 nil
   │     └─ 失败 → 返回 ErrSendTimeout
   │
   └─ 返回错误（如果有）
```

**关键特性：**
- 不缓存离线消息，接收方不在线立即返回失败
- 发送方和接收方都需要在线

### 4.6 心跳检测流程

```
pingLoop（后台协程，PingInterval 驱动）
   │
   └─ [ticker.C]
      │
      ├─ 发送 Ping 消息
      │     └─ 遍历所有客户端
      │           ├─ 已断开 → 跳过
      │           └─ 发送 MessageTypePing 消息
      │
      └─ 超时检查
            └─ 遍历所有客户端
                  ├─ 已断开 → 跳过
                  ├─ 计算超时阈值：now - PongTimeout
                  ├─ lastPong < 超时阈值 → 调用 Disconnect()
                  └─ 记录超时断开日志
```

**心跳机制说明：**
- `PingInterval` 控制心跳发送频率
- `PongTimeout` 控制超时判定阈值
- 客户端需要在 `PongTimeout` 内回复 Pong 消息
- 超时未回复的客户端被视为断线，主动断开连接

### 4.7 客户端断开流程

```
Disconnect(clientID)
   │
   ├─ 参数校验 → ID 为空 → 返回 ErrInvalidID
   │
   ├─ mu.Lock() → 查找客户端
   │     └─ 不存在 → 返回 ErrClientNotFound
   │
   ├─ 获取客户端加入的所有房间
   │
   ├─ 从 clients 映射删除客户端
   │
   ├─ mu.Unlock()
   │
   ├─ 遍历所有房间
   │     ├─ 从房间移除客户端
   │     ├─ 客户端移除房间记录
   │     ├─ 广播离开通知（MessageTypeLeave）
   │     └─ 房间为空 → 自动销毁
   │
   └─ 关闭客户端连接
         └─ client.close()
               ├─ 设置 disconnect = true
               ├─ 关闭 sendCh
               └─ 调用 conn.Close()
```

**关键特性：**
- 客户端断开后，自动从所有房间移除
- 每个房间广播离开通知，告知其他成员
- 房间为空时自动销毁，释放资源

## 5. 核心机制说明

### 5.1 房间自动销毁机制

房间采用"懒删除"策略：
- 每次 `LeaveRoom` 或 `Disconnect` 后检查房间是否为空
- 为空时从 `rooms` 映射中删除
- 避免空房间占用内存资源

实现细节见 [LeaveRoom()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L439-L473)

### 5.2 异步发送机制

每个客户端有独立的发送通道 `sendCh`：
- `client.send()` 方法将消息写入通道（带超时）
- `clientWriteLoop` 后台协程从通道读取并实际发送
- 通道满时写入超时，返回 `ErrSendTimeout`
- 单个客户端的发送阻塞不会影响其他客户端

实现细节见 [clientWriteLoop()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L701-L718)

### 5.3 发送失败隔离

广播消息时采用"失败隔离"策略：
- 遍历客户端列表，逐个发送
- 单个客户端发送失败（超时或断开）只记录日志
- 继续向其他客户端发送，不中断广播流程
- 保证广播的高可用性

实现细节见 [BroadcastToRoom()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L521-L560)

### 5.4 并发安全设计

连接中心完全并发安全：
- 所有共享状态（clients、rooms）受 `mu` 互斥锁保护
- 客户端内部状态受 `client.mu` 保护
- 房间内部状态受 `room.mu` 保护
- 后台协程通过 `stopCh` + `WaitGroup` 实现优雅退出
- 消息 ID 使用 `atomic` 原子操作生成

**锁层级设计：**
- 避免嵌套锁导致死锁
- 锁获取顺序：`ws.mu` → `room.mu` → `client.mu`
- 操作完成后立即释放锁

## 6. 使用示例

### 6.1 基础使用：简单聊天服务器

```go
package main

import (
    "fmt"
    "log"
    "net/http"
    "time"

    "github.com/gorilla/websocket"
    "solocoder-go/internal/wscenter"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

type WSConn struct {
    conn *websocket.Conn
    id   string
}

func (c *WSConn) SendMessage(msg *wscenter.Message) error {
    return c.conn.WriteJSON(msg)
}

func (c *WSConn) Close() error {
    return c.conn.Close()
}

func (c *WSConn) ID() string {
    return c.id
}

func main() {
    cfg := wscenter.DefaultConfig()
    cfg.PingInterval = 30 * time.Second
    cfg.PongTimeout = 10 * time.Second

    center := wscenter.NewWSCenter(cfg)
    center.Start()
    defer center.Stop()

    center.CreateRoom("lobby")

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        conn, err := upgrader.Upgrade(w, r, nil)
        if err != nil {
            log.Println(err)
            return
        }

        clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
        wsConn := &WSConn{conn: conn, id: clientID}

        client, err := center.Connect(wsConn)
        if err != nil {
            log.Printf("Connect failed: %v", err)
            conn.Close()
            return
        }

        center.JoinRoom(clientID, "lobby")

        go handleIncomingMessages(center, client, conn)
    })

    log.Println("Server starting on :8080")
    http.ListenAndServe(":8080", nil)
}

func handleIncomingMessages(center *wscenter.WSCenter, client *wscenter.Client, conn *websocket.Conn) {
    defer center.Disconnect(client.ID())

    for {
        var msg wscenter.Message
        err := conn.ReadJSON(&msg)
        if err != nil {
            log.Printf("Read error: %v", err)
            return
        }

        switch msg.Type {
        case wscenter.MessageTypePong:
            center.HandlePong(client.ID())
        case wscenter.MessageTypeBroadcast:
            center.BroadcastToRoom(msg.RoomID, msg.Payload, client.ID())
        case wscenter.MessageTypeDirect:
            center.SendToClient(client.ID(), msg.To, msg.Payload)
        }
    }
}
```

### 6.2 查询房间在线用户

```go
roomID := "game-room-1"
clients, err := center.GetRoomClients(roomID)
if err != nil {
    if err == wscenter.ErrRoomNotFound {
        log.Printf("Room %s does not exist", roomID)
    }
    return
}
fmt.Printf("Online users in %s: %d\n", roomID, len(clients))
for _, clientID := range clients {
    fmt.Printf("  - %s\n", clientID)
}
```

### 6.3 监控连接状态

```go
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for range ticker.C {
        totalClients := center.ClientCount()
        totalRooms := center.RoomCount()
        log.Printf("WSCenter: clients=%d rooms=%d", totalClients, totalRooms)
    }
}()
```

### 6.4 单元测试风格的模拟连接

```go
type mockConn struct {
    id        string
    mu        sync.Mutex
    messages  []*wscenter.Message
    closed    bool
    sendDelay time.Duration
}

func newMockConn(id string) *mockConn {
    return &mockConn{id: id, messages: make([]*wscenter.Message, 0)}
}

func (m *mockConn) SendMessage(msg *wscenter.Message) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if m.sendDelay > 0 {
        time.Sleep(m.sendDelay)
    }
    m.messages = append(m.messages, msg)
    return nil
}

func (m *mockConn) Close() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.closed = true
    return nil
}

func (m *mockConn) ID() string {
    return m.id
}

func TestBasicFlow(t *testing.T) {
    center := wscenter.NewWSCenter(wscenter.DefaultConfig())
    defer center.Stop()

    conn1 := newMockConn("client1")
    conn2 := newMockConn("client2")

    center.Connect(conn1)
    center.Connect(conn2)

    center.CreateRoom("room1")
    center.JoinRoom("client1", "room1")
    center.JoinRoom("client2", "room1")

    center.BroadcastToRoom("room1", []byte("hello"), "client1")

    time.Sleep(100 * time.Millisecond)

    // 验证 client2 收到了消息
    if len(conn2.messages) == 0 {
        t.Error("expected client2 to receive message")
    }
}
```

## 7. 文件结构

```
internal/wscenter/
├── wscenter.go       # WebSocket 连接中心核心实现
└── wscenter_test.go  # 单元测试（覆盖正常流程、边界条件、异常分支）

docs/
└── wscenter.md       # 本文档
```

## 8. 测试覆盖范围

单元测试覆盖以下场景：

### 正常流程
- ✅ 客户端连接与断开
- ✅ 房间创建与自动销毁
- ✅ 加入房间与离开房间
- ✅ 房间广播消息
- ✅ 点对点消息
- ✅ 心跳保持与超时断开
- ✅ 加入/离开通知
- ✅ 断线通知
- ✅ 并发操作

### 边界条件
- ✅ 空 ID 参数处理
- ✅ 重复连接/重复加入
- ✅ 离开未加入的房间
- ✅ 查询不存在的房间/客户端
- ✅ 房间为空时自动销毁
- ✅ 发送缓冲区满时的超时

### 异常分支
- ✅ 目标客户端不在线时发送消息
- ✅ 发送方客户端已断开
- ✅ 中心已停止后的操作
- ✅ 消息发送超时
- ✅ 单个客户端发送失败不影响其他
- ✅ Pong 超时自动断开
- ✅ 多房间断线通知
