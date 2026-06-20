让我先探索项目结构，了解代码风格和约定，然后规划实现方案。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\tracectx\tracectx.go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/*.go
          
让我先看看几个现有模块的代码风格，特别是测试代码和文档的格式。

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\retry\retry_test.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\retry.md
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          
让我再看看 serialize 包的实现，作为序列化的参考。


        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\json.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\serialize\serialize.go
          
现在我对项目结构和代码风格有了充分了解。让我创建任务计划并开始实现。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          
让我先移除未使用的导入占位符，然后做一次编译检查。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          

        
            
toolName: run_command
            
status: success
          
command: go build ./internal/snaptest/ undefined
          
编译成功！现在开始编写单元测试。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
现在运行测试看看是否通过。





        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
有一个未使用的变量，让我修复它。


        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
还有另一个未使用的变量需要修复。





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
再次运行测试。





        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
有一个测试失败了。问题是 `NewWithConfig` 在用户显式传入 `UpdateMode: false` 时仍然用环境变量覆盖了它。让我修复这个设计问题——环境变量检查应该只在 `New()` 中进行，`NewWithConfig` 应该尊重用户显式传入的配置。



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
让我修改逻辑：环境变量检查只在 `New()` 中进行，`NewWithConfig` 完全信任用户传入的配置。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest.go
          
changes: undefined
          
现在需要修改对应的测试，因为环境变量逻辑移到了 `New()` 中。







        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\snaptest\snaptest_test.go
          
changes: undefined
          
再次运行测试。









        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          
所有测试都通过了！现在开始编写文档。







        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\snaptest.md
          
changes: undefined
          
让我再次确认测试全部通过，然后标记所有任务完成。








        
            
toolName: run_command
            
status: success
          
command: go test ./internal/snaptest/ -v undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
快照测试工具模块已全部实现完成，所有测试通过。以下是完成情况汇总：

## 交付文件

### 1. 核心模块 [snaptest.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest.go)

实现了以下核心功能：

| 功能 | 实现说明 |
|------|---------|
| **数据序列化** | `Serialize()` 使用格式化 JSON（2 空格缩进，禁用 HTML 转义），支持任意可 JSON 序列化的数据结构 |
| **快照文件读写** | 快照存储于 `__snapshots__/` 目录，以 `<name>.snap` 命名，自动创建子目录，换行符归一化 |
| **自动比对** | `Match()` 首次运行自动创建快照，后续运行自动比对，完全一致才通过 |
| **LCS Diff 算法** | `Diff()` 基于经典最长公共子序列算法实现行级差异计算，时间复杂度 O(N×M) |
| **差异可视化** | `DiffResult.Format()` 输出类 unified diff 格式：`-` 标记删除行，`+` 标记新增行，`  ` 标记相同行，默认保留 3 行上下文 |
| **快照更新模式** | 支持 `Config{UpdateMode: true}` 或环境变量 `SNAPTEST_UPDATE=1`，更新模式下直接覆写快照不做比对 |
| **安全防护** | 快照名称校验，防止 `../` 路径遍历攻击 |
| **便捷 API** | `Assert(t, name, v)` 与 Go testing 框架无缝集成 |

### 2. 单元测试 [snaptest_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/snaptest/snaptest_test.go)

共 **40+** 个测试用例，覆盖：

- 序列化：nil、基本类型、结构体、嵌套结构体、Slice、Map、HTML 非转义
- Diff 算法：完全相同、完全不同、部分变更、尾部新增、头部删除、空字符串、单行、Windows 换行符、行号正确性
- 差异格式化：无差异、有差异、上下文行数、负上下文处理
- Matcher 配置：默认值、归一化、环境变量读取、显式配置优先级、Config 返回拷贝
- 路径安全：有效路径、子目录、空名称、路径遍历防护
- Match 流程：首次创建快照、已有匹配、不匹配报告、更新模式、无效名称
- Update/Assert/便捷函数：直接更新、无效名称、断言通过

### 3. 模块文档 [snaptest.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/snaptest.md)

包含模块概述、核心结构体职责、快照存储格式、Diff 算法原理、差异展示格式说明（含完整示例）、更新机制、7 个使用示例、错误定义、配置说明、安全防护、最佳实践等 12 个章节。

## 测试结果

```
PASS
ok      solocoder-go/internal/snaptest  0.927s
```

全部 **40+** 个测试用例通过。