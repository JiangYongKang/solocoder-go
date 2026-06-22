package skiplist

import (
	"testing"
)

func TestNew(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if sl == nil {
		t.Fatal("New() returned nil")
	}
	if sl.Len() != 0 {
		t.Errorf("Len() = %d, want 0", sl.Len())
	}
	if sl.Level() != 1 {
		t.Errorf("Level() = %d, want 1", sl.Level())
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := &Config{MaxLevel: 16, P: 0.5}
	sl, err := New[string, int](cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if sl == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{"zero max level", &Config{MaxLevel: 0, P: 0.25}},
		{"negative max level", &Config{MaxLevel: -5, P: 0.25}},
		{"zero probability", &Config{MaxLevel: 16, P: 0}},
		{"one probability", &Config{MaxLevel: 16, P: 1}},
		{"negative probability", &Config{MaxLevel: 16, P: -0.5}},
		{"probability over one", &Config{MaxLevel: 16, P: 1.5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sl, err := New[int, string](tt.cfg)
			if err == nil {
				t.Errorf("New() expected error, got nil")
			}
			if sl != nil {
				t.Errorf("New() expected nil SkipList, got non-nil")
			}
		})
	}
}

func TestInsertAndSearch(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sl.Insert(3, "three")
	sl.Insert(1, "one")
	sl.Insert(2, "two")
	sl.Insert(5, "five")
	sl.Insert(4, "four")

	if sl.Len() != 5 {
		t.Errorf("Len() = %d, want 5", sl.Len())
	}

	tests := []struct {
		key      int
		wantVal  string
		wantOk   bool
	}{
		{1, "one", true},
		{2, "two", true},
		{3, "three", true},
		{4, "four", true},
		{5, "five", true},
		{0, "", false},
		{6, "", false},
		{100, "", false},
	}

	for _, tt := range tests {
		val, ok := sl.Search(tt.key)
		if ok != tt.wantOk {
			t.Errorf("Search(%d) ok = %v, want %v", tt.key, ok, tt.wantOk)
		}
		if ok && val != tt.wantVal {
			t.Errorf("Search(%d) val = %v, want %v", tt.key, val, tt.wantVal)
		}
	}
}

func TestInsertUpdate(t *testing.T) {
	sl, err := New[string, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sl.Insert("a", 1)
	sl.Insert("b", 2)
	sl.Insert("a", 100)

	if sl.Len() != 2 {
		t.Errorf("Len() = %d, want 2 (update should not increase length)", sl.Len())
	}

	val, ok := sl.Search("a")
	if !ok {
		t.Error("Search('a') not found")
	}
	if val != 100 {
		t.Errorf("Search('a') = %d, want 100", val)
	}
}

func TestDelete(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	keys := []int{3, 1, 4, 1, 5, 9, 2, 6}
	for _, k := range keys {
		sl.Insert(k, string(rune('0'+k)))
	}

	tests := []struct {
		key     int
		wantDel bool
	}{
		{1, true},
		{9, true},
		{5, true},
		{100, false},
		{0, false},
		{1, false},
	}

	for _, tt := range tests {
		val, ok := sl.Delete(tt.key)
		if ok != tt.wantDel {
			t.Errorf("Delete(%d) ok = %v, want %v", tt.key, ok, tt.wantDel)
		}
		if ok && val != string(rune('0'+tt.key)) {
			t.Errorf("Delete(%d) val = %v, want %v", tt.key, val, string(rune('0'+tt.key)))
		}
	}

	if sl.Len() != 4 {
		t.Errorf("Len() = %d, want 4 after 3 deletions (duplicate key 1)", sl.Len())
	}

	if sl.Contains(1) {
		t.Error("Contains(1) should be false after deletion")
	}
	if !sl.Contains(2) {
		t.Error("Contains(2) should be true")
	}
}

func TestDeleteFromEmpty(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	val, ok := sl.Delete(42)
	if ok {
		t.Error("Delete from empty SkipList should return false")
	}
	var zero string
	if val != zero {
		t.Errorf("Delete from empty should return zero value, got %v", val)
	}
}

func TestDeleteAll(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := 0; i < 100; i++ {
		sl.Insert(i, "val")
	}

	for i := 0; i < 100; i++ {
		_, ok := sl.Delete(i)
		if !ok {
			t.Errorf("Delete(%d) failed", i)
		}
	}

	if sl.Len() != 0 {
		t.Errorf("Len() = %d, want 0 after deleting all", sl.Len())
	}
	if sl.Level() != 1 {
		t.Errorf("Level() = %d, want 1 after deleting all", sl.Level())
	}
}

func TestContains(t *testing.T) {
	sl, err := New[float64, bool]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	sl.Insert(1.5, true)
	sl.Insert(2.7, false)
	sl.Insert(3.14, true)

	if !sl.Contains(1.5) {
		t.Error("Contains(1.5) = false, want true")
	}
	if !sl.Contains(3.14) {
		t.Error("Contains(3.14) = false, want true")
	}
	if sl.Contains(0.0) {
		t.Error("Contains(0.0) = true, want false")
	}
	if sl.Contains(99.9) {
		t.Error("Contains(99.9) = true, want false")
	}
}

func TestRange(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := 1; i <= 10; i++ {
		sl.Insert(i, string(rune('a'+i-1)))
	}

	t.Run("full range inclusive", func(t *testing.T) {
		result := sl.Range(1, 10)
		if len(result) != 10 {
			t.Fatalf("Range(1,10) len = %d, want 10", len(result))
		}
		for i, p := range result {
			if p.Key != i+1 {
				t.Errorf("result[%d].Key = %d, want %d", i, p.Key, i+1)
			}
			if p.Value != string(rune('a'+i)) {
				t.Errorf("result[%d].Value = %s, want %s", i, p.Value, string(rune('a'+i)))
			}
		}
	})

	t.Run("partial range", func(t *testing.T) {
		result := sl.Range(3, 7)
		if len(result) != 5 {
			t.Fatalf("Range(3,7) len = %d, want 5", len(result))
		}
		if result[0].Key != 3 || result[len(result)-1].Key != 7 {
			t.Errorf("Range(3,7) = [%d..%d], want [3..7]", result[0].Key, result[len(result)-1].Key)
		}
	})

	t.Run("start > end returns empty", func(t *testing.T) {
		result := sl.Range(10, 1)
		if len(result) != 0 {
			t.Errorf("Range(10,1) len = %d, want 0", len(result))
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		result := sl.Range(100, 200)
		if len(result) != 0 {
			t.Errorf("Range(100,200) len = %d, want 0", len(result))
		}

		result = sl.Range(-10, 0)
		if len(result) != 0 {
			t.Errorf("Range(-10,0) len = %d, want 0", len(result))
		}
	})

	t.Run("single element", func(t *testing.T) {
		result := sl.Range(5, 5)
		if len(result) != 1 {
			t.Fatalf("Range(5,5) len = %d, want 1", len(result))
		}
		if result[0].Key != 5 {
			t.Errorf("Range(5,5)[0].Key = %d, want 5", result[0].Key)
		}
	})

	t.Run("start exclusive", func(t *testing.T) {
		opts := DefaultRangeOptions().WithStartInclusive(false)
		result := sl.Range(3, 7, opts)
		if len(result) != 4 {
			t.Fatalf("Range(3,7, startExcl) len = %d, want 4", len(result))
		}
		if result[0].Key != 4 {
			t.Errorf("start exclusive: first key = %d, want 4", result[0].Key)
		}
	})

	t.Run("end exclusive", func(t *testing.T) {
		opts := DefaultRangeOptions().WithEndInclusive(false)
		result := sl.Range(3, 7, opts)
		if len(result) != 4 {
			t.Fatalf("Range(3,7, endExcl) len = %d, want 4", len(result))
		}
		if result[len(result)-1].Key != 6 {
			t.Errorf("end exclusive: last key = %d, want 6", result[len(result)-1].Key)
		}
	})

	t.Run("both exclusive", func(t *testing.T) {
		opts := DefaultRangeOptions().WithStartInclusive(false).WithEndInclusive(false)
		result := sl.Range(3, 7, opts)
		if len(result) != 3 {
			t.Fatalf("Range(3,7, bothExcl) len = %d, want 3", len(result))
		}
		if result[0].Key != 4 || result[len(result)-1].Key != 6 {
			t.Errorf("both exclusive: got [%d..%d], want [4..6]", result[0].Key, result[len(result)-1].Key)
		}
	})

	t.Run("with limit", func(t *testing.T) {
		opts := DefaultRangeOptions().WithLimit(3)
		result := sl.Range(1, 10, opts)
		if len(result) != 3 {
			t.Errorf("Range with Limit=3 len = %d, want 3", len(result))
		}
		if result[0].Key != 1 || result[2].Key != 3 {
			t.Errorf("limit result keys = [%d,%d,%d], want [1,2,3]", result[0].Key, result[1].Key, result[2].Key)
		}
	})

	t.Run("with offset", func(t *testing.T) {
		opts := DefaultRangeOptions().WithOffset(2)
		result := sl.Range(1, 10, opts)
		if len(result) != 8 {
			t.Errorf("Range with Offset=2 len = %d, want 8", len(result))
		}
		if result[0].Key != 3 {
			t.Errorf("offset result first key = %d, want 3", result[0].Key)
		}
	})

	t.Run("with offset and limit", func(t *testing.T) {
		opts := DefaultRangeOptions().WithOffset(2).WithLimit(3)
		result := sl.Range(1, 10, opts)
		if len(result) != 3 {
			t.Errorf("Range offset=2,limit=3 len = %d, want 3", len(result))
		}
		if result[0].Key != 3 || result[2].Key != 5 {
			t.Errorf("offset+limit keys = [%d,%d,%d], want [3,4,5]", result[0].Key, result[1].Key, result[2].Key)
		}
	})

	t.Run("offset >= count", func(t *testing.T) {
		opts := DefaultRangeOptions().WithOffset(100)
		result := sl.Range(1, 10, opts)
		if len(result) != 0 {
			t.Errorf("Range offset=100 len = %d, want 0", len(result))
		}
	})

	t.Run("limit 0 means no limit", func(t *testing.T) {
		opts := DefaultRangeOptions().WithLimit(0)
		result := sl.Range(1, 10, opts)
		if len(result) != 10 {
			t.Errorf("Range with Limit=0 (no limit) len = %d, want 10", len(result))
		}
	})
}

func TestRangeEmptySkipList(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result := sl.Range(1, 100)
	if len(result) != 0 {
		t.Errorf("Range on empty list len = %d, want 0", len(result))
	}
}

func TestAll(t *testing.T) {
	sl, err := New[int, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	n := 50
	for i := n; i >= 1; i-- {
		sl.Insert(i, i*i)
	}

	result := sl.All()
	if len(result) != n {
		t.Fatalf("All() len = %d, want %d", len(result), n)
	}

	for i, p := range result {
		expectedKey := i + 1
		expectedVal := expectedKey * expectedKey
		if p.Key != expectedKey {
			t.Errorf("All()[%d].Key = %d, want %d", i, p.Key, expectedKey)
		}
		if p.Value != expectedVal {
			t.Errorf("All()[%d].Value = %d, want %d", i, p.Value, expectedVal)
		}
	}
}

func TestAllEmpty(t *testing.T) {
	sl, err := New[string, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	result := sl.All()
	if len(result) != 0 {
		t.Errorf("All() on empty = %d, want 0", len(result))
	}
}

func TestClear(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := 0; i < 100; i++ {
		sl.Insert(i, "val")
	}

	if sl.Len() != 100 {
		t.Fatalf("Before Clear: Len() = %d, want 100", sl.Len())
	}

	sl.Clear()

	if sl.Len() != 0 {
		t.Errorf("After Clear: Len() = %d, want 0", sl.Len())
	}
	if sl.Level() != 1 {
		t.Errorf("After Clear: Level() = %d, want 1", sl.Level())
	}
	if len(sl.All()) != 0 {
		t.Errorf("After Clear: All() len = %d, want 0", len(sl.All()))
	}

	_, ok := sl.Search(50)
	if ok {
		t.Error("After Clear: Search(50) found something")
	}

	sl.Insert(42, "new")
	if sl.Len() != 1 {
		t.Errorf("After Clear + Insert: Len() = %d, want 1", sl.Len())
	}
}

func TestFirstLast(t *testing.T) {
	sl, err := New[int, string]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, ok := sl.First()
	if ok {
		t.Error("First() on empty should return false")
	}
	_, ok = sl.Last()
	if ok {
		t.Error("Last() on empty should return false")
	}

	sl.Insert(5, "five")
	sl.Insert(1, "one")
	sl.Insert(10, "ten")

	first, ok := sl.First()
	if !ok {
		t.Error("First() returned false")
	}
	if first.Key != 1 || first.Value != "one" {
		t.Errorf("First() = (%d, %s), want (1, one)", first.Key, first.Value)
	}

	last, ok := sl.Last()
	if !ok {
		t.Error("Last() returned false")
	}
	if last.Key != 10 || last.Value != "ten" {
		t.Errorf("Last() = (%d, %s), want (10, ten)", last.Key, last.Value)
	}

	sl.Delete(1)
	first, _ = sl.First()
	if first.Key != 5 {
		t.Errorf("After delete first: First().Key = %d, want 5", first.Key)
	}

	sl.Delete(10)
	last, _ = sl.Last()
	if last.Key != 5 {
		t.Errorf("After delete last: Last().Key = %d, want 5", last.Key)
	}

	sl.Delete(5)
	_, ok = sl.First()
	if ok {
		t.Error("After all deleted: First() should return false")
	}
	_, ok = sl.Last()
	if ok {
		t.Error("After all deleted: Last() should return false")
	}
}

func TestStringKeys(t *testing.T) {
	sl, err := New[string, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	words := []string{"banana", "apple", "cherry", "date", "elderberry"}
	for i, w := range words {
		sl.Insert(w, i+1)
	}

	result := sl.All()
	sorted := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for i, p := range result {
		if p.Key != sorted[i] {
			t.Errorf("String key order: result[%d] = %s, want %s", i, p.Key, sorted[i])
		}
	}

	r := sl.Range("banana", "date")
	if len(r) != 3 {
		t.Errorf("Range(banana, date) len = %d, want 3", len(r))
	}
}

func TestCustomProbability(t *testing.T) {
	const trials = 5
	const n = 1000

	sumHigh := 0
	sumLow := 0

	for i := 0; i < trials; i++ {
		cfgHigh := &Config{MaxLevel: 32, P: 0.9}
		slHigh, err := New[int, string](cfgHigh)
		if err != nil {
			t.Fatalf("New(high) error = %v", err)
		}
		for j := 0; j < n; j++ {
			slHigh.Insert(j, "val")
		}
		if slHigh.Len() != n {
			t.Errorf("High prob trial %d: Len() = %d, want %d", i, slHigh.Len(), n)
		}
		lvlHigh := slHigh.Level()
		if lvlHigh <= 1 {
			t.Errorf("High prob (0.9) trial %d: level = %d, expected > 1", i, lvlHigh)
		}
		sumHigh += lvlHigh

		cfgLow := &Config{MaxLevel: 32, P: 0.01}
		slLow, _ := New[int, string](cfgLow)
		for j := 0; j < n; j++ {
			slLow.Insert(j, "val")
		}
		if slLow.Len() != n {
			t.Errorf("Low prob trial %d: Len() = %d, want %d", i, slLow.Len(), n)
		}
		lvlLow := slLow.Level()
		if lvlLow <= 1 {
			t.Errorf("Low prob (0.01) trial %d: level = %d, expected > 1", i, lvlLow)
		}
		sumLow += lvlLow
	}

	avgHigh := sumHigh / trials
	avgLow := sumLow / trials

	if avgHigh <= avgLow {
		t.Errorf("Average level with P=0.9 (%d over %d trials) should be > P=0.01 (%d); "+
			"probability mechanism may not be working correctly", avgHigh, trials, avgLow)
	}

	cfgCheck := &Config{MaxLevel: 32, P: 0.5}
	slCheck, _ := New[int, string](cfgCheck)
	for i := 0; i < n; i++ {
		slCheck.Insert(i, "val")
	}
	all := slCheck.All()
	if len(all) != n {
		t.Fatalf("Data integrity check: All() len = %d, want %d", len(all), n)
	}
	for i, p := range all {
		if p.Key != i {
			t.Errorf("Data integrity: All()[%d].Key = %d, want %d", i, p.Key, i)
		}
	}
}

func TestLargeInsert(t *testing.T) {
	sl, err := New[int, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	n := 10000
	for i := 0; i < n; i++ {
		sl.Insert(i, i*2)
	}

	if sl.Len() != n {
		t.Errorf("Len() = %d, want %d", sl.Len(), n)
	}

	for i := 0; i < n; i += 100 {
		val, ok := sl.Search(i)
		if !ok {
			t.Errorf("Search(%d) not found", i)
		}
		if val != i*2 {
			t.Errorf("Search(%d) = %d, want %d", i, val, i*2)
		}
	}

	result := sl.Range(5000, 5099)
	if len(result) != 100 {
		t.Errorf("Range(5000,5099) len = %d, want 100", len(result))
	}
}

func TestConcurrentSafe(t *testing.T) {
	sl, err := New[int, int]()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	n := 1000
	done := make(chan bool)

	go func() {
		for i := 0; i < n; i++ {
			sl.Insert(i, i)
		}
		done <- true
	}()

	go func() {
		for i := n; i < 2*n; i++ {
			sl.Insert(i, i)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < n; i++ {
			sl.Search(i)
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}

	if sl.Len() != 2*n {
		t.Errorf("After concurrent inserts: Len() = %d, want %d", sl.Len(), 2*n)
	}

	for i := 0; i < 2*n; i++ {
		val, ok := sl.Search(i)
		if !ok {
			t.Errorf("Search(%d) not found after concurrent inserts", i)
		}
		if ok && val != i {
			t.Errorf("Search(%d) = %d, want %d", i, val, i)
		}
	}

	all := sl.All()
	if len(all) != 2*n {
		t.Errorf("All() len = %d, want %d", len(all), 2*n)
	}
	for i, p := range all {
		if p.Key != i {
			t.Errorf("All()[%d].Key = %d, want %d (ordering broken)", i, p.Key, i)
		}
		if p.Value != i {
			t.Errorf("All()[%d].Value = %d, want %d", i, p.Value, i)
		}
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("single element operations", func(t *testing.T) {
		sl, _ := New[int, string]()
		sl.Insert(1, "one")

		if sl.Len() != 1 {
			t.Errorf("Len() = %d, want 1", sl.Len())
		}

		val, ok := sl.Search(1)
		if !ok || val != "one" {
			t.Errorf("Search(1) = (%v, %v), want (one, true)", val, ok)
		}

		delVal, ok := sl.Delete(1)
		if !ok || delVal != "one" {
			t.Errorf("Delete(1) = (%v, %v), want (one, true)", delVal, ok)
		}

		if sl.Len() != 0 {
			t.Errorf("After delete Len() = %d, want 0", sl.Len())
		}
	})

	t.Run("duplicate insert replaces", func(t *testing.T) {
		sl, _ := New[int, int]()
		sl.Insert(42, 1)
		sl.Insert(42, 2)
		sl.Insert(42, 3)

		if sl.Len() != 1 {
			t.Errorf("Len after duplicate inserts = %d, want 1", sl.Len())
		}
		val, _ := sl.Search(42)
		if val != 3 {
			t.Errorf("Value after duplicates = %d, want 3", val)
		}
	})

	t.Run("search min and max", func(t *testing.T) {
		sl, _ := New[int, bool]()
		for i := -100; i <= 100; i++ {
			sl.Insert(i, true)
		}

		if !sl.Contains(-100) {
			t.Error("Contains(-100) = false")
		}
		if !sl.Contains(100) {
			t.Error("Contains(100) = false")
		}
		if sl.Contains(-101) {
			t.Error("Contains(-101) = true")
		}
		if sl.Contains(101) {
			t.Error("Contains(101) = true")
		}
	})

	t.Run("range around boundaries", func(t *testing.T) {
		sl, _ := New[int, int]()
		for i := 10; i <= 20; i++ {
			sl.Insert(i, i)
		}

		r := sl.Range(1, 9)
		if len(r) != 0 {
			t.Errorf("Range below all len = %d, want 0", len(r))
		}

		r = sl.Range(21, 100)
		if len(r) != 0 {
			t.Errorf("Range above all len = %d, want 0", len(r))
		}

		r = sl.Range(5, 15)
		if len(r) != 6 {
			t.Errorf("Range overlapping bottom len = %d, want 6", len(r))
		}
		if r[0].Key != 10 {
			t.Errorf("First key = %d, want 10", r[0].Key)
		}

		r = sl.Range(15, 25)
		if len(r) != 6 {
			t.Errorf("Range overlapping top len = %d, want 6", len(r))
		}
		if r[len(r)-1].Key != 20 {
			t.Errorf("Last key = %d, want 20", r[len(r)-1].Key)
		}
	})
}

func BenchmarkInsert(b *testing.B) {
	sl, _ := New[int, int]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Insert(i, i)
	}
}

func BenchmarkSearch(b *testing.B) {
	sl, _ := New[int, int]()
	for i := 0; i < 10000; i++ {
		sl.Insert(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Search(i % 10000)
	}
}

func BenchmarkDelete(b *testing.B) {
	sl, _ := New[int, int]()
	for i := 0; i < b.N; i++ {
		sl.Insert(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sl.Delete(i)
	}
}
