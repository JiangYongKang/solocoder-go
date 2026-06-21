我来分析并修复这三个问题。让我先创建任务列表，然后逐一修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
首先让我读取相关源文件，了解当前实现细节。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\sync.go
          
现在我已经清楚地了解了三个问题。让我逐一修复：

## 问题 1：Store 结构体中只读方法使用排他锁

将 `sync.Mutex` 改为 `sync.RWMutex`，只读方法使用读锁，写方法保持写锁。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          
changes: undefined
          
现在将所有只读方法改为使用读锁（RLock/RUnlock），写方法保持写锁。让我逐个修改：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          
changes: undefined
          
现在修改 ExportSession、ExportAll、Stats 这些只读方法也改为读锁：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\distsess.go
          
changes: undefined
          
问题 1 已修复。现在标记完成并开始修复问题 2。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 问题 2：Node.Get 方法无条件广播变更通知

需要先了解 `TieredStore.Get` 在什么情况下会产生变更（即自动续期）。让我查看：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\tiered_store.go
          
我看到 `TieredStore.Get` 在 `autoRenew=true` 且会话未过期的情况下会执行续期操作。问题在于 `Node.Get` 无条件地广播，即使没有实际产生变更。

修复策略：在 `Node.Get` 中，先使用 `GetWithoutRenew` 获取原始会话，然后根据 `autoRenew` 配置决定是否续期，只在真正续期成功时才广播。

但更简单的方法是：比较续期前后的版本号。如果版本号变了，说明发生了续期，才需要广播。

让我修改 `Node.Get` 方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\sync.go
          
changes: undefined
          
问题 2 已修复。现在开始修复问题 3。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 问题 3：handleChangeNotify 硬编码变更类型

修复策略：在调用 `mergeRemoteSession` 之前先检查本地是否存在该会话，如果不存在则是 `Create` 事件，否则是 `Update` 事件。






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distsess\sync.go
          
changes: undefined
          
所有三个问题都已修复。现在运行测试验证。






        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/distsess/ -v -count=1 2>&1 undefined
          
所有 70+ 测试全部通过！让我标记任务完成。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
三个问题已全部修复完成，所有测试通过！

---

## 修复总结

### 问题 1：Store 只读方法使用排他锁 ✅

**修改文件**: [distsess.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L11-L21)

**改动**:
- 将 `Store.mu` 从 `sync.Mutex` 改为 `sync.RWMutex`
- 以下只读方法改用 `RLock/RUnlock`：
  - [Get](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L93-L102)
  - [Exists](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L191-L200)
  - [GetAll](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L202-L211)
  - [ExportSession](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L236-L250)
  - [ExportAll](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L252-L266)
  - [Stats](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/distsess.go#L387-L392)
- 写方法（Set/Delete/Renew/Import*/CleanupExpired/Clear/Close）保持使用 `Lock/Unlock`

**效果**: 底层 `TieredStore` 已有自己的并发保护，外层 Store 的读操作可并发执行，读密集场景吞吐量大幅提升。

---

### 问题 2：Node.Get 无条件广播变更 ✅

**修改文件**: [sync.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/sync.go#L355-L386)

**改动**:
- 当 `AutoRenew=false` 或 `EnableSync=false` 时，直接返回，不做任何广播
- 否则先调用 `GetWithoutRenew` 获取续期前的版本号，再调用 `Get` 执行可能的续期
- 仅当 `session.Version > beforeVersion`（即确实发生了续期）时才广播变更通知

**效果**: 纯读取操作不再触发跨节点网络广播，只有实际产生变更（续期）时才同步。

---

### 问题 3：handleChangeNotify 硬编码变更类型 ✅

**修改文件**: [sync.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distsess/sync.go#L290-L324)

**改动**:
- 在调用 `mergeRemoteSession` 前，通过 `local != nil` 判断本地是否已存在该会话
- 如果本地不存在 → `ChangeTypeCreate`（新建事件）
- 如果本地已存在 → `ChangeTypeUpdate`（更新事件）

**效果**: 变更处理器现在能正确区分远程同步过来的是新会话创建还是已有会话更新。

---

### 测试结果

```
PASS
ok      solocoder-go/internal/distsess  2.492s
```

**70+ 测试用例全部通过**，覆盖正常流程、边界条件和异常分支。