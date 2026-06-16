我来帮你实现这个密码策略引擎模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\writethrough.md
          
让我先查看几个现有模块的代码风格，以便保持一致性。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rbac\rbac.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\secenv\secenv.go
          
让我再看一个测试文件的风格，确保测试代码风格一致。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rbac\rbac_test.go
          
现在我对项目结构和代码风格有了清晰的了解。让我创建任务计划并开始实现。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在开始实现核心代码文件 `passpolicy.go`：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy.go
          
changes: undefined
          
现在需要添加依赖并创建测试文件。先检查 go.mod 并添加 bcrypt 依赖：




        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go get golang.org/x/crypto/bcrypt undefined
          
网络问题导致无法下载依赖。让我检查项目中是否已存在 golang.org/x/crypto 相关的依赖，或者尝试使用其他方式：




        
            
toolName: search_by_regex
            
status: success
          
query: golang.org/x/crypto
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          
让我检查是否存在 go.sum 以及其他可能使用 bcrypt 的模块：







        
            
toolName: file_search
            
status: success
          
file_pattern: **/go.sum
          

        
            
toolName: search_by_regex
            
status: success
          
query: bcrypt
          
让我尝试设置国内代理来下载依赖：






        
            
toolName: run_command
            
status: success
          
command: $env:GOPROXY = "https://goproxy.cn,direct"; cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go get golang.org/x/crypto/bcrypt undefined
          
依赖安装成功！现在更新任务进度并编写测试文件：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在编写完整的单元测试文件：









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
changes: undefined
          
现在运行测试来验证所有功能：
















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/passpolicy/ -v -count=1 undefined
          
有语法错误，让我检查第 64 行附近的代码：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
发现第 62 行多了一个多余的 `}`。让我修复这个问题：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
changes: undefined
          
现在再次运行测试：



















        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/passpolicy/ -v -count=1 undefined
          
有几个测试失败了，问题是测试中使用的简单密码（如 "Pass1!"）不满足默认配置的复杂度要求（最小长度8个字符）。让我修复这些测试用例：
































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
changes: undefined
          
现在修复另外两个失败的测试：





































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
changes: undefined
          
现在修复最后一个失败的测试：








































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\passpolicy\passpolicy_test.go
          
changes: undefined
          
再次运行测试：

















































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/passpolicy/ -v -count=1 undefined
          
所有测试都通过了！现在更新任务进度并编写需求文档：


















































        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在编写模块需求文档：

















































        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\passpolicy.md
          
changes: undefined
          
我注意到文档中引用了 `IsPasswordExpiredError` 辅助函数但代码中未实现。让我修改文档使用标准的 `errors.Is` 方式，并完成任务：










































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\passpolicy.md
          
changes: undefined
          
现在为该示例添加 errors 的 import：










































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\passpolicy.md
          
changes: undefined
          
最后再运行一次测试确保一切正常：











































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/passpolicy/ -v -count=1 2>&1 | Select-Object -Last 20 undefined
          
所有测试都通过了！完成任务进度：










































        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## ✅ 密码策略引擎模块实现完成

所有 7 项要求已全部实现并验证通过：

---

### 📁 交付文件清单

| 文件 | 行数 | 说明 |
|------|------|------|
| [passpolicy.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/passpolicy/passpolicy.go) | ~660 行 | 核心业务逻辑实现 |
| [passpolicy_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/passpolicy/passpolicy_test.go) | ~1070 行 | 34 个测试函数（含大量子测试） |
| [passpolicy.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/passpolicy.md) | ~830 行 | 完整需求文档 |

---

### ✅ 测试运行结果

```
go test ./internal/passpolicy/ -v -count=1
...
PASS
ok   solocoder-go/internal/passpolicy   7.097s
```

**所有测试通过 ✅**，覆盖：
- 配置边界校验（MinLength、Cost、HistoryDepth、ExpiryDays）
- 密码复杂度 5 维度规则校验（长度/大写/小写/数字/特殊字符）
- 历史密码 FIFO 裁剪与复用检测
- 密码过期生命周期（正常→警告期→过期）
- bcrypt cost 动态升级与自动 rehash
- 130 goroutine 并发安全测试
- 完整生命周期集成测试

---

### 🧩 核心功能一览

| 功能模块 | 实现要点 |
|---------|---------|
| **复杂度校验** | `ValidatePassword()` 返回 `[]PolicyViolation` 精确违规原因列表 |
| **历史检查** | FIFO 队列存储 bcrypt 哈希，逐个 CompareAndSwap 验证复用 |
| **过期管理** | ExpiryDays=0 永不过期，WarningDays 提醒期，MustChange 强制标记 |
| **自适应哈希** | `VerifyPassword` 验证通过后若当前 Cost 更高自动 rehash，返回 `Rehashed=true` |
| **并发安全** | `sync.RWMutex` 读写分离，历史比较在写锁内完成保证一致性 |
| **测试友好** | `SetNowFunc()` 注入时间，支持模拟时间流逝的确定性测试 |