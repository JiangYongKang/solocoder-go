我来逐一修复这两个问题。先规划任务：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
### 问题 1 修复：执行器接管批处理调度，同层级 Load 合并为一批

先给 DataLoader 增加线程安全的窗口设置方法：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          
changes: undefined
          
现在修改执行器，层级执行期间禁用自动窗口，保证同层级合并为一批：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在修复**问题 2**：统一两个 parseTypeReference 的分类行为，让 query_parser 支持 Schema 上下文。

首先，给 queryParser 增加 schema 字段，并新增带 Schema 的解析入口：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
先查看 query_parser.go 的当前结构和 ParseQuery 函数：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\query_parser.go
          
现在修改 query_parser.go，增加 Schema 支持：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\query_parser.go
          
changes: undefined
          
现在修改 parseTypeReference，有 Schema 时从 Schema 查真实 Kind：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\query_parser.go
          
changes: undefined
          
现在更新 Executor，使用 ParseQueryWithSchema 传入 Schema：










        
            
toolName: search_by_regex
            
status: success
          
query: ParseQuery
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在运行测试验证所有改动：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -v -count=1 2>&1 undefined
          
编译错误：需要重新导入 `time` 包。










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -v -count=1 2>&1 undefined
          

        
            
toolName: check_command_status
            
status: success
          
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -count=1 undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gqlparser/ -count=1 -v 2>&1 undefined
          

        
            
toolName: check_command_status
            
status: success
          
          
等等，我需要先解决一个关键问题。让我仔细检查一下：禁用自动窗口后，Load 会阻塞等待 Flush，但 Flush 又在 wg.Wait 之后，这会导致**死锁**！

让我重新设计方案：**层级开始时统一重置批处理窗口 + 自动窗口保留 + 收尾 Flush**，既保证合并效果，又不会死锁。

先给 DataLoader 增加重置窗口的方法：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          
changes: undefined
          
现在修改执行器策略：**层级开始时重置窗口起点 + 增大窗口 + 自动 Flush + 收尾 Flush**，既保证合并效果，又不会死锁。











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在重写 `runConcurrentlyWithFlush`，采用 **层级窗口 + 自动 Flush + 收尾 Flush** 策略：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在运行测试验证：











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gqlparser/ -count=1 -v 2>&1 undefined
          
所有 **62 个测试全部通过**！现在更新任务状态：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有修复完成，**62 个测试全部通过**！

---

## 修复总结

### 1. DataLoader 批次合并优化

**问题**：同层级不同 goroutine 的 Load 调用时机因业务逻辑耗时差异而不同步，先到的 Load 触发自动窗口 Flush 后，后到的只能进入下一批，导致多次小批量查询。

**修复方案**：**执行器层级窗口 + 自动窗口 + 收尾 Flush** 三级策略

#### DataLoader 新增 API（[dataloader.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/dataloader.go)）：

- **`SetBatchWindow(d)`**：线程安全地设置批处理窗口大小，传 `0` 禁用自动批
- **`ResetBatchWindow()`**：重置/重启当前窗口计时器，用于统一起点
- **`NewDataLoaderWithWindow(fn, window)`**：自定义窗口大小的构造函数

#### Executor 策略（[executor.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/executor.go#L177-L204)）：

```go
const levelBatchWindow = 2 * time.Millisecond  // 层级窗口，比默认 200μs 大 10 倍

func runConcurrentlyWithFlush(...) {
    // 1. 层级开始：保存原窗口，设置更大的层级窗口，重置窗口起点
    for _, dl := range ctx.DataLoaders {
        dl.SetBatchWindow(levelBatchWindow)
        dl.ResetBatchWindow()       // 所有 DataLoader 窗口从层级开始同时计时
    }
    
    // 2. 并发执行所有 goroutine，DataLoader 自动窗口保证不阻塞
    spawnGoroutines()
    wg.Wait()
    
    // 3. 层级结束：恢复原窗口大小
    for _, s := range saved {
        s.dl.SetBatchWindow(s.window)
    }
    
    // 4. 收尾 Flush，确保零遗漏
    e.flushDataLoaders(ctx)
}
```

**设计要点**：
- 层级窗口从"层级开始"统一计时，而不是从"第一个 Load"开始，消除起点差异
- 2ms 窗口比默认 200μs 大 10 倍，给业务逻辑执行留出充足时间聚集
- 自动窗口机制保留，避免死锁（Load 不会永久阻塞）
- 收尾 Flush 做双重保险，确保所有请求都被处理

---

### 2. 两个 parseTypeReference 分类行为统一

**问题**：query_parser.go 仅识别 5 个内置标量，自定义标量（如 `DateTime`）仍被标记为 `TypeKindObject`，与 schema.go 的同名方法行为不一致。

**修复方案**：query_parser 支持 Schema 上下文，有 Schema 时从 Schema 查真实 Kind

#### query_parser.go 变更（[query_parser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/query_parser.go#L9-L22)）：

- `queryParser` 结构体增加 `schema *Schema` 字段
- 新增 **`ParseQueryWithSchema(query, schema)`** 函数，带 Schema 上下文
- 原 `ParseQuery(query)` 保持向后兼容（内部调用 `ParseQueryWithSchema(query, nil)`）

#### parseTypeReference 分类策略（[query_parser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/query_parser.go#L413-L430)）：

```go
kind := TypeKindObject
if p.schema != nil {
    // 有 Schema 上下文：从 Schema 查真实类型的 Kind，与 schema.go 行为一致
    if realType, ok := p.schema.GetType(name); ok {
        kind = realType.Kind
    }
} else if builtinScalarNames[name] {
    // 无 Schema 上下文：fallback 到 5 个内置标量判断
    kind = TypeKindScalar
}
```

#### Executor 切换到带 Schema 的解析（[executor.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/executor.go#L34)）：

```go
doc, err := ParseQueryWithSchema(query, e.Schema)
```

**效果**：在正常使用路径（Executor.Execute）中，查询解析阶段就能获得正确的类型 Kind，与 schema.go 的 `parseTypeReference` 行为完全一致。validator 中的 `normalizeVariableType` 作为双重保险仍然保留。