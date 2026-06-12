package eventsrc

type Aggregate interface {
	AggregateID() string
	Version() int64
	SetVersion(version int64)
	Apply(event *Event) error
	MarshalState() ([]byte, error)
	UnmarshalState(data []byte) error
}

type BaseAggregate struct {
	id      string
	version int64
}

func NewBaseAggregate(id string) *BaseAggregate {
	return &BaseAggregate{
		id:      id,
		version: 0,
	}
}

func (a *BaseAggregate) AggregateID() string {
	return a.id
}

func (a *BaseAggregate) Version() int64 {
	return a.version
}

func (a *BaseAggregate) IncrementVersion() {
	a.version++
}

func (a *BaseAggregate) SetVersion(version int64) {
	a.version = version
}
