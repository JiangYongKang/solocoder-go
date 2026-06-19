我来帮你修复这些问题。让我先分析问题并制定修复计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先查看相关代码，了解当前实现：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
现在我清楚地看到了问题。让我逐一修复。

**问题1**：业务错误被标记为 `StageStatusTimedOut`。让我先看完整的错误处理部分：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
现在让我逐一修复这些问题。首先修复问题1 - 区分业务错误和超时错误：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在修复问题2 - 最小阈值检查使用根 context 剩余时间，零预算阶段也检查：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题2 - 最小阈值检查逻辑：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop.go
          
changes: undefined
          
现在修复问题3 - StageTimeoutError.Unwrap 返回标准 context.DeadlineExceeded：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题3：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\errors.go
          
changes: undefined
          
现在修复问题4 - 重写 TestExecute_MinThresholdSkip 测试，添加实质性断言：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在重写 TestExecute_MinThresholdSkip 测试：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在我需要更新 `TestExecute_StageReturnsError` 测试，因为现在业务错误返回 `StageStatusFailed` 而不是 `StageStatusTimedOut`：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
现在更新 `TestExecute_StageReturnsError` 测试以匹配新的行为：























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在让我添加一个测试来验证 `errors.Is` 可以正确判断 `context.DeadlineExceeded`：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
让我在合适的位置添加一个测试来验证 `errors.Is` 可以正确判断 `context.DeadlineExceeded`：



























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
现在添加测试来验证 `errors.Is` 可以正确判断 `context.DeadlineExceeded`：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在让我编译并运行测试：































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/timeoutprop/ undefined
          
编译通过！现在运行所有测试：

































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=1 undefined
          
所有 41 个测试全部通过！现在更新文档：



































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先查看现有的文档内容，找到需要更新的部分：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
让我继续查看文档的相关部分：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
让我继续查看文档的超时类型与阶段状态部分：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
让我继续查看错误定义部分，然后进行更新：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
让我继续查看文档的错误定义和示例部分：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
现在让我更新文档。首先更新 4.3 最小阈值跳过机制部分：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
现在更新 5.2 StageStatus 部分，明确区分超时错误和业务错误：























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
现在更新 7.4 检查超时类型的示例，添加使用 `errors.Is` 的示例：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
现在更新错误定义部分，说明 `StageTimeoutError` 的 `Unwrap` 方法返回 `context.DeadlineExceeded`：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
现在更新配置说明中的 MinThreshold 说明：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\timeoutprop.md
          
changes: undefined
          
让我再次运行所有测试，确保一切正常：


































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=1 undefined
          
所有 41 个测试全部通过！现在更新任务状态：






































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## 修复完成总结

已成功修复超时传播器模块的所有问题，**41 个测试全部通过**。

---

### 🔧 修复详情

| 问题 | 修复位置 | 修复内容 |
|------|----------|----------|
| **问题1**：业务错误和超时错误无法区分 | [timeoutprop.go#L191-L213](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop.go#L191-L213) | 引入 `stageStatus` 变量，超时错误设为 `StageStatusTimedOut`，业务错误设为 `StageStatusFailed` |
| **问题2**：最小阈值检查使用阶段预算而非剩余时间 | [timeoutprop.go#L137-L140](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop.go#L137-L140) | 移到预算计算之前，使用 `remainingTime`（根 context 剩余时间）进行判断，零预算阶段也检查 |
| **问题3**：`Unwrap` 返回自定义类型导致 `errors.Is` 失败 | [errors.go#L34-L41](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/errors.go#L34-L41) | 返回标准 `context.DeadlineExceeded`，删除自定义的 `deadlineExceededError` 类型 |
| **问题4**：测试无实质断言，恒为 PASS | [timeoutprop_test.go#L266-L337](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop_test.go#L266-L337) | 重写为 3 个测试用例，覆盖时间不足、零预算、时间充足三种场景 |

---

### 📋 错误类型区分策略

| 错误场景 | 阶段状态 | 超时类型 | `errors.Is(err, context.DeadlineExceeded)` |
|----------|----------|----------|-------------------------------------------|
| 根 context 超时 | `TimedOut` | `Total` | ✅ `true` |
| 阶段预算超时 | `TimedOut` | `Budget` | ✅ `true` |
| 业务错误 | `Failed` | `None` | ❌ `false` |
| Panic | `Failed` | `None` | ❌ `false` |

---

### ✅ 最小阈值检查判定逻辑

```
剩余时间 = 根 context 截止时间 - 当前系统时间
如果 剩余时间 < MinThreshold:
    标记阶段为 SKIPPED，超时类型为 MIN_THRESHOLD_SKIP
    跳过当前阶段
```

- ✅ 使用根 context 剩余时间判断，与阶段预算无关
- ✅ 零预算阶段同样检查，不设例外
- ✅ `MinThreshold = 0` 时跳过检查

---

### 📝 文档更新

在 [docs/timeoutprop.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/timeoutprop.md) 中新增：
- **错误类型区分策略表**（5.2 节）
- **最小阈值检查判定逻辑**（4.3 节）
- **`StageTimeoutError` 结构与 `Unwrap` 约定**（第 8 节）
- **`errors.Is` 和 `errors.As` 完整使用示例**（7.4 节）

---

### 🧪 测试结果

```
PASS: 41 tests
ok  solocoder-go/internal/timeoutprop  0.561s
```