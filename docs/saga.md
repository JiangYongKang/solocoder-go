# Saga 分布式事务协调器

## 模块功能

Saga 分布式事务协调器用于管理分布式系统中的长事务，通过将大事务拆分为多个本地事务（步骤），并为每个步骤提供补偿操作来实现最终一致性。核心能力包括：

- **正向操作顺序执行**：支持将一组正向操作按注册顺序依次执行，每个正向操作是一个可执行函数返回执行结果，正向操作之间可传递上下文数据用于后续操作使用前序操作的输出，任意一个正向操作执行失败时停止执行后续操作并触发补偿流程。
- **补偿事务回滚**：每个正向操作在注册时需同时注册对应的补偿操作，当正向操作链执行到某一步失败时事务协调器按照已成功执行的正向操作的反序依次调用各操作的补偿函数，补偿操作执行成功则继续回滚前一个操作，补偿失败时记录失败信息并继续回滚剩余操作不因单个补偿失败而中断整体回滚流程。
- **补偿失败人工干预**：当补偿操作执行失败时将对应的事务记录标记为需要人工干预状态，标记信息包含失败的正向操作和补偿操作的标识、失败原因和执行时间，提供查询接口列出所有待人工干预的事务供外部系统轮询处理。
- **事务日志记录**：整个 Saga 事务的执行过程包括每个正向操作的开始、成功、失败和每个补偿操作的开始、成功、失败都记录到事务日志中，日志包含时间戳、操作标识、执行结果和错误详情，事务日志支持按事务 ID 查询完整执行轨迹。

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
| StepID | string | 步骤 ID |
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
| StepID | string | 关联的步骤 ID（可为空表示事务级日志） |
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
| StepID | string | 失败的补偿步骤 ID |
| ForwardStepID | string | 对应的正向步骤 ID |
| FailureReason | string | 失败原因描述 |
| Error | error | 原始错误对象 |
| FailureTime | time.Time | 失败发生时间 |
| Resolved | bool | 是否已解决 |
| ResolutionNotes | string | 解决备注 |

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
| StepResults | map[string]*StepResult | 正向操作执行结果映射 |
| Compensations | map[string]*StepResult | 补偿操作执行结果映射 |
| Data | map[string]interface{} | 上下文数据，用于步骤间传递数据 |
| Error | error | 整体执行错误（如有） |
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
| Execute(ctx, sagaID, initialData) (*SagaExecution, error) | 执行指定的 Saga 事务 |
| GetExecution(executionID) (*SagaExecution, error) | 获取执行结果 |
| GetExecutionsBySaga(sagaID) []*SagaExecution | 获取指定 Saga 的所有执行记录 |
| GetLogs(transactionID) ([]*LogEntry, error) | 按事务 ID 查询执行日志 |
| GetAllLogs() []*LogEntry | 获取所有日志 |
| GetPendingInterventions() []*CompensationFailure | 获取待人工干预的事务列表 |
| ResolveIntervention(transactionID, stepID, notes) error | 标记人工干预为已解决 |

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

## 错误定义

| 错误 | 说明 |
|------|------|
| ErrSagaNotFound | Saga 不存在 |
| ErrSagaAlreadyExists | Saga 已存在 |
| ErrStepNotFound | 步骤不存在 |
| ErrStepAlreadyExists | 步骤已存在 |
| ErrInvalidStepID | 步骤 ID 无效 |
| ErrNoStepsRegistered | Saga 没有注册任何步骤 |
| ErrCompensationFailed | 补偿操作失败 |
| ErrInterventionNotFound | 人工干预记录不存在 |

## Saga 正向执行流程

```
┌─────────────────────────────────────────────────────────┐
│  Execute(ctx, sagaID, initialData)                      │
│                                                         │
│  1. 查找 Saga 定义，检查是否有步骤                       │
│  2. 创建执行实例，生成唯一执行 ID                         │
│  3. 复制初始数据到上下文 data map                        │
│  4. 记录事务开始日志                                     │
│                                                         │
│  ┌──────────── 正向执行循环 ─────────────┐               │
│  │                                        │               │
│  │  按注册顺序遍历步骤：                  │               │
│  │                                        │               │
│  │  ┌────────────────