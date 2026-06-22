我来修复这两个问题。核心思路是：
1. 移除使用 `runtime.NumGoroutine` 的不可靠测试，改用锁状态正确性和后续操作可用性来间接验证
2. 用「后续锁操作成功」替代 `time.Sleep`，提供确定性保证 —— 如果 cleanup 未完成，后续获取锁会阻塞或失败

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: runtime\.NumGoroutine|time\.Sleep
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在实施修复。核心改进：

**问题 1 修复**：移除 `runtime.NumGoroutine` 测试，替换为锁状态正确性 + 统计一致性验证 + 最终写锁可获取性验证

**问题 2 修复**：用「获取并立即释放写锁」替换 `time.Sleep`。写锁需要等待所有现存读者释放，因此能确定性地证明 cleanup goroutine 已完成 `readerCount--` 和 `mu.RUnlock()`，无需猜等待时长。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在替换 `TestRLockTimeoutCleanupGoroutineNoLeak`，用「获取写锁」作为确定性同步屏障：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在替换两个使用 `runtime.NumGoroutine` 的测试，改用锁状态正确性 + 统计一致性 + 最终锁所有权验证，这比全局 goroutine 计数更可靠：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在修复 `TestRLockTimeoutReaderCountCleanup` 和 `TestLockTimeoutSubsequentAcquire`，去掉 `time.Sleep`，改用写锁做屏障：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
现在修复 `TestLockTimeoutCleanupGoroutineNoLeak` 中的屏障 —— 当前 Config 设置了 WriteTimeout=50ms，所以屏障处的 `Lock()` 会带上超时，不够可靠。改用 `RLock()` 作为屏障（因为 ReadTimeout=0 即无限等待，完全确定性），而 cleanup goroutine 持有的是写锁，所以 RLock() 会阻塞直到 cleanup 释放写锁：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在修复 `TestRLockTimeoutReaderCountCleanup` —— 去掉 `time.Sleep`，改用 `Lock()/Unlock()` 屏障（WriteTimeout=0 即无限等待，完全确定性）：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
`TestLockTimeoutSubsequentAcquire` 也存在同样的竞态：`<-holderDone` 后 cleanup goroutine 可能还没调用 `mu.Unlock()`，直接调用 `Lock()` 可能再次超时。用 `RLock()` 作为屏障（ReadTimeout=0 无限等待）：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 120s undefined
          
`GetStats()` 返回了 nil，因为直接 `&Config{ReadTimeout: 500ms}` 没有设置 `EnableStats`，默认为 false。需要显式设置 `EnableStats: true`：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 120s undefined
          
所有测试通过！现在更新文档，在第 2.2 节的末尾补充关于超时清理路径的测试设计说明：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          

        
            
toolName: search_by_regex
            
status: success
          
query: ^## 
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
在文档末尾添加第 10 节「测试可靠性设计」，详细说明修复后的测试策略：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
最后再验证一次测试全部通过：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -count=3 -timeout 120s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
两个问题已全部修复，测试连跑 3 次全部通过。

---

## 修复总结

### 1. 移除全局 `runtime.NumGoroutine` 差值判断

将 `TestRLockSuccessPathNoGoroutineLeak` / `TestLockSuccessPathNoGoroutineLeak` 重写为 [TestRLockSuccessPathStateIntegrity](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1278-L1320) 和 [TestLockSuccessPathStateIntegrity](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1322-L1371)，用三层验证替代全局计数：

| 验证层 | 做法 |
|--------|------|
| **迭代内状态** | 每次 RLock/RUnlock 后断言 `ReaderCount()` 分别等于 1/0 |
| **统计一致性** | 迭代结束后断言 `ReadRequests == ReadSuccess == 20`，`TimeoutCount == DeadlockDetected == 0` |
| **最终锁所有权** | 20 轮 RLock/RUnlock 后调用 `Lock()` 立即成功（证明无泄漏读者仍持有读锁）；20 轮 Lock/Unlock 后并发 5 个 RLock 全部成功（证明无泄漏写者） |

### 2. 确定性屏障替代固定 `time.Sleep`

利用锁互斥语义形成强制同步 —— **屏障返回的瞬间，100% 保证 cleanup goroutine 已完成所有清理动作**，无需猜测等待时长：

| 测试 | 原代码 | 修复后屏障 | 原理（互斥性） |
|------|--------|------------|--------------|
| [TestRLockTimeoutCleanupGoroutineNoLeak](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1198-L1236) | `time.Sleep(80ms)` | `Lock()/Unlock()` | Lock() 等待所有现存读者释放，cleanup goroutine 的 mu.RUnlock 不执行则 Lock() 不会返回 |
| [TestLockTimeoutCleanupGoroutineNoLeak](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1238-L1276) | `time.Sleep(80ms)` | `RLock()/RUnlock()` | RLock() 等待写锁释放，cleanup goroutine 的 mu.Unlock 不执行则 RLock() 不会返回 |
| [TestRLockTimeoutReaderCountCleanup](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1373-L1414) | `time.Sleep(100ms)` | `Lock()/Unlock()` | 同上，屏障后读 `readerCount` 断言完全可靠 |
| [TestLockTimeoutSubsequentAcquire](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker_test.go#L1416-L1447) | 无（存在隐藏竞态） | `RLock()/RUnlock()` | 消除了 `<-holderDone` 后 cleanup 尚未完成导致后续 `Lock()` 再次超时的竞态 |

**屏障互补原则**：屏障锁类型必须与 cleanup 持有锁类型互斥，且屏障锁超时配置为 0（无限等待）。Config 中只设置一种锁的超时，另一种天然为 0，正好匹配。

### 3. 文档更新

在 [docs/rwlocker.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/rwlocker.md#L537-L593) 新增第 10 节「测试可靠性设计」，详细说明了两种不可靠测试的问题根源、修复策略以及屏障模式的工作原理。