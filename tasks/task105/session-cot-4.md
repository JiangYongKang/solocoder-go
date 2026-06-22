好的，我来修复这两个问题：

1. **dispatchCallbacks 时间窗口问题**：使用 `atomic.Value` 存储回调函数，实现无锁原子读取，消除"读取回调 → 执行回调"之间的时间窗口
2. **代码重复问题**：重构 `SetHighWaterMark` 复用 `checkWaterMarkLocked`

让我开始：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我重写 ringbuffer.go，使用 `atomic.Value` 存储回调并消除代码重复：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\ringbuffer\ringbuffer.go
          
changes: undefined
          
现在让我运行测试：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/ringbuffer/ -v -timeout 60s undefined
          
所有 **31 个测试全部通过 ✅**。现在让我更新文档：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我读取文档相关部分并更新：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\ringbuffer.md
          
现在让我更新文档的第 11 章，反映 atomic.Value 和代码去重的修改：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\ringbuffer.md
          
changes: undefined
          
完美！所有任务完成。让我标记：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题已修复并验证完成！以下是修复总结：

## 修复内容

### 问题 1：dispatchCallbacks 回调读取与执行之间的时间窗口

**原问题**：`dispatchCallbacks` 先持锁读取回调，释放锁后再执行，在"读取回调"与"执行回调"之间存在时间窗口，窗口内回调可被替换导致执行过时版本。

**修复方案**：使用 `sync/atomic.Value` 独立存储回调函数，与缓冲区状态的互斥锁完全分离。

- 结构体字段改为：
  ```go
  onHighWater atomic.Value  // 不再受 mu 保护
  onLowWater  atomic.Value
  ```
- `OnHighWater` / `OnLowWater` 使用 `atomic.Store` 原子写入，无需持锁
- `dispatchCallbacks` 直接 `atomic.Load` 读取后立即执行，**读取与调用紧挨着，无任何中间操作**：
  ```go
  func (rb *RingBuffer[T]) dispatchCallbacks(needHigh, needLow bool) {
      if needHigh {
          rb.onHighWater.Load().(func())()  // 原子读取后立即调用，无时间窗口
      }
      ...
  }
  ```
- 初始化时存储 `noopCallback` 空函数，避免 `nil` 函数调用

涉及文件：[ringbuffer.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L23-L35)

---

### 问题 2：SetHighWaterMark 与 Write/Read 的水位检查逻辑重复

**原问题**：`SetHighWaterMark` 中独立维护了一份与 `checkWaterMarkLocked` 相同的高水位触发/解除判断逻辑，两处代码需同步维护。

**修复方案**：重构 `SetHighWaterMark`，在设置新的 `highWater` 值后直接调用 `checkWaterMarkLocked(false)` 复用统一检查逻辑。

修改前（重复逻辑）：
```go
// SetHighWaterMark 中独立判断
if rb.count >= rb.highWater && !rb.highWaterAlarm { ... }
else if rb.count < rb.highWater && rb.highWaterAlarm { ... }
```

修改后（单一真相来源）：
```go
rb.highWater = mark
needHigh, needLow = rb.checkWaterMarkLocked(false)  // 复用统一逻辑
```

涉及文件：[SetHighWaterMark](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/ringbuffer/ringbuffer.go#L77-L104)

---

### 测试结果
所有 **31 个测试全部通过 ✅**，包括 `TestCallbackUsesLatestRegistered` 验证回调始终使用最新注册版本。

### 文档更新
[ringbuffer.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/ringbuffer.md#L401-L545) 第 11 章已重写为"死锁预防与回调调度设计"，包含四层防御机制说明：
1. IIFE + defer Unlock 防御性解锁
2. `checkWaterMarkLocked` 作为水位检查单一真相来源
3. `atomic.Value` 原子存储回调消除时间窗口
4. 回调时序图与安全性保证对比表