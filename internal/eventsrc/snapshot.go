package eventsrc

import "time"

type Snapshot struct {
	AggregateID string
	Version     int64
	State       []byte
	Timestamp   time.Time
}

func NewSnapshot(aggregateID string, version int64, state []byte) *Snapshot {
	return &Snapshot{
		AggregateID: aggregateID,
		Version:     version,
		State:       state,
		Timestamp:   time.Now(),
	}
}
