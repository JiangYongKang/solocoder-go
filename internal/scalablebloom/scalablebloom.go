package scalablebloom

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"sync"
)

var (
	ErrInvalidFPRate       = errors.New("scalablebloom: false positive rate must be in (0, 1)")
	ErrInvalidCapacity     = errors.New("scalablebloom: capacity must be > 0")
	ErrInvalidRatio        = errors.New("scalablebloom: ratio must be in (0, 1)")
	ErrEmptyKey            = errors.New("scalablebloom: key must not be empty")
	ErrNoFilters           = errors.New("scalablebloom: no filters provided for union query")
	ErrFileOpen            = errors.New("scalablebloom: failed to open file")
	ErrFileWrite           = errors.New("scalablebloom: failed to write file")
	ErrFileRead            = errors.New("scalablebloom: failed to read file")
	ErrInvalidData         = errors.New("scalablebloom: invalid serialized data")
	ErrVersionMismatch     = errors.New("scalablebloom: version mismatch in serialized data")
	ErrVersionUnsupported  = errors.New("scalablebloom: serialized data version is too old and not supported")
	ErrCapacityExceeded    = errors.New("scalablebloom: bloom filter capacity exceeded")
	ErrIncompatibleFilters = errors.New("scalablebloom: filters have incompatible hash configurations for union query")
	ErrCorruptedFilter     = errors.New("scalablebloom: corrupted filter state detected")
)

const (
	version           uint32 = 2
	minSupportedVersion uint32 = 1
	defaultRatio      float64 = 0.85
)

type Config struct {
	InitialCapacity uint
	FPRate          float64
	Ratio           float64
}

func DefaultConfig() Config {
	return Config{
		InitialCapacity: 1000,
		FPRate:          0.01,
		Ratio:           defaultRatio,
	}
}

type bloomFilter struct {
	bits      []uint64
	numBits   uint
	hashCount uint
	capacity  uint
	count     uint
}

func newBloomFilter(capacity uint, fpRate float64) *bloomFilter {
	numBits := optimalNumBits(capacity, fpRate)
	hashCount := optimalHashCount(numBits, capacity)
	numWords := (numBits + 63) / 64
	return &bloomFilter{
		bits:      make([]uint64, numWords),
		numBits:   numBits,
		hashCount: hashCount,
		capacity:  capacity,
		count:     0,
	}
}

func optimalNumBits(n uint, p float64) uint {
	return uint(math.Ceil(-float64(n) * math.Log(p) / (math.Log(2) * math.Log(2))))
}

func optimalHashCount(m uint, n uint) uint {
	k := float64(m) / float64(n) * math.Log(2)
	ki := uint(math.Ceil(k))
	if ki < 1 {
		return 1
	}
	return ki
}

func doubleHash(key string, numBits uint) (uint, uint) {
	sum := sha256.Sum256([]byte(key))
	h1 := binary.BigEndian.Uint64(sum[0:8])
	h2 := binary.BigEndian.Uint64(sum[8:16])
	return uint(h1 % uint64(numBits)), uint(h2 % uint64(numBits))
}

func (bf *bloomFilter) add(key string) error {
	if bf.isFull() {
		return ErrCapacityExceeded
	}
	h1, h2 := doubleHash(key, bf.numBits)
	for i := uint(0); i < bf.hashCount; i++ {
		idx := (h1 + uint(i)*h2) % bf.numBits
		wordIdx := idx / 64
		bitIdx := idx % 64
		bf.bits[wordIdx] |= 1 << bitIdx
	}
	bf.count++
	return nil
}

func (bf *bloomFilter) mightContain(key string) bool {
	h1, h2 := doubleHash(key, bf.numBits)
	for i := uint(0); i < bf.hashCount; i++ {
		idx := (h1 + uint(i)*h2) % bf.numBits
		wordIdx := idx / 64
		bitIdx := idx % 64
		if bf.bits[wordIdx]&(1<<bitIdx) == 0 {
			return false
		}
	}
	return true
}

func (bf *bloomFilter) isFull() bool {
	return bf.count >= bf.capacity
}

func (bf *bloomFilter) validate() error {
	if bf.numBits == 0 {
		return ErrCorruptedFilter
	}
	if bf.hashCount == 0 {
		return ErrCorruptedFilter
	}
	if bf.capacity == 0 {
		return ErrCorruptedFilter
	}
	expectedWords := (bf.numBits + 63) / 64
	if uint(len(bf.bits)) != expectedWords {
		return ErrCorruptedFilter
	}
	return nil
}

func (bf *bloomFilter) fillRatio() float64 {
	if bf.capacity == 0 {
		return 0
	}
	return float64(bf.count) / float64(bf.capacity)
}

type ScalableBloom struct {
	mu      sync.Mutex
	filters []*bloomFilter
	cfg     Config
	count   uint
}

func New(cfg Config) (*ScalableBloom, error) {
	if cfg.InitialCapacity == 0 {
		return nil, ErrInvalidCapacity
	}
	if cfg.FPRate <= 0 || cfg.FPRate >= 1 {
		return nil, ErrInvalidFPRate
	}
	if cfg.Ratio <= 0 || cfg.Ratio >= 1 {
		return nil, ErrInvalidRatio
	}

	first := newBloomFilter(cfg.InitialCapacity, cfg.FPRate)
	return &ScalableBloom{
		filters: []*bloomFilter{first},
		cfg:     cfg,
		count:   0,
	}, nil
}

func (sb *ScalableBloom) Add(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	active := sb.filters[len(sb.filters)-1]
	if active.isFull() {
		if err := sb.expandLocked(); err != nil {
			return err
		}
		active = sb.filters[len(sb.filters)-1]
	}

	if err := active.add(key); err != nil {
		if errors.Is(err, ErrCapacityExceeded) {
			if err := sb.expandLocked(); err != nil {
				return err
			}
			active = sb.filters[len(sb.filters)-1]
			return active.add(key)
		}
		return err
	}
	sb.count++
	return nil
}

func (sb *ScalableBloom) expandLocked() error {
	last := sb.filters[len(sb.filters)-1]
	newCapacity := last.capacity * 2
	if newCapacity < last.capacity {
		return ErrCapacityExceeded
	}
	newFPRate := sb.cfg.FPRate * math.Pow(sb.cfg.Ratio, float64(len(sb.filters)))
	newFilter := newBloomFilter(newCapacity, newFPRate)
	sb.filters = append(sb.filters, newFilter)
	return nil
}

func (sb *ScalableBloom) FillRatio() float64 {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	if len(sb.filters) == 0 {
		return 0
	}
	active := sb.filters[len(sb.filters)-1]
	return active.fillRatio()
}

func (sb *ScalableBloom) MightContain(key string) (bool, error) {
	if key == "" {
		return false, ErrEmptyKey
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	for _, f := range sb.filters {
		if f.mightContain(key) {
			return true, nil
		}
	}
	return false, nil
}

func (sb *ScalableBloom) Count() uint {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.count
}

func (sb *ScalableBloom) FilterCount() int {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return len(sb.filters)
}

func (sb *ScalableBloom) Capacity() uint {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	var total uint
	for _, f := range sb.filters {
		total += f.capacity
	}
	return total
}

func (sb *ScalableBloom) Serialize(path string) error {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	for _, bf := range sb.filters {
		if err := bf.validate(); err != nil {
			return err
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return ErrFileOpen
	}
	defer f.Close()

	buf4 := make([]byte, 4)
	buf8 := make([]byte, 8)

	binary.BigEndian.PutUint32(buf4, version)
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	binary.BigEndian.PutUint32(buf4, minSupportedVersion)
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	binary.BigEndian.PutUint32(buf4, uint32(sb.cfg.InitialCapacity))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	fpBits := math.Float64bits(sb.cfg.FPRate)
	binary.BigEndian.PutUint32(buf4, uint32(fpBits>>32))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}
	binary.BigEndian.PutUint32(buf4, uint32(fpBits))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	ratioBits := math.Float64bits(sb.cfg.Ratio)
	binary.BigEndian.PutUint32(buf4, uint32(ratioBits>>32))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}
	binary.BigEndian.PutUint32(buf4, uint32(ratioBits))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	binary.BigEndian.PutUint32(buf4, uint32(sb.count))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	binary.BigEndian.PutUint32(buf4, uint32(len(sb.filters)))
	if _, err := f.Write(buf4); err != nil {
		return ErrFileWrite
	}

	for _, bf := range sb.filters {
		binary.BigEndian.PutUint32(buf4, uint32(bf.numBits))
		if _, err := f.Write(buf4); err != nil {
			return ErrFileWrite
		}
		binary.BigEndian.PutUint32(buf4, uint32(bf.hashCount))
		if _, err := f.Write(buf4); err != nil {
			return ErrFileWrite
		}
		binary.BigEndian.PutUint32(buf4, uint32(bf.capacity))
		if _, err := f.Write(buf4); err != nil {
			return ErrFileWrite
		}
		binary.BigEndian.PutUint32(buf4, uint32(bf.count))
		if _, err := f.Write(buf4); err != nil {
			return ErrFileWrite
		}
		binary.BigEndian.PutUint32(buf4, uint32(len(bf.bits)))
		if _, err := f.Write(buf4); err != nil {
			return ErrFileWrite
		}
		for _, word := range bf.bits {
			binary.BigEndian.PutUint64(buf8, word)
			if _, err := f.Write(buf8); err != nil {
				return ErrFileWrite
			}
		}
	}

	reserved := make([]byte, 32)
	if _, err := f.Write(reserved); err != nil {
		return ErrFileWrite
	}

	return nil
}

func Deserialize(path string) (*ScalableBloom, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, ErrFileRead
	}
	defer f.Close()

	buf4 := make([]byte, 4)
	buf8 := make([]byte, 8)

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	ver := binary.BigEndian.Uint32(buf4)

	if ver < minSupportedVersion {
		return nil, ErrVersionUnsupported
	}
	if ver > version {
		return nil, ErrVersionMismatch
	}

	var minVer uint32
	if ver >= 2 {
		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		minVer = binary.BigEndian.Uint32(buf4)
		if minVer > version {
			return nil, ErrVersionMismatch
		}
	}

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	initialCap := uint(binary.BigEndian.Uint32(buf4))

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	fpHi := binary.BigEndian.Uint32(buf4)
	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	fpLo := binary.BigEndian.Uint32(buf4)
	fpRate := math.Float64frombits(uint64(fpHi)<<32 | uint64(fpLo))

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	ratioHi := binary.BigEndian.Uint32(buf4)
	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	ratioLo := binary.BigEndian.Uint32(buf4)
	ratio := math.Float64frombits(uint64(ratioHi)<<32 | uint64(ratioLo))

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	totalCount := uint(binary.BigEndian.Uint32(buf4))

	if _, err := f.Read(buf4); err != nil {
		return nil, ErrInvalidData
	}
	numFilters := int(binary.BigEndian.Uint32(buf4))
	if numFilters <= 0 {
		return nil, ErrInvalidData
	}

	if initialCap == 0 {
		return nil, ErrInvalidData
	}
	if fpRate <= 0 || fpRate >= 1 {
		return nil, ErrInvalidData
	}
	if ratio <= 0 || ratio >= 1 {
		return nil, ErrInvalidData
	}

	filters := make([]*bloomFilter, 0, numFilters)
	for i := 0; i < numFilters; i++ {
		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		numBits := uint(binary.BigEndian.Uint32(buf4))

		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		hashCount := uint(binary.BigEndian.Uint32(buf4))

		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		capacity := uint(binary.BigEndian.Uint32(buf4))

		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		count := uint(binary.BigEndian.Uint32(buf4))

		if _, err := f.Read(buf4); err != nil {
			return nil, ErrInvalidData
		}
		numWords := int(binary.BigEndian.Uint32(buf4))
		if numWords < 0 {
			return nil, ErrInvalidData
		}

		bits := make([]uint64, numWords)
		for j := 0; j < numWords; j++ {
			if _, err := f.Read(buf8); err != nil {
				return nil, ErrInvalidData
			}
			bits[j] = binary.BigEndian.Uint64(buf8)
		}

		bf := &bloomFilter{
			bits:      bits,
			numBits:   numBits,
			hashCount: hashCount,
			capacity:  capacity,
			count:     count,
		}

		if err := bf.validate(); err != nil {
			return nil, ErrInvalidData
		}

		filters = append(filters, bf)
	}

	if ver >= 2 {
		reserved := make([]byte, 32)
		_, _ = f.Read(reserved)
	}

	sb := &ScalableBloom{
		filters: filters,
		cfg: Config{
			InitialCapacity: initialCap,
			FPRate:          fpRate,
			Ratio:           ratio,
		},
		count: totalCount,
	}

	if err := sb.validateLocked(); err != nil {
		return nil, ErrInvalidData
	}

	return sb, nil
}

func (sb *ScalableBloom) validateLocked() error {
	if len(sb.filters) == 0 {
		return ErrCorruptedFilter
	}
	var sum uint
	for _, bf := range sb.filters {
		if err := bf.validate(); err != nil {
			return err
		}
		sum += bf.count
	}
	if sb.count != sum {
		return ErrCorruptedFilter
	}
	return nil
}

func UnionQuery(filters []*ScalableBloom, key string) (bool, error) {
	if len(filters) == 0 {
		return false, ErrNoFilters
	}
	if key == "" {
		return false, ErrEmptyKey
	}

	if err := validateFiltersCompatible(filters); err != nil {
		return false, err
	}

	for _, sb := range filters {
		found, err := sb.MightContain(key)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func validateFiltersCompatible(filters []*ScalableBloom) error {
	if len(filters) <= 1 {
		return nil
	}

	first := filters[0]
	first.mu.Lock()
	firstCfg := first.cfg
	firstFilters := len(first.filters)
	var firstHashCount uint
	if firstFilters > 0 {
		firstHashCount = first.filters[0].hashCount
	}
	first.mu.Unlock()

	for i := 1; i < len(filters); i++ {
		sb := filters[i]
		sb.mu.Lock()
		if sb.cfg.FPRate != firstCfg.FPRate {
			sb.mu.Unlock()
			return ErrIncompatibleFilters
		}
		if sb.cfg.Ratio != firstCfg.Ratio {
			sb.mu.Unlock()
			return ErrIncompatibleFilters
		}
		if sb.cfg.InitialCapacity != firstCfg.InitialCapacity {
			sb.mu.Unlock()
			return ErrIncompatibleFilters
		}
		if len(sb.filters) > 0 && firstFilters > 0 {
			if sb.filters[0].hashCount != firstHashCount {
				sb.mu.Unlock()
				return ErrIncompatibleFilters
			}
		}
		sb.mu.Unlock()
	}

	return nil
}
