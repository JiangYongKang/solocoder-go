package compressor

import (
	"fmt"
	"io"
	"math"
)

type Manager struct {
	config Config
}

func NewManager(cfg Config) (*Manager, error) {
	if !cfg.Algorithm.Validate() {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, cfg.Algorithm)
	}
	if !cfg.Level.Validate() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCompressionLevel, cfg.Level)
	}
	if !cfg.Mode.Validate() {
		return nil, fmt.Errorf("%w: %s", ErrInvalidMode, cfg.Mode)
	}
	if cfg.AutoSpeedRatio < 0 || cfg.AutoSpeedRatio > 1 {
		cfg.AutoSpeedRatio = 0.5
	}
	return &Manager{
		config: cfg,
	}, nil
}

func (m *Manager) NewCompressor() (Compressor, error) {
	return NewCompressor(m.config.Algorithm, m.config.Level)
}

func (m *Manager) NewDecompressor() (Decompressor, error) {
	return NewDecompressor(m.config.Algorithm)
}

func (m *Manager) Compress(data []byte) ([]byte, *CompressionResult, error) {
	if len(data) == 0 {
		return nil, nil, ErrEmptyData
	}

	compressor, err := m.selectCompressor(data)
	if err != nil {
		return nil, nil, err
	}

	compressed, err := compressor.Compress(data)
	if err != nil {
		return nil, nil, err
	}

	result := &CompressionResult{
		Algorithm:        compressor.Algorithm(),
		Level:            compressor.Level(),
		OriginalSize:     len(data),
		CompressedSize:   len(compressed),
		CompressionRatio: float64(len(compressed)) / float64(len(data)),
	}

	return compressed, result, nil
}

func (m *Manager) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	if m.config.Mode == ModeManual {
		decompressor, err := m.NewDecompressor()
		if err != nil {
			return nil, err
		}
		return decompressor.Decompress(data)
	}

	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
	var lastErr error
	for _, alg := range algorithms {
		decompressor, err := NewDecompressor(alg)
		if err != nil {
			lastErr = err
			continue
		}
		result, err := decompressor.Decompress(data)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("%w: tried all algorithms, last error: %v", ErrCorruptedData, lastErr)
}

func (m *Manager) selectCompressor(data []byte) (Compressor, error) {
	if m.config.Mode == ModeManual {
		return m.NewCompressor()
	}

	return m.autoSelectCompressor(data)
}

func (m *Manager) autoSelectCompressor(data []byte) (Compressor, error) {
	characteristics := AnalyzeData(data)
	speedRatio := m.config.AutoSpeedRatio

	var algorithm Algorithm
	var level CompressionLevel

	switch characteristics.DataType {
	case DataTypeRandom:
		algorithm = AlgorithmSnappy
		level = LevelFastest
	case DataTypeText:
		if speedRatio > 0.7 {
			algorithm = AlgorithmLZ4
			level = LevelFast
		} else if speedRatio > 0.3 {
			algorithm = AlgorithmGzip
			level = LevelDefault
		} else {
			algorithm = AlgorithmGzip
			level = LevelBest
		}
	case DataTypeStructured:
		if speedRatio > 0.6 {
			algorithm = AlgorithmLZ4
			level = LevelDefault
		} else {
			algorithm = AlgorithmGzip
			level = LevelBetter
		}
	case DataTypeBinary:
		if speedRatio > 0.5 {
			algorithm = AlgorithmSnappy
			level = LevelDefault
		} else {
			algorithm = AlgorithmLZ4
			level = LevelBetter
		}
	default:
		if speedRatio > 0.5 {
			algorithm = AlgorithmSnappy
		} else {
			algorithm = AlgorithmGzip
		}
		level = LevelDefault
	}

	if characteristics.Size < 1024 {
		level = LevelFastest
	} else if characteristics.Size > 10*1024*1024 {
		if speedRatio > 0.5 {
			level = LevelFastest
		}
	}

	if characteristics.RepeatRatio > 0.6 {
		if speedRatio > 0.5 {
			algorithm = AlgorithmLZ4
		} else {
			algorithm = AlgorithmGzip
		}
	}

	return NewCompressor(algorithm, level)
}

func NewCompressor(alg Algorithm, level CompressionLevel) (Compressor, error) {
	if !level.Validate() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCompressionLevel, level)
	}

	switch alg {
	case AlgorithmGzip:
		return NewGzipCompressor(level)
	case AlgorithmSnappy:
		return NewSnappyCompressor(level)
	case AlgorithmLZ4:
		return NewLZ4Compressor(level)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

func NewDecompressor(alg Algorithm) (Decompressor, error) {
	switch alg {
	case AlgorithmGzip:
		return NewGzipDecompressor(), nil
	case AlgorithmSnappy:
		return NewSnappyDecompressor(), nil
	case AlgorithmLZ4:
		return NewLZ4Decompressor(), nil
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedAlgorithm, alg)
	}
}

func AnalyzeData(data []byte) *DataCharacteristics {
	size := len(data)
	if size == 0 {
		return &DataCharacteristics{
			Size:        0,
			Entropy:     0,
			RepeatRatio: 0,
			DataType:    DataTypeRandom,
		}
	}

	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / float64(size)
		entropy -= p * math.Log2(p)
	}

	repeatCount := 0
	for i := 0; i < size-1; i++ {
		if data[i] == data[i+1] {
			repeatCount++
		}
	}
	repeatRatio := float64(repeatCount) / float64(size-1)

	printableCount := 0
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t' {
			printableCount++
		}
	}
	printableRatio := float64(printableCount) / float64(size)

	var dataType DataType
	if entropy > 7.5 {
		dataType = DataTypeRandom
	} else if printableRatio > 0.85 {
		dataType = DataTypeText
	} else if repeatRatio > 0.3 {
		dataType = DataTypeStructured
	} else {
		dataType = DataTypeBinary
	}

	return &DataCharacteristics{
		Size:        size,
		Entropy:     entropy,
		RepeatRatio: repeatRatio,
		DataType:    dataType,
	}
}

func (m *Manager) NewStreamCompressor(w io.Writer) (io.WriteCloser, error) {
	compressor, err := m.NewCompressor()
	if err != nil {
		return nil, err
	}
	return compressor.NewCompressedWriter(w)
}

func (m *Manager) NewStreamDecompressor(r io.Reader) (io.ReadCloser, error) {
	decompressor, err := m.NewDecompressor()
	if err != nil {
		return nil, err
	}
	return decompressor.NewDecompressedReader(r)
}
