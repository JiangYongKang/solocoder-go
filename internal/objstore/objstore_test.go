package objstore

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewObjectStore(t *testing.T) {
	store := NewObjectStore()
	if store == nil {
		t.Fatal("NewObjectStore returned nil")
	}
	if store.Count() != 0 {
		t.Errorf("expected initial count 0, got %d", store.Count())
	}
}

func TestNewObjectStoreWithConfig(t *testing.T) {
	cfg := Config{
		MaxVersions:      5,
		CleanupBatchSize: 2,
		CleanupInterval:  3,
	}
	store := NewObjectStoreWithConfig(cfg)
	if store == nil {
		t.Fatal("NewObjectStoreWithConfig returned nil")
	}
	if store.config.MaxVersions != 5 {
		t.Errorf("expected MaxVersions 5, got %d", store.config.MaxVersions)
	}
	if store.config.CleanupBatchSize != 2 {
		t.Errorf("expected CleanupBatchSize 2, got %d", store.config.CleanupBatchSize)
	}
	if store.config.CleanupInterval != 3 {
		t.Errorf("expected CleanupInterval 3, got %d", store.config.CleanupInterval)
	}
}

func TestNewObjectStoreWithConfig_Defaults(t *testing.T) {
	cfg := Config{}
	store := NewObjectStoreWithConfig(cfg)
	if store.config.MaxVersions != 10 {
		t.Errorf("expected default MaxVersions 10, got %d", store.config.MaxVersions)
	}
	if store.config.CleanupBatchSize != 1 {
		t.Errorf("expected default CleanupBatchSize 1, got %d", store.config.CleanupBatchSize)
	}
	if store.config.CleanupInterval != 1 {
		t.Errorf("expected default CleanupInterval 1, got %d", store.config.CleanupInterval)
	}
}

func TestNewObjectStoreWithConfig_Invalid(t *testing.T) {
	cfg := Config{
		MaxVersions:      -1,
		CleanupBatchSize: 0,
		CleanupInterval:  -5,
	}
	store := NewObjectStoreWithConfig(cfg)
	if store.config.MaxVersions != 10 {
		t.Errorf("expected MaxVersions to default to 10, got %d", store.config.MaxVersions)
	}
	if store.config.CleanupBatchSize != 1 {
		t.Errorf("expected CleanupBatchSize to default to 1, got %d", store.config.CleanupBatchSize)
	}
	if store.config.CleanupInterval != 1 {
		t.Errorf("expected CleanupInterval to default to 1, got %d", store.config.CleanupInterval)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxVersions != 10 {
		t.Errorf("expected default MaxVersions 10, got %d", cfg.MaxVersions)
	}
	if cfg.CleanupBatchSize != 1 {
		t.Errorf("expected default CleanupBatchSize 1, got %d", cfg.CleanupBatchSize)
	}
	if cfg.CleanupInterval != 1 {
		t.Errorf("expected default CleanupInterval 1, got %d", cfg.CleanupInterval)
	}
}

func TestPut_SingleVersion(t *testing.T) {
	store := NewObjectStore()

	ver, err := store.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	count, err := store.VersionCount("key1")
	if err != nil {
		t.Fatalf("VersionCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 version, got %d", count)
	}
}

func TestPut_MultipleVersions(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 5; i++ {
		data := []byte(fmt.Sprintf("value_%d", i))
		ver, err := store.Put("key1", data)
		if err != nil {
			t.Fatalf("Put iteration %d failed: %v", i, err)
		}
		if ver != uint64(i) {
			t.Errorf("iteration %d: expected version %d, got %d", i, i, ver)
		}
	}

	count, err := store.VersionCount("key1")
	if err != nil {
		t.Fatalf("VersionCount failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 versions, got %d", count)
	}
}

func TestPut_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	ver, err := store.Put("", []byte("value"))
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if ver != 0 {
		t.Errorf("expected version 0 for error case, got %d", ver)
	}
}

func TestPut_NilData(t *testing.T) {
	store := NewObjectStore()

	ver, err := store.Put("key1", nil)
	if err != ErrNilData {
		t.Errorf("expected ErrNilData, got %v", err)
	}
	if ver != 0 {
		t.Errorf("expected version 0 for error case, got %d", ver)
	}
}

func TestPut_DataIsolation(t *testing.T) {
	store := NewObjectStore()

	data := []byte("original")
	_, err := store.Put("key1", data)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	data[0] = 'm'

	got, _, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(got, []byte("original")) {
		t.Errorf("stored data was modified after Put: got %q", got)
	}
}

func TestGet_LatestVersion(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))
	store.Put("key1", []byte("v2"))
	store.Put("key1", []byte("v3"))

	data, ver, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(data, []byte("v3")) {
		t.Errorf("expected 'v3', got %q", data)
	}
	if ver != 3 {
		t.Errorf("expected version 3, got %d", ver)
	}
}

func TestGet_KeyNotFound(t *testing.T) {
	store := NewObjectStore()

	data, ver, err := store.Get("nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
	if ver != 0 {
		t.Errorf("expected version 0, got %d", ver)
	}
}

func TestGet_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	data, ver, err := store.Get("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
	if ver != 0 {
		t.Errorf("expected version 0, got %d", ver)
	}
}

func TestGet_ReturnsCopy(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("original"))

	data1, _, _ := store.Get("key1")
	data2, _, _ := store.Get("key1")

	data1[0] = 'X'

	if !bytes.Equal(data2, []byte("original")) {
		t.Error("Get should return a copy of the data")
	}
}

func TestGetVersion_Specific(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("value_%d", i)))
	}

	for i := 1; i <= 5; i++ {
		data, err := store.GetVersion("key1", uint64(i))
		if err != nil {
			t.Fatalf("GetVersion(%d) failed: %v", i, err)
		}
		expected := []byte(fmt.Sprintf("value_%d", i))
		if !bytes.Equal(data, expected) {
			t.Errorf("version %d: expected %q, got %q", i, expected, data)
		}
	}
}

func TestGetVersion_NotFound(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))

	data, err := store.GetVersion("key1", 999)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
}

func TestGetVersion_KeyNotFound(t *testing.T) {
	store := NewObjectStore()

	data, err := store.GetVersion("nonexistent", 1)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
}

func TestGetVersion_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	data, err := store.GetVersion("", 1)
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if data != nil {
		t.Errorf("expected nil data, got %v", data)
	}
}

func TestListVersions(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 3; i++ {
		time.Sleep(time.Millisecond)
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	versions, err := store.ListVersions("key1")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	for i, v := range versions {
		expectedVer := uint64(i + 1)
		if v.Version != expectedVer {
			t.Errorf("version %d: expected version number %d, got %d", i, expectedVer, v.Version)
		}
		if v.CreatedAt.IsZero() {
			t.Errorf("version %d: CreatedAt should not be zero", i)
		}
	}

	for i := 1; i < len(versions); i++ {
		if !versions[i].CreatedAt.After(versions[i-1].CreatedAt) {
			t.Errorf("version %d should be created after version %d", i+1, i)
		}
	}
}

func TestListVersions_KeyNotFound(t *testing.T) {
	store := NewObjectStore()

	versions, err := store.ListVersions("nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if versions != nil {
		t.Errorf("expected nil versions, got %v", versions)
	}
}

func TestListVersions_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	versions, err := store.ListVersions("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if versions != nil {
		t.Errorf("expected nil versions, got %v", versions)
	}
}

func TestRollback_ToEarlierVersion(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))
	store.Put("key1", []byte("v2"))
	store.Put("key1", []byte("v3"))

	newVer, err := store.Rollback("key1", 1)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if newVer != 4 {
		t.Errorf("expected new version 4, got %d", newVer)
	}

	data, _, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get after rollback failed: %v", err)
	}
	if !bytes.Equal(data, []byte("v1")) {
		t.Errorf("expected 'v1' after rollback to version 1, got %q", data)
	}

	count, _ := store.VersionCount("key1")
	if count != 4 {
		t.Errorf("expected 4 versions after rollback, got %d", count)
	}
}

func TestRollback_ToLatestVersion(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))
	store.Put("key1", []byte("v2"))

	newVer, err := store.Rollback("key1", 2)
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}
	if newVer != 3 {
		t.Errorf("expected new version 3, got %d", newVer)
	}

	data, _, _ := store.Get("key1")
	if !bytes.Equal(data, []byte("v2")) {
		t.Errorf("expected 'v2' after rollback to version 2, got %q", data)
	}
}

func TestRollback_KeyNotFound(t *testing.T) {
	store := NewObjectStore()

	newVer, err := store.Rollback("nonexistent", 1)
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if newVer != 0 {
		t.Errorf("expected version 0, got %d", newVer)
	}
}

func TestRollback_VersionNotFound(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))

	newVer, err := store.Rollback("key1", 999)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}
	if newVer != 0 {
		t.Errorf("expected version 0, got %d", newVer)
	}
}

func TestRollback_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	newVer, err := store.Rollback("", 1)
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if newVer != 0 {
		t.Errorf("expected version 0, got %d", newVer)
	}
}

func TestRollback_CreatesNewVersion(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("original"))
	store.Put("key1", []byte("modified"))

	rollbackVer, _ := store.Rollback("key1", 1)

	versionData, err := store.GetVersion("key1", 2)
	if err != nil {
		t.Fatalf("GetVersion(2) failed: %v", err)
	}
	if !bytes.Equal(versionData, []byte("modified")) {
		t.Error("rollback should not modify existing versions")
	}

	rollbackData, err := store.GetVersion("key1", rollbackVer)
	if err != nil {
		t.Fatalf("GetVersion(rollback) failed: %v", err)
	}
	if !bytes.Equal(rollbackData, []byte("original")) {
		t.Error("rollback version should contain rolled back data")
	}
}

func TestDelete(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))
	store.Put("key2", []byte("v2"))

	deleted := store.Delete("key1")
	if !deleted {
		t.Error("expected Delete to return true for existing key")
	}

	_, _, err := store.Get("key1")
	if err != ErrKeyNotFound {
		t.Error("expected key1 to be deleted")
	}

	_, _, err = store.Get("key2")
	if err != nil {
		t.Error("expected key2 to still exist")
	}

	if store.Count() != 1 {
		t.Errorf("expected count 1, got %d", store.Count())
	}
}

func TestDelete_NonExistent(t *testing.T) {
	store := NewObjectStore()

	deleted := store.Delete("nonexistent")
	if deleted {
		t.Error("expected Delete to return false for non-existent key")
	}
}

func TestDelete_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	deleted := store.Delete("")
	if deleted {
		t.Error("expected Delete to return false for empty key")
	}
}

func TestCount(t *testing.T) {
	store := NewObjectStore()

	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}

	for i := 0; i < 10; i++ {
		store.Put(fmt.Sprintf("key%d", i), []byte("value"))
	}

	if store.Count() != 10 {
		t.Errorf("expected 10, got %d", store.Count())
	}

	store.Delete("key0")
	if store.Count() != 9 {
		t.Errorf("expected 9 after delete, got %d", store.Count())
	}
}

func TestVersionCount(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("v1"))
	store.Put("key1", []byte("v2"))
	store.Put("key2", []byte("v1"))

	count, err := store.VersionCount("key1")
	if err != nil {
		t.Fatalf("VersionCount failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 versions for key1, got %d", count)
	}

	count, err = store.VersionCount("key2")
	if err != nil {
		t.Fatalf("VersionCount failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 version for key2, got %d", count)
	}
}

func TestVersionCount_KeyNotFound(t *testing.T) {
	store := NewObjectStore()

	count, err := store.VersionCount("nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestVersionCount_EmptyKey(t *testing.T) {
	store := NewObjectStore()

	count, err := store.VersionCount("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestCleanup_AutoOnPut(t *testing.T) {
	cfg := Config{
		MaxVersions:      3,
		CleanupBatchSize: 1,
		CleanupInterval:  1,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ := store.VersionCount("key1")
	if count != 3 {
		t.Errorf("expected 3 versions after cleanup, got %d", count)
	}

	versions, _ := store.ListVersions("key1")
	expectedVersions := []uint64{3, 4, 5}
	for i, v := range versions {
		if v.Version != expectedVersions[i] {
			t.Errorf("position %d: expected version %d, got %d", i, expectedVersions[i], v.Version)
		}
	}
}

func TestCleanup_BatchSize(t *testing.T) {
	cfg := Config{
		MaxVersions:      5,
		CleanupBatchSize: 2,
		CleanupInterval:  10,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 9; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ := store.VersionCount("key1")
	if count != 9 {
		t.Errorf("expected 9 versions before cleanup trigger (interval 10), got %d", count)
	}

	store.Put("key1", []byte("v10"))

	count, _ = store.VersionCount("key1")
	if count != 8 {
		t.Errorf("expected 8 versions after first batch cleanup (10 - 2), got %d", count)
	}

	for i := 11; i <= 19; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}
	count, _ = store.VersionCount("key1")
	if count != 17 {
		t.Errorf("expected 17 versions (8 + 9) before second cleanup trigger, got %d", count)
	}

	store.Put("key1", []byte("v20"))
	count, _ = store.VersionCount("key1")
	if count != 16 {
		t.Errorf("expected 16 versions after second batch cleanup (18 - 2), got %d", count)
	}
}

func TestCleanup_BatchSizeLargerThanExcess(t *testing.T) {
	cfg := Config{
		MaxVersions:      3,
		CleanupBatchSize: 10,
		CleanupInterval:  1,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ := store.VersionCount("key1")
	if count != 3 {
		t.Errorf("expected 3 versions, got %d", count)
	}
}

func TestCleanupAll(t *testing.T) {
	cfg := Config{
		MaxVersions:      2,
		CleanupBatchSize: 10,
		CleanupInterval:  100,
	}
	store := NewObjectStoreWithConfig(cfg)

	for k := 0; k < 3; k++ {
		key := fmt.Sprintf("key%d", k)
		for i := 1; i <= 5; i++ {
			store.Put(key, []byte(fmt.Sprintf("v%d", i)))
		}
	}

	for k := 0; k < 3; k++ {
		key := fmt.Sprintf("key%d", k)
		count, _ := store.VersionCount(key)
		if count != 5 {
			t.Errorf("%s: expected 5 versions before CleanupAll, got %d", key, count)
		}
	}

	cleaned := store.CleanupAll()
	if cleaned != 9 {
		t.Errorf("expected 9 versions cleaned (3 keys * 3 excess each), got %d", cleaned)
	}

	for k := 0; k < 3; k++ {
		key := fmt.Sprintf("key%d", k)
		count, _ := store.VersionCount(key)
		if count != 2 {
			t.Errorf("%s: expected 2 versions after CleanupAll, got %d", key, count)
		}
	}
}

func TestCleanupAll_MultipleBatches(t *testing.T) {
	cfg := Config{
		MaxVersions:      2,
		CleanupBatchSize: 1,
		CleanupInterval:  100,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 6; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ := store.VersionCount("key1")
	if count != 6 {
		t.Errorf("expected 6 versions before CleanupAll, got %d", count)
	}

	cleaned := store.CleanupAll()
	if cleaned != 1 {
		t.Errorf("expected 1 version cleaned in first batch, got %d", cleaned)
	}

	count, _ = store.VersionCount("key1")
	if count != 5 {
		t.Errorf("expected 5 versions after first CleanupAll batch, got %d", count)
	}

	store.CleanupAll()
	store.CleanupAll()
	store.CleanupAll()

	count, _ = store.VersionCount("key1")
	if count != 2 {
		t.Errorf("expected 2 versions after multiple CleanupAll calls, got %d", count)
	}
}

func TestCleanup_WithRollback(t *testing.T) {
	cfg := Config{
		MaxVersions:      3,
		CleanupBatchSize: 1,
		CleanupInterval:  1,
	}
	store := NewObjectStoreWithConfig(cfg)

	store.Put("key1", []byte("v1"))
	store.Put("key1", []byte("v2"))
	store.Put("key1", []byte("v3"))

	store.Rollback("key1", 1)
	store.Rollback("key1", 2)

	count, _ := store.VersionCount("key1")
	if count != 3 {
		t.Errorf("expected 3 versions after rollbacks with cleanup, got %d", count)
	}

	versions, _ := store.ListVersions("key1")
	if versions[0].Version != 3 {
		t.Errorf("oldest version should be 3, got %d", versions[0].Version)
	}
	if versions[len(versions)-1].Version != 5 {
		t.Errorf("newest version should be 5, got %d", versions[len(versions)-1].Version)
	}
}

func TestCleanup_NoCleanupNeeded(t *testing.T) {
	cfg := Config{
		MaxVersions:      10,
		CleanupBatchSize: 1,
		CleanupInterval:  1,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	cleaned := store.CleanupAll()
	if cleaned != 0 {
		t.Errorf("expected 0 versions cleaned, got %d", cleaned)
	}

	count, _ := store.VersionCount("key1")
	if count != 5 {
		t.Errorf("expected 5 versions, got %d", count)
	}
}

func TestCleanupInterval(t *testing.T) {
	cfg := Config{
		MaxVersions:      5,
		CleanupBatchSize: 3,
		CleanupInterval:  5,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ := store.VersionCount("key1")
	if count != 5 {
		t.Errorf("expected 5 versions (at max), got %d", count)
	}

	for i := 6; i <= 9; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	count, _ = store.VersionCount("key1")
	if count != 9 {
		t.Errorf("expected 9 versions (before interval trigger), got %d", count)
	}

	store.Put("key1", []byte("v10"))

	count, _ = store.VersionCount("key1")
	if count != 7 {
		t.Errorf("expected 7 versions after interval cleanup (10 - 3), got %d", count)
	}
}

func TestConcurrentPut(t *testing.T) {
	store := NewObjectStore()

	var wg sync.WaitGroup
	numGoroutines := 20
	numOps := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("g%d_k%d", id, i%10)
				data := []byte(fmt.Sprintf("v_g%d_i%d", id, i))
				store.Put(key, data)
			}
		}(g)
	}

	wg.Wait()

	if store.Count() != numGoroutines*10 {
		t.Errorf("expected %d keys, got %d", numGoroutines*10, store.Count())
	}
}

func TestConcurrentGet(t *testing.T) {
	store := NewObjectStore()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		store.Put(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("value%d", i)))
	}

	var wg sync.WaitGroup
	numGoroutines := 30

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numKeys; i++ {
				data, ver, err := store.Get(fmt.Sprintf("key%d", i))
				if err != nil {
					t.Errorf("Get failed for key%d: %v", i, err)
					return
				}
				if ver != 1 {
					t.Errorf("key%d: expected version 1, got %d", i, ver)
				}
				expected := []byte(fmt.Sprintf("value%d", i))
				if !bytes.Equal(data, expected) {
					t.Errorf("key%d: value mismatch", i)
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentPutAndGet(t *testing.T) {
	store := NewObjectStore()

	numKeys := 50
	for i := 0; i < numKeys; i++ {
		store.Put(fmt.Sprintf("k%d", i), []byte(fmt.Sprintf("v%d_initial", i)))
	}

	var wg sync.WaitGroup
	var getErrors int32

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			idx := i % numKeys
			store.Put(fmt.Sprintf("k%d", idx), []byte(fmt.Sprintf("v%d_%d", idx, i)))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			idx := i % numKeys
			_, _, err := store.Get(fmt.Sprintf("k%d", idx))
			if err != nil {
				getErrors++
			}
		}
	}()

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get errors during concurrent Put/Get", getErrors)
	}
}

func TestConcurrentRollback(t *testing.T) {
	cfg := Config{
		MaxVersions:      100,
		CleanupBatchSize: 1,
		CleanupInterval:  1,
	}
	store := NewObjectStoreWithConfig(cfg)

	for i := 1; i <= 5; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	var wg sync.WaitGroup
	numGoroutines := 10

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			targetVersion := uint64((id % 5) + 1)
			_, err := store.Rollback("key1", targetVersion)
			if err != nil {
				t.Errorf("Rollback failed in goroutine %d: %v", id, err)
			}
		}(g)
	}

	wg.Wait()

	count, _ := store.VersionCount("key1")
	expectedMin := 5 + numGoroutines
	if count < expectedMin {
		t.Errorf("expected at least %d versions, got %d", expectedMin, count)
	}
}

func TestConcurrentListVersions(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 10; i++ {
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	var wg sync.WaitGroup
	numGoroutines := 20

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			versions, err := store.ListVersions("key1")
			if err != nil {
				t.Errorf("ListVersions failed: %v", err)
				return
			}
			if len(versions) != 10 {
				t.Errorf("expected 10 versions, got %d", len(versions))
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentCleanup(t *testing.T) {
	cfg := Config{
		MaxVersions:      5,
		CleanupBatchSize: 1,
		CleanupInterval:  3,
	}
	store := NewObjectStoreWithConfig(cfg)

	var wg sync.WaitGroup
	numGoroutines := 10
	numOps := 50

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("concurrent_key_%d", id%3)
				data := []byte(fmt.Sprintf("v_g%d_i%d", id, i))
				store.Put(key, data)
			}
		}(g)
	}

	wg.Wait()

	for k := 0; k < 3; k++ {
		key := fmt.Sprintf("concurrent_key_%d", k)
		count, err := store.VersionCount(key)
		if err != nil {
			t.Errorf("VersionCount failed for %s: %v", key, err)
			continue
		}
		if count < 1 {
			t.Errorf("%s should have at least 1 version, got %d", key, count)
		}
	}
}

func TestEmptyData(t *testing.T) {
	store := NewObjectStore()

	ver, err := store.Put("key1", []byte{})
	if err != nil {
		t.Fatalf("Put with empty data failed: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	data, _, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data, got %d bytes", len(data))
	}
	if data == nil {
		t.Error("data should be empty slice, not nil")
	}
}

func TestVersionMonotonicallyIncreasing(t *testing.T) {
	store := NewObjectStore()

	var prevVer uint64
	for i := 0; i < 20; i++ {
		ver, err := store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
		if err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if ver <= prevVer {
			t.Errorf("version should be strictly increasing: got %d after %d", ver, prevVer)
		}
		prevVer = ver
	}
}

func TestMultipleKeysIndependent(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 3; i++ {
		for j := 1; j <= i; j++ {
			store.Put(fmt.Sprintf("key%d", i), []byte(fmt.Sprintf("v%d", j)))
		}
	}

	for i := 1; i <= 3; i++ {
		key := fmt.Sprintf("key%d", i)
		count, err := store.VersionCount(key)
		if err != nil {
			t.Fatalf("VersionCount(%s) failed: %v", key, err)
		}
		if count != i {
			t.Errorf("%s: expected %d versions, got %d", key, i, count)
		}
	}
}

func TestErrors_Values(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrVersionNotFound == nil {
		t.Error("ErrVersionNotFound should not be nil")
	}
	if ErrInvalidMaxVersion == nil {
		t.Error("ErrInvalidMaxVersion should not be nil")
	}
	if ErrNilData == nil {
		t.Error("ErrNilData should not be nil")
	}
	if ErrEmptyKey == nil {
		t.Error("ErrEmptyKey should not be nil")
	}
	if ErrInvalidBatchSize == nil {
		t.Error("ErrInvalidBatchSize should not be nil")
	}
}

func TestPut_LargeData(t *testing.T) {
	store := NewObjectStore()

	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	ver, err := store.Put("large_key", largeData)
	if err != nil {
		t.Fatalf("Put with large data failed: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}

	retrieved, _, err := store.Get("large_key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !bytes.Equal(retrieved, largeData) {
		t.Error("large data mismatch after Put/Get")
	}
}

func TestListVersions_Ordering(t *testing.T) {
	store := NewObjectStore()

	for i := 1; i <= 10; i++ {
		time.Sleep(time.Microsecond)
		store.Put("key1", []byte(fmt.Sprintf("v%d", i)))
	}

	versions, err := store.ListVersions("key1")
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}

	for i := 1; i < len(versions); i++ {
		if versions[i].Version <= versions[i-1].Version {
			t.Errorf("versions should be in ascending order: %d after %d",
				versions[i].Version, versions[i-1].Version)
		}
		if !versions[i].CreatedAt.After(versions[i-1].CreatedAt) {
			t.Errorf("creation times should be ascending")
		}
	}
}

func TestRollback_VersionsPreserved(t *testing.T) {
	store := NewObjectStore()

	store.Put("key1", []byte("first"))
	store.Put("key1", []byte("second"))
	store.Put("key1", []byte("third"))

	store.Rollback("key1", 1)

	v1, err := store.GetVersion("key1", 1)
	if err != nil {
		t.Fatalf("GetVersion(1) failed: %v", err)
	}
	if !bytes.Equal(v1, []byte("first")) {
		t.Error("version 1 should still be 'first' after rollback")
	}

	v2, err := store.GetVersion("key1", 2)
	if err != nil {
		t.Fatalf("GetVersion(2) failed: %v", err)
	}
	if !bytes.Equal(v2, []byte("second")) {
		t.Error("version 2 should still be 'second' after rollback")
	}
}
