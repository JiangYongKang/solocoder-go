# 快照测试工具模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [快照文件格式与存储](#4-快照文件格式与存储)
5. [Diff 算法原理](#5-diff-算法原理)
6. [差异展示格式说明](#6-差异展示格式说明)
7. [快照创建与更新机制](#7-快照创建与更新机制)
8. [内容归一化规则](#8-内容归一化规则)
9. [使用示例](#9-使用示例)
10. [错误定义](#10-错误定义)
11. [配置说明](#11-配置说明)
12. [安全防护](#12-安全防护)
13. [最佳实践](#13-最佳实践)

---

## 1. 模块概述

快照测试（Snapshot Testing）是一种通过将当前输出与已保存的"黄金标准"快照进行比对来验证程序行为的测试方法。适用于输出内容复杂、结构多变但预期输出相对固定的场景，如 API 响应、复杂数据结构序列化结果、渲染输出等。

**包路径**: `internal/snaptest`

**设计目标**:
- 支持任意可 JSON 序列化的数据结构
- 快照文件使用人类可读的格式化 JSON 文本存储
- 测试运行时自动进行逐行文本比对
- 比对不一致时输出并排格式的差异报告
- 提供显式的快照创建与更新机制，避免意外覆盖
- 路径遍历防护，确保快照文件仅写入指定目录

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 数据序列化 | 将任意数据结构序列化为格式化 JSON 文本（2 空格缩进，禁用 HTML 转义） |
| 快照文件读写 | 按测试用例名称将快照写入 `__snapshots__/` 目录，文件扩展名为 `.snap` |
| 快照创建 | 需显式调用 `Update()` 或启用更新模式创建快照，不会自动创建 |
| 逐行文本比对 | 基于 LCS（最长公共子序列）算法进行行级差异计算 |
| 差异可视化 | 以并排（side-by-side）格式展示差异，四种类别：相同、删除、新增、修改 |
| 上下文保留 | 差异报告默认保留变更行前后各 3 行的上下文，支持自定义 |
| 快照更新模式 | 支持通过配置或环境变量 `SNAPTEST_UPDATE` 触发快照覆写 |
| 路径安全 | 对快照名称进行路径遍历检测，防止写入快照目录之外 |
| 便捷断言函数 | 提供 `Assert()` 包装函数，与 Go 标准 testing 框架无缝集成 |
| 换行符归一化 | 自动将 `\r\n` 转换为 `\n`，保留尾部空行，消除跨平台差异 |

---

## 3. 核心结构体与职责

### 3.1 Matcher

快照匹配器主结构体，封装快照的读写、比对与断言逻辑。

```go
type Matcher struct {
    cfg Config
}
```

**职责**:
- 管理快照目录配置与更新模式
- 提供 `Serialize()` 序列化入口
- 实现 `Match()` 核心比对逻辑
- 实现 `Update()` 显式快照创建/更新
- 实现 `Assert()` 测试断言封装
- 内部调用 `Diff()` 算法与 `Format()` 差异格式化

**构造函数**:
- `New()`: 使用默认配置，自动检查 `SNAPTEST_UPDATE` 环境变量
- `NewWithConfig(cfg Config)`: 使用自定义配置，完全信任调用方传入的参数

### 3.2 Config

配置结构体，定制快照匹配器行为。

```go
type Config struct {
    SnapshotDir  string
    UpdateMode   bool
    ContextLines int
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `SnapshotDir` | `string` | `__snapshots__` | 快照文件存储目录 |
| `UpdateMode` | `bool` | `false` | 是否处于更新模式（直接覆写快照） |
| `ContextLines` | `int` | `3` | 差异报告中变更行前后保留的上下文行数 |

### 3.3 DiffLine

表示差异比对中的单行结果，采用并排结构，同时存储左右两侧内容。

```go
type DiffLine struct {
    Type     DiffType
    LeftNum  int
    RightNum int
    Left     string
    Right    string
}
```

| 字段 | 说明 |
|------|------|
| `Type` | 行类型：`DiffSame`（相同）、`DiffRemoved`（删除）、`DiffAdded`（新增）、`DiffModified`（修改） |
| `LeftNum` | 在期望（快照）中的行号 |
| `RightNum` | 在实际（当前输出）中的行号 |
| `Left` | 左侧（快照）行的文本内容 |
| `Right` | 右侧（当前输出）行的文本内容 |

### 3.4 DiffResult

完整的差异比对结果。

```go
type DiffResult struct {
    Lines        []DiffLine
    TotalSame    int
    TotalAdded   int
    TotalRemoved int
    TotalModified int
}
```

**职责**:
- 存储逐行差异详情
- 通过 `Matches()` 判断是否完全一致
- 通过 `Format(contextLines int)` 生成人类可读的并排差异报告

### 3.5 DiffType

差异行类型枚举。

```go
const (
    DiffSame DiffType = iota
    DiffRemoved
    DiffAdded
    DiffModified
)
```

### 3.6 MismatchError

快照不匹配错误，包装了详细的差异信息。

```go
type MismatchError struct {
    Name   string
    Diff   DiffResult
    Report string
}
```

| 字段 | 说明 |
|------|------|
| `Name` | 快照名称 |
| `Diff` | 完整的差异结果对象 |
| `Report` | 格式化后的差异报告文本 |

该错误实现了 `Unwrap()` 方法，可通过 `errors.Is(err, ErrSnapshotMismatch)` 判断错误类型。

---

## 4. 快照文件格式与存储

### 4.1 文件命名规则

快照文件以测试用例名称命名，扩展名为 `.snap`，存储于 `SnapshotDir` 目录下：

```
测试用例名: "TestMyFeature/case1"
快照路径:   __snapshots__/TestMyFeature/case1.snap
```

支持通过 `/` 在名称中创建子目录结构，便于按测试套件组织快照。

### 4.2 文件内容格式

快照文件内容为格式化后的 JSON 文本，使用 2 空格缩进，以换行符结尾：

```json
{
  "ID": 1,
  "Name": "test",
  "Items": [
    1,
    2,
    3
  ]
}

```

### 4.3 归一化规则

读取和写入快照时会进行以下归一化处理（详见 [第 8 节](#8-内容归一化规则)：
1. 将 Windows 换行符 `\r\n` 统一转换为 `\n`
2. 保留所有尾部空行，不做裁剪
3. 写入时自动确保文件以单个换行符结尾（如果原内容无末尾换行时追加）

---

## 5. Diff 算法原理

### 5.1 算法选择

模块采用 **LCS（Longest Common Subsequence，最长公共子序列）** 算法进行行级差异计算。这是 `diff` 工具的经典算法，能够产生最小数量的编辑操作（删除 + 新增）。

### 5.2 算法流程

1. 将期望文本（快照）和实际文本（当前输出）分别按行拆分为数组 `A` 和 `B`
2. 构建动态规划表 `dp[i][j]`，表示 `A[i:]` 与 `B[j:]` 的 LCS 长度
3. 从 `dp[0][0]` 开始回溯，提取出所有匹配的行对
4. 在匹配行对之间填充删除行（来自 `A`）、新增行（来自 `B`）或修改行（两侧都有）

### 5.3 动态规划递推式

```
dp[i][j] = dp[i+1][j+1] + 1,              当 A[i] == B[j]
dp[i][j] = max(dp[i+1][j], dp[i][j+1]),   当 A[i] != B[j]
```

边界条件：`dp[len(A)][*] = 0`，`dp[*][len(B)] = 0`

### 5.4 时间与空间复杂度

- 时间复杂度：O(N × M)，其中 N、M 分别为期望和实际文本的行数
- 空间复杂度：O(N × M)，用于存储动态规划表

---

## 6. 差异展示格式说明

### 6.1 整体结构

差异报告采用**并排（side-by-side）**格式，左右两列分别展示快照内容和当前输出内容，中间以 `|` 分隔：

```
Expected (snapshot)                                           | Actual (current output)
------------------------------------------------------------+------------------------------------------------------------
    1 line1                                                  |    1 line1
~   2 old_line                                              |    2 new_line
    3 line3                                                 |    3 line3
-   4 only_in_snapshot                                        |
+                                                           |    4 only_in_actual

Summary: 2 same, 1 removed, 1 added, 1 modified
```

### 6.2 行类型标记说明

每行开头的符号表示该行的差异类型：

| 符号 | 类型 | 含义 | 左右内容 |
|------|------|------|---------|
| ` `（空格） | `DiffSame` | 相同行 | 左右两侧都有，内容一致 |
| `-` | `DiffRemoved` | 删除行 | 仅左侧（快照）有内容，右侧为空 |
| `+` | `DiffAdded` | 新增行 | 仅右侧（当前输出）有内容，左侧为空 |
| `~` | `DiffModified` | 修改行 | 左右两侧都有内容，但内容不同 |

### 6.3 格式详解

每一行的格式为：

```
<符号> <左行号> <左内容> | <右行号> <右内容>
```

- 行号占 4 个字符宽度，右对齐
- 内容列默认宽度为 60 字符宽度，超出时显示省略号 `...`
- 相同行和修改行左右都有行号
- 删除行只有左行号，右行号位置为空
- 新增行只有右行号，左行号位置为空

### 6.4 完整示例

假设有以下快照内容：

```
line1
line2
line3
line4
line5
```

当前输出为：

```
line1
line2_modified
line3
line5
line6_new
```

差异报告输出：

```
Expected (snapshot)                                           | Actual (current output)
------------------------------------------------------------+------------------------------------------------------------
    1 line1                                                  |    1 line1
~   2 line2                                                  |    2 line2_modified
    3 line3                                                 |    3 line3
-   4 line4                                                  |
    5 line5                                                 |    4 line5
+                                                           |    5 line6_new

Summary: 3 same, 1 removed, 1 added, 1 modified
```

### 6.5 上下文折叠

当差异较大时，默认只显示变更行前后各 3 行上下文，未变化的大段内容以 `...` 标记折叠。可通过 `Config.ContextLines` 自定义上下文行数。

---

## 7. 快照创建与更新机制

### 7.1 快照创建

快照**不会**在首次运行时自动创建。必须通过以下方式显式创建：

1. **调用 `Update()` 方法**：直接创建或覆写快照文件
2. **启用更新模式调用 `Match()`**：在更新模式下，`Match()` 会直接写入快照

### 7.2 快照比对流程

正常比对模式下的 `Match()` 方法的执行流程：

```
开始
  ↓
序列化当前输出
  ↓
读取快照文件 ── 不存在 ──→ 返回 ErrSnapshotNotFound 错误
  ↓ 存在
逐行比对快照与当前输出
  ↓
完全一致？ ── 是 ──→ 返回成功 (ok=true, report="", err=nil)
  ↓ 否
生成并排差异报告
  ↓
返回失败 + MismatchError 错误
  (ok=false, report=差异报告, err=MismatchError)
```

### 7.3 更新模式

更新模式通过以下两种方式触发（优先级从高到低）：

1. **配置显式指定**: `NewWithConfig(Config{UpdateMode: true})`
2. **环境变量**: 设置 `SNAPTEST_UPDATE=1`（或 `true`、`yes`，大小写不敏感）后调用 `New()`

在更新模式下，`Match()` 和 `Assert()` 的行为发生变化：
- 跳过与已有快照的比对
- 直接将当前输出序列化并覆写对应快照文件
- 始终返回成功（除非发生 IO 错误）
- 不产生差异报告

### 7.4 安全性说明

- 快照文件**只有**在更新模式显式启用时才会被创建或覆写
- 正常比对模式下，快照不存在时直接返回错误，不会创建或修改任何文件
- 强烈建议将快照文件纳入版本控制，以便审查变更

---

## 8. 内容归一化规则

### 8.1 换行符归一化

所有文本在进入 Diff 算法前都会经过 `normalizeSnapshotContent()` 处理：

1. **Windows 换行符转换**：将所有 `\r\n` 统一替换为 `\n`
2. **保留尾部空行**：**不**裁剪尾部的空行，完整保留原始内容中的所有换行符

### 8.2 设计原则

- 跨平台兼容：消除 Windows 和 Unix 换行符差异不会导致误报
- 数据完整性：尾部空行是内容的一部分，不应被静默丢弃
- 可预测性：用户输出是什么，快照就保存什么，不会意外裁剪

### 8.3 写入时的处理

写入快照文件时：
- 已包含换行统一为 `\n`
- 如果内容不以换行符结尾，自动追加一个 `\n`
- 如果内容已有换行符结尾，保持不变

### 8.4 读取时的处理

读取快照文件时：
- 将 `\r\n` 转换为 `\n`
- 保留所有原始换行符不变

---

## 9. 使用示例

### 9.1 基本使用（Assert 断言）

```go
package mypackage

import (
    "testing"
    "solocoder-go/internal/snaptest"
)

func TestMyFeature(t *testing.T) {
    // 首次运行前需要先创建快照
    // snaptest.Update("TestMyFeature/basic", ComplexFunction())

    result := ComplexFunction()
    snaptest.Assert(t, "TestMyFeature/basic", result)
}
```

### 9.2 创建快照

```go
func TestCreateSnapshots(t *testing.T) {
    testCases := []struct{
        name string
        data interface{}
    }{
        {"case1", GenerateCase1()},
        {"case2", GenerateCase2()},
    }

    m := snaptest.New()
    for _, tc := range testCases {
        if err := m.Update(tc.name, tc.data); err != nil {
            t.Fatalf("create snapshot %s failed: %v", tc.name, err)
        }
    }
}
```

### 9.3 自定义配置

```go
func TestWithCustomDir(t *testing.T) {
    m := snaptest.NewWithConfig(snaptest.Config{
        SnapshotDir:  "testdata/snapshots",
        ContextLines: 5,
    })

    result := ProcessData()
    m.Assert(t, "custom_dir_test", result)
}
```

### 9.4 使用 Match 获取详细信息

```go
func TestManualCheck(t *testing.T) {
    m := snaptest.New()
    data := GenerateOutput()

    ok, report, err := m.Match("manual_check", data)
    if err != nil {
        if errors.Is(err, snaptest.ErrSnapshotNotFound) {
            t.Skip("snapshot not found, skip test")
            return
        }
        t.Fatal(err)
    }
    if !ok {
        t.Logf("差异详情:\n%s", report)
        t.Fail()
    }
}
```

### 9.5 显式更新快照

```go
func TestUpdateSnapshots(t *testing.T) {
    m := snaptest.NewWithConfig(snaptest.Config{
        UpdateMode: true,
    })

    testCases := []struct{
        name string
        data interface{}
    }{
        {"case1", GenerateCase1()},
        {"case2", GenerateCase2()},
    }

    for _, tc := range testCases {
        if err := m.Update(tc.name, tc.data); err != nil {
            t.Fatalf("update %s failed: %v", tc.name, err)
        }
    }
}
```

### 9.6 通过环境变量批量更新

在命令行运行时通过环境变量触发更新：

```bash
# Windows PowerShell
$env:SNAPTEST_UPDATE="1"; go test ./... -v

# Linux / macOS
SNAPTEST_UPDATE=1 go test ./... -v
```

### 9.7 序列化独立使用

```go
import "solocoder-go/internal/snaptest"

data := map[string]interface{}{
    "users": []string{"alice", "bob"},
    "count": 2,
}

text, err := snaptest.Serialize(data)
if err != nil {
    panic(err)
}
fmt.Println(text)
```

输出：

```json
{
  "count": 2,
  "users": [
    "alice",
    "bob"
  ]
}
```

### 9.8 Diff 功能独立使用

```go
expected := "a\nb\nc"
actual := "a\nX\nc"

diff := snaptest.Diff(expected, actual)
if !diff.Matches() {
    fmt.Println(diff.Format(3))
}
```

### 9.9 处理不匹配错误

```go
ok, report, err := m.Match("my_snapshot", data)
if err != nil {
    var mismatchErr *snaptest.MismatchError
    if errors.As(err, &mismatchErr) {
        // 快照不匹配，访问详细信息
        fmt.Printf("快照 %q 不匹配\n", mismatchErr.Name)
        fmt.Printf("差异详情:\n%s", mismatchErr.Report)
        // mismatchErr.Diff 包含结构化的差异数据
    } else if errors.Is(err, snaptest.ErrSnapshotNotFound) {
        fmt.Println("快照不存在")
        // 可以选择创建快照或跳过测试
    } else {
        // 其他错误
    }
}
```

---

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrSnapshotNotFound` | 快照文件不存在 | 尝试读取不存在的快照，`Match()` 返回此错误 |
| `ErrSnapshotMismatch` | 快照比对不一致 | 快照存在但内容不匹配，通过 `MismatchError` 包装返回 |
| `ErrInvalidName` | 无效的快照名称 | 名称为空、包含 `..` 路径遍历、`.` 等非法值 |
| `ErrSerialization` | 序列化失败 | 传入不可 JSON 序列化的数据类型（如包含通道、函数、循环引用等） |
| `ErrWriteSnapshot` | 写入快照失败 | 目录创建失败、磁盘无权限、IO 错误等 |
| `ErrReadSnapshot` | 读取快照失败 | 文件存在但无法读取（权限问题、磁盘损坏等） |

### 10.1 MismatchError

`MismatchError` 是 `ErrSnapshotMismatch` 的具体实现，包含详细的差异信息：

```go
type MismatchError struct {
    Name   string      // 快照名称
    Diff   DiffResult // 结构化差异结果
    Report string    // 格式化的差异报告
}
```

使用 `errors.Is(err, ErrSnapshotMismatch)` 可判断是否为不匹配错误，使用 `errors.As(err, &mismatchErr)` 可获取详细信息。

---

## 11. 配置说明

### 11.1 默认配置

`DefaultConfig()` 返回：

| 参数 | 值 |
|------|-----|
| `SnapshotDir` | `__snapshots__` |
| `UpdateMode` | `false` |
| `ContextLines` | `3` |

### 11.2 配置归一化

`NewWithConfig()` 会对传入配置做以下自动修正：

| 非法值 | 修正为 |
|--------|--------|
| `SnapshotDir == ""` | `"__snapshots__` |
| `ContextLines <= 0` | `3` |

**注意**: `UpdateMode` 不会被归一化，调用方显式传入的值会被原样保留。环境变量检查仅在 `New()` 中进行。

---

## 12. 安全防护

### 12.1 路径遍历防护

模块对快照名称进行严格校验，防止通过 `../` 将快照写入或读取到配置目录之外：

- 名称不能为空
- 经过 `filepath.Clean()` 后不能等于 `.` 或 `..`
- 名称中不能包含 `..` 路径段

非法名称示例：
- `""`（空字符串）
- `"../etc/passwd"`
- `"foo/../../bar"`
- `".."`
- `"."`

### 12.2 自动创建目录

写入快照时会自动调用 `os.MkdirAll()` 创建缺失的中间目录（权限 0755）。

---

## 13. 最佳实践

### 13.1 快照命名规范

建议使用 `测试函数名/用例名` 的层级命名方式，便于组织：

```go
snaptest.Assert(t, "TestAPIResponse/ok_200", resp)
snaptest.Assert(t, "TestAPIResponse/not_found_404", resp)
snaptest.Assert(t, "TestAPIResponse/internal_error_500", resp)
```

对应文件结构：

```
__snapshots__/
└── TestAPIResponse/
    ├── ok_200.snap
    ├── not_found_404.snap
    └── internal_error_500.snap
```

### 13.2 快照版本控制

- 将 `__snapshots__/` 目录纳入 Git 版本控制
- 代码评审时同时审查快照变更，确保输出变化符合预期
- 不要在 `.gitignore` 中排除 `.snap` 文件

### 13.3 更新快照工作流

推荐的快照更新流程：

1. 修改代码导致输出变化
2. 运行测试，确认快照不匹配是预期变更
3. 运行 `SNAPTEST_UPDATE=1 go test ./...` 更新所有快照
4. 使用 `git diff` 审查快照变更内容
5. 确认无误后提交代码和快照变更

### 13.4 测试稳定性注意事项

快照测试对输出变化极为敏感，以下做法可提高测试稳定性：

- **避免序列化非确定性数据**: 如时间戳、随机数、自增 ID 等，测试前应 mock 或替换为固定值
- **排除不稳定字段**: 如果某些字段不参与比对，应在序列化前从数据结构中移除
- **Map 顺序问题**: Go 的 `encoding/json` 会按键名排序序列化 map，通常不会有问题；如需严格顺序请使用结构体替代
- **尾部空行**: 注意尾部空行也会参与比对，确保输出一致性

### 13.5 与其他测试类型配合

快照测试适合：
- API 响应体结构验证
- 复杂模板渲染输出
- 数据转换/迁移结果验证
- 配置文件生成结果

快照测试不适合：
- 业务逻辑正确性验证（应使用单元测试 + 断言）
- 性能测试（快照测试有额外的 IO 开销）
- 包含大量动态数据的输出
