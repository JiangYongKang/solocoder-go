package oauth2svc

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type jwtHeader struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
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

func GenerateJWT(claims *AccessTokenClaims, signingKey []byte) (string, error) {
	header := jwtHeader{
		Alg: "HS256",
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
	signature := signHS256([]byte(signingInput), signingKey)
	encodedSignature := base64URLEncode(signature)

	return signingInput + "." + encodedSignature, nil
}

func ParseJWT(token string, signingKey []byte) (*AccessTokenClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	encodedHeader, encodedClaims, encodedSignature := parts[0], parts[1], parts[2]

	headerJSON, err := base64URLDecode(encodedHeader)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var header jwtHeader
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, ErrInvalidToken
	}

	if header.Alg != "HS256" {
		return nil, ErrInvalidToken
	}

	signingInput := encodedHeader + "." + encodedClaims

	signature, err := base64URLDecode(encodedSignature)
	if err != nil {
		return nil, ErrInvalidToken
	}

	if !verifyHS256([]byte(signingInput), signature, signingKey) {
		return nil, ErrInvalidToken
	}

	claimsJSON, err := base64URLDecode(encodedClaims)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims AccessTokenClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().After(claims.ExpiresAt) {
		return nil, ErrExpiredToken
	}

	return &claims, nil
}

func ValidateJWT(token string, signingKey []byte) (*AccessTokenClaims, error) {
	return ParseJWT(token, signingKey)
}
