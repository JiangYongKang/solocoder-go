package eventsrc

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent("agg-1", "TestEvent", []byte("data"), 1)
	if event.AggregateID != "agg-1" {
		t.Errorf("expected AggregateID to be 'agg-1', got '%s'", event.AggregateID)
	}
	if event.EventType != "TestEvent" {
		t.Errorf("expected EventType to be 'TestEvent', got '%s'", event.EventType)
	}
	if string(event.Data) != "data" {
		t.Errorf("expected Data to be 'data', got '%s'", string(event.Data))
	}
	if event.Version != 1 {
		t.Errorf("expected Version to be 1, got %d", event.Version)
	}
	if event.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestNewSnapshot(t *testing.T) {
	snapshot := NewSnapshot("agg-1", 5, []byte("state"))
	if snapshot.AggregateID != "agg-1" {
		t.Errorf("expected AggregateID to be 'agg-1', got '%s'", snapshot.AggregateID)
	}
	if snapshot.Version != 5 {
		t.Errorf("expected Version to be 5, got %d", snapshot.Version)
	}
	if string(snapshot.State) != "state" {
		t.Errorf("expected State to be 'state', got '%s'", string(snapshot.State))
	}
	if snapshot.Timestamp.IsZero() {
		t.Error("expected Timestamp to be set")
	}
}

func TestBaseAggregate(t *testing.T) {
	agg := NewBaseAggregate("agg-1")
	if agg.AggregateID() != "agg-1" {
		t.Errorf("expected AggregateID to be 'agg-1', got '%s'", agg.AggregateID())
	}
	if agg.Version() != 0 {
		t.Errorf("expected Version to be 0, got %d", agg.Version())
	}

	agg.IncrementVersion()
	if agg.Version() != 1 {
		t.Errorf("expected Version to be 1 after increment, got %d", agg.Version())
	}

	agg.SetVersion(10)
	if agg.Version() != 10 {
		t.Errorf("expected Version to be 10 after set, got %d", agg.Version())
	}
}

func TestInMemoryEventStore_AppendEvents_SingleEvent(t *testing.T) {
	store := NewInMemoryEventStore()
	event := NewEvent("agg-1", "TestEvent", []byte("data"), 0)

	err := store.AppendEvents("agg-1", 0, []*Event{event})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := store.GetVersion("agg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

func TestInMemoryEventStore_AppendEvents_MultipleEvents(t *testing.T) {
	store := NewInMemoryEventStore()
	events := []*Event{
		NewEvent("agg-1", "Event1", []byte("data1"), 0),
		NewEvent("agg-1", "Event2", []byte("data2"), 0),
		NewEvent("agg-1", "Event3", []byte("data3"), 0),
	}

	err := store.AppendEvents("agg-1", 0, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := store.GetVersion("agg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 3 {
		t.Errorf("expected version 3, got %d", version)
	}

	loadedEvents, err := store.LoadEvents("agg-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loadedEvents) != 3 {
		t.Errorf("expected 3 events, got %d", len(loadedEvents))
	}
	for i, e := range loadedEvents {
		expectedVersion := int64(i + 1)
		if e.Version != expectedVersion {
			t.Errorf("event %d: expected version %d, got %d", i, expectedVersion, e.Version)
		}
	}
}

func TestInMemoryEventStore_AppendEvents_VersionConflict(t *testing.T) {
	store := NewInMemoryEventStore()
	event1 := NewEvent("agg-1", "Event1", []byte("data1"), 0)

	err := store.AppendEvents("agg-1", 0, []*Event{event1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event2 := NewEvent("agg-1", "Event2", []byte("data2"), 0)
	err = store.AppendEvents("agg-1", 0, []*Event{event2})
	if !errors.Is(err, ErrVersionConflict) {
		t.Errorf("expected ErrVersionConflict, got %v", err)
	}
}

func TestInMemoryEventStore_AppendEvents_EmptyAggregateID(t *testing.T) {
	store := NewInMemoryEventStore()
	event := NewEvent("", "TestEvent", []byte("data"), 0)

	err := store.AppendEvents("", 0, []*Event{event})
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestInMemoryEventStore_AppendEvents_EmptyEvents(t *testing.T) {
	store := NewInMemoryEventStore()

	err := store.AppendEvents("agg-1", 0, []*Event{})
	if !errors.Is(err, ErrInvalidEvent) {
		t.Errorf("expected ErrInvalidEvent, got %v", err)
	}
}

func TestInMemoryEventStore_AppendEvents_NilEvent(t *testing.T) {
	store := NewInMemoryEventStore()

	err := store.AppendEvents("agg-1", 0, []*Event{nil})
	if !errors.Is(err, ErrEventNil) {
		t.Errorf("expected ErrEventNil, got %v", err)
	}
}

func TestInMemoryEventStore_AppendEvents_NewAggregate(t *testing.T) {
	store := NewInMemoryEventStore()

	_, err := store.GetVersion("nonexistent")
	if !errors.Is(err, ErrAggregateNotFound) {
		t.Errorf("expected ErrAggregateNotFound for nonexistent aggregate, got %v", err)
	}

	event := NewEvent("agg-new", "Created", []byte("data"), 0)
	err = store.AppendEvents("agg-new", 0, []*Event{event})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := store.GetVersion("agg-new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

func TestInMemoryEventStore_LoadEvents_FromVersion(t *testing.T) {
	store := NewInMemoryEventStore()
	events := []*Event{
		NewEvent("agg-1", "Event1", []byte("data1"), 0),
		NewEvent("agg-1", "Event2", []byte("data2"), 0),
		NewEvent("agg-1", "Event3", []byte("data3"), 0),
		NewEvent("agg-1", "Event4", []byte("data4"), 0),
		NewEvent("agg-1", "Event5", []byte("data5"), 0),
	}

	err := store.AppendEvents("agg-1", 0, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loadedEvents, err := store.LoadEvents("agg-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loadedEvents) != 3 {
		t.Errorf("expected 3 events from version 2, got %d", len(loadedEvents))
	}
	if loadedEvents[0].Version != 3 {
		t.Errorf("expected first event version to be 3, got %d", loadedEvents[0].Version)
	}
}

func TestInMemoryEventStore_LoadEvents_AggregateNotFound(t *testing.T) {
	store := NewInMemoryEventStore()

	_, err := store.LoadEvents("nonexistent", 0)
	if !errors.Is(err, ErrAggregateNotFound) {
		t.Errorf("expected ErrAggregateNotFound, got %v", err)
	}
}

func TestInMemoryEventStore_LoadEvents_EmptyAggregateID(t *testing.T) {
	store := NewInMemoryEventStore()

	_, err := store.LoadEvents("", 0)
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestInMemoryEventStore_GetVersion_EmptyAggregateID(t *testing.T) {
	store := NewInMemoryEventStore()

	_, err := store.GetVersion("")
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestInMemoryEventStore_ConcurrentAppend(t *testing.T) {
	store := NewInMemoryEventStore()
	aggregateID := "agg-concurrent"

	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	var mu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			for retries := 0; retries < 5; retries++ {
				version, _ := store.GetVersion(aggregateID)
				event := NewEvent(aggregateID, "Event", []byte("data"), 0)
				err := store.AppendEvents(aggregateID, version, []*Event{event})
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()
					return
				}
				if errors.Is(err, ErrVersionConflict) {
					continue
				}
				t.Errorf("unexpected error: %v", err)
				return
			}
			mu.Lock()
			failCount++
			mu.Unlock()
		}(i)
	}

	wg.Wait()

	version, err := store.GetVersion(aggregateID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != int64(successCount) {
		t.Errorf("expected version %d, got %d", successCount, version)
	}
	if successCount != 10 {
		t.Errorf("expected 10 successful appends with retries, got %d", successCount)
	}
}

func TestInMemorySnapshotStore_SaveAndLoad(t *testing.T) {
	store := NewInMemorySnapshotStore()
	snapshot := NewSnapshot("agg-1", 5, []byte("state-data"))

	err := store.SaveSnapshot(snapshot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := store.LoadSnapshot("agg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Version != 5 {
		t.Errorf("expected version 5, got %d", loaded.Version)
	}
	if string(loaded.State) != "state-data" {
		t.Errorf("expected state 'state-data', got '%s'", string(loaded.State))
	}
}

func TestInMemorySnapshotStore_OverwriteSnapshot(t *testing.T) {
	store := NewInMemorySnapshotStore()

	snapshot1 := NewSnapshot("agg-1", 3, []byte("old-state"))
	err := store.SaveSnapshot(snapshot1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot2 := NewSnapshot("agg-1", 7, []byte("new-state"))
	err = store.SaveSnapshot(snapshot2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	loaded, err := store.LoadSnapshot("agg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded.Version != 7 {
		t.Errorf("expected version 7, got %d", loaded.Version)
	}
	if string(loaded.State) != "new-state" {
		t.Errorf("expected state 'new-state', got '%s'", string(loaded.State))
	}
}

func TestInMemorySnapshotStore_NotFound(t *testing.T) {
	store := NewInMemorySnapshotStore()

	_, err := store.LoadSnapshot("nonexistent")
	if !errors.Is(err, ErrSnapshotNotFound) {
		t.Errorf("expected ErrSnapshotNotFound, got %v", err)
	}
}

func TestInMemorySnapshotStore_NilSnapshot(t *testing.T) {
	store := NewInMemorySnapshotStore()

	err := store.SaveSnapshot(nil)
	if !errors.Is(err, ErrSnapshotNil) {
		t.Errorf("expected ErrSnapshotNil, got %v", err)
	}
}

func TestInMemorySnapshotStore_EmptyAggregateID(t *testing.T) {
	store := NewInMemorySnapshotStore()

	snapshot := NewSnapshot("", 1, []byte("data"))
	err := store.SaveSnapshot(snapshot)
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}

	_, err = store.LoadSnapshot("")
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestEventSourcingEngine_ReplayEvents(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())
	account := NewTestAccount("acc-1")

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Alice"})
	depositData, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	withdrawData, _ := json.Marshal(map[string]interface{}{"amount": 30.0})

	events := []*Event{
		{EventType: "AccountCreated", Data: createdData, Version: 1},
		{EventType: "Deposit", Data: depositData, Version: 2},
		{EventType: "Withdraw", Data: withdrawData, Version: 3},
	}

	err := engine.ReplayEvents(account, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if account.Owner != "Alice" {
		t.Errorf("expected owner 'Alice', got '%s'", account.Owner)
	}
	if account.Balance != 70.0 {
		t.Errorf("expected balance 70.0, got %f", account.Balance)
	}
	if !account.Active {
		t.Error("expected account to be active")
	}
	if account.Version() != 3 {
		t.Errorf("expected version 3, got %d", account.Version())
	}
}

func TestEventSourcingEngine_ReplayEvents_NilAggregate(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())

	err := engine.ReplayEvents(nil, []*Event{})
	if !errors.Is(err, ErrAggregateNil) {
		t.Errorf("expected ErrAggregateNil, got %v", err)
	}
}

func TestEventSourcingEngine_ReplayEvents_UnknownEventType(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())
	account := NewTestAccount("acc-1")

	events := []*Event{
		{EventType: "UnknownEvent", Data: []byte("{}"), Version: 1},
	}

	err := engine.ReplayEvents(account, events)
	if err == nil {
		t.Error("expected error for unknown event type")
	}
}

func TestEventSourcingEngine_RebuildState_FromEvents(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Bob"})
	depositData, _ := json.Marshal(map[string]interface{}{"amount": 500.0})

	events := []*Event{
		NewEvent("acc-1", "AccountCreated", createdData, 0),
		NewEvent("acc-1", "Deposit", depositData, 0),
	}

	err := eventStore.AppendEvents("acc-1", 0, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account := NewTestAccount("acc-1")
	err = engine.RebuildState(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if account.Owner != "Bob" {
		t.Errorf("expected owner 'Bob', got '%s'", account.Owner)
	}
	if account.Balance != 500.0 {
		t.Errorf("expected balance 500.0, got %f", account.Balance)
	}
	if account.Version() != 2 {
		t.Errorf("expected version 2, got %d", account.Version())
	}
}

func TestEventSourcingEngine_RebuildState_FromSnapshotAndEvents(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Charlie"})
	depositData1, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	depositData2, _ := json.Marshal(map[string]interface{}{"amount": 100.0})

	initialEvents := []*Event{
		NewEvent("acc-1", "AccountCreated", createdData, 0),
		NewEvent("acc-1", "Deposit", depositData1, 0),
		NewEvent("acc-1", "Deposit", depositData2, 0),
	}
	err := eventStore.AppendEvents("acc-1", 0, initialEvents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	account := NewTestAccount("acc-1")
	err = engine.ReplayEvents(account, initialEvents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = engine.CreateSnapshot(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	depositData3, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	withdrawData, _ := json.Marshal(map[string]interface{}{"amount": 50.0})
	subsequentEvents := []*Event{
		NewEvent("acc-1", "Deposit", depositData3, 0),
		NewEvent("acc-1", "Withdraw", withdrawData, 0),
	}
	err = eventStore.AppendEvents("acc-1", 3, subsequentEvents)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	rebuiltAccount := NewTestAccount("acc-1")
	err = engine.RebuildState(rebuiltAccount)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rebuiltAccount.Owner != "Charlie" {
		t.Errorf("expected owner 'Charlie', got '%s'", rebuiltAccount.Owner)
	}
	if rebuiltAccount.Balance != 250.0 {
		t.Errorf("expected balance 250.0, got %f", rebuiltAccount.Balance)
	}
	if rebuiltAccount.Version() != 5 {
		t.Errorf("expected version 5, got %d", rebuiltAccount.Version())
	}
}

func TestEventSourcingEngine_RebuildState_NoEventsNoSnapshot(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	account := NewTestAccount("acc-new")
	err := engine.RebuildState(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if account.Version() != 0 {
		t.Errorf("expected version 0, got %d", account.Version())
	}
	if account.Balance != 0 {
		t.Errorf("expected balance 0, got %f", account.Balance)
	}
}

func TestEventSourcingEngine_RebuildState_NilAggregate(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())

	err := engine.RebuildState(nil)
	if !errors.Is(err, ErrAggregateNil) {
		t.Errorf("expected ErrAggregateNil, got %v", err)
	}
}

func TestEventSourcingEngine_RebuildState_EmptyAggregateID(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())
	account := NewTestAccount("")

	err := engine.RebuildState(account)
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestEventSourcingEngine_CreateSnapshot(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	account := NewTestAccount("acc-1")
	account.Owner = "Dave"
	account.Balance = 1000.0
	account.Active = true
	account.SetVersion(5)

	err := engine.CreateSnapshot(account)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	snapshot, err := snapshotStore.LoadSnapshot("acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snapshot.Version != 5 {
		t.Errorf("expected snapshot version 5, got %d", snapshot.Version)
	}

	restoredAccount := NewTestAccount("acc-1")
	err = restoredAccount.UnmarshalState(snapshot.State)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if restoredAccount.Owner != "Dave" {
		t.Errorf("expected owner 'Dave', got '%s'", restoredAccount.Owner)
	}
	if restoredAccount.Balance != 1000.0 {
		t.Errorf("expected balance 1000.0, got %f", restoredAccount.Balance)
	}
}

func TestEventSourcingEngine_CreateSnapshot_NilAggregate(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())

	err := engine.CreateSnapshot(nil)
	if !errors.Is(err, ErrAggregateNil) {
		t.Errorf("expected ErrAggregateNil, got %v", err)
	}
}

func TestEventSourcingEngine_CreateSnapshot_EmptyAggregateID(t *testing.T) {
	engine := NewEventSourcingEngine(NewInMemoryEventStore(), NewInMemorySnapshotStore())
	account := NewTestAccount("")

	err := engine.CreateSnapshot(account)
	if !errors.Is(err, ErrInvalidAggregateID) {
		t.Errorf("expected ErrInvalidAggregateID, got %v", err)
	}
}

func TestEventSourcingEngine_AppendEvents(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Eve"})
	events := []*Event{
		NewEvent("acc-1", "AccountCreated", createdData, 0),
	}

	err := engine.AppendEvents("acc-1", 0, events)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := engine.GetVersion("acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

func TestEventSourcingEngine_OptimisticLockRetry(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Frank"})
	err := engine.AppendEvents("acc-1", 0, []*Event{
		NewEvent("acc-1", "AccountCreated", createdData, 0),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	appendWithRetry := func(aggregateID string, event *Event, maxRetries int) error {
		for i := 0; i < maxRetries; i++ {
			version, err := engine.GetVersion(aggregateID)
			if err != nil {
				return err
			}
			err = engine.AppendEvents(aggregateID, version, []*Event{event})
			if err == nil {
				return nil
			}
			if errors.Is(err, ErrVersionConflict) {
				continue
			}
			return err
		}
		return ErrVersionConflict
	}

	depositData, _ := json.Marshal(map[string]interface{}{"amount": 50.0})
	err = appendWithRetry("acc-1", NewEvent("acc-1", "Deposit", depositData, 0), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, err := engine.GetVersion("acc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}
}

func TestEventSourcingEngine_FullWorkflow(t *testing.T) {
	eventStore := NewInMemoryEventStore()
	snapshotStore := NewInMemorySnapshotStore()
	engine := NewEventSourcingEngine(eventStore, snapshotStore)

	aggregateID := "acc-full"

	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Grace"})
	err := engine.AppendEvents(aggregateID, 0, []*Event{
		NewEvent(aggregateID, "AccountCreated", createdData, 0),
	})
	if err != nil {
		t.Fatalf("failed to create account: %v", err)
	}

	depositData1, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	depositData2, _ := json.Marshal(map[string]interface{}{"amount": 200.0})
	err = engine.AppendEvents(aggregateID, 1, []*Event{
		NewEvent(aggregateID, "Deposit", depositData1, 0),
		NewEvent(aggregateID, "Deposit", depositData2, 0),
	})
	if err != nil {
		t.Fatalf("failed to deposit: %v", err)
	}

	account := NewTestAccount(aggregateID)
	err = engine.RebuildState(account)
	if err != nil {
		t.Fatalf("failed to rebuild state: %v", err)
	}
	if account.Balance != 300.0 {
		t.Errorf("expected balance 300.0, got %f", account.Balance)
	}
	if account.Version() != 3 {
		t.Errorf("expected version 3, got %d", account.Version())
	}

	err = engine.CreateSnapshot(account)
	if err != nil {
		t.Fatalf("failed to create snapshot: %v", err)
	}

	withdrawData, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	err = engine.AppendEvents(aggregateID, 3, []*Event{
		NewEvent(aggregateID, "Withdraw", withdrawData, 0),
	})
	if err != nil {
		t.Fatalf("failed to withdraw: %v", err)
	}

	rebuiltAccount := NewTestAccount(aggregateID)
	err = engine.RebuildState(rebuiltAccount)
	if err != nil {
		t.Fatalf("failed to rebuild state from snapshot: %v", err)
	}
	if rebuiltAccount.Balance != 200.0 {
		t.Errorf("expected balance 200.0 after rebuild, got %f", rebuiltAccount.Balance)
	}
	if rebuiltAccount.Version() != 4 {
		t.Errorf("expected version 4 after rebuild, got %d", rebuiltAccount.Version())
	}

	snapshot, err := engine.LoadSnapshot(aggregateID)
	if err != nil {
		t.Fatalf("failed to load snapshot: %v", err)
	}
	if snapshot.Version != 3 {
		t.Errorf("expected snapshot version 3, got %d", snapshot.Version)
	}
}

func TestTestAccount_Apply_InsufficientBalance(t *testing.T) {
	account := NewTestAccount("acc-1")
	createdData, _ := json.Marshal(map[string]interface{}{"owner": "Test"})
	account.Apply(&Event{EventType: "AccountCreated", Data: createdData, Version: 1})

	withdrawData, _ := json.Marshal(map[string]interface{}{"amount": 100.0})
	err := account.Apply(&Event{EventType: "Withdraw", Data: withdrawData, Version: 2})
	if err == nil {
		t.Error("expected error for insufficient balance")
	}
}
