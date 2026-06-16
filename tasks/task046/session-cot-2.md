让我先查看相关代码来确认这些问题：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\types.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\errors.go
          
让我创建待办事项列表，然后逐一修复这些问题：


        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
现在开始修复问题。首先从 **问题 1**：修复 `VerifyCertificateChain` 中 `IntermediateCAs` 无效死代码，以及 **问题 5**：添加 `ErrLoaderNil` 错误变量。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
先修改 `errors.go` 添加 `ErrLoaderNil` 错误：


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\errors.go
          
changes: undefined
          
现在修改 `certrotator.go` 中的多个问题。首先修复 **问题 5**（New 函数的 loader 错误）和 **问题 1**（IntermediateCAs 无效死代码）和 **问题 4**（ActiveConnections 用写锁）：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          
changes: undefined
          
现在修复 **问题 1**：`VerifyCertificateChain` 中 `IntermediateCAs` 无效死代码。由于 `x509.CertPool` 无法直接读取内部证书，我们采用将证书链中间证书添加到配置池的策略：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          
changes: undefined
          
现在修复 **问题 4**：`ActiveConnections` 使用写锁做纯读取。需要将 `connMu` 改为 `sync.RWMutex`：





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          
changes: undefined
          
现在修改 `ActiveConnections` 使用读锁：







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          
changes: undefined
          
现在修复 **问题 3**：`ConnectionTracker` 接口与方法签名不匹配。更新 `types.go` 中的接口定义：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\types.go
          
changes: undefined
          
现在修复 **问题 2**：`renewCertificate` 紧耦合签发和切换。拆分签发和切换逻辑，让 `pendingCert` 有真正的"待切换"状态：








        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
修改 `renewCertificate` 只签发不切换，修改 `checkAndRenew` 在续期成功后调用切换：






        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator.go
          
changes: undefined
          
现在需要为这些问题补充单元测试。先让我读取现有的测试文件：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
让我读取测试文件的结尾部分：







        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
现在让我在测试文件末尾添加新的测试用例。先查看 `TestNewWithNilIssuer` 测试的写法：








        
            
toolName: search_by_regex
            
status: success
          
query: func TestNewWithNil
          
让我检查现有的 `TestForceRenew` 测试，因为拆分签发和切换后，`ForceRenew` 的行为会变化：










        
            
toolName: search_by_regex
            
status: success
          
query: func TestForceRenew
          
我看到 `ForceRenew` 现在只签发不切换了，需要更新相关的测试。让我先完整读取需要修改的测试：









        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
现在让我修改现有测试并添加新测试。首先更新 `TestNewWithNilLoader`：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在更新 `TestForceRenew` 测试，因为 `ForceRenew` 现在只签发不切换了：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在更新 `TestMultipleRenewals` 测试：










        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在添加新的测试用例。首先添加一个生成三级证书链（根 -> 中间 -> 叶子）的辅助函数和新测试：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在运行测试看看结果：












        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/certrotator/ -v undefined
          
有 3 个测试失败了，都是因为 `ForceRenew` 现在只签发不切换了。让我查看这些测试并修复：











        
            
toolName: search_by_regex
            
status: success
          
query: func TestCompleteRetirement
          
让我读取这三个失败测试的完整内容：










        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
现在修复这三个失败的测试，在 `ForceRenew` 后添加 `ForceSwitch` 调用：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在修复 `TestForceRetirement` 测试：












        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在修复 `TestEventHandler` 测试。先看看它的内容：














        
            
toolName: search_by_regex
            
status: success
          
query: func TestEventHandler
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在再次运行测试：









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/certrotator/ -v undefined
          
还有一个测试失败了：`TestForceRenew_PendingStatePersists`。问题是两个证书生成太快导致 SerialNumber 相同。让我修复这个测试，使用更可靠的方式判断：













        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\certrotator\certrotator_test.go
          
changes: undefined
          
现在再次运行测试：











        
            
toolName: run_command
            
status: success
          
command: go test ./internal/certrotator/ -v undefined
          
完美！所有 51 个测试全部通过！让我更新任务状态：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 7 items
          
## ✅ 所有问题已修复，51 个测试全部通过！

### 🔧 修复内容总结

#### 1. **IntermediateCAs 无效死代码修复** ✅
- **文件**: [certrotator.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator.go#L167-L178)
- **问题**: 原代码仅遍历 `Subjects()` 但不做任何事，配置的中间证书完全被忽略
- **修复**: 当配置了 `IntermediateCAs` 时，将证书链中的中间证书添加到配置池中，使用合并后的池进行验证
- **新增测试**: 
  - [TestVerifyCertificateChain_WithConfigIntermediates](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L1468-L1493)
  - [TestVerifyCertificateChain_WithoutConfigIntermediatesFails](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L1495-L1519)

#### 2. **签发与切换解耦** ✅
- **文件**: [certrotator.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator.go#L282-L304)
- **问题**: `renewCertificate()` 将签发和切换紧耦合，`pendingCert` 从未真正处于"待切换"状态
- **修复**: 
  - `renewCertificate()` 只负责签发证书并设置为 `pending` 状态
  - `checkAndRenew()` 在续期成功后调用 `switchToPendingCert()` 完成切换
  - `ForceRenew()` 只签发不切换，调用者可通过 `ForceSwitch()` 控制切换时机
- **新增测试**: [TestForceRenew_PendingStatePersists](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L1584-L1646)

#### 3. **ConnectionTracker 接口签名匹配** ✅
- **文件**: [types.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/types.go#L55-L59)
- **问题**: 接口定义 `TrackConnection(certID string) func()` 与实际方法签名不一致
- **修复**: 更新接口为 `TrackConnection(certID string, conn interface{}, closeFn func() error) func()`
- **新增测试**: [TestConnectionTracker_InterfaceSatisfied](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L1521-L1532)

#### 4. **ActiveConnections 读锁优化** ✅
- **文件**: [certrotator.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator.go#L439-L446)
- **问题**: `ActiveConnections()` 使用互斥写锁做纯读取，高并发下性能差
- **修复**: 将 `connMu` 改为 `sync.RWMutex`，读操作使用 `RLock()/RUnlock()`
- **新增测试**: [TestActiveConnections_ConcurrentRead](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L1534-L1570)

#### 5. **ErrLoaderNil 专用错误** ✅
- **文件**: [errors.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/errors.go#L20)
- **问题**: `loader` 为 nil 时返回 `ErrLoadCertificateFailed`，错误语义不匹配（对比 issuer 有专用的 `ErrIssuerNil`）
- **修复**: 新增 `ErrLoaderNil` 错误变量，`New()` 函数中 loader 为 nil 时返回该错误
- **更新测试**: [TestNewWithNilLoader](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/certrotator/certrotator_test.go#L228-L237)

### 📊 测试结果

```
PASS: 51/51 测试全部通过
运行时间: ~2秒
- 原有测试: 45 个（已适配新行为）
- 新增测试: 6 个
```