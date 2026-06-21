# Snowflake 分布式 ID 生成器模块需求文档

## 1. 模块概述

Snowflake 分布式 ID 生成器是一个基于 Twitter Snowflake 算法的 64 位唯一 ID 生成组件。通过将时间戳、机器 ID 和序列号三个字段组合到单个 64 位整数中，实现全局唯一、趋势递增、高性能的分布式 ID 生成，适用于分布式系统中需要唯一标识符的场景（如订单号、消息 ID、日志追踪 ID 等）。

本模块提供 ID 生成、ID 解析、时钟回拨保护等核心能力，保证在多节点部署环境下 ID 的唯一性和有序性。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | ID 生成 (Next) | 基于时间戳 + 机器 ID + 序列号生成 64 位全局唯一 ID，同毫秒内序列号递增 |
| F2 | 序列号耗尽阻塞 | 同一毫秒内序列号耗尽（4096 个）时阻塞等待下一毫秒，不返回重复 ID |
| F3 | 小幅时钟回拨容忍 | 检测到时钟回拨 ≤ 5ms 时短暂等待时钟追上后继续生成 |
| F4 | 大幅时钟回拨拒绝 | 检测到时钟回拨 > 5ms 时拒绝生成 ID 并返回包含回拨幅度的错误 |
| F5 | ID 解析 (Parse) | 从 64 位 ID 中提取时间戳、机器 ID、序列号三个原始字段 |
| F6 | 时间恢复 (ParsedID.Time) | 将解析出的相对时间戳转换为绝对时间（time.Time），便于审计和排查 |
| F7 | ID 分解 (Decompose) | Parse 的别名，提供更语义化的函数名用于 ID 分解场景 |
| F8 | 机器 ID 校验 | 构造时校验机器 ID 在有效范围 [0, 1023] 内，超出范围返回错误 |

## 3. 核心结构体与职责

### 3.1 Config - 生成器配置

```go
type Config struct {
    MachineID int64 // 机器标识，范围 [0, 1023]
}
```

**配置约束：**
- `MachineID`：必须满足 0 ≤ MachineID ≤ 1023。超出范围时 `New()` 返回 `ErrInvalidMachineID`
- 每个部署节点应分配不同的 MachineID，以确保跨节点 ID 唯一性

### 3.2 Snowflake - 生成器主体

```go
type Snowflake struct {
    mu        sync.Mutex    // 保护内部状态的互斥锁
    machineID int64         // 机器标识（10 位，0-1023）
    lastTS    int64         // 上一次生成 ID 时的相对时间戳（毫秒）
    sequence  int64         // 当前毫秒内的序列号（12 位，0-4095）
    nowFunc   func() time.Time // 时间获取函数（默认 time.Now，可注入用于测试）
}
```

**主要职责：**
- 维护 ID 生成的内部状态（上次时间戳、当前序列号），通过互斥锁保证并发安全
- 在 `Next()` 方法中协调时间戳获取、序列号递增、时钟回拨检测三者的交互逻辑
- 通过 `nowFunc` 字段支持依赖注入，便于单元测试中模拟可控时间

### 3.3 ID - 64 位标识符

```go
type ID int64
```

**主要职责：**
- 作为 64 位 Snowflake ID 的类型别名，提供类型安全性
- 可直接作为 int64 使用，支持格式化输出和算术运算

### 3.4 ParsedID - 解析结果

```go
type ParsedID struct {
    Timestamp int64 // 相对于 Epoch 的毫秒时间戳（41 位）
    MachineID int64 // 机器标识（10 位）
    Sequence  int64 // 序列号（12 位）
}
```

**主要职责：**
- 存储 ID 解析后的三个原始字段，便于问题排查和日志审计
- 提供 `Time()` 方法将相对时间戳转换为绝对时间

### 3.5 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidMachineID` | 机器 ID 超出有效范围 | `New()` 传入 MachineID < 0 或 > 1023 |
| `ErrClockBackward` | 时钟回拨 | `Next()` 检测到当前时间戳小于上次记录的时间戳且回拨幅度 > 5ms |

## 4. 64 位 ID 位分配方案

```
 0         | 41                              | 51               | 63
 ├─────────┼─────────────────────────────────┼──────────────────┼──────────┤
 │ 1 bit   │ 41 bits                         │ 10 bits          │ 12 bits  │
 │ 符号位  │ 时间戳（相对 Epoch 的毫秒数）    │ 机器 ID          │ 序列号   │
 │ (恒为 0) │                                 │ (0-1023)         │ (0-4095) │
 └─────────┴─────────────────────────────────┴──────────────────┴──────────┘
```

**各字段详细说明：**

| 字段 | 位数 | 范围 | 说明 |
|------|------|------|------|
| 符号位 | 1 | 恒为 0 | 保证 ID 为正数 |
| 时间戳 | 41 | 0 ~ 2^41-1 | 相对于自定义 Epoch（2024-01-01 00:00:00 UTC）的毫秒数，可用约 69.7 年 |
| 机器 ID | 10 | 0 ~ 1023 | 支持 1024 个节点，部署时需确保各节点 ID 不同 |
| 序列号 | 12 | 0 ~ 4095 | 同一毫秒内的递增计数，每毫秒最多生成 4096 个 ID |

**ID 组合公式：**

```
ID = (Timestamp << 22) | (MachineID << 12) | Sequence
```

其中 `22 = sequenceBits + machineIDBits`，`12 = sequenceBits`。

**容量计算：**
- 单节点每秒最大 ID 生成量：4096 × 1000 = 4,096,000（约 410 万/秒）
- 时间戳可用年限：2^41 / (1000 × 3600 × 24 × 365) ≈ 69.7 年（从 Epoch 起算）

## 5. 时钟回拨处理策略

时钟回拨是指系统时钟突然向过去跳变的现象，常见于 NTP 同步修正、虚拟机迁移等场景。Snowflake 算法依赖单调递增的时间戳，时钟回拨可能导致 ID 重复或乱序。

### 5.1 检测机制

每次调用 `Next()` 生成 ID 时，将当前时间戳与 `lastTS`（上次生成 ID 时记录的时间戳）进行比较：

```
if now < lastTS → 检测到时钟回拨
    offset = lastTS - now  (回拨偏移量，单位毫秒)
```

### 5.2 分级处理策略

```
Next() 检测到 now < lastTS
   │
   ├─ offset ≤ 5ms（小幅回拨）
   │     ├─ 释放锁
   │     ├─ 等待 offset 毫秒
   │     └─ 重新获取锁，重新读取时间戳
   │           ├─ 若 now ≥ lastTS → 正常生成 ID
   │           └─ 若 now < lastTS → 返回 ErrClockBackward（等待后仍未恢复）
   │
   └─ offset > 5ms（大幅回拨）
         └─ 返回 ErrClockBackward，错误信息包含回拨幅度
              fmt.Errorf("snowflake: clock moved backward: offset %dms", offset)
```

**策略设计考量：**
- **5ms 阈值**：小幅 NTP 同步通常在几毫秒内，短暂等待即可恢复，避免因轻微时钟抖动中断服务
- **等待后二次检查**：等待期间时钟可能继续回拨，因此重新获取锁后需要再次验证时间戳
- **大幅回拨直接拒绝**：超过 5ms 的回拨可能意味着严重的时钟异常，继续生成 ID 风险过高，应向上层报告

### 5.3 序列号耗尽处理

```
Next() 检测到 now == lastTS 且 sequence >= 4095
   │
   ├─ 释放锁
   ├─ 循环等待直到 timestamp() > lastTS（进入下一毫秒）
   │     └─ 每次检查间隔 100μs
   ├─ 重新获取锁，重新执行 ID 生成逻辑
   └─ 在新的毫秒内 sequence 从 0 开始递增
```

**设计要点：**
- 序列号耗尽时阻塞等待而非返回错误，保证调用方总能获得 ID
- 释放锁后等待，允许其他协程在当前毫秒的序列号未耗尽时继续生成
- 等待结束后通过重试（continue）重新执行完整逻辑，避免状态不一致

## 6. 核心机制详解

### 6.1 ID 生成流程 (Next)

```
Next()
   │
   └─ [循环重试]
         │
         ├─ mu.Lock()
         ├─ now = timestamp()（当前相对毫秒时间戳）
         │
         ├─ [时钟回拨检测] now < lastTS
         │     ├─ offset ≤ 5ms → mu.Unlock()，Sleep(offset)，continue 重试
         │     └─ offset > 5ms → mu.Unlock()，返回 ErrClockBackward(offset)
         │
         ├─ [同毫秒递增] now == lastTS
         │     ├─ sequence < 4095 → sequence++
         │     └─ sequence >= 4095 → mu.Unlock()，waitUntilNextMs()，continue 重试
         │
         ├─ [新毫秒重置] now > lastTS
         │     └─ sequence = 0
         │
         ├─ lastTS = now
         ├─ id = (now << 22) | (machineID << 12) | sequence
         ├─ mu.Unlock()
         └─ 返回 id
```

### 6.2 ID 解析流程 (Parse)

```
Parse(id)
   │
   ├─ Timestamp = id >> 22           （取高 41 位，右移 22 位）
   ├─ MachineID = (id >> 12) & 0x3FF （取中间 10 位，右移 12 位后掩码）
   ├─ Sequence  = id & 0xFFF         （取低 12 位，掩码）
   └─ 返回 ParsedID{Timestamp, MachineID, Sequence}
```

### 6.3 并发安全设计

- `Next()` 方法通过 `sync.Mutex` 保护 `lastTS` 和 `sequence` 的读写
- 采用"释放锁 → 等待 → 重新获取锁 → 重新检查状态"的模式，避免持锁等待阻塞其他协程
- 等待结束后通过 `continue` 重试整个生成逻辑，保证状态一致性

## 7. 使用示例

### 7.1 基础使用：创建生成器并生成 ID

```go
package main

import (
    "errors"
    "fmt"
    "log"
    "solocoder-go/internal/snowflake"
)

func main() {
    s, err := snowflake.New(snowflake.Config{MachineID: 1})
    if err != nil {
        log.Fatalf("创建 Snowflake 生成器失败: %v", err)
    }

    id, err := s.Next()
    if err != nil {
        log.Fatalf("生成 ID 失败: %v", err)
    }
    fmt.Printf("生成的 ID: %d\n", id)
}
```

### 7.2 解析 ID 用于问题排查

```go
id, _ := s.Next()
parsed := snowflake.Parse(id)
fmt.Printf("ID: %d\n", id)
fmt.Printf("  时间戳: %d (相对 Epoch)\n", parsed.Timestamp)
fmt.Printf("  绝对时间: %v\n", parsed.Time())
fmt.Printf("  机器 ID: %d\n", parsed.MachineID)
fmt.Printf("  序列号: %d\n", parsed.Sequence)
```

### 7.3 处理时钟回拨错误

```go
id, err := s.Next()
if err != nil {
    if errors.Is(err, snowflake.ErrClockBackward) {
        log.Printf("时钟回拨，暂停生成 ID: %v", err)
        // 等待一段时间后重试，或向上层报告
        return
    }
    log.Fatalf("生成 ID 失败: %v", err)
}
```

### 7.4 多节点部署

```go
// 节点 1
s1, _ := snowflake.New(snowflake.Config{MachineID: 1})

// 节点 2
s2, _ := snowflake.New(snowflake.Config{MachineID: 2})

// 各节点独立生成 ID，通过不同的 MachineID 保证跨节点唯一
id1, _ := s1.Next()
id2, _ := s2.Next()
// id1 和 id2 的 MachineID 字段不同，因此全局唯一
```

### 7.5 使用 Decompose 分解 ID（Parse 的语义化别名）

```go
id, _ := s.Next()
parts := snowflake.Decompose(id)
fmt.Printf("时间戳=%d, 机器ID=%d, 序列号=%d\n",
    parts.Timestamp, parts.MachineID, parts.Sequence)
```

## 8. 文件结构

```
internal/snowflake/
├── snowflake.go      # Snowflake ID 生成器核心实现（ID 生成、解析、时钟回拨处理）
└── snowflake_test.go # 单元测试（覆盖正常流程、边界条件、异常分支、并发场景）

docs/
└── snowflake.md      # 本文档
```

## 9. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **构造校验** | `TestNew_ValidMachineID`、`TestNew_InvalidMachineID_Negative`、`TestNew_InvalidMachineID_TooLarge` | 有效机器 ID 创建、负数和超范围校验 |
| **基本生成** | `TestNext_BasicGeneration`、`TestNext_IDsAreMonotonicallyIncreasing`、`TestNext_NoDuplicateIDs` | ID 正数、单调递增、无重复 |
| **字段编码** | `TestNext_MachineIDEncoded`、`TestNext_MachineIDZero`、`TestNext_MachineIDMax` | 机器 ID 正确编码、边界值 |
| **序列号递增** | `TestNext_SequenceIncrementInSameMS`、`TestNext_SequenceZeroForNewMS` | 同毫秒递增、新毫秒重置 |
| **序列号耗尽** | `TestNext_SequenceExhaustion_WaitsForNextMS` | 序列号耗尽后阻塞等待下一毫秒 |
| **时钟回拨** | `TestNext_ClockBackward_SmallDrift`、`TestNext_ClockBackward_LargeDrift`、`TestNext_ClockBackward_BoundaryExactlyAtThreshold`、`TestNext_ClockBackward_OneMsAboveThreshold` | 小幅等待恢复、大幅拒绝、边界阈值 |
| **ID 解析** | `TestParse_RoundTrip`、`TestParse_ManualID`、`TestParse_ZeroID`、`TestParse_MaxValues`、`TestParsedID_Time` | 生成-解析往返、手动构造 ID、零值、最大值、时间恢复 |
| **Decompose** | `TestDecompose_EquivalentToParse` | Decompose 与 Parse 结果一致 |
| **位布局** | `TestIDBitLayout`、`TestEpochValue`、`TestMaxMachineIDValue`、`TestMaxSequenceValue` | 位组合正确性、常量值验证 |
| **并发安全** | `TestNext_Concurrent`、`TestNext_ConcurrentDifferentMachineIDs`、`TestNext_ConcurrentClockBackward` | 并发生成无重复、跨机器 ID 唯一、回拨时并发 |
| **压力测试** | `TestNext_StressHighFrequency` | 10000 次快速生成无重复且单调递增 |
