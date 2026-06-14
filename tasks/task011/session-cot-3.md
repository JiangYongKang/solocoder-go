我来修复这两个新引入的问题。先创建任务清单并查看相关代码。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先查看 DisconnectConsumer 和 redeliverOrDeadLetter 的当前实现：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在我清楚地看到了两个问题。先修复 `DisconnectConsumer`：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在修复第二个问题 - `redeliverOrDeadLetter` 中断开连接分支的消息丢失：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复 `redeliverOrDeadLetter` 的断开连接分支：













        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
          
让我重新读取这个函数的精确内容：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在修改 `redeliverOrDeadLetter` 的断开连接分支，同时我需要添加一个辅助方法 `hasDurableSubscription`：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
先添加 `hasDurableSubscription` 辅助方法，然后修复 `redeliverOrDeadLetter`：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在运行测试验证修复：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先编译检查，然后运行测试：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ 2>&1 undefined
          
编译通过，现在运行全部测试：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 undefined
          
有两个测试失败了，让我先看一下这两个测试的内容：
















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
让我看看 `GetMessageStatus` 的实现，以及 `PendingCount` 和 `UnackedCount`：


















        
            
toolName: search_by_regex
            
status: success
          
query: func.*GetMessageStatus|func.*PendingCount|func.*UnackedCount
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在我理解了测试的期望。测试期望断开连接后 pending map 仍然保留消息状态（用于 GetMessageStatus 查询），但 pendingList 需要清空（不再进行超时检测）。

修复方案是：逐个从 pendingList 中 Remove 元素（而不是 Init()），但保留 pending map 不变。这样既解决了悬空指针问题，又保留了状态查询功能。















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在运行测试验证修复：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestDisconnectNonDurableLosesPending|TestGetMessageStatusPending|TestDisconnect|TestReconnect" 2>&1 undefined
          
让我看看 `TestReconnectWithPendingMessages` 测试：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
让我看看测试的开头部分：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
现在我理解了问题。断开连接时 pending map 保留了 msg1 的条目，但重连后重投消息时又创建了新的条目，导致重复。我需要看看 `deliverToConsumer` 的实现：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
问题在于重投时 `deliverToConsumer` 会创建新条目覆盖旧的，但 `unackedCount` 被重复递增了。我需要在 `deliverToConsumer` 中检查并清理旧条目：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在运行测试验证：





















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestDisconnectNonDurableLosesPending|TestGetMessageStatusPending|TestDisconnect|TestReconnect" 2>&1 undefined
          
相关测试都通过了。现在运行全部测试验证：























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 undefined
          
所有测试都通过了！现在更新文档：

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先读取文档找到合适的更新位置：


























        
            
toolName: search_by_regex
            
status: success
          
query: 状态管理|主题树剪枝|断开连接|Disconnect|pending
          
让我看看文档的末尾部分，确定在哪里添加新内容：































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
现在我在文档中添加相关内容。先在"线程安全说明"之前添加新章节：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
文档更新完成。让我做最终的编译和测试验证：










































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ 2>&1 ; echo "Build OK" ; go vet ./internal/pubsub/ 2>&1 ; echo "Vet OK" undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复完成 ✅

已成功修复第2轮修复中引入的两个问题，所有 **77 个测试全部通过**。

### 🐛 问题 1：DisconnectConsumer 数据结构一致性问题

**问题描述**：`DisconnectConsumer` 中使用 `pendingList.Init()` 清空链表，但 `pending` map 保留了消息条目，导致 map 中的元素持有指向已清空链表的悬空指针。后续调用 `Nack` 会在已失效的链表元素上执行 `Remove` 操作，造成数据结构损坏。

**修复方案** ([DisconnectConsumer](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L757-L803))：
- 改用 `for` 循环逐个调用 `c.pendingList.Remove(e)` 清空链表
- `Remove(e)` 会正确设置 `e.list = nil`，后续对这些元素的 `Remove` 操作成为安全空操作
- `pending` map 保留不变，用于 `GetMessageStatus` 查询

**额外增强** ([deliverToConsumer](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L582-L586))：
- 添加了重复条目检测，重投时如果 `pending` map 中已有同 ID 旧条目，先清理旧的再添加新的
- 确保 `pending` map、`pendingList`、`unackedCount` 三者始终一致

### 🐛 问题 2：redeliverOrDeadLetter 中断开连接分支消息丢失

**问题描述**：`redeliverOrDeadLetter` 中断开连接分支在从 `pendingList` 和 `pending` map 移除消息后，仅设置状态为 `Pending` 就直接 `return`，消息既没有放入 `durableBuffer`，也没有进入死信队列，持久化订阅者的离线消息被静默丢弃。

**修复方案** ([redeliverOrDeadLetter](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L723-L729))：
- 添加 `hasDurableSubscription` 辅助方法，检查消费者是否有持久化订阅
- 断开连接时，如果是持久化订阅者，将消息放入 `durableBuffer`
- 非持久化订阅者的消息正常丢弃（符合预期行为）

### 📝 文档更新

在 [pubsub.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/pubsub.md) 中新增 **6.5 内部机制详解** 章节，包含：

| 小节 | 内容 |
|------|------|
| 6.5.1 | 消息状态管理策略 - `MessageStatus` 四种状态如何参与控制流判断 |
| 6.5.2 | 消费者断开连接时的消息处理策略 - 完整流程图和关键设计决策 |
| 6.5.3 | `pending` map 与 `pendingList` 一致性保证 - 各操作的一致性处理机制 |
| 6.5.4 | 主题树剪枝机制 - `isEmpty()` 判断和自底向上剪枝算法 |

### ✅ 验证结果

```bash
go test ./internal/pubsub/ -v -count=1
# 77 个测试全部通过
# go build 和 go vet 均无错误
```