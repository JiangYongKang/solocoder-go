让我先详细查看当前代码，理解每个问题的具体位置。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\engine.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
让我先运行测试查看当前状态，了解哪些测试需要修复：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v undefined
          
现在我清楚了问题全貌。`engine.go` 已经改为返回 `(*TSEngine, error)` 并添加了 `ValidateConfig`，但测试文件还没同步更新。让我创建任务列表：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先读取测试文件被截断的部分：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在我逐一修复所有 9 处编译错误。这些都是 `NewTSEngineWithConfig` 返回值从 1 个变为 2 个的问题：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
现在修复 `TestTTL_ZeroTTL` - 应该期望 `ErrInvalidTTL` 而不是静默替换：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestTTL_CleanupBatchSizeRespected`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestTTL_CleanupOnEmptyEngine`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestConcurrentWriteAndCleanup`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestNewTSEngineWithConfig_ZeroCleanupBatchSize` - 应该期望错误：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestNewTSEngineWithConfig_NegativeCleanupInterval` - 应该期望错误：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
修复 `TestTagIndex_AfterCleanupStillCorrect`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
所有 9 处编译错误已修复。同时需要添加对 `ErrInvalidTTL`、`ErrInvalidBatchSize`、`ErrInvalidInterval` 错误变量的测试到 `TestErrors_Values`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在修复非确定性降采样测试。需要将所有使用 `time.Now().UnixMilli()` 搭配固定窗口做降采样的测试对齐到窗口边界：

**修复 TestDownsample_Avg**：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
**修复 TestDownsample_Sorted**：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
**修复 TestDownsample_WithTagFilter**：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
changes: undefined
          
现在让我也检查 `TestDownsample_NoDataInRange` 和 `TestDownsample_Count`，看看是否也有类似的时间戳问题：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
`TestDownsample_NoDataInRange` 和 `TestDownsample_Count` 使用固定时间戳或只检查数量，不受窗口对齐影响。现在让我检查 `TestConcurrentDownsample`：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tsdb\tsdb_test.go
          
`TestConcurrentDownsample` 只检查是否报错，不验证具体值，所以不受窗口对齐影响。现在运行测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -v undefined
          
所有 78 个测试通过。让我多运行几次确保确定性：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tsdb/ -count=5 -run "TestDownsample" undefined
          
5 次连续运行全部通过，测试具有确定性。现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
现在更新文档。需要修改多处，涵盖配置校验约定和 TTL 禁用语义：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
在 3.5 节之后添加 3.6 TTLDisabled 常量说明：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
更新 5.4.2 清理执行逻辑中的 TTL 检查描述：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
更新 8.3 自定义 TTL 配置示例，展示错误返回：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
更新 8.4 禁用 TTL 示例，使用 `TTLDisabled` 常量并处理错误：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
更新第 9 节错误定义表：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
现在在错误定义之后、性能特征之前添加配置校验约定和 TTL 禁用语义的专门章节：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          
我注意到旧的错误表行还残留了。让我检查并修复：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
我看到错误表中 663-665 行还有旧描述，需要更新：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tsdb.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有任务完成。以下是修复内容的总结：

## 修复总结

### 问题 1：非确定性降采样测试

**根因**：`TestDownsample_Avg`、`TestDownsample_Sorted`、`TestDownsample_WithTagFilter` 使用 `time.Now().UnixMilli()` 生成时间戳搭配 1 秒窗口做降采样，但没有将时间对齐到窗口边界。当 `time.Now()` 的毫秒偏移量导致数据点意外跨窗口时，测试结果不确定。

**修复**：将所有三个测试的时间戳生成方式从 `now := time.Now().UnixMilli()` 改为 `baseTs := (time.Now().UnixMilli() / windowMs) * windowMs`，确保数据点始终落在预期的窗口内。这与已有的 `TestDownsample_MinMax`、`TestDownsample_SingleWindowMultiplePoints` 等测试的做法保持一致。

涉及的测试函数：
- [TestDownsample_Avg](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/tsdb_test.go#L359-L394)
- [TestDownsample_Sorted](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/tsdb_test.go#L544-L570)
- [TestDownsample_WithTagFilter](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/tsdb_test.go#L917-L945)

### 问题 2：配置校验错误返回

**根因**：[engine.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go) 已定义了 `ErrInvalidTTL`、`ErrInvalidBatchSize`、`ErrInvalidInterval` 三个错误变量，且 `NewTSEngineWithConfig` 已通过 `ValidateConfig` 返回错误，但有 9 处测试调用仍使用旧的单返回值签名 `e := NewTSEngineWithConfig(cfg)`，导致编译失败。此外，3 个测试（`TestTTL_ZeroTTL`、`TestNewTSEngineWithConfig_ZeroCleanupBatchSize`、`TestNewTSEngineWithConfig_NegativeCleanupInterval`）仍然期望无效配置被静默替换为默认值。

**修复**：
1. 9 处调用改为 `e, err := NewTSEngineWithConfig(cfg)` 并检查 `err`
2. 3 个测试从"期望静默替换"改为"期望返回具体错误"：
   - `TestTTL_ZeroTTL`：期望 `ErrInvalidTTL`（TTL=0 不再被替换为 24h）
   - `TestNewTSEngineWithConfig_ZeroCleanupBatchSize`：期望 `ErrInvalidBatchSize`
   - `TestNewTSEngineWithConfig_NegativeCleanupInterval`：期望 `ErrInvalidInterval`

### 问题 3：TTL 禁用语义一致性

**根因**：`cleanupExpired` 用 `e.ttl == TTLDisabled` 判断禁用，而 `ValidateConfig` 用 `cfg.TTL != TTLDisabled && cfg.TTL <= 0` 拒绝非 `-1` 的负值。两个代码点已通过 `TTLDisabled` 常量协作，但文档中仍描述为"负数表示禁用"、"0 表示使用默认值"的旧语义。

**修复**：
- [engine.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tsdb/engine.go) 中的 `TTLDisabled` 常量和 `ValidateConfig` 逻辑已是正确的，无需修改
- [docs/tsdb.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tsdb.md) 新增了 **9.1 配置校验错误返回约定** 和 **9.2 TTL 禁用语义的实现方式** 两节，详细说明了 `TTLDisabled` 常量在入口校验和运行时检查两个代码点之间的协作保证

### 验证结果

`go test ./internal/tsdb/ -v` 所有 78 个测试通过，`-count=5` 连续运行 5 次均通过，确认测试具有确定性。