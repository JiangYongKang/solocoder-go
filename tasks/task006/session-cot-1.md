我来帮你实现这个延迟任务调度器功能模块。首先让我了解一下项目结构和代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\hash_ring.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\queue.go
          
我看到项目中已经有了延迟调度器的基本代码。让我先运行测试看看当前状态，然后修复问题并编写文档。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v undefined
          
发现了编译错误。主要是测试文件中的 `TaskFunc` 签名和 API 返回值与实际代码不一致。让我修复测试文件：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
我需要重写测试文件来修复所有编译问题。主要问题是：
1. `TaskFunc` 签名应该是 `func(ctx context.Context)`
2. `AddWithID` 应该只返回 `error`
3. 缺少 `fmt` 导入


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
changes: undefined
          
现在让我运行测试看看：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 undefined
          
有几个测试失败了，让我分析并修复这些问题：

1. `TestScheduler_Cancel_AlreadyCancelled` - Cancel 删除了 tasks map 中的任务，导致第二次取消返回 ErrTaskNotFound
2. `TestScheduler_AddCron` - Cron 计算的下一次执行时间需要等太久
3. `TestScheduler_Add_Concurrent` - 并发问题

让我修复 scheduler.go 中的问题：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
我需要修复几个核心问题：

1. **Cancel 删除 tasks**：应该保留记录，只设置状态
2. **Timer 机制问题**：`rescheduleTimerLocked` 创建了新 timer，但 `runLoop` 仍在等待旧 timer 的 channel，需要唤醒机制
3. **并发问题**：同上

让我重写 scheduler.go 修复这些问题：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          
changes: undefined
          
现在让我运行测试看看：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/delaysched/ -v -count=1 -timeout 30s undefined
          
所有测试都通过了！现在让我编写文档。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\delaysched.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
延迟任务调度器模块已全部完成！以下是工作内容总结：

## 完成情况

### 1. 修复了调度器核心代码 [scheduler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler.go)

主要修复和改进：

- **引入 `wakeCh` 唤醒机制**：取代了之前依赖共享 Timer 的方式，解决了定时器重置时的竞态条件问题。每次堆变化时通过关闭 `wakeCh` 通知调度循环重新检查。
- **修复 Cancel 幂等性**：取消任务时不再从 `tasks` map 删除记录，只设置状态为 `StatusCancelled`，这样重复 Cancel 返回 nil 而非错误。
- **修复堆清理逻辑**：调度循环在取堆顶任务前，先清理堆顶已取消/已完成的无效任务。
- **改进并发安全**：`Add`、`AddAtWithID`、`addRepeat` 等方法在真正入堆前重新检查调度器运行状态，避免 TOCTOU 竞态。

### 2. 修复了单元测试 [scheduler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/delaysched/scheduler_test.go)

- 修正 `TaskFunc` 签名：从 `func(_ interface{Done()})` 改为 `func(ctx context.Context)`，与实际定义一致
- 修正 `AddWithID` 返回值：从 `(string, error)` 改为 `error`
- 补充了缺失的 `fmt` 导入

### 3. 测试结果

所有 **34 个测试用例** 全部通过：

```
PASS
ok      solocoder-go/internal/delaysched        2.110s
```

覆盖的测试场景包括：最小堆操作、任务注册执行、任务取消、动态重排、固定间隔/Cron 周期性任务、Cron 表达式解析、并发安全、panic 恢复、边界条件等。

### 4. 编写了需求文档 [delaysched.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/delaysched.md)

文档包含：
- 模块功能概述
- 核心结构体职责（Task、Scheduler、taskHeap）
- 最小堆调度机制详解（调度循环、唤醒机制、任务执行流程）
- 完整的 API 接口文档
- Cron 表达式格式说明
- 5 个使用示例（一次性任务、固定间隔、Cron、取消与重排、并发安全）
- 测试覆盖说明