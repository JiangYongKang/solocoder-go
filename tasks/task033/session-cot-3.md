让我先仔细阅读当前代码，理解每个缺陷的具体情况。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
现在我已经完整阅读了代码，清楚理解了4个缺陷。让我开始修复。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
由于4个缺陷涉及多处相互关联的代码修改，我将使用子代理并行处理实现文件和测试文件的重写。
        
            
toolName: no_need_execute
            
status: no_need_execute
          
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
开始逐步应用修复。首先添加 `reflect` 到 import 块：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
**Defect 1**: 修复 `RecvMsg` 丢弃数据的问题：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
**Defect 2**: 修复 cancel 函数泄漏。首先添加 `cancelFn` 字段到 `serverStream` 结构体：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
修复 `NewStream` 中的 cancel 处理：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
修复 `Close()` 方法，在 `closeOnce.Do` 中调用 `cancelFn`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
**Defect 3**: 将 `Send()` 重命名为 `RecvFromServer()`。先修改 Stream 接口：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
然后修改 `serverStream` 的方法名：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
**Defect 4**: 简化双 goroutine 模式。先替换 `Invoke` 中的 handler：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在替换 `HandleStream` 中的 handler：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修改已完成，现在验证编译是否通过：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./internal/grpcsvc/ undefined
          
编译通过了！但还需要检查项目中是否有其他文件引用了旧的 `Send()` 方法名，确保没有编译断裂：
        
            
toolName: search_by_regex
            
status: success
          
query: \.Send\(\)
          
测试文件中有两处引用了旧的 `Send()` 方法名，需要一并更新：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
现在再次验证整个项目编译：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go build ./... undefined
          
编译错误在 `servicereg` 包中，是预存问题，与我们的修改无关。让我们单独验证 `grpcsvc` 包及其测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/grpcsvc/ undefined
          
测试文件中的 `mockStream` 也需要添加 `RecvFromServer` 方法。让我查看它：
        
            
toolName: search_by_regex
            
status: success
          
query: mockStream
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/grpcsvc/ undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
测试失败是因为 `Close()` 现在会调用 `cancelFn()` 取消上下文，导致 handler 看到 `context canceled`。让我检查一下 echo handler 的行为：
        
            
toolName: search_by_regex
            
status: success
          
query: newEchoServiceDesc|BidiStream
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
这是正确的行为变化——`Close()` 现在调用 `cancelFn()` 取消上下文后，handler 会看到 `context.Canceled` 而非 `ErrStreamClosed`。测试需要更新以匹配修复后的正确行为：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          
再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/grpcsvc/ -v -count=1 2>&1 | Select-Object -Last 40 undefined
          
所有测试都通过了！
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
全部 4 个缺陷已修复，编译和测试均通过。以下是修改摘要：

---

### 修改总结

**Defect 1: RecvMsg 丢弃数据** (`grpcsvc.go:795-805`)
- `RecvMsg` 原来用 `_` 丢弃从 `recvCh` 读取的数据，导致 `msg` 参数从未被填充
- 修复：使用 `reflect.ValueOf` 检查 `msg` 是否为非 nil 指针，然后用 `rv.Elem().Set()` 将数据写入
- 添加了 `"reflect"` 到 import 块

**Defect 2: Cancel 函数泄漏** (`grpcsvc.go:744-746, 552-565, 919-933`)
- `NewStream` 中 `context.WithTimeout` 的 cancel 函数被 `_ = cancel` 丢弃，导致资源泄漏
- 修复：在 `serverStream` 结构体添加 `cancelFn context.CancelFunc` 字段；在 `NewStream` 中存储 cancel 函数；在 `Close()` 的 `closeOnce.Do` 中优先调用 `ss.cancelFn()` 释放资源

**Defect 3: Send() 重命名为 RecvFromServer()** (`grpcsvc.go:157, 841`)
- 原 `Send()` 方法实际是从 `sendCh` 读取（接收服务端消息），与 `SendMsg` 容易混淆
- 修复：Stream 接口和 serverStream 实现中的 `Send()` 统一重命名为 `RecvFromServer()`
- 同步更新了测试文件中的 `mockStream` 和调用处

**Defect 4: 简化双 goroutine 为单 goroutine + select** (`grpcsvc.go:452-491, 593-631`)
- `Invoke` 和 `HandleStream` 中原来的模式是：外层 goroutine 包裹内层 goroutine + 中间 channel 转发
- 修复：改为单 goroutine 运行 handler + 调用方直接 select 监听 `handlerDone`/`ticker.C`/`ctx.Done()`，消除了 `resultCh` 中转 channel 和多余的 `done` channel，代码更简洁高效
        
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
让我先验证子代理的修改结果，读取更新后的文件。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
实现文件已正确更新。现在需要更新测试文件。让我修改测试中的 `Send()` → `RecvFromServer()` 并添加新测试。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
测试文件中 `Send()` 已经被改为 `RecvFromServer()`，mockStream 也已更新。现在让我先运行测试看看当前状态。
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go && go test ./internal/grpcsvc/ -v 2>&1 undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
70 个现有测试全部通过。现在添加新的单元测试覆盖3个场景：RecvMsg 数据传递、cancel 资源释放、RecvFromServer 命名正确性。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
现在运行测试。
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
两个问题需要修复：

1. `TestRecvMsg_NilPointer` 花了30秒 — 因为 `RecvMsg` 先从 recvCh 读取数据再校验 msg，无效参数导致数据被丢弃后又阻塞。需要先校验再读取。
2. `TestCancel_NoCancelFnWhenNoConnectionTimeout` — `NewServer()` 默认有 ConnectionTimeout=30s，所以 cancelFn 一定不为 nil。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          
现在修复测试 `TestCancel_NoCancelFnWhenNoConnectionTimeout`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
`TestStream_RecvOnClosed` 失败了，因为 `RecvMsg(nil)` 现在先校验参数再检查流状态。应该先检查流是否关闭。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\grpcsvc\grpcsvc.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v 2>&1 undefined
          
所有 **80 个测试全部通过**！现在更新文档。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
现在让我更新文档中需要修改的关键部分。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
现在更新超时校验流程图（从双层 goroutine 简化为单层+select）：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
同样更新流式调用流程：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
更新 NewStream 流程图，添加 cancelFn：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
更新超时持续校验流程图：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
更新超时持续校验机制描述：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
现在更新文档中的使用示例（将 `Send()` 替换为 `RecvFromServer()`）：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
现在更新文档中的修复记录部分：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
现在更新文档中的测试覆盖范围，添加新的测试场景：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\grpcsvc.md
          
changes: undefined
          
最后运行一次完整测试确保一切正常：
        
            
toolName: run_command
            
status: success
          
command: go test ./internal/grpcsvc/ -v -count=1 2>&1 undefined
          
**80 个测试全部通过**，执行时间 0.407 秒（之前有 30 秒超时问题已修复）。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          
所有任务已完成！以下是修复总结：

---

## ✅ 修复的 4 个代码缺陷

### 1. RecvMsg 数据传递（[grpcsvc.go:742-772](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L742-L772)）

**问题**：`RecvMsg` 从 `recvCh` 读取消息后用 `_` 丢弃，`msg` 参数从未被填充。

**修复**：
- 使用 `reflect.ValueOf(msg)` 校验 msg 必须为非 nil 指针
- 通过 `rv.Elem().Set(reflect.ValueOf(data))` 将数据写入 msg
- 校验顺序：先检查流是否关闭，再校验参数（避免无效参数导致数据从 channel 中被丢弃）

### 2. cancel 资源泄漏（[grpcsvc.go:531-534](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L531-L534), [879-881](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L879-L881)）

**问题**：`NewStream` 中 `context.WithTimeout` 的 cancel 函数被 `_ = cancel` 丢弃，流关闭后 context 内部 goroutine 泄漏到超时自然过期。

**修复**：
- 在 `serverStream` 中新增 `cancelFn context.CancelFunc` 字段
- `NewStream` 中将 cancel 函数保存到 `cancelFn`
- `Close()` 的 `closeOnce.Do` 中优先调用 `ss.cancelFn()` 释放资源

### 3. Send/Recv 命名冲突（[grpcsvc.go:157](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L157), [799-823](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L799-L823)）

**问题**：`Send()` 方法实际从 `sendCh` 读取（接收服务端消息），与 `SendMsg()`（写入 sendCh）方向相反，在双向流场景下极易误用。

**修复**：将 `Send()` 重命名为 `RecvFromServer()`，语义为"接收来自服务端的消息"。同步更新了 Stream 接口、serverStream 实现、mockStream、所有测试和文档。

消息流向对照表：

| 方法 | 数据方向 | 使用方 |
|------|----------|--------|
| `SendMsg(msg)` | 服务端 → sendCh → 客户端 | 服务端 handler |
| `RecvFromServer()` | sendCh → 客户端/测试方 | 客户端/测试方 |
| `PutRecv(msg)` | 客户端/测试方 → recvCh | 客户端/测试方 |
| `Recv()` / `RecvMsg(msg)` | recvCh → 服务端 handler | 服务端 handler |

### 4. 双层 goroutine 简化（[grpcsvc.go:452-491](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L452-L491), [593-631](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/grpcsvc/grpcsvc.go#L593-L631)）

**问题**：`Invoke` 和 `HandleStream` 每调用一次 RPC 启动两个 goroutine（外层监控+内层 handler），高并发下调度开销成倍。

**修复**：改为单 goroutine 执行 handler + 调用方 goroutine 直接 `select` 监听 `handlerDone`/`ticker.C`/`ctx.Done()`。消除了 `resultCh` 中转 channel 和多余的 `done` channel。每 RPC 调用减少 1 个 goroutine。

## ✅ 新增 10 个单元测试

| 测试 | 验证场景 |
|------|----------|
| `TestRecvMsg_DataPopulation` | RecvMsg 将字符串数据写入 msg 指针 |
| `TestRecvMsg_MultipleDataPopulation` | RecvMsg 连续多条消息传递 |
| `TestRecvMsg_NilPointer` | RecvMsg 对 nil/非指针参数返回错误 |
| `TestRecvMsg_IntegerData` | RecvMsg 传递整数类型数据 |
| `TestCancel_ReleaseOnStreamClose` | 流关闭后 context 立即被 cancel |
| `TestCancel_NoLeakOnStreamClose` | cancel 后 ctx.Err() 返回 context.Canceled |
| `TestCancel_CancelFnSetWithConnectionTimeout` | ConnectionTimeout > 0 时 cancelFn 存在 |
| `TestRecvFromServer_NamingCorrectness` | 服务端流通过 RecvFromServer 读取响应 |
| `TestRecvFromServer_BidiNaming` | 双向流通过 RecvFromServer 读取服务端消息 |
| `TestRecvFromServer_ClientStream` | 客户端流通过 RecvFromServer 读取汇总响应 |

## ✅ 测试结果

`go test ./internal/grpcsvc/ -v` — **80 个测试全部通过**，执行时间 0.407 秒。

## ✅ 文档更新

[docs/grpcsvc.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/grpcsvc.md) 已更新：Stream 接口语义描述、消息流向对照表、超时校验流程图（单层 goroutine+select）、NewStream 流程图（cancelFn）、使用示例（RecvFromServer）、v3.0 修复记录。