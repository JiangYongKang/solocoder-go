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

	variableTypes := make(map[string]*Type)
	for _, vd := range op.VariableDefs {
		variableTypes[vd.Name] = normalizeVariableType(schema, vd.Type)
	}

	fieldErrs := v.validateSelectionSet(schema, rootType, op.SelectionSet, variableTypes, 0)
	errs = append(errs, fieldErrs...)

	return errs
}

func normalizeVariableType(schema *Schema, t *Type) *Type {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case TypeKindNonNull:
		return &Type{
			Kind:   TypeKindNonNull,
			OfType: normalizeVariableType(schema, t.OfType),
		}
	case TypeKindList:
		return &Type{
			Kind:   TypeKindList,
			OfType: normalizeVariableType(schema, t.OfType),
		}
	default:
		if t.Name != "" {
			if realType, ok := schema.GetType(t.Name); ok {
				return &Type{
					Kind: realType.Kind,
					Name: t.Name,
				}
			}
		}
		return &Type{
			Kind: t.Kind,
			Name: t.Name,
		}
	}
}

func (v *Validator) validateSelectionSet(
	schema *Schema,
	parentType *Type,
	selections []*Selection,
	variableTypes map[string]*Type,
	depth int,
) []*ValidationError {
	var errs []*ValidationError

	if depth > v.MaxDepth {
		errs = append(errs, NewValidationError("", "query nested too deep (max depth %d)", v.MaxDepth))
		return errs
	}

	unwrappedParent := parentType.Unwrap()
	parentIsObject := unwrappedParent.Kind == TypeKindObject ||
		unwrappedParent.Kind == TypeKindQuery ||
		unwrappedParent.Kind == TypeKindMutation

	var actualParent *Type
	if parentIsObject && unwrappedParent.Name != "" {
		if t, ok := schema.GetType(unwrappedParent.Name); ok {
			actualParent = t
		} else {
			actualParent = unwrappedParent
		}
	} else {
		actualParent = unwrappedParent
	}

	if len(selections) == 0 {
		if parentIsObject && len(actualParent.Fields) > 0 {
			errs = append(errs, NewValidationError("", "selection set cannot be empty for object type"))
		}
		return errs
	}

	if !parentIsObject {
		errs = append(errs, NewValidationError("", "cannot select fields on scalar type %q", unwrappedParent.Name))
		return errs
	}

	for _, sel := range selections {
		switch s := (*sel).(type) {
		case *FieldSelection:
			fieldErrs := v.validateField(schema, actualParent, s, variableTypes, depth)
			errs = append(errs, fieldErrs...)
		case *InlineFragment:
			fragType, ok := schema.GetType(s.TypeCondition)
			if !ok {
				errs = append(errs, NewValidationError("", "inline fragment references unknown type %q", s.TypeCondition))
				continue
			}
			fragErrs := v.validateSelectionSet(schema, fragType, s.SelectionSet, variableTypes, depth+1)
			errs = append(errs, fragErrs...)
		case *FragmentSpread:
			errs = append(errs, NewValidationError("", "named fragments are not supported in this implementation"))
		}
	}

	return errs
}

func (v *Validator) validateField(
	schema *Schema,
	parentType *Type,
	field *FieldSelection,
	variableTypes map[string]*Type,
	depth int,
) []*ValidationError {
	var errs []*ValidationError

	fieldPath := field.Name
	if field.Alias != "" {
		fieldPath = field.Alias
	}

	unwrappedParent := parentType.Unwrap()
	schemaField, ok := unwrappedParent.Fields[field.Name]
	if !ok {
		if field.Name != "__typename" {
			errs = append(errs, NewValidationError(fieldPath, "field %q does not exist on type %q", field.Name, unwrappedParent.Name))
		}
		return errs
	}

	argErrs := v.validateArguments(schemaField, field, variableTypes, fieldPath)
	errs = append(errs, argErrs...)

	if len(field.SelectionSet) > 0 && schemaField.Type != nil {
		fieldType := schemaField.Type.Unwrap()
		var actualFieldType *Type
		if fieldType.Name != "" {
			if t, ok := schema.GetType(fieldType.Name); ok {
				actualFieldType = t
			} else {
				actualFieldType = fieldType
			}
		} else {
			actualFieldType = fieldType
		}
		nestedErrs := v.validateSelectionSet(schema, actualFieldType, field.SelectionSet, variableTypes, depth+1)
		errs = append(errs, nestedErrs...)
	}

	return errs
}

func (v *Validator) validateArguments(
	schemaField *Field,
	queryField *FieldSelection,
	variableTypes map[string]*Type,
	path string,
) []*ValidationError {
	var errs []*ValidationError

	for argName, argDef := range schemaField.Args {
		if argDef.Type != nil && argDef.Type.IsNonNull() && argDef.Default == nil {
			if queryField.Args == nil {
				errs = append(errs, NewValidationError(joinPath(path, argName), "required argument %q is missing", argName))
				continue
			}
			argVal, ok := queryField.Args[argName]
			if !ok {
				errs = append(errs, NewValidationError(joinPath(path, argName), "required argument %q is missing", argName))
				continue
			}
			if vr, ok := argVal.(VariableRef); ok {
				varType, varExists := variableTypes[vr.Name]
				if !varExists {
					errs = append(errs, NewValidationError(joinPath(path, argName), "variable $%s is not defined", vr.Name))
					continue
				}
				if varType.IsNonNull() {
					continue
				}
				errs = append(errs, NewValidationError(joinPath(path, argName), "required argument %q cannot be null (variable $%s is nullable)", argName, vr.Name))
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

		if !v.validateArgumentValue(argDef.Type, argVal, variableTypes) {
			errs = append(errs, NewValidationError(argPath, "invalid value type for argument %q", argName))
		}
	}

	return errs
}

func (v *Validator) validateArgumentValue(expectedType *Type, value interface{}, variableTypes map[string]*Type) bool {
	if expectedType == nil {
		return true
	}

	if vr, ok := value.(VariableRef); ok {
		varType, ok := variableTypes[vr.Name]
		if !ok {
			return false
		}
		return v.isVariableTypeCompatible(expectedType, varType)
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
			if !v.validateArgumentValue(t.OfType, item, variableTypes) {
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

func (v *Validator) isVariableTypeCompatible(expectedType *Type, varType *Type) bool {
	expected := expectedType
	variable := varType

	if variable.IsNonNull() && !expected.IsNonNull() {
		variable = variable.OfType
	}

	if expected.IsNonNull() {
		if !variable.IsNonNull() {
			return false
		}
		expected = expected.OfType
		variable = variable.OfType
	}

	if expected.Kind == TypeKindList && variable.Kind == TypeKindList {
		return v.isVariableTypeCompatible(expected.OfType, variable.OfType)
	}

	return expected.Name == variable.Name
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

func joinPath(base, name string) string {
	if base == "" {
		return name
	}
	return fmt.Sprintf("%s.%s", base, name)
}
