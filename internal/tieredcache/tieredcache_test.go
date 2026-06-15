package tieredcache

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.L1Config.Capacity != 1000 {
		t.Errorf("expected L1 capacity 1000, got %d", cfg.L1Config.Capacity)
	}
	if cfg.L1Config.CapacityMode != CapacityModeCount {
		t.Errorf("expected L1 capacity mode count, got %s", cfg.L1Config.CapacityMode)
	}
	if cfg.L2Config.Capacity != 100*1024*1024 {
		t.Errorf("expected L2 capacity 100MB, got %d", cfg.L2Config.Capacity)
	}
	if cfg.L2Config.CapacityMode != CapacityModeBytes {
		t.Errorf("expected L2 capacity mode bytes, got %s", cfg.L2Config.CapacityMode)
	}
	if cfg.WritePolicy != WritePolicyWriteThrough {
		t.Errorf("expected write through policy, got %s", cfg.WritePolicy)
	}
}

func TestNewTieredCache_Default(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	if tc == nil {
		t.Fatal("NewTieredCache returned nil")
	}
	if tc.L1Count() != 0 {
		t.Errorf("expected L1 count 0, got %d", tc.L1Count())
	}
	if tc.L2Count() != 0 {
		t.Errorf("expected L2 count 0, got %d", tc.L2Count())
	}
}

func TestNewTieredCacheWithConfig_InvalidL1Capacity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L1Config.Capacity = 0

	_, err := NewTieredCacheWithConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid L1 capacity")
	}
}

func TestNewTieredCacheWithConfig_InvalidL2Capacity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L2Config.Capacity = -1

	_, err := NewTieredCacheWithConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid L2 capacity")
	}
}

func TestNewTieredCacheWithConfig_InvalidPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WritePolicy = "invalid"

	_, err := NewTieredCacheWithConfig(cfg)
	if err != ErrInvalidPolicy {
		t.Errorf("expected ErrInvalidPolicy, got %v", err)
	}
}

func TestNewTieredCacheWithConfig_InvalidEvictionPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L1Config.EvictionPolicy = "invalid"

	_, err := NewTieredCacheWithConfig(cfg)
	if err == nil {
		t.Error("expected error for invalid eviction policy")
	}
}

func TestPutAndGet_WriteThrough(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("expected value1, got %s", string(val))
	}

	if tc.L1Count() != 1 {
		t.Errorf("expected L1 count 1, got %d", tc.L1Count())
	}
	if tc.L2Count() != 1 {
		t.Errorf("expected L2 count 1, got %d", tc.L2Count())
	}

	filename := filepath.Join(tmpDir, sanitizeKey("key1"))
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read L2 file: %v", err)
	}
	if string(data) != "value1" {
		t.Errorf("expected value1 in L2 file, got %s", string(data))
	}
}

func TestPutAndGet_WriteBack(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: 100 * time.Millisecond,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "value1" {
		t.Errorf("expected value1, got %s", string(val))
	}

	if tc.L1Count() != 1 {
		t.Errorf("expected L1 count 1, got %d", tc.L1Count())
	}
	if tc.L2Count() != 0 {
		t.Errorf("expected L2 count 0 before flush, got %d", tc.L2Count())
	}

	filename := filepath.Join(tmpDir, sanitizeKey("key1"))
	if _, err := os.Stat(filename); err == nil {
		t.Error("expected L2 file not to exist before flush")
	}

	tc.Flush()

	if tc.L2Count() != 1 {
		t.Errorf("expected L2 count 1 after flush, got %d", tc.L2Count())
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("failed to read L2 file after flush: %v", err)
	}
	if string(data) != "value1" {
		t.Errorf("expected value1 in L2 file, got %s", string(data))
	}
}

func TestCascadingQuery(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       10,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("value%d", i)
		err = tc.Put(key, []byte(val))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	if tc.L1Count() != 10 {
		t.Errorf("expected L1 count 10, got %d", tc.L1Count())
	}
	if tc.L2Count() != 15 {
		t.Errorf("expected L2 count 15, got %d", tc.L2Count())
	}

	val, err := tc.Get("key0")
	if err != nil {
		t.Fatalf("Get key0 failed: %v", err)
	}
	if string(val) != "value0" {
		t.Errorf("expected value0, got %s", string(val))
	}

	if tc.L1Count() != 10 {
		t.Errorf("expected L1 count still 10 after L2 hit, got %d", tc.L1Count())
	}

	_, err = tc.Get("key100")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
}

func TestLRUEviction_CountMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       3,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	tc.Put("key2", []byte("value2"))
	tc.Put("key3", []byte("value3"))

	if tc.L1Count() != 3 {
		t.Errorf("expected L1 count 3, got %d", tc.L1Count())
	}

	tc.Get("key1")
	tc.Put("key4", []byte("value4"))

	if tc.L1Count() != 3 {
		t.Errorf("expected L1 count still 3, got %d", tc.L1Count())
	}

	if tc.ContainsL1("key2") {
		t.Error("key2 should have been evicted from L1")
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Errorf("key1 should still exist, got err=%v", err)
	}
	if string(val) != "value1" {
		t.Errorf("expected value1, got %s", string(val))
	}

	val, err = tc.Get("key2")
	if err != nil {
		t.Fatalf("key2 should exist in L2, got err=%v", err)
	}
	if string(val) != "value2" {
		t.Errorf("expected value2 from L2, got %s", string(val))
	}
}

func TestLRUEviction_BytesMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       30,
			CapacityMode:   CapacityModeBytes,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeBytes,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("0123456789"))
	tc.Put("key2", []byte("0123456789"))
	tc.Put("key3", []byte("0123456789"))

	if tc.L1Size() != 30 {
		t.Errorf("expected L1 size 30, got %d", tc.L1Size())
	}

	tc.Get("key1")
	tc.Put("key4", []byte("0123456789"))

	if tc.L1Size() != 30 {
		t.Errorf("expected L1 size still 30, got %d", tc.L1Size())
	}

	if tc.ContainsL1("key2") {
		t.Error("key2 should have been evicted from L1")
	}

	val, err := tc.Get("key2")
	if err != nil {
		t.Fatalf("key2 should exist in L2, got err=%v", err)
	}
	if string(val) != "0123456789" {
		t.Errorf("expected value from L2, got %s", string(val))
	}
}

func TestWriteBackOnEviction(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       2,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: time.Hour,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	tc.Put("key2", []byte("value2"))
	tc.Put("key3", []byte("value3"))

	tc.l1.waitEvictions()

	if tc.L1Count() != 2 {
		t.Errorf("expected L1 count 2, got %d", tc.L1Count())
	}

	filename := filepath.Join(tmpDir, sanitizeKey("key1"))
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("key1 should have been written to L2 on eviction: %v", err)
	}
	if string(data) != "value1" {
		t.Errorf("expected value1, got %s", string(data))
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Fatalf("key1 should be retrievable, got err=%v", err)
	}
	if string(val) != "value1" {
		t.Errorf("expected value1, got %s", string(val))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	tc.Put("key2", []byte("value2"))

	err = tc.Delete("key1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = tc.Get("key1")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound for deleted key, got %v", err)
	}

	val, err := tc.Get("key2")
	if err != nil {
		t.Fatalf("key2 should still exist, got err=%v", err)
	}
	if string(val) != "value2" {
		t.Errorf("expected value2, got %s", string(val))
	}

	filename := filepath.Join(tmpDir, sanitizeKey("key1"))
	if _, err := os.Stat(filename); err == nil {
		t.Error("expected L2 file to be deleted")
	}
}

func TestUpdateExistingKey(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("old_value"))
	tc.Put("key1", []byte("new_value"))

	if tc.L1Count() != 1 {
		t.Errorf("expected L1 count 1, got %d", tc.L1Count())
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if string(val) != "new_value" {
		t.Errorf("expected new_value, got %s", string(val))
	}
}

func TestPut_EmptyKey(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Put("", []byte("value"))
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestPut_NilValue(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Put("key", nil)
	if err != ErrNilValue {
		t.Errorf("expected ErrNilValue, got %v", err)
	}
}

func TestGet_EmptyKey(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	_, err = tc.Get("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestDelete_EmptyKey(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Delete("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestDelete_NonExistentKey(t *testing.T) {
	tc, err := NewTieredCache()
	if err != nil {
		t.Fatalf("NewTieredCache failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	err = tc.Delete("nonexistent")
	if err != nil {
		t.Errorf("expected no error for non-existent key, got %v", err)
	}
}

func TestClear(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("value%d", i)
		tc.Put(key, []byte(val))
	}

	if tc.L1Count() != 10 {
		t.Errorf("expected L1 count 10, got %d", tc.L1Count())
	}
	if tc.L2Count() != 10 {
		t.Errorf("expected L2 count 10, got %d", tc.L2Count())
	}

	err = tc.Clear()
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if tc.L1Count() != 0 {
		t.Errorf("expected L1 count 0 after clear, got %d", tc.L1Count())
	}
	if tc.L2Count() != 0 {
		t.Errorf("expected L2 count 0 after clear, got %d", tc.L2Count())
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, file := range files {
		if !file.IsDir() {
			t.Errorf("expected no files in L2 dir, found %s", file.Name())
		}
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with-dash"},
		{"with_underscore", "with__underscore"},
		{"with.dot", "with.dot"},
		{"with space", "with_20space"},
		{"with/slash", "with_2fslash"},
		{"中文", "_e4_b8_ad_e6_96_87"},
		{"UPPER_case", "UPPER__case"},
		{"num123", "num123"},
	}

	for _, tt := range tests {
		result := sanitizeKey(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeKey(%q) = %q, expected %q", tt.input, result, tt.expected)
		}

		unsanitized := unsanitizeKey(result)
		if unsanitized != tt.input {
			t.Errorf("unsanitizeKey(%q) = %q, expected %q", result, unsanitized, tt.input)
		}
	}
}

func TestLoadL2FromDisk(t *testing.T) {
	tmpDir := t.TempDir()

	preCreated := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	for key, val := range preCreated {
		filename := filepath.Join(tmpDir, sanitizeKey(key))
		err := os.WriteFile(filename, []byte(val), 0644)
		if err != nil {
			t.Fatalf("failed to pre-create L2 file: %v", err)
		}
	}

	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	if tc.L2Count() != 3 {
		t.Errorf("expected L2 count 3 after load, got %d", tc.L2Count())
	}

	for key, expectedVal := range preCreated {
		val, err := tc.Get(key)
		if err != nil {
			t.Errorf("Get %q failed: %v", key, err)
			continue
		}
		if string(val) != expectedVal {
			t.Errorf("Get %q = %q, expected %q", key, string(val), expectedVal)
		}
	}

	if tc.L1Count() != 3 {
		t.Errorf("expected L1 count 3 after loading from L2, got %d", tc.L1Count())
	}
}

func TestWriteBackAutoFlush(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: 50 * time.Millisecond,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))

	if tc.L2Count() != 0 {
		t.Errorf("expected L2 count 0 before auto flush, got %d", tc.L2Count())
	}

	time.Sleep(200 * time.Millisecond)

	if tc.L2Count() != 1 {
		t.Errorf("expected L2 count 1 after auto flush, got %d", tc.L2Count())
	}
}

func TestLRUOrder(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       3,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	tc.Put("key2", []byte("value2"))
	tc.Put("key3", []byte("value3"))

	tc.Get("key1")
	tc.Get("key3")

	tc.Put("key4", []byte("value4"))

	if tc.ContainsL1("key2") {
		t.Error("key2 should have been evicted from L1 (least recently used)")
	}

	val, err := tc.Get("key1")
	if err != nil {
		t.Errorf("key1 should exist, got err=%v", err)
	}
	if string(val) != "value1" {
		t.Errorf("expected value1, got %s", string(val))
	}

	val, err = tc.Get("key3")
	if err != nil {
		t.Errorf("key3 should exist, got err=%v", err)
	}
	if string(val) != "value3" {
		t.Errorf("expected value3, got %s", string(val))
	}
}

func TestConcurrentPut(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       10000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("g%d_k%d", id, i)
				val := fmt.Sprintf("v%d_%d", id, i)
				tc.Put(key, []byte(val))
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numOps
	if int(tc.L1Count()) != expected {
		t.Errorf("expected %d keys in L1, got %d", expected, tc.L1Count())
	}
	if int(tc.L2Count()) != expected {
		t.Errorf("expected %d keys in L2, got %d", expected, tc.L2Count())
	}
}

func TestConcurrentGet(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       10000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("value%d", i)
		tc.Put(key, []byte(val))
	}

	var wg sync.WaitGroup
	numGoroutines := 20
	var getErrors int64
	var valueMismatches int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numKeys; i++ {
				key := fmt.Sprintf("key%d", i)
				expected := fmt.Sprintf("value%d", i)
				val, err := tc.Get(key)
				if err != nil {
					atomic.AddInt64(&getErrors, 1)
					t.Errorf("Get %q failed: %v", key, err)
					continue
				}
				if string(val) != expected {
					atomic.AddInt64(&valueMismatches, 1)
					t.Errorf("Get %q = %q, expected %q", key, string(val), expected)
				}
			}
		}()
	}

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get errors", getErrors)
	}
	if valueMismatches > 0 {
		t.Errorf("found %d value mismatches", valueMismatches)
	}
}

func TestConcurrentPutAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: 50 * time.Millisecond,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	numKeys := 50
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("pkey%d", i)
		val := fmt.Sprintf("pval%d", i)
		tc.Put(key, []byte(val))
	}

	var wg sync.WaitGroup
	var getErrors int64
	var valueMismatches int64

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("pkey%d", i)
			val := fmt.Sprintf("pval_updated_%d", i)
			tc.Put(key, []byte(val))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("pkey%d", i)
			expectedOld := fmt.Sprintf("pval%d", i)
			expectedNew := fmt.Sprintf("pval_updated_%d", i)

			val, err := tc.Get(key)
			if err != nil {
				atomic.AddInt64(&getErrors, 1)
				t.Errorf("Get %q failed: %v", key, err)
				continue
			}

			if string(val) != expectedOld && string(val) != expectedNew {
				atomic.AddInt64(&valueMismatches, 1)
				t.Errorf("Get %q = %q, expected %q or %q", key, string(val), expectedOld, expectedNew)
			}
		}
	}()

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get errors", getErrors)
	}
	if valueMismatches > 0 {
		t.Errorf("found %d value mismatches", valueMismatches)
	}
}

func TestEvictionCallback(t *testing.T) {
	var evictedKeys []string
	var mu sync.Mutex

	onEvict := func(entry *CacheEntry) {
		mu.Lock()
		defer mu.Unlock()
		evictedKeys = append(evictedKeys, entry.Key)
	}

	cfg := CacheLevelConfig{
		Capacity:       3,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, onEvict)

	cache.put("key1", []byte("value1"))
	cache.put("key2", []byte("value2"))
	cache.put("key3", []byte("value3"))
	cache.put("key4", []byte("value4"))

	cache.waitEvictions()

	mu.Lock()
	defer mu.Unlock()

	if len(evictedKeys) != 1 {
		t.Errorf("expected 1 eviction, got %d", len(evictedKeys))
	}
	if len(evictedKeys) > 0 && evictedKeys[0] != "key1" {
		t.Errorf("expected key1 to be evicted, got %s", evictedKeys[0])
	}
}

func TestCapacityModeMixed(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       3,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       50,
			CapacityMode:   CapacityModeBytes,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		val := fmt.Sprintf("%010d", i)
		tc.Put(key, []byte(val))
	}

	if tc.L1Count() != 3 {
		t.Errorf("expected L1 count 3, got %d", tc.L1Count())
	}

	if tc.L2Size() > 50 {
		t.Errorf("expected L2 size <= 50, got %d", tc.L2Size())
	}
}

func TestLRUCache_UpdateExisting(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       10,
		CapacityMode:   CapacityModeBytes,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	cache.put("key1", []byte("0123456789"))
	if cache.size() != 10 {
		t.Errorf("expected size 10, got %d", cache.size())
	}

	cache.put("key1", []byte("01234"))
	if cache.size() != 5 {
		t.Errorf("expected size 5 after update, got %d", cache.size())
	}
}

func TestLRUCache_EvictAll(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       10,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		cache.put(key, []byte("value"))
	}

	if cache.len() != 5 {
		t.Errorf("expected len 5, got %d", cache.len())
	}

	entries := cache.evictAll()
	if len(entries) != 5 {
		t.Errorf("expected 5 evicted entries, got %d", len(entries))
	}
	if cache.len() != 0 {
		t.Errorf("expected len 0 after evictAll, got %d", cache.len())
	}
	if cache.size() != 0 {
		t.Errorf("expected size 0 after evictAll, got %d", cache.size())
	}
}

func TestLRUCache_DeleteNonExistent(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       10,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	deleted := cache.delete("nonexistent")
	if deleted {
		t.Error("expected delete to return false for non-existent key")
	}
}

func TestClose_FlushesWriteBack(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: time.Hour,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}

	tc.Put("key1", []byte("value1"))
	tc.Put("key2", []byte("value2"))

	if tc.L2Count() != 0 {
		t.Errorf("expected L2 count 0 before close, got %d", tc.L2Count())
	}

	err = tc.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	filename1 := filepath.Join(tmpDir, sanitizeKey("key1"))
	data, err := os.ReadFile(filename1)
	if err != nil {
		t.Fatalf("key1 should have been flushed on close: %v", err)
	}
	if string(data) != "value1" {
		t.Errorf("expected value1, got %s", string(data))
	}

	filename2 := filepath.Join(tmpDir, sanitizeKey("key2"))
	data, err = os.ReadFile(filename2)
	if err != nil {
		t.Fatalf("key2 should have been flushed on close: %v", err)
	}
	if string(data) != "value2" {
		t.Errorf("expected value2, got %s", string(data))
	}

	tc.Clear()
}

func TestFlush_WriteThrough(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	tc.Flush()

	if tc.L1Count() != 1 {
		t.Errorf("expected L1 count 1, got %d", tc.L1Count())
	}
}

func TestLRUCache_GetNonExistent(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       10,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	_, ok := cache.get("nonexistent")
	if ok {
		t.Error("expected get to return false for non-existent key")
	}
}

func TestLRUCache_GetUpdatesOrder(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       3,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	cache.put("key1", []byte("value1"))
	cache.put("key2", []byte("value2"))
	cache.put("key3", []byte("value3"))

	cache.get("key1")

	cache.put("key4", []byte("value4"))

	_, ok := cache.get("key2")
	if ok {
		t.Error("key2 should have been evicted")
	}

	_, ok = cache.get("key1")
	if !ok {
		t.Error("key1 should still exist")
	}
}

func TestLRUCache_EvictFromEmpty(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       10,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}

	cache := newLRUCache(cfg, nil)

	entry := cache.evictOneLocked()
	if entry != nil {
		t.Errorf("expected nil entry from evicting empty cache, got %v", entry)
	}
}

func TestValidateConfig_Default(t *testing.T) {
	cfg := DefaultConfig()
	validateConfig(&cfg)
	err := validateConfigValues(cfg)
	if err != nil {
		t.Errorf("validateConfigValues should pass for default config, got %v", err)
	}
}

func TestValidateConfig_NegativeL1Capacity(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L1Config.Capacity = -5

	validateConfig(&cfg)
	err := validateConfigValues(cfg)
	if err == nil {
		t.Error("expected error for negative L1 capacity")
	}
}

func TestValidateConfig_InvalidL2EvictionPolicy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.L2Config.EvictionPolicy = "fifo"

	validateConfig(&cfg)
	err := validateConfigValues(cfg)
	if err == nil {
		t.Error("expected error for invalid L2 eviction policy")
	}
}

func TestNewTieredCacheWithConfig_DefaultEvictionPolicy(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:     100,
			CapacityMode: CapacityModeCount,
		},
		L2Config: CacheLevelConfig{
			Capacity:     1000,
			CapacityMode: CapacityModeCount,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	if tc == nil {
		t.Fatal("expected non-nil TieredCache")
	}
}

func TestNewTieredCacheWithConfig_DefaultCapacityMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	tc.Put("key1", []byte("value1"))
	if tc.L1Count() != 1 {
		t.Errorf("expected L1 count 1, got %d", tc.L1Count())
	}
}

func TestLRUCache_Defaults(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity: 0,
	}

	cache := newLRUCache(cfg, nil)
	if cache.capacity != 1000 {
		t.Errorf("expected default capacity 1000, got %d", cache.capacity)
	}
	if cache.capacityMode != CapacityModeCount {
		t.Errorf("expected default capacity mode count, got %s", cache.capacityMode)
	}
	if cache.evictionPolicy != EvictionPolicyLRU {
		t.Errorf("expected default eviction policy LRU, got %s", cache.evictionPolicy)
	}
}

func TestErrorValues(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrInvalidCapacity == nil {
		t.Error("ErrInvalidCapacity should not be nil")
	}
	if ErrInvalidPolicy == nil {
		t.Error("ErrInvalidPolicy should not be nil")
	}
	if ErrNilValue == nil {
		t.Error("ErrNilValue should not be nil")
	}
	if ErrEmptyKey == nil {
		t.Error("ErrEmptyKey should not be nil")
	}

	if ErrKeyNotFound.Error() != "key not found" {
		t.Errorf("unexpected ErrKeyNotFound message: %s", ErrKeyNotFound.Error())
	}
}

func TestWritePolicyConstants(t *testing.T) {
	if string(WritePolicyWriteThrough) != "write_through" {
		t.Errorf("expected write_through, got %s", WritePolicyWriteThrough)
	}
	if string(WritePolicyWriteBack) != "write_back" {
		t.Errorf("expected write_back, got %s", WritePolicyWriteBack)
	}
}

func TestCapacityModeConstants(t *testing.T) {
	if string(CapacityModeCount) != "count" {
		t.Errorf("expected count, got %s", CapacityModeCount)
	}
	if string(CapacityModeBytes) != "bytes" {
		t.Errorf("expected bytes, got %s", CapacityModeBytes)
	}
}

func TestEvictionPolicyConstants(t *testing.T) {
	if string(EvictionPolicyLRU) != "lru" {
		t.Errorf("expected lru, got %s", EvictionPolicyLRU)
	}
}

func TestConcurrentGetWithRace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       500,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       5000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	numKeys := 200
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key:%d", i)
		val := []byte(fmt.Sprintf("value:%d", i))
		if err := tc.Put(key, val); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	numGoroutines := 50
	numIterations := 200
	var wg sync.WaitGroup
	var getErrors int64
	var valueMismatches int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("key:%d", keyIdx)
				expected := fmt.Sprintf("value:%d", keyIdx)

				val, err := tc.Get(key)
				if err != nil {
					atomic.AddInt64(&getErrors, 1)
					continue
				}
				if string(val) != expected {
					atomic.AddInt64(&valueMismatches, 1)
				}
			}
		}(g)
	}

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get errors during concurrent reads", getErrors)
	}
	if valueMismatches > 0 {
		t.Errorf("found %d value mismatches during concurrent reads", valueMismatches)
	}
}

func TestConcurrentPutAndGetWithRace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       10000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteBack,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	numWriters := 20
	numReaders := 30
	numKeys := 500
	numIterations := 100
	var wg sync.WaitGroup
	var putErrors int64

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("key:%d", keyIdx)
				val := []byte(fmt.Sprintf("writer:%d:iter:%d", id, i))
				if err := tc.Put(key, val); err != nil {
					atomic.AddInt64(&putErrors, 1)
				}
			}
		}(w)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("key:%d", keyIdx)
				tc.Get(key)
			}
		}(r)
	}

	wg.Wait()

	if putErrors > 0 {
		t.Errorf("found %d Put errors during concurrent operations", putErrors)
	}
}

func TestLoadL2FromDisk_NoDataLossOnCapacityLimit(t *testing.T) {
	tmpDir := t.TempDir()

	numFiles := 20
	for i := 0; i < numFiles; i++ {
		key := fmt.Sprintf("key:%d", i)
		val := []byte(fmt.Sprintf("value:%d", i))
		filename := filepath.Join(tmpDir, sanitizeKey(key))
		if err := os.WriteFile(filename, val, 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	}

	beforeFiles, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir: %v", err)
	}
	beforeCount := len(beforeFiles)
	if beforeCount != numFiles {
		t.Fatalf("expected %d files before, got %d", numFiles, beforeCount)
	}

	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       5,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	afterFiles, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir after load: %v", err)
	}
	afterCount := len(afterFiles)
	if afterCount != beforeCount {
		t.Errorf("disk file count changed during load: before=%d, after=%d, lost=%d files",
			beforeCount, afterCount, beforeCount-afterCount)
	}

	if tc.L2Count() > 5 {
		t.Errorf("expected L2 count <= 5 due to capacity limit, got %d", tc.L2Count())
	}

	allExist := true
	for i := 0; i < numFiles; i++ {
		key := fmt.Sprintf("key:%d", i)
		filename := filepath.Join(tmpDir, sanitizeKey(key))
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			allExist = false
			t.Errorf("disk file for key %q was deleted during load", key)
		}
	}
	if !allExist {
		t.Fatal("some disk files were deleted during L2 load due to capacity limit")
	}
}

func TestLoadL2FromDisk_RestartAndVerifyData(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc1, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("first NewTieredCacheWithConfig failed: %v", err)
	}

	numKeys := 50
	expected := make(map[string]string)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("restart:key:%d", i)
		val := fmt.Sprintf("restart:value:%d", i)
		expected[key] = val
		if err := tc1.Put(key, []byte(val)); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	if err := tc1.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	tc2, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("second NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc2.Close()
	defer tc2.Clear()

	for key, val := range expected {
		got, err := tc2.Get(key)
		if err != nil {
			t.Errorf("Get %q after restart failed: %v", key, err)
			continue
		}
		if string(got) != val {
			t.Errorf("Get %q after restart = %q, expected %q", key, string(got), val)
		}
	}

	afterFiles, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir: %v", err)
	}
	if len(afterFiles) < numKeys {
		t.Errorf("expected >= %d disk files after restart, got %d", numKeys, len(afterFiles))
	}
}

func TestWriteBackFailureHandling(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: 10 * time.Millisecond,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	key := "test:writeback:key"
	val := []byte("test:writeback:value")
	if err := tc.Put(key, val); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	if err := os.Chmod(tmpDir, 0444); err != nil {
		t.Skipf("cannot chmod directory to read-only, skipping test: %v", err)
	}
	defer os.Chmod(tmpDir, 0755)

	time.Sleep(200 * time.Millisecond)

	os.Chmod(tmpDir, 0755)

	errCount := tc.WriteBackErrorCount()
	if errCount > 1 {
		t.Errorf("unexpected write-back error count: %d (should be 0 or 1, depending on timing)", errCount)
	}
}

func TestWriteBackMaxRetriesExceeded(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       10,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       100,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy:       WritePolicyWriteBack,
		L2Dir:             tmpDir,
		WriteBackInterval: 5 * time.Millisecond,
	}

	lc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}

	for i := 0; i < maxWriteBackRetries+1; i++ {
		key := fmt.Sprintf("retry:key:%d", i)
		val := []byte(fmt.Sprintf("retry:val:%d", i))
		if err := lc.Put(key, val); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	if err := os.Chmod(tmpDir, 0444); err != nil {
		lc.Close()
		lc.Clear()
		t.Skipf("cannot chmod directory, skipping: %v", err)
	}
	defer os.Chmod(tmpDir, 0755)

	time.Sleep(200 * time.Millisecond)
	os.Chmod(tmpDir, 0755)

	if err := lc.Close(); err != nil {
		if err != ErrWriteBackFailed {
			t.Logf("Close returned error (expected if failures occurred): %v", err)
		}
	}
	lc.Clear()
}

func TestLRUCacheConcurrentGet_NoRace(t *testing.T) {
	cfg := CacheLevelConfig{
		Capacity:       1000,
		CapacityMode:   CapacityModeCount,
		EvictionPolicy: EvictionPolicyLRU,
	}
	cache := newLRUCache(cfg, nil)
	defer cache.Close()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key:%d", i)
		val := []byte(fmt.Sprintf("val:%d", i))
		cache.put(key, val)
	}

	numGoroutines := 30
	numIterations := 300
	var wg sync.WaitGroup

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("key:%d", keyIdx)
				cache.get(key)
			}
		}(g)
	}

	wg.Wait()
}

func TestTieredCacheConcurrentDeleteAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		L1Config: CacheLevelConfig{
			Capacity:       1000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		L2Config: CacheLevelConfig{
			Capacity:       10000,
			CapacityMode:   CapacityModeCount,
			EvictionPolicy: EvictionPolicyLRU,
		},
		WritePolicy: WritePolicyWriteThrough,
		L2Dir:       tmpDir,
	}

	tc, err := NewTieredCacheWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewTieredCacheWithConfig failed: %v", err)
	}
	defer tc.Close()
	defer tc.Clear()

	numKeys := 200
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("del:key:%d", i)
		val := []byte(fmt.Sprintf("del:val:%d", i))
		if err := tc.Put(key, val); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	var wg sync.WaitGroup
	numDeleters := 10
	numGetters := 20
	numIterations := 100
	var deleteErrors int64

	for d := 0; d < numDeleters; d++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("del:key:%d", keyIdx)
				if err := tc.Delete(key); err != nil {
					atomic.AddInt64(&deleteErrors, 1)
				}
			}
		}(d)
	}

	for g := 0; g < numGetters; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numIterations; i++ {
				keyIdx := (id*numIterations + i) % numKeys
				key := fmt.Sprintf("del:key:%d", keyIdx)
				tc.Get(key)
			}
		}(g)
	}

	wg.Wait()

	if deleteErrors > 0 {
		t.Errorf("found %d Delete errors during concurrent operations", deleteErrors)
	}
}
