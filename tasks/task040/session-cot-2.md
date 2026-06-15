让我分析并修复这些问题。首先制定修复计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我先读取需要修改的文件，了解当前实现：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\jwt.go
          
现在开始修复问题。首先修复 `handleAuthorizationCode` 中缺少的 Scope 子集校验：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc.go
          
changes: undefined
          
现在修复第二个问题：RefreshTokenRotation 配置未生效：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc.go
          
changes: undefined
          
现在修复第三个问题：ParseJWT 中缺少的算法 alg 校验：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\jwt.go
          
changes: undefined
          
现在添加单元测试。首先读取当前的测试文件，然后添加新的测试用例：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc_test.go
          
现在在测试文件末尾添加新的测试用例：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc_test.go
          
changes: undefined
          
现在运行测试看看是否有问题：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc_test.go
          
需要添加 `encoding/json` 到测试文件的导入中：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc_test.go
          
changes: undefined
          
现在运行测试：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/oauth2svc/ -v undefined
          
所有测试都通过了！现在更新文档：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
让我读取文档的更多部分，然后进行更新：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
让我更新文档，添加修复的内容说明：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新 Token 签发与刷新部分，添加 RefreshTokenRotation 配置说明：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新 JWT 工具部分，添加算法校验说明：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新授权码流程时序图，添加 Scope 子集校验步骤：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新刷新令牌时序图，添加滚动刷新配置说明：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
让我查看文档末尾，然后添加新的使用示例并更新安全注意事项：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
现在让我在使用示例部分添加 RefreshTokenRotation 配置的示例，并更新安全注意事项：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
让我添加新的使用示例并更新安全注意事项：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新安全注意事项：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
让我更新错误码说明，添加更详细的信息：























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在所有任务都已完成，让我更新 todo 列表：

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有问题已修复完成！以下是修复总结：

## 修复内容总结

### 1. Scope 子集校验修复
**文件**: [oauth2svc.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc.go#L172-L184)

在 `handleAuthorizationCode` 方法中添加了 Scope 子集校验逻辑，与 `handleRefreshToken` 保持一致。当用户仅授权 `read` 而令牌请求传入 `read write` 时，将返回 `ErrInvalidScope` 错误，阻止权限升级。

### 2. RefreshTokenRotation 配置生效修复
**文件**: [oauth2svc.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc.go#L254-L261)

修改 `handleRefreshToken` 方法，根据配置决定滚动刷新行为：
- `RefreshTokenRotation = true`（默认）：吊销旧令牌，签发新的刷新令牌
- `RefreshTokenRotation = false`：不吊销旧令牌，不返回新的刷新令牌，允许重复使用

### 3. JWT 算法校验修复
**文件**: [jwt.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/jwt.go#L71-L83)

在 `ParseJWT` 函数中添加了 header 解码和 `alg` 字段校验：
- 显式解码 JWT header
- 校验 `alg` 必须为 `HS256`
- 防止算法混淆攻击（如 `alg: none`、`alg: RS256` 等）

### 4. 新增单元测试
**文件**: [oauth2svc_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc_test.go#L1534-L2255)

新增 20+ 测试用例，覆盖：
- **Scope 子集校验**：权限扩大、相等、缩小、无 Scope、部分重叠、顺序无关等
- **RefreshTokenRotation 配置**：启用/禁用时的行为验证、Scope 缩小与配置的交互
- **JWT 算法校验**：HS256、none、RS256、空算法、大小写敏感、无效 Base64/JSON header

### 5. 文档更新
**文件**: [docs/oauth2svc.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/oauth2svc.md)

- 功能概述中添加 Scope 子集校验和滚动刷新配置说明
- JWT 工具部分添加算法校验说明
- 更新授权码流程时序图，添加 Scope 子集校验步骤
- 新增滚动刷新禁用时的时序图
- 添加禁用滚动刷新和 Scope 子集校验的使用示例
- 更新安全注意事项和错误码说明

## 测试结果
运行 `go test ./internal/oauth2svc/ -v`，**79 个测试用例全部通过** ✅