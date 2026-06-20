修复 WriteNotAcceptable 备选路径中的 message 字段不一致问题：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          
changes: undefined
          
运行测试验证：









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -count=1 undefined
          
已修复，所有测试通过 ✅

---

## 修复内容

**修改文件**：[contentneg.go#L381](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentneg/contentneg.go#L381)

**问题**：WriteNotAcceptable 备选 JSON 拼接路径中 `message` 字段内容不一致
- 主路径：`No acceptable representation found for the requested resource.`
- 原备选：`No acceptable representation found.`（缺少 `for the requested resource` 部分

**修复**：将备选路径的 message 统一改为与主路径和文档定义完全一致：
```
No acceptable representation found for the requested resource.
```