package compressor

import (
	"bytes"
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

	patternScore := analyzePatterns(data)

	compressibility := 1.0 - (entropy / 8.0)

	totalRepeatScore := (repeatRatio*0.4 + patternScore*0.4 + compressibility*0.2)

	var dataType DataType
	if entropy > 7.5 {
		dataType = DataTypeRandom
	} else if printableRatio > 0.85 {
		dataType = DataTypeText
	} else if totalRepeatScore > 0.25 {
		dataType = DataTypeStructured
	} else if printableRatio > 0.5 && entropy < 6.5 {
		dataType = DataTypeStructured
	} else {
		dataType = DataTypeBinary
	}

	return &DataCharacteristics{
		Size:        size,
		Entropy:     entropy,
		RepeatRatio: totalRepeatScore,
		DataType:    dataType,
	}
}

func analyzePatterns(data []byte) float64 {
	size := len(data)
	if size < 8 {
		return 0
	}

	score := 0.0
	patternCount := 0

	for patternLen := 2; patternLen <= min(16, size/4); patternLen++ {
		patterns := make(map[string]int)
		maxOccurrences := 0

		for i := 0; i <= size-patternLen; i += patternLen {
			pattern := string(data[i : i+patternLen])
			patterns[pattern]++
			if patterns[pattern] > maxOccurrences {
				maxOccurrences = patterns[pattern]
			}
		}

		if maxOccurrences >= 2 {
			patternCount++
			uniquePatterns := len(patterns)
			expectedUnique := float64(size) / float64(patternLen)
			patternDensity := 1.0 - (float64(uniquePatterns) / expectedUnique)
			if patternDensity > score {
				score = patternDensity
			}
		}
	}

	if patternCount > 0 {
		score = math.Min(1.0, score+float64(patternCount)*0.2)
	}

	runScore := 0.0
	for i := 0; i < size; {
		j := i + 1
		for j < size && data[j] == data[i] {
			j++
		}
		runLen := j - i
		if runLen >= 3 {
			runScore += float64(runLen) * 0.1
		}
		i = j
	}
	runScore = math.Min(1.0, runScore/float64(size))

	score = math.Max(score, runScore)

	return score
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *Manager) NewStreamCompressor(w io.Writer) (io.WriteCloser, error) {
	if w == nil {
		return nil, ErrNilWriter
	}

	if m.config.Mode == ModeManual {
		compressor, err := m.NewCompressor()
		if err != nil {
			return nil, err
		}
		return compressor.NewCompressedWriter(w)
	}

	return &adaptiveStreamWriter{
		manager: m,
		writer:  w,
		buf:     make([]byte, 0, 4096),
		maxBuf:  4096,
	}, nil
}

func (m *Manager) NewStreamDecompressor(r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return nil, ErrNilReader
	}

	if m.config.Mode == ModeManual {
		decompressor, err := m.NewDecompressor()
		if err != nil {
			return nil, err
		}
		return decompressor.NewDecompressedReader(r)
	}

	return &adaptiveStreamReader{
		manager: m,
		reader:  r,
	}, nil
}

type adaptiveStreamWriter struct {
	manager    *Manager
	writer     io.Writer
	actual     io.WriteCloser
	buf        []byte
	maxBuf     int
	compressed bool
	closed     bool
}

func (a *adaptiveStreamWriter) Write(p []byte) (int, error) {
	if a.closed {
		return 0, fmt.Errorf("stream writer is closed")
	}

	if a.compressed {
		return a.actual.Write(p)
	}

	remaining := a.maxBuf - len(a.buf)
	if len(p) <= remaining {
		a.buf = append(a.buf, p...)
		return len(p), nil
	}

	if remaining > 0 {
		a.buf = append(a.buf, p[:remaining]...)
	}

	if err := a.initCompressor(); err != nil {
		return 0, err
	}

	if remaining < len(p) {
		n, err := a.actual.Write(p[remaining:])
		if err != nil {
			return remaining + n, err
		}
	}

	return len(p), nil
}

func (a *adaptiveStreamWriter) initCompressor() error {
	compressor, err := a.manager.autoSelectCompressor(a.buf)
	if err != nil {
		return err
	}

	actual, err := compressor.NewCompressedWriter(a.writer)
	if err != nil {
		return err
	}

	a.actual = actual
	a.compressed = true

	if len(a.buf) > 0 {
		if _, err := a.actual.Write(a.buf); err != nil {
			a.actual.Close()
			return err
		}
		a.buf = nil
	}

	return nil
}

func (a *adaptiveStreamWriter) Close() error {
	if a.closed {
		return nil
	}
	a.closed = true

	if !a.compressed {
		if err := a.initCompressor(); err != nil {
			return err
		}
	}

	return a.actual.Close()
}

type adaptiveStreamReader struct {
	manager *Manager
	reader  io.Reader
	actual  io.ReadCloser
	buf     []byte
}

func (a *adaptiveStreamReader) Read(p []byte) (int, error) {
	if a.actual != nil {
		return a.actual.Read(p)
	}

	if a.buf == nil {
		preview := make([]byte, 4096)
		n, err := io.ReadFull(a.reader, preview)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return 0, err
		}
		a.buf = preview[:n]

		if n == 0 {
			return 0, io.EOF
		}

		algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
		for _, alg := range algorithms {
			testBuf := make([]byte, len(a.buf))
			copy(testBuf, a.buf)

			decompressor, err := NewDecompressor(alg)
			if err != nil {
				continue
			}

			testReader, err := decompressor.NewDecompressedReader(bytes.NewReader(testBuf))
			if err != nil {
				continue
			}
			_, err = io.ReadAll(testReader)
			testReader.Close()

			if err == nil {
				actualDecompressor, _ := NewDecompressor(alg)
				a.actual, _ = actualDecompressor.NewDecompressedReader(
					io.MultiReader(bytes.NewReader(a.buf), a.reader),
				)
				a.buf = nil
				return a.actual.Read(p)
			}
		}

		return 0, ErrCorruptedData
	}

	return 0, nil
}

func (a *adaptiveStreamReader) Close() error {
	if a.actual != nil {
		return a.actual.Close()
	}
	return nil
}
