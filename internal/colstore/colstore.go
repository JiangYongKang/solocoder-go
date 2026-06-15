package colstore

import (
	"errors"
	"fmt"
	"sync"
)

var (
	ErrEmptyBatch          = errors.New("empty batch: no columns provided")
	ErrColumnMismatch      = errors.New("column length mismatch: all columns must have the same number of rows")
	ErrDuplicateColumnName = errors.New("duplicate column name")
	ErrColumnNotFound      = errors.New("column not found")
	ErrEmptyColumnSet      = errors.New("empty column set for projection")
	ErrInvalidPredicate    = errors.New("invalid predicate")
	ErrStoreClosed         = errors.New("column store is closed")
	ErrInvalidOp           = errors.New("invalid predicate operator")
)

type Operator string

const (
	OpEq    Operator = "="
	OpNeq   Operator = "!="
	OpGt    Operator = ">"
	OpGte   Operator = ">="
	OpLt    Operator = "<"
	OpLte   Operator = "<="
	OpIn    Operator = "IN"
	OpNotIn Operator = "NOT IN"
)

type Value interface{}

type Predicate struct {
	Column   string
	Op       Operator
	Value    Value
	Values   []Value
}

type Column struct {
	name       string
	dict       map[Value]int
	reverseDict []Value
	data       []int
	mu         sync.RWMutex
}

func newColumn(name string) *Column {
	return &Column{
		name:        name,
		dict:        make(map[Value]int),
		reverseDict: make([]Value, 0),
		data:        make([]int, 0),
	}
}

func (c *Column) getName() string {
	return c.name
}

func (c *Column) encode(v Value) int {
	if idx, ok := c.dict[v]; ok {
		return idx
	}
	idx := len(c.reverseDict)
	c.dict[v] = idx
	c.reverseDict = append(c.reverseDict, v)
	return idx
}

func (c *Column) decode(idx int) Value {
	if idx < 0 || idx >= len(c.reverseDict) {
		return nil
	}
	return c.reverseDict[idx]
}

func (c *Column) appendValues(values []Value) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range values {
		c.data = append(c.data, c.encode(v))
	}
}

func (c *Column) length() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.data)
}

func (c *Column) getValueAt(rowIdx int) Value {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if rowIdx < 0 || rowIdx >= len(c.data) {
		return nil
	}
	return c.decode(c.data[rowIdx])
}

func (c *Column) getValuesAt(rows []int) []Value {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]Value, len(rows))
	for i, r := range rows {
		if r < 0 || r >= len(c.data) {
			result[i] = nil
		} else {
			result[i] = c.decode(c.data[r])
		}
	}
	return result
}

func (c *Column) dictionarySize() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.reverseDict)
}

type Row struct {
	Values map[string]Value
}

type QueryResult struct {
	Rows        []*Row
	Columns     []string
	TotalScanned int
	TotalMatched int
}

type ColumnStore struct {
	columns map[string]*Column
	colOrder []string
	rowCount int
	mu       sync.RWMutex
	closed   bool
}

type Config struct {
	DictionaryEnabled bool
}

func DefaultConfig() Config {
	return Config{
		DictionaryEnabled: true,
	}
}

func NewColumnStore() *ColumnStore {
	return NewColumnStoreWithConfig(DefaultConfig())
}

func NewColumnStoreWithConfig(cfg Config) *ColumnStore {
	return &ColumnStore{
		columns:  make(map[string]*Column),
		colOrder: make([]string, 0),
		rowCount: 0,
	}
}

type ColumnBatch struct {
	Name   string
	Values []Value
}

func (cs *ColumnStore) Write(batch []ColumnBatch) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if cs.closed {
		return ErrStoreClosed
	}

	if len(batch) == 0 {
		return ErrEmptyBatch
	}

	expectedRows := -1
	seenNames := make(map[string]bool)
	for i, cb := range batch {
		if seenNames[cb.Name] {
			return ErrDuplicateColumnName
		}
		seenNames[cb.Name] = true
		if expectedRows == -1 {
			expectedRows = len(cb.Values)
		} else if len(cb.Values) != expectedRows {
			return ErrColumnMismatch
		}
		_ = i
	}

	for _, cb := range batch {
		col, exists := cs.columns[cb.Name]
		if !exists {
			col = newColumn(cb.Name)
			cs.columns[cb.Name] = col
			cs.colOrder = append(cs.colOrder, cb.Name)
		}
	}

	if expectedRows == 0 {
		return nil
	}

	for _, cb := range batch {
		cs.columns[cb.Name].appendValues(cb.Values)
	}

	cs.rowCount += expectedRows
	return nil
}

func (cs *ColumnStore) Read(columns []string) (*QueryResult, error) {
	return cs.ReadWithFilter(columns, nil)
}

func (cs *ColumnStore) ReadWithFilter(columns []string, predicates []*Predicate) (*QueryResult, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	if cs.closed {
		return nil, ErrStoreClosed
	}

	if len(columns) == 0 {
		return nil, ErrEmptyColumnSet
	}

	for _, col := range columns {
		if _, ok := cs.columns[col]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrColumnNotFound, col)
		}
	}

	for _, p := range predicates {
		if _, ok := cs.columns[p.Column]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrColumnNotFound, p.Column)
		}
		if err := validatePredicate(p); err != nil {
			return nil, err
		}
	}

	totalRows := cs.rowCount
	matchedRows := make([]int, 0, totalRows)

	if len(predicates) == 0 {
		for i := 0; i < totalRows; i++ {
			matchedRows = append(matchedRows, i)
		}
	} else {
		for i := 0; i < totalRows; i++ {
			if cs.evaluatePredicates(i, predicates) {
				matchedRows = append(matchedRows, i)
			}
		}
	}

	resultRows := make([]*Row, len(matchedRows))
	for i, rowIdx := range matchedRows {
		row := &Row{Values: make(map[string]Value)}
		for _, colName := range columns {
			row.Values[colName] = cs.columns[colName].getValueAt(rowIdx)
		}
		resultRows[i] = row
	}

	return &QueryResult{
		Rows:         resultRows,
		Columns:      columns,
		TotalScanned: totalRows,
		TotalMatched: len(matchedRows),
	}, nil
}

func validatePredicate(p *Predicate) error {
	switch p.Op {
	case OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte:
		if p.Value == nil {
			return ErrInvalidPredicate
		}
	case OpIn, OpNotIn:
		if len(p.Values) == 0 {
			return ErrInvalidPredicate
		}
	default:
		return ErrInvalidOp
	}
	return nil
}

func (cs *ColumnStore) evaluatePredicates(rowIdx int, predicates []*Predicate) bool {
	for _, p := range predicates {
		col := cs.columns[p.Column]
		val := col.getValueAt(rowIdx)
		if !evaluateSinglePredicate(val, p) {
			return false
		}
	}
	return true
}

func evaluateSinglePredicate(val Value, p *Predicate) bool {
	switch p.Op {
	case OpEq:
		return compareValues(val, p.Value) == 0
	case OpNeq:
		return compareValues(val, p.Value) != 0
	case OpGt:
		return compareValues(val, p.Value) > 0
	case OpGte:
		return compareValues(val, p.Value) >= 0
	case OpLt:
		return compareValues(val, p.Value) < 0
	case OpLte:
		return compareValues(val, p.Value) <= 0
	case OpIn:
		for _, v := range p.Values {
			if compareValues(val, v) == 0 {
				return true
			}
		}
		return false
	case OpNotIn:
		for _, v := range p.Values {
			if compareValues(val, v) == 0 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func compareValues(a, b Value) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch av := a.(type) {
	case int:
		switch bv := b.(type) {
		case int:
			if av < bv {
				return -1
			} else if av > bv {
				return 1
			}
			return 0
		case float64:
			af := float64(av)
			if af < bv {
				return -1
			} else if af > bv {
				return 1
			}
			return 0
		}
	case float64:
		switch bv := b.(type) {
		case float64:
			if av < bv {
				return -1
			} else if av > bv {
				return 1
			}
			return 0
		case int:
			bf := float64(bv)
			if av < bf {
				return -1
			} else if av > bf {
				return 1
			}
			return 0
		}
	case string:
		if bv, ok := b.(string); ok {
			if av < bv {
				return -1
			} else if av > bv {
				return 1
			}
			return 0
		}
	case bool:
		if bv, ok := b.(bool); ok {
			if !av && bv {
				return -1
			} else if av && !bv {
				return 1
			}
			return 0
		}
	}

	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	} else if as > bs {
		return 1
	}
	return 0
}

func (cs *ColumnStore) Close() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.closed = true
}

func (cs *ColumnStore) RowCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.rowCount
}

func (cs *ColumnStore) ColumnCount() int {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return len(cs.columns)
}

func (cs *ColumnStore) ColumnNames() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	result := make([]string, len(cs.colOrder))
	copy(result, cs.colOrder)
	return result
}

func (cs *ColumnStore) DictionarySize(column string) (int, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	col, ok := cs.columns[column]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrColumnNotFound, column)
	}
	return col.dictionarySize(), nil
}
