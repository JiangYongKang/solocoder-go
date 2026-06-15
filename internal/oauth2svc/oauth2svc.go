package oauth2svc

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

type AuthorizationServer struct {
	config          Config
	clientStore     ClientStore
	codeStore       AuthorizationCodeStore
	refreshTokenStore RefreshTokenStore
}

func NewAuthorizationServer(
	config Config,
	clientStore ClientStore,
	codeStore AuthorizationCodeStore,
	refreshTokenStore RefreshTokenStore,
) *AuthorizationServer {
	return &AuthorizationServer{
		config:          config,
		clientStore:     clientStore,
		codeStore:       codeStore,
		refreshTokenStore: refreshTokenStore,
	}
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func parseScope(scope string) []string {
	if scope == "" {
		return nil
	}
	return strings.Fields(scope)
}

func (s *AuthorizationServer) validateScope(clientScopes []string, requestedScope string) bool {
	if requestedScope == "" {
		return true
	}
	requested := parseScope(requestedScope)
	clientScopeMap := make(map[string]bool)
	for _, s := range clientScopes {
		clientScopeMap[s] = true
	}
	for _, r := range requested {
		if !clientScopeMap[r] {
			return false
		}
	}
	return true
}

func (s *AuthorizationServer) validateRedirectURI(client *Client, redirectURI string) bool {
	if redirectURI == "" {
		return len(client.RedirectURIs) == 1
	}
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			return true
		}
	}
	return false
}

func (s *AuthorizationServer) Authorize(req *AuthorizeRequest) (string, error) {
	if req.ResponseType != ResponseTypeCode {
		return "", ErrInvalidRequest
	}
	if req.ClientID == "" {
		return "", ErrInvalidRequest
	}

	client, err := s.clientStore.GetClient(req.ClientID)
	if err != nil {
		return "", ErrInvalidClient
	}

	if !s.validateRedirectURI(client, req.RedirectURI) {
		return "", ErrInvalidRequest
	}

	if req.Scope != "" && !s.validateScope(client.Scopes, req.Scope) {
		return "", ErrInvalidScope
	}

	scope := req.Scope
	if scope == "" && len(client.Scopes) > 0 {
		scope = strings.Join(client.Scopes, " ")
	}

	code := generateRandomString(32)
	redirectURI := req.RedirectURI
	if redirectURI == "" && len(client.RedirectURIs) == 1 {
		redirectURI = client.RedirectURIs[0]
	}

	authCode := &AuthorizationCode{
		Code:        code,
		ClientID:    client.ID,
		UserID:      req.UserID,
		Scope:       scope,
		RedirectURI: redirectURI,
		ExpiresAt:   time.Now().Add(s.config.AuthorizationCodeTTL),
		CreatedAt:   time.Now(),
	}

	if err := s.codeStore.SaveCode(authCode); err != nil {
		return "", err
	}

	return code, nil
}

func (s *AuthorizationServer) Token(req *TokenRequest) (*TokenResponse, error) {
	switch req.GrantType {
	case GrantTypeAuthorizationCode:
		return s.handleAuthorizationCode(req)
	case GrantTypeClientCredentials:
		return s.handleClientCredentials(req)
	case GrantTypeRefreshToken:
		return s.handleRefreshToken(req)
	default:
		return nil, ErrUnsupportedGrantType
	}
}

func (s *AuthorizationServer) handleAuthorizationCode(req *TokenRequest) (*TokenResponse, error) {
	if req.Code == "" {
		return nil, ErrInvalidRequest
	}

	client, err := s.clientStore.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, ErrInvalidClient
	}

	authCode, err := s.codeStore.GetCode(req.Code)
	if err != nil {
		return nil, ErrInvalidGrant
	}

	if authCode.ClientID != client.ID {
		return nil, ErrInvalidGrant
	}

	if authCode.RedirectURI != req.RedirectURI {
		return nil, ErrInvalidGrant
	}

	if err := s.codeStore.MarkCodeUsed(req.Code); err != nil {
		if err == ErrCodeUsed {
			return nil, ErrInvalidGrant
		}
		return nil, err
	}

	if req.Scope != "" && !s.validateScope(client.Scopes, req.Scope) {
		return nil, ErrInvalidScope
	}

	scope := authCode.Scope
	if req.Scope != "" {
		scope = req.Scope
	}

	return s.createTokenResponse(client.ID, authCode.UserID, scope, true)
}

func (s *AuthorizationServer) handleClientCredentials(req *TokenRequest) (*TokenResponse, error) {
	client, err := s.clientStore.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, ErrInvalidClient
	}

	if req.Scope != "" && !s.validateScope(client.Scopes, req.Scope) {
		return nil, ErrInvalidScope
	}

	scope := req.Scope
	if scope == "" && len(client.Scopes) > 0 {
		scope = strings.Join(client.Scopes, " ")
	}

	return s.createTokenResponse(client.ID, "", scope, false)
}

func (s *AuthorizationServer) handleRefreshToken(req *TokenRequest) (*TokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, ErrInvalidRequest
	}

	client, err := s.clientStore.ValidateClient(req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, ErrInvalidClient
	}

	rt, err := s.refreshTokenStore.GetToken(req.RefreshToken)
	if err != nil {
		return nil, ErrInvalidGrant
	}

	if rt.ClientID != client.ID {
		return nil, ErrInvalidGrant
	}

	if req.Scope != "" && !s.validateScope(client.Scopes, req.Scope) {
		return nil, ErrInvalidScope
	}

	if req.Scope != "" {
		requested := parseScope(req.Scope)
		original := parseScope(rt.Scope)
		originalMap := make(map[string]bool)
		for _, s := range original {
			originalMap[s] = true
		}
		for _, r := range requested {
			if !originalMap[r] {
				return nil, ErrInvalidScope
			}
		}
	}

	if err := s.refreshTokenStore.RevokeToken(req.RefreshToken); err != nil {
		return nil, err
	}

	scope := rt.Scope
	if req.Scope != "" {
		scope = req.Scope
	}

	return s.createTokenResponse(rt.ClientID, rt.UserID, scope, true)
}

func (s *AuthorizationServer) createTokenResponse(clientID, userID, scope string, includeRefresh bool) (*TokenResponse, error) {
	now := time.Now()
	expiresIn := int(s.config.AccessTokenTTL / time.Second)
	tokenID := generateRandomString(16)

	claims := &AccessTokenClaims{
		Issuer:    s.config.Issuer,
		Subject:   userID,
		Audience:  clientID,
		ExpiresAt: now.Add(s.config.AccessTokenTTL),
		IssuedAt:  now,
		ClientID:  clientID,
		Scope:     scope,
		TokenID:   tokenID,
	}

	accessToken, err := GenerateJWT(claims, s.config.SigningKey)
	if err != nil {
		return nil, err
	}

	response := &TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
		Scope:       scope,
	}

	if includeRefresh {
		refreshToken := generateRandomString(64)
		rt := &RefreshToken{
			Token:     refreshToken,
			ClientID:  clientID,
			UserID:    userID,
			Scope:     scope,
			ExpiresAt: time.Now().Add(s.config.RefreshTokenTTL),
			CreatedAt: time.Now(),
		}
		if err := s.refreshTokenStore.SaveToken(rt); err != nil {
			return nil, err
		}
		response.RefreshToken = refreshToken
	}

	return response, nil
}

func (s *AuthorizationServer) ValidateToken(token string) (*AccessTokenClaims, error) {
	return ValidateJWT(token, s.config.SigningKey)
}

func (s *AuthorizationServer) GetConfig() Config {
	return s.config
}
