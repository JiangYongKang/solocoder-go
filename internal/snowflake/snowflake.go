package snowflake

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	epoch         int64 = 1704067200000
	timestampBits uint8 = 41
	machineIDBits uint8 = 10
	sequenceBits  uint8 = 12

	maxMachineID int64 = (1 << machineIDBits) - 1
	maxSequence  int64 = (1 << sequenceBits) - 1

	machineIDShift uint8 = sequenceBits
	timestampShift uint8 = sequenceBits + machineIDBits

	clockBackwardSmallMaxMs int64 = 5
)

var (
	ErrInvalidMachineID = errors.New("snowflake: machine id out of range")
	ErrClockBackward     = errors.New("snowflake: clock moved backward")
)

type ID int64

type ParsedID struct {
	Timestamp int64
	MachineID int64
	Sequence  int64
}

type Snowflake struct {
	mu        sync.Mutex
	machineID int64
	lastTS    int64
	sequence  int64
	nowFunc   func() time.Time
}

type Config struct {
	MachineID int64
}

func New(cfg Config) (*Snowflake, error) {
	if cfg.MachineID < 0 || cfg.MachineID > maxMachineID {
		return nil, fmt.Errorf("%w: machine id %d is not in range [0, %d]", ErrInvalidMachineID, cfg.MachineID, maxMachineID)
	}
	return &Snowflake{
		machineID: cfg.MachineID,
		lastTS:    -1,
		nowFunc:   time.Now,
	}, nil
}

func (s *Snowflake) Next() (ID, error) {
	for {
		s.mu.Lock()
		now := s.timestamp()

		if now < s.lastTS {
			offset := s.lastTS - now
			if offset <= clockBackwardSmallMaxMs {
				s.mu.Unlock()
				time.Sleep(time.Duration(offset) * time.Millisecond)
				continue
			}
			s.mu.Unlock()
			return 0, fmt.Errorf("%w: offset %dms", ErrClockBackward, offset)
		}

		if now == s.lastTS {
			if s.sequence >= maxSequence {
				s.mu.Unlock()
				s.waitUntilNextMs(s.lastTS)
				continue
			}
			s.sequence++
		} else {
			s.sequence = 0
		}

		s.lastTS = now
		id := ID((now << timestampShift) | (s.machineID << machineIDShift) | s.sequence)
		s.mu.Unlock()
		return id, nil
	}
}

func (s *Snowflake) timestamp() int64 {
	return s.nowFunc().UnixMilli() - epoch
}

func (s *Snowflake) waitUntilNextMs(lastTS int64) {
	for s.timestamp() <= lastTS {
		time.Sleep(100 * time.Microsecond)
	}
}

func Parse(id ID) ParsedID {
	return ParsedID{
		Timestamp: int64(id) >> timestampShift,
		MachineID: (int64(id) >> machineIDShift) & maxMachineID,
		Sequence:  int64(id) & maxSequence,
	}
}

func (p ParsedID) Time() time.Time {
	return time.UnixMilli(p.Timestamp + epoch)
}

func Decompose(id ID) ParsedID {
	return Parse(id)
}
