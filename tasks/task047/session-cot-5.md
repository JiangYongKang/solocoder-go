toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
现在我需要修改 data URI 白名单逻辑，对 `image/svg+xml` 做内容检测：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
changes: undefined
          
现在我需要添加 `isSafeSVGDataURI` 函数。让我找到一个合适的位置来添加它：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
我会在 `isSelfClosingTag` 函数之前添加 `isSafeSVGDataURI` 函数。我需要检查文件顶部的导入项：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
好的，已经有 `strings` 导入了。我需要添加 `encoding/base64` 来解码 base64 SVG 内容：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
changes: undefined
          
现在，请在 `isSelfClosingTag` 之前添加 `isSafeSVGDataURI` 函数：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
现在我来更新测试。我需要修改那些目前期望所有SVG都被阻止的测试，并添加测试来验证安全的SVG被允许：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
现在我需要更新测试。关键的修改是：
1. 在 `TestSanitizeDataURIDangerousTypesBlocked` 中，将“SVG without base64”测试用例从“被阻止”更改为“被允许”（因为它是一个安全的SVG）
2. 更新 `TestSanitizeDataURISVGScriptBlocked`，使其只验证危险的SVG被阻止
3. 添加一个新的测试 `TestSanitizeDataURISafeSVGAllowed`，用于验证安全的SVG通过
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
changes: undefined
          
现在更新 `TestSanitizeDataURISVGScriptBlocked` 测试并添加 `TestSanitizeDataURISafeSVGAllowed`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
changes: undefined
          
现在，我来添加 `encoding/base64` 和 `fmt` 导入到测试文件中。让我检查一下现有的导入：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
需要添加 `encoding/base64` 和 `fmt`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          

        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/contentsec/ -v -count=1 undefined
          
有一个失败 - `TestSanitizeDataURIDangerousTypesBlocked/SVG_XSS_with_script_tag_blocked`。`base64` 内容 `PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+` 解码为 `<svg onload=alert(1)>`，其中包含 `onload=`，所以它应该被阻止。但是测试正在检查整个 `data:image/svg+xml` 字符串是否被阻止。由于 SVG 内容检测现在会阻止包含 `onload` 的特定 SVG，因此属性被完全移除，但测试预期将 `data:image/svg+xml` 视为已阻止。

等等 - 更仔细地查看输出：`<img src="data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+" alt="xss" />` - SVG 实际上是被允许通过的！这意味着我的 `isSafeSVGDataURI` 没有正确检测到 `onload=`。让我调试一下...

问题在于，在 `processAttributes` 中，`trimmed` 已经通过 `strings.ToLower` 转换为小写，但原始的 `attrValue` 仍然保留其原始大小写。让我检查一下流程：

在 `processAttributes` 中，`trimmed` 被设置为 `strings.TrimSpace(strings.ToLower(attrValue))`。然后将 `trimmed` 和 `dataRest` 传递给 `isSafeSVGDataURI`。由于 `trimmed` 是小写的，`dataRest` 也是小写的。`base64` 编码是区分大小写的 - 将 `attrValue` 小写会破坏 `base64` 编码！

我需要从原始的 `attrValue` 中提取 `base64` 内容，而不是从小写的 `trimmed` 中提取。让我来修复这个问题：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
问题是 `trimmed` 是小写的，这破坏了 base64 编码。我需要将原始的 `attrValue` 传递给 `isSafeSVGDataURI`:
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
changes: undefined
          
现在更新 `isSafeSVGDataURI` 函数签名，使其只接受 `originalDataRest` 和 `commaIdx`：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec.go
          
changes: undefined
          
现在，我还将修复 `TestSanitizeDataURIDangerousTypesBlocked` 测试，针对 base64 编码的 SVG 与 onload 的情况。由于该 SVG 现在被内容检查阻止了（而不是整个 SVG 类型被阻止），我需要调整预期：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
“带脚本标签的 SVG XSS 被阻止”的测试用例期望 `data:image/svg+xml` 不出现在输出中。但现在我们做的是内容检查 - 包含 `onload` 的 SVG 会被阻止（属性被移除），但 `data:image/svg+xml` 字符串本身不是我们检查的依据。测试应该检查危险的 SVG 内容不会出现在输出中。让我更新一下：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\contentsec\contentsec_test.go
          
changes: undefined
          
现在再次运行测试：
        
            
toolName: run_command
            
status: success
          
command: cd c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go ; go test ./internal/contentsec/ -v -count=1 undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 3 items
          
所有测试通过！以下是修复总结：

---

## ✅ 修复完成

### 🔧 代码修改

#### 核心修改：SVG data URI 从全量封锁改为脚本内容检测

**修改位置**：[contentsec.go#L618-L621](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentsec/contentsec.go#L618-L621)

```go
// 之前：image/svg+xml 被完全排除在白名单外
// 现在：对 SVG 做内容检测
} else if mimeType == "image/svg+xml" {
    isSafe = isSafeSVGDataURI(originalDataRest, commaIdx)
}
```

**新增函数**：[isSafeSVGDataURI](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentsec/contentsec.go#L648-L677)

检测逻辑：
1. 提取 data URI 中逗号后的 SVG 内容
2. 对 base64 编码的 SVG 进行解码（支持 `StdEncoding` 和 `RawStdEncoding`）
3. 对 URL 编码的 SVG 进行 `%XX` 解码
4. 对解码后的内容匹配两个危险模式：
   - `(?i)<script[\s>]` — 检测 `<script>` 标签
   - `(?i)\bon\w+\s*=` — 检测 `onload=`、`onclick=`、`onmouseover=` 等事件处理器
5. 匹配到危险内容则拦截，否则放行

**关键修复**：传递原始 `attrValue`（保留大小写）给 `isSafeSVGDataURI`，因为 base64 编码是大小写敏感的，之前传入了 `strings.ToLower` 后的值导致 base64 解码失败，危险 SVG 被误放行。

**新增辅助函数**：[unescapeURIComponent](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/contentsec/contentsec.go#L679-L709) — 解码 data URI 中的 `%XX` 百分号编码

### 🧪 测试更新

| 测试函数 | 修改 | 子用例 |
|---------|------|--------|
| `TestSanitizeDataURISVGScriptBlocked` | 新增 onfocus/onerror/inline script 测试 | 8 个 |
| `TestSanitizeDataURISafeSVGAllowed` | **新增** — 验证安全 SVG 图标放行 | 5 个 |
| `TestSanitizeDataURIDangerousTypesBlocked` | 调整 SVG 测试的断言语义 | 14 个 |

### ✅ 测试结果

```
go test ./internal/contentsec/ -v -count=1
PASS
ok      solocoder-go/internal/contentsec        0.286s
```

**行为对比**：

| 输入 | 之前（全量封锁） | 现在（内容检测） |
|------|--------------|-------------|
| `data:image/svg+xml;base64,...(安全图标)` | ❌ 被拦截 | ✅ 放行 |
| `data:image/svg+xml,<svg onload=alert(1)>` | ❌ 被拦截 | ❌ 被拦截 |
| `data:image/svg+xml;base64,...(含script)` | ❌ 被拦截 | ❌ 被拦截 |
| `data:image/png;base64,...` | ✅ 放行 | ✅ 放行 |