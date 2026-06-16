package datadedup

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"hash"
)

type hashProvider struct {
	algo     HashAlgorithm
	newHash  func() hash.Hash
}

func NewHashProvider(algo HashAlgorithm) (HashProvider, error) {
	var newHash func() hash.Hash

	switch algo {
	case HashAlgorithmSHA256:
		newHash = sha256.New
	case HashAlgorithmSHA1:
		newHash = sha1.New
	case HashAlgorithmMD5:
		newHash = md5.New
	default:
		return nil, ErrUnsupportedHashAlgo
	}

	return &hashProvider{
		algo:    algo,
		newHash: newHash,
	}, nil
}

func (h *hashProvider) Hash(data []byte) (Fingerprint, error) {
	if len(data) == 0 {
		return "", ErrEmptyData
	}

	hasher := h.newHash()
	if _, err := hasher.Write(data); err != nil {
		return "", err
	}

	fp := hex.EncodeToString(hasher.Sum(nil))
	return Fingerprint(fp), nil
}

func (h *hashProvider) Algorithm() HashAlgorithm {
	return h.algo
}
