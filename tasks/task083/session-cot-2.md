toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先读取现有代码，然后进行修复：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
好的，分析完成。现在开始修复所有4个问题。我将重写核心代码，一次性修复所有问题：










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
代码修复完成。现在更新任务状态并修改测试文件：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在读取现有测试文件并更新：














        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
现在更新测试文件，修改现有季节性配置（增加PeriodSlot），并添加新增场景的测试：
















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          

        
            
toolName: CompactFake
            
status: success
          
          
好的，根据摘要，代码修复和测试用例编写已完成。现在我先运行测试验证修复效果。
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 undefined
          
有个编译错误，变量 v 未使用。让我查看这个位置的代码：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
修复未使用变量 v 的问题：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
现在重新运行测试：



        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 undefined
          
所有 37 个测试（包括子测试）全部通过！现在更新待办事项并开始更新文档：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在读取现有文档，然后进行更新：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
现在更新文档。我需要进行多处修改：

1. Config 字段表新增 `PeriodSlot` 和 `SeasonalEpoch`
2. 错误定义新增 `ErrInvalidPeriodSlot`
3. 季节性周期模型公式重写（基于时间戳的计算）
4. 新增"配置变更基线迁移策略"章节
5. 更新季节性模式的示例代码

先更新 Config 字段表和错误定义：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
现在更新季节性周期模型的数学公式部分：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
现在更新错误定义，新增 PeriodSlot 相关错误：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
现在更新季节性模式的示例代码，补充 PeriodSlot 和 SeasonalEpoch：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
现在更新最后一个完整示例中的 qpsCfg 配置：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
文档更新完成。现在标记最后一个任务完成：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复任务已全部完成。下面是本次修复的总结：

---

## 修复完成总结

### 1. 季节性周期索引修复 ✅
**问题**：季节性索引使用纯序列计数器 `pointCount % PeriodLength`，完全未使用 `DataPoint.Timestamp`。

**修复**：
- `Config` 新增两个字段：
  - `PeriodSlot time.Duration`：每个周期槽位的时间跨度（如 1小时）
  - `SeasonalEpoch time.Time`：周期基准时间（对齐点）
- 新增 `computeSeasonalIndex(timestamp)` 方法，公式：
  ```
  totalSlots = (T - Epoch) / PeriodSlot
  idx = totalSlots % PeriodLength
  ```
- 新增 `ErrInvalidPeriodSlot` 错误校验

### 2. UpdateConfig 基线数据迁移 ✅
**问题**：配置变更时直接丢弃所有周期位置基线数据，检测器冷启动。

**修复**：实现 5 种配置变更场景的智能迁移：

| 场景 | 策略 |
|------|------|
| 非季→非季 | 全局基线完全保留 |
| 季节→非季 | 所有槽位的窗口数据合并到全局基线 |
| 非季→季节 | 全局窗口值按轮询分配到各槽位 |
| 同长度（PeriodLength/Slot/Epoch 均不变） | 逐槽位深拷贝，100% 保留 |
| 不同长度 | **GCD最大公约数映射**：按 `gcd(old,new)` 步长均匀分散旧数据 |

新增辅助函数：`cloneWindowStats`、`mergeWindowStats`、`gcd(a,b)`

### 3. BatchAdd 竞态条件修复 ✅
**问题**：`BatchAdd` 先 `Lock→检查closed→Unlock`，再循环调用 `Add`（每次再加锁），存在检查/执行间的竞态窗口。

**修复**：
- 抽取核心逻辑到无锁内部方法 `addLocked(point)`
- `Add` 和 `BatchAdd` 统一调用 `addLocked`
- `BatchAdd` 改为**全程单次持锁**：`Lock→检查closed→循环addLocked→Unlock`

### 4. 单元测试通过 ✅
运行 `go test ./internal/tsanomaly/ -v -count=1`，**37个测试（含子测试）全部 PASS**：

新增 10 个测试用例：
- `TestDetector_Seasonal_UnevenTimestamps`：不均匀时间戳（偏移量 0/3/7/12/15/22/28/31/33/37/42/48）
- `TestDetector_Seasonal_DailyHourCycle`：完整 24 小时日周期（4天数据 + 早/晚高峰 + 凌晨低谷）
- `TestDetector_BatchAdd_NoRaceWithClose`：并发 Close + BatchAdd 竞态验证
- `TestDetector_UpdateConfig_PreservesGlobalBaseline`：非季→非季基线完整保留
- `TestDetector_UpdateConfig_NonSeasonalToSeasonal`：关→开季节性，轮询分配
- `TestDetector_UpdateConfig_SeasonalToNonSeasonal`：开→关季节性，合并到全局
- `TestDetector_UpdateConfig_SamePeriodLength_PreservesData`：同周期长度数据完全一致
- `TestDetector_UpdateConfig_ChangePeriodLength`：PeriodLength 4→6，GCD=2 映射
- `TestGcdHelper`：gcd 工具函数单元测试（4&6→2，7&13→1，负数等）
- `TestValidateConfig` 新增 PeriodSlot 正负校验用例

### 5. 文档更新 ✅
[docs/tsanomaly.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tsanomaly.md) 已更新：
- **Config 字段表**：新增 `PeriodSlot`、`SeasonalEpoch` 两字段说明
- **季节性数学模型**：完全重写公式，说明基于时间戳的槽位索引计算
- **错误定义**：新增 `ErrInvalidPeriodSlot`
- **新增章节 "配置变更基线迁移策略"**：详细说明 5 种场景的迁移策略和设计原则
- **示例代码**：所有启用季节性的示例（日周期示例、服务监控示例）均已补充 `PeriodSlot` 和 `SeasonalEpoch` 配置