package templater

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func (e *Engine) RegisterFunction(name string, fn interface{}) error {
	if name == "" {
		return ErrFunctionNotFound
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.functions[name] = fn
	return nil
}

func (e *Engine) RegisterTemplate(name string, source string) error {
	if name == "" {
		return ErrEmptyTemplateName
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.templates[name] = source
	delete(e.cache, name)
	return nil
}

func (e *Engine) InvalidateCache(name string) error {
	if name == "" {
		return ErrEmptyTemplateName
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.cache[name]; !ok {
		return ErrTemplateNotFound
	}
	delete(e.cache, name)
	return nil
}

func (e *Engine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cache = make(map[string]*Template)
}

func (e *Engine) getExtendsParent(name string) (string, bool, error) {
	e.mu.RLock()
	source, ok := e.templates[name]
	e.mu.RUnlock()
	if !ok {
		return "", false, ErrTemplateNotFound
	}
	matches := extendsPattern.FindStringSubmatch(source)
	if len(matches) < 2 {
		return "", false, nil
	}
	return matches[1], true, nil
}

func (e *Engine) GetTemplate(name string) (*Template, error) {
	e.mu.RLock()
	cached, ok := e.cache[name]
	e.mu.RUnlock()
	if ok {
		return cached, nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	cached, ok = e.cache[name]
	if ok {
		return cached, nil
	}

	source, ok := e.templates[name]
	if !ok {
		return nil, ErrTemplateNotFound
	}

	tmpl, err := parseTemplate(name, source)
	if err != nil {
		return nil, err
	}

	e.cache[name] = tmpl
	return tmpl, nil
}

func (e *Engine) Render(name string, data interface{}) (string, error) {
	return e.RenderWithVisited(name, data, make(map[string]bool))
}

func (e *Engine) RenderWithVisited(name string, data interface{}, visited map[string]bool) (string, error) {
	if visited[name] {
		return "", ErrTemplateInheritanceLoop
	}
	visited[name] = true

	tmpl, err := e.GetTemplate(name)
	if err != nil {
		return "", err
	}

	if tmpl.Extends != nil {
		if visited[tmpl.Extends.ParentName] {
			return "", ErrTemplateInheritanceLoop
		}
		parentTmpl, err := e.GetTemplate(tmpl.Extends.ParentName)
		if err != nil {
			if err == ErrTemplateNotFound {
				return "", ErrParentTemplateNotFound
			}
			return "", err
		}

		visited[tmpl.Extends.ParentName] = true

		currentParent := tmpl.Extends.ParentName
		for {
			grandparent, hasExtends, err := e.getExtendsParent(currentParent)
			if err != nil {
				if err == ErrTemplateNotFound {
					return "", ErrParentTemplateNotFound
				}
				return "", err
			}
			if !hasExtends {
				break
			}
			if visited[grandparent] {
				return "", ErrTemplateInheritanceLoop
			}
			visited[grandparent] = true
			currentParent = grandparent
		}

		parentCopy := &Template{
			Name:   parentTmpl.Name,
			Source: parentTmpl.Source,
			Nodes:  make([]Node, len(parentTmpl.Nodes)),
			Blocks: make(map[string]*BlockNode),
		}
		copy(parentCopy.Nodes, parentTmpl.Nodes)
		for k, v := range parentTmpl.Blocks {
			parentCopy.Blocks[k] = v
		}

		for blockName, childBlock := range tmpl.Blocks {
			if _, exists := parentTmpl.Blocks[blockName]; !exists {
				return "", ErrBlockNotFound
			}
			parentCopy.Blocks[blockName] = childBlock
		}

		mergedNodes := make([]Node, 0, len(parentCopy.Nodes))
		for _, node := range parentCopy.Nodes {
			if bn, ok := node.(*BlockNode); ok {
				if override, exists := tmpl.Blocks[bn.Name]; exists {
					mergedNodes = append(mergedNodes, override)
					continue
				}
			}
			mergedNodes = append(mergedNodes, node)
		}
		parentCopy.Nodes = mergedNodes

		e.mu.RLock()
		funcs := make(map[string]interface{})
		for k, v := range e.functions {
			funcs[k] = v
		}
		e.mu.RUnlock()

		return renderNodes(parentCopy.Nodes, parentCopy.Blocks, data, funcs, e.config, visited)
	}

	e.mu.RLock()
	funcs := make(map[string]interface{})
	for k, v := range e.functions {
		funcs[k] = v
	}
	e.mu.RUnlock()

	return renderNodes(tmpl.Nodes, tmpl.Blocks, data, funcs, e.config, visited)
}

var (
	varPattern       = regexp.MustCompile(`\{\{\s*\.([a-zA-Z_][a-zA-Z0-9_.]*)\s*\}\}`)
	dollarVarPattern = regexp.MustCompile(`\{\{\s*(\$[a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)
	funcPattern      = regexp.MustCompile(`\{\{\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*((?:\s+\S+)*)\s*\}\}`)
	ifPattern        = regexp.MustCompile(`\{\{\s*if\s+(.+?)\s*\}\}`)
	elseIfPattern    = regexp.MustCompile(`\{\{\s*else\s+if\s+(.+?)\s*\}\}`)
	elsePattern      = regexp.MustCompile(`\{\{\s*else\s*\}\}`)
	endIfPattern     = regexp.MustCompile(`\{\{\s*endif\s*\}\}`)
	rangePattern     = regexp.MustCompile(`\{\{\s*range\s+(\$[a-zA-Z_][a-zA-Z0-9_]*)\s*,\s*(\$[a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*range\s+(.+?)\s*\}\}`)
	rangeSimplePattern = regexp.MustCompile(`\{\{\s*range\s+(\$[a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*range\s+(.+?)\s*\}\}`)
	endRangePattern  = regexp.MustCompile(`\{\{\s*endrange\s*\}\}`)
	blockPattern     = regexp.MustCompile(`\{\{\s*block\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\}\}`)
	endBlockPattern  = regexp.MustCompile(`\{\{\s*endblock\s*\}\}`)
	extendsPattern   = regexp.MustCompile(`\{\{\s*extends\s+"([^"]+)"\s*\}\}`)
)

func parseTemplate(name string, source string) (*Template, error) {
	tmpl := &Template{
		Name:   name,
		Source: source,
		Blocks: make(map[string]*BlockNode),
	}

	var err error
	tmpl.Nodes, tmpl.Extends, tmpl.Blocks, err = parseContent(source)
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}

func parseContent(source string) ([]Node, *ExtendsNode, map[string]*BlockNode, error) {
	nodes := make([]Node, 0)
	blocks := make(map[string]*BlockNode)
	var extends *ExtendsNode

	pos := 0
	for pos < len(source) {
		nextVar := strings.Index(source[pos:], "{{")
		if nextVar == -1 {
			if pos < len(source) {
				nodes = append(nodes, &TextNode{Content: source[pos:]})
			}
			break
		}
		nextVar += pos

		if nextVar > pos {
			nodes = append(nodes, &TextNode{Content: source[pos:nextVar]})
		}

		endTag := strings.Index(source[nextVar:], "}}")
		if endTag == -1 {
			return nil, nil, nil, ErrUnclosedBlock
		}
		endTag += nextVar + 2

		tagContent := strings.TrimSpace(source[nextVar+2 : endTag-2])

		if matches := extendsPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			extends = &ExtendsNode{ParentName: matches[1]}
			pos = endTag
			continue
		}

		if matches := blockPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			blockName := matches[1]
			if blockName == "" {
				return nil, nil, nil, ErrInvalidBlockSyntax
			}
			blockEndTag, blockNodes, err := findMatchingEnd(source, endTag, "block", "endblock")
			if err != nil {
				return nil, nil, nil, err
			}
			blockNode := &BlockNode{Name: blockName, Nodes: blockNodes}
			nodes = append(nodes, blockNode)
			blocks[blockName] = blockNode
			pos = blockEndTag
			continue
		}

		if strings.HasPrefix(tagContent, "block ") {
			return nil, nil, nil, ErrInvalidBlockSyntax
		}

		if matches := rangePattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			indexVar := matches[1]
			valueVar := matches[2]
			iterable := strings.TrimSpace(matches[3])
			rangeEndTag, rangeNodes, err := findMatchingEnd(source, endTag, "range", "endrange")
			if err != nil {
				return nil, nil, nil, err
			}
			rangeNode := &RangeNode{
				Iterable: iterable,
				IndexVar: indexVar,
				ValueVar: valueVar,
				Nodes:    rangeNodes,
			}
			nodes = append(nodes, rangeNode)
			pos = rangeEndTag
			continue
		}

		if matches := rangeSimplePattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			valueVar := matches[1]
			iterable := strings.TrimSpace(matches[2])
			rangeEndTag, rangeNodes, err := findMatchingEnd(source, endTag, "range", "endrange")
			if err != nil {
				return nil, nil, nil, err
			}
			rangeNode := &RangeNode{
				Iterable: iterable,
				IndexVar: "",
				ValueVar: valueVar,
				Nodes:    rangeNodes,
			}
			nodes = append(nodes, rangeNode)
			pos = rangeEndTag
			continue
		}

		if strings.HasPrefix(tagContent, "range ") {
			return nil, nil, nil, ErrInvalidRange
		}

		if matches := ifPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			condition := strings.TrimSpace(matches[1])
			ifEndTag, trueNodes, falseNodes, err := parseIfBlock(source, endTag, condition)
			if err != nil {
				return nil, nil, nil, err
			}
			ifNode := &IfNode{
				Condition:  condition,
				TrueNodes:  trueNodes,
				FalseNodes: falseNodes,
			}
			nodes = append(nodes, ifNode)
			pos = ifEndTag
			continue
		}

		if matches := dollarVarPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			name := matches[1]
			nodes = append(nodes, &VariableNode{Path: name})
			pos = endTag
			continue
		}

		if matches := varPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			path := matches[1]
			nodes = append(nodes, &VariableNode{Path: path})
			pos = endTag
			continue
		}

		if matches := funcPattern.FindStringSubmatch("{{" + tagContent + "}}"); len(matches) > 0 {
			funcName := matches[1]
			argsStr := strings.TrimSpace(matches[2])
			var args []string
			if argsStr != "" {
				args = strings.Fields(argsStr)
			}
			if funcName == "if" || funcName == "endif" || funcName == "else" || funcName == "range" || funcName == "endrange" || funcName == "block" || funcName == "endblock" || funcName == "extends" {
				pos = endTag
				continue
			}
			nodes = append(nodes, &FunctionNode{Name: funcName, Arguments: args})
			pos = endTag
			continue
		}

		nodes = append(nodes, &TextNode{Content: "{{" + tagContent + "}}"})
		pos = endTag
	}

	return nodes, extends, blocks, nil
}

func findMatchingEnd(source string, startPos int, openTag string, closeTag string) (int, []Node, error) {
	depth := 1
	pos := startPos
	for depth > 0 {
		nextOpen := strings.Index(source[pos:], "{{ "+openTag)
		if nextOpen != -1 {
			nextOpen += pos
		}
		nextClose := strings.Index(source[pos:], "{{ "+closeTag)
		if nextClose != -1 {
			nextClose += pos
		}
		nextAltOpen := strings.Index(source[pos:], "{{"+openTag)
		if nextAltOpen != -1 {
			nextAltOpen += pos
			if nextOpen == -1 || nextAltOpen < nextOpen {
				nextOpen = nextAltOpen
			}
		}
		nextAltClose := strings.Index(source[pos:], "{{"+closeTag)
		if nextAltClose != -1 {
			nextAltClose += pos
			if nextClose == -1 || nextAltClose < nextClose {
				nextClose = nextAltClose
			}
		}

		if nextClose == -1 {
			return 0, nil, ErrUnclosedBlock
		}

		if nextOpen != -1 && nextOpen < nextClose {
			depth++
			pos = nextOpen + len("{{"+openTag)
		} else {
			depth--
			if depth == 0 {
				endEndTag := strings.Index(source[nextClose:], "}}")
				if endEndTag == -1 {
					return 0, nil, ErrUnclosedBlock
				}
				endEndTag += nextClose + 2

				content := source[startPos:nextClose]
				nodes, _, _, err := parseContent(content)
				if err != nil {
					return 0, nil, err
				}
				return endEndTag, nodes, nil
			}
			pos = nextClose + len("{{"+closeTag)
		}
	}
	return 0, nil, ErrUnclosedBlock
}

func parseIfBlock(source string, startPos int, initialCondition string) (int, []Node, []Node, error) {
	var trueNodes []Node
	var falseNodes []Node
	currentIsTrue := true

	pos := startPos
	depth := 1
	for depth > 0 {
		nextIf := strings.Index(source[pos:], "{{ if ")
		if nextIf != -1 {
			nextIf += pos
		}
		nextEndIf := strings.Index(source[pos:], "{{ endif")
		if nextEndIf != -1 {
			nextEndIf += pos
		}
		nextElseIf := strings.Index(source[pos:], "{{ else if ")
		if nextElseIf != -1 {
			nextElseIf += pos
		}
		nextAltElseIf := strings.Index(source[pos:], "{{else if ")
		if nextAltElseIf != -1 {
			nextAltElseIf += pos
			if nextElseIf == -1 || nextAltElseIf < nextElseIf {
				nextElseIf = nextAltElseIf
			}
		}
		nextElse := strings.Index(source[pos:], "{{ else")
		if nextElse != -1 {
			nextElse += pos
		}
		nextAltIf := strings.Index(source[pos:], "{{if ")
		if nextAltIf != -1 {
			nextAltIf += pos
			if nextIf == -1 || nextAltIf < nextIf {
				nextIf = nextAltIf
			}
		}
		nextAltEndIf := strings.Index(source[pos:], "{{endif")
		if nextAltEndIf != -1 {
			nextAltEndIf += pos
			if nextEndIf == -1 || nextAltEndIf < nextEndIf {
				nextEndIf = nextAltEndIf
			}
		}
		nextAltElse := strings.Index(source[pos:], "{{else")
		if nextAltElse != -1 {
			nextAltElse += pos
			if nextElse == -1 || nextAltElse < nextElse {
				nextElse = nextAltElse
			}
		}

		minPos := -1
		var tagType int
		if nextIf != -1 {
			minPos = nextIf
			tagType = 0
		}
		if nextElseIf != -1 && (minPos == -1 || nextElseIf < minPos) {
			minPos = nextElseIf
			tagType = 3
		}
		if nextElse != -1 && (minPos == -1 || nextElse < minPos) {
			minPos = nextElse
			tagType = 1
		}
		if nextEndIf != -1 && (minPos == -1 || nextEndIf < minPos) {
			minPos = nextEndIf
			tagType = 2
		}

		if minPos == -1 {
			return 0, nil, nil, ErrUnclosedBlock
		}

		switch tagType {
		case 0:
			if depth == 1 {
				content := source[pos:minPos]
				nodes, _, _, err := parseContent(content)
				if err != nil {
					return 0, nil, nil, err
				}
				if currentIsTrue {
					trueNodes = append(trueNodes, nodes...)
				} else {
					falseNodes = append(falseNodes, nodes...)
				}
			}
			depth++
			endTag := strings.Index(source[minPos:], "}}")
			pos = minPos + endTag + 2
		case 3:
			if depth == 1 {
				content := source[pos:minPos]
				nodes, _, _, err := parseContent(content)
				if err != nil {
					return 0, nil, nil, err
				}
				if currentIsTrue {
					trueNodes = append(trueNodes, nodes...)
				} else {
					falseNodes = append(falseNodes, nodes...)
				}

				elseIfTag := source[minPos:]
				matches := elseIfPattern.FindStringSubmatch(elseIfTag)
				if len(matches) < 2 {
					return 0, nil, nil, ErrInvalidCondition
				}
				condition := strings.TrimSpace(matches[1])

				endTag := strings.Index(elseIfTag, "}}")
				remainingStart := minPos + endTag + 2

				nestedEndPos, nestedTrue, nestedFalse, err := parseIfBlock(source, remainingStart, condition)
				if err != nil {
					return 0, nil, nil, err
				}

				nestedIfNode := &IfNode{
					Condition:  condition,
					TrueNodes:  nestedTrue,
					FalseNodes: nestedFalse,
				}
				falseNodes = append(falseNodes, nestedIfNode)
				return nestedEndPos, trueNodes, falseNodes, nil
			}
			depth++
			endTag := strings.Index(source[minPos:], "}}")
			pos = minPos + endTag + 2
		case 1:
			if depth == 1 {
				content := source[pos:minPos]
				nodes, _, _, err := parseContent(content)
				if err != nil {
					return 0, nil, nil, err
				}
				if currentIsTrue {
					trueNodes = append(trueNodes, nodes...)
				} else {
					falseNodes = append(falseNodes, nodes...)
				}
				currentIsTrue = false
			}
			endTag := strings.Index(source[minPos:], "}}")
			pos = minPos + endTag + 2
		case 2:
			depth--
			if depth == 0 {
				content := source[pos:minPos]
				nodes, _, _, err := parseContent(content)
				if err != nil {
					return 0, nil, nil, err
				}
				if currentIsTrue {
					trueNodes = append(trueNodes, nodes...)
				} else {
					falseNodes = append(falseNodes, nodes...)
				}
				endEndTag := strings.Index(source[minPos:], "}}")
				return minPos + endEndTag + 2, trueNodes, falseNodes, nil
			}
			endTag := strings.Index(source[minPos:], "}}")
			pos = minPos + endTag + 2
		}
	}

	return 0, nil, nil, ErrUnclosedBlock
}

func renderNodes(nodes []Node, blocks map[string]*BlockNode, data interface{}, funcs map[string]interface{}, config Config, visited map[string]bool) (string, error) {
	var sb strings.Builder
	for _, node := range nodes {
		switch n := node.(type) {
		case *TextNode:
			sb.WriteString(n.Content)
		case *VariableNode:
			var val interface{}
			var err error
			if strings.HasPrefix(n.Path, "$") {
				val, err = resolveDollarVariable(n.Path, data)
			} else {
				val, err = resolveVariable(n.Path, data)
			}
			if err != nil {
				if err == ErrVariableNotFound && !config.StrictVariables {
					continue
				}
				return "", err
			}
			sb.WriteString(stringifyValue(val))
		case *FunctionNode:
			result, err := callFunction(n.Name, n.Arguments, data, funcs)
			if err != nil {
				return "", err
			}
			sb.WriteString(stringifyValue(result))
		case *IfNode:
			condResult, err := evaluateCondition(n.Condition, data)
			if err != nil {
				return "", err
			}
			if condResult {
				rendered, err := renderNodes(n.TrueNodes, blocks, data, funcs, config, visited)
				if err != nil {
					return "", err
				}
				sb.WriteString(rendered)
			} else {
				rendered, err := renderNodes(n.FalseNodes, blocks, data, funcs, config, visited)
				if err != nil {
					return "", err
				}
				sb.WriteString(rendered)
			}
		case *RangeNode:
			iterVal, err := resolveVariable(strings.TrimPrefix(n.Iterable, "."), data)
			if err != nil {
				return "", err
			}
			items, err := toSlice(iterVal)
			if err != nil {
				return "", err
			}
			for i, item := range items {
				scopeData := make(map[string]interface{})
				if m, ok := data.(map[string]interface{}); ok {
					for k, v := range m {
						scopeData[k] = v
					}
				}
				if n.IndexVar != "" {
					scopeData[strings.TrimPrefix(n.IndexVar, "$")] = i
				}
				scopeData[strings.TrimPrefix(n.ValueVar, "$")] = item
				rendered, err := renderNodes(n.Nodes, blocks, scopeData, funcs, config, visited)
				if err != nil {
					return "", err
				}
				sb.WriteString(rendered)
			}
		case *BlockNode:
			rendered, err := renderNodes(n.Nodes, blocks, data, funcs, config, visited)
			if err != nil {
				return "", err
			}
			sb.WriteString(rendered)
		}
	}
	return sb.String(), nil
}

func resolveDollarVariable(name string, data interface{}) (interface{}, error) {
	key := strings.TrimPrefix(name, "$")
	if key == "" {
		return nil, ErrInvalidVariablePath
	}
	if m, ok := data.(map[string]interface{}); ok {
		if val, exists := m[key]; exists {
			return val, nil
		}
	}
	return nil, ErrVariableNotFound
}

var fieldNotFound = new(struct{})

func resolveVariable(path string, data interface{}) (interface{}, error) {
	if path == "" {
		return nil, ErrInvalidVariablePath
	}
	parts := strings.Split(path, ".")
	current := data
	for _, part := range parts {
		if part == "" {
			return nil, ErrInvalidVariablePath
		}
		if current == nil {
			return nil, ErrVariableNotFound
		}
		result := getField(current, part)
		if result == fieldNotFound {
			return nil, ErrVariableNotFound
		}
		current = result
	}
	return current, nil
}

func getField(data interface{}, field string) interface{} {
	if data == nil {
		return fieldNotFound
	}
	switch v := data.(type) {
	case map[string]interface{}:
		if val, ok := v[field]; ok {
			return val
		}
		return fieldNotFound
	case map[string]string:
		if val, ok := v[field]; ok {
			return val
		}
		return fieldNotFound
	case map[string]int:
		if val, ok := v[field]; ok {
			return val
		}
		return fieldNotFound
	default:
		rv := reflect.ValueOf(data)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			fv := rv.FieldByName(field)
			if fv.IsValid() {
				return fv.Interface()
			}
		}
		return fieldNotFound
	}
}

func evaluateCondition(condition string, data interface{}) (bool, error) {
	condition = strings.TrimSpace(condition)

	if strings.HasPrefix(condition, ".") || strings.HasPrefix(condition, "$") {
		parts := strings.SplitN(condition, " ", 2)
		if len(parts) == 1 {
			var val interface{}
			var err error
			if strings.HasPrefix(condition, "$") {
				val, err = resolveDollarVariable(condition, data)
			} else {
				val, err = resolveVariable(strings.TrimPrefix(condition, "."), data)
			}
			if err != nil {
				return false, err
			}
			return isTruthy(val), nil
		}
	}

	if strings.Contains(condition, " == ") {
		parts := strings.SplitN(condition, " == ", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		leftVal, err := resolveExpr(left, data)
		if err != nil {
			return false, err
		}
		rightVal, err := resolveExpr(right, data)
		if err != nil {
			return false, err
		}
		return valuesEqual(leftVal, rightVal), nil
	}

	if strings.Contains(condition, " != ") {
		parts := strings.SplitN(condition, " != ", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		leftVal, err := resolveExpr(left, data)
		if err != nil {
			return false, err
		}
		rightVal, err := resolveExpr(right, data)
		if err != nil {
			return false, err
		}
		return !valuesEqual(leftVal, rightVal), nil
	}

	if strings.HasPrefix(condition, "empty ") {
		varPath := strings.TrimSpace(strings.TrimPrefix(condition, "empty "))
		var val interface{}
		var err error
		if strings.HasPrefix(varPath, "$") {
			val, err = resolveDollarVariable(varPath, data)
		} else {
			val, err = resolveVariable(strings.TrimPrefix(varPath, "."), data)
		}
		if err != nil {
			return true, nil
		}
		return isEmpty(val), nil
	}

	if strings.HasPrefix(condition, ".") || strings.HasPrefix(condition, "$") {
		var val interface{}
		var err error
		if strings.HasPrefix(condition, "$") {
			val, err = resolveDollarVariable(condition, data)
		} else {
			val, err = resolveVariable(strings.TrimPrefix(condition, "."), data)
		}
		if err != nil {
			return false, err
		}
		return isTruthy(val), nil
	}

	return false, ErrInvalidCondition
}

func resolveExpr(expr string, data interface{}) (interface{}, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "$") {
		return resolveDollarVariable(expr, data)
	}
	if strings.HasPrefix(expr, ".") {
		return resolveVariable(strings.TrimPrefix(expr, "."), data)
	}
	if strings.HasPrefix(expr, "\"") && strings.HasSuffix(expr, "\"") {
		return expr[1 : len(expr)-1], nil
	}
	if strings.HasPrefix(expr, "'") && strings.HasSuffix(expr, "'") {
		return expr[1 : len(expr)-1], nil
	}
	if n, err := strconv.Atoi(expr); err == nil {
		return n, nil
	}
	if f, err := strconv.ParseFloat(expr, 64); err == nil {
		return f, nil
	}
	if expr == "true" {
		return true, nil
	}
	if expr == "false" {
		return false, nil
	}
	if expr == "nil" || expr == "null" {
		return nil, nil
	}
	return expr, nil
}

func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func canBeNil(k reflect.Kind) bool {
	switch k {
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return true
	}
	return false
}

func isTruthy(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != ""
	case int, int8, int16, int32, int64:
		return reflect.ValueOf(v).Int() != 0
	case uint, uint8, uint16, uint32, uint64:
		return reflect.ValueOf(v).Uint() != 0
	case float32, float64:
		return reflect.ValueOf(v).Float() != 0
	default:
		rv := reflect.ValueOf(v)
		switch rv.Kind() {
		case reflect.Slice, reflect.Map, reflect.Array:
			return rv.Len() > 0
		case reflect.Ptr, reflect.Interface:
			return !rv.IsNil()
		}
		return true
	}
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.Len() == 0
	case reflect.Slice, reflect.Map, reflect.Array:
		return rv.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	}
	return false
}

func toSlice(v interface{}) ([]interface{}, error) {
	if v == nil {
		return nil, ErrRangeNotIterable
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		result := make([]interface{}, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			result[i] = rv.Index(i).Interface()
		}
		return result, nil
	default:
		return nil, ErrRangeNotIterable
	}
}

func stringifyValue(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func callFunction(name string, args []string, data interface{}, funcs map[string]interface{}) (interface{}, error) {
	fn, ok := funcs[name]
	if !ok {
		return nil, ErrFunctionNotFound
	}
	fv := reflect.ValueOf(fn)
	ft := fv.Type()

	if ft.Kind() != reflect.Func {
		return nil, ErrInvalidFunctionCall
	}

	numIn := ft.NumIn()
	if len(args) != numIn {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrInvalidArgumentCount, numIn, len(args))
	}

	inVals := make([]reflect.Value, len(args))
	for i, arg := range args {
		var argVal interface{}
		if strings.HasPrefix(arg, "$") {
			var err error
			argVal, err = resolveDollarVariable(arg, data)
			if err != nil {
				return nil, err
			}
		} else if strings.HasPrefix(arg, ".") {
			var err error
			argVal, err = resolveVariable(strings.TrimPrefix(arg, "."), data)
			if err != nil {
				return nil, err
			}
		} else {
			argVal = arg
			if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
				argVal = arg[1 : len(arg)-1]
			}
		}

		expectedType := ft.In(i)
		converted, err := convertToType(argVal, expectedType)
		if err != nil {
			return nil, err
		}
		inVals[i] = reflect.ValueOf(converted)
	}

	results := fv.Call(inVals)
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 1 {
		return results[0].Interface(), nil
	}
	if len(results) == 2 {
		second := results[1]
		secondType := second.Type()
		if !secondType.Implements(errorType) {
			return nil, ErrInvalidFunctionCall
		}
		if canBeNil(second.Kind()) && second.IsNil() {
			return results[0].Interface(), nil
		}
		if errVal, ok := second.Interface().(error); ok {
			return nil, errVal
		}
		return nil, ErrInvalidFunctionCall
	}
	return results[0].Interface(), nil
}

func convertToType(val interface{}, targetType reflect.Type) (interface{}, error) {
	if val == nil {
		return reflect.Zero(targetType).Interface(), nil
	}

	valType := reflect.TypeOf(val)
	if valType.AssignableTo(targetType) {
		return val, nil
	}
	if valType.ConvertibleTo(targetType) {
		return reflect.ValueOf(val).Convert(targetType).Interface(), nil
	}

	switch targetType.Kind() {
	case reflect.String:
		return fmt.Sprintf("%v", val), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := val.(type) {
		case string:
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, err
			}
			return reflect.ValueOf(n).Convert(targetType).Interface(), nil
		case float64:
			return reflect.ValueOf(int64(v)).Convert(targetType).Interface(), nil
		case float32:
			return reflect.ValueOf(int64(v)).Convert(targetType).Interface(), nil
		}
	case reflect.Float32, reflect.Float64:
		switch v := val.(type) {
		case string:
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, err
			}
			return reflect.ValueOf(f).Convert(targetType).Interface(), nil
		case int:
			return reflect.ValueOf(float64(v)).Convert(targetType).Interface(), nil
		}
	case reflect.Bool:
		switch v := val.(type) {
		case string:
			b, err := strconv.ParseBool(v)
			if err != nil {
				return nil, err
			}
			return b, nil
		}
	}

	return nil, fmt.Errorf("cannot convert %T to %v", val, targetType)
}
