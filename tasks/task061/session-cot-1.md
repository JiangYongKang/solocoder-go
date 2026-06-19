我来帮你实现这个超时传播器功能模块。首先让我了解一下项目的结构和现有代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我查看几个相关的包来了解代码风格和约定：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\types.go
          
让我查看一个测试文件来了解测试风格：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\retry.md
          
现在我已经了解了项目的代码风格和文档约定。让我开始实现超时传播器模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先，让我创建 `internal/timeoutprop/` 目录并实现核心模块。我会按照项目的代码风格，将类型定义、错误定义和主逻辑组织清晰。


        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Path "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop" -Force undefined
          
现在让我创建核心类型和错误定义文件：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\types.go
          
changes: undefined
          
现在创建错误定义文件：






        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\errors.go
          
changes: undefined
          
现在让我实现核心的超时传播器主逻辑文件：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我实现核心的超时传播器主逻辑：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
让我先编译检查一下代码是否有语法错误：






        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/timeoutprop/ undefined
          
让我修复这些编译错误：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\errors.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我再次编译检查：






        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/timeoutprop/ undefined
          
编译通过！现在让我编写完整的单元测试：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在让我运行测试，看看测试结果：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v undefined
          
命令似乎被中断了。让我用更安全的方式运行测试，先看看基本测试是否通过：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestNewPropagator" -timeout 10s undefined
          
好的，基本测试通过了。让我继续测试更多功能：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestAddStage" -timeout 10s undefined
          
发现死锁问题了！在 `Execute` 方法中调用 `TotalBudget()` 会导致锁重入。让我修复这个问题：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
我看到问题了。在持锁状态下调用 `TotalBudget()` 会导致死锁。让我修复这个问题：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我重新运行测试，看看死锁问题是否解决了：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestAddStage" -timeout 10s undefined
          
很好，AddStage 测试都通过了！让我继续运行其他测试：















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_NoStages|TestExecute_BudgetExceedsTotal|TestExecute_AllStagesSuccess" -timeout 10s undefined
          
好的，让我继续运行超时相关的测试：


















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_BudgetTimeout|TestExecute_TotalTimeout|TestExecute_MinThresholdSkip" -timeout 30s undefined
          
有一个空指针错误。让我查看测试代码的那一行：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
我看到问题了。当预算超时时，`Execute` 返回 `nil` report，导致空指针。让我修复 Execute 函数，确保在所有错误情况下都返回有效的 report：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
我需要重构 `Execute` 函数，确保在所有情况下都返回有效的 report。让我重新组织代码：





















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
让我重构 `Execute` 函数，确保始终返回有效的 report。我会重新编写整个执行逻辑：
























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
我需要添加 `errors` 包的导入，因为我使用了 `errors.Is`。让我检查并修复：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我编译并运行测试：

























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/timeoutprop/ undefined
          
编译通过了！让我运行超时相关的测试：



























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_BudgetTimeout|TestExecute_TotalTimeout|TestExecute_MinThresholdSkip" -timeout 30s undefined
          
我找到问题了！测试 `TestExecute_TotalTimeout` 中总预算 (80ms + 50ms = 130ms) 超过了总超时 (100ms)，所以在预算检查阶段就失败了，根本没有进入执行循环。让我修复测试：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在让我也修复 `TestExecute_BudgetExceedsTotal` 测试，验证预算超限时返回的 report 是有效的：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在让我运行这些测试看看：






























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_BudgetTimeout|TestExecute_TotalTimeout|TestExecute_MinThresholdSkip|TestExecute_BudgetExceedsTotal" -timeout 30s undefined
          
太好了！这些测试都通过了。让我继续运行其他测试：
































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_BudgetCarryOver|TestExecute_ContextAlreadyCanceled|TestExecute_StageReturnsError|TestGetStageInfo|TestReport|TestReset" -timeout 30s undefined
          
有两个测试失败了。让我分析并修复：

1. **TestExecute_ContextAlreadyCanceled**: 当 context 已取消时，应该返回错误
2. **TestExecute_StageReturnsError**: 当 stage 返回错误时，应该跳过后续 stage

让我修复代码：

































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
让我修复这两个问题：

1. 当 context 已取消时，需要设置错误
2. 当 stage 返回普通错误时，也应该跳过后续 stage


































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我修复第二个问题 - 当 stage 返回普通错误时，也应该跳过后续 stage：



































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我重新运行这些测试：







































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -run "TestExecute_ContextAlreadyCanceled|TestExecute_StageReturnsError|TestExecute_BudgetCarryOver" -timeout 30s undefined
          
太好了！这些测试都通过了。现在让我运行所有测试，看看整体情况：










































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -timeout 60s undefined
          
只有 `TestExecute_ZeroBudgetStage` 测试失败了。问题在于零预算的 stage 会立即超时。我觉得更合理的设计是：当预算为 0 时，不设置额外超时，直接使用父 context 的剩余时间。让我修改代码：










































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
让我修改代码，当 stage 的原始预算为 0 时，不设置额外超时，直接使用父 context 的剩余时间：













































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在我还需要修改超时类型判断逻辑，确保在没有预算限制时正确识别为总超时：


















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
让我修改超时类型判断逻辑，以及预算结转逻辑：

















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我运行所有测试，看看是否都通过了：


















































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -timeout 60s undefined
          
太棒了！所有测试都通过了！现在让我更新任务状态，然后编写需求文档：











































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我编写模块需求文档。我会参考项目中已有文档的风格：


















































        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
现在让我再次运行所有测试，确保一切正常：



















































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=1 undefined
          
所有测试都通过了！让我完成最后的任务状态更新：


















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## 完成总结

我已经成功实现了超时传播器功能模块，所有测试均已通过。以下是完成的内容：

### 📁 项目结构

```
internal/timeoutprop/
├── types.go          # 核心类型定义
├── errors.go         # 错误定义
├── timeoutprop.go    # 主逻辑实现
└── timeoutprop_test.go # 单元测试

docs/
└── timeoutprop.md    # 模块需求文档
```

### ✨ 核心功能实现

| 功能 | 说明 |
|------|------|
| **Context 超时自动传递** | 根 context 设置总超时，各子环节自动派生子 context，父超时则所有子同步取消 |
| **剩余时间动态计算** | 每个环节开始时自动查询剩余可用时间（截止时间 - 当前时间） |
| **最小阈值跳过** | 剩余时间不足可配置的最小阈值时，直接跳过该环节并记录 |
| **超时预算分配** | 为各子环节预分配时间预算，总预算不超过总超时 |
| **预算结转复用** | 环节提前完成时，未消费的剩余预算传递给后续环节 |
| **超时链路追踪** | 记录每个环节的超时类型、已消耗时间与分配预算对比 |
| **调用链报告** | 汇总整条链的追踪信息，生成可读的文本报告 |

### 🧪 测试覆盖

共 **37 个测试用例**，全部通过，覆盖：
- 正常流程（所有阶段成功、预算结转等）
- 边界条件（零预算、已取消 context、空名称等）
- 异常分支（预算超时、总超时、阶段错误、panic 等）
- 配置验证（归一化、Option 模式）
- 数据安全（返回副本、并发安全说明）

运行 `go test ./internal/timeoutprop/ -v` 所有测试 PASS ✅

### 📄 文档

在 [docs/timeoutprop.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/timeoutprop.md) 中提供了完整的模块需求文档，包含：
- 模块概述与设计目标
- 核心结构体与职责说明
- 超时传播与预算分配流程图
- 超时类型与阶段状态枚举
- 链路追踪报告格式
- 丰富的使用示例
- 配置说明与最佳实践