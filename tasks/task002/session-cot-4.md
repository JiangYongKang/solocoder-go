我来清理这些死代码和不可达路径。先读取当前的 cluster.go 文件：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
我现在清理这两处死代码和不可达路径。分析后，对于问题2我选择**移除不可达的回滚逻辑**，因为 `migrateOnRemove` 使用的 `ForcePut/ForceDelete` 不会返回错误，补充错误路径没有实际意义；且迁移部分失败时中止移除操作会导致更严重的状态不一致。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在我来修复这两个问题：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          
现在修改 `migrateOnRemove` 的函数签名，移除 error 返回值，并清理两处死代码：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
现在移除这三处死代码：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在运行测试验证：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/shardkv/ -v -count=1 undefined
          
所有测试都通过了！



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
---

## ✅ 代码清理完成

### 🔧 修复内容（[cluster.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go)）

| 问题 | 修复 |
|------|------|
| **死代码 #1**：第 235 行 `if tid == removedShardID` | 哈希环已在 `RemoveShard` 开头调用 `RemoveNode`，`newRingReplicas` 是从新环取的副本集合，不可能包含被移除 ID。已移除该守卫。 |
| **死代码 #2**：第 260 行 `if sid == removedShardID` | `remainingShardIDs` 在构造时已显式排除了 `removedShardID`。已移除该守卫。 |
| **不可达回滚**：第 97-102 行 `if err != nil` 分支 | `migrateOnRemove` 使用 `ForcePut/ForceDelete` 操作，无错误返回路径，始终返回 `nil`。已移除该不可达分支，并将 `migrateOnRemove` 签名改为无返回值。 |

### 🧪 测试结果

```
PASS: 41/41 全部通过
ok  solocoder-go/internal/shardkv  0.578s
```

### 📝 代码变更摘要

1. **[RemoveShard](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L80-L103)**：移除了错误检查和回滚逻辑，直接调用 `migrateOnRemove`
2. **[migrateOnRemove](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L170-L267)**：
   - 函数签名从 `func(...) error` 改为 `func(...)`
   - 移除了两处 `== removedShardID` 的死代码守卫
   - 移除了末尾的 `return nil`