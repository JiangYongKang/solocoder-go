我先查看项目结构，了解现有代码风格和约定，然后制定实现计划。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\lsm\level.go
          
让我查看现有的 tsdb 代码和其他模块的代码风格，以便更好地遵循项目约定。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\engine.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
我看到项目中已经有 `internal/tsdb/` 包。让我先运行测试，查看当前状态，然后修复问题并完善文档。


        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v 2>&1 | head -100 undefined
          
让我直接运行测试：


        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v undefined
          
现在我了解了情况。测试文件缺少 `math` 包的导入。让我创建任务列表来规划工作：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我修复缺失的 `math` 包导入：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在运行测试确认所有测试通过：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v undefined
          
有一个测试失败了 `TestDownsample_MultipleWindowsWithAvg`。让我分析并修复这个测试：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
我看到问题了。时间戳 2100 和 3500 都落在同一个窗口（2000-4000）里，所以只有 2 个窗口而不是 3 个。让我修复测试数据：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
同时更新测试中 bucket2 的计算：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
再次运行测试确认所有测试通过：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v undefined
          
所有测试通过！现在更新 todo 列表并开始编写文档：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先查看其他文档的风格，保持一致：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
现在创建 docs/tsdb.md 需求文档：
















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
进行最终验证，再次运行测试确保所有功能正常：



















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v -count=1 undefined
          
所有测试通过！让我更新 todo 列表并给出最终总结。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
时序数据库引擎模块已完整实现并通过所有测试。以下是完成的工作总结：

## 完成的工作

### 1. 代码实现
项目中已存在完整的时序数据库引擎实现，位于 [internal/tsdb/engine.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go)，包含：

| 功能 | 核心方法 | 位置 |
|------|---------|------|
| 按时间戳写入 | `Write(points []*DataPoint)` | [engine.go#L103-L159](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go#L103-L159) |
| 降采样聚合 | `Downsample(start, end, windowSize, agg, tags)` | [engine.go#L270-L358](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go#L270-L358) |
| TTL 自动过期 | `cleanupLoop()`, `cleanupExpired()` | [engine.go#L360-L424](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go#L360-L424) |
| 多维度标签索引 | `filterByTags(tags)`, `rebuildTagIndex()` | [engine.go#L215-L268](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go#L215-L268) |

### 2. 问题修复
在 [internal/tsdb/tsdb_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/tsdb_test.go) 中修复了两个问题：

- **第 5 行**：添加了缺失的 `"math"` 包导入，用于 `TestDownsample_AvgPrecision` 测试中的浮点数精度比较
- **第 1692 行**：修复 `TestDownsample_MultipleWindowsWithAvg` 测试数据，将时间戳 `3500` 改为 `4500`，确保数据点分布在 3 个独立的 2 秒窗口中
- **第 1707 行**：同步更新对应的 bucket 计算引用

### 3. 测试结果
运行 `go test ./internal/tsdb/ -v`，**全部 78 个测试通过**，测试覆盖了：
- ✅ 正常流程：写入、查询、降采样、TTL 清理
- ✅ 边界条件：空数据、单数据点、相同时间戳、负数时间戳、精确边界匹配
- ✅ 异常分支：无效参数、引擎已关闭、nil 数据点、空标签
- ✅ 并发场景：并发写入、并发查询、并发读写混合

### 4. 需求文档
创建了完整的 [docs/tsdb.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tsdb.md) 需求文档，包含：

1. **模块概述**：功能定位和包路径
2. **核心功能**：7 大功能特性说明
3. **核心结构体职责**：`TSEngine`、`DataPoint`、`AggregatedPoint`、`Config`、`AggregatorType`
4. **数据结构设计**：数据存储结构、标签倒排索引结构
5. **完整生命周期流程**：
   - 数据写入流程
   - 数据查询流程
   - 降采样聚合流程
   - TTL 自动过期清理流程（后台循环 + 清理执行）
   - 引擎关闭流程
6. **降采样窗口对齐算法**：原理和示例
7. **并发安全设计**：锁分层策略、使用规则、死锁避免
8. **使用示例**：6 个完整代码示例（基本使用、降采样、TTL 配置、禁用 TTL、多标签查询、并发使用）
9. **错误定义**：9 种错误变量的含义和触发场景
10. **性能特征**：时间复杂度、空间复杂度、优化点
11. **注意事项与限制**：10 项使用注意
12. **设计权衡**：5 项关键设计决策的优缺点

## 核心设计亮点

1. **数据完整性保护**：写入时复制 Tags map，查询时返回数据副本，避免外部修改影响内部状态

2. **多锁分层设计**：使用 3 把读写锁（`closedMu`、`dataMu`、`tagIndexMu`）减少锁竞争，提高并发度

3. **批次清理策略**：每次清理限制数量，避免长时间持有写锁，平衡清理效率和写入可用性

4. **完整的生命周期管理**：后台 goroutine 通过 `stopCh` + `WaitGroup` 实现优雅关闭，无资源泄漏

5. **丰富的聚合算子**：内置 5 种聚合算子（Avg、Max、Min、Sum、Count），窗口与时间戳自动对齐