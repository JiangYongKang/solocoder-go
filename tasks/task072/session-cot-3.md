好的，我来修复这两个问题。先让我看一下当前的 `doRequest` 实现。




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
现在修复这两个问题：
1. 5xx 错误时返回 `nil` 响应而不是已关闭 Body 的响应，避免上层二次 Close
2. 限制读取响应体的大小（最多 1KB），避免大体积错误页面造成的内存浪费







        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
同时修复这两个问题：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在修复 `doRequest` 方法：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
changes: undefined
          
现在让我检查一下 `Do` 方法的逻辑，确保 `resp == nil` 时不会有问题：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\restclient\restclient.go
          
`Do` 方法的逻辑是安全的，因为 `resp != nil && resp.Body != nil` 判断会跳过 nil 的 resp。现在更新任务并运行测试：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/restclient/ -v -count=1 undefined
          
全部 51 个测试通过！让我更新任务列表。

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
两个问题都已修复，全部 **51 个测试通过**。以下是修复详情：

---

## 修复总结

### 问题 1：已关闭 Body 的响应导致二次 Close ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L353-L376)

**修复前的问题流程**：
```
doRequest 检测 5xx：
  io.ReadAll(resp.Body) → 读取全部
  resp.Body.Close()     → 关闭 Body
  return resp, serverError  → 返回已关闭 Body 的 resp

Do 方法上层：
  resp != nil 判断为 true → resp.Body.Close() → 二次 Close ❌
```

**修复方案**：
- 在读取 Body **之前**，先将 `resp.StatusCode` 和 `resp.Status` 保存到局部变量
- 读取并关闭 Body 后，返回 `(nil, &serverError{...})`，而不是返回已关闭 Body 的 resp
- Do 方法上层 `resp != nil && resp.Body != nil` 判断安全跳过，不会二次 Close

```go
statusCode := resp.StatusCode   // 先保存到局部变量
status := resp.Status
// ... 读取 & 关闭 Body ...
return nil, &serverError{       // 返回 nil 响应
    statusCode: statusCode,
    status:     status,
}
```

---

### 问题 2：io.ReadAll 大响应体内存浪费 ✅
**修改文件**: [restclient.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L16) + [doRequest](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/restclient/restclient.go#L360-L369)

**修复方案**：
- 新增常量 `maxErrorBodySize = 1024`（最多读取 1KB 错误信息）
- 使用 `io.LimitReader(resp.Body, maxErrorBodySize)` 限制读取大小
- 如果恰好读取到 `maxErrorBodySize` 字节（说明还有更多内容），末尾追加 `"..."` 提示截断

```go
const maxErrorBodySize = 1024

limitedReader := io.LimitReader(resp.Body, maxErrorBodySize)
body, readErr := io.ReadAll(limitedReader)
// ...
if len(body) == maxErrorBodySize {
    bodyStr += "..."
}
```

**效果**：
- 上游返回 1MB 错误页面时，最多只分配 1KB 内存（原先是 1MB）
- 重试最终成功时，之前失败尝试的内存开销从 `N × 全量响应` 降到 `N × 1KB`
- 错误信息仍然保留，便于调试，只是超过 1KB 的部分被明确标记截断

---

## 测试结果

```
PASS: 51 个测试全部通过
ok  	solocoder-go/internal/restclient	0.879s
```