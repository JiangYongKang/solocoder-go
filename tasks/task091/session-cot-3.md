让我先查看需要修复的具体代码位置。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
让我找到 TestConcurrency 中 verify_node_count 的具体位置：



        
            
toolName: search_by_regex
            
status: success
          
query: verify node count
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
现在修复这两个问题：

### 问题 1：修复 verify_node_count 的无效断言




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
changes: undefined
          
### 问题 2：修复 decrease_weight 中迁移比例断言






        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
changes: undefined
          
现在运行测试确保所有测试通过：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/chainhash/ -v undefined
          
所有测试通过！让我更新任务状态。










        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## 测试质量问题修复完成 ✅

所有测试通过：`PASS ok solocoder-go/internal/chainhash 0.428s`

---

### 修复内容汇总

#### 1. verify_node_count 无效断言修复

**文件**: [chainhash_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chainhash/chainhash_test.go#L804-L810)

**问题**: `nodeCount < 0` 断言恒为 false，因为 Go 中 `len(map)` 永远返回非负值。

**修复**: 改为检查节点数量是否在合理范围内 `[0, numGoroutines * 5]`。
- 下界 0：节点数不可能为负
- 上界 `numGoroutines * 5 = 50`：因为 nodeID 格式为 `node-{id}-{j%5}`，每个 goroutine 最多操作 5 个唯一节点，10 个 goroutine 最多 50 个

```go
// 修复前（恒不触发）
if nodeCount < 0 { ... }

// 修复后（有实际意义）
maxPossibleNodes := numGoroutines * 5
if nodeCount < 0 || nodeCount > maxPossibleNodes { ... }
```

---

#### 2. decrease_weight 迁移比例断言修复

**文件**: [chainhash_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chainhash/chainhash_test.go#L426-L432)

**问题**: 当实际迁移比例超出容差范围时，使用 `t.Logf` 仅记录日志而不标记失败，导致测试即使比例偏离也显示 PASS。

**修复**: 将 `t.Logf` 改为 `t.Errorf`，超出容差时正确标记测试失败。

```go
// 修复前（即使超差也 PASS）
t.Logf("migration ratio %.4f differs from expected %.4f (within tolerance %.4f)", ...)

// 修复后（超差时 FAIL）
t.Errorf("migration ratio %.4f differs from expected %.4f by more than tolerance %.4f", ...)
```