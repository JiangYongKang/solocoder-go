package compressor

import (
	"errors"
	"io"
)

var (
	ErrUnsupportedAlgorithm = errors.New("unsupported compression algorithm")
	ErrInvalidCompressionLevel = errors.New("invalid compression level")
	ErrNilReader = errors.New("nil reader")
	ErrNilWriter = errors.New("nil writer")
	ErrEmptyData = errors.New("empty data")
	ErrCorruptedData = errors.New("corrupted compressed data")
	ErrInvalidMode = errors.New("invalid selection mode")
)

type Algorithm string

const (
	AlgorithmGzip   Algorithm = "gzip"
	AlgorithmSnappy Algorithm = "snappy"
	AlgorithmLZ4    Algorithm = "lz4"
)

type CompressionLevel int

const (
	LevelFastest    CompressionLevel = 1
	LevelFast       CompressionLevel = 2
	LevelDefault    CompressionLevel = 3
	LevelBetter     CompressionLevel = 4
	LevelBest       CompressionLevel = 5
)

type SelectionMode string

const (
	ModeManual SelectionMode = "manual"
	ModeAuto   SelectionMode = "auto"
)

type Config struct {
	Algorithm         Algorithm
	Level             CompressionLevel
	Mode              SelectionMode
	AutoSpeedRatio    float64
}

type Compressor interface {
	Compress(data []byte) ([]byte, error)
	NewCompressedWriter(w io.Writer) (io.WriteCloser, error)
	Algorithm() Algorithm
	Level() CompressionLevel
}

type Decompressor interface {
	Decompress(data []byte) ([]byte, error)
	NewDecompressedReader(r io.Reader) (io.ReadCloser, error)
	Algorithm() Algorithm
}

type CompressionResult struct {
	Algorithm         Algorithm
	Level             CompressionLevel
	OriginalSize      int
	CompressedSize    int
	CompressionRatio  float64
}

type DataCharacteristics struct {
	Size              int
	Entropy           float64
	RepeatRatio       float64
	DataType          DataType
}

type DataType string

const (
	DataTypeText      DataType = "text"
	DataTypeBinary    DataType = "binary"
	DataTypeRandom    DataType = "random"
	DataTypeStructured DataType = "structured"
)

func DefaultConfig() Config {
	return Config{
		Algorithm:      AlgorithmGzip,
		Level:          LevelDefault,
		Mode:           ModeManual,
		AutoSpeedRatio: 0.5,
	}
}

func (l CompressionLevel) Validate() bool {
	return l >= LevelFastest && l <= LevelBest
}

func (a Algorithm) Validate() bool {
	switch a {
	case AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4:
		return true
	default:
		return false
	}
}

func (m SelectionMode) Validate() bool {
	switch m {
	case ModeManual, ModeAuto:
		return true
	default:
		return false
	}
}
