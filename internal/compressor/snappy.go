package compressor

import (
	"bytes"
	"fmt"
	"io"

	"github.com/golang/snappy"
)

type snappyCompressor struct {
	level CompressionLevel
}

type snappyDecompressor struct{}

func NewSnappyCompressor(level CompressionLevel) (Compressor, error) {
	if !level.Validate() {
		return nil, fmt.Errorf("%w: %d", ErrInvalidCompressionLevel, level)
	}
	return &snappyCompressor{
		level: level,
	}, nil
}

func NewSnappyDecompressor() Decompressor {
	return &snappyDecompressor{}
}

func (s *snappyCompressor) Compress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	compressed := snappy.Encode(nil, data)
	return compressed, nil
}

func (s *snappyCompressor) NewCompressedWriter(w io.Writer) (io.WriteCloser, error) {
	if w == nil {
		return nil, ErrNilWriter
	}
	writer := snappy.NewBufferedWriter(w)
	return &snappyWriterCloser{writer: writer}, nil
}

func (s *snappyCompressor) Algorithm() Algorithm {
	return AlgorithmSnappy
}

func (s *snappyCompressor) Level() CompressionLevel {
	return s.level
}

type snappyWriterCloser struct {
	writer *snappy.Writer
}

func (s *snappyWriterCloser) Write(p []byte) (int, error) {
	return s.writer.Write(p)
}

func (s *snappyWriterCloser) Close() error {
	return s.writer.Close()
}

func (s *snappyDecompressor) Decompress(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	decoded, err := snappy.Decode(nil, data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCorruptedData, err)
	}

	return decoded, nil
}

func (s *snappyDecompressor) NewDecompressedReader(r io.Reader) (io.ReadCloser, error) {
	if r == nil {
		return nil, ErrNilReader
	}
	reader := snappy.NewReader(r)
	return &snappyReaderCloser{reader: reader}, nil
}

func (s *snappyDecompressor) Algorithm() Algorithm {
	return AlgorithmSnappy
}

type snappyReaderCloser struct {
	reader *snappy.Reader
}

func (s *snappyReaderCloser) Read(p []byte) (int, error) {
	return s.reader.Read(p)
}

func (s *snappyReaderCloser) Close() error {
	return nil
}

type snappyBufferedWriter struct {
	buf    bytes.Buffer
	writer io.Writer
	closed bool
}

func newSnappyBufferedWriter(w io.Writer) *snappyBufferedWriter {
	return &snappyBufferedWriter{
		writer: w,
	}
}

func (s *snappyBufferedWriter) Write(p []byte) (int, error) {
	if s.closed {
		return 0, fmt.Errorf("snappy writer is closed")
	}
	return s.buf.Write(p)
}

func (s *snappyBufferedWriter) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true

	compressed := snappy.Encode(nil, s.buf.Bytes())
	_, err := s.writer.Write(compressed)
	return err
}
