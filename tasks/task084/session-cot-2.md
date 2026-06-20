让我分析这些问题并修复它们。首先让我创建一个计划并检查代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先查看代码确认这些问题。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
我确认了三个问题：

1. **ProbabilitySampler 回退逻辑**：短 ID 和非法十六进制 ID 时 `return s.rate > 0` 导致无条件采样

2. **Export 错误信息**：未停止时返回 `ErrProfilerNotStarted` 具有误导性

3. **未使用字段**：`cpuNodeMap` 从未被使用

让我开始修复。首先修复 errors.go 添加新的错误变量：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\errors.go
          
现在让我逐一修复这些问题。首先添加新的错误变量：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在让我修复 errors.go，添加新的错误变量：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\errors.go
          
changes: undefined
          
现在修复 ProbabilitySampler 的回退逻辑。我需要使用哈希方法将任意格式的 requestID 转换为 uint64 进行比较，确保采样比例正确。让我先检查 imports：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
现在修复 ProbabilitySampler。我需要使用哈希方法确保异常 ID 也能按比例采样，同时移除未使用字段和修复 Export 错误信息：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在修复 ProbabilitySampler 的 ShouldSample 方法，使用 FNV 哈希作为回退策略确保正确的采样比例：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在移除未使用的 `cpuNodeMap` 字段：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在移除未使用的字段并修复 Export 错误信息：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在检查 NewRequestProfiler 中是否有 cpuNodeMap 的初始化需要移除：




















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
现在移除初始化中的 cpuNodeMap：






















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在修复 Export 方法的错误信息：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler.go
          
changes: undefined
          
现在需要更新测试。首先检查需要修改现有测试，然后添加新的测试用例：
























        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先检查需要更新的现有测试，然后添加新测试：

























        
            
toolName: search_by_regex
            
status: success
          
query: ErrProfilerNotStarted
          
我需要更新 `TestRequestProfiler_ExportWithoutStop` 测试，因为现在未停止时返回的是 `ErrProfilerNotStopped`。同时我还需要更新其他几个相关测试。让我先查看需要修改的测试：


























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
现在我需要更新现有测试并添加新的测试。让我先更新 `TestRequestProfiler_ExportWithoutStop`，然后添加新的测试用例：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
changes: undefined
          
现在让我在测试文件末尾添加新的测试用例。首先看看测试文件的末尾：































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
现在让我在测试文件末尾添加新的测试用例：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
changes: undefined
          
现在我需要在测试文件中添加缺少的 imports。让我检查并更新：

































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
我需要添加 `fmt` 到 imports：

































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
changes: undefined
          
现在让我运行测试看看是否所有测试都通过了：



































        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v -count=1 undefined
          
有一个旧测试 `TestProbabilitySampler_ShortID` 失败了，因为它期望旧的错误行为。让我查看并修复这个测试：





































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
我需要修复这两个旧测试，因为它们测试的是旧的错误行为。现在短 ID 和非法十六进制 ID 应该按正确的采样比例进行采样：

































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\perfsampler\perfsampler_test.go
          
changes: undefined
          
现在再次运行测试：









































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v -count=1 undefined
          
太棒了！所有 61 个测试全部通过！现在让我检查覆盖率，然后更新文档：







































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -cover -count=1 undefined
          
测试覆盖率达到了 90.0%！现在让我更新文档：









































        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在让我更新文档。首先查看文档中需要修改的部分：








































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
让我查看文档中的概率采样算法和错误处理部分，以便更新：









































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
现在让我更新文档。首先在概率采样算法部分添加回退策略说明：










































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
changes: undefined
          
现在更新错误处理表格，添加新的错误变量：











































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
changes: undefined
          
现在添加 Export 方法在各状态下的预期行为说明：













































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\perfsampler.md
          
changes: undefined
          
现在让我运行诊断检查，然后更新 todo：













































        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有文件都没有诊断错误。让我最后运行一次测试确保一切正常：













































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/perfsampler/ -v -count=1 undefined
          
完美！所有 61 个测试全部通过！让我更新 todo 并完成总结：















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
## 问题修复完成总结

已成功修复请求级性能采样器模块的所有问题，**所有 61 个测试全部通过**，测试覆盖率 **90.0%**。

### 修复的问题

**1. ProbabilitySampler 回退逻辑修复**
- **问题**：短 ID 和非法十六进制 ID 时 `return s.rate > 0` 导致这些请求被无条件采样，破坏了按比例配置的采样率
- **修复**：使用 FNV-1a 64 位哈希算法作为回退策略，确保所有 ID 格式都能按正确比例采样
- **代码位置**：[perfsampler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/perfsampler.go#L56-L83)

**2. Export 错误信息修复**
- **问题**：profiler 已启动但未停止时返回 `ErrProfilerNotStarted`，会误导调用方
- **修复**：未停止时返回新的 `ErrProfilerNotStopped` 错误
- **代码位置**：[perfsampler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/perfsampler.go#L469-L478)

**3. 移除未使用字段**
- **问题**：`RequestProfiler` 结构体中 `cpuNodeMap` 字段从未被使用，增加了不必要的内存开销
- **修复**：从结构体定义和初始化中完全移除该字段
- **代码位置**：[perfsampler.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/perfsampler.go#L89-L107)

**4. 新增错误变量**
- 新增 `ErrProfilerNotStopped = errors.New("perfsampler: profiler not stopped")`
- **代码位置**：[errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/perfsampler/errors.go#L8)

### 新增的单元测试

| 测试名称 | 验证内容 |
|---------|---------|
| `TestProbabilitySampler_ShortIDSampleRate` | 短 ID 采样比例符合配置 |
| `TestProbabilitySampler_InvalidHexIDSampleRate` | 非法十六进制 ID 采样比例符合配置 |
| `TestProbabilitySampler_MixedIDSampleRate` | 混合格式 ID 采样比例符合配置 |
| `TestProbabilitySampler_HashConsistency` | 同一 ID 采样决策始终一致 |
| `TestProbabilitySampler_ShortIDRateZero` | 短 ID + 0 采样率时从不采样 |
| `TestProbabilitySampler_ShortIDRateOne` | 短 ID + 1 采样率时总是采样 |
| `TestExport_ErrorStates` | Export 在三种状态下返回正确错误 |
| `TestExport_ErrorMessagesDistinct` | 两个状态错误信息明确区分 |
| `TestProbabilitySampler_AnomalousIDNotAlwaysSampled` | 异常 ID 不会被无条件采样 |

### 文档更新

在 [perfsampler.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/perfsampler.md) 中新增：

1. **「采样率回退策略」章节** - 说明了异常 ID 格式的处理策略
2. **「Export 方法状态行为」章节** - 详细说明了四种状态下的返回结果
3. 更新了错误处理表格，添加了 `ErrProfilerNotStopped`

运行验证命令：
```bash
go test ./internal/perfsampler/ -v
```