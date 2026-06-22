package hotconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "hotconfig_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})
	return dir
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	now := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatalf("failed to touch file %s: %v", path, err)
	}
}

func TestNewHotConfig_EmptyPath(t *testing.T) {
	_, err := NewHotConfig("", nil, nil)
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
	if !errors.Is(err, ErrInvalidConfigPath) {
		t.Fatalf("expected ErrInvalidConfigPath, got %v", err)
	}
}

func TestNewHotConfig_ValidPath(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name":"test"}`)

	hc, err := NewHotConfig(path, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hc == nil {
		t.Fatal("expected non-nil HotConfig")
	}
	if hc.Path() == "" {
		t.Error("expected non-empty path")
	}
}

func TestParser_JSON(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name":"test","port":8080,"debug":true}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, ok := hc.GetString("name")
	if !ok || name != "test" {
		t.Errorf("expected name='test', got %v (ok=%v)", name, ok)
	}

	port, ok := hc.GetInt("port")
	if !ok || port != 8080 {
		t.Errorf("expected port=8080, got %v (ok=%v)", port, ok)
	}

	debug, ok := hc.GetBool("debug")
	if !ok || !debug {
		t.Errorf("expected debug=true, got %v (ok=%v)", debug, ok)
	}
}

func TestParser_YAML(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.yaml")
	content := `
name: test_yaml
port: 9090
nested:
  key: value
list:
  - 1
  - 2
  - 3
`
	writeFile(t, path, content)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, ok := hc.GetString("name")
	if !ok || name != "test_yaml" {
		t.Errorf("expected name='test_yaml', got %v", name)
	}

	key, ok := hc.GetString("nested.key")
	if !ok || key != "value" {
		t.Errorf("expected nested.key='value', got %v (ok=%v)", key, ok)
	}

	snap := hc.GetSnapshot()
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	if snap.Format != FormatYAML {
		t.Errorf("expected format YAML, got %s", snap.Format)
	}
}

func TestParser_YML(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.yml")
	writeFile(t, path, "name: test_yml")

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, ok := hc.GetString("name")
	if !ok || name != "test_yml" {
		t.Errorf("expected name='test_yml', got %v", name)
	}
}

func TestParser_TOML(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.toml")
	content := `
name = "test_toml"
port = 8888

[database]
host = "localhost"
port = 5432
`
	writeFile(t, path, content)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, ok := hc.GetString("name")
	if !ok || name != "test_toml" {
		t.Errorf("expected name='test_toml', got %v", name)
	}

	host, ok := hc.GetString("database.host")
	if !ok || host != "localhost" {
		t.Errorf("expected database.host='localhost', got %v", host)
	}

	dbPort, ok := hc.GetInt("database.port")
	if !ok || dbPort != 5432 {
		t.Errorf("expected database.port=5432, got %v", dbPort)
	}
}

func TestParser_UnsupportedFormat(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.xml")
	writeFile(t, path, "<name>test</name>")

	hc, _ := NewHotConfig(path, nil, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestParser_InvalidJSON(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name": "test", invalid}`)

	hc, _ := NewHotConfig(path, nil, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected parse error for invalid JSON")
	}
}

func TestParser_FileNotFound(t *testing.T) {
	hc, _ := NewHotConfig("/nonexistent/path/config.json", nil, nil)
	err := hc.Load()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !errors.Is(err, ErrFileNotFound) {
		t.Fatalf("expected ErrFileNotFound, got %v", err)
	}
}

func TestValidation_RequiredField(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 8080}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:     "name",
				Required: true,
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected validation error for missing required field")
	}
	aggErr, ok := err.(*AggregateValidationError)
	if !ok {
		t.Fatalf("expected AggregateValidationError, got %T", err)
	}
	if len(aggErr.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(aggErr.Errors))
	}
	if !errors.Is(aggErr.Errors[0].Err, ErrFieldRequired) {
		t.Errorf("expected ErrFieldRequired, got %v", aggErr.Errors[0].Err)
	}
}

func TestValidation_MinValue(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 10}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "port",
				Rules: []*ValidationRule{
					{
						Type:     RuleMinValue,
						MinValue: 1024,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected min value validation error")
	}
}

func TestValidation_MaxValue(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 70000}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "port",
				Rules: []*ValidationRule{
					{
						Type:     RuleMaxValue,
						MaxValue: 65535,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected max value validation error")
	}
}

func TestValidation_MinLength(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name": "ab"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "name",
				Rules: []*ValidationRule{
					{
						Type:   RuleMinLength,
						MinLen: 5,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected min length validation error")
	}
}

func TestValidation_MaxLength(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name": "abcdefghij"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "name",
				Rules: []*ValidationRule{
					{
						Type:   RuleMaxLength,
						MaxLen: 5,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected max length validation error")
	}
}

func TestValidation_Pattern(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"email": "invalid-email"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "email",
				Rules: []*ValidationRule{
					{
						Type:    RulePattern,
						Pattern: `^[\w.+-]+@[\w-]+\.[\w.-]+$`,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected pattern validation error")
	}
}

func TestValidation_PatternValid(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"email": "test@example.com"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "email",
				Rules: []*ValidationRule{
					{
						Type:    RulePattern,
						Pattern: `^[\w.+-]+@[\w-]+\.[\w.-]+$`,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_Enum(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"env": "invalid"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "env",
				Rules: []*ValidationRule{
					{
						Type: RuleEnum,
						Enum: []interface{}{"dev", "test", "prod"},
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected enum validation error")
	}
}

func TestValidation_EnumValid(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"env": "prod"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "env",
				Rules: []*ValidationRule{
					{
						Type: RuleEnum,
						Enum: []interface{}{"dev", "test", "prod"},
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_Custom(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"value": 5}`)

	customErr := errors.New("must be even")
	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "value",
				Rules: []*ValidationRule{
					{
						Type: RuleCustom,
						Custom: func(v interface{}) error {
							if n, ok := v.(float64); ok && int(n)%2 == 0 {
								return nil
							}
							return customErr
						},
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError: true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected custom validation error")
	}
}

func TestValidation_NumericRange(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 8080}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "port",
				Rules: []*ValidationRule{
					{
						Type:     RuleMinValue,
						MinValue: 1024,
					},
					{
						Type:     RuleMaxValue,
						MaxValue: 65535,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDefaults_MissingField(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:         "name",
				DefaultValue: "default_name",
			},
			{
				Path:         "port",
				DefaultValue: 9090,
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, ok := hc.GetString("name")
	if !ok || name != "default_name" {
		t.Errorf("expected default name, got %v (ok=%v)", name, ok)
	}

	port, ok := hc.GetInt("port")
	if !ok || port != 9090 {
		t.Errorf("expected default port, got %v (ok=%v)", port, ok)
	}
}

func TestDefaults_OverrideByConfig(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name":"custom","port":8080}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:         "name",
				DefaultValue: "default_name",
			},
			{
				Path:         "port",
				DefaultValue: 9090,
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	name, _ := hc.GetString("name")
	if name != "custom" {
		t.Errorf("expected custom name, got %s", name)
	}

	port, _ := hc.GetInt("port")
	if port != 8080 {
		t.Errorf("expected custom port, got %d", port)
	}
}

func TestDefaults_FallbackOnValidationFailure(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 10}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:         "port",
				DefaultValue: 8080,
				Rules: []*ValidationRule{
					{
						Type:     RuleMinValue,
						MinValue: 1024,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError:        false,
		UseDefaultOnError:  true,
	})
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	port, ok := hc.GetInt("port")
	if !ok || port != 8080 {
		t.Errorf("expected fallback to default port 8080, got %v (ok=%v)", port, ok)
	}
}

func TestDefaults_NoFallbackOnValidationFailure(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"port": 10}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:         "port",
				DefaultValue: 8080,
				Rules: []*ValidationRule{
					{
						Type:     RuleMinValue,
						MinValue: 1024,
					},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{
		FailOnError:        true,
		UseDefaultOnError:  true,
	})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected validation error with FailOnError=true")
	}
}

func TestDefaults_NestedField(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path:         "database.host",
				DefaultValue: "localhost",
			},
			{
				Path:         "database.port",
				DefaultValue: 5432,
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	host, ok := hc.GetString("database.host")
	if !ok || host != "localhost" {
		t.Errorf("expected default db host, got %v", host)
	}

	port, ok := hc.GetInt("database.port")
	if !ok || port != 5432 {
		t.Errorf("expected default db port, got %v", port)
	}
}

func TestGetSnapshot(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"value"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	snap := hc.GetSnapshot()
	if snap == nil {
		t.Fatal("expected snapshot")
	}
	if snap.Version != 1 {
		t.Errorf("expected version 1, got %d", snap.Version)
	}
	if snap.Source == "" {
		t.Error("expected non-empty source")
	}
	if snap.Data["key"] != "value" {
		t.Errorf("expected key=value in snapshot data")
	}

	snap2 := hc.GetSnapshot()
	snap2.Data["key"] = "modified"
	original, _ := hc.GetString("key")
	if original != "value" {
		t.Error("snapshot modification should not affect original")
	}
}

func TestGet_Nonexistent(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"value"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, ok := hc.Get("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent key")
	}

	_, ok = hc.GetString("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent string key")
	}

	_, ok = hc.GetInt("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent int key")
	}

	_, ok = hc.GetFloat64("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent float key")
	}

	_, ok = hc.GetBool("nonexistent")
	if ok {
		t.Error("expected ok=false for nonexistent bool key")
	}
}

func TestGetFloat64(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"ratio": 0.85, "count": 42}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ratio, ok := hc.GetFloat64("ratio")
	if !ok {
		t.Fatal("expected ok=true for ratio")
	}
	if ratio != 0.85 {
		t.Errorf("expected 0.85, got %f", ratio)
	}

	count, ok := hc.GetFloat64("count")
	if !ok {
		t.Fatal("expected ok=true for count")
	}
	if count != 42.0 {
		t.Errorf("expected 42.0, got %f", count)
	}
}

func TestGet_TypeMismatch(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name":"test","port":"not_a_number"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, ok := hc.GetInt("name")
	if ok {
		t.Error("expected ok=false when getting string as int")
	}

	_, ok = hc.GetInt("port")
	if ok {
		t.Error("expected ok=false when getting non-numeric string as int")
	}

	_, ok = hc.GetBool("name")
	if ok {
		t.Error("expected ok=false when getting string as bool")
	}
}

func TestCallback_RegisterAndUnregister(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"value"}`)

	hc, _ := NewHotConfig(path, nil, nil)

	id1, err := hc.RegisterCallback(func(old, new *ConfigSnapshot) {})
	if err != nil {
		t.Fatalf("RegisterCallback failed: %v", err)
	}
	if id1 == "" {
		t.Error("expected non-empty callback id")
	}
	if hc.CallbackCount() != 1 {
		t.Errorf("expected 1 callback, got %d", hc.CallbackCount())
	}

	id2, _ := hc.RegisterCallback(func(old, new *ConfigSnapshot) {})
	if hc.CallbackCount() != 2 {
		t.Errorf("expected 2 callbacks, got %d", hc.CallbackCount())
	}

	if hc.UnregisterCallback(id1) != true {
		t.Error("expected UnregisterCallback to return true")
	}
	if hc.CallbackCount() != 1 {
		t.Errorf("expected 1 callback after unregister, got %d", hc.CallbackCount())
	}

	if hc.UnregisterCallback("nonexistent") != false {
		t.Error("expected false for nonexistent callback id")
	}

	if hc.UnregisterCallback(id2) != true {
		t.Error("expected UnregisterCallback to return true for id2")
	}
	if hc.CallbackCount() != 0 {
		t.Errorf("expected 0 callbacks, got %d", hc.CallbackCount())
	}
}

func TestCallback_NilCallback(t *testing.T) {
	hc, _ := NewHotConfig("config.json", nil, nil)
	_, err := hc.RegisterCallback(nil)
	if err == nil {
		t.Fatal("expected error for nil callback")
	}
	if !errors.Is(err, ErrNilCallback) {
		t.Errorf("expected ErrNilCallback, got %v", err)
	}
}

func TestStartAndStop(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"value"}`)

	hc, _ := NewHotConfig(path, nil, nil)

	if hc.IsRunning() {
		t.Error("should not be running initially")
	}

	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !hc.IsRunning() {
		t.Error("should be running after Start")
	}

	err := hc.Start()
	if err == nil {
		t.Fatal("expected error on double Start")
	}
	if !errors.Is(err, ErrWatcherAlreadyRunning) {
		t.Errorf("expected ErrWatcherAlreadyRunning, got %v", err)
	}

	hc.Stop()
	if hc.IsRunning() {
		t.Error("should not be running after Stop")
	}

	hc.Stop()
}

func TestStart_LoadsInitialConfig(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"initial"}`)

	hc, _ := NewHotConfig(path, nil, nil)

	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer hc.Stop()

	key, ok := hc.GetString("key")
	if !ok || key != "initial" {
		t.Errorf("expected initial config loaded, got %v", key)
	}
}

func TestReload_WithoutWatcher(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	v1, _ := hc.GetString("key")
	if v1 != "v1" {
		t.Errorf("expected v1, got %s", v1)
	}

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	v2, _ := hc.GetString("key")
	if v2 != "v2" {
		t.Errorf("expected v2 after reload, got %s", v2)
	}
	if hc.Version() != 2 {
		t.Errorf("expected version 2, got %d", hc.Version())
	}
}

func TestReload_TriggersCallback(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var mu sync.Mutex
	var callbackCalled bool
	var oldKey, newKey string

	_, err := hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		if old != nil {
			oldKey, _ = old.Data["key"].(string)
		}
		if new != nil {
			newKey, _ = new.Data["key"].(string)
		}
	})
	if err != nil {
		t.Fatalf("RegisterCallback failed: %v", err)
	}

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	mu.Lock()
	called := callbackCalled
	mu.Unlock()

	if !called {
		t.Fatal("expected callback to be called")
	}
	if oldKey != "v1" {
		t.Errorf("expected oldKey=v1, got %s", oldKey)
	}
	if newKey != "v2" {
		t.Errorf("expected newKey=v2, got %s", newKey)
	}
}

func TestReload_NoChange(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	callbackCalled := false
	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		callbackCalled = true
	})

	v1 := hc.Version()

	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if hc.Version() != v1 {
		t.Errorf("expected version to stay %d, got %d", v1, hc.Version())
	}
	if callbackCalled {
		t.Error("callback should not be called when no change")
	}
}

func TestFileWatcher_ChangeDetection(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, &HotConfigOptions{
		DebounceTime: 20 * time.Millisecond,
	})
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer hc.Stop()

	var mu sync.Mutex
	var callbackCalled bool

	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		mu.Lock()
		callbackCalled = true
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	writeFile(t, path, `{"key":"v2"}`)
	touchFile(t, path)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		called := callbackCalled
		mu.Unlock()
		if called {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	called := callbackCalled
	mu.Unlock()

	if !called {
		key, _ := hc.GetString("key")
		t.Fatalf("expected callback to be called, key=%s", key)
	}
}

func TestFileWatcher_Debounce(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, &HotConfigOptions{
		DebounceTime: 100 * time.Millisecond,
	})
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer hc.Stop()

	var mu sync.Mutex
	callCount := 0

	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		mu.Lock()
		callCount++
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		writeFile(t, path, fmt.Sprintf(`{"key":"v%d"}`, i+2))
		touchFile(t, path)
		time.Sleep(20 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	calls := callCount
	mu.Unlock()

	if calls > 2 {
		t.Errorf("debounce should merge rapid changes, got %d calls", calls)
	}
}

func TestCallback_PanicRecovery(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		panic("intentional panic in callback")
	})

	var secondCalled bool
	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		secondCalled = true
	})

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if !secondCalled {
		t.Error("second callback should still be called after first panics")
	}
}

func TestValidation_EmptyStringRequired(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name": ""}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "name",
				Rules: []*ValidationRule{
					{Type: RuleRequired},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{FailOnError: true})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected validation error for empty string with required rule")
	}
}

func TestValidation_ValidRequired(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"name": "test"}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "name",
				Rules: []*ValidationRule{
					{Type: RuleRequired},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidation_NilSchema(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"anything": "goes"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("nil schema should not cause error: %v", err)
	}
}

func TestValidation_EmptySchema(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"anything": "goes"}`)

	hc, _ := NewHotConfig(path, &Schema{}, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("empty schema should not cause error: %v", err)
	}
}

func TestApplyDefaults_NoMutation(t *testing.T) {
	original := map[string]interface{}{
		"a": "original",
	}

	schema := &Schema{
		Fields: []*FieldSchema{
			{Path: "b", DefaultValue: "default_b"},
		},
	}

	result := ApplyDefaults(original, schema)

	if result["a"] != "original" {
		t.Error("original field should be preserved")
	}
	if result["b"] != "default_b" {
		t.Error("default should be applied")
	}
	if original["b"] != nil {
		t.Error("original map should not be mutated")
	}
}

func TestGetParser_CaseInsensitive(t *testing.T) {
	tests := []struct {
		path    string
		format  ConfigFormat
		wantErr bool
	}{
		{"config.JSON", FormatJSON, false},
		{"config.Json", FormatJSON, false},
		{"config.YAML", FormatYAML, false},
		{"config.YML", FormatYAML, false},
		{"config.TOML", FormatTOML, false},
		{"config.txt", "", true},
	}

	for _, tt := range tests {
		_, format, err := GetParser(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("GetParser(%s) error=%v, wantErr=%v", tt.path, err, tt.wantErr)
		}
		if !tt.wantErr && format != tt.format {
			t.Errorf("GetParser(%s) format=%v, want=%v", tt.path, format, tt.format)
		}
	}
}

func TestDeepCopyMap(t *testing.T) {
	original := map[string]interface{}{
		"nested": map[string]interface{}{
			"key": "value",
		},
		"list": []interface{}{1, 2, 3},
		"simple": "value",
	}

	copy := deepCopyMap(original)

	nested, _ := copy["nested"].(map[string]interface{})
	nested["key"] = "modified"
	if original["nested"].(map[string]interface{})["key"] != "value" {
		t.Error("modifying nested copy should not affect original")
	}

	list, _ := copy["list"].([]interface{})
	list[0] = 999
	if original["list"].([]interface{})[0] != 1 {
		t.Error("modifying list copy should not affect original")
	}
}

func TestConfigSnapshot_StringMethod(t *testing.T) {
	_ = ConfigSnapshot{
		Data:      map[string]interface{}{},
		Timestamp: time.Now(),
		Source:    "/path/to/config.json",
		Format:    FormatJSON,
		Version:   42,
	}
}

func TestMultipleCallbacks_AllCalled(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var mu sync.Mutex
	called := make(map[string]bool)

	makeCallback := func(name string) ChangeCallback {
		return func(old, new *ConfigSnapshot) {
			mu.Lock()
			called[name] = true
			mu.Unlock()
		}
	}

	for i := 0; i < 3; i++ {
		cbName := fmt.Sprintf("cb%d", i)
		_, _ = hc.RegisterCallback(makeCallback(cbName))
	}

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	for i := 0; i < 3; i++ {
		cbName := fmt.Sprintf("cb%d", i)
		if !called[cbName] {
			t.Errorf("callback %s was not called", cbName)
		}
	}
}

func TestValidationError_ErrorString(t *testing.T) {
	ve := &ValidationError{
		Field:   "test_field",
		Message: "test message",
	}
	errStr := ve.Error()
	if errStr == "" {
		t.Error("ValidationError.Error() should not return empty string")
	}

	veWithCause := &ValidationError{
		Field:   "test",
		Message: "msg",
		Err:     errors.New("cause"),
	}
	if veWithCause.Error() == "" {
		t.Error("ValidationError.Error() with cause should not be empty")
	}
}

func TestAggregateValidationError_Empty(t *testing.T) {
	agg := &AggregateValidationError{Errors: nil}
	errStr := agg.Error()
	if errStr == "" {
		t.Error("empty AggregateValidationError should have error string")
	}
}

func TestParseError_ErrorString(t *testing.T) {
	pe := &ParseError{
		Format: "json",
		Path:   "/path/file.json",
		Err:    errors.New("syntax error"),
	}
	errStr := pe.Error()
	if errStr == "" {
		t.Error("ParseError.Error() should not be empty")
	}
	if pe.Unwrap() == nil {
		t.Error("ParseError.Unwrap() should return cause")
	}
}

func TestReload_WithWatcher(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer hc.Stop()

	v1 := hc.Version()

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if hc.Version() <= v1 {
		t.Errorf("expected version to increase from %d, got %d", v1, hc.Version())
	}
}

func TestGetWithoutLoad(t *testing.T) {
	hc, _ := NewHotConfig("/fake/path.json", nil, nil)

	_, ok := hc.Get("anything")
	if ok {
		t.Error("Get should return false when no config loaded")
	}

	snap := hc.GetSnapshot()
	if snap != nil {
		t.Error("GetSnapshot should return nil when no config loaded")
	}
}

func TestValidation_Pattern_TypeMismatch(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"code": 12345}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "code",
				Rules: []*ValidationRule{
					{Type: RulePattern, Pattern: `^\d+$`},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{FailOnError: true})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected error for pattern on non-string")
	}
}

func TestValidation_Length_TypeMismatch(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"num": 123}`)

	schema := &Schema{
		Fields: []*FieldSchema{
			{
				Path: "num",
				Rules: []*ValidationRule{
					{Type: RuleMinLength, MinLen: 1},
				},
			},
		},
	}

	hc, _ := NewHotConfig(path, schema, &HotConfigOptions{FailOnError: true})
	err := hc.Load()
	if err == nil {
		t.Fatal("expected error for length on non-sizable type")
	}
}

func TestGetParser_NoExtension(t *testing.T) {
	_, _, err := GetParser("/path/configfile")
	if err == nil {
		t.Fatal("expected error for file without extension")
	}
	if !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestDefaultHotConfigOptions(t *testing.T) {
	opts := DefaultHotConfigOptions()
	if opts == nil {
		t.Fatal("DefaultHotConfigOptions should not return nil")
	}
	if !opts.AutoReload {
		t.Error("AutoReload should default to true")
	}
	if opts.FailOnError {
		t.Error("FailOnError should default to false")
	}
	if !opts.UseDefaultOnError {
		t.Error("UseDefaultOnError should default to true")
	}
}

func TestCallback_SnapshotsAreIndependent(t *testing.T) {
	dir := tempDir(t)
	path := filepath.Join(dir, "config.json")
	writeFile(t, path, `{"key":"v1"}`)

	hc, _ := NewHotConfig(path, nil, nil)
	if err := hc.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	var capturedOld, capturedNew map[string]interface{}
	_, _ = hc.RegisterCallback(func(old, new *ConfigSnapshot) {
		if old != nil {
			capturedOld = old.Data
		}
		if new != nil {
			capturedNew = new.Data
		}
	})

	writeFile(t, path, `{"key":"v2"}`)
	if err := hc.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if capturedOld != nil {
		capturedOld["key"] = "tampered"
		current, _ := hc.GetString("key")
		if current != "v2" {
			t.Error("modifying captured old snapshot should not affect hotconfig")
		}
	}

	if capturedNew != nil {
		capturedNew["key"] = "modified"
		current, _ := hc.GetString("key")
		if current != "v2" {
			t.Error("modifying captured new snapshot should not affect hotconfig")
		}
	}
}
