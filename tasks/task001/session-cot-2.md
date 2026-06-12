我来分析并修复这些问题。先查看相关代码确认问题。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\circuit.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\ratelimit.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\types.go
          
确认了三个问题：
1. MockUpstreamHandler 的 `statusCode`/`responseBody`/`customHandler`/`latency` 无锁保护，并发下可能不一致
2. FailureEntry.Success 始终为 false（RecordSuccess 直接清空切片，不追加成功记录），过滤逻辑是死代码
3. RateLimiterStats 从未被引用，属于死代码

开始修复：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先修复 MockUpstreamHandler 的并发安全问题。由于 statusCode 和 responseBody 需要一起原子更新，统一用一个 `mu` 互斥锁保护所有可变状态：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复 FailureEntry.Success 死代码问题。由于 RecordSuccess 直接清空切片（不追加成功记录），Success 字段永远为 false，过滤逻辑形同虚设。直接移除该字段，简化为纯时间窗口计数：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\types.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\circuit.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\circuit.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
移除 RateLimiterStats 死代码，同时检查 extractPort 是否也未被使用：






        
            
toolName: search_by_regex
            
status: success
          
query: RateLimiterStats|extractPort
          

        
            
toolName: search_by_regex
            
status: success
          
query: RateLimiterStats|extractPort
          
`RateLimiterStats` 和 `extractPort` 都是死代码，一起移除：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\ratelimit.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\ratelimit.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在补充 MockUpstreamHandler 并发读写的单元测试。先读取现有测试文件找到合适位置：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway_test.go
          

        
            
toolName: search_by_regex
            
status: success
          
query: func TestMockUpstreamHandler
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway_test.go
          
在 `TestMockUpstreamHandler` 之后添加并发测试：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\gateway_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gateway/ -v -count=1 -race 2>&1 undefined
          
Windows 32位不支持 race detector，直接跑普通测试：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gateway/ -v -count=1 2>&1 undefined
          
`sync` 导入未使用？让我检查 ratelimit.go 中的实际使用情况：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\ratelimit.go
          
啊，我明白了。`sync` 在 `ratelimit.go` 中是通过结构体字段间接使用的，而 `RateLimiter` 和 `tokenBucket` 结构体定义在 `types.go` 中。`ratelimit.go` 本身不需要 import `sync`，因为它只是调用方法，不直接引用 sync 包的类型。让我检查一下：








        
            
toolName: search_by_regex
            
status: success
          
query: sync\.
          
对，`ratelimit.go` 只调用结构体字段的方法，不直接引用 `sync` 包的类型，所以不需要 import。移除它：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\gateway\ratelimit.go
          
changes: undefined
          
再次运行测试：











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gateway/ -v -count=1 2>&1 undefined
          
输出好像是 eventsrc 的测试？让我重新明确地跑 gateway 的测试：










        
            
toolName: run_command
            
status: success
          
command: go test -v -count=1 ./internal/gateway/ 2>&1 | Select-Object -Last 80 undefined
          
所有测试通过！现在更新 `docs/gateway.md` 文档，说明修复后的并发保证策略和熔断器记录机制：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          
我已经完成了前 5 项修复（MockUpstreamHandler 并发安全、移除 Success 死代码、移除 RateLimiterStats、补充并发测试、测试通过），正在更新文档。让我继续完成文档更新。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          

        
            
toolName: search_by_regex
            
status: success
          
query: 线程安全|文件结构|## 8|## 9
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          
现在更新文档中关于熔断器记录机制和并发保证策略的部分：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          
changes: undefined
          
现在更新 MockUpstreamHandler 部分的并发安全说明和线程安全表：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\gateway.md
          
changes: undefined
          
最后再跑一次测试确认全部通过：




        
            
toolName: run_command
            
status: success
          
command: go test ./internal/gateway/ -v -count=1 2>&1 | Select-Object -Last 20 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有问题已修复并验证通过。以下是本次修复的完整总结：

---

## ✅ 修复总览

### 1. MockUpstreamHandler 并发安全修复

**问题**：`SetResponse` 和 `ServeHTTP` 对 `statusCode` / `responseBody` / `customHandler` / `latency` 字段的读写无锁保护，并发下会出现"新状态码 + 旧响应体"的数据撕裂。

**修复**（[gateway.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gateway/gateway.go#L329-L416)）：
- 将分散的 `healthyMu` / `countMu` 替换为**单一 `sync.RWMutex`**，统一保护所有可变字段
- `ServeHTTP` 采用"锁内读快照 + 锁外执行"模式：一次性把 statusCode / body / handler / latency 拷到局部变量后立即解锁，再执行 sleep 和响应写入，既保证一致性又避免长耗时持锁
- 所有 setter（`SetResponse` / `SetHealthy` / `SetLatency` / `SetCustomHandler` / `ResetCount`）加写锁
- 所有 getter（`HealthCheck` / `RequestCount`）加读锁

### 2. FailureEntry.Success 死代码清理

**问题**：`Success` 字段始终为 false（`RecordSuccess` 直接清空切片而非追加成功记录），`countRecentFailures` 中 `if !entry.Success` 的过滤逻辑形同虚设。

**修复**（[circuit.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gateway/circuit.go) + [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gateway/types.go#L77-L79)）：
- 移除 `FailureEntry.Success` 字段，只保留 `Time time.Time`
- 简化 `countRecentFailures`，直接按时间窗口计数，无需判断 Success
- `RecordFailure` 不再写入 `Success: false`

### 3. RateLimiterStats 死代码移除

**问题**：`RateLimiterStats` 结构体及其 3 个方法（`RecordAllowed` / `RecordDenied` / `GetStats`）从未被任何生产代码或测试代码引用，连带 `extractPort` 也是死代码。

**修复**（[ratelimit.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gateway/ratelimit.go)）：
- 完整移除 `RateLimiterStats` 类型、构造函数及三个方法
- 移除 `extractPort` 辅助函数
- 清理对应的 `strconv` 和 `sync` 未使用 import

### 4. 新增并发单元测试

新增 5 个并发测试用例（[gateway_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/gateway/gateway_test.go#L1033-L1261)）：

| 测试用例 | 验证目标 |
|---------|---------|
| `TestMockUpstreamHandler_ConcurrentSetResponse` | 5 个写 goroutine 并发改响应 + 10 个读 goroutine 并发发请求，断言状态码与 body 始终匹配（无撕裂） |
| `TestMockUpstreamHandler_ConcurrentHealthToggle` | 4 个写 goroutine 并发切健康状态 + 8 个读 goroutine 读，断言不出现非法值 |
| `TestMockUpstreamHandler_ConcurrentSetLatency` | 5 个写 goroutine 改延迟 + 5 个读 goroutine 发请求，断言总请求数正确 |
| `TestMockUpstreamHandler_ConcurrentCustomHandler` | 1 个写 goroutine 频繁替换 customHandler + 10 个读 goroutine 发请求，断言状态码始终是预期值之一 |
| `TestMockUpstreamHandler_ConcurrentResetCount` | 10 个写 goroutine 累加请求 + 1 个 goroutine 周期 Reset，断言计数不为负 |

### 5. 测试结果

```
PASS: TestMockUpstreamHandler_ConcurrentSetResponse
PASS: TestMockUpstreamHandler_ConcurrentHealthToggle
PASS: TestMockUpstreamHandler_ConcurrentSetLatency
PASS: TestMockUpstreamHandler_ConcurrentCustomHandler
PASS: TestMockUpstreamHandler_ConcurrentResetCount
... (原有 40+ 测试全部通过)

ok  solocoder-go/internal/gateway   1.644s
```

### 6. 文档更新

[gateway.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/gateway.md) 更新内容：

- **3.7 UpstreamHandler 接口**：补充 MockUpstreamHandler 的原子更新和并发安全说明
- **3.5 CircuitBreaker**：新增"熔断器记录机制说明"小节，详细说明 FailureEntry 只记失败、各状态下的计数行为及设计原因
- **第 8 节 线程安全说明**：
  - 更新 MockUpstreamHandler 行，说明单一 `sync.RWMutex` 保护所有字段
  - 新增"MockUpstreamHandler 并发保证策略"小节，对比修复前后的差异，说明读-拷贝-使用模式