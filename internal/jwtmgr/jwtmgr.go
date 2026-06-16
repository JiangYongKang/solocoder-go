package jwtmgr

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Manager struct {
	config       Config
	signingKey   SigningKey
	blacklist    Blacklist
	refreshStore RefreshTokenStore
}

func NewManager(config Config, signingKey SigningKey, blacklist Blacklist, refreshStore RefreshTokenStore) (*Manager, error) {
	if err := validateSigningKey(signingKey); err != nil {
		return nil, err
	}
	if blacklist == nil {
		blacklist = NewMemoryBlacklist(config.BlacklistCleanupInt)
	}
	if refreshStore == nil {
		refreshStore = NewMemoryRefreshStore()
	}
	return &Manager{
		config:       config,
		signingKey:   signingKey,
		blacklist:    blacklist,
		refreshStore: refreshStore,
	}, nil
}

func validateSigningKey(key SigningKey) error {
	switch key.Algorithm {
	case HS256:
		if len(key.HMACKey) == 0 {
			return ErrMissingKey
		}
	case RS256:
		if key.PrivateKey == nil {
			return ErrMissingKey
		}
	default:
		return ErrInvalidAlgorithm
	}
	return nil
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func generateRandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func signHS256(data []byte, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

func verifyHS256(data []byte, signature []byte, key []byte) bool {
	expected := signHS256(data, key)
	return hmac.Equal(expected, signature)
}

func signRS256(data []byte, privateKey *rsa.PrivateKey) ([]byte, error) {
	hashed := sha256.Sum256(data)
	return rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
}

func verifyRS256(data []byte, signature []byte, publicKey *rsa.PublicKey) bool {
	hashed := sha256.Sum256(data)
	err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], signature)
	return err == nil
}

func (m *Manager) IssueToken(claims *Claims) (string, error) {
	if claims == nil {
		return "", ErrInvalidToken
	}
	now := time.Now()
	if claims.IssuedAt.IsZero() {
		claims.IssuedAt = now
	}
	if claims.ExpiresAt.IsZero() {
		claims.ExpiresAt = now.Add(m.config.AccessTokenTTL)
	}
	if claims.Issuer == "" {
		claims.Issuer = m.config.Issuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = m.config.Audience
	}
	if claims.ID == "" {
		claims.ID = generateRandomString(16)
	}
	return m.signToken(claims)
}

func (m *Manager) IssueTokenPair(claims *Claims) (*TokenPair, error) {
	if claims == nil {
		return nil, ErrInvalidToken
	}
	now := time.Now()
	if claims.IssuedAt.IsZero() {
		claims.IssuedAt = now
	}
	if claims.ExpiresAt.IsZero() {
		claims.ExpiresAt = now.Add(m.config.AccessTokenTTL)
	}
	if claims.Issuer == "" {
		claims.Issuer = m.config.Issuer
	}
	if len(claims.Audience) == 0 {
		claims.Audience = m.config.Audience
	}
	if claims.ID == "" {
		claims.ID = generateRandomString(16)
	}
	accessToken, err := m.signToken(claims)
	if err != nil {
		return nil, err
	}
	refreshToken := generateRandomString(64)
	rt := &RefreshTokenInfo{
		Token:     refreshToken,
		TokenID:   claims.ID,
		Subject:   claims.Subject,
		ExpiresAt: now.Add(m.config.RefreshTokenTTL),
		CreatedAt: now,
		Claims:    claims,
	}
	if err := m.refreshStore.Save(rt); err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenID:      claims.ID,
		ExpiresAt:    claims.ExpiresAt,
	}, nil
}

func (m *Manager) signToken(claims *Claims) (string, error) {
	header := Header{
		Alg: m.signingKey.Algorithm,
		Typ: "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("marshal header: %w", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	encodedHeader := base64URLEncode(headerJSON)
	encodedClaims := base64URLEncode(claimsJSON)
	signingInput := encodedHeader + "." + encodedClaims
	var signature []byte
	switch m.signingKey.Algorithm {
	case HS256:
		signature = signHS256([]byte(signingInput), m.signingKey.HMACKey)
	case RS256:
		signature, err = signRS256([]byte(signingInput), m.signingKey.PrivateKey)
		if err != nil {
			return "", fmt.Errorf("sign token: %w", err)
		}
	default:
		return "", ErrInvalidAlgorithm
	}
	encodedSignature := base64URLEncode(signature)
	return signingInput + "." + encodedSignature, nil
}

func (m *Manager) ValidateToken(token string, options ValidationOptions) (*Claims, error) {
	if token == "" {
		return nil, ErrEmptyToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	encodedHeader, encodedClaims, encodedSignature := parts[0], parts[1], parts[2]
	headerJSON, err := base64URLDecode(encodedHeader)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header Header
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, ErrInvalidToken
	}
	if header.Alg != m.signingKey.Algorithm {
		return nil, ErrInvalidAlgorithm
	}
	signingInput := encodedHeader + "." + encodedClaims
	signature, err := base64URLDecode(encodedSignature)
	if err != nil {
		return nil, ErrInvalidToken
	}
	if !m.verifySignature(signingInput, signature, header.Alg) {
		return nil, ErrInvalidSignature
	}
	claimsJSON, err := base64URLDecode(encodedClaims)
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if err := m.validateClaims(&claims, options); err != nil {
		return nil, err
	}
	blacklisted, err := m.blacklist.Contains(claims.ID)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return nil, ErrTokenBlacklisted
	}
	return &claims, nil
}

func (m *Manager) verifySignature(signingInput string, signature []byte, alg Algorithm) bool {
	switch alg {
	case HS256:
		return verifyHS256([]byte(signingInput), signature, m.signingKey.HMACKey)
	case RS256:
		pubKey := m.signingKey.PublicKey
		if pubKey == nil && m.signingKey.PrivateKey != nil {
			pubKey = &m.signingKey.PrivateKey.PublicKey
		}
		if pubKey == nil {
			return false
		}
		return verifyRS256([]byte(signingInput), signature, pubKey)
	default:
		return false
	}
}

func (m *Manager) validateClaims(claims *Claims, options ValidationOptions) error {
	now := time.Now()
	if options.ValidateExpiry && !claims.ExpiresAt.IsZero() {
		if now.After(claims.ExpiresAt) {
			return ErrExpiredToken
		}
	}
	if options.ValidateNotBefore && !claims.NotBefore.IsZero() {
		if now.Before(claims.NotBefore) {
			return ErrNotYetValid
		}
	}
	if options.ValidateIssuer && options.ExpectedIssuer != "" {
		if claims.Issuer != options.ExpectedIssuer {
			return ErrInvalidIssuer
		}
	}
	if options.ValidateAudience && len(options.ExpectedAudience) > 0 {
		if !audienceContains(claims.Audience, options.ExpectedAudience) {
			return ErrInvalidAudience
		}
	}
	return nil
}

func audienceContains(tokenAudience, expectedAudience []string) bool {
	for _, expected := range expectedAudience {
		found := false
		for _, aud := range tokenAudience {
			if aud == expected {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func (m *Manager) RevokeToken(tokenID string) error {
	return m.blacklist.Add(tokenID, m.config.BlacklistTTL)
}

func (m *Manager) RenewToken(token string, options ValidationOptions) (string, error) {
	opts := options
	opts.ValidateExpiry = false
	claims, err := m.ValidateToken(token, opts)
	if err != nil {
		return "", err
	}
	now := time.Now()
	renewalDeadline := claims.ExpiresAt.Add(m.config.RenewalWindow)
	if now.After(renewalDeadline) {
		return "", ErrRenewalWindowExpired
	}
	if m.config.AutoBlacklistOld {
		if err := m.RevokeToken(claims.ID); err != nil {
			return "", err
		}
	}
	newClaims := *claims
	newClaims.ID = generateRandomString(16)
	newClaims.IssuedAt = now
	newClaims.ExpiresAt = now.Add(m.config.AccessTokenTTL)
	return m.signToken(&newClaims)
}

func (m *Manager) RefreshAccessToken(refreshToken string) (*TokenPair, error) {
	if refreshToken == "" {
		return nil, ErrInvalidRefreshToken
	}
	rt, err := m.refreshStore.Get(refreshToken)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var baseClaims *Claims
	if rt.Claims != nil {
		baseClaims = rt.Claims
	} else {
		baseClaims = &Claims{}
	}
	newClaims := *baseClaims
	newClaims.ID = generateRandomString(16)
	newClaims.Subject = rt.Subject
	newClaims.Issuer = m.config.Issuer
	newClaims.Audience = m.config.Audience
	newClaims.IssuedAt = now
	newClaims.ExpiresAt = now.Add(m.config.AccessTokenTTL)
	newAccessToken, err := m.signToken(&newClaims)
	if err != nil {
		return nil, err
	}
	pair := &TokenPair{
		AccessToken: newAccessToken,
		TokenID:     newClaims.ID,
		ExpiresAt:   newClaims.ExpiresAt,
	}
	if m.config.RefreshTokenRotation {
		if err := m.refreshStore.Revoke(refreshToken); err != nil {
			return nil, err
		}
		newRefreshToken := generateRandomString(64)
		newRT := &RefreshTokenInfo{
			Token:     newRefreshToken,
			TokenID:   newClaims.ID,
			Subject:   rt.Subject,
			ExpiresAt: now.Add(m.config.RefreshTokenTTL),
			CreatedAt: now,
			Claims:    &newClaims,
		}
		if err := m.refreshStore.Save(newRT); err != nil {
			return nil, err
		}
		pair.RefreshToken = newRefreshToken
	} else {
		pair.RefreshToken = refreshToken
	}
	return pair, nil
}

func (m *Manager) Close() error {
	if err := m.blacklist.Close(); err != nil {
		return err
	}
	return m.refreshStore.Close()
}

func (m *Manager) GetConfig() Config {
	return m.config
}
