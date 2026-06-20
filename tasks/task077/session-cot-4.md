toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
现在我理解了两个问题。让我逐一修复：

**问题 1 修复思路**：重写 `TestQueryAuditLogs_ByTimeRange`，不依赖 `distinctCount` 条件跳过断言，改用基于索引的语义断言和精确边界点断言，即使 ts0==ts1==ts2 时也能完整覆盖边界逻辑。

**问题 2 修复思路**：清理 `bCreateTs`、`bSetTs` 和 `strictEnd` 未使用变量。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          
现在修复问题 2，清理未使用的变量：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          
现在运行测试验证所有修复：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/featureflag/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
两个问题已全部修复，`go test ./internal/featureflag/ -v` 全通过。

---

## 修复 1：`TestQueryAuditLogs_ByTimeRange` 边界覆盖能力

核心改动位于 [featureflag_test.go#L1005-L1199](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/featureflag/featureflag_test.go#L1005-L1199)：

- **移除条件跳过**：删除了基于 `distinctCount` 的 `if distinctCount == 3 { ... }` 和 `if ts1.After(ts0) { ... }` 条件跳过逻辑
- **新增 `StartTime=ts1` 双向断言**：无论时间戳是否相等，都验证 `StartTime=ts1` 必须包含 SET_BOOLEAN 和 late-flag CREATE
- **新增 `EndTime=ts1` 双向断言**：无论时间戳是否相等，都验证 `EndTime=ts1` 必须包含 early-flag CREATE 和 SET_BOOLEAN
- **新增 `[ts0, ts0]` 单时间戳等值范围断言**：按四种分布场景（全相等/ts0==ts1≠ts2/ts0≠ts1==ts2/全distinct）分别验证返回条数为 3/2/2/1
- **新增 `[ts2, ts2]` 单时间戳等值范围断言**：同样按四种分布场景验证
- **新增 `StartTime=ts2` 语义断言**：必须包含 late-flag CREATE
- **新增 `EndTime=ts0` 语义断言**：必须包含 early-flag CREATE
- **保留原基础断言**：宽范围全包含、±1h 全排除、±1ns 边界排除等

从测试日志可见，`allEqual=true` 时所有断言均执行并通过，不再退化跳过。

---

## 修复 2：清理未使用的中间变量

核心改动位于 [featureflag_test.go#L1546-L1547](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/featureflag/featureflag_test.go#L1546-L1547) 和 [featureflag_test.go#L1647](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/featureflag/featureflag_test.go#L1647)：

- 删除 `bCreateTs`、`bSetTs` 声明及对应的 `_ = bCreateTs`、`_ = bSetTs` 抑制语句
- 删除 `strictEnd` 声明及对应的 `_ = strictEnd` 抑制语句
- 保留 `bLogs` 切片（在"wide three-param query filters FlagKey but not time"子用例中用于验证 flag-b 过滤）