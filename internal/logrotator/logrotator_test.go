package logrotator

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input   string
		want    Level
		wantErr bool
	}{
		{"DEBUG", LevelDebug, false},
		{"debug", LevelDebug, false},
		{" Debug ", LevelDebug, false},
		{"INFO", LevelInfo, false},
		{"info", LevelInfo, false},
		{"WARN", LevelWarn, false},
		{"warn", LevelWarn, false},
		{"WARNING", LevelWarn, false},
		{"warning", LevelWarn, false},
		{"ERROR", LevelError, false},
		{"error", LevelError, false},
		{"INVALID", LevelDebug, true},
		{"", LevelDebug, true},
	}

	for _, tt := range tests {
		got, err := ParseLevel(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseLevel(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.RotationMode != RotationModeSize {
		t.Errorf("Default RotationMode = %v, want %v", cfg.RotationMode, RotationModeSize)
	}
	if cfg.MaxFileSize != 100*1024*1024 {
		t.Errorf("Default MaxFileSize = %d, want %d", cfg.MaxFileSize, 100*1024*1024)
	}
	if cfg.MaxBackups != 10 {
		t.Errorf("Default MaxBackups = %d, want 10", cfg.MaxBackups)
	}
	if !cfg.Compress {
		t.Error("Default Compress should be true")
	}
	if cfg.TTL != 7*24*time.Hour {
		t.Errorf("Default TTL = %v, want 7d", cfg.TTL)
	}
	if len(cfg.LevelFileMap) != 4 {
		t.Errorf("Default LevelFileMap size = %d, want 4", len(cfg.LevelFileMap))
	}
}

func TestNewNilConfig(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := DefaultConfig()
	for lvl := range cfg.LevelFileMap {
		cfg.LevelFileMap[lvl] = logPath
	}
	cfg.TTL = 0
	cfg.CleanInterval = 0

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	if err := lr.Log(LevelInfo, "test message"); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if err := lr.Sync(); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "test message") {
		t.Errorf("log content missing message: %s", string(data))
	}
	if !strings.Contains(string(data), "[INFO]") {
		t.Errorf("log content missing level marker: %s", string(data))
	}
}

func TestNewWithEmptyLevelMap(t *testing.T) {
	lr, err := New(&Config{
		TTL:           0,
		CleanInterval: 0,
	})
	if err != nil {
		t.Fatalf("New(empty level map) error = %v", err)
	}
	defer lr.Close()
}

func TestMultiLevelSeparateFiles(t *testing.T) {
	dir := t.TempDir()
	debugPath := filepath.Join(dir, "debug.log")
	infoPath := filepath.Join(dir, "info.log")
	warnPath := filepath.Join(dir, "warn.log")
	errorPath := filepath.Join(dir, "error.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: debugPath,
			LevelInfo:  infoPath,
			LevelWarn:  warnPath,
			LevelError: errorPath,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	if err := lr.Log(LevelDebug, "debug msg"); err != nil {
		t.Fatalf("Log(DEBUG) error = %v", err)
	}
	if err := lr.Log(LevelInfo, "info msg"); err != nil {
		t.Fatalf("Log(INFO) error = %v", err)
	}
	if err := lr.Log(LevelWarn, "warn msg"); err != nil {
		t.Fatalf("Log(WARN) error = %v", err)
	}
	if err := lr.Log(LevelError, "error msg"); err != nil {
		t.Fatalf("Log(ERROR) error = %v", err)
	}
	lr.Sync()

	debugData, _ := os.ReadFile(debugPath)
	if !strings.Contains(string(debugData), "debug msg") {
		t.Error("debug.log should contain debug msg")
	}
	if !strings.Contains(string(debugData), "info msg") {
		t.Error("debug.log should contain info msg (level >= DEBUG)")
	}
	if !strings.Contains(string(debugData), "warn msg") {
		t.Error("debug.log should contain warn msg")
	}
	if !strings.Contains(string(debugData), "error msg") {
		t.Error("debug.log should contain error msg")
	}

	infoData, _ := os.ReadFile(infoPath)
	if strings.Contains(string(infoData), "debug msg") {
		t.Error("info.log should NOT contain debug msg")
	}
	if !strings.Contains(string(infoData), "info msg") {
		t.Error("info.log should contain info msg")
	}
	if !strings.Contains(string(infoData), "warn msg") {
		t.Error("info.log should contain warn msg")
	}
	if !strings.Contains(string(infoData), "error msg") {
		t.Error("info.log should contain error msg")
	}

	warnData, _ := os.ReadFile(warnPath)
	if strings.Contains(string(warnData), "debug msg") {
		t.Error("warn.log should NOT contain debug msg")
	}
	if strings.Contains(string(warnData), "info msg") {
		t.Error("warn.log should NOT contain info msg")
	}
	if !strings.Contains(string(warnData), "warn msg") {
		t.Error("warn.log should contain warn msg")
	}
	if !strings.Contains(string(warnData), "error msg") {
		t.Error("warn.log should contain error msg")
	}

	errorData, _ := os.ReadFile(errorPath)
	if strings.Contains(string(errorData), "debug msg") {
		t.Error("error.log should NOT contain debug msg")
	}
	if strings.Contains(string(errorData), "info msg") {
		t.Error("error.log should NOT contain info msg")
	}
	if strings.Contains(string(errorData), "warn msg") {
		t.Error("error.log should NOT contain warn msg")
	}
	if !strings.Contains(string(errorData), "error msg") {
		t.Error("error.log should contain error msg")
	}
}

func TestAllLevelsSingleFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	lr.Log(LevelDebug, "d1")
	lr.Log(LevelInfo, "i1")
	lr.Log(LevelWarn, "w1")
	lr.Log(LevelError, "e1")
	lr.Sync()

	data, _ := os.ReadFile(logPath)
	content := string(data)
	for _, expected := range []string{"d1", "i1", "w1", "e1"} {
		if !strings.Contains(content, expected) {
			t.Errorf("single file missing %q in: %s", expected, content)
		}
	}
}

func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	msg := strings.Repeat("x", 100)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   150,
		MaxBackups:    3,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	for i := 0; i < 6; i++ {
		if err := lr.Log(LevelInfo, msg); err != nil {
			t.Fatalf("Log() #%d error = %v", i, err)
		}
	}
	lr.Sync()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var logFiles []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "app.") && strings.HasSuffix(e.Name(), ".log") {
			logFiles = append(logFiles, e.Name())
		}
	}

	if len(logFiles) > cfg.MaxBackups+1 {
		t.Errorf("too many backup files: got %d, want <= %d (%v)", len(logFiles), cfg.MaxBackups+1, logFiles)
	}

	_, err = os.Stat(logPath)
	if err != nil {
		t.Errorf("current log file missing: %v", err)
	}
}

func TestRotateByHourly(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	now := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeHourly,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	lr.clock = func() time.Time { return now }
	defer lr.Close()

	if err := lr.Log(LevelInfo, "msg1"); err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	lr.Sync()

	now = now.Add(time.Hour)

	if err := lr.Log(LevelInfo, "msg2"); err != nil {
		t.Fatalf("Log() after hour error = %v", err)
	}
	lr.Sync()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	timeFileCount := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), "2025-01-15-10") {
			timeFileCount++
		}
	}

	if timeFileCount == 0 {
		t.Errorf("expected timestamped backup file not found, got files: %v", listFileNames(entries))
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "msg2") {
		t.Error("current file should have msg2")
	}
}

func TestRotateByDaily(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	now := time.Date(2025, 6, 1, 23, 59, 0, 0, time.UTC)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:   RotationModeDaily,
		Compress:       false,
		TTL:            0,
		CleanInterval:  0,
		FileDateFormat: "2006-01-02",
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	lr.clock = func() time.Time { return now }
	defer lr.Close()

	if err := lr.Log(LevelInfo, "day1-msg"); err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	lr.Sync()

	now = now.Add(2 * time.Minute)

	if err := lr.Log(LevelInfo, "day2-msg"); err != nil {
		t.Fatalf("Log() after day error = %v", err)
	}
	lr.Sync()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	dateFileCount := 0
	for _, e := range entries {
		if strings.Contains(e.Name(), "2025-06-01") {
			dateFileCount++
		}
	}

	if dateFileCount == 0 {
		t.Errorf("expected dated backup file not found, got files: %v", listFileNames(entries))
	}

	data, _ := os.ReadFile(logPath)
	if !strings.Contains(string(data), "day2-msg") {
		t.Error("current file should have day2-msg")
	}
}

func TestCompressBackup(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	msg := strings.Repeat("compress-test-data", 20)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   200,
		MaxBackups:    5,
		Compress:      true,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	for i := 0; i < 5; i++ {
		if err := lr.Log(LevelInfo, msg); err != nil {
			t.Fatalf("Log() #%d error = %v", i, err)
		}
	}

	lr.Sync()
	lr.Close()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	hasGz := false
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".gz") {
			hasGz = true
			gzPath := filepath.Join(dir, e.Name())
			if _, err := os.Stat(gzPath); err != nil {
				t.Fatalf("gz file stat error: %v", err)
			}
			gzFile, err := os.Open(gzPath)
			if err != nil {
				t.Fatalf("open gz error: %v", err)
			}
			gr, err := gzip.NewReader(gzFile)
			if err != nil {
				t.Fatalf("gzip reader error: %v", err)
			}
			content, err := io.ReadAll(gr)
			gr.Close()
			gzFile.Close()
			if err != nil {
				t.Fatalf("read gz content error: %v", err)
			}
			if !strings.Contains(string(content), "compress-test-data") {
				t.Errorf("gz content missing expected data")
			}
			break
		}
	}

	if !hasGz {
		t.Errorf("expected at least one .gz compressed backup, got files: %v", listFileNames(entries))
	}
}

func TestTTLExpiredCleanup(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)
	newTime := now.Add(-1 * time.Hour)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   1000,
		MaxBackups:    100,
		Compress:      false,
		TTL:           3 * 24 * time.Hour,
		CleanInterval: time.Second,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	oldBackup := filepath.Join(dir, "app.1.log")
	if err := os.WriteFile(oldBackup, []byte("old backup content"), 0644); err != nil {
		t.Fatalf("WriteFile old backup error: %v", err)
	}
	os.Chtimes(oldBackup, oldTime, oldTime)

	oldBackupGz := filepath.Join(dir, "app.2.log.gz")
	if err := os.WriteFile(oldBackupGz, []byte("fake gz"), 0644); err != nil {
		t.Fatalf("WriteFile old backup gz error: %v", err)
	}
	os.Chtimes(oldBackupGz, oldTime, oldTime)

	newBackup := filepath.Join(dir, "app.3.log")
	if err := os.WriteFile(newBackup, []byte("new backup content"), 0644); err != nil {
		t.Fatalf("WriteFile new backup error: %v", err)
	}
	os.Chtimes(newBackup, newTime, newTime)

	newBackupGz := filepath.Join(dir, "app.4.log.gz")
	if err := os.WriteFile(newBackupGz, []byte("fake gz new"), 0644); err != nil {
		t.Fatalf("WriteFile new backup gz error: %v", err)
	}
	os.Chtimes(newBackupGz, newTime, newTime)

	lr.CleanExpiredNow()

	if _, err := os.Stat(oldBackup); !os.IsNotExist(err) {
		t.Error("old backup should be deleted by TTL")
	}
	if _, err := os.Stat(oldBackupGz); !os.IsNotExist(err) {
		t.Error("old backup gz should be deleted by TTL")
	}
	if _, err := os.Stat(newBackup); os.IsNotExist(err) {
		t.Error("new backup should NOT be deleted by TTL")
	}
	if _, err := os.Stat(newBackupGz); os.IsNotExist(err) {
		t.Error("new backup gz should NOT be deleted by TTL")
	}
}

func TestMaxBackupsLimit(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	msg := strings.Repeat("data", 25)

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   100,
		MaxBackups:    2,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	for i := 0; i < 20; i++ {
		lr.Log(LevelInfo, msg)
	}
	lr.Sync()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}

	var backups []string
	for _, e := range entries {
		name := e.Name()
		if name != "app.log" && strings.HasPrefix(name, "app.") && strings.HasSuffix(name, ".log") {
			backups = append(backups, name)
		}
	}

	if len(backups) > cfg.MaxBackups {
		t.Errorf("backup count = %d, want <= %d, backups: %v", len(backups), cfg.MaxBackups, backups)
	}
}

func TestNoLevelConfigured(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "only-error.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelError: logPath,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	if err := lr.Log(LevelDebug, "no file for this"); err == nil {
		t.Error("expected error when logging level with no file config")
	}
}

func TestCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub1", "sub2", "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: nested,
			LevelInfo:  nested,
			LevelWarn:  nested,
			LevelError: nested,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	if err := lr.Log(LevelInfo, "nested dir test"); err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	lr.Sync()

	if _, err := os.Stat(nested); err != nil {
		t.Errorf("nested log file not created: %v", err)
	}
}

func TestCloseMultipleTimes(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   100,
		Compress:      true,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	msg := strings.Repeat("x", 50)
	for i := 0; i < 5; i++ {
		lr.Log(LevelInfo, msg)
	}

	if err := lr.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	if err := lr.Close(); err != nil {
		t.Errorf("Second Close() error = %v (expected no panic)", err)
	}
}

func TestSyncNoFiles(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	if err := lr.Sync(); err != nil {
		t.Errorf("Sync() error = %v", err)
	}
}

func TestConcurrentLogs(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   1024 * 1024,
		MaxBackups:    10,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	done := make(chan bool, 4)
	for g := 0; g < 4; g++ {
		go func(id int) {
			for i := 0; i < 200; i++ {
				lvl := Level(id % 4)
				lr.Log(lvl, "goroutine log message")
			}
			done <- true
		}(g)
	}

	for g := 0; g < 4; g++ {
		<-done
	}
	lr.Sync()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	lines := strings.Count(string(data), "\n")
	if lines != 800 {
		t.Errorf("expected 800 lines, got %d", lines)
	}
}

func TestRotateBySizeEdgeCase(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeSize,
		MaxFileSize:   50,
		MaxBackups:    1,
		Compress:      false,
		TTL:           0,
		CleanInterval: 0,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	huge := strings.Repeat("A", 500)
	if err := lr.Log(LevelInfo, huge); err != nil {
		t.Fatalf("Log() huge message error = %v", err)
	}
	lr.Sync()

	entries, _ := os.ReadDir(dir)
	if len(entries) < 1 {
		t.Error("expected at least current file")
	}
}

func TestCleanerWithTTL(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")

	cfg := &Config{
		LevelFileMap: map[Level]string{
			LevelDebug: logPath,
			LevelInfo:  logPath,
			LevelWarn:  logPath,
			LevelError: logPath,
		},
		RotationMode:  RotationModeNone,
		Compress:      false,
		TTL:           50 * time.Millisecond,
		CleanInterval: 20 * time.Millisecond,
	}

	lr, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer lr.Close()

	oldBackup := filepath.Join(dir, "app.old.log")
	os.WriteFile(oldBackup, []byte("old"), 0644)
	os.Chtimes(oldBackup, time.Now().Add(-1*time.Hour), time.Now().Add(-1*time.Hour))

	time.Sleep(200 * time.Millisecond)

	if _, err := os.Stat(oldBackup); !os.IsNotExist(err) {
		t.Error("cleaner goroutine should have deleted old backup")
	}
}

func listFileNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
