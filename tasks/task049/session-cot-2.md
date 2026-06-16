好的，我来逐一修复这些问题。首先制定计划：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先，让我读取当前代码，确认实现细节：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe.go
          
现在开始修复。首先修复 MemorySource.Fetch 方法，实现真正的增量提取逻辑：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe.go
          
changes: undefined
          
现在修复第二个问题 - NewMemorySourceFactory 工厂函数：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe.go
          
changes: undefined
          
现在修复第三个问题 - TestPipelineWriteTimeout 测试中的无效断言。先读取测试文件中的相关部分：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestPipelineWriteTimeout
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          
changes: undefined
          
现在我需要更新 slowMemoryTarget 结构体（因为我移除了 onceDelay 字段），并同时补充增量提取和工厂函数的测试。让我先读取 slowMemoryTarget 的完整实现：







        
            
toolName: search_by_regex
            
status: success
          
query: type slowMemoryTarget struct
          
现在来补充增量提取模式的单元测试和工厂函数测试。让我先读取测试文件末尾，找到合适的位置添加：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestErrorUnwrap
          
现在我在测试文件末尾添加新的测试用例：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\etlpipe\etlpipe_test.go
          
changes: undefined
          
现在运行测试，验证所有修复是否正确：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/etlpipe/ -v -count=1 2>&1 undefined
          
所有 **42 个测试全部通过**！让我更新任务状态：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## ✅ 所有问题已修复，42 个测试全部通过

### 修复的三个问题：

#### 1. MemorySource.Fetch 增量提取失效
**问题**：`Fetch` 方法只使用 `cursor.LastOffset` 做数组切片，完全不读取 `LastValue` 和 `cursor.Mode`，增量模式形同虚设。

**修复**：在 [etlpipe.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/etlpipe/etlpipe.go#L1085-L1181) 中新增 `findStartIndex` 方法：
- `ExtractModeFull`：使用 `LastOffset` 切片（原有逻辑）
- `ExtractModeID`：从 `currentOffset` 开始扫描，找到第一个 `SeqID > lastID` 的记录
- `ExtractModeTimestamp`：从 `currentOffset` 开始扫描，找到第一个 `Timestamp.After(lastTs)` 的记录
- 类型不匹配或 `LastValue` 为 `nil` 时，安全回退到 offset 模式

#### 2. NewMemorySourceFactory 增量字段被静默忽略
**问题**：要求 `timestamp_field` 和 `id_field` **必须同时存在**才调用 `SetIncrementalFields`，单传一个字段会被静默忽略。

**修复**：在 [etlpipe.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/etlpipe/etlpipe.go#L1200-L1212) 中改为分别读取两个字段，只要有一个非空就设置。

#### 3. TestPipelineWriteTimeout 无效断言
**问题**：`stats.WriteErrorCount < 0` 在 int64 语义下永远为假，无法验证超时逻辑。

**修复**：在 [etlpipe_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/etlpipe/etlpipe_test.go#L1125-L1182) 中加强断言：
- 验证 `WriteErrorCount == 10`（全部超时失败）
- 验证 `WrittenCount == 0`
- 验证 `TransformedCount == 10`（转换成功但写入失败）
- 验证错误队列中每条错误都包含 `ErrWriteTimeout`

### 新增的 12 个单元测试：

| 测试函数 | 覆盖场景 |
|---------|---------|
| `TestMemorySourceIncrementalByID` | ID 增量模式下多批次提取 |
| `TestMemorySourceIncrementalByTimestamp` | 时间戳增量模式下多批次提取 |
| `TestMemorySourceIncrementalWrongType` | LastValue 类型不匹配时的安全回退 |
| `TestMemorySourceIncrementalNilCursor` | nil 游标处理 |
| `TestMemorySourceIncrementalFullModeStillWorks` | 全量模式不受影响 |
| `TestMemorySourceIncremental_IDModeWithMixedSeqIDs` | 非连续 SeqID 的过滤 |
| `TestPipelineIncrementalByID_NoDuplicatesOnSecondRun` | **两次运行管道不重复提取**（核心验证） |
| `TestPipelineIncrementalByTimestamp_NoDuplicatesOnSecondRun` | 时间戳增量两次运行无重复 |
| `TestNewMemorySourceFactory_OnlyTimestampField` | 只传 timestamp_field |
| `TestNewMemorySourceFactory_OnlyIDField` | 只传 id_field |
| `TestNewMemorySourceFactory_BothFields` | 两个字段都传 |
| `TestNewMemorySourceFactory_NoFields` | 都不传 |

### 测试运行结果
```
PASS
ok  solocoder-go/internal/etlpipe   1.032s
```
**42 个测试函数，全部通过 ✅**