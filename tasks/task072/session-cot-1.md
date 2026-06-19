我来帮你实现这个声明式 REST HTTP 客户端功能模块。首先让我了解一下项目的现有结构。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: go.mod
          
让我查看一下项目的 go.mod 和一些现有的模块，了解代码风格和规范。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
让我再看看文档的格式，以及其他模块的结构，确保代码风格一致。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\retry.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\httplb\httplb.go
          
现在我对项目结构和代码风格有了充分了解。让我开始实现 restclient 包。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
首先创建目录并实现核心代码。


        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Path "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient" -Force undefined
          
目录创建成功。现在让我创建核心实现文件。





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
核心代码已创建。现在开始编写单元测试，使用 `net/http/httptest` 提供的内存 HTTP 服务器来模拟网络请求。






        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
现在让我运行测试看看是否通过。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v undefined
          
我看到有几个测试失败了。主要问题有两个：
1. 非可重试错误（如路径参数缺失、认证提供器不存在）仍然被重试并包装了 `ErrMaxRetriesExceeded`
2. 路径参数特殊字符测试中，`r.URL.Path` 返回的是解码后的路径，不是原始编码路径

让我修复这些问题。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
我需要修复两个问题：
1. 非可重试错误（如请求构建失败、认证提供器缺失）应该直接返回，不应该重试
2. 路径参数特殊字符测试需要检查原始路径而不是解码后的路径

让我修复代码：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在让我修复测试中路径参数特殊字符的检查方式，并再次运行测试。










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
现在重新运行测试。









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v undefined
          
很好，大部分测试通过了！还剩 2 个测试失败，是因为错误没有正确地链式包装。让我修复错误包装，使得 `errors.Is` 能够穿透错误链。












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在让我添加 `requestBuildError` 结构体。











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在再次运行测试。











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v undefined
          
所有测试都通过了！现在让我更新任务列表并开始编写文档。














        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我编写模块需求文档。












        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\restclient.md
          
changes: undefined
          
文档已编写完成。让我最后再运行一次测试，确保一切正常。












        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v -count=1 undefined
          
所有 42 个测试用例全部通过！以下是完成的工作总结：

## 已完成的工作

### 1. 核心代码实现 — [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go)

**主要结构体**:
- **`Client`** — HTTP 客户端主结构体，管理模板和认证提供器
- **`RequestTemplate`** — 请求模板，包含 HTTP 方法、路径、默认头、超时、重试配置
- **`RequestOptions`** — 单次请求的动态参数（路径参数、查询参数、请求头、请求体）
- **`AuthProvider`** 接口 — 认证提供器，支持自定义注入逻辑

**核心功能**:
| 功能 | 说明 |
|------|------|
| 模板注册 | `RegisterTemplate()` 注册模板，同名覆盖，自动归一化默认值 |
| 路径参数 | `{param}` 占位符 + `url.PathEscape` 编码 |
| 查询参数 | `url.Values` 编码，自动处理 `?` 和 `&` 拼接 |
| 请求头合并 | 模板默认头 + 请求额外头，追加模式 |
| 认证注入 | 全局认证提供器，请求前自动注入 |
| 请求超时 | 模板级 `Timeout`，使用 `context.WithTimeout` |
| 自动重试 | 固定间隔重试，支持 Context 中断 |
| 错误链 | `requestBuildError` 支持 `errors.Is` 穿透检查 |

### 2. 单元测试 — [restclient_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go)

共 **42 个测试用例**，使用 `net/http/httptest` 内存 HTTP 服务器模拟网络请求，覆盖：
- 模板注册/获取/注销/覆盖
- 路径参数（正常、缺失、部分缺失、特殊字符编码）
- 查询参数（正常、特殊字符）
- 请求头合并与优先级
- 认证提供器注入
- 请求超时
- 重试机制（首次成功、重试后成功、超次数、零重试、Context 取消）
- 请求体发送
- 基础 URL 拼接与斜杠处理
- 并发安全
- 错误链式检查

### 3. 模块文档 — [restclient.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/restclient.md)

详细文档包含：模块概述、核心功能、核心结构体职责、请求模板生命周期、参数绑定流程、认证注入机制、重试与超时机制、使用示例、错误定义、并发安全、最佳实践。

**运行测试命令**:
```bash
go test ./internal/restclient/ -v
```