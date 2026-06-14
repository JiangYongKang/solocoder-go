我来修复这两个问题。让我先规划一下任务。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先读取当前的代码确认状态，然后进行修复。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
现在让我修复代码。首先修复 `Processor` 结构体，添加可取消的 context 支持。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在修改 `NewProcessor`、`Start`、`Stop` 和 `processMessage` 方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在修改 `Start` 方法，在启动时重新创建 context：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在修改 `Stop` 方法，先取消 context 再等待：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          
现在修改 `processMessage` 方法，使用 `p.ctx` 而不是 `context.Background()`：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复测试中的无限循环 goroutine 泄漏问题：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter_test.go
          
现在修复这个测试，给无限循环 goroutine 添加退出条件：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\deadletter\deadletter_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/deadletter/ -v -timeout 60s undefined
          
所有测试通过！现在更新文档，说明 handler 上下文传递机制和优雅关闭保障策略。


        
            
toolName: todo_write
            
status: interrupted
          
Todos updated: 0 items
          
所有测试已通过！现在让我更新文档，说明 handler 上下文传递机制和优雅关闭保障策略。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\deadletter.md
          
现在让我更新文档，添加 handler 上下文传递机制和优雅关闭保障策略的说明。让我先查看文档的完整结构，然后在适当位置添加内容。





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\deadletter.md
          
现在让我更新文档。首先更新基本使用示例中的 Handler，展示如何正确使用 ctx 响应取消信号，然后添加专门的章节说明上下文传递机制和优雅关闭保障策略。

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\deadletter.md
          
changes: undefined
          
现在让我在文档中添加专门的章节，详细说明 handler 上下文传递机制和优雅关闭保障策略。让我在第 6 章之前插入新的章节：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\deadletter.md
          
changes: undefined
          
现在让我最后运行一次测试，确保所有修改都正确，然后完成任务。




        
            
toolName: run_command
            
status: success
          
command: go test ./internal/deadletter/ -v -timeout 60s -count=1 undefined
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有测试通过，诊断无错误。让我更新任务状态。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复完成总结

### 问题 1：Handler 上下文传递机制

**问题**：`processMessage` 方法中使用 `context.Background()` 作为 handler 的上下文，当 handler 内部出现死循环或长时间阻塞时 `processMessage` 永不返回，导致 `Stop()` 调用 `taskWg.Wait()` 永久挂起。

**修复**（[deadletter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/deadletter/deadletter.go)）：

1. 在 `Processor` 结构体中添加 `ctx context.Context` 和 `cancel context.CancelFunc` 字段
2. `NewProcessor()` 初始化根 context
3. `Start()` 重新创建可取消的 context，支持多次启停
4. `Stop()` 先调用 `p.cancel()` 广播取消信号，再等待任务完成
5. `processMessage` 在调用 handler 前获取 `p.ctx`，将其传递给 handler，使 handler 能够监听到关闭信号

**关键代码变化**：

```go
// Stop() 先取消 context
func (p *Processor) Stop() {
    p.mu.Lock()
    if !p.running {
        p.mu.Unlock()
        return
    }
    p.running = false
    p.cancel()          // 先广播取消信号给所有 Handler
    close(p.stopCh)
    p.wake()
    p.mu.Unlock()

    p.wg.Wait()
    p.taskWg.Wait()     // 再等待 Handler 响应取消后返回
}

// processMessage 传递 p.ctx 给 handler
func (p *Processor) processMessage(msg *DeadLetterMessage, handler MessageHandler) {
    defer p.taskWg.Done()

    p.mu.Lock()
    ctx := p.ctx       // 获取处理器的可取消 context
    p.mu.Unlock()

    err := func() (retErr error) {
        defer func() {
            if r := recover(); r != nil {
                retErr = fmt.Errorf("handler panic: %v", r)
            }
        }()
        return handler(ctx, msg)  // 将 ctx 传递给 handler
    }()
    // ...
}
```

### 问题 2：测试中无限循环 goroutine 泄漏

**问题**：`TestConcurrent_ProcessAndQuery` 启动了一个无退出条件的无限循环 goroutine，测试函数返回后该 goroutine 仍在运行，多次运行会累积僵尸 goroutine。

**修复**（[deadletter_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/deadletter/deadletter_test.go#L1305-L1322)）：

添加 `done` channel 控制 goroutine 退出，并添加 `time.Sleep(10ms)` 避免 CPU 空转：

```go
done := make(chan struct{})
go func() {
    for {
        select {
        case <-done:
            return
        default:
            _ = p.PendingCount()
            _ = p.PermanentlyFailedCount()
            _ = p.GetAllMessages()
            time.Sleep(10 * time.Millisecond)
        }
    }
}()

wg.Wait()
time.Sleep(300 * time.Millisecond)
close(done)  // 通知查询 goroutine 退出
```

### 问题 3：文档更新

在 [docs/deadletter.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/deadletter.md) 中新增了两个章节：

**第 6 章：Handler 上下文传递机制**
- 6.1 上下文生命周期（创建→启动→停止）
- 6.2 上下文传递路径图
- 6.3 Handler 编写最佳实践（3 条原则 + 代码示例）
- 6.4 取消后的消息处理策略

**第 7 章：优雅关闭保障策略**
- 7.1 关闭流程详解（三级保障机制流程图）
- 7.2 各级保障的作用（表格说明）
- 7.3 防止死循环阻塞的保障
- 7.4 优雅关闭的最佳实践（超时保护、分阶段关闭、监控耗时）

### 测试结果

所有 **34 个测试用例** 全部通过 ✓：

```
PASS
ok      solocoder-go/internal/deadletter        3.608s
```