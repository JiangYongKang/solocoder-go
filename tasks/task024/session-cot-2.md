用户指出了测试中 mockSource 实现的几个问题。让我创建任务计划并修复：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先读取测试文件中的 mockSource 实现：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
现在修改 mockSource，让它根据 Cursor.Mode 和 LastValue 正确实现增量过滤：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          
现在修复 TestRun_CheckpointResume 测试，让它通过 Cursor 驱动 Source 定位而非手动切片：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
现在重写 TestRun_CheckpointResume，让它通过 Cursor 驱动 Source 定位：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          
现在运行测试看看修复后是否通过：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 undefined
          
测试失败，原因是 context 取消是异步的，Pipeline 在取消信号到达前已处理了更多批次。我需要改用更可控的 Stop() 机制来精确控制停止点，或者在 mock 中加入延迟同步点：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 -run TestRun_CheckpointResume undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datapipe\datapipe_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 -run TestRun_CheckpointResume undefined
          
CheckpointResume 测试通过了！现在运行完整测试套件：

















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 undefined
          
所有测试全部通过！现在更新文档：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\datapipe.md
          
让我读取 Source 接口部分，然后在适当位置添加 Cursor 契约约定：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\datapipe.md
          
现在在 Source 接口部分之后添加详细的 Cursor 契约约定说明：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\datapipe.md
          
changes: undefined
          
文档更新完成。最后再运行一次完整测试确认一切正常：

















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datapipe/ -v -count=1 2>&1 | Select-Object -Last 30 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题已修复完成！以下是修复总结：

---

## 修复内容

### 1. mockSource 增量过滤实现修复
**文件**: [datapipe_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datapipe/datapipe_test.go#L89-L141)

新增 `findStartIdx` 方法，`Fetch` 和 `Count` 不再仅依赖 `cursor.LastOffset` 切片，而是根据 `cursor.Mode` 正确实现增量过滤：

| 模式 | 定位逻辑 |
|------|---------|
| **Full 全量** | `return int(cursor.LastOffset)`（按偏移切片，与之前一致） |
| **Timestamp 时间戳** | 线性扫描找到第一条 `Record.Timestamp > cursor.LastValue.(time.Time)` 的记录 |
| **ID 递增** | 线性扫描找到第一条 `Record.SeqID > cursor.LastValue.(int64)` 的记录 |
| cursor=nil | 所有模式返回 0（全新迁移） |

同时 `Count` 方法也通过同一 `findStartIdx` 方法计算 cursor 之后的**剩余记录数**（而非总数）。

### 2. 断点续传测试重写
**文件**: [datapipe_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datapipe/datapipe_test.go#L633-L753)

`TestRun_CheckpointResume` 改为表驱动，**覆盖三种增量模式**：
- `FullMode_LastOffset`：验证全量模式 + LastOffset 分页
- `IDMode_LastValue`：验证 ID 模式 + LastValue 过滤
- `TimestampMode_LastValue`：验证时间戳模式 + LastValue 过滤

**关键改进**：
- 移除了手动切片 `src2Copy.records[loadedCursor.LastOffset:]` 的逻辑
- 第二次恢复运行时使用 `newMockSource(total)`（完整数据源），让 mockSource 自行根据 Cursor 中的 `Mode + LastValue/LastOffset` 定位起点
- 新增重复记录检测（`idSeen` map），验证无重复写入
- 新增 Cursor 最终状态断言（LastValue 类型与值验证）

测试日志证明了三种模式的正确行为：
```
[Full mode]  First run: written=50, LastOffset=50, LastValue=<nil>
[ID mode]    First run: written=50, LastOffset=50, LastValue=50
[Timestamp]  First run: written=50, LastOffset=50, LastValue=2025-01-01 00:49:00 UTC
```

### 3. 文档更新
**文件**: [datapipe.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/datapipe.md#L134-L262)

在 **2.6 Source** 章节后新增 5 个子章节：

| 章节 | 内容 |
|------|------|
| **2.6.1 Source 对 Cursor 的契约约定（强制）** | 三种模式的起点定位规则表（LastValue 类型、是否使用 LastOffset） |
| **2.6.2 增量过滤的参考实现机制** | 完整的 `Fetch` / `findStartIndex` / `Count` 伪代码 |
| **2.6.3 Batch 字段填充规范** | Records / FirstSeq-LastSeq / StartTs-EndTs 的填充要求 |
| **2.6.4 典型模式下的断点续传工作流程** | 用 ASCII 流程图展示 ID 模式下首次迁移 → 暂停 → 恢复的完整 Cursor 闭环 |
| **2.6.5 Source 实现错误后果** | 列出 4 种常见违反契约的情形及其导致的问题（重复读取、漏读、进度错误等） |

### 4. 测试结果
所有测试全部通过（29 个顶层测试 + 10 个表驱动子用例）：

```
PASS
ok      solocoder-go/internal/datapipe  0.411s
```