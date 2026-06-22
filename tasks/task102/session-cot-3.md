toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
先读取需要修改的两个文件的当前内容：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\hotconfig\hotconfig.go
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\hotconfig\validator.go
          
问题很清晰，开始修复。

## 修复 1：loadLocked post-Stat 错误包装
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\hotconfig\hotconfig.go
          
changes: undefined
          
## 修复 2：normalizeFieldSchema 快速路径
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\hotconfig\validator.go
          
changes: undefined
          
两个 if 分支可以合并，更简洁：
        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\internal\hotconfig\validator.go
          
changes: undefined
          
现在运行测试：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: run_command
            
status: success
          
command: $env:GOPROXY = "https://goproxy.cn,direct" ; go test ./internal/hotconfig/ -v -count=1 -timeout 120s undefined
          
59/59 全部通过。现在更新文档：
        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\hotconfig.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\hotconfig.md
          
changes: undefined
          
现在更新 §8 设计要点中的 TOCTOU 说明和 normalizeFieldSchema 说明：
        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\hotconfig.md
          

        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\hotconfig.md
          
changes: undefined
          

        
            
toolName: todo_write
            
status: success
          
Todos updated: 4 items
          
两个问题均已修复，**59/59 测试全部通过**。具体修复内容：

---

## ✅ 修复 1：post-Stat 错误语义丢失

**问题**：`loadLocked` 中 ReadFile 成功后，若文件被外部删除，post-Stat 返回的 `os.ErrNotExist` 被包装为 `"failed to re-stat config file: ..."`，导致调用方 `errors.Is(err, ErrFileNotFound)` 无法匹配。

**修复**：[hotconfig.go#L86-L92](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/hotconfig/hotconfig.go#L86-L92)

```go
// 修复前
postInfo, err := os.Stat(hc.path)
if err != nil {
    return false, fmt.Errorf("failed to re-stat config file: %w", err)
}

// 修复后
postInfo, err := os.Stat(hc.path)
if os.IsNotExist(err) {
    return false, fmt.Errorf("%w: %s", ErrFileNotFound, hc.path)
}
if err != nil {
    return false, fmt.Errorf("failed to re-stat config file: %w", err)
}
```

现在 pre-Stat 和 post-Stat 两侧对文件不存在的情况使用完全一致的错误包装，调用方可通过 `errors.Is(err, ErrFileNotFound)` 统一匹配。

---

## ✅ 修复 2：normalizeFieldSchema 零分配快速路径

**问题**：`Required=false` 且无 `RuleRequired` 是最常用的声明模式（绝大多数字段只需范围/格式校验），但原实现对所有字段一律分配新的 `FieldSchema` 副本。

**修复**：[validator.go#L40-L68](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/hotconfig/validator.go#L40-L68)

```go
func normalizeFieldSchema(field *FieldSchema) *FieldSchema {
    hasRequiredRule := false
    for _, r := range field.Rules {
        if r.Type == RuleRequired {
            hasRequiredRule = true
            break
        }
    }

    if !field.Required {       // 最常用路径：直接返回原指针，零分配
        return field
    }

    if hasRequiredRule {       // 已有 RuleRequired：无需注入
        return field
    }

    // 仅 Required=true 且缺少 RuleRequired 时才分配副本
    rulesCopy := make([]*ValidationRule, 0, len(field.Rules)+1)
    rulesCopy = append(rulesCopy, &ValidationRule{Type: RuleRequired})
    rulesCopy = append(rulesCopy, field.Rules...)
    return &FieldSchema{...}
}
```

三条路径的语义：
- `Required=false` → 直接返回原字段，**零分配**（最常用路径）
- `Required=true` 且已有 `RuleRequired` → 直接返回原字段，**零分配**
- `Required=true` 且缺少 `RuleRequired` → 分配副本并注入规则

---

## 📝 文档更新

[docs/hotconfig.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/hotconfig.md) 已同步更新：

- **§3.1 流程图**：标注 pre-Stat 和 post-Stat 均将 `os.IsNotExist` 包装为 `ErrFileNotFound`
- **§8 设计要点 8**：补充 post-Stat 的 `ErrFileNotFound` 语义保证
- **§8 设计要点 9**：补充 `normalizeFieldSchema` 零分配快速路径说明