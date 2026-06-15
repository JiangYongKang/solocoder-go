toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
先读取需要修改的文件的最新内容。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\consistent_hash.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\round_robin.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\server.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\httplb_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\httplb.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\httplb.md
          
现在我对所有文件有了完整了解，开始逐个修复问题。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
**修复 1：一致性哈希 Next 方法** — 目标节点不健康时沿哈希环顺时针继续查找下一个健康节点。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\consistent_hash.go
          
changes: undefined
          
**修复 2：RemoveServer 安全检查** — 添加活跃连接检查和新错误，提供 `IsDrained` 方法。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\server.go
          
changes: undefined
          
现在给 `BackendServer` 添加 `IsDraining` 方法，并修改 `ServerPool.RemoveServer` 增加安全检查。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\server.go
          
changes: undefined
          
现在修改 `ServerPool.RemoveServer` 增加安全检查：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\server.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
**修复 3：移除 RoundRobin 中未使用的 mu sync.Mutex 死代码**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\round_robin.go
          
changes: undefined
          
**修复 4 和 5：修复现有测试并补充一致性哈希故障转移场景的单元测试**
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在需要重写测试文件。主要修改：
1. 修复 `TestConsistentHash_DrainedServer` — 不再静默跳过错误
2. 修复所有受 `RemoveServer` 变更影响的测试（需要先 Drain 再 Remove）
3. 补充一致性哈希故障转移场景测试
4. 补充 RemoveServer 安全检查的测试
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\httplb_test.go
          
changes: undefined
          
现在运行测试。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/httplb/ -v undefined
          
所有 45 个测试用例全部通过！现在更新文档。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\httplb.md
          
现在更新文档，反映所有修复内容。
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\httplb.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有任务已完成。以下是本次修复的完整总结：

## 修复总结

### 1. 一致性哈希故障转移 ✅
**问题**：`Next` 方法在目标节点不健康时直接返回 `ErrNoHealthyServer`，导致哈希到该节点的请求全部失败。

**修复**：沿哈希环顺时针遍历，使用 `visited` 集合去重，跳过不健康节点继续查找，直到找到健康服务器或遍历完所有物理节点。

### 2. RemoveServer 安全移除机制 ✅
**问题**：`RemoveServer` 直接删除服务器，不检查活跃连接和状态。

**修复**：新增两个前置条件检查——必须先 `DrainServer`（`ErrServerNotDraining`）且活跃连接为 0（`ErrServerHasConns`），建立完整的 `Drain → 等待连接归零 → Remove` 安全下线流程。

### 3. 移除死代码 ✅
**问题**：`RoundRobin` 结构体中 `mu sync.Mutex` 从未使用。

**修复**：删除该字段和 `sync` 导入，计数器并发安全完全由 `sync/atomic` 保证。

### 4. 修复测试静默跳过错误 ✅
**问题**：`TestConsistentHash_DrainedServer` 遇到 `ErrNoHealthyServer` 时静默跳过，掩盖了故障转移缺陷。

**修复**：改用 `t.Fatalf` 确保错误不被忽略；同时更新所有受 `RemoveServer` 新契约影响的测试用例。

### 5. 补充故障转移测试 ✅
新增 7 个测试用例：
- `TestConsistentHash_Failover_OnDrain` — 单节点 Drain 后请求重定向到健康节点
- `TestConsistentHash_Failover_MultipleDrain` — 多节点 Drain 后请求全部路由到剩余健康节点
- `TestConsistentHash_Failover_AllDown` — 所有节点 Drain 后返回 `ErrNoHealthyServer`
- `TestConsistentHash_Failover_RestoreAfterDrain` — Restore 后服务器重新接收请求
- `TestConsistentHash_RemoveServerRequiresDrain` — 直接 Remove 返回 `ErrServerNotDraining`
- `TestServerPool_RemoveServerWithActiveConns` — 有活跃连接时 Remove 返回 `ErrServerHasConns`
- `TestServerPool_RemoveServerNotDraining` — 未 Drain 时 Remove 返回 `ErrServerNotDraining`

### 6. 测试结果 ✅
全部 **45 个测试用例通过**（`go test ./internal/httplb/ -v`）。

### 7. 文档更新 ✅
[docs/httplb.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/httplb.md) 已更新，主要变更：
- F4/F6 功能描述更新（故障转移、安全移除）
- Section 3.4 移除 `mu sync.Mutex`，标注"无需额外互斥锁"
- Section 3.7 新增"故障转移机制"说明
- Section 5.2 重写 `RemoveServer` 流程图，增加安全移除约束和推荐下线流程
- 新增 Section 6 "一致性哈希故障转移机制"
- Section 8 新增 `ErrServerHasConns` 和 `ErrServerNotDraining`
- Section 9.5 改写为"完整的优雅下线流程"示例
- 新增 Section 9.7 "一致性哈希的故障转移"示例