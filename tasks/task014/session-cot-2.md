好的，我来逐一修复这些问题。让我先仔细查看现有代码，然后制定修复计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 8 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\webhook\scheduler.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\webhook\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\webhook\signer.go
          
让我再仔细检查一下测试文件，看看这些功能是否有对应的测试覆盖：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\webhook\webhook_test.go
          
让我先运行测试，看看当前的状态：



        
            
toolName: run_command
            
status: success
          
command: go test ./internal/webhook/ -v undefined
          
所有测试都已通过。让我检查一下 `docs/webhook.md` 的状态，然后更新文档以反映这些功能：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我检查 `docs/` 目录和 `docs/webhook.md` 文件：




        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\webhook.md
          
经过仔细检查代码和运行测试，我发现您提到的这 4 个问题在当前代码中**都已经被修复**了。让我逐一确认并向您展示修复的位置：

## 问题修复状态确认

### 1. Trigger 方法区分回调不存在和已取消 ✅

已修复，在 [scheduler.go 第302-305行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L302-L305)：

```go
if cb.Status == CallbackStatusCancelled {
    s.mu.Unlock()
    return ErrCallbackCancelled
}
```

- 回调不存在 → 返回 `ErrCallbackNotFound`
- 回调已取消 → 返回 `ErrCallbackCancelled`

对应测试：[`TestTrigger_CancelledVsNotFound`](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/webhook_test.go#L1246-L1268)

### 2. Content-Type 不强制覆盖 ✅

已修复，在 [scheduler.go 第602-614行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L602-L614)：

```go
for k, v := range cb.Headers {
    req.Header.Set(k, v)
}
// ...
if req.Header.Get("Content-Type") == "" {
    req.Header.Set("Content-Type", "application/json")
}
```

- 先设置用户自定义 Headers
- 仅当 `Content-Type` 为空时才设置默认值 `"application/json"`

对应测试：[`TestContentType_NotOverridden`](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/webhook_test.go#L1270-L1327)

### 3. markFinalResult 使用 err 参数汇总错误 ✅

已修复，在 [scheduler.go 第651-658行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L651-L658)：

```go
func (s *Scheduler) markFinalResult(callbackID string, delivery *Delivery, final bool, err error) {
    s.mu.Lock()
    s.deliveryResults[callbackID] = &DeliveryResult{
        Delivery: delivery,
        Final:    final,
        Error:    err,
    }
    // ...
}
```

重试耗尽时在 [第553行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L553) 传入汇总错误：
```go
s.markFinalResult(cb.ID, delivery, true, fmt.Errorf("max retries exhausted: %s", delivery.Error))
```

`DeliveryResult` 结构体也增加了 `Error` 字段，在 [types.go 第119行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/types.go#L116-L120)。

对应测试：[`TestDeliveryResult_ErrorOnRetryExhausted`](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/webhook_test.go#L1329-L1373)

### 4. BodyTemplate 模板渲染 ✅

已修复，在 [scheduler.go 第584行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L584) 和 [第668-684行](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/scheduler.go#L668-L684)：

```go
body, err := renderTemplate(cb.BodyTemplate, data)

func renderTemplate(tpl string, data interface{}) ([]byte, error) {
    if tpl == "" {
        return []byte{}, nil
    }
    if data == nil {
        return []byte(tpl), nil
    }
    t, err := template.New("webhook").Parse(tpl)
    // ...
}
```

支持通过 `TriggerWithData(callbackID, data)` 传入模板数据。

对应测试：[`TestTemplateRendering`](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/webhook_test.go#L1375-L1538) 和 [`TestRenderTemplate`](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/webhook/webhook_test.go#L1540-L1577)

---

## 测试结果

所有 **30+ 测试用例全部通过**：

```
PASS
ok      solocoder-go/internal/webhook   3.745s
```

## 文档状态

[docs/webhook.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/webhook.md) 也已经包含了相关内容：

- **第6.1节 错误语义区分**：说明了 `ErrCallbackNotFound` 与 `ErrCallbackCancelled` 的区别
- **第6.2节 失败结果的错误汇总**：说明了 `DeliveryResult.Error` 汇总错误的机制
- **第5.1节基本使用** 等示例中展示了模板的使用

---

所有功能都已正确实现并通过测试。请问您还需要我做其他调整吗？