toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
现在读取测试文件以了解需要添加测试的位置：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\datadedup_test.go
          
现在我理解了所有三个问题。开始修复：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
**问题 1 核心思路**：当前 `appendLocked` 在第一遍扫描时把所有 entry 原始字节收集到 `entryBuffer`，然后计算新 checksum 时 `newChecksum.Write(entryBuffer)`，内存 O(n)。

关键洞察：当前校验和计算逻辑是 `checksum = SHA256(oldHeader || entry1 || entry2 || ... || entryN)`，新校验和需要 `newChecksum = SHA256(newHeader || entry1 || entry2 || ... || entryN || newEntry)`。

但 SHA256 的内部状态是不可导出的。我需要换一种方式：将当前校验和的**最终结果**作为新校验和的一部分，而不是重放所有原始字节。具体来说：

- 在 `Save` 和 `loadLocked` 中，当前校验和覆盖范围是 `header || entries`
- 新的追加校验和可以改为：`SHA256(newHeader || SHA256(oldHeader || entries) || newEntry)`

但这会改变 checksum 计算方式，导致已保存文件的 checksum 与新算法不兼容。更好的方案：**在第一遍扫描时同时维护一个增量 checksum（仅覆盖 entries 部分，不含 header）**，然后在追加时用 `SHA256(newHeader || entriesChecksum || newEntry)` 来计算新校验和。

等等，这仍然改变了 checksum 计算方式。让我重新思考——最干净的方案是修改持久化格式，在文件中额外存储一个"entries checksum"中间状态，但那也改变了格式。

最简单且兼容的做法：**在第一遍扫描时保存增量 SHA256 状态（仅 feed entries 数据），然后第二遍只需要 feed newHeader 和 newEntry**。但 `crypto/sha256` 的 `digest` 类型不暴露内部状态……

Go 标准库的 `sha256.New()` 返回的 `hash.Hash` 接口没有导出内部状态的方法。但我可以这样做：

1. 第一遍扫描：计算 `entriesChecksum = SHA256(entry1 || entry2 || ... || entryN)`（不含 header）
2. 第二遍：`newChecksum = SHA256(newHeader || entriesChecksumSum || newEntry)`

但这改变了新文件的 checksum 计算方式。新写入的文件用 `Save` 写入时 checksum = `SHA256(header || entries)`，而 Append 写入的文件 checksum = `SHA256(newHeader || entriesChecksum || newEntry)`。`loadLocked` 无法区分这两种格式。

**最佳方案**：修改 `saveLocked` 的 checksum 计算方式，使其也分两层——先算 entries 的 digest，再用 `SHA256(header || entriesDigest)` 作为文件 checksum。这样 Save 和 Append 使用统一的公式，且 Append 时只需要 O(1) 空间来保存 entriesDigest。

让我实现这个方案：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
等等，我的 `saveLocked` 实现有问题——我需要先写 header 再写 entries 再写 checksum。让我重新修正 write 顺序：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在修改 `loadLocked` 使用同样的两层校验和逻辑：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在修改 `appendLocked`，用 `entriesDigest` 替代 `entryBuffer`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
persistence.go 的修复完成了。现在修复问题 2：移除 `calcTargetMask` 中未使用的 `windowSize` 参数：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\chunker.go
          
changes: undefined
          
现在补充 Append 幂等性的单元测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\datadedup_test.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
Append 操作后的 Load 出现 checksum mismatch。问题在于 `appendLocked` 中，我使用了 `entriesDigest`（旧 entries 的 digest）来计算新的 entries digest，方式是 `SHA256(oldEntriesDigest || newEntryBytes)`。但 `loadLocked` 中 `entriesChecksum` 是直接对每个 entry 的原始字节做哈希，不是递归摘要。

关键不一致：
- `saveLocked`/`loadLocked`：`entriesChecksum = SHA256(entry1 || entry2 || ... || entryN)` — 直接哈希所有 entry 字节
- `appendLocked`：`newEntriesChecksum = SHA256(oldEntriesDigest || fpEntryBytes)` — 递归哈希

这两种方式产生的 digest 不同！需要让 appendLocked 的新 entries digest 与 saveLocked 产生的完全一致。

解决方案：我需要在 `saveLocked` 和 `appendLocked` 之间统一 entries digest 的计算方式。最简洁的方式是：在 `saveLocked` 写入文件时，额外存储一个 `entriesDigest entry`，这样 `appendLocked` 可以直接读取这个值。但这改变了文件格式。

更简单的方式：**不改变文件格式，而是让 fileChecksum 的计算中，entriesDigest 是一个 Merkle-like 的递归哈希**。这样 Save 和 Append 都可以用同样的方式计算：

- `saveLocked`：`entriesDigest = SHA256(entry1 || SHA256(entry2 || ... || SHA256(entryN)))`，但这也需要特定顺序……

最干净的方式：**修改 saveLocked 在 checksum entry 之前再写一个 entriesDigest entry**，这样 appendLocked 可以直接读取。

实际上更简单的方案是：**在文件中把 entries digest 作为 checksum entry 的 FP 值的一部分存储**。我可以用两个 entry：一个 entriesDigest entry，一个 fileChecksum entry。

不，让我换一种更简单的思路。核心问题是 `appendLocked` 无法获得旧 entries 的 digest，因为 saveLocked 没有在文件中存储它。

**最佳方案**：在持久化文件中增加一个 `entryTypeEntriesDigest` 类型的条目，存储 entries 的 SHA256 digest。这样 `appendLocked` 可以直接读取这个值，然后计算新的 entries digest 和 file checksum，完全 O(1) 空间。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在修改 `saveLocked`，在 checksum entry 之前写入 entriesDigest entry：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在修改 `loadLocked`，跳过 entriesDigest entry 但仍然正确计算校验和：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在修改 `appendLocked`，让它直接从文件中读取 `entriesDigest entry`，而不是自己重新计算：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
还是 checksum mismatch。问题在于 Append 后，`loadLocked` 需要能正确验证。让我写一个调试测试来看看到底哪里不匹配：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -run TestPersistence/Append$ -v -count=1 2>&1 undefined
          
问题出在 Append 后 loadLocked 校验失败。关键问题是：`loadLocked` 中 `entriesChecksum` 对每个 fp entry 的原始字节做增量哈希，最终得到的 `entriesDigest` 需要与 `storedEntriesDigest` 一致。但 Append 后 `storedEntriesDigest` 是用 `SHA256(oldEntriesDigest || newEntryBytes)` 计算的，而 `loadLocked` 中的 `entriesChecksum` 是对每个 entry 的完整原始字节（包括 type, len, fp, timestamp）做哈希。

这两种计算方式不一致！`saveLocked` 中的 `entriesChecksum.Write(entryBytes)` 写入的是 `encodeEntry` 编码后的完整字节，而 `appendLocked` 中的 `newEntriesChecksum.Write(fpEntryBytes)` 也是写入 `encodeEntry` 编码后的完整字节，但 `newEntriesChecksum.Write(oldEntriesDigest)` 写入的是旧 entries 的 32 字节摘要。

所以 `loadLocked` 中逐 entry 计算的 `entriesDigest` ≠ `appendLocked` 中用递归方式计算的 `newEntriesDigest`。

根本原因：两种方式计算的 entriesDigest 不同。`loadLocked` 是 `SHA256(entry1_bytes || entry2_bytes || ... || entryN_bytes)`，而 Append 后存储的 entriesDigest 是 `SHA256(SHA256(entry1_bytes || ... || entryN-1_bytes) || entryN_bytes)`。

解决方案：**`loadLocked` 应该信任存储的 `storedEntriesDigest`**，不需要自己重新计算 entries digest 来验证它。因为 fileChecksum 验证已经足够保证完整性。或者，`loadLocked` 也需要用递归方式来验证。

最简单的方式：`loadLocked` 中不做 entriesDigest 的独立验证，只做 fileChecksum 验证。entriesDigest 的作用只是让 appendLocked 能以 O(1) 空间更新。

让我修改 `loadLocked`，移除 entriesDigest 的独立验证（因为 fileChecksum 已经覆盖了完整性校验）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
等等，这个逻辑不对。`loadLocked` 需要验证文件完整性，但现在如果 `storedEntriesDigest` 被篡改了，fileChecksum 只验证了 `SHA256(header || storedEntriesDigest)`，但 `storedEntriesDigest` 本身可能被篡改。

我需要更仔细地设计这个逻辑。核心问题是：

1. `saveLocked` 写入时，fileChecksum = `SHA256(header || SHA256(entries))`，entriesDigest entry 存储 `SHA256(entries)`
2. `appendLocked` 写入时，newEntriesDigest = `SHA256(oldEntriesDigest || newEntryBytes)`，这不等于 `SHA256(all_entries)`

最好的做法是：**`loadLocked` 对 Append 后的文件，用递归方式重新计算 entriesDigest 来验证**。但 `loadLocked` 并不知道 entries 的添加顺序（map 迭代顺序不确定）。

实际上，更根本的问题是：**saveLocked 中 FingerprintIndex 是 map，迭代顺序不确定，所以每次 Save 产生的 entriesDigest 可能不同**。这也是为什么原始代码用 `io.MultiWriter` 直接写文件并同时计算 checksum——这样 checksum 是按写入文件的顺序计算的。

让我重新思考这个问题。最简单且正确的方案：

**让 `saveLocked` 在写入 entries 之后（在 entriesDigest entry 之前），存储 entriesDigest。`loadLocked` 重新计算 entriesDigest 并与存储的值比对。`appendLocked` 从文件读取 entriesDigest，用递归哈希 `SHA256(oldDigest || newEntry)` 更新它，同时更新 entriesDigest entry。`loadLocked` 也用同样的递归方式验证。**

但 `loadLocked` 怎么递归验证？它需要知道哪些 entries 是原始的、哪些是后来追加的。这不可行。

最简单的方案：**`loadLocked` 总是从 fp entries 重新计算 `SHA256(entry1_bytes || entry2_bytes || ...)`，并与存储的 entriesDigest 比对**。这意味着 `appendLocked` 必须产生与 `SHA256(all_entries_bytes)` 一致的 entriesDigest。

问题在于 `appendLocked` 不知道所有旧 entries 的原始字节（否则就不是 O(1) 空间了）。

**真正的解决方案**：让 `appendLocked` 不需要更新 entriesDigest。把 entriesDigest entry 保持不变，然后在 fileChecksum 中包含 entriesDigest 和所有 fp entries 的增量 checksum。

等等，我想到一个更好的方案：**完全不用 entriesDigest entry，而是使用 hash.Hash 的内部状态序列化**。但 Go 标准库不支持导出 sha256 内部状态。

让我换个思路：**最简单的方案是让 `saveLocked` 在文件中保存一个 checksum，其计算方式为 `SHA256(header || entry1 || entry2 || ... || entryN)`。`appendLocked` 在第一遍扫描时，维护一个 `sha256.Hash` 状态，逐个 entry feed 进去。在扫描完所有旧 entries 后，保存这个 hash 的内部状态。但由于 Go 不支持导出 hash 状态，所以我需要用另一种方式。**

OK，我想到最终的正确方案了。**不使用递归哈希，不使用 entriesDigest entry。**

**方案**：修改文件格式，在 entriesDigest entry 中存储的不是递归哈希，而是**旧的 fileChecksum 对应的 entries 部分**的 SHA256 哈希。但这和之前一样的问题——appendLocked 无法在不读所有 entry 字节的情况下计算出这个值。

让我彻底换一种思路：**使用 HMAC 或分层哈希**。

实际上最简单的方式：**在文件中直接存储 entries 的 SHA256 digest 作为 metadata entry，appendLocked 直接读取它。但 loadLocked 验证时不重新计算 entries digest，只验证 fileChecksum**。

fileChecksum 的计算方式：`SHA256(header || entriesDigest_bytes || entries_count)`。这里 entriesDigest_bytes 是文件中存储的 entriesDigest entry 的 FP 值。这样：

- `saveLocked`: 计算 `entriesDigest = SHA256(entry1 || entry2 || ...)`，写入 entriesDigest entry，fileChecksum = `SHA256(header || entriesDigest)`
- `appendLocked`: 读取旧的 entriesDigest，计算 `newEntriesDigest = SHA256(oldEntriesDigest || newEntry)`，写入新的 entriesDigest entry 和 new entry，fileChecksum = `SHA256(newHeader || newEntriesDigest)`
- `loadLocked`: 读取 entriesDigest entry，用 fileChecksum = `SHA256(header || entriesDigest)` 验证

但问题是 `saveLocked` 写入时 `entriesDigest = SHA256(entries)`，而 `appendLocked` 写入时 `newEntriesDigest = SHA256(oldEntriesDigest || newEntry)`，这两种方式不一致。`loadLocked` 无法知道 entriesDigest 是哪种方式计算的。

**最终方案**：让 `appendLocked` 也用 `SHA256(entries)` 的方式计算 newEntriesDigest。这意味着 `appendLocked` 必须读取所有旧 entry 的字节。但用户要求 O(1) 空间……

等等，O(1) 空间 ≠ O(1) 磁盘读取。`appendLocked` 可以重新读取文件来计算 entriesDigest，但不把所有 entry 字节保存在内存中。可以流式读取+增量哈希。

但这需要读两遍文件（第一遍找 csEntryOffset 和检查重复，第二遍重新计算 entriesDigest）。

实际上让我再想想——第一遍扫描时已经在逐 entry 读取了，可以同时做增量哈希。唯一的问题是：第一遍扫描时没有把 entry 字节保存到 entryBuffer，而是直接 feed 到 entriesChecksum。这不需要额外内存！

```go
entriesChecksum := sha256.New()
for each fp entry {
    entriesChecksum.Write(entryHeader)
    entriesChecksum.Write(rest)
    ...
}
entriesDigest := entriesChecksum.Sum(nil)  // SHA256(entry1 || entry2 || ...)
```

这样 `entriesDigest` 就是 `SHA256(entries)`，与 `saveLocked` 中计算方式完全一致！

但这样 `appendLocked` 就需要在内存中维护 `entriesChecksum` 的增量状态，这个状态是 O(1) 的（sha256 内部状态只有几十个字节）。计算新 entriesDigest 只需：

```go
newEntriesChecksum := sha256.New()
// 重新写入旧 entriesDigest？不行，这不是递归哈希
```

问题是：**我需要把旧的 `entriesChecksum` 状态恢复到一个新的 `sha256.Hash` 中，然后继续 feed 新 entry**。但 Go 的 `sha256.New()` 返回的 `hash.Hash` 不支持导出/导入内部状态。

所以最终答案是：**在第一遍扫描时就同时维护 entriesChecksum，然后 `entriesChecksum.Write(fpEntryBytes)` 来加入新 entry，这样最终 `entriesChecksum.Sum(nil)` 就是新的 entriesDigest**。

但这有一个问题：`entriesChecksum` 在第一遍扫描时已经被 `Sum(nil)` 调用过了（如果我在循环中调用了的话），之后它的状态就不能再用了。

让我检查——在当前代码中，第一遍扫描时我维护了 `entriesChecksum`，但在循环结束后没有调用 `Sum(nil)`。所以我可以直接继续 feed 新 entry 字节！

但等等，当前的 `appendLocked` 代码中我用了 `entriesChecksum` 但后面改成了从文件读取 `storedEntriesDigest`……让我重构 `appendLocked`，回到在第一遍扫描时用 `entriesChecksum` 增量计算的方式，然后直接 `entriesChecksum.Write(fpEntryBytes)` 得到新 entriesDigest：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在也需要修正 `loadLocked`——当文件中有 entriesDigest entry 时，用它来验证；否则用重新计算的 entriesDigest 验证：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
我需要确保 `bytes` 包被导入了：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
需要添加 `bytes` 包：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\datadedup\persistence.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/datadedup/ -v -timeout 2m undefined
          
所有测试全部通过！包括新增的 `AppendIdempotent` 测试。让我更新任务状态：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有三个问题已修复完成，全部测试通过！

---

## ✅ 修复总结

### 问题 1：appendLocked 内存 O(n) → O(1)

**修复前**：使用 `entryBuffer []byte` 收集所有已有 entry 的完整原始字节，内存占用随索引条目数线性增长。

**修复后** [appendLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L285-L454)：

核心思路——利用 SHA256 的**增量哈希**特性，`hash.Hash` 内部只需维护固定大小的状态（~100 字节），无需保存原始输入数据：

1. 新增 `entryTypeEntriesDigest = 3` 条目类型，[saveLocked](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/persistence.go#L50-L141) 在写入所有 fp entries 后，将 `SHA256(entries)` 的 32 字节摘要存入 entriesDigest entry
2. `appendLocked` 第一遍扫描时，维护 `entriesChecksum` 增量哈希状态，逐 entry feed 字节（空间 O(1)）
3. 扫描完成后直接 `entriesChecksum.Write(fpEntryBytes)`，调用 `Sum(nil)` 得到新 entriesDigest
4. 不再使用 `entryBuffer`，额外内存仅为 `sha256.digest` 内部状态（~100 字节）

**文件格式更新**：

```
Header(16B) → FP entries → EntriesDigest entry → Checksum entry
                                      ↑ 新增       ↑ 包含 SHA256(header || entriesDigest)
```

- `entriesDigest` = `SHA256(entry1_bytes || entry2_bytes || ... || entryN_bytes)`
- `fileChecksum` = `SHA256(headerBytes || entriesDigest)`
- `loadLocked` 同时验证 entriesDigest 和 fileChecksum，双重完整性保证

### 问题 2：calcTargetMask 未使用的 windowSize 参数

**修复** [calcTargetMask](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/chunker.go#L206-L213)：移除函数签名中的 `windowSize int` 参数，同时更新两处调用点（`NewContentBasedChunker` 和 `NewContentBasedChunkerWithProvider`）。

### 问题 3：Append 幂等性测试缺失

**新增** [TestPersistence/AppendIdempotent](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/datadedup/datadedup_test.go#L1010-L1071)：

- 保存 2 个指纹后 Append 第 3 个
- 对同一指纹连续重复 Append 2 次，验证不报错
- Load 验证只有 3 个条目（无重复）
- Verify 验证文件完整性
- 追加第 4 个不同指纹，Load 验证有 4 个条目

### 测试结果

```
PASS: TestPersistence (8 subtests) - 含新增 AppendIdempotent
... (全部 55 个测试用例)
ok  solocoder-go/internal/datadedup  0.493s
```