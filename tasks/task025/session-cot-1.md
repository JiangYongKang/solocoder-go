我来帮你实现一个图数据存储引擎功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先查看一下 go.mod 文件和一个现有模块的代码风格，以便保持一致。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\kvstore.md
          
现在我已经了解了项目结构和代码风格。让我创建任务列表并开始实现。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先创建目录结构，然后实现核心代码。


        
            
toolName: run_command
            
status: success
          
command: mkdir -p internal/graphdb undefined
          
现在让我创建核心的 graphdb.go 文件，实现所有功能：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graphdb\graphdb.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先看一下现有测试文件的风格：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore_test.go
          
现在让我创建完整的单元测试文件：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graphdb\graphdb_test.go
          
changes: undefined
          
现在让我更新 todo 状态并运行测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/graphdb/ -v undefined
          
所有测试都通过了！现在让我编写模块需求文档：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\graphdb.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
图数据存储引擎模块已全部完成！以下是实现总结：

## 完成内容

### 1. 核心代码 - [graphdb.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graphdb/graphdb.go)

**数据结构：**
- `Graph`: 图存储主结构体，使用 `sync.RWMutex` 保证线程安全
- `Node`: 节点，包含唯一 ID 和可选属性 `map[string]interface{}`
- `Edge`: 有向边，包含 From/To、权重 Weight、标签 Label 和可选属性
- `PathResult`: 最短路径结果（节点序列 + 总权重）

**核心功能：**

| 功能 | 方法 | 说明 |
|------|------|------|
| 节点增删 | `AddNode` / `RemoveNode` | 支持属性，删除节点自动清理关联边 |
| 边增删 | `AddEdge` / `RemoveEdge` | 校验自环、负权重、节点存在性、重复边 |
| 邻接表索引 | `GetOutEdges` / `GetInEdges` | 懒排序策略，按权重升序返回 |
| BFS 遍历 | `BFS(start, maxDepth)` | 队列实现，支持最大深度限制 |
| DFS 遍历 | `DFS(start, maxDepth)` | 递归实现，支持最大深度限制 |
| 最短路径 | `ShortestPath(from, to)` | 堆优化 Dijkstra 算法 |

### 2. 单元测试 - [graphdb_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graphdb/graphdb_test.go)

共 **48 个测试用例**全部通过，覆盖：
- 正常流程：节点/边增删、遍历、路径查询
- 边界条件：空图、单节点、同节点路径、零权重边
- 异常分支：不存在节点/边、自环、负权重、无效深度、无路径等
- 并发测试：多 goroutine 并发添加节点

测试结果：
```
PASS
ok      solocoder-go/internal/graphdb   0.207s
```

### 3. 模块文档 - [graphdb.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/graphdb.md)

包含模块概述、核心结构体职责、邻接表索引机制、BFS/DFS 算法原理、Dijkstra 最短路径算法、并发安全设计、完整流程图、使用示例、错误定义、性能特征及注意事项。