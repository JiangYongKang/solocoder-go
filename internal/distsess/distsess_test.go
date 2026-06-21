package distsess

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.DefaultTTL != DefaultTTL {
		t.Errorf("expected DefaultTTL %v, got %v", DefaultTTL, cfg.DefaultTTL)
	}
	if cfg.CleanupInterval != DefaultCleanupInterval {
		t.Errorf("expected CleanupInterval %v, got %v", DefaultCleanupInterval, cfg.CleanupInterval)
	}
	if cfg.AutoRenew != DefaultAutoRenew {
		t.Errorf("expected AutoRenew %v, got %v", DefaultAutoRenew, cfg.AutoRenew)
	}
	if cfg.SyncBuffer != DefaultSyncBuffer {
		t.Errorf("expected SyncBuffer %d, got %d", DefaultSyncBuffer, cfg.SyncBuffer)
	}
	if cfg.EnableSync != true {
		t.Errorf("expected EnableSync true, got %v", cfg.EnableSync)
	}
}

func TestSession_IsExpired(t *testing.T) {
	now := time.Now()

	s := &Session{
		TTL:       1 * time.Hour,
		ExpiresAt: now.Add(1 * time.Hour),
	}
	if s.IsExpired() {
		t.Error("session should not be expired")
	}

	s2 := &Session{
		TTL:       1 * time.Hour,
		ExpiresAt: now.Add(-1 * time.Hour),
	}
	if !s2.IsExpired() {
		t.Error("session should be expired")
	}

	s3 := &Session{
		TTL: 0,
	}
	if s3.IsExpired() {
		t.Error("session with TTL=0 should never expire")
	}
}

func TestSession_Renew(t *testing.T) {
	now := time.Now()
	ttl := 1 * time.Hour
	s := &Session{
		ID:        "test",
		Data:      SessionData{"key": "value"},
		TTL:       ttl,
		ExpiresAt: now.Add(-1 * time.Hour),
		Version:   1,
	}

	s.Renew()

	if !s.ExpiresAt.After(now) {
		t.Error("ExpiresAt should be renewed to future")
	}
	if s.Version != 2 {
		t.Errorf("expected Version 2, got %d", s.Version)
	}
	if s.UpdatedAt.Before(now) {
		t.Error("UpdatedAt should be updated")
	}

	s2 := &Session{TTL: 0, Version: 5}
	oldExpiresAt := s2.ExpiresAt
	s2.Renew()
	if s2.Version != 5 {
		t.Errorf("expected Version unchanged for TTL=0, got %d", s2.Version)
	}
	if s2.ExpiresAt != oldExpiresAt {
		t.Error("ExpiresAt should not change for TTL=0")
	}
}

func TestSession_DeepCopy(t *testing.T) {
	original := &Session{
		ID:        "test",
		Data:      SessionData{"key": "value", "num": 42},
		TTL:       1 * time.Hour,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   3,
		NodeID:    "node1",
	}

	copy := original.DeepCopy()

	if copy.ID != original.ID {
		t.Error("ID should match")
	}
	if copy.TTL != original.TTL {
		t.Error("TTL should match")
	}
	if copy.Version != original.Version {
		t.Error("Version should match")
	}
	if copy.NodeID != original.NodeID {
		t.Error("NodeID should match")
	}
	if copy.Data["key"] != original.Data["key"] {
		t.Error("Data should match")
	}

	copy.Data["key"] = "modified"
	if original.Data["key"] == "modified" {
		t.Error("modifying copy should not affect original")
	}
}

func TestMemoryPersistenceStore(t *testing.T) {
	mps := NewMemoryPersistenceStore()

	session := &Session{
		ID:        "sess1",
		Data:      SessionData{"user": "alice"},
		TTL:       1 * time.Hour,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := mps.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := mps.Load("sess1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != "sess1" {
		t.Errorf("expected ID sess1, got %s", loaded.ID)
	}
	if loaded.Data["user"] != "alice" {
		t.Errorf("expected user alice, got %v", loaded.Data["user"])
	}

	count, err := mps.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	all, err := mps.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 session, got %d", len(all))
	}

	loaded.Data["user"] = "bob"
	verify, _ := mps.Load("sess1")
	if verify.Data["user"] != "alice" {
		t.Error("modifying loaded session should not affect stored data")
	}

	if _, err := mps.Load("nonexistent"); err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}

	err = mps.Delete("sess1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	count, _ = mps.Count()
	if count != 0 {
		t.Errorf("expected count 0 after delete, got %d", count)
	}

	if err := mps.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if err := mps.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestMemoryPersistenceStore_EdgeCases(t *testing.T) {
	mps := NewMemoryPersistenceStore()

	if err := mps.Save(nil); err != ErrNilSessionData {
		t.Errorf("expected ErrNilSessionData, got %v", err)
	}

	if err := mps.Save(&Session{}); err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID, got %v", err)
	}

	if _, err := mps.Load(""); err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID for Load, got %v", err)
	}

	if err := mps.Delete(""); err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID for Delete, got %v", err)
	}

	if err := mps.Delete("nonexistent"); err != nil {
		t.Errorf("Delete nonexistent should not error, got %v", err)
	}
}

func TestFilePersistenceStore(t *testing.T) {
	tmpDir := t.TempDir()

	fps, err := NewFilePersistenceStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFilePersistenceStore failed: %v", err)
	}

	session := &Session{
		ID:        "sess1",
		Data:      SessionData{"user": "alice", "age": 30},
		TTL:       1 * time.Hour,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   1,
		NodeID:    "node1",
	}

	if err := fps.Save(session); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := fps.Load("sess1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.ID != "sess1" {
		t.Errorf("expected ID sess1, got %s", loaded.ID)
	}
	if loaded.Data["user"] != "alice" {
		t.Errorf("expected user alice, got %v", loaded.Data["user"])
	}
	if loaded.Version != 1 {
		t.Errorf("expected Version 1, got %d", loaded.Version)
	}

	session.Data["age"] = 31
	session.Version = 2
	if err := fps.Save(session); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	loaded, _ = fps.Load("sess1")
	if loaded.Data["age"] != float64(31) {
		t.Errorf("expected age 31, got %v", loaded.Data["age"])
	}

	count, err := fps.Count()
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}

	if err := fps.Delete("sess1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if _, err := fps.Load("sess1"); err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound after delete, got %v", err)
	}

	if err := fps.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if err := fps.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestFilePersistenceStore_InvalidDir(t *testing.T) {
	_, err := NewFilePersistenceStore("")
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestNewTieredStore(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, err := NewTieredStore(cfg, mps)
	if err != nil {
		t.Fatalf("NewTieredStore failed: %v", err)
	}
	if ts == nil {
		t.Fatal("NewTieredStore returned nil")
	}

	if ts.GetMemoryCount() != 0 {
		t.Errorf("expected memory count 0, got %d", ts.GetMemoryCount())
	}
}

func TestNewTieredStore_NilPersistence(t *testing.T) {
	cfg := DefaultConfig()
	_, err := NewTieredStore(cfg, nil)
	if err == nil {
		t.Error("expected error for nil persistence")
	}
}

func TestTieredStore_SetAndGet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = false
	mps := NewMemoryPersistenceStore()

	ts, err := NewTieredStore(cfg, mps)
	if err != nil {
		t.Fatalf("NewTieredStore failed: %v", err)
	}

	data := SessionData{"user": "alice", "role": "admin"}
	session, err := ts.SetWithTTL("sess1", data, 1*time.Hour)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}
	if session.ID != "sess1" {
		t.Errorf("expected ID sess1, got %s", session.ID)
	}
	if session.Version != 1 {
		t.Errorf("expected Version 1, got %d", session.Version)
	}

	loaded, err := ts.Get("sess1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if loaded.Data["user"] != "alice" {
		t.Errorf("expected user alice, got %v", loaded.Data["user"])
	}

	persisted, _ := mps.Load("sess1")
	if persisted == nil {
		t.Error("session should be persisted")
	}
	if persisted.Data["user"] != "alice" {
		t.Errorf("persisted data mismatch: %v", persisted.Data)
	}

	data2 := SessionData{"user": "bob"}
	updated, err := ts.SetWithTTL("sess1", data2, 1*time.Hour)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected Version 2, got %d", updated.Version)
	}

	loaded, _ = ts.Get("sess1")
	if loaded.Data["user"] != "bob" {
		t.Errorf("expected updated user bob, got %v", loaded.Data["user"])
	}

	if ts.GetMemoryCount() != 1 {
		t.Errorf("expected memory count 1, got %d", ts.GetMemoryCount())
	}

	persistedCount, _ := ts.GetPersistedCount()
	if persistedCount != 1 {
		t.Errorf("expected persisted count 1, got %d", persistedCount)
	}
}

func TestTieredStore_GetWithAutoRenew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = true
	mps := NewMemoryPersistenceStore()

	ts, err := NewTieredStore(cfg, mps)
	if err != nil {
		t.Fatalf("NewTieredStore failed: %v", err)
	}

	ttl := 1 * time.Hour
	data := SessionData{"user": "alice"}
	_, err = ts.SetWithTTL("sess1", data, ttl)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	initial, _ := ts.GetWithoutRenew("sess1")
	initialExpiresAt := initial.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	loaded, err := ts.Get("sess1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if !loaded.ExpiresAt.After(initialExpiresAt) {
		t.Error("ExpiresAt should be renewed")
	}
	if loaded.Version != 2 {
		t.Errorf("expected Version 2 after renew, got %d", loaded.Version)
	}
}

func TestTieredStore_GetNotFound(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	_, err := ts.Get("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestTieredStore_GetExpired(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = false
	mps := NewMemoryPersistenceStore()

	ts, err := NewTieredStore(cfg, mps)
	if err != nil {
		t.Fatalf("NewTieredStore failed: %v", err)
	}

	shortTTL := 10 * time.Millisecond
	_, err = ts.SetWithTTL("sess1", SessionData{"k": "v"}, shortTTL)
	if err != nil {
		t.Fatalf("SetWithTTL failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	_, err = ts.Get("sess1")
	if err != ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}

	if ts.Exists("sess1") {
		t.Error("expired session should be removed")
	}

	persisted, _ := mps.Load("sess1")
	if persisted != nil {
		t.Error("expired session should be removed from persistence")
	}
}

func TestTieredStore_Delete(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	_, _ = ts.SetWithTTL("sess1", SessionData{"k": "v"}, 1*time.Hour)

	existed, err := ts.Delete("sess1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !existed {
		t.Error("existed should be true")
	}

	if ts.Exists("sess1") {
		t.Error("session should not exist after delete")
	}

	persisted, _ := mps.Load("sess1")
	if persisted != nil {
		t.Error("session should be deleted from persistence")
	}

	existed, err = ts.Delete("sess1")
	if err != nil {
		t.Fatalf("Delete nonexistent failed: %v", err)
	}
	if existed {
		t.Error("existed should be false for nonexistent")
	}
}

func TestTieredStore_Renew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = false
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	shortTTL := 100 * time.Millisecond
	original, _ := ts.SetWithTTL("sess1", SessionData{"k": "v"}, shortTTL)
	originalExpiresAt := original.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	renewed, err := ts.Renew("sess1")
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}
	if !renewed.ExpiresAt.After(originalExpiresAt) {
		t.Error("ExpiresAt should be extended")
	}
	if renewed.Version != original.Version+1 {
		t.Errorf("expected Version %d, got %d", original.Version+1, renewed.Version)
	}

	time.Sleep(150 * time.Millisecond)
	_, err = ts.Renew("sess1")
	if err != ErrSessionExpired {
		t.Errorf("expected ErrSessionExpired, got %v", err)
	}
}

func TestTieredStore_CleanupExpired(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	shortTTL := 10 * time.Millisecond
	longTTL := 1 * time.Hour

	_, _ = ts.SetWithTTL("expired1", SessionData{}, shortTTL)
	_, _ = ts.SetWithTTL("expired2", SessionData{}, shortTTL)
	_, _ = ts.SetWithTTL("valid", SessionData{}, longTTL)

	time.Sleep(50 * time.Millisecond)

	cleaned := ts.CleanupExpired()
	if cleaned != 2 {
		t.Errorf("expected 2 expired sessions cleaned, got %d", cleaned)
	}

	if !ts.Exists("valid") {
		t.Error("valid session should still exist")
	}
	if ts.Exists("expired1") {
		t.Error("expired1 should be cleaned")
	}
	if ts.Exists("expired2") {
		t.Error("expired2 should be cleaned")
	}
}

func TestTieredStore_LoadFromPersistence(t *testing.T) {
	mps := NewMemoryPersistenceStore()

	existingSession := &Session{
		ID:        "existing",
		Data:      SessionData{"key": "value"},
		TTL:       1 * time.Hour,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Version:   5,
	}
	_ = mps.Save(existingSession)

	expiredSession := &Session{
		ID:        "expired",
		Data:      SessionData{},
		TTL:       1 * time.Millisecond,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_ = mps.Save(expiredSession)

	cfg := DefaultConfig()
	cfg.AutoRenew = false
	ts, err := NewTieredStore(cfg, mps)
	if err != nil {
		t.Fatalf("NewTieredStore failed: %v", err)
	}

	if !ts.Exists("existing") {
		t.Error("existing session should be loaded")
	}

	loaded, _ := ts.Get("existing")
	if loaded.Version != 5 {
		t.Errorf("expected Version 5, got %d", loaded.Version)
	}

	if ts.Exists("expired") {
		t.Error("expired session should not be loaded")
	}

	persistedExpired, _ := mps.Load("expired")
	if persistedExpired != nil {
		t.Error("expired session should be deleted from persistence")
	}
}

func TestTieredStore_GetAll(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = ts.SetWithTTL(id, SessionData{"index": i}, 1*time.Hour)
	}

	all, err := ts.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(all) != 5 {
		t.Errorf("expected 5 sessions, got %d", len(all))
	}
}

func TestTieredStore_Clear(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = ts.SetWithTTL(id, SessionData{"index": i}, 1*time.Hour)
	}

	if err := ts.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	if ts.GetMemoryCount() != 0 {
		t.Errorf("expected memory count 0 after clear, got %d", ts.GetMemoryCount())
	}

	persistedCount, _ := mps.Count()
	if persistedCount != 0 {
		t.Errorf("expected persistence count 0 after clear, got %d", persistedCount)
	}
}

func TestTieredStore_EdgeCases(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()
	ts, _ := NewTieredStore(cfg, mps)

	_, err := ts.SetWithTTL("", SessionData{}, 1*time.Hour)
	if err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID, got %v", err)
	}

	_, err = ts.SetWithTTL("id", nil, 1*time.Hour)
	if err != ErrNilSessionData {
		t.Errorf("expected ErrNilSessionData, got %v", err)
	}

	_, err = ts.Get("")
	if err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID for Get, got %v", err)
	}

	_, err = ts.Renew("")
	if err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID for Renew, got %v", err)
	}

	_, err = ts.Delete("")
	if err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID for Delete, got %v", err)
	}

	if ts.Exists("") {
		t.Error("Exists should return false for empty ID")
	}

	_, err = ts.Renew("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound for Renew nonexistent, got %v", err)
	}
}

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.PersistenceDir = tmpDir

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()

	if store == nil {
		t.Fatal("NewStore returned nil")
	}
}

func TestNewStoreWithMemoryPersistence(t *testing.T) {
	cfg := DefaultConfig()
	store, err := NewStoreWithMemoryPersistence(cfg)
	if err != nil {
		t.Fatalf("NewStoreWithMemoryPersistence failed: %v", err)
	}
	defer store.Close()

	if store.GetPersistence() == nil {
		t.Error("persistence should not be nil")
	}
}

func TestStore_SetAndGet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = false
	store, err := NewStoreWithMemoryPersistence(cfg)
	if err != nil {
		t.Fatalf("NewStoreWithMemoryPersistence failed: %v", err)
	}
	defer store.Close()

	data := SessionData{"user": "alice", "age": 30}
	session, err := store.Set("sess1", data, 1*time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if session.ID != "sess1" {
		t.Errorf("expected ID sess1, got %s", session.ID)
	}
	if session.Version != 1 {
		t.Errorf("expected Version 1 for new session, got %d", session.Version)
	}

	loaded, err := store.Get("sess1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if loaded.Data["user"] != "alice" {
		t.Errorf("expected user alice, got %v", loaded.Data["user"])
	}

	if !store.Exists("sess1") {
		t.Error("session should exist")
	}

	updated, err := store.Set("sess1", SessionData{"user": "bob"}, 1*time.Hour)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("expected Version 2, got %d", updated.Version)
	}

	existed, err := store.Delete("sess1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !existed {
		t.Error("existed should be true")
	}

	if store.Exists("sess1") {
		t.Error("session should not exist after delete")
	}
}

func TestStore_Renew(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	original, _ := store.Set("sess1", SessionData{}, 1*time.Hour)
	originalExpiresAt := original.ExpiresAt

	time.Sleep(10 * time.Millisecond)

	renewed, err := store.Renew("sess1")
	if err != nil {
		t.Fatalf("Renew failed: %v", err)
	}
	if !renewed.ExpiresAt.After(originalExpiresAt) {
		t.Error("ExpiresAt should be extended")
	}
}

func TestStore_ChangeHandler(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	var wg sync.WaitGroup
	notifications := make([]ChangeNotification, 0)
	var mu sync.Mutex

	handler := func(n ChangeNotification) {
		mu.Lock()
		defer mu.Unlock()
		notifications = append(notifications, n)
		wg.Done()
	}

	store.AddChangeHandler(handler)
	store.AddChangeHandler(nil)

	wg.Add(1)
	_, _ = store.Set("sess1", SessionData{}, 1*time.Hour)
	wg.Wait()

	mu.Lock()
	if len(notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(notifications))
	}
	if notifications[0].Type != ChangeTypeCreate {
		t.Errorf("expected Create type, got %v", notifications[0].Type)
	}
	mu.Unlock()

	wg.Add(1)
	_, _ = store.Set("sess1", SessionData{"updated": true}, 1*time.Hour)
	wg.Wait()

	mu.Lock()
	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(notifications))
	}
	if notifications[1].Type != ChangeTypeUpdate {
		t.Errorf("expected Update type, got %v", notifications[1].Type)
	}
	mu.Unlock()

	wg.Add(1)
	_, _ = store.Delete("sess1")
	wg.Wait()

	mu.Lock()
	if len(notifications) != 3 {
		t.Errorf("expected 3 notifications, got %d", len(notifications))
	}
	if notifications[2].Type != ChangeTypeDelete {
		t.Errorf("expected Delete type, got %v", notifications[2].Type)
	}
	mu.Unlock()

	wg.Add(1)
	_, _ = store.Set("sess2", SessionData{}, 1*time.Hour)
	wg.Wait()

	wg.Add(1)
	_, _ = store.Renew("sess2")
	wg.Wait()

	mu.Lock()
	if len(notifications) != 5 {
		t.Errorf("expected 5 notifications, got %d", len(notifications))
	}
	if notifications[4].Type != ChangeTypeRenew {
		t.Errorf("expected Renew type, got %v", notifications[4].Type)
	}
	mu.Unlock()
}

func TestStore_GetAll(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{"i": i}, 1*time.Hour)
	}

	all, err := store.GetAll()
	if err != nil {
		t.Fatalf("GetAll failed: %v", err)
	}
	if len(all) != 10 {
		t.Errorf("expected 10 sessions, got %d", len(all))
	}
}

func TestStore_Stats(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{}, 1*time.Hour)
	}

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Get(id)
	}

	_, _ = store.Get("nonexistent")

	stats := store.Stats()
	if stats.MemoryCount != 5 {
		t.Errorf("expected MemoryCount 5, got %d", stats.MemoryCount)
	}
	if stats.HitCount != 5 {
		t.Errorf("expected HitCount 5, got %d", stats.HitCount)
	}
	if stats.MissCount != 1 {
		t.Errorf("expected MissCount 1, got %d", stats.MissCount)
	}
}

func TestStore_Clear(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{}, 1*time.Hour)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	stats := store.Stats()
	if stats.MemoryCount != 0 {
		t.Errorf("expected MemoryCount 0, got %d", stats.MemoryCount)
	}
}

func TestStore_DefaultTTL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultTTL = 2 * time.Hour
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	session, _ := store.Set("sess1", SessionData{})
	if session.TTL != 2*time.Hour {
		t.Errorf("expected TTL 2h, got %v", session.TTL)
	}
}

func TestStore_Closed(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	store.Close()

	_, err := store.Get("sess1")
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped, got %v", err)
	}

	_, err = store.Set("sess1", SessionData{})
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for Set, got %v", err)
	}

	_, err = store.Delete("sess1")
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for Delete, got %v", err)
	}

	_, err = store.Renew("sess1")
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for Renew, got %v", err)
	}

	_, err = store.ExportAll()
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for ExportAll, got %v", err)
	}

	_, err = store.ImportAll([]byte("{}"), false)
	if err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for ImportAll, got %v", err)
	}

	if err := store.Clear(); err != ErrClusterStopped {
		t.Errorf("expected ErrClusterStopped for Clear, got %v", err)
	}

	if err := store.Close(); err != nil {
		t.Errorf("double Close should not error, got %v", err)
	}
}

func TestStandaloneStore(t *testing.T) {
	cfg := DefaultConfig()
	store, err := NewStandaloneStore(cfg)
	if err != nil {
		t.Fatalf("NewStandaloneStore failed: %v", err)
	}
	defer store.Close()

	_, _ = store.Set("sess1", SessionData{}, 1*time.Hour)

	_, _ = store.Get("sess1")
	_, _ = store.Get("sess1")
	_, _ = store.Get("nonexistent")

	if store.HitCount() != 2 {
		t.Errorf("expected HitCount 2, got %d", store.HitCount())
	}
	if store.MissCount() != 1 {
		t.Errorf("expected MissCount 1, got %d", store.MissCount())
	}
}

func TestMigration_ExportImportSession(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	originalData := SessionData{"user": "alice", "age": 30, "roles": []string{"admin", "user"}}
	_, _ = store.Set("sess1", originalData, 1*time.Hour)

	exported, err := store.ExportSession("sess1")
	if err != nil {
		t.Fatalf("ExportSession failed: %v", err)
	}
	if len(exported) == 0 {
		t.Fatal("exported data is empty")
	}

	if err := ValidateMigrationData(exported); err != nil {
		t.Fatalf("ValidateMigrationData failed: %v", err)
	}

	cfg2 := DefaultConfig()
	store2, _ := NewStoreWithMemoryPersistence(cfg2)
	defer store2.Close()

	result, err := store2.ImportSession(exported)
	if err != nil {
		t.Fatalf("ImportSession failed: %v", err)
	}
	if result.ImportedCount != 1 {
		t.Errorf("expected ImportedCount 1, got %d", result.ImportedCount)
	}

	imported, _ := store2.Get("sess1")
	if imported.Data["user"] != "alice" {
		t.Errorf("imported data mismatch: %v", imported.Data)
	}
	if imported.Data["age"] != float64(30) {
		t.Errorf("imported age mismatch: %v", imported.Data["age"])
	}
}

func TestMigration_ExportImportAll(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{"index": i}, 1*time.Hour)
	}

	exported, err := store.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll failed: %v", err)
	}

	if err := ValidateMigrationData(exported); err != nil {
		t.Fatalf("ValidateMigrationData failed: %v", err)
	}

	cfg2 := DefaultConfig()
	store2, _ := NewStoreWithMemoryPersistence(cfg2)
	defer store2.Close()

	_, _ = store2.Set("sess0", SessionData{"existing": true}, 1*time.Hour)

	result, err := store2.ImportAll(exported, false)
	if err != nil {
		t.Fatalf("ImportAll failed: %v", err)
	}
	if result.ImportedCount != 9 {
		t.Errorf("expected ImportedCount 9, got %d", result.ImportedCount)
	}
	if result.SkippedCount != 1 {
		t.Errorf("expected SkippedCount 1, got %d", result.SkippedCount)
	}

	existing, _ := store2.Get("sess0")
	if existing.Data["existing"] != true {
		t.Error("existing session should not be overwritten without overwrite flag")
	}

	result2, err := store2.ImportAll(exported, true)
	if err != nil {
		t.Fatalf("ImportAll with overwrite failed: %v", err)
	}
	if result2.ImportedCount != 10 {
		t.Errorf("expected ImportedCount 10 with overwrite, got %d", result2.ImportedCount)
	}

	all, _ := store2.GetAll()
	if len(all) != 10 {
		t.Errorf("expected 10 sessions after import, got %d", len(all))
	}
}

func TestMigration_ChecksumMismatch(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	_, _ = store.Set("sess1", SessionData{"k": "v"}, 1*time.Hour)

	exported, _ := store.ExportSession("sess1")

	tampered := make([]byte, len(exported))
	copy(tampered, exported)
	tampered[len(tampered)-10] = 'X'

	err := ValidateMigrationData(tampered)
	if err == nil {
		t.Error("expected error for tampered data")
	}
}

func TestMigration_InvalidData(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	_, err := store.ImportSession([]byte(""))
	if err == nil {
		t.Error("expected error for empty data")
	}

	_, err = store.ImportSession([]byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid json")
	}

	invalidFormat := `{"header":{"format_version":999,"session_count":0,"checksum":"abc"},"sessions":[]}`
	_, err = store.ImportSession([]byte(invalidFormat))
	if err == nil {
		t.Error("expected error for invalid format version")
	}

	mismatchedCount := `{"header":{"format_version":1,"session_count":5,"checksum":"abc"},"sessions":[]}`
	err = ValidateMigrationData([]byte(mismatchedCount))
	if err == nil {
		t.Error("expected error for mismatched session count")
	}
}

func TestMigration_ExpiredSessionSkipped(t *testing.T) {
	expiredSession := &Session{
		ID:        "expired",
		Data:      SessionData{"k": "v"},
		TTL:       1 * time.Millisecond,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	exported, _ := ExportSession(expiredSession)

	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	result, err := store.ImportSession(exported)
	if err != nil {
		t.Fatalf("ImportSession failed: %v", err)
	}
	if result.SkippedCount != 1 {
		t.Errorf("expected SkippedCount 1, got %d", result.SkippedCount)
	}
	if result.ImportedCount != 0 {
		t.Errorf("expected ImportedCount 0, got %d", result.ImportedCount)
	}

	if store.Exists("expired") {
		t.Error("expired session should not be imported")
	}
}

func TestMigration_ExportSessionErrors(t *testing.T) {
	_, err := ExportSession(nil)
	if err != ErrNilSessionData {
		t.Errorf("expected ErrNilSessionData, got %v", err)
	}

	_, err = ExportSession(&Session{})
	if err != ErrEmptySessionID {
		t.Errorf("expected ErrEmptySessionID, got %v", err)
	}
}

func TestMigration_ImportAllErrors(t *testing.T) {
	_, err := ImportAllSessions([]byte(""), nil, false)
	if err == nil {
		t.Error("expected error for nil node")
	}

	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	_, err = ImportAllSessions(nil, nil, false)
	if err == nil {
		t.Error("expected error for nil data and nil node")
	}
}

func TestMigration_ExportAllErrors(t *testing.T) {
	_, err := ExportAllSessions(nil)
	if err == nil {
		t.Error("expected error for nil node")
	}
}

func TestCluster_AddRemoveNode(t *testing.T) {
	cfg := DefaultConfig()
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	if cluster.NodeCount() != 0 {
		t.Errorf("expected 0 nodes, got %d", cluster.NodeCount())
	}

	mps1 := NewMemoryPersistenceStore()
	node1, err := cluster.AddNode("node1", mps1)
	if err != nil {
		t.Fatalf("AddNode failed: %v", err)
	}
	if node1 == nil {
		t.Fatal("AddNode returned nil")
	}
	if node1.ID != "node1" {
		t.Errorf("expected ID node1, got %s", node1.ID)
	}

	if cluster.NodeCount() != 1 {
		t.Errorf("expected 1 node, got %d", cluster.NodeCount())
	}

	_, err = cluster.AddNode("node1", NewMemoryPersistenceStore())
	if err == nil {
		t.Error("expected error for duplicate node ID")
	}

	_, err = cluster.AddNode("", NewMemoryPersistenceStore())
	if err == nil {
		t.Error("expected error for empty node ID")
	}

	mps2 := NewMemoryPersistenceStore()
	node2, err := cluster.AddNode("node2", mps2)
	if err != nil {
		t.Fatalf("AddNode node2 failed: %v", err)
	}
	if node2.ID != "node2" {
		t.Errorf("expected node2 ID, got %s", node2.ID)
	}
	if cluster.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", cluster.NodeCount())
	}

	retrieved, err := cluster.GetNode("node1")
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}
	if retrieved.ID != "node1" {
		t.Errorf("expected ID node1, got %s", retrieved.ID)
	}

	_, err = cluster.GetNode("nonexistent")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound, got %v", err)
	}

	err = cluster.RemoveNode("node2")
	if err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}
	if cluster.NodeCount() != 1 {
		t.Errorf("expected 1 node after remove, got %d", cluster.NodeCount())
	}

	err = cluster.RemoveNode("nonexistent")
	if err != ErrNodeNotFound {
		t.Errorf("expected ErrNodeNotFound for remove, got %v", err)
	}
}

func TestCluster_SyncOnSet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = true
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	mps1 := NewMemoryPersistenceStore()
	node1, _ := cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	node2, _ := cluster.AddNode("node2", mps2)

	time.Sleep(50 * time.Millisecond)

	data := SessionData{"user": "alice"}
	_, err := node1.Set("sess1", data, 1*time.Hour)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if !node2.Exists("sess1") {
		t.Error("session should be synced to node2")
	}

	loaded, _ := node2.Get("sess1")
	if loaded.Data["user"] != "alice" {
		t.Errorf("synced data mismatch: %v", loaded.Data)
	}

	stats := node2.Stats()
	if stats.SyncedCount < 1 {
		t.Errorf("expected at least 1 synced session, got %d", stats.SyncedCount)
	}
}

func TestCluster_SyncOnDelete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = true
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	mps1 := NewMemoryPersistenceStore()
	node1, _ := cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	node2, _ := cluster.AddNode("node2", mps2)

	time.Sleep(50 * time.Millisecond)

	_, _ = node1.Set("sess1", SessionData{}, 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	if !node2.Exists("sess1") {
		t.Fatal("session should be synced before delete")
	}

	_, _ = node1.Delete("sess1")
	time.Sleep(100 * time.Millisecond)

	if node2.Exists("sess1") {
		t.Error("session should be deleted from node2 after sync")
	}
}

func TestCluster_SyncOnRenew(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = true
	cfg.AutoRenew = false
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	mps1 := NewMemoryPersistenceStore()
	node1, _ := cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	node2, _ := cluster.AddNode("node2", mps2)

	time.Sleep(50 * time.Millisecond)

	_, _ = node1.Set("sess1", SessionData{}, 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	original, _ := node2.GetWithoutRenew("sess1")
	originalVersion := original.Version

	time.Sleep(10 * time.Millisecond)
	_, _ = node1.Renew("sess1")
	time.Sleep(100 * time.Millisecond)

	renewed, _ := node2.GetWithoutRenew("sess1")
	if renewed.Version <= originalVersion {
		t.Errorf("expected Version > %d after renew sync, got %d", originalVersion, renewed.Version)
	}
}

func TestCluster_ConcurrentAccess(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = true
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	mps1 := NewMemoryPersistenceStore()
	node1, _ := cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	node2, _ := cluster.AddNode("node2", mps2)

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOperations := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < numOperations; i++ {
				id := fmt.Sprintf("sess-%d-%d", goroutineID, i)
				data := SessionData{"goroutine": goroutineID, "op": i}
				_, _ = node1.Set(id, data, 1*time.Hour)
			}
		}(g)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	all1, _ := node1.GetAll()
	all2, _ := node2.GetAll()

	totalExpected := numGoroutines * numOperations
	if len(all1) != totalExpected {
		t.Errorf("node1: expected %d sessions, got %d", totalExpected, len(all1))
	}
	if len(all2) != totalExpected {
		t.Errorf("node2: expected %d sessions, got %d", totalExpected, len(all2))
	}
}

func TestCluster_Stop(t *testing.T) {
	cfg := DefaultConfig()
	cluster := NewCluster(cfg)

	mps1 := NewMemoryPersistenceStore()
	_, _ = cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	_, _ = cluster.AddNode("node2", mps2)

	cluster.Stop()

	if _, err := cluster.AddNode("node3", NewMemoryPersistenceStore()); err == nil {
		t.Error("expected error adding node to stopped cluster")
	}

	cluster.Stop()
}

func TestCluster_VersionConflict(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = true
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	mps1 := NewMemoryPersistenceStore()
	node1, _ := cluster.AddNode("node1", mps1)

	mps2 := NewMemoryPersistenceStore()
	node2, _ := cluster.AddNode("node2", mps2)

	time.Sleep(50 * time.Millisecond)

	_, _ = node1.Set("sess1", SessionData{"v": 1}, 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	_, _ = node1.Set("sess1", SessionData{"v": 2}, 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	_, _ = node2.Set("sess1", SessionData{"v": 3}, 1*time.Hour)
	time.Sleep(100 * time.Millisecond)

	loaded1, _ := node1.Get("sess1")
	loaded2, _ := node2.Get("sess1")

	if loaded1.Version != loaded2.Version {
		t.Errorf("versions should match after sync: node1=%d, node2=%d", loaded1.Version, loaded2.Version)
	}
}

func TestComputeDataDigest(t *testing.T) {
	data1 := SessionData{"a": "1", "b": "2"}
	data2 := SessionData{"b": "2", "a": "1"}
	data3 := SessionData{"a": "1", "b": "3"}

	digest1 := computeDataDigest(data1)
	digest2 := computeDataDigest(data2)
	digest3 := computeDataDigest(data3)

	if digest1 != digest2 {
		t.Error("same data in different order should have same digest")
	}
	if digest1 == digest3 {
		t.Error("different data should have different digest")
	}
}

func TestComputeChecksum(t *testing.T) {
	s1 := &Session{ID: "a", Data: SessionData{"k": "v"}, Version: 1, TTL: 1 * time.Hour}
	s2 := &Session{ID: "b", Data: SessionData{"k": "v"}, Version: 1, TTL: 1 * time.Hour}

	checksum1 := computeChecksum([]*Session{s1, s2})
	checksum2 := computeChecksum([]*Session{s2, s1})
	checksum3 := computeChecksum([]*Session{s1})

	if checksum1 == checksum2 {
		t.Error("different order should produce different checksum (before sorting)")
	}
	if checksum1 == checksum3 {
		t.Error("different sessions should have different checksum")
	}
}

func TestGenerateNodeID(t *testing.T) {
	id1 := generateNodeID()
	id2 := generateNodeID()

	if id1 == id2 {
		t.Error("generated node IDs should be unique")
	}
	if len(id1) == 0 {
		t.Error("node ID should not be empty")
	}
}

func TestRandomString(t *testing.T) {
	s1 := randomString(10)
	s2 := randomString(10)

	if s1 == s2 {
		t.Error("random strings should be different")
	}
	if len(s1) != 10 {
		t.Errorf("expected length 10, got %d", len(s1))
	}
}

func TestTieredStore_Stats(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()
	ts, _ := NewTieredStore(cfg, mps)

	_, _ = ts.SetWithTTL("sess1", SessionData{}, 1*time.Hour)
	_, _ = ts.SetWithTTL("sess2", SessionData{}, 10*time.Millisecond)

	_, _ = ts.Get("sess1")
	_, _ = ts.Get("nonexistent")

	time.Sleep(50 * time.Millisecond)
	_ = ts.CleanupExpired()

	stats := ts.getStats()
	if stats.MemoryCount != 1 {
		t.Errorf("expected MemoryCount 1, got %d", stats.MemoryCount)
	}
	if stats.ExpiredCount < 1 {
		t.Errorf("expected ExpiredCount >= 1, got %d", stats.ExpiredCount)
	}
	if stats.HitCount != 1 {
		t.Errorf("expected HitCount 1, got %d", stats.HitCount)
	}
	if stats.MissCount != 1 {
		t.Errorf("expected MissCount 1, got %d", stats.MissCount)
	}
}

func TestNode_Stats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = false
	cfg.CleanupInterval = 0
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	node, _ := cluster.AddNode("node1", NewMemoryPersistenceStore())

	_, _ = node.Set("sess1", SessionData{}, 1*time.Hour)
	_, _ = node.Set("sess2", SessionData{}, 1*time.Hour)

	stats := node.Stats()
	if stats.MemoryCount != 2 {
		t.Errorf("expected MemoryCount 2, got %d", stats.MemoryCount)
	}

	sent, recv, reject := node.MessageStats()
	if sent != 0 {
		t.Errorf("expected sent 0, got %d", sent)
	}
	if recv != 0 {
		t.Errorf("expected recv 0, got %d", recv)
	}
	if reject != 0 {
		t.Errorf("expected reject 0, got %d", reject)
	}
}

func TestNode_Clear(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = false
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	node, _ := cluster.AddNode("node1", NewMemoryPersistenceStore())

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = node.Set(id, SessionData{}, 1*time.Hour)
	}

	if err := node.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	all, _ := node.GetAll()
	if len(all) != 0 {
		t.Errorf("expected 0 sessions after clear, got %d", len(all))
	}
}

func TestNode_CleanupExpired(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableSync = false
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	node, _ := cluster.AddNode("node1", NewMemoryPersistenceStore())

	_, _ = node.Set("short", SessionData{}, 10*time.Millisecond)
	_, _ = node.Set("long", SessionData{}, 1*time.Hour)

	time.Sleep(50 * time.Millisecond)

	cleaned := node.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("expected 1 session cleaned, got %d", cleaned)
	}

	if node.Exists("short") {
		t.Error("short-lived session should be cleaned")
	}
	if !node.Exists("long") {
		t.Error("long-lived session should still exist")
	}
}

func TestStore_ExportSessionNotFound(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	_, err := store.ExportSession("nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestStore_WithFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.PersistenceDir = tmpDir
	cfg.CleanupInterval = 0

	store, err := NewStore(cfg)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer store.Close()
	defer os.RemoveAll(tmpDir)

	_, _ = store.Set("sess1", SessionData{"k": "v"}, 1*time.Hour)

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	jsonCount := 0
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" {
			jsonCount++
		}
	}
	if jsonCount != 1 {
		t.Errorf("expected 1 json file, got %d", jsonCount)
	}
}

func TestStore_GetConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.NodeID = "test-node"
	cfg.DefaultTTL = 2 * time.Hour

	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	retrieved := store.GetConfig()
	if retrieved.NodeID != "test-node" {
		t.Errorf("expected NodeID test-node, got %s", retrieved.NodeID)
	}
	if retrieved.DefaultTTL != 2*time.Hour {
		t.Errorf("expected DefaultTTL 2h, got %v", retrieved.DefaultTTL)
	}
}

func TestStore_GetNode(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	node := store.GetNode()
	if node != nil {
		t.Error("GetNode should return nil for standalone store")
	}
}

func TestCluster_MessageDropRate(t *testing.T) {
	cfg := DefaultConfig()
	cluster := NewCluster(cfg)
	defer cluster.Stop()

	cluster.SetMessageDropRate(0.5)

	dropped := 0
	for i := 0; i < 100; i++ {
		time.Sleep(1 * time.Microsecond)
		if cluster.shouldDropMessage() {
			dropped++
		}
	}
	if dropped == 0 {
		t.Error("should drop some messages with 50% drop rate")
	}

	cluster.SetMessageDropRate(0)
	for i := 0; i < 10; i++ {
		if cluster.shouldDropMessage() {
			t.Error("should not drop messages with 0% drop rate")
		}
	}

	cluster.SetMessageDropRate(1.0)
	allDropped := true
	for i := 0; i < 10; i++ {
		if !cluster.shouldDropMessage() {
			allDropped = false
			break
		}
	}
	if !allDropped {
		t.Error("should drop all messages with 100% drop rate")
	}
}

func TestValidateAndParseMigrationData(t *testing.T) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	_, _ = store.Set("sess1", SessionData{}, 1*time.Hour)
	exported, _ := store.ExportAll()

	var md MigrationData
	err := validateAndParseMigrationData(exported, &md)
	if err != nil {
		t.Fatalf("validateAndParseMigrationData failed: %v", err)
	}
	if md.Header.SessionCount != 1 {
		t.Errorf("expected SessionCount 1, got %d", md.Header.SessionCount)
	}

	err = validateAndParseMigrationData([]byte(""), &md)
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestTieredStore_DefaultTTL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DefaultTTL = 2 * time.Hour
	mps := NewMemoryPersistenceStore()

	ts, _ := NewTieredStore(cfg, mps)

	session, _ := ts.SetWithTTL("sess1", SessionData{}, 0)
	if session.TTL != 2*time.Hour {
		t.Errorf("expected TTL 2h (default), got %v", session.TTL)
	}

	session2, _ := ts.SetWithTTL("sess2", SessionData{}, -1)
	if session2.TTL != 2*time.Hour {
		t.Errorf("expected TTL 2h (default) for negative TTL, got %v", session2.TTL)
	}
}

func TestTieredStore_MergeRemoteSession(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoRenew = false
	mps := NewMemoryPersistenceStore()
	ts, _ := NewTieredStore(cfg, mps)

	remote := &Session{
		ID:        "remote1",
		Data:      SessionData{"remote": true},
		TTL:       1 * time.Hour,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Version:   5,
	}

	if !ts.mergeRemoteSession(remote) {
		t.Error("merge should succeed for new session")
	}

	loaded, _ := ts.Get("remote1")
	if loaded.Data["remote"] != true {
		t.Error("remote session data should be merged")
	}
	if loaded.Version != 5 {
		t.Errorf("expected Version 5, got %d", loaded.Version)
	}

	older := &Session{
		ID:      "remote1",
		Data:    SessionData{"older": true},
		Version: 3,
	}

	if ts.mergeRemoteSession(older) {
		t.Error("merge should fail for older version")
	}

	loaded, _ = ts.Get("remote1")
	if loaded.Data["remote"] != true {
		t.Error("data should not be overwritten by older version")
	}

	newer := &Session{
		ID:      "remote1",
		Data:    SessionData{"newer": true},
		Version: 10,
	}

	if !ts.mergeRemoteSession(newer) {
		t.Error("merge should succeed for newer version")
	}

	loaded, _ = ts.Get("remote1")
	if loaded.Data["newer"] != true {
		t.Error("data should be updated to newer version")
	}
	if loaded.Version != 10 {
		t.Errorf("expected Version 10, got %d", loaded.Version)
	}

	if ts.mergeRemoteSession(nil) {
		t.Error("merge should fail for nil session")
	}
}

func TestTieredStore_ApplyRemoteDelete(t *testing.T) {
	cfg := DefaultConfig()
	mps := NewMemoryPersistenceStore()
	ts, _ := NewTieredStore(cfg, mps)

	_, _ = ts.SetWithTTL("sess1", SessionData{}, 1*time.Hour)

	if !ts.applyRemoteDelete("sess1", 1) {
		t.Error("delete should succeed")
	}

	if ts.Exists("sess1") {
		t.Error("session should be deleted")
	}

	if ts.applyRemoteDelete("nonexistent", 1) {
		t.Error("delete should fail for nonexistent session")
	}

	_, _ = ts.SetWithTTL("sess2", SessionData{}, 1*time.Hour)

	if ts.applyRemoteDelete("sess2", 0) {
		t.Error("delete should fail for older version")
	}

	if !ts.Exists("sess2") {
		t.Error("session should not be deleted with older version")
	}
}

func BenchmarkStore_Set(b *testing.B) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{"i": i}, 1*time.Hour)
	}
}

func BenchmarkStore_Get(b *testing.B) {
	cfg := DefaultConfig()
	store, _ := NewStoreWithMemoryPersistence(cfg)
	defer store.Close()

	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = store.Set(id, SessionData{"i": i}, 1*time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("sess%d", i%1000)
		_, _ = store.Get(id)
	}
}

func BenchmarkTieredStore_Get(b *testing.B) {
	cfg := DefaultConfig()
	ts, _ := NewTieredStore(cfg, NewMemoryPersistenceStore())
	defer ts.Close()

	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("sess%d", i)
		_, _ = ts.SetWithTTL(id, SessionData{"i": i}, 1*time.Hour)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := fmt.Sprintf("sess%d", i%1000)
		_, _ = ts.Get(id)
	}
}