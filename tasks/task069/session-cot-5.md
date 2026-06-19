让我先查看当前代码状态，制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: snapshotInFlight|snapshotInstalling|func init|atomic\.
          

        
            
toolName: search_by_regex
            
status: success
          
query: blockingStateMachine|snapshotInFlight|snapshotInstalling
          
让我读取更多相关代码细节，确保修改正确。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
让我看看 Propose 的完整流程，理解锁的范围。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
现在开始修复。先处理第 1 和第 2 个问题：改为原子计数器并清理残留代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
修改 `raftlog.go` 中的字段类型和操作，使用 `atomic` 包：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
修改 `sendHeartbeats` 中的自增自减为 `atomic` 操作：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
修改 `Propose` 方法，使用 `atomic.LoadInt32` 无锁读取：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
现在移除空的 `init()` 函数：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
现在更新任务状态，然后重构 `blockingStateMachine` 和测试。









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先检查测试文件的 import 情况，然后重写 `blockingStateMachine` 和测试。











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
现在重写 `blockingStateMachine` 和 `TestSnapshotInstallingError`。先添加 import，再替换结构体和测试：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
changes: undefined
          
现在替换 `blockingStateMachine` 为支持多并发的版本：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
changes: undefined
          
现在替换整个 `TestSnapshotInstallingError` 测试函数：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
现在用全新的测试替换整个 `TestSnapshotInstallingError` 函数：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog_test.go
          
changes: undefined
          
先编译检查一下是否有语法错误：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/raftlog/ 2>&1 undefined
          
编译通过！现在运行测试：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/raftlog/ -v -count=2 -timeout 90s 2>&1 undefined
          
让我检查命令输出：
















        
            
toolName: check_command_status
            
status: success
          
          
输出为空但退出码为 0。让我换一种方式运行测试：












        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/raftlog/ -count=2 -timeout 90s 2>&1 | Out-String undefined
          
让我先运行单个测试来验证输出：














        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/raftlog/ -v -run TestSnapshotInstallingError -count=1 -timeout 30s undefined
          
有死锁问题。让我查看 `sendHeartbeats` 的完整逻辑，找出问题：
















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
我发现问题了！快照路径的 return 之前可能缺少 `Unlock`。让我检查 `handleInstallSnapshotReply` 的实现：


















        
            
toolName: search_by_regex
            
status: success
          
query: func \(n \*RaftNode\) handleInstallSnapshotReply
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
找到死锁原因了！`handleInstallSnapshotReply` 内部自己会调用 `n.mu.Lock()`，但快照路径在调用前已经持有锁，导致重入死锁。让我修复：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
其实 `snapshotInFlight` 改用 atomic 后，尾部的 `n.mu.Lock()` 已经不需要了（原来只是为了保护计数器）。让我进一步简化代码：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\raftlog\raftlog.go
          
changes: undefined
          
现在运行测试看看死锁是否解决了：


















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/raftlog/ -v -run TestSnapshotInstallingError -count=1 -timeout 30s undefined
          
完美！快照相关测试通过了。现在运行完整测试，连续两次：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/raftlog/ -v -count=2 -timeout 90s undefined
          
所有测试连续两次全部通过！让我完成任务：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复完成！以下是详细修复内容：

## 修复总结

### 1. `snapshotInFlight` 改为 `int32` + `atomic` 原子操作
**文件**: [raftlog.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/raftlog/raftlog.go)
- 字段类型: `int` → `int32`
- 自增: `atomic.AddInt32(&n.snapshotInFlight, 1)`
- 自减: `atomic.AddInt32(&n.snapshotInFlight, -1)`
- **Propose 双重检查**: 锁外 `atomic.LoadInt32` 快速失败 + 锁内二次确认
- **修复死锁 bug**: 原快照路径在持锁调用 `handleInstallSnapshotReply`，而该函数内部又会拿锁导致重入死锁。修复后先解锁再调用。

### 2. 移除 `init()` 空函数
**文件**: [raftlog.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/raftlog/raftlog.go)
- 删除 `func init() { _ = atomic.Int32{} }` 占位代码已移除
- `sync/atomic` 包现在被正确使用

### 3. 重构 `TestSnapshotInstallingError` 测试
**文件**: [raftlog_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/raftlog/raftlog_test.go)
- 移除 `DirectCounterTest` 和 `ConcurrentSnapshotCounterTest`（直接赋值的伪测试）
- 新增三个真实路径子测试：
  - **`SingleSnapshotBlocksPropose`: 单 Follower 触发单快照
  - **`MultipleConcurrentSnapshots`**: 3 个 Follower 触发 3 并发快照
  - **`SnapshotReleaseAllowsPropose`**: 快照释放后 Propose 恢复正常
- 新增辅助函数 `setupSnapshotTestLeader` / `setupSnapshotTestFollower`

### 4. 重写 `blockingStateMachine` 并发协调机制
**文件**: [raftlog_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/raftlog/raftlog_test.go)
- 用 `int32` 原子计数器跟踪 in-flight 快照数
- `EnableBlock() 开启阻塞，ReleaseAll() 通过关闭 channel 一次性释放所有 goroutine
- WaitForInFlight(target, timeout) 轮询等待指定数量快照进入阻塞

### 5. 测试验证
```
go test ./internal/raftlog/ -v -count=2 -timeout 90s
```
**50 个测试连续两次全部通过 ✅