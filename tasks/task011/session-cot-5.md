让我先仔细分析当前代码中的问题，然后制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先仔细阅读相关代码。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我先开始第一个修复任务。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
好的，我来逐一分析并修复这两个问题。先修复 `redeliverOrDeadLetter` 正常重投路径的查询盲区。

**问题1分析**：当前代码在第750行先执行了 `delete(c.pending, pm.msg.ID)`，然后进入 select。当 channel 满进入 default 分支时，消息被追加到 durableBuffer，但 pending map 中已无记录，导致查询盲区。

**修复方案**：将 delete 操作移到 select 内部的成功投递分支中，default 分支保留 pending map 条目，状态改为 Pending。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在修复第二个问题：`deliverToConsumer` 中多个分支将持久化消息放入 durableBuffer 时，都需要在 pending map 中创建对应记录，确保状态查询覆盖。

让我仔细分析这个函数中的三个相关分支：
1. 消费者不在线（第566-571行）
2. 背压满（第573-578行）  
3. channel 满 default 分支（第604-608行）

我需要提取一个辅助方法来减少重复代码，同时确保一致性。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在我来修复 `deliverToConsumer`。我需要先添加一个辅助方法来为进入 durableBuffer 的持久化消息创建 pending 记录。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在让我运行测试验证修复是否正确。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/pubsub/ ; go vet ./internal/pubsub/ undefined
          
构建和 vet 检查通过。现在运行所有测试：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | Select-Object -Last 120 undefined
          
有两个测试失败了，`unackedCount` 出现了负数。问题原因是：`trackDurablePending` 创建的 pending 记录没有计入 `unackedCount`，但在 `deliverToConsumer` 清理旧记录时仍然减了计数。让我修复这个问题。




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
问题找到了：清理旧记录时，对于 `trackDurablePending` 创建的记录（`element == nil`），它们从未计入 `unackedCount`，所以不应该递减计数。让我修复。






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
同样，`Ack` 和 `redeliverOrDeadLetter` 中也有类似问题——当处理 `trackDurablePending` 创建的记录（element==nil，从未计入 unackedCount）时也会错误地减计数。让我检查并修复。







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
让我也查看 redeliverOrDeadLetter 的开头：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
现在修复 `Ack` 和 `redeliverOrDeadLetter` 中的同类问题。








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在修复 `redeliverOrDeadLetter`：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pubsub\broker.go
          
changes: undefined
          
现在再次运行测试验证：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 -run "TestBackpressure|TestReconnectWithPendingMessages" 2>&1 undefined
          
这两个测试通过了。现在运行全部测试：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | Select-Object -Last 30 undefined
          
所有 77 个测试通过！现在更新文档。

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先读取文档中需要更新的部分：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我读取 6.5.4 节的完整内容：




















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
让我找到并读取 6.5.4 节：























        
            
toolName: search_by_regex
            
status: success
          
query: ### 6\.5\.
          
让我查看 6.5.3 和 6.5.5 之间的内容：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
我需要更新 6.5.3 节并插入 6.5.4 节（pending map 与 durableBuffer 一致性保证）。让我先更新 6.5.3 节：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
changes: undefined
          
文档更新完成。让我最后再运行一次完整测试确认所有都通过：

























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/pubsub/ -v -count=1 2>&1 | Select-Object -Last 10 undefined
          
所有测试通过！让我更新待办事项：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有任务已完成！以下是本轮修复的详细总结：

## 第 5 轮修复总结

### 修复 1：redeliverOrDeadLetter 正常重投路径的查询盲区

**问题**：正常重投路径在进入 select 之前就执行了 `delete(c.pending, pm.msg.ID)`，当 consumer channel 满进入 default 分支时，消息被追加到 durableBuffer 但 pending map 中已无记录，形成查询盲区。

**修复**：将 `delete(c.pending, pm.msg.ID)` 从 select 外部移至成功投递分支内部。

代码位置：[broker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L749-L768) 第 749-768 行

### 修复 2：deliverToConsumer 中 durableBuffer 消息的状态查询覆盖

**问题**：`deliverToConsumer` 有三个分支将持久化消息加入 durableBuffer（消费者离线、背压满、channel 满），但都没有在 pending map 中创建记录，导致 `GetMessageStatus()` 查询不到。

**修复**：
1. 引入辅助方法 `trackDurablePending`，在所有将持久化消息加入 durableBuffer 的分支统一调用
2. `trackDurablePending` 创建 element==nil 的占位条目（不加入 pendingList、不计入 unackedCount），仅用于状态查询
3. 所有操作 pendingList 和 unackedCount 的地方都增加 `pm.element != nil` 检查，防止对占位条目执行错误操作

涉及的修改：
- [deliverToConsumer](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L565-L625) 三个分支都增加 `trackDurablePending` 调用
- [trackDurablePending](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L614-L625) 新增辅助方法
- [deliverToConsumer 重复检测](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L585-L591) 增加 element 判断
- [Ack](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L649-L656) 增加 element 判断
- [redeliverOrDeadLetter](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pubsub/broker.go#L738-L743) 增加 element 判断

### 修复 3：测试验证

所有 **77 个测试**全部通过 ✓

### 修复 4：文档更新

在 [pubsub.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/pubsub.md) 中大幅更新了以下章节：

- **6.5.3 pending map 与 pendingList 一致性保证**：补充了 trackDurablePending 创建的占位条目的说明，更新了一致性维护机制表格，增加了安全计数检查的说明
- **6.5.4 pending map 与 durableBuffer 一致性保证**（完整新增）：
  - 6.5.4.1 问题背景：查询盲区的两种典型场景
  - 6.5.4.2 一致性保证策略：全面覆盖的核心原则、trackDurablePending 实现、8 种场景的处理表格
  - 6.5.4.3 redeliverOrDeadLetter 正常重投路径的时序修复：错误时序 vs 正确时序对比
  - 6.5.4.4 重连时的一致性恢复：7 步自动恢复流程
  - 6.5.4.5 unackedCount 计数一致性的保证：两种条目类型对比、必须遵守的规则