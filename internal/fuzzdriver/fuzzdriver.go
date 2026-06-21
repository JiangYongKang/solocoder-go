package fuzzdriver

import (
	"bytes"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
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
	DefaultCorpusDir         = "corpus"
	DefaultCrashDir          = "crashes"
	DefaultMaxInputSize      = 1 << 16
	DefaultMemoryThreshold   = 10 * 1024 * 1024
	DefaultMemoryAllocThreshold = 1000
	DefaultMemoryMultiplier  = 5
	DefaultMutationsPerInput = 100
	DefaultCoverageTraceDepth = 10
	DefaultBaselineRuns      = 10
)

var (
	ErrNilTargetFunction     = errors.New("fuzzdriver: target function cannot be nil")
	ErrEmptyCorpus           = errors.New("fuzzdriver: corpus is empty")
	ErrInvalidInput          = errors.New("fuzzdriver: invalid input")
	ErrCorpusDirNotFound     = errors.New("fuzzdriver: corpus directory not found")
	ErrCrashDirNotFound      = errors.New("fuzzdriver: crash directory not found")
	ErrInvalidMaxInputSize   = errors.New("fuzzdriver: max input size must be positive")
	ErrInvalidThreshold      = errors.New("fuzzdriver: memory threshold must be positive")
	ErrNilInput              = errors.New("fuzzdriver: input cannot be nil")
	ErrInputTooLarge         = errors.New("fuzzdriver: input exceeds max size")
	ErrCorpusLoadFailed      = errors.New("fuzzdriver: failed to load corpus")
	ErrCrashSaveFailed       = errors.New("fuzzdriver: failed to save crash input")
	ErrCorpusSaveFailed      = errors.New("fuzzdriver: failed to save corpus input")
	ErrBaselineCalibrationFailed = errors.New("fuzzdriver: baseline calibration failed")
	ErrInvalidMultiplier     = errors.New("fuzzdriver: memory multiplier must be greater than 1")
)

type TargetFunc func(input []byte) error

type CoverageHook func(input []byte) []uint64

type MemoryBaseline struct {
	AvgAllocatedBytes   float64
	AvgNumAllocations   float64
	MaxAllocatedBytes   uint64
	MaxNumAllocations   uint64
	MinAllocatedBytes   uint64
	MinNumAllocations   uint64
	StdDevAllocated     float64
	StdDevAllocations   float64
	Calibrated          bool
}

type BaselineSample struct {
	AllocatedBytes uint64
	NumAllocations uint64
}

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

var (
	coverageMap   = make(map[uint64]*Coverage)
	coverageMapMu sync.RWMutex
)

func getGoroutineID() uint64 {
	b := make([]byte, 64)
	b = b[:runtime.Stack(b, false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}

func SetCurrentCoverage(cov *Coverage) {
	gid := getGoroutineID()
	coverageMapMu.Lock()
	defer coverageMapMu.Unlock()
	coverageMap[gid] = cov
}

func GetCurrentCoverage() *Coverage {
	gid := getGoroutineID()
	coverageMapMu.RLock()
	defer coverageMapMu.RUnlock()
	return coverageMap[gid]
}

func ClearCurrentCoverage() {
	gid := getGoroutineID()
	coverageMapMu.Lock()
	defer coverageMapMu.Unlock()
	delete(coverageMap, gid)
}

func Cover(addr uint64) {
	if cov := GetCurrentCoverage(); cov != nil {
		cov.Add(addr)
	}
}

func DefaultCoverageHook(depth int) CoverageHook {
	return func(input []byte) []uint64 {
		pcs := make([]uintptr, depth)
		n := runtime.Callers(2, pcs)
		if n == 0 {
			return nil
		}
		result := make([]uint64, n)
		for i := 0; i < n; i++ {
			result[i] = uint64(pcs[i])
		}
		return result
	}
}

func InputBasedCoverageHook(input []byte) []uint64 {
	if len(input) == 0 {
		return nil
	}
	result := make([]uint64, 0, len(input)*2)
	for i := 0; i < len(input); i++ {
		result = append(result, uint64(input[i])*2654435761)
	}
	for i := 0; i < len(input)-1; i++ {
		result = append(result, uint64(input[i])*2654435761+uint64(input[i+1]))
	}
	return result
}

type InstrumentedTarget func(input []byte, cover func(uint64)) error

func WrapInstrumentedTarget(target InstrumentedTarget) TargetFunc {
	return func(input []byte) error {
		return target(input, Cover)
	}
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
	Input              []byte
	Timestamp          time.Time
	AllocatedDiff      uint64
	AllocationDiff     uint64
	ThresholdBytes     uint64
	ThresholdAllocs    uint64
	TriggeredByBytes   bool
	TriggeredByAllocs  bool
}

type CrashRecord struct {
	Input       []byte
	Timestamp   time.Time
	FunctionName string
	Error       string
}

type FuzzerConfig struct {
	FunctionName         string
	CorpusDir            string
	CrashDir             string
	MaxInputSize         int
	MemoryThreshold      uint64
	MemoryAllocThreshold uint64
	MemoryMultiplier     float64
	MutationsPerInput    int
	MaxIterations        int
	MaxDuration          time.Duration
	CoverageHook         CoverageHook
	CoverageTraceDepth   int
	BaselineRuns         int
	EnableBaselineCalibration bool
}

func DefaultConfig(functionName string) FuzzerConfig {
	return FuzzerConfig{
		FunctionName:              functionName,
		CorpusDir:                 filepath.Join(DefaultCorpusDir, functionName),
		CrashDir:                  filepath.Join(DefaultCrashDir, functionName),
		MaxInputSize:              DefaultMaxInputSize,
		MemoryThreshold:           DefaultMemoryThreshold,
		MemoryAllocThreshold:      DefaultMemoryAllocThreshold,
		MemoryMultiplier:          DefaultMemoryMultiplier,
		MutationsPerInput:         DefaultMutationsPerInput,
		MaxIterations:             0,
		MaxDuration:               0,
		CoverageHook:              nil,
		CoverageTraceDepth:        DefaultCoverageTraceDepth,
		BaselineRuns:              DefaultBaselineRuns,
		EnableBaselineCalibration: true,
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
	config              FuzzerConfig
	target              TargetFunc
	corpus              *Corpus
	mutator             *Mutator
	globalCoverage      *Coverage
	memoryBaseline      MemoryBaseline
	baselineSamples     []BaselineSample
	coverageHook        CoverageHook
	suspiciousRecords   []SuspiciousMemoryRecord
	crashRecords        []CrashRecord
	stats               FuzzerStats
	statsMu             sync.Mutex
	stopChan            chan struct{}
	stopped             bool
	mu                  sync.Mutex
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
	if config.MemoryMultiplier <= 1 && config.EnableBaselineCalibration {
		return nil, ErrInvalidMultiplier
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
	if config.CoverageTraceDepth <= 0 {
		config.CoverageTraceDepth = DefaultCoverageTraceDepth
	}
	if config.BaselineRuns <= 0 {
		config.BaselineRuns = DefaultBaselineRuns
	}
	if config.MemoryAllocThreshold <= 0 {
		config.MemoryAllocThreshold = DefaultMemoryAllocThreshold
	}

	var coverageHook CoverageHook
	if config.CoverageHook != nil {
		coverageHook = config.CoverageHook
	} else {
		coverageHook = DefaultCoverageHook(config.CoverageTraceDepth)
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
		coverageHook:   coverageHook,
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

func (f *Fuzzer) executeWithCoverage(input []byte) (coverage *Coverage, execErr error, crashed bool) {
	coverage = NewCoverage()
	SetCurrentCoverage(coverage)

	preAddrs := f.coverageHook(input)
	for _, addr := range preAddrs {
		coverage.Add(addr)
	}

	execErr = f.target(input)

	postAddrs := f.coverageHook(input)
	for _, addr := range postAddrs {
		coverage.Add(addr | 0x8000000000000000)
	}

	if execErr != nil {
		coverage.Add(0xDEADBEEF)
	}

	return coverage, execErr, false
}

func (f *Fuzzer) executeSafe(input []byte) (cov *Coverage, execErr error, crashed bool) {
	defer func() {
		if r := recover(); r != nil {
			crashed = true
			execErr = fmt.Errorf("panic: %v", r)
			if cov == nil {
				cov = GetCurrentCoverage()
			}
			if cov == nil {
				cov = NewCoverage()
			}
			postAddrs := f.coverageHook(input)
			for _, addr := range postAddrs {
				cov.Add(addr | 0x8000000000000000)
			}
			cov.Add(0xDEADBEEF)
		}
		ClearCurrentCoverage()
	}()
	cov, execErr, crashed = f.executeWithCoverage(input)
	return cov, execErr, crashed
}

func (f *Fuzzer) CalibrateMemoryBaseline() error {
	if f.corpus.Count() == 0 {
		return ErrEmptyCorpus
	}

	seeds := f.corpus.GetAll()
	numRuns := f.config.BaselineRuns
	f.baselineSamples = make([]BaselineSample, 0, numRuns*len(seeds))

	for _, seed := range seeds {
		for i := 0; i < numRuns; i++ {
			before := ReadMemoryStats()
			err := func() (panicErr error) {
				defer func() {
					if r := recover(); r != nil {
						panicErr = fmt.Errorf("panic: %v", r)
					}
				}()
				return f.target(seed)
			}()
			after := ReadMemoryStats()

			if err != nil {
				continue
			}

			var allocDiff uint64
			var allocCountDiff uint64
			if after.AllocatedBytes > before.AllocatedBytes {
				allocDiff = after.AllocatedBytes - before.AllocatedBytes
			}
			if after.NumAllocations > before.NumAllocations {
				allocCountDiff = after.NumAllocations - before.NumAllocations
			}

			f.baselineSamples = append(f.baselineSamples, BaselineSample{
				AllocatedBytes: allocDiff,
				NumAllocations: allocCountDiff,
			})
		}
	}

	if len(f.baselineSamples) == 0 {
		return ErrBaselineCalibrationFailed
	}

	f.computeBaselineStats()
	f.memoryBaseline.Calibrated = true
	return nil
}

func (f *Fuzzer) computeBaselineStats() {
	n := len(f.baselineSamples)
	if n == 0 {
		return
	}

	var sumBytes, sumAllocs float64
	minBytes := ^uint64(0)
	minAllocs := ^uint64(0)
	var maxBytes, maxAllocs uint64

	for _, s := range f.baselineSamples {
		sumBytes += float64(s.AllocatedBytes)
		sumAllocs += float64(s.NumAllocations)
		if s.AllocatedBytes < minBytes {
			minBytes = s.AllocatedBytes
		}
		if s.AllocatedBytes > maxBytes {
			maxBytes = s.AllocatedBytes
		}
		if s.NumAllocations < minAllocs {
			minAllocs = s.NumAllocations
		}
		if s.NumAllocations > maxAllocs {
			maxAllocs = s.NumAllocations
		}
	}

	avgBytes := sumBytes / float64(n)
	avgAllocs := sumAllocs / float64(n)

	var varianceBytes, varianceAllocs float64
	for _, s := range f.baselineSamples {
		diff := float64(s.AllocatedBytes) - avgBytes
		varianceBytes += diff * diff
		diff = float64(s.NumAllocations) - avgAllocs
		varianceAllocs += diff * diff
	}

	stdDevBytes := 0.0
	stdDevAllocs := 0.0
	if n > 1 {
		varianceBytes /= float64(n - 1)
		varianceAllocs /= float64(n - 1)
		stdDevBytes = math.Sqrt(varianceBytes)
		stdDevAllocs = math.Sqrt(varianceAllocs)
	}

	f.memoryBaseline = MemoryBaseline{
		AvgAllocatedBytes: avgBytes,
		AvgNumAllocations: avgAllocs,
		MaxAllocatedBytes: maxBytes,
		MaxNumAllocations: maxAllocs,
		MinAllocatedBytes: minBytes,
		MinNumAllocations: minAllocs,
		StdDevAllocated:   stdDevBytes,
		StdDevAllocations: stdDevAllocs,
		Calibrated:        true,
	}
}

func (f *Fuzzer) GetMemoryBaseline() MemoryBaseline {
	return f.memoryBaseline
}

func (f *Fuzzer) checkMemory(before, after MemoryStats) (suspicious bool, allocDiff uint64, allocCountDiff uint64, thresholdBytes uint64, thresholdAllocs uint64, triggeredByBytes bool, triggeredByAllocs bool) {
	allocDiff = 0
	if after.AllocatedBytes > before.AllocatedBytes {
		allocDiff = after.AllocatedBytes - before.AllocatedBytes
	}
	allocCountDiff = 0
	if after.NumAllocations > before.NumAllocations {
		allocCountDiff = after.NumAllocations - before.NumAllocations
	}

	if f.memoryBaseline.Calibrated && f.config.EnableBaselineCalibration {
		thresholdBytes = uint64(f.memoryBaseline.AvgAllocatedBytes * f.config.MemoryMultiplier)
		if thresholdBytes < f.config.MemoryThreshold {
			thresholdBytes = f.config.MemoryThreshold
		}
		triggeredByBytes = allocDiff > thresholdBytes

		thresholdAllocs = uint64(f.memoryBaseline.AvgNumAllocations * f.config.MemoryMultiplier)
		if thresholdAllocs < f.config.MemoryAllocThreshold {
			thresholdAllocs = f.config.MemoryAllocThreshold
		}
		triggeredByAllocs = allocCountDiff > thresholdAllocs
	} else {
		thresholdBytes = f.config.MemoryThreshold
		thresholdAllocs = f.config.MemoryAllocThreshold
		triggeredByBytes = allocDiff > thresholdBytes
		triggeredByAllocs = allocCountDiff > thresholdAllocs
	}

	suspicious = triggeredByBytes || triggeredByAllocs
	return suspicious, allocDiff, allocCountDiff, thresholdBytes, thresholdAllocs, triggeredByBytes, triggeredByAllocs
}

func (f *Fuzzer) recordSuspiciousMemory(input []byte, before, after MemoryStats) {
	suspicious, allocDiff, allocCountDiff, thresholdBytes, thresholdAllocs, triggeredByBytes, triggeredByAllocs := f.checkMemory(before, after)
	if !suspicious {
		return
	}

	record := SuspiciousMemoryRecord{
		Input:             input,
		Timestamp:         time.Now(),
		AllocatedDiff:     allocDiff,
		AllocationDiff:    allocCountDiff,
		ThresholdBytes:    thresholdBytes,
		ThresholdAllocs:   thresholdAllocs,
		TriggeredByBytes:  triggeredByBytes,
		TriggeredByAllocs: triggeredByAllocs,
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
	suspicious, _, _, _, _, _, _ := f.checkMemory(beforeMem, afterMem)
	if suspicious {
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

	if f.config.EnableBaselineCalibration && !f.memoryBaseline.Calibrated {
		if err := f.CalibrateMemoryBaseline(); err != nil {
			if err != ErrBaselineCalibrationFailed {
				return err
			}
		}
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
		case "memoryallocthreshold", "allocthreshold":
			threshold, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return config, fmt.Errorf("%w: invalid memory alloc threshold: %v", ErrInvalidThreshold, err)
			}
			config.MemoryAllocThreshold = threshold
		case "memorymultiplier", "memmultiplier":
			multiplier, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return config, fmt.Errorf("%w: invalid memory multiplier: %v", ErrInvalidMultiplier, err)
			}
			config.MemoryMultiplier = multiplier
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
		case "coveragetracedepth", "tracedepth":
			depth, err := strconv.Atoi(v)
			if err != nil {
				return config, fmt.Errorf("invalid coverage trace depth: %v", err)
			}
			if depth > 0 {
				config.CoverageTraceDepth = depth
			}
		case "baselineruns", "baseline":
			runs, err := strconv.Atoi(v)
			if err != nil {
				return config, fmt.Errorf("invalid baseline runs: %v", err)
			}
			if runs > 0 {
				config.BaselineRuns = runs
			}
		case "enablebaselinecalibration", "usebaseline":
			enabled, err := strconv.ParseBool(v)
			if err != nil {
				return config, fmt.Errorf("invalid enable baseline calibration: %v", err)
			}
			config.EnableBaselineCalibration = enabled
		}
	}
	return config, nil
}
