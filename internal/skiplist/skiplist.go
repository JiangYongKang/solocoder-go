package skiplist

import (
	"cmp"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	DefaultMaxLevel = 32
	DefaultP        = 0.25
)

type Config struct {
	MaxLevel     int
	P            float64
	RandomSource *rand.Rand
}

func DefaultConfig() *Config {
	return &Config{
		MaxLevel: DefaultMaxLevel,
		P:        DefaultP,
	}
}

func (c *Config) validate() error {
	if c.MaxLevel <= 0 {
		return fmt.Errorf("skiplist: max level must be positive, got %d", c.MaxLevel)
	}
	if c.P <= 0 || c.P >= 1 {
		return fmt.Errorf("skiplist: probability must be in (0, 1), got %f", c.P)
	}
	return nil
}

type node[K cmp.Ordered, V any] struct {
	key     K
	value   V
	forward []*node[K, V]
}

type SkipList[K cmp.Ordered, V any] struct {
	mu       sync.RWMutex
	header   *node[K, V]
	tail     *node[K, V]
	level    int
	length   int
	maxLevel int
	p        float64
	random   *rand.Rand
}

type Pair[K cmp.Ordered, V any] struct {
	Key   K
	Value V
}

type RangeOptions struct {
	StartInclusive bool
	EndInclusive   bool
	Limit          int
	Offset         int
}

func DefaultRangeOptions() *RangeOptions {
	return &RangeOptions{
		StartInclusive: true,
		EndInclusive:   true,
		Limit:          0,
		Offset:         0,
	}
}

func (o *RangeOptions) WithStartInclusive(v bool) *RangeOptions {
	o.StartInclusive = v
	return o
}

func (o *RangeOptions) WithEndInclusive(v bool) *RangeOptions {
	o.EndInclusive = v
	return o
}

func (o *RangeOptions) WithLimit(v int) *RangeOptions {
	o.Limit = v
	return o
}

func (o *RangeOptions) WithOffset(v int) *RangeOptions {
	o.Offset = v
	return o
}

func New[K cmp.Ordered, V any](configs ...*Config) (*SkipList[K, V], error) {
	cfg := DefaultConfig()
	if len(configs) > 0 && configs[0] != nil {
		cfg = configs[0]
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var zeroK K
	var zeroV V
	header := &node[K, V]{
		key:     zeroK,
		value:   zeroV,
		forward: make([]*node[K, V], cfg.MaxLevel),
	}

	rng := cfg.RandomSource
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return &SkipList[K, V]{
		header:   header,
		tail:     nil,
		level:    1,
		length:   0,
		maxLevel: cfg.MaxLevel,
		p:        cfg.P,
		random:   rng,
	}, nil
}

func (sl *SkipList[K, V]) SetRandomSeed(seed int64) {
	sl.mu.Lock()
	defer sl.mu.Unlock()
	sl.random = rand.New(rand.NewSource(seed))
}

func (sl *SkipList[K, V]) randomLevel() int {
	level := 1
	for level < sl.maxLevel && sl.random.Float64() < sl.p {
		level++
	}
	return level
}

func (sl *SkipList[K, V]) Insert(key K, value V) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	update := make([]*node[K, V], sl.maxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].key < key {
			x = x.forward[i]
		}
		update[i] = x
	}

	x = x.forward[0]
	if x != nil && x.key == key {
		x.value = value
		return
	}

	lvl := sl.randomLevel()
	if lvl > sl.level {
		for i := sl.level; i < lvl; i++ {
			update[i] = sl.header
		}
		sl.level = lvl
	}

	newNode := &node[K, V]{
		key:     key,
		value:   value,
		forward: make([]*node[K, V], lvl),
	}

	for i := 0; i < lvl; i++ {
		newNode.forward[i] = update[i].forward[i]
		update[i].forward[i] = newNode
	}

	if newNode.forward[0] == nil {
		sl.tail = newNode
	}

	sl.length++
}

func (sl *SkipList[K, V]) Delete(key K) (V, bool) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var zeroV V
	update := make([]*node[K, V], sl.maxLevel)
	x := sl.header

	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].key < key {
			x = x.forward[i]
		}
		update[i] = x
	}

	x = x.forward[0]
	if x == nil || x.key != key {
		return zeroV, false
	}

	for i := 0; i < sl.level; i++ {
		if update[i].forward[i] != x {
			break
		}
		update[i].forward[i] = x.forward[i]
	}

	for sl.level > 1 && sl.header.forward[sl.level-1] == nil {
		sl.level--
	}

	if sl.tail == x {
		sl.tail = update[0]
		if sl.tail == sl.header {
			sl.tail = nil
		}
	}

	sl.length--
	return x.value, true
}

func (sl *SkipList[K, V]) Search(key K) (V, bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	var zeroV V
	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].key < key {
			x = x.forward[i]
		}
	}
	x = x.forward[0]
	if x != nil && x.key == key {
		return x.value, true
	}
	return zeroV, false
}

func (sl *SkipList[K, V]) Range(start, end K, opts ...*RangeOptions) []Pair[K, V] {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	options := DefaultRangeOptions()
	if len(opts) > 0 && opts[0] != nil {
		userOpts := opts[0]
		options.StartInclusive = userOpts.StartInclusive
		options.EndInclusive = userOpts.EndInclusive
		options.Limit = userOpts.Limit
		options.Offset = userOpts.Offset
	}

	if start > end {
		return []Pair[K, V]{}
	}

	result := make([]Pair[K, V], 0)

	x := sl.header
	for i := sl.level - 1; i >= 0; i-- {
		for x.forward[i] != nil && x.forward[i].key < start {
			x = x.forward[i]
		}
	}
	x = x.forward[0]

	skipped := 0
	for x != nil {
		if x.key > end {
			break
		}

		includeStart := true
		if x.key == start && !options.StartInclusive {
			includeStart = false
		}

		includeEnd := true
		if x.key == end && !options.EndInclusive {
			includeEnd = false
		}

		if includeStart && includeEnd {
			if skipped < options.Offset {
				skipped++
			} else {
				result = append(result, Pair[K, V]{Key: x.key, Value: x.value})
				if options.Limit > 0 && len(result) >= options.Limit {
					break
				}
			}
		}

		x = x.forward[0]
	}

	return result
}

func (sl *SkipList[K, V]) Len() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.length
}

func (sl *SkipList[K, V]) Level() int {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.level
}

func (sl *SkipList[K, V]) Contains(key K) bool {
	_, ok := sl.Search(key)
	return ok
}

func (sl *SkipList[K, V]) All() []Pair[K, V] {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	result := make([]Pair[K, V], 0, sl.length)
	x := sl.header.forward[0]
	for x != nil {
		result = append(result, Pair[K, V]{Key: x.key, Value: x.value})
		x = x.forward[0]
	}
	return result
}

func (sl *SkipList[K, V]) Clear() {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	var zeroK K
	var zeroV V
	sl.header = &node[K, V]{
		key:     zeroK,
		value:   zeroV,
		forward: make([]*node[K, V], sl.maxLevel),
	}
	sl.tail = nil
	sl.level = 1
	sl.length = 0
}

func (sl *SkipList[K, V]) First() (Pair[K, V], bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	var zero Pair[K, V]
	x := sl.header.forward[0]
	if x == nil {
		return zero, false
	}
	return Pair[K, V]{Key: x.key, Value: x.value}, true
}

func (sl *SkipList[K, V]) Last() (Pair[K, V], bool) {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	var zero Pair[K, V]
	if sl.tail == nil {
		return zero, false
	}
	return Pair[K, V]{Key: sl.tail.key, Value: sl.tail.value}, true
}
