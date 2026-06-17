toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
在测试文件中添加无效策略的单元测试。先读取测试文件末尾定位添加位置：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
在 `TestNewHealthAggregatorWithInvalidRatio` 之后添加无效策略测试：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\healthagg\healthagg_test.go
          
changes: undefined
          
现在运行测试验证：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/healthagg/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
修复完成，全部 55 个测试用例通过。

---

### 修改内容

**[NewHealthAggregator()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/healthagg/healthagg.go#L110-L132)** — 新增 `Strategy` 枚举值校验

在 MajorityRatio 校验之前增加 Strategy 的 switch 校验：

```go
switch cfg.Strategy {
case StrategyAllHealthy, StrategyWeightedMajority:
default:
    return nil, ErrInvalidConfig
}
```

只有 `StrategyAllHealthy`(0) 和 `StrategyWeightedMajority`(1) 是合法值，传入 `AggregationStrategy(99)` 等无效枚举值时返回 `ErrInvalidConfig`，调用方可以明确感知配置错误。

### 新增测试

| 测试用例 | 说明 |
|----------|------|
| `TestNewHealthAggregatorWithInvalidStrategy` | 验证 -1、10、99 等无效策略值都返回 `ErrInvalidConfig`，且返回 nil aggregator |
| `TestNewHealthAggregatorValidStrategies` | 验证两种合法策略值能成功创建并正确保存 strategy |