我来增强 `TestConcurrentStartStop` 测试，补充对 `expireLoop` 协程启动次数的验证。

首先，我需要在代码中添加一个机制来统计 `expireLoop` 的启动次数。让我先查看当前的实现：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
我需要在 `Registry` 结构体中添加一个测试用的启动次数字段，并在 `expiryLoop` 启动时递增它。让我修改代码：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          
现在在 `expiryLoop` 启动时递增计数器：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
让我先检查导入部分：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
很好，`sync/atomic` 已经导入。现在在 `expiryLoop` 开头添加计数器递增：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          
现在增强 `TestConcurrentStartStop` 测试，添加对 `expiryLoopStarts` 计数器的验证：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
现在运行测试确保所有测试通过：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v -run "TestConcurrentStartStop" 2>&1 | Out-String undefined
          
问题是计数器递增在协程内部，检查时协程可能还没执行到。让我修改为在 `Start()` 中持锁时递增计数器，这样更可靠：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          
现在再次运行测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v -run "TestConcurrentStartStop" 2>&1 | Out-String undefined
          
`TestConcurrentStartStop` 测试通过了！现在运行全部测试确保没有破坏其他测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v 2>&1 | Out-String undefined
          
所有 55 个测试全部通过！下面是修改总结：

## 修复内容

### 1. 新增 `expiryLoopStarts` 计数器 — [servicereg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg.go#L72)

在 `Registry` 结构体中新增 `expiryLoopStarts int32` 字段，用于统计 `expiryLoop` 协程的启动次数。

### 2. 计数器递增位置调整 — [servicereg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg.go#L351)

将计数器递增从 `expiryLoop()` 协程内部（异步）移到 `Start()` 方法中持锁时（同步），避免检查时协程尚未执行到递增语句的竞态问题：

```go
r.expiryRunning = true
r.wg.Add(1)
atomic.AddInt32(&r.expiryLoopStarts, 1)  // 同步递增，确保可见性
r.mu.Unlock()

go r.expiryLoop()
```

### 3. 增强 `TestConcurrentStartStop` 测试 — [servicereg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg_test.go#L1560-L1627)

补充了对后台协程启动次数的独立验证，完整的验证流程：

| 验证点 | 检查逻辑 |
|--------|---------|
| 初始状态 | `expiryLoopStarts == 0` |
| 并发 Start 后状态 | `expiryRunning == true` 且 `expiryLoopStarts == 1`（10 个并发只启动了 1 个协程） |
| Stop 后状态 | `expiryRunning == false` 且 `expiryLoopStarts` 保持不变 |
| 重启后状态 | `expiryRunning == true` 且 `expiryLoopStarts == 2` |
| 第二次 Stop 后 | `expiryRunning == false` 且 `expiryLoopStarts == 2` |

这样即使未来 `Start()` 方法的幂等逻辑被错误修改导致多次启动协程，测试也能捕获这一回归。

### 测试结果

```
PASS
ok      solocoder-go/internal/servicereg        5.028s
```

所有 55 个测试全部通过 ✓