让我先查看文档当前状态，确认需要补充的具体内容：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
文档的 JobStatus 部分已经有较好的说明了。让我继续查看监控统计和 API 部分：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
监控统计部分已经有了更新。让我继续查看文档，确保所有内容都与代码一致：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
文档的大部分内容已经更新。让我补充完善细节，特别是等待阶段说明、状态流转规则表、GetJobStatus 区分示例和统计计数的正式定义：








        
            
toolName: edit_file_search_replace
            
status: failed
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
          
让我重新读取文档中生命周期阶段说明的准确内容：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
现在我知道文档的准确内容了。让我逐个进行修改：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
我需要在状态机流转总结和优先级排序之间加上「4. 核心算法与策略」的标题：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
现在让我在监控与统计部分之前，添加一个专门的统计 API 语义定义章节，确保 CompletedCount 和 FailedCount 的精确定义：















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
好的，监控与统计部分已经写得不错了。让我在这一节前面加一个专门的 API 语义定义小节，然后补充专门的 GetJobStatus 区分示例。先让我检查 5.4 节的完整内容然后再强化：

















        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
监控与统计部分已经写得不错了。让我强化它，加入更正式的 CompletedCount/FailedCount 语义定义，然后在 5.4 节末尾补充一个 GetJobStatus 用于监控告警的实用示例：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\jobqueue.md
          
changes: undefined
          
文档更新全部完成。让我确认文档内容与实际代码完全一致，进行最终检查：











        
            
toolName: search_by_regex
            
status: success
          
query: CompletedCount|FailedCount|successResults|deadLetterResults|JobStatusPending|JobStatusFailed
          
文档已全部更新完成。总结一下文档的改动：

---

## ✅ 文档补充完成

### 1. 任务状态机中 Pending 与 Failed 的语义区分

**更新位置**：[docs/jobqueue.md#L185-L197](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/jobqueue.md#L185-L197)（生命周期 → 等待阶段）

| 特征 | `JobStatusPending` | `JobStatusFailed` |
|------|-------------------|-------------------|
| 语义 | **首次等待执行** | **执行失败后等待重试** |
| Handler 执行次数 | 从未执行过 | 至少执行过 1 次 |
| RetryCount | 恒等于 0 | ≥ 1 |
| Error 字段 | 恒为 nil | 保存上一次失败的 error |
| 迁移来源 | 只能来自新建任务入队 | 只能来自 Running 执行失败（未超重试上限） |

**区分方式**：通过 `GetJobStatus(jobID)` 返回值直接判断，不依赖任务所在的堆类型（优先级/延迟队列），仅由 `Status` 字段决定。文档中补充了使用示例（第 189-196 行）和 switch-case 多路监控告警示例（第 466-481 行）。

**新增章节 3.1**：[docs/jobqueue.md#L215-L229](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/jobqueue.md#L215-L229)，以表格形式完整呈现 6 种状态迁移路径，含 RetryCount 和 Error 字段的变化规则。

---

### 2. CompletedCount 与 FailedCount 的语义定义

**新增章节 5.4.1 和 5.4.2**：[docs/jobqueue.md#L395-L420](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/jobqueue.md#L395-L420)

#### 结果存储机制
- `successResults`：仅存 `JobStatusCompleted`（Handler 返回 nil error）
- `deadLetterResults`：仅存 `JobStatusDeadLetter`（RetryCount > MaxRetries）
- 两个 map 互斥，无重复计数

#### 统计方法语义表

| 方法 | 精确语义 | 计数来源 |
|------|----------|----------|
| `CompletedCount()` | **成功完成**的任务总数（仅 Handler 返回 nil error 的） | `len(successResults)` |
| `FailedCount()` | **失败且已放弃**的任务总数（仅已进入死信队列的） | `len(deadLetterResults)` |
| `PendingCount()` | 所有未开始执行的任务总数（含 Pending + Failed 等待状态的） | 两个堆长度之和 |
| `ActiveCount()` | 正在执行中的任务数（Running 状态） | 原子计数器 |
| `DeadLetterCount()` | 死信队列中的任务结构体数 | `len(deadLetters)` |

#### 关键恒等式

```
CompletedCount() + FailedCount() = 总已结束任务数
```

文档第 5.4.3 节（第 422-485 行）补充了：
- 任务总数一致性校验示例
- 死信队列超过阈值自动告警的监控示例
- GetJobStatus 多路状态分流示例
- 两者之和验证总已结束任务数的代码片段

文档描述与代码实现（`successResults` / `deadLetterResults` 双 map 分离存储、`CompletedCount` 返回 `len(successResults)`、重试任务状态设为 `Failed`）完全一致。