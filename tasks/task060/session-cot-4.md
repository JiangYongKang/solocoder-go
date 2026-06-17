我来帮你修复这两个问题。让我先制定修复计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
现在开始修复。首先修复问题 1：添加注释说明错误率检查的处理位置：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
现在修复问题 2：增强测试用例，增加有窗口模式的测试，并将 `t.Logf` 改为断言：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
现在重写这个测试函数，增加有窗口模式测试和真正的断言：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在运行测试验证：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v -run TestCountRecentSuccessesWithAndWithoutWindow ./internal/fallback/ -timeout 30s 2>&1 undefined
          
有两个测试失败。让我分析原因：

1. `without_window`：`initiateRecovery` 是在 goroutine 中异步执行的，测试时还没来得及切换
2. `with_window_filtering`：时间窗口边界计算问题，`entry.Time.After(cutoff)` 不包含等于 cutoff 的情况

让我修复测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
我注意到测试中有个问题：在持有读锁的情况下调用了 `countRecentSuccesses`，虽然 Go 的 RWMutex 支持嵌套读锁，但不够清晰。让我修复这个问题并运行完整测试：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
同样修复 `with_window_filtering` 子测试：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在运行完整测试：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 undefined
          
所有测试都通过了！🎉 让我更新任务状态：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## ✅ 第四轮修复完成总结

本次修复针对用户指出的两个残留问题，均已全部解决，共 **41 个测试用例**（含子用例共 49 个）全部通过 ✓。

---

### 问题 1：`matchTriggerCondition` 缺少错误率检查的注释说明

**位置**：[fallback.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go#L303-L330) 中的 `matchTriggerCondition` 方法

**修复内容**：
- 在 switch 语句前添加了清晰的注释，明确说明：
  - `TriggerConditionErrorRate` 类型不在此处处理
  - 原因是错误率触发需要基于策略的历史统计数据
  - 实际由 `shouldSkipByErrorRate` 方法在 `Execute` 循环的策略执行前进行检查
- 后续维护者看到 `TriggerConditionErrorRate` 常量存在时，能通过注释快速找到处理位置

```go
// NOTE: TriggerConditionErrorRate 类型不在此处处理。
// 错误率触发条件需要基于策略的历史统计数据进行判断，
// 由 shouldSkipByErrorRate 方法在 Execute 循环的策略执行前进行检查。
```

---

### 问题 2：`TestCountRecentSuccessesWithAndWithoutWindow` 测试覆盖不足、断言缺失

**位置**：[fallback_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback_test.go#L1402-L1618) 中的测试

**修复内容**：
- 将测试重写为 **4 个子测试**（通过 `t.Run`），结构更清晰：

| 子测试名称 | 验证内容 |
|-----------|---------|
| `without_window` | 无窗口配置时，`countRecentSuccesses` 返回 `SuccessWindow` 的长度（而非全量历史计数器），并验证数量正确 |
| `with_window_filtering` | 有窗口配置时，时间过滤逻辑正确（10 个条目，5 秒窗口内只有 4 个） |
| `with_window_above_threshold_triggers_recovery` | 有窗口配置且成功数超过阈值时，确实触发被动恢复（切换回主策略、状态变为 HEALTHY），使用 **真实断言** 而非 `t.Logf` |
| `with_window_below_threshold_no_recovery` | 有窗口配置但成功数低于阈值时，不触发恢复，保持在降级策略 |

**关键改进**：
- 从 **仅 1 个测试路径** 增加到 **4 个测试路径**，覆盖有无窗口、达不达标阈值等多种场景
- 将原来只记录日志的 `t.Logf` 全部替换为 **真正的断言**（`t.Errorf` + 具体期望值）
- 异步恢复场景增加了 `time.Sleep(100 * time.Millisecond)` 等待 goroutine 执行完毕后再断言

---

### 测试结果

```
PASS: 41 tests in 2.280s
- TestCountRecentSuccessesWithAndWithoutWindow: 4/4 sub-tests passed
  - without_window ✓
  - with_window_filtering ✓
  - with_window_above_threshold_triggers_recovery ✓
  - with_window_below_threshold_no_recovery ✓
```