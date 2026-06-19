# 资源配额管理器 (quotamgr)

## 1. 模块概述

资源配额管理器（Quota Manager）是一个内存级别的租户资源配额管理模块，用于在多租户环境下对各个租户的资源使用进行精细化控制和监控。模块支持 CPU、内存、并发数三类资源的配额管理，提供软硬两种限制策略，并支持运行时动态调整配额。

## 2. 核心功能

### 2.1 按租户设置资源配额

支持为每个租户独立配置三类资源的配额上限：
- **CPU**：以核心数为单位（如 4.0 表示 4 个 CPU 核心）
- **内存**：以 MB 为单位（如 2048 表示 2GB 内存）
- **并发数**：以请求数为单位（如 100 表示最多 100 个并发请求）

未配置配额的租户使用系统默认配额值。

### 2.2 配额使用量实时统计

每次租户发起资源使用请求时记录当前使用量并与配额上限做比对，提供查询接口返回：
- 各租户当前各类资源的使用量
- 剩余可用量
- 使用率百分比

统计信息通过读写锁保证并发安全。

### 2.3 超限软硬限制策略

#### 硬限制模式 (LimitModeHard)
当租户资源使用量达到配额上限时，直接拒绝后续请求并返回配额不足的错误。

#### 软限制模式 (LimitModeSoft)
- 允许在一定范围内超过配额上限
- 当使用量首次超过配额上限时触发告警通知
- 当使用量超过配额上限的一定比例（软阈值，默认 1.5 倍）后，降级为硬限制模式，拒绝后续请求

两种模式可按租户粒度独立配置。

### 2.4 配额动态调整

支持在运行时动态调整任意租户的任意资源配额上限：
- 调整后立即生效
- 不影响当前已执行中的请求
- 配额缩小到当前使用量以下时，不中断已分配的资源，但禁止新的资源申请直到使用量回落到新配额以下

## 3. 核心结构体职责

### 3.1 Manager

配额管理器的核心结构体，负责：
- 维护所有租户的配额配置和使用量
- 提供资源申请、释放、查询等核心操作
- 处理并发安全控制

```go
type Manager struct {
    mu           sync.RWMutex
    config       *Config
    tenantQuotas map[string]*TenantQuota
    tenantUsages map[string]*TenantUsage
}
```

### 3.2 ResourceType

资源类型枚举，定义了三类受管理的资源：
- `ResourceCPU`：CPU 资源
- `ResourceMemory`：内存资源
- `ResourceConcurrency`：并发数资源

### 3.3 LimitMode

限制模式枚举：
- `LimitModeHard`：硬限制模式
- `LimitModeSoft`：软限制模式

### 3.4 Quota

配额配置结构体，定义三类资源的配额上限：
```go
type Quota struct {
    CPU         float64  // CPU 核心数上限
    MemoryMB    int64    // 内存 MB 上限
    Concurrency int64    // 并发请求数上限
}
```

### 3.5 Usage

资源使用量结构体，记录当前使用量：
```go
type Usage struct {
    CPU         float64  // 当前使用的 CPU 核心数
    MemoryMB    int64    // 当前使用的内存 MB
    Concurrency int64    // 当前并发请求数
}
```

### 3.6 TenantQuota

租户配额配置，包含配额、限制模式和软阈值：
```go
type TenantQuota struct {
    Quota         Quota      // 配额上限
    LimitMode     LimitMode  // 限制模式
    SoftThreshold float64    // 软阈值（软限制模式下的超额比例）
}
```

### 3.7 TenantUsage

租户资源使用量，包含并发安全的使用量统计：
```go
type TenantUsage struct {
    mu    sync.RWMutex
    usage Usage
}
```

### 3.8 ResourceUsageInfo

资源使用详情，用于查询接口返回：
```go
type ResourceUsageInfo struct {
    ResourceType ResourceType  // 资源类型
    Used         float64       // 已使用量
    Limit        float64       // 配额上限
    Remaining    float64       // 剩余可用量
    UsagePercent float64       // 使用率百分比
}
```

### 3.9 TenantQuotaInfo

租户配额和使用信息汇总：
```go
type TenantQuotaInfo struct {
    TenantID  string              // 租户 ID
    Quota     Quota               // 配额配置
    Usage     Usage               // 当前使用量
    LimitMode LimitMode           // 限制模式
    Resources []ResourceUsageInfo // 各类资源的详细使用信息
}
```

### 3.10 QuotaExceededError

配额超限错误，包含详细的超限信息：
```go
type QuotaExceededError struct {
    TenantID     string       // 租户 ID
    ResourceType ResourceType // 超限的资源类型
    Used         float64      // 已使用量
    Limit        float64      // 配额上限
    LimitMode    LimitMode    // 当前限制模式
}
```

## 4. 软硬限制策略切换逻辑

### 4.1 硬限制模式工作流程

```
申请资源
    |
    v
检查使用量 + 申请量 <= 配额上限
    |
    +-- 是 --> 分配资源，更新使用量
    |
    +-- 否 --> 返回 QuotaExceededError 错误
```

### 4.2 软限制模式工作流程

```
申请资源
    |
    v
计算软限制上限 = 配额上限 × 软阈值
    |
    v
检查使用量 + 申请量 <= 配额上限
    |
    +-- 是 --> 分配资源，更新使用量
    |
    +-- 否 --> 检查使用量 + 申请量 <= 软限制上限
                |
                +-- 是 --> 检查是否首次超限
                |           |
                |           +-- 是 --> 触发告警回调
                |           |
                |           +-- 否 --> 分配资源，更新使用量
                |
                +-- 否 --> 返回 QuotaExceededError 错误
```

### 4.3 软阈值说明

- 软阈值范围：1.0 ~ 2.0
- 默认值：1.5
- 示例：配额上限为 4.0 CPU，软阈值为 1.5，则软限制上限为 6.0 CPU
- 当使用量从 3.0 申请到 5.0 时（超过 4.0 但未超过 6.0），触发告警但允许申请
- 当使用量从 5.0 申请到 7.0 时（超过 6.0），拒绝申请

## 5. 主要 API 接口

### 5.1 创建管理器

```go
func NewManager(cfg *Config) *Manager
```

### 5.2 设置租户配额

```go
func (m *Manager) SetTenantQuota(tenantID string, quota TenantQuota) error
```

### 5.3 获取租户配额

```go
func (m *Manager) GetTenantQuota(tenantID string) (TenantQuota, error)
```

### 5.4 申请资源

```go
func (m *Manager) AcquireResource(tenantID string, resource ResourceType, amount float64) error
```

### 5.5 释放资源

```go
func (m *Manager) ReleaseResource(tenantID string, resource ResourceType, amount float64) error
```

### 5.6 查询租户使用情况

```go
func (m *Manager) GetTenantUsage(tenantID string) (*TenantQuotaInfo, error)
```

### 5.7 查询所有租户使用情况

```go
func (m *Manager) GetAllTenantsUsage() []*TenantQuotaInfo
```

### 5.8 动态调整配额

```go
func (m *Manager) AdjustQuota(tenantID string, resource ResourceType, newLimit float64) error
```

### 5.9 设置限制模式

```go
func (m *Manager) SetLimitMode(tenantID string, mode LimitMode) error
```

### 5.10 设置软阈值

```go
func (m *Manager) SetSoftThreshold(tenantID string, threshold float64) error
```

### 5.11 移除租户

```go
func (m *Manager) RemoveTenant(tenantID string) error
```

### 5.12 获取所有租户 ID

```go
func (m *Manager) TenantIDs() []string
```

## 6. 使用示例

### 6.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/quotamgr"
)

func main() {
    // 创建配额管理器，使用默认配置
    mgr := quotamgr.NewManager(nil)

    // 为租户设置自定义配额
    err := mgr.SetTenantQuota("tenant-001", quotamgr.TenantQuota{
        Quota: quotamgr.Quota{
            CPU:         8.0,
            MemoryMB:    4096,
            Concurrency: 200,
        },
        LimitMode:     quotamgr.LimitModeHard,
    })
    if err != nil {
        panic(err)
    }

    // 申请资源
    err = mgr.AcquireResource("tenant-001", quotamgr.ResourceCPU, 2.0)
    if err != nil {
        if quotaErr, ok := err.(*quotamgr.QuotaExceededError); ok {
            fmt.Printf("配额超限: %+v\n", quotaErr)
        } else {
            panic(err)
        }
    }

    // 申请内存
    err = mgr.AcquireResource("tenant-001", quotamgr.ResourceMemory, 1024)
    if err != nil {
        panic(err)
    }

    // 查询使用情况
    usage, err := mgr.GetTenantUsage("tenant-001")
    if err != nil {
        panic(err)
    }

    fmt.Printf("租户 %s 使用情况:\n", usage.TenantID)
    for _, r := range usage.Resources {
        fmt.Printf("  %s: %.2f / %.2f (%.1f%%)\n",
            r.ResourceType, r.Used, r.Limit, r.UsagePercent)
    }

    // 释放资源
    err = mgr.ReleaseResource("tenant-001", quotamgr.ResourceCPU, 1.0)
    if err != nil {
        panic(err)
    }
}
```

### 6.2 软限制模式与告警

```go
package main

import (
    "fmt"
    "solocoder-go/internal/quotamgr"
)

func alertCallback(tenantID string, resource quotamgr.ResourceType, used, limit float64) {
    fmt.Printf("⚠️  告警: 租户 %s 的 %s 资源已超过配额，当前使用: %.2f，配额上限: %.2f\n",
        tenantID, resource, used, limit)
}

func main() {
    // 创建配置，设置告警回调
    cfg := quotamgr.DefaultConfig()
    cfg.AlertCallback = alertCallback

    mgr := quotamgr.NewManager(cfg)

    // 为租户设置软限制模式
    err := mgr.SetTenantQuota("tenant-002", quotamgr.TenantQuota{
        Quota: quotamgr.Quota{
            CPU: 4.0,
        },
        LimitMode:     quotamgr.LimitModeSoft,
        SoftThreshold: 1.5, // 允许超额 50%
    })
    if err != nil {
        panic(err)
    }

    // 正常使用，在配额内
    err = mgr.AcquireResource("tenant-002", quotamgr.ResourceCPU, 3.0)
    fmt.Println("申请 3.0 CPU:", err) // 成功

    // 超过配额但未超过软阈值，触发告警但允许申请
    err = mgr.AcquireResource("tenant-002", quotamgr.ResourceCPU, 2.0)
    fmt.Println("申请 2.0 CPU:", err) // 成功，触发告警

    // 超过软阈值，拒绝申请
    err = mgr.AcquireResource("tenant-002", quotamgr.ResourceCPU, 2.0)
    fmt.Println("申请 2.0 CPU:", err) // 失败，返回超限错误
}
```

### 6.3 动态调整配额

```go
package main

import (
    "fmt"
    "solocoder-go/internal/quotamgr"
)

func main() {
    mgr := quotamgr.NewManager(nil)

    mgr.SetTenantQuota("tenant-003", quotamgr.TenantQuota{
        Quota: quotamgr.Quota{
            CPU: 4.0,
        },
        LimitMode: quotamgr.LimitModeHard,
    })

    // 申请 3.0 CPU
    mgr.AcquireResource("tenant-003", quotamgr.ResourceCPU, 3.0)

    // 将 CPU 配额提升到 8.0
    err := mgr.AdjustQuota("tenant-003", quotamgr.ResourceCPU, 8.0)
    if err != nil {
        panic(err)
    }

    // 现在可以继续申请更多 CPU
    err = mgr.AcquireResource("tenant-003", quotamgr.ResourceCPU, 4.0)
    fmt.Println("申请 4.0 CPU:", err) // 成功

    // 将 CPU 配额降低到 5.0（当前使用 7.0，已超过新配额）
    err = mgr.AdjustQuota("tenant-003", quotamgr.ResourceCPU, 5.0)
    if err != nil {
        panic(err)
    }

    // 尝试申请新的资源会被拒绝
    err = mgr.AcquireResource("tenant-003", quotamgr.ResourceCPU, 1.0)
    fmt.Println("申请 1.0 CPU:", err) // 失败，使用量已超过新配额

    // 释放部分资源后可以重新申请
    mgr.ReleaseResource("tenant-003", quotamgr.ResourceCPU, 3.0)
    err = mgr.AcquireResource("tenant-003", quotamgr.ResourceCPU, 1.0)
    fmt.Println("申请 1.0 CPU:", err) // 成功
}
```

### 6.4 并发安全使用

```go
package main

import (
    "sync"
    "solocoder-go/internal/quotamgr"
)

func main() {
    mgr := quotamgr.NewManager(nil)

    mgr.SetTenantQuota("tenant-004", quotamgr.TenantQuota{
        Quota: quotamgr.Quota{
            Concurrency: 100,
        },
        LimitMode: quotamgr.LimitModeHard,
    })

    var wg sync.WaitGroup

    // 模拟 50 个并发 goroutine 申请资源
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for j := 0; j < 10; j++ {
                err := mgr.AcquireResource("tenant-004", quotamgr.ResourceConcurrency, 1)
                if err == nil {
                    // 处理业务逻辑
                    // ...
                    mgr.ReleaseResource("tenant-004", quotamgr.ResourceConcurrency, 1)
                }
            }
        }()
    }

    wg.Wait()

    // 查询最终使用情况
    usage, _ := mgr.GetTenantUsage("tenant-004")
    fmt.Printf("最终并发使用量: %d\n", usage.Usage.Concurrency)
}
```

## 7. 错误定义

| 错误 | 说明 |
|------|------|
| `ErrInvalidTenantID` | 租户 ID 不能为空 |
| `ErrInvalidQuota` | 配额值必须为非负数 |
| `ErrInvalidSoftThreshold` | 软阈值必须在 1.0 到 2.0 之间 |
| `ErrTenantNotFound` | 租户不存在 |
| `ErrInvalidResourceType` | 无效的资源类型 |
| `ErrInvalidAmount` | 申请/释放数量必须为正数 |
| `ErrReleaseTooLarge` | 释放数量超过当前使用量 |
| `QuotaExceededError` | 配额超限（包含详细信息的结构体错误） |

## 8. 默认配置

- 默认 CPU 配额：4.0 核心
- 默认内存配额：2048 MB
- 默认并发数配额：100
- 默认限制模式：硬限制
- 默认软阈值：1.5
