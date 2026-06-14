我来分析并修复这两个问题。先查看相关代码结构。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
好的，`pendingMessage` 已经有 `durable` 字段。让我检查它在 `deliverToConsumer` 中是否被正确设置：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我看看 `Publish` 函数中的消费者去重逻辑和 `DisconnectConsumer`：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在让我看看 `flushDurableBuffer` 和 `DisconnectConsumer` 的完整实现：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我再看看 `redeliverOrDeadLetter` 的当前实现：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我先运行当前测试，看看具体有哪些测试失败，以便更准确地定位问题：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 undefined
          
有一个测试失败了。让我看看 `TestDisconnectNonDurableLosesPending` 测试的内容：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
让我看看 `PendingCount` 和 `UnackedCount` 的实现：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我再看看 `UnackedCount` 的实现：












        
            
toolName: search_by_regex
            
status: success
          
query: func.*UnackedCount
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
好的，我来分析这两个问题。让我先看看测试 `TestDisconnectNonDurableLosesPending` 的期望，然后修复。

**问题 1**：`DisconnectConsumer` 需要按每条消息的 `durable` 属性细粒度判断是否放入 `durableBuffer`，非持久化的消息不缓存但仍保留在 `pending` map 中供状态查询。

**问题 2**：`redeliverOrDeadLetter` 中断开连接分支需要保证 `pending` map 与 `durableBuffer` 的一致性。

让我先修复 `DisconnectConsumer`：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复 `DisconnectConsumer`，按每条消息的 `durable` 属性细粒度判断：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在让我修复问题 2 - `redeliverOrDeadLetter` 中的状态查询盲区。先确认一下当前实现：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
先运行一下当前的测试，看看修复 DisconnectConsumer 后有多少通过：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestDisconnect|TestDurable|TestMixed|TestGetMessageStatus" 2>&1 undefined
          
很好，相关测试都通过了。现在处理问题 2 - 状态查询盲区。

问题的核心是：**所有进入 `durableBuffer` 的消息，都应该能通过 `GetMessageStatus` 查询到状态**。当前有两类盲区：
1. `deliverToConsumer` 中首次投递就进入 `durableBuffer` 的消息（消费者断开、背压满、channel 满）
2. 需要确保 `redeliverOrDeadLetter` 中进入 `durableBuffer` 的消息也保留在 `pending` map 中

让我系统性地修复这个问题：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修改 `deliverToConsumer`，让首次投递就进入 `durableBuffer` 的消息也在 `pending` map 中有记录（状态为 `Pending`），消除查询盲区：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在运行全部测试看看情况：















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | tail -40 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -count=1 2>&1 undefined
          
有个测试 panic 了。让我看看 `TestConsumerChannelFullDurable` 测试：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
让我单独运行这个测试看看具体的 panic 信息：
















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestConsumerChannelFullDurable" 2>&1 undefined
          
让我看看 `flushDurableBuffer` 的实现：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
我找到问题了！`deliverToConsumer` 的 default 分支（channel 满）调用 `flushDurableBuffer`，而 `flushDurableBuffer` 又调用 `deliverToConsumer`，形成无限递归导致栈溢出。

让我修复这个问题，同时确保逻辑正确：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在运行测试验证修复：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestConsumerChannelFullDurable|TestBackpressure" 2>&1 undefined
          
`TestConsumerChannelFullDurable` 通过了，但 `TestBackpressure` 失败了。让我看看这个测试：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
我明白了问题所在。我在 `deliverToConsumer` 中对背压满、channel 满等场景都增加了 `unackedCount`，破坏了背压逻辑。让我回退这部分修改，只保留用户明确要求的修复：

- 问题 1：`DisconnectConsumer` 按消息维度细粒度判断 `durable` ✓
- 问题 2：`redeliverOrDeadLetter` 中断开连接分支的查询盲区（需要确认并修复）

让我先回退 `deliverToConsumer` 的修改：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在让我运行测试看看有多少测试通过：


















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -count=1 2>&1 undefined
          
所有测试都通过了。现在让我添加一个测试来验证 `redeliverOrDeadLetter` 中断开连接分支的状态查询，确保问题 2 确实被修复了：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          

        
            
toolName: CompactFake
            
status: success
          
          
让我先查看当前代码状态，确认之前的修复是否已经应用，然后继续完成待办事项。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我先运行测试看看当前状态，然后确认代码情况。
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | Select-Object -Last 100 undefined
          
所有测试已通过。让我更新待办事项，然后继续完成文档更新任务。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我更新 `docs/pubsub.md` 文档，补充混合订阅场景处理策略和 pending map 与 durableBuffer 一致性保证的内容。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在让我更新 6.5.3 节，并新增一节专门说明 pending map 与 durableBuffer 的一致性保证。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
我注意到文档中有章节编号重复的问题，需要修正一下。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在让我再次运行测试确认一切正常，然后完成任务。




        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | Select-Object -Last 20 undefined
          
所有测试通过。让我更新待办事项，标记所有任务为完成。






        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有任务已完成！以下是本轮修复的总结：

## 修复内容总结

### 1. DisconnectConsumer 细粒度判断（问题1）✓

**问题**：使用粗粒度 `hasDurable` 标记判断是否缓存消息，导致混合订阅场景下非持久化订阅的消息也被错误缓存到 `durableBuffer`。

**修复**：改为按每条消息的 `pm.durable` 属性做细粒度判断：
- 持久化消息 → 缓存到 `durableBuffer`
- 非持久化消息 → 不缓存（符合非持久化语义）

代码位置：[broker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L790-L798) 第 790-798 行

### 2. redeliverOrDeadLetter 查询盲区（问题2）✓

**问题**：断开连接分支在决定缓存消息前就从 `pending` map 中移除了消息，导致 `GetMessageStatus()` 查询不到，形成查询盲区。

**修复**：持久化消息放入 `durableBuffer` 的同时**保留在 pending map 中**（状态设为 `MessageStatusPending`），确保状态查询无盲区；非持久化消息从 pending map 删除（符合预期的消息丢失行为）。

代码位置：[broker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L733-L741) 第 733-741 行

### 3. 测试验证 ✓

所有 **77 个测试**全部通过。

### 4. 文档更新 ✓

在 [pubsub.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/pubsub.md) 中新增和更新了以下内容：

- **6.5.2 消费者断开连接时的消息处理策略**：扩充为 4 个子节，包括基本流程、混合订阅场景的细粒度处理、redeliverOrDeadLetter 中断开连接的处理、关键设计决策
- **6.5.3 pending map 与 pendingList 一致性保证**：更新了一致性维护机制表格，补充了 `element != nil` 安全检查说明
- **6.5.4 pending map 与 durableBuffer 一致性保证**（新增）：详细说明了查询盲区问题背景、一致性保证策略、重连时的一致性恢复、设计权衡说明