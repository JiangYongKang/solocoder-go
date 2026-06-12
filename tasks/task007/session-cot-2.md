我来逐一修复这些问题。先创建任务列表，然后开始修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
现在我理解了所有问题。让我一次性修复 pool.go 中的所有问题：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在我重写 pool.go，一次性修复所有 4 个问题：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
changes: undefined
          
修复完成。现在我补充单元测试来验证修复效果：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
现在添加新的测试用例来验证修复：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
changes: undefined
          
现在运行测试：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -timeout 120s undefined
          
测试失败是因为我的测试逻辑设计有问题。当连接被归还后，`Put` 会调用 `cond.Signal()` 唤醒等待者，不需要等到空闲超时。让我修正测试用例：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
changes: undefined
          
让我完全重写这几个测试用例，确保逻辑正确可靠：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
让我完全重写这三个测试用例，确保逻辑正确：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
changes: undefined
          
现在运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -timeout 120s undefined
          
所有 26 个测试通过！现在更新文档：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
现在更新文档，反映修复后的实现：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          
现在更新连接借用流程和空闲超时回收流程的文档：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          
现在更新空闲超时回收流程的文档：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          
最后更新线程安全说明，添加 Ping nil 优化的说明：


























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          
最后再运行一次测试，确认所有修复完成：





























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -timeout 120s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复完成！以下是修复总结：

---

## 修复内容汇总

### ✅ 问题 1：回收唤醒
**修复位置**：[reclaimIdleTimeout()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L423-L425)
- 空闲超时回收连接后，当 `len(expired) > 0` 时调用 `cond.Broadcast()` 唤醒阻塞的 Get 调用者
- 心跳检测回收后同样保持 `cond.Broadcast()` 唤醒

### ✅ 问题 2：原子超时检查
**修复位置**：[Get()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L193-L221)
- 移除了 `timedOut` 和 `stopped` 两个 channel
- 超时检查改为在持有锁的状态下执行 `time.Now().After(deadline)`
- 超时协程触发时先获取锁再 Broadcast，确保等待者被唤醒后立即检测到超时
- 彻底避免"假等待"问题

### ✅ 问题 3：删除死代码
**修复位置**：[pool.go#L11-L14](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L11-L14)
- 删除未使用的 `ErrConnExpired` 和 `ErrConnBad` 错误变量
- 保留 `ErrPoolClosed` 和 `ErrPoolExhausted` 两个实际使用的错误

### ✅ 问题 4：Ping nil 优化
**修复位置**：[NewPool()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L72-L74)
- 移除 `if cfg.Ping == nil { cfg.Ping = func(Conn) error { return nil } }` 默认填充
- 保留 `getIdle()` 中的 `if p.cfg.Ping != nil` 判断
- 当 Ping 为 nil 时，跳过检测，避免不必要的锁释放/重新获取开销

### ✅ 问题 5：新增单元测试
新增 **5 个测试用例**，全部通过：
- `TestReclaimIdleTimeout_ReleasesCapacity` - 验证空闲超时回收释放容量
- `TestReclaimIdleTimeout_WakesWaitingGet` - 验证回收后唤醒等待者
- `TestWaitTimeout_AtomicCheck` - 验证超时检查原子性，时间精确在 90-150ms
- `TestPingNil_NoUnnecessaryLocking` - 验证 Ping nil 时无调用，非 nil 时有调用
- `TestReclaimIdleTimeout_WakesMultipleWaiters` - 验证多等待者场景
- `TestHeartbeatReclaim_ReleasesCapacity` - 验证心跳回收释放容量

### ✅ 问题 6：测试结果
**26 个测试全部通过**
```
PASS
ok      solocoder-go/internal/connpool  6.893s
```

### ✅ 问题 7：文档更新
**[docs/connpool.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/connpool.md)** 更新内容：
- 配置约束：更新 Ping 默认行为为 `nil`
- 预定义错误：移除两个未使用的错误
- Get 流程：更新为原子超时检查机制，新增"等待唤醒机制说明"
- 空闲超时回收流程：新增 Broadcast 步骤和"回收唤醒机制说明"
- 线程安全章节：升级为"线程安全与性能优化说明"，新增三个关键优化点的说明和代码链接