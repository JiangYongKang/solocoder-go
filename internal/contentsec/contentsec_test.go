package contentsec

import (
	"strings"
	"sync"
	"testing"
)

// ==================== XSS Detector Tests ====================

func TestNewXSSDetector(t *testing.T) {
	d := NewXSSDetector()
	if d == nil {
		t.Fatal("NewXSSDetector returned nil")
	}
	if d.FilterCount() == 0 {
		t.Error("expected builtin filters to be registered")
	}
	if !d.HasFilter("script_tag") {
		t.Error("expected builtin script_tag filter to exist")
	}
	if !d.HasFilter("event_handler") {
		t.Error("expected builtin event_handler filter to exist")
	}
}

func TestXSSDetectScriptTag(t *testing.T) {
	d := NewXSSDetector()
	input := `<script>alert('xss')</script>Hello`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "script_tag" {
			found = true
			if v.Position != 0 {
				t.Errorf("expected position 0, got %d", v.Position)
			}
			break
		}
	}
	if !found {
		t.Error("expected script_tag violation")
	}
}

func TestXSSDetectEventHandler(t *testing.T) {
	d := NewXSSDetector()
	input := `<img src="test.jpg" onerror="alert('xss')">`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "event_handler" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected event_handler violation")
	}
}

func TestXSSDetectJavaScriptProtocol(t *testing.T) {
	d := NewXSSDetector()
	input := `<a href="javascript:alert('xss')">click</a>`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "javascript_protocol" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected javascript_protocol violation")
	}
}

func TestXSSDetectIframe(t *testing.T) {
	d := NewXSSDetector()
	input := `<iframe src="http://evil.com"></iframe>`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "iframe_tag" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected iframe_tag violation")
	}
}

func TestXSSDetectEval(t *testing.T) {
	d := NewXSSDetector()
	input := `eval("alert('xss')")`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "eval_expression" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected eval_expression violation")
	}
}

func TestXSSDetectVBScript(t *testing.T) {
	d := NewXSSDetector()
	input := `<a href="vbscript:msgbox('xss')">click</a>`
	violations := d.Detect(input)

	found := false
	for _, v := range violations {
		if v.Type == "vbscript_protocol" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected vbscript_protocol violation")
	}
}

func TestXSSDetectEmptyInput(t *testing.T) {
	d := NewXSSDetector()
	violations := d.Detect("")
	if violations != nil {
		t.Errorf("expected nil violations for empty input, got %v", violations)
	}
}

func TestXSSDetectSafeInput(t *testing.T) {
	d := NewXSSDetector()
	input := `<p>Hello <b>World</b></p>`
	violations := d.Detect(input)
	if len(violations) != 0 {
		t.Errorf("expected no violations for safe input, got %d", len(violations))
	}
}

func TestXSSRegisterFilter(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterFilter("custom", func(input string) []XSSViolation {
		if strings.Contains(input, "badword") {
			return []XSSViolation{{Position: 0, Type: "custom", Content: "badword"}}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("RegisterFilter failed: %v", err)
	}
	if !d.HasFilter("custom") {
		t.Error("custom filter should exist after registration")
	}

	violations := d.Detect("this has badword in it")
	found := false
	for _, v := range violations {
		if v.Type == "custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("custom filter should detect badword")
	}
}

func TestXSSRegisterFilterDuplicate(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterFilter("script_tag", func(input string) []XSSViolation {
		return nil
	})
	if err != ErrFilterExists {
		t.Errorf("expected ErrFilterExists, got %v", err)
	}
}

func TestXSSRegisterFilterEmptyName(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterFilter("", func(input string) []XSSViolation {
		return nil
	})
	if err == nil {
		t.Error("expected error for empty filter name")
	}
}

func TestXSSRegisterFilterNil(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterFilter("nil_filter", nil)
	if err != ErrNilFilter {
		t.Errorf("expected ErrNilFilter, got %v", err)
	}
}

func TestXSSUnregisterFilter(t *testing.T) {
	d := NewXSSDetector()
	err := d.UnregisterFilter("script_tag")
	if err != nil {
		t.Fatalf("UnregisterFilter failed: %v", err)
	}
	if d.HasFilter("script_tag") {
		t.Error("script_tag filter should not exist after unregister")
	}
}

func TestXSSUnregisterFilterNotFound(t *testing.T) {
	d := NewXSSDetector()
	err := d.UnregisterFilter("nonexistent")
	if err != ErrFilterNotFound {
		t.Errorf("expected ErrFilterNotFound, got %v", err)
	}
}

func TestXSSUnregisterFilterEmptyName(t *testing.T) {
	d := NewXSSDetector()
	err := d.UnregisterFilter("")
	if err == nil {
		t.Error("expected error for empty filter name")
	}
}

func TestXSSRegisterPatternFilter(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterPatternFilter("custom_pattern", `custom_\d+`, "custom_violation")
	if err != nil {
		t.Fatalf("RegisterPatternFilter failed: %v", err)
	}

	violations := d.Detect("test custom_123 end")
	found := false
	for _, v := range violations {
		if v.Type == "custom_violation" && v.Content == "custom_123" {
			found = true
			break
		}
	}
	if !found {
		t.Error("pattern filter should detect custom_123")
	}
}

func TestXSSRegisterPatternFilterInvalidPattern(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterPatternFilter("invalid", `[invalid`, "")
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestXSSRegisterPatternFilterEmptyName(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterPatternFilter("", `test`, "")
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestXSSRegisterPatternFilterEmptyPattern(t *testing.T) {
	d := NewXSSDetector()
	err := d.RegisterPatternFilter("test", "", "")
	if err == nil {
		t.Error("expected error for empty pattern")
	}
}

func TestXSSDetectMultipleViolations(t *testing.T) {
	d := NewXSSDetector()
	input := `<script>xss</script><img onerror="alert(1)">`
	violations := d.Detect(input)

	if len(violations) < 2 {
		t.Errorf("expected at least 2 violations, got %d", len(violations))
	}

	for i := 1; i < len(violations); i++ {
		if violations[i].Position < violations[i-1].Position {
			t.Error("violations should be sorted by position")
		}
	}
}

func TestXSSDetectorConcurrent(t *testing.T) {
	d := NewXSSDetector()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Detect(`<script>alert(1)</script>`)
		}()
	}
	wg.Wait()
}

// ==================== HTML Sanitizer Tests ====================

func TestNewHTMLSanitizer(t *testing.T) {
	s := NewHTMLSanitizer()
	if s == nil {
		t.Fatal("NewHTMLSanitizer returned nil")
	}
	cfg := s.GetConfig()
	if cfg == nil {
		t.Fatal("GetConfig returned nil")
	}
	if !cfg.AllowedTags["p"] {
		t.Error("p tag should be allowed by default")
	}
	if !cfg.AllowedAttributes["href"] {
		t.Error("href attribute should be allowed by default")
	}
}

func TestNewHTMLSanitizerWithConfig(t *testing.T) {
	cfg := &SanitizerConfig{
		AllowedTags:       map[string]bool{"b": true},
		AllowedAttributes: map[string]bool{"class": true},
		EscapeCharacters:  map[rune]string{'<': "&lt;"},
		StripComments:     true,
	}
	s, err := NewHTMLSanitizerWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewHTMLSanitizerWithConfig failed: %v", err)
	}
	if !s.GetConfig().AllowedTags["b"] {
		t.Error("b tag should be allowed")
	}
}

func TestNewHTMLSanitizerWithNilConfig(t *testing.T) {
	_, err := NewHTMLSanitizerWithConfig(nil)
	if err != ErrNilSanitizerRule {
		t.Errorf("expected ErrNilSanitizerRule, got %v", err)
	}
}

func TestSanitizeRemoveScriptTag(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p>Hello<script>alert('xss')</script>World</p>`
	result := s.Sanitize(input)
	if strings.Contains(result, "<script") {
		t.Errorf("script tag should be removed, got: %s", result)
	}
	if !strings.Contains(result, "Hello") {
		t.Errorf("text content should be preserved, got: %s", result)
	}
}

func TestSanitizeKeepAllowedTags(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p>Hello <b>World</b></p>`
	result := s.Sanitize(input)
	if !strings.Contains(result, "<p>") {
		t.Errorf("p tag should be preserved, got: %s", result)
	}
	if !strings.Contains(result, "<b>") {
		t.Errorf("b tag should be preserved, got: %s", result)
	}
}

func TestSanitizeRemoveDisallowedAttributes(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p onclick="alert(1)" class="test">Hello</p>`
	result := s.Sanitize(input)
	if strings.Contains(result, "onclick") {
		t.Errorf("onclick attribute should be removed, got: %s", result)
	}
	if !strings.Contains(result, "class=\"test\"") {
		t.Errorf("class attribute should be preserved, got: %s", result)
	}
}

func TestSanitizeEscapeSpecialChars(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p>a & b</p>`
	result := s.Sanitize(input)
	if !strings.Contains(result, "&amp;") {
		t.Errorf("& should be escaped, got: %s", result)
	}
}

func TestSanitizeStripComments(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p>Hello<!-- comment -->World</p>`
	result := s.Sanitize(input)
	if strings.Contains(result, "<!--") || strings.Contains(result, "-->") {
		t.Errorf("comments should be stripped, got: %s", result)
	}
	if !strings.Contains(result, "HelloWorld") {
		t.Errorf("text should be preserved, got: %s", result)
	}
}

func TestSanitizeEmptyInput(t *testing.T) {
	s := NewHTMLSanitizer()
	result := s.Sanitize("")
	if result != "" {
		t.Errorf("expected empty string, got: %s", result)
	}
}

func TestSanitizeSelfClosingTag(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<br><img src="test.jpg" alt="test">`
	result := s.Sanitize(input)
	if !strings.Contains(result, "<br") {
		t.Errorf("br tag should be preserved, got: %s", result)
	}
	if !strings.Contains(result, "<img") {
		t.Errorf("img tag should be preserved, got: %s", result)
	}
}

func TestSanitizeEscapeAttributeValues(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<a href="test&url">link</a>`
	result := s.Sanitize(input)
	if !strings.Contains(result, "test&amp;url") {
		t.Errorf("attribute value should be escaped, got: %s", result)
	}
}

func TestSanitizeAddRemoveAllowedTag(t *testing.T) {
	s := NewHTMLSanitizer()
	s.AddAllowedTag("custom")
	if !s.GetConfig().AllowedTags["custom"] {
		t.Error("custom tag should be allowed after adding")
	}

	s.RemoveAllowedTag("custom")
	if s.GetConfig().AllowedTags["custom"] {
		t.Error("custom tag should not be allowed after removal")
	}
}

func TestSanitizeAddRemoveAllowedAttribute(t *testing.T) {
	s := NewHTMLSanitizer()
	s.AddAllowedAttribute("data-test")
	if !s.GetConfig().AllowedAttributes["data-test"] {
		t.Error("data-test attribute should be allowed after adding")
	}

	s.RemoveAllowedAttribute("data-test")
	if s.GetConfig().AllowedAttributes["data-test"] {
		t.Error("data-test attribute should not be allowed after removal")
	}
}

func TestSanitizeSetConfig(t *testing.T) {
	s := NewHTMLSanitizer()
	newCfg := &SanitizerConfig{
		AllowedTags:       map[string]bool{"span": true},
		AllowedAttributes: map[string]bool{"id": true},
		EscapeCharacters:  map[rune]string{},
		StripComments:     false,
	}
	err := s.SetConfig(newCfg)
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}
	if !s.GetConfig().AllowedTags["span"] {
		t.Error("span tag should be allowed after SetConfig")
	}
}

func TestSanitizeSetNilConfig(t *testing.T) {
	s := NewHTMLSanitizer()
	err := s.SetConfig(nil)
	if err != ErrNilSanitizerRule {
		t.Errorf("expected ErrNilSanitizerRule, got %v", err)
	}
}

func TestSanitizeGetConfigReturnsCopy(t *testing.T) {
	s := NewHTMLSanitizer()
	cfg := s.GetConfig()
	cfg.AllowedTags["newtag"] = true
	if s.GetConfig().AllowedTags["newtag"] {
		t.Error("GetConfig should return a copy, modifying it should not affect sanitizer")
	}
}

func TestSanitizeUnclosedTag(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<p>Hello`
	result := s.Sanitize(input)
	if !strings.Contains(result, "Hello") {
		t.Errorf("text should be preserved, got: %s", result)
	}
}

func TestSanitizerConcurrent(t *testing.T) {
	s := NewHTMLSanitizer()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Sanitize(`<p>test<script>x</script></p>`)
		}()
	}
	wg.Wait()
}

// ==================== Output Encoder Tests ====================

func TestNewOutputEncoder(t *testing.T) {
	e := NewOutputEncoder()
	if e == nil {
		t.Fatal("NewOutputEncoder returned nil")
	}
}

func TestEncodeHTML(t *testing.T) {
	e := NewOutputEncoder()
	input := `<script>alert("xss") & 'test'</script>`
	result := e.EncodeHTML(input)
	expected := `&lt;script&gt;alert(&#34;xss&#34;) &amp; &#39;test&#39;&lt;/script&gt;`
	if result != expected {
		t.Errorf("HTML encoding failed:\n got: %s\nwant: %s", result, expected)
	}
}

func TestEncodeJavaScript(t *testing.T) {
	e := NewOutputEncoder()
	input := `alert("xss");\ntest`
	result := e.EncodeJavaScript(input)
	if !strings.Contains(result, `\"`) {
		t.Errorf("quotes should be escaped, got: %s", result)
	}
	if !strings.Contains(result, `\\`) {
		t.Errorf("backslashes should be escaped, got: %s", result)
	}
	if !strings.Contains(result, `\n`) {
		t.Errorf("newlines should be escaped, got: %s", result)
	}
}

func TestEncodeJavaScriptAngleBrackets(t *testing.T) {
	e := NewOutputEncoder()
	input := `<script>test</script>`
	result := e.EncodeJavaScript(input)
	if !strings.Contains(result, `\u003c`) {
		t.Errorf("< should be unicode escaped, got: %s", result)
	}
	if !strings.Contains(result, `\u003e`) {
		t.Errorf("> should be unicode escaped, got: %s", result)
	}
}

func TestEncodeJavaScriptControlChars(t *testing.T) {
	e := NewOutputEncoder()
	input := "\x00\x1f"
	result := e.EncodeJavaScript(input)
	if !strings.Contains(result, `\u0000`) {
		t.Errorf("null char should be escaped, got: %s", result)
	}
	if !strings.Contains(result, `\u001f`) {
		t.Errorf("control char should be escaped, got: %s", result)
	}
}

func TestEncodeURL(t *testing.T) {
	e := NewOutputEncoder()
	input := `hello world&test=value`
	result := e.EncodeURL(input)
	if !strings.Contains(result, "hello+world") && !strings.Contains(result, "hello%20world") {
		t.Errorf("spaces should be encoded, got: %s", result)
	}
	if !strings.Contains(result, "%26") {
		t.Errorf("& should be encoded, got: %s", result)
	}
}

func TestEncodeCSS(t *testing.T) {
	e := NewOutputEncoder()
	input := `content: "test";`
	result := e.EncodeCSS(input)
	if !strings.Contains(result, `\"`) {
		t.Errorf("quotes should be escaped in CSS, got: %s", result)
	}
}

func TestEncodeCSSNewlineTab(t *testing.T) {
	e := NewOutputEncoder()
	input := "\n\t"
	result := e.EncodeCSS(input)
	if !strings.Contains(result, `\A`) {
		t.Errorf("newline should be escaped in CSS, got: %s", result)
	}
	if !strings.Contains(result, `\9`) {
		t.Errorf("tab should be escaped in CSS, got: %s", result)
	}
}

func TestEncodeCSSAngleBrackets(t *testing.T) {
	e := NewOutputEncoder()
	input := "<test>"
	result := e.EncodeCSS(input)
	if !strings.Contains(result, `\3C`) {
		t.Errorf("< should be escaped in CSS, got: %s", result)
	}
	if !strings.Contains(result, `\3E`) {
		t.Errorf("> should be escaped in CSS, got: %s", result)
	}
}

func TestEncodeByContext(t *testing.T) {
	e := NewOutputEncoder()
	input := `<test>`

	htmlResult := e.Encode(input, ContextHTML)
	if htmlResult != e.EncodeHTML(input) {
		t.Error("ContextHTML should use EncodeHTML")
	}

	jsResult := e.Encode(input, ContextJavaScript)
	if jsResult != e.EncodeJavaScript(input) {
		t.Error("ContextJavaScript should use EncodeJavaScript")
	}

	urlResult := e.Encode(input, ContextURL)
	if urlResult != e.EncodeURL(input) {
		t.Error("ContextURL should use EncodeURL")
	}

	cssResult := e.Encode(input, ContextCSS)
	if cssResult != e.EncodeCSS(input) {
		t.Error("ContextCSS should use EncodeCSS")
	}

	unknownResult := e.Encode(input, OutputContext(999))
	if unknownResult != input {
		t.Error("unknown context should return input unchanged")
	}
}

func TestEncodeEmptyString(t *testing.T) {
	e := NewOutputEncoder()
	contexts := []OutputContext{ContextHTML, ContextJavaScript, ContextURL, ContextCSS}
	for _, ctx := range contexts {
		result := e.Encode("", ctx)
		if result != "" {
			t.Errorf("empty string should remain empty for context %d, got: %s", ctx, result)
		}
	}
}

func TestEncoderConcurrent(t *testing.T) {
	e := NewOutputEncoder()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.EncodeHTML("<test>")
			e.EncodeJavaScript("alert(1)")
			e.EncodeURL("test url")
			e.EncodeCSS("content: 'test'")
		}()
	}
	wg.Wait()
}

// ==================== Data Masker Tests ====================

func TestNewDataMasker(t *testing.T) {
	m := NewDataMasker()
	if m == nil {
		t.Fatal("NewDataMasker returned nil")
	}
	if m.PatternCount() < 4 {
		t.Errorf("expected at least 4 builtin patterns, got %d", m.PatternCount())
	}
	if !m.HasPattern("id_card") {
		t.Error("id_card pattern should exist")
	}
	if !m.HasPattern("phone") {
		t.Error("phone pattern should exist")
	}
	if !m.HasPattern("bank_card") {
		t.Error("bank_card pattern should exist")
	}
	if !m.HasPattern("email") {
		t.Error("email pattern should exist")
	}
}

func TestMaskIDCard(t *testing.T) {
	m := NewDataMasker()
	input := "身份证号: 110101199003076578"
	result := m.Mask(input)
	if !strings.Contains(result, "110101********6578") {
		t.Errorf("ID card should be masked, got: %s", result)
	}
	if strings.Contains(result, "19900307") {
		t.Errorf("ID card middle digits should be hidden, got: %s", result)
	}
}

func TestMaskPhone(t *testing.T) {
	m := NewDataMasker()
	input := "手机号: 13812345678"
	result := m.Mask(input)
	if !strings.Contains(result, "138****5678") {
		t.Errorf("phone should be masked, got: %s", result)
	}
}

func TestMaskBankCard(t *testing.T) {
	m := NewDataMasker()
	input := "银行卡号: 6222021234567890123"
	result := m.Mask(input)
	if !strings.Contains(result, "6222 **** **** 0123") {
		t.Errorf("bank card should be masked, got: %s", result)
	}
}

func TestMaskEmail(t *testing.T) {
	m := NewDataMasker()
	input := "邮箱: testuser@example.com"
	result := m.Mask(input)
	if !strings.Contains(result, "te******@example.com") {
		t.Errorf("email should be masked, got: %s", result)
	}
}

func TestMaskEmailShortUsername(t *testing.T) {
	m := NewDataMasker()
	input := "ab@example.com"
	result := m.Mask(input)
	if !strings.Contains(result, "a*@example.com") {
		t.Errorf("short email should be masked, got: %s", result)
	}
}

func TestMaskMultipleSensitiveData(t *testing.T) {
	m := NewDataMasker()
	input := "联系电话: 13812345678, 邮箱: test@example.com, 身份证: 110101199003076578"
	result := m.Mask(input)

	if strings.Contains(result, "13812345678") {
		t.Error("phone should be masked")
	}
	if strings.Contains(result, "test@example.com") {
		t.Error("email should be masked")
	}
	if strings.Contains(result, "19900307") {
		t.Error("ID card should be masked")
	}
}

func TestMaskNoSensitiveData(t *testing.T) {
	m := NewDataMasker()
	input := "Hello World, this is safe text."
	result := m.Mask(input)
	if result != input {
		t.Errorf("text without sensitive data should not change, got: %s", result)
	}
}

func TestMaskEmptyInput(t *testing.T) {
	m := NewDataMasker()
	result := m.Mask("")
	if result != "" {
		t.Errorf("empty input should return empty, got: %s", result)
	}
}

func TestMaskRegisterPattern(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern(
		"ssn",
		`\b\d{3}-\d{2}-\d{4}\b`,
		func(s string) string {
			return "***-**-" + s[len(s)-4:]
		},
		"US Social Security Number",
	)
	if err != nil {
		t.Fatalf("RegisterPattern failed: %v", err)
	}
	if !m.HasPattern("ssn") {
		t.Error("ssn pattern should exist after registration")
	}

	input := "SSN: 123-45-6789"
	result := m.Mask(input)
	if !strings.Contains(result, "***-**-6789") {
		t.Errorf("SSN should be masked, got: %s", result)
	}
}

func TestMaskRegisterPatternDuplicate(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern("phone", `\d+`, func(s string) string { return s }, "duplicate")
	if err != ErrMaskPatternExists {
		t.Errorf("expected ErrMaskPatternExists, got %v", err)
	}
}

func TestMaskRegisterPatternEmptyName(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern("", `\d+`, func(s string) string { return s }, "")
	if err == nil {
		t.Error("expected error for empty pattern name")
	}
}

func TestMaskRegisterPatternEmptyPattern(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern("test", "", func(s string) string { return s }, "")
	if err == nil {
		t.Error("expected error for empty pattern")
	}
}

func TestMaskRegisterPatternNilFunc(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern("test", `\d+`, nil, "")
	if err != ErrNilMaskPattern {
		t.Errorf("expected ErrNilMaskPattern, got %v", err)
	}
}

func TestMaskRegisterPatternInvalidRegex(t *testing.T) {
	m := NewDataMasker()
	err := m.RegisterPattern("test", `[invalid`, func(s string) string { return s }, "")
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestMaskUnregisterPattern(t *testing.T) {
	m := NewDataMasker()
	err := m.UnregisterPattern("email")
	if err != nil {
		t.Fatalf("UnregisterPattern failed: %v", err)
	}
	if m.HasPattern("email") {
		t.Error("email pattern should not exist after unregister")
	}

	input := "test@example.com"
	result := m.Mask(input)
	if result != input {
		t.Errorf("after unregister, email should not be masked, got: %s", result)
	}
}

func TestMaskUnregisterPatternNotFound(t *testing.T) {
	m := NewDataMasker()
	err := m.UnregisterPattern("nonexistent")
	if err != ErrMaskPatternNotFound {
		t.Errorf("expected ErrMaskPatternNotFound, got %v", err)
	}
}

func TestMaskUnregisterPatternEmptyName(t *testing.T) {
	m := NewDataMasker()
	err := m.UnregisterPattern("")
	if err == nil {
		t.Error("expected error for empty pattern name")
	}
}

func TestMaskWithPatterns(t *testing.T) {
	m := NewDataMasker()
	input := "phone: 13812345678, email: test@example.com"

	result, err := m.MaskWithPatterns(input, []string{"phone"})
	if err != nil {
		t.Fatalf("MaskWithPatterns failed: %v", err)
	}
	if strings.Contains(result, "13812345678") {
		t.Error("phone should be masked")
	}
	if !strings.Contains(result, "test@example.com") {
		t.Error("email should NOT be masked when only phone pattern is used")
	}
}

func TestMaskWithPatternsNotFound(t *testing.T) {
	m := NewDataMasker()
	_, err := m.MaskWithPatterns("test", []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent pattern")
	}
}

func TestMaskWithPatternsEmptyInput(t *testing.T) {
	m := NewDataMasker()
	result, err := m.MaskWithPatterns("", []string{"phone"})
	if err != nil {
		t.Fatalf("MaskWithPatterns empty input should not error, got: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty result, got: %s", result)
	}
}

func TestMaskerConcurrent(t *testing.T) {
	m := NewDataMasker()
	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Mask("13812345678 test@example.com")
		}()
	}
	wg.Wait()
}

// ==================== Content Security Engine Tests ====================

func TestNewContentSecurityEngine(t *testing.T) {
	engine := NewContentSecurityEngine()
	if engine == nil {
		t.Fatal("NewContentSecurityEngine returned nil")
	}
	if engine.XSSDetector == nil {
		t.Error("XSSDetector should not be nil")
	}
	if engine.Sanitizer == nil {
		t.Error("Sanitizer should not be nil")
	}
	if engine.Encoder == nil {
		t.Error("Encoder should not be nil")
	}
	if engine.Masker == nil {
		t.Error("Masker should not be nil")
	}
}

func TestCheckAndSanitize(t *testing.T) {
	engine := NewContentSecurityEngine()
	input := `<p>Hello<script>alert(1)</script>World</p>`
	sanitized, violations := engine.CheckAndSanitize(input)

	if len(violations) == 0 {
		t.Error("expected at least one XSS violation")
	}
	if strings.Contains(sanitized, "<script") {
		t.Errorf("script tag should be sanitized, got: %s", sanitized)
	}
	if !strings.Contains(sanitized, "Hello") {
		t.Errorf("text content should be preserved, got: %s", sanitized)
	}
}

func TestSecureOutput(t *testing.T) {
	engine := NewContentSecurityEngine()
	input := `<test>`
	result := engine.SecureOutput(input, ContextHTML)
	if result != `&lt;test&gt;` {
		t.Errorf("SecureOutput HTML encoding failed, got: %s", result)
	}
}

func TestMaskSensitiveData(t *testing.T) {
	engine := NewContentSecurityEngine()
	input := "电话: 13812345678"
	result := engine.MaskSensitiveData(input)
	if strings.Contains(result, "13812345678") {
		t.Errorf("phone should be masked, got: %s", result)
	}
}

func TestEngineFullWorkflow(t *testing.T) {
	engine := NewContentSecurityEngine()

	userInput := `<p onclick="alert(1)">联系我: 13812345678 或 test@example.com</p>`

	sanitized, violations := engine.CheckAndSanitize(userInput)
	if len(violations) == 0 {
		t.Error("should detect XSS violations")
	}

	masked := engine.MaskSensitiveData(sanitized)
	if strings.Contains(masked, "13812345678") {
		t.Error("phone should be masked in final output")
	}
	if strings.Contains(masked, "test@example.com") {
		t.Error("email should be masked in final output")
	}

	htmlEncoded := engine.SecureOutput(masked, ContextHTML)
	if strings.Contains(htmlEncoded, "<p") {
		t.Error("HTML tags should be encoded for final HTML output")
	}
}

// ==================== Regression Tests for Bug Fixes ====================

func TestEncodeCSSAngleBracketsSingleBackslash(t *testing.T) {
	e := NewOutputEncoder()
	input := "<test>&"
	result := e.EncodeCSS(input)

	if strings.Contains(result, `\\3C`) {
		t.Errorf("< should use single backslash, got double backslash: %s", result)
	}
	if strings.Contains(result, `\\3E`) {
		t.Errorf("> should use single backslash, got double backslash: %s", result)
	}
	if strings.Contains(result, `\\26`) {
		t.Errorf("& should use single backslash, got double backslash: %s", result)
	}

	if !strings.Contains(result, `\3C`) {
		t.Errorf("< should be encoded as \\3C, got: %s", result)
	}
	if !strings.Contains(result, `\3E`) {
		t.Errorf("> should be encoded as \\3E, got: %s", result)
	}
	if !strings.Contains(result, `\26`) {
		t.Errorf("& should be encoded as \\26, got: %s", result)
	}
}

func TestEncodeCSSConsistentBackslash(t *testing.T) {
	e := NewOutputEncoder()
	input := "\n<>"
	result := e.EncodeCSS(input)

	if strings.Count(result, `\A`) != strings.Count(result, `\3C`) {
		t.Errorf("newline and < should use same backslash style, got: %s", result)
	}
	if strings.Count(result, `\A`) != strings.Count(result, `\3E`) {
		t.Errorf("newline and > should use same backslash style, got: %s", result)
	}
}

func TestSanitizeSelfClosingTagNoSpace(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []string{"<br/>", "<br />", "<hr/>", "<hr />", "<img/>", "<img />"}
	for _, input := range tests {
		result := s.Sanitize(input)
		if result == "" {
			t.Errorf("self-closing tag %q should not be discarded, got empty string", input)
		}
		if !strings.Contains(result, "br") && !strings.Contains(result, "hr") && !strings.Contains(result, "img") {
			t.Errorf("self-closing tag %q should preserve tag name, got: %s", input, result)
		}
	}
}

func TestSanitizeBrTagPreservesNewlineSemantic(t *testing.T) {
	s := NewHTMLSanitizer()
	input := "line1<br/>line2<br />line3"
	result := s.Sanitize(input)

	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") || !strings.Contains(result, "line3") {
		t.Errorf("text content should be preserved, got: %s", result)
	}
	if strings.Count(result, "<br") != 2 {
		t.Errorf("both <br/> and <br /> should be preserved, got: %s", result)
	}
	if strings.Contains(result, "br/") {
		t.Errorf("tag name should not include trailing slash, got: %s", result)
	}
}

func TestSanitizeSelfClosingTagWithAttributes(t *testing.T) {
	s := NewHTMLSanitizer()
	input := `<img src="test.jpg" alt="test"/>`
	result := s.Sanitize(input)

	if !strings.Contains(result, `<img`) {
		t.Errorf("img tag should be preserved, got: %s", result)
	}
	if !strings.Contains(result, `src="test.jpg"`) {
		t.Errorf("src attribute should be preserved, got: %s", result)
	}
	if !strings.Contains(result, `alt="test"`) {
		t.Errorf("alt attribute should be preserved, got: %s", result)
	}
	if strings.Contains(result, "img/") {
		t.Errorf("tag name should not include trailing slash, got: %s", result)
	}
}

func TestDataMaskerConsistentOrder(t *testing.T) {
	m := NewDataMasker()

	idCard := "110101199003076578"
	inputs := []string{
		"身份证: " + idCard,
		"ID: " + idCard + " 电话: 13812345678",
		"多个证件: " + idCard + " 和 110101199003071234",
		"复杂混合: 手机 13812345678 邮箱 test@example.com 身份证 " + idCard + " 银行卡 6222021234567890123",
	}

	baselineResults := make([]string, len(inputs))
	for i, input := range inputs {
		baselineResults[i] = m.Mask(input)
	}

	for iter := 0; iter < 50; iter++ {
		for i, input := range inputs {
			result := m.Mask(input)
			if result != baselineResults[i] {
				t.Errorf("跨迭代一致性校验失败: 迭代 %d, 输入索引 %d\n基准: %s\n当前: %s",
					iter, i, baselineResults[i], result)
			}
			sameCall := m.Mask(input)
			if result != sameCall {
				t.Errorf("同迭代内两次调用不一致: 迭代 %d, 输入索引 %d\n首次: %s\n再次: %s",
					iter, i, result, sameCall)
			}
		}
	}
}

func TestDataMaskerIdCardNotTreatedAsBankCard(t *testing.T) {
	m := NewDataMasker()

	idCard := "110101199003076578"
	result := m.Mask(idCard)

	if strings.Contains(result, "**** ****") {
		t.Errorf("ID card should use ID mask format (********), not bank card format (**** ****), got: %s", result)
	}
	if !strings.Contains(result, "********") {
		t.Errorf("ID card should be masked with 8 asterisks in middle, got: %s", result)
	}
	if !strings.HasPrefix(result, "110101") {
		t.Errorf("ID card should keep first 6 digits, got: %s", result)
	}
	if !strings.HasSuffix(result, "6578") {
		t.Errorf("ID card should keep last 4 digits, got: %s", result)
	}
}

func TestSanitizeHrefJavaScriptProtocolBlocked(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		input    string
		contains string
		excludes string
	}{
		{`<a href="javascript:alert(1)">click</a>`, "<a>", "javascript"},
		{`<a href='javascript:alert(1)'>click</a>`, "<a>", "javascript"},
		{`<a href="  javascript:alert(1)">click</a>`, "<a>", "javascript"},
		{`<a href="JAVASCRIPT:alert(1)">click</a>`, "<a>", "javascript"},
		{`<a href="vbscript:msgbox(1)">click</a>`, "<a>", "vbscript"},
		{`<a href="data:text/html,alert(1)">click</a>`, "<a>", "data:text/html"},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		if tt.contains != "" && !strings.Contains(result, tt.contains) {
			t.Errorf("input %q: expected %q in result, got: %s", tt.input, tt.contains, result)
		}
		if tt.excludes != "" && strings.Contains(strings.ToLower(result), tt.excludes) {
			t.Errorf("input %q: expected no %q in result, got: %s", tt.input, tt.excludes, result)
		}
	}
}

func TestSanitizeHrefSafeProtocolsAllowed(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		input    string
		contains string
	}{
		{`<a href="http://example.com">link</a>`, `href="http://example.com"`},
		{`<a href="https://example.com">link</a>`, `href="https://example.com"`},
		{`<a href="mailto:user@example.com">email</a>`, `href="mailto:user@example.com"`},
		{`<a href="tel:13812345678">call</a>`, `href="tel:13812345678"`},
		{`<a href="/path/to/page">link</a>`, `href="/path/to/page"`},
		{`<a href="./relative/path">link</a>`, `href="./relative/path"`},
		{`<a href="../parent/path">link</a>`, `href="../parent/path"`},
		{`<a href="#section">anchor</a>`, `href="#section"`},
		{`<img src="https://example.com/image.jpg" alt="test">`, `src="https://example.com/image.jpg"`},
	}

	for _, tt := range tests {
		result := s.Sanitize(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("input %q: expected %q in result, got: %s", tt.input, tt.contains, result)
		}
	}
}

func TestSanitizeSrcJavaScriptBlocked(t *testing.T) {
	s := NewHTMLSanitizer()

	input := `<img src="javascript:alert(1)" alt="test">`
	result := s.Sanitize(input)

	if strings.Contains(result, "javascript") {
		t.Errorf("javascript protocol in src should be blocked, got: %s", result)
	}
	if strings.Contains(result, "src=") {
		t.Errorf("src attribute with javascript should be removed, got: %s", result)
	}
	if !strings.Contains(result, `<img`) {
		t.Errorf("img tag should be preserved even if src is removed, got: %s", result)
	}
}

func TestSanitizeDataURIImageAllowed(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name:     "PNG image data URI",
			input:    `<img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" alt="test">`,
			contains: `src="data:image/png;base64,`,
		},
		{
			name:     "JPEG image data URI",
			input:    `<img src="data:image/jpeg;base64,/9j/4AAQSkZJRgABAQEAAAAAAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/9oACAEBAAA/APn+iiiv/9k=" alt="photo">`,
			contains: `src="data:image/jpeg;base64,`,
		},
		{
			name:     "JPG image data URI",
			input:    `<img src="data:image/jpg;base64,/9j/4AAQSkZJRg==" alt="photo">`,
			contains: `src="data:image/jpg;base64,`,
		},
		{
			name:     "GIF image data URI",
			input:    `<img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7" alt="gif">`,
			contains: `src="data:image/gif;base64,`,
		},
		{
			name:     "WebP image data URI",
			input:    `<img src="data:image/webp;base64,UklGRkoAAABXRUJQVlA4WAoAAAAQAAAAAAAAAAAAQUxQSAwAAAABBxAR/Q9ERP8DAABWUDggGAAAADABAJ0BKgEAAQADADQlpAADcAD++/1QAA==" alt="webp">`,
			contains: `src="data:image/webp;base64,`,
		},
		{
			name:     "BMP image data URI",
			input:    `<img src="data:image/bmp;base64,Qk16AAAAAAAAAHsAAABsAAAAAQAAAAEAAAABAAYAAAAAAACAgIAAgICAgAEBAQABAQEAAAAAAAD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wD///8A////AP///wd///8A==" alt="bmp">`,
			contains: `src="data:image/bmp;base64,`,
		},
		{
			name:     "ICO image data URI",
			input:    `<img src="data:image/ico;base64,AAABAA==" alt="icon">`,
			contains: `src="data:image/ico;base64,`,
		},
		{
			name:     "X-ICON image data URI",
			input:    `<img src="data:image/x-icon;base64,AAABAA==" alt="icon">`,
			contains: `src="data:image/x-icon;base64,`,
		},
		{
			name:     "TIFF image data URI",
			input:    `<img src="data:image/tiff;base64,SUkqAA==" alt="tiff">`,
			contains: `src="data:image/tiff;base64,`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("%s: expected data URI to be preserved, missing %q in result: %s", tt.name, tt.contains, result)
			}
		})
	}
}

func TestSanitizeDataURIDangerousTypesBlocked(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		name     string
		input    string
		excludes string
	}{
		{
			name:     "data:text/html blocked",
			input:    `<a href="data:text/html,<script>alert(1)</script>">click</a>`,
			excludes: "data:text/html",
		},
		{
			name:     "data:text/javascript blocked",
			input:    `<a href="data:text/javascript,alert(1)">click</a>`,
			excludes: "data:text/javascript",
		},
		{
			name:     "data:application/javascript blocked",
			input:    `<a href="data:application/javascript,alert(1)">click</a>`,
			excludes: "data:application/javascript",
		},
		{
			name:     "data:application/x-shockwave-flash blocked",
			input:    `<embed src="data:application/x-shockwave-flash,base64,abc">`,
			excludes: "data:application/x-shockwave-flash",
		},
		{
			name:     "SVG XSS with onload blocked",
			input:    `<img src="data:image/svg+xml,<svg onload=alert(1)>" alt="xss">`,
			excludes: "data:image/svg+xml",
		},
		{
			name:     "SVG XSS with script tag blocked",
			input:    `<img src="data:image/svg+xml;base64,PHN2ZyBvbmxvYWQ9YWxlcnQoMSk+" alt="xss">`,
			excludes: "data:image/svg+xml",
		},
		{
			name:     "SVG without base64 blocked",
			input:    `<img src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg'/%3E" alt="svg">`,
			excludes: "data:image/svg+xml",
		},
		{
			name:     "data:text/css blocked",
			input:    `<link href="data:text/css,body{color:red}" rel="stylesheet">`,
			excludes: "data:text/css",
		},
		{
			name:     "data:application/json blocked",
			input:    `<a href="data:application/json,{\"key\":\"value\"}">data</a>`,
			excludes: "data:application/json",
		},
		{
			name:     "data:application/octet-stream blocked",
			input:    `<a href="data:application/octet-stream,abc">download</a>`,
			excludes: "data:application/octet-stream",
		},
		{
			name:     "data:application/vnd.ms-excel blocked",
			input:    `<a href="data:application/vnd.ms-excel,abc">excel</a>`,
			excludes: "data:application/vnd.ms-excel",
		},
		{
			name:     "data:font/woff blocked",
			input:    `<img src="data:font/woff;base64,d09GRgABAAAAAB..." alt="font">`,
			excludes: "data:font/woff",
		},
		{
			name:     "data:audio/mpeg blocked",
			input:    `<img src="data:audio/mpeg;base64,SUQzBAAAAAAAI1..." alt="audio">`,
			excludes: "data:audio/mpeg",
		},
		{
			name:     "data:video/mp4 blocked",
			input:    `<img src="data:video/mp4;base64,AAAAIGZ0eXBpc29tAAACAGlzb21pc28yYXZjMW1wNDEAAAAIZm..." alt="video">`,
			excludes: "data:video/mp4",
		},
		{
			name:     "data:application/pdf blocked",
			input:    `<a href="data:application/pdf;base64,JVBERi0xLjQKJcOkw7zDts..." target="_blank">下载PDF</a>`,
			excludes: "data:application/pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if strings.Contains(result, tt.excludes) {
				t.Errorf("%s: dangerous data URI should be blocked, but found %q in result: %s", tt.name, tt.excludes, result)
			}
		})
	}
}

func TestSanitizeDataURISVGScriptBlocked(t *testing.T) {
	s := NewHTMLSanitizer()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "SVG onload event handler",
			input: `<img src="data:image/svg+xml,<svg onload='alert(1)'>" alt="xss">`,
		},
		{
			name:  "SVG onclick event handler",
			input: `<img src='data:image/svg+xml,<svg onclick="alert(1)"/>' alt="xss">`,
		},
		{
			name:  "SVG with script tag",
			input: `<img src='data:image/svg+xml;base64,PHN2Zz48c2NyaXB0PmFsZXJ0KDEpPC9zY3JpcHQ+PC9zdmc+' alt='xss'>`,
		},
		{
			name:  "SVG base64 encoded script",
			input: `<img src="data:image/svg+xml;base64,PHN2Zz48c2NyaXB0PmFsZXJ0KDEpPC9zY3JpcHQ+PC9zdmc+" alt="xss">`,
		},
		{
			name:  "SVG with onmouseover",
			input: `<img src="data:image/svg+xml,%3Csvg%20onmouseover%3D%22alert%281%29%22%3E" alt="xss">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.Sanitize(tt.input)
			if strings.Contains(result, "image/svg+xml") {
				t.Errorf("%s: image/svg+xml data URI should be blocked, got: %s", tt.name, result)
			}
			if strings.Contains(result, "onload") || strings.Contains(result, "onclick") || strings.Contains(result, "onmouseover") {
				t.Errorf("%s: event handlers should not appear in output, got: %s", tt.name, result)
			}
		})
	}
}

func TestSanitizeDataURIRegressionImage(t *testing.T) {
	s := NewHTMLSanitizer()

	input := `<p>这是一张内联图片: <img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" alt="测试图"></p>`
	result := s.Sanitize(input)

	if !strings.Contains(result, "data:image/png") {
		t.Errorf("data URI 内联图片不应被拦截，这是回归缺陷。结果: %s", result)
	}
	if !strings.Contains(result, `alt="测试图"`) {
		t.Errorf("alt 属性应被保留，结果: %s", result)
	}
	if !strings.Contains(result, "<p>") || !strings.Contains(result, "</p>") {
		t.Errorf("p 标签应被保留，结果: %s", result)
	}
}

func TestEncodeJavaScriptForwardSlashNotEscaped(t *testing.T) {
	e := NewOutputEncoder()

	tests := []struct {
		input    string
		expected string
	}{
		{"/", "/"},
		{"http://example.com/path", "http://example.com/path"},
		{"https://example.com/path/to/page", "https://example.com/path/to/page"},
		{`/^test$/`, `/^test$/`},
		{`/\d+/g`, `/\\d+/g`},
		{"a/b/c", "a/b/c"},
	}

	for _, tt := range tests {
		result := e.EncodeJavaScript(tt.input)
		if strings.Contains(result, `\/`) {
			t.Errorf("input %q: forward slash should not be escaped, got: %s", tt.input, result)
		}
		if result != tt.expected {
			t.Errorf("input %q: expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestEncodeJavaScriptPreservesRegexAndUrls(t *testing.T) {
	e := NewOutputEncoder()

	url := "https://api.example.com/v1/users/123"
	result := e.EncodeJavaScript(url)

	if strings.Contains(result, `\/`) {
		t.Errorf("URL should not have escaped slashes, got: %s", result)
	}
	if result != url {
		t.Errorf("URL should be preserved exactly, expected %q, got %q", url, result)
	}

	regex := `/^[a-z0-9]+@[a-z0-9]+\.[a-z]+$/i`
	expectedRegex := `/^[a-z0-9]+@[a-z0-9]+\\.[a-z]+$/i`
	result2 := e.EncodeJavaScript(regex)
	if strings.Contains(result2, `\/`) {
		t.Errorf("regex should not have escaped slashes, got: %s", result2)
	}
	if result2 != expectedRegex {
		t.Errorf("regex backslashes should be escaped, expected %q, got %q", expectedRegex, result2)
	}
}

func TestEncodeJavaScriptStillEscapesQuotesAndBackslashes(t *testing.T) {
	e := NewOutputEncoder()

	input := `test"quote'and\backslash`
	result := e.EncodeJavaScript(input)

	if !strings.Contains(result, `\"`) {
		t.Errorf("double quote should still be escaped, got: %s", result)
	}
	if !strings.Contains(result, `\'`) {
		t.Errorf("single quote should still be escaped, got: %s", result)
	}
	if !strings.Contains(result, `\\`) {
		t.Errorf("backslash should still be escaped, got: %s", result)
	}
}

func TestEncodeCSSControlCharsSingleBackslash(t *testing.T) {
	e := NewOutputEncoder()
	input := "\x00\x7f"
	result := e.EncodeCSS(input)

	if strings.Contains(result, `\\0`) {
		t.Errorf("null char should use single backslash, got double backslash: %s", result)
	}
	if strings.Contains(result, `\\7F`) {
		t.Errorf("DEL char should use single backslash, got double backslash: %s", result)
	}
	if !strings.Contains(result, `\0`) {
		t.Errorf("null char should be encoded as \\0, got: %s", result)
	}
	if !strings.Contains(result, `\7F`) {
		t.Errorf("DEL char should be encoded as \\7F, got: %s", result)
	}
}
