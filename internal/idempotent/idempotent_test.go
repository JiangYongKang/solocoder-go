package idempotent

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewIdempotent(t *testing.T) {
	i := NewIdempotent()
	if i == nil {
		t.Fatal("NewIdempotent returned nil")
	}
	count, err := i.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected Count=0, got %d", count)
	}
}

func TestNewIdempotentWithConfig_Defaults(t *testing.T) {
	i, err := NewIdempotentWithConfig(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i == nil {
		t.Fatal("NewIdempotentWithConfig returned nil")
	}
	if i.cfg.TTL <= 0 {
		t.Error("TTL should have default value")
	}
	if i.cfg.CleanInterval <= 0 {
		t.Error("CleanInterval should have default value")
	}
	if i.cfg.KeyHeader == "" {
		t.Error("KeyHeader should have default value")
	}
}

func TestNewIdempotentWithConfig_InvalidConfig(t *testing.T) {
	_, err := NewIdempotentWithConfig(Config{
		TTL: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative TTL, got %v", err)
	}

	_, err = NewIdempotentWithConfig(Config{
		CleanInterval: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative CleanInterval, got %v", err)
	}
}

func TestNewIdempotentWithConfig_CleanIntervalFromTTL(t *testing.T) {
	ttl := 10 * time.Second
	i, err := NewIdempotentWithConfig(Config{
		TTL: ttl,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := ttl / 5
	if i.cfg.CleanInterval != expected {
		t.Errorf("expected CleanInterval=%v, got %v", expected, i.cfg.CleanInterval)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.TTL != 5*time.Minute {
		t.Errorf("expected TTL=5m, got %v", cfg.TTL)
	}
	if cfg.CleanInterval != 1*time.Minute {
		t.Errorf("expected CleanInterval=1m, got %v", cfg.CleanInterval)
	}
	if cfg.KeyHeader != "X-Idempotency-Key" {
		t.Errorf("expected KeyHeader=X-Idempotency-Key, got %v", cfg.KeyHeader)
	}
}

func TestExecute_FirstRequest(t *testing.T) {
	i := NewIdempotent()
	handler := func() Response {
		return Response{
			StatusCode: http.StatusOK,
			Body:       []byte("hello world"),
			Header:     http.Header{"X-Custom": []string{"value"}},
		}
	}

	resp, fromCache, err := i.Execute("key-1", handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fromCache {
		t.Error("expected first request to be MISS (fromCache=false)")
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected StatusCode=%d, got %d", http.StatusOK, resp.StatusCode)
	}
	if string(resp.Body) != "hello world" {
		t.Errorf("expected Body='hello world', got '%s'", string(resp.Body))
	}
	if resp.Header.Get("X-Custom") != "value" {
		t.Errorf("expected Header X-Custom='value', got '%s'", resp.Header.Get("X-Custom"))
	}

	count, _ := i.Count()
	if count != 1 {
		t.Errorf("expected Count=1, got %d", count)
	}
}

func TestExecute_CacheHit(t *testing.T) {
	i := NewIdempotent()
	callCount := 0
	handler := func() Response {
		callCount++
		return Response{
			StatusCode: http.StatusOK,
			Body:       []byte("cached response"),
			Header:     http.Header{"Content-Type": []string{"application/json"}},
		}
	}

	resp1, fromCache1, err := i.Execute("key-cache", handler)
	if err != nil {
		t.Fatalf("first Execute error: %v", err)
	}
	if fromCache1 {
		t.Error("first request should be MISS")
	}
	if callCount != 1 {
		t.Errorf("expected callCount=1, got %d", callCount)
	}

	resp2, fromCache2, err := i.Execute("key-cache", handler)
	if err != nil {
		t.Fatalf("second Execute error: %v", err)
	}
	if !fromCache2 {
		t.Error("second request should be HIT")
	}
	if callCount != 1 {
		t.Errorf("handler should not be called again, callCount=%d", callCount)
	}
	if resp2.StatusCode != resp1.StatusCode {
		t.Errorf("StatusCode mismatch: %d vs %d", resp1.StatusCode, resp2.StatusCode)
	}
	if string(resp2.Body) != string(resp1.Body) {
		t.Errorf("Body mismatch: '%s' vs '%s'", string(resp1.Body), string(resp2.Body))
	}
	if resp2.Header.Get("Content-Type") != "application/json" {
		t.Errorf("expected Header Content-Type='application/json', got '%s'", resp2.Header.Get("Content-Type"))
	}
}

func TestExecute_EmptyKey(t *testing.T) {
	i := NewIdempotent()
	handler := func() Response {
		return Response{StatusCode: http.StatusOK, Body: []byte("test")}
	}

	_, _, err := i.Execute("", handler)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestExecute_NilHandler(t *testing.T) {
	i := NewIdempotent()

	_, _, err := i.Execute("key-1", nil)
	if !errors.Is(err, ErrHandlerNil) {
		t.Errorf("expected ErrHandlerNil, got %v", err)
	}
}

func TestExecute_MultipleKeys(t *testing.T) {
	i := NewIdempotent()
	n := 50

	for j := 0; j < 2; j++ {
		for k := 0; k < n; k++ {
			key := fmt.Sprintf("multi-key-%d", k)
			handler := func(k int) func() Response {
				return func() Response {
					return Response{
						StatusCode: http.StatusOK,
						Body:       []byte(fmt.Sprintf("response-%d", k)),
						Header:     http.Header{"X-Key": []string{fmt.Sprintf("k-%d", k)}},
					}
				}
			}(k)

			_, fromCache, err := i.Execute(key, handler)
			if err != nil {
				t.Fatalf("Execute error for key %s: %v", key, err)
			}
			if j == 0 && fromCache {
				t.Errorf("iteration %d, key %s should be MISS", j, key)
			}
			if j == 1 && !fromCache {
				t.Errorf("iteration %d, key %s should be HIT", j, key)
			}
		}
	}

	count, _ := i.Count()
	if count != n {
		t.Errorf("expected Count=%d, got %d", n, count)
	}
}

func TestExecute_DifferentStatusCodes(t *testing.T) {
	i := NewIdempotent()

	handler200 := func() Response {
		return Response{StatusCode: http.StatusOK, Body: []byte("ok")}
	}
	handler404 := func() Response {
		return Response{StatusCode: http.StatusNotFound, Body: []byte("not found")}
	}
	handler500 := func() Response {
		return Response{StatusCode: http.StatusInternalServerError, Body: []byte("error")}
	}

	resp200, _, _ := i.Execute("key-200", handler200)
	resp404, _, _ := i.Execute("key-404", handler404)
	resp500, _, _ := i.Execute("key-500", handler500)

	if resp200.StatusCode != http.StatusOK || string(resp200.Body) != "ok" {
		t.Error("200 response mismatch")
	}
	if resp404.StatusCode != http.StatusNotFound || string(resp404.Body) != "not found" {
		t.Error("404 response mismatch")
	}
	if resp500.StatusCode != http.StatusInternalServerError || string(resp500.Body) != "error" {
		t.Error("500 response mismatch")
	}
}

func TestExecute_ConcurrentSameKey(t *testing.T) {
	i := NewIdempotent()
	i.Start()
	defer i.Stop()

	var callCount int64
	numGoroutines := 20

	handler := func() Response {
		atomic.AddInt64(&callCount, 1)
		time.Sleep(50 * time.Millisecond)
		return Response{
			StatusCode: http.StatusOK,
			Body:       []byte("concurrent result"),
			Header:     http.Header{"X-Concurrent": []string{"true"}},
		}
	}

	var wg sync.WaitGroup
	results := make([]struct {
		resp      Response
		fromCache bool
		err       error
	}, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			resp, fc, err := i.Execute("concurrent-key", handler)
			results[idx].resp = resp
			results[idx].fromCache = fc
			results[idx].err = err
		}(g)
	}

	wg.Wait()

	if callCount != 1 {
		t.Errorf("expected handler to be called exactly once, got %d", callCount)
	}

	for idx, r := range results {
		if r.err != nil {
			t.Errorf("goroutine %d error: %v", idx, r.err)
			continue
		}
		if r.resp.StatusCode != http.StatusOK {
			t.Errorf("goroutine %d: expected status %d, got %d", idx, http.StatusOK, r.resp.StatusCode)
		}
		if string(r.resp.Body) != "concurrent result" {
			t.Errorf("goroutine %d: expected body 'concurrent result', got '%s'", idx, string(r.resp.Body))
		}
		if r.resp.Header.Get("X-Concurrent") != "true" {
			t.Errorf("goroutine %d: expected header X-Concurrent='true', got '%s'", idx, r.resp.Header.Get("X-Concurrent"))
		}
	}
}

func TestExecute_ConcurrentDifferentKeys(t *testing.T) {
	i := NewIdempotent()
	i.Start()
	defer i.Stop()

	numGoroutines := 20
	keysPerGoroutine := 50

	var wg sync.WaitGroup
	var totalCalls int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for k := 0; k < keysPerGoroutine; k++ {
				key := fmt.Sprintf("g%d-k%d", gid, k)
				handler := func() Response {
					atomic.AddInt64(&totalCalls, 1)
					return Response{StatusCode: http.StatusOK, Body: []byte(key)}
				}
				_, _, err := i.Execute(key, handler)
				if err != nil {
					t.Errorf("goroutine %d key %s error: %v", gid, key, err)
				}
			}
		}(g)
	}

	wg.Wait()

	expected := int64(numGoroutines * keysPerGoroutine)
	if totalCalls != expected {
		t.Errorf("expected %d handler calls, got %d", expected, totalCalls)
	}

	count, _ := i.Count()
	if count != int(expected) {
		t.Errorf("expected Count=%d, got %d", expected, count)
	}
}

func TestExecute_StopDuringHandler_WaitingRequestsReturnError(t *testing.T) {
	i := NewIdempotent()

	handlerStarted := make(chan struct{})
	handlerCanFinish := make(chan struct{})

	handler := func() Response {
		close(handlerStarted)
		<-handlerCanFinish
		return Response{
			StatusCode: http.StatusOK,
			Body:       []byte("should not be used"),
		}
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, err := i.Execute("stop-wait-key", handler)
		if !errors.Is(err, ErrIdempotentStopped) {
			t.Errorf("first request: expected ErrIdempotentStopped, got %v", err)
		}
	}()

	<-handlerStarted

	numWaiters := 10
	waiterErrs := make([]error, numWaiters)
	for w := 0; w < numWaiters; w++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := i.Execute("stop-wait-key", func() Response {
				return Response{StatusCode: http.StatusOK, Body: []byte("waiter")}
			})
			waiterErrs[idx] = err
		}(w)
	}

	time.Sleep(50 * time.Millisecond)

	i.Stop()
	close(handlerCanFinish)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock waiting for goroutines")
	}

	for idx, err := range waiterErrs {
		if !errors.Is(err, ErrIdempotentStopped) {
			t.Errorf("waiter %d: expected ErrIdempotentStopped, got %v", idx, err)
		}
	}
}

func TestExecute_StopWithPendingRequests(t *testing.T) {
	i := NewIdempotent()

	handlerStarted := make(chan struct{})
	handler := func() Response {
		close(handlerStarted)
		time.Sleep(100 * time.Millisecond)
		return Response{
			StatusCode: http.StatusOK,
			Body:       []byte("result"),
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = i.Execute("pending-stop", handler)
	}()

	<-handlerStarted

	wg.Add(1)
	go func() {
		defer wg.Done()
		i.Stop()
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop with pending request deadlocked")
	}
}

func TestGet(t *testing.T) {
	i := NewIdempotent()

	_, ok, err := i.Get("not-exist")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok {
		t.Error("Get should return false for non-existent key")
	}

	header := http.Header{"X-Custom": []string{"val1", "val2"}}
	_ = i.Set("exist-key", http.StatusCreated, []byte("created"), header)
	resp, ok, err := i.Get("exist-key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !ok {
		t.Error("Get should return true for existing key")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected StatusCode=%d, got %d", http.StatusCreated, resp.StatusCode)
	}
	if string(resp.Body) != "created" {
		t.Errorf("expected Body='created', got '%s'", string(resp.Body))
	}
	if resp.Header.Get("X-Custom") != "val1" {
		t.Errorf("expected Header X-Custom='val1', got '%s'", resp.Header.Get("X-Custom"))
	}
	if len(resp.Header.Values("X-Custom")) != 2 {
		t.Errorf("expected 2 X-Custom header values, got %d", len(resp.Header.Values("X-Custom")))
	}

	_, _, err = i.Get("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestGet_HeaderIsolation(t *testing.T) {
	i := NewIdempotent()
	header := http.Header{"X-Shared": []string{"original"}}
	_ = i.Set("iso-key", http.StatusOK, []byte("body"), header)

	resp1, _, _ := i.Get("iso-key")
	resp1.Header.Set("X-Shared", "modified")

	resp2, _, _ := i.Get("iso-key")
	if resp2.Header.Get("X-Shared") != "original" {
		t.Errorf("cached header should be isolated, expected 'original', got '%s'", resp2.Header.Get("X-Shared"))
	}
}

func TestGet_Expired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 30 * time.Millisecond,
	})

	_ = i.Set("expired-key", http.StatusOK, []byte("temp"), nil)

	_, ok, _ := i.Get("expired-key")
	if !ok {
		t.Fatal("key should exist initially")
	}

	time.Sleep(60 * time.Millisecond)

	_, ok, err := i.Get("expired-key")
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if ok {
		t.Error("expired key should not be found")
	}
}

func TestSet(t *testing.T) {
	i := NewIdempotent()

	err := i.Set("set-key", http.StatusOK, []byte("value"), http.Header{"H": []string{"v"}})
	if err != nil {
		t.Fatalf("Set error: %v", err)
	}

	ok, _ := i.Contains("set-key")
	if !ok {
		t.Error("Contains should return true after Set")
	}

	err = i.Set("", http.StatusOK, []byte("value"), nil)
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestSet_Overwrite(t *testing.T) {
	i := NewIdempotent()

	_ = i.Set("overwrite-key", http.StatusOK, []byte("v1"), http.Header{"H": []string{"h1"}})
	_ = i.Set("overwrite-key", http.StatusCreated, []byte("v2"), http.Header{"H": []string{"h2"}})

	resp, ok, _ := i.Get("overwrite-key")
	if !ok {
		t.Fatal("key should exist")
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected StatusCode=%d, got %d", http.StatusCreated, resp.StatusCode)
	}
	if string(resp.Body) != "v2" {
		t.Errorf("expected Body='v2', got '%s'", string(resp.Body))
	}
	if resp.Header.Get("H") != "h2" {
		t.Errorf("expected Header H='h2', got '%s'", resp.Header.Get("H"))
	}
}

func TestDelete(t *testing.T) {
	i := NewIdempotent()

	_ = i.Set("del-key", http.StatusOK, []byte("value"), nil)
	ok, _ := i.Contains("del-key")
	if !ok {
		t.Fatal("key should exist before delete")
	}

	err := i.Delete("del-key")
	if err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	ok, _ = i.Contains("del-key")
	if ok {
		t.Error("key should not exist after delete")
	}

	err = i.Delete("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestContains(t *testing.T) {
	i := NewIdempotent()

	ok, err := i.Contains("not-exist")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if ok {
		t.Error("Contains should return false for non-existent key")
	}

	_ = i.Set("exist-1", http.StatusOK, []byte("val"), nil)
	ok, err = i.Contains("exist-1")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if !ok {
		t.Error("Contains should return true for existing key")
	}

	_, err = i.Contains("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestContains_Expired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 50 * time.Millisecond,
	})

	_ = i.Set("expired-contains", http.StatusOK, []byte("val"), nil)
	exists, _ := i.Contains("expired-contains")
	if !exists {
		t.Fatal("key should be contained initially")
	}

	time.Sleep(100 * time.Millisecond)

	exists, err := i.Contains("expired-contains")
	if err != nil {
		t.Fatalf("Contains error: %v", err)
	}
	if exists {
		t.Error("expired key should not be contained")
	}
}

func TestCount(t *testing.T) {
	i := NewIdempotent()

	count, err := i.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected Count=0, got %d", count)
	}

	for j := 0; j < 10; j++ {
		_ = i.Set(fmt.Sprintf("count-key-%d", j), http.StatusOK, []byte("val"), nil)
	}

	count, _ = i.Count()
	if count != 10 {
		t.Errorf("expected Count=10, got %d", count)
	}
}

func TestCount_ExcludesExpired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 100 * time.Millisecond,
	})

	for j := 0; j < 5; j++ {
		_ = i.Set(fmt.Sprintf("early-%d", j), http.StatusOK, []byte("val"), nil)
	}

	time.Sleep(70 * time.Millisecond)

	for j := 0; j < 5; j++ {
		_ = i.Set(fmt.Sprintf("late-%d", j), http.StatusOK, []byte("val"), nil)
	}

	time.Sleep(50 * time.Millisecond)

	count, err := i.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected Count=5 (only late entries), got %d", count)
	}
}

func TestClear(t *testing.T) {
	i := NewIdempotent()

	for j := 0; j < 50; j++ {
		_ = i.Set(fmt.Sprintf("clear-%d", j), http.StatusOK, []byte("val"), nil)
	}
	count, _ := i.Count()
	if count != 50 {
		t.Fatalf("expected 50 entries, got %d", count)
	}

	err := i.Clear()
	if err != nil {
		t.Fatalf("Clear error: %v", err)
	}
	count, _ = i.Count()
	if count != 0 {
		t.Errorf("expected Count=0 after Clear, got %d", count)
	}

	for j := 0; j < 50; j++ {
		ok, _ := i.Contains(fmt.Sprintf("clear-%d", j))
		if ok {
			t.Errorf("cleared key %d should not exist", j)
		}
	}
}

func TestCleanExpired_NoExpired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 1 * time.Hour,
	})

	for j := 0; j < 10; j++ {
		_ = i.Set(fmt.Sprintf("fresh-%d", j), http.StatusOK, []byte("val"), nil)
	}

	cleaned, err := i.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 0 {
		t.Errorf("expected 0 cleaned, got %d", cleaned)
	}
	count, _ := i.Count()
	if count != 10 {
		t.Errorf("expected Count=10, got %d", count)
	}
}

func TestCleanExpired_AllExpired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 30 * time.Millisecond,
	})

	for j := 0; j < 10; j++ {
		_ = i.Set(fmt.Sprintf("exp-all-%d", j), http.StatusOK, []byte("val"), nil)
	}

	time.Sleep(80 * time.Millisecond)

	cleaned, err := i.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 10 {
		t.Errorf("expected 10 cleaned, got %d", cleaned)
	}
	count, _ := i.Count()
	if count != 0 {
		t.Errorf("expected Count=0 after cleaning all, got %d", count)
	}
}

func TestCleanExpired_PartialExpired(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 100 * time.Millisecond,
	})

	for j := 0; j < 5; j++ {
		_ = i.Set(fmt.Sprintf("early-%d", j), http.StatusOK, []byte("val"), nil)
	}

	time.Sleep(70 * time.Millisecond)

	for j := 0; j < 5; j++ {
		_ = i.Set(fmt.Sprintf("late-%d", j), http.StatusOK, []byte("val"), nil)
	}

	time.Sleep(50 * time.Millisecond)

	cleaned, err := i.CleanExpired()
	if err != nil {
		t.Fatalf("CleanExpired error: %v", err)
	}
	if cleaned != 5 {
		t.Errorf("expected 5 cleaned (early ones), got %d", cleaned)
	}
	count, _ := i.Count()
	if count != 5 {
		t.Errorf("expected Count=5 (late ones remain), got %d", count)
	}

	for j := 0; j < 5; j++ {
		exists, _ := i.Contains(fmt.Sprintf("late-%d", j))
		if !exists {
			t.Errorf("late-%d should still be contained", j)
		}
		exists, _ = i.Contains(fmt.Sprintf("early-%d", j))
		if exists {
			t.Errorf("early-%d should be cleaned", j)
		}
	}
}

func TestExecute_ExpiredThenReexecute(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL: 40 * time.Millisecond,
	})

	callCount := 0
	handler := func() Response {
		callCount++
		return Response{StatusCode: http.StatusOK, Body: []byte("result")}
	}

	_, fromCache1, err := i.Execute("reexec-key", handler)
	if err != nil {
		t.Fatalf("first Execute error: %v", err)
	}
	if fromCache1 {
		t.Fatal("first Execute should be MISS")
	}
	if callCount != 1 {
		t.Errorf("expected callCount=1, got %d", callCount)
	}

	time.Sleep(100 * time.Millisecond)

	_, fromCache2, err := i.Execute("reexec-key", handler)
	if err != nil {
		t.Fatalf("second Execute error: %v", err)
	}
	if fromCache2 {
		t.Error("expired key should be re-executed (MISS)")
	}
	if callCount != 2 {
		t.Errorf("expected callCount=2 after expiry, got %d", callCount)
	}
}

func TestStartStop_Idempotent(t *testing.T) {
	i := NewIdempotent()

	i.Start()
	i.Start()

	i.Stop()
	i.Stop()

	done := make(chan struct{})
	go func() {
		i.Start()
		i.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start/Stop deadlocked")
	}
}

func TestStop_RejectsAllOperations(t *testing.T) {
	i := NewIdempotent()
	i.Start()

	_ = i.Set("before-stop", http.StatusOK, []byte("val"), nil)
	i.Stop()

	handler := func() Response {
		return Response{StatusCode: http.StatusOK, Body: []byte("test")}
	}
	_, _, err := i.Execute("after-stop", handler)
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Execute after Stop, got %v", err)
	}

	_, _, err = i.Get("before-stop")
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Get after Stop, got %v", err)
	}

	err = i.Set("new-key", http.StatusOK, []byte("val"), nil)
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Set after Stop, got %v", err)
	}

	err = i.Delete("before-stop")
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Delete after Stop, got %v", err)
	}

	_, err = i.Contains("before-stop")
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Contains after Stop, got %v", err)
	}

	_, err = i.Count()
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Count after Stop, got %v", err)
	}

	_, err = i.CleanExpired()
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from CleanExpired after Stop, got %v", err)
	}

	err = i.Clear()
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped from Clear after Stop, got %v", err)
	}
}

func TestStop_WithoutStart(t *testing.T) {
	i := NewIdempotent()

	i.Stop()

	handler := func() Response {
		return Response{StatusCode: http.StatusOK, Body: []byte("test")}
	}
	_, _, err := i.Execute("key", handler)
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("expected ErrIdempotentStopped after Stop (without Start), got %v", err)
	}
}

func TestStart_AfterStop(t *testing.T) {
	i := NewIdempotent()
	i.Start()
	i.Stop()

	i.Start()

	handler := func() Response {
		return Response{StatusCode: http.StatusOK, Body: []byte("test")}
	}
	_, _, err := i.Execute("after-restart", handler)
	if !errors.Is(err, ErrIdempotentStopped) {
		t.Errorf("Start after Stop should not revive, expected ErrIdempotentStopped, got %v", err)
	}
}

func TestStartStop_BackgroundCleanup(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL:           30 * time.Millisecond,
		CleanInterval: 20 * time.Millisecond,
	})

	i.Start()
	defer i.Stop()

	for j := 0; j < 20; j++ {
		_ = i.Set(fmt.Sprintf("bg-%d", j), http.StatusOK, []byte("val"), nil)
	}
	count, _ := i.Count()
	if count != 20 {
		t.Fatalf("expected 20 entries, got %d", count)
	}

	time.Sleep(150 * time.Millisecond)

	finalCount, err := i.Count()
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if finalCount != 0 {
		t.Errorf("expected Count=0 after background cleanup, got %d", finalCount)
	}
}

func TestMiddleware_NoKey(t *testing.T) {
	i := NewIdempotent()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	})

	mw := i.Middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if rr.Body.String() != "hello" {
		t.Errorf("expected body 'hello', got '%s'", rr.Body.String())
	}
	if rr.Header().Get("X-Idempotent-Cache") != "" {
		t.Errorf("expected no X-Idempotent-Cache header, got '%s'", rr.Header().Get("X-Idempotent-Cache"))
	}
}

func TestMiddleware_WithKeyFirstRequest(t *testing.T) {
	i := NewIdempotent()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-Custom", "value")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created resource"))
	})

	mw := i.Middleware(handler)

	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("X-Idempotency-Key", "mw-key-1")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	if callCount != 1 {
		t.Errorf("expected callCount=1, got %d", callCount)
	}
	if rr.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}
	if rr.Body.String() != "created resource" {
		t.Errorf("expected body 'created resource', got '%s'", rr.Body.String())
	}
	if rr.Header().Get("X-Idempotent-Cache") != "MISS" {
		t.Errorf("expected X-Idempotent-Cache=MISS, got '%s'", rr.Header().Get("X-Idempotent-Cache"))
	}
	if rr.Header().Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom=value, got '%s'", rr.Header().Get("X-Custom"))
	}
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type=application/json, got '%s'", rr.Header().Get("Content-Type"))
	}
}

func TestMiddleware_WithKeyCacheHit(t *testing.T) {
	i := NewIdempotent()

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("X-Custom", "value")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Location", "/resource/123")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response"))
	})

	mw := i.Middleware(handler)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Idempotency-Key", "mw-cache-key")
	rr1 := httptest.NewRecorder()
	mw.ServeHTTP(rr1, req1)

	if callCount != 1 {
		t.Fatalf("first request: expected callCount=1, got %d", callCount)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Idempotency-Key", "mw-cache-key")
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)

	if callCount != 1 {
		t.Errorf("second request: handler should not be called again, callCount=%d", callCount)
	}
	if rr2.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, rr2.Code)
	}
	if rr2.Body.String() != "response" {
		t.Errorf("expected body 'response', got '%s'", rr2.Body.String())
	}
	if rr2.Header().Get("X-Idempotent-Cache") != "HIT" {
		t.Errorf("expected X-Idempotent-Cache=HIT, got '%s'", rr2.Header().Get("X-Idempotent-Cache"))
	}
	if rr2.Header().Get("X-Custom") != "value" {
		t.Errorf("cache hit should preserve X-Custom header, expected 'value', got '%s'", rr2.Header().Get("X-Custom"))
	}
	if rr2.Header().Get("Content-Type") != "application/json" {
		t.Errorf("cache hit should preserve Content-Type header, expected 'application/json', got '%s'", rr2.Header().Get("Content-Type"))
	}
	if rr2.Header().Get("Location") != "/resource/123" {
		t.Errorf("cache hit should preserve Location header, expected '/resource/123', got '%s'", rr2.Header().Get("Location"))
	}
}

func TestMiddleware_CacheHitPreservesMultipleHeaderValues(t *testing.T) {
	i := NewIdempotent()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("X-Multi", "val1")
		w.Header().Add("X-Multi", "val2")
		w.Header().Add("X-Multi", "val3")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := i.Middleware(handler)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Idempotency-Key", "multi-hdr-key")
	rr1 := httptest.NewRecorder()
	mw.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Idempotency-Key", "multi-hdr-key")
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)

	values := rr2.Header().Values("X-Multi")
	if len(values) != 3 {
		t.Fatalf("expected 3 X-Multi header values on cache hit, got %d: %v", len(values), values)
	}
	if values[0] != "val1" || values[1] != "val2" || values[2] != "val3" {
		t.Errorf("cache hit header values mismatch: %v", values)
	}
}

func TestMiddleware_CustomHeader(t *testing.T) {
	cfg := Config{
		KeyHeader: "X-Custom-Idempotency",
	}
	i, _ := NewIdempotentWithConfig(cfg)

	callCount := 0
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := i.Middleware(handler)

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-Custom-Idempotency", "custom-key")
	rr1 := httptest.NewRecorder()
	mw.ServeHTTP(rr1, req1)

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-Custom-Idempotency", "custom-key")
	rr2 := httptest.NewRecorder()
	mw.ServeHTTP(rr2, req2)

	if callCount != 1 {
		t.Errorf("expected callCount=1, got %d", callCount)
	}
	if rr2.Header().Get("X-Idempotent-Cache") != "HIT" {
		t.Errorf("expected cache HIT with custom header, got '%s'", rr2.Header().Get("X-Idempotent-Cache"))
	}
}

func TestConcurrent_ExecuteAndClean(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL:           50 * time.Millisecond,
		CleanInterval: 25 * time.Millisecond,
	})
	i.Start()
	defer i.Stop()

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 200; j++ {
			_, _ = i.CleanExpired()
			time.Sleep(500 * time.Microsecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		handler := func() Response {
			return Response{
				StatusCode: http.StatusOK,
				Body:       []byte("val"),
				Header:     http.Header{"H": []string{"v"}},
			}
		}
		for j := 0; j < 500; j++ {
			key := fmt.Sprintf("race-key-%d", j)
			_, _, _ = i.Execute(key, handler)
			time.Sleep(200 * time.Microsecond)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 300; j++ {
			_, _ = i.Count()
			_, _, _ = i.Get(fmt.Sprintf("race-key-%d", j%100))
			time.Sleep(300 * time.Microsecond)
		}
	}()

	wg.Wait()
}

func TestMemoryLeak_AfterCleanup(t *testing.T) {
	i, _ := NewIdempotentWithConfig(Config{
		TTL:           20 * time.Millisecond,
		CleanInterval: 10 * time.Millisecond,
	})
	i.Start()
	defer i.Stop()

	const iterations = 100
	const batchSize = 50

	for j := 0; j < iterations; j++ {
		for k := 0; k < batchSize; k++ {
			key := fmt.Sprintf("leak-%d-%d", j, k)
			_ = i.Set(key, http.StatusOK, []byte("val"), nil)
		}
		time.Sleep(40 * time.Millisecond)
	}

	count, _ := i.Count()
	if count > batchSize {
		t.Errorf("after %d iterations, Count=%d exceeds batch size %d — memory leak?",
			iterations, count, batchSize)
	}
}

func TestResponseRecorder(t *testing.T) {
	rr := &responseRecorder{
		header: make(http.Header),
	}

	rr.Header().Set("Content-Type", "text/plain")
	rr.WriteHeader(http.StatusCreated)
	n, err := rr.Write([]byte("test body"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != 9 {
		t.Errorf("expected Write to return 9, got %d", n)
	}

	if rr.statusCode != http.StatusCreated {
		t.Errorf("expected statusCode=%d, got %d", http.StatusCreated, rr.statusCode)
	}
	if string(rr.body) != "test body" {
		t.Errorf("expected body='test body', got '%s'", string(rr.body))
	}
	if rr.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("expected Content-Type=text/plain, got '%s'", rr.Header().Get("Content-Type"))
	}
}

func TestResponseRecorder_DefaultStatus(t *testing.T) {
	rr := &responseRecorder{
		header: make(http.Header),
	}

	_, _ = rr.Write([]byte("auto status"))

	if rr.statusCode != http.StatusOK {
		t.Errorf("expected default statusCode=%d, got %d", http.StatusOK, rr.statusCode)
	}
}

func TestCloneHeader(t *testing.T) {
	original := http.Header{
		"K1": []string{"v1"},
		"K2": []string{"v2a", "v2b"},
	}

	cloned := cloneHeader(original)

	if cloned.Get("K1") != "v1" {
		t.Errorf("cloned K1 mismatch")
	}
	if len(cloned.Values("K2")) != 2 || cloned.Values("K2")[0] != "v2a" || cloned.Values("K2")[1] != "v2b" {
		t.Errorf("cloned K2 mismatch: %v", cloned.Values("K2"))
	}

	cloned.Set("K1", "modified")
	if original.Get("K1") != "v1" {
		t.Error("modifying cloned header should not affect original")
	}

	if cloneHeader(nil) != nil {
		t.Error("cloneHeader(nil) should return nil")
	}
}
