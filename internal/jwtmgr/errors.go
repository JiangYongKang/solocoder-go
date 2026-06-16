package jwtmgr

import "errors"

var (
	ErrInvalidToken           = errors.New("jwtmgr: invalid token")
	ErrExpiredToken           = errors.New("jwtmgr: token has expired")
	ErrNotYetValid            = errors.New("jwtmgr: token not yet valid")
	ErrInvalidSignature       = errors.New("jwtmgr: invalid signature")
	ErrInvalidIssuer          = errors.New("jwtmgr: invalid issuer")
	ErrInvalidAudience        = errors.New("jwtmgr: invalid audience")
	ErrTokenBlacklisted       = errors.New("jwtmgr: token has been revoked")
	ErrRenewalWindowExpired   = errors.New("jwtmgr: renewal window has expired")
	ErrInvalidAlgorithm       = errors.New("jwtmgr: invalid signing algorithm")
	ErrInvalidKey             = errors.New("jwtmgr: invalid signing key")
	ErrRefreshTokenRevoked    = errors.New("jwtmgr: refresh token has been revoked")
	ErrRefreshTokenExpired    = errors.New("jwtmgr: refresh token has expired")
	ErrInvalidRefreshToken    = errors.New("jwtmgr: invalid refresh token")
	ErrMissingKey             = errors.New("jwtmgr: missing signing key")
	ErrEmptyToken             = errors.New("jwtmgr: empty token")
)
