让我逐一修复这些问题。首先查看当前代码：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
现在开始逐一修复。我将用互斥锁重写 `BeginRequest`，用注册顺序切片替换 Priority，并移除 Shutdown 中的多余检查：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
继续修改，移除 Priority 字段并增加注册顺序：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修改 Manager 结构体，增加回调注册顺序追踪：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修改 NewManager 初始化新字段：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
修改 RegisterCallback，移除 priority 参数并追踪注册顺序：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
修改 UnregisterCallback，同时从 callbackOrder 中移除：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修复问题 1，重写 BeginRequest 用互斥锁实现正确的并发控制：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修复问题 3，移除 Shutdown 中 shutdownOnce.Do 内部的多余二次状态检查：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修复 phaseExecuteCallbacks，改为按注册序的逆序执行：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
现在需要更新测试文件，移除所有 RegisterCallback 调用中的 priority 参数，并修改反序测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
继续批量替换所有其他 RegisterCallback 调用：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
继续逐个替换剩余的 RegisterCallback 调用：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
让我先找出所有剩余需要修改的 RegisterCallback 调用：












        
            
toolName: search_by_regex
            
status: success
          
query: RegisterCallback\(
          
让我查看这些行的上下文：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
我来批量替换所有 priority 参数，同时重命名并修改反序测试：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
继续替换其他 priority 参数：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
现在修改反序测试用例，改为按注册序的逆序：




















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
再检查一下是否还有遗漏的 priority 参数：



















        
            
toolName: search_by_regex
            
status: success
          
query: RegisterCallback\(
          

        
            
toolName: search_by_regex
            
status: success
          
query: Priority|priority
          
还有 4 个遗漏，继续处理：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
发现 969-977 行已经正确，只需修复 950-951 行：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful_test.go
          
changes: undefined
          
现在确认所有修改完成并运行测试：

































        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/graceful/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复，全部 35 个测试通过。下面是修复总结：

## ✅ 修复内容

### 问题 1：BeginRequest 死代码循环
**原代码问题**：`for {}` 循环中所有分支都 `return`，实际从未循环迭代，两个 rollback 分支完全相同。

**修复方案**：改用互斥锁实现原子性检查+递增，代码简洁正确：

```go
// [graceful.go#L265-L275](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graceful/graceful.go#L265-L275)
func (m *Manager) BeginRequest() error {
    m.mu.Lock()
    defer m.mu.Unlock()
    if !m.accepting || m.state != StateRunning {
        return ErrManagerAlreadyShuttingDown
    }
    atomic.AddInt64(&m.activeRequests, 1)
    return nil
}
```

---

### 问题 2：移除 Priority，改为按注册序逆序
**原代码问题**：自行引入 `Priority` 概念，偏离原始需求"按注册的反序"。

**修复方案**：
- 从 `CleanupCallback` 中移除 `Priority` 字段，增加内部 `order int`
- `Manager` 增加 `callbackOrder []string` 和 `nextCallbackOrder int` 追踪注册顺序
- `RegisterCallback` 签名从 `(name, fn, timeout, priority)` 改为 `(name, fn, timeout)`
- `phaseExecuteCallbacks` 直接从 `callbackOrder` 切片末尾向前遍历

---

### 问题 3：移除 Shutdown 中多余的二次状态检查
**原代码问题**：`shutdownOnce.Do` 闭包内部检查 `m.state != StateRunning` 永远不会触发（外层已检查 + once 只执行一次），属于无效死代码。同时 `firstErr` 变量也不再需要。

**修复方案**：移除闭包内的二次检查和 `firstErr` 逻辑，`Shutdown()` 直接返回 `nil`（外层已拦截重复调用）。

---

### 问题 4：更新测试
- 所有 `RegisterCallback` 调用移除最后一个 `priority` 参数
- `TestShutdown_CallbacksReverseOrderByPriority` 重命名为 `TestShutdown_CallbacksReverseRegistrationOrder`，按注册序（first→second→third）验证执行顺序为 third→second→first

---

### 测试结果
```
PASS: 35 / 35
ok   solocoder-go/internal/graceful  2.511s
```