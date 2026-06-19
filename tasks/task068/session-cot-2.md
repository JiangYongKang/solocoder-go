让我先仔细阅读当前的代码，理解每个问题的具体位置，然后制定修复计划。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\saga.md
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我逐一实施修复。先修改核心代码 [saga.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/saga/saga.go)。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
我将一次性应用对 `saga.go` 的所有修复。让我按顺序进行编辑：

**修复 1 & 4：添加 `ErrExecutionNotFound` 错误，并修复 `executeCompensations` 中的错误日志**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
**修复 2b: 将 `runningSagas` 添加到 Coordinator 结构体**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
**在 NewCoordinator 中初始化 runningSagas**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
现在进行重大更改——重写 `Execute` 方法以添加并发保护和补偿错误封装：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
**修复 1 & 3：重写 `executeCompensations` - 移除虚假成功日志，区分 StepID/ForwardStepID，并使用补偿步骤 ID**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
**修复 3 (续)：更新 `executeCompensation` 以接受并使用 `compStepID`**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
**修复 4: 更新 `GetExecution` 和 `GetLogs` 以返回 `ErrExecutionNotFound`**
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga.go
          
changes: undefined
          
好的，所有核心代码修复已应用。现在让我更新测试文件以匹配这些更改。让我仔细地重写测试文件：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\saga\saga_test.go
          
changes: undefined
          
现在我来运行测试：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/saga/ -v -count=1 undefined
          
所有 37 个测试全部通过！现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\saga.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有修复已完成，全部 37 个测试通过。以下是修复总结：

---

## 修复总结

### Fix 1: 移除无补偿函数步骤的虚假日志

**问题**：[executeCompensations](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/saga/saga.go#L356) 在 `CompensateFunc == nil` 时记录了 `OpTypeCompensation` + `StatusSuccess` 的日志，产生虚假的"补偿成功"记录。

**修复**：将 `c.logOperation(...)` 调用替换为 `continue`，无补偿函数的步骤不再产生任何补偿日志条目。

### Fix 2a: 启用 ErrCompensationFailed 并写入 SagaExecution.Error

**问题**：`ErrCompensationFailed` 已定义但从未使用；补偿失败后 `SagaExecution.Error` 未包含补偿失败信息。

**修复**：在 [Execute](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/saga/saga.go#L218) 中，补偿执行后检查 `execution.NeedsIntervention`，若为 true 则将错误包装为：
```go
execution.Error = fmt.Errorf("%w: forward failed: %v", ErrCompensationFailed, executionErr)
```
可通过 `errors.Is(result.Error, ErrCompensationFailed)` 检测。

### Fix 2b: 启用 ErrSagaRunning 并发执行保护

**问题**：`ErrSagaRunning` 已定义但从未使用；未检查是否有正在执行中的事务。

**修复**：在 [Coordinator](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/saga/saga.go#L132) 中新增 `runningSagas map[string]bool`，`Execute` 开始时检查 `runningSagas[sagaID]`，若已运行则返回 `ErrSagaRunning`，执行完成后通过 `defer` 清除标记。

### Fix 3: CompensationFailure.StepID 与 ForwardStepID 独立标识

**问题**：`StepID` 和 `ForwardStepID` 被赋相同值 `step.ID`，无法区分。

**修复**：
- `ForwardStepID = step.ID`（正向操作标识，如 `"step1"`）
- `StepID = step.ID + "-compensate"`（补偿操作标识，如 `"step1-compensate"`）
- 日志、Compensations 映射键、StepResult.StepID 统一使用 `步骤ID-compensate` 格式

### Fix 4: ErrExecutionNotFound 替代 ErrSagaNotFound 用于执行记录查询

**问题**：`GetExecution` 和 `GetLogs` 在找不到记录时统一返回 `ErrSagaNotFound`，无法区分 Saga 定义不存在和执行记录不存在。

**修复**：新增 `ErrExecutionNotFound` 错误变量，`GetExecution` 和 `GetLogs` 使用该错误返回。`GetSaga`、`RemoveSaga`、`Execute` 继续使用 `ErrSagaNotFound`。

### 测试结果

```
go test ./internal/saga/ -v -count=1
PASS
ok      solocoder-go/internal/saga      0.645s
```

37 个测试全部通过，新增测试覆盖：
- `TestExecute_SagaRunningConcurrentGuard` — 并发执行保护
- `TestExecute_SagaRunningAllowsSequentialExecution` — 顺序执行允许
- `TestExecute_DifferentSagasConcurrent` — 不同 Saga 并发允许
- `TestExecute_ErrCompensationFailedInErrorChain` — `errors.Is` 检查
- `TestCompensationFailure_DistinctStepIDs` — StepID/ForwardStepID 区分
- `TestErrorClassification` — 错误分类验证

### 文档更新

[docs/saga.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/saga.md) 已更新，新增：
- **错误分类策略**：按 Saga 定义相关、执行相关、补偿相关三类分组，含触发场景
- **错误语义区分**：`ErrSagaNotFound` vs `ErrExecutionNotFound` 的区别说明
- **CompensationFailure 字段含义**：`StepID`（补偿操作标识）与 `ForwardStepID`（正向操作标识）的独立设计说明
- **并发执行保护**：保护规则和使用示例
- **无补偿函数步骤不产生日志**：关键设计说明