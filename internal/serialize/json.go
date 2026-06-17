package serialize

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"sort"
	"strings"
	"unicode"
)

type JSONSerializer struct{}

func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

func (s *JSONSerializer) Name() string {
	return "json"
}

func (s *JSONSerializer) ContentType() string {
	return ContentTypeJSON
}

func (s *JSONSerializer) Marshal(v interface{}, opts Options) ([]byte, error) {
	if v == nil {
		return nil, ErrNilInput
	}

	if opts.Version > 0 {
		rv := reflect.ValueOf(v)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		if rv.Kind() == reflect.Struct {
			versioned := make(map[string]interface{})
			versioned["__version__"] = opts.Version
			for i := 0; i < rv.NumField(); i++ {
				field := rv.Type().Field(i)
				if !field.IsExported() {
					continue
				}
				tag := field.Tag.Get("serialize")
				if tag == "-" {
					continue
				}
				name := getFieldName(field)
				versioned[name] = rv.Field(i).Interface()
			}
			return json.Marshal(versioned)
		}
	}

	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidType, err)
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}
	return result, nil
}

func (s *JSONSerializer) Unmarshal(data []byte, v interface{}, opts Options) error {
	if v == nil {
		return ErrUnmarshalNil
	}
	if len(data) == 0 {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return ErrUnmarshalNil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	if opts.SkipUnknownFields || opts.UnknownFieldBehavior == SkipUnknownField {
	} else {
		dec.DisallowUnknownFields()
	}

	rv = rv.Elem()
	if rv.Kind() == reflect.Struct {
		var raw map[string]json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if opts.UnknownFieldBehavior == ReturnUnknownFieldError && strings.Contains(err.Error(), "unknown field") {
				return fmt.Errorf("%w: %v", ErrUnknownField, err)
			}
			return fmt.Errorf("%w: %v", ErrInvalidType, err)
		}

		dataVersion := 0
		if versionRaw, ok := raw["__version__"]; ok {
			json.Unmarshal(versionRaw, &dataVersion)
			delete(raw, "__version__")
		}

		if opts.StrictMode && dataVersion > 0 && dataVersion != opts.Version {
			return fmt.Errorf("%w: expected %d, got %d", ErrVersionMismatch, opts.Version, dataVersion)
		}

		fieldMap := make(map[string]reflect.StructField)
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			tag := field.Tag.Get("serialize")
			if tag == "-" {
				continue
			}
			name := getFieldName(field)
			fieldMap[name] = field
			fieldMap[strings.ToLower(name)] = field
		}

		for key, rawValue := range raw {
			field, ok := fieldMap[key]
			if !ok {
				field, ok = fieldMap[strings.ToLower(key)]
			}
			if !ok {
				if opts.UnknownFieldBehavior == ReturnUnknownFieldError {
					return fmt.Errorf("%w: %s", ErrUnknownField, key)
				}
				continue
			}

			fieldValue := rv.FieldByName(field.Name)
			if !fieldValue.CanSet() {
				continue
			}

			if err := s.unmarshalField(rawValue, fieldValue, opts); err != nil {
				return err
			}
		}

		return nil
	}

	return json.Unmarshal(data, v)
}

func (s *JSONSerializer) unmarshalField(raw json.RawMessage, field reflect.Value, opts Options) error {
	fieldType := field.Type()

	switch fieldType.Kind() {
	case reflect.String:
		if opts.ZeroCopy && len(raw) >= 2 {
			str := string(raw)
			if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
				unquoted, err := unquoteBytes(raw)
				if err != nil {
					return err
				}
				field.SetString(zeroCopyString(unquoted))
				return nil
			}
		}
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return err
		}
		field.SetString(str)

	case reflect.Slice:
		if fieldType.Elem().Kind() == reflect.Uint8 {
			var b []byte
			if err := json.Unmarshal(raw, &b); err != nil {
				return err
			}
			if opts.ZeroCopy {
				field.SetBytes(b)
			} else {
				copied := make([]byte, len(b))
				copy(copied, b)
				field.SetBytes(copied)
			}
		} else {
			slicePtr := reflect.New(fieldType)
			if err := json.Unmarshal(raw, slicePtr.Interface()); err != nil {
				return err
			}
			field.Set(slicePtr.Elem())
		}

	case reflect.Ptr:
		if raw[0] == 'n' && len(raw) >= 4 && string(raw[:4]) == "null" {
			field.Set(reflect.Zero(fieldType))
			return nil
		}
		newVal := reflect.New(fieldType.Elem())
		if err := s.unmarshalField(raw, newVal.Elem(), opts); err != nil {
			return err
		}
		field.Set(newVal)

	case reflect.Struct:
		if fieldType == reflect.TypeOf(VersionInfo{}) {
			var vi VersionInfo
			if err := json.Unmarshal(raw, &vi); err != nil {
				return err
			}
			field.Set(reflect.ValueOf(vi))
			return nil
		}
		newStruct := reflect.New(fieldType)
		if err := s.Unmarshal(raw, newStruct.Interface(), opts); err != nil {
			return err
		}
		field.Set(newStruct.Elem())

	case reflect.Map:
		mapPtr := reflect.New(fieldType)
		mapPtr.Elem().Set(reflect.MakeMap(fieldType))
		if err := json.Unmarshal(raw, mapPtr.Interface()); err != nil {
			return err
		}
		field.Set(mapPtr.Elem())

	default:
		if err := json.Unmarshal(raw, field.Addr().Interface()); err != nil {
			return err
		}
	}

	return nil
}

type VersionInfo struct {
	Version int `json:"__version__"`
}

func unquoteBytes(b []byte) ([]byte, error) {
	if len(b) < 2 || b[0] != '"' || b[len(b)-1] != '"' {
		return nil, fmt.Errorf("%w: invalid string", ErrInvalidFormat)
	}
	b = b[1 : len(b)-1]

	var buf bytes.Buffer
	for i := 0; i < len(b); i++ {
		if b[i] == '\\' && i+1 < len(b) {
			switch b[i+1] {
			case '"':
				buf.WriteByte('"')
				i++
			case '\\':
				buf.WriteByte('\\')
				i++
			case '/':
				buf.WriteByte('/')
				i++
			case 'n':
				buf.WriteByte('\n')
				i++
			case 'r':
				buf.WriteByte('\r')
				i++
			case 't':
				buf.WriteByte('\t')
				i++
			default:
				buf.WriteByte(b[i])
			}
		} else {
			buf.WriteByte(b[i])
		}
	}
	return buf.Bytes(), nil
}

func jsonBase64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func toSnakeCase(s string) string {
	var buf bytes.Buffer
	for i, r := range s {
		if i > 0 && unicode.IsUpper(r) {
			buf.WriteByte('_')
		}
		buf.WriteRune(unicode.ToLower(r))
	}
	return buf.String()
}

func sortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type jsonDecoder struct {
	r     io.Reader
	buf   []byte
	pos   int
	depth int
}

func newJSONDecoder(r io.Reader) *jsonDecoder {
	return &jsonDecoder{r: r, buf: make([]byte, 0, 4096)}
}

func init() {
	Register("json", NewJSONSerializer())
}
