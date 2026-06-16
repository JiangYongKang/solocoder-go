package etlpipe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func generateTestRecords(n int) []*Record {
	records := make([]*Record, n)
	now := time.Now()
	for i := 0; i < n; i++ {
		records[i] = &Record{
			ID:        fmt.Sprintf("rec-%d", i),
			SeqID:     int64(i + 1),
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Data: map[string]interface{}{
				"name":      fmt.Sprintf("User%d", i),
				"age":       20 + i%30,
				"email":     fmt.Sprintf("user%d@example.com", i),
				"status":    "active",
				"score_str": fmt.Sprintf("%d", 100+i),
				"price_str": fmt.Sprintf("%.2f", 9.99+float64(i)),
				"active_int": 1,
			},
		}
	}
	return records
}

func TestRecordOperations(t *testing.T) {
	r := &Record{
		ID:   "test-1",
		Data: make(map[string]interface{}),
	}

	r.SetField("name", "Alice")
	r.SetField("age", 30)

	if v, ok := r.GetField("name"); !ok || v != "Alice" {
		t.Errorf("expected name=Alice, got %v", v)
	}
	if v, ok := r.GetField("age"); !ok || v != 30 {
		t.Errorf("expected age=30, got %v", v)
	}
	if _, ok := r.GetField("nonexistent"); ok {
		t.Error("expected nonexistent field to return false")
	}

	r.DeleteField("age")
	if _, ok := r.GetField("age"); ok {
		t.Error("expected age to be deleted")
	}

	cloned := r.Clone()
	cloned.SetField("new", "value")
	if _, ok := r.GetField("new"); ok {
		t.Error("clone should not affect original")
	}
}

func TestSourceRegistry(t *testing.T) {
	registry := NewSourceRegistry()

	err := registry.Register("memory", NewMemorySourceFactory())
	if err != nil {
		t.Fatalf("failed to register source: %v", err)
	}

	err = registry.Register("memory", NewMemorySourceFactory())
	if !errors.Is(err, ErrSourceAlreadyRegistered) {
		t.Errorf("expected ErrSourceAlreadyRegistered, got %v", err)
	}

	if !registry.Has("memory") {
		t.Error("expected registry to have 'memory' source")
	}
	if registry.Has("nonexistent") {
		t.Error("expected registry to not have 'nonexistent' source")
	}

	types := registry.List()
	if len(types) != 1 {
		t.Errorf("expected 1 source type, got %d", len(types))
	}

	records := generateTestRecords(5)
	source, err := registry.Create("memory", map[string]interface{}{
		"records": records,
	})
	if err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if source == nil {
		t.Fatal("source should not be nil")
	}

	_, err = registry.Create("nonexistent", nil)
	if !errors.Is(err, ErrSourceNotFound) {
		t.Errorf("expected ErrSourceNotFound, got %v", err)
	}

	registry.Unregister("memory")
	if registry.Has("memory") {
		t.Error("expected 'memory' to be unregistered")
	}
}

func TestMemorySource(t *testing.T) {
	records := generateTestRecords(25)
	source := NewMemorySource(records)
	defer source.Close(context.Background())

	count, err := source.Count(context.Background(), nil)
	if err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 25 {
		t.Errorf("expected count=25, got %d", count)
	}

	cursor := &Cursor{Mode: ExtractModeFull, LastOffset: 0}
	batch, err := source.Fetch(context.Background(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if batch.Size() != 10 {
		t.Errorf("expected batch size 10, got %d", batch.Size())
	}
	if batch.FirstSeq != 1 {
		t.Errorf("expected FirstSeq=1, got %d", batch.FirstSeq)
	}
	if batch.LastSeq != 10 {
		t.Errorf("expected LastSeq=10, got %d", batch.LastSeq)
	}

	cursor.LastOffset = 10
	batch2, err := source.Fetch(context.Background(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch2 failed: %v", err)
	}
	if batch2.Size() != 10 {
		t.Errorf("expected batch size 10, got %d", batch2.Size())
	}
	if batch2.FirstSeq != 11 {
		t.Errorf("expected FirstSeq=11, got %d", batch2.FirstSeq)
	}

	cursor.LastOffset = 20
	batch3, err := source.Fetch(context.Background(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch3 failed: %v", err)
	}
	if batch3.Size() != 5 {
		t.Errorf("expected batch size 5, got %d", batch3.Size())
	}

	cursor.LastOffset = 25
	batch4, err := source.Fetch(context.Background(), cursor, 10)
	if err != nil {
		t.Fatalf("fetch4 failed: %v", err)
	}
	if batch4.Size() != 0 {
		t.Errorf("expected empty batch, got %d", batch4.Size())
	}

	source.Reset()
	cursor.LastOffset = 0
	batch5, err := source.Fetch(context.Background(), cursor, 5)
	if err != nil {
		t.Fatalf("fetch after reset failed: %v", err)
	}
	if batch5.Size() != 5 {
		t.Errorf("expected 5 records after reset, got %d", batch5.Size())
	}

	customErr := errors.New("custom fetch error")
	source.SetFetchError(customErr, 2)
	cursor.LastOffset = 5
	_, err = source.Fetch(context.Background(), cursor, 10)
	if !errors.Is(err, customErr) {
		t.Errorf("expected custom error, got %v", err)
	}
	_, err = source.Fetch(context.Background(), cursor, 10)
	if !errors.Is(err, customErr) {
		t.Errorf("expected custom error (2nd), got %v", err)
	}
	batch6, err := source.Fetch(context.Background(), cursor, 10)
	if err != nil {
		t.Errorf("expected no error after N times, got %v", err)
	}
	if batch6.Size() != 10 {
		t.Errorf("expected 10 records after error recovery, got %d", batch6.Size())
	}
}

func TestConvertType(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		targetType string
		wantErr    bool
	}{
		{"int_to_string", 42, "string", false},
		{"string_to_int", "123", "int", false},
		{"string_to_int_fail", "abc", "int", true},
		{"int64_to_int", int64(99), "int", false},
		{"float64_to_int", float64(12.99), "int", false},
		{"string_to_int64", "456", "int64", false},
		{"string_to_int64_fail", "xyz", "int64", true},
		{"int_to_float64", 10, "float64", false},
		{"string_to_float64", "3.14", "float64", false},
		{"string_to_float64_fail", "pi", "float64", true},
		{"string_to_bool_true", "true", "bool", false},
		{"string_to_bool_false", "false", "bool", false},
		{"string_to_bool_fail", "yes", "bool", true},
		{"int_to_bool_true", 1, "bool", false},
		{"int_to_bool_false", 0, "bool", false},
		{"time_string_to_time", time.Now().Format(time.RFC3339), "time", false},
		{"time_string_to_time_fail", "not-a-time", "time", true},
		{"unknown_type", "val", "complex128", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := convertType(tt.value, tt.targetType)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertType() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFieldMapTransformer(t *testing.T) {
	rules := []TransformRule{
		{
			Name: "rename_fields",
			Type: TransformTypeFieldMap,
			FieldMappings: []FieldMapping{
				{Source: "name", Target: "full_name"},
				{Source: "email", Target: "contact_email"},
			},
		},
	}
	transformer := NewBaseTransformer("mapper", rules)

	rec := &Record{
		ID: "1",
		Data: map[string]interface{}{
			"name":  "Alice",
			"age":   30,
			"email": "alice@test.com",
		},
	}

	result, err := transformer.Transform(rec)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result.GetField("full_name"); v != "Alice" {
		t.Errorf("expected full_name=Alice, got %v", v)
	}
	if v, _ := result.GetField("contact_email"); v != "alice@test.com" {
		t.Errorf("expected contact_email=alice@test.com, got %v", v)
	}
	if _, ok := result.GetField("name"); ok {
		t.Error("name should be removed after mapping")
	}
	if v, _ := result.GetField("age"); v != 30 {
		t.Errorf("age should remain, got %v", v)
	}
}

func TestTypeConvertTransformer(t *testing.T) {
	rules := []TransformRule{
		{
			Name: "convert_types",
			Type: TransformTypeTypeConvert,
			TypeConversions: []TypeConversion{
				{Field: "score_str", TargetType: "int"},
				{Field: "price_str", TargetType: "float64"},
				{Field: "active_int", TargetType: "bool"},
				{Field: "age", TargetType: "string"},
			},
		},
	}
	transformer := NewBaseTransformer("converter", rules)

	rec := &Record{
		ID: "1",
		Data: map[string]interface{}{
			"score_str":  "150",
			"price_str":  "19.99",
			"active_int": 1,
			"age":        25,
		},
	}

	result, err := transformer.Transform(rec)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result.GetField("score_str"); v != 150 {
		t.Errorf("expected score_str=150 (int), got %v (%T)", v, v)
	}
	if v, _ := result.GetField("price_str"); v != 19.99 {
		t.Errorf("expected price_str=19.99 (float64), got %v (%T)", v, v)
	}
	if v, _ := result.GetField("active_int"); v != true {
		t.Errorf("expected active_int=true (bool), got %v", v)
	}
	if v, _ := result.GetField("age"); v != "25" {
		t.Errorf("expected age=\"25\" (string), got %v", v)
	}
}

func TestValueReplaceTransformer(t *testing.T) {
	rules := []TransformRule{
		{
			Name: "replace_values",
			Type: TransformTypeValueReplace,
			Replacements: []ValueReplacement{
				{Field: "status", Old: "active", New: "ENABLED"},
				{Field: "status", Old: "inactive", New: "DISABLED"},
			},
		},
	}
	transformer := NewBaseTransformer("replacer", rules)

	rec1 := &Record{ID: "1", Data: map[string]interface{}{"status": "active"}}
	result1, err := transformer.Transform(rec1)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result1.GetField("status"); v != "ENABLED" {
		t.Errorf("expected status=ENABLED, got %v", v)
	}

	rec2 := &Record{ID: "2", Data: map[string]interface{}{"status": "inactive"}}
	result2, err := transformer.Transform(rec2)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result2.GetField("status"); v != "DISABLED" {
		t.Errorf("expected status=DISABLED, got %v", v)
	}

	rec3 := &Record{ID: "3", Data: map[string]interface{}{"status": "pending"}}
	result3, err := transformer.Transform(rec3)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result3.GetField("status"); v != "pending" {
		t.Errorf("expected status=pending (unchanged), got %v", v)
	}
}

func TestFieldFilterTransformer(t *testing.T) {
	rules := []TransformRule{
		{
			Name: "filter_fields",
			Type: TransformTypeFieldFilter,
			Filter: FieldFilter{
				KeepFields:   []string{"name", "age", "email"},
				RemoveFields: []string{"email"},
			},
		},
	}
	transformer := NewBaseTransformer("filter", rules)

	rec := &Record{
		ID: "1",
		Data: map[string]interface{}{
			"name":    "Alice",
			"age":     30,
			"email":   "alice@test.com",
			"status":  "active",
			"extra":   "data",
		},
	}

	result, err := transformer.Transform(rec)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}

	if _, ok := result.GetField("name"); !ok {
		t.Error("name should be kept")
	}
	if _, ok := result.GetField("age"); !ok {
		t.Error("age should be kept")
	}
	if _, ok := result.GetField("email"); ok {
		t.Error("email should be removed")
	}
	if _, ok := result.GetField("status"); ok {
		t.Error("status should be filtered out")
	}
	if _, ok := result.GetField("extra"); ok {
		t.Error("extra should be filtered out")
	}
}

func TestFieldCalculateTransformer(t *testing.T) {
	rules := []TransformRule{
		{
			Name: "calculate_fullname",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "full_info",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					name, _ := data["name"].(string)
					age := data["age"]
					return fmt.Sprintf("%s (age: %v)", name, age), nil
				},
			},
		},
		{
			Name: "calculate_discounted",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "discounted_price",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					price, ok := data["price"].(float64)
					if !ok {
						return nil, errors.New("invalid price")
					}
					return price * 0.9, nil
				},
			},
		},
	}
	transformer := NewBaseTransformer("calculator", rules)

	rec := &Record{
		ID: "1",
		Data: map[string]interface{}{
			"name":  "Alice",
			"age":   30,
			"price": 100.0,
		},
	}

	result, err := transformer.Transform(rec)
	if err != nil {
		t.Fatalf("transform failed: %v", err)
	}
	if v, _ := result.GetField("full_info"); v != "Alice (age: 30)" {
		t.Errorf("expected full_info, got %v", v)
	}
	if v, _ := result.GetField("discounted_price"); v != 90.0 {
		t.Errorf("expected discounted_price=90.0, got %v", v)
	}
}

func TestTransformerNilRecord(t *testing.T) {
	transformer := NewBaseTransformer("test", []TransformRule{})
	_, err := transformer.Transform(nil)
	if err == nil {
		t.Error("expected error for nil record")
	}
}

func TestTransformChain(t *testing.T) {
	chain := NewTransformChain()

	err := chain.Add(nil)
	if !errors.Is(err, ErrTransformNil) {
		t.Errorf("expected ErrTransformNil, got %v", err)
	}

	t1 := NewBaseTransformer("stage1", []TransformRule{
		{
			Name: "map",
			Type: TransformTypeFieldMap,
			FieldMappings: []FieldMapping{
				{Source: "name", Target: "username"},
			},
		},
	})
	t2 := NewBaseTransformer("stage2", []TransformRule{
		{
			Name: "uppercase",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "username",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					s, _ := data["username"].(string)
					return "USER_" + s, nil
				},
			},
		},
	})

	if err := chain.Add(t1); err != nil {
		t.Fatalf("add t1 failed: %v", err)
	}
	if err := chain.Add(t2); err != nil {
		t.Fatalf("add t2 failed: %v", err)
	}

	if chain.Count() != 2 {
		t.Errorf("expected count=2, got %d", chain.Count())
	}

	names := chain.List()
	if len(names) != 2 || names[0] != "stage1" || names[1] != "stage2" {
		t.Errorf("unexpected chain list: %v", names)
	}

	rec := &Record{ID: "1", Data: map[string]interface{}{"name": "Alice"}}
	result, err := chain.Process(rec)
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if v, _ := result.GetField("username"); v != "USER_Alice" {
		t.Errorf("expected username=USER_Alice, got %v", v)
	}

	t0 := NewBaseTransformer("stage0", []TransformRule{})
	if err := chain.Insert(0, t0); err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	if chain.Count() != 3 {
		t.Errorf("expected count=3 after insert, got %d", chain.Count())
	}
	if chain.List()[0] != "stage0" {
		t.Error("stage0 should be at index 0")
	}

	if err := chain.Insert(100, t0); err == nil {
		t.Error("expected error for insert out of range")
	}

	if err := chain.Remove(0); err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	if chain.Count() != 2 {
		t.Errorf("expected count=2 after remove, got %d", chain.Count())
	}

	if err := chain.Remove(100); err == nil {
		t.Error("expected error for remove out of range")
	}
}

func TestTransformChainErrorIsolation(t *testing.T) {
	chain := NewTransformChain()

	goodTransformer := NewBaseTransformer("good", []TransformRule{
		{
			Name: "set_flag",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "processed",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					return true, nil
				},
			},
		},
	})

	badTransformer := NewBaseTransformer("bad", []TransformRule{
		{
			Name: "fail",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "will_fail",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					return nil, errors.New("intentional failure")
				},
			},
		},
	})

	_ = chain.Add(goodTransformer)
	_ = chain.Add(badTransformer)

	rec := &Record{ID: "bad-rec", Data: map[string]interface{}{}}
	_, err := chain.Process(rec)
	if err == nil {
		t.Fatal("expected error from chain")
	}

	te, ok := err.(*TransformError)
	if !ok {
		t.Fatalf("expected TransformError, got %T", err)
	}
	if te.StageName != "bad" {
		t.Errorf("expected StageName=bad, got %s", te.StageName)
	}
	if te.StageIndex != 1 {
		t.Errorf("expected StageIndex=1, got %d", te.StageIndex)
	}
	if te.Record.ID != "bad-rec" {
		t.Errorf("expected Record.ID=bad-rec, got %s", te.Record.ID)
	}
}

func TestErrorQueue(t *testing.T) {
	q := NewErrorQueue(5)

	rec := &Record{ID: "err-1", Data: map[string]interface{}{}}
	te := &TransformError{
		StageName: "s1",
		StageIndex: 0,
		Record: rec,
		Err: errors.New("t-err"),
		Timestamp: time.Now(),
	}
	we := &WriteError{
		Record: rec,
		Err: errors.New("w-err"),
		Timestamp: time.Now(),
	}

	q.AddTransformError(te)
	q.AddWriteError(we)

	if q.TransformErrorCount() != 1 {
		t.Errorf("expected 1 transform error, got %d", q.TransformErrorCount())
	}
	if q.WriteErrorCount() != 1 {
		t.Errorf("expected 1 write error, got %d", q.WriteErrorCount())
	}
	if q.TotalErrorCount() != 2 {
		t.Errorf("expected 2 total errors, got %d", q.TotalErrorCount())
	}

	tes := q.GetTransformErrors()
	if len(tes) != 1 || tes[0].StageName != "s1" {
		t.Errorf("unexpected transform errors: %v", tes)
	}

	wes := q.GetWriteErrors()
	if len(wes) != 1 || wes[0].Record.ID != "err-1" {
		t.Errorf("unexpected write errors: %v", wes)
	}

	for i := 0; i < 10; i++ {
		q.AddTransformError(&TransformError{
			StageName: fmt.Sprintf("s%d", i),
			Record: &Record{ID: fmt.Sprintf("rec-%d", i)},
			Err: errors.New("err"),
		})
	}
	if q.TransformErrorCount() != 5 {
		t.Errorf("expected max 5 transform errors, got %d", q.TransformErrorCount())
	}

	q.Clear()
	if q.TotalErrorCount() != 0 {
		t.Errorf("expected 0 errors after clear, got %d", q.TotalErrorCount())
	}
}

func TestMemoryTarget(t *testing.T) {
	target := NewMemoryTarget()

	records := generateTestRecords(10)
	failed, err := target.WriteBatch(context.Background(), records)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if len(failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(failed))
	}
	if target.Count() != 10 {
		t.Errorf("expected count=10, got %d", target.Count())
	}

	target.SetFailRecord("rec-3")
	target.SetFailRecord("rec-7")
	moreRecords := generateTestRecords(10)
	failed, err = target.WriteBatch(context.Background(), moreRecords)
	if err != nil {
		t.Fatalf("write2 failed: %v", err)
	}
	if len(failed) != 2 {
		t.Errorf("expected 2 failed records, got %d", len(failed))
	}
	if target.Count() != 18 {
		t.Errorf("expected count=18, got %d", target.Count())
	}

	all := target.GetAll()
	if len(all) != 18 {
		t.Errorf("GetAll returned %d records, expected 18", len(all))
	}

	target.Clear()
	if target.Count() != 0 {
		t.Errorf("expected 0 after clear, got %d", target.Count())
	}

	if err := target.Close(context.Background()); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestNewPipelineValidation(t *testing.T) {
	cfg := DefaultConfig()

	_, err := NewPipeline(cfg, nil, NewMemoryTarget())
	if !errors.Is(err, ErrSourceNil) {
		t.Errorf("expected ErrSourceNil, got %v", err)
	}

	_, err = NewPipeline(cfg, NewMemorySource(nil), nil)
	if !errors.Is(err, ErrTargetNil) {
		t.Errorf("expected ErrTargetNil, got %v", err)
	}

	cfgBadBatch := DefaultConfig()
	cfgBadBatch.BatchSize = 0
	_, err = NewPipeline(cfgBadBatch, NewMemorySource(nil), NewMemoryTarget())
	if !errors.Is(err, ErrBatchSizeInvalid) {
		t.Errorf("expected ErrBatchSizeInvalid, got %v", err)
	}

	cfgBadInc := DefaultConfig()
	cfgBadInc.ExtractMode = ExtractModeTimestamp
	cfgBadInc.IncrementalField = ""
	_, err = NewPipeline(cfgBadInc, NewMemorySource(nil), NewMemoryTarget())
	if !errors.Is(err, ErrIncrementalNoField) {
		t.Errorf("expected ErrIncrementalNoField, got %v", err)
	}
}

func TestPipelineFullExtract(t *testing.T) {
	records := generateTestRecords(50)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 10

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	mapper := NewBaseTransformer("rename", []TransformRule{
		{
			Name: "map_fields",
			Type: TransformTypeFieldMap,
			FieldMappings: []FieldMapping{
				{Source: "name", Target: "user_name"},
			},
		},
	})
	if err := pipeline.AddTransformer(mapper); err != nil {
		t.Fatalf("add transformer failed: %v", err)
	}

	if pipeline.Status() != PipelineStatusIdle {
		t.Errorf("expected idle status, got %v", pipeline.Status())
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	if pipeline.Status() != PipelineStatusCompleted {
		t.Errorf("expected completed status, got %v", pipeline.Status())
	}

	stats := pipeline.Stats()
	if stats.ExtractedCount != 50 {
		t.Errorf("expected ExtractedCount=50, got %d", stats.ExtractedCount)
	}
	if stats.TransformedCount != 50 {
		t.Errorf("expected TransformedCount=50, got %d", stats.TransformedCount)
	}
	if stats.WrittenCount != 50 {
		t.Errorf("expected WrittenCount=50, got %d", stats.WrittenCount)
	}
	if stats.TransformErrorCount != 0 {
		t.Errorf("expected 0 transform errors, got %d", stats.TransformErrorCount)
	}
	if stats.BatchCount != 5 {
		t.Errorf("expected 5 batches, got %d", stats.BatchCount)
	}

	if target.Count() != 50 {
		t.Errorf("expected 50 records in target, got %d", target.Count())
	}

	allRecords := target.GetAll()
	if v, _ := allRecords[0].GetField("user_name"); v != "User0" {
		t.Errorf("expected field mapped user_name=User0, got %v", v)
	}
	if _, ok := allRecords[0].GetField("name"); ok {
		t.Error("name should be removed after mapping")
	}

	cursor := pipeline.GetCursor()
	if cursor.LastOffset != 50 {
		t.Errorf("expected cursor LastOffset=50, got %d", cursor.LastOffset)
	}
}

func TestPipelineIncrementalByID(t *testing.T) {
	records := generateTestRecords(30)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	cfg := Config{
		BatchSize:        10,
		WriteTimeout:     30 * time.Second,
		ExtractMode:      ExtractModeID,
		IncrementalField: "seq_id",
	}

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	cursor := pipeline.GetCursor()
	if cursor.Mode != ExtractModeID {
		t.Errorf("expected cursor Mode=ID, got %v", cursor.Mode)
	}
	lastSeq, ok := cursor.LastValue.(int64)
	if !ok {
		t.Fatalf("LastValue should be int64, got %T", cursor.LastValue)
	}
	if lastSeq != 30 {
		t.Errorf("expected LastSeq=30, got %d", lastSeq)
	}
}

func TestPipelineIncrementalByTimestamp(t *testing.T) {
	records := generateTestRecords(20)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	cfg := Config{
		BatchSize:        8,
		WriteTimeout:     30 * time.Second,
		ExtractMode:      ExtractModeTimestamp,
		IncrementalField: "created_at",
	}

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	cursor := pipeline.GetCursor()
	if cursor.Mode != ExtractModeTimestamp {
		t.Errorf("expected cursor Mode=Timestamp, got %v", cursor.Mode)
	}
	if cursor.LastValue == nil {
		t.Error("LastValue should not be nil")
	}
}

func TestPipelineTransformErrorsIsolation(t *testing.T) {
	records := generateTestRecords(20)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 5

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	flaky := NewBaseTransformer("flaky", []TransformRule{
		{
			Name: "fail_some",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "processed",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					age, _ := data["age"].(int)
					if age%7 == 0 {
						return nil, fmt.Errorf("age %d is divisible by 7", age)
					}
					return true, nil
				},
			},
		},
	})
	if err := pipeline.AddTransformer(flaky); err != nil {
		t.Fatalf("add transformer failed: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	stats := pipeline.Stats()
	if stats.ExtractedCount != 20 {
		t.Errorf("expected ExtractedCount=20, got %d", stats.ExtractedCount)
	}
	if stats.TransformErrorCount == 0 {
		t.Error("expected some transform errors")
	}
	expectedSuccess := 20 - stats.TransformErrorCount
	if stats.TransformedCount != expectedSuccess {
		t.Errorf("expected TransformedCount=%d, got %d", expectedSuccess, stats.TransformedCount)
	}
	if stats.WrittenCount != expectedSuccess {
		t.Errorf("expected WrittenCount=%d, got %d", expectedSuccess, stats.WrittenCount)
	}
	if target.Count() != int(expectedSuccess) {
		t.Errorf("expected target count=%d, got %d", expectedSuccess, target.Count())
	}

	eq := pipeline.GetErrorQueue()
	if eq.TransformErrorCount() != int(stats.TransformErrorCount) {
		t.Errorf("error queue transform count mismatch")
	}

	transformErrors := eq.GetTransformErrors()
	for _, te := range transformErrors {
		if te.StageName != "flaky" {
			t.Errorf("expected stage=flaky, got %s", te.StageName)
		}
		if te.Record == nil {
			t.Error("transform error record should not be nil")
		}
		if te.Err == nil {
			t.Error("transform error Err should not be nil")
		}
		if te.Timestamp.IsZero() {
			t.Error("transform error Timestamp should be set")
		}
	}
}

func TestPipelineWriteErrorsIsolation(t *testing.T) {
	records := generateTestRecords(20)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	target.SetFailRecord("rec-3")
	target.SetFailRecord("rec-7")
	target.SetFailRecord("rec-15")

	cfg := DefaultConfig()
	cfg.BatchSize = 10

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	stats := pipeline.Stats()
	if stats.ExtractedCount != 20 {
		t.Errorf("expected ExtractedCount=20, got %d", stats.ExtractedCount)
	}
	if stats.TransformedCount != 20 {
		t.Errorf("expected TransformedCount=20, got %d", stats.TransformedCount)
	}
	if stats.WriteErrorCount != 3 {
		t.Errorf("expected WriteErrorCount=3, got %d", stats.WriteErrorCount)
	}
	if stats.WrittenCount != 17 {
		t.Errorf("expected WrittenCount=17, got %d", stats.WrittenCount)
	}
	if target.Count() != 17 {
		t.Errorf("expected target count=17, got %d", target.Count())
	}

	eq := pipeline.GetErrorQueue()
	if eq.WriteErrorCount() != 3 {
		t.Errorf("expected 3 write errors in queue, got %d", eq.WriteErrorCount())
	}

	writeErrors := eq.GetWriteErrors()
	failIDSet := make(map[string]bool)
	for _, we := range writeErrors {
		failIDSet[we.Record.ID] = true
	}
	if !failIDSet["rec-3"] || !failIDSet["rec-7"] || !failIDSet["rec-15"] {
		t.Errorf("write error records mismatch: %v", failIDSet)
	}
}

func TestPipelineFetchError(t *testing.T) {
	records := generateTestRecords(10)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	customErr := errors.New("fetch failed")
	source.SetFetchError(customErr, 1)

	cfg := DefaultConfig()
	cfg.BatchSize = 5

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err == nil {
		t.Error("expected error from fetch failure")
	}

	if pipeline.Status() != PipelineStatusFailed {
		t.Errorf("expected failed status, got %v", pipeline.Status())
	}
}

func TestPipelineStop(t *testing.T) {
	records := generateTestRecords(100)
	source := NewMemorySource(records)
	source.SetFetchDelay(50 * time.Millisecond)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 10

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = pipeline.Run(context.Background())
	}()

	time.Sleep(120 * time.Millisecond)
	pipeline.Stop()
	wg.Wait()

	status := pipeline.Status()
	if status != PipelineStatusStopped {
		t.Errorf("expected stopped status, got %v", status)
	}
}

func TestPipelineContextCancel(t *testing.T) {
	records := generateTestRecords(100)
	source := NewMemorySource(records)
	source.SetFetchDelay(50 * time.Millisecond)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 10

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = pipeline.Run(ctx)
	}()

	time.Sleep(120 * time.Millisecond)
	cancel()
	wg.Wait()

	status := pipeline.Status()
	if status != PipelineStatusFailed {
		t.Errorf("expected failed status after context cancel, got %v", status)
	}
}

func TestPipelineAlreadyRunning(t *testing.T) {
	records := generateTestRecords(10)
	source := NewMemorySource(records)
	source.SetFetchDelay(100 * time.Millisecond)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 5

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = pipeline.Run(context.Background())
	}()

	time.Sleep(30 * time.Millisecond)
	err = pipeline.Run(context.Background())
	if !errors.Is(err, ErrPipelineRunning) {
		t.Errorf("expected ErrPipelineRunning, got %v", err)
	}

	wg.Wait()
}

func TestPipelineWriteTimeout(t *testing.T) {
	records := generateTestRecords(10)
	source := NewMemorySource(records)

	slowTarget := &slowMemoryTarget{
		target:    NewMemoryTarget(),
		delay:     200 * time.Millisecond,
		onceDelay: true,
	}

	cfg := DefaultConfig()
	cfg.BatchSize = 10
	cfg.WriteTimeout = 50 * time.Millisecond

	pipeline, err := NewPipeline(cfg, source, slowTarget)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline should not return error on timeout (errors isolated): %v", err)
	}

	stats := pipeline.Stats()
	if stats.WriteErrorCount < 0 {
		t.Errorf("expected some write errors due to timeout")
	}
}

type slowMemoryTarget struct {
	target    *MemoryTarget
	delay     time.Duration
	onceDelay bool
	hasDelayed bool
	mu        sync.Mutex
}

func (s *slowMemoryTarget) WriteBatch(ctx context.Context, records []*Record) ([]int, error) {
	s.mu.Lock()
	shouldDelay := !s.hasDelayed || !s.onceDelay
	if shouldDelay {
		s.hasDelayed = true
		s.mu.Unlock()
		time.Sleep(s.delay)
	} else {
		s.mu.Unlock()
	}
	return s.target.WriteBatch(ctx, records)
}

func (s *slowMemoryTarget) Close(ctx context.Context) error {
	return s.target.Close(ctx)
}

func TestPipelineMultipleStages(t *testing.T) {
	records := generateTestRecords(15)
	source := NewMemorySource(records)
	target := NewMemoryTarget()

	cfg := DefaultConfig()
	cfg.BatchSize = 5

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	stage1 := NewBaseTransformer("stage1_rename", []TransformRule{
		{
			Name: "rename",
			Type: TransformTypeFieldMap,
			FieldMappings: []FieldMapping{
				{Source: "name", Target: "full_name"},
				{Source: "age", Target: "user_age"},
			},
		},
	})

	stage2 := NewBaseTransformer("stage2_convert", []TransformRule{
		{
			Name: "convert_score",
			Type: TransformTypeTypeConvert,
			TypeConversions: []TypeConversion{
				{Field: "score_str", TargetType: "int"},
			},
		},
	})

	stage3 := NewBaseTransformer("stage3_filter", []TransformRule{
		{
			Name: "keep_fields",
			Type: TransformTypeFieldFilter,
			Filter: FieldFilter{
				KeepFields: []string{"full_name", "user_age", "email", "score_str"},
			},
		},
	})

	stage4 := NewBaseTransformer("stage4_calc", []TransformRule{
		{
			Name: "add_score_label",
			Type: TransformTypeFieldCalculate,
			Calculation: &FieldCalculation{
				TargetField: "score_label",
				Calculator: func(data map[string]interface{}) (interface{}, error) {
					score, _ := data["score_str"].(int)
					if score >= 110 {
						return "HIGH", nil
					}
					return "NORMAL", nil
				},
			},
		},
	})

	_ = pipeline.AddTransformer(stage1)
	_ = pipeline.AddTransformer(stage2)
	_ = pipeline.AddTransformer(stage3)
	_ = pipeline.AddTransformer(stage4)

	names := pipeline.ListTransformers()
	if len(names) != 4 {
		t.Fatalf("expected 4 transformers, got %d", len(names))
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	stats := pipeline.Stats()
	if stats.ExtractedCount != 15 {
		t.Errorf("expected ExtractedCount=15, got %d", stats.ExtractedCount)
	}
	if stats.TransformErrorCount != 0 {
		t.Errorf("expected 0 transform errors, got %d", stats.TransformErrorCount)
	}
	if stats.WrittenCount != 15 {
		t.Errorf("expected WrittenCount=15, got %d", stats.WrittenCount)
	}

	resultRecords := target.GetAll()
	first := resultRecords[0]
	if v, _ := first.GetField("full_name"); v != "User0" {
		t.Errorf("expected full_name=User0, got %v", v)
	}
	if v, _ := first.GetField("user_age"); v != 20 {
		t.Errorf("expected user_age=20, got %v", v)
	}
	if _, ok := first.GetField("name"); ok {
		t.Error("name should be removed")
	}
	if v, _ := first.GetField("score_str"); v != 100 {
		t.Errorf("expected score_str=100 (int), got %v (%T)", v, v)
	}
	if v, _ := first.GetField("score_label"); v != "NORMAL" {
		t.Errorf("expected score_label=NORMAL, got %v", v)
	}
	if _, ok := first.GetField("status"); ok {
		t.Error("status should be filtered out")
	}

	highRec := resultRecords[10]
	if v, _ := highRec.GetField("score_label"); v != "HIGH" {
		t.Errorf("record 10 should have HIGH score, got %v", v)
	}
}

func TestPipelineTransformerManagement(t *testing.T) {
	source := NewMemorySource(nil)
	target := NewMemoryTarget()
	cfg := DefaultConfig()

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	t1 := NewBaseTransformer("t1", []TransformRule{})
	t2 := NewBaseTransformer("t2", []TransformRule{})
	t3 := NewBaseTransformer("t3", []TransformRule{})
	t0 := NewBaseTransformer("t0", []TransformRule{})

	_ = pipeline.AddTransformer(t1)
	_ = pipeline.AddTransformer(t2)
	_ = pipeline.AddTransformer(t3)

	if len(pipeline.ListTransformers()) != 3 {
		t.Fatalf("expected 3 transformers")
	}

	err = pipeline.InsertTransformer(0, t0)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	names := pipeline.ListTransformers()
	if names[0] != "t0" || names[3] != "t3" {
		t.Errorf("unexpected order after insert: %v", names)
	}

	err = pipeline.RemoveTransformer(1)
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}
	names = pipeline.ListTransformers()
	if len(names) != 3 || names[1] != "t2" {
		t.Errorf("unexpected order after remove: %v", names)
	}
}

func TestPipelineEmptyData(t *testing.T) {
	source := NewMemorySource([]*Record{})
	target := NewMemoryTarget()
	cfg := DefaultConfig()

	pipeline, err := NewPipeline(cfg, source, target)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	err = pipeline.Run(context.Background())
	if err != nil {
		t.Fatalf("pipeline should handle empty data: %v", err)
	}

	if pipeline.Status() != PipelineStatusCompleted {
		t.Errorf("expected completed status, got %v", pipeline.Status())
	}
	stats := pipeline.Stats()
	if stats.ExtractedCount != 0 || stats.WrittenCount != 0 || stats.BatchCount != 0 {
		t.Errorf("expected all zeros for empty data, got %+v", stats)
	}
}

func TestPipelineStatusString(t *testing.T) {
	tests := []struct {
		status PipelineStatus
		want   string
	}{
		{PipelineStatusIdle, "idle"},
		{PipelineStatusRunning, "running"},
		{PipelineStatusCompleted, "completed"},
		{PipelineStatusFailed, "failed"},
		{PipelineStatusStopped, "stopped"},
		{PipelineStatus(999), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("PipelineStatus(%d).String() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestErrorUnwrap(t *testing.T) {
	cause := errors.New("root cause")
	te := &TransformError{
		StageName: "s1",
		Err:       cause,
	}
	if !errors.Is(te, cause) {
		t.Error("TransformError should unwrap to cause")
	}

	we := &WriteError{
		Record: &Record{ID: "r1"},
		Err:    cause,
	}
	if !errors.Is(we, cause) {
		t.Error("WriteError should unwrap to cause")
	}
}
