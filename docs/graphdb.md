# GraphDB 图数据存储引擎模块

## 1. 模块概述

GraphDB 是一个内存型图数据存储引擎，专为需要高效存储和查询带权有向图的场景设计。模块提供完整的节点和边的增删改查、邻接表索引、BFS/DFS 图遍历以及基于 Dijkstra 算法的最短路径查询等功能，并通过读写锁保证线程安全。

**包路径**: `internal/graphdb`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 节点管理 | 支持节点的添加、删除、查询，节点拥有唯一 ID 和可选属性 |
| 边管理 | 支持有向边的添加、删除、查询，边可携带权重、标签和自定义属性 |
| 邻接表索引 | 按节点维护出边和入边索引，支持按权重排序加速遍历 |
| BFS 遍历 | 广度优先搜索，支持最大遍历深度限制 |
| DFS 遍历 | 深度优先搜索，支持最大遍历深度限制 |
| 最短路径 | 基于 Dijkstra 算法的带权最短路径查询，返回路径节点序列和总权重 |

## 3. 核心结构体与职责

### 3.1 Graph

图存储主结构体，对外提供所有操作接口。

```go
type Graph struct {
    nodes     map[string]*Node
    outEdges  map[string][]*Edge
    inEdges   map[string][]*Edge
    outSorted map[string]bool
    inSorted  map[string]bool
    mu        sync.RWMutex
}
```

**职责**:
- 管理图中所有节点和边的存储
- 维护邻接表索引（出边表 outEdges、入边表 inEdges）
- 通过 `outSorted` / `inSorted` 标记邻接表是否已按权重排序，实现懒排序
- 通过 `sync.RWMutex` 保护并发访问，读共享、写互斥

### 3.2 Node

图节点表示。

```go
type Node struct {
    ID         string
    Properties map[string]interface{}
}
```

**职责**:
- `ID`: 节点唯一标识
- `Properties`: 节点附加属性，键值对形式，可选

### 3.3 Edge

图中有向边表示。

```go
type Edge struct {
    From       string
    To         string
    Weight     float64
    Label      string
    Properties map[string]interface{}
}
```

**职责**:
- `From`: 起始节点 ID
- `To`: 目标节点 ID
- `Weight`: 边权重，非负浮点数，用于最短路径计算
- `Label`: 边标签，可选字符串
- `Properties`: 边附加属性，键值对形式，可选

### 3.4 PathResult

最短路径查询结果。

```go
type PathResult struct {
    Nodes  []string
    Weight float64
}
```

**职责**:
- `Nodes`: 路径上按顺序排列的节点 ID 序列
- `Weight`: 路径总权重

## 4. 邻接表索引机制

### 4.1 数据结构

Graph 使用双邻接表存储拓扑关系：

```
outEdges[nodeID] -> [Edge1, Edge2, ...]  // 所有从 nodeID 出发的边
inEdges[nodeID]  -> [Edge1, Edge2, ...]  // 所有指向 nodeID 的边
```

### 4.2 懒排序（Lazy Sorting）

为了平衡写入和查询性能，邻接表采用懒排序策略：

1. **写入阶段**: 每次 `AddEdge` 时仅将边追加到对应列表末尾，标记 `outSorted[from] = false` / `inSorted[to] = false`
2. **查询阶段**: `GetOutEdges` / `GetInEdges` 时检查排序标记，若未排序则先排序再返回

**优势**:
- 批量写入时无需频繁排序，写入性能接近 O(1)
- 查询时按需排序，排序结果可复用直到下一次写入
- 排序按权重升序排列，便于遍历时优先选择权值小的边

## 5. BFS / DFS 遍历机制

### 5.1 BFS（广度优先搜索）

使用队列实现，逐层扩展：

```
BFS(start, maxDepth):
  queue = [(start, depth=0)]
  visited = {start}
  result = [start]
  while queue not empty:
    (node, depth) = dequeue()
    if depth >= maxDepth: skip
    for each out-edge of node:
      if neighbor not visited:
        mark visited
        add to result
        enqueue (neighbor, depth+1)
  return result
```

### 5.2 DFS（深度优先搜索）

使用递归实现，尽可能深入：

```
DFS(start, maxDepth):
  visited = {}
  result = []
  dfs(node, depth):
    mark visited
    add to result
    if depth >= maxDepth: return
    for each out-edge of node:
      if neighbor not visited:
        dfs(neighbor, depth+1)
  dfs(start, 0)
  return result
```

### 5.3 深度限制

`maxDepth` 参数控制最大遍历深度：
- `maxDepth = 1`: 仅访问起始节点
- `maxDepth = 2`: 访问起始节点及其直接邻居
- 以此类推

## 6. Dijkstra 最短路径算法

### 6.1 算法流程

使用优先队列（最小堆）优化的 Dijkstra 算法：

```
ShortestPath(from, to):
  dist[all] = INF
  dist[from] = 0
  prev[all] = undefined
  pq = [(from, 0)]  // 最小堆，按 dist 排序

  while pq not empty:
    u = extract-min(pq)
    if u == to: break
    if u visited: continue
    mark u visited

    for each edge (u -> v, weight w):
      if v not visited:
        alt = dist[u] + w
        if alt < dist[v]:
          dist[v] = alt
          prev[v] = u
          push (v, alt) into pq

  if dist[to] == INF: return ErrNoPath
  reconstruct path by backtracking prev[] from to to from
  return (path, dist[to])
```

### 6.2 时间复杂度

- 使用二叉堆：O((V + E) log V)
- 其中 V 为节点数，E 为边数

### 6.3 约束条件

- 边权重必须非负（模块在 AddEdge 时已校验）
- 若 from == to，直接返回单节点路径，权重为 0

## 7. 并发安全设计

所有公共方法均通过 `sync.RWMutex` 保护：

| 操作类型 | 锁类型 | 说明 |
|---------|--------|------|
| AddNode / RemoveNode | 写锁 | 修改节点集合 |
| AddEdge / RemoveEdge | 写锁 | 修改邻接表 |
| GetNode / HasNode | 读锁 | 只读访问 |
| GetOutEdges / GetInEdges | 写锁 | 可能触发排序，需要写权限 |
| BFS / DFS | 读锁 | 只读遍历 |
| ShortestPath | 读锁 | 只读计算 |

**注意**: GetOutEdges / GetInEdges 使用写锁是因为内部可能触发懒排序，需要修改 `outSorted` / `inSorted` 标记以及邻接表顺序。

## 8. 图从构建到遍历查询的完整流程

```
1. 创建图实例
   g := NewGraph()

2. 添加节点
   g.AddNode("A", map[string]interface{}{"name": "Alice"})
   g.AddNode("B", nil)
   g.AddNode("C", nil)
   g.AddNode("D", nil)

3. 添加边（建立拓扑关系）
   g.AddEdge("A", "B", 2.0, "friend", nil)
   g.AddEdge("A", "C", 5.0, "colleague", nil)
   g.AddEdge("B", "D", 1.0, "", nil)
   g.AddEdge("C", "D", 3.0, "", nil)

4. 邻接表查询
   outEdges, _ := g.GetOutEdges("A")  // 按权重升序: A->B(2), A->C(5)
   inEdges, _ := g.GetInEdges("D")    // 按权重升序: B->D(1), C->D(3)

5. 图遍历
   bfsResult, _ := g.BFS("A", 3)      // BFS, 最大深度 3
   dfsResult, _ := g.DFS("A", 3)      // DFS, 最大深度 3

6. 最短路径查询
   path, _ := g.ShortestPath("A", "D")
   // path.Nodes  = ["A", "B", "D"]
   // path.Weight = 3.0 (2.0 + 1.0)
```

## 9. 使用示例

### 9.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/graphdb"
)

func main() {
    g := graphdb.NewGraph()

    g.AddNode("S", map[string]interface{}{"city": "Shanghai"})
    g.AddNode("B", map[string]interface{}{"city": "Beijing"})
    g.AddNode("G", map[string]interface{}{"city": "Guangzhou"})
    g.AddNode("C", map[string]interface{}{"city": "Chengdu"})

    g.AddEdge("S", "B", 1318, "高铁", nil)
    g.AddEdge("S", "G", 1790, "高铁", nil)
    g.AddEdge("B", "C", 1874, "高铁", nil)
    g.AddEdge("G", "C", 2400, "高铁", nil)
    g.AddEdge("S", "C", 2500, "直达", nil)

    path, err := g.ShortestPath("S", "C")
    if err != nil {
        fmt.Println("无路径:", err)
        return
    }
    fmt.Printf("最短路径: %v, 总里程: %.0f km\n", path.Nodes, path.Weight)
}
```

### 9.2 带深度限制的遍历

```go
g := graphdb.NewGraph()
// ... 添加节点和边构建社交网络

// 只查询 2 度好友
friends, err := g.BFS("me", 2)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("我和我的好友共 %d 人\n", len(friends))
```

### 9.3 防御性拷贝

返回的 Node 和 Edge 中的 Properties 均为深拷贝，修改返回值不会影响图内部数据：

```go
node, _ := g.GetNode("A")
node.Properties["name"] = "Bob"  // 仅修改本地副本

node2, _ := g.GetNode("A")
fmt.Println(node2.Properties["name"])  // 仍为原始值
```

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrNodeNotFound` | 节点不存在 | 查询、删除不存在的节点；空 ID 添加节点 |
| `ErrNodeExists` | 节点已存在 | 重复添加相同 ID 的节点 |
| `ErrEdgeNotFound` | 边不存在 | 删除、查询不存在的边 |
| `ErrEdgeExists` | 边已存在 | 重复添加同一起点和终点的边 |
| `ErrSelfLoop` | 禁止自环 | 添加 from == to 的边 |
| `ErrNegativeWeight` | 权重不能为负 | 添加 weight < 0 的边 |
| `ErrInvalidStartNode` | 起始节点无效 | 边操作中 from 节点不存在 |
| `ErrInvalidEndNode` | 终止节点无效 | 边操作中 to 节点不存在 |
| `ErrMaxDepthNonPositive` | 深度必须为正 | BFS/DFS 中 maxDepth <= 0 |
| `ErrNoPath` | 无可达路径 | ShortestPath 中两节点不连通 |

## 11. 性能特征

### 11.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| AddNode | O(1) | 哈希表插入 |
| RemoveNode | O(degree) | 需清理所有关联边 |
| AddEdge | O(1) | 追加到邻接表 |
| RemoveEdge | O(degree) | 线性扫描邻接表 |
| GetOutEdges / GetInEdges | O(d log d) | 首次查询需排序，d 为节点度数 |
| BFS / DFS | O(V + E) | 标准图遍历 |
| ShortestPath | O((V + E) log V) | 堆优化 Dijkstra |

### 11.2 空间复杂度

- 节点存储: O(V)
- 邻接表存储: O(E)
- 总计: O(V + E)

## 12. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **不支持负权边**: Dijkstra 算法要求边权重非负，模块在 AddEdge 时已校验
3. **不支持多边**: 同一对 (from, to) 之间只能存在一条边，重复添加返回 ErrEdgeExists
4. **不支持自环**: from == to 的边被禁止
5. **节点 ID 约束**: 空字符串 ID 不被允许
6. **懒排序写锁**: GetOutEdges / GetInEdges 使用写锁，高并发读场景下可能成为瓶颈
7. **DFS 递归深度**: DFS 使用递归实现，极深的图可能触发栈溢出，此时建议使用 BFS 或增大 maxDepth 限制
