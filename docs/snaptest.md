# 快照测试工具模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [快照文件格式与存储](#4-快照文件格式与存储)
5. [Diff 算法原理](#5-diff-算法原理)
6. [差异展示格式说明](#6-差异展示格式说明)
7. [快照更新机制](#7-快照更新机制)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [配置说明](#10-配置说明)
11. [安全防护](#11-安全防护)
12. [最佳实践](#12-最佳实践)

---

## 1. 模块概述

快照测试（Snapshot Testing）是一种通过将当前输出与已保存的"黄金标准"快照进行比对来验证程序行为的测试方法。适用于输出内容复杂、结构多变但预期输出相对固定的场景，如 API 响应、复杂数据结构序列化结果、渲染输出等。

**包路径**: `internal/snaptest`

**设计目标**:
- 支持任意可 JSON 序列化的数据结构
- 快照文件使用人类可读的格式化 JSON 文本存储
- 测试运行时自动进行逐行文本比对
- 比对不一致时输出结构化的差异报告
- 提供显式的快照更新模式，避免意外覆盖
- 路径遍历防护，确保快照文件仅写入指定目录

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 数据序列化 | 将任意数据结构序列化为格式化 JSON 文本（2 空格缩进，禁用 HTML 转义） |
| 快照文件读写 | 按测试用例名称将快照写入 `__snapshots__/` 目录，文件扩展名为 `.snap` |
| 自动快照创建 | 首次运行时自动创建快照文件，无需手动初始化 |
| 逐行文本比对 | 基于 LCS（最长公共子序列）算法进行行级差异计算 |
| 差异可视化 | 以并排格式展示差异，删除行/新增行分别以 `-`/`+` 前缀标记 |
| 上下文保留 | 差异报告默认保留变更行前后各 3 行的上下文，支持自定义 |
| 快照更新模式 | 支持通过配置或环境变量 `SNAPTEST_UPDATE` 触发快照覆写 |
| 路径安全 | 对快照名称进行路径遍历检测，防止写入快照目录之外 |
| 便捷断言函数 | 提供 `Assert()` 包装函数，与 Go 标准 testing 框架无缝集成 |
| 换行符归一化 | 自动将 `\r\n` 转换为 `\n`，消除跨平台差异 |

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
- 实现 `Update()` 显式快照更新
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

表示差异比对中的单行结果。

```go
type DiffLine struct {
    Type     DiffType
    LeftNum  int
    RightNum int
    Content  string
}
```

| 字段 | 说明 |
|------|------|
| `Type` | 行类型：`DiffSame`（相同）、`DiffRemoved`（删除）、`DiffAdded`（新增） |
| `LeftNum` | 在期望（快照）中的行号，仅 `DiffSame` 和 `DiffRemoved` 有效 |
| `RightNum` | 在实际（当前输出）中的行号，仅 `DiffSame` 和 `DiffAdded` 有效 |
| `Content` | 行的文本内容 |

### 3.4 DiffResult

完整的差异比对结果。

```go
type DiffResult struct {
    Lines        []DiffLine
    TotalSame    int
    TotalAdded   int
    TotalRemoved int
}
```

**职责**:
- 存储逐行差异详情
- 通过 `Matches()` 判断是否完全一致
- 通过 `Format(contextLines int)` 生成人类可读的差异报告

### 3.5 DiffType

差异行类型枚举。

```go
const (
    DiffSame DiffType = iota
    DiffRemoved
    DiffAdded
)
```

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
  ],
  "Tags": [
    "a",
    "b"
  ]
}
```

### 4.3 归一化规则

读取和写入快照时会进行以下归一化处理：
1. 将 Windows 换行符 `\r\n` 统一转换为 `\n`
2. 去除文件末尾多余的空行
3. 写入时自动追加单个末尾换行符

---

## 5. Diff 算法原理

### 5.1 算法选择

模块采用 **LCS（Longest Common Subsequence，最长公共子序列）** 算法进行行级差异计算。这是 `diff` 工具的经典算法，能够产生最小数量的编辑操作（删除 + 新增）。

### 5.2 算法流程

1. 将期望文本（快照）和实际文本（当前输出）分别按行拆分为数组 `A` 和 `B`
2. 构建动态规划表 `dp[i][j]`，表示 `A[i:]` 与 `B[j:]` 的 LCS 长度
3. 从 `dp[0][0]` 开始回溯，提取出所有匹配的行对
4. 在匹配行对之间填充删除行（来自 `A`）和新增行（来自 `B`）

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

差异报告遵循类 unified diff 格式，由以下部分组成：

```
--- Expected (snapshot)
+++ Actual (current output)
@@ -<起始行号> +<起始行号> @@
  <左行号> <右行号> | <相同行内容>
- <左行号>      | <删除行内容>
+      <右行号> | <新增行内容>
  ...

Summary: <N> same, <N> removed, <N> added
```

### 6.2 前缀标记说明

| 前缀 | 含义 | 行号显示 |
|------|------|---------|
| `  `（双空格） | 相同行，同时存在于快照和当前输出中 | 同时显示左右行号 |
| `-` | 删除行，仅存在于快照中，当前输出中已移除 | 仅显示左侧（快照）行号，右侧以空格填充 |
| `+` | 新增行，仅存在于当前输出中，快照中不存在 | 仅显示右侧（当前）行号，左侧以空格填充 |

### 6.3 示例

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
line4
line5_new
```

差异报告输出：

```
--- Expected (snapshot)
+++ Actual (current output)
@@ -1 +1 @@
     1    1 | line1
-    2      | line2
+         2 | line2_modified
     3    3 | line3
     4    4 | line4
-    5      | line5
+         5 | line5_new

Summary: 3 same, 2 removed, 2 added
```

### 6.4 上下文折叠

当差异较大时，默认只显示变更行前后各 3 行上下文，未变化的大段内容以 `...` 标记折叠。可通过 `Config.ContextLines` 自定义上下文行数。

---

## 7. 快照更新机制

### 7.1 更新模式触发方式

快照更新通过以下两种方式触发（优先级从高到低）：

1. **配置显式指定**: `NewWithConfig(Config{UpdateMode: true})`
2. **环境变量**: 设置 `SNAPTEST_UPDATE=1`（或 `true`、`yes`，大小写不敏感）后调用 `New()`

### 7.2 更新模式行为

在更新模式下，`Match()` 和 `Assert()` 的行为发生变化：
- 跳过与已有快照的比对
- 直接将当前输出序列化并覆写对应快照文件
- 始终返回成功（除非发生 IO 错误）
- 不产生差异报告

### 7.3 安全性说明

- 快照文件**只有**在更新模式显式启用时才会被覆写
- 正常比对模式下，即使快照不存在也只会自动创建，不会修改已有快照
- 强烈建议将快照文件纳入版本控制，以便审查变更

---

## 8. 使用示例

### 8.1 基本使用（Assert 断言）

```go
package mypackage

import (
    "testing"
    "solocoder-go/internal/snaptest"
)

func TestMyFeature(t *testing.T) {
    result := ComplexFunction()
    snaptest.Assert(t, "TestMyFeature/basic", result)
}
```

首次运行时会自动创建快照文件，后续运行会自动比对。

### 8.2 自定义配置

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

### 8.3 使用 Match 获取详细信息

```go
func TestManualCheck(t *testing.T) {
    m := snaptest.New()
    data := GenerateOutput()

    ok, report, err := m.Match("manual_check", data)
    if err != nil {
        t.Fatal(err)
    }
    if !ok {
        t.Logf("差异详情:\n%s", report)
        t.Fail()
    }
}
```

### 8.4 显式更新快照

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

### 8.5 通过环境变量批量更新

在命令行运行时通过环境变量触发更新：

```bash
# Windows PowerShell
$env:SNAPTEST_UPDATE="1"; go test ./... -v

# Linux / macOS
SNAPTEST_UPDATE=1 go test ./... -v
```

### 8.6 序列化独立使用

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

### 8.7 Diff 功能独立使用

```go
expected := "a\nb\nc"
actual := "a\nX\nc"

diff := snaptest.Diff(expected, actual)
if !diff.Matches() {
    fmt.Println(diff.Format(3))
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrSnapshotNotFound` | 快照文件不存在 | 尝试读取不存在的快照（注意：首次 `Match()` 会自动创建，不会返回此错误） |
| `ErrSnapshotMismatch` | 快照比对不一致 | 预留常量，实际比对失败通过 `Match()` 返回值和差异报告体现 |
| `ErrInvalidName` | 无效的快照名称 | 名称为空、包含 `..` 路径遍历、`.` 等非法值 |
| `ErrSerialization` | 序列化失败 | 传入不可 JSON 序列化的数据类型（如包含通道、函数、循环引用等） |
| `ErrWriteSnapshot` | 写入快照失败 | 目录创建失败、磁盘无权限、IO 错误等 |
| `ErrReadSnapshot` | 读取快照失败 | 文件存在但无法读取（权限问题、磁盘损坏等） |

---

## 10. 配置说明

### 10.1 默认配置

`DefaultConfig()` 返回：

| 参数 | 值 |
|------|-----|
| `SnapshotDir` | `__snapshots__` |
| `UpdateMode` | `false` |
| `ContextLines` | `3` |

### 10.2 配置归一化

`NewWithConfig()` 会对传入配置做以下自动修正：

| 非法值 | 修正为 |
|--------|--------|
| `SnapshotDir == ""` | `"__snapshots__"` |
| `ContextLines <= 0` | `3` |

**注意**: `UpdateMode` 不会被归一化，调用方显式传入的值会被原样保留。环境变量检查仅在 `New()` 中进行。

---

## 11. 安全防护

### 11.1 路径遍历防护

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

### 11.2 自动创建目录

写入快照时会自动调用 `os.MkdirAll()` 创建缺失的中间目录（权限 0755）。

---

## 12. 最佳实践

### 12.1 快照命名规范

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

### 12.2 快照版本控制

- 将 `__snapshots__/` 目录纳入 Git 版本控制
- 代码评审时同时审查快照变更，确保输出变化符合预期
- 不要在 `.gitignore` 中排除 `.snap` 文件

### 12.3 更新快照工作流

推荐的快照更新流程：

1. 修改代码导致输出变化
2. 运行测试，确认快照不匹配是预期变更
3. 运行 `SNAPTEST_UPDATE=1 go test ./...` 更新所有快照
4. 使用 `git diff` 审查快照变更内容
5. 确认无误后提交代码和快照变更

### 12.4 测试稳定性注意事项

快照测试对输出变化极为敏感，以下做法可提高测试稳定性：

- **避免序列化非确定性数据**: 如时间戳、随机数、自增 ID 等，测试前应 mock 或替换为固定值
- **排除不稳定字段**: 如果某些字段不参与比对，应在序列化前从数据结构中移除
- **Map 顺序问题**: Go 的 `encoding/json` 会按键名排序序列化 map，通常不会有问题；如需严格顺序请使用结构体替代

### 12.5 与其他测试类型配合

快照测试适合：
- API 响应体结构验证
- 复杂模板渲染输出
- 数据转换/迁移结果验证
- 配置文件生成结果

快照测试不适合：
- 业务逻辑正确性验证（应使用单元测试 + 断言）
- 性能测试（快照测试有额外的 IO 开销）
- 包含大量动态数据的输出
