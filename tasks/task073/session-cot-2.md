我来修复这两个问题。先读取相关文件：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestMarshalProtobuf
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
现在我来修复这两个问题。首先修复 WriteNotAcceptable 中的非法 JSON 拼接：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
先检查测试文件的 import 部分：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
现在修复测试：增加 serialize 包导入和完善 TestMarshalProtobuf：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
changes: undefined
          
现在完善 TestMarshalProtobuf：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在运行测试验证：








        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
两个问题已全部修复，所有测试通过 ✅

---

## 修复内容

### 问题 1：WriteNotAcceptable 备选路径非法 JSON

**修改文件**：[contentneg.go#L375-L383](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentneg/contentneg.go#L375-L383)

**问题**：`supported_formats` 数组的元素未用双引号包裹，也没有数组方括号 `[]`，例如输出的是：
```
supported_formats:application/json,application/xml,application/protobuf
```

**修复**：遍历 formats 列表，逐个用双引号包裹，并在整体上加上方括号，输出合法 JSON 数组：
```
supported_formats:["application/json","application/xml","application/protobuf"]
```

---

### 问题 2：TestMarshalProtobuf 缺少往返验证

**修改文件**：[contentneg_test.go#L591-L611](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentneg/contentneg_test.go#L591-L611)

**问题**：原测试只断言了结果非空，无法检测到损坏的序列化数据。

**修复**：
1. 导入项目内的 `solocoder-go/internal/serialize` 包
2. 使用 `serialize.NewProtoBufSerializer()` 创建反序列化器
3. 将序列化后的数据再次 Unmarshal 回 `testUserPB` 结构体
4. 断言反序列化结果与原始数据的每个字段完全一致（ID、Name、Age）

与 TestMarshalJSON、TestMarshalXML 的测试模式保持一致，实现了 marshal → unmarshal → 字段比较 的完整往返验证。

---

### 测试结果

```
PASS
ok  	solocoder-go/internal/contentneg	0.318s
```

全部 27 个测试函数、100+ 子测试用例全部通过。