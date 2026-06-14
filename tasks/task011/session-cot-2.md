我来逐一修复这些问题。先创建任务清单，然后开始修复。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先读取 broker.go 确认当前状态：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在开始修复。先删除无用的 channel 字段：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在修复第二个问题 - removeFromTopicTree 的节点剪枝逻辑：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先看看当前的 removeFromTopicTree 和辅助函数：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在让我实现节点剪枝。首先需要一个辅助函数判断节点是否为空，然后在 `removeFromTopicTree` 中自底向上地删除空节点：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在重写 `removeFromTopicTree`，实现自底向上的剪枝：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在处理第三个问题 - MessageStatus。让我检查状态在哪里被使用，以及如何让它们真正参与控制流：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我全面检查 MessageStatus 的使用情况：








        
            
toolName: search_by_regex
            
status: success
          
query: MessageStatus
          
现在让我看看 MessageStatusPending 被设置的地方和所有消息处理的流程，以便决定如何让状态真正参与控制流。我决定让这些状态在多个关键操作中都参与判断：
- ProcessTimeouts 中只处理 Delivered 状态（已有）
- Ack/Nack 中检查状态有效性
- deliverToConsumer 中检查状态









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
我继续处理剩余的问题。现在让 MessageStatus 真正参与控制流判断。我将在 Ack、Nack 等关键操作中加入状态校验，防止重复确认等问题，并让状态字段成为有效的状态机。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取 Ack、Nack 以及相关代码，确保正确修改：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在让我修改 Ack、Nack、ProcessTimeouts、redeliverOrDeadLetter、deliverToConsumer 以及 DisconnectConsumer，让 MessageStatus 真正参与控制流判断：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在修改 Nack：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在看看 ProcessTimeouts 和 DisconnectConsumer，确保状态都参与正确的控制流：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在让我在 DisconnectConsumer 中也加入状态判断，只处理 Delivered 状态的消息，并确保 deliverToConsumer 中的状态设置合理。让我也检查 flushDurableBuffer 和 deliverToConsumer 对 Pending 状态的处理：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在让我也在 redeliverOrDeadLetter 中检查状态，避免重复处理。同时再看 ProcessTimeouts：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
ProcessTimeouts 已经正确使用了 MessageStatusDelivered。现在让我也在 redeliverOrDeadLetter 中检查状态，并且添加一个对外的查询方法，让状态机更加完整：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在添加一个查询消息状态的 API，让状态可以被外部读取。让我看看 broker.go 末尾的 API：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在在合适的位置添加查询消息状态的 API：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在让我运行测试验证修复是否正确：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ 2>&1 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 undefined
          
所有 71 个测试都通过了！现在让我添加针对新功能的测试（主题树剪枝和 MessageStatus 状态机），然后更新文档：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我在 broker_test.go 末尾添加新的测试用例：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker_test.go
          
changes: undefined
          
现在运行所有新添加的测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestTopicTree|TestTopicNode|TestGetMessageStatus|TestMessageStateMachine" 2>&1 undefined
          
有一个测试失败了：`TestGetMessageStatusPending`。问题是当消费者断开连接时，消息从 `c.pending` map 中移除并放入 `durableBuffer`，但此时消息 ID 和状态也一起丢失了。让我重新审视设计，需要修复这个问题：

当持久化消费者断开连接时，应该保留 pendingMessage 的状态信息，而不只是把 msg 放到 durableBuffer 中。让我修改 DisconnectConsumer 的逻辑：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
我看到了问题。在 DisconnectConsumer 中，当我们把消息移到 durableBuffer 时，同时清空了 c.pending map，所以之后 GetMessageStatus 无法找到消息。更好的做法是：把状态是 Pending 的消息依然保留在 c.pending 中，而不只是将其移入 durableBuffer。这样 GetMessageStatus 才能查到 Pending 状态的消息。

让我调整一下设计，保留 pending map 中的记录：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在我需要让 ReconnectConsumer 时也能处理这些 pending 状态的消息。让我查看 ReconnectConsumer 和相关代码：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在让我运行测试看看状态：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestGetMessageStatusPending|TestDurable|TestReconnect|TestDisconnect" 2>&1 undefined
          
太好了！现在运行全部测试确保没有回归：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 undefined
          
所有 **83 个测试** 全部通过！现在更新文档：














        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我找到文档中的相关部分进行更新。先看 topicNode 和主题树部分，以及消息流向部分：













        
            
toolName: search_by_regex
            
status: success
          
query: topicNode|主题树|剪枝|状态机|状态
          
让我读取文档的相关段落，找到合适的位置插入新内容：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我继续读，找到 topicNode 章节的末尾和主题树算法部分：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
现在我需要更新文档。让我先更新 Broker 结构体（移除 ackTimerCh 和 retryTimerCh），然后在 MessageStatus 章节补充状态机说明，在 topicNode 章节补充剪枝机制说明，在主题树算法部分补充剪枝算法：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在更新 Broker 结构体，移除无用的 channel 字段：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在在 topicNode 章节补充剪枝机制说明：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在让我在主题树路由算法部分补充剪枝相关说明。先找到 4.2 节：























        
            
toolName: search_by_regex
            
status: success
          
query: ^### 4\.
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我在主题树路由算法部分追加剪枝说明：
























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在让我在使用示例中补充状态查询的 API 使用示例：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我在死信队列示例之后添加状态查询的示例：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
我注意到原来的 5.7 节标题和代码块标签被破坏了，需要修复：


























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
现在做最后一次完整测试确保一切正常：



























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ ; go vet ./internal/pubsub/ ; go test ./internal/pubsub/ -v -count=1 2>&1 | tail -20 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ ; go vet ./internal/pubsub/ ; go test ./internal/pubsub/ -count=1 2>&1 undefined
          
所有构建、检查和测试全部通过！让我更新任务清单：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## 所有问题修复完成 ✅

### 修复内容总结

#### 1. 移除无效的 channel 字段

从 [Broker](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L115-L126) 结构体中删除了未使用的 `ackTimerCh` 和 `retryTimerCh` 字段，以及 [NewBroker](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L121-L143) 中的初始化代码。超时检测完全由 `timeoutLoop` 的 `time.Ticker` (100ms 轮询) 驱动。

#### 2. 实现主题树节点剪枝机制

新增 [topicNode.isEmpty()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L108-L113) 方法，重写了 [removeFromTopicTree](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L262-L308)：

- 删除订阅后，沿路径自底向上回溯
- 对每个节点执行 `isEmpty()` 检查（subscribers、wildcardOne、wildcardAny、children 全部为空）
- 空节点从父节点的 `children` 中移除，遇到非空节点立即停止
- 有效防止 Subscribe/Unsubscribe 循环导致的空壳节点链内存泄漏

#### 3. MessageStatus 状态机落地

MessageStatus 不再是"只写不读"的装饰字段，而是构成完整的控制流状态机：

| 函数 | 状态参与逻辑 |
|------|-------------|
| [Ack()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L601-L631) | 仅处理 `Delivered`/`Pending` 状态，其他返回 `ErrMessageNotFound` |
| [Nack()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L666-L690) | 仅处理 `Delivered`/`Pending` 状态，防止重复处理 |
| [ProcessTimeouts()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L806-L826) | 仅对 `Delivered` 且超时的消息触发重推 |
| [redeliverOrDeadLetter()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L692-L739) | 入口校验状态有效性，背压/离线时设置为 `Pending` |
| [DisconnectConsumer()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L741-L778) | `Delivered` → `Pending`，保留 pending map 便于状态追踪 |

新增对外 API [GetMessageStatus()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L943-L964) 用于查询消息当前状态。

#### 4. 测试覆盖

新增 **12 个**测试用例，总计 **83 个**测试全部通过：

- `TestTopicTreePruningOnUnsubscribe` - 订阅/取消订阅的节点回收
- `TestTopicTreePruningMultipleSubs` - 多订阅者共享路径的剪枝
- `TestTopicTreePruningSharedPrefix` - 共享前缀路径的剪枝策略
- `TestTopicTreePruningWildcardOne/WildcardAny` - 通配符订阅的剪枝
- `TestTopicTreePruningOnRemoveConsumer` - 移除消费者时的剪枝
- `TestTopicNodeIsEmpty` - 节点空判断的各字段覆盖
- `TestGetMessageStatus` - Delivered/Acked 状态查询
- `TestGetMessageStatusDeadLetter` - Dead 状态查询
- `TestGetMessageStatusPending` - Pending 状态查询
- `TestMessageStateMachineTransitions` - Delivered→Delivered(重推)→Dead 完整状态流转

#### 5. 文档更新

[docs/pubsub.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/pubsub.md) 已同步更新：

- **MessageStatus 章节**：补充状态机控制流说明和状态转移图
- **topicNode 章节**：补充节点剪枝机制说明
- **Broker 章节**：移除失效字段，补充超时检测策略说明
- **主题树路由算法**：新增删除与剪枝流程小节
- **使用示例**：新增 `5.7 消息状态查询` 示例代码