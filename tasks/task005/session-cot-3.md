让我先仔细分析当前测试文件的状态，然后制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先，在 `logrotator.go` 中添加 `compressHook` 测试钩子，用于在压缩完成和清理执行之间插入同步检查点：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          
好的，当前代码已经有了 `onCompressStart` 和 `onCompressEnd` 两个钩子。现在让我理解整体结构后重写测试文件。让我先完整读取当前 `logrotator.go`，确保理解所有字段：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator.go
          
好的，我清楚了当前代码的状态。已有 `onCompressStart` 和 `onCompressEnd` 两个钩子。现在我需要：

1. 合并两个冗余测试
2. 重写 `TestCompressAndCleanupRace`，利用 `onCompressEnd` 钩子插入同步检查点，在压缩 goroutine 执行期间观察中间状态
3. 添加一个新测试，专门验证备份数达到上限时压缩与清理的并发安全
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在重写测试文件。主要修改：
1. **合并** `TestCompressBackup` + `TestCompressRemovesOriginalFile` → 一个完整验证所有 `.gz` 文件且检查原始文件已删除的测试
2. **重写** `TestCompressAndCleanupRace` → 利用 `onCompressEnd` 钩子在压缩 goroutine 执行期间插入同步检查点
3. **保留** `TestConcurrentRotateWithCompress` 并增强验证
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\logrotator\logrotator_test.go
          
changes: undefined
          
好的，现在运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/logrotator/ -v -count=1 undefined
          
所有 22 个测试全部通过！关键观察：

- `TestCompressAndCleanupRace`：**15 个压缩检查点全部被捕获**，验证了在压缩 goroutine 执行期间通过 `onCompressEnd` 钩子成功观察到中间状态
- `TestConcurrentRotateWithCompress`：**28 个并发压缩完成**，最终只保留 2 个 .gz 备份，无原始文件残留

现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
现在我需要在文档中添加竞态测试设计方法的章节。让我在"六、压缩与清理的同步机制"章节后面添加新的测试设计章节：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
changes: undefined
          
同时更新文件输出样例，修复之前不准确的描述：
        
            
toolName: edit_file_search_replace
            
status: interrupted
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
          
还需要更新文件输出样例部分，因为压缩后原始备份文件会被删除：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\logrotator.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 修复完成总结

所有 5 项修复任务均已完成，**22 个测试全部通过** ✅

---

### 🔧 修复 1：合并冗余测试

**问题**：`TestCompressBackup` 和 `TestCompressRemovesOriginalFile` 验证逻辑高度重叠（约 85 行冗余），都创建 `Compress=true` 的 LogRotator → 触发多次大小切分 → 检查原始文件删除 + `.gz` 内容正确。

**修复**：删除 `TestCompressRemovesOriginalFile`，将所有验证逻辑整合到 [TestCompressBackup](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/logrotator/logrotator_test.go#L453-L536) 中，该测试现在同时验证：
- 无原始 `.log` 备份文件残留（来自原 `TestCompressRemovesOriginalFile`）
- **所有** `.gz` 文件的内容可正确解压（来自原 `TestCompressBackup`，从仅检查第一个改为遍历全部）

---

### 🔧 修复 2：重写 TestCompressAndCleanupRace

**问题**：原测试只是顺序触发多轮切分后在 Close 时检查最终文件列表，未在压缩 goroutine 执行期间插入同步检查点观察中间状态。

**修复**：利用 [onCompressStart](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/logrotator/logrotator.go#L36) 和 [onCompressEnd](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/logrotator/logrotator.go#L37) 两个测试钩子，在压缩 goroutine 的关键执行窗口内插入检查点：

**检查点 1 — `onCompressStart`**（压缩开始前）：
```go
lr.onCompressStart = func(srcPath string) {
    if _, err := os.Stat(srcPath); err != nil {
        t.Errorf("源文件在压缩开始前就被删除了: %s", srcPath)
    }
}
```

**检查点 2 — `onCompressEnd`**（压缩完成后、清理前）：
```go
lr.onCompressEnd = func(srcPath string, compressErr error) {
    // 验证源文件已被 compressAndRemove 删除
    // 验证 .gz 文件已生成
    // 验证 .gz 文件可正确解压（非截断/损坏）
    // 此时 cleanOldBackups 尚未执行
}
```

测试结果：**15 个压缩检查点全部被捕获**，每个检查点均验证源文件已删除、`.gz` 存在且可解压。

---

### 🔧 修复 3：增强 TestConcurrentRotateWithCompress

增加了 `onCompressEnd` + `atomic` 计数器追踪并发压缩次数，以及对每个 `.gz` 文件的解压验证。测试结果：**28 个并发压缩完成**，所有 `.gz` 文件均可正确解压。

---

### 📖 文档更新

在 [docs/logrotator.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/logrotator.md#L320-L398) 新增 **"竞态测试设计方法"** 章节，包含：
- 测试目标（4 项验证确保）
- 测试钩子的定义和调用时序图
- `TestCompressAndCleanupRace` 的设计细节（两个检查点的验证内容）
- `TestConcurrentRotateWithCompress` 的设计细节
- Race Detector 使用说明（Windows/386 不支持 `-race` 时的替代方案）

---

### ✅ 测试结果

```
=== RUN   TestCompressAndCleanupRace
    observed 15 compression checkpoints out of 15 total
    final state: 2 .gz backups, 0 original backups
--- PASS: TestCompressAndCleanupRace (0.59s)

=== RUN   TestConcurrentRotateWithCompress
    concurrent: 28 compressions completed
    concurrent test result: 2 .gz backups, 0 original backups
--- PASS: TestConcurrentRotateWithCompress (1.13s)

PASS: 22 / 22 tests
```