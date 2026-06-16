我需要先了解当前项目的结构，然后规划并实现指数退避重试器功能模块。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: go.mod
          
让我先查看项目的 go.mod 文件和几个现有包的代码风格，以便遵循项目的编码规范。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\delaysched\scheduler_test.go
          
让我再查看一个简单的模块和一个文档示例，以确保完全遵循项目规范。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\csrf.md
          
现在我已经充分了解了项目结构和代码风格。让我创建任务列表并开始实现指数退避重试器功能模块。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我开始实现核心功能。首先创建 retry.go 文件。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          
changes: undefined
          
现在核心代码已完成，让我标记第一个任务完成并开始编写单元测试。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
changes: undefined
          
现在让我标记第二个任务完成并运行测试。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/retry/ -v undefined
          
所有 33 个测试全部通过！让我标记第三个任务完成并开始编写需求文档。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\retry.md
          
changes: undefined
          
让我标记最后一个任务完成并进行最终验证。








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我运行一下诊断检查，确保代码没有类型或 lint 错误。





        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有任务都已完成！让我为用户总结一下完成的工作。
## 完成总结

指数退避重试器模块已全部实现并验证通过。以下是交付的内容：

### 📁 **核心代码**

| 文件 | 说明 |
|------|------|
| [retry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/retry/retry.go) | 指数退避重试器核心实现 |
| [retry_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/retry/retry_test.go) | 完整单元测试（33 个测试用例 |
| [retry.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/retry.md) | 需求文档 |

### ✅ **已实现的功能**

1. **指数退避算法**：`InitialInterval × 2^attempt`，达到 `MaxInterval` 后封顶
2. **随机抖动机制**：±`JitterFactor` 百分比范围内随机偏移，避免羊群效应
3. **可重试错误判定**：通过 `IsRetryableFunc` 接口自定义，遇到不可重试错误立即中止
4. **最大重试次数**：`MaxRetries` 控制，超限返回 `AggregateError` 聚合所有错误
5. **重试回调钩子**：`OnRetryBefore` 和 `OnRetryAfter`，回调 panic 不影响主流程
6. **Context 支持**：完全兼容 `context.Context`，支持取消和超时
7. **便捷函数式 API**：`Do()` + `WithXxx()` Option 模式

### 🧪 **测试结果**

运行 `go test ./internal/retry/ -v` 结果：**33/33 全部通过 ✅

覆盖场景包括：
- 正常流程（首次成功、重试后成功
- 边界条件（MaxRetries=0、间隔封顶）
- 异常分支（不可重试错误、Context 取消/超时、回调 panic）
- 退避曲线验证、抖动范围验证、并发安全

### 📖 **文档内容**

需求文档 `docs/retry.md` 包含：模块概述、核心结构体职责、指数退避曲线原理（含公式与示例表格、随机抖动机制、可重试错误判定、回调执行时序、8 个使用示例、配置参数说明、并发安全指南、最佳实践（调优建议和反模式警示。