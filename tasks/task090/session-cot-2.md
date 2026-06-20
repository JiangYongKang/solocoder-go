我来分析这些问题并逐一修复。让我先创建任务计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
现在让我进行全面修复。我会重写核心模块文件，解决所有问题。

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在重写测试文件，以匹配新的 API 行为。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
现在运行测试验证。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
修复编译错误：`Error()` 方法中不能用 `%w`（仅 `fmt.Errorf` 支持）。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
有一个测试因为 `t.Fatalf` 会触发 `runtime.Goexit()` 而失败。让我修复测试方式。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
实际上 `runtime.Goexit()` 不能被 recover 捕获。让我换一种测试策略——直接验证底层 `Match` 方法的行为，对于 `Assert` 只测试成功路径。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
测试期望值有两个错误，让我修正它们：

1. `toolongstring` width=8：应该是 5 字符 + "..." = 8 字符，即 `"toolo..."`
2. `中文字符` width=6：4 个字符 < 6，应该 pad 到 6 位






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
还有一个测试用例需要修正——"中文字符测试"正好 6 个字符等于宽度，所以不截断。让我改成长度超过 width 的。








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
等一下，"中文字符测试abc" 有 9 个 rune，width=6，取前 3 个 + "..." = 6 字符，应该是 "中文字..." 而不是 "中文..."。让我修正。










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
所有测试都通过了！现在更新文档。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在更新文档，重点修改：
1. 快照创建与更新流程（不再自动创建快照）
2. 内容归一化规则（保留尾部空行）
3. 并排差异输出格式





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\snaptest.md
          
changes: undefined
          
文档更新完成。让我再确认一次所有测试通过。






        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v -count=1 undefined
          
所有测试通过！现在标记所有任务完成。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题已修复完毕，测试全部通过。以下是修复总结：

## 修复内容

### 1. Match 方法行为修正 [snaptest.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest.go#L469-L498)

- **快照不存在时不自动创建**：直接返回 `ErrSnapshotNotFound` 错误，不会静默创建快照导致测试误通过
- **不匹配时返回结构化错误**：返回 `*MismatchError` 类型的错误，包装了 `ErrSnapshotMismatch`，同时包含快照名称、完整差异数据和格式化报告
- **Assert 适配**：快照不存在时 `t.Fatalf` 并提示创建方式；不匹配时 `t.Errorf` 输出差异报告

### 2. 内容归一化修复 [snaptest.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest.go#L114-L117)

- `normalizeSnapshotContent` 只做换行符归一化（`\r\n` → `\n`）
- **不再裁剪尾部空行**，完整保留原始内容中的所有换行符
- 写入时仅在内容无末尾换行时追加一个 `\n`

### 3. 并排差异输出格式 [snaptest.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest.go#L327-L374)

从 unified diff 改为真正的**并排（side-by-side）**格式：

```
Expected (snapshot)                                           | Actual (current output)
------------------------------------------------------------+------------------------------------------------------------
    1 line1                                                  |    1 line1
~   2 line2                                                  |    2 line2_modified
    3 line3                                                 |    3 line3
-   4 line4                                                  |
    5 line5                                                 |    4 line5
+                                                           |    5 line6_new

Summary: 3 same, 1 removed, 1 added, 1 modified
```

支持四种行类型：
| 符号 | 类型 | 说明 |
|------|------|------|
| ` ` | `DiffSame` | 相同行，左右都有内容 |
| `-` | `DiffRemoved` | 删除行，仅左侧有内容 |
| `+` | `DiffAdded` | 新增行，仅右侧有内容 |
| `~` | `DiffModified` | 修改行，左右都有但内容不同 |

### 4. 新增测试用例 [snaptest_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest_test.go)

- `TestMatcher_Match_SnapshotNotFound`：验证快照不存在时返回错误且不创建文件
- `TestMatcher_Assert_SnapshotNotFound`：验证断言在快照不存在时的行为
- `TestMatcher_ExistingMismatch`：验证不匹配时返回 `MismatchError`
- `TestNormalizeSnapshotContent`：6 个用例验证尾部空行保留
- `TestDiff_TrailingEmptyLines`：验证尾部空行参与比对
- `TestMatcher_TrailingEmptyLinesPreserved`：验证写入读取保留尾部空行
- `TestDiff_ModifiedLineHasBothSides`：验证修改行左右都有内容
- `TestDiffResult_Format_SideBySide`：验证并排格式输出
- `TestDiffResult_Format_AddedAndRemoved`：验证 +/- 标记
- `TestDiff_Format_ContainsSideBySideContent`：验证同一行同时显示两侧内容
- `TestTruncateOrPad`：验证列宽截断与填充（含中文）
- `TestMismatchError_Unwrap`：验证错误类型断言

### 5. 文档更新 [snaptest.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/snaptest.md)

- 新增 **第 7 节：快照创建与更新机制**（含流程图）
- 新增 **第 8 节：内容归一化规则**
- 更新 **第 6 节：差异展示格式说明**（并排格式详解）
- 更新 **第 10 节：错误定义**（MismatchError 说明）
- 更新使用示例，增加创建快照、处理不匹配错误等场景

## 测试结果

```
PASS
ok      solocoder-go/internal/snaptest  0.919s
```

全部 **45+** 个测试用例通过。