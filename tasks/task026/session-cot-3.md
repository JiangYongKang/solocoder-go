toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache.go
          
我现在已经清楚地理解了这三个问题。让我来实施修复：

**问题1**：Delete 中 `l1.delete(key)` 将 Dirty=true 的 entry 放入异步回调队列，但 `deleteFromL2(key)` 已立即删除磁盘文件，异步回调后执行时 `handleL1Eviction` 会重新写入磁盘文件。

**问题2**：`handleL1Eviction` 写入失败时静默丢弃，没有用 `writeBackErrors` 追踪。

**问题3**：`failedEntries` 声明了但从未使用。

修复方案：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题2：`handleL1Eviction` 写入失败时追踪错误：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache.go
          
changes: undefined
          
现在修复问题3：移除 `flushWriteBack` 中未使用的 `failedEntries` 切片：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在补充测试用例。让我先查看测试文件末尾：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -v -timeout 300s 2>&1 undefined
          
`TestHandleL1Eviction_TracksWriteFailure` 失败是因为 Windows 上 `os.Chmod` 不能真正阻止文件创建。让我改用删除目录的方式让写入失败：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          
同样修复其他使用 `os.Chmod` 的测试（Windows 上 chmod 无法阻止文件创建）：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
这两个测试也在使用 `os.Chmod`，在 Windows 上不可靠。让我一并修复：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          
现在修复 `TestWriteBackMaxRetriesExceeded`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -v -timeout 300s 2>&1 undefined
          
进程异常退出，让我尝试只运行新增的测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -run "TestHandleL1Eviction_TracksWriteFailure" -v -timeout 60s 2>&1 undefined
          
单个测试通过了。让我运行所有 Delete 相关的新测试和修改后的测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -run "TestDelete_WriteBack|TestHandleL1Eviction|TestWriteBackFailure|TestWriteBackMaxRetries" -v -timeout 120s 2>&1 undefined
          
问题在于 `Flush()` 需要调用多次才能超过重试限制，以及 `os.RemoveAll` 在 Windows 上可能不能完全删除正在使用的目录。让我重构这些测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tieredcache\tieredcache_test.go
          
changes: undefined
          
先运行这几个关键测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -run "TestWriteBackFailure|TestWriteBackMaxRetries|TestDelete_WriteBack|TestHandleL1Eviction_Tracks" -v -timeout 60s 2>&1 undefined
          
关键测试全部通过！现在运行全部测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -timeout 300s -count=1 2>&1 undefined
          
所有测试通过！现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          
现在我需要在 5.4 淘汰流程之后添加 Delete 流程的文档，并在并发安全部分更新相关内容。让我在 5.4 后添加 5.5 Delete 流程：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          
changes: undefined
          
现在更新并发安全部分中的淘汰回调和回写失败追踪的内容，确保文档反映 `handleL1Eviction` 也使用 `writeBackErrors` 追踪失败：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          
现在更新 7.6 回写失败部分，添加 `handleL1Eviction` 的一致失败追踪说明：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          
changes: undefined
          
现在更新并发一致性保证表格，加入 Delete 语义：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tieredcache.md
          
changes: undefined
          
现在运行最终测试确认所有测试通过：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/tieredcache/ -v -timeout 300s 2>&1 undefined
          
全部 **57 个测试通过**！
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题修复完成！以下是本轮修复总结：

## 修复总结

### 问题 1：Delete 在 WriteBack 模式下的数据泄漏

**根因**：[Delete](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tieredcache/tieredcache.go#L531-L545) 调用 `l1.delete(key)` 时，Dirty=true 的条目被放入异步淘汰队列。随后 `deleteFromL2(key)` 立即删除磁盘文件。异步回调 `handleL1Eviction` 延迟执行时发现 Dirty=true，调用 `writeToL2` 将已删除的数据重新写回磁盘，导致数据永久残留。

**修复**：在 `l1.delete(key)` 之前先调用 `l1.clearDirty(key)`，确保异步回调执行时看到 Dirty=false，跳过写入操作。

```go
tc.l1.clearDirty(key)  // 先清除 Dirty 标记
tc.l1.delete(key)       // 再删除（入异步队列的条目 Dirty=false）
tc.l2.delete(key)
tc.deleteFromL2(key)
```

### 问题 2：handleL1Eviction 静默丢弃错误

**根因**：[handleL1Eviction](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tieredcache/tieredcache.go#L547-L558) 中 `writeToL2` 失败时直接 `return`，调用方无法感知脏数据丢失。

**修复**：与 `flushWriteBack` 使用统一的 `writeBackErrors` 计数器，失败时调用 `tc.writeBackErrors.Add(1)` 记录错误。

### 问题 3：flushWriteBack 中未使用的 failedEntries 切片

**根因**：[flushWriteBack](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tieredcache/tieredcache.go#L585-L611) 声明了 `failedEntries` 切片并追加数据，但循环结束后从未使用。

**修复**：移除 `failedEntries` 变量，失败条目通过 `FailCount` 机制和 `writeBackErrors` 计数器追踪。

### 新增测试用例（5 个）

| 测试 | 验证内容 |
|------|----------|
| `TestDelete_WriteBackNoDataLeak` | Delete 后 Dirty 条目不会重新写入磁盘 |
| `TestDelete_WriteBackNoDataLeakAfterFlush` | Flush 后 Delete 磁盘文件被正确删除 |
| `TestDelete_WriteBackEvictionNoDataLeak` | L1 淘汰后 Delete 不会导致数据泄漏 |
| `TestDelete_MultipleKeysWriteBackNoResidue` | 批量 Delete 后磁盘无残留文件 |
| `TestHandleL1Eviction_TracksWriteFailure` | L1 淘汰回调写入失败时 writeBackErrors 递增 |

### 测试结果

```
PASS — ok  solocoder-go/internal/tieredcache  49.431s
```

全部 **57 个测试通过**（原 52 + 新增 5）。