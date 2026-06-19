我来帮你实现这个基于租约的 Leader 选举模块。首先让我了解一下项目结构和现有的 distlock 包，因为 Leader 选举通常会依赖分布式锁。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: internal/distlock/*.go
          
让我再看看几个其他模块的实现来了解项目的代码风格和测试模式：

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\distlock\distlock_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
好的，我已经了解了项目结构和代码风格。现在开始实现 Leader 选举模块。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在开始实现 Leader 选举模块。首先创建包目录和核心文件：


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect.go
          
changes: undefined
          
现在开始编写单元测试文件：



        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect_test.go
          
changes: undefined
          
现在让我们运行测试看看是否通过：

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=1 undefined
          
有一个测试失败了。让我修复一下。问题是选举器启动后需要等待第一个检查周期才会开始选举，我来修改实现让它启动时立即进行一次检查，同时也调整测试的等待时间：



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect.go
          
让我修改启动逻辑，启动时立即进行一次检查：



        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\leaderelect\leaderelect.go
          
changes: undefined
          
现在让我们再运行一次测试：




        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=1 -run TestLeaderElector_LeaderRecoversAsFollower undefined
          
好的，那个测试通过了。现在让我们运行所有测试：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=1 undefined
          
所有测试都通过了！让我们再用 race detector 运行一次确保没有竞态条件：





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -race -count=1 undefined
          
好的，race detector 在这个平台不支持，没关系。测试都通过了。现在让我们开始编写文档：







        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先看看现有的文档格式，保持风格一致：






        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\pubsub.md
          
好的，我了解了文档的风格。现在让我编写 Leader 选举模块的需求文档：









        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\leaderelect.md
          
changes: undefined
          
让我们再运行一次测试，确保所有功能正常：










        
            
toolName: run_command
            
status: success
          
command: go test ./internal/leaderelect/ -v -count=3 undefined
          
所有测试都通过了，连续运行 3 次都很稳定。让我也检查一下代码诊断：












        
            
toolName: GetDiagnostics
            
status: success
          
          
代码诊断也没有问题。让我完成最后的任务状态更新：













        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 完成总结

我已成功实现了基于租约的 Leader 选举模块。以下是完成的内容：

### 📁 项目文件

1. **[leaderelect.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/leaderelect/leaderelect.go)** - 核心实现
2. **[leaderelect_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/leaderelect/leaderelect_test.go)** - 单元测试（22 个测试用例）
3. **[leaderelect.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/leaderelect.md)** - 需求文档

### ✅ 实现的功能

1. **候选者竞选**：多个候选者同时争抢，基于分布式锁确保只有一个 Leader，避免脑裂
2. **心跳续约**：Leader 定期心跳续约，每次续约将租约时间顺延一个租约周期
3. **自动重选举**：Follower 定期检查 Leader 状态，检测不到心跳则触发新一轮选举
4. **事件回调通知**：支持 6 种事件类型的回调通知
   - `EventBecomeLeader` - 成为 Leader
   - `EventBecomeFollower` - 成为 Follower
   - `EventElectionStart` - 选举开始
   - `EventElectionEnd` - 选举结束
   - `EventHeartbeat` - 心跳续约成功
   - `EventLeaderLost` - 失去 Leader 身份

### 🧪 测试覆盖

所有 **22 个测试用例全部通过**，覆盖了：
- 基本功能测试（单节点选举、多节点选举、心跳续约）
- 边界条件测试（配置验证、空参数、重复启动/停止）
- 异常分支测试（Leader 故障重选举、主动辞职、脑裂防护）
- 回调机制测试（多回调、事件顺序）

运行命令：`go test ./internal/leaderelect/ -v`