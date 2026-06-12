# 安全信封模块 (secenv)

## 1. 模块概述

安全信封模块（`internal/secenv`）提供了一套完整的敏感数据加密封装和完整性校验解决方案。该模块采用分层安全设计，结合对称加密、消息认证码和防重放机制，确保数据在传输和存储过程中的保密性、完整性和新鲜性。

### 主要功能

- **AES-GCM 加密**：使用 AES-256-GCM 算法对任意字节数据（包括空字节切片）进行加密，提供保密性和完整性保护
- **HMAC-SHA256 签名**：对加密信封附加 HMAC 签名，验证方先校验签名再解密
- **版本化密钥管理**：支持管理多个版本的密钥对（加密密钥 + 签名密钥）
- **密钥轮换**：支持生成新版本密钥，自动保留历史版本用于解密旧数据
- **防重放攻击**：通过单调递增序列号和滑动窗口机制防止重放攻击

## 2. 核心结构体

### 2.1 KeyVersion

表示一个版本的密钥对，包含加密密钥和签名密钥。

```go
type KeyVersion struct {
    Version      uint32    // 密钥版本号
    EncryptKey   []byte    // AES-256 加密密钥（32字节）
    SignKey      []byte    // HMAC-SHA256 签名密钥（32字节）
    CreatedAt    time.Time // 创建时间
}
```

**职责**：
- 存储特定版本的加密密钥和签名密钥
- 确保密钥在内存中被安全复制，防止意外修改

### 2.2 Envelope

加密信封的数据结构，包含所有加密和验证所需的信息。

```go
type Envelope struct {
    FormatVersion  byte     // 信封格式版本（当前为 1）
    KeyVersion     uint32   // 使用的密钥版本号
    SequenceNum    uint64   // 单调递增序列号
    Nonce          []byte   // AES-GCM 随机数（12字节）
    Ciphertext     []byte   // 加密后的密文
    GCMTag         []byte   // AES-GCM 认证标签（16字节）
    HMACSignature  []byte   // HMAC-SHA256 签名（32字节）
}
```

**职责**：
- 封装所有加密相关的元数据和密文
- 支持序列化/反序列化为字节流进行传输

### 2.3 KeyManager

密钥管理器，负责版本化密钥的存储、查询和轮换。

```go
type KeyManager struct {
    keys        map[uint32]*KeyVersion // 版本号到密钥的映射
    currentVer  uint32                 // 当前活跃的密钥版本
    maxKeys     int                    // 最多保留的密钥版本数
    mu          sync.RWMutex           // 读写锁
}
```

**职责**：
- 管理多个版本的密钥
- 提供密钥查询（按版本号）
- 支持密钥轮换，自动修剪旧版本密钥
- 确保并发访问安全

### 2.4 ReplayProtector

防重放保护器，通过跟踪已处理的序列号防止重放攻击。

```go
type ReplayProtector struct {
    seen        map[uint32]uint64 // 每个密钥版本的最大已见序列号
    windowSize  uint64            // 序列号滑动窗口大小
    mu          sync.Mutex        // 互斥锁
}
```

**职责**：
- 记录每个密钥版本已处理的最大序列号
- 拒绝序列号小于等于已见最大值的信封
- 拒绝序列号超过滑动窗口上限的信封（防止序列号跳跃攻击）
- 支持重置状态

### 2.5 SecureEnvelope

安全信封的主接口，整合所有组件提供完整的加密/解密功能。

```go
type SecureEnvelope struct {
    keyManager    *KeyManager     // 密钥管理器
    replayGuard   *ReplayProtector // 防重放保护器
    sequenceNum   uint64          // 当前序列号计数器
    mu            sync.Mutex      // 互斥锁
}
```

**职责**：
- 提供 `Encrypt()` 方法加密数据
- 提供 `Decrypt()` 方法解密并验证数据
- 提供 `Verify()` 方法仅验证不解密
- 管理加密时的序列号递增
- 协调整个安全流程

### 2.6 Config

配置结构体，用于自定义安全信封的行为。

```go
type Config struct {
    MaxKeys       int    // 最多保留的密钥版本数（默认 10）
    ReplayWindow  uint64 // 防重放滑动窗口大小（默认 1000）
}
```

## 3. 安全信封格式

### 3.1 二进制格式

```
+-----------------+-----------------+-----------------+-------------------+
|  FormatVersion  |   KeyVersion    |  SequenceNum    |      Nonce        |
|    (1 byte)     |   (4 bytes)     |   (8 bytes)     |    (12 bytes)     |
+-----------------+-----------------+-----------------+-------------------+
|                         Ciphertext (variable)                          |
+-----------------------------------------------------------------------+
|         GCM Tag (16 bytes)      |      HMAC Signature (32 bytes)      |
+---------------------------------+-------------------------------------+
```

**总开销**：1 + 4 + 8 + 12 + 16 + 32 = **73 字节** + 明文长度

### 3.2 安全参数

| 参数 | 值 | 说明 |
|------|----|------|
| AES 密钥长度 | 256 位 (32 字节) | 提供最高级别的保密性 |
| GCM Nonce 长度 | 96 位 (12 字节) | NIST 推荐的标准长度 |
| GCM 认证标签 | 128 位 (16 字节) | 标准认证标签长度 |
| HMAC 算法 | SHA-256 | 产生 256 位 (32 字节) 签名 |
| 序列号 | 64 位无符号整数 | 单调递增，防止溢出 |

## 4. 加密与签名流程

### 4.1 加密流程

```
明文输入
    ↓
1. 获取当前最新版本的加密密钥和签名密钥
    ↓
2. 递增序列号（从 1 开始）
    ↓
3. 生成 12 字节的加密安全随机数 (Nonce)
    ↓
4. 构建 AAD (Additional Authenticated Data):
   - FormatVersion (1 byte)
   - KeyVersion (4 bytes)
   - SequenceNum (8 bytes)
   - Nonce (12 bytes)
    ↓
5. 使用 AES-256-GCM 加密明文，附带 AAD
   输出 = 密文 + GCM 认证标签
    ↓
6. 构建待签名数据（HMAC 覆盖所有字段）：
   - FormatVersion
   - KeyVersion
   - SequenceNum
   - Nonce
   - Ciphertext
   - GCM Tag
    ↓
7. 计算 HMAC-SHA256 签名
    ↓
8. 序列化为二进制信封格式
    ↓
输出：加密信封字节流
```

### 4.2 解密与验证流程

```
加密信封字节流输入
    ↓
1. 反序列化为 Envelope 结构体
    ↓
2. 验证 FormatVersion 是否支持（必须为 1）
    ↓
3. 根据 KeyVersion 查找对应的密钥对
   └─ 若找不到 → 返回 ErrKeyNotFound
    ↓
4. 使用签名密钥重新计算 HMAC 签名
   └─ 与信封中的签名比较，使用恒定时间比较
   └─ 若不匹配 → 返回 ErrInvalidSignature，终止流程
    ↓
5. 防重放检查：
   ├─ 序列号 <= 已见最大值 → ErrReplayDetected
   └─ 序列号 >= 已见最大值 + 窗口大小 → ErrReplayDetected
    ↓
6. 更新已见最大序列号
    ↓
7. 重新构建 AAD（与加密时相同）
    ↓
8. 使用 AES-256-GCM 解密并验证：
   输入 = 密文 + GCM 认证标签 + Nonce + AAD
   └─ 若认证失败 → 返回 ErrInvalidTag
    ↓
输出：明文数据
```

### 4.3 安全特性说明

**双重完整性保护**：
- 内层：AES-GCM 提供密文和 AAD 的完整性保护
- 外层：HMAC-SHA256 提供整个信封的完整性保护
- 设计原则：先验证 HMAC，再进行解密操作，避免无效的密码学计算

**AAD 完整性**：
- 密钥版本号、序列号、随机数等元数据都被纳入 AAD 保护
- 任何对元数据的篡改都会被 GCM 或 HMAC 检测到

**密钥隔离**：
- 加密密钥和签名密钥是独立生成的
- 一个密钥泄露不会影响另一个的安全性

## 5. 密钥管理

### 5.1 密钥轮换流程

```
当前状态：活跃密钥版本 = N
    ↓
调用 RotateKey()
    ↓
1. 生成新的加密密钥（32字节随机）
2. 生成新的签名密钥（32字节随机）
3. 新版本号 = N + 1
4. 创建新的 KeyVersion 对象
5. 添加到 KeyManager
6. 设置当前活跃版本为 N + 1
    ↓
检查密钥数量：
   若超过 maxKeys → 删除最旧的版本
    ↓
返回新的 KeyVersion
```

### 5.2 密钥修剪策略

- 当密钥数量超过 `MaxKeys` 配置时，自动删除最旧的密钥版本
- 保留最新的 `MaxKeys` 个版本
- 确保历史数据可以被解密，只要使用的密钥版本还在保留范围内

### 5.3 密钥安全

- 密钥使用 `crypto/rand` 生成，符合密码学安全标准
- `GetKey()` 方法返回密钥的副本，防止外部修改内部密钥
- 所有密钥操作都在锁保护下进行，确保线程安全

## 6. 使用示例

### 6.1 基本加密解密

```go
package main

import (
    "fmt"
    "solocoder-go/internal/secenv"
)

func main() {
    // 创建安全信封实例（使用默认配置）
    se, err := secenv.NewSecureEnvelope(nil)
    if err != nil {
        panic(err)
    }

    // 加密敏感数据
    plaintext := []byte("这是敏感数据，需要加密保护")
    envelope, err := se.Encrypt(plaintext)
    if err != nil {
        panic(err)
    }

    fmt.Printf("加密后长度: %d 字节\n", len(envelope))

    // 解密数据（需要使用相同的密钥管理器）
    se2 := secenv.NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
    decrypted, err := se2.Decrypt(envelope)
    if err != nil {
        panic(err)
    }

    fmt.Printf("解密结果: %s\n", decrypted)
}
```

### 6.2 自定义配置

```go
cfg := &secenv.Config{
    MaxKeys:      5,     // 最多保留 5 个历史密钥版本
    ReplayWindow: 500,   // 防重放窗口大小为 500
}

se, err := secenv.NewSecureEnvelope(cfg)
if err != nil {
    panic(err)
}
```

### 6.3 密钥轮换

```go
se, err := secenv.NewSecureEnvelope(nil)
if err != nil {
    panic(err)
}

// 加密一些数据（使用版本 1 的密钥）
env1, _ := se.Encrypt([]byte("使用密钥版本 1 加密"))

// 轮换密钥
newKV, err := se.RotateKey()
if err != nil {
    panic(err)
}
fmt.Printf("新密钥版本: %d\n", newKV.Version)

// 加密新数据（使用版本 2 的密钥）
env2, _ := se.Encrypt([]byte("使用密钥版本 2 加密"))

// 解密时会自动选择正确的密钥版本
se2 := secenv.NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
data1, _ := se2.Decrypt(env1) // 使用版本 1 解密
data2, _ := se2.Decrypt(env2) // 使用版本 2 解密
```

### 6.4 仅验证不解密

```go
se, err := secenv.NewSecureEnvelope(nil)
if err != nil {
    panic(err)
}

envelope, _ := se.Encrypt([]byte("test"))

// 只验证签名和防重放，不解密
seVerifier := secenv.NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
err = seVerifier.Verify(envelope)
if err != nil {
    fmt.Printf("验证失败: %v\n", err)
} else {
    fmt.Println("验证通过，信封完整且未被重放")
}
```

### 6.5 使用预定义密钥

```go
// 如果你有自己的密钥材料
encKey := make([]byte, 32) // 必须是 32 字节
signKey := make([]byte, 32) // 必须是 32 字节
// ... 从安全的密钥管理系统获取密钥 ...

kv, err := secenv.NewKeyVersionWithKeys(1, encKey, signKey)
if err != nil {
    panic(err)
}

km := secenv.NewKeyManager(nil)
km.AddKey(kv)

se := secenv.NewSecureEnvelopeWithKeyManager(km, 1000)
```

### 6.6 错误处理

```go
se, _ := secenv.NewSecureEnvelope(nil)
envelope, _ := se.Encrypt([]byte("test"))

// 篡改数据
envelope[50] ^= 0xFF

se2 := secenv.NewSecureEnvelopeWithKeyManager(se.GetKeyManager(), 1000)
_, err := se2.Decrypt(envelope)

switch err {
case secenv.ErrInvalidSignature:
    fmt.Println("签名验证失败，数据可能被篡改")
case secenv.ErrInvalidTag:
    fmt.Println("GCM 认证失败，数据可能被篡改")
case secenv.ErrReplayDetected:
    fmt.Println("检测到重放攻击")
case secenv.ErrKeyNotFound:
    fmt.Println("密钥版本不存在")
default:
    fmt.Printf("其他错误: %v\n", err)
}
```

## 7. 加密输入约束

### 7.1 Encrypt 输入约束

`Encrypt` 方法接受任意长度的字节切片作为明文输入，**包括空字节切片（`[]byte{}`）和 `nil`**。AES-GCM 算法本身支持对空明文的加密操作，会生成仅包含认证标签的有效密文。空数据加密后的信封仅包含头部和认证开销（共 73 字节），不含密文部分。

设计原则：
- 空数据属于合法输入，不应对加密方施加"数据不能为空"的业务约束
- 对空数据的加密结果依然具有完整的 HMAC 签名和 GCM 认证标签保护
- 解密空数据信封后返回空字节切片，与加密时输入一致

### 7.2 Decrypt / Verify 输入约束

`Decrypt` 和 `Verify` 方法要求输入的信封字节流不为空。传入空切片或 `nil` 将返回 `ErrEmptyData`，因为空字节流无法构成有效的加密信封。

## 8. 错误分类策略

模块的错误类型按照安全验证的层次结构组织，调用方可根据错误类型判断数据在哪个安全层被拒绝：

| 验证阶段 | 错误变量 | 说明 |
|----------|----------|------|
| 格式解析 | `ErrInvalidFormat` | 信封二进制格式无效，长度不足以构成合法信封 |
| 格式解析 | `ErrInvalidVersion` | 信封格式版本不支持（当前仅支持版本 1） |
| 密钥查找 | `ErrKeyNotFound` | 信封中携带的密钥版本在当前 KeyManager 中不存在 |
| 签名验证 | `ErrInvalidSignature` | HMAC-SHA256 签名验证失败，信封可能被篡改 |
| 防重放检查 | `ErrReplayDetected` | 序列号 ≤ 已见最大值或 ≥ 已见最大值 + 窗口大小 |
| 密码学验证 | `ErrInvalidTag` | AES-GCM 认证标签验证失败，密文可能被篡改 |
| 输入校验 | `ErrEmptyData` | Decrypt/Verify 收到空信封字节流 |
| 密钥校验 | `ErrInvalidKeySize` | 密钥长度不正确（必须为 32 字节） |

**验证顺序**：格式解析 → 密钥查找 → HMAC 签名验证 → 防重放检查 → GCM 解密验证

这种分层设计确保：
1. 签名验证失败时直接拒绝，不进入解密流程，避免无效的密码学计算
2. 防重放检查在签名验证之后执行，因为只有确认信封完整后才可信任其序列号
3. GCM 解密是最后一步，只有通过所有前置验证后才执行

## 9. 安全最佳实践

1. **密钥保护**：
   - 不要硬编码密钥，使用安全的密钥管理系统
   - 定期轮换密钥（根据安全策略）
   - 确保密钥在内存中的安全，避免泄露到日志或错误消息

2. **防重放配置**：
   - 根据业务场景设置合适的 `ReplayWindow`
   - 对于高安全性场景，可以减小窗口大小
   - 对于异步消息场景，可能需要增大窗口大小

3. **错误处理**：
   - 始终检查错误返回值
   - 不要向攻击者透露过多错误细节
   - 记录安全相关的错误以便审计

4. **并发安全**：
   - `SecureEnvelope` 是并发安全的
   - 但建议加密和解密使用不同的实例（共享 `KeyManager`）
   - 防重放状态是按实例隔离的

## 10. 性能考虑

- **加密开销**：主要来自 AES-GCM 和 HMAC-SHA256，都是硬件加速的算法
- **内存开销**：每个密钥版本占用 64 字节（两个 32 字节密钥），保留 10 个版本仅占用 640 字节
- **序列化开销**：直接内存拷贝，无额外分配
- **推荐使用场景**：适合加密小到中等大小的数据（几 KB 到几 MB）
- **大数据处理**：对于 GB 级数据，建议使用流式加密或分块加密
