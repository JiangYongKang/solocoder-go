让我仔细分析 `diffToChanges` 中的坐标问题。首先重新阅读当前代码和理解用户描述的新 Bug。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\diffpatch\merge.go
          

        
            
toolName: CompactFake
            
status: success
          
          
我来修复这个回归问题。首先让我查看当前的代码状态，然后进行修复。
        
            
toolName: view_files
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/merge.go
          

        
            
toolName: view_files
            
status: failed
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/merge_test.go
          
让我查看diffpatch目录下的测试文件。

        
            
toolName: file_search
            
status: success
          
file_pattern: **/diffpatch/*_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v undefined
          
现在让我先写一个测试用例来复现这个问题。问题出在单 Hunk 包含多组非连续变更的场景。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先查看 myers.go 中 buildHunks 的实现，了解如何构造单 Hunk 多组变更的场景。

        
            
toolName: view_files
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/myers.go
          
现在让我构造一个测试用例来复现这个问题。我需要创建一个单 Hunk 包含多组非连续变更的场景。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          
changes: undefined
          
现在让我运行测试来复现问题：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDiffToChanges_SingleHunkMultipleChanges|TestThreeWayMerge_SingleHunkMultipleChanges" undefined
          
太好了！问题已经成功复现。现在让我分析并修复这个问题：

**问题分析**：
1. `changeStart = line.OldLineNo - 1` 错误地将 `changeStart` 设置为当前相等行的 0-based 索引
2. 但 `changeStart` 应该指向下一个变更块的起始位置，即当前相等行的下一行
3. 由于 `line.OldLineNo` 是 1-based 的行号，其数值恰好等于下一行的 0-based 索引（例如：第3行的下一行索引是3）

让我修复这个问题：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我修复 `merge.go` 中的问题：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/merge.go
          
changes: undefined
          
现在让我运行测试来验证修复是否有效：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDiffToChanges_SingleHunkMultipleChanges|TestThreeWayMerge_SingleHunkMultipleChanges" undefined
          
第一个测试通过了！第二个测试只是末尾换行符的小问题。让我检查一下合并逻辑是如何处理换行符的，然后修正测试用例：



        
            
toolName: search_by_regex
            
status: success
          
query: func splitLines
          
让我看看其他合并测试是如何处理换行符的：



        
            
toolName: search_by_regex
            
status: success
          
query: TestThreeWayMerge_NonOverlappingChanges
          
让我修正测试用例，使用与其他测试一致的断言方式：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          
changes: undefined
          
现在让我运行新添加的测试用例：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDiffToChanges_SingleHunkMultipleChanges|TestThreeWayMerge_SingleHunkMultipleChanges" undefined
          
太好了！新添加的测试用例都通过了。现在让我运行所有测试，确保修复没有破坏其他功能：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题已成功修复！以下是完成的工作总结：

## 修复内容

### 1. 核心问题修复 - [merge.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/merge.go#L70-L107)

**问题根源**：上一次修复中 `changeStart = line.OldLineNo - 1` 引入了坐标回退。

- **错误行为**：当处理 `LineEqual` 时，将 `changeStart` 设置为当前相等行的 0-based 索引（例如：第3行 → 索引2）
- **正确行为**：`changeStart` 应该指向下一个变更块的起始位置，即当前相等行的下一行（例如：第3行的下一行 → 索引3）

**修复方案**：
```go
// 恢复为原始正确逻辑：line.OldLineNo (1-based) 数值上等于下一行的 0-based 索引
changeStart = line.OldLineNo

// 添加边界条件处理：文件开头纯插入场景 (hunk.OldStart = 0)
changeStart := hunk.OldStart - 1
if changeStart < 0 {
    changeStart = 0
}
```

### 2. 新增测试用例 - [diffpatch_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go#L1142-L1236)

添加了两个测试用例覆盖单 Hunk 多组变更场景：

- **`TestDiffToChanges_SingleHunkMultipleChanges`**：直接测试 `diffToChanges` 函数，验证在单 Hunk 包含 3 组非连续变更时，每个变更块的坐标计算正确
- **`TestThreeWayMerge_SingleHunkMultipleChanges`**：测试三方合并场景，验证 ours 和 theirs 在不同位置的修改能够正确合并，无行丢失或顺序错乱

### 3. 测试结果

所有 **52 个测试** 全部通过 ✅

```
PASS
ok      solocoder-go/internal/diffpatch 0.229s
```

## 技术说明

这个修复巧妙利用了坐标系统的数值对应关系：
- `line.OldLineNo` 是 1-based 行号（如第 N 行）
- 下一行的 0-based 索引恰好等于 N（因为第 N 行的索引是 N-1，下一行就是 N）
- 所以 `changeStart = line.OldLineNo` 在数值上是正确的