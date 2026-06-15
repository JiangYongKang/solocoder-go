你说得对，生产代码不应该为测试妥协。让我移除测试专用字段，通过间接方式验证协程启动行为。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
首先移除生产代码中的测试专用字段：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          
现在移除 `Start` 方法中的递增逻辑：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在重写测试，通过**状态一致性 + 功能验证 + 幂等循环**的间接方式来验证协程启动行为：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
现在运行测试验证：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v -run "TestConcurrentStartStop" 2>&1 | Out-String undefined
          
过期事件没触发，应该是默认 TTL 太长了。让我使用更短的 TTL 配置：







        
            
toolName: search_by_regex
            
status: success
          
query: defaultHeartbeatTTL|defaultCheckInterval
          
默认 TTL 是 30 秒，难怪 150ms 内不会过期。我来使用更短的配置重写测试：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v -run "TestConcurrentStartStop" 2>&1 | Out-String undefined
          
`TestConcurrentStartStop` 通过了！现在运行全部测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v 2>&1 | Out-String undefined
          
所有 55 个测试全部通过！







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## 完成情况

### 1. 生产代码净化 — [servicereg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg.go#L65-L75)

移除了 `Registry` 结构体中的 `expiryLoopStarts` 字段以及 `Start` 方法中对该字段的递增逻辑，保持了生产代码的纯粹性，没有任何测试专用字段。

### 2. 重写 `TestConcurrentStartStop` 测试 — [servicereg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg_test.go#L1560-L1679)

通过**间接验证手段**覆盖以下行为：

| 验证维度 | 验证方式 | 说明 |
|---------|---------|------|
| **初始状态** | 检查 `expiryRunning == false` | 确保启动前状态正确 |
| **并发 Start 幂等性** | 10 个并发 goroutine 调用 Start，然后检查 `expiryRunning == true` | 验证并发 Start 后状态一致，没有竞态 |
| **协程实际运行** | 注册实例 → 订阅过期事件 → 等待 TTL 后验证收到 expire 事件 | 功能级验证：后台协程确实在工作（至少有一个） |
| **Stop 状态** | 检查 `expiryRunning == false` | 验证 Stop 后状态正确 |
| **重启功能** | 再次 Start → 注册新实例 → 验证收到新的过期事件 | 验证协程能正常重启 |
| **第二次 Stop** | 检查 `expiryRunning == false` | 验证 Stop 幂等性 |
| **5 轮幂等循环** | 每轮 3 次 Start + 3 次 Stop，验证状态一致性 | 验证 Start/Stop 的幂等语义在多次调用下稳定 |

**核心设计思路**：不直接统计协程数量，而是通过**可观测的行为和状态**来间接验证：
- 如果 `expiryRunning` 状态位保持正确的布尔值，说明 Start 的幂等逻辑（锁 + 标志位检查）有效
- 如果过期功能正常工作，说明至少有一个后台协程在运行
- 如果多次 Start/Stop 循环后状态始终一致，说明生命周期管理是稳健的

### 3. 测试结果

```
PASS
ok      solocoder-go/internal/servicereg        5.433s
```

所有 55 个测试全部通过 ✓