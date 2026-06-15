toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
先读取当前实现代码，理解每个问题的上下文：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在我已经完全理解了所有 6 个问题。让我一次性修复所有问题，然后补充测试。

**问题分析：**

1. **handleStream 双重操作**：`pool.Remove` 减 activeCount，然后 `defer upstreamConn.Close()` → `pool.Put` 又减 activeCount，导致负数。需要在 `pooledConn` 上加 `removed` 标志。

2. **ConnPool.Get 索引越界**：循环中 unlock→close→relock 期间 idleList 可能被修改。应该先收集过期连接再统一删除，锁外关闭。

3. **IPHashBalancer 死锁**：`GetUpstream` 持 RLock(balancer) 调 `hc.GetHealthyUpstreams()` (RLock hc)；而 `onChange` 路径是 Lock(hc) → Lock(balancer)。应改为不持 balancer 锁时调 hc 方法。

4. **handleFin/handleRst 竞态**：两者对同一 streamID 的操作应原子化，在同一个 Lock 下完成 check+delete+cleanup。

5. **HealthChecker.checkOne**：重加后 uh 指针变了但 failCount/passCount 继承了旧值。应验证 `uh.upstream` 指针是否还是同一个对象。

现在执行全部修复：
        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy.go
          
changes: undefined
          
核心代码修复完成。现在更新 todo 并编写补充测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在编写补充的并发场景单元测试，追加到现有测试文件末尾：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在运行测试验证所有修复和新增用例：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go && go test ./internal/tcpproxy/ -v -timeout 180s -race undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tcpproxy/ -v -timeout 180s -race undefined
          
Race detector 不支持 windows/386，去掉 `-race` 运行：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tcpproxy/ -v -timeout 180s undefined
          
`TestTCPProxy_EndToEnd` 超时了。问题是 `handleStream` 中上游读协程在 `upstreamConn.Read()` 上阻塞，而流端关闭时无法取消它。需要加一个 cancel 机制：当任一方向退出时，强制上游连接的 deadline 使另一个方向也退出。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy.go
          
changes: undefined
          
现在重新运行测试：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tcpproxy/ -v -timeout 120s undefined
          
`TestHealthChecker_RemoveAndReaddResetsCounters` 测试的断言逻辑反了。重新添加后 passCount 从 0 开始，一次 pass 后 passCount=1 < PassThreshold=2，所以 u2 应该仍然不健康。这才证明计数器没有继承旧值：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tcpproxy\tcpproxy_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tcpproxy/ -v -timeout 120s undefined
          
全部 45 个测试通过！现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tcpproxy.md
          
现在更新文档，添加并发保证策略章节：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tcpproxy.md
          
changes: undefined
          
现在更新测试覆盖策略表格，加入新增的并发测试：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tcpproxy.md
          
changes: undefined
          
更新文件结构部分的测试数量：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tcpproxy.md
          
changes: undefined
          
最后运行一次完整测试确认一切通过：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tcpproxy/ -v -timeout 120s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
全部任务完成。45 个测试全部 PASS。以下是修复内容总结：

## 修复的 5 个并发安全问题

### 1. 连接池 Remove/Put 双重操作 → 计数负数（[tcpproxy.go#L574-L580](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L574-L580)）
- **根因**：`handleStream` 中 `pool.Remove` 减 activeCount，`defer upstreamConn.Close()` 又调用 `pool.Put` 再减一次
- **修复**：`poolConn` 新增 `removed atomic.Bool`；`Remove` 用 `Swap(true)` 保证幂等只减一次；`Put` 和 `pooledConn.Close` 检查 removed 标志跳过已移除连接

### 2. ConnPool.Get 空闲过期扫描索引越界（[tcpproxy.go#L626-L656](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L626-L656)）
- **根因**：原实现 for 循环中检测到超时后 unlock→close→relock，期间 idleList 可能被并发修改
- **修复**：在单次持锁内完成：遍历收集过期+紧凑排列有效连接→选取→移除→解锁后关闭过期连接

### 3. IPHashBalancer 与 HealthChecker 锁顺序死锁（[tcpproxy.go#L905-L944](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L905-L944)）
- **根因**：`GetUpstream` 持 balancer RLock 时调 hc.RLock，而 onChange 路径是 hc.Lock → balancer.Lock，ABBA 死锁
- **修复**：新增 `getHealthyUpstreamsSnapshot()` 方法，先获取健康快照（释放 hc 锁），再操作 balancer 锁，消除嵌套持锁

### 4. handleFin/handleRst check-then-act 竞态（[tcpproxy.go#L303-L335](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L303-L335)）
- **根因**：两者先 RLock 检查流存在再 Lock 操作，中间窗口可被对方抢先
- **修复**：两个方法都改为全程持写锁，在锁内原子完成 check + delete + cleanup

### 5. HealthChecker.checkOne 上下文一致性（[tcpproxy.go#L533-L539](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L533-L539)）
- **根因**：RLock→Probe→Lock 期间上游可能被删除后以同地址重新添加，新对象的计数器继承了旧状态
- **修复**：Lock 后校验 `uh.upstream != upstream`（指针比较），不一致则跳过，新 upstreamHealth 从零开始计数

### 额外修复：handleStream 协程退出阻塞（[tcpproxy.go#L1155-L1200](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tcpproxy/tcpproxy.go#L1155-L1200)）
- **根因**：代理关闭时 stream→upstream 协程退出，但 upstream→stream 协程阻塞在 `upstreamConn.Read()` 上
- **修复**：引入 `closeUpstream`（sync.Once 保护），任一方向退出时 `SetDeadline(time.Now())` 强制另一方向 Read 返回

## 新增 9 个并发安全测试

| 测试 | 验证目标 |
|------|----------|
| `TestConnPool_RemoveThenCloseNoNegativeCount` | Remove 后 Close 不负数 |
| `TestConnPool_RemoveIdempotent` | 多次 Remove 只减一次 |
| `TestConnPool_PutAfterRemovedNoReturnToIdle` | 已移除连接不回池 |
| `TestConnPool_ConcurrentRemoveAndPut` | 并发 Remove+Put 安全 |
| `TestConnPool_GetIdleExpiryNoIndexPanic` | 极短空闲超时并发 Get 无越界 |
| `TestIPHashBalancer_NoDeadlockWithHealthChecker` | HC+Balancer 无死锁 |
| `TestMuxConcurrentFinAndRst` | FIN/RST 并发不 panic |
| `TestHealthChecker_RemoveAndReaddResetsCounters` | 删除重加计数器清零 |
| `TestConnPool_ConcurrentGetWithIdleExpiry` | 高并发+快速过期计数正确 |

文档 [tcpproxy.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tcpproxy.md) 已更新，新增 §6.1 并发安全保证策略（6 个子节详细说明每个问题的根因、修复方案和保证机制）和并发安全专项测试表格。