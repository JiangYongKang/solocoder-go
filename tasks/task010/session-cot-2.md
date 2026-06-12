让我创建修复计划并逐一解决这些问题：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          
让我查看更多代码以确认当前状态：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          
看起来代码已经有部分修改。让我先运行测试查看当前状态，然后确保所有修复都正确：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/jobqueue/ -v -timeout 180s undefined
          
测试全部通过！让我更新进度，然后检查并更新文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我更新文档，反映修复后的状态机流转规则和统计计数语义：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
让我查看文档的监控与统计部分：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
现在更新文档，反映修复后的状态机和统计计数语义：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
现在更新生命周期流程图，反映修复后的状态流转：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
现在更新生命周期阶段说明和监控与统计部分：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
ASCII 图有格式问题，让我修复它，然后更新监控与统计部分：













        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
让我重新更新整个流程图和监控统计部分：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
现在更新监控与统计部分，加入结果存储分离和 FailedCount 的说明：

















        
            
toolName: edit_file_search_replace
            
status: interrupted
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
          
现在更新监控与统计部分，反映分离的结果存储和新的 FailedCount 方法：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
现在让我再次运行所有测试，确保一切正常：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/jobqueue/ -v -timeout 180s undefined
          
所有 40 个测试全部通过！现在完成最后的更新：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
修复工作全部完成！以下是修复内容总结：

## 修复内容

### 问题 1：任务状态混淆（已修复）

**修复位置**：[executeJob](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L311-L349)

**原问题**：任务执行失败但未超过最大重试次数时，状态被错误地设为 `JobStatusPending`，导致无法区分"首次等待的新任务"和"失败后等待重试的任务"。

**修复后**：重试任务的状态设为 `JobStatusFailed`，与 `JobStatusPending` 严格区分：

| 状态 | 语义 |
|------|------|
| `JobStatusPending` | **首次等待执行**：新任务入队后从未执行过 |
| `JobStatusFailed` | **执行失败，等待重试**：已失败过，正在等待下一次重试 |

### 问题 2：CompletedCount 语义错误（已修复）

**修复位置**：
- 结果存储分离：[storeSuccessResult](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L365-L373) 和 [storeDeadLetterResult](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L375-L383)
- 统计方法：[CompletedCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L507-L511) 和新增 [FailedCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L513-L517)

**原问题**：成功任务和死信任务都写入同一个 `results` map，导致 `CompletedCount()` 返回的是两者之和。

**修复后**：
- `successResults`：仅存成功任务（状态为 `Completed`）
- `deadLetterResults`：仅存死信任务（状态为 `DeadLetter`）
- `CompletedCount()` → `len(successResults)`（仅成功）
- `FailedCount()` → `len(deadLetterResults)`（仅死信）
- `GetResult()` / `WaitForResult()` → 同时查找两个 map，兼容原有 API

### 新增单元测试（9 个）

覆盖状态流转和混合场景：

| 测试 | 验证点 |
|------|--------|
| `TestJobStatus_RetryIsFailedNotPending` | 失败重试任务状态为 Failed，而非 Pending |
| `TestJobStatus_PendingVsFailedDistinction` | 新任务和重试任务可通过状态区分 |
| `TestJobStatus_TransitionFullCycle_RetryThenSuccess` | 完整流转：Pending→Running→Failed→Running→Completed |
| `TestJobStatus_TransitionFullCycle_ToDeadLetter` | 完整流转：Pending→Running→Failed→...→DeadLetter |
| `TestFailedStatus_WhileAwaitingRetry` | 重试等待期间状态保持 Failed |
| `TestCompletedCount_OnlySuccessful` | CompletedCount 仅统计成功任务 |
| `TestCompletedCount_MixedSuccessAndDeadLetter` | 混合场景下计数正确 |
| `TestGetResult_ReturnsBothSuccessAndDeadLetter` | 结果查询对成功和死信均有效 |

### 文档更新（[docs/jobqueue.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/jobqueue.md)）

- **JobStatus 说明**：明确 Pending 与 Failed 的语义边界
- **状态机流程图**：更新为修复后的正确流转，Failed 状态单独显示
- **状态流转规则总结表**：6 种状态迁移路径，含关键约束说明
- **监控与统计**：说明分离存储机制，补充 `FailedCount()` 使用示例
- **统计计数语义**：`CompletedCount + FailedCount = 总已结束任务数`

### 测试结果

```
PASS: 40 / 40 tests passed
ok   solocoder-go/internal/jobqueue
```