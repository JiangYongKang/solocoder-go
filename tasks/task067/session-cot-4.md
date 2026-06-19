toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func toFloat64
          
让我查看 engine.go 中的 restoreNodeState 是如何被调用的，以更好地理解 FlakyTask 的状态恢复流程：







        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\engine.go
          
好的，我现在理解了整个流程。让我先扩展 `asInt` 函数，然后修复 FlakyTask 的逻辑。

首先扩展 `asInt` 与 `toFloat64` 覆盖范围对齐：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\types.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\types.go
          
changes: undefined
          
现在让我修复 FlakyTask 的逻辑。根据分析，`failures` 是"当前执行尝试内的失败次数"，属于 transient 状态，不应该参与持久化：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我修复 FlakyTask 的逻辑：由于 `failures` 是当前执行尝试的 transient 状态，不需要持久化：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\task.go
          
changes: undefined
          
现在让我补充 asInt 的单元测试。先看看现有测试的结构：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestCompareNumbers
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\workflow_test.go
          

        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; Get-Content "internal\workflow\workflow_test.go" | Select-Object -Last 10 undefined
          
现在让我添加 asInt 的单元测试。先找到文件最后一行的位置：









        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; (Get-Content "internal\workflow\workflow_test.go").Count undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\workflow_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\workflow_test.go
          
changes: undefined
          
现在让我运行所有测试：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/workflow/ -v -count=1 2>&1 undefined
          
让我看看 executeWithRetry 的逻辑，以及 FlakyTask 的行为：










        
            
toolName: search_by_regex
            
status: success
          
query: func executeWithRetry
          
我看到问题了。测试中 `FailCount=2, MaxRetries=3` 意味着 flaky 任务会在第 3 次尝试时成功，所以第一次执行不会失败。让我修复测试参数：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\workflow_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/workflow/ -v -count=1 -run "TestAsInt_TypeCoverage|TestFlakyTask_RestoreStateResetsFailures|TestFlakyTask_ResumeFromBreakpoint_RetryFromScratch" 2>&1 undefined
          
新增的测试全部通过。现在运行完整的测试套件：












        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/workflow/ -v -count=1 2>&1 | Select-Object -Last 30 undefined
          
所有测试通过！让我更新 todo 列表：














        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复，**55 个测试全部通过**！

## 修复内容

### 1. `asInt` 函数扩展覆盖范围与 `toFloat64` 对齐

**问题**：原 `asInt` 只覆盖 7 种类型，而 `toFloat64` 覆盖 13 种类型，覆盖范围不对齐。

**修复方案**：在 [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/types.go#L21-L54) 中扩展 `asInt` 函数，新增支持 `int8`、`int16`、`uint8`、`uint16`、`float32` 和 `string` 共 6 种类型，与 `toFloat64` 完全对齐：

| 类型 | 之前 | 修复后 |
|------|------|--------|
| int, int32, int64 | ✅ | ✅ |
| int8, int16 | ❌ | ✅ |
| uint, uint32, uint64 | ✅ | ✅ |
| uint8, uint16 | ❌ | ✅ |
| float64 | ✅ | ✅ |
| float32 | ❌ | ✅ |
| string (数字解析) | ❌ | ✅ |

### 2. FlakyTask failures 重置逻辑重构

**问题**：之前的实现存在职责混乱：
- `FlakyTask.RestoreState` 先从持久化状态恢复 `failures`
- `FlakyTask.ExecuteWithState` 又立即将其清零
- 两个操作互相拉扯，语义不清晰

**修复方案**：重构 [task.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/task.go#L220-L244)，使职责单一化：

- **`GetState()`** → 返回 `nil`：`failures` 是当前执行尝试的 transient 状态，不需要持久化
- **`RestoreState()`** → 直接将 `failures` 重置为 0：断点续传时明确表示新的执行开始，失败计数清零
- **`ExecuteWithState()`** → 移除重置逻辑，专注于执行本身

这样语义清晰：`RestoreState` 负责断点续传时的状态初始化（清零），`ExecuteWithState` 只负责执行逻辑。

### 3. 新增单元测试

新增 **3 个测试函数**（含 21 个子测试）：

1. [TestAsInt_TypeCoverage](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/workflow_test.go#L1760-L1798) - 覆盖 18 种类型场景（13 种有效类型 + 5 种无效类型）
2. [TestFlakyTask_RestoreStateResetsFailures](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/workflow_test.go#L1800-L1814) - 验证 RestoreState 会将 failures 归零
3. [TestFlakyTask_ResumeFromBreakpoint_RetryFromScratch](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/workflow_test.go#L1816-L1868) - 验证断点续传时 FlakyTask 会从零开始重试，而不是继承之前的失败计数

## 测试结果
```
PASS
ok      solocoder-go/internal/workflow  0.632s
```
共 **55 个测试用例**全部通过。