package apiver

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestVersion_String(t *testing.T) {
	v := Version("v1")
	if v.String() != "v1" {
		t.Errorf("expected v1, got %s", v.String())
	}
}

func TestVersion_Compare(t *testing.T) {
	tests := []struct {
		name     string
		v1       Version
		v2       Version
		expected int
	}{
		{"v1 < v2", "v1", "v2", -1},
		{"v2 > v1", "v2", "v1", 1},
		{"v1 == v1", "v1", "v1", 0},
		{"v10 > v2", "v10", "v2", 1},
		{"v0 < v1", "v0", "v1", -1},
		{"v99 < v100", "v99", "v100", -1},
		{"invalid vs valid", "invalid", "v1", -1},
		{"both invalid", "invalid1", "invalid2", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.v1.Compare(tt.v2)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestParseVersionNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    Version
		expected int
	}{
		{"v1", "v1", 1},
		{"v123", "v123", 123},
		{"v0", "v0", 0},
		{"no prefix", "123", 123},
		{"invalid", "invalid", 0},
		{"empty", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseVersionNumber(tt.input)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestPathVersionExtractor_ExtractVersion(t *testing.T) {
	extractor := NewPathVersionExtractor()

	tests := []struct {
		name           string
		path           string
		expectedVer    Version
		expectedFound  bool
	}{
		{"v1 with path", "/v1/users", "v1", true},
		{"v2 with path", "/v2/orders/123", "v2", true},
		{"v10 with path", "/v10/api/test", "v10", true},
		{"root only v1", "/v1", "v1", true},
		{"root only v2", "/v2/", "v2", true},
		{"no version", "/users", "", false},
		{"version in middle", "/api/v1/users", "", false},
		{"invalid version", "/va/users", "", false},
		{"root path", "/", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			ver, found := extractor.ExtractVersion(req)
			if found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, found)
			}
			if ver != tt.expectedVer {
				t.Errorf("expected version %s, got %s", tt.expectedVer, ver)
			}
		})
	}

	if extractor.Strategy() != PathStrategy {
		t.Error("expected PathStrategy")
	}
}

func TestHeaderVersionExtractor_ExtractVersion(t *testing.T) {
	extractor := NewHeaderVersionExtractor()

	tests := []struct {
		name          string
		headerValue   string
		expectedVer   Version
		expectedFound bool
	}{
		{"v1", "v1", "v1", true},
		{"v2", "v2", "v2", true},
		{"custom version", "v2024.1", "v2024.1", true},
		{"empty header", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/users", nil)
			if tt.headerValue != "" {
				req.Header.Set("API-Version", tt.headerValue)
			}
			ver, found := extractor.ExtractVersion(req)
			if found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, found)
			}
			if ver != tt.expectedVer {
				t.Errorf("expected version %s, got %s", tt.expectedVer, ver)
			}
		})
	}

	if extractor.Strategy() != HeaderStrategy {
		t.Error("expected HeaderStrategy")
	}
}

func TestHeaderVersionExtractor_CustomHeader(t *testing.T) {
	extractor := NewHeaderVersionExtractorWithName("X-API-Version")
	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("X-API-Version", "v3")

	ver, found := extractor.ExtractVersion(req)
	if !found {
		t.Error("expected to find version with custom header")
	}
	if ver != "v3" {
		t.Errorf("expected v3, got %s", ver)
	}
}

func TestQueryVersionExtractor_ExtractVersion(t *testing.T) {
	extractor := NewQueryVersionExtractor()

	tests := []struct {
		name          string
		queryString   string
		expectedVer   Version
		expectedFound bool
	}{
		{"v1", "version=v1", "v1", true},
		{"v2", "version=v2", "v2", true},
		{"custom version", "version=v2024-beta", "v2024-beta", true},
		{"no query", "", "", false},
		{"different param", "ver=v1", "", false},
		{"empty value", "version=", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/users"
			if tt.queryString != "" {
				url += "?" + tt.queryString
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			ver, found := extractor.ExtractVersion(req)
			if found != tt.expectedFound {
				t.Errorf("expected found=%v, got %v", tt.expectedFound, found)
			}
			if ver != tt.expectedVer {
				t.Errorf("expected version %s, got %s", tt.expectedVer, ver)
			}
		})
	}

	if extractor.Strategy() != QueryStrategy {
		t.Error("expected QueryStrategy")
	}
}

func TestQueryVersionExtractor_CustomParam(t *testing.T) {
	extractor := NewQueryVersionExtractorWithName("api_version")
	req := httptest.NewRequest(http.MethodGet, "/users?api_version=v3", nil)

	ver, found := extractor.ExtractVersion(req)
	if !found {
		t.Error("expected to find version with custom param")
	}
	if ver != "v3" {
		t.Errorf("expected v3, got %s", ver)
	}
}

func TestStripVersionPrefix(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		version  Version
		expected string
	}{
		{"v1 with users", "/v1/users", "v1", "/users"},
		{"v2 with nested", "/v2/api/users/123", "v2", "/api/users/123"},
		{"root only", "/v1", "v1", "/"},
		{"root with slash", "/v1/", "v1", "/"},
		{"no match", "/users", "v1", "/users"},
		{"partial match", "/v10/users", "v1", "/v10/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripVersionPrefix(tt.path, tt.version)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestVersionRouter_RegisterAndGetHandler(t *testing.T) {
	vr := NewVersionRouter()

	handler1 := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1 handler"))
	}
	handler2 := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2 handler"))
	}

	vr.RegisterHandler("v1", handler1)
	vr.RegisterHandler("v2", handler2)

	h, ok := vr.GetHandler("v1")
	if !ok {
		t.Error("expected to find v1 handler")
	}
	if h == nil {
		t.Error("v1 handler should not be nil")
	}

	h, ok = vr.GetHandler("v2")
	if !ok {
		t.Error("expected to find v2 handler")
	}

	_, ok = vr.GetHandler("v3")
	if ok {
		t.Error("should not find v3 handler")
	}
}

func TestVersionRouter_RegisterAndGetConverters(t *testing.T) {
	vr := NewVersionRouter()

	reqConv := func(r *http.Request) (*http.Request, error) {
		return r, nil
	}
	respConv := func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		return status, header, body, nil
	}

	vr.RegisterRequestConverter("v1", "v2", reqConv)
	vr.RegisterResponseConverter("v2", "v1", respConv)

	conv, ok := vr.GetRequestConverter("v1", "v2")
	if !ok {
		t.Error("expected to find request converter v1->v2")
	}
	if conv == nil {
		t.Error("request converter should not be nil")
	}

	_, ok = vr.GetRequestConverter("v2", "v1")
	if ok {
		t.Error("should not find request converter v2->v1")
	}

	rconv, ok := vr.GetResponseConverter("v2", "v1")
	if !ok {
		t.Error("expected to find response converter v2->v1")
	}
	if rconv == nil {
		t.Error("response converter should not be nil")
	}

	_, ok = vr.GetResponseConverter("v1", "v2")
	if ok {
		t.Error("should not find response converter v1->v2")
	}
}

func TestVersionRouter_Versions(t *testing.T) {
	vr := NewVersionRouter()

	vr.RegisterHandler("v3", nil)
	vr.RegisterHandler("v1", nil)
	vr.RegisterHandler("v2", nil)
	vr.RegisterHandler("v10", nil)

	versions := vr.Versions()
	expected := []Version{"v1", "v2", "v3", "v10"}

	if len(versions) != len(expected) {
		t.Fatalf("expected %d versions, got %d", len(expected), len(versions))
	}

	for i, v := range expected {
		if versions[i] != v {
			t.Errorf("expected %s at index %d, got %s", v, i, versions[i])
		}
	}
}

func TestVersionRouter_LatestVersion(t *testing.T) {
	vr := NewVersionRouter()

	_, ok := vr.LatestVersion()
	if ok {
		t.Error("expected no latest version when no handlers registered")
	}

	vr.RegisterHandler("v1", nil)
	vr.RegisterHandler("v2", nil)

	latest, ok := vr.LatestVersion()
	if !ok {
		t.Error("expected to find latest version")
	}
	if latest != "v2" {
		t.Errorf("expected latest v2, got %s", latest)
	}
}

func TestVersionRouter_DefaultVersion(t *testing.T) {
	vr := NewVersionRouter()

	if vr.GetDefaultVersion() != "" {
		t.Error("expected empty default version initially")
	}

	vr.SetDefaultVersion("v1")
	if vr.GetDefaultVersion() != "v1" {
		t.Errorf("expected default version v1, got %s", vr.GetDefaultVersion())
	}
}

func TestVersionRouter_Extractors(t *testing.T) {
	vr := NewVersionRouter()

	extractors := vr.GetExtractors()
	if len(extractors) != 3 {
		t.Errorf("expected 3 default extractors, got %d", len(extractors))
	}

	customExtractor := NewHeaderVersionExtractorWithName("X-Custom-Version")
	vr.SetExtractors(customExtractor)

	extractors = vr.GetExtractors()
	if len(extractors) != 1 {
		t.Errorf("expected 1 custom extractor, got %d", len(extractors))
	}
	if extractors[0].Strategy() != HeaderStrategy {
		t.Error("expected HeaderStrategy for custom extractor")
	}
}

func TestVersionRouter_ExtractVersion_Path(t *testing.T) {
	vr := NewVersionRouter()
	vr.RegisterHandler("v1", nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	ver, newReq, err := vr.ExtractVersion(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "v1" {
		t.Errorf("expected v1, got %s", ver)
	}
	if newReq.URL.Path != "/users" {
		t.Errorf("expected stripped path /users, got %s", newReq.URL.Path)
	}

	stripped, ok := StrippedPathFromContext(newReq.Context())
	if !ok {
		t.Error("expected stripped path in context")
	}
	if stripped != "/users" {
		t.Errorf("expected stripped path /users in context, got %s", stripped)
	}
}

func TestVersionRouter_ExtractVersion_Header(t *testing.T) {
	vr := NewVersionRouter()
	vr.RegisterHandler("v2", nil)
	vr.SetExtractors(NewHeaderVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("API-Version", "v2")

	ver, newReq, err := vr.ExtractVersion(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "v2" {
		t.Errorf("expected v2, got %s", ver)
	}
	if newReq.URL.Path != "/users" {
		t.Errorf("path should not change for header strategy, got %s", newReq.URL.Path)
	}
}

func TestVersionRouter_ExtractVersion_Query(t *testing.T) {
	vr := NewVersionRouter()
	vr.RegisterHandler("v1", nil)
	vr.SetExtractors(NewQueryVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users?version=v1", nil)

	ver, newReq, err := vr.ExtractVersion(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "v1" {
		t.Errorf("expected v1, got %s", ver)
	}
	if newReq.URL.Path != "/users" {
		t.Errorf("path should not change for query strategy, got %s", newReq.URL.Path)
	}
}

func TestVersionRouter_ExtractVersion_Priority(t *testing.T) {
	vr := NewVersionRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/users?version=v2", nil)
	req.Header.Set("API-Version", "v3")

	ver, _, err := vr.ExtractVersion(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "v1" {
		t.Errorf("path should have highest priority, expected v1, got %s", ver)
	}

	vr.SetExtractors(NewHeaderVersionExtractor(), NewQueryVersionExtractor())
	req2 := httptest.NewRequest(http.MethodGet, "/users?version=v2", nil)
	req2.Header.Set("API-Version", "v3")

	ver, _, err = vr.ExtractVersion(req2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ver != "v3" {
		t.Errorf("header should have higher priority than query, expected v3, got %s", ver)
	}
}

func TestVersionRouter_ExtractVersion_DefaultFallback(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewHeaderVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	_, _, err := vr.ExtractVersion(req)
	if err != ErrVersionNotFound {
		t.Errorf("expected ErrVersionNotFound, got %v", err)
	}

	vr.SetDefaultVersion("v1")
	ver, _, err := vr.ExtractVersion(req)
	if err != nil {
		t.Fatalf("unexpected error with default version: %v", err)
	}
	if ver != "v1" {
		t.Errorf("expected default version v1, got %s", ver)
	}
}

func TestVersionRouter_ExtractVersion_NoExtractors(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors()

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	_, _, err := vr.ExtractVersion(req)
	if err != ErrNoVersionExtractor {
		t.Errorf("expected ErrNoVersionExtractor, got %v", err)
	}
}

func TestVersionRouter_ServeHTTP_PathStrategy(t *testing.T) {
	vr := NewVersionRouter()

	v1Called := false
	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		v1Called = true
		if r.URL.Path != "/users" {
			t.Errorf("expected path /users, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1 response"))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v1Called {
		t.Error("v1 handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "v1 response") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_HeaderStrategy(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewHeaderVersionExtractor())

	v2Called := false
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		v2Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2 response"))
	})

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("API-Version", "v2")
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v2Called {
		t.Error("v2 handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "v2 response") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_QueryStrategy(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewQueryVersionExtractor())

	v1Called := false
	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		v1Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1 response"))
	})

	req := httptest.NewRequest(http.MethodGet, "/users?version=v1", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v1Called {
		t.Error("v1 handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestVersionRouter_ServeHTTP_HandlerNotFound(t *testing.T) {
	vr := NewVersionRouter()

	req := httptest.NewRequest(http.MethodGet, "/v99/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), ErrHandlerNotFound.Error()) {
		t.Errorf("expected error message, got: %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_VersionExtractionError(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors()

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestVersionRouter_ServeHTTP_WithConversion(t *testing.T) {
	vr := NewVersionRouter()

	v1RequestReceived := ""
	v2ResponseSent := ""

	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		v1RequestReceived = string(body)
		w.Header().Set("X-Version", "v1")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"old_field":"old_value"}`))
	})

	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		v2ResponseSent = string(body)
		w.Header().Set("X-Version", "v2")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"new_field":"new_value","metadata":{"version":2}}`))
	})

	vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		converted := strings.Replace(string(body), "old_field", "new_field", 1)
		newBody := io.NopCloser(strings.NewReader(converted))
		newReq := r.Clone(r.Context())
		newReq.Body = newBody
		newReq.ContentLength = int64(len(converted))
		return newReq, nil
	})

	vr.RegisterResponseConverter("v2", "v1", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		converted := strings.Replace(string(body), "new_field", "old_field", 1)
		newHeader := http.Header{}
		for k, v := range header {
			if k != "X-Version" {
				newHeader[k] = v
			}
		}
		newHeader.Set("X-Version", "v1")
		return http.StatusOK, newHeader, []byte(converted), nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/users", strings.NewReader(`{"old_field":"request_value"}`))
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if v2ResponseSent != `{"new_field":"request_value"}` {
		t.Errorf("expected v2 to receive converted request, got: %s", v2ResponseSent)
	}

	if v1RequestReceived != "" {
		t.Error("v1 handler should not be called directly")
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 after response conversion, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), `"old_field":"new_value"`) {
		t.Errorf("expected converted response body, got: %s", w.Body.String())
	}

	if w.Header().Get("X-Version") != "v1" {
		t.Errorf("expected X-Version: v1, got: %s", w.Header().Get("X-Version"))
	}
}

func TestVersionRouter_ServeHTTP_SameVersionNoConversion(t *testing.T) {
	vr := NewVersionRouter()

	v2Called := false
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		v2Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2 direct"))
	})

	req := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v2Called {
		t.Error("v2 handler should be called directly")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "v2 direct") {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_GracefulDegradation_NoRequestConverter(t *testing.T) {
	vr := NewVersionRouter()

	v1Called := false
	v2Called := false
	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		v1Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1 direct"))
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		v2Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2 direct"))
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v1Called {
		t.Error("v1 handler should be called directly (graceful degradation)")
	}
	if v2Called {
		t.Error("v2 handler should not be called when no request converter")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with graceful degradation, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "v1 direct") {
		t.Errorf("expected v1 direct response, got: %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_ResponseConverterMissing(t *testing.T) {
	vr := NewVersionRouter()

	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1"))
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", "value")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("v2 response"))
	})

	vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
		return r, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 when response converter not found, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), ErrConverterNotFound.Error()) {
		t.Errorf("expected converter error message, got: %s", w.Body.String())
	}
}

func TestIsValidVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    Version
		expected bool
	}{
		{"v1", "v1", true},
		{"v10", "v10", true},
		{"v123", "v123", true},
		{"v0", "v0", true},
		{"empty", "", false},
		{"no v prefix", "1", false},
		{"letters after v", "va", false},
		{"mixed", "v1a", false},
		{"dot version", "v1.0", false},
		{"beta suffix", "v1-beta", false},
		{"capital V", "V1", false},
		{"only v", "v", false},
		{"v-1", "v-1", false},
		{"v 1", "v 1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidVersion(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v for %q, got %v", tt.expected, tt.input, result)
			}
		})
	}
}

func TestVersionRouter_ExtractVersion_InvalidFormatHeader(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewHeaderVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("API-Version", "invalid")

	_, _, err := vr.ExtractVersion(req)
	if err != ErrInvalidVersionFormat {
		t.Errorf("expected ErrInvalidVersionFormat, got %v", err)
	}
}

func TestVersionRouter_ExtractVersion_InvalidFormatQuery(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewQueryVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users?version=beta", nil)

	_, _, err := vr.ExtractVersion(req)
	if err != ErrInvalidVersionFormat {
		t.Errorf("expected ErrInvalidVersionFormat, got %v", err)
	}
}

func TestVersionRouter_ExtractVersion_InvalidDefaultVersion(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewHeaderVersionExtractor())
	vr.SetDefaultVersion("invalid")

	req := httptest.NewRequest(http.MethodGet, "/users", nil)

	_, _, err := vr.ExtractVersion(req)
	if err != ErrInvalidVersionFormat {
		t.Errorf("expected ErrInvalidVersionFormat for invalid default, got %v", err)
	}
}

func TestVersionRouter_ServeHTTP_InvalidVersionFormat(t *testing.T) {
	vr := NewVersionRouter()
	vr.SetExtractors(NewHeaderVersionExtractor())

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	req.Header.Set("API-Version", "bad-version")
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid version format, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), ErrInvalidVersionFormat.Error()) {
		t.Errorf("expected invalid version error message, got: %s", w.Body.String())
	}
}

func TestVersionRouter_GracefulDegradation_MultipleVersions(t *testing.T) {
	vr := NewVersionRouter()

	v1Called := false
	v3Called := false
	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		v1Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v1"))
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2"))
	})
	vr.RegisterHandler("v3", func(w http.ResponseWriter, r *http.Request) {
		v3Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v3"))
	})

	vr.RegisterRequestConverter("v2", "v3", func(r *http.Request) (*http.Request, error) {
		return r, nil
	})
	vr.RegisterResponseConverter("v3", "v2", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		return status, header, body, nil
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if !v1Called {
		t.Error("v1 handler should be called directly (graceful degradation)")
	}
	if v3Called {
		t.Error("v3 handler should not be called when no v1->v3 converter")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with graceful degradation, got %d", w.Code)
	}

	v2Called := false
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		v2Called = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("v2"))
	})

	req2 := httptest.NewRequest(http.MethodGet, "/v2/users", nil)
	w2 := httptest.NewRecorder()
	vr.ServeHTTP(w2, req2)

	if v2Called {
		t.Error("v2 handler should NOT be called directly when converter exists")
	}
	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 with conversion, got %d", w2.Code)
	}
}

func TestResponseCapture(t *testing.T) {
	rc := newResponseCapture()

	rc.Header().Set("X-Test", "value")
	rc.WriteHeader(http.StatusCreated)
	rc.Write([]byte("test body"))

	if rc.statusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", rc.statusCode)
	}
	if rc.Header().Get("X-Test") != "value" {
		t.Errorf("expected X-Test header, got: %s", rc.Header().Get("X-Test"))
	}
	if string(rc.body) != "test body" {
		t.Errorf("expected test body, got: %s", string(rc.body))
	}
}

func TestResponseCapture_DefaultStatusCode(t *testing.T) {
	rc := newResponseCapture()

	rc.Write([]byte("body without header"))

	if rc.statusCode != http.StatusOK {
		t.Errorf("expected default 200, got %d", rc.statusCode)
	}
}

func TestStrippedPathFromContext(t *testing.T) {
	ctx := context.Background()

	_, ok := StrippedPathFromContext(ctx)
	if ok {
		t.Error("should not find stripped path in empty context")
	}

	ctx = context.WithValue(ctx, StrippedPathKey, "/users")
	path, ok := StrippedPathFromContext(ctx)
	if !ok {
		t.Error("should find stripped path in context")
	}
	if path != "/users" {
		t.Errorf("expected /users, got %s", path)
	}

	ctx = context.WithValue(ctx, StrippedPathKey, 123)
	_, ok = StrippedPathFromContext(ctx)
	if ok {
		t.Error("should not find stripped path for wrong type")
	}
}

func TestVersionRouter_ConcurrentAccess(t *testing.T) {
	vr := NewVersionRouter()

	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
		return r, nil
	})
	vr.RegisterResponseConverter("v2", "v1", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		return status, header, body, nil
	})

	var wg sync.WaitGroup
	numGoroutines := 20
	numIterations := 100
	errors := int64(0)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				path := "/v1/users"
				if (id+j)%2 == 0 {
					path = "/v2/users"
				}
				req := httptest.NewRequest(http.MethodGet, path, nil)
				w := httptest.NewRecorder()
				vr.ServeHTTP(w, req)
				if w.Code != http.StatusOK && w.Code != http.StatusBadRequest {
					t.Logf("unexpected status: %d", w.Code)
				}

				_ = vr.Versions()
				_ = vr.GetDefaultVersion()
				_, _ = vr.LatestVersion()
				_, _ = vr.GetHandler("v1")
				_, _ = vr.GetRequestConverter("v1", "v2")
				_, _ = vr.GetResponseConverter("v2", "v1")
				_ = vr.GetExtractors()
			}
		}(i)
	}

	wg.Wait()

	if errors > 0 {
		t.Errorf("observed %d errors during concurrent access", errors)
	}
}

func TestVersionRouter_ConcurrentRegistration(t *testing.T) {
	vr := NewVersionRouter()

	var wg sync.WaitGroup
	numVersions := 20

	for i := 0; i < numVersions; i++ {
		wg.Add(1)
		go func(ver int) {
			defer wg.Done()
			v := Version("v" + itoa(ver))
			vr.RegisterHandler(v, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			if ver > 0 {
				from := Version("v" + itoa(ver-1))
				to := Version("v" + itoa(ver))
				vr.RegisterRequestConverter(from, to, func(r *http.Request) (*http.Request, error) {
					return r, nil
				})
				vr.RegisterResponseConverter(to, from, func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
					return status, header, body, nil
				})
			}
		}(i)
	}

	wg.Wait()

	versions := vr.Versions()
	if len(versions) != numVersions {
		t.Errorf("expected %d versions, got %d", numVersions, len(versions))
	}
}

func TestVersionRouter_RequestConversionWithBody(t *testing.T) {
	vr := NewVersionRouter()

	var receivedBody string
	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"version":2}`))
	})

	vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		converted := bytes.Replace(body, []byte("name"), []byte("full_name"), 1)
		newReq := r.Clone(r.Context())
		newReq.Body = io.NopCloser(bytes.NewReader(converted))
		newReq.ContentLength = int64(len(converted))
		return newReq, nil
	})

	vr.RegisterResponseConverter("v2", "v1", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		return status, header, bytes.Replace(body, []byte("2"), []byte("1"), 1), nil
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/test", bytes.NewBufferString(`{"name":"test"}`))
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if receivedBody != `{"full_name":"test"}` {
		t.Errorf("expected converted request body, got: %s", receivedBody)
	}

	if !strings.Contains(w.Body.String(), `"version":1`) {
		t.Errorf("expected converted response, got: %s", w.Body.String())
	}
}

func TestVersionRouter_ResponseConversionWithHeaders(t *testing.T) {
	vr := NewVersionRouter()

	vr.RegisterHandler("v1", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	vr.RegisterHandler("v2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-API-Version", "2")
		w.Header().Set("X-Custom", "value")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{"data":"v2"}`))
	})

	vr.RegisterRequestConverter("v1", "v2", func(r *http.Request) (*http.Request, error) {
		return r, nil
	})
	vr.RegisterResponseConverter("v2", "v1", func(status int, header http.Header, body []byte) (int, http.Header, []byte, error) {
		newHeader := http.Header{}
		for k, v := range header {
			if k != "X-API-Version" {
				newHeader[k] = v
			}
		}
		newHeader.Set("X-API-Version", "1")
		newHeader.Set("X-Converted", "true")
		return http.StatusOK, newHeader, []byte(`{"data":"v1"}`), nil
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/test", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected converted status 200, got %d", w.Code)
	}
	if w.Header().Get("X-API-Version") != "1" {
		t.Errorf("expected X-API-Version: 1, got %s", w.Header().Get("X-API-Version"))
	}
	if w.Header().Get("X-Custom") != "value" {
		t.Errorf("expected X-Custom header preserved, got %s", w.Header().Get("X-Custom"))
	}
	if w.Header().Get("X-Converted") != "true" {
		t.Errorf("expected X-Converted header added, got %s", w.Header().Get("X-Converted"))
	}
	if w.Body.String() != `{"data":"v1"}` {
		t.Errorf("expected converted body, got %s", w.Body.String())
	}
}

func TestVersionRouter_ServeHTTP_NoHandlers(t *testing.T) {
	vr := NewVersionRouter()

	req := httptest.NewRequest(http.MethodGet, "/v1/users", nil)
	w := httptest.NewRecorder()
	vr.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestVersionRouter_ErrorVariables(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrVersionNotFound", ErrVersionNotFound, "apiver: version not found"},
		{"ErrHandlerNotFound", ErrHandlerNotFound, "apiver: handler not found for version"},
		{"ErrNoVersionExtractor", ErrNoVersionExtractor, "apiver: no version extractor configured"},
		{"ErrInvalidVersionFormat", ErrInvalidVersionFormat, "apiver: invalid version format"},
		{"ErrConverterNotFound", ErrConverterNotFound, "apiver: converter not found for version pair"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("expected %q, got %q", tt.msg, tt.err.Error())
			}
		})
	}
}

func TestVersionedHandler_Struct(t *testing.T) {
	h := func(w http.ResponseWriter, r *http.Request) {}
	vh := VersionedHandler{
		Version: "v1",
		Handler: h,
	}

	if vh.Version != "v1" {
		t.Errorf("expected v1, got %s", vh.Version)
	}
	if vh.Handler == nil {
		t.Error("handler should not be nil")
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
