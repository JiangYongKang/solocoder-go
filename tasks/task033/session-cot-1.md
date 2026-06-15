我来帮你实现这个 gRPC 服务框架功能模块。首先让我了解一下当前项目的结构和代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我查看几个现有模块来了解代码风格和模式。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\servicereg\servicereg.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wscenter.md
          
现在我了解了项目的代码风格和文档模式。让我开始实现 gRPC 服务框架模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我创建 gRPC 服务框架的核心实现文件。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
核心实现已完成。现在让我编写完整的单元测试。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我编写完整的单元测试文件。






        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
现在让我运行测试，看看有哪些问题需要修复。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 | Select-Object -First 100 undefined
          
我看到有类型不匹配的问题。让我修复这些错误。







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
我看到问题了。让我修复类型不匹配的问题——`StreamHandler` 接受 `Stream` 类型参数，但拦截器链内部使用的是 `ServerStream`。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我修复 `HandleStream` 中的类似问题。







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我修复 `ChainStreamInterceptors` 函数。







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我修复测试文件中的类型不匹配问题。








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
现在让我再次运行测试，看看是否还有其他问题。










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
有两个测试失败了。让我来修复它们：

1. `TestRegisterService` - 方法数量断言错误（实际是 2 个一元 + 3 个流 = 5 个）
2. `TestTrailer` - trailer 没有正确返回到调用者的上下文








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
现在让我修复 trailer 的问题。问题是 `Invoke` 内部创建的新上下文没有返回给调用者。让我调整设计，让 trailer 能够正确传递。










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
让我修改 trailer 的设计，使用户可以在调用前创建带 trailer 的上下文。









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我更新 `Invoke` 和 `NewStream` 方法，使其正确使用已有的 trailer 上下文。












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我更新 `NewStream` 方法。












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在让我更新测试文件中的 `TestTrailer` 测试。















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
现在让我再次运行测试。
























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
所有测试都通过了！现在让我编写模块需求文档。















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我编写模块需求文档。


















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
现在让我更新 todo 列表并做最终验证。

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务已完成！以下是实现总结：

## 实现总结

### 1. 核心实现文件
**[grpcsvc.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go)** - gRPC 服务框架核心实现

### 2. 实现的功能模块

**服务注册**
- `RegisterService()` - 注册服务，指定服务描述符和服务实现
- `GetService()` - 查询已注册的服务信息
- 支持一元方法和流式方法的注册

**一元调用与流式调用**
- `Invoke()` - 处理一元 RPC 请求（单次请求-单次响应）
- `NewStream()` + `HandleStream()` - 处理流式 RPC
- 支持服务端流式、客户端流式、双向流式三种模式

**拦截器链**
- `AddUnaryInterceptor()` - 添加一元拦截器
- `AddStreamInterceptor()` - 添加流式拦截器
- 采用洋葱模型，按注册顺序执行
- 提供 `ChainUnaryInterceptors()` 和 `ChainStreamInterceptors() 工具函数

**超时传播**
- 使用 `context.Context` 的 Deadline 机制
- 在请求入口、处理器执行前、流操作等多点校验
- 超时返回 `ErrDeadlineExceeded` 错误

**元数据透传**
- `MD` 类型 - 键值对元数据（支持多值）
- `NewContextWithMD()` / `FromContext()` - 请求元数据
- `NewContextWithTrailer()` / `SetTrailer()` / `TrailerFromContext()` - 响应元数据

### 3. 单元测试
**[grpcsvc_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc_test.go)** - 65 个测试用例全部通过

覆盖范围：
- **正常流程**：服务注册、一元调用、流式调用、拦截器链、元数据、并发操作
- **边界条件**：空参数、nil 指针、重复注册、空拦截器链、双重关闭
- **异常分支**：服务不存在、方法不存在、超时、服务器已停止、流已关闭、认证失败

### 4. 模块需求文档
**[grpcsvc.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/grpcsvc.md)** - 完整的需求文档

包含：模块概述、功能清单、核心结构体说明、核心流程图、使用示例、文件结构、测试覆盖范围

运行 `go test ./internal/grpcsvc/ -v` 可以看到所有 65 个测试用例全部通过 ✅