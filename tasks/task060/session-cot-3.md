我来帮你继续修复这三个问题。让我先制定修复方案：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
现在让我看一下 CalculateErrorRate 和 countRecentSuccesses：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
好的，我已经分析清楚了所有问题。现在开始逐一修复：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先修复问题 1：删除 `matchTriggerCondition` 中的 `TriggerConditionErrorRate` 死代码分支并添加注释：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
现在修复问题 2：提取公共方法统一错误率计算逻辑，消除重复代码：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
现在修改 `CalculateErrorRate` 方法，复用公共方法消除重复代码：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
现在修复问题 3：`countRecentSuccesses` 无窗口配置时也基于 `SuccessWindow` 近期数据：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback.go
          
changes: undefined
          
现在先编译检查代码是否正确：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/fallback/ undefined
          
编译成功！现在运行测试：




















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 undefined
          
所有测试都通过了！让我再添加几个测试用例来专门覆盖这次修复的内容，确保代码质量：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
现在添加针对本次修复的测试用例：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在运行完整测试：






















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 undefined
          
有一个测试用例的断言有问题。因为执行很快，`time.Nanosecond` 窗口实际上还是会包含所有事件。让我修复这个测试：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fallback\fallback_test.go
          
changes: undefined
          
现在再次运行测试：
























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test -v ./internal/fallback/ -timeout 60s 2>&1 undefined
          
所有测试都通过了！🎉 让我更新任务状态：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## ✅ 第三轮修复完成总结

本次修复针对用户指出的三个残留问题，均已全部解决，共 **41 个测试用例**全部通过 ✓。

---

### 问题 1：`matchTriggerCondition` 中 `TriggerConditionErrorRate` 死代码

**位置**：[fallback.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go#L303-L319) 中的 `matchTriggerCondition` 方法

**修复内容**：
- 删除了 `TriggerConditionErrorRate` 分支中的 `return false` 死代码
- `TriggerConditionErrorRate` 类型不再进入 `matchTriggerCondition` 的 switch 判断
- 错误率检查由独立的 `shouldSkipByErrorRate` 方法在 `Execute` 循环中处理（该方法需要 strategy 上下文，不适合基于 error+condition 的 matchTriggerCondition 接口）
- 新增测试 `TestMatchTriggerConditionCoverage` 覆盖所有现存分支：`ErrorType`、`Timeout`、`Custom`

---

### 问题 2：错误率计算逻辑重复，语义不一致

**位置**：`calculateStrategyErrorRate` 和 `CalculateErrorRate` 两个方法

**修复内容**：
- 提取公共方法 `calculateErrorRateLocked`（[fallback.go#L267-L285](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go#L267-L285)），统一处理窗口模式和非窗口模式的语义
- 提取公共方法 `countEventsInWindowLocked`（[fallback.go#L287-L301](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go#L287-L301)），消除窗口遍历和事件计数的重复代码
- `CalculateErrorRate` 从原来的 40+ 行精简为 10 行，直接调用 `calculateErrorRateLocked`
- 新增测试 `TestUnifiedErrorRateCalculation` 和 `TestCountEventsInWindow` 验证一致性

**重构后的错误率计算调用链**：
```
shouldSkipByErrorRate        CalculateErrorRate (public)
       │                            │
       ▼                            ▼
calculateStrategyErrorRate    GetStrategy + RLock
       │                            │
       └──────────────┬─────────────┘
                      ▼
            calculateErrorRateLocked  ← 统一逻辑
                      │
                      ▼
            countEventsInWindowLocked  ← 窗口计数
```

---

### 问题 3：`countRecentSuccesses` 无窗口配置时直接返回全量历史计数器

**位置**：[fallback.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fallback/fallback.go#L505-L523) 中的 `countRecentSuccesses` 方法

**修复内容**：
- `PassiveSuccessWindow <= 0` 时，从 `return int(strategy.SuccessCount)` 改为 `return len(strategy.SuccessWindow)`
- 无窗口配置时也基于独立时间窗口的近期成功事件（`SuccessWindow` 会被 cleanup 限制在 1000 条以内），而非暴露全量历史累计值
- 新增测试 `TestCountRecentSuccessesWithAndWithoutWindow` 验证两种模式

---

### 新增测试用例（4 个）

| 测试名称 | 覆盖内容 |
|---------|---------|
| `TestCountRecentSuccessesWithAndWithoutWindow` | 有无窗口配置时 SuccessWindow 的正确使用、被动恢复触发 |
| `TestUnifiedErrorRateCalculation` | `window=0` 和 `window=1h` 结果一致性、窗口清理和恢复一致性 |
| `TestMatchTriggerConditionCoverage` | `matchTriggerCondition` 所有分支（9 个子用例）：nil、匹配/不匹配、wrapped error、timeout、custom 等 |
| `TestCountEventsInWindow` | `countEventsInWindowLocked` 正确计数、与 `CalculateErrorRate` 结果一致性 |

---

### 测试结果

```
PASS: 41 tests in 2.178s
- 37 个原有测试全部通过
- 4 个新增测试全部通过
```