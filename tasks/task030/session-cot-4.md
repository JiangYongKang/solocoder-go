让我分析这 4 个问题并制定修复方案。首先创建任务计划：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先让我阅读关键代码：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
让我查看 reliableBroadcast 和 handleLockGranted 的实现：





        
            
toolName: search_by_regex
            
status: success
          
query: func \(c \*Cluster\) reliableBroadcast
          

        
            
toolName: search_by_regex
            
status: success
          
query: func \(n \*Node\) handleLockGranted
          

        
            
toolName: search_by_regex
            
status: success
          
query: type releaseMark struct
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
好的，现在开始修复。先处理问题 1：将 Grant/Deny 回复改用 reliableUnicast，并移除 Lock 上层重试机制：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在修改 handleLockAcquire，将 Grant/Deny 回复改用 reliableUnicast：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题 2：reliableBroadcast 改回串行发送，保证因果顺序：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题 3：releaseMark 统一使用消息 Timestamp 作为时间源，避免双重时间源矛盾。将 expiresAt 改为基于消息 Timestamp 计算：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在修改 handleLockRelease，让 expiresAt 基于消息的 Timestamp 计算，而不是接收端的 time.Now()：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在需要修改 cleanupExpiredReleaseMarks，确保它也使用一致的逻辑。不过这里需要说明：timestamp 和 expiresAt 都基于发送端时间源（msg.Timestamp），而清理时使用接收端的 time.Now() 只是做垃圾回收，不会影响正确性判断。让我确认这个函数没问题，然后继续处理问题 4。

现在修复问题 4：为状态机单向转换增加显式注释和断言。先看看状态设置的位置：









        
            
toolName: search_by_regex
            
status: success
          
query: pl\.status = pendingLockStatus
          

        
            
toolName: search_by_regex
            
status: success
          
query: pendingLockStatus
          
让我看看状态机定义和状态设置的完整代码：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
现在修复问题 4：为状态机单向转换增加显式注释和断言。首先在状态机定义处增加详细注释：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在在两处状态设置的地方使用断言函数：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在在 handleLockGranted 中增加注释，说明双重检查依赖状态机单向转换的不变式：














        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在运行测试，验证所有修复是否正确：
















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cachesync/ -count=5 -v undefined
          
太棒了！所有测试连续 5 次全部通过。现在更新文档，说明消息发送的可靠性分级策略：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先读取当前文档中需要更新的部分：






















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cachesync.md
          
现在更新文档。首先移除 Lock 自动重试的章节，因为已经移除了上层重试：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cachesync.md
          
changes: undefined
          
现在更新 4.3 节，修改消息传递可靠性分级策略：































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cachesync.md
          
changes: undefined
          
现在完成所有任务：
































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复已完成！以下是本次修复的详细总结：

## 修复内容总结

### 问题 1：Lock 上层重试掩盖协议层不可靠
**根因**：Grant 和 Deny 回复仍使用普通 `unicast` 非阻塞发送，通道满时被静默丢弃；而 Lock 方法通过上层 `tryLock` 重试掩盖了这个协议层问题。

**修复**：
- [handleLockAcquire](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L638-L682) 中 Grant 和 Deny 回复从 `unicast` 改为 `reliableUnicast`
- 移除了 Lock 方法的上层 `tryLock` 重试机制，将 [Lock](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L935) 简化为直接执行，可靠性统一在协议层解决

### 问题 2：reliableBroadcast 并发发送导致因果顺序不可控
**根因**：reliableBroadcast 使用多个 goroutine 并发向各节点发送，调度时序不可控，破坏了发送端的因果顺序保证。

**修复**：
- [reliableBroadcast](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L855-L890) 从并发 goroutine 发送改回串行发送，按节点顺序逐个可靠传递，保证因果顺序由发送端串行化控制

### 问题 3：releaseMark 双重时间源矛盾
**根因**：消息自带的 `Timestamp`（发送端设置）和 `expiresAt`（接收端 `time.Now` 设置）是两套时间源，节点时钟偏差时可能判断矛盾。

**修复**：
- 定义 `releaseMarkTTL = 5 * time.Second` 常量
- [handleLockRelease](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L700-L719) 中 `expiresAt` 改为基于消息 Timestamp 计算：`msg.Timestamp.Add(releaseMarkTTL)`，统一使用发送端时间源

### 问题 4：状态机单向转换不变式未显式表达
**根因**：handleLockGranted 的双重检查模式依赖于状态机不会从 Failed/Succeeded 回退到 Pending 这一隐含不变式，但代码中没有显式注释或断言。

**修复**：
- 新增 [assertStatusTransitionValid](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L100-L104) 断言函数，在状态转换处强制检查 `Pending → Succeeded/Failed` 的单向转换
- 两处状态设置点（[startRollbackWatcher](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L924-L928) 和 [Lock](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L1018-L1020)）均加入断言调用
- 在 [handleLockGranted](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L514-L521) 开头增加详细注释，说明双重检查依赖的状态机不变式及断言保障

### 文档更新
在 [cachesync.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cachesync.md) 中更新了：
- 4.2.4 节：状态机不变式与断言说明
- 4.3 节：消息传递可靠性分级策略表格，明确区分各消息类型的可靠性级别及其原因
- 4.3.4 节：释放标记时间源一致性设计原则

### 验证结果
`go test ./internal/cachesync/ -count=5 -v` 所有测试连续 5 次稳定通过。