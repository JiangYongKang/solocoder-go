package bplustree

import (
	"fmt"
	"testing"
)

func TestNewBPlusTree(t *testing.T) {
	tree := NewBPlusTree()
	if tree == nil {
		t.Fatal("NewBPlusTree returned nil")
	}
	if tree.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", tree.Count())
	}
	if tree.maxKeys != 32 {
		t.Errorf("expected default maxKeys 32, got %d", tree.maxKeys)
	}
}

func TestNewBPlusTreeWithConfig(t *testing.T) {
	cfg := Config{MaxKeys: 4}
	tree := NewBPlusTreeWithConfig(cfg)
	if tree == nil {
		t.Fatal("NewBPlusTreeWithConfig returned nil")
	}
	if tree.maxKeys != 4 {
		t.Errorf("expected maxKeys 4, got %d", tree.maxKeys)
	}
}

func TestNewBPlusTreeWithConfig_TooSmall(t *testing.T) {
	cfg := Config{MaxKeys: 1}
	tree := NewBPlusTreeWithConfig(cfg)
	if tree.maxKeys != 32 {
		t.Errorf("expected maxKeys to default to 32 for invalid config, got %d", tree.maxKeys)
	}
}

func TestNewBPlusTreeWithConfig_OddNumber(t *testing.T) {
	cfg := Config{MaxKeys: 5}
	tree := NewBPlusTreeWithConfig(cfg)
	if tree.maxKeys != 6 {
		t.Errorf("expected maxKeys to be rounded up to 6, got %d", tree.maxKeys)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxKeys != 32 {
		t.Errorf("expected default MaxKeys 32, got %d", cfg.MaxKeys)
	}
}

func TestInsertAndSearch(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "value1")
	tree.Insert("key2", "value2")
	tree.Insert("key3", "value3")

	val, ok := tree.Search("key1")
	if !ok || val != "value1" {
		t.Errorf("expected key1=value1, got %s (ok=%v)", val, ok)
	}

	val, ok = tree.Search("key2")
	if !ok || val != "value2" {
		t.Errorf("expected key2=value2, got %s (ok=%v)", val, ok)
	}

	val, ok = tree.Search("key3")
	if !ok || val != "value3" {
		t.Errorf("expected key3=value3, got %s (ok=%v)", val, ok)
	}

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}
}

func TestInsert_UpdateExisting(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "old_value")
	tree.Insert("key1", "new_value")

	val, ok := tree.Search("key1")
	if !ok || val != "new_value" {
		t.Errorf("expected new_value, got %s (ok=%v)", val, ok)
	}

	if tree.Count() != 1 {
		t.Errorf("expected count 1 after update, got %d", tree.Count())
	}
}

func TestSearch_NonExistent(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "value1")

	_, ok := tree.Search("nonexistent")
	if ok {
		t.Error("expected ok=false for non-existent key")
	}
}

func TestSearch_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	_, ok := tree.Search("anything")
	if ok {
		t.Error("expected ok=false for search on empty tree")
	}
}

func TestInsert_ReverseOrder(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("c", "3")
	tree.Insert("b", "2")
	tree.Insert("a", "1")

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}

	val, ok := tree.Search("a")
	if !ok || val != "1" {
		t.Errorf("expected a=1, got %s (ok=%v)", val, ok)
	}
	val, ok = tree.Search("b")
	if !ok || val != "2" {
		t.Errorf("expected b=2, got %s (ok=%v)", val, ok)
	}
	val, ok = tree.Search("c")
	if !ok || val != "3" {
		t.Errorf("expected c=3, got %s (ok=%v)", val, ok)
	}
}

func TestInsert_ManyKeys(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 100
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	if tree.Count() != n {
		t.Errorf("expected count %d, got %d", n, tree.Count())
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found", key)
			continue
		}
		expected := fmt.Sprintf("val%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestInsert_DuplicateKeys(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert("same_key", fmt.Sprintf("val_%d", i))
	}

	if tree.Count() != 1 {
		t.Errorf("expected count 1 after duplicate inserts, got %d", tree.Count())
	}

	val, ok := tree.Search("same_key")
	if !ok || val != "val_9" {
		t.Errorf("expected val_9, got %s (ok=%v)", val, ok)
	}
}

func TestInsert_EmptyKey(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("", "empty_key_value")

	val, ok := tree.Search("")
	if !ok || val != "empty_key_value" {
		t.Errorf("expected empty_key_value, got %s (ok=%v)", val, ok)
	}
}

func TestInsert_EmptyValue(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key_empty_val", "")

	val, ok := tree.Search("key_empty_val")
	if !ok || val != "" {
		t.Errorf("expected empty value, got %s (ok=%v)", val, ok)
	}
}

func TestSplit_LeafSplit(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 5; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	if tree.Count() != 5 {
		t.Errorf("expected count 5, got %d", tree.Count())
	}

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("k%d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found after split", key)
			continue
		}
		expected := fmt.Sprintf("v%d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestSplit_MultipleSplits(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 50
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	if tree.Count() != n {
		t.Errorf("expected count %d, got %d", n, tree.Count())
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found after multiple splits", key)
			continue
		}
		expected := fmt.Sprintf("val%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestSplit_InternalNodeSplit(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 30
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	if tree.root.isLeaf {
		t.Error("expected root to be internal node after many inserts")
	}

	if tree.Count() != n {
		t.Errorf("expected count %d, got %d", n, tree.Count())
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found", key)
			continue
		}
		expected := fmt.Sprintf("val%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestSplit_ReverseInsertion(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 30
	for i := n - 1; i >= 0; i-- {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	if tree.Count() != n {
		t.Errorf("expected count %d, got %d", n, tree.Count())
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found", key)
			continue
		}
		expected := fmt.Sprintf("val%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestSplit_RootSplitCreatesNewRoot(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")

	if !tree.root.isLeaf {
		t.Error("root should still be leaf with 4 keys and maxKeys=4")
	}

	tree.Insert("e", "5")

	if tree.root.isLeaf {
		t.Error("root should be internal node after split")
	}

	if len(tree.root.children) < 2 {
		t.Error("root should have at least 2 children after split")
	}
}

func TestSplit_LeafLinkedList(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	current := tree.root
	for !current.isLeaf {
		current = current.children[0]
	}

	var keys []string
	for current != nil {
		keys = append(keys, current.keys...)
		current = current.next
	}

	if len(keys) != 10 {
		t.Errorf("expected 10 keys in leaf chain, got %d", len(keys))
	}

	for i := 0; i < len(keys)-1; i++ {
		if keys[i] > keys[i+1] {
			t.Errorf("leaf chain not sorted at position %d: %s > %s", i, keys[i], keys[i+1])
		}
	}
}

func TestDelete_Basic(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "value1")
	tree.Insert("key2", "value2")
	tree.Insert("key3", "value3")

	deleted := tree.Delete("key2")
	if !deleted {
		t.Error("expected Delete to return true for existing key")
	}

	_, ok := tree.Search("key2")
	if ok {
		t.Error("expected key2 to be deleted")
	}

	if tree.Count() != 2 {
		t.Errorf("expected count 2 after delete, got %d", tree.Count())
	}
}

func TestDelete_NonExistent(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "value1")

	deleted := tree.Delete("nonexistent")
	if deleted {
		t.Error("expected Delete to return false for non-existent key")
	}

	if tree.Count() != 1 {
		t.Errorf("expected count unchanged at 1, got %d", tree.Count())
	}
}

func TestDelete_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()

	deleted := tree.Delete("anything")
	if deleted {
		t.Error("expected Delete to return false on empty tree")
	}
}

func TestDelete_AllKeys(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	for i := 0; i < 10; i++ {
		deleted := tree.Delete(fmt.Sprintf("k%d", i))
		if !deleted {
			t.Errorf("expected delete of k%d to succeed", i)
		}
	}

	if tree.Count() != 0 {
		t.Errorf("expected count 0 after deleting all keys, got %d", tree.Count())
	}
}

func TestDelete_FirstKey(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")

	deleted := tree.Delete("a")
	if !deleted {
		t.Error("expected delete of 'a' to succeed")
	}

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}

	_, ok := tree.Search("a")
	if ok {
		t.Error("expected 'a' to be gone")
	}

	val, ok := tree.Search("b")
	if !ok || val != "2" {
		t.Errorf("expected b=2, got %s (ok=%v)", val, ok)
	}
}

func TestDelete_LastKey(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")

	deleted := tree.Delete("d")
	if !deleted {
		t.Error("expected delete of 'd' to succeed")
	}

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}

	_, ok := tree.Search("d")
	if ok {
		t.Error("expected 'd' to be gone")
	}

	val, ok := tree.Search("c")
	if !ok || val != "3" {
		t.Errorf("expected c=3, got %s (ok=%v)", val, ok)
	}
}

func TestDelete_MiddleKey(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")

	deleted := tree.Delete("b")
	if !deleted {
		t.Error("expected delete of 'b' to succeed")
	}

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}

	_, ok := tree.Search("b")
	if ok {
		t.Error("expected 'b' to be gone")
	}
}

func TestDelete_WithSplits(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 20
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	for i := 0; i < n; i += 2 {
		key := fmt.Sprintf("key%03d", i)
		deleted := tree.Delete(key)
		if !deleted {
			t.Errorf("expected delete of %s to succeed", key)
		}
	}

	if tree.Count() != n/2 {
		t.Errorf("expected count %d, got %d", n/2, tree.Count())
	}

	for i := 1; i < n; i += 2 {
		key := fmt.Sprintf("key%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should still exist", key)
			continue
		}
		expected := fmt.Sprintf("val%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}
}

func TestRangeScan_Basic(t *testing.T) {
	tree := NewBPlusTree()
	keys := []string{"apple", "banana", "cherry", "date", "elderberry"}
	for _, k := range keys {
		tree.Insert(k, "value_"+k)
	}

	result, err := tree.RangeScan("banana", "date")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	expectedKeys := []string{"banana", "cherry", "date"}
	if len(result) != len(expectedKeys) {
		t.Fatalf("expected %d items, got %d", len(expectedKeys), len(result))
	}

	for i, expected := range expectedKeys {
		if result[i].Key != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, result[i].Key)
		}
	}
}

func TestRangeScan_InvalidRange(t *testing.T) {
	tree := NewBPlusTree()
	_, err := tree.RangeScan("z", "a")
	if err != ErrInvalidRange {
		t.Errorf("expected ErrInvalidRange, got %v", err)
	}
}

func TestRangeScan_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	result, err := tree.RangeScan("a", "z")
	if err != nil {
		t.Fatalf("RangeScan on empty tree should not error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results, got %d", len(result))
	}
}

func TestRangeScan_NoResults(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")

	result, err := tree.RangeScan("x", "z")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 results for range with no matching keys, got %d", len(result))
	}
}

func TestRangeScan_SingleKey(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	result, err := tree.RangeScan("b", "b")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 1 || result[0].Key != "b" {
		t.Errorf("expected single item 'b', got %v", result)
	}
}

func TestRangeScan_FullRange(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("m", "2")
	tree.Insert("z", "3")

	result, err := tree.RangeScan("a", "z")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 items for full range, got %d", len(result))
	}
}

func TestRangeScan_SortedOrder(t *testing.T) {
	tree := NewBPlusTree()
	unsortedKeys := []string{"zebra", "apple", "mango", "banana", "cherry"}
	for _, k := range unsortedKeys {
		tree.Insert(k, "v")
	}

	result, err := tree.RangeScan("apple", "zebra")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	expected := []string{"apple", "banana", "cherry", "mango", "zebra"}
	if len(result) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(result))
	}

	for i, exp := range expected {
		if result[i].Key != exp {
			t.Errorf("position %d: expected %s, got %s", i, exp, result[i].Key)
		}
	}
}

func TestRangeScan_CrossLeafNodes(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 20; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	result, err := tree.RangeScan("key005", "key014")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	expectedCount := 10
	if len(result) != expectedCount {
		t.Fatalf("expected %d items, got %d", expectedCount, len(result))
	}

	for i, item := range result {
		expectedKey := fmt.Sprintf("key%03d", i+5)
		if item.Key != expectedKey {
			t.Errorf("position %d: expected %s, got %s", i, expectedKey, item.Key)
		}
	}
}

func TestRangeScan_WithSplits(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 100
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	result, err := tree.RangeScan("key010", "key019")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	if len(result) != 10 {
		t.Fatalf("expected 10 items, got %d", len(result))
	}

	for i, item := range result {
		expectedKey := fmt.Sprintf("key%03d", i+10)
		if item.Key != expectedKey {
			t.Errorf("position %d: expected %s, got %s", i, expectedKey, item.Key)
		}
	}
}

func TestRangeScan_Inclusive(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("m", "2")
	tree.Insert("z", "3")

	result, err := tree.RangeScan("a", "z")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 items (inclusive), got %d", len(result))
	}
}

func TestRangeScan_StartBeforeAll(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")

	result, err := tree.RangeScan("a", "d")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 items (c, d), got %d", len(result))
	}
}

func TestRangeScan_EndAfterAll(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")

	result, err := tree.RangeScan("d", "z")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 items (d, e), got %d", len(result))
	}
}

func TestIterator_BasicForward(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()
	if !it.Valid() {
		t.Fatal("iterator should be valid")
	}

	var keys []string
	for it.Valid() {
		key, err := it.Key()
		if err != nil {
			t.Fatalf("Key() error: %v", err)
		}
		keys = append(keys, key)
		it.Next()
	}

	expected := []string{"a", "b", "c"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d", len(expected), len(keys))
	}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("position %d: expected %s, got %s", i, k, keys[i])
		}
	}
}

func TestIterator_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()
	if it.Valid() {
		t.Error("iterator on empty tree should not be valid")
	}
}

func TestIterator_KeyAndValue(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("key1", "value1")

	it := tree.NewIterator()
	if !it.Valid() {
		t.Fatal("iterator should be valid")
	}

	key, err := it.Key()
	if err != nil || key != "key1" {
		t.Errorf("expected key1, got %s (err=%v)", key, err)
	}

	val, err := it.Value()
	if err != nil || val != "value1" {
		t.Errorf("expected value1, got %s (err=%v)", val, err)
	}
}

func TestIterator_InvalidKey(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()

	_, err := it.Key()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid, got %v", err)
	}
}

func TestIterator_InvalidValue(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()

	_, err := it.Value()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid, got %v", err)
	}
}

func TestIterator_NextOnInvalid(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()

	err := it.Next()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid, got %v", err)
	}
}

func TestIterator_PrevOnInvalid(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()

	err := it.Prev()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid, got %v", err)
	}
}

func TestIterator_PrevBasic(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()

	it.Next()
	it.Next()

	key, _ := it.Key()
	if key != "c" {
		t.Fatalf("expected to be at 'c', got %s", key)
	}

	err := it.Prev()
	if err != nil {
		t.Fatalf("Prev() error: %v", err)
	}

	key, _ = it.Key()
	if key != "b" {
		t.Errorf("expected 'b' after Prev, got %s", key)
	}

	err = it.Prev()
	if err != nil {
		t.Fatalf("Prev() error: %v", err)
	}

	key, _ = it.Key()
	if key != "a" {
		t.Errorf("expected 'a' after second Prev, got %s", key)
	}

	err = it.Prev()
	if err != ErrIteratorDone {
		t.Errorf("expected ErrIteratorDone at beginning, got %v", err)
	}
}

func TestIterator_PrevCrossNode(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	it := tree.NewIterator()
	for it.Valid() {
		it.Next()
	}

	var keys []string
	if it.Valid() || true {
		it2 := tree.NewIteratorAt("k9")
		if it2.Valid() {
			key, _ := it2.Key()
			keys = append(keys, key)
			for {
				err := it2.Prev()
				if err != nil {
					break
				}
				key, _ = it2.Key()
				keys = append(keys, key)
			}
		}
	}

	expected := []string{"k9", "k8", "k7", "k6", "k5", "k4", "k3", "k2", "k1", "k0"}
	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys in reverse, got %d", len(expected), len(keys))
	}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("position %d: expected %s, got %s", i, k, keys[i])
		}
	}
}

func TestIterator_NextExhaustion(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")

	it := tree.NewIterator()
	err := it.Next()
	if err != ErrIteratorDone {
		t.Errorf("expected ErrIteratorDone after last element, got %v", err)
	}
	if it.Valid() {
		t.Error("iterator should be invalid after exhaustion")
	}
}

func TestIterator_DeleteCurrent(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()

	key, _ := it.Key()
	if key != "a" {
		t.Fatalf("expected to be at 'a', got %s", key)
	}

	err := it.Delete()
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if tree.Count() != 2 {
		t.Errorf("expected count 2 after delete, got %d", tree.Count())
	}

	_, ok := tree.Search("a")
	if ok {
		t.Error("expected 'a' to be deleted")
	}

	if it.Valid() {
		key, _ := it.Key()
		if key != "b" {
			t.Errorf("expected iterator to point to 'b' after delete, got %s", key)
		}
	}
}

func TestIterator_DeleteMiddle(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()
	it.Next()

	key, _ := it.Key()
	if key != "b" {
		t.Fatalf("expected to be at 'b', got %s", key)
	}

	err := it.Delete()
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if tree.Count() != 2 {
		t.Errorf("expected count 2 after delete, got %d", tree.Count())
	}

	if it.Valid() {
		key, _ := it.Key()
		if key != "c" {
			t.Errorf("expected iterator to point to 'c' after deleting 'b', got %s", key)
		}
	}
}

func TestIterator_DeleteLastInNode(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")

	it := tree.NewIteratorAt("e")
	if !it.Valid() {
		t.Fatal("iterator should be valid at 'e'")
	}

	key, _ := it.Key()
	if key != "e" {
		t.Fatalf("expected to be at 'e', got %s", key)
	}

	err := it.Delete()
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	if tree.Count() != 4 {
		t.Errorf("expected count 4 after delete, got %d", tree.Count())
	}

	_, ok := tree.Search("e")
	if ok {
		t.Error("expected 'e' to be deleted")
	}
}

func TestIterator_DeleteAndContinueForward(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	it := tree.NewIterator()
	var remaining []string
	for it.Valid() {
		key, _ := it.Key()
		if key == "k3" || key == "k7" {
			it.Delete()
			continue
		}
		remaining = append(remaining, key)
		it.Next()
	}

	if len(remaining) != 8 {
		t.Errorf("expected 8 remaining keys, got %d", len(remaining))
	}

	_, ok := tree.Search("k3")
	if ok {
		t.Error("k3 should be deleted")
	}
	_, ok = tree.Search("k7")
	if ok {
		t.Error("k7 should be deleted")
	}
}

func TestIterator_DeleteOnInvalid(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIterator()

	err := it.Delete()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid, got %v", err)
	}
}

func TestIteratorAt_Basic(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")

	it := tree.NewIteratorAt("c")
	if !it.Valid() {
		t.Fatal("iterator should be valid at 'c'")
	}

	key, _ := it.Key()
	if key != "c" {
		t.Errorf("expected key 'c', got %s", key)
	}
}

func TestIteratorAt_NonExistent(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("c", "3")
	tree.Insert("e", "5")

	it := tree.NewIteratorAt("b")
	if !it.Valid() {
		t.Fatal("iterator should be valid (positioned at 'c')")
	}

	key, _ := it.Key()
	if key != "c" {
		t.Errorf("expected to position at 'c' (first key >= 'b'), got %s", key)
	}
}

func TestIteratorAt_AfterAll(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")

	it := tree.NewIteratorAt("z")
	if it.Valid() {
		t.Error("iterator should be invalid for key after all existing keys")
	}
}

func TestIteratorAt_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	it := tree.NewIteratorAt("anything")
	if it.Valid() {
		t.Error("iterator should be invalid on empty tree")
	}
}

func TestIterator_WithSplits(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 30
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%03d", i), fmt.Sprintf("val%03d", i))
	}

	it := tree.NewIterator()
	var keys []string
	for it.Valid() {
		key, _ := it.Key()
		keys = append(keys, key)
		it.Next()
	}

	if len(keys) != n {
		t.Fatalf("expected %d keys from iterator, got %d", n, len(keys))
	}

	for i := 0; i < n; i++ {
		expected := fmt.Sprintf("key%03d", i)
		if keys[i] != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, keys[i])
		}
	}
}

func TestString_EmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	s := tree.String()
	if s != "(empty tree)" {
		t.Errorf("expected '(empty tree)', got %s", s)
	}
}

func TestString_NonEmptyTree(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")

	s := tree.String()
	if s == "" {
		t.Error("expected non-empty string representation")
	}
	if s == "(empty tree)" {
		t.Error("tree should not be represented as empty")
	}
}

func TestCount(t *testing.T) {
	tree := NewBPlusTree()
	if tree.Count() != 0 {
		t.Errorf("expected 0, got %d", tree.Count())
	}

	for i := 0; i < 100; i++ {
		tree.Insert(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if tree.Count() != 100 {
		t.Errorf("expected 100, got %d", tree.Count())
	}
}

func TestErrors_Values(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrInvalidRange == nil {
		t.Error("ErrInvalidRange should not be nil")
	}
	if ErrInvalidMaxKeys == nil {
		t.Error("ErrInvalidMaxKeys should not be nil")
	}
	if ErrIteratorInvalid == nil {
		t.Error("ErrIteratorInvalid should not be nil")
	}
	if ErrIteratorDone == nil {
		t.Error("ErrIteratorDone should not be nil")
	}
}

func TestLargeScale(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 8})

	n := 500
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("key%05d", i), fmt.Sprintf("val%05d", i))
	}

	if tree.Count() != n {
		t.Errorf("expected count %d, got %d", n, tree.Count())
	}

	for i := 0; i < n; i++ {
		key := fmt.Sprintf("key%05d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s not found", key)
			continue
		}
		expected := fmt.Sprintf("val%05d", i)
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}

	result, err := tree.RangeScan("key00100", "key00200")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 101 {
		t.Errorf("expected 101 items in range, got %d", len(result))
	}
}

func TestDeleteThenInsert(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	tree.Delete("b")
	tree.Insert("b", "new_2")

	val, ok := tree.Search("b")
	if !ok || val != "new_2" {
		t.Errorf("expected b=new_2, got %s (ok=%v)", val, ok)
	}

	if tree.Count() != 3 {
		t.Errorf("expected count 3, got %d", tree.Count())
	}
}

func TestRangeScan_AfterDelete(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	tree.Delete("k3")
	tree.Delete("k7")

	result, err := tree.RangeScan("k0", "k9")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	if len(result) != 8 {
		t.Errorf("expected 8 items after deleting 2, got %d", len(result))
	}

	for _, item := range result {
		if item.Key == "k3" || item.Key == "k7" {
			t.Errorf("deleted key %s should not appear in range scan", item.Key)
		}
	}
}

func TestIterator_ForwardBackward(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 8; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	it := tree.NewIterator()

	key, _ := it.Key()
	if key != "k0" {
		t.Fatalf("expected k0, got %s", key)
	}

	it.Next()
	key, _ = it.Key()
	if key != "k1" {
		t.Errorf("expected k1 after Next, got %s", key)
	}

	it.Prev()
	key, _ = it.Key()
	if key != "k0" {
		t.Errorf("expected k0 after Prev, got %s", key)
	}
}

func TestMinKeyOrderSplit(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 2})

	keys := []string{"e", "d", "c", "b", "a"}
	for _, k := range keys {
		tree.Insert(k, "val_"+k)
	}

	if tree.Count() != 5 {
		t.Errorf("expected count 5, got %d", tree.Count())
	}

	for _, k := range keys {
		val, ok := tree.Search(k)
		if !ok {
			t.Errorf("key %s not found", k)
			continue
		}
		if val != "val_"+k {
			t.Errorf("expected val_%s, got %s", k, val)
		}
	}
}

func TestIteratorAt_ExactKey(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	it := tree.NewIteratorAt("k5")
	if !it.Valid() {
		t.Fatal("iterator should be valid at 'k5'")
	}

	key, _ := it.Key()
	if key != "k5" {
		t.Errorf("expected k5, got %s", key)
	}
}

func TestRangeScan_SameStartEnd(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	result, err := tree.RangeScan("b", "b")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 item for single-key range, got %d", len(result))
	}
	if result[0].Key != "b" || result[0].Value != "2" {
		t.Errorf("expected {b,2}, got %v", result[0])
	}
}

func TestInsert_AfterDeletingAll(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Delete("a")
	tree.Delete("b")

	if tree.Count() != 0 {
		t.Errorf("expected count 0, got %d", tree.Count())
	}

	tree.Insert("x", "10")
	tree.Insert("y", "20")

	if tree.Count() != 2 {
		t.Errorf("expected count 2, got %d", tree.Count())
	}

	val, ok := tree.Search("x")
	if !ok || val != "10" {
		t.Errorf("expected x=10, got %s (ok=%v)", val, ok)
	}
}

func TestIterator_DeleteAllElements(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()
	count := 0
	for it.Valid() {
		it.Delete()
		count++
	}

	if count != 3 {
		t.Errorf("expected to delete 3 elements, got %d", count)
	}
	if tree.Count() != 0 {
		t.Errorf("expected count 0 after deleting all, got %d", tree.Count())
	}
}

func TestRangeScan_KVItemValues(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "alpha")
	tree.Insert("b", "beta")
	tree.Insert("c", "gamma")

	result, err := tree.RangeScan("a", "c")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	expectedKVs := []KVItem{
		{Key: "a", Value: "alpha"},
		{Key: "b", Value: "beta"},
		{Key: "c", Value: "gamma"},
	}

	if len(result) != len(expectedKVs) {
		t.Fatalf("expected %d items, got %d", len(expectedKVs), len(result))
	}

	for i, exp := range expectedKVs {
		if result[i].Key != exp.Key || result[i].Value != exp.Value {
			t.Errorf("position %d: expected {%s,%s}, got {%s,%s}", i, exp.Key, exp.Value, result[i].Key, result[i].Value)
		}
	}
}

func TestSplit_ManyDuplicatesWithSplit(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert("a", fmt.Sprintf("val_%d", i))
		tree.Insert(fmt.Sprintf("b%d", i), fmt.Sprintf("bv%d", i))
	}

	if tree.Count() != 11 {
		t.Errorf("expected 11 (1 dup 'a' + 10 unique b0-b9), got %d", tree.Count())
	}

	val, ok := tree.Search("a")
	if !ok || val != "val_9" {
		t.Errorf("expected a=val_9 (last update), got %s (ok=%v)", val, ok)
	}
}

func TestUnderflow_LeafBorrowFromLeft(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")
	tree.Insert("f", "6")
	tree.Insert("g", "7")
	tree.Insert("h", "8")
	tree.Insert("i", "9")

	if tree.Count() != 9 {
		t.Fatalf("expected 9 keys, got %d", tree.Count())
	}

	for i := 7; i >= 4; i-- {
		key := fmt.Sprintf("%c", 'a'+i)
		tree.Delete(key)
	}

	if tree.Count() != 5 {
		t.Errorf("expected 5 keys after deletions, got %d", tree.Count())
	}

	remaining := []string{"a", "b", "c", "d", "i"}
	for _, k := range remaining {
		_, ok := tree.Search(k)
		if !ok {
			t.Errorf("key %s should exist after borrow", k)
		}
	}

	deleted := []string{"e", "f", "g", "h"}
	for _, k := range deleted {
		_, ok := tree.Search(k)
		if ok {
			t.Errorf("key %s should be deleted", k)
		}
	}
}

func TestUnderflow_LeafBorrowFromRight(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")
	tree.Insert("d", "4")
	tree.Insert("e", "5")
	tree.Insert("f", "6")
	tree.Insert("g", "7")
	tree.Insert("h", "8")
	tree.Insert("i", "9")

	tree.Delete("a")
	tree.Delete("b")
	tree.Delete("c")
	tree.Delete("d")

	if tree.Count() != 5 {
		t.Errorf("expected 5 keys, got %d", tree.Count())
	}

	remaining := []string{"e", "f", "g", "h", "i"}
	for _, k := range remaining {
		_, ok := tree.Search(k)
		if !ok {
			t.Errorf("key %s should exist after borrow from right", k)
		}
	}

	result, err := tree.RangeScan("a", "z")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 5 {
		t.Errorf("expected 5 items in range, got %d", len(result))
	}
	for i, k := range remaining {
		if result[i].Key != k {
			t.Errorf("position %d: expected %s, got %s", i, k, result[i].Key)
		}
	}
}

func TestUnderflow_LeafMergeWithLeft(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 6
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	tree.Delete("k4")
	tree.Delete("k5")

	if tree.Count() != 4 {
		t.Errorf("expected 4 keys after merge-triggering deletions, got %d", tree.Count())
	}

	for i := 0; i < 4; i++ {
		key := fmt.Sprintf("k%d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after merge", key)
		}
	}
}

func TestUnderflow_LeafMergeWithRight(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 6
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	tree.Delete("k0")
	tree.Delete("k1")

	if tree.Count() != 4 {
		t.Errorf("expected 4 keys, got %d", tree.Count())
	}

	for i := 2; i < 6; i++ {
		key := fmt.Sprintf("k%d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after merge with right", key)
		}
	}
}

func TestUnderflow_InternalNodeBorrow(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 30
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d", i))
	}

	for i := 0; i < 5; i++ {
		tree.Delete(fmt.Sprintf("k%03d", i))
	}

	if tree.Count() != 25 {
		t.Errorf("expected 25 keys, got %d", tree.Count())
	}

	for i := 5; i < n; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after internal borrow", key)
		}
	}

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if ok {
			t.Errorf("key %s should not exist", key)
		}
	}
}

func TestUnderflow_InternalNodeMerge(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 20
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d", i))
	}

	for i := 0; i < 8; i++ {
		tree.Delete(fmt.Sprintf("k%03d", i))
	}

	if tree.Count() != 12 {
		t.Errorf("expected 12 keys, got %d", tree.Count())
	}

	for i := 8; i < n; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after internal merge", key)
		}
	}

	result, err := tree.RangeScan("k000", "k999")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 12 {
		t.Errorf("expected 12 items in range, got %d", len(result))
	}
}

func TestUnderflow_RootShrinks(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 6; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	if tree.root.isLeaf {
		t.Error("root should be internal node after 6 inserts with maxKeys=4")
	}

	tree.Delete("k0")
	tree.Delete("k1")
	tree.Delete("k2")
	tree.Delete("k3")

	if !tree.root.isLeaf {
		t.Error("root should have shrunk back to leaf after sufficient deletions")
	}

	if tree.Count() != 2 {
		t.Errorf("expected 2 keys, got %d", tree.Count())
	}

	for i := 4; i < 6; i++ {
		key := fmt.Sprintf("k%d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after root shrink", key)
		}
	}
}

func TestUnderflow_MultipleCascadeMerges(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 50
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d", i))
	}

	for i := 0; i < 35; i++ {
		key := fmt.Sprintf("k%03d", i)
		tree.Delete(key)
	}

	if tree.Count() != 15 {
		t.Errorf("expected 15 keys after cascade deletions, got %d", tree.Count())
	}

	for i := 35; i < n; i++ {
		key := fmt.Sprintf("k%03d", i)
		val, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist after cascade merges", key)
			continue
		}
		expected := fmt.Sprintf("v%03d", i)
		if val != expected {
			t.Errorf("expected %s for key %s, got %s", expected, key, val)
		}
	}

	for i := 0; i < 35; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if ok {
			t.Errorf("key %s should be deleted", key)
		}
	}

	result, err := tree.RangeScan("k000", "k999")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 15 {
		t.Errorf("expected 15 items in range, got %d", len(result))
	}
	for idx, item := range result {
		expected := fmt.Sprintf("k%03d", idx+35)
		if item.Key != expected {
			t.Errorf("position %d: expected %s, got %s", idx, expected, item.Key)
		}
	}
}

func TestUnderflow_DeleteFromRightSide(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 30
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d", i))
	}

	for i := n - 1; i >= 10; i-- {
		tree.Delete(fmt.Sprintf("k%03d", i))
	}

	if tree.Count() != 10 {
		t.Errorf("expected 10 keys, got %d", tree.Count())
	}

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if !ok {
			t.Errorf("key %s should exist", key)
		}
	}
}

func TestUnderflow_IteratorDeleteWithMerges(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	n := 20
	for i := 0; i < n; i++ {
		tree.Insert(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d", i))
	}

	it := tree.NewIterator()
	count := 0
	var remaining []string
	for it.Valid() {
		key, _ := it.Key()
		if count%2 == 0 {
			it.Delete()
		} else {
			remaining = append(remaining, key)
			it.Next()
		}
		count++
	}

	if tree.Count() != 10 {
		t.Errorf("expected 10 keys after deleting every other, got %d", tree.Count())
	}

	if len(remaining) != 10 {
		t.Errorf("expected 10 remaining keys from iteration, got %d", len(remaining))
	}

	for _, k := range remaining {
		_, ok := tree.Search(k)
		if !ok {
			t.Errorf("key %s should exist", k)
		}
	}

	expectedCount := 0
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("k%03d", i)
		_, ok := tree.Search(key)
		if ok {
			expectedCount++
		}
	}
	if expectedCount != 10 {
		t.Errorf("expected 10 existing keys in tree, found %d", expectedCount)
	}
}

func TestUnderflow_LeafLinkedListAfterMerge(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for i := 0; i < 10; i++ {
		tree.Insert(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i))
	}

	tree.Delete("k0")
	tree.Delete("k1")
	tree.Delete("k2")
	tree.Delete("k3")
	tree.Delete("k4")

	current := tree.root
	for !current.isLeaf {
		current = current.children[0]
	}

	var collected []string
	for current != nil {
		collected = append(collected, current.keys...)
		current = current.next
	}

	expected := []string{"k5", "k6", "k7", "k8", "k9"}
	if len(collected) != len(expected) {
		t.Fatalf("expected %d keys in leaf chain, got %d", len(expected), len(collected))
	}
	for i, k := range expected {
		if collected[i] != k {
			t.Errorf("position %d: expected %s, got %s", i, k, collected[i])
		}
	}

	current = tree.root
	for !current.isLeaf {
		current = current.children[len(current.children)-1]
	}
	var backward []string
	for current != nil {
		for i := len(current.keys) - 1; i >= 0; i-- {
			backward = append(backward, current.keys[i])
		}
		current = current.prev
	}

	if len(backward) != len(expected) {
		t.Fatalf("expected %d keys in backward traversal, got %d", len(expected), len(backward))
	}
	for i := range expected {
		if backward[i] != expected[len(expected)-1-i] {
			t.Errorf("backward position %d: expected %s, got %s", i, expected[len(expected)-1-i], backward[i])
		}
	}
}

func TestIteratorDelete_ReturnsErrIteratorInvalid(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")

	it := tree.NewIterator()
	it.Next()
	if it.Valid() {
		t.Error("iterator should be invalid after Next past last element")
	}

	err := it.Delete()
	if err != ErrIteratorInvalid {
		t.Errorf("expected ErrIteratorInvalid on invalid iterator, got %v", err)
	}
}

func TestIteratorDelete_ReturnsErrKeyNotFound(t *testing.T) {
	tree := NewBPlusTree()
	tree.Insert("a", "1")
	tree.Insert("b", "2")
	tree.Insert("c", "3")

	it := tree.NewIterator()
	it.Next()
	it.Next()
	if !it.Valid() {
		t.Fatal("expected valid iterator at c")
	}
	key, _ := it.Key()
	if key != "c" {
		t.Fatalf("expected key 'c', got %s", key)
	}

	deleted := tree.Delete("c")
	if !deleted {
		t.Fatal("expected tree.Delete to succeed")
	}

	err := it.Delete()
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound when index out of range after tree.Delete, got %v", err)
	}
	if it.Valid() {
		t.Error("iterator should be invalid after ErrKeyNotFound")
	}
}

func TestUnderflow_MinKeysCalculation(t *testing.T) {
	cases := []struct {
		maxKeys int
		minKeys int
	}{
		{4, 2},
		{6, 3},
		{8, 4},
		{32, 16},
		{2, 1},
	}

	for _, c := range cases {
		tree := NewBPlusTreeWithConfig(Config{MaxKeys: c.maxKeys})
		if tree.minKeys() != c.minKeys {
			t.Errorf("maxKeys=%d: expected minKeys=%d, got %d", c.maxKeys, c.minKeys, tree.minKeys())
		}
	}
}

func TestUnderflow_AlternatingInsertDelete(t *testing.T) {
	tree := NewBPlusTreeWithConfig(Config{MaxKeys: 4})

	for phase := 0; phase < 5; phase++ {
		base := phase * 10
		for i := 0; i < 10; i++ {
			tree.Insert(fmt.Sprintf("k%03d", base+i), fmt.Sprintf("v%03d", base+i))
		}

		for i := 0; i < 5; i++ {
			tree.Delete(fmt.Sprintf("k%03d", base+i))
		}

		if tree.Count() != (phase+1)*5 {
			t.Errorf("phase %d: expected %d keys, got %d", phase, (phase+1)*5, tree.Count())
		}
	}

	result, err := tree.RangeScan("k000", "k999")
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result) != 25 {
		t.Errorf("expected 25 items in total, got %d", len(result))
	}

	for _, item := range result {
		val, ok := tree.Search(item.Key)
		if !ok {
			t.Errorf("key %s from RangeScan should exist in tree", item.Key)
			continue
		}
		if val != item.Value {
			t.Errorf("value mismatch for key %s: expected %s, got %s", item.Key, item.Value, val)
		}
	}
}
