我来帮你实现这个 HTTP 内容协商模块。首先让我了解一下项目的结构和现有代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\json.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\protobuf.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\serialize.go
          
让我再看几个测试文件和其他模块，确保了解代码风格：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\serialize.md
          
很好，我已经了解了项目的代码风格和结构。现在开始实现内容协商模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
首先创建包目录和核心代码文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在编写完整的单元测试文件：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
changes: undefined
          
现在让我运行测试看看是否能通过：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v undefined
          
让我查看完整的错误输出：



        
            
toolName: check_command_status
            
status: success
          
          
输出被截断了，让我用更少的测试来定位问题：

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -run "TestNewNegotiator|TestParseAcceptEmpty|TestMediaType" 2>&1 | Select-Object -First 200 undefined
          
基础测试通过了，让我测试与序列化相关的部分：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -run "TestMarshalJSON|TestMarshalXML" 2>&1 | Select-Object -First 200 undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -run "TestMarshalProtobuf" 2>&1 | Select-Object -First 300 undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -run "TestSerializationFailure" 2>&1 | Select-Object -First 300 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v -run "TestSerializationFailure" ./internal/contentneg/ undefined
          
问题找到了，是循环引用导致的栈溢出。让我移除那个有问题的测试并修复代码：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestSerializationFailure
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
我需要修改这个测试，因为 XML 循环引用会导致栈溢出。我来用更安全的方式测试序列化失败：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
changes: undefined
          
同时我需要修复内容协商逻辑中 q=0 的处理问题：q=0 表示不接受，应该被排除。让我也修复一下 WriteResponse 的错误处理，因为当协商失败时调用 WriteNotAcceptable，而 WriteNotAcceptable 的返回值也需要处理。

让我先检查 Negotiate 方法中 q=0 的处理：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          
changes: undefined
          
现在运行完整的测试：






        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v 2>&1 | Select-Object -First 400 undefined
          
我有两个测试用例写错了。q 值优先级高于 matchLevel，让我修正这两个测试：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg_test.go
          
changes: undefined
          
现在还有一个问题：当同一个媒体类型（如 `*/*`）匹配多个格式时，由于 map 遍历顺序不确定，选中的格式也不确定。让我修改排序逻辑保证确定性：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentneg\contentneg.go
          
changes: undefined
          
现在再次运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v 2>&1 | Select-Object -First 500 undefined
          
所有测试都通过了！现在来写需求文档：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\contentneg.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我最后再运行一次测试确认一切正常：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/contentneg/ -v -count=1 undefined
          
所有任务已完成！以下是实现的完整总结。

---

## ✅ 任务完成总结

### 1. 项目文件结构

| 文件 | 位置 | 说明 |
|------|------|------|
| 核心代码 | [contentneg.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentneg/contentneg.go) | 内容协商模块核心实现 |
| 单元测试 | [contentneg_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentneg/contentneg_test.go) | 完整单元测试（27个测试函数，约870+行，覆盖100+子测试用例） |
| 需求文档 | [contentneg.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/contentneg.md) | 详细的模块需求文档（约700+行） |

---

### 2. 核心功能实现

#### (1) Accept 头解析与格式选择

- **MediaType 结构体**：完整解析媒体类型、q值、扩展参数，并记录原始出现顺序
- **多级匹配支持**：支持 `*/*`、`application/*`、`application/json` 三级匹配精度
- **智能排序算法**：按 `q值降序 → 匹配精度降序 → 出现顺序升序 → ContentType字母序（确定性兜底）
- **q=0 自动排除**：q值≤0的媒体类型被视为明确拒绝，不参与候选

#### (2) 内置三种序列化格式

| 格式 | ContentType | 实现 |
|------|-------------|------|
| JSON | `application/json` | `encoding/json` 标准库 |
| XML | `application/xml` | `encoding/xml` 标准库 |
| Protobuf | `application/protobuf` | 复用项目内 `internal/serialize` 模块 |

#### (3) 406 Not Acceptable 响应

- 状态码 406，Content-Type 固定为 `application/json`
- 响应体包含：状态、错误码、消息、**服务器支持格式完整列表**
- 序列化失败时也有极简 JSON 兜底
- 406 响应不受客户端 Accept 头影响

#### (4) 灵活的 API 接口

- `Negotiate()` / `NegotiateRequest()` - 纯协商
- `NegotiateWithDefault()` - 带默认回退
- `WriteResponse()` - HTTP 标准接口自动写入
- `WriteNotAcceptable()` - 直接写入 406
- `RegisterFormat()` - 动态注册自定义格式

---

### 3. 测试覆盖

✅ **测试通过**：`go test ./internal/contentneg/ -v` 全部通过

| 测试分类 | 覆盖场景 |
|----------|---------|
| MediaType 方法 | FullType、IsWildcardAll、IsWildcardSubtype、Matches（通配/精确/大小写不敏感） |
| Accept 头解析 | 空值、单条目、多条目、带 q 值边界（负数/超1/0值/无效）、带参数、畸形输入、大小写 |
| Negotiator | 创建、注册（nil/空类型/nil函数/有效）、格式列表查询、格式查找 |
| 协商决策 | 单格式、q优先级、同q顺序优先、通配符、无匹配、q=0全排除 |
| HTTP 集成 | nil请求、带Accept头、无Accept头、带默认回退 |
| 序列化 | JSON/XML/Protobuf序列化正确性 |
| 响应写入 | JSON响应、XML响应、nil writer、406响应、指定格式写入 |
| 复杂场景 | 浏览器Accept头、API客户端Accept头、序列化失败、匹配级别优先级、边界条件 |

---

### 4. 快速使用示例

```go
import "solocoder-go/internal/contentneg"

// 创建协商器（预注册JSON/XML/Protobuf）
n := contentneg.NewNegotiator()

// HTTP Handler 中使用
func handler(w http.ResponseWriter, r *http.Request) {
    data := &User{ID: 1, Name: "Alice"}
    // 自动按 Accept 选择格式，不匹配时自动返回 406
    n.WriteResponse(w, r, http.StatusOK, data)
}
```