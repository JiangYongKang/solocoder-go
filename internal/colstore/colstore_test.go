package colstore

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewColumnStore(t *testing.T) {
	cs := NewColumnStore()
	if cs == nil {
		t.Fatal("NewColumnStore returned nil")
	}
	if cs.RowCount() != 0 {
		t.Errorf("expected initial row count 0, got %d", cs.RowCount())
	}
	if cs.ColumnCount() != 0 {
		t.Errorf("expected initial column count 0, got %d", cs.ColumnCount())
	}
	if len(cs.ColumnNames()) != 0 {
		t.Errorf("expected empty column names, got %v", cs.ColumnNames())
	}
}

func TestNewColumnStoreWithConfig(t *testing.T) {
	cfg := Config{DictionaryEnabled: true}
	cs := NewColumnStoreWithConfig(cfg)
	if cs == nil {
		t.Fatal("NewColumnStoreWithConfig returned nil")
	}
	if cs.RowCount() != 0 {
		t.Errorf("expected initial row count 0, got %d", cs.RowCount())
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.DictionaryEnabled {
		t.Error("expected DictionaryEnabled to be true by default")
	}
}

func TestWrite_Basic(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "name", Values: []Value{"Alice", "Bob", "Charlie"}},
		{Name: "age", Values: []Value{30, 25, 35}},
	}

	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if cs.RowCount() != 3 {
		t.Errorf("expected 3 rows, got %d", cs.RowCount())
	}
	if cs.ColumnCount() != 3 {
		t.Errorf("expected 3 columns, got %d", cs.ColumnCount())
	}

	names := cs.ColumnNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 column names, got %d", len(names))
	}
}

func TestWrite_EmptyBatch(t *testing.T) {
	cs := NewColumnStore()
	err := cs.Write([]ColumnBatch{})
	if err != ErrEmptyBatch {
		t.Errorf("expected ErrEmptyBatch, got %v", err)
	}
}

func TestWrite_NilBatch(t *testing.T) {
	cs := NewColumnStore()
	err := cs.Write(nil)
	if err != ErrEmptyBatch {
		t.Errorf("expected ErrEmptyBatch for nil, got %v", err)
	}
}

func TestWrite_ColumnMismatch(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "name", Values: []Value{"Alice", "Bob"}},
	}

	err := cs.Write(batch)
	if err != ErrColumnMismatch {
		t.Errorf("expected ErrColumnMismatch, got %v", err)
	}
	if cs.RowCount() != 0 {
		t.Errorf("expected 0 rows after failed write, got %d", cs.RowCount())
	}
}

func TestWrite_DuplicateColumnName(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2}},
		{Name: "id", Values: []Value{3, 4}},
	}

	err := cs.Write(batch)
	if err != ErrDuplicateColumnName {
		t.Errorf("expected ErrDuplicateColumnName, got %v", err)
	}
}

func TestWrite_EmptyRows(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{}},
		{Name: "name", Values: []Value{}},
	}

	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Write with empty rows failed: %v", err)
	}
	if cs.RowCount() != 0 {
		t.Errorf("expected 0 rows, got %d", cs.RowCount())
	}
}

func TestWrite_MultipleBatches(t *testing.T) {
	cs := NewColumnStore()

	batch1 := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2}},
		{Name: "name", Values: []Value{"Alice", "Bob"}},
	}
	err := cs.Write(batch1)
	if err != nil {
		t.Fatalf("Write batch1 failed: %v", err)
	}

	batch2 := []ColumnBatch{
		{Name: "id", Values: []Value{3, 4, 5}},
		{Name: "name", Values: []Value{"Charlie", "Dave", "Eve"}},
	}
	err = cs.Write(batch2)
	if err != nil {
		t.Fatalf("Write batch2 failed: %v", err)
	}

	if cs.RowCount() != 5 {
		t.Errorf("expected 5 total rows, got %d", cs.RowCount())
	}
}

func TestWrite_AppendNewColumns(t *testing.T) {
	cs := NewColumnStore()

	batch1 := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2}},
	}
	err := cs.Write(batch1)
	if err != nil {
		t.Fatalf("Write batch1 failed: %v", err)
	}

	batch2 := []ColumnBatch{
		{Name: "id", Values: []Value{3}},
		{Name: "name", Values: []Value{"Alice"}},
	}
	err = cs.Write(batch2)
	if err != nil {
		t.Fatalf("Write batch2 with new column failed: %v", err)
	}

	if cs.RowCount() != 3 {
		t.Errorf("expected 3 total rows, got %d", cs.RowCount())
	}
	if cs.ColumnCount() != 2 {
		t.Errorf("expected 2 columns, got %d", cs.ColumnCount())
	}
}

func TestWrite_AtomicityOnError(t *testing.T) {
	cs := NewColumnStore()

	batch1 := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2}},
		{Name: "name", Values: []Value{"Alice", "Bob"}},
	}
	err := cs.Write(batch1)
	if err != nil {
		t.Fatalf("initial Write failed: %v", err)
	}

	badBatch := []ColumnBatch{
		{Name: "id", Values: []Value{3}},
		{Name: "name", Values: []Value{"Charlie", "Dave"}},
	}
	err = cs.Write(badBatch)
	if err != ErrColumnMismatch {
		t.Errorf("expected ErrColumnMismatch, got %v", err)
	}

	if cs.RowCount() != 2 {
		t.Errorf("expected row count to remain 2 after failed write, got %d", cs.RowCount())
	}

	result, err := cs.Read([]string{"id", "name"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Errorf("expected 2 rows in result, got %d", len(result.Rows))
	}
}

func TestRead_Basic(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "name", Values: []Value{"Alice", "Bob", "Charlie"}},
		{Name: "age", Values: []Value{30, 25, 35}},
	}
	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	result, err := cs.Read([]string{"id", "name"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns in result, got %d", len(result.Columns))
	}

	expectedIds := []Value{1, 2, 3}
	expectedNames := []Value{"Alice", "Bob", "Charlie"}
	for i, row := range result.Rows {
		if row.Values["id"] != expectedIds[i] {
			t.Errorf("row %d: expected id=%v, got %v", i, expectedIds[i], row.Values["id"])
		}
		if row.Values["name"] != expectedNames[i] {
			t.Errorf("row %d: expected name=%v, got %v", i, expectedNames[i], row.Values["name"])
		}
		if _, exists := row.Values["age"]; exists {
			t.Errorf("row %d: age should not be in projected columns", i)
		}
	}
}

func TestRead_EmptyColumnSet(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	_, err := cs.Read([]string{})
	if err != ErrEmptyColumnSet {
		t.Errorf("expected ErrEmptyColumnSet, got %v", err)
	}
}

func TestRead_ColumnNotFound(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	_, err := cs.Read([]string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent column")
	}
}

func TestRead_SingleColumn(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{10, 20, 30}},
		{Name: "name", Values: []Value{"a", "b", "c"}},
	}
	cs.Write(batch)

	result, err := cs.Read([]string{"id"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
	for i, row := range result.Rows {
		expected := Value((i + 1) * 10)
		if row.Values["id"] != expected {
			t.Errorf("row %d: expected id=%v, got %v", i, expected, row.Values["id"])
		}
	}
}

func TestRead_EmptyStore(t *testing.T) {
	cs := NewColumnStore()

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{}},
		{Name: "name", Values: []Value{}},
	}
	cs.Write(batch)

	result, err := cs.Read([]string{"id"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

func TestDictionaryEncoding(t *testing.T) {
	cs := NewColumnStore()

	repeatedValues := make([]Value, 100)
	for i := 0; i < 100; i++ {
		repeatedValues[i] = "category_A"
	}
	for i := 0; i < 50; i++ {
		repeatedValues = append(repeatedValues, "category_B")
	}
	for i := 0; i < 25; i++ {
		repeatedValues = append(repeatedValues, "category_C")
	}

	ids := make([]Value, len(repeatedValues))
	for i := range ids {
		ids[i] = i
	}

	batch := []ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "category", Values: repeatedValues},
	}
	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	dictSize, err := cs.DictionarySize("category")
	if err != nil {
		t.Fatalf("DictionarySize failed: %v", err)
	}
	if dictSize != 3 {
		t.Errorf("expected dictionary size 3 for category column, got %d", dictSize)
	}

	idDictSize, err := cs.DictionarySize("id")
	if err != nil {
		t.Fatalf("DictionarySize for id failed: %v", err)
	}
	if idDictSize != 175 {
		t.Errorf("expected dictionary size 175 for id column, got %d", idDictSize)
	}

	result, err := cs.Read([]string{"category"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		if result.Rows[i].Values["category"] != "category_A" {
			t.Errorf("row %d: expected category_A, got %v", i, result.Rows[i].Values["category"])
			break
		}
	}
	for i := 100; i < 150; i++ {
		if result.Rows[i].Values["category"] != "category_B" {
			t.Errorf("row %d: expected category_B, got %v", i, result.Rows[i].Values["category"])
			break
		}
	}
}

func TestDictionarySize_ColumnNotFound(t *testing.T) {
	cs := NewColumnStore()
	_, err := cs.DictionarySize("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent column")
	}
}

func TestPredicate_Eq(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "name", Values: []Value{"Alice", "Bob", "Alice", "Charlie", "Alice"}},
		{Name: "age", Values: []Value{30, 25, 30, 35, 28}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "name", Op: OpEq, Value: "Alice"},
	}

	result, err := cs.ReadWithFilter([]string{"id", "name", "age"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
	if result.TotalScanned != 5 {
		t.Errorf("expected 5 scanned, got %d", result.TotalScanned)
	}

	expectedIds := []Value{1, 3, 5}
	for i, row := range result.Rows {
		if row.Values["id"] != expectedIds[i] {
			t.Errorf("row %d: expected id=%v, got %v", i, expectedIds[i], row.Values["id"])
		}
	}
}

func TestPredicate_Neq(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "status", Values: []Value{"active", "inactive", "active"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "status", Op: OpNeq, Value: "active"},
	}

	result, err := cs.ReadWithFilter([]string{"id", "status"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 1 {
		t.Errorf("expected 1 match, got %d", result.TotalMatched)
	}
	if result.Rows[0].Values["status"] != "inactive" {
		t.Errorf("expected status=inactive, got %v", result.Rows[0].Values["status"])
	}
}

func TestPredicate_GtLt(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "score", Values: []Value{50, 75, 90, 60, 85}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "score", Op: OpGt, Value: 70},
		{Column: "score", Op: OpLt, Value: 90},
	}

	result, err := cs.ReadWithFilter([]string{"id", "score"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 2 {
		t.Errorf("expected 2 matches (75 and 85), got %d", result.TotalMatched)
	}

	expectedScores := []Value{75, 85}
	for i, row := range result.Rows {
		if row.Values["score"] != expectedScores[i] {
			t.Errorf("row %d: expected score=%v, got %v", i, expectedScores[i], row.Values["score"])
		}
	}
}

func TestPredicate_GteLte(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "value", Values: []Value{10, 20, 30, 40, 50}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "value", Op: OpGte, Value: 20},
		{Column: "value", Op: OpLte, Value: 40},
	}

	result, err := cs.ReadWithFilter([]string{"value"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
}

func TestPredicate_In(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "city", Values: []Value{"NYC", "LA", "SF", "NYC", "CHI"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "city", Op: OpIn, Values: []Value{"NYC", "SF"}},
	}

	result, err := cs.ReadWithFilter([]string{"id", "city"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
}

func TestPredicate_NotIn(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "city", Values: []Value{"NYC", "LA", "SF", "NYC", "CHI"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "city", Op: OpNotIn, Values: []Value{"NYC", "SF"}},
	}

	result, err := cs.ReadWithFilter([]string{"id", "city"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 2 {
		t.Errorf("expected 2 matches (LA and CHI), got %d", result.TotalMatched)
	}
}

func TestPredicate_Combined(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5, 6}},
		{Name: "dept", Values: []Value{"eng", "sales", "eng", "hr", "eng", "sales"}},
		{Name: "salary", Values: []Value{100, 80, 120, 90, 110, 70}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "dept", Op: OpEq, Value: "eng"},
		{Column: "salary", Op: OpGt, Value: 100},
	}

	result, err := cs.ReadWithFilter([]string{"id", "dept", "salary"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 2 {
		t.Errorf("expected 2 matches, got %d", result.TotalMatched)
	}
}

func TestPredicate_NoMatches(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "val", Values: []Value{10, 20, 30}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "val", Op: OpGt, Value: 100},
	}

	result, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 0 {
		t.Errorf("expected 0 matches, got %d", result.TotalMatched)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}

func TestPredicate_AllMatch(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "val", Values: []Value{10, 20, 30}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "val", Op: OpGt, Value: 0},
	}

	result, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
}

func TestPredicate_InvalidColumn(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "nonexistent", Op: OpEq, Value: 1},
	}

	_, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err == nil {
		t.Error("expected error for invalid predicate column")
	}
}

func TestPredicate_InvalidOp(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "id", Op: Operator("INVALID"), Value: 1},
	}

	_, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != ErrInvalidOp {
		t.Errorf("expected ErrInvalidOp, got %v", err)
	}
}

func TestPredicate_InvalidValue(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "id", Op: OpEq, Value: nil},
	}

	_, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != ErrInvalidPredicate {
		t.Errorf("expected ErrInvalidPredicate, got %v", err)
	}
}

func TestPredicate_EmptyInValues(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "id", Op: OpIn, Values: []Value{}},
	}

	_, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != ErrInvalidPredicate {
		t.Errorf("expected ErrInvalidPredicate, got %v", err)
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		a, b     Value
		expected int
	}{
		{1, 2, -1},
		{2, 1, 1},
		{1, 1, 0},
		{1.5, 2.5, -1},
		{2.5, 1.5, 1},
		{1.0, 1.0, 0},
		{1, 1.0, 0},
		{1.0, 2, -1},
		{"apple", "banana", -1},
		{"banana", "apple", 1},
		{"apple", "apple", 0},
		{false, true, -1},
		{true, false, 1},
		{true, true, 0},
		{false, false, 0},
		{nil, nil, 0},
		{nil, 1, -1},
		{1, nil, 1},
	}

	for _, tt := range tests {
		result := compareValues(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("compareValues(%v, %v): expected %d, got %d", tt.a, tt.b, tt.expected, result)
		}
	}
}

func TestClose(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{{Name: "id", Values: []Value{1}}}
	cs.Write(batch)

	cs.Close()

	err := cs.Write(batch)
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed on Write after Close, got %v", err)
	}

	_, err = cs.Read([]string{"id"})
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed on Read after Close, got %v", err)
	}
}

func TestConcurrentWrite(t *testing.T) {
	cs := NewColumnStore()

	var wg sync.WaitGroup
	numWriters := 10
	rowsPerWriter := 100

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ids := make([]Value, rowsPerWriter)
			names := make([]Value, rowsPerWriter)
			for i := 0; i < rowsPerWriter; i++ {
				ids[i] = id*rowsPerWriter + i
				names[i] = fmt.Sprintf("name_%d_%d", id, i)
			}
			batch := []ColumnBatch{
				{Name: "id", Values: ids},
				{Name: "name", Values: names},
			}
			err := cs.Write(batch)
			if err != nil {
				t.Errorf("writer %d Write failed: %v", id, err)
			}
		}(w)
	}

	wg.Wait()

	expectedRows := numWriters * rowsPerWriter
	if cs.RowCount() != expectedRows {
		t.Errorf("expected %d rows, got %d", expectedRows, cs.RowCount())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	cs := NewColumnStore()

	initialBatch := []ColumnBatch{
		{Name: "id", Values: []Value{0, 1, 2}},
		{Name: "name", Values: []Value{"a", "b", "c"}},
	}
	cs.Write(initialBatch)

	var wg sync.WaitGroup
	numWriters := 5
	numReaders := 10
	iterations := 50

	for w := 0; w < numWriters; w++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				batch := []ColumnBatch{
					{Name: "id", Values: []Value{id*iterations + i + 100}},
					{Name: "name", Values: []Value{fmt.Sprintf("w%d_%d", id, i)}},
				}
				cs.Write(batch)
			}
		}(w)
	}

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				result, err := cs.Read([]string{"id"})
				if err != nil {
					t.Errorf("Read failed: %v", err)
					return
				}
				if len(result.Rows) == 0 {
					t.Error("Read returned 0 rows")
					return
				}
			}
		}()
	}

	wg.Wait()

	expectedRows := 3 + numWriters*iterations
	if cs.RowCount() != expectedRows {
		t.Errorf("expected %d rows, got %d", expectedRows, cs.RowCount())
	}
}

func TestConcurrentReadWithFilter(t *testing.T) {
	cs := NewColumnStore()

	categories := []Value{"A", "B", "C"}
	ids := make([]Value, 300)
	cats := make([]Value, 300)
	for i := 0; i < 300; i++ {
		ids[i] = i
		cats[i] = categories[i%3]
	}
	cs.Write([]ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "category", Values: cats},
	})

	var wg sync.WaitGroup
	numReaders := 10
	iterations := 50

	for r := 0; r < numReaders; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				predicates := []*Predicate{
					{Column: "category", Op: OpEq, Value: "A"},
				}
				result, err := cs.ReadWithFilter([]string{"id", "category"}, predicates)
				if err != nil {
					t.Errorf("ReadWithFilter failed: %v", err)
					return
				}
				if result.TotalMatched != 100 {
					t.Errorf("expected 100 matches for category A, got %d", result.TotalMatched)
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestPredicate_StringComparison(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4}},
		{Name: "name", Values: []Value{"alpha", "beta", "gamma", "delta"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "name", Op: OpGt, Value: "c"},
	}

	result, err := cs.ReadWithFilter([]string{"name"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 2 {
		t.Errorf("expected 2 matches (gamma, delta after 'c'), got %d", result.TotalMatched)
	}
}

func TestPredicate_BoolComparison(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4}},
		{Name: "active", Values: []Value{true, false, true, false}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "active", Op: OpEq, Value: true},
	}

	result, err := cs.ReadWithFilter([]string{"id", "active"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 2 {
		t.Errorf("expected 2 matches, got %d", result.TotalMatched)
	}
}

func TestDictionaryEncoding_MixedTypes(t *testing.T) {
	cs := NewColumnStore()

	values := make([]Value, 0)
	for i := 0; i < 10; i++ {
		values = append(values, 42)
	}
	for i := 0; i < 10; i++ {
		values = append(values, "hello")
	}
	for i := 0; i < 10; i++ {
		values = append(values, true)
	}

	ids := make([]Value, len(values))
	for i := range ids {
		ids[i] = i
	}

	batch := []ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "mixed", Values: values},
	}
	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	dictSize, err := cs.DictionarySize("mixed")
	if err != nil {
		t.Fatalf("DictionarySize failed: %v", err)
	}
	if dictSize != 3 {
		t.Errorf("expected dictionary size 3, got %d", dictSize)
	}
}

func TestErrors(t *testing.T) {
	if ErrEmptyBatch == nil {
		t.Error("ErrEmptyBatch should not be nil")
	}
	if ErrColumnMismatch == nil {
		t.Error("ErrColumnMismatch should not be nil")
	}
	if ErrDuplicateColumnName == nil {
		t.Error("ErrDuplicateColumnName should not be nil")
	}
	if ErrColumnNotFound == nil {
		t.Error("ErrColumnNotFound should not be nil")
	}
	if ErrEmptyColumnSet == nil {
		t.Error("ErrEmptyColumnSet should not be nil")
	}
	if ErrInvalidPredicate == nil {
		t.Error("ErrInvalidPredicate should not be nil")
	}
	if ErrStoreClosed == nil {
		t.Error("ErrStoreClosed should not be nil")
	}
	if ErrInvalidOp == nil {
		t.Error("ErrInvalidOp should not be nil")
	}
}

func TestReadWithFilter_NoPredicates(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "name", Values: []Value{"a", "b", "c"}},
	}
	cs.Write(batch)

	result, err := cs.ReadWithFilter([]string{"id"}, nil)
	if err != nil {
		t.Fatalf("ReadWithFilter with nil predicates failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
	if result.TotalScanned != 3 {
		t.Errorf("expected 3 scanned, got %d", result.TotalScanned)
	}
}

func TestWrite_LargeBatch(t *testing.T) {
	cs := NewColumnStore()

	n := 10000
	ids := make([]Value, n)
	names := make([]Value, n)
	for i := 0; i < n; i++ {
		ids[i] = i
		names[i] = fmt.Sprintf("name_%d", i)
	}

	batch := []ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "name", Values: names},
	}
	err := cs.Write(batch)
	if err != nil {
		t.Fatalf("Large batch Write failed: %v", err)
	}

	if cs.RowCount() != n {
		t.Errorf("expected %d rows, got %d", n, cs.RowCount())
	}

	result, err := cs.Read([]string{"id"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(result.Rows) != n {
		t.Errorf("expected %d rows in result, got %d", n, len(result.Rows))
	}
}

func TestColumnProjectionOrder(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "a", Values: []Value{1}},
		{Name: "b", Values: []Value{2}},
		{Name: "c", Values: []Value{3}},
	}
	cs.Write(batch)

	result, err := cs.Read([]string{"c", "a"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if len(result.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Columns))
	}
	if result.Columns[0] != "c" || result.Columns[1] != "a" {
		t.Errorf("expected column order [c, a], got %v", result.Columns)
	}
}

func TestQueryResultStructure(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "x", Values: []Value{10, 20}},
		{Name: "y", Values: []Value{100, 200}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "x", Op: OpEq, Value: 10},
	}

	result, err := cs.ReadWithFilter([]string{"x", "y"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalScanned != 2 {
		t.Errorf("expected TotalScanned=2, got %d", result.TotalScanned)
	}
	if result.TotalMatched != 1 {
		t.Errorf("expected TotalMatched=1, got %d", result.TotalMatched)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(result.Rows))
	}
	if result.Rows[0].Values["x"] != 10 {
		t.Errorf("expected x=10, got %v", result.Rows[0].Values["x"])
	}
	if result.Rows[0].Values["y"] != 100 {
		t.Errorf("expected y=100, got %v", result.Rows[0].Values["y"])
	}
}

func TestDictionaryEnabled_Default(t *testing.T) {
	cs := NewColumnStore()
	if !cs.IsDictionaryEnabled() {
		t.Error("expected dictionary to be enabled by default")
	}
}

func TestDictionaryEnabled_ExplicitTrue(t *testing.T) {
	cs := NewColumnStoreWithConfig(Config{DictionaryEnabled: true})
	if !cs.IsDictionaryEnabled() {
		t.Error("expected dictionary to be enabled when configured true")
	}

	repeatedValues := make([]Value, 100)
	for i := range repeatedValues {
		repeatedValues[i] = "same_value"
	}
	ids := make([]Value, 100)
	for i := range ids {
		ids[i] = i
	}

	cs.Write([]ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "val", Values: repeatedValues},
	})

	dictSize, err := cs.DictionarySize("val")
	if err != nil {
		t.Fatalf("DictionarySize failed: %v", err)
	}
	if dictSize != 1 {
		t.Errorf("expected dictionary size 1 (all same value), got %d", dictSize)
	}

	result, err := cs.Read([]string{"val"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for i, row := range result.Rows {
		if row.Values["val"] != "same_value" {
			t.Errorf("row %d: expected 'same_value', got %v", i, row.Values["val"])
		}
	}
}

func TestDictionaryEnabled_ExplicitFalse(t *testing.T) {
	cs := NewColumnStoreWithConfig(Config{DictionaryEnabled: false})
	if cs.IsDictionaryEnabled() {
		t.Error("expected dictionary to be disabled when configured false")
	}

	repeatedValues := make([]Value, 100)
	for i := range repeatedValues {
		repeatedValues[i] = "same_value"
	}
	ids := make([]Value, 100)
	for i := range ids {
		ids[i] = i
	}

	cs.Write([]ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "val", Values: repeatedValues},
	})

	dictSize, err := cs.DictionarySize("val")
	if err != nil {
		t.Fatalf("DictionarySize failed: %v", err)
	}
	if dictSize != 0 {
		t.Errorf("expected dictionary size 0 when disabled, got %d", dictSize)
	}

	result, err := cs.Read([]string{"val"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(result.Rows) != 100 {
		t.Fatalf("expected 100 rows, got %d", len(result.Rows))
	}
	for i, row := range result.Rows {
		if row.Values["val"] != "same_value" {
			t.Errorf("row %d: expected 'same_value', got %v", i, row.Values["val"])
		}
	}
}

func TestDictionaryDisabled_AllUniqueValues(t *testing.T) {
	cs := NewColumnStoreWithConfig(Config{DictionaryEnabled: false})

	n := 50
	ids := make([]Value, n)
	names := make([]Value, n)
	for i := 0; i < n; i++ {
		ids[i] = i
		names[i] = fmt.Sprintf("unique_name_%d", i)
	}

	cs.Write([]ColumnBatch{
		{Name: "id", Values: ids},
		{Name: "name", Values: names},
	})

	dictSize, err := cs.DictionarySize("name")
	if err != nil {
		t.Fatalf("DictionarySize failed: %v", err)
	}
	if dictSize != 0 {
		t.Errorf("expected dictionary size 0 when disabled, got %d", dictSize)
	}

	result, err := cs.Read([]string{"id", "name"})
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for i, row := range result.Rows {
		if row.Values["id"] != i {
			t.Errorf("row %d: expected id=%d, got %v", i, i, row.Values["id"])
		}
		expectedName := fmt.Sprintf("unique_name_%d", i)
		if row.Values["name"] != expectedName {
			t.Errorf("row %d: expected name=%s, got %v", i, expectedName, row.Values["name"])
		}
	}
}

func TestDictionaryDisabled_WithFilter(t *testing.T) {
	cs := NewColumnStoreWithConfig(Config{DictionaryEnabled: false})

	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3, 4, 5}},
		{Name: "status", Values: []Value{"active", "inactive", "active", "pending", "active"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "status", Op: OpEq, Value: "active"},
	}

	result, err := cs.ReadWithFilter([]string{"id", "status"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches, got %d", result.TotalMatched)
	}
	expectedIds := []Value{1, 3, 5}
	for i, row := range result.Rows {
		if row.Values["id"] != expectedIds[i] {
			t.Errorf("row %d: expected id=%v, got %v", i, expectedIds[i], row.Values["id"])
		}
		if row.Values["status"] != "active" {
			t.Errorf("row %d: expected status=active, got %v", i, row.Values["status"])
		}
	}
}

func TestCompareValues_TypeMismatch_IntString(t *testing.T) {
	result := compareValues(1, "1")
	if result == 0 {
		t.Error("int(1) and string(\"1\") should NOT be equal (type mismatch)")
	}

	result = compareValues("1", 1)
	if result == 0 {
		t.Error("string(\"1\") and int(1) should NOT be equal (type mismatch)")
	}
}

func TestCompareValues_TypeMismatch_BoolInt(t *testing.T) {
	result := compareValues(true, 1)
	if result == 0 {
		t.Error("bool(true) and int(1) should NOT be equal (type mismatch)")
	}

	result = compareValues(false, 0)
	if result == 0 {
		t.Error("bool(false) and int(0) should NOT be equal (type mismatch)")
	}
}

func TestCompareValues_TypeMismatch_StringBool(t *testing.T) {
	result := compareValues("true", true)
	if result == 0 {
		t.Error("string(\"true\") and bool(true) should NOT be equal (type mismatch)")
	}

	result = compareValues("false", false)
	if result == 0 {
		t.Error("string(\"false\") and bool(false) should NOT be equal (type mismatch)")
	}
}

func TestCompareValues_TypeMismatch_FloatString(t *testing.T) {
	result := compareValues(1.0, "1.0")
	if result == 0 {
		t.Error("float64(1.0) and string(\"1.0\") should NOT be equal (type mismatch)")
	}
}

func TestPredicate_TypeMismatch_NoFalsePositive(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "code", Values: []Value{100, 200, 300}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "code", Op: OpEq, Value: "100"},
	}

	result, err := cs.ReadWithFilter([]string{"id", "code"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 0 {
		t.Errorf("expected 0 matches (int vs string type mismatch), got %d", result.TotalMatched)
	}
}

func TestPredicate_TypeMismatch_Neq(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "val", Values: []Value{10, 20, 30}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "val", Op: OpNeq, Value: "10"},
	}

	result, err := cs.ReadWithFilter([]string{"id", "val"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches (all differ from string type), got %d", result.TotalMatched)
	}
}

func TestPredicate_TypeMismatch_InOperator(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "tag", Values: []Value{"a", "b", "c"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "tag", Op: OpIn, Values: []Value{1, 2, 3}},
	}

	result, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 0 {
		t.Errorf("expected 0 matches (string column vs int list type mismatch), got %d", result.TotalMatched)
	}
}

func TestPredicate_TypeMismatch_NotInOperator(t *testing.T) {
	cs := NewColumnStore()
	batch := []ColumnBatch{
		{Name: "id", Values: []Value{1, 2, 3}},
		{Name: "tag", Values: []Value{"a", "b", "c"}},
	}
	cs.Write(batch)

	predicates := []*Predicate{
		{Column: "tag", Op: OpNotIn, Values: []Value{1, 2, 3}},
	}

	result, err := cs.ReadWithFilter([]string{"id"}, predicates)
	if err != nil {
		t.Fatalf("ReadWithFilter failed: %v", err)
	}

	if result.TotalMatched != 3 {
		t.Errorf("expected 3 matches (all not in int list due to type mismatch), got %d", result.TotalMatched)
	}
}

func TestCompareValues_TypeMismatch_OrderingDeterministic(t *testing.T) {
	r1 := compareValues(1, "a")
	r2 := compareValues(1, "a")
	if r1 != r2 {
		t.Error("type mismatch comparison should be deterministic")
	}
	if r1 == 0 {
		t.Error("different types should never compare equal")
	}

	r3 := compareValues("a", 1)
	if r3 == r1 {
		t.Error("type mismatch ordering should be antisymmetric: compare(a,b) should not equal compare(b,a)")
	}
}

func TestCompareValues_IntFloatMixed(t *testing.T) {
	if compareValues(1, 1.0) != 0 {
		t.Error("int(1) and float64(1.0) should be equal")
	}
	if compareValues(1.0, 1) != 0 {
		t.Error("float64(1.0) and int(1) should be equal")
	}
	if compareValues(2, 1.5) <= 0 {
		t.Error("int(2) should be greater than float64(1.5)")
	}
	if compareValues(1.5, 2) >= 0 {
		t.Error("float64(1.5) should be less than int(2)")
	}
}
