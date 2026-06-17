我来分析并修复这两个问题。首先让我查看当前的实现：

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
现在让我分析并修复这两个问题。首先更新 todo 状态并开始修复：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在我来修复这两个问题。首先修复 `canSubmit` 方法，然后添加信号量接口。关键修改点：

1. **`canSubmit` 逻辑修复**：队列满时（`len(queue) >= maxQueueSize`）一律拒绝，除非是 `maxQueueSize=0` 且有空闲 worker 的特殊情况
2. **添加信号量接口**：`Acquire(timeout)` 和 `Release()` 方法，与 worker 池共享 `active` 计数器
3. **修改 worker 逻辑**：worker 执行任务前也需要检查信号量是否可用
























        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead.go
          
changes: undefined
          
现在更新 todo 状态，然后添加信号量相关的单元测试：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先读取测试文件，然后添加信号量相关的测试：


























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
现在让我在测试文件末尾添加信号量相关的测试用例，以及验证 `canSubmit` 修复的测试：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在让我们更新 todo 并运行测试验证：





























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test -v -timeout 60s .\internal\bulkhead\ undefined
          
我看到问题了。测试 `TestSubmit_QueueFull_SemaphoreSlotsAvailable` 有两个问题：
1. 第二个任务被 worker 取走执行了（因为有可用信号量槽位），队列实际为空
2. 测试失败后 `defer b.Close()` 会等待 worker 退出导致死锁

让我修复这个测试：































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
让我也检查并修复 `TestSemaphore_WithWorkerPool` 测试中的潜在死锁问题：
































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
让我也修复 `TestSemaphore_WithWorkerPool` 以及其他有类似潜在死锁的测试：

































        
            
toolName: search_by_regex
            
status: success
          
query: defer b\.Close\(\)
          
让我修复 `TestSemaphore_WithWorkerPool` 测试中的潜在死锁：




































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
现在让我也修复 `TestCanSubmit_QueueFullWithIdleWorkers` 和 `TestResizeQueue_Shrink_RejectsNewTasks` 测试中的潜在死锁：


































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
让我也检查并修复 `TestResize_Queue_Shrink` 测试：



































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          
让我也检查并修复 `TestResize_Concurrency_ShrinkWithActiveTasks` 和 `TestResize_Concurrency_Shrink`：





































        
            
toolName: search_by_regex
            
status: success
          
query: func TestResize_Concurrency_ShrinkWithActiveTasks
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\bulkhead\bulkhead_test.go
          
changes: undefined
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestResize_Concurrency_Shrink\b
          
这个测试没有 block channel，应该是安全的。现在让我们运行测试看看结果：

































        
            
toolName: run_command
            
status: success
          
command: go test -v -timeout 60s .\internal\bulkhead\ undefined
          
太好了！所有 38 个 bulkhead 测试全部通过！现在让我更新 todo 并更新文档以包含信号量相关内容：


































        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我更新 `docs/bulkhead.md` 文档，添加信号量相关内容：



































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
让我更新文档，添加信号量相关内容。首先更新功能清单和结构体：



































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新 Bulkhead 结构体定义，添加 `semHolders` 字段：





































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在更新主要职责说明，添加信号量相关内容：






































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我找到预定义错误部分并添加 `ErrNotAcquired`：









































        
            
toolName: search_by_regex
            
status: success
          
query: 预定义错误
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新"舱壁隔离与信号量限流的协作方式"部分，添加信号量接口的说明：










































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
现在让我更新协作机制说明，添加信号量接口的内容：

```go
### 4.2 协作机制说明

**舱壁隔离**与**信号量限流**是两个互补的机制，共同保障系统稳定性：

1. **舱壁隔离（空间维度）**：通过 Registry 将系统划分为多个独立的 Bulkhead，每个 Bulkhead 拥有独立的资源配额（协程池 + 队列）。一个 Bulkhead 的任务积压或故障不会影响其他 Bulkhead 的正常运行。

2. **信号量限流（时间维度）**：在单个 Bulkhead 内部，通过信号量机制控制并发执行数量。信号量是底层的并发控制原语，支持两种使用模式：
   - **Worker 池模式**：通过 `Submit()` 提交任务，由内部 worker 协程执行
   - **直接调用模式**：通过 `Acquire()` / `Release()` 在调用方自己的 goroutine 中执行受保护代码
   
   两种模式共享同一个并发上限（`active` 计数器），总并发数不超过 `maxConcurrency`。

两者的协作关系：
- **舱壁隔离**是"粗粒度"的资源划分，解决"故障蔓延"问题
- **信号量限流**是"细粒度"的并发控制，解决"单个舱室内过载"问题
- 信号量是核心控制机制，worker 池模式是其上层封装
- 两者结合形成纵深防御体系，既防止故障跨舱扩散，又防止单个舱室自身过载

### 4.2.1 信号量的两种使用模式

```
信号量计数器 active (maxConcurrency = 5)

┌───────────────────────────────────────────────────────┐
│  总并发槽位: 5                                 │
│                                               │
│  ┌──────────────────────────────────────────┐        │
│  │  Worker 池模式 (Submit)        │        │
│  │  - worker 执行的任务占用槽位: 2   │        │
│  │  (active += 包含这部分)       │        │
│  └──────────────────────────────────────────┘        │
│                                               │
│  ┌──────────────────────────────────────────┐        │
│  │  直接调用模式 (Acquire/Release)    │        │
│  │  外部调用方持有槽位: 2          │        │
│  │  (semHolders 跟踪这部分)       │        │
│  └──────────────────────────────────────────┘        │
│                                               │
│  剩余可用槽位: 1                                 │
└───────────────────────────────────────────────────────┘
```

**模式一：Worker 池模式（Submit）**
- 调用方通过 `Submit(task)` 提交任务
- 任务进入队列等待，由内部 worker 协程执行
- 每个 worker 执行任务前获取信号量槽位（`active++）
- 任务执行完成后释放信号量槽位（`active--`）
- 适用于批量任务处理、异步任务执行等场景

**模式二：直接调用模式（Acquire/Release）**
- 调用方通过 `Acquire(timeout)` 获取信号量槽位
- 在自己的 goroutine 中执行受保护代码
- 执行完成后通过 `Release()` 释放槽位
- `semHolders` 计数器跟踪通过 `Acquire` 持有的槽位数
- 适用于需要精确控制并发的场景（如数据库操作、外部服务调用等）

**共享并发配额：
- `active` = worker 执行的任务数 + `semHolders`
- `active` <= `maxConcurrency` 始终成立
- 两种模式相互影响，一种模式占用的槽位会影响另一种模式

### 4.3 任务提交流程（canSubmit 逻辑

```
canSubmit()
    │
    ├─ maxQueueSize == 0 ?
    │   ├─ 是：需要 idleWorkers > 0 && active < maxConcurrency
    │   └─ 否：需要 len(taskQueue) < maxQueueSize
    │
    └─ 返回结果
```

**关键修复：队列满时（`len(queue) >= maxQueueSize）一律拒绝新任务，即使有空闲 worker。

让我更新文档。</think_never_used_51bce0c785ca2f68081bfa7d91973934>现在让我更新协作机制说明，添加信号量两种使用模式的说明，同时修复 `canSubmit` 逻辑说明：















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新任务提交流程中的 `canSubmit` 逻辑说明：



















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我添加信号量 `Acquire` 和 `Release` 的流程说明，以及使用示例。首先让我找到使用示例部分：





















































        
            
toolName: search_by_regex
            
status: success
          
query: ## 7\. 使用示例
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
现在让我在使用示例部分添加信号量 `Acquire`/`Release` 的使用示例：

















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新文档末尾的章节编号（因为添加了新的第 8 章）：





















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
现在让我更新章节编号：























































        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
          
让我重新读取文件末尾，获取准确的内容：




















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
让我分两次编辑来修复章节编号：






















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
让我检查一下当前的章节编号情况：

























































        
            
toolName: search_by_regex
            
status: success
          
query: ^## 
          
让我检查一下文档结构，看看"核心 API 说明"章节在哪里：





















































        
            
toolName: search_by_regex
            
status: success
          
query: 核心 API 说明
          
让我重新读取文档相关部分，看看实际内容：



























































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
我发现文档内容有些缺失。让我重新修复章节编号并添加缺失的内容：


















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先修复章节编号，然后添加缺失的内容：



















































        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
          
让我重新读取获取准确内容：





















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我在7.5监控之后添加信号量使用示例和核心API说明：






















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新监控示例，添加信号量计数：























































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
我发现刚才的编辑覆盖了之前的内容。让我重新添加信号量使用示例和核心API说明：





















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
现在让我更新剩余的章节编号，并在关键设计决策中添加信号量相关内容：























































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
现在让我更新剩余的章节编号和内容：

























































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
让我检查一下当前的章节编号是否正确：

























































        
            
toolName: search_by_regex
            
status: success
          
query: ^(## |### )
          
我发现之前添加的内容又丢失了。让我重新在正确的位置添加信号量相关内容。首先让我看看7.5后面的内容：





















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
让我先修复当前的章节编号问题，然后添加缺失的内容：

























































        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
          
让我读取准确的内容，然后一次性添加缺失的内容：




















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
让我一次性在正确的位置添加所有缺失的内容并修复章节编号：





















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
changes: undefined
          
完美！现在让我验证一下章节编号是否正确：

























































        
            
toolName: search_by_regex
            
status: success
          
query: ^(## |### )
          
让我检查一下文件的实际内容：

























































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
文件内容是正确的！让我再确认一下后面的章节：


























































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\bulkhead.md
          
完美！文档内容已正确更新。现在让我最后运行一次测试，确保所有功能都正常工作：

























































        
            
toolName: run_command
            
status: success
          
command: go test -v -timeout 60s ./internal/bulkhead/ undefined
          
完美！所有 38 个测试全部通过。现在让我更新 todo 列表：


























































        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 修复完成总结

所有问题已成功修复，以下是详细的修复内容：

### 1. 修复 `canSubmit` 方法逻辑 ([bulkhead.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go))

**修复前**的问题：`idleWorkers > 0` 时直接返回 `true`，绕过了队列容量限制，导致队列已满但有空闲 worker 时新任务仍被追加到队列中。

**修复后**的逻辑：
```go
func (b *Bulkhead) canSubmit() bool {
    if b.maxQueueSize == 0 {
        return b.idleWorkers > 0 && b.active < b.maxConcurrency
    }
    return len(b.taskQueue) < b.maxQueueSize
}
```

- 队列满时（`len(queue) >= maxQueueSize`）一律拒绝新任务
- 仅在 `maxQueueSize == 0` 的无缓冲场景下，才检查空闲 worker 和并发数

### 2. 补充信号量接口 ([bulkhead.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead.go))

新增三个方法，支持调用方在自己的 goroutine 中控制并发：

- **`Acquire(timeout time.Duration) error`**：带超时获取并发槽位
- **`Release() error`**：释放通过 `Acquire` 获取的槽位
- **`SemaphoreCount() int`**：查询当前信号量持有数

**关键设计**：`active` 计数器统一管理 worker 执行的任务和外部调用方持有的信号量，两种模式共享同一个并发上限（`maxConcurrency`）。

### 3. 单元测试验证 ([bulkhead_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/bulkhead/bulkhead_test.go))

新增 12 个信号量相关测试，总测试数从 26 个增加到 **38 个**，覆盖：
- 信号量获取/释放的基本功能
- 超时等待和非阻塞模式
- 信号量与 worker 池共享配额
- 并发场景下的正确性
- 队列满时即使有空闲 worker 也拒绝新任务
- 队列缩容时正确拒绝新任务

### 4. 测试结果

```
PASS
ok  solocoder-go/internal/bulkhead  1.224s
```
**38 个测试全部通过** ✅

### 5. 文档更新 ([bulkhead.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/bulkhead.md))

- 添加信号量的两种使用模式说明（Worker 池模式 / 直接调用模式）
- 添加共享并发配额的架构说明
- 添加 4 个信号量使用示例（基础使用、非阻塞、混合使用、HTTP 限流）
- 添加核心 API 说明章节
- 更新关键设计决策，添加信号量与 Worker 池共享配额说明