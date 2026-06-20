toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先读取所有需要修改的文件：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有文件已读取完毕。现在开始逐一修复问题。

**修复 1：GetTemplate 并发竞态条件**

当前的问题：在 `GetTemplate` 中，释放读锁后、获取写锁前，另一个 goroutine 可能已经通过 `RegisterTemplate` 更新了模板源和缓存，但当前 goroutine 仍然用旧的 source 进行解析并写入缓存，覆盖了新注册的模板。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
**修复 2：callFunction 双返回值类型断言 panic**
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
**修复 3：处理未使用的错误变量和 elseIfPattern**
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
需要做三件事：
1. 实现 `else if` 语法（使用 `elseIfPattern`）
2. 让 `ErrInvalidRange` 在 range 语法无效时返回
3. 让 `ErrBlockNotFound` 和 `ErrInvalidBlockSyntax` 在适当场景返回

首先实现 `else if` 支持。当前 `parseIfBlock` 把 `{{ else if condition }}` 当作 `{{ else }}` 处理，丢失了条件信息。需要重构 `parseIfBlock` 来支持 else if 链。

我需要修改 `IfNode` 以支持 else if 链，或者把 else if 转换为嵌套的 if-else 结构。后者更简单——把 `{{ else if condition }}` 当作 `{{ else }}{{ if condition }}...{{ endif }}` 来处理。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
现在修改 `parseIfBlock` 来支持 `else if` 语法。核心思路：当遇到 `{{ else if condition }}` 时，把剩余内容当作嵌套 if 处理，把解析出的嵌套 if 节点放入 falseNodes。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在让 `ErrInvalidRange`、`ErrInvalidBlockSyntax`、`ErrBlockNotFound` 在适当的地方被返回。让我找到它们应该在的位置：

1. **ErrInvalidRange** - 当 range 表达式语法无效时（不匹配 rangePattern 或 rangeSimplePattern 但以 `range` 开头）
2. **ErrInvalidBlockSyntax** - 当 block 名字无效时
3. **ErrBlockNotFound** - 当渲染继承模板时子模板引用了父模板中不存在的块
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
我需要添加对无效 range 语法和无效 block 语法的检测。当 tagContent 以 `range` 开头但不匹配任何合法 rangePattern 时，返回 `ErrInvalidRange`。当 tagContent 以 `block` 开头但不匹配 blockPattern 时，返回 `ErrInvalidBlockSyntax`。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在添加 `ErrBlockNotFound` 的使用。在模板继承渲染时，如果子模板试图重写一个父模板中不存在的块，应该返回 `ErrBlockNotFound`。让我找到渲染继承的逻辑：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
在模板继承中，子模板定义的块在父模板中不存在时，应该返回 `ErrBlockNotFound`。我需要在块合并时检查：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在让我先编译检查一下：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go build ./internal/templater/ undefined
          
编译成功！现在添加测试用例。让我先运行一下现有测试确保没有回归：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v 2>&1 undefined
          
所有现有测试都通过了。现在添加新的测试用例覆盖缺失的错误路径：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          
changes: undefined
          
现在运行测试看看结果：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v 2>&1 undefined
          
有三个问题需要修复：

1. **TestTemplateInheritanceLoop** - 循环继承没有检测到。问题在于 `RenderWithVisited` 先调用 `GetTemplate`，再检查 `visited`，但 `GetTemplate` 对 loopA 解析后，loopA extends loopB，接着 `GetTemplate("loopB")` 成功，然后 loopB extends loopA，但此时 `GetTemplate("loopA")` 从缓存直接返回，不会再经过 `RenderWithVisited`。需要在模板继承渲染时也传入 visited。

2. **TestRangeNilNotIterable** - 当 `.Value` 为 nil 时，`resolveVariable` 返回了 `ErrVariableNotFound` 而不是让 `toSlice` 返回 `ErrRangeNotIterable`。这是因为 `getField` 对 nil map 找不到 key 时返回 nil，而 `resolveVariable` 检测到 nil 就返回 `ErrVariableNotFound`。需要在 range 渲染中改进错误映射。

3. **TestFunctionNonErrorSecondReturn** - `IsNil()` 对 bool 类型会 panic。需要先检查 kind。

让我修复这些问题：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
问题 1 的修复：在继承模板获取父模板时也需要检查 visited：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
问题 2 的修复：在 range 渲染时，如果 resolveVariable 返回 ErrVariableNotFound，而变量值就是 nil，应该改为返回 ErrRangeNotIterable。让我看看当前 range 渲染的代码：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
问题是当 `.Value` 为 nil 时，`resolveVariable` 的遍历路径是：`getField(data, "Value")` 在 map 中找到 nil 值，返回 nil，然后 `resolveVariable` 检测到 `current == nil` 返回 `ErrVariableNotFound`。

实际上，如果 key 存在但值是 nil，应该传递 nil 给 `toSlice`，让它返回 `ErrRangeNotIterable`。问题出在 `resolveVariable` 的逻辑中——当 `getField` 返回 nil 时，无法区分"字段不存在"和"字段值为 nil"。

让我修改 `resolveVariable` 的逻辑，通过检查 map 或 struct 中是否真的存在该字段来区分：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
我需要修改 `resolveVariable` 和 `getField` 来区分"字段不存在"和"字段值为 nil"。使用一个 sentinel 值或修改返回签名：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
问题 3 的修复：`callFunction` 中 `IsNil()` 对非 nilable 类型（如 bool）会 panic。需要先检查类型是否可以调用 `IsNil()`：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在重新运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v 2>&1 undefined
          
还有两个失败：

1. **TestTemplateInheritanceLoop** - 循环继承仍然没有检测到。我需要调试看看。问题可能是 `GetTemplate` 使用缓存，所以循环模板不会在渲染阶段被检测到。我需要在 `GetTemplate` 中也进行循环检测，或者换一种方式来检测。

2. **TestInvalidVariablePath** - `{{ . }}` 没有返回 `ErrInvalidVariablePath`。问题在于 varPattern 是 `\{\{\s*\.([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}`，而 `{{ . }}` 中的 `.` 后面没有字母，所以不匹配 varPattern。它被 funcPattern 匹配为函数调用 `.`，而 `.` 不是关键字所以被当作函数名处理。我需要换一种方式触发 `ErrInvalidVariablePath`。

让我先修复这两个问题：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater_test.go
          
changes: undefined
          
等等，`{{ .Name. }}` 实际上会匹配 varPattern，因为路径是 `Name.`。但 `resolveVariable("Name.")` 中 `strings.Split("Name.", ".")` 会得到 `["Name", ""]`，然后尝试 `getField` 访问空字段名，返回 `fieldNotFound`。所以会返回 `ErrVariableNotFound` 而非 `ErrInvalidVariablePath`。

我需要在 `resolveVariable` 中增加对空路径部分的检测：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在处理循环继承的问题。问题是 `RenderWithVisited` 调用 `GetTemplate(name)` 时，visited 检查在此之前已经标记了当前 name，但 `GetTemplate` 只是返回缓存/解析模板，不会递归。真正的递归发生在获取父模板时。让我调试一下：

当渲染 "loopA" 时：
1. `visited["loopA"] = true`
2. `GetTemplate("loopA")` - 成功，loopA extends "loopB"
3. 检查 `visited["loopB"]` - false，然后 `visited["loopB"] = true`
4. `GetTemplate("loopB")` - 成功，loopB extends "loopA"
5. 但渲染 loopB 的 extends 时需要再次调用 `RenderWithVisited`... 

等等，当前的实现不是递归调用 `RenderWithVisited` 的。当前代码是在 `RenderWithVisited` 中直接处理继承，不是通过递归。所以当处理 loopB 作为父模板时，代码没有再检查 loopB 是否 extends loopA。

我需要在获取父模板后也检查父模板是否有 extends，并验证不会形成循环。让我重新看一下继承渲染的代码：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
问题清楚了！当 loopA extends loopB 时：
1. `visited["loopA"] = true`
2. `visited["loopB"]` 检查是 false
3. `visited["loopB"] = true`
4. 但 parentTmpl (loopB) 自身也有 `Extends: loopA`！而当前代码只处理了一级继承，没有递归处理父模板的继承。

当父模板也 extends 另一个模板时，当前代码不会处理这种情况。但更重要的是，对于循环继承，我需要检查父模板是否也 extends 了当前模板（或更长的循环链）。

最好的修复方式是：在继承链中递归地检查每个父模板的 extends，同时用 visited 集合检测循环：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\templater\templater.go
          
changes: undefined
          
现在再运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v 2>&1 undefined
          
所有 65 个测试全部通过！现在修复文档中的问题：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复 docs/templater.md 中的问题。需要修改：
1. 移除 `{{ $i + 1 }}` 算术表达式（引擎不支持）
2. 更新条件渲染部分说明 `else if` 支持
3. 确保文档与引擎实际行为一致
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
首先更新条件渲染部分，添加 else if 说明：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
changes: undefined
          
现在修复完整示例中的 `{{ $i + 1 }}` 问题：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
修复 `{{ $i + 1 }}` 为引擎支持的语法。引擎不支持算术表达式，需要注册一个自定义函数来实现：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
changes: undefined
          
现在还需要更新文档中模板继承部分，提到子模板不能重写父模板中不存在的块（会返回 `ErrBlockNotFound`）：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
changes: undefined
          
现在更新自定义函数部分，说明双返回值必须是 `(result, error)` 类型：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\templater.md
          
changes: undefined
          
最后让我再运行一次测试确保一切正常：
        
            
toolName: run_command
            
status: success
          
command: cd "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go" ; go test ./internal/templater/ -v -count=1 2>&1 undefined
          
所有 **65 个测试全部通过** ✅
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有 6 个问题已全部修复完成。以下是修复总结：

## 修复内容

### 1. GetTemplate 并发竞态条件
- **问题**：`GetTemplate` 在释放读锁后、获取写锁前，旧版本模板解析结果可能覆盖新注册的模板缓存
- **修复**：采用 double-checked locking 模式，获取写锁后再次检查缓存是否存在，确保不会用旧解析结果覆盖新缓存
- **文件**：[templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L51-L79)

### 2. callFunction 双返回值类型断言 panic
- **问题**：`results[1].Interface().(error)` 直接类型断言，当第二个返回值不是 error 类型时会 panic；`IsNil()` 对非 nilable 类型（如 bool）也会 panic
- **修复**：先检查 `Kind()` 是否为 `Interface` 或 `Ptr`，再调用 `IsNil()`；使用 comma-ok 模式安全地进行类型断言，非 error 类型返回 `ErrInvalidFunctionCall`
- **文件**：[templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L972-L984)

### 3. 未使用的错误变量和 elseIfPattern
- **问题**：`ErrBlockNotFound`、`ErrInvalidRange`、`ErrInvalidBlockSyntax` 从未被返回；`elseIfPattern` 已声明但未使用，`else if` 语法无法解析
- **修复**：
  - 实现 `else if` 语法支持，在 `parseIfBlock` 中新增 tagType=3 处理 `{{ else if condition }}` 分支
  - 在 `parseContent` 中添加对无效 range 语法的检测，返回 `ErrInvalidRange`
  - 在 `parseContent` 中添加对无效 block 语法的检测，返回 `ErrInvalidBlockSyntax`
  - 在模板继承渲染中，子模板重写父模板不存在的块时返回 `ErrBlockNotFound`
- **文件**：[templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L234-L277), [templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L116-L121), [templater.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater.go#L470-L508)

### 4. 补充缺失的关键错误路径测试用例
新增 19 个测试用例覆盖：
- `TestTemplateInheritanceLoop` - 循环继承返回 `ErrTemplateInheritanceLoop`
- `TestRangeNotIterable` / `TestRangeNilNotIterable` / `TestRangeStringNotIterable` - 非切片类型返回 `ErrRangeNotIterable`
- `TestUnclosedBlockTag` / `TestUnclosedIfTag` / `TestUnclosedRangeTag` / `TestUnclosedVariableTag` - 未闭合标签返回 `ErrUnclosedBlock`
- `TestInvalidRangeSyntax` - 无效 range 语法返回 `ErrInvalidRange`
- `TestInvalidBlockSyntax` - 无效 block 语法返回 `ErrInvalidBlockSyntax`
- `TestBlockNotFoundInInheritance` - 子模板重写不存在的块返回 `ErrBlockNotFound`
- `TestElseIfSyntax` / `TestElseIfChained` - else if 多条件分支
- `TestFunctionNonErrorSecondReturn` - 非 error 双返回值返回 `ErrInvalidFunctionCall`
- `TestFunctionNonFuncRegistration` - 注册非函数值返回 `ErrInvalidFunctionCall`
- `TestInvalidCondition` / `TestInvalidVariablePath` / `TestRegisterFunctionEmptyName` / `TestFunctionArgumentCountMismatch` / `TestGetTemplateRaceCondition`
- **文件**：[templater_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/templater/templater_test.go#L900-L1157)

### 5. 修正 docs/templater.md 文档
- 将 `{{ $i + 1 }}` 替换为 `{{ add $i 1 }}`（注册 `add` 自定义函数实现加法）
- 条件渲染部分新增 `else if` 多条件分支说明和示例
- 模板继承部分新增子模板重写不存在块时返回 `ErrBlockNotFound` 的说明
- 自定义函数部分新增双返回值必须为 `(result, error)` 类型的说明
- **文件**：[templater.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/templater.md)

### 6. 其他改进
- 修改 `getField` 使用 sentinel 值 `fieldNotFound` 区分"字段不存在"和"字段值为 nil"，确保 range 目标为 nil 值时正确返回 `ErrRangeNotIterable`
- 修改 `resolveVariable` 增加对空路径部分的检测，返回 `ErrInvalidVariablePath`
- 在 `RenderWithVisited` 中增加对继承链的循环检测，递归检查父模板的 extends 链