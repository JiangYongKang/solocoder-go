好的，我来修复这些问题。让我先分析问题并制定修复计划。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先查看当前的代码，然后进行修复。




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
首先修复 Snapshot 原子性问题。我需要引入注册器级别的全局读写锁，所有指标写入操作持有读锁，快照操作持有写锁。

让我先修改 counter.go，增加对全局锁的支持：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
现在我来设计原子快照的实现方案。核心思路是使用注册器级别的全局读写锁：
- 所有指标写入操作（Inc/Add/Observe等）获取读锁
- 快照操作获取写锁，确保快照期间所有写入被阻塞

让我依次修改各个文件。首先修改 counter.go：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
changes: undefined
          
现在修改 gauge.go：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
changes: undefined
          
现在修改 histogram.go：









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\histogram.go
          
changes: undefined
          
现在修改 summary.go，同时实现蓄水池采样算法来代替简单轮换：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\summary.go
          
changes: undefined
          
现在修改 registry.go，添加全局 snapshot 锁并更新 Snapshot 方法：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
现在修改 registry.go：










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在修复测试断言问题，并添加更多针对蓄水池采样的测试：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
现在修改测试文件。首先修复 TestSnapshot_Atomic 测试，将 t.Logf 改为 t.Errorf，并设置合理的阈值（修复后差异应该不超过 1）。同时添加蓄水池采样的相关测试：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
让我找到 TestSnapshot_Atomic 测试并进行修改，同时添加蓄水池采样测试：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
现在修改 TestSnapshot_Atomic 测试并添加蓄水池采样测试：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
现在添加蓄水池采样的测试用例。让我在测试文件末尾添加：























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
现在在测试文件末尾添加蓄水池采样的测试：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
现在运行测试来验证修复效果：

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
有测试直接调用了内部构造函数，需要修复。让我找到这些测试：
























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
修复测试中的内部构造函数调用：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
再次运行测试：



























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 47 个测试都通过了！现在更新 docs/metrics.md 文档：
























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
现在更新文档，添加快照原子性保证策略和 Summary 采样算法的说明：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
现在更新 Summary 部分，添加蓄水池采样算法的说明：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
现在更新并发安全部分的文档：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
现在更新测试覆盖部分：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
让我再次运行测试确认一切正常：


























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 47 个测试都通过了！让我完成最后的任务：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有修复工作已完成！以下是修复内容总结：

## 修复内容

### 1. 快照原子性保证
**问题**：原 Snapshot 方法仅对注册器本身加读锁，不阻止各指标的写入操作，导致跨指标的快照值不是同一时刻的瞬时状态。

**解决方案**：引入全局 `snapshotMu` 读写锁（参考 [registry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/registry.go#L11)）
- 所有指标写入操作（Inc/Add/Set/Observe 等）在修改值之前先获取 `snapshotMu` 的读锁
- 快照操作在遍历采集之前先获取 `snapshotMu` 的写锁
- 写锁与所有读锁互斥，确保快照期间所有写入被阻塞，所有指标值反映同一时刻的瞬时状态

涉及修改的文件：
- [counter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/counter.go)
- [gauge.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/gauge.go)
- [histogram.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/histogram.go)
- [summary.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/summary.go)

### 2. Summary 采样算法改进
**问题**：原实现使用简单循环轮换策略，仅保留最近约 1000 条数据，长期运行后丢失整体分布代表性。

**解决方案**：实现蓄水池采样算法（Algorithm R）（参考 [summary.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/summary.go#L65-L72)）
- 前 k 个观测值直接存入蓄水池（默认 k=1024）
- 第 n 个观测值（n > k）以 k/n 的概率随机替换蓄水池中的一个元素
- 每个样本被选中的概率均等，保证无偏采样和整体分布代表性
- 观测总数 Count 和总和 Sum 始终精确累计，不受采样影响

### 3. 测试断言修复
**问题**：`TestSnapshot_Atomic` 测试使用 `t.Logf` 输出提示而非 `t.Errorf`，导致检测到非原子性行为时测试仍报告 PASS。

**修复**：将 `t.Logf` 改为 `t.Errorf`，设置差异阈值为 1（参考 [metrics_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/metrics_test.go#L808-L810)）。

同时新增 5 个蓄水池采样相关测试：
- `TestSummary_ReservoirSampling_SmallSample` - 小样本量验证
- `TestSummary_ReservoirSampling_LargeSample` - 大样本量验证
- `TestSummary_ReservoirSampling_UniformDistribution` - 均匀分布验证
- `TestSummary_ConcurrentReservoirSampling` - 并发采样验证

### 4. 文档更新
在 [docs/metrics.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/metrics.md) 中增加了：
- 快照原子性保证策略的详细说明
- Summary 蓄水池采样算法的原理、优势和容量配置约定
- 三层锁机制的并发安全说明

### 测试结果
所有 **47 个测试用例全部通过**：
```
PASS
ok   solocoder-go/internal/metrics   0.202s
```