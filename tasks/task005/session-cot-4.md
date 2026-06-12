好的，我来重构：从生产代码中移除测试钩子，改用测试端轮询文件系统的方式观察中间状态。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          
现在从生产代码中移除钩子：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在重构测试文件。核心设计：

**TestCompressAndCleanupRace**：启动独立的监控 goroutine，以极短间隔（1ms）持续轮询文件系统，在整个压缩/清理生命周期内捕获中间状态快照：
- 监控 goroutine 在压缩进行期间观察到 `.log` 文件存在 → 证明压缩确实在执行
- 最终所有 `.log` 消失、`.gz` 存在且可解压 → 证明流程正确完成
- 多次切分 + 备份数上限 → 验证清理不会破坏正在进行的压缩

**TestConcurrentRotateWithCompress**：类似监控策略 + 4 goroutine 并发写入。

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          
changes: undefined
          
现在运行测试：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          
好的，测试结果非常关键：

1. **最终状态完全正确**：`final state: 2 .gz backups, 0 original backups` —— 说明压缩和清理都正常工作
2. **捕获到了中间状态**：580 个快照，确实在压缩进行期间观察到了 `.gz` 文件为 0 字节的中间状态
3. **这不是 bug，是特性**：`compressAndRemove` 先 `os.Create(dstPath)` 创建 .gz（此时 0 字节），再写数据。1ms 间隔的轮询正好捕捉到了这个合法的中间状态

我需要修复测试断言：`.gz` 文件在压缩期间短暂为 0 字节是正常的，只要**最终**它是有效即可。同时增加对"同一快照中源文件和 .gz 同时存在且 .gz 已非 0 字节 → 源文件应已被删除"的竞态检测。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          
changes: undefined
          
现在再运行测试：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          
很好！关键发现：

1. **最终状态完全正确**：`final state: 2 .gz backups, 0 original backups`
2. **快照确实捕捉到了中间状态**：压缩 goroutine 正在执行 `compressAndRemove` 时，源文件和 .gz 文件会短暂共存
3. **这不是真正的竞态 bug** —— 这是 `compressAndRemove` 的正常执行窗口：先写 .gz，写完后再删源文件。两个文件短暂共存是设计如此，不是 bug

我的断言过于严格了。真正的竞态 bug 应该是：**cleanOldBackups 误删了还没压缩完的源文件**，导致 .gz 不完整。但这里最终所有 .gz 都可正确解压，说明没有真正的竞态问题。

我需要修改测试逻辑：把检测"源文件和 .gz 共存"改为**追踪每个压缩操作的生命周期**——验证源文件最终消失、.gz 最终可解压，且在**后续快照中**不再出现源文件（证明被正确删除而非被清理误删）。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          
changes: undefined
          
现在再运行测试：


        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          
编译错误：`gzHasLast` 声明但未使用。修复：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          
**22 个测试全部通过！** ✅

关键观察：
- **TestCompressAndCleanupRace**：捕获了 622 个快照，观察到 **14 个不同的原始备份** 及其对应的 14 个 `.gz` 文件。每个生命周期追踪都验证了 `.gz` 的最后出现快照索引 ≥ 原始备份的最后出现快照索引（例如 `app.2.log` 在快照 9-21 可见，`app.2.log.gz` 在 12-117 可见），证明**源文件被正确删除而非被清理误删**。最终状态 `2 .gz backups, 0 original backups`。
- **TestConcurrentRotateWithCompress**：760 个快照，最终 `3 .gz backups, 0 original backups`。

现在更新文档，将钩子相关内容替换为文件系统轮询策略：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
现在将文档中的"测试钩子"部分替换为文件系统轮询策略：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
changes: undefined
          
最后，再跑一次测试确认一切正常，并标记所有任务完成：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 重构完成总结

所有 3 项重构任务均已完成，**22 个测试全部通过** ✅

---

### 🔧 修复 1：从生产代码移除测试钩子

**移除位置**：[LogRotator 结构体](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/logrotator/logrotator.go#L23-L36) 和 [rotate 方法](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/logrotator/logrotator.go#L216-L228)

已从生产代码中完全删除：
- `onCompressStart func(path string)` 字段
- `onCompressEnd func(path string, err error)` 字段
- `rotate` 方法中调用两个钩子的 6 行代码

`LogRotator` 结构体现在只包含业务逻辑所需的字段（`config`、`writers`、`mu`、`closed`、`ctx`、`cancel`、`cleanOnce`、`wg`、`clock`），不再暴露任何测试专用接口。

---

### 🔧 修复 2：用文件系统高频轮询替代钩子

**核心设计**：在测试侧启动独立的监控 goroutine，以 **1ms 间隔**持续轮询目标目录，采集 `fsSnapshot`（记录每个快照中未压缩 `.log` 和 `.gz` 的文件名及大小），在整个压缩/清理生命周期内追踪中间状态。

**TestCompressAndCleanupRace** — 生命周期追踪：
- 记录每个原始备份首次/最后出现的快照索引
- 记录对应 `.gz` 文件首次/最后出现的快照索引
- 断言：每个 `.gz` 的最后出现索引 ≥ 对应原始备份的最后出现索引（`.gz` 不会在源文件被删除前消失）
- 断言：监控确实捕捉到了 `.log` 和 `.gz` 的中间状态

典型测试输出（证明中间状态被观察到）：
```
captured 349 filesystem snapshots
observed 13 distinct original backups, 13 distinct .gz backups
  app.2.log: seen snapshots [10..129], app.2.log.gz: firstSeen=13 lastSeen=129
  app.13.log: seen snapshots [88..129], app.13.log.gz: firstSeen=90 lastSeen=348
final state: 1 .gz backups, 0 original backups
```

**TestConcurrentRotateWithCompress** — 4 goroutine 并发写入 + 同样的轮询策略。

---

### 📖 文档更新

[docs/logrotator.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/logrotator.md#L320-L400) 的竞态测试章节已重写：
- **设计原则**：明确"不在生产代码嵌入测试钩子"的约束
- **测试侧观察机制**：详细说明 `fsSnapshot` 结构、监控 goroutine 实现、为什么 1ms 轮询间隔有效
- **TestCompressAndCleanupRace / TestConcurrentRotateWithCompress 设计**：每个断言的含义和验证目标
- **Race Detector 使用**：说明在支持的平台上可用 `-race` 作为补充验证手段

---

### ✅ 测试结果

```
PASS: 22 / 22 tests
  ...
  TestCompressAndCleanupRace        PASS (0.48s)  — 349 snapshots, 13 个压缩生命周期追踪完成
  TestConcurrentRotateWithCompress  PASS (0.93s)  — 649 snapshots, 并发验证完成
```