package envmgr

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewEnvManager(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewEnvManager returned nil")
	}
	if len(mgr.aesKey) != aesKeySize {
		t.Errorf("expected AES key size %d, got %d", aesKeySize, len(mgr.aesKey))
	}
}

func TestNewEnvManagerWithKey(t *testing.T) {
	key := make([]byte, aesKeySize)
	for i := range key {
		key[i] = byte(i)
	}

	mgr, err := NewEnvManagerWithKey(key)
	if err != nil {
		t.Fatalf("NewEnvManagerWithKey failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("NewEnvManagerWithKey returned nil")
	}

	shortKey := make([]byte, 16)
	_, err = NewEnvManagerWithKey(shortKey)
	if err != ErrInvalidKeySize {
		t.Errorf("expected ErrInvalidKeySize, got %v", err)
	}
}

func mockEnvSource(envs []string) func() []string {
	return func() []string {
		return envs
	}
}

func TestLoadGroupWithPrefix(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
		"APP_PORT=8080",
		"APP_DEBUG=true",
		"DB_HOST=localhost",
		"DB_PORT=5432",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}
	if group.Prefix() != "APP_" {
		t.Errorf("expected prefix 'APP_', got '%s'", group.Prefix())
	}

	name, err := group.GetString("NAME")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if name != "myapp" {
		t.Errorf("expected 'myapp', got '%s'", name)
	}

	all := group.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 values, got %d", len(all))
	}
}

func TestLoadGroupWithoutPrefix(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"KEY1=value1",
		"KEY2=value2",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	val, err := group.GetString("KEY1")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got '%s'", val)
	}
}

func TestLoadGroupWithConfig(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
		"APP_PORT=8080",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		{Key: "PORT", Required: true},
		{Key: "DEBUG", Required: false, Default: "false"},
	}

	group, err := mgr.LoadGroup("APP_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	debug, err := group.GetString("DEBUG")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if debug != "false" {
		t.Errorf("expected default 'false', got '%s'", debug)
	}
}

func TestLoadGroupMissingRequired(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		{Key: "PORT", Required: true},
		{Key: "HOST", Required: true},
	}

	_, err = mgr.LoadGroup("APP_", configs...)
	if err == nil {
		t.Fatal("expected error for missing required variables")
	}

	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error should mention missing 'PORT', got: %v", err)
	}
	if !strings.Contains(err.Error(), "HOST") {
		t.Errorf("error should mention missing 'HOST', got: %v", err)
	}
}

func TestLoadGroupEmptyRequired(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
		"APP_PORT=  ",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		{Key: "PORT", Required: true},
	}

	_, err = mgr.LoadGroup("APP_", configs...)
	if err == nil {
		t.Fatal("expected error for empty required variable")
	}
	if !strings.Contains(err.Error(), "PORT") {
		t.Errorf("error should mention empty 'PORT', got: %v", err)
	}
}

func TestLoadGroupWithDefaultForEmpty(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
		"APP_PORT=  ",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		{Key: "PORT", Required: true, Default: "8080"},
	}

	group, err := mgr.LoadGroup("APP_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup should succeed with default: %v", err)
	}

	port, err := group.GetString("PORT")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if port != "8080" {
		t.Errorf("expected default '8080', got '%s'", port)
	}
}

func TestTypeConversionInt(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_PORT=8080",
		"APP_INVALID=notanumber",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	port, err := group.GetInt("PORT")
	if err != nil {
		t.Fatalf("GetInt failed: %v", err)
	}
	if port != 8080 {
		t.Errorf("expected 8080, got %d", port)
	}

	_, err = group.GetInt("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid int conversion")
	}
}

func TestTypeConversionInt64(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_BIG=9223372036854775807",
		"APP_INVALID=notanumber",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	big, err := group.GetInt64("BIG")
	if err != nil {
		t.Fatalf("GetInt64 failed: %v", err)
	}
	if big != 9223372036854775807 {
		t.Errorf("expected 9223372036854775807, got %d", big)
	}

	_, err = group.GetInt64("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid int64 conversion")
	}
}

func TestTypeConversionFloat64(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_PRICE=19.99",
		"APP_INVALID=notanumber",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	price, err := group.GetFloat64("PRICE")
	if err != nil {
		t.Fatalf("GetFloat64 failed: %v", err)
	}
	if price != 19.99 {
		t.Errorf("expected 19.99, got %f", price)
	}

	_, err = group.GetFloat64("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid float64 conversion")
	}
}

func TestTypeConversionBool(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_DEBUG=true",
		"APP_ENABLED=1",
		"APP_DISABLED=false",
		"APP_OFF=0",
		"APP_INVALID=notbool",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	testCases := []struct {
		key      string
		expected bool
	}{
		{"DEBUG", true},
		{"ENABLED", true},
		{"DISABLED", false},
		{"OFF", false},
	}

	for _, tc := range testCases {
		val, err := group.GetBool(tc.key)
		if err != nil {
			t.Fatalf("GetBool(%s) failed: %v", tc.key, err)
		}
		if val != tc.expected {
			t.Errorf("GetBool(%s): expected %v, got %v", tc.key, tc.expected, val)
		}
	}

	_, err = group.GetBool("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid bool conversion")
	}
}

func TestTypeConversionDuration(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_TIMEOUT=30s",
		"APP_INTERVAL=5m",
		"APP_INVALID=notaduration",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	timeout, err := group.GetDuration("TIMEOUT")
	if err != nil {
		t.Fatalf("GetDuration failed: %v", err)
	}
	if timeout != 30*time.Second {
		t.Errorf("expected 30s, got %v", timeout)
	}

	interval, err := group.GetDuration("INTERVAL")
	if err != nil {
		t.Fatalf("GetDuration failed: %v", err)
	}
	if interval != 5*time.Minute {
		t.Errorf("expected 5m, got %v", interval)
	}

	_, err = group.GetDuration("INVALID")
	if err == nil {
		t.Fatal("expected error for invalid duration conversion")
	}
}

func TestSensitiveValue(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"DB_PASSWORD=supersecret123",
		"DB_USER=admin",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "USER", Required: true},
		{Key: "PASSWORD", Required: true, Sensitive: true},
	}

	group, err := mgr.LoadGroup("DB_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	_, err = group.Get("PASSWORD")
	if err == nil {
		t.Fatal("expected error when directly reading sensitive value")
	}

	all := group.GetAll()
	if all["PASSWORD"] != "[ENCRYPTED]" {
		t.Errorf("expected '[ENCRYPTED]' for sensitive value, got '%s'", all["PASSWORD"])
	}

	sv, err := mgr.GetSensitive(group, "PASSWORD")
	if err != nil {
		t.Fatalf("GetSensitive failed: %v", err)
	}

	plaintext, err := sv.Decrypt()
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if plaintext != "supersecret123" {
		t.Errorf("expected 'supersecret123', got '%s'", plaintext)
	}
}

func TestGetSensitiveNonSensitiveKey(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"DB_USER=admin",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "USER", Required: true},
	}

	group, err := mgr.LoadGroup("DB_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	_, err = mgr.GetSensitive(group, "USER")
	if err == nil {
		t.Fatal("expected error for non-sensitive key")
	}
}

func TestGetSensitiveMissingKey(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"DB_USER=admin",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "PASSWORD", Required: false, Sensitive: true},
	}

	group, err := mgr.LoadGroup("DB_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	_, err = mgr.GetSensitive(group, "PASSWORD")
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestGetNonExistentKey(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	_, err = group.GetString("NONEXISTENT")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestGetWithDefault(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		{Key: "DEBUG", Required: false, Default: "false"},
	}

	group, err := mgr.LoadGroup("APP_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	debug, err := group.GetString("DEBUG")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if debug != "false" {
		t.Errorf("expected default 'false', got '%s'", debug)
	}
}

func TestExists(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	if !group.Exists("NAME") {
		t.Error("expected NAME to exist")
	}
	if group.Exists("NONEXISTENT") {
		t.Error("expected NONEXISTENT to not exist")
	}
}

func TestGetGroup(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	_, err = mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	group, ok := mgr.GetGroup("APP_")
	if !ok {
		t.Fatal("expected to get loaded group")
	}
	if group.Prefix() != "APP_" {
		t.Errorf("expected prefix 'APP_', got '%s'", group.Prefix())
	}

	_, ok = mgr.GetGroup("NONEXISTENT_")
	if ok {
		t.Error("expected to not get non-existent group")
	}
}

func TestLoadGroupNilConfig(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "NAME", Required: true},
		nil,
	}

	group, err := mgr.LoadGroup("APP_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup should handle nil config: %v", err)
	}
	if !group.Exists("NAME") {
		t.Error("expected NAME to exist")
	}
}

func TestEnvWithEqualsInValue(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_CONNECTION=host=localhost;port=5432",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	conn, err := group.GetString("CONNECTION")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if conn != "host=localhost;port=5432" {
		t.Errorf("expected 'host=localhost;port=5432', got '%s'", conn)
	}
}

func TestInvalidEnvFormat(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
		"INVALID_WITHOUT_EQUALS",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	if !group.Exists("NAME") {
		t.Error("expected NAME to exist")
	}
}

func TestConcurrentAccess(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=test",
		"APP_PORT=8080",
		"APP_DEBUG=true",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			_, _ = group.GetString("NAME")
			_, _ = group.GetInt("PORT")
			_, _ = group.GetBool("DEBUG")
			_ = group.GetAll()
			_ = group.Exists("NAME")
		}(i)
	}

	wg.Wait()
}

func TestConcurrentSensitiveAccess(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"DB_PASSWORD=secret123",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	configs := []*EnvConfig{
		{Key: "PASSWORD", Required: true, Sensitive: true},
	}

	group, err := mgr.LoadGroup("DB_", configs...)
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	var wg sync.WaitGroup
	numGoroutines := 50

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			sv, err := mgr.GetSensitive(group, "PASSWORD")
			if err != nil {
				t.Errorf("goroutine %d: GetSensitive failed: %v", id, err)
				return
			}

			plaintext, err := sv.Decrypt()
			if err != nil {
				t.Errorf("goroutine %d: Decrypt failed: %v", id, err)
				return
			}

			if plaintext != "secret123" {
				t.Errorf("goroutine %d: expected 'secret123', got '%s'", id, plaintext)
			}
		}(i)
	}

	wg.Wait()
}

func TestTypeConversionEdgeCases(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_ZERO=0",
		"APP_NEGATIVE=-123",
		"APP_LARGE=9223372036854775807",
		"APP_FLOAT_ZERO=0.0",
		"APP_BOOL_TRUE=TRUE",
		"APP_BOOL_FALSE=FALSE",
		"APP_DURATION_ZERO=0s",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	zero, err := group.GetInt("ZERO")
	if err != nil {
		t.Fatalf("GetInt(ZERO) failed: %v", err)
	}
	if zero != 0 {
		t.Errorf("expected 0, got %d", zero)
	}

	negative, err := group.GetInt("NEGATIVE")
	if err != nil {
		t.Fatalf("GetInt(NEGATIVE) failed: %v", err)
	}
	if negative != -123 {
		t.Errorf("expected -123, got %d", negative)
	}

	large, err := group.GetInt64("LARGE")
	if err != nil {
		t.Fatalf("GetInt64(LARGE) failed: %v", err)
	}
	if large != 9223372036854775807 {
		t.Errorf("expected 9223372036854775807, got %d", large)
	}

	floatZero, err := group.GetFloat64("FLOAT_ZERO")
	if err != nil {
		t.Fatalf("GetFloat64(FLOAT_ZERO) failed: %v", err)
	}
	if floatZero != 0.0 {
		t.Errorf("expected 0.0, got %f", floatZero)
	}

	boolTrue, err := group.GetBool("BOOL_TRUE")
	if err != nil {
		t.Fatalf("GetBool(BOOL_TRUE) failed: %v", err)
	}
	if !boolTrue {
		t.Error("expected true, got false")
	}

	boolFalse, err := group.GetBool("BOOL_FALSE")
	if err != nil {
		t.Fatalf("GetBool(BOOL_FALSE) failed: %v", err)
	}
	if boolFalse {
		t.Error("expected false, got true")
	}

	durZero, err := group.GetDuration("DURATION_ZERO")
	if err != nil {
		t.Fatalf("GetDuration(DURATION_ZERO) failed: %v", err)
	}
	if durZero != 0 {
		t.Errorf("expected 0, got %v", durZero)
	}
}

func TestMultipleGroups(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=myapp",
		"APP_PORT=8080",
		"DB_HOST=localhost",
		"DB_PORT=5432",
		"CACHE_TTL=300s",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	appGroup, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup APP_ failed: %v", err)
	}

	dbGroup, err := mgr.LoadGroup("DB_")
	if err != nil {
		t.Fatalf("LoadGroup DB_ failed: %v", err)
	}

	cacheGroup, err := mgr.LoadGroup("CACHE_")
	if err != nil {
		t.Fatalf("LoadGroup CACHE_ failed: %v", err)
	}

	appName, _ := appGroup.GetString("NAME")
	if appName != "myapp" {
		t.Errorf("expected 'myapp', got '%s'", appName)
	}

	dbHost, _ := dbGroup.GetString("HOST")
	if dbHost != "localhost" {
		t.Errorf("expected 'localhost', got '%s'", dbHost)
	}

	cacheTTL, _ := cacheGroup.GetString("TTL")
	if cacheTTL != "300s" {
		t.Errorf("expected '300s', got '%s'", cacheTTL)
	}

	_, ok := mgr.GetGroup("APP_")
	if !ok {
		t.Error("APP_ group should exist")
	}

	_, ok = mgr.GetGroup("DB_")
	if !ok {
		t.Error("DB_ group should exist")
	}
}

func TestSensitiveValueDecryptWithoutKey(t *testing.T) {
	sv := &SensitiveValue{
		ciphertext: []byte("encrypted"),
		nonce:      make([]byte, nonceSize),
		key:        make([]byte, aesKeySize),
	}

	_, err := sv.Decrypt()
	if err == nil {
		t.Fatal("expected error when decrypting without proper key")
	}
}

func TestParseEncryptedValueInvalidBase64(t *testing.T) {
	_, err := parseEncryptedValue("invalid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestParseEncryptedValueTooShort(t *testing.T) {
	_, err := parseEncryptedValue("YWJj")
	if err == nil {
		t.Fatal("expected error for data too short")
	}
}

func TestRealOSEnvironment(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	os.Setenv("TEST_ENVMGR_NAME", "testvalue")
	defer os.Unsetenv("TEST_ENVMGR_NAME")

	group, err := mgr.LoadGroup("TEST_ENVMGR_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	val, err := group.GetString("NAME")
	if err != nil {
		t.Fatalf("GetString failed: %v", err)
	}
	if val != "testvalue" {
		t.Errorf("expected 'testvalue', got '%s'", val)
	}
}

func TestCaseSensitiveKeys(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"APP_NAME=lowercase",
		"APP_name=uppercase",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("APP_")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	val1, err := group.GetString("NAME")
	if err != nil {
		t.Fatalf("GetString(NAME) failed: %v", err)
	}

	val2, err := group.GetString("name")
	if err != nil {
		t.Fatalf("GetString(name) failed: %v", err)
	}

	if val1 == val2 {
		t.Error("keys should be case sensitive")
	}
}

func TestEmptyPrefixMatchesAll(t *testing.T) {
	mgr, err := NewEnvManager()
	if err != nil {
		t.Fatalf("NewEnvManager failed: %v", err)
	}

	mockEnvs := []string{
		"KEY1=val1",
		"KEY2=val2",
		"OTHER=val3",
	}
	mgr.setEnvSource(mockEnvSource(mockEnvs))

	group, err := mgr.LoadGroup("")
	if err != nil {
		t.Fatalf("LoadGroup failed: %v", err)
	}

	all := group.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 values with empty prefix, got %d", len(all))
	}
}
