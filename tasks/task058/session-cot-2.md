toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
我来仔细分析这4个问题，然后逐一修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先修复问题1：从 `ProbeResult` 移除 `Name` 字段，新增 `ProbeCheckResult` 输出类型，同时重构 `aggregateResults` 消除对 `result.Name` 的依赖。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
现在更新 `AggregatedHealth` 使用 `ProbeCheckResult`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
添加内部 `probeOutcome` 类型并重构 `Check` 和 `aggregateResults`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
现在重构 `Check` 方法，用 `probeOutcome` 替代直接操作 `ProbeResult`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
现在重构 `aggregateResults` 方法，用 `probeOutcome` 替代原来的 `results + configs` 双参数：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
问题1修复完成。现在修复问题2：`maybeTriggerAlert` 增加不健康状态过滤条件。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
问题2修复完成。现在修复问题3：`NewHealthAggregator` 配置校验失败时返回错误。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          
3个代码修复都完成了。现在更新单元测试以适配这些改动。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
需要修改以下测试：

1. **TestNewHealthAggregatorWithInvalidRatio** — 负数和>1现在应返回 `ErrInvalidConfig`，零值视为"未设置"默认为0.5不报错
2. **TestAlertDegradedToUnhealthy** — healthy→degraded 不再触发告警，只期望1个事件
3. **TestCheckProbeResults** — 使用 `ProbeCheckResult` 类型
4. 新增测试：healthy→degraded 不触发告警、degraded→healthy 不触发告警
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
changes: undefined
          
现在修改 `TestAlertDegradedToUnhealthy`：healthy→degraded 不触发告警，只期望1个事件（degraded→unhealthy）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
changes: undefined
          
现在修改 `TestCheckProbeResults` 使用 `ProbeCheckResult`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
changes: undefined
          
现在在测试文件末尾添加针对告警过滤逻辑的新测试用例：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
changes: undefined
          
现在运行测试验证所有修改是否正确：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/healthagg/ -v undefined
          
所有测试通过！
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
3个问题全部修复完成，53个测试用例全部通过。以下是具体修改总结：

---

### 问题1：ProbeResult.Name 被静默覆盖

**根因**：[Check()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/healthagg/healthagg.go#L207-L236) 中 `result.Name = pc.Name` 强制覆盖了探针函数返回的 Name 字段。

**修复方案**：
- 从 `ProbeResult` 中移除 `Name` 字段，探针函数只需返回 `Healthy` 和 `Details`
- 新增 `ProbeCheckResult` 输出类型（包含 `Name`、`Healthy`、`Details`），由 `ProbeConfig.Name` 填充 Name
- 新增内部类型 `probeOutcome` 配对探针名称、结果和配置，重构 `aggregateResults` 消除按 Name 查找 config 的逻辑

### 问题2：告警回调范围过大

**根因**：[maybeTriggerAlert()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/healthagg/healthagg.go#L329-L361) 对所有状态变化都触发告警，但需求限定在不健康相关场景。

**修复方案**：在更新 `lastStatus` 之后、触发回调之前增加过滤条件：
```go
if prevStatus != StatusUnhealthy && aggregated.Status != StatusUnhealthy {
    ha.mu.Unlock()
    return
}
```
只有当 PreviousStatus 或 CurrentStatus 涉及 `StatusUnhealthy` 时才触发告警回调，`lastStatus` 仍然对所有状态变化正确更新。

### 问题3：NewHealthAggregator 从不返回错误

**根因**：[NewHealthAggregator()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/healthagg/healthagg.go#L110-L126) 对无效 `MajorityRatio` 静默回退到默认值，调用方无法感知配置错误。

**修复方案**：将 `MajorityRatio < 0` 或 `> 1` 改为返回 `ErrInvalidConfig`。`MajorityRatio == 0` 视为 Go 零值（"未设置"），默认为 0.5 不报错。

### 测试更新

- `TestNewHealthAggregatorWithInvalidRatio` — 负数和>1现在期望 `ErrInvalidConfig`，零值期望无错
- `TestAlertDegradedToUnhealthy` — healthy→degraded 不再触发告警，只期望1个事件
- `TestCheckProbeResults` — 使用 `ProbeCheckResult` 类型
- 新增3个测试：`TestAlertNotTriggeredForHealthyToDegraded`、`TestAlertNotTriggeredForDegradedToHealthy`、`TestAlertTriggeredForUnhealthyToDegraded`