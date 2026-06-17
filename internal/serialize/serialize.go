package serialize

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unsafe"
)

const (
	ContentTypeJSON     = "application/json"
	ContentTypeMsgPack  = "application/msgpack"
	ContentTypeProtobuf = "application/protobuf"

	DefaultFormat = "json"
)

var (
	ErrSerializerNotFound = errors.New("serialize: serializer not found")
	ErrNilInput           = errors.New("serialize: nil input")
	ErrInvalidType        = errors.New("serialize: invalid type")
	ErrUnmarshalNil       = errors.New("serialize: cannot unmarshal into nil")
	ErrUnknownField       = errors.New("serialize: unknown field")
	ErrVersionMismatch    = errors.New("serialize: version mismatch")
	ErrInvalidFormat      = errors.New("serialize: invalid format")
)

type UnknownFieldBehavior int

const (
	SkipUnknownField UnknownFieldBehavior = iota
	ReturnUnknownFieldError
)

type Options struct {
	ZeroCopy             bool
	SkipUnknownFields    bool
	UnknownFieldBehavior UnknownFieldBehavior
	Version              int
	StrictMode           bool
}

func DefaultOptions() Options {
	return Options{
		ZeroCopy:             true,
		SkipUnknownFields:    true,
		UnknownFieldBehavior: SkipUnknownField,
		Version:              1,
		StrictMode:           false,
	}
}

type Serializer interface {
	Name() string
	ContentType() string
	Marshal(v interface{}, opts Options) ([]byte, error)
	Unmarshal(data []byte, v interface{}, opts Options) error
}

type Registry struct {
	mu            sync.RWMutex
	serializers   map[string]Serializer
	defaultName   string
}

var defaultRegistry = NewRegistry()

func NewRegistry() *Registry {
	return &Registry{
		serializers: make(map[string]Serializer),
	}
}

func (r *Registry) Register(name string, s Serializer) error {
	if name == "" {
		return fmt.Errorf("%w: empty name", ErrInvalidFormat)
	}
	if s == nil {
		return fmt.Errorf("%w: nil serializer", ErrInvalidType)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serializers[name] = s
	if r.defaultName == "" {
		r.defaultName = name
	}
	return nil
}

func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.serializers, name)
	if r.defaultName == name {
		r.defaultName = ""
		for k := range r.serializers {
			r.defaultName = k
			break
		}
	}
}

func (r *Registry) Get(name string) (Serializer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.serializers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrSerializerNotFound, name)
	}
	return s, nil
}

func (r *Registry) GetByContentType(contentType string) (Serializer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, s := range r.serializers {
		if s.ContentType() == contentType {
			return s, nil
		}
	}
	return nil, fmt.Errorf("%w for content type: %s", ErrSerializerNotFound, contentType)
}

func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.serializers[name]; !ok {
		return fmt.Errorf("%w: %s", ErrSerializerNotFound, name)
	}
	r.defaultName = name
	return nil
}

func (r *Registry) Default() (Serializer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.defaultName == "" {
		return nil, ErrSerializerNotFound
	}
	s, ok := r.serializers[r.defaultName]
	if !ok {
		return nil, ErrSerializerNotFound
	}
	return s, nil
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.serializers))
	for k := range r.serializers {
		names = append(names, k)
	}
	return names
}

func Register(name string, s Serializer) error {
	return defaultRegistry.Register(name, s)
}

func Unregister(name string) {
	defaultRegistry.Unregister(name)
}

func Get(name string) (Serializer, error) {
	return defaultRegistry.Get(name)
}

func GetByContentType(contentType string) (Serializer, error) {
	return defaultRegistry.GetByContentType(contentType)
}

func SetDefault(name string) error {
	return defaultRegistry.SetDefault(name)
}

func Default() (Serializer, error) {
	return defaultRegistry.Default()
}

func List() []string {
	return defaultRegistry.List()
}

func Marshal(v interface{}, opts Options) ([]byte, error) {
	s, err := defaultRegistry.Default()
	if err != nil {
		return nil, err
	}
	return s.Marshal(v, opts)
}

func MarshalWith(name string, v interface{}, opts Options) ([]byte, error) {
	s, err := defaultRegistry.Get(name)
	if err != nil {
		return nil, err
	}
	return s.Marshal(v, opts)
}

func Unmarshal(data []byte, v interface{}, opts Options) error {
	s, err := defaultRegistry.Default()
	if err != nil {
		return err
	}
	return s.Unmarshal(data, v, opts)
}

func UnmarshalWith(name string, data []byte, v interface{}, opts Options) error {
	s, err := defaultRegistry.Get(name)
	if err != nil {
		return err
	}
	return s.Unmarshal(data, v, opts)
}

// getFieldName extracts the serialized field name from a struct field's "serialize" tag,
// following consistent rules across all serializer implementations.
//
// Tag format (comma-separated):
//
//	`serialize:"fieldname"`               => field name "fieldname"
//	`serialize:"fieldname,protobuf:3"`    => field name "fieldname"
//	`serialize:",protobuf:3"`             => field name uses default (field.Name)
//	`serialize:"protobuf:3"`              => field name uses default (field.Name)
//	`serialize:""`                        => field name uses default (field.Name)
//
// A leading "protobuf:" prefix on parts[0] is treated as a field number declaration,
// NOT as a field name, to keep behavior consistent across JSON/MessagePack/Protobuf.
func getFieldName(field reflect.StructField) string {
	name := field.Name
	tag := field.Tag.Get("serialize")
	if tag == "" {
		return name
	}
	parts := strings.Split(tag, ",")
	if parts[0] != "" && !strings.HasPrefix(parts[0], "protobuf:") {
		name = parts[0]
	}
	return name
}

func setFieldValue(field reflect.Value, value interface{}) error {
	if !field.CanSet() {
		return fmt.Errorf("%w: cannot set field", ErrInvalidType)
	}

	val := reflect.ValueOf(value)
	if !val.IsValid() {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}

	if val.Type().AssignableTo(field.Type()) {
		field.Set(val)
		return nil
	}

	if val.Type().ConvertibleTo(field.Type()) {
		field.Set(val.Convert(field.Type()))
		return nil
	}

	return fmt.Errorf("%w: cannot assign %s to %s", ErrInvalidType, val.Type(), field.Type())
}

func zeroCopyString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func zeroCopyBytes(s string) []byte {
	if s == "" {
		return nil
	}
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
