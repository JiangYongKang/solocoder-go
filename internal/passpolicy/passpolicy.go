package passpolicy

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

const (
	DefaultMinLength         = 8
	DefaultBcryptCost        = bcrypt.DefaultCost
	DefaultHistoryDepth      = 5
	DefaultExpiryDays        = 90
	DefaultWarningDays       = 7
	MinBcryptCost            = bcrypt.MinCost
	MaxBcryptCost            = bcrypt.MaxCost
)

var (
	ErrEmptyUserID               = errors.New("passpolicy: user id cannot be empty")
	ErrEmptyPassword             = errors.New("passpolicy: password cannot be empty")
	ErrUserNotFound              = errors.New("passpolicy: user not found")
	ErrPasswordExpired           = errors.New("passpolicy: password has expired")
	ErrMustChangePassword        = errors.New("passpolicy: password must be changed")
	ErrPasswordHistoryReused     = errors.New("passpolicy: password cannot reuse recent passwords")
	ErrInvalidBcryptCost         = errors.New("passpolicy: invalid bcrypt cost")
	ErrInvalidMinLength          = errors.New("passpolicy: invalid min length, must be >= 1")
	ErrInvalidHistoryDepth       = errors.New("passpolicy: invalid history depth, must be >= 0")
	ErrInvalidExpiryDays         = errors.New("passpolicy: invalid expiry days, must be >= 0")
	ErrInvalidWarningDays        = errors.New("passpolicy: invalid warning days, must be >= 0")
	ErrPasswordMismatch          = errors.New("passpolicy: password mismatch")
	ErrPasswordTooShort          = errors.New("passpolicy: password too short")
	ErrPasswordMissingUppercase  = errors.New("passpolicy: password must contain uppercase letter")
	ErrPasswordMissingLowercase  = errors.New("passpolicy: password must contain lowercase letter")
	ErrPasswordMissingDigit      = errors.New("passpolicy: password must contain digit")
	ErrPasswordMissingSpecial    = errors.New("passpolicy: password must contain special character")
)

type PolicyViolation struct {
	Err     error
	Message string
}

type ValidationResult struct {
	Valid      bool
	Violations []PolicyViolation
}

type ComplexityConfig struct {
	RequireUppercase bool
	RequireLowercase bool
	RequireDigit     bool
	RequireSpecial   bool
}

type Config struct {
	MinLength    int
	BcryptCost   int
	HistoryDepth int
	ExpiryDays   int
	WarningDays  int
	Complexity   ComplexityConfig
}

type PasswordRecord struct {
	Hash      []byte
	Cost      int
	CreatedAt time.Time
}

type HistoryEntry struct {
	Hash      []byte
	Cost      int
	CreatedAt time.Time
}

type UserState struct {
	UserID       string
	Current      *PasswordRecord
	History      []HistoryEntry
	LastChanged  time.Time
	MustChange   bool
}

type PasswordStatus struct {
	UserID            string
	IsExpired         bool
	DaysRemaining     int
	DaysSinceChanged  int
	IsWarningPeriod   bool
	MustChange        bool
	CreatedAt         time.Time
}

type VerifyResult struct {
	Valid         bool
	Rehashed      bool
	NewHash       []byte
	NewCost       int
	PasswordState *PasswordStatus
}

type Engine struct {
	mu     sync.RWMutex
	config Config
	users  map[string]*UserState
	now    func() time.Time
}

func DefaultConfig() Config {
	return Config{
		MinLength:    DefaultMinLength,
		BcryptCost:   DefaultBcryptCost,
		HistoryDepth: DefaultHistoryDepth,
		ExpiryDays:   DefaultExpiryDays,
		WarningDays:  DefaultWarningDays,
		Complexity: ComplexityConfig{
			RequireUppercase: true,
			RequireLowercase: true,
			RequireDigit:     true,
			RequireSpecial:   true,
		},
	}
}

func NewEngine() (*Engine, error) {
	return NewEngineWithConfig(DefaultConfig())
}

func NewEngineWithConfig(cfg Config) (*Engine, error) {
	if cfg.MinLength < 1 {
		return nil, ErrInvalidMinLength
	}
	if cfg.BcryptCost < MinBcryptCost || cfg.BcryptCost > MaxBcryptCost {
		return nil, ErrInvalidBcryptCost
	}
	if cfg.HistoryDepth < 0 {
		return nil, ErrInvalidHistoryDepth
	}
	if cfg.ExpiryDays < 0 {
		return nil, ErrInvalidExpiryDays
	}
	if cfg.WarningDays < 0 {
		return nil, ErrInvalidWarningDays
	}

	return &Engine{
		config: cfg,
		users:  make(map[string]*UserState),
		now:    time.Now,
	}, nil
}

func (e *Engine) SetNowFunc(f func() time.Time) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.now = f
}

func (e *Engine) GetConfig() Config {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

func (e *Engine) UpdateBcryptCost(cost int) error {
	if cost < MinBcryptCost || cost > MaxBcryptCost {
		return ErrInvalidBcryptCost
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.config.BcryptCost = cost
	return nil
}

func (e *Engine) ValidatePassword(password string) ValidationResult {
	var violations []PolicyViolation

	e.mu.RLock()
	minLength := e.config.MinLength
	requireUpper := e.config.Complexity.RequireUppercase
	requireLower := e.config.Complexity.RequireLowercase
	requireDigit := e.config.Complexity.RequireDigit
	requireSpecial := e.config.Complexity.RequireSpecial
	e.mu.RUnlock()

	if len(password) < minLength {
		violations = append(violations, PolicyViolation{
			Err:     ErrPasswordTooShort,
			Message: fmt.Sprintf("password must be at least %d characters long (got %d)", minLength, len(password)),
		})
	}

	if requireUpper {
		if !containsUpper(password) {
			violations = append(violations, PolicyViolation{
				Err:     ErrPasswordMissingUppercase,
				Message: "password must contain at least one uppercase letter",
			})
		}
	}

	if requireLower {
		if !containsLower(password) {
			violations = append(violations, PolicyViolation{
				Err:     ErrPasswordMissingLowercase,
				Message: "password must contain at least one lowercase letter",
			})
		}
	}

	if requireDigit {
		if !containsDigit(password) {
			violations = append(violations, PolicyViolation{
				Err:     ErrPasswordMissingDigit,
				Message: "password must contain at least one digit",
			})
		}
	}

	if requireSpecial {
		if !containsSpecial(password) {
			violations = append(violations, PolicyViolation{
				Err:     ErrPasswordMissingSpecial,
				Message: "password must contain at least one special character",
			})
		}
	}

	return ValidationResult{
		Valid:      len(violations) == 0,
		Violations: violations,
	}
}

func (e *Engine) SetPassword(userID, password string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if password == "" {
		return ErrEmptyPassword
	}

	vr := e.ValidatePassword(password)
	if !vr.Valid {
		if len(vr.Violations) > 0 {
			return vr.Violations[0].Err
		}
		return errors.New("passpolicy: password validation failed")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	historyDepth := e.config.HistoryDepth
	if historyDepth > 0 {
		state, exists := e.users[userID]
		if exists {
			if state.Current != nil {
				if bcrypt.CompareHashAndPassword(state.Current.Hash, []byte(password)) == nil {
					return ErrPasswordHistoryReused
				}
			}
			startIdx := len(state.History) - historyDepth
			if startIdx < 0 {
				startIdx = 0
			}
			for i := startIdx; i < len(state.History); i++ {
				if bcrypt.CompareHashAndPassword(state.History[i].Hash, []byte(password)) == nil {
					return ErrPasswordHistoryReused
				}
			}
		}
	}

	now := e.now()
	cost := e.config.BcryptCost
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return fmt.Errorf("passpolicy: hash password: %w", err)
	}

	state, exists := e.users[userID]
	if !exists {
		state = &UserState{
			UserID:  userID,
			History: make([]HistoryEntry, 0),
		}
		e.users[userID] = state
	} else {
		if state.Current != nil {
			state.History = append(state.History, HistoryEntry{
				Hash:      state.Current.Hash,
				Cost:      state.Current.Cost,
				CreatedAt: state.Current.CreatedAt,
			})
			if len(state.History) > e.config.HistoryDepth {
				state.History = state.History[len(state.History)-e.config.HistoryDepth:]
			}
		}
	}

	state.Current = &PasswordRecord{
		Hash:      hash,
		Cost:      cost,
		CreatedAt: now,
	}
	state.LastChanged = now
	state.MustChange = false

	return nil
}

func (e *Engine) ChangePassword(userID, oldPassword, newPassword string) error {
	if userID == "" {
		return ErrEmptyUserID
	}
	if newPassword == "" {
		return ErrEmptyPassword
	}

	e.mu.RLock()
	state, exists := e.users[userID]
	if !exists {
		e.mu.RUnlock()
		return ErrUserNotFound
	}
	if state.Current == nil {
		e.mu.RUnlock()
		return ErrUserNotFound
	}

	err := bcrypt.CompareHashAndPassword(state.Current.Hash, []byte(oldPassword))
	if err != nil {
		e.mu.RUnlock()
		return ErrPasswordMismatch
	}

	historyDepth := e.config.HistoryDepth
	if historyDepth > 0 {
		if bcrypt.CompareHashAndPassword(state.Current.Hash, []byte(newPassword)) == nil {
			e.mu.RUnlock()
			return ErrPasswordHistoryReused
		}
		startIdx := len(state.History) - historyDepth
		if startIdx < 0 {
			startIdx = 0
		}
		for i := startIdx; i < len(state.History); i++ {
			if bcrypt.CompareHashAndPassword(state.History[i].Hash, []byte(newPassword)) == nil {
				e.mu.RUnlock()
				return ErrPasswordHistoryReused
			}
		}
	}
	e.mu.RUnlock()

	vr := e.ValidatePassword(newPassword)
	if !vr.Valid {
		if len(vr.Violations) > 0 {
			return vr.Violations[0].Err
		}
		return errors.New("passpolicy: password validation failed")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	state, exists = e.users[userID]
	if !exists || state.Current == nil {
		return ErrUserNotFound
	}

	historyDepth = e.config.HistoryDepth
	if historyDepth > 0 {
		if bcrypt.CompareHashAndPassword(state.Current.Hash, []byte(newPassword)) == nil {
			return ErrPasswordHistoryReused
		}
		startIdx := len(state.History) - historyDepth
		if startIdx < 0 {
			startIdx = 0
		}
		for i := startIdx; i < len(state.History); i++ {
			if bcrypt.CompareHashAndPassword(state.History[i].Hash, []byte(newPassword)) == nil {
				return ErrPasswordHistoryReused
			}
		}
	}

	now := e.now()
	cost := e.config.BcryptCost
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), cost)
	if err != nil {
		return fmt.Errorf("passpolicy: hash password: %w", err)
	}

	state.History = append(state.History, HistoryEntry{
		Hash:      state.Current.Hash,
		Cost:      state.Current.Cost,
		CreatedAt: state.Current.CreatedAt,
	})
	if len(state.History) > e.config.HistoryDepth {
		state.History = state.History[len(state.History)-e.config.HistoryDepth:]
	}

	state.Current = &PasswordRecord{
		Hash:      newHash,
		Cost:      cost,
		CreatedAt: now,
	}
	state.LastChanged = now
	state.MustChange = false

	return nil
}

func (e *Engine) VerifyPassword(userID, password string) (VerifyResult, error) {
	result := VerifyResult{Valid: false}

	if userID == "" {
		return result, ErrEmptyUserID
	}
	if password == "" {
		return result, ErrEmptyPassword
	}

	e.mu.RLock()
	state, exists := e.users[userID]
	if !exists || state.Current == nil {
		e.mu.RUnlock()
		return result, ErrUserNotFound
	}

	current := state.Current
	currentCopy := *current
	mustChange := state.MustChange
	expiryDays := e.config.ExpiryDays
	warningDays := e.config.WarningDays
	currentCost := e.config.BcryptCost
	now := e.now()
	e.mu.RUnlock()

	err := bcrypt.CompareHashAndPassword(currentCopy.Hash, []byte(password))
	if err != nil {
		return result, ErrPasswordMismatch
	}

	daysSinceChanged := int(now.Sub(currentCopy.CreatedAt).Hours() / 24)
	daysRemaining := expiryDays - daysSinceChanged
	isExpired := expiryDays > 0 && daysRemaining <= 0
	isWarningPeriod := expiryDays > 0 && !isExpired && daysRemaining <= warningDays

	result.PasswordState = &PasswordStatus{
		UserID:           userID,
		IsExpired:        isExpired,
		DaysRemaining:    daysRemaining,
		DaysSinceChanged: daysSinceChanged,
		IsWarningPeriod:  isWarningPeriod,
		MustChange:       mustChange,
		CreatedAt:        currentCopy.CreatedAt,
	}

	if isExpired {
		result.Valid = false
		return result, ErrPasswordExpired
	}
	if mustChange {
		result.Valid = false
		return result, ErrMustChangePassword
	}

	result.Valid = true

	storedCost := currentCopy.Cost
	if storedCost < currentCost {
		newHash, hashErr := bcrypt.GenerateFromPassword([]byte(password), currentCost)
		if hashErr == nil {
			e.mu.Lock()
			if st, ok := e.users[userID]; ok && st.Current != nil {
				st.Current.Hash = newHash
				st.Current.Cost = currentCost
			}
			e.mu.Unlock()

			result.Rehashed = true
			result.NewHash = newHash
			result.NewCost = currentCost
		}
	}

	return result, nil
}

func (e *Engine) GetPasswordStatus(userID string) (*PasswordStatus, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.users[userID]
	if !exists || state.Current == nil {
		return nil, ErrUserNotFound
	}

	now := e.now()
	daysSinceChanged := int(now.Sub(state.Current.CreatedAt).Hours() / 24)
	daysRemaining := e.config.ExpiryDays - daysSinceChanged
	isExpired := e.config.ExpiryDays > 0 && daysRemaining <= 0
	isWarningPeriod := e.config.ExpiryDays > 0 && !isExpired && daysRemaining <= e.config.WarningDays

	return &PasswordStatus{
		UserID:           userID,
		IsExpired:        isExpired,
		DaysRemaining:    daysRemaining,
		DaysSinceChanged: daysSinceChanged,
		IsWarningPeriod:  isWarningPeriod,
		MustChange:       state.MustChange,
		CreatedAt:        state.Current.CreatedAt,
	}, nil
}

func (e *Engine) ForcePasswordChange(userID string) error {
	if userID == "" {
		return ErrEmptyUserID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	state, exists := e.users[userID]
	if !exists {
		return ErrUserNotFound
	}

	state.MustChange = true
	return nil
}

func (e *Engine) DeleteUser(userID string) error {
	if userID == "" {
		return ErrEmptyUserID
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.users[userID]; !exists {
		return ErrUserNotFound
	}

	delete(e.users, userID)
	return nil
}

func (e *Engine) UserCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.users)
}

func (e *Engine) HistoryDepth() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config.HistoryDepth
}

func (e *Engine) GetPasswordHash(userID string) ([]byte, int, error) {
	if userID == "" {
		return nil, 0, ErrEmptyUserID
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.users[userID]
	if !exists || state.Current == nil {
		return nil, 0, ErrUserNotFound
	}

	hashCopy := make([]byte, len(state.Current.Hash))
	copy(hashCopy, state.Current.Hash)
	return hashCopy, state.Current.Cost, nil
}

func (e *Engine) GetHistoryEntries(userID string) ([]HistoryEntry, error) {
	if userID == "" {
		return nil, ErrEmptyUserID
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	state, exists := e.users[userID]
	if !exists {
		return nil, ErrUserNotFound
	}

	result := make([]HistoryEntry, len(state.History))
	for i, entry := range state.History {
		hashCopy := make([]byte, len(entry.Hash))
		copy(hashCopy, entry.Hash)
		result[i] = HistoryEntry{
			Hash:      hashCopy,
			Cost:      entry.Cost,
			CreatedAt: entry.CreatedAt,
		}
	}
	return result, nil
}

func containsUpper(s string) bool {
	for _, r := range s {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func containsLower(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func containsDigit(s string) bool {
	for _, r := range s {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func containsSpecial(s string) bool {
	for _, r := range s {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) || strings.ContainsRune("!@#$%^&*()-_=+[]{};:,.<>?/~`|\\", r) {
			return true
		}
	}
	return false
}

func (vr ValidationResult) ErrorMessages() []string {
	msgs := make([]string, len(vr.Violations))
	for i, v := range vr.Violations {
		msgs[i] = v.Message
	}
	return msgs
}

func (vr ValidationResult) CombinedError() error {
	if vr.Valid {
		return nil
	}
	var errs []error
	for _, v := range vr.Violations {
		errs = append(errs, v.Err)
	}
	if len(errs) == 1 {
		return errs[0]
	}
	return errors.New(strings.Join(vr.ErrorMessages(), "; "))
}
