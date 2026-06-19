package gqlparser

import (
	"fmt"
	"reflect"
	"sync"
)

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

	doc, err := ParseQuery(query)
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

	result := make(map[string]interface{})
	var errs []error

	for _, sel := range op.SelectionSet {
		field, ok := (*sel).(*FieldSelection)
		if !ok {
			continue
		}

		fieldName := field.Name
		if field.Alias != "" {
			fieldName = field.Alias
		}

		resolvedArgs := resolveArguments(field.Args, ctx.Variables, op.VariableDefs)
		value, fieldErrs := e.executeField(ctx, rootType, field, nil, resolvedArgs, "")
		if len(fieldErrs) > 0 {
			errs = append(errs, fieldErrs...)
		}
		result[fieldName] = value
	}

	return result, errs
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
	if parentType != nil {
		unwrapped := parentType.Unwrap()
		if unwrapped != nil {
			typeName = unwrapped.Name
		}
	}

	resolver, hasResolver := ctx.Schema.GetResolver(typeName, field.Name)
	var value interface{}
	var resolverErr error

	if hasResolver {
		value, resolverErr = resolver(parent, args)
		if resolverErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", fieldPath, resolverErr))
			return nil, errs
		}
	} else if parent != nil {
		value = getFieldFromParent(parent, field.Name)
	}

	var fieldType *Type
	if parentType != nil {
		unwrappedParent := parentType.Unwrap()
		var actualParentType *Type
		if unwrappedParent != nil && unwrappedParent.Name != "" {
			if t, ok := ctx.Schema.GetType(unwrappedParent.Name); ok {
				actualParentType = t
			} else {
				actualParentType = unwrappedParent
			}
		} else {
			actualParentType = unwrappedParent
		}
		if actualParentType != nil {
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
	results := make([]interface{}, 0)

	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Slice && val.Kind() != reflect.Array {
		return results, append(errs, fmt.Errorf("%s: expected list but got %T", path, value))
	}

	innerType := listType
	if innerType.IsNonNull() {
		innerType = innerType.OfType
	}
	if innerType != nil && innerType.Kind == TypeKindList {
		innerType = innerType.OfType
	}

	for i := 0; i < val.Len(); i++ {
		itemPath := fmt.Sprintf("%s[%d]", path, i)
		item := val.Index(i).Interface()
		itemResult, itemErrs := e.executeObject(ctx, innerType, selections, item, itemPath)
		if len(itemErrs) > 0 {
			errs = append(errs, itemErrs...)
		}
		results = append(results, itemResult)
	}

	return results, errs
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
		actualType = objType.Unwrap()
	} else if value != nil {
		t, _ := ctx.Schema.GetType(fmt.Sprintf("%T", value))
		actualType = t
	}

	for _, sel := range selections {
		switch s := (*sel).(type) {
		case *FieldSelection:
			fieldName := s.Name
			if s.Alias != "" {
				fieldName = s.Alias
			}

			var fieldType *Type
			if actualType != nil {
				if f, ok := actualType.Fields[s.Name]; ok {
					fieldType = f.Type
				}
			}

			resolvedArgs := resolveArguments(s.Args, ctx.Variables, nil)
			fieldValue, fieldErrs := e.executeField(
				ctx,
				actualType,
				s,
				value,
				resolvedArgs,
				path,
			)
			if len(fieldErrs) > 0 {
				errs = append(errs, fieldErrs...)
			}
			result[fieldName] = fieldValue
			_ = fieldType

		case *InlineFragment:
			fragType, ok := ctx.Schema.GetType(s.TypeCondition)
			if !ok {
				continue
			}
			fragResult, fragErrs := e.executeObject(ctx, fragType, s.SelectionSet, value, path)
			if len(fragErrs) > 0 {
				errs = append(errs, fragErrs...)
			}
			for k, v := range fragResult {
				result[k] = v
			}
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

func (e *Executor) ExecuteWithDataLoaders(
	query string,
	variables map[string]interface{},
	dataLoaders map[string]*DataLoader,
) *ExecutionResult {
	result := e.Execute(query, variables, dataLoaders)

	var wg sync.WaitGroup
	for _, dl := range dataLoaders {
		wg.Add(1)
		go func(loader *DataLoader) {
			defer wg.Done()
			_ = loader.Flush()
		}(dl)
	}
	wg.Wait()

	return result
}
