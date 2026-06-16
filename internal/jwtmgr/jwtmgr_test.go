package jwtmgr

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func generateRSAKeys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return privateKey, &privateKey.PublicKey
}

func newHS256Manager(t *testing.T) (*Manager, func()) {
	t.Helper()
	config := DefaultConfig()
	config.AccessTokenTTL = time.Hour
	config.RefreshTokenTTL = 24 * time.Hour
	config.RenewalWindow = 5 * time.Minute
	config.BlacklistCleanupInt = 100 * time.Millisecond

	signingKey := NewHS256Config([]byte("test-secret-key-1234567890"))
	blacklist := NewMemoryBlacklist(config.BlacklistCleanupInt)
	refreshStore := NewMemoryRefreshStore()

	mgr, err := NewManager(config, signingKey, blacklist, refreshStore)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	cleanup := func() {
		mgr.Close()
	}

	return mgr, cleanup
}

func newRS256Manager(t *testing.T) (*Manager, func()) {
	t.Helper()
	config := DefaultConfig()
	config.AccessTokenTTL = time.Hour
	config.RefreshTokenTTL = 24 * time.Hour
	config.RenewalWindow = 5 * time.Minute
	config.BlacklistCleanupInt = 100 * time.Millisecond

	privateKey, publicKey := generateRSAKeys(t)
	signingKey := NewRS256Config(privateKey, publicKey)
	blacklist := NewMemoryBlacklist(config.BlacklistCleanupInt)
	refreshStore := NewMemoryRefreshStore()

	mgr, err := NewManager(config, signingKey, blacklist, refreshStore)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	cleanup := func() {
		mgr.Close()
	}

	return mgr, cleanup
}

func TestNewManager(t *testing.T) {
	t.Run("HS256 valid", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
		if mgr.GetConfig().Issuer != "jwtmgr" {
			t.Errorf("expected issuer 'jwtmgr', got '%s'", mgr.GetConfig().Issuer)
		}
	})

	t.Run("RS256 valid", func(t *testing.T) {
		mgr, cleanup := newRS256Manager(t)
		defer cleanup()

		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})

	t.Run("invalid algorithm", func(t *testing.T) {
		config := DefaultConfig()
		signingKey := SigningKey{
			Algorithm: "invalid",
		}
		_, err := NewManager(config, signingKey, nil, nil)
		if !errors.Is(err, ErrInvalidAlgorithm) {
			t.Errorf("expected ErrInvalidAlgorithm, got %v", err)
		}
	})

	t.Run("missing HMAC key", func(t *testing.T) {
		config := DefaultConfig()
		signingKey := SigningKey{
			Algorithm: HS256,
			HMACKey:   []byte{},
		}
		_, err := NewManager(config, signingKey, nil, nil)
		if !errors.Is(err, ErrMissingKey) {
			t.Errorf("expected ErrMissingKey, got %v", err)
		}
	})

	t.Run("missing RSA private key", func(t *testing.T) {
		config := DefaultConfig()
		signingKey := SigningKey{
			Algorithm: RS256,
		}
		_, err := NewManager(config, signingKey, nil, nil)
		if !errors.Is(err, ErrMissingKey) {
			t.Errorf("expected ErrMissingKey, got %v", err)
		}
	})

	t.Run("nil blacklist and refresh store", func(t *testing.T) {
		config := DefaultConfig()
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer mgr.Close()
		if mgr == nil {
			t.Fatal("expected non-nil manager")
		}
	})
}

func TestIssueToken(t *testing.T) {
	tests := []struct {
		name      string
		claims    *Claims
		wantError bool
	}{
		{
			name: "valid claims",
			claims: &Claims{
				Subject: "user123",
				Custom: map[string]interface{}{
					"role": "admin",
				},
			},
			wantError: false,
		},
		{
			name:      "nil claims",
			claims:    nil,
			wantError: true,
		},
		{
			name:      "empty claims",
			claims:    &Claims{},
			wantError: false,
		},
	}

	for _, alg := range []struct {
		name    string
		factory func(*testing.T) (*Manager, func())
	}{
		{"HS256", newHS256Manager},
		{"RS256", newRS256Manager},
	} {
		t.Run(alg.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					mgr, cleanup := alg.factory(t)
					defer cleanup()

					token, err := mgr.IssueToken(tt.claims)
					if tt.wantError {
						if err == nil {
							t.Error("expected error, got nil")
						}
						return
					}
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}
					if token == "" {
						t.Error("expected non-empty token")
					}
					parts := strings.Split(token, ".")
					if len(parts) != 3 {
						t.Errorf("expected 3 parts, got %d", len(parts))
					}
				})
			}
		})
	}
}

func TestTokenFormatCompliance(t *testing.T) {
	mgr, cleanup := newHS256Manager(t)
	defer cleanup()

	claims := &Claims{
		Subject:  "user123",
		Issuer:   "test-issuer",
		Audience: []string{"api1", "api2"},
		Custom: map[string]interface{}{
			"role": "user",
			"age":  30,
		},
	}

	token, err := mgr.IssueToken(claims)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT must have 3 parts, got %d", len(parts))
	}

	opts := DefaultValidationOptions()
	opts.ExpectedIssuer = "test-issuer"
	opts.ExpectedAudience = []string{"api1"}

	parsedClaims, err := mgr.ValidateToken(token, opts)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if parsedClaims.Subject != "user123" {
		t.Errorf("expected subject 'user123', got '%s'", parsedClaims.Subject)
	}
	if parsedClaims.Issuer != "test-issuer" {
		t.Errorf("expected issuer 'test-issuer', got '%s'", parsedClaims.Issuer)
	}
	if len(parsedClaims.Audience) != 2 {
		t.Errorf("expected 2 audiences, got %d", len(parsedClaims.Audience))
	}
	if parsedClaims.Custom["role"] != "user" {
		t.Errorf("expected custom claim 'role'='user', got '%v'", parsedClaims.Custom["role"])
	}
	if parsedClaims.Custom["age"] != float64(30) {
		t.Errorf("expected custom claim 'age'=30, got '%v'", parsedClaims.Custom["age"])
	}
	if parsedClaims.ID == "" {
		t.Error("expected non-empty jti")
	}
	if parsedClaims.IssuedAt.IsZero() {
		t.Error("expected non-zero issued at")
	}
	if parsedClaims.ExpiresAt.IsZero() {
		t.Error("expected non-zero expires at")
	}
}

func TestValidateToken(t *testing.T) {
	t.Run("HS256", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject: "user123",
			Issuer:  "jwtmgr",
		}

		token, err := mgr.IssueToken(claims)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		parsed, err := mgr.ValidateToken(token, opts)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
		if parsed.Subject != "user123" {
			t.Errorf("expected subject 'user123', got '%s'", parsed.Subject)
		}
	})

	t.Run("RS256", func(t *testing.T) {
		mgr, cleanup := newRS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject: "user456",
			Issuer:  "jwtmgr",
		}

		token, err := mgr.IssueToken(claims)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		parsed, err := mgr.ValidateToken(token, opts)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
		if parsed.Subject != "user456" {
			t.Errorf("expected subject 'user456', got '%s'", parsed.Subject)
		}
	})

	t.Run("RS256 with nil public key", func(t *testing.T) {
		config := DefaultConfig()
		privateKey, _ := generateRSAKeys(t)
		signingKey := SigningKey{
			Algorithm:  RS256,
			PrivateKey: privateKey,
			PublicKey:  nil,
		}
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{
			Subject: "user789",
			Issuer:  "jwtmgr",
		}

		token, err := mgr.IssueToken(claims)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		_, err = mgr.ValidateToken(token, opts)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken("", opts)
		if !errors.Is(err, ErrEmptyToken) {
			t.Errorf("expected ErrEmptyToken, got %v", err)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken("invalid.token", opts)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("invalid base64 header", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken("!!!.abc.def", opts)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("invalid base64 signature", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.e30.!!!", opts)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("invalid JSON header", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken("YWJjZA.e30.def", opts)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("algorithm mismatch", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		rs256Mgr, rs256Cleanup := newRS256Manager(t)
		defer rs256Cleanup()

		claims := &Claims{Subject: "user123"}
		token, _ := rs256Mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrInvalidAlgorithm) {
			t.Errorf("expected ErrInvalidAlgorithm, got %v", err)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{Subject: "user123"}
		token, _ := mgr.IssueToken(claims)

		parts := strings.Split(token, ".")
		parts[2] = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
		tamperedToken := strings.Join(parts, ".")

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken(tamperedToken, opts)
		if !errors.Is(err, ErrInvalidSignature) {
			t.Errorf("expected ErrInvalidSignature, got %v", err)
		}
	})

	t.Run("invalid claims JSON", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		header := base64URLEncode([]byte(`{"alg":"HS256","typ":"JWT"}`))
		badClaims := base64URLEncode([]byte("not-json"))
		signingInput := header + "." + badClaims
		signature := signHS256([]byte(signingInput), []byte("test-secret-key-1234567890"))
		token := signingInput + "." + base64URLEncode(signature)

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		config := DefaultConfig()
		config.AccessTokenTTL = 10 * time.Millisecond
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		token, _ := mgr.IssueToken(claims)

		time.Sleep(50 * time.Millisecond)

		opts := DefaultValidationOptions()
		_, err = mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrExpiredToken) {
			t.Errorf("expected ErrExpiredToken, got %v", err)
		}
	})

	t.Run("not yet valid", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject:   "user123",
			NotBefore: time.Now().Add(time.Hour),
		}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		_, err := mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrNotYetValid) {
			t.Errorf("expected ErrNotYetValid, got %v", err)
		}
	})

	t.Run("invalid issuer", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject: "user123",
			Issuer:  "wrong-issuer",
		}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		_, err := mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrInvalidIssuer) {
			t.Errorf("expected ErrInvalidIssuer, got %v", err)
		}
	})

	t.Run("invalid audience", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject:  "user123",
			Audience: []string{"other-api"},
		}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		opts.ExpectedAudience = []string{"api"}
		_, err := mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrInvalidAudience) {
			t.Errorf("expected ErrInvalidAudience, got %v", err)
		}
	})

	t.Run("single audience string", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject:  "user123",
			Audience: []string{"api"},
		}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		opts.ExpectedAudience = []string{"api"}
		parsed, err := mgr.ValidateToken(token, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(parsed.Audience) != 1 || parsed.Audience[0] != "api" {
			t.Errorf("expected audience ['api'], got %v", parsed.Audience)
		}
	})

	t.Run("skip validation", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject: "user123",
			Issuer:  "wrong-issuer",
		}
		token, _ := mgr.IssueToken(claims)

		opts := ValidationOptions{
			ValidateExpiry:   false,
			ValidateIssuer:   false,
			ValidateAudience: false,
			ValidateNotBefore: false,
		}
		_, err := mgr.ValidateToken(token, opts)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestRevokeToken(t *testing.T) {
	mgr, cleanup := newHS256Manager(t)
	defer cleanup()

	claims := &Claims{Subject: "user123"}
	token, err := mgr.IssueToken(claims)
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}

	opts := DefaultValidationOptions()
	opts.ExpectedIssuer = "jwtmgr"
	opts.ExpectedAudience = []string{"api"}

	parsed, err := mgr.ValidateToken(token, opts)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	err = mgr.RevokeToken(parsed.ID)
	if err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	_, err = mgr.ValidateToken(token, opts)
	if !errors.Is(err, ErrTokenBlacklisted) {
		t.Errorf("expected ErrTokenBlacklisted, got %v", err)
	}
}

func TestBlacklist(t *testing.T) {
	t.Run("add and contains", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		defer bl.Close()

		err := bl.Add("token1", time.Hour)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		exists, err := bl.Contains("token1")
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if !exists {
			t.Error("expected token1 to exist")
		}

		exists, err = bl.Contains("token2")
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if exists {
			t.Error("expected token2 to not exist")
		}
	})

	t.Run("add empty token id", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		defer bl.Close()

		err := bl.Add("", time.Hour)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("contains empty token id", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		defer bl.Close()

		exists, err := bl.Contains("")
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if exists {
			t.Error("expected empty token id to not exist")
		}
	})

	t.Run("remove", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		defer bl.Close()

		bl.Add("token1", time.Hour)
		err := bl.Remove("token1")
		if err != nil {
			t.Fatalf("Remove failed: %v", err)
		}

		exists, _ := bl.Contains("token1")
		if exists {
			t.Error("expected token1 to be removed")
		}
	})

	t.Run("TTL expiration", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		defer bl.Close()

		err := bl.Add("token1", 10*time.Millisecond)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		exists, err := bl.Contains("token1")
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if exists {
			t.Error("expected token1 to be expired")
		}
	})

	t.Run("cleanup loop", func(t *testing.T) {
		bl := NewMemoryBlacklist(10 * time.Millisecond)
		defer bl.Close()

		bl.Add("token1", 5*time.Millisecond)
		bl.Add("token2", time.Hour)

		if bl.Size() != 2 {
			t.Errorf("expected size 2, got %d", bl.Size())
		}

		time.Sleep(100 * time.Millisecond)

		if bl.Size() != 1 {
			t.Errorf("expected size 1 after cleanup, got %d", bl.Size())
		}
	})

	t.Run("close", func(t *testing.T) {
		bl := NewMemoryBlacklist(10 * time.Millisecond)
		err := bl.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}

		err = bl.Add("token1", time.Hour)
		if err != nil {
			t.Errorf("expected no error adding to closed blacklist, got %v", err)
		}
	})

	t.Run("double close", func(t *testing.T) {
		bl := NewMemoryBlacklist(0)
		err := bl.Close()
		if err != nil {
			t.Fatalf("first Close failed: %v", err)
		}
		err = bl.Close()
		if err != nil {
			t.Fatalf("second Close failed: %v", err)
		}
	})
}

func TestRenewToken(t *testing.T) {
	t.Run("normal renewal", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{
			Subject: "user123",
			Custom: map[string]interface{}{
				"role": "admin",
			},
		}
		token, err := mgr.IssueToken(claims)
		if err != nil {
			t.Fatalf("IssueToken failed: %v", err)
		}

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		newToken, err := mgr.RenewToken(token, opts)
		if err != nil {
			t.Fatalf("RenewToken failed: %v", err)
		}
		if newToken == "" {
			t.Error("expected non-empty new token")
		}
		if newToken == token {
			t.Error("expected new token to be different")
		}

		parsed, err := mgr.ValidateToken(newToken, opts)
		if err != nil {
			t.Fatalf("ValidateToken failed: %v", err)
		}
		if parsed.Subject != "user123" {
			t.Errorf("expected subject 'user123', got '%s'", parsed.Subject)
		}
		if parsed.Custom["role"] != "admin" {
			t.Errorf("expected custom claim preserved")
		}
	})

	t.Run("renewal window expired", func(t *testing.T) {
		config := DefaultConfig()
		config.AccessTokenTTL = 10 * time.Millisecond
		config.RenewalWindow = 10 * time.Millisecond
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		token, _ := mgr.IssueToken(claims)

		time.Sleep(100 * time.Millisecond)

		opts := DefaultValidationOptions()
		_, err = mgr.RenewToken(token, opts)
		if !errors.Is(err, ErrRenewalWindowExpired) {
			t.Errorf("expected ErrRenewalWindowExpired, got %v", err)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		opts := DefaultValidationOptions()
		_, err := mgr.RenewToken("invalid.token", opts)
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("auto blacklist old", func(t *testing.T) {
		config := DefaultConfig()
		config.AutoBlacklistOld = true
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		_, err = mgr.RenewToken(token, opts)
		if err != nil {
			t.Fatalf("RenewToken failed: %v", err)
		}

		_, err = mgr.ValidateToken(token, opts)
		if !errors.Is(err, ErrTokenBlacklisted) {
			t.Errorf("expected old token to be blacklisted, got %v", err)
		}
	})

	t.Run("no auto blacklist", func(t *testing.T) {
		config := DefaultConfig()
		config.AutoBlacklistOld = false
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		token, _ := mgr.IssueToken(claims)

		opts := DefaultValidationOptions()
		opts.ExpectedIssuer = "jwtmgr"
		opts.ExpectedAudience = []string{"api"}

		_, err = mgr.RenewToken(token, opts)
		if err != nil {
			t.Fatalf("RenewToken failed: %v", err)
		}

		_, err = mgr.ValidateToken(token, opts)
		if err != nil {
			t.Errorf("expected old token to still be valid, got %v", err)
		}
	})
}

func TestIssueTokenPair(t *testing.T) {
	t.Run("HS256", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		claims := &Claims{Subject: "user123"}
		pair, err := mgr.IssueTokenPair(claims)
		if err != nil {
			t.Fatalf("IssueTokenPair failed: %v", err)
		}
		if pair.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if pair.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
		if pair.TokenID == "" {
			t.Error("expected non-empty token ID")
		}
		if pair.ExpiresAt.IsZero() {
			t.Error("expected non-zero expires at")
		}
	})

	t.Run("RS256", func(t *testing.T) {
		mgr, cleanup := newRS256Manager(t)
		defer cleanup()

		claims := &Claims{Subject: "user456"}
		pair, err := mgr.IssueTokenPair(claims)
		if err != nil {
			t.Fatalf("IssueTokenPair failed: %v", err)
		}
		if pair.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if pair.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
	})

	t.Run("nil claims", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		_, err := mgr.IssueTokenPair(nil)
		if !errors.Is(err, ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})
}

func TestRefreshAccessToken(t *testing.T) {
	t.Run("with rotation", func(t *testing.T) {
		config := DefaultConfig()
		config.RefreshTokenRotation = true
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{
			Subject: "user123",
			Custom: map[string]interface{}{
				"role": "user",
			},
		}
		pair, err := mgr.IssueTokenPair(claims)
		if err != nil {
			t.Fatalf("IssueTokenPair failed: %v", err)
		}

		newPair, err := mgr.RefreshAccessToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshAccessToken failed: %v", err)
		}
		if newPair.AccessToken == "" {
			t.Error("expected non-empty access token")
		}
		if newPair.RefreshToken == "" {
			t.Error("expected non-empty refresh token")
		}
		if newPair.RefreshToken == pair.RefreshToken {
			t.Error("expected new refresh token to be different")
		}

		_, err = mgr.RefreshAccessToken(pair.RefreshToken)
		if !errors.Is(err, ErrRefreshTokenRevoked) {
			t.Errorf("expected old refresh token to be revoked, got %v", err)
		}
	})

	t.Run("without rotation", func(t *testing.T) {
		config := DefaultConfig()
		config.RefreshTokenRotation = false
		signingKey := NewHS256Config([]byte("test-key"))
		mgr, err := NewManager(config, signingKey, nil, nil)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		pair, err := mgr.IssueTokenPair(claims)
		if err != nil {
			t.Fatalf("IssueTokenPair failed: %v", err)
		}

		newPair, err := mgr.RefreshAccessToken(pair.RefreshToken)
		if err != nil {
			t.Fatalf("RefreshAccessToken failed: %v", err)
		}
		if newPair.RefreshToken != pair.RefreshToken {
			t.Error("expected refresh token to remain the same")
		}
	})

	t.Run("empty refresh token", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		_, err := mgr.RefreshAccessToken("")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("invalid refresh token", func(t *testing.T) {
		mgr, cleanup := newHS256Manager(t)
		defer cleanup()

		_, err := mgr.RefreshAccessToken("invalid-token")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("expired refresh token", func(t *testing.T) {
		config := DefaultConfig()
		config.RefreshTokenTTL = 10 * time.Millisecond
		signingKey := NewHS256Config([]byte("test-key"))
		refreshStore := NewMemoryRefreshStore()
		mgr, err := NewManager(config, signingKey, nil, refreshStore)
		if err != nil {
			t.Fatalf("NewManager failed: %v", err)
		}
		defer mgr.Close()

		claims := &Claims{Subject: "user123"}
		pair, err := mgr.IssueTokenPair(claims)
		if err != nil {
			t.Fatalf("IssueTokenPair failed: %v", err)
		}

		time.Sleep(50 * time.Millisecond)

		_, err = mgr.RefreshAccessToken(pair.RefreshToken)
		if !errors.Is(err, ErrRefreshTokenExpired) {
			t.Errorf("expected ErrRefreshTokenExpired, got %v", err)
		}
	})
}

func TestRefreshTokenStore(t *testing.T) {
	t.Run("save and get", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		rt := &RefreshTokenInfo{
			Token:     "token123",
			TokenID:   "jti123",
			Subject:   "user123",
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}

		err := store.Save(rt)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		got, err := store.Get("token123")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.Token != "token123" {
			t.Errorf("expected token 'token123', got '%s'", got.Token)
		}
		if got.Subject != "user123" {
			t.Errorf("expected subject 'user123', got '%s'", got.Subject)
		}
	})

	t.Run("save nil", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		err := store.Save(nil)
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("save empty token", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		rt := &RefreshTokenInfo{Token: ""}
		err := store.Save(rt)
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("get empty token", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		_, err := store.Get("")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("get not found", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		_, err := store.Get("nonexistent")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("revoke", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		rt := &RefreshTokenInfo{
			Token:     "token123",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		store.Save(rt)

		err := store.Revoke("token123")
		if err != nil {
			t.Fatalf("Revoke failed: %v", err)
		}

		_, err = store.Get("token123")
		if !errors.Is(err, ErrRefreshTokenRevoked) {
			t.Errorf("expected ErrRefreshTokenRevoked, got %v", err)
		}
	})

	t.Run("revoke empty token", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		err := store.Revoke("")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("revoke not found", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		err := store.Revoke("nonexistent")
		if !errors.Is(err, ErrInvalidRefreshToken) {
			t.Errorf("expected ErrInvalidRefreshToken, got %v", err)
		}
	})

	t.Run("size", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		defer store.Close()

		if store.Size() != 0 {
			t.Errorf("expected size 0, got %d", store.Size())
		}

		store.Save(&RefreshTokenInfo{
			Token:     "token1",
			ExpiresAt: time.Now().Add(time.Hour),
		})
		store.Save(&RefreshTokenInfo{
			Token:     "token2",
			ExpiresAt: time.Now().Add(time.Hour),
		})

		if store.Size() != 2 {
			t.Errorf("expected size 2, got %d", store.Size())
		}
	})

	t.Run("close", func(t *testing.T) {
		store := NewMemoryRefreshStore()
		err := store.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
	})
}

func TestClaimsMarshalUnmarshal(t *testing.T) {
	claims := &Claims{
		Issuer:    "test-issuer",
		Subject:   "user123",
		Audience:  []string{"api1", "api2"},
		ExpiresAt: time.Unix(1700000000, 0),
		NotBefore: time.Unix(1600000000, 0),
		IssuedAt:  time.Unix(1650000000, 0),
		ID:        "jti123",
		Custom: map[string]interface{}{
			"role": "admin",
			"age":  float64(30),
		},
	}

	data, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed Claims
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Issuer != claims.Issuer {
		t.Errorf("issuer mismatch: %s != %s", parsed.Issuer, claims.Issuer)
	}
	if parsed.Subject != claims.Subject {
		t.Errorf("subject mismatch: %s != %s", parsed.Subject, claims.Subject)
	}
	if len(parsed.Audience) != len(claims.Audience) {
		t.Errorf("audience length mismatch")
	}
	if !parsed.ExpiresAt.Equal(claims.ExpiresAt) {
		t.Errorf("expires at mismatch")
	}
	if !parsed.NotBefore.Equal(claims.NotBefore) {
		t.Errorf("not before mismatch")
	}
	if !parsed.IssuedAt.Equal(claims.IssuedAt) {
		t.Errorf("issued at mismatch")
	}
	if parsed.ID != claims.ID {
		t.Errorf("jti mismatch: %s != %s", parsed.ID, claims.ID)
	}
	if parsed.Custom["role"] != "admin" {
		t.Errorf("custom role mismatch")
	}
	if parsed.Custom["age"] != float64(30) {
		t.Errorf("custom age mismatch")
	}
}

func TestClaimsSingleAudience(t *testing.T) {
	claims := &Claims{
		Audience: []string{"api"},
	}

	data, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m map[string]interface{}
	err = json.Unmarshal(data, &m)
	if err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if aud, ok := m["aud"].(string); !ok || aud != "api" {
		t.Errorf("expected single audience string, got %v", m["aud"])
	}
}

func TestClaimsUnmarshalInvalidJSON(t *testing.T) {
	var claims Claims
	err := json.Unmarshal([]byte("not-json"), &claims)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDefaultValidationOptions(t *testing.T) {
	opts := DefaultValidationOptions()
	if !opts.ValidateExpiry {
		t.Error("expected ValidateExpiry to be true")
	}
	if !opts.ValidateIssuer {
		t.Error("expected ValidateIssuer to be true")
	}
	if !opts.ValidateAudience {
		t.Error("expected ValidateAudience to be true")
	}
	if !opts.ValidateNotBefore {
		t.Error("expected ValidateNotBefore to be true")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.Issuer != "jwtmgr" {
		t.Errorf("expected issuer 'jwtmgr', got '%s'", config.Issuer)
	}
	if len(config.Audience) != 1 || config.Audience[0] != "api" {
		t.Errorf("expected audience ['api'], got %v", config.Audience)
	}
	if config.AccessTokenTTL != time.Hour {
		t.Errorf("expected AccessTokenTTL 1h, got %v", config.AccessTokenTTL)
	}
	if config.RefreshTokenTTL != 7*24*time.Hour {
		t.Errorf("expected RefreshTokenTTL 7d, got %v", config.RefreshTokenTTL)
	}
	if config.RenewalWindow != 5*time.Minute {
		t.Errorf("expected RenewalWindow 5m, got %v", config.RenewalWindow)
	}
	if !config.AutoBlacklistOld {
		t.Error("expected AutoBlacklistOld to be true")
	}
	if !config.RefreshTokenRotation {
		t.Error("expected RefreshTokenRotation to be true")
	}
}

func TestAudienceContains(t *testing.T) {
	tests := []struct {
		name     string
		tokenAud []string
		expected []string
		want     bool
	}{
		{
			name:     "single match",
			tokenAud: []string{"api"},
			expected: []string{"api"},
			want:     true,
		},
		{
			name:     "multiple expected all present",
			tokenAud: []string{"api1", "api2", "api3"},
			expected: []string{"api1", "api2"},
			want:     true,
		},
		{
			name:     "expected not present",
			tokenAud: []string{"api1"},
			expected: []string{"api2"},
			want:     false,
		},
		{
			name:     "one expected missing",
			tokenAud: []string{"api1", "api2"},
			expected: []string{"api1", "api3"},
			want:     false,
		},
		{
			name:     "empty token audience",
			tokenAud: []string{},
			expected: []string{"api"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := audienceContains(tt.tokenAud, tt.expected)
			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr, cleanup := newHS256Manager(t)
	defer cleanup()

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			claims := &Claims{
				Subject:  "user123",
				Audience: []string{"api"},
				Custom: map[string]interface{}{
					"user_id": id,
				},
			}

			token, err := mgr.IssueToken(claims)
			if err != nil {
				t.Errorf("IssueToken failed: %v", err)
				return
			}

			opts := DefaultValidationOptions()
			opts.ExpectedIssuer = "jwtmgr"
			opts.ExpectedAudience = []string{"api"}

			parsed, err := mgr.ValidateToken(token, opts)
			if err != nil {
				t.Errorf("ValidateToken failed: %v", err)
				return
			}

			if parsed.Custom["user_id"] != float64(id) {
				t.Errorf("expected user_id %d, got %v", id, parsed.Custom["user_id"])
			}

			_, err = mgr.RenewToken(token, opts)
			if err != nil {
				t.Errorf("RenewToken failed: %v", err)
				return
			}
		}(i)
	}

	wg.Wait()
}

func TestFullWorkflow(t *testing.T) {
	mgr, cleanup := newHS256Manager(t)
	defer cleanup()

	claims := &Claims{
		Subject:  "john.doe@example.com",
		Audience: []string{"api", "web"},
		Custom: map[string]interface{}{
			"role":  "admin",
			"name":  "John Doe",
			"email": "john.doe@example.com",
		},
	}

	pair, err := mgr.IssueTokenPair(claims)
	if err != nil {
		t.Fatalf("IssueTokenPair failed: %v", err)
	}

	opts := DefaultValidationOptions()
	opts.ExpectedIssuer = "jwtmgr"
	opts.ExpectedAudience = []string{"api"}

	parsed, err := mgr.ValidateToken(pair.AccessToken, opts)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if parsed.Subject != "john.doe@example.com" {
		t.Errorf("expected subject 'john.doe@example.com', got '%s'", parsed.Subject)
	}
	if parsed.Custom["role"] != "admin" {
		t.Errorf("expected role 'admin', got '%v'", parsed.Custom["role"])
	}

	renewedToken, err := mgr.RenewToken(pair.AccessToken, opts)
	if err != nil {
		t.Fatalf("RenewToken failed: %v", err)
	}

	parsedRenewed, err := mgr.ValidateToken(renewedToken, opts)
	if err != nil {
		t.Fatalf("ValidateToken renewed failed: %v", err)
	}
	if parsedRenewed.Subject != "john.doe@example.com" {
		t.Errorf("expected subject preserved after renewal")
	}
	if parsedRenewed.Custom["role"] != "admin" {
		t.Errorf("expected custom claims preserved after renewal")
	}

	refreshedPair, err := mgr.RefreshAccessToken(pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken failed: %v", err)
	}

	parsedRefreshed, err := mgr.ValidateToken(refreshedPair.AccessToken, opts)
	if err != nil {
		t.Fatalf("ValidateToken refreshed failed: %v", err)
	}
	if parsedRefreshed.Subject != "john.doe@example.com" {
		t.Errorf("expected subject preserved after refresh")
	}

	err = mgr.RevokeToken(parsedRefreshed.ID)
	if err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	_, err = mgr.ValidateToken(refreshedPair.AccessToken, opts)
	if !errors.Is(err, ErrTokenBlacklisted) {
		t.Errorf("expected token to be blacklisted")
	}
}

func TestHelperFunctions(t *testing.T) {
	t.Run("base64URL encode/decode", func(t *testing.T) {
		data := []byte("test data 123 !@#$%^&*()")
		encoded := base64URLEncode(data)
		decoded, err := base64URLDecode(encoded)
		if err != nil {
			t.Fatalf("base64URLDecode failed: %v", err)
		}
		if string(decoded) != string(data) {
			t.Errorf("data mismatch: %s != %s", string(decoded), string(data))
		}
	})

	t.Run("base64URL decode invalid", func(t *testing.T) {
		_, err := base64URLDecode("!!!invalid!!!")
		if err == nil {
			t.Error("expected error for invalid base64")
		}
	})

	t.Run("generateRandomString", func(t *testing.T) {
		s1 := generateRandomString(16)
		s2 := generateRandomString(16)
		if s1 == s2 {
			t.Error("expected different random strings")
		}
		if len(s1) != 32 {
			t.Errorf("expected length 32, got %d", len(s1))
		}
	})

	t.Run("HS256 sign/verify", func(t *testing.T) {
		key := []byte("test-key")
		data := []byte("test data")
		signature := signHS256(data, key)
		if !verifyHS256(data, signature, key) {
			t.Error("expected signature to verify")
		}
		if verifyHS256(data, signature, []byte("wrong-key")) {
			t.Error("expected signature to fail with wrong key")
		}
	})

	t.Run("RS256 sign/verify", func(t *testing.T) {
		privateKey, publicKey := generateRSAKeys(t)
		data := []byte("test data")
		signature, err := signRS256(data, privateKey)
		if err != nil {
			t.Fatalf("signRS256 failed: %v", err)
		}
		if !verifyRS256(data, signature, publicKey) {
			t.Error("expected signature to verify")
		}

		_, wrongPublicKey := generateRSAKeys(t)
		if verifyRS256(data, signature, wrongPublicKey) {
			t.Error("expected signature to fail with wrong key")
		}
	})
}
