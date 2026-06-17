我来逐一修复这三个问题。先查看相关代码：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func getPBFieldInfo
          
发现问题了！标签格式是 `serialize:"name,protobuf:N"`，但代码只检查 `tag` 是否以 `"protobuf:"` 开头，没有考虑到前面有字段名和逗号。让我查看完整函数并修复：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
现在修复标签解析逻辑，按逗号分割后正确提取各部分：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
changes: undefined
          
现在修复第 2 个问题：msgpack.go 中 decodeBytes 的零拷贝路径。让我查看当前的实现：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\msgpack.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\msgpack.go
          
问题找到了！`string(b)` 转换会分配新内存。既然 `b` 已经是从原始数据切片出来的 `[]byte`，直接复用它就好了：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\msgpack.go
          
changes: undefined
          
让我也检查并修复 protobuf.go 中类似的问题：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
同样的问题，修复 protobuf.go 中的零拷贝路径：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          
changes: undefined
          
现在处理第 3 个问题：TestMsgPackEdgeCases 中 12 个子测试的空函数体。让我查看这个测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestMsgPackEdgeCases
          
现在我明白了问题。子测试的函数体是空的，只定义了一个 Container 类型。我需要为每种测试值类型创建对应的容器类型并实现往返编解码断言：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\serialize_test.go
          
changes: undefined
          
现在运行测试，检查所有修复是否正确：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/serialize/ -v -count=1 undefined
          
所有测试通过！让我更新任务状态：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题已成功修复！以下是详细总结：

## 修复内容总结

### 1. Protobuf 标签解析修复 ([protobuf.go#L39-L73](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/serialize/protobuf.go#L39-L73))

**问题**：`getPBFieldInfo` 函数检查 `tag` 是否以 `"protobuf:"` 开头，但实际标签格式为 `serialize:"name,protobuf:N"`，导致用户声明的字段号被完全忽略。

**修复**：
- 使用 `strings.Split(tag, ",")` 按逗号分割标签
- 第一部分作为字段名
- 遍历剩余部分查找 `"protobuf:"` 前缀
- 正确提取用户声明的字段号
- 未指定时默认从 2 开始分配（字段号 1 保留给版本号）

### 2. MessagePack 零拷贝优化修复 ([msgpack.go#L645-L664](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/serialize/msgpack.go#L645-L664))

**问题**：`decodeBytes` 函数零拷贝路径使用 `zeroCopyBytes(string(b))`，但 Go 的 `string(b)` 转换会分配新内存并拷贝数据，导致零拷贝名存实亡。

**修复**：
- `msgpack.go` 的 `decodeBytes`：直接 `rv.SetBytes(b)` 复用从原始数据切片出的 `[]byte`
- `protobuf.go` 的 `setLengthDelimValue`：同样修复了相同问题，直接使用传入的 `data` 切片

### 3. TestMsgPackEdgeCases 子测试补充 ([serialize_test.go#L696-L928](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/serialize/serialize_test.go#L696-L928))

**问题**：12 个子测试的函数体完全为空，只定义了一个未使用的 `Container` 类型。

**修复**：为每个子测试补充：
- 对应数据类型的容器结构体
- Marshal 序列化操作
- Unmarshal 反序列化操作
- 值相等性断言（`bool`、`int`、`int64`、`float32`、`float64`、`string`、`[]byte`）

覆盖的 12 个边界用例：
- `bool true` / `bool false`
- `small int` (42) / `negative int` (-42)
- `large int` (1234567890) / `large negative int` (-1234567890)
- `float32` (3.14) / `float64` (3.1415926535)
- `empty string` / `long string` (1000 个字符)
- `byte slice` / `nil bytes`

### 测试结果

`go test ./internal/serialize/ -v -count=1` 全部通过，共 **59 个测试用例**：

```
PASS
ok      solocoder-go/internal/serialize 0.231s
```