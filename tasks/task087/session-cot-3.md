让我先查看当前的代码，然后修复这些问题。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先查看当前 mockgen.go 的内容，识别未使用的类型和函数：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
现在我可以清楚地看到问题。让我先删除未使用的类型和函数，然后重新设计 Instance() 方法的实现。

首先，识别未使用的代码：
- `MethodCaller` 接口（第 154-156 行）
- `mockInterfaceWrapper` 结构体（第 257-259 行）
- `GetMockProxy()` 方法（第 319-321 行）
- `zeroValue()` 函数（第 323-325 行）

让我删除这些并重新实现 Instance() 方法：

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我更新 todo 列表，然后查看测试文件，了解需要通过的测试：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先运行测试，看看当前的实现情况：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v 2>&1 | Select-Object -First 150 undefined
          
现有测试都通过了。现在让我查看测试文件，看看是否有关于 Instance() 类型断言的测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
现在让我添加测试来验证 Instance() 和 As[T] 的功能：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          
现在让我运行测试，看看 As[T] 函数是否正确工作：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestMockProxy_Instance|TestAs_GenericMock" 2>&1 undefined
          
需要在测试文件中添加 reflect 包的导入：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestMockProxy_Instance|TestAs_GenericMock" 2>&1 undefined
          
`As[T]` 函数崩溃了。问题在于我从函数值中提取代码指针的方式不正确。让我修复 `buildItab` 函数：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我运行测试，看看修复是否有效：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestAs_GenericMock" 2>&1 undefined
          
我需要重新设计 `As[T]` 函数的实现。构造 itab 的方法太复杂且依赖 Go 内部实现。让我采取一个更实用的方案：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
我需要重新设计 `As[T]` 函数。构造 itab 的方法太复杂且不稳定。让我采取一个更实用的方案：使用结构体字段 + 泛型函数的组合，同时保持 Instance() 的向后兼容性。



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我重新实现 `As[T]` 函数，使用一个更实用的方案：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我删除不再使用的 `methodLookup` 类型，然后运行测试：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          

        
            
toolName: CompactFake
            
status: success
          
          
让我先查看当前的代码状态，了解现有实现，然后运行测试看看具体的错误。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
现在让我运行测试看看当前的状态：

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v 2>&1 undefined
          
有编译错误：`sync` 包导入了但没有使用。让我检查 types.go 看看 sync 的使用情况：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\types.go
          
好的，`sync` 在 `types.go` 中使用，但在 `mockgen.go` 中没有使用。让我先修复这个编译错误，然后运行测试看看 `As[T]` 的具体问题：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我运行测试看看 `As[T]` 的具体问题：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestAs" 2>&1 undefined
          
让我先运行不涉及 `As[T]` 的测试，看看基线状态：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestMockProxy_Instance$|TestMockProxy_Instance_DirectMethodCall|TestMockProxy_Instance_TypeAssertion|TestAs_GenericMock_MismatchedType" 2>&1 undefined
          
现在我看到了问题。让我先修复 `TestAs_GenericMock_MismatchedType` 测试中的问题，然后重新设计 `As[T]` 函数。让我创建一个完整的修复计划：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
首先修复 `TestAs_GenericMock_MismatchedType` 测试中的问题：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          
现在让我更新任务并开始重新设计 `As[T]` 函数：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我深入研究 Go 接口的底层结构，重新设计 `As[T]` 函数。核心问题是需要手动构造接口值（iface）和方法表（itab）。让我创建一个完整的实现：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
等等，我意识到一个问题。在 `createTrampoline` 中，`methodIndex` 被闭包捕获了，但是当通过 itab.fun 调用时，闭包上下文不会被传递。让我先运行测试看看当前的状态，然后修复这个问题：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestAs" 2>&1 undefined
          
我看到了问题——`makeFuncStub` 收到的第一个参数是 `data` 而不是 `ctxt`，导致它从错误的位置读取类型信息。让我重新设计，采用一个更务实且类型安全的方案：使用注册机制 + 代码生成风格的实现。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我在测试文件中添加 `TestService` 的 mock 实现和注册：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          
现在让我运行测试看看状态：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestAs" 2>&1 undefined
          
需要修复导入问题：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我再次运行测试：















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -run "TestAs" 2>&1 undefined
          
太好了！`As[T]` 测试通过了！现在让我更新任务并运行所有测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我更新测试，添加更清晰的文档说明：


















        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
          
测试已经是正确的，它期望类型断言失败。现在让我更新任务并运行所有测试：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我运行所有测试确保全部通过：























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v 2>&1 undefined
          
所有测试都通过了！让我更新任务状态：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有问题都已修复，**67 个测试全部通过**！让我总结一下修复内容：

## 修复总结

### 问题 2：清理未使用的代码 ✅
删除了 `mockgen.go` 中 4 处未引用的类型和函数：
- `MethodCaller` 接口
- `mockInterfaceWrapper` 结构体
- `GetMockProxy` 方法
- `zeroValue` 函数

### 问题 1 & 3：接口实现与测试 ✅

**核心技术限制**：Go 静态类型系统不支持运行时给类型添加方法，因此 `instance.(MyInterface)` 类型断言无法真正实现。`runtime.getitab()` 会检查具体类型的方法集，而动态创建的结构体只有函数字段，没有方法。

**解决方案**：采用 **注册机制 + 类型安全实现** 的方案：

1. **新增 `RegisterMock[T]` API**：
   ```go
   // 注册接口的 mock 实现
   RegisterMock[TestService](func(mp *MockProxy) TestService {
       return &testServiceMock{mp: mp}
   })
   ```

2. **改进 `As[T]` 泛型函数**：
   - 从注册表中查找工厂函数
   - 返回真正实现了接口 T 的类型实例
   - 完全类型安全，不使用 `unsafe` 包，避免未定义行为

3. **保留 `Instance()` 方法**：
   - 返回动态创建的结构体指针
   - 用户可以通过字段名直接调用方法（如 `instance.Greet("World")`）
   - 文档化说明类型断言的限制

### API 使用示例

```go
// 1. 创建 mock
mock := MustCreateMock((*TestService)(nil))
mock.On("Greet", "World").Return("Hello, World!")

// 2. 方式一：通过 As[T] 获取接口实例（推荐）
svc := As[TestService](mock)
result := svc.Greet("World") // 返回 "Hello, World!"

// 3. 方式二：通过 Instance() 获取动态结构体，直接字段调用
instance := mock.Instance()
instanceVal := reflect.ValueOf(instance).Elem()
greetFn := instanceVal.FieldByName("Greet").Interface().(func(string) string)
result := greetFn("World") // 返回 "Hello, World!"
```

### 测试结果
```
PASS
ok      solocoder-go/internal/mockgen   0.287s
```
**67 个测试全部通过**，包括并发安全、错误处理、参数匹配、调用次数断言等所有功能。