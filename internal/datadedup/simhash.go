package datadedup

import (
	"encoding/binary"
	"encoding/hex"
	"hash/fnv"
	"sync"
	"unicode"
)

const simHashBits = 64

type simHashCalc struct {
	stopWords map[string]bool
}

func NewSimHashCalculator() SimilarityCalculator {
	return &simHashCalc{
		stopWords: make(map[string]bool),
	}
}

func (s *simHashCalc) Calculate(data []byte) (Fingerprint, error) {
	if len(data) == 0 {
		return "", ErrEmptyData
	}

	tokens := s.tokenize(string(data))
	if len(tokens) == 0 {
		tokens = []string{string(data)}
	}

	weights := s.calculateWeights(tokens)
	hash := s.computeSimHash(tokens, weights)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, hash)
	fp := hex.EncodeToString(buf)
	return Fingerprint(fp), nil
}

func (s *simHashCalc) Similarity(fp1, fp2 Fingerprint) (float64, error) {
	if len(fp1) == 0 || len(fp2) == 0 {
		return 0, ErrInvalidFingerprint
	}

	h1, err := hex.DecodeString(string(fp1))
	if err != nil {
		return 0, ErrInvalidFingerprint
	}
	h2, err := hex.DecodeString(string(fp2))
	if err != nil {
		return 0, ErrInvalidFingerprint
	}

	if len(h1) != 8 || len(h2) != 8 {
		return 0, ErrInvalidFingerprint
	}

	hash1 := binary.BigEndian.Uint64(h1)
	hash2 := binary.BigEndian.Uint64(h2)

	distance := s.hammingDistance(hash1, hash2)
	similarity := 1.0 - float64(distance)/simHashBits
	return similarity, nil
}

func (s *simHashCalc) Algorithm() SimilarityAlgorithm {
	return SimilarityAlgorithmSimHash
}

func (s *simHashCalc) tokenize(text string) []string {
	var tokens []string
	var current []rune

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current = append(current, unicode.ToLower(r))
		} else {
			if len(current) > 0 {
				tokens = append(tokens, string(current))
				current = nil
			}
		}
	}

	if len(current) > 0 {
		tokens = append(tokens, string(current))
	}

	var result []string
	for _, token := range tokens {
		if len(token) >= 2 && !s.stopWords[token] {
			result = append(result, token)
		}
	}

	return result
}

func (s *simHashCalc) calculateWeights(tokens []string) map[string]float64 {
	freq := make(map[string]int)
	for _, token := range tokens {
		freq[token]++
	}

	maxFreq := 0
	for _, f := range freq {
		if f > maxFreq {
			maxFreq = f
		}
	}

	weights := make(map[string]float64)
	for token, f := range freq {
		if maxFreq > 0 {
			weights[token] = 0.5 + 0.5*float64(f)/float64(maxFreq)
		} else {
			weights[token] = 1.0
		}
	}

	return weights
}

func (s *simHashCalc) computeSimHash(tokens []string, weights map[string]float64) uint64 {
	vector := make([]float64, simHashBits)

	for _, token := range tokens {
		weight := weights[token]
		h := s.fnvHash(token)

		for i := 0; i < simHashBits; i++ {
			bit := (h >> uint(i)) & 1
			if bit == 1 {
				vector[i] += weight
			} else {
				vector[i] -= weight
			}
		}
	}

	var hash uint64
	for i := 0; i < simHashBits; i++ {
		if vector[i] > 0 {
			hash |= 1 << uint(i)
		}
	}

	return hash
}

func (s *simHashCalc) fnvHash(text string) uint64 {
	h := fnv.New64()
	h.Write([]byte(text))
	return h.Sum64()
}

func (s *simHashCalc) hammingDistance(h1, h2 uint64) int {
	xor := h1 ^ h2
	distance := 0
	for xor != 0 {
		distance++
		xor &= xor - 1
	}
	return distance
}

type fuzzyDedup struct {
	index            FingerprintIndex
	simHashList      []Fingerprint
	similarityCalc   SimilarityCalculator
	threshold        float64
	mu               sync.RWMutex
	closed           bool
}

func NewFuzzyDedup(algo SimilarityAlgorithm, threshold float64) (*fuzzyDedup, error) {
	if threshold < 0 || threshold > 1 {
		return nil, ErrInvalidThreshold
	}

	var calc SimilarityCalculator
	switch algo {
	case SimilarityAlgorithmSimHash:
		calc = NewSimHashCalculator()
	default:
		return nil, ErrUnsupportedSimAlgo
	}

	return &fuzzyDedup{
		index:          make(FingerprintIndex),
		simHashList:    make([]Fingerprint, 0),
		similarityCalc: calc,
		threshold:      threshold,
		closed:         false,
	}, nil
}

func NewFuzzyDedupWithCalculator(calc SimilarityCalculator, threshold float64) (*fuzzyDedup, error) {
	if calc == nil {
		return nil, ErrNilSimilarityCalculator
	}
	if threshold < 0 || threshold > 1 {
		return nil, ErrInvalidThreshold
	}

	return &fuzzyDedup{
		index:          make(FingerprintIndex),
		simHashList:    make([]Fingerprint, 0),
		similarityCalc: calc,
		threshold:      threshold,
		closed:         false,
	}, nil
}

func (f *fuzzyDedup) Check(data []byte) (*DedupResult, error) {
	if f.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	fp, err := f.similarityCalc.Calculate(data)
	if err != nil {
		return nil, err
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	result := &DedupResult{
		IsDuplicate: false,
	}

	maxSimilarity := 0.0
	for _, existingFp := range f.simHashList {
		sim, err := f.similarityCalc.Similarity(fp, existingFp)
		if err != nil {
			continue
		}
		if sim > maxSimilarity {
			maxSimilarity = sim
		}
		if sim >= f.threshold {
			result.IsDuplicate = true
			result.MatchedFPs = append(result.MatchedFPs, existingFp)
			result.Similarity = sim
		}
	}

	if !result.IsDuplicate {
		result.Similarity = maxSimilarity
	}

	return result, nil
}

func (f *fuzzyDedup) Add(data []byte) error {
	if f.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	fp, err := f.similarityCalc.Calculate(data)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.index[fp] {
		f.index[fp] = true
		f.simHashList = append(f.simHashList, fp)
	}

	return nil
}

func (f *fuzzyDedup) CheckAndAdd(data []byte) (*DedupResult, error) {
	if f.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	fp, err := f.similarityCalc.Calculate(data)
	if err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	result := &DedupResult{
		IsDuplicate: false,
	}

	maxSimilarity := 0.0
	for _, existingFp := range f.simHashList {
		sim, err := f.similarityCalc.Similarity(fp, existingFp)
		if err != nil {
			continue
		}
		if sim > maxSimilarity {
			maxSimilarity = sim
		}
		if sim >= f.threshold {
			result.IsDuplicate = true
			result.MatchedFPs = append(result.MatchedFPs, existingFp)
			result.Similarity = sim
		}
	}

	if !result.IsDuplicate {
		result.Similarity = maxSimilarity
		if !f.index[fp] {
			f.index[fp] = true
			f.simHashList = append(f.simHashList, fp)
		}
	}

	return result, nil
}

func (f *fuzzyDedup) Contains(data []byte) (bool, error) {
	if f.closed {
		return false, ErrEngineClosed
	}
	if len(data) == 0 {
		return false, ErrEmptyData
	}

	result, err := f.Check(data)
	if err != nil {
		return false, err
	}

	return result.IsDuplicate, nil
}

func (f *fuzzyDedup) Delete(data []byte) error {
	if f.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	fp, err := f.similarityCalc.Calculate(data)
	if err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.index, fp)
	for i, existingFp := range f.simHashList {
		if existingFp == fp {
			f.simHashList = append(f.simHashList[:i], f.simHashList[i+1:]...)
			break
		}
	}

	return nil
}

func (f *fuzzyDedup) Clear() error {
	if f.closed {
		return ErrEngineClosed
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.index = make(FingerprintIndex)
	f.simHashList = make([]Fingerprint, 0)
	return nil
}

func (f *fuzzyDedup) Count() int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return len(f.simHashList)
}

func (f *fuzzyDedup) Threshold() float64 {
	return f.threshold
}

func (f *fuzzyDedup) SimilarityCalculator() SimilarityCalculator {
	return f.similarityCalc
}

func (f *fuzzyDedup) GetIndex() FingerprintIndex {
	f.mu.RLock()
	defer f.mu.RUnlock()

	index := make(FingerprintIndex, len(f.index))
	for k, v := range f.index {
		index[k] = v
	}
	return index
}

func (f *fuzzyDedup) SetIndex(index FingerprintIndex) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.index = make(FingerprintIndex, len(index))
	f.simHashList = make([]Fingerprint, 0, len(index))
	for k, v := range index {
		f.index[k] = v
		f.simHashList = append(f.simHashList, k)
	}
}

func (f *fuzzyDedup) AddFingerprint(fp Fingerprint) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if !f.index[fp] {
		f.index[fp] = true
		f.simHashList = append(f.simHashList, fp)
	}
}

func (f *fuzzyDedup) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.closed {
		return ErrEngineClosed
	}

	f.closed = true
	return nil
}
