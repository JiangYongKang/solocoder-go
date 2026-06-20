package suggest

import (
	"errors"
	"sort"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrEmptyWord        = errors.New("suggest: empty word")
	ErrEmptyPrefix      = errors.New("suggest: empty prefix")
	ErrEmptyQuery       = errors.New("suggest: empty query")
	ErrEmptyUserID      = errors.New("suggest: empty user id")
	ErrInvalidMaxResult = errors.New("suggest: invalid max result")
	ErrInvalidMaxDist   = errors.New("suggest: invalid max distance")
	ErrInvalidHistoryN  = errors.New("suggest: invalid history n")
	ErrWordNotFound     = errors.New("suggest: word not found")
)

type Suggestion struct {
	Word      string
	Frequency int
}

type HistoryRecord struct {
	Word      string
	Timestamp time.Time
}

type trieNode struct {
	children map[rune]*trieNode
	isEnd    bool
	freq     int
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

func (t *Trie) Insert(word string) error {
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
	node.freq++

	return nil
}

func (t *Trie) InsertWithFreq(word string, freq int) error {
	if word == "" {
		return ErrEmptyWord
	}
	if freq < 0 {
		freq = 0
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
	node.freq = freq

	return nil
}

func (t *Trie) EnsureWord(word string) error {
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
		node.freq = 0
		t.size++
	}

	return nil
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
	node.freq = 0
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

func (t *Trie) Search(word string) (bool, int) {
	if word == "" {
		return false, 0
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range word {
		if node.children[ch] == nil {
			return false, 0
		}
		node = node.children[ch]
	}

	return node.isEnd, node.freq
}

func (t *Trie) StartsWith(prefix string) ([]Suggestion, error) {
	if prefix == "" {
		return nil, ErrEmptyPrefix
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.root
	for _, ch := range prefix {
		if node.children[ch] == nil {
			return []Suggestion{}, nil
		}
		node = node.children[ch]
	}

	var results []Suggestion
	t.collectWords(node, prefix, &results)

	sort.Slice(results, func(i, j int) bool {
		if results[i].Frequency != results[j].Frequency {
			return results[i].Frequency > results[j].Frequency
		}
		return results[i].Word < results[j].Word
	})

	return results, nil
}

func (t *Trie) StartsWithLimit(prefix string, maxResults int) ([]Suggestion, error) {
	if prefix == "" {
		return nil, ErrEmptyPrefix
	}
	if maxResults <= 0 {
		return nil, ErrInvalidMaxResult
	}

	all, err := t.StartsWith(prefix)
	if err != nil {
		return nil, err
	}

	if len(all) > maxResults {
		return all[:maxResults], nil
	}
	return all, nil
}

func (t *Trie) collectWords(node *trieNode, prefix string, results *[]Suggestion) {
	if node.isEnd {
		*results = append(*results, Suggestion{
			Word:      prefix,
			Frequency: node.freq,
		})
	}

	for ch, child := range node.children {
		t.collectWords(child, prefix+string(ch), results)
	}
}

func (t *Trie) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

func (t *Trie) GetAllWords() []Suggestion {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var results []Suggestion
	t.collectWords(t.root, "", &results)

	sort.Slice(results, func(i, j int) bool {
		if results[i].Frequency != results[j].Frequency {
			return results[i].Frequency > results[j].Frequency
		}
		return results[i].Word < results[j].Word
	})

	return results
}

func EditDistance(a, b string) int {
	la := utf8.RuneCountInString(a)
	lb := utf8.RuneCountInString(b)

	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	ra := []rune(a)
	rb := []rune(b)

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := 0; j <= lb; j++ {
		prev[j] = j
	}

	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			curr[j] = minThree(
				prev[j]+1,
				curr[j-1]+1,
				prev[j-1]+cost,
			)
		}
		prev, curr = curr, prev
	}

	return prev[lb]
}

func minThree(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}

type SearchHistory struct {
	mu       sync.RWMutex
	history  map[string][]*HistoryRecord
	maxSize  int
}

func NewSearchHistory() *SearchHistory {
	return &SearchHistory{
		history: make(map[string][]*HistoryRecord),
		maxSize: 100,
	}
}

func NewSearchHistoryWithMaxSize(maxSize int) (*SearchHistory, error) {
	if maxSize <= 0 {
		return nil, ErrInvalidHistoryN
	}
	return &SearchHistory{
		history: make(map[string][]*HistoryRecord),
		maxSize: maxSize,
	}, nil
}

func (h *SearchHistory) Add(userID, word string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if word == "" {
		return ErrEmptyWord
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	records := h.history[userID]

	for i, r := range records {
		if r.Word == word {
			copy(records[1:i+1], records[:i])
			records[0] = &HistoryRecord{
				Word:      word,
				Timestamp: time.Now(),
			}
			h.history[userID] = records
			return nil
		}
	}

	record := &HistoryRecord{
		Word:      word,
		Timestamp: time.Now(),
	}

	if len(records) >= h.maxSize {
		records = records[:h.maxSize-1]
	}
	records = append([]*HistoryRecord{record}, records...)
	h.history[userID] = records

	return nil
}

func (h *SearchHistory) GetRecent(userID string, n int) ([]*HistoryRecord, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}
	if n <= 0 {
		return nil, ErrInvalidHistoryN
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	records, exists := h.history[userID]
	if !exists {
		return []*HistoryRecord{}, nil
	}

	if n > len(records) {
		n = len(records)
	}

	result := make([]*HistoryRecord, n)
	for i := 0; i < n; i++ {
		result[i] = &HistoryRecord{
			Word:      records[i].Word,
			Timestamp: records[i].Timestamp,
		}
	}

	return result, nil
}

func (h *SearchHistory) Clear(userID string) error {
	if userID == "" {
		return ErrEmptyUserID
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.history, userID)

	return nil
}

func (h *SearchHistory) Count(userID string) (int, error) {
	if userID == "" {
		return 0, ErrEmptyUserID
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.history[userID]), nil
}

type SuggestEngine struct {
	trie         *Trie
	history      *SearchHistory
	mu           sync.RWMutex
	maxEditDist  int
	defaultLimit int
}

type Config struct {
	MaxEditDistance  int
	DefaultMaxResult int
	HistoryMaxSize   int
}

func DefaultConfig() Config {
	return Config{
		MaxEditDistance:  2,
		DefaultMaxResult: 10,
		HistoryMaxSize:   100,
	}
}

func NewSuggestEngine() *SuggestEngine {
	eng, err := NewSuggestEngineWithConfig(DefaultConfig())
	if err != nil {
		panic("suggest: DefaultConfig is invalid: " + err.Error())
	}
	return eng
}

func NewSuggestEngineWithConfig(cfg Config) (*SuggestEngine, error) {
	if cfg.MaxEditDistance < 0 {
		return nil, ErrInvalidMaxDist
	}
	if cfg.DefaultMaxResult <= 0 {
		return nil, ErrInvalidMaxResult
	}
	if cfg.HistoryMaxSize <= 0 {
		return nil, ErrInvalidHistoryN
	}

	history, err := NewSearchHistoryWithMaxSize(cfg.HistoryMaxSize)
	if err != nil {
		return nil, err
	}

	return &SuggestEngine{
		trie:         NewTrie(),
		history:      history,
		maxEditDist:  cfg.MaxEditDistance,
		defaultLimit: cfg.DefaultMaxResult,
	}, nil
}

func (e *SuggestEngine) AddWord(word string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.trie.EnsureWord(word)
}

func (e *SuggestEngine) AddWordWithFreq(word string, freq int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.trie.InsertWithFreq(word, freq)
}

func (e *SuggestEngine) RemoveWord(word string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.trie.Delete(word)
}

func (e *SuggestEngine) HasWord(word string) (bool, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.trie.Search(word)
}

func (e *SuggestEngine) WordCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.trie.Size()
}

func (e *SuggestEngine) Autocomplete(prefix string) ([]Suggestion, error) {
	return e.AutocompleteLimit(prefix, e.defaultLimit)
}

func (e *SuggestEngine) AutocompleteLimit(prefix string, maxResults int) ([]Suggestion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.trie.StartsWithLimit(prefix, maxResults)
}

func (e *SuggestEngine) Correct(query string) ([]Suggestion, error) {
	return e.CorrectLimit(query, e.defaultLimit)
}

func (e *SuggestEngine) CorrectLimit(query string, maxResults int) ([]Suggestion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.correctNoLock(query, maxResults)
}

func (e *SuggestEngine) SubmitSearch(userID, word string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if word == "" {
		return ErrEmptyWord
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	err := e.trie.Insert(word)
	if err != nil {
		return err
	}

	err = e.history.Add(userID, word)
	if err != nil {
		return err
	}

	return nil
}

func (e *SuggestEngine) GetHistory(userID string, n int) ([]*HistoryRecord, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.history.GetRecent(userID, n)
}

func (e *SuggestEngine) ClearHistory(userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.history.Clear(userID)
}

func (e *SuggestEngine) Suggest(query string) ([]Suggestion, error) {
	return e.SuggestLimit(query, e.defaultLimit)
}

func (e *SuggestEngine) SuggestLimit(query string, maxResults int) ([]Suggestion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if query == "" {
		return nil, ErrEmptyQuery
	}
	if maxResults <= 0 {
		return nil, ErrInvalidMaxResult
	}

	autocomplete, err := e.trie.StartsWithLimit(query, maxResults)
	if err != nil {
		return nil, err
	}

	if len(autocomplete) >= maxResults {
		return autocomplete[:maxResults], nil
	}

	remaining := maxResults - len(autocomplete)
	corrections, err := e.correctNoLock(query, remaining)
	if err != nil {
		return autocomplete, nil
	}

	existing := make(map[string]bool)
	for _, s := range autocomplete {
		existing[s.Word] = true
	}

	for _, c := range corrections {
		if !existing[c.Word] {
			autocomplete = append(autocomplete, c)
			existing[c.Word] = true
		}
	}

	return autocomplete, nil
}

func (e *SuggestEngine) GetHotWords(n int) ([]Suggestion, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if n <= 0 {
		return nil, ErrInvalidMaxResult
	}

	all := e.trie.GetAllWords()
	if len(all) > n {
		return all[:n], nil
	}
	return all, nil
}

func (e *SuggestEngine) correctNoLock(query string, maxResults int) ([]Suggestion, error) {
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if maxResults <= 0 {
		return nil, ErrInvalidMaxResult
	}

	exists, _ := e.trie.Search(query)
	if exists {
		return []Suggestion{}, nil
	}

	allWords := e.trie.GetAllWords()
	if len(allWords) == 0 {
		return []Suggestion{}, nil
	}

	type candidate struct {
		word Suggestion
		dist int
	}

	var candidates []candidate
	for _, w := range allWords {
		dist := EditDistance(query, w.Word)
		if dist <= e.maxEditDist {
			candidates = append(candidates, candidate{word: w, dist: dist})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].dist != candidates[j].dist {
			return candidates[i].dist < candidates[j].dist
		}
		if candidates[i].word.Frequency != candidates[j].word.Frequency {
			return candidates[i].word.Frequency > candidates[j].word.Frequency
		}
		return candidates[i].word.Word < candidates[j].word.Word
	})

	resultCount := len(candidates)
	if resultCount > maxResults {
		resultCount = maxResults
	}

	results := make([]Suggestion, resultCount)
	for i := 0; i < resultCount; i++ {
		results[i] = candidates[i].word
	}

	return results, nil
}
