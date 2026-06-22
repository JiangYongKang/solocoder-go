好的，我来修复这些问题。让我先创建任务列表并逐一处理。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\semaphore\semaphore.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\semaphore\semaphore_test.go
          
现在修复第一个问题：Release 方法在 held 为零时不应调用 dispatchWaiters。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\semaphore\semaphore.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在增强公平模式顺序测试，验证完整的 FIFO 获取链路：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\semaphore\semaphore_test.go
          
changes: undefined
          
现在运行测试来验证修复是否正确：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/semaphore/ -v -count=1 -run TestFairModeOrdering undefined
          
公平模式顺序测试通过了。现在运行全部测试：









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/semaphore/ -v -count=1 undefined
          
所有 29 个测试全部通过！现在更新文档：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\semaphore.md
          
现在更新文档，反映 Release 方法的修改和更详细的公平模式说明：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\semaphore.md
          
changes: undefined
          
现在更新公平模式的说明，增加更详细的 FIFO 链路说明：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\semaphore.md
          
changes: undefined
          
现在更新设计考量部分，增加 Release 解耦的设计说明：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\semaphore.md
          
changes: undefined
          
发现编号重复了，让我修复一下：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\semaphore.md
          
changes: undefined
          
最后再运行一次全部测试，确保一切正常：











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/semaphore/ -v -count=1 undefined
          
所有测试都通过了！让我更新任务状态：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
已完成所有问题的修复！以下是详细的修改总结：

## 修复内容

### 1. Release 方法解耦
文件：[semaphore.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/semaphore/semaphore.go#L93-L103)

**修改前**：无论 held 是否为 0，都无条件调用 dispatchWaiters
```go
func (s *Semaphore) Release() {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.held > 0 {
        s.held--
    }
    s.dispatchWaiters()  // 总是调用
}
```

**修改后**：只有真正释放了许可（held > 0）才触发调度
```go
func (s *Semaphore) Release() {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.held <= 0 {
        return  // 无许可可释放，直接返回
    }
    s.held--
    s.dispatchWaiters()
}
```

### 2. TestFairModeOrdering 测试增强
文件：[semaphore_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/semaphore/semaphore_test.go#L401-L450)

**修改前**：只验证第一个获取者的 FIFO 顺序

**修改后**：验证完整的 5 个 goroutine 获取链路
- 按顺序启动 5 个 goroutine 进入等待队列
- 每次只释放 1 个许可
- 验证每个位置上获取许可的 goroutine ID 与入队顺序一致
- 使用 channel 同步控制，确保完整的 FIFO 链路验证

### 3. 测试结果
所有 **29 个测试** 全部通过 ✅

```
PASS
ok  	solocoder-go/internal/semaphore	1.724s
```

### 4. 文档更新
文件：[semaphore.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/semaphore.md)

- 更新了 Release 方法的流程图和设计说明
- 增加了公平模式完整获取链路的图文示例
- 新增"释放与调度解耦"设计考量章节
- 修正了章节编号