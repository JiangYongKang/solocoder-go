package mockgen

import (
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"
)

type nameOff int32
type typeOff int32
type textOff int32

type abiType struct {
	Size_       uintptr
	PtrBytes    uintptr
	Hash        uint32
	TFlag       uint8
	Align_      uint8
	FieldAlign_ uint8
	Kind_       uint8
	Equal       func(unsafe.Pointer, unsafe.Pointer) bool
	GCData      *byte
	Str         nameOff
	PtrToThis   typeOff
}

type abiUncommonType struct {
	PkgPath nameOff
	Mcount  uint16
	Xcount  uint16
	Moff    uint32
	_       uint32
}

type abiMethod struct {
	Name nameOff
	Mtyp typeOff
	Ifn  textOff
	Tfn  textOff
}

type abiImethod struct {
	Name nameOff
	Typ  typeOff
}

type abiName struct {
	Bytes *byte
}

type abiInterfaceType struct {
	Type    abiType
	PkgPath abiName
	Methods []abiImethod
}

type abiItab struct {
	Inter *abiInterfaceType
	Type  *abiType
	Hash  uint32
	Fun   [1]uintptr
}

type eface struct {
	_type *abiType
	data  unsafe.Pointer
}

type iface struct {
	tab  *abiItab
	data unsafe.Pointer
}

//go:linkname getitab runtime.getitab
func getitab(inter *abiInterfaceType, typ *abiType, canfail bool) *abiItab

func itabHashFunc(inter *abiInterfaceType, typ *abiType) uintptr {
	return uintptr(inter.Type.Hash ^ typ.Hash)
}

type mockMethod struct {
	name string
	typ  reflect.Type
	fn   interface{}
}

type internalMockIface interface {
	InternalMockMarker()
}

type mockImpl struct {
	id      uint64
	mp      *MockProxy
	methods []mockMethod
}

func (*mockImpl) InternalMockMarker() {}

var (
	mockImplPtrType   = reflect.TypeOf((*mockImpl)(nil))
	internalIfaceType = reflect.TypeOf((*internalMockIface)(nil)).Elem()
	itabStore         []*abiItab
	itabStoreLock     sync.Mutex
	mockRegistry      sync.Map
	nextMockID  uint64 = 0
	itabEntriesBase  unsafe.Pointer
	itabEntriesLock  sync.Once
)

func registerMockImpl(impl *mockImpl) {
	id := atomic.AddUint64(&nextMockID, 1)
	impl.id = id
	mockRegistry.Store(id, impl)
}

func getMockImpl(id uint64) (*mockImpl, bool) {
	v, ok := mockRegistry.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*mockImpl), true
}

func getABITypeFromReflect(t reflect.Type) *abiType {
	e := *(*eface)(unsafe.Pointer(&t))
	return (*abiType)(e.data)
}

func getABIInterfaceTypeFromReflect(t reflect.Type) *abiInterfaceType {
	return (*abiInterfaceType)(unsafe.Pointer(getABITypeFromReflect(t)))
}

func buildItab(inter *abiInterfaceType, typ *abiType, funcPtrs []uintptr) *abiItab {
	numFuncs := len(funcPtrs)
	funOffset := unsafe.Offsetof(abiItab{}.Fun)
	itabSize := funOffset + uintptr(numFuncs)*unsafe.Sizeof(uintptr(0))

	mem := make([]byte, itabSize)
	tab := (*abiItab)(unsafe.Pointer(&mem[0]))

	tab.Inter = inter
	tab.Type = typ
	tab.Hash = typ.Hash

	funBase := unsafe.Pointer(&tab.Fun[0])
	for i, fn := range funcPtrs {
		*(*uintptr)(unsafe.Pointer(uintptr(funBase) + uintptr(i)*unsafe.Sizeof(uintptr(0)))) = fn
	}

	itabStoreLock.Lock()
	itabStore = append(itabStore, tab)
	itabStoreLock.Unlock()
	return tab
}

func getFunctionCodePtr(fn interface{}) uintptr {
	return reflect.ValueOf(fn).Pointer()
}

func makeTrampolineFunc(impl *mockImpl, methodIndex int, methodName string, methodType reflect.Type) interface{} {
	numIn := methodType.NumIn()
	numOut := methodType.NumOut()

	inTypes := make([]reflect.Type, 0, numIn+1)
	inTypes = append(inTypes, mockImplPtrType)
	for i := 0; i < numIn; i++ {
		inTypes = append(inTypes, methodType.In(i))
	}

	outTypes := make([]reflect.Type, numOut)
	for i := 0; i < numOut; i++ {
		outTypes[i] = methodType.Out(i)
	}

	fnType := reflect.FuncOf(inTypes, outTypes, methodType.IsVariadic())

	implID := impl.id
	mi := methodIndex
	mn := methodName
	mt := methodType
	no := numOut

	fn := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		implInst, ok := getMockImpl(implID)
		if !ok {
			out := make([]reflect.Value, no)
			for j := 0; j < no; j++ {
				out[j] = reflect.Zero(mt.Out(j))
			}
			return out
		}

		in := make([]interface{}, len(args)-1)
		for j := 1; j < len(args); j++ {
			in[j-1] = args[j].Interface()
		}

		results := implInst.mp.controller.CallMethod(mn, in)

		out := make([]reflect.Value, no)
		for j := 0; j < no; j++ {
			if j < len(results) && results[j] != nil {
				out[j] = reflect.ValueOf(results[j])
			} else {
				out[j] = reflect.Zero(mt.Out(j))
			}
		}
		return out
	})

	_ = mi
	return fn.Interface()
}

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

func findItabTable() unsafe.Pointer {
	inter := getABIInterfaceTypeFromReflect(internalIfaceType)
	typ := getABITypeFromReflect(mockImplPtrType)

	knownItab := getitab(inter, typ, false)
	if knownItab == nil {
		return nil
	}

	knownPtr := uintptr(unsafe.Pointer(knownItab))
	hash := itabHashFunc(inter, typ) & (512 - 1)

	startPtr := uintptr(unsafe.Pointer(&itabStore))
	searchRange := uintptr(0x1000000)

	for i := uintptr(0); i < searchRange; i += 4 {
		if startPtr+i >= 4096 {
			addr := startPtr + i
			val := *(*uintptr)(unsafe.Pointer(addr))
			if val == knownPtr {
				base := addr - hash*unsafe.Sizeof(uintptr(0))
				return unsafe.Pointer(base)
			}
		}
		if startPtr >= i+4096 {
			addr := startPtr - i
			val := *(*uintptr)(unsafe.Pointer(addr))
			if val == knownPtr {
				base := addr - hash*unsafe.Sizeof(uintptr(0))
				return unsafe.Pointer(base)
			}
		}
	}
	return nil
}

func addItabToRuntime(tab *abiItab) bool {
	itabEntriesLock.Do(func() {
		itabEntriesBase = findItabTable()
	})
	if itabEntriesBase == nil {
		return false
	}
	h := itabHashFunc(tab.Inter, tab.Type) & (512 - 1)
	for i := uintptr(1); ; i++ {
		idx := h & (512 - 1)
		p := (*uintptr)(unsafe.Pointer(uintptr(itabEntriesBase) + idx*unsafe.Sizeof(uintptr(0))))
		val := *p
		if val == 0 {
			atomic.StoreUintptr(p, uintptr(unsafe.Pointer(tab)))
			return true
		}
		if val == uintptr(unsafe.Pointer(tab)) {
			return true
		}
		h += i
		h &= (512 - 1)
	}
}

func (mp *MockProxy) Instance() interface{} {
	numMethods := mp.targetType.NumMethod()

	impl := &mockImpl{
		mp:      mp,
		methods: make([]mockMethod, numMethods),
	}
	registerMockImpl(impl)

	interInternal := getABIInterfaceTypeFromReflect(internalIfaceType)
	typMockPtr := getABITypeFromReflect(mockImplPtrType)
	_ = getitab(interInternal, typMockPtr, false)

	funcPtrs := make([]uintptr, numMethods)
	for i := 0; i < numMethods; i++ {
		method := mp.targetType.Method(i)
		fn := makeTrampolineFunc(impl, i, method.Name, method.Type)
		impl.methods[i] = mockMethod{
			name: method.Name,
			typ:  method.Type,
			fn:   fn,
		}
		funcPtrs[i] = getFunctionCodePtr(fn)
	}

	inter := getABIInterfaceTypeFromReflect(mp.targetType)
	typ := typMockPtr

	tab := buildItab(inter, typ, funcPtrs)

	addItabToRuntime(tab)

	var resultEface eface
	resultEface._type = typ
	resultEface.data = unsafe.Pointer(impl)

	return *(*interface{})(unsafe.Pointer(&resultEface))
}
