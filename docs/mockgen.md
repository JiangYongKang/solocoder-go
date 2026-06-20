# MockGen 模块需求文档

## 1. 模块概述

MockGen 是一个轻量级的 Mock 测试框架，用于在单元测试中创建接口的 Mock 实现。通过 MockGen，开发者可以：

- 动态生成接口的 Mock 实现
- 配置方法调用的期望行为
- 验证方法调用的次数和参数
- 自定义参数匹配逻辑
- 灵活配置返回值和行为

## 2. 核心功能

### 2.1 接口自动 Mock 实现生成

提供 `CreateMock` 函数，用户传入一个接口类型，框架返回该接口的 Mock 代理实例。Mock 实例自动实现接口的所有方法，未配置期望的方法调用时返回零值。

### 2.2 期望调用参数匹配

支持为 Mock 实例的每个方法设置期望调用，期望调用可指定输入参数的匹配方式：

- **精确匹配**：通过直接传入参数值进行精确匹配
- **自定义匹配函数**：通过 `Matches` 函数传入自定义匹配逻辑，接收实际参数返回是否匹配的布尔值
- **任意匹配**：通过 `Any()` 匹配任意参数值

### 2.3 返回值与行为配置

为期望调用配置对应的返回值和执行行为，支持两种模式：

- **固定返回值**：通过 `Return` 方法设置固定的返回值
- **回调函数动态计算**：通过 `Run` 方法传入回调函数，接收实际调用参数并返回计算结果

### 2.4 调用次数断言

支持为期望调用设置期望的调用次数：

- 精确次数：`Times(n)` - 恰好调用 n 次
- 最小次数：`MinTimes(n)` - 至少调用 n 次
- 最大次数：`MaxTimes(n)` - 最多调用 n 次
- 一次：`Once()` - 恰好调用 1 次
- 从不：`Never()` - 不允许调用

每个期望调用独立计数，验证时如果实际调用次数不在期望范围内则报告失败。未设置期望的方法如果被调用也视为失败。

### 2.5 未匹配调用的失败报告

当 Mock 方法被调用但找不到任何匹配的期望时，生成详细的失败报告，包含：

- 方法名称
- 实际传入的参数值
- 已注册但未匹配的期望列表

报告以错误信息字符串形式输出，便于在测试中直接断言。

## 3. 核心结构体职责

### 3.1 MockController

Mock 控制器，是框架的核心入口。负责管理所有的期望调用、记录调用历史、执行验证。

主要职责：
- 创建和管理 Mock 实例
- 注册方法调用期望
- 处理方法调用并返回结果
- 验证调用是否符合期望

### 3.2 Expectation

表示一个方法调用的期望配置。

主要职责：
- 存储方法名称和参数匹配器
- 存储返回值或返回函数
- 存储调用次数限制
- 记录实际调用次数
- 验证调用次数是否符合期望

### 3.3 ExpectationBuilder

期望构建器，提供链式调用 API 来配置期望。

主要职责：
- 提供流畅的链式 API
- 配置返回值
- 配置回调函数
- 配置调用次数限制

### 3.4 MockProxy

接口 Mock 代理，包装 MockController 并提供接口方法的动态代理。

主要职责：
- 包装 MockController
- 提供接口方法的动态函数
- 委托调用给 MockController

### 3.5 Mock

底层的 Mock 状态存储。

主要职责：
- 存储所有已注册的期望
- 记录未匹配的调用
- 提供查找匹配期望的方法
- 提供验证功能

### 3.6 Matcher

参数匹配器函数类型。

```go
type Matcher func(actual interface{}) bool
```

## 4. 期望调用的匹配优先级规则

当一个方法被调用时，框架按照以下规则查找匹配的期望：

1. **按注册顺序查找**：期望按照注册的顺序进行匹配，先注册的期望具有更高的优先级。
2. **第一个匹配即返回**：找到第一个匹配的期望后立即返回，不再继续查找后续的期望。
3. **参数数量必须匹配**：期望的参数匹配器数量必须与实际调用的参数数量相同，否则不匹配。
4. **所有参数都匹配才算匹配**：每个参数位置的匹配器都必须匹配对应的实际参数。

**注意**：由于是按注册顺序匹配，更具体的匹配应该先注册，更通用的匹配（如 `Any()`）应该后注册，否则更通用的匹配会先被匹配到。

## 5. 使用示例

### 5.1 基本使用

```go
package mockgen_test

import (
    "testing"
    "solocoder-go/internal/mockgen"
)

type UserService interface {
    GetUser(id int) (string, error)
    Add(a, b int) int
}

func TestBasicMock(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    mock.On("GetUser", 1).Return("user1", nil)

    getUserName, _ := mock.Method("GetUser").(func(int) (string, error))
    name, err := getUserName(1)

    if name != "user1" {
        t.Errorf("expected user1, got %s", name)
    }
    if err != nil {
        t.Errorf("expected nil error, got %v", err)
    }

    if err := mock.Verify(); err != nil {
        t.Errorf("verification failed: %v", err)
    }
}
```

### 5.2 使用任意匹配器

```go
func TestAnyMatcher(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    mock.On("GetUser", mockgen.Any()).Return("any_user", nil)

    getUserName, _ := mock.Method("GetUser").(func(int) (string, error))
    
    name, _ := getUserName(123)
    if name != "any_user" {
        t.Errorf("expected any_user, got %s", name)
    }
    
    name, _ = getUserName(456)
    if name != "any_user" {
        t.Errorf("expected any_user, got %s", name)
    }
}
```

### 5.3 使用自定义匹配器

```go
func TestCustomMatcher(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    isPositive := mockgen.Matches(func(v interface{}) bool {
        n, ok := v.(int)
        return ok && n > 0
    })

    mock.On("GetUser", isPositive).Return("positive_user", nil)

    getUserName, _ := mock.Method("GetUser").(func(int) (string, error))
    
    name, _ := getUserName(10)
    if name != "positive_user" {
        t.Errorf("expected positive_user, got %s", name)
    }
}
```

### 5.4 使用回调函数动态返回值

```go
func TestCallbackReturn(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    mock.On("Add", mockgen.Any(), mockgen.Any()).Run(func(a, b int) int {
        return a + b
    })

    addFunc, _ := mock.Method("Add").(func(int, int) int)
    
    result := addFunc(2, 3)
    if result != 5 {
        t.Errorf("expected 5, got %d", result)
    }
}
```

### 5.5 调用次数验证

```go
func TestCallCount(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    mock.On("GetUser", 1).Return("user1", nil).Times(2)

    getUserName, _ := mock.Method("GetUser").(func(int) (string, error))
    
    getUserName(1)
    getUserName(1)

    if err := mock.Verify(); err != nil {
        t.Errorf("verification should pass, got: %v", err)
    }

    getUserName(1) // 第三次调用，超过期望

    if err := mock.Verify(); err == nil {
        t.Error("verification should fail for too many calls")
    }
}
```

### 5.6 未匹配调用报告

```go
func TestUnmatchedCallReport(t *testing.T) {
    mock := mockgen.CreateMock((*UserService)(nil))

    mock.On("GetUser", 1).Return("user1", nil)

    getUserName, _ := mock.Method("GetUser").(func(int) (string, error))
    getUserName(999) // 不匹配的调用

    report := mock.VerifyVerbose()
    if report == "" {
        t.Error("expected non-empty report")
    }
}
```

### 5.7 使用 MockController 直接调用

```go
func TestMockControllerDirect(t *testing.T) {
    mc := mockgen.NewMockController()

    mc.On("Greet", "World").Return("Hello, World!")

    results := mc.Call("Greet", "World")
    if results[0] != "Hello, World!" {
        t.Errorf("expected Hello, World!, got %v", results[0])
    }
}
```

## 6. 错误类型

| 错误 | 说明 |
|------|------|
| `ErrNoMatchingExpectation` | 找不到匹配的期望调用 |
| `ErrCallCountMismatch` | 调用次数不符合期望 |
| `ErrInvalidInterface` | 无效的接口类型 |
| `ErrMethodNotFound` | 方法未找到 |

## 7. API 参考

### 7.1 CreateMock

```go
func CreateMock(iface interface{}) *MockProxy
```

创建一个接口的 Mock 代理实例。

**参数**：
- `iface`：接口类型的 nil 指针，例如 `(*MyInterface)(nil)`

**返回值**：
- `*MockProxy`：Mock 代理实例

**注意**：如果传入的不是接口类型，会 panic。

### 7.2 NewMockController

```go
func NewMockController() *MockController
```

创建一个新的 Mock 控制器。

### 7.3 On

```go
func (mc *MockController) On(methodName string, args ...interface{}) *ExpectationBuilder
```

为指定方法注册一个调用期望。

**参数**：
- `methodName`：方法名
- `args`：参数列表，可以是具体值或 Matcher

### 7.4 Return

```go
func (eb *ExpectationBuilder) Return(returnValues ...interface{}) *ExpectationBuilder
```

设置期望调用的固定返回值。

### 7.5 Run

```go
func (eb *ExpectationBuilder) Run(fn interface{}) *ExpectationBuilder
```

设置期望调用的回调函数，动态计算返回值。

### 7.6 Times

```go
func (eb *ExpectationBuilder) Times(n int) *ExpectationBuilder
```

设置期望的精确调用次数。

### 7.7 MinTimes / MaxTimes

```go
func (eb *ExpectationBuilder) MinTimes(n int) *ExpectationBuilder
func (eb *ExpectationBuilder) MaxTimes(n int) *ExpectationBuilder
```

设置期望的最小/最大调用次数。

### 7.8 Once / Never

```go
func (eb *ExpectationBuilder) Once() *ExpectationBuilder
func (eb *ExpectationBuilder) Never() *ExpectationBuilder
```

设置恰好调用一次 / 不允许调用。

### 7.9 Verify

```go
func (mc *MockController) Verify() error
```

验证所有期望是否都满足。

### 7.10 VerifyVerbose

```go
func (mc *MockController) VerifyVerbose() string
```

返回详细的验证报告，包含未匹配调用和调用次数不匹配的信息。

### 7.11 Any

```go
func Any() Matcher
```

返回一个匹配任意值的匹配器。

### 7.12 Matches

```go
func Matches(fn func(interface{}) bool) Matcher
```

创建一个自定义匹配器。

### 7.13 Method

```go
func (mp *MockProxy) Method(methodName string) interface{}
```

返回接口中指定方法的动态函数，可类型转换为对应的函数类型后调用。

## 8. 注意事项

1. **线程安全**：Mock 框架是线程安全的，可以在并发场景下使用。
2. **零值返回**：未配置期望的方法被调用时，返回对应类型的零值，并记录为未匹配调用。
3. **匹配顺序**：期望按注册顺序匹配，更具体的匹配应先注册。
4. **接口要求**：`CreateMock` 函数要求传入接口类型，否则会 panic。
