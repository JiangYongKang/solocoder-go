让我先查看当前的代码确认问题，然后删除未使用的 `currentRate` 死代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: currentRate
          
确认了问题。`currentRate` 方法（第89-101行）确实没有任何调用者，所有调用点都使用的是 `currentRateLocked`。现在删除这个死代码。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tokenbucket\tokenbucket.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tokenbucket\tokenbucket.go
          
changes: undefined
          
现在运行测试确保所有测试通过。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tokenbucket/ -v -count=1 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
---

## 令牌桶限流模块修复完成 ✅

### 问题分析
`currentRate` 方法是上次修复 `IsWarmingUp` 副作用时拆分出的只读版本（不修改 `warmup` 字段），但实际情况是：

| 调用点 | 使用的方法 |
|--------|-----------|
| `refill()` | `currentRateLocked` |
| `Take()` | `currentRateLocked` |
| `CurrentRate()` | `currentRateLocked` |
| `IsWarmingUp()` | 直接用时间差判断，不调用任何方法 |

**没有任何代码调用 `currentRate`**，该方法成为死代码。

### 修复方案
删除了 [tokenbucket.go#L89-L101](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tokenbucket/tokenbucket.go#L89-L101) 中的未使用方法：

```go
// 已删除的死代码
func (b *Bucket) currentRate(now time.Time) float64 {
    if !b.warmup {
        return b.rate
    }
    elapsed := now.Sub(b.warmupStartTime)
    if elapsed >= b.warmupDuration {
        return b.rate
    }
    progress := float64(elapsed) / float64(b.warmupDuration)
    return b.warmupStartRate + (b.rate-b.warmupStartRate)*progress
}
```

保留的 `currentRateLocked` 方法供所有内部写操作调用（可在预热结束时修改 `warmup` 状态）。

### 测试结果

```
PASS
ok      solocoder-go/internal/tokenbucket       0.217s
```

**62 个测试用例全部通过**。