我来修复这些问题。首先让我查看当前的实现代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
现在我完全理解了问题。让我逐个修复：

**问题1**：cacheEntry 未存储响应头，缓存命中时 header 丢失
**问题2**：pendingEntry 未存储错误，Stop 后等待请求返回默认零值

让我开始修复代码：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
我将重写整个 idempotent.go 文件，修复所有问题：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
changes: undefined
          
现在更新任务状态并修改测试文件：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在更新测试文件，适配新的 API 并添加新的测试用例：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent_test.go
          
changes: undefined
          
现在运行测试并启用竞态检测：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
          
让我运行竞态检测测试：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v -race 2>&1 | Out-String undefined
          
看起来 -race 在 32 位 Windows 上不支持。让我直接运行普通测试：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v 2>&1 | Out-String undefined
          
有个编译错误，让我修复测试代码：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent_test.go
          
changes: undefined
          
现在重新运行测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v 2>&1 | Out-String undefined
          
问题是 Stop() 关闭了 pending.done，然后 Execute 完成后又尝试关闭同一个 channel，导致 panic。让我修复这个问题：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
问题在于 Stop() 已经关闭了 pending.done，然后第一个请求 handler 执行完又尝试再次关闭。我需要修复这个竞争条件：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
问题清楚了：Stop() 已经关闭了 pending.done，第一个请求完成后又尝试再次关闭。我需要修改 Execute 中 stopped 分支，先检查 pending 是否已被 Stop() 处理：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
changes: undefined
          
现在重新运行测试：










        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v 2>&1 | Out-String undefined
          
所有 47 个测试全部通过！现在更新任务状态并更新文档：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我读取当前文档并进行更新：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\idempotent.md
          
现在让我更新文档，反映 API 的变化：













        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\idempotent.md
          
changes: undefined
          
现在让我更新任务状态：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有修复工作已完成！以下是本次修复的详细总结：

## 修复内容总结

### 问题 1：缓存命中时响应头丢失 ✅

**原因分析**：原 `cacheEntry` 结构体仅存储了 `statusCode` 和 `body`，未存储业务设置的响应头（如 `Content-Type`、`Location`、自定义头等）。缓存命中时业务 handler 不执行，`responseRecorder` 的 header 为空，导致所有响应头丢失。

**修复方案**：
- 新增 `Response` 结构体，统一封装 `StatusCode`、`Body`、`Header` 三个字段
- `cacheEntry` 增加 `header http.Header` 字段存储完整响应头
- `pendingEntry` 增加 `header http.Header` 字段
- 新增 `cloneHeader()` 函数实现响应头深拷贝，确保缓存与调用方隔离
- `Middleware` 在缓存命中时从 `resp.Header` 完整恢复所有响应头
- `Execute`、`Get`、`Set` 等 API 均适配新的响应结构

### 问题 2：Stop 后等待请求返回错误值 ✅

**原因分析**：原 `pendingEntry` 没有 `err` 字段。当 `Stop()` 被调用时，它关闭了 `pending.done` channel，但等待的请求被唤醒后读取到的是未初始化的零值（`statusCode=0`, `body=nil`），并错误地返回"命中缓存且无错误"。

**修复方案**：
- `pendingEntry` 增加 `err error` 字段
- `Stop()` 遍历所有 pending 条目，设置 `pending.err = ErrIdempotentStopped` 后再关闭 channel
- 等待的请求被唤醒后先检查 `pending.err != nil`，若有错误则正确返回
- 修复了 `Stop()` 与第一个请求 handler 完成后重复关闭 channel 导致的 panic：第一个请求重新获取锁时先检查 `pending` map 中自己的条目是否仍存在，若已被 `Stop()` 移除则直接返回错误，不再尝试关闭 channel

### 新增测试用例 ✅

共新增 6 个测试用例，全部通过：
- `TestGet_HeaderIsolation`：验证缓存响应头的深拷贝隔离性
- `TestCloneHeader`：验证 `cloneHeader` 函数的正确性
- `TestMiddleware_WithKeyCacheHit`：验证缓存命中时 Content-Type、Location、自定义头等所有响应头完整恢复
- `TestMiddleware_CacheHitPreservesMultipleHeaderValues`：验证多值响应头的完整性
- `TestExecute_StopDuringHandler_WaitingRequestsReturnError`：验证 Stop 时第一个请求和所有等待请求均返回 `ErrIdempotentStopped`
- `TestExecute_StopWithPendingRequests`：验证 Stop 期间无 deadlock 和 panic

### 测试运行结果 ✅

```
PASS
ok      solocoder-go/internal/idempotent        5.563s
```

**47 个测试用例全部通过**。

### 文档更新 ✅

[docs/idempotent.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/idempotent.md) 已完整更新，包含：
- 新增 `Response` 结构体说明
- `cacheEntry` / `pendingEntry` 新字段说明
- Stop 期间等待请求错误通知机制的详细流程图
- 响应头深拷贝机制说明
- 更新的方法签名表
- 新增使用示例（Stop 期间错误处理、完整响应头使用）
- 更新的测试覆盖说明表

### 主要代码变更文件

- [idempotent.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/idempotent/idempotent.go)：核心实现修复
- [idempotent_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/idempotent/idempotent_test.go)：新增测试用例
- [idempotent.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/idempotent.md)：需求文档更新

**注意**：当前运行环境为 windows/386，不支持 `-race` 竞态检测标志。在 linux/amd64、darwin/amd64、windows/amd64 等 64 位平台上可使用 `go test ./internal/idempotent/ -v -race` 运行竞态检测。