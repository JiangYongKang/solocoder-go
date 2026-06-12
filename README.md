# solocoder-go

Go 1.26

## 常用命令

```bash
# 构建
go build ./...

# 运行所有测试
go test ./internal/... -v

# 运行指定包的测试
go test ./internal/order/ -v

# 运行指定包的测试并显示覆盖率
go test ./internal/cache/ -v -cover
```
