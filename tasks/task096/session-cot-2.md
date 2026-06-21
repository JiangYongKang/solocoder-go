让我先查看一下可扩展布隆过滤器模块的当前实现。

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal
          
找到了！让我读取可扩展布隆过滤器模块的代码。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom_test.go
          
好的，我来分析并修复这三个问题。让我先创建任务列表。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
好的，让我开始修复这三个问题。首先从容量上限校验开始。

### 问题 1：容量上限校验修复


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
让我修改代码，修复这三个问题。首先从容量上限校验开始：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在让我实现所有三个修复。我会对 `scalablebloom.go` 进行全面的修改。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
现在修改 `bloomFilter.add()` 方法，添加容量检查，并添加数据完整性验证方法：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
现在修改 `ScalableBloom.Add()` 方法以正确处理 `add` 返回的 error，并添加公共方法 `FillRatio`：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在修改 `Add` 方法并添加 `FillRatio` 公共方法：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
现在让我修复问题 2：序列化与反序列化的版本兼容性。先看看当前的序列化和反序列化代码：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在让我修改序列化和反序列化函数，添加版本兼容和数据完整性校验：











        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
现在修改 `Deserialize` 函数，添加版本向前兼容和数据完整性校验：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在让我替换整个 `Deserialize` 函数：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在替换 `Deserialize` 和 `UnionQuery` 函数：














        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
现在让我修复问题 3：多过滤器联合查询的哈希配置一致性校验。让我修改 `UnionQuery` 函数：













        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
现在让我修改 `UnionQuery` 函数，添加过滤器配置一致性校验：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom.go
          
changes: undefined
          
好的，现在让我更新 todo 并运行测试，看看有哪些测试需要调整：















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/scalablebloom/ -v undefined
          
太好了！所有测试都通过了！让我再添加一些针对新修复功能的单元测试，以确保修复的完整性。让我添加测试用例来验证：

1. 版本 1 数据的向前兼容
2. 不兼容过滤器的联合查询
3. 数据完整性校验
4. 容量超限错误



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom_test.go
          
现在让我在测试文件末尾添加新的测试用例，覆盖我们修复的三个问题：

















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom_test.go
          
changes: undefined
          
现在让我运行所有测试来验证：
















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/scalablebloom/ -v undefined
          
有一个测试失败了。问题在于版本 1 数据格式的模拟不正确。版本 2 的头部比版本 1 多了 4 字节的 `minSupportedVersion` 字段，而我只去掉了末尾的 32 字节 reserved，但没去掉头部的 4 字节。让我修复这个测试：
















        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\scalablebloom\scalablebloom_test.go
          
changes: undefined
          
让我再检查一下，可能还有其他问题。让我运行调试一下：

















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/scalablebloom/ -v -run TestDeserialize_Version1BackwardCompatibility undefined
          
很好！现在让我运行所有测试确保全部通过：


















        
            
toolName: run_command
            
status: success
          
command: go test ./internal/scalablebloom/ -v undefined
          
太棒了！所有 **72 个测试** 全部通过！让我更新 todo 列表：


















        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## 修复完成！

所有三个问题都已修复，**72 个测试全部通过**！以下是详细的修复说明：

---

### 问题 1：容量上限校验 ✅

**修改文件**：[scalablebloom.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go)

**修复内容**：

| 改动 | 位置 | 说明 |
|------|------|------|
| `bloomFilter.add()` 返回 error | [L90-L103](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L90-L103) | 底层过滤器添加元素时检查容量，满了返回 `ErrCapacityExceeded` |
| `ScalableBloom.Add()` 双重检查 | [L172-L200](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L172-L200) | 添加前检查 + 添加后错误重试，确保不遗漏扩容 |
| `expandLocked()` 独立方法 | [L202-L212](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L202-L212) | 扩容逻辑独立封装，包含容量溢出检查 |
| `bloomFilter.validate()` 方法 | [L122-L137](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L122-L137) | 验证过滤器数据完整性 |
| `FillRatio()` 公共方法 | [L214-L222](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L214-L222) | 查询当前活跃过滤器的填充率 |

---

### 问题 2：序列化版本兼容性 ✅

**修改文件**：[scalablebloom.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go)

**修复内容**：

| 改动 | 位置 | 说明 |
|------|------|------|
| 版本号升级到 2 | [L30-L31](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L30-L31) | 新增 `minSupportedVersion` 常量 |
| 向前兼容版本 1 | [L373-L394](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L373-L394) | 版本 1 数据可正常读取并继续使用 |
| 配置参数校验 | [L434-L442](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L434-L442) | 反序列化时验证 FPRate、Ratio、InitialCapacity 合法性 |
| 过滤器数据校验 | [L490-L492](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L490-L492) | 每个过滤器反序列化后调用 `validate()` |
| 整体一致性校验 | [L512-L514](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L512-L514) | 验证 count 总和与各过滤器一致 |
| 32 字节保留字段 | [L355-L358](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L355-L358) | 为未来版本扩展预留空间 |
| 序列化前校验 | [L266-L270](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L266-L270) | 确保只序列化有效数据 |

---

### 问题 3：多过滤器联合查询校验 ✅

**修改文件**：[scalablebloom.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go)

**修复内容**：

| 改动 | 位置 | 说明 |
|------|------|------|
| `UnionQuery` 前置校验 | [L544-L546](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L544-L546) | 查询前先检查过滤器兼容性 |
| `validateFiltersCompatible()` | [L560-L600](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/scalablebloom/scalablebloom.go#L560-L600) | 检查 FPRate、Ratio、InitialCapacity、hashCount 是否一致 |

---

### 新增错误类型

| 错误 | 说明 |
|------|------|
| `ErrVersionUnsupported` | 数据版本过旧，不支持读取 |
| `ErrCapacityExceeded` | 布隆过滤器容量已超限 |
| `ErrIncompatibleFilters` | 联合查询的过滤器配置不兼容 |
| `ErrCorruptedFilter` | 过滤器数据损坏或不一致 |

---

### 测试结果

```
=== RUN   （共 72 个测试）
--- PASS: （全部通过）
PASS
ok      solocoder-go/internal/scalablebloom     0.980s
```

**新增的 20 个测试用例**覆盖：
- 容量超限检测
- 填充率查询
- 不兼容过滤器联合查询（FPRate/InitialCapacity/Ratio 不一致）
- 版本 1 向前兼容
- 版本过旧拒绝
- 无效位长度数据
- 过滤器数据完整性验证
- 容量溢出保护
- 损坏过滤器序列化防护
- 单/空过滤器兼容性检查