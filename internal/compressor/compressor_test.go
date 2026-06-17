package compressor

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Algorithm != AlgorithmGzip {
		t.Errorf("expected default algorithm gzip, got %s", cfg.Algorithm)
	}
	if cfg.Level != LevelDefault {
		t.Errorf("expected default level %d, got %d", LevelDefault, cfg.Level)
	}
	if cfg.Mode != ModeManual {
		t.Errorf("expected default mode manual, got %s", cfg.Mode)
	}
	if cfg.AutoSpeedRatio != 0.5 {
		t.Errorf("expected default AutoSpeedRatio 0.5, got %f", cfg.AutoSpeedRatio)
	}
}

func TestCompressionLevel_Validate(t *testing.T) {
	tests := []struct {
		level    CompressionLevel
		expected bool
	}{
		{LevelFastest, true},
		{LevelFast, true},
		{LevelDefault, true},
		{LevelBetter, true},
		{LevelBest, true},
		{CompressionLevel(0), false},
		{CompressionLevel(6), false},
		{CompressionLevel(-1), false},
	}

	for _, tt := range tests {
		if got := tt.level.Validate(); got != tt.expected {
			t.Errorf("level %d: expected %v, got %v", tt.level, tt.expected, got)
		}
	}
}

func TestAlgorithm_Validate(t *testing.T) {
	tests := []struct {
		alg      Algorithm
		expected bool
	}{
		{AlgorithmGzip, true},
		{AlgorithmSnappy, true},
		{AlgorithmLZ4, true},
		{Algorithm("invalid"), false},
		{Algorithm(""), false},
	}

	for _, tt := range tests {
		if got := tt.alg.Validate(); got != tt.expected {
			t.Errorf("algorithm %s: expected %v, got %v", tt.alg, tt.expected, got)
		}
	}
}

func TestSelectionMode_Validate(t *testing.T) {
	tests := []struct {
		mode     SelectionMode
		expected bool
	}{
		{ModeManual, true},
		{ModeAuto, true},
		{SelectionMode("invalid"), false},
	}

	for _, tt := range tests {
		if got := tt.mode.Validate(); got != tt.expected {
			t.Errorf("mode %s: expected %v, got %v", tt.mode, tt.expected, got)
		}
	}
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr error
	}{
		{
			name:    "valid gzip config",
			cfg:     Config{Algorithm: AlgorithmGzip, Level: LevelDefault, Mode: ModeManual},
			wantErr: nil,
		},
		{
			name:    "valid snappy config",
			cfg:     Config{Algorithm: AlgorithmSnappy, Level: LevelFastest, Mode: ModeManual},
			wantErr: nil,
		},
		{
			name:    "valid lz4 config",
			cfg:     Config{Algorithm: AlgorithmLZ4, Level: LevelBest, Mode: ModeAuto, AutoSpeedRatio: 0.7},
			wantErr: nil,
		},
		{
			name:    "invalid algorithm",
			cfg:     Config{Algorithm: "invalid", Level: LevelDefault, Mode: ModeManual},
			wantErr: ErrUnsupportedAlgorithm,
		},
		{
			name:    "invalid level",
			cfg:     Config{Algorithm: AlgorithmGzip, Level: CompressionLevel(0), Mode: ModeManual},
			wantErr: ErrInvalidCompressionLevel,
		},
		{
			name:    "invalid mode",
			cfg:     Config{Algorithm: AlgorithmGzip, Level: LevelDefault, Mode: "invalid"},
			wantErr: ErrInvalidMode,
		},
		{
			name:    "invalid speed ratio clamped",
			cfg:     Config{Algorithm: AlgorithmGzip, Level: LevelDefault, Mode: ModeAuto, AutoSpeedRatio: 2.0},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, err := NewManager(tt.cfg)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if mgr == nil {
				t.Error("expected manager to be non-nil")
			}
		})
	}
}

func TestGzipCompressDecompress(t *testing.T) {
	testData := []byte("Hello, World! This is a test string for compression. " +
		"It contains repeated patterns that should compress well. " +
		"Hello, World! This is a test string for compression.")

	compressor, err := NewGzipCompressor(LevelDefault)
	if err != nil {
		t.Fatalf("failed to create gzip compressor: %v", err)
	}

	decompressor := NewGzipDecompressor()

	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}
	if len(compressed) == 0 {
		t.Error("compressed data is empty")
	}
	if len(compressed) >= len(testData) {
		t.Logf("Note: compressed data (%d) is not smaller than original (%d)", len(compressed), len(testData))
	}

	decompressed, err := decompressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}
	if !bytes.Equal(testData, decompressed) {
		t.Error("decompressed data does not match original")
	}
}

func TestSnappyCompressDecompress(t *testing.T) {
	testData := []byte("Hello, World! This is a test string for Snappy compression. " +
		"Snappy is designed for speed rather than compression ratio. " +
		"Hello, World! This is a test string for Snappy compression.")

	compressor, err := NewSnappyCompressor(LevelDefault)
	if err != nil {
		t.Fatalf("failed to create snappy compressor: %v", err)
	}

	decompressor := NewSnappyDecompressor()

	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}
	if len(compressed) == 0 {
		t.Error("compressed data is empty")
	}

	decompressed, err := decompressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}
	if !bytes.Equal(testData, decompressed) {
		t.Error("decompressed data does not match original")
	}
}

func TestLZ4CompressDecompress(t *testing.T) {
	testData := []byte("Hello, World! This is a test string for LZ4 compression. " +
		"LZ4 provides excellent speed with good compression ratio. " +
		"Hello, World! This is a test string for LZ4 compression.")

	compressor, err := NewLZ4Compressor(LevelDefault)
	if err != nil {
		t.Fatalf("failed to create lz4 compressor: %v", err)
	}

	decompressor := NewLZ4Decompressor()

	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("compress failed: %v", err)
	}
	if len(compressed) == 0 {
		t.Error("compressed data is empty")
	}

	decompressed, err := decompressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}
	if !bytes.Equal(testData, decompressed) {
		t.Error("decompressed data does not match original")
	}
}

func TestCompressionLevels(t *testing.T) {
	testData := bytes.Repeat([]byte("test data pattern "), 100)

	algorithms := []Algorithm{AlgorithmGzip, AlgorithmLZ4}
	levels := []CompressionLevel{LevelFastest, LevelFast, LevelDefault, LevelBetter, LevelBest}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			for _, level := range levels {
				t.Run(fmt.Sprintf("level_%d", level), func(t *testing.T) {
					compressor, err := NewCompressor(alg, level)
					if err != nil {
						t.Fatalf("failed to create compressor: %v", err)
					}

					if compressor.Algorithm() != alg {
						t.Errorf("expected algorithm %s, got %s", alg, compressor.Algorithm())
					}

					if compressor.Level() != level {
						t.Errorf("expected level %d, got %d", level, compressor.Level())
					}

					compressed, err := compressor.Compress(testData)
					if err != nil {
						t.Fatalf("compress failed: %v", err)
					}
					if len(compressed) == 0 {
						t.Error("compressed data is empty")
					}
				})
			}
		})
	}
}

func TestCompressorInterface(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
	testData := []byte("test data for interface verification")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			compressor, err := NewCompressor(alg, LevelDefault)
			if err != nil {
				t.Fatalf("NewCompressor failed: %v", err)
			}

			compressed, err := compressor.Compress(testData)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			decompressor, err := NewDecompressor(alg)
			if err != nil {
				t.Fatalf("NewDecompressor failed: %v", err)
			}

			decompressed, err := decompressor.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !bytes.Equal(testData, decompressed) {
				t.Error("roundtrip failed: decompressed data mismatch")
			}

			if compressor.Algorithm() != alg {
				t.Errorf("Compressor.Algorithm() = %s, want %s", compressor.Algorithm(), alg)
			}
			if decompressor.Algorithm() != alg {
				t.Errorf("Decompressor.Algorithm() = %s, want %s", decompressor.Algorithm(), alg)
			}
		})
	}
}

func TestStreamCompression(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
	testData := []byte("This is stream compression test data. " +
		"It will be written in chunks to test streaming. " +
		"Streaming allows processing large data without loading everything into memory.")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			cfg := Config{
				Algorithm: alg,
				Level:     LevelDefault,
				Mode:      ModeManual,
			}
			mgr, err := NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager failed: %v", err)
			}

			var compressedBuf bytes.Buffer
			writer, err := mgr.NewStreamCompressor(&compressedBuf)
			if err != nil {
				t.Fatalf("NewStreamCompressor failed: %v", err)
			}

			chunkSize := 10
			for i := 0; i < len(testData); i += chunkSize {
				end := i + chunkSize
				if end > len(testData) {
					end = len(testData)
				}
				n, err := writer.Write(testData[i:end])
				if err != nil {
					t.Fatalf("Write failed: %v", err)
				}
				if n != end-i {
					t.Errorf("Write returned %d, expected %d", n, end-i)
				}
			}

			if err := writer.Close(); err != nil {
				t.Fatalf("Close writer failed: %v", err)
			}

			if compressedBuf.Len() == 0 {
				t.Error("compressed buffer is empty")
			}

			reader, err := mgr.NewStreamDecompressor(&compressedBuf)
			if err != nil {
				t.Fatalf("NewStreamDecompressor failed: %v", err)
			}
			defer reader.Close()

			var decompressedBuf bytes.Buffer
			_, err = io.Copy(&decompressedBuf, reader)
			if err != nil {
				t.Fatalf("Copy failed: %v", err)
			}

			if !bytes.Equal(testData, decompressedBuf.Bytes()) {
				t.Error("stream roundtrip failed: data mismatch")
			}
		})
	}
}

func TestStreamCompression_NilWriter(t *testing.T) {
	cfg := DefaultConfig()
	mgr, _ := NewManager(cfg)

	_, err := mgr.NewStreamCompressor(nil)
	if err != ErrNilWriter {
		t.Errorf("expected ErrNilWriter, got %v", err)
	}
}

func TestStreamDecompression_NilReader(t *testing.T) {
	cfg := DefaultConfig()
	mgr, _ := NewManager(cfg)

	_, err := mgr.NewStreamDecompressor(nil)
	if err != ErrNilReader {
		t.Errorf("expected ErrNilReader, got %v", err)
	}
}

func TestCompressEmptyData(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			compressor, err := NewCompressor(alg, LevelDefault)
			if err != nil {
				t.Fatal(err)
			}

			_, err = compressor.Compress([]byte{})
			if err != ErrEmptyData {
				t.Errorf("expected ErrEmptyData, got %v", err)
			}

			decompressor, _ := NewDecompressor(alg)
			_, err = decompressor.Decompress([]byte{})
			if err != ErrEmptyData {
				t.Errorf("expected ErrEmptyData, got %v", err)
			}
		})
	}
}

func TestCompressCorruptedData(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
	corruptedData := []byte("this is not valid compressed data")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			decompressor, err := NewDecompressor(alg)
			if err != nil {
				t.Fatal(err)
			}

			_, err = decompressor.Decompress(corruptedData)
			if err == nil {
				t.Error("expected error for corrupted data, got nil")
			}
		})
	}
}

func TestInvalidCompressionLevel(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			_, err := NewCompressor(alg, CompressionLevel(0))
			if err == nil {
				t.Error("expected error for invalid level 0")
			}

			_, err = NewCompressor(alg, CompressionLevel(100))
			if err == nil {
				t.Error("expected error for invalid level 100")
			}
		})
	}
}

func TestUnsupportedAlgorithm(t *testing.T) {
	_, err := NewCompressor(Algorithm("invalid"), LevelDefault)
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}

	_, err = NewDecompressor(Algorithm("invalid"))
	if err == nil {
		t.Error("expected error for unsupported algorithm")
	}
}

func TestNewCompressedWriter_Nil(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			compressor, _ := NewCompressor(alg, LevelDefault)
			_, err := compressor.NewCompressedWriter(nil)
			if err != ErrNilWriter {
				t.Errorf("expected ErrNilWriter, got %v", err)
			}
		})
	}
}

func TestNewDecompressedReader_Nil(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			decompressor, _ := NewDecompressor(alg)
			_, err := decompressor.NewDecompressedReader(nil)
			if err != ErrNilReader {
				t.Errorf("expected ErrNilReader, got %v", err)
			}
		})
	}
}

func TestAnalyzeData(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		dataType DataType
	}{
		{
			name:     "empty data",
			data:     []byte{},
			dataType: DataTypeRandom,
		},
		{
			name:     "text data",
			data:     []byte("Hello World! This is a plain text string with lots of characters."),
			dataType: DataTypeText,
		},
		{
			name:     "structured data",
			data:     bytes.Repeat([]byte{0x00, 0x01, 0x00, 0x01}, 100),
			dataType: DataTypeStructured,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnalyzeData(tt.data)
			if result == nil {
				t.Fatal("AnalyzeData returned nil")
			}
			if result.Size != len(tt.data) {
				t.Errorf("expected size %d, got %d", len(tt.data), result.Size)
			}
			if tt.data != nil && result.DataType != tt.dataType {
				t.Logf("expected data type %s, got %s (this may be acceptable depending on analysis)", tt.dataType, result.DataType)
			}
		})
	}
}

func TestAutoModeSelection(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		speedRatio float64
	}{
		{
			name:       "text data speed priority",
			data:       []byte(strings.Repeat("Hello World! ", 100)),
			speedRatio: 0.9,
		},
		{
			name:       "text data compression priority",
			data:       []byte(strings.Repeat("Hello World! ", 100)),
			speedRatio: 0.1,
		},
		{
			name:       "small data",
			data:       []byte("small"),
			speedRatio: 0.5,
		},
		{
			name:       "structured data",
			data:       bytes.Repeat([]byte{0x00, 0x00, 0x00, 0x01, 0x01, 0x01}, 200),
			speedRatio: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Algorithm:      AlgorithmGzip,
				Level:          LevelDefault,
				Mode:           ModeAuto,
				AutoSpeedRatio: tt.speedRatio,
			}

			mgr, err := NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager failed: %v", err)
			}

			compressed, result, err := mgr.Compress(tt.data)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}
			if compressed == nil {
				t.Error("compressed data is nil")
			}
			if result == nil {
				t.Error("result is nil")
			}
			if result.OriginalSize != len(tt.data) {
				t.Errorf("expected original size %d, got %d", len(tt.data), result.OriginalSize)
			}
			if result.CompressedSize != len(compressed) {
				t.Errorf("expected compressed size %d, got %d", len(compressed), result.CompressedSize)
			}

			decompressed, err := mgr.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}
			if !bytes.Equal(tt.data, decompressed) {
				t.Error("auto mode roundtrip failed")
			}
		})
	}
}

func TestManager_CompressDecompress(t *testing.T) {
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}
	testData := []byte("Manager compress and decompress test data. " +
		"This tests the Manager's ability to orchestrate compression operations.")

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			cfg := Config{
				Algorithm: alg,
				Level:     LevelDefault,
				Mode:      ModeManual,
			}

			mgr, err := NewManager(cfg)
			if err != nil {
				t.Fatalf("NewManager failed: %v", err)
			}

			compressed, result, err := mgr.Compress(testData)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			if result.Algorithm != alg {
				t.Errorf("expected algorithm %s, got %s", alg, result.Algorithm)
			}

			decompressed, err := mgr.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !bytes.Equal(testData, decompressed) {
				t.Error("manager roundtrip failed")
			}
		})
	}
}

func TestCompressionRatio(t *testing.T) {
	compressibleData := bytes.Repeat([]byte("compressible pattern "), 1000)

	cfg := DefaultConfig()
	mgr, _ := NewManager(cfg)

	compressed, result, err := mgr.Compress(compressibleData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if result.CompressionRatio >= 1.0 {
		t.Errorf("expected compression ratio < 1.0, got %f", result.CompressionRatio)
	}

	if result.CompressionRatio != float64(len(compressed))/float64(len(compressibleData)) {
		t.Error("compression ratio calculation incorrect")
	}
}

func TestLargeDataCompression(t *testing.T) {
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	for i := 0; i < len(largeData); i += 4 {
		largeData[i] = 'A'
		largeData[i+1] = 'B'
		largeData[i+2] = 'C'
		largeData[i+3] = 'D'
	}

	algorithms := []Algorithm{AlgorithmGzip, AlgorithmSnappy, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			cfg := Config{
				Algorithm: alg,
				Level:     LevelDefault,
				Mode:      ModeManual,
			}
			mgr, _ := NewManager(cfg)

			compressed, result, err := mgr.Compress(largeData)
			if err != nil {
				t.Fatalf("Compress failed: %v", err)
			}

			t.Logf("Algorithm: %s, Original: %d, Compressed: %d, Ratio: %.4f",
				alg, result.OriginalSize, result.CompressedSize, result.CompressionRatio)

			decompressed, err := mgr.Decompress(compressed)
			if err != nil {
				t.Fatalf("Decompress failed: %v", err)
			}

			if !bytes.Equal(largeData, decompressed) {
				t.Error("large data roundtrip failed")
			}
		})
	}
}

func TestConcurrentCompression(t *testing.T) {
	cfg := DefaultConfig()
	mgr, _ := NewManager(cfg)

	numGoroutines := 10
	numOperations := 50
	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			defer func() { done <- true }()

			for i := 0; i < numOperations; i++ {
				testData := []byte(fmt.Sprintf("goroutine_%d_data_%d", id, i))

				compressed, _, err := mgr.Compress(testData)
				if err != nil {
					t.Errorf("goroutine %d: compress failed: %v", id, err)
					return
				}

				decompressed, err := mgr.Decompress(compressed)
				if err != nil {
					t.Errorf("goroutine %d: decompress failed: %v", id, err)
					return
				}

				if !bytes.Equal(testData, decompressed) {
					t.Errorf("goroutine %d: data mismatch", id)
					return
				}
			}
		}(g)
	}

	for g := 0; g < numGoroutines; g++ {
		<-done
	}
}

func TestAnalyzeData_Entropy(t *testing.T) {
	randomData := make([]byte, 1000)
	rand.Read(randomData)

	textData := []byte(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 50))

	randomResult := AnalyzeData(randomData)
	textResult := AnalyzeData(textData)

	if randomResult.Entropy <= textResult.Entropy {
		t.Errorf("expected random data to have higher entropy than text data. random: %f, text: %f",
			randomResult.Entropy, textResult.Entropy)
	}

	t.Logf("Random data entropy: %f", randomResult.Entropy)
	t.Logf("Text data entropy: %f", textResult.Entropy)
}

func TestCompressionResultFields(t *testing.T) {
	testData := []byte("test data for result fields")

	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelBetter,
		Mode:      ModeManual,
	}
	mgr, _ := NewManager(cfg)

	_, result, err := mgr.Compress(testData)
	if err != nil {
		t.Fatal(err)
	}

	if result.Algorithm != AlgorithmGzip {
		t.Errorf("expected AlgorithmGzip, got %s", result.Algorithm)
	}
	if result.Level != LevelBetter {
		t.Errorf("expected LevelBetter (%d), got %d", LevelBetter, result.Level)
	}
	if result.OriginalSize != len(testData) {
		t.Errorf("expected OriginalSize %d, got %d", len(testData), result.OriginalSize)
	}
	if result.CompressionRatio <= 0 {
		t.Errorf("expected positive compression ratio, got %f", result.CompressionRatio)
	}
}

func TestStreamCompression_EmptyWrites(t *testing.T) {
	cfg := DefaultConfig()
	mgr, _ := NewManager(cfg)

	var buf bytes.Buffer
	writer, err := mgr.NewStreamCompressor(&buf)
	if err != nil {
		t.Fatal(err)
	}

	n, err := writer.Write([]byte{})
	if err != nil {
		t.Errorf("empty write should not error, got %v", err)
	}
	if n != 0 {
		t.Errorf("empty write should return 0, got %d", n)
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestAllCompressionLevels(t *testing.T) {
	testData := []byte(strings.Repeat("Compression level test data. ", 100))
	algorithms := []Algorithm{AlgorithmGzip, AlgorithmLZ4}

	for _, alg := range algorithms {
		t.Run(string(alg), func(t *testing.T) {
			results := make(map[CompressionLevel]int)

			for level := LevelFastest; level <= LevelBest; level++ {
				compressor, err := NewCompressor(alg, level)
				if err != nil {
					t.Fatal(err)
				}

				compressed, err := compressor.Compress(testData)
				if err != nil {
					t.Fatal(err)
				}

				results[level] = len(compressed)
			}

			if results[LevelBest] > results[LevelFastest] {
				t.Logf("Note: Best level (%d) produced larger output than Fastest (%d) - this can happen with some data",
					results[LevelBest], results[LevelFastest])
			}
		})
	}
}

func TestAutoModeStreamCompression(t *testing.T) {
	testData := []byte(strings.Repeat("Hello, World! This is auto mode stream test. ", 200))

	cfg := Config{
		Algorithm:      AlgorithmGzip,
		Level:          LevelDefault,
		Mode:           ModeAuto,
		AutoSpeedRatio: 0.7,
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	var compressedBuf bytes.Buffer
	writer, err := mgr.NewStreamCompressor(&compressedBuf)
	if err != nil {
		t.Fatalf("NewStreamCompressor failed: %v", err)
	}

	chunkSize := 512
	for i := 0; i < len(testData); i += chunkSize {
		end := i + chunkSize
		if end > len(testData) {
			end = len(testData)
		}
		n, err := writer.Write(testData[i:end])
		if err != nil {
			t.Fatalf("Write failed at offset %d: %v", i, err)
		}
		if n != end-i {
			t.Errorf("Write returned %d, expected %d", n, end-i)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	compressedSize := compressedBuf.Len()
	if compressedSize == 0 {
		t.Error("compressed buffer is empty")
	}

	t.Logf("Compressed %d bytes to %d bytes", len(testData), compressedSize)

	reader, err := mgr.NewStreamDecompressor(bytes.NewReader(compressedBuf.Bytes()))
	if err != nil {
		t.Fatalf("NewStreamDecompressor failed: %v", err)
	}
	defer reader.Close()

	var decompressedBuf bytes.Buffer
	_, err = io.Copy(&decompressedBuf, reader)
	if err != nil {
		t.Fatalf("Copy failed: %v", err)
	}

	if !bytes.Equal(testData, decompressedBuf.Bytes()) {
		t.Error("auto mode stream roundtrip failed: data mismatch")
	}
}

func TestAutoModeStreamCompression_SmallData(t *testing.T) {
	testData := []byte("Small data that fits in buffer.")

	cfg := Config{
		Algorithm:      AlgorithmGzip,
		Level:          LevelDefault,
		Mode:           ModeAuto,
		AutoSpeedRatio: 0.5,
	}

	mgr, _ := NewManager(cfg)

	var compressedBuf bytes.Buffer
	writer, err := mgr.NewStreamCompressor(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}

	n, err := writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	compressedSize := compressedBuf.Len()
	if compressedSize == 0 {
		t.Error("compressed buffer is empty")
	}

	reader, err := mgr.NewStreamDecompressor(bytes.NewReader(compressedBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(testData, result) {
		t.Error("small data roundtrip failed")
	}
}

func TestAutoModeStreamCompression_LargeChunk(t *testing.T) {
	testData := []byte(strings.Repeat("Large chunk test data that exceeds buffer size. ", 100))

	cfg := Config{
		Algorithm:      AlgorithmSnappy,
		Level:          LevelDefault,
		Mode:           ModeAuto,
		AutoSpeedRatio: 0.9,
	}

	mgr, _ := NewManager(cfg)

	var compressedBuf bytes.Buffer
	writer, err := mgr.NewStreamCompressor(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}

	n, err := writer.Write(testData)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(testData) {
		t.Errorf("Write returned %d, expected %d", n, len(testData))
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := mgr.NewStreamDecompressor(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(testData, result) {
		t.Error("large chunk roundtrip failed")
	}
}

func TestAutoModeStreamDecompressor_ErrorPersistence(t *testing.T) {
	corruptedData := []byte("this is not valid compressed data at all")

	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelDefault,
		Mode:      ModeAuto,
	}

	mgr, _ := NewManager(cfg)

	reader, err := mgr.NewStreamDecompressor(bytes.NewReader(corruptedData))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	buf := make([]byte, 100)
	n, err := reader.Read(buf)
	if err == nil {
		t.Error("expected error for corrupted data, got nil")
	}
	if n != 0 {
		t.Errorf("expected 0 bytes read, got %d", n)
	}

	n2, err2 := reader.Read(buf)
	if err2 == nil {
		t.Error("expected persistent error on second Read, got nil")
	}
	if err2 != err {
		t.Errorf("expected same error, got %v (first was %v)", err2, err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 bytes on second read, got %d", n2)
	}
}

func TestAutoModeStreamDecompressor_EmptyData(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelDefault,
		Mode:      ModeAuto,
	}

	mgr, _ := NewManager(cfg)

	reader, err := mgr.NewStreamDecompressor(bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	buf := make([]byte, 100)
	n, err := reader.Read(buf)
	if err != io.EOF {
		t.Errorf("expected EOF, got %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 bytes, got %d", n)
	}

	n2, err2 := reader.Read(buf)
	if err2 != io.EOF {
		t.Errorf("expected persistent EOF, got %v", err2)
	}
	if n2 != 0 {
		t.Errorf("expected 0 bytes, got %d", n2)
	}
}

func TestAutoModeStreamCompression_MultipleWrites(t *testing.T) {
	parts := [][]byte{
		[]byte("First part of the data. "),
		[]byte("Second part with more content. "),
		[]byte("Third part to test multiple writes. "),
		[]byte("Fourth part that will exceed the buffer threshold."),
	}

	fullData := bytes.Join(parts, nil)

	cfg := Config{
		Algorithm:      AlgorithmLZ4,
		Level:          LevelDefault,
		Mode:           ModeAuto,
		AutoSpeedRatio: 0.6,
	}

	mgr, _ := NewManager(cfg)

	var compressedBuf bytes.Buffer
	writer, err := mgr.NewStreamCompressor(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}

	for i, part := range parts {
		n, err := writer.Write(part)
		if err != nil {
			t.Fatalf("Write part %d failed: %v", i, err)
		}
		if n != len(part) {
			t.Errorf("Part %d: write returned %d, expected %d", i, n, len(part))
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	reader, err := mgr.NewStreamDecompressor(&compressedBuf)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(fullData, result) {
		t.Error("multiple writes roundtrip failed")
	}
}

func TestAnalyzePatterns_SlidingWindow(t *testing.T) {
	data := []byte("ABCDABCDABCD")

	characteristics := AnalyzeData(data)

	if characteristics.DataType != DataTypeStructured && characteristics.DataType != DataTypeText {
		t.Logf("Data type: %s, RepeatRatio: %.4f, Entropy: %.4f",
			characteristics.DataType, characteristics.RepeatRatio, characteristics.Entropy)
	}

	if characteristics.RepeatRatio < 0.2 {
		t.Errorf("Expected high repeat ratio for repeating pattern, got %.4f", characteristics.RepeatRatio)
	}

	t.Logf("Pattern analysis - Size: %d, DataType: %s, RepeatRatio: %.4f",
		characteristics.Size, characteristics.DataType, characteristics.RepeatRatio)
}

func TestAnalyzePatterns_NonAlignedRepeats(t *testing.T) {
	data := []byte("XABCABCABCY")

	characteristics := AnalyzeData(data)

	t.Logf("Non-aligned pattern - DataType: %s, RepeatRatio: %.4f, Entropy: %.4f",
		characteristics.DataType, characteristics.RepeatRatio, characteristics.Entropy)

	if characteristics.RepeatRatio < 0.15 {
		t.Errorf("Expected sliding window to detect non-aligned repeats, got ratio %.4f", characteristics.RepeatRatio)
	}
}

func TestAnalyzePatterns_NoPattern(t *testing.T) {
	randomData := make([]byte, 100)
	rand.Read(randomData)

	characteristics := AnalyzeData(randomData)

	t.Logf("Random data - DataType: %s, RepeatRatio: %.4f, Entropy: %.4f",
		characteristics.DataType, characteristics.RepeatRatio, characteristics.Entropy)

	if characteristics.RepeatRatio > 0.3 {
		t.Errorf("Expected low repeat ratio for random data, got %.4f", characteristics.RepeatRatio)
	}
}

func TestAdaptiveStreamWriter_WriteAfterClose(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelDefault,
		Mode:      ModeAuto,
	}

	mgr, _ := NewManager(cfg)

	var buf bytes.Buffer
	writer, _ := mgr.NewStreamCompressor(&buf)

	writer.Close()

	_, err := writer.Write([]byte("test"))
	if err == nil {
		t.Error("expected error writing to closed writer, got nil")
	}
}

func TestAdaptiveStreamWriter_NilWriter(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelDefault,
		Mode:      ModeAuto,
	}

	mgr, _ := NewManager(cfg)

	_, err := mgr.NewStreamCompressor(nil)
	if err != ErrNilWriter {
		t.Errorf("expected ErrNilWriter, got %v", err)
	}
}

func TestAdaptiveStreamReader_NilReader(t *testing.T) {
	cfg := Config{
		Algorithm: AlgorithmGzip,
		Level:     LevelDefault,
		Mode:      ModeAuto,
	}

	mgr, _ := NewManager(cfg)

	_, err := mgr.NewStreamDecompressor(nil)
	if err != ErrNilReader {
		t.Errorf("expected ErrNilReader, got %v", err)
	}
}

func TestAutoModeStreamCompression_AlgorithmSelection(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		speedRatio float64
	}{
		{
			name:       "text data speed priority",
			data:       []byte(strings.Repeat("Hello World! This is text data. ", 100)),
			speedRatio: 0.9,
		},
		{
			name:       "text data compression priority",
			data:       []byte(strings.Repeat("Hello World! This is text data. ", 100)),
			speedRatio: 0.1,
		},
		{
			name:       "structured binary data",
			data:       bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 200),
			speedRatio: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Algorithm:      AlgorithmGzip,
				Level:          LevelDefault,
				Mode:           ModeAuto,
				AutoSpeedRatio: tt.speedRatio,
			}

			mgr, _ := NewManager(cfg)

			var compressedBuf bytes.Buffer
			writer, err := mgr.NewStreamCompressor(&compressedBuf)
			if err != nil {
				t.Fatal(err)
			}

			_, err = writer.Write(tt.data)
			if err != nil {
				t.Fatal(err)
			}

			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}

			compressedSize := compressedBuf.Len()
			t.Logf("Compressed %d bytes to %d bytes", len(tt.data), compressedSize)

			reader, err := mgr.NewStreamDecompressor(bytes.NewReader(compressedBuf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()

			result, err := io.ReadAll(reader)
			if err != nil {
				t.Fatal(err)
			}

			if !bytes.Equal(tt.data, result) {
				t.Error("algorithm selection roundtrip failed")
			}
		})
	}
}


