package gqlparser

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

const levelBatchWindow = 2 * time.Millisecond

type Executor struct {
	Schema    *Schema
	Validator *Validator
}

func NewExecutor(schema *Schema) *Executor {
	return &Executor{
		Schema:    schema,
		Validator: NewValidator(),
	}
}

func NewExecutorWithValidator(schema *Schema, validator *Validator) *Executor {
	return &Executor{
		Schema:    schema,
		Validator: validator,
	}
}

func (e *Executor) Execute(query string, variables map[string]interface{}, dataLoaders map[string]*DataLoader) *ExecutionResult {
	result := &ExecutionResult{
		Data:   make(map[string]interface{}),
		Errors: make([]error, 0),
	}

	doc, err := ParseQueryWithSchema(query, e.Schema)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}

	if e.Validator != nil {
		validationErrs := e.Validator.Validate(e.Schema, doc)
		if len(validationErrs) > 0 {
			for _, ve := range validationErrs {
				result.Errors = append(result.Errors, ve)
			}
			return result
		}
	}

	if variables == nil {
		variables = make(map[string]interface{})
	}

	for _, op := range doc.Operations {
		varErrs := e.validateVariables(op, variables)
		if len(varErrs) > 0 {
			result.Errors = append(result.Errors, varErrs...)
			return result
		}
	}

	ctx := &ExecutionContext{
		Schema:      e.Schema,
		DataLoaders: dataLoaders,
		Variables:   variables,
		MaxDepth:    e.Validator.MaxDepth,
	}

	for _, op := range doc.Operations {
		opResult, opErrs := e.executeOperation(ctx, op)
		if len(opErrs) > 0 {
			result.Errors = append(result.Errors, opErrs...)
		}
		for k, v := range opResult {
			result.Data[k] = v
		}
	}

	return result
}

func (e *Executor) validateVariables(op *Operation, variables map[string]interface{}) []error {
	var errs []error

	for _, vd := range op.VariableDefs {
		if vd.Type.IsNonNull() && vd.DefaultValue == nil {
			val, exists := variables[vd.Name]
			if !exists || val == nil {
				errs = append(errs, fmt.Errorf("variable $%s is required but not provided", vd.Name))
			}
		}
	}

	return errs
}

func (e *Executor) executeOperation(ctx *ExecutionContext, op *Operation) (map[string]interface{}, []error) {
	var rootType *Type
	switch op.Type {
	case OperationQuery:
		rootType = ctx.Schema.GetQueryType()
	case OperationMutation:
		rootType = ctx.Schema.GetMutationType()
	default:
		return nil, []error{ErrUnknownOperation}
	}

	if rootType == nil {
		return nil, []error{fmt.Errorf("no root type defined for operation")}
	}

	fields := make([]*FieldSelection, 0)
	aliases := make(map[string]string)
	for _, sel := range op.SelectionSet {
		if field, ok := (*sel).(*FieldSelection); ok {
			fields = append(fields, field)
			name := field.Name
			if field.Alias != "" {
				name = field.Alias
			}
			aliases[field.Name] = name
		}
	}

	type fieldResult struct {
		name  string
		value interface{}
		errs  []error
	}

	results := make(chan fieldResult, len(fields))
	var wg sync.WaitGroup

	e.runConcurrentlyWithFlush(ctx, &wg, func() {
		for _, field := range fields {
			wg.Add(1)
			go func(f *FieldSelection) {
				defer wg.Done()
				resolvedArgs := resolveArguments(f.Args, ctx.Variables, op.VariableDefs)
				value, fieldErrs := e.executeField(ctx, rootType, f, nil, resolvedArgs, "")
				name := f.Name
				if f.Alias != "" {
					name = f.Alias
				}
				results <- fieldResult{name: name, value: value, errs: fieldErrs}
			}(field)
		}
	})

	close(results)

	result := make(map[string]interface{})
	var errs []error

	for r := range results {
		if len(r.errs) > 0 {
			errs = append(errs, r.errs...)
		}
		result[r.name] = r.value
	}

	return result, errs
}

func (e *Executor) flushDataLoaders(ctx *ExecutionContext) {
	if ctx.DataLoaders == nil {
		return
	}
	for _, dl := range ctx.DataLoaders {
		dl.Flush()
	}
}

func (e *Executor) runConcurrentlyWithFlush(
	ctx *ExecutionContext,
	wg *sync.WaitGroup,
	spawnGoroutines func(),
) {
	type savedWindow struct {
		dl     *DataLoader
		window time.Duration
	}
	var saved []savedWindow
	if ctx.DataLoaders != nil {
		saved = make([]savedWindow, 0, len(ctx.DataLoaders))
		for _, dl := range ctx.DataLoaders {
			saved = append(saved, savedWindow{dl: dl, window: dl.batchWindow})
			dl.SetBatchWindow(levelBatchWindow)
			dl.ResetBatchWindow()
		}
	}

	spawnGoroutines()
	wg.Wait()

	for _, s := range saved {
		s.dl.SetBatchWindow(s.window)
	}

	e.flushDataLoaders(ctx)
}

func (e *Executor) executeField(
	ctx *ExecutionContext,
	parentType *Type,
	field *FieldSelection,
	parent interface{},
	args map[string]interface{},
	path string,
) (interface{}, []error) {
	var errs []error

	fieldPath := path
	if field.Alias != "" {
		fieldPath = joinPath(fieldPath, field.Alias)
	} else {
		fieldPath = joinPath(fieldPath, field.Name)
	}

	typeName := ""
	unwrappedParent := parentType.Unwrap()
	if unwrappedParent != nil {
		typeName = unwrappedParent.Name
	}

	resolver, hasResolver := ctx.Schema.GetResolver(typeName, field.Name)
	var value interface{}
	var resolverErr error

	if hasResolver {
		value, resolverErr = resolver(ctx, parent, args)
		if resolverErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", fieldPath, resolverErr))
			return nil, errs
		}
	} else if parent != nil {
		value = getFieldFromParent(parent, field.Name)
	}

	var fieldType *Type
	if unwrappedParent != nil && unwrappedParent.Name != "" {
		if actualParentType, ok := ctx.Schema.GetType(unwrappedParent.Name); ok {
			if schemaField, hasField := actualParentType.Fields[field.Name]; hasField {
				fieldType = schemaField.Type
			}
		}
	}

	if len(field.SelectionSet) > 0 && value != nil {
		return e.executeSelectionSetOnValue(ctx, fieldType, field.SelectionSet, value, fieldPath)
	}

	return value, errs
}

func (e *Executor) executeSelectionSetOnValue(
	ctx *ExecutionContext,
	fieldType *Type,
	selections []*Selection,
	value interface{},
	path string,
) (interface{}, []error) {
	if fieldType != nil && fieldType.IsList() {
		return e.executeList(ctx, fieldType, selections, value, path)
	}

	return e.executeObject(ctx, fieldType, selections, value, path)
}

func (e *Executor) executeList(
	ctx *ExecutionContext,
	listType *Type,
	selections []*Selection,
	value interface{},
	path string,
) ([]interface{}, []error) {
	var errs []error

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return nil, append(errs, fmt.Errorf("%s: expected list but got %T", path, value))
	}

	innerType := listType
	if innerType.IsNonNull() {
		innerType = innerType.OfType
	}
	if innerType != nil && innerType.Kind == TypeKindList {
		innerType = innerType.OfType
	}

	n := val.Len()
	if n == 0 {
		return []interface{}{}, nil
	}

	type itemResult struct {
		index int
		value map[string]interface{}
		errs  []error
	}

	results := make(chan itemResult, n)
	var wg sync.WaitGroup

	e.runConcurrentlyWithFlush(ctx, &wg, func() {
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				itemPath := fmt.Sprintf("%s[%d]", path, idx)
				item := val.Index(idx).Interface()
				itemResultMap, itemErrs := e.executeObject(ctx, innerType, selections, item, itemPath)
				results <- itemResult{index: idx, value: itemResultMap, errs: itemErrs}
			}(i)
		}
	})

	close(results)

	resultList := make([]interface{}, n)
	for r := range results {
		if len(r.errs) > 0 {
			errs = append(errs, r.errs...)
		}
		resultList[r.index] = r.value
	}

	return resultList, errs
}

func (e *Executor) executeObject(
	ctx *ExecutionContext,
	objType *Type,
	selections []*Selection,
	value interface{},
	path string,
) (map[string]interface{}, []error) {
	var errs []error
	result := make(map[string]interface{})

	var actualType *Type
	if objType != nil {
		unwrapped := objType.Unwrap()
		if unwrapped.Name != "" {
			if t, ok := ctx.Schema.GetType(unwrapped.Name); ok {
				actualType = t
			} else {
				actualType = unwrapped
			}
		} else {
			actualType = unwrapped
		}
	}

	fieldSelections := make([]*FieldSelection, 0)
	fragments := make([]*InlineFragment, 0)
	for _, sel := range selections {
		switch s := (*sel).(type) {
		case *FieldSelection:
			fieldSelections = append(fieldSelections, s)
		case *InlineFragment:
			fragments = append(fragments, s)
		}
	}

	if len(fieldSelections) > 0 {
		type fieldResult struct {
			name  string
			value interface{}
			errs  []error
		}

		results := make(chan fieldResult, len(fieldSelections))
		var wg sync.WaitGroup

		e.runConcurrentlyWithFlush(ctx, &wg, func() {
			for _, fs := range fieldSelections {
				wg.Add(1)
				go func(f *FieldSelection) {
					defer wg.Done()
					resolvedArgs := resolveArguments(f.Args, ctx.Variables, nil)
					fieldValue, fieldErrs := e.executeField(
						ctx,
						actualType,
						f,
						value,
						resolvedArgs,
						path,
					)
					name := f.Name
					if f.Alias != "" {
						name = f.Alias
					}
					results <- fieldResult{name: name, value: fieldValue, errs: fieldErrs}
				}(fs)
			}
		})

		close(results)

		for r := range results {
			if len(r.errs) > 0 {
				errs = append(errs, r.errs...)
			}
			result[r.name] = r.value
		}
	}

	for _, frag := range fragments {
		fragType, ok := ctx.Schema.GetType(frag.TypeCondition)
		if !ok {
			continue
		}
		fragResult, fragErrs := e.executeObject(ctx, fragType, frag.SelectionSet, value, path)
		if len(fragErrs) > 0 {
			errs = append(errs, fragErrs...)
		}
		for k, v := range fragResult {
			result[k] = v
		}
	}

	return result, errs
}

func resolveArguments(
	queryArgs map[string]interface{},
	variables map[string]interface{},
	varDefs []*VariableDefinition,
) map[string]interface{} {
	result := make(map[string]interface{})

	if queryArgs == nil {
		return result
	}

	defaults := make(map[string]interface{})
	for _, vd := range varDefs {
		if vd.DefaultValue != nil {
			defaults[vd.Name] = vd.DefaultValue
		}
	}

	for name, val := range queryArgs {
		if vr, ok := val.(VariableRef); ok {
			if v, ok := variables[vr.Name]; ok {
				result[name] = v
			} else if def, ok := defaults[vr.Name]; ok {
				result[name] = def
			}
		} else {
			result[name] = val
		}
	}

	return result
}

func getFieldFromParent(parent interface{}, fieldName string) interface{} {
	if parent == nil {
		return nil
	}

	if m, ok := parent.(map[string]interface{}); ok {
		if v, exists := m[fieldName]; exists {
			return v
		}
	}

	val := reflect.ValueOf(parent)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() == reflect.Map {
		key := reflect.ValueOf(fieldName)
		mapVal := val.MapIndex(key)
		if mapVal.IsValid() {
			return mapVal.Interface()
		}
	}

	if val.Kind() == reflect.Struct {
		structField := val.FieldByName(fieldName)
		if structField.IsValid() {
			return structField.Interface()
		}

		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == fieldName {
				return val.Field(i).Interface()
			}
			tag = f.Tag.Get("gql")
			if tag == fieldName {
				return val.Field(i).Interface()
			}
		}
	}

	return nil
}
