让我分析这三个问题并逐一修复。先创建任务计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我完整地重新读取当前 wal.go，然后进行全面修复：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
现在我要全面重写 wal.go 来解决这三个问题。核心思路：

1. **文件句柄管理**：非活跃段不持久持有 `*os.File`，只有 `activeSeg` 持有 `O_RDWR|O_APPEND`。读取时临时打开 `O_RDONLY`，用完关闭。
2. **并发读取安全**：每次读取都打开独立的只读句柄，各 reader 有独立文件位置，互不干扰。
3. **内存优化**：流式读取，逐条处理，不 `io.ReadAll` 整个段文件。

让我逐步修改：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修改 `New` 函数、`openSegment` 和 `loadExistingSegments`，让非活跃段不持久持有文件句柄：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修改 `scanSegmentOffsets` 为包级函数，使用流式 `bufio.Reader` 逐条读取，不一次性加载整个文件：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修复 `rotateSegment`，让它关闭旧活跃段的文件句柄：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修改 `readSegmentEntries` 和 `recoverSegment`，让它们每次打开独立只读句柄并流式读取：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修改 `recoverSegment`，同样使用流式读取并跟踪位置信息以生成警告：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在修改 `Close` 和 `Sync` 方法，适配非活跃段 `file = nil` 的情况：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\wal\wal.go
          
changes: undefined
          
现在让我更新任务并运行测试验证修复：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/wal/ undefined
          
编译通过，现在运行测试：









        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/wal/ -v -count=1 undefined
          
所有 42 个测试全部通过！现在更新文档，补充段切分文件句柄管理策略和并发读取安全保证：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先读取当前文档，然后进行更新：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
现在更新文档。首先更新 segment 结构体说明：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
现在更新段创建与加载流程图：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
更新段写入与切分流程图：



















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
更新段读取（ReadFrom）流程图：






















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
更新段恢复（RecoverFrom）流程图：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
更新关闭流程图：
























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
现在在损坏恢复策略之后添加新章节，详细说明文件句柄管理和并发读取安全保证：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\wal.md
          
changes: undefined
          
现在让我完成最后的任务状态更新：


























        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复，以下是完成的工作总结：

---

## 修复内容

### 问题 1：文件句柄泄漏 — [wal.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal.go#L435-L446)

**`rotateSegment` 三步保障**：
```go
func (w *WAL) rotateSegment() error {
    if err := w.activeSeg.file.Sync(); err != nil { ... }  // ① 刷盘
    if err := w.activeSeg.file.Close(); err != nil { ... } // ② 关闭旧句柄
    w.activeSeg.file = nil                                 // ③ 标记释放
    return w.createSegment(w.activeSeg.id + 1)
}
```

- 仅 `activeSeg` 持久持有 **1 个** `O_RDWR|O_APPEND` 句柄，非活跃段 `file = nil`
- `openSegment` 加载时临时 `os.Open(O_RDONLY)`，`defer f.Close()` 立即释放
- `readSegmentEntries` / `recoverSegment` 按需 `os.Open`，`defer f.Close()` 即开即关

### 问题 2：并发读取竞争 — [readSegmentEntries](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal.go#L500-L561)、[recoverSegment](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal.go#L622-L704)

每次读取都 `os.Open(seg.path)` 获取**独立文件描述符**，每个 reader 有独立的文件偏移量：

```
Goroutine A: os.Open(seg1) → fd=5, offset=0
Goroutine B: os.Open(seg1) → fd=6, offset=0  （互不干扰）
```

配合 `sync.RWMutex` 读锁保护元数据，整个读取链路完全并发安全。

### 问题 3：内存浪费 — [scanSegmentOffsets](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/wal/wal.go#L293-L361)

三个函数全部改为 `bufio.NewReaderSize(f, 64*1024)` **流式缓冲读取**：
- 固定 64KB 缓冲，不一次性加载整段
- 仅分配 19B 头部缓冲，Magic 匹配后才按需分配 DataLen 字节
- 单条记录处理完立即被 GC 回收

**内存对比**：

| 场景 | 旧方案（io.ReadAll） | 新方案（流式读取） |
|------|-------------------|----------------|
| 64MB 段扫描偏移量 | ~64 MB | < 100 KB |
| 10 段并发读取 | ~640 MB | ~640 KB |

---

## 测试结果

```
PASS
ok      solocoder-go/internal/wal       2.480s
```
**42 个测试全部通过** ✅

---

## 文档更新

[wal.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/wal.md) 新增三大章节并同步更新所有流程图：

1. **文件句柄管理策略**：设计原则、各场景句柄生命周期表、关键实现要点、FD 上限保证
2. **并发读取安全保证**：问题背景、独立句柄解决方案、并发层级安全总结表、性能考虑
3. **内存使用优化**：旧方案问题、流式缓冲读取方案、新旧方案内存占用对比表