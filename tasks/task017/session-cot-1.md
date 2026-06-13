让我先探索项目结构和已有代码，了解当前的实现状态。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我查看现有的 lsm 模块所有文件，了解当前的实现状态。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\config.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\entry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\level.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\memtable.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\skiplist.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\sstable.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\lsm_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
现在让我先运行现有的测试，看看当前代码的状态：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/lsm/ -v 2>&1 | Select-Object -First 200 undefined
          
我发现了几个问题。让我创建一个任务列表来系统地修复它们：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先修复 SSTable 的 footer 偏移量问题。写入时 footer 是 indexOffset(int64=8) + indexLen(int32=4) = 12 bytes，但读取时 seek 到了 fileSize-16，这是错误的。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\sstable.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\sstable.go
          
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我分析 tombstone 的问题。我发现了关键问题：**MemTable.Get 在遇到 tombstone 时直接返回 `nil, false`，导致上层继续搜索 SSTable 并返回旧版本的数据**。

正确的做法是 MemTable.Get 应该返回 tombstone entry，让上层判断是否为删除标记。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\memtable.go
          
changes: undefined
          
现在需要修改 DB.Get，让它在 MemTable 和 Immutable MemTable 返回 tombstone 时正确返回 ErrKeyNotFound：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在让我分析并发 Put 失败的问题。我发现了另一个关键 bug：在 `flushImmutable` 中，immutable memtable 在被 flush 到 SSTable **之前**就从 immutable 队列中被移除了，导致在 flush 过程中这个时间窗口内，Get 方法找不到这个 key。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我修复 flushImmutable 的问题。我会添加一个 `flushing` 字段来保存正在被 flush 的 memtable，这样搜索时不会遗漏数据。

首先修改 DB 结构体：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在修改 flushImmutable 函数，将正在被 flush 的 memtable 放到 flushing 字段：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在修改 DB.Get 方法，让它也搜索 flushing memtable：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
同样修改 DB.Range 方法也包含 flushing memtable：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在更新 DebugInfo 和 Immutable 方法，也处理 flushing 字段：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在让我运行测试看看修复效果：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/lsm/ -v 2>&1 | Select-Object -Last 100 undefined
          
还有两个测试失败。让我修复它们：

1. **TestMemTable_Basic**：测试直接调用 MemTable.Get，期望 tombstone 返回 false。我需要给 DB 层提供一个单独的方法来获取包含 tombstone 的原始 entry。
2. **TestLevel_AddAndRemove**：`NeedsCompaction` 应该是 `>=` 而不是 `>`。
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\level.go
          
changes: undefined
          
现在修复 MemTable，恢复原始的 Get 行为（过滤 tombstone），同时添加一个新方法供 DB 层内部使用：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\memtable.go
          
changes: undefined
          
现在修改 DB.Get 方法使用 `GetWithTombstone`：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\db.go
          
changes: undefined
          
现在让我再次运行所有测试：


















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/lsm/ -v 2>&1 | Select-Object -Last 80 undefined
          
所有测试通过了！现在让我创建需求文档：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先看看已有的文档格式以保持风格一致：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\kvstore.md
          
现在让我创建 `docs/lsm.md` 文档：





















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\lsm.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务已完成。以下是本次工作的完整总结：

## 已修复的 Bug

### 1. SSTable Footer 偏移量错误
- 文件：[sstable.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/lsm/sstable.go#L144)
- 问题：`LoadSSTable` 读取 footer 时 seek 到 `fileSize-16`，但实际 footer 只有 12 字节（`indexOffset` int64 = 8B + `indexLen` int32 = 4B）
- 修复：将偏移量改为 `fileSize-12`

### 2. Tombstone（墓碑）处理逻辑错误
- 文件：[memtable.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/lsm/memtable.go#L64-L84)
- 问题：`MemTable.Get` 遇到 tombstone 时直接返回 `nil, false`，导致上层 `DB.Get` 继续搜索下层 SSTable 并返回已被删除 key 的旧版本数据
- 修复：新增 `GetWithTombstone()` 方法供 DB 层内部使用（返回包含 tombstone 的原始 entry），原有 `Get()` 保持对外语义不变

### 3. 刷盘过程中数据不可见（并发幽灵丢失）
- 文件：[db.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/lsm/db.go#L15-L30)
- 问题：`flushImmutable` 在刷盘开始前就将 MemTable 从 `immutable` 队列移除，导致刷盘期间该 MemTable 中的数据对 Get/Range 不可见
- 修复：新增 `flushing` 字段作为第三层内存结构，在整个刷盘期间持有该 MemTable 引用，`Get` 和 `Range` 搜索路径均已覆盖该层

### 4. Level 合并阈值判断错误
- 文件：[level.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/lsm/level.go#L87-L91)
- 问题：`NeedsCompaction()` 使用 `len(tables) > maxSize`，应在等于阈值时就触发合并
- 修复：改为 `>=`

## 新增文档

- 文件：[docs/lsm.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/lsm.md)
- 内容涵盖：模块概述、核心功能列表、8 个核心结构体的职责说明、写入/刷盘/读取/合并/范围查询 5 大流程的完整路径图、3 项关键设计决策（三层内存结构/Tombstone/层级差异）、6 个使用示例、错误定义表、SSTable 文件格式图解、时间复杂度与写放大分析、并发安全保证、注意事项与限制

## 测试结果

执行 `go test ./internal/lsm/ -v`，**全部 52 个测试用例通过**，总耗时约 11 秒。