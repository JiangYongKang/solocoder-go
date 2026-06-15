# DNS 解析器 (DNSResolver) 模块需求文档

## 1. 模块概述

DNS 解析器是一个功能完整的 DNS 客户端组件，支持递归解析、迭代解析、并行查询、缓存与 TTL 管理等核心功能。模块实现了完整的 DNS 协议编解码，能够从根域名服务器开始逐级解析，支持 CNAME 别名链追踪，并通过并行查询多个上游服务器实现最快响应优先返回策略。

本模块使用 Go 标准库 `net` 包进行 UDP 网络通信，通过可配置的上游服务器列表与根服务器列表，支持灵活的部署模式。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 递归解析 | 从根域名服务器开始逐级解析，支持 CNAME 别名链追踪，可配置最大递归深度防止死循环 |
| F2 | 迭代解析 | 通过配置的上游 DNS 服务器进行迭代查询，同样支持 CNAME 追踪 |
| F3 | 并行查询 | 向所有配置的上游服务器同时发起查询，不等待所有响应返回 |
| F4 | 最快响应优先 | 第一个返回有效解析结果的响应被采纳为最终答案，后续响应直接丢弃 |
| F5 | DNS 缓存 | 将解析结果缓存到内存中，查询时优先检查缓存 |
| F6 | TTL 管理 | 缓存条目携带 TTL 并自动过期，未命中或已过期则发起新的解析请求 |
| F7 | 缓存自动清理 | 后台协程定期清理过期缓存条目 |
| F8 | A/AAAA 记录解析 | 支持 IPv4 (A) 和 IPv6 (AAAA) 记录解析 |
| F9 | 线程安全 | 所有公共 API 都是并发安全的，支持多协程同时调用 |
| F10 | RCODE 错误处理 | 正确解析并传递 DNS 协议级响应码（NXDOMAIN、SERVFAIL、FORMERR、REFUSED 等），保留真实错误语义 |
| F11 | 事务 ID 校验 | UDP 环境下校验响应事务 ID 与查询是否匹配，防止接受无关响应包 |
| F12 | Context 取消传播 | 查询全程支持 context 取消，超时时立即关闭连接并停止 goroutine，避免资源泄漏 |
| F13 | CNAME 同区优化 | CNAME 追踪时基于 DNS 区域边界判定（isSameZone）判断目标域名是否在当前权威服务器的 zone 内，同区则直接复用当前服务器查询，避免重建整条 NS 链 |

## 3. 核心结构体与职责

### 3.1 Config - 解析器配置

```go
type Config struct {
    EnableCache       bool          // 是否启用缓存
    EnableRecursion bool          // 是否启用递归解析（true=递归，false=迭代）
    CacheTTL         time.Duration  // 默认缓存 TTL（当记录无 TTL 时使用）
    QueryTimeout     time.Duration  // DNS 查询超时时间
    MaxRecursionDepth int         // 最大递归深度，防止死循环
    UpstreamServers  []string      // 上游 DNS 服务器列表（迭代模式使用）
    RootServers    []string      // 根域名服务器列表（递归模式使用）
    CleanupInterval time.Duration  // 缓存清理间隔
}
```

**配置约束与默认值：**
- `EnableCache` 默认为 `true`
- `EnableRecursion` 默认为 `false`（迭代模式）
- `CacheTTL` 默认为 5 分钟
- `QueryTimeout` 默认为 5 秒
- `MaxRecursionDepth` 默认为 10
- `UpstreamServers` 默认为 `["8.8.8.8:53", "8.8.4.4:53"]`
- `RootServers` 默认为 13 个根域名服务器 IP
- `CleanupInterval` 默认为 1 分钟

### 3.2 Resolver - DNS 解析器主体

```go
type Resolver struct {
    cfg         Config                 // 配置快照
    mu          sync.RWMutex       // 保护内部状态的读写锁
    cache       map[string]*CacheEntry // 缓存表（domain:记录
    closed      bool                 // 解析器是否已关闭
    stopCh      chan struct{}      // 后台协程停止信号
    wg          sync.WaitGroup     // 后台协程同步等待组
    dialUDP     func(network, address string) (net.Conn, error) // UDP 拨号函数（可替换用于测试）
}
```

**主要职责：**
- 协调递归/迭代解析流程
- 管理 DNS 缓存的读写与过期
- 驱动并行查询与最快响应策略
- 驱动缓存自动清理后台协程
- 保证线程安全，通过读写锁实现同步

### 3.3 CacheEntry - 缓存条目

```go
type CacheEntry struct {
    Records   []DNSRecord // DNS 记录列表
    ExpiresAt time.Time   // 过期时间
    TTL       time.Duration // TTL 持续时间
}
```

**主要职责：**
- 存储解析结果记录
- 记录过期时间戳用于缓存有效性检查
- 保留原始 TTL 用于缓存管理

### 3.4 DNSRecord - DNS 资源记录

```go
type DNSRecord struct {
    Name  string // 记录所属域名
    Type  uint16 // 记录类型（A, AAAA, CNAME, NS 等）
    Class uint16 // 记录类（通常为 IN）
    TTL   uint32 // 存活时间（秒）
    Data  string // 记录数据（IP 地址或域名）
}
```

**主要职责：**
- 表示单个 DNS 资源记录
- 存储完整的记录元数据与数据

### 3.5 DNSResponse - DNS 响应结构

```go
type DNSResponse struct {
    TransactionID uint16      // 事务 ID
    RCode         uint16      // 响应码（RCODE）
    Flags         uint16      // 标志位
    Answers     []DNSRecord // 回答区记录
    Authorities []DNSRecord // 授权区记录
    Additionals []DNSRecord // 附加区记录
}
```

**主要职责：**
- 表示完整的 DNS 响应消息
- 包含回答、授权、附加三个区域的记录
- 携带事务 ID 和响应码用于校验

### 3.6 DNSError - DNS 协议错误

```go
type DNSError struct {
    RCODE uint16 // DNS 响应码
    Msg   string // 错误描述
}
```

**主要职责：**
- 表示 DNS 协议级别的错误（NXDOMAIN、SERVFAIL 等）
- 支持 `errors.Is()` 比较（按 RCODE 匹配）
- 支持 `errors.As()` 提取详细 RCODE 信息

**预定义 DNS 错误变量：**
| 错误变量 | RCODE | 含义 |
|----------|-------|------|
| `ErrNXDOMAIN` | 3 | 域名不存在（Name Error） |
| `ErrSERVFAIL` | 2 | 服务器失败（Server Failure） |
| `ErrFORMERR` | 1 | 格式错误（Format Error） |
| `ErrREFUSED` | 5 | 查询被拒绝（Refused） |
| `ErrTransactionIDMismatch` | - | 事务 ID 不匹配 |

### 3.7 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidConfig` | 配置无效 | 配置参数不合法 |
| `ErrResolverClosed` | 解析器已关闭 | 已关闭的解析器上调用 Resolve 方法 |
| `ErrMaxDepthExceeded` | 超出最大递归深度 | CNAME 链过长或递归过深 |
| `ErrNoUpstreamServers` | 无可用上游服务器 | 迭代模式下未配置上游服务器 |
| `ErrNoRecordsFound` | 未找到记录 | 域名不存在或无对应类型记录 |
| `ErrAllUpstreamsFailed` | 所有上游服务器失败 | 所有上游服务器查询均失败 |
| `ErrInvalidDomain` | 域名无效 | 请求的域名格式不合法 |
| `ErrInvalidResponse` | 响应无效 | DNS 响应格式错误 |
| `ErrQueryTimeout` | 查询超时 | 单次查询超时 |
| `ErrTransactionIDMismatch` | 事务 ID 不匹配 | UDP 响应事务 ID 与查询不匹配 |
| `ErrNXDOMAIN` | 域名不存在 | DNS 服务器返回 NXDOMAIN (RCODE=3) |
| `ErrSERVFAIL` | 服务器失败 | DNS 服务器返回 SERVFAIL (RCODE=2) |
| `ErrFORMERR` | 格式错误 | DNS 服务器返回 FORMERR (RCODE=1) |
| `ErrREFUSED` | 查询被拒绝 | DNS 服务器返回 REFUSED (RCODE=5) |

## 4. 递归解析执行流程

### 4.1 总览

递归解析从根域名服务器开始，逐级查询权威 DNS 服务器，直到获取最终的 IP 地址。整个过程遵循 DNS 协议规范，处理 NS 记录委派、胶水记录以及 CNAME 别名链。

### 4.2 递归解析流程 (resolveRecursive)

```
resolveRecursive(domain, qtype, depth)
   │
   ├─ depth >= MaxRecursionDepth → 返回 ErrMaxDepthExceeded
   │
   ├─ 检查缓存 → 命中且未过期 → 返回缓存结果
   │
   ├─ 拆分域名为标签（如 "www.example.com" → ["www", "example", "com"]）
   │
   ├─ servers = 根服务器列表
   │
   ├─ [从根开始逐级查询 NS 记录]
   │     │
   │     ├─ 当前 zone = "."（根）
   │     ├─ 查询 zone 的 NS 记录
   │     ├─ 解析响应中的 NS 记录和胶水记录
   │     ├─ 胶水记录优先使用（避免额外查询）
   │     ├─ 无胶水记录时递归解析 NS 服务器的 IP
   │     ├─ servers = 下一级权威服务器
   │     └─ 继续查询更具体的 zone（如 "com." → "example.com."）
   │
   ├─ 向最终权威服务器查询目标记录
   │
   ├─ 处理 CNAME 别名链
   │     └─ 存在 CNAME 记录
   │           ├─ depth+1 >= MaxRecursionDepth → ErrMaxDepthExceeded
   │           ├─ [CNAME 优化追踪 followCNAME]
   │           │     ├─ 当前响应已含目标记录 → 直接返回
   │           │     ├─ CNAME 与原域名同区 → 复用当前服务器查询
   │           │     └─ CNAME 跨区 → 回退到完整递归解析
   │           └─ 返回 CNAME 追踪结果
   │
   ├─ 过滤目标类型记录 → 无 → ErrNoRecordsFound
   │
   ├─ 存入缓存（使用记录中最小的 TTL）
   │
   └─ 返回结果
```

**CNAME 同区优化说明（修复后）：**
- 使用 DNS 区域边界判定（`isSameZone`）替代标签级后缀比较
- 递归解析过程中跟踪当前授权区域（zone），通过 NS 委派链自动确定
- 当 CNAME 目标域名在当前权威服务器的 zone 内时，直接向当前服务器查询
- 判定逻辑：`domain == zone` 或 `domain` 以 `."+zone"` 为后缀
- 例如 `www.example.com` → `cdn.example.com`：zone 为 `example.com.`，`cdn.example.com` 以 `.example.com` 为后缀，判定为同区，直接复用 example.com 权威服务器
- 跨区 CNAME 仍需完整递归解析，确保正确性
- 若当前响应已附带目标记录（如 A 记录），直接使用无需额外查询
- 修复前的问题：标签级比较要求 CNAME 标签数 ≥ 原始标签数且完全匹配后缀，导致 `www.example.com` → `cdn.example.com` 被误判为跨区（标签数相同但首标签不同），触发不必要的完整根递归

### 4.3 并行查询流程 (queryParallel)

```
queryParallel(servers, domain, qtype)
   │
   ├─ len(servers) == 0 → 返回 ErrNoUpstreamServers
   │
   ├─ 创建带超时的 context（ctx, cancel）
   │
   ├─ 为每个服务器启动 goroutine
   │     │
   │     ├─ 构建 DNS 查询报文（生成随机 TransactionID）
   │     ├─ 建立 UDP 连接
   │     ├─ 发送查询
   │     ├─ [异步读取响应]
   │     │     ├─ ctx 被取消 → 关闭连接，goroutine 退出
   │     │     └─ 读取完成 → 校验 TransactionID，解析响应
   │     └─ 将结果发送到结果通道
   │
   ├─ 等待第一个有效响应（有 Answers 或 Authorities）
   │     ├─ 收到有效响应 → 调用 cancel() 取消其他 goroutine
   │     ├─ 立即返回结果（不等待其他服务器）
   │     └─ 后续到达的响应直接丢弃
   │
   └─ 所有服务器均失败
   │     ├─ 返回第一个错误或 ErrAllUpstreamsFailed
   │
   └─ 所有服务器返回但无有效数据
         └─ 返回第一个有效响应（即使无 Answers）
```

**最快响应优先策略说明：**
- 使用 `sync/atomic` 原子计数器跟踪未完成请求数
- 结果通道使用缓冲通道缓冲所有服务器的结果
- 第一个有效响应（包含 Answers 或 Authorities）立即返回
- 后续响应被丢弃，不会被处理
- 使用 `context.WithTimeout` 控制整体超时

**Context 取消传播机制：**
- `querySingle` 全程感知 context 取消
- I/O 操作在独立 goroutine 中执行，主 goroutine select 监听 `ctx.Done()`
- context 取消时立即关闭 UDP 连接，阻塞的 Read/Write 会返回错误
- 确保高延迟网络下 goroutine 不会堆积，避免资源泄漏
- 收到第一个有效响应后立即调用 cancel，终止所有未完成的查询

### 4.4 迭代解析流程 (resolveIterative)

```
resolveIterative(domain, qtype)
   │
   ├─ len(UpstreamServers) == 0 → ErrNoUpstreamServers
   │
   ├─ 检查缓存 → 命中且未过期 → 返回缓存
   │
   ├─ 并行查询上游服务器
   │
   ├─ 处理 CNAME 别名链
   │     └─ depth < MaxRecursionDepth
   │           └─ 存在 CNAME → 继续查询 CNAME 目标
   │           └─ depth++
   │
   ├─ depth >= MaxRecursionDepth 且仍有 CNAME → ErrMaxDepthExceeded
   │
   ├─ 过滤目标类型记录 → 无 → ErrNoRecordsFound
   │
   ├─ 存入缓存
   │
   └─ 返回结果
```

### 4.5 缓存管理流程

**缓存写入 (putToCache)：
```
putToCache(domain, qtype, records)
   │
   ├─ EnableCache == false → 跳过
   │
   ├─ 计算缓存 key = "domain:qtype
   │
   ├─ 计算 TTL = 记录中最小的 TTL（秒）
   │
   ├─ ExpiresAt = now + TTL
   │
   └─ 加写锁
   │
   ├─ cache[key] = &CacheEntry{...}
   │
   └─ 释放锁
```

**缓存读取 (getFromCache)：
```
getFromCache(domain, qtype)
   │
   ├─ EnableCache == false → 返回 nil, false
   │
   ├─ 计算缓存 key
   │
   ├─ 加读锁
   │
   ├─ 查找缓存条目
   │     ├─ 不存在 → 返回 nil, false
   │     └─ 已过期 → 删除条目，返回 nil, false
   │
   └─ 返回记录, true
```

**缓存自动清理 (cleanupExpired)：
```
cleanupLoop（后台协程）
   │
   └─ [ticker.C，CleanupInterval 驱动]
      │
      ├─ 加写锁
      │
      ├─ 遍历所有缓存条目
      │     └─ 已过期 → 删除
      │
      └─ 释放锁
```

**延迟过期策略：
- 读取时检查过期（延迟删除）
- 后台协程定期清理
- 读写锁保证并发安全

## 5. DNS 协议编解码

### 5.1 DNS 查询报文构建 (buildQuery)

```
buildQuery(domain, qtype) → (queryBytes, transactionID, error)
   │
   ├─ 验证域名合法性
   │
   ├─ 生成随机 16 位 TransactionID
   │
   ├─ 构建 DNS 头部（12 字节）
   │     ├─ ID: TransactionID
   │     ├─ Flags: RD=1（期望递归）
   │     ├─ QDCOUNT: 1
   │     └─ 其他：0
   │
   ├─ 编码问题部分
   │     ├─ 域名编码：标签长度 + 标签内容 + 0 结尾
   │     ├─ QTYPE: qtype
   │     └─ QCLASS: IN (1)
   │
   └─ 返回完整查询报文和事务 ID
```

**事务 ID 说明：**
- 使用 `crypto/rand` 生成随机 16 位 ID
- 每个查询生成独立的随机 ID
- 响应解析时需校验 ID 匹配
- 防止 UDP 环境下接受无关响应

### 5.2 DNS 响应解析 (parseResponse)

```
parseResponse(msg, expectedID)
   │
   ├─ 检查报文长度 < 12 → ErrInvalidResponse
   │
   ├─ 解析头部（12 字节）
   │     ├─ TransactionID
   │     ├─ Flags
   │     ├─ QDCOUNT, ANCOUNT, NSCOUNT, ARCOUNT
   │     └─ 从 Flags 提取 RCODE（低 4 位）
   │
   ├─ 校验 TransactionID
   │     └─ 与 expectedID 不匹配 → ErrTransactionIDMismatch
   │
   ├─ 跳过问题部分（QDCOUNT 个问题）
   │
   ├─ 解析回答区（ANCOUNT 个 RR）
   │
   ├─ 解析授权区（NSCOUNT 个 RR）
   │
   ├─ 解析附加区（ARCOUNT 个 RR）
   │
   ├─ 校验 RCODE
   │     ├─ RCODE != NOERROR → 返回对应 DNS 错误
   │     ├─ RCODE=FORMERR → ErrFORMERR
   │     ├─ RCODE=SERVFAIL → ErrSERVFAIL
   │     ├─ RCODE=NXDOMAIN → ErrNXDOMAIN
   │     ├─ RCODE=REFUSED → ErrREFUSED
   │     └─ 其他 → &DNSError{RCODE: rcode, ...}
   │
   └─ 返回 DNSResponse
```

**RCODE 响应码说明：**
| RCODE | 名称 | 含义 |
|-------|------|------|
| 0 | NOERROR | 无错误 |
| 1 | FORMERR | 格式错误 - 查询报文格式不正确 |
| 2 | SERVFAIL | 服务器失败 - 服务器处理时发生内部错误 |
| 3 | NXDOMAIN | 域名不存在 - 查询的域名不存在 |
| 5 | REFUSED | 拒绝 - 服务器拒绝处理请求 |

**事务 ID 校验说明：**
- 每个 DNS 查询生成随机 16 位事务 ID
- 响应解析时校验 ID 必须与查询匹配
- 防止 UDP 环境下接受不属于本次查询的响应包
- 增强 DNS 解析的安全性和正确性

### 5.3 域名解码 (decodeName)

域名解码支持 DNS 消息压缩：
- 0xC0 开头表示指针（最高两位为 11）
- 指针值为 14 位偏移量
- 最多支持 10 次跳转防止无限循环
- 正确处理压缩指针的偏移计算

## 6. 线程安全与性能优化

DNS 解析器是完全并发安全的：
- 缓存读写使用 `sync.RWMutex` 读写锁
- 读多写少场景下允许多个读操作并发
- 并行查询使用 goroutine 并发查询
- 后台协程通过 `stopCh` + `WaitGroup` 优雅退出

**关键性能优化：**
- **读写锁分离**：缓存读取使用读锁，允许多 goroutine 并发读
- **并行查询**：同时向多个上游服务器查询，取最快响应
- **缓存 TTL**：使用记录自身的 TTL，而非固定缓存时间
- **延迟过期**：读取时检查过期，避免不必要的锁竞争
- **原子计数器**：并行查询使用原子计数器跟踪未完成请求

## 7. 使用示例

### 7.1 基础使用：迭代解析（上游服务器）

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/dnsresolver"
)

func main() {
    // 使用默认配置创建解析器
    resolver := dnsresolver.NewResolver()
    defer resolver.Close()

    // 解析 A 记录（IPv4）
    ips, err := resolver.ResolveA("www.example.com")
    if err != nil {
        panic(err)
    }
    fmt.Printf("IPv4 地址: %v\n", ips)

    // 解析 AAAA 记录（IPv6）
    ips6, err := resolver.ResolveAAAA("www.google.com")
    if err != nil {
        panic(err)
    }
    fmt.Printf("IPv6 地址: %v\n", ips6)
}
```

### 7.2 递归解析模式

```go
cfg := dnsresolver.Config{
    EnableRecursion: true,           // 启用递归解析
    EnableCache:     true,             // 启用缓存
    MaxRecursionDepth: 10,              // 最大递归深度
    QueryTimeout:     5 * time.Second,  // 查询超时
}

resolver, err := dnsresolver.NewResolverWithConfig(cfg)
if err != nil {
    panic(err)
}
defer resolver.Close()

ips, err := resolver.ResolveA("www.example.com")
if err != nil {
    panic(err)
}
fmt.Printf("递归解析结果: %v\n", ips)
```

### 7.3 自定义上游服务器

```go
cfg := dnsresolver.Config{
    EnableRecursion: false,           // 迭代模式
    UpstreamServers: []string{
        "114.114.114.114:53",  // 国内 DNS
        "8.8.8.8:53",              // Google DNS
        "1.1.1.1:53",              // Cloudflare DNS
    },
    QueryTimeout: 3 * time.Second,
}

resolver, _ := dnsresolver.NewResolverWithConfig(cfg)
defer resolver.Close()

// 并行查询三个上游服务器，取最快响应
ips, err := resolver.ResolveA("www.example.com")
```

### 7.4 缓存管理

```go
// 手动清理缓存
resolver.ClearCache()

// 获取缓存条目数
count := resolver.CacheCount()
fmt.Printf("缓存条目数: %d\n", count)

// 禁用缓存
cfg := dnsresolver.Config{
    EnableCache: false,
}
```

### 7.5 错误处理

```go
ips, err := resolver.ResolveA("nonexistent.example.com")
if err != nil {
    if errors.Is(err, dnsresolver.ErrNoRecordsFound) {
        fmt.Println("域名不存在或无 A 记录")
    } else if errors.Is(err, dnsresolver.ErrNXDOMAIN) {
        fmt.Println("DNS 服务器返回 NXDOMAIN - 域名不存在")
    } else if errors.Is(err, dnsresolver.ErrSERVFAIL) {
        fmt.Println("DNS 服务器返回 SERVFAIL - 服务器错误")
    } else if errors.Is(err, dnsresolver.ErrMaxDepthExceeded) {
        fmt.Println("递归深度超限")
    } else if errors.Is(err, dnsresolver.ErrAllUpstreamsFailed) {
        fmt.Println("所有上游服务器均失败")
    } else if errors.Is(err, dnsresolver.ErrTransactionIDMismatch) {
        fmt.Println("事务 ID 不匹配 - 可能存在响应伪造")
    } else {
        fmt.Printf("解析失败: %v\n", err)
    }

    // 提取详细 RCODE 信息
    var dnsErr *dnsresolver.DNSError
    if errors.As(err, &dnsErr) {
        fmt.Printf("DNS 错误 RCODE: %d, 消息: %s\n", dnsErr.RCODE, dnsErr.Msg)
    }
}
```

## 8. 单元测试覆盖

### 8.1 测试分类

| 测试类别 | 覆盖范围 |
|----------|----------|
| 基础功能 | 创建、配置验证、关闭 |
| 缓存测试 | 缓存命中、过期、清理、TTL 管理 |
| 迭代解析 | 正常解析、CNAME 追踪、最大深度、无上游 |
| 递归解析 | 根到叶子、NS 委派、胶水记录、CNAME 递归 |
| 并行查询 | 最快响应、全部失败、无服务器、部分失败、迟来响应丢弃 |
| 协议编解码 | 查询构建、响应解析、域名解码 |
| 并发安全 | 并发访问、并发读写 |
| 错误处理 | 各种错误场景 |
| 边界条件 | 空域名、超长域名、无效响应 |
| RCODE 处理 | NXDOMAIN、SERVFAIL、FORMERR、REFUSED 响应码解析与传递 |
| 事务 ID | 事务 ID 匹配验证、不匹配错误处理 |
| Context 传播 | 查询取消、超时传播、goroutine 泄漏防护 |
| CNAME 优化 | 同区 CNAME 优化效率、跨区 CNAME 完整递归、isSameZone 判定、深层子域名同区、链式 CNAME 同区 |

### 8.2 关键测试说明

**Mock DNS 服务器：**
- 实现了 `mockDNSServer` 模拟 DNS 服务器
- 支持自定义响应延迟用于测试并行查询的最快响应策略
- 模拟各种响应内容
- 自动回显事务 ID，确保 ID 匹配

**Mock 连接：**
- `mockConn` 实现 `net.Conn` 接口
- 用于模拟网络错误场景（拨号失败、写入失败、读取失败）
- 支持捕获查询 ID 并在响应中回显

**RCODE 错误测试：**
- 验证各 RCODE (NXDOMAIN/SERVFAIL/FORMERR/REFUSED) 正确解析
- 验证 `errors.Is()` 可正确匹配预定义错误
- 验证 `DNSError` 类型可通过 `errors.As()` 提取
- 验证递归/迭代模式下 RCODE 错误正确传递

**Context 取消测试：**
- 验证 `querySingle` 正确响应 context 取消
- 验证 `queryParallel` 收到第一个响应后取消其他 goroutine
- 验证 context 取消时连接被关闭，goroutine 及时退出
- 验证超时 deadline 正确传播

**CNAME 优化测试：**
- 验证同区 CNAME 追踪复用当前权威服务器（查询数更少）
- 验证跨区 CNAME 追踪回退到完整递归解析
- 验证 CNAME 链上的多级追踪正确性
- 验证 `isSameZone` 函数的边界判定（同域名、子域名、不同 TLD、根区域等）
- 验证深层子域名同区优化（如 `api.v2.example.com` → `cdn.example.com`）
- 验证链式同区 CNAME（A → B → C 均在同一 zone 内）不触发完整递归

## 9. 文件结构

```
internal/dnsresolver/
├── dnsresolver.go      # DNS 解析器核心实现
└── dnsresolver_test.go # 单元测试（70 个测试用例）

docs/
└── dnsresolver.md      # 本文档
```

### 9.1 核心文件说明

**dnsresolver.go：**
- 包含完整的 DNS 解析器实现
- 核心类型：`Resolver`、`Config`、`DNSResponse`、`DNSRecord`、`DNSError`
- 核心功能：递归解析、迭代解析、并行查询、缓存管理
- 协议编解码：查询构建、响应解析、域名编解码
- 错误类型：预定义错误变量与 `DNSError` 结构体

**dnsresolver_test.go：**
- 70 个单元测试用例，覆盖全部功能
- Mock 组件：`mockConn`、`mockConnWithDeadline`、`mockDNSServer`
- 测试分类：基础功能、缓存、迭代解析、递归解析、并行查询、
  协议编解码、并发安全、错误处理、边界条件、RCODE 处理、
  事务 ID、Context 传播、CNAME 优化
