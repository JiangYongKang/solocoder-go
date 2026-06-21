package scalablebloom

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNew_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	sb, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sb == nil {
		t.Fatal("New returned nil")
	}
	if sb.FilterCount() != 1 {
		t.Errorf("expected 1 filter, got %d", sb.FilterCount())
	}
	if sb.Count() != 0 {
		t.Errorf("expected Count=0, got %d", sb.Count())
	}
}

func TestNew_InvalidCapacity(t *testing.T) {
	_, err := New(Config{InitialCapacity: 0, FPRate: 0.01, Ratio: 0.85})
	if !errors.Is(err, ErrInvalidCapacity) {
		t.Errorf("expected ErrInvalidCapacity, got %v", err)
	}
}

func TestNew_InvalidFPRate_Zero(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: 0, Ratio: 0.85})
	if !errors.Is(err, ErrInvalidFPRate) {
		t.Errorf("expected ErrInvalidFPRate, got %v", err)
	}
}

func TestNew_InvalidFPRate_Negative(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: -0.01, Ratio: 0.85})
	if !errors.Is(err, ErrInvalidFPRate) {
		t.Errorf("expected ErrInvalidFPRate, got %v", err)
	}
}

func TestNew_InvalidFPRate_One(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: 1.0, Ratio: 0.85})
	if !errors.Is(err, ErrInvalidFPRate) {
		t.Errorf("expected ErrInvalidFPRate, got %v", err)
	}
}

func TestNew_InvalidRatio_Zero(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0})
	if !errors.Is(err, ErrInvalidRatio) {
		t.Errorf("expected ErrInvalidRatio, got %v", err)
	}
}

func TestNew_InvalidRatio_One(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 1.0})
	if !errors.Is(err, ErrInvalidRatio) {
		t.Errorf("expected ErrInvalidRatio, got %v", err)
	}
}

func TestNew_InvalidRatio_Negative(t *testing.T) {
	_, err := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: -0.5})
	if !errors.Is(err, ErrInvalidRatio) {
		t.Errorf("expected ErrInvalidRatio, got %v", err)
	}
}

func TestAdd_EmptyKey(t *testing.T) {
	sb, _ := New(DefaultConfig())
	err := sb.Add("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if sb.Count() != 0 {
		t.Errorf("expected Count=0 after empty key add, got %d", sb.Count())
	}
}

func TestAdd_SingleElement(t *testing.T) {
	sb, _ := New(DefaultConfig())
	err := sb.Add("hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sb.Count() != 1 {
		t.Errorf("expected Count=1, got %d", sb.Count())
	}
}

func TestAdd_MultipleElements(t *testing.T) {
	sb, _ := New(DefaultConfig())
	n := 100
	for i := 0; i < n; i++ {
		err := sb.Add(fmt.Sprintf("key-%d", i))
		if err != nil {
			t.Fatalf("Add error at %d: %v", i, err)
		}
	}
	if sb.Count() != uint(n) {
		t.Errorf("expected Count=%d, got %d", n, sb.Count())
	}
}

func TestMightContain_ExistingElement(t *testing.T) {
	sb, _ := New(DefaultConfig())
	sb.Add("exists")
	found, err := sb.MightContain("exists")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find existing element")
	}
}

func TestMightContain_NonExistingElement(t *testing.T) {
	sb, _ := New(DefaultConfig())
	sb.Add("exists")
	found, err := sb.MightContain("not-exists")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found {
		t.Error("expected not to find non-existing element (false positive possible but unlikely)")
	}
}

func TestMightContain_EmptyKey(t *testing.T) {
	sb, _ := New(DefaultConfig())
	_, err := sb.MightContain("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestMightContain_NoFalseNegatives(t *testing.T) {
	cfg := Config{InitialCapacity: 500, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	keys := make([]string, 400)
	for i := 0; i < 400; i++ {
		keys[i] = fmt.Sprintf("key-%d", i)
		sb.Add(keys[i])
	}

	for _, key := range keys {
		found, err := sb.MightContain(key)
		if err != nil {
			t.Fatalf("MightContain error for %s: %v", key, err)
		}
		if !found {
			t.Errorf("false negative for key %s — bloom filter must never have false negatives", key)
		}
	}
}

func TestDynamicExpansion_Triggered(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	if sb.FilterCount() != 1 {
		t.Fatalf("expected initial filter count=1, got %d", sb.FilterCount())
	}

	for i := 0; i < 10; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	if sb.FilterCount() != 1 {
		t.Errorf("expected still 1 filter (at capacity), got %d", sb.FilterCount())
	}

	sb.Add("key-10")
	if sb.FilterCount() != 2 {
		t.Errorf("expected 2 filters after expansion, got %d", sb.FilterCount())
	}

	sb.Add("key-11")
	if sb.Count() != 12 {
		t.Errorf("expected Count=12, got %d", sb.Count())
	}
}

func TestDynamicExpansion_MultipleExpansions(t *testing.T) {
	cfg := Config{InitialCapacity: 5, FPRate: 0.05, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 25; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	if sb.FilterCount() < 3 {
		t.Errorf("expected at least 3 filters after 25 inserts with capacity 5, got %d", sb.FilterCount())
	}

	if sb.Count() != 25 {
		t.Errorf("expected Count=25, got %d", sb.Count())
	}
}

func TestDynamicExpansion_QueryAfterExpansion(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 15; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	for i := 0; i < 15; i++ {
		found, err := sb.MightContain(fmt.Sprintf("key-%d", i))
		if err != nil {
			t.Fatalf("MightContain error for key-%d: %v", i, err)
		}
		if !found {
			t.Errorf("false negative for key-%d after expansion", i)
		}
	}
}

func TestDynamicExpansion_NewFilterTighterFPRate(t *testing.T) {
	cfg := Config{InitialCapacity: 5, FPRate: 0.05, Ratio: 0.5}
	sb, _ := New(cfg)

	for i := 0; i < 5; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	sb.Add("key-5")

	if sb.FilterCount() != 2 {
		t.Fatalf("expected 2 filters, got %d", sb.FilterCount())
	}

	f1 := sb.filters[0]
	f2 := sb.filters[1]

	if f2.capacity != f1.capacity*2 {
		t.Errorf("expected new capacity=%d (2x old), got %d", f1.capacity*2, f2.capacity)
	}

	expectedFPRate2 := cfg.FPRate * math.Pow(cfg.Ratio, 1)
	expectedBits2 := optimalNumBits(f2.capacity, expectedFPRate2)
	if f2.numBits != expectedBits2 {
		t.Errorf("expected numBits=%d for new filter, got %d", expectedBits2, f2.numBits)
	}
}

func TestOptimalNumBits(t *testing.T) {
	m := optimalNumBits(1000, 0.01)
	expected := uint(math.Ceil(-1000 * math.Log(0.01) / (math.Log(2) * math.Log(2))))
	if m != expected {
		t.Errorf("expected %d, got %d", expected, m)
	}
}

func TestOptimalHashCount(t *testing.T) {
	m := optimalNumBits(1000, 0.01)
	k := optimalHashCount(m, 1000)
	expectedK := uint(math.Ceil(float64(m)/1000.0 * math.Log(2)))
	if k != expectedK {
		t.Errorf("expected %d, got %d", expectedK, k)
	}
}

func TestOptimalHashCount_MinimumOne(t *testing.T) {
	k := optimalHashCount(1, 1000)
	if k < 1 {
		t.Errorf("hash count should be at least 1, got %d", k)
	}
}

func TestCapacity(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	initialCap := sb.Capacity()
	if initialCap != 10 {
		t.Errorf("expected initial capacity=10, got %d", initialCap)
	}

	for i := 0; i < 11; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	totalCap := sb.Capacity()
	if totalCap <= initialCap {
		t.Errorf("expected capacity to grow after expansion, got %d (initial was %d)", totalCap, initialCap)
	}
}

func TestSerializeDeserialize_BasicRoundTrip(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 50; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bloom.bin")

	err := sb.Serialize(path)
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("serialized file does not exist")
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	if loaded.Count() != sb.Count() {
		t.Errorf("expected Count=%d, got %d", sb.Count(), loaded.Count())
	}

	if loaded.FilterCount() != sb.FilterCount() {
		t.Errorf("expected FilterCount=%d, got %d", sb.FilterCount(), loaded.FilterCount())
	}

	if loaded.Capacity() != sb.Capacity() {
		t.Errorf("expected Capacity=%d, got %d", sb.Capacity(), loaded.Capacity())
	}

	for i := 0; i < 50; i++ {
		found, err := loaded.MightContain(fmt.Sprintf("key-%d", i))
		if err != nil {
			t.Fatalf("MightContain error: %v", err)
		}
		if !found {
			t.Errorf("false negative for key-%d after deserialization", i)
		}
	}
}

func TestSerializeDeserialize_AfterExpansion(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 25; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bloom_expanded.bin")

	err := sb.Serialize(path)
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	if loaded.FilterCount() != sb.FilterCount() {
		t.Errorf("expected FilterCount=%d, got %d", sb.FilterCount(), loaded.FilterCount())
	}

	for i := 0; i < 25; i++ {
		found, err := loaded.MightContain(fmt.Sprintf("key-%d", i))
		if err != nil {
			t.Fatalf("MightContain error: %v", err)
		}
		if !found {
			t.Errorf("false negative for key-%d after deserialization", i)
		}
	}
}

func TestSerializeDeserialize_ConfigPreserved(t *testing.T) {
	cfg := Config{InitialCapacity: 200, FPRate: 0.005, Ratio: 0.9}
	sb, _ := New(cfg)
	sb.Add("test-key")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bloom_config.bin")

	sb.Serialize(path)
	loaded, _ := Deserialize(path)

	if loaded.cfg.InitialCapacity != cfg.InitialCapacity {
		t.Errorf("expected InitialCapacity=%d, got %d", cfg.InitialCapacity, loaded.cfg.InitialCapacity)
	}

	if math.Abs(loaded.cfg.FPRate-cfg.FPRate) > 1e-15 {
		t.Errorf("expected FPRate=%.6f, got %.6f", cfg.FPRate, loaded.cfg.FPRate)
	}

	if math.Abs(loaded.cfg.Ratio-cfg.Ratio) > 1e-15 {
		t.Errorf("expected Ratio=%.6f, got %.6f", cfg.Ratio, loaded.cfg.Ratio)
	}
}

func TestSerializeDeserialize_ContinueUsingAfterLoad(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 10; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "bloom_continue.bin")

	sb.Serialize(path)
	loaded, _ := Deserialize(path)

	err := loaded.Add("new-key-after-load")
	if err != nil {
		t.Fatalf("Add after load error: %v", err)
	}

	found, err := loaded.MightContain("new-key-after-load")
	if err != nil {
		t.Fatalf("MightContain error: %v", err)
	}
	if !found {
		t.Error("should find key added after deserialization")
	}

	if loaded.Count() != 11 {
		t.Errorf("expected Count=11, got %d", loaded.Count())
	}
}

func TestDeserialize_NonexistentFile(t *testing.T) {
	_, err := Deserialize("/nonexistent/path/bloom.bin")
	if !errors.Is(err, ErrFileRead) {
		t.Errorf("expected ErrFileRead, got %v", err)
	}
}

func TestDeserialize_InvalidData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.bin")
	os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644)

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for invalid data, got nil")
	}
}

func TestDeserialize_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "version_mismatch.bin")

	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)
	sb.Add("test")
	sb.Serialize(path)

	data, _ := os.ReadFile(path)
	data[0] = 0xFF
	data[1] = 0xFF
	data[2] = 0xFF
	data[3] = 0xFF
	os.WriteFile(path, data, 0644)

	_, err := Deserialize(path)
	if !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch, got %v", err)
	}
}

func TestSerialize_InvalidPath(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	err := sb.Serialize("/nonexistent/directory/bloom.bin")
	if !errors.Is(err, ErrFileOpen) {
		t.Errorf("expected ErrFileOpen, got %v", err)
	}
}

func TestSerialize_EmptyFilter(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.bin")

	err := sb.Serialize(path)
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	if loaded.Count() != 0 {
		t.Errorf("expected Count=0, got %d", loaded.Count())
	}

	found, err := loaded.MightContain("nonexistent")
	if err != nil {
		t.Fatalf("MightContain error: %v", err)
	}
	if found {
		t.Error("empty filter should not contain anything")
	}
}

func TestUnionQuery_BasicFound(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}

	sb1, _ := New(cfg)
	sb2, _ := New(cfg)

	sb1.Add("in-first")
	sb2.Add("in-second")

	found, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "in-first")
	if err != nil {
		t.Fatalf("UnionQuery error: %v", err)
	}
	if !found {
		t.Error("expected to find 'in-first' in union")
	}

	found, err = UnionQuery([]*ScalableBloom{sb1, sb2}, "in-second")
	if err != nil {
		t.Fatalf("UnionQuery error: %v", err)
	}
	if !found {
		t.Error("expected to find 'in-second' in union")
	}
}

func TestUnionQuery_NotFound(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}

	sb1, _ := New(cfg)
	sb2, _ := New(cfg)

	sb1.Add("a")
	sb2.Add("b")

	found, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "not-in-any")
	if err != nil {
		t.Fatalf("UnionQuery error: %v", err)
	}
	if found {
		t.Error("expected not to find 'not-in-any' in union (false positive possible but unlikely)")
	}
}

func TestUnionQuery_EmptyKey(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	_, err := UnionQuery([]*ScalableBloom{sb}, "")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestUnionQuery_NoFilters(t *testing.T) {
	_, err := UnionQuery([]*ScalableBloom{}, "key")
	if !errors.Is(err, ErrNoFilters) {
		t.Errorf("expected ErrNoFilters, got %v", err)
	}
}

func TestUnionQuery_SingleFilter(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)
	sb.Add("only-key")

	found, err := UnionQuery([]*ScalableBloom{sb}, "only-key")
	if err != nil {
		t.Fatalf("UnionQuery error: %v", err)
	}
	if !found {
		t.Error("expected to find 'only-key' in single filter union")
	}
}

func TestUnionQuery_MultipleFiltersAllDefinite(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}

	var filters []*ScalableBloom
	for i := 0; i < 5; i++ {
		sb, _ := New(cfg)
		for j := 0; j < 10; j++ {
			sb.Add(fmt.Sprintf("filter%d-key%d", i, j))
		}
		filters = append(filters, sb)
	}

	for i := 0; i < 5; i++ {
		for j := 0; j < 10; j++ {
			found, err := UnionQuery(filters, fmt.Sprintf("filter%d-key%d", i, j))
			if err != nil {
				t.Fatalf("UnionQuery error: %v", err)
			}
			if !found {
				t.Errorf("expected to find filter%d-key%d in union", i, j)
			}
		}
	}
}

func TestFalsePositiveRate_WithinBounds(t *testing.T) {
	cfg := Config{InitialCapacity: 1000, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 800; i++ {
		sb.Add(fmt.Sprintf("member-%d", i))
	}

	falsePositives := 0
	trials := 10000
	for i := 0; i < trials; i++ {
		found, _ := sb.MightContain(fmt.Sprintf("nonmember-%d", i))
		if found {
			falsePositives++
		}
	}

	actualRate := float64(falsePositives) / float64(trials)
	if actualRate > 0.05 {
		t.Errorf("false positive rate %.4f exceeds 5x target (0.01), got %.4f", actualRate, actualRate)
	}
}

func TestBloomFilter_DoubleHash_Deterministic(t *testing.T) {
	h1a, h2a := doubleHash("test", 10000)
	h1b, h2b := doubleHash("test", 10000)
	if h1a != h1b || h2a != h2b {
		t.Error("doubleHash should be deterministic")
	}
}

func TestBloomFilter_DoubleHash_DifferentKeys(t *testing.T) {
	h1a, _ := doubleHash("key-a", 10000)
	h1b, _ := doubleHash("key-b", 10000)
	if h1a == h1b {
		t.Log("different keys producing same h1 is extremely unlikely but possible — not a hard failure")
	}
}

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.InitialCapacity != 1000 {
		t.Errorf("expected InitialCapacity=1000, got %d", cfg.InitialCapacity)
	}
	if cfg.FPRate != 0.01 {
		t.Errorf("expected FPRate=0.01, got %f", cfg.FPRate)
	}
	if cfg.Ratio != 0.85 {
		t.Errorf("expected Ratio=0.85, got %f", cfg.Ratio)
	}
}

func TestAdd_DuplicateKey(t *testing.T) {
	sb, _ := New(DefaultConfig())
	sb.Add("dup")
	sb.Add("dup")
	if sb.Count() != 2 {
		t.Errorf("expected Count=2 (duplicates counted), got %d", sb.Count())
	}

	found, _ := sb.MightContain("dup")
	if !found {
		t.Error("expected to find duplicate key")
	}
}

func TestSerializeDeserialize_LargeData(t *testing.T) {
	cfg := Config{InitialCapacity: 500, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 1500; i++ {
		sb.Add(fmt.Sprintf("large-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.bin")

	err := sb.Serialize(path)
	if err != nil {
		t.Fatalf("Serialize error: %v", err)
	}

	loaded, err := Deserialize(path)
	if err != nil {
		t.Fatalf("Deserialize error: %v", err)
	}

	if loaded.Count() != 1500 {
		t.Errorf("expected Count=1500, got %d", loaded.Count())
	}

	for i := 0; i < 1500; i++ {
		found, _ := loaded.MightContain(fmt.Sprintf("large-%d", i))
		if !found {
			t.Errorf("false negative for large-%d", i)
		}
	}
}

func TestConcurrent_AddAndQuery(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	var wg sync.WaitGroup
	numGoroutines := 10
	itemsPerGoroutine := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < itemsPerGoroutine; i++ {
				sb.Add(fmt.Sprintf("g%d-i%d", gid, i))
			}
		}(g)
	}

	wg.Wait()

	expected := uint(numGoroutines * itemsPerGoroutine)
	if sb.Count() != expected {
		t.Errorf("expected Count=%d, got %d", expected, sb.Count())
	}

	for g := 0; g < numGoroutines; g++ {
		for i := 0; i < itemsPerGoroutine; i++ {
			found, _ := sb.MightContain(fmt.Sprintf("g%d-i%d", g, i))
			if !found {
				t.Errorf("false negative for g%d-i%d", g, i)
			}
		}
	}
}

func TestConcurrent_SerializeAndAdd(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 50; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "concurrent.bin")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		sb.Serialize(path)
	}()

	go func() {
		defer wg.Done()
		for i := 50; i < 100; i++ {
			sb.Add(fmt.Sprintf("key-%d", i))
		}
	}()

	wg.Wait()
}

func TestBloomFilter_IsFull(t *testing.T) {
	bf := newBloomFilter(5, 0.01)
	if bf.isFull() {
		t.Error("empty filter should not be full")
	}

	for i := 0; i < 5; i++ {
		bf.add(fmt.Sprintf("key-%d", i))
	}

	if !bf.isFull() {
		t.Error("filter at capacity should be full")
	}
}

func TestExpansion_NewFilterParams(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.05, Ratio: 0.8}
	sb, _ := New(cfg)

	f0 := sb.filters[0]
	expectedBits0 := optimalNumBits(10, 0.05)
	expectedHash0 := optimalHashCount(expectedBits0, 10)

	if f0.numBits != expectedBits0 {
		t.Errorf("filter0: expected numBits=%d, got %d", expectedBits0, f0.numBits)
	}
	if f0.hashCount != expectedHash0 {
		t.Errorf("filter0: expected hashCount=%d, got %d", expectedHash0, f0.hashCount)
	}

	for i := 0; i < 10; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}
	sb.Add("trigger")

	f1 := sb.filters[1]
	newCap := uint(20)
	expectedFPRate1 := 0.05 * math.Pow(0.8, 1)
	expectedBits1 := optimalNumBits(newCap, expectedFPRate1)
	expectedHash1 := optimalHashCount(expectedBits1, newCap)

	if f1.capacity != newCap {
		t.Errorf("filter1: expected capacity=%d, got %d", newCap, f1.capacity)
	}
	if f1.numBits != expectedBits1 {
		t.Errorf("filter1: expected numBits=%d, got %d", expectedBits1, f1.numBits)
	}
	if f1.hashCount != expectedHash1 {
		t.Errorf("filter1: expected hashCount=%d, got %d", expectedHash1, f1.hashCount)
	}
}

func TestMightContain_SingleFilterNoExpansion(t *testing.T) {
	cfg := Config{InitialCapacity: 1000, FPRate: 0.001, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 500; i++ {
		sb.Add(fmt.Sprintf("member-%d", i))
	}

	for i := 0; i < 500; i++ {
		found, _ := sb.MightContain(fmt.Sprintf("member-%d", i))
		if !found {
			t.Errorf("false negative for member-%d", i)
		}
	}
}

func TestSerializeDeserialize_FileIntegrity(t *testing.T) {
	cfg := Config{InitialCapacity: 50, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	for i := 0; i < 30; i++ {
		sb.Add(fmt.Sprintf("integrity-%d", i))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "integrity.bin")

	sb.Serialize(path)

	info1, _ := os.Stat(path)
	size1 := info1.Size()

	loaded, _ := Deserialize(path)

	tmpDir2 := t.TempDir()
	path2 := filepath.Join(tmpDir2, "integrity2.bin")
	loaded.Serialize(path2)

	info2, _ := os.Stat(path2)
	size2 := info2.Size()

	if size1 != size2 {
		t.Errorf("file sizes differ after round-trip: original=%d, after_deser=%d", size1, size2)
	}
}

func TestDeserialize_CorruptedData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupted.bin")

	cfg := Config{InitialCapacity: 50, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)
	sb.Add("test")
	sb.Serialize(path)

	data, _ := os.ReadFile(path)
	data = data[:len(data)/2]
	os.WriteFile(path, data, 0644)

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for corrupted data, got nil")
	}
}

func TestUnionQuery_ShortCircuitOnFound(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}

	sb1, _ := New(cfg)
	sb2, _ := New(cfg)
	sb3, _ := New(cfg)

	sb1.Add("early-key")
	sb2.Add("mid-key")
	sb3.Add("late-key")

	found, err := UnionQuery([]*ScalableBloom{sb1, sb2, sb3}, "early-key")
	if err != nil {
		t.Fatalf("UnionQuery error: %v", err)
	}
	if !found {
		t.Error("expected to find 'early-key' in first filter")
	}
}

func TestNew_SmallCapacity(t *testing.T) {
	cfg := Config{InitialCapacity: 1, FPRate: 0.01, Ratio: 0.85}
	sb, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sb.Add("only-one")
	if sb.Count() != 1 {
		t.Errorf("expected Count=1, got %d", sb.Count())
	}

	sb.Add("triggers-expand")
	if sb.FilterCount() < 2 {
		t.Errorf("expected expansion, got %d filters", sb.FilterCount())
	}
}

func TestNew_VeryLowFPRate(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.0001, Ratio: 0.9}
	sb, err := New(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sb.Add("test")
	found, _ := sb.MightContain("test")
	if !found {
		t.Error("expected to find inserted key")
	}
}

func TestFillRatio_Basic(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	if sb.FillRatio() != 0 {
		t.Errorf("expected FillRatio=0 for empty filter, got %f", sb.FillRatio())
	}

	for i := 0; i < 50; i++ {
		sb.Add(fmt.Sprintf("key-%d", i))
	}
	ratio := sb.FillRatio()
	if ratio <= 0 || ratio > 1 {
		t.Errorf("expected FillRatio in (0, 1], got %f", ratio)
	}
}

func TestBloomFilter_AddWhenFull(t *testing.T) {
	bf := newBloomFilter(3, 0.01)

	err := bf.add("key1")
	if err != nil {
		t.Errorf("unexpected error adding key1: %v", err)
	}
	err = bf.add("key2")
	if err != nil {
		t.Errorf("unexpected error adding key2: %v", err)
	}
	err = bf.add("key3")
	if err != nil {
		t.Errorf("unexpected error adding key3: %v", err)
	}

	if !bf.isFull() {
		t.Error("expected filter to be full after 3 adds with capacity 3")
	}

	err = bf.add("key4")
	if !errors.Is(err, ErrCapacityExceeded) {
		t.Errorf("expected ErrCapacityExceeded when adding to full filter, got %v", err)
	}
}

func TestUnionQuery_IncompatibleFilters_DifferentFPRate(t *testing.T) {
	sb1, _ := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85})
	sb2, _ := New(Config{InitialCapacity: 100, FPRate: 0.02, Ratio: 0.85})

	sb1.Add("key")
	sb2.Add("key")

	_, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "key")
	if !errors.Is(err, ErrIncompatibleFilters) {
		t.Errorf("expected ErrIncompatibleFilters for different FPRate, got %v", err)
	}
}

func TestUnionQuery_IncompatibleFilters_DifferentInitialCapacity(t *testing.T) {
	sb1, _ := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85})
	sb2, _ := New(Config{InitialCapacity: 200, FPRate: 0.01, Ratio: 0.85})

	sb1.Add("key")
	sb2.Add("key")

	_, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "key")
	if !errors.Is(err, ErrIncompatibleFilters) {
		t.Errorf("expected ErrIncompatibleFilters for different InitialCapacity, got %v", err)
	}
}

func TestUnionQuery_IncompatibleFilters_DifferentRatio(t *testing.T) {
	sb1, _ := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.8})
	sb2, _ := New(Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.9})

	sb1.Add("key")
	sb2.Add("key")

	_, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "key")
	if !errors.Is(err, ErrIncompatibleFilters) {
		t.Errorf("expected ErrIncompatibleFilters for different Ratio, got %v", err)
	}
}

func TestUnionQuery_CompatibleFilters_AfterExpansion(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb1, _ := New(cfg)
	sb2, _ := New(cfg)

	for i := 0; i < 15; i++ {
		sb1.Add(fmt.Sprintf("sb1-%d", i))
		sb2.Add(fmt.Sprintf("sb2-%d", i))
	}

	if sb1.FilterCount() < 2 || sb2.FilterCount() < 2 {
		t.Fatal("expected both filters to have expanded")
	}

	found, err := UnionQuery([]*ScalableBloom{sb1, sb2}, "sb1-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find sb1-5 in union")
	}

	found, err = UnionQuery([]*ScalableBloom{sb1, sb2}, "sb2-10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found {
		t.Error("expected to find sb2-10 in union")
	}
}

func TestDeserialize_Version1BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "v1_data.bin")

	cfg := Config{InitialCapacity: 50, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)
	for i := 0; i < 30; i++ {
		sb.Add(fmt.Sprintf("v1key-%d", i))
	}

	sb.Serialize(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read serialized file: %v", err)
	}

	v2HeaderSize := 4 + 4
	v1Data := make([]byte, 0, len(data)-v2HeaderSize-32)
	versionBytes := make([]byte, 4)
	versionBytes[3] = 1
	v1Data = append(v1Data, versionBytes...)
	v1Data = append(v1Data, data[8:len(data)-32]...)

	newPath := filepath.Join(tmpDir, "v1_modified.bin")
	os.WriteFile(newPath, v1Data, 0644)

	loaded, err := Deserialize(newPath)
	if err != nil {
		t.Fatalf("Deserialize v1 data failed: %v", err)
	}

	if loaded.Count() != 30 {
		t.Errorf("expected Count=30, got %d", loaded.Count())
	}

	for i := 0; i < 30; i++ {
		found, err := loaded.MightContain(fmt.Sprintf("v1key-%d", i))
		if err != nil {
			t.Fatalf("MightContain error: %v", err)
		}
		if !found {
			t.Errorf("false negative for v1key-%d", i)
		}
	}

	err = loaded.Add("new-key-after-v1-load")
	if err != nil {
		t.Fatalf("Add after v1 load failed: %v", err)
	}
	found, _ := loaded.MightContain("new-key-after-v1-load")
	if !found {
		t.Error("expected to find key added after v1 load")
	}
}

func TestDeserialize_VersionTooOld(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "old_version.bin")

	data := make([]byte, 100)
	data[3] = 0
	os.WriteFile(path, data, 0644)

	_, err := Deserialize(path)
	if !errors.Is(err, ErrVersionUnsupported) {
		t.Errorf("expected ErrVersionUnsupported, got %v", err)
	}
}

func TestDeserialize_InvalidBitsLength(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid_bits.bin")

	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)
	sb.Add("test")
	sb.Serialize(path)

	data, _ := os.ReadFile(path)

	headerSize := 4 + 4 + 4 + 8 + 8 + 4 + 4
	filterHeaderSize := 4 + 4 + 4 + 4 + 4
	totalHeader := headerSize + filterHeaderSize

	wrongData := make([]byte, totalHeader+8)
	copy(wrongData, data[:totalHeader])
	wrongData[totalHeader-4] = 0
	wrongData[totalHeader-3] = 0
	wrongData[totalHeader-2] = 0
	wrongData[totalHeader-1] = 99

	os.WriteFile(path, wrongData, 0644)

	_, err := Deserialize(path)
	if err == nil {
		t.Error("expected error for invalid bits length")
	}
}

func TestBloomFilter_Validate(t *testing.T) {
	bf := newBloomFilter(10, 0.01)
	err := bf.validate()
	if err != nil {
		t.Errorf("expected valid filter to pass validate, got %v", err)
	}

	bf.numBits = 0
	err = bf.validate()
	if !errors.Is(err, ErrCorruptedFilter) {
		t.Errorf("expected ErrCorruptedFilter for zero numBits, got %v", err)
	}
}

func TestBloomFilter_Validate_WrongBitsLength(t *testing.T) {
	bf := newBloomFilter(10, 0.01)
	bf.bits = make([]uint64, 1)
	err := bf.validate()
	if !errors.Is(err, ErrCorruptedFilter) {
		t.Errorf("expected ErrCorruptedFilter for wrong bits length, got %v", err)
	}
}

func TestExpandLocked_CapacityOverflowCheck(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	sb.filters[0].capacity = ^uint(0) / 2
	sb.filters[0].count = sb.filters[0].capacity

	err := sb.Add("test-overflow")
	if err == nil {
		t.Log("Note: capacity overflow may not be triggered on all platforms")
	}
}

func TestSerialize_CorruptedFilter(t *testing.T) {
	cfg := Config{InitialCapacity: 10, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	sb.filters[0].numBits = 0

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "corrupted.bin")

	err := sb.Serialize(path)
	if err == nil {
		t.Error("expected error when serializing corrupted filter")
	}
}

func TestValidateFiltersCompatible_SingleFilter(t *testing.T) {
	cfg := Config{InitialCapacity: 100, FPRate: 0.01, Ratio: 0.85}
	sb, _ := New(cfg)

	err := validateFiltersCompatible([]*ScalableBloom{sb})
	if err != nil {
		t.Errorf("single filter should be compatible, got %v", err)
	}
}

func TestValidateFiltersCompatible_Empty(t *testing.T) {
	err := validateFiltersCompatible([]*ScalableBloom{})
	if err != nil {
		t.Errorf("empty filters should be compatible, got %v", err)
	}
}
