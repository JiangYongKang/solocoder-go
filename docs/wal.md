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
    file        *os.File   // 打开的文件句柄（仅活跃段非 nil）
    size        int64      // 当前段的已写入字节数
    startOffset int64      // 本段第一条记录的偏移量（-1 表示空段）
    endOffset   int64      // 本段最后一条记录的偏移量（-1 表示空段）
}
```

**职责**：持有单个日志段文件的元数据状态。段文件命名格式为 `wal_%08d.log`，例如 `wal_00000001.log`、`wal_00000002.log`。

- `startOffset` 和 `endOffset` 通过段加载时的流式扫描建立，用于快速定位某个偏移量所在的段。
- **文件句柄管理策略**：只有当前活跃段（`activeSeg`）以 `O_RDWR|O_APPEND` 模式持久持有 `*os.File` 句柄。所有非活跃旧段的 `file` 字段为 `nil`，不长期占用文件描述符。
- 读取非活跃段（或活跃段）时，每次调用 `ReadFrom` / `RecoverFrom` 都会临时以 `O_RDONLY` 打开独立句柄，读取完成后立即关闭，避免 FD 累积和多 reader 位置竞争。

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

## 文件句柄管理策略

### 设计原则

WAL 模块采用**"活跃段持久持有 + 非活跃段按需打开"**的文件句柄管理策略，确保任意时刻进程内只为 WAL 打开 **1 个持久文件描述符**（即当前活跃段的写入句柄），彻底避免段切分导致的 FD 累积泄漏问题。

### 各场景下的句柄生命周期

| 场景 | 句柄打开方式 | 句柄关闭时机 | 存活时间 |
|------|------------|------------|---------|
| WAL 加载（`New`） | 每个旧段临时 `os.Open(O_RDONLY)` | `scanSegmentOffsets` 返回后 `defer f.Close()` | 单次扫描 |
| 活跃段写入 | `os.OpenFile(O_RDWR\|O_APPEND)` | `rotateSegment` 或 `Close` 时 | 整个活跃期（直到被切分） |
| 段切分（`rotateSegment`） | 新段 `os.OpenFile(O_RDWR\|O_APPEND)` | 下一次切分或 `Close` 时 | 整个活跃期 |
| 段切分（旧段） | 旧段原句柄 `file.Close()` | 切分瞬间同步关闭 | 立即释放 |
| 段读取（`ReadFrom`） | 每段 `os.Open(O_RDONLY)` | 读取完成 `defer f.Close()` | 单次读取调用 |
| 段恢复（`RecoverFrom`） | 每段 `os.Open(O_RDONLY)` | 恢复完成 `defer f.Close()` | 单次恢复调用 |
| WAL 关闭（`Close`） | — | 仅关闭当前活跃段句柄（非活跃段已为 nil） | — |

### 关键实现要点

1. **`segment.file` 字段语义变化**：仅 `activeSeg` 的 `file` 字段非 nil，所有非活跃段恒为 nil。`Close` 遍历时只需 `seg.file != nil` 判断。
2. **`rotateSegment` 三步保障**：先 `Sync` 刷盘 → 再 `Close` 旧句柄 → 最后 `file = nil` 标记，确保数据完整后才释放 FD。
3. **按需打开即开即关**：`readSegmentEntries` 和 `recoverSegment` 对每个段都是 `os.Open` 开头、`defer f.Close()` 结尾，函数退出即释放。

### FD 上限保证

假设系统默认 `ulimit -n` = 1024，极端高并发下：

- 持久占用：**1**（活跃段写入句柄）
- 并发读取：每 `ReadFrom` / `RecoverFrom` 调用占用 = 段数量（通常 N ≤ 数十）
- 总占用 = 1 + 并发 reader 数 × 段数量，远低于系统 FD 上限

---

## 并发读取安全保证

### 问题背景

`*os.File` 内部维护共享的文件偏移量（file offset）。若多个 goroutine 持有同一句柄并发 `Seek` + `Read`：

```
Goroutine A: Seek(0) → Read() → 读到位置 0
Goroutine B: Seek(0) → （A 已推进偏移到末尾）→ Read() → 读到空/截断
```

### 解决方案：每次读取独立句柄

`ReadFrom` 和 `RecoverFrom` 内部对每个段都执行 `os.Open(seg.path)`，获取全新的 `*os.File`，各自维护独立的文件位置：

```
Goroutine A: os.Open(seg1) → fd=5, offset=0 → 顺序读到末尾
Goroutine B: os.Open(seg1) → fd=6, offset=0 → 顺序读到末尾  （互不干扰）
Goroutine C: os.Open(seg2) → fd=7, offset=0 → ...
```

配合 `sync.RWMutex` 读锁保护 `segments` 切片和元数据，整个读取链路完全并发安全。

### 并发层级安全总结

| 层级 | 保护机制 | 说明 |
|------|---------|------|
| 元数据（segments / activeSeg / nextOffset） | `sync.RWMutex` | 写操作加写锁，读操作加读锁 |
| 文件偏移量（每个 reader） | 独立 `os.Open` 句柄 | 每个 reader 持有独立 fd + 独立 offset |
| 活跃段写入 | `sync.RWMutex` 写锁 | `Append` 串行化写入 |
| 段文件磁盘内容 | OS 页缓存 + `fsync` | `O_RDONLY` 只读打开，与写入互不干扰 |

### 性能考虑

`os.Open` 的开销约为微秒级（Linux 下典型 < 5μs），相比 64MB 段文件的读取成本（毫秒级）可忽略不计。流式缓冲读取（`bufio.NewReaderSize(64KB)`）进一步降低了系统调用次数。

---

## 内存使用优化

### 旧方案问题

`io.ReadAll` 将整个段文件一次性加载到内存：

```go
rawData, err := io.ReadAll(seg.file)  // 64MB 段 → 64MB 内存分配
```

问题：
- **峰值内存高**：默认 64MB 段，读取 N 个段即 64×N MB，极端场景触发 GC 压力甚至 OOM。
- **扫描偏移量浪费**：`scanSegmentOffsets` 仅需 `startOffset` / `endOffset` 两个整数，却加载了整段数据。
- **GC 压力大**：大块字节切片在堆上分配，GC 扫描和回收成本高。

### 新方案：流式缓冲读取

所有涉及文件读取的函数（`scanSegmentOffsets` / `readSegmentEntriesStream` / `recoverSegmentStream`）统一使用 `bufio.NewReaderSize(f, 64*1024)` 流式读取：

1. **固定缓冲**：64KB `bufio.Reader` 缓冲，单次系统调用批量填充，避免频繁 syscall。
2. **按需分配**：
   - 仅分配 `entryHeaderSize` (19B) 头部缓冲
   - 仅在 Magic 匹配后才按 `DataLen` 分配数据缓冲
   - 单条数据处理完立即被 GC 回收
3. **峰值内存恒定**：与段文件大小无关，仅取决于最大单条记录的数据长度。

### 内存占用对比

| 场景 | 旧方案（io.ReadAll） | 新方案（流式读取） |
|------|-------------------|----------------|
| 64MB 段扫描偏移量 | ~64 MB | < 100 KB |
| 64MB 段顺序读取 | ~64 MB | ~64 KB + 最大单条记录 |
| 10 个段并发读取 | ~640 MB | ~640 KB + 记录总数 |
| GC 压力 | 高（大块分配） | 低（小块短生命周期） |

---

## 条目解码策略统一性

### 历史问题

最初实现中，三个流式读取函数对二进制条目解码采用了不一致的策略：

| 函数 | 解码策略 | 问题 |
|------|---------|------|
| `scanSegmentOffsets` | **内联手动实现**：独立读取 Magic/Offset/DataLen/CRC，手动拼装 checksumPayload 计算 CRC32 做校验，手动解析 Offset | 代码重复，格式变更需改两处 |
| `readSegmentEntriesStream` | 拼装 `fullBuf` 后调用 `decodeEntry` 统一解码 | 正确 |
| `recoverSegmentStream` | 拼装 `fullBuf` 后调用 `decodeEntry` 统一解码 | 正确 |

**风险**：若日志条目二进制格式（如 Magic、头部字段、校验算法）后续变更，需同时修改 `decodeEntry` 和 `scanSegmentOffsets` 的内联逻辑，极容易遗漏导致两者校验结果不一致或产生隐蔽 Bug。

### 统一方案：`streamScanEntry` 辅助函数

引入包级辅助函数 `streamScanEntry(br *bufio.Reader)` 作为三个读取函数的统一入口，所有解码与校验完全复用 `decodeEntry`：

```
调用者（scan/read/recover）循环调用：
    │
    ▼
streamScanEntry(br)  ──► 统一底层 Peek + Discard 字节级扫描
    │
    ├─ Peek(entryHeaderSize) 预读头部不消费
    │   └─ 检查 Magic，不匹配则 Discard(1) 返回 advanced=1
    │
    ├─ Peek(totalLen) 预读整条（header+data）不消费
    │   └─ 不足则返回 readErr=io.EOF
    │
    └─ ★ decodeEntry(fullPeek[:totalLen]) 统一解码+CRC校验
        ├─ 失败：Discard(1)，返回 decodeErr + advanced=1
        └─ 成功：Discard(totalLen)，返回 entry + advanced=totalLen
```

**设计要点**：
1. **单一事实来源**：所有校验（Magic、CRC32、DataLen 边界）完全由 `decodeEntry` 决定。格式变更只改 `encodeEntry` + `decodeEntry` 两处。
2. **字节级正确扫描**：使用 `bufio.Reader.Peek + Discard` 模式，而非 `ReadFull + Discard(1)`，保证 Magic 不匹配或校验失败时逐字节推进扫描，不会因之前已 `ReadFull` 消耗 19 字节而跳过中间候选位置。
3. **返回四元组**：`(entry *Entry, advanced int64, decodeErr error, readErr error)`，让调用方自主决定是否记录警告、过滤偏移量、调用回调等差异化逻辑：
   - `scanSegmentOffsets`：忽略 `decodeErr`，仅用 `entry.Offset` 维护 first/last
   - `readSegmentEntriesStream`：忽略 `decodeErr`，收集满足 `minOffset` 的 entry
   - `recoverSegmentStream`：用 `decodeErr + advanced` 生成 `CorruptedEntryWarning{SegmentID, filePos, Reason}`，对满足 `minOffset` 的 entry 调用 `cb`

### 维护性收益

- **格式变更零重复**：新增字段、调整校验算法、修改 Magic 等改动只需在 `encodeEntry` / `decodeEntry` 中完成，`scanSegmentOffsets` 自动同步。
- **减少逻辑分支**：三个流式处理函数从 50+ 行各自实现 → 10 行以内循环调用 `streamScanEntry`。
- **扫描正确性**：修复了旧版 `ReadFull(header) → Magic 不匹配 → Discard(1)` 会跳过 19 字节候选窗口的隐蔽 Bug。

---

## 并发读取测试覆盖

### 测试矩阵

针对并发读取安全性，模块设计了 **4 个层级** 的测试覆盖，验证"每次读取独立句柄"方案确实消除了文件偏移量竞争：

| 测试用例 | 读 goroutine 数 | 写 goroutine 数 | 数据规模 | 场景说明 |
|---------|---------------|---------------|---------|---------|
| **`TestConcurrentReadWrite`** | **2** | **1** | **200 条** | **多 reader + 单 writer 并行：2 条 reader 与 1 条 writer 同时运行，reader 1 在写入期间反复 ReadFrom(0) 校验 Offset 顺序一致性，reader 2 在写入期间反复 ReadFrom(0) 校验 Offset 顺序一致性，写入完成后做最终全量校验（200 条数据完整性 + Offset + Data 内容）** |
| **`TestMultipleReadersConcurrent`** | **5** | **0** | **500 条 × 每 reader 50 轮** | **多 reader 纯并发：5 条 goroutine 反复 ReadFrom(0)，校验每条 entry 的 Offset 和 Data 前缀完全一致** |
| **`TestConcurrentReadersDifferentOffsets`** | **12** | **0** | **200 条 × 每 reader 30 轮** | **4 种起始偏移(0/50/100/150)×3 轮重复 = 12 条 reader 并发，校验不同起始偏移下结果的数量与内容都正确** |
| **`TestConcurrentReadersAcrossSegments`** | **4** | **0** | **5 个段 × 每 reader 40 轮** | **跨段并发：4 条 reader 跨越多段文件 ReadFrom(0)，校验多段场景下跨段拼接的 entry 顺序与前缀一致** |

### 多 reader 并发测试详细说明

#### `TestConcurrentReadWrite` — 多 reader + writer 并行完整性

```
主线程：创建空 WAL，启动 3 条 goroutine
  │
  ├─► Writer ─── 追加 200 条 entry_0 ~ entry_199 ─── close(stop)
  │
  ├─► Reader 1 ─ 循环 ReadFrom(0)：
  │              ├─ 写入期间：每次校验所有已返回 entry 的 Offset 连续递增（0,1,2,...）
  │              └─ stop 后：最终 ReadFrom(0) 全量校验
  │                  ├─ len(entries) == 200
  │                  ├─ entry[i].Offset == i
  │                  └─ entry[i].Data == "entry_i"
  │
  └─► Reader 2 ─ 循环 ReadFrom(0)：
                 ├─ 写入期间：每次校验 Offset 连续递增
                 └─ stop 后：最终 ReadFrom(0) 全量校验
                     ├─ len(entries) == 200
                     ├─ entry[i].Offset == i
                     └─ entry[i].Data == "entry_i"
  │
  ▼
断言：reader1ErrCount == 0 AND reader2ErrCount == 0
```

**验证点**：
1. **多 reader 并发无偏移竞争**：2 条 reader 同时对同一段文件执行 `ReadFrom(0)`，各自独立句柄确保不会出现"reader A 消耗文件指针 → reader B 读到空/截断数据"的问题。
2. **读写并行安全**：1 条 writer 持写锁追加数据，2 条 reader 持读锁并发读取，读锁不阻塞读锁，写锁与读锁互斥，保证数据一致性。
3. **双方全量一致性校验**：写入完成后，reader 1 和 reader 2 **均**执行最终 ReadFrom(0) 全量 200 条数据完整性校验（数量 + Offset + Data 内容三重验证），确保并发期间写入的数据全部可读且两条 reader 独立读到的结果一致。

#### `TestMultipleReadersConcurrent` — 多 reader 同偏移完整性

```
主线程：预加载 500 条条目（每条含可识别索引前缀）
  │
  ├─► Reader 0 ─ ReadFrom(0) 50 次 ─ 校验 500 条都在 + 顺序正确
  ├─► Reader 1 ─ ReadFrom(0) 50 次 ─ 校验 500 条都在 + 顺序正确
  ├─► Reader 2 ─ ReadFrom(0) 50 次 ─ 校验 500 条都在 + 顺序正确
  ├─► Reader 3 ─ ReadFrom(0) 50 次 ─ 校验 500 条都在 + 顺序正确
  └─► Reader 4 ─ ReadFrom(0) 50 次 ─ 校验 500 条都在 + 顺序正确
  │
  ▼
断言：errCount=0 AND dataMismatch=0
```

**验证点**：5 条 reader 互不干扰，不会出现"reader A 把文件指针消耗到末尾 → reader B 读到空/截断数据"的竞争问题。

#### `TestConcurrentReadersDifferentOffsets` — 异偏移并发正确性

```
主线程：预加载 200 条条目
  │
  ├─ Run 0: Reader(off=0) / Reader(off=50) / Reader(off=100) / Reader(off=150)
  ├─ Run 1: Reader(off=0) / Reader(off=50) / Reader(off=100) / Reader(off=150)  ← 同偏移多实例
  └─ Run 2: Reader(off=0) / Reader(off=50) / Reader(off=100) / Reader(off=150)
  │
  ▼
每个 reader 30 轮迭代，校验：
  - 条目数量 = 200 - startOffset
  - 第 i 条 entry.Offset == startOffset + i
  - 第 i 条 entry.Data == "entry_{startOffset+i}"
```

**验证点**：即使多条 goroutine 从不同偏移量开始读取同一段文件，各自独立的文件句柄也能返回正确子集。同偏移多实例（Run 0/1/2 相同 startOffset）进一步验证高并发下的结果一致性。

#### `TestConcurrentReadersAcrossSegments` — 跨段并发拼接完整性

```
主线程：预加载 5 个日志段（MaxSegmentSize=250B 小尺寸强制切分）
  │
  ├─► Reader 0 ─ ReadFrom(0) 40 次 ─ 跨段读取 1-N 段
  ├─► Reader 1 ─ ReadFrom(0) 40 次 ─ 跨段读取 1-N 段
  ├─► Reader 2 ─ ReadFrom(0) 40 次 ─ 跨段读取 1-N 段
  └─► Reader 3 ─ ReadFrom(0) 40 次 ─ 跨段读取 1-N 段
  │
  ▼
校验：每条 reader 返回的所有段拼接后，entry.Data 前缀与全局索引一致
```

**验证点**：跨多段文件的 `ReadFrom` 拼接逻辑在并发下依然正确，所有段的独立句柄打开/关闭互不干扰。

### 并发测试保障总览

| 并发场景 | 测试覆盖 | 核心断言 |
|---------|---------|---------|
| 多 writer 并发写入 | `TestConcurrentAppend`（10 goroutine × 100 条） | 无重复偏移、总数 = 1000 |
| 2 reader + 单 writer 并行 | `TestConcurrentReadWrite`（2 reader × 1 writer） | 双方全量校验通过（200 条数据完整 + Offset + Data 一致） |
| 多 reader 同偏移 | `TestMultipleReadersConcurrent`（5 goroutine × 50 轮） | 数据完整一致、0 mismatch |
| 多 reader 异偏移 | `TestConcurrentReadersDifferentOffsets`（12 goroutine × 30 轮） | 数量与内容均匹配 |
| 多 reader 跨多段 | `TestConcurrentReadersAcrossSegments`（4 goroutine × 40 轮） | 跨段拼接完整正确 |

所有并发测试均使用 `-race` 友好的同步原语（`atomic` + `sync.WaitGroup`），可在 `go test -race` 下无警告通过。

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
