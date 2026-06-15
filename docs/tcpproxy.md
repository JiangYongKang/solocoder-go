# TCP 反向代理 (TCPProxy) 模块需求文档

## 1. 模块概述

TCP 反向代理模块是一个高性能的 Layer 4 流量转发组件，在客户端与上游服务集群之间充当中间层。通过连接多路复用减少 TCP 握手开销，通过健康探测自动摘除故障节点，通过连接池复用上游连接，通过源 IP 哈希实现会话保持，为大规模分布式 TCP 服务提供稳定可靠的流量调度能力。

本模块核心特性：
- **连接多路复用**：单条控制连接承载多个逻辑流（Stream），每个流由唯一 ID 标识
- **上游健康探测**：定时 TCP 探活，自动维护上游可用列表
- **连接池管理**：每个上游独立连接池，支持最大连接数和空闲超时配置
- **源 IP 会话保持**：FNV-1a 哈希算法，上游变更时尽量保持已有映射

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 帧协议编解码 | 实现多路复用帧的 Encode/Decode，支持 6 种帧类型 |
| F2 | 逻辑流 (Stream) | 实现独立的可读可写字节流，由 StreamID 区分 |
| F3 | 多路复用连接 (MuxConn) | 在底层 TCP 连接之上管理多条逻辑流，处理帧的分发与收集 |
| F4 | 上游服务管理 | Upstream 结构体封装地址与健康状态，提供 Connect/Probe 能力 |
| F5 | 健康检查器 | 后台定时对所有上游发起 TCP 探测，基于阈值更新健康状态 |
| F6 | 健康状态回调 | 上游状态变更时触发回调，通知负载均衡器清理映射 |
| F7 | 连接池 Get/Put | 连接借用/归还，复用空闲连接，超量新建 |
| F8 | 连接池上限控制 | 达到 MaxConns 时支持阻塞等待或立即返回耗尽错误 |
| F9 | 空闲超时回收 | 后台定时扫描 idleList，回收超过 IdleTimeout 的连接 |
| F10 | 错误连接剔除 | 读写异常时自动从池中移除坏连接 |
| F11 | FNV-1a IP 哈希 | 对客户端 IP 做非加密哈希，映射到上游列表索引 |
| F12 | 粘性会话映射表 | 保存 IP → Upstream 映射，同一 IP 始终选择同一上游 |
| F13 | 不健康上游自动绕行 | 已映射的上游故障时，重新哈希到其他健康上游 |
| F14 | 故障节点映射清理 | 上游摘除时同步清理所有关联的粘性会话映射 |
| F15 | 代理启动/停止 | 监听端口、接受连接、优雅关闭所有资源 |
| F16 | 请求转发管道 | Stream ↔ 池化连接的双向数据拷贝协程 |
| F17 | 无可用上游容错 | 所有上游不健康时向客户端发送 RST 帧 |

## 3. 核心结构体与职责

### 3.1 Frame - 多路复用帧

```go
type Frame struct {
    Type     uint16 // 帧类型 (SYNC/DATA/ACK/FIN/RST/HEARTBEAT)
    StreamID uint16 // 所属逻辑流 ID
    Length   uint32 // Payload 字节长度
    Payload  []byte // 业务数据载荷
}
```

**职责**：多路复用协议的最小传输单元，负责封装逻辑流的数据和控制信令。

**帧类型定义**：

| 常量名 | 值 | 含义 | Payload |
|--------|----|------|---------|
| FrameTypeSYNC | 0x01 | 新建逻辑流请求 | 可选，预留扩展 |
| FrameTypeDATA | 0x02 | 业务数据传输 | 业务字节流分片 |
| FrameTypeACK | 0x03 | 确认帧 (预留) | 可选 |
| FrameTypeFIN | 0x04 | 主动关闭流 | 无 |
| FrameTypeRST | 0x05 | 重置流 (异常) | 无 |
| FrameTypeHEARTBEAT | 0x06 | 心跳探测 | 无 |

### 3.2 Stream - 逻辑流

```go
type Stream struct {
    ID          uint16        // 流唯一标识符
    mux         *MuxConn      // 所属多路复用连接
    readCh      chan []byte   // 数据接收缓冲通道
    readBuf     []byte        // 未读尽的剩余数据
    closeOnce   sync.Once     // 关闭流程幂等保护
    chCloseOnce sync.Once     // 通道关闭幂等保护
    closed      atomic.Bool   // 流关闭标志
    finSent     atomic.Bool   // 是否已发送 FIN
    finRecv     atomic.Bool   // 是否已接收 FIN
}
```

**职责**：
- 实现 `io.Reader` / `io.Writer` / `io.Closer` 接口，对外呈现为普通字节流
- 从 MuxConn 的帧分发中接收数据，通过 channel 解耦读写协程
- 关闭时发送 FIN 帧，完成四次挥手的客户端侧
- 使用双 Once 设计，避免 cleanupOnError 与 Close 并发导致的重复关闭 panic

### 3.3 MuxConn - 多路复用连接

```go
type MuxConn struct {
    conn       net.Conn              // 底层 TCP 连接
    streams    map[uint16]*Stream    // 活跃流注册表
    streamMu   sync.RWMutex          // 保护 streams map
    nextStream atomic.Uint32         // 自增流 ID 分配器
    writeMu    sync.Mutex            // 帧写入串行化锁
    closed     atomic.Bool           // 连接关闭标志
    stopCh     chan struct{}         // 停止信号通道
    wg         sync.WaitGroup        // 后台协程同步组
    onStream   func(*Stream)         // 被动接收新流的回调钩子
}
```

**职责**：
- **帧写入调度**：通过 `writeMu` 保证并发写帧的原子性，防止帧头部交错
- **帧读取与分发**：`readLoop` 持续读取完整帧，根据 Type 分发到对应 handler
- **流生命周期**：创建 (handleSYNC)、数据投递 (handleDATA)、正常关闭 (handleFIN)、异常重置 (handleRST)
- **双向关闭安全**：`cleanupOnError` + `wg.Wait()` 分离设计，避免 readLoop 中 defer Close 导致的自等待死锁

**关键实现细节**：
- `NewMuxConn` 立即启动 `readLoop` 协程，通过 onStream 回调将被动接收到的流交给上层
- `NewStream` 主动发起 SYNC 帧，同步等待对端确认（隐式通过数据传输确认）
- `cleanupOnError` 使用 `atomic.Swap` 保证清理逻辑只执行一次，安全关闭所有流的 readCh

### 3.4 Upstream - 上游服务

```go
type Upstream struct {
    Address string       // host:port 地址
    healthy atomic.Bool  // 健康状态原子标志
}
```

**职责**：
- 封装上游服务的网络标识和运行时健康状态
- `Connect()`：带 5s 超时的 TCP 拨号，供连接池使用
- `Probe(timeout)`：健康探测，成功建立连接并立即关闭即视为健康

### 3.5 HealthChecker - 健康检查器

```go
type HealthCheckerConfig struct {
    CheckInterval time.Duration // 探测周期，默认 10s
    ProbeTimeout  time.Duration // 单次探测超时，默认 3s
    FailThreshold int           // 连续失败 N 次判不健康，默认 3
    PassThreshold int           // 连续成功 N 次判健康，默认 2
}

type HealthChecker struct {
    cfg       HealthCheckerConfig
    upstreams map[string]*upstreamHealth // addr → 健康元数据
    mu        sync.RWMutex
    stopCh    chan struct{}
    running   atomic.Bool
    wg        sync.WaitGroup
    onChange  func(addr string, healthy bool) // 状态变更回调
}

type upstreamHealth struct {
    upstream  *Upstream
    failCount int       // 当前连续失败次数
    passCount int       // 当前连续成功次数
    lastCheck time.Time // 最近探测时间
}
```

**职责**：
- **周期性探测**：`checkLoop` 由 ticker 驱动，每轮遍历所有注册的上游
- **阈值判定**：避免网络抖动导致的频繁切换，失败/成功都需达到阈值才变更状态
- **变更通知**：通过 `onChange` 回调，将状态变更事件推送给负载均衡器以清理粘性映射
- **并发安全**：探测在 RLock 下取快照，写状态时升级为 Lock，避免长时间持锁阻塞外部查询

**状态流转**：
```
初始 healthy=true
    │
    ├─ 探测失败 → failCount++ → failCount ≥ FailThreshold → healthy=false
    │                                            └─ onChange(addr, false)
    │
    └─ 探测成功 → passCount++ → passCount ≥ PassThreshold → healthy=true
                                                 └─ onChange(addr, true)
```

### 3.6 ConnPool - 上游连接池

```go
type ConnPoolConfig struct {
    MaxConns    int           // 最大连接数，默认 10
    IdleTimeout time.Duration // 空闲超时，默认 5min
    WaitTimeout time.Duration // 获取等待超时，0=不等待立即返回
}

type ConnPool struct {
    cfg         ConnPoolConfig
    upstream    *Upstream
    mu          sync.Mutex
    cond        *sync.Cond    // 等待唤醒条件变量
    idleList    []*poolConn   // 空闲连接栈（末尾=最近使用）
    activeCount int           // 已借出未归还的连接数
    closed      bool
    stopCh      chan struct{}
    wg          sync.WaitGroup
}

type poolConn struct {
    conn     net.Conn
    upstream *Upstream
    lastUsed time.Time
    idle     atomic.Bool
}
```

**职责**：
- **LIFO 空闲栈**：`idleList` 从尾部取/放，优先复用"最热"的连接（保活概率更高）
- **借用超时保护**：借用时即使是空闲连接也检查 IdleTimeout，避免拿到半死连接
- **连接失败回滚**：新建连接失败时归还配额（activeCount--）并唤醒等待者
- **坏连接自动剔除**：`pooledConn.Read/Write` 出错时调用 `pool.Remove` 而非归还
- **双路径关闭安全**：Pool.Close 与 idleTimeoutLoop 并发时均正确处理

**Get 流程**：
```
Get()
  ├─ [遍历 idleList 从尾到头]
  │    ├─ 超时 → 关闭、跳过
  │    └─ 有效 → activeCount++ → 返回包装后的 pooledConn
  │
  ├─ activeCount < MaxConns → 新建 → 成功/失败均正确维护计数
  │
  └─ 已满
       ├─ WaitTimeout=0 → ErrPoolExhausted
       └─ WaitTimeout>0 → cond.Wait 循环，含 deadline 检查
```

### 3.7 pooledConn - 池化连接包装器

```go
type pooledConn struct {
    pc   *poolConn
    pool *ConnPool
}
```

**职责**：
- 实现完整的 `net.Conn` 接口，对上层透明
- `Close()` 不真正关闭底层连接，而是归还到池（Put）
- `Read/Write` 异常时从池移除底层连接（Remove），防止污染池
- 通过 `idle` 标志防止重复归还

### 3.8 IPHashBalancer - 源 IP 哈希负载均衡器

```go
type IPHashBalancer struct {
    mu        sync.RWMutex
    upstreams []*Upstream       // 全量上游列表
    mapping   map[string]*Upstream // clientIP → 选定上游（粘性表）
    hc        *HealthChecker    // 关联健康检查器
}
```

**职责**：
- **FNV-1a 哈希**：对客户端 IP 字符串做 32 位非加密哈希，分布均匀、计算快速
- **两级选择策略**：
  1. 查 `mapping` 表 → 命中且该上游仍健康 → 直接返回（保持会话）
  2. 未命中或原上游故障 → `hash % len(healthy)` 计算新索引并记录映射
- **锁粒度优化**：`GetUpstream` 先 RLock 做只读判断，确需写入时才 Lock，减少写锁竞争
- **映射清理**：`RemoveFromMapping` 遍历全表删除某上游的所有条目，由健康检查回调触发

### 3.9 TCPProxy - 反向代理总控

```go
type ProxyConfig struct {
    ListenAddress       string
    Upstreams           []string
    PoolMaxConns        int
    PoolIdleTimeout     time.Duration
    PoolWaitTimeout     time.Duration
    HealthCheckConfig   HealthCheckerConfig
    EnableStickySession bool
}

type TCPProxy struct {
    cfg      ProxyConfig
    listener net.Listener
    hc       *HealthChecker
    balancer *IPHashBalancer
    pools    map[string]*ConnPool  // addr → 连接池
    poolsMu  sync.RWMutex
    muxes    map[string]*MuxConn   // remoteAddr → 客户端多路复用
    muxesMu  sync.RWMutex
    closed   atomic.Bool
    stopCh   chan struct{}
    wg       sync.WaitGroup
}
```

**职责**：
- **资源组装**：创建时一次性初始化健康检查器、负载均衡器、每个上游的连接池
- **事件串联**：将 HealthChecker.onChange 绑定到 Balancer.RemoveFromMapping，形成状态联动
- **连接生命周期**：`acceptLoop` → `handleClientConn` → 每个 MuxConn 注册到 muxes 表
- **请求处理流水线**：`handleStream` = 选上游 → 借连接 → 双向拷贝 → 归还连接
- **优雅关闭**：Stop 依次关闭 listener→健康检查器→所有客户端 Mux→所有连接池，wg 保证协程全部退出

## 4. 连接多路复用协议设计

### 4.1 帧格式（二进制，大端序）

```
 0                   1                   2                   3
 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|          Type (16)           |         StreamID (16)         |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                         Length (32)                          |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
|                                                               |
|                       Payload (variable)                      |
|                                                               |
+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
```

**头部固定 8 字节**：
- Type (2B)：帧类型枚举值
- StreamID (2B)：逻辑流标识符，0 保留给心跳等控制帧
- Length (4B)：Payload 字节长度，允许为 0

**帧最大理论尺寸**：65535 流 × 4GB 载荷（实际受 TCP 缓冲区限制）

### 4.2 流建立与关闭时序

**主动建流（客户端 → 代理）**：
```
Client Mux                          Proxy Mux
    |                                    |
    |-- SYNC(StreamID=N) -------------->|
    |                                    |-- onStream(stream[N])
    |-- DATA(StreamID=N, payload) ----->|
    |                                    |-- stream[N].pushData(payload)
    |                                    |
    |<---- DATA(StreamID=N, resp) ------|
    |                                    |
    |-- FIN(StreamID=N) --------------->|
    |                                    |-- handleFIN → stream[N].Close()
    |<---- FIN(StreamID=N) -------------| （由 Close() 触发）
    |                                    |-- 双向 FIN → removeStream(N)
```

**异常流重置**：
- 当代理无法连接任何上游时，主动向客户端发送 `RST(StreamID=N)`
- 收到 RST 后立即释放流资源，不等待 FIN

### 4.3 心跳机制

- 帧类型：`FrameTypeHEARTBEAT`
- 流 ID：惯例使用 0
- 行为：接收方收到 HEARTBEAT 后立即回发同类型帧
- 可选用途：客户端/代理空闲时发送以防止中间设备超时断开

## 5. 核心流程

### 5.1 请求处理总流程

```
客户端 TCP 连接到达
    │
    ├─ 包装为 MuxConn（readLoop 后台启动）
    │
    ├─ [客户端发起 SYNC 建流]
    │     │
    │     ├─ MuxConn.handleSYNC → 创建 Stream
    │     └─ 触发 onStream 回调 → handleStream(s, clientIP)
    │           │
    │           ├─ [选上游]
    │           │    ├─ EnableStickySession=true
    │           │    │    └─ Balancer.GetUpstream(clientIP)
    │           │    └─ EnableStickySession=false
    │           │         └─ hashIP % len(healthy)
    │           │
    │           ├─ [借连接] pools[upstream].Get()
    │           │    ├─ 失败 → 发 RST 帧，流结束
    │           │    └─ 成功 → pooledConn
    │           │
    │           ├─ 启动双协程双向拷贝：
    │           │    ├─ goroutine A: Stream.Read → upstream.Write
    │           │    └─ goroutine B: upstream.Read → Stream.Write
    │           │
    │           └─ wg.Wait → Close(Stream) → Close(pooledConn) → 归还连接
    │
    └─ MuxConn 底层断开 → cleanupOnError → 清理所有残余流
```

### 5.2 健康检查流程

```
Start()
  │
  └─ checkLoop（ticker 驱动）
       │
       └─ checkAll()
            │
            ├─ RLock 取所有 addr 快照
            └─ 对每个 addr 并发安全地 checkOne：
                 │
                 ├─ RLock → 取 Upstream 指针
                 ├─ （锁外）执行 Probe 网络 IO
                 └─ Lock → 更新 failCount/passCount，达阈值则翻转状态 + 回调
```

### 5.3 连接池归还与回收流程

```
pooledConn.Close()
  │
  └─ pool.Put(pc)
       │
       ├─ closed → 真关闭
       ├─ 归还时已超过 IdleTimeout → 真关闭
       └─ 正常 → idleList.PushBack + Signal 唤醒一个等待者

idleTimeoutLoop（ticker = IdleTimeout/2）
  │
  └─ reclaimIdle()
       ├─ 扫描 idleList，超时的收集到 expired
       ├─ Broadcast 唤醒所有 Get 等待者（释放了配额）
       └─ 锁外逐个 Close 过期连接
```

## 6. 线程安全与设计要点

| 组件 | 同步机制 | 关键设计 |
|------|----------|----------|
| MuxConn 写入 | writeMu | 帧头部必须原子写入，避免字节交错 |
| MuxConn.streams | RWMutex | 读多写少场景，数据分发用 RLock 降低冲突 |
| Stream 状态 | atomic.Bool + sync.Once | closed/finSent 用原子变量，channel 关闭用 Once |
| HealthChecker | RWMutex + 快照 | 探测 IO 移到锁外，仅状态更新持写锁 |
| ConnPool | Mutex + Cond | 所有状态修改持互斥锁，等待/唤醒用 Cond |
| Balancer.mapping | 先 RLock 再 Lock | 写入路径罕见，尽量缩短写锁持有时间 |
| TCPProxy | WaitGroup + stopCh | 所有后台协程统一注册 wg，Stop 处集中等待 |

**死锁预防关键**：
- `MuxConn.Close()` 不调用自身协程中的 wg.Wait()，cleanupOnError 仅执行清理不等待
- `Stream.readCh` 的关闭使用独立的 `chCloseOnce`，与 `closeOnce` 分离避免双重关闭
- 连接池的广播（Broadcast）总是先修改状态后释放锁，防止唤醒后立即再次阻塞

### 6.1 并发安全保证策略

本节详细说明各组件在并发场景下的安全保证机制，涵盖已修复的竞态窗口和资源管理缺陷。

#### 6.1.1 连接池 Remove/Put 双重操作防护

**问题**：`handleStream` 中，当客户端到上游的转发协程 `s.Read` 返回错误时，会调用 `pool.Remove` 移除上游连接。但 `defer upstreamConn.Close()` 随后又会调用 `pool.Put` 归还同一连接。两次操作分别对 `activeCount` 减 1，导致计数出现负数，已关闭的连接被放回空闲列表。

**修复**：
- `poolConn` 新增 `removed atomic.Bool` 标志位
- `pool.Remove` 首次调用时 `removed.Swap(true)` 返回 false，正常执行 activeCount-- 和关闭；再次调用时 Swap 返回 true，直接跳过
- `pool.Put` 在 `activeCount--` 后检查 `pc.removed.Load()`，若为 true 则直接返回，不再将连接放回空闲列表
- `pooledConn.Close` 同时检查 `idle` 和 `removed` 标志，任一为 true 均跳过归还

```
时序：Remove 先于 Close
  goroutine A: pool.Remove(pc) → removed=true, activeCount=0
  goroutine B: pooledConn.Close() → pool.Put(pc) → removed=true → 跳过归还
  结果：activeCount=0, idleList 不受污染 ✓
```

#### 6.1.2 ConnPool.Get 空闲连接过期扫描安全

**问题**：原实现在 for 循环中检测到空闲连接超时后，先解锁关闭连接再重新加锁。锁释放期间 `reclaimIdle` 或 `Put` 可能修改 `idleList` 长度，导致循环索引越界。

**修复**：将过期检测、有效连接筛选和连接选取全部在单次持锁内完成：
1. 从尾到头遍历 idleList，将过期连接收集到 `expired` 切片，将有效连接紧凑排列到 `idleList[:validIdx]`
2. 在有效连接中选取最尾部的连接作为 `selected`
3. 从 idleList 中移除 selected 并调整 activeCount
4. 解锁后关闭过期连接

整个过程中，idleList 的修改完全在锁内完成，不会出现中途解锁导致的索引失效。

#### 6.1.3 IPHashBalancer 与 HealthChecker 锁顺序一致性

**问题**：原 `GetUpstream` 在持有 balancer 的 RLock 时调用 `hc.GetHealthyUpstreams()`（获取 hc 的 RLock），而 `onChange` 回调路径是 `hc.Lock` → `balancer.Lock`，两条路径锁顺序不一致，存在 ABBA 死锁风险。

**修复**：
- 新增 `getHealthyUpstreamsSnapshot()` 方法，先获取健康列表快照，再操作 balancer 自身的锁
- `GetUpstream` 调用快照方法获取 healthy 列表后，才进入 balancer 的 RLock/Lock 区域
- 保证锁顺序始终为：先释放 hc 锁，再获取 balancer 锁，不存在嵌套持锁

```
修复前锁顺序：
  路径1: RLock(balancer) → RLock(hc)        // GetUpstream
  路径2: Lock(hc) → Lock(balancer)           // onChange 回调
  → ABBA 死锁风险

修复后锁顺序：
  路径1: RLock(hc) → Unlock(hc) → RLock(balancer)  // GetUpstream (先拿快照，再操作 balancer)
  路径2: Lock(hc) → Unlock(hc) → Lock(balancer)     // onChange 回调 (hc.Unlock 在 defer 中，回调在锁内调用)
  → 同方向，无死锁风险
```

#### 6.1.4 handleFin/handleRst 原子化流操作

**问题**：原 `handleFin` 和 `handleRst` 采用 check-then-act 模式：先 RLock 检查流是否存在，再 Lock 执行操作。两个方法并发处理同一 StreamID 时，可能在 RLock 释放后、Lock 获取前被另一个方法抢先处理，导致对已删除的流重复操作。

**修复**：
- `handleFin` 改为全程持写锁（Lock），在锁内完成：检查流存在 → 设置 finRecv → 条件删除 → 设置 closed → 关闭 readCh
- `handleRst` 保持全程持写锁（Lock），在锁内完成：检查流存在 → 删除流 → 设置 closed → 关闭 readCh
- 两个方法对同一 StreamID 的操作完全串行化，消除竞态窗口
- `chCloseOnce.Do` 作为最后防线，即使两者对同一流操作也不会 panic

#### 6.1.5 HealthChecker.checkOne 上下文一致性验证

**问题**：`checkOne` 在 RLock 读取 uh.upstream 指针后释放锁执行探测，然后在 Lock 中重新查找 uh。如果上游在两次加锁之间被删除后以相同地址重新添加，uh 指向的是新的 `upstreamHealth` 对象，但 failCount/passCount 可能继承旧实例的状态。

**修复**：在 Lock 内重新获取 uh 后，增加指针一致性校验：
```go
uh, ok = hc.upstreams[addr]
if !ok { return }
if uh.upstream != upstream { return }  // upstream 是 RLock 阶段保存的指针
```
如果 `uh.upstream` 与 RLock 阶段保存的指针不一致，说明上游已被替换，跳过本次更新。新的 upstreamHealth 对象从零开始计数，不会继承旧状态。

#### 6.1.6 handleStream 协程退出与连接归还

**问题**：原 `handleStream` 使用 `defer upstreamConn.Close()` 在函数返回时归还连接，但 `wg.Wait()` 等待两个双向拷贝协程退出。当代理关闭导致流端 Read 返回 EOF 时，stream→upstream 协程退出，但 upstream→stream 协程可能阻塞在 `upstreamConn.Read()` 上，wg.Wait 永远不返回。

**修复**：
- 移除 `defer upstreamConn.Close()`，改为在 `wg.Wait()` 之后显式调用 `upstreamConn.Close()`
- 引入 `closeUpstream` 函数（由 `sync.Once` 保护），任一方向退出时调用 `upstreamConn.SetDeadline(time.Now())`
- SetDeadline 强制阻塞的 Read/Write 立即返回 deadline 错误，使另一方向协程也能退出
- 两个协程都退出后 `wg.Wait()` 返回，`upstreamConn.Close()` 正确归还连接到池中

## 7. 使用示例

### 7.1 基础使用 - 启动反向代理

```go
package main

import (
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "solocoder-go/internal/tcpproxy"
)

func main() {
    cfg := tcpproxy.ProxyConfig{
        ListenAddress:     "0.0.0.0:8080",
        Upstreams:         []string{"10.0.0.1:9000", "10.0.0.2:9000", "10.0.0.3:9000"},
        PoolMaxConns:      50,
        PoolIdleTimeout:   10 * time.Minute,
        PoolWaitTimeout:   3 * time.Second,
        HealthCheckConfig: tcpproxy.HealthCheckerConfig{
            CheckInterval: 5 * time.Second,
            ProbeTimeout:  2 * time.Second,
            FailThreshold: 2,
            PassThreshold: 2,
        },
        EnableStickySession: true,
    }

    proxy, err := tcpproxy.NewTCPProxy(cfg)
    if err != nil {
        log.Fatalf("create proxy: %v", err)
    }

    if err := proxy.Start(); err != nil {
        log.Fatalf("start proxy: %v", err)
    }
    log.Printf("TCP proxy listening on %s", proxy.Addr())

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    log.Println("shutting down...")
    if err := proxy.Stop(); err != nil {
        log.Fatalf("stop proxy: %v", err)
    }
    log.Println("graceful shutdown complete")
}
```

### 7.2 客户端使用多路复用连接

```go
package main

import (
    "fmt"
    "io"
    "net"
    "sync"

    "solocoder-go/internal/tcpproxy"
)

func main() {
    conn, err := net.Dial("tcp", "proxy.example.com:8080")
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // 一条物理连接承载多路复用
    mux := tcpproxy.NewMuxConn(conn, nil)
    defer mux.Close()

    // 并发发起 10 个逻辑请求，只消耗 1 条 TCP
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(idx int) {
            defer wg.Done()

            stream, err := mux.NewStream()
            if err != nil {
                fmt.Printf("stream %d: create fail: %v\n", idx, err)
                return
            }
            defer stream.Close()

            req := fmt.Sprintf("REQUEST-%d", idx)
            stream.Write([]byte(req))

            resp, _ := io.ReadAll(stream)
            fmt.Printf("stream %d: %s → %s\n", idx, req, resp)
        }(i)
    }
    wg.Wait()
}
```

### 7.3 独立使用组件 - 连接池

```go
// 仅使用连接池，不启用整个代理
upstream := tcpproxy.NewUpstream("db.internal:5432")
pool := tcpproxy.NewConnPool(upstream, tcpproxy.ConnPoolConfig{
    MaxConns:    20,
    IdleTimeout: 5 * time.Minute,
    WaitTimeout: 2 * time.Second,
})
defer pool.Close()

conn, err := pool.Get()
if err == tcpproxy.ErrPoolExhausted {
    // 降级逻辑：排队 / 拒绝 / 熔断
    return
}
defer conn.Close() // 归还而非真关闭

conn.Write([]byte("PING\r\n"))
```

### 7.4 独立使用组件 - 健康检查器

```go
hc := tcpproxy.NewHealthChecker(tcpproxy.HealthCheckerConfig{
    CheckInterval: 3 * time.Second,
    FailThreshold: 3,
    PassThreshold: 1,
})
hc.SetOnChange(func(addr string, healthy bool) {
    if !healthy {
        log.Printf("ALERT: upstream %s is DOWN", addr)
    } else {
        log.Printf("INFO: upstream %s is back UP", addr)
    }
})

for _, addr := range backendList {
    hc.AddUpstream(tcpproxy.NewUpstream(addr))
}
hc.Start()
defer hc.Stop()

// 业务代码中随时获取健康列表
for _, u := range hc.GetHealthyUpstreams() {
    fmt.Println(u.Address)
}
```

## 8. 测试覆盖策略

| 测试类别 | 覆盖范围 | 代表测试函数 |
|----------|----------|-------------|
| 帧协议 | 6 种类型编解码、nil 帧、短帧截断 | TestEncodeDecodeFrame, TestDecodeFrame_ShortHeader, TestFrame_AllTypes |
| 多路复用 | 单流通信、多流并发、流关闭幂等、心跳处理、RST 重置 | TestMuxConn_NewStreamAndCommunicate, TestMuxConn_MultipleStreams, TestMuxConn_RSTFrame |
| 健康检查 | 加删上游、故障检测、恢复检测、回调触发、空列表 | TestHealthChecker_DetectFailure, TestHealthChecker_OnChange, TestHealthChecker_StartStop |
| 连接池 | 借还复用、MaxConns 限流、超时返回、空闲回收、并发安全、双关归还 | TestConnPool_GetAndPut, TestConnPool_MaxConns, TestConnPool_IdleTimeout, TestConnPool_ConcurrentAccess |
| 负载均衡 | 哈希一致性、粘性映射、不健康绕行、空上游异常、并发哈希、分布均匀性 | TestIPHashBalancer_HashConsistency, TestIPHashBalancer_StickySession, TestIPHashBalancer_Concurrency, TestIPHashBalancer_DifferentIPs |
| 端到端 | 配置校验、启停生命周期、完整 Echo 请求链路、大数据分片传输 | TestNewTCPProxy_Validation, TestTCPProxy_StartStop, TestTCPProxy_EndToEnd, TestStream_ReadLargeData |
| 并发安全 | Remove/Put 双重计数、Remove 幂等、连接归还竞态、空闲过期索引、死锁探测、FIN/RST 竞态、计数器重置 | 见下方详细列表 |

**并发安全专项测试**：

| 测试函数 | 验证目标 |
|----------|----------|
| TestConnPool_RemoveThenCloseNoNegativeCount | Remove 后 Close 不导致 activeCount 负数 |
| TestConnPool_RemoveIdempotent | 多次 Remove 同一连接 activeCount 只减一次 |
| TestConnPool_PutAfterRemovedNoReturnToIdle | Remove 后 Put 不将已关闭连接放回空闲列表 |
| TestConnPool_ConcurrentRemoveAndPut | 并发 Remove+Put 不出现负计数或脏连接 |
| TestConnPool_GetIdleExpiryNoIndexPanic | 空闲过期并发 Get 不触发索引越界 panic |
| TestIPHashBalancer_NoDeadlockWithHealthChecker | HC onChange 回调与 GetUpstream 并发不死锁 |
| TestMuxConcurrentFinAndRst | 同一 StreamID 的 FIN/RST 并发到达不 panic |
| TestHealthChecker_RemoveAndReaddResetsCounters | 删除重加后计数器从零开始，不继承旧状态 |
| TestConnPool_ConcurrentGetWithIdleExpiry | 极短空闲超时 + 高并发 Get/Put 不出现计数异常 |

**并发测试设计要点**：
- `TestConnPool_ConcurrentAccess`：20 协程 × 50 次借还循环，验证 Mutex + Cond 的正确性
- `TestIPHashBalancer_Concurrency`：100 协程 × 100 次查询，验证 RLock→Lock 升级无竞争
- `TestIPHashBalancer_NoDeadlockWithHealthChecker`：HC checkLoop 持续运行 + 100 次 GetUpstream 交替，5 秒超时检测死锁
- `TestMuxConcurrentFinAndRst`：10 轮试验，每轮对同一 Stream 并发写 FIN 和 RST 帧

## 9. 文件结构

```
internal/tcpproxy/
├── tcpproxy.go       # 所有核心实现（帧/流/Mux/健康/池/均衡/代理）
└── tcpproxy_test.go  # 单元测试（45 个用例，覆盖正常/边界/异常/并发安全）

docs/
└── tcpproxy.md       # 本文档
```
