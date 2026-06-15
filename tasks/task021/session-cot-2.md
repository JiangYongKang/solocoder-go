我来帮你修复这些问题。首先让我创建任务计划。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
让我先读取当前的实现，然后一次性修复所有问题。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore.go
          
现在让我一次性修复所有问题：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我更新测试文件以适配修复后的 API。首先读取当前的测试文件：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
让我重写整个测试文件来适配修复后的 API：





        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我运行测试确保所有测试通过：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          
我发现了问题！二分查找本身是正确的，但测试使用了默认配置（MaxVersions=10），当写入 1000 个版本时，前面的版本都被自动清理了。让我修复这些测试：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\objstore\objstore_test.go
          
changes: undefined
          
现在让我重新运行测试：









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/objstore/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
现在让我更新文档。首先读取当前文档的相关部分：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\objstore.md
          
让我继续读取更多内容，然后一次性更新文档：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\objstore.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\objstore.md
          
现在让我一次性更新整个文档，修复所有需要更新的内容：









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\objstore.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 6 items
          
所有修复工作已完成！以下是修复总结：

## 修复内容

### 1. 配置错误返回修复 - [objstore.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore.go#L57-L72)

**问题**: `NewObjectStoreWithConfig` 接收到无效配置参数时静默替换为默认值，调用方无法感知。

**修复**:
- 函数签名从 `func NewObjectStoreWithConfig(cfg Config) *ObjectStore` 改为 `func NewObjectStoreWithConfig(cfg Config) (*ObjectStore, error)`
- `MaxVersions <= 0` → 返回 `ErrInvalidMaxVersion`
- `CleanupBatchSize <= 0` → 返回 `ErrInvalidBatchSize`
- `CleanupInterval <= 0` → 返回新增加的 `ErrInvalidCleanupInterval`
- `NewObjectStore()` 内部忽略错误（因为默认配置永远有效）

### 2. CleanupAll 语义修复 - [objstore.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore.go#L250-L261)

**问题**: `CleanupAll` 方法名暗示一次性清理全部，但实际按 `CleanupBatchSize` 分批清理。

**修复**:
- 新增内部方法 `cleanupKeyAllLocked(key string)`，一次性清理单个键的所有超出版本
- `CleanupAll()` 现在调用 `cleanupKeyAllLocked`，不受 `CleanupBatchSize` 限制
- 一次调用即可确保所有键的版本数不超过 `MaxVersions`

### 3. 版本查找优化 - [objstore.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore.go#L263-L271)

**问题**: `GetVersion` 和 `Rollback` 使用线性遍历 O(n) 查找版本号。

**修复**:
- 新增内部方法 `findVersionLocked`，使用 `sort.Search` 进行二分查找
- 时间复杂度从 O(n) 降至 O(log n)
- 即使版本号不连续（旧版本被清理），二分查找依然有效

### 4. 单元测试更新 - [objstore_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/objstore/objstore_test.go)

共 **64 个测试用例**，全部通过：
- 新增配置错误测试（`ErrInvalidMaxVersion`、`ErrInvalidBatchSize`、`ErrInvalidCleanupInterval`）
- 新增二分查找测试（1000 个版本查找、500 个版本回滚）
- 更新 `CleanupAll` 测试验证一次性清理语义
- 所有调用 `NewObjectStoreWithConfig` 的测试均处理错误返回

### 5. 文档更新 - [objstore.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/objstore.md)

更新内容包括：
- 配置参数约束说明（必须 > 0）
- 新增版本查找策略章节（二分查找算法与优势）
- 明确 `CleanupAll` 与自动清理的语义区别
- 更新时间复杂度表
- 更新错误定义表，新增 `ErrInvalidCleanupInterval`
- 所有使用示例均更新为正确的错误处理方式
- 注意事项部分补充配置验证和 CleanupAll 语义说明

### 6. 测试运行结果

```
PASS
ok  solocoder-go/internal/objstore  0.238s
```

所有 **64 个测试用例** 全部通过。