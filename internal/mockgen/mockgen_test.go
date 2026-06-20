package mockgen

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

type TestService interface {
	GetUser(id int) (string, error)
	Add(a, b int) int
	Greet(name string) string
	VoidMethod()
	MultiReturn() (string, int, bool)
}

func TestNewMockController(t *testing.T) {
	mc := NewMockController()
	if mc == nil {
		t.Fatal("expected non-nil MockController")
	}
	if mc.Mock() == nil {
		t.Fatal("expected non-nil Mock")
	}
}

func TestExpectation_ExactMatch_ExactArgs(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	results := mc.Call("GetUser", 1)

	if len(results) != 2 {
		t.Fatalf("expected 2 return values, got %d", len(results))
	}
	if results[0] != "user1" {
		t.Errorf("expected 'user1', got %v", results[0])
	}
	if results[1] != nil {
		t.Errorf("expected nil error, got %v", results[1])
	}
}

func TestExpectation_AnyMatcher(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", Any()).Return("any_user", nil)

	results := mc.Call("GetUser", 123)

	if len(results) != 2 {
		t.Fatalf("expected 2 return values, got %d", len(results))
	}
	if results[0] != "any_user" {
		t.Errorf("expected 'any_user', got %v", results[0])
	}
}

func TestExpectation_CustomMatcher(t *testing.T) {
	mc := NewMockController()
	isPositive := Matches(func(v interface{}) bool {
		n, ok := v.(int)
		return ok && n > 0
	})
	mc.On("GetUser", isPositive).Return("positive_user", nil)

	results := mc.Call("GetUser", 10)
	if results[0] != "positive_user" {
		t.Errorf("expected 'positive_user', got %v", results[0])
	}

	results = mc.Call("GetUser", -5)
	if len(results) != 0 {
		t.Errorf("expected no results for unmatched call, got %v", results)
	}
}

func TestExpectation_MultipleArgs(t *testing.T) {
	mc := NewMockController()
	mc.On("Add", 2, 3).Return(5)

	results := mc.Call("Add", 2, 3)
	if len(results) != 1 {
		t.Fatalf("expected 1 return value, got %d", len(results))
	}
	if results[0] != 5 {
		t.Errorf("expected 5, got %v", results[0])
	}
}

func TestReturn_FixedValues(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("Hello, World!")

	results := mc.Call("Greet", "World")
	if results[0] != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %v", results[0])
	}
}

func TestReturn_CallbackFunction(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", Any()).Run(func(name string) string {
		return "Hello, " + name + "!"
	})

	results := mc.Call("Greet", "Alice")
	if results[0] != "Hello, Alice!" {
		t.Errorf("expected 'Hello, Alice!', got %v", results[0])
	}
}

func TestReturn_MultipleValuesFromCallback(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", Any()).Run(func(id int) (string, error) {
		if id > 0 {
			return fmt.Sprintf("user%d", id), nil
		}
		return "", errors.New("invalid id")
	})

	results := mc.Call("GetUser", 1)
	if results[0] != "user1" {
		t.Errorf("expected 'user1', got %v", results[0])
	}
	if results[1] != nil {
		t.Errorf("expected nil error, got %v", results[1])
	}

	results = mc.Call("GetUser", -1)
	if results[0] != "" {
		t.Errorf("expected empty string, got %v", results[0])
	}
	if results[1] == nil {
		t.Error("expected error, got nil")
	}
}

func TestTimes_Once(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Once()

	mc.Call("Greet", "World")

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTimes_Once_CalledTooManyTimes(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Once()

	mc.Call("Greet", "World")
	mc.Call("Greet", "World")

	err := mc.Verify()
	if err == nil {
		t.Error("expected error for too many calls")
	}
	if !errors.Is(err, ErrCallCountMismatch) {
		t.Errorf("expected ErrCallCountMismatch, got %v", err)
	}
}

func TestTimes_Never(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Never()

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTimes_Never_Called(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Never()

	mc.Call("Greet", "World")

	err := mc.Verify()
	if err == nil {
		t.Error("expected error for called but should never called")
	}
}

func TestTimes_ExactCount(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Times(3)

	for i := 0; i < 3; i++ {
		mc.Call("Greet", "World")
	}

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMinTimes(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").MinTimes(2)

	mc.Call("Greet", "World")

	err := mc.Verify()
	if err == nil {
		t.Error("expected error for not enough calls")
	}

	mc.Call("Greet", "World")
	mc.Call("Greet", "World")

	err = mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMaxTimes(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").MaxTimes(2)

	mc.Call("Greet", "World")
	mc.Call("Greet", "World")

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	mc.Call("Greet", "World")

	err = mc.Verify()
	if err == nil {
		t.Error("expected error for too many calls")
	}
}

func TestUnmatchedCall(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	mc.Call("GetUser", 999)

	unmatched := mc.Mock().UnmatchedCalls()
	if len(unmatched) != 1 {
		t.Fatalf("expected 1 unmatched call, got %d", len(unmatched))
	}
	if unmatched[0].MethodName != "GetUser" {
		t.Errorf("expected method 'GetUser', got %s", unmatched[0].MethodName)
	}
}

func TestVerify_UnmatchedCalls(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	mc.Call("GetUser", 999)

	err := mc.Verify()
	if err == nil {
		t.Error("expected error for unmatched calls")
	}
	if !errors.Is(err, ErrNoMatchingExpectation) {
		t.Errorf("expected ErrNoMatchingExpectation, got %v", err)
	}
}

func TestVerify_UnmatchedCalls_ContainsMethodNameAndArgs(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	mc.Call("GetUser", 999)

	err := mc.Verify()
	if err == nil {
		t.Fatal("expected error for unmatched calls")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "GetUser") {
		t.Errorf("expected error to contain method name 'GetUser', got: %s", errStr)
	}
	if !strings.Contains(errStr, "999") {
		t.Errorf("expected error to contain arg value '999', got: %s", errStr)
	}
}

func TestVerify_MultipleUnmatchedCalls(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	mc.Call("GetUser", 999)
	mc.Call("Greet", "World")

	err := mc.Verify()
	if err == nil {
		t.Fatal("expected error for unmatched calls")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "GetUser") {
		t.Errorf("expected error to contain 'GetUser', got: %s", errStr)
	}
	if !strings.Contains(errStr, "Greet") {
		t.Errorf("expected error to contain 'Greet', got: %s", errStr)
	}
}

func TestVerifyVerbose_UnmatchedCalls(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)

	mc.Call("GetUser", 999)

	report := mc.VerifyVerbose()
	if report == "" {
		t.Error("expected non-empty verbose report")
	}
	if !strings.Contains(report, "Unmatched calls") {
		t.Errorf("expected report to contain 'Unmatched calls', got:\n%s", report)
	}
	if !strings.Contains(report, "GetUser") {
		t.Errorf("expected report to contain method name, got:\n%s", report)
	}
}

func TestVerifyVerbose_CallCountMismatch(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Times(2)

	mc.Call("Greet", "World")

	report := mc.VerifyVerbose()
	if !strings.Contains(report, "Call count mismatch") {
		t.Errorf("expected report to contain 'Call count mismatch', got:\n%s", report)
	}
}

func TestNoExpectation_ReturnsZeroValues(t *testing.T) {
	mc := NewMockController()

	results := mc.Call("Greet", "World")
	if len(results) != 0 {
		t.Errorf("expected 0 results for unmatched call, got %v", results)
	}
}

func TestMultipleExpectations_SameMethod_DifferentArgs(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", 1).Return("user1", nil)
	mc.On("GetUser", 2).Return("user2", nil)

	results := mc.Call("GetUser", 1)
	if results[0] != "user1" {
		t.Errorf("expected 'user1', got %v", results[0])
	}

	results = mc.Call("GetUser", 2)
	if results[0] != "user2" {
		t.Errorf("expected 'user2', got %v", results[0])
	}
}

func TestMultipleExpectations_FirstMatchWins(t *testing.T) {
	mc := NewMockController()
	mc.On("GetUser", Any()).Return("any_user", nil)
	mc.On("GetUser", 1).Return("specific_user", nil)

	results := mc.Call("GetUser", 1)
	if results[0] != "any_user" {
		t.Errorf("expected first matching expectation should win, got %v", results[0])
	}
}

func TestCallCount(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello")

	for i := 0; i < 5; i++ {
		mc.Call("Greet", "World")
	}

	expectations := mc.Mock().Expectations()
	exps := expectations["Greet"]
	if len(exps) != 1 {
		t.Fatalf("expected 1 expectation, got %d", len(exps))
	}
	if exps[0].CallCount() != 5 {
		t.Errorf("expected 5 calls, got %d", exps[0].CallCount())
	}
}

func TestVoidMethod(t *testing.T) {
	mc := NewMockController()
	mc.On("VoidMethod").Return().Once()

	results := mc.Call("VoidMethod")
	if len(results) != 0 {
		t.Errorf("expected 0 return values, got %d", len(results))
	}

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMultiReturn(t *testing.T) {
	mc := NewMockController()
	mc.On("MultiReturn").Return("hello", 42, true)

	results := mc.Call("MultiReturn")
	if len(results) != 3 {
		t.Fatalf("expected 3 return values, got %d", len(results))
	}
	if results[0] != "hello" {
		t.Errorf("expected 'hello', got %v", results[0])
	}
	if results[1] != 42 {
		t.Errorf("expected 42, got %v", results[1])
	}
	if results[2] != true {
		t.Errorf("expected true, got %v", results[2])
	}
}

func TestCreateMock(t *testing.T) {
	var svc TestService
	_ = svc
	mock, err := CreateMock((*TestService)(nil))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mock == nil {
		t.Fatal("expected non-nil MockProxy")
	}
	if mock.Controller() == nil {
		t.Fatal("expected non-nil controller")
	}
}

func TestCreateMock_NonInterface_ReturnsError(t *testing.T) {
	mock, err := CreateMock("not an interface")

	if err == nil {
		t.Error("expected error for non-interface type")
	}
	if mock != nil {
		t.Error("expected nil mock for non-interface type")
	}
	if !errors.Is(err, ErrInvalidInterface) {
		t.Errorf("expected ErrInvalidInterface, got %v", err)
	}
}

func TestMustCreateMock_NonInterface_Panics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-interface type")
		}
	}()
	MustCreateMock("not an interface")
}

func TestMustCreateMock_Success(t *testing.T) {
	mock := MustCreateMock((*TestService)(nil))
	if mock == nil {
		t.Fatal("expected non-nil MockProxy")
	}
}

func TestMockProxy_On(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.On("GetUser", 1).Return("user1", nil)

	results := mock.Call("GetUser", 1)
	if results[0] != "user1" {
		t.Errorf("expected 'user1', got %v", results[0])
	}
}

func TestMockProxy_Verify(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.On("Greet", "World").Return("hello").Once()

	mock.Call("Greet", "World")

	err = mock.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMockProxy_VerifyVerbose(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mock.Call("UnknownMethod", 123)

	report := mock.VerifyVerbose()
	if report == "" {
		t.Error("expected non-empty verbose report")
	}
}

func TestMockProxy_Method(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.On("Greet", "World").Return("Hello, World!")

	greetFunc := mock.Method("Greet")
	if greetFunc == nil {
		t.Fatal("expected non-nil method function")
	}

	fn, ok := greetFunc.(func(string) string)
	if !ok {
		t.Fatalf("expected func(string) string, got %T", greetFunc)
	}

	result := fn("World")
	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got %s", result)
	}
}

func TestMockProxy_TryMethod_Success(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fn, err := mock.TryMethod("Greet")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if fn == nil {
		t.Error("expected non-nil method function")
	}
}

func TestMockProxy_TryMethod_NotFound(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	fn, err := mock.TryMethod("NonExistentMethod")
	if err == nil {
		t.Error("expected error for non-existent method")
	}
	if fn != nil {
		t.Error("expected nil function for non-existent method")
	}
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound, got %v", err)
	}
}

func TestMockProxy_MethodNotFound(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for unknown method")
		}
	}()

	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.Method("NonExistentMethod")
}

func TestMockProxy_MethodWithMultipleReturns(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.On("GetUser", 1).Return("user1", nil)

	getUserFunc := mock.Method("GetUser")
	fn, ok := getUserFunc.(func(int) (string, error))
	if !ok {
		t.Fatalf("expected func(int) (string, error), got %T", getUserFunc)
	}

	name, err := fn(1)
	if name != "user1" {
		t.Errorf("expected 'user1', got %s", name)
	}
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestMockProxy_MethodZeroValues(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	getUserFunc := mock.Method("GetUser")
	fn, ok := getUserFunc.(func(int) (string, error))
	if !ok {
		t.Fatalf("expected func(int) (string, error), got %T", getUserFunc)
	}

	name, err := fn(999)
	if name != "" {
		t.Errorf("expected empty string (zero value), got %s", name)
	}
	if err != nil {
		t.Errorf("expected nil error (zero value), got %v", err)
	}
}

func TestMockProxy_Instance(t *testing.T) {
	mock, err := CreateMock((*TestService)(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.On("Greet", "World").Return("Hello, World!")

	instance := mock.Instance()
	if instance == nil {
		t.Fatal("expected non-nil instance")
	}
}

func TestConcurrentCalls(t *testing.T) {
	mc := NewMockController()
	mc.On("Add", Any(), Any()).Return(0).MinTimes(10)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(a, b int) {
			defer wg.Done()
			mc.Call("Add", a, b)
		}(i, i+1)
	}
	wg.Wait()

	expectations := mc.Mock().Expectations()
	exps := expectations["Add"]
	if len(exps) != 1 {
		t.Fatalf("expected 1 expectation, got %d", len(exps))
	}

	if exps[0].CallCount() < 10 {
		t.Errorf("expected at least 10 calls, got %d", exps[0].CallCount())
	}
}

func TestConcurrent_VerifyAndCalls(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", Any()).Return("hello").MinTimes(10).MaxTimes(100)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(a int) {
			defer wg.Done()
			mc.Call("Greet", fmt.Sprintf("name%d", a))
		}(i)
		go func() {
			defer wg.Done()
			_ = mc.Mock().Expectations()
			_ = mc.Mock().UnmatchedCalls()
		}()
	}
	wg.Wait()

	err := mc.Verify()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestConcurrent_VerifyVerbose_NoRace(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", "World").Return("hello").Once()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			mc.Call("Greet", "World")
		}()
		go func() {
			defer wg.Done()
			_ = mc.VerifyVerbose()
		}()
	}
	wg.Wait()
}

func TestConcurrent_CallCountAccess_NoRace(t *testing.T) {
	mc := NewMockController()
	mc.On("Add", Any(), Any()).Return(0)

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(2)
		go func(a, b int) {
			defer wg.Done()
			mc.Call("Add", a, b)
		}(i, i+1)
		go func() {
			defer wg.Done()
			expectations := mc.Mock().Expectations()
			for _, exps := range expectations {
				for _, exp := range exps {
					_ = exp.CallCount()
				}
			}
		}()
	}
	wg.Wait()
}

func TestMatcher_Any(t *testing.T) {
	matcher := Any()
	if !matcher(nil) {
		t.Error("Any() should match nil")
	}
	if !matcher(42) {
		t.Error("Any() should match int")
	}
	if !matcher("hello") {
		t.Error("Any() should match string")
	}
}

func TestMatcher_AnyOf(t *testing.T) {
	matcher := AnyOf(42)
	if !matcher(42) {
		t.Error("AnyOf(42) should match 42")
	}
	if matcher(43) {
		t.Error("AnyOf(42) should not match 43")
	}
}

func TestMatcher_Matches(t *testing.T) {
	isEven := Matches(func(v interface{}) bool {
		n, ok := v.(int)
		return ok && n%2 == 0
	})
	if !isEven(4) {
		t.Error("isEven should match 4")
	}
	if isEven(3) {
		t.Error("isEven should not match 3")
	}
}

func TestExpectationBuilder_Chaining(t *testing.T) {
	mc := NewMockController()
	builder := mc.On("Test", "arg").
		Return("result").
		Times(2)

	if builder == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestNewMock(t *testing.T) {
	m := NewMock()
	if m == nil {
		t.Fatal("expected non-nil Mock")
	}
	if m.Expectations() == nil {
		t.Error("expected non-nil expectations map")
	}
	if len(m.UnmatchedCalls()) != 0 {
		t.Error("expected 0 unmatched calls initially")
	}
}

func TestNewExpectation(t *testing.T) {
	exp := NewExpectation("TestMethod")
	if exp.methodName != "TestMethod" {
		t.Errorf("expected method name 'TestMethod', got %s", exp.methodName)
	}
	if exp.CallCount() != 0 {
		t.Errorf("expected 0 call count initially, got %d", exp.CallCount())
	}
}

func TestExpectation_Matches_WrongArgCount(t *testing.T) {
	exp := NewExpectation("TestMethod")
	exp.argMatchers = append(exp.argMatchers, Any())

	if exp.Matches([]interface{}{1, 2}) {
		t.Error("should not match with wrong argument count")
	}
}

func TestMock_AddExpectation(t *testing.T) {
	m := NewMock()
	exp := NewExpectation("Test")
	m.AddExpectation("Test", exp)

	expectations := m.Expectations()
	if len(expectations["Test"]) != 1 {
		t.Errorf("expected 1 expectation for 'Test', got %d", len(expectations["Test"]))
	}
}

func TestMock_FindMatchingExpectation_NoExpectations(t *testing.T) {
	m := NewMock()
	_, found := m.FindMatchingExpectation("Test", []interface{}{})
	if found {
		t.Error("should not find matching expectation when none exist")
	}
}

func TestMock_RecordUnmatchedCall(t *testing.T) {
	m := NewMock()
	m.RecordUnmatchedCall("Test", []interface{}{1, 2})

	unmatched := m.UnmatchedCalls()
	if len(unmatched) != 1 {
		t.Fatalf("expected 1 unmatched call, got %d", len(unmatched))
	}
	if unmatched[0].MethodName != "Test" {
		t.Errorf("expected method 'Test', got %s", unmatched[0].MethodName)
	}
	if len(unmatched[0].Args) != 2 {
		t.Errorf("expected 2 args, got %d", len(unmatched[0].Args))
	}
}

func TestMock_Verify_NoExpectations(t *testing.T) {
	m := NewMock()
	err := m.Verify()
	if err != nil {
		t.Errorf("expected no error with no expectations, got %v", err)
	}
}

func TestMock_Verbose_NoIssues(t *testing.T) {
	m := NewMock()
	report := m.VerifyVerbose()
	if report != "" {
		t.Errorf("expected empty report with no issues, got %q", report)
	}
}

func TestRun_PanicOnNonFunction(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for non-function return value")
		}
	}()

	mc := NewMockController()
	mc.On("Test").Run("not a function")
	mc.Call("Test")
}

func TestCallReturnFunc_NilArgs(t *testing.T) {
	mc := NewMockController()
	mc.On("Greet", Any()).Run(func(name string) string {
		if name == "" {
			return "empty"
		}
		return name
	})

	results := mc.Call("Greet", "")
	if results[0] != "empty" {
		t.Errorf("expected 'empty', got %v", results[0])
	}
}

func TestErrInvalidInterface_Used(t *testing.T) {
	_, err := CreateMock("invalid")
	if !errors.Is(err, ErrInvalidInterface) {
		t.Errorf("expected ErrInvalidInterface to be used, got %v", err)
	}
}

func TestErrMethodNotFound_Used(t *testing.T) {
	mock, _ := CreateMock((*TestService)(nil))
	_, err := mock.TryMethod("NonExistent")
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("expected ErrMethodNotFound to be used, got %v", err)
	}
}

func TestVerify_ErrorContainsArgsInDetail(t *testing.T) {
	mc := NewMockController()
	mc.On("Add", 1, 2).Return(3)

	mc.Call("Add", 10, 20)

	err := mc.Verify()
	if err == nil {
		t.Fatal("expected error")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, "[10 20]") && !strings.Contains(errStr, "10") && !strings.Contains(errStr, "20") {
		t.Errorf("expected error to contain args [10 20], got: %s", errStr)
	}
}
