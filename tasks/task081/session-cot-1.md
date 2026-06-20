我来帮你实现这个分布式追踪上下文传播器功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先查看一下项目的 go.mod 和一些现有模块的代码风格，以便遵循项目规范。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
好的，我已经了解了项目结构和代码风格。现在让我创建任务计划并开始实现。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在让我创建完整的 tracectx.go 文件，包含所有核心功能。

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          
changes: undefined
          
核心代码已完成。现在让我更新任务状态并编写单元测试。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在编写完整的单元测试文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在让我运行测试来验证所有功能是否正常工作。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tracectx/ -v undefined
          
有一些编译错误，让我修复它们。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在修复未使用变量的问题：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在让我重新运行测试：


        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tracectx/ -v undefined
          
所有测试都通过了！现在让我更新任务状态并编写文档。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在编写模块需求文档：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tracectx.md
          
changes: undefined
          
现在让我完成最后的任务状态更新：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
分布式追踪上下文传播器模块已全部实现完成！以下是完成的工作总结：

## 已完成的功能

### 1. TraceID/SpanID/ParentSpanID 生成与传播
- `GenerateTraceID()` 生成 16 字节（32 位十六进制）的全局唯一 TraceID
- `GenerateSpanID()` 生成 8 字节（16 位十六进制）的 SpanID
- `IsValidTraceID()` / `IsValidSpanID()` 验证 ID 格式有效性
- `NewRootContext()` 创建根上下文，`NewChildContext()` 创建子上下文

### 2. Span 树构建
- `SpanTree` 结构体支持添加、查询 Span
- `GetChildren()` 获取直接子节点
- `GetSubtree()` 递归获取完整子树
- `GetRoots()` / `AllSpans()` / `SpanCount()` 等辅助方法
- 使用 `sync.RWMutex` 保证并发安全

### 3. 采样策略
- `AlwaysSample`：全量采样
- `NeverSample`：从不采样
- `ProbabilitySampler`：概率采样（基于 TraceID 哈希，保证同一 Trace 决策一致）
- 采样决策在根 Span 创建时确定，沿调用链向下传播

### 4. W3C Trace Context 跨进程传播
- `InjectTraceContext()` 将上下文序列化为 `traceparent` 请求头
- `ExtractTraceContext()` 从请求头恢复追踪上下文
- 兼容 W3C Trace Context 标准格式：`00-<traceid>-<spanid>-<flags>`
- 支持大小写不敏感的 header 提取

## 文件结构

- 核心代码：[tracectx.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go)
- 单元测试：[tracectx_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx_test.go)
- 需求文档：[tracectx.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tracectx.md)

## 测试结果

所有 **50+ 个测试用例全部通过**，覆盖了：
- ID 生成与验证
- 三种采样策略
- Span 基本操作
- SpanTree 的增删查与并发安全
- 上下文注入/提取及各种异常格式
- 完整工作流
- 边界条件与异常分支