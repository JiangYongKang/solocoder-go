toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore_test.go
          
现在修复这两个问题：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\kvstore\kvstore_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/kvstore/ -v -run TestConcurrentBatchPutAndGet_Consistency -count=1 undefined
          
测试通过。现在运行完整测试套件确保一切正常：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/kvstore/ -v -count=1 undefined
          
所有 55 个测试全部通过！现在更新文档：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
所有测试通过！让我完成最后的任务标记：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## ✅ 修复完成

### 修复内容

**1. 动态键抽样策略修复** — [kvstore_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/kvstore/kvstore_test.go#L1317-L1340)

| 项目 | 修复前 | 修复后 |
|------|--------|--------|
| 公式 | `(s * iterations) / samplesPerWriter` | `s * (iterations - 1) / (samplesPerWriter - 1)` |
| 采样点 (iter=200) | 0, 20, 40, ..., 180 | 0, 22, 44, ..., 177, 199 |
| 覆盖率 | 仅覆盖前 90% (0-180) | 全范围覆盖 (0-199)，首尾都包含 |

**等距抽样原理**：分子分母同时减1，确保 `s=0` 时得 `0`，`s=samplesPerWriter-1` 时得 `iterations-1`，均匀分布首尾。

---

**2. 预存键分区验证** — [kvstore_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/kvstore/kvstore_test.go#L1342-L1360)

| 键范围 | 是否被 writer 触碰 | 验证级别 |
|--------|-------------------|----------|
| `init_0` ~ `init_4` | ✅ 被 writer 0-4 覆盖 | 检查存在性 + 非空 |
| `init_5` ~ `init_9` | ❌ 不应被触碰 | **精确值校验**（等于 `initval_5` ~ `initval_9`） |

如果并发 BatchPut 存在越界写入错误，`init_5` ~ `init_9` 会被意外污染，精确值校验能立即检测到。

---

### 测试结果

```
=== 全部 55 个测试通过 ===
--- PASS: TestConcurrentBatchPutAndGet_Consistency (0.00s)
    Data integrity verified: 5010 total keys match expected count
    Sampled 250 dynamic keys across 5 writers (full range 0..199, including last iteration): all present and correct
PASS
ok  solocoder-go/internal/kvstore  0.294s
```