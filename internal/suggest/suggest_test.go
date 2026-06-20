package suggest

import (
	"sort"
	"sync"
	"testing"
)

func TestTrie_Insert(t *testing.T) {
	trie := NewTrie()

	err := trie.Insert("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trie.Size() != 1 {
		t.Errorf("expected size 1, got %d", trie.Size())
	}

	exists, freq := trie.Search("hello")
	if !exists {
		t.Error("expected 'hello' to exist")
	}
	if freq != 1 {
		t.Errorf("expected freq 1, got %d", freq)
	}
}

func TestTrie_Insert_EmptyWord(t *testing.T) {
	trie := NewTrie()
	err := trie.Insert("")
	if err != ErrEmptyWord {
		t.Errorf("expected ErrEmptyWord, got %v", err)
	}
}

func TestTrie_Insert_DuplicateIncrementsFreq(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("hello")
	_ = trie.Insert("hello")
	_ = trie.Insert("hello")

	if trie.Size() != 1 {
		t.Errorf("expected size 1, got %d", trie.Size())
	}

	exists, freq := trie.Search("hello")
	if !exists {
		t.Error("expected 'hello' to exist")
	}
	if freq != 3 {
		t.Errorf("expected freq 3, got %d", freq)
	}
}

func TestTrie_InsertWithFreq(t *testing.T) {
	trie := NewTrie()

	err := trie.InsertWithFreq("hello", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, freq := trie.Search("hello")
	if !exists {
		t.Error("expected 'hello' to exist")
	}
	if freq != 10 {
		t.Errorf("expected freq 10, got %d", freq)
	}
}

func TestTrie_InsertWithFreq_NegativeFreq(t *testing.T) {
	trie := NewTrie()

	err := trie.InsertWithFreq("hello", -5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, freq := trie.Search("hello")
	if freq != 0 {
		t.Errorf("expected freq 0 for negative input, got %d", freq)
	}
}

func TestTrie_InsertWithFreq_Overwrite(t *testing.T) {
	trie := NewTrie()

	_ = trie.InsertWithFreq("hello", 5)
	_ = trie.InsertWithFreq("hello", 20)

	_, freq := trie.Search("hello")
	if freq != 20 {
		t.Errorf("expected freq 20 after overwrite, got %d", freq)
	}
}

func TestTrie_Delete(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("hello")
	_ = trie.Insert("hell")
	_ = trie.Insert("help")

	err := trie.Delete("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if trie.Size() != 2 {
		t.Errorf("expected size 2, got %d", trie.Size())
	}

	exists, _ := trie.Search("hello")
	if exists {
		t.Error("expected 'hello' to be deleted")
	}

	exists, _ = trie.Search("hell")
	if !exists {
		t.Error("expected 'hell' to still exist")
	}

	exists, _ = trie.Search("help")
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
	_ = trie.Insert("hello")

	err := trie.Delete("world")
	if err != ErrWordNotFound {
		t.Errorf("expected ErrWordNotFound, got %v", err)
	}
}

func TestTrie_Delete_NotEndNode(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello")

	err := trie.Delete("hel")
	if err != ErrWordNotFound {
		t.Errorf("expected ErrWordNotFound for non-end node, got %v", err)
	}
}

func TestTrie_Search(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("apple")
	_ = trie.Insert("app")

	tests := []struct {
		word     string
		expected bool
		freq     int
	}{
		{"apple", true, 1},
		{"app", true, 1},
		{"appl", false, 0},
		{"banana", false, 0},
		{"", false, 0},
	}

	for _, tt := range tests {
		exists, freq := trie.Search(tt.word)
		if exists != tt.expected {
			t.Errorf("Search(%q): expected exists=%v, got %v", tt.word, tt.expected, exists)
		}
		if freq != tt.freq {
			t.Errorf("Search(%q): expected freq=%d, got %d", tt.word, tt.freq, freq)
		}
	}
}

func TestTrie_StartsWith(t *testing.T) {
	trie := NewTrie()

	words := []string{"apple", "app", "application", "banana", "appreciate"}
	for _, w := range words {
		_ = trie.Insert(w)
	}

	results, err := trie.StartsWith("app")
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
}

func TestTrie_StartsWith_EmptyPrefix(t *testing.T) {
	trie := NewTrie()
	_, err := trie.StartsWith("")
	if err != ErrEmptyPrefix {
		t.Errorf("expected ErrEmptyPrefix, got %v", err)
	}
}

func TestTrie_StartsWith_NoMatch(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("hello")

	results, err := trie.StartsWith("xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestTrie_StartsWith_SortedByFreq(t *testing.T) {
	trie := NewTrie()

	_ = trie.InsertWithFreq("apple", 10)
	_ = trie.InsertWithFreq("app", 5)
	_ = trie.InsertWithFreq("application", 20)

	results, err := trie.StartsWith("app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Word != "application" || results[0].Frequency != 20 {
		t.Errorf("expected first to be 'application' with freq 20, got %q with freq %d", results[0].Word, results[0].Frequency)
	}

	if results[1].Word != "apple" || results[1].Frequency != 10 {
		t.Errorf("expected second to be 'apple' with freq 10, got %q with freq %d", results[1].Word, results[1].Frequency)
	}

	if results[2].Word != "app" || results[2].Frequency != 5 {
		t.Errorf("expected third to be 'app' with freq 5, got %q with freq %d", results[2].Word, results[2].Frequency)
	}
}

func TestTrie_StartsWith_SameFreqLexOrder(t *testing.T) {
	trie := NewTrie()

	_ = trie.InsertWithFreq("cat", 5)
	_ = trie.InsertWithFreq("car", 5)
	_ = trie.InsertWithFreq("can", 5)

	results, err := trie.StartsWith("ca")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if results[0].Word != "can" {
		t.Errorf("expected first to be 'can', got %q", results[0].Word)
	}
	if results[1].Word != "car" {
		t.Errorf("expected second to be 'car', got %q", results[1].Word)
	}
	if results[2].Word != "cat" {
		t.Errorf("expected third to be 'cat', got %q", results[2].Word)
	}
}

func TestTrie_StartsWithLimit(t *testing.T) {
	trie := NewTrie()

	words := []string{"a1", "a2", "a3", "a4", "a5"}
	for i, w := range words {
		_ = trie.InsertWithFreq(w, 5-i)
	}

	results, err := trie.StartsWithLimit("a", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestTrie_StartsWithLimit_InvalidLimit(t *testing.T) {
	trie := NewTrie()
	_, err := trie.StartsWithLimit("a", 0)
	if err != ErrInvalidMaxResult {
		t.Errorf("expected ErrInvalidMaxResult, got %v", err)
	}
}

func TestTrie_StartsWithLimit_LimitLargerThanResults(t *testing.T) {
	trie := NewTrie()
	_ = trie.Insert("apple")

	results, err := trie.StartsWithLimit("a", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestTrie_Size(t *testing.T) {
	trie := NewTrie()

	if trie.Size() != 0 {
		t.Errorf("expected size 0, got %d", trie.Size())
	}

	_ = trie.Insert("a")
	_ = trie.Insert("b")
	_ = trie.Insert("c")

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

	_ = trie.InsertWithFreq("banana", 3)
	_ = trie.InsertWithFreq("apple", 1)
	_ = trie.InsertWithFreq("cherry", 2)

	results := trie.GetAllWords()

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	if !sort.SliceIsSorted(results, func(i, j int) bool {
		if results[i].Frequency != results[j].Frequency {
			return results[i].Frequency > results[j].Frequency
		}
		return results[i].Word < results[j].Word
	}) {
		t.Error("results not sorted correctly")
	}
}

func TestTrie_Unicode(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("你好")
	_ = trie.Insert("你好世界")
	_ = trie.Insert("hello")

	exists, _ := trie.Search("你好")
	if !exists {
		t.Error("expected '你好' to exist")
	}

	results, err := trie.StartsWith("你好")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results for '你好' prefix, got %d", len(results))
	}
}

func TestEditDistance(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"kitten", "sitting", 3},
		{"flaw", "lawn", 2},
		{"gumbo", "gambol", 2},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"abc", "axc", 1},
		{"abc", "xbc", 1},
		{"intention", "execution", 5},
		{"saturday", "sunday", 3},
		{"你好", "你好吗", 1},
	}

	for _, tt := range tests {
		result := EditDistance(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("EditDistance(%q, %q): expected %d, got %d", tt.a, tt.b, tt.expected, result)
		}

		result2 := EditDistance(tt.b, tt.a)
		if result2 != tt.expected {
			t.Errorf("EditDistance(%q, %q) (reversed): expected %d, got %d", tt.b, tt.a, tt.expected, result2)
		}
	}
}

func TestSearchHistory_Add(t *testing.T) {
	h := NewSearchHistory()

	err := h.Add("user1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count, _ := h.Count("user1")
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestSearchHistory_Add_EmptyUserID(t *testing.T) {
	h := NewSearchHistory()
	err := h.Add("", "hello")
	if err != ErrEmptyUserID {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestSearchHistory_Add_EmptyWord(t *testing.T) {
	h := NewSearchHistory()
	err := h.Add("user1", "")
	if err != ErrEmptyWord {
		t.Errorf("expected ErrEmptyWord, got %v", err)
	}
}

func TestSearchHistory_Add_DuplicateMovesToFront(t *testing.T) {
	h := NewSearchHistory()

	_ = h.Add("user1", "first")
	_ = h.Add("user1", "second")
	_ = h.Add("user1", "third")

	_ = h.Add("user1", "first")

	records, err := h.GetRecent("user1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].Word != "first" {
		t.Errorf("expected first record to be 'first', got %q", records[0].Word)
	}
	if records[1].Word != "third" {
		t.Errorf("expected second record to be 'third', got %q", records[1].Word)
	}
	if records[2].Word != "second" {
		t.Errorf("expected third record to be 'second', got %q", records[2].Word)
	}
}

func TestSearchHistory_Add_MaxSizeLimit(t *testing.T) {
	h, _ := NewSearchHistoryWithMaxSize(3)

	words := []string{"a", "b", "c", "d", "e"}
	for _, w := range words {
		_ = h.Add("user1", w)
	}

	count, _ := h.Count("user1")
	if count != 3 {
		t.Errorf("expected count 3 (max size), got %d", count)
	}

	records, _ := h.GetRecent("user1", 10)
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	if records[0].Word != "e" {
		t.Errorf("expected first to be 'e', got %q", records[0].Word)
	}
	if records[2].Word != "c" {
		t.Errorf("expected last to be 'c', got %q", records[2].Word)
	}
}

func TestSearchHistory_GetRecent(t *testing.T) {
	h := NewSearchHistory()

	words := []string{"first", "second", "third"}
	for _, w := range words {
		_ = h.Add("user1", w)
	}

	records, err := h.GetRecent("user1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if records[0].Word != "third" {
		t.Errorf("expected first to be 'third', got %q", records[0].Word)
	}
	if records[1].Word != "second" {
		t.Errorf("expected second to be 'second', got %q", records[1].Word)
	}
}

func TestSearchHistory_GetRecent_EmptyUserID(t *testing.T) {
	h := NewSearchHistory()
	_, err := h.GetRecent("", 5)
	if err != ErrEmptyUserID {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestSearchHistory_GetRecent_InvalidN(t *testing.T) {
	h := NewSearchHistory()
	_, err := h.GetRecent("user1", 0)
	if err != ErrInvalidHistoryN {
		t.Errorf("expected ErrInvalidHistoryN, got %v", err)
	}
}

func TestSearchHistory_GetRecent_NoUser(t *testing.T) {
	h := NewSearchHistory()
	records, err := h.GetRecent("nonexistent", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for nonexistent user, got %d", len(records))
	}
}

func TestSearchHistory_GetRecent_NLargerThanCount(t *testing.T) {
	h := NewSearchHistory()
	_ = h.Add("user1", "hello")

	records, err := h.GetRecent("user1", 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestSearchHistory_Clear(t *testing.T) {
	h := NewSearchHistory()

	_ = h.Add("user1", "hello")
	_ = h.Add("user1", "world")
	_ = h.Add("user2", "test")

	err := h.Clear("user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	count1, _ := h.Count("user1")
	if count1 != 0 {
		t.Errorf("expected user1 count 0 after clear, got %d", count1)
	}

	count2, _ := h.Count("user2")
	if count2 != 1 {
		t.Errorf("expected user2 count 1 (unchanged), got %d", count2)
	}
}

func TestSearchHistory_Clear_EmptyUserID(t *testing.T) {
	h := NewSearchHistory()
	err := h.Clear("")
	if err != ErrEmptyUserID {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestSearchHistory_Clear_NonexistentUser(t *testing.T) {
	h := NewSearchHistory()
	err := h.Clear("nonexistent")
	if err != nil {
		t.Errorf("unexpected error clearing nonexistent user: %v", err)
	}
}

func TestSearchHistory_Count(t *testing.T) {
	h := NewSearchHistory()

	count, _ := h.Count("user1")
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}

	_ = h.Add("user1", "hello")
	_ = h.Add("user1", "world")

	count, _ = h.Count("user1")
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}
}

func TestSearchHistory_Count_EmptyUserID(t *testing.T) {
	h := NewSearchHistory()
	_, err := h.Count("")
	if err != ErrEmptyUserID {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestSearchHistory_MultipleUsers(t *testing.T) {
	h := NewSearchHistory()

	_ = h.Add("user1", "hello")
	_ = h.Add("user2", "world")
	_ = h.Add("user1", "foo")
	_ = h.Add("user2", "bar")

	count1, _ := h.Count("user1")
	if count1 != 2 {
		t.Errorf("expected user1 count 2, got %d", count1)
	}

	count2, _ := h.Count("user2")
	if count2 != 2 {
		t.Errorf("expected user2 count 2, got %d", count2)
	}
}

func TestSearchHistory_OrderDescending(t *testing.T) {
	h := NewSearchHistory()

	words := []string{"first", "second", "third", "fourth"}
	for _, w := range words {
		_ = h.Add("user1", w)
	}

	records, _ := h.GetRecent("user1", 10)
	expectedOrder := []string{"fourth", "third", "second", "first"}

	for i, expected := range expectedOrder {
		if records[i].Word != expected {
			t.Errorf("record %d: expected %q, got %q", i, expected, records[i].Word)
		}
	}
}

func TestSuggestEngine_New(t *testing.T) {
	eng := NewSuggestEngine()
	if eng == nil {
		t.Fatal("expected non-nil engine")
	}

	if eng.WordCount() != 0 {
		t.Errorf("expected word count 0, got %d", eng.WordCount())
	}
}

func TestSuggestEngine_NewWithConfig(t *testing.T) {
	cfg := Config{
		MaxEditDistance:  3,
		DefaultMaxResult: 5,
		HistoryMaxSize:   50,
	}

	eng, err := NewSuggestEngineWithConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eng == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestSuggestEngine_NewWithConfig_Invalid(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"negative max edit dist", Config{MaxEditDistance: -1, DefaultMaxResult: 10, HistoryMaxSize: 100}},
		{"zero default max result", Config{MaxEditDistance: 2, DefaultMaxResult: 0, HistoryMaxSize: 100}},
		{"zero history max size", Config{MaxEditDistance: 2, DefaultMaxResult: 10, HistoryMaxSize: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSuggestEngineWithConfig(tt.cfg)
			if err == nil {
				t.Error("expected error for invalid config")
			}
		})
	}
}

func TestSuggestEngine_AddAndRemoveWord(t *testing.T) {
	eng := NewSuggestEngine()

	err := eng.AddWord("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if eng.WordCount() != 1 {
		t.Errorf("expected word count 1, got %d", eng.WordCount())
	}

	exists, freq := eng.HasWord("hello")
	if !exists {
		t.Error("expected 'hello' to exist")
	}
	if freq != 0 {
		t.Errorf("expected freq 0 for newly added word, got %d", freq)
	}

	err = eng.RemoveWord("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if eng.WordCount() != 0 {
		t.Errorf("expected word count 0, got %d", eng.WordCount())
	}
}

func TestSuggestEngine_Autocomplete(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 10)
	_ = eng.AddWordWithFreq("app", 5)
	_ = eng.AddWordWithFreq("application", 20)
	_ = eng.AddWord("banana")

	results, err := eng.Autocomplete("app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	if results[0].Word != "application" {
		t.Errorf("expected first to be 'application', got %q", results[0].Word)
	}
}

func TestSuggestEngine_AutocompleteLimit(t *testing.T) {
	eng := NewSuggestEngine()

	for i := 0; i < 20; i++ {
		_ = eng.AddWordWithFreq("a"+string(rune('a'+i)), 20-i)
	}

	results, err := eng.AutocompleteLimit("a", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}
}

func TestSuggestEngine_Correct(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 10)
	_ = eng.AddWordWithFreq("apply", 5)
	_ = eng.AddWordWithFreq("apples", 8)
	_ = eng.AddWord("banana")

	results, err := eng.Correct("appla")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one correction")
	}
}

func TestSuggestEngine_Correct_WordExists(t *testing.T) {
	eng := NewSuggestEngine()
	_ = eng.AddWord("hello")

	results, err := eng.Correct("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 corrections when word exists, got %d", len(results))
	}
}

func TestSuggestEngine_Correct_NoCandidates(t *testing.T) {
	eng := NewSuggestEngine()
	_ = eng.AddWord("hello")

	results, err := eng.Correct("xyzabc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 corrections for very different word, got %d", len(results))
	}
}

func TestSuggestEngine_CorrectLimit(t *testing.T) {
	eng := NewSuggestEngine()

	words := []string{"cat", "car", "can", "cap", "cad"}
	for _, w := range words {
		_ = eng.AddWord(w)
	}

	results, err := eng.CorrectLimit("ca", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) > 3 {
		t.Errorf("expected at most 3 results, got %d", len(results))
	}
}

func TestSuggestEngine_Correct_EmptyQuery(t *testing.T) {
	eng := NewSuggestEngine()
	_, err := eng.Correct("")
	if err != ErrEmptyQuery {
		t.Errorf("expected ErrEmptyQuery, got %v", err)
	}
}

func TestSuggestEngine_SubmitSearch(t *testing.T) {
	eng := NewSuggestEngine()

	err := eng.SubmitSearch("user1", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, freq := eng.HasWord("hello")
	if !exists {
		t.Error("expected 'hello' to exist after submit")
	}
	if freq != 1 {
		t.Errorf("expected freq 1, got %d", freq)
	}

	history, _ := eng.GetHistory("user1", 5)
	if len(history) != 1 {
		t.Errorf("expected 1 history record, got %d", len(history))
	}
}

func TestSuggestEngine_SubmitSearch_IncrementFreq(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.SubmitSearch("user1", "hello")
	_ = eng.SubmitSearch("user2", "hello")
	_ = eng.SubmitSearch("user1", "hello")

	_, freq := eng.HasWord("hello")
	if freq != 3 {
		t.Errorf("expected freq 3 after 3 submits, got %d", freq)
	}
}

func TestSuggestEngine_SubmitSearch_EmptyUserID(t *testing.T) {
	eng := NewSuggestEngine()
	err := eng.SubmitSearch("", "hello")
	if err != ErrEmptyUserID {
		t.Errorf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestSuggestEngine_SubmitSearch_EmptyWord(t *testing.T) {
	eng := NewSuggestEngine()
	err := eng.SubmitSearch("user1", "")
	if err != ErrEmptyWord {
		t.Errorf("expected ErrEmptyWord, got %v", err)
	}
}

func TestSuggestEngine_GetHistory(t *testing.T) {
	eng := NewSuggestEngine()

	words := []string{"first", "second", "third"}
	for _, w := range words {
		_ = eng.SubmitSearch("user1", w)
	}

	history, err := eng.GetHistory("user1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 history records, got %d", len(history))
	}

	if history[0].Word != "third" {
		t.Errorf("expected first history to be 'third', got %q", history[0].Word)
	}
}

func TestSuggestEngine_ClearHistory(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.SubmitSearch("user1", "hello")
	_ = eng.SubmitSearch("user1", "world")

	err := eng.ClearHistory("user1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, _ := eng.GetHistory("user1", 10)
	if len(history) != 0 {
		t.Errorf("expected 0 history records after clear, got %d", len(history))
	}
}

func TestSuggestEngine_Suggest(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 10)
	_ = eng.AddWordWithFreq("app", 5)
	_ = eng.AddWordWithFreq("application", 20)
	_ = eng.AddWordWithFreq("apply", 15)

	results, err := eng.Suggest("app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one suggestion")
	}
}

func TestSuggestEngine_SuggestLimit(t *testing.T) {
	eng := NewSuggestEngine()

	for i := 0; i < 15; i++ {
		_ = eng.AddWord("a" + string(rune('a'+i)))
	}

	results, err := eng.SuggestLimit("a", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("expected 5 suggestions, got %d", len(results))
	}
}

func TestSuggestEngine_Suggest_EmptyQuery(t *testing.T) {
	eng := NewSuggestEngine()
	_, err := eng.Suggest("")
	if err != ErrEmptyQuery {
		t.Errorf("expected ErrEmptyQuery, got %v", err)
	}
}

func TestSuggestEngine_GetHotWords(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 30)
	_ = eng.AddWordWithFreq("banana", 20)
	_ = eng.AddWordWithFreq("cherry", 10)

	hot, err := eng.GetHotWords(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hot) != 2 {
		t.Fatalf("expected 2 hot words, got %d", len(hot))
	}

	if hot[0].Word != "apple" || hot[0].Frequency != 30 {
		t.Errorf("expected first hot word 'apple' with freq 30, got %q with freq %d", hot[0].Word, hot[0].Frequency)
	}

	if hot[1].Word != "banana" || hot[1].Frequency != 20 {
		t.Errorf("expected second hot word 'banana' with freq 20, got %q with freq %d", hot[1].Word, hot[1].Frequency)
	}
}

func TestSuggestEngine_GetHotWords_InvalidN(t *testing.T) {
	eng := NewSuggestEngine()
	_, err := eng.GetHotWords(0)
	if err != ErrInvalidMaxResult {
		t.Errorf("expected ErrInvalidMaxResult, got %v", err)
	}
}

func TestSuggestEngine_GetHotWords_MoreThanAvailable(t *testing.T) {
	eng := NewSuggestEngine()
	_ = eng.AddWord("hello")

	hot, err := eng.GetHotWords(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hot) != 1 {
		t.Errorf("expected 1 hot word, got %d", len(hot))
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
			_ = trie.Insert(word)
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
			_ = trie.Insert("test")
		}()
	}

	wg.Wait()

	if trie.Size() != 1 {
		t.Errorf("expected size 1, got %d", trie.Size())
	}

	_, freq := trie.Search("test")
	if freq != numGoroutines {
		t.Errorf("expected freq %d, got %d", numGoroutines, freq)
	}
}

func TestConcurrent_SearchHistory(t *testing.T) {
	h := NewSearchHistory()
	var wg sync.WaitGroup

	numGoroutines := 50
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = h.Add("user1", "word"+string(rune('a'+id%10)))
		}(i)
	}

	wg.Wait()

	count, _ := h.Count("user1")
	if count == 0 {
		t.Error("expected some history records")
	}
}

func TestConcurrent_SuggestEngine(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	numGoroutines := 30
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			_ = eng.AddWord("word" + string(rune('a'+id%10)))
		}(i)

		go func(id int) {
			defer wg.Done()
			_, _ = eng.Autocomplete("wo")
		}(i)
	}

	wg.Wait()
}

func TestTrie_Delete_LeafCleanup(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("a")
	_ = trie.Insert("ab")
	_ = trie.Insert("abc")

	_ = trie.Delete("abc")

	exists, _ := trie.Search("ab")
	if !exists {
		t.Error("expected 'ab' to still exist after deleting 'abc'")
	}

	_ = trie.Delete("ab")

	exists, _ = trie.Search("a")
	if !exists {
		t.Error("expected 'a' to still exist after deleting 'ab'")
	}

	_ = trie.Delete("a")

	if trie.Size() != 0 {
		t.Errorf("expected size 0 after all deletes, got %d", trie.Size())
	}
}

func TestEditDistance_SingleCharOps(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
		desc     string
	}{
		{"abc", "abc", 0, "same string"},
		{"abc", "abd", 1, "substitution"},
		{"abc", "ab", 1, "deletion from end"},
		{"ab", "abc", 1, "insertion at end"},
		{"abc", "xabc", 1, "insertion at beginning"},
		{"xabc", "abc", 1, "deletion from beginning"},
		{"abc", "axc", 1, "substitution middle"},
	}

	for _, tt := range tests {
		result := EditDistance(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("%s: EditDistance(%q, %q) = %d, expected %d", tt.desc, tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestSearchHistory_TimestampOrder(t *testing.T) {
	h := NewSearchHistory()

	_ = h.Add("user1", "first")
	_ = h.Add("user1", "second")
	_ = h.Add("user1", "third")

	records, _ := h.GetRecent("user1", 10)

	if len(records) < 2 {
		t.Fatalf("expected at least 2 records, got %d", len(records))
	}

	if !records[0].Timestamp.After(records[1].Timestamp) && !records[0].Timestamp.Equal(records[1].Timestamp) {
		t.Error("expected first record to have later timestamp than second")
	}
}

func TestSuggestEngine_Correct_Sorting(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 10)
	_ = eng.AddWordWithFreq("apply", 5)
	_ = eng.AddWordWithFreq("apples", 8)

	results, err := eng.Correct("appls")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	if results[0].Frequency < results[1].Frequency && results[0].Word >= results[1].Word {
		t.Error("results not sorted correctly")
	}
}

func TestSuggestEngine_Suggest_CombinesAutocompleteAndCorrect(t *testing.T) {
	cfg := Config{
		MaxEditDistance:  2,
		DefaultMaxResult: 10,
		HistoryMaxSize:   100,
	}
	eng, _ := NewSuggestEngineWithConfig(cfg)

	_ = eng.AddWordWithFreq("apple", 20)
	_ = eng.AddWordWithFreq("app", 10)
	_ = eng.AddWordWithFreq("apply", 15)

	results, err := eng.SuggestLimit("appl", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected some suggestions")
	}

	hasApple := false
	hasApply := false
	for _, r := range results {
		if r.Word == "apple" {
			hasApple = true
		}
		if r.Word == "apply" {
			hasApply = true
		}
	}

	if !hasApple {
		t.Error("expected 'apple' in suggestions")
	}
	if !hasApply {
		t.Error("expected 'apply' in suggestions")
	}
}

func TestNewSearchHistoryWithMaxSize_Invalid(t *testing.T) {
	_, err := NewSearchHistoryWithMaxSize(0)
	if err != ErrInvalidHistoryN {
		t.Errorf("expected ErrInvalidHistoryN, got %v", err)
	}

	_, err = NewSearchHistoryWithMaxSize(-5)
	if err != ErrInvalidHistoryN {
		t.Errorf("expected ErrInvalidHistoryN for negative, got %v", err)
	}
}

func TestSuggestEngine_HistorySeparateFromWords(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.SubmitSearch("user1", "hello")
	_ = eng.SubmitSearch("user1", "world")

	_ = eng.ClearHistory("user1")

	exists, freq := eng.HasWord("hello")
	if !exists {
		t.Error("word should still exist after clearing history")
	}
	if freq != 1 {
		t.Errorf("word freq should still be 1, got %d", freq)
	}

	history, _ := eng.GetHistory("user1", 10)
	if len(history) != 0 {
		t.Errorf("history should be empty after clear, got %d records", len(history))
	}
}

func TestTrie_Insert_PrefixWords(t *testing.T) {
	trie := NewTrie()

	_ = trie.Insert("a")
	_ = trie.Insert("ab")
	_ = trie.Insert("abc")
	_ = trie.Insert("abcd")

	if trie.Size() != 4 {
		t.Errorf("expected size 4, got %d", trie.Size())
	}

	results, err := trie.StartsWith("a")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 4 {
		t.Errorf("expected 4 results with prefix 'a', got %d", len(results))
	}
}

func TestSuggestEngine_RemoveWord_NotAffectHistory(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.SubmitSearch("user1", "hello")
	_ = eng.SubmitSearch("user1", "world")

	_ = eng.RemoveWord("hello")

	history, _ := eng.GetHistory("user1", 10)
	if len(history) != 2 {
		t.Errorf("history should still have 2 records after removing word, got %d", len(history))
	}
}

func TestSuggestEngine_AddWord_FreqStartsAtZero(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWord("hello")
	_ = eng.AddWord("world")

	_, freqHello := eng.HasWord("hello")
	if freqHello != 0 {
		t.Errorf("expected freq 0 for AddWord('hello'), got %d", freqHello)
	}

	_, freqWorld := eng.HasWord("world")
	if freqWorld != 0 {
		t.Errorf("expected freq 0 for AddWord('world'), got %d", freqWorld)
	}

	if eng.WordCount() != 2 {
		t.Errorf("expected word count 2, got %d", eng.WordCount())
	}
}

func TestSuggestEngine_SubmitSearch_AfterAddWord_IncrementsFreq(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWord("hello")

	_, freq := eng.HasWord("hello")
	if freq != 0 {
		t.Fatalf("expected freq 0 after AddWord, got %d", freq)
	}

	_ = eng.SubmitSearch("user1", "hello")

	_, freq = eng.HasWord("hello")
	if freq != 1 {
		t.Errorf("expected freq 1 after one SubmitSearch, got %d", freq)
	}

	_ = eng.SubmitSearch("user2", "hello")

	_, freq = eng.HasWord("hello")
	if freq != 2 {
		t.Errorf("expected freq 2 after two SubmitSearch, got %d", freq)
	}
}

func TestSuggestEngine_AddWordWithFreq_CustomFreq(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 50)
	_ = eng.AddWordWithFreq("banana", 30)

	_, freqApple := eng.HasWord("apple")
	if freqApple != 50 {
		t.Errorf("expected freq 50 for apple, got %d", freqApple)
	}

	_, freqBanana := eng.HasWord("banana")
	if freqBanana != 30 {
		t.Errorf("expected freq 30 for banana, got %d", freqBanana)
	}

	hot, _ := eng.GetHotWords(2)
	if hot[0].Word != "apple" {
		t.Errorf("expected first hot word 'apple', got %q", hot[0].Word)
	}
}

func TestSuggestEngine_HotWords_InitWordsNotSearch_SortedByFreq(t *testing.T) {
	eng := NewSuggestEngine()

	_ = eng.AddWordWithFreq("apple", 0)
	_ = eng.AddWordWithFreq("banana", 0)
	_ = eng.AddWordWithFreq("cherry", 0)

	_ = eng.SubmitSearch("user1", "banana")
	_ = eng.SubmitSearch("user1", "banana")
	_ = eng.SubmitSearch("user1", "cherry")

	hot, _ := eng.GetHotWords(3)

	if len(hot) != 3 {
		t.Fatalf("expected 3 hot words, got %d", len(hot))
	}

	if hot[0].Word != "banana" || hot[0].Frequency != 2 {
		t.Errorf("expected first: banana(2), got %s(%d)", hot[0].Word, hot[0].Frequency)
	}

	if hot[1].Word != "cherry" || hot[1].Frequency != 1 {
		t.Errorf("expected second: cherry(1), got %s(%d)", hot[1].Word, hot[1].Frequency)
	}

	if hot[2].Word != "apple" || hot[2].Frequency != 0 {
		t.Errorf("expected third: apple(0), got %s(%d)", hot[2].Word, hot[2].Frequency)
	}
}

func TestConcurrent_SuggestEngine_AddWordAndSubmitSearch(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	_ = eng.AddWordWithFreq("shared_word", 0)

	numAddGoroutines := 20
	numSubmitGoroutines := 30

	for i := 0; i < numAddGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = eng.AddWord("word_" + string(rune('a'+id%10)))
		}(i)
	}

	for i := 0; i < numSubmitGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = eng.SubmitSearch("user_"+string(rune('0'+id%5)), "shared_word")
		}(i)
	}

	wg.Wait()

	_, freq := eng.HasWord("shared_word")
	if freq != numSubmitGoroutines {
		t.Errorf("expected shared_word freq = %d, got %d", numSubmitGoroutines, freq)
	}
}

func TestConcurrent_SuggestEngine_AutocompleteAndSubmitSearch(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	words := []string{"apple", "app", "application", "apply", "appoint", "appreciate"}
	for _, w := range words {
		_ = eng.AddWord(w)
	}

	numSubmit := 50
	numQuery := 50

	for i := 0; i < numSubmit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = eng.SubmitSearch("user1", "apple")
		}(i)
	}

	for i := 0; i < numQuery; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			results, err := eng.Autocomplete("app")
			if err != nil {
				t.Errorf("unexpected error in Autocomplete: %v", err)
				return
			}
			if len(results) == 0 {
				t.Error("expected at least one autocomplete result")
			}
		}(i)
	}

	wg.Wait()

	_, freq := eng.HasWord("apple")
	if freq != numSubmit {
		t.Errorf("expected 'apple' freq = %d after concurrent submits, got %d", numSubmit, freq)
	}
}

func TestConcurrent_SuggestEngine_CorrectAndSubmitSearch(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	words := []string{"apple", "apply", "apples", "banana", "bandana"}
	for _, w := range words {
		_ = eng.AddWord(w)
	}

	numSubmit := 30
	numCorrect := 30

	for i := 0; i < numSubmit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = eng.SubmitSearch("user1", "apple")
		}(i)
	}

	for i := 0; i < numCorrect; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			results, err := eng.Correct("appla")
			if err != nil {
				t.Errorf("unexpected error in Correct: %v", err)
				return
			}
			_ = results
		}(i)
	}

	wg.Wait()

	_, freq := eng.HasWord("apple")
	if freq != numSubmit {
		t.Errorf("expected 'apple' freq = %d, got %d", numSubmit, freq)
	}
}

func TestConcurrent_SuggestEngine_GetHotWordsAndSubmit(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	_ = eng.AddWord("word1")
	_ = eng.AddWord("word2")
	_ = eng.AddWord("word3")

	numSubmit := 40
	numGetHot := 40

	for i := 0; i < numSubmit; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := "word" + string(rune('1'+id%3))
			_ = eng.SubmitSearch("user1", w)
		}(i)
	}

	for i := 0; i < numGetHot; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hot, err := eng.GetHotWords(3)
			if err != nil {
				t.Errorf("unexpected error in GetHotWords: %v", err)
				return
			}
			if len(hot) != 3 {
				t.Errorf("expected 3 hot words, got %d", len(hot))
			}
		}()
	}

	wg.Wait()

	if eng.WordCount() != 3 {
		t.Errorf("expected 3 words, got %d", eng.WordCount())
	}
}

func TestConcurrent_SuggestEngine_HistoryAndSubmit(t *testing.T) {
	eng := NewSuggestEngine()
	var wg sync.WaitGroup

	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			_ = eng.SubmitSearch("user1", "word_"+string(rune('a'+id%10)))
		}(i)

		go func() {
			defer wg.Done()
			_, _ = eng.GetHistory("user1", 10)
		}()
	}

	wg.Wait()

	count, _ := eng.GetHistory("user1", 100)
	if len(count) == 0 {
		t.Error("expected some history records")
	}
}
