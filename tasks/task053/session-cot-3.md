好的，我来修复这三个残留问题。让我先创建任务列表，然后逐一修复。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我先读取当前文件确认最新状态：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
好的，让我开始修复这三个问题。首先修复 persistence.go 的两个问题（增量校验和 + 重复指纹去重）：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在让我重写 `appendLocked` 方法，同时解决问题 1（避免第二遍磁盘读取）和问题 3（重复指纹去重）：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
好的，persistence.go 的两个问题已修复。现在修复 chunker.go 的滚动哈希窗口大小自适应问题：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复 chunker.go，让滚动哈希窗口大小根据 minChunkSize 自适应：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
changes: undefined
          
好的，chunker.go 也修复了。现在运行测试验证：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
太好了！所有测试都通过了。让我更新任务状态：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有三个残留问题已修复完成，所有测试通过！

---

## ✅ 修复总结

### 问题 1：appendLocked 第二遍全量重读磁盘

**修复前**：第一遍扫描读取所有 entry 后，第二遍又用 4KB 缓冲区从磁盘重新读取所有 entry 来重新计算 checksum，磁盘读取量是索引大小的 2 倍。

**修复后** [appendLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L245-L404)：
- 第一遍扫描时，将每个指纹条目的原始字节追加到 `entryBuffer` 内存缓冲区
- 计算新 checksum 时，直接使用内存中的 `entryBuffer` + 新条目 + 新 header
- 彻底消除第二遍磁盘读取，磁盘 I/O 从 2x 降为 1x
- 内存中只保存原始字节流，不构建 map 结构，内存开销最小化

### 问题 2：滚动哈希窗口大小硬编码

**修复前**：窗口大小固定为 48 字节，当 `minChunkSize < 48` 时，达到最小块大小后滚动哈希窗口还没填满，CDC 边界检测延迟生效，小分块场景下算法效果大打折扣。

**修复后** [chunker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/chunker.go#L191-L214)：
- 新增 `calcRollingWindowSize(minChunkSize)` 函数：
  - `minChunkSize >= 48`：使用默认 48 字节窗口
  - `minChunkSize < 48`：窗口大小 = `minChunkSize / 2`，最小 4 字节
- 新增 `calcTargetMask(minChunkSize, windowSize)` 函数：
  - `minChunkSize < 256`：target bits = 8（更容易找到边界，避免块过大）
  - `minChunkSize < 1024`：target bits = 10
  - 其他：target bits = 13（默认）
- 确保小分块场景下 CDC 算法仍然有效

### 问题 3：Append 重复指纹导致文件不一致

**修复前**：对同一指纹重复调用 `Append` 会在文件中产生重复条目，`header.Count` 与实际条目数不一致，破坏文件完整性。

**修复后** [appendLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L319-L334)：
- 在第一遍扫描时，每读到一个指纹条目就与待追加的指纹比较
- 如果指纹已存在，**直接返回 nil**（幂等操作，不报错也不修改文件）
- 保证 Append 方法的幂等性，不会产生重复条目

---

### 测试结果

```
PASS: TestPersistence (7 subtests) - Append 幂等性正确
PASS: TestContentBasedChunker (3 subtests) - 小分块 CDC 正常工作
... (全部 50+ 测试用例)
ok  solocoder-go/internal/datadedup  0.514s
```