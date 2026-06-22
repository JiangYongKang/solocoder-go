package trie

import (
	"errors"
	"sort"
	"sync"
)

var (
	ErrEmptyWord        = errors.New("trie: empty word")
	ErrEmptyPrefix      = errors.New("trie: empty prefix")
	ErrEmptyPattern     = errors.New("trie: empty pattern")
	ErrEmptyQuery       = errors.New("trie: empty query")
	ErrInvalidMaxResult = errors.New("trie: invalid max result")
	ErrWordNotFound     = errors.New("trie: word not found")
)

type SearchResult struct {
	Word string
	Data interface{}
}

type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	data     interface{}
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
	}
}

type Trie struct {
	root *trieNode
	mu   sync.RWMutex
	size int
}

func NewTrie() *Trie {
	return &Trie{
		root: newTrieNode(),
	}
}

func (t *Trie) Insert(word string, data interface{}) error {
	if word == "" {
		return ErrEmptyWord
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	for _, ch := range word {
		if node.children[ch] == nil {
			node.children[ch] = newTrieNode()
		}
		node = node.children[ch]
	}

	if !node.isEnd {
		node.isEnd = true
		t.size++
	}
	node.data = data

	return nil
}

func (t *Trie) Search(word string) (interface{}, bool) {
	if word == "" {
		return nil, false
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range word {
		if node.children[ch] == nil {
			return nil, false
		}
		node = node.children[ch]
	}

	return node.data, node.isEnd
}

func (t *Trie) Delete(word string) error {
	if word == "" {
		return ErrEmptyWord
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	node := t.root
	path := make([]*trieNode, 0, len(word))
	charPath := make([]rune, 0, len(word))

	for _, ch := range word {
		if node.children[ch] == nil {
			return ErrWordNotFound
		}
		path = append(path, node)
		charPath = append(charPath, ch)
		node = node.children[ch]
	}

	if !node.isEnd {
		return ErrWordNotFound
	}

	node.isEnd = false
	node.data = nil
	t.size--

	for i := len(path) - 1; i >= 0; i-- {
		parent := path[i]
		ch := charPath[i]
		child := parent.children[ch]
		if !child.isEnd && len(child.children) == 0 {
			delete(parent.children, ch)
		} else {
			break
		}
	}

	return nil
}

func (t *Trie) PrefixMatch(prefix string) ([]SearchResult, error) {
	return t.PrefixMatchLimit(prefix, 0)
}

func (t *Trie) PrefixMatchLimit(prefix string, maxResults int) ([]SearchResult, error) {
	if prefix == "" {
		return nil, ErrEmptyPrefix
	}
	if maxResults < 0 {
		return nil, ErrInvalidMaxResult
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range prefix {
		if node.children[ch] == nil {
			return []SearchResult{}, nil
		}
		node = node.children[ch]
	}

	var results []SearchResult
	t.collectWords(node, prefix, &results)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	})

	if maxResults > 0 && len(results) > maxResults {
		return results[:maxResults], nil
	}
	return results, nil
}

func (t *Trie) collectWords(node *trieNode, prefix string, results *[]SearchResult) {
	if node.isEnd {
		*results = append(*results, SearchResult{
			Word: prefix,
			Data: node.data,
		})
	}

	for ch, child := range node.children {
		t.collectWords(child, prefix+string(ch), results)
	}
}

func (t *Trie) WildcardSearch(pattern string) ([]SearchResult, error) {
	if pattern == "" {
		return nil, ErrEmptyPattern
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	seen := make(map[string]bool)
	var results []SearchResult
	patternRunes := []rune(pattern)
	t.wildcardDFS(t.root, patternRunes, 0, "", &results, seen)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	})

	return results, nil
}

func (t *Trie) wildcardDFS(node *trieNode, pattern []rune, patternIdx int, currentWord string, results *[]SearchResult, seen map[string]bool) {
	if patternIdx >= len(pattern) {
		if node.isEnd && !seen[currentWord] {
			seen[currentWord] = true
			*results = append(*results, SearchResult{
				Word: currentWord,
				Data: node.data,
			})
		}
		return
	}

	ch := pattern[patternIdx]

	if ch == '*' {
		t.wildcardDFS(node, pattern, patternIdx+1, currentWord, results, seen)

		for childCh, child := range node.children {
			t.wildcardDFS(child, pattern, patternIdx, currentWord+string(childCh), results, seen)
		}
	} else if ch == '.' {
		for childCh, child := range node.children {
			t.wildcardDFS(child, pattern, patternIdx+1, currentWord+string(childCh), results, seen)
		}
	} else {
		if child, ok := node.children[ch]; ok {
			t.wildcardDFS(child, pattern, patternIdx+1, currentWord+string(ch), results, seen)
		}
	}
}

func (t *Trie) LongestMatch(query string) (SearchResult, error) {
	if query == "" {
		return SearchResult{}, ErrEmptyQuery
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	var longestMatch SearchResult
	currentWord := ""
	found := false

	for _, ch := range query {
		if node.children[ch] == nil {
			break
		}
		node = node.children[ch]
		currentWord += string(ch)

		if node.isEnd {
			longestMatch = SearchResult{
				Word: currentWord,
				Data: node.data,
			}
			found = true
		}
	}

	if !found {
		return SearchResult{}, ErrWordNotFound
	}

	return longestMatch, nil
}

func (t *Trie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

func (t *Trie) GetAllWords() []SearchResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var results []SearchResult
	t.collectWords(t.root, "", &results)

	sort.Slice(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	})

	return results
}
