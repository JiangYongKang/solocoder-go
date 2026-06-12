# 并发任务编排器 (Orchestrator)

## 模块功能

并发任务编排器用于管理具有依赖关系的任务集合的执行。核心能力包括：

- **DAG 依赖调度**：支持定义任务间的有向无环图依赖关系，调度器按拓扑顺序并发执行任务
- **超时控制**：每个任务支持配置独立的执行超时时间，超时后自动取消并标记为 Timeout
- **错误传播**：任务失败时，错误信息沿依赖链向上传播，所有直接或间接依赖该失败任务的后继任务标记为 Skipped，并携带完整错误原因；不依赖失败任务的其他分支继续正常执行
- **局部重试**：支持对 DAG 中失败的单个任务及其下游进行重试，已成功完成的上游任务不重复执行；重试前可通过 API 动态更新任务的执行函数和超时配置以修复问题
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
| UpdateTaskFunc(id, fn) error | 在重试前更新任务的执行函数（编排器未运行时） |
| UpdateTaskTimeout(id, timeout) error | 在重试前更新任务的超时时间（编排器未运行时） |
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
5. **错误原因链传播**：Skipped 任务的 Error 字段会递归包装上游失败原因，使用 `errors.Is` 可沿整条依赖链定位根因

### 错误传播实现细节

`shouldSkip` 方法在检测到上游失败时，会同时返回：
- `skip: bool` — 是否需要跳过
- `depID: string` — 导致跳过的直接依赖 ID
- `depErr: error` — 该依赖上记录的实际错误（本身可能是另一个 Skipped 任务的包装错误）

标记 Skipped 时使用 `fmt.Errorf` 的 `%w` 动词进行递归包装：

```go
o.results[id].Error = fmt.Errorf("skipped due to failure in dependency '%s': %w", depID, depErr)
```

因此，对于依赖链 `t0(根因失败) → t1 → t2 → t3`：
- t3.Error 的字符串形如：
  `skipped due to failure in dependency 't2': skipped due to failure in dependency 't1': skipped due to failure in dependency 't0': <根因错误>`
- `errors.Is(t3.Error, rootErr)` 返回 `true`
- `errors.Is(t3.Error, ErrTimeout)` 也能匹配到超时类型的根因

## 局部重试流程

```
1. 初始执行：
   A(Success) → B(Failed) → C(Skipped) → D(Skipped)

2. 调用 UpdateTaskFunc("B", newFn) 替换 B 的执行函数以修复 Bug
   - 也可调用 UpdateTaskTimeout 调整超时配置

3. 调用 RetryTask(ctx, "B")：

   - 收集重试目标：B 及其所有下游 = {B, C, D}
   - 将 B、C、D 的状态重置为 Pending
   - A 保持 Success 不变，不重新执行
   - 以重试目标子集作为 taskSet 重新运行调度器
   - 重试目标中仅统计内部的依赖关系（A 不在 taskSet 中，不计入 readyCount）

4. 重试结果：
   A 仍为 Success
   B → Success（修复后）
   C → Success
   D → Success
```

### 重试的典型步骤

```
Run 发现失败
    ↓
查看失败任务的 Error，定位根因
    ↓
UpdateTaskFunc / UpdateTaskTimeout 修正问题
    ↓
RetryTask(ctx, failedTaskID) 触发局部重试
    ↓
获得新的 ExecutionReport
```

### 可重试的任务状态

只有以下状态的任务可作为 RetryTask 的入口：
- **Failed**：执行函数返回错误或 panic
- **Timeout**：执行超时
- **Skipped**：本身未执行，但上游已被修复（可通过重试其上游恢复，或直接重试本节点时同时重置下游）

若任务状态为 Success 或 Running，调用 RetryTask 将返回 `ErrCannotRetry` 或 `ErrOrchestratorRunning`。

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

### 局部重试 + 修复函数

```go
// 首次执行，部分任务失败（如数据库连接错误）
report, _ := o.Run(context.Background())

// 查看失败原因，用 errors.Is / errors.As 溯源根因
if !report.Success {
    fetchResult := report.TaskResults["fetch_db"]
    if fetchResult.Status == StatusFailed {
        var dbErr *MyDBError
        if errors.As(fetchResult.Error, &dbErr) {
            log.Printf("DB error code=%d: %s", dbErr.Code, dbErr.Message)
        }
    }

    // 用修复后的函数替换原有逻辑（例如切换到备用数据源）
    o.UpdateTaskFunc("fetch_db", func(ctx context.Context) (interface{}, error) {
        return backupDB.Query(ctx, "SELECT ...")
    })

    // 如果是超时问题，也可以放宽超时时间
    o.UpdateTaskTimeout("fetch_db", 30*time.Second)

    // 仅重试失败节点及其下游，已成功的上游不复用
    retryReport, err := o.RetryTask(context.Background(), "fetch_db")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("重试后整体结果: success=%v\n", retryReport.Success)
}
```

### 多次重试 + 最终修复

```go
// 第 1 次运行：失败
r1, _ := o.Run(ctx)
if !r1.Success {
    // 第 1 次重试：沿用旧函数，可能因外部临时故障自愈
    r2, err := o.RetryTask(ctx, "failed_id")
    if err != nil { log.Fatal(err) }

    if !r2.Success {
        // 第 2 次重试：替换为修复后的函数再试
        o.UpdateTaskFunc("failed_id", fixedFunc)
        r3, _ := o.RetryTask(ctx, "failed_id")
    }
}
```

### 深度依赖链错误溯源

```go
// DAG: t0 → t1 → t2 → t3 → t4
// t0 因业务错误失败
rootErr := &MyAppError{Code: "AUTH_TOKEN_EXPIRED"}
o.AddTask("t0", "根节点", func(ctx context.Context) (interface{}, error) {
    return nil, rootErr
}, 0)
// ... t1-t4 依次链式依赖

report, _ := o.Run(ctx)

// 从最末端任务 t4 的错误一路用 errors.Is / errors.As 找到根因
t4Err := report.TaskResults["t4"].Error

var appErr *MyAppError
if errors.As(t4Err, &appErr) {
    log.Printf("根因: code=%s", appErr.Code) // AUTH_TOKEN_EXPIRED
}

// 也可以验证错误类型
if errors.Is(t4Err, rootErr) {
    log.Println("t4 错误链中找到了原始错误对象")
}

// 错误链字符串形式示例：
// skipped due to failure in dependency 't3':
//   skipped due to failure in dependency 't2':
//     skipped due to failure in dependency 't1':
//       skipped due to failure in dependency 't0':
//         MyAppError code=AUTH_TOKEN_EXPIRED
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
