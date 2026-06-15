我来帮你实现一个请求幂等性中间件功能模块。首先让我了解一下项目的当前结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/go.mod
          
让我先查看项目的 go.mod 和一些现有模块的代码风格，以便遵循项目的编码规范。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway.go
          
让我再看看测试文件的风格，以及 docs 目录下的文档风格。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
现在我已经了解了项目的代码风格和文档规范。让我创建一个任务计划来实现幂等性中间件模块。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我开始实现。首先创建核心幂等性中间件代码文件。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我创建核心实现文件 `idempotent.go`：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          
changes: undefined
          
核心代码已完成。现在让我更新任务状态并开始编写单元测试。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我编写完整的单元测试文件：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent_test.go
          
changes: undefined
          
测试文件已创建。现在让我运行测试来验证代码是否正确。






        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v undefined
          
让我查看完整的测试输出：


        
            
toolName: check_command_status
            
status: success
          
          
让我重新运行测试并查看输出：






        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/idempotent/ -v 2>&1 | Out-String undefined
          
所有测试都通过了！41 个测试全部通过。现在让我更新任务状态并编写文档。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在让我编写模块需求文档：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\idempotent.md
          
changes: undefined
          
文档已编写完成。让我更新任务状态并做最后的验证。









        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有任务已完成！下面是实现总结：

## 完成情况

### 1. 核心实现 - [idempotent.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/idempotent/idempotent.go)

实现了完整的请求幂等性中间件，包含以下功能：

- **基于幂等键的请求去重**：通过 `Execute(key, handler)` 方法检查幂等键，已存在则直接返回缓存结果
- **幂等结果缓存**：缓存响应状态码（`statusCode`）和响应体（`body`），与幂等键关联存储
- **并发请求锁等待**：采用 single-flight 模式，相同幂等键的并发请求中，第一个执行业务逻辑，其余通过 channel 等待共享结果
- **过期键自动回收**：采用"惰性删除 + 定期清理"双策略，后台协程按配置间隔自动扫描清理过期记录
- **HTTP 中间件**：提供标准 `Middleware(http.Handler) http.Handler` 方法，可直接集成到 net/http 生态

### 2. 单元测试 - [idempotent_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/idempotent/idempotent_test.go)

共 **41 个测试用例**，全部通过，覆盖：
- 基础创建与配置验证
- 正常执行与缓存命中
- 边界条件（空键、nil handler 等）
- 并发锁等待机制（同键并发只执行一次）
- HTTP 中间件功能（MISS/HIT、自定义请求头）
- 过期清理与重新执行
- 生命周期管理（Start/Stop、不可逆停止）
- 并发安全与内存泄漏检测
- responseRecorder 实现验证

### 3. 模块文档 - [idempotent.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/idempotent.md)

详细的需求文档，包含模块功能清单、核心结构体职责、并发锁等待机制说明、HTTP 中间件工作流程、过期清理策略、生命周期管理、以及多个使用示例。

### 测试运行结果

```
PASS
ok      solocoder-go/internal/idempotent        5.480s
```

所有 41 个测试用例全部通过 ✓