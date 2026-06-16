我来帮你实现这个 CSRF 防护中间件模块。首先让我了解一下项目结构和现有的代码风格。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\writethrough.md
          
让我先看看类似模块的代码风格，以确保实现风格一致。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\idempotent\idempotent.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rbac\rbac.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
现在我已经了解了项目结构和代码风格。让我创建一个详细的任务计划来实现 CSRF 防护中间件模块。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在开始实现核心代码：

        
            
toolName: run_command
            
status: success
          
command: mkdir -p internal/csrf undefined
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          
changes: undefined
          
核心代码已完成。现在创建单元测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          
测试文件已创建。现在运行测试来验证：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/csrf/ -v undefined
          
所有测试全部通过！现在创建需求文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\csrf.md
          
changes: undefined
          
完成！让我更新任务状态：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
CSRF 防护中间件模块已全部实现完成，测试全部通过。以下是交付内容总结：

## 实现完成

### 1. 核心代码：[csrf.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/csrf/csrf.go)

**功能实现覆盖**：

| 需求 | 实现方式 |
|------|---------|
| **同步令牌模式** | Token 存储在服务端 `map[string]*sessionToken`，支持从请求头/表单/Cookie 三种路径提取，验证后通过 |
| **双重提交 Cookie 模式** | 校验请求中 Cookie Token 与 Header Token 是否完全一致，一致才放行 |
| **会话绑定** | 双向映射 `sessions map[sessionID]token` + `tokens map[token]*sessionToken{SessionID}`，跨会话使用返回 `ErrSessionMismatch` |
| **请求来源校验** | Origin + Referer 双重校验，支持白名单（完整 URL、主机名含子域名、IP:端口 三种匹配模式） |
| **Token 轮换** | 每次验证通过后调用 `RotateToken` 原子删除旧 Token 生成新 Token，旧 Token 立即失效 |

**关键 API**：
- `NewCSRF()` / `NewCSRFWithConfig(cfg)` - 构造函数
- `Middleware(next http.Handler)` - HTTP 中间件
- `GenerateToken(sessionID)` / `ValidateToken(token, sessionID)` / `RotateToken(token, sessionID)` - Token 生命周期管理
- `InvalidateSession(sessionID)` / `InvalidateToken(token)` - 失效接口
- `GenerateHandler(w, r)` - RESTful Token 发放端点
- `CleanExpired()` - 过期清理

### 2. 单元测试：[csrf_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/csrf/csrf_test.go)

**测试结果**：`35 个测试用例 / 35 通过`，耗时 0.522s

测试覆盖范围：
- ✅ 配置参数有效性校验（5 边界条件）
- ✅ Token 生成/获取/验证/过期
- ✅ Token 轮换（含旧 Token 失效验证）
- ✅ 会话失效、Token 失效
- ✅ 过期清理
- ✅ 受保护方法判定（GET/POST/PUT/DELETE 等 9 种）
- ✅ 白名单匹配（10 种场景：完整 URL、子域名、IP、错误协议等）
- ✅ 同源判定（7 种场景）
- ✅ 同步令牌模式中间件（Header/表单提交/无 Token/跨会话 Token）
- ✅ 双重提交模式中间件（匹配/不匹配/缺失 Cookie 或 Header）
- ✅ Origin 校验（同源/白名单/非法来源/无 Header 四种情况）
- ✅ Referer 校验
- ✅ Token 轮换（同步 + 双重提交两种模式）
- ✅ 自定义错误处理器
- ✅ 并发安全（50 协程同时操作）
- ✅ Token 唯一性（1000 个会话无重复）
- ✅ 会话 ID 从 Cookie 提取、Token 从 Cookie 提取

### 3. 需求文档：[csrf.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/csrf.md)

包含 13 个章节：模块概述、核心功能、核心结构体职责、两种防护模式详解（含流程图）、会话绑定机制、来源校验机制、Token 轮换机制、4 个完整使用示例、错误定义表、完整配置说明、并发安全、生产环境最佳实践。