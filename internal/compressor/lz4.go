package compressor

import (
	"bytes"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

type lz4Compressor struct {
	level lz4.CompressionLevel
	origLevel CompressionLevel
}

type lz4Decompressor struct{}

var lz4LevelMap = map[CompressionLevel]lz4.CompressionLevel{
	LevelFastest: lz4.Fast,
	LevelFast:    lz4.Level1,
	LevelDefault: lz4.Level3,
	LevelBetter:  lz4.Level6,
	LevelBest:    lz4.Level9,
}

func NewLZ4Compressor(level CompressionLevel) (Compressor, error) {
	if !level.Validate() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCompressionLevel, level)
	}
	return &lz4Compressor{
		level: lz4LevelMap[level],
		origLevel: level,
	}, nil
}

func NewLZ4Decompressor() Decompressor {
	return &lz4Decompressor{}
}

func (l *lz4Compressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	var buf bytes.Buffer
	writer := lz4.NewWriter(&buf)
	if err := writer.Apply(lz4.CompressionLevelOption(l.level)); err != nil {
		return nil, fmt.Errorf("lz4 apply level error: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("lz4 write error: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("lz4 close error: %w", err)
	}

	return buf.Bytes(), nil
}

func (l *lz4Compressor) NewCompressedWriter(w io.Writer) (io.WriteCloser, error) {
	if w == nil {
		return nil, ErrNilWriter
	}
	writer := lz4.NewWriter(w)
	if err := writer.Apply(lz4.CompressionLevelOption(l.level)); err != nil {
		return nil, fmt.Errorf("lz4 writer option error: %w", err)
	}
	return &lz4WriterCloser{writer: writer}, nil
}

func (l *lz4Compressor) Algorithm() Algorithm {
	return AlgorithmLZ4
}

func (l *lz4Compressor) Level() CompressionLevel {
	return l.origLevel
}

type lz4WriterCloser struct {
	writer *lz4.Writer
}

func (l *lz4WriterCloser) Write(p []byte) (int, error) {
	return l.writer.Write(p)
}

func (l *lz4WriterCloser) Close() error {
	return l.writer.Close()
}

func (l *lz4Decompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	reader := lz4.NewReader(bytes.NewReader(data))

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptedData, err)
	}

	return buf.Bytes(), nil
}

func (l *lz4Decompressor) NewDecompressedReader(r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return nil, ErrNilReader
	}
	reader := lz4.NewReader(r)
	return &lz4ReaderCloser{reader: reader}, nil
}

func (l *lz4Decompressor) Algorithm() Algorithm {
	return AlgorithmLZ4
}

type lz4ReaderCloser struct {
	reader *lz4.Reader
}

func (l *lz4ReaderCloser) Read(p []byte) (int, error) {
	return l.reader.Read(p)
}

func (l *lz4ReaderCloser) Close() error {
	return nil
}
