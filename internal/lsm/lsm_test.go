package lsm

import (
	"fmt"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func getTestConfig(t *testing.T) Config {
	t.Helper()
	tmpDir := t.TempDir()
	return Config{
		MemTableSize:   1024,
		MaxLevel:       4,
		LevelMaxFiles:  []int{2, 4, 8, 16},
		TargetFileSize: 512,
		DataDir:        tmpDir,
	}
}

func TestNewDB(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("NewDB returned nil")
	}
	if db.IsClosed() {
		t.Error("new DB should not be closed")
	}
	if db.MemTable() == nil {
		t.Error("memTable should not be nil")
	}
	if len(db.Levels()) != 4 {
		t.Errorf("expected 4 levels, got %d", len(db.Levels()))
	}
}

func TestNewDB_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	config := Config{
		DataDir: tmpDir,
	}
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB with empty config failed: %v", err)
	}
	defer db.Close()

	if db.config.MemTableSize <= 0 {
		t.Error("MemTableSize should have default value")
	}
	if db.config.MaxLevel <= 0 {
		t.Error("MaxLevel should have default value")
	}
}

func TestDB_PutAndGet(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = db.Put("key2", "value2")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := db.Get("key1")
	if err != nil {
		t.Fatalf("Get key1 failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	val, err = db.Get("key2")
	if err != nil {
		t.Fatalf("Get key2 failed: %v", err)
	}
	if val != "value2" {
		t.Errorf("expected value2, got %s", val)
	}
}

func TestDB_Put_EmptyKey(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Put("", "value")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestDB_Get_NotFound(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	_, err = db.Get("nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestDB_Get_EmptyKey(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	_, err = db.Get("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestDB_Update(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Put("key", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = db.Put("key", "value2")
	if err != nil {
		t.Fatalf("Update Put failed: %v", err)
	}

	val, err := db.Get("key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value2" {
		t.Errorf("expected updated value value2, got %s", val)
	}
}

func TestDB_Delete(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Put("key", "value")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := db.Get("key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "value" {
		t.Errorf("expected value, got %s", val)
	}

	err = db.Delete("key")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = db.Get("key")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
	}
}

func TestDB_Delete_EmptyKey(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Delete("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestDB_Delete_Nonexistent(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	err = db.Delete("nonexistent")
	if err != nil {
		t.Fatalf("Delete nonexistent key failed: %v", err)
	}
}

func TestDB_Range(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	keys := []string{"a", "b", "c", "d", "e"}
	for _, k := range keys {
		err = db.Put(k, "value_"+k)
		if err != nil {
			t.Fatalf("Put %s failed: %v", k, err)
		}
	}

	entries, err := db.Range("b", "d")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	expectedKeys := []string{"b", "c", "d"}
	for i, entry := range entries {
		if entry.Key != expectedKeys[i] {
			t.Errorf("expected key %s, got %s", expectedKeys[i], entry.Key)
		}
		if entry.Value != "value_"+expectedKeys[i] {
			t.Errorf("expected value value_%s, got %s", expectedKeys[i], entry.Value)
		}
	}
}

func TestDB_Range_Invalid(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	_, err = db.Range("z", "a")
	if err != ErrInvalidRange {
		t.Errorf("expected ErrInvalidRange, got %v", err)
	}
}

func TestDB_Range_EmptyResult(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	db.Put("a", "1")
	db.Put("b", "2")

	entries, err := db.Range("x", "z")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestDB_Range_SingleKey(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	db.Put("a", "1")
	db.Put("b", "2")
	db.Put("c", "3")

	entries, err := db.Range("b", "b")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Key != "b" || entries[0].Value != "2" {
		t.Errorf("expected b=2, got %s=%s", entries[0].Key, entries[0].Value)
	}
}

func TestDB_FlushMemTable(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 200
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%03d", i)
		value := fmt.Sprintf("value_%03d", i)
		err = db.Put(key, value)
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	l0Tables := db.Levels()[0].Tables()
	if len(l0Tables) == 0 {
		t.Error("expected at least one SSTable in L0 after flush")
	}

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%03d", i)
		expected := fmt.Sprintf("value_%03d", i)
		val, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}
}

func TestDB_Compaction(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 100
	config.LevelMaxFiles = []int{2, 4, 8, 16}
	config.TargetFileSize = 200
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	value := "this_is_a_long_value_to_make_entries_larger"
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, value)
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%03d", i)
		val, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val != value {
			t.Errorf("expected %s, got %s", value, val)
		}
	}

	l0Size := db.Levels()[0].Size()
	l1Size := db.Levels()[1].Size()
	t.Logf("L0 tables: %d, L1 tables: %d", l0Size, l1Size)
}

func TestDB_DeleteWithCompaction(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 100
	config.LevelMaxFiles = []int{2, 4, 8, 16}
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Delete(key)
		if err != nil {
			t.Fatalf("Delete %s failed: %v", key, err)
		}
	}

	time.Sleep(300 * time.Millisecond)

	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key_%03d", i)
		_, err = db.Get(key)
		if err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound for %s, got %v", key, err)
		}
	}

	for i := 15; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		expected := fmt.Sprintf("value_%03d", i)
		val, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}
}

func TestDB_RangeAfterFlush(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 200
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	entries, err := db.Range("key_010", "key_020")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(entries) != 11 {
		t.Errorf("expected 11 entries, got %d", len(entries))
	}

	for i, entry := range entries {
		expectedKey := fmt.Sprintf("key_%03d", i+10)
		expectedValue := fmt.Sprintf("value_%03d", i+10)
		if entry.Key != expectedKey {
			t.Errorf("expected key %s, got %s", expectedKey, entry.Key)
		}
		if entry.Value != expectedValue {
			t.Errorf("expected value %s, got %s", expectedValue, entry.Value)
		}
	}
}

func TestDB_ConcurrentPut(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 4096
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("goroutine_%d_key_%d", goroutineID, i)
				value := fmt.Sprintf("goroutine_%d_value_%d", goroutineID, i)
				err := db.Put(key, value)
				if err != nil {
					t.Errorf("Put failed: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	for g := 0; g < numGoroutines; g++ {
		for i := 0; i < numOps; i++ {
			key := fmt.Sprintf("goroutine_%d_key_%d", g, i)
			expected := fmt.Sprintf("goroutine_%d_value_%d", g, i)
			val, err := db.Get(key)
			if err != nil {
				t.Fatalf("Get %s failed: %v", key, err)
			}
			if val != expected {
				t.Errorf("expected %s, got %s", expected, val)
			}
		}
	}
}

func TestDB_ConcurrentPutAndGet(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 4096
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	numEntries := 100
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key_%d", i)
		db.Put(key, fmt.Sprintf("value_%d", i))
	}

	var wg sync.WaitGroup
	var readErrors int64
	var writeErrors int64

	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				key := fmt.Sprintf("new_key_%d", i)
				err := db.Put(key, fmt.Sprintf("new_value_%d", i))
				if err != nil {
					atomic.AddInt64(&writeErrors, 1)
				}
			}
		}()
	}

	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numEntries; i++ {
				key := fmt.Sprintf("key_%d", i)
				_, err := db.Get(key)
				if err != nil && err != ErrKeyNotFound {
					atomic.AddInt64(&readErrors, 1)
				}
			}
		}()
	}

	wg.Wait()

	if readErrors > 0 {
		t.Errorf("got %d read errors", readErrors)
	}
	if writeErrors > 0 {
		t.Errorf("got %d write errors", writeErrors)
	}
}

func TestDB_Close(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}

	err = db.Put("key", "value")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if !db.IsClosed() {
		t.Error("DB should be closed after Close()")
	}

	err = db.Put("another", "value")
	if err != ErrDBClosed {
		t.Errorf("expected ErrDBClosed, got %v", err)
	}

	_, err = db.Get("key")
	if err != ErrDBClosed {
		t.Errorf("expected ErrDBClosed, got %v", err)
	}

	err = db.Delete("key")
	if err != ErrDBClosed {
		t.Errorf("expected ErrDBClosed, got %v", err)
	}

	_, err = db.Range("a", "z")
	if err != ErrDBClosed {
		t.Errorf("expected ErrDBClosed, got %v", err)
	}
}

func TestDB_CloseTwice(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("First Close failed: %v", err)
	}

	err = db.Close()
	if err != nil {
		t.Fatalf("Second Close should not return error, got %v", err)
	}
}

func TestDB_LoadExistingSSTables(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	err = db.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	db2, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB for existing data failed: %v", err)
	}
	defer db2.Close()

	l0Tables := db2.Levels()[0].Tables()
	if len(l0Tables) == 0 {
		t.Error("expected SSTables to be loaded from disk")
	}

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("key_%03d", i)
		expected := fmt.Sprintf("value_%03d", i)
		val, err := db2.Get(key)
		if err != nil {
			t.Fatalf("Get %s from reloaded DB failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}
}

func TestDB_UpdateAfterFlush(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 200
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("old_value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("new_value_%03d", i))
		if err != nil {
			t.Fatalf("Update %s failed: %v", key, err)
		}
	}

	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		expected := fmt.Sprintf("new_value_%03d", i)
		val, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}
}

func TestDB_RangeIncludesMemTableAndSSTables(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 200
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	for i := 0; i < 30; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("sst_value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(200 * time.Millisecond)

	for i := 25; i < 35; i++ {
		key := fmt.Sprintf("key_%03d", i)
		err = db.Put(key, fmt.Sprintf("mem_value_%03d", i))
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	entries, err := db.Range("key_020", "key_034")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(entries) != 15 {
		t.Errorf("expected 15 entries, got %d", len(entries))
	}

	for i, entry := range entries {
		keyNum := i + 20
		expectedKey := fmt.Sprintf("key_%03d", keyNum)
		var expectedValue string
		if keyNum < 25 {
			expectedValue = fmt.Sprintf("sst_value_%03d", keyNum)
		} else {
			expectedValue = fmt.Sprintf("mem_value_%03d", keyNum)
		}

		if entry.Key != expectedKey {
			t.Errorf("expected key %s, got %s", expectedKey, entry.Key)
		}
		if entry.Value != expectedValue {
			t.Errorf("expected value %s, got %s", expectedValue, entry.Value)
		}
	}
}

func TestMemTable_Basic(t *testing.T) {
	mt := NewMemTable(1024)

	mt.Put("a", "1")
	mt.Put("b", "2")
	mt.Put("c", "3")

	if mt.Len() != 3 {
		t.Errorf("expected 3 entries, got %d", mt.Len())
	}

	entry, found := mt.Get("b")
	if !found {
		t.Error("expected to find key b")
	}
	if entry.Value != "2" {
		t.Errorf("expected value 2, got %s", entry.Value)
	}

	mt.Delete("b")
	_, found = mt.Get("b")
	if found {
		t.Error("expected key b to be deleted")
	}

	mt.Put("a", "updated")
	entry, found = mt.Get("a")
	if !found {
		t.Error("expected to find key a")
	}
	if entry.Value != "updated" {
		t.Errorf("expected updated value, got %s", entry.Value)
	}
}

func TestMemTable_ShouldFlush(t *testing.T) {
	mt := NewMemTable(100)

	mt.Put("key", "value")
	if mt.ShouldFlush() {
		t.Error("should not need flush yet")
	}

	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		mt.Put(key, value)
		if mt.ShouldFlush() {
			break
		}
	}

	if !mt.ShouldFlush() {
		t.Error("should need flush after many inserts")
	}
}

func TestMemTable_Range(t *testing.T) {
	mt := NewMemTable(1024)

	keys := []string{"a", "b", "c", "d", "e"}
	for _, k := range keys {
		mt.Put(k, "val_"+k)
	}

	entries := mt.Range("b", "d")
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	expectedKeys := []string{"b", "c", "d"}
	for i, e := range entries {
		if e.Key != expectedKeys[i] {
			t.Errorf("expected key %s, got %s", expectedKeys[i], e.Key)
		}
	}
}

func TestMemTable_Freeze(t *testing.T) {
	mt := NewMemTable(1024)

	if mt.IsFrozen() {
		t.Error("new memtable should not be frozen")
	}

	mt.Freeze()
	if !mt.IsFrozen() {
		t.Error("memtable should be frozen after Freeze()")
	}
}

func TestSkipList_Basic(t *testing.T) {
	sl := NewSkipList()

	entry1 := &Entry{Key: "b", Value: "2", Timestamp: 1}
	entry2 := &Entry{Key: "a", Value: "1", Timestamp: 1}
	entry3 := &Entry{Key: "c", Value: "3", Timestamp: 1}

	sl.Insert(entry1)
	sl.Insert(entry2)
	sl.Insert(entry3)

	if sl.Len() != 3 {
		t.Errorf("expected 3 entries, got %d", sl.Len())
	}

	entry, found := sl.Get("a")
	if !found {
		t.Error("expected to find key a")
	}
	if entry.Value != "1" {
		t.Errorf("expected value 1, got %s", entry.Value)
	}

	updatedEntry := &Entry{Key: "a", Value: "updated", Timestamp: 2}
	sl.Insert(updatedEntry)

	if sl.Len() != 3 {
		t.Errorf("expected 3 entries after update, got %d", sl.Len())
	}

	entry, found = sl.Get("a")
	if !found {
		t.Error("expected to find key a")
	}
	if entry.Value != "updated" {
		t.Errorf("expected updated value, got %s", entry.Value)
	}

	deleted, ok := sl.Delete("b")
	if !ok {
		t.Error("expected Delete to succeed")
	}
	if deleted.Key != "b" {
		t.Errorf("expected deleted key b, got %s", deleted.Key)
	}

	if sl.Len() != 2 {
		t.Errorf("expected 2 entries after delete, got %d", sl.Len())
	}
}

func TestSkipList_Range(t *testing.T) {
	sl := NewSkipList()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%03d", i)
		entry := &Entry{Key: key, Value: fmt.Sprintf("val_%d", i), Timestamp: 1}
		sl.Insert(entry)
	}

	entries := sl.Range("key_002", "key_005")
	if len(entries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(entries))
	}

	expectedKeys := []string{"key_002", "key_003", "key_004", "key_005"}
	for i, e := range entries {
		if e.Key != expectedKeys[i] {
			t.Errorf("expected key %s, got %s", expectedKeys[i], e.Key)
		}
	}
}

func TestSkipList_Iterator(t *testing.T) {
	sl := NewSkipList()

	keys := []string{"a", "b", "c"}
	for _, k := range keys {
		entry := &Entry{Key: k, Value: "val_" + k, Timestamp: 1}
		sl.Insert(entry)
	}

	iter := sl.Iterator()
	defer iter.Close()
	var result []string
	for iter.Next() {
		result = append(result, iter.Entry().Key)
	}

	if len(result) != 3 {
		t.Errorf("expected 3 entries, got %d", len(result))
	}

	for i, k := range keys {
		if result[i] != k {
			t.Errorf("expected %s, got %s", k, result[i])
		}
	}
}

func TestSkipList_Seek(t *testing.T) {
	sl := NewSkipList()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key_%03d", i)
		entry := &Entry{Key: key, Value: fmt.Sprintf("val_%d", i), Timestamp: 1}
		sl.Insert(entry)
	}

	iter := sl.Iterator()
	defer iter.Close()
	iter.Seek("key_005")

	count := 0
	for iter.Next() {
		count++
	}

	if count != 5 {
		t.Errorf("expected 5 entries from seek position, got %d", count)
	}
}

func TestSSTable_CreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.sst")

	entries := []*Entry{
		{Key: "a", Value: "1", Timestamp: 1},
		{Key: "b", Value: "2", Timestamp: 1},
		{Key: "c", Value: "3", Timestamp: 1},
	}

	sst, err := NewSSTable(filename, 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	if sst.EntryCount() != 3 {
		t.Errorf("expected 3 entries, got %d", sst.EntryCount())
	}
	if sst.MinKey() != "a" {
		t.Errorf("expected min key a, got %s", sst.MinKey())
	}
	if sst.MaxKey() != "c" {
		t.Errorf("expected max key c, got %s", sst.MaxKey())
	}

	entry, found, err := sst.Get("b")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected to find key b")
	}
	if entry.Value != "2" {
		t.Errorf("expected value 2, got %s", entry.Value)
	}

	loaded, err := LoadSSTable(filename, 0)
	if err != nil {
		t.Fatalf("LoadSSTable failed: %v", err)
	}

	if loaded.EntryCount() != 3 {
		t.Errorf("expected 3 entries in loaded SSTable, got %d", loaded.EntryCount())
	}

	entry, found, err = loaded.Get("c")
	if err != nil {
		t.Fatalf("Get from loaded SSTable failed: %v", err)
	}
	if !found {
		t.Error("expected to find key c")
	}
	if entry.Value != "3" {
		t.Errorf("expected value 3, got %s", entry.Value)
	}
}

func TestSSTable_Range(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.sst")

	var entries []*Entry
	for i := 0; i < 10; i++ {
		entries = append(entries, &Entry{
			Key:       fmt.Sprintf("key_%03d", i),
			Value:     fmt.Sprintf("val_%d", i),
			Timestamp: 1,
		})
	}

	sst, err := NewSSTable(filename, 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	rangeEntries, err := sst.Range("key_002", "key_005")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(rangeEntries) != 4 {
		t.Errorf("expected 4 entries, got %d", len(rangeEntries))
	}
}

func TestSSTable_Tombstone(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.sst")

	entries := []*Entry{
		{Key: "a", Value: "1", Timestamp: 1},
		{Key: "b", Value: "", Tombstone: true, Timestamp: 2},
		{Key: "c", Value: "3", Timestamp: 1},
	}

	sst, err := NewSSTable(filename, 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entry, found, err := sst.Get("b")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected to find tombstone for key b")
	}
	if !entry.Tombstone {
		t.Error("expected entry to be tombstone")
	}

	rangeEntries, err := sst.Range("a", "c")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(rangeEntries) != 2 {
		t.Errorf("expected 2 entries (tombstone excluded), got %d", len(rangeEntries))
	}
}

func TestSSTable_OverlapsWith(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "test.sst")

	entries := []*Entry{
		{Key: "c", Value: "1", Timestamp: 1},
		{Key: "e", Value: "2", Timestamp: 1},
	}

	sst, err := NewSSTable(filename, 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	if !sst.OverlapsWith("a", "d") {
		t.Error("should overlap with a-d")
	}

	if !sst.OverlapsWith("d", "f") {
		t.Error("should overlap with d-f")
	}

	if sst.OverlapsWith("a", "b") {
		t.Error("should not overlap with a-b")
	}

	if sst.OverlapsWith("f", "g") {
		t.Error("should not overlap with f-g")
	}
}

func TestLevel_Basic(t *testing.T) {
	level := NewLevel(0, 2)

	if level.Level() != 0 {
		t.Errorf("expected level 0, got %d", level.Level())
	}
	if level.MaxSize() != 2 {
		t.Errorf("expected max size 2, got %d", level.MaxSize())
	}
	if level.Size() != 0 {
		t.Errorf("expected size 0, got %d", level.Size())
	}
	if level.NeedsCompaction() {
		t.Error("should not need compaction when empty")
	}
}

func TestLevel_AddAndRemove(t *testing.T) {
	tmpDir := t.TempDir()

	level := NewLevel(0, 2)

	entries1 := []*Entry{{Key: "a", Value: "1", Timestamp: 1}}
	sst1, err := NewSSTable(filepath.Join(tmpDir, "L0_001.sst"), 0, entries1)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entries2 := []*Entry{{Key: "b", Value: "2", Timestamp: 1}}
	sst2, err := NewSSTable(filepath.Join(tmpDir, "L0_002.sst"), 0, entries2)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	level.AddTable(sst1)
	level.AddTable(sst2)

	if level.Size() != 2 {
		t.Errorf("expected size 2, got %d", level.Size())
	}
	if !level.NeedsCompaction() {
		t.Error("should need compaction at max size")
	}

	level.RemoveTable(sst1)
	if level.Size() != 1 {
		t.Errorf("expected size 1 after remove, got %d", level.Size())
	}
}

func TestLevel_Get_L0(t *testing.T) {
	tmpDir := t.TempDir()

	level := NewLevel(0, 4)

	entries1 := []*Entry{{Key: "a", Value: "old", Timestamp: 1}}
	sst1, err := NewSSTable(filepath.Join(tmpDir, "L0_001.sst"), 0, entries1)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entries2 := []*Entry{{Key: "a", Value: "new", Timestamp: 2}}
	sst2, err := NewSSTable(filepath.Join(tmpDir, "L0_002.sst"), 0, entries2)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	level.AddTable(sst1)
	level.AddTable(sst2)

	entry, found, err := level.Get("a")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected to find key a")
	}
	if entry.Value != "new" {
		t.Errorf("expected new value (from newer SSTable), got %s", entry.Value)
	}
}

func TestLevel_Get_NonL0(t *testing.T) {
	tmpDir := t.TempDir()

	level := NewLevel(1, 4)

	entries1 := []*Entry{
		{Key: "a", Value: "1", Timestamp: 1},
		{Key: "b", Value: "2", Timestamp: 1},
	}
	sst1, err := NewSSTable(filepath.Join(tmpDir, "L1_001.sst"), 1, entries1)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entries2 := []*Entry{
		{Key: "c", Value: "3", Timestamp: 1},
		{Key: "d", Value: "4", Timestamp: 1},
	}
	sst2, err := NewSSTable(filepath.Join(tmpDir, "L1_002.sst"), 1, entries2)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	level.AddTable(sst1)
	level.AddTable(sst2)

	entry, found, err := level.Get("c")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected to find key c")
	}
	if entry.Value != "3" {
		t.Errorf("expected value 3, got %s", entry.Value)
	}

	_, found, err = level.Get("e")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Error("should not find key e")
	}
}

func TestLevel_FindOverlappingTables(t *testing.T) {
	tmpDir := t.TempDir()

	level := NewLevel(1, 4)

	entries1 := []*Entry{
		{Key: "a", Value: "1", Timestamp: 1},
		{Key: "c", Value: "3", Timestamp: 1},
	}
	sst1, err := NewSSTable(filepath.Join(tmpDir, "L1_001.sst"), 1, entries1)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entries2 := []*Entry{
		{Key: "d", Value: "4", Timestamp: 1},
		{Key: "f", Value: "6", Timestamp: 1},
	}
	sst2, err := NewSSTable(filepath.Join(tmpDir, "L1_002.sst"), 1, entries2)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	entries3 := []*Entry{
		{Key: "g", Value: "7", Timestamp: 1},
		{Key: "i", Value: "9", Timestamp: 1},
	}
	sst3, err := NewSSTable(filepath.Join(tmpDir, "L1_003.sst"), 1, entries3)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	level.AddTable(sst1)
	level.AddTable(sst2)
	level.AddTable(sst3)

	overlapping := level.FindOverlappingTables("b", "e")
	if len(overlapping) != 2 {
		t.Errorf("expected 2 overlapping tables, got %d", len(overlapping))
	}
}

func TestEntry_EncodeDecode(t *testing.T) {
	entry := &Entry{
		Key:       "test_key",
		Value:     "test_value",
		Tombstone: false,
		Timestamp: 1234567890,
	}

	encoded := entry.Encode()
	decoded, read, err := DecodeEntry(encoded)
	if err != nil {
		t.Fatalf("DecodeEntry failed: %v", err)
	}

	if read != len(encoded) {
		t.Errorf("expected read %d, got %d", len(encoded), read)
	}

	if decoded.Key != entry.Key {
		t.Errorf("expected key %s, got %s", entry.Key, decoded.Key)
	}
	if decoded.Value != entry.Value {
		t.Errorf("expected value %s, got %s", entry.Value, decoded.Value)
	}
	if decoded.Tombstone != entry.Tombstone {
		t.Errorf("expected tombstone %v, got %v", entry.Tombstone, decoded.Tombstone)
	}
	if decoded.Timestamp != entry.Timestamp {
		t.Errorf("expected timestamp %d, got %d", entry.Timestamp, decoded.Timestamp)
	}
}

func TestEntry_Size(t *testing.T) {
	entry := &Entry{
		Key:       "abc",
		Value:     "defgh",
		Tombstone: false,
		Timestamp: 1,
	}

	expectedSize := len("abc") + len("defgh") + 1 + 8
	if entry.Size() != expectedSize {
		t.Errorf("expected size %d, got %d", expectedSize, entry.Size())
	}
}

func TestIndexEntry_EncodeDecode(t *testing.T) {
	ie := &IndexEntry{
		Key:      "index_key",
		Offset:   12345,
		EntryLen: 678,
	}

	encoded := ie.Encode()
	decoded, read, err := DecodeIndexEntry(encoded)
	if err != nil {
		t.Fatalf("DecodeIndexEntry failed: %v", err)
	}

	if read != len(encoded) {
		t.Errorf("expected read %d, got %d", len(encoded), read)
	}

	if decoded.Key != ie.Key {
		t.Errorf("expected key %s, got %s", ie.Key, decoded.Key)
	}
	if decoded.Offset != ie.Offset {
		t.Errorf("expected offset %d, got %d", ie.Offset, decoded.Offset)
	}
	if decoded.EntryLen != ie.EntryLen {
		t.Errorf("expected entryLen %d, got %d", ie.EntryLen, decoded.EntryLen)
	}
}

func TestConfig_Default(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MemTableSize != 1<<20 {
		t.Errorf("expected MemTableSize %d, got %d", 1<<20, cfg.MemTableSize)
	}
	if cfg.MaxLevel != 4 {
		t.Errorf("expected MaxLevel 4, got %d", cfg.MaxLevel)
	}
	if len(cfg.LevelMaxFiles) != 4 {
		t.Errorf("expected 4 LevelMaxFiles, got %d", len(cfg.LevelMaxFiles))
	}
	if cfg.TargetFileSize != 1<<19 {
		t.Errorf("expected TargetFileSize %d, got %d", 1<<19, cfg.TargetFileSize)
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	cfg.validate()

	if cfg.MemTableSize <= 0 {
		t.Error("MemTableSize should have default")
	}
	if cfg.MaxLevel <= 0 {
		t.Error("MaxLevel should have default")
	}
	if len(cfg.LevelMaxFiles) == 0 {
		t.Error("LevelMaxFiles should have default")
	}
	if cfg.TargetFileSize <= 0 {
		t.Error("TargetFileSize should have default")
	}
	if cfg.DataDir == "" {
		t.Error("DataDir should have default")
	}
}

func TestDB_DebugInfo(t *testing.T) {
	config := getTestConfig(t)
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	db.Put("key1", "value1")
	db.Put("key2", "value2")

	info := db.DebugInfo()
	if info == "" {
		t.Error("DebugInfo should not return empty string")
	}
	if !contains(info, "MemTable") {
		t.Error("DebugInfo should contain MemTable info")
	}
}

func TestLevel_DebugInfo(t *testing.T) {
	tmpDir := t.TempDir()
	level := NewLevel(0, 2)

	entries := []*Entry{{Key: "a", Value: "1", Timestamp: 1}}
	sst, err := NewSSTable(filepath.Join(tmpDir, "L0_001.sst"), 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}
	level.AddTable(sst)

	info := level.DebugInfo()
	if info == "" {
		t.Error("DebugInfo should not return empty string")
	}
}

func TestDB_LargeDataset(t *testing.T) {
	config := getTestConfig(t)
	config.MemTableSize = 1024
	config.LevelMaxFiles = []int{4, 8, 16, 32}
	config.TargetFileSize = 1024
	db, err := NewDB(config)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	defer db.Close()

	numEntries := 200
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key_%05d", i)
		value := fmt.Sprintf("value_%05d_this_is_a_somewhat_longer_value_to_fill_up_space", i)
		err = db.Put(key, value)
		if err != nil {
			t.Fatalf("Put %s failed: %v", key, err)
		}
	}

	time.Sleep(500 * time.Millisecond)

	sampleKeys := []int{0, 50, 100, 150, 199}
	for _, idx := range sampleKeys {
		key := fmt.Sprintf("key_%05d", idx)
		expected := fmt.Sprintf("value_%05d_this_is_a_somewhat_longer_value_to_fill_up_space", idx)
		val, err := db.Get(key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if val != expected {
			t.Errorf("expected %s, got %s", expected, val)
		}
	}

	entries, err := db.Range("key_00050", "key_00060")
	if err != nil {
		t.Fatalf("Range failed: %v", err)
	}

	if len(entries) != 11 {
		t.Errorf("expected 11 entries, got %d", len(entries))
	}

	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	if !sort.StringsAreSorted(keys) {
		t.Error("range results should be sorted by key")
	}
}

func TestGenerateSSTableFilename(t *testing.T) {
	filename := GenerateSSTableFilename("/tmp/test", 1, 42)
	expected := filepath.Join("/tmp/test", "L1_000042.sst")
	if filename != expected {
		t.Errorf("expected %s, got %s", expected, filename)
	}
}

func TestSSTable_EmptyEntries(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "empty.sst")

	_, err := NewSSTable(filename, 0, []*Entry{})
	if err == nil {
		t.Error("expected error when creating SSTable with empty entries")
	}
}

func TestSSTable_DuplicateKeys(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "dup.sst")

	entries := []*Entry{
		{Key: "a", Value: "old", Timestamp: 1},
		{Key: "a", Value: "new", Timestamp: 2},
	}

	sst, err := NewSSTable(filename, 0, entries)
	if err != nil {
		t.Fatalf("NewSSTable failed: %v", err)
	}

	if sst.EntryCount() != 1 {
		t.Errorf("expected 1 entry after dedup, got %d", sst.EntryCount())
	}

	entry, found, err := sst.Get("a")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected to find key a")
	}
	if entry.Value != "new" {
		t.Errorf("expected new value, got %s", entry.Value)
	}
	if entry.Timestamp != 2 {
		t.Errorf("expected timestamp 2, got %d", entry.Timestamp)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
