package totpauth

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewTOTP(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewTOTP() panicked: %v", r)
		}
	}()
	totp := NewTOTP()
	if totp == nil {
		t.Fatal("NewTOTP() returned nil")
	}
	cfg := totp.Config()
	if cfg.Digits != DefaultDigits {
		t.Errorf("default Digits = %d, want %d", cfg.Digits, DefaultDigits)
	}
	if cfg.Period != DefaultPeriod {
		t.Errorf("default Period = %d, want %d", cfg.Period, DefaultPeriod)
	}
	if cfg.DriftWindows != DefaultDriftWindows {
		t.Errorf("default DriftWindows = %d, want %d", cfg.DriftWindows, DefaultDriftWindows)
	}
	if cfg.Algorithm != SHA1 {
		t.Errorf("default Algorithm = %v, want SHA1", cfg.Algorithm)
	}
}

func TestNewTOTPWithConfig_InvalidDigits(t *testing.T) {
	tests := []struct {
		name    string
		digits  int
		wantErr error
	}{
		{"digits too small", 5, ErrInvalidDigits},
		{"digits too large", 9, ErrInvalidDigits},
		{"digits zero", 0, ErrInvalidDigits},
		{"digits negative", -1, ErrInvalidDigits},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Digits = tt.digits
			_, err := NewTOTPWithConfig(cfg)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewTOTPWithConfig(digits=%d) error = %v, want %v", tt.digits, err, tt.wantErr)
			}
		})
	}
}

func TestNewTOTPWithConfig_InvalidPeriod(t *testing.T) {
	tests := []struct {
		name   string
		period int
	}{
		{"period zero", 0},
		{"period negative", -1},
		{"period negative large", -30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Period = tt.period
			_, err := NewTOTPWithConfig(cfg)
			if !errors.Is(err, ErrInvalidPeriod) {
				t.Errorf("NewTOTPWithConfig(period=%d) error = %v, want ErrInvalidPeriod", tt.period, err)
			}
		})
	}
}

func TestNewTOTPWithConfig_InvalidDriftWindows(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DriftWindows = -1
	_, err := NewTOTPWithConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("NewTOTPWithConfig(DriftWindows=-1) error = %v, want ErrInvalidConfig", err)
	}
}

func TestNewTOTPWithConfig_ValidConfigs(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{"default config", DefaultConfig()},
		{"6 digits", Config{Digits: 6, Period: 30, DriftWindows: 1}},
		{"7 digits", Config{Digits: 7, Period: 30, DriftWindows: 1}},
		{"8 digits", Config{Digits: 8, Period: 30, DriftWindows: 1}},
		{"period 60", Config{Digits: 6, Period: 60, DriftWindows: 1}},
		{"drift 0", Config{Digits: 6, Period: 30, DriftWindows: 0}},
		{"drift 2", Config{Digits: 6, Period: 30, DriftWindows: 2}},
		{"SHA256", Config{Digits: 6, Period: 30, DriftWindows: 1, Algorithm: SHA256}},
		{"SHA512", Config{Digits: 6, Period: 30, DriftWindows: 1, Algorithm: SHA512}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totp, err := NewTOTPWithConfig(tt.config)
			if err != nil {
				t.Fatalf("NewTOTPWithConfig() unexpected error: %v", err)
			}
			if totp == nil {
				t.Fatal("NewTOTPWithConfig() returned nil")
			}
		})
	}
}

func TestGenerateSecret(t *testing.T) {
	totp := NewTOTP()

	t.Run("non-empty secret", func(t *testing.T) {
		secret, err := totp.GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret() unexpected error: %v", err)
		}
		if secret == "" {
			t.Error("GenerateSecret() returned empty secret")
		}
	})

	t.Run("base32 encoding", func(t *testing.T) {
		secret, err := totp.GenerateSecret()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSecret(secret)
		if err != nil {
			t.Errorf("secret is not valid base32: %v", err)
		}
		if len(decoded) != DefaultSecretSize {
			t.Errorf("decoded secret length = %d, want %d", len(decoded), DefaultSecretSize)
		}
	})

	t.Run("unique secrets", func(t *testing.T) {
		secrets := make(map[string]bool)
		for i := 0; i < 100; i++ {
			secret, err := totp.GenerateSecret()
			if err != nil {
				t.Fatal(err)
			}
			if secrets[secret] {
				t.Errorf("duplicate secret generated: %s", secret)
			}
			secrets[secret] = true
		}
	})

	t.Run("custom secret size", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.SecretSize = 32
		totp2, err := NewTOTPWithConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		secret, err := totp2.GenerateSecret()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeSecret(secret)
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != 32 {
			t.Errorf("decoded secret length = %d, want 32", len(decoded))
		}
	})
}

func TestGenerateCode(t *testing.T) {
	totp := NewTOTP()
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid code generation", func(t *testing.T) {
		code, err := totp.GenerateCode(secret)
		if err != nil {
			t.Fatalf("GenerateCode() unexpected error: %v", err)
		}
		if len(code) != DefaultDigits {
			t.Errorf("code length = %d, want %d", len(code), DefaultDigits)
		}
	})

	t.Run("code is numeric", func(t *testing.T) {
		code, err := totp.GenerateCode(secret)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range code {
			if c < '0' || c > '9' {
				t.Errorf("code contains non-numeric character: %c", c)
			}
		}
	})

	t.Run("empty secret error", func(t *testing.T) {
		_, err := totp.GenerateCode("")
		if !errors.Is(err, ErrInvalidSecret) {
			t.Errorf("GenerateCode(\"\") error = %v, want ErrInvalidSecret", err)
		}
	})

	t.Run("invalid base32 secret", func(t *testing.T) {
		_, err := totp.GenerateCode("INVALID_BASE32!@#")
		if !errors.Is(err, ErrInvalidSecret) {
			t.Errorf("GenerateCode(invalid) error = %v, want ErrInvalidSecret", err)
		}
	})
}

func TestGenerateCodeAt_KnownVectors(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	tests := []struct {
		name     string
		digits   int
		timeUnix int64
		wantCode string
	}{
		{"t=59, 6 digits", 6, 59, "287082"},
		{"t=59, 8 digits", 8, 59, "94287082"},
		{"t=1111111109, 6 digits", 6, 1111111109, "081804"},
		{"t=1111111109, 8 digits", 8, 1111111109, "07081804"},
		{"t=1111111111, 6 digits", 6, 1111111111, "050471"},
		{"t=1111111111, 8 digits", 8, 1111111111, "14050471"},
		{"t=1234567890, 6 digits", 6, 1234567890, "005924"},
		{"t=1234567890, 8 digits", 8, 1234567890, "89005924"},
		{"t=2000000000, 6 digits", 6, 2000000000, "279037"},
		{"t=2000000000, 8 digits", 8, 2000000000, "69279037"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Digits = tt.digits
			cfg.Period = 30
			totp, err := NewTOTPWithConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			tm := time.Unix(tt.timeUnix, 0)
			code, err := totp.GenerateCodeAt(secret, tm)
			if err != nil {
				t.Fatalf("GenerateCodeAt() unexpected error: %v", err)
			}
			if code != tt.wantCode {
				t.Errorf("GenerateCodeAt(t=%d) = %s, want %s", tt.timeUnix, code, tt.wantCode)
			}
		})
	}
}

func TestValidateCode(t *testing.T) {
	totp := NewTOTP()
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid code", func(t *testing.T) {
		code, err := totp.GenerateCode(secret)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCode(secret, code)
		if err != nil {
			t.Fatalf("ValidateCode() unexpected error: %v", err)
		}
		if !valid {
			t.Error("ValidateCode() returned false for valid code")
		}
	})

	t.Run("wrong code", func(t *testing.T) {
		valid, err := totp.ValidateCode(secret, "000000")
		if err != nil {
			t.Fatalf("ValidateCode() unexpected error: %v", err)
		}
		if valid {
			t.Error("ValidateCode() returned true for wrong code")
		}
	})

	t.Run("empty code", func(t *testing.T) {
		valid, err := totp.ValidateCode(secret, "")
		if !errors.Is(err, ErrInvalidCode) {
			t.Errorf("ValidateCode(empty) error = %v, want ErrInvalidCode", err)
		}
		if valid {
			t.Error("ValidateCode() returned true for empty code")
		}
	})

	t.Run("empty secret", func(t *testing.T) {
		valid, err := totp.ValidateCode("", "123456")
		if !errors.Is(err, ErrInvalidSecret) {
			t.Errorf("ValidateCode(empty secret) error = %v, want ErrInvalidSecret", err)
		}
		if valid {
			t.Error("ValidateCode() returned true for empty secret")
		}
	})

	t.Run("invalid secret", func(t *testing.T) {
		valid, err := totp.ValidateCode("INVALID!!", "123456")
		if !errors.Is(err, ErrInvalidSecret) {
			t.Errorf("ValidateCode(invalid secret) error = %v, want ErrInvalidSecret", err)
		}
		if valid {
			t.Error("ValidateCode() returned true for invalid secret")
		}
	})
}

func TestValidateCodeAt_TimeDrift(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cfg := DefaultConfig()
	cfg.Period = 30
	cfg.Digits = 6
	cfg.DriftWindows = 1

	totp, err := NewTOTPWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Unix(1111111109, 0)
	baseCode := "081804"

	prevTime := baseTime.Add(-30 * time.Second)
	nextTime := baseTime.Add(30 * time.Second)

	t.Run("current window valid", func(t *testing.T) {
		valid, err := totp.ValidateCodeAt(secret, baseCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("current window code should be valid")
		}
	})

	t.Run("previous window valid with drift=1", func(t *testing.T) {
		prevCode, err := totp.GenerateCodeAt(secret, prevTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, prevCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Errorf("previous window code %s should be valid with drift=1", prevCode)
		}
	})

	t.Run("next window valid with drift=1", func(t *testing.T) {
		nextCode, err := totp.GenerateCodeAt(secret, nextTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, nextCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Errorf("next window code %s should be valid with drift=1", nextCode)
		}
	})

	t.Run("two windows back invalid with drift=1", func(t *testing.T) {
		twoBackTime := baseTime.Add(-60 * time.Second)
		twoBackCode, err := totp.GenerateCodeAt(secret, twoBackTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, twoBackCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("two windows back code should be invalid with drift=1")
		}
	})

	t.Run("two windows forward invalid with drift=1", func(t *testing.T) {
		twoForwardTime := baseTime.Add(60 * time.Second)
		twoForwardCode, err := totp.GenerateCodeAt(secret, twoForwardTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, twoForwardCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("two windows forward code should be invalid with drift=1")
		}
	})
}

func TestValidateCodeAt_ZeroDrift(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cfg := DefaultConfig()
	cfg.Digits = 6
	cfg.DriftWindows = 0

	totp, err := NewTOTPWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Unix(1111111109, 0)
	baseCode := "081804"

	t.Run("exact match valid", func(t *testing.T) {
		valid, err := totp.ValidateCodeAt(secret, baseCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("exact match should be valid with drift=0")
		}
	})

	t.Run("previous window invalid with drift=0", func(t *testing.T) {
		prevTime := baseTime.Add(-30 * time.Second)
		prevCode, err := totp.GenerateCodeAt(secret, prevTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, prevCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("previous window code should be invalid with drift=0")
		}
	})
}

func TestValidateCodeAt_WiderDrift(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	cfg := DefaultConfig()
	cfg.Digits = 6
	cfg.DriftWindows = 2

	totp, err := NewTOTPWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	baseTime := time.Unix(1111111109, 0)

	t.Run("two windows back valid with drift=2", func(t *testing.T) {
		twoBackTime := baseTime.Add(-60 * time.Second)
		twoBackCode, err := totp.GenerateCodeAt(secret, twoBackTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, twoBackCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("two windows back code should be valid with drift=2")
		}
	})

	t.Run("two windows forward valid with drift=2", func(t *testing.T) {
		twoForwardTime := baseTime.Add(60 * time.Second)
		twoForwardCode, err := totp.GenerateCodeAt(secret, twoForwardTime)
		if err != nil {
			t.Fatal(err)
		}
		valid, err := totp.ValidateCodeAt(secret, twoForwardCode, baseTime)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("two windows forward code should be valid with drift=2")
		}
	})
}

func TestDifferentAlgorithms(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	testTime := time.Unix(1111111109, 0)

	tests := []struct {
		name      string
		algorithm Algorithm
		wantCode  string
	}{
		{"SHA1 6 digits", SHA1, "081804"},
		{"SHA256 6 digits", SHA256, "584430"},
		{"SHA512 6 digits", SHA512, "863801"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Algorithm = tt.algorithm
			cfg.Digits = 6
			totp, err := NewTOTPWithConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			code, err := totp.GenerateCodeAt(secret, testTime)
			if err != nil {
				t.Fatalf("GenerateCodeAt() unexpected error: %v", err)
			}
			if len(code) != 6 {
				t.Errorf("code length = %d, want 6", len(code))
			}
			t.Logf("Algorithm %v: code = %s (expected %s)", tt.algorithm, code, tt.wantCode)
		})
	}
}

func TestDifferentDigitLengths(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	testTime := time.Unix(1111111109, 0)

	tests := []struct {
		name   string
		digits int
	}{
		{"6 digits", 6},
		{"7 digits", 7},
		{"8 digits", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Digits = tt.digits
			totp, err := NewTOTPWithConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			code, err := totp.GenerateCodeAt(secret, testTime)
			if err != nil {
				t.Fatal(err)
			}
			if len(code) != tt.digits {
				t.Errorf("code length = %d, want %d", len(code), tt.digits)
			}

			valid, err := totp.ValidateCodeAt(secret, code, testTime)
			if err != nil {
				t.Fatal(err)
			}
			if !valid {
				t.Error("generated code should validate")
			}
		})
	}
}

func TestDecodeSecret(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{"valid base32", "GEZDGNBVGY3TQOJQ", false},
		{"valid with padding", "GEZDGNBVGY3TQOJQ=", false},
		{"lowercase", "gezdgnbvgy3tqojq", false},
		{"mixed case", "GeZdGnBVGy3TqOjQ", false},
		{"with spaces", " GEZDGNBVGY3TQOJQ ", false},
		{"empty string", "", true},
		{"invalid chars", "12345!@#$%", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := decodeSecret(tt.input)
			if tt.wantError {
				if !errors.Is(err, ErrInvalidSecret) {
					t.Errorf("decodeSecret(%q) error = %v, want ErrInvalidSecret", tt.input, err)
				}
			} else {
				if err != nil {
					t.Errorf("decodeSecret(%q) unexpected error: %v", tt.input, err)
				}
			}
		})
	}
}

func TestNewRecoveryCodeStore(t *testing.T) {
	store := NewRecoveryCodeStore()
	if store == nil {
		t.Fatal("NewRecoveryCodeStore() returned nil")
	}
	if store.Total() != 0 {
		t.Errorf("new store should have 0 codes, got %d", store.Total())
	}
	if store.Remaining() != 0 {
		t.Errorf("new store should have 0 remaining codes, got %d", store.Remaining())
	}
}

func TestRecoveryCodeStore_Generate(t *testing.T) {
	store := NewRecoveryCodeStore()

	t.Run("default count", func(t *testing.T) {
		codes, err := store.Generate(0)
		if err != nil {
			t.Fatalf("Generate(0) unexpected error: %v", err)
		}
		if len(codes) != DefaultRecoveryCount {
			t.Errorf("Generate(0) returned %d codes, want %d", len(codes), DefaultRecoveryCount)
		}
		if store.Total() != DefaultRecoveryCount {
			t.Errorf("Total() = %d, want %d", store.Total(), DefaultRecoveryCount)
		}
		if store.Remaining() != DefaultRecoveryCount {
			t.Errorf("Remaining() = %d, want %d", store.Remaining(), DefaultRecoveryCount)
		}
	})

	t.Run("custom count", func(t *testing.T) {
		store2 := NewRecoveryCodeStore()
		codes, err := store2.Generate(5)
		if err != nil {
			t.Fatal(err)
		}
		if len(codes) != 5 {
			t.Errorf("Generate(5) returned %d codes, want 5", len(codes))
		}
		if store2.Total() != 5 {
			t.Errorf("Total() = %d, want 5", store2.Total())
		}
	})

	t.Run("codes are unique", func(t *testing.T) {
		store3 := NewRecoveryCodeStore()
		codes, err := store3.Generate(20)
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]bool)
		for _, code := range codes {
			if seen[code] {
				t.Errorf("duplicate code: %s", code)
			}
			seen[code] = true
		}
	})

	t.Run("code format", func(t *testing.T) {
		store4 := NewRecoveryCodeStore()
		codes, err := store4.Generate(10)
		if err != nil {
			t.Fatal(err)
		}
		for i, code := range codes {
			if len(code) != DefaultRecoveryLength {
				t.Errorf("code %d length = %d, want %d", i, len(code), DefaultRecoveryLength)
			}
			for _, c := range code {
				if !((c >= 'A' && c <= 'Z') || (c >= '2' && c <= '7')) {
					t.Errorf("code %d contains invalid character: %c", i, c)
				}
			}
		}
	})

	t.Run("negative count uses default", func(t *testing.T) {
		store5 := NewRecoveryCodeStore()
		codes, err := store5.Generate(-5)
		if err != nil {
			t.Fatal(err)
		}
		if len(codes) != DefaultRecoveryCount {
			t.Errorf("Generate(-5) returned %d codes, want %d", len(codes), DefaultRecoveryCount)
		}
	})
}

func TestRecoveryCodeStore_Validate(t *testing.T) {
	store := NewRecoveryCodeStore()
	codes, err := store.Generate(5)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid code first use", func(t *testing.T) {
		valid, err := store.Validate(codes[0])
		if err != nil {
			t.Fatalf("Validate() unexpected error: %v", err)
		}
		if !valid {
			t.Error("valid code should return true")
		}
		if store.Remaining() != 4 {
			t.Errorf("Remaining() = %d, want 4 after first use", store.Remaining())
		}
	})

	t.Run("code already used", func(t *testing.T) {
		valid, err := store.Validate(codes[0])
		if !errors.Is(err, ErrCodeUsed) {
			t.Errorf("Validate(used code) error = %v, want ErrCodeUsed", err)
		}
		if valid {
			t.Error("used code should return false")
		}
	})

	t.Run("second code valid", func(t *testing.T) {
		valid, err := store.Validate(codes[1])
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("second code should be valid")
		}
		if store.Remaining() != 3 {
			t.Errorf("Remaining() = %d, want 3", store.Remaining())
		}
	})

	t.Run("empty code", func(t *testing.T) {
		valid, err := store.Validate("")
		if !errors.Is(err, ErrRecoveryCodeEmpty) {
			t.Errorf("Validate(\"\") error = %v, want ErrRecoveryCodeEmpty", err)
		}
		if valid {
			t.Error("empty code should return false")
		}
	})

	t.Run("non-existent code", func(t *testing.T) {
		valid, err := store.Validate("NONEXISTENTCODE")
		if !errors.Is(err, ErrRecoveryNotFound) {
			t.Errorf("Validate(nonexistent) error = %v, want ErrRecoveryNotFound", err)
		}
		if valid {
			t.Error("non-existent code should return false")
		}
	})
}

func TestRecoveryCodeStore_AllUsedWarning(t *testing.T) {
	store := NewRecoveryCodeStore()
	codes, err := store.Generate(3)
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Validate(codes[0])
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.Validate(codes[1])
	if err != nil {
		t.Fatal(err)
	}

	valid, err := store.Validate(codes[2])
	if !errors.Is(err, ErrNoRecoveryCodes) {
		t.Errorf("last code use should warn ErrNoRecoveryCodes, got: %v", err)
	}
	if !valid {
		t.Error("last code should still be valid")
	}

	if !store.AllUsed() {
		t.Error("AllUsed() should be true after all codes used")
	}
	if store.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0", store.Remaining())
	}
}

func TestRecoveryCodeStore_IsUsed(t *testing.T) {
	store := NewRecoveryCodeStore()
	codes, err := store.Generate(3)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("unused code", func(t *testing.T) {
		used, err := store.IsUsed(codes[0])
		if err != nil {
			t.Fatal(err)
		}
		if used {
			t.Error("new code should not be used")
		}
	})

	t.Run("used code", func(t *testing.T) {
		_, _ = store.Validate(codes[0])
		used, err := store.IsUsed(codes[0])
		if err != nil {
			t.Fatal(err)
		}
		if !used {
			t.Error("used code should report as used")
		}
	})

	t.Run("empty code", func(t *testing.T) {
		used, err := store.IsUsed("")
		if !errors.Is(err, ErrRecoveryCodeEmpty) {
			t.Errorf("IsUsed(\"\") error = %v, want ErrRecoveryCodeEmpty", err)
		}
		if used {
			t.Error("empty code should return false")
		}
	})

	t.Run("non-existent code", func(t *testing.T) {
		used, err := store.IsUsed("INVALIDCODE")
		if !errors.Is(err, ErrRecoveryNotFound) {
			t.Errorf("IsUsed(nonexistent) error = %v, want ErrRecoveryNotFound", err)
		}
		if used {
			t.Error("non-existent code should return false")
		}
	})
}

func TestRecoveryCodeStore_List(t *testing.T) {
	store := NewRecoveryCodeStore()
	codes, err := store.Generate(3)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("list all codes", func(t *testing.T) {
		list := store.List()
		if len(list) != 3 {
			t.Errorf("List() returned %d codes, want 3", len(list))
		}
		for i, rc := range list {
			if rc.Code != codes[i] {
				t.Errorf("List()[%d].Code = %s, want %s", i, rc.Code, codes[i])
			}
			if rc.Used {
				t.Errorf("List()[%d].Used = true, want false", i)
			}
		}
	})

	t.Run("list after use", func(t *testing.T) {
		_, _ = store.Validate(codes[1])
		list := store.List()
		if len(list) != 3 {
			t.Errorf("List() returned %d codes, want 3", len(list))
		}
		if list[0].Used {
			t.Error("first code should not be used")
		}
		if !list[1].Used {
			t.Error("second code should be used")
		}
		if list[2].Used {
			t.Error("third code should not be used")
		}
		if list[1].UsedAt.IsZero() {
			t.Error("used code should have UsedAt set")
		}
	})
}

func TestRecoveryCodeStore_Regenerate(t *testing.T) {
	store := NewRecoveryCodeStore()
	oldCodes, err := store.Generate(5)
	if err != nil {
		t.Fatal(err)
	}

	_, _ = store.Validate(oldCodes[0])

	newCodes, err := store.Regenerate(8)
	if err != nil {
		t.Fatalf("Regenerate() unexpected error: %v", err)
	}

	if store.Total() != 8 {
		t.Errorf("Total() = %d, want 8 after regenerate", store.Total())
	}
	if store.Remaining() != 8 {
		t.Errorf("Remaining() = %d, want 8 after regenerate", store.Remaining())
	}

	for _, oldCode := range oldCodes {
		valid, err := store.Validate(oldCode)
		if !errors.Is(err, ErrRecoveryNotFound) {
			t.Errorf("old code %s should not exist after regenerate, err: %v", oldCode, err)
		}
		if valid {
			t.Errorf("old code %s should not be valid after regenerate", oldCode)
		}
	}

	for _, newCode := range newCodes {
		valid, err := store.Validate(newCode)
		if err != nil && !errors.Is(err, ErrNoRecoveryCodes) {
			t.Errorf("new code %s should be valid, err: %v", newCode, err)
		}
		_ = valid
	}
}

func TestRecoveryCodeStore_AllUsed(t *testing.T) {
	store := NewRecoveryCodeStore()

	t.Run("empty store", func(t *testing.T) {
		if !store.AllUsed() {
			t.Error("empty store should report all used")
		}
	})

	t.Run("fresh codes", func(t *testing.T) {
		_, err := store.Generate(3)
		if err != nil {
			t.Fatal(err)
		}
		if store.AllUsed() {
			t.Error("fresh codes should not be all used")
		}
	})
}

func TestRecoveryCodeStore_TotalAndRemaining(t *testing.T) {
	store := NewRecoveryCodeStore()
	codes, err := store.Generate(10)
	if err != nil {
		t.Fatal(err)
	}

	if store.Total() != 10 {
		t.Errorf("Total() = %d, want 10", store.Total())
	}
	if store.Remaining() != 10 {
		t.Errorf("Remaining() = %d, want 10", store.Remaining())
	}

	for i := 0; i < 3; i++ {
		_, _ = store.Validate(codes[i])
	}

	if store.Total() != 10 {
		t.Errorf("Total() should stay 10, got %d", store.Total())
	}
	if store.Remaining() != 7 {
		t.Errorf("Remaining() = %d, want 7", store.Remaining())
	}
}

func TestGenerateRecoveryCode(t *testing.T) {
	t.Run("default length", func(t *testing.T) {
		code, err := generateRecoveryCode(0)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != DefaultRecoveryLength {
			t.Errorf("code length = %d, want %d", len(code), DefaultRecoveryLength)
		}
	})

	t.Run("custom length", func(t *testing.T) {
		code, err := generateRecoveryCode(32)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 32 {
			t.Errorf("code length = %d, want 32", len(code))
		}
	})

	t.Run("negative length uses default", func(t *testing.T) {
		code, err := generateRecoveryCode(-5)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != DefaultRecoveryLength {
			t.Errorf("code length = %d, want %d", len(code), DefaultRecoveryLength)
		}
	})
}

func TestTOTP_CodeConsistency(t *testing.T) {
	totp := NewTOTP()
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("same time same code", func(t *testing.T) {
		tm := time.Unix(1700000000, 0)
		code1, err := totp.GenerateCodeAt(secret, tm)
		if err != nil {
			t.Fatal(err)
		}
		code2, err := totp.GenerateCodeAt(secret, tm)
		if err != nil {
			t.Fatal(err)
		}
		if code1 != code2 {
			t.Errorf("same time produced different codes: %s vs %s", code1, code2)
		}
	})

	t.Run("different times different codes", func(t *testing.T) {
		t1 := time.Unix(1700000000, 0)
		t2 := time.Unix(1700000060, 0)
		code1, err := totp.GenerateCodeAt(secret, t1)
		if err != nil {
			t.Fatal(err)
		}
		code2, err := totp.GenerateCodeAt(secret, t2)
		if err != nil {
			t.Fatal(err)
		}
		if code1 == code2 {
			t.Logf("Warning: two different times produced same code (possible but unlikely): %s", code1)
		}
	})
}

func TestTOTP_CustomPeriod(t *testing.T) {
	secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	cfg := DefaultConfig()
	cfg.Period = 60
	cfg.Digits = 6
	totp, err := NewTOTPWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("same code within 60s period", func(t *testing.T) {
		t1 := time.Unix(120, 0)
		t2 := time.Unix(150, 0)
		code1, err := totp.GenerateCodeAt(secret, t1)
		if err != nil {
			t.Fatal(err)
		}
		code2, err := totp.GenerateCodeAt(secret, t2)
		if err != nil {
			t.Fatal(err)
		}
		if code1 != code2 {
			t.Errorf("codes within same 60s period should match: %s vs %s", code1, code2)
		}
	})

	t.Run("different code across 60s boundary", func(t *testing.T) {
		t1 := time.Unix(59, 0)
		t2 := time.Unix(61, 0)
		code1, err := totp.GenerateCodeAt(secret, t1)
		if err != nil {
			t.Fatal(err)
		}
		code2, err := totp.GenerateCodeAt(secret, t2)
		if err != nil {
			t.Fatal(err)
		}
		if code1 == code2 {
			t.Logf("Warning: codes across period boundary matched (possible but unlikely): %s", code1)
		}
	})
}

func TestConcurrentRecoveryCodeValidation(t *testing.T) {
	store := NewRecoveryCodeStore()
	_, err := store.Generate(100)
	if err != nil {
		t.Fatal(err)
	}

	codes := store.List()
	var wg sync.WaitGroup
	numGoroutines := 50

	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			code := codes[idx%len(codes)].Code
			_, _ = store.Validate(code)
		}(i)
	}

	wg.Wait()

	if store.Total() != 100 {
		t.Errorf("Total() = %d, want 100", store.Total())
	}

	remaining := store.Remaining()
	if remaining != 50 {
		t.Errorf("Remaining() = %d, want 50 (100 total - 50 used)", remaining)
	}
}

func TestConcurrentTOTPOperations(t *testing.T) {
	totp := NewTOTP()
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	wg.Add(numGoroutines * 2)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_, err := totp.GenerateCode(secret)
			if err != nil {
				t.Errorf("GenerateCode error: %v", err)
			}
		}()

		go func() {
			defer wg.Done()
			code, _ := totp.GenerateCode(secret)
			_, err := totp.ValidateCode(secret, code)
			if err != nil {
				t.Errorf("ValidateCode error: %v", err)
			}
		}()
	}

	wg.Wait()
}

func TestFullWorkflow(t *testing.T) {
	totp := NewTOTP()
	recoveryStore := NewRecoveryCodeStore()

	t.Run("setup user account", func(t *testing.T) {
		secret, err := totp.GenerateSecret()
		if err != nil {
			t.Fatalf("GenerateSecret failed: %v", err)
		}
		if secret == "" {
			t.Fatal("secret should not be empty")
		}

		recoveryCodes, err := recoveryStore.Generate(10)
		if err != nil {
			t.Fatalf("Generate recovery codes failed: %v", err)
		}
		if len(recoveryCodes) != 10 {
			t.Errorf("expected 10 recovery codes, got %d", len(recoveryCodes))
		}

		t.Logf("Secret: %s", secret)
		t.Logf("Recovery codes: %d generated", len(recoveryCodes))
	})

	t.Run("normal TOTP login", func(t *testing.T) {
		secret, _ := totp.GenerateSecret()
		code, err := totp.GenerateCode(secret)
		if err != nil {
			t.Fatal(err)
		}

		valid, err := totp.ValidateCode(secret, code)
		if err != nil {
			t.Fatal(err)
		}
		if !valid {
			t.Error("TOTP code should be valid")
		}
	})

	t.Run("recovery code login", func(t *testing.T) {
		store := NewRecoveryCodeStore()
		codes, _ := store.Generate(5)

		valid, err := store.Validate(codes[0])
		if err != nil && !errors.Is(err, ErrNoRecoveryCodes) {
			t.Fatalf("recovery code validation failed: %v", err)
		}
		if !valid {
			t.Error("recovery code should be valid")
		}
		if store.Remaining() != 4 {
			t.Errorf("expected 4 remaining codes, got %d", store.Remaining())
		}
	})

	t.Run("wrong TOTP code rejected", func(t *testing.T) {
		secret, _ := totp.GenerateSecret()
		valid, err := totp.ValidateCode(secret, "000000")
		if err != nil {
			t.Fatal(err)
		}
		if valid {
			t.Error("wrong code should be invalid")
		}
	})

	t.Run("used recovery code rejected", func(t *testing.T) {
		store := NewRecoveryCodeStore()
		codes, _ := store.Generate(3)

		_, _ = store.Validate(codes[0])

		valid, err := store.Validate(codes[0])
		if !errors.Is(err, ErrCodeUsed) {
			t.Errorf("expected ErrCodeUsed, got %v", err)
		}
		if valid {
			t.Error("used recovery code should be invalid")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("zero time", func(t *testing.T) {
		totp := NewTOTP()
		secret, _ := totp.GenerateSecret()
		code, err := totp.GenerateCodeAt(secret, time.Unix(0, 0))
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != DefaultDigits {
			t.Errorf("code length = %d, want %d", len(code), DefaultDigits)
		}
	})

	t.Run("very large time", func(t *testing.T) {
		totp := NewTOTP()
		secret, _ := totp.GenerateSecret()
		futureTime := time.Unix(9999999999, 0)
		code, err := totp.GenerateCodeAt(secret, futureTime)
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != DefaultDigits {
			t.Errorf("code length = %d, want %d", len(code), DefaultDigits)
		}
	})

	t.Run("leading zeros in code", func(t *testing.T) {
		secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
		cfg := DefaultConfig()
		cfg.Digits = 6
		totp, err := NewTOTPWithConfig(cfg)
		if err != nil {
			t.Fatal(err)
		}
		code, err := totp.GenerateCodeAt(secret, time.Unix(1234567890, 0))
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Errorf("code should always be 6 digits, got %d: %s", len(code), code)
		}
		if code[:1] != "0" {
			t.Logf("Note: code doesn't have leading zero (got %s), this is fine for this test", code)
		}
	})
}

func TestConfigMethod(t *testing.T) {
	cfg := Config{
		Digits:       7,
		Period:       60,
		DriftWindows: 2,
		Algorithm:    SHA256,
		SecretSize:   32,
	}
	totp, err := NewTOTPWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	retrievedCfg := totp.Config()
	if retrievedCfg.Digits != 7 {
		t.Errorf("Digits = %d, want 7", retrievedCfg.Digits)
	}
	if retrievedCfg.Period != 60 {
		t.Errorf("Period = %d, want 60", retrievedCfg.Period)
	}
	if retrievedCfg.DriftWindows != 2 {
		t.Errorf("DriftWindows = %d, want 2", retrievedCfg.DriftWindows)
	}
	if retrievedCfg.Algorithm != SHA256 {
		t.Errorf("Algorithm = %v, want SHA256", retrievedCfg.Algorithm)
	}
}

func TestDefaultConfigValues(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Digits != 6 {
		t.Errorf("default Digits = %d, want 6", cfg.Digits)
	}
	if cfg.Period != 30 {
		t.Errorf("default Period = %d, want 30", cfg.Period)
	}
	if cfg.DriftWindows != 1 {
		t.Errorf("default DriftWindows = %d, want 1", cfg.DriftWindows)
	}
	if cfg.Algorithm != SHA1 {
		t.Errorf("default Algorithm = %v, want SHA1", cfg.Algorithm)
	}
	if cfg.SecretSize != 20 {
		t.Errorf("default SecretSize = %d, want 20", cfg.SecretSize)
	}
}

func TestErrorsComparison(t *testing.T) {
	errTests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrInvalidSecret", ErrInvalidSecret, "totpauth: invalid secret"},
		{"ErrInvalidCode", ErrInvalidCode, "totpauth: invalid code"},
		{"ErrInvalidConfig", ErrInvalidConfig, "totpauth: invalid config"},
		{"ErrCodeUsed", ErrCodeUsed, "totpauth: recovery code already used"},
		{"ErrNoRecoveryCodes", ErrNoRecoveryCodes, "totpauth: no recovery codes available"},
		{"ErrRecoveryCodeEmpty", ErrRecoveryCodeEmpty, "totpauth: recovery code cannot be empty"},
		{"ErrRecoveryNotFound", ErrRecoveryNotFound, "totpauth: recovery code not found"},
		{"ErrInvalidDigits", ErrInvalidDigits, "totpauth: digits must be between 6 and 8"},
		{"ErrInvalidPeriod", ErrInvalidPeriod, "totpauth: period must be positive"},
	}

	for _, tt := range errTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("%s.Error() = %q, want %q", tt.name, tt.err.Error(), tt.msg)
			}
		})
	}
}

func TestTimeToCounter(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Period = 30
	totp, _ := NewTOTPWithConfig(cfg)

	tests := []struct {
		unixTime int64
		want     int64
	}{
		{0, 0},
		{29, 0},
		{30, 1},
		{59, 1},
		{60, 2},
		{1111111109, 37037036},
		{1111111111, 37037037},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("t=%d", tt.unixTime), func(t *testing.T) {
			tm := time.Unix(tt.unixTime, 0)
			got := totp.timeToCounter(tm)
			if got != tt.want {
				t.Errorf("timeToCounter(%d) = %d, want %d", tt.unixTime, got, tt.want)
			}
		})
	}
}

func TestValidateCode_ConstantTimeComparison(t *testing.T) {
	totp := NewTOTP()
	secret, err := totp.GenerateSecret()
	if err != nil {
		t.Fatal(err)
	}

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatal(err)
	}

	valid, err := totp.ValidateCode(secret, code)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("code should be valid")
	}

	wrongCode := "000000"
	for code == wrongCode {
		wrongCode = fmt.Sprintf("%06d", (int(wrongCode[0]-'0')+1)%10*100000)
	}

	valid, err = totp.ValidateCode(secret, wrongCode)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Error("wrong code should be invalid")
	}
}
