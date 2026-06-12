我来逐一修复这些问题。让我先创建一个任务计划：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
好的，我来逐一修复这些问题。让我先读取当前的 cluster.go，然后一次性修复前三个问题：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
现在我来修复这三个问题。我会重写 cluster.go 来解决：
1. `migrateOnRemove` 过度复制问题
2. `MarkShardUp` 先置可用再同步的时序问题
3. 移除死代码 `keyOwnerCache`




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          
好的，三个代码问题已经修复完成。现在让我更新任务状态并补充单元测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我来读取现有测试文件，然后添加补充的测试用例：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\shardkv_test.go
          
现在我来添加补充的测试用例：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\shardkv_test.go
          
changes: undefined
          
现在运行测试看看是否全部通过：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/shardkv/ -v -count=1 -timeout 120s undefined
          
所有 43 个测试全部通过！现在更新文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
现在我来更新文档，添加修复后的数据迁移策略和故障恢复时序保证：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
changes: undefined
          
现在更新测试覆盖部分，反映新增的补充测试：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
changes: undefined
          
最后再运行一次测试确认所有内容正常：










        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/shardkv/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题已修复完成，所有测试通过。以下是修复总结：

---

## ✅ 修复完成总结

### 1. 修复 `migrateOnRemove` 过度复制问题
**文件**：[cluster.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L174-L255)

**原问题**：无条件向所有剩余分片执行 `ForcePut`，导致副本数远超配置

**修复策略**：
- 先统计其他可用分片中该键的**已有副本数**
- 计算 `needed = ReplicaCount - 已有副本数`
- 沿一致性哈希环按顺序查找候选目标
- 目标已持有该键 → 跳过
- 目标未持有 → ForcePut，`needed--`
- `needed == 0` 时立即停止，精确控制副本数量

### 2. 修复 `MarkShardUp` 时序问题
**文件**：[cluster.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L272-L289)

**原问题**：先将分片置为 Up，再异步同步数据，时间窗口内 Get 可能返回不存在

**修复策略**：
- 调换顺序：**先同步数据，再置为可用**
- `MarkShardUp` 变为**同步阻塞**调用
- 返回时数据已全部就绪，零时间窗口
- 消除了"状态已上线但数据未同步"的间隙

### 3. 移除死代码 `keyOwnerCache`
- 移除了 `keyOwnerCache map[string]string` 字段
- 移除了 `cacheMu sync.RWMutex` 字段
- 移除了 `invalidateCache()` 方法
- 移除了 Put/Delete 中对缓存的写入操作

### 4. 补充单元测试（新增 6 个）
**文件**：[shardkv_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/shardkv_test.go)

| 测试名 | 验证内容 |
|--------|---------|
| `TestMigration_RemoveShard_NoOverCopy` | 移除分片后总副本数 = 键数 × ReplicaCount，无过度复制 |
| `TestMigration_AddShard_ReplicaCountCorrect` | 新增分片前后总副本数保持一致 |
| `TestMigration_TotalKeysConsistency` | 多次扩缩容后总键数和值正确性 |
| `TestFailover_RecoveryTiming` | MarkShardUp 返回后立即可读取所有数据 |
| `TestFailover_RecoveryDataIntegrity` | 多节点轮流宕机/恢复的数据完整性 |
| `TestMigration_RemoveShard_ReplicaPreservation` | 移除分片时其他分片已有数据不被破坏 |

### 5. 测试结果

```
PASS: 43/43 全部通过
ok  solocoder-go/internal/shardkv  0.599s
```

### 6. 更新文档
**文件**：[docs/shardkv.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/shardkv.md)

新增/更新章节：
- **4.5 节点移除流程**：详细说明精确副本计数、按需补充、及时终止的迁移策略
- **4.6 故障恢复时序保证**：两阶段流程（离线同步 → 原子上线）、零时间窗口、读一致性保证
- **8. 测试覆盖**：更新为更详细的分类统计