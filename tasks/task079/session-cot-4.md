好的，让我先查看当前代码状态，然后规划修复方案。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\guard.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
让我先检查一下 Go 版本，以及查看注册表中指标存储的详细情况：

        
            
toolName: run_command
            
status: success
          
command: go version undefined
          
让我也看看顺序场景下的蓄水池采样测试容差：


        
            
toolName: search_by_regex
            
status: success
          
query: UniformDistribution
          
让我看看更多的测试用例：



        
            
toolName: search_by_regex
            
status: success
          
query: LargeSample
          
好的，现在我清楚了所有问题。让我开始修复。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先修复问题 3：counter.go 和 gauge.go 写入方法使用 defer 释放锁。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
changes: undefined
          
现在修复 gauge.go：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
changes: undefined
          
问题 3 修复完成。现在修复问题 1：增强 `snapshotProtected` 接口的编译期保证。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
首先修改 guard.go，让 `snapshotProtected` 接口嵌入 `Metric` 接口：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\guard.go
          
changes: undefined
          
现在修改 registry.go，增加 `allMetrics` map 集中存储所有指标，提供统一的编译期保证：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在修改四个 `Register` 方法，将指标存入 `allMetrics`，这样可以提供编译期接口实现保证。先改 `RegisterCounter`：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在修改 `Snapshot` 方法，改用 `allMetrics` 遍历，以及修改 `Unregister` 方法从 `allMetrics` 删除：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
修改 `Snapshot` 和 `Unregister` 方法：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在修改 `Unregister` 方法，在每个分支中增加从 `allMetrics` 删除的逻辑：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在去掉四个指标文件中零散的 `var _` 检查行（因为 `allMetrics` map 已经提供了集中的编译期保证），同时保留 `snapshotGuardPtr()` 方法以满足接口要求：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\histogram.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\summary.go
          
changes: undefined
          
问题 1 修复完成。现在修复问题 2：收紧并发蓄水池采样测试的分位值容差，使其与顺序场景（`TestSummary_ReservoirSampling_LargeSample`）的标准一致：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
所有代码修改完成，现在运行测试验证：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 47 个测试全部通过！现在更新文档：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
更新文档中的并发安全部分，说明集中式编译期保证机制：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
同时更新指标内部锁的说明，强调使用 `defer` 释放锁保证 panic 安全：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
更新测试覆盖说明：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
最后再运行一次测试确认所有修改正确：












        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有修复完成，47 个测试全部通过。以下是修复内容总结：

---

## 修复内容

### 问题 1：`snapshotProtected` 接口的编译期保证不够强

**问题**：`snapshotProtected` 接口在生产代码中从未被调用，四个指标文件中零散的 `var _ snapshotProtected = (*xxx)(nil)` 检查行若被遗漏，代码依然能编译通过。

**修复**：
- 在 [guard.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/guard.go#L19-L22) 中让 `snapshotProtected` 接口嵌入 `Metric` 接口，要求同时实现 `Snapshot()` 和 `snapshotGuardPtr()` 方法
- 在 [registry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/registry.go#L17) 中新增 `allMetrics map[string]snapshotProtected` 集中存储所有已注册指标
- 四个 `Register` 方法在注册指标时同时存入 `allMetrics`，编译期自动验证指标类型是否实现了 `snapshotProtected` 接口
- `Snapshot()` 方法改为遍历 `allMetrics`，代码更简洁
- `Unregister()` 方法同步从 `allMetrics` 中删除
- 去掉了四个指标文件中零散的 `var _` 检查行，改为集中式编译期保证

### 问题 2：并发蓄水池采样测试容差太宽松

**问题**：`TestSummary_ConcurrentReservoirSampling` 中 P50 容差 ±1500（比例 30%），比顺序场景（`LargeSample` 测试，容差 ±1000，比例 20%）更宽松，无法有效验证并发竞争下的统计无偏性。

**修复**：在 [metrics_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/metrics_test.go#L1002-L1059) 中收紧断言标准，与顺序场景 `TestSummary_ReservoirSampling_LargeSample` 保持一致：
- P50：区间 [4000, 6000]（容差 ±1000，比例 20%）
- P90：区间 [8000, 10000]
- P99：区间 [9000, 10000]

### 问题 3：counter.go 和 gauge.go 写入方法未使用 defer 释放锁

**问题**：`counter.go` 和 `gauge.go` 的写入方法在 `guard.write` 闭包内获取 `mu.Lock()` 后没有用 `defer` 释放，若闭包内因异常触发 panic，锁将永久锁定。

**修复**：
- [counter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/counter.go#L42-L72) 的 `Inc()`、`Add()`、`Reset()` 方法改为 `defer c.mu.Unlock()` 模式
- [gauge.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/gauge.go#L42-L71) 的 `Set()`、`Add()`、`Sub()` 方法改为 `defer g.mu.Unlock()` 模式
- 与 `histogram.go` 和 `summary.go` 的写法保持一致，确保 panic 安全

### 测试结果

所有 **47 个测试用例全部通过**：
```
PASS
ok   solocoder-go/internal/metrics   0.223s
```