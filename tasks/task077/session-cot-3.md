让我先查看当前代码状态，然后规划修复方案。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
**修复 1**：新增 `ErrFlagTypeMismatch` 错误变量，统一所有类型不匹配场景的错误包装，与"类型参数非法"语义区分开。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag.go
          
changes: undefined
          
现在将所有四个便捷方法的类型不匹配错误统一为 `ErrFlagTypeMismatch`：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag.go
          
changes: undefined
          
现在更新测试中对 `ErrInvalidFlagType` 的引用，改为 `ErrFlagTypeMismatch`：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我用 Grep 查找所有需要修改的断言位置。




        
            
toolName: search_by_regex
            
status: success
          
query: ErrInvalidFlagType
          
其中第 141 行是 `Validate` 方法中"类型参数本身非法"的场景，应保持 `ErrInvalidFlagType`。其他行都是类型不匹配，需要改为 `ErrFlagTypeMismatch`。






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
现在一次性替换所有类型不匹配测试用例的断言（保留第141行 Validate 中的 ErrInvalidFlagType）：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          
现在替换 TestTypeValidation_AllSetterMethods 中的所有断言：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
**修复 2**：重构 `TestQueryAuditLogs_ByTimeRange` — 从实际审计日志条目中提取时间戳作为锚点，完全不依赖 `time.Sleep`，避免 Windows 时钟精度问题。








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
**修复 3**：重构 `TestQueryAuditLogs_TimeRangeWithFlagKey`，移除 `time.Sleep`，并补充 `FlagKey+StartTime+EndTime` 三参数组合场景。








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在运行测试验证所有修复。





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/featureflag/ -v undefined
          
问题出在 Windows 低精度时钟下，多条日志的时间戳完全相同，导致 midpoint 策略失效。让我重新设计这两个测试，直接用时间戳的 ±1ns 偏移来验证过滤逻辑，而不依赖时间戳间的实际差异。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          
现在修复 `TestQueryAuditLogs_TimeRangeWithFlagKey` 中的 StartTime 过滤断言问题：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\featureflag\featureflag_test.go
          
changes: undefined
          
现在再次运行测试。




        
            
toolName: run_command
            
status: success
          
command: go test ./internal/featureflag/ -v undefined
          
所有测试均通过。让我更新 todo 列表以标记完成。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
三个问题已全部修复，`go test ./internal/featureflag/ -v` 全通过。以下是各问题的修复总结：

---

## 修复 1：类型错误语义区分

**核心代码**位于 [featureflag.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/featureflag/featureflag.go)：

- 新增 sentinel error：`ErrFlagTypeMismatch = errors.New("feature flag type mismatch")`
- 新增自定义错误结构体 `flagTypeMismatchError`，持有 `actual` / `expected` 两个 `FlagType`
- 实现 `Error()` 输出格式：`"feature flag type mismatch: flag type is <Actual>, not <Expected>"`
- 实现 `Is(target error) bool` 接口，使 `errors.Is(err, ErrFlagTypeMismatch)` 正确识别
- 新增构造函数 `newFlagTypeMismatchError(actual, expected FlagType) error`
- `SetBooleanValue` / `SetPercentage` / `AddToWhitelist` / `RemoveFromWhitelist` 四个便捷方法的类型校验全部统一使用 `newFlagTypeMismatchError`
- `FlagConfig.Validate()` 中的非法枚举值场景仍然使用 `ErrInvalidFlagType`（这是正确的语义）

对应测试断言（`TestSetBooleanValue` 等 35 个子用例）全部更新为 `errors.Is(err, ErrFlagTypeMismatch)`。

---

## 修复 2：`TestQueryAuditLogs_ByTimeRange` 时钟精度不稳定

原问题：Windows 下 `time.Sleep(20ms)` + 锚点捕获不可靠，且 `ts0==ts1==ts2` 全量相等时 midpoint 分区全部失败。

修复策略：
1. **完全移除 `time.Sleep`**，从已写入的审计日志实际 `Timestamp` 提取 ts0/ts1/ts2
2. 用 **宽时间锚点**（`ts0 - 1h` / `ts2 + 1h`）做全包含 / 全排除断言
3. 用 **±1ns 偏移** 验证闭区间边界（`ts0 - 1ns` 单点查询应返回 0，`[ts0, ts2]` 精确边界应包含全部）
4. **条件化断言**：只有当 `distinctCount == 3`（三个时间戳实际不同）时才做 `ts0+1ns` / `ts2-1ns` 的严格内外分区断言，避免假失败
5. 日志 `ts0==ts1==ts2 (distinct=1)` 时自动跳过需要时间差的分支

---

## 修复 3：补全 FlagKey+StartTime+EndTime 三参数场景

`TestQueryAuditLogs_TimeRangeWithFlagKey` 原只测双参数，现重写为 6 个子用例：

1. `FlagKey + StartTime only (far past)` — 验证从远过去起应包含该 flag 所有日志
2. `FlagKey + EndTime only (far future)` — 验证到远未来止应包含该 flag 所有日志  
3. `FlagKey + StartTime only (far future)` — 验证未来时间点返回 0
4. **`FlagKey + StartTime + EndTime three-parameter combination (exact bounds)`** — docs 第 9.3 节要求的核心场景：`[aCreateTs, aSetTs]` 精确闭区间查询，验证 (a) 只含 flag-a (b) 所有 Timestamp 在范围内 (c) 不含 flag-b 任何日志
5. **`three-param query (wide range) filters FlagKey but not time`** — 用覆盖所有日志的宽时间范围 + 单独 FlagKey 查询，验证 FlagKey 过滤优先
6. `three-param query (exclusive range boundaries)` — 可选：当 aCreateTs ≠ aSetTs 时用 `aCreateTs+1ns` 作为 StartTime 验证 CREATE 日志被排除

同样完全移除 `time.Sleep`，所有锚点基于实际 `Timestamp` 的加减偏移构造。