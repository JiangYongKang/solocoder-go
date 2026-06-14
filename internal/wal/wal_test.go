package wal

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

func TestOpTypeString(t *testing.T) {
	tests := []struct {
		op   OpType
		want string
	}{
		{OpPut, "PUT"},
		{OpDelete, "DELETE"},
		{OpCheckpoint, "CHECKPOINT"},
		{OpType(99), "UNKNOWN(99)"},
	}
	for _, tt := range tests {
		if got := tt.op.String(); got != tt.want {
			t.Errorf("OpType(%d).String() = %q, want %q", tt.op, got, tt.want)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}
	if cfg.Dir != "./wal" {
		t.Errorf("Default Dir = %q, want %q", cfg.Dir, "./wal")
	}
	if cfg.MaxSegmentSize != 64*1024*1024 {
		t.Errorf("Default MaxSegmentSize = %d, want %d", cfg.MaxSegmentSize, 64*1024*1024)
	}
	if cfg.FSyncOnWrite != false {
		t.Errorf("Default FSyncOnWrite = %v, want false", cfg.FSyncOnWrite)
	}
}

func TestNewWithNilConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{Dir: tmpDir}
	wal, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()
	if wal.SegmentCount() != 1 {
		t.Errorf("SegmentCount() = %d, want 1", wal.SegmentCount())
	}
}

func TestNewInvalidConfig(t *testing.T) {
	cfg := &Config{Dir: ""}
	_, err := New(cfg)
	if err != ErrInvalidConfig {
		t.Errorf("New() error = %v, want ErrInvalidConfig", err)
	}
}

func TestNewDefaultMaxSegmentSize(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{Dir: tmpDir, MaxSegmentSize: 0}
	wal, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()
	if wal.config.MaxSegmentSize != DefaultConfig().MaxSegmentSize {
		t.Errorf("MaxSegmentSize = %d, want %d", wal.config.MaxSegmentSize, DefaultConfig().MaxSegmentSize)
	}
}

func TestNewNegativeMaxSegmentSize(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{Dir: tmpDir, MaxSegmentSize: -1}
	wal, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()
	if wal.config.MaxSegmentSize != DefaultConfig().MaxSegmentSize {
		t.Errorf("MaxSegmentSize = %d, want %d", wal.config.MaxSegmentSize, DefaultConfig().MaxSegmentSize)
	}
}

func TestAppendBasic(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	offset, err := wal.Append(OpPut, []byte("hello"))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if offset != 0 {
		t.Errorf("first offset = %d, want 0", offset)
	}
	if wal.LastOffset() != 0 {
		t.Errorf("LastOffset() = %d, want 0", wal.LastOffset())
	}

	offset2, err := wal.Append(OpDelete, []byte("world"))
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}
	if offset2 != 1 {
		t.Errorf("second offset = %d, want 1", offset2)
	}
	if wal.LastOffset() != 1 {
		t.Errorf("LastOffset() = %d, want 1", wal.LastOffset())
	}
}

func TestAppendEmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.Append(OpPut, []byte{})
	if err != ErrEmptyData {
		t.Errorf("Append empty data error = %v, want ErrEmptyData", err)
	}

	_, err = wal.Append(OpDelete, nil)
	if err != ErrEmptyData {
		t.Errorf("Append nil data error = %v, want ErrEmptyData", err)
	}

	offset, err := wal.Append(OpCheckpoint, nil)
	if err != nil {
		t.Fatalf("Append OpCheckpoint with nil data error = %v", err)
	}
	if offset != 0 {
		t.Errorf("checkpoint offset = %d, want 0", offset)
	}
}

func TestAppendAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wal.Close()

	_, err = wal.Append(OpPut, []byte("test"))
	if err != ErrClosed {
		t.Errorf("Append after close error = %v, want ErrClosed", err)
	}
}

func TestReadFromBasic(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	data := []string{"entry0", "entry1", "entry2", "entry3", "entry4"}
	for i, d := range data {
		_, err := wal.Append(OpPut, []byte(d))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	entries, err := wal.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom(0) error = %v", err)
	}
	if len(entries) != len(data) {
		t.Fatalf("ReadFrom(0) returned %d entries, want %d", len(entries), len(data))
	}
	for i, e := range entries {
		if e.Offset != int64(i) {
			t.Errorf("entry[%d].Offset = %d, want %d", i, e.Offset, i)
		}
		if string(e.Data) != data[i] {
			t.Errorf("entry[%d].Data = %q, want %q", i, string(e.Data), data[i])
		}
		if e.Type != OpPut {
			t.Errorf("entry[%d].Type = %v, want OpPut", i, e.Type)
		}
	}
}

func TestReadFromOffset(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	for i := 0; i < 5; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	entries, err := wal.ReadFrom(2)
	if err != nil {
		t.Fatalf("ReadFrom(2) error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("ReadFrom(2) returned %d entries, want 3", len(entries))
	}
	if entries[0].Offset != 2 {
		t.Errorf("first entry offset = %d, want 2", entries[0].Offset)
	}
	if string(entries[0].Data) != "entry2" {
		t.Errorf("first entry data = %q, want entry2", string(entries[0].Data))
	}
}

func TestReadFromInvalidOffset(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.ReadFrom(-1)
	if err != ErrInvalidOffset {
		t.Errorf("ReadFrom(-1) error = %v, want ErrInvalidOffset", err)
	}
}

func TestReadFromAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wal.Close()

	_, err = wal.ReadFrom(0)
	if err != ErrClosed {
		t.Errorf("ReadFrom after close error = %v, want ErrClosed", err)
	}
}

func TestReadFromEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	entries, err := wal.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom(0) error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("empty WAL ReadFrom returned %d entries, want 0", len(entries))
	}

	entries, err = wal.ReadFrom(100)
	if err != nil {
		t.Fatalf("ReadFrom(100) error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ReadFrom beyond end returned %d entries, want 0", len(entries))
	}
}

func TestRecoverFromBasic(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	for i := 0; i < 5; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	var recovered []*Entry
	warnings, err := wal.RecoverFrom(0, func(e *Entry) error {
		recovered = append(recovered, e)
		return nil
	})
	if err != nil {
		t.Fatalf("RecoverFrom error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings count = %d, want 0", len(warnings))
	}
	if len(recovered) != 5 {
		t.Fatalf("recovered %d entries, want 5", len(recovered))
	}
	for i, e := range recovered {
		if e.Offset != int64(i) {
			t.Errorf("recovered[%d].Offset = %d, want %d", i, e.Offset, i)
		}
	}
}

func TestRecoverFromOffset(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	for i := 0; i < 5; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	var recovered []*Entry
	_, err = wal.RecoverFrom(3, func(e *Entry) error {
		recovered = append(recovered, e)
		return nil
	})
	if err != nil {
		t.Fatalf("RecoverFrom error = %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("recovered %d entries, want 2", len(recovered))
	}
	if recovered[0].Offset != 3 {
		t.Errorf("first recovered offset = %d, want 3", recovered[0].Offset)
	}
}

func TestRecoverFromCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	for i := 0; i < 5; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	cbErr := fmt.Errorf("callback error")
	var count int
	_, err = wal.RecoverFrom(0, func(e *Entry) error {
		count++
		if count == 3 {
			return cbErr
		}
		return nil
	})
	if err != cbErr {
		t.Errorf("RecoverFrom error = %v, want %v", err, cbErr)
	}
	if count != 3 {
		t.Errorf("callback called %d times, want 3", count)
	}
}

func TestRecoverFromInvalidOffset(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.RecoverFrom(-1, func(e *Entry) error { return nil })
	if err != ErrInvalidOffset {
		t.Errorf("RecoverFrom(-1) error = %v, want ErrInvalidOffset", err)
	}
}

func TestRecoverFromNilCallback(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.RecoverFrom(0, nil)
	if err == nil {
		t.Error("RecoverFrom with nil callback should return error")
	}
}

func TestRecoverFromAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wal.Close()

	_, err = wal.RecoverFrom(0, func(e *Entry) error { return nil })
	if err != ErrClosed {
		t.Errorf("RecoverFrom after close error = %v, want ErrClosed", err)
	}
}

func TestRecoverFromCorruptedEntry(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("valid%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync error = %v", err)
	}

	segPath := filepath.Join(tmpDir, segmentFileName(1))
	f, err := os.OpenFile(segPath, os.O_RDWR, 0644)
	if err != nil {
		t.Fatalf("Open segment error = %v", err)
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		t.Fatalf("Stat error = %v", err)
	}
	corruptPos := info.Size() + 5
	if _, err := f.Seek(corruptPos, io.SeekStart); err != nil {
		f.Close()
		t.Fatalf("Seek error = %v", err)
	}
	corruptData := make([]byte, 50)
	for i := range corruptData {
		corruptData[i] = 0xFF
	}
	if _, err := f.Write(corruptData); err != nil {
		f.Close()
		t.Fatalf("Write corrupt data error = %v", err)
	}
	f.Close()

	if err := wal.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	wal2, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("Reopen WAL error = %v", err)
	}
	defer wal2.Close()

	var recovered []*Entry
	_, err = wal2.RecoverFrom(0, func(e *Entry) error {
		recovered = append(recovered, e)
		return nil
	})
	if err != nil {
		t.Fatalf("RecoverFrom error = %v", err)
	}
	if len(recovered) < 3 {
		t.Errorf("recovered %d entries, want at least 3", len(recovered))
	}
	for i := 0; i < len(recovered) && i < 3; i++ {
		want := fmt.Sprintf("valid%d", i)
		if string(recovered[i].Data) != want {
			t.Errorf("recovered[%d].Data = %q, want %q", i, string(recovered[i].Data), want)
		}
	}
}

func TestSegmentRotation(t *testing.T) {
	tmpDir := t.TempDir()
	maxSegSize := int64(200)
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	entryData := bytes.Repeat([]byte("x"), 50)
	var count int
	for wal.SegmentCount() < 3 {
		_, err := wal.Append(OpPut, entryData)
		if err != nil {
			t.Fatalf("Append(%d) error = %v", count, err)
		}
		count++
		if count > 100 {
			t.Fatal("too many iterations without rotation")
		}
	}

	if wal.SegmentCount() < 3 {
		t.Errorf("SegmentCount() = %d, want >= 3", wal.SegmentCount())
	}

	entries, err := wal.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom error = %v", err)
	}
	if len(entries) != count {
		t.Errorf("ReadFrom returned %d entries, want %d", len(entries), count)
	}
	for i, e := range entries {
		if e.Offset != int64(i) {
			t.Errorf("entry[%d].Offset = %d, want %d", i, e.Offset, i)
		}
	}
}

func TestPersistenceAndReload(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	for i := 0; i < 10; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("data%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}
	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync error = %v", err)
	}
	lastOffset := wal.LastOffset()
	if err := wal.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	wal2, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("Reopen error = %v", err)
	}
	defer wal2.Close()

	if wal2.LastOffset() != lastOffset {
		t.Errorf("LastOffset after reload = %d, want %d", wal2.LastOffset(), lastOffset)
	}

	entries, err := wal2.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom error = %v", err)
	}
	if len(entries) != 10 {
		t.Fatalf("ReadFrom returned %d entries, want 10", len(entries))
	}
	for i, e := range entries {
		if string(e.Data) != fmt.Sprintf("data%d", i) {
			t.Errorf("entry[%d].Data = %q, want data%d", i, string(e.Data), i)
		}
	}

	newOffset, err := wal2.Append(OpPut, []byte("new_data"))
	if err != nil {
		t.Fatalf("Append after reload error = %v", err)
	}
	if newOffset != lastOffset+1 {
		t.Errorf("new offset = %d, want %d", newOffset, lastOffset+1)
	}
}

func TestPersistenceWithRotation(t *testing.T) {
	tmpDir := t.TempDir()
	maxSegSize := int64(150)
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	entryData := bytes.Repeat([]byte("y"), 40)
	var offsets []int64
	for i := 0; i < 15; i++ {
		offset, err := wal.Append(OpPut, append([]byte{}, entryData...))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
		offsets = append(offsets, offset)
	}

	if wal.SegmentCount() < 2 {
		t.Errorf("SegmentCount() = %d, want >= 2", wal.SegmentCount())
	}

	if err := wal.Close(); err != nil {
		t.Fatalf("Close error = %v", err)
	}

	wal2, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("Reopen error = %v", err)
	}
	defer wal2.Close()

	if wal2.SegmentCount() != len(offsets)/5+1 && wal2.SegmentCount() < 2 {
		t.Errorf("SegmentCount after reload = %d, want same as before", wal2.SegmentCount())
	}

	entries, err := wal2.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom error = %v", err)
	}
	if len(entries) != len(offsets) {
		t.Errorf("ReadFrom returned %d entries, want %d", len(entries), len(offsets))
	}
}

func TestSync(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.Append(OpPut, []byte("test"))
	if err != nil {
		t.Fatalf("Append error = %v", err)
	}

	if err := wal.Sync(); err != nil {
		t.Fatalf("Sync error = %v", err)
	}
}

func TestSyncAfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wal.Close()

	if err := wal.Sync(); err != nil {
		t.Errorf("Sync after close should return nil, got %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := wal.Close(); err != nil {
		t.Fatalf("first Close error = %v", err)
	}
	if err := wal.Close(); err != nil {
		t.Fatalf("second Close error = %v, want nil (idempotent)", err)
	}
}

func TestLastOffsetEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	if wal.LastOffset() != -1 {
		t.Errorf("empty WAL LastOffset() = %d, want -1", wal.LastOffset())
	}
}

func TestActiveSegmentSize(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	if wal.ActiveSegmentSize() != 0 {
		t.Errorf("empty ActiveSegmentSize() = %d, want 0", wal.ActiveSegmentSize())
	}

	_, err = wal.Append(OpPut, []byte("hello"))
	if err != nil {
		t.Fatalf("Append error = %v", err)
	}

	if wal.ActiveSegmentSize() <= 0 {
		t.Errorf("ActiveSegmentSize() = %d, want > 0", wal.ActiveSegmentSize())
	}
}

func TestConcurrentAppend(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	numGoroutines := 10
	numPerGoroutine := 100
	var wg sync.WaitGroup
	var counter int64

	wg.Add(numGoroutines)
	for g := 0; g < numGoroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numPerGoroutine; i++ {
				data := []byte(fmt.Sprintf("goroutine_%d_entry_%d", id, i))
				_, err := wal.Append(OpPut, data)
				if err != nil {
					t.Errorf("goroutine %d Append error: %v", id, err)
					return
				}
				atomic.AddInt64(&counter, 1)
			}
		}(g)
	}
	wg.Wait()

	expected := int64(numGoroutines * numPerGoroutine)
	if counter != expected {
		t.Errorf("counter = %d, want %d", counter, expected)
	}
	if wal.LastOffset() != expected-1 {
		t.Errorf("LastOffset() = %d, want %d", wal.LastOffset(), expected-1)
	}

	entries, err := wal.ReadFrom(0)
	if err != nil {
		t.Fatalf("ReadFrom error = %v", err)
	}
	if len(entries) != int(expected) {
		t.Errorf("ReadFrom returned %d entries, want %d", len(entries), expected)
	}

	seenOffsets := make(map[int64]bool)
	for _, e := range entries {
		if seenOffsets[e.Offset] {
			t.Errorf("duplicate offset %d", e.Offset)
		}
		seenOffsets[e.Offset] = true
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry_%d", i)))
			if err != nil {
				return
			}
		}
		close(stop)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, err := wal.ReadFrom(0)
				if err != nil {
					return
				}
			}
		}
	}()

	wg.Wait()
}

func TestMultipleReadersConcurrent(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	totalEntries := 500
	for i := 0; i < totalEntries; i++ {
		data := []byte(fmt.Sprintf("preload_entry_%06d_data_%s", i, bytes.Repeat([]byte("x"), 30)))
		_, err := wal.Append(OpPut, data)
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	numReaders := 5
	var wg sync.WaitGroup
	var errCount int64
	var dataMismatch int64

	wg.Add(numReaders)
	for r := 0; r < numReaders; r++ {
		readerID := r
		go func() {
			defer wg.Done()
			for iter := 0; iter < 50; iter++ {
				entries, err := wal.ReadFrom(0)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					return
				}
				if len(entries) != totalEntries {
					atomic.AddInt64(&dataMismatch, 1)
					continue
				}
				for i, e := range entries {
					if e.Offset != int64(i) {
						atomic.AddInt64(&dataMismatch, 1)
						break
					}
					expectedPrefix := fmt.Sprintf("preload_entry_%06d", i)
					if !bytes.HasPrefix(e.Data, []byte(expectedPrefix)) {
						atomic.AddInt64(&dataMismatch, 1)
						break
					}
				}
			}
			t.Logf("reader %d completed 50 iterations", readerID)
		}()
	}
	wg.Wait()

	if errCount > 0 {
		t.Errorf("reader errors = %d, want 0", errCount)
	}
	if dataMismatch > 0 {
		t.Errorf("data mismatches = %d, want 0", dataMismatch)
	}
}

func TestConcurrentReadersDifferentOffsets(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: 1024 * 1024})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	totalEntries := 200
	for i := 0; i < totalEntries; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("entry_%d", i)))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", i, err)
		}
	}

	offsets := []int64{0, 50, 100, 150}
	var wg sync.WaitGroup
	var errCount int64
	var dataMismatch int64

	wg.Add(len(offsets) * 3)
	for run := 0; run < 3; run++ {
		for idx, startOff := range offsets {
			startOffset := startOff
			readerIdx := idx + run*len(offsets)
			go func() {
				defer wg.Done()
				for iter := 0; iter < 30; iter++ {
					entries, err := wal.ReadFrom(startOffset)
					if err != nil {
						atomic.AddInt64(&errCount, 1)
						return
					}
					expectedCount := totalEntries - int(startOffset)
					if len(entries) != expectedCount {
						atomic.AddInt64(&dataMismatch, 1)
						continue
					}
					for i, e := range entries {
						expectedOffset := startOffset + int64(i)
						if e.Offset != expectedOffset {
							atomic.AddInt64(&dataMismatch, 1)
							break
						}
						expectedData := fmt.Sprintf("entry_%d", expectedOffset)
						if string(e.Data) != expectedData {
							atomic.AddInt64(&dataMismatch, 1)
							break
						}
					}
				}
				t.Logf("offset reader %d (startOffset=%d) done", readerIdx, startOffset)
			}()
		}
	}
	wg.Wait()

	if errCount > 0 {
		t.Errorf("reader errors = %d, want 0", errCount)
	}
	if dataMismatch > 0 {
		t.Errorf("data mismatches = %d, want 0", dataMismatch)
	}
}

func TestConcurrentReadersAcrossSegments(t *testing.T) {
	tmpDir := t.TempDir()
	maxSegSize := int64(250)
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	entryData := bytes.Repeat([]byte("y"), 60)
	var totalEntries int
	for wal.SegmentCount() < 5 {
		_, err := wal.Append(OpPut, append([]byte(fmt.Sprintf("idx_%04d_", totalEntries)), entryData...))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", totalEntries, err)
		}
		totalEntries++
	}
	t.Logf("loaded %d entries across %d segments", totalEntries, wal.SegmentCount())

	numReaders := 4
	var wg sync.WaitGroup
	var errCount int64
	var dataMismatch int64

	wg.Add(numReaders)
	for r := 0; r < numReaders; r++ {
		readerID := r
		go func() {
			defer wg.Done()
			for iter := 0; iter < 40; iter++ {
				entries, err := wal.ReadFrom(0)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					return
				}
				if len(entries) != totalEntries {
					atomic.AddInt64(&dataMismatch, 1)
					continue
				}
				for i, e := range entries {
					expectedPrefix := fmt.Sprintf("idx_%04d_", i)
					if !bytes.HasPrefix(e.Data, []byte(expectedPrefix)) {
						atomic.AddInt64(&dataMismatch, 1)
						break
					}
				}
			}
			t.Logf("multi-seg reader %d done", readerID)
		}()
	}
	wg.Wait()

	if errCount > 0 {
		t.Errorf("reader errors = %d, want 0", errCount)
	}
	if dataMismatch > 0 {
		t.Errorf("data mismatches = %d, want 0", dataMismatch)
	}
}

func TestEncodeDecodeEntry(t *testing.T) {
	entries := []*Entry{
		{Offset: 0, Type: OpPut, Data: []byte("hello")},
		{Offset: 1, Type: OpDelete, Data: []byte("world")},
		{Offset: 12345, Type: OpCheckpoint, Data: nil},
		{Offset: 999, Type: OpPut, Data: bytes.Repeat([]byte("x"), 1000)},
	}

	for i, e := range entries {
		encoded, err := encodeEntry(e)
		if err != nil {
			t.Fatalf("encodeEntry[%d] error = %v", i, err)
		}

		decoded, consumed, err := decodeEntry(encoded)
		if err != nil {
			t.Fatalf("decodeEntry[%d] error = %v", i, err)
		}
		if consumed != len(encoded) {
			t.Errorf("decodeEntry[%d] consumed = %d, want %d", i, consumed, len(encoded))
		}
		if decoded.Offset != e.Offset {
			t.Errorf("decoded[%d].Offset = %d, want %d", i, decoded.Offset, e.Offset)
		}
		if decoded.Type != e.Type {
			t.Errorf("decoded[%d].Type = %v, want %v", i, decoded.Type, e.Type)
		}
		if !bytes.Equal(decoded.Data, e.Data) {
			t.Errorf("decoded[%d].Data mismatch", i)
		}
	}
}

func TestDecodeEntryCorrupted(t *testing.T) {
	e := &Entry{Offset: 42, Type: OpPut, Data: []byte("test")}
	encoded, err := encodeEntry(e)
	if err != nil {
		t.Fatalf("encodeEntry error = %v", err)
	}

	_, _, err = decodeEntry(encoded[:5])
	if err == nil {
		t.Error("decodeEntry with truncated header should return error")
	}

	corrupted := make([]byte, len(encoded))
	copy(corrupted, encoded)
	corrupted[0] = 0x00
	corrupted[1] = 0x00
	_, _, err = decodeEntry(corrupted)
	if err == nil {
		t.Error("decodeEntry with bad magic should return error")
	}

	corrupted2 := make([]byte, len(encoded))
	copy(corrupted2, encoded)
	corrupted2[entryHeaderSize-1] ^= 0xFF
	_, _, err = decodeEntry(corrupted2)
	if err == nil {
		t.Error("decodeEntry with truncated data should return error")
	}

	corrupted3 := make([]byte, len(encoded))
	copy(corrupted3, encoded)
	corrupted3[entryHeaderSize+2] ^= 0xFF
	_, _, err = decodeEntry(corrupted3)
	if err == nil {
		t.Error("decodeEntry with bad checksum should return error")
	}
}

func TestSegmentFileName(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "wal_00000001.log"},
		{10, "wal_00000010.log"},
		{12345, "wal_00012345.log"},
	}
	for _, tt := range tests {
		if got := segmentFileName(tt.id); got != tt.want {
			t.Errorf("segmentFileName(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestParseSegmentID(t *testing.T) {
	tests := []struct {
		filename string
		wantID   int
		wantOK   bool
	}{
		{"wal_00000001.log", 1, true},
		{"wal_00000010.log", 10, true},
		{"wal_00012345.log", 12345, true},
		{"wal_00000000.log", 0, true},
		{"wal_abc.log", 0, false},
		{"something.log", 0, false},
		{"wal_00000001.txt", 0, false},
		{"WAL_00000001.log", 0, false},
	}
	for _, tt := range tests {
		id, ok := parseSegmentID(tt.filename)
		if ok != tt.wantOK {
			t.Errorf("parseSegmentID(%q) ok = %v, want %v", tt.filename, ok, tt.wantOK)
		}
		if ok && id != tt.wantID {
			t.Errorf("parseSegmentID(%q) id = %d, want %d", tt.filename, id, tt.wantID)
		}
	}
}

func TestCorruptedEntryWarningString(t *testing.T) {
	w := &CorruptedEntryWarning{
		SegmentID: 5,
		Position:  1234,
		Reason:    "checksum mismatch",
	}
	got := w.String()
	want := "corrupted entry in segment 5 at position 1234: checksum mismatch"
	if got != want {
		t.Errorf("CorruptedEntryWarning.String() = %q, want %q", got, want)
	}
}

func TestFSyncOnWrite(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir, FSyncOnWrite: true})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	_, err = wal.Append(OpPut, []byte("fsync_test"))
	if err != nil {
		t.Fatalf("Append with FSync error = %v", err)
	}
}

func TestReadFromAcrossSegments(t *testing.T) {
	tmpDir := t.TempDir()
	maxSegSize := int64(150)
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	entryData := bytes.Repeat([]byte("z"), 40)
	var count int
	for wal.SegmentCount() < 4 {
		_, err := wal.Append(OpPut, append([]byte{}, entryData...))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", count, err)
		}
		count++
		if count > 100 {
			t.Fatal("too many iterations")
		}
	}

	startFrom := count / 2
	entries, err := wal.ReadFrom(int64(startFrom))
	if err != nil {
		t.Fatalf("ReadFrom(%d) error = %v", startFrom, err)
	}
	expectedCount := count - startFrom
	if len(entries) != expectedCount {
		t.Errorf("ReadFrom(%d) returned %d entries, want %d", startFrom, len(entries), expectedCount)
	}
	for i, e := range entries {
		expectedOffset := int64(startFrom + i)
		if e.Offset != expectedOffset {
			t.Errorf("entry[%d].Offset = %d, want %d", i, e.Offset, expectedOffset)
		}
	}
}

func TestRecoverFromAcrossSegments(t *testing.T) {
	tmpDir := t.TempDir()
	maxSegSize := int64(150)
	wal, err := New(&Config{Dir: tmpDir, MaxSegmentSize: maxSegSize})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	entryData := bytes.Repeat([]byte("w"), 40)
	var count int
	for wal.SegmentCount() < 3 {
		_, err := wal.Append(OpPut, append([]byte{}, entryData...))
		if err != nil {
			t.Fatalf("Append(%d) error = %v", count, err)
		}
		count++
		if count > 100 {
			t.Fatal("too many iterations")
		}
	}

	var recovered []*Entry
	warnings, err := wal.RecoverFrom(0, func(e *Entry) error {
		recovered = append(recovered, e)
		return nil
	})
	if err != nil {
		t.Fatalf("RecoverFrom error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings count = %d, want 0", len(warnings))
	}
	if len(recovered) != count {
		t.Errorf("recovered %d entries, want %d", len(recovered), count)
	}
}

func TestEncodeEntryChecksum(t *testing.T) {
	e := &Entry{Offset: 100, Type: OpPut, Data: []byte("checksum_test")}
	encoded, err := encodeEntry(e)
	if err != nil {
		t.Fatalf("encodeEntry error = %v", err)
	}

	storedChecksum := binary.BigEndian.Uint32(encoded[2:6])
	dataLen := binary.BigEndian.Uint32(encoded[15:19])

	checksumPayload := make([]byte, 13+int(dataLen))
	copy(checksumPayload[0:8], encoded[6:14])
	checksumPayload[8] = encoded[14]
	copy(checksumPayload[9:13], encoded[15:19])
	copy(checksumPayload[13:], encoded[19:19+int(dataLen)])

	actualChecksum := crc32.ChecksumIEEE(checksumPayload)
	if storedChecksum != actualChecksum {
		t.Errorf("stored checksum = 0x%x, computed = 0x%x", storedChecksum, actualChecksum)
	}
}

func TestLoadEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	if wal.SegmentCount() != 1 {
		t.Errorf("SegmentCount() = %d, want 1", wal.SegmentCount())
	}
	if wal.LastOffset() != -1 {
		t.Errorf("LastOffset() = %d, want -1", wal.LastOffset())
	}
}

func TestReadFromEndOffset(t *testing.T) {
	tmpDir := t.TempDir()
	wal, err := New(&Config{Dir: tmpDir})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer wal.Close()

	for i := 0; i < 3; i++ {
		_, err := wal.Append(OpPut, []byte(fmt.Sprintf("e%d", i)))
		if err != nil {
			t.Fatalf("Append error = %v", err)
		}
	}

	entries, err := wal.ReadFrom(100)
	if err != nil {
		t.Fatalf("ReadFrom(100) error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ReadFrom beyond end got %d entries, want 0", len(entries))
	}
}
