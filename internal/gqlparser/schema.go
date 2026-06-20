package gqlparser

import (
	"fmt"
	"strings"
	"unicode"
)

var builtinScalars = []string{"Int", "Float", "String", "Boolean", "ID"}

func NewSchema() *Schema {
	s := &Schema{
		types:     make(map[string]*Type),
		resolvers: make(map[string]map[string]ResolverFunc),
	}
	s.registerBuiltinScalars()
	return s
}

func (s *Schema) registerBuiltinScalars() {
	for _, name := range builtinScalars {
		s.types[name] = &Type{
			Kind:      TypeKindScalar,
			Name:      name,
			IsBuiltin: true,
		}
	}
}

func (s *Schema) ParseSDL(sdl string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	parser := &sdlParser{input: sdl, pos: 0}
	return parser.parse(s)
}

func (s *Schema) GetType(name string) (*Type, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.types[name]
	return t, ok
}

func (s *Schema) GetAllTypes() map[string]*Type {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]*Type)
	for k, v := range s.types {
		result[k] = v
	}
	return result
}

func (s *Schema) RegisterType(t *Type) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.types[t.Name]; exists {
		return ErrTypeAlreadyExists
	}
	s.types[t.Name] = t
	return nil
}

func (s *Schema) SetQueryType(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.types[name]
	if !ok {
		return ErrTypeNotFound
	}
	t.Kind = TypeKindQuery
	s.queryType = t
	return nil
}

func (s *Schema) SetMutationType(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.types[name]
	if !ok {
		return ErrTypeNotFound
	}
	t.Kind = TypeKindMutation
	s.mutationType = t
	return nil
}

func (s *Schema) GetQueryType() *Type {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.queryType
}

func (s *Schema) GetMutationType() *Type {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mutationType
}

func (s *Schema) RegisterResolver(typeName, fieldName string, resolver ResolverFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.types[typeName]
	if !ok {
		return ErrTypeNotFound
	}
	if _, ok := t.Fields[fieldName]; !ok {
		return ErrFieldNotFound
	}

	if s.resolvers[typeName] == nil {
		s.resolvers[typeName] = make(map[string]ResolverFunc)
	}
	s.resolvers[typeName][fieldName] = resolver
	return nil
}

func (s *Schema) GetResolver(typeName, fieldName string) (ResolverFunc, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	typeResolvers, ok := s.resolvers[typeName]
	if !ok {
		return nil, false
	}
	resolver, ok := typeResolvers[fieldName]
	return resolver, ok
}

type sdlParser struct {
	input  string
	pos    int
	schema *Schema
}

func (p *sdlParser) parse(s *Schema) error {
	p.schema = s

	typeDefs := make([]string, 0)
	scalarDefs := make([]string, 0)

	{
		savedPos := p.pos
		for p.pos < len(p.input) {
			p.skipWhitespaceAndComments()
			if p.matchKeyword("type") {
				name, err := p.parseName()
				if err == nil {
					typeDefs = append(typeDefs, name)
				}
				p.skipUntilTypeEnd()
			} else if p.matchKeyword("scalar") {
				name, err := p.parseName()
				if err == nil {
					scalarDefs = append(scalarDefs, name)
				}
			} else if p.matchKeyword("schema") {
				p.skipUntilTypeEnd()
			} else {
				if p.pos < len(p.input) {
					return fmt.Errorf("%w: unexpected token at position %d", ErrInvalidSDL, p.pos)
				}
				break
			}
		}
		p.pos = savedPos
	}

	for _, name := range scalarDefs {
		if _, exists := s.types[name]; !exists {
			s.types[name] = &Type{
				Kind:      TypeKindScalar,
				Name:      name,
				IsBuiltin: false,
			}
		}
	}
	for _, name := range typeDefs {
		if _, exists := s.types[name]; !exists {
			s.types[name] = &Type{
				Kind:   TypeKindObject,
				Name:   name,
				Fields: make(map[string]*Field),
			}
		}
	}

	p.skipWhitespaceAndComments()
	for p.pos < len(p.input) {
		if err := p.parseDefinition(s); err != nil {
			return err
		}
		p.skipWhitespaceAndComments()
	}

	return nil
}

func (p *sdlParser) skipUntilTypeEnd() {
	depth := 0
	inBraces := false
	for p.pos < len(p.input) {
		ch := p.peek()
		if ch == '{' {
			depth++
			inBraces = true
			p.pos++
		} else if ch == '}' {
			depth--
			p.pos++
			if depth == 0 && inBraces {
				return
			}
		} else if ch == '\n' && !inBraces && depth == 0 {
			return
		} else {
			p.pos++
		}
	}
}

func (p *sdlParser) parseDefinition(s *Schema) error {
	p.skipWhitespaceAndComments()

	if p.matchKeyword("type") {
		return p.parseObjectType(s)
	}
	if p.matchKeyword("schema") {
		return p.parseSchemaDefinition(s)
	}
	if p.matchKeyword("scalar") {
		return p.parseScalarDefinition(s)
	}

	return fmt.Errorf("%w: unexpected token at position %d", ErrInvalidSDL, p.pos)
}

func (p *sdlParser) parseObjectType(s *Schema) error {
	p.skipWhitespaceAndComments()

	name, err := p.parseName()
	if err != nil {
		return err
	}

	p.skipWhitespaceAndComments()
	if !p.match("{") {
		return fmt.Errorf("%w: expected '{' after type name", ErrInvalidSDL)
	}

	t := &Type{
		Kind:   TypeKindObject,
		Name:   name,
		Fields: make(map[string]*Field),
	}

	p.skipWhitespaceAndComments()
	for !p.match("}") && p.pos < len(p.input) {
		field, err := p.parseField()
		if err != nil {
			return err
		}
		t.Fields[field.Name] = field
		p.skipWhitespaceAndComments()
	}

	if _, exists := s.types[name]; !exists {
		s.types[name] = t
	} else {
		existing := s.types[name]
		for fn, f := range t.Fields {
			existing.Fields[fn] = f
		}
	}

	return nil
}

func (p *sdlParser) parseSchemaDefinition(s *Schema) error {
	p.skipWhitespaceAndComments()
	if !p.match("{") {
		return fmt.Errorf("%w: expected '{' after schema", ErrInvalidSDL)
	}

	p.skipWhitespaceAndComments()
	for !p.match("}") && p.pos < len(p.input) {
		name, err := p.parseName()
		if err != nil {
			return err
		}
		p.skipWhitespaceAndComments()
		if !p.match(":") {
			return fmt.Errorf("%w: expected ':' in schema definition", ErrInvalidSDL)
		}
		p.skipWhitespaceAndComments()
		typeName, err := p.parseName()
		if err != nil {
			return err
		}

		if t, ok := s.types[typeName]; ok {
			if name == "query" {
				t.Kind = TypeKindQuery
				s.queryType = t
			} else if name == "mutation" {
				t.Kind = TypeKindMutation
				s.mutationType = t
			}
		}
		p.skipWhitespaceAndComments()
	}
	return nil
}

func (p *sdlParser) parseScalarDefinition(s *Schema) error {
	p.skipWhitespaceAndComments()
	name, err := p.parseName()
	if err != nil {
		return err
	}
	if _, exists := s.types[name]; !exists {
		s.types[name] = &Type{
			Kind:      TypeKindScalar,
			Name:      name,
			IsBuiltin: false,
		}
	}
	return nil
}

func (p *sdlParser) parseField() (*Field, error) {
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}

	field := &Field{
		Name: name,
		Args: make(map[string]*Argument),
	}

	p.skipWhitespaceAndComments()
	if p.match("(") {
		p.skipWhitespaceAndComments()
		for !p.match(")") && p.pos < len(p.input) {
			arg, err := p.parseArgument()
			if err != nil {
				return nil, err
			}
			field.Args[arg.Name] = arg
			p.skipWhitespaceAndComments()
			p.match(",")
			p.skipWhitespaceAndComments()
		}
	}

	p.skipWhitespaceAndComments()
	if !p.match(":") {
		return nil, fmt.Errorf("%w: expected ':' after field name", ErrInvalidSDL)
	}

	p.skipWhitespaceAndComments()
	fieldType, err := p.parseTypeReference()
	if err != nil {
		return nil, err
	}
	field.Type = fieldType
	field.NonNull = fieldType.Kind == TypeKindNonNull

	return field, nil
}

func (p *sdlParser) parseArgument() (*Argument, error) {
	name, err := p.parseName()
	if err != nil {
		return nil, err
	}

	arg := &Argument{Name: name}

	p.skipWhitespaceAndComments()
	if !p.match(":") {
		return nil, fmt.Errorf("%w: expected ':' in argument definition", ErrInvalidSDL)
	}

	p.skipWhitespaceAndComments()
	argType, err := p.parseTypeReference()
	if err != nil {
		return nil, err
	}
	arg.Type = argType

	p.skipWhitespaceAndComments()
	if p.match("=") {
		p.skipWhitespaceAndComments()
		defaultVal, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		arg.Default = defaultVal
	}

	return arg, nil
}

func (p *sdlParser) parseTypeReference() (*Type, error) {
	var t *Type

	if p.match("[") {
		p.skipWhitespaceAndComments()
		innerType, err := p.parseTypeReference()
		if err != nil {
			return nil, err
		}
		p.skipWhitespaceAndComments()
		if !p.match("]") {
			return nil, fmt.Errorf("%w: expected ']'", ErrInvalidSDL)
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

		kind := TypeKindObject
		if p.schema != nil {
			if existing, ok := p.schema.types[name]; ok {
				kind = existing.Kind
			}
		}

		t = &Type{
			Kind: kind,
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

func (p *sdlParser) parseValue() (interface{}, error) {
	p.skipWhitespaceAndComments()

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
	if p.peek() == '-' || (p.peek() >= '0' && p.peek() <= '9') {
		return p.parseNumber()
	}

	name, err := p.parseName()
	if err != nil {
		return nil, err
	}
	return name, nil
}

func (p *sdlParser) parseName() (string, error) {
	p.skipWhitespaceAndComments()

	if p.pos >= len(p.input) {
		return "", fmt.Errorf("%w: unexpected end of input", ErrInvalidSDL)
	}

	start := p.pos
	if !isIdentStart(p.peek()) {
		return "", fmt.Errorf("%w: expected identifier at position %d", ErrInvalidSDL, p.pos)
	}

	p.pos++
	for p.pos < len(p.input) && isIdentContinue(p.peek()) {
		p.pos++
	}

	return p.input[start:p.pos], nil
}

func (p *sdlParser) parseString() (string, error) {
	var sb strings.Builder
	for p.pos < len(p.input) && p.peek() != '"' {
		if p.peek() == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return "", fmt.Errorf("%w: unterminated string escape", ErrInvalidSDL)
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
		return "", fmt.Errorf("%w: unterminated string", ErrInvalidSDL)
	}
	return sb.String(), nil
}

func (p *sdlParser) parseNumber() (interface{}, error) {
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

func (p *sdlParser) skipWhitespaceAndComments() {
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

func (p *sdlParser) peek() byte {
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *sdlParser) match(s string) bool {
	if p.pos+len(s) > len(p.input) {
		return false
	}
	if p.input[p.pos:p.pos+len(s)] == s {
		p.pos += len(s)
		return true
	}
	return false
}

func (p *sdlParser) matchKeyword(kw string) bool {
	saved := p.pos
	p.skipWhitespaceAndComments()

	start := p.pos
	if !p.match(kw) {
		p.pos = saved
		return false
	}

	if p.pos < len(p.input) && isIdentContinue(p.peek()) {
		p.pos = saved
		return false
	}
	_ = start
	return true
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentContinue(ch byte) bool {
	return isIdentStart(ch) || (ch >= '0' && ch <= '9')
}

func (t *Type) Unwrap() *Type {
	current := t
	for current.Kind == TypeKindNonNull || current.Kind == TypeKindList {
		if current.OfType == nil {
			break
		}
		current = current.OfType
	}
	return current
}

func (t *Type) IsList() bool {
	current := t
	if current.Kind == TypeKindNonNull {
		current = current.OfType
	}
	return current != nil && current.Kind == TypeKindList
}

func (t *Type) IsNonNull() bool {
	return t.Kind == TypeKindNonNull
}

func (t *Type) InnerType() *Type {
	if t.Kind == TypeKindNonNull || t.Kind == TypeKindList {
		return t.OfType
	}
	return t
}
