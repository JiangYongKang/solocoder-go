package shortlink

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.Count() != 0 {
		t.Errorf("expected 0 links, got %d", m.Count())
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoIncrement = AutoIncrementConfig{StartID: 100}
	cfg.DefaultStrategy = StrategyAutoIncrement
	m, err := NewManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewManagerWithConfig returned error: %v", err)
	}
	if m == nil {
		t.Fatal("NewManagerWithConfig returned nil")
	}

	meta, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if meta.ShortCode == "" {
		t.Error("expected non-empty short code")
	}

	decoded, err := base62Decode(meta.ShortCode)
	if err != nil {
		t.Fatalf("base62Decode failed: %v", err)
	}
	if decoded != 100 {
		t.Errorf("expected first ID 100, got %d", decoded)
	}
}

func TestNewManagerWithConfigDefaults(t *testing.T) {
	tests := []struct {
		name      string
		cfg       Config
		expectErr error
	}{
		{
			name: "invalid hash length zero",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 0, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrHashLengthInvalid,
		},
		{
			name: "invalid hash length negative",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: -1, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrHashLengthInvalid,
		},
		{
			name: "invalid hash length exceeds md5 max",
			cfg: Config{
				HashConfig: HashStrategyConfig{Algorithm: HashMD5, Length: 33, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrHashLengthInvalid,
		},
		{
			name: "invalid hash length exceeds sha1 max",
			cfg: Config{
				HashConfig: HashStrategyConfig{Algorithm: HashSHA1, Length: 41, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrHashLengthInvalid,
		},
		{
			name: "invalid hash length too large",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 65, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrHashLengthInvalid,
		},
		{
			name: "invalid hash max retries zero",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 0},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrMaxRetriesZeroOrNegative,
		},
		{
			name: "invalid hash max retries negative",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: -1},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrMaxRetriesZeroOrNegative,
		},
		{
			name: "invalid random length zero",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 0, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrRandomLengthInvalid,
		},
		{
			name: "invalid random length negative",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: -1, Charset: "abc", MaxRetries: 10},
			},
			expectErr: ErrRandomLengthInvalid,
		},
		{
			name: "invalid charset empty",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "", MaxRetries: 10},
			},
			expectErr: ErrInvalidCharset,
		},
		{
			name: "invalid random max retries zero",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: 0},
			},
			expectErr: ErrMaxRetriesZeroOrNegative,
		},
		{
			name: "invalid random max retries negative",
			cfg: Config{
				HashConfig: HashStrategyConfig{Length: 8, MaxRetries: 10},
				RandomConfig: RandomStrategyConfig{Length: 8, Charset: "abc", MaxRetries: -5},
			},
			expectErr: ErrMaxRetriesZeroOrNegative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewManagerWithConfig(tt.cfg)
			if !errors.Is(err, tt.expectErr) {
				t.Errorf("expected error %v, got %v", tt.expectErr, err)
			}
		})
	}
}

func TestNewManagerWithConfigValidDefaults(t *testing.T) {
	cfg := Config{
		HashConfig: HashStrategyConfig{
			Length:     8,
			MaxRetries: 10,
		},
		RandomConfig: RandomStrategyConfig{
			Length:     8,
			Charset:    "0123456789",
			MaxRetries: 10,
		},
	}
	m, err := NewManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("NewManagerWithConfig returned nil")
	}

	_, err = m.Create(CreateOptions{
		Strategy:    StrategyRandom,
		OriginalURL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("Create with random strategy failed: %v", err)
	}
}

func TestBase62Encode(t *testing.T) {
	tests := []struct {
		name   int64
		expect string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "A"},
		{35, "Z"},
		{36, "a"},
		{61, "z"},
		{62, "10"},
		{63, "11"},
		{123, "1z"},
		{999, "G7"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("encode_%d", tt.name), func(t *testing.T) {
			result := base62Encode(tt.name)
			if result != tt.expect {
				t.Errorf("base62Encode(%d) = %q, want %q", tt.name, result, tt.expect)
			}

			decoded, err := base62Decode(result)
			if err != nil {
				t.Fatalf("base62Decode(%q) error: %v", result, err)
			}
			if decoded != tt.name {
				t.Errorf("base62Decode(%q) = %d, want %d", result, decoded, tt.name)
			}
		})
	}
}

func TestBase62EncodeNegative(t *testing.T) {
	result := base62Encode(-123)
	expected := base62Encode(123)
	if result != expected {
		t.Errorf("base62Encode(-123) = %q, want %q (absolute value)", result, expected)
	}
}

func TestBase62DecodeInvalid(t *testing.T) {
	_, err := base62Decode("abc!@#")
	if err == nil {
		t.Error("expected error for invalid base62 string")
	}
}

func TestCreateBasicAutoIncrement(t *testing.T) {
	m := NewManager()

	meta1, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/page1",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if meta1.ShortCode != "1" {
		t.Errorf("expected shortcode '1', got %q", meta1.ShortCode)
	}
	if meta1.OriginalURL != "https://example.com/page1" {
		t.Errorf("expected original URL to match")
	}
	if meta1.VisitCount != 0 {
		t.Errorf("expected 0 visits, got %d", meta1.VisitCount)
	}

	meta2, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/page2",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if meta2.ShortCode != "2" {
		t.Errorf("expected shortcode '2', got %q", meta2.ShortCode)
	}

	if m.Count() != 2 {
		t.Errorf("expected 2 links, got %d", m.Count())
	}
}

func TestCreateWithEmptyURL(t *testing.T) {
	m := NewManager()
	_, err := m.Create(CreateOptions{})
	if !errors.Is(err, ErrEmptyOriginalURL) {
		t.Errorf("expected ErrEmptyOriginalURL, got %v", err)
	}
}

func TestCreateEmptyShortCode(t *testing.T) {
	m := NewManager()
	_, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
		Strategy:  StrategyCustom,
	})
	if !errors.Is(err, ErrEmptyShortCode) {
		t.Errorf("expected ErrEmptyShortCode, got %v", err)
	}
}

func TestCreateCustomShortCode(t *testing.T) {
	m := NewManager()

	meta, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
		CustomCode: "my-custom-code",
	})
	if err != nil {
		t.Fatalf("Create with custom code failed: %v", err)
	}
	if meta.ShortCode != "my-custom-code" {
		t.Errorf("expected custom shortcode 'my-custom-code', got %q", meta.ShortCode)
	}
}

func TestCreateCustomShortCodeInvalid(t *testing.T) {
	m := NewManager()

	invalidCodes := []string{
		"has space",
		"has/slash",
		"has?query=1",
		"special!@#",
	}

	for _, code := range invalidCodes {
		t.Run(fmt.Sprintf("invalid_%q", code), func(t *testing.T) {
			_, err := m.Create(CreateOptions{
				OriginalURL: "https://example.com",
				CustomCode: code,
			})
			if !errors.Is(err, ErrInvalidCustomShortCode) && !errors.Is(err, ErrEmptyShortCode) {
				t.Errorf("expected error for invalid code %q, got %v", code, err)
			}
		})
	}
}

func TestCreateCustomShortCodeExists(t *testing.T) {
	m := NewManager()

	_, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/1",
		CustomCode: "custom1",
	})
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	_, err = m.Create(CreateOptions{
		OriginalURL: "https://example.com/2",
		CustomCode: "custom1",
	})
	if !errors.Is(err, ErrShortCodeExists) {
		t.Errorf("expected ErrShortCodeExists, got %v", err)
	}
}

func TestCreateCustomShortCodeValid(t *testing.T) {
	m := NewManager()

	validCodes := []string{
		"a",
		"abc123",
		"MY_CODE-123",
		"a-b_c",
		"ABCDEFGHIJKLMNOP",
	}

	for _, code := range validCodes {
		t.Run(fmt.Sprintf("valid_%q", code), func(t *testing.T) {
			meta, err := m.Create(CreateOptions{
				OriginalURL: "https://example.com/" + code,
				CustomCode: code,
			})
			if err != nil {
				t.Fatalf("create failed for valid code %q: %v", code, err)
			}
			if meta.ShortCode != code {
				t.Errorf("expected shortcode %q, got %q", code, meta.ShortCode)
			}
		})
	}
}

func TestCreateHashStrategy(t *testing.T) {
	m := NewManager()

	meta, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/page1",
		Strategy:  StrategyHash,
	})
	if err != nil {
		t.Fatalf("Create with hash strategy failed: %v", err)
	}
	if meta.ShortCode == "" {
		t.Error("expected non-empty short code")
	}

	expectedLen := 8
	if len(meta.ShortCode) != expectedLen {
		t.Errorf("expected shortcode length %d, got %d", expectedLen, len(meta.ShortCode))
	}

	meta2, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/page2",
		Strategy:  StrategyHash,
	})
	if err != nil {
		t.Fatalf("second create failed: %v", err)
	}
	if meta2.ShortCode == meta.ShortCode {
		t.Error("different URLs should produce different shortcodes")
	}
}

func TestCreateHashStrategyDifferentAlgorithms(t *testing.T) {
	algos := []HashAlgorithm{HashMD5, HashSHA1, HashSHA256}

	for _, algo := range algos {
		t.Run(string(algo), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.HashConfig.Algorithm = algo
			m, err := NewManagerWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewManagerWithConfig failed: %v", err)
			}

			meta, err := m.Create(CreateOptions{
				OriginalURL: "https://example.com/test",
				Strategy:  StrategyHash,
			})
			if err != nil {
				t.Fatalf("Create with %s failed: %v", algo, err)
			}
			if meta.ShortCode == "" {
				t.Error("expected non-empty short code")
			}
		})
	}
}

func TestCreateRandomStrategy(t *testing.T) {
	m := NewManager()

	codes := make(map[string]bool)
	for i := 0; i < 20; i++ {
		meta, err := m.Create(CreateOptions{
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
			Strategy:  StrategyRandom,
		})
		if err != nil {
			t.Fatalf("Create with random strategy failed: %v", err)
		}
		if len(meta.ShortCode) != 8 {
			t.Errorf("expected shortcode length 8, got %d", len(meta.ShortCode))
		}
		if codes[meta.ShortCode] {
			t.Errorf("duplicate shortcode generated: %s", meta.ShortCode)
		}
		codes[meta.ShortCode] = true
	}
}

func TestCreateRandomStrategyCustomCharset(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RandomConfig.Charset = "0123456789"
	cfg.RandomConfig.Length = 6
	m, err := NewManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewManagerWithConfig failed: %v", err)
	}

	for i := 0; i < 10; i++ {
		meta, err := m.Create(CreateOptions{
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
			Strategy:  StrategyRandom,
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if len(meta.ShortCode) != 6 {
			t.Errorf("expected length 6, got %d", len(meta.ShortCode))
		}
		for _, c := range meta.ShortCode {
			if c < '0' || c > '9' {
				t.Errorf("expected only digits, got character %c", c)
			}
		}
	}
}

func TestGetOriginalURL(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	url, err := m.GetOriginalURL(meta.ShortCode)
	if err != nil {
		t.Fatalf("GetOriginalURL failed: %v", err)
	}
	if url != "https://example.com" {
		t.Errorf("expected original URL, got %q", url)
	}
}

func TestGetOriginalURLNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetOriginalURL("nonexistent")
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Errorf("expected ErrShortCodeNotFound, got %v", err)
	}
}

func TestGetOriginalURLEmpty(t *testing.T) {
	m := NewManager()

	_, err := m.GetOriginalURL("")
	if !errors.Is(err, ErrEmptyShortCode) {
		t.Errorf("expected ErrEmptyShortCode, got %v", err)
	}
}

func TestVisitCountIncrement(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	for i := int64(1); i <= 5; i++ {
		_, err := m.GetOriginalURL(meta.ShortCode)
		if err != nil {
			t.Fatalf("GetOriginalURL failed: %v", err)
		}

		count, err := m.GetVisitCount(meta.ShortCode)
		if err != nil {
			t.Fatalf("GetVisitCount failed: %v", err)
		}
		if count != i {
			t.Errorf("expected count %d, got %d", i, count)
		}
	}
}

func TestGetVisitCount(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	count, err := m.GetVisitCount(meta.ShortCode)
	if err != nil {
		t.Fatalf("GetVisitCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestGetVisitCountNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetVisitCount("nonexistent")
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Errorf("expected ErrShortCodeNotFound, got %v", err)
	}
}

func TestGetVisitCountEmpty(t *testing.T) {
	m := NewManager()

	_, err := m.GetVisitCount("")
	if !errors.Is(err, ErrEmptyShortCode) {
		t.Errorf("expected ErrEmptyShortCode, got %v", err)
	}
}

func TestGetTotalVisitCount(t *testing.T) {
	m := NewManager()

	m.Create(CreateOptions{OriginalURL: "https://a.com"})
	m.Create(CreateOptions{OriginalURL: "https://b.com"})
	m.Create(CreateOptions{OriginalURL: "https://c.com"})

	all := m.ListAll()
	for i, link := range all {
		for j := 0; j <= i; j++ {
			m.GetOriginalURL(link.ShortCode)
		}
	}

	total := m.GetTotalVisitCount()
	expected := int64(1 + 2 + 3)
	if total != expected {
		t.Errorf("expected total %d, got %d", expected, total)
	}
}

func TestGetTotalVisitCountEmpty(t *testing.T) {
	m := NewManager()
	total := m.GetTotalVisitCount()
	if total != 0 {
		t.Errorf("expected 0, got %d", total)
	}
}

func TestGetMeta(t *testing.T) {
	m := NewManager()

	meta, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	fetched, err := m.GetMeta(meta.ShortCode)
	if err != nil {
		t.Fatalf("GetMeta failed: %v", err)
	}
	if fetched.ShortCode != meta.ShortCode {
		t.Errorf("expected %q, got %q", meta.ShortCode, fetched.ShortCode)
	}
	if fetched.OriginalURL != meta.OriginalURL {
		t.Errorf("original URL mismatch")
	}
}

func TestGetMetaNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetMeta("nonexistent")
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Errorf("expected ErrShortCodeNotFound, got %v", err)
	}
}

func TestGetMetaEmpty(t *testing.T) {
	m := NewManager()
	_, err := m.GetMeta("")
	if !errors.Is(err, ErrEmptyShortCode) {
		t.Errorf("expected ErrEmptyShortCode, got %v", err)
	}
}

func TestListAll(t *testing.T) {
	m := NewManager()

	for i := 0; i < 5; i++ {
		time.Sleep(1 * time.Millisecond)
		_, err := m.Create(CreateOptions{
			OriginalURL: fmt.Sprintf("https://example.com/%d", i),
		})
		if err != nil {
			t.Fatalf("Create failed: %v", err)
		}
	}

	all := m.ListAll()
	if len(all) != 5 {
		t.Errorf("expected 5 links, got %d", len(all))
	}

	for i := 1; i < len(all); i++ {
		if all[i-1].CreatedAt.Before(all[i].CreatedAt) {
			t.Error("expected sorted by CreatedAt descending")
		}
	}
}

func TestListAllEmpty(t *testing.T) {
	m := NewManager()
	all := m.ListAll()
	if len(all) != 0 {
		t.Errorf("expected 0 links, got %d", len(all))
	}
}

func TestDelete(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	err := m.Delete(meta.ShortCode)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if m.Count() != 0 {
		t.Errorf("expected 0 links after delete, got %d", m.Count())
	}

	_, err = m.GetOriginalURL(meta.ShortCode)
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Error("expected ErrShortCodeNotFound after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	m := NewManager()
	err := m.Delete("nonexistent")
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Errorf("expected ErrShortCodeNotFound, got %v", err)
	}
}

func TestDeleteEmpty(t *testing.T) {
	m := NewManager()
	err := m.Delete("")
	if !errors.Is(err, ErrEmptyShortCode) {
		t.Errorf("expected ErrEmptyShortCode, got %v", err)
	}
}

func TestCount(t *testing.T) {
	m := NewManager()
	if m.Count() != 0 {
		t.Errorf("expected 0, got %d", m.Count())
	}

	m.Create(CreateOptions{OriginalURL: "https://a.com"})
	if m.Count() != 1 {
		t.Errorf("expected 1, got %d", m.Count())
	}

	m.Create(CreateOptions{OriginalURL: "https://b.com"})
	m.Create(CreateOptions{OriginalURL: "https://c.com"})
	if m.Count() != 3 {
		t.Errorf("expected 3, got %d", m.Count())
	}
}

func TestUnknownStrategy(t *testing.T) {
	m := NewManager()
	_, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
		Strategy:    ShortCodeStrategy("unknown"),
	})
	if err == nil {
		t.Error("expected error for unknown strategy")
	}
}

func TestConcurrentCreate(t *testing.T) {
	m := NewManager()

	var wg sync.WaitGroup
	numLinks := 100
	var errCount int32

	for i := 0; i < numLinks; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := m.Create(CreateOptions{
				OriginalURL: fmt.Sprintf("https://example.com/%d", i),
			})
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if errCount != 0 {
		t.Errorf("expected 0 errors, got %d", errCount)
	}

	if m.Count() != numLinks {
		t.Errorf("expected %d links, got %d", numLinks, m.Count())
	}
}

func TestConcurrentVisitCount(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	var wg sync.WaitGroup
	numVisits := 500

	for i := 0; i < numVisits; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.GetOriginalURL(meta.ShortCode)
		}()
	}

	wg.Wait()

	count, _ := m.GetVisitCount(meta.ShortCode)
	if count != int64(numVisits) {
		t.Errorf("expected %d visits, got %d", numVisits, count)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	m := NewManager()

	meta, _ := m.Create(CreateOptions{
		OriginalURL: "https://example.com",
	})

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.GetOriginalURL(meta.ShortCode)
				}
			}
		}()
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.GetMeta(meta.ShortCode)
				}
			}
		}()
	}

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.Create(CreateOptions{
						OriginalURL: fmt.Sprintf("https://example.com/extra/%d", i),
						Strategy: StrategyRandom,
					})
				}
			}
		}(i)
	}

	time.Sleep(100 * time.Millisecond)
	close(stop)
	wg.Wait()
}

func TestAutoIncrementStartID(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoIncrement = AutoIncrementConfig{StartID: 1000}
	m, err := NewManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewManagerWithConfig failed: %v", err)
	}

	meta, err := m.Create(CreateOptions{OriginalURL: "https://example.com"})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	decoded, _ := base62Decode(meta.ShortCode)
	if decoded != 1000 {
		t.Errorf("expected ID 1000, got %d", decoded)
	}
}

func TestIsValidCustomShortCode(t *testing.T) {
	tests := []struct {
		code   string
		expect bool
	}{
		{"a", true},
		{"abc123", true},
		{"A-B_C", true},
		{"0123456789", true},
		{"ABCDEFGHIJKLMNOPQRSTUVWXYZ", true},
		{"abcdefghijklmnopqrstuvwxyz", true},
		{"", false},
		{"has space", false},
		{"has/slash", false},
		{"has.dot", false},
		{"special!chars", false},
		{"toolongstring123456789012345678901", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.code), func(t *testing.T) {
			result := isValidCustomShortCode(tt.code)
			if result != tt.expect {
				t.Errorf("isValidCustomShortCode(%q) = %v, want %v", tt.code, result, tt.expect)
			}
		})
	}
}

func TestNewHashUnsupported(t *testing.T) {
	_, err := newHash(HashAlgorithm("unknown"))
	if !errors.Is(err, ErrUnsupportedHashAlgo) {
		t.Errorf("expected ErrUnsupportedHashAlgo, got %v", err)
	}
}

func TestFullWorkflow(t *testing.T) {
	m := NewManager()

	meta1, err := m.Create(CreateOptions{
		OriginalURL: "https://example.com/article/123",
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	meta2, err := m.Create(CreateOptions{
		OriginalURL: "https://blog.example.com/posts/hello-world",
		Strategy:  StrategyHash,
	})
	if err != nil {
		t.Fatalf("Create with hash failed: %v", err)
	}

	meta3, err := m.Create(CreateOptions{
		OriginalURL: "https://shop.example.com/products/42",
		Strategy:  StrategyRandom,
	})
	if err != nil {
		t.Fatalf("Create with random failed: %v", err)
	}

	meta4, err := m.Create(CreateOptions{
		OriginalURL: "https://special.example.com",
		CustomCode: "my-special-link",
	})
	if err != nil {
		t.Fatalf("Create with custom failed: %v", err)
	}

	if m.Count() != 4 {
		t.Fatalf("expected 4 links, got %d", m.Count())
	}

	for i := 0; i < 10; i++ {
		m.GetOriginalURL(meta1.ShortCode)
	}
	for i := 0; i < 5; i++ {
		m.GetOriginalURL(meta2.ShortCode)
	}
	for i := 0; i < 3; i++ {
		m.GetOriginalURL(meta3.ShortCode)
	}

	count1, _ := m.GetVisitCount(meta1.ShortCode)
	count2, _ := m.GetVisitCount(meta2.ShortCode)
	count3, _ := m.GetVisitCount(meta3.ShortCode)
	count4, _ := m.GetVisitCount(meta4.ShortCode)

	if count1 != 10 {
		t.Errorf("meta1 expected 10 visits, got %d", count1)
	}
	if count2 != 5 {
		t.Errorf("meta2 expected 5 visits, got %d", count2)
	}
	if count3 != 3 {
		t.Errorf("meta3 expected 3 visits, got %d", count3)
	}
	if count4 != 0 {
		t.Errorf("meta4 expected 0 visits, got %d", count4)
	}

	total := m.GetTotalVisitCount()
	if total != 18 {
		t.Errorf("expected total 18 visits, got %d", total)
	}

	err = m.Delete(meta2.ShortCode)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if m.Count() != 3 {
		t.Errorf("expected 3 links after delete, got %d", m.Count())
	}

	_, err = m.GetOriginalURL(meta2.ShortCode)
	if !errors.Is(err, ErrShortCodeNotFound) {
		t.Error("expected ErrShortCodeNotFound for deleted link")
	}
}

func TestGenerateWithHashLengthEqualsHexLength(t *testing.T) {
	tests := []struct {
		name   string
		algo   HashAlgorithm
		length int
	}{
		{"sha256 length 64", HashSHA256, 64},
		{"sha1 length 40", HashSHA1, 40},
		{"md5 length 32", HashMD5, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.HashConfig.Algorithm = tt.algo
			cfg.HashConfig.Length = tt.length
			m, err := NewManagerWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewManagerWithConfig failed: %v", err)
			}

			meta, err := m.Create(CreateOptions{
				OriginalURL: "https://example.com/test",
				Strategy:    StrategyHash,
			})
			if err != nil {
				t.Fatalf("Create with hash strategy failed: %v", err)
			}
			if len(meta.ShortCode) != tt.length {
				t.Errorf("expected shortcode length %d, got %d", tt.length, len(meta.ShortCode))
			}
		})
	}
}

func TestGenerateWithHashDefensiveLengthBounds(t *testing.T) {
	tests := []struct {
		name          string
		algo          HashAlgorithm
		configLength  int
		expectedMax   int
	}{
		{"md5 length 100 exceeds max 32", HashMD5, 100, 32},
		{"sha1 length 999 exceeds max 40", HashSHA1, 999, 40},
		{"sha256 length 500 exceeds max 64", HashSHA256, 500, 64},
		{"md5 length zero falls back to full", HashMD5, 0, 32},
		{"md5 length negative falls back to full", HashMD5, -5, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Manager{links: make(map[string]*ShortLink)}
			config := HashStrategyConfig{
				Algorithm:  tt.algo,
				Length:     tt.configLength,
				MaxRetries: 3,
			}

			code, err := m.generateWithHash("https://example.com/defensive", config)
			if err != nil {
				t.Fatalf("generateWithHash returned error: %v", err)
			}
			if len(code) != tt.expectedMax {
				t.Errorf("expected safe length %d, got %d (code=%q)", tt.expectedMax, len(code), code)
			}
			if len(code) == 0 {
				t.Error("returned empty short code")
			}
		})
	}
}

func TestGenerateWithHashLengthExceedsAlgoMax(t *testing.T) {
	tests := []struct {
		name   string
		algo   HashAlgorithm
		length int
	}{
		{"md5 length 40", HashMD5, 40},
		{"sha1 length 50", HashSHA1, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.HashConfig.Algorithm = tt.algo
			cfg.HashConfig.Length = tt.length
			_, err := NewManagerWithConfig(cfg)
			if !errors.Is(err, ErrHashLengthInvalid) {
				t.Errorf("expected ErrHashLengthInvalid for %s length %d, got %v", tt.algo, tt.length, err)
			}
		})
	}
}

func makeLargeCharset(size int) string {
	buf := make([]rune, 0, size)
	for r := rune(32); len(buf) < size && r <= 126; r++ {
		buf = append(buf, r)
	}
	return string(buf)
}

func chiSquaredCriticalValue(df int, z float64) float64 {
	k := float64(df)
	a := 1.0 - 2.0/(9.0*k)
	b := z * math.Sqrt(2.0/(9.0*k))
	return k * (a + b) * (a + b) * (a + b)
}

func TestGenerateWithRandomNoModuloBias(t *testing.T) {
	testCases := []struct {
		name       string
		charset    string
		numTrials  int
		codeLength int
	}{
		{
			name:       "charset_7_small",
			charset:    "0123456",
			numTrials:  1000,
			codeLength: 8,
		},
		{
			name:       "charset_62_base62",
			charset:    base62Chars,
			numTrials:  3000,
			codeLength: 8,
		},
		{
			name:       "charset_95_ascii_printable",
			charset:    makeLargeCharset(95),
			numTrials:  500,
			codeLength: 8,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			charsetLen := len(tc.charset)

			cfg := DefaultConfig()
			cfg.RandomConfig.Charset = tc.charset
			cfg.RandomConfig.Length = tc.codeLength

			m, err := NewManagerWithConfig(cfg)
			if err != nil {
				t.Fatalf("NewManagerWithConfig failed: %v", err)
			}

			charCounts := make(map[rune]int)
			totalChars := 0

			for i := 0; i < tc.numTrials; i++ {
				code, err := m.generateWithRandom(cfg.RandomConfig)
				if err != nil {
					t.Fatalf("generateWithRandom failed at trial %d: %v", i, err)
				}
				for _, c := range code {
					charCounts[c]++
					totalChars++
				}
			}

			expectedPerChar := float64(totalChars) / float64(charsetLen)
			var chiSquared float64
			for _, c := range tc.charset {
				observed := float64(charCounts[c])
				chiSquared += (observed - expectedPerChar) * (observed - expectedPerChar) / expectedPerChar
			}

			df := charsetLen - 1
			criticalValue := chiSquaredCriticalValue(df, 3.090)

			t.Logf("chi-squared = %.2f, df = %d, critical value = %.2f", chiSquared, df, criticalValue)
			t.Logf("expected per char = %.1f, total chars = %d", expectedPerChar, totalChars)

			if chiSquared > criticalValue {
				minCount, maxCount := totalChars, 0
				for _, c := range tc.charset {
					if charCounts[c] < minCount {
						minCount = charCounts[c]
					}
					if charCounts[c] > maxCount {
						maxCount = charCounts[c]
					}
				}
				t.Errorf("chi-squared = %.2f exceeds critical value %.2f, "+
					"indicating non-uniform character distribution (modulo bias); "+
					"min count = %d, max count = %d, expected ≈ %.0f",
					chiSquared, criticalValue, minCount, maxCount, expectedPerChar)
			}
		})
	}
}
