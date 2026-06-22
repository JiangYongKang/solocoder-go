让我先查看当前的 Start() 实现和相关测试，了解具体的代码逻辑。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          
现在让我查看测试文件中与 Start 相关的测试用例，特别是测试幂等启动的部分。

        
            
toolName: search_by_regex
            
status: success
          
query: TestStart|Start\(\)
          
让我查看 TestStartStop_Idempotent 测试的完整内容：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
现在我理解了问题。需要添加一个新的 sentinel error `ErrAlreadyRunning`，当 Start() 被调用且监听器已经在运行时返回这个错误，让调用方能够区分"首次启动成功"和"早已在运行"这两种情况。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先添加 `ErrAlreadyRunning` sentinel error：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          
changes: undefined
          
现在修改 `Start()` 方法，当 `running == true` 时返回 `ErrAlreadyRunning`：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          
changes: undefined
          
现在更新测试文件。首先更新 `TestStartStop_Idempotent` 测试以适配新的返回值：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
changes: undefined
          
现在添加新的测试用例 `TestStart_AlreadyRunning`，我需要先找到合适的插入位置：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
在 `TestStart_WithoutWatch` 和 `TestIsRunning` 之间添加新的测试用例：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
changes: undefined
          
现在运行测试确保所有测试通过：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/filewatcher/ -v -count=1 undefined
          
所有 45 个测试全部通过，包括新增的 `TestStart_AlreadyRunning`。现在更新文档：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在更新文档，首先更新预定义错误表格：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
现在更新 Start 流程部分：







        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
          
让我重新读取文档，查看 Start 流程的确切内容：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
现在更新生命周期管理中的状态行为说明和不可逆停止约定：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
现在更新使用示例，展示如何利用 `ErrAlreadyRunning` 区分首次启动：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
让我查看 6.5 之后的内容，添加一个新的使用示例：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
在 6.5 和 6.6 之间添加一个新的使用示例，展示如何利用 `ErrAlreadyRunning`：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
现在更新测试覆盖说明，添加新的测试用例：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
最后再次运行测试确认所有修改都正确：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/filewatcher/ -v -count=1 -run "TestStart" undefined
          
所有修复已完成，所有 45 个测试全部通过。下面是本次修复的总结：

## 修复内容总结

### 问题分析
`Start()` 方法在已处于运行状态时返回 `nil`，与首次启动成功返回 `nil` 无法区分。调用方无法判断是"刚启动成功"还是"早已在运行"，导致无法在首次启动时执行初始化操作。

### 修复方案
添加新的 sentinel error `ErrAlreadyRunning`，当 `Start()` 检测到监听器已在运行时返回该错误，让调用方能够通过 `errors.Is(err, ErrAlreadyRunning)` 明确区分三种情况：

| 返回值 | 含义 |
|--------|------|
| `nil` | 首次启动成功 |
| `ErrAlreadyRunning` | 已在运行，无需重复初始化 |
| `ErrNoWatchedDir` | 未设置监听目录 |
| `ErrWatcherStopped` | 已永久停止 |

### 代码变更

**1. [filewatcher.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher.go#L12-L19)** - 添加 `ErrAlreadyRunning` sentinel error：
```go
ErrAlreadyRunning = errors.New("filewatcher: watcher is already running")
```

**2. [filewatcher.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher.go#L232-L253)** - 修改 `Start()` 方法：
```go
if fw.running {
    fw.mu.Unlock()
    return ErrAlreadyRunning  // 之前返回 nil
}
```

**3. [filewatcher_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher_test.go#L799-L832)** - 更新 `TestStartStop_Idempotent` 测试，验证第二次调用返回 `ErrAlreadyRunning`

**4. [filewatcher_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher_test.go#L878-L904)** - 新增 `TestStart_AlreadyRunning` 测试用例，完整验证 `ErrAlreadyRunning` 的返回语义

### 文档更新
在 [docs/filewatcher.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/filewatcher.md) 中更新了：
- 预定义错误表格，添加 `ErrAlreadyRunning` 说明
- Start 流程图，更新 `running == true` 分支的返回值
- 生命周期管理部分，增加 `Start()` 返回值语义说明
- 使用示例，新增"区分首次启动与重复启动"示例（6.6 节）
- 测试覆盖说明，添加 `TestStart_AlreadyRunning` 条目

### 测试结果
所有 45 个测试全部通过，包括 4 个 Start 相关测试：
- `TestStartStop_Idempotent` ✅
- `TestStart_WithoutWatch` ✅
- `TestStart_AlreadyRunning` ✅（新增）
- `TestStart_AfterStop` ✅