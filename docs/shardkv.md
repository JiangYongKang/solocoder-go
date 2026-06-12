# ShardKV 内存分片存储域模块

## 1. 模块概述

ShardKV 是一个基于内存的分布式键值存储模块，实现了分片存储、一致性哈希、数据副本、故障转移等核心功能。该模块设计用于在单机环境模拟分布式存储系统的关键特性，为需要高性能、高可用内存存储的场景提供解决方案。

### 1.1 核心特性

- **一致性哈希环**：使用 SHA-1 哈希算法将键均匀映射到分片节点，支持动态增删节点
- **虚拟节点**：每个物理分片映射到多个虚拟节点，显著降低数据分布不均匀性
- **自动数据迁移**：节点加入或离开时自动迁移最小范围的键，迁移过程不阻塞读写
- **副本同步**：支持配置副本数量，写入操作达到配置的仲裁数后返回成功
- **故障转移**：分片不可用时自动路由到副本节点，恢复后自动同步数据

## 2. 文件结构

```
internal/shardkv/
├── hash_ring.go       # 一致性哈希环实现
├── shard.go           # 单个分片存储实现
├── cluster.go         # 集群管理器（核心调度逻辑）
└── shardkv_test.go    # 单元测试（覆盖所有功能）
```

## 3. 核心结构体与职责

### 3.1 HashRing（一致性哈希环）

**文件**：`hash_ring.go`

**职责**：
- 维护哈希环上的虚拟节点映射
- 提供节点到哈希值的双向查找
- 计算键对应的主节点和副本节点列表

**核心字段**：
| 字段 | 类型 | 说明 |
|------|------|------|
| `ring` | `[]uint64` | 有序的哈希值数组，构成哈希环 |
| `hashMap` | `map[uint64]string` | 哈希值到物理节点ID的映射 |
| `nodeSet` | `map[string]struct{}` | 已注册物理节点集合 |
| `virtualNodes` | `int` | 每个物理节点对应的虚拟节点数 |

**核心方法**：
- `AddNode(nodeID)`：添加物理节点及其虚拟节点
- `RemoveNode(nodeID)`：移除物理节点及其虚拟节点
- `GetNode(key)`：获取键对应的主节点
- `GetReplicaNodes(key, n)`：获取键对应的 n 个不重复副本节点

### 3.2 Shard（分片存储）

**文件**：`shard.go`

**职责**：
- 存储单个分片的键值对数据
- 维护分片状态（Up/Down/Migrating）
- 提供原子读写操作接口

**核心字段**：
| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | `string` | 分片唯一标识 |
| `status` | `ShardStatus` | 分片运行状态 |
| `data` | `map[string][]byte` | 键值存储，值为字节切片 |
| `mu` | `sync.RWMutex` | 读写锁，保证并发安全 |

**状态枚举**：
```go
ShardStatusUp        // 正常运行
ShardStatusDown      // 故障离线
ShardStatusMigrating // 数据迁移中
```

**核心方法**：
- `Get/Put/Delete(key)`：常规读写操作，受状态检查保护
- `ForcePut/ForceDelete(key)`：强制操作，用于数据迁移和故障恢复
- `GetAllKeys()/GetAllData()`：批量导出，用于迁移扫描

### 3.3 ShardKVCluster（集群管理器）

**文件**：`cluster.go`

**职责**：
- 协调管理所有分片节点
- 调度数据迁移和副本同步
- 实现故障检测和自动转移
- 对外提供统一的 KV 操作接口

**核心字段**：
| 字段 | 类型 | 说明 |
|------|------|------|
| `config` | `ShardKVConfig` | 集群配置参数 |
| `hashRing` | `*HashRing` | 一致性哈希环实例 |
| `shards` | `map[string]*Shard` | 所有分片实例 |
| `downShards` | `map[string]struct{}` | 标记为故障的分片集合 |
| `migrating` | `bool` | 是否有迁移任务正在执行 |
| `migratingMu` | `sync.Mutex` | 迁移状态保护锁 |

**配置结构体**：
```go
type ShardKVConfig struct {
    VirtualNodes int  // 虚拟节点数（默认100）
    ReplicaCount int  // 副本数量（默认2）
    WriteQuorum  int  // 写入仲裁数（默认2）
}
```

## 4. 工作流程详解

### 4.1 一致性哈希工作流程

```
         key = "user:123"
              │
              ▼
    ┌─────────────────────┐
    │  SHA-1(key) → uint64 │  hash = 0x7a3f...
    └─────────────────────┘
              │
              ▼
    ┌───────────────────────────────┐
    │ 二分查找哈希环上第一个 >= hash 的位置 │
    └───────────────────────────────┘
              │
     ┌────────┴────────┐
     ▼                 ▼
 找到节点         环绕到环首
     │                 │
     └────────┬────────┘
              ▼
      返回主节点ID (shard-3)
```

**虚拟节点分布策略**：
- 虚拟键格式：`{nodeID}#vn{index}` 例如 `shard-1#vn0`, `shard-1#vn1` ...
- 每个虚拟键独立哈希后插入哈希环
- 虚拟节点越多，数据分布越均匀（测试数据：1VN→CV=1.17，100VN→CV=0.00045）

### 4.2 副本同步写入流程

```
Put(key="order:42", value)
           │
           ▼
  GetReplicaNodes(key, ReplicaCount=3)
           │
     ┌─────┼─────┐
     ▼     ▼     ▼
  shard-A shard-B shard-C   (基础副本节点)
     │     │     │
     └─────┼─────┘
           │
  ┌────────▼────────┐
  │ 过滤可用节点（排除Down） │
  │ 不足则扩展查找额外节点   │
  └────────┬────────┘
           │
     ┌─────┴──────┐
     ▼            ▼
  并发写入     并发写入
 (goroutine)  (goroutine)
     │            │
     └─────┬──────┘
           ▼
  ┌─────────────────┐
  │ 成功数 >= WriteQuorum ? │
  └────────┬────────┘
     │YES         │NO
     ▼            ▼
  返回nil    返回错误
```

### 4.3 读取与故障转移流程

```
Get(key="order:42")
           │
           ▼
  GetReplicaNodes(key, ReplicaCount+5)  ← 扩展范围增加容错
           │
     ┌─────┼─────┐
     ▼     ▼     ▼
  shard-A shard-B shard-C ...
     │     │     │
   Down?  Down?  Down?
     │YES  │YES  │NO
     ▼     ▼     ▼
   跳过   跳过   尝试读取
                 │
              成功？
             │YES │NO
             ▼    ▼
         返回值  继续下一个
```

**故障转移特性**：
- 读取时自动跳过故障节点，遍历副本列表直到找到可用数据
- 写入时动态补充可用节点到仲裁数要求
- 主节点恢复后自动从副本同步缺失数据

### 4.4 节点加入与数据迁移流程

```
AddShard(newShard="shard-D")
           │
           ▼
  ┌──────────────────────────┐
  │ 1. 创建分片实例，注册到哈希环 │
  └──────────────┬───────────┘
                 ▼
  ┌──────────────────────────┐
  │ 2. 遍历所有现有分片的全部键   │
  └──────────────┬───────────┘
                 ▼
  对每个 key 计算 GetReplicaNodes(key)
                 │
    newShard 是副本节点之一吗？
        │YES           │NO
        ▼              ▼
  从旧分片复制数据   跳过（不受影响）
  到新分片
        │
  旧分片仍是副本吗？
     │NO       │YES
     ▼         ▼
  从旧分片删除  保留副本
```

**迁移保证**：
- 迁移过程使用 `ForcePut/ForceDelete` 不受状态检查限制
- 迁移期间读写请求正常处理，最终一致性由副本机制保障
- 仅迁移归属关系变化的键，最小化数据移动量

### 4.5 节点移除流程

```
RemoveShard(removed="shard-B")
           │
           ▼
  将分片标记为 Migrating 状态
           │
           ▼
  ┌──────────────────────────────┐
  │ Stage A: 原子切换哈希环（关键）│
  │   c.hashRing.RemoveNode("B") │  ← 先从环上移除
  │   → 所有后续 GetReplicaNodes │
  │     均使用「新环」计算        │
  └──────────────┬───────────────┘
                 ▼
  ┌─────────────────────────────────────┐
  │ Stage B: 基于新环执行迁移             │
  │  遍历被移除分片的全部键                │
  │     │                                 │
  │     ▼                                 │
  │  newRingReplicas =                    │
  │   新环.GetReplicaNodes(key, N)        │ ← 新环目标集合
  │     │                                 │
  │     ▼                                 │
  │  existingReplicas =                   │
  │   count(存活 ∩ newRingReplicas        │ ← 仅统计合法副本
  │         ∩ 已持有该键)                 │
  │     │                                 │
  │     ▼                                 │
  │  needed = N - existingReplicas        │
  └──────────────┬────────────────────────┘
                 │
     ┌───────────┴───────────┐
     ▼                       ▼
  needed <= 0            needed > 0
  副本数足够              需补充副本
     │                       │
     ▼                       ▼
  跳过复制           优先 newRingReplicas 内节点
                     → 逐个 ForcePut
                     → needed == 0 即停止
                     → 仍不够？用非合法节点兜底
     │                       │
     └───────────┬───────────┘
                 ▼
  ┌──────────────────────────────┐
  │ Stage C: 清理分片映射          │
  │  delete(c.shards, "B")        │
  │  delete(c.downShards, "B")    │
  └──────────────────────────────┘
```

**节点移除迁移策略**：
- **哈希环一致性（先切环后迁移）**：必须先 `RemoveNode` 从哈希环移除节点，再基于新环计算目标。若使用旧环，排序差异会导致旧环选定的目标在新环中不再是合法 replica，最终数据分布偏离预期。若迁移失败，需回滚调用 `AddNode` 将节点重新加入哈希环。
- **合法副本集合约束**：`newRingReplicas = 新环.GetReplicaNodes(key, ReplicaCount)` 是该键的唯一合法目标集合。迁移目标首选该集合内的存活节点，集合外的节点仅作仲裁不足时的兜底使用。
- **精确副本计数范围**：`existingReplicas` **只统计**「存活 ∩ newRingReplicas ∩ 已持有该键」的交集。非合法 replica 上的陈旧副本不计入统计，防止 needed 被低估导致合法节点缺少应有数据。
- **按需补充**：仅当已有副本数少于 `ReplicaCount` 时才复制到新目标
- **目标过滤**：跳过已持有该键的分片，避免重复写入
- **及时终止**：补充到足够副本数后立即停止，防止过度复制
- **候选顺序**：沿新环副本列表优先选择，保证迁移后分布与新哈希环预期完全一致
- **回滚保证**：`migrateOnRemove` 失败时，先调用 `AddNode` 把被移除节点重新挂回哈希环，再恢复其 `Up` 状态

### 4.6 故障恢复时序保证

```
MarkShardUp(recovered="shard-A")
           │
           ▼
  ┌─────────────────────────┐
  │ Stage 1: 数据同步（离线） │
  │  - 分片仍处于 Down 状态  │
  │  - 从所有其他副本拉取数据 │
  │  - 逐个键校验归属关系    │
  │  - ForcePut 写入本地     │
  └─────────────┬───────────┘
                │
           同步完成
                │
           ▼
  ┌─────────────────────────┐
  │ Stage 2: 状态切换（原子） │
  │  - 分片状态 → Up        │
  │  - 从 downShards 移除    │
  └─────────────┬───────────┘
                │
           ▼
  MarkShardUp 返回
  → 此时所有数据已就绪
  → Get 请求立即可用
```

**故障恢复时序保证**：
- **先同步后上线**：`MarkShardUp` 是同步阻塞调用，数据全部同步完成后才将分片置为可用
- **零时间窗口**：不存在"状态已上线但数据未同步"的间隙，避免 Get 返回不存在的假象
- **读一致性**：`MarkShardUp` 返回后，所有应归属于该分片的键都已可从该分片读取
- **来源可靠性**：从所有可用副本分片聚合数据，确保取到完整的数据集
- **归属校验**：只同步那些按一致性哈希规则应由本分片持有副本的键

## 5. 错误码说明

| 错误变量 | 触发场景 |
|---------|---------|
| `ErrKeyNotFound` | 键不存在（Get/Delete） |
| `ErrShardDown` | 目标分片处于 Down 状态 |
| `ErrNoAvailable` | 哈希环为空或无任何可用分片 |
| `ErrQuorumFailed` | 写入成功数未达到仲裁要求 |

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/shardkv"
)

func main() {
    // 创建集群（默认配置：2副本，2仲裁）
    cluster := shardkv.NewShardKVCluster()

    // 添加3个分片节点
    cluster.AddShard("shard-1")
    cluster.AddShard("shard-2")
    cluster.AddShard("shard-3")

    // 等待初始迁移完成
    cluster.WaitForMigration()

    // 写入数据
    err := cluster.Put("user:1001", []byte(`{"name":"Alice","age":30}`))
    if err != nil {
        panic(err)
    }

    // 读取数据
    val, err := cluster.Get("user:1001")
    if err != nil {
        panic(err)
    }
    fmt.Println(string(val))

    // 检查存在性
    if cluster.HasKey("user:1001") {
        fmt.Println("key exists")
    }

    // 删除数据
    cluster.Delete("user:1001")
}
```

### 6.2 自定义配置

```go
// 高性能配置：3副本，2仲裁（允许1个副本写入失败）
config := shardkv.ShardKVConfig{
    VirtualNodes: 200,   // 更多虚拟节点，分布更均匀
    ReplicaCount: 3,     // 3个副本
    WriteQuorum:  2,     // 2个成功即返回
}
cluster := shardkv.NewShardKVClusterWithConfig(config)
```

### 6.3 故障转移演示

```go
cluster := shardkv.NewShardKVCluster()
cluster.AddShard("s1")
cluster.AddShard("s2")
cluster.AddShard("s3")
cluster.WaitForMigration()

// 写入测试数据
for i := 0; i < 100; i++ {
    cluster.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d", i)))
}

// 模拟 s1 故障
cluster.MarkShardDown("s1")

// 数据仍可正常读取（自动路由到副本）
val, err := cluster.Get("k42")  // 成功！

// 写入仍可工作（动态选择其他可用节点达到仲裁）
cluster.Put("k_new", []byte("data"))

// s1 恢复上线
cluster.MarkShardUp("s1")
cluster.WaitForMigration()

// 数据已自动同步回 s1
```

### 6.4 动态扩缩容

```go
cluster := shardkv.NewShardKVCluster()
cluster.AddShard("s1")
cluster.AddShard("s2")

// 写入大量数据
for i := 0; i < 10000; i++ {
    cluster.Put(fmt.Sprintf("item:%d", i), []byte("payload"))
}

// 扩容：添加2个新分片
cluster.AddShard("s3")
cluster.AddShard("s4")
cluster.WaitForMigration()  // 数据自动再平衡

// 缩容：移除1个分片
cluster.RemoveShard("s2")
cluster.WaitForMigration()  // 数据迁移到剩余分片
```

## 7. 并发安全

所有公共方法均为 goroutine-safe：
- 分片内部使用 `sync.RWMutex` 保护数据访问
- 集群管理器使用 `sync.RWMutex` 保护分片映射
- 迁移操作使用独立锁标记状态
- 写入操作使用 `sync.WaitGroup` 等待副本同步完成

## 8. 测试覆盖

单元测试覆盖范围：
- **HashRing (7)**：基础操作、一致性验证、虚拟节点效果、副本节点获取
- **Shard (6)**：CRUD、状态控制、批量导出、强制操作
- **Cluster (30+)**：
  - 正常流程：Put/Get/Delete、HasKey
  - 数据迁移：
    - 节点加入时的数据完整性与副本数正确性
    - 节点移除时的副本数保持与无过度复制
    - 总键数在多次扩缩容后保持一致
    - 迁移过程中删除操作的正确性
    - 其他分片已有数据不被破坏（副本保留）
  - 副本同步：多副本写入验证、仲裁失败场景
  - 故障转移：
    - 节点宕机读写、恢复自动同步
    - 故障恢复时序：MarkShardUp 返回时数据已就绪
    - 多节点宕机与逐节点恢复的数据完整性
  - 边界条件：单分片、全宕机、空环
  - 并发安全：多协程并发读写、动态扩缩容下的一致性
