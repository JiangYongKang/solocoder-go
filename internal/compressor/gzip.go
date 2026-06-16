package compressor

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
)

type gzipCompressor struct {
	level     int
	origLevel CompressionLevel
}

type gzipDecompressor struct{}

var gzipLevelMap = map[CompressionLevel]int{
	LevelFastest: gzip.BestSpeed,
	LevelFast:    gzip.BestSpeed,
	LevelDefault: gzip.DefaultCompression,
	LevelBetter:  7,
	LevelBest:    gzip.BestCompression,
}

func NewGzipCompressor(level CompressionLevel) (Compressor, error) {
	if !level.Validate() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCompressionLevel, level)
	}
	return &gzipCompressor{
		level:     gzipLevelMap[level],
		origLevel: level,
	}, nil
}

func NewGzipDecompressor() Decompressor {
	return &gzipDecompressor{}
}

func (g *gzipCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, g.level)
	if err != nil {
		return nil, fmt.Errorf("gzip compress error: %w", err)
	}

	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil, fmt.Errorf("gzip write error: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("gzip close error: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *gzipCompressor) NewCompressedWriter(w io.Writer) (io.WriteCloser, error) {
	if w == nil {
		return nil, ErrNilWriter
	}
	writer, err := gzip.NewWriterLevel(w, g.level)
	if err != nil {
		return nil, fmt.Errorf("gzip writer error: %w", err)
	}
	return &gzipWriterCloser{writer: writer}, nil
}

func (g *gzipCompressor) Algorithm() Algorithm {
	return AlgorithmGzip
}

func (g *gzipCompressor) Level() CompressionLevel {
	return g.origLevel
}

type gzipWriterCloser struct {
	writer *gzip.Writer
}

func (g *gzipWriterCloser) Write(p []byte) (int, error) {
	return g.writer.Write(p)
}

func (g *gzipWriterCloser) Close() error {
	return g.writer.Close()
}

func (g *gzipDecompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptedData, err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("gzip decompress error: %w", err)
	}

	return buf.Bytes(), nil
}

func (g *gzipDecompressor) NewDecompressedReader(r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return nil, ErrNilReader
	}
	reader, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader error: %w", err)
	}
	return &gzipReaderCloser{reader: reader}, nil
}

func (g *gzipDecompressor) Algorithm() Algorithm {
	return AlgorithmGzip
}

type gzipReaderCloser struct {
	reader *gzip.Reader
}

func (g *gzipReaderCloser) Read(p []byte) (int, error) {
	return g.reader.Read(p)
}

func (g *gzipReaderCloser) Close() error {
	return g.reader.Close()
}
