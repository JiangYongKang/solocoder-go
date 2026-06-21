package mockgen

import (
	"fmt"
	"reflect"
)

type MockController struct {
	mock *Mock
}

func NewMockController() *MockController {
	return &MockController{
		mock: NewMock(),
	}
}

func (mc *MockController) Mock() *Mock {
	return mc.mock
}

func (mc *MockController) Verify() error {
	return mc.mock.Verify()
}

func (mc *MockController) VerifyVerbose() string {
	return mc.mock.VerifyVerbose()
}

type ExpectationBuilder struct {
	mock        *Mock
	methodName  string
	expectation *Expectation
}

func (mc *MockController) On(methodName string, args ...interface{}) *ExpectationBuilder {
	exp := NewExpectation(methodName)

	for _, arg := range args {
		if matcher, ok := arg.(Matcher); ok {
			exp.argMatchers = append(exp.argMatchers, matcher)
		} else {
			exp.argMatchers = append(exp.argMatchers, exactMatcher(arg))
		}
	}

	mc.mock.AddExpectation(methodName, exp)

	return &ExpectationBuilder{
		mock:        mc.mock,
		methodName:  methodName,
		expectation: exp,
	}
}

func (eb *ExpectationBuilder) Return(returnValues ...interface{}) *ExpectationBuilder {
	eb.expectation.returnValues = returnValues
	return eb
}

func (eb *ExpectationBuilder) Run(fn interface{}) *ExpectationBuilder {
	eb.expectation.returnFunc = fn
	return eb
}

func (eb *ExpectationBuilder) Times(n int) *ExpectationBuilder {
	eb.expectation.minCalls = n
	eb.expectation.maxCalls = n
	return eb
}

func (eb *ExpectationBuilder) MinTimes(n int) *ExpectationBuilder {
	eb.expectation.minCalls = n
	return eb
}

func (eb *ExpectationBuilder) MaxTimes(n int) *ExpectationBuilder {
	eb.expectation.maxCalls = n
	return eb
}

func (eb *ExpectationBuilder) Once() *ExpectationBuilder {
	return eb.Times(1)
}

func (eb *ExpectationBuilder) Never() *ExpectationBuilder {
	return eb.Times(0)
}

func Any() Matcher {
	return anyMatcher()
}

func AnyOf(expected interface{}) Matcher {
	return exactMatcher(expected)
}

func Matches(fn func(interface{}) bool) Matcher {
	return Matcher(fn)
}

func (mc *MockController) CallMethod(methodName string, args []interface{}) []interface{} {
	argsInterfaces := make([]interface{}, len(args))
	for i, arg := range args {
		argsInterfaces[i] = arg
	}

	exp, found := mc.mock.FindMatchingExpectation(methodName, argsInterfaces)
	if !found {
		mc.mock.RecordUnmatchedCall(methodName, argsInterfaces)
		return nil
	}

	exp.IncrementCallCount()

	if exp.returnFunc != nil {
		return callReturnFunc(exp.returnFunc, argsInterfaces)
	}

	if len(exp.returnValues) > 0 {
		return exp.returnValues
	}

	return nil
}

func callReturnFunc(fn interface{}, args []interface{}) []interface{} {
	fnValue := reflect.ValueOf(fn)
	fnType := fnValue.Type()

	if fnType.Kind() != reflect.Func {
		panic("return function is not a function")
	}

	in := make([]reflect.Value, len(args))
	for i, arg := range args {
		if arg == nil {
			in[i] = reflect.Zero(fnType.In(i))
		} else {
			in[i] = reflect.ValueOf(arg)
		}
	}

	results := fnValue.Call(in)

	out := make([]interface{}, len(results))
	for i, result := range results {
		out[i] = result.Interface()
	}

	return out
}

func (mc *MockController) Call(methodName string, args ...interface{}) []interface{} {
	return mc.CallMethod(methodName, args)
}

type MockProxy struct {
	controller *MockController
	targetType reflect.Type
}

func CreateMock(iface interface{}) (*MockProxy, error) {
	ifaceType := reflect.TypeOf(iface)
	if ifaceType.Kind() == reflect.Ptr {
		ifaceType = ifaceType.Elem()
	}
	if ifaceType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("%w: expected interface type, got %s",
			ErrInvalidInterface, ifaceType.Kind())
	}

	controller := NewMockController()

	return &MockProxy{
		controller: controller,
		targetType: ifaceType,
	}, nil
}

func MustCreateMock(iface interface{}) *MockProxy {
	mp, err := CreateMock(iface)
	if err != nil {
		panic(err)
	}
	return mp
}

func (mp *MockProxy) Controller() *MockController {
	return mp.controller
}

func (mp *MockProxy) On(methodName string, args ...interface{}) *ExpectationBuilder {
	return mp.controller.On(methodName, args...)
}

func (mp *MockProxy) Verify() error {
	return mp.controller.Verify()
}

func (mp *MockProxy) VerifyVerbose() string {
	return mp.controller.VerifyVerbose()
}

func (mp *MockProxy) Call(methodName string, args ...interface{}) []interface{} {
	return mp.controller.Call(methodName, args...)
}

func (mp *MockProxy) Method(methodName string) interface{} {
	fn, err := mp.TryMethod(methodName)
	if err != nil {
		panic(err)
	}
	return fn
}

func (mp *MockProxy) TryMethod(methodName string) (interface{}, error) {
	method, ok := mp.targetType.MethodByName(methodName)
	if !ok {
		return nil, fmt.Errorf("%w: method %q not found on interface %s",
			ErrMethodNotFound, methodName, mp.targetType.Name())
	}

	methodType := method.Type

	fn := reflect.MakeFunc(methodType, func(args []reflect.Value) []reflect.Value {
		in := make([]interface{}, len(args))
		for i, arg := range args {
			in[i] = arg.Interface()
		}

		results := mp.controller.CallMethod(methodName, in)

		out := make([]reflect.Value, methodType.NumOut())
		for i := 0; i < methodType.NumOut(); i++ {
			if i < len(results) && results[i] != nil {
				out[i] = reflect.ValueOf(results[i])
			} else {
				out[i] = reflect.Zero(methodType.Out(i))
			}
		}

		return out
	})

	return fn.Interface(), nil
}

func (mp *MockProxy) Instance() interface{} {
	numMethods := mp.targetType.NumMethod()
	fields := make([]reflect.StructField, numMethods)

	for i := 0; i < numMethods; i++ {
		method := mp.targetType.Method(i)
		fn, err := mp.TryMethod(method.Name)
		if err != nil {
			panic(err)
		}
		fields[i] = reflect.StructField{
			Name: method.Name,
			Type: reflect.TypeOf(fn),
		}
	}

	structType := reflect.StructOf(fields)
	structValue := reflect.New(structType).Elem()

	for i := 0; i < numMethods; i++ {
		method := mp.targetType.Method(i)
		fn, err := mp.TryMethod(method.Name)
		if err != nil {
			panic(err)
		}
		structValue.Field(i).Set(reflect.ValueOf(fn))
	}

	return structValue.Interface()
}

func As[T any](mp *MockProxy) T {
	var zero T
	targetType := reflect.TypeOf(&zero).Elem()

	if targetType.Kind() != reflect.Struct {
		return zero
	}

	instance := mp.Instance()
	instanceValue := reflect.ValueOf(instance)
	targetValue := reflect.New(targetType).Elem()

	for i := 0; i < targetType.NumField(); i++ {
		field := targetType.Field(i)
		instanceField := instanceValue.FieldByName(field.Name)
		if !instanceField.IsValid() {
			continue
		}
		if instanceField.Type().AssignableTo(field.Type) {
			targetValue.Field(i).Set(instanceField)
		}
	}

	return targetValue.Interface().(T)
}
