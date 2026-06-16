package datadedup

type Config struct {
	Mode               DedupMode
	HashAlgorithm      HashAlgorithm
	SimilarityAlgo     SimilarityAlgorithm
	SimilarityThreshold float64
	ChunkStrategy      ChunkStrategy
	ChunkSize          int
	ContentBoundary    byte
	MinChunkSize       int
	MaxChunkSize       int
	PersistPath        string
	AutoPersist        bool
	AutoPersistCount   int
	SimilarityCalc     SimilarityCalculator
	Chunker            Chunker
	Persister          PersistIndex
	HashProvider       HashProvider
}

func DefaultConfig() Config {
	return Config{
		Mode:                DedupModeExact,
		HashAlgorithm:       HashAlgorithmSHA256,
		SimilarityAlgo:      SimilarityAlgorithmSimHash,
		SimilarityThreshold: 0.85,
		ChunkStrategy:       ChunkStrategyFixedSize,
		ChunkSize:           4096,
		ContentBoundary:     '\n',
		MinChunkSize:        1024,
		MaxChunkSize:        16384,
		AutoPersist:         false,
		AutoPersistCount:    1000,
	}
}

func (c Config) Validate() error {
	if c.Mode != DedupModeExact && c.Mode != DedupModeFuzzy && c.Mode != DedupModeChunked {
		return ErrInvalidDedupMode
	}

	if c.SimilarityThreshold < 0 || c.SimilarityThreshold > 1 {
		return ErrInvalidThreshold
	}

	if c.Mode == DedupModeChunked {
		if c.ChunkSize <= 0 {
			return ErrInvalidChunkSize
		}
		if c.ChunkStrategy == ChunkStrategyContent {
			if c.MinChunkSize <= 0 {
				return ErrInvalidChunkSize
			}
			if c.MaxChunkSize < c.MinChunkSize {
				return ErrInvalidChunkSize
			}
		}
	}

	return nil
}

func (c Config) WithMode(mode DedupMode) Config {
	c.Mode = mode
	return c
}

func (c Config) WithHashAlgorithm(algo HashAlgorithm) Config {
	c.HashAlgorithm = algo
	return c
}

func (c Config) WithSimilarityThreshold(threshold float64) Config {
	c.SimilarityThreshold = threshold
	return c
}

func (c Config) WithChunkStrategy(strategy ChunkStrategy) Config {
	c.ChunkStrategy = strategy
	return c
}

func (c Config) WithChunkSize(size int) Config {
	c.ChunkSize = size
	return c
}

func (c Config) WithPersistPath(path string) Config {
	c.PersistPath = path
	c.AutoPersist = true
	return c
}
