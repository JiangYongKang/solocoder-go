我来修复这个测试中 stop channel 成为死代码的问题。首先查看测试代码：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest_test.go
          
问题很明确了。stop channel 确实是死代码，因为 `close(stop)` 在 `wg.Wait()` 之后调用，此时所有 goroutine 已经完成。让我移除这些死代码，简化测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest_test.go
          
changes: undefined
          
现在运行测试验证修复：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/abtest/ -v -run TestConcurrent_ReadAndModifyExperimentPointer -count=1 undefined
          
修复后的测试通过了。现在运行所有测试确保全部通过：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/abtest/ -v -count=1 undefined
          
所有 72 个测试全部通过！让我更新任务状态：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          
---

## 问题修复完成 ✅

### 🔧 修复内容

**问题**：`TestConcurrent_ReadAndModifyExperimentPointer` 测试中的 stop channel 是死代码

**原因分析**：
- `stop` channel 在 `wg.Wait()` 之后才被 `close()`
- 但所有 goroutine 都通过固定迭代次数的 for 循环执行完毕后调用 `wg.Done()`
- 当 `wg.Wait()` 返回时，所有 goroutine 已经退出，`close(stop)` 对任何 goroutine 都没有实际作用
- 循环内的 `select { case <-stop: ... default: ... }` 的 `<-stop` 分支永远不会被触发

**修复方案**：移除所有 stop channel 相关代码，直接使用固定迭代次数的 for 循环

**修改文件**：[abtest_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/abtest/abtest_test.go#L493-L550)

删除的内容：
- `stop := make(chan struct{})` 变量声明
- 每个 goroutine 循环内的 `select { case <-stop: return; default: ... }` 包装
- `close(stop)` 调用

保留的逻辑：
- 40 个 goroutine 并发执行（10 个只读 + 10 个修改返回值 + 20 个 AssignGroup）
- 每个 goroutine 执行 100 次迭代
- `wg.Wait()` 等待所有 goroutine 完成
- 验证内部实验配置未被外部修改破坏

### ✅ 测试结果

```
PASS
ok      solocoder-go/internal/abtest    0.273s
```

**72 个测试全部通过**