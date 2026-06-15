# servicereg - 服务注册与发现模块

## 模块功能

`servicereg` 提供了一套完整的服务注册与发现机制，支持以下核心功能：

1. **心跳注册**：服务实例启动后向注册中心发送注册请求，注册成功后通过定期心跳维持注册状态；心跳超时未收到的实例自动从注册列表中被移除。
2. **健康状态上报**：服务实例在心跳中携带自身的健康状态信息（CPU 使用率、内存使用率、请求成功率等），注册中心记录每个实例的最新健康状态供调用方参考。
3. **客户端负载均衡**：调用方从注册中心获取某个服务的所有可用实例列表后，在客户端侧实现负载均衡策略（轮询或随机选择）将请求分发到不同实例。
4. **服务列表变更推送**：当服务实例上下线导致实例列表发生变化时，注册中心主动向订阅了该服务的所有调用方推送最新的实例列表。

## 核心结构体

### HealthStatus

健康状态信息，由服务实例在心跳中上报。

| 字段 | 类型 | 说明 |
|------|------|------|
| CPUUsage | float64 | CPU 使用率（百分比） |
| MemoryUsage | float64 | 内存使用率（百分比） |
| RequestSuccessRate | float64 | 请求成功率（百分比） |

### ServiceInstance

服务实例，代表一个已注册的服务节点。

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 实例唯一标识（同一服务内唯一） |
| ServiceName | string | 所属服务名称 |
| Address | string | 实例地址（如 `10.0.0.1:8080`） |
| Health | HealthStatus | 最新健康状态 |
| LastHeartbeat | time.Time | 最后一次心跳时间 |

### RegistryConfig

注册中心配置。

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| HeartbeatTTL | time.Duration | 30s | 心跳超时阈值，超过此时间未收到心跳的实例将被移除 |
| CheckInterval | time.Duration | 1s | 过期检查的间隔时间 |

### Registry

注册中心，管理所有服务实例的注册、心跳、过期和推送。

主要方法：

| 方法 | 说明 |
|------|------|
| `Register(inst *ServiceInstance) error` | 注册服务实例 |
| `Deregister(serviceName, instanceID string) error` | 注销服务实例 |
| `Heartbeat(serviceName, instanceID string, health HealthStatus) error` | 发送心跳并更新健康状态 |
| `GetInstances(serviceName string) ([]*ServiceInstance, error)` | 获取指定服务的所有可用实例 |
| `GetHealth(serviceName, instanceID string) (*HealthStatus, error)` | 获取指定实例的健康状态 |
| `Subscribe(serviceName string, handler SubscriberFunc) (string, error)` | 订阅服务实例列表变更通知 |
| `Unsubscribe(serviceName, subscriberID string) error` | 取消订阅 |
| `Start()` | 启动后台心跳过期检查协程 |
| `Stop()` | 停止注册中心 |
| `InstanceCount(serviceName string) int` | 获取指定服务的实例数 |
| `ServiceCount() int` | 获取已注册的服务数 |
| `SubscriberCount(serviceName string) int` | 获取指定服务的订阅者数 |

### ServiceChangeEvent

服务变更事件，当实例列表发生变更时推送给订阅者。

| 字段 | 类型 | 说明 |
|------|------|------|
| ServiceName | string | 发生变更的服务名称 |
| Instances | []*ServiceInstance | 变更后的最新实例列表 |
| Action | string | 变更类型：`register`、`deregister`、`expire` |

### LoadBalancer（接口）

客户端负载均衡器接口。

| 方法 | 说明 |
|------|------|
| `Select(instances []*ServiceInstance) (*ServiceInstance, error)` | 从实例列表中选择一个实例 |

#### RoundRobinLB

轮询负载均衡器，按顺序依次选择实例。

#### RandomLB

随机负载均衡器，随机选择一个实例。

## 服务实例生命周期

一个服务实例从注册到发现的完整生命周期如下：

```
1. 注册阶段
   实例启动 → 调用 Registry.Register() → 注册中心记录实例信息
   → 通知订阅该服务的所有调用方（action: "register"）

2. 心跳维持阶段
   实例定期调用 Registry.Heartbeat() → 更新 LastHeartbeat 和 HealthStatus
   → 只要心跳间隔小于 HeartbeatTTL，实例保持存活

3. 过期移除阶段
   若 HeartbeatTTL 时间内未收到心跳 → 后台检查协程检测到超时
   → 从注册列表移除该实例 → 通知订阅者（action: "expire"）

4. 主动注销阶段
   实例正常关闭时调用 Registry.Deregister() → 从注册列表移除
   → 通知订阅者（action: "deregister"）

5. 重新注册
   实例可再次调用 Register() 重新加入注册列表（如重启后）
```

## 使用示例

### 创建注册中心

```go
cfg := servicereg.RegistryConfig{
    HeartbeatTTL:   30 * time.Second,
    CheckInterval:  1 * time.Second,
}
registry := servicereg.NewRegistry(cfg)
registry.Start()
defer registry.Stop()
```

### 注册服务实例

```go
inst := &servicereg.ServiceInstance{
    ID:          "order-service-1",
    ServiceName: "order-service",
    Address:     "10.0.0.1:8080",
    Health: servicereg.HealthStatus{
        CPUUsage:          45.0,
        MemoryUsage:       60.0,
        RequestSuccessRate: 99.5,
    },
}
err := registry.Register(inst)
```

### 发送心跳

```go
health := servicereg.HealthStatus{
    CPUUsage:          50.0,
    MemoryUsage:       65.0,
    RequestSuccessRate: 99.2,
}
err := registry.Heartbeat("order-service", "order-service-1", health)
```

### 注销服务实例

```go
err := registry.Deregister("order-service", "order-service-1")
```

### 订阅服务变更

```go
subID, err := registry.Subscribe("order-service", func(event servicereg.ServiceChangeEvent) {
    fmt.Printf("服务 %s 发生变更: %s, 当前实例数: %d\n",
        event.ServiceName, event.Action, len(event.Instances))
})
defer registry.Unsubscribe("order-service", subID)
```

### 发现服务并负载均衡

```go
instances, err := registry.GetInstances("order-service")

// 轮询负载均衡
lb := servicereg.NewRoundRobinLB()
selected, err := lb.Select(instances)
fmt.Printf("选中实例: %s (%s)\n", selected.ID, selected.Address)

// 随机负载均衡
randLB := servicereg.NewRandomLB()
selected, err = randLB.Select(instances)
```

### 查询健康状态

```go
health, err := registry.GetHealth("order-service", "order-service-1")
fmt.Printf("CPU: %.1f%%, 内存: %.1f%%, 成功率: %.1f%%\n",
    health.CPUUsage, health.MemoryUsage, health.RequestSuccessRate)
```
