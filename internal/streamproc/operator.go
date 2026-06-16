package streamproc

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

type FilterFunc func(ctx context.Context, record *Record) (bool, error)

type FilterOperator struct {
	processed int64
	passed    int64
	dropped   int64

	name     string
	filterFn FilterFunc
	mu       sync.RWMutex
}

func NewFilterOperator(name string, fn FilterFunc) *FilterOperator {
	return &FilterOperator{
		name:     name,
		filterFn: fn,
	}
}

func (o *FilterOperator) Name() string {
	return o.name
}

func (o *FilterOperator) Process(ctx context.Context, input *Record) ([]*Record, error) {
	if o.filterFn == nil {
		atomic.AddInt64(&o.processed, 1)
		atomic.AddInt64(&o.passed, 1)
		return []*Record{input}, nil
	}

	atomic.AddInt64(&o.processed, 1)
	pass, err := o.filterFn(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("filter operator '%s': %w", o.name, err)
	}
	if pass {
		atomic.AddInt64(&o.passed, 1)
		return []*Record{input}, nil
	}
	atomic.AddInt64(&o.dropped, 1)
	return nil, nil
}

func (o *FilterOperator) Stats() (processed, passed, dropped int64) {
	return atomic.LoadInt64(&o.processed),
		atomic.LoadInt64(&o.passed),
		atomic.LoadInt64(&o.dropped)
}

type filterOperatorState struct {
	Processed int64 `json:"processed"`
	Passed    int64 `json:"passed"`
	Dropped   int64 `json:"dropped"`
}

func (o *FilterOperator) SaveState() ([]byte, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	state := filterOperatorState{
		Processed: atomic.LoadInt64(&o.processed),
		Passed:    atomic.LoadInt64(&o.passed),
		Dropped:   atomic.LoadInt64(&o.dropped),
	}
	return json.Marshal(state)
}

func (o *FilterOperator) RestoreState(data []byte) error {
	var state filterOperatorState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("filter operator '%s' restore state: %w", o.name, err)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	atomic.StoreInt64(&o.processed, state.Processed)
	atomic.StoreInt64(&o.passed, state.Passed)
	atomic.StoreInt64(&o.dropped, state.Dropped)
	return nil
}

type MapFunc func(ctx context.Context, record *Record) (*Record, error)

type MapOperator struct {
	processed int64

	name  string
	mapFn MapFunc
	mu    sync.RWMutex
}

func NewMapOperator(name string, fn MapFunc) *MapOperator {
	return &MapOperator{
		name:  name,
		mapFn: fn,
	}
}

func (o *MapOperator) Name() string {
	return o.name
}

func (o *MapOperator) Process(ctx context.Context, input *Record) ([]*Record, error) {
	if o.mapFn == nil {
		atomic.AddInt64(&o.processed, 1)
		return []*Record{input}, nil
	}

	atomic.AddInt64(&o.processed, 1)
	result, err := o.mapFn(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("map operator '%s': %w", o.name, err)
	}
	if result == nil {
		return nil, nil
	}
	return []*Record{result}, nil
}

func (o *MapOperator) Stats() (processed int64) {
	return atomic.LoadInt64(&o.processed)
}

type mapOperatorState struct {
	Processed int64 `json:"processed"`
}

func (o *MapOperator) SaveState() ([]byte, error) {
	state := mapOperatorState{
		Processed: atomic.LoadInt64(&o.processed),
	}
	return json.Marshal(state)
}

func (o *MapOperator) RestoreState(data []byte) error {
	var state mapOperatorState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("map operator '%s' restore state: %w", o.name, err)
	}
	atomic.StoreInt64(&o.processed, state.Processed)
	return nil
}

type FlatMapFunc func(ctx context.Context, record *Record) ([]*Record, error)

type FlatMapOperator struct {
	processed int64
	outputCnt int64

	name      string
	flatMapFn FlatMapFunc
	mu        sync.RWMutex
}

func NewFlatMapOperator(name string, fn FlatMapFunc) *FlatMapOperator {
	return &FlatMapOperator{
		name:      name,
		flatMapFn: fn,
	}
}

func (o *FlatMapOperator) Name() string {
	return o.name
}

func (o *FlatMapOperator) Process(ctx context.Context, input *Record) ([]*Record, error) {
	if o.flatMapFn == nil {
		atomic.AddInt64(&o.processed, 1)
		atomic.AddInt64(&o.outputCnt, 1)
		return []*Record{input}, nil
	}

	atomic.AddInt64(&o.processed, 1)
	results, err := o.flatMapFn(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("flatMap operator '%s': %w", o.name, err)
	}
	if results == nil {
		return nil, nil
	}
	atomic.AddInt64(&o.outputCnt, int64(len(results)))
	return results, nil
}

func (o *FlatMapOperator) Stats() (processed, outputCnt int64) {
	return atomic.LoadInt64(&o.processed),
		atomic.LoadInt64(&o.outputCnt)
}

type flatMapOperatorState struct {
	Processed int64 `json:"processed"`
	OutputCnt int64 `json:"output_cnt"`
}

func (o *FlatMapOperator) SaveState() ([]byte, error) {
	state := flatMapOperatorState{
		Processed: atomic.LoadInt64(&o.processed),
		OutputCnt: atomic.LoadInt64(&o.outputCnt),
	}
	return json.Marshal(state)
}

func (o *FlatMapOperator) RestoreState(data []byte) error {
	var state flatMapOperatorState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("flatMap operator '%s' restore state: %w", o.name, err)
	}
	atomic.StoreInt64(&o.processed, state.Processed)
	atomic.StoreInt64(&o.outputCnt, state.OutputCnt)
	return nil
}

type OperatorChain struct {
	operators []Operator
	mu        sync.RWMutex
}

func NewOperatorChain() *OperatorChain {
	return &OperatorChain{
		operators: make([]Operator, 0),
	}
}

func (c *OperatorChain) Add(op Operator) error {
	if op == nil {
		return ErrOperatorNil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.operators = append(c.operators, op)
	return nil
}

func (c *OperatorChain) Insert(index int, op Operator) error {
	if op == nil {
		return ErrOperatorNil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index > len(c.operators) {
		return fmt.Errorf("streamproc: operator index out of range: %d", index)
	}
	c.operators = append(c.operators[:index], append([]Operator{op}, c.operators[index:]...)...)
	return nil
}

func (c *OperatorChain) Remove(index int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if index < 0 || index >= len(c.operators) {
		return fmt.Errorf("streamproc: operator index out of range: %d", index)
	}
	c.operators = append(c.operators[:index], c.operators[index+1:]...)
	return nil
}

func (c *OperatorChain) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.operators)
}

func (c *OperatorChain) List() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	names := make([]string, len(c.operators))
	for i, op := range c.operators {
		names[i] = op.Name()
	}
	return names
}

func (c *OperatorChain) Process(ctx context.Context, input *Record) ([]*Record, bool, error) {
	c.mu.RLock()
	operators := make([]Operator, len(c.operators))
	copy(operators, c.operators)
	c.mu.RUnlock()

	current := []*Record{input}
	filteredByFilter := false

	for i, op := range operators {
		nextBatch := make([]*Record, 0, len(current))
		for _, rec := range current {
			results, err := op.Process(ctx, rec)
			if err != nil {
				return nil, false, err
			}
			if results != nil {
				nextBatch = append(nextBatch, results...)
			}
		}
		current = nextBatch
		if len(current) == 0 {
			if _, ok := op.(*FilterOperator); ok {
				filteredByFilter = true
			} else {
				filteredByFilter = false
			}
			return nil, filteredByFilter, nil
		}
		_ = i
	}
	return current, false, nil
}

func (c *OperatorChain) SaveStates() (map[string][]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	states := make(map[string][]byte, len(c.operators))
	for _, op := range c.operators {
		data, err := op.SaveState()
		if err != nil {
			return nil, err
		}
		states[op.Name()] = data
	}
	return states, nil
}

func (c *OperatorChain) RestoreStates(states map[string][]byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, op := range c.operators {
		if data, ok := states[op.Name()]; ok {
			if err := op.RestoreState(data); err != nil {
				return err
			}
		}
	}
	return nil
}
