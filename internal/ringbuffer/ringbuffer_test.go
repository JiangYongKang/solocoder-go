package ringbuffer

import (
	"sync"
	"testing"
)

func TestNewRingBuffer(t *testing.T) {
	rb, err := NewRingBuffer[int](5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rb.Cap() != 5 {
		t.Errorf("expected capacity 5, got %d", rb.Cap())
	}
	if rb.Len() != 0 {
		t.Errorf("expected length 0, got %d", rb.Len())
	}
}

func TestNewRingBufferInvalidCapacity(t *testing.T) {
	_, err := NewRingBuffer[int](0)
	if err == nil {
		t.Error("expected error for capacity 0")
	}

	_, err = NewRingBuffer[int](-1)
	if err == nil {
		t.Error("expected error for negative capacity")
	}
}

func TestNewRingBufferWithConfig(t *testing.T) {
	cfg := Config{
		Capacity: 10,
		Strategy: Overwrite,
		HighWaterMark: 8,
	}
	rb, err := NewRingBufferWithConfig[int](cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rb.Cap() != 10 {
		t.Errorf("expected capacity 10, got %d", rb.Cap())
	}
	if rb.GetStrategy() != Overwrite {
		t.Error("expected Overwrite strategy")
	}
}

func TestNewRingBufferWithConfigInvalidHighWater(t *testing.T) {
	cfg := Config{
		Capacity: 10,
		HighWaterMark: 15,
	}
	_, err := NewRingBufferWithConfig[int](cfg)
	if err == nil {
		t.Error("expected error for high water mark > capacity")
	}

	cfg.HighWaterMark = -1
	_, err = NewRingBufferWithConfig[int](cfg)
	if err == nil {
		t.Error("expected error for negative high water mark")
	}
}

func TestWriteAndRead(t *testing.T) {
	rb, _ := NewRingBuffer[int](5)

	for i := 0; i < 3; i++ {
		if !rb.Write(i) {
			t.Errorf("write %d failed", i)
		}
	}

	if rb.Len() != 3 {
		t.Errorf("expected length 3, got %d", rb.Len())
	}

	for i := 0; i < 3; i++ {
		val, ok := rb.Read()
		if !ok {
			t.Errorf("read %d failed", i)
		}
		if val != i {
			t.Errorf("expected %d, got %d", i, val)
		}
	}

	if rb.Len() != 0 {
		t.Errorf("expected length 0, got %d", rb.Len())
	}
}

func TestReadEmptyBuffer(t *testing.T) {
	rb, _ := NewRingBuffer[int](5)

	val, ok := rb.Read()
	if ok {
		t.Error("expected read to fail on empty buffer")
	}
	if val != 0 {
		t.Errorf("expected zero value 0, got %d", val)
	}
}

func TestWriteFullBufferNoOverwrite(t *testing.T) {
	rb, _ := NewRingBuffer[int](3)

	for i := 0; i < 3; i++ {
		if !rb.Write(i) {
			t.Errorf("write %d failed", i)
		}
	}

	if !rb.IsFull() {
		t.Error("expected buffer to be full")
	}

	if rb.Write(99) {
		t.Error("expected write to fail on full buffer with NoOverwrite strategy")
	}

	if rb.Len() != 3 {
		t.Errorf("expected length 3, got %d", rb.Len())
	}
}

func TestWriteFullBufferOverwrite(t *testing.T) {
	cfg := Config{
		Capacity: 3,
		Strategy: Overwrite,
	}
	rb, _ := NewRingBufferWithConfig[int](cfg)

	for i := 0; i < 3; i++ {
		rb.Write(i)
	}

	if !rb.Write(99) {
		t.Error("write with overwrite should succeed")
	}

	if rb.Len() != 3 {
		t.Errorf("expected length 3, got %d", rb.Len())
	}

	val, ok := rb.Read()
	if !ok {
		t.Fatal("read failed")
	}
	if val != 1 {
		t.Errorf("expected 1 (oldest overwritten), got %d", val)
	}
}

func TestWrapAround(t *testing.T) {
	rb, _ := NewRingBuffer[int](4)

	rb.Write(1)
	rb.Write(2)
	rb.Write(3)

	val, _ := rb.Read()
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}
	val, _ = rb.Read()
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	rb.Write(4)
	rb.Write(5)

	if rb.Len() != 3 {
		t.Errorf("expected length 3, got %d", rb.Len())
	}

	expected := []int{3, 4, 5}
	for i, exp := range expected {
		val, ok := rb.Read()
		if !ok {
			t.Fatalf("read %d failed", i)
		}
		if val != exp {
			t.Errorf("expected %d, got %d", exp, val)
		}
	}
}

func TestPeek(t *testing.T) {
	rb, _ := NewRingBuffer[int](5)

	_, ok := rb.Peek()
	if ok {
		t.Error("expected peek to fail on empty buffer")
	}

	rb.Write(42)

	val, ok := rb.Peek()
	if !ok {
		t.Fatal("peek failed")
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	if rb.Len() != 1 {
		t.Errorf("expected length 1 after peek, got %d", rb.Len())
	}
}

func TestClear(t *testing.T) {
	rb, _ := NewRingBuffer[int](5)

	for i := 0; i < 3; i++ {
		rb.Write(i)
	}

	rb.Clear()

	if rb.Len() != 0 {
		t.Errorf("expected length 0 after clear, got %d", rb.Len())
	}

	if !rb.IsEmpty() {
		t.Error("expected buffer to be empty after clear")
	}

	_, ok := rb.Read()
	if ok {
		t.Error("expected read to fail after clear")
	}
}

func TestIsEmptyAndIsFull(t *testing.T) {
	rb, _ := NewRingBuffer[int](3)

	if !rb.IsEmpty() {
		t.Error("expected new buffer to be empty")
	}
	if rb.IsFull() {
		t.Error("expected new buffer not to be full")
	}

	rb.Write(1)
	rb.Write(2)

	if rb.IsEmpty() {
		t.Error("buffer should not be empty")
	}
	if rb.IsFull() {
		t.Error("buffer should not be full")
	}

	rb.Write(3)

	if rb.IsEmpty() {
		t.Error("buffer should not be empty")
	}
	if !rb.IsFull() {
		t.Error("buffer should be full")
	}
}

func TestSetStrategy(t *testing.T) {
	rb, _ := NewRingBuffer[int](3)

	if rb.GetStrategy() != NoOverwrite {
		t.Error("expected default NoOverwrite strategy")
	}

	rb.Write(1)
	rb.Write(2)
	rb.Write(3)

	if rb.Write(4) {
		t.Error("expected write to fail with NoOverwrite")
	}

	rb.SetStrategy(Overwrite)

	if rb.GetStrategy() != Overwrite {
		t.Error("expected Overwrite strategy after set")
	}

	if !rb.Write(4) {
		t.Error("expected write to succeed with Overwrite")
	}
}

func TestHighWaterMark(t *testing.T) {
	cfg := Config{
		Capacity: 10,
		HighWaterMark: 5,
	}
	rb, _ := NewRingBufferWithConfig[int](cfg)

	highWaterCount := 0
	lowWaterCount := 0

	rb.OnHighWater(func() {
		highWaterCount++
	})
	rb.OnLowWater(func() {
		lowWaterCount++
	})

	for i := 0; i < 4; i++ {
		rb.Write(i)
	}
	if highWaterCount != 0 {
		t.Errorf("expected 0 high water triggers, got %d", highWaterCount)
	}

	rb.Write(4)
	if highWaterCount != 1 {
		t.Errorf("expected 1 high water trigger, got %d", highWaterCount)
	}

	rb.Write(5)
	if highWaterCount != 1 {
		t.Errorf("expected high water to trigger only once, got %d", highWaterCount)
	}

	rb.Read()
	if lowWaterCount != 0 {
		t.Errorf("expected 0 low water triggers (still above), got %d", lowWaterCount)
	}

	rb.Read()
	if lowWaterCount != 1 {
		t.Errorf("expected 1 low water trigger, got %d", lowWaterCount)
	}

	rb.Read()
	if lowWaterCount != 1 {
		t.Errorf("expected low water to trigger only once, got %d", lowWaterCount)
	}
}

func TestSetHighWaterMark(t *testing.T) {
	rb, _ := NewRingBuffer[int](10)

	triggered := false
	rb.OnHighWater(func() {
		triggered = true
	})

	for i := 0; i < 5; i++ {
		rb.Write(i)
	}
	if triggered {
		t.Error("high water should not be triggered yet")
	}

	err := rb.SetHighWaterMark(3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !triggered {
		t.Error("high water should be triggered after setting mark below current count")
	}

	err = rb.SetHighWaterMark(11)
	if err == nil {
		t.Error("expected error for high water mark > capacity")
	}

	err = rb.SetHighWaterMark(-1)
	if err == nil {
		t.Error("expected error for negative high water mark")
	}
}

func TestHighWaterMarkWithOverwrite(t *testing.T) {
	cfg := Config{
		Capacity: 5,
		Strategy: Overwrite,
		HighWaterMark: 3,
	}
	rb, _ := NewRingBufferWithConfig[int](cfg)

	highWaterCount := 0
	lowWaterCount := 0

	rb.OnHighWater(func() {
		highWaterCount++
	})
	rb.OnLowWater(func() {
		lowWaterCount++
	})

	for i := 0; i < 5; i++ {
		rb.Write(i)
	}
	if highWaterCount != 1 {
		t.Errorf("expected 1 high water trigger, got %d", highWaterCount)
	}

	rb.Write(100)
	rb.Write(101)
	if highWaterCount != 1 {
		t.Errorf("expected high water to stay triggered, got %d", highWaterCount)
	}
	if lowWaterCount != 0 {
		t.Errorf("expected 0 low water triggers with overwrite, got %d", lowWaterCount)
	}
}

func TestGenericString(t *testing.T) {
	rb, _ := NewRingBuffer[string](3)

	rb.Write("hello")
	rb.Write("world")

	val, ok := rb.Read()
	if !ok {
		t.Fatal("read failed")
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got '%s'", val)
	}

	rb.Write("foo")

	expected := []string{"world", "foo"}
	for i, exp := range expected {
		val, ok := rb.Read()
		if !ok {
			t.Fatalf("read %d failed", i)
		}
		if val != exp {
			t.Errorf("expected '%s', got '%s'", exp, val)
		}
	}
}

func TestGenericStruct(t *testing.T) {
	type Item struct {
		ID   int
		Name string
	}

	rb, _ := NewRingBuffer[Item](3)

	rb.Write(Item{ID: 1, Name: "one"})
	rb.Write(Item{ID: 2, Name: "two"})

	val, ok := rb.Read()
	if !ok {
		t.Fatal("read failed")
	}
	if val.ID != 1 || val.Name != "one" {
		t.Errorf("expected Item{1, 'one'}, got Item{%d, '%s'}", val.ID, val.Name)
	}
}

func TestConcurrentAccess(t *testing.T) {
	rb, _ := NewRingBuffer[int](1000)

	var wg sync.WaitGroup
	writers := 10
	readers := 10
	itemsPerWriter := 100

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := 0; j < itemsPerWriter; j++ {
				rb.Write(base*itemsPerWriter + j)
			}
		}(i)
	}

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := 0
			for count < itemsPerWriter {
				if _, ok := rb.Read(); ok {
					count++
				}
			}
		}()
	}

	wg.Wait()
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Capacity != 1024 {
		t.Errorf("expected default capacity 1024, got %d", cfg.Capacity)
	}
	if cfg.Strategy != NoOverwrite {
		t.Error("expected default NoOverwrite strategy")
	}
	if cfg.HighWaterMark != 0 {
		t.Errorf("expected default high water mark 0, got %d", cfg.HighWaterMark)
	}
}

func TestBufferFullEdge(t *testing.T) {
	rb, _ := NewRingBuffer[int](1)

	if !rb.Write(42) {
		t.Error("first write should succeed")
	}

	if rb.Len() != 1 {
		t.Errorf("expected length 1, got %d", rb.Len())
	}

	if rb.Write(99) {
		t.Error("second write should fail with NoOverwrite")
	}

	val, ok := rb.Read()
	if !ok {
		t.Fatal("read failed")
	}
	if val != 42 {
		t.Errorf("expected 42, got %d", val)
	}

	if rb.Len() != 0 {
		t.Errorf("expected length 0, got %d", rb.Len())
	}
}

func TestOverwriteEdgeOneElement(t *testing.T) {
	cfg := Config{
		Capacity: 1,
		Strategy: Overwrite,
	}
	rb, _ := NewRingBufferWithConfig[int](cfg)

	rb.Write(1)
	rb.Write(2)
	rb.Write(3)

	if rb.Len() != 1 {
		t.Errorf("expected length 1, got %d", rb.Len())
	}

	val, ok := rb.Read()
	if !ok {
		t.Fatal("read failed")
	}
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
}

func TestReadAllFromFull(t *testing.T) {
	rb, _ := NewRingBuffer[int](5)

	for i := 0; i < 5; i++ {
		rb.Write(i)
	}

	if rb.Len() != 5 {
		t.Errorf("expected length 5, got %d", rb.Len())
	}

	for i := 0; i < 5; i++ {
		val, ok := rb.Read()
		if !ok {
			t.Fatalf("read %d failed", i)
		}
		if val != i {
			t.Errorf("expected %d, got %d", i, val)
		}
	}

	if rb.Len() != 0 {
		t.Errorf("expected length 0 after reading all, got %d", rb.Len())
	}
}

func TestAlternateReadWrite(t *testing.T) {
	rb, _ := NewRingBuffer[int](3)

	rb.Write(1)
	val, _ := rb.Read()
	if val != 1 {
		t.Errorf("expected 1, got %d", val)
	}

	rb.Write(2)
	val, _ = rb.Read()
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}

	rb.Write(3)
	rb.Write(4)
	rb.Write(5)

	if rb.Len() != 3 {
		t.Errorf("expected length 3, got %d", rb.Len())
	}

	expected := []int{3, 4, 5}
	for i, exp := range expected {
		val, ok := rb.Read()
		if !ok {
			t.Fatalf("read %d failed", i)
		}
		if val != exp {
			t.Errorf("expected %d, got %d", exp, val)
		}
	}
}
