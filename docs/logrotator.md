# LogRotator 日志文件轮转器模块

## 模块功能概述

LogRotator 是一个功能完善的 Go 语言日志文件轮转器模块，位于 `internal/logrotator/` 包下。它负责将日志输出到文件系统，并提供以下核心能力：

1. **多级别日志分流**：支持 DEBUG、INFO、WARN、ERROR 四个级别，可灵活配置不同级别写入不同文件，或统一写入同一文件。低级别配置的文件自动包含高级别日志（如配置为 INFO 级别的文件同时接受 INFO、WARN、ERROR）。

2. **按大小切分**：当单个日志文件达到配置的大小上限时，自动将当前文件重命名为带序号的备份文件，并创建新的日志文件。

3. **按时间切分**：支持按小时（Hourly）或按天（Daily）自动切分，到达时间边界时自动关闭当前文件并创建新文件，备份文件以时间戳命名。

4. **日志压缩**：切分产生的旧日志文件可配置自动进行 gzip 压缩，压缩在后台 goroutine 中异步执行，不阻塞主流程。

5. **TTL 过期清理**：配置日志保留天数后，后台定时任务自动扫描并删除过期的备份文件（包括原始文件和对应的 .gz 压缩文件）。

6. **文件数量上限**：可配置最大保留的备份文件数量，超过时自动删除最旧的备份。

7. **并发安全**：内部使用互斥锁保护，支持多 goroutine 并发写入日志。

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

**职责**：定义日志级别枚举，级别值越高优先级越高。提供：
- `String()` 方法：将级别转换为可读字符串（DEBUG/INFO/WARN/ERROR）
- `ParseLevel(s string)`：从字符串解析级别，支持大小写和前后空白

---

### RotationMode

```go
type RotationMode int

const (
    RotationModeNone   RotationMode = iota  // 不切分
    RotationModeSize                         // 按大小切分
    RotationModeHourly                       // 按小时切分
    RotationModeDaily                        // 按天切分
)
```

**职责**：定义日志切分模式枚举。

---

### Config

```go
type Config struct {
    LevelFileMap   map[Level]string    // 级别 -> 文件路径映射
    RotationMode   RotationMode        // 切分模式
    MaxFileSize    int64               // 单文件大小上限（字节，仅按大小模式有效）
    MaxBackups     int                 // 最大保留备份文件数（<=0 表示不限制）
    Compress       bool                // 是否对备份文件进行 gzip 压缩
    TTL            time.Duration       // 备份文件保留时长（<=0 表示不清理）
    CleanInterval  time.Duration       // TTL 清理任务扫描间隔
    FileDateFormat string              // 日期格式（默认 "2006-01-02"）
}
```

**职责**：模块配置结构体。通过 `DefaultConfig()` 获取默认配置：

| 字段 | 默认值 |
|------|--------|
| LevelFileMap | 所有级别均指向 "app.log" |
| RotationMode | RotationModeSize (按大小) |
| MaxFileSize | 100 MB |
| MaxBackups | 10 |
| Compress | true |
| TTL | 7 天 |
| CleanInterval | 1 小时 |
| FileDateFormat | "2006-01-02" |

---

### LogRotator

```go
type LogRotator struct {
    config    *Config
    writers   map[string]*fileWriter
    mu        sync.Mutex
    closed    bool
    ctx       context.Context
    cancel    context.CancelFunc
    cleanOnce sync.Once
    wg        sync.WaitGroup
    clock     func() time.Time
}
```

**职责**：日志轮转器的核心结构体。

- **config**：保存用户配置
- **writers**：文件路径 -> fileWriter 映射，确保同一文件只打开一次文件句柄
- **mu**：互斥锁，保护所有并发访问
- **closed**：已关闭标记，防止重复 Close 和关闭后继续写入
- **ctx / cancel**：用于终止后台清理 goroutine
- **cleanOnce**：确保清理 goroutine 只启动一次
- **wg**：等待所有压缩和清理 goroutine 优雅退出
- **clock**：可注入的时间函数，便于单元测试（默认 `time.Now`）

对外方法：
- `New(config *Config) (*LogRotator, error)`：创建并初始化实例
- `Log(level Level, message string) error`：写入一条日志
- `Sync() error`：将缓冲同步到磁盘
- `Close() error`：关闭所有文件句柄，等待所有后台任务完成
- `CleanExpiredNow()`：立即执行一次 TTL 过期清理

---

### fileWriter（内部结构体）

```go
type fileWriter struct {
    path string
    file *os.File
    size int64
    date string
}
```

**职责**：持有单个日志文件的状态。

- **path**：文件路径
- **file**：打开的文件句柄
- **size**：当前文件已写入的字节数（跟踪大小切分触发条件）
- **date**：文件创建时的时间标记（用于时间切分比较）

---

## 日志轮转与清理流程

### 一、日志写入流程

```
调用 Log(level, message)
        │
        ▼
  加互斥锁 mu.Lock()
        │
        ▼
  检查 closed 是否已关闭 ──是──► 返回错误
        │否
        ▼
  pathsForLevel(level)
  （找出所有 配置级别 <= 日志级别的文件路径，去重）
        │
        ▼
  为每个目标文件路径：
    ┌─ 从 writers map 获取 fileWriter
    │       │
    │       ▼
    │  checkAndRotate：判断是否需要切分
    │    ├─ 按大小：size + incoming > MaxFileSize
    │    └─ 按时间：currentDate != fw.date
    │       │
    │    需要切分？──是──► 执行 rotate()
    │       │否              │
    │       │                ▼
    │       │          关闭旧文件
    │       │          重命名为备份（序号/时间戳）
    │       │          异步压缩（如果启用）
    │       │          打开新文件句柄
    │       │          更新 writers map
    │       │          清理超出 MaxBackups 的旧备份
    │       │                │
    │       ▼◄───────────────┘
    │  重新从 writers map 获取最新的 fileWriter
    │       │
    │       ▼
    │  写入日志行并累加 size
    │
    └────► 处理下一个文件路径
        │
        ▼
  解锁 mu.Unlock()
        │
        ▼
      返回
```

### 二、多级别分流规则

`pathsForLevel(level)` 的核心判断：

```go
if level >= lvl {   // 日志级别 >= 配置的最低接受级别
    将路径加入候选集合
}
```

**示例**（配置如下）：
- LevelDebug → `debug.log`
- LevelInfo → `info.log`
- LevelWarn → `warn.log`
- LevelError → `error.log`

写入行为：
- 写 DEBUG：进入 debug.log（仅 DEBUG 配置满足 0>=0）
- 写 INFO：进入 debug.log + info.log（DEBUG 配置 0>=1？否，所以实际上 INFO 只进入 info.log/warn.log/error.log 对应的配置如果 level>=INFO 的话）

更正：实际是配置的级别 lvl 代表该文件"最低接受什么级别"。如果配置 info→info.log，意思是 info.log 接受的是 INFO 及以上。所以：
- 写 INFO(1)：所有 lvl <= 1 的配置路径 → debug.log(lvl=0) 和 info.log(lvl=1) 都接受
- 写 WARN(2)：lvl <= 2 → debug.log + info.log + warn.log

### 三、按大小切分流程

切分命名格式：`{base}.{index}.{ext}`，如 `app.1.log`、`app.2.log`。

1. 关闭当前文件句柄
2. 从 1 开始递增寻找第一个不存在的序号（同时检查 `.log` 和 `.log.gz` 均不存在）
3. 将 `app.log` 重命名为 `app.{index}.log`
4. 创建新的空 `app.log` 并更新 writers map
5. 如果启用压缩：
   - 启动后台 goroutine 异步执行压缩
   - 压缩成功后删除原始备份文件（只保留 `.gz`）
   - 压缩完成后再执行 `cleanOldBackups` 清理超出数量限制的旧备份
6. 如果不启用压缩：直接同步执行 `cleanOldBackups` 清理

**关键同步设计**：压缩与数量清理在同一个 goroutine 中串行执行（先压缩、再清理），确保 `cleanOldBackups` 统计的是 `.gz` 文件数量，且不会在压缩完成前删除源文件。

### 四、按时间切分流程

切分命名格式：
- 按小时：`{base}.{YYYY-MM-DD-HH}.{ext}`（如 `app.2025-06-01-10.log`）
- 按天：`{base}.{YYYY-MM-DD}.{ext}`（如 `app.2025-06-01.log`）
- 同日冲突：追加纳秒时间戳 `{base}.{date}-{nanos}.{ext}`

判断条件：每次写入时比较 `currentDate()` 与文件创建时的 `date` 标记，不一致则切分。

### 五、TTL 过期清理流程

```
New() 创建时启动 cleanLoop goroutine（仅一次，通过 cleanOnce）
        │
        ▼
  for 循环：
    ┌─ 等待 CleanInterval 或 ctx.Done()
    │       │
    │    收到 ticker.C：
    │       ▼
    │  cleanExpired()：
    │    ├─ 收集所有配置的唯一文件路径
    │    └─ 对每个路径执行 cleanPathExpired(path, cutoff)：
    │         ├─ 读取目录所有条目
    │         ├─ 识别以该文件名开头 + "." 的备份（支持 .gz）
    │         ├─ 检查 ModTime 是否早于 (now - TTL)
    │         └─ 过期则删除（同时清理原始和 .gz）
    │
    └─ 收到 ctx.Done()（Close() 调用）：退出循环
```

手动清理：调用 `CleanExpiredNow()` 立即执行一次扫描。

### 六、压缩与清理的同步机制

#### 问题背景

早期实现中存在两个关键问题：

1. **竞态条件**：`rotate()` 方法中，压缩 goroutine 异步启动后，`cleanOldBackups()` 在持有主锁的同步上下文中立即执行。当备份数达到 `MaxBackups` 上限时，清理操作可能在压缩 goroutine 读取源文件之前就将其删除，导致压缩失败或生成损坏的 `.gz` 文件。

2. **双倍磁盘占用**：`compressFile()` 只完成了压缩写入 `.gz` 文件的操作，没有删除原始备份文件，导致原始文件和压缩文件同时保留，造成双倍磁盘占用。

#### 修复方案

**核心思路**：将压缩、源文件删除、数量清理三个步骤放入同一个后台 goroutine 中串行执行，形成一个原子化的备份处理流水线。

```
rotate() 触发切分
    │
    ├─ 启用压缩时：
    │      │
    │      ▼
    │  启动后台 goroutine
    │      │
    │      ├─ 步骤 1：compressAndRemove(src)
    │      │     ├─ 读取源文件并 gzip 压缩
    │      │     ├─ 压缩失败则清理 .gz 半成品
    │      │     └─ 压缩成功则删除原始 .log 备份
    │      │
    │      └─ 步骤 2：cleanOldBackups(path)
    │            ├─ 只统计 .gz 文件数量
    │            └─ 超出 MaxBackups 则删除最旧的 .gz
    │
    └─ 不启用压缩时：
           同步直接执行 cleanOldBackups()
```

#### 设计要点

1. **串行执行**：压缩和清理在同一 goroutine 中顺序执行，避免了清理操作删除正在压缩的源文件。

2. **失败安全**：`compressAndRemove()` 函数中任何一步失败（复制失败、gzip 关闭失败、目标文件关闭失败）都会删除不完整的 `.gz` 文件，避免留下损坏文件。

3. **计数一致性**：启用压缩时，`cleanOldBackups` 只统计 `.gz` 文件，确保 `MaxBackups` 限制的是已完成压缩的备份数量。

4. **生命周期管理**：所有压缩 goroutine 都通过 `sync.WaitGroup` 跟踪，`Close()` 方法会 `wg.Wait()` 等待所有压缩和清理任务完成，确保优雅关闭。

5. **与 TTL 清理的协同**：TTL 过期清理独立于数量清理运行，它会同时删除过期的 `.log` 和 `.gz` 文件，不受上述流水线影响。

---

## 竞态测试设计方法

### 测试目标

验证压缩 goroutine 与清理操作之间的并发安全性，确保：

1. `cleanOldBackups` 不会在压缩 goroutine 读取源文件前将其删除
2. `compressAndRemove` 完成后原始备份文件被正确删除
3. 产生的 `.gz` 文件都是完整且可解压的（非截断或损坏）
4. 多个压缩 goroutine 并发执行时最终状态一致

### 测试钩子

`LogRotator` 提供两个可注入的测试钩子，用于在压缩 goroutine 执行期间插入同步检查点：

```go
type LogRotator struct {
    // ...
    onCompressStart func(path string)     // 压缩开始前调用，此时源文件必须存在
    onCompressEnd   func(path string, err error)  // 压缩完成后、清理前调用
}
```

**钩子调用时序**：

```
compress goroutine 启动
    │
    ├─ onCompressStart(src)     ← 检查点 1：源文件必须存在
    │
    ├─ compressAndRemove(src)   ← 压缩 + 删除源文件
    │
    ├─ onCompressEnd(src, err)  ← 检查点 2：源文件应已删除，.gz 应存在且有效
    │
    └─ cleanOldBackups(path)    ← 数量清理（仅统计 .gz）
```

### TestCompressAndCleanupRace 设计

此测试通过 `onCompressEnd` 钩子在压缩 goroutine 的关键执行窗口内插入检查点，捕获中间状态：

**配置**：`MaxFileSize=100`，`MaxBackups=2`，`Compress=true`，连续写入 15 条大日志触发多轮切分。

**检查点 1 — `onCompressStart`**：
- 验证源文件在压缩开始时仍然存在
- 如果源文件已被其他 goroutine 的 `cleanOldBackups` 删除，说明存在竞态条件

**检查点 2 — `onCompressEnd`**：
- 验证源文件已被 `compressAndRemove` 删除（非被其他 goroutine 提前删除）
- 验证对应的 `.gz` 文件已生成
- 验证 `.gz` 文件可被 `gzip.NewReader` 正确解压（非截断或损坏）
- 此时 `cleanOldBackups` 尚未执行，确保检查点观察的是压缩完成瞬间的真实状态

**最终一致性验证**（`Close()` 之后）：
- 无原始 `.log` 备份文件残留
- `.gz` 文件数不超过 `MaxBackups`
- 所有 `.gz` 文件可正确解压

### TestConcurrentRotateWithCompress 设计

此测试验证多 goroutine 并发写入时压缩与清理的最终一致性：

**配置**：4 个 goroutine 各写 50 条日志，`MaxFileSize=500`，`MaxBackups=3`，`Compress=true`。

**验证项**：
- 通过 `onCompressEnd` + `atomic` 计数器确认压缩操作确实发生
- `Close()` 后无原始 `.log` 备份残留
- `.gz` 文件数不超过 `MaxBackups`
- 每个 `.gz` 文件可正确解压（非截断/损坏）

### Race Detector 使用

由于 Windows/386 环境不支持 Go 内置 race detector，上述测试通过同步原语（`onCompressStart`/`onCompressEnd` 钩子 + `sync.Mutex` + `sync/atomic`）在压缩 goroutine 执行期间主动观察中间状态来替代 race detector 的功能。在支持 race detector 的平台上，可以额外运行：

```bash
go test ./internal/logrotator/ -race -v
```

来检测潜在的数据竞争。

### 七、关闭流程

```
调用 Close()
    │
    ▼
  加锁检查 closed 标记 ──已关闭──► 返回 nil（幂等）
    │未关闭
    ▼
  设置 closed = true
  调用 cancel() 通知清理 goroutine 退出
    │
    ▼
  关闭所有 writers 中的文件句柄
    │
    ▼
  解锁
    │
    ▼
  wg.Wait() 等待所有压缩/清理 goroutine 完成
    │
    ▼
  返回第一个错误（如果有）
```

---

## 使用示例

### 示例 1：默认配置，所有级别写入同一文件，按大小切分

```go
package main

import (
    "log"
    "solocoder-go/internal/logrotator"
)

func main() {
    cfg := logrotator.DefaultConfig()
    // 默认：按大小切分 100MB，保留 10 个备份，gzip 压缩，TTL 7 天
    lr, err := logrotator.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer lr.Close()

    lr.Log(logrotator.LevelDebug, "调试信息")
    lr.Log(logrotator.LevelInfo, "业务操作成功")
    lr.Log(logrotator.LevelWarn, "性能告警，响应时间过高")
    lr.Log(logrotator.LevelError, "数据库连接失败")

    lr.Sync()
}
```

### 示例 2：多级别分流到不同文件

```go
cfg := &logrotator.Config{
    LevelFileMap: map[logrotator.Level]string{
        logrotator.LevelDebug: "logs/debug.log",   // 接受 DEBUG + INFO + WARN + ERROR
        logrotator.LevelInfo:  "logs/info.log",    // 接受 INFO + WARN + ERROR
        logrotator.LevelWarn:  "logs/warn.log",    // 接受 WARN + ERROR
        logrotator.LevelError: "logs/error.log",   // 只接受 ERROR
    },
    RotationMode:  logrotator.RotationModeSize,
    MaxFileSize:   50 * 1024 * 1024, // 50MB
    MaxBackups:    5,
    Compress:      true,
    TTL:           3 * 24 * time.Hour, // 保留 3 天
    CleanInterval: 30 * time.Minute,
}

lr, _ := logrotator.New(cfg)
defer lr.Close()
```

### 示例 3：按小时切分，不压缩

```go
cfg := &logrotator.Config{
    LevelFileMap: map[logrotator.Level]string{
        logrotator.LevelDebug: "access.log",
        logrotator.LevelInfo:  "access.log",
        logrotator.LevelWarn:  "access.log",
        logrotator.LevelError: "access.log",
    },
    RotationMode: logrotator.RotationModeHourly, // 每小时切分一次
    Compress:     false,
    TTL:          24 * time.Hour,
}
```

### 示例 4：按天切分 + 手动清理

```go
cfg := &logrotator.Config{
    LevelFileMap: map[logrotator.Level]string{
        logrotator.LevelInfo:  "daily/app.log",
        logrotator.LevelError: "daily/app.log",
    },
    RotationMode:   logrotator.RotationModeDaily,
    FileDateFormat: "2006-01-02",
    Compress:       true,
    TTL:            30 * 24 * time.Hour, // 保留 30 天
    CleanInterval:  time.Hour,
}

lr, _ := logrotator.New(cfg)

// ... 业务运行 ...

// 手动触发过期清理
lr.CleanExpiredNow()

lr.Close()
```

---

## 文件输出样例

按大小切分后目录结构（启用压缩）：

```
logs/
├── app.log              ← 当前正在写入的文件
├── app.1.log.gz         ← 已压缩的备份（原始 .log 已被 compressAndRemove 删除）
├── app.2.log.gz
└── app.3.log.gz
```

按天切分后目录结构：

```
logs/
├── app.log              ← 2025-06-02 的当前文件
├── app.2025-06-01.log.gz
├── app.2025-05-31.log.gz
└── app.2025-05-30.log.gz
```

日志内容格式：

```
[INFO] 2025-06-02 14:30:25.123 用户登录成功
[WARN] 2025-06-02 14:31:02.456 请求耗时 523ms 超过阈值
[ERROR] 2025-06-02 14:32:10.789 支付回调解析失败: invalid signature
```
