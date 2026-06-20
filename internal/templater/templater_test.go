package templater

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	config := Config{StrictVariables: false}
	e := NewEngine(config)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.config.StrictVariables != false {
		t.Errorf("expected StrictVariables to be false, got true")
	}
}

func TestRegisterTemplate(t *testing.T) {
	e := NewEngine(Config{})

	err := e.RegisterTemplate("test", "Hello {{ .Name }}")
	if err != nil {
		t.Fatalf("RegisterTemplate failed: %v", err)
	}

	err = e.RegisterTemplate("", "empty name")
	if !errors.Is(err, ErrEmptyTemplateName) {
		t.Errorf("expected ErrEmptyTemplateName, got %v", err)
	}
}

func TestRenderVariable(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("hello", "Hello, {{ .Name }}!")

	data := map[string]interface{}{
		"Name": "World",
	}

	result, err := e.Render("hello", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hello, World!" {
		t.Errorf("expected 'Hello, World!', got '%s'", result)
	}
}

func TestRenderVariableNested(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("nested", "User: {{ .User.Name }}, Age: {{ .User.Age }}")

	data := map[string]interface{}{
		"User": map[string]interface{}{
			"Name": "Alice",
			"Age":  30,
		},
	}

	result, err := e.Render("nested", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "User: Alice, Age: 30" {
		t.Errorf("expected 'User: Alice, Age: 30', got '%s'", result)
	}
}

type TestUser struct {
	Name  string
	Email string
	Profile TestProfile
}

type TestProfile struct {
	City    string
	Country string
}

func TestRenderVariableStruct(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("struct", "{{ .Name }} from {{ .Profile.City }}, {{ .Profile.Country }}")

	user := TestUser{
		Name:  "Bob",
		Email: "bob@example.com",
		Profile: TestProfile{
			City:    "Beijing",
			Country: "China",
		},
	}

	result, err := e.Render("struct", user)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Bob from Beijing, China" {
		t.Errorf("expected 'Bob from Beijing, China', got '%s'", result)
	}
}

func TestRenderVariableNotFoundStrict(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("missing", "Hello {{ .MissingVar }}")

	_, err := e.Render("missing", map[string]interface{}{})
	if !errors.Is(err, ErrVariableNotFound) {
		t.Errorf("expected ErrVariableNotFound, got %v", err)
	}
}

func TestRenderVariableNotFoundNonStrict(t *testing.T) {
	e := NewEngine(Config{StrictVariables: false})
	e.RegisterTemplate("missing", "Hello{{ .MissingVar }} World")

	result, err := e.Render("missing", map[string]interface{}{})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hello World" {
		t.Errorf("expected 'Hello World', got '%s'", result)
	}
}

func TestRenderMultipleVariables(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("multi", "{{ .Greeting }}, {{ .Name }}! You are {{ .Age }} years old.")

	data := map[string]interface{}{
		"Greeting": "Hi",
		"Name":     "Charlie",
		"Age":      25,
	}

	result, err := e.Render("multi", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Hi, Charlie! You are 25 years old." {
		t.Errorf("unexpected result: '%s'", result)
	}
}

func TestRenderTemplateNotFound(t *testing.T) {
	e := NewEngine(Config{})

	_, err := e.Render("nonexistent", nil)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestRenderIfConditionEquals(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("ifeq", "{{ if .Role == \"admin\" }}Admin{{ else }}User{{ endif }}")

	data1 := map[string]interface{}{"Role": "admin"}
	result1, err := e.Render("ifeq", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "Admin" {
		t.Errorf("expected 'Admin', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Role": "user"}
	result2, err := e.Render("ifeq", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "User" {
		t.Errorf("expected 'User', got '%s'", result2)
	}
}

func TestRenderIfConditionNotEquals(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("ifne", "{{ if .Status != \"active\" }}Inactive{{ else }}Active{{ endif }}")

	data1 := map[string]interface{}{"Status": "banned"}
	result1, err := e.Render("ifne", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "Inactive" {
		t.Errorf("expected 'Inactive', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Status": "active"}
	result2, err := e.Render("ifne", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Active" {
		t.Errorf("expected 'Active', got '%s'", result2)
	}
}

func TestRenderIfEmpty(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("ifempty", "{{ if empty .Items }}No items{{ else }}Has items{{ endif }}")

	data1 := map[string]interface{}{"Items": []string{}}
	result1, err := e.Render("ifempty", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "No items" {
		t.Errorf("expected 'No items', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Items": []string{"a", "b"}}
	result2, err := e.Render("ifempty", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Has items" {
		t.Errorf("expected 'Has items', got '%s'", result2)
	}
}

func TestRenderIfTruthy(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("iftruthy", "{{ if .Enabled }}On{{ else }}Off{{ endif }}")

	data1 := map[string]interface{}{"Enabled": true}
	result1, err := e.Render("iftruthy", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "On" {
		t.Errorf("expected 'On', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Enabled": false}
	result2, err := e.Render("iftruthy", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Off" {
		t.Errorf("expected 'Off', got '%s'", result2)
	}
}

func TestRenderIfNoElse(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("ifnoelse", "Start{{ if .Show }} ShowMe{{ endif }} End")

	data1 := map[string]interface{}{"Show": true}
	result1, err := e.Render("ifnoelse", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "Start ShowMe End" {
		t.Errorf("expected 'Start ShowMe End', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Show": false}
	result2, err := e.Render("ifnoelse", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Start End" {
		t.Errorf("expected 'Start End', got '%s'", result2)
	}
}

func TestRenderRangeSimple(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangesimple", "{{ range $item := range .Items }}{{ $item }};{{ endrange }}")

	data := map[string]interface{}{
		"Items": []string{"a", "b", "c"},
	}

	result, err := e.Render("rangesimple", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "a;b;c;" {
		t.Errorf("expected 'a;b;c;', got '%s'", result)
	}
}

func TestRenderRangeWithIndex(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangeidx", "{{ range $i, $item := range .Items }}{{ $i }}:{{ $item }};{{ endrange }}")

	data := map[string]interface{}{
		"Items": []string{"x", "y", "z"},
	}

	result, err := e.Render("rangeidx", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "0:x;1:y;2:z;" {
		t.Errorf("expected '0:x;1:y;2:z;', got '%s'", result)
	}
}

func TestRenderRangeEmpty(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangeempty", "Items:{{ range $item := range .Items }}{{ $item }}{{ endrange }}")

	data := map[string]interface{}{
		"Items": []string{},
	}

	result, err := e.Render("rangeempty", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Items:" {
		t.Errorf("expected 'Items:', got '%s'", result)
	}
}

func TestRenderRangeNestedInIf(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	template := `{{ if .ShowItems }}{{ range $item := range .Items }}[{{ $item }}]{{ endrange }}{{ else }}No items{{ endif }}`
	e.RegisterTemplate("rangenested", template)

	data1 := map[string]interface{}{
		"ShowItems": true,
		"Items":     []int{1, 2, 3},
	}
	result1, err := e.Render("rangenested", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "[1][2][3]" {
		t.Errorf("expected '[1][2][3]', got '%s'", result1)
	}

	data2 := map[string]interface{}{
		"ShowItems": false,
		"Items":     []int{1, 2, 3},
	}
	result2, err := e.Render("rangenested", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "No items" {
		t.Errorf("expected 'No items', got '%s'", result2)
	}
}

func TestRenderIfNestedInRange(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	template := `{{ range $item := range .Items }}{{ if $item == 2 }}TWO{{ else }}{{ $item }}{{ endif }};{{ endrange }}`
	e.RegisterTemplate("ifnested", template)

	data := map[string]interface{}{
		"Items": []int{1, 2, 3},
	}

	result, err := e.Render("ifnested", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "1;TWO;3;" {
		t.Errorf("expected '1;TWO;3;', got '%s'", result)
	}
}

func TestRenderRangeIntSlice(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangeint", "{{ range $i, $n := range .Numbers }}{{ $i }}={{ $n }},{{ endrange }}")

	data := map[string]interface{}{
		"Numbers": []int{10, 20, 30},
	}

	result, err := e.Render("rangeint", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "0=10,1=20,2=30," {
		t.Errorf("expected '0=10,1=20,2=30,', got '%s'", result)
	}
}

func TestTemplateInheritance(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	parentTmpl := `<html><head>{{ block title }}Default Title{{ endblock }}</head><body>{{ block content }}Default Content{{ endblock }}</body></html>`
	e.RegisterTemplate("parent", parentTmpl)

	childTmpl := `{{ extends "parent" }}{{ block title }}My Page{{ endblock }}{{ block content }}Hello World{{ endblock }}`
	e.RegisterTemplate("child", childTmpl)

	result, err := e.Render("child", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	expected := `<html><head>My Page</head><body>Hello World</body></html>`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateInheritancePartialOverride(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	parentTmpl := `Header: {{ block header }}Default Header{{ endblock }} | Body: {{ block body }}Default Body{{ endblock }} | Footer: {{ block footer }}Default Footer{{ endblock }}`
	e.RegisterTemplate("parent2", parentTmpl)

	childTmpl := `{{ extends "parent2" }}{{ block body }}Custom Body{{ endblock }}`
	e.RegisterTemplate("child2", childTmpl)

	result, err := e.Render("child2", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	expected := `Header: Default Header | Body: Custom Body | Footer: Default Footer`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateInheritanceWithVariables(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	parentTmpl := `<div class="container">{{ block content }}Default: {{ .DefaultValue }}{{ endblock }}</div>`
	e.RegisterTemplate("parent3", parentTmpl)

	childTmpl := `{{ extends "parent3" }}{{ block content }}Custom: {{ .CustomValue }}{{ endblock }}`
	e.RegisterTemplate("child3", childTmpl)

	data := map[string]interface{}{
		"DefaultValue": "default",
		"CustomValue":  "custom",
	}
	result, err := e.Render("child3", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	expected := `<div class="container">Custom: custom</div>`
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestTemplateInheritanceParentNotFound(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	childTmpl := `{{ extends "nonexistent_parent" }}{{ block content }}Content{{ endblock }}`
	e.RegisterTemplate("orphan", childTmpl)

	_, err := e.Render("orphan", nil)
	if !errors.Is(err, ErrParentTemplateNotFound) {
		t.Errorf("expected ErrParentTemplateNotFound, got %v", err)
	}
}

func TestCustomFunction(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	upperFn := func(s string) string {
		return strings.ToUpper(s)
	}
	e.RegisterFunction("upper", upperFn)

	e.RegisterTemplate("fn", "Result: {{ upper .Name }}")

	data := map[string]interface{}{
		"Name": "hello",
	}

	result, err := e.Render("fn", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Result: HELLO" {
		t.Errorf("expected 'Result: HELLO', got '%s'", result)
	}
}

func TestCustomFunctionMultipleArgs(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	concatFn := func(a, b string) string {
		return a + b
	}
	e.RegisterFunction("concat", concatFn)

	e.RegisterTemplate("fn2", "Result: {{ concat .A .B }}")

	data := map[string]interface{}{
		"A": "foo",
		"B": "bar",
	}

	result, err := e.Render("fn2", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Result: foobar" {
		t.Errorf("expected 'Result: foobar', got '%s'", result)
	}
}

func TestCustomFunctionNotFound(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("fnnf", "{{ nonexistent .X }}")

	_, err := e.Render("fnnf", map[string]interface{}{"X": "test"})
	if !errors.Is(err, ErrFunctionNotFound) {
		t.Errorf("expected ErrFunctionNotFound, got %v", err)
	}
}

func TestCustomFunctionFormatTime(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	formatTime := func(t time.Time, layout string) string {
		return t.Format(layout)
	}
	e.RegisterFunction("formatTime", formatTime)

	e.RegisterTemplate("time", "Date: {{ formatTime .CreatedAt .Layout }}")

	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	data := map[string]interface{}{
		"CreatedAt": now,
		"Layout":    "2006-01-02",
	}

	result, err := e.Render("time", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Date: 2024-01-15" {
		t.Errorf("expected 'Date: 2024-01-15', got '%s'", result)
	}
}

func TestCustomFunctionWithError(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	divideFn := func(a, b int) (int, error) {
		if b == 0 {
			return 0, errors.New("division by zero")
		}
		return a / b, nil
	}
	e.RegisterFunction("divide", divideFn)

	e.RegisterTemplate("fndiv", "Result: {{ divide .A .B }}")

	data1 := map[string]interface{}{"A": 10, "B": 2}
	result1, err := e.Render("fndiv", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "Result: 5" {
		t.Errorf("expected 'Result: 5', got '%s'", result1)
	}

	data2 := map[string]interface{}{"A": 10, "B": 0}
	_, err = e.Render("fndiv", data2)
	if err == nil {
		t.Error("expected error for division by zero")
	}
}

func TestCache(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("cachetest", "Hello {{ .Name }}")

	_, err := e.Render("cachetest", map[string]interface{}{"Name": "First"})
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}

	tmpl, err := e.GetTemplate("cachetest")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}

	tmpl2, err := e.GetTemplate("cachetest")
	if err != nil {
		t.Fatalf("Second GetTemplate failed: %v", err)
	}

	if tmpl != tmpl2 {
		t.Error("expected cached template to be the same instance")
	}
}

func TestInvalidateCache(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("invalidate", "Version 1: {{ .Name }}")

	result1, err := e.Render("invalidate", map[string]interface{}{"Name": "Test"})
	if err != nil {
		t.Fatalf("First render failed: %v", err)
	}
	if result1 != "Version 1: Test" {
		t.Errorf("expected 'Version 1: Test', got '%s'", result1)
	}

	e.RegisterTemplate("invalidate", "Version 2: {{ .Name }}")

	result2, err := e.Render("invalidate", map[string]interface{}{"Name": "Test"})
	if err != nil {
		t.Fatalf("Second render failed: %v", err)
	}
	if result2 != "Version 2: Test" {
		t.Errorf("expected 'Version 2: Test', got '%s'", result2)
	}
}

func TestInvalidateCacheExplicit(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("invalid2", "V1")

	e.Render("invalid2", nil)

	err := e.InvalidateCache("invalid2")
	if err != nil {
		t.Fatalf("InvalidateCache failed: %v", err)
	}

	e.RegisterTemplate("invalid2", "V2")
	result, err := e.Render("invalid2", nil)
	if err != nil {
		t.Fatalf("Render after invalidate failed: %v", err)
	}
	if result != "V2" {
		t.Errorf("expected 'V2', got '%s'", result)
	}
}

func TestInvalidateCacheNotFound(t *testing.T) {
	e := NewEngine(Config{})
	err := e.InvalidateCache("nonexistent")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestInvalidateCacheEmptyName(t *testing.T) {
	e := NewEngine(Config{})
	err := e.InvalidateCache("")
	if !errors.Is(err, ErrEmptyTemplateName) {
		t.Errorf("expected ErrEmptyTemplateName, got %v", err)
	}
}

func TestClearCache(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("clear1", "A")
	e.RegisterTemplate("clear2", "B")

	e.Render("clear1", nil)
	e.Render("clear2", nil)

	e.ClearCache()

	e.RegisterTemplate("clear1", "A2")
	e.RegisterTemplate("clear2", "B2")

	result1, _ := e.Render("clear1", nil)
	result2, _ := e.Render("clear2", nil)

	if result1 != "A2" {
		t.Errorf("expected 'A2', got '%s'", result1)
	}
	if result2 != "B2" {
		t.Errorf("expected 'B2', got '%s'", result2)
	}
}

func TestTextOnlyTemplate(t *testing.T) {
	e := NewEngine(Config{})
	e.RegisterTemplate("text", "Just plain text with no variables")

	result, err := e.Render("text", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Just plain text with no variables" {
		t.Errorf("unexpected result: '%s'", result)
	}
}

func TestEmptyTemplate(t *testing.T) {
	e := NewEngine(Config{})
	e.RegisterTemplate("empty", "")

	result, err := e.Render("empty", nil)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty string, got '%s'", result)
	}
}

func TestVariableTypes(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("types", "Int: {{ .Int }}, Float: {{ .Float }}, Bool: {{ .Bool }}")

	data := map[string]interface{}{
		"Int":   42,
		"Float": 3.14,
		"Bool":  true,
	}

	result, err := e.Render("types", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	expected := "Int: 42, Float: 3.14, Bool: true"
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestComplexScenario(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	e.RegisterFunction("length", func(s []interface{}) int { return len(s) })

	parent := `<!DOCTYPE html><html><head>{{ block head }}<title>{{ .Title }}</title>{{ endblock }}</head><body>{{ block body }}{{ endblock }}</body></html>`
	e.RegisterTemplate("layout", parent)

	child := `{{ extends "layout" }}{{ block head }}<title>{{ .Title }} - MySite</title>{{ endblock }}{{ block body }}
<h1>{{ .Header }}</h1>
{{ if .ShowWelcome }}<p>Welcome, {{ .User.Name }}!</p>{{ endif }}
<ul>
{{ range $i, $item := range .Items }}
<li>{{ $i }}: {{ $item }}</li>
{{ endrange }}
</ul>
{{ if empty .Items }}<p>No items to display</p>{{ endif }}
{{ endblock }}`
	e.RegisterTemplate("page", child)

	data := map[string]interface{}{
		"Title":       "Home",
		"Header":      "Welcome Page",
		"ShowWelcome": true,
		"User": map[string]interface{}{
			"Name": "Alice",
		},
		"Items": []string{"Item One", "Item Two", "Item Three"},
	}

	result, err := e.Render("page", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if !strings.Contains(result, "<title>Home - MySite</title>") {
		t.Errorf("title not found in result: %s", result)
	}
	if !strings.Contains(result, "<h1>Welcome Page</h1>") {
		t.Errorf("header not found in result")
	}
	if !strings.Contains(result, "<p>Welcome, Alice!</p>") {
		t.Errorf("welcome message not found in result")
	}
	if !strings.Contains(result, "<li>0: Item One</li>") {
		t.Errorf("item 0 not found in result")
	}
	if !strings.Contains(result, "<li>1: Item Two</li>") {
		t.Errorf("item 1 not found in result")
	}
	if !strings.Contains(result, "<li>2: Item Three</li>") {
		t.Errorf("item 2 not found in result")
	}
}

func TestConcurrentRender(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("concurrent", "Hello, {{ .Name }}!")

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			data := map[string]interface{}{
				"Name": fmt.Sprintf("User%d", i),
			}
			result, err := e.Render("concurrent", data)
			if err != nil {
				t.Errorf("render failed: %v", err)
				return
			}
			expected := fmt.Sprintf("Hello, User%d!", i)
			if result != expected {
				t.Errorf("expected '%s', got '%s'", expected, result)
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentRegisterAndRender(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("tmpl%d", i)
			e.RegisterTemplate(name, "{{ .Value }}")
		}(i)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("tmpl%d", i)
			e.Render(name, map[string]interface{}{"Value": i})
		}(i)
	}
	wg.Wait()
}

func TestIfIntegerComparison(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("intcmp", "{{ if .Count == 0 }}Empty{{ else }}Has Items{{ endif }}")

	data1 := map[string]interface{}{"Count": 0}
	result1, err := e.Render("intcmp", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "Empty" {
		t.Errorf("expected 'Empty', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Count": 5}
	result2, err := e.Render("intcmp", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Has Items" {
		t.Errorf("expected 'Has Items', got '%s'", result2)
	}
}

func TestEmptyStringVariable(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("emptystr", "Value: '{{ .Text }}'")

	data := map[string]interface{}{"Text": ""}
	result, err := e.Render("emptystr", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Value: ''" {
		t.Errorf("expected 'Value: \\'\\'', got '%s'", result)
	}
}

func TestMapStringString(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("mapss", "Key: {{ .foo }}")

	data := map[string]string{"foo": "bar"}
	result, err := e.Render("mapss", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Key: bar" {
		t.Errorf("expected 'Key: bar', got '%s'", result)
	}
}

func TestPointerStruct(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("ptrstruct", "{{ .Name }} - {{ .Profile.City }}")

	user := &TestUser{
		Name: "Dave",
		Profile: TestProfile{
			City:    "Shanghai",
			Country: "China",
		},
	}

	result, err := e.Render("ptrstruct", user)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "Dave - Shanghai" {
		t.Errorf("expected 'Dave - Shanghai', got '%s'", result)
	}
}

func TestTemplateInheritanceLoop(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	e.RegisterTemplate("loopA", `{{ extends "loopB" }}{{ block content }}A{{ endblock }}`)
	e.RegisterTemplate("loopB", `{{ extends "loopA" }}{{ block content }}B{{ endblock }}`)

	_, err := e.Render("loopA", nil)
	if !errors.Is(err, ErrTemplateInheritanceLoop) {
		t.Errorf("expected ErrTemplateInheritanceLoop, got %v", err)
	}
}

func TestRangeNotIterable(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangenoiter", "{{ range $item := range .Value }}{{ $item }}{{ endrange }}")

	_, err := e.Render("rangenoiter", map[string]interface{}{"Value": 42})
	if !errors.Is(err, ErrRangeNotIterable) {
		t.Errorf("expected ErrRangeNotIterable, got %v", err)
	}
}

func TestRangeNilNotIterable(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangenil", "{{ range $item := range .Value }}{{ $item }}{{ endrange }}")

	_, err := e.Render("rangenil", map[string]interface{}{"Value": nil})
	if !errors.Is(err, ErrRangeNotIterable) {
		t.Errorf("expected ErrRangeNotIterable, got %v", err)
	}
}

func TestRangeStringNotIterable(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("rangestr", "{{ range $item := range .Value }}{{ $item }}{{ endrange }}")

	_, err := e.Render("rangestr", map[string]interface{}{"Value": "hello"})
	if !errors.Is(err, ErrRangeNotIterable) {
		t.Errorf("expected ErrRangeNotIterable, got %v", err)
	}
}

func TestUnclosedBlockTag(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("unclosedblock", "{{ block header }}Content without endblock")

	_, err := e.Render("unclosedblock", nil)
	if !errors.Is(err, ErrUnclosedBlock) {
		t.Errorf("expected ErrUnclosedBlock, got %v", err)
	}
}

func TestUnclosedIfTag(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("unclosedif", "{{ if .Show }}Content without endif")

	_, err := e.Render("unclosedif", nil)
	if !errors.Is(err, ErrUnclosedBlock) {
		t.Errorf("expected ErrUnclosedBlock, got %v", err)
	}
}

func TestUnclosedRangeTag(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("unclosedrange", "{{ range $item := range .Items }}Content without endrange")

	_, err := e.Render("unclosedrange", map[string]interface{}{"Items": []int{1}})
	if !errors.Is(err, ErrUnclosedBlock) {
		t.Errorf("expected ErrUnclosedBlock, got %v", err)
	}
}

func TestUnclosedVariableTag(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("unclosedvar", "Hello {{ .Name")

	_, err := e.Render("unclosedvar", map[string]interface{}{"Name": "World"})
	if !errors.Is(err, ErrUnclosedBlock) {
		t.Errorf("expected ErrUnclosedBlock, got %v", err)
	}
}

func TestInvalidRangeSyntax(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("badrange", "{{ range badsyntax }}{{ endrange }}")

	_, err := e.Render("badrange", nil)
	if !errors.Is(err, ErrInvalidRange) {
		t.Errorf("expected ErrInvalidRange, got %v", err)
	}
}

func TestInvalidBlockSyntax(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("badblock", "{{ block 123 }}content{{ endblock }}")

	_, err := e.Render("badblock", nil)
	if !errors.Is(err, ErrInvalidBlockSyntax) {
		t.Errorf("expected ErrInvalidBlockSyntax, got %v", err)
	}
}

func TestBlockNotFoundInInheritance(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	e.RegisterTemplate("parentbnf", "{{ block header }}Default{{ endblock }}")
	e.RegisterTemplate("childbnf", `{{ extends "parentbnf" }}{{ block nonexistent }}Override{{ endblock }}`)

	_, err := e.Render("childbnf", nil)
	if !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("expected ErrBlockNotFound, got %v", err)
	}
}

func TestElseIfSyntax(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("elseif", "{{ if .Level == 1 }}One{{ else if .Level == 2 }}Two{{ else }}Other{{ endif }}")

	data1 := map[string]interface{}{"Level": 1}
	result1, err := e.Render("elseif", data1)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result1 != "One" {
		t.Errorf("expected 'One', got '%s'", result1)
	}

	data2 := map[string]interface{}{"Level": 2}
	result2, err := e.Render("elseif", data2)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result2 != "Two" {
		t.Errorf("expected 'Two', got '%s'", result2)
	}

	data3 := map[string]interface{}{"Level": 3}
	result3, err := e.Render("elseif", data3)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result3 != "Other" {
		t.Errorf("expected 'Other', got '%s'", result3)
	}
}

func TestElseIfChained(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("elseifchain", "{{ if .X == 1 }}A{{ else if .X == 2 }}B{{ else if .X == 3 }}C{{ else }}D{{ endif }}")

	data := map[string]interface{}{"X": 3}
	result, err := e.Render("elseifchain", data)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "C" {
		t.Errorf("expected 'C', got '%s'", result)
	}
}

func TestFunctionNonErrorSecondReturn(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	badFn := func(a int) (int, bool) {
		return a, true
	}
	e.RegisterFunction("badfn", badFn)

	e.RegisterTemplate("badfn", "{{ badfn .Val }}")

	_, err := e.Render("badfn", map[string]interface{}{"Val": 5})
	if !errors.Is(err, ErrInvalidFunctionCall) {
		t.Errorf("expected ErrInvalidFunctionCall, got %v", err)
	}
}

func TestFunctionNonFuncRegistration(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	e.RegisterFunction("notafunc", 42)
	e.RegisterTemplate("notafn", "{{ notafunc }}")

	_, err := e.Render("notafn", nil)
	if !errors.Is(err, ErrInvalidFunctionCall) {
		t.Errorf("expected ErrInvalidFunctionCall, got %v", err)
	}
}

func TestInvalidCondition(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("badcond", "{{ if invalidexpr }}yes{{ endif }}")

	_, err := e.Render("badcond", nil)
	if !errors.Is(err, ErrInvalidCondition) {
		t.Errorf("expected ErrInvalidCondition, got %v", err)
	}
}

func TestInvalidVariablePath(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})
	e.RegisterTemplate("badpath", "{{ .Name. }}")

	_, err := e.Render("badpath", map[string]interface{}{"Name": "test"})
	if !errors.Is(err, ErrInvalidVariablePath) {
		t.Errorf("expected ErrInvalidVariablePath, got %v", err)
	}
}

func TestRegisterFunctionEmptyName(t *testing.T) {
	e := NewEngine(Config{})
	err := e.RegisterFunction("", func() {})
	if !errors.Is(err, ErrFunctionNotFound) {
		t.Errorf("expected ErrFunctionNotFound, got %v", err)
	}
}

func TestFunctionArgumentCountMismatch(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	addFn := func(a, b int) int { return a + b }
	e.RegisterFunction("add", addFn)

	e.RegisterTemplate("addtmpl", "{{ add .A }}")

	_, err := e.Render("addtmpl", map[string]interface{}{"A": 1})
	if !errors.Is(err, ErrInvalidArgumentCount) {
		t.Errorf("expected ErrInvalidArgumentCount, got %v", err)
	}
}

func TestGetTemplateRaceCondition(t *testing.T) {
	e := NewEngine(Config{StrictVariables: true})

	e.RegisterTemplate("race", "v1: {{ .Name }}")

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			e.RegisterTemplate("race", fmt.Sprintf("v%d: {{ .Name }}", i))
		}(i)
		go func() {
			defer wg.Done()
			e.GetTemplate("race")
		}()
	}
	wg.Wait()

	e.RegisterTemplate("race", "final: {{ .Name }}")
	result, err := e.Render("race", map[string]interface{}{"Name": "test"})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if result != "final: test" {
		t.Errorf("expected 'final: test', got '%s'", result)
	}
}
