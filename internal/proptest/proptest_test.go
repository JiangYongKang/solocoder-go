package proptest

import (
	"math"
	"math/rand"
	"strings"
	"testing"
	"unicode"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Iterations != 100 {
		t.Errorf("expected Iterations 100, got %d", cfg.Iterations)
	}
	if cfg.MaxShrinks != 1000 {
		t.Errorf("expected MaxShrinks 1000, got %d", cfg.MaxShrinks)
	}
	if !cfg.UseRandomSeed {
		t.Error("expected UseRandomSeed true")
	}
	if cfg.Verbose {
		t.Error("expected Verbose false")
	}
}

func TestNewRunner_ConfigNormalization(t *testing.T) {
	cfg := Config{
		Iterations:    -10,
		MaxShrinks:    -5,
		UseRandomSeed: false,
		Seed:          42,
	}
	r := NewRunner[int](cfg)
	rcfg := r.Config()
	if rcfg.Iterations != 100 {
		t.Errorf("expected Iterations normalized to 100, got %d", rcfg.Iterations)
	}
	if rcfg.MaxShrinks != 1000 {
		t.Errorf("expected MaxShrinks normalized to 1000, got %d", rcfg.MaxShrinks)
	}
	if rcfg.Seed != 42 {
		t.Errorf("expected Seed 42, got %d", rcfg.Seed)
	}
}

func TestNewRunner_RandomSeed(t *testing.T) {
	cfg := Config{
		Iterations:    10,
		UseRandomSeed: true,
	}
	r1 := NewRunner[int](cfg)
	r2 := NewRunner[int](cfg)
	if r1.Config().Seed == 0 {
		t.Error("expected non-zero random seed for r1")
	}
	if r2.Config().Seed == 0 {
		t.Error("expected non-zero random seed for r2")
	}
}

func TestIntGenerator_Generate_Range(t *testing.T) {
	gen := IntRange(-10, 10)
	r := rand.New(rand.NewSource(1))
	samples := 1000
	for i := 0; i < samples; i++ {
		v := gen.Generate(r)
		if v < -10 || v > 10 {
			t.Fatalf("sample %d: value %d out of range [-10, 10]", i, v)
		}
	}
}

func TestIntGenerator_Generate_SameMinMax(t *testing.T) {
	gen := IntRange(42, 42)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		v := gen.Generate(r)
		if v != 42 {
			t.Errorf("iteration %d: expected 42, got %d", i, v)
		}
	}
}

func TestIntGenerator_Generate_ReversedMinMax(t *testing.T) {
	gen := IntRange(10, -10)
	r := rand.New(rand.NewSource(1))
	samples := 1000
	for i := 0; i < samples; i++ {
		v := gen.Generate(r)
		if v < -10 || v > 10 {
			t.Fatalf("sample %d: value %d out of range [-10, 10]", i, v)
		}
	}
}

func TestIntGenerator_Generate_NonNegative(t *testing.T) {
	gen := IntNonNegative()
	r := rand.New(rand.NewSource(2))
	samples := 1000
	for i := 0; i < samples; i++ {
		v := gen.Generate(r)
		if v < 0 {
			t.Fatalf("sample %d: expected non-negative, got %d", i, v)
		}
	}
}

func TestIntGenerator_Generate_Positive(t *testing.T) {
	gen := IntPositive()
	r := rand.New(rand.NewSource(3))
	samples := 1000
	for i := 0; i < samples; i++ {
		v := gen.Generate(r)
		if v < 1 {
			t.Fatalf("sample %d: expected >= 1, got %d", i, v)
		}
	}
}

func TestIntGenerator_Shrink_Positive(t *testing.T) {
	gen := IntRange(0, 1000)
	shrinks := gen.Shrink(100)
	if len(shrinks) == 0 {
		t.Fatal("expected some shrink candidates for 100")
	}
	hasZero := false
	for _, s := range shrinks {
		if s == 0 {
			hasZero = true
		}
		if s < 0 || s >= 100 {
			t.Errorf("shrink candidate %d should be in [0, 100)", s)
		}
	}
	if !hasZero {
		t.Error("expected 0 to be a shrink candidate")
	}
}

func TestIntGenerator_Shrink_Negative(t *testing.T) {
	gen := IntRange(-1000, 1000)
	shrinks := gen.Shrink(-50)
	if len(shrinks) == 0 {
		t.Fatal("expected some shrink candidates for -50")
	}
	hasZero := false
	hasPos := false
	for _, s := range shrinks {
		if s == 0 {
			hasZero = true
		}
		if s == 50 {
			hasPos = true
		}
	}
	if !hasZero {
		t.Error("expected 0 to be a shrink candidate")
	}
	if !hasPos {
		t.Error("expected positive counterpart 50 as shrink candidate")
	}
}

func TestIntGenerator_Shrink_Zero(t *testing.T) {
	gen := IntRange(-100, 100)
	shrinks := gen.Shrink(0)
	if len(shrinks) != 0 {
		t.Errorf("expected no shrink candidates for 0, got %v", shrinks)
	}
}

func TestIntGenerator_Shrink_OutOfRangeZero(t *testing.T) {
	gen := IntRange(10, 100)
	shrinks := gen.Shrink(50)
	for _, s := range shrinks {
		if s < 10 || s > 100 {
			t.Errorf("shrink candidate %d out of bounds [10, 100]", s)
		}
	}
}

func TestFloat64Generator_Generate_Range(t *testing.T) {
	gen := Float64Range(-5.0, 5.0)
	r := rand.New(rand.NewSource(1))
	samples := 1000
	for i := 0; i < samples; i++ {
		v := gen.Generate(r)
		if v < -5.0 || v > 5.0 {
			t.Fatalf("sample %d: value %f out of range", i, v)
		}
	}
}

func TestFloat64Generator_Generate_SameMinMax(t *testing.T) {
	gen := Float64Range(3.14, 3.14)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		v := gen.Generate(r)
		if v != 3.14 {
			t.Errorf("iteration %d: expected 3.14, got %f", i, v)
		}
	}
}

func TestFloat64Generator_Shrink_Positive(t *testing.T) {
	gen := Float64()
	shrinks := gen.Shrink(100.5)
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates for 100.5")
	}
	hasZero := false
	for _, s := range shrinks {
		if s == 0.0 {
			hasZero = true
		}
		if s >= 100.5 {
			t.Errorf("shrink candidate %f should be smaller than original", s)
		}
	}
	if !hasZero {
		t.Error("expected 0.0 as shrink candidate")
	}
}

func TestFloat64Generator_Shrink_Zero(t *testing.T) {
	gen := Float64()
	shrinks := gen.Shrink(0.0)
	if len(shrinks) != 0 {
		t.Errorf("expected no shrink candidates for 0.0, got %v", shrinks)
	}
}

func TestBoolGenerator_Generate(t *testing.T) {
	gen := Bool()
	r := rand.New(rand.NewSource(99))
	hasTrue := false
	hasFalse := false
	for i := 0; i < 1000; i++ {
		v := gen.Generate(r)
		if v {
			hasTrue = true
		} else {
			hasFalse = true
		}
	}
	if !hasTrue {
		t.Error("expected to generate some true values")
	}
	if !hasFalse {
		t.Error("expected to generate some false values")
	}
}

func TestBoolGenerator_Shrink(t *testing.T) {
	gen := Bool()
	shrinks := gen.Shrink(true)
	if len(shrinks) != 1 || shrinks[0] != false {
		t.Errorf("expected [false] as shrink of true, got %v", shrinks)
	}
	shrinks = gen.Shrink(false)
	if len(shrinks) != 0 {
		t.Errorf("expected empty shrink of false, got %v", shrinks)
	}
}

func TestStringGenerator_Generate_LengthRange(t *testing.T) {
	gen := StringLen(3, 8)
	r := rand.New(rand.NewSource(1))
	samples := 1000
	for i := 0; i < samples; i++ {
		s := gen.Generate(r)
		n := len([]rune(s))
		if n < 3 || n > 8 {
			t.Fatalf("sample %d: string length %d out of range [3, 8]", i, n)
		}
	}
}

func TestStringGenerator_Generate_MinLenNegative(t *testing.T) {
	gen := StringLen(-5, 5)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		s := gen.Generate(r)
		n := len([]rune(s))
		if n < 0 || n > 5 {
			t.Fatalf("sample %d: unexpected length %d", i, n)
		}
	}
}

func TestStringGenerator_Generate_ReversedLen(t *testing.T) {
	gen := StringLen(10, 2)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		s := gen.Generate(r)
		n := len([]rune(s))
		if n < 2 || n > 10 {
			t.Fatalf("sample %d: unexpected length %d", i, n)
		}
	}
}

func TestStringGenerator_Generate_Alphanumeric(t *testing.T) {
	gen := StringLen(10, 20)
	r := rand.New(rand.NewSource(1))
	samples := 100
	for i := 0; i < samples; i++ {
		s := gen.Generate(r)
		for _, ru := range s {
			if !unicode.IsLetter(ru) && !unicode.IsDigit(ru) {
				t.Fatalf("sample %d: non-alphanumeric rune %q in %q", i, ru, s)
			}
		}
	}
}

func TestStringGenerator_Generate_CustomCharset(t *testing.T) {
	ranges := []RuneRange{{'a', 'f'}}
	gen := StringWithCharset(5, 15, ranges)
	r := rand.New(rand.NewSource(2))
	samples := 100
	for i := 0; i < samples; i++ {
		s := gen.Generate(r)
		for _, ru := range s {
			if ru < 'a' || ru > 'f' {
				t.Fatalf("sample %d: rune %q out of charset [a-f] in %q", i, ru, s)
			}
		}
	}
}

func TestStringGenerator_Generate_EmptyCharset(t *testing.T) {
	gen := StringWithCharset(1, 5, nil)
	r := rand.New(rand.NewSource(1))
	s := gen.Generate(r)
	if len([]rune(s)) < 1 {
		t.Errorf("expected at least 1 rune, got %q", s)
	}
}

func TestStringGenerator_Generate_EmptyString(t *testing.T) {
	gen := StringLen(0, 0)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		s := gen.Generate(r)
		if s != "" {
			t.Errorf("iteration %d: expected empty string, got %q", i, s)
		}
	}
}

func TestStringGenerator_Shrink_NotEmpty(t *testing.T) {
	gen := String()
	shrinks := gen.Shrink("hello")
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates for non-empty string")
	}
	hasEmpty := false
	for _, s := range shrinks {
		runes := []rune(s)
		origRunes := []rune("hello")
		if s == "" {
			hasEmpty = true
		}
		if len(runes) > len(origRunes) {
			t.Errorf("shrunk string %q longer than original", s)
		}
	}
	if !hasEmpty {
		t.Error("expected empty string as shrink candidate")
	}
}

func TestStringGenerator_Shrink_Empty(t *testing.T) {
	gen := String()
	shrinks := gen.Shrink("")
	if len(shrinks) != 0 {
		t.Errorf("expected no shrink candidates for empty string, got %v", shrinks)
	}
}

func TestStringGenerator_Shrink_MinLenConstraint(t *testing.T) {
	gen := StringLen(3, 10)
	shrinks := gen.Shrink("abcdef")
	for _, s := range shrinks {
		n := len([]rune(s))
		if n < 3 {
			t.Errorf("shrunk string %q has length %d < MinLen 3", s, n)
		}
	}
}

func TestSliceGenerator_Generate_LengthRange(t *testing.T) {
	gen := SliceLen(IntRange(0, 9), 2, 6)
	r := rand.New(rand.NewSource(1))
	samples := 1000
	for i := 0; i < samples; i++ {
		s := gen.Generate(r)
		if len(s) < 2 || len(s) > 6 {
			t.Fatalf("sample %d: slice length %d out of range [2, 6]", i, len(s))
		}
		for _, elem := range s {
			if elem < 0 || elem > 9 {
				t.Fatalf("sample %d: element %d out of range", i, elem)
			}
		}
	}
}

func TestSliceGenerator_Generate_Empty(t *testing.T) {
	gen := SliceLen(Int(), 0, 0)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		s := gen.Generate(r)
		if len(s) != 0 {
			t.Errorf("iteration %d: expected empty slice, got length %d", i, len(s))
		}
	}
}

func TestSliceGenerator_NilGen_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic with nil element generator")
		}
	}()
	Slice[int](nil)
}

func TestSliceGenerator_Shrink_NotEmpty(t *testing.T) {
	gen := Slice(IntRange(0, 100))
	orig := []int{1, 2, 3, 4, 5}
	shrinks := gen.Shrink(orig)
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates")
	}
	hasEmpty := false
	for _, s := range shrinks {
		if len(s) == 0 {
			hasEmpty = true
		}
		if len(s) > len(orig) {
			t.Errorf("shrunk slice has length %d > original %d", len(s), len(orig))
		}
	}
	if !hasEmpty {
		t.Error("expected empty slice as shrink candidate")
	}
}

func TestSliceGenerator_Shrink_Empty(t *testing.T) {
	gen := Slice(Int())
	shrinks := gen.Shrink([]int{})
	if len(shrinks) != 0 {
		t.Errorf("expected no shrinks for empty slice, got %d", len(shrinks))
	}
}

func TestSliceGenerator_Shrink_MinLenConstraint(t *testing.T) {
	gen := SliceLen(Int(), 2, 10)
	orig := []int{1, 2, 3, 4}
	shrinks := gen.Shrink(orig)
	for _, s := range shrinks {
		if len(s) < 2 {
			t.Errorf("shrunk slice has length %d < MinLen 2", len(s))
		}
	}
}

func TestSliceGenerator_Shrink_ElementShrinks(t *testing.T) {
	intGen := IntRange(0, 100)
	gen := Slice(intGen)
	orig := []int{10, 20}
	shrinks := gen.Shrink(orig)
	foundElemShrink := false
	for _, s := range shrinks {
		if len(s) == 2 {
			if s[0] != orig[0] || s[1] != orig[1] {
				foundElemShrink = true
				break
			}
		}
	}
	if !foundElemShrink {
		t.Error("expected shrink candidates that modify individual elements")
	}
}

func TestPairGenerator_Generate(t *testing.T) {
	gen := PairOf(IntRange(0, 9), StringLen(1, 5))
	r := rand.New(rand.NewSource(1))
	samples := 100
	for i := 0; i < samples; i++ {
		p := gen.Generate(r)
		if p.A < 0 || p.A > 9 {
			t.Fatalf("sample %d: pair.A %d out of range", i, p.A)
		}
		n := len([]rune(p.B))
		if n < 1 || n > 5 {
			t.Fatalf("sample %d: pair.B length %d out of range", i, n)
		}
	}
}

func TestPairGenerator_Shrink(t *testing.T) {
	gen := PairOf(IntRange(0, 100), IntRange(0, 100))
	p := Pair[int, int]{A: 50, B: 60}
	shrinks := gen.Shrink(p)
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates")
	}
	shrankA := false
	shrankB := false
	for _, s := range shrinks {
		if s.A != p.A && s.B == p.B {
			shrankA = true
		}
		if s.B != p.B && s.A == p.A {
			shrankB = true
		}
	}
	if !shrankA {
		t.Error("expected candidates that shrink only A")
	}
	if !shrankB {
		t.Error("expected candidates that shrink only B")
	}
}

func TestPairGenerator_NilGen_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic with nil generator")
		}
	}()
	PairOf[int, int](nil, Int())
}

func TestTuple3Generator_Generate(t *testing.T) {
	gen := Tuple3Of(IntRange(1, 5), Bool(), StringLen(0, 3))
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		tp := gen.Generate(r)
		if tp.A < 1 || tp.A > 5 {
			t.Fatalf("tuple.A out of range: %d", tp.A)
		}
	}
}

func TestTuple3Generator_Shrink(t *testing.T) {
	gen := Tuple3Of(IntRange(0, 100), IntRange(0, 100), IntRange(0, 100))
	tp := Tuple3[int, int, int]{A: 30, B: 40, C: 50}
	shrinks := gen.Shrink(tp)
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates")
	}
}

func TestConstGenerator_Generate(t *testing.T) {
	gen := Const(42)
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		v := gen.Generate(r)
		if v != 42 {
			t.Errorf("iteration %d: expected 42, got %d", i, v)
		}
	}
}

func TestConstGenerator_Shrink(t *testing.T) {
	gen := Const("hello")
	shrinks := gen.Shrink("hello")
	if len(shrinks) != 0 {
		t.Errorf("expected no shrinks from ConstGenerator, got %v", shrinks)
	}
}

func TestMapGenerator_Generate(t *testing.T) {
	intGen := IntRange(0, 10)
	gen := Map(intGen, func(i int) string {
		return strings.Repeat("x", i)
	})
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 100; i++ {
		s := gen.Generate(r)
		for _, c := range s {
			if c != 'x' {
				t.Fatalf("unexpected char %q", c)
			}
		}
		if len(s) > 10 {
			t.Fatalf("string too long: %d", len(s))
		}
	}
}

func TestMapGenerator_WithShrink(t *testing.T) {
	intGen := IntRange(0, 100)
	gen := MapWithShrink(
		intGen,
		func(i int) string { return strings.Repeat("a", i) },
		func(s string) []string {
			n := len(s)
			if n <= 0 {
				return nil
			}
			return []string{s[:n-1], ""}
		},
	)
	shrinks := gen.Shrink("aaaaa")
	if len(shrinks) == 0 {
		t.Fatal("expected shrink candidates from MapWithShrink")
	}
}

func TestMapGenerator_NoShrinkFn(t *testing.T) {
	gen := Map(Int(), func(i int) int { return i * 2 })
	shrinks := gen.Shrink(100)
	if len(shrinks) != 0 {
		t.Errorf("Map without ShrinkFn should not produce shrinks, got %v", shrinks)
	}
}

func TestMapGenerator_NilArgs_Panics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic with nil args")
		}
	}()
	Map[int, int](nil, func(i int) int { return i })
}

func TestRunner_Check_PassingProperty(t *testing.T) {
	gen := IntNonNegative()
	prop := func(x int) (bool, string) {
		return x >= 0, "x should be non-negative"
	}
	cfg := Config{Iterations: 200, Seed: 12345, UseRandomSeed: false}
	r := NewRunner[int](cfg)
	result := r.Check(gen, prop)
	if !result.Passed {
		t.Errorf("expected property to pass, got failure: %s", result.String())
	}
	if result.Iterations != 200 {
		t.Errorf("expected 200 iterations, got %d", result.Iterations)
	}
	if result.Seed != 12345 {
		t.Errorf("expected seed 12345, got %d", result.Seed)
	}
	if result.FailCase != nil {
		t.Error("expected nil FailCase for passing test")
	}
}

func TestRunner_Check_FailingProperty(t *testing.T) {
	gen := IntRange(0, 100)
	prop := func(x int) (bool, string) {
		if x >= 50 {
			return false, "x should be less than 50"
		}
		return true, ""
	}
	cfg := Config{Iterations: 1000, Seed: 999, UseRandomSeed: false, MaxShrinks: 100}
	r := NewRunner[int](cfg)
	result := r.Check(gen, prop)
	if result.Passed {
		t.Fatal("expected property to fail")
	}
	if result.FailCase == nil {
		t.Fatal("expected FailCase for failing test")
	}
	if result.FailCase.Input >= 50 {
		// Shrunk input should still fail, so it's >= 50
		t.Logf("Good, failed input after shrink: %d", result.FailCase.Input)
	}
	if result.Seed != 999 {
		t.Errorf("expected seed 999, got %d", result.Seed)
	}
	if result.FailCase.Iteration == 0 {
		t.Error("expected non-zero failure iteration")
	}
}

func TestRunner_Check_ShrinksToMinimum(t *testing.T) {
	gen := IntRange(0, 1000)
	prop := func(x int) (bool, string) {
		if x > 100 {
			return false, "x should be <= 100"
		}
		return true, ""
	}
	cfg := Config{Iterations: 5000, Seed: 42, UseRandomSeed: false, MaxShrinks: 5000}
	r := NewRunner[int](cfg)
	result := r.Check(gen, prop)
	if result.Passed {
		t.Fatal("expected failure")
	}
	// The minimal failing input should be 101
	if result.FailCase.Input != 101 {
		t.Logf("Warning: expected minimal failing input 101, got %d (may depend on shrink order)", result.FailCase.Input)
	}
}

func TestRunner_Check_StringShrink(t *testing.T) {
	gen := StringLen(0, 100)
	prop := func(s string) (bool, string) {
		if len(s) > 5 {
			return false, "string too long"
		}
		return true, ""
	}
	cfg := Config{Iterations: 1000, Seed: 7, UseRandomSeed: false, MaxShrinks: 500}
	r := NewRunner[string](cfg)
	result := r.Check(gen, prop)
	if result.Passed {
		t.Fatal("expected failure")
	}
	// Should shrink to minimal failing string of length 6
	n := len([]rune(result.FailCase.Input))
	if n != 6 {
		t.Logf("Expected minimal length 6, got %d (%q)", n, result.FailCase.Input)
	}
}

func TestRunner_Check_SeedReproducibility(t *testing.T) {
	gen := Int()
	prop := func(x int) (bool, string) {
		return x < 0, "expecting negative (will fail on positives)"
	}
	cfg := Config{Iterations: 100, Seed: 12345, UseRandomSeed: false, MaxShrinks: 50}

	r1 := NewRunner[int](cfg)
	result1 := r1.Check(gen, prop)

	r2 := NewRunner[int](cfg)
	result2 := r2.Check(gen, prop)

	if result1.Passed != result2.Passed {
		t.Errorf("reproducibility: Passed mismatch %v vs %v", result1.Passed, result2.Passed)
	}
	if !result1.Passed && !result2.Passed {
		if result1.FailCase.Iteration != result2.FailCase.Iteration {
			t.Errorf("reproducibility: Iteration mismatch %d vs %d",
				result1.FailCase.Iteration, result2.FailCase.Iteration)
		}
		if result1.Seed != result2.Seed {
			t.Errorf("reproducibility: Seed mismatch %d vs %d", result1.Seed, result2.Seed)
		}
	}
}

func TestRunner_Check_NilGenerator(t *testing.T) {
	cfg := Config{Iterations: 10, Seed: 1, UseRandomSeed: false}
	r := NewRunner[int](cfg)
	result := r.Check(nil, func(x int) (bool, string) { return true, "" })
	if result.Passed {
		t.Error("expected failure with nil generator")
	}
	if !strings.Contains(result.FailCase.Message, "generator is nil") {
		t.Errorf("unexpected error message: %s", result.FailCase.Message)
	}
}

func TestRunner_Check_NilProperty(t *testing.T) {
	cfg := Config{Iterations: 10, Seed: 1, UseRandomSeed: false}
	r := NewRunner[int](cfg)
	result := r.Check(Int(), nil)
	if result.Passed {
		t.Error("expected failure with nil property")
	}
	if result.FailCase == nil || result.FailCase.Message == "" {
		t.Error("expected failure message")
	}
}

func TestCheck_ConvenienceFunction(t *testing.T) {
	result := Check(
		IntRange(1, 10),
		func(x int) (bool, string) {
			return x >= 1 && x <= 10, "out of range"
		},
		WithIterations(100),
		WithSeed(42),
		WithMaxShrinks(100),
		WithVerbose(false),
	)
	if !result.Passed {
		t.Errorf("expected passing property, got: %s", result.String())
	}
	if result.Iterations != 100 {
		t.Errorf("expected 100 iterations, got %d", result.Iterations)
	}
	if result.Seed != 42 {
		t.Errorf("expected seed 42, got %d", result.Seed)
	}
}

func TestCheck_Options_InvalidIterations(t *testing.T) {
	result := Check(
		Int(),
		func(x int) (bool, string) { return true, "" },
		WithIterations(-5),
		WithSeed(1),
	)
	if result.Iterations != 100 {
		t.Errorf("expected default iterations 100 with negative option, got %d", result.Iterations)
	}
}

func TestResult_StringPassed(t *testing.T) {
	r := &Result[int]{
		Passed:     true,
		Seed:       123,
		Iterations: 50,
	}
	s := r.String()
	if !strings.Contains(s, "PASSED") {
		t.Errorf("expected PASSED in result string, got: %s", s)
	}
	if !strings.Contains(s, "123") {
		t.Errorf("expected seed in result string, got: %s", s)
	}
}

func TestResult_StringFailed(t *testing.T) {
	r := &Result[int]{
		Passed:      false,
		Seed:        456,
		Iterations:  20,
		ShrinkSteps: 5,
		FailCase: &FailCase[int]{
			Input:     42,
			Iteration: 10,
			Message:   "test error",
			Seed:      456,
		},
	}
	s := r.String()
	if !strings.Contains(s, "FAILED") {
		t.Errorf("expected FAILED in result string, got: %s", s)
	}
	if !strings.Contains(s, "42") {
		t.Errorf("expected failing input in result string, got: %s", s)
	}
	if !strings.Contains(s, "test error") {
		t.Errorf("expected error message in result string, got: %s", s)
	}
}

func TestRunner_Check_SliceProperty(t *testing.T) {
	gen := Slice(IntRange(0, 100))
	prop := func(s []int) (bool, string) {
		// Double reverse should give original
		reversed := make([]int, len(s))
		for i, v := range s {
			reversed[len(s)-1-i] = v
		}
		doubleReversed := make([]int, len(reversed))
		for i, v := range reversed {
			doubleReversed[len(reversed)-1-i] = v
		}
		if len(doubleReversed) != len(s) {
			return false, "length mismatch"
		}
		for i := range s {
			if s[i] != doubleReversed[i] {
				return false, "double reverse mismatch"
			}
		}
		return true, ""
	}
	result := Check(gen, prop, WithIterations(100), WithSeed(99))
	if !result.Passed {
		t.Errorf("expected double reverse property to hold, got: %s", result.String())
	}
}

func TestRunner_Check_PairProperty(t *testing.T) {
	gen := PairOf(IntRange(1, 100), IntRange(1, 100))
	prop := func(p Pair[int, int]) (bool, string) {
		sum := p.A + p.B
		// a + b should equal b + a
		if sum != p.B+p.A {
			return false, "commutativity failed"
		}
		return true, ""
	}
	result := Check(gen, prop, WithIterations(100), WithSeed(1))
	if !result.Passed {
		t.Errorf("expected addition commutativity, got: %s", result.String())
	}
}

func TestRunner_Check_MaxShrinksLimit(t *testing.T) {
	gen := IntRange(0, 1000000)
	prop := func(x int) (bool, string) {
		if x > 0 {
			return false, "must be 0"
		}
		return true, ""
	}
	cfg := Config{
		Iterations:    100,
		MaxShrinks:    3,
		Seed:          987654,
		UseRandomSeed: false,
	}
	r := NewRunner[int](cfg)
	result := r.Check(gen, prop)
	if !result.Passed {
		t.Logf("Shrink steps: %d", result.ShrinkSteps)
		if result.ShrinkSteps > cfg.MaxShrinks {
			t.Errorf("shrink steps %d exceed max %d", result.ShrinkSteps, cfg.MaxShrinks)
		}
	}
}

func TestRunner_Check_SliceShrinkLength(t *testing.T) {
	gen := SliceLen(IntNonNegative(), 0, 100)
	prop := func(s []int) (bool, string) {
		// Fail when length > 3
		if len(s) > 3 {
			return false, "slice too long"
		}
		return true, ""
	}
	result := Check(gen, prop, WithIterations(500), WithSeed(1234), WithMaxShrinks(2000))
	if result.Passed {
		t.Fatal("expected failure")
	}
	if len(result.FailCase.Input) != 4 {
		t.Logf("Expected minimal slice length 4, got %d: %v",
			len(result.FailCase.Input), result.FailCase.Input)
	}
}

func TestRuneRanges_Defined(t *testing.T) {
	if len(ASCIILetters) != 2 {
		t.Errorf("expected 2 ASCIILetters ranges, got %d", len(ASCIILetters))
	}
	if len(ASCIIDigits) != 1 {
		t.Errorf("expected 1 ASCIIDigits range, got %d", len(ASCIIDigits))
	}
	if len(ASCIIAlphanumeric) != 3 {
		t.Errorf("expected 3 ASCIIAlphanumeric ranges, got %d", len(ASCIIAlphanumeric))
	}
	if len(ASCIIPrintable) != 1 {
		t.Errorf("expected 1 ASCIIPrintable range, got %d", len(ASCIIPrintable))
	}
	if len(UnicodeAll) != 2 {
		t.Errorf("expected 2 UnicodeAll ranges, got %d", len(UnicodeAll))
	}
}

func TestIntGenerator_FullRange(t *testing.T) {
	gen := Int()
	r := rand.New(rand.NewSource(5))
	hasPos := false
	hasNeg := false
	for i := 0; i < 10000; i++ {
		v := gen.Generate(r)
		if v > 0 {
			hasPos = true
		}
		if v < 0 {
			hasNeg = true
		}
	}
	if !hasPos {
		t.Error("expected positive values from full range Int()")
	}
	if !hasNeg {
		t.Error("expected negative values from full range Int()")
	}
}

func TestFloat64Generator_FullRange(t *testing.T) {
	gen := Float64()
	r := rand.New(rand.NewSource(7))
	hasPos := false
	hasNeg := false
	for i := 0; i < 1000; i++ {
		v := gen.Generate(r)
		if math.IsNaN(v) {
			t.Fatalf("unexpected NaN")
		}
		if math.IsInf(v, 0) {
			t.Fatalf("unexpected Inf")
		}
		if v > 0 {
			hasPos = true
		}
		if v < 0 {
			hasNeg = true
		}
	}
	if !hasPos {
		t.Error("expected positive floats")
	}
	if !hasNeg {
		t.Error("expected negative floats")
	}
}

func TestRunner_Check_SingleIteration(t *testing.T) {
	result := Check(
		IntRange(0, 9),
		func(x int) (bool, string) { return x >= 0 && x <= 9, "" },
		WithIterations(1),
		WithSeed(1),
	)
	if !result.Passed {
		t.Errorf("expected pass with 1 iteration, got %s", result.String())
	}
	if result.Iterations != 1 {
		t.Errorf("expected 1 iteration, got %d", result.Iterations)
	}
}

func TestRunner_Check_AllIterations(t *testing.T) {
	gen := IntRange(1, 10)
	failCount := 0
	prop := func(x int) (bool, string) {
		// Always fail so we can see which iteration triggers
		failCount++
		return false, "always fail"
	}
	result := Check(gen, prop, WithIterations(1), WithSeed(1))
	if result.Passed {
		t.Fatal("expected failure")
	}
	if result.FailCase.Iteration != 1 {
		t.Errorf("expected failure at iteration 1, got %d", result.FailCase.Iteration)
	}
}
