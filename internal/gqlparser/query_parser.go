package gqlparser

import (
	"fmt"
	"strings"
	"unicode"
)

type queryParser struct {
	input string
	pos   int
}

func ParseQuery(query string) (*Document, error) {
	p := &queryParser{input: query, pos: 0}
	return p.parseDocument()
}

func (p *queryParser) parseDocument() (*Document, error) {
	doc := &Document{}
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		op, err := p.parseOperation()
		if err != nil {
			return nil, err
		}
		doc.Operations = append(doc.Operations, op)
		p.skipWhitespaceAndComments()
	}

	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("%w: no operations found", ErrInvalidQuery)
	}

	return doc, nil
}

func (p *queryParser) parseOperation() (*Operation, error) {
	p.skipWhitespaceAndComments()

	op := &Operation{}

	if p.matchKeyword("query") {
		op.Type = OperationQuery
		p.skipWhitespaceAndComments()
		if isIdentStart(p.peek()) {
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			op.Name = name
		}
	} else if p.matchKeyword("mutation") {
		op.Type = OperationMutation
		p.skipWhitespaceAndComments()
		if isIdentStart(p.peek()) {
			name, err := p.parseName()
			if err != nil {
				return nil, err
			}
			op.Name = name
		}
	} else if p.peek() == '{' {
		op.Type = OperationQuery
	} else {
		return nil, fmt.Errorf("%w: expected operation type or '{' at position %d", ErrInvalidQuery, p.pos)
	}

	p.skipWhitespaceAndComments()
	if p.match("(") {
		varDefs, err := p.parseVariableDefinitions()
		if err != nil {
			return nil, err
		}
		op.VariableDefs = varDefs
		p.skipWhitespaceAndComments()
	}

	p.skipWhitespaceAndComments()
	if !p.match("{") {
		return nil, fmt.Errorf("%w: expected '{' for operation", ErrInvalidQuery)
	}

	selections, err := p.parseSelectionSet()
	if err != nil {
		return nil, err
	}
	if len(selections) == 0 {
		return nil, fmt.Errorf("%w: operation selection set cannot be empty", ErrInvalidQuery)
	}
	op.SelectionSet = selections

	return op, nil
}

func (p *queryParser) parseVariableDefinitions() ([]*VariableDefinition, error) {
	var defs []*VariableDefinition
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		if p.match(")") {
			return defs, nil
		}
		def, err := p.parseVariableDefinition()
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
		p.skipWhitespaceAndComments()
		p.match(",")
		p.skipWhitespaceAndComments()
	}

	return nil, fmt.Errorf("%w: unterminated variable definitions, expected ')'", ErrInvalidQuery)
}

func (p *queryParser) parseVariableDefinition() (*VariableDefinition, error) {
	p.skipWhitespaceAndComments()
	if !p.match("$") {
		return nil, fmt.Errorf("%w: expected '$' for variable", ErrInvalidQuery)
	}

	name, err := p.parseName()
	if err != nil {
		return nil, err
	}

	p.skipWhitespaceAndComments()
	if !p.match(":") {
		return nil, fmt.Errorf("%w: expected ':' after variable name", ErrInvalidQuery)
	}

	p.skipWhitespaceAndComments()
	varType, err := p.parseTypeReference()
	if err != nil {
		return nil, err
	}

	def := &VariableDefinition{
		Name: name,
		Type: varType,
	}

	p.skipWhitespaceAndComments()
	if p.match("=") {
		p.skipWhitespaceAndComments()
		defaultVal, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		def.DefaultValue = defaultVal
	}

	return def, nil
}

func (p *queryParser) parseSelectionSet() ([]*Selection, error) {
	var selections []*Selection
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		if p.match("}") {
			return selections, nil
		}
		sel, err := p.parseSelection()
		if err != nil {
			return nil, err
		}
		selections = append(selections, sel)
		p.skipWhitespaceAndComments()
	}

	return nil, fmt.Errorf("%w: unterminated selection set, expected '}'", ErrInvalidQuery)
}

func (p *queryParser) parseSelection() (*Selection, error) {
	p.skipWhitespaceAndComments()

	if p.match("...") {
		p.skipWhitespaceAndComments()
		if p.matchKeyword("on") {
			p.skipWhitespaceAndComments()
			typeCond, err := p.parseName()
			if err != nil {
				return nil, err
			}
			p.skipWhitespaceAndComments()
			if !p.match("{") {
				return nil, fmt.Errorf("%w: expected '{' for inline fragment", ErrInvalidQuery)
			}
			selSet, err := p.parseSelectionSet()
			if err != nil {
				return nil, err
			}
			sel := Selection(&InlineFragment{
				TypeCondition: typeCond,
				SelectionSet:  selSet,
			})
			return &sel, nil
		}
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		sel := Selection(&FragmentSpread{Name: name})
		return &sel, nil
	}

	field, err := p.parseFieldSelection()
	if err != nil {
		return nil, err
	}
	sel := Selection(field)
	return &sel, nil
}

func (p *queryParser) parseFieldSelection() (*FieldSelection, error) {
	field := &FieldSelection{}

	name, err := p.parseName()
	if err != nil {
		return nil, err
	}

	p.skipWhitespaceAndComments()
	if p.match(":") {
		field.Alias = name
		p.skipWhitespaceAndComments()
		actualName, err := p.parseName()
		if err != nil {
			return nil, err
		}
		field.Name = actualName
	} else {
		field.Name = name
	}

	p.skipWhitespaceAndComments()
	if p.match("(") {
		args, err := p.parseArguments()
		if err != nil {
			return nil, err
		}
		field.Args = args
	}

	p.skipWhitespaceAndComments()
	if p.match("{") {
		selSet, err := p.parseSelectionSet()
		if err != nil {
			return nil, err
		}
		field.SelectionSet = selSet
	}

	return field, nil
}

func (p *queryParser) parseArguments() (map[string]interface{}, error) {
	args := make(map[string]interface{})
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		if p.match(")") {
			return args, nil
		}
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		p.skipWhitespaceAndComments()
		if !p.match(":") {
			return nil, fmt.Errorf("%w: expected ':' in argument", ErrInvalidQuery)
		}
		p.skipWhitespaceAndComments()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		args[name] = val
		p.skipWhitespaceAndComments()
		p.match(",")
		p.skipWhitespaceAndComments()
	}

	return nil, fmt.Errorf("%w: unterminated arguments list, expected ')'", ErrInvalidQuery)
}

func (p *queryParser) parseValue() (interface{}, error) {
	p.skipWhitespaceAndComments()

	if p.match("$") {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		return VariableRef{Name: name}, nil
	}
	if p.match("\"") {
		return p.parseString()
	}
	if p.match("true") {
		return true, nil
	}
	if p.match("false") {
		return false, nil
	}
	if p.match("null") {
		return nil, nil
	}
	if p.match("[") {
		return p.parseListValue()
	}
	if p.match("{") {
		return p.parseObjectValue()
	}
	if p.peek() == '-' || (p.peek() >= '0' && p.peek() <= '9') {
		return p.parseNumber()
	}
	if isIdentStart(p.peek()) {
		return p.parseName()
	}

	return nil, fmt.Errorf("%w: unexpected token at position %d", ErrInvalidQuery, p.pos)
}

func (p *queryParser) parseListValue() ([]interface{}, error) {
	var list []interface{}
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		if p.match("]") {
			return list, nil
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		list = append(list, val)
		p.skipWhitespaceAndComments()
		p.match(",")
		p.skipWhitespaceAndComments()
	}

	return nil, fmt.Errorf("%w: unterminated list value, expected ']'", ErrInvalidQuery)
}

func (p *queryParser) parseObjectValue() (map[string]interface{}, error) {
	obj := make(map[string]interface{})
	p.skipWhitespaceAndComments()

	for p.pos < len(p.input) {
		if p.match("}") {
			return obj, nil
		}
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		p.skipWhitespaceAndComments()
		if !p.match(":") {
			return nil, fmt.Errorf("%w: expected ':' in object value", ErrInvalidQuery)
		}
		p.skipWhitespaceAndComments()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		obj[name] = val
		p.skipWhitespaceAndComments()
		p.match(",")
		p.skipWhitespaceAndComments()
	}

	return nil, fmt.Errorf("%w: unterminated object value, expected '}'", ErrInvalidQuery)
}

type VariableRef struct {
	Name string
}

func (p *queryParser) parseTypeReference() (*Type, error) {
	var t *Type

	if p.match("[") {
		p.skipWhitespaceAndComments()
		innerType, err := p.parseTypeReference()
		if err != nil {
			return nil, err
		}
		p.skipWhitespaceAndComments()
		if !p.match("]") {
			return nil, fmt.Errorf("%w: expected ']'", ErrInvalidQuery)
		}
		t = &Type{
			Kind:   TypeKindList,
			OfType: innerType,
		}
	} else {
		name, err := p.parseName()
		if err != nil {
			return nil, err
		}
		t = &Type{
			Kind: TypeKindObject,
			Name: name,
		}
	}

	p.skipWhitespaceAndComments()
	if p.match("!") {
		return &Type{
			Kind:   TypeKindNonNull,
			OfType: t,
		}, nil
	}

	return t, nil
}

func (p *queryParser) parseName() (string, error) {
	p.skipWhitespaceAndComments()

	if p.pos >= len(p.input) {
		return "", fmt.Errorf("%w: unexpected end of input", ErrInvalidQuery)
	}

	start := p.pos
	if !isIdentStart(p.peek()) {
		return "", fmt.Errorf("%w: expected identifier at position %d", ErrInvalidQuery, p.pos)
	}

	p.pos++
	for p.pos < len(p.input) && isIdentContinue(p.peek()) {
		p.pos++
	}

	return p.input[start:p.pos], nil
}

func (p *queryParser) parseString() (string, error) {
	var sb strings.Builder
	for p.pos < len(p.input) && p.peek() != '"' {
		if p.peek() == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("%w: unterminated string escape", ErrInvalidQuery)
			}
			switch p.peek() {
			case '"':
				sb.WriteByte('"')
			case '\\':
				sb.WriteByte('\\')
			case '/':
				sb.WriteByte('/')
			case 'n':
				sb.WriteByte('\n')
			case 'r':
				sb.WriteByte('\r')
			case 't':
				sb.WriteByte('\t')
			default:
				sb.WriteByte(p.peek())
			}
		} else {
			sb.WriteByte(p.peek())
		}
		p.pos++
	}
	if !p.match("\"") {
		return "", fmt.Errorf("%w: unterminated string", ErrInvalidQuery)
	}
	return sb.String(), nil
}

func (p *queryParser) parseNumber() (interface{}, error) {
	start := p.pos
	if p.peek() == '-' {
		p.pos++
	}

	isFloat := false
	for p.pos < len(p.input) {
		ch := p.peek()
		if ch >= '0' && ch <= '9' {
			p.pos++
		} else if ch == '.' {
			isFloat = true
			p.pos++
		} else if ch == 'e' || ch == 'E' {
			isFloat = true
			p.pos++
			if p.peek() == '+' || p.peek() == '-' {
				p.pos++
			}
		} else {
			break
		}
	}

	numStr := p.input[start:p.pos]
	if isFloat {
		var f float64
		fmt.Sscanf(numStr, "%f", &f)
		return f, nil
	}
	var i int64
	fmt.Sscanf(numStr, "%d", &i)
	return int(i), nil
}

func (p *queryParser) skipWhitespaceAndComments() {
	for p.pos < len(p.input) {
		ch := p.peek()
		if unicode.IsSpace(rune(ch)) {
			p.pos++
		} else if ch == '#' {
			for p.pos < len(p.input) && p.peek() != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *queryParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *queryParser) match(s string) bool {
	if p.pos+len(s) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(s)] == s {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *queryParser) matchKeyword(kw string) bool {
	saved := p.pos
	p.skipWhitespaceAndComments()

	if !p.match(kw) {
		p.pos = saved
		return false
	}

	if p.pos < len(p.input) && isIdentContinue(p.peek()) {
		p.pos = saved
		return false
	}
	return true
}
