我来分析并修复这些问题。让我先仔细查看相关代码：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我仔细阅读代码理解问题所在：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
现在我理解了问题。让我详细分析并修复：

**问题 1 分析**：当前代码中：
- `nodeExpiries` 记录为 `start.Add(ttl)`（假设所有节点都在 start 时刻加锁成功）
- 然后 `minExpiry` 再减去 `elapsed + ClockDrift`
- 在加锁完成时刻 `now = start + elapsed`，实际剩余时间 = `(start+ttl - elapsed - drift) - (start+elapsed) = ttl - 2*elapsed - drift`
- 这导致 elapsed 被扣除了两次！

**问题 2 分析**：`ErrReentrantNotAllowed` 定义了但从未使用，应该删除。

让我修复代码：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
现在让我修复这两个问题。先修复 attemptLock 和 Heartbeat 方法的 TTL 双倍扣除问题，同时删除未使用的错误变量：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
changes: undefined
          
现在修复 attemptLock 方法的 TTL 双倍扣除问题：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
changes: undefined
          
让我去掉未使用的 start 变量，然后修复 Heartbeat 方法：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
changes: undefined
          
现在修复 Heartbeat 方法的同样问题：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          
changes: undefined
          
现在让我更新任务状态并运行测试：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先运行测试确认代码修复正确：










        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock ; go test -v -count=1 undefined
          
所有 38 个测试全部通过！现在让我完善文档，补充更详细的 Redlock 加锁流程说明和完整可运行示例：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\distlock.md
          
让我完整重写文档，更新流程说明并添加完整可运行的示例：













        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\distlock.md
          
changes: undefined
          
文档已更新完成。让我再运行一次测试确保所有修改正确：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock ; go test -v -count=1 undefined
          
所有 38 个测试全部通过！让我完成最后的状态更新：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复完成总结

已成功修复所有问题，所有 38 个测试全部通过 ✅。以下是修复详情：

### 问题 1：Redlock TTL 双倍扣除时钟漂移

**原因分析**：旧实现中：
- `nodeExpiries` 统一记录为 `start.Add(ttl)`（假设所有节点都在 `start` 时刻加锁成功）
- 然后 `minExpiry` 再减去 `elapsed + ClockDrift`
- 导致实际剩余时间 = `(start+ttl - elapsed - drift) - (start+elapsed) = ttl - 2*elapsed - drift`，**elapsed 被扣除了两次**

**修复方案**（文件：[distlock.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distlock/distlock.go)）：
- `attemptLock` 和 `Heartbeat` 方法中，每个节点加锁/续期成功后，立即使用 `time.Now().Add(ttl)` 记录该节点的**实际**过期时间
- 最终计算全局有效时间 `Expiry` 时，仅扣除一次 `ClockDrift`，不再减 `elapsed`
- 加锁耗时已自然反映在各节点的实际过期时间中，不会再出现双倍缩短

### 问题 2：未使用的错误变量

**修复**：从 [distlock.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/distlock/distlock.go#L10-L23) 中删除了定义但未使用的 `ErrReentrantNotAllowed` 错误变量。

**注意**：重入计数上限逻辑本身是正确的——`LockManager.lockInternal` 在达到 `MaxReentrancy` 时正确返回 `ErrMaxReentrancy`，对应测试用例 `TestLockManager_MaxReentrancy` 已验证通过。

### 问题 3：文档完善

**文件**：[distlock.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/distlock.md)

补充内容：

1. **Redlock 加锁流程步骤级详解**（含时间轴示例）：
   - 使用 3 节点、TTL=10s、ClockDrift=50ms 的具体数值示例
   - 标注每个时刻 T=0ms/10ms/20ms 的操作与 NodeExpiries 值
   - 明确说明"精确节点过期时间"、"单次漂移扣除"、"持有时间一致性"三大设计要点

2. **Redlock 解锁流程步骤级详解**：参数校验 → 广播释放 → 结果判定

3. **Redlock 心跳续期流程步骤级详解**

4. **4 个完整可运行的使用示例**（含 `package main` 和 `func main()`，可直接运行）：
   - 示例 1：单节点锁基本使用 + 重入计数（含递归演示）
   - 示例 2：5 节点 Redlock 完整流程（加锁 → 业务处理 → 心跳续期 → 解锁 → 验证）
   - 示例 3：锁过期与心跳续期（后台心跳协程演示）
   - 示例 4：TryLock 非阻塞模式与 5 worker 并发竞争

### 测试验证

```
PASS
ok      solocoder-go/internal/distlock  0.923s
```
38 个测试用例全部通过，涵盖正常流程、边界条件、异常分支和并发场景。