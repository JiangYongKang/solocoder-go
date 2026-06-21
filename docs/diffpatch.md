# 文本差异与补丁引擎 (DiffPatch) 模块需求文档

## 1. 模块概述

文本差异与补丁引擎是一个基于 Myers 差分算法的文本比较与合并组件，提供行级差异计算、统一差异格式（Unified Diff）补丁生成、补丁应用校验以及三方合并冲突处理四大核心能力。模块适用于代码审查工具、版本控制系统、协同编辑器、配置文件同步等需要精确文本差异比较与自动合并的场景。

## 2. 模块功能清单

| 编号 | 功能名称 | 描述 |
|------|----------|------|
| F1 | 行级差异计算 (Diff) | 基于 Myers 算法计算两段文本的最短编辑脚本，输出包含删除行和新增行的差异结果 |
| F2 | 差异结果标注 | 每个变更块标注原文本和新文本中的起止行号（OldStart/OldCount/NewStart/NewCount） |
| F3 | 统一差异格式生成 (GeneratePatch) | 将差异结果序列化为标准 Unified Diff 格式的补丁文本 |
| F4 | 补丁解析 (ParsePatch) | 将 Unified Diff 格式的补丁文本解析为结构化的 Patch 对象 |
| F5 | 补丁应用 (ApplyPatch) | 将补丁应用到原始文本上生成新文本，校验上下文匹配性 |
| F6 | 上下文校验 | 应用补丁时逐块验证上下文行是否与原始文本匹配，匹配失败则拒绝并返回冲突信息 |
| F7 | 三方合并 (ThreeWayMerge) | 基于原始版本、当前版本和补丁目标版本的三方差异合并 |
| F8 | 冲突检测与标记 | 合并时检测重叠变更，自动合并非冲突部分，标记冲突区域供人工介入 |

## 3. 核心结构体与职责

### 3.1 LineType - 行类型枚举

```go
type LineType int

const (
    LineEqual  LineType = iota // 上下文行（未变更）
    LineDelete                  // 删除行
    LineInsert                  // 新增行
)
```

### 3.2 Line - 差异行

```go
type Line struct {
    Content   string   // 行内容（不含换行符）
    Type      LineType // 行类型
    OldLineNo int      // 原文本行号（删除行和上下文行有效，新增行为 0）
    NewLineNo int      // 新文本行号（新增行和上下文行有效，删除行为 0）
}
```

**主要职责：** 表示差异结果中的单行信息，携带行内容、变更类型和在原/新文本中的行号。

### 3.3 Hunk - 变更块

```go
type Hunk struct {
    OldStart int     // 原文本起始行号（1-based）
    OldCount int     // 原文本行数（包含上下文行和删除行）
    NewStart int     // 新文本起始行号（1-based）
    NewCount int     // 新文本行数（包含上下文行和新增行）
    Lines    []Line  // 变更块中的所有行
}
```

**主要职责：** 表示一个连续的变更区域，包含行号范围和具体的差异行。每个 Hunk 对应 Unified Diff 中的一个 `@@ ... @@` 块。

### 3.4 DiffResult - 差异计算结果

```go
type DiffResult struct {
    Hunks []Hunk // 所有变更块
}
```

**主要职责：** 承载差异计算的完整结果。当两段文本完全相同时，Hunks 为空切片。

### 3.5 PatchHeader - 补丁文件头

```go
type PatchHeader struct {
    OldFile string // 原文件名（对应 --- 行）
    NewFile string // 新文件名（对应 +++ 行）
}
```

### 3.6 Patch - 补丁结构

```go
type Patch struct {
    Header PatchHeader // 补丁文件头
    Hunks  []Hunk      // 所有变更块
}
```

**主要职责：** 表示一个完整的 Unified Diff 补丁，包含文件名信息和所有变更块。

### 3.7 ConflictRange - 冲突范围

```go
type ConflictRange struct {
    StartLine int       // 冲突起始行号
    EndLine   int       // 冲突结束行号
    Ours      []string  // 当前版本的冲突行内容
    Theirs    []string  // 补丁目标版本的冲突行内容
    Base      []string  // 原始版本的对应行内容
}
```

**主要职责：** 描述合并冲突的区域，携带三方版本的行内容，供调用方展示和人工解决。

### 3.8 ApplyResult - 补丁应用结果

```go
type ApplyResult struct {
    Text      string          // 应用后的文本（冲突时为部分应用结果）
    Rejected  bool            // 是否有变更块被拒绝
    Conflicts []ConflictRange // 冲突信息列表
}
```

### 3.9 MergeResult - 合并结果

```go
type MergeResult struct {
    Text         string          // 合并后的文本（冲突时包含冲突标记）
    HasConflicts bool            // 是否存在无法自动解决的冲突
    Conflicts    []ConflictRange // 冲突详情列表
}
```

### 3.10 预定义错误

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyInput` | 输入文本为空 | 调用 `Diff("", "")` 两段文本都为空时 |
| `ErrInvalidPatch` | 补丁格式无效 | `ParsePatch` 遇到无法解析的补丁文本时 |
| `ErrPatchConflict` | 补丁上下文不匹配 | 保留错误，用于扩展场景 |
| `ErrMergeConflict` | 合并冲突 | 保留错误，用于扩展场景 |

## 4. 核心机制详解

### 4.1 Myers 差分算法

Myers 算法在编辑图（Edit Graph）上寻找从 (0,0) 到 (N,M) 的最短路径，其中：
- 向右移动 = 从原文本删除一行
- 向下移动 = 向新文本插入一行
- 对角线移动 = 相同行（无变更）

**算法步骤：**
1. 对每个编辑距离 d（从 0 到 N+M），在 k 值范围 [-d, d] 内搜索
2. 对每个 k 值，计算能到达的最远 x 坐标，并沿对角线延伸
3. 当到达 (N, M) 时，记录搜索轨迹
4. 通过回溯轨迹重建最短编辑脚本

**时间复杂度：** O((N+M)D)，其中 D 为编辑距离。对于差异较小的文本比较，性能接近 O(N+M)。

### 4.2 变更块（Hunk）构建

差异计算后，将编辑脚本分组为变更块：
1. 上下文行数默认为 3 行（前后各 3 行）
2. 相邻变更之间的上下文行数 ≤ 6 行时，合并为同一个变更块
3. 超过 6 行上下文间隔时，拆分为独立的变更块
4. 文件开头和结尾的上下文行数可能少于 3 行

### 4.3 统一差异格式（Unified Diff）输出规范

补丁文本遵循标准 Unified Diff 格式：

```
--- 原文件名
+++ 新文件名
@@ -OldStart,OldCount +NewStart,NewCount @@
 上下文行（空格前缀）
-删除行（减号前缀）
+新增行（加号前缀）
 上下文行
```

**格式约定：**
- 文件头：`---` 行标记原文件名，`+++` 行标记新文件名
- 变更块头：`@@ -OldStart,OldCount +NewStart,NewCount @@` 标注行号范围
- 行前缀：空格表示上下文行，`-` 表示删除行，`+` 表示新增行
- 行号从 1 开始计数
- 行内容不含尾部换行符（换行符由格式本身隐含）

**特殊情形：**
- 空文件差异：当原文本为空时，`@@ -0,0 +1,N @@` 表示全部为新增
- 当新文本为空时，`@@ -1,N +0,0 @@` 表示全部为删除
- 当 OldCount 或 NewCount 为 1 时，可省略逗号和计数部分

### 4.4 补丁应用流程

```
ApplyPatch(originalText, patchText)
   │
   ├─ ParsePatch(patchText) → Patch 对象
   │     解析失败 → 返回 ErrInvalidPatch
   │
   ├─ 逐块应用每个 Hunk
   │     │
   │     ├─ 跳过 Hunk.OldStart 之前的原始文本行（直接复制到结果）
   │     │
   │     ├─ 校验上下文：verifyContext()
   │     │     │
   │     │     ├─ 匹配成功：执行删除和插入操作
   │     │     │     - LineEqual → 复制原始行到结果
   │     │     │     - LineDelete → 跳过原始行
   │     │     │     - LineInsert → 添加新行到结果
   │     │     │
   │     │     └─ 匹配失败：记录冲突，保留原始行
   │     │           创建 ConflictRange，标记 Rejected=true
   │     │
   │     └─ 继续处理下一个 Hunk
   │
   └─ 返回 ApplyResult
         - Text: 应用后的完整文本
         - Rejected: 是否有冲突
         - Conflicts: 冲突详情列表
```

**上下文校验机制：**
- 对每个变更块，从 `OldStart-1` 位置开始逐行比对
- 上下文行（LineEqual）和删除行（LineDelete）必须与原始文本完全匹配
- 新增行（LineInsert）不需要在原始文本中匹配
- 任何一行不匹配即判定该变更块冲突

### 4.5 三方合并策略

三方合并基于原始版本（Base）、当前版本（Ours）和补丁目标版本（Theirs）进行：

**快速路径：**
1. Base == Ours → 直接采用 Theirs
2. Base == Theirs → 直接采用 Ours
3. Ours == Theirs → 两者相同，直接采用

**合并流程：**
```
ThreeWayMerge(base, ours, theirs)
   │
   ├─ [快速路径判断] → 直接返回
   │
   ├─ Diff(base, ours) → oursChanges
   ├─ Diff(base, theirs) → theirsChanges
   │
   ├─ diffToChanges() 将 DiffResult 转为 change 列表
   │     每个 change 包含：oldStart, oldEnd, newLines
   │
   ├─ detectConflicts() 检测重叠变更
   │     │
   │     ├─ 两个 change 的行范围重叠且内容不同 → 冲突
   │     │
   │     └─ 两个 change 内容完全相同 → 非冲突（自动合并）
   │
   ├─ [无冲突]
   │     applyBothChanges() → 按位置排序合并两个变更集
   │     返回 MergeResult{HasConflicts: false}
   │
   └─ [有冲突]
        mergeWithConflictMarkers() → 插入冲突标记
        返回 MergeResult{HasConflicts: true, Conflicts: [...]}
```

**冲突标记格式：**
```
<<<<<<< ours
当前版本的冲突行
=======
补丁目标版本的冲突行
>>>>>>> theirs
```

**重叠判定规则：**
- 两个 change 起始位置相同 → 重叠
- 两个 change 行范围有交叉 → 重叠
- 两个 change 内容完全相同 → 不视为冲突（自动解决）

## 5. 使用示例

### 5.1 计算两段文本的差异

```go
package main

import (
    "fmt"
    "solocoder-go/internal/diffpatch"
)

func main() {
    oldText := "line1\nline2\nline3\nline4\n"
    newText := "line1\nLINE2\nline3\nline4\nadded\n"

    result, err := diffpatch.Diff(oldText, newText)
    if err != nil {
        fmt.Printf("差异计算失败: %v\n", err)
        return
    }

    for _, hunk := range result.Hunks {
        fmt.Printf("@@ -%d,%d +%d,%d @@\n",
            hunk.OldStart, hunk.OldCount,
            hunk.NewStart, hunk.NewCount)
        for _, line := range hunk.Lines {
            switch line.Type {
            case diffpatch.LineEqual:
                fmt.Printf(" %s\n", line.Content)
            case diffpatch.LineDelete:
                fmt.Printf("-%s\n", line.Content)
            case diffpatch.LineInsert:
                fmt.Printf("+%s\n", line.Content)
            }
        }
    }
}
```

### 5.2 生成并应用补丁

```go
oldText := "a\nb\nc\n"
newText := "a\nB\nc\n"

patch, err := diffpatch.GeneratePatch("old.txt", "new.txt", oldText, newText)
if err != nil {
    log.Fatal(err)
}
fmt.Println("生成的补丁:")
fmt.Println(patch)

applyResult, err := diffpatch.ApplyPatch(oldText, patch)
if err != nil {
    log.Fatal(err)
}
if applyResult.Rejected {
    fmt.Println("补丁应用冲突:")
    for _, c := range applyResult.Conflicts {
        fmt.Printf("  行 %d-%d 冲突\n", c.StartLine, c.EndLine)
    }
} else {
    fmt.Println("补丁应用成功:")
    fmt.Println(applyResult.Text)
}
```

### 5.3 三方合并

```go
base := "line1\nline2\nline3\nline4\nline5\n"
ours := "line1\nMODIFIED2\nline3\nline4\nline5\n"
theirs := "line1\nline2\nline3\nline4\nMODIFIED5\n"

result, err := diffpatch.ThreeWayMerge(base, ours, theirs)
if err != nil {
    log.Fatal(err)
}

if result.HasConflicts {
    fmt.Println("合并存在冲突:")
    fmt.Println(result.Text)
    for _, c := range result.Conflicts {
        fmt.Printf("冲突区域行 %d-%d:\n", c.StartLine, c.EndLine)
        fmt.Println(diffpatch.FormatConflict(c))
    }
} else {
    fmt.Println("合并成功:")
    fmt.Println(result.Text)
}
```

### 5.4 冲突场景处理

```go
base := "a\nb\nc\n"
ours := "a\nOURS\nc\n"
theirs := "a\nTHEIRS\nc\n"

result, err := diffpatch.ThreeWayMerge(base, ours, theirs)
if err != nil {
    log.Fatal(err)
}

// result.HasConflicts == true
// result.Text 包含冲突标记:
// a
// <<<<<<< ours
// OURS
// =======
// THEIRS
// >>>>>>> theirs
// c
```

## 6. 文件结构

```
internal/diffpatch/
├── diffpatch.go       # 核心类型定义和公共入口函数
├── myers.go           # Myers 差分算法实现和变更块构建
├── patch.go           # 统一差异格式生成与解析
├── apply.go           # 补丁应用与上下文校验
├── merge.go           # 三方合并与冲突检测
└── diffpatch_test.go  # 单元测试

docs/
└── diffpatch.md       # 本文档
```

## 7. 测试覆盖说明

单元测试覆盖以下场景类别：

| 测试类别 | 代表性测试用例 | 覆盖目标 |
|----------|---------------|----------|
| **差异计算基础** | `TestDiff_SimpleInsertion`、`TestDiff_SimpleDeletion`、`TestDiff_SimpleModification` | 插入、删除、修改行 |
| **差异计算边界** | `TestDiff_BothEmpty`、`TestDiff_OldEmpty`、`TestDiff_NewEmpty`、`TestDiff_IdenticalTexts` | 空文本、相同文本 |
| **行号标注** | `TestDiff_HunkLineNumbers` | OldStart/NewStart/OldCount/NewCount 正确性 |
| **多变更块** | `TestDiff_MultipleChanges`、`TestDiff_ConsecutiveModifications` | 多处修改、连续修改 |
| **补丁生成** | `TestGeneratePatch_Basic`、`TestGeneratePatch_AllDeleted`、`TestGeneratePatch_AllInserted` | 标准生成、全删除、全插入 |
| **补丁解析** | `TestParsePatch_Valid`、`TestParsePatch_Invalid`、`TestParsePatch_MultipleHunks` | 正常解析、格式错误、多变更块 |
| **补丁应用** | `TestApplyPatch_Basic`、`TestApplyPatch_Insertion`、`TestApplyPatch_Deletion` | 基本应用、插入、删除 |
| **上下文校验** | `TestApplyPatch_ContextVerificationSuccess`、`TestApplyPatch_ContextVerificationFailure`、`TestApplyPatch_ConflictingContext` | 校验通过、校验失败、冲突返回 |
| **三方合并正常** | `TestThreeWayMerge_OursNoChange`、`TestThreeWayMerge_NonOverlappingChanges`、`TestThreeWayMerge_BothSameChange` | 单方变更、非重叠变更、相同变更 |
| **三方合并冲突** | `TestThreeWayMerge_ConflictingChanges`、`TestThreeWayMerge_DeleteVsModify`、`TestThreeWayMerge_EmptyBase` | 双方冲突、删除与修改冲突、空基线冲突 |
| **端到端** | `TestEndToEnd_DiffGenerateApply`、`TestEndToEnd_DiffMerge` | 完整工作流验证 |
| **辅助函数** | `TestSplitLines`、`TestRangesOverlap`、`TestSameContent` | 内部工具函数正确性 |
