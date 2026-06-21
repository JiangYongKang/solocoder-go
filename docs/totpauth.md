# TOTP 一次性密码认证器模块

## 目录

1. [模块概述](#1-模块概述)
2. [核心功能](#2-核心功能)
3. [核心结构体与职责](#3-核心结构体与职责)
4. [TOTP 标准算法流程](#4-totp-标准算法流程)
5. [时间窗口漂移容忍机制](#5-时间窗口漂移容忍机制)
6. [密钥 Base32 编码存储](#6-密钥-base32-编码存储)
7. [备用恢复码管理](#7-备用恢复码管理)
8. [使用示例](#8-使用示例)
9. [错误定义](#9-错误定义)
10. [并发安全设计](#10-并发安全设计)

---

## 1. 模块概述

TOTP（Time-based One-Time Password）一次性密码认证器模块是一个基于 RFC 6238 标准的双因素认证（2FA）组件。它通过共享密钥和当前时间戳生成动态的一次性密码，为用户账户提供额外的安全保护层。模块同时支持备用恢复码机制，确保用户在无法获取 TOTP 密码时仍能完成认证。

**包路径**: `internal/totpauth`

**设计目标**:
- 严格遵循 RFC 6238（TOTP）和 RFC 4226（HOTP）标准
- 支持 6-8 位数字密码，可配置长度
- 支持可配置的时间步长（默认 30 秒）
- 支持多种哈希算法（SHA-1、SHA-256、SHA-512）
- 支持时间窗口漂移容忍，解决客户端与服务端时钟不同步问题
- 密钥使用 Base32 编码存储和传输，兼容 Google Authenticator 等认证器应用
- 提供一次性备用恢复码管理功能
- 完整的并发安全保证

---

## 2. 核心功能

| 功能 | 描述 |
|------|------|
| 密钥生成 | 使用 `crypto/rand` 安全随机生成密钥，自动转换为 Base32 编码格式 |
| TOTP 密码生成 | 根据共享密钥和当前时间戳，按照 TOTP 标准算法生成一次性密码 |
| TOTP 密码校验 | 将用户输入的密码与相同算法生成的密码比对，使用恒定时间比较防止时序攻击 |
| 密码长度配置 | 支持 6 至 8 位数字密码，默认 6 位 |
| 时间步长配置 | 可配置每个密码的有效秒数，默认 30 秒 |
| 时间窗口漂移 | 支持配置前后各容忍几个时间窗口的密码均为有效 |
| 多算法支持 | 支持 SHA-1、SHA-256、SHA-512 三种哈希算法 |
| 备用恢复码生成 | 为每个密钥生成一组一次性使用的备用恢复码 |
| 恢复码校验 | 验证恢复码有效性，使用后立即失效 |
| 恢复码状态查询 | 查询恢复码总数、剩余数量、使用状态 |
| 恢复码重新生成 | 恢复码用完后可重新生成一组新的恢复码 |

---

## 3. 核心结构体与职责

### 3.1 TOTP

TOTP 认证器主结构体，对外提供所有 TOTP 相关操作接口。

```go
type TOTP struct {
    cfg Config
}
```

**职责**:
- 管理 TOTP 认证的配置参数
- 提供密钥生成、密码生成、密码校验等核心功能
- 内部实现 HOTP 底层算法和时间窗口计算

**主要方法**:
- `NewTOTP()` - 使用默认配置创建 TOTP 认证器，返回错误
- `NewTOTPWithConfig(cfg)` - 使用自定义配置创建 TOTP 认证器
- `Config()` - 获取当前配置
- `GenerateSecret()` - 生成新的共享密钥（Base32 编码）
- `GenerateCode(secretBase32)` - 生成当前时间的 TOTP 密码
- `GenerateCodeAt(secretBase32, tm)` - 生成指定时间的 TOTP 密码
- `ValidateCode(secretBase32, code)` - 校验当前时间的 TOTP 密码
- `ValidateCodeAt(secretBase32, code, tm)` - 校验指定时间的 TOTP 密码

### 3.2 Config

TOTP 配置结构体，包含所有可配置参数。

```go
type Config struct {
    Digits       int
    Period       int
    DriftWindows int
    Algorithm    Algorithm
    SecretSize   int
}
```

**字段说明**:
- `Digits`: 密码位数，范围 6-8，默认 6
- `Period`: 时间步长（秒），每个密码的有效时长，默认 30
- `DriftWindows`: 漂移窗口数，前后各容忍的窗口数，默认 1
- `Algorithm`: 哈希算法，支持 SHA1、SHA256、SHA512，默认 SHA1
- `SecretSize`: 密钥字节数，默认 20

### 3.3 Algorithm

哈希算法枚举类型。

```go
type Algorithm int

const (
    SHA1   Algorithm = iota
    SHA256
    SHA512
)
```

**算法选项**:
- `SHA1`: 默认算法，兼容性最好，Google Authenticator 默认使用
- `SHA256`: 更高安全性的 SHA-256 算法
- `SHA512`: 最高安全性的 SHA-512 算法

### 3.4 RecoveryCodeStore

备用恢复码存储管理器，负责恢复码的生成、验证和状态管理。

```go
type RecoveryCodeStore struct {
    mu     sync.RWMutex
    codes  map[string]*RecoveryCode
    order  []string
}
```

**职责**:
- 管理一组备用恢复码的生命周期
- 提供恢复码的生成、验证、查询功能
- 通过 `sync.RWMutex` 保证并发安全
- 维护恢复码的生成顺序，便于按序展示

**主要方法**:
- `NewRecoveryCodeStore()` - 创建新的恢复码存储
- `Generate(count)` - 生成指定数量的恢复码
- `Validate(code)` - 验证并使用恢复码
- `IsUsed(code)` - 查询恢复码是否已使用
- `Remaining()` - 获取剩余可用恢复码数量
- `Total()` - 获取恢复码总数
- `List()` - 列出所有恢复码及其状态
- `AllUsed()` - 检查是否所有恢复码都已使用
- `Regenerate(count)` - 清除旧码并生成一组新的恢复码

### 3.5 RecoveryCode

单个恢复码的内部表示。

```go
type RecoveryCode struct {
    Code   string
    Used   bool
    UsedAt time.Time
}
```

**字段说明**:
- `Code`: 恢复码字符串，使用 Base32 字符集
- `Used`: 是否已使用
- `UsedAt`: 使用时间，未使用则为零值

---

## 4. TOTP 标准算法流程

TOTP（Time-based One-Time Password）算法基于 HOTP（HMAC-based One-Time Password）算法，将时间戳作为计数器输入。

### 4.1 算法整体流程

```
┌─────────────────────────────────────────────────────────┐
│                     TOTP 生成流程                       │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  1. 获取当前 Unix 时间戳（秒）                            │
│     timestamp = time.Now().Unix()                       │
│                                                         │
│  2. 计算时间计数器                                       │
│     counter = floor(timestamp / period)                 │
│     （period 为时间步长，默认 30 秒）                     │
│                                                         │
│  3. 将计数器转换为 8 字节大端序整数                        │
│     counter_bytes = BigEndian(counter, 8)               │
│                                                         │
│  4. 使用共享密钥计算 HMAC                                │
│     hmac_result = HMAC-SHA1(secret, counter_bytes)      │
│     （支持 SHA1 / SHA256 / SHA512）                      │
│                                                         │
│  5. 动态截断（Dynamic Truncation）                       │
│     offset = hmac_result[last_byte] & 0x0F              │
│     binary_code = (hmac_result[offset:offset+4]         │
│                    按大端序组合成 32 位整数)              │
│     binary_code &= 0x7FFFFFFF （清除最高位，避免符号位）  │
│                                                         │
│  6. 取模得到 N 位数字密码                                 │
│     otp = binary_code % (10 ^ digits)                   │
│                                                         │
│  7. 格式化为指定位数，前面补零                            │
│     result = fmt.Sprintf("%0*d", digits, otp)           │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### 4.2 HOTP 核心算法（RFC 4226）

HOTP 是 TOTP 的基础，算法定义如下：

```
HOTP(K, C) = Truncate(HMAC-SHA-1(K, C))
```

其中：
- `K` 为共享密钥（字节数组）
- `C` 为 8 字节的计数器值（大端序）
- `Truncate` 为动态截断函数

**动态截断详细步骤**:
1. 取 HMAC 结果的最后一个字节的低 4 位作为偏移量 `offset`
2. 从 `offset` 位置开始取 4 个字节
3. 将这 4 个字节按大端序组合成 32 位无符号整数
4. 将最高位清零（与 0x7FFFFFFF 按位与），避免符号位问题
5. 对 10^digits 取模得到 N 位数字

### 4.3 TOTP 时间计算（RFC 6238）

```
T = floor((current_unix_time - T0) / X)
```

其中：
- `T0` 为起始时间，通常为 0（Unix 纪元）
- `X` 为时间步长（Period），默认 30 秒
- `T` 即为传入 HOTP 的计数器值

### 4.4 密码校验流程

```
ValidateCode(secret, code):
    1. 解码 Base32 密钥
    2. 计算当前时间计数器
    3. 对于 drift 范围内的每个偏移量 offset:
       - counter = current_counter + offset
       - 生成该 counter 对应的 HOTP 密码
       - 使用恒定时间比较用户输入与生成的密码
       - 如果匹配，返回 valid=true
    4. 如果所有窗口都不匹配，返回 valid=false
```

**安全说明**:
- 使用 `crypto/hmac.Equal` 进行恒定时间字符串比较
- 防止时序攻击（Timing Attack）
- 即使在漂移窗口内，也不会泄露密码信息

---

## 5. 时间窗口漂移容忍机制

### 5.1 问题背景

由于客户端和服务端的系统时钟可能存在一定偏差，如果仅校验当前时间窗口的密码，可能会导致合法用户的密码被错误拒绝。

### 5.2 漂移容忍原理

通过校验多个时间窗口的密码来容忍时钟偏差：

```
时间轴:
  过去                          现在                          未来
    |-------|-------|-------|-------|-------|-------|-------|
     T-3    T-2    T-1     T     T+1    T+2    T+3

当 DriftWindows = 1 时，校验 T-1, T, T+1 三个窗口
当 DriftWindows = 2 时，校验 T-2, T-1, T, T+1, T+2 五个窗口
```

### 5.3 配置选项

- `DriftWindows = 0`: 仅校验当前窗口（最严格，无时差容忍）
- `DriftWindows = 1`: 校验前一窗口、当前窗口、后一窗口（默认，推荐）
- `DriftWindows = 2`: 校验前后各两个窗口（更宽松，适合时差较大场景）

### 5.4 安全权衡

| 漂移窗口数 | 有效窗口数 | 容忍时差 | 安全性 | 推荐场景 |
|-----------|-----------|----------|--------|----------|
| 0 | 1 | ±0 秒 | 最高 | 对安全性要求极高，时钟同步有保障 |
| 1 | 3 | ±30 秒 | 高 | 大多数场景的默认选择 |
| 2 | 5 | ±60 秒 | 中 | 客户端时钟可能偏差较大的场景 |
| 3 | 7 | ±90 秒 | 较低 | 不推荐，仅在特殊场景使用 |

**注意**: 更大的漂移窗口意味着更长的密码有效期，会降低安全性。建议保持默认值 1，仅在确有时钟同步问题时适当增大。

---

## 6. 密钥 Base32 编码存储

### 6.1 为什么使用 Base32

- **标准兼容**: Google Authenticator、Authy 等主流认证器应用均使用 Base32 编码的密钥
- **人类可读**: Base32 使用大写字母和数字 2-7，易于人工输入和识别
- **大小写不敏感**: 便于用户输入，不区分大小写
- **URL 安全**: Base32 字符集不包含特殊字符，适合在 URL 中传递

### 6.2 Base32 字符集

标准 Base32 使用以下 32 个字符：

```
A B C D E F G H I J K L M N O P Q R S T U V W X Y Z 2 3 4 5 6 7
```

### 6.3 密钥生成流程

```
1. 使用 crypto/rand 生成指定长度的随机字节（默认 20 字节）
2. 使用 Base32 标准编码转换为字符串
3. 移除填充字符（=），使密钥更简洁
4. 返回 Base32 编码的密钥字符串
```

### 6.4 密钥解码流程

```
1. 去除首尾空白字符
2. 转换为大写字母
3. 去除已有的填充字符（=）
4. 计算并补充正确的填充字符
5. 使用 Base32 标准解码为原始字节
6. 验证解码结果非空
```

**容错处理**:
- 自动处理大小写：输入小写字母自动转为大写
- 自动处理填充：无论输入是否带填充都能正确解码
- 自动去除空白：首尾空格不影响解码结果

---

## 7. 备用恢复码管理

### 7.1 设计目的

当用户无法获取 TOTP 密码时（如手机丢失、认证器应用损坏），可以使用备用恢复码完成认证。恢复码是一次性使用的，每个码只能使用一次。

### 7.2 恢复码格式

- 使用 Base32 字符集（A-Z, 2-7）
- 默认长度 16 个字符
- 示例格式: `ABCDEFGHIJKLMNOP`
- 使用安全随机数生成

### 7.3 恢复码生命周期

```
生成 → 未使用 → 使用 → 已失效
         ↑
         |
      重新生成（清空所有旧码）
```

### 7.4 使用规则

1. **一次性使用**: 每个恢复码只能使用一次，使用后立即失效
2. **用完提醒**: 当最后一个恢复码被使用时，返回 `ErrNoRecoveryCodes` 警告
3. **重新生成**: 恢复码用完后可调用 `Regenerate()` 生成一组新的
4. **顺序无关**: 恢复码没有使用顺序限制，可以任意使用
5. **不重复生成**: 重新生成会清除所有旧码，无论是否使用过

### 7.5 安全特性

- 使用 `crypto/rand` 安全随机生成
- 存储在内存中，不做持久化（由上层应用负责持久化）
- 验证操作是原子性的，并发下不会重复使用同一码
- 不存储明文哈希，恢复码本身即密钥（请妥善保存）

---

## 8. 使用示例

### 8.1 基本使用：创建 TOTP 认证器

```go
package main

import (
    "fmt"
    "solocoder-go/internal/totpauth"
)

func main() {
    // 使用默认配置
    totp, err := totpauth.NewTOTP()
    if err != nil {
        panic(err)
    }
    
    // 生成新密钥
    secret, err := totp.GenerateSecret()
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("共享密钥（Base32）: %s\n", secret)
    fmt.Println("请将此密钥配置到您的认证器应用中")
    
    // 生成当前密码
    code, err := totp.GenerateCode(secret)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("当前 TOTP 密码: %s\n", code)
}
```

### 8.2 自定义配置

```go
func createCustomTOTP() (*totpauth.TOTP, error) {
    cfg := totpauth.Config{
        Digits:       8,                    // 8 位密码
        Period:       60,                   // 60 秒有效期
        DriftWindows: 2,                    // 前后各容忍 2 个窗口
        Algorithm:    totpauth.SHA256,      // 使用 SHA-256 算法
        SecretSize:   32,                   // 32 字节密钥
    }
    
    return totpauth.NewTOTPWithConfig(cfg)
}
```

### 8.3 用户登录时的密码校验

```go
func verifyTOTP(totp *totpauth.TOTP, secret, userCode string) bool {
    valid, err := totp.ValidateCode(secret, userCode)
    if err != nil {
        // 处理错误（如密钥格式错误）
        fmt.Printf("校验出错: %v\n", err)
        return false
    }
    
    if valid {
        fmt.Println("密码正确，认证成功！")
        return true
    } else {
        fmt.Println("密码错误，认证失败")
        return false
    }
}
```

### 8.4 指定时间校验（用于测试或调试）

```go
func verifyAtTime() {
    totp, err := totpauth.NewTOTP()
    if err != nil {
        panic(err)
    }
    secret := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
    
    // 生成指定时间的密码
    testTime := time.Unix(1111111109, 0)
    code, _ := totp.GenerateCodeAt(secret, testTime)
    fmt.Printf("指定时间的密码: %s\n", code)
    
    // 校验指定时间的密码
    valid, _ := totp.ValidateCodeAt(secret, code, testTime)
    fmt.Printf("校验结果: %v\n", valid)
}
```

### 8.5 备用恢复码完整流程

```go
func recoveryCodeExample() {
    store := totpauth.NewRecoveryCodeStore()
    
    // 生成 10 个备用恢复码
    codes, err := store.Generate(10)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("请妥善保存以下恢复码（每个仅能使用一次）：")
    for i, code := range codes {
        fmt.Printf("  %d. %s\n", i+1, code)
    }
    
    fmt.Printf("总恢复码数: %d\n", store.Total())
    fmt.Printf("剩余可用数: %d\n", store.Remaining())
    
    // 用户使用恢复码登录
    userInputCode := codes[2] // 假设用户输入第三个恢复码
    
    valid, err := store.Validate(userInputCode)
    if err != nil {
        if errors.Is(err, totpauth.ErrCodeUsed) {
            fmt.Println("该恢复码已使用过，请使用其他恢复码")
        } else if errors.Is(err, totpauth.ErrRecoveryNotFound) {
            fmt.Println("恢复码无效")
        } else if errors.Is(err, totpauth.ErrNoRecoveryCodes) {
            fmt.Println("登录成功！注意：这是最后一个恢复码，请重新生成新的恢复码")
        }
    } else if valid {
        fmt.Println("恢复码验证成功，登录成功")
    }
    
    fmt.Printf("剩余恢复码: %d / %d\n", store.Remaining(), store.Total())
    
    // 检查恢复码是否全部用完
    if store.AllUsed() {
        fmt.Println("⚠️  所有恢复码已用完，请重新生成！")
        
        // 重新生成一组新的恢复码
        newCodes, err := store.Regenerate(10)
        if err != nil {
            panic(err)
        }
        fmt.Println("已生成新的恢复码：")
        for _, c := range newCodes[:3] {
            fmt.Printf("  %s...\n", c[:8])
        }
    }
}
```

### 8.6 完整的双因素认证流程

```go
func twoFactorAuth(totp *totpauth.TOTP, recoveryStore *totpauth.RecoveryCodeStore, 
    secret, userCode string) (bool, error) {
    
    // 首先尝试 TOTP 密码
    valid, err := totp.ValidateCode(secret, userCode)
    if err != nil {
        return false, fmt.Errorf("TOTP 校验错误: %w", err)
    }
    
    if valid {
        return true, nil
    }
    
    // TOTP 密码不对，尝试作为恢复码验证
    valid, err = recoveryStore.Validate(userCode)
    if err != nil {
        if errors.Is(err, totpauth.ErrCodeUsed) {
            return false, fmt.Errorf("恢复码已使用")
        }
        if errors.Is(err, totpauth.ErrRecoveryNotFound) {
            return false, fmt.Errorf("密码或恢复码无效")
        }
        if errors.Is(err, totpauth.ErrNoRecoveryCodes) {
            // 恢复码有效且是最后一个，返回成功但带警告
            fmt.Println("警告：最后一个恢复码已使用，请重新生成")
            return true, nil
        }
        return false, err
    }
    
    return valid, nil
}
```

### 8.7 列出所有恢复码状态

```go
func listRecoveryCodes(store *totpauth.RecoveryCodeStore) {
    codes := store.List()
    
    fmt.Println("恢复码状态：")
    for i, rc := range codes {
        status := "可用"
        if rc.Used {
            status = fmt.Sprintf("已使用 (%s)", rc.UsedAt.Format("2006-01-02 15:04:05"))
        }
        // 实际应用中不应明文显示恢复码，这里仅作示例
        masked := rc.Code[:4] + "****" + rc.Code[len(rc.Code)-4:]
        fmt.Printf("  %d. %s - %s\n", i+1, masked, status)
    }
    
    fmt.Printf("\n总计: %d 个，可用: %d 个\n", store.Total(), store.Remaining())
}
```

---

## 9. 错误定义

| 错误变量 | 含义 | 触发场景 |
|----------|------|----------|
| `ErrInvalidSecret` | 密钥无效 | 传入空密钥、非 Base32 编码、解码失败 |
| `ErrInvalidCode` | 密码为空 | 校验时空字符串密码 |
| `ErrInvalidConfig` | 配置无效 | 漂移窗口数为负等配置错误 |
| `ErrInvalidDigits` | 密码位数无效 | Digits 不在 6-8 范围内 |
| `ErrInvalidPeriod` | 时间步长无效 | Period 小于等于 0 |
| `ErrInvalidSecretSize` | 密钥字节数无效 | SecretSize 小于等于 0 |
| `ErrCodeUsed` | 恢复码已使用 | 对已使用的恢复码再次调用 Validate |
| `ErrNoRecoveryCodes` | 无可用恢复码 | 最后一个恢复码被使用时返回警告 |
| `ErrRecoveryCodeEmpty` | 恢复码为空 | 传入空字符串作为恢复码 |
| `ErrRecoveryNotFound` | 恢复码不存在 | 传入的恢复码不在存储中 |

---

## 10. 并发安全设计

### 10.1 并发安全保证

模块采用以下机制保证并发安全：

| 组件 | 同步机制 | 说明 |
|------|---------|------|
| TOTP | 无锁 | 配置创建后只读，纯函数计算，天然线程安全 |
| RecoveryCodeStore | `sync.RWMutex` | 读写锁保护恢复码存储，多读单写 |

### 10.2 TOTP 的线程安全性

`TOTP` 结构体是**完全线程安全**的，因为：
- 配置参数在创建时设置，之后只读
- 所有方法都是纯函数，不修改内部状态
- 可以在多个 goroutine 中并发调用，无需额外同步

### 10.3 RecoveryCodeStore 的并发安全性

`RecoveryCodeStore` 使用 `sync.RWMutex` 保护：
- **读取操作**（`IsUsed`, `Remaining`, `Total`, `List`, `AllUsed`）: 共享读锁，可并发执行
- **写入操作**（`Generate`, `Validate`, `Regenerate`）: 独占写锁，串行执行

### 10.4 原子性保证

恢复码的验证操作是原子的：
- 在同一个锁保护范围内完成"检查是否已使用"和"标记为已使用"
- 高并发下不会出现同一恢复码被多次使用的情况

### 10.5 并发测试覆盖

测试套件包含以下并发场景：
- 50 并发 `Validate` 操作：验证原子性和正确性
- 100 并发 `GenerateCode` + `ValidateCode` 混合操作
- 混合读写并发压力测试

---
