我来逐一修复这些问题。首先创建任务清单：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先读取相关文件，了解当前代码结构：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
现在开始逐一修复问题。首先修改核心代码文件：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先修改核心代码，同时修复 New() 和 Start()：


好的，使用 Edit 工具逐个修改：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher.go
          
changes: undefined
          
现在更新任务状态，并修改测试文件以适配 API 变更和替换 stringsHasSuffix：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在修改测试文件。需要做以下改动：
1. 添加 `strings` 包到 import
2. 修改所有 `New()` 调用以处理返回的 error
3. 修改所有 `fw.Start()` 调用以处理返回的 error
4. 删除自定义 `stringsHasSuffix` 函数，用标准库 `strings.HasSuffix` 替换
5. 添加新测试用例验证 Start() 返回 ErrNoWatchedDir 和 ErrWatcherStopped




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
changes: undefined
          
我注意到测试文件中有两个重复的测试函数，让我删除重复的并运行测试。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\filewatcher\filewatcher_test.go
          
changes: undefined
          
现在运行测试：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: CompactFake
            
status: success
          
          
让我先运行测试验证修复是否通过，然后更新文档。
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/filewatcher/ -v -count=1 undefined
          
所有 44 个测试全部通过。现在让我读取并更新需求文档。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
根据之前的工作进展，前 3 项代码修复已完成且所有 44 个测试通过。现在让我完成最后一项：更新需求文档，反映 API 变更。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在更新文档，反映 API 变更。需要修改的地方包括：
1. 预定义错误表格中 ErrNoWatchedDir 的触发场景描述（其实已经正确标注了，但需要确保 Start 返回该错误
2. Start 流程中静默返回改为返回错误
3. 不可逆停止约定中关于 Start() 在已停止状态下的行为
4. 所有使用示例中的 New() 和 Start() 调用

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
现在让我再检查一下文件，确保所有 API 调用示例都已更新，特别是监听流程总览中的 `New()` 和 `Start()`：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          
changes: undefined
          
所有文档更新已完成。让我再确认文档中的构造函数部分也补充 New() 返回 error 的说明：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\filewatcher.md
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
全部 5 项修复任务已完成。以下是修复总结：

## 修复内容概览

### 1. `ErrNoWatchedDir` 真正被使用 — Start() 不再静默失败
- [filewatcher.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher.go) 中 `Start()` 方法签名从 `func (fw *FileWatcher) Start()` 改为 `func (fw *FileWatcher) Start() error`
- 三种情况明确返回错误：
  - 已停止状态 → `ErrWatcherStopped`
  - 监听目录为空 → `ErrNoWatchedDir`
  - 正常启动 → `nil`

### 2. 删除重复的辅助函数
- [filewatcher_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/filewatcher/filewatcher_test.go) 中移除了自定义 `stringsHasSuffix` 函数，全部改用标准库 `strings.HasSuffix`
- 所有测试代码已适配 API 变更：`New()` 和 `Start()` 均检查返回错误

### 3. New() 不再 panic
- `New()` 函数签名从 `func New() *FileWatcher` 改为 `func New() (*FileWatcher, error)`
- 内部直接委托给 `NewWithConfig(DefaultConfig())` 并透传其错误，符合 Go 库最佳实践

### 4. 测试全部通过
运行 `go test ./internal/filewatcher/ -v -count=1`，44 个测试用例全部 PASS。

### 5. 需求文档已更新
[docs/filewatcher.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/filewatcher.md) 中的以下部分已同步更新：
- 监听流程总览：标注 `New() (*FileWatcher, error)` 和 `Start() error`
- Start 流程图：明确各分支返回值
- 不可逆停止约定：`Start()` 在已停止状态下返回 `ErrWatcherStopped`
- 所有使用示例代码已适配新 API（基础使用、配置热加载、模式过滤、优雅关闭、单元测试示例）