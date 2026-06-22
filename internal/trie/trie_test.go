package trie

import (
	"sort"
	"sync"
	"testing"
)

func TestTrie_Insert(t *testing.T) {
	trie := NewTrie()

	err := trie.Insert("hello", "greeting")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trie.Size() != 1 {
		t.Errorf("expected size 1, got %d", trie.Size())
	}

	data, exists := trie.Search("hello")
	if !exists {
		t.Error("expected 'hello' to exist")
	}
	if data != "greeting" {
		t.Errorf("expected data 'greeting', got %v", data)
	}
}

func TestTrie_Insert_EmptyWord(t *testing.T) {
	trie := NewTrie()
	err := trie.Insert("", "data")
	if err != ErrEmptyWord {
		t.Errorf("expected ErrEmptyWord, got %v", err)
	}
}

func TestTrie_Insert_OverwriteData(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("hello", "value1")
	data1, _ := trie.Search("hello")
	if data1 != "value1" {
		t.Errorf("expected data 'value1', got %v", data1)
	}

	_ = trie.Insert("hello", "value2")

	data2, _ := trie.Search("hello")
	if data2 != "value2" {
		t.Errorf("expected data 'value2' after overwrite, got %v", data2)
	}

	if trie.Size() != 1 {
		t.Errorf("expected size 1 after overwrite, got %d", trie.Size())
	}
}

func TestTrie_Insert_NilData(t *testing.T) {
	trie := NewTrie()

	err := trie.Insert("test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, exists := trie.Search("test")
	if !exists {
		t.Error("expected 'test' to exist")
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
}

func TestTrie_Insert_PrefixWords(t *testing.T) {
	trie := NewTrie()

	words := []string{"a", "ab", "abc", "abcd"}
	for _, w := range words {
		_ = trie.Insert(w, w+"_data")
	}

	if trie.Size() != 4 {
		t.Errorf("expected size 4, got %d", trie.Size())
	}

	for _, w := range words {
		data, exists := trie.Search(w)
		if !exists {
			t.Errorf("expected %q to exist", w)
		}
		if data != w+"_data" {
			t.Errorf("expected data %q, got %v", w+"_data", data)
		}
	}
}

func TestTrie_Search(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("apple", "fruit")
	_ = trie.Insert("app", "abbreviation")

	tests := []struct {
		word     string
		expected bool
		data     interface{}
	}{
		{"apple", true, "fruit"},
		{"app", true, "abbreviation"},
		{"appl", false, nil},
		{"banana", false, nil},
		{"", false, nil},
	}

	for _, tt := range tests {
		data, exists := trie.Search(tt.word)
		if exists != tt.expected {
			t.Errorf("Search(%q): expected exists=%v, got %v", tt.word, tt.expected, exists)
		}
		if tt.expected && data != tt.data {
			t.Errorf("Search(%q): expected data=%v, got %v", tt.word, tt.data, data)
		}
	}
}

func TestTrie_Delete(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("hello", "data1")
	_ = trie.Insert("hell", "data2")
	_ = trie.Insert("help", "data3")

	err := trie.Delete("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trie.Size() != 2 {
		t.Errorf("expected size 2, got %d", trie.Size())
	}

	_, exists := trie.Search("hello")
	if exists {
		t.Error("expected 'hello' to be deleted")
	}

	_, exists = trie.Search("hell")
	if !exists {
		t.Error("expected 'hell' to still exist")
	}

	_, exists = trie.Search("help")
	if !exists {
		t.Error("expected 'help' to still exist")
	}
}

func TestTrie_Delete_EmptyWord(t *testing.T) {
	trie := NewTrie()
	err := trie.Delete("")
	if err != ErrEmptyWord {
		t.Errorf("expected ErrEmptyWord, got %v", err)
	}
}

func TestTrie_Delete_NotFound(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello", "data")

	err := trie.Delete("world")
	if err != ErrWordNotFound {
		t.Errorf("expected ErrWordNotFound, got %v", err)
	}
}

func TestTrie_Delete_NotEndNode(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello", "data")

	err := trie.Delete("hel")
	if err != ErrWordNotFound {
		t.Errorf("expected ErrWordNotFound for non-end node, got %v", err)
	}
}

func TestTrie_Delete_LeafCleanup(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("a", "1")
	_ = trie.Insert("ab", "2")
	_ = trie.Insert("abc", "3")

	_ = trie.Delete("abc")

	_, exists := trie.Search("ab")
	if !exists {
		t.Error("expected 'ab' to still exist after deleting 'abc'")
	}

	_ = trie.Delete("ab")

	_, exists = trie.Search("a")
	if !exists {
		t.Error("expected 'a' to still exist after deleting 'ab'")
	}

	_ = trie.Delete("a")

	if trie.Size() != 0 {
		t.Errorf("expected size 0 after all deletes, got %d", trie.Size())
	}
}

func TestTrie_PrefixMatch(t *testing.T) {
	trie := NewTrie()

	words := []string{"apple", "app", "application", "banana", "appreciate"}
	for i, w := range words {
		_ = trie.Insert(w, i)
	}

	results, err := trie.PrefixMatch("app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}

	wordSet := make(map[string]bool)
	for _, r := range results {
		wordSet[r.Word] = true
	}

	expected := []string{"app", "apple", "application", "appreciate"}
	for _, w := range expected {
		if !wordSet[w] {
			t.Errorf("expected result to contain %q", w)
		}
	}

	if !sort.SliceIsSorted(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	}) {
		t.Error("results not sorted lexicographically")
	}
}

func TestTrie_PrefixMatch_EmptyPrefix(t *testing.T) {
	trie := NewTrie()
	_, err := trie.PrefixMatch("")
	if err != ErrEmptyPrefix {
		t.Errorf("expected ErrEmptyPrefix, got %v", err)
	}
}

func TestTrie_PrefixMatch_NoMatch(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello", "data")

	results, err := trie.PrefixMatch("xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestTrie_PrefixMatchLimit(t *testing.T) {
	trie := NewTrie()

	words := []string{"a1", "a2", "a3", "a4", "a5"}
	for i, w := range words {
		_ = trie.Insert(w, i)
	}

	results, err := trie.PrefixMatchLimit("a", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestTrie_PrefixMatchLimit_InvalidLimit(t *testing.T) {
	trie := NewTrie()
	_, err := trie.PrefixMatchLimit("a", -1)
	if err != ErrInvalidMaxResult {
		t.Errorf("expected ErrInvalidMaxResult, got %v", err)
	}
}

func TestTrie_PrefixMatchLimit_LimitLargerThanResults(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("apple", "data")

	results, err := trie.PrefixMatchLimit("a", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestTrie_PrefixMatchLimit_ZeroLimitReturnsAll(t *testing.T) {
	trie := NewTrie()

	words := []string{"a1", "a2", "a3"}
	for _, w := range words {
		_ = trie.Insert(w, "data")
	}

	results, err := trie.PrefixMatchLimit("a", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results (all), got %d", len(results))
	}
}

func TestTrie_WildcardSearch_Dot(t *testing.T) {
	trie := NewTrie()

	words := []string{"cat", "car", "can", "bat", "rat"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	results, err := trie.WildcardSearch("c.t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for 'c.t', got %d", len(results))
	}
	if len(results) > 0 && results[0].Word != "cat" {
		t.Errorf("expected 'cat', got %q", results[0].Word)
	}

	results, err = trie.WildcardSearch(".at")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results for '.at', got %d", len(results))
	}

	expected := map[string]bool{"cat": true, "bat": true, "rat": true}
	for _, r := range results {
		if !expected[r.Word] {
			t.Errorf("unexpected result %q", r.Word)
		}
	}
}

func TestTrie_WildcardSearch_Star(t *testing.T) {
	trie := NewTrie()

	words := []string{"app", "apple", "application", "appreciate", "banana"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	results, err := trie.WildcardSearch("app*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results for 'app*', got %d", len(results))
	}

	results, err = trie.WildcardSearch("*e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]bool{"apple": true, "appreciate": true}
	for _, r := range results {
		if !expected[r.Word] {
			t.Errorf("unexpected result %q for '*e'", r.Word)
		}
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results for '*e', got %d", len(results))
	}
}

func TestTrie_WildcardSearch_Combined(t *testing.T) {
	trie := NewTrie()

	words := []string{"abcde", "abxde", "abxyz", "a123e", "test"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	results, err := trie.WildcardSearch("a.*e")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results for 'a.*e', got %d", len(results))
	}

	expected := map[string]bool{"abcde": true, "abxde": true, "a123e": true}
	for _, r := range results {
		if !expected[r.Word] {
			t.Errorf("unexpected result %q", r.Word)
		}
	}
}

func TestTrie_WildcardSearch_StarMatchesEmpty(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("app", "data")

	results, err := trie.WildcardSearch("app*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result (star matches empty), got %d", len(results))
	}
	if len(results) > 0 && results[0].Word != "app" {
		t.Errorf("expected 'app', got %q", results[0].Word)
	}
}

func TestTrie_WildcardSearch_EmptyPattern(t *testing.T) {
	trie := NewTrie()
	_, err := trie.WildcardSearch("")
	if err != ErrEmptyPattern {
		t.Errorf("expected ErrEmptyPattern, got %v", err)
	}
}

func TestTrie_WildcardSearch_NoMatch(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello", "data")

	results, err := trie.WildcardSearch("xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestTrie_WildcardSearch_SingleStar(t *testing.T) {
	trie := NewTrie()

	words := []string{"a", "ab", "abc", "abcd"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	results, err := trie.WildcardSearch("*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results for '*', got %d", len(results))
	}
}

func TestTrie_WildcardSearch_ConsecutiveStars(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("hello", "data")
	_ = trie.Insert("world", "data")

	results, err := trie.WildcardSearch("h**o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result for 'h**o', got %d", len(results))
	}
	if len(results) > 0 && results[0].Word != "hello" {
		t.Errorf("expected 'hello', got %q", results[0].Word)
	}
}

func TestTrie_LongestMatch(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("中", "zhōng")
	_ = trie.Insert("中国", "Zhōngguó")
	_ = trie.Insert("中国人", "Zhōngguórén")
	_ = trie.Insert("中国人民", "Zhōngguórénmín")

	result, err := trie.LongestMatch("中国人民银行")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Word != "中国人民" {
		t.Errorf("expected longest match '中国人民', got %q", result.Word)
	}
	if result.Data != "Zhōngguórénmín" {
		t.Errorf("expected data 'Zhōngguórénmín', got %v", result.Data)
	}
}

func TestTrie_LongestMatch_EmptyQuery(t *testing.T) {
	trie := NewTrie()
	_, err := trie.LongestMatch("")
	if err != ErrEmptyQuery {
		t.Errorf("expected ErrEmptyQuery, got %v", err)
	}
}

func TestTrie_LongestMatch_NoMatch(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello", "data")

	_, err := trie.LongestMatch("world")
	if err != ErrWordNotFound {
		t.Errorf("expected ErrWordNotFound, got %v", err)
	}
}

func TestTrie_LongestMatch_ExactMatch(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("app", "short")
	_ = trie.Insert("apple", "medium")
	_ = trie.Insert("application", "long")

	result, err := trie.LongestMatch("apple")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Word != "apple" {
		t.Errorf("expected exact match 'apple', got %q", result.Word)
	}
}

func TestTrie_LongestMatch_PrefixOnly(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("app", "data")
	_ = trie.Insert("apple", "data")

	result, err := trie.LongestMatch("app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Word != "app" {
		t.Errorf("expected 'app', got %q", result.Word)
	}
}

func TestTrie_LongestMatch_NoLongerMatchAfterShorter(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("a", "1")
	_ = trie.Insert("ab", "2")
	_ = trie.Insert("abc", "3")

	result, err := trie.LongestMatch("abx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Word != "ab" {
		t.Errorf("expected longest match 'ab', got %q", result.Word)
	}
}

func TestTrie_Size(t *testing.T) {
	trie := NewTrie()

	if trie.Size() != 0 {
		t.Errorf("expected size 0, got %d", trie.Size())
	}

	_ = trie.Insert("a", "1")
	_ = trie.Insert("b", "2")
	_ = trie.Insert("c", "3")

	if trie.Size() != 3 {
		t.Errorf("expected size 3, got %d", trie.Size())
	}

	_ = trie.Delete("b")

	if trie.Size() != 2 {
		t.Errorf("expected size 2 after delete, got %d", trie.Size())
	}
}

func TestTrie_GetAllWords(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("banana", "3")
	_ = trie.Insert("apple", "1")
	_ = trie.Insert("cherry", "2")

	results := trie.GetAllWords()

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if !sort.SliceIsSorted(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	}) {
		t.Error("results not sorted lexicographically")
	}

	if results[0].Word != "apple" {
		t.Errorf("expected first 'apple', got %q", results[0].Word)
	}
	if results[1].Word != "banana" {
		t.Errorf("expected second 'banana', got %q", results[1].Word)
	}
	if results[2].Word != "cherry" {
		t.Errorf("expected third 'cherry', got %q", results[2].Word)
	}
}

func TestTrie_Unicode(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("你好", "hello")
	_ = trie.Insert("你好世界", "hello world")
	_ = trie.Insert("hello", "english")

	data, exists := trie.Search("你好")
	if !exists {
		t.Error("expected '你好' to exist")
	}
	if data != "hello" {
		t.Errorf("expected data 'hello', got %v", data)
	}

	results, err := trie.PrefixMatch("你好")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for '你好' prefix, got %d", len(results))
	}

	result, err := trie.LongestMatch("你好世界")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Word != "你好世界" {
		t.Errorf("expected '你好世界', got %q", result.Word)
	}
}

func TestTrie_MixedCase(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("Apple", "fruit")
	_ = trie.Insert("apple", "company")

	data1, exists1 := trie.Search("Apple")
	if !exists1 {
		t.Error("expected 'Apple' to exist")
	}
	if data1 != "fruit" {
		t.Errorf("expected data 'fruit', got %v", data1)
	}

	data2, exists2 := trie.Search("apple")
	if !exists2 {
		t.Error("expected 'apple' to exist")
	}
	if data2 != "company" {
		t.Errorf("expected data 'company', got %v", data2)
	}
}

func TestTrie_SpecialCharacters(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("user@example.com", "email")
	_ = trie.Insert("key=value", "pair")
	_ = trie.Insert("path/to/file", "filepath")

	data, exists := trie.Search("user@example.com")
	if !exists {
		t.Error("expected 'user@example.com' to exist")
	}
	if data != "email" {
		t.Errorf("expected data 'email', got %v", data)
	}

	results, err := trie.PrefixMatch("path/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestConcurrent_TrieInsert(t *testing.T) {
	trie := NewTrie()
	var wg sync.WaitGroup

	numGoroutines := 50
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			word := "word" + string(rune('a'+id%26)) + string(rune('0'+id/26))
			_ = trie.Insert(word, id)
		}(i)
	}

	wg.Wait()

	if trie.Size() == 0 {
		t.Error("expected some words to be inserted")
	}
}

func TestConcurrent_TrieInsertSameWord(t *testing.T) {
	trie := NewTrie()
	var wg sync.WaitGroup

	numGoroutines := 100
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = trie.Insert("test", "data")
		}()
	}

	wg.Wait()

	if trie.Size() != 1 {
		t.Errorf("expected size 1, got %d", trie.Size())
	}

	data, _ := trie.Search("test")
	if data != "data" {
		t.Errorf("expected data 'data', got %v", data)
	}
}

func TestConcurrent_TrieReadWrite(t *testing.T) {
	trie := NewTrie()
	var wg sync.WaitGroup

	words := []string{"apple", "app", "application", "apply", "appoint", "appreciate"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	numReaders := 50
	numWriters := 30

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			word := "new" + string(rune('a'+id%10))
			_ = trie.Insert(word, id)
		}(i)
	}

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = trie.PrefixMatch("app")
			_, _ = trie.Search("apple")
			_, _ = trie.WildcardSearch("app*")
			_, _ = trie.LongestMatch("application")
			_ = trie.Size()
			_ = trie.GetAllWords()
		}()
	}

	wg.Wait()
}

func TestTrie_SearchResultDataIntegrity(t *testing.T) {
	type CustomData struct {
		ID   int
		Name string
	}

	trie := NewTrie()

	expected := CustomData{ID: 42, Name: "test"}
	_ = trie.Insert("key", expected)

	data, exists := trie.Search("key")
	if !exists {
		t.Fatal("expected 'key' to exist")
	}

	actual, ok := data.(CustomData)
	if !ok {
		t.Fatalf("expected CustomData type, got %T", data)
	}

	if actual.ID != expected.ID || actual.Name != expected.Name {
		t.Errorf("expected %+v, got %+v", expected, actual)
	}
}

func TestTrie_Delete_ClearsData(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("test", "sensitive_data")
	_ = trie.Delete("test")

	_ = trie.Insert("test", "new_data")

	data, _ := trie.Search("test")
	if data != "new_data" {
		t.Errorf("expected 'new_data', got %v", data)
	}
}

func TestTrie_WildcardSearch_SortedResults(t *testing.T) {
	trie := NewTrie()

	words := []string{"zebra", "apple", "banana", "cherry", "date"}
	for _, w := range words {
		_ = trie.Insert(w, w)
	}

	results, err := trie.WildcardSearch("*")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !sort.SliceIsSorted(results, func(i, j int) bool {
		return results[i].Word < results[j].Word
	}) {
		t.Error("wildcard results not sorted lexicographically")
	}
}

func TestTrie_LongestMatch_SegmentationExample(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("上海", "city")
	_ = trie.Insert("上海市", "municipality")
	_ = trie.Insert("上海自来水", "water_company")
	_ = trie.Insert("来自", "from")
	_ = trie.Insert("自来水", "tap_water")

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

	expected := []string{"上海自来水", "来自"}
	if len(segments) != len(expected) {
		t.Errorf("expected %d segments, got %d: %v", len(expected), len(segments), segments)
	} else {
		for i, seg := range segments {
			if seg != expected[i] {
				t.Errorf("segment %d: expected %q, got %q", i, expected[i], seg)
			}
		}
	}
}
