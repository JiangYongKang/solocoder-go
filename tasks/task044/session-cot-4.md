好的，我来逐一修复这两个问题。首先查看相关代码的当前状态：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
先查看相关代码：
        
            
toolName: search_by_regex
            
status: success
          
query: func verifyLogSetConsistency
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
找到问题 1 的位置了。在第 673 行，严格模式数据丢失时 `TamperedIndex` 设置为 `len(writerLogs)`，应改为 `-1`。同时需要修改相关测试的断言。

现在让我查看相关测试：

        
            
toolName: search_by_regex
            
status: success
          
query: DetectsWriterDataLoss_AfterStop|WriterDeleteSpecificLog
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
现在我清楚所有需要修改的地方了。开始逐一修复：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
先修改 `auditlog.go` 中数据丢失场景的 TamperedIndex：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog.go
          
changes: undefined
          
现在更新两个相关测试：`TestFix_Issue4_VerifyIntegrity_DetectsWriterDataLoss_AfterStop` 和 `TestFix_Issue4_VerifyIntegrity_WriterDeleteSpecificLog` 的断言：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在更新 `TestFix_Issue4_VerifyIntegrity_WriterDeleteSpecificLog`：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在补充自洽性校验的独立测试用例。需要篡改 Writer 中日志的 CurrentHash **同时**篡改内存中对应日志的 CurrentHash（绕过跨层哈希对比），并确保篡改最后一条（绕过链式校验），这样才能走到自洽性校验路径。

先查看测试文件最后部分的结构：


        
            
toolName: search_by_regex
            
status: success
          
query: ^func TestLogger_LogSync_StartedStoppedConsistency
          
让我查看 NonReadableWriter 测试结束的位置：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
现在添加自洽性校验的独立测试用例。因为测试代码与实现在同一个包内，可以直接访问未导出字段来同步篡改内存和Writer中的CurrentHash，从而绕过前几步校验，走到自洽性校验路径：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
现在运行所有测试，验证全部通过：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/auditlog/ -v 2>&1 undefined
          
问题是：我篡改了内存中日志的 CurrentHash，导致内存层的哈希链自洽性校验先失败了。需要让内存层校验也通过，即内存日志的 CurrentHash 也要保持与内存内容计算一致。

策略：**修改 Writer 中最后一条日志的内容字段（如 Detail）但不改变其 CurrentHash**，这样：
- 内存层链式校验通过（没改动内存日志）
- Writer 层链式校验通过（PreviousHash 关系不变，因为修改的是最后一条，不影响前面链）
- 跨层哈希对比：wl.CurrentHash == ml.CurrentHash（两边 CurrentHash 还是原值），但内容不同导致跨层检测先触发——哦不行。

更好的策略：只修改 Writer 层最后一条日志的内容（Detail），同时**同步修改内存中对应日志的内容保持一致**，但两边的 CurrentHash 都保留原来错误的旧值。这样：
- 内存自洽性校验：内容变了，但 CurrentHash 是旧值 -> 内存自洽性先失败！还是不行。

**正确策略**：只修改 Writer 层最后一条的内容，Writer 的 CurrentHash 保留原值（导致 Writer 自洽性失败）；但为了不让内存层先失败，要保证内存层自洽。同时为了不让跨层哈希对比先失败，要保证 wl.CurrentHash == ml.CurrentHash。

做法是：Writer 最后一条日志内容改变 + 内存最后一条日志也用相同方式改变 + 两边 CurrentHash 都不变。这样：
- 内存层：内容变了，CurrentHash 不变 -> 内存自洽性先失败。还是不行！

**根本问题**：内存层自洽性校验在 Writer 层校验之前执行。要测试 Writer 自洽性校验路径，必须先让内存层校验通过。必须保证内存层的日志内容与内存日志的 CurrentHash 一致（自洽），而只让 Writer 层日志内容与 Writer 日志 CurrentHash 不一致。

**最终策略**：
- 内存日志：内容和 CurrentHash 都保持原样正确
- Writer 最后一条日志：内容**不改动**，CurrentHash **不改动**（所以前面所有检查都通过）。然后：Writer 最后一条日志的 **PreviousHash** 也正确。但是 Writer 最后一条日志内容改了——这会导致跨层哈希对比先发现。

哦，等等！跨层对比是比较 wl.CurrentHash 和 ml.CurrentHash。如果我只改 Writer 日志内容，两边 CurrentHash 还是相同的原值 -> 跨层对比通过。但 Writer 自洽性（computeHash(wl) != wl.CurrentHash）就会失败。同时内存自洽性也没问题。

关键问题：内存层自洽性没问题，Writer 层跨层对比（比较 CurrentHash 值）也没问题，那 Writer 自洽性就会执行！

让我仔细重写这个测试：只修改 Writer 最后一条的内容字段，其他不改动。内存层完全不动。这样：
1. 内存自洽：通过
2. Writer 重复：通过  
3. Writer 存在性：通过
4. fullySynced + 链式校验（PreviousHash）：通过，因为 PreviousHash 没动，且是最后一条不影响后续
5. 跨层哈希对比：wl.CurrentHash == ml.CurrentHash，通过（都是原值）
6. Writer 自洽性校验：computeHash(wl) != wl.CurrentHash，因为 wl 内容被修改了 -> **触发！**

完美，这就是正确的测试方案。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\auditlog\auditlog_test.go
          
changes: undefined
          
等等，这样会导致跨层哈希对比（步骤 4）先触发，因为 wl.CurrentHash != ml.CurrentHash（因为 wl 内容变了，computeHash(wl) 变了但 wl.CurrentHash 没变，而 ml.CurrentHash 是正确的原值 == computeHash(ml) == 正确的旧哈希；wl.CurrentHash = 正确的旧哈希，ml.CurrentHash 也是正确的旧哈希——所以 **wl.CurrentHash == ml.CurrentHash！** 步骤 4 通过。然后才到步骤 5 自洽性校验！

是的，没错！步骤 4 是比较 **wl.CurrentHash 和 ml.CurrentHash 的值是否相等**，不是比较 computeHash(wl) 和 computeHash(ml)。因为 wl.CurrentHash 和 ml.CurrentHash 都是正确的旧值（都没变），所以相等，步骤 4 通过。然后步骤 5 computeHash(wl) == wl.CurrentHash？wl 内容变了所以 computeHash(wl) 变了，不等于 wl.CurrentHash（旧值），所以步骤 5 触发！正确！

运行测试：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/auditlog/ -v 2>&1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
全部 **63 个测试**通过！以下是本次修复的总结：

---

## 修复总结

### 问题 1：严格模式数据丢失场景 TamperedIndex 越界

**修改文件**：[auditlog.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog.go#L670-L676)

**核心改动**：
在 `verifyLogSetConsistency` 函数的严格数量检查分支中，将 `TamperedIndex` 从 `len(writerLogs)` 修改为 `-1` 哨兵值。

**修改前**（有问题）：
```go
TamperedIndex: len(writerLogs),  // = 4 但 writerLogs 合法索引是 0-3，越界
```

**修改后**：
```go
TamperedIndex: -1,  // 明确语义的哨兵值：数据丢失无单一被篡改日志
```

**相关测试更新**：
- [TestFix_Issue4_VerifyIntegrity_DetectsWriterDataLoss_AfterStop](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog_test.go#L1710-L1723)：断言 `TamperedIndex == -1`
- [TestFix_Issue4_VerifyIntegrity_WriterDeleteSpecificLog](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog_test.go#L1800-L1810)：断言 `TamperedIndex == -1`

---

### 问题 2：Writer 自洽性校验路径缺失测试覆盖

**新增测试**：[TestFix_Issue4_VerifyIntegrity_DetectsWriterSelfConsistencyHashFailure](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/auditlog/auditlog_test.go#L1931-L1967)

**设计思路**（精确定位到自洽性校验路径）：
1. 写入 4 条日志并等同步完成
2. 只修改 **Writer 层**最后一条日志的 `Detail` 内容（**不修改任何 CurrentHash**）
3. 这样各层校验路径的结果是：
   - ✅ 内存自洽性校验：通过（内存日志未被改动）
   - ✅ Writer 重复/存在性检查：通过
   - ✅ fullySynced 链式校验：通过（PreviousHash 关系不变，修改的是最后一条）
   - ✅ 跨层哈希对比：通过（wl.CurrentHash 和 ml.CurrentHash 都是正确的旧值，完全相等）
   - ❌ **Writer 自洽性校验**：触发！（computeHash(wl) ≠ wl.CurrentHash，因为内容变了但哈希没更新）

**断言覆盖**：
- `TamperedIndex == 3`（精确指向最后一条）
- `Message` 包含 `"self-consistency"`（证明走的是自洽性路径而非其他路径）
- `Message` 包含 `"hash mismatch"`
- `FullySynchronized == false`

---

### 测试结果

```
PASS
ok  	solocoder-go/internal/auditlog	1.932s
```

**63 个测试全部通过** ✅