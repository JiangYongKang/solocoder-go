我来修复这两个问题。先查看 `SetTotal` 方法的当前实现：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination.go
          
确认了两个问题：
1. 第 213 行 `if meta.CurrentPage > 1` 缺少 `&& meta.TotalPages > 0`
2. 第 220 行 `if meta.TotalPages > 0` 只在大于 0 时才更新 LastPage，导致 TotalPages=0 时 LastPage 保留旧值

现在修复：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination.go
          
changes: undefined
          
代码修复完成。现在补充相关单元测试，覆盖这两个边界场景：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination_test.go
          
现在在 `TestSetTotalShrinkToZero` 之后补充针对这两个问题的专门测试：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination_test.go
          
changes: undefined
          
现在运行测试验证修复：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/pagination/ -v -count=1 undefined
          
测试发现两个我预期设置错误的用例。分析一下：
- `page=5, totalAfter=5, size=10` → `TotalPages=1`，此时 `page > TotalPages`，`HasPrevPage` 应该是什么？
- `page=2, totalAfter=10, size=10` → `TotalPages=1`，同样 `page > TotalPages`

当 `CurrentPage > TotalPages`（页码超范围）时，虽然 `CurrentPage > 1`，但从逻辑上讲"不存在有效上一页"。让我修正测试期望值，并与 BuildOffsetResponse 行为对齐：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/pagination/ -v -count=1 2>&1 | Select-Object -Last 50 undefined
          
我理解了当前代码的逻辑。`HasPrevPage` 只检查 `currentPage > 1 && totalPages > 0`，并不关心页码是否超范围（超范围已由 Data 清空处理）。让我修正测试用例以匹配实际设计逻辑：




















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\pagination\pagination_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/pagination/ -v -count=1 2>&1 | Select-Object -Last 30 undefined
          
全部测试通过！现在更新任务状态：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
---

## 修复完成总结

### 修复的问题

#### 1. SetTotal HasPrevPage 判断逻辑与 BuildOffsetResponse 不一致

**修复位置**：[pagination.go#L213](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pagination/pagination.go#L213)

**问题描述**：`SetTotal` 方法的偏移量分页分支中 `HasPrevPage` 只检查了 `meta.CurrentPage > 1`，缺少 `&& meta.TotalPages > 0`。调用 `SetTotal(0)` 将总条数缩减到零后，`TotalPages` 变为 0 但 `page > 1` 的请求仍会得到 `HasPrevPage` 为 `true`。

**修复方案**：
```go
// 修复前
if meta.CurrentPage > 1 {
// 修复后
if meta.CurrentPage > 1 && meta.TotalPages > 0 {
```

---

#### 2. SetTotal TotalPages 归零时 Nav.LastPage 未同步更新

**修复位置**：[pagination.go#L220](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/pagination/pagination.go#L220)

**问题描述**：`SetTotal` 方法在更新 `Nav.LastPage 时用 `if meta.TotalPages > 0 { nav.LastPage = meta.TotalPages }`，当 `TotalPages=0` 时不更新，导致 `LastPage` 保留旧值。

**修复方案**：
```go
// 修复前
if meta.TotalPages > 0 {
    nav.LastPage = meta.TotalPages
}
// 修复后
nav.LastPage = meta.TotalPages
```

---

### 新增测试用例

| 测试函数 | 子用例数 | 覆盖场景 |
|---------|---------|----------|
| `TestSetTotalShrinkToZeroHasPrevPage | 4 | page=1/2/5/10 缩减 total 到 0 时 HasPrevPage=false |
| `TestSetTotalShrinkToZeroNavLastPage` | 1 | SetTotal(0) 后 LastPage=0 |
| `TestSetTotalShrinkZeroNavLastPageConsistency` | 1 | LastPage 始终等于 TotalPages（多次 SetTotal） |
| `TestSetTotalOffsetHasPrevPageLogic` | 14 | 表驱动测试 HasPrevPage 在各种缩减场景下的逻辑 |

### 测试结果

```
PASS
ok      solocoder-go/internal/pagination        0.218s
```

**共 67 个测试函数（含子用例）全部通过
**