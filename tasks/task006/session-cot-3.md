好的，我来修复这些问题。首先创建计划：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
首先修复 scheduler.go 中的永假代码：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在补充 Cron 端到端测试。策略：等待当前时间接近分钟边界（秒数 > 55），用 `* * * * *` Cron 注册任务，验证它在下一分钟 00 秒被 Timer 自然唤醒并执行，然后验证后处理逻辑：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          

        
          
我已经在之前的回复中修复了代码问题并补充了 Cron 端到端测试。现在让我运行测试确认通过，然后更新文档。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -timeout 120s undefined
          

        
            
toolName: check_command_status
            
status: success
          
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -timeout 120s 2>&1 | tee test_output.txt ; Get-Content test_output.txt undefined
          
所有测试全部通过！特别是 `TestScheduler_Cron_NaturalTimerEndToEnd` 成功验证了完整的端到端路径：任务在 `16:37:55` 注册，Cron 计算首次执行时间为 `16:38:00`，Timer 自然等待 4.62 秒后，任务在 `16:38:00.0005` 精确执行（误差仅 0.5ms）。

现在更新文档：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
现在让我更新文档，修正伪代码并添加 Cron 端到端验证策略和异步执行模型说明。我会使用多个 Edit 操作：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          
现在让我读取文档末尾，然后在适当位置添加 Cron 端到端验证策略和异步执行模型的说明：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
现在让我更新文档。首先添加 Cron 端到端测试章节，然后更新异步执行模型说明：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          
让我修复文档结构问题并更新异步执行模型部分：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          
现在让我更新第 8 章设计权衡，详细说明异步执行模型：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有修复和更新已完成！以下是工作内容总结：

## 完成的修复

### 1. 移除永假代码 [scheduler.go#L371-L378](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L371-L378)

从 `runLoop` 的堆顶清理循环中移除了 `|| task.Status == StatusRunning` 条件。因为任务执行流程是 **先 Pop 出堆 → 再标记为 Running**，任务不可能同时存在于堆中和处于 Running 状态，这行是永假死代码。

### 2. 补充 Cron 端到端自然调度测试 [scheduler_test.go#L1168-L1285](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go#L1168-L1285)

新增 `TestScheduler_Cron_NaturalTimerEndToEnd` 测试，**完全不依赖 Reschedule 干预**，覆盖完整调度路径：

**验证策略**：
- `waitForNextMinuteBoundary(t)` 等待当前秒数 ≥ 55，确保只需等待几秒就到下一分钟
- 使用 `* * * * *` Cron 表达式注册任务，验证首次 `ExecuteAt` 对齐到分钟 00 秒
- 完全依赖 `time.NewTimer(waitTime)` 自然等待，不做任何 API 干预
- 任务执行后，验证后处理逻辑正确：状态回到 Pending、第二次 ExecuteAt 也是分钟对齐、元数据正确

**实际运行结果**（误差仅 0.5ms）：
```
Cron end-to-end: registerTime=16:37:55, expectedFirst=16:38:00, waitDuration=4.62s
Cron task executed at 16:38:00.0005104
PASS
```

### 3. 所有测试通过

全部 **41 个测试用例** 均通过，包括新增的 Cron 端到端测试和 3 个内存泄漏专项测试。

### 4. 更新文档 [delaysched.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/delaysched.md)

- **7.6.1 Cron 端到端自然调度测试**：详细说明验证策略、6 步校验流程、实际运行日志
- **7.6.2 伪调度路径与端到端路径的区别**：对比三种 Cron 测试的覆盖范围
- **8.1 异步执行模型详解**：详细说明调度循环永不阻塞、任务独立 goroutine、Stop 两阶段等待的设计
- **8.6 永假代码的清理**：说明 StatusRunning 不可能存在于堆中的原因
- 修正了 4.2 节调度循环伪代码，移除了 StatusRunning 判断