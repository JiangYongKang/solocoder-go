package fulltext

import (
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"unicode"
)

var (
	ErrEmptyQuery       = errors.New("query cannot be empty")
	ErrEmptyDocument    = errors.New("document content cannot be empty")
	ErrEmptyDocID       = errors.New("document ID cannot be empty")
	ErrDuplicateDocID   = errors.New("document ID already exists")
	ErrDocNotFound      = errors.New("document not found")
	ErrNilTokenizer     = errors.New("tokenizer cannot be nil")
	ErrTokenizerExists  = errors.New("tokenizer for language already exists")
	ErrEmptyPhrase      = errors.New("phrase query cannot be empty")
	ErrPhraseTooShort   = errors.New("phrase query must contain at least two terms")
)

type Tokenizer interface {
	Tokenize(text string) []string
}

type DefaultTokenizer struct{}

func NewDefaultTokenizer() *DefaultTokenizer {
	return &DefaultTokenizer{}
}

func (dt *DefaultTokenizer) Tokenize(text string) []string {
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(unicode.ToLower(r))
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

type TermPosting struct {
	DocID     string
	Frequency int
	Positions []int
}

type PostingList struct {
	Postings []*TermPosting
}

type InvertedIndex struct {
	index map[string]*PostingList
	mu    sync.RWMutex
}

func NewInvertedIndex() *InvertedIndex {
	return &InvertedIndex{
		index: make(map[string]*PostingList),
	}
}

func (ii *InvertedIndex) AddTerm(term string, docID string, position int) {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	postingList, exists := ii.index[term]
	if !exists {
		postingList = &PostingList{}
		ii.index[term] = postingList
	}

	var found bool
	for _, posting := range postingList.Postings {
		if posting.DocID == docID {
			posting.Frequency++
			posting.Positions = append(posting.Positions, position)
			found = true
			break
		}
	}

	if !found {
		postingList.Postings = append(postingList.Postings, &TermPosting{
			DocID:     docID,
			Frequency: 1,
			Positions: []int{position},
		})
	}
}

func (ii *InvertedIndex) GetPostingList(term string) (*PostingList, bool) {
	ii.mu.RLock()
	defer ii.mu.RUnlock()

	postingList, exists := ii.index[term]
	return postingList, exists
}

func (ii *InvertedIndex) GetTermCount() int {
	ii.mu.RLock()
	defer ii.mu.RUnlock()
	return len(ii.index)
}

func (ii *InvertedIndex) HasTerm(term string) bool {
	ii.mu.RLock()
	defer ii.mu.RUnlock()
	_, exists := ii.index[term]
	return exists
}

func (ii *InvertedIndex) RemoveDocument(docID string, terms map[string]struct{}) {
	ii.mu.Lock()
	defer ii.mu.Unlock()

	for term := range terms {
		postingList, exists := ii.index[term]
		if !exists {
			continue
		}

		newPostings := postingList.Postings[:0]
		for _, posting := range postingList.Postings {
			if posting.DocID != docID {
				newPostings = append(newPostings, posting)
			}
		}

		if len(newPostings) == 0 {
			delete(ii.index, term)
		} else {
			postingList.Postings = newPostings
		}
	}
}

type Document struct {
	ID      string
	Content string
	Length  int
	Terms   map[string]struct{}
}

type SearchResult struct {
	DocID   string
	Score   float64
	Content string
}

type Engine struct {
	docs          map[string]*Document
	invertedIndex *InvertedIndex
	tokenizers    map[string]Tokenizer
	defaultLang   string
	mu            sync.RWMutex
}

func NewEngine() *Engine {
	e := &Engine{
		docs:          make(map[string]*Document),
		invertedIndex: NewInvertedIndex(),
		tokenizers:    make(map[string]Tokenizer),
		defaultLang:   "default",
	}
	e.tokenizers["default"] = NewDefaultTokenizer()
	return e
}

func (e *Engine) RegisterTokenizer(language string, tokenizer Tokenizer) error {
	if tokenizer == nil {
		return ErrNilTokenizer
	}
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return errors.New("language cannot be empty")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.tokenizers[language]; exists {
		return ErrTokenizerExists
	}

	e.tokenizers[language] = tokenizer
	return nil
}

func (e *Engine) SetDefaultLanguage(language string) error {
	language = strings.ToLower(strings.TrimSpace(language))
	if language == "" {
		return errors.New("language cannot be empty")
	}

	e.mu.RLock()
	_, exists := e.tokenizers[language]
	e.mu.RUnlock()

	if !exists {
		return errors.New("tokenizer for language not registered")
	}

	e.mu.Lock()
	e.defaultLang = language
	e.mu.Unlock()

	return nil
}

func (e *Engine) getTokenizer(language string) Tokenizer {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if language != "" {
		if t, ok := e.tokenizers[strings.ToLower(language)]; ok {
			return t
		}
	}
	return e.tokenizers[e.defaultLang]
}

func (e *Engine) AddDocument(docID string, content string) error {
	return e.AddDocumentWithLanguage(docID, content, "")
}

func (e *Engine) AddDocumentWithLanguage(docID string, content string, language string) error {
	if strings.TrimSpace(docID) == "" {
		return ErrEmptyDocID
	}
	if strings.TrimSpace(content) == "" {
		return ErrEmptyDocument
	}

	tokenizer := e.getTokenizer(language)
	tokens := tokenizer.Tokenize(content)
	if len(tokens) == 0 {
		return ErrEmptyDocument
	}

	terms := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		terms[token] = struct{}{}
	}

	doc := &Document{
		ID:      docID,
		Content: content,
		Length:  len(tokens),
		Terms:   terms,
	}

	e.mu.Lock()
	if _, exists := e.docs[docID]; exists {
		e.mu.Unlock()
		return ErrDuplicateDocID
	}
	for pos, token := range tokens {
		e.invertedIndex.AddTerm(token, docID, pos)
	}
	e.docs[docID] = doc
	e.mu.Unlock()

	return nil
}

func (e *Engine) DeleteDocument(docID string) error {
	if strings.TrimSpace(docID) == "" {
		return ErrEmptyDocID
	}

	e.mu.Lock()
	doc, exists := e.docs[docID]
	if !exists {
		e.mu.Unlock()
		return ErrDocNotFound
	}
	e.invertedIndex.RemoveDocument(docID, doc.Terms)
	delete(e.docs, docID)
	e.mu.Unlock()

	return nil
}

func (e *Engine) GetDocument(docID string) (*Document, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	doc, exists := e.docs[docID]
	return doc, exists
}

func (e *Engine) DocumentCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.docs)
}

func (e *Engine) Search(query string) ([]*SearchResult, error) {
	return e.SearchWithLanguage(query, "")
}

func (e *Engine) SearchWithLanguage(query string, language string) ([]*SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, ErrEmptyQuery
	}

	tokenizer := e.getTokenizer(language)
	queryTokens := tokenizer.Tokenize(query)
	if len(queryTokens) == 0 {
		return []*SearchResult{}, nil
	}

	e.mu.RLock()
	totalDocs := len(e.docs)

	if totalDocs == 0 {
		e.mu.RUnlock()
		return []*SearchResult{}, nil
	}

	type termPostingData struct {
		docsWithTerm int
		postings     []*TermPosting
	}
	termDataList := make([]termPostingData, 0, len(queryTokens))

	for _, term := range queryTokens {
		postingList, exists := e.invertedIndex.GetPostingList(term)
		if !exists {
			continue
		}
		termDataList = append(termDataList, termPostingData{
			docsWithTerm: len(postingList.Postings),
			postings:     postingList.Postings,
		})
	}

	docScores := make(map[string]float64)
	docContents := make(map[string]string)

	for _, td := range termDataList {
		idf := math.Log(1 + float64(totalDocs)/float64(td.docsWithTerm+1))

		for _, posting := range td.postings {
			doc, docExists := e.docs[posting.DocID]
			if !docExists {
				continue
			}

			tf := float64(posting.Frequency) / float64(doc.Length)
			tfidf := tf * idf

			docScores[posting.DocID] += tfidf
			if _, exists := docContents[posting.DocID]; !exists {
				docContents[posting.DocID] = doc.Content
			}
		}
	}
	e.mu.RUnlock()

	results := make([]*SearchResult, 0, len(docScores))
	for docID, score := range docScores {
		content, ok := docContents[docID]
		if !ok {
			continue
		}
		results = append(results, &SearchResult{
			DocID:   docID,
			Score:   score,
			Content: content,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func (e *Engine) SearchPhrase(phrase string) ([]*SearchResult, error) {
	return e.SearchPhraseWithLanguage(phrase, "")
}

func (e *Engine) SearchPhraseWithLanguage(phrase string, language string) ([]*SearchResult, error) {
	if strings.TrimSpace(phrase) == "" {
		return nil, ErrEmptyPhrase
	}

	tokenizer := e.getTokenizer(language)
	terms := tokenizer.Tokenize(phrase)
	if len(terms) < 2 {
		return nil, ErrPhraseTooShort
	}

	e.mu.RLock()
	totalDocs := len(e.docs)
	if totalDocs == 0 {
		e.mu.RUnlock()
		return []*SearchResult{}, nil
	}

	postingLists := make([]*PostingList, 0, len(terms))
	for _, term := range terms {
		pl, exists := e.invertedIndex.GetPostingList(term)
		if !exists {
			e.mu.RUnlock()
			return []*SearchResult{}, nil
		}
		postingLists = append(postingLists, pl)
	}

	candidateDocs := make(map[string][][]int)
	for _, posting := range postingLists[0].Postings {
		posCopy := make([]int, len(posting.Positions))
		copy(posCopy, posting.Positions)
		candidateDocs[posting.DocID] = [][]int{posCopy}
	}

	for i := 1; i < len(postingLists); i++ {
		nextCandidates := make(map[string][][]int)
		for _, posting := range postingLists[i].Postings {
			if prevPositions, exists := candidateDocs[posting.DocID]; exists {
				posCopy := make([]int, len(posting.Positions))
				copy(posCopy, posting.Positions)
				nextCandidates[posting.DocID] = append(prevPositions, posCopy)
			}
		}
		candidateDocs = nextCandidates
		if len(candidateDocs) == 0 {
			e.mu.RUnlock()
			return []*SearchResult{}, nil
		}
	}

	matchedDocs := make(map[string]bool)
	for docID, positionsList := range candidateDocs {
		if len(positionsList) != len(terms) {
			continue
		}
		if checkPhraseMatch(positionsList) {
			matchedDocs[docID] = true
		}
	}

	type termPostingData struct {
		docsWithTerm int
		postings     []*TermPosting
	}
	termDataList := make([]termPostingData, 0, len(terms))
	for _, term := range terms {
		postingList, exists := e.invertedIndex.GetPostingList(term)
		if !exists {
			continue
		}
		termDataList = append(termDataList, termPostingData{
			docsWithTerm: len(postingList.Postings),
			postings:     postingList.Postings,
		})
	}

	docScores := make(map[string]float64)
	docContents := make(map[string]string)

	for docID := range matchedDocs {
		doc, docExists := e.docs[docID]
		if !docExists {
			continue
		}
		docContents[docID] = doc.Content
		var score float64
		for _, td := range termDataList {
			idf := math.Log(1 + float64(totalDocs)/float64(td.docsWithTerm+1))
			for _, posting := range td.postings {
				if posting.DocID != docID {
					continue
				}
				tf := float64(posting.Frequency) / float64(doc.Length)
				score += tf * idf
			}
		}
		docScores[docID] = score
	}
	e.mu.RUnlock()

	results := make([]*SearchResult, 0, len(docScores))
	for docID, score := range docScores {
		content, ok := docContents[docID]
		if !ok {
			continue
		}
		results = append(results, &SearchResult{
			DocID:   docID,
			Score:   score,
			Content: content,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

func checkPhraseMatch(positionsList [][]int) bool {
	if len(positionsList) < 2 {
		return false
	}

	firstPositions := positionsList[0]
	for _, startPos := range firstPositions {
		matched := true
		for i := 1; i < len(positionsList); i++ {
			expectedPos := startPos + i
			found := false
			for _, pos := range positionsList[i] {
				if pos == expectedPos {
					found = true
					break
				}
			}
			if !found {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}

	return false
}
