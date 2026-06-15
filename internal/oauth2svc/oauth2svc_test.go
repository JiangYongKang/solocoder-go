package oauth2svc

import (
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func setupTestServer() *AuthorizationServer {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read", "write", "profile"},
	})

	clientStore.SaveClient(&Client{
		ID:           "multi-uri-client",
		Secret:       "multi-secret",
		RedirectURIs: []string{"http://a.com/cb", "http://b.com/cb"},
		Scopes:       []string{"read"},
	})

	clientStore.SaveClient(&Client{
		ID:           "admin-client",
		Secret:       "admin-secret",
		RedirectURIs: []string{"http://admin.local/callback"},
		Scopes:       []string{"admin:read", "admin:write"},
	})

	return NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)
}

func TestAuthorize_Success(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		State:        "state123",
		UserID:       "user123",
	}

	code, err := srv.Authorize(req)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}
	if code == "" {
		t.Fatal("expected non-empty authorization code")
	}
	if len(code) != 64 {
		t.Errorf("expected code length 64, got %d", len(code))
	}
}

func TestAuthorize_DefaultScope(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		UserID:       "user123",
	}

	code, err := srv.Authorize(req)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	authCode, err := srv.codeStore.GetCode(code)
	if err != nil {
		t.Fatalf("GetCode failed: %v", err)
	}
	if authCode.Scope != "read write profile" {
		t.Errorf("expected default scope 'read write profile', got %q", authCode.Scope)
	}
}

func TestAuthorize_DefaultRedirectURI(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		Scope:        "read",
		UserID:       "user123",
	}

	code, err := srv.Authorize(req)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	authCode, err := srv.codeStore.GetCode(code)
	if err != nil {
		t.Fatalf("GetCode failed: %v", err)
	}
	if authCode.RedirectURI != "http://localhost/callback" {
		t.Errorf("expected default redirect URI, got %q", authCode.RedirectURI)
	}
}

func TestAuthorize_InvalidResponseType(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: "token",
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestAuthorize_MissingClientID(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		RedirectURI:  "http://localhost/callback",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestAuthorize_InvalidClient(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "nonexistent",
		RedirectURI:  "http://localhost/callback",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient, got %v", err)
	}
}

func TestAuthorize_InvalidRedirectURI(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://evil.com/callback",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestAuthorize_MissingRedirectURI_MultipleAvailable(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "multi-uri-client",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest when multiple URIs available but none specified, got %v", err)
	}
}

func TestAuthorize_InvalidScope(t *testing.T) {
	srv := setupTestServer()

	req := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read delete",
		UserID:       "user123",
	}

	_, err := srv.Authorize(req)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope, got %v", err)
	}
}

func TestToken_AuthorizationCode_Success(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token type 'Bearer', got %q", resp.TokenType)
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("expected expires_in 3600, got %d", resp.ExpiresIn)
	}
	if resp.RefreshToken == "" {
		t.Error("expected non-empty refresh token")
	}
	if resp.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", resp.Scope)
	}
}

func TestToken_AuthorizationCode_InvalidClient(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "wrong-client",
		ClientSecret: "test-secret",
		Code:         "some-code",
		RedirectURI:  "http://localhost/callback",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient, got %v", err)
	}
}

func TestToken_AuthorizationCode_InvalidSecret(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "wrong-secret",
		Code:         "some-code",
		RedirectURI:  "http://localhost/callback",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient, got %v", err)
	}
}

func TestToken_AuthorizationCode_InvalidCode(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         "nonexistent-code",
		RedirectURI:  "http://localhost/callback",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant, got %v", err)
	}
}

func TestToken_AuthorizationCode_ClientMismatch(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "admin-client",
		ClientSecret: "admin-secret",
		Code:         code,
		RedirectURI:  "http://admin.local/callback",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for client mismatch, got %v", err)
	}
}

func TestToken_AuthorizationCode_RedirectURIMismatch(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://wrong.com/callback",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for redirect URI mismatch, got %v", err)
	}
}

func TestToken_AuthorizationCode_Reuse(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}

	_, err = srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("First token request failed: %v", err)
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for code reuse, got %v", err)
	}
}

func TestToken_AuthorizationCode_Expired(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.AuthorizationCodeTTL = 10 * time.Millisecond

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for expired code, got %v", err)
	}
}

func TestToken_AuthorizationCode_InvalidScopeInTokenRequest(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read admin:read",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope, got %v", err)
	}
}

func TestToken_AuthorizationCode_NarrowScope(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write profile",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "read" {
		t.Errorf("expected narrowed scope 'read', got %q", resp.Scope)
	}
}

func TestToken_AuthorizationCode_MissingCode(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURI:  "http://localhost/callback",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestToken_ClientCredentials_Success(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "read write",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("expected non-empty access token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("expected token type 'Bearer', got %q", resp.TokenType)
	}
	if resp.RefreshToken != "" {
		t.Error("client credentials should NOT return refresh token")
	}
	if resp.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", resp.Scope)
	}

	claims, err := srv.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}
	if claims.ClientID != "test-client" {
		t.Errorf("expected client_id 'test-client', got %q", claims.ClientID)
	}
	if claims.Subject != "" {
		t.Errorf("expected empty subject for client credentials, got %q", claims.Subject)
	}
}

func TestToken_ClientCredentials_DefaultScope(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "read write profile" {
		t.Errorf("expected default scope, got %q", resp.Scope)
	}
}

func TestToken_ClientCredentials_InvalidClient(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "wrong-client",
		ClientSecret: "test-secret",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient, got %v", err)
	}
}

func TestToken_ClientCredentials_InvalidScope(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "delete",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope, got %v", err)
	}
}

func TestToken_RefreshToken_Success(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, _ := srv.Authorize(authReq)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}
	resp1, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Initial token request failed: %v", err)
	}

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: resp1.RefreshToken,
	}

	resp2, err := srv.Token(refreshReq)
	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}

	if resp2.AccessToken == resp1.AccessToken {
		t.Error("expected new access token after refresh")
	}
	if resp2.RefreshToken == resp1.RefreshToken {
		t.Error("expected new refresh token after refresh (rotation)")
	}
	if resp2.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", resp2.Scope)
	}

	_, err = srv.refreshTokenStore.GetToken(resp1.RefreshToken)
	if err == nil {
		t.Error("old refresh token should be revoked after use")
	}
}

func TestToken_RefreshToken_NarrowScope(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write profile",
		UserID:       "user123",
	}
	code, _ := srv.Authorize(authReq)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}
	resp, _ := srv.Token(tokenReq)

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: resp.RefreshToken,
		Scope:        "read",
	}

	resp2, err := srv.Token(refreshReq)
	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}
	if resp2.Scope != "read" {
		t.Errorf("expected narrowed scope 'read', got %q", resp2.Scope)
	}
}

func TestToken_RefreshToken_InvalidScope(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, _ := srv.Authorize(authReq)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}
	resp, _ := srv.Token(tokenReq)

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: resp.RefreshToken,
		Scope:        "read write",
	}

	_, err := srv.Token(refreshReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope for requesting scope not originally granted, got %v", err)
	}
}

func TestToken_RefreshToken_InvalidClient(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, _ := srv.Authorize(authReq)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}
	resp, _ := srv.Token(tokenReq)

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "admin-client",
		ClientSecret: "admin-secret",
		RefreshToken: resp.RefreshToken,
	}

	_, err := srv.Token(refreshReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for client mismatch, got %v", err)
	}
}

func TestToken_RefreshToken_InvalidToken(t *testing.T) {
	srv := setupTestServer()

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: "nonexistent-refresh-token",
	}

	_, err := srv.Token(refreshReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant, got %v", err)
	}
}

func TestToken_RefreshToken_MissingToken(t *testing.T) {
	srv := setupTestServer()

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	_, err := srv.Token(refreshReq)
	if err != ErrInvalidRequest {
		t.Errorf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestToken_RefreshToken_Expired(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenTTL = 10 * time.Millisecond

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, _ := srv.Authorize(authReq)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}
	resp, _ := srv.Token(tokenReq)

	time.Sleep(50 * time.Millisecond)

	refreshReq := &TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: resp.RefreshToken,
	}

	_, err := srv.Token(refreshReq)
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant for expired refresh token, got %v", err)
	}
}

func TestToken_UnsupportedGrantType(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    "password",
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}

	_, err := srv.Token(tokenReq)
	if err != ErrUnsupportedGrantType {
		t.Errorf("expected ErrUnsupportedGrantType, got %v", err)
	}
}

func TestValidateToken_Success(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "read write",
	}
	resp, _ := srv.Token(tokenReq)

	claims, err := srv.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.Issuer != "oauth2svc" {
		t.Errorf("expected issuer 'oauth2svc', got %q", claims.Issuer)
	}
	if claims.Audience != "test-client" {
		t.Errorf("expected audience 'test-client', got %q", claims.Audience)
	}
	if claims.ClientID != "test-client" {
		t.Errorf("expected client_id 'test-client', got %q", claims.ClientID)
	}
	if claims.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", claims.Scope)
	}
	if claims.TokenID == "" {
		t.Error("expected non-empty token ID")
	}
	if claims.ExpiresAt.Before(time.Now()) {
		t.Error("token should not be expired")
	}
}

func TestValidateToken_InvalidSignature(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	resp, _ := srv.Token(tokenReq)

	parts := strings.Split(resp.AccessToken, ".")
	tampered := parts[0] + "." + parts[1] + ".tampered-signature"

	_, err := srv.ValidateToken(tampered)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for tampered signature, got %v", err)
	}
}

func TestValidateToken_InvalidFormat(t *testing.T) {
	srv := setupTestServer()

	_, err := srv.ValidateToken("invalid-token-format")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for invalid format, got %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.AccessTokenTTL = 10 * time.Millisecond

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	resp, _ := srv.Token(tokenReq)

	time.Sleep(50 * time.Millisecond)

	_, err := srv.ValidateToken(resp.AccessToken)
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestValidateToken_WrongKey(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
	}
	resp, _ := srv.Token(tokenReq)

	_, err := ValidateJWT(resp.AccessToken, []byte("wrong-signing-key"))
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for wrong key, got %v", err)
	}
}

func TestMemoryClientStore_Basic(t *testing.T) {
	store := NewMemoryClientStore()

	client := &Client{
		ID:           "c1",
		Secret:       "s1",
		RedirectURIs: []string{"http://a.com"},
		Scopes:       []string{"read"},
	}

	if err := store.SaveClient(client); err != nil {
		t.Fatalf("SaveClient failed: %v", err)
	}

	got, err := store.GetClient("c1")
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}
	if got.ID != "c1" {
		t.Errorf("expected client c1, got %s", got.ID)
	}

	_, err = store.GetClient("nonexistent")
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient, got %v", err)
	}

	valid, err := store.ValidateClient("c1", "s1")
	if err != nil {
		t.Fatalf("ValidateClient failed: %v", err)
	}
	if valid.ID != "c1" {
		t.Errorf("expected valid client c1")
	}

	_, err = store.ValidateClient("c1", "wrong")
	if err != ErrInvalidClient {
		t.Errorf("expected ErrInvalidClient for wrong secret, got %v", err)
	}
}

func TestMemoryAuthorizationCodeStore_Basic(t *testing.T) {
	store := NewMemoryAuthorizationCodeStore()

	code := &AuthorizationCode{
		Code:        "code123",
		ClientID:    "c1",
		UserID:      "u1",
		Scope:       "read",
		RedirectURI: "http://a.com",
		ExpiresAt:   time.Now().Add(time.Hour),
		CreatedAt:   time.Now(),
	}

	if err := store.SaveCode(code); err != nil {
		t.Fatalf("SaveCode failed: %v", err)
	}

	got, err := store.GetCode("code123")
	if err != nil {
		t.Fatalf("GetCode failed: %v", err)
	}
	if got.Code != "code123" {
		t.Errorf("expected code123, got %s", got.Code)
	}

	_, err = store.GetCode("nonexistent")
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant, got %v", err)
	}

	if err := store.MarkCodeUsed("code123"); err != nil {
		t.Fatalf("MarkCodeUsed failed: %v", err)
	}

	got2, _ := store.GetCode("code123")
	if !got2.Used {
		t.Error("code should be marked as used")
	}

	err = store.MarkCodeUsed("code123")
	if err != ErrCodeUsed {
		t.Errorf("expected ErrCodeUsed, got %v", err)
	}
}

func TestMemoryAuthorizationCodeStore_Expired(t *testing.T) {
	store := NewMemoryAuthorizationCodeStore()

	code := &AuthorizationCode{
		Code:        "expired-code",
		ClientID:    "c1",
		ExpiresAt:   time.Now().Add(-time.Hour),
		CreatedAt:   time.Now().Add(-2 * time.Hour),
	}

	store.SaveCode(code)

	_, err := store.GetCode("expired-code")
	if err != ErrCodeExpired {
		t.Errorf("expected ErrCodeExpired, got %v", err)
	}
}

func TestMemoryRefreshTokenStore_Basic(t *testing.T) {
	store := NewMemoryRefreshTokenStore()

	rt := &RefreshToken{
		Token:     "rt123",
		ClientID:  "c1",
		UserID:    "u1",
		Scope:     "read",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	if err := store.SaveToken(rt); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	got, err := store.GetToken("rt123")
	if err != nil {
		t.Fatalf("GetToken failed: %v", err)
	}
	if got.Token != "rt123" {
		t.Errorf("expected rt123, got %s", got.Token)
	}

	_, err = store.GetToken("nonexistent")
	if err != ErrInvalidGrant {
		t.Errorf("expected ErrInvalidGrant, got %v", err)
	}

	if err := store.RevokeToken("rt123"); err != nil {
		t.Fatalf("RevokeToken failed: %v", err)
	}

	_, err = store.GetToken("rt123")
	if err != ErrRefreshTokenRevoked {
		t.Errorf("expected ErrRefreshTokenRevoked, got %v", err)
	}
}

func TestMemoryRefreshTokenStore_Expired(t *testing.T) {
	store := NewMemoryRefreshTokenStore()

	rt := &RefreshToken{
		Token:     "expired-rt",
		ClientID:  "c1",
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}

	store.SaveToken(rt)

	_, err := store.GetToken("expired-rt")
	if err != ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWT_RoundTrip(t *testing.T) {
	key := []byte("test-key")

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		Audience:  "client1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
		ClientID:  "client1",
		Scope:     "read write",
		TokenID:   "token-id-123",
	}

	token, err := GenerateJWT(claims, key)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts in JWT, got %d", len(parts))
	}

	parsed, err := ParseJWT(token, key)
	if err != nil {
		t.Fatalf("ParseJWT failed: %v", err)
	}

	if parsed.Issuer != "test" {
		t.Errorf("expected issuer 'test', got %q", parsed.Issuer)
	}
	if parsed.Subject != "user1" {
		t.Errorf("expected subject 'user1', got %q", parsed.Subject)
	}
	if parsed.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", parsed.Scope)
	}
}

func TestFullAuthorizationCodeFlow(t *testing.T) {
	srv := setupTestServer()

	authCode, err := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read profile",
		UserID:       "user-456",
	})
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}

	claims, err := srv.ValidateToken(tokenResp.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed: %v", err)
	}

	if claims.Subject != "user-456" {
		t.Errorf("expected user-456 in token, got %q", claims.Subject)
	}
	if claims.Scope != "read profile" {
		t.Errorf("expected scope 'read profile', got %q", claims.Scope)
	}
	if claims.ClientID != "test-client" {
		t.Errorf("expected client test-client, got %q", claims.ClientID)
	}

	refreshResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: tokenResp.RefreshToken,
	})
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	claims2, err := srv.ValidateToken(refreshResp.AccessToken)
	if err != nil {
		t.Fatalf("Validate refreshed token failed: %v", err)
	}

	if claims2.Subject != "user-456" {
		t.Errorf("refreshed token should keep same subject")
	}
	if claims2.Scope != "read profile" {
		t.Errorf("refreshed token should keep same scope")
	}
}

func TestConcurrent_AuthorizationCode(t *testing.T) {
	srv := setupTestServer()

	var wg sync.WaitGroup
	var errors int64
	var success int64

	numGoroutines := 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			authReq := &AuthorizeRequest{
				ResponseType: ResponseTypeCode,
				ClientID:     "test-client",
				RedirectURI:  "http://localhost/callback",
				Scope:        "read",
				UserID:       "user" + string(rune('0'+id)),
			}

			code, err := srv.Authorize(authReq)
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}

			tokenReq := &TokenRequest{
				GrantType:    GrantTypeAuthorizationCode,
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				Code:         code,
				RedirectURI:  "http://localhost/callback",
			}

			resp, err := srv.Token(tokenReq)
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}

			if resp.AccessToken == "" {
				atomic.AddInt64(&errors, 1)
				return
			}

			atomic.AddInt64(&success, 1)
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt64(&errors) != 0 {
		t.Errorf("got %d errors during concurrent authorization code flow", atomic.LoadInt64(&errors))
	}
	if atomic.LoadInt64(&success) != int64(numGoroutines) {
		t.Errorf("expected %d successes, got %d", numGoroutines, atomic.LoadInt64(&success))
	}
}

func TestConcurrent_TokenRefresh(t *testing.T) {
	srv := setupTestServer()

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user1",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	var wg sync.WaitGroup
	var success int64
	var invalidGrant int64

	numGoroutines := 10
	refreshToken := tokenResp.RefreshToken

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			refreshReq := &TokenRequest{
				GrantType:    GrantTypeRefreshToken,
				ClientID:     "test-client",
				ClientSecret: "test-secret",
				RefreshToken: refreshToken,
			}

			_, err := srv.Token(refreshReq)
			if err == nil {
				atomic.AddInt64(&success, 1)
			} else if err == ErrInvalidGrant {
				atomic.AddInt64(&invalidGrant, 1)
			}
		}()
	}

	wg.Wait()

	if atomic.LoadInt64(&success) != 1 {
		t.Errorf("expected exactly 1 successful refresh, got %d", atomic.LoadInt64(&success))
	}
	if atomic.LoadInt64(&invalidGrant) != int64(numGoroutines-1) {
		t.Errorf("expected %d invalid_grant errors, got %d", numGoroutines-1, atomic.LoadInt64(&invalidGrant))
	}
}

func TestScopeValidation(t *testing.T) {
	tests := []struct {
		name      string
		client    []string
		requested string
		expected  bool
	}{
		{"empty request", []string{"read", "write"}, "", true},
		{"single match", []string{"read", "write"}, "read", true},
		{"multiple match", []string{"read", "write", "admin"}, "read write", true},
		{"partial match", []string{"read", "write"}, "read delete", false},
		{"no match", []string{"read"}, "write", false},
		{"empty client scopes", nil, "read", false},
		{"substring not match", []string{"read:all"}, "read", false},
		{"exact match with colon", []string{"read:all", "write:all"}, "read:all", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupTestServer()
			result := srv.validateScope(tt.client, tt.requested)
			if result != tt.expected {
				t.Errorf("validateScope(%v, %q) = %v, expected %v",
					tt.client, tt.requested, result, tt.expected)
			}
		})
	}
}

func TestParseScope(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", nil},
		{"read", []string{"read"}},
		{"read write", []string{"read", "write"}},
		{"read  write   profile", []string{"read", "write", "profile"}},
		{"read:all write:all", []string{"read:all", "write:all"}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseScope(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseScope(%q) returned %d elements, expected %d",
					tt.input, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("parseScope(%q)[%d] = %q, expected %q",
						tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestRedirectURIValidation(t *testing.T) {
	tests := []struct {
		name      string
		client    *Client
		requested string
		expected  bool
	}{
		{
			"exact match single URI",
			&Client{RedirectURIs: []string{"http://a.com/cb"}},
			"http://a.com/cb",
			true,
		},
		{
			"exact match multiple URIs",
			&Client{RedirectURIs: []string{"http://a.com/cb", "http://b.com/cb"}},
			"http://b.com/cb",
			true,
		},
		{
			"empty request single URI",
			&Client{RedirectURIs: []string{"http://a.com/cb"}},
			"",
			true,
		},
		{
			"empty request multiple URIs",
			&Client{RedirectURIs: []string{"http://a.com/cb", "http://b.com/cb"}},
			"",
			false,
		},
		{
			"no match",
			&Client{RedirectURIs: []string{"http://a.com/cb"}},
			"http://evil.com/cb",
			false,
		},
		{
			"prefix not match",
			&Client{RedirectURIs: []string{"http://a.com/cb"}},
			"http://a.com/cb/extra",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupTestServer()
			result := srv.validateRedirectURI(tt.client, tt.requested)
			if result != tt.expected {
				t.Errorf("validateRedirectURI(%v, %q) = %v, expected %v",
					tt.client.RedirectURIs, tt.requested, result, tt.expected)
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Issuer != "oauth2svc" {
		t.Errorf("expected issuer 'oauth2svc', got %q", config.Issuer)
	}
	if config.AccessTokenTTL != time.Hour {
		t.Errorf("expected AccessTokenTTL 1h, got %v", config.AccessTokenTTL)
	}
	if config.RefreshTokenTTL != 7*24*time.Hour {
		t.Errorf("expected RefreshTokenTTL 7d, got %v", config.RefreshTokenTTL)
	}
	if config.AuthorizationCodeTTL != 10*time.Minute {
		t.Errorf("expected AuthorizationCodeTTL 10m, got %v", config.AuthorizationCodeTTL)
	}
	if !config.RefreshTokenRotation {
		t.Error("expected RefreshTokenRotation true")
	}
	if len(config.SigningKey) == 0 {
		t.Error("expected non-empty signing key")
	}
}

func TestGetConfig(t *testing.T) {
	config := DefaultConfig()
	config.Issuer = "custom-issuer"
	srv := NewAuthorizationServer(
		config,
		NewMemoryClientStore(),
		NewMemoryAuthorizationCodeStore(),
		NewMemoryRefreshTokenStore(),
	)

	got := srv.GetConfig()
	if got.Issuer != "custom-issuer" {
		t.Errorf("expected custom-issuer, got %q", got.Issuer)
	}
}

func TestGenerateRandomString(t *testing.T) {
	s1 := generateRandomString(16)
	s2 := generateRandomString(16)

	if s1 == s2 {
		t.Error("expected different random strings")
	}
	if len(s1) != 32 {
		t.Errorf("expected length 32, got %d", len(s1))
	}

	s3 := generateRandomString(32)
	if len(s3) != 64 {
		t.Errorf("expected length 64, got %d", len(s3))
	}
}

func TestToken_AuthorizationCode_ScopeSubsetValidation(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope for requesting scope larger than authorized, got %v", err)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_Equal(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "read write" {
		t.Errorf("expected scope 'read write', got %q", resp.Scope)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_Narrow(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write profile",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "read" {
		t.Errorf("expected narrowed scope 'read', got %q", resp.Scope)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_NoScopeInTokenRequest(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "read write" {
		t.Errorf("expected original scope 'read write' when no scope in token request, got %q", resp.Scope)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_CompletelyUnauthorized(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "admin:read",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope for completely unauthorized scope, got %v", err)
	}
}

func TestToken_RefreshToken_RotationEnabled(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenRotation = true

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	oldRefreshToken := tokenResp.RefreshToken

	refreshResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: oldRefreshToken,
	})

	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}

	if refreshResp.RefreshToken == "" {
		t.Error("expected new refresh token when rotation is enabled")
	}
	if refreshResp.RefreshToken == oldRefreshToken {
		t.Error("expected new refresh token to be different from old one")
	}

	_, err = srv.refreshTokenStore.GetToken(oldRefreshToken)
	if err == nil {
		t.Error("old refresh token should be revoked when rotation is enabled")
	}

	_, err = srv.refreshTokenStore.GetToken(refreshResp.RefreshToken)
	if err != nil {
		t.Errorf("new refresh token should be valid: %v", err)
	}
}

func TestToken_RefreshToken_RotationDisabled(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenRotation = false

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	oldRefreshToken := tokenResp.RefreshToken

	refreshResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: oldRefreshToken,
	})

	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}

	if refreshResp.RefreshToken != "" {
		t.Error("expected NO refresh token when rotation is disabled")
	}

	rt, err := srv.refreshTokenStore.GetToken(oldRefreshToken)
	if err != nil {
		t.Errorf("old refresh token should still be valid when rotation is disabled: %v", err)
	}
	if rt.Revoked {
		t.Error("old refresh token should NOT be revoked when rotation is disabled")
	}
}

func TestToken_RefreshToken_RotationDisabled_ReuseRefreshToken(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenRotation = false

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read",
		UserID:       "user123",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	refreshToken := tokenResp.RefreshToken

	resp1, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: refreshToken,
	})
	if err != nil {
		t.Fatalf("First refresh failed: %v", err)
	}

	resp2, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: refreshToken,
	})
	if err != nil {
		t.Fatalf("Second refresh failed: %v", err)
	}

	if resp1.AccessToken == resp2.AccessToken {
		t.Error("expected different access tokens for each refresh")
	}
}

func TestParseJWT_AlgorithmValidation(t *testing.T) {
	key := []byte("test-key")

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		Audience:  "client1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
		ClientID:  "client1",
		Scope:     "read",
		TokenID:   "token123",
	}

	validToken, err := GenerateJWT(claims, key)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	_, err = ParseJWT(validToken, key)
	if err != nil {
		t.Errorf("valid HS256 token should be accepted, got error: %v", err)
	}
}

func TestParseJWT_AlgorithmValidation_NoneAlgorithm(t *testing.T) {
	key := []byte("test-key")

	noneHeader := jwtHeader{
		Alg: "none",
		Typ: "JWT",
	}

	headerJSON, _ := json.Marshal(noneHeader)
	encodedHeader := base64URLEncode(headerJSON)

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	claimsJSON, _ := json.Marshal(claims)
	encodedClaims := base64URLEncode(claimsJSON)

	signingInput := encodedHeader + "." + encodedClaims
	signature := base64URLEncode([]byte(""))

	noneToken := signingInput + "." + signature

	_, err := ParseJWT(noneToken, key)
	if err != ErrInvalidToken {
		t.Errorf("token with alg=none should be rejected, got error: %v", err)
	}
}

func TestParseJWT_AlgorithmValidation_RS256Algorithm(t *testing.T) {
	key := []byte("test-key")

	rs256Header := jwtHeader{
		Alg: "RS256",
		Typ: "JWT",
	}

	headerJSON, _ := json.Marshal(rs256Header)
	encodedHeader := base64URLEncode(headerJSON)

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	claimsJSON, _ := json.Marshal(claims)
	encodedClaims := base64URLEncode(claimsJSON)

	signingInput := encodedHeader + "." + encodedClaims
	signature := base64URLEncode(signHS256([]byte(signingInput), key))

	rs256Token := signingInput + "." + signature

	_, err := ParseJWT(rs256Token, key)
	if err != ErrInvalidToken {
		t.Errorf("token with alg=RS256 should be rejected, got error: %v", err)
	}
}

func TestParseJWT_AlgorithmValidation_EmptyAlgorithm(t *testing.T) {
	key := []byte("test-key")

	emptyAlgHeader := jwtHeader{
		Alg: "",
		Typ: "JWT",
	}

	headerJSON, _ := json.Marshal(emptyAlgHeader)
	encodedHeader := base64URLEncode(headerJSON)

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	claimsJSON, _ := json.Marshal(claims)
	encodedClaims := base64URLEncode(claimsJSON)

	signingInput := encodedHeader + "." + encodedClaims
	signature := base64URLEncode(signHS256([]byte(signingInput), key))

	emptyAlgToken := signingInput + "." + signature

	_, err := ParseJWT(emptyAlgToken, key)
	if err != ErrInvalidToken {
		t.Errorf("token with empty alg should be rejected, got error: %v", err)
	}
}

func TestParseJWT_AlgorithmValidation_CaseSensitive(t *testing.T) {
	key := []byte("test-key")

	lowerCaseHeader := jwtHeader{
		Alg: "hs256",
		Typ: "JWT",
	}

	headerJSON, _ := json.Marshal(lowerCaseHeader)
	encodedHeader := base64URLEncode(headerJSON)

	claims := &AccessTokenClaims{
		Issuer:    "test",
		Subject:   "user1",
		ExpiresAt: time.Now().Add(time.Hour),
		IssuedAt:  time.Now(),
	}
	claimsJSON, _ := json.Marshal(claims)
	encodedClaims := base64URLEncode(claimsJSON)

	signingInput := encodedHeader + "." + encodedClaims
	signature := base64URLEncode(signHS256([]byte(signingInput), key))

	lowerCaseToken := signingInput + "." + signature

	_, err := ParseJWT(lowerCaseToken, key)
	if err != ErrInvalidToken {
		t.Errorf("token with alg=hs256 (lowercase) should be rejected due to case sensitivity, got error: %v", err)
	}
}

func TestParseJWT_InvalidHeaderBase64(t *testing.T) {
	key := []byte("test-key")

	invalidToken := "!!!invalid-base64!!!.eyJzdWIiOiJ1c2VyMSJ9.signature"

	_, err := ParseJWT(invalidToken, key)
	if err != ErrInvalidToken {
		t.Errorf("token with invalid base64 header should be rejected, got error: %v", err)
	}
}

func TestParseJWT_InvalidHeaderJSON(t *testing.T) {
	key := []byte("test-key")

	invalidJSON := base64URLEncode([]byte("not valid json"))
	claims := base64URLEncode([]byte(`{"sub":"user1"}`))
	signature := base64URLEncode([]byte("sig"))

	invalidToken := invalidJSON + "." + claims + "." + signature

	_, err := ParseJWT(invalidToken, key)
	if err != ErrInvalidToken {
		t.Errorf("token with invalid JSON header should be rejected, got error: %v", err)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_PartialOverlap(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "read profile",
	}

	_, err = srv.Token(tokenReq)
	if err != ErrInvalidScope {
		t.Errorf("expected ErrInvalidScope for partially overlapping scope with unauthorized 'profile', got %v", err)
	}
}

func TestToken_AuthorizationCode_ScopeSubset_OrderIndependent(t *testing.T) {
	srv := setupTestServer()

	authReq := &AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write profile",
		UserID:       "user123",
	}
	code, err := srv.Authorize(authReq)
	if err != nil {
		t.Fatalf("Authorize failed: %v", err)
	}

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         code,
		RedirectURI:  "http://localhost/callback",
		Scope:        "profile read",
	}

	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}
	if resp.Scope != "profile read" {
		t.Errorf("expected scope 'profile read' (order preserved), got %q", resp.Scope)
	}
}

func TestToken_RefreshToken_RotationEnabled_ScopeNarrowed(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenRotation = true

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read", "write", "profile"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write profile",
		UserID:       "user123",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	refreshResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: tokenResp.RefreshToken,
		Scope:        "read",
	})

	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}

	if refreshResp.Scope != "read" {
		t.Errorf("expected narrowed scope 'read', got %q", refreshResp.Scope)
	}
	if refreshResp.RefreshToken == "" {
		t.Error("expected new refresh token even when scope is narrowed")
	}
}

func TestToken_RefreshToken_RotationDisabled_ScopeNarrowed(t *testing.T) {
	config := DefaultConfig()
	config.SigningKey = []byte("test-signing-key")
	config.RefreshTokenRotation = false

	clientStore := NewMemoryClientStore()
	codeStore := NewMemoryAuthorizationCodeStore()
	refreshTokenStore := NewMemoryRefreshTokenStore()

	clientStore.SaveClient(&Client{
		ID:           "test-client",
		Secret:       "test-secret",
		RedirectURIs: []string{"http://localhost/callback"},
		Scopes:       []string{"read", "write"},
	})

	srv := NewAuthorizationServer(config, clientStore, codeStore, refreshTokenStore)

	authCode, _ := srv.Authorize(&AuthorizeRequest{
		ResponseType: ResponseTypeCode,
		ClientID:     "test-client",
		RedirectURI:  "http://localhost/callback",
		Scope:        "read write",
		UserID:       "user123",
	})

	tokenResp, _ := srv.Token(&TokenRequest{
		GrantType:    GrantTypeAuthorizationCode,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Code:         authCode,
		RedirectURI:  "http://localhost/callback",
	})

	oldRefreshToken := tokenResp.RefreshToken

	refreshResp, err := srv.Token(&TokenRequest{
		GrantType:    GrantTypeRefreshToken,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RefreshToken: oldRefreshToken,
		Scope:        "read",
	})

	if err != nil {
		t.Fatalf("Refresh token failed: %v", err)
	}

	if refreshResp.Scope != "read" {
		t.Errorf("expected narrowed scope 'read', got %q", refreshResp.Scope)
	}
	if refreshResp.RefreshToken != "" {
		t.Error("expected NO refresh token when rotation is disabled, even with scope narrowing")
	}

	rt, err := srv.refreshTokenStore.GetToken(oldRefreshToken)
	if err != nil {
		t.Fatalf("old refresh token should still be valid: %v", err)
	}
	if rt.Scope != "read write" {
		t.Errorf("original refresh token should keep its original scope 'read write', got %q", rt.Scope)
	}
}

func TestValidateToken_AlgorithmValidation_Integration(t *testing.T) {
	srv := setupTestServer()

	tokenReq := &TokenRequest{
		GrantType:    GrantTypeClientCredentials,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Scope:        "read",
	}
	resp, err := srv.Token(tokenReq)
	if err != nil {
		t.Fatalf("Token failed: %v", err)
	}

	claims, err := srv.ValidateToken(resp.AccessToken)
	if err != nil {
		t.Fatalf("ValidateToken failed for valid token: %v", err)
	}
	if claims.Scope != "read" {
		t.Errorf("expected scope 'read', got %q", claims.Scope)
	}
}
