package datadedup

import (
	"sync"
)

type dedupEngine struct {
	cfg            Config
	exactEngine    *exactDedup
	fuzzyEngine    *fuzzyDedup
	chunkedEngine  *chunkedDedup
	persister      PersistIndex
	mu             sync.RWMutex
	closed         bool
	opCount        int
}

func NewDedupEngine(cfg Config) (DedupEngine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	engine := &dedupEngine{
		cfg:    cfg,
		closed: false,
	}

	switch cfg.Mode {
	case DedupModeExact:
		var err error
		if cfg.HashProvider != nil {
			engine.exactEngine, err = NewExactDedupWithProvider(cfg.HashProvider)
		} else {
			engine.exactEngine, err = NewExactDedup(cfg.HashAlgorithm)
		}
		if err != nil {
			return nil, err
		}

	case DedupModeFuzzy:
		var err error
		if cfg.SimilarityCalc != nil {
			engine.fuzzyEngine, err = NewFuzzyDedupWithCalculator(cfg.SimilarityCalc, cfg.SimilarityThreshold)
		} else {
			engine.fuzzyEngine, err = NewFuzzyDedup(cfg.SimilarityAlgo, cfg.SimilarityThreshold)
		}
		if err != nil {
			return nil, err
		}

	case DedupModeChunked:
		var err error
		if cfg.Chunker != nil && cfg.HashProvider != nil {
			engine.chunkedEngine, err = NewChunkedDedupWithChunker(cfg.Chunker, cfg.HashProvider)
		} else {
			engine.chunkedEngine, err = NewChunkedDedup(
				cfg.ChunkStrategy,
				cfg.ChunkSize,
				cfg.MinChunkSize,
				cfg.MaxChunkSize,
				cfg.ContentBoundary,
				cfg.HashAlgorithm,
			)
		}
		if err != nil {
			return nil, err
		}
	}

	if cfg.Persister != nil {
		engine.persister = cfg.Persister
	} else {
		engine.persister = NewPersistIndex()
	}

	if cfg.PersistPath != "" {
		if err := engine.Load(cfg.PersistPath); err != nil {
			if err != ErrPersistFileNotExist {
				return nil, err
			}
		}
	}

	return engine, nil
}

func (e *dedupEngine) Check(data []byte) (*DedupResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return nil, ErrEngineClosed
	}

	switch e.cfg.Mode {
	case DedupModeExact:
		return e.exactEngine.Check(data)
	case DedupModeFuzzy:
		return e.fuzzyEngine.Check(data)
	case DedupModeChunked:
		return e.chunkedEngine.Check(data)
	default:
		return nil, ErrInvalidDedupMode
	}
}

func (e *dedupEngine) Add(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	var err error

	switch e.cfg.Mode {
	case DedupModeExact:
		err = e.exactEngine.Add(data)
	case DedupModeFuzzy:
		err = e.fuzzyEngine.Add(data)
	case DedupModeChunked:
		err = e.chunkedEngine.Add(data)
	default:
		return ErrInvalidDedupMode
	}

	if err == nil {
		e.opCount++
		if e.cfg.AutoPersist && e.cfg.PersistPath != "" && e.opCount >= e.cfg.AutoPersistCount {
			_ = e.saveLocked(e.cfg.PersistPath)
			e.opCount = 0
		}
	}

	return err
}

func (e *dedupEngine) CheckAndAdd(data []byte) (*DedupResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil, ErrEngineClosed
	}

	var result *DedupResult
	var err error

	switch e.cfg.Mode {
	case DedupModeExact:
		result, err = e.exactEngine.CheckAndAdd(data)
	case DedupModeFuzzy:
		result, err = e.fuzzyEngine.CheckAndAdd(data)
	case DedupModeChunked:
		result, err = e.chunkedEngine.CheckAndAdd(data)
	default:
		return nil, ErrInvalidDedupMode
	}

	if err == nil && result != nil && !result.IsDuplicate {
		e.opCount++
		if e.cfg.AutoPersist && e.cfg.PersistPath != "" && e.opCount >= e.cfg.AutoPersistCount {
			_ = e.saveLocked(e.cfg.PersistPath)
			e.opCount = 0
		}
	}

	return result, err
}

func (e *dedupEngine) Contains(data []byte) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return false, ErrEngineClosed
	}

	switch e.cfg.Mode {
	case DedupModeExact:
		return e.exactEngine.Contains(data)
	case DedupModeFuzzy:
		return e.fuzzyEngine.Contains(data)
	case DedupModeChunked:
		return e.chunkedEngine.Contains(data)
	default:
		return false, ErrInvalidDedupMode
	}
}

func (e *dedupEngine) Delete(data []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	switch e.cfg.Mode {
	case DedupModeExact:
		return e.exactEngine.Delete(data)
	case DedupModeFuzzy:
		return e.fuzzyEngine.Delete(data)
	case DedupModeChunked:
		return e.chunkedEngine.Delete(data)
	default:
		return ErrInvalidDedupMode
	}
}

func (e *dedupEngine) Clear() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	var err error

	switch e.cfg.Mode {
	case DedupModeExact:
		err = e.exactEngine.Clear()
	case DedupModeFuzzy:
		err = e.fuzzyEngine.Clear()
	case DedupModeChunked:
		err = e.chunkedEngine.Clear()
	default:
		return ErrInvalidDedupMode
	}

	e.opCount = 0
	return err
}

func (e *dedupEngine) Count() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return 0
	}

	switch e.cfg.Mode {
	case DedupModeExact:
		return e.exactEngine.Count()
	case DedupModeFuzzy:
		return e.fuzzyEngine.Count()
	case DedupModeChunked:
		return e.chunkedEngine.Count()
	default:
		return 0
	}
}

func (e *dedupEngine) Save(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	return e.saveLocked(path)
}

func (e *dedupEngine) saveLocked(path string) error {
	var persister indexPersister

	switch e.cfg.Mode {
	case DedupModeExact:
		persister = e.exactEngine
	case DedupModeFuzzy:
		persister = e.fuzzyEngine
	case DedupModeChunked:
		persister = e.chunkedEngine
	default:
		return ErrInvalidDedupMode
	}

	return saveIndex(persister, e.persister, path)
}

func (e *dedupEngine) Load(path string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	var persister indexPersister

	switch e.cfg.Mode {
	case DedupModeExact:
		persister = e.exactEngine
	case DedupModeFuzzy:
		persister = e.fuzzyEngine
	case DedupModeChunked:
		persister = e.chunkedEngine
	default:
		return ErrInvalidDedupMode
	}

	return loadIndex(persister, e.persister, path)
}

func (e *dedupEngine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return ErrEngineClosed
	}

	if e.cfg.PersistPath != "" {
		_ = e.saveLocked(e.cfg.PersistPath)
	}

	switch e.cfg.Mode {
	case DedupModeExact:
		_ = e.exactEngine.Close()
	case DedupModeFuzzy:
		_ = e.fuzzyEngine.Close()
	case DedupModeChunked:
		_ = e.chunkedEngine.Close()
	}

	e.closed = true
	return nil
}

func (e *dedupEngine) Verify(path string) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.closed {
		return false, ErrEngineClosed
	}

	return verifyIndex(e.persister, path)
}

func (e *dedupEngine) Mode() DedupMode {
	return e.cfg.Mode
}
