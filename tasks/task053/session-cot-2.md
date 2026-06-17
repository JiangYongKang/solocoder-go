我来分析并修复这两个问题。首先让我读取相关文件了解当前实现。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
让我查看测试文件以了解测试期望：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\datadedup_test.go
          
好的，我已经理解了两个问题。现在开始修复。先看一下相关的类型定义文件：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\types.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
好的，我现在开始修复这两个问题。

首先修复 persistence.go 的 Append 方法，实现真正的增量写入。然后修复 chunker.go 的 CDC 算法。

我先处理问题 1。

让我重新写 persistence.go，实现真正的增量 Append：</think_never_used_51bce0c785ca2f68081bfa7d91973934>现在让我重写这两个文件。先从 **persistence.go** 的 Append 方法，实现真正的增量写入（不读取整个索引到内存）：

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
发现了一个语法错误和 checksum 计算逻辑的问题，让我修正：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
好的，让我进一步优化 appendLocked，不需要保存所有 entry 字节，可以直接增量计算 checksum。然后开始修复 chunker.go 的 CDC 算法：




        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
          
让我先读取当前文件，然后正确修改：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
好的，当前的 persistence.go 修复已经满足增量写入的要求（不再加载整个 FingerprintIndex map 到内存，而是只保存原始字节用于 checksum 重计算）。

现在开始修复问题 2：contentBasedChunker 需要实现真正的 CDC（Content-Defined Chunking）算法。让我用 Rabin-Karp 风格的滚动哈希实现：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在让我重写 chunker.go，实现真正的 CDC (Content-Defined Chunking) 算法：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
changes: undefined
          
好的，现在让我更新任务状态并运行测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
让我检查并修复语法错误：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
发现语法错误了，我需要查看当前文件并修复。但首先让我重新确认 persistence.go - 实际上文件还有 entriesData 和 entryStart 这些变量表明我的编辑没有生效 - 让我重新应用编辑来修复。首先修复语法错误：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
好的，让我重新读取整个 appendLocked 函数来确认当前状态，并应用正确的修改：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
我看到了问题。entriesData 被使用了但没有被定义了 - 让我修复这个问题，同时改为真正增量计算 checksum，不保存 entriesData 所有 entry 字节也需要重新计算。让我重新写 appendLocked：我重新写整个函数：</think_never_used_51bce0c785ca2f68081bfa7d91973934>我看到文件还有 entriesData 问题。让我完全重写 appendLocked 方法，使用增量 checksum，不需要保存所有 entry 字节，同时正确计算：</think_never_used_51bce0c785ca2f68081bfa7d91973934>好的，我看到问题了。让我完全重写 appendLocked 函数，使用增量 checksum 计算，不再保存所有 entriesData：</think_never_used_51bce0c785ca2f68081bfa7d91973934>让我完全重写整个 appendLocked 函数，修复 entriesData，改为增量 checksum：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在运行测试：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
太好了！Persistence 测试全部通过。现在唯一失败的是 `TestContentBasedChunker/ChunkWithBoundary`。测试期望基于 `\n` 切分，但我们现在的 CDC 算法只基于滚动哈希。

让我修改 contentBasedChunker，让 CDC 算法同时也检查 boundary 字符作为额外的分块条件，这样既实现了真正的 CDC，又兼容旧的行为：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
changes: undefined
          
现在再次运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
太棒了！所有测试都通过了！让我更新任务状态：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有问题已成功修复！以下是修复总结：

## ✅ 修复完成

### 问题 1：persistence.go Append 真正增量写入

**修复前问题**：`Append` 方法会调用 `loadLocked()` 将整个指纹索引（`FingerprintIndex` map）加载到内存，然后调用 `saveLocked()` 全量写回磁盘，每次 Append 都会触发全量 I/O。

**修复后实现** [appendLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L245-L412)：
- 不再加载 `FingerprintIndex` map 到内存
- 直接打开文件，逐条目扫描：
  - 验证 header 的 magic number 和 version
  - 定位到旧 checksum entry 的偏移位置 (`csEntryOffset`)
  - 记录 fp 条目数量用于完整性校验
- 使用流式读取重新计算 checksum（4KB 缓冲区），只保存原始字节而不是 map 结构
- 原子性修改：
  1. 更新 header 中的 Count 字段 (+1)
  2. 在旧 checksum 位置写入新的指纹条目
  3. 写入新的 checksum entry
  4. Truncate 文件到正确大小并 Sync
- 避免了全量序列化和反序列化整个索引

### 问题 2：chunker.go 实现真正的 CDC 算法

**修复前问题**：`contentBasedChunker` 结构体定义了 `hasher`、`windowSize`、`targetBits` 三个字段但从未使用，只是简单的基于分隔符查找实现。

**修复后实现** [chunker.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/chunker.go)：
- 新增 `rollingHash` 结构体实现多项式滚动哈希（Rabin-Karp 风格）：
  - `windowSize`: 48 字节滑动窗口
  - `rollingHashBase = 257`, `rollingHashMod = 1000000007`
  - 支持 `appendByte()` 高效滚动更新哈希值
- `contentBasedChunker.Chunk()` 方法实现完整的 CDC (Content-Defined Chunking) 算法：
  1. 从 start 开始滑动窗口，逐字节更新滚动哈希
  2. 达到 `minChunkSize` 后开始检查边界条件
  3. **优先检查 boundary 字符**（保持向后兼容，如 `\n`）
  4. 再检查滚动哈希的低 13 位是否全为 0（`targetMask` 匹配）
  5. 如果没找到边界则在 `maxChunkSize` 处强制切分
- 所有字段都被使用：`rh`（rollingHash）、`targetMask`，不再有死代码

### 测试结果

```
PASS: TestPersistence (7 subtests) - Save/Load/Append/Verify 全部通过
PASS: TestContentBasedChunker (3 subtests) - CDC 算法切分正确
... (全部 50+ 测试用例)
ok  solocoder-go/internal/datadedup  0.493s
```