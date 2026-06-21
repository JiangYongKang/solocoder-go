package shortlink

import (
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

const (
	base62Chars       = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	defaultMaxRetries = 10
	defaultHashLen    = 8
	defaultRandomLen  = 8
	defaultRandomCharset = base62Chars
)

var (
	ErrShortCodeNotFound       = errors.New("shortlink: short code not found")
	ErrShortCodeExists         = errors.New("shortlink: short code already exists")
	ErrEmptyOriginalURL        = errors.New("shortlink: original URL cannot be empty")
	ErrEmptyShortCode          = errors.New("shortlink: short code cannot be empty")
	ErrInvalidCustomShortCode  = errors.New("shortlink: invalid custom short code format")
	ErrGenerateFailed          = errors.New("shortlink: failed to generate unique short code after max retries")
	ErrMaxRetriesZeroOrNegative = errors.New("shortlink: max retries must be positive")
	ErrHashLengthInvalid       = errors.New("shortlink: hash length is invalid for the selected algorithm")
	ErrRandomLengthInvalid     = errors.New("shortlink: random length must be positive")
	ErrInvalidCharset          = errors.New("shortlink: charset cannot be empty")
	ErrUnsupportedHashAlgo     = errors.New("shortlink: unsupported hash algorithm")
)

var validShortCodeRegex = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

type ShortCodeStrategy string

const (
	StrategyAutoIncrement ShortCodeStrategy = "auto_increment"
	StrategyHash          ShortCodeStrategy = "hash"
	StrategyRandom        ShortCodeStrategy = "random"
	StrategyCustom        ShortCodeStrategy = "custom"
)

type HashAlgorithm string

const (
	HashMD5    HashAlgorithm = "md5"
	HashSHA1   HashAlgorithm = "sha1"
	HashSHA256 HashAlgorithm = "sha256"
)

type ShortLink struct {
	ShortCode   string
	OriginalURL string
	VisitCount  atomic.Int64
	CreatedAt   time.Time
}

type ShortLinkMeta struct {
	ShortCode   string
	OriginalURL string
	VisitCount  int64
	CreatedAt   time.Time
}

type CreateOptions struct {
	OriginalURL string
	CustomCode  string
	Strategy    ShortCodeStrategy
}

type HashStrategyConfig struct {
	Algorithm  HashAlgorithm
	Length     int
	MaxRetries int
}

type RandomStrategyConfig struct {
	Length     int
	Charset    string
	MaxRetries int
}

type AutoIncrementConfig struct {
	StartID int64
}

type Config struct {
	HashConfig      HashStrategyConfig
	RandomConfig    RandomStrategyConfig
	AutoIncrement   AutoIncrementConfig
	DefaultStrategy ShortCodeStrategy
}

func DefaultConfig() Config {
	return Config{
		HashConfig: HashStrategyConfig{
			Algorithm:  HashMD5,
			Length:     defaultHashLen,
			MaxRetries: defaultMaxRetries,
		},
		RandomConfig: RandomStrategyConfig{
			Length:     defaultRandomLen,
			Charset:    defaultRandomCharset,
			MaxRetries: defaultMaxRetries,
		},
		AutoIncrement: AutoIncrementConfig{
			StartID: 1,
		},
		DefaultStrategy: StrategyAutoIncrement,
	}
}

type Manager struct {
	mu             sync.RWMutex
	links          map[string]*ShortLink
	autoIncrement  atomic.Int64
	cfg            Config
}

func NewManager() *Manager {
	m, _ := NewManagerWithConfig(DefaultConfig())
	return m
}

func NewManagerWithConfig(cfg Config) (*Manager, error) {
	if cfg.HashConfig.Algorithm == "" {
		cfg.HashConfig.Algorithm = HashMD5
	}
	maxLen := hashHexLength(cfg.HashConfig.Algorithm)
	if cfg.HashConfig.Length <= 0 || cfg.HashConfig.Length > maxLen {
		return nil, ErrHashLengthInvalid
	}
	if cfg.HashConfig.MaxRetries <= 0 {
		return nil, ErrMaxRetriesZeroOrNegative
	}
	if cfg.RandomConfig.Length <= 0 {
		return nil, ErrRandomLengthInvalid
	}
	if cfg.RandomConfig.Charset == "" {
		return nil, ErrInvalidCharset
	}
	if cfg.RandomConfig.MaxRetries <= 0 {
		return nil, ErrMaxRetriesZeroOrNegative
	}
	if cfg.AutoIncrement.StartID < 0 {
		cfg.AutoIncrement.StartID = 1
	}
	if cfg.DefaultStrategy == "" {
		cfg.DefaultStrategy = StrategyAutoIncrement
	}

	m := &Manager{
		links: make(map[string]*ShortLink),
		cfg:   cfg,
	}
	m.autoIncrement.Store(cfg.AutoIncrement.StartID - 1)
	return m, nil
}

func base62Encode(num int64) string {
	if num == 0 {
		return string(base62Chars[0])
	}
	if num < 0 {
		num = -num
	}
	var result []byte
	base := int64(len(base62Chars))
	for num > 0 {
		remainder := num % base
		result = append([]byte{base62Chars[remainder]}, result...)
		num = num / base
	}
	return string(result)
}

func base62Decode(s string) (int64, error) {
	base := int64(len(base62Chars))
	var result int64
	for i, c := range s {
		idx := int64(-1)
		for j, ch := range base62Chars {
			if ch == c {
				idx = int64(j)
				break
			}
		}
		if idx < 0 {
			return 0, fmt.Errorf("shortlink: invalid base62 character '%c' at position %d", c, i)
		}
		result = result*base + idx
	}
	return result, nil
}

func newHash(algo HashAlgorithm) (hash.Hash, error) {
	switch algo {
	case HashMD5:
		return md5.New(), nil
	case HashSHA1:
		return sha1.New(), nil
	case HashSHA256:
		return sha256.New(), nil
	default:
		return nil, ErrUnsupportedHashAlgo
	}
}

func hashHexLength(algo HashAlgorithm) int {
	switch algo {
	case HashMD5:
		return 32
	case HashSHA1:
		return 40
	case HashSHA256:
		return 64
	default:
		return 64
	}
}

func (m *Manager) generateWithAutoIncrement() (string, error) {
	id := m.autoIncrement.Add(1)
	return base62Encode(id), nil
}

func (m *Manager) generateWithHash(originalURL string, config HashStrategyConfig) (string, error) {
	h, err := newHash(config.Algorithm)
	if err != nil {
		return "", err
	}

	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		var input string
		if attempt == 0 {
			input = originalURL
		} else {
			input = originalURL + strconv.Itoa(attempt) + time.Now().String()
		}

		h.Reset()
		h.Write([]byte(input))
		hashBytes := h.Sum(nil)
		hashHex := hex.EncodeToString(hashBytes)

		targetLen := config.Length
		if targetLen <= 0 {
			targetLen = len(hashHex)
		}
		if targetLen > len(hashHex) {
			targetLen = len(hashHex)
		}

		var startPos int
		if targetLen < len(hashHex) {
			startPos = (attempt * 2) % (len(hashHex) - targetLen)
		}

		shortCode := hashHex[startPos : startPos+targetLen]

		m.mu.RLock()
		_, exists := m.links[shortCode]
		m.mu.RUnlock()

		if !exists {
			return shortCode, nil
		}
	}
	return "", ErrGenerateFailed
}

func (m *Manager) generateWithRandom(config RandomStrategyConfig) (string, error) {
	charsetLen := len(config.Charset)
	maxByte := 255 - (256 % charsetLen)
	buf := make([]byte, 1)

	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		shortCode := make([]byte, config.Length)
		idx := 0
		for idx < config.Length {
			_, err := rand.Read(buf)
			if err != nil {
				return "", err
			}
			b := buf[0]
			if int(b) > maxByte {
				continue
			}
			shortCode[idx] = config.Charset[int(b)%charsetLen]
			idx++
		}
		code := string(shortCode)

		m.mu.RLock()
		_, exists := m.links[code]
		m.mu.RUnlock()

		if !exists {
			return code, nil
		}
	}
	return "", ErrGenerateFailed
}

func isValidCustomShortCode(code string) bool {
	return validShortCodeRegex.MatchString(code)
}

func (m *Manager) Create(opts CreateOptions) (*ShortLinkMeta, error) {
	if opts.OriginalURL == "" {
		return nil, ErrEmptyOriginalURL
	}

	var shortCode string
	var err error

	strategy := opts.Strategy
	if strategy == "" {
		strategy = m.cfg.DefaultStrategy
	}

	if opts.CustomCode != "" {
		strategy = StrategyCustom
	}

	switch strategy {
	case StrategyCustom:
		if opts.CustomCode == "" {
			return nil, ErrEmptyShortCode
		}
		if !isValidCustomShortCode(opts.CustomCode) {
			return nil, ErrInvalidCustomShortCode
		}
		m.mu.RLock()
		_, exists := m.links[opts.CustomCode]
		m.mu.RUnlock()
		if exists {
			return nil, ErrShortCodeExists
		}
		shortCode = opts.CustomCode

	case StrategyAutoIncrement:
		shortCode, err = m.generateWithAutoIncrement()
		if err != nil {
			return nil, err
		}

	case StrategyHash:
		shortCode, err = m.generateWithHash(opts.OriginalURL, m.cfg.HashConfig)
		if err != nil {
			return nil, err
		}

	case StrategyRandom:
		shortCode, err = m.generateWithRandom(m.cfg.RandomConfig)
		if err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("shortlink: unknown strategy: %s", strategy)
	}

	link := &ShortLink{
		ShortCode:   shortCode,
		OriginalURL: opts.OriginalURL,
		CreatedAt:   time.Now(),
	}

	m.mu.Lock()
	if _, exists := m.links[shortCode]; exists {
		m.mu.Unlock()
		return nil, ErrShortCodeExists
	}
	m.links[shortCode] = link
	m.mu.Unlock()

	return linkToMeta(link), nil
}

func (m *Manager) GetOriginalURL(shortCode string) (string, error) {
	if shortCode == "" {
		return "", ErrEmptyShortCode
	}

	m.mu.RLock()
	link, exists := m.links[shortCode]
	m.mu.RUnlock()

	if !exists {
		return "", ErrShortCodeNotFound
	}

	link.VisitCount.Add(1)
	return link.OriginalURL, nil
}

func (m *Manager) GetMeta(shortCode string) (*ShortLinkMeta, error) {
	if shortCode == "" {
		return nil, ErrEmptyShortCode
	}

	m.mu.RLock()
	link, exists := m.links[shortCode]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrShortCodeNotFound
	}

	return linkToMeta(link), nil
}

func (m *Manager) GetVisitCount(shortCode string) (int64, error) {
	if shortCode == "" {
		return 0, ErrEmptyShortCode
	}

	m.mu.RLock()
	link, exists := m.links[shortCode]
	m.mu.RUnlock()

	if !exists {
		return 0, ErrShortCodeNotFound
	}

	return link.VisitCount.Load(), nil
}

func (m *Manager) GetTotalVisitCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total int64
	for _, link := range m.links {
		total += link.VisitCount.Load()
	}
	return total
}

func (m *Manager) ListAll() []*ShortLinkMeta {
	m.mu.RLock()
	metas := make([]*ShortLinkMeta, 0, len(m.links))
	for _, link := range m.links {
		metas = append(metas, linkToMeta(link))
	}
	m.mu.RUnlock()

	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas
}

func (m *Manager) Delete(shortCode string) error {
	if shortCode == "" {
		return ErrEmptyShortCode
	}

	m.mu.Lock()
	_, exists := m.links[shortCode]
	if !exists {
		m.mu.Unlock()
		return ErrShortCodeNotFound
	}
	delete(m.links, shortCode)
	m.mu.Unlock()
	return nil
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.links)
}

func linkToMeta(link *ShortLink) *ShortLinkMeta {
	return &ShortLinkMeta{
		ShortCode:   link.ShortCode,
		OriginalURL: link.OriginalURL,
		VisitCount:  link.VisitCount.Load(),
		CreatedAt:   link.CreatedAt,
	}
}
