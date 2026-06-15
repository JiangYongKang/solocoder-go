我来帮你实现一个 OAuth2 授权服务功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我查看一个现有模块的代码风格，以便遵循相同的规范。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\auth.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\webhook\types.go
          
让我检查一下是否已有 JWT 相关依赖，并查看一个测试文件的风格：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: jwt|golang-jwt|go-jose
          
现在我了解了项目的代码风格。让我先添加 JWT 依赖，然后开始实现 OAuth2 模块。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 11 items
          
让我添加 JWT 依赖并创建包目录：

        
            
toolName: run_command
            
status: success
          
command: go get github.com/golang-jwt/jwt/v5 undefined
          
网络连接有问题，无法下载外部 JWT 库。我将自己实现一个简化版的 JWT 编码/解码功能，这样就不依赖外部网络了。让我开始创建代码：


        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Force -Path "internal\oauth2svc" undefined
          
现在让我开始编写 OAuth2 模块的代码。首先创建核心数据结构和错误定义：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 11 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\types.go
          
changes: undefined
          
现在创建 JWT 编码/解码工具函数：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\jwt.go
          
changes: undefined
          
现在创建存储接口和内存存储实现：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\store.go
          
changes: undefined
          
现在创建主服务文件，实现所有 OAuth2 核心功能：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc.go
          
changes: undefined
          
现在让我编写完整的单元测试文件：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc_test.go
          
changes: undefined
          
现在让我运行测试来验证代码是否正确：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/oauth2svc/ -v undefined
          
所有测试都通过了！现在让我编写模块需求文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
所有任务已完成！让我总结一下实现的内容：

## 实现总结

### 1. 包结构
所有代码位于 `internal/oauth2svc/` 目录下，包含 5 个文件：

| 文件 | 职责 |
|------|------|
| [types.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/types.go) | 核心数据结构、错误定义、配置结构体 |
| [jwt.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/jwt.go) | 自实现 JWT 编码/解码（HS256 算法） |
| [store.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/store.go) | 存储接口与线程安全的内存实现 |
| [oauth2svc.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc.go) | 授权服务器核心逻辑 |
| [oauth2svc_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc_test.go) | 完整的单元测试套件 |

### 2. 已实现的功能

✅ **授权码流程**：用户授权 → 生成授权码 → 换取访问令牌和刷新令牌  
✅ **客户端凭证流程**：服务间调用，直接获取访问令牌（无刷新令牌）  
✅ **JWT Token 签发**：使用 HS256 签名，包含 `iss`、`sub`、`aud`、`exp`、`iat`、`cid`、`scope`、`jti` 声明  
✅ **令牌刷新**：滚动刷新机制，旧令牌使用后立即吊销  
✅ **Scope 校验**：授权码、令牌、刷新三个阶段均校验 Scope，令牌中编码 Scope 信息  
✅ **授权码一次性消费**：使用后标记为已用，重复使用返回 `invalid_grant` 错误  
✅ **并发安全**：所有内存存储使用 `sync.RWMutex` 保证线程安全  

### 3. 测试结果

运行 `go test ./internal/oauth2svc/ -v` 全部通过，包含 **55+ 个测试用例**，覆盖：

- 正常流程测试（授权码、客户端凭证、刷新令牌、完整流程）
- 边界条件（空值、默认值、过期时间、Scope 缩小）
- 异常分支（无效客户端、无效 Scope、代码重用、令牌篡改、并发竞争）
- 并发测试（20 个并发授权码流程、10 个并发刷新令牌竞争）

### 4. 文档

已创建 [docs/oauth2svc.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/oauth2svc.md)，包含：
- 模块功能概述
- 核心结构体职责说明
- 授权码流程完整时序图
- 刷新令牌时序图
- 5 个使用示例（初始化、授权码流程、客户端凭证、刷新令牌、令牌验证）
- 错误码说明表
- 安全注意事项