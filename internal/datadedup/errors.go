package datadedup

import "errors"

var (
	ErrEmptyData               = errors.New("datadedup: empty data")
	ErrInvalidConfig           = errors.New("datadedup: invalid config")
	ErrUnsupportedHashAlgo     = errors.New("datadedup: unsupported hash algorithm")
	ErrUnsupportedSimAlgo      = errors.New("datadedup: unsupported similarity algorithm")
	ErrUnsupportedChunkStrat   = errors.New("datadedup: unsupported chunk strategy")
	ErrInvalidThreshold        = errors.New("datadedup: invalid similarity threshold, must be between 0 and 1")
	ErrInvalidChunkSize        = errors.New("datadedup: invalid chunk size, must be positive")
	ErrEngineClosed            = errors.New("datadedup: engine is closed")
	ErrNilSimilarityCalculator = errors.New("datadedup: similarity calculator is nil")
	ErrNilChunker              = errors.New("datadedup: chunker is nil")
	ErrNilPersister            = errors.New("datadedup: persister is nil")
	ErrNilHashProvider         = errors.New("datadedup: hash provider is nil")
	ErrInvalidFingerprint      = errors.New("datadedup: invalid fingerprint format")
	ErrPersistFileNotExist     = errors.New("datadedup: persist file does not exist")
	ErrPersistCorrupted        = errors.New("datadedup: persist file is corrupted")
	ErrChecksumMismatch        = errors.New("datadedup: checksum mismatch")
	ErrInvalidDedupMode        = errors.New("datadedup: invalid deduplication mode")
)
