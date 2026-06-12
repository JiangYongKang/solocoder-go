# 并发任务编排器 (Orchestrator)

## 模块功能

并发任务编排器用于管理具有依赖关系的任务集合的执行。核心能力包括：

- **DAG 依赖调度**：支持定义任务间的有向无环图依赖关系，调度器按拓扑顺序并发执行任务
- **超时控制**：每个任务支持配置独立的执行超时时间，超时后自动取消并标记为 Timeout
- **错误传播**：任务失败时，所有直接或间接依赖该失败任务的后继任务标记为 Skipped，不依赖失败任务的其他分支继续正常执行
- **局部重试**：支持对 DAG 中失败的单个任务及其下游进行重试，已成功完成的上游任务不重复执行
- **结果聚合**：所有任务执行完毕后，将每个任务的状态、输出结果和耗时汇总为整体执行报告

## 核心结构体

### Task

任务定义，描述 DAG 中的一个节点：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 任务唯一标识 |
| Name | string | 任务名称（描述性） |
| Func | TaskFunc | 任务执行函数，签名为 `func(ctx context.Context) (interface{}, error)` |
| Timeout | time.Duration | 执行超时时间，0 表示不限制 |
| Dependencies | []string | 前置依赖任务 ID 列表 |
| Successors | []string | 后继任务 ID 列表（由 AddTask 自动维护） |
| MaxRetry | int | 最大重试次数（不含首次执行） |

### TaskResult

任务执行结果：

| 字段 | 类型 | 说明 |
|------|------|------|
| TaskID | string | 任务 ID |
| Status | TaskStatus | 最终状态（Pending / Running / Success / Failed / Skipped / Timeout） |
| Output | interface{} | 任务输出值 |
| Error | error | 执行错误 |
| StartTime | time.Time | 开始执行时间 |
| EndTime | time.Time | 执行结束时间 |
| Duration | time.Duration | 执行耗时 |
| Attempts | int | 实际执行次数（含重试） |

### ExecutionReport

整体执行报告：

| 字段 | 类型 | 说明 |
|------|------|------|
| Success | bool | 所有任务是否成功（无 Failed / Timeout） |
| StartTime | time.Time | 整体开始时间 |
| EndTime | time.Time | 整体结束时间 |
| Duration | time.Duration | 整体执行耗时 |
| TaskResults | map[string]*TaskResult | 每个任务的执行结果 |

### Orchestrator

编排器主结构体，管理任务的注册、调度和执行：

| 方法 | 说明 |
|------|------|
| NewOrchestrator() | 创建新的编排器实例 |
| AddTask(id, name, fn, timeout, ...deps) | 添加任务，指定 ID、名称、执行函数、超时时间和前置依赖 |
| SetTaskRetry(id, maxRetry) | 设置任务的最大重试次数 |
| ValidateDAG() error | 验证 DAG 的合法性（检测环和无效依赖） |
| Run(ctx) (*ExecutionReport, error) | 执行整个 DAG，返回执行报告 |
| RetryTask(ctx, taskID) (*ExecutionReport, error) | 重试失败任务及其下游 |
| Stop() | 取消正在执行的任务 |
| GetTask(id) | 获取任务定义副本 |
| GetTaskResult(id) | 获取任务执行结果副本 |
| TaskCount() | 获取任务总数 |

## 任务状态

```
Pending → Running → Success
                  → Failed
                  → Timeout
                  → Skipped
```

- **Pending**：任务等待执行
- **Running**：任务正在执行
- **Success**：任务执行成功
- **Failed**：任务执行失败（包括 panic）
- **Timeout**：任务执行超时
- **Skipped**：任务因上游依赖失败/超时而被跳过

## DAG 调度流程

```
┌─────────────────────────────────────────────┐
│  Run(ctx)                                   │
│                                             │
│  1. 验证 DAG（环检测 + 依赖完整性检查）      │
│  2. 重置所有任务状态为 Pending               │
│  3. 计算每个任务的 readyCount（依赖数）       │
│  4. 将 readyCount=0 的任务加入调度队列        │
│                                             │
│  ┌──────────── 调度循环 ──────────────┐      │
│  │                                    │      │
│  │  从 taskCh 取任务 → 并发执行       │      │
│  │  从 completed 取完成通知 →         │      │
│  │    若成功：递减后继的 readyCount    │      │
│  │      readyCount=0 → 检查是否应跳过 │      │
│  │        不跳过 → 加入 taskCh        │      │
│  │        跳过 → 标记 Skipped         │      │
│  │          → 加入 completed          │      │
│  │    若失败：递减后继的 readyCount    │      │
│  │      后继就绪时自动检测上游失败     │      │
│  │      → 标记 Skipped                │      │
│  │                                    │      │
│  │  completedCount >= pendingCount    │      │
│  │    → 调度结束                      │      │
│  └────────────────────────────────────┘      │
│                                             │
│  5. 构建执行报告                            │
└─────────────────────────────────────────────┘
```

### 关键设计：shouldSkip 检查

当任务的所有依赖都已完成（readyCount=0）时，调度器会调用 `shouldSkip` 检查是否有任何依赖处于失败状态：

- 如果任何依赖状态为 Failed / Timeout / Skipped，则当前任务标记为 Skipped
- Skipped 任务直接进入 completed 通道，确保完成计数正确
- 不依赖失败任务的其他分支继续正常调度执行

这个设计确保：
1. 调度器不会因为跳过任务而卡死（所有任务都经过 completed 通道计数）
2. 错误传播自然发生在依赖解析阶段，无需额外的 BFS 遍历

## 错误传播流程

```
     A (Success)
    / \
   B   C (Failed)
   |   |
   D   E

结果：
  A → Success
  B → Success（不依赖 C）
  C → Failed
  D → Success（不依赖 C）
  E → Skipped（依赖 C）
```

错误传播规则：
1. 任务失败（Failed / Timeout）时，其所有后继任务在就绪时检测到上游失败，标记为 Skipped
2. 不依赖失败任务的其他分支不受影响，继续正常执行
3. 如果一个任务同时依赖成功和失败的任务，该任务标记为 Skipped
4. Skipped 状态会沿依赖链继续传播（间接后继也会被跳过）

## 局部重试流程

```
1. 初始执行：
   A(Success) → B(Failed) → C(Skipped) → D(Skipped)

2. 修复 B 的函数后，调用 RetryTask(ctx, "B")：

   - 收集重试目标：B 及其所有下游 = {B, C, D}
   - 将 B、C、D 的状态重置为 Pending
   - A 保持 Success 不变，不重新执行
   - 以重试目标子集作为 taskSet 重新运行调度器
   - 重试目标中仅统计内部的依赖关系（A 不在 taskSet 中，不计入 readyCount）

3. 重试结果：
   A 仍为 Success
   B → Success（修复后）
   C → Success
   D → Success
```

## 超时控制

每个任务可配置独立的超时时间：
- `timeout = 0`：不限制执行时间
- `timeout > 0`：任务执行超过指定时间后自动取消，标记为 Timeout
- 超时任务的后继任务标记为 Skipped
- 超时后任务函数会收到 `context.Done()` 信号，应尽快退出
- 超时任务可通过 `RetryTask` 重试

## 使用示例

### 基本用法

```go
o := orchestrator.NewOrchestrator()

o.AddTask("fetch", "获取数据", func(ctx context.Context) (interface{}, error) {
    resp, err := http.Get("https://api.example.com/data")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, _ := io.ReadAll(resp.Body)
    return string(body), nil
}, 5*time.Second)

o.AddTask("transform", "转换数据", func(ctx context.Context) (interface{}, error) {
    result, _ := o.GetTaskResult("fetch")
    data := result.Output.(string)
    return strings.ToUpper(data), nil
}, 3*time.Second, "fetch")

o.AddTask("save", "保存结果", func(ctx context.Context) (interface{}, error) {
    result, _ := o.GetTaskResult("transform")
    return os.WriteFile("output.txt", []byte(result.Output.(string)), 0644)
}, 2*time.Second, "transform")

report, err := o.Run(context.Background())
if err != nil {
    log.Fatal(err)
}

for id, result := range report.TaskResults {
    fmt.Printf("Task %s: %s (duration: %v)\n", id, result.Status, result.Duration)
}
```

### 并行分支 + 错误隔离

```go
o := orchestrator.NewOrchestrator()

o.AddTask("source", "数据源", fetchFn, 5*time.Second)
o.AddTask("branch_a", "分支A", processAFn, 3*time.Second, "source")
o.AddTask("branch_b", "分支B", processBFn, 3*time.Second, "source")
o.AddTask("merge", "合并", mergeFn, 2*time.Second, "branch_a", "branch_b")

report, _ := o.Run(context.Background())

// 如果 branch_a 失败：
//   branch_a → Failed
//   merge → Skipped（依赖 branch_a）
//   branch_b → Success（不依赖 branch_a，继续执行）
```

### 局部重试

```go
// 首次执行，部分任务失败
report, _ := o.Run(context.Background())

// 修复失败任务的原因后，仅重试失败节点及其下游
retryReport, err := o.RetryTask(context.Background(), "failed_task_id")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("重试结果: success=%v\n", retryReport.Success)
```

### 超时与重试组合

```go
o := orchestrator.NewOrchestrator()

o.AddTask("unstable", "不稳定任务", func(ctx context.Context) (interface{}, error) {
    select {
    case <-time.After(100 * time.Millisecond):
        return "done", nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}, 50*time.Millisecond)

// 设置最多重试 2 次（总共最多执行 3 次）
o.SetTaskRetry("unstable", 2)

report, _ := o.Run(context.Background())
```

### 取消执行

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(2 * time.Second)
    cancel() // 或调用 o.Stop()
}()

report, _ := o.Run(ctx)
// 未完成的任务会被标记为 Skipped
```
