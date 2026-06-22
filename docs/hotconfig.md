# HotConfig 配置热加载模块

## 1. 模块概述

HotConfig 是一个功能完备的配置热加载模块，为 Go 应用提供配置文件的动态加载、变更监听、校验验证以及默认值回退等能力。模块支持多种主流配置格式（JSON、YAML、TOML），通过声明式 Schema 定义配置结构与约束，使得应用能够在运行时安全地响应配置文件的变更。

### 核心特性

- **多格式解析**：根据文件扩展名自动选择解析器，支持 JSON、YAML(.yaml/.yml)、TOML 三种格式
- **配置变更监听**：基于文件系统轮询的热加载机制，配置文件修改后自动重新加载并触发回调
- **声明式校验**：通过 Schema 声明校验规则，包括必填、数值范围、字符串长度、正则模式、枚举、自定义校验
- **默认值回退**：配置项缺失或校验失败时，自动回退到预声明的默认值，保证系统可用性
- **快照隔离**：配置读取和回调传递使用深度拷贝的快照，避免外部修改影响内部状态
- **并发安全**：所有公共 API 均通过互斥锁保护，支持多协程并发访问

---

## 2. 核心结构体与职责

### 2.1 HotConfig

模块的核心入口，封装了配置加载、热监听、回调管理等全部能力。

```go
type HotConfig struct {
    mu          sync.RWMutex      // 读写锁，保护所有共享状态
    path        string            // 配置文件的绝对路径
    schema      *Schema           // 配置结构描述与校验规则
    options     *HotConfigOptions // 运行时配置选项
    snapshot    *ConfigSnapshot   // 当前生效的配置快照
    callbacks   []ChangeCallback  // 已注册的变更回调列表
    callbackIDs map[string]int    // 回调ID到索引的映射
    nextCBID    int               // 下一个回调ID的计数器
    running     bool              // 热加载监听是否运行中
    stopCh      chan struct{}     // 停止信号通道
    wg          sync.WaitGroup    // 后台协程等待组
    version     uint64            // 配置版本号，每次成功变更递增
    lastModTime time.Time         // 上次检测到的文件修改时间
    eventCh     chan *fileEvent   // 文件变更事件缓冲通道
}
```

**主要职责**：
- 管理配置文件的生命周期（加载、监听、停止）
- 协调解析器、校验器与默认值机制的执行顺序
- 调度文件变更事件与回调通知
- 维护线程安全的配置快照

### 2.2 Schema 与 FieldSchema

描述配置文件的结构元数据，包括每个字段的路径、类型、默认值和校验规则。

```go
type Schema struct {
    Fields []*FieldSchema  // 所有字段的声明列表
}

type FieldSchema struct {
    Path         string            // 点分隔的嵌套路径，如 "database.host"
    Type         string            // 字段类型提示，供文档或扩展使用
    Required     bool              // 必填字段语法糖（等价于追加一条 RuleRequired）
    DefaultValue interface{}       // 字段的默认值
    Rules        []*ValidationRule // 针对该字段的校验规则列表
}
```

> **必填校验说明（统一语义）**：`FieldSchema.Required` 与 `ValidationRule{Type: RuleRequired}` 完全等价，调用方可任选其一。若同时设置，也只会执行一次必填校验。必填语义统一为：**字段缺失 或 值为空（nil/空串/空切片/空映射/空指针）** 均判定为校验失败，底层均通过 `isEmpty()` 判据与 `ErrFieldRequired` 错误报告，调用方不会感知到两套机制。

**校验规则 ValidationRule**：

| 规则类型 | 触发条件 | 相关参数 |
|---------|---------|---------|
| `RuleRequired` | 字段缺失 或 值为空（nil、空串、空切片/映射/指针） | - |
| `RuleMinValue` | 数值小于 `MinValue` | `MinValue` |
| `RuleMaxValue` | 数值大于 `MaxValue` | `MaxValue` |
| `RuleMinLength` | 字符串/切片/映射长度小于 `MinLen` | `MinLen` |
| `RuleMaxLength` | 字符串/切片/映射长度大于 `MaxLen` | `MaxLen` |
| `RulePattern` | 字符串不匹配正则表达式 `Pattern` | `Pattern` |
| `RuleEnum` | 值不在 `Enum` 列表中 | `Enum` |
| `RuleCustom` | 自定义函数 `Custom` 返回非 nil 错误 | `Custom` |

### 2.3 ConfigSnapshot

配置的不可变快照，用于回调比较和外部读取。所有对外暴露的快照都是内部数据的深度拷贝。

```go
type ConfigSnapshot struct {
    Data      map[string]interface{} // 配置数据的深拷贝
    Timestamp time.Time              // 快照创建时间
    Source    string                 // 配置文件路径
    Format    ConfigFormat           // 原始文件格式
    Version   uint64                 // 单调递增的版本号
}
```

### 2.4 HotConfigOptions

控制模块运行时行为的选项集合。

```go
type HotConfigOptions struct {
    AutoReload        bool          // 是否启用自动热加载（默认 true）
    DebounceTime      time.Duration // 变更事件防抖窗口（默认 100ms）
    FailOnError       bool          // 解析/校验失败时是否返回错误（默认 false，使用上一版配置）
    UseDefaultOnError bool          // 校验失败时是否回退默认值（默认 true）
}
```

**错误处理策略**：
- `FailOnError=true`：任何解析或校验错误都会立即返回给调用方，不会更新配置
- `FailOnError=false`（推荐）：首次加载若失败则使用空配置 + 默认值；后续加载若失败则保留上一份有效配置，保证服务不中断

---

## 3. 配置热加载完整流程

### 3.1 初始化与首次加载

```
用户代码
   │
   ▼
NewHotConfig(path, schema, options)
   │  ├─ 参数校验（路径非空）
   │  └─ 转换为绝对路径
   │
   ▼
Load() / Start()
   │
   ├─ ① 安全文件读取（TOCTOU 防护，最多 3 次重试）
   │    ├─ pre-Stat：记录 mtime₁ + size₁
   │    ├─ ReadFile：读取文件原始字节
   │    ├─ post-Stat：记录 mtime₂ + size₂
   │    ├─ 若 mtime₁==mtime₂ && size₁==size₂ → 读取一致，使用内容与 mtime₂
   │    └─ 否则等待 5ms 重试，最终次不一致则使用最后一次结果
   │
   ├─ ② 根据扩展名选择解析器
   │    ├─ .json        → JSON 解析器
   │    ├─ .yaml/.yml   → YAML 解析器
   │    └─ .toml        → TOML 解析器
   │
   ├─ ③ 解析为 map[string]interface{}
   │    └─ 失败分支：FailOnError? 抛错 : (已有快照? 保留 : 空map)
   │
   ├─ ④ Schema 规范化（必填校验统一语义）
   │    └─ 若 Field.Required=true 且未显式添加 RuleRequired
   │       → 自动前置注入一条 RuleRequired 规则
   │
   ├─ ⑤ 应用默认值（填充缺失字段）
   │    └─ ApplyDefaults(data, schema)
   │
   ├─ ⑥ 配置校验
   │    ├─ ValidateConfig(data, schema) （含必填的两种语法糖）
   │    └─ 失败分支：
   │         ├─ FailOnError=true  → 抛出 AggregateValidationError
   │         └─ UseDefaultOnError → 将失败字段替换为默认值
   │
   ├─ ⑦ 与上一版快照比对（DeepEqual）
   │    └─ 内容相同则跳过，不递增版本，只更新 lastModTime
   │
   └─ ⑧ 更新快照（版本号 +1，记录时间戳/格式/最后一致的 mtime）
```

### 3.2 运行时热加载（自动监听）

```
 pollLoop (50ms 轮询协程)
   │
   ├─ 每次 tick 检查文件 mtime
   │    └─ mtime 未变化 → 跳过
   │
   └─ mtime 变化 → 投递 fileEvent 到 eventCh
                        │
                        ▼
               eventLoop (事件处理协程，无数据竞争设计)
                  │
                  ├─ 接收 evt，声明局部变量 currentEvent = evt（栈上捕获）
                  │
                  ├─ DebounceTimer（防抖）
                  │    ├─ 窗口内新事件重置定时器
                  │    └─ 定时器闭包直接捕获 currentEvent 的局部副本
                  │         （不通过共享变量传递，避免跨协程竞争）
                  │
                  └─ processEvent(currentEvent)
                       │
                       ├─ 持有写锁重新走 3.1 的 ①~⑧ 流程
                       │
                       ├─ 内容有变化？
                       │    └─ 否 → 释放锁，结束
                       │
                       └─ 是 → 复制回调列表（释放写锁）
                            │
                            ▼
                       依次触发所有回调
                       ├─ 每个回调独立 recover 保护
                       ├─ 传入 oldSnapshot / newSnapshot 深拷贝
                       └─ 回调 panic 不影响其他回调与模块主流程
```

### 3.3 手动触发重新加载

调用 `Reload()` 方法可跳过文件轮询，强制触发一次完整的加载流程。行为与自动监听完全一致（包括回调触发、防抖之外的所有逻辑）。

---

## 4. 使用示例

### 4.1 最简使用（仅加载一次）

```go
package main

import "solocoder-go/internal/hotconfig"

func main() {
    hc, err := hotconfig.NewHotConfig("config.json", nil, nil)
    if err != nil {
        panic(err)
    }

    if err := hc.Load(); err != nil {
        panic(err)
    }

    name, _ := hc.GetString("app.name")
    port, _ := hc.GetInt("server.port")
}
```

### 4.2 完整示例（Schema 校验 + 热加载 + 回调）

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "solocoder-go/internal/hotconfig"
)

func main() {
    // 1. 定义配置 Schema
    schema := &hotconfig.Schema{
        Fields: []*hotconfig.FieldSchema{
            {
                Path:         "app.name",
                Required:     true,
                DefaultValue: "my-app",
                Rules: []*hotconfig.ValidationRule{
                    {Type: hotconfig.RuleMinLength, MinLen: 2},
                    {Type: hotconfig.RuleMaxLength, MaxLen: 50},
                },
            },
            {
                Path:         "server.port",
                Required:     true,
                DefaultValue: 8080,
                Rules: []*hotconfig.ValidationRule{
                    {Type: hotconfig.RuleMinValue, MinValue: 1024},
                    {Type: hotconfig.RuleMaxValue, MaxValue: 65535},
                },
            },
            {
                Path:         "server.mode",
                DefaultValue: "prod",
                Rules: []*hotconfig.ValidationRule{
                    {Type: hotconfig.RuleEnum, Enum: []interface{}{"dev", "test", "prod"}},
                },
            },
            {
                Path: "database.dsn",
                Rules: []*hotconfig.ValidationRule{
                    {
                        Type:    hotconfig.RulePattern,
                        Pattern: `^\w+://[\w.:@/-]+$`,
                    },
                },
            },
            {
                Path: "feature.flag_ratio",
                DefaultValue: 0.5,
                Rules: []*hotconfig.ValidationRule{
                    {
                        Type: hotconfig.RuleCustom,
                        Custom: func(v interface{}) error {
                            f, ok := v.(float64)
                            if !ok {
                                return fmt.Errorf("must be a number")
                            }
                            if f < 0 || f > 1.0 {
                                return fmt.Errorf("must be between 0 and 1")
                            }
                            return nil
                        },
                    },
                },
            },
        },
    }

    // 2. 创建 HotConfig 实例（使用默认选项）
    hc, err := hotconfig.NewHotConfig("config.yaml", schema, &hotconfig.HotConfigOptions{
        DebounceTime:      200 * time.Millisecond,
        FailOnError:       false,
        UseDefaultOnError: true,
    })
    if err != nil {
        log.Fatalf("create hotconfig failed: %v", err)
    }

    // 3. 注册变更回调
    _, _ = hc.RegisterCallback(func(oldSnap, newSnap *hotconfig.ConfigSnapshot) {
        log.Printf("[config changed] v%d → v%d", oldSnap.Version, newSnap.Version)

        oldPort, _ := oldSnap.Data["server"].(map[string]interface{})["port"]
        newPort, _ := newSnap.Data["server"].(map[string]interface{})["port"]
        if oldPort != newPort {
            log.Printf("  server.port changed: %v → %v", oldPort, newPort)
        }
    })

    // 4. 启动热加载（内部会自动执行首次 Load）
    if err := hc.Start(); err != nil {
        log.Fatalf("start hotconfig failed: %v", err)
    }
    defer hc.Stop()

    printConfig(hc)

    // 5. 模拟服务运行，按 Ctrl+C 退出
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-sigCh:
            log.Println("shutdown...")
            return
        case <-ticker.C:
            printConfig(hc)
        }
    }
}

func printConfig(hc *hotconfig.HotConfig) {
    snap := hc.GetSnapshot()
    appName, _ := hc.GetString("app.name")
    port, _ := hc.GetInt("server.port")
    mode, _ := hc.GetString("server.mode")
    ratio, _ := hc.GetFloat64("feature.flag_ratio")

    log.Printf("config v%d | app=%s | port=%d | mode=%s | flag_ratio=%.2f",
        snap.Version, appName, port, mode, ratio)
}
```

### 4.3 示例配置文件

**config.yaml**:
```yaml
app:
  name: my-awesome-service

server:
  port: 9090
  mode: dev

database:
  dsn: postgres://user:pass@localhost:5432/app

feature:
  flag_ratio: 0.75
```

**等价的 config.json**:
```json
{
  "app": { "name": "my-awesome-service" },
  "server": { "port": 9090, "mode": "dev" },
  "database": { "dsn": "postgres://user:pass@localhost:5432/app" },
  "feature": { "flag_ratio": 0.75 }
}
```

**等价的 config.toml**:
```toml
[app]
name = "my-awesome-service"

[server]
port = 9090
mode = "dev"

[database]
dsn = "postgres://user:pass@localhost:5432/app"

[feature]
flag_ratio = 0.75
```

---

## 5. 错误类型与处理

模块提供了一系列具名错误变量，便于调用方通过 `errors.Is` 进行模式匹配：

| 错误变量 | 触发场景 |
|---------|---------|
| `ErrFileNotFound` | 配置文件不存在 |
| `ErrUnsupportedFormat` | 文件扩展名不支持（非 json/yaml/yml/toml） |
| `ErrParseFailed` | 底层解析器返回错误（通过 `ParseError` 包装） |
| `ErrFieldRequired` | 必填字段缺失或为空 |
| `ErrFieldOutOfRange` | 数值/长度超出上下限 |
| `ErrFieldInvalidFormat` | 字符串不匹配正则模式 |
| `ErrFieldTypeMismatch` | 校验规则与字段实际类型不兼容 |
| `ErrWatcherAlreadyRunning` | 对已经 Start 的实例再次 Start |
| `ErrInvalidConfigPath` | 传入空路径或非法路径 |
| `ErrNilCallback` | 注册 nil 回调 |

结构化错误包装：
- `*ValidationError`：单字段校验失败，含 `Field`、`Message` 与底层 `Err`
- `*AggregateValidationError`：多个字段校验失败，含 `Errors []*ValidationError`
- `*ParseError`：解析失败，含 `Format`、`Path` 与底层 `Err`

所有包装错误均实现了 `Unwrap()/Unwrap() []error`，可通过 `errors.Is/As` 链式判断。

---

## 6. API 速查表

| 方法签名 | 说明 |
|---------|------|
| `NewHotConfig(path, schema, opts) (*HotConfig, error)` | 构造实例，转换为绝对路径 |
| `hc.Load() error` | 执行一次同步加载（不启动监听） |
| `hc.Start() error` | 启动后台协程，自动首次加载 + 持续监听 |
| `hc.Stop()` | 停止后台协程，释放资源（幂等） |
| `hc.Reload() error` | 手动触发一次重新加载，触发回调 |
| `hc.RegisterCallback(fn) (id string, err)` | 注册变更回调，返回唯一 ID |
| `hc.UnregisterCallback(id) bool` | 按 ID 注销回调 |
| `hc.GetSnapshot() *ConfigSnapshot` | 获取当前配置的深拷贝快照 |
| `hc.Get(key) (interface{}, bool)` | 按点分隔路径读取任意值 |
| `hc.GetString(key) (string, bool)` | 读取字符串 |
| `hc.GetInt(key) (int, bool)` | 读取整数（兼容 float64/int64） |
| `hc.GetFloat64(key) (float64, bool)` | 读取浮点数 |
| `hc.GetBool(key) (bool, bool)` | 读取布尔值 |
| `hc.IsRunning() bool` | 监听是否运行中 |
| `hc.Version() uint64` | 当前配置版本号 |
| `hc.Path() string` | 配置文件绝对路径 |
| `hc.CallbackCount() int` | 当前已注册回调数量 |

---

## 7. 文件结构

```
internal/hotconfig/
├── hotconfig.go       # 核心 HotConfig 结构体与生命周期管理
├── types.go           # 公共类型、常量与 Schema 定义
├── parser.go          # JSON/YAML/TOML 多格式解析器实现
├── validator.go       # 校验引擎、默认值、嵌套路径访问、深拷贝
├── errors.go          # 错误变量与结构化错误类型
└── hotconfig_test.go  # 单元测试（覆盖解析、校验、默认值、监听、并发）
```

---

## 8. 设计要点与最佳实践

1. **快照不可变性**：`GetSnapshot()`、回调参数均为深拷贝，用户可任意修改而不影响内部状态
2. **回调隔离**：每个回调运行在独立的 `defer recover` 保护中，单个回调 panic 不影响整体
3. **版本单调递增**：仅当配置内容经 `reflect.DeepEqual` 判断实际变化时才递增版本，避免"无变更重载"污染版本号
4. **防抖合并**：短时间内连续写入文件会被合并为一次回调，避免业务抖动
5. **优雅降级**：默认策略下任何错误都不中断服务；推荐始终配置默认值，即使配置文件完全丢失也能以最低可用模式启动
6. **路径约定**：所有嵌套字段使用点号分隔的扁平路径（`database.host`），兼容 JSON/YAML/TOML 任意嵌套结构
7. **事件无竞争传递**：`eventLoop` 通过**栈上局部变量闭包捕获**将 `fileEvent` 从事件协程传递给定时器协程，不使用跨协程共享的 pending 变量，从根源避免数据竞争
8. **TOCTOU 安全读取**：`loadLocked` 采用 **pre-Stat → ReadFile → post-Stat** 的一致性校验读取流程，比较两次 `mtime+size`，不一致则重试（最多 3 次，间隔 5ms），确保读取到的文件内容与最终记录的 `lastModTime` 是匹配的快照，避免轮询漏检
9. **必填校验单一语义**：`FieldSchema.Required` 在 `ValidateConfig` 入口处自动规范化为 `RuleRequired` 规则，两套 API 完全等价，均使用同一套 `isEmpty()` 判据与 `ErrFieldRequired` 错误输出，调用方无需区分使用场景

---

## 9. 测试覆盖

模块单元测试位于 [hotconfig_test.go](../internal/hotconfig/hotconfig_test.go)，共 59 个测试用例，覆盖以下场景：

| 分类 | 覆盖点 |
|------|--------|
| **解析器** | JSON/YAML/YML/TOML 四种格式、大小写不敏感扩展名、不支持格式、无效语法、文件不存在 |
| **校验规则** | 必填、数值范围、字符串长度、正则模式、枚举、自定义校验、类型不兼容、组合规则 |
| **默认值** | 缺失字段填充、配置值覆盖默认、校验失败回退默认、嵌套字段默认、`FailOnError` 开关 |
| **快照与读取** | 快照独立性（修改不污染内部）、类型化 Get 方法、不存在键、类型不匹配、未加载读取 |
| **回调** | 注册/注销、nil 回调、多回调全部触发、panic 隔离、快照独立、变更前后对比 |
| **热加载** | Start/Stop 幂等、重复 Start 报错、文件变更检测、防抖合并、无内容变更不触发、手动 Reload |
| **工具函数** | 深拷贝嵌套结构、默认值应用不修改原 map、错误类型 Error() 输出、无扩展名路径 |
