package contentneg

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"solocoder-go/internal/serialize"
)

type testUser struct {
	ID   int    `json:"id" xml:"id"`
	Name string `json:"name" xml:"name"`
	Age  int    `json:"age" xml:"age"`
}

type testUserPB struct {
	ID   int    `serialize:"id,protobuf:2"`
	Name string `serialize:"name,protobuf:3"`
	Age  int    `serialize:"age,protobuf:4"`
}

func TestMediaTypeFullType(t *testing.T) {
	tests := []struct {
		name     string
		mt       *MediaType
		expected string
	}{
		{"json", &MediaType{Type: "application", Subtype: "json"}, "application/json"},
		{"xml", &MediaType{Type: "application", Subtype: "xml"}, "application/xml"},
		{"wildcard all", &MediaType{Type: "*", Subtype: "*"}, "*/*"},
		{"wildcard subtype", &MediaType{Type: "application", Subtype: "*"}, "application/*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.FullType(); got != tt.expected {
				t.Errorf("FullType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMediaTypeIsWildcardAll(t *testing.T) {
	tests := []struct {
		name     string
		mt       *MediaType
		expected bool
	}{
		{"wildcard all", &MediaType{Type: "*", Subtype: "*"}, true},
		{"wildcard subtype", &MediaType{Type: "application", Subtype: "*"}, false},
		{"specific type", &MediaType{Type: "application", Subtype: "json"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.IsWildcardAll(); got != tt.expected {
				t.Errorf("IsWildcardAll() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMediaTypeIsWildcardSubtype(t *testing.T) {
	tests := []struct {
		name     string
		mt       *MediaType
		expected bool
	}{
		{"wildcard all", &MediaType{Type: "*", Subtype: "*"}, false},
		{"wildcard subtype", &MediaType{Type: "application", Subtype: "*"}, true},
		{"specific type", &MediaType{Type: "application", Subtype: "json"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.IsWildcardSubtype(); got != tt.expected {
				t.Errorf("IsWildcardSubtype() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMediaTypeMatches(t *testing.T) {
	tests := []struct {
		name        string
		mt          *MediaType
		contentType string
		expected    bool
	}{
		{"wildcard all matches json", &MediaType{Type: "*", Subtype: "*"}, "application/json", true},
		{"wildcard all matches xml", &MediaType{Type: "*", Subtype: "*"}, "application/xml", true},
		{"application/* matches json", &MediaType{Type: "application", Subtype: "*"}, "application/json", true},
		{"application/* matches xml", &MediaType{Type: "application", Subtype: "*"}, "application/xml", true},
		{"application/* matches text", &MediaType{Type: "application", Subtype: "*"}, "text/plain", false},
		{"application/json matches json", &MediaType{Type: "application", Subtype: "json"}, "application/json", true},
		{"application/json matches xml", &MediaType{Type: "application", Subtype: "json"}, "application/xml", false},
		{"case insensitive match", &MediaType{Type: "APPLICATION", Subtype: "JSON"}, "application/json", true},
		{"invalid content type", &MediaType{Type: "application", Subtype: "json"}, "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mt.Matches(tt.contentType); got != tt.expected {
				t.Errorf("Matches(%q) = %v, want %v", tt.contentType, got, tt.expected)
			}
		})
	}
}

func TestParseAcceptEmpty(t *testing.T) {
	result := ParseAccept("")
	if len(result) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(result))
	}
	if !result[0].IsWildcardAll() {
		t.Error("expected wildcard media type")
	}
	if result[0].Quality != 1.0 {
		t.Errorf("expected quality 1.0, got %f", result[0].Quality)
	}
}

func TestParseAcceptSingle(t *testing.T) {
	result := ParseAccept("application/json")
	if len(result) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(result))
	}
	if result[0].FullType() != "application/json" {
		t.Errorf("expected application/json, got %s", result[0].FullType())
	}
	if result[0].Quality != 1.0 {
		t.Errorf("expected quality 1.0, got %f", result[0].Quality)
	}
}

func TestParseAcceptWithQuality(t *testing.T) {
	result := ParseAccept("application/json;q=0.8")
	if len(result) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(result))
	}
	if result[0].FullType() != "application/json" {
		t.Errorf("expected application/json, got %s", result[0].FullType())
	}
	if result[0].Quality != 0.8 {
		t.Errorf("expected quality 0.8, got %f", result[0].Quality)
	}
}

func TestParseAcceptMultiple(t *testing.T) {
	result := ParseAccept("application/json;q=0.9, application/xml;q=0.5, */*;q=0.1")
	if len(result) != 3 {
		t.Fatalf("expected 3 media types, got %d", len(result))
	}

	expected := []struct {
		mtType  string
		subtype string
		q       float64
	}{
		{"application", "json", 0.9},
		{"application", "xml", 0.5},
		{"*", "*", 0.1},
	}

	for i, exp := range expected {
		if result[i].Type != exp.mtType {
			t.Errorf("item %d: expected type %s, got %s", i, exp.mtType, result[i].Type)
		}
		if result[i].Subtype != exp.subtype {
			t.Errorf("item %d: expected subtype %s, got %s", i, exp.subtype, result[i].Subtype)
		}
		if result[i].Quality != exp.q {
			t.Errorf("item %d: expected quality %f, got %f", i, exp.q, result[i].Quality)
		}
		if result[i].OrderIndex != i {
			t.Errorf("item %d: expected order index %d, got %d", i, i, result[i].OrderIndex)
		}
	}
}

func TestParseAcceptQualityBounds(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected float64
	}{
		{"negative clamped to 0", "application/json;q=-0.5", 0.0},
		{"above 1 clamped to 1", "application/json;q=1.5", 1.0},
		{"zero valid", "application/json;q=0", 0.0},
		{"one valid", "application/json;q=1", 1.0},
		{"invalid q uses default", "application/json;q=abc", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAccept(tt.header)
			if result[0].Quality != tt.expected {
				t.Errorf("expected quality %f, got %f", tt.expected, result[0].Quality)
			}
		})
	}
}

func TestParseAcceptWithParams(t *testing.T) {
	result := ParseAccept("application/json;charset=utf-8;q=0.9")
	if len(result) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(result))
	}
	if result[0].Quality != 0.9 {
		t.Errorf("expected quality 0.9, got %f", result[0].Quality)
	}
	if result[0].Params["charset"] != "utf-8" {
		t.Errorf("expected charset=utf-8, got %s", result[0].Params["charset"])
	}
}

func TestParseAcceptMalformed(t *testing.T) {
	result := ParseAccept("application/json, , text/html")
	if len(result) != 2 {
		t.Fatalf("expected 2 media types, got %d", len(result))
	}
}

func TestParseAcceptCaseInsensitive(t *testing.T) {
	result := ParseAccept("APPLICATION/JSON;Q=0.8")
	if len(result) != 1 {
		t.Fatalf("expected 1 media type, got %d", len(result))
	}
	if result[0].Type != "application" {
		t.Errorf("expected type 'application', got %s", result[0].Type)
	}
	if result[0].Subtype != "json" {
		t.Errorf("expected subtype 'json', got %s", result[0].Subtype)
	}
	if result[0].Quality != 0.8 {
		t.Errorf("expected quality 0.8, got %f", result[0].Quality)
	}
}

func TestNewNegotiator(t *testing.T) {
	n := NewNegotiator()
	if n == nil {
		t.Fatal("NewNegotiator returned nil")
	}
	formats := n.SupportedFormats()
	if len(formats) != 3 {
		t.Errorf("expected 3 formats, got %d", len(formats))
	}
}

func TestRegisterFormat(t *testing.T) {
	n := NewNegotiator()

	t.Run("nil format", func(t *testing.T) {
		err := n.RegisterFormat(nil)
		if err == nil {
			t.Error("expected error for nil format")
		}
	})

	t.Run("empty content type", func(t *testing.T) {
		err := n.RegisterFormat(&Format{
			ContentType: "",
			Marshal:     func(v interface{}) ([]byte, error) { return nil, nil },
		})
		if err == nil {
			t.Error("expected error for empty content type")
		}
	})

	t.Run("nil marshal function", func(t *testing.T) {
		err := n.RegisterFormat(&Format{
			ContentType: "text/plain",
			Marshal:     nil,
		})
		if err == nil {
			t.Error("expected error for nil marshal function")
		}
	})

	t.Run("valid format", func(t *testing.T) {
		err := n.RegisterFormat(&Format{
			ContentType: "text/plain",
			Marshal:     func(v interface{}) ([]byte, error) { return []byte("test"), nil },
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if _, ok := n.GetFormat("text/plain"); !ok {
			t.Error("format was not registered")
		}
	})
}

func TestSupportedFormats(t *testing.T) {
	n := NewNegotiator()
	formats := n.SupportedFormats()
	if len(formats) != 3 {
		t.Errorf("expected 3 formats, got %d", len(formats))
	}

	expected := []string{ContentTypeJSON, ContentTypeProtobuf, ContentTypeXML}
	for i, exp := range expected {
		if formats[i] != exp {
			t.Errorf("format %d: expected %s, got %s", i, exp, formats[i])
		}
	}
}

func TestGetFormat(t *testing.T) {
	n := NewNegotiator()

	t.Run("existing format", func(t *testing.T) {
		f, ok := n.GetFormat(ContentTypeJSON)
		if !ok {
			t.Error("expected to find json format")
		}
		if f.ContentType != ContentTypeJSON {
			t.Errorf("expected content type %s, got %s", ContentTypeJSON, f.ContentType)
		}
	})

	t.Run("case insensitive lookup", func(t *testing.T) {
		f, ok := n.GetFormat("APPLICATION/JSON")
		if !ok {
			t.Error("expected to find json format with case mismatch")
		}
		if f.ContentType != ContentTypeJSON {
			t.Errorf("expected content type %s, got %s", ContentTypeJSON, f.ContentType)
		}
	})

	t.Run("non-existing format", func(t *testing.T) {
		_, ok := n.GetFormat("text/plain")
		if ok {
			t.Error("expected not to find non-existing format")
		}
	})
}

func TestNegotiateSingleFormat(t *testing.T) {
	n := NewNegotiator()

	t.Run("json only", func(t *testing.T) {
		f, err := n.Negotiate("application/json")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ContentType != ContentTypeJSON {
			t.Errorf("expected %s, got %s", ContentTypeJSON, f.ContentType)
		}
	})

	t.Run("xml only", func(t *testing.T) {
		f, err := n.Negotiate("application/xml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ContentType != ContentTypeXML {
			t.Errorf("expected %s, got %s", ContentTypeXML, f.ContentType)
		}
	})

	t.Run("protobuf only", func(t *testing.T) {
		f, err := n.Negotiate("application/protobuf")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ContentType != ContentTypeProtobuf {
			t.Errorf("expected %s, got %s", ContentTypeProtobuf, f.ContentType)
		}
	})
}

func TestNegotiateQualityPriority(t *testing.T) {
	n := NewNegotiator()

	tests := []struct {
		name     string
		accept   string
		expected string
	}{
		{"higher q wins", "application/xml;q=0.9, application/json;q=0.5", ContentTypeXML},
		{"higher q wins 2", "application/json;q=0.8, application/xml;q=0.9", ContentTypeXML},
		{"higher q wildcard beats lower q specific", "application/xml;q=0.5, */*;q=0.8", ContentTypeJSON},
		{"same q specific beats wildcard", "application/xml;q=0.8, */*;q=0.8", ContentTypeXML},
		{"same q specific beats application/*", "application/*;q=1.0, application/json;q=1.0", ContentTypeJSON},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := n.Negotiate(tt.accept)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.ContentType != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, f.ContentType)
			}
		})
	}
}

func TestNegotiateOrderPriority(t *testing.T) {
	n := NewNegotiator()

	tests := []struct {
		name     string
		accept   string
		expected string
	}{
		{"same q first wins", "application/json;q=0.8, application/xml;q=0.8", ContentTypeJSON},
		{"same q first wins 2", "application/xml;q=0.8, application/json;q=0.8", ContentTypeXML},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := n.Negotiate(tt.accept)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.ContentType != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, f.ContentType)
			}
		})
	}
}

func TestNegotiateWildcards(t *testing.T) {
	n := NewNegotiator()

	t.Run("wildcard all picks default order", func(t *testing.T) {
		f, err := n.Negotiate("*/*")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil format")
		}
	})

	t.Run("application/* picks any application type", func(t *testing.T) {
		f, err := n.Negotiate("application/*")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil format")
		}
	})

	t.Run("empty accept uses wildcard", func(t *testing.T) {
		f, err := n.Negotiate("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil format")
		}
	})
}

func TestNegotiateNoMatch(t *testing.T) {
	n := NewNegotiator()

	_, err := n.Negotiate("text/html, image/png")
	if !errors.Is(err, ErrNoAcceptableFormat) {
		t.Errorf("expected ErrNoAcceptableFormat, got %v", err)
	}
}

func TestNegotiateQualityZero(t *testing.T) {
	n := NewNegotiator()

	_, err := n.Negotiate("application/json;q=0, application/xml;q=0")
	if !errors.Is(err, ErrNoAcceptableFormat) {
		t.Errorf("expected ErrNoAcceptableFormat for q=0, got %v", err)
	}
}

func TestNegotiateRequest(t *testing.T) {
	n := NewNegotiator()

	t.Run("nil request", func(t *testing.T) {
		f, err := n.NegotiateRequest(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil format")
		}
	})

	t.Run("request with accept header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "application/xml")
		f, err := n.NegotiateRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.ContentType != ContentTypeXML {
			t.Errorf("expected %s, got %s", ContentTypeXML, f.ContentType)
		}
	})

	t.Run("request without accept header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		f, err := n.NegotiateRequest(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Fatal("expected non-nil format")
		}
	})
}

func TestNegotiateWithDefault(t *testing.T) {
	n := NewNegotiator()

	t.Run("normal negotiation", func(t *testing.T) {
		result, err := n.NegotiateWithDefault("application/xml", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ContentType != ContentTypeXML {
			t.Errorf("expected %s, got %s", ContentTypeXML, result.ContentType)
		}
	})

	t.Run("no match with valid default", func(t *testing.T) {
		result, err := n.NegotiateWithDefault("text/html", ContentTypeJSON)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ContentType != ContentTypeJSON {
			t.Errorf("expected %s, got %s", ContentTypeJSON, result.ContentType)
		}
	})

	t.Run("no match with invalid default", func(t *testing.T) {
		_, err := n.NegotiateWithDefault("text/html", "text/plain")
		if !errors.Is(err, ErrNoAcceptableFormat) {
			t.Errorf("expected ErrNoAcceptableFormat, got %v", err)
		}
	})

	t.Run("no match without default", func(t *testing.T) {
		_, err := n.NegotiateWithDefault("text/html", "")
		if !errors.Is(err, ErrNoAcceptableFormat) {
			t.Errorf("expected ErrNoAcceptableFormat, got %v", err)
		}
	})
}

func TestMarshalJSON(t *testing.T) {
	user := testUser{ID: 1, Name: "Alice", Age: 30}
	data, err := marshalJSON(&user)
	if err != nil {
		t.Fatalf("marshalJSON failed: %v", err)
	}

	var result testUser
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.ID != user.ID || result.Name != user.Name || result.Age != user.Age {
		t.Errorf("data mismatch: expected %+v, got %+v", user, result)
	}
}

func TestMarshalXML(t *testing.T) {
	user := testUser{ID: 1, Name: "Alice", Age: 30}
	data, err := marshalXML(&user)
	if err != nil {
		t.Fatalf("marshalXML failed: %v", err)
	}

	var result testUser
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if result.ID != user.ID || result.Name != user.Name || result.Age != user.Age {
		t.Errorf("data mismatch: expected %+v, got %+v", user, result)
	}
}

func TestMarshalProtobuf(t *testing.T) {
	user := testUserPB{ID: 1, Name: "Alice", Age: 30}
	data, err := marshalProtobuf(&user)
	if err != nil {
		t.Fatalf("marshalProtobuf failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty protobuf data")
	}

	var result testUserPB
	ser := serialize.NewProtoBufSerializer()
	opts := serialize.DefaultOptions()
	opts.ZeroCopy = false
	if err := ser.Unmarshal(data, &result, opts); err != nil {
		t.Fatalf("protobuf unmarshal failed: %v", err)
	}
	if result.ID != user.ID || result.Name != user.Name || result.Age != user.Age {
		t.Errorf("protobuf round-trip mismatch: expected %+v, got %+v", user, result)
	}
}

func TestWriteResponse(t *testing.T) {
	n := NewNegotiator()
	user := testUser{ID: 1, Name: "Alice", Age: 30}

	t.Run("json response", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "application/json")

		err := n.WriteResponse(w, req, http.StatusOK, &user)
		if err != nil {
			t.Fatalf("WriteResponse failed: %v", err)
		}
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != ContentTypeJSON {
			t.Errorf("expected content type %s, got %s", ContentTypeJSON, ct)
		}

		var result testUser
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if result != user {
			t.Errorf("data mismatch: expected %+v, got %+v", user, result)
		}
	})

	t.Run("xml response", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "application/xml")

		err := n.WriteResponse(w, req, http.StatusOK, &user)
		if err != nil {
			t.Fatalf("WriteResponse failed: %v", err)
		}
		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != ContentTypeXML {
			t.Errorf("expected content type %s, got %s", ContentTypeXML, ct)
		}

		var result testUser
		if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if result != user {
			t.Errorf("data mismatch: expected %+v, got %+v", user, result)
		}
	})

	t.Run("nil response writer", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		err := n.WriteResponse(nil, req, http.StatusOK, &user)
		if !errors.Is(err, ErrNilResponseWriter) {
			t.Errorf("expected ErrNilResponseWriter, got %v", err)
		}
	})

	t.Run("406 not acceptable", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept", "text/html, image/png")

		err := n.WriteResponse(w, req, http.StatusOK, &user)
		if err != nil {
			t.Fatalf("WriteResponse failed: %v", err)
		}
		if w.Code != http.StatusNotAcceptable {
			t.Errorf("expected status %d, got %d", http.StatusNotAcceptable, w.Code)
		}
	})
}

func TestWriteResponseWithFormat(t *testing.T) {
	n := NewNegotiator()
	user := testUser{ID: 1, Name: "Bob", Age: 25}
	format, _ := n.GetFormat(ContentTypeJSON)

	t.Run("valid format", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := n.WriteResponseWithFormat(w, format, http.StatusCreated, &user)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != ContentTypeJSON {
			t.Errorf("expected content type %s, got %s", ContentTypeJSON, ct)
		}
	})

	t.Run("nil writer", func(t *testing.T) {
		err := n.WriteResponseWithFormat(nil, format, http.StatusOK, &user)
		if !errors.Is(err, ErrNilResponseWriter) {
			t.Errorf("expected ErrNilResponseWriter, got %v", err)
		}
	})

	t.Run("nil format", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := n.WriteResponseWithFormat(w, nil, http.StatusOK, &user)
		if err == nil {
			t.Error("expected error for nil format")
		}
	})
}

func TestWriteNotAcceptable(t *testing.T) {
	n := NewNegotiator()

	t.Run("normal case", func(t *testing.T) {
		w := httptest.NewRecorder()
		err := n.WriteNotAcceptable(w)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if w.Code != http.StatusNotAcceptable {
			t.Errorf("expected status %d, got %d", http.StatusNotAcceptable, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); ct != ContentTypeJSON {
			t.Errorf("expected content type %s, got %s", ContentTypeJSON, ct)
		}

		var result NotAcceptableResponse
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response failed: %v", err)
		}
		if result.Code != http.StatusNotAcceptable {
			t.Errorf("expected code %d, got %d", http.StatusNotAcceptable, result.Code)
		}
		if len(result.Formats) != 3 {
			t.Errorf("expected 3 supported formats, got %d", len(result.Formats))
		}
	})

	t.Run("nil writer", func(t *testing.T) {
		err := n.WriteNotAcceptable(nil)
		if !errors.Is(err, ErrNilResponseWriter) {
			t.Errorf("expected ErrNilResponseWriter, got %v", err)
		}
	})
}

func TestComplexAcceptHeaders(t *testing.T) {
	n := NewNegotiator()

	tests := []struct {
		name     string
		accept   string
		expected string
	}{
		{
			"browser-like accept header",
			"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
			ContentTypeXML,
		},
		{
			"api client prefers json",
			"application/json;q=1.0, application/xml;q=0.8, */*;q=0.1",
			ContentTypeJSON,
		},
		{
			"wildcard app with specific lower q",
			"application/*;q=0.5, application/json;q=0.3",
			ContentTypeJSON,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := n.Negotiate(tt.accept)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if f.ContentType != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, f.ContentType)
			}
		})
	}
}

func TestSerializationFailure(t *testing.T) {
	n := NewNegotiator()

	n.RegisterFormat(&Format{
		ContentType: "application/fail",
		Marshal: func(v interface{}) ([]byte, error) {
			return nil, errors.New("serialization failure")
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept", "application/fail")

	err := n.WriteResponse(w, req, http.StatusOK, "test data")
	if err == nil {
		t.Error("expected serialization error")
	}
	if !errors.Is(err, ErrSerialization) {
		t.Errorf("expected ErrSerialization, got %v", err)
	}
}

func TestParseAcceptOrderPreserved(t *testing.T) {
	header := "application/json, application/xml, application/protobuf"
	result := ParseAccept(header)

	expected := []string{"application/json", "application/xml", "application/protobuf"}
	for i, exp := range expected {
		if result[i].FullType() != exp {
			t.Errorf("item %d: expected %s, got %s", i, exp, result[i].FullType())
		}
		if result[i].OrderIndex != i {
			t.Errorf("item %d: expected order index %d, got %d", i, i, result[i].OrderIndex)
		}
	}
}

func TestMatchLevelPriority(t *testing.T) {
	n := NewNegotiator()

	f, err := n.Negotiate("*/*;q=1.0, application/*;q=1.0, application/json;q=1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ContentType != ContentTypeJSON {
		t.Errorf("expected exact match to win, got %s", f.ContentType)
	}

	f2, err := n.Negotiate("*/*;q=1.0, application/*;q=1.0")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	formats := n.SupportedFormats()
	found := false
	for _, ct := range formats {
		if f2.ContentType == ct {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected one of supported formats, got %s", f2.ContentType)
	}
}

func TestEdgeCases(t *testing.T) {
	n := NewNegotiator()

	t.Run("only commas", func(t *testing.T) {
		result := ParseAccept(",,,")
		if len(result) != 1 {
			t.Errorf("expected fallback wildcard, got %d items", len(result))
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		result := ParseAccept("   ")
		if len(result) != 1 {
			t.Errorf("expected fallback wildcard, got %d items", len(result))
		}
	})

	t.Run("no subtype", func(t *testing.T) {
		result := ParseAccept("application")
		if len(result) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result))
		}
		if result[0].Type != "application" {
			t.Errorf("expected type application, got %s", result[0].Type)
		}
		if result[0].Subtype != "*" {
			t.Errorf("expected subtype *, got %s", result[0].Subtype)
		}
	})

	t.Run("param without value", func(t *testing.T) {
		result := ParseAccept("application/json;level")
		if len(result) != 1 {
			t.Fatalf("expected 1 item, got %d", len(result))
		}
		if _, ok := result[0].Params["level"]; !ok {
			t.Error("expected 'level' param to exist")
		}
	})

	t.Run("all q=0 except wildcard", func(t *testing.T) {
		f, err := n.Negotiate("application/json;q=0, application/xml;q=0, */*;q=0.5")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f == nil {
			t.Error("expected non-nil format via wildcard")
		}
	})
}
