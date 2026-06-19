# Metrics 指标采集器模块

## 模块概述

`internal/metrics` 包提供了一个轻量级的指标采集器功能模块，支持四种核心指标类型（Counter、Gauge、Histogram、Summary），提供标签维度区分、快照导出和 Prometheus 格式兼容输出等功能。

## 核心结构体职责

### Registry（指标注册器）

Registry 是指标管理的核心接口，负责注册、存储和查询各类指标。

**主要职责：**
- 注册和管理四种类型的指标
- 按名称和标签组合查找指标
- 生成全局指标快照
- 导出 Prometheus 格式数据
- 注销指标

**实现：** `registry` 结构体，使用读写锁保证并发安全，内部通过 `map[string]map[string]Metric` 的二级映射结构存储指标，一级键为指标名称，二级键为标签哈希值。

### Counter（计数器）

Counter 是单调递增的计数器指标，只能增加或重置，不能减少。

**适用场景：**
- 统计请求总数
- 统计错误总数
- 统计任务完成数
- 统计字节传输量

**主要方法：**
- `Inc()`: 计数器加 1
- `Add(delta float64)`: 计数器增加指定值（负值被忽略）
- `Value() float64`: 获取当前累计值
- `Reset()`: 重置计数器为 0

### Gauge（仪表盘）

Gauge 是可增可减的瞬时值指标，反映某个时刻的状态。

**适用场景：**
- 当前在线用户数
- 内存使用量
- CPU 使用率
- 队列长度
- 活跃连接数

**主要方法：**
- `Set(value float64)`: 设置为指定值
- `Inc()`: 值加 1
- `Dec()`: 值减 1
- `Add(delta float64)`: 增加指定值（可为负）
- `Sub(delta float64)`: 减少指定值
- `Value() float64`: 获取当前值

### Histogram（直方图）

Histogram 用于统计数值分布的观测值，将观测值归入预先配置的桶中，累计各桶的计数。

**适用场景：**
- 请求延迟分布
- 响应时间统计
- 数据大小分布
- 处理时长统计

**主要方法：**
- `Observe(value float64)`: 观测一个数值
- `Buckets() []BucketValue`: 获取各桶的累计计数
- `Count() uint64`: 获取观测总次数
- `Sum() float64`: 获取观测值总和

**桶配置辅助函数：**
- `DefaultBuckets()`: 返回默认分桶（0.005, 0.01, 0.025, ..., 10）
- `ExponentialBuckets(start, factor, count)`: 生成指数分桶
- `LinearBuckets(start, width, count)`: 生成分桶

### Summary（摘要）

Summary 用于统计数值分布的百分位值，记录原始观测值并计算指定的百分位数。

**适用场景：**
- 延迟百分位统计（P50、P90、P99）
- 响应时间分位数
- 性能指标分位分析

**主要方法：**
- `Observe(value float64)`: 观测一个数值
- `Quantiles() []QuantileValue`: 获取各百分位的近似值
- `Count() uint64`: 获取观测总次数
- `Sum() float64`: 获取观测值总和

**默认配置：**
- `DefaultQuantiles()`: 返回默认百分位列表 [0.5, 0.9, 0.99]

### Labels（标签）

标签是键值对组合，用于区分同一指标名的不同维度。

**特性：**
- 相同指标名 + 不同标签组合 = 不同的指标实例
- 标签在指标创建时指定，创建后不可修改
- 查询和导出时按标签维度分别输出

## 使用示例

### 基本使用

```go
package main

import "solocoder-go/internal/metrics"

func main() {
    // 使用默认注册器
    requests := metrics.RegisterCounter("http_requests_total", metrics.Labels{
        {Name: "method", Value: "GET"},
    })
    
    requests.Inc()
    requests.Add(5)
    value := requests.Value()
}
```

### 使用自定义注册器

```go
reg := metrics.NewRegistry()

counter := reg.RegisterCounter("my_counter", nil)
gauge := reg.RegisterGauge("my_gauge", nil)

counter.Inc()
gauge.Set(42)
```

### Counter 使用示例

```go
// 统计不同 HTTP 方法的请求数
getRequests := metrics.RegisterCounter("http_requests_total", metrics.Labels{
    {Name: "method", Value: "GET"},
    {Name: "status", Value: "200"},
})
postRequests := metrics.RegisterCounter("http_requests_total", metrics.Labels{
    {Name: "method", Value: "POST"},
    {Name: "status", Value: "200"},
})

getRequests.Inc()
postRequests.Inc()
```

### Gauge 使用示例

```go
// 监控在线用户数
onlineUsers := metrics.RegisterGauge("online_users", metrics.Labels{
    {Name: "service", Value: "api"},
})

onlineUsers.Set(100)
onlineUsers.Inc()   // 用户登录
onlineUsers.Dec()   // 用户登出
```

### Histogram 使用示例

```go
// 统计请求延迟分布
requestLatency := metrics.RegisterHistogram("request_duration_seconds", 
    metrics.Labels{{Name: "service", Value: "api"}},
    []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10},
)

// 记录每次请求的耗时
requestLatency.Observe(0.023)
requestLatency.Observe(0.156)

// 获取统计结果
buckets := requestLatency.Buckets()
count := requestLatency.Count()
sum := requestLatency.Sum()
```

### Summary 使用示例

```go
// 统计接口延迟的百分位
apiLatency := metrics.RegisterSummary("api_latency_seconds",
    metrics.Labels{{Name: "endpoint", Value: "/users"}},
    []float64{0.5, 0.9, 0.99},
)

// 记录观测值
for i := 0; i < 1000; i++ {
    apiLatency.Observe(float64(i) * 0.001)
}

// 获取百分位值
quantiles := apiLatency.Quantiles()
for _, q := range quantiles {
    fmt.Printf("P%.0f: %v\n", q.Quantile*100, q.Value)
}
```

### 快照导出

```go
// 获取当前所有指标的快照
snapshot := metrics.Snapshot()

for _, mv := range snapshot {
    fmt.Printf("Metric: %s, Type: %s, Value: %v\n", 
        mv.Name, mv.Type, mv.Value)
    
    if mv.Type == metrics.HistogramType {
        for _, bucket := range mv.Buckets {
            fmt.Printf("  Bucket le=%v: %d\n", bucket.UpperBound, bucket.Count)
        }
    }
}
```

### Prometheus 格式输出

```go
// 导出为 Prometheus 文本格式
data := metrics.PrometheusFormat()
fmt.Print(string(data))
```

输出示例：
```
# HELP http_requests_total 
# TYPE http_requests_total counter
http_requests_total{method="GET"} 42
# HELP memory_usage 
# TYPE memory_usage gauge
memory_usage 1024.5
# HELP request_duration_seconds 
# TYPE request_duration_seconds histogram
request_duration_seconds_bucket{le="0.1"} 5
request_duration_seconds_bucket{le="0.5"} 10
request_duration_seconds_bucket{le="+Inf"} 15
request_duration_seconds_sum 8.5
request_duration_seconds_count 15
```

## 并发安全

所有指标类型和注册器都支持并发安全访问，内部使用读写锁（sync.RWMutex）保证线程安全。

## 错误处理

模块使用 `panic` 机制处理配置错误（如重复注册、无效名称等），以下情况会触发 panic：

- 指标名称不符合命名规范
- 标签名称不符合命名规范（如以 `__` 开头）
- 重复注册相同名称和标签的指标
- Histogram 配置空的分桶列表
- Summary 配置无效的百分位值（<0 或 >1）

## 测试

运行单元测试：

```bash
go test ./internal/metrics/ -v
```

测试覆盖以下方面：
- 四种指标类型的基本操作
- 标签功能和多维度指标
- 注册表的增删查操作
- 快照导出功能
- Prometheus 格式输出
- 并发安全测试
- 边界条件和异常分支
