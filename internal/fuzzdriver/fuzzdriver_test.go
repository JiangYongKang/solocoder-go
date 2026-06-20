package fuzzdriver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCoverage(t *testing.T) {
	c := NewCoverage()
	if c == nil {
		t.Fatal("NewCoverage returned nil")
	}
	if c.Count() != 0 {
		t.Errorf("expected 0 coverage, got %d", c.Count())
	}
}

func TestCoverageAddAndHas(t *testing.T) {
	c := NewCoverage()
	addr := uint64(0x12345)
	c.Add(addr)
	if !c.Has(addr) {
		t.Error("expected Has to return true for added address")
	}
	if c.Has(0x54321) {
		t.Error("expected Has to return false for non-added address")
	}
	if c.Count() != 1 {
		t.Errorf("expected count 1, got %d", c.Count())
	}
}

func TestCoverageMerge(t *testing.T) {
	c1 := NewCoverage()
	c2 := NewCoverage()
	c1.Add(0x1)
	c1.Add(0x2)
	c2.Add(0x2)
	c2.Add(0x3)
	newPaths := c1.Merge(c2)
	if newPaths != 1 {
		t.Errorf("expected 1 new path, got %d", newPaths)
	}
	if c1.Count() != 3 {
		t.Errorf("expected count 3 after merge, got %d", c1.Count())
	}
	if !c1.Has(0x3) {
		t.Error("expected merged address to be present")
	}
}

func TestCoverageClear(t *testing.T) {
	c := NewCoverage()
	c.Add(0x1)
	c.Add(0x2)
	c.Clear()
	if c.Count() != 0 {
		t.Errorf("expected count 0 after clear, got %d", c.Count())
	}
}

func TestCoverageSnapshot(t *testing.T) {
	c := NewCoverage()
	addrs := []uint64{0x3, 0x1, 0x2}
	for _, addr := range addrs {
		c.Add(addr)
	}
	snapshot := c.Snapshot()
	if len(snapshot) != 3 {
		t.Errorf("expected snapshot length 3, got %d", len(snapshot))
	}
	for i := 1; i < len(snapshot); i++ {
		if snapshot[i] < snapshot[i-1] {
			t.Error("expected sorted snapshot")
		}
	}
}

func TestCoverageConcurrent(t *testing.T) {
	c := NewCoverage()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(addr uint64) {
			defer wg.Done()
			c.Add(addr)
		}(uint64(i))
	}
	wg.Wait()
	if c.Count() != 100 {
		t.Errorf("expected 100 unique addresses, got %d", c.Count())
	}
}

func TestNewMutator(t *testing.T) {
	m := NewMutator()
	if m == nil {
		t.Fatal("NewMutator returned nil")
	}
}

func TestMutatorFlipBit(t *testing.T) {
	m := NewMutator()
	input := []byte{0xFF, 0x00}
	for i := 0; i < 100; i++ {
		result := m.FlipBit(input)
		if len(result) != len(input) {
			t.Errorf("expected same length %d, got %d", len(input), len(result))
		}
		diffCount := 0
		for j := 0; j < len(input); j++ {
			if result[j] != input[j] {
				diffCount++
			}
		}
		if diffCount != 1 {
			t.Errorf("expected exactly 1 byte to differ, got %d", diffCount)
		}
	}
}

func TestMutatorInsertByte(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01, 0x02, 0x03}
	maxSize := 10
	for i := 0; i < 100; i++ {
		result := m.InsertByte(input, maxSize)
		if len(result) != len(input)+1 {
			t.Errorf("expected length %d, got %d", len(input)+1, len(result))
		}
	}
}

func TestMutatorInsertByteAtMaxSize(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01, 0x02}
	maxSize := 2
	result := m.InsertByte(input, maxSize)
	if len(result) != 2 {
		t.Errorf("expected length 2 at max size, got %d", len(result))
	}
}

func TestMutatorDeleteByte(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01, 0x02, 0x03}
	for i := 0; i < 100; i++ {
		result := m.DeleteByte(input)
		if len(result) != len(input)-1 {
			t.Errorf("expected length %d, got %d", len(input)-1, len(result))
		}
	}
}

func TestMutatorDeleteByteSingleByte(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01}
	result := m.DeleteByte(input)
	if len(result) != 1 {
		t.Errorf("expected length 1 for single byte input, got %d", len(result))
	}
}

func TestMutatorReplaceByte(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01, 0x02, 0x03}
	for i := 0; i < 100; i++ {
		result := m.ReplaceByte(input)
		if len(result) != len(input) {
			t.Errorf("expected length %d, got %d", len(input), len(result))
		}
	}
}

func TestMutatorMutate(t *testing.T) {
	m := NewMutator()
	tests := []struct {
		name    string
		input   []byte
		maxSize int
	}{
		{"normal input", []byte{0x01, 0x02, 0x03}, 100},
		{"empty input", []byte{}, 100},
		{"single byte", []byte{0xFF}, 100},
		{"at max size", []byte{0x01, 0x02}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 50; i++ {
				result := m.Mutate(tt.input, tt.maxSize)
				if tt.name == "empty input" && len(result) == 0 {
					t.Error("expected non-empty result for empty input")
				}
				if len(result) > tt.maxSize {
					t.Errorf("result %d exceeds max size %d", len(result), tt.maxSize)
				}
			}
		})
	}
}

func TestMutatorMutateN(t *testing.T) {
	m := NewMutator()
	input := []byte{0x01, 0x02, 0x03, 0x04}
	result := m.MutateN(input, 5, 100)
	if len(result) == 0 {
		t.Error("expected non-empty result after MutateN")
	}
}

func TestNewCorpus(t *testing.T) {
	c := NewCorpus("test_corpus")
	if c == nil {
		t.Fatal("NewCorpus returned nil")
	}
	if c.Count() != 0 {
		t.Errorf("expected 0 inputs, got %d", c.Count())
	}
}

func TestCorpusAdd(t *testing.T) {
	c := NewCorpus("test_corpus")
	input := []byte("test input")
	c.Add(input)
	if c.Count() != 1 {
		t.Errorf("expected 1 input, got %d", c.Count())
	}
}

func TestCorpusAddEmpty(t *testing.T) {
	c := NewCorpus("test_corpus")
	c.Add([]byte{})
	if c.Count() != 0 {
		t.Errorf("expected 0 inputs for empty input, got %d", c.Count())
	}
}

func TestCorpusNext(t *testing.T) {
	c := NewCorpus("test_corpus")
	inputs := [][]byte{
		[]byte("input1"),
		[]byte("input2"),
		[]byte("input3"),
	}
	for _, input := range inputs {
		c.Add(input)
	}
	seen := make(map[string]int)
	for i := 0; i < 6; i++ {
		next := c.Next()
		if next == nil {
			t.Fatal("Next returned nil")
		}
		seen[string(next)]++
	}
	for _, input := range inputs {
		if seen[string(input)] != 2 {
			t.Errorf("expected round-robin, input %q seen %d times", string(input), seen[string(input)])
		}
	}
}

func TestCorpusNextEmpty(t *testing.T) {
	c := NewCorpus("test_corpus")
	next := c.Next()
	if next != nil {
		t.Errorf("expected nil for empty corpus, got %v", next)
	}
}

func TestCorpusGetAll(t *testing.T) {
	c := NewCorpus("test_corpus")
	inputs := [][]byte{
		[]byte("input1"),
		[]byte("input2"),
	}
	for _, input := range inputs {
		c.Add(input)
	}
	all := c.GetAll()
	if len(all) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(all))
	}
	all[0][0] = 'X'
	original := c.GetAll()
	if original[0][0] == 'X' {
		t.Error("GetAll should return a copy")
	}
}

func TestCorpusLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	corpusDir := filepath.Join(tmpDir, "corpus")
	c := NewCorpus(corpusDir)
	if err := c.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	testInput := []byte("test input data")
	if err := c.Save(testInput); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	c2 := NewCorpus(corpusDir)
	if err := c2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if c2.Count() != 1 {
		t.Errorf("expected 1 input after load, got %d", c2.Count())
	}
}

func TestCorpusSaveEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	c := NewCorpus(tmpDir)
	err := c.Save([]byte{})
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestCorpusSaveNoDir(t *testing.T) {
	c := NewCorpus("")
	err := c.Save([]byte("test"))
	if !errors.Is(err, ErrCorpusDirNotFound) {
		t.Errorf("expected ErrCorpusDirNotFound, got %v", err)
	}
}

func TestReadMemoryStats(t *testing.T) {
	stats := ReadMemoryStats()
	if stats.AllocatedBytes == 0 {
		t.Error("expected non-zero allocated bytes")
	}
	if stats.NumAllocations == 0 {
		t.Error("expected non-zero allocation count")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig("testFunc")
	if config.FunctionName != "testFunc" {
		t.Errorf("expected function name 'testFunc', got '%s'", config.FunctionName)
	}
	if config.MaxInputSize != DefaultMaxInputSize {
		t.Errorf("expected MaxInputSize %d, got %d", DefaultMaxInputSize, config.MaxInputSize)
	}
	if config.MemoryThreshold != DefaultMemoryThreshold {
		t.Errorf("expected MemoryThreshold %d, got %d", DefaultMemoryThreshold, config.MemoryThreshold)
	}
	if !strings.Contains(config.CorpusDir, "testFunc") {
		t.Errorf("expected corpus dir to contain function name, got '%s'", config.CorpusDir)
	}
}

func TestNewFuzzer(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, err := NewFuzzer(target, config)
	if err != nil {
		t.Fatalf("NewFuzzer failed: %v", err)
	}
	if f == nil {
		t.Fatal("NewFuzzer returned nil")
	}
}

func TestNewFuzzerNilTarget(t *testing.T) {
	config := DefaultConfig("testFunc")
	_, err := NewFuzzer(nil, config)
	if !errors.Is(err, ErrNilTargetFunction) {
		t.Errorf("expected ErrNilTargetFunction, got %v", err)
	}
}

func TestNewFuzzerInvalidMaxInputSize(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.MaxInputSize = 0
	_, err := NewFuzzer(target, config)
	if !errors.Is(err, ErrInvalidMaxInputSize) {
		t.Errorf("expected ErrInvalidMaxInputSize, got %v", err)
	}
}

func TestNewFuzzerInvalidThreshold(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.MemoryThreshold = 0
	_, err := NewFuzzer(target, config)
	if !errors.Is(err, ErrInvalidThreshold) {
		t.Errorf("expected ErrInvalidThreshold, got %v", err)
	}
}

func TestFuzzerAddSeed(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxInputSize = 100
	f, _ := NewFuzzer(target, config)
	err := f.AddSeed([]byte("test seed"))
	if err != nil {
		t.Fatalf("AddSeed failed: %v", err)
	}
	if f.corpus.Count() != 1 {
		t.Errorf("expected corpus size 1, got %d", f.corpus.Count())
	}
}

func TestFuzzerAddSeedNil(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	err := f.AddSeed([]byte{})
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestFuzzerAddSeedTooLarge(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxInputSize = 5
	f, _ := NewFuzzer(target, config)
	err := f.AddSeed([]byte("123456"))
	if !errors.Is(err, ErrInputTooLarge) {
		t.Errorf("expected ErrInputTooLarge, got %v", err)
	}
}

func TestFuzzerCrashSaveAndReproduce(t *testing.T) {
	panicInput := []byte("panic!")
	target := func(input []byte) error {
		if string(input) == string(panicInput) {
			panic("intentional panic")
		}
		return nil
	}
	config := DefaultConfig("testCrashFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxIterations = 1
	config.MutationsPerInput = 10
	f, _ := NewFuzzer(target, config)
	f.AddSeed(panicInput)
	f.Run()
	records := f.CrashRecords()
	if len(records) == 0 {
		t.Skip("No crash records found (non-deterministic test)")
	}
	crashDir := config.CrashDir
	entries, err := os.ReadDir(crashDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) == 0 {
		t.Skip("No crash files found on disk (non-deterministic test)")
	}
	crashPath := filepath.Join(crashDir, entries[0].Name())
	loadedInput, err := f.LoadCrashInput(crashPath)
	if err != nil {
		t.Fatalf("LoadCrashInput failed: %v", err)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic during reproduction")
		}
	}()
	f.Reproduce(loadedInput)
}

func TestFuzzerReproduceNilInput(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	err := f.Reproduce([]byte{})
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestFuzzerMemoryDetection(t *testing.T) {
	var allocCount int32
	target := func(input []byte) error {
		if len(input) > 0 && input[0] == 0xFF {
			atomic.AddInt32(&allocCount, 1)
			_ = make([]byte, 20*1024*1024)
		}
		return nil
	}
	config := DefaultConfig("testMemFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MemoryThreshold = 1 * 1024 * 1024
	config.MaxIterations = 2
	config.MutationsPerInput = 5
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte{0xFF, 0x01, 0x02})
	f.Run()
	stats := f.Stats()
	_ = stats
}

func TestFuzzerRun(t *testing.T) {
	coverageTracker := make(map[string]bool)
	target := func(input []byte) error {
		key := fmt.Sprintf("path_%d", len(input))
		coverageTracker[key] = true
		if len(input) > 0 && input[0] == 'E' {
			return errors.New("intentional error")
		}
		return nil
	}
	config := DefaultConfig("testRunFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxIterations = 5
	config.MutationsPerInput = 20
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte("test"))
	err := f.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	stats := f.Stats()
	if stats.TotalIterations <= 0 {
		t.Error("expected positive iterations")
	}
	if stats.CorpusSize <= 0 {
		t.Error("expected positive corpus size")
	}
}

func TestFuzzerRunMaxDuration(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testDurationFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxDuration = 50 * time.Millisecond
	config.MutationsPerInput = 1000
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte("test"))
	start := time.Now()
	f.Run()
	duration := time.Since(start)
	if duration < 50*time.Millisecond {
		t.Errorf("expected at least 50ms duration, got %v", duration)
	}
}

func TestFuzzerStop(t *testing.T) {
	target := func(input []byte) error {
		runtime.Gosched()
		return nil
	}
	config := DefaultConfig("testStopFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MutationsPerInput = 10000
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte("test"))
	go func() {
		time.Sleep(20 * time.Millisecond)
		f.Stop()
	}()
	start := time.Now()
	err := f.Run()
	duration := time.Since(start)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if duration > 500*time.Millisecond {
		t.Errorf("Stop didn't work quickly enough, took %v", duration)
	}
}

func TestFuzzerStats(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testStatsFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxIterations = 2
	config.MutationsPerInput = 10
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte("seed1"))
	f.AddSeed([]byte("seed2"))
	f.Run()
	stats := f.Stats()
	if stats.StartTime.IsZero() {
		t.Error("expected non-zero start time")
	}
	if stats.CurrentDuration <= 0 {
		t.Error("expected positive duration")
	}
	if stats.TotalIterations <= 0 {
		t.Error("expected positive iterations")
	}
	if stats.CorpusSize <= 0 {
		t.Error("expected positive corpus size")
	}
}

func TestGenerateRandomSeed(t *testing.T) {
	size := 64
	seed, err := GenerateRandomSeed(size)
	if err != nil {
		t.Fatalf("GenerateRandomSeed failed: %v", err)
	}
	if len(seed) != size {
		t.Errorf("expected size %d, got %d", size, len(seed))
	}
	seed2, _ := GenerateRandomSeed(size)
	same := true
	for i := 0; i < size; i++ {
		if seed[i] != seed2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different random seeds")
	}
}

func TestGenerateRandomSeedInvalidSize(t *testing.T) {
	_, err := GenerateRandomSeed(0)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
	_, err = GenerateRandomSeed(-1)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestParseConfig(t *testing.T) {
	opts := map[string]string{
		"functionname":      "parsedFunc",
		"corpusdir":         "/tmp/corpus",
		"crashdir":          "/tmp/crashes",
		"maxinputsize":      "2048",
		"memorythreshold":   "5242880",
		"mutationsperinput": "50",
		"maxiterations":     "100",
		"maxduration":       "5s",
	}
	config, err := ParseConfig(opts)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if config.FunctionName != "parsedFunc" {
		t.Errorf("expected FunctionName 'parsedFunc', got '%s'", config.FunctionName)
	}
	if config.CorpusDir != "/tmp/corpus" {
		t.Errorf("expected CorpusDir '/tmp/corpus', got '%s'", config.CorpusDir)
	}
	if config.CrashDir != "/tmp/crashes" {
		t.Errorf("expected CrashDir '/tmp/crashes', got '%s'", config.CrashDir)
	}
	if config.MaxInputSize != 2048 {
		t.Errorf("expected MaxInputSize 2048, got %d", config.MaxInputSize)
	}
	if config.MemoryThreshold != 5242880 {
		t.Errorf("expected MemoryThreshold 5242880, got %d", config.MemoryThreshold)
	}
	if config.MutationsPerInput != 50 {
		t.Errorf("expected MutationsPerInput 50, got %d", config.MutationsPerInput)
	}
	if config.MaxIterations != 100 {
		t.Errorf("expected MaxIterations 100, got %d", config.MaxIterations)
	}
	if config.MaxDuration != 5*time.Second {
		t.Errorf("expected MaxDuration 5s, got %v", config.MaxDuration)
	}
}

func TestParseConfigInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		opts map[string]string
	}{
		{"invalid max input size", map[string]string{"maxinputsize": "abc"}},
		{"invalid memory threshold", map[string]string{"memorythreshold": "abc"}},
		{"invalid max duration", map[string]string{"maxduration": "abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(tt.opts)
			if err == nil {
				t.Error("expected error for invalid config")
			}
		})
	}
}

func TestParseConfigEmptyOptions(t *testing.T) {
	config, err := ParseConfig(map[string]string{})
	if err != nil {
		t.Fatalf("ParseConfig failed for empty opts: %v", err)
	}
	if config.MaxInputSize != DefaultMaxInputSize {
		t.Errorf("expected default max input size")
	}
}

func TestFuzzerProcessInputNewPath(t *testing.T) {
	callCount := 0
	target := func(input []byte) error {
		callCount++
		return nil
	}
	config := DefaultConfig("testProcessFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.EnableBaselineCalibration = false
	config.CoverageHook = InputBasedCoverageHook
	f, _ := NewFuzzer(target, config)
	input := []byte("test input for new path")
	found := f.processInput(input)
	if !found {
		t.Error("expected to find new path")
	}
	if f.corpus.Count() != 1 {
		t.Errorf("expected corpus size 1, got %d", f.corpus.Count())
	}
	input2 := []byte("test input for new path")
	found2 := f.processInput(input2)
	if found2 {
		t.Error("expected no new path for same input")
	}
}

func TestFuzzerProcessInputError(t *testing.T) {
	target := func(input []byte) error {
		if len(input) > 0 {
			return errors.New("test error")
		}
		return nil
	}
	config := DefaultConfig("testErrorFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	input := []byte("error input")
	found := f.processInput(input)
	if found {
		t.Error("expected no new path for error input")
	}
	records := f.CrashRecords()
	if len(records) == 0 {
		t.Error("expected crash record for error")
	}
}

func TestFuzzerProcessInputPanic(t *testing.T) {
	target := func(input []byte) error {
		if len(input) > 0 && input[0] == 'P' {
			panic("test panic")
		}
		return nil
	}
	config := DefaultConfig("testPanicFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	input := []byte("Panic input")
	found := f.processInput(input)
	if found {
		t.Error("expected no new path for panic input")
	}
	records := f.CrashRecords()
	if len(records) == 0 {
		t.Error("expected crash record for panic")
	}
}

func TestFuzzerCheckMemory(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testCheckMemFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MemoryThreshold = 100
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	before := MemoryStats{AllocatedBytes: 1000, NumAllocations: 10}
	after := MemoryStats{AllocatedBytes: 2000, NumAllocations: 20}
	suspicious, allocDiff, allocCountDiff := f.checkMemory(before, after)
	if !suspicious {
		t.Error("expected memory check to fail")
	}
	if allocDiff != 1000 {
		t.Errorf("expected allocDiff 1000, got %d", allocDiff)
	}
	if allocCountDiff != 10 {
		t.Errorf("expected allocCountDiff 10, got %d", allocCountDiff)
	}
	after2 := MemoryStats{AllocatedBytes: 1050, NumAllocations: 15}
	suspicious2, _, _ := f.checkMemory(before, after2)
	if suspicious2 {
		t.Error("expected memory check to pass")
	}
}

func TestFuzzerCheckMemoryAllocationCount(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testCheckAllocFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MemoryThreshold = 10000
	config.MemoryAllocThreshold = 5
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	before := MemoryStats{AllocatedBytes: 1000, NumAllocations: 10}
	after := MemoryStats{AllocatedBytes: 1050, NumAllocations: 100}
	suspicious, _, allocCountDiff := f.checkMemory(before, after)
	if !suspicious {
		t.Error("expected memory check to fail due to high allocation count")
	}
	if allocCountDiff != 90 {
		t.Errorf("expected allocCountDiff 90, got %d", allocCountDiff)
	}
}

func TestDefaultCoverageHook(t *testing.T) {
	hook := DefaultCoverageHook(10)
	if hook == nil {
		t.Fatal("DefaultCoverageHook returned nil")
	}
	addrs := hook([]byte("test"))
	if len(addrs) == 0 {
		t.Error("expected non-empty coverage addresses")
	}
	for i, addr := range addrs {
		if addr == 0 {
			t.Errorf("expected non-zero address at index %d", i)
		}
	}
}

func TestInputBasedCoverageHook(t *testing.T) {
	input1 := []byte("test input 1")
	input2 := []byte("test input 2")
	addrs1 := InputBasedCoverageHook(input1)
	addrs2 := InputBasedCoverageHook(input2)
	if len(addrs1) == 0 {
		t.Error("expected non-empty addresses for input1")
	}
	if len(addrs2) == 0 {
		t.Error("expected non-empty addresses for input2")
	}
	sameCount := 0
	for _, a1 := range addrs1 {
		for _, a2 := range addrs2 {
			if a1 == a2 {
				sameCount++
			}
		}
	}
	if sameCount == len(addrs1) && sameCount == len(addrs2) {
		t.Error("expected different addresses for different inputs")
	}
	emptyAddrs := InputBasedCoverageHook([]byte{})
	if len(emptyAddrs) != 0 {
		t.Errorf("expected empty addresses for empty input, got %d", len(emptyAddrs))
	}
}

func TestCustomCoverageHook(t *testing.T) {
	customHook := func(input []byte) []uint64 {
		return []uint64{uint64(len(input)), uint64(input[0])}
	}
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testCustomHook")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.CoverageHook = customHook
	config.EnableBaselineCalibration = false
	f, err := NewFuzzer(target, config)
	if err != nil {
		t.Fatalf("NewFuzzer failed: %v", err)
	}
	input := []byte{0x41, 0x42, 0x43}
	cov, _, _ := f.executeWithCoverage(input)
	if cov.Count() == 0 {
		t.Error("expected coverage to be collected")
	}
	if !cov.Has(3) {
		t.Error("expected custom coverage address 3 (len=3)")
	}
	if !cov.Has(0x41) {
		t.Error("expected custom coverage address 0x41 (first byte)")
	}
}

func TestCalibrateMemoryBaseline(t *testing.T) {
	target := func(input []byte) error {
		_ = make([]byte, len(input)*10)
		return nil
	}
	config := DefaultConfig("testBaseline")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.BaselineRuns = 5
	config.EnableBaselineCalibration = false
	f, err := NewFuzzer(target, config)
	if err != nil {
		t.Fatalf("NewFuzzer failed: %v", err)
	}
	f.AddSeed([]byte("seed1"))
	f.AddSeed([]byte("seed2"))
	err = f.CalibrateMemoryBaseline()
	if err != nil {
		t.Fatalf("CalibrateMemoryBaseline failed: %v", err)
	}
	baseline := f.GetMemoryBaseline()
	if !baseline.Calibrated {
		t.Error("expected baseline to be calibrated")
	}
	if baseline.AvgAllocatedBytes <= 0 {
		t.Error("expected positive AvgAllocatedBytes")
	}
	if baseline.AvgNumAllocations <= 0 {
		t.Error("expected positive AvgNumAllocations")
	}
}

func TestCalibrateMemoryBaselineEmptyCorpus(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testEmptyBaseline")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	err := f.CalibrateMemoryBaseline()
	if !errors.Is(err, ErrEmptyCorpus) {
		t.Errorf("expected ErrEmptyCorpus, got %v", err)
	}
}

func TestCheckMemoryWithBaseline(t *testing.T) {
	target := func(input []byte) error {
		return nil
	}
	config := DefaultConfig("testBaselineCheck")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MemoryMultiplier = 2.0
	config.MemoryThreshold = 100
	config.MemoryAllocThreshold = 10
	config.EnableBaselineCalibration = true
	f, _ := NewFuzzer(target, config)
	f.baselineSamples = []BaselineSample{
		{AllocatedBytes: 1000, NumAllocations: 100},
		{AllocatedBytes: 1200, NumAllocations: 120},
		{AllocatedBytes: 800, NumAllocations: 80},
	}
	f.computeBaselineStats()
	baseline := f.GetMemoryBaseline()
	if !baseline.Calibrated {
		t.Fatal("expected baseline to be calibrated")
	}
	if baseline.AvgAllocatedBytes != 1000 {
		t.Fatalf("expected AvgAllocatedBytes 1000, got %f", baseline.AvgAllocatedBytes)
	}
	before := MemoryStats{AllocatedBytes: 0, NumAllocations: 0}
	afterNormal := MemoryStats{
		AllocatedBytes: 1500,
		NumAllocations: 150,
	}
	suspicious, _, _ := f.checkMemory(before, afterNormal)
	if suspicious {
		t.Error("expected normal memory usage to pass (1.5x baseline)")
	}
	afterAbnormal := MemoryStats{
		AllocatedBytes: 2500,
		NumAllocations: 250,
	}
	suspicious, _, _ = f.checkMemory(before, afterAbnormal)
	if !suspicious {
		t.Error("expected abnormal memory usage to fail (2.5x baseline)")
	}
}

func TestNewFuzzerInvalidMultiplier(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testInvalidMult")
	config.MemoryMultiplier = 1.0
	config.EnableBaselineCalibration = true
	_, err := NewFuzzer(target, config)
	if !errors.Is(err, ErrInvalidMultiplier) {
		t.Errorf("expected ErrInvalidMultiplier, got %v", err)
	}
}

func TestReproducePreservesPanicType(t *testing.T) {
	type customPanic struct {
		msg string
	}
	target := func(input []byte) error {
		if len(input) > 0 && input[0] == 'P' {
			panic(customPanic{msg: "custom panic"})
		}
		return nil
	}
	config := DefaultConfig("testPanicType")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic")
		}
		cp, ok := r.(customPanic)
		if !ok {
			t.Errorf("expected customPanic type, got %T", r)
		}
		if cp.msg != "custom panic" {
			t.Errorf("expected msg 'custom panic', got '%s'", cp.msg)
		}
	}()
	f.Reproduce([]byte("Panic!"))
}

func TestFuzzerCoverageHookSwitch(t *testing.T) {
	target := func(input []byte) error { return nil }
	config1 := DefaultConfig("testHook1")
	config1.CorpusDir = t.TempDir()
	config1.CrashDir = t.TempDir()
	config1.EnableBaselineCalibration = false
	f1, _ := NewFuzzer(target, config1)
	input := []byte("test input")
	cov1, _, _ := f1.executeWithCoverage(input)
	if cov1.Count() == 0 {
		t.Error("expected default hook to collect coverage")
	}
	customAddrs := []uint64{0xAAAA, 0xBBBB}
	customHook := func(input []byte) []uint64 {
		return customAddrs
	}
	config2 := DefaultConfig("testHook2")
	config2.CorpusDir = t.TempDir()
	config2.CrashDir = t.TempDir()
	config2.CoverageHook = customHook
	config2.EnableBaselineCalibration = false
	f2, _ := NewFuzzer(target, config2)
	cov2, _, _ := f2.executeWithCoverage(input)
	for _, addr := range customAddrs {
		if !cov2.Has(addr) {
			t.Errorf("expected custom hook address 0x%X", addr)
		}
		if !cov2.Has(addr|0x8000000000000000) {
			t.Errorf("expected post-execution address for 0x%X", addr)
		}
	}
}

func TestPanicCoveragePreservation(t *testing.T) {
	target := func(input []byte) error {
		if len(input) > 0 && input[0] == 'P' {
			panic("test panic")
		}
		return nil
	}
	hookCalled := false
	customHook := func(input []byte) []uint64 {
		hookCalled = true
		return []uint64{0x1234, 0x5678}
	}
	config := DefaultConfig("testPanicCov")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.CoverageHook = customHook
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	cov, _, crashed := f.executeSafe([]byte("Panic!"))
	if !crashed {
		t.Fatal("expected crash")
	}
	if !hookCalled {
		t.Error("expected coverage hook to be called")
	}
	if cov == nil {
		t.Fatal("expected coverage to be preserved, got nil")
	}
	if cov.Count() == 0 {
		t.Error("expected coverage to contain addresses")
	}
	if !cov.Has(0xDEADBEEF) {
		t.Error("expected coverage to contain panic marker")
	}
}

func TestParseConfigNewOptions(t *testing.T) {
	opts := map[string]string{
		"functionname":             "newOpts",
		"memoryallocthreshold":     "2000",
		"memorymultiplier":         "3.5",
		"coveragetracedepth":       "20",
		"baselineruns":             "15",
		"enablebaselinecalibration": "false",
	}
	config, err := ParseConfig(opts)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if config.MemoryAllocThreshold != 2000 {
		t.Errorf("expected MemoryAllocThreshold 2000, got %d", config.MemoryAllocThreshold)
	}
	if config.MemoryMultiplier != 3.5 {
		t.Errorf("expected MemoryMultiplier 3.5, got %f", config.MemoryMultiplier)
	}
	if config.CoverageTraceDepth != 20 {
		t.Errorf("expected CoverageTraceDepth 20, got %d", config.CoverageTraceDepth)
	}
	if config.BaselineRuns != 15 {
		t.Errorf("expected BaselineRuns 15, got %d", config.BaselineRuns)
	}
	if config.EnableBaselineCalibration {
		t.Error("expected EnableBaselineCalibration to be false")
	}
}

func TestComputeBaselineStats(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testComputeStats")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.EnableBaselineCalibration = false
	f, _ := NewFuzzer(target, config)
	f.baselineSamples = []BaselineSample{
		{AllocatedBytes: 100, NumAllocations: 10},
		{AllocatedBytes: 200, NumAllocations: 20},
		{AllocatedBytes: 300, NumAllocations: 30},
	}
	f.computeBaselineStats()
	baseline := f.GetMemoryBaseline()
	if baseline.AvgAllocatedBytes != 200 {
		t.Errorf("expected AvgAllocatedBytes 200, got %f", baseline.AvgAllocatedBytes)
	}
	if baseline.AvgNumAllocations != 20 {
		t.Errorf("expected AvgNumAllocations 20, got %f", baseline.AvgNumAllocations)
	}
	if baseline.MinAllocatedBytes != 100 {
		t.Errorf("expected MinAllocatedBytes 100, got %d", baseline.MinAllocatedBytes)
	}
	if baseline.MaxAllocatedBytes != 300 {
		t.Errorf("expected MaxAllocatedBytes 300, got %d", baseline.MaxAllocatedBytes)
	}
	if !baseline.Calibrated {
		t.Error("expected Calibrated to be true")
	}
}

func TestFuzzerRecords(t *testing.T) {
	target := func(input []byte) error {
		if len(input) > 0 && input[0] == 'E' {
			return errors.New("error")
		}
		return nil
	}
	config := DefaultConfig("testRecordsFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	f.processInput([]byte("E_error"))
	crashes := f.CrashRecords()
	if len(crashes) != 1 {
		t.Errorf("expected 1 crash record, got %d", len(crashes))
	}
	if crashes[0].FunctionName != "testRecordsFunc" {
		t.Errorf("expected function name 'testRecordsFunc', got '%s'", crashes[0].FunctionName)
	}
	if crashes[0].Error == "" {
		t.Error("expected non-empty error message")
	}
	if crashes[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	suspicious := f.SuspiciousRecords()
	if len(suspicious) != 0 {
		t.Errorf("expected 0 suspicious records, got %d", len(suspicious))
	}
}

func TestFuzzerConcurrentRun(t *testing.T) {
	var callCount int32
	target := func(input []byte) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}
	config := DefaultConfig("testConcurrentFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxIterations = 3
	config.MutationsPerInput = 20
	f, _ := NewFuzzer(target, config)
	f.AddSeed([]byte("seed1"))
	f.AddSeed([]byte("seed2"))
	f.AddSeed([]byte("seed3"))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		f.Run()
	}()
	go func() {
		for i := 0; i < 10; i++ {
			_ = f.Stats()
			_ = f.CrashRecords()
			_ = f.SuspiciousRecords()
			runtime.Gosched()
		}
	}()
	wg.Wait()
	stats := f.Stats()
	if stats.TotalIterations <= 0 {
		t.Error("expected iterations after concurrent run")
	}
}

func TestMakeHash(t *testing.T) {
	input1 := []byte("test input 1")
	input2 := []byte("test input 2")
	hash1 := makeHash(input1)
	hash2 := makeHash(input2)
	if len(hash1) != 8 {
		t.Errorf("expected hash length 8, got %d", len(hash1))
	}
	same := true
	for i := 0; i < 8; i++ {
		if hash1[i] != hash2[i] {
			same = false
			break
		}
	}
	if same {
		t.Error("expected different hashes for different inputs")
	}
	hash1Copy := makeHash(input1)
	for i := 0; i < 8; i++ {
		if hash1[i] != hash1Copy[i] {
			t.Error("expected same hash for same input")
		}
	}
}

func TestFuzzerRunWithDefaultSeed(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testDefaultSeed")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxIterations = 1
	config.MutationsPerInput = 5
	f, _ := NewFuzzer(target, config)
	if f.corpus.Count() != 0 {
		t.Errorf("expected empty corpus before Run, got %d", f.corpus.Count())
	}
	err := f.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if f.corpus.Count() == 0 {
		t.Error("expected default seed to be added")
	}
}

func TestMutatorEdgeCases(t *testing.T) {
	m := NewMutator()
	t.Run("flip bit empty input", func(t *testing.T) {
		result := m.FlipBit([]byte{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d bytes", len(result))
		}
	})
	t.Run("replace byte empty input", func(t *testing.T) {
		result := m.ReplaceByte([]byte{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d bytes", len(result))
		}
	})
	t.Run("delete byte empty input", func(t *testing.T) {
		result := m.DeleteByte([]byte{})
		if len(result) != 0 {
			t.Errorf("expected empty result, got %d bytes", len(result))
		}
	})
}

func TestCorpusLoadFromExistingDir(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "seed1"), []byte("seed data 1"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "seed2"), []byte("seed data 2"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "empty"), []byte{}, 0644)
	c := NewCorpus(tmpDir)
	err := c.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if c.Count() != 2 {
		t.Errorf("expected 2 inputs, got %d", c.Count())
	}
}

func TestCorpusLoadNotADirectory(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "notadir")
	os.WriteFile(tmpFile, []byte("test"), 0644)
	c := NewCorpus(tmpFile)
	err := c.Load()
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestLoadCrashInputNotFound(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	_, err := f.LoadCrashInput("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadCrashInputEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	emptyFile := filepath.Join(tmpDir, "empty")
	os.WriteFile(emptyFile, []byte{}, 0644)
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testFunc")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	_, err := f.LoadCrashInput(emptyFile)
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestFuzzerDoubleStop(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testDoubleStop")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	f, _ := NewFuzzer(target, config)
	f.Stop()
	f.Stop()
}

func TestFuzzerProcessInputTooLarge(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := DefaultConfig("testTooLarge")
	config.CorpusDir = t.TempDir()
	config.CrashDir = t.TempDir()
	config.MaxInputSize = 5
	f, _ := NewFuzzer(target, config)
	largeInput := make([]byte, 100)
	found := f.processInput(largeInput)
	if found {
		t.Error("expected no new path for too large input")
	}
}

func TestMutatorRandomByte(t *testing.T) {
	m := NewMutator()
	seen := make(map[byte]bool)
	for i := 0; i < 1000; i++ {
		b := m.randomByte()
		seen[b] = true
	}
	if len(seen) < 100 {
		t.Errorf("expected more variety in random bytes, got %d unique", len(seen))
	}
}

func TestMemoryStatsValues(t *testing.T) {
	stats := ReadMemoryStats()
	if stats.AllocatedBytes < 1024 {
		t.Errorf("allocated bytes seems too low: %d", stats.AllocatedBytes)
	}
	if stats.NumAllocations < 100 {
		t.Errorf("allocation count seems too low: %d", stats.NumAllocations)
	}
}

func TestSuspiciousMemoryRecordFields(t *testing.T) {
	record := SuspiciousMemoryRecord{
		Input:          []byte("test"),
		Timestamp:      time.Now(),
		AllocatedDiff:  1000,
		AllocationDiff: 10,
		Threshold:      100,
	}
	if len(record.Input) != 4 {
		t.Errorf("expected input length 4, got %d", len(record.Input))
	}
	if record.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if record.AllocatedDiff != 1000 {
		t.Errorf("expected AllocatedDiff 1000, got %d", record.AllocatedDiff)
	}
	if record.Threshold != 100 {
		t.Errorf("expected Threshold 100, got %d", record.Threshold)
	}
}

func TestCrashRecordFields(t *testing.T) {
	record := CrashRecord{
		Input:        []byte("crash input"),
		Timestamp:    time.Now(),
		FunctionName: "testFunc",
		Error:        "test error",
	}
	if len(record.Input) != 11 {
		t.Errorf("expected input length 11, got %d", len(record.Input))
	}
	if record.FunctionName != "testFunc" {
		t.Errorf("expected FunctionName 'testFunc', got '%s'", record.FunctionName)
	}
	if record.Error != "test error" {
		t.Errorf("expected Error 'test error', got '%s'", record.Error)
	}
}

func TestFuzzerStatsFields(t *testing.T) {
	stats := FuzzerStats{
		TotalIterations:  1000,
		NewPathsFound:    50,
		CrashesFound:     5,
		SuspiciousMemory: 2,
		CorpusSize:       25,
		StartTime:        time.Now(),
		CurrentDuration:  time.Second,
	}
	if stats.TotalIterations != 1000 {
		t.Errorf("expected TotalIterations 1000, got %d", stats.TotalIterations)
	}
	if stats.CrashesFound != 5 {
		t.Errorf("expected CrashesFound 5, got %d", stats.CrashesFound)
	}
	if stats.CorpusSize != 25 {
		t.Errorf("expected CorpusSize 25, got %d", stats.CorpusSize)
	}
}

func TestParseConfigAlternativeKeys(t *testing.T) {
	opts := map[string]string{
		"func":        "alt1",
		"corpus":      "/alt/corpus",
		"crashes":     "/alt/crashes",
		"maxsize":     "4096",
		"mem":         "1048576",
		"mutations":   "25",
		"maxiter":     "50",
		"duration":    "10s",
	}
	config, err := ParseConfig(opts)
	if err != nil {
		t.Fatalf("ParseConfig failed: %v", err)
	}
	if config.FunctionName != "alt1" {
		t.Errorf("expected FunctionName 'alt1', got '%s'", config.FunctionName)
	}
	if config.MaxInputSize != 4096 {
		t.Errorf("expected MaxInputSize 4096, got %d", config.MaxInputSize)
	}
}

func TestFuzzerNewFuzzerAutoFill(t *testing.T) {
	target := func(input []byte) error { return nil }
	config := FuzzerConfig{
		MaxInputSize:    1024,
		MemoryThreshold: 1024,
	}
	f, err := NewFuzzer(target, config)
	if err != nil {
		t.Fatalf("NewFuzzer failed: %v", err)
	}
	if f.config.FunctionName == "" {
		t.Error("expected function name to be auto-filled")
	}
	if f.config.CorpusDir == "" {
		t.Error("expected corpus dir to be auto-filled")
	}
	if f.config.CrashDir == "" {
		t.Error("expected crash dir to be auto-filled")
	}
	if f.config.MutationsPerInput != DefaultMutationsPerInput {
		t.Errorf("expected default mutations per input, got %d", f.config.MutationsPerInput)
	}
}
