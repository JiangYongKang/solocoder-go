我来帮你实现 TOTP 一次性密码认证器功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我查看几个现有模块的代码风格和结构，以便遵循项目规范。

        
            
toolName: view_files
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrft\csrf.go
          

        
            
toolName: view_files
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrft\csrf_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\jwtmgr\jwtmgr.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
现在我已经了解了项目的代码风格和结构。让我查看一下现有的文档格式，然后开始实现。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\apikey.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
现在我已经了解了项目的代码风格和文档格式。让我开始实现 TOTP 认证模块。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在开始实现。首先创建 `internal/totpauth/` 目录并实现核心模块：


        
            
toolName: run_command
            
status: success
          
command: mkdir -p "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\totpauth" undefined
          
现在创建核心实现文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\totpauth\totpauth.go
          
changes: undefined
          
核心功能已实现。现在更新进度并编写完整的单元测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在编写完整的单元测试：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\totpauth\totpauth_test.go
          
changes: undefined
          
现在运行测试来验证实现：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/totpauth/ -v undefined
          
有一个测试失败了，问题在于 `decodeSecret` 函数处理已有填充的 Base32 字符串时出错。让我修复这个问题：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\totpauth\totpauth.go
          
问题是当输入已经有 `=` 填充时，我的代码会重复添加更多填充。我需要先去掉已有的填充，再重新计算。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\totpauth\totpauth.go
          
changes: undefined
          
现在重新运行测试：







        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/totpauth/ -v undefined
          
所有测试都通过了！现在更新进度并编写模块需求文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在编写模块需求文档：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\totpauth.md
          
changes: undefined
          
所有任务已完成！让我更新进度并总结：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## TOTP 一次性密码认证器模块 - 开发完成 ✅

所有 7 项需求均已全部实现：

### 📁 项目文件

| 文件 | 说明 |
|------|------|
| [internal/totpauth/totpauth.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/totpauth/totpauth.go) | 核心实现代码 |
| [internal/totpauth/totpauth_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/totpauth/totpauth_test.go) | 完整单元测试 |
| [docs/totpauth.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/totpauth.md) | 模块需求文档 |

### 🔐 核心功能

1. **TOTP 密码生成与校验**
   - 遵循 RFC 6238 / RFC 4226 标准算法
   - 支持 6-8 位数字密码（可配置）
   - 支持可配置时间步长（默认 30 秒）
   - 支持 SHA-1 / SHA-256 / SHA-512 三种哈希算法
   - 使用恒定时间比较，防止时序攻击

2. **Base32 密钥编码存储**
   - 密钥使用 `crypto/rand` 安全随机生成
   - 自动转换为 Base32 编码字符串
   - 解码时自动处理大小写、空格和填充

3. **时间窗口漂移容忍**
   - 可配置前后各容忍 N 个时间窗口
   - 默认容忍 1 个窗口（±30秒）
   - 有效解决客户端与服务端时钟不同步问题

4. **备用恢复码管理**
   - 生成一组一次性使用的恢复码
   - 每个恢复码使用后立即失效
   - 支持查询总数、剩余数、使用状态
   - 恢复码用完后可重新生成
   - 最后一个恢复码使用时返回警告

### ✅ 测试结果

所有测试全部通过：
```
PASS
ok  solocoder-go/internal/totpauth  0.219s
```

共包含 40+ 测试用例，覆盖：
- 配置验证（有效/无效配置）
- 密钥生成（非空、Base32 编码、唯一性）
- 密码生成（RFC 6238 标准测试向量验证）
- 密码校验（正常、错误、空值、异常）
- 时间漂移（当前窗口、前一窗口、后一窗口、零漂移、宽漂移）
- 不同算法（SHA1 / SHA256 / SHA512）
- 不同位数（6 / 7 / 8 位）
- Base32 解码（正常、带填充、小写、混合大小写、空格、空串、非法字符）
- 恢复码（生成、验证、使用、失效、重新生成、状态查询）
- 并发安全测试
- 完整工作流测试
- 边界条件测试（零时间、极大时间、前导零）

### 📖 文档内容

`docs/totpauth.md` 包含 10 个章节：
- 模块概述
- 核心功能
- 核心结构体与职责
- TOTP 标准算法流程（含算法图示）
- 时间窗口漂移容忍机制
- 密钥 Base32 编码存储
- 备用恢复码管理
- 使用示例（8 个完整示例）
- 错误定义
- 并发安全设计