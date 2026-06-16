toolName: todo_write
            
status: success
          
Todos updated: 10 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\jobqueue_test.go
          
我再来看看现有的文档格式和另一个模块，作为风格参考：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
现在我对项目的约定有了深入的了解。让我来实现 MapReduce 模块。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
changes: undefined
          
现在让我运行测试，看看它们是否通过：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go && go test ./internal/mapreduce/ -v -timeout 60s undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 60s undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 60s undefined
          
问题是由于 386 架构上 `int64` 原子字段的对齐问题引起的。我需要将 64 位原子字段移动到结构体的开头：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 60s undefined
          
reduce 逻辑存在问题——每个 reduce 分区有多个键，但我们只存储最后一个结果。我还需要处理空的 reduce 分区。让我修复核心逻辑：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
现在我需要修复 `reduce` 逻辑，使其能够按分区收集所有 `key` 的结果，而不仅仅是最后一个：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
现在我需要更新测试以匹配新的数据结构，其中每个 reduce 结果是一个 `[]KeyValue` 而不是单个 `interface{}`。我还需要更新 `mergeResults` 逻辑，因为它需要扁平化 `[]KeyValue` 结果：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
现在我来重写测试，使其与新的数据流匹配，其中每个 Reduce 任务都会产生一个 `[]KeyValue`：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
现在我需要完全重写测试，因为数据模型已经改变——每个 reduce 结果现在都是 `[]KeyValue` 而不是单个值：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
changes: undefined
          
现在，我还要添加 `panic recovery` 到 `map` 任务执行中，因为 `TestRun_PanicRecovery` 测试需要它：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
我需要在 `MapFunc` 调用周围添加 `panic` 恢复功能：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
现在我需要添加 `safeExecuteMap` 方法。我还要为 reduce 阶段添加 panic 恢复功能：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
现在更新 `reduce` 执行，使其使用 `safeExecuteReduce`：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
现在我们再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 60s undefined
          

        
            
toolName: check_command_status
            
status: success
          
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 60s 2>&1 undefined
          
所有 39 个测试都通过了。现在我来编写文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务均已完成。以下是已实现功能的总结：

## 实现概要

### 1. 核心实现 — [mapreduce.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce.go)

**核心类型与接口**：
- `KeyValue` — 键值对基本数据单元
- `MapFunc` / `ReduceFunc` — 用户定义的 Map/Reduce 函数签名
- `PartitionFunc` — 可配置分区策略接口，默认提供 `HashPartition`（FNV-1a 哈希）
- `MergeFunc` — 自定义结果合并函数
- `ShuffleMode` — 同步（`ShuffleSync`）和异步（`ShuffleAsync`）两种 Shuffle 模式
- `TaskError` — 任务失败详情，支持 `Unwrap()` 链式错误检查

**MapReduce 编排器**：
- `Run(ctx)` — 完整的 Map → Shuffle → Reduce → Merge 流程
- `Cancel()` — 主动取消作业
- `Status()` / `Result()` / `Errors()` — 状态查询
- `CompletedMapCount()` / `CompletedReduceCount()` — 进度跟踪

**关键设计**：
- Map 任务并行执行，每个输入分片一个 goroutine
- Shuffle 按分区函数路由中间键值对，按 Key 分组
- Reduce 任务并行执行，每个分区一个 goroutine
- 任务失败自动重试，重试时完整重新计算
- Panic 恢复（`safeExecuteMap` / `safeExecuteReduce`）
- int64 原子字段放在结构体头部，确保 32 位平台对齐安全

### 2. 单元测试 — [mapreduce_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce_test.go)

**39 个测试用例**，覆盖以下场景：
- **正常流程**：WordCount、整数求和、排序去重、单输入、单 Reduce、多 Reduce
- **Shuffle 模式**：同步/异步 Shuffle 结果一致性验证
- **边界条件**：空 Map 输出、nil Map 输出、空分区、大量输入（50 条）
- **异常分支**：Map/Reduce 失败重试成功、重试耗尽、零重试、Panic 恢复、上下文取消、作业取消、混合成功/失败、自定义合并函数错误
- **配置校验**：空 MapFunc、空 ReduceFunc、无效 NumReduce、无效 MaxRetries
- **并发安全**：并发 Map 任务执行验证、运行中设置输入、重复调用 Run

### 3. 文档 — [mapreduce.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/mapreduce.md)

详细说明了模块功能、核心结构体职责、完整的 Map → Shuffle → Reduce 数据流转图、核心算法（哈希分区、重试策略、Panic 恢复）以及多种使用示例。