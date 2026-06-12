让我先读取当前 cluster.go 的内容，了解现有实现然后逐一修复所有问题：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
让我先读取测试文件，了解当前测试覆盖情况：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\shardkv_test.go
          
现在我清楚了所有问题。需要修复的核心是：
1. **哈希环顺序问题**：migrateOnRemove 时使用的是包含待删除节点的旧环
2. **existingReplicas 范围问题**：需要限定统计在新环的合法 replica 集合内

让我重写 cluster.go 的关键部分：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
我直接修复 cluster.go 中的核心问题。关键点：**先从哈希环移除节点，再执行迁移**，这样迁移时用的就是新环；同时 existingReplicas 严格限定在新环合法 replica 范围内。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          
现在修复 migrateOnRemove 内部的 existingReplicas 计数逻辑，严格限定在新环合法 replica 集合内：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\cluster.go
          
changes: undefined
          
好的，现在运行测试验证修复效果：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/shardkv/ -v -count=1 -timeout 60s undefined
          
所有 42 个测试全部通过！现在更新文档，说明修复后的哈希环一致性和迁移策略。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我读取现有文档，然后更新对应章节：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
现在更新文档，重点补充哈希环一致性和目标选取约束：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
changes: undefined
          
所有修复工作已完成！让我最后运行一次测试确认：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/shardkv/ -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题修复完成，`go test ./internal/shardkv/` 测试全部通过（42 个用例，耗时 0.58s）。

---

## ✅ 修复内容总结

### 问题 1：哈希环时序问题

**根因**：`RemoveShard` 中 `migrateOnRemove` 执行时，哈希环仍包含待删除节点。旧环的节点排序与新环不同，导致旧环选定的迁移目标在新环中不再是合法 replica。

**修复**：调整 `RemoveShard` 执行顺序，在 [cluster.go#L93-L109](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L93-L109) 中：

```
原顺序：migrateOnRemove → hashRing.RemoveNode  ❌（旧环选目标）
新顺序：hashRing.RemoveNode → migrateOnRemove  ✅（新环选目标）
```

增加失败回滚：迁移失败时先 `hashRing.AddNode` 把节点重新挂回哈希环，再恢复 `Up` 状态。

---

### 问题 2：existingReplicas 范围过宽

**根因**：`existingReplicas` 遍历全部存活分片计数，未限定在"新环合法 replica 集合"范围内。非合法 replica 上的陈旧副本被计入后 `needed` 被低估，导致合法节点缺少应有副本。

**修复**：在 [cluster.go#L204-L224](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/shardkv/cluster.go#L204-L224) 中：

```go
newRingReplicas := c.hashRing.GetReplicaNodes(key, replicaCount)  // 合法集合
newRingReplicaSet := map[string]struct{}{...}

existingReplicas := 0
for _, sid := range remainingShardIDs {
    if _, isLegal := newRingReplicaSet[sid]; !isLegal {
        continue  // 非合法 replica → 不计入
    }
    ...  // 存活且已持有键 → existingReplicas++
}
```

迁移目标首选 `newRingReplicas` 内节点；只有合法节点仍不足时才退回到非合法节点兜底。

---

### 文档更新

[docs/shardkv.md#L234-L295](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/shardkv.md#L234-L295) 中新增了三阶段流程图（Stage A 切环 → Stage B 迁移 → Stage C 清理），并补充了 8 条策略说明：哈希环一致性原则、合法副本集合约束、精确副本计数范围、回滚保证等。