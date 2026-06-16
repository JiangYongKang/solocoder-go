我来帮你实现这个降级策略链功能模块。首先让我了解一下当前项目的结构，然后规划实现方案。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          
让我先看几个现有模块的代码风格，以便遵循项目规范：

        
            
toolName: view_files
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\circuit\circuit.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\circuit.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我再看一下 gateway 模块的 types.go 和另一个完整的模块来了解代码风格：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\types.go
          

        
            
toolName: view_files
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\types.go
          
让我再看一下错误和配置的写法规范：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\config.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
现在我已经了解了项目的代码风格，让我开始实现降级策略链功能模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: mkdir -p c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback undefined
          
目录创建成功，现在开始创建核心代码文件。首先创建类型定义文件：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\types.go
          
changes: undefined
          
现在创建错误定义文件：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\errors.go
          
changes: undefined
          
现在创建配置文件：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\config.go
          
changes: undefined
          
现在创建核心实现文件 fallback.go：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
让我先编译检查一下代码是否有错误：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/fallback/ undefined
          
编译成功！现在让我更新 todo 列表，然后开始编写单元测试：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在创建完整的单元测试文件：










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在让我运行测试看看是否通过：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fallback/ -v -timeout 120s undefined
          
有一个未使用的变量错误，让我修复它：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在再次运行测试：












        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fallback/ -v -timeout 120s undefined
          
让我检查完整的测试输出：















        
            
toolName: check_command_status
            
status: success
          
          
看起来运行了整个项目的测试。让我专门只运行 fallback 包的测试：


        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 | Select-Object -First 200 undefined
          
很好！大部分测试都通过了，只有一个测试失败。问题是当策略切换到 fallback 后，后续调用不会再执行主策略，所以主策略的失败次数只有 1 次。让我修复这个测试：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在再次运行测试：















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 undefined
          
所有测试都通过了！现在让我更新 todo 列表并编写模块需求文档：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fallback.md
          
changes: undefined
          
文档编写完成！让我最后再运行一次测试确保所有功能正常：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fallback/ -v -timeout 60s undefined
          
所有测试都通过了！让我总结一下完成的工作：

## ✅ 任务完成总结

### 1. 创建的文件

| 文件 | 路径 |
|------|------|
| 类型定义 | [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/types.go) |
| 错误定义 | [errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/errors.go) |
| 配置定义 | [config.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/config.go) |
| 核心实现 | [fallback.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go) |
| 单元测试 | [fallback_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback_test.go) |
| 需求文档 | [fallback.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fallback.md) |

### 2. 实现的功能

✅ **多级降级方案按优先级排列**：支持注册多个降级处理函数并指定优先级，优先级数字越小越优先使用。

✅ **主策略失败后自动切换**：主策略失败时自动按优先级顺序尝试下一个降级方案，全部失败时返回聚合错误。

✅ **降级触发条件可配置**：支持四种触发条件类型：
- `TriggerConditionTimeout` - 超时触发
- `TriggerConditionErrorType` - 特定错误类型触发
- `TriggerConditionErrorRate` - 错误率阈值触发
- `TriggerConditionCustom` - 自定义判断逻辑

✅ **降级恢复后的自动切回**：
- 主动探测模式 (`RecoveryModeActive`)：后台定期探测主策略可用性
- 被动检测模式 (`RecoveryModePassive`)：通过统计主策略成功次数判断
- 可配置预热期 (`WarmUpDuration`) 验证主策略稳定性
- 平滑切换，不中断当前服务

### 3. 测试覆盖情况

共 **31 个测试用例**，全部通过 ✓，覆盖：
- 正常执行流程
- 降级切换流程
- 所有策略失败的边界情况
- 四种触发条件
- 主动/被动恢复模式
- 预热期机制
- 并发安全
- panic 恢复
- Context 取消
- 状态流转
- 错误率计算
- 强制切换
- 优先级排序
- 多级降级

### 4. 核心结构体

- **Chain**：降级策略链的核心管理器，负责协调所有策略
- **Strategy**：单个降级策略，包含处理函数和统计数据
- **TriggerCondition**：降级触发条件配置
- **RecoveryConfig**：恢复检测行为配置
- **AggregateError**：聚合错误，包含所有失败策略的错误信息

所有代码遵循了项目的现有代码风格，使用 `sync.RWMutex` 保证并发安全，完整支持 `context` 传递和取消。