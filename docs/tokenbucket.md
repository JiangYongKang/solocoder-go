# Token Bucket 令牌桶限流器

## 模块功能

`internal/tokenbucket` 包实现了一个高性能的令牌桶限流器，支持以下核心功能：

- **动态速率配置**：运行时可修改桶容量和令牌生成速率，立即对新请求生效
- **突发流量允许**：流量低谷期间积累的令牌可被突发流量一次性消耗
- **多维度限流键**：按用户 ID、IP 地址、API 路径等维度独立限流
- **预热模式**：冷启动时令牌生成速率从低到高线性增长
- **Retry-After 计算**：请求被拒时生成预估等待时间

## 核心结构体

### Bucket

`Bucket` 是单个令牌桶，维护令牌数量、容量、生成速率和上次填充时间。

| 字段 | 类型 | 职责 |
|------|------|------|
| `tokens` | `float64` | 当前令牌数 |
| `capacity` | `float64` | 桶容量上限 |
| `rate` | `float64` | 目标令牌生成速率（令牌/秒） |
| `lastRefill` | `time.Time` | 上次填充时间 |
| `warmup` | `bool` | 是否处于预热状态 |
| `warmupStartRate` | `float64` | 预热起始速率 |
| `warmupDuration` | `time.Duration` | 预热持续时间 |
| `warmupStartTime` | `time.Time` | 预热开始时间 |

主要方法：

- `Take(count)` — 尝试消费令牌，返回 `Result`
- `PutBack(count)` — 归还令牌（用于多维度回滚）
- `SetRate(rate)` — 动态修改生成速率
- `SetCapacity(capacity)` — 动态修改桶容量
- `CurrentRate()` — 获取当前有效速率（预热期间可能低于目标速率）
- `IsWarmingUp()` — 是否处于预热状态

### BucketConfig

`BucketConfig` 用于创建令牌桶的配置：

| 字段 | 说明 |
|------|------|
| `Capacity` | 桶容量（必须 > 0） |
| `Rate` | 令牌生成速率，令牌/秒（必须 > 0） |
| `Warmup` | 是否启用预热模式 |
| `WarmupStartRate` | 预热起始速率（必须 > 0 且 < Rate） |
| `WarmupDuration` | 预热持续时间（必须 > 0） |

### Limiter

`Limiter` 管理多个按 key 索引的令牌桶，提供多维度限流能力。

| 方法 | 说明 |
|------|------|
| `Take(key, count)` | 按单个维度消费令牌 |
| `TakeMulti(keys, count)` | 按多个维度消费令牌（任一拒绝则全部回滚） |
| `SetRate(key, rate)` | 修改指定桶的速率 |
| `SetCapacity(key, capacity)` | 修改指定桶的容量 |
| `SetAllRates(rate)` | 批量修改所有桶的速率 |
| `SetAllCapacities(capacity)` | 批量修改所有桶的容量 |
| `Bucket(key)` | 获取指定桶 |
| `Remove(key)` | 移除指定桶 |
| `Keys()` | 获取所有桶的 key 列表 |

### Result

`Result` 是令牌消费操作的返回值：

| 字段 | 说明 |
|------|------|
| `Allowed` | 是否允许（令牌是否充足） |
| `RetryAfter` | 预计等待时间（被拒时有效） |
| `Remaining` | 消费后剩余令牌数 |

方法：

- `RetryAfterSeconds()` — 返回向上取整的等待秒数，适合作为 HTTP `Retry-After` 头值
- `String()` — 可读的字符串表示

## 令牌桶的填充与消费机制

### 填充（Refill）

令牌桶采用**惰性填充**策略：不在后台定时添加令牌，而是在每次 `Take` 或查询操作时计算自上次填充以来应添加的令牌数。

```
新增令牌 = 经过秒数 × 当前有效速率
当前令牌数 = min(当前令牌数 + 新增令牌, 桶容量)
```

惰性填充的优势：
- 无需定时器线程，降低系统开销
- 即使长时间无请求，令牌数也不会超过容量上限
- 精确计算，无定时器漂移问题

### 消费（Take）

1. 先执行填充，更新令牌数
2. 判断当前令牌数是否 ≥ 请求令牌数
3. 若充足，扣减令牌并返回 `Allowed=true`
4. 若不足，不扣减令牌，计算 `RetryAfter` 并返回 `Allowed=false`

```
RetryAfter = (请求令牌数 - 当前令牌数) / 当前有效速率
```

### 突发流量

桶在空闲期间持续积累令牌，最大可达容量上限。当突发流量到来时，可以一次性消耗桶中积攒的所有令牌，允许短时间内的请求量超过平均速率。

示例：容量=100，速率=10/s
- 空闲 10 秒后桶满（100 令牌）
- 突发请求一次消耗 100 令牌 → 允许
- 之后恢复正常 10/s 速率限制

## 预热模式的速率变化曲线

预热模式下，令牌生成速率从 `WarmupStartRate` 线性增长至 `Rate`，持续 `WarmupDuration` 时间。

```
当前速率 = WarmupStartRate + (Rate - WarmupStartRate) × 进度
进度 = 已过预热时间 / WarmupDuration
```

速率变化曲线：

```
速率
Rate  |                        /‾‾‾‾‾‾‾‾‾‾‾‾
      |                    /
      |                /
      |            /
      |        /
      |    /
      | /
Start +----------------------------------------> 时间
      0          WarmupDuration
```

- 预热开始时，速率为 `WarmupStartRate`
- 预热期间，速率线性递增
- 预热结束后，速率稳定在 `Rate`，`IsWarmingUp()` 返回 `false`

## 使用示例

### 基本用法

```go
bucket, err := tokenbucket.NewBucket(tokenbucket.BucketConfig{
    Capacity: 100,
    Rate:     10, // 每秒生成 10 个令牌
})
if err != nil {
    log.Fatal(err)
}

result := bucket.Take(1)
if result.Allowed {
    // 请求通过
} else {
    // 请求被拒，RetryAfter 为预计等待时间
    retrySeconds := result.RetryAfterSeconds()
    w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
    w.WriteHeader(http.StatusTooManyRequests)
}
```

### 多维度限流

```go
limiter := tokenbucket.NewLimiter(tokenbucket.BucketConfig{
    Capacity: 100,
    Rate:     10,
})

// 同一请求受多个维度约束
result, err := limiter.TakeMulti(
    []string{"user:alice", "ip:10.0.0.1", "path:/api/data"},
    1,
)
if err != nil {
    log.Fatal(err)
}
if !result.Allowed {
    // 任一维度限流即拒绝，已消费的令牌自动回滚
    retrySeconds := result.RetryAfterSeconds()
    w.Header().Set("Retry-After", strconv.Itoa(retrySeconds))
    w.WriteHeader(http.StatusTooManyRequests)
    return
}
```

### 动态速率配置

```go
limiter := tokenbucket.NewLimiter(tokenbucket.BucketConfig{
    Capacity: 100,
    Rate:     10,
})

limiter.Take("user:alice", 1)

// 运行时修改单个桶的速率
limiter.SetRate("user:alice", 50)

// 批量修改所有桶的速率
limiter.SetAllRates(50)

// 修改桶容量
limiter.SetCapacity("user:alice", 200)
```

### 预热模式

```go
bucket, err := tokenbucket.NewBucket(tokenbucket.BucketConfig{
    Capacity:       1000,
    Rate:           100,    // 目标速率：100 令牌/秒
    Warmup:         true,
    WarmupStartRate: 10,    // 起始速率：10 令牌/秒
    WarmupDuration:  30 * time.Second, // 30 秒预热
})
if err != nil {
    log.Fatal(err)
}

// 预热期间 IsWarmingUp() 返回 true
// CurrentRate() 返回当前实际速率（10 → 100 线性增长）
// 预热结束后 IsWarmingUp() 返回 false，CurrentRate() 等于 Rate
```

### Retry-After 头生成

```go
result := bucket.Take(5)
if !result.Allowed {
    // result.RetryAfter 是精确的等待时间
    // result.RetryAfterSeconds() 是向上取整的秒数，适合 HTTP 头
    w.Header().Set("Retry-After", strconv.Itoa(result.RetryAfterSeconds()))
    w.WriteHeader(http.StatusTooManyRequests)
}
```
