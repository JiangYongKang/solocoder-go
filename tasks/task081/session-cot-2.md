我来分析并修复这些问题。首先让我查看当前代码的状态。
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
好的，我已经仔细阅读了所有代码。让我创建任务计划来修复这些问题。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先修复问题 1：`ExtractTraceContext` 中 `ParentSpanID` 始终为空。

在 W3C Trace Context 规范中，`traceparent` 的 `parent-id` 字段代表调用方（上游）的 SpanID。当下游服务提取上下文时，该 `parent-id` 就是下游新 Span 的父 Span，因此应当将 `ParentSpanID` 设为 `parent-id`，以保留完整的父子关系语义。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在修复问题 2 和 3。这两个问题都涉及 `SpanTree`，我一起修改：添加 TraceID 一致性校验 + 使用 children 索引优化查询性能。
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          
changes: undefined
          
现在重写 `SpanTree` 结构体，添加 `traceID` 字段用于一致性校验和 `children` 索引用于性能优化：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          
changes: undefined
          
核心代码修复完成。现在更新任务状态并补充单元测试。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在需要修复现有测试中受影响的测试用例，并补充新的边界条件测试。先来分析哪些现有测试需要调整：

1. `TestSpanTreeGetRoots` — 之前往同一棵树加了不同 TraceID 的 span，现在会被拒绝
2. `TestInjectExtractRoundTrip` — 需要验证 ParentSpanID 的传播
3. `TestExtractTraceContext` — 需要验证 ParentSpanID 非空
4. `TestSpanTreeConcurrentAdd` — 使用了 `generateTestSpanID`，需要确保 ID 唯一性
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在修复 `TestSpanTreeGetRoots` 测试，它之前把不同 TraceID 的 span 放进了同一棵树：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在修复 `TestInjectExtractRoundTrip` 以验证 ParentSpanID 的正确传播：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在在测试文件末尾添加新的边界条件测试。我需要读取文件末尾来确认插入位置：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在还需要修复原来的 `TestSpanTreeConcurrentAdd` 测试，它使用了 `generateTestSpanID` 但这些 ID 可能会重复（某些不同 i 值会产生相同结果），并且现在需要统一 TraceID：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx_test.go
          
changes: undefined
          
现在运行测试验证所有修复：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/tracectx/ -v undefined
          
所有测试全部通过！现在更新文档。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tracectx.md
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\tracectx.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复已完成，全部测试通过。以下是修复内容的总结：

## 修复内容

### 1. ExtractTraceContext ParentSpanID 修复

**问题**：[ExtractTraceContext](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go#L432-L437) 中 `ParentSpanID` 始终为空字符串，丢失了上游调用方的 SpanID 语义信息。

**修复**：将 `ParentSpanID` 设为 W3C `traceparent` 头中的 `parent-id` 值。该字段代表上游调用方的 SpanID，是下游所有子 Span 的父节点。这样 `NewChildContext(extractedCtx, name)` 创建的子 Span 能正确建立 `ParentSpanID → 上游 SpanID` 的父子关系。

### 2. SpanTree TraceID 一致性校验

**问题**：[AddSpan](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go#L161-L193) 缺少跨 TraceID 校验，不同追踪链路的 Span 可被混入同一棵树。

**修复**：SpanTree 新增 `traceID` 字段，首个 Span 添加时设定 TraceID，后续添加的 Span 若 TraceID 不匹配则返回 `ErrTraceIDMismatch`。新增 [TraceID()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go#L196-L199) 方法用于查询树绑定的 TraceID。

### 3. SpanTree 查询性能优化

**问题**：[GetChildren](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go#L217-L233) 和 [GetSubtree](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/tracectx/tracectx.go#L244-L268) 原实现遍历所有 Span（O(N)），节点数多时性能严重衰减。

**修复**：新增 `children map[string][]*Span` 索引，AddSpan 时自动维护。`GetChildren` 通过哈希直接定位子节点列表（O(k)），`GetSubtree` 递归遍历仅访问子树内节点（O(n)），不再扫描全量数据。

### 4. 补充的单元测试

新增 14 个边界条件测试，覆盖：
- ParentSpanID 传播语义（注入-提取-创建子上下文全链路）
- 未采样场景下 ParentSpanID 仍然正确设置
- TraceID 一致性校验（不同 TraceID 被拒绝、空树首次设定、同 Trace 多根节点）
- GetChildren 返回副本（修改不影响内部数据）、叶节点无子节点
- 大规模树（5000 节点）查询正确性
- 并发读写安全、并发添加相同/不同 TraceID
- 完整工作流中 ParentSpanID 的端到端验证

### 5. 文档更新

[docs/tracectx.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/tracectx.md) 新增：
- **数据一致性约束**章节：TraceID 一致性校验规则和 SpanID 唯一性约束
- **查询性能特性**章节：内部数据结构说明、各操作复杂度表、性能优化说明
- **错误类型**参考表
- ParentSpanID 传播语义说明