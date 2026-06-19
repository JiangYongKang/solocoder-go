package quotamgr

import (
	"fmt"
	"sync"
)


type ResourceType int

const (
	ResourceCPU ResourceType = iota
	ResourceMemory
	ResourceConcurrency
)

func (r ResourceType) String() string {
	switch r {
	case ResourceCPU:
		return "CPU"
	case ResourceMemory:
		return "Memory"
	case ResourceConcurrency:
		return "Concurrency"
	default:
		return "Unknown"
	}
}

type LimitMode int

const (
	LimitModeHard LimitMode = iota
	LimitModeSoft
)

func (m LimitMode) String() string {
	switch m {
	case LimitModeHard:
		return "Hard"
	case LimitModeSoft:
		return "Soft"
	default:
		return "Unknown"
	}
}

type Quota struct {
	CPU         float64
	MemoryMB    int64
	Concurrency int64
}

type Usage struct {
	CPU         float64
	MemoryMB    int64
	Concurrency int64
}

type TenantQuota struct {
	Quota     Quota
	LimitMode LimitMode
	SoftThreshold float64
}

type TenantUsage struct {
	mu    sync.RWMutex
	usage Usage
}

type ResourceUsageInfo struct {
	ResourceType ResourceType
	Used         float64
	Limit        float64
	Remaining    float64
	UsagePercent float64
}

type TenantQuotaInfo struct {
	TenantID string
	Quota    Quota
	Usage    Usage
	LimitMode LimitMode
	Resources []ResourceUsageInfo
}

type AlertCallback func(tenantID string, resource ResourceType, used, limit float64)

type QuotaExceededError struct {
	TenantID     string
	ResourceType ResourceType
	Used         float64
	Limit        float64
	LimitMode    LimitMode
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf(
		"quotamgr: tenant '%s' %s quota exceeded: used=%.2f, limit=%.2f, mode=%s",
		e.TenantID, e.ResourceType, e.Used, e.Limit, e.LimitMode,
	)
}


