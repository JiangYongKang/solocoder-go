# StructLog 结构化日志库模块

## 模块功能概述

StructLog 是一个 Go 语言结构化日志库，位于 `internal/structlog/` 包下。所有日志以 JSON 对象格式输出，每条日志独占一行，便于日志采集系统解析和处理。核心功能包括：

1. **JSON 格式输出**：日志记录以 JSON 对象序列化输出，每条日志包含时间戳（ISO 8601 / RFC 3339 Nano 格式）、日志级别、消息文本和上下文字段。输出目标可配置为标准输出或 `io.Writer` 接口的任意实现（如字节缓冲区），便于测试验证。

2. **日志级别动态调整**：支持 Debug、Info、Warn、Error 四个级别，按严重程度递增。提供运行时切换当前输出级别的接口 `SetLevel()`，低于当前级别的日志消息被丢弃不输出，切换后立即对新产生的日志生效。

3. **上下文字段透传**：通过 `WithFields()` 创建包含固定键值对的子日志实例。子日志实例继承父日志实例的所有上下文字段并追加自身的新字段，子实例不修改父实例的字段集合。

4. **采样率控制**：按日志级别分别配置采样率，采样率以整数分母的形式配置（如采样率为 10 表示平均每 10 条同级别日志只输出 1 条）。计数器按级别独立维护，未被采样的日志被丢弃。设置级别采样率为 0 时该级别完全不采样，即每条都输出。

5. **调用栈信息自动附加**：Error 级别的日志自动附加产生日志的调用栈信息（文件名和行号），调用栈信息作为 JSON 输出中的独立 `stack` 字段，不包含完整的 goroutine 堆栈。其他级别的日志记录文件名和行号于 `caller` 字段，但不展开完整调用栈。

6. **系统字段保护**：`ts`、`level`、`msg`、`caller`、`stack` 为系统保留字段，具有最高写入优先级。即使用户通过 `WithFields()` 传入同名上下文字段，系统字段的值也不会被覆盖，确保日志采集系统的可靠性。

---

## 核心结构体与职责

### Level

```go
type Level int

const (
    LevelDebug Level = iota  // 0
    LevelInfo                // 1
    LevelWarn                // 2
    LevelError               // 3
)
```

**职责**：定义日志级别枚举，级别值越高严重程度越高。提供 `String()` 方法将级别转换为可读字符串（DEBUG / INFO / WARN / ERROR）。

---

### sharedState（内部结构体）

```go
type sharedState struct {
    output         io.Writer
    level          atomic.Int32
    samplingRates  [4]atomic.Int32
    sampleCounters [4]atomic.Int64
    mu             sync.Mutex
}
```

**职责**：Logger 实例之间共享的状态，确保通过 `WithFields()` 创建的子日志实例与父实例共享输出目标、级别设置和采样配置。

- **output**：日志输出目标，实现 `io.Writer` 接口
- **level**：当前日志输出级别，使用原子操作支持并发安全的动态切换
- **samplingRates**：每个级别的采样率（分母），使用原子操作存储
- **sampleCounters**：每个级别的采样计数器，使用原子操作递增
- **mu**：互斥锁，保护输出写入的串行化

---

### Logger

```go
type Logger struct {
    state  *sharedState
    fields map[string]interface{}
}
```

**职责**：结构化日志记录器，是对外暴露的核心类型。

- **state**：指向共享状态的指针，同一族（通过 WithFields 派生）的 Logger 共享同一个 state
- **fields**：当前日志实例携带的上下文字段，WithFields 派生时拷贝并追加

对外方法：

| 方法 | 签名 | 说明 |
|------|------|------|
| New | `New(output io.Writer, level Level) *Logger` | 创建日志实例，output 为 nil 时默认输出到 os.Stdout |
| SetLevel | `(l *Logger) SetLevel(level Level)` | 动态设置输出级别，立即生效 |
| GetLevel | `(l *Logger) GetLevel() Level` | 获取当前输出级别 |
| WithFields | `(l *Logger) WithFields(fields map[string]interface{}) *Logger` | 创建子日志实例，继承父字段并追加 |
| SetSamplingRate | `(l *Logger) SetSamplingRate(level Level, rate int)` | 设置指定级别的采样率（分母），0 表示不采样 |
| Debug | `(l *Logger) Debug(msg string)` | 输出 Debug 级别日志 |
| Info | `(l *Logger) Info(msg string)` | 输出 Info 级别日志 |
| Warn | `(l *Logger) Warn(msg string)` | 输出 Warn 级别日志 |
| Error | `(l *Logger) Error(msg string)` | 输出 Error 级别日志 |

---

## 日志级别与采样率的关系

### 日志级别过滤

日志级别按严重程度递增排列：`Debug(0) < Info(1) < Warn(2) < Error(3)`。

当前输出级别由 `SetLevel()` 设置，低于该级别的日志在 `log()` 方法入口处即被丢弃，不会进入采样判断或 JSON 序列化流程：

```
调用 Debug/Info/Warn/Error
        │
        ▼
  检查 level >= 当前输出级别 ──否──► 丢弃，不输出
        │是
        ▼
  检查采样率 ──被采样丢弃──► 丢弃，不输出
        │通过
        ▼
  构造 JSON 对象并写入 output
```

### 采样率机制

采样率以整数分母形式配置：

| 采样率值 | 含义 |
|----------|------|
| 0 | 不采样，该级别每条日志都输出 |
| 1 | 每 1 条输出 1 条（等同不采样） |
| 10 | 平均每 10 条输出 1 条 |
| 100 | 平均每 100 条输出 1 条 |

采样判定逻辑：每个级别维护独立的原子计数器，每次调用日志方法时递增。当 `(counter - 1) % rate == 0` 时通过采样（输出），否则丢弃。第 1 条日志始终通过，之后每隔 rate 条通过一条。

**计数器独立性**：各级别的采样计数器完全独立，互不影响。Debug 级别的采样不影响 Info 级别的计数。

**重置行为**：调用 `SetSamplingRate()` 设置新采样率时，该级别的计数器重置为 0，确保新配置的第一条日志始终输出。

---

## JSON 输出格式

### 非 Error 级别输出示例

```json
{"caller":"main.go:15","level":"INFO","msg":"request processed","service":"api","ts":"2025-06-19T10:30:00.123456789Z"}
```

### Error 级别输出示例

```json
{"caller":"handler.go:42","level":"ERROR","msg":"database connection failed","service":"api","stack":["handler.go:42","router.go:28","main.go:10"],"ts":"2025-06-19T10:30:00.123456789Z"}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| ts | string | ISO 8601 / RFC 3339 Nano 格式的时间戳（UTC） |
| level | string | 日志级别字符串（DEBUG / INFO / WARN / ERROR） |
| msg | string | 日志消息文本 |
| caller | string | 调用方文件名和行号（如 `main.go:15`），所有级别均包含 |
| stack | []string | 调用栈（仅 Error 级别），每项为 `文件名:行号` 格式，不包含 structlog 内部帧 |
| * | any | 通过 WithFields 附加的上下文字段 |

### 系统字段保护机制

`ts`、`level`、`msg`、`caller`、`stack` 为系统保留字段，具有最高写入优先级。即使用户通过 `WithFields()` 传入同名的上下文字段，最终输出的 JSON 中这些字段的值仍然是系统生成的真实值，不会被用户字段覆盖。

**实现原理**：在 `log()` 方法中，先将用户上下文字段写入 `entry` map，再依次写入系统字段。由于 map 后写入的键值会覆盖先写入的，因此系统字段始终保留真实值，确保日志采集系统可以可靠地按级别过滤、按时间排序。

---

## 调用者定位逻辑

### 基本原理

`captureCaller()` 和 `captureStack()` 函数通过 `runtime.Callers()` 获取调用栈，跳过 `structlog.go` 源文件中的所有内部帧（包括 Logger 的方法和内部辅助函数），定位到业务代码中的实际调用位置。

### 内部帧过滤规则

内部帧的判定基于源文件名：文件名以 `structlog.go` 结尾的帧被视为内部帧，会被自动跳过。这种基于文件名的过滤策略相比基于包名前缀的过滤有以下优势：

1. **精准性**：只跳过日志库自身的实现代码，不会误过滤同包内的其他代码（如同包的测试文件、同包的业务代码等）。
2. **稳定性**：不受函数名、方法名、接收者类型变化的影响。
3. **可测试性**：测试代码位于 `structlog_test.go` 中，不会被误过滤，可以精确验证调用者指向测试文件中的具体行号。

### 各字段的调用信息

| 字段 | 级别 | 内容 | 定位方式 |
|------|------|------|----------|
| caller | 所有级别 | 单个 `文件名:行号` | 跳过内部帧后的第一帧 |
| stack | 仅 Error 级别 | `[]string`，每元素为 `文件名:行号` | 跳过内部帧后的全部剩余帧 |

---

## 使用示例

### 示例 1：基本使用

```go
package main

import (
    "os"
    "solocoder-go/internal/structlog"
)

func main() {
    logger := structlog.New(os.Stdout, structlog.LevelDebug)

    logger.Debug("调试信息")
    logger.Info("业务操作成功")
    logger.Warn("性能告警")
    logger.Error("数据库连接失败")
}
```

### 示例 2：动态切换日志级别

```go
logger := structlog.New(os.Stdout, structlog.LevelDebug)

logger.Debug("这条会输出")
logger.SetLevel(structlog.LevelWarn)
logger.Debug("这条被丢弃")
logger.Info("这条也被丢弃")
logger.Warn("这条会输出")
```

### 示例 3：上下文字段透传

```go
logger := structlog.New(os.Stdout, structlog.LevelInfo)

requestLogger := logger.WithFields(map[string]interface{}{
    "request_id": "abc-123",
    "method":     "GET",
})

requestLogger.Info("处理请求")

dbLogger := requestLogger.WithFields(map[string]interface{}{
    "db": "postgres",
})

dbLogger.Error("查询超时")
// 输出包含 request_id、method、db 三个上下文字段
```

### 示例 4：采样率控制

```go
logger := structlog.New(os.Stdout, structlog.LevelDebug)

logger.SetSamplingRate(structlog.LevelDebug, 100)
logger.SetSamplingRate(structlog.LevelInfo, 10)

// Debug 级别平均每 100 条输出 1 条
// Info 级别平均每 10 条输出 1 条
// Warn 和 Error 级别不设采样率，每条都输出
```

### 示例 5：输出到字节缓冲区（测试场景）

```go
import "bytes"

var buf bytes.Buffer
logger := structlog.New(&buf, structlog.LevelInfo)

logger.Info("test message")

// 读取 buf.Bytes() 验证输出内容
```

### 示例 6：系统字段保护（防止用户字段覆盖）

```go
logger := structlog.New(os.Stdout, structlog.LevelDebug)

// 即使用户传入与系统字段同名的上下文字段
child := logger.WithFields(map[string]interface{}{
    "ts":     "fake-timestamp",
    "level":  "FAKE",
    "msg":    "fake message",
    "caller": "fake.go:0",
})

child.Info("real message")

// 输出的 ts、level、msg、caller 仍然是系统生成的真实值
// 用户传入的同名字段会被系统字段覆盖，不会生效
```
