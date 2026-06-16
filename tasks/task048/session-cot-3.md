让我先仔细阅读当前 pipeline.go 的 Stop 方法和相关代码，然后规划修复方案。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先重新读取 pipeline.go 的相关代码，确认当前实现：


        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\types.go
          
让我也查看一下 OperatorChain.Process 来了解如何区分 Filter 和 FlatMap 的返回：





        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\operator.go
          
现在我完全理解了所有问题。让我逐个修复：

**问题分析：**
1. **Stop 顺序问题**：`wg.Wait()` 在 `window.Stop()` 之前执行，`windowResultReader` 因 `ctx.Done()` 退出后，`window.Stop()` 产出的结果没有消费者
2. **语义区分问题**：`OperatorChain.Process` 统一返回 `nil` 无法区分 Filter 过滤和 FlatMap 展开为空
3. **死锁问题**：先 `source.Stop()` 可能导致死锁，应该先取消上下文让下游退出，再停止 source

让我开始修复，首先修改 PipelineStats 添加 `RecordsExpanded` 字段：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先，在 `types.go` 的 `PipelineStats` 中添加 `RecordsExpanded` 字段：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\types.go
          
changes: undefined
          
现在让我修改 `operator.go`，添加丢弃原因的标识，让 `OperatorChain.Process` 能够区分 Filter 过滤和 FlatMap 展开为空：







        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\operator.go
          
让我修改 `OperatorChain.Process` 方法，添加返回值来区分丢弃原因：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\operator.go
          
changes: undefined
          
现在修改 `pipeline.go` 的 `recordProcessor` 方法，使用新的返回值区分两种丢弃：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在添加 `incrementRecordsExpanded()` 方法：















        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
添加 `incrementRecordsExpanded()` 方法并更新 `Stats()` 方法：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在更新 `Stats()` 方法，让 `RecordsDropped` 包含 `RecordsExpanded`：

















        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在修复最关键的 `Stop()` 方法顺序问题。正确的顺序应该是：
1. 先 `window.Stop()` 让窗口 flush 结果（此时 `windowResultReader` 仍在消费）
2. 然后 `cancel()` 和 `wg.Wait()` 等待协程退出
3. 最后 `source.Stop()`（此时下游已退出，不会死锁）











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在我需要搜索测试文件中所有调用 `Process` 方法的地方，因为 `OperatorChain.Process` 的签名已经改变：



        
            
toolName: search_by_regex
            
status: success
          
query: operators\.Process
          

        
            
toolName: search_by_regex
            
status: success
          
query: \.Process\(
          
让我更精确地搜索 `OperatorChain` 的 `Process` 调用：





        
            
toolName: search_by_regex
            
status: success
          
query: chain\.Process
          
现在让我读取并修复这些测试。先修复 `TestOperatorChain`：







        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
修复 `TestOperatorChain` 中的 `chain.Process` 调用：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
changes: undefined
          
现在修复第 738 行的 `TestOperatorChainState` 中的调用：










        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
changes: undefined
          
现在让我搜索是否还有其他地方调用了 `chain.Process`：









        
            
toolName: search_by_regex
            
status: success
          
query: chain\.Process
          
好的，所有 `chain.Process` 调用都已修复。现在让我更新 todo 列表并添加新的测试用例：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我在测试文件末尾添加新的测试用例。先找到测试文件的末尾：










        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
现在在测试文件末尾添加新的测试用例：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
changes: undefined
          
现在需要更新 `TestRecordsDroppedDistinguishesFilterAndErrors` 测试以反映新的 `RecordsDropped = `RecordsDropped = RecordsFiltered + RecordsExpanded + RecordsErrors`：

让我检查一下当前的测试：









        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
这个测试中 `RecordsExpanded = 0（没有 FlatMap），所以测试仍然通过。但为了更清晰，让我更新断言：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\streamproc_test.go
          
changes: undefined
          
现在让我检查 `SaveCheckpoint` 和 `restoreCheckpoint` 方法，确保 `RecordsExpanded` 字段也被正确保存和恢复：











        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
让我查看完整的 `SaveCheckpoint` 和 `restoreCheckpoint` 方法：













        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
现在让我运行测试验证所有修复是否正确：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/streamproc/ -v -timeout 180s undefined
          
`CollectSink` 没有 `WindowResults()` 方法。让我检查 `CollectSink` 的实现：







        
            
toolName: search_by_regex
            
status: success
          
query: type CollectSink struct
          
让我给 `CollectSink` 添加 `WindowResults()` 方法：











        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
已经有 `GetResults()` 方法了。让我添加 `WindowResults()` 方法作为别名，保持 API 一致性：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
重复了！让我删除重复的 `Count()` 方法：









        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
删除重复的 Count() 方法：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在让我重新运行测试：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/streamproc/ -v -timeout 180s undefined
          
问题很清楚了：`close(p.stopCh)` 在 `p.window.Stop()` 之前调用，导致 `windowResultReader` 在窗口结果产出之前就收到 `stopCh` 信号而退出。我需要调整 Stop 顺序：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\streamproc\pipeline.go
          
changes: undefined
          
现在让我重新运行测试：












        
            
toolName: run_command
            
status: success
          
command: go test ./internal/streamproc/ -v -timeout 180s -run "TestCountWindowStopResultsNotLost|TestTimeWindowStopResultsNotLost|TestSlidingTimeWindowStopResultsNotLost|TestPipelineStopPreservesSinkOrder|TestTumblingTimeWindowStopPartialResults" undefined
          
这些新测试都通过了！现在让我运行完整的测试套件：















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/streamproc/ -v -timeout 180s undefined
          
太棒了！所有 **97 个测试** 全部通过！让我更新 todo 列表：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有 **97 个测试全部通过！以下是本次修复的详细总结：

---

## 修复总结

### 1. Stop 方法顺序问题 — 窗口结果丢失

**问题**：`Stop()` 方法先 `close(p.stopCh)` 在 `p.window.Stop()` 之前执行，导致 `windowResultReader` 在窗口产出结果之前就收到 `stopCh` 信号而退出，窗口 Stop 时产出的 Partial 结果和时间窗口剩余桶结果没有消费者接收，导致结果直接丢失。

**修复**：[pipeline.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/pipeline.go#L618-L657) 的 `Stop()` 方法调整顺序为：

```
旧顺序：close(stopCh) → 设置状态 → source.Stop() → cancel() → wg.Wait() → window.Stop()

新顺序：设置状态 → **window.Stop()** → close(stopCh) → SaveCheckpoint() → cancel() → wg.Wait() → source.Stop()
```

**关键改动**：先调用 `window.Stop()` 让窗口产出所有结果，然后才 close(stopCh) 和 cancel()，确保 `windowResultReader` 有足够的时间消费完所有窗口结果。

### 2. RecordsDropped 语义混淆

**问题**：`recordProcessor` 将 Filter 过滤和 FlatMap 展开为空两种不同语义的丢弃都计入 `RecordsFiltered`，调用方无法区分。

**修复**：
- [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/types.go#L259-L272) 新增 `RecordsExpanded int64` 字段
- [operator.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/operator.go#L281-L313) `OperatorChain.Process` 方法签名修改为返回 `([]*Record, bool, error)`，第二个返回值区分是否被 Filter 过滤
- [pipeline.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/pipeline.go#L179-L181) 新增 `incrementRecordsExpanded()` 方法
- [pipeline.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/pipeline.go#L156-L165) `Stats()` 方法中 `RecordsDropped = RecordsFiltered + RecordsExpanded + RecordsErrors`

现在调用方可以通过 `Stats()` 获取：
- `RecordsFiltered`：Filter 显式过滤的记录数
- `RecordsExpanded`：FlatMap 展开为空的记录数
- `RecordsErrors`：算子执行出错的记录数
- `RecordsDropped`：总丢弃数（以上三者之和

### 3. Stop 方法死锁问题

**问题**：先 `pipeline.Stop() 先调用 `source.Stop()`，如果 source 的输出 channel 和 pipeline 内部 channel 同时满时，source goroutine 阻塞在向下游写入，而 `source.Stop()` 又在等待 source goroutine 退出，形成死锁。

**修复**：[pipeline.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/pipeline.go#L618-L657) 的 `Stop()` 方法调整顺序为：
先处理完所有下游数据后再停止 source，最后停止 source，此时所有下游协程已退出，不会形成死锁。

### 新增测试用例（8个）

| 测试 | 覆盖内容 |
|------|----------|
| `TestCountWindowStopResultsNotLost` | 滚动计数窗口 Stop 时 Partial 结果不丢失 |
| `TestTimeWindowStopResultsNotLost` | 滚动时间窗口 Stop 时结果不丢失 |
| `TestSlidingTimeWindowStopResultsNotLost` | 滑动时间窗口 Stop 时结果不丢失 |
| `TestRecordsFilteredVsRecordsExpanded` | Filter 过滤与 FlatMap 展开为空的区分 |
| `TestOperatorChainProcessDiscriminatesFilterVsFlatMap` | OperatorChain.Process 返回值语义区分 |
| `TestPipelineStopOrderNoDeadlock` | Stop 顺序正确性，无死锁 |
| `TestPipelineStopPreservesSinkOrder` | Stop 期间记录和窗口结果的完整性 |
| `TestTumblingTimeWindowStopPartialResults` | 滚动时间窗口 Partial 结果 |

### 其他修复

- [pipeline.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/streamproc/pipeline.go#L726-L728) 给 `CollectSink` 新增 `WindowResults()` 方法
- 修复了 `CollectSink` 中重复的 `Count()` 方法

所有 **97** 个测试全部通过！✅