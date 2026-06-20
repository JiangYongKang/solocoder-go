我来分析并修复这些问题。让我先仔细查看当前代码，然后制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
让我先查看测试文件中 caller 相关的测试，以了解当前断言强度。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
现在开始修复问题。让我一次性完成所有修改。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我来一次性修复 structlog.go 中的所有问题：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          
现在修复 `captureCaller` 和 `captureStack` 的过滤逻辑：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我来添加新的测试用例并加强现有测试。首先查看测试文件末尾：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
现在让我加强 caller 相关测试并添加系统字段冲突测试。首先修改现有测试：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
changes: undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
现在让我加强 `TestErrorStack` 和 `TestStackExcludesInternalFrames` 测试，并添加系统字段冲突测试。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
changes: undefined
          
现在修改 `TestStackExcludesInternalFrames` 以匹配新的过滤逻辑：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
changes: undefined
          
现在让我在测试文件末尾添加系统字段冲突的测试用例：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/structlog/ -v undefined
          
我发现了问题 —— `isInternalFrame` 中判断 `.Logger.` 的逻辑无法匹配 Go 运行时中方法的实际函数名格式（形如 `(*Logger).log`）。让我修复这个问题：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          
其实有一个更简洁可靠的方法：通过文件名判断。`structlog.go` 中的所有帧都应被视为内部帧，而 `structlog_test.go` 中的帧不被跳过。让我重写：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/structlog/ -v -run "TestErrorStack|TestNonErrorHasCaller|TestSystemFieldCallerNotOverridden|TestSystemFieldStackNotOverridden|TestSystemFieldsAllProtected" undefined
          
这几个测试通过了。现在让我运行全部测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/structlog/ -v undefined
          
所有 44 个测试都通过了！现在让我更新文档。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先读取当前文档，然后进行更新：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          
现在让我更新文档，添加字段保护机制和 caller 定位逻辑的说明：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          
changes: undefined
          
现在让我再添加一个系统字段保护的使用示例，并更新一下概述中提及系统字段保护：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          
changes: undefined
          
现在让我在模块概述中也加入系统字段保护的说明：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\structlog.md
          
changes: undefined
          
让我再运行一次测试，确保全部通过：












        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/structlog/ -v undefined
          
所有 44 个测试都通过了。让我更新任务状态：














        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题已修复，44 个测试全部通过。以下是修复内容的总结：

## 修复内容总结

### 1. 系统字段保护 ([structlog.go#L131-L149](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog.go#L131-L149))

**问题**：用户上下文字段可以覆盖 `ts`、`level`、`msg` 等系统字段，导致日志采集系统失效。

**修复**：调整 `log()` 方法中的字段写入顺序 —— 先写入用户上下文字段，再写入系统字段。由于 map 后写入的键值会覆盖先写入的，因此系统字段始终保留真实值。

### 2. 采样函数命名优化 ([structlog.go#L112-L119](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog.go#L112-L119))

**问题**：`shouldSample` 函数返回 `true` 表示"应被丢弃"，名称与语义相悖，构成双重否定。

**修复**：重命名为 `isSampledOut`，语义更清晰 —— "是否被采样淘汰"，返回 `true` 表示该条日志被采样丢弃，调用处 `if l.isSampledOut(level) { return }` 读起来自然。

### 3. 调用者定位偏差 ([structlog.go#L163-L169](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog.go#L163-L169))

**问题**：原 `pkgPrefix` 过滤规则误将同包的测试函数也过滤掉，导致 caller 指向 `testing.go` 而非测试文件中的实际调用行。

**修复**：改为基于源文件名的过滤策略 —— 文件名以 `structlog.go` 结尾的帧被视为内部帧。这种方式更精准，不会误过滤同包的测试代码或其他业务代码。

### 4. 新增测试用例 ([structlog_test.go#L894-L1048](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog_test.go#L894-L1048))

新增 6 个系统字段保护测试：
- `TestSystemFieldLevelNotOverridden` — 验证 level 不被覆盖
- `TestSystemFieldMsgNotOverridden` — 验证 msg 不被覆盖
- `TestSystemFieldTsNotOverridden` — 验证 ts 不被覆盖且格式正确
- `TestSystemFieldCallerNotOverridden` — 验证 caller 不被覆盖且指向正确文件
- `TestSystemFieldStackNotOverridden` — 验证 stack 不被覆盖且类型正确
- `TestSystemFieldsAllProtected` — 所有系统字段同时冲突的综合测试

同时加强了 `TestNonErrorHasCaller`、`TestErrorStack`、`TestStackExcludesInternalFrames` 等测试的断言强度，确保 caller/stack 确实指向 `structlog_test.go`。

### 5. 文档更新 ([structlog.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/structlog.md))

- 模块概述中增加第 6 项"系统字段保护"功能说明
- 新增"系统字段保护机制"小节，说明实现原理
- 新增"调用者定位逻辑"章节，详细说明内部帧过滤规则和优势
- 新增示例 6：系统字段保护（防止用户字段覆盖）