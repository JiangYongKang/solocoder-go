package mapreduce

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func wordCountMap(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
	text := value.(string)
	words := strings.Fields(text)
	var kvs []KeyValue
	for _, w := range words {
		kvs = append(kvs, KeyValue{Key: w, Value: 1})
	}
	return kvs, nil
}

func wordCountReduce(_ context.Context, key string, values []interface{}) (interface{}, error) {
	sum := 0
	for _, v := range values {
		sum += v.(int)
	}
	return sum, nil
}

func alwaysFailMap(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
	return nil, errors.New("map always fails")
}

func alwaysFailReduce(_ context.Context, key string, values []interface{}) (interface{}, error) {
	return nil, errors.New("reduce always fails")
}

func failNTimesMap(n int32) MapFunc {
	var calls int32
	return func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		if atomic.AddInt32(&calls, 1) <= n {
			return nil, fmt.Errorf("map fail #%d", calls)
		}
		return wordCountMap(context.Background(), key, value)
	}
}

func failNTimesReduce(n int32) ReduceFunc {
	var calls int32
	return func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		if atomic.AddInt32(&calls, 1) <= n {
			return nil, fmt.Errorf("reduce fail #%d", calls)
		}
		sum := 0
		for _, v := range values {
			sum += v.(int)
		}
		return sum, nil
	}
}

func collectWordCounts(result interface{}) map[string]int {
	wordCounts := make(map[string]int)
	partitions := result.([]interface{})
	for _, partition := range partitions {
		if partition == nil {
			continue
		}
		kvs := partition.([]KeyValue)
		for _, kv := range kvs {
			wordCounts[kv.Key] += kv.Value.(int)
		}
	}
	return wordCounts
}

func TestNewMapReduce_NoMapFunc(t *testing.T) {
	_, err := NewMapReduce(Config{
		MapFunc:    nil,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})
	if err != ErrNoMapFunc {
		t.Errorf("expected ErrNoMapFunc, got %v", err)
	}
}

func TestNewMapReduce_NoReduceFunc(t *testing.T) {
	_, err := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: nil,
		NumReduce:  2,
	})
	if err != ErrNoReduceFunc {
		t.Errorf("expected ErrNoReduceFunc, got %v", err)
	}
}

func TestNewMapReduce_InvalidReduceNum(t *testing.T) {
	_, err := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  0,
	})
	if err != ErrInvalidReduceNum {
		t.Errorf("expected ErrInvalidReduceNum, got %v", err)
	}

	_, err = NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  -1,
	})
	if err != ErrInvalidReduceNum {
		t.Errorf("expected ErrInvalidReduceNum, got %v", err)
	}
}

func TestNewMapReduce_InvalidRetry(t *testing.T) {
	_, err := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
		MaxRetries: -1,
	})
	if err != ErrInvalidRetry {
		t.Errorf("expected ErrInvalidRetry, got %v", err)
	}
}

func TestNewMapReduce_DefaultPartitionFunc(t *testing.T) {
	mr, err := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.cfg.PartitionFunc == nil {
		t.Error("expected default PartitionFunc to be set")
	}
}

func TestNewMapReduce_ValidConfig(t *testing.T) {
	mr, err := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  3,
		MaxRetries: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mr.cfg.NumReduce != 3 {
		t.Errorf("expected NumReduce=3, got %d", mr.cfg.NumReduce)
	}
	if mr.cfg.MaxRetries != 2 {
		t.Errorf("expected MaxRetries=2, got %d", mr.cfg.MaxRetries)
	}
}

func TestRun_NoInputData(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})
	_, err := mr.Run(context.Background())
	if err != ErrNoInputData {
		t.Errorf("expected ErrNoInputData, got %v", err)
	}
}

func TestRun_SyncShuffle_BasicWordCount(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   2,
		ShuffleMode: ShuffleSync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello world hello"},
		{Key: "doc2", Value: "world go world"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["hello"] != 2 {
		t.Errorf("expected hello=2, got %d", wc["hello"])
	}
	if wc["world"] != 3 {
		t.Errorf("expected world=3, got %d", wc["world"])
	}
	if wc["go"] != 1 {
		t.Errorf("expected go=1, got %d", wc["go"])
	}
}

func TestRun_AsyncShuffle_BasicWordCount(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   2,
		ShuffleMode: ShuffleAsync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello world hello"},
		{Key: "doc2", Value: "world go world"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["hello"] != 2 {
		t.Errorf("expected hello=2, got %d", wc["hello"])
	}
	if wc["world"] != 3 {
		t.Errorf("expected world=3, got %d", wc["world"])
	}
	if wc["go"] != 1 {
		t.Errorf("expected go=1, got %d", wc["go"])
	}
}

func TestRun_SingleReduce(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c"},
		{Key: "doc2", Value: "a b"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["a"] != 2 {
		t.Errorf("expected a=2, got %d", wc["a"])
	}
	if wc["b"] != 2 {
		t.Errorf("expected b=2, got %d", wc["b"])
	}
	if wc["c"] != 1 {
		t.Errorf("expected c=1, got %d", wc["c"])
	}
}

func TestRun_SingleInput(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["hello"] != 1 {
		t.Errorf("expected hello=1, got %d", wc["hello"])
	}
}

func TestRun_CustomMergeFunc(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
		MergeFunc: func(results []interface{}) (interface{}, error) {
			merged := make(map[string]int)
			for _, r := range results {
				if r == nil {
					continue
				}
				kvs := r.([]KeyValue)
				for _, kv := range kvs {
					merged[kv.Key] += kv.Value.(int)
				}
			}
			return merged, nil
		},
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello world"},
		{Key: "doc2", Value: "hello go"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	merged := result.(map[string]int)
	if merged["hello"] != 2 {
		t.Errorf("expected hello=2, got %d", merged["hello"])
	}
	if merged["world"] != 1 {
		t.Errorf("expected world=1, got %d", merged["world"])
	}
	if merged["go"] != 1 {
		t.Errorf("expected go=1, got %d", merged["go"])
	}
}

func TestRun_CustomPartitionFunc(t *testing.T) {
	customPartition := func(key string, numPartitions int) int {
		if key < "m" {
			return 0
		}
		return 1
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:       wordCountMap,
		ReduceFunc:    wordCountReduce,
		NumReduce:     2,
		PartitionFunc: customPartition,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "apple banana cherry"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := result.([]interface{})
	if len(results) != 2 {
		t.Errorf("expected 2 reduce results, got %d", len(results))
	}
}

func TestRun_MapTaskRetry_Success(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    failNTimesMap(2),
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 3,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["a"] != 1 {
		t.Errorf("expected a=1, got %d", wc["a"])
	}
}

func TestRun_MapTaskRetry_Exhausted(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error for exhausted map retries")
	}
}

func TestRun_ReduceTaskRetry_Exhausted(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: alwaysFailReduce,
		NumReduce:  1,
		MaxRetries: 2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error for exhausted reduce retries")
	}
}

func TestRun_ReduceTaskRetry_Success(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: failNTimesReduce(1),
		NumReduce:  1,
		MaxRetries: 2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello world"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := result.([]interface{})
	if len(results) == 0 {
		t.Error("expected non-empty results")
	}
}

func TestRun_ZeroRetries(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error for zero retries with failing map")
	}
}

func TestRun_ContextCancellation(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(5 * time.Second)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "doc1", Value: 1},
	}

	mr.SetInput(input)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := mr.Run(ctx)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestRun_Cancel(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(5 * time.Second)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "doc1", Value: 1},
	}

	mr.SetInput(input)

	go func() {
		time.Sleep(50 * time.Millisecond)
		mr.Cancel()
	}()

	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error after cancel")
	}
}

func TestRun_AlreadyRunning(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(200 * time.Millisecond)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: 1},
	}
	mr.SetInput(input)

	var runErr error
	var runMu sync.Mutex
	go func() {
		runMu.Lock()
		defer runMu.Unlock()
		_, runErr = mr.Run(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	_, err := mr.Run(context.Background())
	if err != ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	runMu.Lock()
	if runErr != nil {
		t.Errorf("first run should succeed, got error: %v", runErr)
	}
	runMu.Unlock()
}

func TestRun_CompletedMapCount(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   2,
		ShuffleMode: ShuffleSync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c"},
		{Key: "doc2", Value: "d e f"},
		{Key: "doc3", Value: "g h i"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mr.CompletedMapCount() != 3 {
		t.Errorf("expected CompletedMapCount=3, got %d", mr.CompletedMapCount())
	}
}

func TestRun_CompletedReduceCount(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   3,
		ShuffleMode: ShuffleSync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mr.CompletedReduceCount() != 3 {
		t.Errorf("expected CompletedReduceCount=3, got %d", mr.CompletedReduceCount())
	}
}

func TestRun_StatusTransitions(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	if mr.Status() != TaskStatusPending {
		t.Errorf("expected initial status=Pending, got %d", mr.Status())
	}

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}
	mr.SetInput(input)

	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mr.Status() != TaskStatusCompleted {
		t.Errorf("expected status=Completed after run, got %d", mr.Status())
	}
}

func TestRun_MultipleReduceTasks(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  4,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "alpha beta gamma delta epsilon"},
		{Key: "doc2", Value: "alpha gamma zeta"},
		{Key: "doc3", Value: "beta delta eta"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["alpha"] != 2 {
		t.Errorf("expected alpha=2, got %d", wc["alpha"])
	}
	if wc["beta"] != 2 {
		t.Errorf("expected beta=2, got %d", wc["beta"])
	}
	if wc["gamma"] != 2 {
		t.Errorf("expected gamma=2, got %d", wc["gamma"])
	}
	if wc["delta"] != 2 {
		t.Errorf("expected delta=2, got %d", wc["delta"])
	}
	if wc["epsilon"] != 1 {
		t.Errorf("expected epsilon=1, got %d", wc["epsilon"])
	}
}

func TestRun_EmptyMapOutput(t *testing.T) {
	emptyMapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		return []KeyValue{}, nil
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    emptyMapFunc,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := result.([]interface{})
	for _, r := range results {
		if r != nil {
			kvs := r.([]KeyValue)
			if len(kvs) > 0 {
				t.Errorf("expected no key-value pairs for empty map output, got %v", kvs)
			}
		}
	}
}

func TestRun_ConcurrentMapTasks(t *testing.T) {
	var concurrentCount int64
	var maxConcurrent int64

	slowMapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		current := atomic.AddInt64(&concurrentCount, 1)
		defer atomic.AddInt64(&concurrentCount, -1)

		for {
			max := atomic.LoadInt64(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt64(&maxConcurrent, max, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		return wordCountMap(context.Background(), key, value)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    slowMapFunc,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	input := make([]KeyValue, 10)
	for i := 0; i < 10; i++ {
		input[i] = KeyValue{Key: fmt.Sprintf("doc%d", i), Value: "hello world"}
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	max := atomic.LoadInt64(&maxConcurrent)
	if max < 2 {
		t.Errorf("expected some concurrent map execution, max concurrent=%d", max)
	}
}

func TestHashPartition_Deterministic(t *testing.T) {
	p1 := HashPartition("hello", 4)
	p2 := HashPartition("hello", 4)
	if p1 != p2 {
		t.Errorf("expected same partition for same key, got %d and %d", p1, p2)
	}

	p3 := HashPartition("world", 4)
	if p3 < 0 || p3 >= 4 {
		t.Errorf("partition out of range: %d", p3)
	}
}

func TestHashPartition_Distribution(t *testing.T) {
	numPartitions := 4
	counts := make(map[int]int)

	keys := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j",
		"k", "l", "m", "n", "o", "p", "q", "r", "s", "t"}

	for _, key := range keys {
		p := HashPartition(key, numPartitions)
		counts[p]++
	}

	for p := 0; p < numPartitions; p++ {
		if counts[p] == 0 {
			t.Logf("warning: partition %d has no keys from test set", p)
		}
	}
}

func TestRun_SetInputWhileRunning(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(200 * time.Millisecond)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	mr.SetInput([]KeyValue{{Key: "doc1", Value: 1}})

	var runErr error
	var runMu sync.Mutex
	go func() {
		runMu.Lock()
		defer runMu.Unlock()
		_, runErr = mr.Run(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	mr.SetInput([]KeyValue{{Key: "doc2", Value: 2}})

	time.Sleep(300 * time.Millisecond)

	runMu.Lock()
	if runErr != nil {
		t.Errorf("run should succeed: %v", runErr)
	}
	runMu.Unlock()
}

func TestRun_TaskErrors(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}
	mr.SetInput(input)

	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error")
	}

	taskErrors := mr.Errors()
	if len(taskErrors) == 0 {
		t.Error("expected task errors to be recorded")
	}

	for _, te := range taskErrors {
		if te.TaskType != "map" {
			t.Errorf("expected map task error, got %s", te.TaskType)
		}
		if te.Err == nil {
			t.Error("expected non-nil inner error")
		}
	}
}

func TestRun_TaskErrorUnwrap(t *testing.T) {
	innerErr := errors.New("inner error")
	te := &TaskError{
		TaskType: "map",
		TaskID:   0,
		Attempt:  1,
		Err:      innerErr,
	}

	if !errors.Is(te, innerErr) {
		t.Error("expected errors.Is to match inner error")
	}
}

func TestRun_MergeFuncError(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MergeFunc: func(results []interface{}) (interface{}, error) {
			return nil, errors.New("merge error")
		},
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}
	mr.SetInput(input)

	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected merge error")
	}
}

func TestRun_LargeInput(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  4,
	})

	input := make([]KeyValue, 50)
	for i := 0; i < 50; i++ {
		input[i] = KeyValue{
			Key:   fmt.Sprintf("doc%d", i),
			Value: fmt.Sprintf("word%d word%d word%d", i%5, (i+1)%5, (i+2)%5),
		}
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := result.([]interface{})
	if len(results) != 4 {
		t.Errorf("expected 4 reduce results, got %d", len(results))
	}
}

func TestRun_MixedSuccessAndFailure(t *testing.T) {
	var mapCalls int32
	mixedMapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		_ = atomic.AddInt32(&mapCalls, 1)
		if key == "fail-doc" {
			return nil, errors.New("intentional failure")
		}
		return wordCountMap(context.Background(), key, value)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    mixedMapFunc,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "ok-doc1", Value: "hello"},
		{Key: "fail-doc", Value: "world"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error when one map task fails")
	}
}

func TestRun_SyncShuffle_ManyPartitions(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   8,
		ShuffleMode: ShuffleSync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c d e f g h i j"},
		{Key: "doc2", Value: "k l m n o p q r s t"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)

	totalWords := 0
	for _, count := range wc {
		totalWords += count
	}

	if totalWords != 20 {
		t.Errorf("expected total 20 word counts, got %d", totalWords)
	}
}

func TestRun_AsyncShuffle_ManyPartitions(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   8,
		ShuffleMode: ShuffleAsync,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "a b c d e f g h i j"},
		{Key: "doc2", Value: "k l m n o p q r s t"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)

	totalWords := 0
	for _, count := range wc {
		totalWords += count
	}

	if totalWords != 20 {
		t.Errorf("expected total 20 word counts, got %d", totalWords)
	}
}

func TestRun_IntegerSum(t *testing.T) {
	intMap := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		nums := value.([]int)
		var kvs []KeyValue
		for _, n := range nums {
			kvs = append(kvs, KeyValue{Key: key, Value: n})
		}
		return kvs, nil
	}

	intReduce := func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		sum := 0
		for _, v := range values {
			sum += v.(int)
		}
		return sum, nil
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    intMap,
		ReduceFunc: intReduce,
		NumReduce:  2,
		MergeFunc: func(results []interface{}) (interface{}, error) {
			total := 0
			for _, r := range results {
				if r == nil {
					continue
				}
				kvs := r.([]KeyValue)
				for _, kv := range kvs {
					total += kv.Value.(int)
				}
			}
			return total, nil
		},
	})

	input := []KeyValue{
		{Key: "nums", Value: []int{1, 2, 3, 4, 5}},
		{Key: "nums", Value: []int{6, 7, 8, 9, 10}},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	total := result.(int)
	if total != 55 {
		t.Errorf("expected total=55, got %d", total)
	}
}

func TestRun_SortAndDedup(t *testing.T) {
	sortMap := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		items := value.([]string)
		sort.Strings(items)
		var kvs []KeyValue
		for _, item := range items {
			kvs = append(kvs, KeyValue{Key: item, Value: true})
		}
		return kvs, nil
	}

	uniqueReduce := func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		return key, nil
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    sortMap,
		ReduceFunc: uniqueReduce,
		NumReduce:  2,
		MergeFunc: func(results []interface{}) (interface{}, error) {
			var items []string
			for _, r := range results {
				if r == nil {
					continue
				}
				kvs := r.([]KeyValue)
				for _, kv := range kvs {
					if s, ok := kv.Value.(string); ok {
						items = append(items, s)
					}
				}
			}
			sort.Strings(items)
			return items, nil
		},
	})

	input := []KeyValue{
		{Key: "list1", Value: []string{"banana", "apple", "cherry", "apple"}},
		{Key: "list2", Value: []string{"date", "apple", "cherry"}},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items := result.([]string)
	expected := []string{"apple", "banana", "cherry", "date"}

	if len(items) != len(expected) {
		t.Fatalf("expected %d items, got %d: %v", len(expected), len(items), items)
	}

	for i, item := range items {
		if item != expected[i] {
			t.Errorf("expected item[%d]=%s, got %s", i, expected[i], item)
		}
	}
}

func TestRun_PanicRecovery(t *testing.T) {
	panicMap := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		if key == "panic-doc" {
			panic("something went wrong")
		}
		return wordCountMap(context.Background(), key, value)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    panicMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "panic-doc", Value: "hello"},
	}

	mr.SetInput(input)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("expected panic to be handled, but it propagated: %v", r)
		}
	}()

	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error from panicked map task")
	}
}

func TestRun_MultipleMapFailures(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
		MaxRetries: 1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
		{Key: "doc2", Value: "world"},
		{Key: "doc3", Value: "foo"},
	}

	mr.SetInput(input)
	_, err := mr.Run(context.Background())
	if err == nil {
		t.Error("expected error when all map tasks fail")
	}

	taskErrors := mr.Errors()
	if len(taskErrors) != 3 {
		t.Errorf("expected 3 task errors, got %d", len(taskErrors))
	}
}

func TestRun_RetryResetsComputation(t *testing.T) {
	var attemptData []string
	var mu sync.Mutex

	trackingMap := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		mu.Lock()
		attemptData = append(attemptData, fmt.Sprintf("key=%s", key))
		mu.Unlock()
		return nil, errors.New("fail")
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    trackingMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	_, _ = mr.Run(context.Background())

	mu.Lock()
	count := len(attemptData)
	mu.Unlock()

	if count != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", count)
	}
}

func TestRun_ResultMethod(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	storedResult, storedErr := mr.Result()
	if storedErr != nil {
		t.Errorf("unexpected stored error: %v", storedErr)
	}
	if storedResult == nil {
		t.Error("expected non-nil stored result")
	}

	results := result.([]interface{})
	storedResults := storedResult.([]interface{})
	if len(results) != len(storedResults) {
		t.Errorf("result length mismatch: %d vs %d", len(results), len(storedResults))
	}
}

func TestRun_DefaultMergeConcatenation(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello world"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results, ok := result.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", result)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results from 2 reduce tasks, got %d", len(results))
	}
}

func TestRun_SyncVsAsyncSameResults(t *testing.T) {
	input := []KeyValue{
		{Key: "doc1", Value: "hello world hello"},
		{Key: "doc2", Value: "world go go go"},
		{Key: "doc3", Value: "hello go world"},
	}

	mrSync, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   3,
		ShuffleMode: ShuffleSync,
	})
	mrSync.SetInput(input)
	syncResult, err := mrSync.Run(context.Background())
	if err != nil {
		t.Fatalf("sync run failed: %v", err)
	}
	syncWC := collectWordCounts(syncResult)

	mrAsync, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   3,
		ShuffleMode: ShuffleAsync,
	})
	mrAsync.SetInput(input)
	asyncResult, err := mrAsync.Run(context.Background())
	if err != nil {
		t.Fatalf("async run failed: %v", err)
	}
	asyncWC := collectWordCounts(asyncResult)

	if len(syncWC) != len(asyncWC) {
		t.Errorf("word count key count mismatch: sync=%d, async=%d", len(syncWC), len(asyncWC))
	}

	for key, syncCount := range syncWC {
		asyncCount, ok := asyncWC[key]
		if !ok {
			t.Errorf("key %s missing from async results", key)
			continue
		}
		if syncCount != asyncCount {
			t.Errorf("count mismatch for key %s: sync=%d, async=%d", key, syncCount, asyncCount)
		}
	}
}

func TestRun_NilMapOutput(t *testing.T) {
	nilMapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		return nil, nil
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:    nilMapFunc,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	input := []KeyValue{
		{Key: "doc1", Value: "hello"},
	}

	mr.SetInput(input)
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	results := result.([]interface{})
	for _, r := range results {
		if r != nil {
			t.Errorf("expected nil partition result for nil map output, got %v", r)
		}
	}
}

func TestRun_CancelStatus(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(5 * time.Second)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	input := []KeyValue{
		{Key: "doc1", Value: 1},
	}

	mr.SetInput(input)

	go func() {
		time.Sleep(50 * time.Millisecond)
		mr.Cancel()
	}()

	_, _ = mr.Run(context.Background())

	if mr.Status() != TaskStatusFailed {
		t.Errorf("expected TaskStatusFailed after cancel, got %d", mr.Status())
	}
}

func TestRun_TaskErrorString(t *testing.T) {
	te := &TaskError{
		TaskType: "reduce",
		TaskID:   3,
		Attempt:  2,
		Err:      errors.New("some error"),
	}

	s := te.Error()
	if !strings.Contains(s, "reduce") {
		t.Errorf("error string should contain task type: %s", s)
	}
	if !strings.Contains(s, "3") {
		t.Errorf("error string should contain task id: %s", s)
	}
	if !strings.Contains(s, "some error") {
		t.Errorf("error string should contain inner error: %s", s)
	}
}

func TestRun_DoneChannel_ClosesAfterRun(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello world"},
	})

	done := mr.Done()

	select {
	case <-done:
		t.Error("Done channel should not be closed before Run()")
	default:
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
	}()

	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed after Run() completes")
	}

	_ = result
}

func TestRun_DoneChannel_ClosesAfterError(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello"},
	})

	done := mr.Done()

	_, _ = mr.Run(context.Background())

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed after Run() returns error")
	}
}

func TestRun_DoneChannel_ClosesAfterCancel(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc: func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
			time.Sleep(2 * time.Second)
			return []KeyValue{{Key: key, Value: value}}, nil
		},
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: 1},
	})

	done := mr.Done()

	go func() {
		time.Sleep(50 * time.Millisecond)
		mr.Cancel()
	}()

	_, _ = mr.Run(context.Background())

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed after cancel")
	}
}

func TestRun_DoneChannel_AsyncShuffle(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:     wordCountMap,
		ReduceFunc:  wordCountReduce,
		NumReduce:   2,
		ShuffleMode: ShuffleAsync,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello world hello"},
		{Key: "doc2", Value: "world go world"},
	})

	done := mr.Done()

	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Error("Done channel should be closed after async shuffle Run() completes")
	}

	wc := collectWordCounts(result)
	if wc["hello"] != 2 {
		t.Errorf("expected hello=2, got %d", wc["hello"])
	}
}

func TestRun_DoneChannel_NotClosedBeforeRun(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	done := mr.Done()

	select {
	case <-done:
		t.Error("Done channel should not be closed before Run() is called")
	default:
	}
}

func TestRun_AsyncShuffle_ProcessesDataIncrementally(t *testing.T) {
	var mu sync.Mutex
	var mapCompleteTimes []time.Time
	var reduceStartTimes []time.Time

	slowMapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		delay := value.(time.Duration)
		time.Sleep(delay)
		mu.Lock()
		mapCompleteTimes = append(mapCompleteTimes, time.Now())
		mu.Unlock()
		return wordCountMap(context.Background(), key, "hello world")
	}

	reducedStarted := int32(0)
	trackingReduceFunc := func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		start := atomic.AddInt32(&reducedStarted, 1)
		if start == 1 {
			mu.Lock()
			reduceStartTimes = append(reduceStartTimes, time.Now())
			mu.Unlock()
		}
		return wordCountReduce(context.Background(), key, values)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:     slowMapFunc,
		ReduceFunc:  trackingReduceFunc,
		NumReduce:   2,
		ShuffleMode: ShuffleAsync,
		MaxRetries:  0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: 80 * time.Millisecond},
		{Key: "doc2", Value: 80 * time.Millisecond},
		{Key: "doc3", Value: 200 * time.Millisecond},
	})

	start := time.Now()
	result, err := mr.Run(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wc := collectWordCounts(result)
	if wc["hello"] != 3 {
		t.Errorf("expected hello=3, got %d", wc["hello"])
	}
	if wc["world"] != 3 {
		t.Errorf("expected world=3, got %d", wc["world"])
	}

	t.Logf("Total elapsed: %v", elapsed)
	mu.Lock()
	for i, mt := range mapCompleteTimes {
		t.Logf("Map %d completed at: %v", i, mt.Sub(start))
	}
	for i, rt := range reduceStartTimes {
		t.Logf("Reduce %d started at: %v", i, rt.Sub(start))
	}
	mu.Unlock()
}

func TestRun_StatusIsFailedOnError(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello"},
	})

	_, runErr := mr.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected error from run")
	}

	if mr.Status() != TaskStatusFailed {
		t.Errorf("expected TaskStatusFailed after error, got %d", mr.Status())
	}
}

func TestRun_StatusIsCompletedOnSuccess(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello world"},
	})

	_, runErr := mr.Run(context.Background())
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if mr.Status() != TaskStatusCompleted {
		t.Errorf("expected TaskStatusCompleted on success, got %d", mr.Status())
	}
}

func TestRun_StatusIsFailedOnReduceError(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: alwaysFailReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello"},
	})

	_, runErr := mr.Run(context.Background())
	if runErr == nil {
		t.Fatal("expected reduce error")
	}

	if mr.Status() != TaskStatusFailed {
		t.Errorf("expected TaskStatusFailed after reduce error, got %d", mr.Status())
	}
}

func TestRun_RepeatedRuns_NoPanic(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  2,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello world"},
	})

	result1, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	wc1 := collectWordCounts(result1)
	if wc1["hello"] != 1 || wc1["world"] != 1 {
		t.Errorf("first run wrong counts: %v", wc1)
	}

	<-mr.Done()

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "foo bar foo"},
		{Key: "doc2", Value: "bar baz"},
	})

	result2, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("second run failed: %v", err)
	}
	wc2 := collectWordCounts(result2)
	if wc2["foo"] != 2 || wc2["bar"] != 2 || wc2["baz"] != 1 {
		t.Errorf("second run wrong counts: %v", wc2)
	}

	<-mr.Done()
}

func TestRun_RepeatedRuns_WithErrorInBetween(t *testing.T) {
	mr, _ := NewMapReduce(Config{
		MapFunc:    wordCountMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "hello"},
	})
	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("first run failed: %v", err)
	}
	if mr.Status() != TaskStatusCompleted {
		t.Errorf("first run: expected Completed, got %d", mr.Status())
	}

	mr.cfg.MaxRetries = 0
	mr2, _ := NewMapReduce(Config{
		MapFunc:    alwaysFailMap,
		ReduceFunc: wordCountReduce,
		NumReduce:  1,
		MaxRetries: 0,
	})
	mr2.SetInput([]KeyValue{
		{Key: "doc1", Value: "test"},
	})
	_, err = mr2.Run(context.Background())
	if err == nil {
		t.Fatal("expected error on second instance run")
	}
	if mr2.Status() != TaskStatusFailed {
		t.Errorf("second instance: expected Failed, got %d", mr2.Status())
	}

	mr.SetInput([]KeyValue{
		{Key: "doc1", Value: "world"},
	})
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("third run (first instance) failed: %v", err)
	}
	if mr.Status() != TaskStatusCompleted {
		t.Errorf("third run: expected Completed, got %d", mr.Status())
	}
	wc := collectWordCounts(result)
	if wc["world"] != 1 {
		t.Errorf("third run wrong counts: %v", wc)
	}
}

func TestRun_AsyncShuffle_ReduceStartsBeforeAllMapsComplete(t *testing.T) {
	var reduceStartedMu sync.Mutex
	var firstReduceStart time.Time
	firstReduceStartSet := false

	var mapReportTimes [4]time.Time
	var mapReportMu sync.Mutex

	p0Key := ""
	p1Key := ""
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("fastkey_p0_%d", i)
		if HashPartition(k, 2) == 0 && p0Key == "" {
			p0Key = k
		}
		k2 := fmt.Sprintf("slowkey_p1_%d", i)
		if HashPartition(k2, 2) == 1 && p1Key == "" {
			p1Key = k2
		}
		if p0Key != "" && p1Key != "" {
			break
		}
	}

	if p0Key == "" || p1Key == "" {
		t.Skip("could not find suitable keys for partition test")
		return
	}

	mapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		docIdx := value.(int)

		var result []KeyValue
		if docIdx == 3 {
			time.Sleep(250 * time.Millisecond)
			result = []KeyValue{
				{Key: p1Key, Value: 1},
			}
		} else {
			time.Sleep(30 * time.Millisecond)
			result = []KeyValue{
				{Key: p0Key, Value: 1},
			}
		}

		mapReportMu.Lock()
		mapReportTimes[docIdx] = time.Now()
		mapReportMu.Unlock()
		return result, nil
	}

	reduceStartRecorded := int32(0)
	reduceFunc := func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		if HashPartition(key, 2) == 0 && atomic.CompareAndSwapInt32(&reduceStartRecorded, 0, 1) {
			reduceStartedMu.Lock()
			firstReduceStart = time.Now()
			firstReduceStartSet = true
			reduceStartedMu.Unlock()
		}
		return wordCountReduce(context.Background(), key, values)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:     mapFunc,
		ReduceFunc:  reduceFunc,
		NumReduce:   2,
		ShuffleMode: ShuffleAsync,
		MaxRetries:  0,
	})

	input := []KeyValue{
		{Key: "doc0", Value: 0},
		{Key: "doc1", Value: 1},
		{Key: "doc2", Value: 2},
		{Key: "doc3", Value: 3},
	}
	mr.SetInput(input)

	start := time.Now()
	_, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	reduceStartedMu.Lock()
	startSet := firstReduceStartSet
	reduceStart := firstReduceStart
	reduceStartedMu.Unlock()

	if !startSet {
		t.Fatal("Partition 0 Reduce start time was never recorded")
	}

	reduceStartOffset := reduceStart.Sub(start)

	mapReportMu.Lock()
	lastReportTime := mapReportTimes[0]
	for i := 1; i < 4; i++ {
		if mapReportTimes[i].After(lastReportTime) {
			lastReportTime = mapReportTimes[i]
		}
	}
	mapReportMu.Unlock()
	lastReportOffset := lastReportTime.Sub(start)

	reduceAfterReport := reduceStart.Sub(lastReportTime)

	t.Logf("Last map reported at: %v", lastReportOffset)
	t.Logf("Partition 0 reduce started at: %v", reduceStartOffset)
	t.Logf("Reduce started %v after last map report", reduceAfterReport)

	if reduceAfterReport > 50*time.Millisecond {
		t.Errorf("Reduce started %v after last map report, should start immediately (< 50ms)", reduceAfterReport)
	}
}

func TestRun_AsyncShuffle_ReduceStartsImmediatelyAfterLastMapReport(t *testing.T) {
	var reduceStartedMu sync.Mutex
	reduceStartTimesByPartition := make(map[int]time.Time)

	p0Key := ""
	p1Key := ""
	for i := 0; i < 1000; i++ {
		k := fmt.Sprintf("findkey_p0_%d", i)
		if HashPartition(k, 2) == 0 && p0Key == "" {
			p0Key = k
		}
		k2 := fmt.Sprintf("findkey_p1_%d", i)
		if HashPartition(k2, 2) == 1 && p1Key == "" {
			p1Key = k2
		}
		if p0Key != "" && p1Key != "" {
			break
		}
	}
	if p0Key == "" || p1Key == "" {
		t.Skip("could not find keys for both partitions")
		return
	}

	var mapReportMu sync.Mutex
	mapReportTimes := make([]time.Time, 4)

	mapFunc := func(_ context.Context, key string, value interface{}) ([]KeyValue, error) {
		docIdx := value.(int)
		var result []KeyValue
		if docIdx < 3 {
			time.Sleep(30 * time.Millisecond)
			result = []KeyValue{{Key: p0Key, Value: docIdx}}
		} else {
			time.Sleep(150 * time.Millisecond)
			result = []KeyValue{{Key: p1Key, Value: docIdx}}
		}
		mapReportMu.Lock()
		mapReportTimes[docIdx] = time.Now()
		mapReportMu.Unlock()
		return result, nil
	}

	reduceFunc := func(_ context.Context, key string, values []interface{}) (interface{}, error) {
		partition := HashPartition(key, 2)
		reduceStartedMu.Lock()
		if _, ok := reduceStartTimesByPartition[partition]; !ok {
			reduceStartTimesByPartition[partition] = time.Now()
		}
		reduceStartedMu.Unlock()
		return wordCountReduce(context.Background(), key, values)
	}

	mr, _ := NewMapReduce(Config{
		MapFunc:     mapFunc,
		ReduceFunc:  reduceFunc,
		NumReduce:   2,
		ShuffleMode: ShuffleAsync,
		MaxRetries:  0,
	})

	input := []KeyValue{
		{Key: "doc0", Value: 0},
		{Key: "doc1", Value: 1},
		{Key: "doc2", Value: 2},
		{Key: "doc3", Value: 3},
	}
	mr.SetInput(input)

	start := time.Now()
	result, err := mr.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_ = result

	reduceStartedMu.Lock()
	p0Start, p0ok := reduceStartTimesByPartition[0]
	p1Start, p1ok := reduceStartTimesByPartition[1]
	reduceStartedMu.Unlock()

	if !p0ok {
		t.Fatal("partition 0 reduce start not recorded")
	}
	if !p1ok {
		t.Fatal("partition 1 reduce start not recorded")
	}

	mapReportMu.Lock()
	doc3ReportTime := mapReportTimes[3].Sub(start)
	mapReportMu.Unlock()

	p0Offset := p0Start.Sub(start)
	p1Offset := p1Start.Sub(start)

	t.Logf("doc3 map reported at: %v", doc3ReportTime)
	t.Logf("Partition 0 reduce start: %v (after doc3 by %v)", p0Offset, p0Start.Sub(mapReportTimes[3]))
	t.Logf("Partition 1 reduce start: %v (after doc3 by %v)", p1Offset, p1Start.Sub(mapReportTimes[3]))

	p0AfterDoc3 := p0Start.Sub(mapReportTimes[3])
	if p0AfterDoc3 > 50*time.Millisecond {
		t.Errorf("Partition 0 reduce started %v after doc3 reported, should start immediately (< 50ms)", p0AfterDoc3)
	}
}
