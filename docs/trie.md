# Trie 前缀树数据结构模块

## 1. 模块概述

Trie（前缀树）是一种树形数据结构，用于高效地存储和检索字符串集合中的键。本模块实现了一个功能完整的前缀树，支持单词插入、精确查找、前缀匹配、通配符模式搜索和最大匹配前缀查询，广泛应用于自动补全、拼写检查、分词系统等场景。

**包路径**: `internal/trie`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 单词插入 | 支持向 Trie 中插入单词及其附加数据，每个字符对应树中的一个节点 |
| 精确查找 | 支持精确查找某个完整单词是否存在于 Trie 中，并返回该单词的附加数据 |
| 前缀匹配 | 输入一个前缀字符串，返回 Trie 中所有以该前缀开头的完整单词列表，结果按字典序排列，支持限制返回数量上限 |
| 通配符模式搜索 | 支持在搜索模式中使用 `.` 匹配任意单个字符和 `*` 匹配任意连续字符序列，模式搜索时遍历所有匹配的单词路径返回完整匹配结果集合 |
| 最大匹配前缀查询 | 输入一个字符串，返回 Trie 中与该字符串前缀匹配的最长单词，用于分词场景中按最长匹配原则从文本中切出已注册的单词 |
| 单词删除 | 支持从 Trie 中删除指定单词，并自动清理不再需要的空节点 |
| 并发安全 | 所有操作均支持并发安全，使用读写锁保证多协程环境下的数据一致性 |

## 3. 核心结构体与职责

### 3.1 Trie

前缀树的主结构体，对外提供所有操作接口。

```go
type Trie struct {
    root *trieNode
    mu   sync.RWMutex
    size int
}
```

**职责**:
- 维护树的根节点和读写锁
- 提供 Insert、Search、Delete、PrefixMatch、WildcardSearch、LongestMatch 等操作入口
- 跟踪树中单词的总数
- 确保所有操作的并发安全性

### 3.2 trieNode

前缀树的节点结构，内部使用。

```go
type trieNode struct {
    children map[rune]*trieNode
    isEnd    bool
    data     interface{}
}
```

**职责**:
- `children`: 存储子节点的映射，键为字符（rune），值为子节点指针
- `isEnd`: 标记该节点是否是某个单词的结尾
- `data`: 存储单词对应的附加数据，可以是任意类型

### 3.3 SearchResult

搜索结果结构体，用于返回匹配的单词及其附加数据。

```go
type SearchResult struct {
    Word string
    Data interface{}
}
```

**职责**:
- `Word`: 匹配到的完整单词
- `Data`: 该单词对应的附加数据

## 4. 核心算法机制

### 4.1 单词插入机制

插入操作将单词的每个字符依次映射到前缀树的节点路径上：

1. 从根节点开始，遍历单词的每个字符
2. 对于当前字符，检查当前节点的子节点中是否存在该字符的映射
3. 如果不存在，创建一个新的子节点并添加到映射中
4. 移动到对应的子节点，继续处理下一个字符
5. 遍历完成后，将当前节点标记为单词结尾（`isEnd = true`），并存储附加数据
6. 如果是新单词（之前该节点未标记为结尾），则单词总数加 1
7. 重复插入同一单词会覆盖原有的附加数据，但单词总数不变

### 4.2 精确查找机制

精确查找验证单词是否完整存在于 Trie 中：

1. 从根节点开始，依次匹配单词的每个字符
2. 如果任何一个字符在当前节点的子节点中不存在，查找失败
3. 遍历完所有字符后，检查当前节点的 `isEnd` 标记
4. 如果 `isEnd` 为 `true`，则单词存在，返回附加数据和 `true`
5. 否则，返回 `nil` 和 `false`

### 4.3 前缀匹配机制

前缀匹配返回所有以指定前缀开头的单词：

1. 从根节点开始，依次匹配前缀的每个字符
2. 如果任何一个字符不存在，返回空结果集
3. 到达前缀的最后一个字符对应的节点后，进行深度优先搜索（DFS）
4. 遍历该节点的所有子树，收集所有 `isEnd` 为 `true` 的节点
5. 将收集到的单词按字典序排序后返回
6. 支持通过 `maxResults` 参数限制返回数量（0 表示不限制）

### 4.4 通配符搜索机制

通配符搜索支持两种通配符：
- `.`: 匹配任意单个字符
- `*`: 匹配任意连续字符序列（包括空序列）

**算法流程**:

1. 使用深度优先搜索（DFS）遍历前缀树
2. 对于当前模式字符：
   - 如果是 `*`，有两种选择：
     1. 跳过该 `*`（匹配空序列），继续匹配下一个模式字符
     2. 使用该 `*` 匹配当前节点的某个子字符，保持在 `*` 模式位置继续匹配
   - 如果是 `.`，匹配当前节点的任意一个子字符，继续匹配下一个模式字符
   - 如果是普通字符，仅当当前节点有对应的子字符时才继续匹配
3. 当模式遍历完成时，如果当前节点是单词结尾，则将该单词加入结果集
4. 使用 `seen` map 防止连续 `*` 导致的重复匹配
5. 最终结果按字典序排序

### 4.5 最大匹配前缀机制

最大匹配前缀查询返回与输入字符串前缀匹配的最长单词，适用于分词场景：

1. 从根节点开始，依次匹配输入字符串的每个字符
2. 每当遇到 `isEnd` 为 `true` 的节点时，记录当前匹配的单词作为候选结果
3. 继续匹配更长的前缀，不断更新最长匹配结果
4. 当无法继续匹配（字符不存在）或字符串遍历完成时停止
5. 如果找到了任何匹配，返回最长的那个；否则返回错误

**最长匹配分词示例**:

对于词典 ["上海", "上海市", "上海自来水", "来自", "自来水"]，文本 "上海自来水来自海上" 的分词过程：
1. 从位置 0 开始，最长匹配为 "上海自来水"（5 个字符）
2. 移动到位置 5，最长匹配为 "来自"（2 个字符）
3. 移动到位置 7，无匹配，跳过
4. 最终分词结果: ["上海自来水", "来自"]

### 4.6 单词删除机制

删除操作不仅标记单词不存在，还会清理不再需要的节点：

1. 从根节点开始，沿着单词路径遍历，记录经过的节点和字符路径
2. 如果路径中断或最终节点不是单词结尾，返回未找到错误
3. 将目标节点的 `isEnd` 设为 `false`，清空附加数据，单词总数减 1
4. 从单词的最后一个字符开始，反向遍历路径：
   - 如果当前节点不是单词结尾且没有子节点，则从父节点中删除该节点
   - 否则停止清理（该节点可能是其他单词的路径部分）

## 5. API 参考

### 5.1 构造函数

```go
func NewTrie() *Trie
```

创建一个新的空前缀树。

### 5.2 基本操作

```go
func (t *Trie) Insert(word string, data interface{}) error
func (t *Trie) Search(word string) (interface{}, bool, error)
func (t *Trie) Delete(word string) error
func (t *Trie) Size() int
func (t *Trie) GetAllWords() []SearchResult
```

### 5.3 前缀匹配

```go
func (t *Trie) PrefixMatch(prefix string) ([]SearchResult, error)
func (t *Trie) PrefixMatchLimit(prefix string, maxResults int) ([]SearchResult, error)
```

### 5.4 通配符搜索

```go
func (t *Trie) WildcardSearch(pattern string) ([]SearchResult, error)
```

### 5.5 最大匹配前缀查询

```go
func (t *Trie) LongestMatch(query string) (SearchResult, error)
```

## 6. 错误定义

| 错误 | 触发场景 |
|------|----------|
| `ErrEmptyWord` | 插入或删除空单词 |
| `ErrEmptyPrefix` | 前缀匹配时空前缀 |
| `ErrEmptyPattern` | 通配符搜索时空模式 |
| `ErrEmptyQuery` | 最大匹配查询时空查询 |
| `ErrInvalidMaxResult` | 最大结果数为负数 |
| `ErrWordNotFound` | 删除不存在的单词或最长匹配无结果 |

## 7. 使用示例

### 7.1 基本插入与查询

```go
trie := trie.NewTrie()

// 插入单词及其附加数据
trie.Insert("hello", "greeting")
trie.Insert("world", "planet")

// 精确查找
data, exists, _ := trie.Search("hello")
// data = "greeting", exists = true

data, exists, _ = trie.Search("unknown")
// data = nil, exists = false

// 空字符串搜索返回错误
_, _, err := trie.Search("")
// err = ErrEmptyWord

// 获取单词总数
size := trie.Size()
// size = 2
```

### 7.2 前缀匹配

```go
trie := trie.NewTrie()
trie.Insert("apple", "fruit")
trie.Insert("app", "abbreviation")
trie.Insert("application", "software")
trie.Insert("banana", "fruit")

// 获取所有以 "app" 开头的单词
results, _ := trie.PrefixMatch("app")
// results = [
//   {Word: "app", Data: "abbreviation"},
//   {Word: "apple", Data: "fruit"},
//   {Word: "application", Data: "software"}
// ]

// 限制最多返回 2 个结果
results, _ = trie.PrefixMatchLimit("app", 2)
// results = [
//   {Word: "app", Data: "abbreviation"},
//   {Word: "apple", Data: "fruit"}
// ]
```

### 7.3 通配符搜索

```go
trie := trie.NewTrie()
trie.Insert("cat", "animal")
trie.Insert("car", "vehicle")
trie.Insert("bat", "animal")
trie.Insert("rat", "animal")
trie.Insert("apple", "fruit")
trie.Insert("appreciate", "verb")

// 使用 . 匹配单个字符
results, _ := trie.WildcardSearch(".at")
// 匹配: cat, bat, rat

// 使用 * 匹配任意序列
results, _ = trie.WildcardSearch("app*")
// 匹配: apple, appreciate

results, _ = trie.WildcardSearch("*e")
// 匹配: apple, appreciate

// 组合使用
results, _ = trie.WildcardSearch("a.*e")
// 匹配: apple, appreciate

// 匹配所有单词
results, _ = trie.WildcardSearch("*")
// 匹配所有已插入的单词
```

### 7.4 最大匹配前缀查询（分词示例）

```go
trie := trie.NewTrie()
trie.Insert("中", "zhōng")
trie.Insert("中国", "Zhōngguó")
trie.Insert("中国人", "Zhōngguórén")
trie.Insert("中国人民", "Zhōngguórénmín")

// 查询最长匹配
result, _ := trie.LongestMatch("中国人民银行")
// result.Word = "中国人民"
// result.Data = "Zhōngguórénmín"

// 中文分词示例
trie.Insert("上海", "city")
trie.Insert("上海市", "municipality")
trie.Insert("上海自来水", "water_company")
trie.Insert("来自", "from")
trie.Insert("自来水", "tap_water")

text := "上海自来水来自海上"
var segments []string

i := 0
for i < len([]rune(text)) {
    substr := string([]rune(text)[i:])
    result, err := trie.LongestMatch(substr)
    if err != nil {
        i++
        continue
    }
    segments = append(segments, result.Word)
    i += len([]rune(result.Word))
}
// segments = ["上海自来水", "来自"]
```

### 7.5 单词删除

```go
trie := trie.NewTrie()
trie.Insert("hello", "data")
trie.Insert("hell", "data")

// 删除单词
err := trie.Delete("hello")
// err = nil

_, exists := trie.Search("hello")
// exists = false

_, exists = trie.Search("hell")
// exists = true（路径节点被保留，因为 "hell" 仍然存在）

// 删除不存在的单词
err = trie.Delete("world")
// err = ErrWordNotFound
```

### 7.6 使用结构体作为附加数据

```go
type WordInfo struct {
    ID       int
    Category string
    Frequency int
}

trie := trie.NewTrie()
trie.Insert("apple", WordInfo{ID: 1, Category: "fruit", Frequency: 100})

data, exists, _ := trie.Search("apple")
if exists {
    info := data.(WordInfo)
    // info.ID = 1
    // info.Category = "fruit"
    // info.Frequency = 100
}
```

### 7.7 获取所有单词

```go
trie := trie.NewTrie()
trie.Insert("banana", "3")
trie.Insert("apple", "1")
trie.Insert("cherry", "2")

results := trie.GetAllWords()
// 结果按字典序排序:
// [{Word: "apple", Data: "1"}, {Word: "banana", Data: "3"}, {Word: "cherry", Data: "2"}]
```

### 7.8 Unicode 支持

```go
trie := trie.NewTrie()
trie.Insert("你好", "hello")
trie.Insert("你好世界", "hello world")
trie.Insert("こんにちは", "konnichiwa")

results, _ := trie.PrefixMatch("你好")
// 匹配: "你好", "你好世界"

result, _ := trie.LongestMatch("你好世界")
// result.Word = "你好世界"
```
