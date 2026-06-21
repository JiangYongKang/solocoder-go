让我先查看 MIME 类型检测模块的代码：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/mimedetect/*
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
现在我已经了解了全部代码。让我分析需要修复的问题：

**问题 1**：RIFF 容器格式 - 三个签名 (`image/webp`, `audio/wav`, `audio/x-wav`) 都只使用前 4 字节 `0x52, 0x49, 0x46, 0x46` (即 "RIFF")，但 RIFF 容器中偏移量 8 位置有区分不同格式的标识：
- WebP: 偏移 8 处是 "WEBP"
- WAV: 偏移 8 处是 "WAVE"

**问题 2**：`ErrNilMagicSignature` 是死代码，从未被返回。

现在让我修复这些问题：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在修改 audio/wav 和 audio/x-wav 的签名：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在删除死代码 `ErrNilMagicSignature`：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在我来运行测试检查是否通过：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mimedetect/ -v undefined
          
所有测试都通过了！让我更新 todo 状态：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
两个问题都已修复完成，所有 46 个测试全部通过。

## 修复内容总结

### 问题 1：RIFF 容器签名冲突（WebP vs WAV）

**原因**：`image/webp`、`audio/wav`、`audio/x-wav` 三个签名都只使用前 4 字节 `0x52, 0x49, 0x46, 0x46`（即 ASCII "RIFF"），而这是多种格式共享的 RIFF 容器前缀。按顺序扫描时 WAV 被排在前面的 WebP 签名误捕获。

**修复方案**：利用 RIFF 容器偏移 8 位置的格式标识字节区分不同格式：

- **WebP**：偏移 8 处是 `0x57, 0x45, 0x42, 0x50`（"WEBP"）
  - 签名 1：完整 `RIFF....WEBP` 12 字节（偏移 0）
  - 签名 2：`WEBP` 4 字节（偏移 8），兼容数据长度字段变化
  
- **WAV**（`audio/wav`）：偏移 8 处是 `0x57, 0x41, 0x56, 0x45`（"WAVE"）
  - 签名：完整 `RIFF....WAVE` 12 字节（偏移 0）
  
- **WAV**（`audio/x-wav`）：偏移 8 处是 `0x57, 0x41, 0x56, 0x45`（"WAVE"）
  - 签名：`WAVE` 4 字节（偏移 8）

修改位于 [mimedetect.go#L315-L316](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L315-L316)、[mimedetect.go#L342](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L342)、[mimedetect.go#L350](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L350)

### 问题 2：死代码 `ErrNilMagicSignature`

**原因**：`RegisterMagicSignature` 接收值类型 `MagicSignature`，Go 中值类型参数不可能为 nil，该错误的触发路径结构上不可达。

**修复方案**：删除 `ErrNilMagicSignature` 变量声明。
修改位于 [mimedetect.go#L16-L21](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L16-L21)