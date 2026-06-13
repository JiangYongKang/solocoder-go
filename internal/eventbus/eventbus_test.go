package eventbus

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewEventBus(t *testing.T) {
	bus := NewEventBus()
	if bus == nil {
		t.Fatal("NewEventBus returned nil")
	}
	if bus.SubscriberCount("any") != 0 {
		t.Errorf("expected 0 subscribers, got %d", bus.SubscriberCount("any"))
	}
}

func TestSubscribe(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error { return nil }
	id, err := bus.Subscribe("user.created", handler, SubscribeConfig{})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if id == "" {
		t.Error("expected non-empty subscriber id")
	}
	if bus.SubscriberCount("user.created") != 1 {
		t.Errorf("expected 1 subscriber, got %d", bus.SubscriberCount("user.created"))
	}
}

func TestSubscribeWithCustomID(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error { return nil }
	id, err := bus.Subscribe("user.created", handler, SubscribeConfig{ID: "custom-id"})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if id != "custom-id" {
		t.Errorf("expected id 'custom-id', got '%s'", id)
	}

	_, err = bus.Subscribe("user.created", handler, SubscribeConfig{ID: "custom-id"})
	if !errors.Is(err, ErrSubscriberExists) {
		t.Errorf("expected ErrSubscriberExists, got %v", err)
	}
}

func TestSubscribeNilHandler(t *testing.T) {
	bus := NewEventBus()

	_, err := bus.Subscribe("user.created", nil, SubscribeConfig{})
	if err == nil {
		t.Error("expected error for nil handler, got nil")
	}
}

func TestUnsubscribe(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error { return nil }
	id, _ := bus.Subscribe("user.created", handler, SubscribeConfig{})

	err := bus.Unsubscribe("user.created", id)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}
	if bus.SubscriberCount("user.created") != 0 {
		t.Errorf("expected 0 subscribers after unsubscribe, got %d", bus.SubscriberCount("user.created"))
	}
}

func TestUnsubscribeNotFound(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error { return nil }
	id, _ := bus.Subscribe("user.created", handler, SubscribeConfig{})

	err := bus.Unsubscribe("user.deleted", id)
	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("expected ErrSubscriberNotFound for wrong eventType, got %v", err)
	}

	err = bus.Unsubscribe("user.created", "non-existent")
	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("expected ErrSubscriberNotFound for wrong id, got %v", err)
	}
}

func TestUnsubscribeRemovesEmptyEventType(t *testing.T) {
	bus := NewEventBus()

	handler := func(event Event) error { return nil }
	id, _ := bus.Subscribe("user.created", handler, SubscribeConfig{})
	bus.Unsubscribe("user.created", id)

	bus.mu.RLock()
	_, exists := bus.subscribers["user.created"]
	bus.mu.RUnlock()
	if exists {
		t.Error("expected eventType key to be removed after last subscriber unsubscribed")
	}
}

func TestPublishSyncBasic(t *testing.T) {
	bus := NewEventBus()

	var callCount int32
	handler := func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	bus.Subscribe("user.created", handler, SubscribeConfig{})
	bus.Subscribe("user.created", handler, SubscribeConfig{})

	event := Event{
		Type:       "user.created",
		Payload:    "test payload",
		Attributes: map[string]interface{}{"user_id": 123},
	}

	err := bus.PublishSync(event)
	if err != nil {
		t.Fatalf("PublishSync failed: %v", err)
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 handler calls, got %d", callCount)
	}
}

func TestPublishSyncNoSubscribers(t *testing.T) {
	bus := NewEventBus()

	event := Event{Type: "user.created"}
	err := bus.PublishSync(event)
	if err != nil {
		t.Fatalf("expected no error for no subscribers, got %v", err)
	}
}

func TestPublishSyncMultipleEventTypes(t *testing.T) {
	bus := NewEventBus()

	var userCreatedCount int32
	var userDeletedCount int32

	bus.Subscribe("user.created", func(event Event) error {
		atomic.AddInt32(&userCreatedCount, 1)
		return nil
	}, SubscribeConfig{})

	bus.Subscribe("user.deleted", func(event Event) error {
		atomic.AddInt32(&userDeletedCount, 1)
		return nil
	}, SubscribeConfig{})

	bus.PublishSync(Event{Type: "user.created"})

	if atomic.LoadInt32(&userCreatedCount) != 1 {
		t.Errorf("expected user.created to be called once, got %d", userCreatedCount)
	}
	if atomic.LoadInt32(&userDeletedCount) != 0 {
		t.Errorf("expected user.deleted to not be called, got %d", userDeletedCount)
	}
}

func TestPublishSyncHandlerError(t *testing.T) {
	bus := NewEventBus()

	errMsg := errors.New("handler error")
	var callCount int32

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return errMsg
	}, SubscribeConfig{})

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}, SubscribeConfig{})

	err := bus.PublishSync(Event{Type: "test.event"})
	if !errors.Is(err, errMsg) {
		t.Errorf("expected error '%v', got '%v'", errMsg, err)
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 handler calls despite error, got %d", callCount)
	}
}

func TestPublishSyncInterrupt(t *testing.T) {
	bus := NewEventBus()

	var callCount int32

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return ErrInterrupt
	}, SubscribeConfig{Priority: 100})

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}, SubscribeConfig{Priority: 50})

	err := bus.PublishSync(Event{Type: "test.event"})
	if !errors.Is(err, ErrInterrupt) {
		t.Errorf("expected ErrInterrupt, got %v", err)
	}

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected only 1 handler call due to interrupt, got %d", callCount)
	}
}

func TestPublishAsyncBasic(t *testing.T) {
	bus := NewEventBus()

	var callCount int32
	var wg sync.WaitGroup
	wg.Add(2)

	handler := func(event Event) error {
		defer wg.Done()
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	bus.Subscribe("user.created", handler, SubscribeConfig{})
	bus.Subscribe("user.created", handler, SubscribeConfig{})

	bus.PublishAsync(Event{Type: "user.created"})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for async handlers")
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 handler calls, got %d", callCount)
	}
}

func TestPublishAsyncNoSubscribers(t *testing.T) {
	bus := NewEventBus()
	bus.PublishAsync(Event{Type: "nonexistent"})
	bus.Wait()
}

func TestPublishAsyncWait(t *testing.T) {
	bus := NewEventBus()

	var callCount int32
	handler := func(event Event) error {
		time.Sleep(50 * time.Millisecond)
		atomic.AddInt32(&callCount, 1)
		return nil
	}

	bus.Subscribe("test.event", handler, SubscribeConfig{})
	bus.Subscribe("test.event", handler, SubscribeConfig{})

	bus.PublishAsync(Event{Type: "test.event"})
	bus.Wait()

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 handler calls after Wait, got %d", callCount)
	}
}

func TestPublishAsyncPanicRecovery(t *testing.T) {
	bus := NewEventBus()

	var wg sync.WaitGroup
	wg.Add(2)

	bus.Subscribe("test.event", func(event Event) error {
		defer wg.Done()
		panic("test panic")
	}, SubscribeConfig{})

	bus.Subscribe("test.event", func(event Event) error {
		defer wg.Done()
		return nil
	}, SubscribeConfig{})

	bus.PublishAsync(Event{Type: "test.event"})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for async handlers after panic")
	}
}

func TestPublishSyncPanicRecovery(t *testing.T) {
	bus := NewEventBus()

	var callCount int32

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		panic("test panic in sync")
	}, SubscribeConfig{})

	bus.Subscribe("test.event", func(event Event) error {
		atomic.AddInt32(&callCount, 1)
		return nil
	}, SubscribeConfig{})

	err := bus.PublishSync(Event{Type: "test.event"})
	if err == nil {
		t.Error("expected error from panic recovery, got nil")
	}

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 calls after panic recovery, got %d", callCount)
	}
}

func TestEqualsFilter(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      interface{}
		attributes map[string]interface{}
		expected   bool
	}{
		{
			name:       "string match",
			key:        "status",
			value:      "active",
			attributes: map[string]interface{}{"status": "active"},
			expected:   true,
		},
		{
			name:       "string mismatch",
			key:        "status",
			value:      "active",
			attributes: map[string]interface{}{"status": "inactive"},
			expected:   false,
		},
		{
			name:       "int match",
			key:        "level",
			value:      42,
			attributes: map[string]interface{}{"level": 42},
			expected:   true,
		},
		{
			name:       "missing key",
			key:        "missing",
			value:      "val",
			attributes: map[string]interface{}{"other": "val"},
			expected:   false,
		},
		{
			name:       "nil attributes",
			key:        "k",
			value:      "v",
			attributes: nil,
			expected:   false,
		},
		{
			name:       "struct match",
			key:        "user",
			value:      struct{ ID int }{ID: 1},
			attributes: map[string]interface{}{"user": struct{ ID int }{ID: 1}},
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &EqualsFilter{Key: tt.key, Value: tt.value}
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRangeFilter(t *testing.T) {
	tests := []struct {
		name       string
		min        float64
		max        float64
		hasMin     bool
		hasMax     bool
		attributes map[string]interface{}
		expected   bool
	}{
		{
			name:       "within range int",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 15},
			expected:   true,
		},
		{
			name:       "within range float",
			min:        1.5,
			max:        2.5,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"score": 2.0},
			expected:   true,
		},
		{
			name:       "boundary min inclusive",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 10},
			expected:   true,
		},
		{
			name:       "boundary max inclusive",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 20},
			expected:   true,
		},
		{
			name:       "below min",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 5},
			expected:   false,
		},
		{
			name:       "above max",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 25},
			expected:   false,
		},
		{
			name:       "only min satisfied",
			min:        10,
			hasMin:     true,
			hasMax:     false,
			attributes: map[string]interface{}{"age": 100},
			expected:   true,
		},
		{
			name:       "only min not satisfied",
			min:        10,
			hasMin:     true,
			hasMax:     false,
			attributes: map[string]interface{}{"age": 5},
			expected:   false,
		},
		{
			name:       "only max satisfied",
			max:        20,
			hasMin:     false,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 10},
			expected:   true,
		},
		{
			name:       "only max not satisfied",
			max:        20,
			hasMin:     false,
			hasMax:     true,
			attributes: map[string]interface{}{"age": 25},
			expected:   false,
		},
		{
			name:       "non-numeric value",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"age": "old"},
			expected:   false,
		},
		{
			name:       "missing key",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: map[string]interface{}{"other": 15},
			expected:   false,
		},
		{
			name:       "nil attributes",
			min:        10,
			max:        20,
			hasMin:     true,
			hasMax:     true,
			attributes: nil,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &RangeFilter{
				Key:    "age",
				Min:    tt.min,
				Max:    tt.max,
				HasMin: tt.hasMin,
				HasMax: tt.hasMax,
			}
			if tt.name == "within range float" {
				filter.Key = "score"
			}
			if tt.name == "non-numeric value" {
				filter.Key = "age"
			}
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRangeFilterAllNumericTypes(t *testing.T) {
	filter := &RangeFilter{Key: "val", Min: 0, Max: 100, HasMin: true, HasMax: true}

	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"int", 50, true},
		{"int8", int8(50), true},
		{"int16", int16(50), true},
		{"int32", int32(50), true},
		{"int64", int64(50), true},
		{"uint", uint(50), true},
		{"uint8", uint8(50), true},
		{"uint16", uint16(50), true},
		{"uint32", uint32(50), true},
		{"uint64", uint64(50), true},
		{"float32", float32(50.0), true},
		{"float64", float64(50.0), true},
		{"string", "not a number", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Attributes: map[string]interface{}{"val": tt.value}}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v for %T, got %v", tt.expected, tt.value, result)
			}
		})
	}
}

func TestAndFilter(t *testing.T) {
	filter := &AndFilter{
		Filters: []Filter{
			&EqualsFilter{Key: "status", Value: "active"},
			&RangeFilter{Key: "age", Min: 18, Max: 65, HasMin: true, HasMax: true},
		},
	}

	tests := []struct {
		name       string
		attributes map[string]interface{}
		expected   bool
	}{
		{
			name:       "both match",
			attributes: map[string]interface{}{"status": "active", "age": 30},
			expected:   true,
		},
		{
			name:       "first fails",
			attributes: map[string]interface{}{"status": "inactive", "age": 30},
			expected:   false,
		},
		{
			name:       "second fails",
			attributes: map[string]interface{}{"status": "active", "age": 10},
			expected:   false,
		},
		{
			name:       "both fail",
			attributes: map[string]interface{}{"status": "inactive", "age": 10},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestOrFilter(t *testing.T) {
	filter := &OrFilter{
		Filters: []Filter{
			&EqualsFilter{Key: "role", Value: "admin"},
			&EqualsFilter{Key: "role", Value: "superuser"},
		},
	}

	tests := []struct {
		name       string
		attributes map[string]interface{}
		expected   bool
	}{
		{
			name:       "first matches",
			attributes: map[string]interface{}{"role": "admin"},
			expected:   true,
		},
		{
			name:       "second matches",
			attributes: map[string]interface{}{"role": "superuser"},
			expected:   true,
		},
		{
			name:       "neither matches",
			attributes: map[string]interface{}{"role": "user"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestNotFilter(t *testing.T) {
	inner := &EqualsFilter{Key: "status", Value: "blocked"}
	filter := &NotFilter{Inner: inner}

	tests := []struct {
		name       string
		attributes map[string]interface{}
		expected   bool
	}{
		{"inner matches, not returns false", map[string]interface{}{"status": "blocked"}, false},
		{"inner fails, not returns true", map[string]interface{}{"status": "active"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFilteredSubscription(t *testing.T) {
	bus := NewEventBus()

	var adminCalled int32
	var userCalled int32

	bus.Subscribe("login", func(event Event) error {
		atomic.AddInt32(&adminCalled, 1)
		return nil
	}, SubscribeConfig{
		Filter: &EqualsFilter{Key: "role", Value: "admin"},
	})

	bus.Subscribe("login", func(event Event) error {
		atomic.AddInt32(&userCalled, 1)
		return nil
	}, SubscribeConfig{})

	bus.PublishSync(Event{
		Type:       "login",
		Attributes: map[string]interface{}{"role": "admin"},
	})

	if atomic.LoadInt32(&adminCalled) != 1 {
		t.Errorf("expected admin handler called once, got %d", adminCalled)
	}
	if atomic.LoadInt32(&userCalled) != 1 {
		t.Errorf("expected user handler called once, got %d", userCalled)
	}

	atomic.StoreInt32(&adminCalled, 0)
	atomic.StoreInt32(&userCalled, 0)

	bus.PublishSync(Event{
		Type:       "login",
		Attributes: map[string]interface{}{"role": "guest"},
	})

	if atomic.LoadInt32(&adminCalled) != 0 {
		t.Errorf("expected admin handler not called for guest, got %d", adminCalled)
	}
	if atomic.LoadInt32(&userCalled) != 1 {
		t.Errorf("expected user handler called once for guest, got %d", userCalled)
	}
}

func TestPriorityOrder(t *testing.T) {
	bus := NewEventBus()

	var order []int
	var mu sync.Mutex

	addOrder := func(n int) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, n)
	}

	bus.Subscribe("test", func(event Event) error {
		addOrder(10)
		return nil
	}, SubscribeConfig{Priority: 10})

	bus.Subscribe("test", func(event Event) error {
		addOrder(100)
		return nil
	}, SubscribeConfig{Priority: 100})

	bus.Subscribe("test", func(event Event) error {
		addOrder(50)
		return nil
	}, SubscribeConfig{Priority: 50})

	bus.PublishSync(Event{Type: "test"})

	expected := []int{100, 50, 10}
	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(order), order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("expected order[%d] = %d, got %d. Full order: %v", i, v, order[i], order)
		}
	}
}

func TestPriorityInterruptWithFilter(t *testing.T) {
	bus := NewEventBus()

	var callOrder []int
	var mu sync.Mutex

	addOrder := func(n int) {
		mu.Lock()
		defer mu.Unlock()
		callOrder = append(callOrder, n)
	}

	bus.Subscribe("msg", func(event Event) error {
		addOrder(200)
		return nil
	}, SubscribeConfig{
		Priority: 200,
		Filter:   &EqualsFilter{Key: "special", Value: true},
	})

	bus.Subscribe("msg", func(event Event) error {
		addOrder(100)
		return ErrInterrupt
	}, SubscribeConfig{Priority: 100})

	bus.Subscribe("msg", func(event Event) error {
		addOrder(50)
		return nil
	}, SubscribeConfig{Priority: 50})

	err := bus.PublishSync(Event{
		Type:       "msg",
		Attributes: map[string]interface{}{"special": true},
	})

	if !errors.Is(err, ErrInterrupt) {
		t.Errorf("expected ErrInterrupt, got %v", err)
	}

	expected := []int{200, 100}
	if len(callOrder) != len(expected) {
		t.Fatalf("expected %d calls, got %d: %v", len(expected), len(callOrder), callOrder)
	}
	for i, v := range expected {
		if callOrder[i] != v {
			t.Errorf("expected callOrder[%d] = %d, got %d. Full: %v", i, v, callOrder[i], callOrder)
		}
	}
}

func TestSamePriorityOrder(t *testing.T) {
	bus := NewEventBus()

	var order []int
	var mu sync.Mutex

	addOrder := func(n int) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, n)
	}

	bus.Subscribe("test", func(event Event) error {
		addOrder(1)
		return nil
	}, SubscribeConfig{Priority: 50})

	bus.Subscribe("test", func(event Event) error {
		addOrder(2)
		return nil
	}, SubscribeConfig{Priority: 50})

	bus.Subscribe("test", func(event Event) error {
		addOrder(3)
		return nil
	}, SubscribeConfig{Priority: 50})

	bus.PublishSync(Event{Type: "test"})

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	for i, v := range []int{1, 2, 3} {
		if order[i] != v {
			t.Errorf("same priority: expected order[%d] = %d, got %d (full: %v)", i, v, order[i], order)
		}
	}
}

func TestDynamicSubscribeUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			eventType := fmt.Sprintf("event.%d", i%10)
			handler := func(event Event) error { return nil }
			id, err := bus.Subscribe(eventType, handler, SubscribeConfig{})
			if err != nil {
				t.Errorf("Subscribe failed: %v", err)
				return
			}
			time.Sleep(time.Millisecond)
			if err := bus.Unsubscribe(eventType, id); err != nil {
				t.Errorf("Unsubscribe failed: %v", err)
			}
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		count := bus.SubscriberCount(fmt.Sprintf("event.%d", i))
		if count != 0 {
			t.Errorf("expected 0 subscribers for event.%d, got %d", i, count)
		}
	}
}

func TestConcurrentPublish(t *testing.T) {
	bus := NewEventBus()

	var totalCalls int64
	handler := func(event Event) error {
		atomic.AddInt64(&totalCalls, 1)
		return nil
	}

	for i := 0; i < 10; i++ {
		bus.Subscribe("event", handler, SubscribeConfig{})
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.PublishSync(Event{Type: "event"})
		}()
	}

	wg.Wait()
	bus.Wait()

	expected := int64(10 * 100)
	if atomic.LoadInt64(&totalCalls) != expected {
		t.Errorf("expected %d total calls, got %d", expected, totalCalls)
	}
}

func TestConcurrentPublishAsync(t *testing.T) {
	bus := NewEventBus()

	var totalCalls int64
	handler := func(event Event) error {
		time.Sleep(time.Millisecond)
		atomic.AddInt64(&totalCalls, 1)
		return nil
	}

	for i := 0; i < 5; i++ {
		bus.Subscribe("async.event", handler, SubscribeConfig{})
	}

	for i := 0; i < 50; i++ {
		bus.PublishAsync(Event{Type: "async.event"})
	}

	bus.Wait()

	expected := int64(5 * 50)
	if atomic.LoadInt64(&totalCalls) != expected {
		t.Errorf("expected %d total calls, got %d", expected, totalCalls)
	}
}

func TestEventPayloadAndAttributes(t *testing.T) {
	bus := NewEventBus()

	type TestPayload struct {
		Name string
		Age  int
	}

	var receivedEvent Event
	bus.Subscribe("user.event", func(event Event) error {
		receivedEvent = event
		return nil
	}, SubscribeConfig{})

	payload := TestPayload{Name: "Alice", Age: 30}
	attrs := map[string]interface{}{
		"source": "web",
		"level":  5,
	}

	event := Event{
		Type:       "user.event",
		Payload:    payload,
		Attributes: attrs,
	}

	bus.PublishSync(event)

	if receivedEvent.Type != "user.event" {
		t.Errorf("expected Type 'user.event', got '%s'", receivedEvent.Type)
	}

	receivedPayload, ok := receivedEvent.Payload.(TestPayload)
	if !ok {
		t.Fatalf("expected Payload type TestPayload, got %T", receivedEvent.Payload)
	}
	if receivedPayload.Name != "Alice" || receivedPayload.Age != 30 {
		t.Errorf("unexpected payload: %+v", receivedPayload)
	}

	if receivedEvent.Attributes["source"] != "web" {
		t.Errorf("expected source='web', got %v", receivedEvent.Attributes["source"])
	}
	if receivedEvent.Attributes["level"] != 5 {
		t.Errorf("expected level=5, got %v", receivedEvent.Attributes["level"])
	}
}

func TestPublishAllErrorsReturned(t *testing.T) {
	bus := NewEventBus()

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")

	bus.Subscribe("test", func(event Event) error { return err1 }, SubscribeConfig{Priority: 10})
	bus.Subscribe("test", func(event Event) error { return nil }, SubscribeConfig{Priority: 5})
	bus.Subscribe("test", func(event Event) error { return err2 }, SubscribeConfig{Priority: 1})

	err := bus.PublishSync(Event{Type: "test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, err1) {
		t.Errorf("expected first error to be returned, got %v", err)
	}
}

func TestNoFilterMatchesAll(t *testing.T) {
	bus := NewEventBus()

	var count int32
	bus.Subscribe("test", func(event Event) error {
		atomic.AddInt32(&count, 1)
		return nil
	}, SubscribeConfig{Filter: nil})

	bus.PublishSync(Event{Type: "test"})
	bus.PublishSync(Event{Type: "test", Attributes: map[string]interface{}{"a": 1}})
	bus.PublishSync(Event{Type: "test", Attributes: nil})

	if atomic.LoadInt32(&count) != 3 {
		t.Errorf("expected 3 calls with nil filter, got %d", count)
	}
}

func TestSubscriberCount(t *testing.T) {
	bus := NewEventBus()

	if bus.SubscriberCount("nonexistent") != 0 {
		t.Errorf("expected 0 for nonexistent eventType")
	}

	bus.Subscribe("a", func(e Event) error { return nil }, SubscribeConfig{})
	bus.Subscribe("a", func(e Event) error { return nil }, SubscribeConfig{})
	bus.Subscribe("b", func(e Event) error { return nil }, SubscribeConfig{})

	if bus.SubscriberCount("a") != 2 {
		t.Errorf("expected 2 for 'a', got %d", bus.SubscriberCount("a"))
	}
	if bus.SubscriberCount("b") != 1 {
		t.Errorf("expected 1 for 'b', got %d", bus.SubscriberCount("b"))
	}
}

func TestComplexFilterCombination(t *testing.T) {
	filter := &AndFilter{
		Filters: []Filter{
			&OrFilter{
				Filters: []Filter{
					&EqualsFilter{Key: "country", Value: "US"},
					&EqualsFilter{Key: "country", Value: "CA"},
				},
			},
			&RangeFilter{Key: "age", Min: 18, Max: 35, HasMin: true, HasMax: true},
			&NotFilter{
				Inner: &EqualsFilter{Key: "status", Value: "banned"},
			},
		},
	}

	tests := []struct {
		name       string
		attributes map[string]interface{}
		expected   bool
	}{
		{
			name:       "US adult active",
			attributes: map[string]interface{}{"country": "US", "age": 25, "status": "active"},
			expected:   true,
		},
		{
			name:       "CA adult active",
			attributes: map[string]interface{}{"country": "CA", "age": 30, "status": "active"},
			expected:   true,
		},
		{
			name:       "US too young",
			attributes: map[string]interface{}{"country": "US", "age": 15, "status": "active"},
			expected:   false,
		},
		{
			name:       "US banned",
			attributes: map[string]interface{}{"country": "US", "age": 25, "status": "banned"},
			expected:   false,
		},
		{
			name:       "wrong country",
			attributes: map[string]interface{}{"country": "UK", "age": 25, "status": "active"},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{Attributes: tt.attributes}
			result := filter.Match(event)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGenerateID(t *testing.T) {
	bus := NewEventBus()

	ids := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := bus.generateID()
		if ids[id] {
			t.Errorf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}
