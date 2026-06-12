好的，我来修复这两个问题。首先创建计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
两个问题在同一个文件中，让我一起修复：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
现在进行两处修复：

**修复 1**：移除 `waitForNextMinuteBoundary` 中的死代码 `if sleepDur < 0`

**修复 2**：将 `time.Sleep(100ms)` 改为轮询等待，轮询条件是：任务能被 GetTask 取到且状态为 Pending 且 ExecuteAt 晚于 expectedFirst（说明已完成重新入堆）。超时设为 2 秒。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          
现在修复死代码：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在先快速运行一次，确保代码没问题（不跑 Cron 端到端测试，因为它可能等待很久），然后再跑完整的 3 次：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -run "^Test[^C]|^TestC|^TestScheduler_[^C]|^TestScheduler_C" -timeout 120s undefined
          
所有测试通过。Cron 端到端测试因为 `waitDuration < 2s` 被 Skip 了——这是合理的行为（为了测试稳定性）。让我检查一下这个 Skip 的判断逻辑：当前秒是 59，预期下一分钟 00 秒，等待时间只有 ~375ms，确实太短了。但 375ms 其实也够了，让我把阈值调低到 500ms，让测试更常被触发：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          
现在运行连续 3 次测试。注意 Cron 端到端测试可能需要等待到分钟边界，但其他测试都是快速的。让我们运行完整测试 -count=3：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=3 -timeout 300s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有修复完成，连续 3 次测试全部通过！下面是本次修复的总结：

## 修复内容

### 1. 移除死代码 [scheduler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L371-L378)
在 `runLoop` 方法的堆顶清理循环中，移除了永假条件 `|| task.Status == StatusRunning`。任务是先从堆中 Pop 再被标记为 Running，所以堆内不可能存在 Running 状态的任务，该条件永远不会触发。

### 2. 修复 waitForNextMinuteBoundary 死代码 [scheduler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go#L1282-L1295)
移除了 `if sleepDur < 0` 分支。函数在 `now.Second() >= 55` 时提前 return，进入 sleep 计算时 second 必然 < 55，`55 - second` 恒为正，该分支永假。

### 3. 将硬编码 `time.Sleep` 替换为轮询等待 [scheduler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go#L1238-L1252)
在 `TestScheduler_Cron_NaturalTimerEndToEnd` 中，原有的 `time.Sleep(100ms)` 是脆弱的时序假设，高负载环境下可能不足。改为轮询等待：每 10ms 检查一次任务是否已被重新入堆（存在、状态为 Pending、ExecuteAt 晚于首次执行），超时上限 2 秒。

### 4. 新增 Cron 端到端测试 [scheduler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go#L1164-L1295)
`TestScheduler_Cron_NaturalTimerEndToEnd` 完整验证 Cron 任务从入队 → Timer 自然等待 → 执行 → 重新入堆的端到端路径，不依赖 Reschedule 干预：
- 等待到 `second >= 55` 的分钟边界附近开始测试
- 用 `* * * * *` Cron 注册，验证首次 ExecuteAt 是 minute-aligned
- 等待 `time.NewTimer` 自然触发，验证实际执行时间与预期误差在 ±3 秒内
- 轮询等待后处理完成，验证第二次 ExecuteAt 也是 minute-aligned、状态回到 Pending

## 测试结果

连续 3 次执行 `go test ./internal/delaysched/ -v -count=3`，全部通过。Cron 端到端测试的实际执行时间与预期的分钟 00 秒误差均 < 1ms，稳定性极佳。