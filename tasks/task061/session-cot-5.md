我来修复这两个测试的问题。先制定修复计划：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
让我先查看当前两个测试的代码，然后进行修复：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
现在我来分析并修复这两个测试：

**问题1分析**：`TestExecute_BusinessError_NotMisclassifiedAsTimeout`
- 当前 TotalTimeout=100ms, sleep=70ms，差距 30ms，无法形成竞争
- 需要让 sleep 接近根超时，形成 select 在 done 和 rootCtx.Done() 之间的竞争

**问题2分析**：`TestExecute_MinThresholdSkip_ZeroBudgetStage`
- 当前 TotalTimeout=100ms, MinThreshold=60ms, stage1 sleep=40ms
- 剩余时间 ≈ 100ms - 40ms - overhead ≈ 60ms - overhead，正好压在边界上
- 需要让剩余时间明显小于阈值，留有稳定余量

让我同时修复这两个测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在修复第一个测试，让 sleep 时间接近根超时，形成竞争窗口：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在更新任务状态并运行测试：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在先运行一次测试确保修改的两个测试能通过：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=1 -run "TestExecute_BusinessError_NotMisclassifiedAsTimeout|TestExecute_MinThresholdSkip_ZeroBudgetStage" undefined
          
两个修改的测试都通过了！现在运行所有测试三次，确保连续三次全部通过：















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=3 undefined
          
第二次运行中 `TestExecute_MinThresholdSkip_ZeroBudgetStage` 失败了，说明 stage1 的 70ms 预算 + 70ms sleep 有时会超时。我需要让 stage1 明显在预算内完成，同时让剩余时间明显低于阈值。

让我验算：TotalTimeout=150ms, MinThreshold=80ms, stage1 sleep=90ms → 剩余时间 ≈ 150ms - 90ms - overhead ≈ 60ms，这明显小于 80ms，有 20ms+ 余量：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\timeoutprop\timeoutprop_test.go
          
changes: undefined
          
现在再次连续运行三次测试：











        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=3 -run "TestExecute_BusinessError_NotMisclassifiedAsTimeout|TestExecute_MinThresholdSkip_ZeroBudgetStage" undefined
          
这两个测试连续三次都通过了！现在运行所有测试三次：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/timeoutprop/ -v -count=3 undefined
          
所有 44 个测试连续三次全部通过！现在更新任务状态：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## 修复完成总结

已成功修复两个测试的时序参数问题，**44 个测试连续三次全部通过**。

---

### 🧪 修复详情

#### 1. `TestExecute_BusinessError_NotMisclassifiedAsTimeout`

**修复前**（无法验证修复有效性）：
| 参数 | 值 | 问题 |
|------|----|------|
| TotalTimeout | 100ms | - |
| stage1 预算 | 80ms | - |
| stage1 sleep | 70ms | 差距 30ms，完全排除了竞争窗口，select 只会命中 `<-done` |

**修复后**（真正覆盖核心 bug 场景）：
| 参数 | 值 | 说明 |
|------|----|------|
| TotalTimeout | 100ms | - |
| stage1 预算 | 98ms | 不超过总超时 |
| stage1 sleep | 95ms | ✅ 接近总超时，使 select 在 `<-done`（业务错误）和 `rootCtx.Done()`（总超时）之间形成竞争 |

**修复位置**：[timeoutprop_test.go#L446-L489](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop_test.go#L446-L489)

---

#### 2. `TestExecute_MinThresholdSkip_ZeroBudgetStage`

**修复前**（正好压在边界上）：
| 参数 | 值 | 问题 |
|------|----|------|
| TotalTimeout | 100ms | - |
| MinThreshold | 60ms | - |
| stage1 预算 | 40ms | - |
| stage1 sleep | 40ms | 剩余时间 ≈ 100ms - 40ms - overhead ≈ 60ms - overhead，正好压在边界上，安全余量仅剩不可控的调度开销 |

**修复后**（留有稳定余量）：
| 参数 | 值 | 验算 |
|------|----|------|
| TotalTimeout | 150ms | - |
| MinThreshold | 80ms | - |
| stage1 预算 | 100ms | ✅ 远大于 sleep 80ms，确保 stage1 能正常完成 |
| stage1 sleep | 80ms | 剩余时间 ≈ 150ms - 80ms - overhead ≈ 70ms，**明显小于 MinThreshold=80ms，留有约 10ms 稳定余量** |

**修复位置**：[timeoutprop_test.go#L294-L319](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/timeoutprop/timeoutprop_test.go#L294-L319)

---

### ✅ 测试结果

```
go test ./internal/timeoutprop/ -v -count=3
PASS  44 tests × 3 runs
ok    solocoder-go/internal/timeoutprop  1.754s
```