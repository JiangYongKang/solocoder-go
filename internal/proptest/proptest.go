package proptest

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

var (
	ErrPropertyFailed   = errors.New("proptest: property invariant failed")
	ErrInvalidConfig    = errors.New("proptest: invalid configuration")
	ErrShrinkLimit      = errors.New("proptest: shrink iteration limit reached")
	ErrGeneratorNil     = errors.New("proptest: generator is nil")
)

type Generator[T any] interface {
	Generate(r *rand.Rand) T
	Shrink(value T) []T
}

type Config struct {
	Iterations     int
	MaxShrinks     int
	Seed           int64
	UseRandomSeed  bool
	Verbose        bool
}

func DefaultConfig() Config {
	return Config{
		Iterations:    100,
		MaxShrinks:    1000,
		UseRandomSeed: true,
		Verbose:       false,
	}
}

type FailCase[T any] struct {
	Input       T
	Seed        int64
	Iteration   int
	Message     string
}

type Result[T any] struct {
	Passed      bool
	Seed        int64
	Iterations  int
	FailCase    *FailCase[T]
	ShrinkSteps int
}

func (r *Result[T]) String() string {
	if r.Passed {
		return fmt.Sprintf("proptest: PASSED (seed=%d, iterations=%d)", r.Seed, r.Iterations)
	}
	fc := r.FailCase
	return fmt.Sprintf(
		"proptest: FAILED (seed=%d, iteration=%d, shrinks=%d)\n  input: %v\n  reason: %s",
		r.Seed, fc.Iteration, r.ShrinkSteps, fc.Input, fc.Message,
	)
}

type IntGenerator struct {
	Min int
	Max int
}

func Int() *IntGenerator {
	return &IntGenerator{Min: math.MinInt, Max: math.MaxInt}
}

func IntRange(min, max int) *IntGenerator {
	if min > max {
		min, max = max, min
	}
	return &IntGenerator{Min: min, Max: max}
}

func IntNonNegative() *IntGenerator {
	return &IntGenerator{Min: 0, Max: math.MaxInt}
}

func IntPositive() *IntGenerator {
	return &IntGenerator{Min: 1, Max: math.MaxInt}
}

func (g *IntGenerator) Generate(r *rand.Rand) int {
	if g.Min == g.Max {
		return g.Min
	}
	min64 := int64(g.Min)
	max64 := int64(g.Max)
	span := max64 - min64 + 1
	if span <= int64(math.MaxInt) && span > 0 {
		return int(min64 + r.Int63n(span))
	}
	u := uint64(r.Int63())<<1 ^ uint64(r.Int63())
	var spanU uint64
	if span == math.MinInt64 {
		spanU = uint64(1) << 63
	} else {
		spanU = uint64(span)
	}
	offset := int64(u % spanU)
	return int(min64 + offset)
}

func (g *IntGenerator) Shrink(value int) []int {
	candidates := make([]int, 0, 32)
	if value == 0 {
		return candidates
	}
	if g.Min <= 0 && g.Max >= 0 {
		candidates = append(candidates, 0)
	}
	absVal := value
	if value < 0 {
		absVal = -value
		if g.Min <= -value && g.Max >= -value {
			candidates = append(candidates, -value)
		}
	}
	step := absVal / 2
	for step > 0 {
		smaller1 := value - step
		if smaller1 >= g.Min && smaller1 <= g.Max && smaller1 != value {
			candidates = append(candidates, smaller1)
		}
		if value < 0 {
			smaller2 := value + step
			if smaller2 >= g.Min && smaller2 <= g.Max && smaller2 != value {
				candidates = append(candidates, smaller2)
			}
		}
		step /= 2
	}
	if value-1 >= g.Min && value-1 != value {
		candidates = appendUnique(candidates, value-1)
	}
	if value < 0 && value+1 <= g.Max && value+1 != value {
		candidates = appendUnique(candidates, value+1)
	}
	return candidates
}

type Float64Generator struct {
	Min float64
	Max float64
}

func Float64() *Float64Generator {
	return &Float64Generator{Min: -math.MaxFloat64, Max: math.MaxFloat64}
}

func Float64Range(min, max float64) *Float64Generator {
	if min > max {
		min, max = max, min
	}
	return &Float64Generator{Min: min, Max: max}
}

func (g *Float64Generator) Generate(r *rand.Rand) float64 {
	if g.Min == g.Max {
		return g.Min
	}
	if g.Min == -math.MaxFloat64 && g.Max == math.MaxFloat64 {
		sign := 1.0
		if r.Intn(2) == 0 {
			sign = -1.0
		}
		return sign * r.Float64() * math.MaxFloat64
	}
	span := g.Max - g.Min
	if math.IsInf(span, 1) {
		sign := 1.0
		mid := 0.0
		if g.Min == -math.MaxFloat64 {
			sign = 1.0
			if r.Intn(2) == 0 {
				sign = -1.0
			}
			mid = 0.0
		} else {
			mid = g.Min
			sign = 1.0
		}
		return mid + sign*r.Float64()*(g.Max-mid)
	}
	return g.Min + r.Float64()*span
}

func (g *Float64Generator) Shrink(value float64) []float64 {
	candidates := make([]float64, 0, 32)
	if value == 0.0 {
		return candidates
	}
	if g.Min <= 0 && g.Max >= 0 {
		candidates = append(candidates, 0.0)
	}
	step := math.Abs(value) / 2.0
	for step > 1e-9 {
		smaller := value - math.Copysign(step, value)
		if smaller >= g.Min && smaller <= g.Max && smaller != value {
			candidates = append(candidates, smaller)
		}
		step /= 2.0
	}
	return candidates
}

type BoolGenerator struct{}

func Bool() *BoolGenerator {
	return &BoolGenerator{}
}

func (g *BoolGenerator) Generate(r *rand.Rand) bool {
	return r.Intn(2) == 1
}

func (g *BoolGenerator) Shrink(value bool) []bool {
	if value {
		return []bool{false}
	}
	return nil
}

type RuneRange struct {
	Start rune
	End   rune
}

var (
	ASCIILetters    = []RuneRange{{'a', 'z'}, {'A', 'Z'}}
	ASCIIDigits     = []RuneRange{{'0', '9'}}
	ASCIIAlphanumeric = append(append([]RuneRange{}, ASCIILetters...), ASCIIDigits...)
	ASCIIPrintable  = []RuneRange{{' ', '~'}}
	UnicodeAll      = []RuneRange{{0x20, 0x7E}, {0xA0, 0x10FFFF}}
)

type StringGenerator struct {
	MinLen    int
	MaxLen    int
	RuneRanges []RuneRange
}

func String() *StringGenerator {
	return &StringGenerator{
		MinLen:     0,
		MaxLen:     50,
		RuneRanges: ASCIIAlphanumeric,
	}
}

func StringLen(minLen, maxLen int) *StringGenerator {
	if minLen < 0 {
		minLen = 0
	}
	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}
	return &StringGenerator{
		MinLen:     minLen,
		MaxLen:     maxLen,
		RuneRanges: ASCIIAlphanumeric,
	}
}

func StringWithCharset(minLen, maxLen int, ranges []RuneRange) *StringGenerator {
	if minLen < 0 {
		minLen = 0
	}
	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}
	if len(ranges) == 0 {
		ranges = ASCIIAlphanumeric
	}
	return &StringGenerator{
		MinLen:     minLen,
		MaxLen:     maxLen,
		RuneRanges: ranges,
	}
}

func (g *StringGenerator) pickRune(r *rand.Rand) rune {
	total := 0
	for _, rr := range g.RuneRanges {
		total += int(rr.End - rr.Start + 1)
	}
	if total <= 0 {
		return 'a'
	}
	pick := r.Intn(total)
	for _, rr := range g.RuneRanges {
		span := int(rr.End - rr.Start + 1)
		if pick < span {
			return rr.Start + rune(pick)
		}
		pick -= span
	}
	return 'a'
}

func (g *StringGenerator) Generate(r *rand.Rand) string {
	length := g.MinLen
	if g.MaxLen > g.MinLen {
		length = g.MinLen + r.Intn(g.MaxLen-g.MinLen+1)
	}
	var sb strings.Builder
	sb.Grow(length)
	for i := 0; i < length; i++ {
		sb.WriteRune(g.pickRune(r))
	}
	return sb.String()
}

func (g *StringGenerator) Shrink(value string) []string {
	runes := []rune(value)
	n := len(runes)
	candidates := make([]string, 0, n+8)
	if n == 0 {
		return candidates
	}
	if g.MinLen <= 0 {
		candidates = append(candidates, "")
	}
	targetLens := make([]int, 0)
	if n-1 >= g.MinLen {
		targetLens = append(targetLens, n-1)
	}
	half := n / 2
	for half > g.MinLen {
		targetLens = append(targetLens, half)
		half /= 2
	}
	if g.MinLen > 0 && n > g.MinLen {
		targetLens = appendUniqueInt(targetLens, g.MinLen)
	}
	for _, tl := range targetLens {
		if tl >= 0 && tl < n {
			candidates = append(candidates, string(runes[:tl]))
			if n-tl >= tl {
				candidates = append(candidates, string(runes[n-tl:]))
			}
		}
	}
	for i := 0; i < n; i++ {
		if n-1 >= g.MinLen {
			shorter := make([]rune, 0, n-1)
			shorter = append(shorter, runes[:i]...)
			shorter = append(shorter, runes[i+1:]...)
			candidates = append(candidates, string(shorter))
		}
	}
	return candidates
}

type SliceGenerator[T any] struct {
	MinLen      int
	MaxLen      int
	ElemGen     Generator[T]
}

func Slice[T any](elemGen Generator[T]) *SliceGenerator[T] {
	if elemGen == nil {
		panic(ErrGeneratorNil)
	}
	return &SliceGenerator[T]{
		MinLen:  0,
		MaxLen:  30,
		ElemGen: elemGen,
	}
}

func SliceLen[T any](elemGen Generator[T], minLen, maxLen int) *SliceGenerator[T] {
	if elemGen == nil {
		panic(ErrGeneratorNil)
	}
	if minLen < 0 {
		minLen = 0
	}
	if minLen > maxLen {
		minLen, maxLen = maxLen, minLen
	}
	return &SliceGenerator[T]{
		MinLen:  minLen,
		MaxLen:  maxLen,
		ElemGen: elemGen,
	}
}

func (g *SliceGenerator[T]) Generate(r *rand.Rand) []T {
	length := g.MinLen
	if g.MaxLen > g.MinLen {
		length = g.MinLen + r.Intn(g.MaxLen-g.MinLen+1)
	}
	result := make([]T, 0, length)
	for i := 0; i < length; i++ {
		result = append(result, g.ElemGen.Generate(r))
	}
	return result
}

func (g *SliceGenerator[T]) Shrink(value []T) [][]T {
	n := len(value)
	candidates := make([][]T, 0, n*2+8)
	if n == 0 {
		return candidates
	}
	if g.MinLen <= 0 {
		candidates = append(candidates, []T{})
	}
	targetLens := make([]int, 0)
	if n-1 >= g.MinLen {
		targetLens = append(targetLens, n-1)
	}
	half := n / 2
	for half > g.MinLen {
		targetLens = append(targetLens, half)
		half /= 2
	}
	if g.MinLen > 0 && n > g.MinLen {
		targetLens = appendUniqueInt(targetLens, g.MinLen)
	}
	for _, tl := range targetLens {
		if tl >= 0 && tl < n {
			prefix := make([]T, tl)
			copy(prefix, value[:tl])
			candidates = append(candidates, prefix)
			suffix := make([]T, tl)
			copy(suffix, value[n-tl:])
			candidates = append(candidates, suffix)
		}
	}
	for i := 0; i < n; i++ {
		if n-1 >= g.MinLen {
			shorter := make([]T, 0, n-1)
			shorter = append(shorter, value[:i]...)
			shorter = append(shorter, value[i+1:]...)
			candidates = append(candidates, shorter)
		}
	}
	for i := 0; i < n; i++ {
		elemShrinks := g.ElemGen.Shrink(value[i])
		for _, es := range elemShrinks {
			modified := make([]T, n)
			copy(modified, value)
			modified[i] = es
			candidates = append(candidates, modified)
		}
	}
	return candidates
}

type Pair[A, B any] struct {
	A A
	B B
}

type PairGenerator[A, B any] struct {
	GenA Generator[A]
	GenB Generator[B]
}

func PairOf[A, B any](genA Generator[A], genB Generator[B]) *PairGenerator[A, B] {
	if genA == nil || genB == nil {
		panic(ErrGeneratorNil)
	}
	return &PairGenerator[A, B]{GenA: genA, GenB: genB}
}

func (g *PairGenerator[A, B]) Generate(r *rand.Rand) Pair[A, B] {
	return Pair[A, B]{
		A: g.GenA.Generate(r),
		B: g.GenB.Generate(r),
	}
}

func (g *PairGenerator[A, B]) Shrink(value Pair[A, B]) []Pair[A, B] {
	candidates := make([]Pair[A, B], 0)
	shrinksA := g.GenA.Shrink(value.A)
	for _, sa := range shrinksA {
		candidates = append(candidates, Pair[A, B]{A: sa, B: value.B})
	}
	shrinksB := g.GenB.Shrink(value.B)
	for _, sb := range shrinksB {
		candidates = append(candidates, Pair[A, B]{A: value.A, B: sb})
	}
	return candidates
}

type Tuple3[A, B, C any] struct {
	A A
	B B
	C C
}

type Tuple3Generator[A, B, C any] struct {
	GenA Generator[A]
	GenB Generator[B]
	GenC Generator[C]
}

func Tuple3Of[A, B, C any](genA Generator[A], genB Generator[B], genC Generator[C]) *Tuple3Generator[A, B, C] {
	if genA == nil || genB == nil || genC == nil {
		panic(ErrGeneratorNil)
	}
	return &Tuple3Generator[A, B, C]{GenA: genA, GenB: genB, GenC: genC}
}

func (g *Tuple3Generator[A, B, C]) Generate(r *rand.Rand) Tuple3[A, B, C] {
	return Tuple3[A, B, C]{
		A: g.GenA.Generate(r),
		B: g.GenB.Generate(r),
		C: g.GenC.Generate(r),
	}
}

func (g *Tuple3Generator[A, B, C]) Shrink(value Tuple3[A, B, C]) []Tuple3[A, B, C] {
	candidates := make([]Tuple3[A, B, C], 0)
	shrinksA := g.GenA.Shrink(value.A)
	for _, sa := range shrinksA {
		candidates = append(candidates, Tuple3[A, B, C]{A: sa, B: value.B, C: value.C})
	}
	shrinksB := g.GenB.Shrink(value.B)
	for _, sb := range shrinksB {
		candidates = append(candidates, Tuple3[A, B, C]{A: value.A, B: sb, C: value.C})
	}
	shrinksC := g.GenC.Shrink(value.C)
	for _, sc := range shrinksC {
		candidates = append(candidates, Tuple3[A, B, C]{A: value.A, B: value.B, C: sc})
	}
	return candidates
}

type ConstGenerator[T any] struct {
	Value T
}

func Const[T any](value T) *ConstGenerator[T] {
	return &ConstGenerator[T]{Value: value}
}

func (g *ConstGenerator[T]) Generate(r *rand.Rand) T {
	return g.Value
}

func (g *ConstGenerator[T]) Shrink(value T) []T {
	return nil
}

type MapGenerator[T, U any] struct {
	Source   Generator[T]
	MapFn    func(T) U
	ShrinkFn func(U) []U
}

func Map[T, U any](src Generator[T], mapFn func(T) U) *MapGenerator[T, U] {
	if src == nil || mapFn == nil {
		panic(ErrGeneratorNil)
	}
	return &MapGenerator[T, U]{
		Source: src,
		MapFn:  mapFn,
	}
}

func MapWithShrink[T, U any](src Generator[T], mapFn func(T) U, shrinkFn func(U) []U) *MapGenerator[T, U] {
	if src == nil || mapFn == nil {
		panic(ErrGeneratorNil)
	}
	return &MapGenerator[T, U]{
		Source:   src,
		MapFn:    mapFn,
		ShrinkFn: shrinkFn,
	}
}

func (g *MapGenerator[T, U]) Generate(r *rand.Rand) U {
	return g.MapFn(g.Source.Generate(r))
}

func (g *MapGenerator[T, U]) Shrink(value U) []U {
	if g.ShrinkFn != nil {
		return g.ShrinkFn(value)
	}
	return nil
}

type PropertyFunc[T any] func(input T) (bool, string)

type Runner[T any] struct {
	cfg Config
}

func NewRunner[T any](cfg Config) *Runner[T] {
	if cfg.Iterations <= 0 {
		cfg.Iterations = DefaultConfig().Iterations
	}
	if cfg.MaxShrinks <= 0 {
		cfg.MaxShrinks = DefaultConfig().MaxShrinks
	}
	if cfg.UseRandomSeed && cfg.Seed == 0 {
		cfg.Seed = time.Now().UnixNano()
	}
	return &Runner[T]{cfg: cfg}
}

func (r *Runner[T]) Config() Config {
	return r.cfg
}

func (r *Runner[T]) Check(gen Generator[T], prop PropertyFunc[T]) *Result[T] {
	if gen == nil {
		return &Result[T]{
			Passed:     false,
			Seed:       r.cfg.Seed,
			Iterations: 0,
			FailCase: &FailCase[T]{
				Message: ErrGeneratorNil.Error(),
			},
		}
	}
	if prop == nil {
		return &Result[T]{
			Passed:     false,
			Seed:       r.cfg.Seed,
			Iterations: 0,
			FailCase: &FailCase[T]{
				Message: "proptest: property function is nil",
			},
		}
	}

	source := rand.New(rand.NewSource(r.cfg.Seed))
	result := &Result[T]{
		Seed:       r.cfg.Seed,
		Iterations: 0,
	}

	var failInput T
	var failIter int
	var failMsg string
	foundFail := false

	for i := 0; i < r.cfg.Iterations; i++ {
		input := gen.Generate(source)
		result.Iterations = i + 1
		ok, msg := prop(input)
		if !ok {
			failInput = input
			failIter = i + 1
			if msg == "" {
				msg = "property returned false"
			}
			failMsg = msg
			foundFail = true
			break
		}
	}

	if !foundFail {
		result.Passed = true
		return result
	}

	result.Passed = false
	shrunkenInput, shrinkSteps := shrinkValue(gen, prop, failInput, r.cfg.MaxShrinks)
	result.ShrinkSteps = shrinkSteps
	result.FailCase = &FailCase[T]{
		Input:     shrunkenInput,
		Seed:      r.cfg.Seed,
		Iteration: failIter,
		Message:   failMsg,
	}
	return result
}

func shrinkValue[T any](gen Generator[T], prop PropertyFunc[T], initial T, maxShrinks int) (T, int) {
	current := initial
	steps := 0
	visited := make(map[string]struct{})

	for steps < maxShrinks {
		candidates := gen.Shrink(current)
		if len(candidates) == 0 {
			break
		}

		improved := false
		for _, cand := range candidates {
			key := fmt.Sprintf("%v", cand)
			if _, ok := visited[key]; ok {
				continue
			}
			visited[key] = struct{}{}

			steps++
			ok, _ := prop(cand)
			if !ok {
				current = cand
				improved = true
				break
			}

			if steps >= maxShrinks {
				break
			}
		}

		if !improved {
			break
		}
	}

	return current, steps
}

func Check[T any](gen Generator[T], prop PropertyFunc[T], opts ...Option) *Result[T] {
	cfg := DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	runner := NewRunner[T](cfg)
	return runner.Check(gen, prop)
}

type Option func(*Config)

func WithIterations(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.Iterations = n
		}
	}
}

func WithMaxShrinks(n int) Option {
	return func(c *Config) {
		if n > 0 {
			c.MaxShrinks = n
		}
	}
}

func WithSeed(seed int64) Option {
	return func(c *Config) {
		c.Seed = seed
		c.UseRandomSeed = false
	}
}

func WithVerbose(v bool) Option {
	return func(c *Config) {
		c.Verbose = v
	}
}

func appendUnique(slice []int, val int) []int {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

func appendUniqueInt(slice []int, val int) []int {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}
