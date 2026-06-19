package quotamgr

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewManager_DefaultConfig(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	quota, err := mgr.GetTenantQuota("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if quota.Quota.CPU != 4.0 {
		t.Errorf("expected default CPU quota 4.0, got %.2f", quota.Quota.CPU)
	}
	if quota.Quota.MemoryMB != 2048 {
		t.Errorf("expected default MemoryMB quota 2048, got %d", quota.Quota.MemoryMB)
	}
	if quota.Quota.Concurrency != 100 {
		t.Errorf("expected default Concurrency quota 100, got %d", quota.Quota.Concurrency)
	}
	if quota.LimitMode != LimitModeHard {
		t.Errorf("expected default LimitMode Hard, got %v", quota.LimitMode)
	}
}

func TestNewManager_CustomConfig(t *testing.T) {
	cfg := &Config{
		DefaultQuota: Quota{
			CPU:         8.0,
			MemoryMB:    4096,
			Concurrency: 200,
		},
		DefaultLimitMode: LimitModeSoft,
		SoftThreshold:    1.3,
	}
	mgr := NewManager(cfg)

	quota, _ := mgr.GetTenantQuota("tenant1")
	if quota.Quota.CPU != 8.0 {
		t.Errorf("expected custom CPU quota 8.0, got %.2f", quota.Quota.CPU)
	}
	if quota.LimitMode != LimitModeSoft {
		t.Errorf("expected custom LimitMode Soft, got %v", quota.LimitMode)
	}
}

func TestSetTenantQuota_InvalidTenantID(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("", TenantQuota{
		Quota: Quota{CPU: 4.0},
	})
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestSetTenantQuota_InvalidQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota: Quota{CPU: -1.0},
	})
	if !errors.Is(err, ErrInvalidQuota) {
		t.Errorf("expected ErrInvalidQuota, got %v", err)
	}
}

func TestSetTenantQuota_InvalidSoftThreshold(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 0.5,
	})
	if !errors.Is(err, ErrInvalidSoftThreshold) {
		t.Errorf("expected ErrInvalidSoftThreshold, got %v", err)
	}

	err = mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 2.5,
	})
	if !errors.Is(err, ErrInvalidSoftThreshold) {
		t.Errorf("expected ErrInvalidSoftThreshold, got %v", err)
	}
}

func TestSetTenantQuota_Success(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota: Quota{
			CPU:         8.0,
			MemoryMB:    8192,
			Concurrency: 50,
		},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 1.2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quota, err := mgr.GetTenantQuota("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quota.Quota.CPU != 8.0 {
		t.Errorf("expected CPU quota 8.0, got %.2f", quota.Quota.CPU)
	}
	if quota.LimitMode != LimitModeSoft {
		t.Errorf("expected LimitMode Soft, got %v", quota.LimitMode)
	}
}

func TestAcquireResource_HardLimit_WithinQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := mgr.GetTenantUsage("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Usage.CPU != 2.0 {
		t.Errorf("expected CPU usage 2.0, got %.2f", info.Usage.CPU)
	}
}

func TestAcquireResource_HardLimit_ExceedQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 3.0)
	if err == nil {
		t.Fatal("expected error when exceeding quota")
	}

	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Errorf("expected QuotaExceededError, got %v", err)
	}
	if quotaErr.LimitMode != LimitModeHard {
		t.Errorf("expected LimitMode Hard, got %v", quotaErr.LimitMode)
	}
}

func TestAcquireResource_SoftLimit_WithinQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 1.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 2.0 {
		t.Errorf("expected CPU usage 2.0, got %.2f", info.Usage.CPU)
	}
}

func TestAcquireResource_SoftLimit_ExceedQuotaButBelowSoftThreshold(t *testing.T) {
	alertCalled := int32(0)
	cfg := DefaultConfig()
	cfg.AlertCallback = func(tenantID string, resource ResourceType, used, limit float64) {
		atomic.AddInt32(&alertCalled, 1)
	}
	mgr := NewManager(cfg)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 1.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 3.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("expected no error when exceeding quota but below soft threshold, got %v", err)
	}

	time.Sleep(10 * time.Millisecond)
	if atomic.LoadInt32(&alertCalled) != 1 {
		t.Errorf("expected alert callback to be called once, got %d", atomic.LoadInt32(&alertCalled))
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 5.0 {
		t.Errorf("expected CPU usage 5.0, got %.2f", info.Usage.CPU)
	}
}

func TestAcquireResource_SoftLimit_ExceedSoftThreshold(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeSoft,
		SoftThreshold: 1.5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 5.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err == nil {
		t.Fatal("expected error when exceeding soft threshold")
	}

	var quotaErr *QuotaExceededError
	if !errors.As(err, &quotaErr) {
		t.Errorf("expected QuotaExceededError, got %v", err)
	}
	if quotaErr.LimitMode != LimitModeSoft {
		t.Errorf("expected LimitMode Soft, got %v", quotaErr.LimitMode)
	}
}

func TestAcquireResource_InvalidParams(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name      string
		tenantID  string
		resource  ResourceType
		amount    float64
		expectedErr error
	}{
		{"empty tenant", "", ResourceCPU, 1.0, ErrInvalidTenantID},
		{"invalid resource", "tenant1", ResourceType(999), 1.0, ErrInvalidResourceType},
		{"zero amount", "tenant1", ResourceCPU, 0, ErrInvalidAmount},
		{"negative amount", "tenant1", ResourceCPU, -1.0, ErrInvalidAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.AcquireResource(tt.tenantID, tt.resource, tt.amount)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestReleaseResource_Success(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.AcquireResource("tenant1", ResourceCPU, 3.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.ReleaseResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 1.0 {
		t.Errorf("expected CPU usage 1.0 after release, got %.2f", info.Usage.CPU)
	}
}

func TestReleaseResource_ReleaseTooLarge(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.ReleaseResource("tenant1", ResourceCPU, 3.0)
	if !errors.Is(err, ErrReleaseTooLarge) {
		t.Errorf("expected ErrReleaseTooLarge, got %v", err)
	}
}

func TestReleaseResource_TenantNotFound(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.ReleaseResource("nonexistent", ResourceCPU, 1.0)
	if !errors.Is(err, ErrTenantNotFound) {
		t.Errorf("expected ErrTenantNotFound, got %v", err)
	}
}

func TestReleaseResource_InvalidParams(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.AcquireResource("tenant1", ResourceCPU, 1.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		tenantID  string
		resource  ResourceType
		amount    float64
		expectedErr error
	}{
		{"empty tenant", "", ResourceCPU, 1.0, ErrInvalidTenantID},
		{"invalid resource", "tenant1", ResourceType(999), 1.0, ErrInvalidResourceType},
		{"zero amount", "tenant1", ResourceCPU, 0, ErrInvalidAmount},
		{"negative amount", "tenant1", ResourceCPU, -1.0, ErrInvalidAmount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ReleaseResource(tt.tenantID, tt.resource, tt.amount)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestGetTenantUsage_ResourceCalculation(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota: Quota{
			CPU:         8.0,
			MemoryMB:    4096,
			Concurrency: 100,
		},
		LimitMode: LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 4.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = mgr.AcquireResource("tenant1", ResourceMemory, 1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = mgr.AcquireResource("tenant1", ResourceConcurrency, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := mgr.GetTenantUsage("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Usage.CPU != 4.0 {
		t.Errorf("expected CPU usage 4.0, got %.2f", info.Usage.CPU)
	}
	if info.Usage.MemoryMB != 1024 {
		t.Errorf("expected MemoryMB usage 1024, got %d", info.Usage.MemoryMB)
	}
	if info.Usage.Concurrency != 50 {
		t.Errorf("expected Concurrency usage 50, got %d", info.Usage.Concurrency)
	}

	for _, r := range info.Resources {
		switch r.ResourceType {
		case ResourceCPU:
			if r.Used != 4.0 || r.Limit != 8.0 || r.Remaining != 4.0 {
				t.Errorf("CPU resource info incorrect: used=%.2f, limit=%.2f, remaining=%.2f", r.Used, r.Limit, r.Remaining)
			}
			if r.UsagePercent != 50.0 {
				t.Errorf("expected CPU usage percent 50.0, got %.2f", r.UsagePercent)
			}
		case ResourceMemory:
			if r.Used != 1024 || r.Limit != 4096 || r.Remaining != 3072 {
				t.Errorf("Memory resource info incorrect: used=%.2f, limit=%.2f, remaining=%.2f", r.Used, r.Limit, r.Remaining)
			}
			if r.UsagePercent != 25.0 {
				t.Errorf("expected Memory usage percent 25.0, got %.2f", r.UsagePercent)
			}
		case ResourceConcurrency:
			if r.Used != 50 || r.Limit != 100 || r.Remaining != 50 {
				t.Errorf("Concurrency resource info incorrect: used=%.2f, limit=%.2f, remaining=%.2f", r.Used, r.Limit, r.Remaining)
			}
			if r.UsagePercent != 50.0 {
				t.Errorf("expected Concurrency usage percent 50.0, got %.2f", r.UsagePercent)
			}
		}
	}
}

func TestGetTenantUsage_ZeroUsage(t *testing.T) {
	mgr := NewManager(nil)

	info, err := mgr.GetTenantUsage("new_tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Usage.CPU != 0 {
		t.Errorf("expected zero CPU usage, got %.2f", info.Usage.CPU)
	}
	if info.Usage.MemoryMB != 0 {
		t.Errorf("expected zero MemoryMB usage, got %d", info.Usage.MemoryMB)
	}
	if info.Usage.Concurrency != 0 {
		t.Errorf("expected zero Concurrency usage, got %d", info.Usage.Concurrency)
	}
}

func TestAdjustQuota_IncreaseQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 3.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AdjustQuota("tenant1", ResourceCPU, 8.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 4.0)
	if err != nil {
		t.Fatalf("unexpected error after increasing quota: %v", err)
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 7.0 {
		t.Errorf("expected CPU usage 7.0, got %.2f", info.Usage.CPU)
	}
}

func TestAdjustQuota_DecreaseQuota(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 8.0},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 6.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AdjustQuota("tenant1", ResourceCPU, 4.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 1.0)
	if err == nil {
		t.Fatal("expected error when acquiring after quota decreased below usage")
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 6.0 {
		t.Errorf("expected CPU usage remains 6.0, got %.2f", info.Usage.CPU)
	}

	err = mgr.ReleaseResource("tenant1", ResourceCPU, 3.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 1.0)
	if err != nil {
		t.Fatalf("unexpected error after releasing: %v", err)
	}

	info, _ = mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 4.0 {
		t.Errorf("expected CPU usage 4.0, got %.2f", info.Usage.CPU)
	}
}

func TestAdjustQuota_InvalidParams(t *testing.T) {
	mgr := NewManager(nil)

	tests := []struct {
		name      string
		tenantID  string
		resource  ResourceType
		newLimit  float64
		expectedErr error
	}{
		{"empty tenant", "", ResourceCPU, 8.0, ErrInvalidTenantID},
		{"invalid resource", "tenant1", ResourceType(999), 8.0, ErrInvalidResourceType},
		{"negative limit", "tenant1", ResourceCPU, -1.0, ErrInvalidQuota},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.AdjustQuota(tt.tenantID, tt.resource, tt.newLimit)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestSetLimitMode_Success(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetLimitMode("tenant1", LimitModeSoft)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quota, _ := mgr.GetTenantQuota("tenant1")
	if quota.LimitMode != LimitModeSoft {
		t.Errorf("expected LimitMode Soft, got %v", quota.LimitMode)
	}
}

func TestSetSoftThreshold_Success(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetSoftThreshold("tenant1", 1.8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quota, _ := mgr.GetTenantQuota("tenant1")
	if quota.SoftThreshold != 1.8 {
		t.Errorf("expected SoftThreshold 1.8, got %.2f", quota.SoftThreshold)
	}
}

func TestSetSoftThreshold_Invalid(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetSoftThreshold("tenant1", 0.5)
	if !errors.Is(err, ErrInvalidSoftThreshold) {
		t.Errorf("expected ErrInvalidSoftThreshold, got %v", err)
	}
}

func TestRemoveTenant(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{CPU: 4.0},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.RemoveTenant("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	quota, err := mgr.GetTenantQuota("tenant1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if quota.Quota.CPU != 4.0 {
		t.Errorf("expected default CPU quota after remove, got %.2f", quota.Quota.CPU)
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 0 {
		t.Errorf("expected zero usage after remove, got %.2f", info.Usage.CPU)
	}
}

func TestTenantIDs(t *testing.T) {
	mgr := NewManager(nil)

	mgr.SetTenantQuota("tenant1", TenantQuota{Quota: Quota{CPU: 4.0}})
	mgr.SetTenantQuota("tenant2", TenantQuota{Quota: Quota{CPU: 8.0}})
	mgr.AcquireResource("tenant3", ResourceCPU, 1.0)

	ids := mgr.TenantIDs()
	if len(ids) != 3 {
		t.Errorf("expected 3 tenant IDs, got %d", len(ids))
	}
}

func TestGetAllTenantsUsage(t *testing.T) {
	mgr := NewManager(nil)

	mgr.SetTenantQuota("tenant1", TenantQuota{Quota: Quota{CPU: 4.0}})
	mgr.SetTenantQuota("tenant2", TenantQuota{Quota: Quota{CPU: 8.0}})
	mgr.AcquireResource("tenant1", ResourceCPU, 2.0)

	infos := mgr.GetAllTenantsUsage()
	if len(infos) != 2 {
		t.Errorf("expected 2 tenant infos, got %d", len(infos))
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr := NewManager(nil)
	mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{Concurrency: 100},
		LimitMode:     LimitModeHard,
	})

	var wg sync.WaitGroup
	workers := 50
	operations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				_ = mgr.AcquireResource("tenant1", ResourceConcurrency, 1)
			}
		}()
	}

	wg.Wait()

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.Concurrency != 100 {
		t.Errorf("expected Concurrency usage 100, got %d", info.Usage.Concurrency)
	}
}

func TestConcurrentAcquireAndRelease(t *testing.T) {
	mgr := NewManager(nil)
	mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{Concurrency: 50},
		LimitMode:     LimitModeHard,
	})

	var wg sync.WaitGroup
	workers := 20
	operations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < operations; j++ {
				err := mgr.AcquireResource("tenant1", ResourceConcurrency, 1)
				if err == nil {
					time.Sleep(time.Microsecond)
					_ = mgr.ReleaseResource("tenant1", ResourceConcurrency, 1)
				}
			}
		}()
	}

	wg.Wait()

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.Concurrency < 0 || info.Usage.Concurrency > 50 {
		t.Errorf("expected Concurrency usage between 0 and 50, got %d", info.Usage.Concurrency)
	}
}

func TestResourceType_String(t *testing.T) {
	tests := []struct {
		resource ResourceType
		expected string
	}{
		{ResourceCPU, "CPU"},
		{ResourceMemory, "Memory"},
		{ResourceConcurrency, "Concurrency"},
		{ResourceType(999), "Unknown"},
	}

	for _, tt := range tests {
		if tt.resource.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.resource.String())
		}
	}
}

func TestLimitMode_String(t *testing.T) {
	tests := []struct {
		mode     LimitMode
		expected string
	}{
		{LimitModeHard, "Hard"},
		{LimitModeSoft, "Soft"},
		{LimitMode(999), "Unknown"},
	}

	for _, tt := range tests {
		if tt.mode.String() != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.mode.String())
		}
	}
}

func TestQuotaExceededError_Error(t *testing.T) {
	err := &QuotaExceededError{
		TenantID:     "test",
		ResourceType: ResourceCPU,
		Used:         5.0,
		Limit:        4.0,
		LimitMode:    LimitModeHard,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestGetTenantQuota_InvalidTenant(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetTenantQuota("")
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestGetTenantUsage_InvalidTenant(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetTenantUsage("")
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestRemoveTenant_InvalidTenant(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.RemoveTenant("")
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestSetLimitMode_InvalidTenant(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetLimitMode("", LimitModeHard)
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestSetSoftThreshold_InvalidTenant(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetSoftThreshold("", 1.5)
	if !errors.Is(err, ErrInvalidTenantID) {
		t.Errorf("expected ErrInvalidTenantID, got %v", err)
	}
}

func TestMemoryResource(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{MemoryMB: 1024},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.AcquireResource("tenant1", ResourceMemory, 512)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.MemoryMB != 512 {
		t.Errorf("expected MemoryMB usage 512, got %d", info.Usage.MemoryMB)
	}
}

func TestConcurrencyResource(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.SetTenantQuota("tenant1", TenantQuota{
		Quota:         Quota{Concurrency: 10},
		LimitMode:     LimitModeHard,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 0; i < 10; i++ {
		err = mgr.AcquireResource("tenant1", ResourceConcurrency, 1)
		if err != nil {
			t.Fatalf("unexpected error at iteration %d: %v", i, err)
		}
	}

	err = mgr.AcquireResource("tenant1", ResourceConcurrency, 1)
	if err == nil {
		t.Fatal("expected error when exceeding concurrency quota")
	}
}

func TestReleaseBelowZero(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.AcquireResource("tenant1", ResourceCPU, 2.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = mgr.ReleaseResource("tenant1", ResourceCPU, 5.0)
	if err == nil {
		t.Fatal("expected error when releasing more than used")
	}

	info, _ := mgr.GetTenantUsage("tenant1")
	if info.Usage.CPU != 2.0 {
		t.Errorf("expected CPU usage remains 2.0, got %.2f", info.Usage.CPU)
	}
}

func TestMultipleTenants(t *testing.T) {
	mgr := NewManager(nil)

	mgr.SetTenantQuota("tenant1", TenantQuota{Quota: Quota{CPU: 4.0}})
	mgr.SetTenantQuota("tenant2", TenantQuota{Quota: Quota{CPU: 8.0}})

	err := mgr.AcquireResource("tenant1", ResourceCPU, 3.0)
	if err != nil {
		t.Fatalf("unexpected error for tenant1: %v", err)
	}

	err = mgr.AcquireResource("tenant2", ResourceCPU, 5.0)
	if err != nil {
		t.Fatalf("unexpected error for tenant2: %v", err)
	}

	info1, _ := mgr.GetTenantUsage("tenant1")
	info2, _ := mgr.GetTenantUsage("tenant2")

	if info1.Usage.CPU != 3.0 {
		t.Errorf("tenant1 CPU usage should be 3.0, got %.2f", info1.Usage.CPU)
	}
	if info2.Usage.CPU != 5.0 {
		t.Errorf("tenant2 CPU usage should be 5.0, got %.2f", info2.Usage.CPU)
	}
}
