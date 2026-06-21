package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newSnowflakeWithTime(machineID int64, nowFunc func() time.Time) *Snowflake {
	s, _ := New(Config{MachineID: machineID})
	s.nowFunc = nowFunc
	return s
}

func TestNew_ValidMachineID(t *testing.T) {
	s, err := New(Config{MachineID: 0})
	if err != nil {
		t.Fatalf("machine id 0 should be valid: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil Snowflake")
	}

	s, err = New(Config{MachineID: maxMachineID})
	if err != nil {
		t.Fatalf("machine id %d should be valid: %v", maxMachineID, err)
	}
}

func TestNew_InvalidMachineID_Negative(t *testing.T) {
	_, err := New(Config{MachineID: -1})
	if !errors.Is(err, ErrInvalidMachineID) {
		t.Errorf("expected ErrInvalidMachineID for negative id, got %v", err)
	}
}

func TestNew_InvalidMachineID_TooLarge(t *testing.T) {
	_, err := New(Config{MachineID: maxMachineID + 1})
	if !errors.Is(err, ErrInvalidMachineID) {
		t.Errorf("expected ErrInvalidMachineID for id %d, got %v", maxMachineID+1, err)
	}
}

func TestNext_BasicGeneration(t *testing.T) {
	s, _ := New(Config{MachineID: 1})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
}

func TestNext_IDsAreMonotonicallyIncreasing(t *testing.T) {
	s, _ := New(Config{MachineID: 1})
	var prev ID
	for i := 0; i < 1000; i++ {
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at iteration %d: %v", i, err)
		}
		if i > 0 && id <= prev {
			t.Errorf("ID %d should be greater than previous %d at iteration %d", id, prev, i)
		}
		prev = id
	}
}

func TestNext_MachineIDEncoded(t *testing.T) {
	machineID := int64(123)
	s, _ := New(Config{MachineID: machineID})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	parsed := Parse(id)
	if parsed.MachineID != machineID {
		t.Errorf("expected machine id %d, got %d", machineID, parsed.MachineID)
	}
}

func TestNext_SequenceIncrementInSameMS(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var callCount int64
	nowFunc := func() time.Time {
		count := atomic.AddInt64(&callCount, 1)
		if count <= int64(maxSequence)+1 {
			return baseTime
		}
		return baseTime.Add(time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	ids := make(map[ID]bool)
	for i := int64(0); i <= maxSequence; i++ {
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at sequence %d: %v", i, err)
		}
		parsed := Parse(id)
		if parsed.Sequence != i {
			t.Errorf("expected sequence %d, got %d", i, parsed.Sequence)
		}
		if ids[id] {
			t.Errorf("duplicate ID generated: %d", id)
		}
		ids[id] = true
	}
}

func TestNext_SequenceExhaustion_WaitsForNextMS(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(0)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	for i := int64(0); i <= maxSequence; i++ {
		_, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at sequence %d: %v", i, err)
		}
	}

	var nextID ID
	var nextErr error
	done := make(chan struct{})
	go func() {
		nextID, nextErr = s.Next()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Next should have blocked until next millisecond")
	default:
	}

	atomic.StoreInt64(&currentMs, 1)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Next did not unblock after advancing time")
	}

	if nextErr != nil {
		t.Fatalf("Next error after wait: %v", nextErr)
	}
	parsed := Parse(nextID)
	if parsed.Sequence != 0 {
		t.Errorf("expected sequence 0 in new ms, got %d", parsed.Sequence)
	}
}

func TestNext_ClockBackward_SmallDrift(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(10)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, err := s.Next()
	if err != nil {
		t.Fatalf("initial Next error: %v", err)
	}

	atomic.StoreInt64(&currentMs, 8)

	var id ID
	var idErr error
	done := make(chan struct{})
	go func() {
		id, idErr = s.Next()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("Next should block during small clock backward")
	default:
	}

	atomic.StoreInt64(&currentMs, 11)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Next did not unblock after clock recovered")
	}

	if idErr != nil {
		t.Fatalf("expected success after small drift recovery, got error: %v", idErr)
	}
	if id <= 0 {
		t.Errorf("expected positive ID after recovery, got %d", id)
	}
}

func TestNext_ClockBackward_LargeDrift(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(100)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, err := s.Next()
	if err != nil {
		t.Fatalf("initial Next error: %v", err)
	}

	atomic.StoreInt64(&currentMs, 50)

	_, err = s.Next()
	if err == nil {
		t.Fatal("expected error for large clock backward")
	}
	if !errors.Is(err, ErrClockBackward) {
		t.Errorf("expected ErrClockBackward, got %v", err)
	}
}

func TestNext_ClockBackward_ErrorContainsOffset(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(1000)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, _ = s.Next()
	atomic.StoreInt64(&currentMs, 900)

	_, err := s.Next()
	if err == nil {
		t.Fatal("expected error for clock backward")
	}
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("error message should contain offset information")
	}
}

func TestParse_RoundTrip(t *testing.T) {
	s, _ := New(Config{MachineID: 512})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}

	parsed := Parse(id)
	if parsed.MachineID != 512 {
		t.Errorf("expected machine id 512, got %d", parsed.MachineID)
	}
	if parsed.Sequence != 0 {
		t.Errorf("expected sequence 0 for first ID, got %d", parsed.Sequence)
	}
	if parsed.Timestamp <= 0 {
		t.Errorf("expected positive timestamp, got %d", parsed.Timestamp)
	}
}

func TestParse_MultipleIDs(t *testing.T) {
	s, _ := New(Config{MachineID: 100})
	for i := 0; i < 100; i++ {
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at iteration %d: %v", i, err)
		}
		parsed := Parse(id)
		if parsed.MachineID != 100 {
			t.Errorf("iteration %d: expected machine id 100, got %d", i, parsed.MachineID)
		}
	}
}

func TestParse_ManualID(t *testing.T) {
	manualID := ID((12345 << timestampShift) | (100 << machineIDShift) | 42)
	parsed := Parse(manualID)
	if parsed.Timestamp != 12345 {
		t.Errorf("expected timestamp 12345, got %d", parsed.Timestamp)
	}
	if parsed.MachineID != 100 {
		t.Errorf("expected machine id 100, got %d", parsed.MachineID)
	}
	if parsed.Sequence != 42 {
		t.Errorf("expected sequence 42, got %d", parsed.Sequence)
	}
}

func TestParsedID_Time(t *testing.T) {
	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	tsMilli := ts.UnixMilli()
	relativeTs := tsMilli - epoch

	parsed := ParsedID{
		Timestamp: relativeTs,
		MachineID: 1,
		Sequence:  0,
	}

	recovered := parsed.Time()
	if !recovered.Equal(ts) {
		t.Errorf("expected %v, got %v", ts, recovered)
	}
}

func TestDecompose_EquivalentToParse(t *testing.T) {
	id := ID((99999 << timestampShift) | (777 << machineIDShift) | 123)
	parsed := Parse(id)
	decomposed := Decompose(id)
	if parsed != decomposed {
		t.Errorf("Parse and Decompose should return same result, got %+v vs %+v", parsed, decomposed)
	}
}

func TestIDBitLayout(t *testing.T) {
	machineID := int64(1023)
	s, _ := New(Config{MachineID: machineID})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}

	parsed := Parse(id)

	reconstructed := ID((parsed.Timestamp << timestampShift) | (parsed.MachineID << machineIDShift) | parsed.Sequence)
	if id != reconstructed {
		t.Errorf("bit layout round-trip failed: original=%d, reconstructed=%d", id, reconstructed)
	}
}

func TestNext_MachineIDZero(t *testing.T) {
	s, _ := New(Config{MachineID: 0})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	parsed := Parse(id)
	if parsed.MachineID != 0 {
		t.Errorf("expected machine id 0, got %d", parsed.MachineID)
	}
}

func TestNext_MachineIDMax(t *testing.T) {
	s, _ := New(Config{MachineID: maxMachineID})
	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	parsed := Parse(id)
	if parsed.MachineID != maxMachineID {
		t.Errorf("expected machine id %d, got %d", maxMachineID, parsed.MachineID)
	}
}

func TestNext_SequenceZeroForNewMS(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(0)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	id1, _ := s.Next()
	p1 := Parse(id1)
	if p1.Sequence != 0 {
		t.Errorf("first ID in ms should have sequence 0, got %d", p1.Sequence)
	}

	id2, _ := s.Next()
	p2 := Parse(id2)
	if p2.Sequence != 1 {
		t.Errorf("second ID in same ms should have sequence 1, got %d", p2.Sequence)
	}

	atomic.StoreInt64(&currentMs, 1)

	id3, _ := s.Next()
	p3 := Parse(id3)
	if p3.Sequence != 0 {
		t.Errorf("first ID in new ms should have sequence 0, got %d", p3.Sequence)
	}
}

func TestNext_NoDuplicateIDs(t *testing.T) {
	s, _ := New(Config{MachineID: 1})
	seen := make(map[ID]bool)
	for i := 0; i < 5000; i++ {
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at iteration %d: %v", i, err)
		}
		if seen[id] {
			t.Errorf("duplicate ID %d at iteration %d", id, i)
		}
		seen[id] = true
	}
}

func TestNext_Concurrent(t *testing.T) {
	s, _ := New(Config{MachineID: 1})
	var wg sync.WaitGroup
	numGoroutines := 20
	iterations := 500

	var mu sync.Mutex
	seen := make(map[ID]bool)
	var duplicates int64

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id, err := s.Next()
				if err != nil {
					t.Errorf("goroutine %d iteration %d error: %v", gid, i, err)
					return
				}
				mu.Lock()
				if seen[id] {
					atomic.AddInt64(&duplicates, 1)
				}
				seen[id] = true
				mu.Unlock()
			}
		}(g)
	}

	wg.Wait()

	if duplicates > 0 {
		t.Errorf("found %d duplicate IDs in concurrent test", duplicates)
	}
	expected := int64(numGoroutines * iterations)
	if int64(len(seen)) != expected {
		t.Errorf("expected %d unique IDs, got %d", expected, len(seen))
	}
}

func TestNext_ConcurrentDifferentMachineIDs(t *testing.T) {
	var wg sync.WaitGroup
	numMachines := 10
	idsPerMachine := 200

	allIDs := make(chan ID, numMachines*idsPerMachine)

	for m := 0; m < numMachines; m++ {
		wg.Add(1)
		go func(machineID int64) {
			defer wg.Done()
			s, err := New(Config{MachineID: machineID})
			if err != nil {
				t.Errorf("New error for machine id %d: %v", machineID, err)
				return
			}
			for i := 0; i < idsPerMachine; i++ {
				id, err := s.Next()
				if err != nil {
					t.Errorf("machine %d iteration %d error: %v", machineID, i, err)
					return
				}
				allIDs <- id
			}
		}(int64(m))
	}

	wg.Wait()
	close(allIDs)

	seen := make(map[ID]bool)
	for id := range allIDs {
		if seen[id] {
			t.Errorf("duplicate ID across machines: %d", id)
		}
		seen[id] = true
	}
}

func TestNext_ClockBackward_BoundaryExactlyAtThreshold(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(100)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, _ = s.Next()

	atomic.StoreInt64(&currentMs, 100-clockBackwardSmallMaxMs)

	var id ID
	var idErr error
	done := make(chan struct{})
	go func() {
		id, idErr = s.Next()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("should block during threshold clock backward")
	default:
	}

	atomic.StoreInt64(&currentMs, 101)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not unblock after clock recovered")
	}

	if idErr != nil {
		t.Fatalf("expected success at exact threshold, got: %v", idErr)
	}
	_ = id
}

func TestNext_ClockBackward_OneMsAboveThreshold(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(100)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, _ = s.Next()

	atomic.StoreInt64(&currentMs, 100-clockBackwardSmallMaxMs-1)

	_, err := s.Next()
	if !errors.Is(err, ErrClockBackward) {
		t.Errorf("expected ErrClockBackward for drift above threshold, got %v", err)
	}
}

func TestParse_ZeroID(t *testing.T) {
	parsed := Parse(ID(0))
	if parsed.Timestamp != 0 {
		t.Errorf("expected timestamp 0, got %d", parsed.Timestamp)
	}
	if parsed.MachineID != 0 {
		t.Errorf("expected machine id 0, got %d", parsed.MachineID)
	}
	if parsed.Sequence != 0 {
		t.Errorf("expected sequence 0, got %d", parsed.Sequence)
	}
}

func TestParse_MaxValues(t *testing.T) {
	maxTS := int64((1 << timestampBits) - 1)
	id := ID((maxTS << timestampShift) | maxMachineID<<machineIDShift | maxSequence)
	parsed := Parse(id)
	if parsed.Timestamp != maxTS {
		t.Errorf("expected max timestamp %d, got %d", maxTS, parsed.Timestamp)
	}
	if parsed.MachineID != maxMachineID {
		t.Errorf("expected max machine id %d, got %d", maxMachineID, parsed.MachineID)
	}
	if parsed.Sequence != maxSequence {
		t.Errorf("expected max sequence %d, got %d", maxSequence, parsed.Sequence)
	}
}

func TestNext_MultipleGenerationsAcrossMS(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(0)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	for ms := int64(0); ms < 5; ms++ {
		atomic.StoreInt64(&currentMs, ms)
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at ms %d: %v", ms, err)
		}
		parsed := Parse(id)
		if parsed.Sequence != 0 {
			t.Errorf("first ID in ms %d should have sequence 0, got %d", ms, parsed.Sequence)
		}

		id2, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at ms %d (second): %v", ms, err)
		}
		parsed2 := Parse(id2)
		if parsed2.Sequence != 1 {
			t.Errorf("second ID in ms %d should have sequence 1, got %d", ms, parsed2.Sequence)
		}
	}
}

func TestNext_StressHighFrequency(t *testing.T) {
	s, _ := New(Config{MachineID: 1})
	ids := make([]ID, 10000)
	for i := 0; i < 10000; i++ {
		id, err := s.Next()
		if err != nil {
			t.Fatalf("Next error at iteration %d: %v", i, err)
		}
		ids[i] = id
	}

	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("ID at index %d (%d) should be greater than previous (%d)", i, ids[i], ids[i-1])
		}
	}
}

func TestEpochValue(t *testing.T) {
	expected := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	if epoch != expected {
		t.Errorf("epoch should be %d (2024-01-01 UTC), got %d", expected, epoch)
	}
}

func TestMaxMachineIDValue(t *testing.T) {
	if maxMachineID != 1023 {
		t.Errorf("max machine id should be 1023 (10 bits), got %d", maxMachineID)
	}
}

func TestMaxSequenceValue(t *testing.T) {
	if maxSequence != 4095 {
		t.Errorf("max sequence should be 4095 (12 bits), got %d", maxSequence)
	}
}

func TestNext_ConcurrentClockBackward(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(100)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	var wg sync.WaitGroup
	var errCount int64
	var successCount int64

	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_, err := s.Next()
				if err != nil {
					atomic.AddInt64(&errCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}()
	}

	time.Sleep(10 * time.Millisecond)
	atomic.StoreInt64(&currentMs, 50)
	time.Sleep(10 * time.Millisecond)
	atomic.StoreInt64(&currentMs, 200)

	wg.Wait()

	if successCount == 0 {
		t.Error("expected some successful ID generations")
	}
}

func TestNext_ClockBackward_SmallDriftExactOffset(t *testing.T) {
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	currentMs := int64(100)
	nowFunc := func() time.Time {
		return baseTime.Add(time.Duration(atomic.LoadInt64(&currentMs)) * time.Millisecond)
	}

	s := newSnowflakeWithTime(1, nowFunc)

	_, _ = s.Next()

	atomic.StoreInt64(&currentMs, 97)

	var id ID
	var idErr error
	done := make(chan struct{})
	go func() {
		id, idErr = s.Next()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("should block during 3ms clock backward")
	default:
	}

	atomic.StoreInt64(&currentMs, 101)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("did not unblock after clock recovered")
	}

	if idErr != nil {
		t.Fatalf("expected success after small drift, got: %v", idErr)
	}
	parsed := Parse(id)
	if parsed.MachineID != 1 {
		t.Errorf("expected machine id 1, got %d", parsed.MachineID)
	}
}

func BenchmarkNext(b *testing.B) {
	s, _ := New(Config{MachineID: 1})
	for i := 0; i < b.N; i++ {
		_, _ = s.Next()
	}
}

func BenchmarkNextParallel(b *testing.B) {
	s, _ := New(Config{MachineID: 1})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = s.Next()
		}
	})
}

func BenchmarkParse(b *testing.B) {
	s, _ := New(Config{MachineID: 1})
	id, _ := s.Next()
	for i := 0; i < b.N; i++ {
		Parse(id)
	}
}

func TestFmt_SnowflakeUsage(t *testing.T) {
	s, err := New(Config{MachineID: 1})
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	id, err := s.Next()
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}

	parsed := Parse(id)
	_ = fmt.Sprintf("id=%d, ts=%d, mid=%d, seq=%d, time=%v",
		id, parsed.Timestamp, parsed.MachineID, parsed.Sequence, parsed.Time())
}
