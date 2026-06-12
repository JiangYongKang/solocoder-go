好的，我来分析并修复这些问题。让我先创建计划：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
让我先确认当前 scheduler.go 的情况，然后进行系统性的修复。从代码来看，Cancel 和 executeTask 的逻辑已经有部分修改，但仍有需要完善的地方，同时需要补充和修正测试。让我开始：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
changes: undefined
          
现在 scheduler.go 的 Cancel 和 executeTask 逻辑已经正确。让我更新 todo 并读取测试文件进行修改：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我重写整个测试文件，修正 Cancel 幂等性、Cron 语义测试，并补充内存泄漏和 Cron 语义验证场景：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          
现在让我更新 todo 并运行测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -timeout 60s undefined
          
有一个测试失败了。问题在于 `AddCronWithID` 的首次执行时间直接用了 `now + delay`，没有经过 Cron 计算。对于 Cron 任务，首次执行也应该遵循 Cron 语义。让我修复 scheduler.go 的 AddCron / AddCronWithID 实现：

        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
          
让我重新读取这部分内容：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
changes: undefined
          
现在 AddCron 首次执行也用 Cron 语义了，但 `TestScheduler_Cron_NextExecuteSemantics` 会因为首次执行也等 Cron 时间（最多 1 分钟）而超时。让我修改这个测试的策略，通过先设置 Cron 验证首次 ExecuteAt，然后手动 Reschedule 到近未来触发执行，再验证第二次的 ExecuteAt：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          
好的，我继续完成之前的工作。让我运行测试确认所有修改都通过：


        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -timeout 60s undefined
          
所有 **40 个测试全部通过**！现在来更新文档：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
现在让我更新文档，重点修正任务取消清理机制、异步并发执行模型和 Cron 语义说明：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
全部修复完成！以下是修改总结：

## 修复内容总览

### 1. Cancel 内存泄漏修复 [scheduler.go#L237-L267](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L237-L267)

**Pending 状态取消**：
```go
case StatusPending:
    heap.Remove(s.heap, task.index)  // 从最小堆移除
    delete(s.tasks, id)              // ★ 立即从 tasks map 删除，防止泄漏
    s.wake()
    return nil
```

**Running 状态取消**：
- 一次性任务 → 返回 `ErrTaskRunning`，不做修改（执行完毕后 executeTask 自动清理）
- 周期性任务 → 返回 `ErrTaskRunning` 但标记 `StatusCancelled`，executeTask 完成时检测到该状态立即 `delete(tasks, id)`

### 2. executeTask 异步并发模型修复 [scheduler.go#L347-L403](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L347-L403) + [scheduler.go#L405-L453](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L405-L453)

**调度循环（runLoop）到期时**：
```go
if waitTime <= 0 {
    heap.Pop(s.heap)
    task.Status = StatusRunning
    s.taskWg.Add(1)
    s.mu.Unlock()
    go s.executeTask(task)  // ★ 独立 goroutine，runLoop 不阻塞
    continue
}
```

**Stop 时**按顺序等待：`s.wg.Wait()`（调度循环）→ `s.taskWg.Wait()`（所有运行中任务）

### 3. Cron 语义修复 [scheduler.go#L196-L218](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go#L196-L218)

首次执行时间不再是 `now+delay`，而是 `nextCronTime(cronExpr, now.Add(delay))`：
```go
firstAt, err := nextCronTime(cronExpr, time.Now().Add(delay))
return s.addRepeat(id, firstAt, fn, RepeatCron, 0, cronExpr)
```

### 4. 单元测试补充 [scheduler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go)

新增/重构的关键测试：

| 测试 | 场景 |
|------|------|
| `TestScheduler_Cancel_AlreadyCancelled_ReturnsNotFound` | Pending 取消后再次 Cancel 返回 ErrTaskNotFound，验证 tasks map 已清理 |
| `TestScheduler_Cron_FirstExecuteByCronSemantics` | 用精确分钟构造 Cron，验证首次 ExecuteAt 分钟匹配、时间区间合理 |
| `TestScheduler_Cron_NextExecuteSemantics` | 先验证首次 minute-aligned → Reschedule 触发执行 → 验证第二次 minute-aligned |
| `TestScheduler_CancelPending_NoMemoryLeak` | 1000 轮 × 100 任务 Add+Cancel，最终 TaskCount=0，侧面验证 map 无残留 |
| `TestScheduler_CompletedOneTime_NoMemoryLeak` | 100 个一次性任务执行完毕后 TaskCount=0 |
| `TestScheduler_RepeatCancel_NoMemoryLeak` | 50 个间隔任务至少执行 1 次后 Cancel，300ms 后 TaskCount=0 |

### 5. 测试结果

```
ok  solocoder-go/internal/delaysched  2.701s
```

**40 个测试全部通过**（含 22 个子测试）。

### 6. 文档更新 [delaysched.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/delaysched.md)

新增/更新的章节：
- **4.4 任务执行模型**：详细说明异步 goroutine 模型、taskWg 机制、executeTask 的清理流程
- **4.5 任务取消清理机制**：分 Pending / Running / 执行完毕三种子节，说明分级清理策略
- **5.5 Cron 表达式格式与支持范围**：完整列出支持语法、各字段取值范围、不支持的特性、首次/后续执行时间计算规则
- **6.5 利用异步执行模型处理长耗时任务**：新使用示例
- **第 7 章**：测试覆盖完整清单