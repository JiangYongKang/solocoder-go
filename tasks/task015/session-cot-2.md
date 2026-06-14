我来修复这些问题。首先创建任务列表：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
先读取当前代码：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
现在进行修复。首先修改 `dedup.go`：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          
现在修改 `Stop` 方法，设置永久停止标志：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          
现在修改 `CheckAndMark` 方法，增加停止状态检查：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          
继续修复其他方法。修改 `Contains` 方法，增加停止检查并改变签名返回 error：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          
现在修改 `Clear` 和 `CleanExpired` 方法：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup.go
          
changes: undefined
          
现在更新单元测试以适配新的 API 签名和行为：



















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在更新测试文件，重写整个测试文件以适配新的 API：
























        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup_test.go
          
changes: undefined
          
现在运行测试：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/dedup/ -v undefined
          
有一个测试的时间设计有问题，让我修复它：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\dedup\dedup_test.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/dedup/ -v undefined
          
所有 31 个测试全部通过！现在运行诊断检查：

























        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在更新 `docs/dedup.md` 文档。先读取当前内容然后更新：

































        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
现在更新文档中的多个部分。首先修改配置约束、结构体定义和预定义错误：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新预定义错误的表格：

























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新 CheckAndMark 流程图，增加 stopped 状态检查：


























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在在功能清单中增加生命周期错误说明，并更新 Count 方法语义：
































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在增加 Count 统计语义的专门说明：




























        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新后台清理流程和生命周期部分：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新生命周期与资源回收部分：







































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新使用示例部分：









































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在更新测试覆盖说明部分：











































        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\dedup.md
          
changes: undefined
          
现在再次运行测试确保所有测试通过：

































        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/dedup/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
所有修复已完成，所有 31 个测试全部通过。以下是修复内容的总结：

---

## 修复内容

### 1. Count 方法语义修复 ([dedup.go#L171-L191](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/dedup/dedup.go#L171-L191))

**问题**：`Count()` 直接返回 `len(idMap)`，包含了已过期但尚未被清理的记录，监控时给出虚高数据。

**修复**：改为遍历链表跳过过期记录，只统计窗口内有效记录：
- 从链表头部（最早记录）开始遍历
- 遇到 `createdAt <= cutoff` 的过期记录则跳过并计数
- 遇到第一个未过期记录即停止（链表有序性保证后续均有效）
- 返回 `idList.Len() - expiredCount`
- 方法签名更新为 `(int, error)`，Stop 后返回 `ErrDeduplicatorStop`

### 2. 生命周期错误处理完善

**新增 `stopped` 字段** ([dedup.go#L49](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/dedup/dedup.go#L49))：
- `Stop()` 调用后 `stopped` 永久设置为 `true`，不可逆
- 区分 `running`（后台协程是否运行）和 `stopped`（是否已永久停止）

**公共方法签名更新**：
| 方法 | 原签名 | 新签名 |
|------|--------|--------|
| `NewDeduplicatorWithConfig` | `*Deduplicator` | `(*Deduplicator, error)` |
| `Contains` | `bool` | `(bool, error)` |
| `Count` | `int` | `(int, error)` |
| `CleanExpired` | `int` | `(int, error)` |
| `Clear` | 无返回 | `error` |

**错误变量被实际使用**：
- `ErrDeduplicatorStop`：`Stop()` 后所有公共方法返回此错误
- `ErrInvalidConfig`：`NewDeduplicatorWithConfig` 传入负数值配置时返回

**Stop 行为** ([dedup.go#L112-L128](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/dedup/dedup.go#L112-L128))：
- 即使未调用 `Start()`，`Stop()` 也可安全调用，调用后同样进入永久停止状态
- `Start()` 在已停止状态下调用会被静默忽略，无法重启

### 3. 新增测试覆盖 ([dedup_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/dedup/dedup_test.go))

新增 6 个测试用例：
- `TestNewDeduplicatorWithConfig_InvalidConfig`：负数值配置返回 `ErrInvalidConfig`
- `TestCount_ExcludesExpired`：Count 排除已过期但未清理的记录
- `TestCount_AccuracyWithMixedExpiry`：Count 在混合过期场景下的准确性，验证与 `len(idMap)` 的差异
- `TestStop_RejectsAllOperations`：Stop 后 5 个公共方法均返回 `ErrDeduplicatorStop`
- `TestStop_WithoutStart`：未 Start 直接 Stop 后的行为
- `TestStart_AfterStop`：Stop 后无法通过 Start 重启

### 4. 文档更新 ([dedup.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/dedup.md))

新增/更新内容：
- **4.1.1 Count 统计语义**：专门章节说明 Count 与 `len(idMap)` 的区别，附示意图
- **4.5 生命周期与资源回收**：3 状态表（初始态/运行态/已停止）、不可逆停止约定
- **4.6 公共方法签名与错误返回约定**：所有方法签名汇总表、错误检查代码示例
- **6.5 检测去重器停止状态**：优雅关闭示例，展示如何通过 `ErrDeduplicatorStop` 实现消费循环退出
- 更新所有使用示例以适配新的 API 签名
- 测试覆盖说明增加「Count 语义」和「停止状态」类别

---

## 测试结果

```
PASS: 31 / 31 测试全部通过
ok   solocoder-go/internal/dedup    5.669s
```