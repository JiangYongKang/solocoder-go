我来修复这些问题。先创建一个修复计划：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取当前代码以了解具体实现：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          
现在我清楚了两个问题。让我开始修复：

**问题 1**：`rollbackInternal` 没有检查迁移是否真的已应用。需要先查询已应用版本，在回滚前检查。

**问题 2**：`Range` 方法返回原始指针，需要返回副本。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          
changes: undefined
          
现在修复 `rollbackInternal` 方法，添加已应用版本检查：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我添加测试用例。先读取测试文件末尾以找到合适位置：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          
现在我在测试文件末尾添加新的测试用例：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          
changes: undefined
          
我注意到测试中有一个拼写错误 `Mutation` 应该是 `Migration`，让我修复：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/schemamig/ -v undefined
          

        
            
toolName: check_command_status
            
status: success
          
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/schemamig/ -v 2>&1 | ForEach-Object { $_; [System.Console]::Out.Flush() } undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/schemamig/ undefined
          
测试出现死锁。问题是测试代码中持有 `exec.mu.Lock()` 后又调用 `CurrentVersion()`，而 `CurrentVersion()` 内部又会调用 `executor.Exec()`，导致同一 goroutine 尝试重复获取同一把锁。

让我修复这些测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          
changes: undefined
          
让我检查其他测试是否也有类似问题：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\schemamig\schemamig_test.go
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go; go test ./internal/schemamig/ -v -timeout 120s undefined
          
全部 61 个测试通过！现在更新文档：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我读取文档，找到合适位置插入更新内容：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
先更新 `Range` 方法说明，添加"返回副本"：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
changes: undefined
          
让我读取文档的回滚部分：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
现在更新回滚流程图，添加已应用版本检查步骤：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
changes: undefined
          
现在让我在文档中添加关于回滚安全性和指针返回策略的专门章节。先找到合适位置：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
让我找到"迁移锁机制详解"之后的位置：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
在"迁移锁机制详解"之后、"使用示例"之前添加新章节：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
changes: undefined
          
现在更新测试覆盖部分，添加新增的测试用例说明：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
更新测试覆盖部分，添加新增的测试用例：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\schemamig.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有修复已完成！以下是修复内容总结：

## 问题修复总结

### 问题 1：回滚安全性修复 - [schemamig.go L413-L454](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/schemamig/schemamig.go#L413-L454)

**修复前**：`rollbackInternal` 方法只检查 `targetVersion > currentVersion`，然后逆序遍历所有注册的迁移脚本执行回滚。如果注册中心包含版本 1-5，但仅通过 `UpTo(3)` 执行了前 3 版，调用 `Rollback(1)` 会错误地对版本 4 和 5 也执行回滚 SQL。

**修复后**：
1. 先调用 `getAppliedVersions()` 获取所有已应用的版本
2. 在逆序遍历每个迁移脚本时，先检查 `if _, ok := applied[mig.Version]; !ok`，未应用则 `continue` 跳过
3. 只有真正已应用的迁移才会执行回滚 SQL

### 问题 2：Range 方法返回副本 - [schemamig.go L136-L148](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/schemamig/schemamig.go#L136-L148)

**修复前**：`Range` 方法直接返回内部 `r.migrations[v]` 原始指针，调用者可通过返回值修改注册中心内部数据。

**修复后**：通过值拷贝创建副本：
```go
m := *r.migrations[v]
result = append(result, &m)
```
与 `All()` 方法保持一致的返回策略。

### 新增测试用例 - [schemamig_test.go L1429-L1609](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/schemamig/schemamig_test.go#L1429-L1609)

新增 5 个测试用例：
1. `TestMigrator_Rollback_PartiallyAppliedMigrations` - 注册 5 个版本仅应用前 3 个，回滚时跳过未应用版本
2. `TestMigrator_Rollback_PartiallyApplied_SkipUnapplied` - 注册 5 个版本仅应用前 2 个，回滚到 0 时验证 DOWN 执行次数
3. `TestRegistry_RangeReturnsCopy` - 验证 Range 返回副本，修改返回值不影响内部数据
4. `TestMigrator_Rollback_UnappliedVersionsIgnored` - 验证未应用版本的回滚被正确忽略
5. `TestMigrator_Rollback_NoDownSQLOnlyForApplied` - 未应用版本的 DownSQL 为空不触发错误

### 文档更新 - [schemamig.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/schemamig.md)

1. 更新回滚阶段流程图，添加"版本已应用？──否──► 跳过"检查步骤
2. 新增"**回滚安全性保证**"章节，说明已应用版本检查机制和修复前后对比
3. 新增"**指针返回策略**"章节，说明 `All()`/`Range()`/`Versions()`/`Get()` 各方法的返回策略
4. 更新测试覆盖表格，添加"部分应用回滚"和"指针安全"两项

### 测试结果

全部 **61 个测试通过**（原有 56 个 + 新增 5 个），`go test ./internal/schemamig/ -v` 运行成功。