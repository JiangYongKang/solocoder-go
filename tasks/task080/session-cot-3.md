我来分析并修复这个问题。问题的核心是需要一个既能自动覆盖包内所有源文件、又不误伤测试文件的过滤策略。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          
让我先查看当前的 `isInternalFrame` 实现：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
我将设计一个更健壮的方案：基于包目录路径 + 非测试文件的双重判定。这样可以自动覆盖包内所有新增的源文件，同时不会误伤测试文件。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          
我需要添加 `path/filepath` 包的导入：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\structlog\structlog.go
          
changes: undefined
          
现在运行测试验证：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/structlog/ -v undefined
          
所有 44 个测试全部通过。让我更新任务状态并总结。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 2 items
          
## 修复总结

### 问题

`isInternalFrame` 函数之前只过滤 `structlog.go` 一个文件，如果包内后续新增其他内部辅助文件（如 `encoder.go`、`formatter.go`），这些文件中的栈帧不会被过滤，会泄漏到 caller 和 stack 输出中。

### 解决方案

重写 `isInternalFrame` 函数，采用**包目录路径 + 非测试文件**的双重判定策略：

```go
func isInternalFrame(frame runtime.Frame) bool {
    hasStructlogDir := strings.Contains(frame.File, "/internal/structlog/") ||
        strings.Contains(frame.File, "\\internal\\structlog\\")
    if !hasStructlogDir {
        return false
    }
    base := filepath.Base(frame.File)
    if strings.HasSuffix(base, "_test.go") {
        return false
    }
    return true
}
```

### 判定逻辑

| 条件 | 结果 | 说明 |
|------|------|------|
| 文件路径不包含 `internal/structlog/` | 返回 false | 不属于 structlog 包，不是内部帧 |
| 文件路径包含 `internal/structlog/` 且文件名以 `_test.go` 结尾 | 返回 false | 是测试文件，不是内部帧 |
| 文件路径包含 `internal/structlog/` 且文件名不以 `_test.go` 结尾 | 返回 true | 是包内源文件，属于内部帧，需要过滤 |

### 优势

1. **自动扩展**：包内新增的任何非测试文件（`encoder.go`、`formatter.go` 等）都会被自动过滤，无需修改代码
2. **精准性**：不会误伤测试文件（`structlog_test.go`），caller 准确定位到测试文件中的调用行
3. **稳定性**：不受文件名、函数名、方法签名变化的影响

### 修改的文件

- [structlog.go#L3-L14](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog.go#L3-L14) — 新增 `path/filepath` 导入
- [structlog.go#L163-L174](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/structlog/structlog.go#L163-L174) — 重写 `isInternalFrame` 函数

所有 44 个测试通过。