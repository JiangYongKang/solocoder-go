让我先验证当前代码的实际状态，确认修复是否仍然有效：

        
            
toolName: search_by_regex
            
status: success
          
query: job\.Status = JobStatusFailed|job\.Status = JobStatusPending
          

        
            
toolName: search_by_regex
            
status: success
          
query: successResults|deadLetterResults|FailedCount|storeSuccessResult|storeDeadLetterResult
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestJobStatus_RetryIsFailedNotPending|TestCompletedCount_OnlySuccessful|TestFailedStatus_WhileAwaitingRetry
          
代码中的修复已经全部存在。让我运行测试确认所有修改都有效：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/jobqueue/ -v -count=1 -timeout 180s undefined
          
测试已通过（exit code 0）。让我获取更详细的输出来展示：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/jobqueue/ -v -count=1 -timeout 180s 2>&1 | Select-Object -Last 20 undefined
          
所有修复已经存在于代码中，**40 个测试全部通过**（包含 9 个状态流转与混合场景新增测试）。

---

## 修复状态总结

### ✅ 问题 1：重试状态混淆 → 已修复

**修改位置**：[executeJob](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L346)

```go
// 修复后：重试任务状态设为 JobStatusFailed，而非 Pending
job.Status = JobStatusFailed
heap.Push(jq.dq, job)
```

**效果**：
- `JobStatusPending` → 首次等待执行的新任务
- `JobStatusFailed` → 执行失败后等待重试的任务

通过 `GetJobStatus()` 可直接区分。

---

### ✅ 问题 2：CompletedCount 语义错误 → 已修复

**修改位置**：
- 结果存储分离：[storeSuccessResult](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L365-L373) / [storeDeadLetterResult](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L375-L383)
- 统计方法：[CompletedCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L510) / [FailedCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/jobqueue/job_queue.go#L513-L517)

```go
func (jq *JobQueue) CompletedCount() int {
    return len(jq.successResults)     // ★ 仅成功任务
}

func (jq *JobQueue) FailedCount() int {
    return len(jq.deadLetterResults)  // ★ 仅死信任务
}
```

**统计含义**：
- `CompletedCount()` = 成功完成的任务数
- `FailedCount()` = 进入死信队列的任务数
- `CompletedCount() + FailedCount()` = 总已结束任务数

---

### ✅ 问题 3：新增状态流转测试 → 全部通过

| 测试 | 验证点 | 状态 |
|------|--------|------|
| `TestJobStatus_RetryIsFailedNotPending` | 失败后状态为 Failed，而非 Pending | ✅ PASS |
| `TestJobStatus_PendingVsFailedDistinction` | 新任务(Pending)与重试任务(Failed)可区分 | ✅ PASS |
| `TestJobStatus_TransitionFullCycle_RetryThenSuccess` | Pending→Running→Failed→Running→Completed | ✅ PASS |
| `TestJobStatus_TransitionFullCycle_ToDeadLetter` | Pending→Running→Failed→...→DeadLetter | ✅ PASS |
| `TestFailedStatus_WhileAwaitingRetry` | 重试等待期间状态保持 Failed | ✅ PASS |
| `TestCompletedCount_OnlySuccessful` | CompletedCount 仅统计成功 | ✅ PASS |
| `TestCompletedCount_MixedSuccessAndDeadLetter` | 混合场景下计数正确 | ✅ PASS |
| `TestGetResult_ReturnsBothSuccessAndDeadLetter` | 结果查询对两类任务均有效 | ✅ PASS |

---

### ✅ 问题 4：测试运行 → 40/40 全部通过

```
PASS
ok  solocoder-go/internal/jobqueue  10.487s
```

---

### ✅ 问题 5：文档已更新

[docs/jobqueue.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/jobqueue.md) 已更新：
- **状态枚举表**：明确 Pending 与 Failed 的语义边界
- **状态机流程图**：Failed 状态独立显示，不再回流到 Pending
- **状态流转规则表**：6 种迁移路径 + 关键约束说明
- **监控与统计**：分离存储机制说明 + FailedCount 使用示例
- **统计语义**：`CompletedCount + FailedCount = 总已结束任务数`