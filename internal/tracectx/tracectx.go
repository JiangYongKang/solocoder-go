package tracectx

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
)

const (
	TraceIDLength = 16
	SpanIDLength  = 8

	W3CTraceParentHeader = "traceparent"
	W3CTraceStateHeader  = "tracestate"

	TraceParentVersion = "00"

	SampledFlag    byte = 0x01
	NotSampledFlag byte = 0x00
)

var (
	ErrInvalidTraceID      = errors.New("tracectx: invalid trace id")
	ErrInvalidSpanID       = errors.New("tracectx: invalid span id")
	ErrInvalidTraceParent  = errors.New("tracectx: invalid traceparent header")
	ErrSpanNotFound        = errors.New("tracectx: span not found")
	ErrNilSpan             = errors.New("tracectx: span cannot be nil")
	ErrDuplicateSpanID     = errors.New("tracectx: duplicate span id")
	ErrInvalidSamplingRate = errors.New("tracectx: invalid sampling rate")
	ErrEmptySpanID         = errors.New("tracectx: span id cannot be empty")
	ErrEmptyTraceID        = errors.New("tracectx: trace id cannot be empty")
)

type Sampler interface {
	ShouldSample(traceID string) bool
}

type AlwaysSample struct{}

func NewAlwaysSample() *AlwaysSample {
	return &AlwaysSample{}
}

func (s *AlwaysSample) ShouldSample(traceID string) bool {
	return true
}

type NeverSample struct{}

func NewNeverSample() *NeverSample {
	return &NeverSample{}
}

func (s *NeverSample) ShouldSample(traceID string) bool {
	return false
}

type ProbabilitySampler struct {
	rate    float64
	threshold uint64
}

func NewProbabilitySampler(rate float64) (*ProbabilitySampler, error) {
	if rate < 0 || rate > 1 {
		return nil, ErrInvalidSamplingRate
	}
	threshold := uint64(rate * float64(math.MaxUint64))
	return &ProbabilitySampler{
		rate:      rate,
		threshold: threshold,
	}, nil
}

func (s *ProbabilitySampler) ShouldSample(traceID string) bool {
	if s.rate >= 1.0 {
		return true
	}
	if s.rate <= 0 {
		return false
	}
	if len(traceID) < 16 {
		return false
	}
	b, err := hex.DecodeString(traceID[:16])
	if err != nil {
		return false
	}
	var val uint64
	for i := 0; i < 8; i++ {
		val = (val << 8) | uint64(b[i])
	}
	return val < s.threshold
}

func (s *ProbabilitySampler) Rate() float64 {
	return s.rate
}

type Span struct {
	TraceID       string
	SpanID        string
	ParentSpanID  string
	Name          string
	Sampled       bool
	StartTime     int64
	EndTime       int64
	Attributes    map[string]string
}

func NewSpan(traceID, spanID, parentSpanID, name string, sampled bool) *Span {
	return &Span{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: parentSpanID,
		Name:         name,
		Sampled:      sampled,
		Attributes:   make(map[string]string),
	}
}

func (s *Span) SetAttribute(key, value string) {
	if s.Attributes == nil {
		s.Attributes = make(map[string]string)
	}
	s.Attributes[key] = value
}

func (s *Span) GetAttribute(key string) (string, bool) {
	if s.Attributes == nil {
		return "", false
	}
	v, ok := s.Attributes[key]
	return v, ok
}

func (s *Span) IsRoot() bool {
	return s.ParentSpanID == ""
}

type SpanTree struct {
	mu    sync.RWMutex
	spans map[string]*Span
	roots []*Span
}

func NewSpanTree() *SpanTree {
	return &SpanTree{
		spans: make(map[string]*Span),
		roots: make([]*Span, 0),
	}
}

func (t *SpanTree) AddSpan(span *Span) error {
	if span == nil {
		return ErrNilSpan
	}
	if span.SpanID == "" {
		return ErrEmptySpanID
	}
	if span.TraceID == "" {
		return ErrEmptyTraceID
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.spans[span.SpanID]; exists {
		return ErrDuplicateSpanID
	}

	t.spans[span.SpanID] = span

	if span.ParentSpanID == "" {
		t.roots = append(t.roots, span)
	}

	return nil
}

func (t *SpanTree) GetSpan(spanID string) (*Span, error) {
	if spanID == "" {
		return nil, ErrEmptySpanID
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	span, exists := t.spans[spanID]
	if !exists {
		return nil, ErrSpanNotFound
	}
	return span, nil
}

func (t *SpanTree) GetChildren(parentSpanID string) ([]*Span, error) {
	if parentSpanID == "" {
		return nil, ErrEmptySpanID
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, exists := t.spans[parentSpanID]; !exists {
		return nil, ErrSpanNotFound
	}

	children := make([]*Span, 0)
	for _, span := range t.spans {
		if span.ParentSpanID == parentSpanID {
			children = append(children, span)
		}
	}
	return children, nil
}

func (t *SpanTree) GetRoots() []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	roots := make([]*Span, len(t.roots))
	copy(roots, t.roots)
	return roots
}

func (t *SpanTree) GetSubtree(rootSpanID string) ([]*Span, error) {
	if rootSpanID == "" {
		return nil, ErrEmptySpanID
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	if _, exists := t.spans[rootSpanID]; !exists {
		return nil, ErrSpanNotFound
	}

	var result []*Span
	t.traverseSubtree(rootSpanID, &result)
	return result, nil
}

func (t *SpanTree) traverseSubtree(spanID string, result *[]*Span) {
	span := t.spans[spanID]
	*result = append(*result, span)

	for _, s := range t.spans {
		if s.ParentSpanID == spanID {
			t.traverseSubtree(s.SpanID, result)
		}
	}
}

func (t *SpanTree) AllSpans() []*Span {
	t.mu.RLock()
	defer t.mu.RUnlock()

	spans := make([]*Span, 0, len(t.spans))
	for _, span := range t.spans {
		spans = append(spans, span)
	}
	return spans
}

func (t *SpanTree) SpanCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.spans)
}

type TraceContext struct {
	TraceID      string
	SpanID       string
	ParentSpanID string
	Sampled      bool
}

func NewRootContext(name string, sampler Sampler) (*TraceContext, *Span, error) {
	traceID, err := GenerateTraceID()
	if err != nil {
		return nil, nil, err
	}
	spanID, err := GenerateSpanID()
	if err != nil {
		return nil, nil, err
	}

	sampled := sampler.ShouldSample(traceID)

	ctx := &TraceContext{
		TraceID:      traceID,
		SpanID:       spanID,
		ParentSpanID: "",
		Sampled:      sampled,
	}

	span := NewSpan(traceID, spanID, "", name, sampled)

	return ctx, span, nil
}

func NewChildContext(parent *TraceContext, name string) (*TraceContext, *Span, error) {
	if parent == nil {
		return nil, nil, errors.New("tracectx: parent context cannot be nil")
	}

	spanID, err := GenerateSpanID()
	if err != nil {
		return nil, nil, err
	}

	ctx := &TraceContext{
		TraceID:      parent.TraceID,
		SpanID:       spanID,
		ParentSpanID: parent.SpanID,
		Sampled:      parent.Sampled,
	}

	span := NewSpan(parent.TraceID, spanID, parent.SpanID, name, parent.Sampled)

	return ctx, span, nil
}

func GenerateTraceID() (string, error) {
	b := make([]byte, TraceIDLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func GenerateSpanID() (string, error) {
	b := make([]byte, SpanIDLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func IsValidTraceID(traceID string) bool {
	if len(traceID) != TraceIDLength*2 {
		return false
	}
	_, err := hex.DecodeString(traceID)
	return err == nil
}

func IsValidSpanID(spanID string) bool {
	if len(spanID) != SpanIDLength*2 {
		return false
	}
	_, err := hex.DecodeString(spanID)
	return err == nil
}

func InjectTraceContext(ctx *TraceContext) map[string]string {
	if ctx == nil {
		return make(map[string]string)
	}

	flags := NotSampledFlag
	if ctx.Sampled {
		flags = SampledFlag
	}

	traceParent := fmt.Sprintf("%s-%s-%s-%02x",
		TraceParentVersion,
		ctx.TraceID,
		ctx.SpanID,
		flags,
	)

	return map[string]string{
		W3CTraceParentHeader: traceParent,
	}
}

func ExtractTraceContext(headers map[string]string) (*TraceContext, error) {
	traceParent := ""
	for k, v := range headers {
		if strings.ToLower(k) == W3CTraceParentHeader {
			traceParent = v
			break
		}
	}

	if traceParent == "" {
		return nil, ErrInvalidTraceParent
	}

	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		return nil, ErrInvalidTraceParent
	}

	version := parts[0]
	traceID := parts[1]
	parentID := parts[2]
	traceFlags := parts[3]

	if version != TraceParentVersion {
		return nil, ErrInvalidTraceParent
	}

	if !IsValidTraceID(traceID) {
		return nil, ErrInvalidTraceID
	}

	if !IsValidSpanID(parentID) {
		return nil, ErrInvalidSpanID
	}

	if len(traceFlags) != 2 {
		return nil, ErrInvalidTraceParent
	}

	flagsBytes, err := hex.DecodeString(traceFlags)
	if err != nil {
		return nil, ErrInvalidTraceParent
	}
	if len(flagsBytes) != 1 {
		return nil, ErrInvalidTraceParent
	}

	sampled := (flagsBytes[0] & SampledFlag) != 0

	return &TraceContext{
		TraceID:      traceID,
		SpanID:       parentID,
		ParentSpanID: "",
		Sampled:      sampled,
	}, nil
}

func (tc *TraceContext) String() string {
	flags := NotSampledFlag
	if tc.Sampled {
		flags = SampledFlag
	}
	return fmt.Sprintf("%s-%s-%s-%02x",
		TraceParentVersion,
		tc.TraceID,
		tc.SpanID,
		flags,
	)
}
