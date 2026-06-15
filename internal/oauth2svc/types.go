package oauth2svc

import (
	"errors"
	"time"
)

var (
	ErrInvalidClient       = errors.New("oauth2: invalid client")
	ErrInvalidGrant      = errors.New("oauth2: invalid grant")
	ErrInvalidScope     = errors.New("oauth2: invalid scope")
	ErrInvalidRequest   = errors.New("oauth2: invalid request")
	ErrUnauthorizedClient = errors.New("oauth2: unauthorized client")
	ErrUnsupportedGrantType = errors.New("oauth2: unsupported grant type")
	ErrInvalidToken    = errors.New("oauth2: invalid token")
	ErrExpiredToken   = errors.New("oauth2: expired token")
	ErrCodeUsed       = errors.New("oauth2: authorization code already used")
	ErrCodeExpired      = errors.New("oauth2: authorization code expired")
	ErrRefreshTokenRevoked = errors.New("oauth2: refresh token revoked")
)

type GrantType string

const (
	GrantTypeAuthorizationCode GrantType = "authorization_code"
	GrantTypeClientCredentials GrantType = "client_credentials"
	GrantTypeRefreshToken GrantType = "refresh_token"
)

type ResponseType string

const (
	ResponseTypeCode ResponseType = "code"
)

type Client struct {
	ID           string
	Secret       string
	RedirectURIs []string
	Scopes       []string
}

type AuthorizationCode struct {
	Code        string
	ClientID    string
	UserID      string
	Scope       string
	RedirectURI string
	ExpiresAt   time.Time
	Used        bool
	CreatedAt   time.Time
}

type Token struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int
	RefreshToken string
	Scope        string
}

type RefreshToken struct {
	Token       string
	ClientID    string
	UserID      string
	Scope       string
	ExpiresAt   time.Time
	Revoked     bool
	CreatedAt   time.Time
}

type AccessTokenClaims struct {
	Issuer    string    `json:"iss"`
	Subject   string    `json:"sub"`
	Audience  string    `json:"aud"`
	ExpiresAt time.Time `json:"exp"`
	IssuedAt  time.Time `json:"iat"`
	ClientID  string    `json:"cid"`
	Scope     string    `json:"scope"`
	TokenID   string    `json:"jti"`
}

type Config struct {
	Issuer              string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	AuthorizationCodeTTL time.Duration
	RefreshTokenRotation  bool
	SigningKey       []byte
}

func DefaultConfig() Config {
	return Config{
		Issuer:              "oauth2svc",
		AccessTokenTTL:      time.Hour,
		RefreshTokenTTL:     7 * 24 * time.Hour,
		RefreshTokenRotation:  true,
		AuthorizationCodeTTL: 10 * time.Minute,
		SigningKey:       []byte("default-signing-key-change-me"),
	}
}

type AuthorizeRequest struct {
	ResponseType ResponseType
	ClientID     string
	RedirectURI string
	Scope        string
	State        string
	UserID       string
}

type TokenRequest struct {
	GrantType    GrantType
	ClientID     string
	ClientSecret string
	Code         string
	RedirectURI  string
	RefreshToken string
	Scope        string
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope"`
}
