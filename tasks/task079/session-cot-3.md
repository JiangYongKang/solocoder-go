让我先查看当前的代码，然后制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先查看当前各文件的代码状态，确保修改基于最新版本。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\summary.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\histogram.go
          
现在我也来读取一下测试文件：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\types.go
          
好的，现在我清楚所有代码的状态了。开始实施修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
首先创建 `snapshotGuard` 结构体，封装快照保护逻辑：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\guard.go
          
changes: undefined
          
现在修改四个指标类型，嵌入 `snapshotGuard` 并使用 `guard.write()` 模式：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\counter.go
          
changes: undefined
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\gauge.go
          
changes: undefined
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\histogram.go
          
changes: undefined
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\summary.go
          
changes: undefined
          
现在修改 registry.go，使用 `snapshotGuard` 替代 `*sync.RWMutex`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\registry.go
          
changes: undefined
          
现在更新四个注册方法，使用 `r.guard` 替代 `&r.snapshotMu`：
        
            
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
          
现在修复测试文件。需要更新 `TestMetricValue_Types` 中的构造函数调用，以及增强 `TestSummary_ConcurrentReservoirSampling` 的分位值断言：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先修复 `TestMetricValue_Types` 的构造函数调用：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
现在修复 `TestSummary_ConcurrentReservoirSampling`，增加分位值准确性断言：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\metrics\metrics_test.go
          
changes: undefined
          
现在运行测试验证：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 47 个测试都通过了。现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
更新并发安全部分，说明 `snapshotGuard` 机制和 `snapshotProtected` 接口的编译时保证：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
同时更新测试覆盖说明：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\metrics.md
          
changes: undefined
          
最后再运行一次测试确认一切正常：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/metrics/ -v -count=1 undefined
          
所有 47 个测试全部通过。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复工作已完成，47 个测试全部通过。以下是修复内容总结：

---

## 修复内容

### 问题 1：并发蓄水池采样测试缺少分位值断言

**问题**：[metrics_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/metrics_test.go#L1002) 的 `TestSummary_ConcurrentReservoirSampling` 只验证了 `Count` 一致性，未对 P50/P90/P99 等分位值做任何准确性断言，无法证明蓄水池算法在并发竞争条件下的统计无偏性。

**修复**：
- 注册 Summary 时配置了 `[0.5, 0.9, 0.99]` 三个百分位（原来只有 `[0.5]`）
- 新增 Sum 精确性断言（与数学公式对比）
- 新增 P50、P90、P99 的区间断言：10 个 goroutine 各写 1000 个值（范围 0~9999 均匀分布），每个分位值与理论期望的偏差容差为 ±1500（约 15%），验证蓄水池采样在并发场景下的统计无偏性

### 问题 2：快照保护机制仅靠代码惯例维持

**问题**：`snapshotMu` 以 `*sync.RWMutex` 裸指针注入到四个指标类型中，每个写入方法需手动先调用 `snapshotMu.RLock()` 再获取 `mu.Lock()`。这一锁顺序约定仅靠代码惯例维持，未来新增指标类型或写入方法时容易遗漏。

**修复**：

1. **新建 [guard.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/guard.go)**：定义 `snapshotGuard` 结构体和 `snapshotProtected` 接口
   - `snapshotGuard` 封装 `*sync.RWMutex`，提供 `write(fn func())` 方法将锁获取模式封装为一处
   - `snapshotProtected` 接口要求实现 `snapshotGuardPtr()` 方法

2. **修改四个指标类型**：将 `snapshotMu *sync.RWMutex` 替换为 `guard snapshotGuard`，所有写入操作改用 `guard.write(func() { ... })` 模式
   - [counter.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/counter.go#L8-L9)
   - [gauge.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/gauge.go#L8-L9)
   - [histogram.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/histogram.go#L10-L11)
   - [summary.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/metrics/summary.go#L13-L14)

3. **编译时接口检查**：每个指标类型添加 `var _ snapshotProtected = (*metricType)(nil)` 保证新增类型必须实现该接口，否则编译失败

4. **修改 [registry.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3\solocoder-go\internal\metrics\registry.go#L19-L28)**：初始化 `guard` 并传递给各指标的构造函数

**防护效果**：
- `snapshotGuard.write()` 将锁获取模式封装为一处，新增写入方法只需调用 `guard.write()` 即可自动获得快照保护
- `snapshotProtected` 接口的编译时检查确保新增指标类型必须嵌入 `snapshotGuard`
- 裸访问 `mu.Lock()` 而不经过 `guard.write()` 的写法在代码审查中易于识别