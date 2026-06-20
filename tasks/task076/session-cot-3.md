让我先查看当前代码，然后修复这两个问题。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
现在让我看看 `callFunction` 的完整错误处理部分：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
好的，我来逐一修复这两个问题。

**问题 1 修复：`callFunction` 对值类型 error 的支持**

使用 `Type().Implements(errorType)` 替代 Kind 白名单检查，正确处理值类型结构体实现 error 接口的情况：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在修复 `callFunction` 中第二个返回值的错误处理：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在添加 `canBeNil` 辅助函数：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: func isTruthy
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
**问题 2 修复：继承链循环检测避免不必要的 GetTemplate**

添加一个轻量级辅助方法，直接从原始模板源中提取 extends 父模板名，避免完整解析和缓存写入：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在用 `getExtendsParent` 替换循环检测中的 `GetTemplate`：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在让我编译并运行测试：





        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go build ./internal/templater/ undefined
          
编译成功，运行测试：







        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v -count=1 2>&1 undefined
          
所有 65 个测试全部通过！现在让我再添加一个测试用例来验证值类型结构体实现 error 接口的场景，确保修复 1 的正确性：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          
changes: undefined
          
等等，值类型 error 不能为 nil，所以零值结构体也会被当作错误返回。让我调整一下测试用例，使其符合 Go 的语义：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          
changes: undefined
          
现在运行所有测试验证：









        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v -count=1 2>&1 undefined
          
所有 **66 个测试全部通过** ✅

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## 修复完成总结

### 修复 1：`callFunction` 值类型 error 支持

**问题**：`callFunction` 方法使用 Kind 白名单（仅 `Interface` 和 `Ptr`）来判断第二个返回值是否为 error，导致值类型结构体（如 `MyError struct`）实现了 error 接口时被误判为非 error 类型，返回 `ErrInvalidFunctionCall`。

**修复**：
- 新增 `errorType` 全局变量，通过 `reflect.TypeOf((*error)(nil)).Elem()` 获取 error 接口类型
- 新增 `canBeNil(kind) bool` 辅助函数，判断一个 Kind 是否可以为 nil
- 将错误检测逻辑改为：先用 `Type().Implements(errorType)` 判断类型是否实现了 error 接口，再根据 Kind 判断是否可 nil 来决定是否调用 `IsNil()`

**文件**：[templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L11-L12) (errorType), [templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L863-L871) (canBeNil), [templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L992-L1007) (callFunction error handling)

### 修复 2：继承链循环检测避免不必要的 GetTemplate

**问题**：`RenderWithVisited` 中循环检测继承链时，对每个祖先模板都调用了 `GetTemplate`，导致祖先模板被完整解析并写入缓存，但这些祖先模板并不参与后续的块合并与渲染，引入了不必要的解析和缓存副作用。

**修复**：
- 新增 `getExtendsParent(name string) (string, bool, error)` 私有方法，直接从原始模板源字符串中用 `extendsPattern` 正则提取父模板名，避免完整解析和缓存
- 将循环检测从 `GetTemplate` 改为使用 `getExtendsParent`
- 保留对直接父模板的 `GetTemplate` 调用（因为渲染需要它的块定义）

**文件**：[templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L53-L65) (getExtendsParent), [templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L126-L143) (cycle detection loop)

### 新增测试

- `TestFunctionValueTypeError`：验证值类型结构体实现 error 接口时能正确返回错误，而非 `ErrInvalidFunctionCall`

### 测试结果

**66 个测试全部通过** ✅
运行 `go test ./internal/templater/ -v -count=1` 验证通过。