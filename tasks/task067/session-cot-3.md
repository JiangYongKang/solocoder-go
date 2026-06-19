让我先查看相关文件的当前实现，然后修复问题：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\task.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\sequential.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\loop.go
          
现在我了解了所有问题。让我先创建一个通用的辅助函数来处理 int/float64 兼容问题，然后逐一修复：

        
            
toolName: search_by_regex
            
status: success
          
query: func asInt
          
让我先检查一下 types.go，添加一个通用的辅助函数：

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\types.go
          
好的，我现在添加辅助函数并修复所有问题。首先在 types.go 中添加 asInt 辅助函数：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\types.go
          
changes: undefined
          
现在修复 task.go 中的 FlakyTask.RestoreState 和 ExecuteWithState：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\task.go
          
changes: undefined
          
现在修复 FlakyTask.RestoreState：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\task.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在修复 sequential.go 中的问题：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\sequential.go
          
changes: undefined
          
现在修复 SequentialNode.RestoreState：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\sequential.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在修复 loop.go 中的两个地方：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\loop.go
          
changes: undefined
          
现在修复 LoopNode.RestoreState：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\loop.go
          
changes: undefined
          
让我也检查一下 parallel.go 和 conditional.go 是否有类似的问题：










        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\parallel.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\conditional.go
          
让我查看 conditional.go 和 parallel.go 的 RestoreState 方法：












        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\conditional.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\workflow\parallel.go
          
好的，ConditionalNode 和 ParallelNode 的 RestoreState 使用 string 和 bool 类型，没有 JSON 序列化问题。现在让我运行测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/workflow/ -v -count=1 2>&1 undefined
          
所有测试通过！让我更新 todo 列表：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复，**52 个测试全部通过**！

## 修复内容

### 1. 三个 RestoreState 方法兼容 float64 类型

**问题**：JSON 反序列化会将整数转为 `float64`，但原代码只判断 `int` 类型，导致类型断言失败、状态恢复静默失效。

**修复方案**：在 [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/types.go#L21-L40) 中新增通用辅助函数 `asInt()`，兼容多种数值类型（`int`, `float64`, `int32`, `int64`, `uint`, `uint32`, `uint64`）：

```go
func asInt(v interface{}) (int, bool)
```

并更新了以下三个方法使用 `asInt()`：
- [FlakyTask.RestoreState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/task.go#L247-L258) - `failures` 字段
- [SequentialNode.ExecuteWithState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/sequential.go#L38-L49) + [RestoreState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/sequential.go#L103-L112) - `current_index` 字段
- [LoopNode.ExecuteWithState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/loop.go#L85-L93) + [RestoreState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/loop.go#L199-L208) - `current_iteration` 字段

### 2. FlakyTask 断点续传时 failures 计数器重置

**问题**：在断点续传场景下，`failures` 计数器被恢复但没有重置。如果节点之前因 flaky 失败且 `failures` 已超过 `FailCount` 阈值，恢复后第一次执行就因为 `failures` 已达上限而直接成功，未按预期重新重试。

**修复方案**：在 [FlakyTask.ExecuteWithState](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/workflow/task.go#L220-L239) 中，当检测到有状态恢复（`nodeState != nil`）时，先将 `failures` 重置为 0，确保恢复后的 flaky 任务能按照预期重新开始失败计数：

```go
func (n *FlakyTask) ExecuteWithState(ctx context.Context, execCtx *ExecutionContext, nodeState *NodeExecutionState) (*NodeResult, error) {
    if nodeState != nil {
        n.mu.Lock()
        n.failures = 0
        n.mu.Unlock()
    }
    // ... 后续执行逻辑
}
```

## 测试结果
```
PASS
ok      solocoder-go/internal/workflow  0.527s
```
所有 52 个测试用例（包括 8 个断点续传相关测试）全部通过。