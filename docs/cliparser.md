# CLI 命令行参数解析器模块

## 1. 模块功能概述

`cliparser` 模块提供了一套功能完整的命令行参数解析解决方案，支持长短选项绑定、子命令路由、位置参数绑定、布尔标志以及默认值设定。该模块位于 `internal/cliparser/` 包下，主要功能包括：

- **长短选项绑定**：支持 `--verbose` 长选项和 `-v` 短选项两种格式，两者可通过同一个 Option 结构体关联，解析时两种格式均可识别并正确赋值到同一个目标变量。
- **子命令路由**：支持定义多个子命令（如 `add`、`remove`、`list`），每个子命令拥有独立的选项集和处理函数，解析器根据第一个非选项位置参数路由到对应的子命令，并继续解析该子命令的后续参数。
- **位置参数绑定**：支持按位置顺序将命令行中的非选项参数绑定到指定的目标变量，位置参数的个数、顺序和类型均可配置，参数数量不匹配时返回明确错误。
- **布尔标志与默认值**：布尔类型选项不需要显式传值，在命令行中出现即为 `true`，不出现即为 `false`（或使用配置的默认值）；所有类型的选项均支持配置默认值，当命令行未提供该选项时使用默认值自动填充。
- **多类型支持**：内置支持 `string`、`int`、`float64`、`bool` 四种常用参数类型，解析时自动进行类型转换，转换失败返回错误。
- **组合短选项**：支持将多个布尔短选项合并书写（如 `-vdf` 等价于 `-v -d -f`），以及布尔选项与需传值选项的混合组合（如 `-vfnvalue` 等价于 `-v -f -n value`）。
- **等号赋值语法**：长选项支持 `--key=value` 形式的等号赋值语法。
- **终止符支持**：支持 `--` 终止符，之后的所有参数均视为位置参数，即使以 `-` 开头也不被解析为选项。

## 2. 核心结构体与职责

### 2.1 OptionType 类型

```go
type OptionType int

const (
    StringType OptionType = iota
    IntType
    BoolType
    FloatType
)
```

- **职责**：枚举选项/参数的类型，用于解析时的类型转换和校验
- **取值**：
  - `StringType`：字符串类型
  - `IntType`：整型
  - `BoolType`：布尔类型
  - `FloatType`：浮点类型

### 2.2 Option 结构体

```go
type Option struct {
    Long        string       // 长选项名称（不含 -- 前缀）
    Short       string       // 短选项名称（单个字符，不含 - 前缀）
    Description string       // 选项描述
    Type        OptionType   // 选项值类型
    Default     interface{}  // 默认值
    Target      interface{}  // 目标变量指针
}
```

- **职责**：定义一个命令行选项（可选参数）的元信息和绑定关系
- **字段说明**：
  - `Long`：长选项名，如 `"verbose"`，解析时自动加上 `--` 前缀匹配
  - `Short`：短选项名，单个字符如 `"v"`，解析时自动加上 `-` 前缀匹配
  - `Description`：选项的文字描述，用于生成帮助信息
  - `Type`：选项值的类型，控制解析时的转换逻辑
  - `Default`：默认值，当命令行未提供该选项时使用此值填充目标变量
  - `Target`：目标变量的指针，必须为 `*string`、`*int`、`*bool` 或 `*float64` 类型之一

**使用约束**：
- `Long` 和 `Short` 至少提供一个，可同时提供形成长短选项绑定
- `Target` 不能为空指针
- `Short` 若提供必须为单个 ASCII 字符
- `Default` 若提供，其类型必须与 `Type` 和 `Target` 的指向类型一致

### 2.3 PositionalArg 结构体

```go
type PositionalArg struct {
    Name        string       // 参数名称
    Description string       // 参数描述
    Type        OptionType   // 参数值类型
    Target      interface{}  // 目标变量指针
}
```

- **职责**：定义一个位置参数的元信息和绑定关系
- **字段说明**：
  - `Name`：参数名，用于错误提示和帮助信息
  - `Description`：参数的文字描述
  - `Type`：参数值的类型
  - `Target`：目标变量指针

位置参数按注册顺序与命令行中非选项参数依次匹配。

### 2.4 HandlerFunc 类型

```go
type HandlerFunc func() error
```

- **职责**：子命令的处理函数类型，解析完成后通过 `Execute()` 调用
- **返回值**：处理过程中的错误，`nil` 表示成功

### 2.5 Command 结构体

```go
type Command struct {
    Name        string
    Description string
    Options     []*Option
    Args        []*PositionalArg
    Handler     HandlerFunc
    optionsMap  map[string]*Option
}
```

- **职责**：表示一个子命令，封装子命令的名称、选项、位置参数和处理函数
- **字段说明**：
  - `Name`：子命令名称，解析器通过第一个位置参数匹配此字段进行路由
  - `Description`：子命令描述
  - `Options`：该子命令专属的选项列表
  - `Args`：该子命令的位置参数列表
  - `Handler`：子命令的处理函数

**主要方法**：
- `NewCommand(name string) *Command`：创建新子命令
- `(c *Command) AddOption(opt *Option) error`：为子命令添加选项
- `(c *Command) AddPositionalArg(arg *PositionalArg) error`：为子命令添加位置参数

### 2.6 Parser 结构体

```go
type Parser struct {
    AppName     string
    Description string
    Options     []*Option
    Commands    []*Command
    commandsMap map[string]*Command
    optionsMap  map[string]*Option
    args        []*PositionalArg
    parsedCmd   *Command
}
```

- **职责**：核心解析器，负责全局选项注册、子命令注册、参数解析和执行调度
- **核心功能**：
  - 注册全局选项（可出现在子命令之前）
  - 注册子命令
  - 注册全局位置参数（无子命令模式下使用）
  - 执行参数解析
  - 调用匹配子命令的处理函数

**主要方法**：
- `NewParser(appName string) *Parser`：创建新的解析器实例
- `(p *Parser) AddOption(opt *Option) error`：注册全局选项
- `(p *Parser) AddCommand(cmd *Command) error`：注册子命令
- `(p *Parser) AddPositionalArg(arg *PositionalArg) error`：注册全局位置参数
- `(p *Parser) Parse(args []string) error`：执行参数解析
- `(p *Parser) Execute() error`：执行匹配到的子命令处理函数
- `(p *Parser) GetCommand() *Command`：获取解析后匹配到的子命令

## 3. 参数解析完整流程

### 3.1 初始化阶段

1. **创建 Parser**：通过 `NewParser(appName)` 创建解析器实例
2. **注册全局选项**：调用 `AddOption()` 添加所有全局选项，内部同时建立 `--long` 和 `-s` 到 Option 的索引映射
3. **注册子命令**：
   - 为每个子命令调用 `NewCommand()` 创建实例
   - 为子命令调用 `AddOption()` 和 `AddPositionalArg()` 注册其专属选项和位置参数
   - 设置子命令的 `Handler` 处理函数
   - 调用 Parser 的 `AddCommand()` 将子命令注册到解析器，内部建立命令名到 Command 的索引映射
4. **注册全局位置参数**（无子命令模式）：调用 `AddPositionalArg()` 添加全局位置参数

### 3.2 解析阶段（Parse 执行流程）

`Parse(args []string)` 按以下步骤执行：

**Step 1：应用全局默认值**
- 遍历所有全局选项，对设有 `Default` 且未显式指定的选项预先将默认值写入目标变量

**Step 2：逐个参数扫描**
- 维护索引 `i` 从 0 开始遍历 `args` 切片
- 对每个参数按以下分类处理：

**(a) 终止符 `--`**
- 若当前参数恰好为 `--`，将剩余所有参数追加为位置参数，终止扫描

**(b) 长选项 `--name[=value]`**
- 检查是否包含 `=`：
  - 包含：拆分为名称 `--name` 和值 `value`
  - 不包含：名称为完整参数，值需从下一个参数获取
- 在索引映射中查找选项：
  - 若已进入子命令上下文，先查子命令选项映射，再查全局选项映射
  - 未进入子命令上下文，只查全局选项映射
- 找不到选项 → 返回 `ErrUnknownOption`
- 找到选项后按类型处理：
  - `BoolType`：若有显式值则解析布尔值并写入，无显式值直接设为 `true`
  - 其他类型：若未用 `=` 提供值则取下一参数作为值（无下一个参数返回 `ErrMissingValue`），将字符串值转换为目标类型后写入，转换失败返回 `ErrInvalidType`
- 标记该选项为"已显式提供"

**(c) 短选项 `-abc[value]`**
- 从左到右逐个处理短选项字符：
  - 每个字符 `-c` 查找对应 Option
  - 找不到 → 返回 `ErrUnknownOption`
  - `BoolType` 选项：立即设为 `true`，继续处理下一字符
  - 非布尔选项：
    - 若短选项字符串还有剩余字符 → 剩余部分整体作为值
    - 否则取下一个参数作为值（无则返回 `ErrMissingValue`）
    - 类型转换后写入目标变量，终止短选项扫描
- 全部为布尔选项 → 所有标志设为 `true`

**(d) 子命令路由（尚未进入子命令且已注册子命令）**
- 当前参数不为空且不以 `-` 开头
- 在子命令映射中查找匹配的命令名
- 找到 → 记录当前子命令上下文 `parsedCmd`，后续选项解析优先查找该子命令的选项
- 找不到 → 返回 `ErrUnknownCommand`

**(e) 位置参数**
- 不满足以上任何条件的参数视为位置参数，按出现顺序收集

**Step 3：后处理**
- 所有参数扫描完成后：
  - 若解析器注册了子命令但未匹配到任何子命令 → 返回 `ErrNoCommand`
  - 对所有"未显式提供"且设有默认值的全局选项和子命令选项再次应用默认值（覆盖 bool 默认 false 场景）
  - 根据当前上下文（子命令或全局）将收集的位置参数依次绑定到 `PositionalArg` 定义：
    - 位置参数数量多于定义 → 返回 `ErrTooManyArgs`
    - 位置参数数量少于定义 → 返回 `ErrTooFewArgs`
    - 按类型逐个转换并写入目标变量，转换失败返回 `ErrInvalidType`

### 3.3 执行阶段（可选）

- 解析成功后，调用 `Execute()` 触发当前匹配子命令的 `Handler` 函数
- 若未匹配子命令或未设置 Handler → 返回 `ErrCommandNotFound`
- Handler 返回的错误由 `Execute()` 透传

## 4. 使用示例

### 4.1 基本用法 - 全局选项 + 位置参数

```go
package main

import (
    "fmt"
    "os"
    "solocoder-go/internal/cliparser"
)

func main() {
    p := cliparser.NewParser("myapp")

    var verbose bool
    var config string
    var port int

    _ = p.AddOption(&cliparser.Option{
        Long:    "verbose",
        Short:   "v",
        Type:    cliparser.BoolType,
        Target:  &verbose,
    })
    _ = p.AddOption(&cliparser.Option{
        Long:    "config",
        Short:   "c",
        Type:    cliparser.StringType,
        Default: "/etc/myapp.conf",
        Target:  &config,
    })
    _ = p.AddOption(&cliparser.Option{
        Long:    "port",
        Short:   "p",
        Type:    cliparser.IntType,
        Default: 8080,
        Target:  &port,
    })

    var inputFile, outputFile string
    _ = p.AddPositionalArg(&cliparser.PositionalArg{
        Name:   "input",
        Type:   cliparser.StringType,
        Target: &inputFile,
    })
    _ = p.AddPositionalArg(&cliparser.PositionalArg{
        Name:   "output",
        Type:   cliparser.StringType,
        Target: &outputFile,
    })

    if err := p.Parse(os.Args[1:]); err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    fmt.Printf("verbose=%v, config=%s, port=%d\n", verbose, config, port)
    fmt.Printf("input=%s, output=%s\n", inputFile, outputFile)
}
```

执行示例：
```bash
$ myapp -v --port=9090 in.txt out.txt
verbose=true, config=/etc/myapp.conf, port=9090
input=in.txt, output=out.txt

$ myapp -c custom.conf -- in.txt -notoption
verbose=false, config=custom.conf, port=8080
input=in.txt, output=-notoption
```

### 4.2 子命令模式 - 增删改查工具

```go
package main

import (
    "fmt"
    "os"
    "solocoder-go/internal/cliparser"
)

func main() {
    p := cliparser.NewParser("todo")

    var verbose bool
    _ = p.AddOption(&cliparser.Option{
        Long:   "verbose",
        Short:  "v",
        Type:   cliparser.BoolType,
        Target: &verbose,
    })

    // --- add 子命令 ---
    addCmd := cliparser.NewCommand("add")
    var addTitle string
    var addPriority int
    var addDone bool
    _ = addCmd.AddOption(&cliparser.Option{
        Long:    "priority",
        Short:   "p",
        Type:    cliparser.IntType,
        Default: 1,
        Target:  &addPriority,
    })
    _ = addCmd.AddOption(&cliparser.Option{
        Long:   "done",
        Short:  "d",
        Type:   cliparser.BoolType,
        Target: &addDone,
    })
    _ = addCmd.AddPositionalArg(&cliparser.PositionalArg{
        Name:   "title",
        Type:   cliparser.StringType,
        Target: &addTitle,
    })
    addCmd.Handler = func() error {
        fmt.Printf("[add] title=%q priority=%d done=%v verbose=%v\n",
            addTitle, addPriority, addDone, verbose)
        return nil
    }
    _ = p.AddCommand(addCmd)

    // --- remove 子命令 ---
    removeCmd := cliparser.NewCommand("remove")
    var removeId int
    _ = removeCmd.AddPositionalArg(&cliparser.PositionalArg{
        Name:   "id",
        Type:   cliparser.IntType,
        Target: &removeId,
    })
    removeCmd.Handler = func() error {
        fmt.Printf("[remove] id=%d verbose=%v\n", removeId, verbose)
        return nil
    }
    _ = p.AddCommand(removeCmd)

    // --- list 子命令 ---
    listCmd := cliparser.NewCommand("list")
    var listFilter string
    var listLimit int
    _ = listCmd.AddOption(&cliparser.Option{
        Long:    "filter",
        Short:   "f",
        Type:    cliparser.StringType,
        Default: "all",
        Target:  &listFilter,
    })
    _ = listCmd.AddOption(&cliparser.Option{
        Long:    "limit",
        Short:   "l",
        Type:    cliparser.IntType,
        Default: 20,
        Target:  &listLimit,
    })
    listCmd.Handler = func() error {
        fmt.Printf("[list] filter=%q limit=%d verbose=%v\n",
            listFilter, listLimit, verbose)
        return nil
    }
    _ = p.AddCommand(listCmd)

    if err := p.Parse(os.Args[1:]); err != nil {
        fmt.Fprintln(os.Stderr, "Error:", err)
        os.Exit(1)
    }

    if err := p.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, "Execute error:", err)
        os.Exit(1)
    }
}
```

执行示例：
```bash
$ todo add "Buy milk"
[add] title="Buy milk" priority=1 done=false verbose=false

$ todo -v add -dp2 "Write report"
[add] title="Write report" priority=2 done=true verbose=true

$ todo remove 42
[remove] id=42 verbose=false

$ todo list --limit=50 -f pending
[list] filter="pending" limit=50 verbose=false
```

## 5. 错误处理

模块定义了以下错误变量，均使用 `errors.Is()` 进行语义判断：

| 错误 | 说明 |
|------|------|
| `ErrUnknownOption` | 遇到未注册的选项（长选项或短选项） |
| `ErrMissingValue` | 需要取值的选项（非布尔）未提供值 |
| `ErrInvalidType` | 值无法转换为目标类型（如将 `"abc"` 转为 int） |
| `ErrUnknownCommand` | 注册了子命令但匹配到未注册的命令名 |
| `ErrNoCommand` | 注册了子命令但命令行中未指定任何子命令 |
| `ErrTooManyArgs` | 提供的位置参数数量多于定义 |
| `ErrTooFewArgs` | 提供的位置参数数量少于定义 |
| `ErrShortOptionFormat` | 注册短选项时名称长度不为 1 |
| `ErrDuplicateOption` | 重复注册相同名称的选项或子命令 |
| `ErrNilTarget` | Option 或 PositionalArg 的目标指针为 nil |
| `ErrCommandNotFound` | Execute() 时未找到可执行的命令或 Handler |

**典型错误场景**：
- 注册期错误：`ErrNilTarget`、`ErrShortOptionFormat`、`ErrDuplicateOption`、`ErrUnknownCommand`（空命令名）等，应在开发阶段通过程序初始化检查避免
- 解析期错误：`ErrUnknownOption`、`ErrMissingValue`、`ErrInvalidType`、`ErrUnknownCommand`、`ErrNoCommand`、`ErrTooManyArgs`、`ErrTooFewArgs`，通常由用户输入导致，应友好提示用户
- 执行期错误：`ErrCommandNotFound`、Handler 自定义错误

## 6. 设计要点与约束

### 6.1 长短选项绑定机制

长短选项的绑定通过"共享同一个 Option 实例"实现。注册选项时，若 `Long` 和 `Short` 同时非空，会在内部索引映射中分别以 `"--" + Long` 和 `"-" + Short` 为键，指向同一个 Option 对象，因此解析任一形式都会修改同一个目标变量。

### 6.2 默认值的两次应用

默认值在解析流程中会被应用两次：
1. **解析开始前**：将所有带默认值的选项预先写入目标变量（防止后续引用到未初始化的 Go 零值）
2. **解析结束后**：仅对"未在命令行中显式提供"的选项再次应用默认值

两次应用保证了：
- 布尔选项的默认值可被覆盖（如 `Default: true` 配合命令行中不提供 → 最终为 true）
- 显式提供的值不会被默认值覆盖

### 6.3 子命令选项查找优先级

进入子命令上下文后，选项查找按"子命令选项 → 全局选项"的顺序进行。这意味着：
- 子命令可以定义与全局选项同名的选项，子命令定义的优先级更高（覆盖全局）
- 未被子命令覆盖的全局选项在子命令上下文中依然有效

### 6.4 双破折号终止符

`--` 之后的所有参数一律视为位置参数，典型用途：
- 传递以 `-` 开头的位置参数（如文件名 `-` 表示标准输入）
- 分隔程序自身参数和后续需要透传给子进程的参数

### 6.5 组合短选项规则

- 短选项可以任意组合书写
- 组合串中出现的第一个非布尔选项会终止组合解析，其后续字符（或下一个参数）作为该选项的值
- 纯布尔组合可任意长度，每个字符对应一个标志

## 7. 测试覆盖

单元测试覆盖了以下场景（共 76 个测试用例）：

**基础结构测试**：
- Parser 和 Command 创建
- 各种 nil/空参数的注册期错误
- 重复注册、格式错误等边界情况
- 所有错误变量的字符串校验

**选项解析测试**：
- 长选项独立传值（`--name value`）
- 长选项等号赋值（`--name=value`）
- 短选项独立传值（`-n value`）
- 短选项拼接传值（`-nvalue`）
- 布尔选项的出现/缺失语义
- 布尔选项的显式赋值
- int / float / string / bool 四种类型的解析和类型转换
- 非法类型值的错误返回
- 长短选项绑定一致性
- 组合布尔短选项（`-vdf`）
- 布尔组合 + 需值选项（`-vfn value`）
- 默认值应用和显式值覆盖
- 未知选项错误
- 缺少选项值错误

**子命令测试**：
- 单/多子命令注册和路由
- 未知子命令错误
- 未指定子命令错误
- 全局选项与子命令的顺序（全局选项可出现在子命令之前）
- 子命令内部选项解析（长短选项、等号、组合等）
- 子命令级默认值
- 子命令 Handler 执行和错误透传
- Execute 边界错误

**位置参数测试**：
- string / int / float / bool 四种类型位置参数绑定
- 位置参数数量不匹配（过多/过少）
- 位置参数类型转换错误
- 位置参数与选项混合顺序
- 双破折号终止符后的位置参数
- 子命令级位置参数

运行测试：
```bash
go test ./internal/cliparser/ -v
```
