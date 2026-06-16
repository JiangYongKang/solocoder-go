package jwtmgr

import (
	"crypto/rsa"
	"encoding/json"
	"time"
)

type Algorithm string

const (
	HS256 Algorithm = "HS256"
	RS256 Algorithm = "RS256"
)

type Header struct {
	Alg Algorithm `json:"alg"`
	Typ string    `json:"typ"`
}

type Claims struct {
	Issuer    string                 `json:"iss"`
	Subject   string                 `json:"sub"`
	Audience  []string               `json:"aud"`
	ExpiresAt time.Time              `json:"exp"`
	NotBefore time.Time              `json:"nbf,omitempty"`
	IssuedAt  time.Time              `json:"iat"`
	ID        string                 `json:"jti"`
	Custom    map[string]interface{} `json:"-"`
}

func (c *Claims) MarshalJSON() ([]byte, error) {
	m := make(map[string]interface{})
	if c.Issuer != "" {
		m["iss"] = c.Issuer
	}
	if c.Subject != "" {
		m["sub"] = c.Subject
	}
	if len(c.Audience) > 0 {
		if len(c.Audience) == 1 {
			m["aud"] = c.Audience[0]
		} else {
			m["aud"] = c.Audience
		}
	}
	if !c.ExpiresAt.IsZero() {
		m["exp"] = c.ExpiresAt.Unix()
	}
	if !c.NotBefore.IsZero() {
		m["nbf"] = c.NotBefore.Unix()
	}
	if !c.IssuedAt.IsZero() {
		m["iat"] = c.IssuedAt.Unix()
	}
	if c.ID != "" {
		m["jti"] = c.ID
	}
	for k, v := range c.Custom {
		m[k] = v
	}
	return json.Marshal(m)
}

func (c *Claims) UnmarshalJSON(data []byte) error {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	c.Custom = make(map[string]interface{})
	for k, v := range m {
		switch k {
		case "iss":
			if s, ok := v.(string); ok {
				c.Issuer = s
			}
		case "sub":
			if s, ok := v.(string); ok {
				c.Subject = s
			}
		case "aud":
			switch vv := v.(type) {
			case string:
				c.Audience = []string{vv}
			case []interface{}:
				for _, a := range vv {
					if s, ok := a.(string); ok {
						c.Audience = append(c.Audience, s)
					}
				}
			}
		case "exp":
			if f, ok := v.(float64); ok {
				c.ExpiresAt = time.Unix(int64(f), 0)
			}
		case "nbf":
			if f, ok := v.(float64); ok {
				c.NotBefore = time.Unix(int64(f), 0)
			}
		case "iat":
			if f, ok := v.(float64); ok {
				c.IssuedAt = time.Unix(int64(f), 0)
			}
		case "jti":
			if s, ok := v.(string); ok {
				c.ID = s
			}
		default:
			c.Custom[k] = v
		}
	}
	return nil
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	TokenID      string
	ExpiresAt    time.Time
}

type RefreshTokenInfo struct {
	Token     string
	TokenID   string
	Subject   string
	ExpiresAt time.Time
	CreatedAt time.Time
	Revoked   bool
	Claims    *Claims
}

type SigningKey struct {
	Algorithm Algorithm
	HMACKey   []byte
	PublicKey  *rsa.PublicKey
	PrivateKey *rsa.PrivateKey
}

type ValidationOptions struct {
	ExpectedIssuer   string
	ExpectedAudience []string
	ValidateExpiry   bool
	ValidateIssuer   bool
	ValidateAudience bool
	ValidateNotBefore bool
}

func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		ValidateExpiry:    true,
		ValidateIssuer:    true,
		ValidateAudience:  true,
		ValidateNotBefore: true,
	}
}
