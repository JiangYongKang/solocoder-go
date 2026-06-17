我需要修复两个问题：1) nextInterval 中间隔封顶后正向抖动超标的问题；2) TestDo_WithAllOptions 测试形同虚设的问题。让我先读取当前文件内容。




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
让我找到 TestDo_WithAllOptions 的具体位置。






        
            
toolName: search_by_regex
            
status: success
          
query: func TestDo_WithAllOptions
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
现在我清楚了两个问题，让我逐一修复。首先修复 nextInterval 中的抖动问题：当基础间隔已达到 MaxInterval 时，仅使用负向抖动，避免超出最大间隔。然后修复 TestDo_WithAllOptions 让操作先失败再成功，正确断言回调执行。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          
changes: undefined
          
现在修复 TestDo_WithAllOptions，让函数先失败再成功以触发重试流程，并正确断言回调。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
changes: undefined
          
现在运行测试验证所有修复。







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/retry/ -v undefined
          
所有 33 个测试全部通过！让我运行诊断确保无错误。







        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
所有问题已修复完毕，诊断无错误。
## 修复完成

### 问题 1：抖动超出 MaxInterval 的修复

**文件**: [retry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/retry/retry.go#L173-L207)

**修复策略**: 
- 在计算完基础间隔后，记录 `isCapped` 标记（间隔是否已达到/超过 `MaxInterval`）
- **未封顶**时：使用双向抖动 `[-jitterRange, +jitterRange]`，与原行为一致
- **已封顶**时：使用仅负向抖动 `[-jitterRange, 0]`，确保抖动叠加后绝不会超出 `MaxInterval`
- 额外增加了二次封顶检查 `if interval > MaxInterval` 作为安全网

### 问题 2：TestDo_WithAllOptions 测试形同虚设的修复

**文件**: [retry_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/retry/retry_test.go#L732-L776)

**修复内容**:
- `RetryableFunc` 改为前 2 次失败，第 3 次成功，确保触发完整重试流程
- 将断言变量从 `bool` 改为 `int` 计数器，能够准确统计调用次数
- 删除了空白标识符 `_ = xxx` 的无用语句，替换为真实断言：
  - `fnCalls == 3`（2 失败 + 1 成功）
  - `isRetryableCalls == 2`（对 2 次失败做判定）
  - `beforeCalls == 2`（2 次重试前回调）
  - `afterCalls == 2`（2 次重试后回调）

### 测试结果

`go test ./internal/retry/ -v`：**33/33 全部通过 ✅**，无 lint/类型错误。