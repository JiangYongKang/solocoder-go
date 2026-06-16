好的，我来逐一修复这两个问题。首先制定计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先读取 findStartIndex 方法确认当前实现：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe.go
          
现在开始修复第一个问题。我需要让 `findStartIndex` 和 `Batch` 的增量值都优先使用配置的自定义字段：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe.go
          
changes: undefined
          
现在修复第二个问题 - 清理 `slowMemoryTarget` 中的死字段：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: type slowMemoryTarget struct
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          
changes: undefined
          
现在补充增量字段配置的单元测试：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          
changes: undefined
          
现在运行测试验证所有修复：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/etlpipe/ -v -count=1 2>&1 undefined
          
所有 **52 个测试全部通过**！让我更新任务状态：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## ✅ 所有问题已修复，52 个测试全部通过

### 修复的两个问题：

#### 1. MemorySource 增量字段配置成死数据
**问题**：`findStartIndex` 和 `Batch` 的增量值直接使用 `Record.SeqID` / `Record.Timestamp`，完全忽略 `SetIncrementalFields` 配置的字段。

**修复**：在 [etlpipe.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/etlpipe/etlpipe.go#L1135-L1220) 中新增两个辅助方法：

| 方法 | 功能 |
|------|------|
| `getRecordSeqID(record)` | 优先从 `record.Data[idField]` 读取（支持 int64/int/float64/string 类型转换），缺省时回退到 `record.SeqID` |
| `getRecordTimestamp(record)` | 优先从 `record.Data[timestampField]` 读取（支持 time.Time / RFC3339 字符串 / 通用时间字符串），缺省时回退到 `record.Timestamp` |

`findStartIndex` 和构造 `Batch` 时都统一调用这两个方法，确保增量字段配置真正生效。

#### 2. slowMemoryTarget 死字段
**问题**：`onceDelay` 和 `hasDelayed` 字段已无设置路径，成为死字段。

**修复**：在 [etlpipe_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/etlpipe/etlpipe_test.go#L1184-L1196) 中精简结构体，只保留 `target` 和 `delay` 两个字段，每次调用都延迟，逻辑更清晰。

### 新增的 10 个单元测试：

| 测试函数 | 覆盖场景 |
|---------|---------|
| `TestMemorySource_CustomIDField_Incremental` | 自定义 ID 字段（int 类型）增量提取 |
| `TestMemorySource_CustomTimestampField_Incremental` | 自定义时间戳字段（time.Time 类型）增量提取 |
| `TestMemorySource_CustomIDField_StringType` | 字符串类型 ID 字段的自动解析 |
| `TestMemorySource_CustomTimestampField_StringType` | 字符串类型时间戳字段的自动解析 |
| `TestMemorySource_CustomIDField_MissingFieldFallback` | ID 字段缺失时回退到 SeqID |
| `TestMemorySource_CustomTimestampField_MissingFieldFallback` | 时间戳字段缺失时回退到 Timestamp |
| `TestMemorySource_SetIncrementalFields_SeparateConfig` | 分别单独设置 timestamp/id 字段 |
| `TestPipeline_CustomIncrementalIDField` | 管道级别自定义 ID 字段 + 两次运行无重复 |
| `TestMemorySource_CustomIDField_Float64Type` | float64 类型 ID 字段的转换 |

### 测试运行结果
```
PASS
ok  solocoder-go/internal/etlpipe   1.028s
```
**52 个测试函数，全部通过 ✅**