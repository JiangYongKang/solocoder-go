package serialize

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
)

const (
	mpNil      = 0xc0
	mpFalse    = 0xc2
	mpTrue     = 0xc3
	mpBin8     = 0xc4
	mpBin16    = 0xc5
	mpBin32    = 0xc6
	mpStr8      = 0xd9
	mpStr16     = 0xda
	mpStr32     = 0xdb
	mpArray16   = 0xdc
	mpArray32   = 0xdd
	mpMap16     = 0xde
	mpMap32     = 0xdf
	mpUint8     = 0xcc
	mpUint16    = 0xcd
	mpUint32    = 0xce
	mpUint64    = 0xcf
	mpInt8      = 0xd0
	mpInt16     = 0xd1
	mpInt32     = 0xd2
	mpInt64     = 0xd3
	mpFloat32   = 0xca
	mpFloat64   = 0xcb
)

type MsgPackSerializer struct{}

func NewMsgPackSerializer() *MsgPackSerializer {
	return &MsgPackSerializer{}
}

func (s *MsgPackSerializer) Name() string {
	return "msgpack"
}

func (s *MsgPackSerializer) ContentType() string {
	return ContentTypeMsgPack
}

func (s *MsgPackSerializer) Marshal(v interface{}, opts Options) ([]byte, error) {
	if v == nil {
		return []byte{mpNil}, nil
	}

	var buf []byte
	var err error

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte{mpNil}, nil
		}
		rv = rv.Elem()
	}

	if opts.Version > 0 && rv.Kind() == reflect.Struct {
		fieldCount := 1
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			tag := field.Tag.Get("serialize")
			if tag == "-" {
				continue
			}
			fieldCount++
		}

		if fieldCount <= 15 {
			buf = append(buf, byte(0x80|fieldCount))
		} else if fieldCount <= 0xffff {
			buf = append(buf, mpMap16)
			buf = binary.BigEndian.AppendUint16(buf, uint16(fieldCount))
		} else {
			buf = append(buf, mpMap32)
			buf = binary.BigEndian.AppendUint32(buf, uint32(fieldCount))
		}

		buf = appendString(buf, "__version__")
		buf, err = appendValue(buf, reflect.ValueOf(opts.Version))
		if err != nil {
			return nil, err
		}
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
			buf = appendString(buf, name)
			buf, err = appendValue(buf, rv.Field(i))
			if err != nil {
				return nil, err
			}
		}
		return buf, nil
	}

	return appendValue(buf, rv)
}

func appendValue(buf []byte, rv reflect.Value) ([]byte, error) {
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return append(buf, mpTrue), nil
		}
		return append(buf, mpFalse), nil

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return appendInt(buf, rv.Int()), nil

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return appendUint(buf, rv.Uint()), nil

	case reflect.Float32:
		buf = append(buf, mpFloat32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, math.Float32bits(float32(rv.Float())))
		return append(buf, b...), nil

	case reflect.Float64:
		buf = append(buf, mpFloat64)
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, math.Float64bits(rv.Float()))
		return append(buf, b...), nil

	case reflect.String:
		return appendString(buf, rv.String()), nil

	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return appendBytes(buf, rv.Bytes()), nil
		}
		return appendArray(buf, rv)

	case reflect.Array:
		return appendArray(buf, rv)

	case reflect.Map:
		return appendMap(buf, rv)

	case reflect.Struct:
		return appendStruct(buf, rv)

	case reflect.Ptr:
		if rv.IsNil() {
			return append(buf, mpNil), nil
		}
		return appendValue(buf, rv.Elem())

	case reflect.Interface:
		if rv.IsNil() {
			return append(buf, mpNil), nil
		}
		return appendValue(buf, rv.Elem())

	default:
		return nil, fmt.Errorf("%w: unsupported type %s", ErrInvalidType, rv.Kind())
	}
}

func appendInt(buf []byte, n int64) []byte {
	if n >= 0 && n <= 127 {
		return append(buf, byte(n))
	}
	if n >= -32 && n < 0 {
		return append(buf, byte(0xe0|(n+32)))
	}
	if n >= math.MinInt8 && n <= math.MaxInt8 {
		buf = append(buf, mpInt8)
		return append(buf, byte(int8(n)))
	}
	if n >= math.MinInt16 && n <= math.MaxInt16 {
		buf = append(buf, mpInt16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(n))
		return append(buf, b...)
	}
	if n >= math.MinInt32 && n <= math.MaxInt32 {
		buf = append(buf, mpInt32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(n))
		return append(buf, b...)
	}
	buf = append(buf, mpInt64)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, uint64(n))
	return append(buf, b...)
}

func appendUint(buf []byte, n uint64) []byte {
	if n <= 127 {
		return append(buf, byte(n))
	}
	if n <= math.MaxUint8 {
		buf = append(buf, mpUint8)
		return append(buf, byte(n))
	}
	if n <= math.MaxUint16 {
		buf = append(buf, mpUint16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(n))
		return append(buf, b...)
	}
	if n <= math.MaxUint32 {
		buf = append(buf, mpUint32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(n))
		return append(buf, b...)
	}
	buf = append(buf, mpUint64)
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, n)
	return append(buf, b...)
}

func appendString(buf []byte, s string) []byte {
	l := len(s)
	if l <= 31 {
		buf = append(buf, 0xa0|byte(l))
	} else if l <= math.MaxUint8 {
		buf = append(buf, mpStr8, byte(l))
	} else if l <= math.MaxUint16 {
		buf = append(buf, mpStr16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(l))
		buf = append(buf, b...)
	} else {
		buf = append(buf, mpStr32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(l))
		buf = append(buf, b...)
	}
	return append(buf, s...)
}

func appendBytes(buf []byte, b []byte) []byte {
	l := len(b)
	if l <= math.MaxUint8 {
		buf = append(buf, mpBin8, byte(l))
	} else if l <= math.MaxUint16 {
		buf = append(buf, mpBin16)
		bl := make([]byte, 2)
		binary.BigEndian.PutUint16(bl, uint16(l))
		buf = append(buf, bl...)
	} else {
		buf = append(buf, mpBin32)
		bl := make([]byte, 4)
		binary.BigEndian.PutUint32(bl, uint32(l))
		buf = append(buf, bl...)
	}
	return append(buf, b...)
}

func appendArray(buf []byte, rv reflect.Value) ([]byte, error) {
	l := rv.Len()
	if l <= 15 {
		buf = append(buf, 0x90|byte(l))
	} else if l <= math.MaxUint16 {
		buf = append(buf, mpArray16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(l))
		buf = append(buf, b...)
	} else {
		buf = append(buf, mpArray32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(l))
		buf = append(buf, b...)
	}
	for i := 0; i < l; i++ {
		var err error
		buf, err = appendValue(buf, rv.Index(i))
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func appendMap(buf []byte, rv reflect.Value) ([]byte, error) {
	l := rv.Len()
	if l <= 15 {
		buf = append(buf, 0x80|byte(l))
	} else if l <= math.MaxUint16 {
		buf = append(buf, mpMap16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(l))
		buf = append(buf, b...)
	} else {
		buf = append(buf, mpMap32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(l))
		buf = append(buf, b...)
	}
	for _, k := range rv.MapKeys() {
		var err error
		buf, err = appendValue(buf, k)
		if err != nil {
			return nil, err
		}
		buf, err = appendValue(buf, rv.MapIndex(k))
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func appendStruct(buf []byte, rv reflect.Value) ([]byte, error) {
	fields := 0
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		if field.IsExported() {
			tag := field.Tag.Get("serialize")
			if tag != "-" {
				fields++
			}
		}
	}

	if fields <= 15 {
		buf = append(buf, 0x80|byte(fields))
	} else if fields <= math.MaxUint16 {
		buf = append(buf, mpMap16)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(fields))
		buf = append(buf, b...)
	} else {
		buf = append(buf, mpMap32)
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, uint32(fields))
		buf = append(buf, b...)
	}

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
		buf = appendString(buf, name)
		var err error
		buf, err = appendValue(buf, rv.Field(i))
		if err != nil {
			return nil, err
		}
	}
	return buf, nil
}

func (s *MsgPackSerializer) Unmarshal(data []byte, v interface{}, opts Options) error {
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

	dec := &mpDecoder{data: data, pos: 0, opts: opts}
	return dec.decodeValue(rv.Elem())
}

type mpDecoder struct {
	data []byte
	pos  int
	opts Options
}

func (d *mpDecoder) decodeValue(rv reflect.Value) error {
	if d.pos >= len(d.data) {
		return fmt.Errorf("%w: unexpected end of data", ErrInvalidFormat)
	}

	code := d.data[d.pos]
	d.pos++

	switch {
	case code == mpNil:
		rv.Set(reflect.Zero(rv.Type()))
		return nil

	case code == mpFalse:
		return setFieldValue(rv, false)

	case code == mpTrue:
		return setFieldValue(rv, true)

	case code >= 0x00 && code <= 0x7f:
		return setFieldValue(rv, int64(code))

	case code >= 0xe0 && code <= 0xff:
		return setFieldValue(rv, int64(int8(code)))

	case code == mpUint8:
		if d.pos+1 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := uint64(d.data[d.pos])
		d.pos++
		return setFieldValue(rv, int64(v))

	case code == mpUint16:
		if d.pos+2 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := binary.BigEndian.Uint16(d.data[d.pos:])
		d.pos += 2
		return setFieldValue(rv, int64(v))

	case code == mpUint32:
		if d.pos+4 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := binary.BigEndian.Uint32(d.data[d.pos:])
		d.pos += 4
		return setFieldValue(rv, int64(v))

	case code == mpUint64:
		if d.pos+8 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := binary.BigEndian.Uint64(d.data[d.pos:])
		d.pos += 8
		return setFieldValue(rv, int64(v))

	case code == mpInt8:
		if d.pos+1 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := int64(int8(d.data[d.pos]))
		d.pos++
		return setFieldValue(rv, v)

	case code == mpInt16:
		if d.pos+2 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := int64(int16(binary.BigEndian.Uint16(d.data[d.pos:])))
		d.pos += 2
		return setFieldValue(rv, v)

	case code == mpInt32:
		if d.pos+4 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := int64(int32(binary.BigEndian.Uint32(d.data[d.pos:])))
		d.pos += 4
		return setFieldValue(rv, v)

	case code == mpInt64:
		if d.pos+8 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := int64(binary.BigEndian.Uint64(d.data[d.pos:]))
		d.pos += 8
		return setFieldValue(rv, v)

	case code == mpFloat32:
		if d.pos+4 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := math.Float32frombits(binary.BigEndian.Uint32(d.data[d.pos:]))
		d.pos += 4
		return setFieldValue(rv, float64(v))

	case code == mpFloat64:
		if d.pos+8 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := math.Float64frombits(binary.BigEndian.Uint64(d.data[d.pos:]))
		d.pos += 8
		return setFieldValue(rv, v)

	case code >= 0xa0 && code <= 0xbf:
		l := int(code & 0x1f)
		return d.decodeString(rv, l)

	case code == mpStr8:
		l, err := d.readUint8()
		if err != nil {
			return err
		}
		return d.decodeString(rv, int(l))

	case code == mpStr16:
		l, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeString(rv, int(l))

	case code == mpStr32:
		l, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeString(rv, int(l))

	case code == mpBin8:
		l, err := d.readUint8()
		if err != nil {
			return err
		}
		return d.decodeBytes(rv, int(l))

	case code == mpBin16:
		l, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeBytes(rv, int(l))

	case code == mpBin32:
		l, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeBytes(rv, int(l))

	case code >= 0x90 && code <= 0x9f:
		l := int(code & 0x0f)
		return d.decodeArray(rv, l)

	case code == mpArray16:
		l, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeArray(rv, int(l))

	case code == mpArray32:
		l, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeArray(rv, int(l))

	case code >= 0x80 && code <= 0x8f:
		l := int(code & 0x0f)
		return d.decodeMap(rv, l)

	case code == mpMap16:
		l, err := d.readUint16()
		if err != nil {
			return err
		}
		return d.decodeMap(rv, int(l))

	case code == mpMap32:
		l, err := d.readUint32()
		if err != nil {
			return err
		}
		return d.decodeMap(rv, int(l))

	default:
		return fmt.Errorf("%w: unknown code 0x%x", ErrInvalidFormat, code)
	}
}

func (d *mpDecoder) readUint8() (uint8, error) {
	if d.pos+1 > len(d.data) {
		return 0, fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
	}
	v := d.data[d.pos]
	d.pos++
	return v, nil
}

func (d *mpDecoder) readUint16() (uint16, error) {
	if d.pos+2 > len(d.data) {
		return 0, fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
	}
	v := binary.BigEndian.Uint16(d.data[d.pos:])
	d.pos += 2
	return v, nil
}

func (d *mpDecoder) readUint32() (uint32, error) {
	if d.pos+4 > len(d.data) {
		return 0, fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
	}
	v := binary.BigEndian.Uint32(d.data[d.pos:])
	d.pos += 4
	return v, nil
}

func (d *mpDecoder) decodeString(rv reflect.Value, l int) error {
	if d.pos+l > len(d.data) {
		return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
	}
	b := d.data[d.pos : d.pos+l]
	d.pos += l

	if rv.Kind() == reflect.String {
		if d.opts.ZeroCopy {
			rv.SetString(zeroCopyString(b))
		} else {
			rv.SetString(string(b))
		}
		return nil
	}

	return setFieldValue(rv, string(b))
}

func (d *mpDecoder) decodeBytes(rv reflect.Value, l int) error {
	if d.pos+l > len(d.data) {
		return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
	}
	b := d.data[d.pos : d.pos+l]
	d.pos += l

	if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		if d.opts.ZeroCopy {
			rv.SetBytes(b)
		} else {
			cp := make([]byte, l)
			copy(cp, b)
			rv.SetBytes(cp)
		}
		return nil
	}

	return setFieldValue(rv, b)
}

func (d *mpDecoder) decodeArray(rv reflect.Value, l int) error {
	if rv.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(rv.Type(), l, l)
		for i := 0; i < l; i++ {
			if err := d.decodeValue(slice.Index(i)); err != nil {
				return err
			}
		}
		rv.Set(slice)
		return nil
	}

	if rv.Kind() == reflect.Array {
		for i := 0; i < l && i < rv.Len(); i++ {
			if err := d.decodeValue(rv.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}

	for i := 0; i < l; i++ {
		var dummy interface{}
		dummyVal := reflect.ValueOf(&dummy).Elem()
		if err := d.decodeValue(dummyVal); err != nil {
			return err
		}
	}
	return nil
}

func (d *mpDecoder) decodeMap(rv reflect.Value, l int) error {
	if rv.Kind() == reflect.Struct {
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

		dataVersion := 0
		versionFound := false

		for i := 0; i < l; i++ {
			var keyStr string
			keyVal := reflect.ValueOf(&keyStr).Elem()
			if err := d.decodeValue(keyVal); err != nil {
				return err
			}

			if keyStr == "__version__" {
				var ver int
				verVal := reflect.ValueOf(&ver).Elem()
				if err := d.decodeValue(verVal); err != nil {
					return err
				}
				dataVersion = ver
				versionFound = true
				continue
			}

			field, ok := fieldMap[keyStr]
			if !ok {
				field, ok = fieldMap[strings.ToLower(keyStr)]
			}
			if !ok {
				if d.opts.UnknownFieldBehavior == ReturnUnknownFieldError {
					return fmt.Errorf("%w: %s", ErrUnknownField, keyStr)
				}
				var dummy interface{}
				dummyVal := reflect.ValueOf(&dummy).Elem()
				if err := d.decodeValue(dummyVal); err != nil {
					return err
				}
				continue
			}

			fieldValue := rv.FieldByName(field.Name)
			if fieldValue.CanSet() {
				if err := d.decodeValue(fieldValue); err != nil {
					return err
				}
			} else {
				var dummy interface{}
				dummyVal := reflect.ValueOf(&dummy).Elem()
				if err := d.decodeValue(dummyVal); err != nil {
					return err
				}
			}
		}

		if d.opts.StrictMode && versionFound && dataVersion > 0 && dataVersion != d.opts.Version {
			return fmt.Errorf("%w: expected %d, got %d", ErrVersionMismatch, d.opts.Version, dataVersion)
		}

		return nil
	}

	if rv.Kind() == reflect.Map {
		newMap := reflect.MakeMap(rv.Type())
		for i := 0; i < l; i++ {
			key := reflect.New(rv.Type().Key())
			if err := d.decodeValue(key.Elem()); err != nil {
				return err
			}
			val := reflect.New(rv.Type().Elem())
			if err := d.decodeValue(val.Elem()); err != nil {
				return err
			}
			newMap.SetMapIndex(key.Elem(), val.Elem())
		}
		rv.Set(newMap)
		return nil
	}

	for i := 0; i < l; i++ {
		var dummyKey, dummyVal interface{}
		dummyKeyVal := reflect.ValueOf(&dummyKey).Elem()
		if err := d.decodeValue(dummyKeyVal); err != nil {
			return err
		}
		dummyValVal := reflect.ValueOf(&dummyVal).Elem()
		if err := d.decodeValue(dummyValVal); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	Register("msgpack", NewMsgPackSerializer())
}
