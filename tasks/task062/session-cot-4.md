我来修复文档中的这两个问题。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\quotamgr.md
          
changes: undefined
          
现在让我也同步更新 8.6 节性能考量部分，确保一致性：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/quotamgr/ -v -count=1 undefined
          
所有测试都通过了！让我完成任务标记。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
## ✅ 所有文档问题已修复完成

### 修复内容总结：

---

### 1. **链式箭头释放时机标注** ([docs/quotamgr.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/quotamgr.md#L523-L545))

**问题**：原链式 `全局写锁 → 租户级锁 → 全局读锁` 未标注释放节点，易被误读为三把锁嵌套同时持有。

**修复**：
- 在链式前添加明确说明：「以下链式表示步骤发生的先后顺序，每一步结束后锁即释放，并非三把锁同时嵌套持有」
- 将链式重写为带编号的完整生命周期：
  ```
  ① 获取全局写锁 → ① 释放全局写锁 → ② 获取租户级锁 → ③ 获取全局读锁 → ③ 释放全局读锁 → ② 释放租户级锁
  ```
- 新增「以 `AcquireResource` 为例的实际持有窗口」段落，说明三个阶段分别持有的锁组合：
  - 阶段①（`getOrCreateTenantUsage` 内部）：仅持有全局写锁，函数返回前释放
  - 阶段②③之间：同时持有「租户级锁」和「全局读锁」（共享锁嵌套，安全）
  - 阶段③之后：仅持有租户级锁，完成使用量更新后释放

---

### 2. **示例函数调用关系补充** ([docs/quotamgr.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/quotamgr.md#L536-L545))

**问题**：原规则1 以 `getOrCreateTenantUsage`、`getTenantUsageIfExists` 作为示例，但这两个辅助函数体内并未获取租户级锁，完整锁模式需要调用方配合，易引起维护者困惑。

**修复**：
- **规则1（允许：先全局锁→再租户级锁）**：
  - 补充说明：「辅助函数仅负责全局锁操作，租户级锁由调用方在辅助函数返回后单独获取」
  - 示例改为完整调用关系：
    - `ReleaseResource` 先调用 `getTenantUsageIfExists()`（内部持有全局读锁→释放），随后调用方获取租户级写锁
    - `AcquireResource` 先调用 `getOrCreateTenantUsage()`（内部可能持有全局写锁→释放），随后调用方获取租户级写锁
- **规则2（允许：先租户级锁→再全局读锁）**：补充具体操作场景描述
- **规则3（禁止）**：补充死锁场景的具体风险说明

---

### 测试结果
所有 **43 个测试全部通过** ✅