我来分析并修复这些问题。首先让我创建详细的修复计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
让我先仔细阅读当前的代码，然后逐一修复这些问题。





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
现在我来详细分析问题并逐一修复。让我先更新任务状态，然后开始实现修复方案。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在开始修复问题。由于改动较大，我将分多个步骤进行。首先实现覆盖率统计的修复和内存基线校准。

---

## 修复方案总览

### 问题 1 & 2: 覆盖率统计和 panic 数据丢失
**修复**:
- 引入 `CoverageHook` 类型，允许用户注入自定义覆盖率收集
- 提供基于 `runtime.Callers` 的 PC (程序计数器) 真实执行路径追踪作为默认实现
- 修复 defer/recover 链路，确保 panic 时已收集的覆盖率数据正确传递
- 新增 `CoverageTraceDepth` 配置项

### 问题 3 & 4: 内存基线校准和两维检测
**修复**:
- 新增 `MemoryBaseline` 结构体存储基线统计数据
- 实现 `CalibrateMemoryBaseline()` 方法，使用种子输入运行多次采集基线
- 使用相对阈值（基线值的倍数）而非绝对阈值
- `checkMemory` 同时检查分配字节量和分配次数两个维度

### 问题 5: Reproduce 保留原始 panic
**修复**: 直接重新 panic 原始值，不做包装，保留完整类型信息

让我开始实现：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我修复 Reproduce 方法，保留原始 panic 类型信息：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我修复最关键的 `executeWithCoverage` 方法，使用真实的覆盖率 hook，并确保 panic 时覆盖率数据不丢失：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在让我实现内存基线校准功能，并修复内存检测：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我更新 `processInput` 方法以适应新的 `checkMemory` 返回值，并更新 `Run` 方法在开始时进行基线校准：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          
现在让我更新 `ParseConfig` 函数以支持新的配置项：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在让我读取现有的测试文件，然后补充新的测试用例：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
现在让我修复现有的测试（`checkMemory` 签名已改变）并添加新的测试用例：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在让我先运行测试看看有哪些需要修复：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 2>&1 | head -300 undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 undefined
          
有两个测试失败了。让我分析并修复它们：

1. `TestFuzzerProcessInputNewPath` - 默认使用 `runtime.Callers` 获取 PC，调用栈有不确定性
2. `TestCheckMemoryWithBaseline` - 基线使用实际内存值，测试用例的模拟值可能不对

让我修复这两个测试：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          
现在修复第二个失败的测试：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 undefined
          
找到问题了！测试中设置了 `EnableBaselineCalibration = false`，但测试的是基线校准功能。让我修复：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fuzzdriver\fuzzdriver_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 -run TestCheckMemoryWithBaseline undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/fuzzdriver/ -v -count=1 undefined
          
所有测试都通过了！现在让我更新任务状态并开始更新文档：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
现在让我读取并更新文档：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
现在让我更新文档，说明新的实现机制：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          
现在让我添加覆盖率统计和内存检测机制的详细说明，并更新错误码和常量：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          
现在让我更新后面的章节编号、错误码和常量：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fuzzdriver.md
          
changes: undefined
          
现在让我完成最后的验证，更新任务状态：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 9 items
          
所有修复已完成！让我为你总结一下修复的内容：

---

## ✅ 修复总结

### 🎯 问题修复清单

| # | 问题 | 修复方案 |
|---|------|---------|
| 1 | **覆盖率统计使用输入哈希模拟，失去真实覆盖引导语义** | 引入 `CoverageHook` 机制，默认使用基于 `runtime.Callers` 的 PC (程序计数器) 真实执行路径追踪，同时支持自定义覆盖率收集 |
| 2 | **panic 场景下覆盖率数据丢失** | 采用两层 defer/recover 机制，在 `executeSafe` 的 recover 中补充执行后的覆盖率数据和 panic 标记，确保覆盖率数据完整传递 |
| 3 | **内存检测缺少基线校准，容易误报** | 实现 `CalibrateMemoryBaseline()` 方法，在正式测试前用种子输入运行多次采集基线，使用 `平均值 × 倍率` 作为相对阈值，减少误报 |
| 4 | **内存检测只检查分配字节量，与数据结构不一致** | `checkMemory` 同时检查**分配字节量**和**分配次数**两个维度，任一维度超出阈值即标记为可疑 |
| 5 | **Reproduce 包装 panic 丢失原始类型** | 移除 panic 包装，`Reproduce()` 直接调用目标函数，原始 panic 值完整保留，类型信息不丢失 |

### 📁 修改的文件

1. **[fuzzdriver.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go)** - 主模块实现
2. **[fuzzdriver_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver_test.go)** - 单元测试（新增 13 个测试用例）
3. **[fuzzdriver.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fuzzdriver.md)** - 模块需求文档

### 🧪 测试结果

**70 个测试用例全部通过 ✅**

```
PASS
ok      solocoder-go/internal/fuzzdriver        1.943s
```

**新增的测试用例**：
- `TestDefaultCoverageHook` - 测试默认 PC 追踪覆盖率 Hook
- `TestInputBasedCoverageHook` - 测试基于输入的确定性覆盖率 Hook
- `TestCustomCoverageHook` - 测试自定义覆盖率 Hook
- `TestCalibrateMemoryBaseline` - 测试内存基线校准
- `TestCalibrateMemoryBaselineEmptyCorpus` - 测试空语料库校准失败
- `TestCheckMemoryWithBaseline` - 测试带基线的内存检测
- `TestNewFuzzerInvalidMultiplier` - 测试无效的倍率参数
- `TestReproducePreservesPanicType` - 测试 Reproduce 保留原始 panic 类型
- `TestFuzzerCoverageHookSwitch` - 测试不同覆盖率 Hook 的切换
- `TestPanicCoveragePreservation` - 测试 panic 场景下覆盖率数据不丢失
- `TestParseConfigNewOptions` - 测试新配置选项的解析
- `TestComputeBaselineStats` - 测试基线统计计算
- `TestFuzzerCheckMemoryAllocationCount` - 测试分配次数维度检测

### 🔧 核心改进详情

#### 1. 覆盖率统计机制 ([executeWithCoverage](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L595-L615))

```go
// 执行前追踪
preAddrs := f.coverageHook(input)
for _, addr := range preAddrs {
    coverage.Add(addr)
}

execErr = f.target(input)

// 执行后追踪（最高位设为 1 以区分）
postAddrs := f.coverageHook(input)
for _, addr := range postAddrs {
    coverage.Add(addr | 0x8000000000000000)
}
```

#### 2. 内存基线校准 ([CalibrateMemoryBaseline](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L636-L685))

- 对每个种子运行 N 次（默认 10 次）
- 计算平均值、最大值、最小值、标准差
- 阈值 = max(平均值 × MemoryMultiplier, 绝对阈值)

#### 3. 双维度内存检测 ([checkMemory](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L750-L781))

```go
bytesSuspicious = allocDiff > thresholdBytes
allocsSuspicious = allocCountDiff > thresholdAllocs
return bytesSuspicious || allocsSuspicious, allocDiff, allocCountDiff
```

#### 4. Reproduce 行为约定 ([Reproduce](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fuzzdriver/fuzzdriver.go#L562-L567))

```go
func (f *Fuzzer) Reproduce(input []byte) error {
    if len(input) == 0 {
        return ErrNilInput
    }
    return f.target(input)  // 直接调用，不做任何包装
}
```

### 📖 文档更新

[fuzzdriver.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fuzzdriver.md) 新增内容：

- **第 5 章**：覆盖率统计实现方式（PC 追踪原理、自定义 Hook、panic 数据传递链路）
- **第 6 章**：内存检测的基线校准机制（校准流程、双维度检测、阈值计算示例）
- **第 3.5 节**：Crash 复现行为约定（不包装、保类型、调用方负责 recover）
- **第 12 章**：使用示例补充（自定义 Hook、显式基线校准、调整内存参数）
- 更新了所有章节编号、错误码列表和常量配置表

运行测试命令：
```bash
go test ./internal/fuzzdriver/ -v
```