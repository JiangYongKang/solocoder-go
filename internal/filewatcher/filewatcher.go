package filewatcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	ErrWatcherStopped    = errors.New("filewatcher: watcher is stopped")
	ErrInvalidConfig     = errors.New("filewatcher: invalid config")
	ErrNilCallback       = errors.New("filewatcher: nil callback")
	ErrNoWatchedDir      = errors.New("filewatcher: no watched directory")
	ErrDirNotExist       = errors.New("filewatcher: watched directory does not exist")
	ErrAlreadyRunning    = errors.New("filewatcher: watcher is already running")
)

type EventType int

const (
	EventCreate EventType = iota
	EventModify
	EventDelete
)

func (e EventType) String() string {
	switch e {
	case EventCreate:
		return "create"
	case EventModify:
		return "modify"
	case EventDelete:
		return "delete"
	default:
		return "unknown"
	}
}

type Event struct {
	Type EventType
	Path string
}

type EventCallback func(event Event)

type FilterConfig struct {
	FilePatterns    []string
	FileExtensions  []string
	ExcludeDirs     []string
	IncludePatterns []string
}

type Config struct {
	DebounceWindow time.Duration
	PollInterval   time.Duration
	Filters        FilterConfig
}

func DefaultConfig() Config {
	return Config{
		DebounceWindow: 100 * time.Millisecond,
		PollInterval:   50 * time.Millisecond,
		Filters:        FilterConfig{},
	}
}

type FileWatcher struct {
	cfg       Config
	watchedDir string

	mu        sync.Mutex
	running   bool
	stopped   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup

	fileStates  map[string]time.Time
	pendingEvents map[string]Event
	debounceTimers map[string]*time.Timer

	onCreate EventCallback
	onModify EventCallback
	onDelete EventCallback
}

func New() (*FileWatcher, error) {
	return NewWithConfig(DefaultConfig())
}

func NewWithConfig(cfg Config) (*FileWatcher, error) {
	if cfg.DebounceWindow < 0 {
		return nil, ErrInvalidConfig
	}
	if cfg.PollInterval < 0 {
		return nil, ErrInvalidConfig
	}

	if cfg.DebounceWindow == 0 {
		cfg.DebounceWindow = 100 * time.Millisecond
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 50 * time.Millisecond
	}

	cfg.Filters.FileExtensions = normalizeExtensions(cfg.Filters.FileExtensions)

	fw := &FileWatcher{
		cfg:            cfg,
		fileStates:     make(map[string]time.Time),
		pendingEvents:  make(map[string]Event),
		debounceTimers: make(map[string]*time.Timer),
		stopCh:         make(chan struct{}),
	}
	return fw, nil
}

func normalizeExtensions(exts []string) []string {
	if len(exts) == 0 {
		return exts
	}
	normalized := make([]string, len(exts))
	for i, ext := range exts {
		e := strings.ToLower(strings.TrimSpace(ext))
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		normalized[i] = e
	}
	return normalized
}

func (fw *FileWatcher) Watch(dir string) error {
	fw.mu.Lock()
	if fw.stopped {
		fw.mu.Unlock()
		return ErrWatcherStopped
	}
	fw.mu.Unlock()

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrDirNotExist
		}
		return err
	}
	if !info.IsDir() {
		return ErrDirNotExist
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	fw.mu.Lock()
	fw.watchedDir = absDir
	fw.scanInitial()
	fw.mu.Unlock()

	return nil
}

func (fw *FileWatcher) scanInitial() {
	if fw.watchedDir == "" {
		return
	}

	_ = filepath.Walk(fw.watchedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if fw.isExcludedDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !fw.passesFilters(path) {
			return nil
		}
		fw.fileStates[path] = info.ModTime()
		return nil
	})
}

func (fw *FileWatcher) OnCreate(cb EventCallback) error {
	if cb == nil {
		return ErrNilCallback
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.stopped {
		return ErrWatcherStopped
	}
	fw.onCreate = cb
	return nil
}

func (fw *FileWatcher) OnModify(cb EventCallback) error {
	if cb == nil {
		return ErrNilCallback
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.stopped {
		return ErrWatcherStopped
	}
	fw.onModify = cb
	return nil
}

func (fw *FileWatcher) OnDelete(cb EventCallback) error {
	if cb == nil {
		return ErrNilCallback
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()

	if fw.stopped {
		return ErrWatcherStopped
	}
	fw.onDelete = cb
	return nil
}

func (fw *FileWatcher) Start() error {
	fw.mu.Lock()
	if fw.stopped {
		fw.mu.Unlock()
		return ErrWatcherStopped
	}
	if fw.running {
		fw.mu.Unlock()
		return ErrAlreadyRunning
	}
	if fw.watchedDir == "" {
		fw.mu.Unlock()
		return ErrNoWatchedDir
	}
	fw.running = true
	fw.stopCh = make(chan struct{})
	fw.mu.Unlock()

	fw.wg.Add(1)
	go fw.pollLoop()
	return nil
}

func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	if fw.stopped {
		fw.mu.Unlock()
		return
	}
	fw.stopped = true
	if fw.running {
		fw.running = false
		close(fw.stopCh)
	}
	for _, timer := range fw.debounceTimers {
		timer.Stop()
	}
	fw.debounceTimers = make(map[string]*time.Timer)
	fw.mu.Unlock()

	fw.wg.Wait()
}

func (fw *FileWatcher) pollLoop() {
	defer fw.wg.Done()

	ticker := time.NewTicker(fw.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-fw.stopCh:
			return
		case <-ticker.C:
			fw.poll()
		}
	}
}

func (fw *FileWatcher) poll() {
	fw.mu.Lock()
	if fw.stopped || fw.watchedDir == "" {
		fw.mu.Unlock()
		return
	}

	currentFiles := make(map[string]time.Time)

	_ = filepath.Walk(fw.watchedDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if fw.isExcludedDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if !fw.passesFilters(path) {
			return nil
		}
		currentFiles[path] = info.ModTime()
		return nil
	})

	var createEvents []Event
	var modifyEvents []Event

	for path, modTime := range currentFiles {
		prevTime, exists := fw.fileStates[path]
		if !exists {
			createEvents = append(createEvents, Event{Type: EventCreate, Path: path})
		} else if !modTime.Equal(prevTime) {
			modifyEvents = append(modifyEvents, Event{Type: EventModify, Path: path})
		}
	}

	var deleteEvents []Event
	for path := range fw.fileStates {
		if _, exists := currentFiles[path]; !exists {
			deleteEvents = append(deleteEvents, Event{Type: EventDelete, Path: path})
		}
	}

	fw.fileStates = currentFiles

	allEvents := make([]Event, 0, len(createEvents)+len(modifyEvents)+len(deleteEvents))
	allEvents = append(allEvents, createEvents...)
	allEvents = append(allEvents, modifyEvents...)
	allEvents = append(allEvents, deleteEvents...)

	for _, evt := range allEvents {
		fw.scheduleDebounced(evt)
	}

	fw.mu.Unlock()
}

func (fw *FileWatcher) scheduleDebounced(evt Event) {
	key := debounceKey(evt)

	if timer, exists := fw.debounceTimers[key]; exists {
		timer.Stop()
	}

	fw.pendingEvents[key] = evt

	timer := time.AfterFunc(fw.cfg.DebounceWindow, func() {
		fw.mu.Lock()
		event, ok := fw.pendingEvents[key]
		if ok {
			delete(fw.pendingEvents, key)
			delete(fw.debounceTimers, key)
		}
		fw.mu.Unlock()

		if ok {
			fw.dispatchEvent(event)
		}
	})

	fw.debounceTimers[key] = timer
}

func debounceKey(evt Event) string {
	return evt.Type.String() + ":" + evt.Path
}

func (fw *FileWatcher) dispatchEvent(evt Event) {
	fw.mu.Lock()
	if fw.stopped {
		fw.mu.Unlock()
		return
	}
	var cb EventCallback
	switch evt.Type {
	case EventCreate:
		cb = fw.onCreate
	case EventModify:
		cb = fw.onModify
	case EventDelete:
		cb = fw.onDelete
	}
	fw.mu.Unlock()

	if cb != nil {
		cb(evt)
	}
}

func (fw *FileWatcher) passesFilters(path string) bool {
	filters := fw.cfg.Filters

	if len(filters.FileExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(path))
		matched := false
		for _, e := range filters.FileExtensions {
			if ext == e {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(filters.FilePatterns) > 0 {
		base := filepath.Base(path)
		matched := false
		for _, pattern := range filters.FilePatterns {
			ok, _ := filepath.Match(pattern, base)
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(filters.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range filters.IncludePatterns {
			ok, _ := filepath.Match(pattern, path)
			if ok {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func (fw *FileWatcher) isExcludedDir(path string) bool {
	if len(fw.cfg.Filters.ExcludeDirs) == 0 {
		return false
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	base := filepath.Base(absPath)

	for _, excludeDir := range fw.cfg.Filters.ExcludeDirs {
		if base == excludeDir {
			return true
		}
		absExclude, err := filepath.Abs(excludeDir)
		if err == nil && absPath == absExclude {
			return true
		}
		if strings.HasPrefix(absPath, absExclude+string(os.PathSeparator)) {
			return true
		}
	}

	return false
}

func (fw *FileWatcher) IsRunning() bool {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.running
}

func (fw *FileWatcher) WatchedDir() string {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return fw.watchedDir
}

func (fw *FileWatcher) WatchedFileCount() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return len(fw.fileStates)
}
