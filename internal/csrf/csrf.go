package csrf

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrTokenNotFound      = errors.New("csrf: token not found")
	ErrTokenMismatch      = errors.New("csrf: token mismatch")
	ErrTokenInvalid       = errors.New("csrf: token invalid")
	ErrOriginNotAllowed   = errors.New("csrf: origin not allowed")
	ErrRefererNotAllowed  = errors.New("csrf: referer not allowed")
	ErrSessionNotFound    = errors.New("csrf: session not found")
	ErrSessionMismatch    = errors.New("csrf: session mismatch")
	ErrInvalidConfig      = errors.New("csrf: invalid config")
	ErrMethodNotProtected = errors.New("csrf: method not in protected list")
)

type ProtectionMode int

const (
	SynchronizerTokenMode ProtectionMode = iota
	DoubleSubmitCookieMode
)

type Config struct {
	Mode              ProtectionMode
	TokenLength       int
	TokenTTL          time.Duration
	CookieName        string
	CookieDomain      string
	CookiePath        string
	CookieSecure      bool
	CookieHTTPOnly    bool
	CookieSameSite    http.SameSite
	HeaderName        string
	FormFieldName     string
	SessionIDHeader   string
	SessionIDCookie   string
	TrustedOrigins    []string
	ProtectedMethods  []string
	EnableOriginCheck bool
	EnableRefererCheck bool
	EnableTokenRotation bool
	ErrorHandler      func(w http.ResponseWriter, r *http.Request, err error)
}

func DefaultConfig() Config {
	return Config{
		Mode:               SynchronizerTokenMode,
		TokenLength:        32,
		TokenTTL:           24 * time.Hour,
		CookieName:         "XSRF-TOKEN",
		CookiePath:         "/",
		CookieSameSite:     http.SameSiteStrictMode,
		HeaderName:         "X-CSRF-Token",
		FormFieldName:      "csrf_token",
		SessionIDHeader:    "X-Session-ID",
		SessionIDCookie:    "SESSIONID",
		TrustedOrigins:     []string{},
		ProtectedMethods:   []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch},
		EnableOriginCheck:  true,
		EnableRefererCheck: true,
		EnableTokenRotation: true,
	}
}

type sessionToken struct {
	Token     string
	SessionID string
	ExpiresAt time.Time
}

type CSRF struct {
	cfg        Config
	mu         sync.RWMutex
	tokens     map[string]*sessionToken
	sessions   map[string]string
}

func NewCSRF() *CSRF {
	c, err := NewCSRFWithConfig(DefaultConfig())
	if err != nil {
		panic("csrf: DefaultConfig is invalid: " + err.Error())
	}
	return c
}

func NewCSRFWithConfig(cfg Config) (*CSRF, error) {
	if cfg.TokenLength < 16 {
		return nil, ErrInvalidConfig
	}
	if cfg.TokenTTL < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.CookieName == "" {
		return nil, ErrInvalidConfig
	}
	if cfg.HeaderName == "" {
		return nil, ErrInvalidConfig
	}
	if cfg.FormFieldName == "" {
		return nil, ErrInvalidConfig
	}
	if cfg.ProtectedMethods == nil || len(cfg.ProtectedMethods) == 0 {
		cfg.ProtectedMethods = []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch}
	}

	return &CSRF{
		cfg:      cfg,
		tokens:   make(map[string]*sessionToken),
		sessions: make(map[string]string),
	}, nil
}

func (c *CSRF) generateToken() (string, error) {
	b := make([]byte, c.cfg.TokenLength)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (c *CSRF) getSessionID(r *http.Request) string {
	if c.cfg.SessionIDHeader != "" {
		if sid := r.Header.Get(c.cfg.SessionIDHeader); sid != "" {
			return sid
		}
	}
	if c.cfg.SessionIDCookie != "" {
		if cookie, err := r.Cookie(c.cfg.SessionIDCookie); err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}
	return ""
}

func (c *CSRF) GenerateToken(sessionID string) (string, error) {
	if sessionID == "" {
		return "", ErrSessionNotFound
	}

	token, err := c.generateToken()
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if oldToken, exists := c.sessions[sessionID]; exists {
		delete(c.tokens, oldToken)
	}

	c.tokens[token] = &sessionToken{
		Token:     token,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(c.cfg.TokenTTL),
	}
	c.sessions[sessionID] = token

	return token, nil
}

func (c *CSRF) GetToken(sessionID string) (string, error) {
	if sessionID == "" {
		return "", ErrSessionNotFound
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	token, exists := c.sessions[sessionID]
	if !exists {
		return "", ErrTokenNotFound
	}

	st, exists := c.tokens[token]
	if !exists || time.Now().After(st.ExpiresAt) {
		return "", ErrTokenNotFound
	}

	return token, nil
}

func (c *CSRF) ValidateToken(token, sessionID string) error {
	if token == "" {
		return ErrTokenNotFound
	}
	if sessionID == "" {
		return ErrSessionNotFound
	}

	c.mu.RLock()
	st, exists := c.tokens[token]
	c.mu.RUnlock()

	if !exists {
		return ErrTokenInvalid
	}

	if time.Now().After(st.ExpiresAt) {
		c.mu.Lock()
		delete(c.tokens, token)
		if curToken, ok := c.sessions[st.SessionID]; ok && curToken == token {
			delete(c.sessions, st.SessionID)
		}
		c.mu.Unlock()
		return ErrTokenInvalid
	}

	if st.SessionID != sessionID {
		return ErrSessionMismatch
	}

	return nil
}

func (c *CSRF) RotateToken(token, sessionID string) (string, error) {
	if err := c.ValidateToken(token, sessionID); err != nil {
		return "", err
	}

	newToken, err := c.generateToken()
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tokens, token)
	if curToken, ok := c.sessions[sessionID]; ok && curToken == token {
		delete(c.sessions, sessionID)
	}

	c.tokens[newToken] = &sessionToken{
		Token:     newToken,
		SessionID: sessionID,
		ExpiresAt: time.Now().Add(c.cfg.TokenTTL),
	}
	c.sessions[sessionID] = newToken

	return newToken, nil
}

func (c *CSRF) InvalidateSession(sessionID string) error {
	if sessionID == "" {
		return ErrSessionNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if token, exists := c.sessions[sessionID]; exists {
		delete(c.tokens, token)
		delete(c.sessions, sessionID)
	}

	return nil
}

func (c *CSRF) InvalidateToken(token string) error {
	if token == "" {
		return ErrTokenNotFound
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if st, exists := c.tokens[token]; exists {
		delete(c.tokens, token)
		if curToken, ok := c.sessions[st.SessionID]; ok && curToken == token {
			delete(c.sessions, st.SessionID)
		}
	}

	return nil
}

func (c *CSRF) CleanExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for token, st := range c.tokens {
		if now.After(st.ExpiresAt) {
			delete(c.tokens, token)
			if curToken, ok := c.sessions[st.SessionID]; ok && curToken == token {
				delete(c.sessions, st.SessionID)
			}
			cleaned++
		}
	}

	return cleaned
}

func (c *CSRF) isProtectedMethod(method string) bool {
	for _, m := range c.cfg.ProtectedMethods {
		if strings.EqualFold(m, method) {
			return true
		}
	}
	return false
}

func (c *CSRF) isTrustedOrigin(origin string) bool {
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}

	originHost := parsed.Host

	for _, trusted := range c.cfg.TrustedOrigins {
		trusted = strings.TrimSpace(trusted)
		if trusted == "" {
			continue
		}

		if !strings.Contains(trusted, "://") {
			if originHost == trusted {
				return true
			}
			if strings.HasSuffix(originHost, "."+trusted) {
				return true
			}
			continue
		}

		tp, err := url.Parse(trusted)
		if err != nil {
			continue
		}

		if parsed.Scheme == tp.Scheme && parsed.Host == tp.Host {
			return true
		}
	}

	return false
}

func (c *CSRF) isSameOrigin(requestOrigin, host string) bool {
	if requestOrigin == "" || host == "" {
		return false
	}

	parsed, err := url.Parse(requestOrigin)
	if err != nil {
		return false
	}

	return parsed.Host == host
}

func (c *CSRF) checkOrigin(r *http.Request) error {
	if !c.cfg.EnableOriginCheck {
		return nil
	}

	origin := r.Header.Get("Origin")

	if origin == "" {
		return nil
	}

	if c.isSameOrigin(origin, r.Host) {
		return nil
	}

	if c.isTrustedOrigin(origin) {
		return nil
	}

	return ErrOriginNotAllowed
}

func (c *CSRF) checkReferer(r *http.Request) error {
	if !c.cfg.EnableRefererCheck {
		return nil
	}

	referer := r.Header.Get("Referer")

	if referer == "" {
		return nil
	}

	parsed, err := url.Parse(referer)
	if err != nil {
		return ErrRefererNotAllowed
	}

	if parsed.Host == r.Host {
		return nil
	}

	origin := parsed.Scheme + "://" + parsed.Host
	if c.isTrustedOrigin(origin) {
		return nil
	}

	return ErrRefererNotAllowed
}

func (c *CSRF) extractToken(r *http.Request) string {
	if token := r.Header.Get(c.cfg.HeaderName); token != "" {
		return token
	}

	if cookie, err := r.Cookie(c.cfg.CookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
		if err := r.ParseForm(); err == nil {
			if token := r.FormValue(c.cfg.FormFieldName); token != "" {
				return token
			}
		}
	}

	return ""
}

func (c *CSRF) extractDoubleSubmitCookieToken(r *http.Request) (string, string) {
	var cookieToken, headerToken string

	if cookie, err := r.Cookie(c.cfg.CookieName); err == nil {
		cookieToken = cookie.Value
	}

	if c.cfg.HeaderName != "" {
		headerToken = r.Header.Get(c.cfg.HeaderName)
	}

	return cookieToken, headerToken
}

func (c *CSRF) setTokenCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     c.cfg.CookieName,
		Value:    token,
		Path:     c.cfg.CookiePath,
		Domain:   c.cfg.CookieDomain,
		MaxAge:   int(c.cfg.TokenTTL.Seconds()),
		Secure:   c.cfg.CookieSecure,
		HttpOnly: c.cfg.CookieHTTPOnly,
		SameSite: c.cfg.CookieSameSite,
	}
	http.SetCookie(w, cookie)
}

func (c *CSRF) clearTokenCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     c.cfg.CookieName,
		Value:    "",
		Path:     c.cfg.CookiePath,
		Domain:   c.cfg.CookieDomain,
		MaxAge:   -1,
		Secure:   c.cfg.CookieSecure,
		HttpOnly: c.cfg.CookieHTTPOnly,
		SameSite: c.cfg.CookieSameSite,
	}
	http.SetCookie(w, cookie)
}

func (c *CSRF) defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, "Forbidden: "+err.Error(), http.StatusForbidden)
}

func (c *CSRF) handleError(w http.ResponseWriter, r *http.Request, err error) {
	if c.cfg.ErrorHandler != nil {
		c.cfg.ErrorHandler(w, r, err)
		return
	}
	c.defaultErrorHandler(w, r, err)
}

func (c *CSRF) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID := c.getSessionID(r)

		if !c.isProtectedMethod(r.Method) {
			if sessionID != "" && c.cfg.Mode == DoubleSubmitCookieMode {
				_, err := c.GetToken(sessionID)
				if err != nil {
					token, genErr := c.GenerateToken(sessionID)
					if genErr == nil {
						c.setTokenCookie(w, token)
						w.Header().Set(c.cfg.HeaderName, token)
					}
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		if err := c.checkOrigin(r); err != nil {
			c.handleError(w, r, err)
			return
		}

		if err := c.checkReferer(r); err != nil {
			c.handleError(w, r, err)
			return
		}

		var validateErr error

		switch c.cfg.Mode {
		case SynchronizerTokenMode:
			token := c.extractToken(r)
			if token == "" {
				c.handleError(w, r, ErrTokenNotFound)
				return
			}
			validateErr = c.ValidateToken(token, sessionID)
			if validateErr == nil && c.cfg.EnableTokenRotation {
				newToken, rotErr := c.RotateToken(token, sessionID)
				if rotErr == nil {
					w.Header().Set(c.cfg.HeaderName, newToken)
				}
			}

		case DoubleSubmitCookieMode:
			cookieToken, headerToken := c.extractDoubleSubmitCookieToken(r)
			if cookieToken == "" || headerToken == "" {
				c.handleError(w, r, ErrTokenNotFound)
				return
			}
			if cookieToken != headerToken {
				c.handleError(w, r, ErrTokenMismatch)
				return
			}
			validateErr = c.ValidateToken(cookieToken, sessionID)
			if validateErr == nil && c.cfg.EnableTokenRotation {
				newToken, rotErr := c.RotateToken(cookieToken, sessionID)
				if rotErr == nil {
					c.setTokenCookie(w, newToken)
					w.Header().Set(c.cfg.HeaderName, newToken)
				}
			}
		}

		if validateErr != nil {
			c.clearTokenCookie(w)
			c.handleError(w, r, validateErr)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (c *CSRF) GenerateHandler(w http.ResponseWriter, r *http.Request) {
	sessionID := c.getSessionID(r)
	if sessionID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := c.GenerateToken(sessionID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	switch c.cfg.Mode {
	case DoubleSubmitCookieMode:
		c.setTokenCookie(w, token)
	}

	w.Header().Set(c.cfg.HeaderName, token)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"token":"` + token + `"}`))
}

func (c *CSRF) Mode() ProtectionMode {
	return c.cfg.Mode
}

func (c *CSRF) Config() Config {
	return c.cfg
}

func (c *CSRF) TokenCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.tokens)
}

func (c *CSRF) SessionCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.sessions)
}
