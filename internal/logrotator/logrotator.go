package logrotator

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type fileWriter struct {
	path string
	file *os.File
	size int64
	date string
}

type LogRotator struct {
	config *Config

	writers   map[string]*fileWriter
	mu        sync.Mutex
	closed    bool

	ctx       context.Context
	cancel    context.CancelFunc
	cleanOnce sync.Once
	wg        sync.WaitGroup

	clock func() time.Time
}

func New(config *Config) (*LogRotator, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if config.LevelFileMap == nil || len(config.LevelFileMap) == 0 {
		config.LevelFileMap = DefaultConfig().LevelFileMap
	}
	if config.FileDateFormat == "" {
		config.FileDateFormat = DefaultConfig().FileDateFormat
	}
	if config.CleanInterval <= 0 {
		config.CleanInterval = time.Hour
	}

	lr := &LogRotator{
		config:  config,
		writers: make(map[string]*fileWriter),
		clock:   time.Now,
	}
	lr.ctx, lr.cancel = context.WithCancel(context.Background())

	if err := lr.initWriters(); err != nil {
		return nil, err
	}

	lr.startCleaner()

	return lr, nil
}

func (lr *LogRotator) initWriters() error {
	uniquePaths := make(map[string]bool)
	for _, p := range lr.config.LevelFileMap {
		uniquePaths[p] = true
	}
	for path := range uniquePaths {
		fw, err := lr.openWriter(path)
		if err != nil {
			return err
		}
		lr.writers[path] = fw
	}
	return nil
}

func (lr *LogRotator) openWriter(path string) (*fileWriter, error) {
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create log dir failed: %w", err)
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file failed: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("stat log file failed: %w", err)
	}

	date := lr.currentDate()

	return &fileWriter{
		path: path,
		file: f,
		size: info.Size(),
		date: date,
	}, nil
}

func (lr *LogRotator) currentDate() string {
	now := lr.clock()
	switch lr.config.RotationMode {
	case RotationModeHourly:
		return now.Format("2006-01-02-15")
	case RotationModeDaily:
		return now.Format(lr.config.FileDateFormat)
	default:
		return now.Format(lr.config.FileDateFormat)
	}
}

func (lr *LogRotator) needsTimeRotate(fw *fileWriter) bool {
	if lr.config.RotationMode == RotationModeHourly || lr.config.RotationMode == RotationModeDaily {
		current := lr.currentDate()
		return fw.date != current
	}
	return false
}

func (lr *LogRotator) Log(level Level, message string) error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.closed {
		return fmt.Errorf("logrotator is closed")
	}

	paths := lr.pathsForLevel(level)
	if len(paths) == 0 {
		return fmt.Errorf("no file configured for level %s", level)
	}

	line := fmt.Sprintf("[%s] %s %s\n", level, lr.clock().Format("2006-01-02 15:04:05.000"), message)
	data := []byte(line)

	for _, path := range paths {
		fw, exists := lr.writers[path]
		if !exists {
			continue
		}

		needTimeRotate := lr.needsTimeRotate(fw)

		if needTimeRotate {
			if err := lr.rotate(fw); err != nil {
				return err
			}
			fw = lr.writers[path]
		}

		n, err := fw.file.Write(data)
		if err != nil {
			return fmt.Errorf("write log failed: %w", err)
		}
		fw.size += int64(n)

		if lr.config.RotationMode == RotationModeSize &&
			lr.config.MaxFileSize > 0 &&
			fw.size >= lr.config.MaxFileSize {
			if err := lr.rotate(fw); err != nil {
				return err
			}
		}
	}

	return nil
}

func (lr *LogRotator) pathsForLevel(level Level) []string {
	seen := make(map[string]bool)
	var paths []string

	for lvl, path := range lr.config.LevelFileMap {
		if level >= lvl {
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}

	return paths
}

func (lr *LogRotator) rotate(fw *fileWriter) error {
	if err := fw.file.Close(); err != nil {
		return fmt.Errorf("close old log file failed: %w", err)
	}

	var backupPath string
	if lr.config.RotationMode == RotationModeSize {
		backupPath = lr.rotateBySize(fw.path)
	} else {
		backupPath = lr.rotateByTime(fw.path, fw.date)
	}

	newFw, err := lr.openWriter(fw.path)
	if err != nil {
		return err
	}

	lr.writers[fw.path] = newFw

	if lr.config.Compress && backupPath != "" {
		lr.wg.Add(1)
		go func(src string, targetPath string) {
			defer lr.wg.Done()

			if err := compressAndRemove(src); err != nil {
				return
			}
			lr.cleanOldBackups(targetPath)
		}(backupPath, fw.path)
	} else if !lr.config.Compress {
		lr.cleanOldBackups(fw.path)
	}

	return nil
}

func (lr *LogRotator) rotateBySize(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	var index int
	for {
		index++
		candidate := fmt.Sprintf("%s.%d%s", base, index, ext)
		candidateGz := candidate + ".gz"
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			if _, errGz := os.Stat(candidateGz); os.IsNotExist(errGz) {
				if err := os.Rename(path, candidate); err != nil {
					return ""
				}
				return candidate
			}
		}
	}
}

func (lr *LogRotator) rotateByTime(path, oldDate string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	candidate := fmt.Sprintf("%s.%s%s", base, oldDate, ext)

	if _, err := os.Stat(candidate); err == nil {
		candidateGz := candidate + ".gz"
		if _, errGz := os.Stat(candidateGz); errGz == nil {
			now := lr.clock()
			candidate = fmt.Sprintf("%s.%s-%d%s", base, oldDate, now.UnixNano(), ext)
		}
	}

	if err := os.Rename(path, candidate); err != nil {
		return ""
	}
	return candidate
}

func (lr *LogRotator) cleanOldBackups(path string) {
	if lr.config.MaxBackups <= 0 {
		return
	}

	dir := filepath.Dir(path)
	baseName := filepath.Base(path)
	ext := filepath.Ext(baseName)
	prefix := strings.TrimSuffix(baseName, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		if lr.config.Compress {
			if !strings.HasSuffix(name, ".gz") {
				continue
			}
		} else {
			if strings.HasSuffix(name, ".gz") {
				continue
			}
		}

		origName := name
		if strings.HasSuffix(name, ".gz") {
			origName = strings.TrimSuffix(name, ".gz")
		}
		if !strings.HasPrefix(origName, prefix+".") {
			continue
		}
		if origName == prefix+ext {
			continue
		}
		backups = append(backups, filepath.Join(dir, name))
	}

	if len(backups) <= lr.config.MaxBackups {
		return
	}

	sort.Slice(backups, func(i, j int) bool {
		fi, err1 := os.Stat(backups[i])
		fj, err2 := os.Stat(backups[j])
		if err1 != nil || err2 != nil {
			return backups[i] < backups[j]
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	excess := len(backups) - lr.config.MaxBackups
	for i := 0; i < excess; i++ {
		_ = os.Remove(backups[i])
	}
}

func compressAndRemove(src string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}

	dstPath := src + ".gz"
	dstFile, err := os.Create(dstPath)
	if err != nil {
		srcFile.Close()
		return err
	}

	gzWriter := gzip.NewWriter(dstFile)

	_, copyErr := io.Copy(gzWriter, srcFile)
	gzCloseErr := gzWriter.Close()
	dstCloseErr := dstFile.Close()
	srcCloseErr := srcFile.Close()

	if copyErr != nil {
		os.Remove(dstPath)
		return copyErr
	}
	if gzCloseErr != nil {
		os.Remove(dstPath)
		return gzCloseErr
	}
	if dstCloseErr != nil {
		os.Remove(dstPath)
		return dstCloseErr
	}
	if srcCloseErr != nil {
		return srcCloseErr
	}

	if err := os.Remove(src); err != nil {
		return err
	}

	return nil
}

func (lr *LogRotator) startCleaner() {
	if lr.config.TTL <= 0 {
		return
	}
	lr.cleanOnce.Do(func() {
		lr.wg.Add(1)
		go lr.cleanLoop()
	})
}

func (lr *LogRotator) cleanLoop() {
	defer lr.wg.Done()

	ticker := time.NewTicker(lr.config.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-lr.ctx.Done():
			return
		case <-ticker.C:
			lr.cleanExpired()
		}
	}
}

func (lr *LogRotator) cleanExpired() {
	lr.mu.Lock()
	uniquePaths := make(map[string]bool)
	for _, p := range lr.config.LevelFileMap {
		uniquePaths[p] = true
	}
	lr.mu.Unlock()

	cutoff := lr.clock().Add(-lr.config.TTL)

	for path := range uniquePaths {
		lr.cleanPathExpired(path, cutoff)
	}
}

func (lr *LogRotator) cleanPathExpired(path string, cutoff time.Time) {
	dir := filepath.Dir(path)
	baseName := filepath.Base(path)
	ext := filepath.Ext(baseName)
	prefix := strings.TrimSuffix(baseName, ext)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()

		origName := name
		if strings.HasSuffix(name, ".gz") {
			origName = strings.TrimSuffix(name, ".gz")
		}

		if !strings.HasPrefix(origName, prefix+".") {
			continue
		}

		fullPath := filepath.Join(dir, name)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			_ = os.Remove(fullPath)
		}
	}
}

func (lr *LogRotator) Close() error {
	lr.mu.Lock()
	if lr.closed {
		lr.mu.Unlock()
		return nil
	}
	lr.closed = true
	lr.cancel()

	var firstErr error
	for _, fw := range lr.writers {
		if err := fw.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	lr.mu.Unlock()

	lr.wg.Wait()

	return firstErr
}

func (lr *LogRotator) Sync() error {
	lr.mu.Lock()
	defer lr.mu.Unlock()

	if lr.closed {
		return nil
	}

	var firstErr error
	for _, fw := range lr.writers {
		if err := fw.file.Sync(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (lr *LogRotator) CleanExpiredNow() {
	lr.cleanExpired()
}
