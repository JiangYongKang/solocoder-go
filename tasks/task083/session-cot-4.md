好的，我来清理已废弃的 `gcd` 函数及其测试。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
确认 `UpdateConfig` 中已不再使用 `gcd`，现在删除该函数：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
现在删除测试文件中的 `TestGcdHelper` 测试：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestGcdHelper
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
现在运行测试验证：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
---

## 代码清理完成总结

### 清理内容

**1. 删除已废弃的 `gcd` 函数**
- 位置：[tsanomaly.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go) 原第 341-349 行
- 原因：PeriodLength 变更逻辑从 GCD 多槽位映射改为一一映射后，`gcd` 函数不再有任何调用方

**2. 删除 `TestGcdHelper` 测试**
- 位置：[tsanomaly_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly_test.go) 原第 1688-1707 行
- 原因：被测函数 `gcd` 已删除，对应单元测试一并清理

### 测试验证

运行 `go test ./internal/tsanomaly/ -v -count=1`，**36 个测试全部 PASS**（清理前为 37 个，减去已删除的 `TestGcdHelper`）。

### 当前测试清单

| 测试函数 | 说明 |
|---------|------|
| `TestValidateConfig` | 配置校验（13 个子用例） |
| `TestNewDetector` | 检测器创建 |
| `TestDetector_Add_NilPoint` | nil 数据点错误 |
| `TestDetector_Add_ClosedDetector` | 已关闭检测器添加 |
| `TestDetector_Add_InsufficientSamples` | 样本不足不检测 |
| `TestDetector_Add_UpAnomaly` | 上偏异常检测 |
| `TestDetector_Add_DownAnomaly` | 下偏异常检测 |
| `TestDetector_DirectionUpOnly` | 仅上偏方向 |
| `TestDetector_DirectionDownOnly` | 仅下偏方向 |
| `TestDetector_BaselineIncrementalUpdate` | 基线增量更新 |
| `TestDetector_WindowEviction` | 窗口淘汰 |
| `TestDetector_SeasonalMode` | 季节性模式基本功能 |
| `TestDetector_Seasonal_UnevenTimestamps` | 不均匀时间戳季节性 |
| `TestDetector_Seasonal_DailyHourCycle` | 24小时日周期场景 |
| `TestDetector_SeasonalBaselineErrors` | 季节性基线查询错误 |
| `TestDetector_AnomalyQuery` | 异常查询过滤 |
| `TestDetector_AnomalySortedOrder` | 异常时间排序 |
| `TestDetector_AnomalyHistoryLimit` | 异常历史上限 |
| `TestDetector_BatchAdd` | 批量添加 |
| `TestDetector_BatchAdd_NoRaceWithClose` | BatchAdd 竞态验证 |
| `TestDetector_Reset` | 重置检测器 |
| `TestDetector_UpdateConfig` | 配置更新基本功能 |
| `TestDetector_UpdateConfig_PreservesGlobalBaseline` | 非季→非季基线保留 |
| `TestDetector_UpdateConfig_NonSeasonalToSeasonal` | 非季→季节迁移 |
| `TestDetector_UpdateConfig_SeasonalToNonSeasonal` | 季节→非季迁移（无重复） |
| `TestDetector_UpdateConfig_SamePeriodLength_PreservesData` | 同周期长度完全保留 |
| `TestDetector_UpdateConfig_ChangePeriodLength` | 周期长度变更（一一映射） |
| `TestDetector_Close` | 关闭检测器 |
| `TestDetector_ConcurrentAccess` | 并发安全 |
| `TestDetector_SeverityLevels` | 严重程度分级 |
| `TestDetector_ZeroStdDev` | 零标准差场景 |
| `TestDeviationDirection_String` | 方向字符串表示 |
| `TestDetector_AnomalyEventFields` | 异常事件字段完整 |
| `TestDetector_PointCount` | 数据点计数 |
| `TestDetector_SeasonalReset` | 季节性重置 |