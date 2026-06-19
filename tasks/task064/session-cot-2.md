好的，我来逐一修复这些问题。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先完整阅读当前的实现：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect.go
          
现在让我逐一修复所有问题。首先开始修复核心代码：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在让我重写核心文件，一次性修复问题 1-4：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect.go
          
changes: undefined
          
好的，核心代码修复完成（问题 1-4）。现在更新 todo 并添加心跳失败的测试用例：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我先读取现有测试文件，然后添加心跳失败场景的测试（由于 notify 改为异步，测试也要相应调整）：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
现在我需要重写测试文件，添加：
1. 一个可控制的 mock backend（用于模拟心跳失败）
2. doHeartbeat 心跳失败的场景测试
3. 由于 notify 是异步的，测试需要用 channel 来等待回调










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
现在运行测试：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=1 undefined
          
有两个测试问题需要修复：
1. 心跳失败测试中，虽然 heartbeat 返回错误，但锁还在，所以下一次 checkLeader 又把节点变回 Leader 了
2. 异步回调测试中，多个事件触发回调，重复 close channel

让我修复这两个测试：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
现在修复异步回调测试中的重复 close channel 问题：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
再次运行测试：











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=1 undefined
          
所有 25 个测试都通过了！让我连续运行几次确认稳定性：













        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -count=3 undefined
          
让我只运行 leaderelect 包的测试：















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -count=3 undefined
          
心跳失败测试偶发失败，因为虽然心跳和 GetHolder 返回错误，但 TryLock 还是成功的，导致 startElection 中又重新成为 Leader。让我修复：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
我需要添加一个 setTryLockFn 方法，然后在心跳失败测试中禁用 TryLock：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
现在修改心跳失败测试，同时禁用 TryLock：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
现在运行多次测试确保稳定：
















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -count=5 undefined
          
连续 5 次测试全部通过！现在更新文档：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
让我读取现有文档并进行更新：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\leaderelect.md
          
现在更新文档，主要更新 Follower 检查流程中对错误的处理说明，并确认回调异步执行的描述与代码一致：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\leaderelect.md
          
changes: undefined
          
继续更新 Follower 检查流程的描述：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\leaderelect.md
          
changes: undefined
          
继续更新并发安全部分，添加更多细节：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\leaderelect.md
          
changes: undefined
          
现在检查诊断：

















        
            
toolName: GetDiagnostics
            
status: success
          
          
完成！让我更新任务状态：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
所有问题已全部修复完成！以下是修复总结：

## 修复内容总览

### 1. notify 方法并发数据竞争修复 ✅
**问题**：`notify` 读取 `e.role`、`e.leaderID`、`e.term` 时只持有 `callbacksMu` 读锁，与 `doHeartbeat`/`startElection` 等方法形成并发读写竞争。

**修复**：在 [leaderelect.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/leaderelect/leaderelect.go) 的 `notify` 方法中，先持有 `e.mu` 读取共享字段的快照后释放锁，再构造 `ElectionEvent`。

### 2. checkLeader 错误处理修复 ✅
**问题**：只处理 `ErrLockNotHeld` 和 `ErrLockExpired`，其他错误静默跳过本周期，可能导致选举延迟。

**修复**：改为对所有错误类型都触发选举，采用保守策略避免漏检 Leader 宕机。

### 3. 回调改为异步执行 ✅
**问题**：回调在当前 goroutine 同步执行，阻塞心跳和 Leader 检查的下一个周期，与文档声明不符。

**修复**：使用 `go func()` 在独立 goroutine 中异步执行所有回调。

### 4. 移除无效 heartbeatCh 字段 ✅
**问题**：`heartbeatCh` 通道定义并初始化但从未使用。

**修复**：从 `LeaderElector` 结构体和构造函数中完全移除。

### 5. 新增心跳失败测试覆盖 ✅
**问题**：缺少 Leader 运行过程中心跳续约失败场景的测试。

**修复**：新增 3 个测试用例：
- `TestLeaderElector_HeartbeatFailure`：验证心跳失败触发 `EventLeaderLost` 和角色转换
- `TestLeaderElector_CheckLeaderBackendErrorTriggersElection`：验证后端任意错误触发选举
- `TestLeaderElector_CallbacksAsync`：验证回调异步执行不阻塞

### 6. 测试全部通过 ✅
连续 5 次运行 `go test ./internal/leaderelect/ -count=5`，所有 25 个测试全部 PASS。

### 7. 文档更新 ✅
在 [leaderelect.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/leaderelect.md) 中更新：
- **Follower → Candidate 触发条件**：补充"后端返回任何错误"都会触发选举
- **4.3 Follower 检查流程**：补充保守策略的错误处理说明
- **7. 并发安全**：详细描述 `notify` 的快照读取机制、回调异步执行设计、回调列表复制等实现细节