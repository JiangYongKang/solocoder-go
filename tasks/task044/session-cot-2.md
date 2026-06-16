我来逐一修复这些问题。首先让我重新审视当前代码，确认问题所在。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
我看到代码已经有一些修复的痕迹。让我先运行当前测试，确认现有测试通过，然后仔细分析并完善修复，同时补充针对每个问题的单元测试。

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/auditlog/ -v 2>&1 | Select-Object -Last 50 undefined
          
有一个测试失败了（多 worker 场景下哈希链校验），同时我需要完善 4 个问题的修复并补充测试。让我先修复代码，再补充测试。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我修复问题2和问题4。首先调整 Stop 的执行顺序，确保完全避免 channel 竞态，然后修复 VerifyIntegrity 的 Writer 校验逻辑以支持多 worker 乱序场景。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          
现在修复问题4 — VerifyIntegrity 的 Writer 校验逻辑，使其在多 worker 乱序场景下也能正确工作：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          
现在清理不再使用的 `stopCh` 字段和相关代码：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          
现在删除 Log 方法中不再需要的 channel 发送 panic 恢复逻辑（因为新的 Stop 顺序已经从根本上避免了这个问题）：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在读取测试文件，然后为每个问题补充单元测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
现在在测试文件末尾添加针对4个问题的补充测试：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
我需要修复测试代码中的两个问题：无用的空 if 语句和无法编译的方法赋值方式：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
我需要修复一个问题：`nonReadableWriter` 的 `Count()` 方法让它意外实现了 `ReadableWriter` 接口，这违背了测试目的。同时 `blockingWriter` 的 `ReadAll()` 也应该移除。











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在运行测试看看结果：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/auditlog/ -v 2>&1 undefined
          
`blockingWriter` 的字段名和方法名冲突了，让我修复：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在更新测试中调用 `block` 和 `unblock` 的地方：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
同样修复 `slowFailingWriter` 的命名冲突问题：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在更新测试中调用 `slowFail.block()` 和 `slowFail.unblock()` 的地方：























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在运行测试看看当前状态：

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/auditlog/ -v 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
## 修复完成！全部 61 个测试通过

### 问题修复总结

#### 问题 1：LogSync 缺少 started 状态检查
**修复位置**：[auditlog.go#L427-L433](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog.go#L427-L433)

`LogSync` 与 `Log` 均检查 `l.stopped || !l.started`，两者行为完全一致。未启动或已停止的 Logger 写入均返回 `ErrLoggerStopped`。

---

#### 问题 2：Log 与 Stop 的 buffer channel 竞态（send on closed channel panic）
**修复位置**：[auditlog.go#L237-L256](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog.go#L237-L256)

**根本原因**：原 Stop 顺序是 `close(stopCh)` → `wg.Wait()` → `close(buffer)`，Log 在释放锁后发送到 channel 的窗口内，Stop 可能已完成 `close(buffer)`。

**修复方案** — 调整 Stop 执行顺序为：
```
1. 设置 stopped = true       (拒绝新的 Log 请求)
2. pendingWg.Wait()          (等待所有进行中的 Log 调用完成，包括 buffer 发送)
3. close(buffer)             (此时没有任何 goroutine 会向 buffer 发送)
4. wg.Wait()                 (等待 worker 处理完所有缓冲数据后退出)
5. asyncWg.Wait()            (等待降级 goroutine 完成)
```

同时，`workerLoop` 改为 `for log := range l.buffer` 自然处理完所有数据后退出，删除了 `stopCh` 和不必要的 `recover` 兜底。

---

#### 问题 3：降级 goroutine 未纳入 WaitGroup 追踪
**修复位置**：[auditlog.go#L382-L391](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog.go#L382-L391)

`Log` 在 buffer 满时启动的降级 goroutine 通过 `asyncWg` 追踪：
```go
l.asyncWg.Add(1)
go func() {
    defer l.asyncWg.Done()
    l.persistWithRetry(log)
}()
```

`Stop` 中 `l.asyncWg.Wait()` 确保所有降级 goroutine 完成后才返回，避免进程退出导致日志丢失。

---

#### 问题 4：VerifyIntegrity 未校验 Writer 持久化数据
**修复位置**：[auditlog.go#L527-L665](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog.go#L527-L665)

新增 `ReadableWriter` 接口和两层校验：

| 校验层级 | 校验方式 | 说明 |
|---------|---------|------|
| 内存层 | `verifyLogChainStrict()` | 严格哈希链校验（顺序+链接+自洽） |
| Writer 层 | `verifyLogSetConsistency()` | 集合一致性校验，支持多 worker 乱序：<br>• 每条日志哈希自洽<br>• 在内存中存在且哈希一致<br>• 无重复、无多余、无缺失<br>• 不要求顺序与内存一致 |

校验能检测：内容篡改、哈希伪造、额外日志、日志缺失、重复日志、链断裂。

---

### 新增测试用例（14 个）

| 测试 | 覆盖问题 | 验证内容 |
|------|---------|---------|
| `TestFix_Issue1_LogSync_NotStarted` | 问题1 | LogSync 未 Start 返回 ErrLoggerStopped |
| `TestFix_Issue1_LogSync_AfterStop` | 问题1 | LogSync 已 Stop 返回 ErrLoggerStopped |
| `TestLogger_LogSync_StartedStoppedConsistency` | 问题1 | Log/LogSync 三阶段（未Start/运行中/已Stop）行为一致 |
| `TestFix_Issue2_ConcurrentLogAndStop_NoPanic` | 问题2 | 50 轮并发 Log+Stop 无 panic |
| `TestFix_Issue2_StopFlushesAllPendingLogs` | 问题2 | Stop 确保所有日志被持久化 |
| `TestFix_Issue2_SendToClosedChannel_NoPanic` | 问题2 | 阻塞 Writer 场景下 Stop 无 panic |
| `TestFix_Issue3_StopWaitsForAsyncDegradeGoroutines` | 问题3 | Stop 等待降级 handler 完成 |
| `TestFix_Issue3_AsyncGoroutineWithoutDegrade` | 问题3 | Stop 等待异步写入 goroutine 完成 |
| `TestFix_Issue4_VerifyIntegrity_DetectsWriterTamper` | 问题4 | 检测 Writer 层内容篡改 |
| `TestFix_Issue4_VerifyIntegrity_DetectsWriterExtraLog` | 问题4 | 检测 Writer 层额外注入日志 |
| `TestFix_Issue4_VerifyIntegrity_DetectsWriterHashMismatch` | 问题4 | 检测 Writer 层哈希伪造 |
| `TestFix_Issue4_VerifyIntegrity_WriterLogNotInMemory` | 问题4 | 检测 Writer 层日志不存在于内存 |
| `TestFix_Issue4_MultipleWorkers_VerifyIntegrity` | 问题4 | 4 Worker 乱序写入场景完整性校验通过 |
| `TestFix_Issue4_WriterWithDuplicateLogs` | 问题4 | 检测 Writer 层重复日志 |
| `TestFix_Issue4_NonReadableWriter_OnlyMemoryCheck` | 问题4 | 非 ReadableWriter 仅校验内存层 |

### 验证结果
```
go test ./internal/auditlog/ -v
=== RUN   共 61 个测试用例
--- PASS: 全部通过
ok      solocoder-go/internal/auditlog  1.574s
```