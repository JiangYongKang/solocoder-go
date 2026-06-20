我来分析这两个问题并逐一修复。先查看相关代码确认当前状态。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
先仔细分析并实现**问题1的修复**：将周期性 ticker 改为 DataLoader 侧自动批处理窗口，确保同一层级的 Load 请求被尽量合并。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在重构 DataLoader，增加自动批处理窗口机制。核心思路：**DataLoader 自己在第一个 Load 到来时启动批处理窗口 timer，窗口结束自动 Flush，不再依赖执行器的周期性 ticker**。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          
changes: undefined
          
现在更新 DataLoader 结构体定义，增加新字段：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\types.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\types.go
          
changes: undefined
          
现在更新 executor.go，移除周期性 ticker，简化为**启动 goroutine → wg.Wait() → 收尾 Flush**，因为 DataLoader 自己现在有自动批处理窗口了。








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\executor.go
          
changes: undefined
          
现在修复**问题 2**：在 Validator 验证阶段对查询中的变量类型做 Kind 规范化，与 schema.go 中类型分类行为保持一致。









        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\validator.go
          
changes: undefined
          
两个问题都已修复！现在运行测试验证：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -v -count=1 2>&1 undefined
          
测试失败了！问题是 TestDataLoader_Clear 测试期望手动控制 Flush 时机，但现在自动批处理窗口 timer 会在 200μs 后自动 Flush，导致 pending 在检查之前就被清空了。让我查看这个测试的具体代码。










        
            
toolName: search_by_regex
            
status: success
          
query: TestDataLoader_Clear[^A]
          
测试失败是因为自动批处理窗口 timer 自动触发了 Flush。需要提供一个方式让手动控制 Flush 的测试能够禁用自动批处理窗口。添加 `NewDataLoaderWithWindow` 构造函数，测试中设置为 0（禁用）。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\dataloader.go
          
changes: undefined
          
现在修改 Clear 和 ClearAll 测试，使用禁用自动窗口的 DataLoader：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\gqlparser_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\gqlparser_test.go
          
changes: undefined
          
现在再检查其他 DataLoader 相关测试是否也有同样问题：



        
            
toolName: search_by_regex
            
status: success
          
query: TestDataLoader_BatchLoading|TestDataLoader_Load\b|TestDataLoader_LoadMany|TestDataLoader_EmptyFlush
          
让我详细查看 TestDataLoader_Load 和 TestDataLoader_BatchLoading 的完整代码：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\gqlparser_test.go
          
继续查看 LoadMany 测试完整代码：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gqlparser\gqlparser_test.go
          
TestDataLoader_Load、BatchLoading、LoadMany 这三个测试虽然是手动调用 Flush，但由于自动窗口 200μs 触发后结果已在 channel 里，最终检查依然能通过。只有 Clear 测试严格依赖 pending 在检查前不为空。让我运行测试验证：








        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/gqlparser/ -v -count=1 2>&1 undefined
          
所有 **62 个测试全部通过**！



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有问题修复完成，**62 个测试全部通过**！

---

## 修复总结

### 1. 批处理机制重构：DataLoader 侧自动批处理窗口

**问题**：`runConcurrentlyWithFlush` 使用 500μs 周期性 ticker 触发 Flush，同一层级的 Load 请求可能跨越 tick 边界被分割为多次小批量。

**修复方案**：将批处理决策从执行器侧**转移到 DataLoader 侧**，实现基于窗口的自动合并。

#### DataLoader 变更（[dataloader.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/dataloader.go)）：

- **新增字段**：`batchWindow`（窗口时长）、`batchTimer`（窗口定时器）
- **`Load` / `LoadMany`**：首个请求到来时，用 `time.AfterFunc(batchWindow, Flush)` 启动定时器，窗口内所有后续请求均加入同一批
- **`Flush`**：执行前先 `batchTimer.Stop()` 取消定时器，避免重复触发
- **`Clear` / `ClearAll`**：如果 pending 被清空，同步停止定时器
- **新增 API**：`NewDataLoaderWithWindow(fn, window)` 允许自定义窗口，传 `0` 禁用自动批（供手动控制 Flush 的测试使用）

#### Executor 变更（[executor.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/executor.go)）：

- 移除 `dataloaderFlushInterval` 和 `time/Ticker` 导入
- `runConcurrentlyWithFlush` 简化为 **启动 goroutine → wg.Wait() → 收尾 Flush**，不再插入周期性 tick

**效果**：同一层级在 200μs 窗口内的所有 Load 请求被自动合并为一个批次，批次合并由数据源本身的到达节奏驱动，不会被外部 tick 强行切割。

---

### 2. 自定义标量类型分类一致化

**问题**：`query_parser.go` 仅识别 5 个内置标量，SDL 中定义的自定义标量（如 `DateTime`）仍被标记为 `TypeKindObject`，与 `schema.go` 通过 `schema.types` 查找真实 Kind 的行为不一致。

**修复方案**：在 Validator 验证阶段增加**类型 Kind 规范化**步骤，覆盖 query_parser 初始分类。

#### Validator 变更（[validator.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gqlparser/validator.go#L67-L96)）：

新增 `normalizeVariableType(schema, type)` 递归函数：
- `NonNull` / `List` 包装类型 → 递归处理 `OfType`
- 命名类型 → 通过 `schema.GetType(name)` 查真实类型，用其 `Kind` 重建 Type
- `validateOperation` 构建 `variableTypes` 映射时，对每个变量类型先调用规范化

**效果**：查询中 `$d: DateTime!` 这种自定义标量变量，在验证阶段与 schema.go 一样得到正确的 `TypeKindScalar`，两处 parseTypeReference 的行为从"结果使用的视角"完全一致。