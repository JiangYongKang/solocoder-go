我来帮你实现这个请求级性能采样器功能模块。首先让我了解一下项目的结构和现有代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
让我先探索项目结构，了解项目的整体布局和代码风格。

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          
让我再看看一些其他模块的代码风格，特别是有完整测试的模块，以及 go.mod 来了解依赖。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\types.go
          
让我再看看文档的风格，然后开始实现。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          
现在我对项目结构和代码风格有了充分的了解。让我开始实现性能采样器模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先创建目录并实现类型定义文件。




        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Force -Path "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler" undefined
          
目录创建成功。现在让我编写 types.go 类型定义文件。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\types.go
          
changes: undefined
          
现在创建 errors.go 错误定义文件。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\errors.go
          
changes: undefined
          
现在实现主模块文件 perfsampler.go，包含所有核心功能。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在让我编写完整的单元测试文件。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
changes: undefined
          
现在让我运行测试，看看是否有需要修复的问题。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v undefined
          
只有一个测试失败了，问题在于 CPU 总时间的计算逻辑。让我修复这个问题。

问题分析：`RecordCPUSample` 方法会将每个调用栈的所有层级都增加 SampleCount，所以 SampleCount 本身已经代表了该节点及其所有子节点的总样本数。计算 TotalTimeNs 时不应该再递归累加子节点的值。SelfTimeNs 应该是总时间减去所有子节点的总时间。

让我修复 `calculateCPUTotals` 函数：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: calculateCPUTotals
          
现在我理解问题了。`RecordCPUSample` 方法会为调用栈的每一层都增加 `SampleCount`，所以 `SampleCount` 本身已经代表了该函数及其所有子函数的总样本数。计算 `TotalTimeNs` 时不应该再累加子节点的值。让我修复：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
问题在于 `RecordCPUSample` 会为调用栈的每一层都增加 `SampleCount`，所以 `SampleCount` 本身已经代表了经过该节点的总样本数。现在的计算逻辑重复累加了子节点的值。让我修复：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在重新运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v undefined
          
太棒了！所有测试都通过了。让我也检查一下测试覆盖率，然后编写文档。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
先检查一下测试覆盖率：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -cover undefined
          
测试覆盖率达到了 89.9%，非常好！现在让我编写文档。









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
changes: undefined
          
现在让我运行诊断检查确保没有编译错误，然后更新 todo。










        
            
toolName: GetDiagnostics
            
status: failed
          
          
让我分别检查每个文件的诊断：











        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有文件都没有诊断错误。让我再运行一次测试确认，然后更新 todo：















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v -count=1 undefined
          
完美！所有 50 个测试全部通过，测试覆盖率 89.9%。让我更新 todo 并总结完成情况。

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
## 任务完成总结

我已成功实现了请求级性能采样器功能模块，所有 9 项需求均已完成。

### 创建的文件

**代码文件（internal/perfsampler/）：**
- [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/types.go) - 核心数据结构定义
- [errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/errors.go) - 错误变量定义
- [perfsampler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/perfsampler.go) - 主模块实现
- [perfsampler_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/perfsampler_test.go) - 50 个单元测试用例

**文档：**
- [perfsampler.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/perfsampler.md) - 模块需求文档

### 实现的功能

| 需求 | 实现说明 |
|-----|---------|
| **CPU Profiling** | 支持 `EnterCPUFunction`/`ExitCPUFunction` 手动调用栈管理和 `RecordCPUSample` 调用栈记录两种方式，数据以 `CPUStackNode` 树形结构存储 |
| **内存 Profiling** | 支持 `RecordAlloc` 和 `RecordFree`，按函数级别统计分配次数、分配字节数、释放次数、仍在使用的内存 |
| **耗时 Profiling** | 支持 `StartSegment`/`EndSegment` 分段计时，支持嵌套分段，支持元数据附加 |
| **采样率配置** | 三种采样器：`AlwaysSample`、`NeverSample`、`ProbabilitySampler`，采样决策在请求开始时一次性确定，未采样请求零开销 |
| **结果导出** | `Export()` 方法返回结构化 `ProfileResult` 对象，支持 `JSON()` 和 `PrettyJSON()` 序列化 |
| **火焰图格式** | `ToFlameGraph()` 方法将 CPU 调用栈转换为火焰图兼容格式，父子关系通过调用栈层级表达 |
| **单元测试** | 50 个测试用例，覆盖正常流程、边界条件、异常分支、并发安全，覆盖率 89.9% |
| **文档** | 完整的模块需求文档，包含功能说明、结构体职责、数据结构说明、使用示例 |

### 测试结果

```
PASS: 50 个测试全部通过
覆盖率: 89.9%
```

运行命令：`go test ./internal/perfsampler/ -v` 可验证所有测试通过。