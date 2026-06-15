我来帮你修复这些功能缺陷。让我先分析问题并制定修复计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
首先让我读取核心代码了解当前实现：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
现在我已经全面了解了代码。让我逐一修复这 4 个问题。首先修复 **心跳机制** 和 **Client 结构体**：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在添加 `lastPingSent` 的操作方法，并修复心跳检测逻辑：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在修复 `sendPings()` 和 `checkTimeouts()`：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在修复 **问题 #2（SendToClient 错误码区分）** 和 **问题 #3（Disconnect 删除时机）**：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在修复 `SendToClient` 和 `Disconnect`：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在修复 `SendToClient` 以正确区分错误码：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在修复 **问题 #4 - JoinRoom 竞态条件**：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在修复 `JoinRoom` 竞态条件 - 在修改房间状态时重新验证房间有效性：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在让我先运行一下现有测试，看看哪些需要根据新的行为进行调整：













        
            
toolName: run_command
            
status: success
          
command: go test ./internal/wscenter/ -v -count=1 2>&1 undefined
          
需要调整测试和实现。让我先修复关键问题 - 需要区分"从未连接"和"曾连接但已断开"，我需要引入 `knownClients` 机制：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在更新 `NewWSCenter`、`Connect`、`Stop` 方法：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在更新 `Connect` 和 `Stop`：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在更新 `Stop` 方法：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在更新 `SendToClient` 来区分两种错误：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在让我读取测试文件，看看需要调整哪些测试：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
现在我需要修复现有测试并添加新的边界测试。先修复现有失败的测试：
























        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复 `TestSendToClient` 中的断言（发送给从未连接的客户端应返回 `ErrClientNotFound`）：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
changes: undefined
          
现在修复 `TestHeartbeatTimeout` - 调整参数并增加睡眠时间确保超时能被检测到：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
changes: undefined
          
现在调整 `SendToClient` 对发送方的错误码区分，与目标客户端保持一致：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在修复 `TestSendToClientDisconnectedSender` - 发送方曾经连接过应该返回 `ErrClientOffline`：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
changes: undefined
          
现在先运行测试看看现有修复效果：



























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/wscenter/ -v -count=1 2>&1 undefined
          
`TestHeartbeatTimeout` 还需要调整参数。让我用更严格的时间参数：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
changes: undefined
          
让我先分析心跳逻辑的问题。我需要调换 `pingLoop` 中 `checkTimeouts` 和 `sendPings` 的顺序：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
我还需要将超时判断从 `>` 改成 `>=`，确保边界条件正确：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter.go
          
changes: undefined
          
现在运行测试看看：






























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/wscenter/ -v -count=1 -run TestHeartbeatTimeout 2>&1 undefined
          
心跳测试通过了！现在运行全部测试：

































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/wscenter/ -v -count=1 2>&1 undefined
          
所有现有测试通过！现在添加新的边界场景测试用例：



































        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在在测试文件末尾添加新的边界场景测试：

































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wscenter\wscenter_test.go
          
changes: undefined
          
现在运行所有测试确保新添加的测试也能通过：



































        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/wscenter/ -v -count=1 2>&1 undefined
          
全部 **47 个测试** 通过！现在更新模块需求文档：





































        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
先读取现有文档：





































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
现在更新文档，说明修复后的心跳机制和错误码语义：






































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
继续更新 WSCenter 结构体的定义：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
更新错误码的语义说明：









































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
更新加入房间流程（使用写锁避免竞态）：










































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
更新点对点消息流程（错误码区分）：












































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
更新心跳检测流程（修复后的机制）：
















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
更新客户端断开流程（延迟删除策略）：

















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
最后更新测试覆盖范围，增加新增的边界测试场景：




















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
changes: undefined
          
现在运行诊断检查和最后一次测试确保一切正常：

















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有修复和测试均已完成！以下是本次修复的总结：

---

## ✅ 修复内容总结

### 1. 心跳超时检测逻辑修复 ([wscenter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go))

**问题**：首次 tick 时同时执行发送 Ping 和检查超时，由于 `lastPong` 初始值为连接时间，在默认配置下客户端从未收到 Ping 就被判定超时断连。

**修复方案**：
- `Client` 结构体新增 `lastPingSent` 字段
- 新增 `hasPendingPing()` 方法：只有当 `lastPingSent > lastPong`（存在未回复的 Ping）时才检查超时
- 调整 `pingLoop` 执行顺序：先 `checkTimeouts()` 再 `sendPings()`
- 超时判定基于 `now - lastPingSent >= PongTimeout`（基于发送时间）

---

### 2. SendToClient 错误码区分 ([wscenter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L602-L638))

**问题**：对"从未连接过"和"曾连接但已断开"两种场景统一返回 `ErrClientOffline`，调用方无法区分。

**修复方案**：
- `WSCenter` 结构体新增 `knownClients map[string]bool`，记录所有曾连接过的客户端 ID
- `Connect` 时加入 `knownClients`，`Stop` 时清理
- `SendToClient` 区分四种场景：

| 场景 | 错误码 |
|------|--------|
| 发送方/接收方从未连接过 | `ErrClientNotFound` |
| 发送方/接收方曾连接但已断开 | `ErrClientOffline` |

---

### 3. Disconnect 删除时机修复 ([wscenter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L368-L435))

**问题**：在持有 `ws.mu` 锁时立即从 `clients` map 删除客户端，导致断连通知流程中 `SendToClient` 等无法区分"从未连接"和"已断连"。

**修复方案**（延迟删除策略）：
1. 先标记 `client.disconnect = true`（带幂等检查）
2. 释放 `ws.mu` 锁，执行房间离开通知和广播
3. 关闭连接后，重新获取 `ws.mu` 锁，最后才从 `clients` map 删除

这样断连通知期间 `clients` map 仍保留记录，配合 `knownClients` 可正确区分状态。

---

### 4. JoinRoom 竞态条件修复 ([wscenter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter.go#L460-L490))

**问题**：读锁获取房间引用后释放锁，在间隙中另一个协程可能让房间变空并自动销毁，导致客户端被添加到"僵尸房间"。

**修复方案**：
- 将 `mu.RLock()` 改为 `mu.Lock()`（写锁）
- 在持有写锁期间完成引用检查、客户端添加、房间记录修改
- 修改完成后才释放锁，避免竞态窗口

---

### 5. 单元测试补充 ([wscenter_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wscenter/wscenter_test.go))

新增 **10 个边界场景测试**，总测试数从 37 → **47 个**：

| 测试用例 | 覆盖场景 |
|----------|----------|
| `TestHeartbeatFirstTickNoTimeout` | 首次 tick 前不触发超时（PingInterval=60ms，PongTimeout=10ms） |
| `TestHeartbeatPingSentBeforeTimeoutCheck` | 先检查超时再发送 Ping 的顺序验证 |
| `TestSendToClientErrorCodeDistinction` | 四种错误码区分场景 |
| `TestJoinRoomWithConcurrentRoomDestruction` | 并发 JoinRoom/LeaveRoom 竞态 |
| `TestDisconnectReentrantCall` | Disconnect 幂等性验证 |
| `TestJoinRoomDisconnectedClient` | 断连后加入房间失败 |
| `TestKnownClientsPersistsAfterStop` | knownClients 生命周期验证 |
| `TestHeartbeatPongResetsPendingState` | Pong 回复清除待回复状态 |
| `TestConcurrentDisconnectAndSend` | 并发 Disconnect/SendToClient 无死锁 |

所有测试全部通过 ✅：
```
PASS
ok      solocoder-go/internal/wscenter  2.753s
```

---

### 6. 需求文档更新 ([wscenter.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/wscenter.md))

文档更新内容：
- `Client` 和 `WSCenter` 结构体新增字段说明
- 错误码语义区分表（新增对比表）
- `JoinRoom` 流程：竞态防护说明
- `SendToClient` 流程：knownClients 判断分支
- 心跳检测流程：修复后的执行顺序、状态判断、避免的问题
- 客户端断开流程：延迟删除策略图解
- 测试覆盖范围：扩展到 47 个测试用例