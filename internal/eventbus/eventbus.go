package eventbus

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

var (
	ErrSubscriberNotFound = errors.New("subscriber not found")
	ErrSubscriberExists   = errors.New("subscriber already exists")
	ErrInterrupt          = errors.New("subscriber interrupted dispatch")
)

type Event struct {
	Type       string
	Payload    interface{}
	Attributes map[string]interface{}
}

type HandlerFunc func(event Event) error

type Filter interface {
	Match(event Event) bool
}

type EqualsFilter struct {
	Key   string
	Value interface{}
}

func (f *EqualsFilter) Match(event Event) bool {
	if event.Attributes == nil {
		return false
	}
	val, ok := event.Attributes[f.Key]
	if !ok {
		return false
	}
	return reflect.DeepEqual(val, f.Value)
}

type RangeFilter struct {
	Key    string
	Min    float64
	Max    float64
	HasMin bool
	HasMax bool
}

func (f *RangeFilter) Match(event Event) bool {
	if event.Attributes == nil {
		return false
	}
	val, ok := event.Attributes[f.Key]
	if !ok {
		return false
	}
	floatVal, ok := toFloat64(val)
	if !ok {
		return false
	}
	if f.HasMin && floatVal < f.Min {
		return false
	}
	if f.HasMax && floatVal > f.Max {
		return false
	}
	return true
}

type AndFilter struct {
	Filters []Filter
}

func (f *AndFilter) Match(event Event) bool {
	for _, filter := range f.Filters {
		if !filter.Match(event) {
			return false
		}
	}
	return true
}

type OrFilter struct {
	Filters []Filter
}

func (f *OrFilter) Match(event Event) bool {
	for _, filter := range f.Filters {
		if filter.Match(event) {
			return true
		}
	}
	return false
}

type NotFilter struct {
	Inner Filter
}

func (f *NotFilter) Match(event Event) bool {
	return !f.Inner.Match(event)
}

func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	case float32:
		return float64(val), true
	case float64:
		return val, true
	default:
		return 0, false
	}
}

type subscriber struct {
	ID       string
	Handler  HandlerFunc
	Priority int
	Filter   Filter
}

type SubscribeConfig struct {
	ID       string
	Priority int
	Filter   Filter
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]*subscriber
	asyncWg     sync.WaitGroup
	nextID      uint64
	idMu        sync.Mutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]*subscriber),
	}
}

func (bus *EventBus) generateID() string {
	bus.idMu.Lock()
	defer bus.idMu.Unlock()
	bus.nextID++
	return fmt.Sprintf("sub-%d", bus.nextID)
}

func (bus *EventBus) Subscribe(eventType string, handler HandlerFunc, config SubscribeConfig) (string, error) {
	if handler == nil {
		return "", errors.New("handler cannot be nil")
	}

	id := config.ID
	if id == "" {
		id = bus.generateID()
	}

	sub := &subscriber{
		ID:       id,
		Handler:  handler,
		Priority: config.Priority,
		Filter:   config.Filter,
	}

	bus.mu.Lock()
	defer bus.mu.Unlock()

	for _, existing := range bus.subscribers[eventType] {
		if existing.ID == id {
			return "", ErrSubscriberExists
		}
	}

	bus.subscribers[eventType] = append(bus.subscribers[eventType], sub)
	return id, nil
}

func (bus *EventBus) Unsubscribe(eventType string, subscriberID string) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()

	subs, ok := bus.subscribers[eventType]
	if !ok {
		return ErrSubscriberNotFound
	}

	for i, sub := range subs {
		if sub.ID == subscriberID {
			bus.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
			if len(bus.subscribers[eventType]) == 0 {
				delete(bus.subscribers, eventType)
			}
			return nil
		}
	}

	return ErrSubscriberNotFound
}

func (bus *EventBus) getMatchedSubscribers(eventType string, event Event) []*subscriber {
	bus.mu.RLock()
	defer bus.mu.RUnlock()

	subs, ok := bus.subscribers[eventType]
	if !ok {
		return nil
	}

	var matched []*subscriber
	for _, sub := range subs {
		if sub.Filter == nil || sub.Filter.Match(event) {
			matched = append(matched, sub)
		}
	}

	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Priority > matched[j].Priority
	})

	return matched
}

func (bus *EventBus) PublishSync(event Event) error {
	matched := bus.getMatchedSubscribers(event.Type, event)
	if len(matched) == 0 {
		return nil
	}

	var firstErr error
	for _, sub := range matched {
		err := callHandler(sub.Handler, event)
		if err != nil {
			if errors.Is(err, ErrInterrupt) {
				return err
			}
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	return firstErr
}

func (bus *EventBus) PublishAsync(event Event) {
	matched := bus.getMatchedSubscribers(event.Type, event)
	if len(matched) == 0 {
		return
	}

	bus.asyncWg.Add(1)
	go func(subs []*subscriber, e Event) {
		defer bus.asyncWg.Done()
		for _, sub := range subs {
			err := callHandler(sub.Handler, e)
			if err != nil && errors.Is(err, ErrInterrupt) {
				return
			}
		}
	}(matched, event)
}

func (bus *EventBus) Wait() {
	bus.asyncWg.Wait()
}

func (bus *EventBus) SubscriberCount(eventType string) int {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	return len(bus.subscribers[eventType])
}

func (bus *EventBus) TotalSubscriberCount() int {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	total := 0
	for _, subs := range bus.subscribers {
		total += len(subs)
	}
	return total
}

func (bus *EventBus) HasSubscriber(eventType string, subscriberID string) bool {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	subs, ok := bus.subscribers[eventType]
	if !ok {
		return false
	}
	for _, sub := range subs {
		if sub.ID == subscriberID {
			return true
		}
	}
	return false
}

func (bus *EventBus) EventTypes() []string {
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	types := make([]string, 0, len(bus.subscribers))
	for t := range bus.subscribers {
		types = append(types, t)
	}
	return types
}

func callHandler(handler HandlerFunc, event Event) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			retErr = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return handler(event)
}
