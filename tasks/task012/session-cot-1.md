我来帮你实现死信队列处理器功能模块。首先让我探索一下当前项目的结构和代码风格。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          
我看到 `internal/deadletter/` 目录已经存在。让我先查看现有文件内容，以及参考其他模块的代码风格。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job.go
          
让我先运行现有的测试，了解当前代码状态。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/deadletter/ -v undefined
          
我看到了多个测试失败的问题。让我分析核心 bug：

1. **`runLoop` 的等待时间计算错误**：只跟踪已到期消息的最早时间，没有跟踪未来消息的最早时间，导致未来消息等待 `time.Hour` 才被处理
2. **`processMessage` 的 panic recover 不返回错误**：导致 panic 的消息被误判为成功
3. **并发测试设计问题**：无限循环查询导致锁竞争

让我修复这些问题。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我修复 `deadletter.go` 中的核心 bug：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在修复 `processMessage` 中的 panic recover 问题：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在让我运行测试，看看还有什么问题需要修复：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/deadletter/ -v -timeout 60s undefined
          
所有测试都通过了！现在让我编写需求文档。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
现在让我编写 deadletter.md 文档：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\deadletter.md
          
changes: undefined
          
最后让我再验证一次所有测试都通过，并运行 Go 诊断检查：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/deadletter/ -v -timeout 60s undefined
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有测试通过，诊断无错误。让我更新任务状态：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务已完成！以下是工作总结：

## 完成情况

### 1. 代码修复与完善
修复了 [deadletter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/deadletter/deadletter.go) 中的两个核心 bug：

- **runLoop 等待时间计算错误**：原来的代码只跟踪已到期消息的最早重试时间，导致未到期的消息需要等待 `time.Hour` 才会被调度。修改为跟踪所有 `StatusPending` 消息的最早 `NextRetryAt`，确保消息在正确的时间点被调度处理。
- **processMessage panic recover 未返回错误**：原来的 `recover()` 调用丢弃了 panic 信息，导致 handler panic 的消息被误判为处理成功。修改为捕获 panic 并转为 `fmt.Errorf("handler panic: %v", r)` 返回，使 panic 消息能够正确地进入重试流程。

### 2. 单元测试
[deadletter_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/deadletter/deadletter_test.go) 包含 **34 个测试用例**，全部通过 ✓，覆盖：
- 配置验证（6 个子用例）
- 生命周期管理（启动、停止、幂等性）
- 失败消息转移（含默认重试次数、零重试次数）
- 延迟策略（固定延迟、指数递增、上限裁剪）
- 重试处理（首次成功、多次重试后成功、永久失败、Handler Panic）
- 手动重试（成功、不存在、永久失败）
- 告警机制（超过阈值、不重复触发、处理后重置、无回调、零阈值）
- 查询与管理（按 ID/状态查询、移除、清理永久失败）
- 并发测试（高并发入队、并发处理与查询）
- 完整流程集成测试（消息恢复、永久失败）
- 优雅关闭（等待任务完成）

### 3. 模块需求文档
编写了 [docs/deadletter.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/deadletter.md)，包含：
- 模块概述与主要特性
- 7 个核心结构体的职责说明（Config、DelayStrategy、DeadLetterMessage、MessageStatus、AlertInfo、Processor）
- 完整的消息流转状态机图和阶段说明
- 两种延迟策略的算法详解
- 告警触发机制说明
- 多个 API 使用示例（基本使用、固定延迟、手动管理、监控统计、独立配置重试次数）
- 错误处理、线程安全与生命周期说明