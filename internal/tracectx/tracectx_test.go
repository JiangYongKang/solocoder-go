package tracectx

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestGenerateTraceID(t *testing.T) {
	id, err := GenerateTraceID()
	if err != nil {
		t.Fatalf("GenerateTraceID failed: %v", err)
	}
	if len(id) != TraceIDLength*2 {
		t.Errorf("expected trace id length %d, got %d", TraceIDLength*2, len(id))
	}
	if !IsValidTraceID(id) {
		t.Error("generated trace id should be valid")
	}
}

func TestGenerateTraceIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := GenerateTraceID()
		if err != nil {
			t.Fatalf("GenerateTraceID failed: %v", err)
		}
		if ids[id] {
			t.Error("duplicate trace id generated")
		}
		ids[id] = true
	}
}

func TestGenerateSpanID(t *testing.T) {
	id, err := GenerateSpanID()
	if err != nil {
		t.Fatalf("GenerateSpanID failed: %v", err)
	}
	if len(id) != SpanIDLength*2 {
		t.Errorf("expected span id length %d, got %d", SpanIDLength*2, len(id))
	}
	if !IsValidSpanID(id) {
		t.Error("generated span id should be valid")
	}
}

func TestGenerateSpanIDUniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id, err := GenerateSpanID()
		if err != nil {
			t.Fatalf("GenerateSpanID failed: %v", err)
		}
		if ids[id] {
			t.Error("duplicate span id generated")
		}
		ids[id] = true
	}
}

func TestIsValidTraceID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "0123456789abcdef0123456789abcdef", true},
		{"too short", "0123456789abcdef", false},
		{"too long", "0123456789abcdef0123456789abcdef00", false},
		{"empty", "", false},
		{"invalid chars", "0123456789abcdef0123456789abcdeg", false},
		{"uppercase", "0123456789ABCDEF0123456789ABCDEF", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidTraceID(tt.input); got != tt.want {
				t.Errorf("IsValidTraceID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidSpanID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid", "0123456789abcdef", true},
		{"too short", "01234567", false},
		{"too long", "0123456789abcdef00", false},
		{"empty", "", false},
		{"invalid chars", "0123456789abcdeg", false},
		{"uppercase", "0123456789ABCDEF", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidSpanID(tt.input); got != tt.want {
				t.Errorf("IsValidSpanID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAlwaysSample(t *testing.T) {
	sampler := NewAlwaysSample()
	traceID := "0123456789abcdef0123456789abcdef"
	if !sampler.ShouldSample(traceID) {
		t.Error("AlwaysSample should always return true")
	}
}

func TestNeverSample(t *testing.T) {
	sampler := NewNeverSample()
	traceID := "0123456789abcdef0123456789abcdef"
	if sampler.ShouldSample(traceID) {
		t.Error("NeverSample should always return false")
	}
}

func TestNewProbabilitySampler(t *testing.T) {
	tests := []struct {
		name      string
		rate      float64
		wantError bool
	}{
		{"zero", 0.0, false},
		{"half", 0.5, false},
		{"one", 1.0, false},
		{"negative", -0.1, true},
		{"greater than one", 1.1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler, err := NewProbabilitySampler(tt.rate)
			if tt.wantError {
				if !errors.Is(err, ErrInvalidSamplingRate) {
					t.Errorf("expected ErrInvalidSamplingRate, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if sampler.Rate() != tt.rate {
					t.Errorf("expected rate %v, got %v", tt.rate, sampler.Rate())
				}
			}
		})
	}
}

func TestProbabilitySamplerBoundary(t *testing.T) {
	sampler, err := NewProbabilitySampler(0.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	traceID := "0123456789abcdef0123456789abcdef"
	if sampler.ShouldSample(traceID) {
		t.Error("rate 0 should never sample")
	}

	sampler, err = NewProbabilitySampler(1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sampler.ShouldSample(traceID) {
		t.Error("rate 1 should always sample")
	}
}

func TestProbabilitySamplerDistribution(t *testing.T) {
	sampler, err := NewProbabilitySampler(0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sampled := 0
	total := 10000
	for i := 0; i < total; i++ {
		traceID, _ := GenerateTraceID()
		if sampler.ShouldSample(traceID) {
			sampled++
		}
	}

	ratio := float64(sampled) / float64(total)
	if ratio < 0.4 || ratio > 0.6 {
		t.Errorf("expected sampling ratio around 0.5, got %v", ratio)
	}
}

func TestProbabilitySamplerInvalidTraceID(t *testing.T) {
	sampler, err := NewProbabilitySampler(0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sampler.ShouldSample("short") {
		t.Error("invalid trace id should not be sampled")
	}
	if sampler.ShouldSample("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz") {
		t.Error("non-hex trace id should not be sampled")
	}
}

func TestNewSpan(t *testing.T) {
	span := NewSpan("trace123", "span123", "parent123", "test-span", true)
	if span.TraceID != "trace123" {
		t.Errorf("expected TraceID trace123, got %s", span.TraceID)
	}
	if span.SpanID != "span123" {
		t.Errorf("expected SpanID span123, got %s", span.SpanID)
	}
	if span.ParentSpanID != "parent123" {
		t.Errorf("expected ParentSpanID parent123, got %s", span.ParentSpanID)
	}
	if span.Name != "test-span" {
		t.Errorf("expected Name test-span, got %s", span.Name)
	}
	if !span.Sampled {
		t.Error("expected Sampled true")
	}
	if span.Attributes == nil {
		t.Error("Attributes map should be initialized")
	}
}

func TestSpanAttributes(t *testing.T) {
	span := NewSpan("trace1", "span1", "", "test", true)

	span.SetAttribute("key1", "value1")
	span.SetAttribute("key2", "value2")

	val, ok := span.GetAttribute("key1")
	if !ok || val != "value1" {
		t.Errorf("expected value1 for key1, got %s, ok=%v", val, ok)
	}

	val, ok = span.GetAttribute("nonexistent")
	if ok {
		t.Errorf("expected not found for nonexistent key, got %s", val)
	}

	span.SetAttribute("key1", "updated")
	val, _ = span.GetAttribute("key1")
	if val != "updated" {
		t.Errorf("expected updated value for key1, got %s", val)
	}
}

func TestSpanIsRoot(t *testing.T) {
	rootSpan := NewSpan("trace1", "span1", "", "root", true)
	if !rootSpan.IsRoot() {
		t.Error("span with empty parent should be root")
	}

	childSpan := NewSpan("trace1", "span2", "span1", "child", true)
	if childSpan.IsRoot() {
		t.Error("span with parent should not be root")
	}
}

func TestNewSpanTree(t *testing.T) {
	tree := NewSpanTree()
	if tree == nil {
		t.Fatal("NewSpanTree returned nil")
	}
	if tree.SpanCount() != 0 {
		t.Errorf("expected 0 spans, got %d", tree.SpanCount())
	}
}

func TestSpanTreeAddSpan(t *testing.T) {
	tree := NewSpanTree()
	span := NewSpan("trace1", "span1", "", "root", true)

	err := tree.AddSpan(span)
	if err != nil {
		t.Fatalf("AddSpan failed: %v", err)
	}
	if tree.SpanCount() != 1 {
		t.Errorf("expected 1 span, got %d", tree.SpanCount())
	}
}

func TestSpanTreeAddNilSpan(t *testing.T) {
	tree := NewSpanTree()
	err := tree.AddSpan(nil)
	if !errors.Is(err, ErrNilSpan) {
		t.Errorf("expected ErrNilSpan, got %v", err)
	}
}

func TestSpanTreeAddEmptySpanID(t *testing.T) {
	tree := NewSpanTree()
	span := NewSpan("trace1", "", "", "test", true)
	err := tree.AddSpan(span)
	if !errors.Is(err, ErrEmptySpanID) {
		t.Errorf("expected ErrEmptySpanID, got %v", err)
	}
}

func TestSpanTreeAddEmptyTraceID(t *testing.T) {
	tree := NewSpanTree()
	span := NewSpan("", "span1", "", "test", true)
	err := tree.AddSpan(span)
	if !errors.Is(err, ErrEmptyTraceID) {
		t.Errorf("expected ErrEmptyTraceID, got %v", err)
	}
}

func TestSpanTreeAddDuplicateSpan(t *testing.T) {
	tree := NewSpanTree()
	span1 := NewSpan("trace1", "span1", "", "test1", true)
	span2 := NewSpan("trace1", "span1", "", "test2", true)

	err := tree.AddSpan(span1)
	if err != nil {
		t.Fatalf("first AddSpan failed: %v", err)
	}

	err = tree.AddSpan(span2)
	if !errors.Is(err, ErrDuplicateSpanID) {
		t.Errorf("expected ErrDuplicateSpanID, got %v", err)
	}
}

func TestSpanTreeGetSpan(t *testing.T) {
	tree := NewSpanTree()
	span := NewSpan("trace1", "span1", "", "test", true)
	tree.AddSpan(span)

	got, err := tree.GetSpan("span1")
	if err != nil {
		t.Fatalf("GetSpan failed: %v", err)
	}
	if got.SpanID != "span1" {
		t.Errorf("expected span1, got %s", got.SpanID)
	}
}

func TestSpanTreeGetSpanNotFound(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetSpan("nonexistent")
	if !errors.Is(err, ErrSpanNotFound) {
		t.Errorf("expected ErrSpanNotFound, got %v", err)
	}
}

func TestSpanTreeGetSpanEmptyID(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetSpan("")
	if !errors.Is(err, ErrEmptySpanID) {
		t.Errorf("expected ErrEmptySpanID, got %v", err)
	}
}

func TestSpanTreeGetChildren(t *testing.T) {
	tree := NewSpanTree()
	root := NewSpan("trace1", "root", "", "root", true)
	child1 := NewSpan("trace1", "child1", "root", "child1", true)
	child2 := NewSpan("trace1", "child2", "root", "child2", true)
	grandchild := NewSpan("trace1", "grandchild", "child1", "grandchild", true)

	tree.AddSpan(root)
	tree.AddSpan(child1)
	tree.AddSpan(child2)
	tree.AddSpan(grandchild)

	children, err := tree.GetChildren("root")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("expected 2 children, got %d", len(children))
	}
}

func TestSpanTreeGetChildrenNotFound(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetChildren("nonexistent")
	if !errors.Is(err, ErrSpanNotFound) {
		t.Errorf("expected ErrSpanNotFound, got %v", err)
	}
}

func TestSpanTreeGetChildrenEmptyID(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetChildren("")
	if !errors.Is(err, ErrEmptySpanID) {
		t.Errorf("expected ErrEmptySpanID, got %v", err)
	}
}

func TestSpanTreeGetRoots(t *testing.T) {
	tree := NewSpanTree()
	root1 := NewSpan("trace1", "root1", "", "root1", true)
	root2 := NewSpan("trace1", "root2", "", "root2", true)
	child := NewSpan("trace1", "child1", "root1", "child1", true)

	tree.AddSpan(root1)
	tree.AddSpan(root2)
	tree.AddSpan(child)

	roots := tree.GetRoots()
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}
}

func TestSpanTreeGetSubtree(t *testing.T) {
	tree := NewSpanTree()
	root := NewSpan("trace1", "root", "", "root", true)
	child1 := NewSpan("trace1", "child1", "root", "child1", true)
	child2 := NewSpan("trace1", "child2", "root", "child2", true)
	grandchild := NewSpan("trace1", "grandchild", "child1", "grandchild", true)

	tree.AddSpan(root)
	tree.AddSpan(child1)
	tree.AddSpan(child2)
	tree.AddSpan(grandchild)

	subtree, err := tree.GetSubtree("root")
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree) != 4 {
		t.Errorf("expected 4 spans in subtree, got %d", len(subtree))
	}

	subtree2, err := tree.GetSubtree("child1")
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree2) != 2 {
		t.Errorf("expected 2 spans in child1 subtree, got %d", len(subtree2))
	}
}

func TestSpanTreeGetSubtreeNotFound(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetSubtree("nonexistent")
	if !errors.Is(err, ErrSpanNotFound) {
		t.Errorf("expected ErrSpanNotFound, got %v", err)
	}
}

func TestSpanTreeGetSubtreeEmptyID(t *testing.T) {
	tree := NewSpanTree()
	_, err := tree.GetSubtree("")
	if !errors.Is(err, ErrEmptySpanID) {
		t.Errorf("expected ErrEmptySpanID, got %v", err)
	}
}

func TestSpanTreeAllSpans(t *testing.T) {
	tree := NewSpanTree()
	for i := 0; i < 5; i++ {
		span := NewSpan("trace1", fmtSpanID(i), "", fmt.Sprintf("span-%d", i), true)
		tree.AddSpan(span)
	}

	spans := tree.AllSpans()
	if len(spans) != 5 {
		t.Errorf("expected 5 spans, got %d", len(spans))
	}
}

func fmtSpanID(i int) string {
	return strings.Repeat(string(rune('0'+i)), 16)
}

func TestNewRootContext(t *testing.T) {
	sampler := NewAlwaysSample()
	ctx, span, err := NewRootContext("test-root", sampler)
	if err != nil {
		t.Fatalf("NewRootContext failed: %v", err)
	}
	if ctx == nil {
		t.Fatal("context is nil")
	}
	if span == nil {
		t.Fatal("span is nil")
	}
	if ctx.TraceID == "" {
		t.Error("TraceID should not be empty")
	}
	if ctx.SpanID == "" {
		t.Error("SpanID should not be empty")
	}
	if ctx.ParentSpanID != "" {
		t.Error("ParentSpanID should be empty for root context")
	}
	if !ctx.Sampled {
		t.Error("Sampled should be true with AlwaysSample")
	}
	if !span.IsRoot() {
		t.Error("root span should be root")
	}
	if span.Name != "test-root" {
		t.Errorf("expected name test-root, got %s", span.Name)
	}
}

func TestNewRootContextWithNeverSample(t *testing.T) {
	sampler := NewNeverSample()
	ctx, span, err := NewRootContext("test-root", sampler)
	if err != nil {
		t.Fatalf("NewRootContext failed: %v", err)
	}
	if ctx.Sampled {
		t.Error("Sampled should be false with NeverSample")
	}
	if span.Sampled {
		t.Error("span Sampled should be false with NeverSample")
	}
}

func TestNewChildContext(t *testing.T) {
	parentCtx := &TraceContext{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "0123456789abcdef",
		ParentSpanID: "",
		Sampled:      true,
	}

	ctx, span, err := NewChildContext(parentCtx, "child-span")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	if ctx.TraceID != parentCtx.TraceID {
		t.Error("TraceID should be same as parent")
	}
	if ctx.ParentSpanID != parentCtx.SpanID {
		t.Error("ParentSpanID should be parent's SpanID")
	}
	if ctx.SpanID == parentCtx.SpanID {
		t.Error("child SpanID should be different from parent")
	}
	if !ctx.Sampled {
		t.Error("Sampled should inherit from parent")
	}
	if span.ParentSpanID != parentCtx.SpanID {
		t.Error("span ParentSpanID should match")
	}
}

func TestNewChildContextNilParent(t *testing.T) {
	_, _, err := NewChildContext(nil, "child")
	if err == nil {
		t.Error("expected error for nil parent")
	}
}

func TestNewChildContextNotSampled(t *testing.T) {
	parentCtx := &TraceContext{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "0123456789abcdef",
		ParentSpanID: "",
		Sampled:      false,
	}

	ctx, span, err := NewChildContext(parentCtx, "child-span")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	if ctx.Sampled {
		t.Error("child should not be sampled if parent is not sampled")
	}
	if span.Sampled {
		t.Error("child span should not be sampled if parent is not sampled")
	}
}

func TestInjectTraceContext(t *testing.T) {
	ctx := &TraceContext{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "0123456789abcdef",
		ParentSpanID: "fedcba9876543210",
		Sampled:      true,
	}

	headers := InjectTraceContext(ctx)
	traceParent, ok := headers[W3CTraceParentHeader]
	if !ok {
		t.Fatal("traceparent header not found")
	}

	parts := strings.Split(traceParent, "-")
	if len(parts) != 4 {
		t.Fatalf("expected 4 parts in traceparent, got %d", len(parts))
	}
	if parts[0] != TraceParentVersion {
		t.Errorf("expected version %s, got %s", TraceParentVersion, parts[0])
	}
	if parts[1] != ctx.TraceID {
		t.Errorf("expected traceid %s, got %s", ctx.TraceID, parts[1])
	}
	if parts[2] != ctx.SpanID {
		t.Errorf("expected spanid %s, got %s", ctx.SpanID, parts[2])
	}
	if parts[3] != "01" {
		t.Errorf("expected flags 01, got %s", parts[3])
	}
}

func TestInjectTraceContextNotSampled(t *testing.T) {
	ctx := &TraceContext{
		TraceID: "0123456789abcdef0123456789abcdef",
		SpanID:  "0123456789abcdef",
		Sampled: false,
	}

	headers := InjectTraceContext(ctx)
	traceParent := headers[W3CTraceParentHeader]
	parts := strings.Split(traceParent, "-")
	if parts[3] != "00" {
		t.Errorf("expected flags 00 for not sampled, got %s", parts[3])
	}
}

func TestInjectTraceContextNil(t *testing.T) {
	headers := InjectTraceContext(nil)
	if len(headers) != 0 {
		t.Errorf("expected empty headers for nil context, got %v", headers)
	}
}

func TestExtractTraceContext(t *testing.T) {
	headers := map[string]string{
		W3CTraceParentHeader: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	}

	ctx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}
	if ctx.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("unexpected TraceID: %s", ctx.TraceID)
	}
	if ctx.SpanID != "0123456789abcdef" {
		t.Errorf("unexpected SpanID: %s", ctx.SpanID)
	}
	if ctx.ParentSpanID != "0123456789abcdef" {
		t.Errorf("ParentSpanID should be set to the parent-id from header, got %s", ctx.ParentSpanID)
	}
	if !ctx.Sampled {
		t.Error("expected Sampled true")
	}
}

func TestExtractTraceContextNotSampled(t *testing.T) {
	headers := map[string]string{
		W3CTraceParentHeader: "00-0123456789abcdef0123456789abcdef-0123456789abcdef-00",
	}

	ctx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}
	if ctx.Sampled {
		t.Error("expected Sampled false")
	}
}

func TestExtractTraceContextCaseInsensitive(t *testing.T) {
	headers := map[string]string{
		"Traceparent": "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01",
	}

	ctx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}
	if ctx.TraceID != "0123456789abcdef0123456789abcdef" {
		t.Errorf("unexpected TraceID: %s", ctx.TraceID)
	}
}

func TestExtractTraceContextMissingHeader(t *testing.T) {
	headers := map[string]string{}
	_, err := ExtractTraceContext(headers)
	if !errors.Is(err, ErrInvalidTraceParent) {
		t.Errorf("expected ErrInvalidTraceParent, got %v", err)
	}
}

func TestExtractTraceContextInvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"too few parts", "00-0123456789abcdef0123456789abcdef"},
		{"too many parts", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01-extra"},
		{"invalid version", "01-0123456789abcdef0123456789abcdef-0123456789abcdef-01"},
		{"invalid traceid", "00-zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz-0123456789abcdef-01"},
		{"short traceid", "00-0123456789abcdef-0123456789abcdef-01"},
		{"invalid spanid", "00-0123456789abcdef0123456789abcdef-zzzzzzzzzzzzzz-01"},
		{"short spanid", "00-0123456789abcdef0123456789abcdef-01234567-01"},
		{"invalid flags", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-zz"},
		{"short flags", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-0"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := map[string]string{
				W3CTraceParentHeader: tt.header,
			}
			_, err := ExtractTraceContext(headers)
			if err == nil {
				t.Error("expected error for invalid traceparent")
			}
		})
	}
}

func TestTraceContextString(t *testing.T) {
	ctx := &TraceContext{
		TraceID: "0123456789abcdef0123456789abcdef",
		SpanID:  "0123456789abcdef",
		Sampled: true,
	}

	s := ctx.String()
	expected := "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01"
	if s != expected {
		t.Errorf("expected %s, got %s", expected, s)
	}
}

func TestInjectExtractRoundTrip(t *testing.T) {
	original := &TraceContext{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "0123456789abcdef",
		ParentSpanID: "fedcba9876543210",
		Sampled:      true,
	}

	headers := InjectTraceContext(original)
	extracted, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}

	if extracted.TraceID != original.TraceID {
		t.Errorf("TraceID mismatch: %s vs %s", extracted.TraceID, original.TraceID)
	}
	if extracted.SpanID != original.SpanID {
		t.Errorf("SpanID mismatch: %s vs %s", extracted.SpanID, original.SpanID)
	}
	if extracted.ParentSpanID != original.SpanID {
		t.Errorf("ParentSpanID should equal original SpanID, got %s", extracted.ParentSpanID)
	}
	if extracted.Sampled != original.Sampled {
		t.Errorf("Sampled mismatch: %v vs %v", extracted.Sampled, original.Sampled)
	}
}

func TestFullTraceWorkflow(t *testing.T) {
	sampler := NewAlwaysSample()
	tree := NewSpanTree()

	rootCtx, rootSpan, err := NewRootContext("service-a", sampler)
	if err != nil {
		t.Fatalf("NewRootContext failed: %v", err)
	}
	tree.AddSpan(rootSpan)

	childCtx, childSpan, err := NewChildContext(rootCtx, "service-b")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	tree.AddSpan(childSpan)

	headers := InjectTraceContext(childCtx)

	extractedCtx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}

	grandchildCtx, grandchildSpan, err := NewChildContext(extractedCtx, "service-c")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	tree.AddSpan(grandchildSpan)

	if grandchildSpan.TraceID != rootSpan.TraceID {
		t.Error("all spans should have same TraceID")
	}

	subtree, err := tree.GetSubtree(rootSpan.SpanID)
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree) != 3 {
		t.Errorf("expected 3 spans in trace, got %d", len(subtree))
	}

	children, err := tree.GetChildren(rootSpan.SpanID)
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 1 {
		t.Errorf("expected 1 child of root, got %d", len(children))
	}

	_ = childCtx
	_ = grandchildCtx
}

func TestSpanTreeConcurrentAdd(t *testing.T) {
	tree := NewSpanTree()
	var wg sync.WaitGroup
	numSpans := 100
	traceID := "0123456789abcdef0123456789abcdef"

	for i := 0; i < numSpans; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			spanID := fmt.Sprintf("span-%04d", i)
			span := NewSpan(traceID, spanID, "", "test", true)
			tree.AddSpan(span)
		}(i)
	}

	wg.Wait()

	if tree.SpanCount() != numSpans {
		t.Errorf("expected %d spans, got %d", numSpans, tree.SpanCount())
	}
}

func generateTestSpanID(i int) string {
	return strings.Repeat(string(rune('0'+(i%10))), 16) + strings.Repeat(string(rune('a'+(i%26))), 16)
}

func TestSampledContextPropagation(t *testing.T) {
	sampler := NewNeverSample()

	rootCtx, _, err := NewRootContext("root", sampler)
	if err != nil {
		t.Fatalf("NewRootContext failed: %v", err)
	}
	if rootCtx.Sampled {
		t.Error("root should not be sampled")
	}

	childCtx, childSpan, err := NewChildContext(rootCtx, "child")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	if childCtx.Sampled {
		t.Error("child should not be sampled (propagated from parent)")
	}
	if childSpan.Sampled {
		t.Error("child span should not be sampled")
	}

	headers := InjectTraceContext(childCtx)
	extracted, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}
	if extracted.Sampled {
		t.Error("extracted context should not be sampled")
	}
}

func TestEdgeCases(t *testing.T) {
	t.Run("span with nil attributes", func(t *testing.T) {
		span := &Span{
			TraceID:  "trace1",
			SpanID:   "span1",
			Sampled:  true,
		}
		val, ok := span.GetAttribute("key")
		if ok {
			t.Error("should return false for nil attributes")
		}
		if val != "" {
			t.Errorf("expected empty string, got %s", val)
		}

		span.SetAttribute("key", "value")
		val, ok = span.GetAttribute("key")
		if !ok || val != "value" {
			t.Errorf("expected value, got %s, ok=%v", val, ok)
		}
	})

	t.Run("tree with single root", func(t *testing.T) {
		tree := NewSpanTree()
		root := NewSpan("trace1", "span1", "", "root", true)
		tree.AddSpan(root)

		roots := tree.GetRoots()
		if len(roots) != 1 {
			t.Errorf("expected 1 root, got %d", len(roots))
		}

		children, _ := tree.GetChildren("span1")
		if len(children) != 0 {
			t.Errorf("expected 0 children, got %d", len(children))
		}

		subtree, _ := tree.GetSubtree("span1")
		if len(subtree) != 1 {
			t.Errorf("expected 1 span in subtree, got %d", len(subtree))
		}
	})

	t.Run("deeply nested spans", func(t *testing.T) {
		tree := NewSpanTree()
		depth := 10
		var prevID string

		for i := 0; i < depth; i++ {
			spanID := fmt.Sprintf("span-%016d", i)
			span := NewSpan("trace1", spanID, prevID, fmt.Sprintf("level-%d", i), true)
			tree.AddSpan(span)
			prevID = spanID
		}

		subtree, err := tree.GetSubtree("span-0000000000000000")
		if err != nil {
			t.Fatalf("GetSubtree failed: %v", err)
		}
		if len(subtree) != depth {
			t.Errorf("expected %d spans, got %d", depth, len(subtree))
		}
	})
}

func TestExtractTraceContextParentSpanIDPropagation(t *testing.T) {
	parentCtx := &TraceContext{
		TraceID:      "0123456789abcdef0123456789abcdef",
		SpanID:       "aaaabbbbccccdddd",
		ParentSpanID: "1111222233334444",
		Sampled:      true,
	}

	headers := InjectTraceContext(parentCtx)
	extracted, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}

	if extracted.ParentSpanID != "aaaabbbbccccdddd" {
		t.Errorf("ParentSpanID should be set to parent-id from header, got %s", extracted.ParentSpanID)
	}

	childCtx, childSpan, err := NewChildContext(extracted, "child")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	if childCtx.ParentSpanID != extracted.SpanID {
		t.Errorf("child ParentSpanID should be extracted.SpanID, got %s", childCtx.ParentSpanID)
	}
	if childSpan.ParentSpanID != extracted.SpanID {
		t.Errorf("child span ParentSpanID should be extracted.SpanID, got %s", childSpan.ParentSpanID)
	}
}

func TestExtractTraceContextParentSpanIDNotSampled(t *testing.T) {
	headers := map[string]string{
		W3CTraceParentHeader: "00-0123456789abcdef0123456789abcdef-aaaabbbbccccdddd-00",
	}

	ctx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}
	if ctx.ParentSpanID != "aaaabbbbccccdddd" {
		t.Errorf("ParentSpanID should be set even when not sampled, got %s", ctx.ParentSpanID)
	}
}

func TestSpanTreeTraceIDMismatch(t *testing.T) {
	tree := NewSpanTree()
	span1 := NewSpan("trace1", "span1", "", "root", true)
	span2 := NewSpan("trace2", "span2", "span1", "child", true)

	err := tree.AddSpan(span1)
	if err != nil {
		t.Fatalf("first AddSpan failed: %v", err)
	}

	err = tree.AddSpan(span2)
	if !errors.Is(err, ErrTraceIDMismatch) {
		t.Errorf("expected ErrTraceIDMismatch, got %v", err)
	}
}

func TestSpanTreeTraceIDConsistency(t *testing.T) {
	tree := NewSpanTree()
	traceID := "0123456789abcdef0123456789abcdef"

	span1 := NewSpan(traceID, "span1", "", "root", true)
	span2 := NewSpan(traceID, "span2", "span1", "child", true)

	err := tree.AddSpan(span1)
	if err != nil {
		t.Fatalf("AddSpan span1 failed: %v", err)
	}
	err = tree.AddSpan(span2)
	if err != nil {
		t.Fatalf("AddSpan span2 failed: %v", err)
	}

	if tree.TraceID() != traceID {
		t.Errorf("expected TraceID %s, got %s", traceID, tree.TraceID())
	}
}

func TestSpanTreeTraceIDSetOnFirstSpan(t *testing.T) {
	tree := NewSpanTree()

	if tree.TraceID() != "" {
		t.Errorf("expected empty TraceID on new tree, got %s", tree.TraceID())
	}

	span := NewSpan("trace1", "span1", "", "root", true)
	tree.AddSpan(span)

	if tree.TraceID() != "trace1" {
		t.Errorf("expected TraceID trace1, got %s", tree.TraceID())
	}
}

func TestSpanTreeTraceIDMismatchWithEmptyParent(t *testing.T) {
	tree := NewSpanTree()
	span1 := NewSpan("trace1", "span1", "", "root1", true)
	span2 := NewSpan("trace2", "span2", "", "root2", true)

	tree.AddSpan(span1)
	err := tree.AddSpan(span2)
	if !errors.Is(err, ErrTraceIDMismatch) {
		t.Errorf("expected ErrTraceIDMismatch for second root with different TraceID, got %v", err)
	}
}

func TestSpanTreeGetChildrenNoChildren(t *testing.T) {
	tree := NewSpanTree()
	root := NewSpan("trace1", "root", "", "root", true)
	child := NewSpan("trace1", "child1", "root", "child1", true)
	tree.AddSpan(root)
	tree.AddSpan(child)

	children, err := tree.GetChildren("child1")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 0 {
		t.Errorf("expected 0 children for leaf node, got %d", len(children))
	}
}

func TestSpanTreeGetChildrenReturnsCopy(t *testing.T) {
	tree := NewSpanTree()
	root := NewSpan("trace1", "root", "", "root", true)
	child := NewSpan("trace1", "child1", "root", "child1", true)
	tree.AddSpan(root)
	tree.AddSpan(child)

	children1, _ := tree.GetChildren("root")
	children1[0] = nil

	children2, _ := tree.GetChildren("root")
	if children2[0] == nil {
		t.Error("GetChildren should return a copy, not a reference to internal slice")
	}
}

func TestSpanTreeLargeScalePerformance(t *testing.T) {
	tree := NewSpanTree()
	traceID := "0123456789abcdef0123456789abcdef"
	numSpans := 5000

	root := NewSpan(traceID, "span-0", "", "root", true)
	tree.AddSpan(root)

	for i := 1; i < numSpans; i++ {
		parentID := fmt.Sprintf("span-%d", (i-1)/3)
		spanID := fmt.Sprintf("span-%d", i)
		span := NewSpan(traceID, spanID, parentID, fmt.Sprintf("node-%d", i), true)
		tree.AddSpan(span)
	}

	if tree.SpanCount() != numSpans {
		t.Errorf("expected %d spans, got %d", numSpans, tree.SpanCount())
	}

	children, err := tree.GetChildren("span-0")
	if err != nil {
		t.Fatalf("GetChildren failed: %v", err)
	}
	if len(children) != 3 {
		t.Errorf("expected 3 children for span-0, got %d", len(children))
	}

	subtree, err := tree.GetSubtree("span-1")
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree) == 0 {
		t.Error("subtree should not be empty")
	}
}

func TestSpanTreeSubtreeOfLeafNode(t *testing.T) {
	tree := NewSpanTree()
	root := NewSpan("trace1", "root", "", "root", true)
	child := NewSpan("trace1", "child1", "root", "child1", true)
	tree.AddSpan(root)
	tree.AddSpan(child)

	subtree, err := tree.GetSubtree("child1")
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree) != 1 {
		t.Errorf("leaf node subtree should contain only itself, got %d", len(subtree))
	}
	if subtree[0].SpanID != "child1" {
		t.Errorf("expected child1, got %s", subtree[0].SpanID)
	}
}

func TestSpanTreeMultipleRootsSameTrace(t *testing.T) {
	tree := NewSpanTree()
	root1 := NewSpan("trace1", "root1", "", "root1", true)
	root2 := NewSpan("trace1", "root2", "", "root2", true)
	child1 := NewSpan("trace1", "child1", "root1", "child1", true)
	child2 := NewSpan("trace1", "child2", "root2", "child2", true)

	tree.AddSpan(root1)
	tree.AddSpan(root2)
	tree.AddSpan(child1)
	tree.AddSpan(child2)

	roots := tree.GetRoots()
	if len(roots) != 2 {
		t.Errorf("expected 2 roots, got %d", len(roots))
	}

	subtree1, _ := tree.GetSubtree("root1")
	if len(subtree1) != 2 {
		t.Errorf("expected 2 spans in root1 subtree, got %d", len(subtree1))
	}

	subtree2, _ := tree.GetSubtree("root2")
	if len(subtree2) != 2 {
		t.Errorf("expected 2 spans in root2 subtree, got %d", len(subtree2))
	}
}

func TestSpanTreeConcurrentAddSameTrace(t *testing.T) {
	tree := NewSpanTree()
	var wg sync.WaitGroup
	numSpans := 100
	traceID := "0123456789abcdef0123456789abcdef"

	for i := 0; i < numSpans; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			spanID := fmt.Sprintf("span-%04d", i)
			span := NewSpan(traceID, spanID, "", "test", true)
			tree.AddSpan(span)
		}(i)
	}

	wg.Wait()

	if tree.SpanCount() != numSpans {
		t.Errorf("expected %d spans, got %d", numSpans, tree.SpanCount())
	}
	if tree.TraceID() != traceID {
		t.Errorf("expected TraceID %s, got %s", traceID, tree.TraceID())
	}
}

func TestSpanTreeConcurrentReadWrite(t *testing.T) {
	tree := NewSpanTree()
	traceID := "0123456789abcdef0123456789abcdef"

	root := NewSpan(traceID, "root", "", "root", true)
	tree.AddSpan(root)

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					tree.GetChildren("root")
				}
			}
		}()
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					spanID := fmt.Sprintf("child-%04d", i)
					span := NewSpan(traceID, spanID, "root", "child", true)
					tree.AddSpan(span)
				}
			}
		}(i)
	}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					tree.GetSubtree("root")
				}
			}
		}()
	}

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					tree.SpanCount()
					tree.TraceID()
				}
			}
		}()
	}

	close(stop)
	wg.Wait()
}

func TestSpanTreeConcurrentAddDifferentTrace(t *testing.T) {
	tree := NewSpanTree()
	span1 := NewSpan("trace1", "span1", "", "root", true)
	tree.AddSpan(span1)

	span2 := NewSpan("trace2", "span2", "", "root", true)
	err := tree.AddSpan(span2)
	if !errors.Is(err, ErrTraceIDMismatch) {
		t.Errorf("expected ErrTraceIDMismatch, got %v", err)
	}
}

func TestFullTraceWorkflowWithParentSpanID(t *testing.T) {
	sampler := NewAlwaysSample()
	tree := NewSpanTree()

	rootCtx, rootSpan, err := NewRootContext("service-a", sampler)
	if err != nil {
		t.Fatalf("NewRootContext failed: %v", err)
	}
	tree.AddSpan(rootSpan)

	childCtx, childSpan, err := NewChildContext(rootCtx, "service-b")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	tree.AddSpan(childSpan)

	headers := InjectTraceContext(childCtx)
	extractedCtx, err := ExtractTraceContext(headers)
	if err != nil {
		t.Fatalf("ExtractTraceContext failed: %v", err)
	}

	if extractedCtx.ParentSpanID != childCtx.SpanID {
		t.Errorf("extracted ParentSpanID should be child SpanID, got %s", extractedCtx.ParentSpanID)
	}

	grandchildCtx, grandchildSpan, err := NewChildContext(extractedCtx, "service-c")
	if err != nil {
		t.Fatalf("NewChildContext failed: %v", err)
	}
	tree.AddSpan(grandchildSpan)

	if grandchildSpan.ParentSpanID != childCtx.SpanID {
		t.Errorf("grandchild ParentSpanID should be child SpanID, got %s", grandchildSpan.ParentSpanID)
	}
	if grandchildSpan.TraceID != rootSpan.TraceID {
		t.Error("all spans should have same TraceID")
	}

	subtree, err := tree.GetSubtree(rootSpan.SpanID)
	if err != nil {
		t.Fatalf("GetSubtree failed: %v", err)
	}
	if len(subtree) != 3 {
		t.Errorf("expected 3 spans in trace, got %d", len(subtree))
	}

	_ = grandchildCtx
}
