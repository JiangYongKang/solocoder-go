toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal_test.go
          
现在我清楚地看到了问题：

1.  **Reader 1**（第 801-821 行）：当 `stop` 关闭时，它立即 `return`，而没有进行最终的完整验证。需要在返回之前添加一个最终的 `ReadFrom(0)` 检查。
2.  **Reader 2**（第 824-864 行）：`select` 语句同时包含 `<-stop` 和 `default` —— 当 `stop` 关闭时，Go 的 `select` 可能会随机选择 `default`，从而延迟确定性验证。需要重构：使用一个专用循环进行读取，然后阻塞等待 `stop`，最后进行完整验证。

我来重写这个测试函数：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal_test.go
          
changes: undefined
          
现在我来运行测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/wal/ -v -run TestConcurrentReadWrite -count=3 undefined
          
`TestConcurrentReadWrite` 通过了 3 次重复运行。现在运行完整测试套件：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/wal/ -v -count=1 undefined
          
全部 46 个测试通过。现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestConcurrentReadWrite
          
我看到了不一致的地方。第 515 行的测试矩阵已更新为 2 个读取器，但第 604 行的概览表仍显示“单 reader + 单 writer 并行”。我还需检查第 522 行及之后的详细描述部分：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
现在我需要更新文档的三个地方：

1. **第 529-531 行**：Reader 1 的流程图需要更新以显示最终完整验证（而不仅仅是 "循环 ReadFrom(0) 直到 stop"）
2. **第 543-546 行**：验证点需要提及 Reader 1 也会进行最终完整验证
3. **第 604 行**：概览表需要从 "单 reader + 单 writer 并行" 更新为 "2 reader + 单 writer 并行"
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
现在更新概览表：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有修复已完成，总结如下：

---

## 修复内容

### 1. Reader 1 增加写入完成后全量校验

[wal_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal_test.go#L800-L841) 中 Reader 1 的循环结构从 `select{stop→return, default→ReadFrom+校验}` 改为 `ReadFrom+校验 → select{stop→全量校验+return, default→continue}`，确保 writer 完成后 reader 1 也执行完整的 200 条数据校验（数量 + Offset + Data 内容）。

### 2. Reader 2 消除 select/default 竞态窗口

[wal_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal_test.go#L843-L884) 中 Reader 2 采用与 Reader 1 相同的确定性流程：先执行 `ReadFrom` + 偏移校验，然后 `select` 仅用于检测 `stop` 信号是否到达，`default` 分支为空（仅 `continue` 到下一轮循环）。当 `stop` 就绪时，Go 的 select 会优先选择就绪的 `<-stop` 分支执行全量校验，不再有竞态窗口延迟最终验证。

### 3. 文档同步更新

[wal.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/wal.md) 三处更新：

| 位置 | 旧内容 | 新内容 |
|------|--------|--------|
| 流程图 Reader 1 | "循环 ReadFrom(0) 直到 stop" | 写入期间校验 Offset + stop 后全量校验 |
| 验证点第 3 条 | "最终一致性校验：reader 2 做全量" | "双方全量一致性校验：reader 1 和 reader 2 **均**执行" |
| 并发保障总览表 | "单 reader + 单 writer 并行 / 不崩溃、无 I/O 错误" | "2 reader + 单 writer 并行 / 双方全量校验通过" |

### 测试结果

全部 **46 个测试 PASS** ✅