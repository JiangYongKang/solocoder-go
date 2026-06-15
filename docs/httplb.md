# HTTP 负载均衡器 (HTTP LB) 模块需求文档

## 1. 模块概述

HTTP 负载均衡器是一个通用的请求分发组件，用于将 HTTP 请求按照指定的调度算法分发到多个后端服务器，实现流量的均衡分配、后端服务器的动态管理和高可用保障。

本模块提供四种经典的负载均衡调度算法：轮询、最少连接、加权轮询和一致性哈希，支持后端服务器的动态上下线和优雅下线，所有操作均为并发安全。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 轮询调度 (Round Robin) | 按后端服务器列表顺序依次分发请求，到达末尾后回到开头循环 |
| F2 | 最少连接调度 (Least Connections) | 选择当前活跃连接数最少的服务器处理请求，连接数相同时按列表顺序选择 |
| F3 | 加权轮询调度 (Weighted Round Robin) | 根据服务器权重比例分发请求，权重越高被选中频率越高，使用平滑加权轮询算法 |
| F4 | 一致性哈希调度 (Consistent Hash) | 基于请求键（URL 路径或客户端 IP）计算哈希值，通过一致性哈希环映射到后端服务器，目标节点不健康时沿环顺时针故障转移到下一个健康节点 |
| F5 | 动态添加服务器 | 运行期间动态添加新的后端服务器，添加后立即参与调度 |
| F6 | 动态移除服务器 | 运行期间将已有服务器从调度列表中安全移除（需先 Drain 且活跃连接为 0） |
| F7 | 服务器优雅下线 (Drain) | 将服务器标记为 draining 状态，完成当前请求后停止接收新请求 |
| F8 | 服务器恢复上线 (Restore) | 将下线中的服务器重新恢复为可用状态 |
| F9 | 连接计数 | 记录每个后端服务器的当前活跃请求数 |
| F10 | HTTP 处理器集成 | 提供标准 `http.Handler` 接口，可直接嵌入 HTTP 服务 |

## 3. 核心结构体与职责

### 3.1 BackendServer - 后端服务器

```go
type BackendServer struct {
    activeConn int64      // 当前活跃连接数（原子操作，需64位对齐）
    Address    string     // 服务器地址
    Weight     int        // 权重值
    status     ServerStatus // 服务器状态
    mu         sync.RWMutex // 保护状态的读写锁
}
```

**主要职责：**
- 维护服务器的基本信息（地址、权重）
- 跟踪服务器的运行状态（上线/优雅下线/下线）
- 记录当前活跃请求连接数
- 提供状态转换和连接计数的线程安全操作

**状态转换：**
- `StatusUp`：正常运行，可接收新请求
- `StatusDraining`：优雅下线中，已接收的请求继续处理，但不接收新请求
- `StatusDown`：已完全下线

**关键方法：**
- `IsHealthy()`：检查服务器是否处于 StatusUp 状态
- `IsDraining()`：检查服务器是否处于 StatusDraining 状态
- `ActiveConn()`：获取当前活跃连接数（原子操作）
- `IncConn()` / `DecConn()`：增减活跃连接计数（原子操作）
- `MarkDraining()`：将 StatusUp 转为 StatusDraining
- `MarkDown()`：将服务器标记为 StatusDown
- `MarkUp()`：将服务器恢复为 StatusUp

### 3.2 ServerPool - 服务器池

```go
type ServerPool struct {
    servers map[string]*BackendServer // 服务器映射表
    order   []string                  // 服务器顺序列表
    mu      sync.RWMutex              // 保护池状态的读写锁
}
```

**主要职责：**
- 管理后端服务器集合的增删改查
- 维护服务器的列表顺序（用于轮询等有序调度）
- 提供健康服务器列表查询
- 作为各调度算法的基础数据结构

### 3.3 Balancer - 调度器接口

```go
type Balancer interface {
    Next(key string) (*BackendServer, error)  // 选择下一个后端服务器
    Servers() []*BackendServer                 // 获取所有服务器
    AddServer(address string, weight int) error    // 添加服务器
    RemoveServer(address string) error         // 移除服务器
    DrainServer(address string) error          // 优雅下线
    RestoreServer(address string) error        // 恢复上线
    ServerCount() int                          // 服务器总数
    HealthyCount() int                         // 健康服务器数
}
```

**主要职责：**
- 定义所有负载均衡算法的统一接口
- 支持动态服务器管理
- 提供服务器状态查询

### 3.4 RoundRobin - 轮询调度器

```go
type RoundRobin struct {
    counter uint64        // 轮询计数器（原子操作，需64位对齐）
    pool    *ServerPool   // 服务器池
}
```

**主要职责：**
- 实现轮询调度算法
- 维护轮询计数器（使用 `sync/atomic` 原子操作，无需额外互斥锁）
- 每次请求按顺序选择下一个健康服务器

### 3.5 LeastConnections - 最少连接调度器

```go
type LeastConnections struct {
    pool *ServerPool  // 服务器池
}
```

**主要职责：**
- 实现最少连接调度算法
- 每次请求选择当前活跃连接数最少的健康服务器
- 连接数相同时按列表顺序选择（保证确定性）

### 3.6 WeightedRoundRobin - 加权轮询调度器

```go
type WeightedRoundRobin struct {
    pool     *ServerPool      // 服务器池
    weighted []*weightedServer // 带权重状态的服务器列表
    mu       sync.Mutex       // 互斥锁
}

type weightedServer struct {
    server        *BackendServer // 后端服务器引用
    currentWeight int            // 当前权重（平滑轮询状态）
}
```

**主要职责：**
- 实现平滑加权轮询算法（Nginx 风格）
- 维护每个服务器的当前权重状态
- 保证在足够多次选择后，各服务器的请求比例接近权重比例
- 分发过程平滑，避免突发流量集中到高权重服务器

**平滑加权轮询算法原理：**
1. 每个服务器有 `weight`（配置权重）和 `currentWeight`（当前权重）
2. 每次选择时，将所有服务器的 `currentWeight += weight`
3. 选择 `currentWeight` 最大的服务器
4. 选中后，该服务器的 `currentWeight -= totalWeight`（总权重）

### 3.7 ConsistentHash - 一致性哈希调度器

```go
type ConsistentHash struct {
    pool         *ServerPool   // 服务器池
    virtualNodes int           // 虚拟节点数
    ring         []hashNode    // 哈希环（已排序）
    hashMap      map[uint64]string // 哈希值到服务器地址的映射
    mu           sync.RWMutex  // 读写锁
}

type hashNode struct {
    hash   uint64  // 哈希值
    server string  // 服务器地址
}
```

**主要职责：**
- 实现一致性哈希调度算法
- 维护哈希环和虚拟节点
- 基于请求键计算哈希并映射到对应服务器
- 目标节点不健康时沿哈希环顺时针故障转移到下一个健康节点
- 服务器增减时尽量减少受影响的请求范围

**虚拟节点机制：**
- 每个物理服务器对应多个虚拟节点（数量 = virtualNodes * weight）
- 虚拟节点通过 `地址#vn编号` 的格式生成哈希值
- 虚拟节点越多，哈希分布越均匀
- 默认虚拟节点数：100

**故障转移机制：**
当通过哈希计算找到目标服务器节点后，若该节点处于 Draining/Down 状态，`Next` 方法不会直接返回错误，而是沿哈希环顺时针方向继续查找下一个健康节点，直到遍历所有物理服务器。使用 `visited` 集合去重，确保每个物理服务器只检查一次。只有当所有物理服务器都不健康时才返回 `ErrNoHealthyServer`。

### 3.8 HTTPLoadBalancer - HTTP 负载均衡器

```go
type HTTPLoadBalancer struct {
    balancer    Balancer               // 底层调度器
    hashKeyFunc func(*http.Request) string // 哈希键生成函数
}
```

**主要职责：**
- 提供 HTTP 层面的负载均衡入口
- 封装底层调度器，提供统一的创建和使用接口
- 支持自定义哈希键生成函数（用于一致性哈希）
- 实现 `http.Handler` 接口，可直接作为 HTTP 处理器

## 4. 四种调度算法适用场景

### 4.1 轮询 (Round Robin)

**适用场景：**
- 后端服务器性能相近，配置相同
- 请求处理时间相对均匀
- 对会话保持（Session Affinity）无要求
- 简单的无状态服务负载均衡

**优点：**
- 实现简单，开销小
- 分配公平，每台服务器分到的请求数基本相等
- 可预测性强

**缺点：**
- 不考虑服务器实际负载，可能导致某些服务器过载
- 无法处理服务器性能差异

### 4.2 最少连接 (Least Connections)

**适用场景：**
- 请求处理时间差异较大的场景（如长请求、文件下载）
- 后端服务器性能相近但负载波动大
- 需要根据实际负载动态调整的场景
- 数据库连接、长连接服务

**优点：**
- 能根据服务器实际负载动态调整
- 避免慢服务器堆积请求
- 对请求时长不均的场景适应性好

**缺点：**
- 实现复杂度略高（需要维护连接计数）
- 新上线服务器可能瞬间接收大量请求（缓存预热问题）

### 4.3 加权轮询 (Weighted Round Robin)

**适用场景：**
- 后端服务器性能差异较大（如高配和低配机器混用）
- 需要按比例分配流量的场景
- 灰度发布、金丝雀发布（逐步调整权重）
- 希望精确控制各服务器流量比例

**优点：**
- 可根据服务器性能精确分配流量
- 平滑加权轮询算法避免突发流量
- 适合灰度发布场景

**缺点：**
- 需要预先评估服务器性能并设置合适的权重
- 权重调整需要人工干预或配合监控系统

### 4.4 一致性哈希 (Consistent Hash)

**适用场景：**
- 需要会话保持（Session Affinity）的场景
- 缓存服务（如 Redis 分片、CDN 缓存）
- 分布式存储系统（如分布式文件系统）
- 服务器频繁增减的动态环境
- 希望同一客户端/同一资源始终路由到同一服务器

**优点：**
- 服务器增减时只有少量请求受影响（约 1/N）
- 支持会话保持，无需集中式 Session 存储
- 适合缓存场景，提高缓存命中率
- 虚拟节点机制保证分布均匀性
- 目标节点不健康时自动故障转移到环上最近的健康节点

**缺点：**
- 实现复杂度较高
- 需要选择合适的哈希键（URL 路径、客户端 IP 等）
- 服务器性能差异大时需要配合权重调整

## 5. 动态上下线机制

### 5.1 添加服务器 (AddServer)

```
AddServer(address, weight)
   │
   ├─ 检查地址是否已存在 → 已存在则返回 ErrServerExists
   ├─ 验证权重合法性 → 权重 <= 0 返回 ErrInvalidWeight
   ├─ 创建 BackendServer 实例，状态为 StatusUp
   ├─ 加入 ServerPool 的 servers map 和 order 列表
   └─ 返回 nil
```

### 5.2 移除服务器 (RemoveServer)

```
RemoveServer(address)
   │
   ├─ 检查地址是否存在 → 不存在返回 ErrServerNotFound
   ├─ 检查服务器是否处于 Draining 状态 → 未 Draining 则返回 ErrServerNotDraining
   ├─ 检查活跃连接数 → 仍有活跃连接则返回 ErrServerHasConns
   ├─ 从 servers map 中删除
   ├─ 从 order 列表中删除
   └─ 返回 nil
```

**安全移除约束：**
`RemoveServer` 强制要求满足两个前置条件才能执行：
1. 服务器必须已通过 `DrainServer` 标记为 Draining 状态，防止正在接收请求的服务器被意外移除
2. 服务器的活跃连接数必须为 0，确保正在处理的请求不会因服务器被移除而丢失连接计数

**推荐的完整下线流程：**
1. 调用 `DrainServer(address)` 停止接收新请求
2. 等待该服务器的活跃连接数降为 0（通过 `server.ActiveConn()` 轮询或事件通知）
3. 调用 `RemoveServer(address)` 安全移除

### 5.3 优雅下线 (DrainServer)

```
DrainServer(address)
   │
   ├─ 检查地址是否存在 → 不存在返回 ErrServerNotFound
   ├─ 将服务器状态从 StatusUp 改为 StatusDraining
   └─ 返回 nil
```

**优雅下线特点：**
- 下线中的服务器不再接收新请求
- 已接收的请求继续处理，连接计数正常维护
- 所有请求处理完成后（ActiveConn() == 0），可安全调用 RemoveServer 完全移除
- 可通过 RestoreServer 恢复上线

### 5.4 恢复上线 (RestoreServer)

```
RestoreServer(address)
   │
   ├─ 检查地址是否存在 → 不存在返回 ErrServerNotFound
   ├─ 将服务器状态改为 StatusUp
   └─ 返回 nil
```

## 6. 一致性哈希故障转移机制

### 6.1 故障转移流程

```
Next(key)
   │
   ├─ 计算请求键的哈希值
   ├─ 在哈希环上二分查找，定位到起始节点
   ├─ [遍历哈希环]
   │     ├─ 取当前位置的虚拟节点对应的服务器地址
   │     ├─ 已检查过的服务器 → 跳过（visited 去重）
   │     ├─ 检查服务器是否存在且健康（IsHealthy）
   │     │     ├─ 健康 → IncConn()，返回该服务器
   │     │     └─ 不健康 → 继续顺时针查找
   │     └─ 回到环起点后继续，直到遍历完所有物理服务器
   ├─ 所有服务器都不健康 → 返回 ErrNoHealthyServer
   └─ 返回 (*BackendServer, error)
```

### 6.2 故障转移保证

- **最小影响原则**：当一台服务器被 Drain 后，原本哈希到该服务器的请求会被故障转移到环上顺时针方向最近的健康服务器，而非全部失败
- **确定性路由**：同一个键在健康服务器集合不变的情况下始终路由到同一个服务器
- **去重保证**：通过 `visited` 集合确保每个物理服务器只被检查一次，避免同一台不健康服务器上的多个虚拟节点导致重复检查

## 7. 线程安全设计

本模块所有对外接口均为并发安全：

- **BackendServer**：状态变更使用 `sync.RWMutex` 保护，连接计数使用 `sync/atomic` 原子操作
- **ServerPool**：使用 `sync.RWMutex` 保护服务器列表和映射
- **RoundRobin**：计数器使用 `sync/atomic` 原子操作，无额外互斥锁
- **WeightedRoundRobin**：权重状态使用 `sync.Mutex` 保护
- **ConsistentHash**：哈希环使用 `sync.RWMutex` 保护

**64 位对齐注意事项：**
- 在 32 位架构上，64 位原子操作要求 8 字节对齐
- 所有包含 `int64`/`uint64` 字段的结构体，将 64 位字段放在结构体最前面
- 受影响结构体：`BackendServer`（activeConn）、`RoundRobin`（counter）

## 8. 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrServerExists` | 服务器已存在 | 添加重复地址的服务器 |
| `ErrServerNotFound` | 服务器不存在 | 移除/下线不存在的服务器 |
| `ErrNoHealthyServer` | 无健康服务器 | 所有服务器均不可用时调用 Next |
| `ErrInvalidWeight` | 无效权重 | 权重 <= 0 |
| `ErrServerHasConns` | 服务器仍有活跃连接 | 移除服务器时检测到活跃连接数 > 0 |
| `ErrServerNotDraining` | 服务器未处于 Draining 状态 | 移除服务器时服务器状态不是 StatusDraining |

## 9. 使用示例

### 9.1 基础使用：轮询负载均衡

```go
package main

import (
    "fmt"
    "net/http"
    "solocoder-go/internal/httplb"
)

func main() {
    cfg := httplb.Config{
        Algorithm: httplb.AlgorithmRoundRobin,
        Servers:   []string{"backend1:8080", "backend2:8080", "backend3:8080"},
    }

    lb, err := httplb.NewHTTPLoadBalancer(cfg)
    if err != nil {
        panic(err)
    }

    http.Handle("/", lb)
    http.ListenAndServe(":8080", nil)
}
```

### 9.2 加权轮询：性能异构集群

```go
cfg := httplb.Config{
    Algorithm: httplb.AlgorithmWeightedRR,
    Servers:   []string{"high-perf:8080", "mid-perf:8080", "low-perf:8080"},
    Weights:   []int{5, 3, 2}, // 5:3:2 的流量分配比例
}

lb, _ := httplb.NewHTTPLoadBalancer(cfg)
```

### 9.3 一致性哈希：缓存服务

```go
cfg := httplb.Config{
    Algorithm:    httplb.AlgorithmConsistentHash,
    Servers:      []string{"cache1:6379", "cache2:6379", "cache3:6379"},
    VirtualNodes: 150,
    HashKeyFunc: func(r *http.Request) string {
        return r.URL.Path // 基于 URL 路径做哈希，保证同一路由到同一服务器
    },
}

lb, _ := httplb.NewHTTPLoadBalancer(cfg)
```

### 9.4 基于客户端 IP 的会话保持

```go
cfg := httplb.Config{
    Algorithm:    httplb.AlgorithmConsistentHash,
    Servers:      []string{"app1:8080", "app2:8080", "app3:8080"},
    HashKeyFunc: func(r *http.Request) string {
        return r.RemoteAddr // 基于客户端 IP 做哈希
    },
}
```

### 9.5 动态管理服务器（完整的优雅下线流程）

```go
lb, _ := httplb.NewHTTPLoadBalancer(cfg)

// 添加新服务器
lb.AddServer("new-backend:8080", 2)

// 步骤 1：优雅下线服务器（停止接收新请求）
lb.DrainServer("old-backend:8080")

// 步骤 2：等待现有请求处理完毕
// 可通过查询活跃连接数判断：
//   server, _ := lb.Balancer().(*ConsistentHash).pool.GetServer("old-backend:8080")
//   server.ActiveConn() == 0 时表示所有请求已处理完毕

// 步骤 3：安全移除服务器（仅当 Draining 且 ActiveConn==0 时允许）
err := lb.RemoveServer("old-backend:8080")
if err == httplb.ErrServerHasConns {
    // 仍有活跃请求，稍后重试
}
if err == httplb.ErrServerNotDraining {
    // 忘记调用 DrainServer
}

// 如果需要取消下线（在 RemoveServer 之前）
lb.RestoreServer("old-backend:8080")
```

### 9.6 直接使用调度器（非 HTTP 场景）

```go
// 直接创建轮询调度器
rr, _ := httplb.NewRoundRobin([]string{"s1:8080", "s2:8080"})

server, err := rr.Next("")
if err != nil {
    // 处理错误
}
defer server.DecConn() // 请求处理完成后减少连接计数

fmt.Printf("Routing to: %s\n", server.Address)
```

### 9.7 一致性哈希的故障转移

```go
ch, _ := httplb.NewConsistentHash([]string{"s1:8080", "s2:8080", "s3:8080"}, 100)

// 正常情况：同一 key 始终路由到同一服务器
s, _ := ch.Next("/api/users/123")
fmt.Printf("Routed to: %s\n", s.Address)
s.DecConn()

// 当 s1 下线后，原来哈希到 s1 的请求自动故障转移到环上最近的健康节点
ch.DrainServer("s1:8080")
s, err := ch.Next("/api/users/123") // 不会返回错误（只要还有健康节点）
if err == httplb.ErrNoHealthyServer {
    // 所有节点都不健康
} else {
    fmt.Printf("Failover to: %s\n", s.Address) // 路由到 s2 或 s3
    s.DecConn()
}
```

## 10. 文件结构

```
internal/httplb/
├── server.go          # 后端服务器与服务器池
├── balancer.go        # 调度器接口定义
├── round_robin.go     # 轮询调度算法
├── least_conn.go      # 最少连接调度算法
├── weighted_rr.go     # 加权轮询调度算法
├── consistent_hash.go # 一致性哈希调度算法
├── httplb.go          # HTTP 负载均衡器主入口
└── httplb_test.go     # 单元测试（覆盖正常流程、边界条件、异常分支、故障转移）

docs/
└── httplb.md          # 本文档
```
