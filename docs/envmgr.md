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
- 加密密钥在 EnvManager 初始化时