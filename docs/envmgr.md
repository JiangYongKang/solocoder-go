# 环境变量管理器（envmgr）模块需求文档

## 1. 模块概述

envmgr 是一个功能完善的环境变量管理模块，提供前缀分组、类型自动转换、必填项校验和敏感变量加密存储等核心功能。该模块位于 `internal/envmgr/` 包下，旨在简化 Go 应用程序中环境变量的管理和使用。

## 2. 核心功能

### 2.1 前缀分组读取

支持按指定前缀过滤环境变量，将具有相同前缀的环境变量归为一组。读取时自动去掉前缀，以键值对形式返回该组所有变量。

**功能说明：**
- 调用 `LoadGroup(prefix string, configs ...*EnvConfig)` 方法加载指定前缀的环境变量组
- 前缀为空字符串时匹配所有环境变量
- 非指定前缀的环境变量将被自动过滤
- 组内变量名自动去除前缀部分

### 2.2 类型自动转换

支持将环境变量的字符串值自动转换为目标 Go 类型，包括以下常用类型：
- `string`：直接返回原始字符串
- `int` / `int64`：整数类型转换
- `float64`：浮点数类型转换
- `bool`：布尔类型转换（支持 true/false、1/0、TRUE/FALSE 等格式）
- `time.Duration`：时间间隔转换（支持 "30s"、"5m"、"1h" 等格式）

**方法列表：**
- `GetString(key string) (string, error)`
- `GetInt(key string) (int, error)`
- `GetInt64(key string) (int64, error)`
- `GetFloat64(key string) (float64, error)`
- `GetBool(key string) (bool, error)`
- `GetDuration(key string) (time.Duration, error)`

转换失败时返回明确的类型错误，包含具体的键名和转换失败原因。

### 2.3 必填项校验

支持标记某些环境变量为必填项，在读取时检查必填项是否存在且非空。

**校验规则：**
- 标记为 `Required: true` 的变量必须存在且值（去除首尾空格后）非空
- 缺失或为空的必填项将直接返回错误
- 错误信息列出所有缺失的变量名
- 标记为 `Required: true` 但同时提供了 `Default` 值的变量，在缺失时自动使用默认值而不报错

### 2.4 敏感变量加密存储

支持标记某些环境变量为敏感变量，读取后自动对变量值进行 AES 对称加密存储于内存中。

**安全机制：**
- 使用 AES-256-GCM 加密算法
- 每个敏感值使用独立的随机 12 字节 nonce
- 加密密钥在 EnvManager 初始化时随机生成（32 字节），也可通过 `NewEnvManagerWithKey` 指定
- 外部只能通过 `GetSensitive()` 接口获取加密后的 `SensitiveValue` 对象
- 必须调用 `SensitiveValue.Decrypt()` 方法才能获取明文值
- 直接调用 `Get()` / `GetString()` 等方法读取敏感变量将返回错误，无论该值来自环境变量还是默认值
- `GetAll()` 方法对敏感变量返回 "[ENCRYPTED]" 占位符
- 敏感变量的默认值同样会被加密存储，确保不存在明文绕过路径

**加密密钥管理：**
- `NewEnvManager()` 在初始化时使用 `crypto/rand` 生成 32 字节随机密钥
- `NewEnvManagerWithKey(key []byte)` 允许传入外部密钥，密钥长度必须为 32 字节
- 密钥存储在 `EnvManager.aesKey` 中，通过 `SetSensitiveKey` 方法在获取敏感值时注入到 `SensitiveValue` 对象
- `SensitiveValue` 自身不持有有效密钥，只有在调用 `GetSensitive` 时由管理器注入，确保密钥访问受控

## 3. 核心结构体与职责

### 3.1 EnvManager

**职责：** 环境变量管理器的核心入口，负责管理多个环境变量组和加密密钥。

```go
type EnvManager struct {
    groups    map[string]*EnvGroup  // 按前缀存储的环境变量组
    aesKey    []byte                // AES-256 加密密钥（32 字节）
    envSource func() []string       // 环境变量源函数（默认 os.Environ）
    mu        sync.RWMutex          // 读写锁，保证并发安全
}
```

**主要方法：**
- `NewEnvManager() (*EnvManager, error)`：创建新的环境变量管理器，自动生成加密密钥
- `NewEnvManagerWithKey(key []byte) (*EnvManager, error)`：使用指定密钥创建管理器
- `LoadGroup(prefix string, configs ...*EnvConfig) (*EnvGroup, error)`：加载指定前缀的环境变量组
- `GetSensitive(group *EnvGroup, key string) (*SensitiveValue, error)`：获取加密的敏感变量值
- `GetGroup(prefix string) (*EnvGroup, bool)`：获取已加载的环境变量组

### 3.2 EnvGroup

**职责：** 代表一个前缀分组的环境变量集合，提供类型安全的访问方法。

```go
type EnvGroup struct {
    prefix string                    // 分组前缀
    values map[string]string         // 环境变量键值对（敏感变量已加密）
    config map[string]*EnvConfig     // 变量配置信息
    mu     sync.RWMutex              // 读写锁
}
```

**主要方法：**
- `Get(key string) (string, error)`：获取原始字符串值（敏感变量不可用，始终返回错误）
- `GetString(key string) (string, error)`：获取字符串类型值
- `GetInt(key string) (int, error)`：获取整数类型值
- `GetInt64(key string) (int64, error)`：获取 64 位整数类型值
- `GetFloat64(key string) (float64, error)`：获取浮点数类型值
- `GetBool(key string) (bool, error)`：获取布尔类型值
- `GetDuration(key string) (time.Duration, error)`：获取时间间隔类型值
- `GetAll() map[string]string`：获取所有变量（敏感变量显示为 [ENCRYPTED]）
- `Exists(key string) bool`：检查变量是否存在
- `Prefix() string`：获取分组前缀

### 3.3 EnvConfig

**职责：** 单个环境变量的配置信息，用于定义变量的属性。

```go
type EnvConfig struct {
    Key       string  // 变量名（去除前缀后的名称）
    Required  bool    // 是否为必填项
    Sensitive bool    // 是否为敏感变量（需要加密存储）
    Default   string  // 默认值（可选）
}
```

### 3.4 SensitiveValue

**职责：** 封装加密后的敏感变量值，提供安全的解密接口。

```go
type SensitiveValue struct {
    ciphertext []byte          // 加密后的密文
    nonce      []byte          // AES-GCM nonce（12 字节）
    key        []byte          // AES 密钥（仅内部使用，不对外暴露）
    mu         sync.RWMutex    // 读写锁
}
```

**主要方法：**
- `Decrypt() (string, error)`：解密并返回明文值

## 4. 工作机制

### 4.1 环境变量分组机制

```
系统环境变量
    ├─ APP_NAME=myapp
    ├─ APP_PORT=8080
    ├─ APP_DEBUG=true
    ├─ DB_HOST=localhost
    ├─ DB_PORT=5432
    └─ DB_PASSWORD=secret123

LoadGroup("APP_") 调用时：
    1. 过滤出所有以 "APP_" 开头的变量
    2. 去除前缀得到键名：NAME, PORT, DEBUG
    3. 存储为 EnvGroup：{NAME: "myapp", PORT: "8080", DEBUG: "true"}

LoadGroup("DB_") 调用时：
    1. 过滤出所有以 "DB_" 开头的变量
    2. 去除前缀得到键名：HOST, PORT, PASSWORD
    3. 对标记为敏感的 PASSWORD 进行 AES 加密
    4. 存储为 EnvGroup：{HOST: "localhost", PORT: "5432", PASSWORD: "[加密值]"}
```

### 4.2 加密存储机制

```
敏感变量加密流程：
    1. EnvManager 初始化时生成 32 字节 AES 密钥
    2. 加载环境变量组时，检查变量是否标记为 Sensitive
    3. 对标记为 Sensitive 且配置了 Default 但环境变量未设置的变量，
       将默认值填入 values 映射，确保默认值也经过加密
    4. 为每个敏感值生成随机 12 字节 nonce
    5. 使用 AES-256-GCM 算法加密明文值
    6. 将 nonce + ciphertext 进行 Base64 编码后存储

敏感变量解密流程：
    1. 调用 GetSensitive(group, key) 获取 SensitiveValue 对象
    2. 解析 Base64 编码的加密数据，分离 nonce 和 ciphertext
    3. 从 EnvManager 注入 AES 密钥到 SensitiveValue
    4. 调用 Decrypt() 方法时，使用 AES-256-GCM 解密获取明文

内存安全特性：
    - 敏感值在内存中始终以加密形式存在
    - 敏感变量的默认值同样被加密，不存在明文绕过路径
    - Get 方法优先检查 Sensitive 标记，即使值来自默认值也拦截直接读取
    - AES 密钥存储在 EnvManager 中，不直接暴露
    - SensitiveValue 无法直接访问密文或密钥
    - 解密后的明文由调用方负责管理
```

### 4.3 必填校验机制

```
LoadGroup 时校验流程：
    1. 遍历所有 EnvConfig
    2. 对标记为 Required: true 的变量进行检查
    3. 检查变量是否存在且值（去除空格后）非空
    4. 若缺失或为空，但配置了 Default 值，则使用默认值
    5. 若缺失或为空且无 Default 值，将变量名加入缺失列表
    6. 所有必填项检查完成后，若存在缺失则返回错误
    7. 错误信息包含所有缺失的变量名，便于一次性修复
```

## 5. 使用示例

### 5.1 基本使用

```go
package main

import (
    "fmt"
    "solocoder-go/internal/envmgr"
)

func main() {
    // 创建环境变量管理器
    mgr, err := envmgr.NewEnvManager()
    if err != nil {
        panic(err)
    }

    // 加载 APP_ 前缀的环境变量组
    configs := []*envmgr.EnvConfig{
        {Key: "NAME", Required: true},
        {Key: "PORT", Required: true},
        {Key: "DEBUG", Required: false, Default: "false"},
    }

    group, err := mgr.LoadGroup("APP_", configs...)
    if err != nil {
        panic(fmt.Sprintf("加载环境变量失败: %v", err))
    }

    // 类型安全地读取变量
    name, _ := group.GetString("NAME")
    port, _ := group.GetInt("PORT")
    debug, _ := group.GetBool("DEBUG")

    fmt.Printf("应用名称: %s\n", name)
    fmt.Printf("监听端口: %d\n", port)
    fmt.Printf("调试模式: %v\n", debug)
}
```

### 5.2 敏感变量使用

```go
// 加载数据库配置，其中 PASSWORD 标记为敏感
dbConfigs := []*envmgr.EnvConfig{
    {Key: "HOST", Required: true},
    {Key: "PORT", Required: true},
    {Key: "USER", Required: true},
    {Key: "PASSWORD", Required: true, Sensitive: true},
}

dbGroup, err := mgr.LoadGroup("DB_", dbConfigs...)
if err != nil {
    panic(err)
}

// 直接读取敏感变量会失败
_, err = dbGroup.GetString("PASSWORD")
if err != nil {
    fmt.Println("无法直接读取敏感变量:", err)
}

// 通过 GetSensitive 获取加密值后解密
sv, err := mgr.GetSensitive(dbGroup, "PASSWORD")
if err != nil {
    panic(err)
}

password, err := sv.Decrypt()
if err != nil {
    panic(err)
}

fmt.Printf("数据库密码已解密，长度: %d\n", len(password))
```

### 5.3 敏感变量带默认值

```go
// 敏感变量配置默认值时，默认值也会被加密存储
// 无论环境变量是否设置，都无法通过 Get 直接获取明文
configs := []*envmgr.EnvConfig{
    {Key: "SECRET", Required: false, Sensitive: true, Default: "changeme"},
}

group, err := mgr.LoadGroup("APP_", configs...)
if err != nil {
    panic(err)
}

// 环境变量未设置时，默认值 "changeme" 会被加密存储
// 直接读取仍然被拦截
_, err = group.Get("SECRET")
if err != nil {
    fmt.Println("敏感变量不可直接读取:", err)
}

// 只能通过解密接口获取明文（此时返回默认值 "changeme"）
sv, err := mgr.GetSensitive(group, "SECRET")
if err != nil {
    panic(err)
}

plaintext, err := sv.Decrypt()
if err != nil {
    panic(err)
}

fmt.Printf("解密后的值: %s\n", plaintext) // 输出: changeme
```

### 5.4 多分组管理

```go
// 同时管理多个配置组
appGroup, _ := mgr.LoadGroup("APP_", appConfigs...)
dbGroup, _ := mgr.LoadGroup("DB_", dbConfigs...)
cacheGroup, _ := mgr.LoadGroup("CACHE_", cacheConfigs...)

// 获取已加载的组
if group, ok := mgr.GetGroup("DB_"); ok {
    host, _ := group.GetString("HOST")
    fmt.Println("数据库主机:", host)
}
```

### 5.5 完整的错误处理

```go
group, err := mgr.LoadGroup("APP_", configs...)
if err != nil {
    // 检查错误类型
    if errors.Is(err, envmgr.ErrMissingRequired) {
        fmt.Printf("缺少必填环境变量: %v\n", err)
        // 可以解析错误信息获取具体缺失的变量名
        return
    }
    panic(err)
}

// 类型转换错误处理
port, err := group.GetInt("PORT")
if err != nil {
    if errors.Is(err, envmgr.ErrInvalidType) {
        fmt.Printf("类型转换失败: %v\n", err)
    } else if errors.Is(err, envmgr.ErrKeyNotFound) {
        fmt.Printf("变量不存在: %v\n", err)
    }
    return
}
```

## 6. 错误定义

| 错误常量 | 说明 |
|---------|------|
| `ErrMissingRequired` | 缺少必填环境变量 |
| `ErrInvalidType` | 类型转换失败 |
| `ErrEmptyValue` | 变量值为空 |
| `ErrKeyNotFound` | 变量不存在 |
| `ErrInvalidKeySize` | AES 密钥长度不正确（必须 32 字节） |
| `ErrDecryptFailed` | 解密失败 |

## 7. 并发安全

envmgr 模块的所有公共方法都是并发安全的：
- `EnvManager` 使用 `sync.RWMutex` 保护 groups 和 aesKey
- `EnvGroup` 使用 `sync.RWMutex` 保护 values 和 config
- `SensitiveValue` 使用 `sync.RWMutex` 保护内部状态
- 支持多 goroutine 同时读取环境变量
- 支持并发解密敏感变量

## 8. 测试覆盖

模块包含完整的单元测试（`internal/envmgr/envmgr_test.go`），覆盖以下场景：

**正常流程：**
- 管理器初始化（带随机密钥和指定密钥）
- 前缀分组加载（带前缀和无前缀）
- 各类型自动转换（string、int、int64、float64、bool、duration）
- 敏感变量加密存储和解密
- 敏感变量带默认值的加密与解密
- 敏感必填变量带默认值的加密与解密
- 环境变量覆盖敏感变量默认值
- 多分组管理
- 真实 OS 环境变量读取

**边界条件：**
- 零值、负值、极大值的类型转换
- 布尔值的多种格式（true/false、1/0、TRUE/FALSE）
- 空前缀匹配所有变量
- 环境变量值中包含等号
- 大小写敏感的键名
- 无效的环境变量格式（无等号）
- nil 配置项
- 敏感变量无默认值且未设置时不存入 values

**异常分支：**
- 缺少必填变量
- 必填变量值为空
- 无效的类型转换
- 直接读取敏感变量（包括带默认值的情况）
- 获取不存在的变量
- 获取不存在的分组
- 解密时密钥不正确
- 无效的 Base64 编码
- 加密数据过短
- AES 密钥长度不正确

**并发测试：**
- 多 goroutine 并发读取普通变量
- 多 goroutine 并发解密敏感变量

## 9. 变更记录

**v1.1 修复**
- 修复 `Get` 方法在敏感变量设有默认值且环境变量未设置时的明文绕过漏洞：当 `Sensitive=true` 且 `Required=false`、`Default` 非空时，`Get` 方法原先直接返回明文默认值，现已修复为始终拦截敏感变量的直接读取
- 在 `LoadGroup` 中增加敏感变量默认值的预填充逻辑，确保带默认值的敏感变量在加载阶段即被加密
- 在 `Get` 方法中将敏感检查提前至值查找之前，作为防御性措施确保任何路径都无法绕过拦截
