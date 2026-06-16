package datadedup

type Fingerprint string

type HashAlgorithm string

const (
	HashAlgorithmSHA256 HashAlgorithm = "sha256"
	HashAlgorithmSHA1   HashAlgorithm = "sha1"
	HashAlgorithmMD5    HashAlgorithm = "md5"
)

type SimilarityAlgorithm string

const (
	SimilarityAlgorithmSimHash SimilarityAlgorithm = "simhash"
	SimilarityAlgorithmMinHash SimilarityAlgorithm = "minhash"
)

type ChunkStrategy string

const (
	ChunkStrategyFixedSize ChunkStrategy = "fixed_size"
	ChunkStrategyContent   ChunkStrategy = "content_based"
)

type DedupMode string

const (
	DedupModeExact   DedupMode = "exact"
	DedupModeFuzzy   DedupMode = "fuzzy"
	DedupModeChunked DedupMode = "chunked"
)

type FingerprintIndex map[Fingerprint]bool

type Chunk struct {
	Data        []byte
	Offset      int64
	Fingerprint Fingerprint
}

type DedupResult struct {
	IsDuplicate   bool
	MatchedFPs    []Fingerprint
	Similarity    float64
	MatchedChunks []Chunk
}

type SimilarityCalculator interface {
	Calculate(data []byte) (Fingerprint, error)
	Similarity(fp1, fp2 Fingerprint) (float64, error)
	Algorithm() SimilarityAlgorithm
}

type Chunker interface {
	Chunk(data []byte) ([]Chunk, error)
	Strategy() ChunkStrategy
}

type PersistIndex interface {
	Save(index FingerprintIndex, path string) error
	Load(path string) (FingerprintIndex, error)
	Append(fp Fingerprint, path string) error
	Verify(path string) (bool, error)
}

type HashProvider interface {
	Hash(data []byte) (Fingerprint, error)
	Algorithm() HashAlgorithm
}

type DedupEngine interface {
	Check(data []byte) (*DedupResult, error)
	Add(data []byte) error
	CheckAndAdd(data []byte) (*DedupResult, error)
	Contains(data []byte) (bool, error)
	Delete(data []byte) error
	Clear() error
	Count() int
	Save(path string) error
	Load(path string) error
	Close() error
}
