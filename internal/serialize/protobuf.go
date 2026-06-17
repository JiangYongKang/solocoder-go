package serialize

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strings"
)

const (
	pbWireTypeVarint     = 0
	pbWireType64bit      = 1
	pbWireTypeLengthDelim = 2
	pbWireType32bit      = 5
)

type ProtoBufSerializer struct{}

func NewProtoBufSerializer() *ProtoBufSerializer {
	return &ProtoBufSerializer{}
}

func (s *ProtoBufSerializer) Name() string {
	return "protobuf"
}

func (s *ProtoBufSerializer) ContentType() string {
	return ContentTypeProtobuf
}

type pbFieldInfo struct {
	fieldNum int
	wireType int
	name     string
	index    int
}

// Protobuf wire field number assignment rules:
//
//   - Wire field number 1 is PERMANENTLY RESERVED for the __version__ field.
//     This is true regardless of whether opts.Version > 0 during encoding.
//     During decoding, wire field 1 is always treated as version metadata
//     and is NEVER mapped to a user struct field, preventing version data
//     from contaminating user structures when opts.Version=0 decodes an
//     encoded message that contained a version.
//   - When a struct tag explicitly declares a protobuf number (e.g.
//     serialize:"name,protobuf:3"), that number is used as-is on the wire.
//     Users MUST NOT declare protobuf:1 for their own fields.
//   - When no protobuf number is declared, the default is field.Index[0] + 2,
//     skipping the reserved field 1 so the first struct field maps to wire field 2.
//   - Marshal encodes using info.fieldNum directly; Unmarshal builds fieldMap
//     keyed by info.fieldNum directly. No additional offset is applied.
func getPBFieldInfo(field reflect.StructField) (*pbFieldInfo, bool) {
	if !field.IsExported() {
		return nil, false
	}
	tag := field.Tag.Get("serialize")
	if tag == "-" {
		return nil, false
	}

	info := &pbFieldInfo{name: getFieldName(field), index: field.Index[0]}

	if tag != "" {
		parts := strings.Split(tag, ",")
		for i := 0; i < len(parts); i++ {
			part := strings.TrimSpace(parts[i])
			if strings.HasPrefix(part, "protobuf:") {
				numStr := strings.TrimPrefix(part, "protobuf:")
				var num int
				if n, err := fmt.Sscanf(numStr, "%d", &num); err == nil && n == 1 && num > 0 {
					if num == 1 {
						num = 2
					}
					info.fieldNum = num
				}
			}
		}
	}

	if info.fieldNum == 0 {
		info.fieldNum = field.Index[0] + 2
	}

	info.wireType = getWireType(field.Type)
	return info, true
}

func getWireType(t reflect.Type) int {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return pbWireTypeVarint
	case reflect.Float32:
		return pbWireType32bit
	case reflect.Float64:
		return pbWireType64bit
	case reflect.String, reflect.Slice, reflect.Map, reflect.Struct, reflect.Ptr, reflect.Array:
		return pbWireTypeLengthDelim
	default:
		return pbWireTypeLengthDelim
	}
}

func (s *ProtoBufSerializer) Marshal(v interface{}, opts Options) ([]byte, error) {
	if v == nil {
		return nil, ErrNilInput
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return nil, nil
		}
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("%w: protobuf only supports struct types", ErrInvalidType)
	}

	var buf []byte

	if opts.Version > 0 {
		versionBuf, err := s.encodeField(1, pbWireTypeVarint, reflect.ValueOf(opts.Version))
		if err != nil {
			return nil, err
		}
		buf = append(buf, versionBuf...)
	}

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		info, ok := getPBFieldInfo(field)
		if !ok {
			continue
		}

		fieldVal := rv.Field(i)
		if isZeroValue(fieldVal) {
			continue
		}

		fieldBuf, err := s.encodeField(info.fieldNum, info.wireType, fieldVal)
		if err != nil {
			return nil, err
		}
		buf = append(buf, fieldBuf...)
	}

	return buf, nil
}

func isZeroValue(rv reflect.Value) bool {
	switch rv.Kind() {
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.String:
		return rv.String() == ""
	case reflect.Slice, reflect.Map:
		return rv.IsNil() || rv.Len() == 0
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	default:
		return false
	}
}

func (s *ProtoBufSerializer) encodeField(fieldNum, wireType int, rv reflect.Value) ([]byte, error) {
	tag := uint64(fieldNum)<<3 | uint64(wireType)
	tagBytes := encodeVarint(tag)

	var valueBytes []byte
	var err error

	switch wireType {
	case pbWireTypeVarint:
		valueBytes, err = s.encodeVarintValue(rv)
	case pbWireType32bit:
		valueBytes, err = s.encode32bitValue(rv)
	case pbWireType64bit:
		valueBytes, err = s.encode64bitValue(rv)
	case pbWireTypeLengthDelim:
		valueBytes, err = s.encodeLengthDelimValue(rv)
	default:
		return nil, fmt.Errorf("%w: unknown wire type %d", ErrInvalidFormat, wireType)
	}

	if err != nil {
		return nil, err
	}

	result := make([]byte, 0, len(tagBytes)+len(valueBytes))
	result = append(result, tagBytes...)
	result = append(result, valueBytes...)
	return result, nil
}

func encodeVarint(v uint64) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func (s *ProtoBufSerializer) encodeVarintValue(rv reflect.Value) ([]byte, error) {
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return []byte{1}, nil
		}
		return []byte{0}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodeVarint(uint64(rv.Int())), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return encodeVarint(rv.Uint()), nil
	default:
		return nil, fmt.Errorf("%w: cannot encode %s as varint", ErrInvalidType, rv.Kind())
	}
}

func (s *ProtoBufSerializer) encode32bitValue(rv reflect.Value) ([]byte, error) {
	switch rv.Kind() {
	case reflect.Float32:
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, math.Float32bits(float32(rv.Float())))
		return b, nil
	default:
		return nil, fmt.Errorf("%w: cannot encode %s as 32-bit", ErrInvalidType, rv.Kind())
	}
}

func (s *ProtoBufSerializer) encode64bitValue(rv reflect.Value) ([]byte, error) {
	switch rv.Kind() {
	case reflect.Float64:
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, math.Float64bits(rv.Float()))
		return b, nil
	default:
		return nil, fmt.Errorf("%w: cannot encode %s as 64-bit", ErrInvalidType, rv.Kind())
	}
}

func (s *ProtoBufSerializer) encodeLengthDelimValue(rv reflect.Value) ([]byte, error) {
	switch rv.Kind() {
	case reflect.String:
		strBytes := []byte(rv.String())
		lenBytes := encodeVarint(uint64(len(strBytes)))
		result := make([]byte, 0, len(lenBytes)+len(strBytes))
		result = append(result, lenBytes...)
		result = append(result, strBytes...)
		return result, nil

	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			b := rv.Bytes()
			lenBytes := encodeVarint(uint64(len(b)))
			result := make([]byte, 0, len(lenBytes)+len(b))
			result = append(result, lenBytes...)
			result = append(result, b...)
			return result, nil
		}
		fallthrough

	case reflect.Array:
		var buf []byte
		for i := 0; i < rv.Len(); i++ {
			elemWireType := getWireType(rv.Index(i).Type())
			elemBytes, err := s.encodeValue(elemWireType, rv.Index(i))
			if err != nil {
				return nil, err
			}
			buf = append(buf, elemBytes...)
		}
		lenBytes := encodeVarint(uint64(len(buf)))
		result := make([]byte, 0, len(lenBytes)+len(buf))
		result = append(result, lenBytes...)
		result = append(result, buf...)
		return result, nil

	case reflect.Struct:
		innerBuf, err := s.Marshal(rv.Addr().Interface(), Options{})
		if err != nil {
			return nil, err
		}
		lenBytes := encodeVarint(uint64(len(innerBuf)))
		result := make([]byte, 0, len(lenBytes)+len(innerBuf))
		result = append(result, lenBytes...)
		result = append(result, innerBuf...)
		return result, nil

	case reflect.Ptr:
		if rv.IsNil() {
			return encodeVarint(0), nil
		}
		return s.encodeLengthDelimValue(rv.Elem())

	default:
		return nil, fmt.Errorf("%w: cannot encode %s as length-delimited", ErrInvalidType, rv.Kind())
	}
}

func (s *ProtoBufSerializer) encodeValue(wireType int, rv reflect.Value) ([]byte, error) {
	switch wireType {
	case pbWireTypeVarint:
		return s.encodeVarintValue(rv)
	case pbWireType32bit:
		return s.encode32bitValue(rv)
	case pbWireType64bit:
		return s.encode64bitValue(rv)
	case pbWireTypeLengthDelim:
		return s.encodeLengthDelimValue(rv)
	default:
		return nil, fmt.Errorf("%w: unknown wire type %d", ErrInvalidFormat, wireType)
	}
}

func (s *ProtoBufSerializer) Unmarshal(data []byte, v interface{}, opts Options) error {
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

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("%w: protobuf only supports struct types", ErrInvalidType)
	}

	fieldMap := make(map[int]reflect.StructField)
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Type().Field(i)
		info, ok := getPBFieldInfo(field)
		if !ok {
			continue
		}
		fieldMap[info.fieldNum] = field
	}

	dec := &pbDecoder{data: data, pos: 0, opts: opts}
	dataVersion := 0
	versionFound := false

	for dec.pos < len(dec.data) {
		tag, err := dec.readVarint()
		if err != nil {
			return err
		}

		fieldNum := int(tag >> 3)
		wireType := int(tag & 0x7)

		if fieldNum == 1 {
			version, err := dec.readVarint()
			if err != nil {
				return err
			}
			dataVersion = int(version)
			versionFound = true
			continue
		}

		field, ok := fieldMap[fieldNum]
		if !ok {
			if opts.UnknownFieldBehavior == ReturnUnknownFieldError {
				return fmt.Errorf("%w: field number %d", ErrUnknownField, fieldNum)
			}
			if err := dec.skipValue(wireType); err != nil {
				return err
			}
			continue
		}

		fieldValue := rv.FieldByName(field.Name)
		if !fieldValue.CanSet() {
			if err := dec.skipValue(wireType); err != nil {
				return err
			}
			continue
		}

		if err := dec.decodeValue(wireType, fieldValue); err != nil {
			return err
		}
	}

	if opts.StrictMode && versionFound && dataVersion > 0 && dataVersion != opts.Version {
		return fmt.Errorf("%w: expected %d, got %d", ErrVersionMismatch, opts.Version, dataVersion)
	}

	return nil
}

type pbDecoder struct {
	data []byte
	pos  int
	opts Options
}

func (d *pbDecoder) readVarint() (uint64, error) {
	var result uint64
	var shift uint

	for {
		if d.pos >= len(d.data) {
			return 0, fmt.Errorf("%w: unexpected end of varint", ErrInvalidFormat)
		}
		b := d.data[d.pos]
		d.pos++
		result |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
		if shift >= 64 {
			return 0, fmt.Errorf("%w: varint too long", ErrInvalidFormat)
		}
	}
	return result, nil
}

func (d *pbDecoder) skipValue(wireType int) error {
	switch wireType {
	case pbWireTypeVarint:
		_, err := d.readVarint()
		return err
	case pbWireType32bit:
		if d.pos+4 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		d.pos += 4
		return nil
	case pbWireType64bit:
		if d.pos+8 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		d.pos += 8
		return nil
	case pbWireTypeLengthDelim:
		l, err := d.readVarint()
		if err != nil {
			return err
		}
		if d.pos+int(l) > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		d.pos += int(l)
		return nil
	default:
		return fmt.Errorf("%w: unknown wire type %d", ErrInvalidFormat, wireType)
	}
}

func (d *pbDecoder) decodeValue(wireType int, rv reflect.Value) error {
	switch wireType {
	case pbWireTypeVarint:
		v, err := d.readVarint()
		if err != nil {
			return err
		}
		return d.setVarintValue(rv, v)

	case pbWireType32bit:
		if d.pos+4 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := binary.LittleEndian.Uint32(d.data[d.pos:])
		d.pos += 4
		return d.set32bitValue(rv, v)

	case pbWireType64bit:
		if d.pos+8 > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		v := binary.LittleEndian.Uint64(d.data[d.pos:])
		d.pos += 8
		return d.set64bitValue(rv, v)

	case pbWireTypeLengthDelim:
		l, err := d.readVarint()
		if err != nil {
			return err
		}
		if d.pos+int(l) > len(d.data) {
			return fmt.Errorf("%w: unexpected end", ErrInvalidFormat)
		}
		data := d.data[d.pos : d.pos+int(l)]
		d.pos += int(l)
		return d.setLengthDelimValue(rv, data)

	default:
		return fmt.Errorf("%w: unknown wire type %d", ErrInvalidFormat, wireType)
	}
}

func (d *pbDecoder) setVarintValue(rv reflect.Value, v uint64) error {
	switch rv.Kind() {
	case reflect.Bool:
		rv.SetBool(v != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		rv.SetInt(int64(v))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		rv.SetUint(v)
	default:
		return fmt.Errorf("%w: cannot set varint to %s", ErrInvalidType, rv.Kind())
	}
	return nil
}

func (d *pbDecoder) set32bitValue(rv reflect.Value, v uint32) error {
	switch rv.Kind() {
	case reflect.Float32:
		rv.SetFloat(float64(math.Float32frombits(v)))
	default:
		return fmt.Errorf("%w: cannot set 32-bit to %s", ErrInvalidType, rv.Kind())
	}
	return nil
}

func (d *pbDecoder) set64bitValue(rv reflect.Value, v uint64) error {
	switch rv.Kind() {
	case reflect.Float64:
		rv.SetFloat(math.Float64frombits(v))
	default:
		return fmt.Errorf("%w: cannot set 64-bit to %s", ErrInvalidType, rv.Kind())
	}
	return nil
}

func (d *pbDecoder) setLengthDelimValue(rv reflect.Value, data []byte) error {
	switch rv.Kind() {
	case reflect.String:
		if d.opts.ZeroCopy {
			rv.SetString(zeroCopyString(data))
		} else {
			rv.SetString(string(data))
		}

	case reflect.Slice:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			if d.opts.ZeroCopy {
				rv.SetBytes(data)
			} else {
				cp := make([]byte, len(data))
				copy(cp, data)
				rv.SetBytes(cp)
			}
			return nil
		}
		fallthrough

	case reflect.Array:
		innerDec := &pbDecoder{data: data, pos: 0, opts: d.opts}
		idx := 0
		elemWireType := getWireType(rv.Type().Elem())
		for innerDec.pos < len(innerDec.data) {
			if idx >= rv.Len() && rv.Kind() == reflect.Array {
				break
			}
			if rv.Kind() == reflect.Slice && idx >= rv.Cap() {
				newSlice := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Cap()*2+1)
				reflect.Copy(newSlice, rv)
				rv.Set(newSlice)
			}
			if rv.Kind() == reflect.Slice && idx >= rv.Len() {
				rv.SetLen(idx + 1)
			}

			if err := innerDec.decodeValue(elemWireType, rv.Index(idx)); err != nil {
				return err
			}
			idx++
		}

	case reflect.Struct:
		ser := &ProtoBufSerializer{}
		return ser.Unmarshal(data, rv.Addr().Interface(), d.opts)

	case reflect.Ptr:
		newVal := reflect.New(rv.Type().Elem())
		if err := d.setLengthDelimValue(newVal.Elem(), data); err != nil {
			return err
		}
		rv.Set(newVal)

	default:
		return fmt.Errorf("%w: cannot set length-delimited to %s", ErrInvalidType, rv.Kind())
	}
	return nil
}

func init() {
	Register("protobuf", NewProtoBufSerializer())
}
