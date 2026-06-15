package hotcold

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func newTestManager(t *testing.T, cfg Config) *HotColdManager {
	t.Helper()
	m, err := NewHotColdManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewHotColdManagerWithConfig failed: %v", err)
	}
	return m
}

func TestNewHotColdManager(t *testing.T) {
	m := NewHotColdManager()
	if m == nil {
		t.Fatal("NewHotColdManager returned nil")
	}
	if m.TotalCount() != 0 {
		t.Errorf("expected initial total count 0, got %d", m.TotalCount())
	}
	if m.HotCount() != 0 {
		t.Errorf("expected initial hot count 0, got %d", m.HotCount())
	}
	if m.ColdCount() != 0 {
		t.Errorf("expected initial cold count 0, got %d", m.ColdCount())
	}
}

func TestNewHotColdManagerWithConfig_Defaults(t *testing.T) {
	cfg := Config{}
	m, err := NewHotColdManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewHotColdManagerWithConfig returned error: %v", err)
	}
	if m == nil {
		t.Fatal("NewHotColdManagerWithConfig returned nil")
	}

	gotCfg := m.GetConfig()
	if gotCfg.HotThreshold <= 0 {
		t.Error("HotThreshold should have default positive value")
	}
	if gotCfg.ColdThreshold <= 0 {
		t.Error("ColdThreshold should have default positive value")
	}
	if gotCfg.HotThreshold <= gotCfg.ColdThreshold {
		t.Errorf("HotThreshold should be > ColdThreshold, got hot=%.2f, cold=%.2f", gotCfg.HotThreshold, gotCfg.ColdThreshold)
	}
	if gotCfg.DecayHalfLife <= 0 {
		t.Error("DecayHalfLife should have default positive value")
	}
	if gotCfg.ColdCheckCycles <= 0 {
		t.Error("ColdCheckCycles should have default positive value")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.HotThreshold != 10.0 {
		t.Errorf("expected default HotThreshold 10.0, got %.2f", cfg.HotThreshold)
	}
	if cfg.ColdThreshold != 2.0 {
		t.Errorf("expected default ColdThreshold 2.0, got %.2f", cfg.ColdThreshold)
	}
	if cfg.DecayHalfLife != time.Hour {
		t.Errorf("expected default DecayHalfLife 1h, got %v", cfg.DecayHalfLife)
	}
	if cfg.ColdCheckCycles != 3 {
		t.Errorf("expected default ColdCheckCycles 3, got %d", cfg.ColdCheckCycles)
	}
	if cfg.HotCapacityRatio != 0.3 {
		t.Errorf("expected default HotCapacityRatio 0.3, got %.2f", cfg.HotCapacityRatio)
	}
	if !cfg.AutoAdjustThresholds {
		t.Error("expected default AutoAdjustThresholds to be true")
	}
}

func TestPutAndGet_Basic(t *testing.T) {
	m := NewHotColdManager()

	err := m.Put("key1", "value1")
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	val, ok, err := m.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !ok {
		t.Error("expected key1 to exist")
	}
	if val != "value1" {
		t.Errorf("expected value1, got %v", val)
	}

	if m.TotalCount() != 1 {
		t.Errorf("expected total count 1, got %d", m.TotalCount())
	}
}

func TestPut_NilManager(t *testing.T) {
	var m *HotColdManager
	err := m.Put("key", "value")
	if err != ErrNilManager {
		t.Errorf("expected ErrNilManager, got %v", err)
	}
}

func TestGet_NilManager(t *testing.T) {
	var m *HotColdManager
	val, ok, err := m.Get("key")
	if err != ErrNilManager {
		t.Errorf("expected ErrNilManager, got %v", err)
	}
	if ok {
		t.Error("expected ok to be false for nil manager")
	}
	if val != nil {
		t.Errorf("expected nil value for nil manager, got %v", val)
	}
}

func TestPut_EmptyKey(t *testing.T) {
	m := NewHotColdManager()
	err := m.Put("", "value")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
}

func TestGet_EmptyKey(t *testing.T) {
	m := NewHotColdManager()
	val, ok, err := m.Get("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if ok {
		t.Error("expected ok to be false for empty key")
	}
	if val != nil {
		t.Errorf("expected nil value for empty key, got %v", val)
	}
}

func TestGet_NonExistent(t *testing.T) {
	m := NewHotColdManager()
	val, ok, err := m.Get("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if ok {
		t.Error("expected ok to be false for non-existent key")
	}
	if val != nil {
		t.Errorf("expected nil value, got %v", val)
	}
}

func TestDelete(t *testing.T) {
	m := NewHotColdManager()

	m.Put("key1", "value1")

	deleted := m.Delete("key1")
	if !deleted {
		t.Error("expected Delete to return true for existing key")
	}

	_, ok, err := m.Get("key1")
	if ok {
		t.Error("expected key1 to be deleted")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound after delete, got %v", err)
	}

	if m.TotalCount() != 0 {
		t.Errorf("expected total count 0 after delete, got %d", m.TotalCount())
	}
}

func TestDelete_NonExistent(t *testing.T) {
	m := NewHotColdManager()
	deleted := m.Delete("nonexistent")
	if deleted {
		t.Error("expected Delete to return false for non-existent key")
	}
}

func TestDelete_NilManager(t *testing.T) {
	var m *HotColdManager
	deleted := m.Delete("key")
	if deleted {
		t.Error("expected Delete to return false for nil manager")
	}
}

func TestPut_UpdateExisting(t *testing.T) {
	m := NewHotColdManager()

	m.Put("key1", "old_value")
	m.Put("key1", "new_value")

	val, ok, _ := m.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "new_value" {
		t.Errorf("expected new_value, got %v", val)
	}

	entry, _, _ := m.GetEntry("key1")
	if entry.AccessCount != 3 {
		t.Errorf("expected access count 3 (put + put + get), got %d", entry.AccessCount)
	}
}

func TestGetScore(t *testing.T) {
	m := newTestManager(t, Config{
		HotThreshold:     5.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
	})

	m.Put("key1", "value1")
	for i := 0; i < 5; i++ {
		m.Get("key1")
	}

	score, ok, err := m.GetScore("key1")
	if err != nil {
		t.Fatalf("GetScore failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key1 to exist for GetScore")
	}
	if score <= 0 {
		t.Errorf("expected positive score, got %.2f", score)
	}
}

func TestGetScore_NonExistent(t *testing.T) {
	m := NewHotColdManager()
	score, ok, err := m.GetScore("nonexistent")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if ok {
		t.Error("expected ok=false for non-existent key")
	}
	if score != 0 {
		t.Errorf("expected score 0, got %.2f", score)
	}
}

func TestPromoteToHot(t *testing.T) {
	cfg := Config{
		HotThreshold:     3.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
	}
	m := newTestManager(t, cfg)

	m.Put("hotkey", "value")
	if m.HotCount() != 0 {
		t.Error("new key should start in cold tier")
	}
	if m.ColdCount() != 1 {
		t.Error("new key should be in cold store")
	}

	for i := 0; i < 10; i++ {
		m.Get("hotkey")
	}

	if m.HotCount() != 1 {
		t.Errorf("expected 1 hot key after many accesses, got %d", m.HotCount())
	}
	if m.ColdCount() != 0 {
		t.Errorf("expected 0 cold keys after promotion, got %d", m.ColdCount())
	}

	entry, ok, _ := m.GetEntry("hotkey")
	if !ok {
		t.Fatal("hotkey should exist")
	}
	if entry.Tier != TierHot {
		t.Errorf("expected TierHot, got %v", entry.Tier)
	}
}

func TestDemoteToCold(t *testing.T) {
	cfg := Config{
		HotThreshold:     2.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Millisecond * 10,
		ColdCheckCycles:  2,
		HotCapacityRatio: 0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	for i := 0; i < 10; i++ {
		m.Get("key1")
	}
	if m.HotCount() != 1 {
		t.Fatal("expected key1 to be hot after accesses")
	}

	migrated := 0
	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond * 15)
		migrated += m.CheckAndMigrate()
	}

	if m.ColdCount() != 1 {
		t.Errorf("expected 1 cold key after decay, got %d", m.ColdCount())
	}
	if m.HotCount() != 0 {
		t.Errorf("expected 0 hot keys after demotion, got %d", m.HotCount())
	}
	if migrated <= 0 {
		t.Error("expected some migrations during CheckAndMigrate")
	}

	entry, ok, _ := m.GetEntry("key1")
	if !ok {
		t.Fatal("key1 should exist")
	}
	if entry.Tier != TierCold {
		t.Errorf("expected TierCold, got %v", entry.Tier)
	}
}

func TestCheckAndMigrate_PromotesColdToHot(t *testing.T) {
	cfg := Config{
		HotThreshold:     5.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	if m.ColdCount() != 1 {
		t.Fatal("expected key1 to start cold")
	}

	for i := 0; i < 20; i++ {
		m.Get("key1")
	}

	if m.HotCount() != 1 {
		migrated := m.CheckAndMigrate()
		t.Logf("migrated %d entries", migrated)
	}

	if m.HotCount() != 1 {
		t.Errorf("expected key1 to be hot after CheckAndMigrate, got %d hot keys", m.HotCount())
	}
}

func TestCheckAndMigrate_NilManager(t *testing.T) {
	var m *HotColdManager
	result := m.CheckAndMigrate()
	if result != 0 {
		t.Errorf("expected 0 for nil manager, got %d", result)
	}
}

func TestConsecutiveColdCycles(t *testing.T) {
	cfg := Config{
		HotThreshold:     5.0,
		ColdThreshold:    3.0,
		DecayHalfLife:    time.Millisecond * 5,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	for i := 0; i < 5; i++ {
		m.Get("key1")
	}
	if m.HotCount() != 1 {
		t.Fatalf("expected key1 to be hot initially, got %d hot keys", m.HotCount())
	}

	entry, _, _ := m.GetEntry("key1")
	if entry.ConsecutiveColdCycles != 0 {
		t.Errorf("expected initial ConsecutiveColdCycles=0, got %d", entry.ConsecutiveColdCycles)
	}
	t.Logf("Initial score: %.2f", entry.Score)

	time.Sleep(time.Millisecond * 15)
	m.CheckAndMigrate()

	entry, _, _ = m.GetEntry("key1")
	t.Logf("After first check: tier=%v, score=%.2f, coldCycles=%d", entry.Tier, entry.Score, entry.ConsecutiveColdCycles)
	if entry.Tier == TierHot && entry.ConsecutiveColdCycles < 1 {
		t.Error("expected ConsecutiveColdCycles to increase after cold check")
	}

	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond * 10)
		m.CheckAndMigrate()
	}

	if m.ColdCount() != 1 {
		t.Errorf("expected key1 to be demoted after enough cold cycles, got %d cold keys", m.ColdCount())
	}
}

func TestAutoAdjustThresholds(t *testing.T) {
	cfg := Config{
		HotThreshold:         10.0,
		ColdThreshold:        2.0,
		DecayHalfLife:        time.Hour,
		ColdCheckCycles:      3,
		HotCapacityRatio:     0.5,
		AutoAdjustThresholds: true,
		AdjustInterval:       time.Millisecond,
		MinHotThreshold:      1.0,
		MaxHotThreshold:      100.0,
		MinColdThreshold:     0.1,
		MaxColdThreshold:     20.0,
	}
	m := newTestManager(t, cfg)

	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("hotkey_%d", i)
		m.Put(key, "value")
		for j := 0; j < 50; j++ {
			m.Get(key)
		}
	}

	for i := 20; i < 30; i++ {
		key := fmt.Sprintf("coldkey_%d", i)
		m.Put(key, "value")
	}

	initialHot := m.HotCount()
	initialCfg := m.GetConfig()

	time.Sleep(time.Millisecond * 2)
	m.CheckAndMigrate()

	adjustedCfg := m.GetConfig()

	if adjustedCfg.HotThreshold == initialCfg.HotThreshold {
		t.Log("Thresholds not adjusted (may be within target range)")
	}

	if adjustedCfg.HotThreshold < adjustedCfg.MinHotThreshold {
		t.Errorf("HotThreshold below min: %.2f < %.2f", adjustedCfg.HotThreshold, adjustedCfg.MinHotThreshold)
	}
	if adjustedCfg.HotThreshold > adjustedCfg.MaxHotThreshold {
		t.Errorf("HotThreshold above max: %.2f > %.2f", adjustedCfg.HotThreshold, adjustedCfg.MaxHotThreshold)
	}
	if adjustedCfg.ColdThreshold < adjustedCfg.MinColdThreshold {
		t.Errorf("ColdThreshold below min: %.2f < %.2f", adjustedCfg.ColdThreshold, adjustedCfg.MinColdThreshold)
	}
	if adjustedCfg.ColdThreshold > adjustedCfg.MaxColdThreshold {
		t.Errorf("ColdThreshold above max: %.2f > %.2f", adjustedCfg.ColdThreshold, adjustedCfg.MaxColdThreshold)
	}

	_ = initialHot
}

func TestAutoAdjustThresholds_Disabled(t *testing.T) {
	cfg := Config{
		HotThreshold:         10.0,
		ColdThreshold:        2.0,
		DecayHalfLife:        time.Hour,
		ColdCheckCycles:      3,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: false,
		AdjustInterval:       time.Millisecond,
		MinHotThreshold:      1.0,
		MaxHotThreshold:      100.0,
		MinColdThreshold:     0.1,
		MaxColdThreshold:     20.0,
	}
	m := newTestManager(t, cfg)

	for i := 0; i < 50; i++ {
		m.Put(fmt.Sprintf("key_%d", i), "value")
	}

	initialCfg := m.GetConfig()
	time.Sleep(time.Millisecond * 2)
	m.CheckAndMigrate()

	afterCfg := m.GetConfig()
	if afterCfg.HotThreshold != initialCfg.HotThreshold {
		t.Error("thresholds should not change when AutoAdjustThresholds is false")
	}
}

func TestGetEntry(t *testing.T) {
	m := NewHotColdManager()
	m.Put("key1", "value1")
	m.Get("key1")

	entry, ok, err := m.GetEntry("key1")
	if err != nil {
		t.Fatalf("GetEntry failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if entry.Key != "key1" {
		t.Errorf("expected key=key1, got %s", entry.Key)
	}
	if entry.Value != "value1" {
		t.Errorf("expected value=value1, got %v", entry.Value)
	}
	if entry.AccessCount != 2 {
		t.Errorf("expected access count 2, got %d", entry.AccessCount)
	}
	if entry.Tier != TierCold {
		t.Errorf("expected TierCold, got %v", entry.Tier)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if entry.LastAccessTime.IsZero() {
		t.Error("LastAccessTime should not be zero")
	}
}

func TestGetEntry_NilManager(t *testing.T) {
	var m *HotColdManager
	entry, ok, err := m.GetEntry("key")
	if err != ErrNilManager {
		t.Errorf("expected ErrNilManager, got %v", err)
	}
	if ok {
		t.Error("expected ok=false for nil manager")
	}
	if entry != nil {
		t.Error("expected nil entry for nil manager")
	}
}

func TestGetEntry_Isolation(t *testing.T) {
	m := NewHotColdManager()
	m.Put("key1", "value1")

	entry1, _, _ := m.GetEntry("key1")
	entry1.Value = "modified"
	entry1.AccessCount = 999

	entry2, _, _ := m.GetEntry("key1")
	if entry2.Value == "modified" {
		t.Error("GetEntry should return a copy, not the original")
	}
	if entry2.AccessCount == 999 {
		t.Error("GetEntry copy should not affect original access count")
	}
}

func TestGetAllHotKeys(t *testing.T) {
	m := newTestManager(t, Config{
		HotThreshold:    2.0,
		ColdThreshold:   1.0,
		DecayHalfLife:   time.Hour,
		ColdCheckCycles: 3,
	})

	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("hot_%d", i)
		m.Put(key, "value")
		for j := 0; j < 10; j++ {
			m.Get(key)
		}
	}

	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("cold_%d", i)
		m.Put(key, "value")
	}

	hotKeys := m.GetAllHotKeys()
	if len(hotKeys) != 5 {
		t.Errorf("expected 5 hot keys, got %d", len(hotKeys))
	}

	coldKeys := m.GetAllColdKeys()
	if len(coldKeys) != 3 {
		t.Errorf("expected 3 cold keys, got %d", len(coldKeys))
	}
}

func TestGetAllHotKeys_NilManager(t *testing.T) {
	var m *HotColdManager
	keys := m.GetAllHotKeys()
	if keys != nil {
		t.Errorf("expected nil for nil manager, got %v", keys)
	}
}

func TestGetAllColdKeys_NilManager(t *testing.T) {
	var m *HotColdManager
	keys := m.GetAllColdKeys()
	if keys != nil {
		t.Errorf("expected nil for nil manager, got %v", keys)
	}
}

func TestHotCount_NilManager(t *testing.T) {
	var m *HotColdManager
	count := m.HotCount()
	if count != 0 {
		t.Errorf("expected 0 for nil manager, got %d", count)
	}
}

func TestColdCount_NilManager(t *testing.T) {
	var m *HotColdManager
	count := m.ColdCount()
	if count != 0 {
		t.Errorf("expected 0 for nil manager, got %d", count)
	}
}

func TestTotalCount(t *testing.T) {
	m := NewHotColdManager()

	if m.TotalCount() != 0 {
		t.Errorf("expected 0 total, got %d", m.TotalCount())
	}

	m.Put("a", "1")
	m.Put("b", "2")

	if m.TotalCount() != 2 {
		t.Errorf("expected 2 total, got %d", m.TotalCount())
	}

	m.Delete("a")
	if m.TotalCount() != 1 {
		t.Errorf("expected 1 total after delete, got %d", m.TotalCount())
	}
}

func TestScoreDecayOverTime(t *testing.T) {
	cfg := Config{
		HotThreshold:         100.0,
		ColdThreshold:        0.1,
		DecayHalfLife:        time.Millisecond * 20,
		ColdCheckCycles:      10,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	for i := 0; i < 10; i++ {
		m.Get("key1")
	}

	initialScore, ok, err := m.GetScore("key1")
	if err != nil {
		t.Fatalf("GetScore failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if initialScore <= 0 {
		t.Fatal("expected positive initial score")
	}
	t.Logf("Initial score: %.2f", initialScore)

	time.Sleep(time.Millisecond * 30)

	afterDecayScore, ok, err := m.GetScore("key1")
	if err != nil {
		t.Fatalf("GetScore after decay failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key1 to still exist")
	}
	t.Logf("After decay score: %.2f", afterDecayScore)
	if afterDecayScore >= initialScore {
		t.Errorf("expected score to decay over time, initial=%.2f, after=%.2f", initialScore, afterDecayScore)
	}
}

func TestConcurrentPut(t *testing.T) {
	m := NewHotColdManager()

	var wg sync.WaitGroup
	numGoroutines := 20
	numOps := 100

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < numOps; i++ {
				key := fmt.Sprintf("g%d_k%d", id, i)
				err := m.Put(key, fmt.Sprintf("v%d_%d", id, i))
				if err != nil {
					t.Errorf("Put failed: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	expected := numGoroutines * numOps
	if m.TotalCount() != expected {
		t.Errorf("expected %d keys, got %d", expected, m.TotalCount())
	}
}

func TestConcurrentGet(t *testing.T) {
	m := NewHotColdManager()

	numKeys := 200
	for i := 0; i < numKeys; i++ {
		m.Put(fmt.Sprintf("key%d", i), fmt.Sprintf("val%d", i))
	}

	var wg sync.WaitGroup
	numGoroutines := 30

	for g := 0; g < numGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < numKeys; i++ {
				val, ok, err := m.Get(fmt.Sprintf("key%d", i))
				if err != nil {
					t.Errorf("Get failed: %v", err)
					return
				}
				if !ok {
					t.Errorf("key%d not found", i)
					return
				}
				expected := fmt.Sprintf("val%d", i)
				if val != expected {
					t.Errorf("key%d: expected %s, got %v", i, expected, val)
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestConcurrentPutAndGet(t *testing.T) {
	m := NewHotColdManager()

	numKeys := 100
	for i := 0; i < numKeys; i++ {
		m.Put(fmt.Sprintf("pkey%d", i), fmt.Sprintf("pval%d", i))
	}

	var wg sync.WaitGroup
	var getErrors int64

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			m.Put(fmt.Sprintf("pkey%d", i), fmt.Sprintf("pval_updated_%d", i))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < numKeys; i++ {
			val, ok, err := m.Get(fmt.Sprintf("pkey%d", i))
			if err != nil {
				atomic.AddInt64(&getErrors, 1)
				t.Errorf("Get error: %v", err)
				continue
			}
			if !ok {
				atomic.AddInt64(&getErrors, 1)
				t.Errorf("Get returned ok=false for pkey%d", i)
				continue
			}
			expectedOld := fmt.Sprintf("pval%d", i)
			expectedNew := fmt.Sprintf("pval_updated_%d", i)
			if val != expectedOld && val != expectedNew {
				atomic.AddInt64(&getErrors, 1)
				t.Errorf("Get returned unexpected value for pkey%d: got %v, expected %s or %s", i, val, expectedOld, expectedNew)
			}
		}
	}()

	wg.Wait()

	if getErrors > 0 {
		t.Errorf("found %d Get errors during concurrent Put/Get", getErrors)
	}
}

func TestConcurrentDelete(t *testing.T) {
	m := NewHotColdManager()

	numKeys := 200
	for i := 0; i < numKeys; i++ {
		m.Put(fmt.Sprintf("dkey%d", i), fmt.Sprintf("dval%d", i))
	}

	var wg sync.WaitGroup
	numDeleters := 10
	keysPerDeleter := numKeys / numDeleters

	for d := 0; d < numDeleters; d++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			start := id * keysPerDeleter
			end := start + keysPerDeleter
			for i := start; i < end; i++ {
				m.Delete(fmt.Sprintf("dkey%d", i))
			}
		}(d)
	}

	wg.Wait()

	if m.TotalCount() != 0 {
		t.Errorf("expected count 0 after all deletes, got %d", m.TotalCount())
	}
}

func TestConcurrentCheckAndMigrate(t *testing.T) {
	cfg := Config{
		HotThreshold:     5.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  2,
		HotCapacityRatio: 0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("ckey%d", i)
		m.Put(key, "value")
		for j := 0; j < i; j++ {
			m.Get(key)
		}
	}

	var wg sync.WaitGroup
	numMigrators := 10

	for g := 0; g < numMigrators; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				m.CheckAndMigrate()
			}
		}()
	}

	wg.Wait()

	total := m.TotalCount()
	hot := m.HotCount()
	cold := m.ColdCount()
	if hot+cold != total {
		t.Errorf("hot + cold != total: %d + %d != %d", hot, cold, total)
	}
}

func TestNewHotColdManagerWithConfig_AutoFixThresholdOrder(t *testing.T) {
	cfg := Config{
		HotThreshold:  1.0,
		ColdThreshold: 5.0,
		DecayHalfLife: time.Hour,
	}
	m, err := NewHotColdManagerWithConfig(cfg)
	if err != nil {
		t.Fatalf("expected no error when HotThreshold < ColdThreshold (auto-fix), got %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager after auto-fix")
	}
	gotCfg := m.GetConfig()
	if gotCfg.HotThreshold <= gotCfg.ColdThreshold {
		t.Errorf("expected HotThreshold > ColdThreshold after auto-fix, got hot=%.2f, cold=%.2f", gotCfg.HotThreshold, gotCfg.ColdThreshold)
	}
	if gotCfg.HotThreshold != 25.0 {
		t.Errorf("expected HotThreshold to be auto-fixed to ColdThreshold*5=25.0, got %.2f", gotCfg.HotThreshold)
	}
}

func TestNewHotColdManagerWithConfig_ZeroValues(t *testing.T) {
	cfg := Config{}
	m := newTestManager(t, cfg)
	gotCfg := m.GetConfig()

	if gotCfg.HotThreshold <= 0 {
		t.Errorf("expected positive HotThreshold, got %.2f", gotCfg.HotThreshold)
	}
	if gotCfg.ColdThreshold <= 0 {
		t.Errorf("expected positive ColdThreshold, got %.2f", gotCfg.ColdThreshold)
	}
	if gotCfg.DecayHalfLife <= 0 {
		t.Errorf("expected positive DecayHalfLife, got %v", gotCfg.DecayHalfLife)
	}
	if gotCfg.ColdCheckCycles <= 0 {
		t.Errorf("expected positive ColdCheckCycles, got %d", gotCfg.ColdCheckCycles)
	}
}

func TestErrors(t *testing.T) {
	if ErrKeyNotFound == nil {
		t.Error("ErrKeyNotFound should not be nil")
	}
	if ErrInvalidConfig == nil {
		t.Error("ErrInvalidConfig should not be nil")
	}
	if ErrNilManager == nil {
		t.Error("ErrNilManager should not be nil")
	}
	if ErrEmptyKey == nil {
		t.Error("ErrEmptyKey should not be nil")
	}
}

func TestDataTierValues(t *testing.T) {
	if TierCold != 0 {
		t.Errorf("expected TierCold=0, got %d", TierCold)
	}
	if TierHot != 1 {
		t.Errorf("expected TierHot=1, got %d", TierHot)
	}
}

func TestPut_PromoteDuringPut(t *testing.T) {
	cfg := Config{
		HotThreshold:     3.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	if m.HotCount() != 0 {
		t.Fatal("expected key1 to start cold")
	}

	for i := 0; i < 5; i++ {
		m.Put("key1", fmt.Sprintf("value%d", i))
	}

	if m.HotCount() != 1 {
		t.Errorf("expected key1 to be promoted to hot after multiple puts, got %d hot keys", m.HotCount())
	}
}

func TestGetScore_NilManager(t *testing.T) {
	var m *HotColdManager
	score, ok, err := m.GetScore("key")
	if err != ErrNilManager {
		t.Errorf("expected ErrNilManager, got %v", err)
	}
	if ok {
		t.Error("expected ok=false")
	}
	if score != 0 {
		t.Errorf("expected score 0, got %.2f", score)
	}
}

func TestGetConfig_NilManager(t *testing.T) {
	var m *HotColdManager
	cfg := m.GetConfig()
	if (cfg != Config{}) {
		t.Logf("nil manager GetConfig returns zero value Config")
	}
}

func TestTotalCount_NilManager(t *testing.T) {
	var m *HotColdManager
	count := m.TotalCount()
	if count != 0 {
		t.Errorf("expected 0 for nil manager, got %d", count)
	}
}

func TestDelete_EmptyKey(t *testing.T) {
	m := NewHotColdManager()
	result := m.Delete("")
	if result {
		t.Error("expected Delete to return false for empty key")
	}
}

func TestGetScore_EmptyKey(t *testing.T) {
	m := NewHotColdManager()
	score, ok, err := m.GetScore("")
	if err != ErrEmptyKey {
		t.Errorf("expected ErrEmptyKey, got %v", err)
	}
	if ok {
		t.Error("expected ok=false for empty key")
	}
	if score != 0 {
		t.Errorf("expected score 0, got %.2f", score)
	}
}

func TestCheckAndMigrate_EmptyManager(t *testing.T) {
	m := NewHotColdManager()
	migrated := m.CheckAndMigrate()
	if migrated != 0 {
		t.Errorf("expected 0 migrations for empty manager, got %d", migrated)
	}
}

func TestPromoteAndDemoteCycle(t *testing.T) {
	cfg := Config{
		HotThreshold:         3.0,
		ColdThreshold:        2.0,
		DecayHalfLife:        time.Millisecond * 5,
		ColdCheckCycles:      2,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("key1", "value1")
	for i := 0; i < 3; i++ {
		m.Get("key1")
	}
	if m.HotCount() != 1 {
		entry, _, _ := m.GetEntry("key1")
		t.Fatalf("expected key1 to be hot after initial accesses, score=%.2f, hotCount=%d", entry.Score, m.HotCount())
	}
	t.Log("Step 1: key1 is hot")

	for i := 0; i < 5; i++ {
		time.Sleep(time.Millisecond * 8)
		m.CheckAndMigrate()
	}
	if m.ColdCount() != 1 {
		entry, _, _ := m.GetEntry("key1")
		t.Fatalf("expected key1 to be cold after decay, tier=%v, score=%.2f, coldCycles=%d",
			entry.Tier, entry.Score, entry.ConsecutiveColdCycles)
	}
	t.Log("Step 2: key1 demoted to cold")

	for i := 0; i < 10; i++ {
		m.Get("key1")
	}
	if m.HotCount() != 1 {
		t.Errorf("expected key1 to be hot again after re-access, got %d hot", m.HotCount())
	}
	t.Log("Step 3: key1 promoted back to hot")
}

func TestMultipleHotKeys(t *testing.T) {
	cfg := Config{
		HotThreshold:     2.0,
		ColdThreshold:    0.5,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.5,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	numHot := 10
	numCold := 20

	for i := 0; i < numHot; i++ {
		key := fmt.Sprintf("hot_%d", i)
		m.Put(key, "hot_value")
		for j := 0; j < 10; j++ {
			m.Get(key)
		}
	}

	for i := 0; i < numCold; i++ {
		key := fmt.Sprintf("cold_%d", i)
		m.Put(key, "cold_value")
	}

	if m.HotCount() != numHot {
		t.Errorf("expected %d hot keys, got %d", numHot, m.HotCount())
	}
	if m.ColdCount() != numCold {
		t.Errorf("expected %d cold keys, got %d", numCold, m.ColdCount())
	}
	if m.TotalCount() != numHot+numCold {
		t.Errorf("expected %d total keys, got %d", numHot+numCold, m.TotalCount())
	}
}

func TestScore_NewEntryBoost(t *testing.T) {
	cfg := Config{
		HotThreshold:     10.0,
		ColdThreshold:    1.0,
		DecayHalfLife:    time.Hour,
		ColdCheckCycles:  3,
		HotCapacityRatio: 0.3,
	}
	m := newTestManager(t, cfg)

	m.Put("new_key", "value")

	entry, ok, _ := m.GetEntry("new_key")
	if !ok {
		t.Fatal("new_key should exist")
	}

	if entry.Score <= 0 {
		t.Error("new entry should have positive score")
	}

	baseScore := float64(entry.AccessCount)
	if entry.Score <= baseScore {
		t.Errorf("new entry should have boosted score, got %.2f (base %.2f)", entry.Score, baseScore)
	}
}

func TestGetScore_LiveScoreDecay(t *testing.T) {
	cfg := Config{
		HotThreshold:         100.0,
		ColdThreshold:        0.1,
		DecayHalfLife:        time.Millisecond * 15,
		ColdCheckCycles:      10,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("k", "v")
	for i := 0; i < 5; i++ {
		m.Get("k")
	}

	s1, ok, err := m.GetScore("k")
	if err != nil {
		t.Fatalf("GetScore 1 failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist")
	}

	time.Sleep(time.Millisecond * 20)

	s2, ok, err := m.GetScore("k")
	if err != nil {
		t.Fatalf("GetScore 2 failed: %v", err)
	}
	if !ok {
		t.Fatal("expected key to exist 2")
	}

	if s2 >= s1 {
		t.Errorf("expected live score to decay without calling CheckAndMigrate, s1=%.2f, s2=%.2f", s1, s2)
	}
	t.Logf("Live decay verified: s1=%.2f -> s2=%.2f", s1, s2)
}

func TestGetEntry_NonExistent(t *testing.T) {
	m := NewHotColdManager()
	entry, ok, err := m.GetEntry("noexist")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got %v", err)
	}
	if ok {
		t.Error("expected ok=false")
	}
	if entry != nil {
		t.Errorf("expected nil entry, got %v", entry)
	}
}

func TestValidateConfig_Valid(t *testing.T) {
	cfg := DefaultConfig()
	err := ValidateConfig(cfg)
	if err != nil {
		t.Errorf("expected ValidateConfig to pass for default config, got %v", err)
	}
}

func TestValidateConfig_InvalidCapacityRatio(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HotCapacityRatio = 1.5
	err := ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for bad capacity ratio, got %v", err)
	}

	cfg.HotCapacityRatio = -0.1
	err = ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative capacity ratio, got %v", err)
	}
}

func TestValidateConfig_InvalidThresholdOrder(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HotThreshold = 1.0
	cfg.ColdThreshold = 5.0
	err := ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig when hot <= cold, got %v", err)
	}
}

func TestValidateConfig_InvalidThresholds(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HotThreshold = 0
	err := ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for zero HotThreshold, got %v", err)
	}

	cfg = DefaultConfig()
	cfg.ColdThreshold = -1
	err = ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for negative ColdThreshold, got %v", err)
	}
}

func TestValidateConfig_InvalidMinMax(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinHotThreshold = 100
	cfg.MaxHotThreshold = 10
	err := ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig when MinHot > MaxHot, got %v", err)
	}
}

func TestValidateConfig_InvalidDurationsAndCycles(t *testing.T) {
	cfg := DefaultConfig()
	cfg.DecayHalfLife = 0
	err := ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for zero DecayHalfLife, got %v", err)
	}

	cfg = DefaultConfig()
	cfg.ColdCheckCycles = 0
	err = ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for zero ColdCheckCycles, got %v", err)
	}

	cfg = DefaultConfig()
	cfg.AdjustInterval = 0
	err = ValidateConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig for zero AdjustInterval, got %v", err)
	}
}

func TestNewHotColdManagerWithConfig_ReturnsError(t *testing.T) {
	cfg := DefaultConfig()
	cfg.HotCapacityRatio = 2.0
	m, err := NewHotColdManagerWithConfig(cfg)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("expected ErrInvalidConfig from constructor, got %v", err)
	}
	if m != nil {
		t.Error("expected nil manager when config is invalid")
	}
}

func TestErrors_IsClassification(t *testing.T) {
	m := NewHotColdManager()

	_, _, err := m.Get("")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey for empty key Get, got %v", err)
	}

	_, _, err = m.Get("missing")
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound for missing key Get, got %v", err)
	}

	err = m.Put("", "v")
	if !errors.Is(err, ErrEmptyKey) {
		t.Errorf("expected ErrEmptyKey for empty key Put, got %v", err)
	}

	var nilM *HotColdManager
	_, _, err = nilM.Get("k")
	if !errors.Is(err, ErrNilManager) {
		t.Errorf("expected ErrNilManager for nil manager, got %v", err)
	}
}

func TestGetEntry_LiveScore(t *testing.T) {
	cfg := Config{
		HotThreshold:         100.0,
		ColdThreshold:        0.1,
		DecayHalfLife:        time.Millisecond * 15,
		ColdCheckCycles:      10,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: false,
	}
	m := newTestManager(t, cfg)

	m.Put("k", "v")
	for i := 0; i < 5; i++ {
		m.Get("k")
	}

	e1, _, _ := m.GetEntry("k")
	time.Sleep(time.Millisecond * 20)
	e2, _, _ := m.GetEntry("k")

	if e2.Score >= e1.Score {
		t.Errorf("expected GetEntry to return live decaying score, e1.Score=%.2f, e2.Score=%.2f", e1.Score, e2.Score)
	}
}

func TestAutoAdjustThresholds_LoadFactorHigh(t *testing.T) {
	cfg := Config{
		HotThreshold:         50.0,
		ColdThreshold:        20.0,
		DecayHalfLife:        time.Hour,
		ColdCheckCycles:      3,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: true,
		AdjustInterval:       100 * time.Millisecond,
		MinHotThreshold:      1.0,
		MaxHotThreshold:      200.0,
		MinColdThreshold:     0.1,
		MaxColdThreshold:     100.0,
	}
	m := newTestManager(t, cfg)

	for i := 0; i < 100; i++ {
		m.Put(fmt.Sprintf("k%d", i), "v")
	}

	for i := 0; i < 2000; i++ {
		for j := 0; j < 100; j++ {
			m.Get(fmt.Sprintf("k%d", j))
		}
	}

	initialCfg := m.GetConfig()
	t.Logf("Before adjust: hotCount=%d, coldCount=%d, hotRatio=%.2f", m.HotCount(), m.ColdCount(), float64(m.HotCount())/float64(m.TotalCount()))

	time.Sleep(150 * time.Millisecond)
	m.CheckAndMigrate()

	afterCfg := m.GetConfig()
	t.Logf("High load - Before: hot=%.2f cold=%.2f", initialCfg.HotThreshold, initialCfg.ColdThreshold)
	t.Logf("High load - After:  hot=%.2f cold=%.2f", afterCfg.HotThreshold, afterCfg.ColdThreshold)

	if afterCfg.HotThreshold >= initialCfg.HotThreshold {
		t.Errorf("expected hot threshold to decrease under high load (capacity+load both push down), before=%.2f, after=%.2f", initialCfg.HotThreshold, afterCfg.HotThreshold)
	}
	if afterCfg.ColdThreshold >= initialCfg.ColdThreshold {
		t.Errorf("expected cold threshold to decrease under high load (capacity+load both push down), before=%.2f, after=%.2f", initialCfg.ColdThreshold, afterCfg.ColdThreshold)
	}
}

func TestAutoAdjustThresholds_LoadFactorLow(t *testing.T) {
	cfg := Config{
		HotThreshold:         1.0,
		ColdThreshold:        0.1,
		DecayHalfLife:        time.Hour,
		ColdCheckCycles:      3,
		HotCapacityRatio:     0.3,
		AutoAdjustThresholds: true,
		AdjustInterval:       100 * time.Millisecond,
		MinHotThreshold:      0.1,
		MaxHotThreshold:      200.0,
		MinColdThreshold:     0.01,
		MaxColdThreshold:     100.0,
	}
	m := newTestManager(t, cfg)

	for i := 0; i < 100; i++ {
		m.Put(fmt.Sprintf("k%d", i), "v")
		m.Get(fmt.Sprintf("k%d", i))
	}

	time.Sleep(150 * time.Millisecond)
	m.CheckAndMigrate()

	afterFirstCfg := m.GetConfig()

	time.Sleep(150 * time.Millisecond)
	m.CheckAndMigrate()

	afterSecondCfg := m.GetConfig()
	t.Logf("Low load - After 1st adjust: hot=%.2f cold=%.2f", afterFirstCfg.HotThreshold, afterFirstCfg.ColdThreshold)
	t.Logf("Low load - After 2nd adjust: hot=%.2f cold=%.2f", afterSecondCfg.HotThreshold, afterSecondCfg.ColdThreshold)

	if afterSecondCfg.HotThreshold <= afterFirstCfg.HotThreshold {
		t.Errorf("expected hot threshold to increase under low load (capacity+load both push up), 1st=%.2f, 2nd=%.2f", afterFirstCfg.HotThreshold, afterSecondCfg.HotThreshold)
	}
	if afterSecondCfg.ColdThreshold <= afterFirstCfg.ColdThreshold {
		t.Errorf("expected cold threshold to increase under low load (capacity+load both push up), 1st=%.2f, 2nd=%.2f", afterFirstCfg.ColdThreshold, afterSecondCfg.ColdThreshold)
	}
}

func TestTotalAccesses_Accumulates(t *testing.T) {
	m := NewHotColdManager()

	m.Put("a", 1)
	m.Put("b", 2)
	m.Get("a")
	m.Get("b")
	m.Get("a")

	expectedOps := int64(5)
	if m.totalAccesses != expectedOps {
		t.Errorf("expected totalAccesses=%d, got %d", expectedOps, m.totalAccesses)
	}
}
