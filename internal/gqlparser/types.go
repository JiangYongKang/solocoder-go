package gqlparser

import "sync"

type TypeKind int

const (
	TypeKindScalar TypeKind = iota
	TypeKindObject
	TypeKindList
	TypeKindNonNull
	TypeKindQuery
	TypeKindMutation
)

type Type struct {
	Kind       TypeKind
	Name       string
	OfType     *Type
	Fields     map[string]*Field
	IsBuiltin  bool
}

type Field struct {
	Name       string
	Type       *Type
	Args       map[string]*Argument
	NonNull    bool
}

type Argument struct {
	Name     string
	Type     *Type
	Default  interface{}
}

type ScalarValue struct {
	Type  string
	Value interface{}
}

type Schema struct {
	types      map[string]*Type
	queryType  *Type
	mutationType *Type
	resolvers  map[string]map[string]ResolverFunc
	mu         sync.RWMutex
}

type ResolverFunc func(parent interface{}, args map[string]interface{}) (interface{}, error)

type DataLoaderFunc func(keys []interface{}) ([]interface{}, error)

type DataLoader struct {
	fn      DataLoaderFunc
	pending []*loaderRequest
	mu      sync.Mutex
	flushed bool
}

type loaderRequest struct {
	key    interface{}
	result chan loaderResult
}

type loaderResult struct {
	value interface{}
	err   error
}

type OperationType int

const (
	OperationQuery OperationType = iota
	OperationMutation
)

type Document struct {
	Operations []*Operation
}

type Operation struct {
	Type          OperationType
	Name          string
	SelectionSet  []*Selection
	VariableDefs  []*VariableDefinition
}

type VariableDefinition struct {
	Name         string
	Type         *Type
	DefaultValue interface{}
}

type Selection interface {
	isSelection()
}

type FieldSelection struct {
	Alias        string
	Name         string
	Args         map[string]interface{}
	SelectionSet []*Selection
}

type FragmentSpread struct {
	Name string
}

type InlineFragment struct {
	TypeCondition string
	SelectionSet  []*Selection
}

func (f *FieldSelection) isSelection()    {}
func (f *FragmentSpread) isSelection()    {}
func (f *InlineFragment) isSelection()    {}

type ValidationError struct {
	Message string
	Path    string
}

func (e *ValidationError) Error() string {
	if e.Path != "" {
		return e.Path + ": " + e.Message
	}
	return e.Message
}

type ExecutionContext struct {
	Schema      *Schema
	DataLoaders map[string]*DataLoader
	Variables   map[string]interface{}
	MaxDepth    int
}

type ExecutionResult struct {
	Data   map[string]interface{}
	Errors []error
}
