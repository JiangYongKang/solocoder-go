toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
changes: undefined
          
现在修复 `TestDetectFromBytesShortForOffset`，更新测试说明以反映当前实际行为（4 字节数据不足以匹配 offset=8 的签名）：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
changes: undefined
          
现在运行测试验证所有修复：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mimedetect/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
全部 53 个测试通过。以下是修复总结：

## 修复内容

### 问题 1：`TestRIFFHeaderAmbiguity` 测试数据过短

**原因**：原测试只有 8 字节数据，而 `image/webp` 和 `audio/wav` 的签名均改为 offset=8（需要至少 12 字节数据才能匹配），导致测试始终回退到 `application/octet-stream`，无法触发 RIFF 类型的实际检测路径。

**修复**：将测试数据扩展为 12 字节完整 RIFF 头部（包含偏移 8 处的 `WEBP` 格式标识），确保能够真正命中 WebP 的魔术字签名，验证 RIFF 容器格式检测路径。

修改位置：[mimedetect_test.go#L920-L936](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect_test.go#L920-L936)

### 问题 2：`validMIMEs` 残留 `audio/x-wav` 条目

**原因**：`audio/x-wav` 的魔术字签名已从 `builtInSignatures` 中移除（与 `audio/wav` 重复且永远不会命中），但 `validMIMEs` 映射中仍保留该条目，形成不一致。

**修复**：从 `validMIMEs` 中移除 `audio/x-wav`，仅保留实际可检测到的 `image/webp` 和 `audio/wav`。同时将测试断言从 `t.Logf` 改为 `t.Errorf`，确保检测失败时能真正报错。

### 问题 3：`TestDetectFromBytesShortForOffset` 测试说明不准确

**原因**：原测试的 log 消息说"多种类型共享 RIFF 头部"，但实际返回 `octet-stream` 的原因是 4 字节数据长度不足以匹配 offset=8 的签名。

**修复**：更新测试断言和说明，明确验证 4 字节 RIFF 头部因数据过短无法匹配 offset-8 签名，正确返回 `application/octet-stream`。

修改位置：[mimedetect_test.go#L248-L255](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect_test.go#L248-L255)