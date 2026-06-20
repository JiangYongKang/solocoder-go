好的，我来逐一修复这些问题。首先让我创建任务列表。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
先让我重新读取当前文件内容，确保使用最新的代码。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
好的，现在让我逐一修复这 4 个问题。首先从第一个问题开始：**GetTemplate 深拷贝 DefaultHeaders**，以及其他 3 个问题一并修复。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我一次性修复所有 4 个问题：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在添加 `serverError` 和 `templateInvalidError` 错误类型，以及 `cloneHeaders` 辅助函数：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在修复 **问题 1**（GetTemplate 深拷贝 DefaultHeaders）和 **问题 3**（RegisterTemplate 校验 AuthProvider）：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在修复 **GetTemplate** 的 DefaultHeaders 深拷贝问题：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在修复 **问题 2**（doRequest 检查状态码，5xx 视为错误触发重试）和 **问题 4**（重试耗尽不返回空响应）：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在修复 **doRequest** 方法，检查响应状态码，5xx 视为错误：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在更新 `isRetryableError` 函数，确保 `ErrServerError` 可重试：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在代码修复完成，让我更新任务并开始补充测试。首先修复旧测试中因为 RegisterTemplate 校验 AuthProvider 而可能失败的测试：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在我需要修改测试文件。首先修复旧测试，然后添加新测试。先让我修改 `TestDo_AuthProvider_NotFound`，因为 RegisterTemplate 现在会校验 AuthProvider 存在性：








        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
现在我需要修复几个使用了自定义 `retryableTransport` 的测试。由于 doRequest 内部已检查 5xx 状态码，不再需要自定义 Transport 了。同时添加新测试用例。先修复旧的重试测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
继续修复其他重试测试（去掉自定义 Transport）：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
现在修复剩下的三个使用自定义 Transport 的重试测试：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
现在增强 `TestGetTemplate_ReturnsCopy` 测试以覆盖 DefaultHeaders 深拷贝，并添加新的场景测试。让我先读取这部分内容然后修改：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
现在增强 `TestGetTemplate_ReturnsCopy` 并添加新的场景测试：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
我需要检查测试文件的 import 是否包含了 `fmt`，因为我在新测试中用了 `fmt.Sprintf`。让我先看一下然后运行测试：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
需要添加 `fmt` 导入：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient_test.go
          
changes: undefined
          
现在让我运行测试看看是否全部通过：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v -count=1 undefined
          
有三个测试失败了。问题在于：
1. `fmt.Errorf` 使用了 `%v` 而不是 `%w`，导致 `errors.Is` 无法穿透错误链
2. 当 `MaxRetries=0` 时，语义上应该直接返回原始错误（因为根本没重试），而不是包装 `ErrMaxRetriesExceeded`

让我修复这两个问题：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在再次运行测试：



















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v -count=1 undefined
          
全部 51 个测试通过！让我更新任务状态。

























        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有 4 个问题都已修复，共 **51 个测试全部通过**。以下是修复的详细总结：

---

## 修复总结

### 问题 1：GetTemplate DefaultHeaders 浅拷贝污染内部存储 ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L77-L87)

新增 `cloneHeaders()` 辅助函数，对 `http.Header`（`map[string][]string` 引用类型）进行深拷贝：
- `GetTemplate` 返回模板副本时，`DefaultHeaders` 使用 `cloneHeaders()` 拷贝
- `RegisterTemplate` 注册模板时，外部传入的 `DefaultHeaders` 也先克隆再存储（双向隔离）

**补充测试**:
- [TestGetTemplate_ReturnsCopy](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1069-L1102) — 修改返回副本的 DefaultHeaders 不影响内部存储
- [TestRegisterTemplate_InputHeadersNotShared](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1144-L1166) — 修改外部原始 Header 不影响已注册模板

---

### 问题 2：doRequest 不检查 5xx 状态码 ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L43-L58) + [doRequest](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L342-L359)

- 新增 `serverError` 错误类型，实现 `Is(target)` 方法以支持 `errors.Is(err, ErrServerError)` 匹配
- 新增 `ErrServerError` 哨兵错误
- `doRequest` 在 `http.Client.Do` 返回后检查 `resp.StatusCode >= 500`，读取响应体并返回 `&serverError{}`
- `isRetryableError()` 中将 `ErrServerError` 标记为**可重试**
- **4xx 错误（如 400、404）视为业务正常响应，直接返回**

**补充测试**:
- [TestDo_Server500Error_ReturnsServerError](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1168-L1199) — 500 错误返回 `ErrServerError`，错误信息包含状态码和响应体
- [TestDo_Server503Error_TriggersRetry](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1201-L1228) — 503 触发重试（1 次初始 + 3 次重试 = 4 次调用）
- [TestDo_Client4xxError_TreatedAsSuccess](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1230-L1258) — 404 不触发重试，直接返回响应

---

### 问题 3：RegisterTemplate 不校验 AuthProvider 存在性 ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L60-L75) + [RegisterTemplate](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L150-L186)

- 新增 `templateInvalidError` 错误类型，包含 `templateName` 字段，实现 `Unwrap()` 和 `Is()` 方法
- 新增 `ErrTemplateInvalid` 哨兵错误
- `RegisterTemplate` 在加锁后，若模板指定了 `AuthProvider`，立即在 `authProviders` map 中校验存在性，不存在则返回：
  ```
  restclient: invalid template: template 'xxx': restclient: auth provider not found: 'yyy'
  ```

**补充测试**:
- [TestRegisterTemplate_AuthProviderNotRegistered](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1104-L1124) — 未注册提供器时报错，错误信息包含模板名称和提供器名称
- [TestRegisterTemplate_AuthProviderRegisteredFirst](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1126-L1142) — 先注册提供器再注册模板，正常通过
- [TestDo_AuthProvider_NotFound](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L542-L559) — 已从运行时测试改为注册时校验测试

---

### 问题 4：重试耗尽返回空响应 Body + 错误 ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L226-L284)

- 移除 `lastResp *http.Response` 变量
- 每次请求失败后，若有响应 Body 立即 `Close()` 释放
- 重试耗尽后统一返回 `(nil, error)`，不再返回空 Body 的响应
- **语义优化**: `MaxRetries=0`（不重试）时直接返回原始错误，不包装 `ErrMaxRetriesExceeded`
- **错误链修复**: 包装错误改用 `fmt.Errorf("%w: %w", ErrMaxRetriesExceeded, lastErr)`，`errors.Is()` 可穿透检查

**补充测试**:
- [TestDo_Retry_MaxRetriesExceeded](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L671-L706) — 响应为 nil，错误包含 `ErrMaxRetriesExceeded` 和 `ErrServerError`
- [TestDo_Retry_ZeroRetries](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L708-L737) — MaxRetries=0 直接返回 `ErrServerError`，不包装
- [TestDo_MaxRetriesExhausted_ReturnsNilResponse](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient_test.go#L1260-L1288) — 专门验证响应为 nil，避免误判

---

## 测试结果

```
PASS: 51 个测试全部通过
ok  	solocoder-go/internal/restclient	0.873s
```