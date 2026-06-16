我来分析并修复这两个问题。首先查看当前代码：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
现在我清楚了这两个问题。让我开始修复：

**问题 1**：第 285 行的 `time.Now().Add(opts.TTL)` 和第 313 行的 `CreatedAt: time.Now()` 使用了两次独立调用，存在时间偏差。

**问题 2**：第 505-514 行的全量扫描 fallback 路径永不可达（因为所有密钥都已注册到 byPrefix 索引），是无意义的 O(n) 遍历。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
首先修复 **问题 1**：在 `CreateKey` 开头统一捕获时间基准，确保 TTL 计算和 CreatedAt 使用同一时间点：

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
现在修复 **问题 2**：移除 `VerifyKey` 中永不可达的全量扫描 fallback。由于所有密钥都注册在 `byPrefix` 索引中，前缀不匹配意味着密钥一定不存在，无需 O(n) 全量遍历：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          
changes: undefined
          
现在我还优化了 `VerifyKey` 中的锁使用：将前缀查找和哈希匹配合并到同一个 `RLock` 中，避免原来每个候选 key 都单独加锁的开销。

接下来调整单元测试。先查看相关测试：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: search_by_regex
            
status: success
          
query: full.?scan|FullScan|verify_key_with
          

        
            
toolName: search_by_regex
            
status: success
          
query: TestCreateKeyWithTTL|TestCreateKeyWithExpiresAt|CreatedAt|ExpiresAt
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
现在我来调整测试。需要做两件事：
1. 加强 `TestCreateKeyWithTTL` 的时间一致性断言，精确验证 `ExpiresAt - CreatedAt == TTL`
2. 重命名并修改 `verify key with full scan` 测试，因为全量扫描已被移除



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
changes: undefined
          
现在修改 `verify key with full scan` 测试，并增加对短密钥（无法提取前缀）直接返回无效的验证：




        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
changes: undefined
          
现在运行测试验证所有修改是否正确：




        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/apikey/ -v -timeout 120s undefined
          
所有 77 个测试全部通过！🎉 让我更新任务状态：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
## ✅ 修复完成

两个问题均已修复，所有 77 个测试全部通过。

---

### 修复 1: CreateKey 时间基准不一致

**文件**: [apikey.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/apikey/apikey.go#L263-L316)

**问题**: `CreateKey` 中 `time.Now()` 被调用了两次 —— 一次用于 TTL 过期计算（`expiresAt = time.Now().Add(opts.TTL)`），一次用于记录创建时间（`CreatedAt: time.Now()`）。两次调用之间存在微小时间差，导致短 TTL 场景下实际可用时长与用户设置不一致。

**修复**: 在函数开头统一捕获一次时间基准 `now := time.Now()`，所有时间相关计算（TTL 过期计算、ExpiresAt 校验、CreatedAt 赋值）都使用这同一个时间点，确保 `ExpiresAt - CreatedAt == TTL` 精确成立。

**测试验证**: 加强了 `TestCreateKeyWithTTL` 的断言，从原来宽松的"23~25 小时范围"改为精确相等断言 `meta.ExpiresAt.Sub(meta.CreatedAt) == ttl`。

---

### 修复 2: 移除 VerifyKey 全量扫描 fallback

**文件**: [apikey.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/apikey/apikey.go#L475-L508)

**问题**: 当前缀索引 `byPrefix` 未命中时，代码会退化为遍历全部密钥做哈希匹配（O(n) 全量扫描）。但由于每个密钥在 `CreateKey` 时都同步注册到 `byPrefix` 索引，前缀不匹配意味着密钥一定不存在，全量扫描永远不会产生有效结果，纯属鸡肋且在密钥数量大时浪费性能。

**修复**: 
- 移除了全量扫描 fallback 代码
- 如果密钥长度不足以提取前缀，直接返回 `ErrInvalidSecret`
- 前缀索引未命中时，直接返回无效，不再做无意义的全量遍历
- 附带优化：将前缀索引查找和哈希匹配合并到同一个 `RLock` 范围内，避免原来每个候选 key 都单独加解锁的开销

**测试调整**:
- 将 `"verify key with full scan"` 测试重命名为 `"verify key via prefix index"`，验证正常前缀索引查找
- 新增 `"verify key with too short secret"` 测试，验证两类场景：
  - 密钥太短无法提取前缀 → 直接返回 `ErrInvalidSecret`
  - 前缀不匹配（修改前缀字符） → 直接返回 `ErrInvalidSecret`

---

### 测试结果

```
PASS
ok  solocoder-go/internal/apikey  0.678s
```

共 **77 个**测试（含子测试）全部通过。