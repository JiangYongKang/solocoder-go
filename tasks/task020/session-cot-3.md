让我先分析这些问题并创建修复计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先让我读取最新的 engine.go 和 engine_test.go 文件：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine_test.go
          
好的，现在让我开始修复。首先修复问题 1：DeleteDocument 的并发窗口问题，将倒排索引清理和 docs 删除放在 Engine.mu 的同一写锁范围内，确保原子性：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在让我添加测试用例。首先更新任务状态，然后添加并发去重测试和倒排索引清理直接验证测试：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine_test.go
          
changes: undefined
          
现在运行测试确保所有测试通过：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -v -count=1 undefined
          
测试失败的原因是 DefaultTokenizer 把下划线 `_` 当作分隔符了，所以 `unique_alpha` 被分成了 `unique` 和 `alpha` 两个词。让我修复测试用例：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine_test.go
          
changes: undefined
          
现在重新运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -v -count=1 undefined
          
所有 73 个测试全部通过！现在让我运行诊断并更新文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
现在让我更新文档。首先更新文档删除流程，反映 DeleteDocument 现在在同一个写锁范围内完成原子操作：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在让我更新第 8 章，补充 DeleteDocument 的原子性保证，并在注意事项中补充测试验证策略：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在让我在文档末尾添加测试策略与验证方法的章节：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
所有修改完成，让我更新任务状态并做最终总结：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复并通过测试！以下是本次修复的详细总结：

---

## ✅ 修复完成总结

### 🐛 问题 1：DeleteDocument IDF 计算窗口期

**修复位置**: [engine.go#L281-L297](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L281-L297)

**问题**: 原先先删 docs map 再清理倒排索引，窗口期内 Search 会用已减少的 `totalDocs` 计算 IDF，但词项的 `docsWithTerm` 仍包含被删文档，导致评分偏低。

**修复**: 将 `invertedIndex.RemoveDocument()` 和 `delete(e.docs, docID)` 放在**同一个 Engine.mu 写锁范围内**原子执行，执行顺序固定为「先清理索引 → 再删 docs」。

---

### 🧪 问题 2：并发去重测试缺失

**修复位置**: [engine_test.go#L1307-L1346](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine_test.go#L1307-L1346)

**新增测试**: `TestConcurrent_AddDocument_SameDocID`

- 启动 **20 个 goroutine** 并发调用 `AddDocument`，全部使用**同一个 docID**
- 断言恰好 **1 次成功**、**19 次 ErrDuplicateDocID**、最终 `DocumentCount() == 1`
- 专门验证 TOCTOU 竞态防护的有效性

---

### 🧪 问题 3：倒排索引清理直接验证缺失

**修复位置**: [engine_test.go#L1348-L1478](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine_test.go#L1348-L1478)

**新增 3 个测试用例**:

| 测试 | 验证目标 |
|------|---------|
| `TestInvertedIndex_RemoveDocument_UniqueTerms` | InvertedIndex 单元级：独有词项从 index map 完全移除，`GetTermCount() == 0` |
| `TestInvertedIndex_RemoveDocument_SharedTerms` | InvertedIndex 单元级：共享词项 PostingList 从 3 减为 2，独有词项被清理 |
| `TestDeleteDocument_InvertedIndexCleanup` | Engine 集成级：删除文档后独有词项消失、共享词项 PostingList 长度正确、词项总数精确减少 |

这些测试**直接检查倒排索引内部状态**（`HasTerm`、`GetPostingList`、`GetTermCount`），避免了仅通过 `Search` 间接验证可能遗漏的索引泄露问题。

---

### 📝 问题 5：文档更新

**修复位置**: [fulltext.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fulltext.md)

更新内容：
- **第 4.4 节**：更新文档删除流程图，说明原子操作范围
- **第 8.3 节**：新增「文档删除的原子性保证」和「IDF 一致性保证」，附错误模式 vs 正确模式对比
- **第 12 章**：更新注意事项第 8 条
- **新增第 13 章**「测试策略与验证方法」，包含：
  - 13.1 并发去重测试策略（设计思路 + 与 TestConcurrent_AddDocument 的区别）
  - 13.2 倒排索引清理验证方法（两层验证 + 为何需要直接验证的原理说明）
  - 13.3 DeleteDocument 原子性测试保证（代码层面 + 测试层面）

---

### ✅ 测试结果

```
PASS
ok  	solocoder-go/internal/fulltext	0.320s
```

**73 个测试全部通过**，其中新增 4 个测试：
- `TestConcurrent_AddDocument_SameDocID`
- `TestInvertedIndex_RemoveDocument_UniqueTerms`
- `TestInvertedIndex_RemoveDocument_SharedTerms`
- `TestDeleteDocument_InvertedIndexCleanup`