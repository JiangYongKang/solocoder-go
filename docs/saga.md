# Saga 分布式事务协调器

## 模块功能

Saga 分布式事务协调器用于管理分布式系统中的长事务，通过将大事务拆分为多个本地事务（步骤），并为每个步骤提供补偿操作来实现最终一致性。核心能力包括：

- **正向操作顺序执行**：支持将一组正向操作按注册顺序依次执行，每个正向操作是一个可执行函数返回执行结果，正向操作之间可传递上下文数据用于后续操作使用前序操作的输出，任意一个正向操作执行失败时停止执行后续操作并触发补偿流程。
- **补偿事务回滚**：每个正向操作在注册时需同时注册对应的补偿操作，当正向操作链执行到某一步失败时事务协调器按照已成功执行的正向操作的反序依次调用各操作的补偿函数，补偿操作执行成功则继续回滚前一个操作，补偿失败时记录失败信息并继续回滚剩余操作不因单个补偿失败而中断整体回滚流程。
- **补偿失败人工干预**：当补偿操作执行失败时将对应的事务记录标记为需要人工干预状态，标记信息包含失败的正向操作和补偿操作的独立标识、失败原因和执行时间，提供查询接口列出所有待人工干预的事务供外部系统轮询处理。
- **事务日志记录**：整个 Saga 事务的执行过程包括每个正向操作的开始、成功、失败和每个补偿操作的开始、成功、失败都记录到事务日志中，日志包含时间戳、操作标识、执行结果和错误详情，事务日志支持按事务 ID 查询完整执行轨迹。无补偿函数的步骤不产生补偿日志条目。
- **并发执行保护**：协调器对同一个 Saga 定义在同一时刻只允许一个执行实例运行，防止并发执行导致状态冲突，不同 Saga 定义可以并发执行。

## 核心结构体

### Step

步骤定义，描述 Saga 中的一个操作步骤：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 步骤唯一标识 |
| Name | string | 步骤名称（描述性） |
| ForwardFunc | StepFunc | 正向操作函数，签名为 `func(ctx context.Context, data map[string]interface{}) (interface{}, error)` |
| CompensateFunc | StepFunc | 补偿操作函数，签名与正向操作相同，可为 nil 表示无需补偿 |

### StepResult

步骤执行结果：

| 字段 | 类型 | 说明 |
|------|------|------|
| StepID | string | 操作标识（正向操作使用步骤 ID，补偿操作使用 `步骤ID-compensate`） |
| Status | OperationStatus | 最终状态（Pending / Running / Success / Failed） |
| Output | interface{} | 步骤输出值 |
| Error | error | 执行错误 |
| StartTime | time.Time | 开始执行时间 |
| EndTime | time.Time | 执行结束时间 |
| Duration | time.Duration | 执行耗时 |

### LogEntry

事务日志条目：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 日志条目唯一标识 |
| TransactionID | string | 事务执行 ID |
| StepID | string | 关联的操作标识（正向操作为步骤 ID，补偿操作为 `步骤ID-compensate`；为空表示事务级日志） |
| OperationType | OperationType | 操作类型（Forward / Compensation） |
| Status | OperationStatus | 操作状态 |
| Timestamp | time.Time | 日志记录时间 |
| Error | error | 错误信息（如有） |
| Details | string | 详细描述信息 |

### CompensationFailure

补偿失败记录，用于人工干预：

| 字段 | 类型 | 说明 |
|------|------|------|
| TransactionID | string | 事务执行 ID |
| StepID | string | 补偿操作标识，格式为 `步骤ID-compensate`（如 `step1-compensate`），唯一标识补偿操作本身 |
| ForwardStepID | string | 正向操作标识，即原始步骤 ID（如 `step1`），标识该补偿操作对应的正向操作 |
| FailureReason | string | 失败原因描述 |
| Error | error | 原始错误对象 |
| FailureTime | time.Time | 失败发生时间 |
| Resolved | bool | 是否已解决 |
| ResolutionNotes | string | 解决备注 |

**StepID 与 ForwardStepID 的区别**：

- `ForwardStepID`：标识补偿操作对应的正向步骤，值为步骤注册时的 ID（如 `reserve-inventory`）
- `StepID`：标识补偿操作本身，格式为 `步骤ID-compensate`（如 `reserve-inventory-compensate`），确保与正向操作标识区分

这两个字段的独立设计使得在人工干预时可以同时追溯失败的补偿操作和其对应的正向操作。

### Saga

Saga 事务定义，包含一组步骤：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | Saga 唯一标识 |
| Name | string | Saga 名称 |
| Steps | []*Step | 按执行顺序排列的步骤列表 |

### SagaExecution

Saga 执行实例，表示一次具体的事务执行：

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | string | 执行实例唯一标识 |
| SagaID | string | 关联的 Saga 定义 ID |
| Name | string | Saga 名称 |
| Status | OperationStatus | 整体执行状态 |
| Steps | []*Step | 本次执行的步骤副本 |
| StepResults | map[string]*StepResult | 正向操作执行结果映射（键为步骤 ID） |
| Compensations | map[string]*StepResult | 补偿操作执行结果映射（键为 `步骤ID-compensate`） |
| Data | map[string]interface{} | 上下文数据，用于步骤间传递数据 |
| Error | error | 整体执行错误；补偿失败时包含 `ErrCompensationFailed` 哨兵错误 |
| StartTime | time.Time | 开始时间 |
| EndTime | time.Time | 结束时间 |
| Duration | time.Duration | 执行耗时 |
| NeedsIntervention | bool | 是否需要人工干预 |
| InterventionNotes | []*CompensationFailure | 人工干预记录列表 |

### Coordinator

协调器主结构体，管理 Saga 定义、执行和日志：

| 方法 | 说明 |
|------|------|
| NewCoordinator() | 创建新的协调器实例 |
| NewSaga(id, name) (*Saga, error) | 创建新的 Saga 定义 |
| GetSaga(id) (*Saga, error) | 获取 Saga 定义 |
| RemoveSaga(id) error | 删除 Saga 定义 |
| Execute(ctx, sagaID, initialData) (*SagaExecution, error) | 执行指定的 Saga 事务；同一 Saga 并发执行返回 `ErrSagaRunning` |
| GetExecution(executionID) (*SagaExecution, error) | 获取执行结果 |
| GetExecutionsBySaga(sagaID) []*SagaExecution | 获取指定 Saga 的所有执行记录 |
| GetLogs(transactionID) ([]*LogEntry, error) | 按事务 ID 查询执行日志 |
| GetAllLogs() []*LogEntry | 获取所有日志 |
| GetPendingInterventions() []*CompensationFailure | 获取待人工干预的事务列表 |
| ResolveIntervention(transactionID, stepID, notes) error | 标记人工干预为已解决（stepID 为补偿操作标识 `步骤ID-compensate`） |

### Saga 方法

| 方法 | 说明 |
|------|------|
| AddStep(id, name, forwardFunc, compensateFunc) error | 添加步骤，指定 ID、名称、正向函数和补偿函数 |
| GetStep(id) (*Step, error) | 获取步骤定义 |

## 操作状态

```
Pending → Running → Success
                  → Failed
```

- **Pending**：等待执行
- **Running**：正在执行
- **Success**：执行成功
- **Failed**：执行失败

## 操作类型

- **Forward**：正向操作
- **Compensation**：补偿操作

## 错误分类策略

模块定义了多个哨兵错误变量，按语义分类如下：

### Saga 定义相关

| 错误 | 说明 | 触发场景 |
|------|------|----------|
| ErrSagaNotFound | Saga 定义不存在 | `GetSaga`、`RemoveSaga`、`Execute` 查询的 sagaID 不存在 |
| ErrSagaAlreadyExists | Saga 定义已存在 | `NewSaga` 创建重复 ID 的 Saga |
| ErrStepNotFound | 步骤不存在 | `GetStep` 查询不存在的步骤 |
| ErrStepAlreadyExists | 步骤已存在 | `AddStep` 添加重复 ID 的步骤 |
| ErrInvalidStepID | 步骤 ID 无效 | `AddStep` 传入空字符串作为步骤 ID |
| ErrNoStepsRegistered | Saga 没有注册任何步骤 | `Execute` 执行没有步骤的 Saga |

### 执行相关

| 错误 | 说明 | 触发场景 |
|------|------|----------|
| ErrExecutionNotFound | 执行记录不存在 | `GetExecution`、`GetLogs` 查询不存在的执行 ID 或事务 ID |
| ErrSagaRunning | Saga 正在执行中 | `Execute` 对同一 Saga 并发执行 |

### 补偿相关

| 错误 | 说明 | 触发场景 |
|------|------|----------|
| ErrCompensationFailed | 补偿操作失败 | `Execute` 执行后补偿失败，通过 `errors.Is(result.Error, ErrCompensationFailed)` 检查 |
| ErrInterventionNotFound | 人工干预记录不存在 | `ResolveIntervention` 查询不存在的干预记录 |

### 错误语义区分

- `ErrSagaNotFound` vs `ErrExecutionNotFound`：前者表示 Saga 定义（模板）不存在，后者表示具体的执行实例或其日志不存在。`GetSaga`/`RemoveSaga`/`Execute` 使用 `ErrSagaNotFound`；`GetExecution`/`GetLogs` 使用 `ErrExecutionNotFound`。
- `ErrCompensationFailed` 在 `SagaExecution.Error` 中通过 `fmt.Errorf("%w: ...")` 包装，可通过 `errors.Is()` 检查。当且仅当补偿操作失败时才会包含此错误。

## Saga 正向执行流程

```
┌─────────────────────────────────────────────────────────┐
│  Execute(ctx, sagaID, initialData)                      │
│                                                         │
│  1. 查找 Saga 定义，检查是否有步骤                       │
│  2. 检查是否有正在执行的同一 Saga（并发保护）             │
│  3. 创建执行实例，生成唯一执行 ID                         │
│  4. 复制初始数据到上下文 data map                        │
│  5. 标记 Saga 为运行中                                  │
│  6. 记录事务开始日志                                     │
│                                                         │
│  ┌──────────── 正向执行循环 ─────────────┐               │
│  │                                        │               │
│  │  按注册顺序遍历步骤：                  │               │
│  │                                        │               │
│  │  ┌──────────────────────────────┐     │               │
│  │  │  记录步骤开始日志            │     │               │
│  │  │  调用 ForwardFunc(ctx, data) │     │               │
│  │  │  捕获 panic 转换为 error     │     │               │
│  │  └──────────────────────────────┘     │               │
│  │                                        │               │
│  │  成功：                                │               │
│  │    • 记录成功日志                      │               │
│  │    • 将 Output 存入 data[stepID]       │               │
│  │    • 加入成功步骤列表                  │               │
│  │    • 继续执行下一步                    │               │
│  │                                        │               │
│  │  失败：                                │               │
│  │    • 记录失败日志                      │               │
│  │    • 终止正向执行                      │               │
│  │    • 进入补偿回滚流程                  │               │
│  │    • 跳出循环                          │               │
│  └────────────────────────────────────────┘               │
│                                                         │
│  7. 全部成功：                                           │
│     • 标记状态为 Success                                 │
│     • 记录事务成功日志                                   │
│                                                         │
│  8. 有失败：                                             │
│     • 标记状态为 Failed                                  │
│     • 记录事务失败日志                                   │
│     • 执行补偿回滚流程                                   │
│     • 如有补偿失败，包装 ErrCompensationFailed            │
│                                                         │
│  9. 清除 Saga 运行标记                                  │
│  10. 返回执行结果                                        │
└─────────────────────────────────────────────────────────┘
```

### 上下文数据传递

每个正向操作的返回值 `Output` 会被自动存入上下文数据 `data` 中，以步骤 ID 作为键。后续步骤可以通过 `data[previousStepID]` 访问前序步骤的输出。

初始数据 `initialData` 在执行开始时被复制到 `data` 中，所有步骤都可以访问这些初始数据。

**重要**：`data` 是执行实例私有的，不会修改传入的 `initialData` map。

## 补偿回滚流程

```
┌─────────────────────────────────────────────────────────┐
│  补偿回滚（正向执行失败后触发）                          │
│                                                         │
│  成功步骤列表：[step1, step2, step3] （按执行顺序）      │
│                                                         │
│  按反序遍历：i = len-1 → 0                              │
│    当前步骤：step3 → step2 → step1                      │
│                                                         │
│  ┌─────────────────────────────────────────────┐        │
│  │  步骤无补偿函数（CompensateFunc == nil）    │        │
│  │    • 直接跳过，不记录任何补偿日志            │        │
│  │    • 继续处理上一个步骤                      │        │
│  └─────────────────────────────────────────────┘        │
│                                                         │
│  ┌─────────────────────────────────────────────┐        │
│  │  步骤有补偿函数                              │        │
│  │    • 生成补偿操作标识 compStepID             │        │
│  │      （格式：步骤ID + "-compensate"）        │        │
│  │    • 记录补偿开始日志（StepID=compStepID）   │        │
│  │    • 调用 CompensateFunc(ctx, data)         │        │
│  │    • 捕获 panic 转换为 error                 │        │
│  └─────────────────────────────────────────────┘        │
│                                                         │
│  补偿成功：                                              │
│    • 记录补偿成功日志（StepID=compStepID）               │
│    • 结果存入 Compensations[compStepID]                 │
│    • 继续回滚上一个步骤                                  │
│                                                         │
│  补偿失败：                                              │
│    • 记录补偿失败日志（StepID=compStepID）               │
│    • 结果存入 Compensations[compStepID]                 │
│    • 创建 CompensationFailure 记录                      │
│      - StepID = compStepID（补偿操作标识）              │
│      - ForwardStepID = step.ID（正向操作标识）          │
│    • 标记事务 NeedsIntervention = true                  │
│    • 加入待人工干预列表                                  │
│    • 继续回滚上一个步骤（不中断整体回滚）                │
│                                                         │
│  补偿全部完成后：                                        │
│    • 所有成功步骤都已尝试回滚（无论补偿是否成功）        │
│    • 如有补偿失败，事务标记为需要人工干预                │
│    • 执行结果 Error 包装 ErrCompensationFailed 哨兵      │
└─────────────────────────────────────────────────────────┘
```

### 关键设计：补偿失败不中断

单个补偿操作失败不会中断整个回滚流程，协调器会继续尝试回滚剩余的步骤。这样可以最大程度地恢复系统状态，即使某些补偿操作失败了。

失败的补偿会被记录下来并标记为需要人工干预，由外部系统或人工进行后续处理。

### 关键设计：无补偿函数的步骤不产生日志

对于没有注册补偿函数的步骤（`CompensateFunc == nil`），在回滚阶段直接跳过，不会产生任何 `OpTypeCompensation` 类型的日志条目。这避免了在事务日志中出现虚假的"补偿成功"记录，确保日志中记录的补偿操作都是实际执行的。

### 关键设计：补偿操作标识独立于正向操作标识

补偿操作的标识（如 `step1-compensate`）与正向操作标识（如 `step1`）是独立的，体现在：

- **日志 StepID**：正向操作日志的 StepID 为步骤 ID，补偿操作日志的 StepID 为 `步骤ID-compensate`
- **Compensations 映射键**：使用 `步骤ID-compensate` 作为键，与 StepResults 映射的步骤 ID 键区分
- **CompensationFailure 字段**：`StepID` 为补偿操作标识，`ForwardStepID` 为正向操作标识

## 人工干预流程

```
补偿失败
    ↓
创建 CompensationFailure 记录
    - StepID = "step1-compensate"（补偿操作标识）
    - ForwardStepID = "step1"（正向操作标识）
    ↓
标记事务 NeedsIntervention = true
    ↓
加入 pendingInterventions 列表
    ↓
执行结果 Error 包装 ErrCompensationFailed
    ↓
外部系统轮询 GetPendingInterventions()
    ↓
人工处理补偿失败
    ↓
调用 ResolveIntervention(transactionID, compStepID, notes)
    注意：compStepID 为补偿操作标识（如 "step1-compensate"）
    ↓
标记 Resolved = true，记录解决备注
    ↓
如所有干预都已解决，清除 NeedsIntervention 标记
```

### 轮询处理示例

外部系统可以定期轮询待处理的人工干预：

```go
for {
    pending := coordinator.GetPendingInterventions()
    for _, failure := range pending {
        fmt.Printf("补偿操作 %s 失败（对应正向操作 %s）: %s\n",
            failure.StepID, failure.ForwardStepID, failure.FailureReason)

        // 人工处理逻辑...

        coordinator.ResolveIntervention(
            failure.TransactionID,
            failure.StepID, // 使用补偿操作标识
            "手动补偿完成",
        )
    }
    time.Sleep(1 * time.Minute)
}
```

## 事务日志查询

每个事务执行产生完整的日志轨迹，包括：

- 事务开始/结束
- 每个正向操作的开始/成功/失败（StepID = 步骤 ID）
- 每个补偿操作的开始/成功/失败（StepID = `步骤ID-compensate`）

按事务 ID 查询可以获得完整的执行轨迹，用于问题排查和审计。

**注意**：无补偿函数的步骤不会产生任何补偿日志条目。

日志条目示例（按时间顺序）：

| StepID | 操作类型 | 状态 | 说明 |
|--------|----------|------|------|
| - | Forward | Running | Saga execution started |
| step1 | Forward | Running | Step 0: Reserve Inventory starting |
| step1 | Forward | Success | Step 0: Reserve Inventory completed successfully |
| step2 | Forward | Running | Step 1: Charge Payment starting |
| step2 | Forward | Success | Step 1: Charge Payment completed successfully |
| step3 | Forward | Running | Step 2: Create Order starting |
| step3 | Forward | Failed | Step 2: Create Order failed |
| - | Forward | Failed | Saga execution failed, starting compensation |
| step2-compensate | Compensation | Running | Compensation for step Charge Payment starting |
| step2-compensate | Compensation | Success | Compensation for step Charge Payment completed successfully |
| step1-compensate | Compensation | Running | Compensation for step Reserve Inventory starting |
| step1-compensate | Compensation | Success | Compensation for step Reserve Inventory completed successfully |

## 并发执行保护

协调器对同一 Saga 定义实施并发执行保护：

- 同一 Saga 在同一时刻只允许一个执行实例
- 并发执行同一 Saga 会返回 `ErrSagaRunning` 错误
- 不同 Saga 之间可以并发执行
- Saga 执行完成后（无论成功或失败），允许后续执行

```go
// 同一 Saga 顺序执行：允许
result1, _ := coordinator.Execute(ctx, "order-saga", data1)
result2, _ := coordinator.Execute(ctx, "order-saga", data2) // 前一个完成后可执行

// 同一 Saga 并发执行：拒绝
go coordinator.Execute(ctx, "order-saga", data1)
_, err := coordinator.Execute(ctx, "order-saga", data2)
// err == ErrSagaRunning

// 不同 Saga 并发执行：允许
go coordinator.Execute(ctx, "order-saga-1", data1)
coordinator.Execute(ctx, "order-saga-2", data2) // 正常执行
```

## 使用示例

### 基本用法：订单处理 Saga

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "solocoder-go/internal/saga"
)

func main() {
    coordinator := saga.NewCoordinator()

    orderSaga, _ := coordinator.NewSaga("order-processing", "订单处理流程")

    orderSaga.AddStep("reserve-inventory", "预留库存",
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            orderID := data["order_id"].(string)
            fmt.Printf("预留库存: 订单 %s\n", orderID)
            return "inventory-reserved-123", nil
        },
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            fmt.Println("释放库存")
            return nil, nil
        },
    )

    orderSaga.AddStep("charge-payment", "扣款",
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            inventoryResult := data["reserve-inventory"].(string)
            fmt.Printf("扣款: 库存 %s\n", inventoryResult)
            return "payment-charged-456", nil
        },
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            fmt.Println("退款操作")
            return nil, nil
        },
    )

    orderSaga.AddStep("create-order", "创建订单",
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            paymentResult := data["charge-payment"].(string)
            fmt.Printf("创建订单: 支付 %s\n", paymentResult)
            return "order-created-789", nil
        },
        func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
            fmt.Println("取消订单")
            return nil, nil
        },
    )

    initialData := map[string]interface{}{
        "order_id": "ORDER-2024-001",
        "user_id":  "user-123",
    }

    result, err := coordinator.Execute(context.Background(), "order-processing", initialData)
    if err != nil {
        if errors.Is(err, saga.ErrSagaRunning) {
            fmt.Println("该 Saga 正在执行中，请稍后重试")
            return
        }
        fmt.Printf("执行错误: %v\n", err)
        return
    }

    fmt.Printf("执行结果: %s\n", result.Status)
    if result.Status == saga.StatusSuccess {
        fmt.Println("订单处理成功!")
        fmt.Printf("订单ID: %s\n", result.Data["create-order"])
    } else {
        fmt.Printf("执行失败: %v\n", result.Error)
        if errors.Is(result.Error, saga.ErrCompensationFailed) {
            fmt.Println("补偿操作失败，需要人工干预!")
            for _, note := range result.InterventionNotes {
                fmt.Printf("  - 补偿操作 %s（正向操作 %s）: %s\n",
                    note.StepID, note.ForwardStepID, note.FailureReason)
            }
        }
        if result.NeedsIntervention {
            fmt.Println("需要人工干预!")
        }
    }
}
```

### 补偿失败场景

```go
orderSaga.AddStep("reserve-inventory", "预留库存",
    func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
        return "inventory-reserved-123", nil
    },
    func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
        return nil, fmt.Errorf("库存系统不可用，无法释放库存")
    },
)

result, _ := coordinator.Execute(context.Background(), "order-processing", initialData)

if errors.Is(result.Error, saga.ErrCompensationFailed) {
    fmt.Println("补偿失败，检查人工干预记录")
}

pending := coordinator.GetPendingInterventions()
for _, failure := range pending {
    fmt.Printf("补偿操作 %s 失败（正向操作 %s）\n",
        failure.StepID, failure.ForwardStepID)
    fmt.Printf("  原因: %s\n", failure.FailureReason)

    coordinator.ResolveIntervention(
        failure.TransactionID,
        failure.StepID,
        "已手动联系仓库释放库存",
    )
}
```

### 查询事务日志

```go
logs, err := coordinator.GetLogs(result.ID)
if err != nil {
    if errors.Is(err, saga.ErrExecutionNotFound) {
        fmt.Println("执行记录不存在")
        return
    }
    fmt.Printf("查询日志失败: %v\n", err)
    return
}

fmt.Println("\n执行日志:")
for _, log := range logs {
    stepInfo := "事务"
    if log.StepID != "" {
        stepInfo = fmt.Sprintf("操作 %s", log.StepID)
    }
    fmt.Printf("[%s] %s %s: %s - %s\n",
        log.Timestamp.Format("15:04:05"),
        stepInfo,
        log.OperationType,
        log.Status,
        log.Details,
    )
    if log.Error != nil {
        fmt.Printf("  错误: %v\n", log.Error)
    }
}
```

### 查询执行记录

```go
exec, err := coordinator.GetExecution(executionID)
if err != nil {
    if errors.Is(err, saga.ErrExecutionNotFound) {
        fmt.Println("执行记录不存在")
        return
    }
}

executions := coordinator.GetExecutionsBySaga("order-processing")
fmt.Printf("订单处理 Saga 共执行 %d 次\n", len(executions))
```

### 并发执行

不同 Saga 可以并发执行，同一 Saga 顺序执行：

```go
var wg sync.WaitGroup
orders := []string{"ORDER-001", "ORDER-002", "ORDER-003"}

for _, orderID := range orders {
    wg.Add(1)
    go func(id string) {
        defer wg.Done()
        sagaID := fmt.Sprintf("order-processing-%s", id)
        saga, _ := coordinator.NewSaga(sagaID, fmt.Sprintf("订单 %s", id))
        // ... 添加步骤
        result, err := coordinator.Execute(context.Background(), sagaID, initialData)
        if errors.Is(err, saga.ErrSagaRunning) {
            fmt.Printf("订单 %s 的 Saga 正在执行中\n", id)
            return
        }
        fmt.Printf("订单 %s 执行结果: %s\n", id, result.Status)
    }(orderID)
}

wg.Wait()
```

### Context 取消

```go
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(100 * time.Millisecond)
    cancel()
}()

result, _ := coordinator.Execute(ctx, "order-processing", initialData)
// 正在执行的步骤会收到 ctx.Done() 信号
// 已成功的步骤会执行补偿回滚
```

### Panic 恢复

```go
orderSaga.AddStep("risky-step", "风险步骤",
    func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
        panic("something went wrong")
    },
    func(ctx context.Context, data map[string]interface{}) (interface{}, error) {
        return nil, nil
    },
)

result, _ := coordinator.Execute(ctx, "order-processing", nil)
// result.Status == StatusFailed
// result.Error 包含 panic 信息
// 自动触发补偿流程
```

## 最佳实践

1. **步骤幂等性**：正向操作和补偿操作都应设计为幂等的，防止重复执行导致数据不一致。

2. **补偿逻辑完整**：每个正向操作都应该有对应的补偿操作，除非该操作没有副作用（如纯计算）。

3. **避免长事务**：Saga 适合处理长事务，但每个步骤应尽量短，避免长时间锁定资源。

4. **监控人工干预**：建立监控机制，及时发现并处理需要人工干预的补偿失败。

5. **日志审计**：事务日志是排查问题的重要依据，应妥善保存。

6. **超时处理**：在步骤函数中正确处理 context 超时，避免悬挂事务。

7. **并发控制**：避免对同一 Saga 并发调用 `Execute`，使用 `ErrSagaRunning` 检测并重试。

8. **错误分类**：使用 `errors.Is()` 检查错误类型，区分 Saga 定义不存在（`ErrSagaNotFound`）、执行记录不存在（`ErrExecutionNotFound`）和补偿失败（`ErrCompensationFailed`）。
