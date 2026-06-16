package csrf

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewCSRFWithConfig_InvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name: "TokenLength too short",
			cfg: Config{
				TokenLength:   8,
				CookieName:    "XSRF-TOKEN",
				HeaderName:    "X-CSRF-Token",
				FormFieldName: "csrf_token",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "Negative TokenTTL",
			cfg: Config{
				TokenLength:   32,
				TokenTTL:      -1 * time.Hour,
				CookieName:    "XSRF-TOKEN",
				HeaderName:    "X-CSRF-Token",
				FormFieldName: "csrf_token",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "Empty CookieName",
			cfg: Config{
				TokenLength:   32,
				CookieName:    "",
				HeaderName:    "X-CSRF-Token",
				FormFieldName: "csrf_token",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "Empty HeaderName",
			cfg: Config{
				TokenLength:   32,
				CookieName:    "XSRF-TOKEN",
				HeaderName:    "",
				FormFieldName: "csrf_token",
			},
			wantErr: ErrInvalidConfig,
		},
		{
			name: "Empty FormFieldName",
			cfg: Config{
				TokenLength:   32,
				CookieName:    "XSRF-TOKEN",
				HeaderName:    "X-CSRF-Token",
				FormFieldName: "",
			},
			wantErr: ErrInvalidConfig,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCSRFWithConfig(tt.cfg)
			if err != tt.wantErr {
				t.Errorf("NewCSRFWithConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewCSRFWithConfig_DefaultProtectedMethods(t *testing.T) {
	cfg := Config{
		TokenLength:   32,
		CookieName:    "XSRF-TOKEN",
		HeaderName:    "X-CSRF-Token",
		FormFieldName: "csrf_token",
	}
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewCSRFWithConfig() unexpected error: %v", err)
	}
	if len(c.cfg.ProtectedMethods) == 0 {
		t.Error("Expected default protected methods to be set")
	}
}

func TestNewCSRF(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("NewCSRF() panicked: %v", r)
		}
	}()
	c := NewCSRF()
	if c == nil {
		t.Fatal("NewCSRF() returned nil")
	}
	if c.Mode() != SynchronizerTokenMode {
		t.Errorf("Default mode = %v, want %v", c.Mode(), SynchronizerTokenMode)
	}
}

func TestGenerateToken(t *testing.T) {
	c := NewCSRF()

	t.Run("empty session id", func(t *testing.T) {
		_, err := c.GenerateToken("")
		if err != ErrSessionNotFound {
			t.Errorf("GenerateToken(\"\") error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("valid session", func(t *testing.T) {
		token, err := c.GenerateToken("session-123")
		if err != nil {
			t.Fatalf("GenerateToken() unexpected error: %v", err)
		}
		if token == "" {
			t.Error("GenerateToken() returned empty token")
		}
		if len(token) != 64 {
			t.Errorf("Token length = %d, want 64 (32 bytes hex encoded)", len(token))
		}
	})

	t.Run("regenerate replaces old token", func(t *testing.T) {
		sessionID := "session-regen"
		token1, err := c.GenerateToken(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		token2, err := c.GenerateToken(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if token1 == token2 {
			t.Error("RegenerateToken should produce different token")
		}
		err = c.ValidateToken(token1, sessionID)
		if err == nil {
			t.Error("Old token should be invalid after regeneration")
		}
	})
}

func TestGetToken(t *testing.T) {
	c := NewCSRF()

	t.Run("empty session id", func(t *testing.T) {
		_, err := c.GetToken("")
		if err != ErrSessionNotFound {
			t.Errorf("GetToken(\"\") error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("non-existent session", func(t *testing.T) {
		_, err := c.GetToken("nonexistent")
		if err != ErrTokenNotFound {
			t.Errorf("GetToken(nonexistent) error = %v, want %v", err, ErrTokenNotFound)
		}
	})

	t.Run("valid session", func(t *testing.T) {
		sessionID := "session-get"
		generated, err := c.GenerateToken(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		retrieved, err := c.GetToken(sessionID)
		if err != nil {
			t.Fatalf("GetToken() unexpected error: %v", err)
		}
		if retrieved != generated {
			t.Errorf("GetToken() = %v, want %v", retrieved, generated)
		}
	})
}

func TestValidateToken(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-validate"
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty token", func(t *testing.T) {
		err := c.ValidateToken("", sessionID)
		if err != ErrTokenNotFound {
			t.Errorf("ValidateToken(\"\") error = %v, want %v", err, ErrTokenNotFound)
		}
	})

	t.Run("empty session id", func(t *testing.T) {
		err := c.ValidateToken(token, "")
		if err != ErrSessionNotFound {
			t.Errorf("ValidateToken(token, \"\") error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		err := c.ValidateToken("invalid-token", sessionID)
		if err != ErrTokenInvalid {
			t.Errorf("ValidateToken(invalid) error = %v, want %v", err, ErrTokenInvalid)
		}
	})

	t.Run("session mismatch", func(t *testing.T) {
		err := c.ValidateToken(token, "different-session")
		if err != ErrSessionMismatch {
			t.Errorf("ValidateToken(session mismatch) error = %v, want %v", err, ErrSessionMismatch)
		}
	})

	t.Run("valid token and session", func(t *testing.T) {
		err := c.ValidateToken(token, sessionID)
		if err != nil {
			t.Errorf("ValidateToken(valid) unexpected error: %v", err)
		}
	})
}

func TestValidateToken_Expired(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TokenTTL = 50 * time.Millisecond
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "session-expired"
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)

	err = c.ValidateToken(token, sessionID)
	if err != ErrTokenInvalid {
		t.Errorf("ValidateToken(expired) error = %v, want %v", err, ErrTokenInvalid)
	}

	_, err = c.GetToken(sessionID)
	if err != ErrTokenNotFound {
		t.Errorf("GetToken after expiry should return ErrTokenNotFound, got %v", err)
	}
}

func TestRotateToken(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-rotate"
	token1, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("rotate invalid token", func(t *testing.T) {
		_, err := c.RotateToken("invalid", sessionID)
		if err == nil {
			t.Error("RotateToken(invalid) should return error")
		}
	})

	t.Run("rotate with wrong session", func(t *testing.T) {
		_, err := c.RotateToken(token1, "wrong-session")
		if err != ErrSessionMismatch {
			t.Errorf("RotateToken(wrong session) error = %v, want %v", err, ErrSessionMismatch)
		}
	})

	t.Run("successful rotation", func(t *testing.T) {
		token2, err := c.RotateToken(token1, sessionID)
		if err != nil {
			t.Fatalf("RotateToken() unexpected error: %v", err)
		}
		if token2 == token1 {
			t.Error("RotateToken should produce new token")
		}

		err = c.ValidateToken(token1, sessionID)
		if err == nil {
			t.Error("Old token should be invalid after rotation")
		}

		err = c.ValidateToken(token2, sessionID)
		if err != nil {
			t.Errorf("New token should be valid after rotation, error: %v", err)
		}

		currentToken, err := c.GetToken(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if currentToken != token2 {
			t.Errorf("Current session token = %v, want %v", currentToken, token2)
		}
	})
}

func TestInvalidateSession(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-invalidate"
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty session id", func(t *testing.T) {
		err := c.InvalidateSession("")
		if err != ErrSessionNotFound {
			t.Errorf("InvalidateSession(\"\") error = %v, want %v", err, ErrSessionNotFound)
		}
	})

	t.Run("successful invalidation", func(t *testing.T) {
		err := c.InvalidateSession(sessionID)
		if err != nil {
			t.Fatalf("InvalidateSession() unexpected error: %v", err)
		}

		err = c.ValidateToken(token, sessionID)
		if err != ErrTokenInvalid {
			t.Errorf("ValidateToken after invalidation error = %v, want %v", err, ErrTokenInvalid)
		}

		_, err = c.GetToken(sessionID)
		if err != ErrTokenNotFound {
			t.Errorf("GetToken after invalidation error = %v, want %v", err, ErrTokenNotFound)
		}
	})

	t.Run("invalidate non-existent session", func(t *testing.T) {
		err := c.InvalidateSession("nonexistent")
		if err != nil {
			t.Errorf("InvalidateSession(nonexistent) should not error, got %v", err)
		}
	})
}

func TestInvalidateToken(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-invalidate-token"
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("empty token", func(t *testing.T) {
		err := c.InvalidateToken("")
		if err != ErrTokenNotFound {
			t.Errorf("InvalidateToken(\"\") error = %v, want %v", err, ErrTokenNotFound)
		}
	})

	t.Run("successful invalidation", func(t *testing.T) {
		err := c.InvalidateToken(token)
		if err != nil {
			t.Fatalf("InvalidateToken() unexpected error: %v", err)
		}

		err = c.ValidateToken(token, sessionID)
		if err != ErrTokenInvalid {
			t.Errorf("ValidateToken after invalidation error = %v, want %v", err, ErrTokenInvalid)
		}

		_, err = c.GetToken(sessionID)
		if err != ErrTokenNotFound {
			t.Errorf("GetToken after token invalidation error = %v, want %v", err, ErrTokenNotFound)
		}
	})
}

func TestCleanExpired(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TokenTTL = 50 * time.Millisecond
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sessions := []string{"s1", "s2", "s3"}
	for _, s := range sessions {
		_, err := c.GenerateToken(s)
		if err != nil {
			t.Fatal(err)
		}
	}

	if c.TokenCount() != 3 {
		t.Errorf("TokenCount before expiry = %d, want 3", c.TokenCount())
	}

	time.Sleep(100 * time.Millisecond)

	cleaned := c.CleanExpired()
	if cleaned != 3 {
		t.Errorf("CleanExpired() = %d, want 3", cleaned)
	}

	if c.TokenCount() != 0 {
		t.Errorf("TokenCount after cleanup = %d, want 0", c.TokenCount())
	}
	if c.SessionCount() != 0 {
		t.Errorf("SessionCount after cleanup = %d, want 0", c.SessionCount())
	}
}

func TestIsProtectedMethod(t *testing.T) {
	c := NewCSRF()

	tests := []struct {
		method string
		want   bool
	}{
		{http.MethodGet, false},
		{http.MethodHead, false},
		{http.MethodOptions, false},
		{http.MethodPost, true},
		{http.MethodPut, true},
		{http.MethodDelete, true},
		{http.MethodPatch, true},
		{"POST", true},
		{"post", true},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := c.isProtectedMethod(tt.method); got != tt.want {
				t.Errorf("isProtectedMethod(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

func TestIsTrustedOrigin(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TrustedOrigins = []string{
		"https://trusted.com",
		"trusted-subdomain.org",
		"http://192.168.1.1:8080",
	}
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{"empty origin", "", false},
		{"exact match trusted.com", "https://trusted.com", true},
		{"trusted.com with path", "https://trusted.com/path", true},
		{"wrong scheme trusted.com", "http://trusted.com", false},
		{"subdomain of trusted.com", "https://sub.trusted.com", false},
		{"trusted subdomain host only", "https://app.trusted-subdomain.org", true},
		{"exact subdomain host", "https://trusted-subdomain.org", true},
		{"ip with port", "http://192.168.1.1:8080", true},
		{"untrusted origin", "https://evil.com", false},
		{"invalid origin url", "://invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.isTrustedOrigin(tt.origin); got != tt.want {
				t.Errorf("isTrustedOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}

func TestIsSameOrigin(t *testing.T) {
	c := NewCSRF()

	tests := []struct {
		name   string
		origin string
		host   string
		want   bool
	}{
		{"empty origin", "", "example.com", false},
		{"empty host", "https://example.com", "", false},
		{"same host", "https://example.com", "example.com", true},
		{"same host with port", "https://example.com:8080", "example.com:8080", true},
		{"different port", "https://example.com:8080", "example.com", false},
		{"different host", "https://other.com", "example.com", false},
		{"invalid origin", "://bad", "example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := c.isSameOrigin(tt.origin, tt.host); got != tt.want {
				t.Errorf("isSameOrigin(%q, %q) = %v, want %v", tt.origin, tt.host, got, tt.want)
			}
		})
	}
}

func setupTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

func TestMiddleware_SynchronizerTokenMode_NormalFlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = false
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-sync"

	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("GET request passes through", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("GET status = %d, want %d", rr.Code, http.StatusOK)
		}
	})

	t.Run("POST with valid token in header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("POST status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("POST with valid token in form field", func(t *testing.T) {
		form := url.Values{}
		form.Set(cfg.FormFieldName, token)
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(form.Encode()))
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("POST form status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("POST without token returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST no token status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("POST with wrong session token returns 403", func(t *testing.T) {
		otherSession := "other-session"
		otherToken, _ := c.GenerateToken(otherSession)
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, otherToken)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST wrong session status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

func TestMiddleware_DoubleSubmitCookieMode_NormalFlow(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = DoubleSubmitCookieMode
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = false
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-double"
	token, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("POST with matching cookie and header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("POST status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("POST with mismatched tokens returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, "wrong-token")
		req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST mismatch status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("POST missing header returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST no header status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("POST missing cookie returns 403", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST no cookie status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

func TestMiddleware_OriginCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = true
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = false
	cfg.TrustedOrigins = []string{"https://allowed.com"}
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-origin"
	token, _ := c.GenerateToken(sessionID)

	t.Run("same origin allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Origin", "https://example.com")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Same origin status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("trusted origin allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Origin", "https://allowed.com")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Trusted origin status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("untrusted origin rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Origin", "https://evil.com")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("Untrusted origin status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("no origin header bypasses check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("No origin status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})
}

func TestMiddleware_RefererCheck(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = true
	cfg.EnableTokenRotation = false
	cfg.TrustedOrigins = []string{"https://allowed.com"}
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-referer"
	token, _ := c.GenerateToken(sessionID)

	t.Run("same referer allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Referer", "https://example.com/page")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Same referer status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("trusted referer allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Referer", "https://allowed.com/form")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("Trusted referer status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}
	})

	t.Run("untrusted referer rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/test", nil)
		req.Host = "example.com"
		req.Header.Set("Referer", "https://evil.com/malicious")
		req.Header.Set(cfg.SessionIDHeader, sessionID)
		req.Header.Set(cfg.HeaderName, token)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusForbidden {
			t.Errorf("Untrusted referer status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})
}

func TestMiddleware_TokenRotation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = true
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-rotation"
	token1, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(cfg.SessionIDHeader, sessionID)
	req.Header.Set(cfg.HeaderName, token1)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	newTokenHeader := rr.Header().Get(cfg.HeaderName)
	if newTokenHeader == "" {
		t.Fatal("Expected new token in response header after rotation")
	}
	if newTokenHeader == token1 {
		t.Error("New token should differ from old token")
	}

	err = c.ValidateToken(token1, sessionID)
	if err == nil {
		t.Error("Old token should be invalid after rotation")
	}

	currentToken, err := c.GetToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if currentToken != newTokenHeader {
		t.Errorf("Session token = %v, header token = %v", currentToken, newTokenHeader)
	}
}

func TestMiddleware_DoubleSubmitCookieMode_TokenRotation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = DoubleSubmitCookieMode
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = true
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "test-session-double-rot"
	token1, err := c.GenerateToken(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(cfg.SessionIDHeader, sessionID)
	req.Header.Set(cfg.HeaderName, token1)
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token1})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
	}

	newTokenHeader := rr.Header().Get(cfg.HeaderName)
	if newTokenHeader == "" {
		t.Fatal("Expected new token in response header")
	}

	cookies := rr.Result().Cookies()
	var cookieToken string
	for _, ck := range cookies {
		if ck.Name == cfg.CookieName {
			cookieToken = ck.Value
			break
		}
	}
	if cookieToken == "" {
		t.Fatal("Expected new token cookie in response")
	}
	if cookieToken != newTokenHeader {
		t.Errorf("Cookie token = %v, header token = %v", cookieToken, newTokenHeader)
	}
}

func TestMiddleware_CustomErrorHandler(t *testing.T) {
	customErrCalled := false
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		customErrCalled = true
		w.Header().Set("X-Custom-Error", "true")
		http.Error(w, "Custom Forbidden", http.StatusForbidden)
	}
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(cfg.SessionIDHeader, "sess")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !customErrCalled {
		t.Error("Custom error handler was not called")
	}
	if rr.Header().Get("X-Custom-Error") != "true" {
		t.Error("Custom error header not set")
	}
	if rr.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}

func TestSessionBinding_CrossSessionTokenReuse(t *testing.T) {
	c := NewCSRF()

	sessionA := "session-A"
	sessionB := "session-B"

	tokenA, _ := c.GenerateToken(sessionA)
	_, _ = c.GenerateToken(sessionB)

	err := c.ValidateToken(tokenA, sessionB)
	if err != ErrSessionMismatch {
		t.Errorf("Cross-session token reuse should return ErrSessionMismatch, got %v", err)
	}
}

func TestSessionInvalidation_AllTokensCleared(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-clear-all"

	token1, _ := c.GenerateToken(sessionID)
	token2, _ := c.GenerateToken(sessionID)
	_ = token1
	_ = token2

	err := c.InvalidateSession(sessionID)
	if err != nil {
		t.Fatal(err)
	}

	if c.TokenCount() != 0 {
		t.Errorf("TokenCount = %d, want 0 after session invalidation", c.TokenCount())
	}
	if c.SessionCount() != 0 {
		t.Errorf("SessionCount = %d, want 0 after session invalidation", c.SessionCount())
	}
}

func TestMiddleware_GET_DoubleSubmitCookieMode_GeneratesToken(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = DoubleSubmitCookieMode
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "session-get-token"

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(cfg.SessionIDHeader, sessionID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", rr.Code, http.StatusOK)
	}

	headerToken := rr.Header().Get(cfg.HeaderName)
	if headerToken == "" {
		t.Error("Expected token in header for GET request in DoubleSubmit mode")
	}

	cookies := rr.Result().Cookies()
	foundCookie := false
	for _, ck := range cookies {
		if ck.Name == cfg.CookieName && ck.Value == headerToken {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Error("Expected matching token cookie for GET request in DoubleSubmit mode")
	}
}

func TestGenerateHandler(t *testing.T) {
	c := NewCSRF()
	sessionID := "session-gen-handler"

	t.Run("without session returns 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/csrf/token", nil)
		rr := httptest.NewRecorder()
		c.GenerateHandler(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("with session returns token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/csrf/token", nil)
		req.Header.Set(c.cfg.SessionIDHeader, sessionID)
		rr := httptest.NewRecorder()
		c.GenerateHandler(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("Status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		tokenHeader := rr.Header().Get(c.cfg.HeaderName)
		if tokenHeader == "" {
			t.Error("Expected token in response header")
		}

		body := rr.Body.String()
		if !strings.Contains(body, tokenHeader) {
			t.Errorf("Response body should contain token. Body: %s, Token: %s", body, tokenHeader)
		}
	})
}

func TestGenerateHandler_DoubleSubmitMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = DoubleSubmitCookieMode
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "session-gen-double"
	req := httptest.NewRequest(http.MethodGet, "/csrf/token", nil)
	req.Header.Set(cfg.SessionIDHeader, sessionID)
	rr := httptest.NewRecorder()
	c.GenerateHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("Status = %d, want %d", rr.Code, http.StatusOK)
	}

	foundCookie := false
	for _, ck := range rr.Result().Cookies() {
		if ck.Name == cfg.CookieName {
			foundCookie = true
			break
		}
	}
	if !foundCookie {
		t.Error("Expected token cookie in DoubleSubmit mode")
	}
}

func TestConcurrentTokenOperations(t *testing.T) {
	c := NewCSRF()
	var wg sync.WaitGroup
	numGoroutines := 50
	sessionIDs := make([]string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		sessionIDs[i] = "session-concurrent-" + string(rune('A'+i%26))
	}

	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			_, _ = c.GenerateToken(sessionIDs[idx])
		}(i)

		go func(idx int) {
			defer wg.Done()
			_, _ = c.GetToken(sessionIDs[idx])
		}(i)

		go func(idx int) {
			defer wg.Done()
			token, err := c.GetToken(sessionIDs[idx])
			if err == nil {
				_ = c.ValidateToken(token, sessionIDs[idx])
			}
		}(i)
	}

	wg.Wait()

	if c.SessionCount() > numGoroutines {
		t.Errorf("SessionCount = %d, should not exceed %d", c.SessionCount(), numGoroutines)
	}
}

func TestModeAndConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Mode = DoubleSubmitCookieMode
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if c.Mode() != DoubleSubmitCookieMode {
		t.Errorf("Mode() = %v, want %v", c.Mode(), DoubleSubmitCookieMode)
	}

	retrievedCfg := c.Config()
	if retrievedCfg.Mode != DoubleSubmitCookieMode {
		t.Errorf("Config().Mode = %v, want %v", retrievedCfg.Mode, DoubleSubmitCookieMode)
	}
	if retrievedCfg.TokenLength != cfg.TokenLength {
		t.Errorf("Config().TokenLength = %d, want %d", retrievedCfg.TokenLength, cfg.TokenLength)
	}
}

func TestTokenUniqueness(t *testing.T) {
	c := NewCSRF()
	tokens := make(map[string]bool)
	numTokens := 1000

	for i := 0; i < numTokens; i++ {
		sessionID := "unique-session-" + string(rune(i))
		token, err := c.GenerateToken(sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if tokens[token] {
			t.Errorf("Duplicate token found: %s", token)
		}
		tokens[token] = true
	}

	if len(tokens) != numTokens {
		t.Errorf("Generated %d unique tokens from %d sessions", len(tokens), numTokens)
	}
}

func TestSessionIDFromCookie(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SessionIDCookie = "APP_SESSION"
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "cookie-session"
	token, _ := c.GenerateToken(sessionID)

	cfg2 := cfg
	cfg2.EnableOriginCheck = false
	cfg2.EnableRefererCheck = false
	cfg2.EnableTokenRotation = false
	c2, _ := NewCSRFWithConfig(cfg2)
	handler := c2.Middleware(setupTestHandler())

	_, _ = c2.GenerateToken(sessionID)
	currentToken, _ := c2.GetToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.AddCookie(&http.Cookie{Name: cfg.SessionIDCookie, Value: sessionID})
	req.Header.Set(cfg.HeaderName, currentToken)
	_ = token
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST with cookie session status = %d, want %d. Body: %s",
			rr.Code, http.StatusOK, rr.Body.String())
	}
}

func TestTokenFromCookie_SynchronizerMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableOriginCheck = false
	cfg.EnableRefererCheck = false
	cfg.EnableTokenRotation = false
	c, err := NewCSRFWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	handler := c.Middleware(setupTestHandler())
	sessionID := "session-cookie-token"
	token, _ := c.GenerateToken(sessionID)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	req.Header.Set(cfg.SessionIDHeader, sessionID)
	req.AddCookie(&http.Cookie{Name: cfg.CookieName, Value: token})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("POST with cookie token status = %d, want %d. Body: %s",
			rr.Code, http.StatusOK, rr.Body.String())
	}
}
