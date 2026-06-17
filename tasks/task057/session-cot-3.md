toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我对两个问题有了清晰的理解，让我一起设计解决方案：

**问题 1 分析**：`Acquire` 超时后返回 `FullError`，其中包含 `QueueLength` 和 `MaxQueueSize`，但信号量槽位耗尽跟队列无关，会误报。需要新增 `SemaphoreFullError` 类型。

**问题 2 分析**：当外部调用方通过 `Acquire` 占满所有 `active` 槽位时，worker 的 `active >= maxConcurrency` 条件为 true，导致 worker 无法取队列任务执行。需要将 worker 和信号量的并发配额分离。

**设计方案**：
1. 新增 `SemaphoreFullError` 错误类型，只包含信号量相关字段（`Name`、`ActiveCount`、`MaxConcurrency`、`SemaphoreHolders`）
2. 将 `active` 拆分为两个独立计数器：
   - `workerActive`：worker 正在执行的任务数
   - `semHolders`：信号量持有者数（已有）
   - 移除 `active`，`ActiveCount()` 返回 `workerActive + semHolders`
3. worker 取任务的条件改为 `workerActive < workerCnt`（每个 worker 最多执行一个任务），不再受信号量影响
4. `Acquire` 的并发上限保持为 `maxConcurrency`（即 `workerActive + semHolders < maxConcurrency`）
5. `canSubmit` 逻辑保持队列容量限制不变

让我实现这个方案：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我更新测试文件。核心变更：
1. `active` 字段已替换为 `workerActive`，`ActiveCount()` 现在返回 `workerActive + semHolders`
2. `Acquire` 失败返回 `*SemaphoreFullError` 而不是 `*FullError`
3. Worker 不再受信号量影响（`workerActive < workerCnt` 替代 `active >= maxConcurrency`）
4. 需要新增测试：验证信号量占满后 worker 仍可执行队列任务
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在让我运行测试来验证：
        
            
toolName: run_command
            
status: success
          
command: go test -v -timeout 60s ./internal/bulkhead/ undefined
          
所有 41 个测试全部通过！现在让我更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
现在我需要更新文档中的关键部分。让我逐个修复：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新 FullError 部分，添加 SemaphoreFullError：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新预定义错误表，添加 ErrNotAcquired：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新协作机制说明，将"共享并发配额"改为"资源隔离"：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新 canSubmit 逻辑描述：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新 Worker 执行流程：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我找到"信号量的两种使用模式"部分并更新架构图：
        
            
toolName: search_by_regex
            
status: success
          
query: 4\.2\.1 信号量的两种使用模式
          

        
            
toolName: search_by_regex
            
status: success
          
query: 信号量的两种使用模式
          

        
            
toolName: search_by_regex
            
status: success
          
query: 共享并发配额
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我找到"核心 API 说明"部分并更新：
        
            
toolName: search_by_regex
            
status: success
          
query: 核心 API 说明
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新关键设计决策第 6 条：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我找到"信号量的两种使用模式"和"共享并发配额"的架构图并更新：
        
            
toolName: search_by_regex
            
status: success
          
query: 总并发槽位
          

        
            
toolName: search_by_regex
            
status: success
          
query: 信号量计数器
          
之前的对话历史中的架构图内容似乎没有在文档中保留。让我检查文档中信号量模式部分是否存在：
        
            
toolName: search_by_regex
            
status: success
          
query: Worker 池模式
          
好的，架构图没有被包含在文档中。让我继续更新使用示例部分，更新 Acquire 的错误类型：
        
            
toolName: search_by_regex
            
status: success
          
query: 7\.6 信号量使用
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新 HTTP 限流示例中的错误类型：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
文档更新基本完成。让我最后确认一下整个文档的章节编号和内容是否一致：
        
            
toolName: search_by_regex
            
status: success
          
query: ^## 
          
文档结构完整。让我完成最后的 todo 更新：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## 修复完成总结

所有 41 个测试全部通过 ✅。以下是本轮修复的核心变更：

### 问题1：Acquire 错误类型误报

**修复前**：`Acquire` 失败返回 `*FullError`，包含 `QueueLength` 和 `MaxQueueSize` 字段，信号量槽位耗尽时误报为队列问题（如 `queue=0/5`）。

**修复后**：新增 [SemaphoreFullError](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go#L37-L49) 类型，仅包含信号量相关字段：
```go
type SemaphoreFullError struct {
    Name           string
    ActiveCount    int
    MaxConcurrency int
    SemHolders     int
}
```
- `FullError`：Submit/TrySubmit 队列满时返回（含队列信息）
- `SemaphoreFullError`：Acquire 槽位耗尽/超时时返回（不含队列信息）

### 问题2：Worker 池冻结问题

**修复前**：`active` 计数器统一管理 worker 和信号量，当信号量占满所有 `maxConcurrency` 槽位后，worker 的 `active >= maxConcurrency` 条件为 true，无法取队列任务执行。

**修复后**：将 `active` 拆分为两个独立计数器：
- [workerActive](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go#L66)：worker 正在执行的任务数
- [semHolders](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go#L67)：信号量持有者数

**关键约束变化**：
- Worker 取任务条件：`workerActive < workerCnt`（不受信号量影响）
- Acquire 获取条件：`workerActive + semHolders < maxConcurrency`（共享总上限）
- 信号量占满所有槽位 → worker 仍可执行队列任务，保障公平性

### 新增测试（41 个全部通过）

| 测试 | 验证内容 |
|------|---------|
| `TestSemaphoreFullError_NoQueueFields` | SemaphoreFullError 不含 queue 字段 |
| `TestSemaphoreFullError_ErrorString` | 错误信息包含 semaphoreHolders、不含 queue |
| `TestWorkerActiveCount` | WorkerActiveCount 不受 Acquire 影响 |
| `TestSemaphore_OccupyAllSlots_WorkersStillProcess` | 信号量占满后 worker 仍可执行队列任务 |
| `TestAcquire_DoesNotBlockWorkers` | Acquire 不阻塞 worker 执行 |