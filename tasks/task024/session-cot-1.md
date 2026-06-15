让我先探索当前项目的结构，了解项目的代码风格和已有依赖，然后规划数据迁移管道模块的实现。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我查看几个现有模块的代码风格，以便保持一致性：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\job_queue.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jobqueue\jobqueue_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
现在我已经了解了项目的代码风格。让我创建一个详细的任务计划来实现数据迁移管道模块。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先创建 internal/datapipe 目录，然后实现核心代码：


        
            
toolName: run_command
            
status: success
          
command: mkdir -p internal/datapipe undefined
          
现在创建 datapipe.go 核心代码：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe.go
          
changes: undefined
          
核心代码已完成。现在更新任务状态并编写单元测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在编写单元测试文件：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          
现在更新任务状态并运行测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v undefined
          
让我修复未使用变量的问题：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v undefined
          
有两个测试需要修复。让我修正这两个问题：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          
现在修复 StopPipeline 测试：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v undefined
          
需要修复 Stop 时的错误处理逻辑。让我修改核心代码，在 Stop 时的写入中断不应该返回错误：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v undefined
          
所有测试都通过了！现在更新任务状态并编写文档：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\datapipe.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
最后再次运行一次完整测试确保一切正常：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 undefined
          
所有任务已完成！以下是实现总结：

## 完成内容

### 1. 核心代码实现
[datapipe.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datapipe/datapipe.go)

**核心功能：**

| 功能 | 实现方式 |
|------|----------|
| **批次同步** | `fetchNextBatch` 读取 → `writeBatchWithRetry` 写入 → `updateCursor` 持久化断点 → 原子更新计数，四步构成批次循环 |
| **增量迁移** | 支持 `IncrementalModeFull`（全量）、`IncrementalModeTimestamp`（时间戳）、`IncrementalModeID`（递增 ID）三种模式，通过 Cursor.LastValue 定位 |
| **断点续传** | `CheckpointStore` 抽象接口 + 内置内存实现，每批次写入成功后调用 Save()，启动时 Load() 恢复起点 |
| **进度上报** | 后台 goroutine 按 `ProgressInterval` 定时触发 + Run 结束前触发一次，通过 `ProgressCallback` 回调输出速率、百分比、剩余时间等 |
| **失败重试** | `writeBatchWithRetry` 按指数退避 `RetryBackoff * 2^(attempt-1)` 重试，支持 Stop 中途取消 |
| **优雅停止** | `Stop()` 通过 stopCh 广播，等待当前批次完成后安全退出，已处理批次不重复 |

**核心结构体职责：**
- `Pipeline`：迁移管道核心管理器
- `Source` / `Target`：源端/目标端抽象接口（使用方实现）
- `CheckpointStore`：断点存储抽象
- `Cursor`：断点游标（记录 LastValue、LastOffset 等）
- `Batch`：原子处理单元
- `Record`：单条数据记录

### 2. 单元测试
[datapipe_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datapipe/datapipe_test.go)

共 **29 个测试用例**，覆盖：
- ✅ **配置校验**：参数非法场景（5 个）
- ✅ **正常流程**：全量/增量迁移、空源、单记录、各种批次大小对齐（7 个 + 7 个子用例）
- ✅ **断点续传**：中途取消后从断点恢复（1 个）
- ✅ **进度上报**：回调触发、统计量计算（2 个）
- ✅ **异常分支**：写入重试成功/耗尽、读取失败、Count 失败忽略（4 个）
- ✅ **停止机制**：ctx 取消、Stop()、重复调用（3 个）
- ✅ **并发安全**：多 goroutine 同时查询 Getters（1 个）
- ✅ **资源释放**：Source/Target Close 被调用（1 个）

运行结果：**全部 PASS**（29/29，含 7 个子表驱动用例）

```
PASS
ok      solocoder-go/internal/datapipe  0.349s
```

### 3. 需求文档
[datapipe.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/datapipe.md)

包含内容：
- 模块概述与主要特性
- 12 个核心结构体/接口的字段说明与职责
- 完整迁移流程（含 ASCII 流程图 + 4 阶段详细说明）
- 断点续传一致性保证与恢复流程
- 3 种增量模式适用场景
- 进度上报触发时机与统计量计算公式
- 4 个完整使用示例（全量、增量、持久化断点、可取消迁移+监控）
- 错误处理分类与线程安全说明