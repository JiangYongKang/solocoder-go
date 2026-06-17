package serialize

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type TestStruct struct {
	ID      int      `serialize:"id"`
	Name    string   `serialize:"name"`
	Age     int      `serialize:"age"`
	Data    []byte   `serialize:"data"`
	Tags    []string `serialize:"tags"`
	Active  bool     `serialize:"active"`
	Score   float64  `serialize:"score"`
	private string
}

type TestStructV1 struct {
	ID   int    `serialize:"id"`
	Name string `serialize:"name"`
}

type TestStructV2 struct {
	ID      int    `serialize:"id"`
	Name    string `serialize:"name"`
	Age     int    `serialize:"age"`
	Address string `serialize:"address"`
}

type TestStructV3 struct {
	ID     int      `serialize:"id,protobuf:2"`
	Name   string   `serialize:"name,protobuf:3"`
	Age    int      `serialize:"age,protobuf:4"`
	Data   []byte   `serialize:"data,protobuf:5"`
	Tags   []string `serialize:"tags,protobuf:6"`
	Active bool     `serialize:"active,protobuf:7"`
}

type NestedStruct struct {
	Inner TestStruct `serialize:"inner"`
	Value string     `serialize:"value"`
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if !opts.ZeroCopy {
		t.Error("expected ZeroCopy to be true by default")
	}
	if !opts.SkipUnknownFields {
		t.Error("expected SkipUnknownFields to be true by default")
	}
	if opts.UnknownFieldBehavior != SkipUnknownField {
		t.Error("expected UnknownFieldBehavior to be SkipUnknownField by default")
	}
	if opts.Version != 1 {
		t.Error("expected Version to be 1 by default")
	}
	if opts.StrictMode {
		t.Error("expected StrictMode to be false by default")
	}
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if len(r.List()) != 0 {
		t.Errorf("expected 0 serializers, got %d", len(r.List()))
	}
}

func TestRegisterSerializer(t *testing.T) {
	r := NewRegistry()
	s := NewJSONSerializer()

	err := r.Register("json", s)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if len(r.List()) != 1 {
		t.Errorf("expected 1 serializer, got %d", len(r.List()))
	}

	err = r.Register("", s)
	if err == nil {
		t.Error("expected error for empty name")
	}
	if !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}

	err = r.Register("nil", nil)
	if err == nil {
		t.Error("expected error for nil serializer")
	}
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestUnregisterSerializer(t *testing.T) {
	r := NewRegistry()
	s := NewJSONSerializer()
	r.Register("json", s)

	r.Unregister("json")
	if len(r.List()) != 0 {
		t.Errorf("expected 0 serializers after unregister, got %d", len(r.List()))
	}

	_, err := r.Default()
	if err == nil {
		t.Error("expected error when no default serializer")
	}
}

func TestGetSerializer(t *testing.T) {
	r := NewRegistry()
	s := NewJSONSerializer()
	r.Register("json", s)

	got, err := r.Get("json")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got != s {
		t.Error("Get returned wrong serializer")
	}

	_, err = r.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent serializer")
	}
	if !errors.Is(err, ErrSerializerNotFound) {
		t.Errorf("expected ErrSerializerNotFound, got %v", err)
	}
}

func TestGetByContentType(t *testing.T) {
	r := NewRegistry()
	jsonSer := NewJSONSerializer()
	msgSer := NewMsgPackSerializer()
	r.Register("json", jsonSer)
	r.Register("msgpack", msgSer)

	got, err := r.GetByContentType(ContentTypeJSON)
	if err != nil {
		t.Fatalf("GetByContentType failed: %v", err)
	}
	if got != jsonSer {
		t.Error("GetByContentType returned wrong serializer")
	}

	_, err = r.GetByContentType("invalid")
	if err == nil {
		t.Error("expected error for invalid content type")
	}
}

func TestDefaultSerializer(t *testing.T) {
	r := NewRegistry()
	jsonSer := NewJSONSerializer()
	msgSer := NewMsgPackSerializer()

	r.Register("json", jsonSer)
	def, err := r.Default()
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
	if def != jsonSer {
		t.Error("first registered should be default")
	}

	r.Register("msgpack", msgSer)
	err = r.SetDefault("msgpack")
	if err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}
	def, err = r.Default()
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
	if def != msgSer {
		t.Error("default should be msgpack after SetDefault")
	}

	err = r.SetDefault("nonexistent")
	if err == nil {
		t.Error("expected error for setting nonexistent as default")
	}

	r.Unregister("msgpack")
	def, err = r.Default()
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
	if def != jsonSer {
		t.Error("should fall back to next available serializer")
	}
}

func TestListSerializers(t *testing.T) {
	r := NewRegistry()
	r.Register("json", NewJSONSerializer())
	r.Register("msgpack", NewMsgPackSerializer())
	r.Register("protobuf", NewProtoBufSerializer())

	list := r.List()
	if len(list) != 3 {
		t.Errorf("expected 3 serializers, got %d", len(list))
	}
}

func TestGlobalFunctions(t *testing.T) {
	_, err := Get("json")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	_, err = GetByContentType(ContentTypeJSON)
	if err != nil {
		t.Fatalf("GetByContentType failed: %v", err)
	}

	list := List()
	if len(list) < 3 {
		t.Errorf("expected at least 3 serializers, got %d", len(list))
	}

	err = SetDefault("json")
	if err != nil {
		t.Fatalf("SetDefault failed: %v", err)
	}

	_, err = Default()
	if err != nil {
		t.Fatalf("Default failed: %v", err)
	}
}

func TestJSONMarshalUnmarshal(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	original := TestStruct{
		ID:     1,
		Name:   "Test",
		Age:    30,
		Data:   []byte("hello"),
		Tags:   []string{"a", "b", "c"},
		Active: true,
		Score:  95.5,
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	var result TestStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", result.ID, original.ID)
	}
	if result.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", result.Name, original.Name)
	}
	if result.Age != original.Age {
		t.Errorf("Age mismatch: got %d, want %d", result.Age, original.Age)
	}
	if !bytes.Equal(result.Data, original.Data) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, original.Data)
	}
	if !reflect.DeepEqual(result.Tags, original.Tags) {
		t.Errorf("Tags mismatch: got %v, want %v", result.Tags, original.Tags)
	}
	if result.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", result.Active, original.Active)
	}
	if result.Score != original.Score {
		t.Errorf("Score mismatch: got %f, want %f", result.Score, original.Score)
	}
}

func TestJSONMarshalNil(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	_, err := s.Marshal(nil, opts)
	if err == nil {
		t.Error("expected error for nil input")
	}
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}
}

func TestJSONUnmarshalNil(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	err := s.Unmarshal([]byte("{}"), nil, opts)
	if err == nil {
		t.Error("expected error for nil target")
	}
	if !errors.Is(err, ErrUnmarshalNil) {
		t.Errorf("expected ErrUnmarshalNil, got %v", err)
	}

	var target *TestStruct
	err = s.Unmarshal([]byte("{}"), target, opts)
	if err == nil {
		t.Error("expected error for nil pointer target")
	}
}

func TestJSONUnmarshalEmpty(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	var result TestStruct
	err := s.Unmarshal([]byte{}, &result, opts)
	if err != nil {
		t.Errorf("expected no error for empty data, got %v", err)
	}
}

func TestJSONUnknownFields(t *testing.T) {
	s := NewJSONSerializer()

	jsonWithExtra := `{"id":1,"name":"Test","extra_field":"ignore_me","another":42}`

	optsSkip := DefaultOptions()
	optsSkip.UnknownFieldBehavior = SkipUnknownField
	var result TestStruct
	err := s.Unmarshal([]byte(jsonWithExtra), &result, optsSkip)
	if err != nil {
		t.Fatalf("Unmarshal with skip failed: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("ID mismatch: got %d, want 1", result.ID)
	}
	if result.Name != "Test" {
		t.Errorf("Name mismatch: got %s, want Test", result.Name)
	}

	optsError := DefaultOptions()
	optsError.UnknownFieldBehavior = ReturnUnknownFieldError
	var result2 TestStruct
	err = s.Unmarshal([]byte(jsonWithExtra), &result2, optsError)
	if err == nil {
		t.Error("expected error for unknown fields")
	}
	if !errors.Is(err, ErrUnknownField) {
		t.Errorf("expected ErrUnknownField, got %v", err)
	}
}

func TestJSONMissingFields(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	jsonPartial := `{"id":1}`

	var result TestStruct
	err := s.Unmarshal([]byte(jsonPartial), &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.ID != 1 {
		t.Errorf("ID mismatch: got %d, want 1", result.ID)
	}
	if result.Name != "" {
		t.Errorf("expected empty Name for missing field, got %s", result.Name)
	}
	if result.Age != 0 {
		t.Errorf("expected zero Age for missing field, got %d", result.Age)
	}
	if result.Data != nil {
		t.Errorf("expected nil Data for missing field, got %v", result.Data)
	}
	if result.Active != false {
		t.Errorf("expected false Active for missing field, got %v", result.Active)
	}
}

func TestJSONVersionControl(t *testing.T) {
	s := NewJSONSerializer()

	original := TestStructV1{ID: 1, Name: "Test"}

	optsV2 := DefaultOptions()
	optsV2.Version = 2
	data, err := s.Marshal(&original, optsV2)
	if err != nil {
		t.Fatalf("Marshal with version failed: %v", err)
	}

	if !bytes.Contains(data, []byte("__version__")) {
		t.Error("expected version field in marshaled data")
	}

	optsStrict := DefaultOptions()
	optsStrict.Version = 2
	optsStrict.StrictMode = true
	var result TestStructV1
	err = s.Unmarshal(data, &result, optsStrict)
	if err != nil {
		t.Fatalf("Unmarshal with matching version failed: %v", err)
	}

	optsStrictWrong := DefaultOptions()
	optsStrictWrong.Version = 3
	optsStrictWrong.StrictMode = true
	var result2 TestStructV1
	err = s.Unmarshal(data, &result2, optsStrictWrong)
	if err == nil {
		t.Error("expected error for version mismatch in strict mode")
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch, got %v", err)
	}

	optsRelaxed := DefaultOptions()
	optsRelaxed.Version = 3
	optsRelaxed.StrictMode = false
	var result3 TestStructV1
	err = s.Unmarshal(data, &result3, optsRelaxed)
	if err != nil {
		t.Errorf("expected no error in non-strict mode, got %v", err)
	}
}

func TestJSONZeroCopy(t *testing.T) {
	s := NewJSONSerializer()
	original := TestStruct{Name: "Hello World", Data: []byte("test data")}

	opts := DefaultOptions()
	opts.ZeroCopy = true
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result TestStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Name != "Hello World" {
		t.Errorf("Name mismatch: got %s, want Hello World", result.Name)
	}
	if !bytes.Equal(result.Data, []byte("test data")) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, []byte("test data"))
	}

	optsNoCopy := DefaultOptions()
	optsNoCopy.ZeroCopy = false
	var result2 TestStruct
	err = s.Unmarshal(data, &result2, optsNoCopy)
	if err != nil {
		t.Fatalf("Unmarshal without zero-copy failed: %v", err)
	}
	if result2.Name != "Hello World" {
		t.Errorf("Name mismatch without zero-copy: got %s, want Hello World", result2.Name)
	}
}

func TestJSONNestedStruct(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	original := NestedStruct{
		Inner: TestStruct{ID: 1, Name: "Inner"},
		Value: "Outer",
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result NestedStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Inner.ID != original.Inner.ID {
		t.Errorf("Inner ID mismatch: got %d, want %d", result.Inner.ID, original.Inner.ID)
	}
	if result.Inner.Name != original.Inner.Name {
		t.Errorf("Inner Name mismatch: got %s, want %s", result.Inner.Name, original.Inner.Name)
	}
	if result.Value != original.Value {
		t.Errorf("Value mismatch: got %s, want %s", result.Value, original.Value)
	}
}

func TestMsgPackMarshalUnmarshal(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	original := TestStruct{
		ID:     1,
		Name:   "Test",
		Age:    30,
		Data:   []byte("hello"),
		Tags:   []string{"a", "b", "c"},
		Active: true,
		Score:  95.5,
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	var result TestStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", result.ID, original.ID)
	}
	if result.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", result.Name, original.Name)
	}
	if result.Age != original.Age {
		t.Errorf("Age mismatch: got %d, want %d", result.Age, original.Age)
	}
	if !bytes.Equal(result.Data, original.Data) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, original.Data)
	}
	if result.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", result.Active, original.Active)
	}
	if result.Score != original.Score {
		t.Errorf("Score mismatch: got %f, want %f", result.Score, original.Score)
	}
}

func TestMsgPackNilInput(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	data, err := s.Marshal(nil, opts)
	if err != nil {
		t.Fatalf("Marshal nil should not error, got %v", err)
	}
	if len(data) != 1 || data[0] != mpNil {
		t.Error("Marshal nil should return nil marker")
	}

	var result TestStruct
	err = s.Unmarshal([]byte{mpNil}, &result, opts)
	if err != nil {
		t.Errorf("Unmarshal nil should not error, got %v", err)
	}
}

func TestMsgPackUnknownFields(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	original := TestStructV2{
		ID:      1,
		Name:    "Test",
		Age:     30,
		Address: "123 Street",
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	optsSkip := DefaultOptions()
	optsSkip.UnknownFieldBehavior = SkipUnknownField
	var result TestStructV1
	err = s.Unmarshal(data, &result, optsSkip)
	if err != nil {
		t.Fatalf("Unmarshal with skip failed: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("ID mismatch: got %d, want 1", result.ID)
	}
	if result.Name != "Test" {
		t.Errorf("Name mismatch: got %s, want Test", result.Name)
	}

	optsError := DefaultOptions()
	optsError.UnknownFieldBehavior = ReturnUnknownFieldError
	var result2 TestStructV1
	err = s.Unmarshal(data, &result2, optsError)
	if err == nil {
		t.Error("expected error for unknown fields")
	}
	if !errors.Is(err, ErrUnknownField) {
		t.Errorf("expected ErrUnknownField, got %v", err)
	}
}

func TestMsgPackVersionControl(t *testing.T) {
	s := NewMsgPackSerializer()

	original := TestStructV1{ID: 1, Name: "Test"}

	optsV2 := DefaultOptions()
	optsV2.Version = 2
	data, err := s.Marshal(&original, optsV2)
	if err != nil {
		t.Fatalf("Marshal with version failed: %v", err)
	}

	optsStrict := DefaultOptions()
	optsStrict.Version = 2
	optsStrict.StrictMode = true
	var result TestStructV1
	err = s.Unmarshal(data, &result, optsStrict)
	if err != nil {
		t.Fatalf("Unmarshal with matching version failed: %v", err)
	}

	optsStrictWrong := DefaultOptions()
	optsStrictWrong.Version = 3
	optsStrictWrong.StrictMode = true
	var result2 TestStructV1
	err = s.Unmarshal(data, &result2, optsStrictWrong)
	if err == nil {
		t.Error("expected error for version mismatch in strict mode")
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch, got %v", err)
	}
}

func TestMsgPackZeroCopy(t *testing.T) {
	s := NewMsgPackSerializer()
	original := TestStruct{Name: "Hello World", Data: []byte("test data")}

	opts := DefaultOptions()
	opts.ZeroCopy = true
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result TestStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Name != "Hello World" {
		t.Errorf("Name mismatch: got %s, want Hello World", result.Name)
	}
	if !bytes.Equal(result.Data, []byte("test data")) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, []byte("test data"))
	}

	optsNoCopy := DefaultOptions()
	optsNoCopy.ZeroCopy = false
	var result2 TestStruct
	err = s.Unmarshal(data, &result2, optsNoCopy)
	if err != nil {
		t.Fatalf("Unmarshal without zero-copy failed: %v", err)
	}
	if result2.Name != "Hello World" {
		t.Errorf("Name mismatch without zero-copy: got %s, want Hello World", result2.Name)
	}
}

func TestMsgPackEdgeCases(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	t.Run("bool true", func(t *testing.T) {
		type BoolContainer struct {
			Value bool `serialize:"value"`
		}
		orig := BoolContainer{Value: true}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res BoolContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Bool mismatch: got %v, want %v", res.Value, orig.Value)
		}
	})

	t.Run("bool false", func(t *testing.T) {
		type BoolContainer struct {
			Value bool `serialize:"value"`
		}
		orig := BoolContainer{Value: false}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res BoolContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Bool mismatch: got %v, want %v", res.Value, orig.Value)
		}
	})

	t.Run("small int", func(t *testing.T) {
		type IntContainer struct {
			Value int `serialize:"value"`
		}
		orig := IntContainer{Value: 42}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res IntContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Int mismatch: got %d, want %d", res.Value, orig.Value)
		}
	})

	t.Run("negative int", func(t *testing.T) {
		type IntContainer struct {
			Value int `serialize:"value"`
		}
		orig := IntContainer{Value: -42}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res IntContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Int mismatch: got %d, want %d", res.Value, orig.Value)
		}
	})

	t.Run("large int", func(t *testing.T) {
		type Int64Container struct {
			Value int64 `serialize:"value"`
		}
		orig := Int64Container{Value: 1234567890}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res Int64Container
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Int64 mismatch: got %d, want %d", res.Value, orig.Value)
		}
	})

	t.Run("large negative int", func(t *testing.T) {
		type Int64Container struct {
			Value int64 `serialize:"value"`
		}
		orig := Int64Container{Value: -1234567890}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res Int64Container
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Int64 mismatch: got %d, want %d", res.Value, orig.Value)
		}
	})

	t.Run("float32", func(t *testing.T) {
		type Float32Container struct {
			Value float32 `serialize:"value"`
		}
		orig := Float32Container{Value: float32(3.14)}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res Float32Container
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Float32 mismatch: got %v, want %v", res.Value, orig.Value)
		}
	})

	t.Run("float64", func(t *testing.T) {
		type Float64Container struct {
			Value float64 `serialize:"value"`
		}
		orig := Float64Container{Value: 3.1415926535}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res Float64Container
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Float64 mismatch: got %v, want %v", res.Value, orig.Value)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		type StringContainer struct {
			Value string `serialize:"value"`
		}
		orig := StringContainer{Value: ""}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res StringContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("String mismatch: got %q, want %q", res.Value, orig.Value)
		}
	})

	t.Run("long string", func(t *testing.T) {
		type StringContainer struct {
			Value string `serialize:"value"`
		}
		longStr := strings.Repeat("x", 1000)
		orig := StringContainer{Value: longStr}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res StringContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if res.Value != orig.Value {
			t.Errorf("Long string mismatch: length got %d, want %d", len(res.Value), len(orig.Value))
		}
	})

	t.Run("byte slice", func(t *testing.T) {
		type BytesContainer struct {
			Value []byte `serialize:"value"`
		}
		orig := BytesContainer{Value: []byte("binary data")}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res BytesContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if !bytes.Equal(res.Value, orig.Value) {
			t.Errorf("Byte slice mismatch: got %v, want %v", res.Value, orig.Value)
		}
	})

	t.Run("nil bytes", func(t *testing.T) {
		type BytesContainer struct {
			Value []byte `serialize:"value"`
		}
		orig := BytesContainer{Value: nil}
		data, err := s.Marshal(&orig, opts)
		if err != nil {
			t.Fatalf("Marshal failed: %v", err)
		}
		var res BytesContainer
		err = s.Unmarshal(data, &res, opts)
		if err != nil {
			t.Fatalf("Unmarshal failed: %v", err)
		}
		if len(res.Value) != 0 {
			t.Errorf("Nil bytes mismatch: got length %d, want 0", len(res.Value))
		}
	})
}

func TestMsgPackInvalidData(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	var result TestStruct

	err := s.Unmarshal([]byte{0xc1}, &result, opts)
	if err == nil {
		t.Error("expected error for invalid code")
	}
	if !errors.Is(err, ErrInvalidFormat) {
		t.Errorf("expected ErrInvalidFormat, got %v", err)
	}

	err = s.Unmarshal([]byte{mpUint8}, &result, opts)
	if err == nil {
		t.Error("expected error for truncated data")
	}
}

func TestProtoBufMarshalUnmarshal(t *testing.T) {
	s := NewProtoBufSerializer()
	opts := DefaultOptions()

	original := TestStructV3{
		ID:     1,
		Name:   "Test",
		Age:    30,
		Data:   []byte("hello"),
		Tags:   []string{"a", "b"},
		Active: true,
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("Marshal returned empty data")
	}

	var result TestStructV3
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.ID != original.ID {
		t.Errorf("ID mismatch: got %d, want %d", result.ID, original.ID)
	}
	if result.Name != original.Name {
		t.Errorf("Name mismatch: got %s, want %s", result.Name, original.Name)
	}
	if result.Age != original.Age {
		t.Errorf("Age mismatch: got %d, want %d", result.Age, original.Age)
	}
	if !bytes.Equal(result.Data, original.Data) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, original.Data)
	}
	if result.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", result.Active, original.Active)
	}
}

func TestProtoBufNilInput(t *testing.T) {
	s := NewProtoBufSerializer()
	opts := DefaultOptions()

	_, err := s.Marshal(nil, opts)
	if err == nil {
		t.Error("expected error for nil input")
	}
	if !errors.Is(err, ErrNilInput) {
		t.Errorf("expected ErrNilInput, got %v", err)
	}

	type NonStruct struct {
		Value int
	}
	_, err = s.Marshal(42, opts)
	if err == nil {
		t.Error("expected error for non-struct")
	}
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}

	err = s.Unmarshal([]byte{}, nil, opts)
	if err == nil {
		t.Error("expected error for nil target")
	}
}

func TestProtoBufUnknownFields(t *testing.T) {
	s := NewProtoBufSerializer()
	opts := DefaultOptions()

	original := TestStructV3{
		ID:   1,
		Name: "Test",
		Age:  30,
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	type SmallStruct struct {
		ID   int    `serialize:"id,protobuf:2"`
		Name string `serialize:"name,protobuf:3"`
	}

	optsSkip := DefaultOptions()
	optsSkip.UnknownFieldBehavior = SkipUnknownField
	var result SmallStruct
	err = s.Unmarshal(data, &result, optsSkip)
	if err != nil {
		t.Fatalf("Unmarshal with skip failed: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("ID mismatch: got %d, want 1", result.ID)
	}
	if result.Name != "Test" {
		t.Errorf("Name mismatch: got %s, want Test", result.Name)
	}

	optsError := DefaultOptions()
	optsError.UnknownFieldBehavior = ReturnUnknownFieldError
	var result2 SmallStruct
	err = s.Unmarshal(data, &result2, optsError)
	if err == nil {
		t.Error("expected error for unknown fields")
	}
	if !errors.Is(err, ErrUnknownField) {
		t.Errorf("expected ErrUnknownField, got %v", err)
	}
}

func TestProtoBufVersionControl(t *testing.T) {
	s := NewProtoBufSerializer()

	original := TestStructV3{ID: 1, Name: "Test"}

	optsV2 := DefaultOptions()
	optsV2.Version = 2
	data, err := s.Marshal(&original, optsV2)
	if err != nil {
		t.Fatalf("Marshal with version failed: %v", err)
	}

	optsStrict := DefaultOptions()
	optsStrict.Version = 2
	optsStrict.StrictMode = true
	var result TestStructV3
	err = s.Unmarshal(data, &result, optsStrict)
	if err != nil {
		t.Fatalf("Unmarshal with matching version failed: %v", err)
	}

	optsStrictWrong := DefaultOptions()
	optsStrictWrong.Version = 3
	optsStrictWrong.StrictMode = true
	var result2 TestStructV3
	err = s.Unmarshal(data, &result2, optsStrictWrong)
	if err == nil {
		t.Error("expected error for version mismatch in strict mode")
	}
	if !errors.Is(err, ErrVersionMismatch) {
		t.Errorf("expected ErrVersionMismatch, got %v", err)
	}
}

func TestProtoBufZeroCopy(t *testing.T) {
	s := NewProtoBufSerializer()
	original := TestStructV3{Name: "Hello World", Data: []byte("test data")}

	opts := DefaultOptions()
	opts.ZeroCopy = true
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result TestStructV3
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Name != "Hello World" {
		t.Errorf("Name mismatch: got %s, want Hello World", result.Name)
	}
	if !bytes.Equal(result.Data, []byte("test data")) {
		t.Errorf("Data mismatch: got %v, want %v", result.Data, []byte("test data"))
	}
}

func TestProtoBufEdgeCases(t *testing.T) {
	s := NewProtoBufSerializer()
	opts := DefaultOptions()

	type EdgeStruct struct {
		BoolVal   bool    `serialize:"bool,protobuf:2"`
		IntVal    int     `serialize:"int,protobuf:3"`
		Int32Val  int32   `serialize:"int32,protobuf:4"`
		Int64Val  int64   `serialize:"int64,protobuf:5"`
		UintVal   uint    `serialize:"uint,protobuf:6"`
		Uint32Val uint32  `serialize:"uint32,protobuf:7"`
		Uint64Val uint64  `serialize:"uint64,protobuf:8"`
		Float32   float32 `serialize:"float32,protobuf:9"`
		Float64   float64 `serialize:"float64,protobuf:10"`
	}

	original := EdgeStruct{
		BoolVal:   true,
		IntVal:    -42,
		Int32Val:  -12345,
		Int64Val:  -1234567890,
		UintVal:   42,
		Uint32Val:  12345,
		Uint64Val:  1234567890,
		Float32:   3.14,
		Float64:   2.71828,
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result EdgeStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.BoolVal != original.BoolVal {
		t.Errorf("BoolVal mismatch")
	}
	if result.IntVal != original.IntVal {
		t.Errorf("IntVal mismatch")
	}
	if result.Int32Val != original.Int32Val {
		t.Errorf("Int32Val mismatch")
	}
	if result.Int64Val != original.Int64Val {
		t.Errorf("Int64Val mismatch")
	}
	if result.UintVal != original.UintVal {
		t.Errorf("UintVal mismatch")
	}
	if result.Uint32Val != original.Uint32Val {
		t.Errorf("Uint32Val mismatch")
	}
	if result.Uint64Val != original.Uint64Val {
		t.Errorf("Uint64Val mismatch")
	}
	if result.Float32 != original.Float32 {
		t.Errorf("Float32 mismatch")
	}
	if result.Float64 != original.Float64 {
		t.Errorf("Float64 mismatch")
	}
}

func TestProtoBufZeroValues(t *testing.T) {
	s := NewProtoBufSerializer()
	opts := DefaultOptions()
	opts.Version = 0

	type ZeroStruct struct {
		ID     int    `serialize:"id,protobuf:2"`
		Name   string `serialize:"name,protobuf:3"`
		Active bool   `serialize:"active,protobuf:4"`
	}

	original := ZeroStruct{ID: 0, Name: "", Active: false}
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty data for zero values, got %d bytes", len(data))
	}

	originalWithID := ZeroStruct{ID: 42, Name: "", Active: false}
	data, err = s.Marshal(&originalWithID, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data for non-zero ID")
	}

	var result ZeroStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result.ID != 42 {
		t.Errorf("ID mismatch: got %d, want 42", result.ID)
	}
	if result.Name != "" {
		t.Errorf("Name should be zero value, got %s", result.Name)
	}
	if result.Active != false {
		t.Errorf("Active should be zero value, got %v", result.Active)
	}
}

func TestGlobalMarshalUnmarshal(t *testing.T) {
	opts := DefaultOptions()

	original := TestStruct{
		ID:   1,
		Name: "Test",
	}

	data, err := Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result TestStruct
	err = Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.ID != original.ID || result.Name != original.Name {
		t.Errorf("Mismatch: got %+v, want %+v", result, original)
	}
}

func TestMarshalWith(t *testing.T) {
	opts := DefaultOptions()
	original := TestStruct{ID: 1, Name: "Test"}

	formats := []string{"json", "msgpack", "protobuf"}

	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			data, err := MarshalWith(format, &original, opts)
			if err != nil {
				t.Fatalf("MarshalWith %s failed: %v", format, err)
			}

			var result TestStruct
			err = UnmarshalWith(format, data, &result, opts)
			if err != nil {
				t.Fatalf("UnmarshalWith %s failed: %v", format, err)
			}

			if result.ID != original.ID || result.Name != original.Name {
				t.Errorf("Mismatch for %s: got %+v, want %+v", format, result, original)
			}
		})
	}

	_, err := MarshalWith("nonexistent", &original, opts)
	if err == nil {
		t.Error("expected error for nonexistent format")
	}

	err = UnmarshalWith("nonexistent", []byte{}, &TestStruct{}, opts)
	if err == nil {
		t.Error("expected error for nonexistent format")
	}
}

func TestZeroCopyHelper(t *testing.T) {
	testBytes := []byte("hello world")

	str := zeroCopyString(testBytes)
	if str != "hello world" {
		t.Errorf("zeroCopyString mismatch: got %s, want hello world", str)
	}

	emptyStr := zeroCopyString([]byte{})
	if emptyStr != "" {
		t.Errorf("zeroCopyString empty mismatch: got %s, want empty", emptyStr)
	}

	b := zeroCopyBytes("test")
	if !bytes.Equal(b, []byte("test")) {
		t.Errorf("zeroCopyBytes mismatch: got %v, want %v", b, []byte("test"))
	}

	emptyB := zeroCopyBytes("")
	if emptyB != nil {
		t.Errorf("zeroCopyBytes empty should be nil, got %v", emptyB)
	}
}

func TestSetFieldValue(t *testing.T) {
	type Target struct {
		IntVal    int
		StringVal string
		BoolVal   bool
		FloatVal  float64
	}

	var tgt Target
	rv := reflect.ValueOf(&tgt).Elem()

	err := setFieldValue(rv.FieldByName("IntVal"), 42)
	if err != nil {
		t.Errorf("setFieldValue int failed: %v", err)
	}
	if tgt.IntVal != 42 {
		t.Errorf("IntVal mismatch: got %d, want 42", tgt.IntVal)
	}

	err = setFieldValue(rv.FieldByName("StringVal"), "hello")
	if err != nil {
		t.Errorf("setFieldValue string failed: %v", err)
	}
	if tgt.StringVal != "hello" {
		t.Errorf("StringVal mismatch: got %s, want hello", tgt.StringVal)
	}

	err = setFieldValue(rv.FieldByName("BoolVal"), true)
	if err != nil {
		t.Errorf("setFieldValue bool failed: %v", err)
	}
	if tgt.BoolVal != true {
		t.Errorf("BoolVal mismatch: got %v, want true", tgt.BoolVal)
	}

	err = setFieldValue(rv.FieldByName("FloatVal"), 3.14)
	if err != nil {
		t.Errorf("setFieldValue float failed: %v", err)
	}
	if tgt.FloatVal != 3.14 {
		t.Errorf("FloatVal mismatch: got %f, want 3.14", tgt.FloatVal)
	}

	err = setFieldValue(rv.FieldByName("FloatVal"), nil)
	if err != nil {
		t.Errorf("setFieldValue nil failed: %v", err)
	}
	if tgt.FloatVal != 0 {
		t.Errorf("FloatVal should be zero after nil, got %f", tgt.FloatVal)
	}

	type Private struct {
		privateField int
	}
	var priv Private
	privRV := reflect.ValueOf(&priv).Elem()
	err = setFieldValue(privRV.FieldByName("privateField"), 42)
	if err == nil {
		t.Error("expected error for unexported field")
	}
}

func TestConcurrentRegistryAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup
	iterations := 100

	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			s := NewJSONSerializer()
			name := fmt.Sprintf("json-%d", i)
			r.Register(name, s)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			r.List()
			if i%10 == 0 {
				name := fmt.Sprintf("json-%d", i)
				r.Get(name)
			}
		}
	}()

	wg.Wait()

	list := r.List()
	if len(list) != iterations {
		t.Errorf("expected %d serializers, got %d", iterations, len(list))
	}
}

func TestConcurrentSerialize(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()
	var wg sync.WaitGroup
	iterations := 100

	results := make([]TestStruct, iterations)
	errors := make([]error, iterations)

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			original := TestStruct{ID: idx, Name: fmt.Sprintf("Test-%d", idx)}
			data, err := s.Marshal(&original, opts)
			if err != nil {
				errors[idx] = err
				return
			}
			var result TestStruct
			err = s.Unmarshal(data, &result, opts)
			if err != nil {
				errors[idx] = err
				return
			}
			results[idx] = result
		}(i)
	}

	wg.Wait()

	for i := 0; i < iterations; i++ {
		if errors[i] != nil {
			t.Errorf("iteration %d failed: %v", i, errors[i])
		}
		if results[i].ID != i {
			t.Errorf("iteration %d ID mismatch: got %d, want %d", i, results[i].ID, i)
		}
	}
}

func TestSerializerNameAndContentType(t *testing.T) {
	tests := []struct {
		serializer    Serializer
		expectedName  string
		expectedCT    string
	}{
		{NewJSONSerializer(), "json", ContentTypeJSON},
		{NewMsgPackSerializer(), "msgpack", ContentTypeMsgPack},
		{NewProtoBufSerializer(), "protobuf", ContentTypeProtobuf},
	}

	for _, tt := range tests {
		t.Run(tt.expectedName, func(t *testing.T) {
			if tt.serializer.Name() != tt.expectedName {
				t.Errorf("Name mismatch: got %s, want %s", tt.serializer.Name(), tt.expectedName)
			}
			if tt.serializer.ContentType() != tt.expectedCT {
				t.Errorf("ContentType mismatch: got %s, want %s", tt.serializer.ContentType(), tt.expectedCT)
			}
		})
	}
}

func TestInvalidJSON(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	var result TestStruct
	err := s.Unmarshal([]byte("{invalid json}"), &result, opts)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestCaseInsensitiveFields(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	jsonData := `{"ID":1,"NAME":"Test","Age":30}`
	var result TestStruct
	err := s.Unmarshal([]byte(jsonData), &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("ID mismatch: got %d, want 1", result.ID)
	}
	if result.Name != "Test" {
		t.Errorf("Name mismatch: got %s, want Test", result.Name)
	}
	if result.Age != 30 {
		t.Errorf("Age mismatch: got %d, want 30", result.Age)
	}
}

func TestTagIgnore(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	type TaggedStruct struct {
		Visible   string `serialize:"visible"`
		Invisible string `serialize:"-"`
	}

	original := TaggedStruct{Visible: "yes", Invisible: "no"}
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if bytes.Contains(data, []byte("Invisible")) || bytes.Contains(data, []byte("invisible")) {
		t.Error("Invisible field should not be marshaled")
	}
	if !bytes.Contains(data, []byte("visible")) {
		t.Error("Visible field should be marshaled")
	}

	var result TaggedStruct
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if result.Visible != "yes" {
		t.Errorf("Visible mismatch: got %s, want yes", result.Visible)
	}
	if result.Invisible != "" {
		t.Errorf("Invisible should be empty, got %s", result.Invisible)
	}
}

func TestMsgPackMap(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	type MapContainer struct {
		Data map[string]int `serialize:"data"`
	}

	original := MapContainer{
		Data: map[string]int{"a": 1, "b": 2, "c": 3},
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result MapContainer
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(result.Data) != len(original.Data) {
		t.Errorf("map length mismatch: got %d, want %d", len(result.Data), len(original.Data))
	}
	for k, v := range original.Data {
		if result.Data[k] != v {
			t.Errorf("map[%s] mismatch: got %d, want %d", k, result.Data[k], v)
		}
	}
}

func TestMsgPackSlice(t *testing.T) {
	s := NewMsgPackSerializer()
	opts := DefaultOptions()

	type SliceContainer struct {
		Ints []int `serialize:"ints"`
	}

	original := SliceContainer{
		Ints: []int{1, 2, 3, 4, 5},
	}

	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result SliceContainer
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !reflect.DeepEqual(result.Ints, original.Ints) {
		t.Errorf("slice mismatch: got %v, want %v", result.Ints, original.Ints)
	}
}

func TestNestedPointers(t *testing.T) {
	s := NewJSONSerializer()
	opts := DefaultOptions()

	type Inner struct {
		Value int `serialize:"value"`
	}

	type Outer struct {
		Inner *Inner `serialize:"inner"`
	}

	original := Outer{Inner: &Inner{Value: 42}}
	data, err := s.Marshal(&original, opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result Outer
	err = s.Unmarshal(data, &result, opts)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Inner == nil {
		t.Fatal("Inner should not be nil")
	}
	if result.Inner.Value != 42 {
		t.Errorf("Inner.Value mismatch: got %d, want 42", result.Inner.Value)
	}

	originalNil := Outer{Inner: nil}
	data, err = s.Marshal(&originalNil, opts)
	if err != nil {
		t.Fatalf("Marshal nil inner failed: %v", err)
	}

	var resultNil Outer
	err = s.Unmarshal(data, &resultNil, opts)
	if err != nil {
		t.Fatalf("Unmarshal nil inner failed: %v", err)
	}
	if resultNil.Inner != nil {
		t.Error("Inner should be nil")
	}
}
