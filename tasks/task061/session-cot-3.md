让我先分析这个问题，查看相关代码：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
我理解了问题。当 `hasBudget` 为 false 时使用 `WithCancel`，导致根 context 超时时 stageCtx 的错误是 `context.Canceled` 而非 `context.DeadlineExceeded`。

**修复方案**：无预算阶段也使用 `WithTimeout(rootCtx, remainingTime)`，这样 stageCtx 的错误就会是 `DeadlineExceeded`，逻辑更一致。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复代码。首先修改 context 创建逻辑，无预算阶段也使用 `WithTimeout`：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在让我在错误处理部分增加一层保护，即使 err 是 `context.Canceled`，也要检查根 context 是否已超时：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在添加测试用例来验证修复：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我找到合适的位置添加测试用例：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
让我在 `TestStageTimeoutError_Unwrap_ContextDeadlineExceeded` 后面添加测试用例：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
现在添加测试用例，验证无预算阶段在总超时时的正确分类：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在运行测试，确保所有测试通过：



































        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/timeoutprop/ undefined
          
编译通过！现在运行所有测试：






































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=1 undefined
          
所有 **43 个测试全部通过**！现在更新任务状态：





































        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复完成总结

已成功修复无预算阶段总超时被误判的问题，**43 个测试全部通过**。

---

### 🐛 问题根因

当 `hasBudget = false`（无预算阶段）时，原代码使用 `context.WithCancel(rootCtx)` 创建子 context：

```go
// 修复前 - 有问题的代码
if hasBudget {
    stageCtx, stageCancel = context.WithTimeout(rootCtx, stageBudget)
} else {
    stageCtx, stageCancel = context.WithCancel(rootCtx)  // ❌ 问题所在
    stageBudget = remainingTime
}
```

当根 context 因总超时时，`select` 语句可能随机命中 `stageCtx.Done()` 分支，此时错误为 `context.Canceled` 而非 `context.DeadlineExceeded`，导致：
- 阶段状态被错误设为 `StageStatusFailed`（业务失败）
- 超时类型被错误设为 `TimeoutTypeNone`
- `errors.Is(err, context.DeadlineExceeded)` 返回 `false`
- 调用方无法识别这是总超时

---

### ✅ 修复方案（双重保障）

**修复1：Context 创建方式统一** - [timeoutprop.go#L152-L155](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop.go#L152-L155)

无预算阶段也使用 `WithTimeout`，这样 stageCtx 的超时错误就是 `DeadlineExceeded`：
```go
if hasBudget {
    stageCtx, stageCancel = context.WithTimeout(rootCtx, stageBudget)
} else {
    stageCtx, stageCancel = context.WithTimeout(rootCtx, remainingTime)  // ✅ 使用 WithTimeout
    stageBudget = remainingTime
}
```

**修复2：错误处理逻辑增强** - [timeoutprop.go#L194-L205](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop.go#L194-L205)

增加对根 context 状态的检查，即使 select 随机命中 stageCtx 分支，只要根 context 已超时，就按总超时处理：

```go
rootCtxErr := rootCtx.Err()
isRootDeadline := errors.Is(rootCtxErr, context.DeadlineExceeded)
isDeadline := errors.Is(err, context.DeadlineExceeded)

if isDeadline || isRootDeadline {  // ✅ 双重检查
    stageStatus = StageStatusTimedOut
    if !hasBudget || isRootDeadline {
        timeoutType = TimeoutTypeTotal
        if !isDeadline {
            err = context.DeadlineExceeded  // ✅ 修正错误类型
        }
    }
    // ...
}
```

---

### 🧪 新增测试用例

| 测试用例 | 验证内容 |
|----------|----------|
| `TestExecute_ZeroBudgetStage_TotalTimeout` | 无预算阶段总超时被正确识别为 `TimedOut` + `TimeoutTypeTotal`，`errors.Is` 返回 `true` |
| `TestExecute_ZeroBudgetStage_NoTimeout` | 无预算阶段正常执行时状态为 `Completed` + `TimeoutTypeNone` |

---

### ✅ 测试结果

```
PASS: 43 tests
ok  solocoder-go/internal/timeoutprop  0.625s
```