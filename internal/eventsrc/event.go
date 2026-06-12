package eventsrc

import "time"

type Event struct {
	AggregateID string
	EventType   string
	Data        []byte
	Version     int64
	Timestamp   time.Time
}

func NewEvent(aggregateID, eventType string, data []byte, version int64) *Event {
	return &Event{
		AggregateID: aggregateID,
		EventType:   eventType,
		Data:        data,
		Version:     version,
		Timestamp:   time.Now(),
	}
}
