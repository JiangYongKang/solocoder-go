package restclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewClient_Default(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.httpClient == nil {
		t.Error("expected non-nil httpClient")
	}
	if c.templates == nil {
		t.Error("expected non-nil templates map")
	}
	if c.authProviders == nil {
		t.Error("expected non-nil authProviders map")
	}
}

func TestNewClient_WithBaseURL(t *testing.T) {
	c := NewClient(WithBaseURL("https://api.example.com"))
	if c.baseURL != "https://api.example.com" {
		t.Errorf("expected baseURL 'https://api.example.com', got '%s'", c.baseURL)
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	c := NewClient(WithHTTPClient(customClient))
	if c.httpClient != customClient {
		t.Error("expected custom httpClient to be used")
	}
}

func TestRegisterTemplate_Success(t *testing.T) {
	c := NewClient()
	tmpl := RequestTemplate{
		Name:   "get_user",
		Method: http.MethodGet,
		Path:   "/users/{id}",
	}
	err := c.RegisterTemplate(tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := c.GetTemplate("get_user")
	if err != nil {
		t.Fatalf("unexpected error getting template: %v", err)
	}
	if got.Name != "get_user" {
		t.Errorf("expected name 'get_user', got '%s'", got.Name)
	}
	if got.Method != http.MethodGet {
		t.Errorf("expected method GET, got '%s'", got.Method)
	}
}

func TestRegisterTemplate_EmptyName(t *testing.T) {
	c := NewClient()
	tmpl := RequestTemplate{
		Name: "",
		Path: "/users",
	}
	err := c.RegisterTemplate(tmpl)
	if !errors.Is(err, ErrTemplateNameEmpty) {
		t.Errorf("expected ErrTemplateNameEmpty, got %v", err)
	}
}

func TestRegisterTemplate_DefaultMethod(t *testing.T) {
	c := NewClient()
	tmpl := RequestTemplate{
		Name: "test",
		Path: "/test",
	}
	err := c.RegisterTemplate(tmpl)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := c.GetTemplate("test")
	if got.Method != http.MethodGet {
		t.Errorf("expected default method GET, got '%s'", got.Method)
	}
}

func TestRegisterTemplate_Overwrite(t *testing.T) {
	c := NewClient()

	tmpl1 := RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/v1/users",
	}
	if err := c.RegisterTemplate(tmpl1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tmpl2 := RequestTemplate{
		Name:   "test",
		Method: http.MethodPost,
		Path:   "/v2/users",
	}
	if err := c.RegisterTemplate(tmpl2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := c.GetTemplate("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Method != http.MethodPost {
		t.Errorf("expected method POST after overwrite, got '%s'", got.Method)
	}
	if got.Path != "/v2/users" {
		t.Errorf("expected path '/v2/users' after overwrite, got '%s'", got.Path)
	}
}

func TestRegisterTemplate_NegativeValuesNormalized(t *testing.T) {
	c := NewClient()
	tmpl := RequestTemplate{
		Name:          "test",
		Path:          "/test",
		Timeout:       -1 * time.Second,
		MaxRetries:    -5,
		RetryInterval: -10 * time.Millisecond,
	}
	if err := c.RegisterTemplate(tmpl); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := c.GetTemplate("test")
	if got.Timeout != 0 {
		t.Errorf("expected Timeout normalized to 0, got %v", got.Timeout)
	}
	if got.MaxRetries != 0 {
		t.Errorf("expected MaxRetries normalized to 0, got %d", got.MaxRetries)
	}
	if got.RetryInterval != 0 {
		t.Errorf("expected RetryInterval normalized to 0, got %v", got.RetryInterval)
	}
}

func TestGetTemplate_NotFound(t *testing.T) {
	c := NewClient()
	_, err := c.GetTemplate("nonexistent")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestUnregisterTemplate(t *testing.T) {
	c := NewClient()
	c.RegisterTemplate(RequestTemplate{Name: "test", Path: "/test"})

	c.UnregisterTemplate("test")
	_, err := c.GetTemplate("test")
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound after unregister, got %v", err)
	}
}

func TestRegisterAuthProvider(t *testing.T) {
	c := NewClient()
	provider := &testAuthProvider{name: "bearer", token: "test-token"}

	err := c.RegisterAuthProvider(provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := c.GetAuthProvider("bearer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Name() != "bearer" {
		t.Errorf("expected provider name 'bearer', got '%s'", got.Name())
	}
}

func TestRegisterAuthProvider_Nil(t *testing.T) {
	c := NewClient()
	err := c.RegisterAuthProvider(nil)
	if !errors.Is(err, ErrAuthProviderNotFound) {
		t.Errorf("expected ErrAuthProviderNotFound, got %v", err)
	}
}

func TestGetAuthProvider_NotFound(t *testing.T) {
	c := NewClient()
	_, err := c.GetAuthProvider("nonexistent")
	if !errors.Is(err, ErrAuthProviderNotFound) {
		t.Errorf("expected ErrAuthProviderNotFound, got %v", err)
	}
}

func TestDo_SimpleRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "hello",
		Method: http.MethodGet,
		Path:   "/hello",
	})

	resp, err := c.Do(context.Background(), "hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("expected body 'hello', got '%s'", string(body))
	}
}

func TestDo_TemplateNotFound(t *testing.T) {
	c := NewClient()
	_, err := c.Do(context.Background(), "nonexistent", nil)
	if !errors.Is(err, ErrTemplateNotFound) {
		t.Errorf("expected ErrTemplateNotFound, got %v", err)
	}
}

func TestDo_PathParams(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "get_user",
		Method: http.MethodGet,
		Path:   "/users/{id}/posts/{postId}",
	})

	opts := &RequestOptions{
		PathParams: map[string]string{
			"id":     "123",
			"postId": "456",
		},
	}

	_, err := c.Do(context.Background(), "get_user", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := "/users/123/posts/456"
	if receivedPath != expectedPath {
		t.Errorf("expected path '%s', got '%s'", expectedPath, receivedPath)
	}
}

func TestDo_PathParams_Missing(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com"))
	c.RegisterTemplate(RequestTemplate{
		Name:   "get_user",
		Method: http.MethodGet,
		Path:   "/users/{id}",
	})

	_, err := c.Do(context.Background(), "get_user", nil)
	if !errors.Is(err, ErrRequestBuildFailed) {
		t.Errorf("expected ErrRequestBuildFailed, got %v", err)
	}
	if !errors.Is(err, ErrPathParamMissing) {
		t.Errorf("expected error to wrap ErrPathParamMissing, got %v", err)
	}
}

func TestDo_PathParams_PartialMissing(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com"))
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/users/{id}/posts/{postId}",
	})

	opts := &RequestOptions{
		PathParams: map[string]string{
			"id": "123",
		},
	}

	_, err := c.Do(context.Background(), "test", opts)
	if !errors.Is(err, ErrPathParamMissing) {
		t.Errorf("expected ErrPathParamMissing, got %v", err)
	}
}

func TestDo_PathParams_SpecialChars(t *testing.T) {
	var receivedRawURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRawURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/items/{name}",
	})

	opts := &RequestOptions{
		PathParams: map[string]string{
			"name": "hello world/test",
		},
	}

	_, err := c.Do(context.Background(), "test", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !contains(receivedRawURI, "hello%20world") {
		t.Errorf("expected raw URI to contain encoded space 'hello%%20world', got '%s'", receivedRawURI)
	}
	if !contains(receivedRawURI, "%2Ftest") && !contains(receivedRawURI, "%2ftest") {
		t.Errorf("expected raw URI to contain encoded slash '%%2Ftest', got '%s'", receivedRawURI)
	}
}

func TestDo_QueryParams(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "search",
		Method: http.MethodGet,
		Path:   "/search",
	})

	opts := &RequestOptions{
		QueryParams: map[string]string{
			"q":    "golang",
			"page": "1",
		},
	}

	_, err := c.Do(context.Background(), "search", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedQuery == "" {
		t.Error("expected query params, got empty string")
	}

	parsed, err := parseQuery(receivedQuery)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if parsed["q"] != "golang" {
		t.Errorf("expected q='golang', got '%s'", parsed["q"])
	}
	if parsed["page"] != "1" {
		t.Errorf("expected page='1', got '%s'", parsed["page"])
	}
}

func TestDo_QueryParams_SpecialChars(t *testing.T) {
	var receivedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/test",
	})

	opts := &RequestOptions{
		QueryParams: map[string]string{
			"q": "hello world&foo=bar",
		},
	}

	_, err := c.Do(context.Background(), "test", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	parsed, err := parseQuery(receivedQuery)
	if err != nil {
		t.Fatalf("failed to parse query: %v", err)
	}
	if parsed["q"] != "hello world&foo=bar" {
		t.Errorf("expected q='hello world&foo=bar', got '%s'", parsed["q"])
	}
}

func TestDo_DefaultHeaders(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	defaultHeaders := make(http.Header)
	defaultHeaders.Set("X-API-Version", "1.0")
	defaultHeaders.Set("Content-Type", "application/json")

	c.RegisterTemplate(RequestTemplate{
		Name:           "test",
		Method:         http.MethodGet,
		Path:           "/test",
		DefaultHeaders: defaultHeaders,
	})

	_, err := c.Do(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedHeaders.Get("X-API-Version") != "1.0" {
		t.Errorf("expected X-API-Version '1.0', got '%s'", receivedHeaders.Get("X-API-Version"))
	}
	if receivedHeaders.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", receivedHeaders.Get("Content-Type"))
	}
}

func TestDo_MergeHeaders_RequestOverrides(t *testing.T) {
	var receivedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	defaultHeaders := make(http.Header)
	defaultHeaders.Set("X-API-Version", "1.0")
	defaultHeaders.Set("Accept", "text/plain")

	c.RegisterTemplate(RequestTemplate{
		Name:           "test",
		Method:         http.MethodGet,
		Path:           "/test",
		DefaultHeaders: defaultHeaders,
	})

	requestHeaders := make(http.Header)
	requestHeaders.Set("Accept", "application/json")
	requestHeaders.Set("Authorization", "Bearer token")

	opts := &RequestOptions{
		Headers: requestHeaders,
	}

	_, err := c.Do(context.Background(), "test", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedHeaders.Get("X-API-Version") != "1.0" {
		t.Errorf("expected X-API-Version '1.0', got '%s'", receivedHeaders.Get("X-API-Version"))
	}

	acceptVals := receivedHeaders.Values("Accept")
	if len(acceptVals) < 2 {
		t.Fatalf("expected at least 2 Accept header values, got %d", len(acceptVals))
	}
	if acceptVals[0] != "text/plain" {
		t.Errorf("expected first Accept 'text/plain', got '%s'", acceptVals[0])
	}
	if acceptVals[1] != "application/json" {
		t.Errorf("expected second Accept 'application/json', got '%s'", acceptVals[1])
	}

	if receivedHeaders.Get("Authorization") != "Bearer token" {
		t.Errorf("expected Authorization 'Bearer token', got '%s'", receivedHeaders.Get("Authorization"))
	}
}

func TestDo_AuthProvider(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	provider := &testAuthProvider{name: "bearer", token: "my-secret-token"}
	c.RegisterAuthProvider(provider)

	c.RegisterTemplate(RequestTemplate{
		Name:         "test",
		Method:       http.MethodGet,
		Path:         "/test",
		AuthProvider: "bearer",
	})

	_, err := c.Do(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if receivedAuth != "Bearer my-secret-token" {
		t.Errorf("expected Authorization 'Bearer my-secret-token', got '%s'", receivedAuth)
	}
}

func TestDo_AuthProvider_NotFound(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com"))
	c.RegisterTemplate(RequestTemplate{
		Name:         "test",
		Method:       http.MethodGet,
		Path:         "/test",
		AuthProvider: "nonexistent",
	})

	_, err := c.Do(context.Background(), "test", nil)
	if !errors.Is(err, ErrAuthProviderNotFound) {
		t.Errorf("expected ErrAuthProviderNotFound, got %v", err)
	}
}

func TestDo_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:    "slow",
		Method:  http.MethodGet,
		Path:    "/slow",
		Timeout: 50 * time.Millisecond,
	})

	_, err := c.Do(context.Background(), "slow", nil)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestDo_Timeout_NoTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:    "fast",
		Method:  http.MethodGet,
		Path:    "/fast",
		Timeout: 500 * time.Millisecond,
	})

	resp, err := c.Do(context.Background(), "fast", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDo_Retry_SuccessOnFirstTry(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:          "test",
		Method:        http.MethodGet,
		Path:          "/test",
		MaxRetries:    3,
		RetryInterval: 5 * time.Millisecond,
	})

	resp, err := c.Do(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 call, got %d", atomic.LoadInt32(&callCount))
	}
}

func TestDo_Retry_SucceedAfterRetries(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	customTransport := &retryableTransport{
		base:      http.DefaultTransport,
		retry5xx:  true,
	}
	customClient := &http.Client{
		Transport: customTransport,
	}

	c := NewClient(WithBaseURL(server.URL), WithHTTPClient(customClient))
	c.RegisterTemplate(RequestTemplate{
		Name:          "flaky",
		Method:        http.MethodGet,
		Path:          "/flaky",
		MaxRetries:    3,
		RetryInterval: 5 * time.Millisecond,
	})

	resp, err := c.Do(context.Background(), "flaky", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if atomic.LoadInt32(&callCount) != 3 {
		t.Errorf("expected 3 calls, got %d", atomic.LoadInt32(&callCount))
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDo_Retry_MaxRetriesExceeded(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	customTransport := &retryableTransport{
		base:     http.DefaultTransport,
		retry5xx: true,
	}
	customClient := &http.Client{
		Transport: customTransport,
	}

	c := NewClient(WithBaseURL(server.URL), WithHTTPClient(customClient))
	c.RegisterTemplate(RequestTemplate{
		Name:          "failing",
		Method:        http.MethodGet,
		Path:          "/failing",
		MaxRetries:    2,
		RetryInterval: 5 * time.Millisecond,
	})

	_, err := c.Do(context.Background(), "failing", nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if !errors.Is(err, ErrMaxRetriesExceeded) {
		t.Errorf("expected ErrMaxRetriesExceeded, got %v", err)
	}

	expectedCalls := int32(3)
	if atomic.LoadInt32(&callCount) != expectedCalls {
		t.Errorf("expected %d calls (1 initial + 2 retries), got %d", expectedCalls, atomic.LoadInt32(&callCount))
	}
}

func TestDo_Retry_ZeroRetries(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	customTransport := &retryableTransport{
		base:     http.DefaultTransport,
		retry5xx: true,
	}
	customClient := &http.Client{
		Transport: customTransport,
	}

	c := NewClient(WithBaseURL(server.URL), WithHTTPClient(customClient))
	c.RegisterTemplate(RequestTemplate{
		Name:       "failing",
		Method:     http.MethodGet,
		Path:       "/failing",
		MaxRetries: 0,
	})

	_, err := c.Do(context.Background(), "failing", nil)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 call (no retries), got %d", atomic.LoadInt32(&callCount))
	}
}

func TestDo_Retry_ContextCanceled(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	customTransport := &retryableTransport{
		base:     http.DefaultTransport,
		retry5xx: true,
	}
	customClient := &http.Client{
		Transport: customTransport,
	}

	c := NewClient(WithBaseURL(server.URL), WithHTTPClient(customClient))
	c.RegisterTemplate(RequestTemplate{
		Name:          "failing",
		Method:        http.MethodGet,
		Path:          "/failing",
		MaxRetries:    10,
		RetryInterval: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := c.Do(ctx, "failing", nil)
		done <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Do did not return after context cancellation")
	}
}

func TestDo_RequestBody(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "post",
		Method: http.MethodPost,
		Path:   "/data",
	})

	opts := &RequestOptions{
		Body: []byte(`{"name":"test"}`),
	}

	resp, err := c.Do(context.Background(), "post", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if receivedBody != `{"name":"test"}` {
		t.Errorf("expected body '{\"name\":\"test\"}', got '%s'", receivedBody)
	}
}

func TestDo_BaseURL_TemplateOverridesClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL("http://wrong.example.com"))
	c.RegisterTemplate(RequestTemplate{
		Name:    "test",
		Method:  http.MethodGet,
		BaseURL: server.URL,
		Path:    "/test",
	})

	resp, err := c.Do(context.Background(), "test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestDo_Path_WithBaseURL_SlashHandling(t *testing.T) {
	testCases := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "both have slash",
			baseURL:  "http://example.com/",
			path:     "/users",
			expected: "/users",
		},
		{
			name:     "neither has slash",
			baseURL:  "http://example.com",
			path:     "users",
			expected: "/users",
		},
		{
			name:     "base has slash path doesn't",
			baseURL:  "http://example.com/",
			path:     "users",
			expected: "/users",
		},
		{
			name:     "path has slash base doesn't",
			baseURL:  "http://example.com",
			path:     "/users",
			expected: "/users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			c := NewClient(WithBaseURL(server.URL))
			c.RegisterTemplate(RequestTemplate{
				Name:    "test",
				Method:  http.MethodGet,
				BaseURL: server.URL,
				Path:    tc.path,
			})

			resp, err := c.Do(context.Background(), "test", nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			resp.Body.Close()

			if receivedPath != tc.expected {
				t.Errorf("expected path '%s', got '%s'", tc.expected, receivedPath)
			}
		})
	}
}

func TestDo_Concurrent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewClient(WithBaseURL(server.URL))
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/test",
	})

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	var successCount int32

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			resp, err := c.Do(context.Background(), "test", nil)
			if err == nil {
				resp.Body.Close()
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != goroutines {
		t.Errorf("expected %d successes, got %d", goroutines, successCount)
	}
}

func TestReplacePathParams(t *testing.T) {
	testCases := []struct {
		name     string
		path     string
		params   map[string]string
		expected string
		wantErr  bool
	}{
		{
			name:     "single param",
			path:     "/users/{id}",
			params:   map[string]string{"id": "123"},
			expected: "/users/123",
			wantErr:  false,
		},
		{
			name:     "multiple params",
			path:     "/users/{id}/posts/{postId}",
			params:   map[string]string{"id": "1", "postId": "2"},
			expected: "/users/1/posts/2",
			wantErr:  false,
		},
		{
			name:     "no params",
			path:     "/users",
			params:   map[string]string{},
			expected: "/users",
			wantErr:  false,
		},
		{
			name:     "missing param",
			path:     "/users/{id}",
			params:   map[string]string{},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "partial missing",
			path:     "/users/{id}/posts/{postId}",
			params:   map[string]string{"id": "1"},
			expected: "",
			wantErr:  true,
		},
		{
			name:     "special chars encoded",
			path:     "/items/{name}",
			params:   map[string]string{"name": "a/b c"},
			expected: "/items/a%2Fb%20c",
			wantErr:  false,
		},
		{
			name:     "same param multiple times",
			path:     "/{id}/{id}",
			params:   map[string]string{"id": "x"},
			expected: "/x/x",
			wantErr:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := replacePathParams(tc.path, tc.params)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != tc.expected {
					t.Errorf("expected '%s', got '%s'", tc.expected, got)
				}
			}
		})
	}
}

func TestMergeHeaders(t *testing.T) {
	dst := make(http.Header)
	dst.Set("X-Existing", "old")
	dst.Add("Accept", "text/plain")

	src := make(http.Header)
	src.Set("X-New", "new")
	src.Add("Accept", "application/json")

	mergeHeaders(dst, src)

	if dst.Get("X-Existing") != "old" {
		t.Errorf("expected X-Existing 'old', got '%s'", dst.Get("X-Existing"))
	}
	if dst.Get("X-New") != "new" {
		t.Errorf("expected X-New 'new', got '%s'", dst.Get("X-New"))
	}
	acceptVals := dst.Values("Accept")
	if len(acceptVals) != 2 {
		t.Fatalf("expected 2 Accept values, got %d", len(acceptVals))
	}
	if acceptVals[0] != "text/plain" || acceptVals[1] != "application/json" {
		t.Errorf("unexpected Accept values: %v", acceptVals)
	}
}

func TestBuildURL(t *testing.T) {
	c := NewClient(WithBaseURL("https://api.example.com"))

	tmpl := &RequestTemplate{
		Path: "/users/{id}",
	}
	opts := &RequestOptions{
		PathParams: map[string]string{"id": "123"},
		QueryParams: map[string]string{
			"verbose": "true",
			"limit":   "10",
		},
	}

	url, err := c.buildURL(tmpl, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "https://api.example.com/users/123"
	if !startsWith(url, expected) {
		t.Errorf("expected URL to start with '%s', got '%s'", expected, url)
	}

	if !contains(url, "verbose=true") {
		t.Errorf("expected URL to contain 'verbose=true', got '%s'", url)
	}
	if !contains(url, "limit=10") {
		t.Errorf("expected URL to contain 'limit=10', got '%s'", url)
	}
}

func TestGetTemplate_ReturnsCopy(t *testing.T) {
	c := NewClient()
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/test",
	})

	got1, _ := c.GetTemplate("test")
	got1.Path = "/modified"

	got2, _ := c.GetTemplate("test")
	if got2.Path != "/test" {
		t.Error("GetTemplate should return a copy, modifications should not affect internal state")
	}
}

func TestDo_ContextCanceledBeforeRequest(t *testing.T) {
	c := NewClient(WithBaseURL("http://example.com"))
	c.RegisterTemplate(RequestTemplate{
		Name:   "test",
		Method: http.MethodGet,
		Path:   "/test",
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Do(ctx, "test", nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

type testAuthProvider struct {
	name  string
	token string
}

func (p *testAuthProvider) Name() string {
	return p.name
}

func (p *testAuthProvider) Inject(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+p.token)
	return nil
}

type retryableTransport struct {
	base     http.RoundTripper
	retry5xx bool
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if t.retry5xx && resp.StatusCode >= 500 {
		resp.Body.Close()
		return nil, errors.New("server error: " + resp.Status)
	}
	return resp, nil
}

func parseQuery(query string) (map[string]string, error) {
	result := make(map[string]string)
	parts := splitQuery(query)
	for _, part := range parts {
		if part == "" {
			continue
		}
		idx := indexOf(part, "=")
		if idx < 0 {
			result[unescapeQuery(part)] = ""
		} else {
			key := part[:idx]
			value := part[idx+1:]
			result[unescapeQuery(key)] = unescapeQuery(value)
		}
	}
	return result, nil
}

func splitQuery(query string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '&' {
			parts = append(parts, query[start:i])
			start = i + 1
		}
	}
	if start < len(query) {
		parts = append(parts, query[start:])
	}
	return parts
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func unescapeQuery(s string) string {
	result := ""
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '+':
			result += " "
		case '%':
			if i+2 < len(s) {
				result += string(hexToByte(s[i+1], s[i+2]))
				i += 2
			}
		default:
			result += string(s[i])
		}
	}
	return result
}

func hexToByte(high, low byte) byte {
	var h, l byte
	if high >= '0' && high <= '9' {
		h = high - '0'
	} else if high >= 'A' && high <= 'F' {
		h = high - 'A' + 10
	} else if high >= 'a' && high <= 'f' {
		h = high - 'a' + 10
	}
	if low >= '0' && low <= '9' {
		l = low - '0'
	} else if low >= 'A' && low <= 'F' {
		l = low - 'A' + 10
	} else if low >= 'a' && low <= 'f' {
		l = low - 'a' + 10
	}
	return h<<4 | l
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	return indexOf(s, substr) >= 0
}
