package filewatcher

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	fw := New()
	if fw == nil {
		t.Fatal("New returned nil")
	}
	if fw.WatchedDir() != "" {
		t.Errorf("expected empty WatchedDir, got %s", fw.WatchedDir())
	}
	if fw.WatchedFileCount() != 0 {
		t.Errorf("expected WatchedFileCount=0, got %d", fw.WatchedFileCount())
	}
}

func TestNewWithConfig_Defaults(t *testing.T) {
	fw, err := NewWithConfig(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw == nil {
		t.Fatal("NewWithConfig returned nil")
	}
	if fw.cfg.DebounceWindow <= 0 {
		t.Error("DebounceWindow should have default value")
	}
	if fw.cfg.PollInterval <= 0 {
		t.Error("PollInterval should have default value")
	}
}

func TestNewWithConfig_InvalidConfig(t *testing.T) {
	_, err := NewWithConfig(Config{
		DebounceWindow: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative DebounceWindow, got %v", err)
	}

	_, err = NewWithConfig(Config{
		PollInterval: -1 * time.Second,
	})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative PollInterval, got %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DebounceWindow != 100*time.Millisecond {
		t.Errorf("expected DebounceWindow=100ms, got %v", cfg.DebounceWindow)
	}
	if cfg.PollInterval != 50*time.Millisecond {
		t.Errorf("expected PollInterval=50ms, got %v", cfg.PollInterval)
	}
}

func TestWatch_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "definitely", "does", "not", "exist")

	fw := New()
	err := fw.Watch(nonExistentDir)
	if !errors.Is(err, ErrDirNotExist) {
		t.Errorf("expected ErrDirNotExist, got %v", err)
	}
}

func TestWatch_FileInsteadOfDir(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-file-*")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	fw := New()
	err = fw.Watch(tmpFile.Name())
	if !errors.Is(err, ErrDirNotExist) {
		t.Errorf("expected ErrDirNotExist for file path, got %v", err)
	}
}

func TestWatch_Success(t *testing.T) {
	tmpDir := t.TempDir()

	fw := New()
	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	absDir, _ := filepath.Abs(tmpDir)
	if fw.WatchedDir() != absDir {
		t.Errorf("expected WatchedDir=%s, got %s", absDir, fw.WatchedDir())
	}
}

func TestWatch_InitialFilesNotTriggerEvents(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "existing.txt")
	err := os.WriteFile(testFile, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(100 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 0 {
		t.Errorf("expected 0 create events for initial files, got %d", count)
	}

	if fw.WatchedFileCount() != 1 {
		t.Errorf("expected WatchedFileCount=1, got %d", fw.WatchedFileCount())
	}
}

func TestOnCreate_NilCallback(t *testing.T) {
	fw := New()
	err := fw.OnCreate(nil)
	if !errors.Is(err, ErrNilCallback) {
		t.Errorf("expected ErrNilCallback, got %v", err)
	}
}

func TestOnModify_NilCallback(t *testing.T) {
	fw := New()
	err := fw.OnModify(nil)
	if !errors.Is(err, ErrNilCallback) {
		t.Errorf("expected ErrNilCallback, got %v", err)
	}
}

func TestOnDelete_NilCallback(t *testing.T) {
	fw := New()
	err := fw.OnDelete(nil)
	if !errors.Is(err, ErrNilCallback) {
		t.Errorf("expected ErrNilCallback, got %v", err)
	}
}

func TestEventType_String(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventCreate, "create"},
		{EventModify, "modify"},
		{EventDelete, "delete"},
		{EventType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.eventType.String() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, tt.eventType.String())
			}
		})
	}
}

func TestDebounceKey(t *testing.T) {
	evt1 := Event{Type: EventCreate, Path: "/tmp/test.txt"}
	evt2 := Event{Type: EventCreate, Path: "/tmp/test.txt"}
	evt3 := Event{Type: EventModify, Path: "/tmp/test.txt"}

	if debounceKey(evt1) != debounceKey(evt2) {
		t.Error("same event should have same debounce key")
	}
	if debounceKey(evt1) == debounceKey(evt3) {
		t.Error("different event types should have different debounce keys")
	}
}

func TestNormalizeExtensions(t *testing.T) {
	tests := []struct {
		input    []string
		expected []string
	}{
		{[]string{}, []string{}},
		{[]string{".txt"}, []string{".txt"}},
		{[]string{"txt"}, []string{".txt"}},
		{[]string{"TXT", ".Go", " json"}, []string{".txt", ".go", ".json"}},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("case-%d", i), func(t *testing.T) {
			result := normalizeExtensions(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected len %d, got %d", len(tt.expected), len(result))
			}
			for j := range result {
				if result[j] != tt.expected[j] {
					t.Errorf("index %d: expected %q, got %q", j, tt.expected[j], result[j])
				}
			}
		})
	}
}

func TestFileCreateEvent(t *testing.T) {
	tmpDir := t.TempDir()

	var createCount int32
	var lastCreatePath string
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		lastCreatePath = evt.Path
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	testFile := filepath.Join(tmpDir, "newfile.txt")
	err = os.WriteFile(testFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count < 1 {
		t.Errorf("expected at least 1 create event, got %d", count)
	}

	mu.Lock()
	defer mu.Unlock()
	if !stringsHasSuffix(lastCreatePath, "newfile.txt") {
		t.Errorf("expected create event path ending with newfile.txt, got %s", lastCreatePath)
	}
}

func stringsHasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func TestFileModifyEvent(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("v1"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var modifyCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnModify(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&modifyCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	time.Sleep(10 * time.Millisecond)
	err = os.WriteFile(testFile, []byte("v2"), 0644)
	if err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&modifyCount)
	if count < 1 {
		t.Errorf("expected at least 1 modify event, got %d", count)
	}
}

func TestFileDeleteEvent(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "delete-me.txt")
	err := os.WriteFile(testFile, []byte("delete me"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var deleteCount int32
	var lastDeletePath string
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnDelete(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&deleteCount, 1)
		lastDeletePath = evt.Path
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	err = os.Remove(testFile)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&deleteCount)
	if count < 1 {
		t.Errorf("expected at least 1 delete event, got %d", count)
	}

	mu.Lock()
	defer mu.Unlock()
	if !stringsHasSuffix(lastDeletePath, "delete-me.txt") {
		t.Errorf("expected delete event path ending with delete-me.txt, got %s", lastDeletePath)
	}
}

func TestRecursiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	subSubDir := filepath.Join(subDir, "nested")
	err = os.MkdirAll(subSubDir, 0755)
	if err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	deepFile := filepath.Join(subSubDir, "deep.txt")
	err = os.WriteFile(deepFile, []byte("deep"), 0644)
	if err != nil {
		t.Fatalf("failed to create deep file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count < 1 {
		t.Errorf("expected at least 1 create event for nested file, got %d", count)
	}
}

func TestDebounce_DuplicateEventsMerged(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "bounce.txt")

	var modifyCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 100 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnModify(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&modifyCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	for i := 0; i < 5; i++ {
		err = os.WriteFile(testFile, []byte(fmt.Sprintf("v%d", i)), 0644)
		if err != nil {
			t.Fatalf("failed to write iteration %d: %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	count := atomic.LoadInt32(&modifyCount)
	if count > 2 {
		t.Errorf("expected at most 2 modify events due to debounce, got %d", count)
	}
}

func TestFilter_FileExtensions(t *testing.T) {
	tmpDir := t.TempDir()

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			FileExtensions: []string{".txt", ".md"},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(tmpDir, "a.txt"),
		filepath.Join(tmpDir, "b.md"),
		filepath.Join(tmpDir, "c.go"),
		filepath.Join(tmpDir, "d.json"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (txt and md), got %d", count)
	}
}

func TestFilter_FileExtensions_WithoutDot(t *testing.T) {
	tmpDir := t.TempDir()

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			FileExtensions: []string{"txt", "go"},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(tmpDir, "a.txt"),
		filepath.Join(tmpDir, "b.go"),
		filepath.Join(tmpDir, "c.md"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (txt and go), got %d", count)
	}
}

func TestFilter_FilePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			FilePatterns: []string{"test_*", "*.log"},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(tmpDir, "test_abc.txt"),
		filepath.Join(tmpDir, "app.log"),
		filepath.Join(tmpDir, "other.txt"),
		filepath.Join(tmpDir, "test.txt"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (test_* and *.log), got %d", count)
	}
}

func TestFilter_ExcludeDirs(t *testing.T) {
	tmpDir := t.TempDir()

	nodeModulesDir := filepath.Join(tmpDir, "node_modules")
	err := os.MkdirAll(nodeModulesDir, 0755)
	if err != nil {
		t.Fatalf("failed to create node_modules dir: %v", err)
	}

	gitDir := filepath.Join(tmpDir, ".git")
	err = os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}

	srcDir := filepath.Join(tmpDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			ExcludeDirs: []string{"node_modules", ".git"},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(nodeModulesDir, "package.json"),
		filepath.Join(gitDir, "HEAD"),
		filepath.Join(srcDir, "main.go"),
		filepath.Join(tmpDir, "README.md"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (src and root), got %d", count)
	}
}

func TestFilter_IncludePatterns(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	err := os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("failed to create src dir: %v", err)
	}

	testDir := filepath.Join(tmpDir, "test")
	err = os.MkdirAll(testDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			IncludePatterns: []string{filepath.Join(tmpDir, "src", "*")},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(srcDir, "main.go"),
		filepath.Join(srcDir, "util.go"),
		filepath.Join(testDir, "main_test.go"),
		filepath.Join(tmpDir, "go.mod"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (only src/*), got %d", count)
	}
}

func TestStartStop_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	fw := New()
	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	fw.Start()

	fw.Stop()
	fw.Stop()

	done := make(chan struct{})
	go func() {
		fw.Start()
		fw.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Start/Stop deadlocked")
	}
}

func TestStop_RejectsCallbacksRegistration(t *testing.T) {
	fw := New()
	fw.Stop()

	err := fw.OnCreate(func(evt Event) {})
	if !errors.Is(err, ErrWatcherStopped) {
		t.Errorf("expected ErrWatcherStopped from OnCreate after Stop, got %v", err)
	}

	err = fw.OnModify(func(evt Event) {})
	if !errors.Is(err, ErrWatcherStopped) {
		t.Errorf("expected ErrWatcherStopped from OnModify after Stop, got %v", err)
	}

	err = fw.OnDelete(func(evt Event) {})
	if !errors.Is(err, ErrWatcherStopped) {
		t.Errorf("expected ErrWatcherStopped from OnDelete after Stop, got %v", err)
	}
}

func TestStop_RejectsWatch(t *testing.T) {
	tmpDir := t.TempDir()

	fw := New()
	fw.Stop()

	err := fw.Watch(tmpDir)
	if !errors.Is(err, ErrWatcherStopped) {
		t.Errorf("expected ErrWatcherStopped from Watch after Stop, got %v", err)
	}
}

func TestStart_WithoutWatch(t *testing.T) {
	fw := New()
	fw.Start()
	if fw.IsRunning() {
		t.Error("should not be running without watched directory")
	}
	fw.Stop()
}

func TestIsRunning(t *testing.T) {
	tmpDir := t.TempDir()

	fw := New()
	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	if fw.IsRunning() {
		t.Error("should not be running before Start")
	}

	fw.Start()
	if !fw.IsRunning() {
		t.Error("should be running after Start")
	}

	fw.Stop()
	if fw.IsRunning() {
		t.Error("should not be running after Stop")
	}
}

func TestStart_AfterStop(t *testing.T) {
	tmpDir := t.TempDir()

	fw := New()
	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	fw.Stop()

	fw.Start()
	if fw.IsRunning() {
		t.Error("Start after Stop should not restart the watcher")
	}
}

func TestStop_WithoutStart(t *testing.T) {
	fw := New()
	fw.Stop()

	err := fw.OnCreate(func(evt Event) {})
	if !errors.Is(err, ErrWatcherStopped) {
		t.Errorf("expected ErrWatcherStopped after Stop (without prior Start), got %v", err)
	}
}

func TestMultipleCallbacks(t *testing.T) {
	tmpDir := t.TempDir()

	var createCount int32
	var modifyCount int32
	var deleteCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})
	_ = fw.OnModify(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&modifyCount, 1)
		mu.Unlock()
	})
	_ = fw.OnDelete(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&deleteCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	testFile := filepath.Join(tmpDir, "multi.txt")

	err = os.WriteFile(testFile, []byte("v1"), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	err = os.WriteFile(testFile, []byte("v2"), 0644)
	if err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	time.Sleep(80 * time.Millisecond)

	err = os.Remove(testFile)
	if err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	if atomic.LoadInt32(&createCount) < 1 {
		t.Error("expected at least 1 create event")
	}
	if atomic.LoadInt32(&modifyCount) < 1 {
		t.Error("expected at least 1 modify event")
	}
	if atomic.LoadInt32(&deleteCount) < 1 {
		t.Error("expected at least 1 delete event")
	}
}

func TestDebounce_CreateAndModifySameFile(t *testing.T) {
	tmpDir := t.TempDir()

	var eventTypes []EventType
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 50 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		eventTypes = append(eventTypes, EventCreate)
		mu.Unlock()
	})
	_ = fw.OnModify(func(evt Event) {
		mu.Lock()
		eventTypes = append(eventTypes, EventModify)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("v1"), 0644)
	if err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	hasCreate := false
	hasModify := false
	for _, et := range eventTypes {
		if et == EventCreate {
			hasCreate = true
		}
		if et == EventModify {
			hasModify = true
		}
	}

	if !hasCreate {
		t.Error("expected at least one create event")
	}
	if hasModify {
		t.Log("modify event also received (timing dependent)")
	}
}

func TestConcurrent_FileOperations(t *testing.T) {
	tmpDir := t.TempDir()

	var eventCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&eventCount, 1)
		mu.Unlock()
	})

	err := fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	numGoroutines := 10
	filesPerGoroutine := 5

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < filesPerGoroutine; i++ {
				filename := filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d-%d.txt", gid, i))
				err := os.WriteFile(filename, []byte("test"), 0644)
				if err != nil {
					return
				}
			}
		}(g)
	}

	wg.Wait()

	time.Sleep(200 * time.Millisecond)

	count := atomic.LoadInt32(&eventCount)
	expected := int32(numGoroutines * filesPerGoroutine)
	if count < expected {
		t.Errorf("expected at least %d events, got %d", expected, count)
	}
}

func TestWatchedFileCount(t *testing.T) {
	tmpDir := t.TempDir()

	for i := 0; i < 5; i++ {
		f := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		err := os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	subDir := filepath.Join(tmpDir, "sub")
	err := os.MkdirAll(subDir, 0755)
	if err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	for i := 0; i < 3; i++ {
		f := filepath.Join(subDir, fmt.Sprintf("subfile%d.txt", i))
		err := os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create sub file: %v", err)
		}
	}

	fw := New()
	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	if fw.WatchedFileCount() != 8 {
		t.Errorf("expected WatchedFileCount=8, got %d", fw.WatchedFileCount())
	}
}

func TestEventStruct(t *testing.T) {
	evt := Event{
		Type: EventCreate,
		Path: "/tmp/test.txt",
	}

	if evt.Type != EventCreate {
		t.Errorf("expected EventCreate, got %v", evt.Type)
	}
	if evt.Path != "/tmp/test.txt" {
		t.Errorf("expected /tmp/test.txt, got %s", evt.Path)
	}
}

func TestFilter_Combined(t *testing.T) {
	tmpDir := t.TempDir()

	nodeModules := filepath.Join(tmpDir, "node_modules")
	err := os.MkdirAll(nodeModules, 0755)
	if err != nil {
		t.Fatalf("failed to create node_modules: %v", err)
	}

	srcDir := filepath.Join(tmpDir, "src")
	err = os.MkdirAll(srcDir, 0755)
	if err != nil {
		t.Fatalf("failed to create src: %v", err)
	}

	var createCount int32
	var mu sync.Mutex

	fw, _ := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   10 * time.Millisecond,
		Filters: FilterConfig{
			FileExtensions: []string{".go"},
			ExcludeDirs:    []string{"node_modules"},
		},
	})
	_ = fw.OnCreate(func(evt Event) {
		mu.Lock()
		atomic.AddInt32(&createCount, 1)
		mu.Unlock()
	})

	err = fw.Watch(tmpDir)
	if err != nil {
		t.Fatalf("Watch failed: %v", err)
	}

	fw.Start()
	defer fw.Stop()

	time.Sleep(50 * time.Millisecond)

	files := []string{
		filepath.Join(tmpDir, "main.go"),
		filepath.Join(tmpDir, "README.md"),
		filepath.Join(srcDir, "util.go"),
		filepath.Join(srcDir, "test.txt"),
		filepath.Join(nodeModules, "index.js"),
		filepath.Join(nodeModules, "package.json"),
	}

	for _, f := range files {
		err = os.WriteFile(f, []byte("test"), 0644)
		if err != nil {
			t.Fatalf("failed to create %s: %v", f, err)
		}
	}

	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt32(&createCount)
	if count != 2 {
		t.Errorf("expected 2 create events (only .go files, excluding node_modules), got %d", count)
	}
}

func TestPassesFilters_NoFilters(t *testing.T) {
	fw := New()

	if !fw.passesFilters("/any/file.txt") {
		t.Error("should pass with no filters")
	}
	if !fw.passesFilters("/any/other/file.go") {
		t.Error("should pass with no filters")
	}
}

func TestIsExcludedDir_NoExcludes(t *testing.T) {
	fw := New()

	if fw.isExcludedDir("/tmp/test") {
		t.Error("should not exclude with no exclude dirs")
	}
}

func TestIsExcludedDir_ByName(t *testing.T) {
	fw, _ := NewWithConfig(Config{
		Filters: FilterConfig{
			ExcludeDirs: []string{"node_modules"},
		},
	})

	if !fw.isExcludedDir("/project/node_modules") {
		t.Error("should exclude node_modules by name")
	}
	if !fw.isExcludedDir("/project/sub/node_modules") {
		t.Error("should exclude nested node_modules by name")
	}
	if fw.isExcludedDir("/project/src") {
		t.Error("should not exclude src")
	}
}

func TestDebounceWindow_ZeroUsesDefault(t *testing.T) {
	fw, err := NewWithConfig(Config{
		DebounceWindow: 0,
		PollInterval:   20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.cfg.DebounceWindow <= 0 {
		t.Error("DebounceWindow should have default value when 0")
	}
}

func TestPollInterval_ZeroUsesDefault(t *testing.T) {
	fw, err := NewWithConfig(Config{
		DebounceWindow: 20 * time.Millisecond,
		PollInterval:   0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fw.cfg.PollInterval <= 0 {
		t.Error("PollInterval should have default value when 0")
	}
}
