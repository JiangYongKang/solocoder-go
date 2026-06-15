我来帮你实现这个缓存失效管理器功能模块。首先让我了解一下当前项目的结构和代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/internal/**/*.go
          
让我查看几个现有模块的代码风格，以便保持一致。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\eventbus\eventbus.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我再看看测试文件的风格：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\eventbus\eventbus_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\kvstore.md
          
好的，我已经了解了项目结构和代码风格。现在开始实现缓存失效管理器模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在开始创建核心代码文件。首先创建包目录和主要实现：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
我注意到 RemoveListener 的实现有问题，让我先修复它，然后再写单元测试。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新构造函数和相关方法：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新 AddListener、RemoveListener 和 PublishEvent 方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
好的，核心代码已经完成。现在让我更新 todo 列表并开始编写单元测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在编写完整的单元测试文件：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
changes: undefined
          
现在运行测试来验证代码是否正确：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cacheinvalid/ -v undefined
          
太好了！所有测试都通过了。现在开始编写模块需求文档：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在编写模块需求文档：










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在让我更新 todo 列表并做最终确认：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
所有任务已完成！让我给你做一个完整的总结：

## 完成总结

### 1. 核心实现文件

**[cacheinvalid.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid.go)**

实现了以下核心功能：

| 功能 | 说明 |
|------|------|
| **TTL 惰性过期** | 每个缓存条目设置 TTL，`Get` 时检查是否过期，过期则惰性删除，无需后台线程 |
| **事件通知失效** | 支持注册 `InvalidationListener` 监听器，数据变更时发送失效事件主动删除缓存 |
| **缓存预加载** | 通过 `PreloadLoader` 接口预加载热点数据，预加载数据跳过 TTL 检查 |
| **热点数据标记** | 支持手动/自动标记热点数据，永不过期，仅显式失效或手动清除时移除 |
| **容量管理** | 支持最大条目数限制，FIFO 淘汰策略，保护热点和预加载数据 |

### 2. 单元测试

**[cacheinvalid_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid_test.go)**

共 **58 个测试用例**，全部通过：

```
PASS
ok      solocoder-go/internal/cacheinvalid      0.811s
```

测试覆盖范围：
- 基本 CRUD 操作
- TTL 惰性过期与主动清理
- 事件监听与发布
- 缓存预加载
- 热点数据标记与自动识别
- 容量限制与淘汰策略
- 并发安全测试
- 边界条件与异常分支

### 3. 需求文档

**[cacheinvalid.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cacheinvalid.md)**

完整的模块需求文档，包含：
- 模块概述与核心功能列表
- 核心结构体职责说明
- 缓存条目完整生命周期（流程图 + 阶段说明）
- 各功能机制的详细设计原理
- 9 个使用示例（基本使用、TTL 过期、事件失效、预加载、热点标记、自动热点、主动清理、并发使用等）
- 错误定义、性能分析、注意事项
- 架构设计权衡与可扩展点
- 4 个典型应用场景

### 运行测试

你可以随时运行以下命令验证测试：

```bash
go test ./internal/cacheinvalid/ -v
```