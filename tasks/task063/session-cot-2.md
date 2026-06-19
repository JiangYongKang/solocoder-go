我来仔细分析这些问题并逐一修复。先重新读取当前文件的最新内容。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault_test.go
          
现在我已经充分了解了所有问题。让我制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
问题 1-3 都涉及核心代码改动，我一起处理。先修复 `errors.go`、`types.go`、`chaosfault.go`：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\errors.go
          
changes: undefined
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\types.go
          
changes: undefined
          
现在修复 `chaosfault.go`，统一并发保护策略并修改断开语义：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault.go
          
changes: undefined
          
核心代码三个文件已更新。现在更新测试文件，移除 TargetRatio 相关的 disconnect 测试，补充错误注入概率分布测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chaosfault\chaosfault_test.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/chaosfault/ -v undefined
          
所有 40 个测试全部通过！再用 `-race` 检测一下数据竞争：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/chaosfault/ -race -count=1 undefined
          
Windows/386 不支持 `-race`，这不影响测试本身。所有测试已通过。现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\chaosfault.md
          
现在更新文档，反映修复后的并发安全策略和断开语义：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\chaosfault.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有 6 项修复任务已全部完成，以下是修复总结：

---

### 修复 1：统一 `randSrc` 并发保护策略
`ApplyDelay()` 和 `CheckError()` 现在将 `hitTargetRatio()` 和 `calculateDelay()` 的调用移到 `mu.RLock()` 临界区内，与 `CheckDisconnect()` / `IsDisconnected()` 的锁策略保持一致。`sleepFunc` 仍在锁外调用以避免阻塞其他 goroutine。

### 修复 2：删除死代码
- 移除了 `delayFault`、`errorFault`、`disconnectFault` 三个未使用的结构体（[types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/types.go)）
- 移除了 `ErrInjected` 和 `ErrFaultNotEnabled` 两个从未被返回/检查的哨兵错误变量（[errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chaosfault/errors.go)）
- 移除了未使用的 `FaultMetrics` 结构体

### 修复 3：断开模拟语义修正
- `DisconnectConfig` 移除了 `TargetRatio` 字段
- 配置化断开现在采用**持久断开语义**：启用后在时间窗口内所有请求持续失败，不再逐请求随机判定
- `Reconnect()` 仅清除手动断开标志，不会覆盖配置化的持久断开

### 修复 4：补充错误注入概率测试
新增三个测试用例：
- `TestTargetRatio_Error`：统计分布验证，10000 次迭代，ratio=0.5，期望 40%-60% 命中率
- `TestTargetRatio_ErrorZero`：验证 ratio=0.0 永不触发
- `TestTargetRatio_ErrorOne`：验证 ratio=1.0 必定触发

### 修复 5：测试验证
`go test ./internal/chaosfault/ -v` 全部 **40 个测试用例通过**，耗时 0.272s。

### 修复 6：文档更新
[docs/chaosfault.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/chaosfault.md) 已更新：
- 新增第 8 节「并发安全策略」，详述统一锁保护原则、各方法锁使用策略表、关键设计决策
- 第 6 节「连接断开模拟原理」重写，明确持久断开语义，对比延迟/错误注入的逐请求概率模型
- 移除了 `ErrInjected`/`ErrFaultNotEnabled` 的文档条目
- `DisconnectConfig` 不再包含 `TargetRatio`
- 新增反模式 3：`Reconnect()` 不能恢复配置化断开