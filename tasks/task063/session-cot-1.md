我来帮你实现这个混沌工程故障注入器模块。首先让我了解一下项目结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先了解项目的模块结构和代码风格。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
让我先查看一下 docs 目录下的文档风格，以及看一下一些其他模块的文件组织方式。</think_never_used_51bce0c785ca2f68081bfa7d91973934>
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\retry.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\errors.go
          
好的，我已经了解了项目的代码风格和文档风格。现在让我开始实现混沌工程故障注入器模块。

首先创建核心文件：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          

        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Path "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault" -Force undefined
          
目录已创建。现在让我创建错误定义文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\errors.go
          
changes: undefined
          
现在创建类型定义文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\types.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在创建主入口文件，实现所有核心功能：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          
changes: undefined
          
现在让我先尝试编译一下，确保代码没有语法错误：




        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/chaosfault/ undefined
          
编译通过！现在让我编写完整的单元测试：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault_test.go
          
changes: undefined
          
现在运行测试看看结果：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/chaosfault/ -v undefined
          
有一个测试失败。让我修复一下 `CheckError` 方法中只有 message 时的处理逻辑：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          
changes: undefined
          
现在重新运行测试：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/chaosfault/ -v undefined
          
所有测试都通过了！现在让我编写需求文档：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\chaosfault.md
          
changes: undefined
          
现在让我再确认一下所有测试都能通过，并检查一下代码诊断：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/chaosfault/ -v -count=1 undefined
          
所有测试都通过了！混沌工程故障注入器模块已全部实现完成。让我为你总结一下：

## 完成内容

### 1. 核心文件（`internal/chaosfault/` 包）

- **[errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/errors.go)** - 错误定义，包括 `InjectedError`、`ConnectionBrokenError` 等类型
- **[types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/types.go)** - 核心类型定义，包括 `FaultInjector`、`DelayConfig`、`ErrorConfig`、`DisconnectConfig`、`TimeWindow` 等
- **[chaosfault.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/chaosfault.go)** - 主入口文件，实现所有故障注入逻辑
- **[chaosfault_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/chaosfault_test.go)** - 单元测试（30+ 测试用例）

### 2. 实现的功能

| 功能 | 说明 |
|------|------|
| **延迟注入** | 支持固定时长和随机范围两种模式，在目标调用执行前生效 |
| **错误注入** | 支持注入预定义错误，按概率触发，命中则直接返回错误不执行原逻辑 |
| **连接断开模拟** | 支持手动断开/恢复，以及配置化概率断开，断开期间所有请求立即失败 |
| **时间窗口控制** | 所有故障支持设置生效时间窗口，仅在窗口内生效 |
| **目标比例控制** | 所有故障支持配置受影响请求的百分比（0%~100%） |

### 3. 测试覆盖

共 **32 个测试用例**，覆盖：
- 正常流程：各故障模式的启用、禁用、参数设置
- 边界条件：比例为 0、比例为 1、时间窗口边界
- 异常分支：无效配置、无效比例、无效时间窗口
- 并发安全：多 goroutine 并发读写测试
- 综合场景：多种故障组合注入测试

### 4. 需求文档

[docs/chaosfault.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/chaosfault.md) - 包含模块功能说明、核心结构体职责、三种故障注入模式原理、时间窗口与比例控制、使用示例等完整文档。

### 快速使用示例

```go
fi := chaosfault.NewFaultInjector()

// 启用 100ms 固定延迟，50% 概率
fi.SetDelayConfig(chaosfault.DelayConfig{
    Enabled:     true,
    Mode:        chaosfault.DelayModeFixed,
    Fixed:       100 * time.Millisecond,
    TargetRatio: 0.5,
})

// 综合注入（先检查断开、再检查错误、最后应用延迟）
err := fi.Inject(func() error {
    return yourBusinessLogic()
})
```