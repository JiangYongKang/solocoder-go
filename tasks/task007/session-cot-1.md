我来帮你实现这个连接池管理器功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\main.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
让我看看其他模块的文档和代码风格，以便保持一致。

        
            
toolName: file_search
            
status: success
          
file_pattern: internal/**/*.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\shardkv\hash_ring.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\shardkv.md
          
好的，项目结构已了解清楚。现在开始实现连接池模块。先创建目录结构和核心代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在创建连接池核心代码文件：

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
changes: undefined
          
我发现了一些设计问题需要修复，特别是等待机制和连接元数据的存储。让我重新设计一下：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
changes: undefined
          
让我用条件变量优化 Get() 方法的等待机制，这样更优雅也更高效：



        
            
toolName: Write
            
status: interrupted
          
            
filePath: undefined
          
          
我继续完成连接池模块的实现。首先让我优化一下核心代码中等待机制的实现，使用 `sync.Cond` 让代码更简洁可靠，然后编写单元测试。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我重新设计连接池，使用 `sync.Cond` 来实现等待机制，让代码更清晰可靠：

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool.go
          
changes: undefined
          
现在创建完整的单元测试文件，覆盖所有功能和边界条件：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
changes: undefined
          
现在来运行测试，看看是否有问题需要修复：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 undefined
          
修复未使用变量的问题：


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\connpool\pool_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -timeout 60s undefined
          
所有测试通过了！现在编写模块需求文档：






        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\connpool.md
          
changes: undefined
          
最后再验证一次所有测试通过：




        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -race -timeout 120s undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/connpool/ -v -count=1 -timeout 120s undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
连接池管理器模块已完整实现，所有需求均已满足。以下是交付内容总结：

---

## 交付文件

### 1. 核心实现：[pool.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go)

实现了所有 5 项核心功能：

| 需求 | 实现位置 | 说明 |
|------|----------|------|
| **连接借用与归还** | [Get()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L150-L238) / [Put()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L240-L285) | 支持 `WaitTimeout` 配置：`0` 立即返回 `ErrPoolExhausted`，非零则阻塞等待（使用 `sync.Cond`） |
| **心跳检测** | [heartbeatLoop()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L338-L398) | 按 `HeartbeatInterval` 定时对所有空闲连接执行 `Ping()`，失败的连接被关闭并移除 |
| **会话超时** | [idleTimeoutLoop()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L400-L443) | 按 `IdleTimeout/2` 间隔检查，`lastUsed` 超过 `IdleTimeout` 的空闲连接被回收 |
| **最大生命周期** | [getIdle()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L116-L148) + [Put()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L259-L264) | 借用和归还时均检查 `createTime`，超过 `MaxLifetime` 则关闭 |
| **空闲回收 (LRU)** | [Put()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool.go#L269-L280) | 使用 `container/list` 双向链表：归还插头部，超量从尾部回收 |

**线程安全**：互斥锁 `mu` + 条件变量 `cond` + 原子操作 `count` + `WaitGroup` 优雅退出。

---

### 2. 单元测试：[pool_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/connpool/pool_test.go)

共 **21 个测试用例全部通过**，覆盖范围：

- ✅ **配置校验**：无 Factory、MaxCap<=0、InitialCap 截断
- ✅ **基础借用/归还**：单连接、多连接、状态计数
- ✅ **等待策略**：非阻塞立即返回、阻塞超时、阻塞后被唤醒
- ✅ **异常输入**：nil 连接、外部连接、重复归还
- ✅ **池关闭**：资源释放、关闭后操作返回错误、等待中关闭
- ✅ **心跳检测**：坏连接被识别并移除
- ✅ **空闲超时**：空闲连接被回收、活跃连接不受影响
- ✅ **最大生命周期**：借用时过期检查、归还时过期检查
- ✅ **LRU 超量回收**：超过 MaxIdle 按创建顺序（最久未用）回收
- ✅ **高并发场景**：20 协程 × 100 次 Get/Put 无竞态
- ✅ **边界/异常**：所有空闲连接都坏时创建新连接、初始化 Factory 失败回滚、Get 时 Factory 失败释放槽位、等待中关闭唤醒

```
=== RUN   TestNewPool_InvalidConfig ... PASS
=== RUN   TestNewPool_InitialCap ... PASS
=== RUN   TestNewPool_InitialCapExceedsMaxCap ... PASS
=== RUN   TestGetAndPut_Single ... PASS
=== RUN   TestGetAndPut_Multiple ... PASS
=== RUN   TestGet_WaitTimeout ... PASS
=== RUN   TestGet_WaitTimeout_Success ... PASS
=== RUN   TestPut_InvalidConnection ... PASS
=== RUN   TestPool_Closed ... PASS
=== RUN   TestHeartbeat_RemovesBadConns ... PASS
=== RUN   TestIdleTimeout_ReclaimsConns ... PASS
=== RUN   TestMaxLifetime_BorrowCheck ... PASS
=== RUN   TestMaxLifetime_PutCheck ... PASS
=== RUN   TestMaxIdle_LRUReclaim ... PASS
=== RUN   TestConcurrent_GetPut ... PASS
=== RUN   TestGet_AllBadIdleCreatesNew ... PASS
=== RUN   TestNewPool_FactoryError ... PASS
=== RUN   TestGet_FactoryErrorCreatesSlot ... PASS
=== RUN   TestLen_Accurate ... PASS
=== RUN   TestGet_PoolClosedDuringWait ... PASS
=== RUN   TestIdleTimeout_ActiveConnNotReclaimed ... PASS
PASS
ok      solocoder-go/internal/connpool  1.078s
```

---

### 3. 需求文档：[connpool.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/connpool.md)

包含内容：
- **模块概述**与 8 项功能清单表格
- **4 个核心结构体**（Config、Pool、idleConn、类型别名）的职责说明与字段注释
- **4 个预定义错误**的触发场景表
- **6 个生命周期流程图**（ASCII 图）：创建、借用、归还、心跳、空闲回收、关闭
- **LRU 策略说明**：链表头/尾部的语义
- **线程安全说明**
- **4 个使用示例**：数据库连接池、非阻塞模式、监控指标、模拟测试