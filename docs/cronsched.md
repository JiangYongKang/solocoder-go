# Cron 表达式解析与调度器 (cronsched) 模块需求文档

## 1. 模块概述

Cron 表达式解析与调度器（Cron Scheduler）是一个功能完整的秒级 Cron 任务调度引擎，支持七段式 Cron 表达式解析、时区感知调度、夏令时处理、语法校验以及人类可读的语义描述生成。

该模块位于 `internal/cronsched/` 包下，提供线程安全的 API，可在多协程并发环境中使用。任务执行采用异步模型，调度循环不会被单个任务的执行时长所阻塞。

## 2. 功能需求

### 2.1 秒级 Cron 字段解析

- 支持**七段式** Cron 表达式完整解析：秒、分、时、日、月、周、年
- 年字段为**可选**，六段式表达式（不含年）也可正常解析
- 每个字段支持五种值类型：
  - **通配符** (`*`)：匹配该字段所有合法值
  - **单值** (`5`)：匹配单个精确值
  - **列表** (`1,3,5`)：匹配多个离散值
  - **范围** (`1-5`)：匹配闭区间内的所有值
  - **步长** (`*/15` 或 `10-30/5`)：从起始值开始按固定步长匹配
- 支持月份和星期的英文缩写（`JAN`-`DEC`、`MON`-`SUN`）
- 解析失败时返回**明确的语法错误提示**，指出具体哪个字段、哪个位置不符合规范

### 2.2 下次执行时间计算

- 根据给定的 Cron 表达式和参考时间，计算下一次触发的精确时间点
- 支持连续获取后续多次的执行时间列表
- 正确处理**月份天数差异**（28/29/30/31 天）
- 正确处理**闰年** 2 月 29 日等特殊日期边界
- 采用高效的逐层递进算法：从参考时间 +1 秒开始，逐层检查年、月、日、时、分、秒，不匹配时快速前进到下一个可能的时间点
- 若在合理范围内（默认 5 年）找不到匹配时间，返回 `ErrNoNextTime` 错误

### 2.3 时区感知调度

- 支持为每个 Cron 任务指定**独立的时区**（`*time.Location`）
- 调度器在计算执行时间时基于对应时区进行时间转换
- **夏令时（DST）变更时**能正确处理：
  - **Spring Forward**（时间跳过）：不存在的时间点自动跳过，不导致任务丢失
  - **Fall Back**（时间重复）：重复的时间点只执行一次，不导致任务重复执行
- 时区转换采用 Go 标准库 `time` 包的原生能力，保证准确性

### 2.4 表达式语法校验

- 提供独立的表达式验证接口 `Validate(expr string)`，返回：
  - `Valid`：表达式是否有效
  - `Description`：人类可读的语义描述（如"每天凌晨 2 点执行"）
  - `Errors`：验证失败时的详细错误列表
- 验证过程包括：
  - **字段值范围检查**：各字段值是否在合法范围内
  - **内部逻辑一致性检查**：日和周字段的互斥关系（不能同时指定非通配符值）
  - **特殊字符有效性检查**：`*`、`,`、`-`、`/` 等字符的使用是否符合规范
- 语义描述支持中文本地化输出

### 2.5 任务调度器

- 基于最小堆（Min-Heap）实现优先级队列，按下次执行时间排序
- 支持任务注册、取消、运行时管理
- 支持自动生成任务 ID 或用户指定自定义 ID
- 任务 panic 自动捕获，不影响调度器整体运行
- 支持并发安全的任务添加和取消操作

## 3. 核心结构体与职责

### 3.1 FieldValue（字段值）

```go
type FieldValue struct {
    Type      ValueType
    Value     int
    Start     int
    End       int
    Step      int
}
```

表示 Cron 字段中单个值项的结构，支持通配符、单值、列表、范围、步长五种类型。

| 字段 | 类型 | 说明 |
|------|------|------|
| Type | ValueType | 值类型：通配符/单值/列表/范围/步长 |
| Value | int | 单值类型的具体值 |
| Start | int | 范围/步长类型的起始值 |
| End | int | 范围/步长类型的结束值 |
| Step | int | 步长类型的步长值 |

### 3.2 CronField（Cron 字段）

```go
type CronField struct {
    Type     FieldType
    Min      int
    Max      int
    Values   []FieldValue
    RawValue string
}
```

表示 Cron 表达式中的一个字段，包含类型、取值范围和多个值项。

| 字段 | 类型 | 说明 |
|------|------|------|
| Type | FieldType | 字段类型：秒/分/时/日/月/周/年 |
| Min | int | 该字段的最小值 |
| Max | int | 该字段的最大值 |
| Values | []FieldValue | 该字段的值项列表 |
| RawValue | string | 原始字符串值（用于错误提示） |

### 3.3 CronExpression（Cron 表达式）

```go
type CronExpression struct {
    Second     *CronField
    Minute     *CronField
    Hour       *CronField
    Day        *CronField
    Month      *CronField
    Weekday    *CronField
    Year       *CronField
    HasYear    bool
    Location   *time.Location
    RawExpr    string
}
```

表示一个完整的 Cron 表达式，包含所有七个字段（年字段可选）。

| 字段 | 类型 | 说明 |
|------|------|------|
| Second | *CronField | 秒字段（0-59） |
| Minute | *CronField | 分字段（0-59） |
| Hour | *CronField | 时字段（0-23） |
| Day | *CronField | 日字段（1-31） |
| Month | *CronField | 月字段（1-12） |
| Weekday | *CronField | 周字段（0-6，0=周日） |
| Year | *CronField | 年字段（1970-2100），可选 |
| HasYear | bool | 是否包含年字段 |
| Location | *time.Location | 时区 |
| RawExpr | string | 原始表达式字符串 |

### 3.4 CronTask（Cron 任务）

```go
type CronTask struct {
    ID         string
    CronExpr   *CronExpression
    Func       TaskFunc
    Status     TaskStatus
    NextRun    time.Time
    LastRun    time.Time
    RunCount   int
    Location   *time.Location
    CreatedAt  time.Time
    index      int
}
```

表示一个待调度的 Cron 任务。

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 任务唯一标识 |
| CronExpr | *CronExpression | Cron 表达式 |
| Func | TaskFunc | 任务执行函数 |
| Status | TaskStatus | 任务状态：待执行/执行中/已取消/已完成 |
| NextRun | time.Time | 下次执行时间 |
| LastRun | time.Time | 上次执行时间 |
| RunCount | int | 执行次数 |
| Location | *time.Location | 任务时区 |
| CreatedAt | time.Time | 创建时间 |
| index | int | 在堆中的索引（内部使用） |

### 3.5 Scheduler（调度器）

```go
type Scheduler struct {
    mu       sync.Mutex
    heap     *taskHeap
    tasks    map[string]*CronTask
    running  bool
    stopCh   chan struct{}
    wakeCh   chan struct{}
    wg       sync.WaitGroup
    taskWg   sync.WaitGroup
    ctx      context.Context
    cancel   context.CancelFunc
    nextID   uint64
    idMu     sync.Mutex
    config   *SchedulerConfig
}
```

核心调度器结构，负责管理和执行 Cron 任务。

| 字段 | 类型 | 说明 |
|------|------|------|
| mu | sync.Mutex | 保护堆和任务映射的互斥锁 |
| heap | *taskHeap | 最小堆，存储待执行任务 |
| tasks | map[string]*CronTask | 任务 ID 到任务指针的映射 |
| running | bool | 调度器运行状态标志 |
| stopCh | chan struct{} | 停止信号通道 |
| wakeCh | chan struct{} | 唤醒信号通道 |
| wg | sync.WaitGroup | 等待调度循环协程退出 |
| taskWg | sync.WaitGroup | 等待所有运行中任务完成 |
| ctx | context.Context | 根上下文 |
| cancel | context.CancelFunc | 取消函数 |
| nextID | uint64 | 自动生成 ID 的计数器 |
| idMu | sync.Mutex | 保护 ID 生成的互斥锁 |
| config | *SchedulerConfig | 调度器配置 |

### 3.6 枚举类型

**FieldType（字段类型）**：
- `Second`：秒
- `Minute`：分
- `Hour`：时
- `Day`：日
- `Month`：月
- `Weekday`：周
- `Year`：年

**ValueType（值类型）**：
- `ValueWildcard`：通配符 (`*`)
- `ValueSingle`：单值 (`5`)
- `ValueList`：列表 (`1,3,5`)
- `ValueRange`：范围 (`1-5`)
- `ValueStep`：步长 (`*/15`)

**TaskStatus（任务状态）**：
- `StatusPending`：待执行
- `StatusRunning`：执行中
- `StatusCancelled`：已取消
- `StatusDone`：已完成

### 3.7 错误类型

| 错误变量 | 说明 |
|----------|------|
| `ErrInvalidExpression` | 无效的 Cron 表达式 |
| `ErrInvalidFieldCount` | 字段数量错误（必须为 6 或 7） |
| `ErrInvalidFieldValue` | 字段值超出合法范围 |
| `ErrInvalidSyntax` | 语法错误 |
| `ErrDayWeekdayMutex` | 日和周字段互斥（不能同时指定非通配符值） |
| `ErrNoNextTime` | 找不到下一次执行时间 |
| `ErrInvalidTimezone` | 无效的时区 |
| `ErrTaskNotFound` | 任务不存在 |
| `ErrTaskAlreadyExists` | 任务 ID 已存在 |
| `ErrTaskRunning` | 任务正在执行中 |
| `ErrSchedulerStopped` | 调度器未启动或已停止 |

**ParseError（解析错误）**：

```go
type ParseError struct {
    Field    FieldType
    Position int
    RawValue string
    Message  string
}
```

自定义解析错误类型，包含精确的错误位置信息：
- `Field`：出错的字段类型
- `Position`：出错的字符位置
- `RawValue`：原始字段值
- `Message`：错误描述

## 4. Cron 表达式语法规范

### 4.1 表达式格式

支持**七段式**（含年）和**六段式**（不含年）两种格式：

```
七段式（含年，可选）：
┌───────────── 秒 (0 - 59)
│ ┌───────────── 分 (0 - 59)
│ │ ┌───────────── 时 (0 - 23)
│ │ │ ┌───────────── 日 (1 - 31)
│ │ │ │ ┌───────────── 月 (1 - 12)
│ │ │ │ │ ┌───────────── 周 (0 - 6，0=周日)
│ │ │ │ │ │ ┌───────────── 年 (1970 - 2100，可选)
│ │ │ │ │ │ │
│ │ │ │ │ │ │
S M H D m W Y

六段式（不含年）：
┌───────────── 秒 (0 - 59)
│ ┌───────────── 分 (0 - 59)
│ │ ┌───────────── 时 (0 - 23)
│ │ │ ┌───────────── 日 (1 - 31)
│ │ │ │ ┌───────────── 月 (1 - 12)
│ │ │ │ │ ┌───────────── 周 (0 - 6，0=周日)
│ │ │ │ │ │
│ │ │ │ │ │
S M H D m W
```

### 4.2 各字段取值范围

| 字段 | 类型 | 最小值 | 最大值 | 备注 |
|------|------|--------|--------|------|
| 秒 (Second) | 第 1 段 | 0 | 59 | 支持 0 秒精确调度 |
| 分 (Minute) | 第 2 段 | 0 | 59 | |
| 时 (Hour) | 第 3 段 | 0 | 23 | 0 表示午夜 12 点 |
| 日 (Day of Month) | 第 4 段 | 1 | 31 | 校验月份实际天数 |
| 月 (Month) | 第 5 段 | 1 | 12 | 支持英文缩写 JAN-DEC |
| 周 (Day of Week) | 第 6 段 | 0 | 6 | 0=周日，1=周一…6=周六；支持英文缩写 MON-SUN |
| 年 (Year) | 第 7 段 | 1970 | 2100 | 可选 |

### 4.3 支持的语法

| 语法形式 | 说明 | 示例 |
|----------|------|------|
| `*` | 通配符，匹配该字段所有合法值 | `* * * * * *`（每秒） |
| 数字精确值 | 匹配单一值 | `0 30 14 * * *`（每天 14:30:00） |
| `,` 逗号列表 | 匹配多个离散值 | `0 0,30 * * * *`（每小时第 0、30 分钟） |
| `-` 范围 | 匹配闭区间内的所有值 | `0 0 9-17 * * 1-5`（工作日 9-17 点整点） |
| `*/n` 步长 | 从最小值开始，每隔 n 匹配一次 | `*/15 * * * * *`（每 15 秒） |
| `start-end/step` | 指定范围内按步长匹配 | `10-40/10 * * * * *`（10-40 秒之间每 10 秒） |
| 列表与范围混合 | 可在逗号列表中混用精确值和范围 | `0,15,30-45/5 * * * * *` |
| 英文月份 | 月份字段支持 JAN-DEC | `0 0 1 * JAN *`（每年 1 月 1 日） |
| 英文星期 | 星期字段支持 MON-SUN | `0 0 9 * * MON-FRI`（工作日 9 点） |

### 4.4 特殊规则

#### 4.4.1 日和周字段互斥

日（第 4 段）和周（第 6 段）字段**不能同时指定非通配符值**。这是为了避免语义歧义。

**正确示例**：
- `0 0 1 * * *`（每天 1 点，日和周都是通配符）
- `0 0 * 15 * *`（每月 15 日，周是通配符）
- `0 0 * * * 1`（每周一，日是通配符）

**错误示例**（将返回 `ErrDayWeekdayMutex`）：
- `0 0 * 15 * 1`（同时指定日=15 和 周=1）

#### 4.4.2 闰年与月份天数

- 2 月 29 日只在闰年匹配，非闰年自动跳过
- 4 月 31 日、6 月 31 日等不存在的日期永远不匹配
- 解析时不校验日期合法性，在匹配时动态校验

## 5. API 接口

### 5.1 表达式解析

| 函数 | 签名 | 说明 |
|------|------|------|
| `Parse` | `func Parse(expr string) (*CronExpression, error)` | 解析 Cron 表达式，使用 UTC 时区 |
| `ParseWithLocation` | `func ParseWithLocation(expr string, loc *time.Location) (*CronExpression, error)` | 解析 Cron 表达式，指定时区 |

### 5.2 执行时间计算

| 函数 | 签名 | 说明 |
|------|------|------|
| `NextTime` | `func NextTime(expr *CronExpression, from time.Time) (time.Time, error)` | 计算下一次执行时间 |
| `NextTimes` | `func NextTimes(expr *CronExpression, from time.Time, count int) ([]time.Time, error)` | 计算后续多次执行时间 |

### 5.3 表达式校验与描述

| 函数 | 签名 | 说明 |
|------|------|------|
| `Validate` | `func Validate(expr string) *ValidationResult` | 验证表达式有效性，返回详细结果 |
| `GenerateDescription` | `func GenerateDescription(expr *CronExpression) string` | 生成人类可读的语义描述 |

### 5.4 调度器创建与生命周期

| 方法 | 签名 | 说明 |
|------|------|------|
| `NewScheduler` | `func NewScheduler() *Scheduler` | 创建新的调度器实例 |
| `NewSchedulerWithConfig` | `func NewSchedulerWithConfig(config *SchedulerConfig) *Scheduler` | 使用自定义配置创建调度器 |
| `Start` | `func (s *Scheduler) Start()` | 启动调度器 |
| `Stop` | `func (s *Scheduler) Stop()` | 停止调度器，等待所有任务完成 |

### 5.5 任务管理

| 方法 | 签名 | 说明 |
|------|------|------|
| `Add` | `func (s *Scheduler) Add(expr string, fn TaskFunc) (string, error)` | 添加 Cron 任务，自动生成 ID，使用 UTC 时区 |
| `AddWithLocation` | `func (s *Scheduler) AddWithLocation(expr string, loc *time.Location, fn TaskFunc) (string, error)` | 添加 Cron 任务，指定时区 |
| `AddWithID` | `func (s *Scheduler) AddWithID(id string, expr string, fn TaskFunc) error` | 添加 Cron 任务，指定 ID |
| `AddWithIDAndLocation` | `func (s *Scheduler) AddWithIDAndLocation(id string, expr string, loc *time.Location, fn TaskFunc) error` | 添加 Cron 任务，指定 ID 和时区 |
| `Cancel` | `func (s *Scheduler) Cancel(id string) error` | 取消任务 |
| `GetTask` | `func (s *Scheduler) GetTask(id string) (*CronTask, error)` | 获取任务信息 |
| `TaskCount` | `func (s *Scheduler) TaskCount() int` | 返回当前任务数量 |

### 5.6 CronExpression 方法

| 方法 | 签名 | 说明 |
|------|------|------|
| `Matches` | `func (e *CronExpression) Matches(t time.Time) bool` | 检查时间是否匹配表达式 |
| `Next` | `func (e *CronExpression) Next(from time.Time) (time.Time, error)` | 计算下一次执行时间 |
| `NextN` | `func (e *CronExpression) NextN(from time.Time, count int) ([]time.Time, error)` | 计算后续多次执行时间 |
| `Description` | `func (e *CronExpression) Description() string` | 生成语义描述 |
| `String` | `func (e *CronExpression) String() string` | 返回原始表达式 |

### 5.7 配置选项

```go
type SchedulerConfig struct {
    MaxIterations  int
    MaxYears       int
}
```

- `MaxIterations`：NextTime 算法的最大迭代次数，默认 1,000,000
- `MaxYears`：向前搜索的最大年数，默认 5 年

## 6. 使用示例

### 6.1 基础使用：解析与执行时间计算

```go
package main

import (
    "fmt"
    "time"
    "solocoder-go/internal/cronsched"
)

func main() {
    // 解析 Cron 表达式（每天凌晨 2 点执行）
    expr, err := cronsched.Parse("0 0 2 * * *")
    if err != nil {
        panic(err)
    }

    // 计算下 5 次执行时间
    now := time.Now()
    times, err := cronsched.NextTimes(expr, now, 5)
    if err != nil {
        panic(err)
    }

    fmt.Println("接下来 5 次执行时间：")
    for i, t := range times {
        fmt.Printf("  %d. %s\n", i+1, t.Format("2006-01-02 15:04:05"))
    }

    // 生成语义描述
    fmt.Printf("\n语义描述：%s\n", cronsched.GenerateDescription(expr))
}
```

### 6.2 时区感知调度

```go
// 使用纽约时区
loc, _ := time.LoadLocation("America/New_York")
expr, err := cronsched.ParseWithLocation("0 30 9 * * 1-5", loc)
if err != nil {
    panic(err)
}

next, _ := cronsched.NextTime(expr, time.Now())
fmt.Printf("下一次执行（纽约时间）：%s\n", next.In(loc).Format("2006-01-02 15:04:05 MST"))
```

### 6.3 表达式验证

```go
result := cronsched.Validate("0 0 9 * * 1-5")
if result.Valid {
    fmt.Printf("表达式有效：%s\n", result.Description)
} else {
    fmt.Printf("表达式无效：\n")
    for _, err := range result.Errors {
        fmt.Printf("  - %v\n", err)
    }
}

// 验证无效表达式（日和周同时指定）
result = cronsched.Validate("0 0 * 15 * 1")
if !result.Valid {
    fmt.Printf("预期错误：%v\n", result.Errors[0])
}
```

### 6.4 使用调度器

```go
package main

import (
    "context"
    "fmt"
    "time"
    "solocoder-go/internal/cronsched"
)

func main() {
    s := cronsched.NewScheduler()
    s.Start()
    defer s.Stop()

    // 每秒执行一次
    _, err := s.Add("* * * * * *", func(ctx context.Context) {
        fmt.Println("每秒执行：", time.Now().Format("15:04:05"))
    })
    if err != nil {
        panic(err)
    }

    // 每天 9:30 执行（纽约时区）
    loc, _ := time.LoadLocation("America/New_York")
    _, err = s.AddWithLocation("0 30 9 * * *", loc, func(ctx context.Context) {
        fmt.Println("纽约时间 9:30 执行")
    })
    if err != nil {
        panic(err)
    }

    // 运行 10 秒
    time.Sleep(10 * time.Second)
}
```

### 6.5 任务取消

```go
s := cronsched.NewScheduler()
s.Start()
defer s.Stop()

// 添加任务并获取 ID
id, _ := s.Add("*/2 * * * * *", func(ctx context.Context) {
    fmt.Println("每 2 秒执行")
})

// 5 秒后取消任务
time.Sleep(5 * time.Second)
err := s.Cancel(id)
if err != nil {
    fmt.Printf("取消失败：%v\n", err)
}
```

### 6.6 处理解析错误

```go
expr, err := cronsched.Parse("60 * * * * *") // 秒=60 超出范围
if err != nil {
    var parseErr *cronsched.ParseError
    if errors.As(err, &parseErr) {
        fmt.Printf("解析错误：\n")
        fmt.Printf("  字段：%s\n", parseErr.Field)
        fmt.Printf("  位置：%d\n", parseErr.Position)
        fmt.Printf("  原始值：%s\n", parseErr.RawValue)
        fmt.Printf("  错误：%s\n", parseErr.Message)
    }
}
```

## 7. 测试覆盖

单元测试位于 `internal/cronsched/cronsched_test.go`，当前共 **53** 个测试用例，覆盖以下维度：

### 7.1 解析测试

- `TestParse_Basic`：基础解析测试，覆盖六段式、七段式、单值、范围、步长、列表、月份名、星期名等
- `TestParse_InvalidFieldCount`：字段数量错误测试
- `TestParse_InvalidValues`：各种无效值测试（越界、非法字符、逆序范围等）
- `TestParse_DayWeekdayMutex`：日和周字段互斥测试
- `TestParse_InvalidTimezone`：空时区测试

### 7.2 字段匹配测试

- `TestField_Matches`：字段匹配测试，覆盖通配符、单值、范围、步长、列表等各种情况

### 7.3 执行时间计算测试

- `TestNextTime_Basic`：基础时间计算
- `TestNextTime_StepAndRange`：步长和范围的时间计算
- `TestNextTime_LeapYear`：闰年 2 月 29 日测试
- `TestNextTime_MonthDays`：不同月份天数测试
- `TestNextTime_Weekday`：星期字段测试
- `TestNextTime_YearField`：年字段测试
- `TestNextTime_NoNextTime`：找不到下一次执行时间测试
- `TestNextTimes_Multiple`：多次执行时间测试
- `TestNextTime_SecondPrecision`：秒级精度测试
- `TestNextTime_YearBoundary`：跨年边界测试
- `TestNextTime_DayBoundary`：跨天边界测试
- `TestNextTime_HourBoundary`：跨小时边界测试
- `TestNextTime_FromExactMatch`：从精确匹配点开始测试

### 7.4 时区与夏令时测试

- `TestParseWithLocation`：带时区解析测试
- `TestNextTime_Timezone`：时区感知计算测试
- `TestNextTime_DST_SpringForward`：夏令时开始（时间跳过）测试
- `TestNextTime_DST_FallBack`：夏令时结束（时间重复）测试

### 7.5 验证与描述测试

- `TestValidate`：表达式验证测试
- `TestGenerateDescription`：语义描述生成测试

### 7.6 调度器测试

- `TestNewScheduler`：创建调度器测试
- `TestScheduler_AddAndExecute`：添加和执行任务测试
- `TestScheduler_AddWithLocation`：带时区添加任务测试
- `TestScheduler_Add_AutoID`：自动生成 ID 测试
- `TestScheduler_Add_DuplicateID`：重复 ID 测试
- `TestScheduler_Cancel`：任务取消测试
- `TestScheduler_Cancel_NotFound`：取消不存在的任务测试
- `TestScheduler_Cancel_WhileRunning`：取消执行中任务测试
- `TestScheduler_CancelPending_NoMemoryLeak`：取消任务无内存泄漏测试
- `TestScheduler_GetTask`：获取任务信息测试
- `TestScheduler_StartStop`：启动停止测试
- `TestScheduler_Add_Stopped`：停止状态添加任务测试
- `TestScheduler_StopBeforeExecute`：停止后不执行测试
- `TestScheduler_TaskPanic`：任务 panic 测试
- `TestScheduler_MultipleTimezones`：多时区任务测试
- `TestScheduler_ConcurrentAdd`：并发添加任务测试
- `TestScheduler_ConcurrentCancel`：并发取消任务测试

### 7.7 其他测试

- `TestExpression_Matches`：表达式匹配测试
- `TestExpression_Next` / `TestExpression_NextN`：表达式方法测试
- `TestFieldValue_Expand`：字段值展开测试
- `TestCronField_Expand`：Cron 字段展开测试
- `TestParseError_Error`：解析错误格式化测试
- `TestIsValidDay`：日期有效性测试
- `TestStringMethods`：字符串方法测试
- `TestTaskStatus_String`：任务状态字符串测试
- `TestFieldType_String`：字段类型字符串测试

## 8. 设计权衡与注意事项

### 8.1 日和周字段的互斥设计

**设计决策**：日和周字段不能同时指定非通配符值。

**原因**：传统 Unix Cron 中，日和周字段是"或"关系（满足一个即匹配），但这种语义容易引起混淆。为了避免歧义，本模块采用更严格的互斥策略，用户必须明确选择按日或按周调度。

**替代方案**：如果需要"每月 15 日或每周一"的语义，可以注册两个独立的 Cron 任务。

### 8.2 NextTime 算法设计

**设计决策**：采用"从参考时间 +1 秒开始，逐层检查并快速前进"的算法。

**优势**：
1. 避免了生成所有可能组合的爆炸式复杂度
2. 可以正确处理闰年、月份天数、时区转换等边界情况
3. 易于理解和维护

**代价**：对于极端不合理的表达式（如"2月30日"），需要遍历到 `MaxYears` 才能确定无解。通过设置合理的 `MaxIterations` 和 `MaxYears` 限制，可以避免无限循环。

### 8.3 夏令时处理策略

**设计决策**：
- **Spring Forward**（时间跳过）：不存在的时间点自动跳过，不执行
- **Fall Back**（时间重复）：重复的时间点只执行第一次

**实现方式**：通过 Go 标准库 `time` 包的时区转换能力，检测时间在 UTC 和本地时区之间转换的一致性。如果转换后时间不一致，说明遇到了夏令时边界，使用转换后的时间作为基准。

### 8.4 异步执行模型

**设计决策**：调度循环检测到任务到期后，通过独立 goroutine 执行任务。

**优势**：
- 长耗时任务不会阻塞其他任务的按时调度
- 任务 panic 不会影响调度器整体运行

**代价**：每个运行中任务占用一个 goroutine。对于极高吞吐且任务极短的场景，可考虑未来扩展 worker pool 模式。

### 8.5 秒级精度的权衡

**设计决策**：支持秒级精度，最小调度间隔为 1 秒。

**优势**：比传统分钟级 Cron 更灵活，适用于需要高频调度的场景。

**代价**：调度循环每秒唤醒一次检查堆顶。对于大多数应用场景，这个开销可以忽略不计。

### 8.6 内存泄漏防护

**设计决策**：
- Pending 状态的任务被取消时，立即从堆和 map 中双重移除
- Running 状态的周期性任务被取消时，执行完毕后从 map 中移除
- 调度循环每次迭代开始时，惰性清理堆顶的无效任务（已取消/已完成）

**效果**：确保长期运行不会因 tasks map 无限增长而导致内存泄漏。
