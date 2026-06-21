toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: audio/x-wav
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mimedetect/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
修复完成，所有 53 个测试全部通过。

## 修复内容

移除了 `builtInMIMEToExt` 和 `builtInMIMEInfo` 两个映射中残留的 `audio/x-wav` 条目：

### 1. `builtInMIMEToExt` 中移除 `"audio/x-wav": "wav"`  
   修改位置：[mimedetect.go#L471-L472](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L471-L472)

### 2. `builtInMIMEInfo` 中移除 `"audio/x-wav": {"audio/x-wav", "WAV Audio"}`  
   修改位置：[mimedetect.go#L531-L532](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L531-L532)

### 说明

`audio/x-wav` 作为 MIME 类型别名，其对应的魔术字签名已在前一轮修复中从 `builtInSignatures` 中移除（与 `audio/wav` 重复且无法被优先命中）。由于该 MIME 类型已无法被任何签名检测命中，继续在这两个映射中保留对应的条目属于不一致的遗留数据，因此一并清理以保持代码一致性。

保留的 WAV 相关条目仍然正常工作：
- 扩展名 `wav` → `audio/wav`（`builtInExtToMIME`）
- MIME `audio/wav` → `wav`（`builtInMIMEToExt`）
- `audio/wav` 的 MIME 信息（`builtInMIMEInfo`）
- `audio/wav` 的魔术字签名（`builtInSignatures`，offset=8 的 "WAVE" 标识）