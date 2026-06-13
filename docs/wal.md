# WAL 预写日志存储引擎模块

## 模块功能概述

WAL (Write-Ahead Log) 是一个功能完善的 Go 语言预写日志存储引擎模块，位于 `internal/wal/` 包下。它负责将数据变更记录以追加方式持久化到磁盘日志文件，为上层存储系统（如 LSM-Tree、数据库等）提供崩溃恢复、数据回放、操作审计等基础能力。核心能力包括：

1. **日志追加写入**：支持将数据变更记录以日志条目的形式追加写入日志文件。每条日志包含全局递增的偏移量、操作类型和数据内容，写入操作先落盘再返回确认，保证原子持久化。

2. **顺序读取回放**：支持从指定偏移量开始顺序读取日志条目，逐条返回日志内容，可用于数据恢复或日志回放场景，支持跨多个日志段的连续读取。

3. **按偏移量恢复**：支持从指定偏移量位置开始恢复，通过回调函数回放该位置之后的所有日志条目以重建状态。恢复过程中对损坏的日志条目进行逐字节扫描跳过，并生成结构化警告信息供上层决策。

4. **日志段自动切分**：当日志文件大小超过配置阈值时，自动关闭当前日志段并创建新的日志段继续写入，旧日志段保留用于按需读取或归档。段切换时自动执行 `fsync` 保证数据完整性。

5. **并发安全**：内部使用读写锁保护，支持多 goroutine 并发追加写入和读取操作。

6. **持久化与重载**：支持关闭后重新打开，自动扫描磁盘上的所有日志段，重建内存中的段索引和偏移量状态，实现崩溃后无缝恢复。

---

## 核心结构体与职责

### OpType

```go
type OpType byte

const (
    OpPut        OpType = iota + 1  // 写入/更新操作
    OpDelete                        // 删除操作
    OpCheckpoint                    // 检查点标记
)
```

**职责**：定义日志条目的操作类型枚举。提供 `String()` 方法将操作类型转换为可读字符串（PUT/DELETE/CHECKPOINT/UNKNOWN）。

---

### Entry

```go
type Entry struct {
    Offset int64    // 全局递增的日志偏移量
    Type   OpType   // 操作类型
    Data   []byte   // 数据内容（可为 nil，如 Checkpoint）
}
```

**职责**：表示一条日志记录的内存结构。`Offset` 为 WAL 全局单调递增的序列号，用于定位和恢复。

---

### CorruptedEntryWarning

```go
type CorruptedEntryWarning struct {
    SegmentID int    // 损坏条目所在的段 ID
    Position  int64  // 损坏位置在段文件中的字节偏移
    Reason    string // 损坏原因描述
}
```

**职责**：恢复过程中发现损坏条目时返回的警告信息结构体。提供 `String()` 方法生成可读描述。上层应用可根据警告信息决定是否需要人工介入或执行更高级别的修复策略。

---

### Config

```go
type Config struct {
    Dir            string // WAL 日志文件存储目录
    MaxSegmentSize int64  // 单个日志段的最大字节数，超过则自动切分
    FSyncOnWrite   bool   // 每次写入后是否立即执行 fsync 刷盘
}
```

**职责**：模块配置结构体。通过 `DefaultConfig()` 获取默认配置：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| Dir | `"./wal"` | 日志存储目录，不存在时自动创建 |
| MaxSegmentSize | `64 * 1024 * 1024` (64 MB) | 单段文件最大大小 |
| FSyncOnWrite | `false` | 是否每条都强制刷盘 |

配置校验规则：
- `Dir` 为空返回 `ErrInvalidConfig`
- `MaxSegmentSize <= 0` 时自动使用默认值

---

### segment（内部结构体）

```go
type segment struct {
    id          int        // 段 ID，从 1 开始递增
    path        string     // 段文件的完整路径
    file        *os.File   // 打开的文件句柄
    size        int64      // 当前段的已写入字节数
    startOffset int64      // 本段第一条记录的偏移量（-1 表示空段）
    endOffset   int64      // 本段最后一条记录的偏移量（-1 表示空段）
}
```

**职责**：持有单个日志段文件的状态。段文件命名格式为 `wal_%08d.log`，例如 `wal_00000001.log`、`wal_00000002.log`。

- `startOffset` 和 `endOffset` 通过段加载时的全量扫描建立，用于快速定位某个偏移量所在的段。
- 活跃段（active segment）以 `O_RDWR|O_APPEND` 模式打开，非活跃旧段以 `O_RDWR` 模式打开，支持读取和按需重写。

---

### WAL

```go
type WAL struct {
    mu         sync.RWMutex  // 读写锁，保护并发访问
    config     *Config       // 模块配置
    segments   []*segment    // 所有段按 ID 升序排列
    activeSeg  *segment      // 当前正在写入的活跃段（最后一个段）
    nextOffset int64         // 下一条记录将使用的偏移量
    closed     bool          // 是否已关闭标记
}
```

**职责**：WAL 模块的核心对外结构体。

对外方法：
- `New(config *Config) (*WAL, error)`：创建或加载 WAL 实例
- `Append(opType OpType, data []byte) (int64, error)`：追加一条日志记录，返回分配的偏移量
- `ReadFrom(startOffset int64) ([]*Entry, error)`：从指定偏移量开始顺序读取所有条目
- `RecoverFrom(startOffset int64, cb RecoverCallback) ([]*CorruptedEntryWarning, error)`：从指定偏移量开始恢复，通过回调回放
- `Sync() error`：将活跃段缓冲同步到磁盘
- `Close() error`：关闭所有段文件句柄（幂等）
- `SegmentCount() int`：当前段总数
- `LastOffset() int64`：最后一条已写入记录的偏移量（空 WAL 返回 -1）
- `ActiveSegmentSize() int64`：活跃段当前大小

---

## 日志条目二进制格式

每条日志条目在磁盘上采用如下固定头 + 变长数据的二进制布局：

```
┌────────────┬────────────┬────────────┬──────┬──────────┬──────────────┐
│  Magic     │  Checksum  │  Offset    │ Type │ DataLen  │    Data      │
│  2 bytes   │  4 bytes   │  8 bytes   │ 1B   │ 4 bytes  │  DataLen B   │
└────────────┴────────────┴────────────┴──────┴──────────┴──────────────┘
 0            2            6           14     15         19
```

总头部大小 `entryHeaderSize = 19` 字节。

| 字段 | 大小 | 编码 | 说明 |
|------|------|------|------|
| Magic | 2B | BigEndian uint16 | 魔数 `0x5741`（"WA"），用于条目边界识别 |
| Checksum | 4B | BigEndian uint32 | CRC32-IEEE 校验和，覆盖 Offset + Type + DataLen + Data |
| Offset | 8B | BigEndian int64 | 全局递增偏移量 |
| Type | 1B | uint8 | 操作类型 OpType |
| DataLen | 4B | BigEndian uint32 | 数据部分字节长度 |
| Data | DataLen B | raw bytes | 实际业务数据 |

**校验和计算范围**：从 Offset 字段开始到 Data 结束，共 `8 + 1 + 4 + DataLen = 13 + DataLen` 字节，确保条目内容的完整性。

---

## 日志段的生命周期管理

### 一、段创建与加载

```
New(config) 调用
      │
      ▼
  os.MkdirAll(Dir) 确保目录存在
      │
      ▼
  loadExistingSegments()
  ┌─ 读取目录所有文件
  ├─ 通过文件名 wal_%08d.log 识别段并提取 ID
  ├─ 按 ID 升序排序
  └─ 对每个段执行 openSegment(id)
      ┌─ os.OpenFile(O_RDWR)
      ├─ 读取文件大小
      └─ scanSegmentOffsets(seg) 扫描建立 startOffset / endOffset
          ┌─ 读取整个段到内存
          ├─ 逐字节查找 Magic 0x5741
          ├─ 验证 CRC32 校验和
          ├─ 记录第一个有效条目的 offset 为 startOffset
          └─ 记录最后一个有效条目的 offset 为 endOffset
      │
      ▼
  无现有段？──是──► createSegment(1) 创建 wal_00000001.log
      │否
      ▼
  将最后一个段重新以 O_RDWR|O_APPEND 打开作为 activeSeg
  nextOffset = activeSeg.endOffset + 1
      │
      ▼
  返回 WAL 实例
```

### 二、段写入与切分

```
Append(opType, data) 调用
      │
      ▼
  加写锁 mu.Lock()
      │
      ▼
  检查 closed ──是──► 返回 ErrClosed
      │否
      ▼
  数据校验：非 Checkpoint 操作 data 不能为空
      │
      ▼
  分配 offset = nextOffset
  encodeEntry(entry) 编码为二进制（含 CRC32）
      │
      ▼
  需要切分？（activeSeg.size + encodedLen > MaxSegmentSize）
      │是
      ▼
  rotateSegment()
  ┌─ activeSeg.file.Sync() 刷盘保证旧段数据完整
  └─ createSegment(activeSeg.id + 1) 创建新段并设为 activeSeg
      │
      ▼
  activeSeg.file.Write(encoded) 写入新段或原活跃段
      │
      ▼
  FSyncOnWrite ?──是──► activeSeg.file.Sync()
      │否
      ▼
  更新 activeSeg.size / startOffset / endOffset
  nextOffset++
      │
      ▼
  解锁
  返回分配的 offset
```

### 三、段读取（ReadFrom）

```
ReadFrom(startOffset) 调用
      │
      ▼
  加读锁 mu.RLock()
      │
      ▼
  定位起始段：遍历 segments，找到第一个 endOffset >= startOffset 的段
  （或 startOffset < seg.startOffset 的第一个段）
      │
      ▼
  从起始段到最后一个段依次：
  readSegmentEntries(seg, minOffset)
  ┌─ 读取整个段文件
  ├─ pos = 0
  ├─ 循环 pos < len(data)：
  │   ├─ decodeEntry(data[pos:]) 尝试解码
  │   ├─ 解码失败：pos++（逐字节扫描下一个可能条目）
  │   ├─ 解码成功：
  │   │   ├─ entry.Offset >= minOffset ? ──是──► 加入结果集
  │   │   └─ pos += consumed
  │   └─ 继续
  └─ 返回该段所有符合条件的条目
      │
      ▼
  合并所有段的结果并按偏移量自然有序返回
```

### 四、段恢复（RecoverFrom）

```
RecoverFrom(startOffset, cb) 调用
      │
      ▼
  加读锁 mu.RLock()
      │
      ▼
  校验参数：cb 不为 nil、startOffset >= 0
      │
      ▼
  定位起始段（同 ReadFrom）
      │
      ▼
  从起始段到最后一个段依次：
  recoverSegment(seg, minOffset, cb)
  ┌─ 读取整个段文件
  ├─ pos = 0
  ├─ 循环 pos < len(data)：
  │   ├─ decodeEntry(data[pos:]) 尝试解码
  │   ├─ 解码失败：
  │   │   ├─ 生成 CorruptedEntryWarning{SegmentID, Position, Reason}
  │   │   ├─ 加入 warnings 列表
  │   │   └─ pos++（逐字节扫描）
  │   ├─ 解码成功：
  │   │   ├─ entry.Offset >= minOffset ? ──是──► 调用 cb(entry)
  │   │   │   └─ cb 返回错误？──是──► 立即终止并向上传递
  │   │   └─ pos += consumed
  │   └─ 继续
  └─ 返回 warnings
      │
      ▼
  返回所有段的 warnings 合并列表和第一个回调错误
```

### 五、关闭流程

```
Close() 调用
      │
      ▼
  加写锁 mu.Lock()
      │
      ▼
  检查 closed ──是──► 解锁，返回 nil（幂等）
      │否
      ▼
  closed = true
      │
      ▼
  遍历所有 segments，逐个关闭 file 句柄
  记录第一个错误
      │
      ▼
  解锁，返回第一个错误（如有）
```

---

## 损坏恢复策略

模块采用**逐字节扫描 + CRC32 校验**的损坏恢复策略：

1. **识别条目边界**：通过魔数 `0x5741` (2 字节) 识别潜在的条目起始位置。
2. **验证完整性**：对每个候选位置，读取完整头部后计算 CRC32，与存储值比较。
3. **跳过损坏字节**：魔数不匹配或校验和错误时，位置指针前进 1 字节，继续扫描下一个候选位置。
4. **记录警告**：`RecoverFrom` 会为每个跳过的损坏位置生成结构化警告，包含段 ID、字节位置和具体原因。

该策略的鲁棒性：即使日志段中间出现任意字节损坏（磁盘坏道、写入中途崩溃等），只要后续条目完整，仍能被正确识别和恢复，最大程度减少数据丢失。

---

## 使用示例

### 示例 1：基础使用 — 追加与顺序读取

```go
package main

import (
    "fmt"
    "log"
    "solocoder-go/internal/wal"
)

func main() {
    cfg := wal.DefaultConfig()
    cfg.Dir = "./data/wal"

    w, err := wal.New(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer w.Close()

    off1, _ := w.Append(wal.OpPut, []byte("key1=value1"))
    off2, _ := w.Append(wal.OpPut, []byte("key2=value2"))
    off3, _ := w.Append(wal.OpDelete, []byte("key3"))

    fmt.Printf("offsets: %d, %d, %d\n", off1, off2, off3)

    entries, err := w.ReadFrom(0)
    if err != nil {
        log.Fatal(err)
    }
    for _, e := range entries {
        fmt.Printf("[%d] %s: %s\n", e.Offset, e.Type, string(e.Data))
    }
}
```

### 示例 2：崩溃恢复 — 从上次检查点重建状态

```go
cfg := &wal.Config{
    Dir:            "./data/wal",
    MaxSegmentSize: 32 * 1024 * 1024,
    FSyncOnWrite:   true,
}

w, err := wal.New(cfg)
if err != nil {
    log.Fatal(err)
}
defer w.Close()

checkpointOffset := int64(100) // 从快照/元数据中读取上次检查点

state := make(map[string]string)
warnings, err := w.RecoverFrom(checkpointOffset, func(e *wal.Entry) error {
    switch e.Type {
    case wal.OpPut:
        parts := bytes.SplitN(e.Data, []byte("="), 2)
        if len(parts) == 2 {
            state[string(parts[0])] = string(parts[1])
        }
    case wal.OpDelete:
        delete(state, string(e.Data))
    case wal.OpCheckpoint:
        // 可用于校验点跳跃等高级逻辑
    }
    return nil
})

if err != nil {
    log.Fatalf("recovery failed: %v", err)
}
if len(warnings) > 0 {
    log.Printf("recovery completed with %d warnings:", len(warnings))
    for _, w := range warnings {
        log.Printf("  WARN: %s", w.String())
    }
}
log.Printf("state rebuilt: %d keys", len(state))
```

### 示例 3：多段切分配置 — 高频写入场景

```go
cfg := &wal.Config{
    Dir:            "./data/wal_highfreq",
    MaxSegmentSize: 8 * 1024 * 1024, // 8MB 小段，便于归档和清理
    FSyncOnWrite:   false,            // 性能优先，批量 Sync
}

w, _ := wal.New(cfg)
defer w.Close()

for i := 0; i < 100000; i++ {
    data := fmt.Sprintf("user_%d_action_%d", i%1000, i)
    _, err := w.Append(wal.OpPut, []byte(data))
    if err != nil {
        log.Fatal(err)
    }
    if i%1000 == 0 {
        w.Sync() // 每 1000 条批量刷盘
    }
}

fmt.Printf("total segments: %d\n", w.SegmentCount())
fmt.Printf("last offset: %d\n", w.LastOffset())
```

### 示例 4：重启后继续写入 — 持久化保证

```go
// 进程 1：写入并关闭
w1, _ := wal.New(&wal.Config{Dir: "./data/wal_persist"})
w1.Append(wal.OpPut, []byte("before_close"))
w1.Close()

// 进程 2（或重启后）：重新加载并继续写入
w2, _ := wal.New(&wal.Config{Dir: "./data/wal_persist"})
defer w2.Close()

// 偏移量自动延续
off, _ := w2.Append(wal.OpPut, []byte("after_reopen"))
fmt.Printf("new offset after reopen: %d\n", off) // 输出：1

// 所有数据都在
entries, _ := w2.ReadFrom(0)
fmt.Printf("total entries after reopen: %d\n", len(entries)) // 输出：2
```

---

## 段文件目录样例

```
data/wal/
├── wal_00000001.log   ← 已关闭的旧段（只读读取）
├── wal_00000002.log   ← 已关闭的旧段
├── wal_00000003.log   ← 已关闭的旧段
└── wal_00000004.log   ← 当前活跃段（正在追加写入）
```

段文件按 ID 自然有序，对应时间上的写入先后顺序，便于按时间范围归档或清理旧段。
