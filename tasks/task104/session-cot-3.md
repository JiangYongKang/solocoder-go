让我找到文档中第 3.3 节执行阶段的描述并修复：



        
            
toolName: search_by_regex
            
status: success
          
query: 执行阶段|ErrCommandNotFound|未匹配子命令|未设置 Handler
          

        
            
toolName: view_files
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
现在修复这个描述：




        
            
toolName: edit_file_search_replace
            
status: success
          
            
filePath: c:\Users\vince\GoletaLab\SoloCoder-3\solocoder-go\docs\cliparser.md
          
changes: undefined
          
文档已修复。现在运行测试验证一切正常：







        
            
toolName: run_command
            
status: success
          
command: go test ./internal/cliparser/ -v undefined
          
修复完成，所有 **81 个测试全部通过 ✅**。

---

## 修复总结

### 修复内容

**文件**：[cliparser.md](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/docs/cliparser.md#L222-L227)

第 3.3 节执行阶段描述已从：
```
- 若未匹配子命令或未设置 Handler → 返回 ErrCommandNotFound
```

修改为：
```
- 若未匹配子命令（parsedCmd == nil）→ 返回 ErrCommandNotFound
- 若子命令已匹配但未设置 Handler → 返回 ErrNoHandler
```

现在文档描述与代码 [Execute()](file:///c:/Users/vince/GoletaLab/SoloCoder-3/solocoder-go/internal/cliparser/cliparser.go#L464-L472) 实现完全一致，调用方可根据返回的错误类型明确区分两种不同的失败场景。