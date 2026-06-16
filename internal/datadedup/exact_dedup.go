package datadedup

import (
	"sync"
)

type exactDedup struct {
	index        FingerprintIndex
	hashProvider HashProvider
	mu           sync.RWMutex
	closed       bool
}

func NewExactDedup(hashAlgo HashAlgorithm) (*exactDedup, error) {
	hp, err := NewHashProvider(hashAlgo)
	if err != nil {
		return nil, err
	}

	return &exactDedup{
		index:        make(FingerprintIndex),
		hashProvider: hp,
		closed:       false,
	}, nil
}

func NewExactDedupWithProvider(hp HashProvider) (*exactDedup, error) {
	if hp == nil {
		return nil, ErrNilHashProvider
	}

	return &exactDedup{
		index:        make(FingerprintIndex),
		hashProvider: hp,
		closed:       false,
	}, nil
}

func (e *exactDedup) Check(data []byte) (*DedupResult, error) {
	if e.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	fp, err := e.hashProvider.Hash(data)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	exists := e.index[fp]
	result := &DedupResult{
		IsDuplicate: exists,
	}

	if exists {
		result.MatchedFPs = []Fingerprint{fp}
	}

	return result, nil
}

func (e *exactDedup) Add(data []byte) error {
	if e.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	fp, err := e.hashProvider.Hash(data)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.index[fp] = true
	return nil
}

func (e *exactDedup) CheckAndAdd(data []byte) (*DedupResult, error) {
	if e.closed {
		return nil, ErrEngineClosed
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	fp, err := e.hashProvider.Hash(data)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	exists := e.index[fp]
	result := &DedupResult{
		IsDuplicate: exists,
	}

	if exists {
		result.MatchedFPs = []Fingerprint{fp}
	} else {
		e.index[fp] = true
	}

	return result, nil
}

func (e *exactDedup) Contains(data []byte) (bool, error) {
	if e.closed {
		return false, ErrEngineClosed
	}
	if len(data) == 0 {
		return false, ErrEmptyData
	}

	fp, err := e.hashProvider.Hash(data)
	if err != nil {
		return false, err
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.index[fp], nil
}

func (e *exactDedup) Delete(data []byte) error {
	if e.closed {
		return ErrEngineClosed
	}
	if len(data) == 0 {
		return ErrEmptyData
	}

	fp, err := e.hashProvider.Hash(data)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.index, fp)
	return nil
}

func (e *exactDedup) Clear() error {
	if e.closed {
		return ErrEngineClosed
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	e.index = make(FingerprintIndex)
	return nil
}

func (e *exactDedup) Count() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return len(e.index)
}

func (e *exactDedup) GetIndex() FingerprintIndex {
	e.mu.RLock()
	defer e.mu.RUnlock()

	index := make(FingerprintIndex, len(e.index))
	for k, v := range e.index {
		index[k] = v
	}
	return index
}

func (e *exactDedup) SetIndex(index FingerprintIndex) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.index = make(FingerprintIndex, len(index))
	for k, v := range index {
		e.index[k] = v
	}
}

func (e *exactDedup) AddFingerprint(fp Fingerprint) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.index[fp] = true
}

func (e *exactDedup) HashProvider() HashProvider {
	return e.hashProvider
}

func (e *exactDedup) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	e.closed = true
	return nil
}
