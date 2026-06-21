toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在来修复 RIFF 签名。核心思路是：**移除把 size 字段硬编码为 0 的 offset-0 死代码签名，改用 offset=8 的格式标识签名**（"WEBP" / "WAVE"），这样才能匹配真实文件中 size 字段可变的情况。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在修复 `audio/wav` 的签名，并移除重复的 `audio/x-wav` 魔术字签名（保留扩展名映射作为兼容别名）：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在移除 `audio/x-wav` 的魔术字签名（与 `audio/wav` 重复，永远不会命中）：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
签名修复完成。现在来添加测试用例，覆盖真实 RIFF 容器格式的检测场景：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
changes: undefined
          
测试用例添加完毕。现在运行测试验证：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mimedetect/ -v undefined
          
所有 53 个测试全部通过！让我更新任务状态：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
修复完成，所有 53 个测试全部通过。

## 修复内容总结

### 问题 1：RIFF 签名 size 字段硬编码为零（死代码）

**原因**：上一轮修复中，`image/webp` 和 `audio/wav` 的 offset-0 签名把 RIFF 容器的 size 字段（字节 4-7）硬编码为 `0x00, 0x00, 0x00, 0x00`，但真实文件中该字段存储的是文件大小，值是可变的，因此这两条签名对任何真实文件都无法命中，属于死代码。

**修复方案**：改用 **offset=8 的格式标识符签名**，这是 RIFF 容器中稳定且唯一的格式区分字段：

| 格式 | 魔术字位置 | 魔术字内容 | 说明 |
|------|-----------|-----------|------|
| `image/webp` | offset 8 | `57 45 42 50` ("WEBP") | 可匹配任意 size 值的真实 WebP 文件 |
| `audio/wav` | offset 8 | `57 41 56 45` ("WAVE") | 可匹配任意 size 值的真实 WAV 文件 |

同时移除了 `audio/x-wav` 的魔术字签名（与 `audio/wav` 完全重复，永远不会命中），仅保留扩展名映射和 MIME 信息作为兼容别名。

修改位置：[mimedetect.go#L314](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L314)、[mimedetect.go#L340](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L340)

### 问题 2：添加 RIFF 格式检测测试用例

新增 7 个测试用例，覆盖完整 RIFF 容器头部的检测场景：

- `TestDetectFromBytesWebP` — 真实 WebP 头检测（带 VP8 chunk）
- `TestDetectFromBytesWebP_DifferentSize` — 非零 size 值的 WebP 检测
- `TestDetectFromBytesWAV` — 真实 WAV 头检测（带 fmt chunk）
- `TestDetectFromBytesWAV_DifferentSize` — 不同 size 值的 WAV 检测
- `TestDetectFromBytesRIFF_WebPNotConfusedWithWAV` — WebP 不会被误识别为 WAV
- `TestDetectFromBytesRIFF_WAVNotConfusedWithWebP` — WAV 不会被误识别为 WebP

测试位置：[mimedetect_test.go#L121-L193](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect_test.go#L121-L193)