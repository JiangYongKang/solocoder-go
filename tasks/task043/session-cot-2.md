我来修复这三个并发安全和一致性问题。首先创建任务计划：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先查看当前代码，然后逐一修复：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          
现在我已经找到了所有三个问题的具体位置。让我一次性修复所有问题。首先修改 `RotateToken` 方法：

**问题分析：**
1. **RotateToken 原子性**：`ValidateToken`（读锁）→ 释放读锁 → 获取写锁，中间存在时间窗口
2. **GetToken 不清理过期**：只读不清理，与 `ValidateToken` 行为不一致
3. **Cookie 不更新**：同步令牌模式轮换后只更新 Header，未更新 Cookie



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          
changes: undefined
          
现在修复 `RotateToken` 原子性问题，将验证-删除-创建合并到单个写锁临界区：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          
changes: undefined
          
现在修复第三个问题：Middleware 同步令牌模式下轮换 Token 后同步更新 Cookie：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf.go
          
changes: undefined
          
三个代码问题已修复。现在补充并发场景的单元测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
现在在测试文件末尾追加新的并发场景测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          
检查一下测试文件是否需要添加 `fmt` 导入，然后运行测试：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
需要添加 `fmt` 导入：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          
现在运行所有测试：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/csrf/ -v -count=1 undefined
          
有两处编译错误，需要修复：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          

        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: 
          
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          
现在重新运行测试：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/csrf/ -v -count=1 undefined
          
有一个测试失败，`TestGetToken_CleansExpiredTokens` 中 `session3` 的 Token 也过期了（因为在 sleep 之前创建，TTL 50ms，sleep 100ms）。需要修复这个测试的时序：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\csrf\csrf_test.go
          
changes: undefined
          
再次运行测试：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/csrf/ -v -count=1 undefined
          
所有 43 个测试全部通过！让我更新任务状态：











        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有问题已修复，**43 个测试全部通过**。以下是修复内容的详细总结：

## 问题 1：RotateToken 原子性（孤儿 Token 风险）

**原问题**：`ValidateToken`（读锁）→ 释放读锁 → 获取写锁 → 删除 → 创建。两次锁操作之间的时间窗口内，Token 可能被其他协程修改，导致创建出在 `tokens` map 中存在但 `sessions` map 中不可达的孤儿 Token，造成内存泄漏。

**修复**（[csrf.go L239-L287](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/csrf/csrf.go#L239-L287)）：
- 移除对 `ValidateToken` 的外部调用
- 将**参数校验 → Token 存在性检查 → 过期检查 → 会话绑定检查 → 删除旧 Token → 创建新 Token** 全部合并到**单个写锁临界区**内
- 增加额外一致性检查：`sessions[sessionID]` 必须等于传入的 `token`，确保未被其他协程替换

## 问题 2：GetToken 不清理过期 Token（内存泄漏）

**原问题**：`GetToken` 用读锁检测到过期后仅返回错误，不清理内存中的过期条目；而 `ValidateToken` 检测到过期后会主动删除。两个方法行为不一致导致过期 Token 持续占用内存。

**修复**（[csrf.go L173-L204](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/csrf/csrf.go#L173-L204)）：
- 读锁检测到过期后，释放读锁，**升级为写锁**
- 在写锁下清理 `sessions` map 和 `tokens` map 中对应的过期条目
- 清理完成后返回 `ErrTokenNotFound`，与 `ValidateToken` 行为保持一致

## 问题 3：Middleware Cookie 未同步更新（客户端 403）

**原问题**：`SynchronizerTokenMode` 模式下 Token 轮换成功后仅更新响应头 `X-CSRF-Token`，但没有同步更新 Cookie。`extractToken` 按**请求头 → Cookie → 表单字段**三级优先级提取 Token。如果客户端通过 Cookie 传递 Token，轮换后 Cookie 仍持有已失效的旧 Token，下次请求会收到 403。

**修复**（[csrf.go L572](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/csrf/csrf.go#L572)）：
- 轮换成功后增加 `c.setTokenCookie(w, newToken)` 调用
- 与 `DoubleSubmitCookieMode` 保持一致，同时更新 Cookie 和响应头

## 新增测试用例（7 个）

| 测试 | 验证点 |
|------|--------|
| `TestRotateToken_Concurrent_Atomicity` | 100 协程并发轮换，最终 `TokenCount=1` 且无孤儿 Token |
| `TestRotateToken_Atomicity_NoOrphanTokens` | 1000 次迭代轮换+失效并发，每次检查无孤儿 Token，`tokens` 与 `sessions` map 大小一致 |
| `TestGetToken_CleansExpiredTokens` | GetToken 清理 2 个过期 Token，仅剩 1 个有效 Token，内存被正确回收 |
| `TestGetToken_ExpiredCleanup_Concurrent` | 50 协程并发 GetToken 过期 Token，最终全部清理完成 |
| `TestMiddleware_SynchronizerMode_CookieRotation_ClosedLoop` | 5 次连续请求通过 Cookie 提交 Token，每次轮换后 Cookie 正确更新，下次请求成功 |
| `TestMiddleware_SynchronizerMode_CookieToken_AfterRotation_Success` | 专门验证"轮换后用新 Cookie 提交"的成功场景（即 bug 修复验证） |
| `TestGetToken_ValidateToken_Consistency_AfterExpiry` | 验证两个方法过期处理行为一致，内存清理效果相同 |

**最终结果**：`go test ./internal/csrf/ -v -count=1` → **PASS (43 tests, 0.779s)**