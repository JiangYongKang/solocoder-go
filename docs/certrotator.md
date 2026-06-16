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
6. **启动监控循环**: 启动后台 goroutine 定期检查证书状态

### 5.2 自动续期流程

1. **定期检查**: 监控循环每 `CheckInterval` 时间触发一次检查
2. **到期判断**: 检查当前活跃证书的到期时间
3. **续期触发**: 如果 `到期时间 - 当前时间 < RenewalBuffer`，触发续期流程
4. **请求新证书**: 调用 `CertificateIssuer.IssueCertificate()` 请求新证书
5. **预校验**: 对新证书执行解析、有效期检查、证书链验证
6. **设置待切换**: 校验通过的新证书设置为 `pendingCert`，状态为 PENDING
7. **原子切换**: 通过 `atomic.Pointer.Swap()` 将新证书切换为活跃证书
8. **旧证书淘汰**: 原活跃证书进入 RETIRING 状态，开始等待连接关闭
9. **事件通知**: 发送证书切换事件

### 5.3 强制续期流程

用户可通过 `ForceRenew()` 方法强制触发续期，流程与自动续期相同，但跳过到期判断。

---

## 6. 证书链验证机制

### 6.1 验证内容

模块在加载证书时执行完整的证书链验证，包括：

| 验证项 | 说明 | 失败错误 |
|--------|------|----------|
| 证书完整性 | 证书数据非空，可正常解析 | `ErrCertChainIncomplete`, `ErrCertParseFailed` |
| 有效期检查 | 链中所有证书在当前时间有效 | `ErrCertExpired`, `ErrCertNotYetValid` |
| 根证书信任 | 证书链最终链接到信任的根证书 | `ErrCertRootUntrusted` |
| 中间证书完整性 | 所有中间证书存在且有效 | `ErrCertChainIncomplete` |
| 签名有效性 | 每个证书的签名都被父证书正确签名 | `ErrCertSignatureInvalid` |
| 证书用途 | 证书的 KeyUsage 允许其作为 CA 或服务器证书 | `ErrCertSignatureInvalid` |

### 6.2 验证流程

1. **解析叶子证书**: 从 `tls.Certificate` 中解析出叶子证书（索引 0）
2. **构建中间证书池**: 将链中除叶子证书外的所有证书加入中间证书池
3. **单证书有效性检查**: 检查链中每个证书的有效期
4. **调用 x509.Verify**: 使用 Go 标准库的 `x509.Certificate.Verify()` 执行完整验证
5. **错误分类**: 将标准库返回的错误映射为模块定义的明确错误
6. **验证通过**: 返回 nil，表示证书链完整可信

### 6.3 信任配置

- **RootCAs**: 信任的根证书池，默认为系统根证书池
- **IntermediateCAs**: 信任的中间证书池，可选配置
- 证书链必须能够链接到 `RootCAs` 中的某个根证书才算验证通过

---

## 7. 平滑切换与优雅淘汰

### 7.1 平滑切换机制

**核心设计**: 使用 `atomic.Pointer[CertificateInfo]` 实现无锁原子切换。

```go
// 原子切换证书
newInfo.Status = CertStatusActive
oldInfo := cr.activeCert.Swap(newInfo)
```

**特点**:
- **读操作永不阻塞**: `GetCertificate()` 只需原子读取指针，无需加锁
- **写操作安全**: 切换操作是原子的，不会出现中间状态
- **活跃连接不受影响**: 已建立的连接继续使用旧证书，直到连接关闭
- **新连接使用新证书**: 证书切换后建立的新连接自动使用新证书

### 7.2 连接追踪机制

**核心设计**: 每个连接关联到其使用的证书 ID，精确追踪每个证书的活跃连接数。

```go
// 追踪连接，返回 release 函数
func (cr *CertRotator) TrackConnection(certID string, conn interface{}, closeFn func() error) func()
```

**使用方式**:
```go
// 在 TLS 握手完成后追踪连接
conn := ... // 新建立的连接
certInfo := cr.ActiveCertificate()
release := cr.TrackConnection(certInfo.ID, conn, conn.Close)
defer release() // 连接关闭时调用 release
```

### 7.3 优雅淘汰流程

1. **进入淘汰阶段**: 旧证书被切换后，状态变为 RETIRING
2. **启动淘汰协程**: 启动后台 goroutine 执行淘汰等待逻辑
3. **定期检查连接数**: 每 10ms 检查一次该证书的活跃连接数
4. **连接全部关闭**: 如果活跃连接数为 0，直接完成淘汰
5. **超时强制关闭**: 如果等待时间超过 `RetirementTimeout`，强制关闭所有残留连接
6. **释放资源**: 从 `retiringCerts` 中移除证书，发送淘汰完成事件
7. **事件通知**: 根据淘汰方式发送 `CERT_RETIRED` 或 `CERT_FORCE_RETIRED` 事件

### 7.4 强制淘汰

当 `RetirementTimeout` 超时时：
- 调用 `CloseConnections(certID)` 强制关闭所有使用该证书的连接
- 发送 `CERT_FORCE_RETIRED` 事件
- 立即完成淘汰流程

---

## 8. 使用示例

### 8.1 基本使用

```go
package main

import (
    "crypto/tls"
    "solocoder-go/internal/certrotator"
    "time"
)

func main() {
    // 1. 实现证书颁发接口
    issuer := &MyCertificateIssuer{}
    
    // 2. 实现证书加载接口
    loader := &MyCertificateLoader{}
    
    // 3. 创建配置
    config := &certrotator.Config{
        CheckInterval:       1 * time.Hour,
        RenewalBuffer:       30 * 24 * time.Hour, // 到期前30天续期
        RetirementTimeout:   5 * time.Minute,      // 淘汰最多等待5分钟
        PreValidationChecks: true,
    }
    
    // 4. 创建证书轮换器
    cr, err := certrotator.New(issuer, loader, config)
    if err != nil {
        panic(err)
    }
    defer cr.Close()
    
    // 5. 设置事件处理器（可选）
    cr.SetEventHandler(func(event certrotator.RotationEvent) {
        switch event.Type {
        case certrotator.EventCertRenewed:
            // 证书续期成功
        case certrotator.EventCertSwitched:
            // 证书已切换
        case certrotator.EventCertRetired:
            // 证书优雅淘汰完成
        case certrotator.EventCertForceRetired:
            // 证书被强制淘汰
        case certrotator.EventRenewalFailed:
            // 续期失败
        }
    })
    
    // 6. 配置 TLS 服务器
    tlsConfig := &tls.Config{
        GetCertificate: cr.GetCertificate,
    }
    
    // 7. 启动服务器
    listener, err := tls.Listen("tcp", ":443", tlsConfig)
    if err != nil {
        panic(err)
    }
    
    // 8. 接受连接并追踪
    for {
        conn, err := listener.Accept()
        if err != nil {
            continue
        }
        
        // 获取当前活跃证书并追踪连接
        certInfo := cr.ActiveCertificate()
        release := cr.TrackConnection(certInfo.ID, conn, conn.Close)
        
        go handleConnection(conn, release)
    }
}

func handleConnection(conn net.Conn, release func()) {
    defer release() // 确保连接关闭时调用 release
    // 处理连接...
}
```

### 8.2 手动续期和切换

```go
// 强制续期（跳过到期检查）
err := cr.ForceRenew()
if err != nil {
    // 处理续期失败
}

// 强制切换待切换证书（如果有）
err = cr.ForceSwitch()
if err != nil {
    // 处理切换失败
}

// 获取当前活跃证书
activeCert := cr.ActiveCertificate()
fmt.Printf("Active cert: %s, expires: %s", activeCert.ID, activeCert.NotAfter)

// 获取待切换证书
pendingCert := cr.PendingCertificate()

// 获取淘汰中证书列表
retiringCerts := cr.RetiringCertificates()

// 检查是否需要续期
needsRenewal := cr.NeedsRenewal()

// 获取到期时间
timeUntilExpiry := cr.TimeUntilExpiry()
timeUntilRenewal := cr.TimeUntilRenewal()
```

### 8.3 使用系统根证书池

```go
// 使用系统默认根证书池（RootCAs 为 nil 时自动使用）
config := &certrotator.Config{
    CheckInterval:       1 * time.Hour,
    RenewalBuffer:       30 * 24 * time.Hour,
    RetirementTimeout:   5 * time.Minute,
    PreValidationChecks: true,
    // RootCAs 为 nil，自动使用系统根证书池
}

// 或手动加载系统根证书池
rootPool, err := x509.SystemCertPool()
if err != nil {
    // 处理错误
}
config.RootCAs = rootPool
```

### 8.4 使用自定义根证书

```go
// 加载自定义根证书
rootCertData, err := os.ReadFile("root-ca.crt")
if err != nil {
    panic(err)
}

rootPool := x509.NewCertPool()
if !rootPool.AppendCertsFromPEM(rootCertData) {
    panic("failed to parse root certificate")
}

// 加载自定义中间证书（可选）
intermediateCertData, err := os.ReadFile("intermediate-ca.crt")
if err != nil {
    panic(err)
}

intermediatePool := x509.NewCertPool()
if !intermediatePool.AppendCertsFromPEM(intermediateCertData) {
    panic("failed to parse intermediate certificate")
}

config := &certrotator.Config{
    CheckInterval:       1 * time.Hour,
    RenewalBuffer:       30 * 24 * time.Hour,
    RetirementTimeout:   5 * time.Minute,
    PreValidationChecks: true,
    RootCAs:             rootPool,
    IntermediateCAs:     intermediatePool,
}
```

---

## 9. 错误定义

| 错误变量 | 错误说明 |
|----------|----------|
| `ErrCertExpired` | 证书已过期 |
| `ErrCertNotYetValid` | 证书尚未生效 |
| `ErrCertChainIncomplete` | 证书链不完整 |
| `ErrCertRootUntrusted` | 根证书不受信任 |
| `ErrCertSignatureInvalid` | 证书签名无效 |
| `ErrCertParseFailed` | 证书解析失败 |
| `ErrCertValidationFailed` | 证书验证失败 |
| `ErrCertNameMismatch` | 证书名称不匹配 |
| `ErrNoActiveCert` | 当前没有活跃证书 |
| `ErrNoPendingCert` | 当前没有待切换证书 |
| `ErrCertAlreadyActive` | 证书已是活跃证书 |
| `ErrIssuerNil` | 证书颁发器为 nil |
| `ErrLoaderNil` | 证书加载器为 nil |
| `ErrIssuerFailed` | 证书颁发失败 |
| `ErrLoaderFailed` | 证书加载失败 |
| `ErrRotatorClosed` | 证书轮换器已关闭 |

---

## 10. 并发安全

### 10.1 原子操作

- **活跃证书指针**: 使用 `atomic.Pointer[CertificateInfo]` 存储活跃证书，读取和切换都是原子操作
- **Mock 组件计数**: 使用 `atomic.Int32` 保证并发场景下的计数准确
- **连接关闭标记**: 使用 `atomic.Bool` 标记连接是否已关闭

### 10.2 锁机制

- **主互斥锁 `mu`**: 保护 `pendingCert`、`retiringCerts`、`connections` 等共享状态的读写
- **Mock 组件锁**: Mock 实现使用独立的互斥锁保护内部状态

### 10.3 并发访问保证

| 操作 | 线程安全 | 说明 |
|------|----------|------|
| `GetCertificate()` | ✅ | 原子读取，无锁 |
| `ActiveCertificate()` | ✅ | 原子读取，无锁 |
| `PendingCertificate()` | ✅ | 读锁保护 |
| `RetiringCertificates()` | ✅ | 读锁保护 |
| `NeedsRenewal()` | ✅ | 原子读取 |
| `ForceRenew()` | ✅ | 写锁保护 |
| `ForceSwitch()` | ✅ | 写锁保护 |
| `TrackConnection()` | ✅ | 写锁保护 |
| `CloseConnections()` | ✅ | 写锁保护 |
| `VerifyCertificateChain()` | ✅ | 只读操作 |
| `SetEventHandler()` | ✅ | 原子操作 |

---

## 11. 配置说明

### 11.1 默认配置

```go
func DefaultConfig() *Config {
    return &Config{
        CheckInterval:       1 * time.Hour,
        RenewalBuffer:       30 * 24 * time.Hour,  // 30天
        RetirementTimeout:   5 * time.Minute,
        PreValidationChecks: true,
    }
}
```

### 11.2 配置项详解

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `CheckInterval` | `time.Duration` | 1小时 | 证书检查周期，多久检查一次证书是否需要续期 |
| `RenewalBuffer` | `time.Duration` | 30天 | 续期缓冲期，证书到期前多久开始续期 |
| `RetirementTimeout` | `time.Duration` | 5分钟 | 淘汰超时时间，等待连接关闭的最长时间 |
| `PreValidationChecks` | `bool` | `true` | 是否启用证书预校验，包括证书链验证 |
| `RootCAs` | `*x509.CertPool` | `nil` | 信任的根证书池，为 nil 时使用系统根证书池 |
| `IntermediateCAs` | `*x509.CertPool` | `nil` | 信任的中间证书池，可选 |

### 11.3 配置建议

- **生产环境**: `CheckInterval` 建议设置为 1-24 小时，`RenewalBuffer` 建议设置为证书有效期的 1/3 或至少 30 天
- **测试环境**: 可将 `CheckInterval` 和 `RenewalBuffer` 设置为较小值（如几秒）以快速测试轮换流程
- **长连接服务**: `RetirementTimeout` 建议设置为稍长于最长连接的预期持续时间
- **短连接服务**: `RetirementTimeout` 可设置为较短时间（如 1 分钟）

### 11.4 零值处理

如果配置项为零值，模块会自动使用默认值：
- `CheckInterval` 为 0 时，使用 1 小时
- `RenewalBuffer` 为 0 时，使用 30 天
- `RetirementTimeout` 为 0 时，使用 5 分钟

---

## 事件类型

| 事件类型 | 说明 | 事件数据 |
|----------|------|----------|
| `EventCertRenewed` | 证书续期成功 | `CertID`: 新证书 ID |
| `EventCertSwitched` | 证书已切换为活跃 | `CertID`: 新证书 ID |
| `EventCertRetired` | 证书优雅淘汰完成 | `CertID`: 被淘汰证书 ID |
| `EventCertForceRetired` | 证书被强制淘汰 | `CertID`: 被淘汰证书 ID |
| `EventRenewalFailed` | 证书续期失败 | `CertID`: 失败时关联的证书 ID, `Error`: 错误信息 |
