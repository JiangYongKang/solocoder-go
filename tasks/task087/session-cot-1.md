我来帮你实现一个 Mock 框架功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先查看一下项目的 go.mod 和一些现有模块的代码风格，以便遵循项目约定。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          
现在我了解了项目结构和代码风格。让我创建一个详细的任务计划来实现这个 Mock 框架模块。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
好的，让我开始实现 Mock 框架。首先创建核心类型定义文件。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在让我创建 mockgen 包的核心类型定义。




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\types.go
          
changes: undefined
          
现在让我创建主要的 mockgen.go 文件，实现核心功能。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在让我创建主要的 mockgen.go 文件，实现核心的 Mock 框架功能。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen.go
          
changes: undefined
          
现在让我创建 errors.go 文件（遵循项目中的错误定义惯例），然后编写单元测试。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
让我先测试一下代码能否编译通过，然后再编写单元测试。



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/mockgen/ undefined
          
很好，代码编译通过了。让我创建 errors.go 文件，然后编写完整的单元测试。





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\errors.go
          
changes: undefined
          
现在我需要更新 types.go，移除重复的错误定义。让我重新写一下 types.go 文件。




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\types.go
          
我需要移除 types.go 中重复的错误变量定义。让我修改这个文件。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\types.go
          
changes: undefined
          
让我再次编译一下确保代码没有问题。







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/mockgen/ undefined
          
很好，代码编译通过了。现在让我开始编写单元测试。








        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
现在让我编写完整的单元测试文件。









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          
现在让我运行测试，看看是否所有测试都通过。











        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
让我运行测试看看是否通过。













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v undefined
          
有一个测试代码错误，让我修复它。

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mockgen\mockgen_test.go
          
changes: undefined
          
让我再次运行测试。

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v undefined
          
太好了！所有测试都通过了。现在让我编写需求文档。




















        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
现在让我编写需求文档。
























        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mockgen.md
          
changes: undefined
          
现在让我更新 todo 列表，标记所有任务为完成状态。
























        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
让我再次运行测试，确认所有测试都通过。


























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v -count=1 undefined
          
让我再试一次运行测试。



























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/mockgen/ -v undefined
          
完美！所有 48 个测试用例都通过了。现在让我总结一下已完成的工作。

## Mock 框架功能模块已实现完成！

### 项目结构

**代码实现**（位于 [internal/mockgen/](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mockgen/)）：

- [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mockgen/types.go) - 核心类型定义
- [mockgen.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mockgen/mockgen.go) - 主要功能实现
- [errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mockgen/errors.go) - 错误定义
- [mockgen_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mockgen/mockgen_test.go) - 单元测试（48个测试用例）

**文档**：
- [docs/mockgen.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/mockgen.md) - 模块需求文档

### 实现的功能

1. **接口自动 Mock 实现生成** - 通过 `CreateMock()` 函数，传入接口类型即可生成 Mock 代理实例，未配置期望的方法返回零值

2. **期望调用参数匹配** - 支持三种匹配方式：
   - 精确匹配：直接传入参数值
   - 任意匹配：`Any()` 匹配任意值
   - 自定义匹配：`Matches(func(interface{}) bool)` 自定义匹配逻辑

3. **返回值与行为配置** - 两种模式：
   - 固定返回值：`.Return(values...)`
   - 动态计算：`.Run(func(...args) (...returns))` 回调函数

4. **调用次数断言** - 支持多种配置：
   - `.Times(n)` - 精确 n 次
   - `.MinTimes(n)` - 至少 n 次
   - `.MaxTimes(n)` - 最多 n 次
   - `.Once()` - 恰好 1 次
   - `.Never()` - 不允许调用

5. **未匹配调用的失败报告** - `VerifyVerbose()` 返回详细报告，包含方法名、实际参数、已注册期望列表

### 使用示例

```go
// 1. 创建 Mock
mock := mockgen.CreateMock((*UserService)(nil))

// 2. 设置期望
mock.On("GetUser", 1).Return("user1", nil).Once()

// 3. 获取方法并调用
getUserFunc := mock.Method("GetUser").(func(int) (string, error))
name, err := getUserFunc(1)

// 4. 验证
if err := mock.Verify(); err != nil {
    t.Errorf("verification failed: %v", err)
}
```

所有测试均已通过，运行 `go test ./internal/mockgen/ -v` 可看到完整的测试输出。