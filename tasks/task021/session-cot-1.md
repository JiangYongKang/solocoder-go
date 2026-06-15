我来帮你实现一个带版本控制的对象存储功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先看看现有的代码风格和模块结构，以便遵循项目的约定。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\kvstore.md
          
让我再看一个测试文件来了解测试风格。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore_test.go
          
现在我已经充分了解了项目的代码风格和测试模式。让我创建任务计划并开始实现。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我创建 objstore 模块的核心实现。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在编写完整的单元测试。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在运行测试来验证实现是否正确。


        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          
有一个编译错误，让我修复一下。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          
有 3 个测试失败了。让我分析并修复这些问题：

1. `TestCleanup_BatchSize` - 测试预期有误，因为每次 Put 后都会触发清理
2. `TestCleanupAll` - 测试预期有误，因为自动清理已经在每次 Put 后执行了
3. `TestConcurrentRollback` - 默认配置下版本会被清理，导致回滚找不到旧版本

让我修复这些测试：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          
我需要重新思考一下清理策略的设计。为了让"触发时机"和"批次大小"更有意义，让我添加一个配置项来控制是否自动清理，以及调整 CleanupAll 的测试方式。让我先修改实现：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore.go
          
让我重新设计清理机制，使"触发时机"和"批次大小"两个配置项都更有意义。我会添加 `CleanupInterval` 来控制多久触发一次清理。




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore.go
          
changes: undefined
          
现在让我更新测试文件，使其与新的实现匹配。





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          
现在让我运行测试看看是否通过。





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          
还有一个测试失败。让我分析并修复 `TestCleanup_BatchSize` - 问题是第 10 次 Put 后会触发清理，所以版本数不是 10 而是 8。让我修复测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          
等一下，让我重新计算一下。第二次清理后应该是 16 个版本而不是 15 个。让我修正测试：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          
现在让我再次运行测试。







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          
所有测试都通过了！现在让我更新任务状态并编写文档。






        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我编写模块需求文档。我会参考项目中现有的文档风格。









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\objstore.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有任务已完成！以下是实现总结：

## 实现概览

### 1. 核心模块 - [objstore.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore.go)

**核心结构体:**
- **ObjectStore**: 主存储结构体，使用 `sync.RWMutex` 保证并发安全
- **ObjectVersion**: 单个对象版本，包含版本号、数据、创建时间
- **VersionInfo**: 版本元信息（不含数据），用于列表查询
- **Config**: 配置结构体，支持 `MaxVersions`、`CleanupBatchSize`、`CleanupInterval`

**主要功能:**
| 方法 | 功能 |
|------|------|
| `Put(key, data)` | 写入对象，自动生成递增版本号 |
| `Get(key)` | 读取最新版本对象 |
| `GetVersion(key, version)` | 读取指定版本对象 |
| `ListVersions(key)` | 查询所有版本列表 |
| `Rollback(key, version)` | 回滚到指定版本（创建新版本） |
| `Delete(key)` | 删除键及其所有版本 |
| `CleanupAll()` | 手动触发全量清理 |

### 2. 单元测试 - [objstore_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore_test.go)

共 **55 个测试用例**，覆盖：
- ✅ 正常流程：写入、读取、版本管理、回滚
- ✅ 边界条件：空键、空数据、nil 数据、大对象
- ✅ 异常分支：键不存在、版本不存在
- ✅ 并发场景：并发读写、并发回滚、并发清理
- ✅ 清理策略：自动清理、批次大小、清理间隔、手动清理

### 3. 运行结果

```
PASS
ok  solocoder-go/internal/objstore  0.235s
```

所有 55 个测试全部通过。

### 4. 文档 - [objstore.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/objstore.md)

包含模块概述、核心结构体职责、对象完整生命周期、并发安全设计、使用示例、错误定义、性能特征和注意事项。