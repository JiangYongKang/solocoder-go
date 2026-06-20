我来帮你实现时序异常检测器功能模块。首先让我了解一下项目结构，然后制定详细的实现计划。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先了解一下现有代码的风格和项目依赖，以及一些相关模块（如 tsdb、windowagg）的实现方式。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\windowagg\windowagg.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\engine.go
          
好的，我已经了解了项目结构和代码风格。现在让我制定详细的实现计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在开始创建核心代码文件。首先实现 `internal/tsanomaly/tsanomaly.go`，包含所有核心功能：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
核心代码文件已创建。现在更新任务状态并创建单元测试。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在创建全面的单元测试文件：




        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
测试文件已创建，现在运行测试验证所有功能：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v undefined
          
有一个测试失败了。问题在于季节性模式检测逻辑：新点先被加入窗口后才检测，导致新点污染了基线，使得异常不明显。需要修改为：**先基于旧基线检测，再更新基线**。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
问题找到了：新点先被加入窗口，才基于包含新点的窗口检测，导致季节性模式下3个相同值 + 1个异常值的均值被偏移，检测不出异常。正确的逻辑应该是**先基于旧基线检测，再更新基线**。让我修复这个问题。



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
修复完成，现在重新运行测试：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v undefined
          
还有一个测试失败：严重程度级别判断有问题。让我分析一下——均值100、标准差5的情况下，125偏离了25，即5σ，超过了 `2*StdDevFactor = 4` 的阈值，所以被判定为critical。需要调整测试用例的数值。让我修复测试：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
再运行一次测试，如果还是有问题我会换用更可控的方式构造测试数据：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -run TestDetector_SeverityLevels undefined
          
SeverityLevels测试单独通过了，运行完整的测试套件：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 undefined
          
输出被截断了，让我单独明确指定包名运行：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test solocoder-go/internal/tsanomaly -v -count=1 2>&1 | Select-Object -First 100 undefined
          
所有26个测试全部通过！现在更新任务状态并编写需求文档。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在编写模块需求文档。让我先参考一下现有文档的风格：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\windowagg.md
          
好的，了解了文档风格。现在编写 tsanomaly.md 需求文档：








        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
最后再运行一次完整的测试，确保所有内容都正确：





        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test solocoder-go/internal/tsanomaly -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
全部任务已完成！以下是实现总结：

## 交付文件清单

### 1. 核心代码：[internal/tsanomaly/tsanomaly.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go)

实现的核心功能：

- **增量移动平均基线**：`windowStats` 结构体通过维护 `sum` 和 `sumSq` 两个累加变量 + `list.List` 链表，实现 O(1) 增量更新均值和样本标准差，窗口满时自动淘汰最旧数据
- **标准差偏离检测**：支持三种偏离方向（`DirectionBoth`/`DirectionUp`/`DirectionDown`），阈值通过 `StdDevFactor * σ` 计算
- **季节性周期建模**：按 `PeriodLength` 维护独立的基线数组，第 t 个点用索引 `(t-1) mod P` 的基线判断
- **异常事件体系**：`AnomalyEvent` 包含 10 个诊断字段，严重程度自动分为 `Warning` 和 `Critical` 两级，`AnomalyQuery` 支持按时间/方向/严重程度查询并带 Limit 限制
- **生命周期管理**：`UpdateConfig` 动态改配、`Reset` 清空状态、`Close` 关闭拒接、`sync.RWMutex` 并发安全

### 2. 单元测试：[internal/tsanomaly/tsanomaly_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly_test.go)

**26 个测试全部通过**（`go test solocoder-go/internal/tsanomaly -v`），覆盖：

| 类别 | 测试场景 |
|------|----------|
| 配置 | 全部错误分支（窗口大小/标准差倍数/最小样本/周期长度/方向） |
| Add 分支 | nil点、关闭检测器、预热期不检测、上偏/下偏异常 |
| 方向过滤 | DirectionUp 忽略下偏、DirectionDown 忽略上偏 |
| 增量计算 | 6 个点的窗口均值和方差精确值验证、窗口淘汰 |
| 季节性 | 4 个周期位置独立基线、季节性索引越界错误 |
| 查询 | StartTime/EndTime/Direction/Severity/Limit 各维度过滤 |
| 排序 | 乱序插入后 GetAnomalies 返回时序升序 |
| 限制 | MaxAnomalyHistory 超限淘汰 |
| BatchAdd | 空批量、含nil点、关闭检测器的错误 |
| 并发 | 10 个 Goroutine 并发 100 次 Add+读操作 |
| 其他 | 严重分级验证、零stddev边界、事件字段正确性、重置、动态配置、关闭 |

### 3. 需求文档：[docs/tsanomaly.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tsanomaly.md)

包含：模块功能概述（5大功能点）、核心结构体职责表、异常检测数学模型（移动平均/无偏方差/判断规则/严重级别/季节性公式）、错误定义、并发安全说明、以及 5 个完整的使用示例代码：

1. 基本全局基线异常检测
2. 季节性模式（24小时日周期流量监控）
3. 方向过滤（仅检测库存超卖）
4. 批量添加与多维查询
5. 实时服务监控（延迟+QPS双检测器）