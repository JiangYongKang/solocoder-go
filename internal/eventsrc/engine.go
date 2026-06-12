package eventsrc

type EventSourcingEngine struct {
	eventStore    EventStore
	snapshotStore SnapshotStore
}

func NewEventSourcingEngine(eventStore EventStore, snapshotStore SnapshotStore) *EventSourcingEngine {
	return &EventSourcingEngine{
		eventStore:    eventStore,
		snapshotStore: snapshotStore,
	}
}

func (e *EventSourcingEngine) AppendEvents(aggregateID string, expectedVersion int64, events []*Event) error {
	return e.eventStore.AppendEvents(aggregateID, expectedVersion, events)
}

func (e *EventSourcingEngine) LoadEvents(aggregateID string, fromVersion int64) ([]*Event, error) {
	return e.eventStore.LoadEvents(aggregateID, fromVersion)
}

func (e *EventSourcingEngine) GetVersion(aggregateID string) (int64, error) {
	return e.eventStore.GetVersion(aggregateID)
}

func (e *EventSourcingEngine) ReplayEvents(aggregate Aggregate, events []*Event) error {
	if aggregate == nil {
		return ErrAggregateNil
	}

	for _, event := range events {
		if err := aggregate.Apply(event); err != nil {
			return err
		}
	}

	return nil
}

func (e *EventSourcingEngine) RebuildState(aggregate Aggregate) error {
	if aggregate == nil {
		return ErrAggregateNil
	}

	aggregateID := aggregate.AggregateID()
	if aggregateID == "" {
		return ErrInvalidAggregateID
	}

	fromVersion := int64(0)

	snapshot, err := e.snapshotStore.LoadSnapshot(aggregateID)
	if err == nil && snapshot != nil {
		if err := aggregate.UnmarshalState(snapshot.State); err != nil {
			return err
		}
		fromVersion = snapshot.Version
		if base, ok := aggregate.(interface{ SetVersion(int64) }); ok {
			base.SetVersion(snapshot.Version)
		}
	}

	events, err := e.eventStore.LoadEvents(aggregateID, fromVersion)
	if err != nil {
		if err == ErrAggregateNotFound && fromVersion == 0 {
			return nil
		}
		return err
	}

	return e.ReplayEvents(aggregate, events)
}

func (e *EventSourcingEngine) CreateSnapshot(aggregate Aggregate) error {
	if aggregate == nil {
		return ErrAggregateNil
	}

	aggregateID := aggregate.AggregateID()
	if aggregateID == "" {
		return ErrInvalidAggregateID
	}

	state, err := aggregate.MarshalState()
	if err != nil {
		return err
	}

	snapshot := NewSnapshot(aggregateID, aggregate.Version(), state)
	return e.snapshotStore.SaveSnapshot(snapshot)
}

func (e *EventSourcingEngine) SaveSnapshot(snapshot *Snapshot) error {
	return e.snapshotStore.SaveSnapshot(snapshot)
}

func (e *EventSourcingEngine) LoadSnapshot(aggregateID string) (*Snapshot, error) {
	return e.snapshotStore.LoadSnapshot(aggregateID)
}
