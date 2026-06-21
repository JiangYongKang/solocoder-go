package totpauth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidSecret     = errors.New("totpauth: invalid secret")
	ErrInvalidCode       = errors.New("totpauth: invalid code")
	ErrInvalidConfig     = errors.New("totpauth: invalid config")
	ErrCodeUsed          = errors.New("totpauth: recovery code already used")
	ErrNoRecoveryCodes   = errors.New("totpauth: no recovery codes available")
	ErrRecoveryCodeEmpty = errors.New("totpauth: recovery code cannot be empty")
	ErrRecoveryNotFound  = errors.New("totpauth: recovery code not found")
	ErrInvalidDigits     = errors.New("totpauth: digits must be between 6 and 8")
	ErrInvalidPeriod     = errors.New("totpauth: period must be positive")
	ErrInvalidSecretSize = errors.New("totpauth: secret size must be positive")
)

type Algorithm int

const (
	SHA1 Algorithm = iota
	SHA256
	SHA512
)

const (
	DefaultDigits         = 6
	DefaultPeriod         = 30
	DefaultDriftWindows   = 1
	DefaultSecretSize     = 20
	DefaultRecoveryCount  = 10
	DefaultRecoveryLength = 16
)

type Config struct {
	Digits       int
	Period       int
	DriftWindows int
	Algorithm    Algorithm
	SecretSize   int
}

func DefaultConfig() Config {
	return Config{
		Digits:       DefaultDigits,
		Period:       DefaultPeriod,
		DriftWindows: DefaultDriftWindows,
		Algorithm:    SHA1,
		SecretSize:   DefaultSecretSize,
	}
}

type TOTP struct {
	cfg Config
}

func NewTOTP() (*TOTP, error) {
	return NewTOTPWithConfig(DefaultConfig())
}

func NewTOTPWithConfig(cfg Config) (*TOTP, error) {
	if cfg.Digits < 6 || cfg.Digits > 8 {
		return nil, ErrInvalidDigits
	}
	if cfg.Period <= 0 {
		return nil, ErrInvalidPeriod
	}
	if cfg.DriftWindows < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.SecretSize <= 0 {
		return nil, ErrInvalidSecretSize
	}

	return &TOTP{
		cfg: cfg,
	}, nil
}

func (t *TOTP) Config() Config {
	return t.cfg
}

func (t *TOTP) GenerateSecret() (string, error) {
	secret := make([]byte, t.cfg.SecretSize)
	_, err := rand.Read(secret)
	if err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret), nil
}

func (t *TOTP) GenerateCode(secretBase32 string) (string, error) {
	return t.GenerateCodeAt(secretBase32, time.Now())
}

func (t *TOTP) GenerateCodeAt(secretBase32 string, tm time.Time) (string, error) {
	secret, err := decodeSecret(secretBase32)
	if err != nil {
		return "", err
	}

	counter := t.timeToCounter(tm)
	return t.generateHotp(secret, counter)
}

func (t *TOTP) ValidateCode(secretBase32, code string) (bool, error) {
	return t.ValidateCodeAt(secretBase32, code, time.Now())
}

func (t *TOTP) ValidateCodeAt(secretBase32, code string, tm time.Time) (bool, error) {
	if code == "" {
		return false, ErrInvalidCode
	}

	secret, err := decodeSecret(secretBase32)
	if err != nil {
		return false, err
	}

	currentCounter := t.timeToCounter(tm)

	for offset := -t.cfg.DriftWindows; offset <= t.cfg.DriftWindows; offset++ {
		counter := currentCounter + int64(offset)
		generated, err := t.generateHotp(secret, counter)
		if err != nil {
			return false, err
		}
		if hmac.Equal([]byte(generated), []byte(code)) {
			return true, nil
		}
	}

	return false, nil
}

func (t *TOTP) timeToCounter(tm time.Time) int64 {
	return tm.Unix() / int64(t.cfg.Period)
}

func (t *TOTP) generateHotp(secret []byte, counter int64) (string, error) {
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, uint64(counter))

	var h func() hash.Hash
	switch t.cfg.Algorithm {
	case SHA1:
		h = sha1.New
	case SHA256:
		h = sha256.New
	case SHA512:
		h = sha512.New
	default:
		return "", ErrInvalidConfig
	}

	mac := hmac.New(h, secret)
	mac.Write(counterBytes)
	hmacResult := mac.Sum(nil)

	offset := hmacResult[len(hmacResult)-1] & 0x0F
	binaryCode := (uint32(hmacResult[offset])&0x7F)<<24 |
		(uint32(hmacResult[offset+1])&0xFF)<<16 |
		(uint32(hmacResult[offset+2])&0xFF)<<8 |
		(uint32(hmacResult[offset+3]) & 0xFF)

	mod := uint32(1)
	for i := 0; i < t.cfg.Digits; i++ {
		mod *= 10
	}
	otp := binaryCode % mod

	return fmt.Sprintf("%0*d", t.cfg.Digits, otp), nil
}

func decodeSecret(secretBase32 string) ([]byte, error) {
	if secretBase32 == "" {
		return nil, ErrInvalidSecret
	}

	secretBase32 = strings.TrimSpace(secretBase32)
	secretBase32 = strings.ToUpper(secretBase32)
	secretBase32 = strings.TrimRight(secretBase32, "=")

	padding := (8 - len(secretBase32)%8) % 8
	if padding > 0 {
		secretBase32 += strings.Repeat("=", padding)
	}

	secret, err := base32.StdEncoding.DecodeString(secretBase32)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSecret, err)
	}

	if len(secret) == 0 {
		return nil, ErrInvalidSecret
	}

	return secret, nil
}

type RecoveryCode struct {
	Code  string
	Used  bool
	UsedAt time.Time
}

type RecoveryCodeStore struct {
	mu     sync.RWMutex
	codes  map[string]*RecoveryCode
	order  []string
}

func NewRecoveryCodeStore() *RecoveryCodeStore {
	return &RecoveryCodeStore{
		codes: make(map[string]*RecoveryCode),
		order: make([]string, 0),
	}
}

func (s *RecoveryCodeStore) Generate(count int) ([]string, error) {
	if count <= 0 {
		count = DefaultRecoveryCount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := generateRecoveryCode(DefaultRecoveryLength)
		if err != nil {
			return nil, err
		}
		codes[i] = code
		s.codes[code] = &RecoveryCode{
			Code: code,
			Used: false,
		}
		s.order = append(s.order, code)
	}

	return codes, nil
}

func generateRecoveryCode(length int) (string, error) {
	if length <= 0 {
		length = DefaultRecoveryLength
	}

	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	result := make([]byte, length)
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	for i := 0; i < length; i++ {
		result[i] = charset[int(b[i])%len(charset)]
	}

	return string(result), nil
}

func (s *RecoveryCodeStore) Validate(code string) (bool, error) {
	if code == "" {
		return false, ErrRecoveryCodeEmpty
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rc, exists := s.codes[code]
	if !exists {
		return false, ErrRecoveryNotFound
	}

	if rc.Used {
		return false, ErrCodeUsed
	}

	rc.Used = true
	rc.UsedAt = time.Now()

	remaining := s.remainingUnlocked()
	if remaining == 0 {
		return true, ErrNoRecoveryCodes
	}

	return true, nil
}

func (s *RecoveryCodeStore) IsUsed(code string) (bool, error) {
	if code == "" {
		return false, ErrRecoveryCodeEmpty
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rc, exists := s.codes[code]
	if !exists {
		return false, ErrRecoveryNotFound
	}

	return rc.Used, nil
}

func (s *RecoveryCodeStore) Remaining() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.remainingUnlocked()
}

func (s *RecoveryCodeStore) remainingUnlocked() int {
	count := 0
	for _, rc := range s.codes {
		if !rc.Used {
			count++
		}
	}
	return count
}

func (s *RecoveryCodeStore) Total() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.codes)
}

func (s *RecoveryCodeStore) List() []RecoveryCode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]RecoveryCode, 0, len(s.order))
	for _, code := range s.order {
		if rc, exists := s.codes[code]; exists {
			result = append(result, *rc)
		}
	}
	return result
}

func (s *RecoveryCodeStore) AllUsed() bool {
	return s.Remaining() == 0
}

func (s *RecoveryCodeStore) Regenerate(count int) ([]string, error) {
	if count <= 0 {
		count = DefaultRecoveryCount
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.codes = make(map[string]*RecoveryCode)
	s.order = make([]string, 0)

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, err := generateRecoveryCode(DefaultRecoveryLength)
		if err != nil {
			return nil, err
		}
		codes[i] = code
		s.codes[code] = &RecoveryCode{
			Code: code,
			Used: false,
		}
		s.order = append(s.order, code)
	}

	return codes, nil
}
