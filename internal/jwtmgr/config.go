package jwtmgr

import (
	"crypto/rsa"
	"time"
)

type Config struct {
	Issuer              string
	Audience            []string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	RenewalWindow       time.Duration
	AutoBlacklistOld    bool
	BlacklistTTL        time.Duration
	BlacklistCleanupInt time.Duration
	RefreshTokenRotation bool
}

func DefaultConfig() Config {
	return Config{
		Issuer:               "jwtmgr",
		Audience:             []string{"api"},
		AccessTokenTTL:       time.Hour,
		RefreshTokenTTL:      7 * 24 * time.Hour,
		RenewalWindow:        5 * time.Minute,
		AutoBlacklistOld:     true,
		BlacklistTTL:         24 * time.Hour,
		BlacklistCleanupInt:  time.Hour,
		RefreshTokenRotation: true,
	}
}

func NewHS256Config(hmacKey []byte) SigningKey {
	return SigningKey{
		Algorithm: HS256,
		HMACKey:   hmacKey,
	}
}

func NewRS256Config(privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey) SigningKey {
	return SigningKey{
		Algorithm:  RS256,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
}
