我来修复这些问题。首先让我重新审视当前代码和文档的状态。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
好的，我来逐一修复这些问题。首先修复核心代码。

### 修复 1：Get 方法使用读锁 + 原子访问计数
### 修复 2：evictOne 全部受保护时不驱逐
### 修复 3：PreloadOnStart 自动触发


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在修改 CacheEntry 结构体，使用原子类型支持读锁下的并发更新：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在重写 Get 方法，使用读锁 + 原子操作 + 双重检查的惰性删除：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新其他使用 IsHot 和 AccessCount 的方法。首先是 PutWithTTL：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新 HotCount、IsExpired、MarkHot、UnmarkHot、IsHot 等方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新 Preload、evictOne、CleanupExpired 和 GetEntry 方法：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新 evictOne 方法，全部受保护时不驱逐：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在更新 CleanupExpired 和 GetEntry 方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
等等，我需要检查一下文件是否有重复的 GetEntry 方法：




        
            
toolName: search_by_regex
            
status: success
          
query: func.*GetEntry
          
有两个 GetEntry 方法，我需要删除旧的那个。让我查看并修复：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
删除旧的 GetEntry 方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
现在实现 PreloadOnStart 自动触发功能，修改 SetPreloadLoader 方法：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid.go
          
changes: undefined
          
好的，核心代码修复完成。现在更新 todo 并运行测试看看情况：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先运行测试看看有哪些失败：







        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cacheinvalid/ -v 2>&1 | Select-Object -First 100 undefined
          
测试文件有编译错误，因为 IsHot 和 AccessCount 现在是原子类型了。让我修复测试文件：









        
            
toolName: search_by_regex
            
status: success
          
query: entry\.IsHot|entry\.AccessCount|\.IsHot\s*=|\.AccessCount\s*[!=]
          
让我查看这些测试的上下文：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
修复 TestCacheEntryFields 测试中的原子类型访问：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
changes: undefined
          
现在再次运行测试看看：













        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cacheinvalid/ -v 2>&1 | Select-Object -First 150 undefined
          
太棒了！所有原有测试都通过了。现在添加新的测试用例：


1. 并发读性能测试
2. 全部受保护时驱逐失败测试
3. PreloadOnStart 自动触发测试






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
让我看看文件末尾：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
现在在文件末尾添加新的测试用例：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\cacheinvalid\cacheinvalid_test.go
          
changes: undefined
          
现在运行所有测试：


























        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cacheinvalid/ -v undefined
          
太棒了！所有 **68 个测试** 全部通过！并发读性能达到 **~1000 万次/秒**，充分证明了读锁的并发优势。


现在更新 todo 并开始补全文档：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先查看当前文档的状态：




























        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我继续读取文档看看生命周期和使用示例部分：






























        
            
toolName: search_by_regex
            
status: success
          
query: ^## 
          
文档结构看起来完整。让我查看一下生命周期图和使用示例部分，然后补充新增功能的说明：
































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我查看一下性能与并发、容量管理等章节，然后进行补充更新：







































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
好的，文档结构完整，但需要更新以反映修复后的实现。让我逐步更新文档：

1. 更新 CacheEntry 结构体说明（原子类型）
2. 更新淘汰算法（全部受保护时返回错误）
3. 补充并发读取策略
4. 补充 PreloadOnStart 启动预加载机制
5. 新增错误定义

让我先更新 CacheEntry 结构体和容量淘汰部分：


































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在更新容量淘汰算法部分（全部受保护时不驱逐，返回错误）：









































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在更新缓存预加载机制部分，添加 PreloadOnStart 自动触发说明：

















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
在预加载机制部分添加 PreloadOnStart 自动触发说明：



















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在更新性能与并发部分，详细说明并发读取策略：
















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我查看第 12 节（性能与并发）的位置：



















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
现在更新错误定义和性能与并发部分：



















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在更新注意事项与限制部分，并在使用示例中加入 PreloadOnStart 的示例：

先看看使用示例部分的末尾：












































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
在使用示例中添加 PreloadOnStart 和容量耗尽处理的示例：















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
现在更新注意事项部分，加入受保护条目驱逐的注意事项：





















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我找到注意事项部分：




















































        
            
toolName: search_by_regex
            
status: success
          
query: 注意事项与限制
          
让我读取并更新注意事项部分：





















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我找到第 12.3 节的注意事项：


























































        
            
toolName: search_by_regex
            
status: success
          
query: 注意事项与限制|纯内存存储
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我找一下第 12 节的位置：


















































        
            
toolName: search_by_regex
            
status: success
          
query: ^### 12\.
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
让我看看错误定义后面的内容：






















































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
现在更新注意事项与限制部分：


















































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cacheinvalid.md
          
changes: undefined
          
文档更新完成！让我更新 todo 列表并做最终确认：


















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
最后再运行一次测试，确保所有测试通过：


















































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/cacheinvalid/ -v undefined
          
完美！所有 **68 个测试** 全部通过！让我更新 todo 并给出最终总结：





















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
## 修复完成总结

所有 68 个测试全部通过！以下是修复的详细内容：

### 修复 1：Get 方法并发读优化

**问题**: Get 方法使用全写锁，并发读能力未被利用

**修复**: 采用 **读锁优先 + 原子操作 + 双重检查 设计：
- `CacheEntry` 中的 `IsHot` 改为 `atomic.Bool`，`AccessCount` 改为 `atomic.Int64`
- 大多数读操作只需读锁，访问计数和热点标记通过原子操作更新
- 惰性删除时才释放读锁、获取写锁、双重检查后删除

**性能提升**: 并发读吞吐量可达 **~1000 万次/秒**（50 个 goroutine，5 万次读取仅需 ~5ms）

**相关文件**:
- [cacheinvalid.go Get 方法](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid.go#L177-L213)
- [cacheinvalid_test.go TestConcurrentReadPerformance](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid_test.go#L1295-L1328)

---

### 修复 2：受保护条目驱逐边界

**问题**: 全部条目都是热点/预加载时，evictOne 会任意删除一个受保护条目

**修复**:
- `evictOne()` 返回 `bool`，全部受保护时返回 `false`，不删除任何条目
- `PutWithTTL` 在无法驱逐时返回 `ErrCapacityExhausted` 错误
- 严格遵守"热点和预加载条目不受容量驱逐"的承诺

**相关文件**:
- [cacheinvalid.go evictOne 方法](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid.go#L493-L513)
- [cacheinvalid.go ErrCapacityExhausted](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid.go#L17)
- [cacheinvalid_test.go TestAllProtectedEntriesEvictionFails](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid_test.go#L1143-L1172)

---

### 修复 3：PreloadOnStart 自动预加载

**问题**: Config 中定义了 PreloadOnStart 字段，但无代码自动触发逻辑

**修复**:
- 在 `SetPreloadLoader` 方法中检查 `PreloadOnStart` 配置
- 当 `PreloadOnStart = true` 时，设置加载器后立即自动执行预加载
- 预加载错误通过 `SetPreloadLoader` 返回值传递

**相关文件**:
- [cacheinvalid.go SetPreloadLoader 方法](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid.go#L397-L408)
- [cacheinvalid_test.go TestPreloadOnStartAutoTrigger](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cacheinvalid/cacheinvalid_test.go#L1207-L1244)

---

### 修复 4：文档补全

在 [cacheinvalid.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cacheinvalid.md) 中补充了：

1. **CacheEntry 原子类型说明** - 第 3.2 节
2. **并发读取策略** - 第 12.2 节（含详细流程图）
3. **受保护条目驱逐边界** - 第 9.2、9.3 节
4. **启动自动预加载机制** - 第 7.5 节
5. **新增错误定义** - 第 11 节（ErrCapacityExhausted）
6. **新增使用示例** - 10.10 启动自动预加载、10.11 容量耗尽错误处理
7. **注意事项更新** - 第 12.3 节（新增 2 条注意事项

---

### 新增测试用例（共 10 个）

| 测试用例 | 验证内容 |
|----------|---------|
| `TestAllProtectedEntriesEvictionFails` | 全部热点条目时驱逐失败 |
| `TestAllPreloadedEntriesEvictionFails` | 全部预加载条目时驱逐失败 |
| `TestPreloadOnStartAutoTrigger` | PreloadOnStart=true 时自动触发 |
| `TestPreloadOnStartFalseDoesNotAutoTrigger` | PreloadOnStart=false 时不自动触发 |
| `TestPreloadOnStartWithLoaderError` | 启动预加载错误传递 |
| `TestConcurrentReadPerformance` | 并发读性能验证 |
| `TestConcurrentReadWithWrite` | 并发读写混合验证 |
| `TestCapacityExhaustedError` | 容量耗尽错误验证 |
| `TestMixedProtectionEviction` | 混合保护场景驱逐 |
| `TestGetEntryReturnsCopy` | GetEntry 返回副本验证 |

运行测试：
```bash
go test ./internal/cacheinvalid/ -v
```