toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\orchestrator.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go && go test ./internal/orchestrator/ -v 2>&1 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/orchestrator/ -v 2>&1 undefined
          
Now I have a clear picture. The existing code has a critical bug — when tasks are skipped via `skipDownstream`, they aren't counted toward completion, causing the scheduler to hang. The test file is also truncated. Let me rewrite the orchestrator with a correct design and complete the tests and docs.
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
Let me check the coding conventions in another package:
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          
Now let me write the complete fixed orchestrator:
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          
changes: undefined
          
Now let me write the comprehensive test file:
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/orchestrator/ -v -timeout 60s 2>&1 undefined
          
I see three issues to fix:

1. **Deadlock in `RetryTask`**: `collectDownstream` tries to re-acquire the mutex that `RetryTask` already holds
2. **Cycle detection test**: The test only modifies Dependencies but not Successors, so DFS on Successors misses it
3. **Duration check**: Fast tasks may report 0 duration
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          
Let me fix all three issues. First, fix the deadlock in `collectDownstream` — I need a lock-free version for internal use:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          
changes: undefined
          
Now update the `RetryTask` call to use the locked version:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          
changes: undefined
          
Now fix the cycle detection test to properly create a cycle by also updating the Successors list:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          
changes: undefined
          
Now fix the duration check for very fast tasks:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          
changes: undefined
          
Now let's run the tests again:
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/orchestrator/ -v -timeout 120s 2>&1 undefined
          
One test failing — `TestRetryTask_SkippedTask`. The issue is that "b" was skipped (never executed) in the first run, so `bAttempts` never incremented. When retry runs "b", it's the first actual attempt. Let me fix the test logic:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/orchestrator/ -v -timeout 120s 2>&1 undefined
          
All 34 tests pass. Now let me write the documentation:
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\orchestrator.md
          
changes: undefined
          
Now let me do a final verification run to make sure everything still passes:
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/orchestrator/ -v -timeout 120s 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
All tasks complete. Here's a summary of what was done:

## 完成内容

### 1. 重写 [orchestrator.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/orchestrator/orchestrator.go)

**修复的关键 Bug**：原有代码使用 `skipDownstream` BFS 遍历来跳过失败任务的下游，但跳过的任务不会进入 `completed` 通道，导致 `completedCount` 永远达不到 `pendingCount`，调度器会永久挂起。

**新设计**：采用 `shouldSkip` 检查替代 `skipDownstream`：
- 当任务的所有依赖完成（`readyCount=0`）时，检查是否有任何依赖处于 Failed/Timeout/Skipped 状态
- 如果应跳过，直接标记为 Skipped 并发送到 `completed` 通道，确保计数正确
- 错误传播自然发生在依赖解析阶段，无需额外的 BFS 遍历
- 提取了 `runScheduler` 方法，供 `Run` 和 `RetryTask` 共用

**修复的死锁**：`RetryTask` 在持有 `o.mu` 锁时调用 `collectDownstream`（后者也尝试获取 `o.mu`），改为 `collectDownstreamLocked`（假设调用方已持有锁）。

### 2. 编写 [orchestrator_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/orchestrator/orchestrator_test.go)

34 个测试用例，覆盖：

| 类别 | 测试 |
|------|------|
| **正常流程** | 空编排器、单任务、线性 DAG、菱形 DAG、并行 DAG、多个根节点、深度 DAG |
| **超时控制** | 任务超时、超时+重试、零超时、超时分支与成功分支共存 |
| **错误传播** | 错误传播、独立分支隔离、级联失败、多个上游失败 |
| **局部重试** | 重试成功、保留上游结果、中间 DAG 重试、超时后重试、跳过任务重试 |
| **边界条件** | 并发执行验证、大规模并行图、panic 恢复、context 取消、重复运行 |
| **防御性** | 返回副本、未找到任务、不可重试、运行中添加任务 |

### 3. 编写 [orchestrator.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/orchestrator.md)

包含模块功能说明、核心结构体职责、DAG 调度流程图、错误传播流程、局部重试流程、超时控制规则，以及 5 个使用示例。