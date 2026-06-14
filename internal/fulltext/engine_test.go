package fulltext

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
)

type MockChineseTokenizer struct{}

func (mct *MockChineseTokenizer) Tokenize(text string) []string {
	var tokens []string
	for _, r := range text {
		if r == ' ' || r == ',' || r == '.' || r == '!' || r == '?' {
			continue
		}
		tokens = append(tokens, string(r))
	}
	return tokens
}

type WhitespaceOnlyTokenizer struct{}

func (wst *WhitespaceOnlyTokenizer) Tokenize(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

func TestNewEngine(t *testing.T) {
	e := NewEngine()
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.DocumentCount() != 0 {
		t.Errorf("expected initial document count 0, got %d", e.DocumentCount())
	}
	if e.invertedIndex == nil {
		t.Error("invertedIndex should not be nil")
	}
	if len(e.tokenizers) != 1 {
		t.Errorf("expected 1 tokenizer (default), got %d", len(e.tokenizers))
	}
}

func TestDefaultTokenizer_Basic(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("Hello World")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "hello" {
		t.Errorf("expected 'hello', got '%s'", tokens[0])
	}
	if tokens[1] != "world" {
		t.Errorf("expected 'world', got '%s'", tokens[1])
	}
}

func TestDefaultTokenizer_Punctuation(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("Hello, World! How are you?")
	if len(tokens) != 5 {
		t.Fatalf("expected 5 tokens, got %d: %v", len(tokens), tokens)
	}
	expected := []string{"hello", "world", "how", "are", "you"}
	for i, exp := range expected {
		if tokens[i] != exp {
			t.Errorf("position %d: expected '%s', got '%s'", i, exp, tokens[i])
		}
	}
}

func TestDefaultTokenizer_Numbers(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("Test123 and 456")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "test123" {
		t.Errorf("expected 'test123', got '%s'", tokens[0])
	}
	if tokens[2] != "456" {
		t.Errorf("expected '456', got '%s'", tokens[2])
	}
}

func TestDefaultTokenizer_Empty(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", len(tokens))
	}

	tokens = tokenizer.Tokenize("   ,.!?   ")
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens for punctuation only, got %d", len(tokens))
	}
}

func TestDefaultTokenizer_MixedCase(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("GoLaNGo Is AWESOME")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0] != "golango" {
		t.Errorf("expected lowercase 'golango', got '%s'", tokens[0])
	}
	if tokens[1] != "is" {
		t.Errorf("expected 'is', got '%s'", tokens[1])
	}
	if tokens[2] != "awesome" {
		t.Errorf("expected 'awesome', got '%s'", tokens[2])
	}
}

func TestRegisterTokenizer(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("zh", &MockChineseTokenizer{})
	if err != nil {
		t.Fatalf("RegisterTokenizer failed: %v", err)
	}

	err = e.RegisterTokenizer("en", &WhitespaceOnlyTokenizer{})
	if err != nil {
		t.Fatalf("RegisterTokenizer for 'en' failed: %v", err)
	}
}

func TestRegisterTokenizer_Nil(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("zh", nil)
	if err != ErrNilTokenizer {
		t.Errorf("expected ErrNilTokenizer, got %v", err)
	}
}

func TestRegisterTokenizer_EmptyLanguage(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("", &MockChineseTokenizer{})
	if err == nil {
		t.Error("expected error for empty language, got nil")
	}

	err = e.RegisterTokenizer("   ", &MockChineseTokenizer{})
	if err == nil {
		t.Error("expected error for whitespace-only language, got nil")
	}
}

func TestRegisterTokenizer_Duplicate(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("zh", &MockChineseTokenizer{})
	if err != nil {
		t.Fatalf("first RegisterTokenizer failed: %v", err)
	}

	err = e.RegisterTokenizer("zh", &WhitespaceOnlyTokenizer{})
	if err != ErrTokenizerExists {
		t.Errorf("expected ErrTokenizerExists, got %v", err)
	}

	err = e.RegisterTokenizer("ZH", &WhitespaceOnlyTokenizer{})
	if err != ErrTokenizerExists {
		t.Errorf("expected ErrTokenizerExists for case-insensitive duplicate, got %v", err)
	}
}

func TestRegisterTokenizer_DefaultCannotBeReplaced(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("default", &WhitespaceOnlyTokenizer{})
	if err != ErrTokenizerExists {
		t.Errorf("expected ErrTokenizerExists when replacing 'default', got %v", err)
	}
}

func TestSetDefaultLanguage(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("zh", &MockChineseTokenizer{})
	if err != nil {
		t.Fatalf("RegisterTokenizer failed: %v", err)
	}

	err = e.SetDefaultLanguage("zh")
	if err != nil {
		t.Fatalf("SetDefaultLanguage failed: %v", err)
	}
}

func TestSetDefaultLanguage_NotRegistered(t *testing.T) {
	e := NewEngine()

	err := e.SetDefaultLanguage("nonexistent")
	if err == nil {
		t.Error("expected error for unregistered language, got nil")
	}
}

func TestSetDefaultLanguage_Empty(t *testing.T) {
	e := NewEngine()

	err := e.SetDefaultLanguage("")
	if err == nil {
		t.Error("expected error for empty language, got nil")
	}
}

func TestAddDocument(t *testing.T) {
	e := NewEngine()

	err := e.AddDocument("doc1", "Hello World")
	if err != nil {
		t.Fatalf("AddDocument failed: %v", err)
	}

	if e.DocumentCount() != 1 {
		t.Errorf("expected document count 1, got %d", e.DocumentCount())
	}

	doc, exists := e.GetDocument("doc1")
	if !exists {
		t.Fatal("document doc1 should exist")
	}
	if doc.ID != "doc1" {
		t.Errorf("expected ID 'doc1', got '%s'", doc.ID)
	}
	if doc.Content != "Hello World" {
		t.Errorf("expected content 'Hello World', got '%s'", doc.Content)
	}
	if doc.Length != 2 {
		t.Errorf("expected length 2, got %d", doc.Length)
	}
}

func TestAddDocument_EmptyDocID(t *testing.T) {
	e := NewEngine()

	err := e.AddDocument("", "Hello World")
	if err != ErrEmptyDocID {
		t.Errorf("expected ErrEmptyDocID, got %v", err)
	}

	err = e.AddDocument("   ", "Hello World")
	if err != ErrEmptyDocID {
		t.Errorf("expected ErrEmptyDocID for whitespace, got %v", err)
	}
}

func TestAddDocument_EmptyContent(t *testing.T) {
	e := NewEngine()

	err := e.AddDocument("doc1", "")
	if err != ErrEmptyDocument {
		t.Errorf("expected ErrEmptyDocument, got %v", err)
	}

	err = e.AddDocument("doc1", "   ")
	if err != ErrEmptyDocument {
		t.Errorf("expected ErrEmptyDocument for whitespace, got %v", err)
	}
}

func TestAddDocument_Duplicate(t *testing.T) {
	e := NewEngine()

	err := e.AddDocument("doc1", "First version")
	if err != nil {
		t.Fatalf("first AddDocument failed: %v", err)
	}

	err = e.AddDocument("doc1", "Second version")
	if err != ErrDuplicateDocID {
		t.Errorf("expected ErrDuplicateDocID, got %v", err)
	}

	if e.DocumentCount() != 1 {
		t.Errorf("document count should remain 1, got %d", e.DocumentCount())
	}
}

func TestAddDocument_Multiple(t *testing.T) {
	e := NewEngine()

	docs := map[string]string{
		"doc1": "Go is a programming language",
		"doc2": "Go is also a board game",
		"doc3": "Python is another programming language",
	}

	for id, content := range docs {
		err := e.AddDocument(id, content)
		if err != nil {
			t.Fatalf("AddDocument %s failed: %v", id, err)
		}
	}

	if e.DocumentCount() != 3 {
		t.Errorf("expected 3 documents, got %d", e.DocumentCount())
	}
}

func TestAddDocumentWithLanguage(t *testing.T) {
	e := NewEngine()
	e.RegisterTokenizer("zh", &MockChineseTokenizer{})

	err := e.AddDocumentWithLanguage("zh1", "你好世界", "zh")
	if err != nil {
		t.Fatalf("AddDocumentWithLanguage failed: %v", err)
	}

	doc, _ := e.GetDocument("zh1")
	if doc.Length != 4 {
		t.Errorf("expected Chinese doc length 4, got %d", doc.Length)
	}
}

func TestDeleteDocument(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "Hello World")
	e.AddDocument("doc2", "Goodbye World")

	err := e.DeleteDocument("doc1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if e.DocumentCount() != 1 {
		t.Errorf("expected 1 document remaining, got %d", e.DocumentCount())
	}

	_, exists := e.GetDocument("doc1")
	if exists {
		t.Error("doc1 should not exist after deletion")
	}

	_, exists = e.GetDocument("doc2")
	if !exists {
		t.Error("doc2 should still exist")
	}
}

func TestDeleteDocument_EmptyID(t *testing.T) {
	e := NewEngine()

	err := e.DeleteDocument("")
	if err != ErrEmptyDocID {
		t.Errorf("expected ErrEmptyDocID, got %v", err)
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	e := NewEngine()

	err := e.DeleteDocument("nonexistent")
	if err != ErrDocNotFound {
		t.Errorf("expected ErrDocNotFound, got %v", err)
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	e := NewEngine()

	doc, exists := e.GetDocument("nonexistent")
	if exists {
		t.Error("should return false for non-existent document")
	}
	if doc != nil {
		t.Error("doc should be nil for non-existent document")
	}
}

func TestInvertedIndex_AddAndGet(t *testing.T) {
	ii := NewInvertedIndex()

	ii.AddTerm("hello", "doc1", 0)
	ii.AddTerm("hello", "doc1", 5)
	ii.AddTerm("world", "doc1", 1)
	ii.AddTerm("hello", "doc2", 0)

	if !ii.HasTerm("hello") {
		t.Error("inverted index should have term 'hello'")
	}
	if !ii.HasTerm("world") {
		t.Error("inverted index should have term 'world'")
	}
	if ii.HasTerm("nonexistent") {
		t.Error("inverted index should not have term 'nonexistent'")
	}
	if ii.GetTermCount() != 2 {
		t.Errorf("expected 2 terms, got %d", ii.GetTermCount())
	}

	pl, exists := ii.GetPostingList("hello")
	if !exists {
		t.Fatal("posting list for 'hello' should exist")
	}
	if len(pl.Postings) != 2 {
		t.Errorf("expected 2 postings for 'hello', got %d", len(pl.Postings))
	}

	for _, posting := range pl.Postings {
		if posting.DocID == "doc1" {
			if posting.Frequency != 2 {
				t.Errorf("expected frequency 2 for doc1, got %d", posting.Frequency)
			}
			if len(posting.Positions) != 2 {
				t.Errorf("expected 2 positions for doc1, got %d", len(posting.Positions))
			}
		} else if posting.DocID == "doc2" {
			if posting.Frequency != 1 {
				t.Errorf("expected frequency 1 for doc2, got %d", posting.Frequency)
			}
		}
	}
}

func TestSearch_Basic(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "Go programming language")
	e.AddDocument("doc2", "Python programming language")
	e.AddDocument("doc3", "Board game night")

	results, err := e.Search("programming")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	found := make(map[string]bool)
	for _, r := range results {
		found[r.DocID] = true
	}
	if !found["doc1"] {
		t.Error("doc1 should be in results")
	}
	if !found["doc2"] {
		t.Error("doc2 should be in results")
	}
	if found["doc3"] {
		t.Error("doc3 should not be in results")
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	e := NewEngine()

	_, err := e.Search("")
	if err != ErrEmptyQuery {
		t.Errorf("expected ErrEmptyQuery, got %v", err)
	}

	_, err = e.Search("   ")
	if err != ErrEmptyQuery {
		t.Errorf("expected ErrEmptyQuery for whitespace, got %v", err)
	}
}

func TestSearch_NoResults(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "Hello World")

	results, err := e.Search("nonexistent")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearch_EmptyEngine(t *testing.T) {
	e := NewEngine()

	results, err := e.Search("test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty engine, got %d", len(results))
	}
}

func TestSearch_TFIDF_Sorting(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "Go Go Go programming language language")
	e.AddDocument("doc2", "Go programming language")
	e.AddDocument("doc3", "board game chess")

	results, err := e.Search("Go language")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results should be sorted by score descending: result[%d]=%f > result[%d]=%f",
				i, results[i].Score, i-1, results[i-1].Score)
		}
	}

	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1 to have highest score (more Go/language frequency), got %s", results[0].DocID)
	}
}

func TestSearch_RareTermHigherScore(t *testing.T) {
	e := NewEngine()

	for i := 0; i < 100; i++ {
		e.AddDocument(fmt.Sprintf("doc%d", i), fmt.Sprintf("the the the content %d", i))
	}
	e.AddDocument("rare_doc", "unique rare special word")

	resultsRare, err := e.Search("unique")
	if err != nil {
		t.Fatalf("Search for rare term failed: %v", err)
	}
	if len(resultsRare) != 1 {
		t.Fatalf("expected 1 result for 'unique', got %d", len(resultsRare))
	}

	resultsCommon, err := e.Search("the")
	if err != nil {
		t.Fatalf("Search for common term failed: %v", err)
	}
	if len(resultsCommon) != 100 {
		t.Fatalf("expected 100 results for 'the', got %d", len(resultsCommon))
	}

	if resultsRare[0].Score <= resultsCommon[0].Score {
		t.Errorf("rare term should have higher score than common term: rare=%f, common=%f",
			resultsRare[0].Score, resultsCommon[0].Score)
	}
}

func TestSearch_SearchResultContent(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "Sample content here")

	results, err := e.Search("sample")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "Sample content here" {
		t.Errorf("expected content to be preserved, got '%s'", results[0].Content)
	}
	if results[0].DocID != "doc1" {
		t.Errorf("expected DocID 'doc1', got '%s'", results[0].DocID)
	}
	if results[0].Score <= 0 {
		t.Errorf("expected positive score, got %f", results[0].Score)
	}
}

func TestSearchWithLanguage(t *testing.T) {
	e := NewEngine()
	e.RegisterTokenizer("zh", &MockChineseTokenizer{})

	e.AddDocumentWithLanguage("zh1", "你好世界", "zh")
	e.AddDocument("en1", "hello world")

	results, err := e.SearchWithLanguage("你好", "zh")
	if err != nil {
		t.Fatalf("SearchWithLanguage failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 Chinese result, got %d", len(results))
	}

	resultsEn, err := e.Search("hello")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(resultsEn) != 1 {
		t.Errorf("expected 1 English result, got %d", len(resultsEn))
	}
}

func TestSearchPhrase_Basic(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "the quick brown fox jumps")
	e.AddDocument("doc2", "the slow brown dog walks")
	e.AddDocument("doc3", "brown quick the fox")

	results, err := e.SearchPhrase("quick brown fox")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for exact phrase, got %d", len(results))
	}
	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1 to match phrase, got %s", results[0].DocID)
	}
}

func TestSearchPhrase_WrongOrder(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "fox brown quick")
	e.AddDocument("doc2", "quick not brown fox")

	results, err := e.SearchPhrase("quick brown fox")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for wrong order/interrupted phrase, got %d", len(results))
	}
}

func TestSearchPhrase_Empty(t *testing.T) {
	e := NewEngine()

	_, err := e.SearchPhrase("")
	if err != ErrEmptyPhrase {
		t.Errorf("expected ErrEmptyPhrase, got %v", err)
	}

	_, err = e.SearchPhrase("   ")
	if err != ErrEmptyPhrase {
		t.Errorf("expected ErrEmptyPhrase for whitespace, got %v", err)
	}
}

func TestSearchPhrase_SingleTerm(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world")

	_, err := e.SearchPhrase("hello")
	if err != ErrPhraseTooShort {
		t.Errorf("expected ErrPhraseTooShort for single term, got %v", err)
	}
}

func TestSearchPhrase_NoMatch(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world")

	results, err := e.SearchPhrase("foo bar")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchPhrase_MultipleMatches(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "machine learning is great machine learning rocks")
	e.AddDocument("doc2", "deep learning and machine learning")
	e.AddDocument("doc3", "just machine code")

	results, err := e.SearchPhrase("machine learning")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	found := make(map[string]bool)
	for _, r := range results {
		found[r.DocID] = true
	}
	if !found["doc1"] {
		t.Error("doc1 should match 'machine learning'")
	}
	if !found["doc2"] {
		t.Error("doc2 should match 'machine learning'")
	}
	if found["doc3"] {
		t.Error("doc3 should not match 'machine learning'")
	}
}

func TestSearchPhrase_SortedByScore(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "machine learning machine learning machine learning")
	e.AddDocument("doc2", "machine learning with python")

	results, err := e.SearchPhrase("machine learning")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("phrase results should be sorted by score descending")
		}
	}

	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1 (more occurrences) to rank higher, got %s", results[0].DocID)
	}
}

func TestSearchPhrase_TermNotInIndex(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world")

	results, err := e.SearchPhrase("hello nonexistent")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results when term not in index, got %d", len(results))
	}
}

func TestSearchPhrase_AdjacentWords(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "a b c d e")
	e.AddDocument("doc2", "a c b d e")

	results, err := e.SearchPhrase("b c d")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1 to match 'b c d', got %s", results[0].DocID)
	}
}

func TestConcurrent_AddDocument(t *testing.T) {
	e := NewEngine()

	var wg sync.WaitGroup
	numGoroutines := 20
	numDocs := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numDocs; i++ {
				docID := fmt.Sprintf("g%d_d%d", id, i)
				content := fmt.Sprintf("document content from goroutine %d doc %d", id, i)
				err := e.AddDocument(docID, content)
				if err != nil {
					t.Errorf("AddDocument failed for %s: %v", docID, err)
				}
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numDocs
	if e.DocumentCount() != expected {
		t.Errorf("expected %d documents, got %d", expected, e.DocumentCount())
	}
}

func TestConcurrent_Search(t *testing.T) {
	e := NewEngine()

	for i := 0; i < 100; i++ {
		e.AddDocument(fmt.Sprintf("doc%d", i), fmt.Sprintf("test content number %d with keyword", i))
	}

	var wg sync.WaitGroup
	numSearchers := 30
	errorsFound := int64(0)

	for s := 0; s < numSearchers; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				results, err := e.Search("keyword")
				if err != nil {
					t.Errorf("Search failed: %v", err)
					errorsFound++
					return
				}
				if len(results) != 100 {
					t.Errorf("expected 100 results, got %d", len(results))
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrent_AddAndSearch(t *testing.T) {
	e := NewEngine()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			e.AddDocument(fmt.Sprintf("adddoc%d", i), fmt.Sprintf("data test %d", i))
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			e.Search("test")
		}
	}()

	wg.Wait()
}

func TestErrors_Values(t *testing.T) {
	if ErrEmptyQuery == nil {
		t.Error("ErrEmptyQuery should not be nil")
	}
	if ErrEmptyDocument == nil {
		t.Error("ErrEmptyDocument should not be nil")
	}
	if ErrEmptyDocID == nil {
		t.Error("ErrEmptyDocID should not be nil")
	}
	if ErrDuplicateDocID == nil {
		t.Error("ErrDuplicateDocID should not be nil")
	}
	if ErrDocNotFound == nil {
		t.Error("ErrDocNotFound should not be nil")
	}
	if ErrNilTokenizer == nil {
		t.Error("ErrNilTokenizer should not be nil")
	}
	if ErrTokenizerExists == nil {
		t.Error("ErrTokenizerExists should not be nil")
	}
	if ErrEmptyPhrase == nil {
		t.Error("ErrEmptyPhrase should not be nil")
	}
	if ErrPhraseTooShort == nil {
		t.Error("ErrPhraseTooShort should not be nil")
	}
}

func TestDocumentCount_Empty(t *testing.T) {
	e := NewEngine()
	if e.DocumentCount() != 0 {
		t.Errorf("expected 0, got %d", e.DocumentCount())
	}
}

func TestSearch_MultipleTerms(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "apple banana cherry")
	e.AddDocument("doc2", "apple date elderberry")
	e.AddDocument("doc3", "cherry date apple")

	results, err := e.Search("apple cherry")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results (all docs have apple or cherry), got %d", len(results))
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf("results should be sorted descending by score")
		}
	}

	hasBoth := 0
	for _, r := range results {
		if r.DocID == "doc1" || r.DocID == "doc3" {
			hasBoth++
		}
	}
	if hasBoth != 2 {
		t.Errorf("expected 2 docs with both apple and cherry, got %d", hasBoth)
	}
}

func TestSearch_ScoreConsistency(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world hello")

	results1, err := e.Search("hello")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	results2, err := e.Search("hello")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results1) != len(results2) {
		t.Fatal("search results count should be deterministic")
	}
	if results1[0].Score != results2[0].Score {
		t.Errorf("search score should be deterministic: %f vs %f", results1[0].Score, results2[0].Score)
	}
}

func TestSearchPhrase_EmptyEngine(t *testing.T) {
	e := NewEngine()

	results, err := e.SearchPhrase("hello world")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results from empty engine, got %d", len(results))
	}
}

func TestSearchPhrase_PunctuationInPhrase(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "Hello, World! This is a test.")

	results, err := e.SearchPhrase("hello world")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchPhrase_CaseInsensitive(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "Machine Learning is FUN")

	results, err := e.SearchPhrase("machine learning")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for case-insensitive phrase, got %d", len(results))
	}
}

func TestNewInvertedIndex(t *testing.T) {
	ii := NewInvertedIndex()
	if ii == nil {
		t.Fatal("NewInvertedIndex returned nil")
	}
	if ii.GetTermCount() != 0 {
		t.Errorf("expected 0 terms, got %d", ii.GetTermCount())
	}
}

func TestInvertedIndex_GetNonExistent(t *testing.T) {
	ii := NewInvertedIndex()
	pl, exists := ii.GetPostingList("nonexistent")
	if exists {
		t.Error("should not have posting list for non-existent term")
	}
	if pl != nil {
		t.Error("posting list should be nil for non-existent term")
	}
}

func TestSearchPhrase_DuplicateTerms(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "go go go go")

	results, err := e.SearchPhrase("go go")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result for duplicate term phrase, got %d", len(results))
	}
}

func TestSearchPhrase_DuplicateTermsNoMatch(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "go is fun")

	results, err := e.SearchPhrase("go go")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestAddDocument_ContentOnlyPunctuation(t *testing.T) {
	e := NewEngine()

	err := e.AddDocument("doc1", ",,,!!!???")
	if err != ErrEmptyDocument {
		t.Errorf("expected ErrEmptyDocument for punctuation-only content, got %v", err)
	}
}

func TestDefaultTokenizer_SpecialCharacters(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("test@#$%^&*()123")
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d: %v", len(tokens), tokens)
	}
	if tokens[0] != "test" {
		t.Errorf("expected 'test', got '%s'", tokens[0])
	}
	if tokens[1] != "123" {
		t.Errorf("expected '123', got '%s'", tokens[1])
	}
}

func TestDefaultTokenizer_Unicode(t *testing.T) {
	tokenizer := NewDefaultTokenizer()

	tokens := tokenizer.Tokenize("café résumé naïve")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestSearchPhrase_WithCustomTokenizer(t *testing.T) {
	e := NewEngine()
	e.RegisterTokenizer("ws", &WhitespaceOnlyTokenizer{})

	e.AddDocumentWithLanguage("doc1", "hello-world foo-bar", "ws")

	results, err := e.SearchPhraseWithLanguage("hello-world foo-bar", "ws")
	if err != nil {
		t.Fatalf("SearchPhraseWithLanguage failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with custom tokenizer, got %d", len(results))
	}
}

func TestRegisterTokenizer_CaseInsensitive(t *testing.T) {
	e := NewEngine()

	err := e.RegisterTokenizer("ZH-CN", &MockChineseTokenizer{})
	if err != nil {
		t.Fatalf("RegisterTokenizer failed: %v", err)
	}

	err = e.RegisterTokenizer("zh-cn", &WhitespaceOnlyTokenizer{})
	if err != ErrTokenizerExists {
		t.Errorf("expected ErrTokenizerExists for case-insensitive match, got %v", err)
	}
}

func TestSearch_OnlyPunctuationQuery(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world")

	results, err := e.Search(",,,!!!")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for punctuation-only query, got %d", len(results))
	}
}

func TestSearchPhrase_LongPhrase(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "a b c d e f g h i j")
	e.AddDocument("doc2", "a b c d e x f g h i")

	results, err := e.SearchPhrase("b c d e f g h i")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for long phrase, got %d", len(results))
	}
	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1, got %s", results[0].DocID)
	}
}

func TestSearch_ScoreAllPositive(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "apple banana")
	e.AddDocument("doc2", "banana cherry")
	e.AddDocument("doc3", "cherry apple")

	results, err := e.Search("apple banana cherry")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	for _, r := range results {
		if r.Score <= 0 {
			t.Errorf("score should be positive, got %f for doc %s", r.Score, r.DocID)
		}
	}
}

func TestDeleteDocument_SearchAfterDelete(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world")
	e.AddDocument("doc2", "hello there")

	e.DeleteDocument("doc1")

	results, err := e.Search("hello")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result after deletion, got %d", len(results))
	}
	if results[0].DocID != "doc2" {
		t.Errorf("expected only doc2 in results, got %s", results[0].DocID)
	}
}

func TestSearch_SortedUnique(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "test test test")

	results, err := e.Search("test")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	docIDs := make(map[string]bool)
	for _, r := range results {
		if docIDs[r.DocID] {
			t.Errorf("duplicate DocID in results: %s", r.DocID)
		}
		docIDs[r.DocID] = true
	}
}

func TestConcurrent_PhraseSearch(t *testing.T) {
	e := NewEngine()
	for i := 0; i < 50; i++ {
		e.AddDocument(fmt.Sprintf("doc%d", i), fmt.Sprintf("machine learning is great %d", i))
	}

	var wg sync.WaitGroup
	for g := 0; g < 20; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				results, err := e.SearchPhrase("machine learning")
				if err != nil {
					t.Errorf("SearchPhrase failed: %v", err)
					return
				}
				if len(results) != 50 {
					t.Errorf("expected 50 results, got %d", len(results))
				}
			}
		}()
	}
	wg.Wait()
}

func TestSearch_ManyTermsQuery(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "a b c d e f g h i j k l m n o p")

	results, err := e.Search("a c e g i k m o")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchPhrase_SameDocMultipleOccurrences(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "hello world and hello again hello world")

	results, err := e.SearchPhrase("hello world")
	if err != nil {
		t.Fatalf("SearchPhrase failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].DocID != "doc1" {
		t.Errorf("expected doc1, got %s", results[0].DocID)
	}
}

func TestGetTokenizer_FallbackToDefault(t *testing.T) {
	e := NewEngine()

	tok := e.getTokenizer("nonexistent_lang")
	if tok == nil {
		t.Error("should fall back to default tokenizer, got nil")
	}

	tok2 := e.getTokenizer("")
	if tok2 == nil {
		t.Error("should fall back to default tokenizer for empty lang, got nil")
	}
}

func TestDocument_Structure(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "one two three")

	doc, _ := e.GetDocument("doc1")
	if doc.ID != "doc1" {
		t.Errorf("expected ID 'doc1', got '%s'", doc.ID)
	}
	if doc.Content != "one two three" {
		t.Errorf("expected content preserved, got '%s'", doc.Content)
	}
	if doc.Length != 3 {
		t.Errorf("expected length 3, got %d", doc.Length)
	}
}

func TestSearchResult_Structure(t *testing.T) {
	e := NewEngine()
	e.AddDocument("doc1", "sample text")

	results, _ := e.Search("sample")
	if len(results) != 1 {
		t.Fatalf("expected 1 result")
	}

	if results[0].DocID != "doc1" {
		t.Errorf("DocID mismatch")
	}
	if results[0].Content != "sample text" {
		t.Errorf("Content mismatch")
	}
	if results[0].Score <= 0 {
		t.Errorf("Score should be positive")
	}
}

func TestTermPosting_Structure(t *testing.T) {
	ii := NewInvertedIndex()
	ii.AddTerm("test", "doc1", 0)
	ii.AddTerm("test", "doc1", 5)

	pl, _ := ii.GetPostingList("test")
	if len(pl.Postings) != 1 {
		t.Fatalf("expected 1 posting")
	}

	p := pl.Postings[0]
	if p.DocID != "doc1" {
		t.Errorf("expected docID 'doc1', got '%s'", p.DocID)
	}
	if p.Frequency != 2 {
		t.Errorf("expected frequency 2, got %d", p.Frequency)
	}
	if len(p.Positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(p.Positions))
	}

	sort.Ints(p.Positions)
	if p.Positions[0] != 0 || p.Positions[1] != 5 {
		t.Errorf("expected positions [0,5], got %v", p.Positions)
	}
}

func TestConcurrent_AddDocument_SameDocID(t *testing.T) {
	e := NewEngine()

	var wg sync.WaitGroup
	numGoroutines := 20
	successCount := int64(0)
	duplicateCount := int64(0)
	var mu sync.Mutex

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			content := fmt.Sprintf("content from goroutine %d", id)
			err := e.AddDocument("shared_doc_id", content)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if err == ErrDuplicateDocID {
				duplicateCount++
			}
		}(g)
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful AddDocument, got %d", successCount)
	}

	expectedDuplicates := int64(numGoroutines) - 1
	if duplicateCount != expectedDuplicates {
		t.Errorf("expected %d ErrDuplicateDocID errors, got %d", expectedDuplicates, duplicateCount)
	}

	if e.DocumentCount() != 1 {
		t.Errorf("expected exactly 1 document in engine, got %d", e.DocumentCount())
	}
}

func TestInvertedIndex_RemoveDocument_UniqueTerms(t *testing.T) {
	ii := NewInvertedIndex()

	ii.AddTerm("alpha", "doc1", 0)
	ii.AddTerm("beta", "doc1", 1)
	ii.AddTerm("gamma", "doc1", 2)

	if ii.GetTermCount() != 3 {
		t.Fatalf("expected 3 terms before removal, got %d", ii.GetTermCount())
	}

	terms := map[string]struct{}{
		"alpha": {},
		"beta":  {},
		"gamma": {},
	}
	ii.RemoveDocument("doc1", terms)

	if ii.GetTermCount() != 0 {
		t.Errorf("expected 0 terms after removing unique terms, got %d", ii.GetTermCount())
	}

	if ii.HasTerm("alpha") {
		t.Error("unique term 'alpha' should be removed from index")
	}
	if ii.HasTerm("beta") {
		t.Error("unique term 'beta' should be removed from index")
	}
	if ii.HasTerm("gamma") {
		t.Error("unique term 'gamma' should be removed from index")
	}
}

func TestInvertedIndex_RemoveDocument_SharedTerms(t *testing.T) {
	ii := NewInvertedIndex()

	ii.AddTerm("shared", "doc1", 0)
	ii.AddTerm("shared", "doc2", 0)
	ii.AddTerm("shared", "doc3", 0)
	ii.AddTerm("only_doc1", "doc1", 1)
	ii.AddTerm("only_doc2", "doc2", 1)

	pl, _ := ii.GetPostingList("shared")
	if len(pl.Postings) != 3 {
		t.Fatalf("expected 3 postings for 'shared' before removal, got %d", len(pl.Postings))
	}

	termsDoc1 := map[string]struct{}{
		"shared":   {},
		"only_doc1": {},
	}
	ii.RemoveDocument("doc1", termsDoc1)

	if ii.HasTerm("only_doc1") {
		t.Error("unique term 'only_doc1' should be removed from index")
	}
	if !ii.HasTerm("only_doc2") {
		t.Error("term 'only_doc2' belonging to doc2 should still exist")
	}

	pl, exists := ii.GetPostingList("shared")
	if !exists {
		t.Fatal("shared term 'shared' should still exist in index")
	}
	if len(pl.Postings) != 2 {
		t.Errorf("expected 2 postings for 'shared' after removing doc1, got %d", len(pl.Postings))
	}

	foundDoc2 := false
	foundDoc3 := false
	for _, posting := range pl.Postings {
		if posting.DocID == "doc2" {
			foundDoc2 = true
		}
		if posting.DocID == "doc3" {
			foundDoc3 = true
		}
	}
	if !foundDoc2 {
		t.Error("doc2 should still be in posting list for 'shared'")
	}
	if !foundDoc3 {
		t.Error("doc3 should still be in posting list for 'shared'")
	}
}

func TestDeleteDocument_InvertedIndexCleanup(t *testing.T) {
	e := NewEngine()

	e.AddDocument("doc1", "xray alpha sharedword")
	e.AddDocument("doc2", "sharedword gamma")

	if e.invertedIndex.GetTermCount() != 4 {
		t.Fatalf("expected 4 unique terms in index, got %d", e.invertedIndex.GetTermCount())
	}

	sharedPl, _ := e.invertedIndex.GetPostingList("sharedword")
	if len(sharedPl.Postings) != 2 {
		t.Fatalf("expected 2 postings for 'sharedword' before delete, got %d", len(sharedPl.Postings))
	}

	err := e.DeleteDocument("doc1")
	if err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}

	if e.invertedIndex.HasTerm("xray") {
		t.Error("unique term 'xray' should be removed from inverted index after delete")
	}
	if e.invertedIndex.HasTerm("alpha") {
		t.Error("unique term 'alpha' should be removed from inverted index after delete")
	}
	if !e.invertedIndex.HasTerm("gamma") {
		t.Error("term 'gamma' from doc2 should still exist")
	}

	sharedPlAfter, exists := e.invertedIndex.GetPostingList("sharedword")
	if !exists {
		t.Fatal("'sharedword' should still exist since doc2 still contains it")
	}
	if len(sharedPlAfter.Postings) != 1 {
		t.Errorf("expected 1 posting for 'sharedword' after deleting doc1, got %d", len(sharedPlAfter.Postings))
	}
	if sharedPlAfter.Postings[0].DocID != "doc2" {
		t.Errorf("expected remaining posting to be for doc2, got '%s'", sharedPlAfter.Postings[0].DocID)
	}

	if e.invertedIndex.GetTermCount() != 2 {
		t.Errorf("expected 2 terms remaining (sharedword, gamma), got %d", e.invertedIndex.GetTermCount())
	}
}

func TestConcurrent_AddDocumentWithLanguage_SameDocID(t *testing.T) {
	e := NewEngine()
	err := e.RegisterTokenizer("ws", &WhitespaceOnlyTokenizer{})
	if err != nil {
		t.Fatalf("RegisterTokenizer failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 20
	successCount := int64(0)
	duplicateCount := int64(0)
	var mu sync.Mutex

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			content := fmt.Sprintf("custom-tokenizer-doc-from-goroutine-%d", id)
			err := e.AddDocumentWithLanguage("shared_custom_doc", content, "ws")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else if err == ErrDuplicateDocID {
				duplicateCount++
			}
		}(g)
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful AddDocumentWithLanguage, got %d", successCount)
	}

	expectedDuplicates := int64(numGoroutines) - 1
	if duplicateCount != expectedDuplicates {
		t.Errorf("expected %d ErrDuplicateDocID errors with custom tokenizer, got %d", expectedDuplicates, duplicateCount)
	}

	if e.DocumentCount() != 1 {
		t.Errorf("expected exactly 1 document in engine, got %d", e.DocumentCount())
	}

	doc, exists := e.GetDocument("shared_custom_doc")
	if !exists {
		t.Fatal("document should exist after concurrent insert with custom tokenizer")
	}
	if doc.Length < 1 {
		t.Errorf("expected non-zero document length, got %d", doc.Length)
	}
}
