package hotcold

import (
	"errors"
	"math"
	"sync"
	"time"
)

var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrInvalidConfig   = errors.New("invalid configuration")
	ErrNilManager      = errors.New("nil manager")
	ErrEmptyKey        = errors.New("empty key")
)

type DataTier int

const (
	TierCold DataTier = iota
	TierHot
)

type DataEntry struct {
	Key               string
	Value             interface{}
	AccessCount       int64
	LastAccessTime    time.Time
	CreatedAt         time.Time
	Tier              DataTier
	ConsecutiveColdCycles int
	Score             float64
}

type Config struct {
	HotThreshold         float64
	ColdThreshold        float64
	DecayHalfLife        time.Duration
	ColdCheckCycles      int
	HotCapacityRatio     float64
	AutoAdjustThresholds bool
	AdjustInterval       time.Duration
	MinHotThreshold      float64
	MaxHotThreshold      float64
	MinColdThreshold     float64
	MaxColdThreshold     float64
}

func DefaultConfig() Config {
	return Config{
		HotThreshold:         10.0,
		ColdThreshold:        2.0,
		DecayHalfLife:        time.Hour,
		ColdCheckCycles:      3,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: true,
		AdjustInterval:       5 * time.Minute,
		MinHotThreshold:      5.0,
		MaxHotThreshold:      50.0,
		MinColdThreshold:     0.5,
		MaxColdThreshold:     10.0,
	}
}

type HotColdManager struct {
	mu                sync.RWMutex
	hotStore          map[string]*DataEntry
	coldStore         map[string]*DataEntry
	cfg               Config
	lastAdjustTime    time.Time
	totalAccesses     int64
	accessesLastEpoch int64
}

func ValidateConfig(cfg Config) error {
	if cfg.HotCapacityRatio <= 0 || cfg.HotCapacityRatio >= 1 {
		return ErrInvalidConfig
	}
	if cfg.MinHotThreshold <= 0 || cfg.MaxHotThreshold <= cfg.MinHotThreshold {
		return ErrInvalidConfig
	}
	if cfg.MinColdThreshold <= 0 || cfg.MaxColdThreshold <= cfg.MinColdThreshold {
		return ErrInvalidConfig
	}
	if cfg.HotThreshold <= 0 || cfg.ColdThreshold <= 0 {
		return ErrInvalidConfig
	}
	if cfg.HotThreshold <= cfg.ColdThreshold {
		return ErrInvalidConfig
	}
	if cfg.DecayHalfLife <= 0 {
		return ErrInvalidConfig
	}
	if cfg.ColdCheckCycles <= 0 {
		return ErrInvalidConfig
	}
	if cfg.AdjustInterval <= 0 {
		return ErrInvalidConfig
	}
	return nil
}

func NewHotColdManager() *HotColdManager {
	m, _ := NewHotColdManagerWithConfig(DefaultConfig())
	return m
}

func NewHotColdManagerWithConfig(cfg Config) (*HotColdManager, error) {
	if cfg.HotCapacityRatio < 0 || cfg.HotCapacityRatio >= 1 {
		return nil, ErrInvalidConfig
	}
	if cfg.MinHotThreshold < 0 || (cfg.MinHotThreshold > 0 && cfg.MaxHotThreshold > 0 && cfg.MaxHotThreshold <= cfg.MinHotThreshold) {
		return nil, ErrInvalidConfig
	}
	if cfg.MinColdThreshold < 0 || (cfg.MinColdThreshold > 0 && cfg.MaxColdThreshold > 0 && cfg.MaxColdThreshold <= cfg.MinColdThreshold) {
		return nil, ErrInvalidConfig
	}
	if cfg.HotThreshold < 0 || cfg.ColdThreshold < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.HotThreshold > 0 && cfg.ColdThreshold > 0 && cfg.HotThreshold <= cfg.ColdThreshold {
		return nil, ErrInvalidConfig
	}
	if cfg.DecayHalfLife < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.ColdCheckCycles < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.AdjustInterval < 0 {
		return nil, ErrInvalidConfig
	}

	if cfg.HotThreshold == 0 {
		cfg.HotThreshold = 10.0
	}
	if cfg.ColdThreshold == 0 {
		cfg.ColdThreshold = 2.0
	}
	if cfg.HotThreshold <= cfg.ColdThreshold {
		cfg.HotThreshold = cfg.ColdThreshold * 5
	}
	if cfg.DecayHalfLife == 0 {
		cfg.DecayHalfLife = time.Hour
	}
	if cfg.ColdCheckCycles == 0 {
		cfg.ColdCheckCycles = 3
	}
	if cfg.HotCapacityRatio == 0 {
		cfg.HotCapacityRatio = 0.3
	}
	if cfg.AdjustInterval == 0 {
		cfg.AdjustInterval = 5 * time.Minute
	}
	if cfg.MinHotThreshold == 0 {
		cfg.MinHotThreshold = 5.0
	}
	if cfg.MaxHotThreshold == 0 {
		cfg.MaxHotThreshold = cfg.MinHotThreshold * 10
	}
	if cfg.MinColdThreshold == 0 {
		cfg.MinColdThreshold = 0.5
	}
	if cfg.MaxColdThreshold == 0 {
		cfg.MaxColdThreshold = cfg.MinColdThreshold * 20
	}

	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	return &HotColdManager{
		hotStore:          make(map[string]*DataEntry),
		coldStore:         make(map[string]*DataEntry),
		cfg:               cfg,
		lastAdjustTime:    time.Now(),
		accessesLastEpoch: 0,
	}, nil
}

func (m *HotColdManager) calculateScore(entry *DataEntry, now time.Time) float64 {
	if entry.AccessCount == 0 {
		return 0
	}

	timeSinceLastAccess := now.Sub(entry.LastAccessTime)
	decayFactor := math.Exp(-timeSinceLastAccess.Seconds() / m.cfg.DecayHalfLife.Seconds() * math.Ln2)

	baseScore := float64(entry.AccessCount) * decayFactor

	timeSinceCreation := now.Sub(entry.CreatedAt)
	if timeSinceCreation < m.cfg.DecayHalfLife {
		boost := 1.0 + (1.0 - float64(timeSinceCreation)/float64(m.cfg.DecayHalfLife))*0.5
		baseScore *= boost
	}

	return baseScore
}

func (m *HotColdManager) Put(key string, value interface{}) error {
	if m == nil {
		return ErrNilManager
	}
	if key == "" {
		return ErrEmptyKey
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	m.totalAccesses++

	if entry, ok := m.hotStore[key]; ok {
		entry.Value = value
		entry.AccessCount++
		entry.LastAccessTime = now
		entry.Score = m.calculateScore(entry, now)
		entry.ConsecutiveColdCycles = 0
		return nil
	}

	if entry, ok := m.coldStore[key]; ok {
		entry.Value = value
		entry.AccessCount++
		entry.LastAccessTime = now
		entry.Score = m.calculateScore(entry, now)

		if entry.Score >= m.cfg.HotThreshold {
			m.promoteToHot(entry)
		}
		return nil
	}

	entry := &DataEntry{
		Key:                   key,
		Value:                 value,
		AccessCount:           1,
		LastAccessTime:        now,
		CreatedAt:             now,
		Tier:                  TierCold,
		ConsecutiveColdCycles: 0,
		Score:                 0,
	}
	entry.Score = m.calculateScore(entry, now)
	m.coldStore[key] = entry

	return nil
}

func (m *HotColdManager) Get(key string) (interface{}, bool, error) {
	if m == nil {
		return nil, false, ErrNilManager
	}
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	if entry, ok := m.hotStore[key]; ok {
		entry.AccessCount++
		entry.LastAccessTime = now
		entry.Score = m.calculateScore(entry, now)
		entry.ConsecutiveColdCycles = 0
		m.totalAccesses++
		return entry.Value, true, nil
	}

	if entry, ok := m.coldStore[key]; ok {
		entry.AccessCount++
		entry.LastAccessTime = now
		entry.Score = m.calculateScore(entry, now)
		m.totalAccesses++

		if entry.Score >= m.cfg.HotThreshold {
			m.promoteToHot(entry)
		}

		return entry.Value, true, nil
	}

	return nil, false, ErrKeyNotFound
}

func (m *HotColdManager) GetScore(key string) (float64, bool, error) {
	if m == nil {
		return 0, false, ErrNilManager
	}
	if key == "" {
		return 0, false, ErrEmptyKey
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	if entry, ok := m.hotStore[key]; ok {
		liveScore := m.calculateScore(entry, now)
		return liveScore, true, nil
	}
	if entry, ok := m.coldStore[key]; ok {
		liveScore := m.calculateScore(entry, now)
		return liveScore, true, nil
	}

	return 0, false, ErrKeyNotFound
}

func (m *HotColdManager) Delete(key string) bool {
	if m == nil || key == "" {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.hotStore[key]; ok {
		delete(m.hotStore, key)
		return true
	}
	if _, ok := m.coldStore[key]; ok {
		delete(m.coldStore, key)
		return true
	}
	return false
}

func (m *HotColdManager) promoteToHot(entry *DataEntry) {
	delete(m.coldStore, entry.Key)
	entry.Tier = TierHot
	entry.ConsecutiveColdCycles = 0
	m.hotStore[entry.Key] = entry
}

func (m *HotColdManager) demoteToCold(entry *DataEntry) {
	delete(m.hotStore, entry.Key)
	entry.Tier = TierCold
	m.coldStore[entry.Key] = entry
}

func (m *HotColdManager) CheckAndMigrate() int {
	if m == nil {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	migratedCount := 0

	for _, entry := range m.hotStore {
		entry.Score = m.calculateScore(entry, now)

		if entry.Score < m.cfg.ColdThreshold {
			entry.ConsecutiveColdCycles++
			if entry.ConsecutiveColdCycles >= m.cfg.ColdCheckCycles {
				m.demoteToCold(entry)
				migratedCount++
			}
		} else {
			entry.ConsecutiveColdCycles = 0
		}
	}

	for _, entry := range m.coldStore {
		entry.Score = m.calculateScore(entry, now)
		if entry.Score >= m.cfg.HotThreshold {
			m.promoteToHot(entry)
			migratedCount++
		}
	}

	if m.cfg.AutoAdjustThresholds {
		m.autoAdjustThresholds(now)
	}

	return migratedCount
}

func (m *HotColdManager) autoAdjustThresholds(now time.Time) {
	if now.Sub(m.lastAdjustTime) < m.cfg.AdjustInterval {
		return
	}

	totalCount := len(m.hotStore) + len(m.coldStore)
	if totalCount == 0 {
		return
	}

	hotRatio := float64(len(m.hotStore)) / float64(totalCount)
	targetRatio := m.cfg.HotCapacityRatio

	adjustFactor := 1.0
	if hotRatio > targetRatio*1.1 {
		adjustFactor = 1.1
	} else if hotRatio < targetRatio*0.9 {
		adjustFactor = 0.9
	}

	accessesInEpoch := m.totalAccesses - m.accessesLastEpoch
	m.accessesLastEpoch = m.totalAccesses

	intervalSec := m.cfg.AdjustInterval.Seconds()
	if intervalSec <= 0 {
		intervalSec = 1.0
	}
	accessRate := float64(accessesInEpoch) / intervalSec

	expectedBaseRate := float64(totalCount) * 0.5
	loadFactor := 1.0
	if expectedBaseRate > 0 {
		loadFactor = accessRate / expectedBaseRate
	}

	if loadFactor > 2.0 {
		adjustFactor *= 0.9
	} else if loadFactor < 0.5 {
		adjustFactor *= 1.1
	}

	if adjustFactor == 1.0 {
		m.lastAdjustTime = now
		return
	}

	newHotThreshold := m.cfg.HotThreshold * adjustFactor
	newColdThreshold := m.cfg.ColdThreshold * adjustFactor

	if newHotThreshold > m.cfg.MaxHotThreshold {
		newHotThreshold = m.cfg.MaxHotThreshold
	}
	if newHotThreshold < m.cfg.MinHotThreshold {
		newHotThreshold = m.cfg.MinHotThreshold
	}
	if newColdThreshold > m.cfg.MaxColdThreshold {
		newColdThreshold = m.cfg.MaxColdThreshold
	}
	if newColdThreshold < m.cfg.MinColdThreshold {
		newColdThreshold = m.cfg.MinColdThreshold
	}

	if newHotThreshold <= newColdThreshold {
		newHotThreshold = newColdThreshold * 2
		if newHotThreshold > m.cfg.MaxHotThreshold {
			newHotThreshold = m.cfg.MaxHotThreshold
			newColdThreshold = newHotThreshold / 2
		}
	}

	m.cfg.HotThreshold = newHotThreshold
	m.cfg.ColdThreshold = newColdThreshold
	m.lastAdjustTime = now
}

func (m *HotColdManager) HotCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hotStore)
}

func (m *HotColdManager) ColdCount() int {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.coldStore)
}

func (m *HotColdManager) TotalCount() int {
	return m.HotCount() + m.ColdCount()
}

func (m *HotColdManager) GetConfig() Config {
	if m == nil {
		return Config{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

func (m *HotColdManager) GetEntry(key string) (*DataEntry, bool, error) {
	if m == nil {
		return nil, false, ErrNilManager
	}
	if key == "" {
		return nil, false, ErrEmptyKey
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()

	if entry, ok := m.hotStore[key]; ok {
		cp := copyEntry(entry)
		cp.Score = m.calculateScore(entry, now)
		return cp, true, nil
	}
	if entry, ok := m.coldStore[key]; ok {
		cp := copyEntry(entry)
		cp.Score = m.calculateScore(entry, now)
		return cp, true, nil
	}
	return nil, false, ErrKeyNotFound
}

func copyEntry(entry *DataEntry) *DataEntry {
	return &DataEntry{
		Key:                   entry.Key,
		Value:                 entry.Value,
		AccessCount:           entry.AccessCount,
		LastAccessTime:        entry.LastAccessTime,
		CreatedAt:             entry.CreatedAt,
		Tier:                  entry.Tier,
		ConsecutiveColdCycles: entry.ConsecutiveColdCycles,
		Score:                 entry.Score,
	}
}

func (m *HotColdManager) GetAllHotKeys() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.hotStore))
	for k := range m.hotStore {
		keys = append(keys, k)
	}
	return keys
}

func (m *HotColdManager) GetAllColdKeys() []string {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]string, 0, len(m.coldStore))
	for k := range m.coldStore {
		keys = append(keys, k)
	}
	return keys
}
