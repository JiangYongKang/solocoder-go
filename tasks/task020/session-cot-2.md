我需要分析并修复这几个问题。让我先创建任务计划，然后逐一修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
好，让我先重新读取 engine.go 确保有最新内容，然后开始修复第一个问题。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
现在我已经清楚了所有问题，让我逐一修复。首先修复问题 1、2、3、4，需要对 engine.go 做多处改动。让我同时修复所有代码问题。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在给 InvertedIndex 添加删除文档的方法：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在修复问题 1：AddDocumentWithLanguage 的 TOCTOU 竞态问题，将重复检查和 docs map 写入放在同一个持锁范围内：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在修复问题 2：DeleteDocument 需要清理倒排索引：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在修复问题 3：删除 SearchWithLanguage 中的无效死代码 docMatchCount：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
现在修复问题 4：checkPhraseMatch 中 TermPosting 隐式共享问题。我需要修改 SearchPhraseWithLanguage 传递独立的位置切片副本，而不是共享的 TermPosting 指针：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
好的，代码修改完成。现在让我更新 todo 状态并运行测试：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -v -count=1 undefined
          
所有 69 个测试全部通过！现在让我运行诊断检查，然后更新文档：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: GetDiagnostics
            
status: success
          
          
诊断没问题。现在让我读取并更新文档：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
现在我来更新文档，补充并发安全保证、倒排索引清理策略、多词项查询排序规则等内容：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
我已经完成了代码修复和部分文档更新，现在继续完成剩余的文档更新工作。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
继续更新文档，首先更新文档索引流程以反映 TOCTOU 修复：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新多词项查询排序规则，在第 5 节 TF-IDF 算法详解中补充：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新并发安全设计部分，增加 TOCTOU 竞态防护说明：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在添加文档删除流程（在第 4 节增加 4.4 小节）：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新性能特征中的 DeleteDocument 时间复杂度，以及注意事项中的删除策略描述：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新注意事项与限制部分的删除策略描述：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
最后更新短语查询匹配算法部分，说明位置切片独立副本的设计：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
文档更新完成。让我最后运行一次测试确保所有修复正常：
























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -v -count=1 undefined
          
所有修复已完成，69 个测试全部通过！以下是修复总结：

---

## ✅ 修复完成总结

### 🐛 问题 1：TOCTOU 竞态失效

**位置**: [engine.go L240-L279](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L240-L279)

**修复**: 将 docID 重复检查与 docs map 写入放在同一个 `Engine.mu` 写锁持锁范围内原子执行，消除了两次持锁之间的竞争窗口。

```go
// 修复前（有漏洞）:
e.mu.Lock()   // 第一次持锁
if _, exists := e.docs[docID]; exists {
    e.mu.Unlock()
    return ErrDuplicateDocID
}
e.mu.Unlock() // 释放锁 —— 窗口期！其他 goroutine 可在此插入同 docID
// ... 构建倒排索引 ...
e.mu.Lock()   // 第二次持锁
e.docs[docID] = doc
e.mu.Unlock()

// 修复后（安全）:
e.mu.Lock()
if _, exists := e.docs[docID]; exists {
    e.mu.Unlock()
    return ErrDuplicateDocID
}
e.docs[docID] = doc  // 检查和写入在同一持锁范围
e.mu.Unlock()
// ... 然后构建倒排索引（索引有独立锁保护）
```

---

### 🐛 问题 2：DeleteDocument 未清理倒排索引

**位置**:
- 新增清理方法: [engine.go L126-L149](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L126-L149) `InvertedIndex.RemoveDocument()`
- Document 新增 Terms 字段: [engine.go L151-L156](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L151-L156)
- DeleteDocument 修复: [engine.go L281-L298](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L281-L298)

**修复**:
1. `Document` 结构新增 `Terms map[string]struct{}` 字段，存储文档的去重词项集合
2. 新增 `InvertedIndex.RemoveDocument(docID, terms)` 方法，遍历文档词项：
   - 从每个词项的 PostingList 中移除该文档的 TermPosting
   - 若移除后 PostingList 为空，则从倒排索引中删除该词项条目以回收空间
3. `DeleteDocument` 在删除 docs 记录后，调用 `RemoveDocument` 同步清理倒排索引

这样保证了 IDF 计算不再统计已删除文档，TF-IDF 评分保持准确。

---

### 🐛 问题 3：无效死代码 docMatchCount

**位置**: [engine.go L336-L361](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L336-L361)

**修复**: 删除了 `SearchWithLanguage` 中定义但从未读取的 `docMatchCount` 变量及其递增语句。

---

### 🐛 问题 4：checkPhraseMatch 隐式共享

**位置**:
- SearchPhraseWithLanguage 修复: [engine.go L391-L470](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L391-L470)
- checkPhraseMatch 独立化: [engine.go L472-L500](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L472-L500)

**修复**:
1. `SearchPhraseWithLanguage` 中候选文档存储从 `map[string][]*TermPosting` 改为 `map[string][][]int`
2. 为每个词项的位置列表创建独立切片副本：`posCopy := make([]int, len(posting.Positions)); copy(posCopy, posting.Positions)`
3. `checkPhraseMatch` 从方法改为独立函数，签名从 `(e *Engine) checkPhraseMatch(postings []*TermPosting)` 改为 `checkPhraseMatch(positionsList [][]int)`
4. 即使短语包含重复词项（如 "go go"），每个位置列表也是独立拷贝，避免共享底层数组导致的副作用

---

### 📝 文档更新

更新了 [fulltext.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fulltext.md)，新增/补充内容：

| 章节 | 更新内容 |
|------|---------|
| 3.2 Document | 新增 `Terms` 字段说明 |
| 3.3 InvertedIndex | 新增 `RemoveDocument` 方法职责说明 |
| 4.1 文档索引流程 | 更新流程图，标注 TOCTOU 防护 |
| 4.4 文档删除与倒排索引清理流程 | 新增完整删除流程图 |
| 5.4 多词项查询排序规则 | 新增 7 条排序规则详述 |
| 6. 短语查询匹配算法 | 新增数据隔离保证说明 |
| 8.3 TOCTOU 竞态防护 | 新增完整章节，含错误/正确模式对比 |
| 8.4 死锁避免 | 补充短语匹配数据隔离说明 |
| 11.1 时间复杂度 | 更新 DeleteDocument 复杂度为 O(U×P) |
| 12. 注意事项与限制 | 更新删除策略、新增匹配隔离和评分一致性说明 |

---

### ✅ 测试结果

```
PASS
ok      solocoder-go/internal/fulltext  0.330s
```

**69 个测试用例全部通过** ✅