package perfsampler

import (
	"crypto/rand"
	"encoding/hex"
	"hash/fnv"
	"math"
	"sync"
	"time"
)

type AlwaysSample struct{}

func NewAlwaysSample() *AlwaysSample {
	return &AlwaysSample{}
}

func (s *AlwaysSample) ShouldSample(requestID string) bool {
	return true
}

func (s *AlwaysSample) Rate() float64 {
	return 1.0
}

type NeverSample struct{}

func NewNeverSample() *NeverSample {
	return &NeverSample{}
}

func (s *NeverSample) ShouldSample(requestID string) bool {
	return false
}

func (s *NeverSample) Rate() float64 {
	return 0.0
}

type ProbabilitySampler struct {
	rate      float64
	threshold uint64
}

func NewProbabilitySampler(rate float64) (*ProbabilitySampler, error) {
	if rate < 0 || rate > 1 {
		return nil, ErrInvalidSampleRate
	}
	threshold := uint64(rate * float64(math.MaxUint64))
	return &ProbabilitySampler{
		rate:      rate,
		threshold: threshold,
	}, nil
}

func (s *ProbabilitySampler) ShouldSample(requestID string) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0 {
		return false
	}

	var val uint64
	if len(requestID) >= 16 {
		b, err := hex.DecodeString(requestID[:16])
		if err == nil {
			for i := 0; i < 8; i++ {
				val = (val << 8) | uint64(b[i])
			}
			return val < s.threshold
		}
	}

	val = hashRequestID(requestID)
	return val < s.threshold
}

func hashRequestID(requestID string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(requestID))
	return h.Sum64()
}

func (s *ProbabilitySampler) Rate() float64 {
	return s.rate
}

type RequestProfiler struct {
	mu           sync.Mutex
	requestID    string
	sampler      Sampler
	sampled      bool
	sampleRate   float64
	startTime    time.Time
	endTime      time.Time
	started      bool
	stopped      bool

	cpuRoot      *CPUStackNode
	cpuStack     []*CPUStackNode

	memoryStats  map[string]*MemoryFuncStat

	timingStack  []*TimingSegment
	timingResult []*TimingSegment
}

func NewRequestProfiler(requestID string, sampler Sampler) (*RequestProfiler, error) {
	if requestID == "" {
		return nil, ErrEmptyRequestID
	}
	if sampler == nil {
		return nil, ErrNilSampler
	}

	sampled := sampler.ShouldSample(requestID)

	return &RequestProfiler{
		requestID:   requestID,
		sampler:     sampler,
		sampled:     sampled,
		sampleRate:  sampler.Rate(),
		memoryStats: make(map[string]*MemoryFuncStat),
	}, nil
}

func GenerateRequestID() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (p *RequestProfiler) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.started {
		return ErrProfilerAlreadyStarted
	}

	p.started = true
	p.startTime = time.Now()

	if !p.sampled {
		return nil
	}

	p.cpuRoot = &CPUStackNode{
		FunctionName: "root",
	}
	p.cpuStack = []*CPUStackNode{p.cpuRoot}

	return nil
}

func (p *RequestProfiler) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return nil
	}

	p.stopped = true
	p.endTime = time.Now()

	if !p.sampled {
		return nil
	}

	for len(p.timingStack) > 0 {
		seg := p.timingStack[len(p.timingStack)-1]
		p.timingStack = p.timingStack[:len(p.timingStack)-1]
		seg.Duration = time.Since(seg.StartTime)
		p.timingResult = append(p.timingResult, seg)
	}

	p.calculateCPUTotals(p.cpuRoot)

	return nil
}

func (p *RequestProfiler) IsSampled() bool {
	return p.sampled
}

func (p *RequestProfiler) RequestID() string {
	return p.requestID
}

func (p *RequestProfiler) EnterCPUFunction(functionName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if functionName == "" {
		return ErrEmptyLabel
	}

	parent := p.cpuStack[len(p.cpuStack)-1]

	var node *CPUStackNode
	for _, child := range parent.Children {
		if child.FunctionName == functionName {
			node = child
			break
		}
	}

	if node == nil {
		node = &CPUStackNode{
			FunctionName: functionName,
		}
		parent.Children = append(parent.Children, node)
	}

	node.SampleCount++
	p.cpuStack = append(p.cpuStack, node)

	return nil
}

func (p *RequestProfiler) ExitCPUFunction() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if len(p.cpuStack) <= 1 {
		return ErrInvalidCPUProfile
	}

	node := p.cpuStack[len(p.cpuStack)-1]
	node.SelfTimeNs += 1000
	p.cpuStack = p.cpuStack[:len(p.cpuStack)-1]

	return nil
}

func (p *RequestProfiler) RecordCPUSample(stack []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if len(stack) == 0 {
		return ErrInvalidCPUProfile
	}

	current := p.cpuRoot
	for _, funcName := range stack {
		found := false
		for _, child := range current.Children {
			if child.FunctionName == funcName {
				current = child
				found = true
				break
			}
		}
		if !found {
			node := &CPUStackNode{
				FunctionName: funcName,
			}
			current.Children = append(current.Children, node)
			current = node
		}
		current.SampleCount++
	}

	return nil
}

func (p *RequestProfiler) RecordAlloc(functionName string, bytes int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if functionName == "" {
		return ErrEmptyLabel
	}

	stat, exists := p.memoryStats[functionName]
	if !exists {
		stat = &MemoryFuncStat{
			FunctionName: functionName,
		}
		p.memoryStats[functionName] = stat
	}

	stat.AllocCount++
	stat.AllocBytes += bytes
	stat.InUseCount++
	stat.InUseBytes += bytes

	return nil
}

func (p *RequestProfiler) RecordFree(functionName string, bytes int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if functionName == "" {
		return ErrEmptyLabel
	}

	stat, exists := p.memoryStats[functionName]
	if !exists {
		stat = &MemoryFuncStat{
			FunctionName: functionName,
		}
		p.memoryStats[functionName] = stat
	}

	stat.FreeCount++
	stat.FreeBytes += bytes
	if stat.InUseCount > 0 {
		stat.InUseCount--
	}
	if stat.InUseBytes >= bytes {
		stat.InUseBytes -= bytes
	} else {
		stat.InUseBytes = 0
	}

	return nil
}

func (p *RequestProfiler) StartSegment(label string, metadata ...map[string]string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if p.stopped {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}
	if label == "" {
		return ErrEmptyLabel
	}

	var md map[string]string
	if len(metadata) > 0 && metadata[0] != nil {
		md = make(map[string]string)
		for k, v := range metadata[0] {
			md[k] = v
		}
	}

	seg := &TimingSegment{
		Label:     label,
		StartTime: time.Now(),
		Metadata:  md,
	}

	p.timingStack = append(p.timingStack, seg)

	return nil
}

func (p *RequestProfiler) EndSegment() (*TimingSegment, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil, ErrProfilerNotStarted
	}
	if p.stopped {
		return nil, ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil, nil
	}
	if len(p.timingStack) == 0 {
		return nil, ErrSegmentNotStarted
	}

	seg := p.timingStack[len(p.timingStack)-1]
	p.timingStack = p.timingStack[:len(p.timingStack)-1]
	seg.Duration = time.Since(seg.StartTime)
	p.timingResult = append(p.timingResult, seg)

	return seg, nil
}

func (p *RequestProfiler) SetSegmentMetadata(label string, key, value string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return ErrProfilerNotStarted
	}
	if !p.sampled {
		return nil
	}

	for _, seg := range p.timingResult {
		if seg.Label == label {
			if seg.Metadata == nil {
				seg.Metadata = make(map[string]string)
			}
			seg.Metadata[key] = value
			return nil
		}
	}

	for _, seg := range p.timingStack {
		if seg.Label == label {
			if seg.Metadata == nil {
				seg.Metadata = make(map[string]string)
			}
			seg.Metadata[key] = value
			return nil
		}
	}

	return ErrSegmentNotStarted
}

func (p *RequestProfiler) Export() (*ProfileResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.started {
		return nil, ErrProfilerNotStarted
	}
	if !p.stopped {
		return nil, ErrProfilerNotStopped
	}

	result := &ProfileResult{
		RequestID:  p.requestID,
		Sampled:    p.sampled,
		SampleRate: p.sampleRate,
		StartTime:  p.startTime,
		EndTime:    p.endTime,
		Duration:   p.endTime.Sub(p.startTime),
	}

	if !p.sampled {
		return result, nil
	}

	result.CPUProfile = p.cpuRoot

	memStats := make([]*MemoryFuncStat, 0, len(p.memoryStats))
	for _, stat := range p.memoryStats {
		memStats = append(memStats, stat)
	}
	result.MemoryStats = memStats

	result.Timing = make([]*TimingSegment, len(p.timingResult))
	copy(result.Timing, p.timingResult)

	return result, nil
}

func (p *RequestProfiler) ToFlameGraph() ([]*FlameGraphEntry, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.sampled {
		return nil, ErrNotSampled
	}
	if p.cpuRoot == nil {
		return nil, ErrInvalidCPUProfile
	}

	var entries []*FlameGraphEntry
	stack := make([]string, 0)
	p.traverseFlameGraph(p.cpuRoot, stack, &entries)

	return entries, nil
}

func (p *RequestProfiler) traverseFlameGraph(node *CPUStackNode, stack []string, entries *[]*FlameGraphEntry) {
	if node == nil {
		return
	}

	currentStack := make([]string, len(stack)+1)
	copy(currentStack, stack)
	currentStack[len(stack)] = node.FunctionName

	if node.SampleCount > 0 {
		entryStack := make([]string, len(currentStack))
		copy(entryStack, currentStack)
		*entries = append(*entries, &FlameGraphEntry{
			Stack: entryStack,
			Value: node.SampleCount,
		})
	}

	for _, child := range node.Children {
		p.traverseFlameGraph(child, currentStack, entries)
	}
}

func (p *RequestProfiler) calculateCPUTotals(node *CPUStackNode) int64 {
	if node == nil {
		return 0
	}

	childrenTotal := int64(0)
	for _, child := range node.Children {
		childrenTotal += p.calculateCPUTotals(child)
	}

	node.TotalTimeNs = int64(node.SampleCount) * 1000
	node.SelfTimeNs = node.TotalTimeNs - childrenTotal
	if node.SelfTimeNs < 0 {
		node.SelfTimeNs = 0
	}

	return node.TotalTimeNs
}
