# 一致性哈希环模块 (chainhash)

## 1. 模块概述

chainhash 是一个功能完整的一致性哈希环实现，用于分布式系统中的数据分片和负载均衡。该模块提供了虚拟节点、权重分配、动态节点管理、数据迁移量估算和序列化持久化等核心功能。

## 2. 核心功能

### 2.1 虚拟节点映射

每个物理节点在哈希环上映射为多个虚拟节点，通过虚拟节点的均匀分布实现数据的负载均衡。虚拟节点数量可配置，并可根据物理节点的处理能力进行调整。

- **虚拟节点生成**: `virtualNodes * weight`
- **虚拟节点标识**: `{nodeID}#vn{index}`
- **哈希算法**: SHA-1 哈希取前 64 位

### 2.2 带权重的节点分布

支持为每个物理节点设置权重值，权重越高的节点在哈希环上映射更多比例的虚拟节点。

- 权重范围: 正整数 (>= 1)
- 虚拟节点数: 基础虚拟节点数 × 权重
- 权重变更: 动态调整，自动重新计算虚拟节点分布

### 2.3 节点动态增减与数据迁移

支持运行时动态添加/移除节点，节点变更时自动计算受影响的数据范围和迁移量。

- **添加节点**: 预先计算需要迁移的数据范围和数量
- **移除节点**: 将该节点的数据迁移到其他节点
- **权重更新**: 调整虚拟节点分布，计算迁移数据量
- **迁移信息**: 包含源节点、目标节点、受影响哈希范围、估计迁移数量

### 2.4 序列化与持久化

支持将哈希环完整状态序列化到磁盘，支持重启后恢复。

- 序列化格式: JSON
- 状态信息: 节点配置、虚拟节点映射、总键数、元数据
- 版本控制: 支持版本校验，确保兼容性

## 3. 核心结构体

### 3.1 HashRing

一致性哈希环的主结构体，管理所有节点和虚拟节点。

```go
type HashRing struct {
    mu           sync.RWMutex
    virtualNodes int
    nodes        map[string]*NodeInfo
    vnodeMap     map[uint64]string
    ring         []uint64
    totalKeys    int64
    metadata     map[string]interface{}
}
```

**主要职责**:
- 管理物理节点的增删改查
- 维护虚拟节点到物理节点的映射
- 提供键到节点的路由功能
- 计算节点变更时的数据迁移量
- 支持序列化和恢复

### 3.2 NodeInfo

物理节点信息结构体。

```go
type NodeInfo struct {
    ID     string                 // 节点唯一标识
    Weight int                    // 节点权重
    Addr   string                 // 节点地址
    Data   map[string]interface{} // 扩展数据
}
```

### 3.3 VirtualNode

虚拟节点信息结构体。

```go
type VirtualNode struct {
    Hash   uint64 // 虚拟节点哈希值
    NodeID string // 所属物理节点ID
    Index  int    // 虚拟节点索引
}
```

### 3.4 HashRange

表示哈希环上的一个连续范围。

```go
type HashRange struct {
    Start uint64 // 范围起始 (包含)
    End   uint64 // 范围结束 (包含)
}
```

### 3.5 MigrationInfo

节点迁移信息结构体。

```go
type MigrationInfo struct {
    AffectedRanges []HashRange // 受影响的哈希范围
    FromNode       string      // 源节点
    ToNode         string      // 目标节点
    EstimatedCount int64       // 估计迁移键数量
    TotalKeys      int64       // 总键数量
    MigrationRatio float64     // 迁移比例
}
```

### 3.6 RingSnapshot

哈希环快照，用于序列化。

```go
type RingSnapshot struct {
    Version      int
    VirtualNodes int
    Nodes        []NodeInfo
    VNodes       []VirtualNode
    TotalKeys    int64
    Metadata     map[string]interface{}
}
```

## 4. 核心 API

### 4.1 创建哈希环

```go
func NewHashRing(virtualNodes int) (*HashRing, error)
```

### 4.2 节点管理

```go
func (hr *HashRing) AddNode(nodeID string, weight int) error
func (hr *HashRing) AddNodeWithInfo(info NodeInfo) error
func (hr *HashRing) RemoveNode(nodeID string) ([]MigrationInfo, error)
func (hr *HashRing) UpdateNodeWeight(nodeID string, newWeight int) ([]MigrationInfo, error)
func (hr *HashRing) NodeExists(nodeID string) bool
func (hr *HashRing) GetNodeInfo(nodeID string) (*NodeInfo, error)
func (hr *HashRing) GetAllNodes() []NodeInfo
func (hr *HashRing) NodeCount() int
func (hr *HashRing) VirtualNodeCount() int
```

### 4.3 键路由

```go
func (hr *HashRing) GetNode(key string) (string, error)
func (hr *HashRing) GetNodes(key string, n int) ([]string, error)
```

### 4.4 迁移计算

```go
func (hr *HashRing) CalculateAddMigration(nodeID string, weight int) ([]MigrationInfo, error)
```

### 4.5 序列化

```go
func (hr *HashRing) Snapshot() *RingSnapshot
func (hr *HashRing) Restore(snapshot *RingSnapshot) error
func (hr *HashRing) SaveToFile(path string) error
func LoadFromFile(path string) (*HashRing, error)
func (hr *HashRing) MarshalJSON() ([]byte, error)
func (hr *HashRing) UnmarshalJSON(data []byte) error
```

### 4.6 元数据

```go
func (hr *HashRing) SetTotalKeys(count int64)
func (hr *HashRing) GetTotalKeys() int64
func (hr *HashRing) SetMetadata(key string, value interface{})
func (hr *HashRing) GetMetadata(key string) (interface{}, bool)
```

## 5. 虚拟节点与物理节点映射关系

### 5.1 映射原理

```
物理节点A (权重=2)
  ├─ 虚拟节点 A#vn0  → 哈希值 H1
  ├─ 虚拟节点 A#vn1  → 哈希值 H2
  └─ ... (共 2×N 个虚拟节点)

物理节点B (权重=3)
  ├─ 虚拟节点 B#vn0  → 哈希值 H3
  ├─ 虚拟节点 B#vn1  → 哈希值 H4
  └─ ... (共 3×N 个虚拟节点)

哈希环: [H1, H2, H3, H4, ...] (顺时针排序)
```

### 5.2 键查找过程

1. 对键进行哈希，得到哈希值 `H`
2. 在哈希环上找到第一个大于等于 `H` 的虚拟节点
3. 返回该虚拟节点对应的物理节点
4. 如果 `H` 大于所有虚拟节点哈希值，返回环上第一个节点

### 5.3 权重分布

- 节点权重为 1: 分配 `virtualNodes` 个虚拟节点
- 节点权重为 W: 分配 `virtualNodes × W` 个虚拟节点
- 数据分布比例 ≈ 节点权重比例

## 6. 节点增减时的数据迁移机制

### 6.1 添加节点

```
添加前: [NodeA, NodeB, NodeC]
添加后: [NodeA, NodeD, NodeB, NodeC]

受影响范围: NodeD 的虚拟节点覆盖的哈希范围
迁移方向: 从 NodeA/NodeB/NodeC 迁移到 NodeD
迁移量: 约 1/4 的总数据量 (N+1 分之 1)
```

### 6.2 移除节点

```
移除前: [NodeA, NodeB, NodeC]
移除后: [NodeA, NodeC]

受影响范围: NodeB 的虚拟节点覆盖的哈希范围
迁移方向: 从 NodeB 迁移到 NodeA/NodeC
迁移量: 约 1/3 的总数据量
```

### 6.3 权重更新

```
更新前: NodeA(权重1), NodeB(权重1)
更新后: NodeA(权重2), NodeB(权重1)

受影响范围: NodeA 新增/减少的虚拟节点范围
迁移方向:
  - 新增虚拟节点: 从 NodeB 迁移到 NodeA
  - 减少虚拟节点: 从 NodeA 迁移到 NodeB
```

### 6.4 迁移量估算

迁移量基于以下公式估算：

```
受影响范围总大小 = Σ (range.End - range.Start + 1)
迁移比例 = 受影响范围总大小 / 2^64
估计迁移量 = 总键数 × 迁移比例
```

## 7. 使用示例

### 7.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/chainhash"
)

func main() {
    // 创建哈希环，每个节点默认 100 个虚拟节点
    hr, err := chainhash.NewHashRing(100)
    if err != nil {
        panic(err)
    }

    // 添加节点，不同权重
    hr.AddNode("node1", 1)  // 100 个虚拟节点
    hr.AddNode("node2", 2)  // 200 个虚拟节点
    hr.AddNode("node3", 3)  // 300 个虚拟节点

    // 设置总键数用于迁移估算
    hr.SetTotalKeys(1000000)

    // 查找键对应的节点
    node, err := hr.GetNode("user:12345")
    if err != nil {
        panic(err)
    }
    fmt.Printf("Key 'user:12345' maps to node: %s\n", node)

    // 获取多个副本节点
    nodes, _ := hr.GetNodes("user:12345", 3)
    fmt.Printf("Replicas: %v\n", nodes)
}
```

### 7.2 动态添加节点

```go
// 预先计算迁移量
migrations, err := hr.CalculateAddMigration("node4", 2)
if err != nil {
    panic(err)
}

fmt.Printf("Adding node4 will cause %d migrations:\n", len(migrations))
for _, m := range migrations {
    fmt.Printf("  From %s to %s: ~%d keys (%.2f%%)\n",
        m.FromNode, m.ToNode, m.EstimatedCount, m.MigrationRatio*100)
}

// 执行添加
