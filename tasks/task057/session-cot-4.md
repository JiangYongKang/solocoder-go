toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
现在我清楚了两个问题的具体内容。让我逐个修复。

**问题 1**：`SemaphoreFullError` 需要添加 `WorkerActive` 字段，且 `Error()` 方法使用中立语义（"concurrency limit reached" 而非 "semaphore full"），让调用方能区分是 worker 占槽还是信号量持有者占槽导致的拒收。

**问题 2**：测试中 `WorkerActiveCount` 断言需恢复为 `ActiveCount` 断言，以维持对 `semHolders` 的验证能力。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
changes: undefined
          
现在更新 `Acquire` 方法中构造 `SemaphoreFullError` 的地方，添加 `WorkerActive` 字段：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在修复测试文件。首先恢复 `TestResize_Concurrency_ShrinkWithActiveTasks` 中的断言：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在修复 `TestCanSubmit_QueueFullWithIdleWorkers`，恢复 `ActiveCount` 断言：
        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
          
有两处匹配，让我用更多上下文来定位：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在更新 `TestSemaphore_WithWorkerPool` 中的 `WorkerActiveCount` 断言：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在更新 `TestWorkerActiveCount` 中的断言，也添加 `ActiveCount` 验证：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在更新 `TestSemaphoreFullError_ErrorString` 测试，使用新的字段和消息格式：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在更新 `TestAcquire_NoWait_Full` 测试，验证新的 `WorkerActive` 字段：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在更新 `TestSemaphoreFullError_NoQueueFields` 测试，验证新的字段和中立语义：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在还需要添加一个新测试，验证当 worker 占槽导致 Acquire 失败时，错误信息中的 `WorkerActive` 能正确反映这种情况。让我在 `TestSemaphoreFullError_NoQueueFields` 之后添加：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
有个格式问题，`TestWorkerActiveCount` 的函数声明被吞掉了。让我修复：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在运行测试验证：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test -v -timeout 60s ./internal/bulkhead/ undefined
          
所有 43 个测试全部通过！
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## 修复完成总结

所有 43 个测试全部通过 ✅。以下是本轮修复的核心变更：

### 问题1：SemaphoreFullError 错误信息中立化

**修复前**：`Error()` 返回 `"semaphore full: active=5/5, semaphoreHolders=2"`——当 worker 占了 3 个槽位、信号量只占了 2 个时，"semaphore full" 将 worker 占槽导致的拒收误报为信号量耗尽。

**修复后**：[SemaphoreFullError](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go#L37-L50) 新增 `WorkerActive` 字段，`Error()` 使用中立语义：

```go
type SemaphoreFullError struct {
    Name           string
    ActiveCount    int
    MaxConcurrency int
    WorkerActive   int  // 新增：worker 正在执行的任务数
    SemHolders     int
}

// 输出示例：
// bulkhead 'svc' concurrency limit reached: active=5/5 (workers=3, semaphoreHolders=2)
// 调用方可清晰看出：是 workers 占了 3 个槽，而非信号量占满
```

### 问题2：恢复 ActiveCount 断言

以下测试中的 `WorkerActiveCount` 断言恢复为 `ActiveCount` 断言，确保 `semHolders` 未正确归零也能被检测到：

| 测试 | 修改 |
|------|------|
| [TestResize_Concurrency_ShrinkWithActiveTasks](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L499) | `WorkerActiveCount != 3` → `ActiveCount != 3` |
| [TestResize_Concurrency_ShrinkWithActiveTasks](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L511) | `WorkerActiveCount != 0` → `ActiveCount != 0` |
| [TestCanSubmit_QueueFullWithIdleWorkers](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L1418) | `WorkerActiveCount != 1` → `ActiveCount != 1` |
| [TestSemaphore_WithWorkerPool](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L1264) | 移除冗余的 `WorkerActiveCount` 断言 |
| [TestWorkerActiveCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L1811) | 最终断言改为 `ActiveCount != 0` |

### 新增测试

[TestAcquire_FailsDueToWorkers_SemaphoreFullError](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go#L1778)：验证当 worker 占满所有并发槽位导致 `Acquire` 失败时，`SemaphoreFullError` 中的 `WorkerActive=2, SemHolders=0` 能正确反映是 worker 而非信号量导致的拒收。