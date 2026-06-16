package windowagg

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	ErrInvalidWindowSize   = errors.New("windowagg: invalid window size")
	ErrInvalidSlideSize    = errors.New("windowagg: invalid slide size")
	ErrSlideGreaterThanWindow = errors.New("windowagg: slide size cannot be greater than window size")
	ErrWindowNotFound      = errors.New("windowagg: window not found")
	ErrWindowExists        = errors.New("windowagg: window already exists")
	ErrInvalidWindowType   = errors.New("windowagg: invalid window type")
	ErrInvalidAggregator   = errors.New("windowagg: invalid aggregator")
	ErrEmptyWindow         = errors.New("windowagg: window is empty")
)

type WindowType int

const (
	WindowTypeCount WindowType = iota
	WindowTypeTime
)

type AggregatorType int

const (
	AggregatorCount AggregatorType = iota
	AggregatorSum
	AggregatorAvg
	AggregatorMax
	AggregatorMin
)

func (a AggregatorType) String() string {
	switch a {
	case AggregatorCount:
		return "count"
	case AggregatorSum:
		return "sum"
	case AggregatorAvg:
		return "avg"
	case AggregatorMax:
		return "max"
	case AggregatorMin:
		return "min"
	default:
		return "unknown"
	}
}

type Aggregator interface {
	Add(value float64)
	Remove(value float64)
	Result() (float64, error)
	Reset()
	Name() string
}

type CountAggregator struct {
	count int64
}

func NewCountAggregator() *CountAggregator {
	return &CountAggregator{}
}

func (a *CountAggregator) Add(value float64) {
	a.count++
}

func (a *CountAggregator) Remove(value float64) {
	if a.count > 0 {
		a.count--
	}
}

func (a *CountAggregator) Result() (float64, error) {
	return float64(a.count), nil
}

func (a *CountAggregator) Reset() {
	a.count = 0
}

func (a *CountAggregator) Name() string {
	return "count"
}

type SumAggregator struct {
	sum float64
}

func NewSumAggregator() *SumAggregator {
	return &SumAggregator{}
}

func (a *SumAggregator) Add(value float64) {
	a.sum += value
}

func (a *SumAggregator) Remove(value float64) {
	a.sum -= value
}

func (a *SumAggregator) Result() (float64, error) {
	return a.sum, nil
}

func (a *SumAggregator) Reset() {
	a.sum = 0
}

func (a *SumAggregator) Name() string {
	return "sum"
}

type AvgAggregator struct {
	sum   float64
	count int64
}

func NewAvgAggregator() *AvgAggregator {
	return &AvgAggregator{}
}

func (a *AvgAggregator) Add(value float64) {
	a.sum += value
	a.count++
}

func (a *AvgAggregator) Remove(value float64) {
	a.sum -= value
	if a.count > 0 {
		a.count--
	}
}

func (a *AvgAggregator) Result() (float64, error) {
	if a.count == 0 {
		return 0, ErrEmptyWindow
	}
	return a.sum / float64(a.count), nil
}

func (a *AvgAggregator) Reset() {
	a.sum = 0
	a.count = 0
}

func (a *AvgAggregator) Name() string {
	return "avg"
}

type MaxAggregator struct {
	values   *list.List
	maxValue float64
	hasValue bool
}

func NewMaxAggregator() *MaxAggregator {
	return &MaxAggregator{
		values:   list.New(),
		maxValue: math.Inf(-1),
	}
}

func (a *MaxAggregator) Add(value float64) {
	a.values.PushBack(value)
	if !a.hasValue || value > a.maxValue {
		a.maxValue = value
		a.hasValue = true
	}
}

func (a *MaxAggregator) Remove(value float64) {
	for e := a.values.Front(); e != nil; e = e.Next() {
		if e.Value.(float64) == value {
			a.values.Remove(e)
			break
		}
	}
	if a.values.Len() == 0 {
		a.hasValue = false
		a.maxValue = math.Inf(-1)
	} else if value == a.maxValue {
		a.maxValue = math.Inf(-1)
		for e := a.values.Front(); e != nil; e = e.Next() {
			v := e.Value.(float64)
			if v > a.maxValue {
				a.maxValue = v
			}
		}
		a.hasValue = true
	}
}

func (a *MaxAggregator) Result() (float64, error) {
	if !a.hasValue {
		return 0, ErrEmptyWindow
	}
	return a.maxValue, nil
}

func (a *MaxAggregator) Reset() {
	a.values.Init()
	a.maxValue = math.Inf(-1)
	a.hasValue = false
}

func (a *MaxAggregator) Name() string {
	return "max"
}

type MinAggregator struct {
	values   *list.List
	minValue float64
	hasValue bool
}

func NewMinAggregator() *MinAggregator {
	return &MinAggregator{
		values:   list.New(),
		minValue: math.Inf(1),
	}
}

func (a *MinAggregator) Add(value float64) {
	a.values.PushBack(value)
	if !a.hasValue || value < a.minValue {
		a.minValue = value
		a.hasValue = true
	}
}

func (a *MinAggregator) Remove(value float64) {
	for e := a.values.Front(); e != nil; e = e.Next() {
		if e.Value.(float64) == value {
			a.values.Remove(e)
			break
		}
	}
	if a.values.Len() == 0 {
		a.hasValue = false
		a.minValue = math.Inf(1)
	} else if value == a.minValue {
		a.minValue = math.Inf(1)
		for e := a.values.Front(); e != nil; e = e.Next() {
			v := e.Value.(float64)
			if v < a.minValue {
				a.minValue = v
			}
		}
		a.hasValue = true
	}
}

func (a *MinAggregator) Result() (float64, error) {
	if !a.hasValue {
		return 0, ErrEmptyWindow
	}
	return a.minValue, nil
}

func (a *MinAggregator) Reset() {
	a.values.Init()
	a.minValue = math.Inf(1)
	a.hasValue = false
}

func (a *MinAggregator) Name() string {
	return "min"
}

func NewAggregator(aggType AggregatorType) (Aggregator, error) {
	switch aggType {
	case AggregatorCount:
		return NewCountAggregator(), nil
	case AggregatorSum:
		return NewSumAggregator(), nil
	case AggregatorAvg:
		return NewAvgAggregator(), nil
	case AggregatorMax:
		return NewMaxAggregator(), nil
	case AggregatorMin:
		return NewMinAggregator(), nil
	default:
		return nil, ErrInvalidAggregator
	}
}

type WindowConfig struct {
	WindowType     WindowType
	AggregatorType AggregatorType
	Size           int64
	Slide          int64
}

type windowItem struct {
	value     float64
	timestamp time.Time
	seq       int64
}

type SlidingWindow struct {
	mu           sync.RWMutex
	name         string
	config       WindowConfig
	aggregator   Aggregator
	items        *list.List
	currentStart int64
	seqCounter   int64
}

func NewSlidingWindow(name string, cfg WindowConfig) (*SlidingWindow, error) {
	if cfg.WindowType != WindowTypeCount && cfg.WindowType != WindowTypeTime {
		return nil, ErrInvalidWindowType
	}
	if cfg.Size <= 0 {
		return nil, ErrInvalidWindowSize
	}
	if cfg.Slide <= 0 {
		return nil, ErrInvalidSlideSize
	}
	if cfg.Slide > cfg.Size {
		return nil, ErrSlideGreaterThanWindow
	}

	agg, err := NewAggregator(cfg.AggregatorType)
	if err != nil {
		return nil, err
	}

	return &SlidingWindow{
		name:       name,
		config:     cfg,
		aggregator: agg,
		items:      list.New(),
	}, nil
}

func (w *SlidingWindow) Name() string {
	return w.name
}

func (w *SlidingWindow) Config() WindowConfig {
	return w.config
}

func (w *SlidingWindow) AddValue(value float64, timestamp time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.seqCounter++
	item := &windowItem{
		value:     value,
		timestamp: timestamp,
		seq:       w.seqCounter,
	}

	w.items.PushBack(item)
	w.aggregator.Add(value)

	w.evictLocked(timestamp)
}

func (w *SlidingWindow) AddValueWithSeq(value float64, timestamp time.Time, seq int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if seq > w.seqCounter {
		w.seqCounter = seq
	}
	item := &windowItem{
		value:     value,
		timestamp: timestamp,
		seq:       seq,
	}

	w.items.PushBack(item)
	w.aggregator.Add(value)

	w.evictLocked(timestamp)
}

func (w *SlidingWindow) evictLocked(currentTime time.Time) {
	if w.config.WindowType == WindowTypeCount {
		slide := w.config.Slide
		size := w.config.Size
		seq := w.seqCounter

		windowStart := ((seq - 1) / slide) * slide + slide - size + 1
		if windowStart < 1 {
			windowStart = 1
		}

		for w.items.Len() > 0 {
			front := w.items.Front()
			item := front.Value.(*windowItem)
			if item.seq < windowStart {
				w.aggregator.Remove(item.value)
				w.items.Remove(front)
			} else {
				break
			}
		}
	} else {
		sizeMs := w.config.Size
		cutoff := currentTime.Add(-time.Duration(sizeMs) * time.Millisecond)

		for w.items.Len() > 0 {
			front := w.items.Front()
			item := front.Value.(*windowItem)
			if item.timestamp.Before(cutoff) {
				w.aggregator.Remove(item.value)
				w.items.Remove(front)
			} else {
				break
			}
		}
	}
}

func (w *SlidingWindow) Result() (float64, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.aggregator.Result()
}

func (w *SlidingWindow) Count() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.items.Len()
}

func (w *SlidingWindow) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.items.Init()
	w.aggregator.Reset()
	w.currentStart = 0
	w.seqCounter = 0
}

type WindowManager struct {
	mu      sync.RWMutex
	windows map[string]*SlidingWindow
}

func NewWindowManager() *WindowManager {
	return &WindowManager{
		windows: make(map[string]*SlidingWindow),
	}
}

func (m *WindowManager) AddWindow(name string, cfg WindowConfig) error {
	if name == "" {
		return ErrInvalidWindowType
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.windows[name]; exists {
		return ErrWindowExists
	}

	w, err := NewSlidingWindow(name, cfg)
	if err != nil {
		return err
	}

	m.windows[name] = w
	return nil
}

func (m *WindowManager) RemoveWindow(name string) error {
	if name == "" {
		return ErrInvalidWindowType
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.windows[name]; !exists {
		return ErrWindowNotFound
	}

	delete(m.windows, name)
	return nil
}

func (m *WindowManager) GetWindow(name string) (*SlidingWindow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w, exists := m.windows[name]
	if !exists {
		return nil, ErrWindowNotFound
	}
	return w, nil
}

func (m *WindowManager) WindowCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.windows)
}

func (m *WindowManager) Push(value float64, timestamp time.Time) {
	m.mu.RLock()
	windows := make([]*SlidingWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	m.mu.RUnlock()

	for _, w := range windows {
		w.AddValue(value, timestamp)
	}
}

func (m *WindowManager) PushWithSeq(value float64, timestamp time.Time, seq int64) {
	m.mu.RLock()
	windows := make([]*SlidingWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	m.mu.RUnlock()

	for _, w := range windows {
		w.AddValueWithSeq(value, timestamp, seq)
	}
}

func (m *WindowManager) GetResult(name string) (float64, error) {
	w, err := m.GetWindow(name)
	if err != nil {
		return 0, err
	}
	return w.Result()
}

func (m *WindowManager) GetAllResults() map[string]float64 {
	m.mu.RLock()
	windows := make([]*SlidingWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	m.mu.RUnlock()

	results := make(map[string]float64, len(windows))
	for _, w := range windows {
		if val, err := w.Result(); err == nil {
			results[w.Name()] = val
		}
	}
	return results
}

func (m *WindowManager) Reset() {
	m.mu.RLock()
	windows := make([]*SlidingWindow, 0, len(m.windows))
	for _, w := range m.windows {
		windows = append(windows, w)
	}
	m.mu.RUnlock()

	for _, w := range windows {
		w.Reset()
	}
}

func (m *WindowManager) ResetWindow(name string) error {
	w, err := m.GetWindow(name)
	if err != nil {
		return err
	}
	w.Reset()
	return nil
}

func validateConfig(cfg WindowConfig) error {
	if cfg.WindowType != WindowTypeCount && cfg.WindowType != WindowTypeTime {
		return fmt.Errorf("%w: %d", ErrInvalidWindowType, cfg.WindowType)
	}
	if cfg.Size <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidWindowSize, cfg.Size)
	}
	if cfg.Slide <= 0 {
		return fmt.Errorf("%w: %d", ErrInvalidSlideSize, cfg.Slide)
	}
	if cfg.Slide > cfg.Size {
		return ErrSlideGreaterThanWindow
	}
	return nil
}
