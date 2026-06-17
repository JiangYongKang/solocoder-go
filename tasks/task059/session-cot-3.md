好的，让我逐一修复这两个问题。先读取当前代码：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
开始修复。先移除未使用的冗余字段：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在修复问题 2，用 CAS 循环重写 BeginRequest：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\graceful\graceful.go
          
changes: undefined
          
现在运行测试验证：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/graceful/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有残留问题已修复，35 个测试全部通过。以下是修复总结：

## ✅ 修复内容

### 问题 1：移除未使用的冗余字段和变量
**移除的死代码**：
- `CleanupCallback.order int` — 只有赋值，从未被读取
- `Manager.nextCallbackOrder int` — 只有自增，从未被读取
- `RegisterCallback` 中 `order: m.nextCallbackOrder` 赋值和 `m.nextCallbackOrder++` 自增

回调顺序完全由 `Manager.callbackOrder []string` 切片管理，`phaseExecuteCallbacks` 直接从该切片逆序遍历，无需额外的 `order` 字段。

---

### 问题 2：BeginRequest 改用 CAS 循环避免粗粒度写锁
**原方案问题**：使用 `sync.RWMutex` 的独占写锁 `m.mu.Lock()`，会阻塞所有并发的 `IsAccepting()`、`State()`、`Phase()` 等读路径。

**新方案（CAS 循环 + 读锁）**：
```go
// [graceful.go#L260-L286](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/graceful/graceful.go#L260-L286)
func (m *Manager) BeginRequest() error {
    for {
        // 步骤 1：用读锁检查状态（不阻塞其他读操作）
        m.mu.RLock()
        accepting := m.accepting
        state := m.state
        m.mu.RUnlock()
        if !accepting || state != StateRunning {
            return ErrManagerAlreadyShuttingDown
        }

        // 步骤 2：CAS 原子递增请求计数（失败则重试）
        cur := atomic.LoadInt64(&m.activeRequests)
        if atomic.CompareAndSwapInt64(&m.activeRequests, cur, cur+1) {
            // 步骤 3：再次用读锁确认状态未变
            m.mu.RLock()
            stillAccepting := m.accepting
            stillRunning := m.state == StateRunning
            m.mu.RUnlock()

            if stillAccepting && stillRunning {
                return nil
            }
            // 状态已变，回滚计数
            atomic.AddInt64(&m.activeRequests, -1)
            return ErrManagerAlreadyShuttingDown
        }
    }
}
```

**并发性能优势**：
- 所有状态检查只使用 **读锁（RLock）**，不阻塞 `IsAccepting()`、`State()`、`Phase()` 等读操作
- 请求计数使用 **CAS（CompareAndSwap）** 原子操作，无需持有写锁
- 只有 CAS 竞争冲突时才会循环重试，高并发下性能远优于粗粒度写锁

---

### 测试结果
```
PASS: 35 / 35
ok   solocoder-go/internal/graceful  2.514s
```