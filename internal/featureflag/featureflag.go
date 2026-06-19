package featureflag

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

var (
	ErrFlagNotFound          = errors.New("feature flag not found")
	ErrFlagAlreadyExists     = errors.New("feature flag already exists")
	ErrInvalidFlagType       = errors.New("invalid feature flag type")
	ErrInvalidPercentage     = errors.New("percentage must be between 0 and 100")
	ErrNilUserID             = errors.New("user ID cannot be empty for percentage/whitelist evaluation")
	ErrInvalidConfig         = errors.New("invalid feature flag configuration")
	ErrNilConfig             = errors.New("feature flag configuration cannot be nil")
	ErrNilFlagKey            = errors.New("feature flag key cannot be empty")
	ErrPercentageTypeNoValue = errors.New("percentage type flag must have percentage value")
)

type FlagType int

const (
	FlagTypeBoolean FlagType = iota
	FlagTypePercentage
	FlagTypeWhitelist
)

func (t FlagType) String() string {
	switch t {
	case FlagTypeBoolean:
		return "Boolean"
	case FlagTypePercentage:
		return "Percentage"
	case FlagTypeWhitelist:
		return "Whitelist"
	default:
		return "Unknown"
	}
}

type FlagConfig struct {
	Key         string
	Type        FlagType
	Enabled     bool
	Percentage  int
	Whitelist   []string
	Description string
}

func (c *FlagConfig) Clone() *FlagConfig {
	if c == nil {
		return nil
	}
	clone := &FlagConfig{
		Key:         c.Key,
		Type:        c.Type,
		Enabled:     c.Enabled,
		Percentage:  c.Percentage,
		Description: c.Description,
	}
	if c.Whitelist != nil {
		clone.Whitelist = make([]string, len(c.Whitelist))
		copy(clone.Whitelist, c.Whitelist)
	}
	return clone
}

func (c *FlagConfig) Marshal() string {
	data, _ := json.Marshal(c)
	return string(data)
}

func (c *FlagConfig) Validate() error {
	if c.Key == "" {
		return ErrNilFlagKey
	}
	switch c.Type {
	case FlagTypeBoolean:
		return nil
	case FlagTypePercentage:
		if c.Percentage < 0 || c.Percentage > 100 {
			return ErrInvalidPercentage
		}
		return nil
	case FlagTypeWhitelist:
		return nil
	default:
		return ErrInvalidFlagType
	}
}

type AuditLogEntry struct {
	Timestamp   time.Time
	FlagKey     string
	Before      *FlagConfig
	After       *FlagConfig
	Operation   string
}

type AuditLogQuery struct {
	FlagKey   string
	StartTime *time.Time
	EndTime   *time.Time
}

type hashSeed uint64

const defaultHashSeed hashSeed = 0x9E3779B97F4A7C15

type Evaluator struct {
	mu      sync.RWMutex
	flags   map[string]*FlagConfig
	audit   []*AuditLogEntry
	seed    hashSeed
}

func NewEvaluator() *Evaluator {
	return &Evaluator{
		flags: make(map[string]*FlagConfig),
		audit: make([]*AuditLogEntry, 0),
		seed:  defaultHashSeed,
	}
}

func NewEvaluatorWithSeed(seed uint64) *Evaluator {
	return &Evaluator{
		flags: make(map[string]*FlagConfig),
		audit: make([]*AuditLogEntry, 0),
		seed:  hashSeed(seed),
	}
}

func (e *Evaluator) CreateFlag(cfg *FlagConfig) error {
	if cfg == nil {
		return ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.flags[cfg.Key]; exists {
		return ErrFlagAlreadyExists
	}

	cloned := cfg.Clone()
	e.flags[cfg.Key] = cloned

	e.appendAuditLogLocked(cfg.Key, nil, cloned, "CREATE")
	return nil
}

func (e *Evaluator) UpdateFlag(cfg *FlagConfig) error {
	if cfg == nil {
		return ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	oldCfg, exists := e.flags[cfg.Key]
	if !exists {
		return ErrFlagNotFound
	}

	oldClone := oldCfg.Clone()
	newClone := cfg.Clone()
	e.flags[cfg.Key] = newClone

	e.appendAuditLogLocked(cfg.Key, oldClone, newClone, "UPDATE")
	return nil
}

func (e *Evaluator) DeleteFlag(key string) error {
	if key == "" {
		return ErrNilFlagKey
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	oldCfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}

	oldClone := oldCfg.Clone()
	delete(e.flags, key)

	e.appendAuditLogLocked(key, oldClone, nil, "DELETE")
	return nil
}

func (e *Evaluator) GetFlag(key string) (*FlagConfig, error) {
	if key == "" {
		return nil, ErrNilFlagKey
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	cfg, exists := e.flags[key]
	if !exists {
		return nil, ErrFlagNotFound
	}
	return cfg.Clone(), nil
}

func (e *Evaluator) ListFlags() []*FlagConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*FlagConfig, 0, len(e.flags))
	for _, cfg := range e.flags {
		result = append(result, cfg.Clone())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

func (e *Evaluator) Evaluate(key string, userID string) (bool, error) {
	if key == "" {
		return false, ErrNilFlagKey
	}

	e.mu.RLock()
	cfg, exists := e.flags[key]
	e.mu.RUnlock()

	if !exists {
		return false, ErrFlagNotFound
	}

	switch cfg.Type {
	case FlagTypeBoolean:
		return cfg.Enabled, nil

	case FlagTypePercentage:
		if userID == "" {
			return false, ErrNilUserID
		}
		if cfg.Percentage <= 0 {
			return false, nil
		}
		if cfg.Percentage >= 100 {
			return true, nil
		}
		bucket := computeUserBucket(userID, uint64(e.seed))
		return bucket < cfg.Percentage, nil

	case FlagTypeWhitelist:
		if userID == "" {
			return false, ErrNilUserID
		}
		return isInWhitelist(cfg.Whitelist, userID), nil

	default:
		return false, ErrInvalidFlagType
	}
}

func (e *Evaluator) SetBooleanValue(key string, enabled bool) error {
	if key == "" {
		return ErrNilFlagKey
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}

	oldClone := cfg.Clone()
	newCfg := cfg.Clone()
	newCfg.Enabled = enabled
	e.flags[key] = newCfg

	e.appendAuditLogLocked(key, oldClone, newCfg, "SET_BOOLEAN")
	return nil
}

func (e *Evaluator) SetPercentage(key string, percentage int) error {
	if key == "" {
		return ErrNilFlagKey
	}
	if percentage < 0 || percentage > 100 {
		return ErrInvalidPercentage
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}
	if cfg.Type != FlagTypePercentage {
		return fmt.Errorf("%w: flag type is %s, not Percentage", ErrInvalidFlagType, cfg.Type.String())
	}

	oldClone := cfg.Clone()
	newCfg := cfg.Clone()
	newCfg.Percentage = percentage
	e.flags[key] = newCfg

	e.appendAuditLogLocked(key, oldClone, newCfg, "SET_PERCENTAGE")
	return nil
}

func (e *Evaluator) AddToWhitelist(key string, userID string) error {
	if key == "" {
		return ErrNilFlagKey
	}
	if userID == "" {
		return ErrNilUserID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}
	if cfg.Type != FlagTypeWhitelist {
		return fmt.Errorf("%w: flag type is %s, not Whitelist", ErrInvalidFlagType, cfg.Type.String())
	}

	if isInWhitelist(cfg.Whitelist, userID) {
		return nil
	}

	oldClone := cfg.Clone()
	newCfg := cfg.Clone()
	newCfg.Whitelist = append(newCfg.Whitelist, userID)
	e.flags[key] = newCfg

	e.appendAuditLogLocked(key, oldClone, newCfg, "ADD_WHITELIST")
	return nil
}

func (e *Evaluator) RemoveFromWhitelist(key string, userID string) error {
	if key == "" {
		return ErrNilFlagKey
	}
	if userID == "" {
		return ErrNilUserID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}
	if cfg.Type != FlagTypeWhitelist {
		return fmt.Errorf("%w: flag type is %s, not Whitelist", ErrInvalidFlagType, cfg.Type.String())
	}

	if !isInWhitelist(cfg.Whitelist, userID) {
		return nil
	}

	oldClone := cfg.Clone()
	newCfg := cfg.Clone()
	newWhitelist := make([]string, 0, len(cfg.Whitelist)-1)
	for _, uid := range cfg.Whitelist {
		if uid != userID {
			newWhitelist = append(newWhitelist, uid)
		}
	}
	newCfg.Whitelist = newWhitelist
	e.flags[key] = newCfg

	e.appendAuditLogLocked(key, oldClone, newCfg, "REMOVE_WHITELIST")
	return nil
}

func (e *Evaluator) ChangeFlagType(key string, newType FlagType, enabled bool, percentage int, whitelist []string) error {
	if key == "" {
		return ErrNilFlagKey
	}

	tempCfg := &FlagConfig{
		Key:        key,
		Type:       newType,
		Enabled:    enabled,
		Percentage: percentage,
		Whitelist:  whitelist,
	}
	if err := tempCfg.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, exists := e.flags[key]
	if !exists {
		return ErrFlagNotFound
	}

	oldClone := cfg.Clone()
	newCfg := tempCfg.Clone()
	e.flags[key] = newCfg

	e.appendAuditLogLocked(key, oldClone, newCfg, "CHANGE_TYPE")
	return nil
}

func (e *Evaluator) QueryAuditLogs(query AuditLogQuery) []*AuditLogEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*AuditLogEntry, 0)

	for _, entry := range e.audit {
		if query.FlagKey != "" && entry.FlagKey != query.FlagKey {
			continue
		}
		if query.StartTime != nil && entry.Timestamp.Before(*query.StartTime) {
			continue
		}
		if query.EndTime != nil && entry.Timestamp.After(*query.EndTime) {
			continue
		}

		result = append(result, cloneAuditLogEntry(entry))
	}

	return result
}

func (e *Evaluator) AuditLogCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.audit)
}

func (e *Evaluator) appendAuditLogLocked(key string, before, after *FlagConfig, operation string) {
	entry := &AuditLogEntry{
		Timestamp: time.Now(),
		FlagKey:   key,
		Before:    before,
		After:     after,
		Operation: operation,
	}
	e.audit = append(e.audit, entry)
}

func computeUserBucket(userID string, seed uint64) int {
	h := sha256.New()
	seedBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(seedBytes, seed)
	h.Write(seedBytes)
	h.Write([]byte(userID))
	hash := h.Sum(nil)

	value := binary.BigEndian.Uint64(hash[:8])
	return int(value % 100)
}

func isInWhitelist(whitelist []string, userID string) bool {
	for _, uid := range whitelist {
		if uid == userID {
			return true
		}
	}
	return false
}

func cloneAuditLogEntry(entry *AuditLogEntry) *AuditLogEntry {
	if entry == nil {
		return nil
	}
	return &AuditLogEntry{
		Timestamp: entry.Timestamp,
		FlagKey:   entry.FlagKey,
		Before:    entry.Before.Clone(),
		After:     entry.After.Clone(),
		Operation: entry.Operation,
	}
}
