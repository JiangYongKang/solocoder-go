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
    ▼
4. 二分查找定位候选起点
    │
    │   使用 sort.Search 在排序后的 byStartIP 切片中：
    │   找到第一个 StartIP > targetIP 的位置 pos
    │   真实候选起点 = pos - 1
    │
    ▼
5. 向前（索引递减方向）遍历候选网段
    │
    │   从候选起点开始，向切片头部遍历：
    │   ├─► 如果当前条目的 PrefixLen ≤ 已找到最佳前缀长度，跳过
    │   ├─► 检查 targetIP 是否在 [StartIP, EndIP] 范围内
    │   │     └─► 在范围内 → 精确验证 (targetIP & Mask) == NetworkIP
    │   │           └─► 验证通过 → 更新最佳匹配和最佳前缀长度
    │   │               └─► 前缀长度 == 32 → 已找到最精确匹配，提前退出
    │   └─► 继续向前遍历直到切片头部
    │
    ▼
6. 根据匹配结果组装返回
    ├─► 未匹配到任何网段 → 返回 Found=false，各字段为空
    └─► 已匹配到网段 → 按 lang 参数从多语言映射中取值
        ├─► 精确匹配 lang 编码（如 "zh-CN"）
        ├─► 未找到 → 尝试匹配语言主码（如 "zh"）
        └─► 仍未找到 → 使用默认中文值
```

### 3.3 索引构建流程

```
buildIndex(entries []cidrEntry)
    │
    ▼
对 entries 执行稳定排序：
    主键：StartIP 升序
    次键：PrefixLen 降序（同起始 IP 时，前缀长的排前面）
    │
    ▼
封装为 ipIndex{ byStartIP: sortedEntries } 返回
```

排序后的数据保证：
- 对于任意 i < j，`entries[i].StartIP ≤ entries[j].StartIP`
- 当 `StartIP` 相同时，前缀更长的条目排在前面，便于优先匹配

## 4. 热更新原子切换机制

### 4.1 设计目标

IP 库数据会周期性更新（例如每日或每小时刷新），热更新机制需保证：
1. **服务不中断**：加载新数据期间，旧索引继续响应所有查询请求
2. **原子切换**：新索引完全就绪后，查询端立即看到新数据，无中间状态
3. **失败隔离**：新数据加载或解析失败时，旧索引不受影响
4. **并发安全**：允许多个 goroutine 同时执行查询和一个 goroutine 执行热更新

### 4.2 原子切换流程

```
HotReloadFromData(newData) / HotReloadFromFile(filePath)
    │
    ▼
1. 获取 reloadMu 互斥锁
    └─► 防止多个热更新并发执行
    │
    ▼
2. 在临时变量中完整构建新索引
    ├─► 逐行解析 newData（或从文件读取后解析）
    ├─► 遇到任何解析错误 → 立即返回错误，旧索引完好无损
    ├─► 解析结果为空 → 返回 ErrEmptyData，旧索引完好无损
    └─► 解析成功 → 调用 buildIndex() 构建新的 *ipIndex
    │
    ▼
3. 原子替换索引指针
    │
    │   调用 currentIdx.Store(newIdx)
    │   这是 sync/atomic.Value 的原子写操作：
    │   ├─► 写入前：所有查询 Load 到旧索引
    │   ├─► 写入中：原子指针替换，不可分割
    │   └─► 写入后：所有新的查询 Load 到新索引
    │
    ▼
4. 释放 reloadMu 互斥锁
    │
    ▼
旧索引自动释放
    └─► Go 垃圾回收器在确认无任何 goroutine 引用旧索引后自动回收内存
```

### 4.3 并发安全保证

| 操作 | 同步机制 | 说明 |
|-----|---------|------|
| 查询（读） | `atomic.Value.Load()` | 无锁，高性能，支持任意并发读 |
| 热更新（写） | `reloadMu` + `atomic.Value.Store()` | 写互斥，原子切换，读写不阻塞 |

查询线程不需要加锁，因为 `atomic.Value` 保证了指针读写的原子性和内存可见性。热更新期间正在进行中的查询会继续使用旧索引，而新发起的查询在 `Store` 完成后会立即使用新索引，实现了无缝切换。

## 5. 数据格式规范

### 5.1 文件格式

IP 库数据文件为纯文本格式，每行一条记录，支持 Tab（`\t`）或空格分隔。

**行格式**：
```
CIDR	国家	省份	城市	区县	ISP	[lang:field=value ...]
```

**字段说明**：

| 序号 | 字段名 | 必填 | 说明 |
|-----|--------|------|------|
| 1 | CIDR | 是 | IPv4 CIDR 表示，如 `10.0.0.0/8` |
| 2 | 国家 | 是 | 默认语言（中文）国家名称 |
| 3 | 省份 | 否 | 默认语言省份/州名称 |
| 4 | 城市 | 否 | 默认语言城市名称 |
| 5 | 区县 | 否 | 默认语言区/县名称 |
| 6 | ISP | 否 | 默认语言 ISP 运营商名称 |
| 7+ | 多语言扩展 | 否 | 格式：`语言编码:字段名=翻译值` |

**多语言扩展字段名**：`country`、`province`、`city`、`district`、`isp`

**支持的特殊行**：
- 空行：自动忽略
- 以 `#` 开头的行：视为注释，自动忽略

### 5.2 示例数据

```
# 中国三大私有地址段
10.0.0.0/8	中国	北京	北京	朝阳区	中国电信	en:country=China	en:province=Beijing	en:city=Beijing	en:district=Chaoyang	en:isp=China Telecom
172.16.0.0/12	中国	上海	上海	浦东新区	中国联通	en:country=China	en:province=Shanghai	en:city=Shanghai	en:district=Pudong	en:isp=China Unicom
192.168.0.0/16	中国	广东	深圳	南山区	中国移动	en:country=China	en:province=Guangdong	en:city=Shenzhen	en:district=Nanshan	en:isp=China Mobile

# Google DNS
8.8.8.0/24	美国	加利福尼亚	山景城			Google DNS	en:country=USA	en:province=California	en:city=Mountain View	en:isp=Google DNS
```

## 6. API 接口说明

### 6.1 构造函数

| 方法 | 签名 | 说明 |
|------|------|------|
| 创建引擎 | `NewEngine() *Engine` | 创建空引擎，需后续加载数据 |
| 从数据创建 | `NewEngineFromData(data []string) (*Engine, error)` | 从字符串数组创建并加载数据 |
| 从文件创建 | `NewEngineFromFile(filePath string) (*Engine, error)` | 从数据文件创建并加载数据 |

### 6.2 数据加载

| 方法 | 签名 | 说明 |
|------|------|------|
| 从文件加载 | `LoadFromFile(filePath string) error` | 初始加载或覆盖加载文件数据 |
| 从数据加载 | `LoadFromData(data []string) error` | 初始加载或覆盖加载字符串数据 |
| 热更新文件 | `HotReloadFromFile(filePath string) error` | 原子热更新（失败保留旧数据） |
| 热更新数据 | `HotReloadFromData(data []string) error` | 原子热更新（失败保留旧数据） |

### 6.3 查询接口

| 方法 | 签名 | 说明 |
|------|------|------|
| 默认语言查询 | `Query(ipStr string) (*QueryResult, error)` | 使用默认语言 zh-CN 查询 |
| 指定语言查询 | `QueryWithLang(ipStr, lang string) (*QueryResult, error)` | 按指定语言编码查询 |
| 线性扫描查询 | `LinearQueryWithLang(ipStr, lang string) (*QueryResult, error)` | 全量线性扫描（用于验证二分查找正确性） |

### 6.4 状态检查

| 方法 | 签名 | 说明 |
|------|------|------|
| 条目数量 | `Count() int` | 返回当前索引中的 CIDR 条目数 |
| 就绪状态 | `IsReady() bool` | 返回引擎是否已加载数据可用于查询 |

## 7. 使用示例

### 7.1 基础使用流程

```go
package main

import (
    "fmt"
    "solocoder-go/internal/ipgeo"
)

func main() {
    // 1. 准备 IP 库数据
    data := []string{
        "10.0.0.0/8\t中国\t北京\t北京\t朝阳区\t中国电信\ten:country=China\ten:province=Beijing",
        "172.16.0.0/12\t中国\t上海\t上海\t浦东新区\t中国联通\ten:country=China\ten:province=Shanghai",
        "192.168.0.0/16\t中国\t广东\t深圳\t南山区\t中国移动\ten:country=China\ten:province=Guangdong",
    }

    // 2. 创建引擎并加载数据
    engine, err := ipgeo.NewEngineFromData(data)
    if err != nil {
        panic(fmt.Sprintf("创建引擎失败: %v", err))
    }

    // 3. 使用默认语言（中文）查询
    result, err := engine.Query("10.1.2.3")
    if err != nil {
        panic(fmt.Sprintf("查询失败: %v", err))
    }
    if result.Found {
        fmt.Printf("IP: %s\n", result.IP)
        fmt.Printf("归属: %s %s %s %s\n", result.Country, result.Province, result.City, result.District)
        fmt.Printf("运营商: %s\n", result.ISP)
    } else {
        fmt.Println("未找到该 IP 的归属信息")
    }

    // 4. 使用英文查询
    resultEn, err := engine.QueryWithLang("172.16.5.100", "en")
    if err != nil {
        panic(fmt.Sprintf("英文查询失败: %v", err))
    }
    if resultEn.Found {
        fmt.Printf("\n[English]\n")
        fmt.Printf("Location: %s, %s\n", resultEn.City, resultEn.Province)
    }
}
```

### 7.2 从文件加载数据

```go
// 从本地文件加载 IP 库
engine, err := ipgeo.NewEngineFromFile("/data/ipdb/ipv4-china.txt")
if err != nil {
    log.Fatalf("加载 IP 库失败: %v", err)
}
log.Printf("IP 库加载完成，共 %d 条 CIDR 记录", engine.Count())
```

### 7.3 热更新使用场景

```go
// 初始化时加载第一版数据
engine, _ := ipgeo.NewEngineFromFile("ipdb-v1.txt")

// 启动后台热更新协程（例如每小时执行一次）
go func() {
    ticker := time.NewTicker(1 * time.Hour)
    defer ticker.Stop()

    for range ticker.C {
        // 下载最新 IP 库到临时文件
        newFile := downloadLatestIPDB()

        // 原子热更新
        err := engine.HotReloadFromFile(newFile)
        if err != nil {
            log.Printf("热更新失败: %v，继续使用旧数据", err)
            continue
        }
        log.Printf("热更新成功，当前条目数: %d", engine.Count())
    }
}()

// 主业务线程持续查询，不受热更新影响
for {
    result, _ := engine.Query(clientIP)
    handleQueryResult(result)
}
```

### 7.4 最长前缀匹配验证

```go
// 构造包含重叠网段的测试数据
data := []string{
    "10.0.0.0/8\t大网\t通用\t通用\t通用\t通用",
    "10.1.0.0/16\t中网\t省级\t市级\t区级\tISP1",
    "10.1.2.0/24\t小网\t深圳\t深圳\t南山\tISP2",
    "10.1.2.3/32\t单主机\t专属\t专属\t专属\t专属ISP",
}

engine, _ := ipgeo.NewEngineFromData(data)

// 测试不同精度匹配
tests := []string{"10.2.3.4", "10.1.3.4", "10.1.2.4", "10.1.2.3"}
for _, ip := range tests {
    result, _ := engine.Query(ip)
    fmt.Printf("%-15s -> %s (前缀长度决定精度)\n", ip, result.Country)
}
// 输出:
// 10.2.3.4        -> 大网 (匹配 /8)
// 10.1.3.4        -> 中网 (匹配 /16)
// 10.1.2.4        -> 小网 (匹配 /24)
// 10.1.2.3        -> 单主机 (匹配 /32，最长前缀)
```

## 8. 错误定义

| 错误变量 | 错误信息 | 触发场景 |
|---------|---------|---------|
| `ErrInvalidIP` | ipgeo: invalid IP address | IP 字符串为空、格式非法、为 IPv6 |
| `ErrInvalidCIDR` | ipgeo: invalid CIDR notation | CIDR 格式不合法（如 "999.1.1.1/24"） |
| `ErrEmptyCIDR` | ipgeo: empty CIDR | CIDR 字段为空字符串 |
| `ErrNotFound` | ipgeo: IP not found in any CIDR range | 预留，暂未直接使用（通过 Found 字段返回） |
| `ErrInvalidDataFormat` | ipgeo: invalid data format in file | 数据行字段不足或文件读失败 |
| `ErrFileNotExist` | ipgeo: data file does not exist | 指定的数据文件路径不存在或为空字符串 |
| `ErrEmptyData` | ipgeo: empty IP database | 解析后无有效 CIDR 条目（全是空行/注释） |
| `ErrEngineNotReady` | ipgeo: engine is not ready (no data loaded) | 未加载数据就发起查询 |
| `ErrUnsupportedLang` | ipgeo: unsupported language code | 预留（当前通过降级机制处理） |

## 9. 性能设计要点

### 9.1 时间复杂度

| 操作 | 平均复杂度 | 最坏复杂度 | 说明 |
|-----|-----------|-----------|------|
| 索引构建 | O(n log n) | O(n log n) | 排序开销 |
| 单次查询 | O(log n + k) | O(n) | k=前缀分布相关，实际接近 O(log n) |
| 热更新 | O(n log n) | O(n log n) | 构建新索引 + 原子指针切换 |

> 注：最坏 O(n) 发生在极端场景（例如目标 IP 属于 0.0.0.0/0 这一超大网段时需遍历到头部）。通过 `PrefixLen ≤ bestPrefixLen` 跳过优化可有效减少实际遍历次数。

### 9.2 内存占用

每条 CIDR 条目内存开销约为：
- `cidrEntry` 结构体：~88 字节（不含字符串和 map）
- 字符串和多语言 map：视实际内容约 100-500 字节/条
- 百万级 CIDR 条目预计占用 **数百 MB 到 1GB 内存**

### 9.3 并发性能

- **查询吞吐量**：查询无锁设计，可线性扩展至 CPU 核心数
- **热更新影响**：对查询零阻塞，仅在原子切换时有纳秒级指针写入开销
- **多读者单写者**：使用 `sync.Mutex` 限制写并发，`atomic.Value` 实现无锁读

## 10. 线程安全

所有公共 API 均为并发安全：
- 查询方法（`Query`、`QueryWithLang`、`LinearQueryWithLang`、`Count`、`IsReady`）：可被任意数量 goroutine 同时调用
- 数据加载/热更新方法（`LoadFromFile`、`LoadFromData`、`HotReloadFromFile`、`HotReloadFromData`）：通过 `reloadMu` 保证串行执行
- 构造函数：返回后的引擎可并发使用（不建议在构造期间并发查询）
