# TLS 证书自动轮换模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [证书状态流转](#4-证书状态流转)
5. [证书轮换完整流程](#5-证书轮换完整流程)
6. [证书链验证机制](#6-证书链验证机制)
7. [平滑切换与优雅淘汰](#7-平滑切换与优雅淘汰)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全](#10-并发安全)
11. [配置说明](#11-配置说明)

---

## 1. 模块概述

TLS 证书自动轮换模块是一个生产级别的证书管理组件，提供证书到期前自动续期、新证书平滑切换、旧证书优雅淘汰以及完整证书链验证等功能。模块设计用于需要高可用性和零停机证书更新的 TLS 服务场景。

**包路径**: `internal/certrotator`

**设计目标**:
- 实现证书到期前的自动化续期，避免证书过期导致的服务中断
- 支持新证书的无锁原子切换，保证活跃连接不受影响
- 提供旧证书的优雅淘汰机制，等待现有连接自然关闭后释放资源
- 严格的证书链验证，确保证书的完整性和可信度
- 支持事件通知机制，便于监控和审计证书生命周期

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 自动续期 | 定期检查证书到期时间，在可配置的缓冲期内自动触发续期流程 |
| 证书颁发接口 | 通过 `CertificateIssuer` 接口对接外部证书颁发机构（CA） |
| 预校验机制 | 新证书加载到内存后先完成预校验，校验通过后才进入切换流程 |
| 原子切换 | 使用 `atomic.Pointer` 实现无锁原子证书切换，读操作永不阻塞 |
| 连接追踪 | 精确追踪每个证书的活跃连接数，实现精准的优雅淘汰 |
| 优雅淘汰 | 新证书切换后，旧证书进入淘汰阶段，等待现有连接自然关闭 |
| 强制淘汰 | 淘汰阶段设置最大等待时长，超时后强制关闭残留连接 |
| 证书链验证 | 加载证书时自动验证证书链的完整性、可信度、有效期和签名 |
| 事件通知 | 提供 `EventHandler` 回调，支持证书生命周期事件的监听和处理 |
| 监控循环 | 后台 goroutine 定期检查证书状态，触发续期流程 |

---

## 3. 核心结构体与职责

### 3.1 CertRotator

证书轮换主结构体，对外提供所有操作接口，协调证书生命周期的完整管理。

```go
type CertRotator struct {
    config         *Config
    issuer         CertificateIssuer
    loader         CertificateLoader
    activeCert     atomic.Pointer[CertificateInfo]
    pendingCert    *CertificateInfo
    retiringCerts  map[string]*CertificateInfo
    connections    map[string][]trackedConn
    mu             sync.RWMutex
    clock          func() time.Time
    eventHandler   EventHandler
    ctx            context.Context
    cancel         context.CancelFunc
    closeCh        chan struct{}
    wg             sync.WaitGroup
}
```

**职责**:
- 管理证书的完整生命周期（加载、续期、切换、淘汰）
- 维护活跃证书、待切换证书和淘汰中证书的状态
- 协调证书颁发接口和证书加载接口的交互
- 实现原子证书切换，保证并发安全
- 追踪和管理每个证书的活跃连接
- 执行证书链验证，确保证书有效性
- 触发证书生命周期事件通知
- 管理后台监控协程的生命周期

### 3.2 CertificateInfo

证书信息结构体，包含证书的元数据、状态和完整证书内容。

```go
type CertificateInfo struct {
    ID          string
    Certificate *x509.Certificate
    TLSCert     *tls.Certificate
    Status      CertStatus
    NotBefore   time.Time
    NotAfter    time.Time
    Issuer      string
    Subject     string
    Serial      string
}
```

**职责**:
- 存储证书的完整信息（X.509 证书、TLS 证书、私钥）
- 维护证书的当前状态（PENDING/ACTIVE/RETIRING/RETIRED）
- 提供证书的有效期信息用于续期判断
- 包含证书的唯一标识用于连接追踪

### 3.3 Config

配置结构体，定义证书轮换模块的所有可配置参数。

```go
type Config struct {
    CheckInterval       time.Duration
    RenewalBuffer       time.Duration
    RetirementTimeout   time.Duration
    PreValidationChecks bool
    RootCAs             *x509.CertPool
    IntermediateCAs     *x509.CertPool
}
```

**职责**:
- 配置证书检查周期（`CheckInterval`）
- 配置续期缓冲期（`RenewalBuffer`），即到期前多久开始续期
- 配置淘汰超时时间（`RetirementTimeout`）
- 配置是否启用预校验（`PreValidationChecks`）
- 配置信任的根证书池（`RootCAs`）
- 配置信任的中间证书池（`IntermediateCAs`）

### 3.4 核心接口

#### CertificateIssuer

证书颁发接口，定义向证书颁发机构请求新证书的契约。

```go
type CertificateIssuer interface {
    IssueCertificate() (*tls.Certificate, error)
}
```

#### CertificateLoader

证书加载接口，定义加载初始证书的契约。

```go
type CertificateLoader interface {
    LoadCertificate() (*tls.Certificate, error)
}
```

#### ConnectionTracker

连接追踪接口，定义连接管理的契约。

```go
type ConnectionTracker interface {
    TrackConnection(certID string, conn interface{}, closeFn func() error) func()
    ActiveConnections(certID string) int
    CloseConnections(certID string) int
}
```

#### EventHandler

事件处理器类型，定义证书生命周期事件的回调签名。

```go
type EventHandler func(event RotationEvent)
```

---

## 4. 证书状态流转

### 4.1 证书状态枚举

```go
type CertStatus int

const (
    CertStatusPending   CertStatus = iota // 待切换
    CertStatusActive                      // 活跃中
    CertStatusRetiring                    // 淘汰中
    CertStatusRetired                     // 已淘汰
)
```

### 4.2 状态流转图

```
  ┌─────────────┐     切换成功      ┌─────────────┐     新证书切换     ┌─────────────┐
  │   PENDING   │──────────────────▶│   ACTIVE    │──────────────────▶│  RETIRING   │
  └─────────────┘                    └─────────────┘                    └─────────────┘
         ▲                                                                  │
         │ 续期成功、预校验通过                                             │ 连接全部关闭
         │                                                                  │ 或超时
         └──────────────────────────────────────────────────────────────────┘
                                                                               │
                                                                               ▼
                                                                        ┌─────────────┐
                                                                        │  RETIRED    │
                                                                        └─────────────┘
```

### 4.3 状态流转说明

| 状态 | 说明 | 触发条件 | 下一状态 |
|------|------|----------|----------|
| PENDING | 新证书已签发并通过预校验，等待切换 | 证书续期成功且预校验通过 | ACTIVE（切换成功） |
| ACTIVE | 当前正在使用的证书 | 初始加载成功、PENDING 证书切换成功 | RETIRING（有新证书切换） |
| RETIRING | 已被替换，等待连接关闭的旧证书 | 新证书切换为 ACTIVE，原 ACTIVE 证书进入此状态 | RETIRED（连接全部关闭或超时） |
| RETIRED | 已完全淘汰，资源已释放 | 所有连接关闭或超时强制关闭 | PENDING（如果再次续期） |

---

## 5. 证书轮换完整流程

### 5.1 初始化流程

1. **加载初始证书**: 通过 `CertificateLoader` 加载初始证书
2. **证书解析**: 解析证书内容，提取有效期、颁发者、主题等信息
3. **证书链验证**: 如果启用 `PreValidationChecks`，执行完整的证书链验证
4. **有效性检查**: 检查证书是否已过期或尚未生效
5. **设置为活跃证书**: 通过原子操作将证书设置为活跃证书
6