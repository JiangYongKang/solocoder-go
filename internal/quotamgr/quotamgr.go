package quotamgr

import (
	"math"
	"sync"
)

type Manager struct {
	mu           sync.RWMutex
	config       *Config
	tenantQuotas map[string]*TenantQuota
	tenantUsages map[string]*TenantUsage
}

func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.SoftThreshold < 1.0 || cfg.SoftThreshold > 2.0 {
		cfg.SoftThreshold = 1.5
	}
	return &Manager{
		config:       cfg,
		tenantQuotas: make(map[string]*TenantQuota),
		tenantUsages: make(map[string]*TenantUsage),
	}
}

func (m *Manager) getOrCreateTenantUsage(tenantID string) *TenantUsage {
	m.mu.Lock()
	defer m.mu.Unlock()

	usage, exists := m.tenantUsages[tenantID]
	if !exists {
		usage = &TenantUsage{}
		m.tenantUsages[tenantID] = usage
	}
	return usage
}

func (m *Manager) getTenantQuotaLocked(tenantID string) *TenantQuota {
	quota, exists := m.tenantQuotas[tenantID]
	if !exists {
		return &TenantQuota{
			Quota:         m.config.DefaultQuota,
			LimitMode:     m.config.DefaultLimitMode,
			SoftThreshold: m.config.SoftThreshold,
		}
	}
	return quota
}

func (m *Manager) SetTenantQuota(tenantID string, quota TenantQuota) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if quota.Quota.CPU < 0 || quota.Quota.MemoryMB < 0 || quota.Quota.Concurrency < 0 {
		return ErrInvalidQuota
	}
	if quota.LimitMode == LimitModeSoft {
		if quota.SoftThreshold < 1.0 || quota.SoftThreshold > 2.0 {
			return ErrInvalidSoftThreshold
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.tenantQuotas[tenantID] = &TenantQuota{
		Quota:         quota.Quota,
		LimitMode:     quota.LimitMode,
		SoftThreshold: quota.SoftThreshold,
	}
	return nil
}

func (m *Manager) GetTenantQuota(tenantID string) (TenantQuota, error) {
	if tenantID == "" {
		return TenantQuota{}, ErrInvalidTenantID
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.tenantQuotas[tenantID]
	if !exists {
		return TenantQuota{
			Quota:         m.config.DefaultQuota,
			LimitMode:     m.config.DefaultLimitMode,
			SoftThreshold: m.config.SoftThreshold,
		}, nil
	}
	return *quota, nil
}

func (m *Manager) getResourceLimit(quota *TenantQuota, resource ResourceType) float64 {
	switch resource {
	case ResourceCPU:
		return quota.Quota.CPU
	case ResourceMemory:
		return float64(quota.Quota.MemoryMB)
	case ResourceConcurrency:
		return float64(quota.Quota.Concurrency)
	default:
		return 0
	}
}

func (m *Manager) getResourceUsage(usage *Usage, resource ResourceType) float64 {
	switch resource {
	case ResourceCPU:
		return usage.CPU
	case ResourceMemory:
		return float64(usage.MemoryMB)
	case ResourceConcurrency:
		return float64(usage.Concurrency)
	default:
		return 0
	}
}

func (m *Manager) addResourceUsage(usage *Usage, resource ResourceType, amount float64) {
	switch resource {
	case ResourceCPU:
		usage.CPU += amount
	case ResourceMemory:
		usage.MemoryMB += int64(amount)
	case ResourceConcurrency:
		usage.Concurrency += int64(amount)
	}
}

func (m *Manager) subResourceUsage(usage *Usage, resource ResourceType, amount float64) {
	switch resource {
	case ResourceCPU:
		usage.CPU = math.Max(0, usage.CPU-amount)
	case ResourceMemory:
		usage.MemoryMB = int64(math.Max(0, float64(usage.MemoryMB)-amount))
	case ResourceConcurrency:
		usage.Concurrency = int64(math.Max(0, float64(usage.Concurrency)-amount))
	}
}

func (m *Manager) AcquireResource(tenantID string, resource ResourceType, amount float64) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if resource < ResourceCPU || resource > ResourceConcurrency {
		return ErrInvalidResourceType
	}

	tenantUsage := m.getOrCreateTenantUsage(tenantID)

	m.mu.RLock()
	quota := m.getTenantQuotaLocked(tenantID)
	m.mu.RUnlock()

	limit := m.getResourceLimit(quota, resource)
	softLimit := limit * quota.SoftThreshold

	tenantUsage.mu.Lock()
	defer tenantUsage.mu.Unlock()

	currentUsage := m.getResourceUsage(&tenantUsage.usage, resource)
	newUsage := currentUsage + amount

	if quota.LimitMode == LimitModeHard {
		if newUsage > limit {
			return &QuotaExceededError{
				TenantID:     tenantID,
				ResourceType: resource,
				Used:         newUsage,
				Limit:        limit,
				LimitMode:    LimitModeHard,
			}
		}
	} else {
		if currentUsage <= limit && newUsage > limit {
			if m.config.AlertCallback != nil {
				m.config.AlertCallback(tenantID, resource, newUsage, limit)
			}
		}
		if newUsage > softLimit {
			return &QuotaExceededError{
				TenantID:     tenantID,
				ResourceType: resource,
				Used:         newUsage,
				Limit:        softLimit,
				LimitMode:    LimitModeSoft,
			}
		}
	}

	m.addResourceUsage(&tenantUsage.usage, resource, amount)
	return nil
}

func (m *Manager) ReleaseResource(tenantID string, resource ResourceType, amount float64) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if amount <= 0 {
		return ErrInvalidAmount
	}
	if resource < ResourceCPU || resource > ResourceConcurrency {
		return ErrInvalidResourceType
	}

	m.mu.RLock()
	_, exists := m.tenantUsages[tenantID]
	m.mu.RUnlock()

	if !exists {
		return ErrTenantNotFound
	}

	tenantUsage := m.getOrCreateTenantUsage(tenantID)

	tenantUsage.mu.Lock()
	defer tenantUsage.mu.Unlock()

	currentUsage := m.getResourceUsage(&tenantUsage.usage, resource)
	if amount > currentUsage {
		return ErrReleaseTooLarge
	}

	m.subResourceUsage(&tenantUsage.usage, resource, amount)
	return nil
}

func (m *Manager) GetTenantUsage(tenantID string) (*TenantQuotaInfo, error) {
	if tenantID == "" {
		return nil, ErrInvalidTenantID
	}

	m.mu.RLock()
	quota := m.getTenantQuotaLocked(tenantID)
	usage, exists := m.tenantUsages[tenantID]
	m.mu.RUnlock()

	var currentUsage Usage
	if exists {
		usage.mu.RLock()
		currentUsage = usage.usage
		usage.mu.RUnlock()
	}

	resources := make([]ResourceUsageInfo, 3)
	resourceTypes := []ResourceType{ResourceCPU, ResourceMemory, ResourceConcurrency}

	for i, rt := range resourceTypes {
		used := m.getResourceUsage(&currentUsage, rt)
		limit := m.getResourceLimit(quota, rt)
		remaining := math.Max(0, limit-used)
		usagePercent := 0.0
		if limit > 0 {
			usagePercent = (used / limit) * 100
		}
		resources[i] = ResourceUsageInfo{
			ResourceType: rt,
			Used:         used,
			Limit:        limit,
			Remaining:    remaining,
			UsagePercent: usagePercent,
		}
	}

	return &TenantQuotaInfo{
		TenantID:  tenantID,
		Quota:     quota.Quota,
		Usage:     currentUsage,
		LimitMode: quota.LimitMode,
		Resources: resources,
	}, nil
}

func (m *Manager) GetAllTenantsUsage() []*TenantQuotaInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantIDs := make([]string, 0, len(m.tenantUsages))
	for id := range m.tenantUsages {
		tenantIDs = append(tenantIDs, id)
	}
	for id := range m.tenantQuotas {
		if _, ok := m.tenantUsages[id]; !ok {
			tenantIDs = append(tenantIDs, id)
		}
	}

	result := make([]*TenantQuotaInfo, 0, len(tenantIDs))
	for _, id := range tenantIDs {
		info, _ := m.GetTenantUsage(id)
		if info != nil {
			result = append(result, info)
		}
	}

	return result
}

func (m *Manager) AdjustQuota(tenantID string, resource ResourceType, newLimit float64) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if newLimit < 0 {
		return ErrInvalidQuota
	}
	if resource < ResourceCPU || resource > ResourceConcurrency {
		return ErrInvalidResourceType
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.tenantQuotas[tenantID]
	if !exists {
		quota = &TenantQuota{
			Quota:         m.config.DefaultQuota,
			LimitMode:     m.config.DefaultLimitMode,
			SoftThreshold: m.config.SoftThreshold,
		}
		m.tenantQuotas[tenantID] = quota
	}

	switch resource {
	case ResourceCPU:
		quota.Quota.CPU = newLimit
	case ResourceMemory:
		quota.Quota.MemoryMB = int64(newLimit)
	case ResourceConcurrency:
		quota.Quota.Concurrency = int64(newLimit)
	}

	return nil
}

func (m *Manager) SetLimitMode(tenantID string, mode LimitMode) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if mode != LimitModeHard && mode != LimitModeSoft {
		return ErrInvalidSoftThreshold
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.tenantQuotas[tenantID]
	if !exists {
		quota = &TenantQuota{
			Quota:         m.config.DefaultQuota,
			LimitMode:     mode,
			SoftThreshold: m.config.SoftThreshold,
		}
		m.tenantQuotas[tenantID] = quota
	} else {
		quota.LimitMode = mode
	}

	return nil
}

func (m *Manager) SetSoftThreshold(tenantID string, threshold float64) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}
	if threshold < 1.0 || threshold > 2.0 {
		return ErrInvalidSoftThreshold
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.tenantQuotas[tenantID]
	if !exists {
		quota = &TenantQuota{
			Quota:         m.config.DefaultQuota,
			LimitMode:     m.config.DefaultLimitMode,
			SoftThreshold: threshold,
		}
		m.tenantQuotas[tenantID] = quota
	} else {
		quota.SoftThreshold = threshold
	}

	return nil
}

func (m *Manager) RemoveTenant(tenantID string) error {
	if tenantID == "" {
		return ErrInvalidTenantID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tenantQuotas, tenantID)
	delete(m.tenantUsages, tenantID)
	return nil
}

func (m *Manager) TenantIDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenantSet := make(map[string]struct{})
	for id := range m.tenantQuotas {
		tenantSet[id] = struct{}{}
	}
	for id := range m.tenantUsages {
		tenantSet[id] = struct{}{}
	}

	ids := make([]string, 0, len(tenantSet))
	for id := range tenantSet {
		ids = append(ids, id)
	}
	return ids
}
