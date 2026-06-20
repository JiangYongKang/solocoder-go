# Suggest 搜索建议引擎模块

## 1. 模块概述

Suggest 是一个高性能的内存搜索建议引擎模块，专为需要自动补全、拼写纠错、热门搜索和搜索历史功能的场景设计。模块提供了完整的前缀树（Trie）索引构建、编辑距离计算、词频统计和用户历史记录管理等功能，并通过读写锁机制实现安全的并发访问。

**包路径**: `internal/suggest`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 前缀树自动补全 | 使用 Trie 数据结构存储候选搜索词，支持按前缀快速检索所有匹配词 |
| 拼写纠错 | 基于编辑距离（Levenshtein Distance）算法，为查询词推荐最相似的候选词 |
| 词频排序 | 为每个候选词维护搜索频率计数器，自动补全和纠错结果按频率从高到低排序 |
| 搜索历史 | 独立管理用户搜索历史，按时间倒序排列，支持添加、查询和清空操作 |
| 综合建议 | 组合自动补全和拼写纠错结果，提供统一的搜索建议接口 |
| 热门搜索 | 获取按搜索频率排序的热门搜索词列表 |
| 动态词库 | 支持动态添加、删除候选词，词库可实时更新 |
| 并发安全 | 内置读写锁保护，支持多 goroutine 并发读写 |

## 3. 核心结构体与职责

### 3.1 SuggestEngine

搜索建议引擎主结构体，对外提供所有操作接口。

```go
type SuggestEngine struct {
    trie         *Trie
    history      *SearchHistory
    mu           sync.RWMutex
    maxEditDist  int
    defaultLimit int
}
```

**职责**:
- 管理前缀树（`trie`），提供候选词的存储与检索
- 管理搜索历史（`history`），记录用户搜索行为
- 协调自动补全和拼写纠错，提供综合建议
- 维护配置参数（最大编辑距离、默认返回条数等）
- 提供词库管理、搜索提交、历史查询等对外 API
- 通过 `mu` 引擎级读写锁保护所有公开方法，确保并发安全

### 3.2 频率计数策略

模块采用**两级频率计数**策略，明确区分「词库初始化」和「实际搜索」对频率的不同影响：

| 操作 | 对频率的影响 | 适用场景 |
|------|-------------|----------|
| `AddWord(word)` | 词加入词库，频率初始化为 **0** | 初始化词库、批量导入候选词 |
| `AddWordWithFreq(word, freq)` | 词加入词库，频率设置为 **指定值** | 初始化时设置已有热度的热门词 |
| `SubmitSearch(userID, word)` | 词不存在则加入词库且频率 = **1**；词已存在则频率 **+1** | 用户实际提交搜索时 |
| `RemoveWord(word)` | 词从词库移除，频率归零 | 删除不再需要的候选词 |

**设计考量**:
- `AddWord` 频率从 0 开始，确保初始化导入的词不会凭借虚假的搜索频率占据热门词榜单
- `SubmitSearch` 是频率递增的唯一来源，真实反映用户搜索行为
- `AddWordWithFreq` 提供灵活的初始化能力，用于从外部系统导入已有热度数据的场景
- 热门词排序（`GetHotWords`）严格基于实际搜索频率，确保榜单可信度

### 3.2 Config

引擎配置结构体。

```go
type Config struct {
    MaxEditDistance  int
    DefaultMaxResult int
    HistoryMaxSize   int
}
```

**字段说明**:
- `MaxEditDistance`: 拼写纠错的最大容忍编辑距离，默认 2
- `DefaultMaxResult`: 默认最大返回结果数，默认 10
- `HistoryMaxSize`: 每个用户历史记录的最大条数，默认 100

### 3.3 Trie

前缀树（字典树）数据结构，用于高效存储和检索候选词。

```go
type Trie struct {
    root *trieNode
    mu   sync.RWMutex
    size int
}
```

**职责**:
- `root`: 前缀树根节点
- `mu`: 读写锁，保护并发访问
- `size`: 当前词库中的词条总数
- 提供词的插入、删除、搜索、前缀匹配等操作

### 3.4 trieNode

前缀树节点结构体。

```go
type trieNode struct {
    children map[rune]*trieNode
    isEnd    bool
    freq     int
}
```

**职责**:
- `children`: 子节点映射，key 为字符（rune），value 为子节点指针
- `isEnd`: 标记该节点是否为某个词的结尾
- `freq`: 该词的搜索频率计数

### 3.5 Suggestion

搜索建议结果结构体。

```go
type Suggestion struct {
    Word      string
    Frequency int
}
```

**职责**:
- `Word`: 候选词文本
- `Frequency`: 该词的搜索频率

### 3.6 SearchHistory

搜索历史管理器。

```go
type SearchHistory struct {
    mu       sync.RWMutex
    history  map[string][]*HistoryRecord
    maxSize  int
}
```

**职责**:
- `history`: 用户 ID 到历史记录列表的映射
- `maxSize`: 每个用户历史记录的最大条数
- 提供历史记录的添加、查询、清空和计数功能
- 历史记录与候选词库完全分离，避免历史噪声污染

### 3.7 HistoryRecord

单条历史记录结构体。

```go
type HistoryRecord struct {
    Word      string
    Timestamp time.Time
}
```

**职责**:
- `Word`: 搜索词
- `Timestamp`: 搜索时间戳

## 4. 前缀树构建与查询算法

### 4.1 前缀树插入算法

```
Insert(word)
    │
    ▼
  参数校验
  └─ word 为空? ──是──► 返回 ErrEmptyWord
    │
    ▼
  获取写锁 (Trie.mu.Lock)
    │
    ▼
  node = root
  遍历 word 的每个字符 ch:
    │
    ├─ node.children[ch] 不存在?
    │  └─ 是 → 创建新的 trieNode
    │
    └─ node = node.children[ch]
    │
    ▼
  检查 node.isEnd:
  ├─ 否 → node.isEnd = true，size++
  └─ 是 → 词已存在，仅更新频率
    │
    ▼
  node.freq++
    │
    ▼
  释放写锁
    │
    ▼
  返回 nil (成功)
```

**算法特点**:
- 时间复杂度: O(L)，L 为词的字符长度
- 每个字符仅需一次 map 查找和可能的节点创建
- 重复插入时仅增加频率计数，不增加节点

### 4.2 前缀树删除算法

```
Delete(word)
    │
    ▼
  参数校验
  └─ word 为空? ──是──► 返回 ErrEmptyWord
    │
    ▼
  获取写锁
    │
    ▼
  遍历 word，记录路径 (path) 和字符路径 (charPath)
  └─ 任一字符不存在? ──是──► 返回 ErrWordNotFound
    │
    ▼
  检查末尾节点 isEnd:
  └─ 为 false? ──是──► 返回 ErrWordNotFound
    │
    ▼
  标记删除:
  ├─ node.isEnd = false
  ├─ node.freq = 0
  └─ size--
    │
    ▼
  从底向上清理空节点:
  └─ 遍历 path（从后往前）:
     ├─ child = parent.children[ch]
     └─ 若 !child.isEnd && len(child.children) == 0:
        └─ 删除该子节点
     └─ 否则: 停止清理
    │
    ▼
  释放写锁
    │
    ▼
  返回 nil (成功)
```

**算法特点**:
- 自动清理无用节点，节省内存空间
- 当路径上某个节点还有其他子节点或本身是词尾时停止删除
- 时间复杂度: O(L)

### 4.3 前缀查询算法

```
StartsWith(prefix)
    │
    ▼
  参数校验
  └─ prefix 为空? ──是──► 返回 ErrEmptyPrefix
    │
    ▼
  获取读锁
    │
    ▼
  定位前缀节点:
  └─ 遍历 prefix 每个字符:
     └─ 字符不存在? ──是──► 返回空结果 []
    │
    ▼
  深度优先搜索 (DFS) 收集所有词:
  └─ collectWords(node, prefix, results):
     ├─ 若 node.isEnd: 添加到 results
     └─ 遍历所有子节点: 递归调用 collectWords
    │
    ▼
  结果排序:
  └─ 按 Frequency 降序
     └─ 频率相同时按 Word 字典序升序
    │
    ▼
  释放读锁
    │
    ▼
  返回排序后的结果
```

**算法特点**:
- 前缀定位时间复杂度: O(P)，P 为前缀长度
- 词收集时间复杂度: O(K)，K 为匹配词的总字符数
- 结果排序按频率优先，字典序次之

## 5. 编辑距离算法

### 5.1 算法原理

本模块使用 **Levenshtein 距离**（莱文斯坦距离）作为编辑距离的度量，定义为将一个字符串转换为另一个字符串所需的最少单字符操作数，支持三种操作：
- **插入** (Insertion): 添加一个字符
- **删除** (Deletion): 删除一个字符
- **替换** (Substitution): 将一个字符替换为另一个字符

### 5.2 动态规划实现

算法使用动态规划求解，空间优化版使用两个一维数组交替计算：

```
设 dp[i][j] 表示字符串 a 的前 i 个字符转换为字符串 b 的前 j 个字符所需的最小编辑距离

递推公式:
    dp[i][j] = min(
        dp[i-1][j] + 1,      // 删除 a 的第 i 个字符
        dp[i][j-1] + 1,      // 插入 b 的第 j 个字符
        dp[i-1][j-1] + cost  // 替换（或相同则 0）
    )

其中 cost = 0 if a[i-1] == b[j-1] else 1

边界条件:
    dp[0][j] = j  // 空串转 b 的前 j 个字符，需要 j 次插入
    dp[i][0] = i  // a 的前 i 个字符转空串，需要 i 次删除
```

**空间优化**:
- 使用两个一维数组 `prev` 和 `curr` 交替计算
- 空间复杂度从 O(m×n) 优化为 O(n)
- 时间复杂度保持 O(m×n)

### 5.3 Unicode 支持

算法使用 `rune` 处理 Unicode 字符，支持中文等多字节字符：
- 使用 `utf8.RuneCountInString` 获取字符长度
- 将字符串转换为 `[]rune` 进行逐字符比较

## 6. 拼写纠错算法

### 6.1 纠错流程

```
Correct(query, maxResults)
    │
    ▼
  参数校验
  ├─ query 为空? ──是──► 返回 ErrEmptyQuery
  └─ maxResults <= 0? ──是──► 返回 ErrInvalidMaxResult
    │
    ▼
  检查 query 是否已在词库中:
  └─ 存在? ──是──► 返回空结果 [] (词正确无需纠错)
    │
    ▼
  获取所有候选词 (GetAllWords)
  └─ 词库为空? ──是──► 返回空结果 []
    │
    ▼
  计算编辑距离，筛选候选:
  └─ 遍历每个候选词 word:
     ├─ dist = EditDistance(query, word)
     └─ dist <= maxEditDist? → 加入候选列表
    │
    ▼
  候选排序 (优先级从高到低):
  1. 编辑距离从小到大
  2. 搜索频率从高到低
  3. 字典序从小到大
    │
    ▼
  截取前 maxResults 个结果
    │
    ▼
  返回纠错建议列表
```

### 6.2 排序规则

拼写纠错结果按以下优先级排序：

1. **编辑距离优先**: 编辑距离越小，排名越靠前
2. **词频次之**: 编辑距离相同时，搜索频率越高排名越靠前
3. **字典序兜底**: 前两者都相同时，按字典序升序排列

## 7. 搜索历史管理

### 7.1 历史记录特点

- **分离存储**: 历史记录与候选词库完全独立管理，避免历史噪声污染词库
- **时间倒序**: 历史记录按搜索时间从近到远排列
- **去重置顶**: 重复搜索的词会被移到历史列表顶部，并更新时间戳
- **容量限制**: 每个用户的历史记录有最大条数限制，超出时丢弃最旧的记录

### 7.2 添加历史记录流程

```
Add(userID, word)
    │
    ▼
  参数校验
  ├─ userID 为空? ──是──► 返回 ErrEmptyUserID
  └─ word 为空? ──是──► 返回 ErrEmptyWord
    │
    ▼
  获取写锁
    │
    ▼
  查找该词是否已在历史中:
  └─ 已存在?
     ├─ 是 → 移到列表头部，更新时间戳
     └─ 否 → 创建新记录，插入列表头部
    │
    ▼
  检查是否超出 maxSize:
  └─ 超出? → 移除列表尾部的旧记录
    │
    ▼
  释放写锁
    │
    ▼
  返回 nil (成功)
```

## 8. 综合建议 (Suggest)

### 8.1 综合建议策略

综合建议接口 (`Suggest`/`SuggestLimit`) 组合自动补全和拼写纠错的结果，策略如下：

1. **优先返回自动补全结果**: 前缀匹配的词通常更相关
2. **不足时用纠错补充**: 当自动补全结果不足时，用拼写纠错结果补充
3. **去重处理**: 避免同一词同时出现在自动补全和纠错结果中

### 8.2 综合建议流程

```
SuggestLimit(query, maxResults)
    │
    ▼
  参数校验
  ├─ query 为空? ──是──► 返回 ErrEmptyQuery
  └─ maxResults <= 0? ──是──► 返回 ErrInvalidMaxResult
    │
    ▼
  获取自动补全结果:
  └─ autocomplete = AutocompleteLimit(query, maxResults)
    │
    ▼
  自动补全结果足够?
  └─ 是 → 直接返回 autocomplete[:maxResults]
    │
    ▼
  计算剩余名额: remaining = maxResults - len(autocomplete)
    │
    ▼
  获取拼写纠错结果:
  └─ corrections = CorrectLimit(query, remaining)
    │
    ▼
  去重合并:
  └─ 遍历 corrections，不在 autocomplete 中的词追加到结果
    │
    ▼
  返回合并后的建议列表
```

## 9. 并发安全设计

### 9.1 锁分层策略

模块采用**三层读写锁**设计，每层锁保护不同粒度的数据：

| 锁 | 保护对象 | 锁类型 | 所在层级 |
|----|---------|--------|---------|
| `SuggestEngine.mu` | 引擎所有公开方法的入口级保护 | `sync.RWMutex` | 引擎层（最外层） |
| `Trie.mu` | 前缀树节点、词库大小 | `sync.RWMutex` | 组件层 |
| `SearchHistory.mu` | 历史记录 map | `sync.RWMutex` | 组件层 |

### 9.2 引擎级锁的保护范围

`SuggestEngine.mu` 是引擎的**入口级读写锁**，所有公开方法都必须先获取该锁，确保：

- **写操作**（获取写锁 `Lock`）：
  - `AddWord` / `AddWordWithFreq` - 添加/更新候选词
  - `RemoveWord` - 删除候选词
  - `SubmitSearch` - 提交搜索（同时修改词库和历史）
  - `ClearHistory` - 清空用户历史记录

- **读操作**（获取读锁 `RLock`）：
  - `HasWord` - 检查词是否存在
  - `WordCount` - 获取词库大小
  - `Autocomplete` / `AutocompleteLimit` - 前缀自动补全
  - `Correct` / `CorrectLimit` - 拼写纠错
  - `GetHistory` - 查询搜索历史
  - `Suggest` / `SuggestLimit` - 综合搜索建议
  - `GetHotWords` - 获取热门搜索词

**锁获取顺序**：始终先获取 `SuggestEngine.mu`，再获取内部组件的锁（`Trie.mu` / `SearchHistory.mu`），避免死锁。

### 9.3 为什么需要引擎级锁

虽然底层的 `Trie` 和 `SearchHistory` 组件各自都有锁保护，但引擎级锁提供了额外的重要保证：

1. **组合操作的原子性**：`SubmitSearch` 同时修改词库（`trie.Insert`）和历史记录（`history.Add`），引擎级写锁确保这两个操作对外是原子的，读操作不会看到「词库已更新但历史未更新」的中间状态。

2. **防止 TOCTOU 竞态**：如果没有引擎级锁，`AddWord` 可以在 `SubmitSearch` 执行期间同时修改同一 trie 节点的频率，虽然 trie 自身的锁能保证内存安全，但可能导致频率计数的语义混乱。

3. **数据一致性视图**：所有读操作在持有引擎级读锁期间，看到的是一致性的引擎状态快照，不会因并发写操作而导致同一查询内的多次数据读取出现不一致。

### 9.4 双重锁设计的权衡

本模块采用「引擎级锁 + 组件级锁」的双重锁设计，优缺点如下：

**优点**:
- 安全性更高：两层锁提供双重保护，即使某一层被绕过也不会出现数据竞争
- 组件可复用：`Trie` 和 `SearchHistory` 可独立使用，自带并发安全保证
- 语义清晰：引擎锁保证 API 级别的原子性，组件锁保证数据结构的完整性

**缺点**:
- 轻微的性能开销：两次加/解锁操作
- 需严格遵守锁顺序：必须先引擎锁、后组件锁，否则可能死锁

### 9.5 数据隔离保证

- 搜索历史返回结果时创建 `HistoryRecord` 的副本，避免外部修改内部数据
- 前缀树查询返回的 `Suggestion` 是值拷贝，不直接暴露内部节点

## 10. 使用示例

### 10.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/suggest"
)

func main() {
    // 创建搜索建议引擎
    engine := suggest.NewSuggestEngine()

    // 添加候选词
    engine.AddWord("apple")
    engine.AddWord("app")
    engine.AddWord("application")
    engine.AddWord("banana")

    // 自动补全
    results, _ := engine.Autocomplete("app")
    for _, r := range results {
        fmt.Printf("%s (freq: %d)\n", r.Word, r.Frequency)
    }

    // 拼写纠错
    corrections, _ := engine.Correct("appla")
    for _, c := range corrections {
        fmt.Printf("Did you mean: %s?\n", c.Word)
    }
}
```

### 10.2 带频率的词库初始化

```go
engine := suggest.NewSuggestEngine()

// 初始化热门词
words := map[string]int{
    "golang":      1000,
    "go tutorial": 800,
    "go web":      600,
    "grpc":        400,
    "gin":         300,
}

for word, freq := range words {
    engine.AddWordWithFreq(word, freq)
}

// 获取热门搜索词
hotWords, _ := engine.GetHotWords(5)
for i, w := range hotWords {
    fmt.Printf("%d. %s (%d)\n", i+1, w.Word, w.Frequency)
}
```

### 10.3 搜索提交与历史记录

```go
engine := suggest.NewSuggestEngine()

// 用户提交搜索
userID := "user001"
engine.SubmitSearch(userID, "golang tutorial")
engine.SubmitSearch(userID, "go concurrency")
engine.SubmitSearch(userID, "golang tutorial") // 重复搜索，频率+1

// 获取搜索历史
history, _ := engine.GetHistory(userID, 10)
for _, h := range history {
    fmt.Printf("[%s] %s\n", h.Timestamp.Format("15:04:05"), h.Word)
}

// 清空搜索历史
engine.ClearHistory(userID)
```

### 10.4 自定义配置

```go
cfg := suggest.Config{
    MaxEditDistance:  3,     // 最大容忍编辑距离 3
    DefaultMaxResult: 5,     // 默认返回 5 条结果
    HistoryMaxSize:   50,    // 每个用户最多保存 50 条历史
}

engine, err := suggest.NewSuggestEngineWithConfig(cfg)
if err != nil {
    // 处理配置错误
}
```

### 10.5 综合建议

```go
engine := suggest.NewSuggestEngine()

// 添加一些词
engine.AddWordWithFreq("apple", 100)
engine.AddWordWithFreq("app", 50)
engine.AddWordWithFreq("apply", 80)
engine.AddWordWithFreq("banana", 90)

// 获取综合建议（自动补全 + 纠错）
suggestions, _ := engine.SuggestLimit("appl", 5)
for i, s := range suggestions {
    fmt.Printf("%d. %s (freq: %d)\n", i+1, s.Word, s.Frequency)
}
```

### 10.6 动态维护词库

```go
engine := suggest.NewSuggestEngine()

// 添加词
engine.AddWord("keyword1")

// 检查词是否存在
exists, freq := engine.HasWord("keyword1")
fmt.Printf("Exists: %v, Frequency: %d\n", exists, freq)

// 删除词
engine.RemoveWord("keyword1")

// 获取词库总词数
count := engine.WordCount()
fmt.Printf("Total words: %d\n", count)
```

### 10.7 并发使用

```go
var wg sync.WaitGroup
engine := suggest.NewSuggestEngine()

// 并发添加词
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        engine.AddWord(fmt.Sprintf("word%d", id))
    }(i)
}

// 并发查询
for i := 0; i < 50; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        results, _ := engine.Autocomplete("wo")
        // 处理结果...
    }()
}

wg.Wait()
```

## 11. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyWord` | 词为空 | 插入/删除空字符串 |
| `ErrEmptyPrefix` | 前缀为空 | 前缀查询时 prefix 为空 |
| `ErrEmptyQuery` | 查询为空 | 纠错/建议时 query 为空 |
| `ErrEmptyUserID` | 用户 ID 为空 | 历史操作时 userID 为空 |
| `ErrInvalidMaxResult` | 最大结果数无效 | maxResults <= 0 |
| `ErrInvalidMaxDist` | 最大编辑距离无效 | 配置中 MaxEditDistance < 0 |
| `ErrInvalidHistoryN` | 历史记录数无效 | n <= 0 或 HistoryMaxSize <= 0 |
| `ErrWordNotFound` | 词不存在 | 删除不存在的词 |

## 12. 性能特征

### 12.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| Insert | O(L) | L 为词的字符长度 |
| Delete | O(L) | 需要遍历路径并清理空节点 |
| Search | O(L) | 精确查找 |
| StartsWith | O(P + K) | P 为前缀长度，K 为匹配词的总字符数 |
| EditDistance | O(m×n) | m、n 为两字符串长度 |
| Correct | O(N × m × n) | N 为词库大小，对每个词计算编辑距离 |
| History Add | O(M) | M 为该用户历史记录数（最坏情况需遍历查找重复） |
| History GetRecent | O(K) | K 为返回的记录数 |

### 12.2 空间复杂度

- 前缀树: O(T × L)，T 为词数，L 为平均词长（共享前缀节省空间）
- 历史记录: O(U × M)，U 为用户数，M 为每用户平均历史数
- 编辑距离计算: O(n)，n 为较短字符串长度（空间优化版）

## 13. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **大小写敏感**: 默认区分大小写，如需大小写不敏感请在调用前统一转为小写
3. **Unicode 支持**: 支持 UTF-8 编码的多语言字符（中文、日文等）
4. **编辑距离性能**: 拼写纠错需遍历整个词库计算编辑距离，词库较大时可能较慢
5. **历史记录去重**: 重复搜索会将记录移到顶部并更新时间，不会产生多条重复记录
6. **词库与历史分离**: 搜索历史与候选词库独立管理，清空历史不影响词库
7. **频率累加**: `Insert` 会累加频率，如需设置固定频率请使用 `InsertWithFreq`

## 14. 测试策略与验证方法

### 14.1 前缀树测试覆盖

| 测试用例 | 验证目标 |
|---------|---------|
| `TestTrie_Insert` | 基本插入和大小统计 |
| `TestTrie_Insert_EmptyWord` | 空词边界处理 |
| `TestTrie_Insert_DuplicateIncrementsFreq` | 重复插入频率累加 |
| `TestTrie_Delete` | 正常删除和节点清理 |
| `TestTrie_Delete_NotFound` | 删除不存在的词 |
| `TestTrie_Delete_LeafCleanup` | 删除后空节点回收 |
| `TestTrie_StartsWith` | 前缀查询正确性 |
| `TestTrie_StartsWith_SortedByFreq` | 频率降序排序 |
| `TestTrie_StartsWith_SameFreqLexOrder` | 同频字典序排序 |
| `TestTrie_Unicode` | 多语言字符支持 |

### 14.2 编辑距离测试覆盖

| 测试用例 | 验证目标 |
|---------|---------|
| `TestEditDistance` | 经典测试用例正确性 |
| `TestEditDistance_SingleCharOps` | 单字符插入/删除/替换 |

### 14.3 搜索历史测试覆盖

| 测试用例 | 验证目标 |
|---------|---------|
| `TestSearchHistory_Add` | 基本添加功能 |
| `TestSearchHistory_Add_DuplicateMovesToFront` | 重复词置顶 |
| `TestSearchHistory_Add_MaxSizeLimit` | 容量限制与淘汰 |
| `TestSearchHistory_GetRecent` | 最近记录查询 |
| `TestSearchHistory_Clear` | 清空历史 |
| `TestSearchHistory_MultipleUsers` | 多用户隔离 |
| `TestSearchHistory_OrderDescending` | 时间倒序排列 |

### 14.4 并发测试覆盖

| 测试用例 | 验证目标 |
|---------|---------|
| `TestConcurrent_TrieInsert` | Trie 并发插入数据完整性 |
| `TestConcurrent_TrieInsertSameWord` | Trie 同词并发插入的频率正确性 |
| `TestConcurrent_SearchHistory` | 历史记录并发安全 |
| `TestConcurrent_SuggestEngine` | 引擎级基础并发读写安全 |
| `TestConcurrent_SuggestEngine_AddWordAndSubmitSearch` | AddWord 与 SubmitSearch 并发时的频率正确性 |
| `TestConcurrent_SuggestEngine_AutocompleteAndSubmitSearch` | Autocomplete 与 SubmitSearch 并发时无数据竞争 |
| `TestConcurrent_SuggestEngine_CorrectAndSubmitSearch` | Correct 与 SubmitSearch 并发时无数据竞争 |
| `TestConcurrent_SuggestEngine_GetHotWordsAndSubmit` | GetHotWords 与 SubmitSearch 并发时一致性 |
| `TestConcurrent_SuggestEngine_HistoryAndSubmit` | GetHistory 与 SubmitSearch 并发时历史记录安全 |

### 14.5 引擎级并发测试设计说明

针对「引擎级锁形同虚设」问题修复后，补充了多维度的引擎级并发测试：

**1. AddWord 与 SubmitSearch 并发频率验证**
- 场景：20 个 goroutine 并发执行 AddWord，30 个 goroutine 并发执行 SubmitSearch（同一词）
- 验证：SubmitSearch 对共享词的频率累加必须精确等于并发提交次数
- 原理：引擎级写锁保证 SubmitSearch 的读-改-写原子性，不会因 AddWord 并发干扰导致频率丢失

**2. 读操作与写操作并发验证**
- 场景：50 个 goroutine 并发执行 SubmitSearch（写），50 个 goroutine 并发执行 Autocomplete/Correct/GetHotWords（读）
- 验证：读操作不会崩溃、不会返回异常、不会观察到中间状态
- 原理：引擎级读写锁保证读操作看到的是一致性状态快照

**3. 历史与词库操作并发验证**
- 场景：SubmitSearch 同时修改词库和历史，与 GetHistory 并发执行
- 验证：历史记录数量正确，不会因并发导致数据竞争
- 原理：引擎级锁保证 SubmitSearch 的「词库+历史」组合操作原子性

### 14.6 引擎集成测试覆盖

| 测试用例 | 验证目标 |
|---------|---------|
| `TestSuggestEngine_Autocomplete` | 自动补全功能 |
| `TestSuggestEngine_Correct` | 拼写纠错功能 |
| `TestSuggestEngine_SubmitSearch` | 搜索提交（词频+历史） |
| `TestSuggestEngine_Suggest` | 综合建议功能 |
| `TestSuggestEngine_GetHotWords` | 热门词排序 |
| `TestSuggestEngine_HistorySeparateFromWords` | 历史与词库隔离 |
| `TestSuggestEngine_AddWord_FreqStartsAtZero` | AddWord 初始化频率为 0 |
| `TestSuggestEngine_SubmitSearch_AfterAddWord_IncrementsFreq` | SubmitSearch 正确递增频率 |
| `TestSuggestEngine_AddWordWithFreq_CustomFreq` | 自定义初始频率 |
| `TestSuggestEngine_HotWords_InitWordsNotSearch_SortedByFreq` | 初始化词不影响真实热门排序 |
