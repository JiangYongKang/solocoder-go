我来分析并修复这个代码重复问题。首先让我查看这两个函数的完整实现。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\validator\validator.go
          
现在让我查看 `buildCondition` 的实现：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\validator\validator.go
          
分析完成。两个函数的解析规则完全一致：
1. **取反模式** `!xxx`：去掉 `!` 前缀后 TrimSpace
2. **等值模式** `xxx=yyy`：`=` 左侧 TrimSpace 为字段名，右侧 TrimSpace 为预期值  
3. **简单模式** `xxx`：整体 TrimSpace

现在我来抽取共用的解析函数：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在我来添加共用的条件表达式解析函数和枚举类型，放在 `extractFieldNameFromCondition` 之前：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\validator\validator.go
          
changes: undefined
          
现在让我重构 `buildCondition` 函数，使用共用的 `parseCondition` 函数：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func buildCondition
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\validator\validator.go
          
现在让我重构 `buildCondition` 函数：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\validator\validator.go
          
changes: undefined
          
重构完成。现在让我运行所有测试确保回归通过：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/validator/ -v -count=1 2>&1 | Select-Object -Last 60 undefined
          
所有测试都通过了！让我再运行一下完整的测试列表确认全部通过：


















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/validator/ -v -count=1 2>&1 | Select-String "^(---|=== RUN|PASS|FAIL|ok)" | Measure-Object -Line undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/validator/ -v -count=1 2>&1 | Select-String "FAIL" undefined
          
Select-String "FAIL" 匹配到了 PASS 中的 FAIL 子串。让我用更精确的方式确认没有失败的测试：





















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/validator/ -count=1 2>&1 undefined
          
全部测试通过！让我更新任务状态：























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务已完成！以下是修复内容总结：

---

## 重构内容：消除条件表达式解析的代码重复

### 问题分析

`extractFieldNameFromCondition` 和 `buildCondition` 两个函数各自独立实现了完全相同的三种条件表达式解析模式：

| 模式 | 语法 | 两处重复的解析逻辑 |
|------|------|---------------------|
| 取反 | `!xxx` | `strings.TrimSpace(strings.TrimPrefix(expr, "!"))` |
| 等值 | `xxx=yyy` | `strings.TrimSpace(expr[:eqIdx])` + `strings.TrimSpace(expr[eqIdx+1:])` |
| 简单 | `xxx` | `strings.TrimSpace(expr)` |

这与上一轮修复的 `dereferenceValue`/`isEmptyValue` 重复解引用属于同一反模式。

### 解决方案

抽取共用的解析函数，两个消费者都依赖它。

---

### 1. 新增共用类型和解析函数

在 [validator.go 第316-349行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/validator/validator.go#L316-L349) 新增：

```go
// 条件模式枚举
type conditionMode int
const (
    conditionSimple conditionMode = iota  // 简单模式：FieldName
    conditionNegate                        // 取反模式：!FieldName
    conditionEquals                        // 等值模式：FieldName=value
)

// 解析结果结构体
type parsedCondition struct {
    mode      conditionMode  // 解析出的模式
    fieldName string         // 解析出的字段名（已 TrimSpace）
    expected  string         // 等值模式下的预期值（已 TrimSpace）
}

// 共用解析函数（唯一一处维护解析规则）
func parseCondition(expr string) parsedCondition {
    if strings.HasPrefix(expr, "!") {
        return parsedCondition{
            mode:      conditionNegate,
            fieldName: strings.TrimSpace(strings.TrimPrefix(expr, "!")),
        }
    }
    eqIdx := strings.Index(expr, "=")
    if eqIdx != -1 {
        return parsedCondition{
            mode:      conditionEquals,
            fieldName: strings.TrimSpace(expr[:eqIdx]),
            expected:  strings.TrimSpace(expr[eqIdx+1:]),
        }
    }
    return parsedCondition{
        mode:      conditionSimple,
        fieldName: strings.TrimSpace(expr),
    }
}
```

### 2. 重构 `extractFieldNameFromCondition`

[validator.go 第351-353行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/validator/validator.go#L351-L353)：

```go
func extractFieldNameFromCondition(expr string) string {
    return parseCondition(expr).fieldName  // 一行搞定
}
```

### 3. 重构 `buildCondition`

[validator.go 第490-528行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/validator/validator.go#L490-L528)：在闭包外预先解析，内部通过 `switch` 分派，消除所有字符串操作：

```go
func buildCondition(expr string) ConditionFunc {
    parsed := parseCondition(expr)  // 闭包外一次性解析
    return func(s interface{}) bool {
        // ... struct 准备逻辑 ...
        
        switch parsed.mode {
        case conditionNegate:
            fv, ok := findFieldByName(val, parsed.fieldName)
            ...
            return isEmptyValue(fv)
        case conditionEquals:
            fv, ok := findFieldByName(val, parsed.fieldName)
            ...
            return fmt.Sprintf("%v", fv.Interface()) == parsed.expected
        default:
            fv, ok := findFieldByName(val, parsed.fieldName)
            ...
            return !isEmptyValue(fv)
        }
    }
}
```

---

### 收益

1. **消除重复**：解析规则现在只在 `parseCondition` 一处维护，将来新增条件语法（如 `>=`、`contains` 等）只需修改一个函数
2. **性能优化**：`buildCondition` 将解析从闭包内部移到外部，同一个条件被多次调用时解析只执行一次
3. **可读性提升**：`switch` 分派比多个 if 嵌套更清晰，`parsedCondition` 结构体命名明确了各字段的语义

---

所有测试全部通过：
```
ok  solocoder-go/internal/validator  0.242s
```