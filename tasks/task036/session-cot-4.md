我来先确认代码实现，然后修正文档。首先查看 `handleRefreshToken` 和 `createTokenResponse` 的实现：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\oauth2svc.go
          
确认了代码行为。从 [oauth2svc.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/oauth2svc/oauth2svc.go#L250-L258) 可以看到：当 `RefreshTokenRotation=false` 时调用 `createTokenResponse` 传入 `includeRefresh=false`，此时不会设置 `RefreshToken` 字段，而该字段带有 `omitempty` 标签，响应中会完全省略该字段，而非返回旧令牌。

现在修正文档：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在同步更新第 1.3 节的文字描述，确保一致性：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
现在更新 4.4 节刷新令牌示例的注释，使其更准确：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\oauth2svc.md
          
changes: undefined
          
文档已修正完毕，以下是修改总结：

## 修复内容

### 1. 非滚动模式刷新令牌时序图修正
**文件**：[oauth2svc.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/oauth2svc.md#L206-L231)

- 第 222 行：将 `不生成新 refresh_token` 改为 `不返回 refresh_token`，准确反映代码行为
- 第 229 行：将响应体中的 `"refresh_token": "OLD_REFRESH"` 改为 `"scope": "read write"`，与实际代码一致。当 `includeRefresh=false` 时，`TokenResponse.RefreshToken` 字段因 `omitempty` 标签被完全省略

### 2. 功能描述同步更新
**文件**：[oauth2svc.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/oauth2svc.md#L22-L24)

- 滚动模式：补充说明「并在响应中返回」新刷新令牌
- 非滚动模式：明确说明「响应中不包含 `refresh_token` 字段」

### 3. 示例代码修正
**文件**：[oauth2svc.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/oauth2svc.md#L347-L352)

- 注释中补充说明非滚动模式下响应不包含 `refresh_token` 字段
- 将直接打印 `RefreshToken` 改为条件判断后打印，避免使用者误以为非滚动模式下一定有值