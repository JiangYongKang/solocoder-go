package datadedup

import (
	"sync"
)

const (
	rollingHashBase    uint64 = 257
	rollingHashMod     uint64 = 1000000007
	rollingHashDefaultWindowSize = 48
	rollingHashDefaultTargetBits = 13
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

type rollingHash struct {
	window     []byte
	windowSize int
	head       int
	count      int
	hash       uint64
	power      uint64
}

func newRollingHash(windowSize int) *rollingHash {
	power := uint64(1)
	for i := 0; i < windowSize-1; i++ {
		power = (power * rollingHashBase) % rollingHashMod
	}

	return &rollingHash{
		window:     make([]byte, windowSize),
		windowSize: windowSize,
		power:      power,
	}
}

func (rh *rollingHash) appendByte(b byte) {
	if rh.count < rh.windowSize {
		rh.window[rh.count] = b
		rh.hash = (rh.hash*rollingHashBase + uint64(b)) % rollingHashMod
		rh.count++
		return
	}

	outgoing := rh.window[rh.head]
	rh.hash = (rh.hash + rollingHashMod - (uint64(outgoing)*rh.power)%rollingHashMod) % rollingHashMod
	rh.hash = (rh.hash*rollingHashBase + uint64(b)) % rollingHashMod
	rh.window[rh.head] = b
	rh.head = (rh.head + 1) % rh.windowSize
}

func (rh *rollingHash) reset() {
	rh.head = 0
	rh.count = 0
	rh.hash = 0
}

func (rh *rollingHash) full() bool {
	return rh.count >= rh.windowSize
}

func (rh *rollingHash) value() uint64 {
	return rh.hash
}

type contentBasedChunker struct {
	minChunkSize int
	maxChunkSize int
	boundary     byte
	hashProvider HashProvider
	rh           *rollingHash
	targetMask   uint64
}

func NewContentBasedChunker(minSize, maxSize int, boundary byte, hashAlgo HashAlgorithm) (Chunker, error) {
	if minSize <= 0 || maxSize < minSize {
		return nil, ErrInvalidChunkSize
	}

	hp, err := NewHashProvider(hashAlgo)
	if err != nil {
		return nil, err
	}

	windowSize := calcRollingWindowSize(minSize)
	rh := newRollingHash(windowSize)
	targetMask := calcTargetMask(minSize)

	return &contentBasedChunker{
		minChunkSize: minSize,
		maxChunkSize: maxSize,
		boundary:     boundary,
		hashProvider: hp,
		rh:           rh,
		targetMask:   targetMask,
	}, nil
}

func NewContentBasedChunkerWithProvider(minSize, maxSize int, boundary byte, hp HashProvider) (Chunker, error) {
	if minSize <= 0 || maxSize < minSize {
		return nil, ErrInvalidChunkSize
	}
	if hp == nil {
		return nil, ErrNilHashProvider
	}

	windowSize := calcRollingWindowSize(minSize)
	rh := newRollingHash(windowSize)
	targetMask := calcTargetMask(minSize)

	return &contentBasedChunker{
		minChunkSize: minSize,
		maxChunkSize: maxSize,
		boundary:     boundary,
		hashProvider: hp,
		rh:           rh,
		targetMask:   targetMask,
	}, nil
}

func calcRollingWindowSize(minChunkSize int) int {
	defaultSize := rollingHashDefaultWindowSize
	if minChunkSize >= defaultSize {
		return defaultSize
	}
	windowSize := minChunkSize / 2
	if windowSize < 4 {
		windowSize = 4
	}
	if windowSize > minChunkSize {
		windowSize = minChunkSize
	}
	return windowSize
}

func calcTargetMask(minChunkSize int) uint64 {
	targetBits := rollingHashDefaultTargetBits
	if minChunkSize < 256 {
		targetBits = 8
	} else if minChunkSize < 1024 {
		targetBits = 10
	}
	return uint64((1 << targetBits) - 1)
}

func (c *contentBasedChunker) Chunk(data []byte) ([]Chunk, error) {
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	var chunks []Chunk
	dataLen := int64(len(data))
	var start int64

	for start < dataLen {
		c.rh.reset()

		remaining := dataLen - start
		if remaining <= int64(c.minChunkSize) {
			chunkData := data[start:dataLen]
			fp, err := c.hashProvider.Hash(chunkData)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, Chunk{
				Data:        chunkData,
				Offset:      start,
				Fingerprint: fp,
			})
			break
		}

		maxEnd := start + int64(c.maxChunkSize)
		if maxEnd > dataLen {
			maxEnd = dataLen
		}

		minEnd := start + int64(c.minChunkSize)

		var end int64 = maxEnd
		foundBoundary := false

		for i := start; i < maxEnd; i++ {
			c.rh.appendByte(data[i])

			if i >= minEnd-1 {
				if data[i] == c.boundary {
					end = i + 1
					foundBoundary = true
					break
				}

				if c.rh.full() && (c.rh.value()&c.targetMask) == 0 {
					end = i + 1
					foundBoundary = true
					break
				}
			}
		}

		if !foundBoundary {
			end = maxEnd
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

		start = end
	}

	return chunks, nil
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
