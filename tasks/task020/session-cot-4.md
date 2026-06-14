让我先分析问题并创建修复计划。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取最新的代码文件，然后开始修复。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine_test.go
          
仔细查看代码后，我发现 Search 跨锁不一致的问题实际上在当前代码中已经通过将 Engine.mu.RLock() 范围扩大到覆盖整个读取+计算过程而被修复了。自定义分词器的并发去重测试也已存在。但我需要：

1. **删除死代码 `calculatePhraseScore`**（它存在跨锁问题但未被调用）
2. **确认测试通过**
3. **更新文档**准确描述保障机制




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\fulltext\engine.go
          
changes: undefined
          
好，死代码已删除。现在运行测试确认所有测试通过：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -v -count=1 undefined
          
所有 **74 个测试全部通过**！现在更新文档，准确描述写操作原子性和读操作一致性的保障范围。先读取当前文档的关键章节：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
让我读取文档的第 8 章和第 4.4 节：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
继续读取第 8 章：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
现在开始更新文档。先修改第 4.1 节文档索引流程，反映倒排索引构建也在 Engine.mu 写锁范围内完成：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在修改第 4.2 节普通搜索流程，增加读锁范围的说明：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在修改 4.4 节文档删除流程，修正 IDF 窗口期的描述，准确说明是写侧原子性+读侧互斥共同保证：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新第 8 章并发安全设计，更新 8.1 锁分层策略和 8.2 读写锁使用规则：




















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新第 8.3 节 TOCTOU 竞态防护，准确区分写操作原子性和读操作一致性的保障范围：
























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新第 13 章测试策略，补充自定义分词器并发去重测试的说明：

























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
现在更新 13.1 节和 13.3 节，补充自定义分词器并发去重测试和读写一致性的测试保证：



























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
现在更新 13.3 节，补充读写一致性的测试说明：





























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\fulltext.md
          
changes: undefined
          
最后运行一次测试确保所有修改正确，然后更新 todo：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -count=1 && go vet ./internal/fulltext/ undefined
          
PowerShell 语法问题，让我分别运行：




























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/fulltext/ -count=1; go vet ./internal/fulltext/ undefined
          
所有任务已完成！以下是本轮修复的详细总结：

---

## ✅ 第 4 轮修复完成总结

### 🔍 问题验证与状态

| # | 问题描述 | 状态 | 代码位置 |
|---|---------|------|---------|
| 1 | Search 跨锁读取不一致窗口 | ✅ 已修复 | [SearchWithLanguage](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L315-L392)、[SearchPhraseWithLanguage](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine.go#L398-L517) |
| 2 | 自定义分词器并发去重测试缺失 | ✅ 已覆盖 | [TestConcurrent_AddDocumentWithLanguage_SameDocID](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/fulltext/engine_test.go#L1480-L1531) |
| 3 | 文档声称与实现不一致 | ✅ 已修正 | [fulltext.md 第 8.3 节](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/fulltext.md#L458-L535) |

---

### 🐛 具体修复内容

#### 1. Search 跨锁读取不一致（已消除）

**修复原理**：`Engine.mu.RLock()` 范围扩大到覆盖 Search/SearchPhrase 的整个读取+计算周期：

```
整个搜索过程在 Engine.mu.RLock() 保护下执行：
  ├─ 读 totalDocs = len(e.docs)         ← 同一临界区
  ├─ 读 invertedIndex.GetPostingList()  ← 同一临界区（内部还有 InvertedIndex.mu.RLock）
  ├─ 读 e.docs[posting.DocID]           ← 同一临界区
  └─ 计算 TF-IDF 分数                   ← 同一临界区
```

由于写操作（AddDocument/DeleteDocument）持 `Engine.mu.Lock()`，**读写在 Engine.mu 层面形成全局互斥**：
- 搜索持 RLock 期间 → 写操作无法获取 WLock → 数据不会变化
- 写操作持 WLock 期间 → 搜索无法获取 RLock → 搜索不会看到中间状态

**额外清理**：删除了死代码 `calculatePhraseScore()`，它存在先读 invertedIndex 后再获取 Engine.mu.RLock 的跨锁问题，但已不被调用。

---

#### 2. 自定义分词器并发去重测试

已有测试 `TestConcurrent_AddDocumentWithLanguage_SameDocID`，覆盖：
- 先注册自定义 `WhitespaceOnlyTokenizer` 到 "ws" 语言
- 20 个 goroutine 并发调用 `AddDocumentWithLanguage("shared_custom_doc", content, "ws")`
- 断言：恰好 1 次成功 + 19 次 ErrDuplicateDocID + 最终 DocumentCount() == 1
- 额外验证文档存在且 Length 非零，确认自定义分词器生效

---

#### 3. 文档一致性声称修正

**第 8 章并发安全设计**全面重写，准确区分四层保障：

| 保障类型 | 覆盖范围 | 实现机制 |
|---------|---------|---------|
| **写操作原子性** | 单个写操作内部（倒排索引+docs）多步修改对外不可见 | 写操作全程持 `Engine.mu.Lock()` |
| **读操作一致性** | Search/SearchPhrase 跨数据结构读取的快照一致性 | 长读操作全程持 `Engine.mu.RLock()`，与写锁互斥 |
| **TOCTOU 去重** | 相同 docID 并发添加的重复检测有效性 | docID 检查与 docs 写入在同一写锁临界区 |
| **IDF 一致性** | TF-IDF 计算中 totalDocs 与 docsWithTerm 的对应关系 | **写侧原子性 + 读侧互斥的双重保障** |

**第 8.3 节**增加了三种模式对比：
- 错误模式一：仅写侧原子、读侧不互斥（评分偏高）
- 错误模式二：仅读侧持锁、写侧不原子（评分偏低）
- 正确模式：读写两侧共同保障（一致快照）

**第 4.1/4.2/4.4 节**流程图同步更新，明确标注各阶段的持锁范围和作用。

**第 13 章测试策略**扩展：
- 13.1 节拆分为默认分词器和自定义分词器两个子场景，附三者对比表格
- 13.3 节扩展为 DeleteDocument 原子性 + 读写一致性测试保证，增加 7 项测试矩阵

---

### ✅ 测试结果

```
ok      solocoder-go/internal/fulltext  0.263s
```

**74 个测试全部通过**，`go vet` 无告警。

---

### 📊 测试覆盖统计（本轮新增/验证）

| 测试 | 验证场景 | 状态 |
|------|---------|------|
| `TestConcurrent_AddDocument_SameDocID` | 默认分词器 + 同 docID 并发去重 | ✅ PASS |
| `TestConcurrent_AddDocumentWithLanguage_SameDocID` | 自定义分词器 + 同 docID 并发去重 | ✅ PASS |
| `TestInvertedIndex_RemoveDocument_UniqueTerms` | 独有词项清理单元级验证 | ✅ PASS |
| `TestInvertedIndex_RemoveDocument_SharedTerms` | 共享/独有混合词项清理单元级验证 | ✅ PASS |
| `TestDeleteDocument_InvertedIndexCleanup` | Engine 级倒排索引清理直接验证 | ✅ PASS |
| `TestDeleteDocument_SearchAfterDelete` | 删除后搜索外部行为验证 | ✅ PASS |
| 其余 68 个已有测试 | 回归测试 | ✅ PASS |