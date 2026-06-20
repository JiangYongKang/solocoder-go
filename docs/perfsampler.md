# Perfsampler 请求级性能采样器模块

## 模块概述

`internal/perfsampler` 包提供了一个请求级性能采样器功能模块，支持对单个请求处理过程进行多维度性能数据采集，包括 CPU 使用情况、内存分配情况和处理耗时分段计时。模块支持可配置的采样率，未被采样的请求不产生任何 Profiling 开销。采样结果可导出为结构化对象并序列化为 JSON 格式，同时支持将 CPU 调用栈数据转换为火焰图兼容格式。

## 核心结构体职责

### Sampler 接口

采样器接口，定义采样决策逻辑。

**主要实现：**

- `AlwaysSample`: 始终采样，采样率 100%
- `NeverSample`: 永不采样，采样率 0%
- `ProbabilitySampler`: 概率采样器，支持 0~1 之间的任意采样率

**主要方法：**
- `ShouldSample(requestID string) bool`: 根据请求 ID 决定是否采样
- `Rate() float64`: 返回采样率配置值

**概率采样算法：**
概率采样器使用请求 ID 的前 8 字节哈希值与阈值比较，确保采样决策是确定性的（同一请求 ID 始终得到相同结果），避免同一请求在不同节点采样状态不一致。阈值计算公式：`threshold = rate * math.MaxUint64`。

### RequestProfiler（请求性能采样器）

RequestProfiler 是单个请求的性能数据采集的核心结构体，负责管理该请求的所有 Profiling 数据。

**主要职责：**
- 管理采样状态管理（开始/停止）
- CPU 调用栈数据采集
- 内存分配数据采集
- 处理耗时分段计时
- 采样结果导出
- 火焰图格式转换

**线程安全：** 所有操作都通过互斥锁保护，支持并发安全访问。

### CPUStackNode（CPU 调用栈节点）

CPUStackNode 表示 CPU 调用栈中的一个函数节点，以树形结构组织，记录函数名、自身执行时间、总执行时间和采样计数。

**字段说明：**
- `FunctionName`: 函数名称
- `SelfTimeNs`: 函数自身执行时间（纳秒），不包含子函数调用时间
- `TotalTimeNs`: 函数总执行时间（纳秒），包含子函数调用时间
- `SampleCount`: 采样命中次数
- `Children`: 子函数调用节点列表

### MemoryFuncStat（内存分配统计）

MemoryFuncStat 记录单个函数的内存分配统计信息，统计粒度到函数级别。

**字段说明：**
- `FunctionName`: 函数名称
- `AllocCount`: 内存分配次数
- `AllocBytes`: 内存分配总字节数
- `FreeCount`: 内存释放次数
- `FreeBytes`: 内存释放总字节数
- `InUseCount`: 当前仍在使用的内存块数
- `InUseBytes`: 当前仍在使用的内存字节数

### TimingSegment（耗时分段）

TimingSegment 记录请求处理过程中一个阶段的耗时信息，支持用户自定义标签划分。

**字段说明：**
- `Label`: 阶段标签名称
- `StartTime`: 阶段开始时间
- `Duration`: 阶段持续时长（纳秒）
- `Metadata`: 附加元数据键值对

### ProfileResult（采样结果）

ProfileResult 是单次请求的完整 Profiling 采样结果导出对象。

**字段说明：**
- `RequestID`: 请求唯一标识
- `Sampled`: 是否被采样
- `SampleRate`: 采样率配置
- `StartTime`: 请求处理开始时间
- `EndTime`: 请求处理结束时间
- `Duration`: 请求总处理时长
- `CPUProfile`: CPU 调用栈树根节点
- `MemoryStats`: 内存分配统计列表
- `Timing`: 耗时分段列表

**主要方法：**
- `JSON() ([]byte, error)`: 序列化为紧凑 JSON 格式
- `PrettyJSON() ([]byte, error)`: 序列化为格式化 JSON 格式

### FlameGraphEntry（火焰图条目）

FlameGraphEntry 表示火焰图格式的一条记录，用于将 CPU 调用栈数据转换为火焰图兼容格式。

**字段说明：**
- `Stack`: 调用栈字符串数组，按调用顺序排列，第一个元素为根节点，最后一个为当前函数
- `Value`: 该调用栈的采样计数值

**火焰图格式说明：**

火焰图格式通过多条记录表达父子关系，每条记录代表一个唯一的调用路径：
- 每条记录的 `Stack` 字段是完整的调用栈路径
- `Value` 字段是该调用路径被采样命中次数
- 父子关系通过调用栈层级隐式表达，如：
  - `["main", "foo", "bar"]` 表示 `main` → `foo` → `bar`
  - `["main", "foo", "baz"]` 表示 `main` → `foo` → `baz`
  - 两条记录共享 `main` → `foo` 前缀，表示 `foo` 是 `bar` 和 `baz` 的父节点

这种格式与 Brendan Gregg 火焰图工具兼容，可直接用于生成可视化。

## 使用示例

### 基本使用流程

```go
package main

import "solocoder-go/internal/perfsampler"

func handleRequest(requestID string) {
    // 1. 创建采样器（10% 采样率
    sampler, _ := perfsampler.NewProbabilitySampler(0.1)
    
    // 2. 创建请求性能采样器
    profiler, err := perfsampler.NewRequestProfiler(requestID, sampler)
    if err != nil {
        // 处理错误
    }
    
    // 3. 开始采样
    profiler.Start()
    
    // 4. 记录性能数据
    profiler.StartSegment("parseRequest")
    
    profiler.EnterCPUFunction("parseJSON")
    profiler.RecordAlloc("parseJSON", 1024)
    // ... 处理逻辑
    profiler.ExitCPUFunction()
    
    profiler.EndSegment()
    
    // 5. 停止采样
    profiler.Stop()
    
    // 6. 导出结果
    if profiler.IsSampled() {
        result, _ := profiler.Export()
        
        // 序列化为 JSON
        jsonData, _ := result.JSON()
        
        // 生成火焰图数据
        flameData, _ := profiler.ToFlameGraph()
    }
}
```

### CPU Profiling 使用示例

```go
// 方式一：手动 Enter/Exit 方式
profiler.EnterCPUFunction("handleRequest")
profiler.EnterCPUFunction("dbQuery")
// 执行数据库查询
profiler.ExitCPUFunction()
profiler.ExitCPUFunction()

// 方式二：记录调用栈方式
stack := []string{"handleRequest", "dbQuery", "mysqlDriver", "readSocket"}
profiler.RecordCPUSample(stack)
```

### 内存 Profiling 使用示例

```go
// 记录内存分配
profiler.RecordAlloc("parseJSON", 2048)  // 分配 2KB

// 记录内存释放
profiler.RecordFree("parseJSON", 2048)   // 释放 2KB
```

### 耗时 Profiling 使用示例

```go
// 简单分段
profiler.StartSegment("dbQuery")
// 执行数据库查询
seg, _ := profiler.EndSegment()
fmt.Printf("数据库查询耗时: %v", seg.Duration)

// 嵌套分段
profiler.StartSegment("handleRequest")
profiler.StartSegment("parseInput")
// ...
profiler.EndSegment()
profiler.StartSegment("processLogic")
// ...
profiler.EndSegment()
profiler.EndSegment()

// 带元数据
meta := map[string]string{"table": "users", "operation": "SELECT"}
profiler.StartSegment("dbQuery", meta)
// ...
profiler.EndSegment()

// 设置分段元数据
profiler.SetSegmentMetadata("dbQuery", "rows", "100")
```

### 采样结果导出示例

```go
// 获取完整结果
result, err := profiler.Export()

// 紧凑 JSON
jsonData, err := result.JSON()

// 格式化 JSON
prettyJSON, err := result.PrettyJSON()

// 火焰图数据
flameEntries, err := profiler.ToFlameGraph()

// 序列化为 JSON
flameJSON, _ := json.Marshal(flameEntries)
```

### 采样器配置示例

```go
// 始终采样
sampler := perfsampler.NewAlwaysSample()

// 永不采样
sampler := perfsampler.NewNeverSample()

// 10% 采样率
sampler, err := perfsampler.NewProbabilitySampler(0.1)

// 100% 采样率
sampler, err := perfsampler.NewProbabilitySampler(1.0)

// 生成请求 ID
requestID, err := perfsampler.GenerateRequestID()
```

### 完整工作流示例

```go
func processRequest(ctx context.Context, req *Request) (*Response, error) {
    // 1. 初始化采样器
    sampler, _ := perfsampler.NewProbabilitySampler(0.05) // 5% 采样率
    
    // 2. 创建采样器
    profiler, err := perfsampler.NewRequestProfiler(req.ID, sampler)
    if err != nil {
        return nil, err
    }
    
    // 3. 开始采样（采样决策在此处一次性确定
    if err := profiler.Start(); err != nil {
        return nil, err
    }
    
    // 4. 记录总耗时
    profiler.StartSegment("total")
    
    // 5. 解析请求
    profiler.StartSegment("parseRequest")
    profiler.EnterCPUFunction("parseJSON")
    profiler.RecordAlloc("parseJSON", 4096)
    params, err := parseRequestBody(req.Body)
    profiler.ExitCPUFunction()
    profiler.EndSegment()
    if err != nil {
        profiler.Stop()
        return nil, err
    }
    
    // 6. 业务逻辑处理
    profiler.StartSegment("businessLogic")
    profiler.EnterCPUFunction("validateParams")
    if err := validate(params); err != nil {
        profiler.ExitCPUFunction()
        profiler.EndSegment()
        profiler.Stop()
        return nil, err
    }
    profiler.ExitCPUFunction()
    
    profiler.EnterCPUFunction("processData")
    profiler.RecordAlloc("processData", 8192)
    result := process(params)
    profiler.ExitCPUFunction()
    profiler.EndSegment()
    
    // 7. 构建响应
    profiler.StartSegment("buildResponse")
    resp := buildResponse(result)
    profiler.EndSegment()
    
    // 8. 结束总耗时
    profiler.EndSegment()
    
    // 9. 停止采样并导出结果
    profiler.Stop()
    
    if profiler.IsSampled() {
        profileResult, _ := profiler.Export()
        
        // 记录采样结果
        jsonData, _ := profileResult.PrettyJSON()
        log.Printf("性能采样数据: %s", string(jsonData))
        
        // 生成火焰图
        flameData, _ := profiler.ToFlameGraph()
        flameJSON, _ := json.Marshal(flameData)
        log.Printf("火焰图数据: %s", string(flameJSON))
    }
    
    return resp, nil
}
```

## 并发安全

RequestProfiler 所有公共方法都通过内部互斥锁保护，支持并发安全访问。在多协程环境下可以安全地从不同协程调用各种采集方法。

**锁机制：
- 所有修改操作都会获取互斥锁
- 数据结构内部状态一致性保证
- 支持从任意协程调用采样方法

## 错误处理

模块使用明确的错误变量表示各种错误场景：

| 错误变量 | 说明
|---------|------
| `ErrInvalidSampleRate` | 无效的采样率，必须在 0~1 之间
| `ErrProfilerNotStarted` | 采样器未启动
| `ErrProfilerAlreadyStarted` | 采样器已启动
| `ErrEmptyRequestID` | 请求 ID 不能为空
| `ErrEmptyLabel` | 标签不能为空
| `ErrSegmentAlreadyStarted` | 分段已启动
| `ErrSegmentNotStarted` | 分段未启动
| `ErrNilSampler` | 采样器不能为空
| `ErrInvalidCPUProfile` | 无效的 CPU 采样数据
| `ErrNotSampled` | 请求未被采样

## 测试

运行单元测试：

```bash
go test ./internal/perfsampler/ -v
```

运行覆盖率测试：

```bash
go test ./internal/perfsampler/ -cover
```

测试覆盖以下方面：
- 三种采样器的基本功能和边界条件
- CPU Profiling 的 Enter/Exit 和 RecordCPUSample 两种采集方式
- 内存 Profiling 的 Alloc/Free 统计
- 耗时 Profiling 的分段计时、嵌套分段、元数据管理
- 采样率配置的概率分布验证
- 采样结果导出和 JSON 序列化
- 火焰图数据格式生成
- 并发安全测试
- 边界条件和异常分支
- 完整工作流测试
- 未采样请求的零开销验证
- CPU 时间计算正确性验证
