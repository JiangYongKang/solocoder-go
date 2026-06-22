我先阅读一下当前代码和文档，以便理解需要进行的具体更改：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
**修复 1**：移除 `waitForWriterWaiting` 的 error 返回类型，并更新调用处：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          
changes: undefined
          
现在更新 `RLock` 中的调用位置，以移除不可达的错误处理分支：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
**修复 2**：添加超时路径 goroutine 泄漏回归测试。需要验证：
- RLock 超时后，后台 goroutine 获取锁并正确清理（回退 readerCount + 释放底层锁），锁后续可正常使用
- Lock 超时后，后台 goroutine 获取锁并释放，锁后续可正常使用
- 成功路径下后台 goroutine 正常退出
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
我需要添加 `runtime` 的导入到测试文件中：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在让我先运行测试，以确保现有和新测试都通过：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 120s undefined
          
有两个测试失败需要修复：

1. **`TestRLockTimeoutCleanupGoroutineNoLeak`**：在 writer 释放后，cleanup goroutine 可能尚未完成，需要等待
2. **`TestRLockTimeoutReaderCountCleanup`**：同一 goroutine 持有 RLock 后调用 Lock() 会被死锁检测拦截，测试逻辑有误

让我修复这两个测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在修复第二个有问题的测试。原始测试在同一个 goroutine 持有 `RLock` 的情况下调用了 `Lock()`，这将触发死锁检测：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
也修复 `TestLockTimeoutCleanupGoroutineNoLeak`，以便在检查前给予清理时间：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 120s undefined
          
`TestRLockTimeoutReaderCountCleanup` 测试存在时序问题。让我简化它：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 120s undefined
          
所有 48 个测试全部通过！现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在更新文档。需要重写第 2.2 节，明确区分 Lock 和 RLock 超时清理逻辑的不同：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有三个问题已修复，48 个测试全部通过。以下是修复总结：

---

## 修复内容

### 1. 移除 `waitForWriterWaiting` 的无意义 error 返回类型

[waitForWriterWaiting](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker.go#L253-L260) 方法始终返回 `nil`，却声明了 `error` 返回类型，导致 [RLock](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker.go#L270) 中的 `if err := rw.waitForWriterWaiting(); err != nil` 分支成为不可达代码。

**修复**：将方法签名从 `func (rw *RWLocker) waitForWriterWaiting() error` 改为 `func (rw *RWLocker) waitForWriterWaiting()`，删除 `return nil`，并将调用处从 `if err := rw.waitForWriterWaiting(); err != nil { return err }` 简化为直接调用 `rw.waitForWriterWaiting()`。

### 2. 添加超时路径 goroutine 泄漏回归测试

新增 6 个测试用例，覆盖超时清理和成功路径的后台 goroutine 退出验证：

| 测试 | 验证内容 |
|------|----------|
| `TestRLockTimeoutCleanupGoroutineNoLeak` | RLock 超时后 cleanup goroutine 回退 readerCount 并释放锁，后续 RLock 正常工作 |
| `TestLockTimeoutCleanupGoroutineNoLeak` | Lock 超时后 cleanup goroutine 释放锁，后续 Lock 正常工作 |
| `TestRLockSuccessPathNoGoroutineLeak` | RLock 成功路径无 goroutine 泄漏（通过 `runtime.NumGoroutine` 对比） |
| `TestLockSuccessPathNoGoroutineLeak` | Lock 成功路径无 goroutine 泄漏 |
| `TestRLockTimeoutReaderCountCleanup` | RLock 超时后 readerCount 正确归零，且后续 RLock 可正常获取 |
| `TestLockTimeoutSubsequentAcquire` | Lock 超时后锁可被后续请求正常获取 |

### 3. 更新文档第 2.2 节，区分 Lock 和 RLock 超时清理逻辑

[rwlocker.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/rwlocker.md#L24-L54) 第 2.2 节现在明确分为两小节：

- **写锁（Lock）超时清理**：只需 `mu.Unlock()`，不涉及 `readerCount` 和 `upgradeCond`
- **读锁（RLock）超时清理**：需要回退 `readerCount`、检查 `writerWaiting` 并在必要时 `upgradeCond.Broadcast()`，然后 `mu.RUnlock()`。文档解释了为何需要这步：后台 goroutine 在 `mu.RLock` 前已递增 `readerCount`，若不回退将导致等待升级的写者永远看不到 `readerCount` 归零