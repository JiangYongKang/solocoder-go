让我先查看链式哈希模块的代码，了解当前实现和问题所在。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
让我查找类型定义文件，了解 VirtualNode 和 Snapshot 的实现。


        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\serialize.go
          
现在我已经理解了所有三个问题。让我逐一修复：

1. **Snapshot 方法中 VirtualNode 的 Index 字段缺失** - 需要重新计算每个虚拟节点的 Index
2. **TestConcurrency 缺少状态验证** - 需要在并发操作后验证哈希环的完整性
3. **TestUpdateNodeWeight 缺少权重下降测试** - 需要补充权重降低的测试用例

让我开始修复。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
### 修复问题 1：Snapshot 方法中 VirtualNode 的 Index 字段

问题在于 `vnodeMap` 只存储了 `hash -> nodeID` 的映射，没有存储每个虚拟节点的原始 Index。我们需要通过节点权重重新计算虚拟节点索引。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\serialize.go
          
changes: undefined
          
现在修复问题 2：增强 TestConcurrency 并发测试，添加哈希环状态验证。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
### 修复问题 2：增强 TestConcurrency 测试

需要在并发操作后验证哈希环的完整性，包括节点数量、ring 排序、虚拟节点映射一致性。




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
changes: undefined
          
现在修复问题 3：补充 TestUpdateNodeWeight 权重下降的测试用例。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
### 修复问题 3：补充权重下降的测试用例

需要在 TestUpdateNodeWeight 中添加权重从高到低的测试，验证 removedRanges 逻辑。让我先查看当前测试的结构，然后添加新的测试用例。







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
现在让我添加权重下降的测试用例，放在 "same weight" 测试之后：









        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
changes: undefined
          
现在让我也添加一个测试来验证 Snapshot 中 VirtualNode 的 Index 字段是否正确。让我查看 TestSerialization 的位置并添加相关验证。





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
让我在 "snapshot and restore" 子测试中添加对 VirtualNode Index 字段的验证：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\chainhash\chainhash_test.go
          
changes: undefined
          
现在让我运行测试来验证所有修复是否正确。










        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/chainhash/ -v undefined
          
所有测试全部通过！让我更新任务状态。











        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 链式哈希模块问题修复完成 ✅

所有问题已全部修复，测试结果：`PASS ok solocoder-go/internal/chainhash 0.444s`

---

### 修复内容汇总

#### 1. Snapshot 方法 VirtualNode Index 字段缺失

**文件**: [serialize.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chainhash/serialize.go#L27-L39)

**问题**: 构建 VirtualNode 快照时漏填了 `Index` 字段，所有虚拟节点的 Index 在快照中都是默认零值。

**修复**: 改为遍历每个节点，根据权重计算虚拟节点数量，然后逐个生成并填充 Index 索引，确保快照中每个虚拟节点都有正确的原始生成顺序索引。

同时新增了测试验证：`TestSerialization/snapshot_and_restore/verify_vnode_index_field`

---

#### 2. TestConcurrency 并发测试增强

**文件**: [chainhash_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chainhash/chainhash_test.go#L677-L739)

**问题**: 并发测试只等待 WaitGroup 结束后没有验证哈希环状态，无法发现并发导致的数据错乱。

**修复**: 新增 5 项状态验证：
- **verify node count**: 验证节点数量非负
- **verify ring sorted**: 验证 ring 数组保持有序
- **verify vnodeMap and ring consistency**: 验证 vnodeMap 和 ring 的 hash 一一对应、数量一致
- **verify vnodeMap references valid nodes**: 验证所有虚拟节点都指向存在的节点
- **verify virtual node count matches weight**: 验证虚拟节点总数与权重匹配

---

#### 3. TestUpdateNodeWeight 权重下降测试

**文件**: [chainhash_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/chainhash/chainhash_test.go#L361-L433)

**问题**: 只测试了权重从低到高的提升场景，权重下降的 `removedRanges` 逻辑完全没有覆盖。

**修复**: 新增 `decrease weight` 测试用例：
- 节点权重从 3 降到 1
- 验证权重值正确更新
- 验证虚拟节点数量正确减少
- 验证存在 `FromNode` 为该节点的迁移信息（removedRanges 分支）
- 验证实际 key 迁移比例符合预期（约 25% 的 key 从该节点迁出）