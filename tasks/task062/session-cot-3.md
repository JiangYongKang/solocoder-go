我来修复这两个问题。首先读取相关文件了解当前状态。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\quotamgr\quotamgr_test.go
          
现在我来修复这两个问题。先修复文档：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          
changes: undefined
          
让我也更新文档中性能考量部分的描述，确保一致性：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          
changes: undefined
          
现在修复第二个问题：测试用例中的不稳定断言：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\quotamgr\quotamgr_test.go
          
changes: undefined
          
现在让我运行测试确保所有修复都正确：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/quotamgr/ -v -count=1 undefined
          
所有 43 个测试都通过了！让我完成任务标记。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## ✅ 所有问题已修复完成

### 修复内容总结：

### 1. **文档锁顺序描述修正** ([docs/quotamgr.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/quotamgr.md#L523-L534))

**问题**：第 8.2 节写"禁止在持有租户级锁的情况下获取全局锁"，与第 8.3.1 节 `AcquireResource` 的实际实现（先租户级写锁 → 再全局读锁）矛盾。

**修复**：将锁获取顺序规则修正为三条明确规则：
- ✅ **允许**：先全局锁 → 再租户级锁（如 `getOrCreateTenantUsage`）
- ✅ **允许**：先租户级锁 → 再全局读锁（全局读锁具备共享特性，如 `AcquireResource` 的实现，安全无死锁）
- ❌ **禁止**：先租户级锁 → 再全局写锁（可能导致死锁）

同步更新了 [8.6 节性能考量](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/quotamgr.md#L642-L648) 中的相关描述。

---

### 2. **测试不稳定断言修复** ([TestAcquireResource_ConcurrentAdjustQuota](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/quotamgr/quotamgr_test.go#L928-L1021))

**问题**：原测试依赖 `i==50` 的 goroutine 调度时序触发 `AdjustQuota`，当调度器过早执行配额缩减时，`info.Usage.CPU < 5.0` 的断言会误报失败。

**修复**：
- 移除所有依赖调度时序的不稳定断言
- 使用 channel 精确控制执行顺序，确保测试行为完全确定：
  1. 先申请 8.0 CPU 并等待完成
  2. 触发 AdjustQuota 缩减到 5.0 并等待完成
  3. 验证超额申请被拒绝
  4. 释放 4.0 使使用量降到 4.0
  5. 验证可以重新申请 1.0 达到 5.0
  6. 验证达到上限后申请被拒绝

---

### 测试结果
所有 **43 个测试全部通过** ✅