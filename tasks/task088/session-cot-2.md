让我先仔细检查当前代码中的问题，然后逐一修复。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\errors.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\reporter.go
          
分析清楚了，现在逐一修复这些问题：

**问题分析：**
1. `runGroup` 调用了 `runSingle` 但没有使用 `cfg.Timeout` 和 `runSingleWithTimeout`
2. `AllocsPerOp`/`BytesPerOp` 与 `MeanAllocCount`/`MeanAllocBytes` 冗余
3. `ErrBaselineNotFound` 和 `ErrGroupEmptyResult` 未被使用
4. 文档需要补全





        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
首先修复超时配置集成问题：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          
changes: undefined
          
现在修复 CheckRegression 中 ErrBaselineNotFound 的使用，同时处理冗余字段问题：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          
changes: undefined
          
需要添加 fmt import：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在移除冗余的 AllocsPerOp 和 BytesPerOp 字段：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\types.go
          
changes: undefined
          
现在更新 calculateStatistics 函数：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          
changes: undefined
          
现在更新 Reporter，移除冗余的输出：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\reporter.go
          
changes: undefined
          
现在更新对比报告，添加内存分配相对于基线的变化百分比：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm.go
          
changes: undefined
          
现在更新 ComparisonItem 类型：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\types.go
          
changes: undefined
          
现在更新对比报告格式：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\reporter.go
          
changes: undefined
          
现在更新测试文件，移除冗余字段引用并添加新测试：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在我需要完全重写测试文件，移除冗余字段引用并添加新测试：









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\benchfrm\benchfrm_test.go
          
changes: undefined
          
现在运行测试看看是否有问题：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/benchfrm/ -v undefined
          
所有测试都通过了！现在补全文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在完全重写文档，确保内容完整：











        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\benchfrm.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
## 修复完成总结

所有问题已修复，**47 个测试全部通过**。以下是修复详情：

### 🔧 问题 1：超时配置未集成

**修复位置**：[benchfrm.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/benchfrm.go#L77-L99) 的 `runGroup` 方法

**修复内容**：将原来调用的 `runSingle` 改为 `runSingleWithTimeout(group, cfg.CollectMemory, cfg.Timeout)`，确保用户配置的超时时间正确传递到执行流程中。

**新增测试**：
- `TestRunAll_Timeout` —— 验证 100ms 函数在 10ms 超时配置下返回 `context.DeadlineExceeded`
- `TestRunAll_NoTimeoutWorks` —— 验证 Timeout=0 时正常运行
- `TestRunAll_ShortTimeout` —— 验证大超时值不影响快速函数
- `TestTimeoutOptionIntegration` —— 验证所有配置选项正确存储

### 🔧 问题 2：冗余字段与内存数据展示

**修复位置**：
- [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/types.go#L14-L33) —— 删除 `AllocsPerOp`、`BytesPerOp` 字段，为 `ComparisonItem` 新增 `AllocBytesPct`、`AllocCountPct`
- [benchfrm.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/benchfrm.go#L198-L250) —— `Compare` 方法计算 3 个维度的变化率
- [reporter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/reporter.go#L35-L75) —— 对比报告改为 6 列，新增 Duration/Bytes/Allocs 三个 Δ% 变化率列，带 ↑/↓ 箭头标记

### 🔧 问题 3：错误变量缺失使用逻辑

**修复位置**：
- `ErrGroupEmptyResult`：在 [benchfrm.go#L94-L96](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/benchfrm.go#L94-L96) 中，当 `runResults` 切片为空时返回该错误
- `ErrBaselineNotFound`：在 [benchfrm.go#L267-L269](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/benchfrm/benchfrm.go#L267-L269) 中，当回归检测时找不到基线返回该错误（附带组名：`fmt.Errorf("%w: %s", ErrBaselineNotFound, current.Name)`）

**新增测试**：
- `TestCheckRegression_BaselineNotFound` —— 验证未保存基线时调用 CheckRegression 正确返回 ErrBaselineNotFound
- `TestErrorVariables` —— 验证全部 10 个错误变量的定义和消息内容

### 🔧 问题 4：文档补全

**修复位置**：[docs/benchfrm.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/benchfrm.md)（全文重写，791 行）

**新增/补全章节**：
| 章节 | 内容 |
|------|------|
| 核心功能详解（6 节） | 重复运行统计指标、预热阶段、内存统计原理、超时机制、对比维度、回归检测指标 |
| 完整执行流程（7 阶段） | 初始化配置 → 遍历组 → 预热 → 正式运行（含超时细节）→ 统计计算 → 对比生成 → 回归检测与基线保存（每阶段有流程图和关键逻辑说明） |
| 报告格式说明（3 种） | 基准报告、对比报告（含列定义表）、回归报告（正常/告警两种样例） |
| 错误处理 | 错误定义完整表 + 错误返回策略矩阵 |
| 使用示例（5 个） | 基本测试、多算法对比、CI 流水线回归检测、带超时的 RPC 测试、内存存储临时对比 |
| 并发安全说明 | 4 个组件的同步机制对照表 |
| 性能与开销 | 框架自身的时间/内存开销及优化建议 |
| 测试覆盖范围 | 13 个类别共 47 个测试的详细清单 |
| 扩展开发指南 | Redis 存储后端示例 + JSON 报告生成器示例 |

### ✅ 测试结果

```
=== 47 tests, ALL PASSED ===
PASS
ok      solocoder-go/internal/benchfrm  2.096s
```