我需要先了解当前项目的结构，然后规划实现环境变量管理器功能模块。让我先查看项目结构。
        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\secenv\secenv.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\secenv\secenv_test.go
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
让我先创建 envmgr 目录和核心代码文件。

        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Path "c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\envmgr" -Force undefined
          
现在让我编写 envmgr 包的核心实现代码。

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\envmgr\envmgr.go
          
changes: undefined
          
我发现代码中有个问题：`GetSensitive` 返回的 `SensitiveValue` 没有设置 AES 密钥，导致解密会失败。让我修复这个问题。

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\envmgr\envmgr.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
现在让我编写完整的单元测试文件。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\envmgr\envmgr_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/envmgr/ -v undefined
          
所有测试都通过了！现在让我编写模块需求文档。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          

        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\envmgr.md
          
changes: undefined
          

        
            
toolName: GetDiagnostics
            
status: success
          
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 5 items
          
## 任务完成总结

已成功实现环境变量管理器（envmgr）功能模块，所有需求均已满足：

### 1. 核心功能实现

**代码位置：** [internal/envmgr/envmgr.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/envmgr/envmgr.go)

- **前缀分组读取**：通过 `LoadGroup(prefix string)` 方法实现，支持按指定前缀过滤环境变量，自动去除前缀后以键值对形式返回
- **类型自动转换**：支持 `string`、`int`、`int64`、`float64`、`bool`、`time.Duration` 六种常用类型，转换失败返回明确的类型错误
- **必填项校验**：通过 `EnvConfig.Required` 标记必填项，加载时检查缺失或为空的变量，一次性返回所有缺失的变量名
- **敏感变量加密存储**：使用 AES-256-GCM 算法加密敏感变量，内存中始终以密文形式存储，必须通过 `GetSensitive()` + `Decrypt()` 才能获取明文

### 2. 核心结构体

| 结构体 | 职责 |
|--------|------|
| `EnvManager` | 管理器核心入口，维护多个环境变量组和加密密钥 |
| `EnvGroup` | 前缀分组的环境变量集合，提供类型安全的访问方法 |
| `EnvConfig` | 单个环境变量的配置（必填、敏感、默认值） |
| `SensitiveValue` | 封装加密后的敏感值，提供安全解密接口 |

### 3. 单元测试

**代码位置：** [internal/envmgr/envmgr_test.go](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/envmgr/envmgr_test.go)

共 33 个测试用例，覆盖：
- 正常流程（分组加载、类型转换、加解密、多分组管理）
- 边界条件（零值、负值、极大值、布尔多种格式、值包含等号）
- 异常分支（缺失必填、类型转换失败、直接读取敏感变量、解密失败）
- 并发安全（多 goroutine 并发读取和解密）

### 4. 测试结果

```
PASS
ok      solocoder-go/internal/envmgr    0.229s
```

所有 33 个测试全部通过 ✅

### 5. 需求文档

**文档位置：** [docs/envmgr.md](file:///C:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/envmgr.md)

文档包含：
- 模块功能概述
- 核心结构体职责说明
- 环境变量分组工作机制
- 加密存储工作流程
- 必填校验机制
- 完整的使用示例（4 个场景）
- 错误定义表
- 并发安全说明
- 测试覆盖范围