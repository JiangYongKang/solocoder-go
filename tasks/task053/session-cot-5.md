好的，我来修复这两个问题。先创建任务列表：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
现在开始修复两个问题：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
让我重写 `appendLocked` 同时修复两个问题：

1. 旧格式文件首次 Append 时自动升级，补写 entriesDigest
2. 检测到重复时立即返回，跳过无意义的哈希计算




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
好的，两个问题都已修复。现在运行测试验证：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
测试失败在 `AppendIdempotent`，说文件 corrupted。让我写个快速调试来看看具体哪里出错了：





        
            
toolName: run_command
            
status: success
          
command: cd internal/datadedup ; go test -run "TestPersistence/AppendIdempotent" -v -count=1 2>&1 | Select-Object -First 100 undefined
          
让我添加调试来看看具体是哪里出了问题：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\datadedup_test.go
          
让我创建一个简单的调试脚本来定位问题：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\debug_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -run TestDebugAppend -v -count=1 undefined
          
找到问题了！第一次 Append 时写入顺序错误——`entriesDigest entry` 写在了新 fp entry 的前面，导致第二次 Append 时覆盖了之前追加的 fp entry。让我修正写入顺序：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在运行调试测试验证：









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -run TestDebugAppend -v -count=1 undefined
          
调试测试通过了！现在删除调试文件并运行完整测试：











        
            
toolName: delete_file
            
status: success
          
deleteFilesSingular
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\debug_test.go
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
所有测试全部通过！让我更新任务状态：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有测试全部通过！

---

## ✅ 修复总结

### 问题 1：旧格式文件 Append 不升级 + 写入顺序错误

**根本原因**：原代码有两个问题——
1. 当文件已有 entriesDigest entry（`edEntryOffset >= 0`）时，写入顺序错误：先写 entriesDigest，再写新 fp entry，导致新 fp entry 出现在 entriesDigest entry 之后
2. 第二次 Append 时，从 entriesDigest entry 位置开始覆盖写，把前一次 Append 的 fp entry 给覆盖了，导致文件结构损坏

**修复后** [appendLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L283-L459)：

两种分支都遵循统一的写入顺序：**新 fp entry → 新 entriesDigest entry → 新 cs entry**

- **`edEntryOffset >= 0`（新格式）**：seek 到 entriesDigest 位置，依次写 fp→ed→cs
- **`edEntryOffset < 0`（旧格式升级）**：seek 到 cs entry 位置，依次写 fp→ed→cs
- 升级后文件格式始终保持：`Header → fp entries... → entriesDigest → cs`

### 问题 2：重复指纹检测时的无意义哈希计算

**修复前**：先写 entriesChecksum，再检查是否重复，重复时 entriesChecksum 里残留了无用数据。

**修复后** [appendLocked 第 357-363 行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L357-L363)：

```go
if entryType == entryTypeFP {
    fpCount++
    if Fingerprint(fpBytes) == fp {
        return nil  // 检测到重复立即返回，不写入 entriesChecksum
    }
    entriesChecksum.Write(entryHeader)
    entriesChecksum.Write(rest)
}
```

- 先检查指纹是否重复，重复则直接 `return nil`
- 不重复才写入 entriesChecksum，避免无意义哈希计算
- 同时提前终止循环，提升性能

### 文件格式（最终）

```
┌─────────────┐
│   Header    │  16字节（magic, version, count, reserved）
├─────────────┤
│  FP entry 1 │
│  FP entry 2 │  ... 所有指纹条目
│    ...      │
├─────────────┤
│    ED entry │  entriesDigest = SHA256(所有FP条目字节)
├─────────────┤
│    CS entry │  fileChecksum = SHA256(header || entriesDigest)
└─────────────┘
```

### 测试结果

```
PASS: TestPersistence (8 subtests)  ← 含 AppendIdempotent
... (全部 55 个测试用例)
ok  solocoder-go/internal/datadedup  0.494s
```