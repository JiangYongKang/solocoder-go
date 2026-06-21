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
	waitUntilNextMsMaxMs    int64 = 200
	sequenceOverflowMaxMs   int64 = 200
)

var (
	ErrInvalidMachineID = errors.New("snowflake: machine id out of range")
	ErrClockBackward     = errors.New("snowflake: clock moved backward")
	ErrClockBackwardMax  = errors.New("snowflake: clock moved backward beyond maximum wait")
	ErrSequenceOverflow  = errors.New("snowflake: sequence overflowed within same millisecond")
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
				if s.waitUntilNextMs(s.lastTS, waitUntilNextMsMaxMs) {
					return 0, fmt.Errorf("%w: offset %dms, exceeded max wait %dms", ErrClockBackwardMax, offset, waitUntilNextMsMaxMs)
				}
				continue
			}
			s.mu.Unlock()
			return 0, fmt.Errorf("%w: offset %dms", ErrClockBackward, offset)
		}

		if now == s.lastTS {
			if s.sequence >= maxSequence {
				s.mu.Unlock()
				if s.waitUntilNextMs(s.lastTS, sequenceOverflowMaxMs) {
					return 0, fmt.Errorf("%w: sequence reached max %d, clock did not advance after %dms", ErrSequenceOverflow, maxSequence, sequenceOverflowMaxMs)
				}
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

func (s *Snowflake) waitUntilNextMs(targetTS int64, maxWaitMs int64) bool {
	start := time.Now()
	timeout := time.Duration(maxWaitMs) * time.Millisecond
	for s.timestamp() <= targetTS {
		if time.Since(start) >= timeout {
			return true
		}
		time.Sleep(100 * time.Microsecond)
	}
	return false
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
