package gqlparser

import (
	"fmt"
)

type Validator struct {
	MaxDepth int
}

func NewValidator() *Validator {
	return &Validator{
		MaxDepth: 10,
	}
}

func NewValidatorWithMaxDepth(maxDepth int) *Validator {
	return &Validator{
		MaxDepth: maxDepth,
	}
}

func (v *Validator) Validate(schema *Schema, doc *Document) []*ValidationError {
	var errs []*ValidationError

	for _, op := range doc.Operations {
		opErrs := v.validateOperation(schema, op)
		errs = append(errs, opErrs...)
	}

	return errs
}

func (v *Validator) validateOperation(schema *Schema, op *Operation) []*ValidationError {
	var errs []*ValidationError

	var rootType *Type
	switch op.Type {
	case OperationQuery:
		rootType = schema.GetQueryType()
		if rootType == nil {
			errs = append(errs, NewValidationError("", "schema does not define a query type"))
			return errs
		}
	case OperationMutation:
		rootType = schema.GetMutationType()
		if rootType == nil {
			errs = append(errs, NewValidationError("", "schema does not define a mutation type"))
			return errs
		}
	default:
		errs = append(errs, NewValidationError("", "unknown operation type"))
		return errs
	}

	fieldErrs := v.validateSelectionSet(schema, rootType, op.SelectionSet, "", 0)
	errs = append(errs, fieldErrs...)

	return errs
}

func (v *Validator) validateSelectionSet(
	schema *Schema,
	parentType *Type,
	selections []*Selection,
	path string,
	depth int,
) []*ValidationError {
	var errs []*ValidationError

	if depth > v.MaxDepth {
		errs = append(errs, NewValidationError(path, "query nested too deep (max depth %d)", v.MaxDepth))
		return errs
	}

	if len(selections) == 0 {
		if parentType != nil {
			unwrapped := parentType.Unwrap()
			var actualType *Type
			if unwrapped.Name != "" {
				if t, ok := schema.GetType(unwrapped.Name); ok {
					actualType = t
				} else {
					actualType = unwrapped
				}
			} else {
				actualType = unwrapped
			}
			if actualType.Kind == TypeKindObject || actualType.Kind == TypeKindQuery || actualType.Kind == TypeKindMutation {
				if len(actualType.Fields) > 0 {
					errs = append(errs, NewValidationError(path, "selection set cannot be empty for object type"))
				}
			}
		}
		return errs
	}

	for _, sel := range selections {
		switch s := (*sel).(type) {
		case *FieldSelection:
			fieldErrs := v.validateField(schema, parentType, s, path, depth)
			errs = append(errs, fieldErrs...)
		case *InlineFragment:
			fragType, ok := schema.GetType(s.TypeCondition)
			if !ok {
				errs = append(errs, NewValidationError(path, "inline fragment references unknown type %q", s.TypeCondition))
				continue
			}
			fragErrs := v.validateSelectionSet(schema, fragType, s.SelectionSet, path, depth+1)
			errs = append(errs, fragErrs...)
		case *FragmentSpread:
			errs = append(errs, NewValidationError(path, "named fragments are not supported in this implementation"))
		}
	}

	return errs
}

func (v *Validator) validateField(
	schema *Schema,
	parentType *Type,
	field *FieldSelection,
	path string,
	depth int,
) []*ValidationError {
	var errs []*ValidationError

	fieldPath := path
	if field.Alias != "" {
		fieldPath = joinPath(fieldPath, field.Alias)
	} else {
		fieldPath = joinPath(fieldPath, field.Name)
	}

	if parentType == nil {
		errs = append(errs, NewValidationError(fieldPath, "cannot validate field on nil parent type"))
		return errs
	}

	unwrappedParent := parentType.Unwrap()
	actualParent := unwrappedParent
	if unwrappedParent.Name != "" {
		if t, ok := schema.GetType(unwrappedParent.Name); ok {
			actualParent = t
		}
	}

	if actualParent.Fields == nil || len(actualParent.Fields) == 0 {
		if field.Name != "__typename" {
			errs = append(errs, NewValidationError(fieldPath, "type %q does not have fields", actualParent.Name))
			return errs
		}
		return errs
	}

	schemaField, ok := actualParent.Fields[field.Name]
	if !ok {
		errs = append(errs, NewValidationError(fieldPath, "field %q does not exist on type %q", field.Name, actualParent.Name))
		return errs
	}

	argErrs := v.validateArguments(schema, schemaField, field, fieldPath)
	errs = append(errs, argErrs...)

	if len(field.SelectionSet) > 0 {
		fieldType := schemaField.Type
		if fieldType != nil {
			nestedErrs := v.validateSelectionSet(schema, fieldType, field.SelectionSet, fieldPath, depth+1)
			errs = append(errs, nestedErrs...)
		}
	}

	requiredErrs := v.validateRequiredFields(schema, schemaField, field, fieldPath)
	errs = append(errs, requiredErrs...)

	return errs
}

func (v *Validator) validateArguments(
	schema *Schema,
	schemaField *Field,
	queryField *FieldSelection,
	path string,
) []*ValidationError {
	var errs []*ValidationError

	for argName, argDef := range schemaField.Args {
		if argDef.Type != nil && argDef.Type.IsNonNull() && argDef.Default == nil {
			if queryField.Args == nil {
				errs = append(errs, NewValidationError(joinPath(path, argName), "required argument %q is missing", argName))
				continue
			}
			if _, ok := queryField.Args[argName]; !ok {
				errs = append(errs, NewValidationError(joinPath(path, argName), "required argument %q is missing", argName))
			}
		}
	}

	if queryField.Args == nil {
		return errs
	}

	for argName, argVal := range queryField.Args {
		argPath := joinPath(path, argName)
		argDef, ok := schemaField.Args[argName]
		if !ok {
			errs = append(errs, NewValidationError(argPath, "unknown argument %q", argName))
			continue
		}

		if !v.validateArgumentValue(argDef.Type, argVal) {
			errs = append(errs, NewValidationError(argPath, "invalid value type for argument %q", argName))
		}
	}

	return errs
}

func (v *Validator) validateArgumentValue(expectedType *Type, value interface{}) bool {
	if expectedType == nil {
		return true
	}

	if _, ok := value.(VariableRef); ok {
		return true
	}

	t := expectedType
	if t.IsNonNull() {
		if value == nil {
			return false
		}
		t = t.OfType
	}

	if value == nil {
		return true
	}

	switch t.Kind {
	case TypeKindList:
		list, ok := value.([]interface{})
		if !ok {
			return false
		}
		for _, item := range list {
			if !v.validateArgumentValue(t.OfType, item) {
				return false
			}
		}
		return true
	case TypeKindScalar, TypeKindObject:
		return v.validateScalarValue(t.Name, value)
	default:
		return true
	}
}

func (v *Validator) validateScalarValue(typeName string, value interface{}) bool {
	switch typeName {
	case "Int":
		_, ok := value.(int)
		return ok
	case "Float":
		switch value.(type) {
		case float64, int:
			return true
		default:
			return false
		}
	case "String", "ID":
		_, ok := value.(string)
		return ok
	case "Boolean":
		_, ok := value.(bool)
		return ok
	default:
		return true
	}
}

func (v *Validator) validateRequiredFields(
	schema *Schema,
	schemaField *Field,
	queryField *FieldSelection,
	path string,
) []*ValidationError {
	var errs []*ValidationError

	if schemaField.Type == nil {
		return errs
	}

	if len(queryField.SelectionSet) > 0 {
		unwrappedType := schemaField.Type.Unwrap()

		var actualType *Type
		if unwrappedType.Name != "" {
			if t, ok := schema.GetType(unwrappedType.Name); ok {
				actualType = t
			} else {
				actualType = unwrappedType
			}
		} else {
			actualType = unwrappedType
		}

		if actualType.Kind != TypeKindObject && actualType.Kind != TypeKindQuery && actualType.Kind != TypeKindMutation {
			errs = append(errs, NewValidationError(
				path,
				"field %q is a scalar type and cannot have sub-selections", queryField.Name,
			))
		}
	}

	return errs
}

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", base, name)
}
