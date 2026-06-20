我来修复这些问题。首先让我查看当前的代码和文档状态。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\abtest.md
          
我已经了解了问题所在。让我逐一修复：

1. **GetExperiment 和 ListExperiments 返回内部指针导致数据竞争**
2. **AddExperiment 存储调用方指针也有类似问题**
3. **ErrInvalidBucketNumber 未使用需要移除**
4. **文档检查并补全**

让我开始修复：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复代码中的问题。首先移除未使用的错误变量，并修复 GetExperiment 返回副本的问题：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest.go
          
changes: undefined
          
现在修复 `GetExperiment` 方法返回副本：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复 `ListExperiments` 方法返回副本：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在修复 `AddExperiment` 存储副本而不是调用方指针：















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest.go
          
changes: undefined
          
现在添加测试来验证返回副本的正确性，防止外部修改影响内部状态：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我读取测试文件，找到合适的位置添加新的测试：
















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest_test.go
          
让我查看 `TestGetExperiment_Success` 的完整内容和后面的测试：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest_test.go
          
现在添加新的测试来验证返回副本的并发安全性：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\abtest\abtest_test.go
          
changes: undefined
          
现在让我更新 todo 并检查文档。我看到文档中有 `ErrInvalidBucketNumber` 需要从错误列表中移除，并确保内容完整：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我更新文档，补充防御性拷贝的说明，并确保内容完整。首先更新并发安全设计章节：


















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\abtest.md
          
现在更新并发安全设计章节，补充防御性拷贝的说明：





















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\abtest.md
          
changes: undefined
          
现在更新 todo 并运行带 race 检测的测试：























        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
运行带 race 检测的测试：

























        
            
toolName: run_command
            
status: success
          
command: go test -race ./internal/abtest/ -v -count=1 undefined
          
Windows 386 架构不支持 -race 检测。让我检查一下 Go 的架构信息，然后运行普通测试确保全部通过：

























        
            
toolName: run_command
            
status: success
          
command: go version undefined
          
环境是 Windows 386 架构，不支持 -race 检测。让我运行普通测试确保通过：



























        
            
toolName: run_command
            
status: success
          
command: go test ./internal/abtest/ -v -count=1 undefined
          
所有 72 个测试全部通过！让我更新任务列表并提供总结：



























        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
---

## A/B 测试模块问题修复完成 ✅

### 🔧 修复的问题

**1. 并发安全问题 - GetExperiment 返回内部指针**
- **修改位置**：[GetExperiment](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/abtest/abtest.go#L158-L177)
- **问题**：直接返回内部 `*Experiment` 指针，调用方可在锁释放后修改内部状态
- **修复**：返回 `Experiment` 结构体的副本，包含所有字段的复制

**2. 并发安全问题 - ListExperiments 返回内部指针**
- **修改位置**：[ListExperiments](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/abtest/abtest.go#L179-L194)
- **问题**：返回的切片包含内部 `*Experiment` 指针
- **修复**：每个元素都创建独立副本后返回

**3. 并发安全问题 - AddExperiment 存储调用方指针**
- **修改位置**：[AddExperiment](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/abtest/abtest.go#L116-L126)
- **问题**：直接存储调用方传入的 `*Experiment` 指针，调用方后续修改会影响内部状态
- **修复**：创建副本后存储到内部 map

**4. 移除未使用的 ErrInvalidBucketNumber**
- **修改位置**：[错误变量定义](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/abtest/abtest.go#L10-L19)
- **问题**：该错误变量定义了但从未被使用，会误导外部调用方

**5. 文档补全**
- **修改位置**：[docs/abtest.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/abtest.md)
- **新增**：4.6 节"防御性拷贝设计"，详细说明了所有防御性拷贝策略
- **确认**："指标采集机制"（4.4 节）和"使用示例"（第 5 节）内容完整

### 🧪 新增测试

| 测试用例 | 验证目标 |
|----------|----------|
| `TestGetExperiment_ReturnsCopy` | 验证 GetExperiment 返回副本，外部修改不影响内部 |
| `TestListExperiments_ReturnsCopies` | 验证 ListExperiments 返回副本切片 |
| `TestAddExperiment_StoresCopy` | 验证 AddExperiment 存储副本，调用方后续修改不影响 |
| `TestConcurrent_ReadAndModifyExperimentPointer` | 验证多协程并发场景下无数据竞争 |

### ✅ 测试结果

```
PASS
ok      solocoder-go/internal/abtest    0.274s
```

**72 个测试全部通过**

### ⚠️ 关于 -race 检测

当前环境是 `go1.26.4 windows/386`，Windows 386 架构不支持 `-race` 检测。如需运行 race 检测，请使用 64 位版本的 Go（windows/amd64）。

但代码已通过以下方式确保无数据竞争：
1. 所有公共方法使用 `sync.RWMutex` 正确保护
2. 所有返回指针的方法均返回副本，切断外部修改路径
3. 新增的 `TestConcurrent_ReadAndModifyExperimentPointer` 测试模拟了并发读写场景
4. `GetExperimentMetrics` 原已实现深拷贝返回