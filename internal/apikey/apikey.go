package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	KeyIDLength  = 16
	SecretLength = 32
	PrefixLength = 8
	SecretPrefix = "sk_"
)

var (
	ErrKeyNotFound            = errors.New("apikey: key not found")
	ErrKeyExists              = errors.New("apikey: key already exists")
	ErrEmptyKeyID             = errors.New("apikey: key id cannot be empty")
	ErrEmptyPrefix            = errors.New("apikey: prefix cannot be empty")
	ErrEmptySecret            = errors.New("apikey: secret cannot be empty")
	ErrEmptyResource          = errors.New("apikey: resource cannot be empty")
	ErrEmptyAction            = errors.New("apikey: action cannot be empty")
	ErrEmptyRevokeReason      = errors.New("apikey: revoke reason cannot be empty")
	ErrKeyRevoked             = errors.New("apikey: key has been revoked")
	ErrKeyExpired             = errors.New("apikey: key has expired")
	ErrUsageLimitExceeded     = errors.New("apikey: usage limit exceeded")
	ErrInvalidPermission      = errors.New("apikey: invalid permission format")
	ErrPermissionDenied       = errors.New("apikey: permission denied")
	ErrInvalidSecret          = errors.New("apikey: invalid secret")
	ErrMaxUsesZeroOrNegative  = errors.New("apikey: max uses must be positive")
	ErrExpiresAtInThePast     = errors.New("apikey: expires at cannot be in the past")
	ErrNegativeTTL            = errors.New("apikey: ttl cannot be negative")
	ErrAlreadyRevoked         = errors.New("apikey: key already revoked")
)

type Permission struct {
	Resource string
	Action   string
}

func (p Permission) String() string {
	return fmt.Sprintf("%s:%s", p.Resource, p.Action)
}

func ParsePermission(s string) (Permission, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Permission{}, ErrInvalidPermission
	}
	return Permission{Resource: parts[0], Action: parts[1]}, nil
}

type KeyStatus string

const (
	StatusActive   KeyStatus = "active"
	StatusExpired  KeyStatus = "expired"
	StatusRevoked  KeyStatus = "revoked"
	StatusDepleted KeyStatus = "depleted"
)

type keyState struct {
	expiresAt     time.Time
	hasExpiration bool
	revokedAt     time.Time
	revokeReason  string
	permissions   map[Permission]bool
	lastUsedAt    time.Time
}

type APIKey struct {
	ID           string
	Prefix       string
	SecretHash   string
	Name         string
	Description  string
	MaxUses      int64
	UsedCount    atomic.Int64
	CreatedAt    time.Time
	revoked      atomic.Bool
	hasExp       atomic.Bool
	stateMu      sync.Mutex
	state        keyState
}

func (k *APIKey) loadState() keyState {
	k.stateMu.Lock()
	s := k.state
	k.stateMu.Unlock()
	return s
}

func (k *APIKey) Status() KeyStatus {
	if k.revoked.Load() {
		return StatusRevoked
	}
	if k.hasExp.Load() {
		s := k.loadState()
		if time.Now().After(s.expiresAt) {
			return StatusExpired
		}
	}
	if k.MaxUses > 0 && k.UsedCount.Load() >= k.MaxUses {
		return StatusDepleted
	}
	return StatusActive
}

func (k *APIKey) RemainingUses() int64 {
	if k.MaxUses <= 0 {
		return -1
	}
	used := k.UsedCount.Load()
	remaining := k.MaxUses - used
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (k *APIKey) RemainingTime() (time.Duration, bool) {
	if !k.hasExp.Load() {
		return 0, false
	}
	s := k.loadState()
	remaining := time.Until(s.expiresAt)
	return remaining, true
}

func (k *APIKey) HasPermission(p Permission) bool {
	s := k.loadState()
	if s.permissions == nil {
		return false
	}
	return s.permissions[p]
}

func (k *APIKey) PermissionsList() []Permission {
	s := k.loadState()
	perms := make([]Permission, 0, len(s.permissions))
	for p := range s.permissions {
		perms = append(perms, p)
	}
	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Resource != perms[j].Resource {
			return perms[i].Resource < perms[j].Resource
		}
		return perms[i].Action < perms[j].Action
	})
	return perms
}

type APIKeyMeta struct {
	ID            string
	Prefix        string
	Name          string
	Description   string
	Status        KeyStatus
	Permissions   []Permission
	MaxUses       int64
	UsedCount     int64
	RemainingUses int64
	CreatedAt     time.Time
	ExpiresAt     time.Time
	HasExpiration bool
	Revoked       bool
	RevokedAt     time.Time
	RevokeReason  string
	LastUsedAt    time.Time
}

type CreatedKey struct {
	ID     string
	Prefix string
	Secret string
}

type CreateKeyOptions struct {
	Name          string
	Description   string
	Permissions   []Permission
	MaxUses       int64
	ExpiresAt     time.Time
	TTL           time.Duration
	HasExpiration bool
}

type VerifyResult struct {
	Valid   bool
	KeyMeta *APIKeyMeta
	Reason  error
}

type CheckAccessResult struct {
	Allowed bool
	Reason  error
}

type Manager struct {
	mu       sync.RWMutex
	keys     map[string]*APIKey
	byPrefix map[string][]string
}

func NewManager() *Manager {
	return &Manager{
		keys:     make(map[string]*APIKey),
		byPrefix: make(map[string][]string),
	}
}

func (m *Manager) generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (m *Manager) generateKeyID() (string, error) {
	for i := 0; i < 100; i++ {
		b, err := m.generateRandomBytes(KeyIDLength)
		if err != nil {
			return "", err
		}
		id := hex.EncodeToString(b)
		m.mu.RLock()
		_, exists := m.keys[id]
		m.mu.RUnlock()
		if !exists {
			return id, nil
		}
	}
	return "", errors.New("apikey: failed to generate unique key id after 100 attempts")
}

func (m *Manager) generateSecret() (string, string, string, error) {
	b, err := m.generateRandomBytes(SecretLength)
	if err != nil {
		return "", "", "", err
	}
	secret := SecretPrefix + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(secret))
	secretHash := hex.EncodeToString(hash[:])
	prefix := secret[:len(SecretPrefix)+PrefixLength]
	return secret, secretHash, prefix, nil
}

func hashSecret(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(hash[:])
}

func (m *Manager) CreateKey(opts CreateKeyOptions) (*CreatedKey, error) {
	now := time.Now()

	id, err := m.generateKeyID()
	if err != nil {
		return nil, err
	}

	secret, secretHash, prefix, err := m.generateSecret()
	if err != nil {
		return nil, err
	}

	if opts.MaxUses < 0 {
		return nil, ErrMaxUsesZeroOrNegative
	}
	if opts.TTL < 0 {
		return nil, ErrNegativeTTL
	}

	var expiresAt time.Time
	hasExpiration := opts.HasExpiration

	if opts.TTL > 0 {
		expiresAt = now.Add(opts.TTL)
		hasExpiration = true
	} else if !opts.ExpiresAt.IsZero() {
		if opts.ExpiresAt.Before(now) {
			return nil, ErrExpiresAtInThePast
		}
		expiresAt = opts.ExpiresAt
		hasExpiration = true
	}

	permMap := make(map[Permission]bool)
	for _, p := range opts.Permissions {
		if p.Resource == "" {
			return nil, ErrEmptyResource
		}
		if p.Action == "" {
			return nil, ErrEmptyAction
		}
		permMap[p] = true
	}

	key := &APIKey{
		ID:           id,
		Prefix:       prefix,
		SecretHash:   secretHash,
		Name:         opts.Name,
		Description:  opts.Description,
		MaxUses:      opts.MaxUses,
		CreatedAt:    now,
	}
	key.state.permissions = permMap
	key.state.expiresAt = expiresAt
	key.state.hasExpiration = hasExpiration
	if hasExpiration {
		key.hasExp.Store(true)
	}

	m.mu.Lock()
	m.keys[id] = key
	m.byPrefix[prefix] = append(m.byPrefix[prefix], id)
	m.mu.Unlock()

	return &CreatedKey{
		ID:     id,
		Prefix: prefix,
		Secret: secret,
	}, nil
}

func (m *Manager) GetKeyMeta(id string) (*APIKeyMeta, error) {
	if id == "" {
		return nil, ErrEmptyKeyID
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	return keyToMeta(key), nil
}

func (m *Manager) ListKeysByPrefix(prefix string) ([]*APIKeyMeta, error) {
	if prefix == "" {
		return nil, ErrEmptyPrefix
	}

	m.mu.RLock()
	ids, exists := m.byPrefix[prefix]
	m.mu.RUnlock()

	if !exists {
		return []*APIKeyMeta{}, nil
	}

	metas := make([]*APIKeyMeta, 0, len(ids))
	for _, id := range ids {
		m.mu.RLock()
		key, ok := m.keys[id]
		m.mu.RUnlock()
		if ok {
			metas = append(metas, keyToMeta(key))
		}
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})

	return metas, nil
}

func keyToMeta(key *APIKey) *APIKeyMeta {
	s := key.loadState()

	perms := make([]Permission, 0, len(s.permissions))
	for p := range s.permissions {
		perms = append(perms, p)
	}
	sort.Slice(perms, func(i, j int) bool {
		if perms[i].Resource != perms[j].Resource {
			return perms[i].Resource < perms[j].Resource
		}
		return perms[i].Action < perms[j].Action
	})

	remaining := int64(-1)
	if key.MaxUses > 0 {
		used := key.UsedCount.Load()
		remaining = key.MaxUses - used
		if remaining < 0 {
			remaining = 0
		}
	}

	return &APIKeyMeta{
		ID:            key.ID,
		Prefix:        key.Prefix,
		Name:          key.Name,
		Description:   key.Description,
		Status:        key.Status(),
		Permissions:   perms,
		MaxUses:       key.MaxUses,
		UsedCount:     key.UsedCount.Load(),
		RemainingUses: remaining,
		CreatedAt:     key.CreatedAt,
		ExpiresAt:     s.expiresAt,
		HasExpiration: s.hasExpiration,
		Revoked:       key.revoked.Load(),
		RevokedAt:     s.revokedAt,
		RevokeReason:  s.revokeReason,
		LastUsedAt:    s.lastUsedAt,
	}
}

func (m *Manager) ListAllKeys() []*APIKeyMeta {
	m.mu.RLock()
	ids := make([]string, 0, len(m.keys))
	for id := range m.keys {
		ids = append(ids, id)
	}
	m.mu.RUnlock()

	metas := make([]*APIKeyMeta, 0, len(ids))
	for _, id := range ids {
		if meta, err := m.GetKeyMeta(id); err == nil {
			metas = append(metas, meta)
		}
	}

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})

	return metas
}

func (m *Manager) RevokeKey(id, reason string) error {
	if id == "" {
		return ErrEmptyKeyID
	}
	if reason == "" {
		return ErrEmptyRevokeReason
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return ErrKeyNotFound
	}

	if !key.revoked.CompareAndSwap(false, true) {
		return ErrAlreadyRevoked
	}

	key.stateMu.Lock()
	key.state.revokedAt = time.Now()
	key.state.revokeReason = reason
	key.stateMu.Unlock()

	return nil
}

func (m *Manager) VerifyKey(secret string) *VerifyResult {
	if secret == "" {
		return &VerifyResult{Valid: false, Reason: ErrEmptySecret}
	}

	secretHash := hashSecret(secret)

	prefix := ""
	if len(secret) >= len(SecretPrefix)+PrefixLength {
		prefix = secret[:len(SecretPrefix)+PrefixLength]
	}

	if prefix == "" {
		return &VerifyResult{Valid: false, Reason: ErrInvalidSecret}
	}

	var matchedKey *APIKey

	m.mu.RLock()
	ids, exists := m.byPrefix[prefix]
	if exists {
		for _, id := range ids {
			key, ok := m.keys[id]
			if ok && key.SecretHash == secretHash {
				matchedKey = key
				break
			}
		}
	}
	m.mu.RUnlock()

	if matchedKey == nil {
		return &VerifyResult{Valid: false, Reason: ErrInvalidSecret}
	}

	if matchedKey.revoked.Load() {
		return &VerifyResult{Valid: false, KeyMeta: keyToMeta(matchedKey), Reason: ErrKeyRevoked}
	}

	if matchedKey.hasExp.Load() {
		s := matchedKey.loadState()
		if time.Now().After(s.expiresAt) {
			return &VerifyResult{Valid: false, KeyMeta: keyToMeta(matchedKey), Reason: ErrKeyExpired}
		}
	}

	if matchedKey.MaxUses > 0 {
		for {
			used := matchedKey.UsedCount.Load()
			if used >= matchedKey.MaxUses {
				return &VerifyResult{Valid: false, KeyMeta: keyToMeta(matchedKey), Reason: ErrUsageLimitExceeded}
			}
			if matchedKey.UsedCount.CompareAndSwap(used, used+1) {
				break
			}
		}
	} else {
		matchedKey.UsedCount.Add(1)
	}

	now := time.Now()
	matchedKey.stateMu.Lock()
	matchedKey.state.lastUsedAt = now
	matchedKey.stateMu.Unlock()

	return &VerifyResult{Valid: true, KeyMeta: keyToMeta(matchedKey), Reason: nil}
}

func (m *Manager) CheckAccess(id string, p Permission) CheckAccessResult {
	if id == "" {
		return CheckAccessResult{Allowed: false, Reason: ErrEmptyKeyID}
	}
	if p.Resource == "" {
		return CheckAccessResult{Allowed: false, Reason: ErrEmptyResource}
	}
	if p.Action == "" {
		return CheckAccessResult{Allowed: false, Reason: ErrEmptyAction}
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return CheckAccessResult{Allowed: false, Reason: ErrKeyNotFound}
	}

	if key.revoked.Load() {
		return CheckAccessResult{Allowed: false, Reason: ErrKeyRevoked}
	}

	if key.hasExp.Load() {
		s := key.loadState()
		if time.Now().After(s.expiresAt) {
			return CheckAccessResult{Allowed: false, Reason: ErrKeyExpired}
		}
	}

	if key.MaxUses > 0 && key.UsedCount.Load() >= key.MaxUses {
		return CheckAccessResult{Allowed: false, Reason: ErrUsageLimitExceeded}
	}

	if !key.HasPermission(p) {
		return CheckAccessResult{
			Allowed: false,
			Reason:  fmt.Errorf("%w: %s", ErrPermissionDenied, p.String()),
		}
	}

	return CheckAccessResult{Allowed: true, Reason: nil}
}

func (m *Manager) VerifyAndCheckAccess(secret string, p Permission) (*APIKeyMeta, CheckAccessResult) {
	verifyResult := m.VerifyKey(secret)
	if !verifyResult.Valid {
		return verifyResult.KeyMeta, CheckAccessResult{Allowed: false, Reason: verifyResult.Reason}
	}

	keyID := verifyResult.KeyMeta.ID
	accessResult := m.CheckAccess(keyID, p)

	return verifyResult.KeyMeta, accessResult
}

func (m *Manager) IncrementUsage(id string) (int64, error) {
	if id == "" {
		return 0, ErrEmptyKeyID
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return 0, ErrKeyNotFound
	}

	if key.revoked.Load() {
		return 0, ErrKeyRevoked
	}

	if key.hasExp.Load() {
		s := key.loadState()
		if time.Now().After(s.expiresAt) {
			return 0, ErrKeyExpired
		}
	}

	if key.MaxUses > 0 {
		for {
			used := key.UsedCount.Load()
			if used >= key.MaxUses {
				return 0, ErrUsageLimitExceeded
			}
			if key.UsedCount.CompareAndSwap(used, used+1) {
				now := time.Now()
				key.stateMu.Lock()
				key.state.lastUsedAt = now
				key.stateMu.Unlock()
				return used + 1, nil
			}
		}
	}

	newVal := key.UsedCount.Add(1)
	now := time.Now()
	key.stateMu.Lock()
	key.state.lastUsedAt = now
	key.stateMu.Unlock()
	return newVal, nil
}

func (m *Manager) GetRemainingUses(id string) (int64, error) {
	if id == "" {
		return 0, ErrEmptyKeyID
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return 0, ErrKeyNotFound
	}

	return key.RemainingUses(), nil
}

func (m *Manager) GetRemainingTime(id string) (time.Duration, bool, error) {
	if id == "" {
		return 0, false, ErrEmptyKeyID
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return 0, false, ErrKeyNotFound
	}

	dur, has := key.RemainingTime()
	return dur, has, nil
}

func (m *Manager) SetExpiresAt(id string, expiresAt time.Time) error {
	if id == "" {
		return ErrEmptyKeyID
	}
	if expiresAt.Before(time.Now()) {
		return ErrExpiresAtInThePast
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return ErrKeyNotFound
	}

	if key.revoked.Load() {
		return ErrKeyRevoked
	}

	key.stateMu.Lock()
	key.state.expiresAt = expiresAt
	key.state.hasExpiration = true
	key.stateMu.Unlock()
	key.hasExp.Store(true)

	return nil
}

func (m *Manager) SetTTL(id string, ttl time.Duration) error {
	if id == "" {
		return ErrEmptyKeyID
	}
	if ttl < 0 {
		return ErrNegativeTTL
	}

	m.mu.RLock()
	key, exists := m.keys[id]
	m.mu.RUnlock()

	if !exists {
		return ErrKeyNotFound
	}

	if key.revoked.Load() {
		return ErrKeyRevoked
	}

	key.stateMu.Lock()
	if ttl == 0 {
		key.state.hasExpiration = false
		key.state.expiresAt = time.Time{}
	} else {
		key.state.expiresAt = time.Now().Add(ttl)
		key.state.hasExpiration = true
	}
	key.stateMu.Unlock()

	if ttl == 0 {
		key.hasExp.Store(false)
	} else {
		key.hasExp.Store(true)
	}

	return nil
}

func (m *Manager) KeyCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.keys)
}
