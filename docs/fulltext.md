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
    Terms   map[string]struct{}
}
```

**职责**:
- `ID`: 文档唯一标识符，用于索引和检索
- `Content`: 文档原始文本内容
- `Length`: 文档分词后的词项总数，用于 TF 计算
- `Terms`: 文档包含的去重词项集合，用于删除文档时快速清理倒排索引

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
- `RemoveDocument(docID, terms)`: 从倒排索引中移除指定文档的所有词项记录，当词项不再被任何文档引用时清理该词项条目
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
  └─ 分词后 tokens 非空? ──否──► ErrEmptyDocument
       │
       ▼
  选择分词器
  └─ 指定语言 → 使用对应分词器
     未指定   → 使用默认分词器 (defaultLang)
       │
       ▼
  调用 Tokenize(content) 获取 tokens
  构建去重词项集合 Terms
       │
       ▼
  获取 Engine.mu 写锁
  ├─ docID 未重复? ──否──► 释放锁，返回 ErrDuplicateDocID
  ├─ 构建倒排索引:
  │  └─ 遍历 tokens，记录位置 pos:
  │     └─ invertedIndex.AddTerm(token, docID, pos)
  │        ├─ 词项首次出现 → 创建新 PostingList
  │        ├─ 文档首次出现该词项 → 添加新 TermPosting
  │        └─ 文档已含该词项 → Frequency++，追加位置
  └─ docs[docID] = &Document{ID, Content, Length, Terms}
  释放 Engine.mu 写锁
  （说明：倒排索引写入、重复检查、文档写入在同一持锁范围内原子执行，
   防止 TOCTOU 竞态；与读操作的 Engine.mu.RLock 形成互斥）
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

### 4.4 文档删除与倒排索引清理流程

```
DeleteDocument(docID)
       │
       ▼
  参数校验
  └─ docID 非空? ──否──► ErrEmptyDocID
       │
       ▼
  获取 Engine.mu 写锁
  ├─ docID 存在? ──否──► 释放锁，返回 ErrDocNotFound
  ├─ 读取 doc.Terms（去重词项集合）
  ├─ invertedIndex.RemoveDocument(docID, doc.Terms)
  │  └─ 遍历文档中的每个词项 term:
  │     ├─ 从该 term 的 PostingList 中移除 docID 对应的 TermPosting
  │     └─ 若移除后 PostingList 为空，则从倒排索引中删除该 term 条目
  ├─ 从 docs map 中删除 docID
  └─ 释放 Engine.mu 写锁
  （说明：倒排索引清理和 docs 删除在同一持锁范围内原子执行，
   消除了 IDF 计算窗口期，保证 totalDocs 与 docsWithTerm 一致性）
       │
       ▼
     返回 nil (成功)
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

### 5.4 多词项查询排序规则

多词项查询的排序遵循以下规则：

1. **累加评分模型**: 文档最终得分为所有匹配查询词项的 TF-IDF 分值累加和
2. **匹配任一即参与排序**: 文档只需匹配查询中的任意一个词项即可出现在结果中，匹配的词项越多通常得分越高（因为有更多分值累加）
3. **降序排列**: 所有结果按 Score 从高到低排序，高分文档排在前面
4. **稀有词项权重更高**: 稀有词项（在少数文档中出现）的 IDF 值更高，因此匹配稀有词项的文档排名会更靠前
5. **词频正相关**: 在同一文档中，词项出现频率越高，该词项对总分的贡献越大
6. **文档长度归一化**: TF 计算使用归一化词频（frequency / docLength），避免长文档因词项绝对数量多而获得不公平优势
7. **同分排序**: 当两个文档 Score 完全相同时，排序顺序不做保证，取决于 Go `sort.Slice` 的稳定性

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

**数据隔离保证**:
- 短语匹配不直接使用倒排索引中的 `*TermPosting` 指针，而是为每个词项创建独立的位置切片副本（`[][]int`）
- 即使短语包含重复词项（如 "go go go"），每个位置列表也是独立拷贝，避免因隐式共享底层数组导致的意外副作用
- `checkPhraseMatch` 函数接收纯位置数据（`[][]int`），与倒排索引内部结构完全解耦，便于扩展匹配策略

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

### 8.3 TOCTOU 竞态防护

本模块针对检查时使用（Time-of-Check to Time-of-Use）竞态漏洞采取了专门防护：

**文档添加的原子性保证**:
- `AddDocumentWithLanguage` 中，docID 重复检查和 docs map 写入操作在**同一个持锁范围内**原子执行
- 避免了原实现中「检查重复 → 释放锁 → 重新获取锁 → 写入」的时间窗口
- 该窗口期曾允许并发 goroutine 插入相同 docID 的文档，导致重复文档检测失效

**防护原理**:
```
错误模式（TOCTOU 漏洞）:
  goroutine A: Lock → 检查 docID 不存在 → Unlock → [窗口] → Lock → 写入
  goroutine B:                          Lock → 检查 docID 不存在 → 写入 → Unlock
  → 两个 goroutine 都认为自己成功，造成数据竞争

正确模式（本实现）:
  goroutine A: Lock → 检查 docID 不存在 → 写入 → Unlock
  goroutine B:           [等待锁] → Lock → 检查 docID 已存在 → 返回错误 → Unlock
  → 保证同一 docID 只有一个 goroutine 能成功写入
```

**文档删除的原子性保证**:
- `DeleteDocument` 中，倒排索引清理（`invertedIndex.RemoveDocument`）和 docs map 删除在**同一个 Engine.mu 写锁范围内**原子执行
- 先清理倒排索引（减少各词项的 docsWithTerm 计数），再删除 docs map（减少 totalDocs），顺序固定
- 消除了「先删 docs 再清理索引」的时间窗口——该窗口期会导致搜索用已减少的 totalDocs 计算 IDF，但词项的 docsWithTerm 仍包含被删文档，造成评分偏低

**IDF 一致性保证**:
```
错误模式（窗口期 IDF 失真）:
  删除前状态: totalDocs=3, docsWithTerm("hello")=2
  goroutine A (Delete): Lock → 删 docs[docID] → Unlock → [窗口] → 清理索引
  goroutine B (Search):        Lock → 读 totalDocs=2 → Unlock
                              → 读 invertedIndex: docsWithTerm("hello")=2（未清理）
                              → IDF = ln(1 + 2/(2+1))  ← 分母偏大，评分偏低

正确模式（本实现）:
  goroutine A (Delete): Lock → 清理索引 → 删 docs → Unlock
  goroutine B (Search):        [等待锁] 或 读完整一致状态
                              → totalDocs 与 docsWithTerm 始终保持一致
```

### 8.4 死锁避免

- 固定锁获取顺序：Engine 锁 → InvertedIndex 锁（如需要）
- 避免嵌套获取同一把锁
- 写操作完成后立即释放锁，减少持有时间
- 短语匹配使用独立的位置切片副本，避免对倒排索引内部数据结构的共享修改

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
| DeleteDocument | O(U × P) | U 为文档去重词项数，P 为词项平均倒排列表长度；需遍历每个词项的 PostingList 并移除该文档条目，当词项无文档引用时清理词项 |
| GetDocument | O(1) | map 查找 |

### 11.2 空间复杂度

- 文档存储: O(N × L)，N 为文档数，L 为平均文档长度
- 倒排索引: O(T × P)，T 为词项数，P 为平均倒排列表长度
- 位置信息: 每个词项出现位置存储一次，为短语查询提供支持

## 12. 注意事项与限制

1. **纯内存存储**: 数据仅存在于内存中，进程退出即丢失
2. **删除策略**: DeleteDocument 同步清理倒排索引中的对应条目
   - 删除文档时遍历文档的去重词项集合，从每个词项的 PostingList 中移除该文档
   - 当词项不再被任何文档引用时，自动从倒排索引中删除该词项条目以回收空间
   - IDF 计算基于实时文档数，删除文档后会立即反映到后续的 TF-IDF 评分中
3. **大小写不敏感**: 默认分词器将所有词项转为小写，搜索自动大小写不敏感
4. **短语查询限制**: 短语查询要求分词后至少 2 个词项，单词项请使用普通 Search
5. **并发度**: 读写锁设计允许多读者并发，写操作串行，适合读多写少场景
6. **TF-IDF 变体**: 本实现使用 `ln(1 + N/(df+1))` 确保分数非负，与经典公式略有差异
7. **短语匹配数据隔离**: 短语查询过程中使用位置切片的独立副本进行匹配判定，避免与倒排索引内部数据结构发生隐式共享，确保扩展匹配策略时的安全性
8. **删除原子性**: DeleteDocument 将倒排索引清理和 docs 删除放在同一写锁范围内原子执行，消除 IDF 计算窗口期，保证 totalDocs 与 docsWithTerm 一致性

## 13. 测试策略与验证方法

### 13.1 并发去重测试策略

文档添加的并发去重正确性通过 `TestConcurrent_AddDocument_SameDocID` 测试验证：

**测试设计**:
- 启动 20 个 goroutine 并发调用 `AddDocument`，全部使用**同一个 docID**
- 每个 goroutine 传入不同的 content 以区分写入路径
- 使用独立 mutex 保护 successCount 和 duplicateCount 的原子累加

**验证断言**:
1. **恰好 1 次成功**: `successCount == 1` — 验证 TOCTOU 防护有效，只有第一个获取写锁的 goroutine 能成功写入
2. **N-1 次重复错误**: `duplicateCount == numGoroutines - 1` — 验证其余 goroutine 正确收到 `ErrDuplicateDocID` 错误
3. **最终文档数为 1**: `DocumentCount() == 1` — 验证未因竞态条件产生重复文档

**与 `TestConcurrent_AddDocument` 的区别**:
- 后者每个 goroutine 使用唯一 docID，仅验证数据结构的并发安全（无竞态崩溃）
- 前者刻意制造 docID 冲突，专门验证 TOCTOU 漏洞的防护有效性

### 13.2 倒排索引清理验证方法

文档删除时的倒排索引清理通过多层测试直接验证内部状态，而非仅依赖搜索结果间接推断：

#### 层次一：InvertedIndex 单元级验证

| 测试用例 | 验证目标 | 关键断言 |
|---------|---------|---------|
| `TestInvertedIndex_RemoveDocument_UniqueTerms` | 独有词项被完全从 index map 移除 | `GetTermCount() == 0`、`HasTerm(term) == false` |
| `TestInvertedIndex_RemoveDocument_SharedTerms` | 共享词项的 PostingList 长度正确减少 | `len(Postings) == N-1`、剩余文档 ID 正确、独有词项被清理 |

#### 层次二：Engine 集成级验证

`TestDeleteDocument_InvertedIndexCleanup` 在 Engine 层面验证端到端清理：

1. **独有词项验证**: 删除文档后，该文档独有的词项（如 `xray`、`alpha`）应从倒排索引中完全消失
   - 验证: `invertedIndex.HasTerm("xray") == false`
2. **共享词项验证**: 多文档共有的词项（如 `sharedword`）的 PostingList 应仅移除被删文档
   - 验证: `len(Postings) == 1` 且剩余 DocID 为未删除文档
3. **词项总数验证**: 索引中的词项总数应精确减少
   - 验证: 删除前 4 个词项 → 删除后剩 2 个（sharedword + gamma）

**为何需要直接验证而非仅依赖搜索结果**:
- 仅通过 `Search` 间接验证可能遗漏索引泄露：`Search` 在计算 TF 时会再次检查 docs map 中是否存在该文档，即使倒排索引中残留了被删文档的 Posting，也会因文档已从 docs 中删除而被跳过，从而掩盖索引泄露问题
- 直接检查倒排索引内部状态（`HasTerm`、`GetPostingList`、`GetTermCount`）可以发现 docs map 删除但索引未清理的不一致状态

### 13.3 DeleteDocument 原子性与读写一致性测试保证

删除操作的完整性通过**写侧原子性**和**读写互斥**双重机制保障，测试从多个维度交叉验证：

---

#### 代码层面保障

**写操作原子性**（防止写操作中间状态被看到）:
- `invertedIndex.RemoveDocument()` 和 `delete(e.docs, docID)` 在同一个 `Engine.mu.Lock()` 保护范围内执行
- 固定执行顺序：先清理索引 → 再删 docs

**读操作一致性**（防止搜索看到不一致快照）:
- `SearchWithLanguage` / `SearchPhraseWithLanguage` 在整个读取+计算周期内持有 `Engine.mu.RLock()`
- 与写操作的 `Engine.mu.Lock()` 形成全局读写互斥
- 锁顺序全局一致：所有操作均按「Engine.mu → InvertedIndex.mu」顺序获取，避免死锁

---

#### 测试层面验证

| 测试用例 | 验证维度 | 验证目标 |
|---------|---------|---------|
| `TestDeleteDocument_SearchAfterDelete` | 外部行为（搜索结果） | 删除后搜索正确排除被删文档，评分正确反映新文档集 |
| `TestDeleteDocument_InvertedIndexCleanup` | 内部状态（索引完整性） | 独有词项从 index map 移除、共享词项 PostingList 长度正确减少、词项总数精确 |
| `TestInvertedIndex_RemoveDocument_UniqueTerms` | 单元级（独立索引） | RemoveDocument 对独有词项的清理逻辑正确 |
| `TestInvertedIndex_RemoveDocument_SharedTerms` | 单元级（独立索引） | RemoveDocument 对共享/独有混合词项的清理逻辑正确 |
| `TestConcurrent_AddAndSearch` | 集成级（并发） | 并发添加与搜索同时进行时无崩溃、无数据损坏 |
| `TestConcurrent_Search` | 集成级（并发） | 多 goroutine 并发搜索结果数量正确、无竞态崩溃 |
| `TestConcurrent_PhraseSearch` | 集成级（并发） | 多 goroutine 并发短语搜索结果正确 |

---

#### 验证原理

倒排索引清理必须通过直接检查内部状态验证，不能仅依赖搜索结果间接推断：
- 搜索计算 TF 时会二次检查 `docs` map 是否存在该文档
- 如果 RemoveDocument 有遗漏（索引残留被删文档），搜索仍会因文档已从 docs 删除而跳过，**掩盖索引泄露问题**
- 直接检查 `HasTerm()`、`GetPostingList()`、`GetTermCount()` 可发现 docs 与 invertedIndex 之间的不一致状态
