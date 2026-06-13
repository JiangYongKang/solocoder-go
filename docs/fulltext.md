# Fulltext 全文检索引擎模块

## 1. 模块概述

Fulltext 是一个高性能的内存全文检索引擎模块，专为需要快速文本搜索、相关性排序和短语精确匹配的场景设计。模块提供了完整的倒排索引构建、多语言分词器注册、TF-IDF 相关性计算、短语精确查询等功能，并通过读写锁机制实现安全的并发访问。

**包路径**: `internal/fulltext`

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 倒排索引构建 | 将文档分词后建立词项到文档列表的映射，记录词项的出现位置和频率 |
| 分词器注册 | 支持注册多种语言的自定义分词策略，内置基于空格和标点的默认分词器 |
| TF-IDF 排序 | 按 TF-IDF 算法计算搜索结果相关度评分，结果按评分降序返回 |
| 短语查询 | 支持多词项按顺序连续出现的精确匹配查询，基于词项位置信息判定 |
| 文档管理 | 支持文档添加、删除、查询等基本操作 |
| 并发安全 | 内置读写锁保护，支持多 goroutine 并发读写 |

## 3. 核心结构体与职责

### 3.1 Engine

全文检索引擎主结构体，对外提供所有操作接口。

```go
type Engine struct {
    docs          map[string]*Document
    invertedIndex *InvertedIndex
    tokenizers    map[string]Tokenizer
    defaultLang   string
    mu            sync.RWMutex
}
```

**职责**:
- 管理文档集合（`docs`），存储已索引的完整文档
- 维护倒排索引（`invertedIndex`），提供快速词项检索
- 管理分词器注册表（`tokenizers`），支持多语言分词
- 协调并发访问，通过 `sync.RWMutex` 保证线程安全
- 提供搜索、短语查询、文档管理等对外 API

### 3.2 Document

文档结构体，存储原始文档信息。

```go
type Document struct {
    ID      string
    Content string
    Length  int
}
```

**职责**:
- `ID`: 文档唯一标识符，用于索引和检索
- `Content`: 文档原始文本内容
- `Length`: 文档分词后的词项总数，用于 TF 计算

### 3.3 InvertedIndex

倒排索引结构体，词项到文档列表的映射。

```go
type InvertedIndex struct {
    index map[string]*PostingList
    mu    sync.RWMutex
}
```

**职责**:
- 存储 `term -> PostingList` 的倒排映射
- `AddTerm(term, docID, position)`: 添加词项出现记录
- `GetPostingList(term)`: 获取词项的所有文档出现信息
- `HasTerm(term)`: 快速判断词项是否存在
- `GetTermCount()`: 获取索引中的词项总数
- 通过独立的 `sync.RWMutex` 保护索引并发访问

### 3.4 PostingList

倒排列表，存储某个词项在所有文档中的出现信息。

```go
type PostingList struct {
    Postings []*TermPosting
}
```

### 3.5 TermPosting

词项在单个文档中的出现记录。

```go
type TermPosting struct {
    DocID     string
    Frequency int
    Positions []int
}
```

**职责**:
- `DocID`: 文档标识符
- `Frequency`: 词项在该文档中出现的总次数
- `Positions`: 词项在文档中的所有位置索引（从 0 开始），用于短语查询

### 3.6 SearchResult

搜索结果结构体。

```go
type SearchResult struct {
    DocID   string
    Score   float64
    Content string
}
```

**职责**:
- `DocID`: 命中文档的 ID
- `Score`: TF-IDF 相关度评分，值越高表示越相关
- `Content`: 文档原始内容，方便调用方直接使用

### 3.7 Tokenizer 接口

分词器接口，定义文本分词行为。

```go
type Tokenizer interface {
    Tokenize(text string) []string
}
```

**职责**:
- `Tokenize(text)`: 将输入文本分割为词项（token）列表
- 调用方可实现此接口以支持自定义分词策略（如中文分词、专业领域分词等）

### 3.8 DefaultTokenizer

默认分词器，基于字母数字边界进行分词。

```go
type DefaultTokenizer struct{}
```

**分词规则**:
- 以 Unicode 字母（`unicode.IsLetter`）和数字（`unicode.IsDigit`）作为有效词项字符
- 其他字符（空格、标点、符号等）作为词项分隔符
- 所有词项自动转换为小写，实现大小写不敏感搜索

## 4. 从文档索引到查询排序的完整流程

### 4.1 文档索引流程

```
AddDocument(docID, content)
       │
       ▼
  参数校验
  ├─ docID 非空? ──否──► ErrEmptyDocID
  ├─ content TrimSpace 非空? ──否──► ErrEmptyDocument
  ├─ 分词后 tokens 非空? ──否──► ErrEmptyDocument
  └─ docID 未重复? ──否──► ErrDuplicateDocID
       │
       ▼
  选择分词器
  └─ 指定语言 → 使用对应分词器
     未指定   → 使用默认分词器 (defaultLang)
       │
       ▼
  调用 Tokenize(content) 获取 tokens
       │
       ▼
  构建倒排索引
  └─ 遍历 tokens，记录位置 pos:
     └─ invertedIndex.AddTerm(token, docID, pos)
        ├─ 词项首次出现 → 创建新 PostingList
        ├─ 文档首次出现该词项 → 添加新 TermPosting(Frequency=1, Positions=[pos])
        └─ 文档已含该词项 → Frequency++，追加 pos 到 Positions
       │
       ▼
  存储文档
  └─ docs[docID] = &Document{ID, Content, Length=len(tokens)}
       │
       ▼
     返回 nil (成功)
```

### 4.2 普通搜索流程（TF-IDF 排序）

```
Search(query)
       │
       ▼
  参数校验
  └─ query 非空? ──否──► ErrEmptyQuery
       │
       ▼
  分词: queryTokens = Tokenize(query)
  └─ len(queryTokens) == 0 → 返回空结果 []
       │
       ▼
  空引擎检查: totalDocs == 0 → 返回空结果 []
       │
       ▼
  计算 TF-IDF 分数
  └─ 遍历每个查询词项 term:
     │
     ├─ 获取倒排列表: postingList = invertedIndex[term]
     │  └─ 词项不存在 → 跳过该 term
     │
     ├─ 计算 IDF (逆文档频率):
     │  idf = ln(1 + totalDocs / (docsWithTerm + 1))
     │  说明: docsWithTerm 为包含该词项的文档数
     │        +1 平滑防止除零，1+... 确保 IDF 始终为正
     │
     └─ 遍历 postingList 中每个文档 posting:
        │
        ├─ 获取文档 doc = docs[posting.DocID]
        │  └─ 文档已删除 → 跳过
        │
        ├─ 计算 TF (词频):
        │  tf = posting.Frequency / doc.Length
        │
        └─ 累加分数:
           docScores[posting.DocID] += tf * idf
       │
       ▼
  构建结果列表
  └─ 遍历 docScores，封装为 []*SearchResult
       │
       ▼
  按 Score 降序排序
       │
       ▼
  返回排序后的搜索结果
```

### 4.3 短语查询流程

```
SearchPhrase(phrase)
       │
       ▼
  参数校验
  ├─ phrase 非空? ──否──► ErrEmptyPhrase
  └─ 分词后 terms 数 >= 2? ──否──► ErrPhraseTooShort
       │
       ▼
  获取每个词项的倒排列表
  └─ 任一词项无倒排列表 → 返回空结果 []
       │
       ▼
  候选文档筛选（逐步求交集）
  └─ candidateDocs = postingLists[0] 的所有文档
     遍历 postingLists[1..n]:
        candidateDocs = candidateDocs ∩ 当前 postingList 的文档
        └─ candidateDocs 为空 → 提前返回空结果 []
       │
       ▼
  位置连续性验证
  └─ 遍历每个候选文档 docID:
     │
     ├─ 获取该文档中每个词项的位置列表
     │  postings = [term1_positions, term2_positions, ..., termN_positions]
     │
     └─ checkPhraseMatch(postings):
        └─ 遍历 term1 的每个起始位置 startPos:
           └─ 验证 term2 是否出现在 startPos+1
               验证 term3 是否出现在 startPos+2
               ...
               验证 termN 是否出现在 startPos+(N-1)
           └─ 任一 startPos 满足所有位置连续 → 匹配成功
       │
       ▼
  计算匹配文档的 TF-IDF 分数（同普通搜索）
       │
       ▼
  按 Score 降序排序
       │
       ▼
  返回短语匹配结果
```

## 5. TF-IDF 算法详解

### 5.1 TF (Term Frequency，词频)

衡量词项在文档中的重要程度：

```
TF(t, d) = 词项 t 在文档 d 中出现的次数 / 文档 d 的总词项数
```

**特性**:
- 词项在文档中出现越频繁，TF 值越高
- 归一化处理，避免长文档具有不公平的优势
- 取值范围: [0, 1]

### 5.2 IDF (Inverse Document Frequency，逆文档频率)

衡量词项在整个文档集合中的稀有程度：

```
IDF(t) = ln(1 + N / (df(t) + 1))
```

其中：
- `N`: 文档集合中的总文档数
- `df(t)`: 包含词项 t 的文档数量
- `+1`: 拉普拉斯平滑，防止除零错误
- `1 + ...`: 确保 IDF 始终为正数

**特性**:
- 词项出现在越少的文档中，IDF 值越高（越稀有越重要）
- 出现在所有文档中的常用词（如 "the"）IDF 值趋近于 0
- 本实现采用 `ln(1 + N/(df+1))` 变体，保证分数始终为正

### 5.3 TF-IDF 综合评分

```
TF-IDF(t, d) = TF(t, d) × IDF(t)
```

对于多词项查询，文档总分为各查询词项 TF-IDF 值之和：

```
Score(d, Q) = Σ TF-IDF(term_i, d)  for term_i in query Q
```

## 6. 短语查询匹配算法

### 6.1 位置连续性原理

短语查询要求多个词项按指定顺序连续出现在文档中。例如查询短语 "quick brown fox" 要求：

```
位置:   0      1       2      3       4
词项: ["the", "quick", "brown", "fox", "jumps"]
          │        │       │
          └────────┴───────┘ 连续位置: 1, 2, 3 → 匹配成功
```

### 6.2 匹配算法

对于短语 `[t1, t2, ..., tn]`：

1. 获取 t1 的所有位置列表 `P1 = [p1_1, p1_2, ...]`
2. 对每个起始位置 `start ∈ P1`：
   - 检查 t2 是否出现在 `start + 1`
   - 检查 t3 是否出现在 `start + 2`
   - ...
   - 检查 tn 是否出现在 `start + (n-1)`
3. 任一 `start` 满足所有条件 → 文档匹配

**时间复杂度**: O(k × m × n)，其中 k 为 t1 出现次数，m 为平均位置列表长度，n 为短语词项数。实际应用中由于倒排索引提前筛选了候选文档，性能通常可接受。

## 7. 分词器系统

### 7.1 默认分词器

内置 `DefaultTokenizer` 采用基于字符类别的分词策略：

- 有效字符: Unicode 字母 (`L*`) 和数字 (`N*`)
- 分隔字符: 其他所有字符（空格、标点、符号等）
- 归一化: 所有词项转为小写

示例:
```
输入:  "Hello, World! Go123 is FUN."
输出:  ["hello", "world", "go123", "is", "fun"]
```

### 7.2 自定义分词器注册

实现 `Tokenizer` 接口即可注册自定义分词策略：

```go
type Tokenizer interface {
    Tokenize(text string) []string
}
```

注册接口：
- `RegisterTokenizer(language, tokenizer)`: 按语言注册分词器
- `SetDefaultLanguage(language)`: 设置默认使用的分词器语言
- 语言名称大小写不敏感，自动统一为小写

## 8. 并发安全设计

### 8.1 锁分层策略

模块采用两层读写锁设计：

| 锁 | 保护对象 | 锁类型 |
|----|---------|--------|
| `Engine.mu` | `docs` map、`tokenizers` map、`defaultLang` | `sync.RWMutex` |
| `InvertedIndex.mu` | `index` map | `sync.RWMutex` |

### 8.2 读写锁使用规则

- **读操作** (GetDocument, Search, SearchPhrase, DocumentCount 等):
  - 获取 `Engine.mu.RLock()` 读取文档和配置
  - 获取 `InvertedIndex.mu.RLock()` 读取倒排索引
  - 允许多个读操作并发执行

- **写操作** (AddDocument, DeleteDocument, RegisterTokenizer 等):
  - 获取对应写锁确保独占访问
  - 写操作与其他读/写操作互斥

### 8.3 死锁避免

- 固定锁获取顺序：Engine 锁 → InvertedIndex 锁（如需要）
- 避免嵌套获取同一把锁
- 写操作完成后立即释放锁，减少持有时间

## 9. 使用示例

### 9.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/fulltext"
)

func main() {
    // 创建搜索引擎实例
    engine := fulltext.NewEngine()

    // 添加文档
    engine.AddDocument("doc1", "Go is a statically typed programming language")
    engine.AddDocument("doc2", "Python is a dynamically typed programming language")
    engine.AddDocument("doc3", "The quick brown fox jumps over the lazy dog")

    // 普通搜索
    results, _ := engine.Search("programming language")
    for _, r := range results {
        fmt.Printf("Doc: %s, Score: %.4f\n", r.DocID, r.Score)
    }

    // 短语查询
    phraseResults, _ := engine.SearchPhrase("programming language")
    for _, r := range phraseResults {
        fmt.Printf("Phrase match: %s\n", r.DocID)
    }
}
```

### 9.2 注册自定义分词器

```go
// 实现中文分词器（示例，实际可集成结巴分词等）
type ChineseTokenizer struct{}

func (ct *ChineseTokenizer) Tokenize(text string) []string {
    // 自定义分词逻辑...
    return tokens
}

// 注册并使用
engine := fulltext.NewEngine()
engine.RegisterTokenizer("zh", &ChineseTokenizer{})

// 按语言添加文档
engine.AddDocumentWithLanguage("zh_doc1", "机器学习是人工智能的分支", "zh")

// 按语言搜索
results, _ := engine.SearchWithLanguage("机器学习", "zh")

// 设置默认语言为中文
engine.SetDefaultLanguage("zh")
```

### 9.3 文档管理

```go
engine := fulltext.NewEngine()

// 添加文档
engine.AddDocument("doc1", "Hello World")

// 查询文档
doc, exists := engine.GetDocument("doc1")
if exists {
    fmt.Println("Content:", doc.Content)
    fmt.Println("Token count:", doc.Length)
}

// 获取文档总数
fmt.Println("Total docs:", engine.DocumentCount())

// 删除文档
engine.DeleteDocument("doc1")
```

### 9.4 并发使用

```go
var wg sync.WaitGroup
engine := fulltext.NewEngine()

// 并发写入文档
for i := 0; i < 100; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()
        engine.AddDocument(
            fmt.Sprintf("doc%d", id),
            fmt.Sprintf("content for document %d with keyword", id),
        )
    }(i)
}

// 并发搜索
for i := 0; i < 50; i++ {
    wg.Add(1)
    go func() {
        defer wg.Done()
        results, _ := engine.Search("keyword")
        // 处理结果...
    }()
}

wg.Wait()
```

## 10. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrEmptyQuery` | 查询为空 | `Search("")` 或空白查询 |
| `ErrEmptyDocument` | 文档内容为空 | `AddDocument` 内容为空或分词后无词项 |
| `ErrEmptyDocID` | 文档 ID 为空 | `AddDocument` 或 `DeleteDocument` 时 ID 为空 |
| `ErrDuplicateDocID` | 文档 ID 重复 | 添加已存在 ID 的文档 |
| `ErrDocNotFound` | 文档不存在 | 删除不存在的文档 |
| `ErrNilTokenizer` | 分词器为 nil | `RegisterTokenizer` 传入 nil |
| `ErrTokenizerExists` | 分词器已注册 | 重复注册同一语言的分词器 |
| `ErrEmptyPhrase` | 短语查询为空 | `SearchPhrase("")` 或空白短语 |
| `ErrPhraseTooShort` | 短语太短 | 短语分词后词项数 < 2 |

## 11. 性能特征

### 11.1 时间复杂度

| 操作 | 时间复杂度 | 说明 |
|------|-----------|------|
| AddDocument | O(L) | L 为文档词项数，每个词项倒排索引插入 O(1) |
| Search | O(Q × P) | Q 为查询词项数，P 为词项平均倒排列表长度 |
| SearchPhrase | O(Q × P × K) | K 为词项平均出现次数（位置匹配） |
| DeleteDocument | O(1) | 仅从 docs map 删除，倒排索引保留（可接受的权衡） |
| GetDocument | O(1) | map 查找 |

### 11.2 空间复杂度

- 文档存储: O(N × L)，N 为文档数，L 为平均文档长度
- 倒排索引: O(T × P)，T 为词项数，P 为平均倒排列表长度
- 位置信息: 每个词项出现位置存储一次，为短语查询提供支持

## 12. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **删除策略**: DeleteDocument 仅删除 docs 记录，倒排索引保留旧数据（搜索时会跳过已删除文档）
3. **大小写不敏感**: 默认分词器将所有词项转为小写，搜索自动大小写不敏感
4. **短语查询限制**: 短语查询要求分词后至少 2 个词项，单词项请使用普通 Search
5. **并发度**: 读写锁设计允许多读者并发，写操作串行，适合读多写少场景
6. **TF-IDF 变体**: 本实现使用 `ln(1 + N/(df+1))` 确保分数非负，与经典公式略有差异
