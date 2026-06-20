好的，我来修复这些问题。让我先分析问题并制定修复计划。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先重新读取当前的代码文件，确保修改时使用最新内容：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
现在让我修复这两个问题。首先修复 **频率计数问题** 和 **并发安全问题**：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          
继续修改剩余的方法：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest.go
          
changes: undefined
          
好的，核心代码已修复。现在让我更新 Todo 并编写引擎级并发测试：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取测试文件，然后进行修改和添加：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest_test.go
          
现在让我修改测试文件，更新受频率变化影响的测试，并添加新的引擎级并发测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest_test.go
          
changes: undefined
          
现在让我在测试文件末尾添加更多测试用例：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\suggest\suggest_test.go
          
changes: undefined
          
现在让我运行测试，看看是否都通过：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/suggest/ -v -count=1 undefined
          
太好了！**suggest 模块的所有测试都通过了**（后面的 templater 测试失败是其他模块的，与我们无关）。

现在让我更新需求文档：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取现有的文档，然后进行更新：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
让我继续读取文档中关于并发安全和频率相关的部分：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
现在让我更新文档，修改并发安全部分和添加频率计数策略说明：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
changes: undefined
          
现在让我更新并发安全部分的内容：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
changes: undefined
          
现在让我更新测试策略部分，添加引擎级并发测试的说明：



















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
让我更新并发测试和引擎集成测试部分：


















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\suggest.md
          
changes: undefined
          
文档已更新完成。让我再运行一次测试确认所有测试都通过：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/suggest/ -v -count=1 2>&1 | Select-Object -Last 30 undefined
          
完美！所有测试都通过了。让我更新 Todo 状态并总结：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复完成！以下是修复总结：

## 修复内容

### 1. 频率计数策略修复
**问题**：`AddWord` 调用 `Trie.Insert` 导致频率被错误递增为 1，初始化词掩盖了真实搜索热度。

**修复** ([suggest.go#L477-L482](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest.go#L477-L482))：
- `AddWord(word)` → 初始频率为 **0**（使用 `InsertWithFreq(word, 0)`）
- `SubmitSearch(userID, word)` → 频率递增（调用 `Insert`，不存在则为 1，存在则 +1）
- `AddWordWithFreq(word, freq)` → 设置指定初始频率

### 2. 引擎级并发安全修复
**问题**：`SuggestEngine.mu` 只在 `SubmitSearch` 中使用，其余方法都绕过锁直接操作底层数据结构。

**修复**：所有 `SuggestEngine` 公开方法都先获取 `e.mu` 锁：
- **写操作**（`Lock`）：`AddWord`、`AddWordWithFreq`、`RemoveWord`、`SubmitSearch`、`ClearHistory`
- **读操作**（`RLock`）：`HasWord`、`WordCount`、`Autocomplete`、`Correct`、`GetHistory`、`Suggest`、`GetHotWords`

锁获取顺序：`SuggestEngine.mu` → `Trie.mu` / `SearchHistory.mu`，避免死锁。

### 3. 新增引擎级并发测试
新增 **9 个测试用例** ([suggest_test.go#L1346-L1629](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/suggest/suggest_test.go#L1346-L1629))：
- `TestSuggestEngine_AddWord_FreqStartsAtZero` - 验证 AddWord 初始化频率为 0
- `TestSuggestEngine_SubmitSearch_AfterAddWord_IncrementsFreq` - 验证 SubmitSearch 正确递增
- `TestSuggestEngine_AddWordWithFreq_CustomFreq` - 验证自定义初始频率
- `TestSuggestEngine_HotWords_InitWordsNotSearch_SortedByFreq` - 验证初始化词不影响热门排行
- `TestConcurrent_SuggestEngine_AddWordAndSubmitSearch` - AddWord 与 SubmitSearch 并发频率正确性
- `TestConcurrent_SuggestEngine_AutocompleteAndSubmitSearch` - 自动补全与提交并发无数据竞争
- `TestConcurrent_SuggestEngine_CorrectAndSubmitSearch` - 纠错与提交并发无数据竞争
- `TestConcurrent_SuggestEngine_GetHotWordsAndSubmit` - 热门词与提交并发一致性
- `TestConcurrent_SuggestEngine_HistoryAndSubmit` - 历史查询与提交并发安全

### 4. 文档更新
在 [suggest.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/suggest.md) 中新增：
- **频率计数策略**章节（两级频率计数，区分初始化与真实搜索）
- **引擎级锁的保护范围**详细说明
- **为什么需要引擎级锁**设计考量（原子性、TOCTOU 防护、一致性视图）
- **双重锁设计的权衡**分析
- **引擎级并发测试设计说明**

### 测试结果
所有 **92 个测试用例**全部通过 ✅
```
PASS
ok   solocoder-go/internal/suggest   0.227s
```