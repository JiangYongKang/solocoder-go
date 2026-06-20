让我先读取当前代码，然后修复这两个问题。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
问题分析清楚了，现在开始修复：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
**修复 1**：新增 `Trie.EnsureWord` 方法，仅添加新词而不覆盖已有频率，然后让 `AddWord` 调用它：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          
现在让 `AddWord` 使用 `EnsureWord` 而不是 `InsertWithFreq`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
**修复 2**：消除 `CorrectLimit` 和 `correctNoLock` 的重复代码，让 `CorrectLimit` 获取锁后直接调用 `correctNoLock`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          
两个修复都完成了。现在运行测试验证：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/suggest/ -v -count=1 undefined
          
`TestTrie_InsertWithFreq_Overwrite` 失败了，因为 `InsertWithFreq` 不应改变其覆盖语义。让我修正——`InsertWithFreq` 应保持覆盖行为（这是它的设计意图），而 `AddWord` 使用新的 `EnsureWord` 不覆盖已有频率：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          
现在再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/suggest/ -v -count=1 2>&1 | Select-String -Pattern "(PASS|FAIL|ok)" undefined
          
所有测试通过！让我更新任务状态：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
两个问题均已修复，所有测试通过 ✅。总结如下：

## 修复内容

### 1. AddWord 擦除已有词频率

**根因**：`AddWord` 调用 `InsertWithFreq(word, 0)`，而 `InsertWithFreq` **无条件**将 `node.freq` 设为指定值。当词已通过 `SubmitSearch` 积累了真实搜索频率后，`AddWord` 会将其归零。

**修复**：新增 [Trie.EnsureWord](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest.go#L109-L132) 方法，仅当词不存在时才创建并设置 `freq=0`，词已存在时不触碰频率。[AddWord](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest.go#L502-L507) 改为调用 `EnsureWord`。

三种 Trie 写入方法的语义区分：
| 方法 | 词已存在时行为 | 调用方 |
|------|-------------|--------|
| `Insert` | `freq++`（累加） | `SubmitSearch` |
| `InsertWithFreq` | `freq = 指定值`（覆盖） | `AddWordWithFreq` |
| `EnsureWord` | 不修改 freq（保护） | `AddWord` |

### 2. 消除纠错逻辑重复

**根因**：[CorrectLimit](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest.go#L552-L557) 与 [correctNoLock](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest.go#L620-L672) 有 50+ 行完全重复的纠错逻辑。

**修复**：`CorrectLimit` 获取引擎级读锁后，直接调用 `correctNoLock`，消除了全部重复代码。`SuggestLimit` 内部同样调用 `correctNoLock`，确保两处纠错逻辑永远一致。

### 测试结果
```
PASS
ok   solocoder-go/internal/suggest   0.233s
```