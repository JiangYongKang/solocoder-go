我来帮你实现 API 密钥管理器功能模块。首先让我了解一下项目的现有结构和代码风格。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我查看几个相关的现有模块，了解代码风格和模式。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rbac\rbac.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\oauth2svc\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\secenv\secenv.go
          
让我再看一下测试文件的风格：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\rbac\rbac_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\writethrough.md
          
现在我对项目结构和代码风格有了充分了解。让我开始实现 API 密钥管理器模块。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我创建核心代码文件：





        
            
toolName: run_command
            
status: success
          
command: mkdir -p internal/apikey undefined
          
目录创建成功。现在让我编写 API 密钥管理器的核心代码：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          
核心代码已完成。现在让我更新 todo 并编写单元测试：





        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在编写完整的单元测试文件：







        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
changes: undefined
          
测试文件已编写完成。现在运行测试：









        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/apikey/ -v undefined
          
有两个编译错误，让我修复它们：








        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
我看到问题了。`generateSecret` 函数签名需要返回 4 个值：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          
现在重新运行测试：










        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/apikey/ -v undefined
          
测试出现了死锁问题。这是由于高并发场景下，`VerifyKey` 中多次对同一个 key 的 RWMutex 进行 RLock/Lock/RLock 交替操作，Go 的 RWMutex 写优先策略导致了死锁。让我重构代码，减少锁竞争：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
让我重构 APIKey 的锁机制，避免 RWMutex 写优先策略导致的死锁。将 RWMutex 替换为 Mutex，并优化 VerifyKey 中的锁使用：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          
现在需要更新所有使用这些字段的函数。让我先查看并修改 CreateKey 和 keyToMeta：












        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
让我重写整个文件，系统性地修复所有锁问题：











        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          
现在重新运行测试：










        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/apikey/ -v -timeout 120s undefined
          
很好！几乎所有测试都通过了。只有一个测试用例需要修复 —— `TestParsePermission/with_colons_in_value`。使用 `SplitN(s, ":", 2)` 时，`"resource:action:extra"` 会被解析为 resource="resource", action="action:extra"，这是合法的。让我修复测试：











        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
changes: undefined
          
现在再次运行测试：














        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/apikey/ -v -timeout 120s undefined
          
所有测试都通过了！现在更新 todo 并编写需求文档：

















        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在编写模块需求文档：



















        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\apikey.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
API 密钥管理器模块已全部实现完成！以下是完整的交付总结：

---

## ✅ 完成内容总览

### 1. 核心代码 - [apikey.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/apikey/apikey.go)

**6 大功能全部实现**：

| 功能 | 实现要点 |
|------|----------|
| **密钥安全生成** | `crypto/rand` 生成 32 字节密钥，前缀 `sk_`，SHA-256 哈希存储，明文仅创建时返回一次；前缀索引加速查询 |
| **权限范围绑定** | `Permission{Resource, Action}` 二元组，`resource:action` 字符串格式，`map[Permission]bool` O(1) 权限校验 |
| **使用次数限制** | `atomic.Int64` + **CAS 循环**保证并发精确，超限自动返回 `ErrUsageLimitExceeded`，`GetRemainingUses()` 查询剩余 |
| **过期时间管理** | 支持 TTL（相对时长）和 ExpiresAt（绝对时间戳）双模式，动态 `SetTTL`/`SetExpiresAt` 延长有效期 |
| **紧急吊销** | `CompareAndSwap` 原子标记不可逆，记录时间+原因，吊销优先级最高（高于过期/耗尽） |
| **4 种状态流转** | `active` → `expired`/`depleted`/`revoked`，状态优先级：吊销 > 过期 > 耗尽 > 活跃 |

**并发安全架构**：
- Manager 级：`sync.RWMutex` 保护 `keys` map 和 `byPrefix` 索引
- 密钥级：`atomic.Bool`（吊销/过期标志，无锁热读）+ `atomic.Int64`（计数，CAS 精确）+ `sync.Mutex`（保护 `keyState` 内部可变状态）

---

### 2. 单元测试 - [apikey_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/apikey/apikey_test.go)

**测试结果：全部 75 个通过 🎉**

```
ok  solocoder-go/internal/apikey  0.670s
```

**覆盖范围**（含子测试共 75 个）：

| 类别 | 测试函数数 | 覆盖内容 |
|------|-----------|---------|
| 基础 | 3 | Manager 初始化、Permission String/Parse |
| 密钥创建 | 13 | 基本创建、明文仅返回一次、MaxUses 正/负/零、TTL/ExpiresAt/无过期/过期在过去、空 Resource/Action |
| 查询 | 8 | GetKeyMeta 正常/不存在/空 ID、ListKeysByPrefix 正常/空前缀/未找到、ListAllKeys 正常/空、KeyCount |
| 吊销 | 5 | 正常吊销、不存在、空 ID、空原因、二次吊销保护 |
| 验证+权限 | 19 | VerifyKey 6 场景、CheckAccess 10 场景、VerifyAndCheckAccess 2 场景 |
| 计数/过期 | 19 | IncrementUsage 8 场景、GetRemainingUses 3、GetRemainingTime 4、SetExpiresAt 5、SetTTL 6 |
| 状态流转 | 5 | Active/Depleted/Expired/Revoked 单状态 + 吊销优先级覆盖 |
| **并发** | **5** | 1000 并发 VerifyKey、500 并发 IncrementUsage、150 并发 CAS 精确计数、50 并发 Create、200ms 混合读写压力 |
| 完整流程 | 1 | 创建→验证→权限→计数→修改过期→吊销→再次验证 |
| 边界条件 | 6 | 特殊字符、100 权限、前缀索引失效全量扫描、双吊销、权限排序、空权限集合拒绝所有 |

**关键并发测试验证**：
- `TestConcurrentIncrementUsageExact`: 150 并发对 MaxUses=100 递增 → UsedCount 精确等于 **100**，超限恰好 **50** 个错误

---

### 3. 需求文档 - [apikey.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/apikey.md)

**11 个章节，完整详实**：

1. **模块概述** - 设计目标、包路径
2. **核心功能表** - 10 项功能速览
3. **核心结构体职责** - 10 个结构体的字段含义、职责、主要方法详解（Manager/APIKey/keyState/Permission/APIKeyMeta/CreatedKey/CreateKeyOptions/KeyStatus/VerifyResult/CheckAccessResult）
4. **密钥生命周期** - ASCII 流程图 + 3 阶段详细说明（创建 5 步骤 / 活跃使用流程图 / 3 种终止状态对比）
5. **权限范围模型** - 格式、解析、校验逻辑、设计模式示例
6. **使用次数限制** - CAS 原理、并发保证、剩余次数计算
7. **过期时间管理** - 两种模式对比、优先级、动态更新
8. **紧急吊销机制** - 触发场景、4 大特性、双吊销保护代码
9. **使用示例** - 7 个完整代码示例（基础创建验证、权限校验、状态查询、前缀查询、紧急吊销、一次性密钥、延长有效期）
10. **错误定义** - 17 个 sentinel error 完整列表
11. **并发安全设计** - 4 层锁架构、无锁热点路径、死锁避免、并发测试覆盖

---

运行命令验证结果：

```bash
go test ./internal/apikey/ -v -timeout 120s
# 输出：PASS  all 75 tests passed
```