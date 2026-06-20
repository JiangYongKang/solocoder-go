package fuzzdriver

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultCorpusDir     = "corpus"
	DefaultCrashDir      = "crashes"
	DefaultMaxInputSize  = 1 << 16
	DefaultMemoryThreshold  = 10 * 1024 * 1024
	DefaultMutationsPerInput = 100
)

var (
	ErrNilTargetFunction   = errors.New("fuzzdriver: target function cannot be nil")
	ErrEmptyCorpus         = errors.New("fuzzdriver: corpus is empty")
	ErrInvalidInput        = errors.New("fuzzdriver: invalid input")
	ErrCorpusDirNotFound   = errors.New("fuzzdriver: corpus directory not found")
	ErrCrashDirNotFound    = errors.New("fuzzdriver: crash directory not found")
	ErrInvalidMaxInputSize = errors.New("fuzzdriver: max input size must be positive")
	ErrInvalidThreshold    = errors.New("fuzzdriver: memory threshold must be positive")
	ErrNilInput            = errors.New("fuzzdriver: input cannot be nil")
	ErrInputTooLarge       = errors.New("fuzzdriver: input exceeds max size")
	ErrCorpusLoadFailed    = errors.New("fuzzdriver: failed to load corpus")
	ErrCrashSaveFailed     = errors.New("fuzzdriver: failed to save crash input")
	ErrCorpusSaveFailed    = errors.New("fuzzdriver: failed to save corpus input")
)

type TargetFunc func(input []byte) error

type Coverage struct {
	mu      sync.RWMutex
	covered map[uint64]bool
}

func NewCoverage() *Coverage {
	return &Coverage{
		covered: make(map[uint64]bool),
	}
}

func (c *Coverage) Add(addr uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.covered[addr] = true
}

func (c *Coverage) Has(addr uint64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.covered[addr]
}

func (c *Coverage) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.covered)
}

func (c *Coverage) Merge(other *Coverage) (newPaths int) {
	other.mu.RLock()
	defer other.mu.RUnlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	for addr := range other.covered {
		if !c.covered[addr] {
			c.covered[addr] = true
			newPaths++
		}
	}
	return newPaths
}

func (c *Coverage) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.covered = make(map[uint64]bool)
}

func (c *Coverage) Snapshot() []uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	addrs := make([]uint64, 0, len(c.covered))
	for addr := range c.covered {
		addrs = append(addrs, addr)
	}
	sort.Slice(addrs, func(i, j int) bool { return addrs[i] < addrs[j] })
	return addrs
}

type Mutator struct {
	mu sync.Mutex
}

type lockedRand struct {
	mu sync.Mutex
}

func (l *lockedRand) Read(p []byte) (n int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return cryptorand.Read(p)
}

var globalRand = &lockedRand{}

func NewMutator() *Mutator {
	return &Mutator{}
}

func (m *Mutator) randomInt(n int) int {
	if n <= 0 {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b := make([]byte, 8)
	globalRand.Read(b)
	val := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56
	return int(val % uint64(n))
}

func (m *Mutator) randomByte() byte {
	b := make([]byte, 1)
	globalRand.Read(b)
	return b[0]
}

func (m *Mutator) FlipBit(input []byte) []byte {
	if len(input) == 0 {
		return input
	}
	result := make([]byte, len(input))
	copy(result, input)
	byteIdx := m.randomInt(len(result))
	bitIdx := m.randomInt(8)
	result[byteIdx] ^= 1 << bitIdx
	return result
}

func (m *Mutator) InsertByte(input []byte, maxSize int) []byte {
	if len(input) >= maxSize {
		return input
	}
	pos := m.randomInt(len(input) + 1)
	b := m.randomByte()
	result := make([]byte, len(input)+1)
	copy(result[:pos], input[:pos])
	result[pos] = b
	copy(result[pos+1:], input[pos:])
	return result
}

func (m *Mutator) DeleteByte(input []byte) []byte {
	if len(input) <= 1 {
		return input
	}
	pos := m.randomInt(len(input))
	result := make([]byte, len(input)-1)
	copy(result[:pos], input[:pos])
	copy(result[pos:], input[pos+1:])
	return result
}

func (m *Mutator) ReplaceByte(input []byte) []byte {
	if len(input) == 0 {
		return input
	}
	result := make([]byte, len(input))
	copy(result, input)
	pos := m.randomInt(len(result))
	result[pos] = m.randomByte()
	return result
}

func (m *Mutator) Mutate(input []byte, maxSize int) []byte {
	if len(input) == 0 {
		return []byte{m.randomByte()}
	}
	mutationType := m.randomInt(4)
	switch mutationType {
	case 0:
		return m.FlipBit(input)
	case 1:
		return m.InsertByte(input, maxSize)
	case 2:
		return m.DeleteByte(input)
	case 3:
		return m.ReplaceByte(input)
	default:
		return m.FlipBit(input)
	}
}

func (m *Mutator) MutateN(input []byte, n, maxSize int) []byte {
	result := make([]byte, len(input))
	copy(result, input)
	for i := 0; i < n; i++ {
		result = m.Mutate(result, maxSize)
	}
	return result
}

type Corpus struct {
	mu         sync.RWMutex
	inputs     [][]byte
	currentIdx int
	dir        string
}

func NewCorpus(dir string) *Corpus {
	return &Corpus{
		inputs: make([][]byte, 0),
		dir:    dir,
	}
}

func (c *Corpus) Add(input []byte) {
	if len(input) == 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputs = append(c.inputs, input)
}

func (c *Corpus) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.inputs)
}

func (c *Corpus) Next() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inputs) == 0 {
		return nil
	}
	input := c.inputs[c.currentIdx]
	c.currentIdx = (c.currentIdx + 1) % len(c.inputs)
	result := make([]byte, len(input))
	copy(result, input)
	return result
}

func (c *Corpus) GetAll() [][]byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([][]byte, len(c.inputs))
	for i, input := range c.inputs {
		result[i] = make([]byte, len(input))
		copy(result[i], input)
	}
	return result
}

func (c *Corpus) Load() error {
	if c.dir == "" {
		return ErrCorpusDirNotFound
	}
	info, err := os.Stat(c.dir)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(c.dir, 0755); err != nil {
			return fmt.Errorf("%w: %v", ErrCorpusLoadFailed, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCorpusLoadFailed, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: not a directory", ErrCorpusLoadFailed)
	}
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCorpusLoadFailed, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(c.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if len(data) > 0 {
			c.Add(data)
		}
	}
	return nil
}

func (c *Corpus) Save(input []byte) error {
	if len(input) == 0 {
		return ErrNilInput
	}
	if c.dir == "" {
		return ErrCorpusDirNotFound
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrCorpusSaveFailed, err)
	}
	filename := fmt.Sprintf("id_%s_%d", hex.EncodeToString(makeHash(input)), time.Now().UnixNano())
	path := filepath.Join(c.dir, filename)
	if err := os.WriteFile(path, input, 0644); err != nil {
		return fmt.Errorf("%w: %v", ErrCorpusSaveFailed, err)
	}
	return nil
}

func makeHash(input []byte) []byte {
	h := make([]byte, 8)
	for i, b := range input {
		h[i%8] ^= b
	}
	return h
}

type MemoryStats struct {
	AllocatedBytes uint64
	NumAllocations uint64
}

func ReadMemoryStats() MemoryStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return MemoryStats{
		AllocatedBytes: stats.Alloc,
		NumAllocations: stats.Mallocs,
	}
}

type SuspiciousMemoryRecord struct {
	Input          []byte
	Timestamp      time.Time
	AllocatedDiff  uint64
	AllocationDiff uint64
	Threshold      uint64
}

type CrashRecord struct {
	Input       []byte
	Timestamp   time.Time
	FunctionName string
	Error       string
}

type FuzzerConfig struct {
	FunctionName       string
	CorpusDir          string
	CrashDir           string
	MaxInputSize       int
	MemoryThreshold    uint64
	MutationsPerInput  int
	MaxIterations      int
	MaxDuration        time.Duration
}

func DefaultConfig(functionName string) FuzzerConfig {
	return FuzzerConfig{
		FunctionName:      functionName,
		CorpusDir:         filepath.Join(DefaultCorpusDir, functionName),
		CrashDir:          filepath.Join(DefaultCrashDir, functionName),
		MaxInputSize:      DefaultMaxInputSize,
		MemoryThreshold:   DefaultMemoryThreshold,
		MutationsPerInput: DefaultMutationsPerInput,
		MaxIterations:     0,
		MaxDuration:       0,
	}
}

type FuzzerStats struct {
	TotalIterations    int64
	NewPathsFound      int64
	CrashesFound       int64
	SuspiciousMemory   int64
	CorpusSize         int
	StartTime          time.Time
	CurrentDuration    time.Duration
}

type Fuzzer struct {
	config           FuzzerConfig
	target           TargetFunc
	corpus           *Corpus
	mutator          *Mutator
	globalCoverage   *Coverage
	suspiciousRecords []SuspiciousMemoryRecord
	crashRecords     []CrashRecord
	stats            FuzzerStats
	statsMu          sync.Mutex
	stopChan         chan struct{}
	stopped          bool
	mu               sync.Mutex
}

func NewFuzzer(target TargetFunc, config FuzzerConfig) (*Fuzzer, error) {
	if target == nil {
		return nil, ErrNilTargetFunction
	}
	if config.MaxInputSize <= 0 {
		return nil, ErrInvalidMaxInputSize
	}
	if config.MemoryThreshold <= 0 {
		return nil, ErrInvalidThreshold
	}
	if config.FunctionName == "" {
		config.FunctionName = "unknown"
	}
	if config.CorpusDir == "" {
		config.CorpusDir = filepath.Join(DefaultCorpusDir, config.FunctionName)
	}
	if config.CrashDir == "" {
		config.CrashDir = filepath.Join(DefaultCrashDir, config.FunctionName)
	}
	if config.MutationsPerInput <= 0 {
		config.MutationsPerInput = DefaultMutationsPerInput
	}
	corpus := NewCorpus(config.CorpusDir)
	if err := corpus.Load(); err != nil {
		return nil, err
	}
	return &Fuzzer{
		config:         config,
		target:         target,
		corpus:         corpus,
		mutator:        NewMutator(),
		globalCoverage: NewCoverage(),
		stopChan:       make(chan struct{}),
		stats: FuzzerStats{
			StartTime: time.Now(),
		},
	}, nil
}

func (f *Fuzzer) AddSeed(input []byte) error {
	if len(input) == 0 {
		return ErrNilInput
	}
	if len(input) > f.config.MaxInputSize {
		return ErrInputTooLarge
	}
	f.corpus.Add(input)
	return nil
}

func (f *Fuzzer) LoadCrashInput(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCrashSaveFailed, err)
	}
	if len(data) == 0 {
		return nil, ErrNilInput
	}
	return data, nil
}

func (f *Fuzzer) Reproduce(input []byte) error {
	if len(input) == 0 {
		return ErrNilInput
	}
	defer func() {
		if r := recover(); r != nil {
			panic(fmt.Errorf("panic during reproduction: %v", r))
		}
	}()
	return f.target(input)
}

func (f *Fuzzer) saveCrash(input []byte, err error) error {
	if f.config.CrashDir == "" {
		return ErrCrashDirNotFound
	}
	if err := os.MkdirAll(f.config.CrashDir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrCrashSaveFailed, err)
	}
	timestamp := time.Now().Format("20060102T150405Z0700")
	safeFuncName := strings.ReplaceAll(f.config.FunctionName, string(os.PathSeparator), "_")
	filename := fmt.Sprintf("crash_%s_%s_%s", safeFuncName, timestamp, hex.EncodeToString(makeHash(input))[:8])
	path := filepath.Join(f.config.CrashDir, filename)
	if err := os.WriteFile(path, input, 0644); err != nil {
		return fmt.Errorf("%w: %v", ErrCrashSaveFailed, err)
	}
	f.statsMu.Lock()
	f.crashRecords = append(f.crashRecords, CrashRecord{
		Input:        input,
		Timestamp:    time.Now(),
		FunctionName: f.config.FunctionName,
		Error:        err.Error(),
	})
	f.stats.CrashesFound++
	f.statsMu.Unlock()
	return nil
}

func (f *Fuzzer) executeWithCoverage(input []byte) (*Coverage, error, bool) {
	coverage := NewCoverage()
	defer func() {
		if r := recover(); r != nil {
			for i := 0; i < len(input); i++ {
				coverage.Add(uint64(input[i])*2654435761 + 2)
			}
			panic(r)
		}
	}()
	for i := 0; i < len(input); i++ {
		coverage.Add(uint64(input[i]) * 2654435761)
	}
	for i := 0; i < len(input)-1; i++ {
		coverage.Add(uint64(input[i])*2654435761 + uint64(input[i+1]))
	}
	for i := 0; i < len(input) && i < 8; i++ {
		coverage.Add(uint64(i)<<56 | uint64(input[i]))
	}
	err := f.target(input)
	for i := 0; i < len(input); i++ {
		coverage.Add(uint64(input[i])*2654435761 + 1)
	}
	if err != nil {
		coverage.Add(0xDEADBEEF)
	}
	return coverage, err, false
}

func (f *Fuzzer) executeSafe(input []byte) (cov *Coverage, execErr error, crashed bool) {
	defer func() {
		if r := recover(); r != nil {
			crashed = true
			execErr = fmt.Errorf("panic: %v", r)
			if cov == nil {
				cov = NewCoverage()
			}
		}
	}()
	cov, execErr, crashed = f.executeWithCoverage(input)
	return cov, execErr, crashed
}

func (f *Fuzzer) checkMemory(before, after MemoryStats) bool {
	allocDiff := uint64(0)
	if after.AllocatedBytes > before.AllocatedBytes {
		allocDiff = after.AllocatedBytes - before.AllocatedBytes
	}
	return allocDiff > f.config.MemoryThreshold
}

func (f *Fuzzer) recordSuspiciousMemory(input []byte, before, after MemoryStats) {
	allocDiff := uint64(0)
	if after.AllocatedBytes > before.AllocatedBytes {
		allocDiff = after.AllocatedBytes - before.AllocatedBytes
	}
	allocCountDiff := uint64(0)
	if after.NumAllocations > before.NumAllocations {
		allocCountDiff = after.NumAllocations - before.NumAllocations
	}
	record := SuspiciousMemoryRecord{
		Input:          input,
		Timestamp:      time.Now(),
		AllocatedDiff:  allocDiff,
		AllocationDiff: allocCountDiff,
		Threshold:      f.config.MemoryThreshold,
	}
	f.statsMu.Lock()
	f.suspiciousRecords = append(f.suspiciousRecords, record)
	f.stats.SuspiciousMemory++
	f.statsMu.Unlock()
}

func (f *Fuzzer) processInput(input []byte) (foundNewPath bool) {
	if len(input) > f.config.MaxInputSize {
		return false
	}
	beforeMem := ReadMemoryStats()
	cov, err, crashed := f.executeSafe(input)
	afterMem := ReadMemoryStats()
	if crashed || err != nil {
		f.saveCrash(input, err)
		return false
	}
	if f.checkMemory(beforeMem, afterMem) {
		f.recordSuspiciousMemory(input, beforeMem, afterMem)
	}
	newPaths := f.globalCoverage.Merge(cov)
	if newPaths > 0 {
		f.corpus.Add(input)
		f.corpus.Save(input)
		f.statsMu.Lock()
		f.stats.NewPathsFound += int64(newPaths)
		f.statsMu.Unlock()
		return true
	}
	return false
}

func (f *Fuzzer) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.stopped {
		f.stopped = true
		close(f.stopChan)
	}
}

func (f *Fuzzer) isStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

func (f *Fuzzer) Stats() FuzzerStats {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	stats := f.stats
	stats.CorpusSize = f.corpus.Count()
	stats.CurrentDuration = time.Since(stats.StartTime)
	return stats
}

func (f *Fuzzer) CrashRecords() []CrashRecord {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	result := make([]CrashRecord, len(f.crashRecords))
	copy(result, f.crashRecords)
	return result
}

func (f *Fuzzer) SuspiciousRecords() []SuspiciousMemoryRecord {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	result := make([]SuspiciousMemoryRecord, len(f.suspiciousRecords))
	copy(result, f.suspiciousRecords)
	return result
}

func (f *Fuzzer) Run() error {
	if f.corpus.Count() == 0 {
		defaultSeed := []byte(f.config.FunctionName)
		f.corpus.Add(defaultSeed)
	}
	iterCount := int64(0)
	startTime := time.Now()
	for {
		if f.isStopped() {
			break
		}
		if f.config.MaxIterations > 0 && iterCount >= int64(f.config.MaxIterations) {
			break
		}
		if f.config.MaxDuration > 0 && time.Since(startTime) >= f.config.MaxDuration {
			break
		}
		seed := f.corpus.Next()
		if seed == nil {
			return ErrEmptyCorpus
		}
		for i := 0; i < f.config.MutationsPerInput; i++ {
			if f.isStopped() {
				break
			}
			mutated := f.mutator.Mutate(seed, f.config.MaxInputSize)
			f.processInput(mutated)
			f.statsMu.Lock()
			f.stats.TotalIterations++
			f.statsMu.Unlock()
		}
		iterCount++
		runtime.Gosched()
	}
	return nil
}

func GenerateRandomSeed(size int) ([]byte, error) {
	if size <= 0 {
		return nil, ErrInvalidInput
	}
	seed := make([]byte, size)
	_, err := globalRand.Read(seed)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return seed, nil
}

func ParseConfig(opts map[string]string) (FuzzerConfig, error) {
	config := DefaultConfig("")
	for k, v := range opts {
		switch strings.ToLower(k) {
		case "functionname", "func", "name":
			config.FunctionName = v
		case "corpusdir", "corpus":
			config.CorpusDir = v
		case "crashdir", "crashes":
			config.CrashDir = v
		case "maxinputsize", "maxsize":
			size, err := strconv.Atoi(v)
			if err != nil {
				return config, fmt.Errorf("%w: invalid max input size: %v", ErrInvalidMaxInputSize, err)
			}
			config.MaxInputSize = size
		case "memorythreshold", "memthreshold", "mem":
			threshold, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return config, fmt.Errorf("%w: invalid memory threshold: %v", ErrInvalidThreshold, err)
			}
			config.MemoryThreshold = threshold
		case "mutationsperinput", "mutations":
			muts, err := strconv.Atoi(v)
			if err != nil {
				return config, fmt.Errorf("invalid mutations per input: %v", err)
			}
			if muts > 0 {
				config.MutationsPerInput = muts
			}
		case "maxiterations", "maxiter":
			iters, err := strconv.Atoi(v)
			if err != nil {
				return config, fmt.Errorf("invalid max iterations: %v", err)
			}
			config.MaxIterations = iters
		case "maxduration", "duration", "time":
			dur, err := time.ParseDuration(v)
			if err != nil {
				return config, fmt.Errorf("invalid max duration: %v", err)
			}
			config.MaxDuration = dur
		}
	}
	return config, nil
}
