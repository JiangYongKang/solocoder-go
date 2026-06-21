我来彻底解决这些测试质量问题。首先运行调试测试看看真实的行号生成规律：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run TestDebug_BuildHunksLineNumbers undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我重写 `TestDiffToChanges_SingleHunkMultipleChanges`，使用真实的 `Diff` 函数生成完整链路的测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          
让我先构造一个真实的测试场景，确保能生成单 Hunk 多组变更：









        
            
toolName: Write
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/debug_test.go
          
changes: undefined
          
现在让我运行这些新的调试测试，确保它们都能通过：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDebug_" undefined
          
太好了！所有调试测试都通过了。现在让我用高质量的测试替换原有的两个测试用例：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我重写 `TestDiffToChanges_SingleHunkMultipleChanges`，使用真实的 `Diff` 函数生成完整链路的测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我重写 `TestThreeWayMerge_SingleHunkMultipleChanges`，添加逐坐标精确校验：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go
          
changes: undefined
          
现在让我运行所有测试，确保改进后的测试都能通过：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先运行所有测试，确保改进后的测试都能通过，然后再决定 debug_test.go 的处理：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDiffToChanges_SingleHunkMultipleChanges|TestThreeWayMerge_SingleHunkMultipleChanges" undefined
          
两个主要测试都通过了。现在让我运行所有测试，包括 debug_test.go 中的测试：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v -run "TestDebug_" undefined
          
debug_test.go 中的测试虽然现在有断言，但与主测试文件中的测试有大量重复。让我删除它，保持测试套件的整洁：










        
            
toolName: delete_file
            
status: success
          
deleteFilesSingular
filePath: C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/debug_test.go
          
现在让我运行所有测试，确保一切正常：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/diffpatch/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有测试质量问题已修复完成！以下是工作总结：

## 修复内容

### 1. 改进 `TestDiffToChanges_SingleHunkMultipleChanges` - [diffpatch_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go#L1142-L1243)

**问题**：原测试手工构造 `DiffResult` 结构体，直接传入 `diffToChanges`，没有经过真实的 `Diff` 函数和 `buildHunks` 行号生成流程。

**修复**：
- 使用真实的文本调用 `Diff()` 函数生成 `DiffResult`
- 验证 `buildHunks` 确实生成了单 Hunk 多组变更（至少 2 个 delete 操作）
- 打印 Hunk 中行号序列，确认 `buildHunks` 生成的行号规律
- 精确校验每个变更块的 `oldStart`、`oldEnd` 和 `newLines`
- 额外校验坐标边界合法性和内容与新文本的一致性

### 2. 增强 `TestThreeWayMerge_SingleHunkMultipleChanges` - [diffpatch_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/diffpatch/diffpatch_test.go#L1245-L1435)

**问题**：原测试只检查内容是否包含和总行数，无法发现坐标错位导致的行位置偏移。

**修复**，添加了多层精确校验：
- **中间链路校验**：分别对 ours 和 theirs 调用 `Diff()` 和 `diffToChanges()`，校验变更块数量和坐标
- **冲突检测校验**：调用 `detectConflicts()` 确认无冲突
- **变更排序校验**：调用 `mergeChanges()` 校验 5 个变更块按 `oldStart` 正确排序
- **逐行精确校验**：对合并结果的每一行进行逐坐标精确比对
- **未变行校验**：校验未修改的行（索引 0,2,4,6,9）与 base 完全一致
- **ours 变更位置校验**：校验 ours 修改的行（索引 1,5,8）在正确位置且与 ours 文本一致
- **theirs 变更位置校验**：校验 theirs 修改的行（索引 3,7）在正确位置且与 theirs 文本一致
- **applyBothChanges 一致性校验**：直接调用 `applyBothChanges()` 验证结果与 `ThreeWayMerge()` 一致

### 3. 删除 `debug_test.go`

**问题**：该文件最初只包含打印调试信息，虽然后来添加了断言，但与主测试文件中的测试大量重复。

**处理**：删除该文件，保持测试套件整洁，避免重复测试。

## 测试结果

所有 **52 个测试** 全部通过 ✅

```
PASS
ok      solocoder-go/internal/diffpatch 0.219s
```

## 关键验证点

新测试现在能够有效发现坐标回退问题：
- 如果 `changeStart = line.OldLineNo - 1`，第二个变更块的 `oldStart` 会变成 4 而不是 5
- 逐坐标校验会立即发现 `oldStart=4` 与期望值 `oldStart=5` 不符
- 合并结果的逐行比对会发现行位置偏移，如 `THEIRS4` 出现在索引 2 而不是索引 3