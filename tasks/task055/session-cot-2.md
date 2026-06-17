toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
Now I understand both issues clearly. Let me fix them:

**Issue 1**: The `shuffleAsync` reduce goroutines are empty loops. The reduce logic must actually execute inside these goroutines, and they should be able to start processing as data arrives.

**Issue 2**: The `onComplete` channel is never closed. I need to close it when the job finishes and export it via a `Done()` method.

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
Now let me rewrite `shuffleAsync` to actually execute Reduce logic inside those goroutines, and add the `Done()` method:

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
Now let me add the `Done()` method to expose `onComplete` to callers:


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce.go
          
changes: undefined
          
Now let me also update the todo list and run the tests:



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 120s undefined
          
All 49 tests pass. Now let me also add a specific test for the `Done()` channel to ensure it's properly tested:

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
Let me add tests for the `Done()` channel to verify the new functionality works:


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
changes: undefined
          
Now let's run all tests again to make sure everything passes including the new tests:




        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 120s undefined
          
There's a naming conflict — the loop variable `t` shadows the testing parameter. Let me fix that:







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mapreduce\mapreduce_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 120s undefined
          
All 55 tests pass. Now let me also update the documentation to mention the new `Done()` method and the corrected async shuffle behavior:







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Let me read more of the docs to find where to add the `Done()` method documentation and update the async shuffle section:





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Now let me update the documentation to reflect the fixes. I'll add the `Done()` method to the lifecycle and API examples, and update the ShuffleAsync description to be more accurate:







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
changes: undefined
          
Now let me also update the ShuffleMode description in section 2:









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Good, the ShuffleMode section is already accurate enough. Now let me update section 4.1 to better describe the async shuffle behavior and also add a section about the `Done()` channel. Let me also add the API example for `Done()`:











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Now let me add the `Done()` method documentation and example to the docs. Let me insert a new API usage example:










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
changes: undefined
          
Now let me update the lifecycle section to include `Done()`:











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
changes: undefined
          
Now let me also add a more detailed description of the async shuffle internals to section 4. Let me add a new subsection 4.5:












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Now let me find section 4.4 about context cancellation so I can add a new section 4.5:













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
Perfect. Let me add sections 4.5 and 4.6 after 4.4:













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mapreduce.md
          
changes: undefined
          
Now let me finalize by running all tests one more time to ensure everything is still working:















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -v -timeout 120s 2>&1 | tail -20 undefined
          
Let me just run the test directly:
















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/mapreduce/ -timeout 60s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题已修复完成，共 **55 个测试用例全部通过**。

## 修复内容总结

### 问题 1：shuffleAsync 中 Reduce goroutine 为空循环
**修复位置**：[mapreduce.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce.go#L310-L499)

- **原问题**：异步模式下为每个 Reduce 分区启动的 goroutine 内部只是空的 `select` 循环，从未执行实际的 Reduce 计算，与同步模式行为完全相同
- **修复后**：
  - Reduce goroutine 提前启动，维护本地分组表 `localGrouped map[string][]interface{}`
  - 每当 Map 任务完成并写入分区数据时，对应 Reduce goroutine 被唤醒，执行**增量分组**（与剩余 Map 任务并行）
  - 所有 Map 完成后，Reduce goroutine 立即对本地已分组的数据调用 `ReduceFunc` 执行聚合
  - 新增 `finalizeAndRunReduce()` 方法负责重试逻辑和结果存储
  - 优化通知机制：Map 完成后仅通知**实际受影响的分区**，而非所有分区

### 问题 2：onComplete 通道未使用
**修复位置**：
- [Run() 方法中关闭 onComplete](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce.go#L185-L192)
- [新增 Done() 方法](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce.go#L705-L707)

- **原问题**：`onComplete` 通道在结构体中定义但从未被 close 或监听
- **修复后**：
  - 在 `Run()` 的 `defer` 中统一关闭 `onComplete`（无论成功、失败还是取消都会关闭）
  - 新增 `Done() <-chan struct{}` 公开方法，返回只读通道供调用方非阻塞监听作业完成

### 新增测试
在 [mapreduce_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mapreduce/mapreduce_test.go#L1396-L1599) 中新增了 6 个测试：
- `TestRun_DoneChannel_ClosesAfterRun` — 正常完成后通道关闭
- `TestRun_DoneChannel_ClosesAfterError` — 出错后通道关闭
- `TestRun_DoneChannel_ClosesAfterCancel` — 取消后通道关闭
- `TestRun_DoneChannel_AsyncShuffle` — 异步模式通道关闭
- `TestRun_DoneChannel_NotClosedBeforeRun` — 运行前通道未关闭
- `TestRun_AsyncShuffle_ProcessesDataIncrementally` — 验证异步模式 Reduce 增量处理数据

### 文档更新
在 [mapreduce.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/mapreduce.md) 中：
- 更新了 Shuffle 模式的详细说明
- 新增 **4.5 异步 Shuffle 模式详解** 和 **4.6 完成通知通道** 两节
- 新增 **5.7 通过 Done() 通道接收完成通知** 使用示例
- 在生命周期章节中补充了 `Done()` 方法