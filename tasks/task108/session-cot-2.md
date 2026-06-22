我来分析并修复这些问题。首先让我仔细阅读当前代码：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我逐一修复这些问题。问题分析如下：

1. **Goroutine 泄漏**：`AfterFunc` 回调向缓冲通道发送 `TimeoutError`，如果锁已成功获取则无人读取，且 `incTimeout()` 被错误调用。改用 `done` channel + `select` + `time.NewTimer` 模式。
2. **写者饥饿**：`RLock`/`TryRLock` 不检查 `writerWaiting`，高并发读下 `readerCount` 永不归零。在入口处增加检查。
3. **统计不准**：`incUpgradeRequest` 在校验之前调用，无效请求也被计入。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
由于三个修复都在同一个文件中且相互关联，我将一次性重写 [rwlocker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/rwlocker/rwlocker.go) 来修复所有问题：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我添加覆盖新修复场景的单元测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
          
让我先查看当前代码状态，看看之前的修复是否已经正确应用：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
我看到之前的修复代码已经在文件中了。让我先运行测试看看当前状态：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 60s undefined
          
现在我了解了当前状况。代码中已经包含了三个 bug 的修复（done+select 模式、writerWaiting 检查、incUpgradeRequest 位置调整），但新增的 4 个测试有逻辑错误，导致测试失败。核心问题：**TryUpgrade 必须由持有读锁的 goroutine 调用**，而之前测试中从另一个 goroutine 调用 TryUpgrade 导致注册表找不到持有者。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
修复思路：测试中 TryUpgrade 必须由持有读锁的同一 goroutine 调用。正确的设计是让升级 goroutine 自己获取读锁再调用 TryUpgrade，而主 goroutine 持有另一个读锁使升级阻塞等待，从而产生 `writerWaiting=true` 的时间窗口。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在修复 `TestRLockTimeoutDuringWriterWaiting`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rwlocker\rwlocker_test.go
          
changes: undefined
          
现在运行测试看看是否全部通过：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/rwlocker/ -v -count=1 -timeout 60s undefined
          
全部 43 个测试通过！现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
现在更新文档，添加关于三个 bug 修复的设计要点：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
现在更新超时检测部分，说明修复后的实现方式：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
现在在锁升级流程中也更新 `writerWaiting` 广播和 `RLock` 阻塞的行为描述：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
现在在文档中新增一个章节，专门说明写者饥饿防护机制：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
需要修正重复的小节编号：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: search_by_regex
            
status: success
          
query: ### 5\.2
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
现在修正后续章节编号：
        
            
toolName: search_by_regex
            
status: success
          
query: ^## [0-9]
          
第6章出现了重复，需要将后续章节重新编号（7→8, 8→9）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
还需要修正子章节编号：
        
            
toolName: search_by_regex
            
status: success
          
query: ### [67]\.
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\rwlocker.md
          
changes: undefined
          
文档更新完成，现在标记任务完成：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有任务已完成！以下是修复总结：

---

## 修复总结

### Bug 1: RLock/Lock 超时路径 goroutine 泄漏
**问题**：使用 `AfterFunc` + 缓冲通道模式时，锁获取成功后超时回调 goroutine 向通道发送 `TimeoutError`，但因无人读取而永久阻塞。

**修复**：改用 `done chan struct{}` + `time.NewTimer` + `select` 模式。超时后启动清理 goroutine 等待 `<-done` 然后释放锁，确保无 goroutine 泄漏。

### Bug 2: TryUpgrade 阻塞模式写者饥饿
**问题**：升级 goroutine 设置 `writerWaiting` 后，`RLock` 和 `TryRLock` 入口未检查该标记，新读者持续进入导致 `readerCount` 无法归零，升级无限等待。

**修复**：
- `RLock` 无超时路径：增加 `waitForWriterWaiting()` 方法，在 `upgradeCond` 上等待直到 `writerWaiting` 被清除
- `RLock` 有超时路径：后台 goroutine 同样先等 `writerWaiting`，超时清理时回退 `readerCount` 并 `Broadcast`
- `TryRLock`：在 `upgradeMu` 保护下检查 `writerWaiting`，为 true 直接返回 `(false, nil)`
- 升级成功/超时时均调用 `upgradeCond.Broadcast()` 唤醒阻塞的 RLock

### Bug 3: incUpgradeRequest 统计不准
**问题**：`incUpgradeRequest` 在校验当前 goroutine 是否持有读锁之前调用，无效升级请求也被计入。

**修复**：将 `incUpgradeRequest()` 移到读锁持有校验通过之后，仅有效请求计入统计。

### 测试结果
全部 **43 个测试** 通过（含新增的 5 个验证测试）。

### 文档更新
[docs/rwlocker.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/rwlocker.md) 已更新：
- 超时检测部分增加实现机制说明（done+select 模式）
- 阻塞升级流程图更新（含 writerWaiting 检查和 Broadcast 步骤）
- 新增第 5 章"写者饥饿防护机制"，详述防护策略和流程
- 关键设计要点补充写者饥饿防护和统计准确性说明