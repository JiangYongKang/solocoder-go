我来逐一修复这两个问题。先查看当前的实现：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
现在我需要检查 Protobuf 中字段号的使用方式，特别是版本字段号和普通字段号的关系：




        
            
toolName: search_by_regex
            
status: success
          
query: fieldNum|fieldMap|field\.Index
          
让我查看测试中 Protobuf 结构体定义，确认字段号的使用方式：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\serialize_test.go
          
好的，我来同时修复这两个问题：1）回退字段号恢复为 field.Index[0]+1，2）修复省略字段名时标签格式解析：
好的，两个问题都修复：1) 回退字段号恢复为 `field.Index[0]+1，2) 省略字段名的标签格式 `serialize:"protobuf:N"` 能正确解析。现在修改 getPBFieldInfo 函数修复这两个问题：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
changes: undefined
          
现在更新任务并运行测试：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/serialize/ -v -count=1 2>&1 undefined
          
所有 59 个测试全部通过！现在标记任务完成：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有问题已成功修复！以下是详细总结：

## 修复内容总结

### 1. Protobuf 回退字段号恢复 ([protobuf.go#L69-L71](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/serialize/protobuf.go#L69-L71))

**问题**：在第二轮修复中，未声明 protobuf 标签字段的回退编号从原来的 `field.Index[0]+1` 变成了 `field.Index[0]+2`，破坏了对已编码历史数据的兼容性。

**修复**：将回退字段号恢复为 `field.Index[0] + 1`，确保历史数据仍能正确反序列化。

### 2. 省略字段名时标签解析修复 ([protobuf.go#L50-L66](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/serialize/protobuf.go#L50-L66))

**问题**：原来的解析逻辑直接将 `parts[0]` 作为字段名，然后从 `parts[1]` 开始查找 `"protobuf:"`。当用户使用 `serialize:"protobuf:N"`（省略字段名）时，`parts[0]` 就是 `"protobuf:N"`，导致字段号声明被静默跳过。

**修复**：将标签解析改为统一遍历所有逗号分隔的部分，每部分独立判断：
- 以 `"protobuf:"` 开头 → 解析字段号
- 其他情况 → 作为字段名

这样两种标签格式都能正确解析：
- `serialize:"name,protobuf:N"` → 字段名 "name"，字段号 N
- `serialize:"protobuf:N"` → 使用默认字段名，字段号 N

### 测试结果

`go test ./internal/serialize/ -v -count=1` 全部通过，共 **59 个测试用例**：

```
PASS
ok      solocoder-go/internal/serialize 0.246s
```