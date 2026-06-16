package contentsec

import (
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	ErrNilFilter         = errors.New("contentsec: filter function cannot be nil")
	ErrFilterExists      = errors.New("contentsec: filter with same name already exists")
	ErrFilterNotFound    = errors.New("contentsec: filter not found")
	ErrInvalidPattern    = errors.New("contentsec: invalid regex pattern")
	ErrNilSanitizerRule  = errors.New("contentsec: sanitizer rule cannot be nil")
	ErrNilMaskPattern    = errors.New("contentsec: mask pattern cannot be nil")
	ErrMaskPatternExists = errors.New("contentsec: mask pattern with same name already exists")
	ErrMaskPatternNotFound = errors.New("contentsec: mask pattern not found")
)

// ==================== XSS Filter ====================

type XSSViolation struct {
	Position int
	Type     string
	Content  string
}

type XSSFilterFunc func(input string) []XSSViolation

type XSSFilter struct {
	Name string
	Func XSSFilterFunc
}

type XSSDetector struct {
	mu      sync.RWMutex
	filters map[string]XSSFilterFunc
}

func NewXSSDetector() *XSSDetector {
	d := &XSSDetector{
		filters: make(map[string]XSSFilterFunc),
	}
	d.registerBuiltinFilters()
	return d
}

func (d *XSSDetector) registerBuiltinFilters() {
	scriptTagPattern := regexp.MustCompile(`(?i)<\s*script[^>]*>.*?<\s*/\s*script\s*>`)
	d.filters["script_tag"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := scriptTagPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "script_tag",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	eventHandlerPattern := regexp.MustCompile(`(?i)\s+on\w+\s*=`)
	d.filters["event_handler"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := eventHandlerPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "event_handler",
				Content:  strings.TrimSpace(input[m[0]:m[1]]),
			})
		}
		return violations
	}

	javascriptProtocolPattern := regexp.MustCompile(`(?i)(href|src|action|data)\s*=\s*["']?\s*javascript:`)
	d.filters["javascript_protocol"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := javascriptProtocolPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "javascript_protocol",
				Content:  strings.TrimSpace(input[m[0]:m[1]]),
			})
		}
		return violations
	}

	iframePattern := regexp.MustCompile(`(?i)<\s*iframe[^>]*>`)
	d.filters["iframe_tag"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := iframePattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "iframe_tag",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	objectEmbedPattern := regexp.MustCompile(`(?i)<\s*(object|embed|applet)[^>]*>`)
	d.filters["object_embed_tag"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := objectEmbedPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "object_embed_tag",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	formTagPattern := regexp.MustCompile(`(?i)<\s*form[^>]*>`)
	d.filters["form_tag"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := formTagPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "form_tag",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	evalExpressionPattern := regexp.MustCompile(`(?i)\beval\s*\(`)
	d.filters["eval_expression"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := evalExpressionPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "eval_expression",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	vbscriptPattern := regexp.MustCompile(`(?i)vbscript:`)
	d.filters["vbscript_protocol"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := vbscriptPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "vbscript_protocol",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	dataUriScriptPattern := regexp.MustCompile(`(?i)data:\s*text/html`)
	d.filters["data_uri_html"] = func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := dataUriScriptPattern.FindAllStringIndex(input, -1)
		for _, m := range matches {
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     "data_uri_html",
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}
}

func (d *XSSDetector) RegisterFilter(name string, fn XSSFilterFunc) error {
	if name == "" {
		return errors.New("contentsec: filter name cannot be empty")
	}
	if fn == nil {
		return ErrNilFilter
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.filters[name]; exists {
		return ErrFilterExists
	}

	d.filters[name] = fn
	return nil
}

func (d *XSSDetector) UnregisterFilter(name string) error {
	if name == "" {
		return errors.New("contentsec: filter name cannot be empty")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.filters[name]; !exists {
		return ErrFilterNotFound
	}

	delete(d.filters, name)
	return nil
}

func (d *XSSDetector) RegisterPatternFilter(name, pattern, violationType string) error {
	if name == "" {
		return errors.New("contentsec: filter name cannot be empty")
	}
	if pattern == "" {
		return errors.New("contentsec: pattern cannot be empty")
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPattern, err)
	}

	fn := func(input string) []XSSViolation {
		var violations []XSSViolation
		matches := re.FindAllStringIndex(input, -1)
		for _, m := range matches {
			vt := violationType
			if vt == "" {
				vt = name
			}
			violations = append(violations, XSSViolation{
				Position: m[0],
				Type:     vt,
				Content:  input[m[0]:m[1]],
			})
		}
		return violations
	}

	return d.RegisterFilter(name, fn)
}

func (d *XSSDetector) Detect(input string) []XSSViolation {
	if input == "" {
		return nil
	}

	d.mu.RLock()
	filters := make([]XSSFilterFunc, 0, len(d.filters))
	for _, fn := range d.filters {
		filters = append(filters, fn)
	}
	d.mu.RUnlock()

	var allViolations []XSSViolation
	for _, fn := range filters {
		violations := fn(input)
		allViolations = append(allViolations, violations...)
	}

	sort.Slice(allViolations, func(i, j int) bool {
		return allViolations[i].Position < allViolations[j].Position
	})

	return allViolations
}

func (d *XSSDetector) FilterCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.filters)
}

func (d *XSSDetector) HasFilter(name string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, exists := d.filters[name]
	return exists
}

// ==================== Input Sanitizer ====================

type SanitizerConfig struct {
	AllowedTags       map[string]bool
	AllowedAttributes map[string]bool
	EscapeCharacters  map[rune]string
	StripComments     bool
}

func DefaultSanitizerConfig() *SanitizerConfig {
	return &SanitizerConfig{
		AllowedTags: map[string]bool{
			"a":          true,
			"b":          true,
			"br":         true,
			"code":       true,
			"div":        true,
			"em":         true,
			"h1":         true,
			"h2":         true,
			"h3":         true,
			"h4":         true,
			"h5":         true,
			"h6":         true,
			"hr":         true,
			"i":          true,
			"img":        true,
			"li":         true,
			"ol":         true,
			"p":          true,
			"pre":        true,
			"span":       true,
			"strong":     true,
			"table":      true,
			"tbody":      true,
			"td":         true,
			"th":         true,
			"thead":      true,
			"tr":         true,
			"u":          true,
			"ul":         true,
		},
		AllowedAttributes: map[string]bool{
			"alt":     true,
			"class":   true,
			"colspan": true,
			"href":    true,
			"id":      true,
			"rel":     true,
			"rowspan": true,
			"src":     true,
			"target":  true,
			"title":   true,
			"width":   true,
			"height":  true,
		},
		EscapeCharacters: map[rune]string{
			'<':  "&lt;",
			'>':  "&gt;",
			'&':  "&amp;",
			'"':  "&quot;",
			'\'': "&#39;",
		},
		StripComments: true,
	}
}

type HTMLSanitizer struct {
	mu     sync.RWMutex
	config *SanitizerConfig
}

func NewHTMLSanitizer() *HTMLSanitizer {
	return &HTMLSanitizer{
		config: DefaultSanitizerConfig(),
	}
}

func NewHTMLSanitizerWithConfig(cfg *SanitizerConfig) (*HTMLSanitizer, error) {
	if cfg == nil {
		return nil, ErrNilSanitizerRule
	}
	if cfg.AllowedTags == nil {
		cfg.AllowedTags = make(map[string]bool)
	}
	if cfg.AllowedAttributes == nil {
		cfg.AllowedAttributes = make(map[string]bool)
	}
	if cfg.EscapeCharacters == nil {
		cfg.EscapeCharacters = make(map[rune]string)
	}
	return &HTMLSanitizer{
		config: cfg,
	}, nil
}

func (s *HTMLSanitizer) SetConfig(cfg *SanitizerConfig) error {
	if cfg == nil {
		return ErrNilSanitizerRule
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = cfg
	return nil
}

func (s *HTMLSanitizer) GetConfig() *SanitizerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := *s.config
	cfg.AllowedTags = make(map[string]bool)
	for k, v := range s.config.AllowedTags {
		cfg.AllowedTags[k] = v
	}
	cfg.AllowedAttributes = make(map[string]bool)
	for k, v := range s.config.AllowedAttributes {
		cfg.AllowedAttributes[k] = v
	}
	cfg.EscapeCharacters = make(map[rune]string)
	for k, v := range s.config.EscapeCharacters {
		cfg.EscapeCharacters[k] = v
	}
	return &cfg
}

func (s *HTMLSanitizer) AddAllowedTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag != "" {
		s.config.AllowedTags[tag] = true
	}
}

func (s *HTMLSanitizer) RemoveAllowedTag(tag string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tag = strings.ToLower(strings.TrimSpace(tag))
	delete(s.config.AllowedTags, tag)
}

func (s *HTMLSanitizer) AddAllowedAttribute(attr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attr = strings.ToLower(strings.TrimSpace(attr))
	if attr != "" {
		s.config.AllowedAttributes[attr] = true
	}
}

func (s *HTMLSanitizer) RemoveAllowedAttribute(attr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	attr = strings.ToLower(strings.TrimSpace(attr))
	delete(s.config.AllowedAttributes, attr)
}

func (s *HTMLSanitizer) Sanitize(input string) string {
	if input == "" {
		return ""
	}

	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()

	result := input

	if cfg.StripComments {
		commentPattern := regexp.MustCompile(`<!--.*?-->`)
		result = commentPattern.ReplaceAllString(result, "")
	}

	result = s.sanitizeHTML(result, cfg)

	return result
}

func (s *HTMLSanitizer) sanitizeHTML(input string, cfg *SanitizerConfig) string {
	var result strings.Builder
	i := 0
	for i < len(input) {
		if input[i] == '<' {
			end := strings.IndexByte(input[i:], '>')
			if end == -1 {
				result.WriteString(s.escapeSpecialChars(input[i:], cfg))
				break
			}
			end += i + 1
			tagStr := input[i:end]
			sanitizedTag := s.processTag(tagStr, cfg)
			result.WriteString(sanitizedTag)
			i = end
		} else {
			nextTag := strings.IndexByte(input[i:], '<')
			var textChunk string
			if nextTag == -1 {
				textChunk = input[i:]
				i = len(input)
			} else {
				textChunk = input[i : i+nextTag]
				i += nextTag
			}
			result.WriteString(s.escapeSpecialChars(textChunk, cfg))
		}
	}
	return result.String()
}

func (s *HTMLSanitizer) processTag(tagStr string, cfg *SanitizerConfig) string {
	isClosing := false
	tagContent := tagStr[1 : len(tagStr)-1]

	if len(tagContent) > 0 && tagContent[0] == '/' {
		isClosing = true
		tagContent = tagContent[1:]
	}

	tagContent = strings.TrimSpace(tagContent)
	if tagContent == "" {
		return ""
	}

	parts := strings.Fields(tagContent)
	tagName := strings.ToLower(parts[0])
	tagName = strings.TrimSuffix(tagName, "/")

	if !cfg.AllowedTags[tagName] {
		return ""
	}

	var result strings.Builder
	result.WriteByte('<')
	if isClosing {
		result.WriteByte('/')
	}
	result.WriteString(tagName)

	if !isClosing && len(parts) > 1 {
		attrStr := strings.Join(parts[1:], " ")
		processedAttrs := s.processAttributes(attrStr, cfg)
		if processedAttrs != "" {
			result.WriteByte(' ')
			result.WriteString(processedAttrs)
		}
	}

	if strings.HasSuffix(tagStr, "/>") || isSelfClosingTag(tagName) {
		if !isClosing {
			result.WriteString(" /")
		}
	}
	result.WriteByte('>')

	return result.String()
}

func (s *HTMLSanitizer) processAttributes(attrStr string, cfg *SanitizerConfig) string {
	var attrs []string
	re := regexp.MustCompile(`(\w+)\s*=\s*("([^"]*)"|'([^']*)'|(\S+))`)
	matches := re.FindAllStringSubmatch(attrStr, -1)

	urlAttrs := map[string]bool{
		"href":   true,
		"src":    true,
		"action": true,
		"data":   true,
		"formaction": true,
	}

	safeProtocols := map[string]bool{
		"http:":  true,
		"https:": true,
		"mailto:": true,
		"tel:":    true,
		"ftp:":    true,
		"sftp:":   true,
		"#":      true,
		"/":      true,
		"./":     true,
		"../":    true,
	}

	for _, match := range matches {
		attrName := strings.ToLower(match[1])
		if !cfg.AllowedAttributes[attrName] {
			continue
		}

		var attrValue string
		if match[3] != "" {
			attrValue = match[3]
		} else if match[4] != "" {
			attrValue = match[4]
		} else {
			attrValue = match[5]
		}

		if urlAttrs[attrName] {
			trimmed := strings.TrimSpace(strings.ToLower(attrValue))
			isSafe := false
			colonIdx := strings.Index(trimmed, ":")
			if colonIdx == -1 {
				isSafe = true
			} else {
				for prefix := range safeProtocols {
					if strings.HasPrefix(trimmed, prefix) {
						isSafe = true
						break
					}
				}
			}
			if !isSafe {
				continue
			}
		}

		attrValue = html.EscapeString(attrValue)
		attrs = append(attrs, fmt.Sprintf("%s=\"%s\"", attrName, attrValue))
	}

	return strings.Join(attrs, " ")
}

func isSelfClosingTag(tag string) bool {
	selfClosing := map[string]bool{
		"br":    true,
		"hr":    true,
		"img":   true,
		"input": true,
		"meta":  true,
		"link":  true,
	}
	return selfClosing[tag]
}

func (s *HTMLSanitizer) escapeSpecialChars(input string, cfg *SanitizerConfig) string {
	if len(cfg.EscapeCharacters) == 0 {
		return input
	}

	var result strings.Builder
	for _, r := range input {
		if replacement, ok := cfg.EscapeCharacters[r]; ok {
			result.WriteString(replacement)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ==================== Output Encoder ====================

type OutputContext int

const (
	ContextHTML OutputContext = iota
	ContextJavaScript
	ContextURL
	ContextCSS
)

type OutputEncoder struct{}

func NewOutputEncoder() *OutputEncoder {
	return &OutputEncoder{}
}

func (e *OutputEncoder) Encode(input string, ctx OutputContext) string {
	switch ctx {
	case ContextHTML:
		return e.EncodeHTML(input)
	case ContextJavaScript:
		return e.EncodeJavaScript(input)
	case ContextURL:
		return e.EncodeURL(input)
	case ContextCSS:
		return e.EncodeCSS(input)
	default:
		return input
	}
}

func (e *OutputEncoder) EncodeHTML(input string) string {
	return html.EscapeString(input)
}

func (e *OutputEncoder) EncodeJavaScript(input string) string {
	var result strings.Builder
	for _, r := range input {
		switch r {
		case '\\':
			result.WriteString(`\\`)
		case '"':
			result.WriteString(`\"`)
		case '\'':
			result.WriteString(`\'`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		case '\b':
			result.WriteString(`\b`)
		case '\f':
			result.WriteString(`\f`)
		case '<':
			result.WriteString(`\u003c`)
		case '>':
			result.WriteString(`\u003e`)
		case '&':
			result.WriteString(`\u0026`)
		default:
			if r < 32 || r == 0x2028 || r == 0x2029 {
				result.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				result.WriteRune(r)
			}
		}
	}
	return result.String()
}

func (e *OutputEncoder) EncodeURL(input string) string {
	return url.QueryEscape(input)
}

func (e *OutputEncoder) EncodeCSS(input string) string {
	var result strings.Builder
	for _, r := range input {
		switch {
		case r == '\\':
			result.WriteString(`\\`)
		case r == '"':
			result.WriteString(`\"`)
		case r == '\'':
			result.WriteString(`\'`)
		case r == '\n':
			result.WriteString(`\A `)
		case r == '\r':
			result.WriteString(`\D `)
		case r == '\t':
			result.WriteString(`\9 `)
		case r == '<':
			result.WriteString(`\3C `)
		case r == '>':
			result.WriteString(`\3E `)
		case r == '&':
			result.WriteString(`\26 `)
		case r < 32 || r == 0x7f:
			result.WriteString(fmt.Sprintf(`\%X `, r))
		default:
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ==================== Data Masking ====================

type MaskPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	MaskFunc    func(string) string
	Description string
}

type DataMasker struct {
	mu       sync.RWMutex
	patterns map[string]*MaskPattern
}

func NewDataMasker() *DataMasker {
	m := &DataMasker{
		patterns: make(map[string]*MaskPattern),
	}
	m.registerBuiltinPatterns()
	return m
}

func (m *DataMasker) registerBuiltinPatterns() {
	idCardPattern := regexp.MustCompile(`\b[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`)
	m.patterns["id_card"] = &MaskPattern{
		Name:        "id_card",
		Pattern:     idCardPattern,
		Description: "Chinese ID card number (18 digits)",
		MaskFunc: func(s string) string {
			if len(s) < 8 {
				return s
			}
			return s[:6] + "********" + s[len(s)-4:]
		},
	}

	phonePattern := regexp.MustCompile(`\b1[3-9]\d{9}\b`)
	m.patterns["phone"] = &MaskPattern{
		Name:        "phone",
		Pattern:     phonePattern,
		Description: "Chinese mobile phone number",
		MaskFunc: func(s string) string {
			if len(s) < 7 {
				return s
			}
			return s[:3] + "****" + s[len(s)-4:]
		},
	}

	bankCardPattern := regexp.MustCompile(`\b\d{16,19}\b`)
	m.patterns["bank_card"] = &MaskPattern{
		Name:        "bank_card",
		Pattern:     bankCardPattern,
		Description: "Bank card number (16-19 digits)",
		MaskFunc: func(s string) string {
			if len(s) < 8 {
				return s
			}
			return s[:4] + " **** **** " + s[len(s)-4:]
		},
	}

	emailPattern := regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	m.patterns["email"] = &MaskPattern{
		Name:        "email",
		Pattern:     emailPattern,
		Description: "Email address",
		MaskFunc: func(s string) string {
			atIndex := strings.Index(s, "@")
			if atIndex <= 1 {
				return s
			}
			username := s[:atIndex]
			domain := s[atIndex:]
			if len(username) <= 2 {
				return username[:1] + "*" + domain
			}
			return username[:2] + strings.Repeat("*", len(username)-2) + domain
		},
	}
}

func (m *DataMasker) RegisterPattern(name, pattern string, maskFunc func(string) string, description string) error {
	if name == "" {
		return errors.New("contentsec: pattern name cannot be empty")
	}
	if pattern == "" {
		return errors.New("contentsec: pattern cannot be empty")
	}
	if maskFunc == nil {
		return ErrNilMaskPattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidPattern, err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.patterns[name]; exists {
		return ErrMaskPatternExists
	}

	m.patterns[name] = &MaskPattern{
		Name:        name,
		Pattern:     re,
		MaskFunc:    maskFunc,
		Description: description,
	}
	return nil
}

func (m *DataMasker) UnregisterPattern(name string) error {
	if name == "" {
		return errors.New("contentsec: pattern name cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.patterns[name]; !exists {
		return ErrMaskPatternNotFound
	}

	delete(m.patterns, name)
	return nil
}

func (m *DataMasker) Mask(input string) string {
	if input == "" {
		return ""
	}

	m.mu.RLock()
	patterns := make([]*MaskPattern, 0, len(m.patterns))
	for _, p := range m.patterns {
		patterns = append(patterns, p)
	}
	sort.Slice(patterns, func(i, j int) bool {
		lenI := len(patterns[i].Pattern.String())
		lenJ := len(patterns[j].Pattern.String())
		if lenI != lenJ {
			return lenI > lenJ
		}
		return patterns[i].Name < patterns[j].Name
	})
	m.mu.RUnlock()

	result := input
	for _, p := range patterns {
		result = p.Pattern.ReplaceAllStringFunc(result, p.MaskFunc)
	}
	return result
}

func (m *DataMasker) MaskWithPatterns(input string, patternNames []string) (string, error) {
	if input == "" {
		return "", nil
	}

	m.mu.RLock()
	var patterns []*MaskPattern
	for _, name := range patternNames {
		p, exists := m.patterns[name]
		if !exists {
			m.mu.RUnlock()
			return "", fmt.Errorf("%w: %s", ErrMaskPatternNotFound, name)
		}
		patterns = append(patterns, p)
	}
	m.mu.RUnlock()

	result := input
	for _, p := range patterns {
		result = p.Pattern.ReplaceAllStringFunc(result, p.MaskFunc)
	}
	return result, nil
}

func (m *DataMasker) PatternCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.patterns)
}

func (m *DataMasker) HasPattern(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.patterns[name]
	return exists
}

// ==================== Content Security Engine ====================

type ContentSecurityEngine struct {
	XSSDetector *XSSDetector
	Sanitizer   *HTMLSanitizer
	Encoder     *OutputEncoder
	Masker      *DataMasker
}

func NewContentSecurityEngine() *ContentSecurityEngine {
	return &ContentSecurityEngine{
		XSSDetector: NewXSSDetector(),
		Sanitizer:   NewHTMLSanitizer(),
		Encoder:     NewOutputEncoder(),
		Masker:      NewDataMasker(),
	}
}

func (e *ContentSecurityEngine) CheckAndSanitize(input string) (string, []XSSViolation) {
	violations := e.XSSDetector.Detect(input)
	sanitized := e.Sanitizer.Sanitize(input)
	return sanitized, violations
}

func (e *ContentSecurityEngine) SecureOutput(input string, ctx OutputContext) string {
	return e.Encoder.Encode(input, ctx)
}

func (e *ContentSecurityEngine) MaskSensitiveData(input string) string {
	return e.Masker.Mask(input)
}
