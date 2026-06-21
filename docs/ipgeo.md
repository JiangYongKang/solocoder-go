# IP 地理位置查询引擎模块

## 1. 模块概述

IP 地理位置查询引擎（ipgeo）模块提供基于 CIDR 网段的 IP 归属地查询功能，支持最高效的最长前缀匹配（Longest Prefix Match, LPM）算法，内置内存二分查找索引以加速大规模数据查询，并提供原子热更新机制实现服务不中断的数据刷新。同时支持多语言地区名称返回，满足全球化业务场景需求。

### 1.1 核心功能

| 功能分类 | 功能描述 |
|---------|---------|
| CIDR 归属查询 | 加载 CIDR 网段与地理位置映射数据，执行最长前缀匹配查询 |
| 二分查找索引 | CIDR 按起始 IP 排序后构建内存索引，二分查找快速定位候选网段 |
| 原子热更新 | 后台加载新数据期间不影响线上查询，完成后原子切换索引指针 |
| 多语言支持 | 地区名称（国家/省/市/区/ISP）支持按语言编码返回对应文本 |
| 数据加载 | 支持从字符串数组和数据文件两种数据源加载 IP 库 |

## 2. 核心结构体职责

### 2.1 Engine

```go
type Engine struct {
    currentIdx atomic.Value
    reloadMu   sync.Mutex
}
```

**职责**：IP 地理位置查询引擎核心，对外暴露所有查询和管理接口。
- `currentIdx`：使用 `atomic.Value` 存储当前生效的索引指针，支持无锁读、原子写
- `reloadMu`：热更新互斥锁，确保同一时刻只有一个热更新操作在执行

### 2.2 ipIndex

```go
type ipIndex struct {
    byStartIP []cidrEntry
}
```

**职责**：内存索引结构，存储按起始 IP 排序后的 CIDR 条目数组。
- `byStartIP`：按 `StartIP` 升序排列的 CIDR 条目切片，是二分查找的数据基础

### 2.3 cidrEntry

```go
type cidrEntry struct {
    CIDR       string
    StartIP    uint32
    EndIP      uint32
    PrefixLen  int
    NetworkIP  uint32
    Mask       uint32
    GeoInfo    *GeoInfo
}
```

**职责**：单个 CIDR 网段及其地理位置信息的存储条目。
- `CIDR`：原始 CIDR 字符串（如 "10.0.0.0/8"）
- `StartIP`：网段起始 IP（uint32，大端序），等于 NetworkIP
- `EndIP`：网段结束 IP（uint32，大端序），等于 NetworkIP | ^Mask
- `PrefixLen`：前缀长度（0-32）
- `NetworkIP`：网络地址（IP 与掩码按位与的结果）
- `Mask`：子网掩码（uint32）
- `GeoInfo`：关联的地理位置信息指针

### 2.4 GeoInfo

```go
type GeoInfo struct {
    Country  string
    Province string
    City     string
    District string
    ISP      string
    Names    *LocalizedName
}
```

**职责**：存储单条 CIDR 对应的地理位置信息及其多语言版本。
- `Country/Province/City/District/ISP`：默认语言（中文）字段值
- `Names`：多语言名称集合指针

### 2.5 LocalizedName

```go
type LocalizedName struct {
    Country  map[string]string
    Province map[string]string
    City     map[string]string
    District map[string]string
    ISP      map[string]string
}
```

**职责**：存储五个地理字段的多语言翻译映射。
- 每个字段为 `map[语言编码]本地化文本`，如 `Country["en"] = "China"`

### 2.6 QueryResult

```go
type QueryResult struct {
    Found   bool
    IP      string
    Country string
    Province string
    City     string
    District string
    ISP      string
    Lang    string
}
```

**职责**：查询结果返回结构体。
- `Found`：是否匹配到 CIDR 网段
- `IP`：用户查询的原始 IP 字符串
- `Country/Province/City/District/ISP`：按指定语言返回的字段
- `Lang`：本次查询实际使用的语言编码

## 3. CIDR 最长前缀匹配查找流程

### 3.1 算法说明

最长前缀匹配（Longest Prefix Match, LPM）是 IP 路由和归属查询中的经典问题。当一个 IP 地址同时属于多个 CIDR 网段时（例如 10.1.2.3 同时属于 10.0.0.0/8、10.1.0.0/16、10.1.2.0/24、10.1.2.3/32），应当选择前缀长度最大（即最具体）的那个网段的归属信息。

### 3.2 查找流程

```
Query(ip, lang)
    │
    ▼
1. 参数合法性校验
    ├─► IP 为空或格式非法 → 返回 ErrInvalidIP
    └─► IPv6 地址 → 返回 ErrInvalidIP（本模块仅支持 IPv4）
    │
    ▼
2. 原子获取当前索引
    └─► 引擎未加载数据 → 返回 ErrEngineNotReady
    │
    ▼
3. IP 地址转 uint32（大端序）
    │
