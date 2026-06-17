package datadedup

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestHashProvider(t *testing.T) {
	t.Run("SHA256", func(t *testing.T) {
		hp, err := NewHashProvider(HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewHashProvider failed: %v", err)
		}

		data := []byte("hello world")
		fp, err := hp.Hash(data)
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		if len(fp) == 0 {
			t.Error("empty fingerprint")
		}

		fp2, _ := hp.Hash(data)
		if fp != fp2 {
			t.Error("same data should have same fingerprint")
		}

		fp3, _ := hp.Hash([]byte("different"))
		if fp == fp3 {
			t.Error("different data should have different fingerprint")
		}

		if hp.Algorithm() != HashAlgorithmSHA256 {
			t.Errorf("expected SHA256, got %s", hp.Algorithm())
		}
	})

	t.Run("SHA1", func(t *testing.T) {
		hp, err := NewHashProvider(HashAlgorithmSHA1)
		if err != nil {
			t.Fatalf("NewHashProvider failed: %v", err)
		}

		data := []byte("hello world")
		fp, err := hp.Hash(data)
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		if len(fp) == 0 {
			t.Error("empty fingerprint")
		}

		if hp.Algorithm() != HashAlgorithmSHA1 {
			t.Errorf("expected SHA1, got %s", hp.Algorithm())
		}
	})

	t.Run("MD5", func(t *testing.T) {
		hp, err := NewHashProvider(HashAlgorithmMD5)
		if err != nil {
			t.Fatalf("NewHashProvider failed: %v", err)
		}

		data := []byte("hello world")
		fp, err := hp.Hash(data)
		if err != nil {
			t.Fatalf("Hash failed: %v", err)
		}

		if len(fp) == 0 {
			t.Error("empty fingerprint")
		}

		if hp.Algorithm() != HashAlgorithmMD5 {
			t.Errorf("expected MD5, got %s", hp.Algorithm())
		}
	})

	t.Run("UnsupportedAlgorithm", func(t *testing.T) {
		_, err := NewHashProvider(HashAlgorithm("invalid"))
		if err == nil {
			t.Error("expected error for unsupported algorithm")
		}
		if !errors.Is(err, ErrUnsupportedHashAlgo) {
			t.Errorf("expected ErrUnsupportedHashAlgo, got %v", err)
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		hp, _ := NewHashProvider(HashAlgorithmSHA256)
		_, err := hp.Hash([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("expected ErrEmptyData, got %v", err)
		}
	})
}

func TestExactDedup(t *testing.T) {
	t.Run("NewExactDedup", func(t *testing.T) {
		d, err := NewExactDedup(HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewExactDedup failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0, got %d", d.Count())
		}
	})

	t.Run("NewExactDedupWithProvider", func(t *testing.T) {
		hp, _ := NewHashProvider(HashAlgorithmSHA256)
		d, err := NewExactDedupWithProvider(hp)
		if err != nil {
			t.Fatalf("NewExactDedupWithProvider failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}

		_, err = NewExactDedupWithProvider(nil)
		if err == nil {
			t.Error("expected error for nil provider")
		}
		if !errors.Is(err, ErrNilHashProvider) {
			t.Errorf("expected ErrNilHashProvider, got %v", err)
		}
	})

	t.Run("CheckAndAdd", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte("test data")

		result, err := d.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate on first add")
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}

		result, err = d.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate on second add")
		}
		if len(result.MatchedFPs) != 1 {
			t.Errorf("expected 1 matched FP, got %d", len(result.MatchedFPs))
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}
	})

	t.Run("Check", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte("test data")

		result, err := d.Check(data)
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		d.Add(data)

		result, err = d.Check(data)
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("Add", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte("test data")

		err := d.Add(data)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}

		err = d.Add(data)
		if err != nil {
			t.Fatalf("Add failed: %v", err)
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}
	})

	t.Run("Contains", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte("test data")

		exists, err := d.Contains(data)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if exists {
			t.Error("expected not exists")
		}

		d.Add(data)

		exists, err = d.Contains(data)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if !exists {
			t.Error("expected exists")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte("test data")

		d.Add(data)
		if d.Count() != 1 {
			t.Fatalf("expected count 1")
		}

		err := d.Delete(data)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0, got %d", d.Count())
		}

		exists, _ := d.Contains(data)
		if exists {
			t.Error("expected not exists after delete")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		d.Add([]byte("data1"))
		d.Add([]byte("data2"))
		if d.Count() != 2 {
			t.Fatalf("expected count 2")
		}

		err := d.Clear()
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0, got %d", d.Count())
		}
	})

	t.Run("EmptyDataErrors", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)

		_, err := d.Check([]byte{})
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("Check: expected ErrEmptyData, got %v", err)
		}

		err = d.Add([]byte{})
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("Add: expected ErrEmptyData, got %v", err)
		}

		_, err = d.CheckAndAdd([]byte{})
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("CheckAndAdd: expected ErrEmptyData, got %v", err)
		}

		_, err = d.Contains([]byte{})
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("Contains: expected ErrEmptyData, got %v", err)
		}

		err = d.Delete([]byte{})
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("Delete: expected ErrEmptyData, got %v", err)
		}
	})

	t.Run("ClosedEngine", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		d.Close()

		_, err := d.Check([]byte("test"))
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Check: expected ErrEngineClosed, got %v", err)
		}

		err = d.Add([]byte("test"))
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Add: expected ErrEngineClosed, got %v", err)
		}

		_, err = d.CheckAndAdd([]byte("test"))
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("CheckAndAdd: expected ErrEngineClosed, got %v", err)
		}

		_, err = d.Contains([]byte("test"))
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Contains: expected ErrEngineClosed, got %v", err)
		}

		err = d.Delete([]byte("test"))
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Delete: expected ErrEngineClosed, got %v", err)
		}

		err = d.Clear()
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Clear: expected ErrEngineClosed, got %v", err)
		}

		err = d.Close()
		if !errors.Is(err, ErrEngineClosed) {
			t.Errorf("Close: expected ErrEngineClosed, got %v", err)
		}
	})

	t.Run("GetIndexSetIndex", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		d.Add([]byte("data1"))
		d.Add([]byte("data2"))

		index := d.GetIndex()
		if len(index) != 2 {
			t.Errorf("expected index size 2, got %d", len(index))
		}

		d.Clear()
		if d.Count() != 0 {
			t.Fatalf("expected count 0 after clear")
		}

		d.SetIndex(index)
		if d.Count() != 2 {
			t.Errorf("expected count 2 after set index, got %d", d.Count())
		}
	})
}

func TestSimHashCalculator(t *testing.T) {
	t.Run("Calculate", func(t *testing.T) {
		calc := NewSimHashCalculator()

		data := []byte("The quick brown fox jumps over the lazy dog")
		fp, err := calc.Calculate(data)
		if err != nil {
			t.Fatalf("Calculate failed: %v", err)
		}
		if len(fp) == 0 {
			t.Error("empty fingerprint")
		}

		if calc.Algorithm() != SimilarityAlgorithmSimHash {
			t.Errorf("expected simhash, got %s", calc.Algorithm())
		}
	})

	t.Run("Similarity", func(t *testing.T) {
		calc := NewSimHashCalculator()

		data1 := []byte("The quick brown fox jumps over the lazy dog")
		data2 := []byte("The quick brown fox jumps over the lazy dogs")
		data3 := []byte("Completely different text content here")

		fp1, _ := calc.Calculate(data1)
		fp2, _ := calc.Calculate(data2)
		fp3, _ := calc.Calculate(data3)

		sim12, err := calc.Similarity(fp1, fp2)
		if err != nil {
			t.Fatalf("Similarity failed: %v", err)
		}
		if sim12 < 0.7 {
			t.Errorf("expected high similarity for similar texts, got %f", sim12)
		}

		sim13, err := calc.Similarity(fp1, fp3)
		if err != nil {
			t.Fatalf("Similarity failed: %v", err)
		}
		if sim13 > sim12 {
			t.Errorf("expected lower similarity for different texts")
		}

		sim11, _ := calc.Similarity(fp1, fp1)
		if sim11 != 1.0 {
			t.Errorf("expected similarity 1.0 for same text, got %f", sim11)
		}
	})

	t.Run("EmptyData", func(t *testing.T) {
		calc := NewSimHashCalculator()
		_, err := calc.Calculate([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("expected ErrEmptyData, got %v", err)
		}
	})

	t.Run("InvalidFingerprint", func(t *testing.T) {
		calc := NewSimHashCalculator()

		_, err := calc.Similarity("", "abc")
		if !errors.Is(err, ErrInvalidFingerprint) {
			t.Errorf("expected ErrInvalidFingerprint for empty fp1, got %v", err)
		}

		_, err = calc.Similarity("abc", "")
		if !errors.Is(err, ErrInvalidFingerprint) {
			t.Errorf("expected ErrInvalidFingerprint for empty fp2, got %v", err)
		}

		_, err = calc.Similarity("invalid", "abc")
		if !errors.Is(err, ErrInvalidFingerprint) {
			t.Errorf("expected ErrInvalidFingerprint for invalid hex, got %v", err)
		}

		_, err = calc.Similarity("abcd", "abcd")
		if !errors.Is(err, ErrInvalidFingerprint) {
			t.Errorf("expected ErrInvalidFingerprint for short fp, got %v", err)
		}
	})
}

func TestFuzzyDedup(t *testing.T) {
	t.Run("NewFuzzyDedup", func(t *testing.T) {
		d, err := NewFuzzyDedup(SimilarityAlgorithmSimHash, 0.8)
		if err != nil {
			t.Fatalf("NewFuzzyDedup failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}
		if d.Threshold() != 0.8 {
			t.Errorf("expected threshold 0.8, got %f", d.Threshold())
		}
	})

	t.Run("NewFuzzyDedupWithCalculator", func(t *testing.T) {
		calc := NewSimHashCalculator()
		d, err := NewFuzzyDedupWithCalculator(calc, 0.8)
		if err != nil {
			t.Fatalf("NewFuzzyDedupWithCalculator failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}

		_, err = NewFuzzyDedupWithCalculator(nil, 0.8)
		if err == nil {
			t.Error("expected error for nil calculator")
		}
		if !errors.Is(err, ErrNilSimilarityCalculator) {
			t.Errorf("expected ErrNilSimilarityCalculator, got %v", err)
		}

		_, err = NewFuzzyDedupWithCalculator(calc, -0.1)
		if err == nil {
			t.Error("expected error for invalid threshold")
		}
		if !errors.Is(err, ErrInvalidThreshold) {
			t.Errorf("expected ErrInvalidThreshold, got %v", err)
		}

		_, err = NewFuzzyDedupWithCalculator(calc, 1.1)
		if err == nil {
			t.Error("expected error for invalid threshold")
		}
		if !errors.Is(err, ErrInvalidThreshold) {
			t.Errorf("expected ErrInvalidThreshold, got %v", err)
		}
	})

	t.Run("UnsupportedAlgorithm", func(t *testing.T) {
		_, err := NewFuzzyDedup(SimilarityAlgorithm("invalid"), 0.8)
		if err == nil {
			t.Error("expected error for unsupported algorithm")
		}
		if !errors.Is(err, ErrUnsupportedSimAlgo) {
			t.Errorf("expected ErrUnsupportedSimAlgo, got %v", err)
		}
	})

	t.Run("CheckAndAdd", func(t *testing.T) {
		d, _ := NewFuzzyDedup(SimilarityAlgorithmSimHash, 0.7)

		data1 := []byte("The quick brown fox jumps over the lazy dog")
		data2 := []byte("The quick brown fox jumps over the lazy dogs")
		data3 := []byte("Completely different text content")

		result, err := d.CheckAndAdd(data1)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate on first add")
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}

		result, err = d.CheckAndAdd(data2)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate for similar text")
		}
		if d.Count() != 1 {
			t.Errorf("expected count 1, got %d", d.Count())
		}

		result, err = d.CheckAndAdd(data3)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate for different text")
		}
		if d.Count() != 2 {
			t.Errorf("expected count 2, got %d", d.Count())
		}
	})

	t.Run("Delete", func(t *testing.T) {
		d, _ := NewFuzzyDedup(SimilarityAlgorithmSimHash, 0.8)
		data := []byte("test data for fuzzy dedup")

		d.Add(data)
		if d.Count() != 1 {
			t.Fatalf("expected count 1")
		}

		err := d.Delete(data)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0, got %d", d.Count())
		}
	})

	t.Run("Clear", func(t *testing.T) {
		d, _ := NewFuzzyDedup(SimilarityAlgorithmSimHash, 0.8)
		d.Add([]byte("data one"))
		d.Add([]byte("data two"))
		if d.Count() != 2 {
			t.Fatalf("expected count 2")
		}

		err := d.Clear()
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0, got %d", d.Count())
		}
	})
}

func TestFixedSizeChunker(t *testing.T) {
	t.Run("NewFixedSizeChunker", func(t *testing.T) {
		c, err := NewFixedSizeChunker(1024, HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewFixedSizeChunker failed: %v", err)
		}
		if c.Strategy() != ChunkStrategyFixedSize {
			t.Errorf("expected fixed_size strategy, got %s", c.Strategy())
		}

		_, err = NewFixedSizeChunker(0, HashAlgorithmSHA256)
		if err == nil {
			t.Error("expected error for zero chunk size")
		}
		if !errors.Is(err, ErrInvalidChunkSize) {
			t.Errorf("expected ErrInvalidChunkSize, got %v", err)
		}

		_, err = NewFixedSizeChunker(-1, HashAlgorithmSHA256)
		if err == nil {
			t.Error("expected error for negative chunk size")
		}
		if !errors.Is(err, ErrInvalidChunkSize) {
			t.Errorf("expected ErrInvalidChunkSize, got %v", err)
		}
	})

	t.Run("Chunk", func(t *testing.T) {
		chunkSize := 100
		c, _ := NewFixedSizeChunker(chunkSize, HashAlgorithmSHA256)

		data := make([]byte, 250)
		for i := range data {
			data[i] = byte(i % 256)
		}

		chunks, err := c.Chunk(data)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}

		if len(chunks) != 3 {
			t.Errorf("expected 3 chunks, got %d", len(chunks))
		}

		if len(chunks[0].Data) != chunkSize {
			t.Errorf("expected chunk 0 size %d, got %d", chunkSize, len(chunks[0].Data))
		}
		if chunks[0].Offset != 0 {
			t.Errorf("expected chunk 0 offset 0, got %d", chunks[0].Offset)
		}

		if len(chunks[1].Data) != chunkSize {
			t.Errorf("expected chunk 1 size %d, got %d", chunkSize, len(chunks[1].Data))
		}
		if chunks[1].Offset != 100 {
			t.Errorf("expected chunk 1 offset 100, got %d", chunks[1].Offset)
		}

		if len(chunks[2].Data) != 50 {
			t.Errorf("expected chunk 2 size 50, got %d", len(chunks[2].Data))
		}
		if chunks[2].Offset != 200 {
			t.Errorf("expected chunk 2 offset 200, got %d", chunks[2].Offset)
		}

		var reconstructed []byte
		for _, chunk := range chunks {
			reconstructed = append(reconstructed, chunk.Data...)
		}
		if !bytes.Equal(data, reconstructed) {
			t.Error("reconstructed data does not match original")
		}
	})

	t.Run("ChunkEmptyData", func(t *testing.T) {
		c, _ := NewFixedSizeChunker(100, HashAlgorithmSHA256)
		_, err := c.Chunk([]byte{})
		if err == nil {
			t.Error("expected error for empty data")
		}
		if !errors.Is(err, ErrEmptyData) {
			t.Errorf("expected ErrEmptyData, got %v", err)
		}
	})

	t.Run("ChunkSingleByte", func(t *testing.T) {
		c, _ := NewFixedSizeChunker(100, HashAlgorithmSHA256)
		chunks, err := c.Chunk([]byte("x"))
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}
		if len(chunks) != 1 {
			t.Errorf("expected 1 chunk, got %d", len(chunks))
		}
		if len(chunks[0].Data) != 1 {
			t.Errorf("expected chunk size 1, got %d", len(chunks[0].Data))
		}
	})
}

func TestContentBasedChunker(t *testing.T) {
	t.Run("NewContentBasedChunker", func(t *testing.T) {
		c, err := NewContentBasedChunker(10, 100, '\n', HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewContentBasedChunker failed: %v", err)
		}
		if c.Strategy() != ChunkStrategyContent {
			t.Errorf("expected content_based strategy, got %s", c.Strategy())
		}

		_, err = NewContentBasedChunker(0, 100, '\n', HashAlgorithmSHA256)
		if err == nil {
			t.Error("expected error for zero min size")
		}
		if !errors.Is(err, ErrInvalidChunkSize) {
			t.Errorf("expected ErrInvalidChunkSize, got %v", err)
		}

		_, err = NewContentBasedChunker(100, 50, '\n', HashAlgorithmSHA256)
		if err == nil {
			t.Error("expected error for max < min")
		}
		if !errors.Is(err, ErrInvalidChunkSize) {
			t.Errorf("expected ErrInvalidChunkSize, got %v", err)
		}
	})

	t.Run("ChunkWithBoundary", func(t *testing.T) {
		c, _ := NewContentBasedChunker(5, 100, '\n', HashAlgorithmSHA256)

		data := []byte("line1\nline2\nline3\n")
		chunks, err := c.Chunk(data)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}

		if len(chunks) < 2 {
			t.Errorf("expected at least 2 chunks, got %d", len(chunks))
		}

		var reconstructed []byte
		for _, chunk := range chunks {
			reconstructed = append(reconstructed, chunk.Data...)
		}
		if !bytes.Equal(data, reconstructed) {
			t.Error("reconstructed data does not match original")
		}
	})

	t.Run("ChunkNoBoundary", func(t *testing.T) {
		c, _ := NewContentBasedChunker(5, 20, '\n', HashAlgorithmSHA256)

		data := []byte("abcdefghijklmnopqrstuvwxyz")
		chunks, err := c.Chunk(data)
		if err != nil {
			t.Fatalf("Chunk failed: %v", err)
		}

		if len(chunks) < 1 {
			t.Errorf("expected at least 1 chunk, got %d", len(chunks))
		}

		var reconstructed []byte
		for _, chunk := range chunks {
			reconstructed = append(reconstructed, chunk.Data...)
		}
		if !bytes.Equal(data, reconstructed) {
			t.Error("reconstructed data does not match original")
		}
	})
}

func TestChunkedDedup(t *testing.T) {
	t.Run("NewChunkedDedupFixedSize", func(t *testing.T) {
		d, err := NewChunkedDedup(ChunkStrategyFixedSize, 100, 0, 0, 0, HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewChunkedDedup failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}
	})

	t.Run("NewChunkedDedupContentBased", func(t *testing.T) {
		d, err := NewChunkedDedup(ChunkStrategyContent, 0, 10, 100, '\n', HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("NewChunkedDedup failed: %v", err)
		}
		if d == nil {
			t.Fatal("expected non-nil engine")
		}
	})

	t.Run("UnsupportedStrategy", func(t *testing.T) {
		_, err := NewChunkedDedup(ChunkStrategy("invalid"), 100, 0, 0, 0, HashAlgorithmSHA256)
		if err == nil {
			t.Error("expected error for unsupported strategy")
		}
		if !errors.Is(err, ErrUnsupportedChunkStrat) {
			t.Errorf("expected ErrUnsupportedChunkStrat, got %v", err)
		}
	})

	t.Run("CheckAndAddPartialDuplicate", func(t *testing.T) {
		d, _ := NewChunkedDedup(ChunkStrategyFixedSize, 10, 0, 0, 0, HashAlgorithmSHA256)

		data1 := []byte("0123456789abcdefghij")
		data2 := []byte("0123456789XXXXXXXXXX")

		result, err := d.CheckAndAdd(data1)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate on first add")
		}

		result, err = d.CheckAndAdd(data2)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate due to shared chunk")
		}
		if len(result.MatchedChunks) == 0 {
			t.Error("expected at least one matched chunk")
		}
	})

	t.Run("DeleteAndClear", func(t *testing.T) {
		d, _ := NewChunkedDedup(ChunkStrategyFixedSize, 10, 0, 0, 0, HashAlgorithmSHA256)
		data := []byte("0123456789abcdefghij")

		d.Add(data)
		countBefore := d.Count()
		if countBefore == 0 {
			t.Fatal("expected non-zero count")
		}

		err := d.Delete(data)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0 after delete, got %d", d.Count())
		}

		d.Add(data)
		d.Add([]byte("zzzzzzzzzzzzzzzzzzzz"))
		if d.Count() == 0 {
			t.Fatal("expected non-zero count")
		}

		err = d.Clear()
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}
		if d.Count() != 0 {
			t.Errorf("expected count 0 after clear, got %d", d.Count())
		}
	})
}

func TestPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "test.idx")

	t.Run("SaveAndLoad", func(t *testing.T) {
		p := NewPersistIndex()

		index := FingerprintIndex{
			"fp1": true,
			"fp2": true,
			"fp3": true,
		}

		err := p.Save(index, indexPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		loaded, err := p.Load(indexPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != len(index) {
			t.Errorf("expected %d entries, got %d", len(index), len(loaded))
		}

		for k := range index {
			if !loaded[k] {
				t.Errorf("missing key %s", k)
			}
		}
	})

	t.Run("Append", func(t *testing.T) {
		p := NewPersistIndex()

		index := FingerprintIndex{
			"fp1": true,
		}

		err := p.Save(index, indexPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		err = p.Append("fp2", indexPath)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		loaded, err := p.Load(indexPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 2 {
			t.Errorf("expected 2 entries, got %d", len(loaded))
		}
		if !loaded["fp1"] || !loaded["fp2"] {
			t.Error("missing entries after append")
		}
	})

	t.Run("AppendNewFile", func(t *testing.T) {
		newPath := filepath.Join(tmpDir, "new.idx")
		p := NewPersistIndex()

		err := p.Append("fp1", newPath)
		if err != nil {
			t.Fatalf("Append new file failed: %v", err)
		}

		loaded, err := p.Load(newPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 1 {
			t.Errorf("expected 1 entry, got %d", len(loaded))
		}
		if !loaded["fp1"] {
			t.Error("missing entry")
		}
	})

	t.Run("Verify", func(t *testing.T) {
		p := NewPersistIndex()

		index := FingerprintIndex{"fp1": true, "fp2": true}
		err := p.Save(index, indexPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		valid, err := p.Verify(indexPath)
		if err != nil {
			t.Fatalf("Verify failed: %v", err)
		}
		if !valid {
			t.Error("expected valid file")
		}
	})

	t.Run("VerifyCorrupted", func(t *testing.T) {
		p := NewPersistIndex()

		index := FingerprintIndex{"fp1": true}
		err := p.Save(index, indexPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		f, err := os.OpenFile(indexPath, os.O_RDWR, 0644)
		if err != nil {
			t.Fatalf("Open failed: %v", err)
		}
		f.Seek(20, 0)
		f.Write([]byte{0xFF, 0xFF, 0xFF, 0xFF})
		f.Close()

		valid, _ := p.Verify(indexPath)
		if valid {
			t.Error("expected invalid file after corruption")
		}

		_, err = p.Load(indexPath)
		if err == nil {
			t.Error("expected error loading corrupted file")
		}
	})

	t.Run("LoadNonExistent", func(t *testing.T) {
		p := NewPersistIndex()
		_, err := p.Load(filepath.Join(tmpDir, "nonexistent.idx"))
		if err == nil {
			t.Error("expected error for non-existent file")
		}
		if !errors.Is(err, ErrPersistFileNotExist) {
			t.Errorf("expected ErrPersistFileNotExist, got %v", err)
		}
	})

	t.Run("VerifyNonExistent", func(t *testing.T) {
		p := NewPersistIndex()
		_, err := p.Verify(filepath.Join(tmpDir, "nonexistent.idx"))
		if err == nil {
			t.Error("expected error for non-existent file")
		}
		if !errors.Is(err, ErrPersistFileNotExist) {
			t.Errorf("expected ErrPersistFileNotExist, got %v", err)
		}
	})

	t.Run("AppendIdempotent", func(t *testing.T) {
		idempotentPath := filepath.Join(tmpDir, "idempotent.idx")
		p := NewPersistIndex()

		index := FingerprintIndex{
			"fp_alpha": true,
			"fp_beta":  true,
		}
		err := p.Save(index, idempotentPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		err = p.Append("fp_gamma", idempotentPath)
		if err != nil {
			t.Fatalf("first Append failed: %v", err)
		}

		err = p.Append("fp_gamma", idempotentPath)
		if err != nil {
			t.Fatalf("duplicate Append returned error: %v", err)
		}

		err = p.Append("fp_gamma", idempotentPath)
		if err != nil {
			t.Fatalf("third duplicate Append returned error: %v", err)
		}

		loaded, err := p.Load(idempotentPath)
		if err != nil {
			t.Fatalf("Load after idempotent appends failed: %v", err)
		}

		if len(loaded) != 3 {
			t.Errorf("expected 3 entries after idempotent appends, got %d", len(loaded))
		}

		if !loaded["fp_alpha"] || !loaded["fp_beta"] || !loaded["fp_gamma"] {
			t.Error("missing expected entries after idempotent appends")
		}

		valid, err := p.Verify(idempotentPath)
		if err != nil {
			t.Fatalf("Verify failed: %v", err)
		}
		if !valid {
			t.Error("expected valid file after idempotent appends")
		}

		err = p.Append("fp_delta", idempotentPath)
		if err != nil {
			t.Fatalf("Append new fp after idempotent appends failed: %v", err)
		}

		loaded2, err := p.Load(idempotentPath)
		if err != nil {
			t.Fatalf("Load after new append failed: %v", err)
		}
		if len(loaded2) != 4 {
			t.Errorf("expected 4 entries after new append, got %d", len(loaded2))
		}
	})
}

func TestDedupEngine(t *testing.T) {
	t.Run("NewExactMode", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeExact)
		engine, err := NewDedupEngine(cfg)
		if err != nil {
			t.Fatalf("NewDedupEngine failed: %v", err)
		}
		defer engine.Close()

		data := []byte("test data")
		result, err := engine.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, err = engine.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("NewFuzzyMode", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeFuzzy).WithSimilarityThreshold(0.7)
		engine, err := NewDedupEngine(cfg)
		if err != nil {
			t.Fatalf("NewDedupEngine failed: %v", err)
		}
		defer engine.Close()

		data1 := []byte("The quick brown fox jumps over the lazy dog")
		data2 := []byte("The quick brown fox jumps over the lazy dogs")

		result, err := engine.CheckAndAdd(data1)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, err = engine.CheckAndAdd(data2)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate for similar text")
		}
	})

	t.Run("NewChunkedMode", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeChunked).WithChunkSize(10)
		engine, err := NewDedupEngine(cfg)
		if err != nil {
			t.Fatalf("NewDedupEngine failed: %v", err)
		}
		defer engine.Close()

		data1 := []byte("0123456789abcdefghij")
		data2 := []byte("0123456789XXXXXXXXXX")

		result, err := engine.CheckAndAdd(data1)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, err = engine.CheckAndAdd(data2)
		if err != nil {
			t.Fatalf("CheckAndAdd failed: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate due to shared chunk")
		}
	})

	t.Run("InvalidConfig", func(t *testing.T) {
		cfg := Config{
			Mode: DedupMode("invalid"),
		}
		_, err := NewDedupEngine(cfg)
		if err == nil {
			t.Error("expected error for invalid mode")
		}
		if !errors.Is(err, ErrInvalidDedupMode) {
			t.Errorf("expected ErrInvalidDedupMode, got %v", err)
		}

		cfg2 := DefaultConfig().WithMode(DedupModeExact)
		cfg2.SimilarityThreshold = 2.0
		_, err = NewDedupEngine(cfg2)
		if err == nil {
			t.Error("expected error for invalid threshold")
		}
		if !errors.Is(err, ErrInvalidThreshold) {
			t.Errorf("expected ErrInvalidThreshold, got %v", err)
		}
	})

	t.Run("SaveAndLoad", func(t *testing.T) {
		tmpDir := t.TempDir()
		indexPath := filepath.Join(tmpDir, "engine.idx")

		cfg := DefaultConfig().WithMode(DedupModeExact)
		engine, _ := NewDedupEngine(cfg)

		engine.Add([]byte("data1"))
		engine.Add([]byte("data2"))
		engine.Add([]byte("data3"))

		err := engine.Save(indexPath)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		engine.Close()

		cfg2 := DefaultConfig().WithMode(DedupModeExact)
		engine2, _ := NewDedupEngine(cfg2)

		err = engine2.Load(indexPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if engine2.Count() != 3 {
			t.Errorf("expected count 3, got %d", engine2.Count())
		}

		exists, _ := engine2.Contains([]byte("data1"))
		if !exists {
			t.Error("expected data1 to exist")
		}
		exists, _ = engine2.Contains([]byte("data2"))
		if !exists {
			t.Error("expected data2 to exist")
		}
		exists, _ = engine2.Contains([]byte("data3"))
		if !exists {
			t.Error("expected data3 to exist")
		}

		engine2.Close()
	})

	t.Run("AutoPersist", func(t *testing.T) {
		tmpDir := t.TempDir()
		indexPath := filepath.Join(tmpDir, "auto.idx")

		cfg := DefaultConfig().WithMode(DedupModeExact).WithPersistPath(indexPath)
		cfg.AutoPersistCount = 2
		cfg.AutoPersist = true

		engine, err := NewDedupEngine(cfg)
		if err != nil {
			t.Fatalf("NewDedupEngine failed: %v", err)
		}

		engine.Add([]byte("data1"))
		engine.Add([]byte("data2"))

		engine.Close()

		cfg2 := DefaultConfig().WithMode(DedupModeExact)
		engine2, _ := NewDedupEngine(cfg2)
		err = engine2.Load(indexPath)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if engine2.Count() != 2 {
			t.Errorf("expected count 2, got %d", engine2.Count())
		}

		engine2.Close()
	})

	t.Run("ConcurrentAccess", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeExact)
		engine, _ := NewDedupEngine(cfg)
		defer engine.Close()

		var wg sync.WaitGroup
		numGoroutines := 10
		numOperations := 100

		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer wg.Done()
				for j := 0; j < numOperations; j++ {
					data := []byte(string(rune('a'+id)) + string(rune('0'+j%10)))
					_, _ = engine.CheckAndAdd(data)
				}
			}(i)
		}
		wg.Wait()

		if engine.Count() == 0 {
			t.Error("expected non-zero count after concurrent operations")
		}
	})

	t.Run("DeleteAndContains", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeExact)
		engine, _ := NewDedupEngine(cfg)
		defer engine.Close()

		data := []byte("test data")
		engine.Add(data)

		exists, err := engine.Contains(data)
		if err != nil {
			t.Fatalf("Contains failed: %v", err)
		}
		if !exists {
			t.Error("expected data to exist")
		}

		err = engine.Delete(data)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		exists, _ = engine.Contains(data)
		if exists {
			t.Error("expected data not to exist after delete")
		}
	})

	t.Run("Clear", func(t *testing.T) {
		cfg := DefaultConfig().WithMode(DedupModeExact)
		engine, _ := NewDedupEngine(cfg)
		defer engine.Close()

		engine.Add([]byte("data1"))
		engine.Add([]byte("data2"))

		if engine.Count() != 2 {
			t.Fatalf("expected count 2")
		}

		err := engine.Clear()
		if err != nil {
			t.Fatalf("Clear failed: %v", err)
		}

		if engine.Count() != 0 {
			t.Errorf("expected count 0, got %d", engine.Count())
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("DefaultConfig", func(t *testing.T) {
		cfg := DefaultConfig()
		if cfg.Mode != DedupModeExact {
			t.Errorf("expected exact mode, got %s", cfg.Mode)
		}
		if cfg.HashAlgorithm != HashAlgorithmSHA256 {
			t.Errorf("expected SHA256, got %s", cfg.HashAlgorithm)
		}
		if cfg.SimilarityThreshold != 0.85 {
			t.Errorf("expected threshold 0.85, got %f", cfg.SimilarityThreshold)
		}
		if cfg.ChunkSize != 4096 {
			t.Errorf("expected chunk size 4096, got %d", cfg.ChunkSize)
		}
	})

	t.Run("Validate", func(t *testing.T) {
		cfg := DefaultConfig()
		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got %v", err)
		}

		cfg.Mode = DedupMode("invalid")
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for invalid mode")
		}
		cfg.Mode = DedupModeExact

		cfg.SimilarityThreshold = -0.1
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for negative threshold")
		}
		cfg.SimilarityThreshold = 1.1
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for threshold > 1")
		}
		cfg.SimilarityThreshold = 0.85

		cfg.Mode = DedupModeChunked
		cfg.ChunkSize = 0
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for zero chunk size")
		}
		cfg.ChunkSize = 4096

		cfg.ChunkStrategy = ChunkStrategyContent
		cfg.MinChunkSize = 0
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for zero min chunk size")
		}
		cfg.MinChunkSize = 1024
		cfg.MaxChunkSize = 512
		if err := cfg.Validate(); err == nil {
			t.Error("expected error for max < min")
		}
		cfg.MaxChunkSize = 16384

		if err := cfg.Validate(); err != nil {
			t.Errorf("expected valid config, got %v", err)
		}
	})

	t.Run("WithMethods", func(t *testing.T) {
		cfg := DefaultConfig().
			WithMode(DedupModeFuzzy).
			WithHashAlgorithm(HashAlgorithmSHA1).
			WithSimilarityThreshold(0.9).
			WithChunkStrategy(ChunkStrategyContent).
			WithChunkSize(2048).
			WithPersistPath("/tmp/test.idx")

		if cfg.Mode != DedupModeFuzzy {
			t.Error("WithMode failed")
		}
		if cfg.HashAlgorithm != HashAlgorithmSHA1 {
			t.Error("WithHashAlgorithm failed")
		}
		if cfg.SimilarityThreshold != 0.9 {
			t.Error("WithSimilarityThreshold failed")
		}
		if cfg.ChunkStrategy != ChunkStrategyContent {
			t.Error("WithChunkStrategy failed")
		}
		if cfg.ChunkSize != 2048 {
			t.Error("WithChunkSize failed")
		}
		if cfg.PersistPath != "/tmp/test.idx" {
			t.Error("WithPersistPath failed")
		}
		if !cfg.AutoPersist {
			t.Error("AutoPersist should be true")
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("LargeData", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := make([]byte, 1024*1024)
		for i := range data {
			data[i] = byte(i % 256)
		}

		result, err := d.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed for large data: %v", err)
		}
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, err = d.CheckAndAdd(data)
		if err != nil {
			t.Fatalf("CheckAndAdd failed for large data: %v", err)
		}
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("SingleByte", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte{0x42}

		result, _ := d.CheckAndAdd(data)
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, _ = d.CheckAndAdd(data)
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("AllZeros", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := make([]byte, 1000)

		result, _ := d.CheckAndAdd(data)
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, _ = d.CheckAndAdd(data)
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("BinaryData", func(t *testing.T) {
		d, _ := NewExactDedup(HashAlgorithmSHA256)
		data := []byte{0x00, 0xFF, 0x80, 0x7F, 0xFE, 0x01}

		result, _ := d.CheckAndAdd(data)
		if result.IsDuplicate {
			t.Error("expected not duplicate")
		}

		result, _ = d.CheckAndAdd(data)
		if !result.IsDuplicate {
			t.Error("expected duplicate")
		}
	})

	t.Run("UnicodeData", func(t *testing.T) {
		calc := NewSimHashCalculator()
		data := []byte("你好世界 Hello World 🌍")

		fp1, err := calc.Calculate(data)
		if err != nil {
			t.Fatalf("Calculate failed: %v", err)
		}

		fp2, _ := calc.Calculate(data)
		if fp1 != fp2 {
			t.Error("same unicode data should have same fingerprint")
		}
	})
}

type mockSimilarityCalc struct {
	calcFunc   func(data []byte) (Fingerprint, error)
	simFunc    func(fp1, fp2 Fingerprint) (float64, error)
	algorithm  SimilarityAlgorithm
}

func (m *mockSimilarityCalc) Calculate(data []byte) (Fingerprint, error) {
	if m.calcFunc != nil {
		return m.calcFunc(data)
	}
	return Fingerprint(string(data)), nil
}

func (m *mockSimilarityCalc) Similarity(fp1, fp2 Fingerprint) (float64, error) {
	if m.simFunc != nil {
		return m.simFunc(fp1, fp2)
	}
	if fp1 == fp2 {
		return 1.0, nil
	}
	return 0.0, nil
}

func (m *mockSimilarityCalc) Algorithm() SimilarityAlgorithm {
	if m.algorithm != "" {
		return m.algorithm
	}
	return "mock"
}

func TestCustomSimilarityCalculator(t *testing.T) {
	mock := &mockSimilarityCalc{
		calcFunc: func(data []byte) (Fingerprint, error) {
			return Fingerprint("mock_" + string(data)), nil
		},
		simFunc: func(fp1, fp2 Fingerprint) (float64, error) {
			return 0.95, nil
		},
	}

	cfg := DefaultConfig().WithMode(DedupModeFuzzy)
	cfg.SimilarityCalc = mock
	cfg.SimilarityThreshold = 0.9

	engine, err := NewDedupEngine(cfg)
	if err != nil {
		t.Fatalf("NewDedupEngine failed: %v", err)
	}
	defer engine.Close()

	engine.Add([]byte("data1"))

	result, _ := engine.Check([]byte("data2"))
	if !result.IsDuplicate {
		t.Error("expected duplicate due to mock similarity")
	}
	if result.Similarity != 0.95 {
		t.Errorf("expected similarity 0.95, got %f", result.Similarity)
	}
}

type mockChunker struct {
	chunkFunc  func(data []byte) ([]Chunk, error)
	strategy   ChunkStrategy
}

func (m *mockChunker) Chunk(data []byte) ([]Chunk, error) {
	if m.chunkFunc != nil {
		return m.chunkFunc(data)
	}
	return []Chunk{{
		Data:        data,
		Offset:      0,
		Fingerprint: Fingerprint(string(data)),
	}}, nil
}

func (m *mockChunker) Strategy() ChunkStrategy {
	if m.strategy != "" {
		return m.strategy
	}
	return "mock"
}

func TestCustomChunker(t *testing.T) {
	chunkCount := 0
	mock := &mockChunker{
		chunkFunc: func(data []byte) ([]Chunk, error) {
			chunkCount++
			return []Chunk{
				{Data: data[:len(data)/2], Offset: 0, Fingerprint: "fp1"},
				{Data: data[len(data)/2:], Offset: int64(len(data) / 2), Fingerprint: "fp2"},
			}, nil
		},
	}

	hp, _ := NewHashProvider(HashAlgorithmSHA256)

	cfg := DefaultConfig().WithMode(DedupModeChunked)
	cfg.Chunker = mock
	cfg.HashProvider = hp

	engine, err := NewDedupEngine(cfg)
	if err != nil {
		t.Fatalf("NewDedupEngine failed: %v", err)
	}
	defer engine.Close()

	data := []byte("0123456789")
	engine.Add(data)

	if chunkCount != 1 {
		t.Errorf("expected chunker to be called once, got %d", chunkCount)
	}

	if engine.Count() != 2 {
		t.Errorf("expected 2 fingerprints, got %d", engine.Count())
	}
}

type mockPersister struct {
	saveFunc   func(index FingerprintIndex, path string) error
	loadFunc   func(path string) (FingerprintIndex, error)
	appendFunc func(fp Fingerprint, path string) error
	verifyFunc func(path string) (bool, error)
}

func (m *mockPersister) Save(index FingerprintIndex, path string) error {
	if m.saveFunc != nil {
		return m.saveFunc(index, path)
	}
	return nil
}

func (m *mockPersister) Load(path string) (FingerprintIndex, error) {
	if m.loadFunc != nil {
		return m.loadFunc(path)
	}
	return make(FingerprintIndex), nil
}

func (m *mockPersister) Append(fp Fingerprint, path string) error {
	if m.appendFunc != nil {
		return m.appendFunc(fp, path)
	}
	return nil
}

func (m *mockPersister) Verify(path string) (bool, error) {
	if m.verifyFunc != nil {
		return m.verifyFunc(path)
	}
	return true, nil
}

func TestCustomPersister(t *testing.T) {
	saveCalled := false
	loadCalled := false

	mock := &mockPersister{
		saveFunc: func(index FingerprintIndex, path string) error {
			saveCalled = true
			return nil
		},
		loadFunc: func(path string) (FingerprintIndex, error) {
			loadCalled = true
			return FingerprintIndex{"loaded_fp": true}, nil
		},
	}

	tmpDir := t.TempDir()
	indexPath := filepath.Join(tmpDir, "custom.idx")

	cfg := DefaultConfig().WithMode(DedupModeExact).WithPersistPath(indexPath)
	cfg.Persister = mock

	engine, err := NewDedupEngine(cfg)
	if err != nil {
		t.Fatalf("NewDedupEngine failed: %v", err)
	}

	engine.Add([]byte("test"))
	engine.Save(indexPath)

	if !saveCalled {
		t.Error("expected Save to be called")
	}

	engine.Close()

	cfg2 := DefaultConfig().WithMode(DedupModeExact)
	cfg2.Persister = mock
	engine2, _ := NewDedupEngine(cfg2)
	engine2.Load(indexPath)

	if !loadCalled {
		t.Error("expected Load to be called")
	}
	if engine2.Count() != 1 {
		t.Errorf("expected count 1, got %d", engine2.Count())
	}

	engine2.Close()
}
