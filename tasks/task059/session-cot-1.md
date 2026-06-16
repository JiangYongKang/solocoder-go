我来帮你实现这个优雅关闭管理器功能模块。首先让我了解一下项目的结构和现有代码风格。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          
让我先查看一些现有模块的代码风格，以便保持一致性。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\eventbus\eventbus.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
现在让我查看一下测试文件的风格：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\orchestrator\orchestrator_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
很好，我已经了解了项目的代码风格。现在开始实现优雅关闭管理器模块。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: mkdir -p internal/graceful undefined
          
现在创建核心实现代码：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
很好！核心代码已完成。现在更新进度并编写完整的单元测试：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
好的，现在运行测试来检查代码是否正确：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/graceful/ -v -count=1 -timeout 120s undefined
          
发现了 64 位原子操作的对齐问题。需要调整结构体字段顺序来解决对齐问题：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在再次运行测试：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/graceful/ -v -count=1 -timeout 120s undefined
          
让我检查完整的测试输出：


        
            
toolName: check_command_status
            
status: success
          
          
让我重新运行测试：

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/graceful/ -v -count=1 -timeout 180s 2>&1 | Out-String undefined
          
让我确保只测试 graceful 包：


        
            
toolName: run_command
            
status: success
          
command: go test -v solocoder-go/internal/graceful -count=1 -timeout 180s undefined
          
让我在具体目录中运行测试：



        
            
toolName: run_command
            
status: success
          
command: cd internal\graceful ; go test -v -count=1 -timeout 180s undefined
          
让我在新终端中直接运行：





        
            
toolName: run_command
            
status: success
          
command: go test -v -count=1 -timeout 180s undefined
          
发现了问题！`signalListener` 中收到 `manualTriggerCh` 信号后没有调用 `Shutdown()`，而且通道设计有问题。让我修复代码：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
还需要将 `manualTriggerCh` 改为带缓冲的通道，以及优化 `TriggerShutdown`：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在优化 `TriggerShutdown` 方法，避免不必要的 goroutine：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在再次运行测试：






        
            
toolName: run_command
            
status: success
          
command: go test -v -count=1 -timeout 120s undefined
          
很好！大部分测试通过了。现在修复剩余的3个问题：

1. `Shutdown()` 第二次调用返回值问题
2. 回调错误/ panic 时报告 Success 不正确






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修复 Success 标志的判断逻辑，需要考虑回调错误的情况：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在再次运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test -v -count=1 -timeout 120s undefined
          
所有测试通过！现在按照用户要求的命令再运行一次验证：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/graceful/ -v -count=1 undefined
          
完美！所有测试通过。现在编写文档：








        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\graceful.md
          
changes: undefined
          
完成！让我更新进度：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务已完成！下面是实现总结：

## ✅ 完成内容

### 1. 核心代码实现
[graceful.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graceful/graceful.go)

**8 大核心功能点已全部实现：**

| 需求 | 实现方式 |
|------|---------|
| **1. 关闭信号监听** | `Start()` 启动信号监听协程，监听 SIGINT/SIGTERM；提供 `TriggerShutdown()` 手动接口；`sync.Once` 保证只执行一次 |
| **2. 等待进行中请求** | `BeginRequest()/EndRequest()` 原子计数；关闭启动后设置 `accepting=false` 拒绝新请求；`RequestWaitTimeout` 配置最大等待时长；50ms 轮询统计剩余请求数 |
| **3. 资源释放回调注册** | `RegisterCallback()` 注册接口；执行时按 Priority 排序后 **反序遍历**；每个回调独立 goroutine + 独立 `Timeout` + panic 捕获 |
| **4. 全局超时强制终止** | `GlobalTimeout` 配置；超时后进入 `PhaseForced` 阶段；记录 `IncompleteCallbacks`、当前 `ActiveRequests`、`GoroutineCount` 等状态快照 |
| **5. 多阶段顺序编排** | 严格按顺序执行：`PhaseStopAccepting` → `PhaseWaitRequests` → `PhaseExecuteCallbacks` → `PhaseComplete/PhaseForced`；每阶段独立超时控制 |

### 2. 单元测试
[graceful_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graceful/graceful_test.go)

**35 个测试用例全部通过**，覆盖：
- **正常流程**：创建/启动/关闭、请求计数、回调注册与反序执行
- **边界条件**：零值配置回退、重复调用、0 请求快速关闭、多等待者
- **异常分支**：回调错误/ panic / timeout、全局 timeout 各阶段触发、竞态并发

```
PASS: 35/35
ok   solocoder-go/internal/graceful  2.511s
```

### 3. 需求文档
[graceful.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/graceful.md)

包含 10 个章节：模块概述、核心结构体职责、状态与阶段定义、**4 阶段执行顺序详解**、超时控制层级图、错误定义、4 个使用示例、并发安全说明、接口速查表、测试覆盖范围。