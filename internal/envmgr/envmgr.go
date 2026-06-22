package envmgr

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrMissingRequired = errors.New("envmgr: missing required environment variables")
	ErrInvalidType     = errors.New("envmgr: invalid type conversion")
	ErrEmptyValue      = errors.New("envmgr: empty value")
	ErrKeyNotFound     = errors.New("envmgr: key not found")
	ErrInvalidKeySize  = errors.New("envmgr: invalid AES key size, must be 32 bytes")
	ErrDecryptFailed   = errors.New("envmgr: decryption failed")
)

const (
	aesKeySize    = 32
	nonceSize     = 12
	defaultPrefix = ""
)

type SensitiveValue struct {
	ciphertext []byte
	nonce      []byte
	key        []byte
	mu         sync.RWMutex
}

type EnvConfig struct {
	Key       string
	Required  bool
	Sensitive bool
	Default   string
}

type EnvGroup struct {
	prefix string
	values map[string]string
	config map[string]*EnvConfig
	mu     sync.RWMutex
}

type EnvManager struct {
	groups   map[string]*EnvGroup
	aesKey   []byte
	envSource func() []string
	mu       sync.RWMutex
}

func NewEnvManager() (*EnvManager, error) {
	key := make([]byte, aesKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("envmgr: generate AES key: %w", err)
	}

	return &EnvManager{
		groups:   make(map[string]*EnvGroup),
		aesKey:   key,
		envSource: os.Environ,
	}, nil
}

func NewEnvManagerWithKey(key []byte) (*EnvManager, error) {
	if len(key) != aesKeySize {
		return nil, ErrInvalidKeySize
	}

	keyCopy := make([]byte, aesKeySize)
	copy(keyCopy, key)

	return &EnvManager{
		groups:   make(map[string]*EnvGroup),
		aesKey:   keyCopy,
		envSource: os.Environ,
	}, nil
}

func (m *EnvManager) LoadGroup(prefix string, configs ...*EnvConfig) (*EnvGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	values := make(map[string]string)
	cfgMap := make(map[string]*EnvConfig)

	for _, cfg := range configs {
		if cfg != nil {
			cfgMap[cfg.Key] = cfg
		}
	}

	for _, env := range m.envSource() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		if prefix != defaultPrefix {
			if !strings.HasPrefix(key, prefix) {
				continue
			}
			key = strings.TrimPrefix(key, prefix)
		}

		values[key] = value
	}

	var missing []string
	for key, cfg := range cfgMap {
		if cfg.Required {
			val, exists := values[key]
			if !exists || strings.TrimSpace(val) == "" {
				if cfg.Default != "" {
					values[key] = cfg.Default
				} else {
					missing = append(missing, key)
				}
			}
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", ErrMissingRequired, strings.Join(missing, ", "))
	}

	group := &EnvGroup{
		prefix: prefix,
		values: values,
		config: cfgMap,
	}

	for key, val := range values {
		if cfg, ok := cfgMap[key]; ok && cfg.Sensitive {
			encrypted, err := m.encrypt([]byte(val))
			if err != nil {
				return nil, err
			}
			group.values[key] = encrypted
		}
	}

	m.groups[prefix] = group
	return group, nil
}

func (g *EnvGroup) Get(key string) (string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	val, exists := g.values[key]
	if !exists {
		if cfg, ok := g.config[key]; ok && cfg.Default != "" {
			return cfg.Default, nil
		}
		return "", fmt.Errorf("%w: %s", ErrKeyNotFound, key)
	}

	if cfg, ok := g.config[key]; ok && cfg.Sensitive {
		return "", fmt.Errorf("envmgr: cannot directly read sensitive value '%s', use GetSensitive", key)
	}

	return val, nil
}

func (m *EnvManager) GetSensitive(group *EnvGroup, key string) (*SensitiveValue, error) {
	group.mu.RLock()
	defer group.mu.RUnlock()

	cfg, exists := group.config[key]
	if !exists || !cfg.Sensitive {
		return nil, fmt.Errorf("envmgr: '%s' is not marked as sensitive", key)
	}

	val, exists := group.values[key]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, key)
	}

	sv, err := parseEncryptedValue(val)
	if err != nil {
		return nil, err
	}

	m.SetSensitiveKey(sv)

	return sv, nil
}

func (g *EnvGroup) GetString(key string) (string, error) {
	return g.Get(key)
}

func (g *EnvGroup) GetInt(key string) (int, error) {
	val, err := g.Get(key)
	if err != nil {
		return 0, err
	}

	n, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot convert '%s' to int: %v", ErrInvalidType, key, err)
	}

	return n, nil
}

func (g *EnvGroup) GetInt64(key string) (int64, error) {
	val, err := g.Get(key)
	if err != nil {
		return 0, err
	}

	n, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot convert '%s' to int64: %v", ErrInvalidType, key, err)
	}

	return n, nil
}

func (g *EnvGroup) GetFloat64(key string) (float64, error) {
	val, err := g.Get(key)
	if err != nil {
		return 0, err
	}

	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot convert '%s' to float64: %v", ErrInvalidType, key, err)
	}

	return f, nil
}

func (g *EnvGroup) GetBool(key string) (bool, error) {
	val, err := g.Get(key)
	if err != nil {
		return false, err
	}

	b, err := strconv.ParseBool(strings.ToLower(val))
	if err != nil {
		return false, fmt.Errorf("%w: cannot convert '%s' to bool: %v", ErrInvalidType, key, err)
	}

	return b, nil
}

func (g *EnvGroup) GetDuration(key string) (time.Duration, error) {
	val, err := g.Get(key)
	if err != nil {
		return 0, err
	}

	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("%w: cannot convert '%s' to duration: %v", ErrInvalidType, key, err)
	}

	return d, nil
}

func (g *EnvGroup) GetAll() map[string]string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make(map[string]string)
	for key, val := range g.values {
		if cfg, ok := g.config[key]; ok && cfg.Sensitive {
			result[key] = "[ENCRYPTED]"
		} else {
			result[key] = val
		}
	}

	return result
}

func (g *EnvGroup) Prefix() string {
	return g.prefix
}

func (sv *SensitiveValue) Decrypt() (string, error) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()

	block, err := aes.NewCipher(sv.key)
	if err != nil {
		return "", fmt.Errorf("%w: create cipher: %v", ErrDecryptFailed, err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("%w: create GCM: %v", ErrDecryptFailed, err)
	}

	plaintext, err := gcm.Open(nil, sv.nonce, sv.ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("%w: decrypt: %v", ErrDecryptFailed, err)
	}

	return string(plaintext), nil
}

func (m *EnvManager) encrypt(plaintext []byte) (string, error) {
	block, err := aes.NewCipher(m.aesKey)
	if err != nil {
		return "", fmt.Errorf("envmgr: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("envmgr: create GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("envmgr: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	combined := make([]byte, nonceSize+len(ciphertext))
	copy(combined[:nonceSize], nonce)
	copy(combined[nonceSize:], ciphertext)

	return base64.StdEncoding.EncodeToString(combined), nil
}

func parseEncryptedValue(encoded string) (*SensitiveValue, error) {
	combined, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: decode base64: %v", ErrDecryptFailed, err)
	}

	if len(combined) < nonceSize {
		return nil, fmt.Errorf("%w: invalid encrypted data", ErrDecryptFailed)
	}

	nonce := make([]byte, nonceSize)
	copy(nonce, combined[:nonceSize])

	ciphertext := make([]byte, len(combined)-nonceSize)
	copy(ciphertext, combined[nonceSize:])

	key := make([]byte, aesKeySize)

	return &SensitiveValue{
		ciphertext: ciphertext,
		nonce:      nonce,
		key:        key,
	}, nil
}

func (m *EnvManager) SetSensitiveKey(sv *SensitiveValue) {
	sv.mu.Lock()
	defer sv.mu.Unlock()

	copy(sv.key, m.aesKey)
}

func (m *EnvManager) GetGroup(prefix string) (*EnvGroup, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, ok := m.groups[prefix]
	return group, ok
}

func (g *EnvGroup) Exists(key string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()

	_, exists := g.values[key]
	return exists
}

func (m *EnvManager) setEnvSource(source func() []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.envSource = source
}
