好的，我来逐一修复这些问题。首先让我创建任务计划并开始修复。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
首先让我重新读取当前代码，详细分析问题所在。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
现在我已经详细了解了代码结构。让我逐一修复这些问题。首先修复核心代码中的两个问题。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在修改 `handleLockGranted` 来记录已授予锁的节点：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在修改 `handleUpdateNotify`，让它返回错误并触发拒绝处理器，增加版本拒绝的可观测性：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在需要修改 `runReconciliation` 中直接调用 `handleUpdateNotify` 的地方，然后修改核心的 `Lock` 方法实现锁状态回滚机制：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在重写 `Lock` 方法，实现锁失败后的状态回滚机制：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在添加版本拒绝的公共 API，修改 Stats 并补充方法：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在先编译检查一下语法错误：








        
            
toolName: run_command
            
status: success
          
command: go build ./internal/cachesync/ undefined
          
核心代码修复成功！现在标记前两个任务完成，开始读取并补全文档。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cachesync.md
          
文档实际上是完整的，但我需要根据修复的内容进行更新。让我重写文档，补充锁回滚机制、版本可观测性相关的内容：

















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cachesync.md
          
changes: undefined
          
文档补全完成！现在更新任务状态并开始补充测试用例。先读取现有测试文件：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
现在修复现有测试中 `Stats()` 签名变化的问题，并补充新的测试用例：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
现在在测试文件末尾追加新的测试用例：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
测试代码已更新完成，现在运行测试验证：





















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -v undefined
          
测试失败的原因是**时序问题**：锁请求者（node2）可能先收到持锁者（node1）的 `MsgLockDenied`，此时 `grantedBy` 中还是空的（因为 node3 的 `MsgLockGranted` 还在路上），回滚什么都不做。随后 node3 的 `MsgLockGranted` 到达，但 `pendingLock` 已被清理，无法再回滚 node3 上的临时锁记录。

我需要修复这个问题：当锁获取失败后，用一个后台 goroutine 在短时间窗口内继续处理迟到的授予消息，并立即对新节点触发回滚。
















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
我需要修复锁回滚的时序问题。核心思路是：锁失败时不要立即删除 pendingLock，而是启动一个后台 goroutine 在一段时间窗口内继续处理迟到的 Grant 消息，自动回滚。


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在运行测试，验证修复后的锁回滚机制：



















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -v -run "TestLockRollback|TestVersionReject|TestMultiNode|TestAddNil|TestLockTimeoutRollback" undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -v 2>&1 undefined
          
测试仍然失败。让我深入分析问题。我怀疑是回滚窗口太短（当 Lock timeout=200ms 时，rollbackWindow 被限制为 200ms），或者消息调度延迟。让我修改 rollbackWindow 不跟随 timeout，并在测试中加入更好的诊断：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
让我写一个简单的诊断测试来直接验证时序问题。先调整测试，加入更合理的等待策略：因为 node3 授予的临时锁 TTL 很短，也许问题在于等待后锁已经自然过期而不是回滚。让我修改测试更精准地验证回滚效果，并且使用较长的锁 TTL：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
我找到根本原因了！问题是**测试逻辑错误**，不是代码问题。分析如下：

1. node1 成功获取锁时，node2 和 node3 本地都记录了临时锁 `{holder: node1}`（因为它们给 node1 授予过）
2. node2 尝试 Lock 时，在**本地检查**就发现有 `holder=node1` 的锁，直接返回错误，**根本不会广播**！所以 node3 不会为 node2 授予临时锁
3. 但测试却期望 node3 的 holder 为空——实际上 node3 正确保留了 `holder=node1` 的记录（TTL=30s，5秒内不会过期）

我需要完全重写这些测试，用正确的方式验证回滚效果：先让节点都无锁（node1 锁 TTL 自然过期），然后用短 TTL 场景验证。



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
现在修复最后一个四节点的回滚测试：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
现在运行完整测试验证修复效果：



























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -v undefined
          
发现了两个问题：

1. **panic: send on closed channel**：Cluster.Stop() 关闭了 inbox，但后台的 `startRollbackWatcher` goroutine 还在尝试向已关闭的 channel 发送消息。需要安全处理。
2. **测试失败**：C.Unlock() 后没等释放广播到达 B，B 就立即尝试获取锁了。

先修复代码中的 panic 问题，然后修复测试等待条件：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync.go
          
changes: undefined
          
现在修复测试中 Unlock 后缺少等待的问题：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cachesync\cachesync_test.go
          
changes: undefined
          
再次运行测试：




























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -v undefined
          
全部 42 个测试通过！再连续运行 3 次确认没有 flaky 测试：





























        
            
toolName: run_command
            
status: success
          
command: for ($i = 0; $i -lt 3; $i++) { Write-Host "=== Run $i ==="; go test ./internal/cachesync/; if ($LASTEXITCODE -ne 0) { Write-Host "FAIL Run $i"; break } } ; Write-Host "All runs OK" undefined
          
用 `-count=3` 强制真实运行确认稳定性：































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cachesync/ -count=3 -timeout 120s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有 5 个任务均已完成。以下是关键修复和验证结果总结：

---

## ✅ 问题修复总结

### 问题 1：锁拒绝后临时锁残留

**修复方案（三段式回滚）**：

1. **`pendingLock.grantedBy map[string]struct{}`** —— 在 `handleLockGranted` 中记录哪些节点已授予临时锁
2. **`rollbackLock(key, pl)` 同步回滚** —— Deny/超时分支立即调用，向 `grantedBy` 中所有节点单播 `MsgLockRelease`
3. **`startRollbackWatcher(key, pl, window)` 异步窗口** —— Deny 不代表所有 Grant 都已到达，启动后台 goroutine 在监视窗口（`max(1s, timeout)`）内持续扫描 `grantedBy`，对迟到的 Grant 立即再发送 Release，窗口结束后清理 `pendingLocks`

关键代码位置：
- [rollbackLock()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L718-L742)
- [startRollbackWatcher()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L744-L792)
- [Lock() 中 Deny/超时分支调用回滚](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L858-L870)

**额外修复**：[unicast()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L256-L287) 和 [broadcast()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L235-L261) 增加了 `Cluster.running` 检查 + `defer recover()`，避免测试结束时 `c.Stop()` 关闭 channel 后后台 watcher goroutine 发送消息导致 panic。

---

### 问题 2：ErrVersionTooOld 未使用 + 拒绝不可观测

**三层可观测性暴露**：

1. **返回值层**：`handleUpdateNotify` 从 `void` 改为返回 `error`，版本过低时返回包装了 `ErrVersionTooOld` 的详细错误（附带 key、本地版本、消息版本、来源节点），支持 `errors.Is(err, ErrVersionTooOld)` 判断
2. **回调层**：新增 `VersionRejectEvent` 结构体和 `VersionRejectHandler` 函数类型，`AddVersionRejectHandler()` 注册，调用时 `defer recover()` 隔离单个 handler panic
3. **统计层**：`Node.rejectCount` 原子计数器，`VersionRejectCount()` 方法获取，`Stats()` 返回值新增 `rejectCount uint64` 字段

关键代码位置：
- [handleUpdateNotify 返回 error + 触发拒绝](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L543-L568)
- [AddVersionRejectHandler + VersionRejectCount](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cachesync/cachesync.go#L942-L971)

---

### 问题 3：文档截断补全

[cachesync.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cachesync.md) 已扩充至 **1037 行**，包含完整 9 大章节：

- **新增结构体文档**：2.3 `VersionRejectEvent`、2.4 `VersionRejectHandler`、更新 2.8 `pendingLock`（新增 `grantedBy/grantedMu`）、2.9 `Node`（新增 reject 相关字段）
- **消息流转路径（第 3 章）**：
  - 3.1 版本化更新通知：明确拒绝分支触发回调+计数+返回错误
  - 3.3 分布式锁获取与回滚流程：完整 ASCII 流程图含回滚路径
  - 3.4 锁释放流程：新增独立流程图
  - 3.5 定期对账修复流程：说明对账调用 handleUpdateNotify 时同样触发拒绝事件
- **核心算法（第 4 章）**：4.2 节新增回滚流程 4 步伪代码
- **使用示例（第 5 章）**：5.2 版本拒绝可观测性 / 5.3 缓存行锁定与回滚（3 节点演示）/ 5.5 三节点对账修复（同 key 冲突）/ 5.6 并发写入监控拒绝事件 / 5.7 锁诊断
- **生产监控建议（9.4）**：Prometheus/Grafana 集成、告警阈值、拒绝率比值监控

---

### 问题 4-5：测试覆盖与验证

**新增 9 个测试用例**：
| 测试名 | 验证点 |
|--------|--------|
| `TestVersionRejectObservability` | 回调触发完整性、计数器递增、旧值不覆盖 |
| `TestLockRollbackAfterDenial` | 3 节点、Deny 触发回滚、无请求者残留临时锁、后续正常获取 |
| `TestLockTimeoutRollback` | 超时分支同样触发回滚 |
| `TestVersionRejectHandlerPanicSafety` | 第 1 个 handler panic 不影响第 2 个，不影响错误返回 |
| `TestMultiNodeReconciliationWithConflicts` | 4 节点+100% 丢包+同 key 不同版本→恢复后两轮对账完全收敛 |
| `TestAddNilVersionRejectHandler` | nil handler 安全忽略 |
| `TestLockRollbackWithFourNodes` | 4 节点回滚、C/D 均无请求者残留 |
| `setupCleanLocks`（辅助） | 测试初始化时清空所有节点锁记录 |

**测试断言修正说明**：原回滚测试断言 `GetLockHolder()==""` 错误，因为其他节点合法保存了真实持锁者（如 A）的记录。修正为 `holder=="" || holder=="A"`——只能是空或合法持锁者，**不能出现请求者（B）的残留记录**。

---

### ✅ 最终验证

```
=== 共 42 个测试 ===
go test ./internal/cachesync/ -v              → PASS (1.197s)
go test ./internal/cachesync/ -count=3        → PASS (3.133s, 连续 3 次稳定通过)
```