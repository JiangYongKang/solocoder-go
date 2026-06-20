我需要先查看相关文件的当前状态，了解问题的具体情况。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
现在让我查看其他两个文件的问题：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\query_parser.go
          
现在让我查看 types.go 了解类型定义，然后开始逐一修复。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\types.go
          
现在我对三个问题都理解清楚了。让我开始修复。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复第一个问题：**executor.go 的 time.Sleep 同步机制不可靠**。改为**周期性 Flush + WaitGroup** 模式，确保无论 goroutine 调度延迟如何，Load 调用都不会被遗漏。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在添加一个辅助方法来封装**并发执行 + 周期性 Flush** 模式，然后修改三个方法使用它。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在修改 `executeOperation` 使用新的并发模式：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在修改 `executeList`：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在修改 `executeObject`：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
第一个问题修复完成！现在修复第二个问题。先查看 validator.go 中如何使用查询中解析的类型：




        
            
toolName: search_by_regex
            
status: success
          
query: vd\.Type|\.Type\.Kind|\.Type\.Unwrap
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\validator.go
          
现在理解了 validator 中的类型使用方式。修复**问题 2**：query_parser.go 中对 5 个内置标量名称正确设置 Kind，其他保持 Object（与 schema 的策略一致，query_parser 没有 Schema 上下文，只能基于名称判断）。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\query_parser.go
          
changes: undefined
          
现在修复**问题 3**：dataloader.go 的 Clear/ClearAll 向被移除的请求发送错误，防止 goroutine 泄漏。先检查是否已有相关错误定义。







        
            
toolName: search_by_regex
            
status: success
          
query: ^var.*Err|ErrDataLoader
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\errors.go
          
添加新错误并修复 Clear/ClearAll：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\errors.go
          
changes: undefined
          
现在修复 Clear/ClearAll 方法：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          
changes: undefined
          
三个问题都已修复！现在运行测试验证所有改动：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -v -count=1 2>&1 undefined
          
所有 62 个测试全部通过！







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题修复完成！**62 个测试全部通过**。

---

## 修复总结

### 1. 修复 executor.go 同步机制不可靠问题

**问题**：使用 `time.Sleep(5ms)` + 单次 Flush 做同步，当 goroutine 调度延迟时，Load 调用会被当前层级 Flush 遗漏，N+1 保证被打破。

**修复方案**：采用**周期性 Ticker + WaitGroup** 模式

新增辅助方法 [runConcurrentlyWithFlush](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/executor.go#L179-L206)：

```go
func (e *Executor) runConcurrentlyWithFlush(ctx, wg, spawnGoroutines) {
    spawnGoroutines()                    // 启动所有字段 goroutine
    done := make(chan struct{})
    go func() { wg.Wait(); close(done) }()
    ticker := time.NewTicker(500μs)      // 每 500μs 检查一次
    for {
        select {
        case <-done:        return       // 所有 goroutine 完成
        case <-ticker.C:    e.flushDataLoaders(ctx)  // 周期性 Flush
        }
    }
    e.flushDataLoaders(ctx)              // 收尾 Flush，确保无遗漏
}
```

`executeOperation`、`executeList`、`executeObject` 三个方法全部改用此模式。

---

### 2. 修复 query_parser.go parseTypeReference 类型分类硬编码

**问题**：对所有命名类型一律标记为 `TypeKindObject`，导致查询中 `$value: ID!` 的 ID 变量被错误分类为 Object，与 schema.go 已修复的同名方法行为不一致。

**修复方案**：在 [query_parser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/query_parser.go#L383-L432) 中，通过 `builtinScalarNames` 表检测 5 个内置标量，正确设置 Kind：

```go
var builtinScalarNames = map[string]bool{
    "Int": true, "Float": true, "String": true, "Boolean": true, "ID": true,
}

kind := TypeKindObject
if builtinScalarNames[name] {
    kind = TypeKindScalar
}
```

---

### 3. 修复 dataloader.go Clear/ClearAll goroutine 泄漏

**问题**：Clear/ClearAll 从 pending 队列移除请求后，不向被移除请求的 `result` channel 发送任何值，调用 `dl.Load` 的 goroutine 永久阻塞在 channel 接收上。

**修复方案**：新增 `ErrDataLoaderCleared` 错误，Clear/ClearAll 移除请求时向 channel 发送该错误：

```go
// Clear
for _, req := range dl.pending {
    if req.key != key {
        newPending = append(newPending, req)
    } else {
        req.result <- loaderResult{nil, ErrDataLoaderCleared}  // 通知被移除的请求
    }
}

// ClearAll
for _, req := range dl.pending {
    req.result <- loaderResult{nil, ErrDataLoaderCleared}
}
```