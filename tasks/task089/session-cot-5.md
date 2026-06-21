让我检查 `executeWithCoverage` 的所有调用点，确认 `crashed` 返回值的使用情况：

        
            
toolName: search_by_regex
            
status: success
          
query: executeWithCoverage
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复 `executeWithCoverage`，移除死代码 `crashed` 返回值：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在更新 `executeSafe` 中的调用：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我检查测试中对 `executeWithCoverage` 的调用方式：







        
            
toolName: search_by_regex
            
status: success
          
query: executeWithCoverage
          
测试中使用了 `cov, _, _ :=` 三个返回值，需要更新为两个返回值：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在运行测试验证所有修改：




        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 2>&1 undefined
          
还有两处调用没被替换（变量名不同），让我修复：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在重新运行测试：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 2>&1 undefined
          
所有 60 个测试全部通过。再运行 `go vet` 确认代码质量：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go vet ./internal/fuzzdriver/ 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复总结

### 问题

[executeWithCoverage](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L645-L670) 方法的 `crashed` 返回值始终硬编码为 `false`，实际崩溃判定完全由 [executeSafe](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L673-L696) 的 `defer recover` 负责，导致 `crashed` 参数成为死代码。

### 修复内容

1. **`executeWithCoverage` 签名简化**：从 `func (f *Fuzzer) executeWithCoverage(input []byte) (coverage *Coverage, execErr error, crashed bool)` 改为 `func (f *Fuzzer) executeWithCoverage(input []byte) (coverage *Coverage, execErr error)`，移除无意义的 `crashed` 返回值。

2. **`executeSafe` 调用更新**：从 `cov, execErr, crashed = f.executeWithCoverage(input)` 改为 `cov, execErr = f.executeWithCoverage(input)`，`crashed` 标志完全由 `defer recover` 逻辑管理。

3. **测试代码更新**：3 处测试调用从 `cov, _, _ :=` 改为 `cov, _ :=`。

### 验证结果

- ✅ 60 个测试全部通过
- ✅ `go vet` 无警告