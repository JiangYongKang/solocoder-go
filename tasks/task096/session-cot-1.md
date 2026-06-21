我来帮你实现这个 MIME 类型检测器功能模块。首先让我了解一下当前项目的结构。
        
            
toolName: view_folder
            
status: success
          
c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go
          

        
            
toolName: file_search
            
status: success
          
file_pattern: go.mod
          

        
            
toolName: file_search
            
status: success
          
file_pattern: **/go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\go.mod
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\apikey\apikey_test.go
          
现在我了解了项目结构和代码风格。让我创建一个详细的 Todo 列表来规划这个 MIME 类型检测器模块的实现。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
让我开始执行任务。首先创建包目录，然后实现核心代码。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          

        
            
toolName: run_command
            
status: success
          
command: New-Item -ItemType Directory -Force -Path "internal/mimedetect" undefined
          
目录已创建。现在让我开始编写核心代码。首先创建主文件，包含所有核心数据结构和功能。


        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
现在我来编写完整的 `mimedetect.go` 文件，包含核心数据结构、内置签名库、映射表和所有功能。


        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在让我更新 Todo 列表并开始编写完整的单元测试。

        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
现在编写完整的单元测试文件：



        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
changes: undefined
          
现在让我运行测试，确保所有测试通过。



        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          

        
            
toolName: run_command
            
status: success
          
command: go test ./internal/mimedetect/ -v undefined
          
有两个测试失败了。让我修复这两个问题：

1. 空文件读取时的 EOF 处理
2. 可变偏移匹配的边界条件



        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
我发现了问题所在。让我修复这两个问题：

1. 空文件读取时的 EOF 处理（需要导入 io 包）
2. 测试中可变偏移边界条件的问题（需要调整测试逻辑）





        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect.go
          
changes: undefined
          
现在修复测试中的问题。问题是第二次注册后，第一个 Offset=3 的签名仍然存在并会先匹配成功。让我修改测试：





        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\mimedetect\mimedetect_test.go
          
changes: undefined
          
现在再次运行测试：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/mimedetect/ -v undefined
          
太好了！`mimedetect` 包的所有测试都通过了！（后面的 `diffpatch` 测试失败是另一个包的问题，不影响我们的模块）。现在让我更新 Todo 并编写文档。








        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
现在编写模块需求文档：










        
            
toolName: Write
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\mimedetect.md
          
changes: undefined
          
现在让我更新 Todo 列表，标记所有任务完成：










        
            
toolName: todo_write
            
status: success
          
Todos updated: 10 items
          
所有任务已完成！以下是完整的实现总结：

## 完成情况

### 1. 核心实现文件
- **[mimedetect.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go)**（547 行）

### 2. 单元测试文件
- **[mimedetect_test.go](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect_test.go)**（1054 行，50+ 测试用例）

### 3. 需求文档
- **[mimedetect.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/mimedetect.md)**（330 行）

### 4. 测试结果
```
go test ./internal/mimedetect/ -v
=== RUN   TestNewDetector
--- PASS: TestNewDetector (0.00s)
...（所有 50+ 测试用例全部通过）
PASS
ok      solocoder-go/internal/mimedetect        0.431s
```

## 核心功能实现

### 1. 魔术字类型识别
- 支持 **固定偏移匹配**（如 TAR 文件在偏移 257 处的 `ustar` 签名）
- 支持 **可变偏移匹配**（如 HTML 文件在任意位置搜索 `<html>` 标签）
- 内置 **45+ 种** 常见文件类型签名（PNG、JPEG、GIF、PDF、ZIP、MP4 等）
- 未匹配时返回 `application/octet-stream`

### 2. 双向映射表
- **扩展名 → MIME**：80+ 种扩展名映射
- **MIME → 扩展名**：60+ 种 MIME 类型映射
- 支持大小写不敏感、自动去除前导点号

### 3. 自定义注册
- `RegisterMagicSignature()` - 注册自定义魔术字签名
- `RegisterExtension()` - 注册自定义扩展名映射
- `RegisterMIMETypeInfo()` - 注册自定义 MIME 类型信息
- **自定义条目优先于内置条目**

### 4. 字节流嗅探
- `DetectFromBytes(data []byte)` - 直接对内存字节数组检测
- 无需从文件读取，适用于网络传输数据实时判断

### 5. 并发安全
- 内部使用 `sync.RWMutex` 读写锁
- 读操作（检测、查询）并发安全
- 写操作（注册）互斥执行

## 核心结构体

| 结构体 | 位置 | 职责 |
|--------|------|------|
| `MagicSignature` | [mimedetect.go#L30-L35](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L30-L35) | 魔术字签名定义 |
| `MIMETypeInfo` | [mimedetect.go#L37-L40](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L37-L40) | MIME 类型信息 |
| `Detector` | [mimedetect.go#L42-L53](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/mimedetect/mimedetect.go#L42-L53) | 检测器主结构体 |

## 使用示例

```go
detector := mimedetect.NewDetector()

// 字节流检测
pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
mime := detector.DetectFromBytes(pngData)  // "image/png"

// 文件检测
mime, _ = detector.DetectFromFile("document.pdf")  // "application/pdf"

// 扩展名查询
mime = detector.GetMIMETypeByExtension(".jpg")  // "image/jpeg"
ext := detector.GetExtensionByMIMEType("application/pdf")  // "pdf"

// 自定义注册
customSig := mimedetect.MagicSignature{
    MagicBytes: []byte{0xDE, 0xAD, 0xBE, 0xEF},
    Mode:       mimedetect.OffsetFixed,
    MIMEType:   "application/x-myformat",
}
detector.RegisterMagicSignature(customSig)
```

所有需求已按要求实现完成，测试全部通过！