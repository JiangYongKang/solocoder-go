让我先查看 `internal/servicereg/` 下的代码，了解当前的实现。
        
            
toolName: file_search
            
status: success
          
file_pattern: internal/servicereg/**/*.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\servicereg.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
我已经完整阅读了代码和文档。现在让我创建任务计划来修复这些问题。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在开始修复。首先修复 `expiryLoop` 中 defer 的顺序问题——先设置 `expiryRunning = false` 再调用 `wg.Done()`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在修复 `expireInstances`：在删除实例前先构建过期前快照，这样 expire 事件的 `Instances` 字段包含的是删除前的完整实例列表。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在重写测试文件中的三个问题测试。让我先处理 `TestMultiServiceExpireNotificationAtomic`（重写为真正验证多服务通知一致性的测试）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
现在重写 `TestConcurrentStartStop`，补充对后台协程启动次数和 `expiryRunning` 状态一致性的验证：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在更新 `TestExpireNotificationSnapshot` 以匹配新的快照语义——expire 事件包含删除前的实例列表（包含即将过期的实例）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
现在还需要更新 `TestMultiServiceExpireNotificationConsistency` 测试中的断言，因为它也检查 expire 事件的 `Instances` 内容。在新语义下，expire 事件包含的是过期前的快照，所以 `inst-c3` 不应该出现在 `svc-a` 或 `svc-b` 的过期事件中（它属于 `svc-c`），这个逻辑仍然正确。但让我确认 `TestExpiryNotifiesSubscribers` 中的 `expire` 事件也需要更新——在新语义下，过期事件的 `Instances` 包含的是过期前的所有实例。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
这个测试只检查了 action 类型，不受影响。让我也看看 `TestExpireWithSubscriberNotification` 和 `TestEventInstanceCount`：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
这个测试也只检查 action 类型，不受影响。现在让我检查 `TestMultiServiceExpireNotificationConsistency` 中的 `inst-c3` 断言是否仍然有效：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
`TestMultiServiceExpireNotificationConsistency` 也需要更新。在新语义下，`inst-c3` 如果恰好在扫描时刻存在，它就会出现在 svc-c 的过期前快照中。测试应该验证跨服务污染不会发生（svc-a 的过期事件不应包含 svc-c 的实例）。让我更新这个测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
现在让我也检查是否还有其他测试需要更新，特别是 `TestEventInstancesAfterDeregister`，它检查 deregister 事件的 Instances 数量：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
这个测试检查的是 deregister 事件（不是 expire 事件），deregister 仍然使用删除后的快照，所以这个测试不需要修改。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在运行带竞态检测的测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v -race 2>&1 | Out-String undefined
          
`-race` 标志在 windows/386 上不支持。让我先不带 `-race` 运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v 2>&1 | Out-String undefined
          
编译错误：`instID` 声明但未使用。这是因为我修改 `expireInstances` 后，第一个扫描循环中用了 `break` 但仍保留了 `instID` 变量。让我修复：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          
changes: undefined
          
再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v 2>&1 | Out-String undefined
          
`TestExpireNotificationSnapshot` 测试失败了。问题是 `inst-a3` 在 t=120ms 注册后也会在 t=250ms 左右过期，产生了第二个 expire 事件覆盖了 `lastEvent`。让我重新设计这个测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg_test.go
          
changes: undefined
          
再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/servicereg/ -v 2>&1 | Out-String undefined
          
所有 55 个测试全部通过！现在更新文档。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在更新文档。我需要修改三个关键部分：
1. Start/Stop 生命周期管理中关于 defer 顺序的说明
2. 通知一致性保证中关于过期前快照的说明
3. ServiceChangeEvent 的 Instances 字段说明
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\servicereg.md
          
现在更新文档，修改三个关键部分：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\servicereg.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\servicereg.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\servicereg.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
所有任务已完成。下面是修复总结：

## 修复内容

### 1. expiryLoop defer 顺序修正 — [servicereg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg.go#L355-L361)

**问题**：`expiryLoop` 的 defer 中先执行 `wg.Done()` 后设置 `expiryRunning = false`，导致 `Stop()` 的 `wg.Wait()` 在 `Done()` 后立即返回，但 `expiryRunning` 仍为 `true`。紧接着调用 `Start()` 时会误判过期协程已在运行而拒绝启动。

**修复**：调整为先设置 `expiryRunning = false`（在锁保护下），再调用 `wg.Done()`。这确保 `Stop()` 返回时 `expiryRunning` 状态已正确更新。

### 2. expireInstances 过期前快照 — [servicereg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg.go#L376-L431)

**问题**：原实现先删除过期实例，再调用 `buildEventLocked` 构建事件，导致 expire 事件的 `Instances` 字段是删除后的存活列表。当某服务全部实例过期时，`Instances` 为空，订阅者无法获知哪些实例被逐出。

**修复**：重构 `expireInstances` 为三阶段：
1. **过期扫描**：标记有过期实例的服务
2. **快照收集**：在删除前为每个变更服务构建过期前实例列表快照
3. **实例删除**：快照收集完成后再删除过期实例

### 3. 重写 TestMultiServiceExpireNotificationAtomic — [servicereg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg_test.go#L1698-L1772)

**问题**：原测试定义了 `atomicViolation` 变量但从未递增，断言始终为 `true`，是虚假测试。

**修复**：重写为真正验证多服务通知一致性的测试——注册 svc-a 和 svc-b 的实例，等待过期后验证：
- 每个服务都收到了 expire 事件
- expire 事件的 `Instances` 包含过期前的实例（pre-deletion snapshot）

### 4. 重写 TestConcurrentStartStop — [servicereg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg_test.go#L1560-L1591)

**问题**：原测试仅验证没有 panic 就退出，未检查 `expiryRunning` 状态一致性。

**修复**：并发调用 `Start()` 后，验证 `expiryRunning == true`；调用 `Stop()` 后，验证 `expiryRunning == false`。

### 5. 更新 TestExpireNotificationSnapshot — [servicereg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/servicereg/servicereg_test.go#L1803-L1868)

**修复**：适配新的过期前快照语义——expire 事件应包含即将被删除的过期实例，不应包含扫描后注册的新实例。使用 channel 同步确保先收到第一个 expire 事件后再注册新实例。

### 6. 文档更新 — [servicereg.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/servicereg.md)

- `ServiceChangeEvent.Instances` 字段说明更新为"register/deregister 为变更后状态，expire 为删除前状态"
- Start/Stop 关键保证新增第 5 条：`expiryLoop` 协程退出时先设置 `expiryRunning = false` 再调用 `wg.Done()`
- 通知一致性保证更新为四阶段流程，明确说明过期前快照的语义

### 测试结果

55 个测试全部通过 ✓（注：`-race` 标志在 windows/386 上不支持，已在不带 `-race` 的模式下验证通过）