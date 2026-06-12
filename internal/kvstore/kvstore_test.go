package kvstore

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
)

func TestNewKVStore(t *testing.T) {
	kv := NewKVStore()
	if kv == nil {
		t.Fatal("NewKVStore returned nil")
	}
	if kv.segmentCount != 16 {
		t.Errorf("expected default segmentCount 16, got %d", kv.segmentCount)
	}
	if kv.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", kv.Count())
	}
}

func TestNewKVStoreWithConfig(t *testing.T) {
	cfg := Config{
		SegmentCount:   8,
		BloomCapacity:  5000,
		BloomFalseRate: 0.001,
	}
	kv := NewKVStoreWithConfig(cfg)
	if kv == nil {
		t.Fatal("NewKVStoreWithConfig returned nil")
	}
	if kv.segmentCount != 8 {
		t.Errorf("expected segmentCount 8, got %d", kv.segmentCount)
	}
	if len(kv.segments) != 8 {
		t.Errorf("expected 8 segments, got %d", len(kv.segments))
	}
}

func TestNewKVStoreWithConfig_Defaults(t *testing.T) {
	cfg := Config{}
	kv := NewKVStoreWithConfig(cfg)
	if kv.segmentCount != 16 {
		t.Errorf("expected default segmentCount 16, got %d", kv.segmentCount)
	}
	if kv.bloomFilter.Size() == 0 {
		t.Error("bloom filter should have non-zero size with default config")
	}
}

func TestPutAndGet(t *testing.T) {
	kv := NewKVStore()

	kv.Put("key1", "value1")
	kv.Put("key2", "value2")

	val, ok := kv.Get("key1")
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %s", val)
	}

	val, ok = kv.Get("key2")
	if !ok {
		t.Error("expected key2 to exist")
	}
	if val != "value2" {
		t.Errorf("expected value2, got %s", val)
	}

	if kv.Count() != 2 {
		t.Errorf("expected count 2, got %d", kv.Count())
	}
}

func TestPut_UpdateExisting(t *testing.T) {
	kv := NewKVStore()

	kv.Put("key1", "old_value")
	kv.Put("key1", "new_value")

	val, ok := kv.Get("key1")
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "new_value" {
		t.Errorf("expected new_value, got %s", val)
	}
	if kv.Count() != 1 {
		t.Errorf("expected count 1, got %d", kv.Count())
	}
}

func TestGet_NonExistent(t *testing.T) {
	kv := NewKVStore()

	val, ok := kv.Get("nonexistent")
	if ok {
		t.Error("expected ok to be false for non-existent key")
	}
	if val != "" {
		t.Errorf("expected empty string, got %s", val)
	}
}

func TestGet_BloomFilterQuickReject(t *testing.T) {
	kv := NewKVStore()

	_, ok := kv.Get("nonexistent_key_12345")
	if ok {
		t.Error("bloom filter should quickly reject non-existent key")
	}
}

func TestDelete(t *testing.T) {
	kv := NewKVStore()

	kv.Put("key1", "value1")
	kv.Put("key2", "value2")

	deleted := kv.Delete("key1")
	if !deleted {
		t.Error("expected Delete to return true for existing key")
	}

	_, ok := kv.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}

	_, ok = kv.Get("key2")
	if !ok {
		t.Error("expected key2 to still exist")
	}

	if kv.Count() != 1 {
		t.Errorf("expected count 1, got %d", kv.Count())
	}
}

func TestDelete_NonExistent(t *testing.T) {
	kv := NewKVStore()

	deleted := kv.Delete("nonexistent")
	if deleted {
		t.Error("expected Delete to return false for non-existent key")
	}
}

func TestCount(t *testing.T) {
	kv := NewKVStore()

	if kv.Count() != 0 {
		t.Errorf("expected 0, got %d", kv.Count())
	}

	for i := 0; i < 100; i++ {
		kv.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("value%d", i))
	}

	if kv.Count() != 100 {
		t.Errorf("expected 100, got %d", kv.Count())
	}
}

func TestBatchPut(t *testing.T) {
	kv := NewKVStore()

	pairs := map[string]string{
		"a": "1",
		"b": "2",
		"c": "3",
	}

	err := kv.BatchPut(pairs)
	if err != nil {
		t.Fatalf("BatchPut failed: %v", err)
	}

	if kv.Count() != 3 {
		t.Errorf("expected count 3, got %d", kv.Count())
	}

	for k, v := range pairs {
		val, ok := kv.Get(k)
		if !ok {
			t.Errorf("expected key %s to exist", k)
		}
		if val != v {
			t.Errorf("expected %s for key %s, got %s", v, k, val)
		}
	}
}

func TestBatchPut_Empty(t *testing.T) {
	kv := NewKVStore()

	err := kv.BatchPut(map[string]string{})
	if err != ErrEmptyBatch {
		t.Errorf("expected ErrEmptyBatch, got %v", err)
	}
}

func TestBatchPut_Nil(t *testing.T) {
	kv := NewKVStore()

	err := kv.BatchPut(nil)
	if err != ErrEmptyBatch {
		t.Errorf("expected ErrEmptyBatch for nil, got %v", err)
	}
}

func TestBatchPut_Atomicity(t *testing.T) {
	kv := NewKVStore()

	kv.Put("existing", "old")

	pairs := map[string]string{
		"existing": "new",
		"new1":     "val1",
		"new2":     "val2",
	}

	err := kv.BatchPut(pairs)
	if err != nil {
		t.Fatalf("BatchPut failed: %v", err)
	}

	val, _ := kv.Get("existing")
	if val != "new" {
		t.Errorf("expected existing to be updated to 'new', got %s", val)
	}

	val, _ = kv.Get("new1")
	if val != "val1" {
		t.Errorf("expected new1=val1, got %s", val)
	}

	val, _ = kv.Get("new2")
	if val != "val2" {
		t.Errorf("expected new2=val2, got %s", val)
	}
}

func TestRangeScan(t *testing.T) {
	kv := NewKVStore()

	keys := []string{"apple", "banana", "cherry", "date", "elderberry", "fig", "grape"}
	for _, k := range keys {
		kv.Put(k, "value_"+k)
	}

	result, err := kv.RangeScan("banana", "elderberry", 100)
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	expectedKeys := []string{"banana", "cherry", "date", "elderberry"}
	if len(result.Items) != len(expectedKeys) {
		t.Fatalf("expected %d items, got %d", len(expectedKeys), len(result.Items))
	}

	for i, expected := range expectedKeys {
		if result.Items[i].Key != expected {
			t.Errorf("expected key %s at position %d, got %s", expected, i, result.Items[i].Key)
		}
	}

	if result.Total != len(expectedKeys) {
		t.Errorf("expected Total %d, got %d", len(expectedKeys), result.Total)
	}

	if result.HasMore {
		t.Error("expected HasMore to be false")
	}
	if result.NextKey != "" {
		t.Errorf("expected NextKey to be empty, got %s", result.NextKey)
	}
}

func TestRangeScan_InvalidRange(t *testing.T) {
	kv := NewKVStore()

	_, err := kv.RangeScan("z", "a", 10)
	if err != ErrInvalidRange {
		t.Errorf("expected ErrInvalidRange, got %v", err)
	}
}

func TestRangeScan_InvalidLimit(t *testing.T) {
	kv := NewKVStore()

	_, err := kv.RangeScan("a", "z", 0)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit for 0, got %v", err)
	}

	_, err = kv.RangeScan("a", "z", -5)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit for negative, got %v", err)
	}
}

func TestRangeScan_Pagination(t *testing.T) {
	kv := NewKVStore()

	for i := 0; i < 10; i++ {
		kv.Put(fmt.Sprintf("key%02d", i), fmt.Sprintf("val%d", i))
	}

	result1, err := kv.RangeScan("key00", "key99", 3)
	if err != nil {
		t.Fatalf("RangeScan page1 failed: %v", err)
	}
	if len(result1.Items) != 3 {
		t.Fatalf("expected 3 items on page 1, got %d", len(result1.Items))
	}
	if !result1.HasMore {
		t.Error("expected HasMore=true on page 1")
	}
	if result1.NextKey == "" {
		t.Error("expected non-empty NextKey on page 1")
	}
	if result1.Total != 10 {
		t.Errorf("expected Total=10, got %d", result1.Total)
	}

	result2, err := kv.RangeScan(result1.NextKey, "key99", 3)
	if err != nil {
		t.Fatalf("RangeScan page2 failed: %v", err)
	}
	if len(result2.Items) != 3 {
		t.Fatalf("expected 3 items on page 2, got %d", len(result2.Items))
	}

	result3, err := kv.RangeScan(result2.NextKey, "key99", 3)
	if err != nil {
		t.Fatalf("RangeScan page3 failed: %v", err)
	}
	if len(result3.Items) != 3 {
		t.Fatalf("expected 3 items on page 3, got %d", len(result3.Items))
	}

	result4, err := kv.RangeScan(result3.NextKey, "key99", 3)
	if err != nil {
		t.Fatalf("RangeScan page4 failed: %v", err)
	}
	if len(result4.Items) != 1 {
		t.Fatalf("expected 1 item on page 4, got %d", len(result4.Items))
	}
	if result4.HasMore {
		t.Error("expected HasMore=false on last page")
	}
	if result4.NextKey != "" {
		t.Errorf("expected empty NextKey on last page, got %s", result4.NextKey)
	}

	var allCollected []string
	for _, r := range []*RangeResult{result1, result2, result3, result4} {
		for _, item := range r.Items {
			allCollected = append(allCollected, item.Key)
		}
	}

	expected := make([]string, 10)
	for i := 0; i < 10; i++ {
		expected[i] = fmt.Sprintf("key%02d", i)
	}
	sort.Strings(allCollected)
	for i, e := range expected {
		if allCollected[i] != e {
			t.Errorf("pagination mismatch at %d: expected %s, got %s", i, e, allCollected[i])
		}
	}
}

func TestRangeScan_EmptyResult(t *testing.T) {
	kv := NewKVStore()

	kv.Put("a", "1")
	kv.Put("b", "2")

	result, err := kv.RangeScan("x", "z", 10)
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.Items))
	}
	if result.Total != 0 {
		t.Errorf("expected Total=0, got %d", result.Total)
	}
}

func TestSnapshot(t *testing.T) {
	kv := NewKVStore()

	kv.Put("a", "1")
	kv.Put("b", "2")
	kv.Put("c", "3")

	snap := kv.Snapshot()
	if snap == nil {
		t.Fatal("Snapshot returned nil")
	}
	if snap.Count() != 3 {
		t.Errorf("expected snapshot count 3, got %d", snap.Count())
	}

	val, ok := snap.Get("a")
	if !ok || val != "1" {
		t.Errorf("snapshot: expected a=1, got %s (ok=%v)", val, ok)
	}

	val, ok = snap.Get("b")
	if !ok || val != "2" {
		t.Errorf("snapshot: expected b=2, got %s (ok=%v)", val, ok)
	}

	val, ok = snap.Get("nonexistent")
	if ok {
		t.Error("snapshot: expected nonexistent key to return ok=false")
	}
	if val != "" {
		t.Errorf("snapshot: expected empty for nonexistent, got %s", val)
	}
}

func TestSnapshot_Empty(t *testing.T) {
	kv := NewKVStore()

	snap := kv.Snapshot()
	if snap.Count() != 0 {
		t.Errorf("expected empty snapshot count 0, got %d", snap.Count())
	}
}

func TestSnapshot_Isolation(t *testing.T) {
	kv := NewKVStore()

	kv.Put("a", "1")
	kv.Put("b", "2")

	snap := kv.Snapshot()

	kv.Put("c", "3")
	kv.Delete("a")
	kv.Put("b", "updated")

	if snap.Count() != 2 {
		t.Errorf("snapshot count should remain 2, got %d", snap.Count())
	}

	val, _ := snap.Get("a")
	if val != "1" {
		t.Errorf("snapshot a should still be 1, got %s", val)
	}

	val, _ = snap.Get("b")
	if val != "2" {
		t.Errorf("snapshot b should still be 2, got %s", val)
	}

	_, ok := snap.Get("c")
	if ok {
		t.Error("snapshot should not contain c")
	}

	if kv.Count() != 2 {
		t.Errorf("kvstore count should be 2, got %d", kv.Count())
	}
}

func TestRestore(t *testing.T) {
	kv1 := NewKVStore()
	kv1.Put("a", "1")
	kv1.Put("b", "2")
	kv1.Put("c", "3")

	snap := kv1.Snapshot()

	kv2 := NewKVStore()
	kv2.Put("x", "99")

	err := kv2.Restore(snap)
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	if kv2.Count() != 3 {
		t.Errorf("expected count 3 after restore, got %d", kv2.Count())
	}

	_, ok := kv2.Get("x")
	if ok {
		t.Error("x should have been cleared during restore")
	}

	val, ok := kv2.Get("a")
	if !ok || val != "1" {
		t.Errorf("expected a=1 after restore, got %s (ok=%v)", val, ok)
	}

	val, ok = kv2.Get("b")
	if !ok || val != "2" {
		t.Errorf("expected b=2 after restore, got %s (ok=%v)", val, ok)
	}
}

func TestRestore_Nil(t *testing.T) {
	kv := NewKVStore()

	err := kv.Restore(nil)
	if err != ErrNilSnapshot {
		t.Errorf("expected ErrNilSnapshot, got %v", err)
	}
}

func TestSnapshot_DuringConcurrentWrites(t *testing.T) {
	kv := NewKVStore()

	var wg sync.WaitGroup
	numWriters := 10
	numWrites := 100

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numWrites; i++ {
				kv.Put(fmt.Sprintf("w%d_k%d", id, i), fmt.Sprintf("v%d_%d", id, i))
			}
		}(w)
	}

	snapshots := make([]*Snapshot, 5)
	for i := 0; i < 5; i++ {
		snapshots[i] = kv.Snapshot()
	}

	wg.Wait()

	finalCount := kv.Count()
	if finalCount != numWriters*numWrites {
		t.Errorf("expected final count %d, got %d", numWriters*numWrites, finalCount)
	}

	for i, snap := range snapshots {
		if snap == nil {
			t.Errorf("snapshot %d is nil", i)
		}
	}
}

func TestConcurrentPut(t *testing.T) {
	kv := NewKVStore()

	var wg sync.WaitGroup
	numGoroutines := 50
	numOps := 200

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				kv.Put(fmt.Sprintf("g%d_k%d", id, i), fmt.Sprintf("v%d_%d", id, i))
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numOps
	if kv.Count() != expected {
		t.Errorf("expected %d keys, got %d", expected, kv.Count())
	}
}

func TestConcurrentGet(t *testing.T) {
	kv := NewKVStore()

	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		kv.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("val%d", i))
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numKeys; i++ {
				val, ok := kv.Get(fmt.Sprintf("key%d", i))
				if !ok {
					t.Errorf("key%d not found", i)
					return
				}
				expected := fmt.Sprintf("val%d", i)
				if val != expected {
					t.Errorf("key%d: expected %s, got %s", i, expected, val)
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentPutAndGet(t *testing.T) {
	kv := NewKVStore()

	numKeys := 500
	for i := 0; i < numKeys; i++ {
		kv.Put(fmt.Sprintf("pkey%d", i), fmt.Sprintf("pval%d", i))
	}

	var wg sync.WaitGroup
	var getErrors int64
	var valueMismatches int64

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			kv.Put(fmt.Sprintf("pkey%d", i), fmt.Sprintf("pval_updated_%d", i))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			val, ok := kv.Get(fmt.Sprintf("pkey%d", i))
			if !ok {
				atomic.AddInt64(&getErrors, 1)
				t.Errorf("Get returned ok=false for key pkey%d which was pre-populated (concurrent consistency violation)", i)
				continue
			}
			expectedOld := fmt.Sprintf("pval%d", i)
			expectedNew := fmt.Sprintf("pval_updated_%d", i)
			if val != expectedOld && val != expectedNew {
				atomic.AddInt64(&valueMismatches, 1)
				t.Errorf("Get returned unexpected value for pkey%d: got %q, expected %q or %q", i, val, expectedOld, expectedNew)
			}
		}
	}()

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get consistency errors (ok=false for pre-existing keys)", getErrors)
	}
	if valueMismatches > 0 {
		t.Errorf("found %d value mismatches during concurrent Put/Get", valueMismatches)
	}
}

func TestConcurrentDelete(t *testing.T) {
	kv := NewKVStore()

	numKeys := 500
	for i := 0; i < numKeys; i++ {
		kv.Put(fmt.Sprintf("dkey%d", i), fmt.Sprintf("dval%d", i))
	}

	var wg sync.WaitGroup
	numDeleters := 10
	keysPerDeleter := numKeys / numDeleters

	for d := 0; d < numDeleters; d++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			start := id * keysPerDeleter
			end := start + keysPerDeleter
			for i := start; i < end; i++ {
				kv.Delete(fmt.Sprintf("dkey%d", i))
			}
		}(d)
	}

	wg.Wait()

	if kv.Count() != 0 {
		t.Errorf("expected count 0 after all deletes, got %d", kv.Count())
	}
}

func TestConcurrentBatchPut(t *testing.T) {
	kv := NewKVStore()

	var wg sync.WaitGroup
	numGoroutines := 20

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pairs := make(map[string]string)
			for i := 0; i < 50; i++ {
				pairs[fmt.Sprintf("bg%d_k%d", id, i)] = fmt.Sprintf("bg%d_v%d", id, i)
			}
			err := kv.BatchPut(pairs)
			if err != nil {
				t.Errorf("BatchPut goroutine %d failed: %v", id, err)
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * 50
	if kv.Count() != expected {
		t.Errorf("expected %d keys, got %d", expected, kv.Count())
	}
}

func TestConcurrentRangeScan(t *testing.T) {
	kv := NewKVStore()

	for i := 0; i < 200; i++ {
		kv.Put(fmt.Sprintf("rkey%03d", i), fmt.Sprintf("rval%d", i))
	}

	var wg sync.WaitGroup
	numScanners := 20

	for s := 0; s < numScanners; s++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				_, err := kv.RangeScan("rkey000", "rkey999", 50)
				if err != nil {
					t.Errorf("RangeScan failed: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}

func TestBloomFilter_Basic(t *testing.T) {
	bf := NewBloomFilter(1000, 0.01)

	if bf.Size() == 0 {
		t.Error("bloom filter size should be > 0")
	}
	if bf.HashCount() == 0 {
		t.Error("hash count should be > 0")
	}

	bf.Add("test_key")

	if !bf.MightContain("test_key") {
		t.Error("should contain test_key after Add")
	}

	if bf.MightContain("nonexistent_key_xyz") {
		t.Log("Note: false positive occurred (acceptable)")
	}
}

func TestBloomFilter_Reset(t *testing.T) {
	bf := NewBloomFilter(100, 0.01)

	bf.Add("key1")
	bf.Add("key2")

	if !bf.MightContain("key1") {
		t.Error("should contain key1 before reset")
	}

	bf.Reset()

	if bf.MightContain("key1") {
		t.Error("should not contain key1 after reset (extremely high probability)")
	}
}

func TestBloomFilter_Empty(t *testing.T) {
	bf := NewBloomFilter(100, 0.01)

	if bf.MightContain("anything") {
		t.Log("Note: false positive on empty filter is impossible, check logic")
		t.Error("empty bloom filter should not report any key as present")
	}
}

func TestBloomFilter_LargeScale(t *testing.T) {
	capacity := uint(10000)
	bf := NewBloomFilter(capacity, 0.01)

	inserted := make(map[string]bool)
	for i := uint(0); i < capacity; i++ {
		key := fmt.Sprintf("bf_key_%d", i)
		bf.Add(key)
		inserted[key] = true
	}

	falseNegatives := 0
	for key := range inserted {
		if !bf.MightContain(key) {
			falseNegatives++
		}
	}
	if falseNegatives > 0 {
		t.Errorf("found %d false negatives, bloom filter is broken", falseNegatives)
	}

	falsePositives := 0
	totalChecks := 10000
	for i := 0; i < totalChecks; i++ {
		key := fmt.Sprintf("nonexistent_%d", i+1000000)
		if bf.MightContain(key) {
			falsePositives++
		}
	}

	fpRate := float64(falsePositives) / float64(totalChecks)
	t.Logf("False positive rate: %.4f%% (%d/%d)", fpRate*100, falsePositives, totalChecks)
	if fpRate > 0.05 {
		t.Errorf("false positive rate too high: %.4f%%", fpRate*100)
	}
}

func TestBloomFilter_DefaultParameters(t *testing.T) {
	bf := NewBloomFilter(0, 0)

	if bf.Size() == 0 {
		t.Error("should have default size for zero capacity")
	}
	if bf.HashCount() == 0 {
		t.Error("should have default hash count")
	}

	bf.Add("default_test")
	if !bf.MightContain("default_test") {
		t.Error("should contain default_test")
	}
}

func TestSegmentLock_DifferentKeys(t *testing.T) {
	cfg := Config{
		SegmentCount: 4,
		BloomCapacity: 1000,
		BloomFalseRate: 0.01,
	}
	kv := NewKVStoreWithConfig(cfg)

	var wg sync.WaitGroup
	numOps := 1000

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			kv.Put(fmt.Sprintf("set_a_%d", i), "val_a")
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numOps; i++ {
			kv.Put(fmt.Sprintf("set_b_%d", i), "val_b")
		}
	}()

	wg.Wait()

	if kv.Count() != numOps*2 {
		t.Errorf("expected %d keys, got %d", numOps*2, kv.Count())
	}
}

func TestSegmentLock_SameKey(t *testing.T) {
	kv := NewKVStore()

	var wg sync.WaitGroup
	var counter int64
	numGoroutines := 50
	numIncrements := 100

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < numIncrements; i++ {
				kv.Put("shared_key", "locked_value")
				_ = counter
			}
		}()
	}

	wg.Wait()

	val, ok := kv.Get("shared_key")
	if !ok {
		t.Fatal("shared_key should exist")
	}
	if val != "locked_value" {
		t.Errorf("expected locked_value, got %s", val)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.SegmentCount != 16 {
		t.Errorf("expected default SegmentCount 16, got %d", cfg.SegmentCount)
	}
	if cfg.BloomCapacity != 10000 {
		t.Errorf("expected default BloomCapacity 10000, got %d", cfg.BloomCapacity)
	}
	if cfg.BloomFalseRate != 0.01 {
		t.Errorf("expected default BloomFalseRate 0.01, got %f", cfg.BloomFalseRate)
	}
}

func TestGetSegmentIndex(t *testing.T) {
	kv := NewKVStoreWithConfig(Config{SegmentCount: 8})

	idx1 := kv.getSegmentIndex("test_key_1")
	idx2 := kv.getSegmentIndex("test_key_1")
	if idx1 != idx2 {
		t.Error("same key should map to same segment")
	}

	if idx1 < 0 || idx1 >= 8 {
		t.Errorf("segment index %d out of range [0, 8)", idx1)
	}

	indices := make(map[int]bool)
	for i := 0; i < 1000; i++ {
		idx := kv.getSegmentIndex(fmt.Sprintf("distributed_key_%d", i))
		indices[idx] = true
	}
	if len(indices) < 2 {
		t.Errorf("keys should distribute across multiple segments, got only %d", len(indices))
	}
}

func TestPut_EmptyKey(t *testing.T) {
	kv := NewKVStore()

	kv.Put("", "empty_key_value")

	val, ok := kv.Get("")
	if !ok {
		t.Error("empty string key should be retrievable")
	}
	if val != "empty_key_value" {
		t.Errorf("expected empty_key_value, got %s", val)
	}
}

func TestPut_EmptyValue(t *testing.T) {
	kv := NewKVStore()

	kv.Put("key_empty_val", "")

	val, ok := kv.Get("key_empty_val")
	if !ok {
		t.Error("key with empty value should exist")
	}
	if val != "" {
		t.Errorf("expected empty value, got %s", val)
	}
}

func TestRangeScan_Inclusive(t *testing.T) {
	kv := NewKVStore()

	kv.Put("a", "1")
	kv.Put("m", "2")
	kv.Put("z", "3")

	result, err := kv.RangeScan("a", "z", 100)
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items (inclusive), got %d", len(result.Items))
	}

	resultA, err := kv.RangeScan("a", "a", 100)
	if err != nil {
		t.Fatalf("RangeScan single key failed: %v", err)
	}
	if len(resultA.Items) != 1 || resultA.Items[0].Key != "a" {
		t.Errorf("expected single item 'a', got %v", resultA.Items)
	}
}

func TestBatchPut_SingleSegment(t *testing.T) {
	kv := NewKVStoreWithConfig(Config{SegmentCount: 1})

	pairs := map[string]string{
		"k1": "v1",
		"k2": "v2",
		"k3": "v3",
	}

	err := kv.BatchPut(pairs)
	if err != nil {
		t.Fatalf("BatchPut with single segment failed: %v", err)
	}

	if kv.Count() != 3 {
		t.Errorf("expected 3, got %d", kv.Count())
	}
}

func TestBatchPut_MultipleSegments(t *testing.T) {
	cfg := Config{SegmentCount: 32}
	kv := NewKVStoreWithConfig(cfg)

	pairs := make(map[string]string)
	for i := 0; i < 100; i++ {
		pairs[fmt.Sprintf("multi_seg_%d", i)] = fmt.Sprintf("v%d", i)
	}

	err := kv.BatchPut(pairs)
	if err != nil {
		t.Fatalf("BatchPut with multiple segments failed: %v", err)
	}

	if kv.Count() != 100 {
		t.Errorf("expected 100, got %d", kv.Count())
	}
}

func TestConcurrentSnapshotAndWrite(t *testing.T) {
	kv := NewKVStore()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 300; i++ {
			kv.Put(fmt.Sprintf("snap_write_%d", i), fmt.Sprintf("v%d", i))
		}
	}()

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap := kv.Snapshot()
			if snap == nil {
				t.Error("snapshot should not be nil")
				return
			}
			for _, item := range snap.Data {
				if item == "" {
					continue
				}
			}
		}()
	}

	wg.Wait()
}

func TestRangeScan_SortOrder(t *testing.T) {
	kv := NewKVStore()

	unsortedKeys := []string{"zebra", "apple", "mango", "banana", "cherry"}
	for _, k := range unsortedKeys {
		kv.Put(k, "v")
	}

	result, err := kv.RangeScan("apple", "zebra", 100)
	if err != nil {
		t.Fatalf("RangeScan failed: %v", err)
	}

	sortedExpected := []string{"apple", "banana", "cherry", "mango", "zebra"}
	if len(result.Items) != len(sortedExpected) {
		t.Fatalf("expected %d items, got %d", len(sortedExpected), len(result.Items))
	}

	for i, expected := range sortedExpected {
		if result.Items[i].Key != expected {
			t.Errorf("position %d: expected %s, got %s", i, expected, result.Items[i].Key)
		}
	}
}

func TestRestore_EmptySnapshot(t *testing.T) {
	kv := NewKVStore()
	kv.Put("existing", "value")

	emptySnap := &Snapshot{Data: make(map[string]string)}

	err := kv.Restore(emptySnap)
	if err != nil {
		t.Fatalf("Restore empty snapshot failed: %v", err)
	}

	if kv.Count() != 0 {
		t.Errorf("expected 0 after restoring empty snapshot, got %d", kv.Count())
	}

	_, ok := kv.Get("existing")
	if ok {
		t.Error("existing key should be gone after restoring empty snapshot")
	}
}

func TestErrors_Values(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrEmptyBatch == nil {
		t.Error("ErrEmptyBatch should not be nil")
	}
	if ErrInvalidRange == nil {
		t.Error("ErrInvalidRange should not be nil")
	}
	if ErrInvalidLimit == nil {
		t.Error("ErrInvalidLimit should not be nil")
	}
	if ErrNilSnapshot == nil {
		t.Error("ErrNilSnapshot should not be nil")
	}
}

func TestConcurrentPutGet_SameKey_Consistency(t *testing.T) {
	kv := NewKVStore()

	const key = "shared_hot_key"
	kv.Put(key, "initial")

	var wg sync.WaitGroup
	var phantomMisses int64
	var totalGets int64

	numWriters := 10
	numReaders := 20
	iterationsPerGoroutine := 1000

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterationsPerGoroutine; i++ {
				kv.Put(key, fmt.Sprintf("writer_%d_iter_%d", id, i))
			}
		}(w)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterationsPerGoroutine; i++ {
				_, ok := kv.Get(key)
				atomic.AddInt64(&totalGets, 1)
				if !ok {
					atomic.AddInt64(&phantomMisses, 1)
					t.Errorf("PHANTOM MISS: Get returned ok=false for key %q which should always exist (concurrent consistency violation)", key)
				}
			}
		}()
	}

	wg.Wait()

	if phantomMisses > 0 {
		t.Errorf("Detected %d phantom misses out of %d total Gets - concurrent consistency is broken!", phantomMisses, totalGets)
	} else {
		t.Logf("All %d Gets returned ok=true during concurrent writes (no phantom misses)", totalGets)
	}

	_, ok := kv.Get(key)
	if !ok {
		t.Error("Final check: key should exist after all goroutines complete")
	}
}

func TestConcurrentPutGet_MultipleKeys_NoPhantomMisses(t *testing.T) {
	kv := NewKVStore()

	numKeys := 200
	for i := 0; i < numKeys; i++ {
		kv.Put(fmt.Sprintf("k%03d", i), fmt.Sprintf("v%03d_initial", i))
	}

	var wg sync.WaitGroup
	var phantomMisses int64

	numWriters := 5
	numReaders := 15
	iterations := 500

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				idx := i % numKeys
				kv.Put(fmt.Sprintf("k%03d", idx), fmt.Sprintf("w%d_v%d", wid, i))
			}
		}(w)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				idx := i % numKeys
				key := fmt.Sprintf("k%03d", idx)
				_, ok := kv.Get(key)
				if !ok {
					atomic.AddInt64(&phantomMisses, 1)
					t.Errorf("PHANTOM MISS on key %q during concurrent Put/Get", key)
				}
			}
		}()
	}

	wg.Wait()

	if phantomMisses > 0 {
		t.Errorf("Found %d phantom misses across %d readers x %d iterations", phantomMisses, numReaders, iterations)
	}

	if kv.Count() != numKeys {
		t.Errorf("expected %d keys after concurrent ops, got %d", numKeys, kv.Count())
	}
}

func TestConcurrentBatchPutAndGet_Consistency(t *testing.T) {
	kv := NewKVStore()

	baseBatchSize := 10
	for i := 0; i < baseBatchSize; i++ {
		kv.Put(fmt.Sprintf("init_%d", i), fmt.Sprintf("initval_%d", i))
	}

	var wg sync.WaitGroup
	var phantomMisses int64
	var batchPutErrors int64

	numBatchWriters := 5
	numReaders := 10
	iterations := 200
	keysPerBatch := 5
	expectedDynamicKeys := numBatchWriters * iterations * keysPerBatch
	expectedTotalKeys := expectedDynamicKeys + baseBatchSize

	for w := 0; w < numBatchWriters; w++ {
		wg.Add(1)
		go func(wid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				pairs := make(map[string]string)
				for k := 0; k < keysPerBatch; k++ {
					pairs[fmt.Sprintf("batch_w%d_i%d_k%d", wid, i, k)] = fmt.Sprintf("v_w%d_%d_%d", wid, i, k)
				}
				pairs[fmt.Sprintf("init_%d", wid%baseBatchSize)] = fmt.Sprintf("updated_by_w%d_i%d", wid, i)
				err := kv.BatchPut(pairs)
				if err != nil {
					atomic.AddInt64(&batchPutErrors, 1)
					t.Errorf("BatchPut failed: %v", err)
				}
			}
		}(w)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func(rid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := fmt.Sprintf("init_%d", (rid+i)%baseBatchSize)
				_, ok := kv.Get(key)
				if !ok {
					atomic.AddInt64(&phantomMisses, 1)
					t.Errorf("PHANTOM MISS on pre-populated key %q during concurrent BatchPut", key)
				}
			}
		}(r)
	}

	wg.Wait()

	if batchPutErrors > 0 {
		t.Errorf("Found %d BatchPut errors during concurrent writes", batchPutErrors)
	}

	if phantomMisses > 0 {
		t.Errorf("Found %d phantom misses during concurrent BatchPut+Get", phantomMisses)
	}

	finalCount := kv.Count()
	if finalCount != expectedTotalKeys {
		t.Errorf("expected exactly %d keys (%d dynamic + %d init), got %d (diff: %+d)",
			expectedTotalKeys, expectedDynamicKeys, baseBatchSize,
			finalCount, finalCount-expectedTotalKeys)
	} else {
		t.Logf("Data integrity verified: %d total keys match expected count", finalCount)
	}

	samplesPerWriter := 10
	missingDynamicKeys := 0
	valueMismatches := 0
	for w := 0; w < numBatchWriters; w++ {
		for s := 0; s < samplesPerWriter; s++ {
			iterIdx := (s * iterations) / samplesPerWriter
			for k := 0; k < keysPerBatch; k++ {
				key := fmt.Sprintf("batch_w%d_i%d_k%d", w, iterIdx, k)
				expectedVal := fmt.Sprintf("v_w%d_%d_%d", w, iterIdx, k)
				val, ok := kv.Get(key)
				if !ok {
					missingDynamicKeys++
					t.Errorf("dynamic key %q missing after concurrent BatchPut (data integrity violation)", key)
				} else if val != expectedVal {
					valueMismatches++
					t.Errorf("dynamic key %q value mismatch: expected %q, got %q", key, expectedVal, val)
				}
			}
		}
	}
	if missingDynamicKeys == 0 && valueMismatches == 0 {
		t.Logf("Sampled %d dynamic keys across %d writers: all present and correct",
			samplesPerWriter*keysPerBatch*numBatchWriters, numBatchWriters)
	}

	for i := 0; i < baseBatchSize; i++ {
		key := fmt.Sprintf("init_%d", i)
		val, ok := kv.Get(key)
		if !ok {
			t.Errorf("init key %q missing after concurrent BatchPut", key)
		}
		if val == "" {
			t.Errorf("init key %q has empty value", key)
		}
	}
}

func TestConcurrentPutDeleteAndGet_NoInconsistency(t *testing.T) {
	kv := NewKVStore()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		kv.Put(fmt.Sprintf("cdkey_%d", i), fmt.Sprintf("cdval_%d", i))
	}

	var wg sync.WaitGroup
	var getErrors int64

	numPutters := 5
	numDeleters := 5
	numReaders := 10
	iterations := 300

	for p := 0; p < numPutters; p++ {
		wg.Add(1)
		go func(pid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				idx := (pid*iterations + i) % numKeys
				kv.Put(fmt.Sprintf("cdkey_%d", idx), fmt.Sprintf("p%d_cdval_%d", pid, i))
			}
		}(p)
	}

	for d := 0; d < numDeleters; d++ {
		wg.Add(1)
		go func(did int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				idx := (did*iterations + i) % numKeys
				kv.Delete(fmt.Sprintf("cdkey_%d", idx))
			}
		}(d)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				for k := 0; k < numKeys; k++ {
					key := fmt.Sprintf("cdkey_%d", k)
					val, ok := kv.Get(key)
					if ok && val == "" {
						atomic.AddInt64(&getErrors, 1)
						t.Errorf("INCONSISTENCY: key %q returned ok=true with empty value", key)
					}
				}
			}
		}()
	}

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("Detected %d value inconsistencies during concurrent Put/Delete/Get", getErrors)
	}
}
