我来逐一修复这些问题。首先让我规划任务。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我先读取当前的代码文件：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
现在开始修复。首先添加新的错误变量以区分不同场景：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复 `Parser.AddOption` 方法返回正确的错误：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
现在修复 `Parser.AddOption` 方法：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          
现在修复 `Parser.AddCommand` 方法：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          
现在修复 `Parser.AddPositionalArg`、`Command.AddOption`、`Command.AddPositionalArg`：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
现在修复 `Parser.AddPositionalArg`：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          
现在修复 `Command.AddOption` 和 `Command.AddPositionalArg`：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
现在修复 `Execute` 方法、`Command.AddOption` 和 `Command.AddPositionalArg`：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复 `parseLongOption` 处理布尔空值的问题。`--flag=` 应该与 `--flag` 行为一致，视为 `true`：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在更新单元测试以匹配新的错误语义。先读取测试文件中需要修改的部分：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
现在更新测试文件中的多个测试用例。先修改 `TestAddOption_Nil`、`TestAddOption_NoName`：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          
现在修改 `TestAddCommand_Nil`、`TestAddCommand_EmptyName`、`TestAddCommand_Duplicate`：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestAddCommand_Nil|func TestAddCommand_EmptyName|func TestAddCommand_Duplicate|func TestAddPositionalArg_Nil|func TestCommand_AddOption_Nil|func TestCommand_AddOption_NoName|func TestCommand_AddPositionalArg_Nil|func TestExecute_NoHandler|func TestExecute_NoCommand|func TestErrorStrings
          
现在逐一修复这些测试函数：





















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
现在逐一修复这些测试：























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          
现在修改其他测试函数：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          
现在修改 Command 相关的测试：



























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          
现在添加缺失的测试用例 `TestCommand_AddOption_NoName` 和 `TestParse_BoolOption_EmptyEquals`，并更新 `TestErrorStrings`：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
现在更新 `TestErrorStrings` 并添加新的测试用例：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cliparser\cliparser_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在运行测试验证所有修复：



























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cliparser/ -v undefined
          
所有 **81 个测试** 全部通过 ✅。现在更新需求文档：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在更新 docs/cliparser.md 文档，修正错误处理部分和添加新错误的说明：





























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          

        
            
toolName: search_by_regex
            
status: success
          
query: ## 5. 错误处理
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
现在更新错误处理部分和相关内容：































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
changes: undefined
          
现在更新设计要点部分，添加布尔选项空值处理的说明，并更新测试覆盖数量：





































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
现在更新测试覆盖数量，并在设计要点中添加布尔空值处理的说明：


































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
changes: undefined
          
现在更新测试覆盖部分，添加新增的测试点：


































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
changes: undefined
          
现在更新基础结构测试的描述：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
changes: undefined
          
最后再运行一次测试确认所有修复完成：







































        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cliparser/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有 5 个问题已经全部修复完成，**81 个测试全部通过 ✅**。以下是修复详情：

---

## 修复总结

### 问题 1：错误语义不区分 → 新增 7 个细分错误变量

**文件**：[cliparser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L10-L29)

新增错误变量：
| 新错误 | 说明 |
|--------|------|
| `ErrNilOption` | `AddOption(nil)` - 传入的 Option 指针为 nil |
| `ErrNilCommand` | `AddCommand(nil)` - 传入的 Command 指针为 nil |
| `ErrNilArg` | `AddPositionalArg(nil)` - 传入的 PositionalArg 指针为 nil |
| `ErrOptionNoName` | Option 同时缺少长名和短名 |
| `ErrCommandNoName` | 注册子命令时名称为空 |
| `ErrDuplicateCommand` | 重复注册相同名称的子命令 |
| `ErrNoHandler` | Execute 时子命令已匹配但 Handler 未设置 |

修复的方法：
- `Parser.AddOption()` [L86-L95](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L86-L95) - nil → `ErrNilOption`，无名称 → `ErrOptionNoName`
- `Parser.AddCommand()` [L115-L124](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L115-L124) - nil → `ErrNilCommand`，空名 → `ErrCommandNoName`，重复 → `ErrDuplicateCommand`
- `Parser.AddPositionalArg()` [L139-L148](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L139-L148) - nil → `ErrNilArg`
- `Command.AddOption()`、`Command.AddPositionalArg()` 同步修复

---

### 问题 2：Execute 错误不区分 → 分情况返回

**文件**：[cliparser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L464-L472)

```go
func (p *Parser) Execute() error {
    if p.parsedCmd == nil {
        return ErrCommandNotFound    // 无匹配子命令
    }
    if p.parsedCmd.Handler == nil {
        return ErrNoHandler          // 有匹配但无处理器
    }
    return p.parsedCmd.Handler()
}
```

调用方现在可以通过 `errors.Is(err, ErrCommandNotFound)` vs `errors.Is(err, ErrNoHandler)` 区分两种情况。

---

### 问题 3：布尔空值 `--flag=` 报错 → 与 `--flag` 行为一致

**文件**：[cliparser.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L285-L300)

```go
if opt.Type == BoolType {
    if hasValue {
        if value == "" {
            setOptionValue(opt, true)  // --flag= 视为 true
        } else {
            b, err := strconv.ParseBool(value)
            ...
        }
    } else {
        setOptionValue(opt, true)      // --flag 视为 true
    }
}
```

现在 `--flag=`、`--flag` 都设为 `true`，`--flag=false` 设为 `false`，行为一致。

---

### 问题 4：错误语义不匹配 → 使用专用错误

已在问题 1 和 2 中一并修复：
- "长名和短名均为空" → `ErrOptionNoName`（原为 `ErrUnknownOption`）
- "命令名重复注册" → `ErrDuplicateCommand`（原为 `ErrDuplicateOption`）

---

### 问题 5：测试验证

**文件**：[cliparser_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser_test.go)

- 更新 11 个现有测试用例以匹配新错误语义
- 新增 4 个测试用例：`TestCommand_AddOption_NoName`、`TestParse_BoolOption_EmptyEquals`、`TestParse_BoolOption_TrueEquals`、`TestParse_BoolOption_FalseEquals`
- 测试总数从 76 → **81 个**，全部通过 ✅

---

### 问题 6：文档更新

**文件**：[cliparser.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cliparser.md)

- 错误处理表格更新：从 11 个错误 → **18 个错误**，补充每个错误的精确使用场景
- 新增 `6.6 布尔选项等号赋值规则` 小节，详细说明 `--flag=`、`--flag=true`、`--flag=false` 的行为
- 测试覆盖更新：76 个用例 → **81 个用例**，补充新增测试点的说明

运行测试验证：
```bash
go test ./internal/cliparser/ -v
# PASS: 81 个测试用例全部通过
```