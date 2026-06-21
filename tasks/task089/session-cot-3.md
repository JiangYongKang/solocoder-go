我来分析并修复这些问题。首先创建任务列表。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先仔细检查当前代码的问题，然后逐一修复。




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
让我重新设计覆盖率收集机制。核心问题是 `runtime.Callers` 只能捕获框架的调用栈，无法反映被测函数内部的分支。我将实现一个基于 goroutine-local 的覆盖率追踪器，让用户可以在被测函数中主动标记覆盖点。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我添加基于 goroutine-local 的覆盖率追踪机制，让用户可以在被测函数中主动标记覆盖点：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
我需要添加 `bytes` 包的导入：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我更新 `executeWithCoverage` 方法，集成 goroutine-local 覆盖率追踪器，并修复 panic 时覆盖率丢失的问题：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
现在让我修复核心问题。首先重写 `executeWithCoverage` 和 `executeSafe`，使用 goroutine-local 覆盖率追踪器确保 panic 时数据不丢失：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在修复问题 3 和 4：标准差计算缺少开平方，以及可疑内存记录阈值不区分维度。













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
现在让我先查看 `SuspiciousMemoryRecord` 结构体的定义：















        
            
toolName: search_by_regex
            
status: success
          
query: type SuspiciousMemoryRecord struct
          
现在让我修复标准差计算和可疑内存记录结构。先修改结构体定义：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在修复标准差计算（添加 `math.Sqrt`）：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我更新 `checkMemory` 和 `recordSuspiciousMemory` 函数，使其与新结构体配合：





















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我更新 `processInput` 中的调用以适配新的 `checkMemory` 签名：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先运行测试看看有哪些问题需要修复：

















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 2>&1 undefined
          
有几个测试文件中的调用需要更新。让我修复测试文件：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在让我找到并修复 `TestCheckMemoryWithBaseline` 测试：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在让我找到并修复 `SuspiciousMemoryRecord` 相关的测试：




















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在让我再次运行测试看看：


















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 2>&1 undefined
          
所有测试都通过了！现在让我再补充一些更有针对性的测试用例来覆盖新功能：


























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
让我检查测试文件的导入并添加 `math` 包：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在让我运行所有测试来验证：




























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 undefined
          
有一个测试失败了。问题是 `executeWithCoverage` 的 defer 会先清理掉 coverage，然后外层的 recover 才执行，导致拿不到数据。让我修复：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
等等，defer 中 `ClearCurrentCoverage()` 放在开头会先清理再 recover。让我调整顺序：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我重新运行测试：





























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 -run TestPanicPreservesInstrumentedCoverage undefined
          
现在让我运行所有测试：



























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 undefined
          
所有 79 个测试都通过了！让我更新任务状态：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题已修复，测试全部通过！以下是修复总结：

---

## ✅ 修复总结

### 🎯 问题修复清单

| # | 问题 | 修复方案 |
|---|------|---------|
| 1 | **覆盖率钩子捕获的是框架调用栈，而非被测函数执行路径** | 实现基于 goroutine-local 的覆盖率追踪器，提供 `Cover()` 函数供被测函数主动标记覆盖点，支持 `InstrumentedTarget` 包装器简化插桩 |
| 2 | **panic 时已收集的覆盖率数据丢失** | 将 `ClearCurrentCoverage()` 移至 defer 函数末尾，确保 `recover` 时能从 goroutine-local 获取完整覆盖率数据 |
| 3 | **标准差计算缺少开平方，存储的是方差** | 在 `computeBaselineStats()` 中添加 `math.Sqrt()` 计算真正的标准差 |
| 4 | **可疑内存记录阈值字段无法区分触发维度** | 新增 `ThresholdBytes`、`ThresholdAllocs`、`TriggeredByBytes`、`TriggeredByAllocs` 四个字段，明确记录两个维度的阈值和触发状态 |

### 📁 修改的文件

- **[fuzzdriver.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go)**
- **[fuzzdriver_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver_test.go)**

### 🧪 测试结果

**79 个测试用例全部通过 ✅**

```
PASS
ok      solocoder-go/internal/fuzzdriver        2.159s
```

### 🔧 核心修复详情

#### 1. 覆盖率统计机制重构

新增 goroutine-local 覆盖率追踪 API：

| 函数 | 位置 | 说明 |
|------|------|------|
| `SetCurrentCoverage(cov)` | [fuzzdriver.go#L143-L148](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L143-L148) | 设置当前 goroutine 的覆盖率收集器 |
| `GetCurrentCoverage()` | [fuzzdriver.go#L150-L155](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L150-L155) | 获取当前 goroutine 的覆盖率收集器 |
| `ClearCurrentCoverage()` | [fuzzdriver.go#L157-L162](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L157-L162) | 清理当前 goroutine 的覆盖率收集器 |
| `Cover(addr uint64)` | [fuzzdriver.go#L164-L168](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L164-L168) | 标记一个覆盖点（被测函数内调用） |
| `WrapInstrumentedTarget()` | [fuzzdriver.go#L201-L205](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L201-L205) | 包装插桩目标函数 |

**使用示例**：
```go
instrumented := func(input []byte, cover func(uint64)) error {
    cover(1)  // 标记入口点
    if len(input) > 0 {
        cover(2)  // 标记分支 A
        // ...
    } else {
        cover(3)  // 标记分支 B
        // ...
    }
    return nil
}
target := WrapInstrumentedTarget(instrumented)
```

#### 2. panic 覆盖率数据保留

修复前的问题：`executeWithCoverage` 的 `defer ClearCurrentCoverage()` 会在外层 `recover` 之前执行，导致覆盖率数据丢失。

修复方案（[executeSafe](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L672-L693)）：
- 移除 `executeWithCoverage` 中的 `defer ClearCurrentCoverage()`
- 在 `executeSafe` 的 defer 中，先执行 `recover` 逻辑（获取覆盖率），最后再 `ClearCurrentCoverage()`

```go
defer func() {
    if r := recover(); r != nil {
        // 1. 先 recover，从 goroutine-local 获取已收集的覆盖率
        if cov == nil {
            cov = GetCurrentCoverage()
        }
        // ... 添加 panic 标记
    }
    ClearCurrentCoverage()  // 2. 最后再清理
}()
```

#### 3. 标准差计算修复

修复前（方差）：
```go
stdDevBytes = varianceBytes / float64(n-1)
```

修复后（标准差，[computeBaselineStats](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L786-L793)）：
```go
varianceBytes /= float64(n - 1)
stdDevBytes = math.Sqrt(varianceBytes)
```

#### 4. 可疑内存记录多维度阈值

[SuspiciousMemoryRecord](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L452-L461) 新增字段：

```go
type SuspiciousMemoryRecord struct {
    Input              []byte
    Timestamp          time.Time
    AllocatedDiff      uint64   // 分配字节增量
    AllocationDiff     uint64   // 分配次数增量
    ThresholdBytes     uint64   // 字节维度阈值
    ThresholdAllocs    uint64   // 次数维度阈值
    TriggeredByBytes   bool     // 是否由字节维度触发
    TriggeredByAllocs  bool     // 是否由次数维度触发
}
```

[checkMemory](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L812-L843) 返回 7 个值，完整记录检测结果。

### 🆕 新增测试用例（9 个）

| 测试 | 覆盖内容 |
|------|---------|
| `TestGoroutineLocalCoverage` | goroutine-local 覆盖率基本操作 |
| `TestCoverWithNoCurrentCoverage` | 无覆盖率时 Cover() 不 panic |
| `TestStdDevCalculation` | 标准差计算正确性验证 |
| `TestStdDevSingleSample` | 单样本时标准差为 0 |
| `TestSuspiciousMemoryTriggeredByAllocsOnly` | 仅次数维度触发 |
| `TestSuspiciousMemoryTriggeredByBoth` | 两个维度同时触发 |
| `TestInstrumentedTargetWrapper` | 插桩目标函数包装器 |
| `TestPanicPreservesInstrumentedCoverage` | panic 时插桩覆盖率保留 |
| `TestGoroutineLocalCoverageConcurrency` | 并发下 goroutine-local 隔离性 |

运行测试命令：
```bash
go test ./internal/fuzzdriver/ -v
```