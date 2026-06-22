package hotconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

type HotConfig struct {
	mu          sync.RWMutex
	path        string
	schema      *Schema
	options     *HotConfigOptions
	snapshot    *ConfigSnapshot
	callbacks   []ChangeCallback
	callbackIDs map[string]int
	nextCBID    int
	running     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
	version     uint64
	lastModTime time.Time
	eventCh     chan *fileEvent
}

func NewHotConfig(path string, schema *Schema, options *HotConfigOptions) (*HotConfig, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: path cannot be empty", ErrInvalidConfigPath)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfigPath, err)
	}

	if options == nil {
		options = DefaultHotConfigOptions()
	}

	hc := &HotConfig{
		path:        absPath,
		schema:      schema,
		options:     options,
		callbacks:   make([]ChangeCallback, 0),
		callbackIDs: make(map[string]int),
		stopCh:      make(chan struct{}),
		eventCh:     make(chan *fileEvent, 16),
	}

	return hc, nil
}

func (hc *HotConfig) Load() error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	_, err := hc.loadLocked()
	return err
}

func (hc *HotConfig) loadLocked() (bool, error) {
	if _, err := os.Stat(hc.path); os.IsNotExist(err) {
		return false, fmt.Errorf("%w: %s", ErrFileNotFound, hc.path)
	}

	data, err := os.ReadFile(hc.path)
	if err != nil {
		return false, fmt.Errorf("failed to read config file: %w", err)
	}

	parsedData, format, err := ParseFile(hc.path, data)
	if err != nil {
		if hc.options.FailOnError {
			return false, err
		}
		if hc.snapshot != nil {
			return false, nil
		}
		parsedData = make(map[string]interface{})
	}

	withDefaults := ApplyDefaults(parsedData, hc.schema)

	validationErr := ValidateConfig(withDefaults, hc.schema)
	if validationErr != nil {
		if hc.options.FailOnError {
			return false, validationErr
		}
		if hc.options.UseDefaultOnError {
			withDefaults = ApplyDefaultsOnValidationFailure(withDefaults, hc.schema, validationErr)
		}
	}

	if hc.snapshot != nil && reflect.DeepEqual(hc.snapshot.Data, withDefaults) {
		if info, err := os.Stat(hc.path); err == nil {
			hc.lastModTime = info.ModTime()
		}
		return false, nil
	}

	hc.version++

	hc.snapshot = &ConfigSnapshot{
		Data:      withDefaults,
		Timestamp: time.Now(),
		Source:    hc.path,
		Format:    format,
		Version:   hc.version,
	}

	if info, err := os.Stat(hc.path); err == nil {
		hc.lastModTime = info.ModTime()
	}

	return true, nil
}

func (hc *HotConfig) Start() error {
	hc.mu.Lock()

	if hc.running {
		hc.mu.Unlock()
		return ErrWatcherAlreadyRunning
	}

	if hc.snapshot == nil {
		if _, err := hc.loadLocked(); err != nil {
			hc.mu.Unlock()
			return err
		}
	}

	hc.running = true
	hc.stopCh = make(chan struct{})
	hc.mu.Unlock()

	hc.wg.Add(2)
	go hc.pollLoop()
	go hc.eventLoop()

	return nil
}

func (hc *HotConfig) Stop() {
	hc.mu.Lock()
	if !hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = false
	close(hc.stopCh)
	hc.mu.Unlock()

	hc.wg.Wait()
}

func (hc *HotConfig) pollLoop() {
	defer hc.wg.Done()

	interval := 50 * time.Millisecond
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.checkFileChange()
		}
	}
}

func (hc *HotConfig) checkFileChange() {
	hc.mu.RLock()
	running := hc.running
	path := hc.path
	lastMod := hc.lastModTime
	hc.mu.RUnlock()

	if !running {
		return
	}

	info, err := os.Stat(path)
	if err != nil {
		return
	}

	if !info.ModTime().After(lastMod) {
		return
	}

	select {
	case hc.eventCh <- &fileEvent{
		path: path,
		time: info.ModTime(),
	}:
	default:
	}
}

func (hc *HotConfig) eventLoop() {
	defer hc.wg.Done()

	var debounceTimer *time.Timer
	var pendingEvent *fileEvent

	for {
		select {
		case <-hc.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case evt := <-hc.eventCh:
			pendingEvent = evt
			debounce := hc.options.DebounceTime
			if debounce <= 0 {
				debounce = 100 * time.Millisecond
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(debounce, func() {
				hc.processEvent(pendingEvent)
			})
		}
	}
}

func (hc *HotConfig) processEvent(evt *fileEvent) {
	if evt == nil {
		return
	}

	hc.mu.Lock()

	if !hc.running {
		hc.mu.Unlock()
		return
	}

	oldSnapshot := hc.snapshot
	oldData := make(map[string]interface{})
	if oldSnapshot != nil {
		oldData = deepCopyMap(oldSnapshot.Data)
	}

	changed, err := hc.loadLocked()
	if err != nil {
		hc.mu.Unlock()
		return
	}

	if !changed {
		hc.mu.Unlock()
		return
	}

	newSnapshot := hc.snapshot
	newData := make(map[string]interface{})
	if newSnapshot != nil {
		newData = deepCopyMap(newSnapshot.Data)
	}

	callbacks := make([]ChangeCallback, len(hc.callbacks))
	copy(callbacks, hc.callbacks)

	hc.mu.Unlock()

	oldSnapCopy := oldSnapshot
	if oldSnapshot != nil {
		oldSnapCopy = &ConfigSnapshot{
			Data:      oldData,
			Timestamp: oldSnapshot.Timestamp,
			Source:    oldSnapshot.Source,
			Format:    oldSnapshot.Format,
			Version:   oldSnapshot.Version,
		}
	}

	newSnapCopy := newSnapshot
	if newSnapshot != nil {
		newSnapCopy = &ConfigSnapshot{
			Data:      newData,
			Timestamp: newSnapshot.Timestamp,
			Source:    newSnapshot.Source,
			Format:    newSnapshot.Format,
			Version:   newSnapshot.Version,
		}
	}

	for _, cb := range callbacks {
		func(callback ChangeCallback) {
			defer func() {
				recover()
			}()
			callback(oldSnapCopy, newSnapCopy)
		}(cb)
	}
}

func (hc *HotConfig) RegisterCallback(callback ChangeCallback) (string, error) {
	if callback == nil {
		return "", ErrNilCallback
	}

	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.nextCBID++
	id := fmt.Sprintf("cb_%d", hc.nextCBID)
	hc.callbacks = append(hc.callbacks, callback)
	hc.callbackIDs[id] = len(hc.callbacks) - 1

	return id, nil
}

func (hc *HotConfig) UnregisterCallback(id string) bool {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	idx, exists := hc.callbackIDs[id]
	if !exists {
		return false
	}

	hc.callbacks = append(hc.callbacks[:idx], hc.callbacks[idx+1:]...)
	delete(hc.callbackIDs, id)

	for cid, cidx := range hc.callbackIDs {
		if cidx > idx {
			hc.callbackIDs[cid] = cidx - 1
		}
	}

	return true
}

func (hc *HotConfig) GetSnapshot() *ConfigSnapshot {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.snapshot == nil {
		return nil
	}

	data := deepCopyMap(hc.snapshot.Data)
	return &ConfigSnapshot{
		Data:      data,
		Timestamp: hc.snapshot.Timestamp,
		Source:    hc.snapshot.Source,
		Format:    hc.snapshot.Format,
		Version:   hc.snapshot.Version,
	}
}

func (hc *HotConfig) Get(key string) (interface{}, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	if hc.snapshot == nil {
		return nil, false
	}

	return getNestedValue(hc.snapshot.Data, key)
}

func (hc *HotConfig) GetString(key string) (string, bool) {
	val, ok := hc.Get(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

func (hc *HotConfig) GetInt(key string) (int, bool) {
	val, ok := hc.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func (hc *HotConfig) GetFloat64(key string) (float64, bool) {
	val, ok := hc.Get(key)
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}

func (hc *HotConfig) GetBool(key string) (bool, bool) {
	val, ok := hc.Get(key)
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

func (hc *HotConfig) Reload() error {
	hc.mu.Lock()

	oldSnapshot := hc.snapshot
	oldData := make(map[string]interface{})
	if oldSnapshot != nil {
		oldData = deepCopyMap(oldSnapshot.Data)
	}

	changed, err := hc.loadLocked()
	if err != nil {
		hc.mu.Unlock()
		return err
	}

	if !changed {
		hc.mu.Unlock()
		return nil
	}

	newSnapshot := hc.snapshot
	newData := make(map[string]interface{})
	if newSnapshot != nil {
		newData = deepCopyMap(newSnapshot.Data)
	}

	callbacks := make([]ChangeCallback, len(hc.callbacks))
	copy(callbacks, hc.callbacks)
	hc.mu.Unlock()

	oldSnapCopy := oldSnapshot
	if oldSnapshot != nil {
		oldSnapCopy = &ConfigSnapshot{
			Data:      oldData,
			Timestamp: oldSnapshot.Timestamp,
			Source:    oldSnapshot.Source,
			Format:    oldSnapshot.Format,
			Version:   oldSnapshot.Version,
		}
	}

	newSnapCopy := newSnapshot
	if newSnapshot != nil {
		newSnapCopy = &ConfigSnapshot{
			Data:      newData,
			Timestamp: newSnapshot.Timestamp,
			Source:    newSnapshot.Source,
			Format:    newSnapshot.Format,
			Version:   newSnapshot.Version,
		}
	}

	for _, cb := range callbacks {
		func(callback ChangeCallback) {
			defer func() {
				recover()
			}()
			callback(oldSnapCopy, newSnapCopy)
		}(cb)
	}

	return nil
}

func (hc *HotConfig) IsRunning() bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.running
}

func (hc *HotConfig) Path() string {
	return hc.path
}

func (hc *HotConfig) Version() uint64 {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.version
}

func (hc *HotConfig) CallbackCount() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return len(hc.callbacks)
}
