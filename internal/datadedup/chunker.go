package datadedup

import (
	"hash"
	"hash/fnv"
	"sync"
)

type fixedSizeChunker struct {
	chunkSize    int
	hashProvider HashProvider
}

func NewFixedSizeChunker(chunkSize int, hashAlgo HashAlgorithm) (Chunker, error) {
	if chunkSize <= 0 {
		return nil, ErrInvalidChunkSize
	}

	hp, err := NewHashProvider(hashAlgo)
	if err != nil {
		return nil, err
	}

	return &fixedSizeChunker{
		chunkSize:    chunkSize,
		hashProvider: hp,
	}, nil
}

func NewFixedSizeChunkerWithProvider(chunkSize int, hp HashProvider) (Chunker, error) {
	if chunkSize <= 0 {
		return nil, ErrInvalidChunkSize
	}
	if hp == nil {
		return nil, ErrNilHashProvider
	}

	return &fixedSizeChunker{
		chunkSize:    chunkSize,
		hashProvider: hp,
	}, nil
}

func (c *fixedSizeChunker) Chunk(data []byte) ([]Chunk, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	var chunks []Chunk
	var offset int64

	for offset < int64(len(data)) {
		end := offset + int64(c.chunkSize)
		if end > int64(len(data)) {
			end = int64(len(data))
		}

		chunkData := data[offset:end]
		fp, err := c.hashProvider.Hash(chunkData)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, Chunk{
			Data:        chunkData,
			Offset:      offset,
			Fingerprint: fp,
		})

		offset = end
	}

	return chunks, nil
}

func (c *fixedSizeChunker) Strategy() ChunkStrategy {
	return ChunkStrategyFixedSize
}

type contentBasedChunker struct {
	minChunkSize int
	maxChunkSize int
	boundary     byte
	hashProvider HashProvider
	hasher       hash.Hash64
	windowSize   int
	targetBits   uint8
}

func NewContentBasedChunker(minSize, maxSize int, boundary byte, hashAlgo HashAlgorithm) (Chunker, error) {
	if minSize <= 0 || maxSize < minSize {
		return nil, ErrInvalidChunkSize
	}

	hp, err := NewHashProvider(hashAlgo)
	if err != nil {
		return nil, err
	}

	return &contentBasedChunker{
		minChunkSize: minSize,
		maxChunkSize: maxSize,
		boundary:     boundary,
		hashProvider: hp,
		hasher:       fnv.New64(),
		windowSize:   48,
		targetBits:   13,
	}, nil
}

func NewContentBasedChunkerWithProvider(minSize, maxSize int, boundary byte, hp HashProvider) (Chunker, error) {
	if minSize <= 0 || maxSize < minSize {
		return nil, ErrInvalidChunkSize
	}
	if hp == nil {
		return nil, ErrNilHashProvider
	}

	return &contentBasedChunker{
		minChunkSize: minSize,
		maxChunkSize: maxSize,
		boundary:     boundary,
		hashProvider: hp,
		hasher:       fnv.New64(),
		windowSize:   48,
		targetBits:   13,
	}, nil
}

func (c *contentBasedChunker) Chunk(data []byte) ([]Chunk, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	var chunks []Chunk
	var offset int64
	dataLen := int64(len(data))

	for offset < dataLen {
		start := offset
		var end int64

		if dataLen-start <= int64(c.minChunkSize) {
			end = dataLen
		} else if dataLen-start <= int64(c.maxChunkSize) {
			end = c.findBoundary(data, start, dataLen)
			if end == start {
				end = dataLen
			}
		} else {
			end = c.findBoundary(data, start, start+int64(c.maxChunkSize))
			if end == start {
				end = start + int64(c.maxChunkSize)
			}
		}

		chunkData := data[start:end]
		fp, err := c.hashProvider.Hash(chunkData)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, Chunk{
			Data:        chunkData,
			Offset:      start,
			Fingerprint: fp,
		})

		offset = end
	}

	return chunks, nil
}

func (c *contentBasedChunker) findBoundary(data []byte, start, limit int64) int64 {
	minEnd := start + int64(c.minChunkSize)
	if minEnd > limit {
		minEnd = limit
	}

	for i := minEnd; i < limit; i++ {
		if data[i] == c.boundary {
			return i + 1
		}
	}

	return limit
}

func (c *contentBasedChunker) Strategy() ChunkStrategy {
	return ChunkStrategyContent
}

type chunkedDedup struct {
	index        FingerprintIndex
	chunker      Chunker
	hashProvider HashProvider
	mu           sync.RWMutex
	closed       bool
}

func NewChunkedDedup(strategy ChunkStrategy, chunkSize, minChunk, maxChunk int, boundary byte, hashAlgo HashAlgorithm) (*chunkedDedup, error) {
	var chunker Chunker
	var err error

	switch strategy {
	case ChunkStrategyFixedSize:
		chunker, err = NewFixedSizeChunker(chunkSize, hashAlgo)
	case ChunkStrategyContent:
		chunker, err = NewContentBasedChunker(minChunk, maxChunk, boundary, hashAlgo)
	default:
		return nil, ErrUnsupportedChunkStrat
	}

	if err != nil {
		return nil, err
	}

	hp, err := NewHashProvider(hashAlgo)
	if err != nil {
		return nil, err
	}

	return &chunkedDedup{
		index:        make(FingerprintIndex),
		chunker:      chunker,
		hashProvider: hp,
		closed:       false,
	}, nil
}

func NewChunkedDedupWithChunker(chunker Chunker, hp HashProvider) (*chunkedDedup, error) {
	if chunker == nil {
		return nil, ErrNilChunker
	}
	if hp == nil {
		return nil, ErrNilHashProvider
	}

	return &chunkedDedup{
		index:        make(FingerprintIndex),
		chunker:      chunker,
		hashProvider: hp,
		closed:       false,
	}, nil
}

func (d *chunkedDedup) Check(data []byte) (*DedupResult, error) {
	if d.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	chunks, err := d.chunker.Chunk(data)
	if err != nil {
		return nil, err
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	result := &DedupResult{
		IsDuplicate: false,
	}

	for _, chunk := range chunks {
		if d.index[chunk.Fingerprint] {
			result.IsDuplicate = true
			result.MatchedFPs = append(result.MatchedFPs, chunk.Fingerprint)
			result.MatchedChunks = append(result.MatchedChunks, chunk)
		}
	}

	return result, nil
}

func (d *chunkedDedup) Add(data []byte) error {
	if d.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	chunks, err := d.chunker.Chunk(data)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, chunk := range chunks {
		d.index[chunk.Fingerprint] = true
	}

	return nil
}

func (d *chunkedDedup) CheckAndAdd(data []byte) (*DedupResult, error) {
	if d.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	chunks, err := d.chunker.Chunk(data)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	result := &DedupResult{
		IsDuplicate: false,
	}

	for _, chunk := range chunks {
		if d.index[chunk.Fingerprint] {
			result.IsDuplicate = true
			result.MatchedFPs = append(result.MatchedFPs, chunk.Fingerprint)
			result.MatchedChunks = append(result.MatchedChunks, chunk)
		} else {
			d.index[chunk.Fingerprint] = true
		}
	}

	return result, nil
}

func (d *chunkedDedup) Contains(data []byte) (bool, error) {
	if d.closed {
		return false, ErrEngineClosed
	}
	if len(data) == 0 {
		return false, ErrEmptyData
	}

	result, err := d.Check(data)
	if err != nil {
		return false, err
	}

	return result.IsDuplicate, nil
}

func (d *chunkedDedup) Delete(data []byte) error {
	if d.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	chunks, err := d.chunker.Chunk(data)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, chunk := range chunks {
		delete(d.index, chunk.Fingerprint)
	}

	return nil
}

func (d *chunkedDedup) Clear() error {
	if d.closed {
		return ErrEngineClosed
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.index = make(FingerprintIndex)
	return nil
}

func (d *chunkedDedup) Count() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.index)
}

func (d *chunkedDedup) GetIndex() FingerprintIndex {
	d.mu.RLock()
	defer d.mu.RUnlock()

	index := make(FingerprintIndex, len(d.index))
	for k, v := range d.index {
		index[k] = v
	}
	return index
}

func (d *chunkedDedup) SetIndex(index FingerprintIndex) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.index = make(FingerprintIndex, len(index))
	for k, v := range index {
		d.index[k] = v
	}
}

func (d *chunkedDedup) AddFingerprint(fp Fingerprint) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.index[fp] = true
}

func (d *chunkedDedup) Chunker() Chunker {
	return d.chunker
}

func (d *chunkedDedup) HashProvider() HashProvider {
	return d.hashProvider
}

func (d *chunkedDedup) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return ErrEngineClosed
	}

	d.closed = true
	return nil
}
