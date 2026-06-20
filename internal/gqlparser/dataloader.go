package gqlparser

import (
	"time"
)

const defaultBatchWindow = 200 * time.Microsecond

func NewDataLoader(fn DataLoaderFunc) *DataLoader {
	return NewDataLoaderWithWindow(fn, defaultBatchWindow)
}

func NewDataLoaderWithWindow(fn DataLoaderFunc, batchWindow time.Duration) *DataLoader {
	return &DataLoader{
		fn:          fn,
		pending:     make([]*loaderRequest, 0),
		batchWindow: batchWindow,
	}
}

func (dl *DataLoader) Load(key interface{}) (interface{}, error) {
	dl.mu.Lock()

	req := &loaderRequest{
		key:    key,
		result: make(chan loaderResult, 1),
	}
	dl.pending = append(dl.pending, req)

	if dl.batchTimer == nil && dl.batchWindow > 0 {
		dl.batchTimer = time.AfterFunc(dl.batchWindow, func() {
			dl.Flush()
		})
	}

	dl.mu.Unlock()

	res := <-req.result
	return res.value, res.err
}

func (dl *DataLoader) LoadMany(keys []interface{}) ([]interface{}, []error) {
	results := make([]interface{}, len(keys))
	errs := make([]error, len(keys))

	requests := make([]*loaderRequest, len(keys))
	dl.mu.Lock()
	for i, key := range keys {
		req := &loaderRequest{
			key:    key,
			result: make(chan loaderResult, 1),
		}
		requests[i] = req
		dl.pending = append(dl.pending, req)
	}

	if dl.batchTimer == nil && dl.batchWindow > 0 {
		dl.batchTimer = time.AfterFunc(dl.batchWindow, func() {
			dl.Flush()
		})
	}

	dl.mu.Unlock()

	for i, req := range requests {
		res := <-req.result
		results[i] = res.value
		errs[i] = res.err
	}

	return results, errs
}

func (dl *DataLoader) Flush() error {
	dl.mu.Lock()

	if dl.batchTimer != nil {
		dl.batchTimer.Stop()
		dl.batchTimer = nil
	}

	if len(dl.pending) == 0 {
		dl.mu.Unlock()
		return nil
	}

	requests := dl.pending
	dl.pending = make([]*loaderRequest, 0)
	dl.mu.Unlock()

	keys := make([]interface{}, len(requests))
	for i, req := range requests {
		keys[i] = req.key
	}

	values, err := dl.fn(keys)
	if err != nil {
		for _, req := range requests {
			req.result <- loaderResult{nil, err}
		}
		return err
	}

	for i, req := range requests {
		var val interface{}
		var reqErr error
		if i < len(values) {
			val = values[i]
		} else {
			reqErr = ErrDataLoaderNotReady
		}
		req.result <- loaderResult{val, reqErr}
	}

	return nil
}

func (dl *DataLoader) Clear(key interface{}) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	newPending := make([]*loaderRequest, 0, len(dl.pending))
	for _, req := range dl.pending {
		if req.key != key {
			newPending = append(newPending, req)
		} else {
			req.result <- loaderResult{nil, ErrDataLoaderCleared}
		}
	}
	dl.pending = newPending

	if len(dl.pending) == 0 && dl.batchTimer != nil {
		dl.batchTimer.Stop()
		dl.batchTimer = nil
	}
}

func (dl *DataLoader) ClearAll() {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	for _, req := range dl.pending {
		req.result <- loaderResult{nil, ErrDataLoaderCleared}
	}
	dl.pending = make([]*loaderRequest, 0)

	if dl.batchTimer != nil {
		dl.batchTimer.Stop()
		dl.batchTimer = nil
	}
}

func (dl *DataLoader) SetBatchWindow(d time.Duration) {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	dl.batchWindow = d
	if d <= 0 && dl.batchTimer != nil {
		dl.batchTimer.Stop()
		dl.batchTimer = nil
	}
}

func (dl *DataLoader) ResetBatchWindow() {
	dl.mu.Lock()
	defer dl.mu.Unlock()

	if dl.batchWindow <= 0 {
		return
	}
	if dl.batchTimer != nil {
		dl.batchTimer.Stop()
	}
	dl.batchTimer = time.AfterFunc(dl.batchWindow, func() {
		dl.Flush()
	})
}
