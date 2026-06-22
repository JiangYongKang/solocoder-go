我来逐一修复这些问题。先分析一下：

1. **Finalizer 测试无效**：需要让 finalizer 通过 channel 发出信号，并真正断言其被调用
2. **移除 defer 保护**：使用立即执行匿名函数（IIFE）包裹锁内逻辑，内部用 `defer Unlock`，既保留防御性又能在锁外执行回调
3. **回调过时问题**：在锁内捕获回调时已经是值拷贝，但需要明确语义并确保一致

让我开始修复：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer.go
          
现在让我开始修复。首先重写 `ringbuffer.go`，使用 IIFE 模式恢复 defer 保护：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer.go
          
changes: undefined
          
现在让我修复测试用例，确保 finalizer 真正被验证：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
现在让我修复这两个 finalizer 测试，用 channel 真正验证 finalizer 被调用：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在让我运行测试，确保所有修改正确：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先运行测试，看看当前修改是否通过：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/ringbuffer/ -v -timeout 30s undefined
          
`TestOverwriteClearsSlotReferenceType` 失败了。问题在于 finalizer 的执行在 Go 中是异步的，且栈上可能仍有引用。让我改进这个测试，让对象的生命周期更清晰：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在让我也添加一个测试来验证回调捕获语义：回调与状态变化在持锁时原子绑定，之后替换回调不会影响已捕获的回调：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
让我在文件末尾添加回调捕获语义的测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在让我运行测试看看结果：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/ringbuffer/ -v -timeout 60s undefined
          
两个测试失败了。让我分析并修复：

1. **Finalizer 测试**：Go finalizer 执行时机不确定，改用直接访问内部 `buf` 数组（测试同包可访问私有字段）来验证槽位清零
2. **回调语义测试**：需要重新设计——当前实现是在持锁时原子捕获回调，这实际上是正确的语义（状态变化时的回调），测试逻辑需要调整

让我修复这两个测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在重写这两个测试，使用直接访问内部 buf 数组验证清零：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在让我修改回调捕获的问题。用户要求回调执行时使用最新注册的回调，而不是状态变化时捕获的回调。我需要修改 `checkWaterMarkLocked` 返回标志而非回调函数，然后在执行回调前重新获取锁读取最新回调：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer.go
          
现在让我修改回调执行机制，确保回调执行时使用最新注册的函数：















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer.go
          
changes: undefined
          
现在让我修复回调语义的测试，验证回调执行时使用最新注册的回调：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在让我运行测试：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/ringbuffer/ -v -timeout 60s undefined
          
回调测试通过了！但 `TestOverwriteClearsSlotReferenceType` 失败了。让我分析：在 Overwrite 模式下，`buf[0]` 先被清零然后又被写入新数据 `n4`，所以它不应该是 nil。我需要修复测试的预期逻辑：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer_test.go
          
changes: undefined
          
现在再次运行测试：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/ringbuffer/ -v -timeout 60s undefined
          
所有 **31 个测试全部通过 ✅。现在让我更新需求文档。

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我读取文档，然后更新死锁预防和回调机制的说明：




















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\ringbuffer.md
          
让我读取文档的剩余部分，然后更新死锁预防、回调调度和防御性解锁的说明：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\ringbuffer.md
          
现在让我更新文档，重写第 11 章并补充回调调度机制：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\ringbuffer.md
          
changes: undefined
          
完美！现在让我标记所有任务完成：

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复并验证完成！以下是修复总结：

## 修复内容

### 问题 1：Finalizer 测试无效
**原问题**：`TestReadClearsSlotReferenceType` 和 `TestOverwriteClearsSlotReferenceType` 通过 finalizer 验证，但 finalizer 函数体为空且无断言，即使删除清零代码测试也会通过。

**修复方案**：放弃使用不可靠的 finalizer，改为测试同包内直接访问私有字段 `rb.buf` 和 `rb.readPos`，精确断言每个槽位是否被正确清零。

- [TestReadClearsSlotReferenceType](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer_test.go#L699-L761) — 逐一验证 Read 后 `rb.buf[0]`、`rb.buf[1]`、`rb.buf[2]` 为 `nil`
- [TestOverwriteClearsSlotReferenceType](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer_test.go#L764-L851) — 验证覆盖模式下 Read 后的槽位清零，以及环绕写入

---

### 问题 2：移除 defer 带来的防御性保护
**原问题**：`Write` 和 `Read` 使用显式 `Unlock` 替代 `defer Unlock`，锁内 panic 会导致锁泄漏。

**修复方案**：采用 **IIFE（立即执行函数）+ defer Unlock** 模式，将锁内逻辑包裹在匿名函数中：

```go
func (rb *RingBuffer[T]) Write(value T) bool {
    var (needHigh, needLow bool; result bool)
    func() {
        rb.mu.Lock()
        defer rb.mu.Unlock()  // 即使 panic 也保证解锁
        // ... 状态修改 ...
        needHigh, needLow = rb.checkWaterMarkLocked(overwrote)
        result = true
    }()  // IIFE 退出时 defer 自动释放锁
    rb.dispatchCallbacks(needHigh, needLow)
    return result
}
```

涉及修改：[ringbuffer.go Write](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L122-L157)、[Read](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L160-L190)、[SetHighWaterMark](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L72-L107)

---

### 问题 3：回调过时问题
**原问题**：在"释放锁"与"执行回调"之间，另一 goroutine 可通过 `OnHighWater`/`OnLowWater` 替换回调，导致执行的是已过时的旧回调。

**修复方案**：引入独立的 `dispatchCallbacks` 方法，采用**三阶段回调调度**：

1. **阶段一（持锁）**：`checkWaterMarkLocked` 仅返回布尔标志 `needHigh`/`needLow`，不捕获回调函数
2. **阶段二（无锁）**：状态修改锁已释放
3. **阶段三（再次短暂持锁）**：`dispatchCallbacks` 重新获取锁，读取**最新注册**的回调函数引用，释放锁后在无锁状态下执行

```go
func (rb *RingBuffer[T]) dispatchCallbacks(needHigh, needLow bool) {
    var highCb, lowCb func()
    if needHigh || needLow {
        rb.mu.Lock()
        if needHigh { highCb = rb.onHighWater }  // 读取最新注册的回调
        if needLow  { lowCb  = rb.onLowWater }
        rb.mu.Unlock()
    }
    if highCb != nil { highCb() }  // 无锁执行
    if lowCb  != nil { lowCb()  }
}
```

新增代码：[dispatchCallbacks](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L269-L292)

---

### 测试结果
所有 **31 个测试全部通过 ✅**，包括新增的 [TestCallbackUsesLatestRegistered](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer_test.go#L873-L917) 用于验证回调总是使用最新注册版本。

### 文档更新
[ringbuffer.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/ringbuffer.md#L401-L523) 第 11 章已重写为"死锁预防与回调调度设计"，详细说明了 IIFE + defer 模式、三层防御机制、回调时序图和安全性对比表。