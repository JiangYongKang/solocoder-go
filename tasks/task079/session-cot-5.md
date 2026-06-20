好的，我来分析并修复这两个问题。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先确认当前 registry.go 的完整状态：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
现在开始修复。首先添加一个辅助函数生成 allMetrics 的键，并修改四个 Register 方法：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在修复 `Unregister` 方法中的 allMetrics 键：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在运行测试验证：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有测试通过。现在添加一个测试用例验证同名不同类型指标共存：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我找到测试文件的末尾，添加新的测试用例：





        
            
toolName: search_by_regex
            
status: success
          
query: ^func Test.*\(
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
现在在测试文件末尾添加同名不同类型指标共存的测试：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
现在运行测试验证：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 48 个测试通过。现在更新文档：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
更新文档：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
更新测试覆盖说明：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
最后再运行一次完整测试确认：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复完成，**48 个测试全部通过。以下是修复内容总结：

---

## 修复内容

### 问题 1：allMetrics 键冲突导致同名不同类型指标互相覆盖

**问题**：`allMetrics` 使用 `name + "\x00" + hash` 作为键，未区分指标类型。同一名称的 counter 和 gauge（同标签）会互相覆盖，导致被覆盖的指标从快照中永久丢失，变成"幽灵指标"可操作但快照不可见。

**修复**：
- 在 [registry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/registry.go#L34-L36) 中新增 `metricKey()` 辅助函数，使用 `type\0name\0hash` 格式作为键
- 四个 `Register` 方法改用 `metricKey(metricType, name, hash)` 生成键存入 `allMetrics`
- `Unregister` 方法同步使用带类型前缀的键删除
- 新增 [metrics_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/metrics_test.go#L1061-L1132) 中新增 `TestRegistry_SameNameDifferentTypes` 测试，验证同名四种类型指标共存、快照完整、注销不误删不产生幽灵指标

### 问题 2：snapshotGuardPtr 方法仅用于编译期约束无运行时价值

**问题**：`snapshotGuardPtr()` 方法从未被实际调用，只是满足接口类型约束的纯装饰方法。

**修复**：
- 在四个 `Register` 方法中调用 `xxx.snapshotGuardPtr().mu == nil` 进行运行时校验，若 guard 未正确初始化则 panic
- 这样 `snapshotGuardPtr()` 既提供编译期接口保证，又具备运行时校验价值，防止未配置的指标被错误注册

### 测试结果

所有 **48 个测试用例全部通过**：
```
PASS
ok   solocoder-go/internal/metrics   0.204s
```