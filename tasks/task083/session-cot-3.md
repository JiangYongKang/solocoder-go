我来分析并修复这两个配置变更时的数据完整性问题。首先读取当前代码理解问题所在。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
我来分析这两个问题：

**问题1分析**：在 [addLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go#L446-L449) 中，季节性模式下每个数据点**同时**被添加到 `globalStats` 和 `seasonalStats[idx]` 中。所以 `globalStats` 已经包含全量数据。但在 [UpdateConfig](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go#L274-L286) 季节→非季节时，又把 `seasonalStats` 合并到 `globalStats`，导致数据重复。

**问题2分析**：[GCD映射策略](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go#L327-L341) 将一个旧槽位的数据复制到多个新槽位（步长=GCD），代表不同周期相位的数据被错误共享。

让我创建修复计划：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复问题1：季节→非季节时，`globalStats` 已包含全量数据，不需要再合并 `seasonalStats`。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
现在修复问题2：PeriodLength 变更时，取消 GCD 多槽位映射，改为一一对应（每个旧槽位只映射到一个新槽位 `oldIdx % newLen`），不同相位的数据不再共享。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly.go
          
changes: undefined
          
现在运行测试，查看是否有测试需要更新：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 2>&1 undefined
          
所有测试都通过了！但我需要检查一下现有的测试用例是否真正验证了修复后的行为。让我查看相关的测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
我发现现有测试不够严格，需要更新测试用例来真正验证修复后的行为。特别是：
1. **问题1测试**：需要验证季节→非季节时 `globalCount` 正好等于样本数（不重复）
2. **问题2测试**：需要验证 PeriodLength 变更时数据总数保持不变（不扩散到多个槽位）

让我先更新这些测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
现在更新 PeriodLength 变更测试，验证数据不扩散到多个槽位：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsanomaly\tsanomaly_test.go
          
changes: undefined
          
现在运行测试验证修复是否正确：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/tsanomaly/ -v -count=1 2>&1 undefined
          
所有 37 个测试全部通过！现在需要更新文档中的"配置变更基线迁移策略"章节，因为策略已从 GCD 映射改为一一对应。同时 `gcd` 函数现在不再被 `UpdateConfig` 使用了，但保留它也无妨。让我更新文档：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsanomaly.md
          
changes: undefined
          
所有任务完成！标记最终状态：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
---

## 修复完成总结

### 问题1：季节→非季节时数据重复计入 ✅

**问题根因**：在 [addLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go#L446-L449) 中，季节性模式下每个数据点**同时**被添加到 `globalStats` 和 `seasonalStats[idx]`。因此 `globalStats` 已经包含全量数据。但 [UpdateConfig](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsanomaly/tsanomaly.go#L274-L286) 季节→非季节时，又把 `seasonalStats` 合并到 `globalStats`，导致数据量虚增一倍。

**修复**：移除重复合并逻辑，直接保留 `globalStats`，仅在需要时裁剪到新的 `WindowSize`。

```go
// 修复前：错误地将 seasonalStats 再次合并到 globalStats
if oldEnable && !newEnable {
    merged := newWindowStats()
    for _, s := range oldSeasonal { mergeWindowStats(merged, s, ...) }
    for e := merged.values.Front(); ... { d.globalStats.add(...) }  // 重复！
}

// 修复后：globalStats 已包含全量数据，直接保留
if oldEnable && !newEnable {
    if d.globalStats.count() > cfg.WindowSize { /* 仅裁剪窗口大小 */ }
    d.seasonalStats = nil
}
```

---

### 问题2：PeriodLength 变更时跨相位数据共享 ✅

**问题根因**：原 GCD 映射策略将一个旧槽位的数据复制到 `newIdx + k*gcd` 等多个新槽位，导致代表不同周期相位的数据被错误共享。

**修复**：改为**一一索引映射**（每个旧槽位只映射到唯一的新槽位），规则：`newIdx = oldIdx % newLen`。

```go
// 修复前：GCD 多槽位映射（错误）
gcdVal := gcd(oldLen, newLen)
for k := 0; k < newLen/gcdVal; k++ {
    mappedIdx := (newIdx + k*gcdVal) % newLen
    mergeWindowStats(newSeasonal[mappedIdx], oldSeasonal[oldIdx], ...)  // 扩散到多个槽位！
}

// 修复后：一一映射（正确）
newIdx := oldIdx % newLen
mergeWindowStats(newSeasonal[newIdx], oldSeasonal[oldIdx], ...)  // 仅映射一次
```

**PeriodLength 4→6 示例**：
| 旧槽位 | 新槽位 | 说明 |
|--------|--------|------|
| 0 → | 0 | 一一对应 |
| 1 → | 1 | 一一对应 |
| 2 → | 2 | 一一对应 |
| 3 → | 3 | 一一对应 |
| - | 4 | 为空（无数据） |
| - | 5 | 为空（无数据） |
| **总样本数** | **12 = 12** | 无复制、无丢失 |

---

### 测试验证 ✅

运行 `go test ./internal/tsanomaly/ -v -count=1`，**37个测试全部 PASS**，包括更新后的严格测试：

1. **`TestDetector_UpdateConfig_SeasonalToNonSeasonal`**：验证 `globalCount == totalSamplesSeasonal`（无重复）
2. **`TestDetector_UpdateConfig_ChangePeriodLength`**：验证：
   - `totalAfter == totalBefore`（总样本数不变）
   - 槽位 0-3 各 3 个样本（正确映射）
   - 槽位 4-5 count=0（无跨相位共享）
   - 各槽位均值正确（10.0, 20.0, 30.0, 40.0）

---

### 文档更新 ✅

[docs/tsanomaly.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tsanomaly.md) 中"配置变更基线迁移策略"章节已更新：
- 场景 5 从"GCD 最大公约数映射策略"改为"一一索引映射策略"
- 补充 PeriodLength 4→6 的具体映射示例
- 更新设计原则：**数据完整性优先于覆盖率，不同相位的数据绝不共享**